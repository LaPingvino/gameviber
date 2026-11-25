//go:build js

package ebiten

import (
	"syscall/js"

	"github.com/LaPingvino/gameviber/runner"
)

// pageURL is page_url: the page's own address.
func pageURL() string { return js.Global().Get("location").Get("href").String() }

// httpDo is the HTTP transport (runner http.go): a fetch, same-origin
// cookies with it; done gets the status and text, or status 0 on failure.
func httpDo(req runner.HTTPRequest, done func(runner.HTTPResponse)) {
	init := js.Global().Get("Object").New()
	init.Set("method", req.Method)
	init.Set("credentials", "same-origin")
	if req.Method == "POST" {
		headers := js.Global().Get("Object").New()
		headers.Set("Content-Type", "application/x-www-form-urlencoded")
		init.Set("headers", headers)
		init.Set("body", req.Body)
	}
	var onResp, onText, onFail js.Func
	status := 0
	release := func() { onResp.Release(); onText.Release(); onFail.Release() }
	onText = js.FuncOf(func(this js.Value, args []js.Value) any {
		done(runner.HTTPResponse{Status: status, Body: args[0].String()})
		release()
		return nil
	})
	onResp = js.FuncOf(func(this js.Value, args []js.Value) any {
		status = args[0].Get("status").Int()
		return args[0].Call("text")
	})
	onFail = js.FuncOf(func(this js.Value, args []js.Value) any {
		done(runner.HTTPResponse{})
		release()
		return nil
	})
	js.Global().Call("fetch", req.URL, init).Call("then", onResp).Call("then", onText).Call("catch", onFail)
}

// In the browser get_string and show_message use the page's own dialogs
// (a phone shows its keyboard for the prompt). Cancel / Esc gives the
// default — ASSUMPTION [engine]: GM 5's dialog returns it unchanged then.
// hasDialogs: get_string can ask the player here.
const hasDialogs = true

func askString(question, def string) string {
	v := js.Global().Call("prompt", question, def)
	if v.Type() != js.TypeString {
		return def
	}
	return v.String()
}

func showMessage(m string) { js.Global().Call("alert", m) }

// askQuestion is show_question: the page's confirm (OK = yes).
func askQuestion(q string) bool { return js.Global().Call("confirm", q).Bool() }

// openURL is url_open: a new tab (relative addresses are the page's), cut
// off from the game's page (opener cleared; "noopener" can't be asked for in
// open() itself, as it makes open() return null even when the tab opens). A
// browser that blocks the new tab (the click was too long ago) gets the page
// itself sent there instead.
func openURL(u, target string) {
	if target != "" && target != "_blank" { // "_self", "_parent", "_top", a named window
		js.Global().Call("open", u, target)
		return
	}
	w := js.Global().Call("open", u, "_blank")
	if w.IsNull() || w.IsUndefined() {
		js.Global().Get("location").Call("assign", u)
		return
	}
	w.Set("opener", js.Null())
}
