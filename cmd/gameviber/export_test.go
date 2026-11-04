package main

import (
	"archive/tar"
	"bytes"
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
