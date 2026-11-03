package runner

import (
	"fmt"
	"math"

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
}

// Get reads a global variable.
func (gv *GlobalVars) Get(name string, idx []int) (gml.Value, bool) {
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
	return gml.Value{}, false
}

// Set writes a global variable (lives through AddLives, so "no more lives"
// is noticed).
func (gv *GlobalVars) Set(name string, idx []int, v gml.Value) {
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
