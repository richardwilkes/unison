package plaf

/*
#include "platform.h"

void goCharCallback(GLFWwindow *window, unsigned int ch);
void goCharModsCallback(GLFWwindow *window, unsigned int ch, int mods);
void goCursorEnterCallback(GLFWwindow *window, int entered);
void goCursorPosCallback(GLFWwindow *window, double xpos, double ypos);
void goDropCallback(GLFWwindow *window, int count, char **names);
void goKeyCallback(GLFWwindow *window, int key, int scancode, int action, int mods);
void goMouseButtonCallback(GLFWwindow *window, int button, int action, int mods);
void goScrollCallback(GLFWwindow *window, double xoff, double yoff);
*/
import "C"

import (
	"image"
)

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
	ModShift    ModifierKey = C.MOD_SHIFT
	ModControl  ModifierKey = C.MOD_CONTROL
	ModAlt      ModifierKey = C.MOD_ALT
	ModSuper    ModifierKey = C.MOD_SUPER
	ModCapsLock ModifierKey = C.MOD_CAPS_LOCK
	ModNumLock  ModifierKey = C.MOD_NUM_LOCK
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

// Action corresponds to a key or button action.
type Action int

// Action types.
const (
	Release Action = C.INPUT_RELEASE // The key or button was released.
	Press   Action = C.INPUT_PRESS   // The key or button was pressed.
	Repeat  Action = C.INPUT_REPEAT  // The key was held down until it repeated.
)

// InputMode corresponds to an input mode.
type InputMode int

// Input modes.
const (
	InputModeCursor                InputMode = C.INPUT_MODE_CURSOR                  // See Cursor mode values
	InputModeStickyKeys            InputMode = C.INPUT_MODE_STICKY_KEYS             // Value can be either 1 or 0
	InputModeStickyMouseButtons    InputMode = C.INPUT_MODE_STICKY_MOUSE_BUTTONS    // Value can be either 1 or 0
	InputModeLockKeyMods           InputMode = C.INPUT_MODE_LOCK_KEY_MODS           // Value can be either 1 or 0
	InputModeUnlimitedMouseButtons InputMode = C.INPUT_MODE_UNLIMITED_MOUSE_BUTTONS // Value can be either 1 or 0
)

// Cursor mode values.
const (
	CursorNormal int = C.CURSOR_NORMAL
	CursorHidden int = C.CURSOR_HIDDEN
)

// Cursor represents a cursor.
type Cursor struct {
	data *C.GLFWcursor
}

// GetInputMode returns the value of an input option of the window.
func (w *Window) GetInputMode(mode InputMode) int {
	ret := int(C.glfwGetInputMode(w.data, C.int(mode)))
	panicError()
	return ret
}

// SetInputMode sets an input option for the window.
func (w *Window) SetInputMode(mode InputMode, value int) {
	C.glfwSetInputMode(w.data, C.int(mode), C.int(value))
	panicError()
}

// GetKeyScancode function returns the platform-specific scancode of the
// specified key.
//
// If the key is KeyUnknown or does not exist on the keyboard this method will
// return -1.
func GetKeyScancode(key Key) int {
	return int(C.glfwGetKeyScancode(C.int(key)))
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
	ret := Action(C.glfwGetKey(w.data, C.int(key)))
	panicError()
	return ret
}

