package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// The editor's rooms (docs/EDITOR.md, step E2): a room as JSON for the grid —
// its size and instances, the objects to place with their sprites' frame size
// and origin — and the instances saved back into room.toml (the room's other
// settings kept).

type editSprite struct {
	W, H, OX, OY int
}

type editObject struct {
	Name   string `json:"name"`
	Sprite string `json:"sprite"`
}

func (h editRooms) room(w http.ResponseWriter, r *http.Request) {
	p, ok := projectPath(h.dir, r.URL.Query().Get("path"))
	if !ok {
		http.Error(w, "not a room of this project", http.StatusBadRequest)
		return
	}
	var room map[string]any
	if _, err := toml.DecodeFile(filepath.Join(p, "room.toml"), &room); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	insts, _ := room["instance"].([]map[string]any)
	if insts == nil {
		insts = []map[string]any{}
	}
	writeJSON(w, map[string]any{
		"width": room["width"], "height": room["height"], "background": room["background_colour"],
		"caption": room["caption"], "speed": room["speed"], "persistent": room["persistent"],
		"instances": insts, "objects": h.objects(), "sprites": h.sprites(),
		"views_enabled": room["views_enabled"], "view0": firstView(room),
		"backgrounds": h.backgrounds(), "background0": firstBackground(room),
	})
}

// saveRoom replaces the room's instances with the posted ones (each a table:
// object, x, y and whatever else it carried), then reports the check.
func (h editRooms) saveRoom(w http.ResponseWriter, r *http.Request) {
	p, ok := projectPath(h.dir, r.URL.Query().Get("path"))
	if !ok {
		http.Error(w, "not a room of this project", http.StatusBadRequest)
		return
	}
	file := filepath.Join(p, "room.toml")
	var room map[string]any
	if _, err := toml.DecodeFile(file, &room); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var insts []map[string]any
	if err := json.NewDecoder(r.Body).Decode(&insts); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, in := range insts { // JSON numbers come as float64; positions are whole pixels
		for _, k := range []string{"x", "y"} {
			if f, ok := in[k].(float64); ok {
				in[k] = int64(f)
			}
		}
	}
	room["instance"] = insts
	f, err := os.Create(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = toml.NewEncoder(f).Encode(room)
	f.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, checkReport(h.dir))
}

