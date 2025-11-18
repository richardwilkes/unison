package plaf

/*
#include "platform.h"

// workaround wrappers needed due to a cgo and/or LLVM bug.
// See: https://github.com/go-gl/glfw/issues/136
void *workaround_glfwGetCocoaWindow(plafWindow *w) {
	return (void *)glfwGetCocoaWindow(w);
}
*/
import "C"
import "unsafe"

// GetCocoaWindow returns the NSWindow of the window.
func (w *Window) GetCocoaWindow() unsafe.Pointer {
	ret := C.workaround_glfwGetCocoaWindow(w.data)
	panicError()
	return ret
}