// GetMouseButton returns the last state reported for the specified mouse button.
//
// If the StickyMouseButtons input mode is enabled, this function returns Press
// the first time you call this function after a mouse button has been pressed,
// even if the mouse button has already been released.
func (w *Window) GetMouseButton(button MouseButton) Action {
	ret := Action(C.glfwGetMouseButton(w.data, C.int(button)))
	panicError()
	return ret
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
// Like all other coordinate systems in GLFW, the X-axis points to the right and the Y-axis points down.
func CreateCursor(img image.Image, xhot, yhot int) *Cursor {
	imgC, free := imageToGLFW(img)
	cursor := C.glfwCreateCursor(&imgC, C.int(xhot), C.int(yhot))
	free()
	panicError()
	return &Cursor{cursor}
}

// CreateStandardCursor returns a cursor with a standard shape,
// that can be set for a window with SetCursor.
func CreateStandardCursor(shape StandardCursor) *Cursor {
	cursor := C.glfwCreateStandardCursor(C.int(shape))
	panicError()
	return &Cursor{cursor}
}

// Destroy destroys a cursor previously created with CreateCursor.
// Any remaining cursors will be destroyed by Terminate.
func (c *Cursor) Destroy() {
	C.glfwDestroyCursor(c.data)
	panicError()
}

// SetCursor sets the cursor image to be used when the cursor is over the client area
// of the specified window. The set cursor will only be visible when the cursor mode of the
// window is CursorNormal.
//
// On some platforms, the set cursor may not be visible unless the window also has input focus.
func (w *Window) SetCursor(c *Cursor) {
	if c == nil {
		C.glfwSetCursor(w.data, nil)
	} else {
		C.glfwSetCursor(w.data, c.data)
	}
	panicError()
}

// KeyCallback is the key callback.
type KeyCallback func(w *Window, key Key, scancode int, action Action, mods ModifierKey)

// SetKeyCallback sets the key callback which is called when a key is pressed,
// repeated or released.
//
// The key functions deal with physical keys, with layout independent key tokens
// named after their values in the standard US keyboard layout. If you want to
// input text, use the SetCharCallback instead.
//
// When a window loses focus, it will generate synthetic key release events for
// all pressed keys. You can tell these events from user-generated events by the
// fact that the synthetic ones are generated after the window has lost focus,
// i.e. Focused will be false and the focus callback will have already been
// called.
func (w *Window) SetKeyCallback(cbfun KeyCallback) (previous KeyCallback) {
	previous = w.fKeyHolder
	w.fKeyHolder = cbfun
	var callback C.keyFunc
	if cbfun != nil {
		callback = C.keyFunc(C.goKeyCallback)
	}
	C.glfwSetKeyCallback(w.data, callback)
	panicError()
	return previous
}

// CharCallback is the character callback.
type CharCallback func(w *Window, char rune)

// SetCharCallback sets the character callback which is called when a
// Unicode character is input.
//
// The character callback is intended for Unicode text input. As it deals with
// characters, it is keyboard layout dependent, whereas the
// key callback is not. Characters do not map 1:1
// to physical keys, as a key may produce zero, one or more characters. If you
// want to know whether a specific physical key was pressed or released, see
// the key callback instead.
//
// The character callback behaves as system text input normally does and will
// not be called if modifier keys are held down that would prevent normal text
// input on that platform, for example a Super (Command) key on OS X or Alt key
// on Windows. There is a character with modifiers callback that receives these events.
func (w *Window) SetCharCallback(cbfun CharCallback) (previous CharCallback) {
	previous = w.fCharHolder
	w.fCharHolder = cbfun
	var callback C.charFunc
	if cbfun != nil {
		callback = C.charFunc(C.goCharCallback)
	}
	C.glfwSetCharCallback(w.data, callback)
	panicError()
	return previous
}

// CharModsCallback is the character with modifiers callback.
type CharModsCallback func(w *Window, char rune, mods ModifierKey)

// SetCharModsCallback sets the character with modifiers callback which is called when a
// Unicode character is input regardless of what modifier keys are used.
//
// Deprecated: Scheduled for removal in version 4.0.
//
// The character with modifiers callback is intended for implementing custom
// Unicode character input. For regular Unicode text input, see the
// character callback. Like the character callback, the character with modifiers callback
// deals with characters and is keyboard layout dependent. Characters do not
// map 1:1 to physical keys, as a key may produce zero, one or more characters.
// If you want to know whether a specific physical key was pressed or released,
// see the key callback instead.
func (w *Window) SetCharModsCallback(cbfun CharModsCallback) (previous CharModsCallback) {
	previous = w.fCharModsHolder
	w.fCharModsHolder = cbfun
	var callback C.charModsFunc
	if cbfun != nil {
		callback = C.charModsFunc(C.goCharModsCallback)
	}
	C.glfwSetCharModsCallback(w.data, callback)
	panicError()
	return previous
}

// MouseButtonCallback is the mouse button callback.
type MouseButtonCallback func(w *Window, button MouseButton, action Action, mods ModifierKey)

// SetMouseButtonCallback sets the mouse button callback which is called when a
// mouse button is pressed or released.
//
// When a window loses focus, it will generate synthetic mouse button release
// events for all pressed mouse buttons. You can tell these events from
// user-generated events by the fact that the synthetic ones are generated after
// the window has lost focus, i.e. Focused will be false and the focus
// callback will have already been called.
func (w *Window) SetMouseButtonCallback(cbfun MouseButtonCallback) (previous MouseButtonCallback) {
	previous = w.fMouseButtonHolder
	w.fMouseButtonHolder = cbfun
	var callback C.mouseButtonFunc
	if cbfun != nil {
		callback = C.mouseButtonFunc(C.goMouseButtonCallback)
	}
	C.glfwSetMouseButtonCallback(w.data, callback)
	panicError()
	return previous
}

// CursorPosCallback the cursor position callback.
type CursorPosCallback func(w *Window, xpos, ypos float64)

// SetCursorPosCallback sets the cursor position callback which is called
// when the cursor is moved. The callback is provided with the position relative
// to the upper-left corner of the client area of the window.
func (w *Window) SetCursorPosCallback(cbfun CursorPosCallback) (previous CursorPosCallback) {
	previous = w.fCursorPosHolder
	w.fCursorPosHolder = cbfun
	var callback C.cursorPosFunc
	if cbfun != nil {
		callback = C.cursorPosFunc(C.goCursorPosCallback)
	}
	C.glfwSetCursorPosCallback(w.data, callback)
	panicError()
	return previous
}

// CursorEnterCallback is the cursor boundary crossing callback.
type CursorEnterCallback func(w *Window, entered bool)

// SetCursorEnterCallback the cursor boundary crossing callback which is called
// when the cursor enters or leaves the client area of the window.
func (w *Window) SetCursorEnterCallback(cbfun CursorEnterCallback) (previous CursorEnterCallback) {
	previous = w.fCursorEnterHolder
	w.fCursorEnterHolder = cbfun
	var callback C.cursorEnterFunc
	if cbfun != nil {
		callback = C.cursorEnterFunc(C.goCursorEnterCallback)
	}
	C.glfwSetCursorEnterCallback(w.data, callback)
	panicError()
	return previous
}

// ScrollCallback is the scroll callback.
type ScrollCallback func(w *Window, xoff, yoff float64)

// SetScrollCallback sets the scroll callback which is called when a scrolling
// device is used, such as a mouse wheel or scrolling area of a touchpad.
func (w *Window) SetScrollCallback(cbfun ScrollCallback) (previous ScrollCallback) {
	previous = w.fScrollHolder
	w.fScrollHolder = cbfun
	var callback C.scrollFunc
	if cbfun != nil {
		callback = C.scrollFunc(C.goScrollCallback)
	}
	C.glfwSetScrollCallback(w.data, callback)
	panicError()
	return previous
}

// DropCallback is the drop callback.
type DropCallback func(w *Window, names []string)

// SetDropCallback sets the drop callback which is called when an object
// is dropped over the window.
func (w *Window) SetDropCallback(cbfun DropCallback) (previous DropCallback) {
	previous = w.fDropHolder
	w.fDropHolder = cbfun
	var callback C.dropFunc
	if cbfun != nil {
		callback = C.dropFunc(C.goDropCallback)
	}
	C.glfwSetDropCallback(w.data, callback)
	panicError()
	return previous
}
