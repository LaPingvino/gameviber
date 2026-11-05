package main

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/LaPingvino/gameviber/gml"
	"github.com/LaPingvino/gameviber/runner"
)

// instanceVarNames are the instance's built-in variables (as the reference
// lists them).
var instanceVarNames = []string{"x", "y", "xprevious", "yprevious", "xstart", "ystart", "hspeed", "vspeed", "speed",
	"direction", "gravity", "gravity_direction", "id", "image_index", "image_single", "image_speed", "depth",
	"sprite_index", "object_index", "solid", "alarm", "timeline_index", "timeline_position", "other", "self",
	"argument0", "argument1", "argument2", "argument3", "argument4", "argument5", "argument6", "argument7",
	"argument8", "argument9", "global"}

// editorNames is what the editor completes: every function GameViber has, the
// built-in variables, the constants, and this project's own resources.
type editorNames struct {
	Functions []string `json:"functions"`
	Variables []string `json:"variables"`
	Constants []string `json:"constants"`
	Resources []string `json:"resources"`
}

func projectNames(dir string) editorNames {
	keys := func(m map[string]bool) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	fn := map[string]bool{}
	for _, list := range (&runner.Game{}).FuncGroups() {
		for _, n := range list {
			fn[n] = true
		}
	}
	vars := map[string]bool{}
	for k := range gml.GlobalBuiltins {
		vars[k] = true
	}
	for _, k := range instanceVarNames {
		vars[k] = true
	}
	cons := map[string]bool{}
	for k := range gml.Constants() {
		cons[k] = true
	}
	res := map[string]bool{}
	if p, err := openVariant(dir, ""); err == nil {
		add := func(name string) {
			if name != "" {
				res[name] = true
			}
		}
		for _, x := range p.Objects {
			if x != nil {
				add(x.Name)
			}
		}
		for _, x := range p.Sprites {
			if x != nil {
				add(x.Name)
			}
		}
		for _, x := range p.Sounds {
			if x != nil {
				add(x.Name)
			}
		}
		for _, x := range p.Backgrounds {
			if x != nil {
				add(x.Name)
			}
		}
		for _, x := range p.Rooms {
			if x != nil {
				add(x.Name)
			}
		}
		for _, x := range p.Scripts {
			if x != nil {
				add(x.Name)
				fn[x.Name] = true
			}
		}
		for _, x := range p.Paths {
			if x != nil {
				add(x.Name)
			}
		}
		for _, x := range p.Timelines {
			if x != nil {
				add(x.Name)
			}
		}
	}
	return editorNames{Functions: keys(fn), Variables: keys(vars), Constants: keys(cons), Resources: keys(res)}
}

// namesHandlers: GET api/names, the editor's completions.
func namesHandlers(dir string, mux *http.ServeMux) {
	mux.HandleFunc("GET /api/names", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(projectNames(dir))
	})
}
