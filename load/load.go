// Package load turns an imported GameMaker project into a running game: the
// runner's objects with their events compiled from GML, and rooms made into
// runner rooms (step 3e).
package load

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/LaPingvino/gameviber/gml"
	"github.com/LaPingvino/gameviber/import/gmfile"
	_ "github.com/LaPingvino/gameviber/lang/js" // event code in JavaScript (.js files)
	"github.com/LaPingvino/gameviber/runner"
)

// Game is a loaded game: the runner's game and the project it came from.
type Game struct {
	*runner.Game
	Project *gmfile.Project
	// Unsupported lists events the runner has no slot for yet (by object).
	Unsupported []string
	// Clock is the time the game's current_* variables read (nil: the real
	// time) — a test play sets one that moves by steps so runs repeat.
	Clock func() time.Time
}

// FromProject builds the runner's objects from the project's.
func FromProject(p *gmfile.Project) (*Game, error) {
	g := &Game{Game: &runner.Game{}, Project: p}
	g.ClearHighscores()
	for _, id := range p.RoomOrder {
		if int(id) < len(p.Rooms) && p.Rooms[id] != nil {
			g.RoomOrder = append(g.RoomOrder, p.Rooms[id].Name)
		}
	}
	g.HasNextRoom = func(room string) bool {
		for i, id := range p.RoomOrder {
			if int(id) < len(p.Rooms) && p.Rooms[id] != nil && p.Rooms[id].Name == room {
				return i+1 < len(p.RoomOrder)
			}
		}
		return false
	}
	g.SpriteWidth = func(name string) int {
		for _, s := range p.Sprites {
			if s != nil && s.Name == name {
				return int(s.W)
			}
		}
		return 0
	}
	objs := make([]*runner.Object, len(p.Objects))
	for i, o := range p.Objects {
		if o == nil {
			objs[i] = &runner.Object{Name: fmt.Sprintf("<deleted %d>", i)}
			continue
		}
		ro := &runner.Object{Name: o.Name, Solid: o.Solid, Visible: o.Visible, Depth: int(o.Depth), Persistent: o.Persistent}
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
			if s.Precise {
				ro.Pixels = pixelMasks(s)
			}
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
	g.IDs = &runner.IDs{Sprites: map[string]int{}, Of: map[string]map[string]int{}}
	for id, x := range p.Sprites {
		if x != nil {
			g.IDs.Sprites[x.Name] = id
		}
	}
	var snd, bg, tl, pa []string
	g.Paths = map[string]*runner.Path{}
	for _, x := range p.Paths {
		pa = append(pa, nameOf(x != nil, func() string { return x.Name }))
		if x != nil {
			g.Paths[x.Name] = &runner.Path{Name: x.Name, Points: x.Points, Smooth: x.Smooth, Closed: x.Closed, Precision: int(x.Precision)}
			g.Resources[x.Name] = true
		}
	}
	for _, x := range p.Sounds {
		snd = append(snd, nameOf(x != nil, func() string { return x.Name }))
	}
	for _, x := range p.Backgrounds {
		bg = append(bg, nameOf(x != nil, func() string { return x.Name }))
	}
	for _, x := range p.Timelines {
		tl = append(tl, nameOf(x != nil, func() string { return x.Name }))
	}
	for kind, names := range map[string][]string{"sound": snd, "background": bg, "timeline": tl, "path": pa} {
		g.IDs.Of[kind] = map[string]int{}
		for id, n := range names {
			if n != "" {
				g.IDs.Of[kind][n] = id
			}
		}
	}
	for _, o := range g.Objects {
		o.IDs = g.IDs
	}
	g.RoomIDs = map[string]int{}
	for id, x := range p.Rooms {
		if x != nil {
			g.RoomIDs[x.Name] = id
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
	g.Timelines = map[string]*runner.Timeline{}
	for _, t := range p.Timelines {
		if t == nil {
			continue
		}
		rt := &runner.Timeline{Name: t.Name, Moments: map[int]runner.Handler{}}
		for _, m := range t.Moments {
			src, err := p.ActionsToGML(m.Actions)
			if err != nil {
				return nil, fmt.Errorf("timeline %s moment %d: %w", t.Name, m.Step, err)
			}
			h, err := g.Code(src)
			if err != nil {
				return nil, fmt.Errorf("timeline %s moment %d: %w", t.Name, m.Step, err)
			}
			rt.Moments[int(m.Step)] = h
		}
		g.Timelines[t.Name] = rt
	}
	for name, code := range p.LibCode { // library actions' own code (filled while converting)
		if err := g.Script(name, code); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	return g, nil
}

func (g *Game) event(ro *runner.Object, o *gmfile.GMObject, ev gmfile.Event) error {
	file := "objects/" + o.Name + "/" + g.Project.EventFile(ev) // names the code in errors; picks its language
	src, ok := ev.Source()
	if !ok {
		var err error
		if src, err = g.Project.ActionsToGML(ev.Actions); err != nil {
			return err
		}
	}
	unsupported := func() { g.Unsupported = append(g.Unsupported, fmt.Sprintf("%s: %s %d", o.Name, ev.Type, ev.Sub)) }
	e := &ro.Events
	switch ev.Type {
	case "collision":
		h, err := g.CollisionCodeIn(file, src)
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
	h, err := g.CodeIn(file, src)
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
		case 1, 2, 3, 7, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25:
			if e.Other == nil {
				e.Other = map[int]runner.Handler{}
			}
			e.Other[int(ev.Sub)] = h
		default:
			unsupported()
		}
	case "destroy":
		e.Destroy = h
	case "draw":
		e.Draw = h
	case "keyrelease":
		if e.KeyRelease == nil {
			e.KeyRelease = map[runner.Key]runner.Handler{}
		}
		e.KeyRelease[runner.Key(ev.Sub)] = h
	case "mouse":
		if e.Mouse == nil {
			e.Mouse = map[int]runner.Handler{}
		}
		e.Mouse[int(ev.Sub)] = h
	default:
		unsupported()
	}
	return nil
}

// Room makes the project's room into a runner room with its instances
// (each one's create event run, in placement order).
func (g *Game) Room(name string, glob *runner.Globals) (*runner.Room, error) {
	return g.room(name, glob, false, nil)
}

// room makes a room; gameStart runs the game start event (the first room)
// after the creation code, before the room start — GM 5's order.
//
// carry are the persistent instances coming from the room left: they are in
// the new room before its own instances are made (ASSUMPTION [engine]: first
// in the order, as the older instances), keep their state, run no create
// event, and get the room start event with the rest.
func (g *Game) room(name string, glob *runner.Globals, gameStart bool, carry []*runner.Instance) (*runner.Room, error) {
	for _, r := range g.Project.Rooms {
		if r == nil || r.Name != name {
			continue
		}
		rr := &runner.Room{Name: r.Name, Caption: r.Caption, W: int(r.W), H: int(r.H), Speed: int(r.Speed), G: glob,
			FrameCount: g.frameCount}
		rr.ViewsEnabled, rr.Persistent = r.ViewsEnabled, r.Persistent
		for _, v := range r.Views {
			follow := ""
			if v.Follow >= 0 && int(v.Follow) < len(g.Objects) {
				follow = g.Objects[v.Follow].Name
			}
			rr.Views = append(rr.Views, runner.View{Visible: v.Visible, X: int(v.X), Y: int(v.Y), W: int(v.W), H: int(v.H),
				PortX: int(v.PortX), PortY: int(v.PortY), PortW: int(v.PortW), PortH: int(v.PortH),
				BorderH: int(v.HBorder), BorderV: int(v.VBorder), SpeedH: int(v.HSpeed), SpeedV: int(v.VSpeed), Follow: follow})
		}
		for _, b := range r.Backgrounds {
			name := ""
			if b.BG >= 0 && int(b.BG) < len(g.Project.Backgrounds) && g.Project.Backgrounds[b.BG] != nil {
				name = g.Project.Backgrounds[b.BG].Name
			}
			rr.Backgrounds = append(rr.Backgrounds, runner.Background{Name: name, Visible: b.Visible, Foreground: b.Foreground,
				X: float64(b.X), Y: float64(b.Y), HSpeed: float64(b.HSpeed), VSpeed: float64(b.VSpeed), TileH: b.TileH, TileV: b.TileV, Stretch: b.Stretch})
		}
		rr.Instances = append(rr.Instances, carry...)
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
		if gameStart {
			rr.Every(func(e *runner.Events) runner.Handler { return e.Other[2] })
		}
		rr.Start()
		return rr, nil
	}
	var names []string
	for _, r := range g.Project.Rooms {
		if r != nil {
			names = append(names, r.Name)
		}
	}
	return nil, fmt.Errorf("no room %q (the rooms: %s)", name, strings.Join(names, ", "))
}

// frameCount is a sprite's number of frames.
func (g *Game) frameCount(sprite string) int {
	for _, s := range g.Project.Sprites {
		if s != nil && s.Name == sprite {
			return max(len(s.Frames), len(s.Images))
		}
	}
	return 0
}

// nameOf is a resource's name ("" for a gap in the ids).
func nameOf(ok bool, name func() string) string {
	if !ok {
		return ""
	}
	return name()
}

// pixelMasks are a precise sprite's frames as masks: the pixels that are not
// transparent, placed relative to the instance (the origin off).
func pixelMasks(s *gmfile.Sprite) []*runner.PixelMask {
	imgs := append([]*image.NRGBA(nil), s.Images...)
	for _, f := range s.Frames {
		if img, err := gmfile.DecodeBMP(f, s.Transparent); err == nil {
			imgs = append(imgs, img)
		}
	}
	var out []*runner.PixelMask
	for _, img := range imgs {
		b := img.Bounds()
		m := &runner.PixelMask{X0: -int(s.OriginX), Y0: -int(s.OriginY), W: b.Dx(), H: b.Dy(), Bits: make([]bool, b.Dx()*b.Dy())}
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				m.Bits[y*m.W+x] = img.NRGBAAt(b.Min.X+x, b.Min.Y+y).A > 0
			}
		}
		out = append(out, m)
	}
	return out
}
