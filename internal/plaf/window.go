package plaf

//#include "platform.h"
import "C"

import (
	"image"
	"log/slog"
	"math"
	"sync"
	"unsafe"
)

// Internal window list stuff
type windowList struct {
	m map[*C.plafWindow]*Window
	l sync.Mutex
}

var windows = windowList{m: map[*C.plafWindow]*Window{}}

func (w *windowList) put(wnd *Window) {
	w.l.Lock()
	defer w.l.Unlock()
	w.m[wnd.plafWnd] = wnd
}

func (w *windowList) remove(wnd *C.plafWindow) {
	w.l.Lock()
	defer w.l.Unlock()
	delete(w.m, wnd)
}

func (w *windowList) get(wnd *C.plafWindow) *Window {
	w.l.Lock()
	defer w.l.Unlock()
	return w.m[wnd]
}

// Other values.
const (
	True     int = 1 // GL_TRUE
	False    int = 0 // GL_FALSE
	DontCare int = C.DONT_CARE
)

// WindowConfig holds the desired window configuration.
type WindowConfig struct {
	Resizable        bool
	Decorated        bool
	Transparent      bool
	Floating         bool
	MousePassThrough bool
}

// Window represents a window.
type Window struct {
	plafWnd                    *C.plafWindow
	CharCallback               func(w *Window, char rune)
	CursorEnterCallback        func(w *Window, entered bool)
	CursorPosCallback          func(w *Window, x, y float64)
	DropCallback               func(w *Window, data []string)
	KeyCallback                func(w *Window, key Key, code int, action Action, mods ModifierKey)
	MouseButtonCallback        func(w *Window, button MouseButton, action Action, mod ModifierKey)
	ScrollCallback             func(w *Window, xOffset, yOffset float64)
	WindowCloseCallback        func(w *Window)
	WindowContentScaleCallback func(w *Window)
	WindowDrawCallback         func(w *Window)
	WindowFocusCallback        func(w *Window, focused bool)
	WindowMaximizeCallback     func(w *Window, maximized bool)
	WindowMinimizeCallback     func(w *Window, minimized bool)
	WindowPosCallback          func(w *Window)
	WindowSizeCallback         func(w *Window)
}

// CreateWindow creates a window and its associated context. Most of the options
// controlling how the window and its context should be created are specified
// through Hint.
//
// Successful creation does not change which context is current. Before you can
// use the newly created context, you need to make it current using
// MakeContextCurrent.
//
// Note that the created window and context may differ from what you requested,
// as not all parameters and hints are hard constraints. This includes the size
// of the window, especially for full screen windows. To retrieve the actual
// attributes of the created window and context, use queries like
// Window.GetAttrib and Window.GetSize.
//
// To create the window at a specific position, make it initially invisible using
// the Visible window hint, set its position and then show it.
//
// If a fullscreen window is active, the screensaver is prohibited from starting.
//
// Windows: If the executable has an icon resource named PLAF_ICON, it will be
// set as the icon for the window. If no such icon is present, the IDI_WINLOGO
// icon will be used instead.
//
// Mac OS X: The PLAF window has no icon, as it is not a document window, but the
// dock icon will be the same as the application bundle's icon.
//
// This function may only be called from the main thread.
func CreateWindow(title string, cfg *WindowConfig, monitor *Monitor, share *Window) *Window {
	t := C.CString(title)
	defer C.free(unsafe.Pointer(t))
	var m *C.plafMonitor
	if monitor != nil {
		m = monitor.data
	}
	var s *C.plafWindow
	if share != nil {
		s = share.plafWnd
	}
	w := C.plafCreateWindow(t, (*C.plafWindowConfig)(unsafe.Pointer(cfg)), m, s)
	if w == nil {
		return nil
	}
	wnd := &Window{plafWnd: w}
	windows.put(wnd)
	return wnd
}

// Destroy destroys the specified window and its context. On calling this
// function, no further callbacks will be called for that window.
//
// This function may only be called from the main thread.
func (w *Window) Destroy() {
	windows.remove(w.plafWnd)
	C.plafDestroyWindow(w.plafWnd)
}

// ShouldClose reports the value of the close flag of the specified window.
func (w *Window) ShouldClose() bool {
	return bool(w.plafWnd.shouldClose)
}

// SetShouldClose sets the value of the close flag of the window. This can be
// used to override the user's attempt to close the window, or to signal that it
// should be closed.
func (w *Window) SetShouldClose(value bool) {
	w.plafWnd.shouldClose = C.bool(value)
}

