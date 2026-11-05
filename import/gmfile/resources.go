package gmfile

import "image"

// The rest of a GM 5.3 project after the sprites, in gm53-reader's order:
// backgrounds, paths, scripts, data files, timelines, objects, rooms, game
// information, library init code, room order.

// sticky wraps a Reader so a run of reads stops at the first error.
type sticky struct {
	r   *Reader
	err error
}

func (s *sticky) i() int32 {
	if s.err != nil {
		return 0
	}
	v, err := s.r.I4()
	s.err = err
	return v
}
func (s *sticky) b() bool { return s.i() != 0 }
func (s *sticky) str() string {
	if s.err != nil {
		return ""
	}
	v, err := s.r.String()
	s.err = err
	return v
}
func (s *sticky) dbl() float64 {
	if s.err != nil {
		return 0
	}
	v, err := s.r.Double()
	s.err = err
	return v
}
func (s *sticky) zblob() []byte {
	if s.err != nil {
		return nil
	}
	v, err := s.r.ZBlob()
	s.err = err
	return v
}
func (s *sticky) actions() []Action {
	if s.err != nil {
		return nil
	}
	v, err := ReadActions(s.r)
	s.err = err
	return v
}

// list reads a resource list: version, count, then each entry if present.
func (s *sticky) list(each func(i int)) {
	s.i()
	n := s.i()
	for i := 0; s.err == nil && i < int(n); i++ {
		if !s.b() {
			each(-1 - i) // a deleted entry
			continue
		}
		each(i)
	}
}

// Background is a background resource (Image: the stored BMP).
type Background struct {
	ID      int
	Name    string
	W, H    int32
	Tileset bool
	Tile    [6]int32
	Image   []byte
	Img     *image.NRGBA // decoded (a text project's; nil: decode Image)
}

// Path is a path resource.
type Path struct {
	ID             int
	Name           string
	Smooth, Closed bool
	Precision      int32
	Points         [][3]float64 // x, y, speed
}

// Script is a script resource.
type Script struct {
	ID   int
	Name string
	Code string
}

// DataFile is a data file stored in the game.
type DataFile struct {
	ID                      int
	Name                    string
	Filename                string
	Data                    []byte
	Export                  int32
	Overwrite, Free, Remove bool
}

// Moment is a timeline step and its actions.
type Moment struct {
	Step    int32
	Actions []Action
}

// Timeline is a timeline resource.
type Timeline struct {
	ID      int
	Name    string
	Moments []Moment
}

// EventTypes are the event types by number.
var EventTypes = []string{"create", "destroy", "alarm", "step", "collision", "keyboard",
	"mouse", "other", "draw", "keypress", "keyrelease"}

// Event is one event of an object: its type (EventTypes), sub-number (alarm
// number, key, other object …) and actions.
type Event struct {
	Type    string
	Sub     int32
	Actions []Action
	// Lang is the event's language when its code is not GML: the code
	// file's extension (".js", ".lua", ".go"), its code the one code
	// action's text. "" is GML (and every GM 5 file's event).
	Lang string
}

// CodeExts are the extensions an event or script file may have: GML and the
// languages runner.Langs may know.
var CodeExts = []string{".gml", ".js", ".lua", ".go"}

// GMObject is an object resource (ids: -1 none).
type GMObject struct {
	ID                         int
	Name                       string
	Sprite                     int32
	Solid, Visible, Persistent bool
	Depth, Parent, Mask        int32
	Events                     []Event
}

// RoomBackground is one of a room's background layers.
type RoomBackground struct {
	Visible, Foreground bool
	BG, X, Y            int32
	TileH, TileV        bool
	HSpeed, VSpeed      int32
	Stretch             bool
}

// View is one of a room's views.
type View struct {
	Visible                          bool
	X, Y, W, H                       int32
	PortX, PortY, PortW, PortH       int32
	HBorder, VBorder, HSpeed, VSpeed int32
	Follow                           int32
}

// RoomInstance is an instance placed in a room.
type RoomInstance struct {
	X, Y, Obj, ID int32
	Code          string
}

// Tile is a background tile placed in a room.
type Tile struct {
	X, Y, BG, SrcX, SrcY, W, H, Depth, ID int32
}

// Room is a room resource.
type Room struct {
	ID            int
	Name, Caption string
	W, H, Speed   int32
	Persistent    bool
	BGColor       int32
	DrawBGColor   bool
	CreationCode  string
	Backgrounds   []RoomBackground
	ViewsEnabled  bool
	Views         []View
	Instances     []RoomInstance
	Tiles         []Tile
}

