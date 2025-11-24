package plaf

//#include "platform.h"
import "C"

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
