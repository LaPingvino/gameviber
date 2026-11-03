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
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/LaPingvino/gameviber/gml"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
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
	case "edit":
		err = cmdEdit(os.Args[2:])
	case "run":
		err = cmdRun(os.Args[2:])
	case "web":
		err = cmdWeb(os.Args[2:])
	case "new":
		err = cmdNew(os.Args[2:])
	case "reference":
		err = cmdReference(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		err = fmt.Errorf("unknown command %q (new, import, play, check, run, web, reference)", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gameviber:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `gameviber — a GameMaker-style engine and toolset

  gameviber reference [FILE]        what GameViber's GML understands: events, functions, variables
  gameviber edit DIR                the editor, in your browser (served on this machine only)
  gameviber new DIR                 a small starter game to build on (collect the coins)
  gameviber import GAME.exe DIR     turn a GameMaker 5.3 game into a GameViber project in DIR
  gameviber check DIR               check a project: every .gml parses, every function called exists
                                    (file:line:col: message; exit 1 when there is anything)
  gameviber run [-room R] GAME      play it in a window (arrow keys, space, enter; M sound, F11 full screen)
  gameviber web DIR OUT             a web page that plays the project DIR (OUT: index.html, game.wasm, project.tar)
                                    (PARTIAL: run it inside the GameViber source; it builds the wasm with go)
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
	problems, err := checkProject(args[0], os.Stdout)
	if err != nil {
		return err
	}
	if problems > 0 {
		os.Exit(1)
	}
	return nil
}

// checkProject is gameviber check: syntax per file, then the calls against
// what the project knows; the report goes to w (the editor shows it too).
func checkProject(dir string, w io.Writer) (int, error) {
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
			fmt.Fprintf(w, "%s:%v\n", path, err)
			problems++
			return nil
		}
		ok = append(ok, parsed{path, list})
		return nil
	})
	if walkErr != nil {
		return 0, walkErr
	}
	if problems == 0 {
		p, err := gmfile.ReadDir(dir)
		if err != nil {
			return 0, err
		}
		g, err := load.FromProject(p)
		if err != nil {
			return 0, err
		}
		known := g.KnownFuncs()
		for _, f := range ok {
			for _, c := range gml.Calls(f.list) {
				if !known[c.Fn] {
					msg := fmt.Sprintf("%s:%s: %s is not a function GameViber knows (nor a script of this project)", f.path, c.P, c.Fn)
					if h := hint(c.Fn, known); h != "" {
						msg += " — " + h
					}
					fmt.Fprintln(w, msg)
					problems++
				}
			}
		}
	}
	fmt.Fprintf(w, "%d .gml files, %d problems\n", files, problems)
	return problems, nil
}

func cmdRun(args []string) error {
	// The window is a binary of its own (gameviber-run): Ebitengine needs a
	// display as soon as it is linked in, and gameviber's other commands must
	// work without one (servers, agents, tests).
	exe, err := os.Executable()
	path := filepath.Join(filepath.Dir(exe), "gameviber-run")
	if _, statErr := os.Stat(path); err != nil || statErr != nil {
		if path, err = exec.LookPath("gameviber-run"); err != nil {
			return fmt.Errorf("the window is gameviber-run: go install ./cmd/gameviber-run (or go run ./cmd/gameviber-run %s)", strings.Join(args, " "))
		}
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

const webPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s — GameViber</title>
<style>html,body{margin:0;height:100%%;background:#111;color:#ddd;font:14px system-ui,sans-serif}
#msg{position:fixed;inset:auto 0 8px 0;text-align:center;opacity:.7}</style></head>
<body><div id="msg">loading…</div>
<script src="wasm_exec.js?v=%s"></script>
<script>
// Each file's URL carries its content hash: a new build has new URLs, an
// unchanged one stays cached.
window.gameviberProject = "project.tar?v=%s";
const go = new Go();
WebAssembly.instantiateStreaming(fetch("game.wasm?v=%s"), go.importObject)
  .then(r => { document.getElementById("msg").remove(); go.run(r.instance); })
  .catch(e => { document.getElementById("msg").textContent = "could not start: " + e; });
</script></body></html>
`

func cmdWeb(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: gameviber web DIR OUT")
	}
	dir, out := args[0], args[1]
	p, err := gmfile.ReadDir(dir) // it must read before it's shipped
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	// The project, packed.
	f, err := os.Create(filepath.Join(out, "project.tar"))
	if err != nil {
		return err
	}
	err = packProject(dir, f)
	f.Close()
	if err != nil {
		return err
	}
	if err := buildPlayer(out); err != nil {
		return err
	}
	name := p.Name()
	if name == "" {
		name = filepath.Base(dir)
	}
	page := fmt.Sprintf(webPage, name, fileHash(filepath.Join(out, "wasm_exec.js")),
		fileHash(filepath.Join(out, "project.tar")), fileHash(filepath.Join(out, "game.wasm")))
	if err := os.WriteFile(filepath.Join(out, "index.html"), []byte(page), 0o644); err != nil {
		return err
	}
	fmt.Printf("web page for %s in %s (index.html, game.wasm, project.tar, wasm_exec.js)\n", name, out)
	return nil
}

// fileHash is the first 12 hex digits of a file's SHA-256 (the cache key in
// the page's URLs).
func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "0"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:6])
}

// packProject writes the project folder as a tar (what the browser player
// opens).
func packProject(dir string, w io.Writer) error {
	tw := tar.NewWriter(w)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: filepath.ToSlash(rel), Mode: 0o644, Size: int64(len(data))}); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
	if err != nil {
		return err
	}
	return tw.Close()
}

// buildPlayer builds the browser player into out: game.wasm (gameviber-run
// for js/wasm — needs the GameViber source and go) and Go's wasm_exec.js.
func buildPlayer(out string) error {
	build := exec.Command("go", "build", "-o", filepath.Join(out, "game.wasm"), "./cmd/gameviber-run")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("building the wasm (run gameviber inside the GameViber source): %w", err)
	}
	root, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return err
	}
	shim, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(root)), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "wasm_exec.js"), shim, 0o644)
}
