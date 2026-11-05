package gmfile

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// ReadDir reads a GameViber text project (Export's format) back into a
// Project, so the same loader runs it: each event's .gml becomes one code
// action; sprite strips are cut into frames (Images, not BMP bytes).
func ReadDir(dir string) (*Project, error) { return ReadFS(os.DirFS(dir), ".") }

// ReadFS is ReadDir on any file system (the web build reads the project
// packed into the page).
func ReadFS(fsys fs.FS, dir string) (*Project, error) {
	p := &Project{}
	var game struct {
		Name      string            `toml:"name"`
		RoomOrder []string          `toml:"room_order"`
		Constants map[string]string `toml:"constants"`
		Settings  struct {
			Icon string `toml:"icon"` // a sprite: the exported game's icon (its first frame)
		} `toml:"settings"`
	}
	if _, err := decodeTOML(fsys, path.Join(dir, "game.toml"), &game); err != nil {
		return nil, fmt.Errorf("game.toml: %w", err)
	}
	p.name = game.Name
	p.IconSprite = game.Settings.Icon
	keys := make([]string, 0, len(game.Constants))
	for k := range game.Constants {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p.Settings.Constants = append(p.Settings.Constants, [2]string{k, game.Constants[k]})
	}
	names := func(sub string) []string {
		ents, _ := fs.ReadDir(fsys, path.Join(dir, sub))
		var out []string
		for _, e := range ents {
			out = append(out, e.Name())
		}
		sort.Strings(out)
		return out
	}
	for _, n := range names("sprites") {
		s, err := readSprite(fsys, path.Join(dir, "sprites", n), n, len(p.Sprites))
		if err != nil {
			return nil, err
		}
		p.Sprites = append(p.Sprites, s)
	}
	for _, f := range names("backgrounds") {
		img, err := readPNG(fsys, path.Join(dir, "backgrounds", f))
		if err != nil {
			return nil, err
		}
		b := img.Bounds()
		p.Backgrounds = append(p.Backgrounds, &Background{ID: len(p.Backgrounds), Name: strings.TrimSuffix(f, ".png"), W: int32(b.Dx()), H: int32(b.Dy()), Img: img})
	}
	for _, f := range names("paths") { // ids by name order, as the other resources
		if !strings.HasSuffix(f, ".toml") {
			continue
		}
		var pt pathTOML
		if _, err := decodeTOML(fsys, path.Join(dir, "paths", f), &pt); err != nil {
			return nil, fmt.Errorf("paths/%s: %w", f, err)
		}
		if pt.Precision == 0 {
			pt.Precision = 4 // GameMaker's default
		}
		p.Paths = append(p.Paths, &Path{ID: len(p.Paths), Name: strings.TrimSuffix(f, ".toml"), Smooth: pt.Smooth, Closed: pt.Closed, Precision: pt.Precision, Points: pt.Points})
	}
	for _, f := range names("sounds") {
		data, err := fs.ReadFile(fsys, path.Join(dir, "sounds", f))
		if err != nil {
			return nil, err
		}
		ext := path.Ext(f)
		if base := strings.TrimSuffix(f, ext); ext == ".mid" && slices.Contains(names("sounds"), base+".opus") {
			continue // the music is played from its .opus (the .mid is kept for editing)
		}
		p.Sounds = append(p.Sounds, &Sound{ID: len(p.Sounds), Name: strings.TrimSuffix(f, ext), Ext: ext, Data: data})
	}
	for _, f := range names("scripts") {
		code, err := fs.ReadFile(fsys, path.Join(dir, "scripts", f))
		if err != nil {
			return nil, err
		}
		ext := path.Ext(f)
		if !slices.Contains(CodeExts, ext) {
			continue
		}
		sc := &Script{ID: len(p.Scripts), Name: strings.TrimSuffix(f, ext), Code: string(code)}
		if ext != ".gml" {
			sc.Lang = ext
		}
		p.Scripts = append(p.Scripts, sc)
	}
	for _, n := range names("timelines") {
		t := &Timeline{ID: len(p.Timelines), Name: n}
		for _, f := range names(path.Join("timelines", n)) {
			step, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(f, "moment-"), ".gml"))
			if err != nil {
				continue
			}
			code, err := fs.ReadFile(fsys, path.Join(dir, "timelines", n, f))
			if err != nil {
				return nil, err
			}
			t.Moments = append(t.Moments, Moment{Step: int32(step), Actions: codeAction(string(code))})
		}
		p.Timelines = append(p.Timelines, t)
	}
	// Objects: ids by name order; sprites, parents and collisions by name.
	objNames := names("objects")
	objID := map[string]int32{}
	for i, n := range objNames {
		objID[n] = int32(i)
	}
	sprID := map[string]int32{}
	for i, s := range p.Sprites {
		sprID[s.Name] = int32(i)
	}
	lookup := func(m map[string]int32, name string) int32 {
		if id, ok := m[name]; ok {
			return id
		}
		return -1
	}
	for _, n := range objNames {
		var meta struct {
			Sprite, Parent, Mask       string
			Solid, Visible, Persistent bool
			Depth                      int32
		}
		if _, err := decodeTOML(fsys, path.Join(dir, "objects", n, "object.toml"), &meta); err != nil {
			return nil, fmt.Errorf("objects/%s/object.toml: %w", n, err)
		}
		o := &GMObject{ID: len(p.Objects), Name: n, Sprite: lookup(sprID, meta.Sprite), Solid: meta.Solid, Visible: meta.Visible,
			Persistent: meta.Persistent, Depth: meta.Depth, Parent: lookup(objID, meta.Parent), Mask: lookup(sprID, meta.Mask)}
		for _, f := range names(path.Join("objects", n)) {
			ext := path.Ext(f)
			if !slices.Contains(CodeExts, ext) {
				continue
			}
			ev, ok := parseEventFile(strings.TrimSuffix(f, ext), objID)
			if !ok {
				return nil, fmt.Errorf("objects/%s/%s: not an event file name", n, f)
			}
			code, err := fs.ReadFile(fsys, path.Join(dir, "objects", n, f))
			if err != nil {
				return nil, err
			}
			ev.Actions = codeAction(string(code))
			if ext != ".gml" {
				ev.Lang = ext
			}
			o.Events = append(o.Events, ev)
		}
		p.Objects = append(p.Objects, o)
	}
	roomID := map[string]int32{}
	for _, n := range names("rooms") {
		r, err := readRoom(fsys, path.Join(dir, "rooms", n), n, len(p.Rooms), objID, p)
		if err != nil {
			return nil, err
		}
		roomID[n] = int32(len(p.Rooms))
		p.Rooms = append(p.Rooms, r)
	}
	for _, n := range game.RoomOrder {
		id, ok := roomID[n]
		if !ok {
			return nil, fmt.Errorf("game.toml: room_order names %q, which has no rooms/%s", n, n)
		}
		p.RoomOrder = append(p.RoomOrder, id)
	}
	return p, nil
}

