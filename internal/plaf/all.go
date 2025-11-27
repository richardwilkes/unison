package plaf

// NOTE: A single Go file that imports the C package was an intentional choice here, as it dramatically reduces the
//       compile time of the package.

/*
#cgo darwin CFLAGS: -Wno-deprecated-declarations -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa -framework CoreVideo -framework OpenGL

#cgo linux LDFLAGS: -lGL -lX11 -lXrandr -lXxf86vm -lXi -lXcursor -lm -lXinerama -ldl -lrt

#cgo windows LDFLAGS: -lgdi32 -lopengl32

#include "platform.h"
*/
import "C"

import (
	"image"
	"image/draw"
	"log/slog"
	"math"
	"runtime"
	"sync"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// ---------- Clipboard ----------

// GetClipboardString returns the contents of the system clipboard, if it contains or is convertible to a UTF-8 encoded
// string.
func GetClipboardString() string {
	s := C.plafGetClipboardString()
	if s == nil {
		return ""
	}
	return C.GoString(s)
}

// SetClipboardString sets the system clipboard to the specified UTF-8 encoded string.
func SetClipboardString(str string) {
	s := C.CString(str)
	defer C.free(unsafe.Pointer(s))
	C.plafSetClipboardString(s)
}

// ---------- Context ----------

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

// ---------- Cursor ----------

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

// ---------- Input ----------

// Key corresponds to a keyboard key.
type Key int

// These key codes are inspired by the USB HID Usage Tables v1.12 (p. 53-60),
// but re-arranged to map to 7-bit ASCII for printable keys (function keys are
// put in the 256+ range).
const (
	KeyUnknown      Key = C.KEY_UNKNOWN
	KeySpace        Key = C.KEY_SPACE
	KeyApostrophe   Key = C.KEY_APOSTROPHE
	KeyComma        Key = C.KEY_COMMA
	KeyMinus        Key = C.KEY_MINUS
	KeyPeriod       Key = C.KEY_PERIOD
	KeySlash        Key = C.KEY_SLASH
	Key0            Key = C.KEY_0
	Key1            Key = C.KEY_1
	Key2            Key = C.KEY_2
	Key3            Key = C.KEY_3
	Key4            Key = C.KEY_4
	Key5            Key = C.KEY_5
	Key6            Key = C.KEY_6
	Key7            Key = C.KEY_7
	Key8            Key = C.KEY_8
	Key9            Key = C.KEY_9
	KeySemicolon    Key = C.KEY_SEMICOLON
	KeyEqual        Key = C.KEY_EQUAL
	KeyA            Key = C.KEY_A
	KeyB            Key = C.KEY_B
	KeyC            Key = C.KEY_C
	KeyD            Key = C.KEY_D
	KeyE            Key = C.KEY_E
	KeyF            Key = C.KEY_F
	KeyG            Key = C.KEY_G
	KeyH            Key = C.KEY_H
	KeyI            Key = C.KEY_I
	KeyJ            Key = C.KEY_J
	KeyK            Key = C.KEY_K
	KeyL            Key = C.KEY_L
	KeyM            Key = C.KEY_M
	KeyN            Key = C.KEY_N
	KeyO            Key = C.KEY_O
	KeyP            Key = C.KEY_P
	KeyQ            Key = C.KEY_Q
	KeyR            Key = C.KEY_R
	KeyS            Key = C.KEY_S
	KeyT            Key = C.KEY_T
	KeyU            Key = C.KEY_U
	KeyV            Key = C.KEY_V
	KeyW            Key = C.KEY_W
	KeyX            Key = C.KEY_X
	KeyY            Key = C.KEY_Y
	KeyZ            Key = C.KEY_Z
	KeyLeftBracket  Key = C.KEY_LEFT_BRACKET
	KeyBackslash    Key = C.KEY_BACKSLASH
	KeyRightBracket Key = C.KEY_RIGHT_BRACKET
	KeyGraveAccent  Key = C.KEY_GRAVE_ACCENT
	KeyWorld1       Key = C.KEY_WORLD_1
	KeyWorld2       Key = C.KEY_WORLD_2
	KeyEscape       Key = C.KEY_ESCAPE
	KeyEnter        Key = C.KEY_ENTER
	KeyTab          Key = C.KEY_TAB
	KeyBackspace    Key = C.KEY_BACKSPACE
	KeyInsert       Key = C.KEY_INSERT
	KeyDelete       Key = C.KEY_DELETE
	KeyRight        Key = C.KEY_RIGHT
	KeyLeft         Key = C.KEY_LEFT
	KeyDown         Key = C.KEY_DOWN
	KeyUp           Key = C.KEY_UP
	KeyPageUp       Key = C.KEY_PAGE_UP
	KeyPageDown     Key = C.KEY_PAGE_DOWN
	KeyHome         Key = C.KEY_HOME
	KeyEnd          Key = C.KEY_END
	KeyCapsLock     Key = C.KEY_CAPS_LOCK
	KeyScrollLock   Key = C.KEY_SCROLL_LOCK
	KeyNumLock      Key = C.KEY_NUM_LOCK
	KeyPrintScreen  Key = C.KEY_PRINT_SCREEN
	KeyPause        Key = C.KEY_PAUSE
	KeyF1           Key = C.KEY_F1
	KeyF2           Key = C.KEY_F2
	KeyF3           Key = C.KEY_F3
	KeyF4           Key = C.KEY_F4
	KeyF5           Key = C.KEY_F5
	KeyF6           Key = C.KEY_F6
	KeyF7           Key = C.KEY_F7
	KeyF8           Key = C.KEY_F8
	KeyF9           Key = C.KEY_F9
	KeyF10          Key = C.KEY_F10
	KeyF11          Key = C.KEY_F11
	KeyF12          Key = C.KEY_F12
	KeyF13          Key = C.KEY_F13
	KeyF14          Key = C.KEY_F14
	KeyF15          Key = C.KEY_F15
	KeyF16          Key = C.KEY_F16
	KeyF17          Key = C.KEY_F17
	KeyF18          Key = C.KEY_F18
	KeyF19          Key = C.KEY_F19
	KeyF20          Key = C.KEY_F20
	KeyF21          Key = C.KEY_F21
	KeyF22          Key = C.KEY_F22
	KeyF23          Key = C.KEY_F23
	KeyF24          Key = C.KEY_F24
	KeyF25          Key = C.KEY_F25
	KeyKP0          Key = C.KEY_KP_0
	KeyKP1          Key = C.KEY_KP_1
	KeyKP2          Key = C.KEY_KP_2
	KeyKP3          Key = C.KEY_KP_3
	KeyKP4          Key = C.KEY_KP_4
	KeyKP5          Key = C.KEY_KP_5
	KeyKP6          Key = C.KEY_KP_6
	KeyKP7          Key = C.KEY_KP_7
	KeyKP8          Key = C.KEY_KP_8
	KeyKP9          Key = C.KEY_KP_9
	KeyKPDecimal    Key = C.KEY_KP_DECIMAL
	KeyKPDivide     Key = C.KEY_KP_DIVIDE
	KeyKPMultiply   Key = C.KEY_KP_MULTIPLY
	KeyKPSubtract   Key = C.KEY_KP_SUBTRACT
	KeyKPAdd        Key = C.KEY_KP_ADD
	KeyKPEnter      Key = C.KEY_KP_ENTER
	KeyKPEqual      Key = C.KEY_KP_EQUAL
	KeyLeftShift    Key = C.KEY_LEFT_SHIFT
	KeyLeftControl  Key = C.KEY_LEFT_CONTROL
	KeyLeftAlt      Key = C.KEY_LEFT_ALT
	KeyLeftSuper    Key = C.KEY_LEFT_SUPER
	KeyRightShift   Key = C.KEY_RIGHT_SHIFT
	KeyRightControl Key = C.KEY_RIGHT_CONTROL
	KeyRightAlt     Key = C.KEY_RIGHT_ALT
	KeyRightSuper   Key = C.KEY_RIGHT_SUPER
	KeyMenu         Key = C.KEY_MENU
	KeyLast         Key = C.KEY_LAST
)

// ModifierKey corresponds to a modifier key.
type ModifierKey int

// Modifier keys.
const (
	ModShift    ModifierKey = C.KEYMOD_SHIFT
	ModControl  ModifierKey = C.KEYMOD_CONTROL
	ModAlt      ModifierKey = C.KEYMOD_ALT
	ModSuper    ModifierKey = C.KEYMOD_SUPER
	ModCapsLock ModifierKey = C.KEYMOD_CAPS_LOCK
	ModNumLock  ModifierKey = C.KEYMOD_NUM_LOCK
)

// MouseButton corresponds to a mouse button.
type MouseButton int

// Mouse buttons.
const (
	MouseButton1      MouseButton = C.MOUSE_BUTTON_1
	MouseButton2      MouseButton = C.MOUSE_BUTTON_2
	MouseButton3      MouseButton = C.MOUSE_BUTTON_3
	MouseButton4      MouseButton = C.MOUSE_BUTTON_4
	MouseButton5      MouseButton = C.MOUSE_BUTTON_5
	MouseButton6      MouseButton = C.MOUSE_BUTTON_6
	MouseButton7      MouseButton = C.MOUSE_BUTTON_7
	MouseButton8      MouseButton = C.MOUSE_BUTTON_8
	MouseButtonLast   MouseButton = C.MOUSE_BUTTON_LAST
	MouseButtonLeft   MouseButton = C.MOUSE_BUTTON_LEFT
	MouseButtonRight  MouseButton = C.MOUSE_BUTTON_RIGHT
	MouseButtonMiddle MouseButton = C.MOUSE_BUTTON_MIDDLE
)

// Action corresponds to a key or button action.
type Action int

// Action types.
const (
	Release Action = C.INPUT_RELEASE // The key or button was released.
	Press   Action = C.INPUT_PRESS   // The key or button was pressed.
	Repeat  Action = C.INPUT_REPEAT  // The key was held down until it repeated.
)

// GetKeyScancode function returns the platform-specific scancode of the
// specified key.
//
// If the key is KeyUnknown or does not exist on the keyboard this method will
// return -1.
func GetKeyScancode(key Key) int {
	return int(C.plafGetKeyScancode(C.int(key)))
}

// ---------- Monitor ----------

// Monitor represents a monitor.
type Monitor struct {
	data *C.plafMonitor
}

// GammaRamp describes the gamma ramp for a monitor.
type GammaRamp struct {
	Red   []uint16
	Green []uint16
	Blue  []uint16
}

// VidMode describes a single video mode.
type VidMode struct {
	Width       int // The width, in screen coordinates, of the video mode.
	Height      int // The height, in screen coordinates, of the video mode.
	RedBits     int // The bit depth of the red channel of the video mode.
	GreenBits   int // The bit depth of the green channel of the video mode.
	BlueBits    int // The bit depth of the blue channel of the video mode.
	RefreshRate int // The refresh rate, in Hz, of the video mode.
}

// MonitorCallback is called when a monitor has been connected or disconnected.
var MonitorCallback func(monitor *Monitor, connected bool)

// GetMonitors returns a slice of handles for all currently connected monitors.
func GetMonitors() []*Monitor {
	count := int(C._plaf.monitorCount)
	if count == 0 {
		return nil
	}
	m := make([]*Monitor, count)
	list := unsafe.Slice(C._plaf.monitors, count)
	for i := range count {
		m[i] = &Monitor{data: list[i]}
	}
	return m
}

// GetPrimaryMonitor returns the primary monitor. This is usually the monitor where elements like the Windows task bar
// or the OS X menu bar is located.
func GetPrimaryMonitor() *Monitor {
	if C._plaf.monitorCount == 0 {
		return nil
	}
	return &Monitor{data: *C._plaf.monitors}
}

// GetPos returns the position, in screen coordinates, of the upper-left corner of the monitor.
func (m *Monitor) GetPos() (x, y int) {
	var cx, cy C.int
	C.plafGetMonitorPos(m.data, &cx, &cy)
	return int(cx), int(cy)
}

// GetWorkarea returns the position, in screen coordinates, of the upper-left corner of the work area of the specified
// monitor along with the work area size in screen coordinates. The work area is defined as the area of the monitor not
// occluded by the operating system task bar where present. If no task bar exists then the work area is the monitor
// resolution in screen coordinates.
func (m *Monitor) GetWorkarea() (x, y, width, height int) {
	var cX, cY, cWidth, cHeight C.int
	C.plafGetMonitorWorkarea(m.data, &cX, &cY, &cWidth, &cHeight)
	return int(cX), int(cY), int(cWidth), int(cHeight)
}

// GetContentScale function retrieves the content scale for the specified monitor. The content scale is the ratio
// between the current DPI and the platform's default DPI. If you scale all pixel dimensions by this scale then your
// content should appear at an appropriate size. This is especially important for text and any UI elements.
func (m *Monitor) GetContentScale() (x, y float32) {
	var cX, cY C.float
	C.plafGetMonitorContentScale(m.data, &cX, &cY)
	return float32(cX), float32(cY)
}

// GetPhysicalSize returns the size, in millimeters, of the display area of the monitor.
//
// Note: Some operating systems do not provide accurate information, either because the monitor's EDID data is
// incorrect, or because the driver does not report it accurately.
func (m *Monitor) GetPhysicalSize() (width, height int) {
	return int(m.data.widthMM), int(m.data.heightMM)
}

// GetName returns a human-readable name of the monitor, encoded as UTF-8.
func (m *Monitor) GetName() string {
	if m.data.name[0] == 0 {
		return ""
	}
	return C.GoString(&m.data.name[0])
}

// GetVideoModes returns an array of all video modes supported by the monitor. The returned array is sorted in ascending
// order, first by color bit depth (the sum of all channel depths) and then by resolution area (the product of width and
// height).
func (m *Monitor) GetVideoModes() []*VidMode {
	if !C.plafRefreshVideoModes(m.data) || m.data.modes == nil {
		return nil
	}
	count := int(m.data.modeCount)
	result := make([]*VidMode, count)
	list := unsafe.Slice(m.data.modes, count)
	for i := range count {
		result[i] = &VidMode{
			Width:       int(list[i].width),
			Height:      int(list[i].height),
			RedBits:     int(list[i].redBits),
			GreenBits:   int(list[i].greenBits),
			BlueBits:    int(list[i].blueBits),
			RefreshRate: int(list[i].refreshRate),
		}
	}
	return result
}

// GetVideoMode returns the current video mode of the monitor. If you are using a full screen window, the return value
// will therefore depend on whether it is focused.
func (m *Monitor) GetVideoMode() *VidMode {
	t := C.plafGetVideoMode(m.data)
	if t == nil {
		return nil
	}
	return &VidMode{int(t.width), int(t.height), int(t.redBits), int(t.greenBits), int(t.blueBits), int(t.refreshRate)}
}

// SetGamma generates a gamma ramp from the specified exponent and then calls SetGamma with it.
func (m *Monitor) SetGamma(gamma float64) {
	if gamma != gamma || gamma <= 0 || gamma > math.MaxFloat64 {
		slog.Warn("SetGamma: ignoring invalid gamma value", "gamma", gamma)
		return
	}
	ramp := m.GetGammaRamp()
	if ramp == nil {
		slog.Warn("SetGamma: unable to get existing gamma ramp")
		return
	}
	channel := make([]uint16, len(ramp.Red))
	for i := range channel {
		channel[i] = uint16(min(math.Pow(float64(i)/float64(len(channel)-1), 1/gamma), 65535))
	}
	ramp.Red = channel
	ramp.Green = channel
	ramp.Blue = channel
	m.SetGammaRamp(ramp)
}

// GetGammaRamp retrieves the current gamma ramp of the monitor.
func (m *Monitor) GetGammaRamp() *GammaRamp {
	rampC := C.plafGetGammaRamp(m.data)
	if rampC == nil {
		return nil
	}
	length := int(rampC.size)
	var ramp GammaRamp
	ramp.Red = make([]uint16, length)
	ramp.Green = make([]uint16, length)
	ramp.Blue = make([]uint16, length)
	copy(ramp.Red, unsafe.Slice((*uint16)(rampC.red), length))
	copy(ramp.Green, unsafe.Slice((*uint16)(rampC.green), length))
	copy(ramp.Blue, unsafe.Slice((*uint16)(rampC.blue), length))
	return &ramp
}

// SetGammaRamp sets the current gamma ramp for the monitor.
func (m *Monitor) SetGammaRamp(ramp *GammaRamp) {
	length := len(ramp.Red)
	if length == 0 || length != len(ramp.Green) || length != len(ramp.Blue) {
		slog.Warn("SetGammaRamp: ignoring invalid ramp")
		return
	}
	cRamp := &C.plafGammaRamp{
		red:   (*C.ushort)(&ramp.Red[0]),
		green: (*C.ushort)(&ramp.Green[0]),
		blue:  (*C.ushort)(&ramp.Blue[0]),
		size:  C.uint(length),
	}
	C.plafSetGammaRamp(m.data, cRamp)
	runtime.KeepAlive(cRamp)
}

// ---------- Platform ----------

var (
	// OpenFilesCallback is called on macOS (and no other platforms, currently) when a user double-clicks on your app's
	// documents.
	OpenFilesCallback func([]string)
	initTermLock      sync.Mutex
	initialized       bool
	initializing      bool
	terminating       bool
)

// Init must be called exactly once before most things in this package.
func Init() error {
	initTermLock.Lock()
	if initialized {
		initTermLock.Unlock()
		return errs.New("already initialized")
	}
	if initializing {
		initTermLock.Unlock()
		return errs.New("initialization already in progress")
	}
	if terminating {
		initTermLock.Unlock()
		return errs.New("termination in progress")
	}
	initializing = true
	initTermLock.Unlock()
	var success bool
	defer func() {
		initTermLock.Lock()
		initializing = false
		initialized = success
		initTermLock.Unlock()
	}()
	if success = bool(C.plafInit()); !success {
		return errs.New("unable to initialize platform")
	}
	return nil
}

// Terminate should be called before exiting. It will destroy all remaining windows and free any allocated resources.
func Terminate() error {
	initTermLock.Lock()
	if terminating {
		initTermLock.Unlock()
		return errs.New("termination already in progress")
	}
	if !initialized {
		initTermLock.Unlock()
		return errs.New("initialization has not been performed")
	}
	terminating = true
	initTermLock.Unlock()
	defer func() {
		initTermLock.Lock()
		terminating = false
		initTermLock.Unlock()
	}()
	C.plafTerminate()
	return nil
}

func isInfOrNaN(value float64) bool {
	return value != value || value < -math.MaxFloat64 || value > math.MaxFloat64
}

// ---------- Window ----------

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
	var pinner runtime.Pinner
	defer pinner.Unpin()
	cImgs := make([]C.plafImageData, 0, len(images))
	for _, img := range images {
		if img.Rect.Dx() > 0 && img.Rect.Dy() > 0 {
			cImgs = append(cImgs, imageToCImageData(&pinner, img))
		}
	}
	if len(cImgs) == 0 {
		C.plafSetWindowIcon(w.plafWnd, 0, nil)
	} else {
		C.plafSetWindowIcon(w.plafWnd, C.int(len(images)), &cImgs[0])
		runtime.KeepAlive(cImgs)
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

// MakeContextCurrent makes the context of the window current.
func (w *Window) MakeContextCurrent() {
	C.plafMakeContextCurrent(w.plafWnd)
}

// SwapBuffers swaps the front and back buffers of the window.
func (w *Window) SwapBuffers() {
	C.plafSwapBuffers(w.plafWnd)
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

// GetKey returns the last reported state of a keyboard key. The returned state
// is one of Press or Release. The higher-level state Repeat is only reported to
// the key callback.
//
// If the StickyKeys input mode is enabled, this function returns Press the first
// time you call this function after a key has been pressed, even if the key has
// already been released.
//
// The key functions deal with physical keys, with key tokens named after their
// use on the standard US keyboard layout. If you want to input text, use the
// Unicode character callback instead.
func (w *Window) GetKey(key Key) Action {
	return Action(C.plafGetKey(w.plafWnd, C.int(key)))
}

// GetMouseButton returns the last state reported for the specified mouse button.
//
// If the StickyMouseButtons input mode is enabled, this function returns Press
// the first time you call this function after a mouse button has been pressed,
// even if the mouse button has already been released.
func (w *Window) GetMouseButton(button MouseButton) Action {
	return Action(C.plafGetMouseButton(w.plafWnd, C.int(button)))
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

// ---------- Callbacks ----------

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
