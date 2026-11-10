package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"github.com/LaPingvino/gameviber/import/gmfile"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// check finds broken names in the TOML: a room placing a missing object (with
// the near name), an object with a missing sprite, a room order naming a
// missing room.
func TestCheckRefs(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	var clean strings.Builder
	if n := checkRefs(dir, &clean); n != 0 {
		t.Fatalf("the starter game: %d problems\n%s", n, clean.String())
	}
	room, _ := os.ReadFile(dir + "/rooms/level/room.toml")
	os.WriteFile(dir+"/rooms/level/room.toml", append(room, []byte("\n[[instance]]\n  object = \"con\"\n  x = 0\n  y = 0\n")...), 0o644)
	os.WriteFile(dir+"/objects/coin/object.toml", []byte("sprite = \"gold\"\n"), 0o644)
	g, _ := os.ReadFile(dir + "/game.toml")
	os.WriteFile(dir+"/game.toml", []byte(strings.Replace(string(g), `["level"]`, `["level", "level2"]`, 1)), 0o644)
	var b strings.Builder
	n := checkRefs(dir, &b)
	out := b.String()
	if n != 3 || !strings.Contains(out, `rooms/level/room.toml: an instance's object "con" does not exist — did you mean coin?`) ||
		!strings.Contains(out, `objects/coin/object.toml: sprite "gold"`) || !strings.Contains(out, `game.toml: room_order's room "level2"`) {
		t.Errorf("%d problems:\n%s", n, out)
	}
}

// + Moment: a timeline's moment made (the timeline with it), the project
// still loads with it; a bad step refused.
func TestEditNewMoment(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	r, err := http.Post(srv.URL+"/api/new?kind=moment&name=30&object=intro", "", nil)
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("new moment: %v %d", err, r.StatusCode)
	}
	if _, err := os.Stat(dir + "/timelines/intro/moment-30.gml"); err != nil {
		t.Fatal(err)
	}
	p, err := gmfile.ReadDir(dir)
	if err != nil || len(p.Timelines) != 1 || p.Timelines[0].Name != "intro" || len(p.Timelines[0].Moments) != 1 || p.Timelines[0].Moments[0].Step != 30 {
		t.Errorf("loaded: %v %+v", err, p.Timelines)
	}
	for _, q := range []string{"name=x&object=intro", "name=-1&object=intro", "name=05&object=intro", "name=3&object=../x"} {
		if r, _ := http.Post(srv.URL+"/api/new?kind=moment&"+q, "", nil); r.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d", q, r.StatusCode)
		}
	}
}