// codeAction is one code action holding src (kind 7: ActionsToGML keeps it).
func codeAction(src string) []Action {
	return []Action{{Lib: 1, ID: 603, Kind: kindCode, AppliesTo: -1, ArgC: 1, Args: []string{src}}}
}

// parseEventFile reads an event file's name: create, step-begin,
// alarm-0, collision-<object>, keyboard-37 …
func parseEventFile(name string, objID map[string]int32) (Event, bool) {
	switch name {
	case "create", "destroy", "draw":
		return Event{Type: name}, true
	case "step":
		return Event{Type: "step", Sub: 0}, true
	case "step-begin":
		return Event{Type: "step", Sub: 1}, true
	case "step-end":
		return Event{Type: "step", Sub: 2}, true
	}
	typ, sub, ok := strings.Cut(name, "-")
	if !ok {
		return Event{}, false
	}
	if typ == "collision" {
		id, ok := objID[sub]
		return Event{Type: typ, Sub: id}, ok
	}
	n, err := strconv.Atoi(sub)
	return Event{Type: typ, Sub: int32(n)}, err == nil
}

func readPNG(fsys fs.FS, file string) (*image.NRGBA, error) {
	f, err := fsys.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	n := image.NewNRGBA(img.Bounds())
	draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
	return n, nil
}

func readSprite(fsys fs.FS, dir, name string, id int) (*Sprite, error) {
	var meta struct {
		Frames      int     `toml:"frames"`
		FrameW      int32   `toml:"frame_w"`
		FrameH      int32   `toml:"frame_h"`
		BBox        []int32 `toml:"bbox_lrbt"`
		Origin      []int32 `toml:"origin"`
		Precise     bool    `toml:"precise"`
		Transparent bool    `toml:"transparent"`
	}
	if _, err := decodeTOML(fsys, path.Join(dir, "sprite.toml"), &meta); err != nil {
		return nil, fmt.Errorf("sprites/%s/sprite.toml: %w", name, err)
	}
	s := &Sprite{ID: id, Name: name, W: meta.FrameW, H: meta.FrameH, Precise: meta.Precise, Transparent: meta.Transparent}
	copy(s.BBox[:], meta.BBox)
	if len(meta.Origin) == 2 {
		s.OriginX, s.OriginY = meta.Origin[0], meta.Origin[1]
	}
	if meta.Frames == 0 {
		return s, nil
	}
	strip, err := readPNG(fsys, path.Join(dir, "strip.png"))
	if err != nil {
		return nil, err
	}
	for i := range meta.Frames {
		f := image.NewNRGBA(image.Rect(0, 0, int(s.W), int(s.H)))
		draw.Draw(f, f.Bounds(), strip, image.Pt(i*int(s.W), 0), draw.Src)
		s.Images = append(s.Images, f)
	}
	return s, nil
}

