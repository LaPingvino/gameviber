package golang_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/gameviber/lang/golang"
)

// The generated packages build with the real gv package and run natively the
// same as the interpreter runs the same files: an event in statement form, a
// whole file with its own helper and imports, and a script called from GML.
func TestGenerateRunsLikeTheInterpreter(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a program")
	}
	files := []golang.GenFile{
		// a statement event calling the script by its GameViber name
		{Path: "objects/p/create.go", Src: "g.Set(\"x\", g.Num(\"x\")+g.Call(\"twice\", 3).Num())\ng.Set(\"label\", g.Call(\"string\", 7).Str()+\"!\")"},
		{Path: "objects/p/step.go", Src: "// whole\npackage main\n\nimport (\n\t\"gv\"\n\t\"math\"\n)\n\nfunc helper() float64 { return math.Hypot(3, 4) }\n\nfunc Event(g *gv.Ctx) {\n\tg.Set(\"h\", helper())\n}\n"},
		{Path: "scripts/twice.go", Src: "return g.Call(\"real\", args[0].Num()*2)", Script: true},
	}

	// a scratch package inside this module, so the imports resolve
	root, err := os.MkdirTemp(".", "gen-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	srcs, err := golang.Generate(files, "main", "github.com/LaPingvino/gameviber/lang/golang/"+filepath.Base(root))
	if err != nil {
		t.Fatal(err)
	}
	for rel, b := range srcs {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// main: the same files through CodeIn / ScriptIn (the compiled stand-ins
	// registered), then the interpreter on them, printing both
	main := `package main

import (
	"fmt"

	"github.com/LaPingvino/gameviber/lang/golang"
	"github.com/LaPingvino/gameviber/runner"
)

var files = map[string]string{` + func() string {
		var b strings.Builder
		for _, f := range files {
			b.WriteString("\n\t" + quote(f.Path) + ": " + quote(f.Src) + ",")
		}
		return b.String()
	}() + `
}

func run(label string) {
	o := &runner.Object{Name: "p"}
	g := &runner.Game{Objects: []*runner.Object{o}}
	if err := g.ScriptIn("scripts/twice.go", "twice", files["scripts/twice.go"]); err != nil {
		panic(err)
	}
	c, err := g.CodeIn("objects/p/create.go", files["objects/p/create.go"])
	if err != nil {
		panic(err)
	}
	s, err := g.CodeIn("objects/p/step.go", files["objects/p/step.go"])
	if err != nil {
		panic(err)
	}
	o.Events.Create = c
	r := &runner.Room{W: 64, H: 64, G: &runner.Globals{Vars: map[string]float64{}}}
	in := r.Create(o, 10, 0)
	s(r, in)
	x, _ := in.Get("x", nil)
	l, _ := in.Get("label", nil)
	h, _ := in.Get("h", nil)
	fmt.Printf("%s x=%v label=%v h=%v errors=%v\n", label, x, l, h, g.Errors)
}

func main() {
	fmt.Println("compiled in:", len(golang.Compiled), len(golang.CompiledScripts))
	run("compiled")
	golang.Compiled, golang.CompiledScripts = map[string]func(*gv.Ctx){}, map[string]func(*gv.Ctx, []gv.Value) gv.Value{}
	run("interpreted")
}
`
	main = strings.Replace(main, "\"github.com/LaPingvino/gameviber/lang/golang\"\n", "\"github.com/LaPingvino/gameviber/gv\"\n\t\"github.com/LaPingvino/gameviber/lang/golang\"\n", 1)
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	outb, err := exec.Command("go", "run", "./"+filepath.Base(root)).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, outb)
	}
	got := strings.Split(strings.TrimSpace(string(outb)), "\n")
	t.Logf("%s", outb)
	if len(got) != 3 || got[0] != "compiled in: 2 1" {
		t.Fatalf("output %q", got)
	}
	compiled, interpreted := strings.TrimPrefix(got[1], "compiled "), strings.TrimPrefix(got[2], "interpreted ")
	if compiled != interpreted || !strings.Contains(compiled, "x=16") || !strings.Contains(compiled, "h=5") {
		t.Errorf("compiled %q, interpreted %q", compiled, interpreted)
	}
}

func quote(s string) string { return "`" + strings.ReplaceAll(s, "`", "`+\"`\"+`") + "`" }
