package runner

import "github.com/LaPingvino/gameviber/gml"

// IDs are the resources' GM ids (their index in the project) by name, per
// kind ("sprite", "sound", "background", "timeline"). The runner keeps
// resources by name inside; code sees their ids, as in GM 5. Shared by the
// game and its objects (an instance's sprite_index reads through its object).
type IDs struct {
	Sprites map[string]int
	Of      map[string]map[string]int // the other kinds
}

func (ids *IDs) table(kind string) map[string]int {
	if ids == nil {
		return nil
	}
	if kind == "sprite" {
		return ids.Sprites
	}
	return ids.Of[kind]
}

// Value is a resource as code sees it: its id, or its name without one.
func (ids *IDs) Value(kind, name string) gml.Value {
	if id, ok := ids.table(kind)[name]; ok {
		return gml.RealV(float64(id))
	}
	return gml.StrV(name)
}

// Name is the resource a value names: an id, or a name (older projects).
// A negative id (-1: none) is "".
func (ids *IDs) Name(kind string, v gml.Value) string {
	if v.IsStr {
		return v.S
	}
	for name, id := range ids.table(kind) {
		if id == int(v.N) {
			return name
		}
	}
	if v.N < 0 {
		return ""
	}
	return v.String()
}

// Lookup finds a name among all kinds (a resource's name in code).
func (ids *IDs) Lookup(name string) (int, bool) {
	if ids == nil {
		return 0, false
	}
	if id, ok := ids.Sprites[name]; ok {
		return id, true
	}
	for _, t := range ids.Of {
		if id, ok := t[name]; ok {
			return id, true
		}
	}
	return 0, false
}

func (ids *IDs) SpriteValue(name string) gml.Value { return ids.Value("sprite", name) }
func (ids *IDs) SpriteName(v gml.Value) string     { return ids.Name("sprite", v) }