// SetTitle sets the window title, encoded as UTF-8, of the window.
//
// This function may only be called from the main thread.
func (w *Window) SetTitle(title string) {
	t := C.CString(title)
	defer C.free(unsafe.Pointer(t))
	C.plafSetWindowTitle(w.plafWnd, t)
}

// SetIcon sets the icon of the specified window. If passed an array of candidate images,
// those of or closest to the sizes desired by the system are selected. If no images are
// specified, the window reverts to its default icon.
//
// The image is ideally provided in the form of *image.NRGBA.
// The pixels are 32-bit, little-endian, non-premultiplied RGBA, i.e. eight
// bits per channel with the red channel first. They are arranged canonically
// as packed sequential rows, starting from the top-left corner. If the image
// type is not *image.NRGBA, it will be converted to it.
//
// The desired image sizes varies depending on platform and system settings. The selected
// images will be rescaled as needed. Good sizes include 16x16, 32x32 and 48x48.
func (w *Window) SetIcon(images []*image.NRGBA) {
	cImgs := make([]C.plafImageData, 0, len(images))
	for _, img := range images {
		if img.Rect.Dx() > 0 && img.Rect.Dy() > 0 {
			cImgs = append(cImgs, imageToCImageData(img))
		}
	}
	if len(cImgs) == 0 {
		C.plafSetWindowIcon(w.plafWnd, 0, nil)
	} else {
		C.plafSetWindowIcon(w.plafWnd, C.int(len(images)), &cImgs[0])
		for i := range cImgs {
			C.free(unsafe.Pointer(cImgs[i].pixels))
		}
	}
}

// GetPos returns the position, in screen coordinates, of the upper-left
// corner of the client area of the window.
func (w *Window) GetPos() (x, y int) {
	var xpos, ypos C.int
	C.plafGetWindowPos(w.plafWnd, &xpos, &ypos)
	return int(xpos), int(ypos)
}

// SetPos sets the position, in screen coordinates, of the upper-left corner
// of the client area of the window.
//
// If it is a full screen window, this function does nothing.
//
// If you wish to set an initial window position you should create a hidden
// window (using Hint and Visible), set its position and then show it.
//
// It is very rarely a good idea to move an already visible window, as it will
// confuse and annoy the user.
//
// The window manager may put limits on what positions are allowed.
//
// This function may only be called from the main thread.
func (w *Window) SetPos(xpos, ypos int) {
	C.plafSetWindowPos(w.plafWnd, C.int(xpos), C.int(ypos))
}

// GetSize returns the size, in screen coordinates, of the client area of the
// specified window.
func (w *Window) GetSize() (width, height int) {
	var cWidth, cHeight C.int
	C.plafGetWindowSize(w.plafWnd, &cWidth, &cHeight)
	return int(cWidth), int(cHeight)
}

// SetSize sets the size, in screen coordinates, of the client area of the
// window.
//
// For full screen windows, this function selects and switches to the resolution
// closest to the specified size, without affecting the window's context. As the
// context is unaffected, the bit depths of the framebuffer remain unchanged.
//
// The window manager may put limits on what window sizes are allowed.
//
// This function may only be called from the main thread.
func (w *Window) SetSize(width, height int) {
	C.plafSetWindowSize(w.plafWnd, C.int(width), C.int(height))
}

// SetSizeLimits sets the size limits of the client area of the specified window.
// If the window is full screen or not resizable, this function does nothing.
//
// The size limits are applied immediately and may cause the window to be resized.
func (w *Window) SetSizeLimits(minw, minh, maxw, maxh int) {
	C.plafSetWindowSizeLimits(w.plafWnd, C.int(minw), C.int(minh), C.int(maxw), C.int(maxh))
}

// Resizable returns true if the window is allowed to be resized by the user.
func (w *Window) Resizable() bool {
	return w.plafWnd != nil && bool(w.plafWnd.resizable)
}

// GetFramebufferSize retrieves the size, in pixels, of the framebuffer of the
// specified window.
func (w *Window) GetFramebufferSize() (width, height int) {
	var cWidth, cHeight C.int
	C.plafGetFramebufferSize(w.plafWnd, &cWidth, &cHeight)
	return int(cWidth), int(cHeight)
}

// GetFrameSize retrieves the size, in screen coordinates, of each edge of the frame
// of the specified window. This size includes the title bar, if the window has one.
// The size of the frame may vary depending on the window-related hints used to create it.
//
// Because this function retrieves the size of each window frame edge and not the offset
// along a particular coordinate axis, the retrieved values will always be zero or positive.
func (w *Window) GetFrameSize() (left, top, right, bottom int) {
	var l, t, r, b C.int
	C.plafGetWindowFrameSize(w.plafWnd, &l, &t, &r, &b)
	return int(l), int(t), int(r), int(b)
}

