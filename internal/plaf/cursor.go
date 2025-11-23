package plaf

// #include "platform.h"
import "C"

import (
	"image"
	"image/draw"
)

// StandardCursor corresponds to a standard cursor icon.
type StandardCursor int

// Standard cursors
const (
	ArrowCursor     StandardCursor = C.STD_CURSOR_ARROW
	IBeamCursor     StandardCursor = C.STD_CURSOR_IBEAM
	CrosshairCursor StandardCursor = C.STD_CURSOR_CROSSHAIR
	HandCursor      StandardCursor = C.STD_CURSOR_POINTING_HAND
	HResizeCursor   StandardCursor = C.STD_CURSOR_HORIZONTAL_RESIZE
	VResizeCursor   StandardCursor = C.STD_CURSOR_VERTICAL_RESIZE
)

// Cursor represents a cursor.
type Cursor struct {
	data *C.plafCursor
}

// CreateCursor creates a new custom cursor image that can be set for a window with SetCursor.
// The cursor can be destroyed with Destroy. Any remaining cursors are destroyed by Terminate.
//
// The image is ideally provided in the form of *image.NRGBA.
// The pixels are 32-bit, little-endian, non-premultiplied RGBA, i.e. eight
// bits per channel with the red channel first. They are arranged canonically
// as packed sequential rows, starting from the top-left corner. If the image
// type is not *image.NRGBA, it will be converted to it.
//
// The cursor hotspot is specified in pixels, relative to the upper-left corner of the cursor image.
// Like all other coordinate systems in PLAF, the X-axis points to the right and the Y-axis points down.
func CreateCursor(img image.Image, xhot, yhot int) *Cursor {
	imgC, free := imageToPLAF(img)
	cursor := C.plafCreateCursor(&imgC, C.int(xhot), C.int(yhot))
	free()
	panicError()
	return &Cursor{cursor}
}

// CreateStandardCursor returns a cursor with a standard shape,
// that can be set for a window with SetCursor.
func CreateStandardCursor(shape StandardCursor) *Cursor {
	cursor := C.plafCreateStandardCursor(C.int(shape))
	panicError()
	return &Cursor{cursor}
}

// Destroy destroys a cursor previously created with CreateCursor.
// Any remaining cursors will be destroyed by Terminate.
func (c *Cursor) Destroy() {
	C.plafDestroyCursor(c.data)
	panicError()
}

// imageToPLAF converts img to be compatible with C.plafImageData.
func imageToPLAF(img image.Image) (r C.plafImageData, free func()) {
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
