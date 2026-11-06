//go:build js

package player

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"syscall/js"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

const inBrowser = true

// open, in the browser: the project packed next to the page (project.tar,
// made by gameviber web), read into memory.
func open(string, string) (*gmfile.Project, error) {
	base, err := url.Parse(js.Global().Get("location").Get("href").String())
	if err != nil {
		return nil, err
	}
	ref := "project.tar" // the page names it with its content hash (a fresh copy after a deploy)
	if v := js.Global().Get("gameviberProject"); v.Type() == js.TypeString {
		ref = v.String()
	}
	rel, err := url.Parse(ref)
	if err != nil {
		return nil, err
	}
	u := base.ResolveReference(rel)
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", u, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return projectFromTar(data)
}
