package gml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseExpr(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"x = a + b * 2;", "(a + (b * 2))"},
		{"x = a = b and not c or d", "(((a == b) && (!c)) || d)"},
		{"x = -a.b[1, i+1] div 2 mod 3", "(((-a.b[1,(i + 1)]) div 2) mod 3)"},
		{"x = f(g(1), 'two') <> $FF", "(f(g(1),'two') != $FF)"},
		{"x = 1 | 2 & 3 << 1", "((1 | 2) & (3 << 1))"},
	} {
		list, err := Parse(c.src)
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		a, ok := list[0].(*Assign)
		if !ok {
			t.Errorf("%q: %T, want an assignment", c.src, list[0])
			continue
		}
		if got := Format(a.Value); got != c.want {
			t.Errorf("%q\n got %s\nwant %s", c.src, got, c.want)
		}
	}
}

func TestParseStatements(t *testing.T) {
	src := `
var i, n; globalvar g
if x > 0 then y := 1 else begin y = 2; exit end
while (i < 10) do i += 1
do { i -= 1 } until i <= 0
repeat 3 instance_create(x, y, obj)
for (i = 0; i < 4; i += 1) { a[i] = i; }
with (other) { speed = 0; }
switch (dir) { case 0: hspeed = 4; break; case 1: case 2: vspeed = 4; default: speed = 0 }
other.x = global.startx; return x
`
	list, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, s := range list {
		kinds = append(kinds, strings.TrimPrefix(strings.Split(strings.TrimPrefix(typeName(s), "*gml."), "{")[0], "*"))
	}
	want := "Var Var If While Do Repeat For With Switch Assign Return"
	if got := strings.Join(kinds, " "); got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
	sw := list[8].(*Switch)
	if len(sw.Cases) != 4 || sw.Cases[3].Values != nil || len(sw.Cases[1].Body) != 0 {
		t.Errorf("switch cases: %d, last default %v", len(sw.Cases), sw.Cases[3].Values == nil)
	}
}

func typeName(s Stmt) string {
	switch s.(type) {
	case *Var:
		return "Var"
	case *If:
		return "If"
	case *While:
		return "While"
	case *Do:
		return "Do"
	case *Repeat:
		return "Repeat"
	case *For:
		return "For"
	case *With:
		return "With"
	case *Switch:
		return "Switch"
	case *Assign:
		return "Assign"
	case *Return:
		return "Return"
	case *ExprStmt:
		return "ExprStmt"
	case *Block:
		return "Block"
	}
	return "?"
}

func TestParseErrors(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"x = ", "1:5: expected a value"},
		{"if x { y = 1", "expected \"}\" to close the block opened at 1:6"},
		{"x + 1", "expected an assignment ('=') or a function call"},
		{"f() = 2", "can't assign to a function call"},
	} {
		_, err := Parse(c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Parse(%q) = %v, want …%s…", c.src, err, c.want)
		}
	}
}

// TestParseCorpus: the recovered GML of Stelo and Pacmanio parses — the
// scripts whole, each code action of the objects and timelines on its own.
func TestParseCorpus(t *testing.T) {
	var files []string
	for _, dir := range []string{"../../stelo/original", "../../pacmanio-recovered/pacmanio"} {
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err == nil && strings.HasSuffix(p, ".gml") {
				files = append(files, p)
			}
			return nil
		})
	}
	if len(files) == 0 {
		t.Skip("the sample games' recovered GML is not checked out next to this repo")
	}
	pieces, stmts := 0, 0
	for _, f := range files {
		src, _ := os.ReadFile(f)
		var parts []string
		if strings.Contains(f, "/objects/") || strings.Contains(f, "/timelines/") {
			parts = codeActionList(string(src))
		} else {
			parts = []string{string(src)}
		}
		for _, part := range parts {
			list, err := Parse(part)
			if err != nil {
				t.Errorf("%s: %v", f, err)
				continue
			}
			pieces++
			stmts += len(list)
		}
	}
	t.Logf("%d files, %d pieces of code, %d top-level statements", len(files), pieces, stmts)
}

// codeActionList returns an object file's code actions, each on its own.
func codeActionList(src string) []string {
	var out []string
	var cur []string
	in := false
	for _, l := range strings.Split(src, "\n") {
		switch strings.TrimSpace(l) {
		case "// --- code action ---":
			in, cur = true, nil
			continue
		case "// --- end code ---":
			if in {
				out = append(out, strings.Join(cur, "\n"))
			}
			in = false
			continue
		}
		if in {
			cur = append(cur, l)
		}
	}
	return out
}
