// Package ebiten is GameViber's screen backend (docs/DESIGN.md §1): a window
// (or the browser canvas) showing a running game through Ebitengine, the
// keyboard feeding the runner. The drawing itself is render's (one picture
// for the screen and for headless checks).
package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"runtime"
	"strings"
	"sync/atomic"

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

// Pause / StepOnce are the editor's Play controls: paused, the game does not
// step (the window keeps drawing); StepOnce runs one step while paused.
var (
	paused  atomic.Bool
	stepReq atomic.Int32
)

func Pause(on bool) { paused.Store(on) }
func StepOnce()     { stepReq.Add(1) }
func Paused() bool  { return paused.Load() }

// Mute turns the sound off (gameviber-run -mute).
var Mute bool

// audioAvailable: Ebitengine's audio stops the game when there is no sound
// device (a server, CI): on Linux, only with a playback device in /dev/snd
// (pcmC*p); elsewhere assumed.
func audioAvailable() bool {
	if runtime.GOOS != "linux" {
		return true
	}
	ents, _ := os.ReadDir("/dev/snd")
	for _, e := range ents {
		if n := e.Name(); strings.HasPrefix(n, "pcmC") && strings.HasSuffix(n, "p") {
			return true
		}
	}
	return false
}

// shortcuts are the player's own keys, not the game's (the export options in
// LOOP.md): M sound on / off, F11 or Alt+Enter full screen. The game never
// sees them (M is not among the keys handed to it).
func (g *Game) shortcuts() {
	if inpututil.IsKeyJustPressed(eb.KeyM) {
		if s, ok := g.play.G.Sound.(*sounds); ok {
			s.SetMuted(!s.muted)
		}
	}
	alt := eb.IsKeyPressed(eb.KeyAlt)
	if inpututil.IsKeyJustPressed(eb.KeyF11) || alt && inpututil.IsKeyJustPressed(eb.KeyEnter) {
		eb.SetFullscreen(!eb.IsFullscreen())
	}
}

// Game is a game in the window.
type Game struct {
	play      *load.Play
	assets    *render.Assets
	proj      *gmfile.Project
	in        input
	frame     *image.RGBA
	img       *eb.Image
	caption   string
	view      frameView
	sw, sh    int // the screen's size (the buttons' place)
	touches   []eb.TouchID
	textCache map[string]*eb.Image // texts drawn at the screen's resolution, by text / font / scale
	touched   bool                 // the screen has been touched: the pads show (touch.go)
	lay       touchLayout          // their last layout
	// MaxFrames ends the run after so many frames (0: run until closed) —
	// for checks under a virtual display.
	MaxFrames int
	frames    int
}

// Attach gives the game the screen backend's sound, messages and questions —
// before load's Start, which already runs the first room's create events.
func Attach(p *gmfile.Project, g *runner.Game) {
	g.Message = showMessage
	if hasDialogs {
		g.AskString = askString
	}
	if !Mute && audioAvailable() { // without a sound device the game plays silently
		g.Sound = newSounds(p)
	}
}

// New prepares a game for the window (after Attach and Start).
//
// Attach must come first: the first room's create events run as the game
// starts, and their sounds and questions need the backend already.
func New(p *gmfile.Project, pl *load.Play) *Game {
	a := render.LoadAssets(p)
	a.TextOver = true // texts drawn over the frame at the screen's resolution (Draw)
	return &Game{play: pl, assets: a, proj: p, in: input{held: map[runner.Key]bool{}, prev: map[runner.Key]bool{}}, textCache: map[string]*eb.Image{}}
}

