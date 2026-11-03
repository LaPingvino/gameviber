//go:build js

package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"syscall/js"
	"testing/fstest"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

const inBrowser = true

// open, in the browser: the project packed next to the page (project.tar,
// made by gameviber web), read into memory.
func open(string) (*gmfile.Project, error) {
	base, err := url.Parse(js.Global().Get("location").Get("href").String())
	if err != nil {
		return nil, err
	}
	u := base.ResolveReference(&url.URL{Path: "project.tar"})
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
	fsys := fstest.MapFS{}
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		fsys[h.Name] = &fstest.MapFile{Data: b}
	}
	return gmfile.ReadFS(fsys, ".")
}
