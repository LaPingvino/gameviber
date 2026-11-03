package load

import (
	"fmt"
	"slices"

	"github.com/LaPingvino/gameviber/runner"
)

// Play is a game being played through its rooms (GameMaker's room order):
// the current room, the globals that live across rooms, and room changes.
type Play struct {
	G       *Game
	Room    *runner.Room
	Globals *runner.Globals
	order   []string
	Ended   bool
	// kept are the persistent rooms left, as they were (GM 5: coming back,
	// their instances are as left; no create events, the room start runs).
	kept map[string]*runner.Room
}

// StartAt begins the game at a named room (a test or tool's shortcut).
func (g *Game) StartAt(name string) (*Play, error) {
	p, err := g.Start()
	if err != nil {
		return nil, err
	}
	p.Room = nil
	return p, p.enter(name)
}

// Start begins the game at its first room. Score and lives start at 0
// (ASSUMPTION [engine]: GM 5's defaults; both sample games set their own).
func (g *Game) Start() (*Play, error) {
	p := &Play{G: g, Globals: &runner.Globals{Vars: map[string]float64{}}}
	for _, id := range g.Project.RoomOrder {
		if int(id) < len(g.Project.Rooms) && g.Project.Rooms[id] != nil {
			p.order = append(p.order, g.Project.Rooms[id].Name)
		}
	}
	if len(p.order) == 0 {
		return nil, fmt.Errorf("the game has no rooms")
	}
	if err := p.enterGame(p.order[0]); err != nil {
		return nil, err
	}
	return p, nil
}

// enterGame enters the first room with the game start event.
func (p *Play) enterGame(name string) error {
	r, err := p.G.room(name, p.Globals, true)
	if err != nil {
		return err
	}
	p.Room = r
	return nil
}

func (p *Play) enter(name string) error {
	if p.Room != nil {
		p.Room.End()
		if p.Room.Persistent {
			if p.kept == nil {
				p.kept = map[string]*runner.Room{}
			}
			p.kept[p.Room.Name] = p.Room
		}
	}
	if r, ok := p.kept[name]; ok {
		delete(p.kept, name)
		r.G, r.Goto = p.Globals, ""
		p.Room = r
		r.Start()
		return nil
	}
	r, err := p.G.Room(name, p.Globals)
	if err != nil {
		return err
	}
	p.Room = r
	return nil
}

// Step runs one step of the current room, then the no-more-lives event, then
// any room change it asked for.
func (p *Play) Step(in runner.Input) error {
	if p.Ended {
		return nil
	}
	p.Room.Step(in)
	p.Room.NoMoreLivesDue()
	to := p.Room.Goto
	if to == "" || p.Room.Sleep > 0 {
		return nil
	}
	p.Room.Goto = ""
	i := slices.Index(p.order, p.Room.Name)
	switch to {
	case "@next":
		if i+1 >= len(p.order) {
			return nil // GM: no next room — stays
		}
		to = p.order[i+1]
	case "@previous":
		if i <= 0 {
			return nil
		}
		to = p.order[i-1]
	case "@restart":
		p.Room.Every(func(e *runner.Events) runner.Handler { return e.Other[3] }) // game end
		p.Room.End()
		p.Globals = &runner.Globals{Vars: map[string]float64{}}
		p.kept = nil
		return p.enterGame(p.order[0])
	case "@end":
		p.Room.Every(func(e *runner.Events) runner.Handler { return e.Other[3] }) // game end
		p.Ended = true
		return nil
	}
	return p.enter(to)
}
