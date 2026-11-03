package gml

import (
	"fmt"
	"strings"
)

// Error is a parse error: where, and what was expected (plain and
// actionable — docs/DESIGN.md §3).
type Error struct {
	Pos Pos
	Msg string
}

func (e *Error) Error() string { return e.Pos.String() + ": " + e.Msg }

// Parse reads a piece of GML (a script, or an event's code) into its
// statements. GM 5.3's language: statements may end with ';' or not, 'then'
// and 'do' are optional, begin/end stand for { }, ':=' assigns, and '='
// inside an expression compares.
func Parse(src string) ([]Stmt, error) {
	p := &parser{toks: Lex(src)}
	var list []Stmt
	for p.peek().Kind != EOF {
		if p.isOp(";") {
			p.i++
			continue
		}
		s, err := p.stmt()
		if err != nil {
			return list, err
		}
		list = append(list, s)
	}
	return list, nil
}

type parser struct {
	toks []Token
	i    int
}

func (p *parser) peek() Token { return p.toks[p.i] }
func (p *parser) next() Token { t := p.toks[p.i]; p.i++; return t }

func (p *parser) isOp(op string) bool { t := p.peek(); return t.Kind == Op && t.Text == op }
func (p *parser) isKw(kw string) bool { t := p.peek(); return t.Kind == Keyword && t.Text == kw }

func (p *parser) errf(t Token, format string, args ...any) error {
	got := t.Text
	if t.Kind == EOF {
		got = "the end"
	}
	return &Error{t.Pos, fmt.Sprintf(format, args...) + fmt.Sprintf(", found %q", got)}
}

func (p *parser) expectOp(op string) error {
	if !p.isOp(op) {
		return p.errf(p.peek(), "expected %q", op)
	}
	p.i++
	return nil
}

func (p *parser) optSemi() {
	if p.isOp(";") {
		p.i++
	}
}

func (p *parser) stmt() (Stmt, error) {
	t := p.peek()
	if t.Kind == Illegal {
		return nil, &Error{t.Pos, fmt.Sprintf("unexpected character %q", t.Text)}
	}
	if t.Kind == Op && t.Text == "{" || t.Kind == Keyword && t.Text == "begin" {
		return p.block()
	}
	if t.Kind == Keyword {
		switch t.Text {
		case "var", "globalvar":
			p.i++
			v := &Var{P: t.Pos, Global: t.Text == "globalvar"}
			for {
				n := p.next()
				if n.Kind != Ident {
					return nil, p.errf(n, "expected a variable name after %s", t.Text)
				}
				v.Names = append(v.Names, n.Text)
				if !p.isOp(",") {
					break
				}
				p.i++
			}
			p.optSemi()
			return v, nil
		case "if":
			p.i++
			c, err := p.expr()
			if err != nil {
				return nil, err
			}
			if p.isKw("then") {
				p.i++
			}
			th, err := p.stmt()
			if err != nil {
				return nil, err
			}
			s := &If{P: t.Pos, Cond: c, Then: th}
			if p.isOp(";") && p.toks[p.i+1].Kind == Keyword && p.toks[p.i+1].Text == "else" {
				p.i++
			}
			if p.isKw("else") {
				p.i++
				if s.Else, err = p.stmt(); err != nil {
					return nil, err
				}
			}
			return s, nil
		case "while", "repeat", "with":
			p.i++
			c, err := p.expr()
			if err != nil {
				return nil, err
			}
			if p.isKw("do") {
				p.i++
			}
			b, err := p.stmt()
			if err != nil {
				return nil, err
			}
			switch t.Text {
			case "while":
				return &While{t.Pos, c, b}, nil
			case "repeat":
				return &Repeat{t.Pos, c, b}, nil
			}
			return &With{t.Pos, c, b}, nil
		case "do":
			p.i++
			b, err := p.stmt()
			if err != nil {
				return nil, err
			}
			p.optSemi()
			if !p.isKw("until") {
				return nil, p.errf(p.peek(), "expected 'until' after the body of 'do'")
			}
			p.i++
			c, err := p.expr()
			if err != nil {
				return nil, err
			}
			p.optSemi()
			return &Do{t.Pos, b, c}, nil
		case "for":
			p.i++
			if err := p.expectOp("("); err != nil {
				return nil, err
			}
			f := &For{P: t.Pos}
			var err error
			if f.Init, err = p.simple(); err != nil {
				return nil, err
			}
			p.optSemi()
			if f.Cond, err = p.expr(); err != nil {
				return nil, err
			}
			p.optSemi()
			if f.Step, err = p.simple(); err != nil {
				return nil, err
			}
			p.optSemi()
			if err := p.expectOp(")"); err != nil {
				return nil, err
			}
			if f.Body, err = p.stmt(); err != nil {
				return nil, err
			}
			return f, nil
		case "switch":
			return p.switchStmt()
		case "exit":
			p.i++
			p.optSemi()
			return &Exit{t.Pos}, nil
		case "break":
			p.i++
			p.optSemi()
			return &Break{t.Pos}, nil
		case "continue":
			p.i++
			p.optSemi()
			return &Continue{t.Pos}, nil
		case "return":
			p.i++
			x, err := p.expr()
			if err != nil {
				return nil, err
			}
			p.optSemi()
			return &Return{t.Pos, x}, nil
		}
		if t.Text != "not" { // 'not' can start an expression; other keywords can't start a statement
			return nil, p.errf(t, "a statement can't start with %q", t.Text)
		}
	}
	s, err := p.simple()
	if err != nil {
		return nil, err
	}
	p.optSemi()
	return s, nil
}

