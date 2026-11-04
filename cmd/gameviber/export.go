package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

// cmdExport is `gameviber export DIR OUT` (LOOP item 6): the finished web
// build — the player, the project and an installable-app layer (a manifest
// with the game's name, opening full screen; icons; the offline service
// worker) — as a folder, or as a zip with index.html at its root (itch.io's
// HTML5 upload, or any static host) when OUT ends in .zip.
func cmdExport(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: gameviber export DIR OUT   (OUT: a folder, or name.zip for itch.io)")
	}
	dir, out := args[0], args[1]
	p, err := gmfile.ReadDir(dir)
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
	if _, err := webBuild(dir, build); err != nil {
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
