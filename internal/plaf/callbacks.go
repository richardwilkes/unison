package plaf

// #include "platform.h"
import "C"

import (
	"unsafe"

	"github.com/richardwilkes/unison/internal/mac"
)

//export goCharCallback
func goCharCallback(window *C.plafWindow, ch C.uint) {
	if w := windows.get(window); w != nil && w.CharCallback != nil {
		w.CharCallback(w, rune(ch))
	}
}

//export goCursorEnterCallback
func goCursorEnterCallback(window *C.plafWindow, entered C.bool) {
	if w := windows.get(window); w != nil && w.CursorEnterCallback != nil {
		w.CursorEnterCallback(w, bool(entered))
	}
}

//export goCursorPosCallback
func goCursorPosCallback(window *C.plafWindow, x, y C.double) {
	if w := windows.get(window); w != nil && w.CursorPosCallback != nil {
		w.CursorPosCallback(w, float64(x), float64(y))
	}
}

//export goDropCallback
func goDropCallback(window *C.plafWindow, count C.int, data **C.char) {
	if w := windows.get(window); w.DropCallback != nil {
		dataSlice := make([]string, int(count))
		list := unsafe.Slice(data, int(count))
		for i := range dataSlice {
			dataSlice[i] = C.GoString(list[i])
		}
		w.DropCallback(w, dataSlice)
	}
}

//export goKeyCallback
func goKeyCallback(window *C.plafWindow, key, scancode, action, mods C.int) {
	if w := windows.get(window); w != nil && w.KeyCallback != nil {
		w.KeyCallback(w, Key(key), int(scancode), Action(action), ModifierKey(mods))
	}
}

//export goMonitorCallback
func goMonitorCallback(monitor *C.plafMonitor, connected C.bool) {
	if MonitorCallback != nil {
		MonitorCallback(&Monitor{data: monitor}, bool(connected))
	}
}

//export goMouseButtonCallback
func goMouseButtonCallback(window *C.plafWindow, button, action, mods C.int) {
	if w := windows.get(window); w != nil && w.MouseButtonCallback != nil {
		w.MouseButtonCallback(w, MouseButton(button), Action(action), ModifierKey(mods))
	}
}

//export goScrollCallback
func goScrollCallback(window *C.plafWindow, xOffset, yOffset C.double) {
	if w := windows.get(window); w != nil && w.ScrollCallback != nil {
		w.ScrollCallback(w, float64(xOffset), float64(yOffset))
	}
}

//export goWindowCloseCallback
func goWindowCloseCallback(window *C.plafWindow) {
	if w := windows.get(window); w != nil && w.WindowCloseCallback != nil {
		w.WindowCloseCallback(w)
	}
}

//export goWindowContentScaleCallback
func goWindowContentScaleCallback(window *C.plafWindow) {
	if w := windows.get(window); w != nil && w.WindowContentScaleCallback != nil {
		w.WindowContentScaleCallback(w)
	}
}

//export goWindowFocusCallback
func goWindowFocusCallback(window *C.plafWindow, focused C.bool) {
	if w := windows.get(window); w != nil && w.WindowFocusCallback != nil {
		w.WindowFocusCallback(w, bool(focused))
	}
}

//export goWindowMinimizeCallback
func goWindowMinimizeCallback(window *C.plafWindow, minimized C.bool) {
	if w := windows.get(window); w != nil && w.WindowMinimizeCallback != nil {
		w.WindowMinimizeCallback(w, bool(minimized))
	}
}

//export goWindowMaximizeCallback
func goWindowMaximizeCallback(window *C.plafWindow, maximized C.bool) {
	if w := windows.get(window); w != nil && w.WindowMaximizeCallback != nil {
		w.WindowMaximizeCallback(w, bool(maximized))
	}
}

//export goWindowPosCallback
func goWindowPosCallback(window *C.plafWindow) {
	if w := windows.get(window); w != nil && w.WindowPosCallback != nil {
		w.WindowPosCallback(w)
	}
}

//export goWindowDrawCallback
func goWindowDrawCallback(window *C.plafWindow) {
	if w := windows.get(window); w != nil && w.WindowDrawCallback != nil {
		w.WindowDrawCallback(w)
	}
}

//export goWindowSizeCallback
func goWindowSizeCallback(window *C.plafWindow) {
	if w := windows.get(window); w != nil && w.WindowSizeCallback != nil {
		w.WindowSizeCallback(w)
	}
}

// OpenFilesCallback is called on macOS (and no other platforms, currently) when a user double-clicks on your app's
// documents.
var OpenFilesCallback func([]string)

//export goAppOpenURLsCallback
func goAppOpenURLsCallback(a C.CFArrayRef) {
	if OpenFilesCallback != nil {
		if urls := mac.Array(a).ArrayOfURLToStringSlice(); len(urls) > 0 {
			OpenFilesCallback(urls)
		}
	}
}
