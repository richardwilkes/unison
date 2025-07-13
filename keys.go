// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/richardwilkes/toolbox/v2/i18n"
)

// KeyCode holds a virtual key code.
type KeyCode int16

// Virtual key codes.
const (
	KeyNone           = KeyCode(glfw.KeyUnknown)
	KeySpace          = KeyCode(glfw.KeySpace)
	KeyApostrophe     = KeyCode(glfw.KeyApostrophe)
	KeyComma          = KeyCode(glfw.KeyComma)
	KeyMinus          = KeyCode(glfw.KeyMinus)
	KeyPeriod         = KeyCode(glfw.KeyPeriod)
	KeySlash          = KeyCode(glfw.KeySlash)
	Key0              = KeyCode(glfw.Key0)
	Key1              = KeyCode(glfw.Key1)
	Key2              = KeyCode(glfw.Key2)
	Key3              = KeyCode(glfw.Key3)
	Key4              = KeyCode(glfw.Key4)
	Key5              = KeyCode(glfw.Key5)
	Key6              = KeyCode(glfw.Key6)
	Key7              = KeyCode(glfw.Key7)
	Key8              = KeyCode(glfw.Key8)
	Key9              = KeyCode(glfw.Key9)
	KeySemiColon      = KeyCode(glfw.KeySemicolon)
	KeyEqual          = KeyCode(glfw.KeyEqual)
	KeyA              = KeyCode(glfw.KeyA)
	KeyB              = KeyCode(glfw.KeyB)
	KeyC              = KeyCode(glfw.KeyC)
	KeyD              = KeyCode(glfw.KeyD)
	KeyE              = KeyCode(glfw.KeyE)
	KeyF              = KeyCode(glfw.KeyF)
	KeyG              = KeyCode(glfw.KeyG)
	KeyH              = KeyCode(glfw.KeyH)
	KeyI              = KeyCode(glfw.KeyI)
	KeyJ              = KeyCode(glfw.KeyJ)
	KeyK              = KeyCode(glfw.KeyK)
	KeyL              = KeyCode(glfw.KeyL)
	KeyM              = KeyCode(glfw.KeyM)
	KeyN              = KeyCode(glfw.KeyN)
	KeyO              = KeyCode(glfw.KeyO)
	KeyP              = KeyCode(glfw.KeyP)
	KeyQ              = KeyCode(glfw.KeyQ)
	KeyR              = KeyCode(glfw.KeyR)
	KeyS              = KeyCode(glfw.KeyS)
	KeyT              = KeyCode(glfw.KeyT)
	KeyU              = KeyCode(glfw.KeyU)
	KeyV              = KeyCode(glfw.KeyV)
	KeyW              = KeyCode(glfw.KeyW)
	KeyX              = KeyCode(glfw.KeyX)
	KeyY              = KeyCode(glfw.KeyY)
	KeyZ              = KeyCode(glfw.KeyZ)
	KeyOpenBracket    = KeyCode(glfw.KeyLeftBracket)
	KeyBackslash      = KeyCode(glfw.KeyBackslash)
	KeyCloseBracket   = KeyCode(glfw.KeyRightBracket)
	KeyBackQuote      = KeyCode(glfw.KeyGraveAccent)
	KeyWorld1         = KeyCode(glfw.KeyWorld1)
	KeyWorld2         = KeyCode(glfw.KeyWorld2)
	KeyEscape         = KeyCode(glfw.KeyEscape)
	KeyReturn         = KeyCode(glfw.KeyEnter)
	KeyTab            = KeyCode(glfw.KeyTab)
	KeyBackspace      = KeyCode(glfw.KeyBackspace)
	KeyInsert         = KeyCode(glfw.KeyInsert)
	KeyDelete         = KeyCode(glfw.KeyDelete)
	KeyRight          = KeyCode(glfw.KeyRight)
	KeyLeft           = KeyCode(glfw.KeyLeft)
	KeyDown           = KeyCode(glfw.KeyDown)
	KeyUp             = KeyCode(glfw.KeyUp)
	KeyPageUp         = KeyCode(glfw.KeyPageUp)
	KeyPageDown       = KeyCode(glfw.KeyPageDown)
	KeyHome           = KeyCode(glfw.KeyHome)
	KeyEnd            = KeyCode(glfw.KeyEnd)
	KeyCapsLock       = KeyCode(glfw.KeyCapsLock)
	KeyScrollLock     = KeyCode(glfw.KeyScrollLock)
	KeyNumLock        = KeyCode(glfw.KeyNumLock)
	KeyPrintScreen    = KeyCode(glfw.KeyPrintScreen)
	KeyPause          = KeyCode(glfw.KeyPause)
	KeyF1             = KeyCode(glfw.KeyF1)
	KeyF2             = KeyCode(glfw.KeyF2)
	KeyF3             = KeyCode(glfw.KeyF3)
	KeyF4             = KeyCode(glfw.KeyF4)
	KeyF5             = KeyCode(glfw.KeyF5)
	KeyF6             = KeyCode(glfw.KeyF6)
	KeyF7             = KeyCode(glfw.KeyF7)
	KeyF8             = KeyCode(glfw.KeyF8)
	KeyF9             = KeyCode(glfw.KeyF9)
	KeyF10            = KeyCode(glfw.KeyF10)
	KeyF11            = KeyCode(glfw.KeyF11)
	KeyF12            = KeyCode(glfw.KeyF12)
	KeyF13            = KeyCode(glfw.KeyF13)
	KeyF14            = KeyCode(glfw.KeyF14)
	KeyF15            = KeyCode(glfw.KeyF15)
	KeyF16            = KeyCode(glfw.KeyF16)
	KeyF17            = KeyCode(glfw.KeyF17)
	KeyF18            = KeyCode(glfw.KeyF18)
	KeyF19            = KeyCode(glfw.KeyF19)
	KeyF20            = KeyCode(glfw.KeyF20)
	KeyF21            = KeyCode(glfw.KeyF21)
	KeyF22            = KeyCode(glfw.KeyF22)
	KeyF23            = KeyCode(glfw.KeyF23)
	KeyF24            = KeyCode(glfw.KeyF24)
	KeyF25            = KeyCode(glfw.KeyF25)
	KeyNumPad0        = KeyCode(glfw.KeyKP0)
	KeyNumPad1        = KeyCode(glfw.KeyKP1)
	KeyNumPad2        = KeyCode(glfw.KeyKP2)
	KeyNumPad3        = KeyCode(glfw.KeyKP3)
	KeyNumPad4        = KeyCode(glfw.KeyKP4)
	KeyNumPad5        = KeyCode(glfw.KeyKP5)
	KeyNumPad6        = KeyCode(glfw.KeyKP6)
	KeyNumPad7        = KeyCode(glfw.KeyKP7)
	KeyNumPad8        = KeyCode(glfw.KeyKP8)
	KeyNumPad9        = KeyCode(glfw.KeyKP9)
	KeyNumPadDecimal  = KeyCode(glfw.KeyKPDecimal)
	KeyNumPadDivide   = KeyCode(glfw.KeyKPDivide)
	KeyNumPadMultiply = KeyCode(glfw.KeyKPMultiply)
	KeyNumPadSubtract = KeyCode(glfw.KeyKPSubtract)
	KeyNumPadAdd      = KeyCode(glfw.KeyKPAdd)
	KeyNumPadEnter    = KeyCode(glfw.KeyKPEnter)
	KeyNumPadEqual    = KeyCode(glfw.KeyKPEqual)
	KeyLShift         = KeyCode(glfw.KeyLeftShift)
	KeyLControl       = KeyCode(glfw.KeyLeftControl)
	KeyLOption        = KeyCode(glfw.KeyLeftAlt)
	KeyLCommand       = KeyCode(glfw.KeyLeftSuper)
	KeyRShift         = KeyCode(glfw.KeyRightShift)
	KeyRControl       = KeyCode(glfw.KeyRightControl)
	KeyROption        = KeyCode(glfw.KeyRightAlt)
	KeyRCommand       = KeyCode(glfw.KeyRightSuper)
	KeyMenu           = KeyCode(glfw.KeyMenu)
	KeyLast           = KeyCode(glfw.KeyLast)
)

