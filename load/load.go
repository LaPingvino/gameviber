// Package load turns an imported GameMaker project into a running game: the
// runner's objects with their events compiled from GML, and rooms made into
// runner rooms (step 3e).
package load

import (
	"fmt"
	"strings"

	"github.com/LaPingvino/gameviber/gml"
	"github.com/LaPingvino/gameviber/import/gmfile"
	"github.com/LaPingvino/gameviber/runner"
)

// Game is a loaded game: the runner's game and the project it came from.
type Game struct {
	*runner.Game
	Project *gmfile.Project
	// Unsupported lists events the runner has no slot for yet (by object).
	Unsupported []string
}

// FromProject builds the runner's objects from the project's.
func FromProject(p *gmfile.Project) (*Game, error) {
	g := &Game{Game: &runner.Game{}, Project: p}
	g.ClearHighscores()
	objs := make([]*runner.Object, len(p.Objects))
	for i, o := range p.Objects {
		if o == nil {
			objs[i] = &runner.Object{Name: fmt.Sprintf("<deleted %d>", i)}
			continue
		}
		ro := &runner.Object{Name: o.Name, Solid: o.Solid, Visible: o.Visible, Depth: int(o.Depth)}
		spr := o.Sprite
		if o.Mask >= 0 {
			spr = o.Mask
		}
		if spr >= 0 && int(spr) < len(p.Sprites) && p.Sprites[spr] != nil {
			s := p.Sprites[spr]
			// bbox_lrbt: left, right, bottom, top — relative to the sprite's
			// corner; the runner's boxes are relative to the instance, so the
			// origin comes off.
			ro.BBox = runner.Box{L: int(s.BBox[0] - s.OriginX), R: int(s.BBox[1] - s.OriginX),
				T: int(s.BBox[3] - s.OriginY), B: int(s.BBox[2] - s.OriginY)}
			ro.Mask = true
		}
		if o.Sprite >= 0 && int(o.Sprite) < len(p.Sprites) && p.Sprites[o.Sprite] != nil {
			ro.Sprite = p.Sprites[o.Sprite].Name
		}
		objs[i] = ro
	}
	g.Objects = objs
	// The game's constants: each value is GML (an expression), evaluated
	// once here.
	g.Constants = map[string]gml.Value{}
	for _, c := range p.Settings.Constants {
		list, err := gml.Parse("__c = " + c[1])
		if err != nil {
			return nil, fmt.Errorf("constant %s = %s: %w", c[0], c[1], err)
		}
		tmp := runner.NewInstance(&runner.Object{Name: "<constants>"}, 0, 0)
		in := &gml.Interp{Self: tmp, Global: &runner.GlobalVars{G: &runner.Globals{Vars: map[string]float64{}}}}
		if _, err := in.Run(list); err != nil {
			return nil, fmt.Errorf("constant %s = %s: %w", c[0], c[1], err)
		}
		v, _ := tmp.Get("__c", nil)
		g.Constants[c[0]] = v
	}
	g.Resources = map[string]bool{}
	for _, x := range p.Sprites {
		if x != nil {
			g.Resources[x.Name] = true
		}
	}
	for _, x := range p.Sounds {
		if x != nil {
			g.Resources[x.Name] = true
		}
	}
	for _, x := range p.Backgrounds {
		if x != nil {
			g.Resources[x.Name] = true
		}
	}
	for _, x := range p.Rooms {
		if x != nil {
			g.Resources[x.Name] = true
		}
	}
	for _, x := range p.Timelines {
		if x != nil {
			g.Resources[x.Name] = true
		}
	}
	for _, sc := range p.Scripts {
		name := ""
		if sc != nil {
			name = sc.Name
			if err := g.Script(sc.Name, sc.Code); err != nil {
				return nil, fmt.Errorf("script %s: %w", sc.Name, err)
			}
		}
		g.ScriptNames = append(g.ScriptNames, name)
	}
	for i, o := range p.Objects {
		if o == nil {
			continue
		}
		if o.Parent >= 0 && int(o.Parent) < len(objs) {
			objs[i].Parent = objs[o.Parent]
		}
		for _, ev := range o.Events {
			if err := g.event(objs[i], o, ev); err != nil {
				return nil, fmt.Errorf("%s %s %d: %w", o.Name, ev.Type, ev.Sub, err)
			}
		}
	}
	return g, nil
}

