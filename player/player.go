// Package player is GameViber's window player (Ebitengine) as a library:
// cmd/gameviber-run is a thin main around it, and an exported desktop game
// with Go game code compiled in is another (LOOP item 13, step 3).
package player

import (
	"flag"
	"fmt"
	"path/filepath"

	ebbackend "github.com/LaPingvino/gameviber/backend/ebiten"
	"github.com/LaPingvino/gameviber/load"
)

// Main plays a game in a window from command-line arguments:
//
//	[-room R] [-frames N] [-mute] [-variant V] [GAME.exe|DIR]
//
// With no path, the project appended to the running program (an exported
// desktop game). The exported game's own main — the window player with the
// game's Go code compiled in — calls it too.
func Main(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	room := fs.String("room", "", "start room")
	frames := fs.Int("frames", 0, "stop after this many frames (0: until the window closes)")
	mute := fs.Bool("mute", false, "no sound")
	variant := fs.String("variant", "", "a build variant: NAME.toml merged over game.toml")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 && !inBrowser { // none: the project appended to this program (open)
		return fmt.Errorf("usage: gameviber run [-room R] GAME.exe|DIR")
	}
	p, err := open(fs.Arg(0), *variant)
	if err != nil {
		return err
	}
	g, err := load.FromProject(p)
	if err != nil {
		return err
	}
	ebbackend.Mute = *mute
	ebbackend.Attach(p, g.Game) // before Start: the first create events may play or ask
	var pl *load.Play
	if *room != "" {
		pl, err = g.StartAt(*room)
	} else {
		pl, err = g.Start()
	}
	if err != nil {
		return err
	}
	w := ebbackend.New(p, pl)
	w.MaxFrames = *frames
	if err := w.Run("GameViber — " + filepath.Base(fs.Arg(0)+p.Name())); err != nil {
		return err
	}
	fmt.Printf("%d frames; room %s, score %d, lives %d\n", w.Frames(), pl.Room.Name, pl.Globals.Score, pl.Globals.Lives)
	for _, e := range g.Errors {
		fmt.Println("error:", e)
	}
	return nil
}