// gameviber mcp: a session — initialize, a notification (no answer),
// tools/list, check, play with a frame, an unknown tool and method.
func TestMCP(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"check","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"play","arguments":{"steps":20,"keys":"right:20","shot":true}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nope"}}`,
		`{"jsonrpc":"2.0","id":"x","method":"resources/list"}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := serveMCP(dir, strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("%d answers (the notification gets none):\n%s", len(lines), out.String())
	}
	var r []map[string]any
	for _, l := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatal(err)
		}
		r = append(r, m)
	}
	res := func(i int) map[string]any { m, _ := r[i]["result"].(map[string]any); return m }
	if res(0)["protocolVersion"] != "2025-06-18" || res(0)["capabilities"].(map[string]any)["tools"] == nil {
		t.Errorf("initialize: %v", r[0])
	}
	var names []string
	for _, tl := range res(1)["tools"].([]any) {
		names = append(names, tl.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "check,play,outline,read,find,edit,reference" {
		t.Errorf("tools: %v", names)
	}
	check := res(2)["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(check, "0 problems") || res(2)["isError"] != false {
		t.Errorf("check: %v", res(2))
	}
	play := res(3)["content"].([]any)
	if len(play) != 2 || !strings.Contains(play[0].(map[string]any)["text"].(string), `"room":"level"`) || play[1].(map[string]any)["mimeType"] != "image/png" {
		t.Errorf("play: %v", play)
	}
	if img, err := base64.StdEncoding.DecodeString(play[1].(map[string]any)["data"].(string)); err != nil || !bytes.HasPrefix(img, []byte("\x89PNG")) {
		t.Errorf("the frame is no PNG: %v", err)
	}
	if r[4]["error"] == nil || r[5]["error"].(map[string]any)["code"] != float64(-32601) || r[5]["id"] != "x" {
		t.Errorf("unknown tool / method: %v / %v", r[4], r[5])
	}
}

// guardLocal: another host is refused, a change without the token is
// refused, with it accepted; the page carries the token.
func TestGuardLocal(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(guardLocal(editHandler(dir), "secret"))
	defer srv.Close()
	r, _ := http.Get(srv.URL + "/")
	page, _ := io.ReadAll(r.Body)
	if !bytes.Contains(page, []byte(`name="gameviber-token" content="secret"`)) {
		t.Error("the page lacks the token")
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/files", nil)
	req.Host = "evil.example"
	if r, _ := http.DefaultClient.Do(req); r.StatusCode != http.StatusForbidden {
		t.Errorf("another host: %d", r.StatusCode)
	}
	put := func(token string) int {
		req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/file?path=notes.txt", strings.NewReader("hi"))
		if token != "" {
			req.Header.Set("X-GameViber-Token", token)
		}
		r, _ := http.DefaultClient.Do(req)
		return r.StatusCode
	}
	if c := put(""); c != http.StatusForbidden {
		t.Errorf("no token: %d", c)
	}
	if c := put("secret"); c != 200 {
		t.Errorf("with the token: %d", c)
	}
}

// The terminal panel's backend: a shell on a pty in the project folder —
// its output streamed (SSE, base64), input sent back, the exit announced.
func TestEditTerminal(t *testing.T) {
	if !ptySupported {
		t.Skip("no pty here")
	}
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	cmd := url.QueryEscape(`sh -c 'echo ready in $(basename $PWD); read x; echo got $x'`)
	if r, err := http.Post(srv.URL+"/api/term/start?cols=80&rows=24&cmd="+cmd, "", nil); err != nil || r.StatusCode != 200 {
		t.Fatalf("start: %v", err)
	}
	resp, err := http.Get(srv.URL + "/api/term/out")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sentInput, exited := false, false
	deadline := time.Now().Add(10 * time.Second)
	for sc.Scan() && time.Now().Before(deadline) {
		line := sc.Text()
		if line == "event: exit" {
			exited = true
			break
		}
		if b, ok := strings.CutPrefix(line, "data: "); ok && b != "" {
			d, _ := base64.StdEncoding.DecodeString(b)
			out.Write(d)
		}
		if !sentInput && strings.Contains(out.String(), "ready in game") {
			sentInput = true
			http.Post(srv.URL+"/api/term/in", "", strings.NewReader("hello\r"))
		}
	}
	if !strings.Contains(out.String(), "ready in game") || !strings.Contains(out.String(), "got hello") || !exited {
		t.Errorf("exited %v, output %q", exited, out.String())
	}
}

// writeAgentConfig: Claude Code's .mcp.json and OpenCode's opencode.json get
// the gameviber server (mcp DIR) and keep what was there; Aider gets none.
func TestWriteAgentConfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/.mcp.json", []byte(`{"mcpServers":{"other":{"command":"x"}},"keep":1}`), 0o644)
	if f, err := writeAgentConfig(dir, "claude --model sonnet"); err != nil || !strings.HasSuffix(f, ".mcp.json") {
		t.Fatalf("claude: %q %v", f, err)
	}
	var c struct {
		MCPServers map[string]struct {
			Command string
			Args    []string
		}
		Keep int
	}
	data, _ := os.ReadFile(dir + "/.mcp.json")
	json.Unmarshal(data, &c)
	if c.Keep != 1 || c.MCPServers["other"].Command != "x" || len(c.MCPServers["gameviber"].Args) != 2 || c.MCPServers["gameviber"].Args[0] != "mcp" {
		t.Errorf(".mcp.json: %s", data)
	}
	if f, _ := writeAgentConfig(dir, "opencode"); !strings.HasSuffix(f, "opencode.json") {
		t.Errorf("opencode: %q", f)
	}
	data, _ = os.ReadFile(dir + "/opencode.json")
	if !strings.Contains(string(data), `"type": "local"`) || !strings.Contains(string(data), `"mcp"`) {
		t.Errorf("opencode.json: %s", data)
	}
	if f, _ := writeAgentConfig(dir, "aider"); f != "" {
		t.Errorf("aider: %q", f)
	}
}

// A web build's project.tar never holds .git or the agents' configs (this
// machine's paths).
func TestPackSkipsPrivate(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	writeAgentConfig(dir, "claude")
	writeAgentConfig(dir, "opencode")
	os.MkdirAll(dir+"/.git", 0o755)
	os.WriteFile(dir+"/.git/config", []byte("x"), 0o644)
	var b bytes.Buffer
	if err := packProject(dir, &b); err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(&b)
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		if strings.HasPrefix(h.Name, ".git") || h.Name == ".mcp.json" || h.Name == "opencode.json" {
			t.Errorf("shipped %s", h.Name)
		}
	}
}

