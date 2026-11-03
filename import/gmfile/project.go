package gmfile

import "fmt"

// Project is a GM 5.3 game's contents, as gm53-reader reads them (same
// fields; image and sound data kept as the bytes stored).
type Project struct {
	GameID   int32
	Settings Settings
	Sounds   []*Sound  // by id; nil where a resource was deleted
	Sprites  []*Sprite // by id
}

// Settings are the game's global settings (the subset the runner uses is
// named; the rest kept for round trips).
type Settings struct {
	Fullscreen, DontDrawBorder, DisplayCursor bool
	Scaling                                   int32
	F4F1EscF5                                 [4]bool // F4 fullscreen, F1 info, Esc ends, F5/F6 save
	Priority                                  int32
	FreezeOnLoseFocus                         bool
	Icon                                      []byte
	Author                                    string
	Version                                   int32
	LastChanged                               float64
	Information                               string
	Constants                                 [][2]string
}

// Sound is a sound resource: its file (ext, data) as stored.
type Sound struct {
	ID      int
	Name    string
	Kind    int32
	Ext     string
	Data    []byte
	Preload bool
}

// Sprite is a sprite resource; Frames are the stored images (BMP).
type Sprite struct {
	ID               int
	Name             string
	W, H             int32
	BBox             [4]int32 // left, right, bottom, top
	Transparent      bool
	Smooth, Preload  bool
	BBoxMode         int32
	Precise          bool
	OriginX, OriginY int32
	Frames           [][]byte
}

// Action is one drag-and-drop action as stored (library, id, its function
// or code, arguments, relative / applies-to / not).
type Action struct {
	Lib, ID, Kind                                 int32
	MayBeRelative, IsQuestion, AppliesToSomething bool
	ExecType                                      int32
	Function, Code                                string
	ArgC                                          int32
	ArgKinds                                      []int32
	AppliesTo                                     int32
	Relative                                      bool
	Args                                          []string
	Not                                           bool
}

// ReadProject reads the project body after the header (an opened Exe).
func (e *Exe) ReadProject() (*Project, error) {
	p := &Project{}
	r := e.Reader
	var err error
	must := func(f func() error) {
		if err == nil {
			err = f()
		}
	}
	i4 := func(dst *int32) func() error { return func() (e error) { *dst, e = r.I4(); return } }
	bl := func(dst *bool) func() error { return func() (e error) { *dst, e = r.Bool(); return } }
	str := func(dst *string) func() error { return func() (e error) { *dst, e = r.String(); return } }
	skip := func(n int) func() error {
		return func() error {
			for range n {
				if _, e := r.I4(); e != nil {
					return e
				}
			}
			return nil
		}
	}
	var reserved int32
	must(i4(&reserved))
	must(i4(&p.GameID))
	must(func() error { _, e := r.Blob(16, false); return e }) // the GUID
	st := &p.Settings
	var ver, loadBar int32
	var dummy bool
	must(i4(&ver))
	must(bl(&st.Fullscreen))
	must(bl(&st.DontDrawBorder))
	must(bl(&st.DisplayCursor))
	must(i4(&st.Scaling))
	must(skip(2))             // fullscreen scale and hardware
	must(bl(&dummy))          // set resolution
	must(skip(2 + 1 + 1 + 2)) // colour depth, exclusive; resolution; frequency; vblank, caption
	must(bl(&dummy))          // don't show buttons
	for i := range st.F4F1EscF5 {
		must(bl(&st.F4F1EscF5[i]))
	}
	must(skip(2))
	must(i4(&st.Priority))
	must(bl(&st.FreezeOnLoseFocus))
	must(i4(&loadBar))
	if err == nil && loadBar == 2 { // a custom load bar: two images
		for range 2 {
			var has int32
			must(i4(&has))
			if has != -1 {
				must(func() error { _, e := r.ZBlob(); return e })
			}
		}
	}
	var customLoad bool
	must(bl(&customLoad))
	if err == nil && customLoad {
		var has int32
		must(i4(&has))
		if has != -1 {
			must(func() error { _, e := r.ZBlob(); return e })
		}
	}
	must(bl(&dummy)) // load image transparent
	must(skip(1))    // its alpha
	must(bl(&dummy)) // scale progress bar
	must(func() error {
		n, e := r.I4()
		if e != nil {
			return e
		}
		st.Icon, e = r.Blob(int(n), false)
		return e
	})
	must(skip(3 + 1)) // display errors, log, abort; error flags
	must(str(&st.Author))
	must(i4(&st.Version))
	must(func() (e error) { st.LastChanged, e = r.Double(); return })
	must(str(&st.Information))
	must(func() error {
		n, e := r.I4()
		for i := int32(0); e == nil && i < n; i++ {
			var c [2]string
			if c[0], e = r.String(); e == nil {
				c[1], e = r.String()
			}
			st.Constants = append(st.Constants, c)
		}
		return e
	})
	must(func() error { return p.readSounds(r) })
	must(func() error { return p.readSprites(r) })
	return p, err
}

