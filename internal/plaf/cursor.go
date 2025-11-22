package plaf

// #include "platform.h"
import "C"

// GetCursorPos returns the last reported position of the cursor.
func (w *Window) GetCursorPos() (x, y float64) {
	var xpos, ypos C.double
	C.plafGetCursorPos(w.data, &xpos, &ypos)
	return float64(xpos), float64(ypos)
}

// SetCursorPos sets the position of the cursor. The specified window must be focused. If the window does not have focus
// when this function is called, it fails silently.
func (w *Window) SetCursorPos(xpos, ypos float64) {
	C.plafSetCursorPos(w.data, C.double(xpos), C.double(ypos))
}
