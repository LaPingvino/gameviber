package main

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestProjectZip: Download gives every project file in a folder named after
// the game, without dot folders or this machine's agent configs.
func TestProjectZip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My Game")
	if err := cmdNew([]string{"-template", "platformer", dir}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte("{}"), 0o644)
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	r, err := http.Get(srv.URL + "/api/project.zip")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r.Body)
	if r.Header.Get("Content-Disposition") != `attachment; filename="My-Game.zip"` {
		t.Errorf("disposition %q", r.Header.Get("Content-Disposition"))
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, f := range zr.File {
		have[f.Name] = true
	}
	for _, want := range []string{"My-Game/game.toml", "My-Game/objects/player/step.gml", "My-Game/sprites/player/strip.png", "My-Game/rooms/level2/room.toml"} {
		if !have[want] {
			t.Errorf("missing %s (have %d files)", want, len(have))
		}
	}
	for n := range have {
		if filepath.Base(n) == ".mcp.json" || bytes.Contains([]byte(n), []byte("/.git/")) {
			t.Errorf("%s shipped", n)
		}
	}
	// the zip unpacked is a project gameviber reads
	out := t.TempDir()
	for _, f := range zr.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		p := filepath.Join(out, filepath.FromSlash(f.Name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, data, 0o644)
	}
	if st, err := playProjectOpts(filepath.Join(out, "My-Game"), playOpts{Steps: 5}); err != nil || st["room"] != "level1" {
		t.Errorf("the unpacked zip doesn't play: %v %v", err, st["room"])
	}
}
