package plaf

//#include "platform.h"
import "C"

// MakeContextCurrent makes the context of the window current.
func (w *Window) MakeContextCurrent() {
	C.plafMakeContextCurrent(w.data)
}

// DetachCurrentContext detaches the current context.
func DetachCurrentContext() {
	C.plafMakeContextCurrent(nil)
}

// GetCurrentContext returns the window whose context is current.
func GetCurrentContext() *Window {
	if C._plaf.wndWithCurrentCtx == nil {
		return nil
	}
	return windows.get(C._plaf.wndWithCurrentCtx)
}

// SwapBuffers swaps the front and back buffers of the window.
func (w *Window) SwapBuffers() {
	C.plafSwapBuffers(w.data)
}
