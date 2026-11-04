package ebiten

import (
	"image"
	"image/color"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/LaPingvino/gameviber/runner"
)

// Touch controls (the export options in LOOP.md, as Stelo's and Pacmanio's):
// once the screen has been touched, arrow pads and the sound / full-screen
// buttons — beside the game on a phone on its side (← → in a column on the
// left, ↑ ↓ on the right), under it upright — never over it; the frame is
// fitted into what is left. A finger on a pad holds that arrow key (the Hold
// mode; Toggle / Push wait for a game that needs them).

type pad struct {
	key runner.Key
	r   image.Rectangle // screen pixels
}

type touchLayout struct {
	area      image.Rectangle // the frame is fitted into this
	pads      []pad
	full, snd image.Rectangle
}

// layoutTouch lays the controls out on a sw×sh screen for a fw×fh frame.
func layoutTouch(fw, fh, sw, sh int) touchLayout {
	d := eb.Monitor().DeviceScaleFactor()
	if d <= 0 {
		d = 1
	}
	m := int(10 * d)
	if sw > sh { // on its side: columns beside the frame
		p := min(int(64*d), (sh-3*m)/3)
		left, right := 2*p+3*m, p+2*m
		y := sh - m - p
		return touchLayout{
			area: image.Rect(left, 0, sw-right, sh),
			pads: []pad{
				{37, image.Rect(m, y, m+p, y+p)},
				{39, image.Rect(2*m+p, y, 2*m+2*p, y+p)},
				{38, image.Rect(sw-m-p, y-m-p, sw-m, y-m)},
				{40, image.Rect(sw-m-p, y, sw-m, y+p)},
			},
			full: image.Rect(m, m, m+p, m+p*2/3),
			snd:  image.Rect(m, 2*m+p*2/3, m+p, 2*m+p*4/3),
		}
	}
	p := min(int(64*d), (sw-5*m)/4) // upright: a row under the frame
	frameH := sw * fh / max(fw, 1)  // the frame as wide as the screen
	bh := p * 2 / 3
	y := min(sh-3*m-p-bh, frameH+m)
	by := y + p + m
	return touchLayout{
		area: image.Rect(0, 0, sw, y-m),
		pads: []pad{
			{37, image.Rect(m, y, m+p, y+p)},
			{39, image.Rect(2*m+p, y, 2*m+2*p, y+p)},
			{38, image.Rect(sw-2*m-2*p, y, sw-2*m-p, y+p)},
			{40, image.Rect(sw-m-p, y, sw-m, y+p)},
		},
		full: image.Rect(sw/2-p-m/2, by, sw/2-m/2, by+bh),
		snd:  image.Rect(sw/2+m/2, by, sw/2+p+m/2, by+bh),
	}
}

// padKeys holds the arrows under the fingers.
func (g *Game) padKeys() {
	for _, id := range eb.AppendTouchIDs(nil) {
		pt := image.Pt(eb.TouchPosition(id))
		for _, p := range g.lay.pads {
			if pt.In(p.r) {
				g.in.held[p.key] = true
			}
		}
	}
}

// drawPads draws the arrow pads (brighter while held).
func (g *Game) drawPads(screen *eb.Image) {
	for _, p := range g.lay.pads {
		a := uint8(70)
		if g.in.held[p.key] {
			a = 150
		}
		vector.FillRect(screen, float32(p.r.Min.X), float32(p.r.Min.Y), float32(p.r.Dx()), float32(p.r.Dy()), color.NRGBA{255, 255, 255, a}, false)
		cx, cy := float32(p.r.Min.X+p.r.Dx()/2), float32(p.r.Min.Y+p.r.Dy()/2)
		s := float32(p.r.Dx()) * 0.28
		var path vector.Path
		switch p.key {
		case 37:
			path.MoveTo(cx-s, cy)
			path.LineTo(cx+s/2, cy-s)
			path.LineTo(cx+s/2, cy+s)
		case 39:
			path.MoveTo(cx+s, cy)
			path.LineTo(cx-s/2, cy-s)
			path.LineTo(cx-s/2, cy+s)
		case 38:
			path.MoveTo(cx, cy-s)
			path.LineTo(cx-s, cy+s/2)
			path.LineTo(cx+s, cy+s/2)
		case 40:
			path.MoveTo(cx, cy+s)
			path.LineTo(cx-s, cy-s/2)
			path.LineTo(cx+s, cy-s/2)
		}
		path.Close()
		var cs eb.ColorScale
		cs.ScaleWithColor(color.NRGBA{0, 0, 0, 160})
		vector.FillPath(screen, &path, &vector.FillOptions{}, &vector.DrawPathOptions{ColorScale: cs})
	}
}
