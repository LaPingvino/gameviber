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

// TestUnpackProjectZip: Download's zip unpacks to the same files; a zip
// without game.toml, and one with a path leaving the project, are refused.
func TestUnpackProjectZip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Coins")
	if err := cmdNew([]string{"-template", "topdown", src}); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := zipProject(src, "Coins", &b); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "back")
	if err := unpackProjectZip(b.Bytes(), out); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"game.toml", "objects/ghost/step.gml", "sprites/gem/strip.png", "rooms/maze/room.toml"} {
		a, _ := os.ReadFile(filepath.Join(src, f))
		c, err := os.ReadFile(filepath.Join(out, f))
		if err != nil || !bytes.Equal(a, c) {
			t.Errorf("%s: %v (same %v)", f, err, bytes.Equal(a, c))
		}
	}
	if st, err := playProjectOpts(out, playOpts{Steps: 3}); err != nil || st["room"] != "maze" {
		t.Errorf("doesn't play: %v %v", err, st["room"])
	}
	mk := func(files map[string]string) []byte {
		var z bytes.Buffer
		w := zip.NewWriter(&z)
		for n, c := range files {
			f, _ := w.Create(n)
			f.Write([]byte(c))
		}
		w.Close()
		return z.Bytes()
	}
	if err := unpackProjectZip(mk(map[string]string{"notes.txt": "hi"}), filepath.Join(t.TempDir(), "x")); err == nil {
		t.Error("a zip without game.toml was taken")
	}
	escape := filepath.Join(t.TempDir(), "y")
	if err := unpackProjectZip(mk(map[string]string{"game.toml": "name = \"g\"\n", "../evil.txt": "x"}), escape); err == nil {
		t.Error("a path leaving the project was taken")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(escape), "evil.txt")); err == nil {
		t.Error("evil.txt was written outside")
	}
}

// GET api/names: the editor's completions — GameViber's functions, built-in
// variables, constants, and this game's own resources.
func TestEditorNames(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	n := projectNames(dir)
	has := func(list []string, w string) bool {
		for _, x := range list {
			if x == w {
				return true
			}
		}
		return false
	}
	if !has(n.Functions, "instance_create") || !has(n.Functions, "draw_text_ext") || !has(n.Variables, "current_time") ||
		!has(n.Variables, "hspeed") || !has(n.Constants, "vk_left") || !has(n.Resources, "player") || !has(n.Resources, "coin") {
		t.Errorf("missing names: %d functions, %d variables, %d constants, resources %v", len(n.Functions), len(n.Variables), len(n.Constants), n.Resources)
	}
}
