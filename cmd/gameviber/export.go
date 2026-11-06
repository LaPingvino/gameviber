package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

// cmdExport is `gameviber export DIR OUT` (LOOP item 6): the finished web
// build — the player, the project and an installable-app layer (a manifest
// with the game's name, opening full screen; icons; the offline service
// worker) — as a folder, or as a zip with index.html at its root (itch.io's
// HTML5 upload, or any static host) when OUT ends in .zip.
func cmdExport(args []string) error {
	variant, args := takeVariant(args)
	if len(args) >= 3 && args[0] == "-desktop" {
		target := runtime.GOOS + "/" + runtime.GOARCH
		if len(args) == 5 && args[1] == "-os" {
			target, args = args[2], append([]string{args[0]}, args[3:]...)
		}
		if len(args) == 3 {
			return exportDesktop(args[1], args[2], target, variant)
		}
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: gameviber export DIR OUT   (OUT: a folder, or name.zip for itch.io; -variant NAME first: NAME.toml merged over game.toml)\n       gameviber export -desktop DIR OUT   (OUT: a folder; the game as a program for this system)\n       gameviber export -desktop -os OS/ARCH DIR OUT   (another system: linux, windows, darwin / amd64, arm64)")
	}
	dir, out := args[0], args[1]
	p, err := gmfile.ReadDirVariant(dir, variant)
	if err != nil {
		return err
	}
	name := p.Name()
	if name == "" {
		name = filepath.Base(filepath.Clean(dir))
	}
	build := out
	asZip := strings.EqualFold(filepath.Ext(out), ".zip")
	if asZip {
		if build, err = os.MkdirTemp("", "gameviber-export-"); err != nil {
			return err
		}
		defer os.RemoveAll(build)
	}
	if _, err := webBuild(dir, build, variant); err != nil {
		return err
	}
	if err := writePWAFiles(build, pwaManifest(name, shortName(name), name+" — made with GameViber", "fullscreen", "#111111", "#111111"),
		"gameviber-game-"+cacheKey(name)); err != nil {
		return err
	}
	if err := writeGameIcons(p, build); err != nil {
		return err
	}
	page, err := os.ReadFile(filepath.Join(build, "index.html"))
	if err != nil {
		return err
	}
	page = []byte(strings.Replace(string(page), "</head>", pwaRegister+"</head>", 1))
	if err := os.WriteFile(filepath.Join(build, "index.html"), page, 0o644); err != nil {
		return err
	}
	if asZip {
		if err := zipDir(build, out); err != nil {
			return err
		}
	}
	fmt.Printf("exported %s to %s (web, installable)\n", name, out)
	return nil
}

// shortName is the name for a home screen: at most 12 characters.
func shortName(name string) string {
	if r := []rune(name); len(r) > 12 {
		return string(r[:12])
	}
	return name
}

// cacheKey is the name made safe for a cache name.
func cacheKey(name string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return '-'
	}, name)
}

// zipDir writes the files of dir into the zip file out, paths relative to dir.
func zipDir(dir, out string) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(w, src)
		return err
	})
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// writeGameIcons replaces the icons with the game's own when game.toml names
// one ([settings] icon = a sprite): its first frame, scaled by whole steps
// (pixel art stays sharp) to fit the middle 70% (the maskable safe zone) of a
// dark square.
func writeGameIcons(p *gmfile.Project, out string) error {
	if p.IconSprite == "" {
		return nil
	}
	var frame image.Image
	for _, s := range p.Sprites {
		if s != nil && s.Name == p.IconSprite && len(s.Images) > 0 {
			frame = s.Images[0]
		}
	}
	if frame == nil {
		return fmt.Errorf("game.toml [settings] icon: no sprite %q with a frame", p.IconSprite)
	}
	for _, n := range []int{192, 512} {
		var b bytes.Buffer
		if err := png.Encode(&b, iconFrom(frame, n)); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("icon-%d.png", n)), b.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// iconFrom draws frame centred on an n × n dark square, scaled by the largest
// whole step that fits 70% of it (at least 1; larger frames are shrunk).
func iconFrom(frame image.Image, n int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, n, n))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.RGBA{0x11, 0x11, 0x11, 0xff}), image.Point{}, draw.Src)
	fb := frame.Bounds()
	fw, fh := fb.Dx(), fb.Dy()
	if fw == 0 || fh == 0 {
		return dst
	}
	room := n * 7 / 10
	scale := float64(max(1, min(room/fw, room/fh)))
	if fw > room || fh > room {
		scale = min(float64(room)/float64(fw), float64(room)/float64(fh))
	}
	w, h := int(float64(fw)*scale), int(float64(fh)*scale)
	x0, y0 := (n-w)/2, (n-h)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := frame.At(fb.Min.X+int(float64(x)/scale), fb.Min.Y+int(float64(y)/scale))
			if _, _, _, a := c.RGBA(); a == 0 {
				continue
			}
			dst.Set(x0+x, y0+y, c)
		}
	}
	return dst
}

