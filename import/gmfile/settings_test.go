package gmfile

import (
	"strings"
	"testing"
	"testing/fstest"
)

// [settings] touch and typing: read from game.toml; an unknown touch mode is
// an error that names the choices.
func TestReadTouchTyping(t *testing.T) {
	read := func(settings string) (*Project, error) {
		return ReadFS(fstest.MapFS{"game.toml": {Data: []byte("name = \"t\"\n[settings]\n" + settings)}}, ".")
	}
	p, err := read("touch = \"tap\"\ntyping = true\n")
	if err != nil {
		t.Fatal(err)
	}
	if p.Touch != "tap" || !p.Typing {
		t.Errorf("touch %q typing %v", p.Touch, p.Typing)
	}
	if p, err = read(""); err != nil || p.Touch != "" || p.Typing {
		t.Errorf("defaults: %+v %v", p, err)
	}
	if _, err := read("touch = \"swipe\"\n"); err == nil || !strings.Contains(err.Error(), `"tap"`) {
		t.Errorf("unknown touch mode: %v", err)
	}
}
