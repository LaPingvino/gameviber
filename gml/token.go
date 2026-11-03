// Package gml reads Game Maker Language: a lexer, a parser to one AST, and an
// interpreter (docs/DESIGN.md §1, §4). GM 5.3's language first; later
// versions as flags on the same front end.
package gml

import "fmt"

// Kind is a token's kind.
type Kind int

const (
	EOF       Kind = iota
	Ident          // names: variables, functions, objects, resources
	Keyword        // var, if, with, … (Keywords); also the word operators and, or, not, xor, div, mod
	Number         // 12, 1.5, $FF
	String         // "…" or '…' (no escapes in GM 5; may span lines)
	Op             // operators and punctuation: + += == ( { ; …
	Directive      // a line starting with '#': the recovery format's "#event Create" headers
	Illegal        // a character that starts no token (the parser reports it)
)

func (k Kind) String() string {
	return [...]string{"EOF", "Ident", "Keyword", "Number", "String", "Op", "Directive", "Illegal"}[k]
}

// Pos is a place in the source, 1-based.
type Pos struct{ Line, Col int }

func (p Pos) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

// Token is one token: its kind, its text as written (a string's without its
// quotes; a directive's without the '#'), and where it starts.
type Token struct {
	Kind Kind
	Text string
	Pos  Pos
}

func (t Token) String() string { return fmt.Sprintf("%s %s %q", t.Pos, t.Kind, t.Text) }

// Keywords are GM 5.3's reserved words (the Game Maker 5 manual, "GML: the
// language"), plus the word forms of operators. Built-in constants (true,
// self, noone, …) are identifiers: the interpreter knows them.
var Keywords = map[string]bool{
	"var": true, "globalvar": true, "if": true, "then": true, "else": true, "while": true, "do": true,
	"until": true, "repeat": true, "for": true, "with": true, "exit": true, "break": true, "continue": true,
	"return": true, "switch": true, "case": true, "default": true, "begin": true, "end": true,
	"and": true, "or": true, "not": true, "xor": true, "div": true, "mod": true,
}

// ops are the operators, longest first so the lexer takes the longest match.
var ops = []string{
	"<<=", ">>=",
	"&&", "||", "^^", "==", "!=", "<>", "<=", ">=", "<<", ">>", ":=", "+=", "-=", "*=", "/=", "&=", "|=", "^=",
	"+", "-", "*", "/", "&", "|", "^", "~", "!", "<", ">", "=", "(", ")", "[", "]", "{", "}", ",", ";", ".", ":", "?",
}
