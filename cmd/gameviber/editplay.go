package main

import (
	"regexp"

	"fmt"
	"github.com/LaPingvino/gameviber/import/gmfile"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Play inside the editor (docs/EDITOR.md, step E3): the browser player built
// once (on the first Play), the project packed fresh for every Play — so it
// runs what was last saved.

type editPlay struct {
	dir  string
	once sync.Once
	out  string // the built player (game.wasm, wasm_exec.js)
	err  error
}

func (p *editPlay) build() {
	p.once.Do(func() {
		p.out, p.err = os.MkdirTemp("", "gameviber-player")
		if p.err == nil {
			p.err = buildPlayer(p.out)
		}
	})
}

const playPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>Play</title>
<style>html,body{margin:0;height:100%%;background:#111;color:#ddd;font:14px system-ui,sans-serif}
#msg{position:fixed;inset:auto 0 8px 0;text-align:center;opacity:.7}</style></head>
<body><div id="msg">building and loading…</div>
<script src="wasm_exec.js"></script>
<script>
window.gameviberProject = "project.tar?t=%d" + (new URLSearchParams(location.search).get("variant") ? "&variant=" + encodeURIComponent(new URLSearchParams(location.search).get("variant")) : "");
const go = new Go();
WebAssembly.instantiateStreaming(fetch("game.wasm"), go.importObject)
  .then(r => { document.getElementById("msg").remove(); go.run(r.instance); })
  .catch(e => { document.getElementById("msg").textContent = "could not start: " + e; });
</script></body></html>
`

// variantName: "" (none) or a variant file's name.
var variantName = regexp.MustCompile(`^[A-Za-z0-9_-]*$`)

func (p *editPlay) handle(mux *http.ServeMux) {
	mux.HandleFunc("GET /play/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprintf(w, playPage, time.Now().UnixNano())
	})
	mux.HandleFunc("GET /play/project.tar", func(w http.ResponseWriter, r *http.Request) {
		variant := r.URL.Query().Get("variant") // a build variant (LOOP 6b): NAME.toml merged over game.toml
		if !variantName.MatchString(variant) {
			http.Error(w, "variant: a name of letters, digits, - and _", http.StatusBadRequest)
			return
		}
		if _, err := gmfile.VariantGameTOML(p.dir, variant); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("Cache-Control", "no-store")
		if err := packProjectVariant(p.dir, variant, w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	for _, f := range []string{"game.wasm", "wasm_exec.js"} {
		mux.HandleFunc("GET /play/"+f, func(w http.ResponseWriter, r *http.Request) {
			p.build()
			if p.err != nil {
				http.Error(w, "the player could not be built: "+p.err.Error(), http.StatusInternalServerError)
				return
			}
			http.ServeFile(w, r, filepath.Join(p.out, f))
		})
	}
}
