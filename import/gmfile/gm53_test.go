package gmfile

import (
	"os"
	"testing"
)

// Stelo's 2006 executable (in the Stelo repo, a sibling checkout) opens:
// marker, seed, the project header and format 530.
func TestOpenExe(t *testing.T) {
	d, err := os.ReadFile("../../../stelo/win/stelo.exe")
	if err != nil {
		t.Skip("Stelo's exe is not checked out next to this repo")
	}
	e, err := OpenExe(d)
	if err != nil {
		t.Fatal(err)
	}
	if e.Version != 530 {
		t.Errorf("format %d, want 530", e.Version)
	}
	t.Logf("seed %d, project at file offset %d", e.Seed, e.P+e.Base)
}

func TestSwapTables(t *testing.T) {
	tab, inv := swapTables(12345)
	seen := map[byte]bool{}
	for i := range 256 {
		seen[tab[i]] = true
		if inv[tab[i]] != byte(i) {
			t.Fatalf("inverse wrong at %d", i)
		}
	}
	if len(seen) != 256 || tab[0] != 0 {
		t.Error("not a permutation keeping 0")
	}
}

func TestCP1252(t *testing.T) {
	if got := cp1252([]byte{'a', 0x80, 0xE9, 0x93}); got != "a€é“" {
		t.Errorf("%q", got)
	}
}