// Ask: the user's own command gets the question (with the open file) in
// $GAMEVIBER_PROMPT, runs in the project folder, its lines are streamed and
// "done" ends it; the presets build their non-interactive command lines.
func TestEditAsk(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	body := `{"command":"echo in $(basename $PWD); printf '%s\\n' \"$GAMEVIBER_PROMPT\" | tail -1","prompt":"make the coins spin","file":"objects/coin/object.toml"}`
	r, err := http.Post(srv.URL+"/api/ask", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	all, _ := io.ReadAll(r.Body)
	s := string(all)
	if !strings.Contains(s, `data: "in game"`) || !strings.Contains(s, `data: "make the coins spin"`) || !strings.Contains(s, "event: done\ndata: \"ok\"") {
		t.Errorf("stream: %s", s)
	}
	if a := askArgs("claude", "q", true, ""); strings.Join(a, " ") != "claude -p q --allowedTools mcp__gameviber__* Bash(gameviber:*) --permission-mode acceptEdits" {
		t.Errorf("claude: %v", a)
	}
	if a := askArgs("opencode", "q", false, ""); strings.Join(a, " ") != "opencode run q" {
		t.Errorf("opencode: %v", a)
	}
	if a := askArgs("agy", "q", true, "gemini-3.8-flash-low"); strings.Join(a, " ") != `agy -p q --settings {"permissions":{"allow":["command(gameviber)"]}} --mode accept-edits --model gemini-3.8-flash-low` {
		t.Errorf("agy: %v", a)
	}
	if askArgs("nonesuch", "q", false, "") != nil {
		t.Error("unknown agent")
	}
}

// setup-agents' OpenCode part keeps the user's config and adds gameviber
// (`gameviber mcp`, the current folder); the MCP server outside a project
// says so instead of failing.
func TestSetupOpenCodeUser(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	os.MkdirAll(cfg+"/opencode", 0o755)
	os.WriteFile(cfg+"/opencode/opencode.json", []byte(`{"theme":"x","mcp":{"other":{"type":"local","command":["y"]}}}`), 0o644)
	if _, err := setupOpenCodeUser("/bin/gameviber"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(cfg + "/opencode/opencode.json")
	s := string(data)
	if !strings.Contains(s, `"theme": "x"`) || !strings.Contains(s, `"other"`) || !strings.Contains(s, `"/bin/gameviber",`) || !strings.Contains(s, `"mcp"`) {
		t.Errorf("config: %s", s)
	}
	content, isErr, _ := mcpCall(t.TempDir(), "check", nil)
	if !isErr || !strings.Contains(content[0]["text"].(string), "not a GameViber project") {
		t.Errorf("outside a project: %v", content)
	}
}

func TestEditPlayAPI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	r, err := http.Get(srv.URL + "/api/play?steps=30&keys=right:30")
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("play: %v %v", err, r.Status)
	}
	var st struct {
		Room  string `json:"room"`
		Steps int    `json:"steps"`
		Shot  string `json:"shot"`
	}
	json.NewDecoder(r.Body).Decode(&st)
	png, _ := base64.StdEncoding.DecodeString(st.Shot)
	if st.Room == "" || st.Steps != 30 || len(png) < 8 || string(png[1:4]) != "PNG" {
		t.Fatalf("play answer: room %q steps %d shot %d bytes", st.Room, st.Steps, len(png))
	}
	for _, q := range []string{"steps=x", "steps=-1", "keys=jump:3"} {
		if r, _ := http.Get(srv.URL + "/api/play?" + q); r.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", q, r.StatusCode)
		}
	}
}

