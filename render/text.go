package render

import (
	"embed"
	"fmt"
	"image"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/LaPingvino/gameviber/runner"
)

// GM 5's text, drawn: the Windows fonts its games name stand in as Liberation
// (SIL OFL, fonts/LICENSE-Liberation) — Liberation Sans for Arial, MS Sans
// Serif and the rest (metric-compatible with Arial), Liberation Mono for
// Courier. Sizes are points at 96 DPI (Windows'), the colour GM's (BGR), the
// text's top-left at (x, y) (GM 5's alignment). PARTIAL: italic, and a
// regular monospace (only Mono Bold is here).

//go:embed fonts/*.ttf
var fontFiles embed.FS

var (
	faceMu sync.Mutex
	faces  = map[string]font.Face{}
	parsed = map[string]*opentype.Font{}
)

func fontFile(f runner.Font) string {
	name := strings.ToLower(f.Name)
	if strings.Contains(name, "courier") || strings.Contains(name, "mono") || strings.Contains(name, "console") {
		return "fonts/LiberationMono-Bold.ttf"
	}
	if f.Bold {
		return "fonts/LiberationSans-Bold.ttf"
	}
	return "fonts/LiberationSans-Regular.ttf"
}

// face is the font face for a GM font (cached by file and size).
func face(f runner.Font) font.Face { return faceScaled(f, 1) }

// faceScaled is face at scale times its size (the text drawn at a screen's
// resolution).
func faceScaled(f runner.Font, scale float64) font.Face {
	size := f.Size
	if size <= 0 {
		size = 12 // GM 5's default font: Arial 12
	}
	size *= scale
	file := fontFile(f)
	key := fmt.Sprintf("%s@%g", file, size)
	faceMu.Lock()
	defer faceMu.Unlock()
	if fc := faces[key]; fc != nil {
		return fc
	}
	ot := parsed[file]
	if ot == nil {
		data, err := fontFiles.ReadFile(file)
		if err != nil {
			return nil
		}
		if ot, err = opentype.Parse(data); err != nil {
			return nil
		}
		parsed[file] = ot
	}
	fc, err := opentype.NewFace(ot, &opentype.FaceOptions{Size: size, DPI: 96, Hinting: font.HintingFull})
	if err != nil {
		return nil
	}
	faces[key] = fc
	return fc
}

// drawText draws one text call into the frame: its top-left at (x, y), lines
// split at '#' (GM 5's newline) and at real newlines.
func drawText(dst *image.RGBA, t TextCall) {
	fc := face(t.Font)
	if fc == nil {
		return
	}
	m := fc.Metrics()
	lineH := m.Height.Ceil()
	d := font.Drawer{Dst: dst, Src: image.NewUniform(bgr(t.Font.Colour)), Face: fc}
	s := strings.ReplaceAll(strings.ReplaceAll(t.S, "\\#", "\x00"), "#", "\n")
	for i, line := range strings.Split(s, "\n") {
		d.Dot = fixed.P(int(t.X), int(t.Y)+m.Ascent.Ceil()+i*lineH)
		d.DrawString(strings.ReplaceAll(line, "\x00", "#"))
	}
}

// TextImage draws a text call at scale times its size into an image of its
// own (transparent around the letters) and says where its top-left goes, in
// frame pixels × scale — for a screen backend drawing the texts over the
// frame at the screen's resolution (Assets.TextOver).
func TextImage(t TextCall, scale float64) (*image.RGBA, image.Point) {
	fc := faceScaled(t.Font, scale)
	if fc == nil || scale <= 0 {
		return nil, image.Point{}
	}
	m := fc.Metrics()
	lineH := m.Height.Ceil()
	s := strings.ReplaceAll(strings.ReplaceAll(t.S, "\\#", "\x00"), "#", "\n")
	lines := strings.Split(s, "\n")
	w := 0
	for i, line := range lines {
		lines[i] = strings.ReplaceAll(line, "\x00", "#")
		w = max(w, font.MeasureString(fc, lines[i]).Ceil())
	}
	h := len(lines)*lineH + m.Descent.Ceil()
	if w == 0 || h == 0 {
		return nil, image.Point{}
	}
	img := image.NewRGBA(image.Rect(0, 0, w+2, h+2))
	d := font.Drawer{Dst: img, Src: image.NewUniform(bgr(t.Font.Colour)), Face: fc}
	for i, line := range lines {
		d.Dot = fixed.P(0, m.Ascent.Ceil()+i*lineH)
		d.DrawString(line)
	}
	return img, image.Pt(int(t.X*scale), int(t.Y*scale))
}
