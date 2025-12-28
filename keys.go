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

	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison/internal/plaf2"
)

// KeyCode holds a virtual key code.
type KeyCode int16

// Virtual key codes.
const (
	KeyNone           = KeyCode(plaf2.KeyUnknown)
	KeySpace          = KeyCode(plaf2.KeySpace)
	KeyApostrophe     = KeyCode(plaf2.KeyApostrophe)
	KeyComma          = KeyCode(plaf2.KeyComma)
	KeyMinus          = KeyCode(plaf2.KeyMinus)
	KeyPeriod         = KeyCode(plaf2.KeyPeriod)
	KeySlash          = KeyCode(plaf2.KeySlash)
	Key0              = KeyCode(plaf2.Key0)
	Key1              = KeyCode(plaf2.Key1)
	Key2              = KeyCode(plaf2.Key2)
	Key3              = KeyCode(plaf2.Key3)
	Key4              = KeyCode(plaf2.Key4)
	Key5              = KeyCode(plaf2.Key5)
	Key6              = KeyCode(plaf2.Key6)
	Key7              = KeyCode(plaf2.Key7)
	Key8              = KeyCode(plaf2.Key8)
	Key9              = KeyCode(plaf2.Key9)
	KeySemiColon      = KeyCode(plaf2.KeySemicolon)
	KeyEqual          = KeyCode(plaf2.KeyEqual)
	KeyA              = KeyCode(plaf2.KeyA)
	KeyB              = KeyCode(plaf2.KeyB)
	KeyC              = KeyCode(plaf2.KeyC)
	KeyD              = KeyCode(plaf2.KeyD)
	KeyE              = KeyCode(plaf2.KeyE)
	KeyF              = KeyCode(plaf2.KeyF)
	KeyG              = KeyCode(plaf2.KeyG)
	KeyH              = KeyCode(plaf2.KeyH)
	KeyI              = KeyCode(plaf2.KeyI)
	KeyJ              = KeyCode(plaf2.KeyJ)
	KeyK              = KeyCode(plaf2.KeyK)
	KeyL              = KeyCode(plaf2.KeyL)
	KeyM              = KeyCode(plaf2.KeyM)
	KeyN              = KeyCode(plaf2.KeyN)
	KeyO              = KeyCode(plaf2.KeyO)
	KeyP              = KeyCode(plaf2.KeyP)
	KeyQ              = KeyCode(plaf2.KeyQ)
	KeyR              = KeyCode(plaf2.KeyR)
	KeyS              = KeyCode(plaf2.KeyS)
	KeyT              = KeyCode(plaf2.KeyT)
	KeyU              = KeyCode(plaf2.KeyU)
	KeyV              = KeyCode(plaf2.KeyV)
	KeyW              = KeyCode(plaf2.KeyW)
	KeyX              = KeyCode(plaf2.KeyX)
	KeyY              = KeyCode(plaf2.KeyY)
	KeyZ              = KeyCode(plaf2.KeyZ)
	KeyOpenBracket    = KeyCode(plaf2.KeyLeftBracket)
	KeyBackslash      = KeyCode(plaf2.KeyBackslash)
	KeyCloseBracket   = KeyCode(plaf2.KeyRightBracket)
	KeyBackQuote      = KeyCode(plaf2.KeyGraveAccent)
	KeyWorld1         = KeyCode(plaf2.KeyWorld1)
	KeyWorld2         = KeyCode(plaf2.KeyWorld2)
	KeyEscape         = KeyCode(plaf2.KeyEscape)
	KeyReturn         = KeyCode(plaf2.KeyEnter)
	KeyTab            = KeyCode(plaf2.KeyTab)
	KeyBackspace      = KeyCode(plaf2.KeyBackspace)
	KeyInsert         = KeyCode(plaf2.KeyInsert)
	KeyDelete         = KeyCode(plaf2.KeyDelete)
	KeyRight          = KeyCode(plaf2.KeyRight)
	KeyLeft           = KeyCode(plaf2.KeyLeft)
	KeyDown           = KeyCode(plaf2.KeyDown)
	KeyUp             = KeyCode(plaf2.KeyUp)
	KeyPageUp         = KeyCode(plaf2.KeyPageUp)
	KeyPageDown       = KeyCode(plaf2.KeyPageDown)
	KeyHome           = KeyCode(plaf2.KeyHome)
	KeyEnd            = KeyCode(plaf2.KeyEnd)
	KeyCapsLock       = KeyCode(plaf2.KeyCapsLock)
	KeyScrollLock     = KeyCode(plaf2.KeyScrollLock)
	KeyNumLock        = KeyCode(plaf2.KeyNumLock)
	KeyPrintScreen    = KeyCode(plaf2.KeyPrintScreen)
	KeyPause          = KeyCode(plaf2.KeyPause)
	KeyF1             = KeyCode(plaf2.KeyF1)
	KeyF2             = KeyCode(plaf2.KeyF2)
	KeyF3             = KeyCode(plaf2.KeyF3)
	KeyF4             = KeyCode(plaf2.KeyF4)
	KeyF5             = KeyCode(plaf2.KeyF5)
	KeyF6             = KeyCode(plaf2.KeyF6)
	KeyF7             = KeyCode(plaf2.KeyF7)
	KeyF8             = KeyCode(plaf2.KeyF8)
	KeyF9             = KeyCode(plaf2.KeyF9)
	KeyF10            = KeyCode(plaf2.KeyF10)
	KeyF11            = KeyCode(plaf2.KeyF11)
	KeyF12            = KeyCode(plaf2.KeyF12)
	KeyF13            = KeyCode(plaf2.KeyF13)
	KeyF14            = KeyCode(plaf2.KeyF14)
	KeyF15            = KeyCode(plaf2.KeyF15)
	KeyF16            = KeyCode(plaf2.KeyF16)
	KeyF17            = KeyCode(plaf2.KeyF17)
	KeyF18            = KeyCode(plaf2.KeyF18)
	KeyF19            = KeyCode(plaf2.KeyF19)
	KeyF20            = KeyCode(plaf2.KeyF20)
	KeyF21            = KeyCode(plaf2.KeyF21)
	KeyF22            = KeyCode(plaf2.KeyF22)
	KeyF23            = KeyCode(plaf2.KeyF23)
	KeyF24            = KeyCode(plaf2.KeyF24)
	KeyF25            = KeyCode(plaf2.KeyF25)
	KeyNumPad0        = KeyCode(plaf2.KeyKp0)
	KeyNumPad1        = KeyCode(plaf2.KeyKp1)
	KeyNumPad2        = KeyCode(plaf2.KeyKp2)
	KeyNumPad3        = KeyCode(plaf2.KeyKp3)
	KeyNumPad4        = KeyCode(plaf2.KeyKp4)
	KeyNumPad5        = KeyCode(plaf2.KeyKp5)
	KeyNumPad6        = KeyCode(plaf2.KeyKp6)
	KeyNumPad7        = KeyCode(plaf2.KeyKp7)
	KeyNumPad8        = KeyCode(plaf2.KeyKp8)
	KeyNumPad9        = KeyCode(plaf2.KeyKp9)
	KeyNumPadDecimal  = KeyCode(plaf2.KeyKpDecimal)
	KeyNumPadDivide   = KeyCode(plaf2.KeyKpDivide)
	KeyNumPadMultiply = KeyCode(plaf2.KeyKpMultiply)
	KeyNumPadSubtract = KeyCode(plaf2.KeyKpSubtract)
	KeyNumPadAdd      = KeyCode(plaf2.KeyKpAdd)
	KeyNumPadEnter    = KeyCode(plaf2.KeyKpEnter)
	KeyNumPadEqual    = KeyCode(plaf2.KeyKpEqual)
	KeyLShift         = KeyCode(plaf2.KeyLeftShift)
	KeyLControl       = KeyCode(plaf2.KeyLeftControl)
	KeyLOption        = KeyCode(plaf2.KeyLeftAlt)
	KeyLCommand       = KeyCode(plaf2.KeyLeftSuper)
	KeyRShift         = KeyCode(plaf2.KeyRightShift)
	KeyRControl       = KeyCode(plaf2.KeyRightControl)
	KeyROption        = KeyCode(plaf2.KeyRightAlt)
	KeyRCommand       = KeyCode(plaf2.KeyRightSuper)
	KeyMenu           = KeyCode(plaf2.KeyMenu)
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