func (p *Project) readRest(r *Reader) error {
	s := &sticky{r: r}
	s.list(func(i int) {
		if i < 0 {
			p.Backgrounds = append(p.Backgrounds, nil)
			return
		}
		b := &Background{ID: i, Name: s.str()}
		v := s.i()
		b.W, b.H = s.i(), s.i()
		s.b() // transparent
		if v > 400 {
			s.b()
			s.b()
			b.Tileset = s.b()
			for k := range b.Tile {
				b.Tile[k] = s.i()
			}
		} else {
			s.i()
			s.b()
		}
		if s.b() && s.i() != -1 {
			b.Image = s.zblob()
		}
		p.Backgrounds = append(p.Backgrounds, b)
	})
	s.list(func(i int) {
		if i < 0 {
			p.Paths = append(p.Paths, nil)
			return
		}
		pa := &Path{ID: i, Name: s.str()}
		s.i()
		pa.Smooth, pa.Closed = s.b(), s.b()
		pa.Precision = s.i()
		s.i()
		s.i()
		s.i() // room, snap x, y
		for n := s.i(); s.err == nil && n > 0; n-- {
			pa.Points = append(pa.Points, [3]float64{s.dbl(), s.dbl(), s.dbl()})
		}
		p.Paths = append(p.Paths, pa)
	})
	s.list(func(i int) {
		if i < 0 {
			p.Scripts = append(p.Scripts, nil)
			return
		}
		sc := &Script{ID: i, Name: s.str()}
		s.i()
		sc.Code = s.str()
		p.Scripts = append(p.Scripts, sc)
	})
	if v := s.i(); s.err == nil && v != 440 {
		return r.errf("data files version %d (only 440 is known)", v)
	}
	for n := s.i(); s.err == nil && len(p.DataFiles) < int(n); {
		i := len(p.DataFiles)
		if !s.b() {
			p.DataFiles = append(p.DataFiles, nil)
			continue
		}
		d := &DataFile{ID: i, Name: s.str()}
		s.i()
		d.Filename = s.str()
		if s.b() {
			d.Data = s.zblob()
		}
		d.Export = s.i()
		d.Overwrite, d.Free, d.Remove = s.b(), s.b(), s.b()
		p.DataFiles = append(p.DataFiles, d)
	}
	s.list(func(i int) {
		if i < 0 {
			p.Timelines = append(p.Timelines, nil)
			return
		}
		t := &Timeline{ID: i, Name: s.str()}
		s.i()
		for n := s.i(); s.err == nil && n > 0; n-- {
			step := s.i()
			t.Moments = append(t.Moments, Moment{Step: step, Actions: s.actions()})
		}
		p.Timelines = append(p.Timelines, t)
	})
	s.list(func(i int) {
		if i < 0 {
			p.Objects = append(p.Objects, nil)
			return
		}
		o := &GMObject{ID: i, Name: s.str()}
		s.i()
		o.Sprite, o.Solid, o.Visible = s.i(), s.b(), s.b()
		o.Depth, o.Persistent, o.Parent, o.Mask = s.i(), s.b(), s.i(), s.i()
		nev := s.i() + 1
		for j := 0; s.err == nil && j < int(nev); j++ {
			for s.err == nil {
				sub := s.i()
				if sub == -1 {
					break
				}
				typ := "?"
				if j < len(EventTypes) {
					typ = EventTypes[j]
				}
				o.Events = append(o.Events, Event{Type: typ, Sub: sub, Actions: s.actions()})
			}
		}
		p.Objects = append(p.Objects, o)
	})
	s.list(func(i int) {
		if i < 0 {
			p.Rooms = append(p.Rooms, nil)
			return
		}
		rm := &Room{ID: i, Name: s.str()}
		v := s.i()
		rm.Caption = s.str()
		rm.W, rm.H = s.i(), s.i()
		s.i()
		s.i() // snap
		s.b() // isometric
		rm.Speed, rm.Persistent = s.i(), s.b()
		rm.BGColor, rm.DrawBGColor = s.i(), s.b()
		rm.CreationCode = s.str()
		for n := s.i(); s.err == nil && n > 0; n-- {
			rm.Backgrounds = append(rm.Backgrounds, RoomBackground{Visible: s.b(), Foreground: s.b(), BG: s.i(),
				X: s.i(), Y: s.i(), TileH: s.b(), TileV: s.b(), HSpeed: s.i(), VSpeed: s.i(), Stretch: s.b()})
		}
		rm.ViewsEnabled = s.b()
		for n := s.i(); s.err == nil && n > 0; n-- {
			vw := View{Visible: s.b(), X: s.i(), Y: s.i(), W: s.i(), H: s.i(), PortX: s.i(), PortY: s.i()}
			if v > 520 {
				vw.PortW, vw.PortH = s.i(), s.i()
			}
			vw.HBorder, vw.VBorder, vw.HSpeed, vw.VSpeed = s.i(), s.i(), s.i(), s.i()
			vw.Follow = s.i()
			rm.Views = append(rm.Views, vw)
		}
		for n := s.i(); s.err == nil && n > 0; n-- {
			in := RoomInstance{X: s.i(), Y: s.i(), Obj: s.i(), ID: s.i(), Code: s.str()}
			s.b() // locked
			rm.Instances = append(rm.Instances, in)
		}
		for n := s.i(); s.err == nil && n > 0; n-- {
			t := Tile{X: s.i(), Y: s.i(), BG: s.i(), SrcX: s.i(), SrcY: s.i(), W: s.i(), H: s.i(), Depth: s.i(), ID: s.i()}
			s.b()
			rm.Tiles = append(rm.Tiles, t)
		}
		s.b()
		s.i()
		s.i()
		for range 8 {
			s.b()
		}
		if v == 520 {
			for range 6 {
				s.i()
			}
		}
		s.i()
		s.i()
		s.i()
		p.Rooms = append(p.Rooms, rm)
	})
	p.LastInstanceID, p.LastTileID = s.i(), s.i()
	gv := s.i()
	s.i() // background colour
	s.b() // embed
	if gv > 430 {
		p.InfoCaption = s.str()
		for range 4 {
			s.i()
		}
		for range 4 {
			s.b()
		}
	}
	p.Information = s.str()
	s.i()
	for n := s.i(); s.err == nil && n > 0; n-- {
		p.LibraryInit = append(p.LibraryInit, s.str())
	}
	s.i()
	for n := s.i(); s.err == nil && n > 0; n-- {
		p.RoomOrder = append(p.RoomOrder, s.i())
	}
	return s.err
}
