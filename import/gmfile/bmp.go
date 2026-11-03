package gmfile

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// DecodeBMP reads the stored images: uncompressed 24-bit BMPs (all that GM
// 5.3 games hold — checked on Stelo and Pacmanio). transparent applies GM's
// rule: the bottom-left pixel's colour is see-through.
func DecodeBMP(b []byte, transparent bool) (*image.NRGBA, error) {
	if len(b) < 54 || string(b[:2]) != "BM" {
		return nil, fmt.Errorf("not a BMP")
	}
	off := int(binary.LittleEndian.Uint32(b[10:]))
	w := int(int32(binary.LittleEndian.Uint32(b[18:])))
	h := int(int32(binary.LittleEndian.Uint32(b[22:])))
	bpp := binary.LittleEndian.Uint16(b[28:])
	if comp := binary.LittleEndian.Uint32(b[30:]); bpp != 24 || comp != 0 {
		return nil, fmt.Errorf("BMP with %d bits, compression %d (only 24-bit uncompressed)", bpp, comp)
	}
	bottomUp := h > 0
	if h < 0 {
		h = -h
	}
	stride := (w*3 + 3) &^ 3
	if w <= 0 || off+stride*h > len(b) {
		return nil, fmt.Errorf("BMP %dx%d: data too short", w, h)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		row := y
		if bottomUp {
			row = h - 1 - y
		}
		src := b[off+row*stride:]
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{src[3*x+2], src[3*x+1], src[3*x], 255})
		}
	}
	if transparent && h > 0 && w > 0 {
		key := img.NRGBAAt(0, h-1)
		for i := 0; i < len(img.Pix); i += 4 {
			if img.Pix[i] == key.R && img.Pix[i+1] == key.G && img.Pix[i+2] == key.B {
				img.Pix[i+3] = 0
			}
		}
	}
	return img, nil
}
