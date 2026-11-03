//go:build !js

package main

import (
	"fmt"
	"os"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

const inBrowser = false

// open reads a GameMaker 5.3 executable or a GameViber project folder.
func open(path string) (*gmfile.Project, error) {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return gmfile.ReadDir(path)
	}
	d, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	e, err := gmfile.OpenExe(d)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return e.ReadProject()
}
