// Command gameviber-run plays a game in a window (Ebitengine): the arrow
// keys, space, enter. gameviber run starts it.
//
//	gameviber-run [-room R] [-frames N] GAME.exe|DIR
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	ebbackend "github.com/LaPingvino/gameviber/backend/ebiten"
	"github.com/LaPingvino/gameviber/load"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gameviber-run:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	room := fs.String("room", "", "start room")
	frames := fs.Int("frames", 0, "stop after this many frames (0: until the window closes)")
	mute := fs.Bool("mute", false, "no sound")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 && !inBrowser {
		return fmt.Errorf("usage: gameviber run [-room R] GAME.exe|DIR")
	}
	p, err := open(fs.Arg(0))
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
