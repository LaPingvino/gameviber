package gmfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestMergeTOML(t *testing.T) {
	base := "name = \"Coins\"\nroom_order = [\"intro\", \"level1\", \"level2\"]\n[settings]\n  author = \"Joop\"\n  icon = \"player\"\n[constants]\n  LIVES = \"3\"\n"
	over := "name = \"Coins (demo)\"\nroom_order = [\"intro\", \"level1\"]\n[constants]\n  DEMO = \"1\"\n"
	got, err := MergeTOML([]byte(base), []byte(over))
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Name      string
		RoomOrder []string `toml:"room_order"`
		Settings  map[string]string
		Constants map[string]string
	}
	if _, err := toml.Decode(string(got), &g); err != nil {
		t.Fatal(err)
	}
	if g.Name != "Coins (demo)" || strings.Join(g.RoomOrder, ",") != "intro,level1" {
		t.Errorf("replaced values: %+v", g)
	}
	if g.Settings["author"] != "Joop" || g.Settings["icon"] != "player" || g.Constants["LIVES"] != "3" || g.Constants["DEMO"] != "1" {
		t.Errorf("merged tables: %+v", g)
	}
	if _, err := MergeTOML([]byte(base), []byte("room_order = [")); err == nil {
		t.Error("a broken variant was accepted")
	}
}

func TestReadDirVariant(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "game.toml"), []byte("name = \"Full\"\nroom_order = []\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "demo.toml"), []byte("name = \"Demo\"\n[constants]\n  DEMO = \"1\"\n"), 0o644)
	p, err := ReadDirVariant(dir, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "Demo" || len(p.Settings.Constants) != 1 || p.Settings.Constants[0] != [2]string{"DEMO", "1"} {
		t.Errorf("variant: name %q constants %v", p.Name(), p.Settings.Constants)
	}
	if p, err := ReadDirVariant(dir, ""); err != nil || p.Name() != "Full" {
		t.Errorf("no variant: %v %q", err, p.Name())
	}
	if _, err := ReadDirVariant(dir, "nosuch"); err == nil || !strings.Contains(err.Error(), "nosuch.toml") {
		t.Errorf("a missing variant: %v", err)
	}
}
