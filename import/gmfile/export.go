package gmfile

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Export writes the project in GameViber's text format (docs/DESIGN.md §2):
// game.toml, sprites/<name>/strip.png + sprite.toml, backgrounds/<name>.png,
// sounds/<name><ext>, scripts/<name>.gml, objects/<name>/object.toml + one
// .gml per event, timelines/<name>/moment-<step>.gml, rooms/<name>/room.toml
// (+ creation.gml), AGENTS.md. Problems that don't stop the export (an image
// that won't decode) come back as notes.
func (p *Project) Export(dir, name string) (notes []string, err error) {
	w := &exporter{p: p, dir: dir}
	game := map[string]any{
		"name": name, "gm_version": 530,
		"settings": map[string]any{
			"fullscreen": p.Settings.Fullscreen, "display_cursor": p.Settings.DisplayCursor,
			"esc_ends_game": p.Settings.F4F1EscF5[2], "f4_fullscreen": p.Settings.F4F1EscF5[0],
			"author": p.Settings.Author, "version": p.Settings.Version,
		},
	}
	var order []string
	for _, id := range p.RoomOrder {
		if int(id) < len(p.Rooms) && p.Rooms[id] != nil {
			order = append(order, p.Rooms[id].Name)
		}
	}
	game["room_order"] = order
	if len(p.Settings.Constants) > 0 {
		cs := map[string]string{}
		for _, c := range p.Settings.Constants {
			cs[c[0]] = c[1]
		}
		game["constants"] = cs
	}
	w.toml("game.toml", game)
	if p.Information != "" {
		w.file("information.rtf", []byte(p.Information))
	}
	for _, s := range p.Sprites {
		if s != nil {
			w.sprite(s)
		}
	}
	for _, b := range p.Backgrounds {
		if b == nil || b.Image == nil {
			continue
		}
		img, err := DecodeBMP(b.Image, false)
		if err != nil {
			w.notes = append(w.notes, "background "+b.Name+": "+err.Error())
			continue
		}
		w.png(filepath.Join("backgrounds", b.Name+".png"), img)
	}
	for _, s := range p.Sounds {
		if s != nil && s.Data != nil {
			w.file(filepath.Join("sounds", s.Name+s.Ext), s.Data)
		}
	}
	for _, s := range p.Scripts {
		if s != nil {
			w.file(filepath.Join("scripts", s.Name+".gml"), []byte(s.Code))
		}
	}
	for _, o := range p.Objects {
		if o != nil {
			w.object(o)
		}
	}
	for _, t := range p.Timelines {
		if t == nil {
			continue
		}
		for _, m := range t.Moments {
			w.code(filepath.Join("timelines", t.Name, fmt.Sprintf("moment-%d.gml", m.Step)), m.Actions)
		}
	}
	for _, r := range p.Rooms {
		if r != nil {
			w.room(r)
		}
	}
	w.file("AGENTS.md", []byte(fmt.Sprintf(`# %s — for agents

Imported by GameViber from a GameMaker 5.3 game. The game's logic is in objects/<name>/<event>.gml (one
file per event: create, step, step-begin, step-end, alarm-N, collision-<object>, keyboard-N, keypress-N,
keyrelease-N, mouse-N, other-N, draw, destroy); drag-and-drop actions became action_*(…) calls (the last
argument is GameMaker's "relative" box when the action has one). Rooms are rooms/<name>/room.toml.
Check your changes with `+"`gameviber check`"+`; run with `+"`gameviber run`"+`.
`, name)))
	return w.notes, w.err
}

type exporter struct {
	p     *Project
	dir   string
	notes []string
	err   error
}

func (w *exporter) file(rel string, data []byte) {
	if w.err != nil {
		return
	}
	path := filepath.Join(w.dir, rel)
	if w.err = os.MkdirAll(filepath.Dir(path), 0o755); w.err == nil {
		w.err = os.WriteFile(path, data, 0o644)
	}
}

func (w *exporter) toml(rel string, v any) {
	var b bytes.Buffer
	if err := toml.NewEncoder(&b).Encode(v); err != nil && w.err == nil {
		w.err = fmt.Errorf("%s: %w", rel, err)
		return
	}
	w.file(rel, b.Bytes())
}

func (w *exporter) png(rel string, img image.Image) {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil && w.err == nil {
		w.err = err
		return
	}
	w.file(rel, b.Bytes())
}