func (p *Project) readSounds(r *Reader) error {
	if _, err := r.I4(); err != nil { // list version
		return err
	}
	n, err := r.I4()
	if err != nil {
		return err
	}
	for i := 0; i < int(n); i++ {
		exists, err := r.Bool()
		if err != nil {
			return err
		}
		if !exists {
			p.Sounds = append(p.Sounds, nil)
			continue
		}
		s := &Sound{ID: i}
		if s.Name, err = r.String(); err != nil {
			return err
		}
		v, err := r.I4()
		if err != nil {
			return err
		}
		if v != 440 {
			return r.errf("sound %s: version %d (only 440 is known)", s.Name, v)
		}
		if s.Kind, err = r.I4(); err != nil {
			return err
		}
		if s.Ext, err = r.String(); err != nil {
			return err
		}
		if s.Kind != -1 {
			if s.Data, err = r.ZBlob(); err != nil {
				return err
			}
		}
		if _, err = r.I4(); err != nil {
			return err
		}
		if _, err = r.I4(); err != nil {
			return err
		}
		noPreload, err := r.Bool()
		if err != nil {
			return err
		}
		s.Preload = !noPreload
		p.Sounds = append(p.Sounds, s)
	}
	return nil
}

func (p *Project) readSprites(r *Reader) error {
	if _, err := r.I4(); err != nil {
		return err
	}
	n, err := r.I4()
	if err != nil {
		return err
	}
	for i := 0; i < int(n); i++ {
		exists, err := r.Bool()
		if err != nil {
			return err
		}
		if !exists {
			p.Sprites = append(p.Sprites, nil)
			continue
		}
		s := &Sprite{ID: i}
		if s.Name, err = r.String(); err != nil {
			return err
		}
		v, err := r.I4()
		if err != nil {
			return err
		}
		vals := []*int32{&s.W, &s.H, &s.BBox[0], &s.BBox[1], &s.BBox[2], &s.BBox[3]}
		for _, d := range vals {
			if *d, err = r.I4(); err != nil {
				return err
			}
		}
		if s.Transparent, err = r.Bool(); err != nil {
			return err
		}
		if v > 400 {
			if s.Smooth, err = r.Bool(); err != nil {
				return err
			}
			if s.Preload, err = r.Bool(); err != nil {
				return err
			}
		}
		if s.BBoxMode, err = r.I4(); err != nil {
			return err
		}
		if s.Precise, err = r.Bool(); err != nil {
			return err
		}
		if v == 400 {
			if _, err = r.I4(); err != nil { // video memory
				return err
			}
			noPreload, err := r.Bool()
			if err != nil {
				return err
			}
			s.Preload = !noPreload
		}
		if s.OriginX, err = r.I4(); err != nil {
			return err
		}
		if s.OriginY, err = r.I4(); err != nil {
			return err
		}
		nsub, err := r.I4()
		if err != nil {
			return err
		}
		for k := int32(0); k < nsub; k++ {
			has, err := r.I4()
			if err != nil {
				return err
			}
			if has == -1 {
				continue
			}
			img, err := r.ZBlob()
			if err != nil {
				return err
			}
			s.Frames = append(s.Frames, img)
		}
		p.Sprites = append(p.Sprites, s)
	}
	return nil
}

// ReadActions reads an action list (an event's or a timeline moment's).
func ReadActions(r *Reader) ([]Action, error) {
	if _, err := r.I4(); err != nil { // version
		return nil, err
	}
	n, err := r.I4()
	if err != nil {
		return nil, err
	}
	if n < 0 || n > 100000 {
		return nil, r.errf("bad action count %d", n)
	}
	acts := make([]Action, 0, n)
	for range n {
		var a Action
		steps := []func() error{
			func() (e error) { _, e = r.I4(); return },
			func() (e error) { a.Lib, e = r.I4(); return },
			func() (e error) { a.ID, e = r.I4(); return },
			func() (e error) { a.Kind, e = r.I4(); return },
			func() (e error) { a.MayBeRelative, e = r.Bool(); return },
			func() (e error) { a.IsQuestion, e = r.Bool(); return },
			func() (e error) { a.AppliesToSomething, e = r.Bool(); return },
			func() (e error) { a.ExecType, e = r.I4(); return },
			func() (e error) { a.Function, e = r.String(); return },
			func() (e error) { a.Code, e = r.String(); return },
			func() (e error) { a.ArgC, e = r.I4(); return },
			func() error {
				nk, e := r.I4()
				for i := int32(0); e == nil && i < nk; i++ {
					var k int32
					k, e = r.I4()
					a.ArgKinds = append(a.ArgKinds, k)
				}
				return e
			},
			func() (e error) { a.AppliesTo, e = r.I4(); return },
			func() (e error) { a.Relative, e = r.Bool(); return },
			func() error {
				na, e := r.I4()
				for i := int32(0); e == nil && i < na; i++ {
					var s string
					s, e = r.String()
					a.Args = append(a.Args, s)
				}
				return e
			},
			func() (e error) { a.Not, e = r.Bool(); return },
		}
		for _, f := range steps {
			if err := f(); err != nil {
				return nil, fmt.Errorf("action %d: %w", len(acts), err)
			}
		}
		acts = append(acts, a)
	}
	return acts, nil
}
