package gml

import (
	"strings"
	"unicode/utf8"
)

// Lex splits src into tokens, ending with EOF. Comments (// to the end of the
// line, /* … */) and white space are dropped. It never fails: what starts no
// token comes back as Illegal, an unterminated string or comment runs to the
// end of the source (as Game Maker 5 read them), and the parser reports.
func Lex(src string) []Token {
	l := lexer{src: src, line: 1, col: 1}
	var toks []Token
	for {
		t := l.next()
		toks = append(toks, t)
		if t.Kind == EOF {
			return toks
		}
	}
}

type lexer struct {
	src       string
	i         int
	line, col int
	bol       bool // only white space since the line began (a '#' there is a directive)
}

func (l *lexer) pos() Pos { return Pos{l.line, l.col} }

func (l *lexer) advance(n int) {
	for _, r := range l.src[l.i : l.i+n] {
		if r == '\n' {
			l.line, l.col, l.bol = l.line+1, 1, true
		} else {
			l.col++
		}
	}
	l.i += n
}

func (l *lexer) next() Token {
	if l.i == 0 {
		l.bol = true
	}
	for l.i < len(l.src) { // white space and comments
		c := l.src[l.i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			l.advance(1)
		case strings.HasPrefix(l.src[l.i:], "//"):
			n := strings.IndexByte(l.src[l.i:], '\n')
			if n < 0 {
				n = len(l.src) - l.i
			}
			l.advance(n)
		case strings.HasPrefix(l.src[l.i:], "/*"):
			n := strings.Index(l.src[l.i+2:], "*/")
			if n < 0 {
				l.advance(len(l.src) - l.i)
			} else {
				l.advance(n + 4)
			}
		default:
			goto token
		}
	}
	return Token{Kind: EOF, Pos: l.pos()}
token:
	p, rest := l.pos(), l.src[l.i:]
	c := rest[0]
	bol := l.bol
	l.bol = false
	switch {
	case c == '#' && bol:
		n := strings.IndexByte(rest, '\n')
		if n < 0 {
			n = len(rest)
		}
		l.advance(n)
		return Token{Directive, strings.TrimSpace(strings.TrimSuffix(rest[1:n], "\r")), p}
	case isLetter(c):
		n := 1
		for n < len(rest) && (isLetter(rest[n]) || isDigit(rest[n])) {
			n++
		}
		l.advance(n)
		word := rest[:n]
		if Keywords[word] {
			return Token{Keyword, word, p}
		}
		return Token{Ident, word, p}
	case isDigit(c) || (c == '.' && len(rest) > 1 && isDigit(rest[1])):
		n := 0
		for n < len(rest) && (isDigit(rest[n]) || rest[n] == '.') {
			n++
		}
		l.advance(n)
		return Token{Number, rest[:n], p}
	case c == '$':
		n := 1
		for n < len(rest) && isHex(rest[n]) {
			n++
		}
		if n == 1 {
			l.advance(1)
			return Token{Illegal, "$", p}
		}
		l.advance(n)
		return Token{Number, rest[:n], p}
	case c == '"' || c == '\'':
		n := strings.IndexByte(rest[1:], c)
		if n < 0 { // unterminated: to the end
			l.advance(len(rest))
			return Token{String, rest[1:], p}
		}
		l.advance(n + 2)
		return Token{String, rest[1 : n+1], p}
	}
	for _, op := range ops {
		if strings.HasPrefix(rest, op) {
			l.advance(len(op))
			return Token{Op, op, p}
		}
	}
	_, size := utf8.DecodeRuneInString(rest)
	l.advance(size)
	return Token{Illegal, rest[:size], p}
}

func isLetter(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isHex(c byte) bool    { return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }
