// Command gameviber is GameViber's command line (docs/DESIGN.md §3), for
// people and LLM agents alike: plain output, errors with places, headless
// runs that can be checked without watching.
//
//	gameviber import GAME.exe DIR       a GameMaker 5.3 game → GameViber's text project
//	gameviber play [flags] GAME.exe     run it (headless): steps, keys, a screenshot, the state
//
// play takes a GameMaker 5.3 executable or a GameViber project folder.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/LaPingvino/gameviber/gml"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LaPingvino/gameviber/import/gmfile"
	"github.com/LaPingvino/gameviber/load"
	"github.com/LaPingvino/gameviber/render"
	"github.com/LaPingvino/gameviber/runner"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "import":
		err = cmdImport(os.Args[2:])
	case "play":
		err = cmdPlay(os.Args[2:])
	case "check":
		err = cmdCheck(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		err = fmt.Errorf("unknown command %q (import, play, check)", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gameviber:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `gameviber — a GameMaker-style engine and toolset

  gameviber import GAME.exe DIR     turn a GameMaker 5.3 game into a GameViber project in DIR
  gameviber check DIR               check a project: every .gml parses, every function called exists
                                    (file:line:col: message; exit 1 when there is anything)
  gameviber play [flags] GAME       run a game headless (GAME: a GameMaker 5.3 .exe or a project folder)
      -room NAME      start in this room (default: the first in the room order)
      -steps N        steps to run (default 300)
      -keys SCRIPT    keys held per span: "right:40,right+up:25,:10,left:30"
                      (names: left right up down space enter; empty = none)
      -shot FILE.png  write the last frame
      -json           print the state as JSON (room, score, lives, instances, errors)
`)
	os.Exit(2)
}

func open(path string) (*gmfile.Project, error) {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() { // a GameViber text project
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

func cmdImport(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: gameviber import GAME.exe DIR")
	}
	p, err := open(args[0])
	if err != nil {
		return err
	}
	name := strings.TrimSuffix(args[0][strings.LastIndex(args[0], "/")+1:], ".exe")
	notes, err := p.Export(args[1], name)
	for _, n := range notes {
		fmt.Println("note:", n)
	}
	if err != nil {
		return err
	}
	fmt.Printf("imported %s into %s: %d objects, %d rooms, %d sprites, %d sounds\n", args[0], args[1], count(p.Objects), count(p.Rooms), count(p.Sprites), count(p.Sounds))
	return nil
}

func count[T any](list []*T) int {
	n := 0
	for _, x := range list {
		if x != nil {
			n++
		}
	}
	return n
}

var keyNames = map[string]runner.Key{"left": 37, "up": 38, "right": 39, "down": 40, "space": 32, "enter": 13}

type span struct {
	keys  []runner.Key
	steps int
}

func parseKeys(s string) ([]span, error) {
	var out []span
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	for _, part := range strings.Split(s, ",") {
		k, n, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			return nil, fmt.Errorf("key span %q: want KEYS:STEPS", part)
		}
		steps, err := strconv.Atoi(n)
		if err != nil || steps < 0 {
			return nil, fmt.Errorf("key span %q: bad step count", part)
		}
		var sp span
		sp.steps = steps
		for _, name := range strings.Split(k, "+") {
			if name == "" {
				continue
			}
			key, ok := keyNames[name]
			if !ok {
				return nil, fmt.Errorf("key span %q: unknown key %q (left right up down space enter)", part, name)
			}
			sp.keys = append(sp.keys, key)
		}
		out = append(out, sp)
	}
	return out, nil
}

type keySet struct{ held, prev map[runner.Key]bool }

func (k *keySet) Held(c runner.Key) bool    { return k.held[c] }
func (k *keySet) Pressed(c runner.Key) bool { return k.held[c] && !k.prev[c] }

