package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

// TestGameIcon: [settings] icon names a sprite → the export's icons are drawn
// from its first frame; a sprite that doesn't exist is a check problem.
func TestGameIcon(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(filepath.Join(dir, "sprites"))
	if len(ents) == 0 {
		t.Fatal("the starter game has no sprites")
	}
	sprite := ents[0].Name()
	toml := filepath.Join(dir, "game.toml")
	data, _ := os.ReadFile(toml)
	if !strings.Contains(string(data), "[settings]") {
		data = append(data, "\n[settings]\n"...)
	}
	os.WriteFile(toml, append(data, []byte("  icon = \""+sprite+"\"\n")...), 0o644)
	p, err := gmfile.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.IconSprite != sprite {
		t.Fatalf("IconSprite %q, want %q", p.IconSprite, sprite)
	}
	out := t.TempDir()
	if err := writeGameIcons(p, out); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{192, 512} {
		b, err := os.ReadFile(filepath.Join(out, "icon-"+map[int]string{192: "192", 512: "512"}[n]+".png"))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil || img.Bounds().Dx() != n {
			t.Fatalf("icon %d: %v %v", n, err, img.Bounds())
		}
		bg, _, _, _ := img.At(2, 2).RGBA()
		drawn := false
		for y := n / 3; y < 2*n/3 && !drawn; y++ {
			for x := n / 3; x < 2*n/3; x++ {
				if r, g, b, _ := img.At(x, y).RGBA(); r != bg || g != bg || b != bg {
					drawn = true
					break
				}
			}
		}
		if !drawn {
			t.Errorf("icon %d: nothing of the sprite in its middle", n)
		}
	}
	var buf bytes.Buffer
	if n := checkRefs(dir, &buf); n != 0 {
		t.Errorf("a valid icon: %d problems: %s", n, buf.String())
	}
	data, _ = os.ReadFile(toml)
	os.WriteFile(toml, []byte(strings.Replace(string(data), "icon = \""+sprite+"\"", "icon = \""+sprite+"x\"", 1)), 0o644)
	buf.Reset()
	if n := checkRefs(dir, &buf); n != 1 || !strings.Contains(buf.String(), "[settings] icon's sprite") {
		t.Errorf("a missing icon sprite: %d problems: %q", n, buf.String())
	}
}

// TestExportOtherSystem: -os fetches that system's player from the release
// (here a stand-in server), caches it, and appends the project; macOS gets a
// .app folder.
func TestExportOtherSystem(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if !strings.HasPrefix(r.URL.Path, "/gameviber-run-") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("PLAYER:" + r.URL.Path))
	}))
	defer srv.Close()
	t.Setenv("GAMEVIBER_PLAYERS_URL", srv.URL)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := cmdExport([]string{"-desktop", "-os", "windows/amd64", dir, out}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "g.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("PLAYER:/gameviber-run-windows-amd64.exe")) || !bytes.HasSuffix(data, []byte("GVPROJ01")) {
		t.Errorf("windows export: %q … %q", data[:40], data[len(data)-8:])
	}
	if err := cmdExport([]string{"-desktop", "-os", "windows/amd64", dir, t.TempDir()}); err != nil || hits != 1 {
		t.Errorf("second export: %v, %d downloads (want the cached one)", err, hits)
	}
	if err := cmdExport([]string{"-desktop", "-os", "darwin/arm64", dir, out}); err != nil {
		t.Fatal(err)
	}
	plist, err := os.ReadFile(filepath.Join(out, "g.app", "Contents", "Info.plist"))
	if err != nil || !strings.Contains(string(plist), "<key>CFBundleExecutable</key><string>g</string>") {
		t.Errorf("Info.plist: %v %s", err, plist)
	}
	if fi, err := os.Stat(filepath.Join(out, "g.app", "Contents", "MacOS", "g")); err != nil || fi.Mode()&0o100 == 0 {
		t.Errorf("the .app's program: %v", err)
	}
	if err := cmdExport([]string{"-desktop", "-os", "plan9/mips", dir, out}); err == nil {
		t.Error("an unknown system was accepted")
	}
}

