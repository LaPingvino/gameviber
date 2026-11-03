package gmfile

import (
	"fmt"
	"strconv"
	"strings"
)

// Action kinds (GM 5/6): a normal action, a block's begin and end, else,
// exit, repeat, set a variable, code.
const (
	kindNormal   = 0
	kindBegin    = 1
	kindEnd      = 2
	kindElse     = 3
	kindExit     = 4
	kindRepeat   = 5
	kindVariable = 6
	kindCode     = 7
)

// Argument kinds that name a resource (the stored value is its id).
var resourceArg = map[int32]string{5: "sprite", 6: "sound", 7: "background", 8: "path", 9: "script",
	10: "object", 11: "room", 12: "font", 14: "timeline"}

// ActionsToGML turns an event's action list into GML, as GameMaker 5 runs
// it: a question guards the next action or block (and an else after it);
// begin / end make a block; repeat repeats the next; code actions are kept
// as written; a variable action assigns (+= when relative). Calls follow
// runner/actions.go: the relative box as one more argument when the action
// can be relative, "applies to" another object or other as with(…).
func (p *Project) ActionsToGML(acts []Action) (string, error) {
	c := &converter{p: p, acts: acts}
	var b strings.Builder
	for c.i < len(acts) {
		s, err := c.stmt()
		if err != nil {
			return b.String(), err
		}
		b.WriteString(s)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

type converter struct {
	p    *Project
	acts []Action
	i    int
}

func (c *converter) stmt() (string, error) {
	if c.i >= len(c.acts) {
		return "", fmt.Errorf("an action list ends where a statement was expected")
	}
	a := c.acts[c.i]
	c.i++
	switch a.Kind {
	case kindBegin:
		var parts []string
		for c.i < len(c.acts) && c.acts[c.i].Kind != kindEnd {
			s, err := c.stmt()
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		c.i++ // the end
		return "{\n" + indent(strings.Join(parts, "\n")) + "\n}", nil
	case kindEnd:
		return "", fmt.Errorf("action %d: end of a block without a begin", c.i-1)
	case kindElse:
		return "", fmt.Errorf("action %d: else without a question", c.i-1)
	case kindExit:
		return "exit", nil
	case kindRepeat:
		body, err := c.stmt()
		if err != nil {
			return "", err
		}
		return c.with(a, "repeat ("+c.arg(a, 0)+") "+body), nil
	case kindVariable:
		op := " = "
		if a.Relative {
			op = " += "
		}
		return c.with(a, arg(a, 0)+op+c.arg(a, 1)), nil
	case kindCode:
		return c.with(a, "{\n"+indent(strings.TrimRight(arg(a, 0), "\n"))+"\n}"), nil
	}
	if a.Lib == 1 && a.ID == 605 { // the comment action
		return "// " + strings.ReplaceAll(arg(a, 0), "\n", " "), nil
	}
	if a.ExecType == 0 && a.Function == "" {
		return fmt.Sprintf("// (action %d/%d does nothing when run)", a.Lib, a.ID), nil
	}
	call := c.call(a)
	if !a.IsQuestion {
		return c.with(a, call), nil
	}
	cond := call
	if a.Not {
		cond = "!" + call
	}
	then, err := c.stmt()
	if err != nil {
		return "", err
	}
	s := "if " + cond + " " + then
	if c.i < len(c.acts) && c.acts[c.i].Kind == kindElse {
		c.i++
		els, err := c.stmt()
		if err != nil {
			return "", err
		}
		s += " else " + els
	}
	if a.AppliesTo != -1 {
		// GM 5: a question applied to others is true when it is for all of
		// them. PARTIAL: asked of the first here.
		return c.with(a, s), nil
	}
	return s, nil
}

func (c *converter) call(a Action) string {
	fn := a.Function
	if fn == "" {
		fn = fmt.Sprintf("action_%d_%d", a.Lib, a.ID)
	}
	var args []string
	for i := 0; i < int(a.ArgC); i++ {
		args = append(args, c.arg(a, i))
	}
	if a.MayBeRelative {
		args = append(args, strconv.Itoa(b2i(a.Relative)))
	}
	return fn + "(" + strings.Join(args, ", ") + ")"
}

// with puts code under "applies to": -1 self, -2 other, else an object.
func (c *converter) with(a Action, code string) string {
	switch {
	case a.AppliesTo == -1:
		return code
	case a.AppliesTo == -2:
		return "with (other) " + code
	}
	return "with (" + c.name(10, a.AppliesTo) + ") " + code
}

func arg(a Action, i int) string {
	if i < len(a.Args) {
		return a.Args[i]
	}
	return ""
}

// arg formats argument i by its kind: expressions as written, strings
// quoted, resources by name.
func (c *converter) arg(a Action, i int) string {
	v := arg(a, i)
	var k int32
	if i < len(a.ArgKinds) {
		k = a.ArgKinds[i]
	}
	switch {
	case k == 0 || k == 3 || k == 4 || k == 13:
		if strings.TrimSpace(v) == "" {
			return "0"
		}
		return v
	case k == 1 || k == 15:
		return quote(v)
	case k == 2: // string or expression: GM takes a leading quote as an expression
		if t := strings.TrimSpace(v); strings.HasPrefix(t, "'") || strings.HasPrefix(t, "\"") {
			return v
		}
		return quote(v)
	}
	if _, ok := resourceArg[k]; ok {
		id, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return quote(v)
		}
		return c.name(k, int32(id))
	}
	return v
}

// name is a resource's name by kind and id ("-1" for none).
func (c *converter) name(kind int32, id int32) string {
	if id < 0 {
		return "-1"
	}
	at := func(n int, get func(int) string) string {
		if int(id) < n {
			if s := get(int(id)); s != "" {
				return s
			}
		}
		return strconv.Itoa(int(id))
	}
	p := c.p
	switch resourceArg[kind] {
	case "sprite":
		return at(len(p.Sprites), func(i int) string { return nameOr(p.Sprites[i] != nil, func() string { return p.Sprites[i].Name }) })
	case "sound":
		return at(len(p.Sounds), func(i int) string { return nameOr(p.Sounds[i] != nil, func() string { return p.Sounds[i].Name }) })
	case "background":
		return at(len(p.Backgrounds), func(i int) string {
			return nameOr(p.Backgrounds[i] != nil, func() string { return p.Backgrounds[i].Name })
		})
	case "path":
		return at(len(p.Paths), func(i int) string { return nameOr(p.Paths[i] != nil, func() string { return p.Paths[i].Name }) })
	case "script":
		return at(len(p.Scripts), func(i int) string { return nameOr(p.Scripts[i] != nil, func() string { return p.Scripts[i].Name }) })
	case "object":
		return at(len(p.Objects), func(i int) string { return nameOr(p.Objects[i] != nil, func() string { return p.Objects[i].Name }) })
	case "room":
		return at(len(p.Rooms), func(i int) string { return nameOr(p.Rooms[i] != nil, func() string { return p.Rooms[i].Name }) })
	case "timeline":
		return at(len(p.Timelines), func(i int) string { return nameOr(p.Timelines[i] != nil, func() string { return p.Timelines[i].Name }) })
	}
	return strconv.Itoa(int(id))
}

func nameOr(ok bool, f func() string) string {
	if ok {
		return f()
	}
	return ""
}

// quote writes a GM 5 string literal (no escapes: the other quote if needed;
// both kinds joined with + when the text has both).
func quote(s string) string {
	switch {
	case !strings.Contains(s, "'"):
		return "'" + s + "'"
	case !strings.Contains(s, "\""):
		return "\"" + s + "\""
	}
	parts := strings.Split(s, "'")
	for i, p := range parts {
		parts[i] = "'" + p + "'"
	}
	return strings.Join(parts, " + \"'\" + ")
}

func indent(s string) string {
	if s == "" {
		return s
	}
	return "\t" + strings.ReplaceAll(s, "\n", "\n\t")
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
