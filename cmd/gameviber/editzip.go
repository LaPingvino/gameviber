package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Download: the whole project as a zip (GET api/project.zip) — a game made in
// the browser editor taken out of the browser, to keep, share, or use with
// `gameviber` on a computer. The files sit in a folder named after the game;
// dot folders (.git …) and this machine's agent configs are left out, as in
// the played project.

func zipHandlers(dir string, mux *http.ServeMux) {
	mux.HandleFunc("GET /api/project.zip", func(w http.ResponseWriter, r *http.Request) {
		name := projectFolderName(dir)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.zip"`)
		if err := zipProject(dir, name, w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

// projectFolderName is the game's name (game.toml) made a safe folder name,
// else the project folder's own name.
func projectFolderName(dir string) string {
	var g struct {
		Name string `toml:"name"`
	}
	toml.DecodeFile(filepath.Join(dir, "game.toml"), &g)
	n := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, g.Name)
	if n == "" {
		n = filepath.Base(dir)
	}
	return n
}

func zipProject(dir, folder string, w io.Writer) error {
	zw := zip.NewWriter(w)
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if agentConfigFiles[d.Name()] {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		f, err := zw.Create(folder + "/" + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	})
	if err != nil {
		return err
	}
	return zw.Close()
}

// unpackProjectZip writes a project zip (Download's, or any with a game.toml
// at its top or in one folder) into dir, which must not exist yet. Paths
// leaving the project, and archives over 256 MB unpacked, are refused.
func unpackProjectZip(data []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("not a zip: %w", err)
	}
	prefix := ""
	found := false
	for _, f := range zr.File {
		n := strings.TrimPrefix(f.Name, "./")
		if n == "game.toml" {
			prefix, found = "", true
			break
		}
		if parts := strings.Split(n, "/"); len(parts) == 2 && parts[1] == "game.toml" {
			prefix, found = parts[0]+"/", true
		}
	}
	if !found {
		return fmt.Errorf("no game.toml in the zip: not a GameViber project")
	}
	var total uint64
	for _, f := range zr.File {
		n := strings.TrimPrefix(f.Name, "./")
		if !strings.HasPrefix(n, prefix) || strings.HasSuffix(n, "/") {
			continue
		}
		rel := strings.TrimPrefix(n, prefix)
		clean := filepath.Clean(filepath.FromSlash(rel))
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("%s: outside the project", f.Name)
		}
		if total += f.UncompressedSize64; total > 256<<20 {
			return fmt.Errorf("the zip unpacks to more than 256 MB")
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		b, err := io.ReadAll(io.LimitReader(rc, 256<<20))
		rc.Close()
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(dir, clean), b); err != nil {
			return err
		}
	}
	return nil
}
