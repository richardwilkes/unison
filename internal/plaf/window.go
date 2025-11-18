package plaf

/*
#include "platform.h"

void goFramebufferSizeCallback(plafWindow *window, int width, int height);
void goWindowCloseCallback(plafWindow *window);
void goWindowContentScaleCallback(plafWindow *window, float x, float y);
void goWindowFocusCallback(plafWindow *window, int focused);
void goWindowIconifyCallback(plafWindow *window, int iconified);
void goWindowMaximizeCallback(plafWindow *window, int maximized);
void goWindowPosCallback(plafWindow *window, int xpos, int ypos);
void goWindowRefreshCallback(plafWindow *window);
void goWindowSizeCallback(plafWindow *window, int width, int height);
*/
import "C"

import (
	"image"
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
	w.m[wnd.data] = wnd
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

// Hint corresponds to hints that can be set before creating a window.
//
// Hint also corresponds to the attributes of the window that can be get after
// its creation.
type Hint int

// Window related hints/attributes.
const (
	Iconified              Hint = C.WINDOW_ATTR_ICONIFIED                    // Specifies whether the window will be minimized.
	Maximized              Hint = C.WINDOW_ATTR_HINT_MAXIMIZED               // Specifies whether the window is maximized.
	Visible                Hint = C.WINDOW_ATTR_VISIBLE                      // Specifies whether the window will be initially visible.
	Hovered                Hint = C.WINDOW_ATTR_HOVERED                      // Specifies whether the cursor is currently directly over the content area of the window, with no other windows between. See Cursor enter/leave events for details.
	Resizable              Hint = C.WINDOW_ATTR_HINT_RESIZABLE               // Specifies whether the window will be resizable by the user.
	Decorated              Hint = C.WINDOW_ATTR_HINT_DECORATED               // Specifies whether the window will have window decorations such as a border, a close widget, etc.
	Floating               Hint = C.WINDOW_ATTR_HINT_FLOATING                // Specifies whether the window will be always-on-top.
	TransparentFramebuffer Hint = C.WINDOW_ATTR_HINT_TRANSPARENT_FRAMEBUFFER // Specifies whether the framebuffer should be transparent.
	ScaleToMonitor         Hint = C.WINDOW_HINT_SCALE_TO_MONITOR             // Specified whether the window content area should be resized based on the monitor content scale of any monitor it is placed on. This includes the initial placement when the window is created.
)

// Context related hints.
const (
	ContextRobustness       Hint = C.WINDOW_ATTR_HINT_CONTEXT_ROBUSTNESS       // Specifies the robustness strategy to be used by the context.
	ContextReleaseBehavior  Hint = C.WINDOW_ATTR_HINT_CONTEXT_RELEASE_BEHAVIOR // Specifies the release behavior to be used by the context.
	OpenGLForwardCompatible Hint = C.WINDOW_ATTR_HINT_OPENGL_FORWARD_COMPAT    // Specifies whether the OpenGL context should be forward-compatible. Hard constraint.
	OpenGLDebugContext      Hint = C.WINDOW_ATTR_HINT_CONTEXT_DEBUG            // Specifies whether to create a debug OpenGL context, which may have additional error and performance issue reporting functionality. If OpenGL ES is requested, this hint is ignored.
	OpenGLProfile           Hint = C.WINDOW_ATTR_HINT_OPENGL_PROFILE           // Specifies which OpenGL profile to create the context for. Hard constraint.
)

// Framebuffer related hints.
const (
	ContextRevision  Hint = C.WINDOW_ATTR_CONTEXT_REVISION
	RedBits          Hint = C.WINDOW_HINT_RED_BITS           // Specifies the desired bit depth of the default framebuffer.
	GreenBits        Hint = C.WINDOW_HINT_GREEN_BITS         // Specifies the desired bit depth of the default framebuffer.
	BlueBits         Hint = C.WINDOW_HINT_BLUE_BITS          // Specifies the desired bit depth of the default framebuffer.
	AlphaBits        Hint = C.WINDOW_HINT_ALPHA_BITS         // Specifies the desired bit depth of the default framebuffer.
	DepthBits        Hint = C.WINDOW_HINT_DEPTH_BITS         // Specifies the desired bit depth of the default framebuffer.
	StencilBits      Hint = C.WINDOW_HINT_STENCIL_BITS       // Specifies the desired bit depth of the default framebuffer.
	AccumRedBits     Hint = C.WINDOW_HINT_ACCUM_RED_BITS     // Specifies the desired bit depth of the accumulation buffer.
	AccumGreenBits   Hint = C.WINDOW_HINT_ACCUM_GREEN_BITS   // Specifies the desired bit depth of the accumulation buffer.
	AccumBlueBits    Hint = C.WINDOW_HINT_ACCUM_BLUE_BITS    // Specifies the desired bit depth of the accumulation buffer.
	AccumAlphaBits   Hint = C.WINDOW_HINT_ACCUM_ALPHA_BITS   // Specifies the desired bit depth of the accumulation buffer.
	AuxBuffers       Hint = C.WINDOW_HINT_AUX_BUFFERS        // Specifies the desired number of auxiliary buffers.
	Samples          Hint = C.WINDOW_HINT_SAMPLES            // Specifies the desired number of samples to use for multisampling. Zero disables multisampling.
	SRGBCapable      Hint = C.WINDOW_HINT_SRGB_CAPABLE       // Specifies whether the framebuffer should be sRGB capable.
	RefreshRate      Hint = C.WINDOW_HINT_REFRESH_RATE       // Specifies the desired refresh rate for full screen windows. If set to zero, the highest available refresh rate will be used. This hint is ignored for windowed mode windows.
	DoubleBuffer     Hint = C.WINDOW_ATTR_HINT_DOUBLE_BUFFER // Specifies whether the framebuffer should be double buffered. You nearly always want to use double buffering. This is a hard constraint.
	ScaleFramebuffer Hint = C.WINDOW_HINT_SCALE_FRAMEBUFFER  // Specifies whether to use full resolution framebuffers on Retina displays.
)

// Values for the ContextRobustness hint.
const (
	ContextRobustnessNone                int = C.CONTEXT_ROBUSTNESS_NONE
	ContextRobustnessNoResetNotification int = C.CONTEXT_ROBUSTNESS_NO_RESET_NOTIFICATION
	ContextRobustnessLoseContextOnReset  int = C.CONTEXT_ROBUSTNESS_LOSE_CONTEXT_ON_RESET
)

// Values for the OpenGLProfile hint.
const (
	OpenGLProfileAny    int = C.OPENGL_PROFILE_ANY
	OpenGLProfileCore   int = C.OPENGL_PROFILE_CORE
	OpenGLProfileCompat int = C.OPENGL_PROFILE_COMPAT
)

// Values for ContextReleaseBehavior hint.
const (
	ReleaseBehaviorAny   int = C.RELEASE_BEHAVIOR_ANY
	ReleaseBehaviorFlush int = C.RELEASE_BEHAVIOR_FLUSH
	ReleaseBehaviorNone  int = C.RELEASE_BEHAVIOR_NONE
)

// Other values.
const (
	True     int = 1 // GL_TRUE
	False    int = 0 // GL_FALSE
	DontCare int = C.DONT_CARE
)

// Window represents a window.
type Window struct {
	data *C.plafWindow

	// Window.
	fPosHolder             func(w *Window, xpos, ypos int)
	fSizeHolder            func(w *Window, width, height int)
	fFramebufferSizeHolder func(w *Window, width, height int)
	fCloseHolder           func(w *Window)
	fMaximizeHolder        func(w *Window, maximized bool)
	fContentScaleHolder    func(w *Window, x, y float32)
	fRefreshHolder         func(w *Window)
	fFocusHolder           func(w *Window, focused bool)
	fIconifyHolder         func(w *Window, iconified bool)

	// Input.
	fMouseButtonHolder func(w *Window, button MouseButton, action Action, mod ModifierKey)
	fCursorPosHolder   func(w *Window, xpos, ypos float64)
	fCursorEnterHolder func(w *Window, entered bool)
	fScrollHolder      func(w *Window, xoff, yoff float64)
	fKeyHolder         func(w *Window, key Key, scancode int, action Action, mods ModifierKey)
	fCharHolder        func(w *Window, char rune)
	fCharModsHolder    func(w *Window, char rune, mods ModifierKey)
	fDropHolder        func(w *Window, names []string)
}

// Handle returns a *C.plafWindow reference (i.e. the GLFW window itself).
// This can be used for passing the GLFW window handle to external libraries.
func (w *Window) Handle() unsafe.Pointer {
	return unsafe.Pointer(w.data)
}

// GoWindow creates a Window from a *C.plafWindow reference.
// Used when an external C library is calling your Go handlers.
func GoWindow(window unsafe.Pointer) *Window {
	return &Window{data: (*C.plafWindow)(window)}
}

// DefaultWindowHints resets all window hints to their default values.
//
// This function may only be called from the main thread.
func DefaultWindowHints() {
	C.glfwDefaultWindowHints()
	panicError()
}

// WindowHint sets hints for the next call to CreateWindow. The hints,
// once set, retain their values until changed by a call to WindowHint or
// DefaultWindowHints, or until the library is terminated with Terminate.
//
// This function may only be called from the main thread.
func WindowHint(target Hint, hint int) {
	C.glfwWindowHint(C.int(target), C.int(hint))
	panicError()
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
// Windows: If the executable has an icon resource named GLFW_ICON, it will be
// set as the icon for the window. If no such icon is present, the IDI_WINLOGO
// icon will be used instead.
//
// Mac OS X: The GLFW window has no icon, as it is not a document window, but the
// dock icon will be the same as the application bundle's icon.
//
// This function may only be called from the main thread.
func CreateWindow(width, height int, title string, monitor *Monitor, share *Window) (*Window, error) {
	var (
		m *C.plafMonitor
		s *C.plafWindow
	)

	t := C.CString(title)
	defer C.free(unsafe.Pointer(t))

	if monitor != nil {
		m = monitor.data
	}

	if share != nil {
		s = share.data
	}

	w := C.glfwCreateWindow(C.int(width), C.int(height), t, m, s)
	if w == nil {
		return nil, acceptError(APIUnavailable, VersionUnavailable)
	}

	wnd := &Window{data: w}
	windows.put(wnd)
	return wnd, nil
}

// Destroy destroys the specified window and its context. On calling this
// function, no further callbacks will be called for that window.
//
// This function may only be called from the main thread.
func (w *Window) Destroy() {
	windows.remove(w.data)
	C.glfwDestroyWindow(w.data)
	panicError()
}

// ShouldClose reports the value of the close flag of the specified window.
func (w *Window) ShouldClose() bool {
	ret := C.glfwWindowShouldClose(w.data)
	panicError()
	return ret != 0
}

// SetShouldClose sets the value of the close flag of the window. This can be
// used to override the user's attempt to close the window, or to signal that it
// should be closed.
func (w *Window) SetShouldClose(value bool) {
	if !value {
		C.glfwSetWindowShouldClose(w.data, C.int(False))
	} else {
		C.glfwSetWindowShouldClose(w.data, C.int(True))
	}
	panicError()
}

// SetTitle sets the window title, encoded as UTF-8, of the window.
//
// This function may only be called from the main thread.
func (w *Window) SetTitle(title string) {
	t := C.CString(title)
	defer C.free(unsafe.Pointer(t))
	C.glfwSetWindowTitle(w.data, t)
	panicError()
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
func (w *Window) SetIcon(images []image.Image) {
	count := len(images)
	cimages := make([]C.ImageData, count)
	freePixels := make([]func(), count)

	for i, img := range images {
		cimages[i], freePixels[i] = imageToGLFW(img)
	}

	var p *C.ImageData
	if count > 0 {
		p = &cimages[0]
	}
	C.glfwSetWindowIcon(w.data, C.int(count), p)

	for _, v := range freePixels {
		v()
	}

	acceptError(invalidValue, FeatureUnavailable)
}

// GetPos returns the position, in screen coordinates, of the upper-left
// corner of the client area of the window.
func (w *Window) GetPos() (x, y int) {
	var xpos, ypos C.int
	C.glfwGetWindowPos(w.data, &xpos, &ypos)
	panicError()
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
	C.glfwSetWindowPos(w.data, C.int(xpos), C.int(ypos))
	panicError()
}

// GetSize returns the size, in screen coordinates, of the client area of the
// specified window.
func (w *Window) GetSize() (width, height int) {
	var wi, h C.int
	C.glfwGetWindowSize(w.data, &wi, &h)
	panicError()
	return int(wi), int(h)
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
	C.glfwSetWindowSize(w.data, C.int(width), C.int(height))
	panicError()
}

// SetSizeLimits sets the size limits of the client area of the specified window.
// If the window is full screen or not resizable, this function does nothing.
//
// The size limits are applied immediately and may cause the window to be resized.
func (w *Window) SetSizeLimits(minw, minh, maxw, maxh int) {
	C.glfwSetWindowSizeLimits(w.data, C.int(minw), C.int(minh), C.int(maxw), C.int(maxh))
	panicError()
}

// SetAspectRatio sets the required aspect ratio of the client area of the specified window.
// If the window is full screen or not resizable, this function does nothing.
//
// The aspect ratio is specified as a numerator and a denominator and both values must be greater
// than zero. For example, the common 16:9 aspect ratio is specified as 16 and 9, respectively.
//
// If the numerator and denominator is set to glfw.DontCare then the aspect ratio limit is disabled.
//
// The aspect ratio is applied immediately and may cause the window to be resized.
func (w *Window) SetAspectRatio(numer, denom int) {
	C.glfwSetWindowAspectRatio(w.data, C.int(numer), C.int(denom))
	panicError()
}

// GetFramebufferSize retrieves the size, in pixels, of the framebuffer of the
// specified window.
func (w *Window) GetFramebufferSize() (width, height int) {
	var wi, h C.int
	C.glfwGetFramebufferSize(w.data, &wi, &h)
	panicError()
	return int(wi), int(h)
}

// GetFrameSize retrieves the size, in screen coordinates, of each edge of the frame
// of the specified window. This size includes the title bar, if the window has one.
// The size of the frame may vary depending on the window-related hints used to create it.
//
// Because this function retrieves the size of each window frame edge and not the offset
// along a particular coordinate axis, the retrieved values will always be zero or positive.
func (w *Window) GetFrameSize() (left, top, right, bottom int) {
	var l, t, r, b C.int
	C.glfwGetWindowFrameSize(w.data, &l, &t, &r, &b)
	panicError()
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
	C.glfwGetWindowContentScale(w.data, &cX, &cY)
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
	return float32(C.glfwGetWindowOpacity(w.data))
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
	C.glfwSetWindowOpacity(w.data, C.float(opacity))
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
	C.glfwRequestWindowAttention(w.data)
}

// Focus brings the specified window to front and sets input focus.
// The window should already be visible and not iconified.
//
// By default, both windowed and full screen mode windows are focused when initially created.
// Set the glfw.Focused to disable this behavior.
//
// Do not use this function to steal focus from other applications unless you are certain that
// is what the user wants. Focus stealing can be extremely disruptive.
func (w *Window) Focus() {
	C.glfwFocusWindow(w.data)
}

// Iconify iconifies/minimizes the window, if it was previously restored. If it
// is a full screen window, the original monitor resolution is restored until the
// window is restored. If the window is already iconified, this function does
// nothing.
//
// This function may only be called from the main thread.
func (w *Window) Iconify() {
	C.glfwIconifyWindow(w.data)
}

// Maximize maximizes the specified window if it was previously not maximized.
// If the window is already maximized, this function does nothing.
//
// If the specified window is a full screen window, this function does nothing.
func (w *Window) Maximize() {
	C.glfwMaximizeWindow(w.data)
}

// Restore restores the window, if it was previously iconified/minimized. If it
// is a full screen window, the resolution chosen for the window is restored on
// the selected monitor. If the window is already restored, this function does
// nothing.
//
// This function may only be called from the main thread.
func (w *Window) Restore() {
	C.glfwRestoreWindow(w.data)
}

// Show makes the window visible, if it was previously hidden. If the window is
// already visible or is in full screen mode, this function does nothing.
//
// This function may only be called from the main thread.
func (w *Window) Show() {
	C.glfwShowWindow(w.data)
	panicError()
}

// Hide hides the window, if it was previously visible. If the window is already
// hidden or is in full screen mode, this function does nothing.
//
// This function may only be called from the main thread.
func (w *Window) Hide() {
	C.glfwHideWindow(w.data)
	panicError()
}

// GetMonitor returns the handle of the monitor that the window is in
// fullscreen on.
//
// Returns nil if the window is in windowed mode.
func (w *Window) GetMonitor() *Monitor {
	m := C.glfwGetWindowMonitor(w.data)
	panicError()
	if m == nil {
		return nil
	}
	return &Monitor{m}
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
	var m *C.plafMonitor
	if monitor == nil {
		m = nil
	} else {
		m = monitor.data
	}
	C.glfwSetWindowMonitor(w.data, m, C.int(xpos), C.int(ypos), C.int(width), C.int(height), C.int(refreshRate))
	panicError()
}

// GetAttrib returns an attribute of the window. There are many attributes,
// some related to the window and others to its context.
func (w *Window) GetAttrib(attrib Hint) int {
	ret := int(C.glfwGetWindowAttrib(w.data, C.int(attrib)))
	panicError()
	return ret
}

// SetAttrib function sets the value of an attribute of the specified window.
//
// The supported attributes are Decorated, Resizeable, and Floating.
//
// Some of these attributes are ignored for full screen windows. The new value
// will take effect if the window is later made windowed.
//
// Some of these attributes are ignored for windowed mode windows. The new value
// will take effect if the window is later made full screen.
//
// This function may only be called from the main thread.
func (w *Window) SetAttrib(attrib Hint, value int) {
	C.glfwSetWindowAttrib(w.data, C.int(attrib), C.int(value))
}

// PosCallback is the window position callback.
type PosCallback func(w *Window, xpos, ypos int)

// SetPosCallback sets the position callback of the window, which is called
// when the window is moved. The callback is provided with the screen position
// of the upper-left corner of the client area of the window.
func (w *Window) SetPosCallback(cbfun PosCallback) (previous PosCallback) {
	previous = w.fPosHolder
	w.fPosHolder = cbfun
	var callback C.windowPosFunc
	if cbfun != nil {
		callback = C.windowPosFunc(C.goWindowPosCallback)
	}
	C.glfwSetWindowPosCallback(w.data, callback)
	panicError()
	return previous
}

// SizeCallback is the window size callback.
type SizeCallback func(w *Window, width, height int)

// SetSizeCallback sets the size callback of the window, which is called when
// the window is resized. The callback is provided with the size, in screen
// coordinates, of the client area of the window.
func (w *Window) SetSizeCallback(cbfun SizeCallback) (previous SizeCallback) {
	previous = w.fSizeHolder
	w.fSizeHolder = cbfun
	var callback C.windowSizeFunc
	if cbfun != nil {
		callback = C.windowSizeFunc(C.goWindowSizeCallback)
	}
	C.glfwSetWindowSizeCallback(w.data, callback)
	panicError()
	return previous
}

// FramebufferSizeCallback is the framebuffer size callback.
type FramebufferSizeCallback func(w *Window, width, height int)

// SetFramebufferSizeCallback sets the framebuffer resize callback of the specified
// window, which is called when the framebuffer of the specified window is resized.
func (w *Window) SetFramebufferSizeCallback(cbfun FramebufferSizeCallback) (previous FramebufferSizeCallback) {
	previous = w.fFramebufferSizeHolder
	w.fFramebufferSizeHolder = cbfun
	var callback C.frameBufferSizeFunc
	if cbfun != nil {
		callback = C.frameBufferSizeFunc(C.goFramebufferSizeCallback)
	}
	C.glfwSetFramebufferSizeCallback(w.data, callback)
	panicError()
	return previous
}

// CloseCallback is the window close callback.
type CloseCallback func(w *Window)

// SetCloseCallback sets the close callback of the window, which is called when
// the user attempts to close the window, for example by clicking the close
// widget in the title bar.
//
// The close flag is set before this callback is called, but you can modify it at
// any time with SetShouldClose.
//
// Mac OS X: Selecting Quit from the application menu will trigger the close
// callback for all windows.
func (w *Window) SetCloseCallback(cbfun CloseCallback) (previous CloseCallback) {
	previous = w.fCloseHolder
	w.fCloseHolder = cbfun
	var callback C.windowCloseFunc
	if cbfun != nil {
		callback = C.windowCloseFunc(C.goWindowCloseCallback)
	}
	C.glfwSetWindowCloseCallback(w.data, callback)
	panicError()
	return previous
}

// MaximizeCallback is the function signature for window maximize callback
// functions.
type MaximizeCallback func(w *Window, maximized bool)

// SetMaximizeCallback sets the maximization callback of the specified window,
// which is called when the window is maximized or restored.
//
// This function must only be called from the main thread.
func (w *Window) SetMaximizeCallback(cbfun MaximizeCallback) MaximizeCallback {
	previous := w.fMaximizeHolder
	w.fMaximizeHolder = cbfun
	var callback C.windowMaximizeFunc
	if cbfun != nil {
		callback = C.windowMaximizeFunc(C.goWindowMaximizeCallback)
	}
	C.glfwSetWindowMaximizeCallback(w.data, callback)
	panicError()
	return previous
}

// ContentScaleCallback is the function signature for window content scale
// callback functions.
type ContentScaleCallback func(w *Window, x, y float32)

// SetContentScaleCallback function sets the window content scale callback of
// the specified window, which is called when the content scale of the specified
// window changes.
//
// This function must only be called from the main thread.
func (w *Window) SetContentScaleCallback(cbfun ContentScaleCallback) ContentScaleCallback {
	previous := w.fContentScaleHolder
	w.fContentScaleHolder = cbfun
	var callback C.windowContextScaleFunc
	if cbfun != nil {
		callback = C.windowContextScaleFunc(C.goWindowContentScaleCallback)
	}
	C.glfwSetWindowContentScaleCallback(w.data, callback)
	panicError()
	return previous
}

// RefreshCallback is the window refresh callback.
type RefreshCallback func(w *Window)

// SetRefreshCallback sets the refresh callback of the window, which
// is called when the client area of the window needs to be redrawn, for example
// if the window has been exposed after having been covered by another window.
//
// On compositing window systems such as Aero, Compiz or Aqua, where the window
// contents are saved off-screen, this callback may be called only very
// infrequently or never at all.
func (w *Window) SetRefreshCallback(cbfun RefreshCallback) (previous RefreshCallback) {
	previous = w.fRefreshHolder
	w.fRefreshHolder = cbfun
	var callback C.windowRefreshFunc
	if cbfun != nil {
		callback = C.windowRefreshFunc(C.goWindowRefreshCallback)
	}
	C.glfwSetWindowRefreshCallback(w.data, callback)
	panicError()
	return previous
}

// FocusCallback is the window focus callback.
type FocusCallback func(w *Window, focused bool)

// SetFocusCallback sets the focus callback of the window, which is called when
// the window gains or loses focus.
//
// After the focus callback is called for a window that lost focus, synthetic key
// and mouse button release events will be generated for all such that had been
// pressed. For more information, see SetKeyCallback and SetMouseButtonCallback.
func (w *Window) SetFocusCallback(cbfun FocusCallback) (previous FocusCallback) {
	previous = w.fFocusHolder
	w.fFocusHolder = cbfun
	var callback C.windowFocusFunc
	if cbfun != nil {
		callback = C.windowFocusFunc(C.goWindowFocusCallback)
	}
	C.glfwSetWindowFocusCallback(w.data, callback)
	panicError()
	return previous
}

// IconifyCallback is the window iconification callback.
type IconifyCallback func(w *Window, iconified bool)

// SetIconifyCallback sets the iconification callback of the window, which is
// called when the window is iconified or restored.
func (w *Window) SetIconifyCallback(cbfun IconifyCallback) (previous IconifyCallback) {
	previous = w.fIconifyHolder
	w.fIconifyHolder = cbfun
	var callback C.windowIconifyFunc
	if cbfun != nil {
		callback = C.windowIconifyFunc(C.goWindowIconifyCallback)
	}
	C.glfwSetWindowIconifyCallback(w.data, callback)
	panicError()
	return previous
}

// PollEvents processes only those events that have already been received and
// then returns immediately. Processing events will cause the window and input
// callbacks associated with those events to be called.
//
// This function may not be called from a callback.
//
// This function may only be called from the main thread.
func PollEvents() {
	C.glfwPollEvents()
	panicError()
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
	C.glfwWaitEvents()
	panicError()
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
	C.glfwWaitEventsTimeout(C.double(timeout))
	panicError()
}

// PostEmptyEvent posts an empty event from the current thread to the main
// thread event queue, causing WaitEvents to return.
//
// If no windows exist, this function returns immediately. For synchronization of threads in
// applications that do not create windows, use native Go primitives.
//
// This function may be called from secondary threads.
func PostEmptyEvent() {
	C.glfwPostEmptyEvent()
	panicError()
}