func TestEditBridge(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	const tok = "secret"
	srv := httptest.NewServer(bridgeGuard(guardLocal(editHandler(dir), tok), tok, pagesOrigin))
	defer srv.Close()
	do := func(method, path, origin, token string, hdr map[string]string) *http.Response {
		req, _ := http.NewRequest(method, srv.URL+path, nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if token != "" {
			req.Header.Set("X-GameViber-Token", token)
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	// the preflight: allowed for the editor's origin, private network included
	r := do(http.MethodOptions, "/api/files", pagesOrigin, "", map[string]string{"Access-Control-Request-Private-Network": "true", "Access-Control-Request-Method": "GET"})
	if r.StatusCode != http.StatusNoContent || r.Header.Get("Access-Control-Allow-Private-Network") != "true" || r.Header.Get("Access-Control-Allow-Origin") != pagesOrigin {
		t.Fatalf("preflight: %d %v", r.StatusCode, r.Header)
	}
	if r := do(http.MethodGet, "/api/files", pagesOrigin, "", nil); r.StatusCode != http.StatusForbidden {
		t.Errorf("read without the token: %d, want 403", r.StatusCode)
	}
	if r := do(http.MethodGet, "/api/files", pagesOrigin, "wrong", nil); r.StatusCode != http.StatusForbidden {
		t.Errorf("read with a wrong token: %d, want 403", r.StatusCode)
	}
	if r := do(http.MethodGet, "/api/files", pagesOrigin, tok, nil); r.StatusCode != 200 || r.Header.Get("Access-Control-Allow-Origin") != pagesOrigin {
		t.Errorf("paired read: %d %q", r.StatusCode, r.Header.Get("Access-Control-Allow-Origin"))
	}
	if r := do(http.MethodGet, "/api/files?gvtoken="+tok, pagesOrigin, "", nil); r.StatusCode != 200 {
		t.Errorf("token in the query: %d", r.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/file?path=notes.txt", strings.NewReader("hi"))
	req.Header.Set("Origin", pagesOrigin)
	req.Header.Set("X-GameViber-Token", tok)
	if r, _ := http.DefaultClient.Do(req); r.StatusCode != 200 {
		t.Errorf("paired write: %d", r.StatusCode)
	}
	// another site gets no CORS answer (its page can't read the reply)
	if r := do(http.MethodGet, "/api/files", "https://evil.example", "", nil); r.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("another origin got CORS: %q", r.Header.Get("Access-Control-Allow-Origin"))
	}
}

// check reads JavaScript events and scripts too: a good one counts, a syntax
// error is reported with its file and line; a .js file elsewhere (a web
// page's) is not game code.
func TestCheckJavaScript(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.Remove(dir + "/objects/player/step.gml") // the step event in JavaScript instead
	os.WriteFile(dir+"/objects/player/step.js", []byte("if (instance_number(coin) == 0) room_goto(room);\n"), 0o644)
	os.MkdirAll(dir+"/scripts", 0o755)
	os.WriteFile(dir+"/scripts/half.js", []byte("return argument0 / 2;\n"), 0o644)
	os.MkdirAll(dir+"/web", 0o755)
	os.WriteFile(dir+"/web/page.js", []byte("this is not ( code"), 0o644)
	var b strings.Builder
	n, err := checkProject(dir, &b)
	if err != nil || n != 0 || !strings.Contains(b.String(), "code files, 0 problems") {
		t.Fatalf("clean: %d problems, %v\n%s", n, err, b.String())
	}
	os.WriteFile(dir+"/scripts/half.js", []byte("let a = 1;\nreturn (argument0 / 2;\n"), 0o644)
	b.Reset()
	n, _ = checkProject(dir, &b)
	if n != 1 || !strings.Contains(b.String(), "scripts/half.js:2") {
		t.Errorf("a syntax error: %d problems\n%s", n, b.String())
	}
	// Unknown names: a call GameViber doesn't know (with its file:line:col and the
	// hint), while a project script, a local function and Math pass.
	os.WriteFile(dir+"/scripts/half.js", []byte("return argument0 / 2;\n"), 0o644)
	os.WriteFile(dir+"/objects/player/step.js", []byte("function twice(n) { return n * 2; }\nx += twice(half(Math.abs(hspeed)));\ninstance_destory();\n"), 0o644)
	b.Reset()
	n, _ = checkProject(dir, &b)
	if n != 1 || !strings.Contains(b.String(), "objects/player/step.js:3:1: instance_destory is not a function") {
		t.Errorf("an unknown name: %d problems\n%s", n, b.String())
	}
}

// Every JavaScript template checks clean: no unknown name where none is.
func TestCheckJavaScriptTemplate(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{"-template", "coins-js", dir}); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if n, err := checkProject(dir, &b); err != nil || n != 0 {
		t.Fatalf("coins-js: %d problems, %v\n%s", n, err, b.String())
	}
}

// check reads Lua events and scripts too, with the file and line of a syntax
// error.
func TestCheckLua(t *testing.T) {
	dir := t.TempDir() + "/game"
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.Remove(dir + "/objects/player/step.gml")
	os.WriteFile(dir+"/objects/player/step.lua", []byte("if instance_number(coin) == 0 then room_goto(room) end\n"), 0o644)
	var b strings.Builder
	if n, err := checkProject(dir, &b); err != nil || n != 0 {
		t.Fatalf("clean: %d problems, %v\n%s", n, err, b.String())
	}
	os.WriteFile(dir+"/objects/player/step.lua", []byte("x = 1\nif x then\n"), 0o644)
	b.Reset()
	if n, _ := checkProject(dir, &b); n != 1 || !strings.Contains(b.String(), "step.lua") {
		t.Errorf("a syntax error: %d problems\n%s", n, b.String())
	}
}
