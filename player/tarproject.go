package player

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"io"
	"testing/fstest"

	"github.com/LaPingvino/gameviber/import/gmfile"
)

// projectFromTar reads a packed project (gameviber web's project.tar, or the
// one an exported desktop game carries).
func projectFromTar(data []byte) (*gmfile.Project, error) {
	fsys := fstest.MapFS{}
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		fsys[h.Name] = &fstest.MapFile{Data: b}
	}
	return gmfile.ReadFS(fsys, ".")
}

// An exported desktop game (gameviber export -desktop) is this player with
// the project's tar appended, then the tar's length (8 bytes, little endian)
// and this marker.
const appendedMarker = "GVPROJ01"

// appendedProject is the tar appended to exe's bytes, if any.
func appendedProject(exe []byte) ([]byte, bool) {
	n := len(exe)
	if n < 16 || string(exe[n-8:]) != appendedMarker {
		return nil, false
	}
	size := binary.LittleEndian.Uint64(exe[n-16 : n-8])
	if size > uint64(n-16) {
		return nil, false
	}
	return exe[n-16-int(size) : n-16], true
}