// simple is an assignment or a call (also for's init and step).
func (p *parser) simple() (Stmt, error) {
	start := p.peek()
	x, err := p.postfix()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.Kind == Op {
		switch t.Text {
		case "=", ":=", "+=", "-=", "*=", "/=", "&=", "|=", "^=", "<<=", ">>=":
			if _, ok := x.(*Call); ok {
				return nil, p.errf(t, "can't assign to a function call")
			}
			p.i++
			v, err := p.expr()
			if err != nil {
				return nil, err
			}
			return &Assign{start.Pos, x, t.Text, v}, nil
		}
	}
	if _, ok := x.(*Call); ok {
		return &ExprStmt{start.Pos, x}, nil
	}
	return nil, p.errf(p.peek(), "expected an assignment ('=') or a function call after %q", start.Text)
}

func (p *parser) block() (Stmt, error) {
	t := p.next()
	closer := "}"
	if t.Text == "begin" {
		closer = "end"
	}
	b := &Block{P: t.Pos}
	for {
		n := p.peek()
		if n.Text == closer && (n.Kind == Op || n.Kind == Keyword) {
			p.i++
			return b, nil
		}
		if n.Kind == EOF {
			return nil, p.errf(n, "expected %q to close the block opened at %s", closer, t.Pos)
		}
		if p.isOp(";") {
			p.i++
			continue
		}
		s, err := p.stmt()
		if err != nil {
			return nil, err
		}
		b.List = append(b.List, s)
	}
}

func (p *parser) switchStmt() (Stmt, error) {
	t := p.next()
	x, err := p.expr()
	if err != nil {
		return nil, err
	}
	open := p.peek()
	if !p.isOp("{") && !p.isKw("begin") {
		return nil, p.errf(open, "expected '{' after switch's value")
	}
	p.i++
	closer := "}"
	if open.Text == "begin" {
		closer = "end"
	}
	s := &Switch{P: t.Pos, X: x}
	for {
		n := p.peek()
		switch {
		case n.Text == closer && (n.Kind == Op || n.Kind == Keyword):
			p.i++
			return s, nil
		case n.Kind == EOF:
			return nil, p.errf(n, "expected %q to close the switch at %s", closer, t.Pos)
		case n.Kind == Keyword && (n.Text == "case" || n.Text == "default"):
			p.i++
			c := Case{P: n.Pos}
			if n.Text == "case" {
				v, err := p.expr()
				if err != nil {
					return nil, err
				}
				c.Values = []Expr{v}
			}
			if err := p.expectOp(":"); err != nil {
				return nil, err
			}
			s.Cases = append(s.Cases, c)
		case p.isOp(";"):
			p.i++
		default:
			if len(s.Cases) == 0 {
				return nil, p.errf(n, "expected 'case' or 'default' in switch")
			}
			st, err := p.stmt()
			if err != nil {
				return nil, err
			}
			last := &s.Cases[len(s.Cases)-1]
			last.Body = append(last.Body, st)
		}
	}
}