// strip serves a sprite's strip.png.
func (h editRooms) strip(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("sprite")
	p, ok := projectPath(h.dir, "sprites/"+name+"/strip.png")
	if !ok || strings.ContainsAny(name, `/\`) {
		http.Error(w, "no such sprite", http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, p)
}

type editRooms struct{ dir string }

func (h editRooms) objects() []editObject {
	var out []editObject
	ents, _ := os.ReadDir(filepath.Join(h.dir, "objects"))
	for _, e := range ents {
		var o struct{ Sprite string }
		if _, err := toml.DecodeFile(filepath.Join(h.dir, "objects", e.Name(), "object.toml"), &o); err != nil {
			continue
		}
		if o.Sprite == "-1" {
			o.Sprite = ""
		}
		out = append(out, editObject{e.Name(), o.Sprite})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (h editRooms) sprites() map[string]editSprite {
	out := map[string]editSprite{}
	ents, _ := os.ReadDir(filepath.Join(h.dir, "sprites"))
	for _, e := range ents {
		var s struct {
			FrameW int   `toml:"frame_w"`
			FrameH int   `toml:"frame_h"`
			Origin []int `toml:"origin"`
		}
		if _, err := toml.DecodeFile(filepath.Join(h.dir, "sprites", e.Name(), "sprite.toml"), &s); err != nil {
			continue
		}
		sp := editSprite{W: s.FrameW, H: s.FrameH}
		if len(s.Origin) == 2 {
			sp.OX, sp.OY = s.Origin[0], s.Origin[1]
		}
		out[e.Name()] = sp
	}
	return out
}

// The editor's sprites (docs/EDITOR.md, step E4): sprite.toml as JSON for the
// sprite view, and its origin / bounding box / precise saved back (the other
// settings kept).
func (h editRooms) sprite(w http.ResponseWriter, r *http.Request) {
	p, ok := projectPath(h.dir, r.URL.Query().Get("path"))
	if !ok {
		http.Error(w, "not a sprite of this project", http.StatusBadRequest)
		return
	}
	var s map[string]any
	if _, err := toml.DecodeFile(filepath.Join(p, "sprite.toml"), &s); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s["name"] = filepath.Base(p)
	writeJSON(w, s)
}

func (h editRooms) saveSprite(w http.ResponseWriter, r *http.Request) {
	p, ok := projectPath(h.dir, r.URL.Query().Get("path"))
	if !ok {
		http.Error(w, "not a sprite of this project", http.StatusBadRequest)
		return
	}
	file := filepath.Join(p, "sprite.toml")
	var s map[string]any
	if _, err := toml.DecodeFile(file, &s); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var in struct {
		Origin   []int `json:"origin"`
		BBoxLRBT []int `json:"bbox_lrbt"`
		Precise  *bool `json:"precise"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(in.Origin) == 2 {
		s["origin"] = []int64{int64(in.Origin[0]), int64(in.Origin[1])}
	}
	if len(in.BBoxLRBT) == 4 {
		b := in.BBoxLRBT
		s["bbox_lrbt"] = []int64{int64(b[0]), int64(b[1]), int64(b[2]), int64(b[3])}
	}
	if in.Precise != nil {
		s["precise"] = *in.Precise
	}
	f, err := os.Create(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = toml.NewEncoder(f).Encode(s)
	f.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, checkReport(h.dir))
}

// importPNG (step E4b) takes a PNG as sprite name's picture: the strip
// (frames side by side, as many as asked; frame width = width / frames),
// sprite.toml made or updated — a new sprite gets its origin at 0, 0 and the
// whole frame as its box.
func (h editRooms) importPNG(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" || strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789_") != "" {
		http.Error(w, "a sprite name is letters a–z, digits and _", http.StatusBadRequest)
		return
	}
	frames, _ := strconv.Atoi(r.URL.Query().Get("frames"))
	frames = max(frames, 1)
	data, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		http.Error(w, "not a PNG: "+err.Error(), http.StatusBadRequest)
		return
	}
	if cfg.Width%frames != 0 {
		http.Error(w, fmt.Sprintf("the picture is %d wide: not %d frames of equal width", cfg.Width, frames), http.StatusBadRequest)
		return
	}
	fw, fh := cfg.Width/frames, cfg.Height
	dir := filepath.Join(h.dir, "sprites", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s := map[string]any{"origin": []int64{0, 0}, "bbox_lrbt": []int64{0, int64(fw - 1), int64(fh - 1), 0}, "precise": false, "transparent": true}
	toml.DecodeFile(filepath.Join(dir, "sprite.toml"), &s) // an existing sprite keeps its settings
	s["frames"], s["frame_w"], s["frame_h"] = int64(frames), int64(fw), int64(fh)
	if err := os.WriteFile(filepath.Join(dir, "strip.png"), data, 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	f, err := os.Create(filepath.Join(dir, "sprite.toml"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = toml.NewEncoder(f).Encode(s)
	f.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, checkReport(h.dir))
}

// newThing (the editor's + Object / + Room / + Event): an object with the
// starter's defaults, a room (640×480, added to game.toml's room order), or
// an object's event file (empty GML, its name as the reference has them:
// create, step, keyboard-37, collision-wall …). Nothing existing is
// overwritten.
func (h editRooms) newThing(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind, name := q.Get("kind"), q.Get("name")
	okName := func(s string, extra string) bool {
		return s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyz0123456789_"+extra) == ""
	}
	var path string
	var data []byte
	switch kind {
	case "object":
		if !okName(name, "") {
			http.Error(w, "an object's name is letters a–z, digits and _", http.StatusBadRequest)
			return
		}
		path = filepath.Join(h.dir, "objects", name, "object.toml")
		data = []byte("sprite = \"-1\"\nsolid = false\nvisible = true\ndepth = 0\npersistent = false\nparent = \"-1\"\nmask = \"-1\"\n")
	case "room":
		if !okName(name, "") {
			http.Error(w, "a room's name is letters a–z, digits and _", http.StatusBadRequest)
			return
		}
		path = filepath.Join(h.dir, "rooms", name, "room.toml")
		data = []byte(fmt.Sprintf("caption = %q\nwidth = 640\nheight = 480\nspeed = 30\npersistent = false\nbackground_colour = \"#20304A\"\ndraw_background_colour = true\nviews_enabled = false\n", name))
	case "event":
		obj := q.Get("object")
		if !okName(obj, "") || !okName(name, "-") {
			http.Error(w, "an event file is like step, create, keyboard-37, collision-wall", http.StatusBadRequest)
			return
		}
		if _, err := os.Stat(filepath.Join(h.dir, "objects", obj, "object.toml")); err != nil {
			http.Error(w, "no object "+obj, http.StatusBadRequest)
			return
		}
		path = filepath.Join(h.dir, "objects", obj, name+".gml")
		data = []byte("// " + obj + ": " + name + "\n")
	case "moment": // a timeline's moment: timelines/<timeline>/moment-<step>.gml (the timeline made with its first)
		tl := q.Get("object")
		step, err := strconv.Atoi(name)
		if !okName(tl, "") || err != nil || step < 0 || strconv.Itoa(step) != name {
			http.Error(w, "a moment is a timeline's name (a–z, 0–9, _) and a step (0, 1, 2 …)", http.StatusBadRequest)
			return
		}
		path = filepath.Join(h.dir, "timelines", tl, fmt.Sprintf("moment-%d.gml", step))
		data = []byte(fmt.Sprintf("// %s: step %d (from the timeline's start; set it going with timeline_index / action_set_timeline)\n", tl, step))
	default:
		http.Error(w, "kind is object, room, event or moment", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(path); err == nil {
		http.Error(w, "it exists already", http.StatusConflict)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if kind == "room" { // the room joins the game's room order
		gamePath := filepath.Join(h.dir, "game.toml")
		var g map[string]any
		if _, err := toml.DecodeFile(gamePath, &g); err == nil {
			order, _ := g["room_order"].([]any)
			g["room_order"] = append(order, name)
			if f, err := os.Create(gamePath); err == nil {
				toml.NewEncoder(f).Encode(g)
				f.Close()
			}
		}
	}
	rel, _ := filepath.Rel(h.dir, path)
	writeJSON(w, map[string]any{"path": filepath.ToSlash(rel), "check": checkReport(h.dir)})
}

// The editor's object settings (object.toml as a form): read as JSON with the
// sprites and objects to choose from; the settings saved back (other keys
// kept).
func (h editRooms) object(w http.ResponseWriter, r *http.Request) {
	p, ok := projectPath(h.dir, r.URL.Query().Get("path"))
	if !ok {
		http.Error(w, "not an object of this project", http.StatusBadRequest)
		return
	}
	var o map[string]any
	if _, err := toml.DecodeFile(filepath.Join(p, "object.toml"), &o); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var sprites []string
	for name := range h.sprites() {
		sprites = append(sprites, name)
	}
	sort.Strings(sprites)
	writeJSON(w, map[string]any{"name": filepath.Base(p), "settings": o, "sprites": sprites, "objects": h.objects(), "spriteInfo": h.sprites()})
}

func (h editRooms) saveObject(w http.ResponseWriter, r *http.Request) {
	p, ok := projectPath(h.dir, r.URL.Query().Get("path"))
	if !ok {
		http.Error(w, "not an object of this project", http.StatusBadRequest)
		return
	}
	file := filepath.Join(p, "object.toml")
	var o map[string]any
	if _, err := toml.DecodeFile(file, &o); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var in map[string]any
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, k := range []string{"sprite", "parent", "mask"} { // a name, or "-1" for none
		if v, ok := in[k].(string); ok {
			o[k] = v
		}
	}
	for _, k := range []string{"solid", "visible", "persistent"} {
		if v, ok := in[k].(bool); ok {
			o[k] = v
		}
	}
	if v, ok := in["depth"].(float64); ok {
		o["depth"] = int64(v)
	}
	f, err := os.Create(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = toml.NewEncoder(f).Encode(o)
	f.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, checkReport(h.dir))
}

// saveRoomSettings (the room view's settings): caption, size, speed,
// persistent, background colour — the instances and other keys kept.
func (h editRooms) saveRoomSettings(w http.ResponseWriter, r *http.Request) {
	p, ok := projectPath(h.dir, r.URL.Query().Get("path"))
	if !ok {
		http.Error(w, "not a room of this project", http.StatusBadRequest)
		return
	}
	file := filepath.Join(p, "room.toml")
	var room map[string]any
	if _, err := toml.DecodeFile(file, &room); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	var in map[string]any
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if v, ok := in["caption"].(string); ok {
		room["caption"] = v
	}
	for _, k := range []string{"width", "height", "speed"} {
		if v, ok := in[k].(float64); ok && v >= 1 && v <= 10000 {
			room[k] = int64(v)
		}
	}
	if v, ok := in["persistent"].(bool); ok {
		room["persistent"] = v
	}
	if v, ok := in["background_colour"].(string); ok && len(v) == 7 && v[0] == '#' {
		room["background_colour"] = strings.ToUpper(v)
	}
	// the camera (LOOP item 8): views on or off, and view 0 — its size (the
	// port the same, both at 0, 0), the object it follows, its borders; the
	// speed -1 (follows at once); any further views are kept
	if v, ok := in["views_enabled"].(bool); ok {
		room["views_enabled"] = v
	}
	if v0, ok := in["view0"].(map[string]any); ok {
		num := func(k string, lo, hi, def float64) int64 {
			if f, ok := v0[k].(float64); ok && f >= lo && f <= hi {
				return int64(f)
			}
			return int64(def)
		}
		vw, vh := num("w", 16, 4096, 320), num("h", 16, 4096, 240)
		view := map[string]any{"view": []int64{0, 0, vw, vh}, "port": []int64{0, 0, vw, vh},
			"border": []int64{num("hborder", 0, 4096, 64), num("vborder", 0, 4096, 64)}, "speed": []int64{-1, -1}}
		if f, _ := v0["follow"].(string); f != "" {
			if _, err := os.Stat(filepath.Join(h.dir, "objects", f, "object.toml")); err != nil || !safeName(f) {
				http.Error(w, "view: no object "+f+" to follow", http.StatusBadRequest)
				return
			}
			view["follow"] = f
		}
		views := []map[string]any{view}
		if old, ok := room["view"].([]map[string]any); ok && len(old) > 1 {
			views = append(views, old[1:]...)
		}
		room["view"] = views
	}
	// the background (LOOP item 8): room background 0 — an image of backgrounds/,
	// tiled across / down or stretched, scrolling, or in front; "" removes it
	if b0, ok := in["background0"].(map[string]any); ok {
		name, _ := b0["background"].(string)
		var bgs []map[string]any
		if old, ok := room["background"].([]map[string]any); ok && len(old) > 1 {
			bgs = old[1:]
		}
		if name != "" {
			if !safeName(name) {
				http.Error(w, "background: not a name", http.StatusBadRequest)
				return
			}
			if _, err := os.Stat(filepath.Join(h.dir, "backgrounds", name+".png")); err != nil {
				http.Error(w, "background: no backgrounds/"+name+".png", http.StatusBadRequest)
				return
			}
			flag := func(k string) bool { v, _ := b0[k].(bool); return v }
			num := func(k string) int64 {
				if f, ok := b0[k].(float64); ok && f >= -1000 && f <= 1000 {
					return int64(f)
				}
				return 0
			}
			bgs = append([]map[string]any{{"background": name, "x": int64(0), "y": int64(0), "tile_h": flag("tileh"), "tile_v": flag("tilev"),
				"stretch": flag("stretch"), "foreground": flag("foreground"), "hspeed": num("hspeed"), "vspeed": num("vspeed")}}, bgs...)
		}
		if len(bgs) > 0 {
			room["background"] = bgs
		} else {
			delete(room, "background")
		}
	}
	f, err := os.Create(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = toml.NewEncoder(f).Encode(room)
	f.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, checkReport(h.dir))
}

// firstView is view 0 of a room.toml, for the editor: its size, the object it
// follows and its borders (nil when the room has none).
func firstView(room map[string]any) map[string]any {
	views, _ := room["view"].([]map[string]any)
	if len(views) == 0 {
		return nil
	}
	v := views[0]
	out := map[string]any{"follow": v["follow"]}
	if a, ok := v["view"].([]any); ok && len(a) == 4 {
		out["w"], out["h"] = a[2], a[3]
	}
	if a, ok := v["border"].([]any); ok && len(a) == 2 {
		out["hborder"], out["vborder"] = a[0], a[1]
	}
	return out
}

// safeName is a resource name the editor may write into a file (no paths).
func safeName(s string) bool {
	for _, r := range s {
		if !(r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return s != ""
}

// backgrounds lists backgrounds/*.png by name.
func (h editRooms) backgrounds() []string {
	out := []string{}
	ents, _ := os.ReadDir(filepath.Join(h.dir, "backgrounds"))
	for _, e := range ents {
		if n, ok := strings.CutSuffix(e.Name(), ".png"); ok && !e.IsDir() {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// firstBackground is room background 0 of a room.toml, for the editor.
func firstBackground(room map[string]any) map[string]any {
	bgs, _ := room["background"].([]map[string]any)
	if len(bgs) == 0 {
		return nil
	}
	b := bgs[0]
	return map[string]any{"background": b["background"], "tileh": b["tile_h"], "tilev": b["tile_v"], "stretch": b["stretch"],
		"foreground": b["foreground"], "hspeed": b["hspeed"], "vspeed": b["vspeed"]}
}

// importBackground (POST /api/background-png?name=N): a PNG as backgrounds/N.png.
func (h editRooms) importBackground(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if !safeName(name) {
		http.Error(w, "a background name is letters, digits, - and _", http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		http.Error(w, "not a PNG: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(filepath.Join(h.dir, "backgrounds"), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(filepath.Join(h.dir, "backgrounds", name+".png"), data, 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, checkReport(h.dir))
}

// background (GET /api/background?name=N): backgrounds/N.png.
func (h editRooms) background(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if !safeName(name) {
		http.Error(w, "not a background name", http.StatusBadRequest)
		return
	}
	data, err := os.ReadFile(filepath.Join(h.dir, "backgrounds", name+".png"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
}