func cmdPlay(args []string) error {
	fs := flag.NewFlagSet("play", flag.ContinueOnError)
	room := fs.String("room", "", "start room")
	steps := fs.Int("steps", 300, "steps to run")
	keys := fs.String("keys", "", "key script")
	shot := fs.String("shot", "", "write the last frame (PNG)")
	asJSON := fs.Bool("json", false, "print the state as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: gameviber play [flags] GAME.exe|DIR")
	}
	p, err := open(fs.Arg(0))
	if err != nil {
		return err
	}
	script, err := parseKeys(*keys)
	if err != nil {
		return err
	}
	g, err := load.FromProject(p)
	if err != nil {
		return err
	}
	var pl *load.Play
	if *room != "" {
		pl, err = g.StartAt(*room)
	} else {
		pl, err = g.Start()
	}
	if err != nil {
		return err
	}
	k := &keySet{held: map[runner.Key]bool{}, prev: map[runner.Key]bool{}}
	keysAt := func(i int) []runner.Key { // the script's keys at step i (none after its end)
		for _, sp := range script {
			if i < sp.steps {
				return sp.keys
			}
			i -= sp.steps
		}
		return nil
	}
	rooms := []string{pl.Room.Name}
	for i := 0; i < *steps && !pl.Ended; i++ {
		k.prev, k.held = k.held, map[runner.Key]bool{}
		for _, c := range keysAt(i) {
			k.held[c] = true
		}
		if err := pl.Step(k); err != nil {
			return err
		}
		if pl.Room.Name != rooms[len(rooms)-1] {
			rooms = append(rooms, pl.Room.Name)
		}
	}
	if *shot != "" {
		var gr *gmfile.Room
		for _, r := range p.Rooms {
			if r != nil && r.Name == pl.Room.Name {
				gr = r
			}
		}
		w, h := pl.Room.W, pl.Room.H
		if pl.Room.ViewsEnabled && len(pl.Room.Views) > 0 && pl.Room.Views[0].Visible {
			w, h = pl.Room.Views[0].W, pl.Room.Views[0].H
		}
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		render.Frame(img, g.Game, pl.Room, render.LoadAssets(p), gr.BGColor, gr.DrawBGColor)
		f, err := os.Create(*shot)
		if err != nil {
			return err
		}
		png.Encode(f, img)
		f.Close()
	}
	state := map[string]any{"rooms": rooms, "room": pl.Room.Name, "score": pl.Globals.Score, "lives": pl.Globals.Lives}
	insts := map[string]int{}
	for _, in := range pl.Room.Instances {
		if !in.Destroyed {
			insts[in.Obj.Name]++
		}
	}
	state["instances"] = insts
	var errs []string
	for _, e := range g.Errors {
		errs = append(errs, e.Error())
	}
	state["errors"] = errs
	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(state)
	}
	fmt.Printf("rooms: %s\nscore %d, lives %d\n", strings.Join(rooms, " → "), pl.Globals.Score, pl.Globals.Lives)
	for _, e := range errs {
		fmt.Println("error:", e)
	}
	return nil
}

func cmdCheck(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gameviber check DIR")
	}
	dir := args[0]
	// First every file on its own (syntax, with its path), then the calls
	// against what the loaded project knows.
	type parsed struct {
		path string
		list []gml.Stmt
	}
	var ok []parsed
	problems, files := 0, 0
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".gml") {
			return err
		}
		files++
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		list, err := gml.Parse(string(src))
		if err != nil {
			fmt.Printf("%s:%v\n", path, err)
			problems++
			return nil
		}
		ok = append(ok, parsed{path, list})
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	if problems == 0 {
		p, err := gmfile.ReadDir(dir)
		if err != nil {
			return err
		}
		g, err := load.FromProject(p)
		if err != nil {
			return err
		}
		known := g.KnownFuncs()
		for _, f := range ok {
			for _, c := range gml.Calls(f.list) {
				if !known[c.Fn] {
					fmt.Printf("%s:%s: %s is not a function GameViber knows (nor a script of this project)\n", f.path, c.P, c.Fn)
					problems++
				}
			}
		}
	}
	fmt.Printf("%d .gml files, %d problems\n", files, problems)
	if problems > 0 {
		os.Exit(1)
	}
	return nil
}
