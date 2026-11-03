package gml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func kinds(toks []Token) string {
	var b strings.Builder
	for _, t := range toks {
		if t.Kind == EOF {
			break
		}
		b.WriteString(t.Kind.String()[:1] + ":" + t.Text + " ")
	}
	return strings.TrimSpace(b.String())
}

func TestLex(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"x += 4;", "I:x O:+= N:4 O:;"},
		{"if (a<>b) and not c then exit", "K:if O:( I:a O:<> I:b O:) K:and K:not I:c K:then K:exit"},
		{"a := $FF div 2 mod .5", "I:a O::= N:$FF K:div N:2 K:mod N:.5"},
		{"s = 'it''s' + \"two\nlines\"", "I:s O:= S:it S:s O:+ S:two\nlines"},
		{"x<<=1; y>=2 // gone\n/* gone\ntoo */ z", "I:x O:<<= N:1 O:; I:y O:>= N:2 I:z"},
		{"#event Collision with pacman\naction_sound(bonus, 0);", "D:event Collision with pacman I:action_sound O:( I:bonus O:, N:0 O:) O:;"},
		{"a = b # c", "I:a O:= I:b I:# I:c"},
		{"with (other) instance_destroy()", "K:with O:( I:other O:) I:instance_destroy O:( O:)"},
	} {
		got := kinds(Lex(c.src))
		got = strings.ReplaceAll(got, "Il:", "I:") // Illegal and Ident share a letter; told apart below
		if got != c.want {
			t.Errorf("Lex(%q)\n got %s\nwant %s", c.src, got, c.want)
		}
	}
	if k := Lex("a = b # c")[3].Kind; k != Illegal {
		t.Errorf("a '#' inside a line: %v, want Illegal", k)
	}
	if p := Lex("a\n  b")[1].Pos; p != (Pos{2, 3}) {
		t.Errorf("position %v, want 2:3", p)
	}
}

// TestLexCorpus: the recovered GML of the two sample games (sibling checkouts
// of the Stelo and Pacmanio repos) lexes without an Illegal token — whole
// scripts, and the code actions inside the object files. (The objects' other
// lines are drag-and-drop actions written out by the recovery tool, with text
// arguments unquoted: not GML; the importer reads actions from the game's
// data instead.)
func TestLexCorpus(t *testing.T) {
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
	n := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		code := string(src)
		if strings.Contains(f, "/objects/") || strings.Contains(f, "/timelines/") {
			code = codeActions(code)
		}
		for _, tok := range Lex(code) {
			n++
			if tok.Kind == Illegal {
				t.Errorf("%s:%s: illegal %q", f, tok.Pos, tok.Text)
			}
		}
	}
	t.Logf("%d files, %d tokens", len(files), n)
}

// codeActions keeps an object file's code actions (between "// --- code
// action ---" and "// --- end code ---"), blanking the other lines so the
// positions stay right.
func codeActions(src string) string {
	lines := strings.Split(src, "\n")
	in := false
	for i, l := range lines {
		switch strings.TrimSpace(l) {
		case "// --- code action ---":
			in = true
			continue
		case "// --- end code ---":
			in = false
		}
		if !in {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}
