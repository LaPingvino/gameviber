package runner

import (
	"fmt"
	"math"
	"strings"

	"github.com/LaPingvino/gameviber/gml"
)

// The interpreter's view of instances and globals (gml.Instance): Game
// Maker's built-in instance variables map to the fields, the rest to the
// instance's own variables. Arrays are kept as "name[i,j]".
//
// PARTIAL: sprite_index and object_index need the resource tables (the
// importer, step 3d): until then they read as the names, as strings.

func varKey(name string, idx []int) string {
	switch len(idx) {
	case 0:
		return name
	case 1:
		return fmt.Sprintf("%s[%d]", name, idx[0])
	}
	return fmt.Sprintf("%s[%d,%d]", name, idx[0], idx[1])
}

// Get reads a variable of the instance.
func (in *Instance) Get(name string, idx []int) (gml.Value, bool) {
	if idx == nil {
		if v, ok := in.builtin(name); ok {
			return gml.RealV(v), true
		}
		switch name {
		case "timeline_index": // PARTIAL: by name until ids; -1 none
			if in.Timeline == nil {
				return gml.RealV(-1), true
			}
			return gml.StrV(in.Timeline.Name), true
		case "sprite_index":
			return gml.StrV(in.Sprite), true
		case "object_index":
			return gml.StrV(in.Obj.Name), true
		}
	}
	if name == "alarm" && len(idx) == 1 && idx[0] >= 0 && idx[0] < len(in.Alarm) {
		return gml.RealV(float64(in.Alarm[idx[0]])), true
	}
	k := varKey(name, idx)
	if s, ok := in.VarsStr[k]; ok {
		return gml.StrV(s), true
	}
	if v, ok := in.Vars[k]; ok {
		return gml.RealV(v), true
	}
	return gml.Value{}, false
}

// Set writes a variable of the instance.
func (in *Instance) Set(name string, idx []int, v gml.Value) {
	if idx == nil && !v.IsStr && in.setBuiltin(name, v.N) {
		return
	}
	if idx == nil && name == "sprite_index" && v.IsStr { // PARTIAL: sprites by name
		in.Sprite = v.S
		return
	}
	if name == "alarm" && len(idx) == 1 && idx[0] >= 0 && idx[0] < len(in.Alarm) {
		in.Alarm[idx[0]] = int(v.N)
		return
	}
	k := varKey(name, idx)
	if v.IsStr {
		delete(in.Vars, k)
		in.VarsStr[k] = v.S
		return
	}
	delete(in.VarsStr, k)
	in.Vars[k] = v.N
}

func (in *Instance) builtin(name string) (float64, bool) {
	switch name {
	case "id":
		return float64(in.ID), true
	case "x":
		return in.X, true
	case "y":
		return in.Y, true
	case "xprevious":
		return in.XPrevious, true
	case "yprevious":
		return in.YPrevious, true
	case "xstart":
		return in.XStart, true
	case "ystart":
		return in.YStart, true
	case "hspeed":
		return in.HSpeed, true
	case "vspeed":
		return in.VSpeed, true
	case "speed":
		return math.Hypot(in.HSpeed, in.VSpeed), true
	case "direction":
		return in.Direction(), true
	case "gravity":
		return in.Gravity, true
	case "gravity_direction":
		return in.GravityDirection, true
	case "image_index", "image_single":
		return in.ImageIndex, true
	case "image_speed":
		return in.ImageSpeed, true
	case "depth":
		return in.Depth, true
	case "solid":
		return b2f(in.Solid), true
	case "timeline_position":
		return float64(in.TimelinePos), true
	}
	return 0, false
}