// exportDesktop is `gameviber export -desktop DIR OUT`: the game as a program
// for this system — the window player (gameviber-run) with the project packed
// onto its end; started with no arguments it plays that project. Nothing else
// is needed on the players' machines.
func exportDesktop(dir, out, target, variant string) error {
	p, err := gmfile.ReadDirVariant(dir, variant)
	if err != nil {
		return err
	}
	name := p.Name()
	if name == "" {
		name = filepath.Base(filepath.Clean(dir))
	}
	goos, goarch, ok := strings.Cut(target, "/")
	if !ok || !map[string]bool{"linux": true, "windows": true, "darwin": true}[goos] || !map[string]bool{"amd64": true, "arm64": true}[goarch] {
		return fmt.Errorf("-os %s: linux, windows or darwin / amd64 or arm64", target)
	}
	var exe []byte
	if files := goFiles(p); len(files) > 0 && os.Getenv("GAMEVIBER_INTERPRET") == "" {
		if exe, err = compiledPlayer(files, goos, goarch); err != nil {
			fmt.Printf("the game's Go runs interpreted (%v)\n", firstLine(err))
			exe = nil
		} else {
			fmt.Printf("compiled the game's Go in (%d files)\n", len(files))
		}
	}
	if exe != nil { // compiled above
	} else if goos == runtime.GOOS && goarch == runtime.GOARCH {
		player, err := runnerPath()
		if err != nil {
			return err
		}
		if exe, err = os.ReadFile(player); err != nil {
			return err
		}
	} else if exe, err = releasePlayer(goos, goarch); err != nil {
		return err
	}
	var tarBuf bytes.Buffer
	if err := packProjectVariant(dir, variant, &tarBuf); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	file := cacheKey(name)
	if goos == "windows" {
		file += ".exe"
	}
	var size [8]byte
	binary.LittleEndian.PutUint64(size[:], uint64(tarBuf.Len()))
	data := append(append(append(exe, tarBuf.Bytes()...), size[:]...), "GVPROJ01"...)
	path := filepath.Join(out, file)
	if goos == "darwin" { // a macOS application is a folder: Name.app/Contents/{Info.plist, MacOS/name}
		app := filepath.Join(out, name+".app", "Contents")
		if err := os.MkdirAll(filepath.Join(app, "MacOS"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(app, "Info.plist"), []byte(fmt.Sprintf(infoPlist, xmlText(name), file, "org.gameviber."+file)), 0o644); err != nil {
			return err
		}
		path = filepath.Join(app, "MacOS", file)
	}
	if err := os.WriteFile(path, data, 0o755); err != nil {
		return err
	}
	note := ""
	if goos == "darwin" {
		note = " — unsigned: macOS asks once (right-click → Open)"
	}
	fmt.Printf("exported %s to %s (%s/%s)%s\n", name, path, goos, goarch, note)
	return nil
}

// playersTag is the release the players for other systems come from (built
// by .github/workflows/players.yml); GAMEVIBER_PLAYERS_URL overrides where.
const playersTag = "players-latest"

// releasePlayer is gameviber-run for goos / goarch from the repo's release,
// kept in the user's cache after the first download.
func releasePlayer(goos, goarch string) ([]byte, error) {
	file := "gameviber-run-" + goos + "-" + goarch
	if goos == "windows" {
		file += ".exe"
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	base := os.Getenv("GAMEVIBER_PLAYERS_URL")
	if base == "" {
		base = "https://github.com/LaPingvino/gameviber/releases/download/" + playersTag
	}
	cached := filepath.Join(cacheDir, "gameviber", "players", cacheKey(base), file)
	if data, err := os.ReadFile(cached); err == nil {
		return data, nil
	}
	url := strings.TrimSuffix(base, "/") + "/" + file
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("the %s/%s player: %w", goos, goarch, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the %s/%s player: %s %s (the players release may not be built yet: .github/workflows/players.yml)", goos, goarch, url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if os.MkdirAll(filepath.Dir(cached), 0o755) == nil {
		os.WriteFile(cached, data, 0o755)
	}
	return data, nil
}

// xmlText escapes s for the plist.
func xmlText(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

const infoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>%s</string>
<key>CFBundleExecutable</key><string>%s</string>
<key>CFBundleIdentifier</key><string>%s</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
`

func firstLine(err error) string {
	s, _, _ := strings.Cut(err.Error(), "\n")
	return s
}
