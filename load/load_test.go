package load

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/LaPingvino/gameviber/import/gmfile"
	"github.com/LaPingvino/gameviber/runner"
)

type noKeys struct{}

func (noKeys) Held(runner.Key) bool    { return false }
func (noKeys) Pressed(runner.Key) bool { return false }

// Pacmanio's first level, loaded from its 2006 executable and run headless
// for 200 steps: what runs, what the runner still lacks.
func TestRunPacmanioLevel(t *testing.T) {
	d, err := os.ReadFile("../../pacmanio/win/pacmanio.exe")
	if err != nil {
		t.Skip("Pacmanio's exe is not checked out next to this repo")
	}
	e, err := gmfile.OpenExe(d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	g, err := FromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	// As if pu_bildo (the region's title room) came first: its creation code
	// sets the region.
	r, err := g.Room("pu_1a_nivelo", &runner.Globals{Lives: 3, Vars: map[string]float64{"murigxu": 2}})
	if err != nil {
		t.Fatal(err)
	}
	start := len(r.All("point"))
	for range 200 {
		r.Step(noKeys{})
		r.NoMoreLivesDue()
	}
	missing := map[string]int{}
	for _, err := range g.Errors {
		missing[strings.TrimSpace(err.Error()[strings.Index(err.Error(), ":")+1:])]++
	}
	var list []string
	for k, n := range missing {
		list = append(list, k)
		_ = n
	}
	sort.Strings(list)
	t.Logf("%d instances, %d dots; %d unsupported events (%s); errors: %s", len(r.Instances), start, len(g.Unsupported), strings.Join(g.Unsupported, ", "), strings.Join(list, " | "))
	if start == 0 {
		t.Error("no dots in the level")
	}
	ghosts := r.All("monster")
	moved := 0
	for _, gh := range ghosts {
		if gh.X != gh.XStart || gh.Y != gh.YStart {
			moved++
		}
	}
	t.Logf("%d of %d ghosts moved", moved, len(ghosts))
}

// Pacmanio from its first room: with no input, komenco's timer sends it to
// fino (the highscores) after 450 steps — the room change, the room end /
// start and the globals across rooms.
func TestPlayPacmanioFromStart(t *testing.T) {
	d, err := os.ReadFile("../../pacmanio/win/pacmanio.exe")
	if err != nil {
		t.Skip("Pacmanio's exe is not checked out next to this repo")
	}
	e, err := gmfile.OpenExe(d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	g, err := FromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := g.Start()
	if err != nil {
		t.Fatal(err)
	}
	rooms := []string{pl.Room.Name}
	for range 600 {
		if err := pl.Step(noKeys{}); err != nil {
			t.Fatal(err)
		}
		if n := pl.Room.Name; n != rooms[len(rooms)-1] {
			rooms = append(rooms, n)
		}
	}
	t.Logf("rooms: %v; lives %d; errors %v", rooms, pl.Globals.Lives, g.Errors)
	if len(rooms) < 2 || rooms[0] != "komenco" || rooms[1] != "fino" {
		t.Errorf("rooms %v, want komenco then fino", rooms)
	}
}