func (w *exporter) sprite(s *Sprite) {
	var frames []*image.NRGBA
	for i, f := range s.Frames {
		img, err := DecodeBMP(f, s.Transparent)
		if err != nil {
			w.notes = append(w.notes, fmt.Sprintf("sprite %s frame %d: %v", s.Name, i, err))
			continue
		}
		frames = append(frames, img)
	}
	meta := map[string]any{"frames": len(frames), "frame_w": s.W, "frame_h": s.H,
		"origin": []int32{s.OriginX, s.OriginY}, "bbox_lrbt": s.BBox[:], "precise": s.Precise, "transparent": s.Transparent}
	w.toml(filepath.Join("sprites", s.Name, "sprite.toml"), meta)
	if len(frames) == 0 {
		return
	}
	strip := image.NewNRGBA(image.Rect(0, 0, int(s.W)*len(frames), int(s.H)))
	for i, f := range frames {
		draw.Draw(strip, f.Bounds().Add(image.Pt(i*int(s.W), 0)), f, image.Point{}, draw.Src)
	}
	w.png(filepath.Join("sprites", s.Name, "strip.png"), strip)
}

func (w *exporter) object(o *GMObject) {
	c := &converter{p: w.p}
	meta := map[string]any{"sprite": c.name(5, o.Sprite), "solid": o.Solid, "visible": o.Visible, "depth": o.Depth,
		"persistent": o.Persistent, "parent": c.name(10, o.Parent), "mask": c.name(5, o.Mask)}
	w.toml(filepath.Join("objects", o.Name, "object.toml"), meta)
	for _, ev := range o.Events {
		w.code(filepath.Join("objects", o.Name, w.eventFile(ev)), ev.Actions)
	}
}

// eventFile names an event's file.
func (w *exporter) eventFile(ev Event) string {
	sub := strconv.Itoa(int(ev.Sub))
	switch ev.Type {
	case "create", "destroy", "draw":
		return ev.Type + ".gml"
	case "step":
		return [...]string{"step.gml", "step-begin.gml", "step-end.gml"}[min(max(int(ev.Sub), 0), 2)]
	case "collision":
		return "collision-" + (&converter{p: w.p}).name(10, ev.Sub) + ".gml"
	}
	return ev.Type + "-" + sub + ".gml"
}

func (w *exporter) code(rel string, acts []Action) {
	src, err := w.p.ActionsToGML(acts)
	if err != nil {
		w.notes = append(w.notes, rel+": "+err.Error())
	}
	w.file(rel, []byte(src))
}

func (w *exporter) room(r *Room) {
	c := &converter{p: w.p}
	var insts []map[string]any
	for _, in := range r.Instances {
		m := map[string]any{"object": c.name(10, in.Obj), "x": in.X, "y": in.Y, "id": in.ID}
		if strings.TrimSpace(in.Code) != "" {
			m["code"] = in.Code
		}
		insts = append(insts, m)
	}
	meta := map[string]any{"caption": r.Caption, "width": r.W, "height": r.H, "speed": r.Speed,
		"persistent": r.Persistent, "background_colour": fmt.Sprintf("#%06X", bgr2rgb(r.BGColor)),
		"draw_background_colour": r.DrawBGColor, "views_enabled": r.ViewsEnabled, "instance": insts}
	var bgs []map[string]any
	for _, b := range r.Backgrounds {
		if b.Visible && b.BG >= 0 {
			bgs = append(bgs, map[string]any{"background": c.name(7, b.BG), "x": b.X, "y": b.Y, "tile_h": b.TileH,
				"tile_v": b.TileV, "hspeed": b.HSpeed, "vspeed": b.VSpeed, "foreground": b.Foreground, "stretch": b.Stretch})
		}
	}
	if bgs != nil {
		meta["background"] = bgs
	}
	var views []map[string]any
	for _, v := range r.Views {
		if v.Visible {
			views = append(views, map[string]any{"view": []int32{v.X, v.Y, v.W, v.H}, "port": []int32{v.PortX, v.PortY, v.PortW, v.PortH},
				"border": []int32{v.HBorder, v.VBorder}, "speed": []int32{v.HSpeed, v.VSpeed}, "follow": c.name(10, v.Follow)})
		}
	}
	if views != nil {
		meta["view"] = views
	}
	w.toml(filepath.Join("rooms", r.Name, "room.toml"), meta)
	if strings.TrimSpace(r.CreationCode) != "" {
		w.file(filepath.Join("rooms", r.Name, "creation.gml"), []byte(r.CreationCode))
	}
}

// bgr2rgb turns a Windows colour (0x00BBGGRR) into 0xRRGGBB.
func bgr2rgb(c int32) int32 {
	u := uint32(c)
	return int32((u&0xFF)<<16 | u&0xFF00 | (u>>16)&0xFF)
}
