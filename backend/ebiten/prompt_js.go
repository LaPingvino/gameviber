//go:build js

package ebiten

import "syscall/js"

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
func openURL(u string) {
	w := js.Global().Call("open", u, "_blank")
	if w.IsNull() || w.IsUndefined() {
		js.Global().Get("location").Call("assign", u)
		return
	}
	w.Set("opener", js.Null())
}
