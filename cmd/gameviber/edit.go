package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// gameviber edit DIR — the editor (docs/EDITOR.md, step E1; Joop 2026-11-04:
// decide and build, correct later): a page served on this machine only
// (127.0.0.1) that edits the project's own files — the resource tree, each
// object's events as text, saved back to the files, gameviber check after
// every save. HTML around the editing (native text boxes, copy and paste);
// the game itself will run inside it later (E3).

//go:embed edit.html
var editPage []byte

func cmdEdit(args []string) error {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8765", "where to serve the editor (this machine only by default)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: gameviber edit [-addr host:port] DIR")
	}
	dir, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "game.toml")); err != nil {
		return fmt.Errorf("%s is not a GameViber project (no game.toml): gameviber new or gameviber import first", dir)
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	fmt.Printf("editing %s at http://%s/ (Ctrl+C to stop)\n", dir, ln.Addr())
	return http.Serve(ln, editHandler(dir))
}

// editHandler serves the page and its API over the project folder dir.
func editHandler(dir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(editPage)
	})
	// The tree: every editable text file (.gml, .toml, .txt, .md), by path.
	mux.HandleFunc("GET /api/files", func(w http.ResponseWriter, r *http.Request) {
		var files []string
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch filepath.Ext(p) {
			case ".gml", ".toml", ".txt", ".md":
				rel, _ := filepath.Rel(dir, p)
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		sort.Strings(files)
		var sounds []string // the sounds (files, not text: shown, played, imported)
		ents, _ := os.ReadDir(filepath.Join(dir, "sounds"))
		for _, e := range ents {
			if soundTypes[strings.ToLower(filepath.Ext(e.Name()))] != "" {
				sounds = append(sounds, e.Name())
			}
		}
		writeJSON(w, map[string]any{"name": filepath.Base(dir), "files": files, "sounds": sounds})
	})
	mux.HandleFunc("GET /api/file", func(w http.ResponseWriter, r *http.Request) {
		p, ok := projectPath(dir, r.URL.Query().Get("path"))
		if !ok {
			http.Error(w, "not a file of this project", http.StatusBadRequest)
			return
		}
		data, err := os.ReadFile(p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	})
	// Saving a file (new event files too), then the check's report.
	mux.HandleFunc("PUT /api/file", func(w http.ResponseWriter, r *http.Request) {
		p, ok := projectPath(dir, r.URL.Query().Get("path"))
		if !ok || !editable(p) {
			http.Error(w, "not an editable file of this project", http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, checkReport(dir))
	})
	(&editPlay{dir: dir}).handle(mux)
	rooms := editRooms{dir}
	mux.HandleFunc("GET /api/room", rooms.room)
	mux.HandleFunc("PUT /api/room", rooms.saveRoom)
	mux.HandleFunc("PUT /api/room-settings", rooms.saveRoomSettings)
	mux.HandleFunc("GET /api/strip", rooms.strip)
	mux.HandleFunc("GET /api/sprite", rooms.sprite)
	mux.HandleFunc("PUT /api/sprite", rooms.saveSprite)
	mux.HandleFunc("POST /api/sprite-png", rooms.importPNG)
	mux.HandleFunc("POST /api/new", rooms.newThing)
	mux.HandleFunc("GET /api/object", rooms.object)
	mux.HandleFunc("PUT /api/object", rooms.saveObject)
	mux.HandleFunc("GET /api/sound", func(w http.ResponseWriter, r *http.Request) {
		f := r.URL.Query().Get("file")
		typ := soundTypes[strings.ToLower(filepath.Ext(f))]
		if typ == "" || strings.ContainsAny(f, `/\`) {
			http.Error(w, "no such sound", http.StatusBadRequest)
			return
		}
		data, err := os.ReadFile(filepath.Join(dir, "sounds", f))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", typ)
		w.Write(data)
	})
	// A sound added or replaced: sounds/<name><ext> (WAV, Ogg / Opus, MP3, MIDI).
	mux.HandleFunc("POST /api/sound", func(w http.ResponseWriter, r *http.Request) {
		name, ext := r.URL.Query().Get("name"), strings.ToLower(r.URL.Query().Get("ext"))
		if name == "" || strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789_") != "" || soundTypes[ext] == "" {
			http.Error(w, "a sound's name is a–z, 0–9, _; the file WAV, Ogg, Opus, MP3 or MIDI", http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(filepath.Join(dir, "sounds"), 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(filepath.Join(dir, "sounds", name+ext), data, 0o644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, checkReport(dir))
	})
	mux.HandleFunc("GET /api/check", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, checkReport(dir))
	})
	return mux
}

// projectPath is rel inside the project, or false when it would leave it.
func projectPath(dir, rel string) (string, bool) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", false
	}
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if p != dir && !strings.HasPrefix(p, dir+string(filepath.Separator)) {
		return "", false
	}
	return p, true
}

func editable(p string) bool {
	switch filepath.Ext(p) {
	case ".gml", ".toml", ".txt", ".md":
		return true
	}
	return false
}

// checkReport is gameviber check's report, its lines made relative to dir.
func checkReport(dir string) map[string]any {
	var b bytes.Buffer
	problems, err := checkProject(dir, &b)
	report := strings.ReplaceAll(b.String(), dir+string(filepath.Separator), "")
	if err != nil {
		report += err.Error() + "\n"
		problems++
	}
	return map[string]any{"problems": problems, "report": report}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// soundTypes are the sound files the editor shows, by extension.
var soundTypes = map[string]string{".wav": "audio/wav", ".ogg": "audio/ogg", ".opus": "audio/ogg", ".mp3": "audio/mpeg", ".mid": "audio/midi"}
