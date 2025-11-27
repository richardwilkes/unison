package plaf

// #include "platform.h"
import "C"

import (
	"image"
	"image/draw"
	"runtime"
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
	plafCursor *C.plafCursor
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
func CreateCursor(img *image.NRGBA, xhot, yhot int) *Cursor {
	if img.Rect.Dx() < 1 || img.Rect.Dy() < 1 {
		return nil
	}
	var pinner runtime.Pinner
	defer pinner.Unpin()
	imgC := imageToCImageData(&pinner, img)
	//nolint:gocritic // Spurious lint flagging due to C code
	cursor := C.plafCreateCursor(&imgC, C.int(xhot), C.int(yhot))
	return &Cursor{plafCursor: cursor}
}

// CreateStandardCursor returns a cursor with a standard shape, that can be set for a window with SetCursor.
func CreateStandardCursor(shape StandardCursor) *Cursor {
	if cursor := C.plafCreateStandardCursor(C.int(shape)); cursor != nil {
		return &Cursor{plafCursor: cursor}
	}
	return nil
}

// Destroy a cursor previously created with CreateCursor.
func (c *Cursor) Destroy() {
	C.plafDestroyCursor(c.plafCursor)
}

// SetCursor sets the cursor image to be used when the cursor is over the client area
// of the specified window. The set cursor will only be visible when the cursor mode of the
// window is CursorNormal.
//
// On some platforms, the set cursor may not be visible unless the window also has input focus.
func (w *Window) SetCursor(c *Cursor) {
	if c == nil {
		w.plafWnd.cursor = nil
	} else {
		w.plafWnd.cursor = c.plafCursor
	}
	C.plafAdjustToCursorChange(w.plafWnd)
}

// GetCursorPos returns the last reported position of the cursor.
func (w *Window) GetCursorPos() (x, y float64) {
	var xpos, ypos C.double
	C.plafGetCursorPos(w.plafWnd, &xpos, &ypos)
	return float64(xpos), float64(ypos)
}

// SetCursorPos sets the position of the cursor. The specified window must be focused. If the window does not have focus
// when this function is called, it fails silently.
func (w *Window) SetCursorPos(x, y float64) {
	if !isInfOrNaN(x) && !isInfOrNaN(y) && w.IsFocused() {
		C.plafSetCursorPos(w.plafWnd, C.double(x), C.double(y))
	}
}

func imageToCImageData(pinner *runtime.Pinner, img *image.NRGBA) C.plafImageData {
	var r C.plafImageData
	w := img.Rect.Dx()
	h := img.Rect.Dy()
	r.width = C.int(w)
	r.height = C.int(h)
	var pixels []byte
	if img.Stride == w*4 {
		pixels = img.Pix[:img.PixOffset(img.Rect.Min.X, img.Rect.Max.Y)]
	} else {
		m := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(m, m.Bounds(), img, img.Rect.Min, draw.Src)
		pixels = m.Pix
	}
	r.pixels = (*C.uchar)(&pixels[0])
	pinner.Pin(r.pixels)
	return r
}
