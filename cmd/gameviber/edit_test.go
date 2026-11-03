package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

// Sprites in the editor: read as JSON, the origin / box / precise saved back
// into sprite.toml (the rest kept).
func TestEditSprite(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	r, _ := http.Get(srv.URL + "/api/sprite?path=sprites/coin")
	var s map[string]any
	json.NewDecoder(r.Body).Decode(&s)
	r.Body.Close()
	if s["name"] != "coin" || s["frame_w"] != float64(32) {
		t.Fatalf("sprite: %v", s)
	}
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/sprite?path=sprites/coin", strings.NewReader(`{"origin":[16,16],"bbox_lrbt":[4,27,27,4],"precise":true}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("save: %v %v", err, resp.Status)
	}
	resp.Body.Close()
	r, _ = http.Get(srv.URL + "/api/sprite?path=sprites/coin")
	s = map[string]any{}
	json.NewDecoder(r.Body).Decode(&s)
	r.Body.Close()
	o, _ := s["origin"].([]any)
	if len(o) != 2 || o[0] != float64(16) || s["precise"] != true || s["frame_w"] != float64(32) {
		t.Errorf("after save: %v", s)
	}
}

// A PNG imported as a new sprite of two frames: strip.png written, sprite.toml
// made (frame size from the picture); a picture that doesn't divide is refused.
func TestEditImportPNG(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	var buf bytes.Buffer
	png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 32, 16)))
	r, err := http.Post(srv.URL+"/api/sprite-png?name=star&frames=2", "image/png", bytes.NewReader(buf.Bytes()))
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("import: %v %v", err, r.Status)
	}
	r.Body.Close()
	r, _ = http.Get(srv.URL + "/api/sprite?path=sprites/star")
	var s map[string]any
	json.NewDecoder(r.Body).Decode(&s)
	r.Body.Close()
	if s["frames"] != float64(2) || s["frame_w"] != float64(16) || s["frame_h"] != float64(16) {
		t.Errorf("sprite: %v", s)
	}
	if r, _ := http.Post(srv.URL+"/api/sprite-png?name=star&frames=3", "image/png", bytes.NewReader(buf.Bytes())); r.StatusCode != http.StatusBadRequest {
		t.Errorf("32 px in 3 frames: %d", r.StatusCode)
	}
	if r, _ := http.Post(srv.URL+"/api/sprite-png?name=../x&frames=1", "image/png", bytes.NewReader(buf.Bytes())); r.StatusCode != http.StatusBadRequest {
		t.Errorf("a name with a path: %d", r.StatusCode)
	}
}

// New things from the editor: an object, an event file for it, a room (in the
// room order); the check stays clean; an existing one is not overwritten.
func TestEditNew(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	post := func(q string) int {
		r, err := http.Post(srv.URL+"/api/new?"+q, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	for _, q := range []string{"kind=object&name=enemy", "kind=event&object=enemy&name=step", "kind=room&name=level2"} {
		if c := post(q); c != 200 {
			t.Fatalf("%s: %d", q, c)
		}
	}
	if c := post("kind=object&name=enemy"); c != http.StatusConflict {
		t.Errorf("again: %d", c)
	}
	if c := post("kind=event&object=nobody&name=step"); c != http.StatusBadRequest {
		t.Errorf("an event of no object: %d", c)
	}
	g, _ := os.ReadFile(dir + "/game.toml")
	if !strings.Contains(string(g), `"level2"`) {
		t.Errorf("room order: %s", g)
	}
	var b strings.Builder
	if n, err := checkProject(dir, &b); err != nil || n != 0 {
		t.Errorf("check: %d %v %s", n, err, b.String())
	}
}

// Object settings as a form: read, changed (solid, depth), saved, read back.
func TestEditObject(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	get := func() map[string]any {
		r, _ := http.Get(srv.URL + "/api/object?path=objects/coin")
		var o struct{ Settings map[string]any }
		json.NewDecoder(r.Body).Decode(&o)
		r.Body.Close()
		return o.Settings
	}
	if s := get(); s["sprite"] != "coin" || s["solid"] != false {
		t.Fatalf("coin: %v", s)
	}
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/object?path=objects/coin", strings.NewReader(`{"solid":true,"depth":5,"sprite":"coin"}`))
	if r, err := http.DefaultClient.Do(req); err != nil || r.StatusCode != 200 {
		t.Fatalf("save: %v", err)
	}
	if s := get(); s["solid"] != true || s["depth"] != float64(5) || s["visible"] != true {
		t.Errorf("after: %v", s)
	}
}

// Sounds in the editor: one added (WAV), listed, served with its type; a bad
// name or kind refused.
func TestEditSound(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	wav := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")
	if r, err := http.Post(srv.URL+"/api/sound?name=coin_get&ext=.wav", "audio/wav", bytes.NewReader(wav)); err != nil || r.StatusCode != 200 {
		t.Fatalf("add: %v", err)
	}
	r, _ := http.Get(srv.URL + "/api/files")
	var files struct{ Sounds []string }
	json.NewDecoder(r.Body).Decode(&files)
	r.Body.Close()
	if len(files.Sounds) != 1 || files.Sounds[0] != "coin_get.wav" {
		t.Fatalf("sounds: %v", files.Sounds)
	}
	r, _ = http.Get(srv.URL + "/api/sound?file=coin_get.wav")
	data, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.Header.Get("Content-Type") != "audio/wav" || !bytes.Equal(data, wav) {
		t.Errorf("served: %s, %d bytes", r.Header.Get("Content-Type"), len(data))
	}
	for _, q := range []string{"name=../x&ext=.wav", "name=x&ext=.exe"} {
		if r, _ := http.Post(srv.URL+"/api/sound?"+q, "", bytes.NewReader(wav)); r.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d", q, r.StatusCode)
		}
	}
}

// A room's own settings saved: caption, size, colour; its instances kept.
func TestEditRoomSettings(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	get := func() map[string]any {
		r, _ := http.Get(srv.URL + "/api/room?path=rooms/level")
		var m map[string]any
		json.NewDecoder(r.Body).Decode(&m)
		r.Body.Close()
		return m
	}
	n := len(get()["instances"].([]any))
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/room-settings?path=rooms/level", strings.NewReader(`{"caption":"Big room","width":800,"height":600,"background_colour":"#102030","persistent":true}`))
	if r, err := http.DefaultClient.Do(req); err != nil || r.StatusCode != 200 {
		t.Fatalf("save: %v", err)
	}
	m := get()
	if m["caption"] != "Big room" || m["width"] != float64(800) || m["background"] != "#102030" || m["persistent"] != true || len(m["instances"].([]any)) != n {
		t.Errorf("after: %v", m)
	}
}