// GetContentScale function retrieves the content scale for the specified
// window. The content scale is the ratio between the current DPI and the
// platform's default DPI. If you scale all pixel dimensions by this scale then
// your content should appear at an appropriate size. This is especially
// important for text and any UI elements.
//
// This function may only be called from the main thread.
func (w *Window) GetContentScale() (x, y float32) {
	var cX, cY C.float
	C.plafGetWindowContentScale(w.plafWnd, &cX, &cY)
	return float32(cX), float32(cY)
}

// GetOpacity function returns the opacity of the window, including any
// decorations.
//
// The opacity (or alpha) value is a positive finite number between zero and
// one, where zero is fully transparent and one is fully opaque. If the system
// does not support whole window transparency, this function always returns one.
//
// The initial opacity value for newly created windows is one.
//
// This function may only be called from the main thread.
func (w *Window) GetOpacity() float32 {
	return float32(C.plafGetWindowOpacity(w.plafWnd))
}

// SetOpacity function sets the opacity of the window, including any
// decorations. The opacity (or alpha) value is a positive finite number between
// zero and one, where zero is fully transparent and one is fully opaque.
//
// The initial opacity value for newly created windows is one.
//
// A window created with framebuffer transparency may not use whole window
// transparency. The results of doing this are undefined.
//
// This function may only be called from the main thread.
func (w *Window) SetOpacity(opacity float32) {
	if opacity != opacity || opacity < 0 || opacity > 1 {
		slog.Warn("SetOpacity: ignoring invalid opacity", "opacity", opacity)
		return
	}
	C.plafSetWindowOpacity(w.plafWnd, C.float(opacity))
}

// IsFocused returns true if the window currently has the keyboard focus.
func (w *Window) IsFocused() bool {
	return bool(C.plafIsWindowFocused(w.plafWnd))
}

// RequestAttention function requests user attention to the specified
// window. On platforms where this is not supported, attention is requested to
// the application as a whole.
//
// Once the user has given attention, usually by focusing the window or
// application, the system will end the request automatically.
//
// This function must only be called from the main thread.
func (w *Window) RequestAttention() {
	C.plafRequestWindowAttention(w.plafWnd)
}

// Focus brings the specified window to front and sets input focus.
// The window should already be visible and not minimized.
//
// By default, both windowed and full screen mode windows are focused when initially created.
// Set the plaf.Focused to disable this behavior.
//
// Do not use this function to steal focus from other applications unless you are certain that
// is what the user wants. Focus stealing can be extremely disruptive.
func (w *Window) Focus() {
	C.plafFocusWindow(w.plafWnd)
}

// HideCursor hides the cursor.
func (w *Window) HideCursor() {
	C.plafHideCursor(w.plafWnd)
}

// ShowCursor shows the cursor.
func (w *Window) ShowCursor() {
	C.plafShowCursor(w.plafWnd)
}

// IsMinimized returns true if the window is currently minimized.
func (w *Window) IsMinimized() bool {
	return bool(C.plafIsWindowMinimized(w.plafWnd))
}

// Minimize the window, if it was previously restored. If it is a full screen window, the original monitor resolution is
// restored until the window is restored. If the window is already minimized, this function does nothing.
//
// This function may only be called from the main thread.
func (w *Window) Minimize() {
	C.plafMinimizeWindow(w.plafWnd)
}

// IsMaximized returns true if the window is currently maximized.
func (w *Window) IsMaximized() bool {
	return bool(C.plafIsWindowMaximized(w.plafWnd))
}

// Maximize maximizes the specified window if it was previously not maximized.
// If the window is already maximized, this function does nothing.
//
// If the specified window is a full screen window, this function does nothing.
func (w *Window) Maximize() {
	C.plafMaximizeWindow(w.plafWnd)
}

// Restore restores the window, if it was previously minimized. If it
// is a full screen window, the resolution chosen for the window is restored on
// the selected monitor. If the window is already restored, this function does
// nothing.
//
// This function may only be called from the main thread.
func (w *Window) Restore() {
	C.plafRestoreWindow(w.plafWnd)
}

// Show makes the window visible, if it was previously hidden. If the window is
// already visible or is in full screen mode, this function does nothing.
//
// This function may only be called from the main thread.
func (w *Window) Show() {
	C.plafShowWindow(w.plafWnd)
}

// Hide hides the window, if it was previously visible. If the window is already
// hidden or is in full screen mode, this function does nothing.
//
// This function may only be called from the main thread.
func (w *Window) Hide() {
	C.plafHideWindow(w.plafWnd)
}

