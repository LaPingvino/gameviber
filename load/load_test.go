package load

import (
	"encoding/json"
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

// held holds one key (pressed on its first step).
type held struct {
	key   runner.Key
	first bool
}

func (h *held) Held(k runner.Key) bool    { return k == h.key }
func (h *held) Pressed(k runner.Key) bool { return k == h.key && h.first }

// Stelo's nivelo2 under GameViber against the hand port (a step-exact
// remake): the player's position at every step of 150 with → held. The hand
// port's trace comes from $GAMEVIBER_STELO_TRACE (JSON [[x,y],…], made by
// a throwaway program in the Stelo repo).
func TestSteloAgainstHandPort(t *testing.T) {
	tp := os.Getenv("GAMEVIBER_STELO_TRACE")
	if tp == "" {
		t.Skip("no hand-port trace ($GAMEVIBER_STELO_TRACE)")
	}
	js, err := os.ReadFile(tp)
	if err != nil {
		t.Fatal(err)
	}
	var want [][2]float64
	if err := json.Unmarshal(js, &want); err != nil {
		t.Fatal(err)
	}
	d, err := os.ReadFile("../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	e, _ := gmfile.OpenExe(d)
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	g, err := FromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	r, err := g.Room("nivelo2", &runner.Globals{Lives: 3, Vars: map[string]float64{}})
	if err != nil {
		t.Fatal(err)
	}
	h := &held{key: runner.KeyRight, first: true}
	for i, w := range want {
		r.Step(h)
		h.first = false
		pl := r.All("character")[0]
		if pl.X != w[0] || pl.Y != w[1] {
			t.Fatalf("step %d: GameViber (%v, %v), hand port (%v, %v)", i+1, pl.X, pl.Y, w[0], w[1])
		}
	}
	t.Logf("%d steps identical; errors %v", len(want), g.Errors)
}

// keySet is the keys held this step and the step before.
type keySet struct{ held, prev map[runner.Key]bool }

func (k *keySet) Held(c runner.Key) bool    { return k.held[c] }
func (k *keySet) Pressed(c runner.Key) bool { return k.held[c] && !k.prev[c] }

// steloScript is the script the hand-port traces were made with: keys held
// for a number of steps (runs, jumps, back, waiting, running into monsters).
var steloScript = []struct {
	keys  []runner.Key
	steps int
}{
	{[]runner.Key{runner.KeyRight}, 40}, {[]runner.Key{runner.KeyRight, runner.KeyUp}, 25}, {[]runner.Key{runner.KeyRight}, 60},
	{[]runner.Key{runner.KeyLeft}, 30}, {[]runner.Key{runner.KeyUp}, 15}, {nil, 10}, {[]runner.Key{runner.KeyRight, runner.KeyUp}, 80}, {[]runner.Key{runner.KeyRight}, 100},
}

// Levels 1–3 under the script, against the hand port: the player's position,
// the score and the lives at every step ($GAMEVIBER_STELO_TRACES: a folder
// with stelo-trace2-<level>.json).
func TestSteloScriptAgainstHandPort(t *testing.T) {
	dir := os.Getenv("GAMEVIBER_STELO_TRACES")
	if dir == "" {
		t.Skip("no hand-port traces ($GAMEVIBER_STELO_TRACES)")
	}
	d, err := os.ReadFile("../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	e, _ := gmfile.OpenExe(d)
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	for _, lv := range []string{"nivelo1", "nivelo2", "nivelo3"} {
		t.Run(lv, func(t *testing.T) {
			js, err := os.ReadFile(dir + "/stelo-trace2-" + lv + ".json")
			if err != nil {
				t.Skip(err)
			}
			var want [][4]float64
			json.Unmarshal(js, &want)
			g, err := FromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			r, err := g.Room(lv, &runner.Globals{Lives: 3, Vars: map[string]float64{}})
			if err != nil {
				t.Fatal(err)
			}
			k := &keySet{held: map[runner.Key]bool{}, prev: map[runner.Key]bool{}}
			i := 0
			for _, s := range steloScript {
				for range s.steps {
					k.prev, k.held = k.held, map[runner.Key]bool{}
					for _, c := range s.keys {
						k.held[c] = true
					}
					r.Step(k)
					got := [4]float64{-1, -1, float64(r.G.Score), float64(r.G.Lives)}
					if ps := r.All("character"); len(ps) > 0 {
						got[0], got[1] = ps[0].X, ps[0].Y
					}
					if got != want[i] {
						t.Fatalf("step %d: GameViber %v, hand port %v (x, y, score, lives); errors %v", i+1, got, want[i], g.Errors)
					}
					i++
				}
			}
			t.Logf("%d steps identical (ends at %v)", i, want[i-1])
		})
	}
}

// Pacmanio's pu_1a_nivelo against the hand port with the ghosts taken out
// (they move at random): Pacman's position, score, lives and the dots left
// at every step of a key script ($GAMEVIBER_PAC_TRACE).
func TestPacmanioAgainstHandPort(t *testing.T) {
	tp := os.Getenv("GAMEVIBER_PAC_TRACE")
	if tp == "" {
		t.Skip("no hand-port trace ($GAMEVIBER_PAC_TRACE)")
	}
	js, err := os.ReadFile(tp)
	if err != nil {
		t.Fatal(err)
	}
	var want [][5]float64
	json.Unmarshal(js, &want)
	d, err := os.ReadFile("../../pacmanio/win/pacmanio.exe")
	if err != nil {
		t.Skip("Pacmanio's exe is not checked out next to this repo")
	}
	e, _ := gmfile.OpenExe(d)
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	{ // through the text project: exported and read back (the round trip), then run
		dir := t.TempDir()
		if _, err := p.Export(dir, "pacmanio"); err != nil {
			t.Fatal(err)
		}
		if p, err = gmfile.ReadDir(dir); err != nil {
			t.Fatal(err)
		}
	}
	g, err := FromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	r, err := g.Room("pu_1a_nivelo", &runner.Globals{Lives: 3, Vars: map[string]float64{"murigxu": 2}})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range r.All("monster") {
		m.Destroyed = true
	}
	script := []struct {
		keys  []runner.Key
		steps int
	}{
		{[]runner.Key{runner.KeyLeft}, 30}, {[]runner.Key{runner.KeyUp}, 25}, {[]runner.Key{runner.KeyRight}, 40}, {[]runner.Key{runner.KeyDown}, 30},
		{nil, 5}, {[]runner.Key{runner.KeyLeft}, 60}, {[]runner.Key{runner.KeyUp}, 40}, {[]runner.Key{runner.KeyRight}, 80},
	}
	k := &keySet{held: map[runner.Key]bool{}, prev: map[runner.Key]bool{}}
	i := 0
	for _, s := range script {
		for range s.steps {
			k.prev, k.held = k.held, map[runner.Key]bool{}
			for _, c := range s.keys {
				k.held[c] = true
			}
			r.Step(k)
			pm := r.All("pacman")[0]
			got := [5]float64{pm.X, pm.Y, float64(r.G.Score), float64(r.G.Lives), float64(len(r.All("point")))}
			if got != want[i] {
				t.Fatalf("step %d: GameViber %v, hand port %v (x, y, score, lives, dots); errors %v", i+1, got, want[i], g.Errors)
			}
			i++
		}
	}
	t.Logf("%d steps identical (ends at %v); errors %v", i, want[i-1], g.Errors)
}

// Stelo's bonus level, as the hand port's TestRoomChanges: the diamond makes
// nivelo1 persistent and goes to bonusnivelo; a flyer there brings the player
// back to nivelo1 as it was left — the same player, the diamond gone, no life
// lost.
func TestSteloBonusRoundTrip(t *testing.T) {
	d, err := os.ReadFile("../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	e, _ := gmfile.OpenExe(d)
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	g, err := FromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := g.StartAt("nivelo1")
	if err != nil {
		t.Fatal(err)
	}
	pl.Globals.Lives = 3
	player := pl.Room.All("character")[0]
	dia := pl.Room.All("iru_al_bonuso")[0]
	player.X, player.Y = dia.X, dia.Y
	for i := 0; i < 10 && pl.Room.Name != "bonusnivelo"; i++ {
		if err := pl.Step(noKeys{}); err != nil {
			t.Fatal(err)
		}
	}
	if pl.Room.Name != "bonusnivelo" {
		t.Fatalf("the diamond led to %s; errors %v", pl.Room.Name, g.Errors)
	}
	lives := pl.Globals.Lives
	bp := pl.Room.All("character")[0]
	f := pl.Room.All("flyer")[0]
	bp.X, bp.Y = f.X, f.Y
	for i := 0; i < 60 && pl.Room.Name != "nivelo1"; i++ {
		if err := pl.Step(noKeys{}); err != nil {
			t.Fatal(err)
		}
	}
	if pl.Room.Name != "nivelo1" || pl.Globals.Lives != lives {
		t.Fatalf("after the flyer: in %s, lives %d (was %d); errors %v", pl.Room.Name, pl.Globals.Lives, lives, g.Errors)
	}
	if len(pl.Room.All("iru_al_bonuso")) != 0 || pl.Room.All("character")[0] != player {
		t.Errorf("nivelo1 not as left: diamonds %d, same player %v", len(pl.Room.All("iru_al_bonuso")), pl.Room.All("character")[0] == player)
	}
	t.Logf("nivelo1 → bonusnivelo → nivelo1 as left; errors %v", g.Errors)
}