var (
	keyCodeToKey = map[KeyCode]string{
		KeySpace:          "space",
		KeyApostrophe:     "'",
		KeyComma:          ",",
		KeyMinus:          "-",
		KeyPeriod:         ".",
		KeySlash:          "/",
		Key0:              "0",
		Key1:              "1",
		Key2:              "2",
		Key3:              "3",
		Key4:              "4",
		Key5:              "5",
		Key6:              "6",
		Key7:              "7",
		Key8:              "8",
		Key9:              "9",
		KeySemiColon:      ";",
		KeyEqual:          "=",
		KeyA:              "A",
		KeyB:              "B",
		KeyC:              "C",
		KeyD:              "D",
		KeyE:              "E",
		KeyF:              "F",
		KeyG:              "G",
		KeyH:              "H",
		KeyI:              "I",
		KeyJ:              "J",
		KeyK:              "K",
		KeyL:              "L",
		KeyM:              "M",
		KeyN:              "N",
		KeyO:              "O",
		KeyP:              "P",
		KeyQ:              "Q",
		KeyR:              "R",
		KeyS:              "S",
		KeyT:              "T",
		KeyU:              "U",
		KeyV:              "V",
		KeyW:              "W",
		KeyX:              "X",
		KeyY:              "Y",
		KeyZ:              "Z",
		KeyOpenBracket:    "[",
		KeyBackslash:      "\\",
		KeyCloseBracket:   "]",
		KeyBackQuote:      "`",
		KeyWorld1:         "world1",
		KeyWorld2:         "world2",
		KeyEscape:         "escape",
		KeyReturn:         "return",
		KeyTab:            "tab",
		KeyBackspace:      "backspace",
		KeyInsert:         "insert",
		KeyDelete:         "delete",
		KeyRight:          "right",
		KeyLeft:           "left",
		KeyDown:           "down",
		KeyUp:             "up",
		KeyPageUp:         "pageup",
		KeyPageDown:       "pagedown",
		KeyHome:           "home",
		KeyEnd:            "end",
		KeyCapsLock:       "caps",
		KeyScrollLock:     "scroll",
		KeyNumLock:        "num",
		KeyPrintScreen:    "print",
		KeyPause:          "pause",
		KeyF1:             "F1",
		KeyF2:             "F2",
		KeyF3:             "F3",
		KeyF4:             "F4",
		KeyF5:             "F5",
		KeyF6:             "F6",
		KeyF7:             "F7",
		KeyF8:             "F8",
		KeyF9:             "F9",
		KeyF10:            "F10",
		KeyF11:            "F11",
		KeyF12:            "F12",
		KeyF13:            "F13",
		KeyF14:            "F14",
		KeyF15:            "F15",
		KeyF16:            "F16",
		KeyF17:            "F17",
		KeyF18:            "F18",
		KeyF19:            "F19",
		KeyF20:            "F20",
		KeyF21:            "F21",
		KeyF22:            "F22",
		KeyF23:            "F23",
		KeyF24:            "F24",
		KeyF25:            "F25",
		KeyNumPad0:        "numpad0",
		KeyNumPad1:        "numpad1",
		KeyNumPad2:        "numpad2",
		KeyNumPad3:        "numpad3",
		KeyNumPad4:        "numpad4",
		KeyNumPad5:        "numpad5",
		KeyNumPad6:        "numpad6",
		KeyNumPad7:        "numpad7",
		KeyNumPad8:        "numpad8",
		KeyNumPad9:        "numpad9",
		KeyNumPadDecimal:  "numpad_decimal",
		KeyNumPadDivide:   "numpad_divide",
		KeyNumPadMultiply: "numpad_multiply",
		KeyNumPadSubtract: "numpad_minus",
		KeyNumPadAdd:      "numpad_add",
		KeyNumPadEnter:    "numpad_enter",
		KeyNumPadEqual:    "numpad_equal",
		KeyLShift:         "left_shift",
		KeyLControl:       "left_control",
		KeyLOption:        "left_option",
		KeyLCommand:       "left_command",
		KeyRShift:         "right_shift",
		KeyRControl:       "right_control",
		KeyROption:        "right_option",
		KeyRCommand:       "right_command",
		KeyMenu:           "menu",
	}
	keyToKeyCode = make(map[string]KeyCode)
)