// IsVisible returns true if the window is currently being shown.
func (w *Window) IsVisible() bool {
	return bool(C.plafWindowVisible(w.plafWnd))
}

// IsTransparent returns true if the window was created with a transparent backing buffer.
func (w *Window) IsTransparent() bool {
	return bool(C.plafIsFramebufferTransparent(w.plafWnd))
}

// GetMonitor returns the handle of the monitor that the window is in
// fullscreen on.
//
// Returns nil if the window is in windowed mode.
func (w *Window) GetMonitor() *Monitor {
	if w.plafWnd.monitor == nil {
		return nil
	}
	return &Monitor{data: w.plafWnd.monitor}
}

// SetMonitor sets the monitor that the window uses for full screen mode or,
// if the monitor is NULL, makes it windowed mode.
//
// When setting a monitor, this function updates the width, height and refresh
// rate of the desired video mode and switches to the video mode closest to it.
// The window position is ignored when setting a monitor.
//
// When the monitor is NULL, the position, width and height are used to place
// the window client area. The refresh rate is ignored when no monitor is specified.
// If you only wish to update the resolution of a full screen window or the size of
// a windowed mode window, see window.SetSize.
//
// When a window transitions from full screen to windowed mode, this function
// restores any previous window settings such as whether it is decorated, floating,
// resizable, has size or aspect ratio limits, etc..
func (w *Window) SetMonitor(monitor *Monitor, xpos, ypos, width, height, refreshRate int) {
	if width <= 0 || height <= 0 || (refreshRate < 0 && refreshRate != C.DONT_CARE) {
		slog.Warn("SetMonitor: invalid size", "width", width, "height", height)
		return
	}
	if refreshRate < 0 && refreshRate != C.DONT_CARE {
		slog.Warn("SetMonitor: invalid refreshRate", "refreshRate", refreshRate)
		return
	}
	var m *C.plafMonitor
	if monitor == nil {
		m = nil
	} else {
		m = monitor.data
	}
	C.plafSetWindowMonitor(w.plafWnd, m, C.int(xpos), C.int(ypos), C.int(width), C.int(height), C.int(refreshRate))
}

// NativeWindow returns the underlying native window.
func (w *Window) NativeWindow() unsafe.Pointer {
	return C.plafGetNativeWindow(w.plafWnd)
}

// PollEvents processes only those events that have already been received and
// then returns immediately. Processing events will cause the window and input
// callbacks associated with those events to be called.
//
// This function may not be called from a callback.
//
// This function may only be called from the main thread.
func PollEvents() {
	C.plafPollEvents()
}

// WaitEvents puts the calling thread to sleep until at least one event has been
// received. Once one or more events have been recevied, it behaves as if
// PollEvents was called, i.e. the events are processed and the function then
// returns immediately. Processing events will cause the window and input
// callbacks associated with those events to be called.
//
// Since not all events are associated with callbacks, this function may return
// without a callback having been called even if you are monitoring all
// callbacks.
//
// This function may not be called from a callback.
//
// This function may only be called from the main thread.
func WaitEvents() {
	C.plafWaitEvents()
}

// WaitEventsTimeout puts the calling thread to sleep until at least one event is available in the
// event queue, or until the specified timeout is reached. If one or more events are available,
// it behaves exactly like PollEvents, i.e. the events in the queue are processed and the function
// then returns immediately. Processing events will cause the window and input callbacks associated
// with those events to be called.
//
// The timeout value must be a positive finite number.
//
// Since not all events are associated with callbacks, this function may return without a callback
// having been called even if you are monitoring all callbacks.
//
// On some platforms, a window move, resize or menu operation will cause event processing to block.
// This is due to how event processing is designed on those platforms. You can use the window
// refresh callback to redraw the contents of your window when necessary during such operations.
//
// On some platforms, certain callbacks may be called outside of a call to one of the event
// processing functions.
//
// If no windows exist, this function returns immediately. For synchronization of threads in
// applications that do not create windows, use native Go primitives.
func WaitEventsTimeout(timeout float64) {
	if timeout != timeout || timeout < 0 || timeout > math.MaxFloat64 {
		slog.Warn("WaitEventsTimeout: invalid timeout", "timeout", timeout)
		WaitEvents()
	} else {
		C.plafWaitEventsTimeout(C.double(timeout))
	}
}

// PostEmptyEvent posts an empty event from the current thread to the main
// thread event queue, causing WaitEvents to return.
//
// If no windows exist, this function returns immediately. For synchronization of threads in
// applications that do not create windows, use native Go primitives.
//
// This function may be called from secondary threads.
func PostEmptyEvent() {
	C.plafPostEmptyEvent()
}
