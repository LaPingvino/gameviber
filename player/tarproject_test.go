package player

import (
	"encoding/binary"
	"testing"
)

func TestAppendedProject(t *testing.T) {
	exe, tarData := []byte("\x7fELF the player's bytes"), []byte("the project's tar")
	var size [8]byte
	binary.LittleEndian.PutUint64(size[:], uint64(len(tarData)))
	whole := append(append(append(append([]byte{}, exe...), tarData...), size[:]...), appendedMarker...)
	got, ok := appendedProject(whole)
	if !ok || string(got) != string(tarData) {
		t.Fatalf("appended: %v %q", ok, got)
	}
	if _, ok := appendedProject(exe); ok {
		t.Error("a plain player has no project")
	}
	binary.LittleEndian.PutUint64(whole[len(whole)-16:], 1<<40) // a length past the start
	if _, ok := appendedProject(whole); ok {
		t.Error("a bad length was accepted")
	}
}