func (g *Game) event(ro *runner.Object, o *gmfile.GMObject, ev gmfile.Event) error {
	src, err := g.Project.ActionsToGML(ev.Actions)
	if err != nil {
		return err
	}
	unsupported := func() { g.Unsupported = append(g.Unsupported, fmt.Sprintf("%s: %s %d", o.Name, ev.Type, ev.Sub)) }
	e := &ro.Events
	switch ev.Type {
	case "collision":
		h, err := g.CollisionCode(src)
		if err != nil {
			return err
		}
		if e.Collision == nil {
			e.Collision = map[string]runner.CollisionHandler{}
		}
		if int(ev.Sub) < len(g.Objects) {
			e.Collision[g.Objects[ev.Sub].Name] = h
		}
		return nil
	}
	h, err := g.Code(src)
	if err != nil {
		return err
	}
	switch ev.Type {
	case "create":
		e.Create = h
	case "alarm":
		if ev.Sub >= 0 && int(ev.Sub) < len(e.Alarm) {
			e.Alarm[ev.Sub] = h
		}
	case "step":
		switch ev.Sub {
		case 0:
			e.Step = h
		case 1:
			e.BeginStep = h
		case 2:
			e.EndStep = h
		}
	case "keyboard":
		if e.Key == nil {
			e.Key = map[runner.Key]runner.Handler{}
		}
		e.Key[runner.Key(ev.Sub)] = h
	case "keypress":
		if e.KeyPress == nil {
			e.KeyPress = map[runner.Key]runner.Handler{}
		}
		e.KeyPress[runner.Key(ev.Sub)] = h
	case "other":
		switch ev.Sub { // GM's numbering of the other events
		case 0:
			e.OutsideRoom = h
		case 4:
			e.RoomStart = h
		case 5:
			e.RoomEnd = h
		case 6:
			e.NoMoreLives = h
		default:
			unsupported() // boundary, game start / end, animation / path end, user events
		}
	case "destroy":
		e.Destroy = h
	default:
		unsupported() // draw (the drawing backend), mouse, key release
	}
	return nil
}

// Room makes the project's room into a runner room with its instances
// (each one's create event run, in placement order).
func (g *Game) Room(name string, glob *runner.Globals) (*runner.Room, error) {
	for _, r := range g.Project.Rooms {
		if r == nil || r.Name != name {
			continue
		}
		rr := &runner.Room{Name: r.Name, W: int(r.W), H: int(r.H), Speed: int(r.Speed), G: glob}
		// GM 5's order: each instance's create event, then its creation code;
		// then the room's creation code; then every room start event.
		for _, in := range r.Instances {
			if in.Obj < 0 || int(in.Obj) >= len(g.Objects) {
				continue
			}
			inst := rr.Create(g.Objects[in.Obj], float64(in.X), float64(in.Y))
			if strings.TrimSpace(in.Code) != "" {
				h, err := g.Code(in.Code)
				if err != nil {
					return nil, fmt.Errorf("room %s, instance %d's code: %w", r.Name, in.ID, err)
				}
				h(rr, inst)
			}
		}
		if strings.TrimSpace(r.CreationCode) != "" {
			h, err := g.Code(r.CreationCode)
			if err != nil {
				return nil, fmt.Errorf("room %s's creation code: %w", r.Name, err)
			}
			// It runs as no instance in particular: a stand-in holds its
			// variables (GM 5 runs it with a dummy self too).
			h(rr, runner.NewInstance(&runner.Object{Name: "<room>"}, 0, 0))
		}
		rr.Start()
		return rr, nil
	}
	return nil, fmt.Errorf("no room %q", name)
}
