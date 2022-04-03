// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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
	"github.com/richardwilkes/json"
	"github.com/richardwilkes/toolbox/i18n"
)

var _ json.Omitter = KeyCode(0)

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
	// KeyCodeToName maps virtual key codes to a human-readable name.
	KeyCodeToName = map[KeyCode]string{
		KeySpace:          i18n.Text("Space"),
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
		KeyWorld1:         i18n.Text("World1"),
		KeyWorld2:         i18n.Text("World2"),
		KeyEscape:         i18n.Text("Escape"),
		KeyReturn:         i18n.Text("Return"),
		KeyTab:            i18n.Text("Tab"),
		KeyBackspace:      i18n.Text("Backspace"),
		KeyInsert:         i18n.Text("Insert"),
		KeyDelete:         i18n.Text("Delete"),
		KeyRight:          i18n.Text("Right"),
		KeyLeft:           i18n.Text("Left"),
		KeyDown:           i18n.Text("Down"),
		KeyUp:             i18n.Text("Up"),
		KeyPageUp:         i18n.Text("PageUp"),
		KeyPageDown:       i18n.Text("PageDown"),
		KeyHome:           i18n.Text("Home"),
		KeyEnd:            i18n.Text("End"),
		KeyCapsLock:       i18n.Text("CapsLock"),
		KeyScrollLock:     i18n.Text("ScrollLock"),
		KeyNumLock:        i18n.Text("NumLock"),
		KeyPrintScreen:    i18n.Text("PrintScreen"),
		KeyPause:          i18n.Text("Pause"),
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
		KeyNumPad0:        i18n.Text("NumPad-0"),
		KeyNumPad1:        i18n.Text("NumPad-1"),
		KeyNumPad2:        i18n.Text("NumPad-2"),
		KeyNumPad3:        i18n.Text("NumPad-3"),
		KeyNumPad4:        i18n.Text("NumPad-4"),
		KeyNumPad5:        i18n.Text("NumPad-5"),
		KeyNumPad6:        i18n.Text("NumPad-6"),
		KeyNumPad7:        i18n.Text("NumPad-7"),
		KeyNumPad8:        i18n.Text("NumPad-8"),
		KeyNumPad9:        i18n.Text("NumPad-9"),
		KeyNumPadDecimal:  i18n.Text("NumPad-Decimal"),
		KeyNumPadDivide:   i18n.Text("NumPad-Divide"),
		KeyNumPadMultiply: i18n.Text("NumPad-Multiply"),
		KeyNumPadSubtract: i18n.Text("NumPad-Minus"),
		KeyNumPadAdd:      i18n.Text("NumPad-Add"),
		KeyNumPadEnter:    i18n.Text("NumPad-Enter"),
		KeyNumPadEqual:    i18n.Text("NumPad-Equal"),
		KeyLShift:         i18n.Text("Left-Shift"),
		KeyLControl:       i18n.Text("Left-Control"),
		KeyLOption:        i18n.Text("Left-Option"),
		KeyLCommand:       i18n.Text("Left-Command"),
		KeyRShift:         i18n.Text("Right-Shift"),
		KeyRControl:       i18n.Text("Right-Control"),
		KeyROption:        i18n.Text("Right-Option"),
		KeyRCommand:       i18n.Text("Right-Command"),
		KeyMenu:           i18n.Text("Menu"),
	}
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
	return mod&NonStickyModifiers == 0 && (keyCode == KeyReturn || keyCode == KeyNumPadEnter || keyCode == KeySpace)
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
	if k.ShouldOmit() {
		return ""
	}
	if v, ok := keyCodeToKey[k]; ok {
		return v
	}
	return fmt.Sprintf("#%d", k)
}

func (k KeyCode) String() string {
	if k.ShouldOmit() {
		return ""
	}
	if v, ok := KeyCodeToName[k]; ok {
		return v
	}
	return fmt.Sprintf("#%d", k)
}

// ShouldOmit implements json.Omitter.
func (k KeyCode) ShouldOmit() bool {
	return k == 0 || k == KeyNone
}
