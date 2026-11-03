// Package gmfile reads GameMaker games and projects (docs/DESIGN.md §1):
// first the GameMaker 5.3 executable, as Stelo's and Pacmanio's sources were
// recovered — a Go port of gm53-reader (github.com/LaPingvino/gm53-reader,
// MIT, the same author), which was written from LateralGM's GmFileReader
// (.gmd 5.3a layout) and the GM4 decompiler notes, read but not run.
package gmfile

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	markOffset = 1500000 // where the GM 5 runner keeps the game's data
	exeMagic   = 1230500 // GM 5 exe marker
	gmdMagic   = 1234321 // GM project header
)

// swapTables is the GM 5 cipher: a byte table permuted by the seed
// (identity; for i in 1…10000: j = 1 + (i·seed) mod 254, swap t[j], t[j+1]),
// and its inverse.
func swapTables(seed uint32) (t, inv [256]byte) {
	for i := range t {
		t[i] = byte(i)
	}
	for i := uint64(1); i <= 10000; i++ {
		j := 1 + (i*uint64(seed))%254
		t[j], t[j+1] = t[j+1], t[j]
	}
	for i := range t {
		inv[t[i]] = byte(i)
	}
	return t, inv
}

// findMarker finds the GM 5 exe marker: normally at 1500000, else the first.
func findMarker(d []byte) (int, error) {
	if len(d) > markOffset+8 && binary.LittleEndian.Uint32(d[markOffset:]) == exeMagic {
		return markOffset, nil
	}
	var m [4]byte
	binary.LittleEndian.PutUint32(m[:], exeMagic)
	p := bytes.Index(d, m[:])
	if p < 0 {
		return 0, errors.New("not a GameMaker 5 game: no exe marker (1230500)")
	}
	return p, nil
}

// Exe is an opened GM 5.3 executable: the data deciphered, a reader at the
// start of the project body (after the header and format version).
type Exe struct {
	Seed    uint32
	Version int // the data format (530 for 5.3)
	*Reader
}

// OpenExe deciphers a GM 5.3 executable's data and finds the project in it.
func OpenExe(d []byte) (*Exe, error) {
	m, err := findMarker(d)
	if err != nil {
		return nil, err
	}
	seed := binary.LittleEndian.Uint32(d[m+4:])
	_, inv := swapTables(seed)
	base := m + 8
	raw := d[base:]
	dec := make([]byte, len(raw))
	for i, c := range raw {
		dec[i] = inv[c]
	}
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], gmdMagic)
	p := bytes.Index(dec, hdr[:])
	if p < 0 {
		return nil, errors.New("GameMaker 5 exe without a project header (1234321) after deciphering")
	}
	r := &Reader{D: dec, Raw: raw, Base: base, P: p + 4}
	v, err := r.I4()
	if err != nil {
		return nil, err
	}
	return &Exe{Seed: seed, Version: int(v), Reader: r}, nil
}

// Reader reads the deciphered stream (and, for blocks the runner stores
// unciphered, the raw one at the same position). Errors say the file offset.
type Reader struct {
	D, Raw []byte
	Base   int // the file offset of D[0]
	P      int
}

func (r *Reader) errf(format string, a ...any) error {
	return fmt.Errorf("at file offset %d: %s", r.P+r.Base, fmt.Sprintf(format, a...))
}

// I4 reads a little-endian int32.
func (r *Reader) I4() (int32, error) {
	if r.P+4 > len(r.D) {
		return 0, r.errf("unexpected end of data")
	}
	v := int32(binary.LittleEndian.Uint32(r.D[r.P:]))
	r.P += 4
	return v, nil
}

// Bool reads an int32 as a boolean.
func (r *Reader) Bool() (bool, error) {
	v, err := r.I4()
	return v != 0, err
}

// Double reads a little-endian float64.
func (r *Reader) Double() (float64, error) {
	if r.P+8 > len(r.D) {
		return 0, r.errf("unexpected end of data")
	}
	v := math.Float64frombits(binary.LittleEndian.Uint64(r.D[r.P:]))
	r.P += 8
	return v, nil
}

// String reads a length-prefixed Windows-1252 string.
func (r *Reader) String() (string, error) {
	n, err := r.I4()
	if err != nil {
		return "", err
	}
	if n < 0 || n > 10_000_000 || r.P+int(n) > len(r.D) {
		r.P -= 4
		return "", r.errf("bad string length %d", n)
	}
	s := cp1252(r.D[r.P : r.P+int(n)])
	r.P += int(n)
	return s, nil
}

// Blob reads n bytes, deciphered or (plain) as stored.
func (r *Reader) Blob(n int, plain bool) ([]byte, error) {
	if n < 0 || r.P+n > len(r.D) {
		return nil, r.errf("bad block length %d", n)
	}
	src := r.D
	if plain {
		src = r.Raw
	}
	v := bytes.Clone(src[r.P : r.P+n])
	r.P += n
	return v, nil
}

// ZBlob reads a length-prefixed zlib block, deciphered or else as stored
// (the runner keeps some blocks unciphered); nil when neither inflates.
func (r *Reader) ZBlob() ([]byte, error) {
	n, err := r.I4()
	if err != nil {
		return nil, err
	}
	if n < 0 || r.P+int(n) > len(r.D) {
		return nil, r.errf("bad block length %d", n)
	}
	start := r.P
	r.P += int(n)
	for _, src := range [][]byte{r.D, r.Raw} {
		zr, err := zlib.NewReader(bytes.NewReader(src[start : start+int(n)]))
		if err != nil {
			continue
		}
		out, err := io.ReadAll(zr)
		if err == nil {
			return out, nil
		}
	}
	return nil, nil
}

// cp1252 decodes Windows-1252 (the 2006 games' Esperanto uses the x-system,
// so mostly ASCII; 0x80–0x9F map to their Windows characters).
func cp1252(b []byte) string {
	high := [32]rune{'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0x8D, 'Ž', 0x8F,
		0x90, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0x9D, 'ž', 'Ÿ'}
	rs := make([]rune, len(b))
	for i, c := range b {
		switch {
		case c >= 0x80 && c < 0xA0:
			rs[i] = high[c-0x80]
		default:
			rs[i] = rune(c)
		}
	}
	return string(rs)
}
