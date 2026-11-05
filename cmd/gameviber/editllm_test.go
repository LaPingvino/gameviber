package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnvKeyProxy: a key in gameviber edit's environment — the page learns the
// provider, never the key; its requests reach the configured endpoint with the
// key added, the answer comes back; without a key, nothing is forwarded.
func TestEnvKeyProxy(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization")+r.Header.Get("x-api-key"), r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"hello from the model"}}]}`)
	}))
	defer up.Close()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-secret-123")
	t.Setenv("OPENAI_BASE_URL", up.URL+"/v1")
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	r, err := http.Get(srv.URL + "/api/llm")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(b), `"id":"openai"`) || strings.Contains(string(b), "sk-secret") || strings.Contains(string(b), "anthropic") {
		t.Errorf("providers: %s", b)
	}
	r, err = http.Post(srv.URL+"/api/llm?prov=openai", "application/json", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(r.Body)
	if r.StatusCode != 200 || !strings.Contains(string(b), "hello from the model") {
		t.Errorf("answer %d: %s", r.StatusCode, b)
	}
	if gotAuth != "Bearer sk-secret-123" || gotPath != "/v1/chat/completions" || !strings.Contains(gotBody, `"content":"hi"`) {
		t.Errorf("forwarded: auth %q path %q body %s", gotAuth, gotPath, gotBody)
	}
	gotPath = ""
	r, _ = http.Post(srv.URL+"/api/llm?prov=anthropic", "application/json", strings.NewReader(`{}`))
	if r.StatusCode != http.StatusNotFound || gotPath != "" {
		t.Errorf("no Anthropic key: status %d, forwarded to %q", r.StatusCode, gotPath)
	}
	r, _ = http.Post(srv.URL+"/api/llm?prov=openai", "application/json", strings.NewReader(`not json`))
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("not JSON: status %d", r.StatusCode)
	}
}