func (in *Instance) setBuiltin(name string, v float64) bool {
	switch name {
	case "x":
		in.X = v
	case "y":
		in.Y = v
	case "xprevious":
		in.XPrevious = v
	case "yprevious":
		in.YPrevious = v
	case "xstart":
		in.XStart = v
	case "ystart":
		in.YStart = v
	case "hspeed":
		in.HSpeed = v
	case "vspeed":
		in.VSpeed = v
	case "speed": // keeps the direction
		d := in.Direction() * math.Pi / 180
		in.HSpeed, in.VSpeed = v*math.Cos(d), -v*math.Sin(d)
	case "direction": // keeps the speed
		s, d := math.Hypot(in.HSpeed, in.VSpeed), v*math.Pi/180
		in.HSpeed, in.VSpeed = s*math.Cos(d), -s*math.Sin(d)
	case "gravity":
		in.Gravity = v
	case "gravity_direction":
		in.GravityDirection = v
	case "image_index", "image_single":
		in.ImageIndex = v
	case "image_speed":
		in.ImageSpeed = v
	case "depth":
		in.Depth = v
	case "timeline_position":
		in.TimelinePos = int(v)
	case "solid":
		in.Solid = v > 0.5
	default:
		return false
	}
	return true
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// GlobalVars is the interpreter's view of the globals: score and lives,
// then the global variables.
type GlobalVars struct {
	G   *Globals
	str map[string]string
	// Room is the current room (its background_* arrays).
	Room *Room
	// roomValue is how `room` reads (the game's ids).
	roomValue func(name string) gml.Value
}

// background reads or writes background_<field>[i] of the current room.
func (gv *GlobalVars) background(name string, idx []int, set *gml.Value) (gml.Value, bool) {
	if idx == nil && strings.HasPrefix(name, "background_") && name != "background_color" {
		idx = []int{0} // background_x alone is background_x[0]
	}
	if gv.Room == nil || len(idx) != 1 || idx[0] < 0 || idx[0] > 7 {
		return gml.Value{}, false
	}
	field, ok := strings.CutPrefix(name, "background_")
	if !ok {
		return gml.Value{}, false
	}
	for len(gv.Room.Backgrounds) <= idx[0] {
		gv.Room.Backgrounds = append(gv.Room.Backgrounds, Background{})
	}
	b := &gv.Room.Backgrounds[idx[0]]
	var f *float64
	var flag *bool
	switch field {
	case "x":
		f = &b.X
	case "y":
		f = &b.Y
	case "hspeed":
		f = &b.HSpeed
	case "vspeed":
		f = &b.VSpeed
	case "visible":
		flag = &b.Visible
	case "foreground":
		flag = &b.Foreground
	case "htiled":
		flag = &b.TileH
	case "vtiled":
		flag = &b.TileV
	case "index":
		if set != nil {
			b.Name = set.String()
		}
		return gml.StrV(b.Name), true // PARTIAL: by name until ids
	default:
		return gml.Value{}, false
	}
	if f != nil {
		if set != nil {
			*f = set.N
		}
		return gml.RealV(*f), true
	}
	if set != nil {
		*flag = set.True()
	}
	return gml.RealV(b2f(*flag)), true
}

// Get reads a global variable.
func (gv *GlobalVars) Get(name string, idx []int) (gml.Value, bool) {
	if v, ok := gv.background(name, idx, nil); ok {
		return v, true
	}
	if v, ok := gv.view(name, idx, nil); ok {
		return v, true
	}
	if idx == nil && name == "room" && gv.Room != nil { // its id (a number), as in GM 5
		if gv.roomValue != nil {
			return gv.roomValue(gv.Room.Name), true
		}
		return gml.StrV(gv.Room.Name), true
	}
	if idx == nil && name == "room_caption" && gv.Room != nil {
		return gml.StrV(gv.Room.Caption), true
	}
	if idx == nil && name == "room_persistent" && gv.Room != nil {
		return gml.RealV(b2f(gv.Room.Persistent)), true
	}
	if idx == nil && gv.Room != nil && (name == "room_width" || name == "room_height") {
		if name == "room_width" {
			return gml.RealV(float64(gv.Room.W)), true
		}
		return gml.RealV(float64(gv.Room.H)), true
	}
	if idx == nil {
		switch name {
		case "score":
			return gml.RealV(float64(gv.G.Score)), true
		case "lives":
			return gml.RealV(float64(gv.G.Lives)), true
		case "keyboard_string":
			return gml.StrV(gv.G.KeyboardString), true
		}
	}
	k := varKey(name, idx)
	if s, ok := gv.str[k]; ok {
		return gml.StrV(s), true
	}
	if v, ok := gv.G.Vars[k]; ok {
		return gml.RealV(v), true
	}
	if v, ok := gv.G.Vars[name]; ok && idx == nil && inputDefaults[name] {
		return gml.RealV(v), true
	}
	if idx == nil && inputDefaults[name] { // the input backend sets them; until then: nothing pressed
		return gml.RealV(0), true
	}
	return gml.Value{}, false
}

// inputDefaults are GM's input variables, 0 until an input backend sets them.
var inputDefaults = map[string]bool{"mouse_button": true, "mouse_x": true, "mouse_y": true,
	"keyboard_key": true, "keyboard_lastkey": true}

// Set writes a global variable (lives through AddLives, so "no more lives"
// is noticed).
func (gv *GlobalVars) Set(name string, idx []int, v gml.Value) {
	if idx == nil && name == "room_persistent" && gv.Room != nil {
		gv.Room.Persistent = v.True()
		return
	}
	if idx == nil && name == "room_caption" && gv.Room != nil {
		gv.Room.Caption = v.String()
		return
	}
	if _, ok := gv.view(name, idx, &v); ok {
		return
	}
	if _, ok := gv.background(name, idx, &v); ok {
		return
	}
	if idx == nil {
		switch name {
		case "score":
			gv.G.Score = int(v.N)
			return
		case "lives":
			gv.G.AddLives(int(v.N) - gv.G.Lives)
			return
		case "keyboard_string":
			gv.G.KeyboardString = v.S
			return
		}
	}
	if gv.str == nil {
		gv.str = map[string]string{}
	}
	k := varKey(name, idx)
	if v.IsStr {
		delete(gv.G.Vars, k)
		gv.str[k] = v.S
		return
	}
	delete(gv.str, k)
	gv.G.Vars[k] = v.N
}

// view reads or writes view_left / top / width / height / visible [i] of the
// current room (without an index: view 0).
func (gv *GlobalVars) view(name string, idx []int, set *gml.Value) (gml.Value, bool) {
	field, ok := strings.CutPrefix(name, "view_")
	if !ok || gv.Room == nil {
		return gml.Value{}, false
	}
	i := 0
	if len(idx) == 1 {
		i = idx[0]
	}
	if field == "enabled" {
		if set != nil {
			gv.Room.ViewsEnabled = set.True()
		}
		return gml.RealV(b2f(gv.Room.ViewsEnabled)), true
	}
	if i < 0 || i > 7 {
		return gml.Value{}, false
	}
	for len(gv.Room.Views) <= i {
		gv.Room.Views = append(gv.Room.Views, View{})
	}
	v := &gv.Room.Views[i]
	var p *int
	switch field {
	case "left", "xview":
		p = &v.X
	case "top", "yview":
		p = &v.Y
	case "width", "wview":
		p = &v.W
	case "height", "hview":
		p = &v.H
	case "visible":
		if set != nil {
			v.Visible = set.True()
		}
		return gml.RealV(b2f(v.Visible)), true
	default:
		return gml.Value{}, false
	}
	if set != nil {
		*p = int(set.N)
	}
	return gml.RealV(float64(*p)), true
}
