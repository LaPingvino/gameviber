//go:build !js

package ebiten

import (
	"log"
	"os/exec"
	"runtime"
)

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

// openURL is url_open: the desktop's browser.
func openURL(u string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		c = exec.Command("open", u)
	default:
		c = exec.Command("xdg-open", u)
	}
	if err := c.Start(); err != nil {
		log.Printf("url_open %s: %v", u, err)
	}
}
