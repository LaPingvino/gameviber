//go:build js

package ebiten

import (
	"github.com/LaPingvino/gameviber/internal/webfiles"
	"github.com/LaPingvino/gameviber/runner"
)

// gameFiles is where the file_* functions read and write in the browser: the
// page's localStorage, per game.
func gameFiles(p *gmfileProject) runner.FileStore { return webfiles.New(p.GameID) }
