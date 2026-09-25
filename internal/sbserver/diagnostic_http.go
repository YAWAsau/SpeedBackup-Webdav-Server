package sbserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type diagnosticContextKey struct{}
type requestDiagnostic struct {
	ID            string  `json:"request_id"`
	Time          string  `json:"time"`
	Kind          string  `json:"kind"`
	Method        string  `json:"method"`
	Protocol      string  `json:"protocol"`
	UserAgent     string  `json:"user_agent,omitempty"`
	Overwrite     string  `json:"overwrite,omitempty"`
	Chunked       bool    `json:"chunked"`
	Path          string  `json:"path"`
	Destination   string  `json:"destination,omitempty"`
	Remote        string  `json:"remote"`
	User          string  `json:"username,omitempty"`
	ContentLength int64   `json:"content_length"`
	Range         string  `json:"range,omitempty"`
	Depth         string  `json:"depth,omitempty"`
	Status        int     `json:"status"`
	Read          int64   `json:"request_bytes"`
	Written       int64   `json:"response_bytes"`
	Elapsed       float64 `json:"elapsed_ms"`
	Error         string  `json:"error,omitempty"`
}

var diagnosticSequence atomic.Uint64

func requestTrace(r *http.Request) *requestDiagnostic {
	v, _ := r.Context().Value(diagnosticContextKey{}).(*requestDiagnostic)
	return v
}
func requestTraceID(r *http.Request) string {
	if v := requestTrace(r); v != nil {
		return v.ID
	}
	return ""
}
func requestTraceError(r *http.Request, err error) {
	if v := requestTrace(r); v != nil && err != nil {
		v.Error = clipDiagnostic(err.Error(), 2048)
	}
}

func diagnosticPath(p string) string {
	for _, prefix := range []string{"/api/v1/native-picker/", "/api/v1/admin/directories/picker/"} {
		if strings.HasPrefix(p, prefix) {
			return prefix + "[redacted]"
		}
	}
	return clipDiagnostic(p, 4096)
}
func (s *HTTPServer) traceRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		v := &requestDiagnostic{ID: fmt.Sprintf("%x-%x", start.UnixMilli(), diagnosticSequence.Add(1)), Time: start.Format(time.RFC3339Nano), Kind: "http_request", Method: r.Method, Path: diagnosticPath(r.URL.Path), Remote: r.RemoteAddr, ContentLength: r.ContentLength, Range: clipDiagnostic(r.Header.Get("Range"), 128), Depth: clipDiagnostic(r.Header.Get("Depth"), 16)}
		if target, err := url.Parse(r.Header.Get("Destination")); err == nil {
			v.Destination = diagnosticPath(target.Path)
		}
		v.Protocol = r.Proto
		v.UserAgent = clipDiagnostic(r.UserAgent(), 512)
		v.Overwrite = clipDiagnostic(r.Header.Get("Overwrite"), 8)
		v.Chunked = len(r.TransferEncoding) > 0
		// No URL query, headers, credentials or request/response body is serialized.
		r = r.WithContext(context.WithValue(r.Context(), diagnosticContextKey{}, v))
		if r.Body != nil {
			r.Body = &diagnosticBody{ReadCloser: r.Body, trace: v}
		}
		meter := &diagnosticResponse{ResponseWriter: w, trace: v}
		defer func() {
			failure := recover()
			if failure != nil {
				v.Status = 500
				v.Error = "handler panicked; see http_server_error"
			}
			if v.Status == 0 {
				v.Status = 200
			}
			v.Elapsed = float64(time.Since(start).Microseconds()) / 1000
			if err := r.Context().Err(); err != nil {
				requestTraceError(r, err)
			}
			if v.Status >= 400 && v.Error == "" {
				v.Error = http.StatusText(v.Status)
			}
			s.store.diagnostics.append(v)
			if failure != nil {
				panic(failure)
			}
		}()
		next.ServeHTTP(meter, r)
	})
}

type diagnosticBody struct {
	io.ReadCloser
	trace *requestDiagnostic
}

func (b *diagnosticBody) Read(p []byte) (int, error) {
	n, e := b.ReadCloser.Read(p)
	b.trace.Read += int64(n)
	if e != nil && e != io.EOF {
		b.trace.Error = clipDiagnostic(e.Error(), 2048)
	}
	return n, e
}

type diagnosticResponse struct {
	http.ResponseWriter
	trace *requestDiagnostic
}

func (w *diagnosticResponse) WriteHeader(code int) {
	if code >= 100 && code < 200 {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if w.trace.Status != 0 {
		return
	}
	w.trace.Status = code
	w.ResponseWriter.WriteHeader(code)
}
func (w *diagnosticResponse) Write(p []byte) (int, error) {
	if w.trace.Status == 0 {
		w.WriteHeader(200)
	}
	n, e := w.ResponseWriter.Write(p)
	w.trace.Written += int64(n)
	if e != nil {
		w.trace.Error = clipDiagnostic(e.Error(), 2048)
	}
	return n, e
}
func (w *diagnosticResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *diagnosticResponse) Flush() {
	if w.trace.Status == 0 {
		w.WriteHeader(200)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
