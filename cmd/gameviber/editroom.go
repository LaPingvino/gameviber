package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
		"instances": insts, "objects": h.objects(), "sprites": h.sprites(),
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