// TestPackVariant: the packed project's game.toml is the variant's merge.
func TestPackVariant(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "demo.toml"), []byte("name = \"Demo\"\n[constants]\n  DEMO = \"1\"\n"), 0o644)
	var b bytes.Buffer
	if err := packProjectVariant(dir, "demo", &b); err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(&b)
	for {
		h, err := tr.Next()
		if err != nil {
			t.Fatal("no game.toml in the pack")
		}
		if h.Name == "game.toml" {
			data, _ := io.ReadAll(tr)
			if !strings.Contains(string(data), `name = "Demo"`) || !strings.Contains(string(data), `DEMO = "1"`) || !strings.Contains(string(data), "room_order") {
				t.Errorf("packed game.toml:\n%s", data)
			}
			return
		}
	}
}

// TestVariantCheckPlay: check and play see the variant's game.toml.
func TestVariantCheckPlay(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "demo.toml"), []byte("name = \"Demo\"\n[constants]\n  DEMO = \"1\"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "broken.toml"), []byte("room_order = [\"nosuch\"]\n"), 0o644)
	var b strings.Builder
	if n, err := checkProjectVariant(dir, "demo", &b); err != nil || n != 0 {
		t.Errorf("demo: %v, %d problems: %s", err, n, b.String())
	}
	b.Reset()
	if n, err := checkProjectVariant(dir, "broken", &b); n == 0 || !strings.Contains(b.String(), "nosuch") {
		t.Errorf("broken: %d problems: %q (err %v)", n, b.String(), err)
	}
	b.Reset()
	if n, err := checkProject(dir, &b); err != nil || n != 0 {
		t.Errorf("no variant: %v, %d: %s", err, n, b.String())
	}
	if st, err := playProjectVariant(dir, "demo", "", 10, "", nil); err != nil || st["room"] == "" {
		t.Errorf("play demo: %v %v", err, st)
	}
	if _, err := playProjectVariant(dir, "nope", "", 10, "", nil); err == nil {
		t.Error("play with a missing variant ran")
	}
}

// TestEditPlayVariant: the editor's play/project.tar packs the chosen variant.
func TestEditPlayVariant(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "demo.toml"), []byte("name = \"Demo\"\n"), 0o644)
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	r, err := http.Get(srv.URL + "/play/project.tar?variant=demo")
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("variant demo: %v %v", err, r.Status)
	}
	tr := tar.NewReader(r.Body)
	found := false
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		if h.Name == "game.toml" {
			data, _ := io.ReadAll(tr)
			found = strings.Contains(string(data), `name = "Demo"`)
		}
	}
	if !found {
		t.Error("the packed game.toml is not the demo's")
	}
	for _, q := range []string{"../x", "nope"} {
		if r, _ := http.Get(srv.URL + "/play/project.tar?variant=" + q); r.StatusCode != http.StatusBadRequest {
			t.Errorf("variant %q: %d, want 400", q, r.StatusCode)
		}
	}
}

// TestRoomCamera: the room settings' camera (views_enabled, view 0 following
// an object) is written into room.toml and plays as a view of that size.
func TestRoomCamera(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	put := func(body string) int {
		req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/room-settings?path=rooms/level", strings.NewReader(body))
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return r.StatusCode
	}
	if c := put(`{"views_enabled":true,"view0":{"w":320,"h":240,"follow":"player","hborder":100,"vborder":80}}`); c != 200 {
		t.Fatalf("camera: %d", c)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "rooms", "level", "room.toml"))
	for _, want := range []string{"views_enabled = true", `follow = "player"`, "view = [0, 0, 320, 240]", "border = [100, 80]"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("room.toml lacks %q:\n%s", want, data)
		}
	}
	var shot bytes.Buffer
	if _, err := playProject(dir, "", 5, "", &shot); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&shot)
	if err != nil || img.Bounds().Dx() != 320 || img.Bounds().Dy() != 240 {
		t.Errorf("the played frame: %v %v (want the view's 320x240)", err, img.Bounds())
	}
	r, _ := http.Get(srv.URL + "/api/room?path=rooms/level")
	var got struct {
		ViewsEnabled bool           `json:"views_enabled"`
		View0        map[string]any `json:"view0"`
	}
	json.NewDecoder(r.Body).Decode(&got)
	if !got.ViewsEnabled || got.View0["follow"] != "player" || got.View0["w"] != float64(320) {
		t.Errorf("read back: %+v", got)
	}
	for _, bad := range []string{`{"view0":{"follow":"nosuch"}}`, `{"view0":{"follow":"../x"}}`} {
		if c := put(bad); c != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", bad, c)
		}
	}
}

