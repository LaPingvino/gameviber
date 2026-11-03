package gmfile

import (
	"os"
	"strings"
	"testing"

	"github.com/LaPingvino/gameviber/gml"
)

// Every event and timeline moment of Stelo, turned into GML, parses.
func TestActionsToGMLStelo(t *testing.T) {
	d, err := os.ReadFile("../../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	e, err := OpenExe(d)
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.ReadProject()
	if err != nil {
		t.Fatal(err)
	}
	events := 0
	var sample string
	check := func(where string, acts []Action) {
		src, err := p.ActionsToGML(acts)
		if err != nil {
			t.Errorf("%s: %v", where, err)
			return
		}
		if _, err := gml.Parse(src); err != nil {
			t.Errorf("%s: %v\n%s", where, err, src)
			return
		}
		events++
		if strings.Contains(src, "action_set_score") && sample == "" {
			sample = where + ":\n" + src
		}
	}
	for _, o := range p.Objects {
		if o == nil {
			continue
		}
		for _, ev := range o.Events {
			check(o.Name+" "+ev.Type, ev.Actions)
		}
	}
	for _, tl := range p.Timelines {
		if tl == nil {
			continue
		}
		for _, m := range tl.Moments {
			check(tl.Name, m.Actions)
		}
	}
	t.Logf("%d events and moments; e.g. %s", events, sample)
}

func TestQuote(t *testing.T) {
	for in, want := range map[string]string{"abc": "'abc'", "it's": "\"it's\"", `a'b"c`: `'a' + "'" + 'b"c'`} {
		if got := quote(in); got != want {
			t.Errorf("quote(%q) = %s, want %s", in, got, want)
		}
	}
}
