package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/LaPingvino/gameviber/sfx"
)

// The editor's sound effects (LOOP item 9): a preset's settings, the settings
// rendered as WAV to play, and saving — sounds/NAME.wav for the game, the
// settings beside it in sfx/NAME.toml so the sound can be opened again.
func sfxHandlers(dir string, mux *http.ServeMux) {
	mux.HandleFunc("POST /api/sfx", func(w http.ResponseWriter, r *http.Request) {
		kind := r.URL.Query().Get("preset")
		if !slices.Contains(sfx.Presets, kind) {
			http.Error(w, "preset: one of the kinds", http.StatusBadRequest)
			return
		}
		seed, _ := strconv.ParseUint(r.URL.Query().Get("seed"), 10, 64)
		writeJSON(w, sfx.Preset(kind, seed))
	})
	mux.HandleFunc("GET /api/sfx", func(w http.ResponseWriter, r *http.Request) { // a saved one's settings
		name := r.URL.Query().Get("name")
		if !safeName(name) {
			http.Error(w, "not a name", http.StatusBadRequest)
			return
		}
		var p sfx.Params
		if _, err := toml.DecodeFile(filepath.Join(dir, "sfx", name+".toml"), &p); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, p)
	})
	params := func(w http.ResponseWriter, r *http.Request) (sfx.Params, bool) {
		var p sfx.Params
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return p, false
		}
		return p, true
	}
	mux.HandleFunc("POST /api/sfx-wav", func(w http.ResponseWriter, r *http.Request) {
		p, ok := params(w, r)
		if !ok {
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(sfx.WAV(sfx.Generate(p)))
	})
	mux.HandleFunc("POST /api/sfx-save", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if !safeName(name) {
			http.Error(w, "a sound name is letters, digits, - and _", http.StatusBadRequest)
			return
		}
		p, ok := params(w, r)
		if !ok {
			return
		}
		var t bytes.Buffer
		if err := toml.NewEncoder(&t).Encode(p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, f := range []struct {
			path string
			data []byte
		}{{filepath.Join(dir, "sounds", name+".wav"), sfx.WAV(sfx.Generate(p))}, {filepath.Join(dir, "sfx", name+".toml"), t.Bytes()}} {
			if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := os.WriteFile(f.path, f.data, 0o644); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		writeJSON(w, checkReport(dir))
	})
}
