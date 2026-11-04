package main

import (
	"archive/zip"
	"fmt"
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
