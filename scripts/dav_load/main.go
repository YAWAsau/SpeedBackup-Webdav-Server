// Isolated loopback load probe. It creates a unique directory and checks every
// downloaded payload. Run only against a disposable server, never a real share.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type pacedReader struct {
	data  *bytes.Reader
	delay time.Duration
}

func (p *pacedReader) Read(b []byte) (int, error) {
	if len(b) > 32768 {
		b = b[:32768]
	}
	n, e := p.data.Read(b)
	if n > 0 {
		time.Sleep(p.delay)
	}
	return n, e
}
func main() {
	base := flag.String("url", "", "disposable loopback WebDAV server")
	parallel := flag.Int("parallel", 1, "concurrent jobs")
	count := flag.Int("count", 64, "jobs")
	size := flag.Int("bytes", 2<<20, "payload bytes per job")
	mode := flag.String("mode", "mixed", "put, get or mixed (PUT HEAD MOVE GET)")
	delay := flag.Int("delay-ms", 0, "delay per 32 KiB upload chunk")
	flag.Parse()
	u, e := url.Parse(*base)
	if e != nil || u.Hostname() != "127.0.0.1" || u.Port() == "8765" || *parallel < 1 || *count < 1 || *size < 1 {
		panic("use disposable 127.0.0.1 server outside port 8765")
	}
	client := &http.Client{Timeout: 2 * time.Minute, Transport: &http.Transport{MaxIdleConns: 256, MaxIdleConnsPerHost: 128}}
	dir := fmt.Sprintf("%s/load-%d/", *base, time.Now().UnixNano())
	pattern := []byte("Backup payload verification.012345")
	data := bytes.Repeat(pattern, *size/len(pattern)+1)[:*size]
	want := sha256.Sum256(data)
	var transferred atomic.Int64
	do := func(method, target string, body io.Reader, headers map[string]string, verify bool) error {
		r, e := http.NewRequest(method, target, body)
		if e != nil {
			return e
		}
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		res, e := client.Do(r)
		if e != nil {
			return e
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			b, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
			return fmt.Errorf("%s: %d %s", method, res.StatusCode, b)
		}
		if verify {
			h := sha256.New()
			n, e := io.Copy(h, res.Body)
			if e != nil {
				return e
			}
			if n != int64(len(data)) || !bytes.Equal(h.Sum(nil), want[:]) {
				return fmt.Errorf("SHA256 mismatch")
			}
			transferred.Add(n)
		} else {
			_, e = io.Copy(io.Discard, res.Body)
			if e != nil {
				return e
			}
		}
		if method == "PUT" {
			transferred.Add(int64(len(data)))
		}
		return nil
	}
	if e = do("MKCOL", dir, nil, nil, false); e != nil {
		panic(e)
	}
	defer func() {
		if e := do("DELETE", dir, nil, nil, false); e != nil {
			fmt.Fprintln(os.Stderr, "cleanup:", e)
		}
	}()
	if *mode == "get" {
		if e := do("PUT", dir+"seed", bytes.NewReader(data), nil, false); e != nil {
			panic(e)
		}
	}
	transferred.Store(0)
	jobs := make(chan int)
	results := make(chan time.Duration, *count)
	failures := make(chan string, *count)
	var wg sync.WaitGroup
	start := time.Now()
	for range *parallel {
		wg.Go(func() {
			for id := range jobs {
				begin := time.Now()
				name := fmt.Sprintf("%s%d.part", dir, id)
				var e error
				if *mode == "get" {
					e = do("GET", dir+"seed", nil, nil, true)
				} else {
					var reader io.Reader = bytes.NewReader(data)
					if *delay > 0 {
						reader = &pacedReader{bytes.NewReader(data), time.Duration(*delay) * time.Millisecond}
					}
					e = do("PUT", name, reader, nil, false)
					if e == nil && *mode == "mixed" {
						e = do("HEAD", name, nil, nil, false)
						if e == nil {
							e = do("MOVE", name, nil, map[string]string{"Destination": name + ".done"}, false)
						}
						if e == nil {
							e = do("GET", name+".done", nil, nil, true)
						}
					}
				}
				if e != nil {
					failures <- e.Error()
				}
				results <- time.Since(begin)
			}
		})
	}
	for i := range *count {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	elapsed := time.Since(start)
	close(results)
	close(failures)
	latencies := []float64{}
	for d := range results {
		latencies = append(latencies, float64(d)/float64(time.Millisecond))
	}
	sort.Float64s(latencies)
	errs := []string{}
	for e := range failures {
		errs = append(errs, e)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"mode": *mode, "parallel": *parallel, "jobs": *count, "payload_bytes": *size, "upload_delay_ms_per_32KiB": *delay, "elapsed_seconds": elapsed.Seconds(), "transferred_bytes": transferred.Load(), "MiB_per_second": float64(transferred.Load()) / (1 << 20) / elapsed.Seconds(), "p50_ms": latencies[len(latencies)/2], "p95_ms": latencies[min(len(latencies)-1, len(latencies)*95/100)], "errors": errs, "verified_downloads": map[bool]int{true: *count, false: 0}[*mode == "mixed" || *mode == "get"]})
	if len(errs) > 0 {
		os.Exit(1)
	}
}
