package plaf

//#include "platform.h"
import "C"

import (
	"image"
	"image/draw"
)

// imageToGLFW converts img to be compatible with C.ImageData.
func imageToGLFW(img image.Image) (r C.ImageData, free func()) {
	b := img.Bounds()
	r.width = C.int(b.Dx())
	r.height = C.int(b.Dy())
	var pixels []byte
	if m, ok := img.(*image.NRGBA); ok && m.Stride == b.Dx()*4 {
		pixels = m.Pix[:m.PixOffset(m.Rect.Min.X, m.Rect.Max.Y)]
	} else {
		m = image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(m, m.Bounds(), img, b.Min, draw.Src)
		pixels = m.Pix
	}
	n := len(pixels)
	if n == 0 {
		r.pixels = nil
		return r, func() {}
	}
	ptr := C.CBytes(pixels)
	r.pixels = (*C.uchar)(ptr)
	return r, func() { C.free(ptr) }
}
