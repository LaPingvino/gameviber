package load

import (
	"encoding/json"
	"fmt"
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

// Stelo's bonus level: with its monsters gone, monsterfarilo starts the
// timeline faruMonsterojn, which makes a b_monster at its steps 30, 60, 90
// (STELO-RULES §4) — at each of the room's spawners (four).
func TestSteloTimeline(t *testing.T) {
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
	r, err := g.Room("bonusnivelo", &runner.Globals{Lives: 3, Vars: map[string]float64{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range r.All("b_monster") {
		m.Destroyed = true
	}
	spawners := len(r.All("monsterfarilo"))
	counts := map[int]int{}
	for i := 1; i <= 100; i++ {
		r.Step(noKeys{})
		if i == 25 || i == 35 || i == 65 || i == 95 {
			counts[i] = len(r.All("b_monster"))
		}
	}
	t.Logf("b_monsters at steps 25/35/65/95: %v; errors %v", counts, g.Errors)
	if counts[25] != 0 || counts[35] != spawners || counts[65] != 2*spawners || counts[95] != 3*spawners {
		t.Errorf("b_monsters %v, want 0 / 1 / 2 / 3 × %d spawners", counts, spawners)
	}
}

// The precise sprites' objects get pixel masks.
func TestPreciseMasksLoaded(t *testing.T) {
	for _, exe := range []string{"../../stelo/win/stelo.exe", "../../pacmanio/win/pacmanio.exe"} {
		d, err := os.ReadFile(exe)
		if err != nil {
			t.Skip("the sample games are not checked out next to this repo")
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
		var with []string
		for _, o := range g.Objects {
			if len(o.Pixels) > 0 {
				with = append(with, o.Name)
			}
		}
		t.Logf("%s: %d objects with pixel masks: %v", exe, len(with), with)
		if len(with) == 0 {
			t.Errorf("%s: no object has a pixel mask", exe)
		}
	}
}

// heldKey holds one key.
type heldKey runner.Key

func (k heldKey) Held(c runner.Key) bool  { return runner.Key(k) == c }
func (k heldKey) Pressed(runner.Key) bool { return false }

// Pacmanio's first level with its ghosts, the same seed and keys, once with
// the precise sprites' pixel masks and once with boxes only (as the hand
// ports collide): where do they first differ? A report, not a check — it
// tells whether box collisions change play against the 2006 game.
func TestPacmanioMasksVersusBoxes(t *testing.T) {
	if os.Getenv("GAMEVIBER_REPORTS") == "" {
		t.Skip("a report (several seconds): GAMEVIBER_REPORTS=1 to run it")
	}
	d, err := os.ReadFile("../../pacmanio/win/pacmanio.exe")
	if err != nil {
		t.Skip("Pacmanio's exe is not checked out next to this repo")
	}
	run := func(masks bool, seed int64) []string {
		e, _ := gmfile.OpenExe(d)
		p, err := e.ReadProject()
		if err != nil {
			t.Fatal(err)
		}
		g, err := FromProject(p)
		if err != nil {
			t.Fatal(err)
		}
		g.Seed = seed
		if !masks {
			for _, o := range g.Objects {
				o.Pixels = nil
			}
		}
		r, err := g.Room("pu_1a_nivelo", &runner.Globals{Lives: 30, Vars: map[string]float64{"murigxu": 2}})
		if err != nil {
			t.Fatal(err)
		}
		var trace []string
		// a fixed pseudo-random walk: a new arrow every 12–20 steps
		lcg, key, next := uint32(seed)*2654435761+1, runner.Key(39), 0
		for step := range 3000 {
			if step == next {
				lcg = lcg*1664525 + 1013904223
				key, next = runner.Key(37+lcg>>30), step+12+int(lcg>>8%9)
			}
			r.Step(heldKey(key))
			r.NoMoreLivesDue()
			pac := r.All("pacman")
			s := fmt.Sprintf("score %d lives %d dots %d", r.G.Score, r.G.Lives, len(r.All("point")))
			if len(pac) > 0 {
				s += fmt.Sprintf(" pacman %.0f,%.0f", pac[0].X, pac[0].Y)
			}
			for _, m := range r.All("monster") {
				s += fmt.Sprintf(" %s %.0f,%.0f", m.Obj.Name, m.X, m.Y)
			}
			trace = append(trace, s)
		}
		return trace
	}
	for seed := int64(1); seed <= 4; seed++ { // 8 seeds checked 2026-11-03: all identical
		withMasks, withBoxes := run(true, seed), run(false, seed)
		same := true
		for i := range withMasks {
			if withMasks[i] != withBoxes[i] {
				t.Logf("seed %d: first difference at step %d:\n masks: %.120s\n boxes: %.120s", seed, i, withMasks[i], withBoxes[i])
				same = false
				break
			}
		}
		if same {
			t.Logf("seed %d: identical over %d steps; the end: %.40s", seed, len(withMasks), withMasks[len(withMasks)-1])
		}
	}
}

// Stelo's levels 1–3, the same as TestPacmanioMasksVersusBoxes: a
// pseudo-random walk (left, right, up = climb / jump, down), pixel masks
// against boxes only. A report, not a check.
func TestSteloMasksVersusBoxes(t *testing.T) {
	if os.Getenv("GAMEVIBER_REPORTS") == "" {
		t.Skip("a report (several seconds): GAMEVIBER_REPORTS=1 to run it")
	}
	d, err := os.ReadFile("../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	run := func(masks bool, room string, seed int64) []string {
		e, _ := gmfile.OpenExe(d)
		p, err := e.ReadProject()
		if err != nil {
			t.Fatal(err)
		}
		g, err := FromProject(p)
		if err != nil {
			t.Fatal(err)
		}
		g.Seed = seed
		if !masks {
			for _, o := range g.Objects {
				o.Pixels = nil
			}
		}
		pl, err := g.StartAt(room)
		if err != nil {
			t.Fatal(err)
		}
		var trace []string
		lcg, key, next := uint32(seed)*2654435761+1, runner.Key(39), 0
		for step := range 2000 {
			if step == next {
				lcg = lcg*1664525 + 1013904223
				key, next = runner.Key(37+lcg>>30), step+8+int(lcg>>8%17)
			}
			if err := pl.Step(heldKey(key)); err != nil {
				t.Fatal(err)
			}
			s := fmt.Sprintf("%s score %d lives %d", pl.Room.Name, pl.Globals.Score, pl.Globals.Lives)
			for _, c := range pl.Room.All("character") {
				s += fmt.Sprintf(" player %.0f,%.0f", c.X, c.Y)
			}
			s += fmt.Sprintf(" instances %d", len(pl.Room.Instances))
			trace = append(trace, s)
		}
		return trace
	}
	for _, room := range []string{"nivelo1", "nivelo2", "nivelo3"} {
		for seed := int64(1); seed <= 2; seed++ {
			a, b := run(true, room, seed), run(false, room, seed)
			diff := -1
			for i := range a {
				if a[i] != b[i] {
					diff = i
					break
				}
			}
			if diff >= 0 {
				t.Logf("%s seed %d: first difference at step %d:\n masks: %s\n boxes: %s", room, seed, diff, a[diff], b[diff])
			} else {
				t.Logf("%s seed %d: identical over %d steps; the end: %s", room, seed, len(a), a[len(a)-1])
			}
		}
	}
}

// Persistent instances go along to the next room: Stelo's life_controller
// (persistent, placed in nivelo1 only) is in nivelo2 after the level change,
// the same instance, without a second create; the lives stay.
func TestPersistentInstancesCarry(t *testing.T) {
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
	lc := pl.Room.All("life_controller")
	if len(lc) != 1 {
		t.Fatalf("nivelo1 has %d life controllers", len(lc))
	}
	pl.Globals.Lives = 2
	pl.Room.Goto = "nivelo2"
	if err := pl.Step(noKeys{}); err != nil {
		t.Fatal(err)
	}
	if pl.Room.Name != "nivelo2" {
		t.Fatalf("in %s", pl.Room.Name)
	}
	got := pl.Room.All("life_controller")
	if len(got) != 1 || got[0] != lc[0] || got[0].ID != lc[0].ID || pl.Globals.Lives != 2 {
		t.Errorf("nivelo2: %d life controllers (same: %v), lives %d; errors %v", len(got), len(got) == 1 && got[0] == lc[0], pl.Globals.Lives, g.Errors)
	}
}

// Stelo's levels 1–3 under pseudo-random walks (the walk of
// TestSteloMasksVersusBoxes), against the hand port given the same keys:
// the player's position and the score at every step
// ($GAMEVIBER_STELO_WALKS: a folder with stelo-walk-<level>-<seed>.json).
// Lives are left out: started in nivelo2 / 3 the original has no life
// controller yet (it comes along from nivelo1), the hand port starts at 3.
func TestSteloWalksAgainstHandPort(t *testing.T) {
	dir := os.Getenv("GAMEVIBER_STELO_WALKS")
	if dir == "" {
		t.Skip("no hand-port walks ($GAMEVIBER_STELO_WALKS)")
	}
	d, err := os.ReadFile("../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	for _, room := range []string{"nivelo1", "nivelo2", "nivelo3"} {
		for seed := int64(1); seed <= 2; seed++ {
			raw, err := os.ReadFile(fmt.Sprintf("%s/stelo-walk-%s-%d.json", dir, room, seed))
			if err != nil {
				t.Fatal(err)
			}
			var want [][4]float64
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			e, _ := gmfile.OpenExe(d)
			p, _ := e.ReadProject()
			g, err := FromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			pl, err := g.StartAt(room)
			if err != nil {
				t.Fatal(err)
			}
			lcg, key, next := uint32(seed)*2654435761+1, runner.Key(39), 0
			for step := range want {
				if step == next {
					lcg = lcg*1664525 + 1013904223
					key, next = runner.Key(37+lcg>>30), step+8+int(lcg>>8%17)
				}
				if err := pl.Step(heldKey(key)); err != nil {
					t.Fatal(err)
				}
				if pl.Room.Name != room { // the hand port's level harness does not change rooms
					t.Logf("%s seed %d: identical for %d steps, then on to %s (score %d, lives %d)", room, seed, step, pl.Room.Name, pl.Globals.Score, pl.Globals.Lives)
					break
				}
				x, y := -1.0, -1.0
				if c := pl.Room.All("character"); len(c) > 0 {
					x, y = c[0].X, c[0].Y
				}
				w := want[step]
				if x != w[0] || y != w[1] || float64(pl.Globals.Score) != w[2] {
					t.Errorf("%s seed %d step %d (key %d): GameViber in %s %v,%v score %d lives %d; hand port %v,%v score %v lives %v",
						room, seed, step, key, pl.Room.Name, x, y, pl.Globals.Score, pl.Globals.Lives, w[0], w[1], w[2], w[3])
					break
				}
				if step == len(want)-1 {
					t.Logf("%s seed %d: identical over %d steps (%v,%v score %v)", room, seed, len(want), x, y, w[2])
				}
			}
		}
	}
}
