//go:build !js

package player

import (
	"fmt"
	"os"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

const inBrowser = false

// open reads a GameMaker 5.3 executable or a GameViber project folder; with
// no path, the project appended to this program (an exported desktop game).
func open(path, variant string) (*gmfile.Project, error) {
	if path == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(exe)
		if err != nil {
			return nil, err
		}
		tarData, ok := appendedProject(data)
		if !ok {
			return nil, fmt.Errorf("usage: gameviber run [-room R] GAME.exe|DIR")
		}
		return projectFromTar(tarData)
	}
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return gmfile.ReadDirVariant(path, variant)
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
