package gmfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/LaPingvino/gameviber/gml"
)

// The two sample games, each from its 2006 executable (sibling checkouts,
// never in this repo), with gm53-reader's project.json to compare against.
var sampleGames = []struct{ name, exe, recovered string }{
	{"Stelo", "../../../stelo/win/stelo.exe", "../../../stelo/original/project.json"},
	{"Pacmanio", "../../../pacmanio/win/pacmanio.exe", "../../../pacmanio-recovered/pacmanio/project.json"},
}

// Each game imports (objects and action counts as gm53-reader read them),
// exports, and every exported .gml parses. The functions the code calls are
// logged: the runner's work list.
func TestImportSampleGames(t *testing.T) {
	for _, g := range sampleGames {
		t.Run(g.name, func(t *testing.T) {
			d, err := os.ReadFile(g.exe)
			if err != nil {
				t.Skip(g.name + "'s exe is not checked out next to this repo")
			}
			e, err := OpenExe(d)
			if err != nil {
				t.Fatal(err)
			}
			p, err := e.ReadProject()
			if err != nil {
				t.Fatal(err)
			}
			if js, err := os.ReadFile(g.recovered); err == nil {
				var want struct {
					Objects []*struct {
						Events []struct {
							Actions []any `json:"actions"`
						} `json:"events"`
					} `json:"objects"`
				}
				json.Unmarshal(js, &want)
				n, m := 0, 0
				for _, o := range want.Objects {
					if o != nil {
						for _, ev := range o.Events {
							n += len(ev.Actions)
						}
					}
				}
				for _, o := range p.Objects {
					if o != nil {
						for _, ev := range o.Events {
							m += len(ev.Actions)
						}
					}
				}
				if len(p.Objects) != len(want.Objects) || n != m {
					t.Errorf("%d objects / %d actions; gm53-reader: %d / %d", len(p.Objects), m, len(want.Objects), n)
				}
			}
			dir := t.TempDir()
			notes, err := p.Export(dir, g.name)
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range notes {
				t.Log("note:", n)
			}
			calls := map[string]bool{}
			files := 0
			filepath.WalkDir(dir, func(path string, de os.DirEntry, err error) error {
				if !strings.HasSuffix(path, ".gml") {
					return nil
				}
				src, _ := os.ReadFile(path)
				list, err := gml.Parse(string(src))
				if err != nil {
					t.Errorf("%s: %v", path, err)
					return nil
				}
				files++
				collectCalls(list, calls)
				return nil
			})
			var names []string
			for n := range calls {
				names = append(names, n)
			}
			sort.Strings(names)
			t.Logf("%d GML files; calls: %s", files, strings.Join(names, " "))
		})
	}
}

func collectCalls(list []gml.Stmt, out map[string]bool) {
	var expr func(e gml.Expr)
	expr = func(e gml.Expr) {
		switch e := e.(type) {
		case *gml.Call:
			out[e.Fn] = true
			for _, a := range e.Args {
				expr(a)
			}
		case *gml.Unary:
			expr(e.X)
		case *gml.Binary:
			expr(e.L)
			expr(e.R)
		case *gml.Field:
			expr(e.X)
		case *gml.Index:
			expr(e.X)
			for _, i := range e.I {
				expr(i)
			}
		}
	}
	var stmt func(s gml.Stmt)
	stmt = func(s gml.Stmt) {
		switch s := s.(type) {
		case *gml.Block:
			for _, x := range s.List {
				stmt(x)
			}
		case *gml.Assign:
			expr(s.Target)
			expr(s.Value)
		case *gml.ExprStmt:
			expr(s.X)
		case *gml.If:
			expr(s.Cond)
			stmt(s.Then)
			if s.Else != nil {
				stmt(s.Else)
			}
		case *gml.While:
			expr(s.Cond)
			stmt(s.Body)
		case *gml.Do:
			stmt(s.Body)
			expr(s.Until)
		case *gml.Repeat:
			expr(s.Count)
			stmt(s.Body)
		case *gml.For:
			stmt(s.Init)
			expr(s.Cond)
			stmt(s.Step)
			stmt(s.Body)
		case *gml.With:
			expr(s.Target)
			stmt(s.Body)
		case *gml.Switch:
			expr(s.X)
			for _, c := range s.Cases {
				for _, x := range c.Body {
					stmt(x)
				}
			}
		case *gml.Return:
			expr(s.X)
		}
	}
	for _, s := range list {
		stmt(s)
	}
}

// A sprite read back from the text project has its frame size (a tag on two
// fields once left it 0 × h, frames cut empty).
func TestReadDirSpriteSize(t *testing.T) {
	d, err := os.ReadFile("../../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	e, _ := OpenExe(d)
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := p.Export(dir, "Stelo"); err != nil {
		t.Fatal(err)
	}
	q, err := ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range q.Sprites {
		if len(s.Images) > 0 && (s.W == 0 || s.H == 0 || s.Images[0].Bounds().Dx() != int(s.W)) {
			t.Fatalf("sprite %s: %dx%d, frame %v", s.Name, s.W, s.H, s.Images[0].Bounds())
		}
	}
}
