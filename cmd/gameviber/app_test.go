//go:build !js

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAppServer: the start page, a new game from a template, the editor under /p/<id>/ with the way back, the
// recent list, and the guards (this machine only; the token for every change).
func TestAppServer(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	s := newAppServer("tok")
	srv := httptest.NewServer(s)
	defer srv.Close()

	post := func(what string, body any, token string) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", srv.URL+"/start/"+what, bytes.NewReader(b))
		req.Header.Set("X-GameViber-Token", token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return map[string]any{"status": resp.StatusCode}
		}
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		return out
	}
	get := func(path string) (int, string) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	if code, body := get("/start/"); code != 200 || !strings.Contains(body, `content="tok"`) {
		t.Fatalf("start page: %d, token meta missing", code)
	}
	if code, body := get("/start/ping"); code != 200 || strings.TrimSpace(body) != "gameviber" {
		t.Fatalf("ping: %d %q", code, body)
	}
	if out := post("new", map[string]string{"name": "rocks", "parent": tmp}, "wrong"); out["status"] != 403 {
		t.Fatalf("a change without the token: %v, want 403", out)
	}
	if out := post("new", map[string]string{"name": "../x", "parent": tmp}, "tok"); out["error"] == nil {
		t.Fatalf("a name with a slash: %v, want an error", out)
	}
	out := post("new", map[string]string{"name": "rocks", "template": "platformer", "parent": tmp}, "tok")
	url, _ := out["url"].(string)
	if !strings.HasPrefix(url, "/p/") {
		t.Fatalf("new: %v", out)
	}
	if _, err := os.Stat(filepath.Join(tmp, "rocks", "game.toml")); err != nil {
		t.Fatal("new: no game.toml", err)
	}
	code, page := get(url)
	if code != 200 || !strings.Contains(page, `href="/start/"`) || !strings.Contains(page, `content="tok"`) {
		t.Fatalf("the editor at %s: %d, without the way back or the token", url, code)
	}
	if code, body := get(url + "api/files"); code != 200 || !strings.Contains(body, "objects") {
		t.Fatalf("the editor's API under %s: %d %.200s", url, code, body)
	}
	if out := post("open", map[string]string{"dir": tmp}, "tok"); out["error"] == nil {
		t.Fatalf("opening a folder without game.toml: %v, want an error", out)
	}
	again := post("open", map[string]string{"dir": filepath.Join(tmp, "rocks")}, "tok")
	if again["url"] != url {
		t.Fatalf("the same folder again: %v, want %s", again, url)
	}
	rec := post("recent", nil, "tok")
	list, _ := rec["recent"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["Dir"] != filepath.Join(tmp, "rocks") {
		t.Fatalf("recent: %v", rec)
	}
	post("forget", map[string]string{"dir": filepath.Join(tmp, "rocks")}, "tok")
	if rec := post("recent", nil, "tok"); rec["recent"] != nil {
		t.Fatalf("recent after forget: %v", rec["recent"])
	}
	if code, _ := get("/p/0000000000/"); code != 200 { // an unknown project: back to the start page (followed)
		t.Fatalf("unknown project: %d", code)
	}

	req := httptest.NewRequest("GET", "/start/", nil)
	req.Host = "example.com"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("another host: %d, want 403", w.Code)
	}
}
