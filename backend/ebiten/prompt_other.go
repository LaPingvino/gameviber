//go:build !js

package ebiten

import "log"

// On the desktop there is no dialog yet (it needs text drawing): get_string
// takes the default, show_message is printed. PARTIAL.
// hasDialogs: get_string can ask the player here.
const hasDialogs = false

func askString(question, def string) string { return def }

func showMessage(m string) { log.Printf("%s", m) }