// Update is one step of the game (Ebitengine calls it at the room's speed).
func (g *Game) Update() error {
	if g.MaxFrames > 0 && g.frames >= g.MaxFrames || g.play.Ended {
		return eb.Termination
	}
	g.shortcuts()
	g.buttonPress()
	if c := g.play.G.Caption(g.play.Room); c != g.caption && c != "" { // the window's caption (GM's)
		g.caption = c
		setTitle(c)
	}
	g.in.prev, g.in.held = g.in.held, map[runner.Key]bool{}
	alt := eb.IsKeyPressed(eb.KeyAlt)
	for k, e := range keys {
		if eb.IsKeyPressed(e) && !(alt && k == 13) { // Alt+Enter is full screen, not the game's Enter
			g.in.held[k] = true
		}
	}
	// Touch (phones, tablets, touch screens): a finger held on the screen
	// holds the arrow towards it — from the screen's centre, along the
	// stronger direction — so arrow-key games play without a keyboard and
	// nothing covers the play area.
	if len(eb.AppendTouchIDs(nil)) > 0 {
		g.touched = true
	}
	if g.touched {
		g.padKeys()
	} else if ids := eb.AppendTouchIDs(g.touches[:0]); len(ids) > 0 && !g.onButton(eb.TouchPosition(ids[0])) {
		g.touches = ids
		w, h := g.size() // the frame's centre on the screen
		tx, ty := eb.TouchPosition(ids[0])
		fx, fy := g.view.toFrame(tx, ty)
		dx, dy := float64(fx)-float64(w)/2, float64(fy)-float64(h)/2
		dx, dy = dx*g.view.scale, dy*g.view.scale // the dead centre in screen pixels
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
	cx, cy := g.view.toFrame(eb.CursorPosition()) // screen → frame; + the view's position = the room
	g.in.mx, g.in.my = float64(cx), float64(cy)
	if r := g.play.Room; r.ViewsEnabled && len(r.Views) > 0 && r.Views[0].Visible {
		g.in.mx += float64(r.Views[0].X)
		g.in.my += float64(r.Views[0].Y)
	}
	if paused.Load() { // the editor's pause: the picture stays, the game waits (a step request runs one step)
		if stepReq.Load() == 0 {
			return nil
		}
		stepReq.Add(-1)
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
	texts := render.Frame(g.frame, g.play.G.Game, r, g.assets, colour, drawColour)
	if g.img == nil || g.img.Bounds().Dx() != w || g.img.Bounds().Dy() != h {
		g.img = eb.NewImage(w, h)
	}
	g.img.WritePixels(g.frame.Pix)
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	if g.touched { // the frame in what the controls leave
		g.lay = layoutTouch(w, h, sw, sh)
		a := g.lay.area
		g.view = fit(w, h, a.Dx(), a.Dy())
		g.view.ox += float64(a.Min.X)
		g.view.oy += float64(a.Min.Y)
	} else {
		g.view = fit(w, h, sw, sh)
	}
	screen.Fill(color.Black)
	op := &eb.DrawImageOptions{Filter: eb.FilterNearest}
	op.GeoM.Scale(g.view.scale, g.view.scale)
	op.GeoM.Translate(g.view.ox, g.view.oy)
	screen.DrawImage(g.img, op)
	g.drawTexts(screen, texts)
	g.sw, g.sh = sw, sh
	g.drawButtons(screen)
	if g.touched {
		g.drawPads(screen)
	}
}

// Sharp scaling (the export options; as Stelo and Pacmanio): the frame on the
// real screen by whole steps, never smoothed — a black border where the
// screen doesn't divide evenly — as long as whole steps fill at least 90%;
// otherwise by the largest fraction that fits, still unsmoothed.
type frameView struct{ scale, ox, oy float64 }

// fit places a fw×fh frame on an sw×sh screen.
func fit(fw, fh, sw, sh int) frameView {
	k := min(float64(sw)/float64(fw), float64(sh)/float64(fh))
	if w := float64(int(k)); w >= 1 && w/k >= 0.9 {
		k = w
	}
	return frameView{scale: k, ox: (float64(sw) - float64(fw)*k) / 2, oy: (float64(sh) - float64(fh)*k) / 2}
}

// toFrame turns a screen position (mouse, touch) into frame coordinates.
func (v frameView) toFrame(x, y int) (int, int) {
	if v.scale == 0 {
		return x, y
	}
	return int((float64(x) - v.ox) / v.scale), int((float64(y) - v.oy) / v.scale)
}

func (g *Game) size() (int, int) {
	r := g.play.Room
	if r.ViewsEnabled && len(r.Views) > 0 && r.Views[0].Visible {
		return r.Views[0].W, r.Views[0].H
	}
	return r.W, r.H
}

// Layout is the screen's real resolution; Draw scales the frame onto it.
func (g *Game) Layout(outW, outH int) (int, int) {
	s := eb.Monitor().DeviceScaleFactor() // the screen's real resolution
	return int(float64(outW) * s), int(float64(outH) * s)
}

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

// drawTexts draws the frame's texts over it at the screen's resolution (the
// export options: text sharp, not scaled up), each drawn once per text, font
// and scale and kept. ASSUMPTION [engine]: over everything the frame shows —
// GM 5 games draw their texts (score, messages) last.
func (g *Game) drawTexts(screen *eb.Image, texts []render.TextCall) {
	if len(g.textCache) > 256 {
		clear(g.textCache)
	}
	for _, t := range texts {
		key := fmt.Sprintf("%q %v %g %g %g", t.S, t.Font, t.X, t.Y, g.view.scale)
		img, ok := g.textCache[key]
		if !ok {
			rgba, _ := render.TextImage(t, g.view.scale)
			if rgba != nil {
				img = eb.NewImageFromImage(rgba)
			}
			g.textCache[key] = img
		}
		if img == nil {
			continue
		}
		op := &eb.DrawImageOptions{}
		op.GeoM.Translate(g.view.ox+float64(int(t.X*g.view.scale)), g.view.oy+float64(int(t.Y*g.view.scale)))
		screen.DrawImage(img, op)
	}
}
