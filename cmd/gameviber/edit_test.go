package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The editor's API over a starter project: the file list, reading, saving
// (with gameviber check after), and no way out of the project.
func TestEditAPI(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	get := func(path string) (int, string) {
		r, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(b)
	}
	put := func(path, body string) (int, string) {
		req, _ := http.NewRequest(http.MethodPut, srv.URL+path, strings.NewReader(body))
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(b)
	}
	if code, page := get("/"); code != 200 || !strings.Contains(page, "GameViber editor") {
		t.Fatalf("page: %d", code)
	}
	_, list := get("/api/files")
	var files struct{ Files []string }
	json.Unmarshal([]byte(list), &files)
	if !strings.Contains(strings.Join(files.Files, " "), "objects/player/step.gml") {
		t.Fatalf("files: %v", files.Files)
	}
	if code, _ := get("/api/file?path=objects/player/step.gml"); code != 200 {
		t.Fatalf("read: %d", code)
	}
	var c struct {
		Problems int
		Report   string
	}
	_, res := put("/api/file?path=objects/player/step.gml", "x = (1 +\n")
	json.Unmarshal([]byte(res), &c)
	if c.Problems == 0 || !strings.Contains(c.Report, "objects/player/step.gml") {
		t.Errorf("a syntax error not reported: %+v", c)
	}
	_, res = put("/api/file?path=objects/player/step.gml", "x += 1\n")
	json.Unmarshal([]byte(res), &c)
	if c.Problems != 0 {
		t.Errorf("fixed but still: %+v", c)
	}
	for _, p := range []string{"../../etc/passwd", "/etc/passwd", "objects/../../x.gml"} {
		if code, _ := get("/api/file?path=" + p); code != http.StatusBadRequest {
			t.Errorf("read %s: %d, want refused", p, code)
		}
	}
	if code, _ := put("/api/file?path=game.exe", "x"); code != http.StatusBadRequest {
		t.Errorf("writing a non-text file: %d, want refused", code)
	}
}

// Rooms in the editor: read as JSON, an instance added and saved back into
// room.toml (the room's settings kept), the check still clean.
func TestEditRoom(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	var room struct {
		Width     any
		Instances []map[string]any
		Objects   []editObject
		Sprites   map[string]editSprite
	}
	r, err := http.Get(srv.URL + "/api/room?path=rooms/level")
	if err != nil {
		t.Fatal(err)
	}
	json.NewDecoder(r.Body).Decode(&room)
	r.Body.Close()
	n := len(room.Instances)
	if n == 0 || len(room.Objects) == 0 || room.Sprites["coin"].W != 32 {
		t.Fatalf("room: %d instances, objects %v, sprites %v", n, room.Objects, room.Sprites)
	}
	room.Instances = append(room.Instances, map[string]any{"object": "coin", "x": 320, "y": 224})
	body, _ := json.Marshal(room.Instances)
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/room?path=rooms/level", strings.NewReader(string(body)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var c struct{ Problems int }
	json.NewDecoder(resp.Body).Decode(&c)
	resp.Body.Close()
	if resp.StatusCode != 200 || c.Problems != 0 {
		t.Fatalf("save: %d, %d problems", resp.StatusCode, c.Problems)
	}
	r, _ = http.Get(srv.URL + "/api/room?path=rooms/level")
	var again struct {
		Width     any
		Instances []map[string]any
	}
	json.NewDecoder(r.Body).Decode(&again)
	r.Body.Close()
	last := again.Instances[len(again.Instances)-1]
	if len(again.Instances) != n+1 || last["object"] != "coin" || last["x"] != float64(320) || again.Width != float64(640) {
		t.Errorf("after save: %d instances, last %v, width %v", len(again.Instances), last, again.Width)
	}
	if code, _ := http.Get(srv.URL + "/api/strip?sprite=../game.toml"); code.StatusCode != http.StatusBadRequest {
		t.Errorf("strip outside sprites: %d", code.StatusCode)
	}
}

// Play in the editor: the page names a fresh project pack, and the pack holds
// the project's files as saved.
func TestEditPlay(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	r, err := http.Get(srv.URL + "/play/")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if !strings.Contains(string(page), `gameviberProject = "project.tar?t=`) {
		t.Fatalf("page: %s", page)
	}
	r, _ = http.Get(srv.URL + "/play/project.tar")
	data, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 200 || !strings.Contains(string(data), "objects/player/step.gml") || !strings.Contains(string(data), "rooms/level/room.toml") {
		t.Errorf("pack: %d, %d bytes", r.StatusCode, len(data))
	}
}
