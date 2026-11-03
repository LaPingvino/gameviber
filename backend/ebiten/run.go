// Package ebiten is GameViber's screen backend (docs/DESIGN.md §1): a window
// (or the browser canvas) showing a running game through Ebitengine, the
// keyboard feeding the runner. The drawing itself is render's (one picture
// for the screen and for headless checks).
package ebiten

import (
	"image"

	eb "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/LaPingvino/gameviber/import/gmfile"
	"github.com/LaPingvino/gameviber/load"
	"github.com/LaPingvino/gameviber/render"
	"github.com/LaPingvino/gameviber/runner"
)

// keys GM 5 games use, Windows key codes → Ebitengine keys.
var keys = map[runner.Key]eb.Key{
	37: eb.KeyArrowLeft, 38: eb.KeyArrowUp, 39: eb.KeyArrowRight, 40: eb.KeyArrowDown,
	32: eb.KeySpace, 13: eb.KeyEnter, 27: eb.KeyEscape, 33: eb.KeyPageUp, 34: eb.KeyPageDown,
}

type input struct {
	held, prev map[runner.Key]bool
	mx, my     float64 // the cursor in the room
}

func (in *input) Held(k runner.Key) bool     { return in.held[k] }
func (in *input) Pressed(k runner.Key) bool  { return in.held[k] && !in.prev[k] }
func (in *input) Released(k runner.Key) bool { return !in.held[k] && in.prev[k] }

// The mouse: 1 left, 2 right, 3 middle (GM's numbering).
var buttons = map[int]eb.MouseButton{1: eb.MouseButtonLeft, 2: eb.MouseButtonRight, 3: eb.MouseButtonMiddle}

func (in *input) Mouse() (float64, float64) { return in.mx, in.my }
func (in *input) MouseHeld(b int) bool      { return eb.IsMouseButtonPressed(buttons[b]) }
func (in *input) MousePressed(b int) bool   { return inpututil.IsMouseButtonJustPressed(buttons[b]) }
func (in *input) MouseReleased(b int) bool  { return inpututil.IsMouseButtonJustReleased(buttons[b]) }

// Game is a game in the window.
type Game struct {
	play    *load.Play
	assets  *render.Assets
	proj    *gmfile.Project
	in      input
	frame   *image.RGBA
	touches []eb.TouchID
	// MaxFrames ends the run after so many frames (0: run until closed) —
	// for checks under a virtual display.
	MaxFrames int
	frames    int
}

// New prepares a game for the window.
func New(p *gmfile.Project, pl *load.Play) *Game {
	return &Game{play: pl, assets: render.LoadAssets(p), proj: p, in: input{held: map[runner.Key]bool{}, prev: map[runner.Key]bool{}}}
}

// Update is one step of the game (Ebitengine calls it at the room's speed).
func (g *Game) Update() error {
	if g.MaxFrames > 0 && g.frames >= g.MaxFrames || g.play.Ended {
		return eb.Termination
	}
	g.in.prev, g.in.held = g.in.held, map[runner.Key]bool{}
	for k, e := range keys {
		if eb.IsKeyPressed(e) {
			g.in.held[k] = true
		}
	}
	// Touch (phones, tablets, touch screens): a finger held on the screen
	// holds the arrow towards it — from the screen's centre, along the
	// stronger direction — so arrow-key games play without a keyboard and
	// nothing covers the play area.
	if ids := eb.AppendTouchIDs(g.touches[:0]); len(ids) > 0 {
		g.touches = ids
		w, h := g.size()
		tx, ty := eb.TouchPosition(ids[0])
		dx, dy := float64(tx)-float64(w)/2, float64(ty)-float64(h)/2
		switch {
		case dx*dx+dy*dy < 24*24: // the very centre: no direction
		case dx*dx > dy*dy && dx < 0:
			g.in.held[37] = true
		case dx*dx > dy*dy:
			g.in.held[39] = true
		case dy < 0:
			g.in.held[38] = true
		default:
			g.in.held[40] = true
		}
	}
	cx, cy := eb.CursorPosition() // in the frame (Layout's size): + the view's position = the room
	g.in.mx, g.in.my = float64(cx), float64(cy)
	if r := g.play.Room; r.ViewsEnabled && len(r.Views) > 0 && r.Views[0].Visible {
		g.in.mx += float64(r.Views[0].X)
		g.in.my += float64(r.Views[0].Y)
	}
	if err := g.play.Step(&g.in); err != nil {
		return err
	}
	if s := g.play.Room.Speed; s > 0 && eb.TPS() != s {
		eb.SetTPS(s)
	}
	return nil
}

// Draw draws the room (through its view) with render.
func (g *Game) Draw(screen *eb.Image) {
	g.frames++
	r := g.play.Room
	w, h := g.size()
	if g.frame == nil || g.frame.Bounds().Dx() != w || g.frame.Bounds().Dy() != h {
		g.frame = image.NewRGBA(image.Rect(0, 0, w, h))
	}
	var colour int32
	drawColour := true
	for _, x := range g.proj.Rooms {
		if x != nil && x.Name == r.Name {
			colour, drawColour = x.BGColor, x.DrawBGColor
		}
	}
	render.Frame(g.frame, g.play.G.Game, r, g.assets, colour, drawColour)
	screen.WritePixels(g.frame.Pix)
}

func (g *Game) size() (int, int) {
	r := g.play.Room
	if r.ViewsEnabled && len(r.Views) > 0 && r.Views[0].Visible {
		return r.Views[0].W, r.Views[0].H
	}
	return r.W, r.H
}

// Layout is the room (or view) size; Ebitengine scales it to the window.
func (g *Game) Layout(_, _ int) (int, int) { return g.size() }

// Frames is how many frames were drawn.
func (g *Game) Frames() int { return g.frames }

// Run opens the window and runs until it is closed (or MaxFrames).
func (g *Game) Run(title string) error {
	w, h := g.size()
	eb.SetWindowSize(w, h)
	eb.SetWindowTitle(title)
	eb.SetWindowResizingMode(eb.WindowResizingModeEnabled)
	if s := g.play.Room.Speed; s > 0 {
		eb.SetTPS(s)
	}
	return eb.RunGame(g)
}
