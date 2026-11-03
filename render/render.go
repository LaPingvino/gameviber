// Package render draws a running room into an image: headless (plain Go
// images), the reference the screen backend is checked against.
package render

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"

	"github.com/LaPingvino/gameviber/import/gmfile"
	"github.com/LaPingvino/gameviber/runner"
)

// Assets are the decoded images of a project: sprite frames and backgrounds
// by name, with the sprites' origins.
type Assets struct {
	Frames  map[string][]*image.NRGBA
	Origins map[string]image.Point
	BGs     map[string]*image.NRGBA
}

// LoadAssets decodes a project's sprites and backgrounds.
func LoadAssets(p *gmfile.Project) *Assets {
	a := &Assets{Frames: map[string][]*image.NRGBA{}, Origins: map[string]image.Point{}, BGs: map[string]*image.NRGBA{}}
	for _, s := range p.Sprites {
		if s == nil {
			continue
		}
		for _, f := range s.Frames {
			if img, err := gmfile.DecodeBMP(f, s.Transparent); err == nil {
				a.Frames[s.Name] = append(a.Frames[s.Name], img)
			}
		}
		a.Origins[s.Name] = image.Pt(int(s.OriginX), int(s.OriginY))
	}
	for _, b := range p.Backgrounds {
		if b != nil && b.Image != nil {
			if img, err := gmfile.DecodeBMP(b.Image, false); err == nil {
				a.BGs[b.Name] = img
			}
		}
	}
	return a
}

// Frame draws the room: the background colour (background_color when the
// game set it, else the room's), then the instances, deepest first, each its
// sprite's frame at floor(image_index).
// PARTIAL: room background layers, views and draw events come next.
func Frame(dst *image.RGBA, r *runner.Room, a *Assets, roomColour int32, drawColour bool) {
	if drawColour {
		c := roomColour
		if v, ok := r.G.Vars["background_color"]; ok {
			c = int32(v)
		}
		u := uint32(c)
		draw.Draw(dst, dst.Bounds(), image.NewUniform(color.RGBA{uint8(u), uint8(u >> 8), uint8(u >> 16), 255}), image.Point{}, draw.Src)
	}
	insts := make([]*runner.Instance, 0, len(r.Instances))
	for _, in := range r.Instances {
		if !in.Destroyed && in.Obj.Visible {
			insts = append(insts, in)
		}
	}
	sort.SliceStable(insts, func(i, j int) bool { return insts[i].Obj.Depth > insts[j].Obj.Depth })
	for _, in := range insts {
		frames := a.Frames[in.Sprite]
		if len(frames) == 0 {
			continue
		}
		k := int(math.Floor(in.ImageIndex)) % len(frames)
		if k < 0 {
			k += len(frames)
		}
		f := frames[k]
		o := a.Origins[in.Sprite]
		at := image.Pt(int(math.Round(in.X))-o.X, int(math.Round(in.Y))-o.Y)
		draw.Draw(dst, f.Bounds().Add(at), f, image.Point{}, draw.Over)
	}
}
