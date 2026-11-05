//go:build !js

package ebiten

import "github.com/LaPingvino/gameviber/runner"

// gameFiles: on the desktop the file_* functions use the working directory
// (runner's default).
func gameFiles(p *gmfileProject) runner.FileStore { return nil }
