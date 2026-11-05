package main

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	bridge := fs.Bool("bridge", false, "also let the online editor (-origin) use this project and its agents, paired by the printed link")
	origin := fs.String("origin", pagesOrigin, "the online editor's origin, for -bridge")
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
	token := newToken()
	var h http.Handler = guardLocal(editHandler(dir), token)
	if *bridge {
		_, port, _ := net.SplitHostPort(ln.Addr().String())
		fmt.Printf("the online editor, connected to this project (keep the link private — it holds the pairing token):\n  %s#bridge=%s:%s\n", *origin+pagesEditorPath, port, token)
		h = bridgeGuard(h, token, *origin)
	}
	return http.Serve(ln, h)
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
			if err != nil {
				return err
			}
			if d.IsDir() {
				if p != dir && strings.HasPrefix(d.Name(), ".") { // .git, .gemini …: not the game's
					return filepath.SkipDir
				}
				return nil
			}
			switch filepath.Ext(p) {
			case ".gml", ".js", ".lua", ".toml", ".txt", ".md":
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
	// the assistant's tools (LOOP item 11): search / replace edits, ranged reads, find, the outline
	mux.HandleFunc("POST /api/edit", func(w http.ResponseWriter, r *http.Request) {
		blocks, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rep, err := editFile(dir, r.URL.Query().Get("path"), string(blocks))
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(w, rep)
	})
	mux.HandleFunc("GET /api/read", func(w http.ResponseWriter, r *http.Request) {
		out, err := readRef(dir, r.URL.Query().Get("ref"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		io.WriteString(w, out)
	})
	mux.HandleFunc("GET /api/find", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, Find(dir, r.URL.Query().Get("q"), 60)) })
	mux.HandleFunc("GET /api/outline", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, Outline(dir)) })
	(&editPlay{dir: dir}).handle(mux)
	(&editTerm{dir: dir}).handle(mux)
	(&editAsk{dir: dir}).handle(mux)
	rooms := editRooms{dir}
	mux.HandleFunc("GET /api/room", rooms.room)
	mux.HandleFunc("PUT /api/room", rooms.saveRoom)
	mux.HandleFunc("PUT /api/room-settings", rooms.saveRoomSettings)
	mux.HandleFunc("GET /api/strip", rooms.strip)
	mux.HandleFunc("GET /api/sprite", rooms.sprite)
	mux.HandleFunc("PUT /api/sprite", rooms.saveSprite)
	mux.HandleFunc("POST /api/sprite-png", rooms.importPNG)
	mux.HandleFunc("POST /api/background-png", rooms.importBackground)
	sfxHandlers(dir, mux)
	llmHandlers(mux)
	zipHandlers(dir, mux)
	namesHandlers(dir, mux)
	mux.HandleFunc("GET /api/background", rooms.background)
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
	// play headless (the MCP play tool's run): steps (default 300), keys (the
	// key script), room; the state after, and the last frame as a base64 PNG.
	mux.HandleFunc("GET /api/play", func(w http.ResponseWriter, r *http.Request) {
		steps := 300
		if s := r.URL.Query().Get("steps"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 || n > 100000 {
				http.Error(w, "steps: a number from 0 to 100000", http.StatusBadRequest)
				return
			}
			steps = n
		}
		var shot bytes.Buffer
		state, err := playProjectOpts(dir, playOpts{Room: r.URL.Query().Get("room"), Steps: steps, Keys: r.URL.Query().Get("keys"), Shot: &shot, Expect: r.URL.Query().Get("expect")})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state["steps"] = steps
		state["shot"] = base64.StdEncoding.EncodeToString(shot.Bytes())
		writeJSON(w, state)
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
	case ".gml", ".js", ".lua", ".toml", ".txt", ".md": // .js: events and scripts in JavaScript
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

// newToken is a random per-run secret for guardLocal.
func newToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// guardLocal keeps other websites out of the editor on this machine: the
// request's Host must be this machine's (no DNS rebinding), and anything that
// changes something (not GET) must carry the per-run token — which the
// served page holds (a meta tag) and other sites can't read or send (a custom
// header needs a CORS permission this server never gives). It matters since
// the terminal panel runs commands.
func guardLocal(h http.Handler, token string) http.Handler {
	page := bytes.Replace(editPage, []byte("<head>"), []byte(`<head><meta name="gameviber-token" content="`+token+`">`), 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "127.0.0.1" && host != "localhost" && host != "::1" && host != "[::1]" {
			http.Error(w, "the editor answers this machine only", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(page)
			return
		}
		if r.Method != http.MethodGet && r.Header.Get("X-GameViber-Token") != token {
			http.Error(w, "missing the editor's token", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// The online editor (GitHub Pages) and its path there.
const (
	pagesOrigin     = "https://lapingvino.github.io"
	pagesEditorPath = "/gameviber/editor/"
)

// bridgeGuard lets the online editor at origin use this server (LOOP item 5f):
// CORS for that one origin, Chrome's private-network preflight (an https page
// calling localhost), and the pairing token on every request from it — reads
// too, since the project must not be readable by any other page. Event streams
// carry the token as ?gvtoken= (EventSource can't send headers).
func bridgeGuard(h http.Handler, token, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != origin {
			h.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "X-GameViber-Token, Content-Type")
			if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
				w.Header().Set("Access-Control-Allow-Private-Network", "true")
			}
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		got := r.Header.Get("X-GameViber-Token")
		if got == "" {
			got = r.URL.Query().Get("gvtoken")
		}
		if got != token {
			http.Error(w, "not paired: open the link gameviber edit -bridge printed", http.StatusForbidden)
			return
		}
		r.Header.Set("X-GameViber-Token", token) // guardLocal's check, for the query-string form
		h.ServeHTTP(w, r)
	})
}
