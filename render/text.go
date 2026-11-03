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
func face(f runner.Font) font.Face {
	size := f.Size
	if size <= 0 {
		size = 12 // GM 5's default font: Arial 12
	}
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