func readRoom(fsys fs.FS, dir, name string, id int, objID map[string]int32, p *Project) (*Room, error) {
	var meta struct {
		Caption              string `toml:"caption"`
		Width, Height, Speed int32
		Persistent           bool
		BackgroundColour     string `toml:"background_colour"`
		DrawBackgroundColour bool   `toml:"draw_background_colour"`
		ViewsEnabled         bool   `toml:"views_enabled"`
		Instance             []struct {
			Object string
			X, Y   int32
			ID     int32
			Code   string
		}
		Background []struct {
			Background           string
			X, Y, HSpeed, VSpeed int32
			TileH                bool `toml:"tile_h"` // as export.go writes them (the untagged
			TileV                bool `toml:"tile_v"` // fields read "tileh" and lost every tiling)
			Foreground, Stretch  bool
		} `toml:"background"`
		View []struct {
			View, Port, Border, Speed []int32
			Follow                    string
		} `toml:"view"`
	}
	if _, err := decodeTOML(fsys, path.Join(dir, "room.toml"), &meta); err != nil {
		return nil, fmt.Errorf("rooms/%s/room.toml: %w", name, err)
	}
	r := &Room{ID: id, Name: name, Caption: meta.Caption, W: meta.Width, H: meta.Height, Speed: meta.Speed, Persistent: meta.Persistent,
		DrawBGColor: meta.DrawBackgroundColour, ViewsEnabled: meta.ViewsEnabled}
	if c, err := strconv.ParseUint(strings.TrimPrefix(meta.BackgroundColour, "#"), 16, 32); err == nil {
		r.BGColor = bgr2rgb(int32(c)) // the same swap both ways
	}
	for _, in := range meta.Instance {
		obj, ok := objID[in.Object]
		if !ok {
			return nil, fmt.Errorf("rooms/%s/room.toml: an instance of %q, which has no objects/%s", name, in.Object, in.Object)
		}
		r.Instances = append(r.Instances, RoomInstance{X: in.X, Y: in.Y, Obj: obj, ID: in.ID, Code: in.Code})
	}
	bgID := func(n string) int32 {
		for i, b := range p.Backgrounds {
			if b != nil && b.Name == n {
				return int32(i)
			}
		}
		return -1
	}
	for _, b := range meta.Background {
		r.Backgrounds = append(r.Backgrounds, RoomBackground{Visible: true, Foreground: b.Foreground, BG: bgID(b.Background), X: b.X, Y: b.Y,
			TileH: b.TileH, TileV: b.TileV, HSpeed: b.HSpeed, VSpeed: b.VSpeed, Stretch: b.Stretch})
	}
	for _, v := range meta.View {
		vw := View{Visible: true, Follow: -1}
		if len(v.View) == 4 {
			vw.X, vw.Y, vw.W, vw.H = v.View[0], v.View[1], v.View[2], v.View[3]
		}
		if len(v.Port) == 4 {
			vw.PortX, vw.PortY, vw.PortW, vw.PortH = v.Port[0], v.Port[1], v.Port[2], v.Port[3]
		}
		if len(v.Border) == 2 {
			vw.HBorder, vw.VBorder = v.Border[0], v.Border[1]
		}
		if len(v.Speed) == 2 {
			vw.HSpeed, vw.VSpeed = v.Speed[0], v.Speed[1]
		}
		if id, ok := objID[v.Follow]; ok {
			vw.Follow = id
		}
		r.Views = append(r.Views, vw)
	}
	if code, err := fs.ReadFile(fsys, path.Join(dir, "creation.gml")); err == nil {
		r.CreationCode = string(code)
	}
	return r, nil
}

// decodeTOML reads a TOML file from the file system.
func decodeTOML(fsys fs.FS, file string, v any) (toml.MetaData, error) {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return toml.MetaData{}, err
	}
	return toml.Decode(string(data), v)
}
