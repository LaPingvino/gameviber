// Command gameviber-run plays a game in a window (Ebitengine): the arrow
// keys, space, enter. gameviber run starts it.
//
//	gameviber-run [-room R] [-frames N] GAME.exe|DIR
package main

import (
	"fmt"
	"os"

	"github.com/LaPingvino/gameviber/player"
)

func main() {
	if err := player.Main(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gameviber-run:", err)
		os.Exit(1)
	}
}
