//go:build !js

package ebiten

import (
	"log"

	"os/exec"
	"runtime"

	"github.com/LaPingvino/gameviber/runner"
)

// On the desktop there is no dialog yet (it needs text drawing): get_string
// takes the default, show_message is printed. PARTIAL.
// hasDialogs: get_string can ask the player here.
const hasDialogs = false

// pageURL: the desktop has no page. httpDo: nil — the runner's own net/http.
func pageURL() string { return "" }

var httpDo func(runner.HTTPRequest, func(runner.HTTPResponse))

func askString(question, def string) string { return def }

func showMessage(m string) { log.Printf("%s", m) }

// askQuestion: no dialog on the desktop yet — the answer is no (PARTIAL).
func askQuestion(q string) bool {
	log.Printf("show_question (answered no: no dialog yet): %s", q)
	return false
}

// openURL is url_open / url_open_ext: the desktop's browser (the target
// doesn't apply).
func openURL(u, target string) {
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
