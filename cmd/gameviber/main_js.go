//go:build js

package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		} else if len(args) > 2 && args[2].InstanceOf(js.Global().Get("Uint8Array")) { // a picture
			body = make([]byte, args[2].Get("length").Int())
			js.CopyBytesToGo(body, args[2])
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
				out.Set("disposition", res.Header.Get("Content-Disposition"))
				if ct := res.Header.Get("Content-Type"); ct == "application/x-tar" || ct == "application/zip" || ct == "image/png" || strings.HasPrefix(ct, "audio/") {
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
	// gameviberReset([template]): the project replaced by a new game from a
	// starting point (gameviber new -template; the starter without one).
	js.Global().Set("gameviberReset", js.FuncOf(func(this js.Value, args []js.Value) any {
		tmpl := "coins"
		if len(args) > 0 && args[0].Type() == js.TypeString {
			tmpl = args[0].String()
		}
		go func() {
			os.RemoveAll(dir)
			if err := cmdNew([]string{"-template", tmpl, dir}); err != nil {
				js.Global().Get("console").Call("error", "gameviber: "+err.Error())
				cmdNew([]string{dir})
			}
			js.Global().Get("location").Call("reload")
		}()
		return nil
	}))
	// gameviberOpenZip(bytes) → a Promise of "" or the error: the project
	// replaced by a downloaded GameViber zip (checked before anything goes).
	js.Global().Set("gameviberOpenZip", js.FuncOf(func(this js.Value, args []js.Value) any {
		data := make([]byte, args[0].Get("length").Int())
		js.CopyBytesToGo(data, args[0])
		return js.Global().Get("Promise").New(js.FuncOf(func(this js.Value, pr []js.Value) any {
			resolve := pr[0]
			go func() {
				tmp := dir + ".new"
				os.RemoveAll(tmp)
				if err := unpackProjectZip(data, tmp); err != nil {
					os.RemoveAll(tmp)
					resolve.Invoke(err.Error())
					return
				}
				// checked: now the real place (the page's file system can't
				// rename folders, so the zip is unpacked again rather than moved)
				os.RemoveAll(tmp)
				os.RemoveAll(dir)
				if err := unpackProjectZip(data, dir); err != nil {
					os.RemoveAll(dir)
					cmdNew([]string{dir}) // never leave the editor without a project
					resolve.Invoke(err.Error())
					return
				}
				resolve.Invoke("")
				js.Global().Get("location").Call("reload")
			}()
			return nil
		}))
	}))
	if ready := js.Global().Get("gameviberReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	_ = http.StatusOK
	select {}
}
