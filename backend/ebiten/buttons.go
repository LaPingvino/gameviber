package ebiten

import (
	"image"
	"image/color"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The player's buttons (the export options in LOOP.md; drawn as Stelo's and
// Pacmanio's): sound on / off and full screen, in the screen's margin beside
// the frame (a column on the right) or under it (a row at the bottom, a phone
// upright) — never over the game. No margin wide enough: no buttons (the keys
// M and F11 still work).

// buttonRects places the two buttons (sound, full screen) for a screen sw×sh
// with the frame at v; empty rectangles when there is no room.
func buttonRects(v frameView, fw, fh, sw, sh int) (sound, full image.Rectangle) {
	b := min(max(min(sw, sh)/12, 32), 64)
	gap := b / 4
	right := sw - int(v.ox+float64(fw)*v.scale) // the margin right of the frame
	bottom := sh - int(v.oy+float64(fh)*v.scale)
	switch {
	case right >= b+2*gap:
		x := sw - gap - b
		return image.Rect(x, gap, x+b, gap+b), image.Rect(x, 2*gap+b, x+b, 2*gap+2*b)
	case bottom >= b+2*gap:
		y := sh - gap - b
		return image.Rect(sw-2*gap-2*b, y, sw-2*gap-b, y+b), image.Rect(sw-gap-b, y, sw-gap, y+b)
	}
	return image.Rectangle{}, image.Rectangle{}
}

// buttonPress handles a click or a new touch on a button; it reports the
// points on buttons (a finger there is not an arrow).
func (g *Game) buttonPress() {
	sound, full := g.buttons()
	press := func(x, y int) {
		p := image.Pt(x, y)
		switch {
		case p.In(sound):
			if s, ok := g.play.G.Sound.(*sounds); ok {
				s.SetMuted(!s.muted)
			}
		case p.In(full):
			eb.SetFullscreen(!eb.IsFullscreen())
		}
	}
	if inpututil.IsMouseButtonJustPressed(eb.MouseButtonLeft) {
		press(eb.CursorPosition())
	}
	for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
		press(eb.TouchPosition(id))
	}
}

// onButton reports whether a screen point is on one of the buttons.
func (g *Game) onButton(x, y int) bool {
	sound, full := g.buttons()
	p := image.Pt(x, y)
	return p.In(sound) || p.In(full)
}

func (g *Game) buttons() (image.Rectangle, image.Rectangle) {
	w, h := g.size()
	return buttonRects(g.view, w, h, g.sw, g.sh)
}

func (g *Game) drawButtons(screen *eb.Image) {
	sound, full := g.buttons()
	muted := true // no sound device: shown as off
	if s, ok := g.play.G.Sound.(*sounds); ok {
		muted = s.muted
	}
	drawSoundButton(screen, sound, muted)
	drawFullButton(screen, full)
}

var (
	buttonBack = color.NRGBA{255, 255, 255, 50}
	buttonInk  = color.NRGBA{255, 255, 255, 210}
)

// drawFullButton: four corner marks.
func drawFullButton(screen *eb.Image, f image.Rectangle) {
	if f.Empty() {
		return
	}
	vector.FillRect(screen, float32(f.Min.X), float32(f.Min.Y), float32(f.Dx()), float32(f.Dy()), buttonBack, false)
	c, w := float32(f.Dy())/4, float32(max(2, f.Dy()/16))
	x0, y0, x1, y1 := float32(f.Min.X)+c, float32(f.Min.Y)+c/1.2, float32(f.Max.X)-c, float32(f.Max.Y)-c/1.2
	for _, l := range [][4]float32{{x0, y0, x0 + c, y0}, {x0, y0, x0, y0 + c}, {x1, y0, x1 - c, y0}, {x1, y0, x1, y0 + c},
		{x0, y1, x0 + c, y1}, {x0, y1, x0, y1 - c}, {x1, y1, x1 - c, y1}, {x1, y1, x1, y1 - c}} {
		vector.StrokeLine(screen, l[0], l[1], l[2], l[3], w, buttonInk, false)
	}
}

// drawSoundButton: a loudspeaker, with waves when on and a cross when off.
func drawSoundButton(screen *eb.Image, r image.Rectangle, muted bool) {
	if r.Empty() {
		return
	}
	vector.FillRect(screen, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), buttonBack, false)
	h := float32(r.Dy())
	cx, cy := float32(r.Min.X)+float32(r.Dx())/2, float32(r.Min.Y)+h/2
	u := h / 8
	var p vector.Path
	p.MoveTo(cx-3*u, cy-u)
	p.LineTo(cx-1.6*u, cy-u)
	p.LineTo(cx, cy-2.4*u)
	p.LineTo(cx, cy+2.4*u)
	p.LineTo(cx-1.6*u, cy+u)
	p.LineTo(cx-3*u, cy+u)
	p.Close()
	var cs eb.ColorScale
	cs.ScaleWithColor(buttonInk)
	vector.FillPath(screen, &p, &vector.FillOptions{}, &vector.DrawPathOptions{ColorScale: cs})
	w := max(1.5, u/2.5)
	if muted {
		vector.StrokeLine(screen, cx+u, cy-1.4*u, cx+3.4*u, cy+1.4*u, w, buttonInk, true)
		vector.StrokeLine(screen, cx+u, cy+1.4*u, cx+3.4*u, cy-1.4*u, w, buttonInk, true)
		return
	}
	vector.StrokeLine(screen, cx+1.2*u, cy-1.2*u, cx+1.2*u, cy+1.2*u, w, buttonInk, true)
	vector.StrokeLine(screen, cx+2.4*u, cy-2*u, cx+2.4*u, cy+2*u, w, buttonInk, true)
}
