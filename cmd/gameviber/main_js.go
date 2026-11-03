//go:build js

package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall/js"
)

// The editor in the browser (the GitHub Pages build, `gameviber editor-web`):
// the same editor — the page's requests answered here instead of by a
// server — over a project in the page's file system (web/editor/fsshim.js,
// which keeps it in localStorage). The project starts as the starter game.
func main() {
	dir := "/w/project"
	if _, err := os.Stat(filepath.Join(dir, "game.toml")); err != nil {
		os.RemoveAll(dir)
		if err := cmdNew([]string{dir}); err != nil {
			js.Global().Get("console").Call("error", "gameviber: "+err.Error())
		}
	}
	h := editHandler(dir)
	// gameviberAPI(method, path, body) → a Promise of {status, type, body}
	// (body: text, or a Uint8Array for binary answers).
	js.Global().Set("gameviberAPI", js.FuncOf(func(this js.Value, args []js.Value) any {
		method, path := args[0].String(), args[1].String()
		var body []byte
		if len(args) > 2 && args[2].Type() == js.TypeString {
			body = []byte(args[2].String())
		}
		return js.Global().Get("Promise").New(js.FuncOf(func(this js.Value, pr []js.Value) any {
			resolve := pr[0]
			go func() { // after the call returns: the file system's callbacks need the event loop free
				req := httptest.NewRequest(method, "/"+path, bytes.NewReader(body))
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				res := rec.Result()
				data, _ := io.ReadAll(res.Body)
				out := js.Global().Get("Object").New()
				out.Set("status", res.StatusCode)
				out.Set("type", res.Header.Get("Content-Type"))
				if ct := res.Header.Get("Content-Type"); ct == "application/x-tar" || ct == "image/png" {
					u := js.Global().Get("Uint8Array").New(len(data))
					js.CopyBytesToJS(u, data)
					out.Set("body", u)
				} else {
					out.Set("body", string(data))
				}
				resolve.Invoke(out)
			}()
			return nil
		}))
	}))
	js.Global().Set("gameviberReset", js.FuncOf(func(this js.Value, args []js.Value) any {
		go func() {
			os.RemoveAll(dir)
			cmdNew([]string{dir})
			js.Global().Get("location").Call("reload")
		}()
		return nil
	}))
	if ready := js.Global().Get("gameviberReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	_ = http.StatusOK
	select {}
}
