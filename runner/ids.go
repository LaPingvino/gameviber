package runner

import "github.com/LaPingvino/gameviber/gml"

// IDs are the resources' GM ids (their index in the project) by name, for
// the kinds code passes around as numbers. The runner keeps resources by name
// inside; code sees their ids, as in GM 5. Shared by the game and its objects
// (an instance's sprite_index reads through its object).
type IDs struct {
	Sprites map[string]int
}

// SpriteValue is a sprite as code sees it: its id, or its name without one.
func (ids *IDs) SpriteValue(name string) gml.Value {
	if ids != nil {
		if id, ok := ids.Sprites[name]; ok {
			return gml.RealV(float64(id))
		}
	}
	return gml.StrV(name)
}

// SpriteName is the sprite a value names: an id, or a name (older projects).
// -1 (no sprite) is "".
func (ids *IDs) SpriteName(v gml.Value) string {
	if v.IsStr {
		return v.S
	}
	if ids != nil {
		for name, id := range ids.Sprites {
			if id == int(v.N) {
				return name
			}
		}
	}
	if v.N < 0 {
		return ""
	}
	return v.String()
}
