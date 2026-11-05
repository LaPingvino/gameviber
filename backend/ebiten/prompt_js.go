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
