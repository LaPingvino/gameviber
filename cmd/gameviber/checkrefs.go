package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// checkRefs is gameviber check's second half: the project's names point at
// things that exist — each room's instances name objects, each object's
// sprite / mask / parent exist, game.toml's room order names rooms. These are
// what an agent (or a person) most often gets wrong in the TOML files; the
// loader would stumble on them at run time. Each problem names its file and,
// for a near miss, the name meant.
func checkRefs(dir string, w io.Writer) int {
	list := func(sub, marker string) map[string]bool {
		out := map[string]bool{}
		ents, _ := os.ReadDir(filepath.Join(dir, sub))
		for _, e := range ents {
			if _, err := os.Stat(filepath.Join(dir, sub, e.Name(), marker)); err == nil {
				out[e.Name()] = true
			}
		}
		return out
	}
	objects, sprites, rooms := list("objects", "object.toml"), list("sprites", "sprite.toml"), list("rooms", "room.toml")
	problems := 0
	report := func(file, what, name string, known map[string]bool) {
		msg := fmt.Sprintf("%s: %s %q does not exist", file, what, name)
		if h := hint(name, known); h != "" && h[:3] == "did" {
			msg += " — " + h
		}
		fmt.Fprintln(w, msg)
		problems++
	}
	sorted := func(m map[string]bool) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	for _, o := range sorted(objects) {
		file := filepath.ToSlash(filepath.Join("objects", o, "object.toml"))
		var meta struct{ Sprite, Mask, Parent string }
		if _, err := toml.DecodeFile(filepath.Join(dir, file), &meta); err != nil {
			fmt.Fprintf(w, "%s: %v\n", file, err)
			problems++
			continue
		}
		if meta.Sprite != "" && meta.Sprite != "-1" && !sprites[meta.Sprite] {
			report(file, "sprite", meta.Sprite, sprites)
		}
		if meta.Mask != "" && meta.Mask != "-1" && !sprites[meta.Mask] {
			report(file, "mask sprite", meta.Mask, sprites)
		}
		if meta.Parent != "" && meta.Parent != "-1" && !objects[meta.Parent] {
			report(file, "parent object", meta.Parent, objects)
		}
	}
	for _, rm := range sorted(rooms) {
		file := filepath.ToSlash(filepath.Join("rooms", rm, "room.toml"))
		var r struct {
			Instance []struct{ Object string }
		}
		if _, err := toml.DecodeFile(filepath.Join(dir, file), &r); err != nil {
			fmt.Fprintf(w, "%s: %v\n", file, err)
			problems++
			continue
		}
		seen := map[string]bool{}
		for _, in := range r.Instance {
			if !objects[in.Object] && !seen[in.Object] {
				seen[in.Object] = true
				report(file, "an instance's object", in.Object, objects)
			}
		}
	}
	var g struct {
		RoomOrder []string `toml:"room_order"`
	}
	if _, err := toml.DecodeFile(filepath.Join(dir, "game.toml"), &g); err == nil {
		for _, rm := range g.RoomOrder {
			if !rooms[rm] {
				report("game.toml", "room_order's room", rm, rooms)
			}
		}
	}
	return problems
}
