package runner

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/LaPingvino/gameviber/gml"
)

// FileStore is where a game's files live (GM 5's file_* functions): the
// working directory on the desktop; a backend may give another (a browser's
// storage).
type FileStore interface {
	Read(name string) ([]byte, error)
	Write(name string, data []byte) error
	Exists(name string) bool
	Delete(name string) error
}

// osFiles is the working directory.
type osFiles struct{}

func (osFiles) Read(name string) ([]byte, error)     { return os.ReadFile(name) }
func (osFiles) Write(name string, data []byte) error { return os.WriteFile(name, data, 0o644) }
func (osFiles) Exists(name string) bool              { _, err := os.Stat(name); return err == nil }
func (osFiles) Delete(name string) error             { return os.Remove(name) }

// openFile is GM 5's one open file: read whole when opened for reading (pos
// the read position), or collected and written on file_close.
type openFile struct {
	name    string
	data    []byte
	pos     int
	writing bool
}

func (g *Game) files() FileStore {
	if g.Files != nil {
		return g.Files
	}
	return osFiles{}
}

// fileFuncs are GM 5's file functions (one file open at a time, as GM 5;
// lines end in \r\n as it wrote them on Windows; reading takes \n too).
// ASSUMPTION [engine]: file_write_real writes the number as string() does.
func (g *Game) fileFuncs() map[string]gml.Func {
	zero := gml.RealV(0)
	arg := func(a []gml.Value, i int) string {
		if i < len(a) {
			return a[i].String()
		}
		return ""
	}
	open := func(a []gml.Value, writing, appending bool) (gml.Value, error) {
		g.closeFile()
		f := &openFile{name: arg(a, 0), writing: writing}
		if !writing || appending {
			b, err := g.files().Read(f.name)
			if err != nil && !appending {
				return zero, fmt.Errorf("file_open_read: %w", err)
			}
			f.data = b
		}
		g.file = f
		return zero, nil
	}
	write := func(s string) {
		if g.file != nil && g.file.writing {
			g.file.data = append(g.file.data, s...)
		}
	}
	reading := func() *openFile {
		if g.file != nil && !g.file.writing {
			return g.file
		}
		return nil
	}
	return map[string]gml.Func{
		"file_exists": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (fname)
			if g.files().Exists(arg(a, 0)) {
				return gml.RealV(1), nil
			}
			return zero, nil
		},
		"file_delete": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (fname)
			g.files().Delete(arg(a, 0))
			return zero, nil
		},
		"file_open_read":   func(in *gml.Interp, a []gml.Value) (gml.Value, error) { return open(a, false, false) },
		"file_open_write":  func(in *gml.Interp, a []gml.Value) (gml.Value, error) { return open(a, true, false) },
		"file_open_append": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { return open(a, true, true) },
		"file_close": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			return zero, g.closeFile()
		},
		"file_write_string": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (str)
			write(arg(a, 0))
			return zero, nil
		},
		"file_write_real": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // (x)
			if len(a) > 0 {
				v, _ := gml.Builtins["string"](in, []gml.Value{gml.RealV(a[0].N)})
				write(v.S)
			}
			return zero, nil
		},
		"file_writeln": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			write("\r\n")
			return zero, nil
		},
		"file_read_string": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // the rest of the line (not its end)
			f := reading()
			if f == nil {
				return gml.StrV(""), nil
			}
			end := f.pos
			for end < len(f.data) && f.data[end] != '\n' && f.data[end] != '\r' {
				end++
			}
			s := string(f.data[f.pos:end])
			f.pos = end
			return gml.StrV(s), nil
		},
		"file_read_real": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // a number (spaces before it skipped)
			f := reading()
			if f == nil {
				return zero, nil
			}
			for f.pos < len(f.data) && (f.data[f.pos] == ' ' || f.data[f.pos] == '\t') {
				f.pos++
			}
			end := f.pos
			for end < len(f.data) && strings.IndexByte("+-.0123456789eE", f.data[end]) >= 0 {
				end++
			}
			v, _ := strconv.ParseFloat(string(f.data[f.pos:end]), 64)
			f.pos = end
			return gml.RealV(v), nil
		},
		"file_readln": func(in *gml.Interp, a []gml.Value) (gml.Value, error) { // past the end of this line
			if f := reading(); f != nil {
				for f.pos < len(f.data) && f.data[f.pos] != '\n' {
					f.pos++
				}
				if f.pos < len(f.data) {
					f.pos++
				}
			}
			return zero, nil
		},
		"file_eof": func(in *gml.Interp, a []gml.Value) (gml.Value, error) {
			if f := reading(); f == nil || f.pos >= len(f.data) {
				return gml.RealV(1), nil
			}
			return zero, nil
		},
	}
}

// closeFile writes a file open for writing and forgets the open file.
func (g *Game) closeFile() error {
	f := g.file
	g.file = nil
	if f != nil && f.writing {
		return g.files().Write(f.name, f.data)
	}
	return nil
}
