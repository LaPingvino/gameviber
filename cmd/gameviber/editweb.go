package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gameviber editor-web OUT — the editor as a static page (the GitHub Pages
// build): the editor compiled to WebAssembly answers the page's requests
// itself (main_js.go), the project kept in the browser (web/editor/fsshim.js);
// Play hands the packed project to the browser player. Run inside the
// GameViber source (it builds with go).
func cmdEditorWeb(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gameviber editor-web OUT")
	}
	out := args[0]
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	build := exec.Command("go", "build", "-o", filepath.Join(out, "editor.wasm"), "./cmd/gameviber")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("building the editor's wasm (run inside the GameViber source): %w", err)
	}
	if err := buildPlayer(out); err != nil { // game.wasm, wasm_exec.js
		return err
	}
	shim, err := os.ReadFile(filepath.Join("web", "editor", "fsshim.js"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "fsshim.js"), shim, 0o644); err != nil {
		return err
	}
	page := strings.Replace(string(editPage), "<script>", editorPrelude+"<script>", 1)
	if err := os.WriteFile(filepath.Join(out, "index.html"), []byte(page), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "play.html"), []byte(staticPlayPage), 0o644); err != nil {
		return err
	}
	fmt.Printf("the editor as a static page in %s (index.html, editor.wasm, game.wasm, fsshim.js, wasm_exec.js, play.html)\n", out)
	return nil
}

// editorPrelude starts the editor's wasm before the page's own script; the
// file system first (wasm_exec.js keeps a globalThis.fs that is there).
const editorPrelude = `<script src="fsshim.js"></script>
<script src="wasm_exec.js"></script>
<script>
window.gameviberStatic = true;
window.gameviberWait = new Promise((ready) => { window.gameviberReady = ready; });
(() => {
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch("editor.wasm"), go.importObject).then((r) => go.run(r.instance))
    .catch((e) => { document.getElementById("check").textContent = "the editor could not start: " + e; });
})();
</script>
`

// staticPlayPage plays the project the editor hands it (?project=<blob URL>).
const staticPlayPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>Play</title>
<style>html,body{margin:0;height:100%;background:#111;color:#ddd;font:14px system-ui,sans-serif}
#msg{position:fixed;inset:auto 0 8px 0;text-align:center;opacity:.7}</style></head>
<body><div id="msg">loading…</div>
<script src="wasm_exec.js"></script>
<script>
window.gameviberProject = new URLSearchParams(location.search).get("project") || "project.tar";
const go = new Go();
WebAssembly.instantiateStreaming(fetch("game.wasm"), go.importObject)
  .then(r => { document.getElementById("msg").remove(); go.run(r.instance); })
  .catch(e => { document.getElementById("msg").textContent = "could not start: " + e; });
</script></body></html>
`