// TestInstanceCodeCheck: an instance's creation code is checked as GML.
func TestInstanceCodeCheck(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "rooms", "level", "room.toml")
	data, _ := os.ReadFile(file)
	good := strings.Replace(string(data), "[[instance]]", "[[instance]]\n  code = \"hspeed = 2\"", 1)
	os.WriteFile(file, []byte(good), 0o644)
	var b strings.Builder
	if n := checkRefs(dir, &b); n != 0 {
		t.Errorf("valid instance code: %d problems: %s", n, b.String())
	}
	if p, err := gmfile.ReadDir(dir); err != nil || p.Rooms[0].Instances[0].Code != "hspeed = 2" {
		t.Errorf("the code doesn't load: %v", err)
	}
	bad := strings.Replace(string(data), "[[instance]]", "[[instance]]\n  code = \"hspeed = (\"", 1)
	os.WriteFile(file, []byte(bad), 0o644)
	b.Reset()
	if n := checkRefs(dir, &b); n != 1 || !strings.Contains(b.String(), "creation code") {
		t.Errorf("broken instance code: %d: %q", n, b.String())
	}
}

// TestRoomBackground: a PNG imported as a background, set tiled on a room,
// written as tile_h / tile_v and drawn by the game.
func TestRoomBackground(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "g")
	if err := cmdNew([]string{dir}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(editHandler(dir))
	defer srv.Close()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for i := range img.Pix {
		img.Pix[i] = []uint8{255, 0, 255, 255}[i%4]
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	if r, err := http.Post(srv.URL+"/api/background-png?name=sky", "image/png", &b); err != nil || r.StatusCode != 200 {
		t.Fatalf("import: %v %v", err, r.Status)
	}
	put := func(body string) int {
		req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/room-settings?path=rooms/level", strings.NewReader(body))
		r, _ := http.DefaultClient.Do(req)
		return r.StatusCode
	}
	if c := put(`{"background0":{"background":"sky","tileh":true,"tilev":true}}`); c != 200 {
		t.Fatalf("set: %d", c)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "rooms", "level", "room.toml"))
	if !strings.Contains(string(data), `background = "sky"`) || !strings.Contains(string(data), "tile_h = true") {
		t.Errorf("room.toml:\n%s", data)
	}
	var shot bytes.Buffer
	if _, err := playProject(dir, "", 2, "", &shot); err != nil {
		t.Fatal(err)
	}
	frame, _ := png.Decode(&shot)
	if r, g, bl, _ := frame.At(300, 200).RGBA(); r>>8 != 255 || g>>8 != 0 || bl>>8 != 255 { // open floor: the tiled background
		t.Errorf("open floor pixel %d,%d,%d: not the background", r>>8, g>>8, bl>>8)
	}
	for _, bad := range []string{`{"background0":{"background":"../x"}}`, `{"background0":{"background":"nosuch"}}`} {
		if c := put(bad); c != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, c)
		}
	}
	if put(`{"background0":{"background":""}}`) != 200 {
		t.Error("removing the background")
	}
	data, _ = os.ReadFile(filepath.Join(dir, "rooms", "level", "room.toml"))
	if strings.Contains(string(data), "[[background]]") {
		t.Errorf("not removed:\n%s", data)
	}
}
