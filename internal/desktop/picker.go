// Package desktop connects the installed Windows folder dialog to a short-lived,
// administrator-created selection request. It never receives login credentials.
package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

var Protocol = "speedbackup-picker"

var requestID = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ParseLaunch(raw string) (port int, id string, err error) {
	u, err := url.Parse(raw)
	// Windows adds a root slash to authority-only URLs during canonicalization.
	// Keep accepting the previous spelling, but reject other or escaped paths.
	if err != nil || u.Scheme != Protocol || u.Host != "choose" || (u.EscapedPath() != "" && u.EscapedPath() != "/") || u.User != nil || u.Fragment != "" {
		return 0, "", fmt.Errorf("invalid folder-picker request")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(q) != 2 || len(q["port"]) != 1 || len(q["request"]) != 1 {
		return 0, "", fmt.Errorf("invalid folder-picker parameters")
	}
	port, err = strconv.Atoi(q.Get("port"))
	id = q.Get("request")
	if err != nil || port < 1 || port > 65535 || !requestID.MatchString(id) {
		return 0, "", fmt.Errorf("invalid folder-picker parameters")
	}
	return port, id, nil
}

func Run(raw string) error {
	port, id, err := ParseLaunch(raw)
	if err != nil {
		return err
	}
	// Do not send the selected path to an SSH tunnel, proxy or unrelated process.
	if !ServiceOwnsPort(port) {
		return fmt.Errorf("此連接埠不是本機 SpeedBackup Windows 服務，請改用網頁目錄瀏覽")
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/api/v1/native-picker/%s", port, id)
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(endpoint)
	if err != nil {
		return err
	}
	var initial struct {
		Directory string `json:"directory"`
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 8192)).Decode(&initial)
	response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("資料夾選擇要求已失效，請回管理頁重試")
	}
	if err != nil {
		return err
	}
	path, cancelled, pickErr := ChooseFolder(initial.Directory)
	result := struct {
		Path      string `json:"path"`
		Cancelled bool   `json:"cancelled"`
		Error     string `json:"error"`
	}{Path: path, Cancelled: cancelled}
	if pickErr != nil {
		result.Error = pickErr.Error()
	}
	if !ServiceOwnsPort(port) {
		return fmt.Errorf("SpeedBackup 服務已變更，請回管理頁重試")
	}
	body, _ := json.Marshal(result)
	response, err = client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("選擇要求已逾時，請回管理頁重新選擇")
	}
	return pickErr
}
