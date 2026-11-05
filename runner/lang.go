package runner

import (
	"fmt"
	"path"
	"strings"

	"github.com/LaPingvino/gameviber/gml"
)

// A Lang compiles event code written in another language than GML. file
// names the code in errors; the handler gets other in collision events (nil
// elsewhere). A language package registers itself in Langs by extension
// (".js"), so a build without it leaves it out.
type Lang func(g *Game, file, src string) (CollisionHandler, error)

// Langs are the languages besides GML, by file extension.
var Langs = map[string]Lang{}

func (g *Game) lang(file string) (Lang, error) {
	ext := strings.ToLower(path.Ext(file))
	if ext == "" || ext == ".gml" {
		return nil, nil
	}
	l := Langs[ext]
	if l == nil {
		return nil, fmt.Errorf("%s: no language for %s files in this build", file, ext)
	}
	return l, nil
}

// CodeIn compiles an event's code in the language its file name says
// (.gml or none: GML).
func (g *Game) CodeIn(file, src string) (Handler, error) {
	l, err := g.lang(file)
	if err != nil || l == nil {
		if err != nil {
			return nil, err
		}
		return g.Code(src)
	}
	h, err := l(g, file, src)
	if err != nil {
		return nil, err
	}
	return func(r *Room, self *Instance) { h(r, self, nil) }, nil
}

// CollisionCodeIn is CodeIn for a collision event.
func (g *Game) CollisionCodeIn(file, src string) (CollisionHandler, error) {
	l, err := g.lang(file)
	if err != nil {
		return nil, err
	}
	if l == nil {
		return g.CollisionCode(src)
	}
	return l(g, file, src)
}

// Enter runs fn with the GML interpreter set up as event code in room r, as
// self (and other) — the context a language's handler calls GameViber's
// functions in — and puts the outer code's context back after. An error fn
// returns is the game's like a GML one.
func (g *Game) Enter(r *Room, self, other *Instance, fn func(in *gml.Interp) error) {
	var outerSelf, outerOther gml.Instance
	outerRoom := g.room
	if g.in != nil {
		outerSelf, outerOther = g.in.Self, g.in.Other
	}
	in := g.interp(r)
	defer func() { in.Self, in.Other, g.room = outerSelf, outerOther, outerRoom }()
	in.Self, in.Other = self, nil
	if other != nil {
		in.Other = other
	}
	if err := fn(in); err != nil && len(g.Errors) < 100 {
		g.Errors = append(g.Errors, err)
	}
}
