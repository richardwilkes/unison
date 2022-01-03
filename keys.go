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
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/richardwilkes/toolbox/i18n"
)

// KeyCode holds a virtual key code.
type KeyCode int

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
	KeyNumPadEnd      = KeyNumPad1
	KeyNumPadDown     = KeyNumPad2
	KeyNumPadPageDown = KeyNumPad3
	KeyNumPadLeft     = KeyNumPad4
	KeyNumPadRight    = KeyNumPad6
	KeyNumPadHome     = KeyNumPad7
	KeyNumPadUp       = KeyNumPad8
	KeyNumPadPageUp   = KeyNumPad9
	KeyNumPadDelete   = KeyNumPadDecimal
	KeyClear          = KeyNumLock
	KeyFn             = KeyInsert
)

// KeyCodeToName maps virtual key codes to a human-readable name.
var KeyCodeToName = map[KeyCode]string{
	KeySpace:          i18n.Text("Space"),
	KeyApostrophe:     i18n.Text("Apostrophe"),
	KeyComma:          i18n.Text("Comma"),
	KeyMinus:          i18n.Text("Minus"),
	KeyPeriod:         i18n.Text("Period"),
	KeySlash:          i18n.Text("Slash"),
	Key0:              i18n.Text("0"),
	Key1:              i18n.Text("1"),
	Key2:              i18n.Text("2"),
	Key3:              i18n.Text("3"),
	Key4:              i18n.Text("4"),
	Key5:              i18n.Text("5"),
	Key6:              i18n.Text("6"),
	Key7:              i18n.Text("7"),
	Key8:              i18n.Text("8"),
	Key9:              i18n.Text("9"),
	KeySemiColon:      i18n.Text("SemiColon"),
	KeyEqual:          i18n.Text("Equal"),
	KeyA:              i18n.Text("A"),
	KeyB:              i18n.Text("B"),
	KeyC:              i18n.Text("C"),
	KeyD:              i18n.Text("D"),
	KeyE:              i18n.Text("E"),
	KeyF:              i18n.Text("F"),
	KeyG:              i18n.Text("G"),
	KeyH:              i18n.Text("H"),
	KeyI:              i18n.Text("I"),
	KeyJ:              i18n.Text("J"),
	KeyK:              i18n.Text("K"),
	KeyL:              i18n.Text("L"),
	KeyM:              i18n.Text("M"),
	KeyN:              i18n.Text("N"),
	KeyO:              i18n.Text("O"),
	KeyP:              i18n.Text("P"),
	KeyQ:              i18n.Text("Q"),
	KeyR:              i18n.Text("R"),
	KeyS:              i18n.Text("S"),
	KeyT:              i18n.Text("T"),
	KeyU:              i18n.Text("U"),
	KeyV:              i18n.Text("V"),
	KeyW:              i18n.Text("W"),
	KeyX:              i18n.Text("X"),
	KeyY:              i18n.Text("Y"),
	KeyZ:              i18n.Text("Z"),
	KeyOpenBracket:    i18n.Text("OpenBracket"),
	KeyBackslash:      i18n.Text("Backslash"),
	KeyCloseBracket:   i18n.Text("CloseBracket"),
	KeyBackQuote:      i18n.Text("BackQuote"),
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
	KeyF1:             i18n.Text("F1"),
	KeyF2:             i18n.Text("F2"),
	KeyF3:             i18n.Text("F3"),
	KeyF4:             i18n.Text("F4"),
	KeyF5:             i18n.Text("F5"),
	KeyF6:             i18n.Text("F6"),
	KeyF7:             i18n.Text("F7"),
	KeyF8:             i18n.Text("F8"),
	KeyF9:             i18n.Text("F9"),
	KeyF10:            i18n.Text("F10"),
	KeyF11:            i18n.Text("F11"),
	KeyF12:            i18n.Text("F12"),
	KeyF13:            i18n.Text("F13"),
	KeyF14:            i18n.Text("F14"),
	KeyF15:            i18n.Text("F15"),
	KeyF16:            i18n.Text("F16"),
	KeyF17:            i18n.Text("F17"),
	KeyF18:            i18n.Text("F18"),
	KeyF19:            i18n.Text("F19"),
	KeyF20:            i18n.Text("F20"),
	KeyF21:            i18n.Text("F21"),
	KeyF22:            i18n.Text("F22"),
	KeyF23:            i18n.Text("F23"),
	KeyF24:            i18n.Text("F24"),
	KeyF25:            i18n.Text("F25"),
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

// IsControlAction returns true if the keyCode should trigger a control, such as a button, that is focused.
func IsControlAction(keyCode KeyCode, mod Modifiers) bool {
	return mod&NonStickyModifiers == 0 && (keyCode == KeyReturn || keyCode == KeyNumPadEnter || keyCode == KeySpace)
}