// Binary operators by precedence, lowest first (GM 5: logic, then
// comparison, then bitwise, shifts, additive, multiplicative).
var levels = [][]string{
	{"&&", "||", "^^", "and", "or", "xor"},
	{"<", "<=", "==", "=", "!=", "<>", ">=", ">"},
	{"|", "&", "^"},
	{"<<", ">>"},
	{"+", "-"},
	{"*", "/", "div", "mod"},
}

func (p *parser) expr() (Expr, error) { return p.binary(0) }

func (p *parser) binary(level int) (Expr, error) {
	if level == len(levels) {
		return p.unary()
	}
	l, err := p.binary(level + 1)
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if (t.Kind != Op && t.Kind != Keyword) || !contains(levels[level], t.Text) {
			return l, nil
		}
		p.i++
		r, err := p.binary(level + 1)
		if err != nil {
			return nil, err
		}
		op := t.Text
		switch op { // one spelling per operator in the tree
		case "and":
			op = "&&"
		case "or":
			op = "||"
		case "xor":
			op = "^^"
		case "=":
			op = "=="
		case "<>":
			op = "!="
		}
		l = &Binary{t.Pos, op, l, r}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (p *parser) unary() (Expr, error) {
	t := p.peek()
	if t.Kind == Op && (t.Text == "!" || t.Text == "-" || t.Text == "+" || t.Text == "~") || t.Kind == Keyword && t.Text == "not" {
		p.i++
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		op := t.Text
		if op == "not" {
			op = "!"
		}
		return &Unary{t.Pos, op, x}, nil
	}
	return p.postfix()
}

func (p *parser) postfix() (Expr, error) {
	x, err := p.primary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		switch {
		case t.Kind == Op && t.Text == ".":
			p.i++
			n := p.next()
			if n.Kind != Ident {
				return nil, p.errf(n, "expected a variable name after '.'")
			}
			x = &Field{t.Pos, x, n.Text}
		case t.Kind == Op && t.Text == "[":
			p.i++
			ix := &Index{P: t.Pos, X: x}
			for {
				e, err := p.expr()
				if err != nil {
					return nil, err
				}
				ix.I = append(ix.I, e)
				if !p.isOp(",") {
					break
				}
				p.i++
			}
			if err := p.expectOp("]"); err != nil {
				return nil, err
			}
			x = ix
		default:
			return x, nil
		}
	}
}

func (p *parser) primary() (Expr, error) {
	t := p.next()
	switch t.Kind {
	case Number:
		return &Num{t.Pos, t.Text}, nil
	case String:
		return &Str{t.Pos, t.Text}, nil
	case Ident:
		if p.isOp("(") {
			p.i++
			c := &Call{P: t.Pos, Fn: t.Text}
			for !p.isOp(")") {
				a, err := p.expr()
				if err != nil {
					return nil, err
				}
				c.Args = append(c.Args, a)
				if p.isOp(",") {
					p.i++
				} else if !p.isOp(")") {
					return nil, p.errf(p.peek(), "expected ',' or ')' in the call to %s", t.Text)
				}
			}
			p.i++
			return c, nil
		}
		return &Name{t.Pos, t.Text}, nil
	case Op:
		if t.Text == "(" {
			x, err := p.expr()
			if err != nil {
				return nil, err
			}
			return x, p.expectOp(")")
		}
	}
	p.i--
	return nil, p.errf(t, "expected a value (a number, string, name, call or '(')")
}

// Format writes an expression back as GML (for tests and error messages).
func Format(e Expr) string {
	switch e := e.(type) {
	case *Num:
		return e.Text
	case *Str:
		return "'" + e.Value + "'"
	case *Name:
		return e.Name
	case *Field:
		return Format(e.X) + "." + e.Name
	case *Index:
		var s []string
		for _, i := range e.I {
			s = append(s, Format(i))
		}
		return Format(e.X) + "[" + strings.Join(s, ",") + "]"
	case *Call:
		var s []string
		for _, a := range e.Args {
			s = append(s, Format(a))
		}
		return e.Fn + "(" + strings.Join(s, ",") + ")"
	case *Unary:
		return "(" + e.Op + Format(e.X) + ")"
	case *Binary:
		return "(" + Format(e.L) + " " + e.Op + " " + Format(e.R) + ")"
	}
	return "?"
}
