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
	return p, p.enter(p.order[0])
}

func (p *Play) enter(name string) error {
	if p.Room != nil {
		p.Room.End()
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
		p.Globals = &runner.Globals{Vars: map[string]float64{}}
		to = p.order[0]
	case "@end":
		p.Ended = true
		return nil
	}
	return p.enter(to)
}
