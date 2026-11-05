package main

import (
	"archive/zip"
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
