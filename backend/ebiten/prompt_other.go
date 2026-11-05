//go:build !js

package ebiten

import "log"

// On the desktop there is no dialog yet (it needs text drawing): get_string
// takes the default, show_message is printed. PARTIAL.
// hasDialogs: get_string can ask the player here.
const hasDialogs = false

func askString(question, def string) string { return def }

func showMessage(m string) { log.Printf("%s", m) }

// askQuestion: no dialog on the desktop yet — the answer is no (PARTIAL).
func askQuestion(q string) bool {
	log.Printf("show_question (answered no: no dialog yet): %s", q)
	return false
}