func init() {
	for k, v := range keyCodeToKey {
		keyToKeyCode[v] = k
	}
}

// IsControlAction returns true if the keyCode should trigger a control, such as a button, that is focused.
func IsControlAction(keyCode KeyCode, mod Modifiers) bool {
	return mod&NonStickyModifiers == 0 && keyCode == KeySpace
}

// MarshalText implements encoding.TextMarshaler.
func (k KeyCode) MarshalText() (text []byte, err error) {
	return []byte(k.Key()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (k *KeyCode) UnmarshalText(text []byte) error {
	*k = KeyCodeFromKey(string(text))
	return nil
}

// KeyCodeFromKey extracts KeyCode from a string created via a call to .Key().
func KeyCodeFromKey(key string) KeyCode {
	if v, ok := keyToKeyCode[key]; ok {
		return v
	}
	if strings.HasPrefix(key, "#") {
		if v, err := strconv.Atoi(key[1:]); err == nil {
			return KeyCode(v)
		}
	}
	return KeyNone
}

// Key returns a string version of the KeyCode for the purpose of serialization.
func (k KeyCode) Key() string {
	if k.IsZero() {
		return ""
	}
	if v, ok := keyCodeToKey[k]; ok {
		return v
	}
	return fmt.Sprintf("#%d", k)
}

func (k KeyCode) String() string {
	if k.IsZero() {
		return ""
	}
	switch k {
	case KeySpace:
		return i18n.Text("Space")
	case KeyApostrophe:
		return "'"
	case KeyComma:
		return ","
	case KeyMinus:
		return "-"
	case KeyPeriod:
		return "."
	case KeySlash:
		return "/"
	case Key0:
		return "0"
	case Key1:
		return "1"
	case Key2:
		return "2"
	case Key3:
		return "3"
	case Key4:
		return "4"
	case Key5:
		return "5"
	case Key6:
		return "6"
	case Key7:
		return "7"
	case Key8:
		return "8"
	case Key9:
		return "9"
	case KeySemiColon:
		return ";"
	case KeyEqual:
		return "="
	case KeyA:
		return "A"
	case KeyB:
		return "B"
	case KeyC:
		return "C"
	case KeyD:
		return "D"
	case KeyE:
		return "E"
	case KeyF:
		return "F"
	case KeyG:
		return "G"
	case KeyH:
		return "H"
	case KeyI:
		return "I"
	case KeyJ:
		return "J"
	case KeyK:
		return "K"
	case KeyL:
		return "L"
	case KeyM:
		return "M"
	case KeyN:
		return "N"
	case KeyO:
		return "O"
	case KeyP:
		return "P"
	case KeyQ:
		return "Q"
	case KeyR:
		return "R"
	case KeyS:
		return "S"
	case KeyT:
		return "T"
	case KeyU:
		return "U"
	case KeyV:
		return "V"
	case KeyW:
		return "W"
	case KeyX:
		return "X"
	case KeyY:
		return "Y"
	case KeyZ:
		return "Z"
	case KeyOpenBracket:
		return "["
	case KeyBackslash:
		return "\\"
	case KeyCloseBracket:
		return "]"
	case KeyBackQuote:
		return "`"
	case KeyWorld1:
		return i18n.Text("World1")
	case KeyWorld2:
		return i18n.Text("World2")
	case KeyEscape:
		return i18n.Text("Escape")
	case KeyReturn:
		return i18n.Text("Return")
	case KeyTab:
		return i18n.Text("Tab")
	case KeyBackspace:
		return i18n.Text("Backspace")
	case KeyInsert:
		return i18n.Text("Insert")
	case KeyDelete:
		return i18n.Text("Delete")
	case KeyRight:
		return i18n.Text("Right")
	case KeyLeft:
		return i18n.Text("Left")
	case KeyDown:
		return i18n.Text("Down")
	case KeyUp:
		return i18n.Text("Up")
	case KeyPageUp:
		return i18n.Text("PageUp")
	case KeyPageDown:
		return i18n.Text("PageDown")
	case KeyHome:
		return i18n.Text("Home")
	case KeyEnd:
		return i18n.Text("End")
	case KeyCapsLock:
		return i18n.Text("CapsLock")
	case KeyScrollLock:
		return i18n.Text("ScrollLock")
	case KeyNumLock:
		return i18n.Text("NumLock")
	case KeyPrintScreen:
		return i18n.Text("PrintScreen")
	case KeyPause:
		return i18n.Text("Pause")
	case KeyF1:
		return "F1"
	case KeyF2:
		return "F2"
	case KeyF3:
		return "F3"
	case KeyF4:
		return "F4"
	case KeyF5:
		return "F5"
	case KeyF6:
		return "F6"
	case KeyF7:
		return "F7"
	case KeyF8:
		return "F8"
	case KeyF9:
		return "F9"
	case KeyF10:
		return "F10"
	case KeyF11:
		return "F11"
	case KeyF12:
		return "F12"
	case KeyF13:
		return "F13"
	case KeyF14:
		return "F14"
	case KeyF15:
		return "F15"
	case KeyF16:
		return "F16"
	case KeyF17:
		return "F17"
	case KeyF18:
		return "F18"
	case KeyF19:
		return "F19"
	case KeyF20:
		return "F20"
	case KeyF21:
		return "F21"
	case KeyF22:
		return "F22"
	case KeyF23:
		return "F23"
	case KeyF24:
		return "F24"
	case KeyF25:
		return "F25"
	case KeyNumPad0:
		return i18n.Text("NumPad-0")
	case KeyNumPad1:
		return i18n.Text("NumPad-1")
	case KeyNumPad2:
		return i18n.Text("NumPad-2")
	case KeyNumPad3:
		return i18n.Text("NumPad-3")
	case KeyNumPad4:
		return i18n.Text("NumPad-4")
	case KeyNumPad5:
		return i18n.Text("NumPad-5")
	case KeyNumPad6:
		return i18n.Text("NumPad-6")
	case KeyNumPad7:
		return i18n.Text("NumPad-7")
	case KeyNumPad8:
		return i18n.Text("NumPad-8")
	case KeyNumPad9:
		return i18n.Text("NumPad-9")
	case KeyNumPadDecimal:
		return i18n.Text("NumPad-Decimal")
	case KeyNumPadDivide:
		return i18n.Text("NumPad-Divide")
	case KeyNumPadMultiply:
		return i18n.Text("NumPad-Multiply")
	case KeyNumPadSubtract:
		return i18n.Text("NumPad-Minus")
	case KeyNumPadAdd:
		return i18n.Text("NumPad-Add")
	case KeyNumPadEnter:
		return i18n.Text("NumPad-Enter")
	case KeyNumPadEqual:
		return i18n.Text("NumPad-Equal")
	case KeyLShift:
		return i18n.Text("Left-Shift")
	case KeyLControl:
		return i18n.Text("Left-Control")
	case KeyLOption:
		return i18n.Text("Left-Option")
	case KeyLCommand:
		return i18n.Text("Left-Command")
	case KeyRShift:
		return i18n.Text("Right-Shift")
	case KeyRControl:
		return i18n.Text("Right-Control")
	case KeyROption:
		return i18n.Text("Right-Option")
	case KeyRCommand:
		return i18n.Text("Right-Command")
	case KeyMenu:
		return i18n.Text("Menu")
	default:
		return fmt.Sprintf("#%d", k)
	}
}

// IsZero implements json.isZero.
func (k KeyCode) IsZero() bool {
	return k == 0 || k == KeyNone
}
