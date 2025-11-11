package plaf

// #include "platform.h"
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

//export goCharCallback
func goCharCallback(window unsafe.Pointer, ch C.uint) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fCharHolder(w, rune(ch))
}

//export goCharModsCallback
func goCharModsCallback(window unsafe.Pointer, ch C.uint, mods C.int) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fCharModsHolder(w, rune(ch), ModifierKey(mods))
}

//export goCursorEnterCallback
func goCursorEnterCallback(window unsafe.Pointer, entered C.int) {
	w := windows.get((*C.GLFWwindow)(window))
	hasEntered := glfwbool(entered)
	w.fCursorEnterHolder(w, hasEntered)
}

//export goCursorPosCallback
func goCursorPosCallback(window unsafe.Pointer, xpos, ypos C.double) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fCursorPosHolder(w, float64(xpos), float64(ypos))
}

//export goDropCallback
func goDropCallback(window unsafe.Pointer, count C.int, names **C.char) {
	w := windows.get((*C.GLFWwindow)(window))
	namesSlice := make([]string, int(count))
	list := unsafe.Slice(names, int(count))
	for i := range namesSlice {
		namesSlice[i] = C.GoString(list[i])
	}
	w.fDropHolder(w, namesSlice)
}

//export goErrorCallback
func goErrorCallback(code C.int, desc *C.char) {
	flushErrors()
	err := &Error{ErrorCode(code), C.GoString(desc)}
	select {
	case lastError <- err:
	default:
		fmt.Fprintln(os.Stderr, "go-gl/glfw: internal error: an uncaught error has occurred:", err)
		fmt.Fprintln(os.Stderr, "go-gl/glfw: Please report this in the Go package issue tracker.")
	}
}

//export goFramebufferSizeCallback
func goFramebufferSizeCallback(window unsafe.Pointer, width, height C.int) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fFramebufferSizeHolder(w, int(width), int(height))
}

//export goKeyCallback
func goKeyCallback(window unsafe.Pointer, key, scancode, action, mods C.int) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fKeyHolder(w, Key(key), int(scancode), Action(action), ModifierKey(mods))
}

//export goMonitorCallback
func goMonitorCallback(monitor unsafe.Pointer, event C.int) {
	fMonitorHolder(&Monitor{(*C.GLFWmonitor)(monitor)}, PeripheralEvent(event))
}

//export goMouseButtonCallback
func goMouseButtonCallback(window unsafe.Pointer, button, action, mods C.int) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fMouseButtonHolder(w, MouseButton(button), Action(action), ModifierKey(mods))
}

//export goScrollCallback
func goScrollCallback(window unsafe.Pointer, xoff, yoff C.double) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fScrollHolder(w, float64(xoff), float64(yoff))
}

//export goWindowCloseCallback
func goWindowCloseCallback(window unsafe.Pointer) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fCloseHolder(w)
}

//export goWindowContentScaleCallback
func goWindowContentScaleCallback(window unsafe.Pointer, x C.float, y C.float) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fContentScaleHolder(w, float32(x), float32(y))
}

//export goWindowFocusCallback
func goWindowFocusCallback(window unsafe.Pointer, focused C.int) {
	w := windows.get((*C.GLFWwindow)(window))
	isFocused := glfwbool(focused)
	w.fFocusHolder(w, isFocused)
}

//export goWindowIconifyCallback
func goWindowIconifyCallback(window unsafe.Pointer, iconified C.int) {
	isIconified := glfwbool(iconified)
	w := windows.get((*C.GLFWwindow)(window))
	w.fIconifyHolder(w, isIconified)
}

//export goWindowMaximizeCallback
func goWindowMaximizeCallback(window unsafe.Pointer, maximized C.int) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fMaximizeHolder(w, glfwbool(maximized))
}

//export goWindowPosCallback
func goWindowPosCallback(window unsafe.Pointer, xpos, ypos C.int) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fPosHolder(w, int(xpos), int(ypos))
}

//export goWindowRefreshCallback
func goWindowRefreshCallback(window unsafe.Pointer) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fRefreshHolder(w)
}

//export goWindowSizeCallback
func goWindowSizeCallback(window unsafe.Pointer, width, height C.int) {
	w := windows.get((*C.GLFWwindow)(window))
	w.fSizeHolder(w, int(width), int(height))
}
