// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

func apiFillKeyCodes() {
	const (
		xkSpace        = 0x0020
		xkApostrophe   = 0x0027
		xkComma        = 0x002c
		xkMinus        = 0x002d
		xkPeriod       = 0x002e
		xkSlash        = 0x002f
		xk0            = 0x0030
		xk1            = 0x0031
		xk2            = 0x0032
		xk3            = 0x0033
		xk4            = 0x0034
		xk5            = 0x0035
		xk6            = 0x0036
		xk7            = 0x0037
		xk8            = 0x0038
		xk9            = 0x0039
		xkSemicolon    = 0x003b
		xkLess         = 0x003c
		xkEqual        = 0x003d
		xkBracketLeft  = 0x005b
		xkBackslash    = 0x005c
		xkBracketRight = 0x005d
		xkGrave        = 0x0060
		xkA            = 0x0061
		xkB            = 0x0062
		xkC            = 0x0063
		xkD            = 0x0064
		xkE            = 0x0065
		xkF            = 0x0066
		xkG            = 0x0067
		xkH            = 0x0068
		xkI            = 0x0069
		xkJ            = 0x006a
		xkK            = 0x006b
		xkL            = 0x006c
		xkM            = 0x006d
		xkN            = 0x006e
		xkO            = 0x006f
		xkP            = 0x0070
		xkQ            = 0x0071
		xkR            = 0x0072
		xkS            = 0x0073
		xkT            = 0x0074
		xkU            = 0x0075
		xkV            = 0x0076
		xkW            = 0x0077
		xkX            = 0x0078
		xkY            = 0x0079
		xkZ            = 0x007a
		xkBackSpace    = 0xff08
		xkTab          = 0xff09
		xkReturn       = 0xff0d
		xkPause        = 0xff13
		xkScrollLock   = 0xff14
		xkEscape       = 0xff1b
		xkHome         = 0xff50
		xkLeft         = 0xff51
		xkUp           = 0xff52
		xkRight        = 0xff53
		xkDown         = 0xff54
		xkPageUp       = 0xff55
		xkPageDown     = 0xff56
		xkEnd          = 0xff57
		xkPrint        = 0xff61
		xkInsert       = 0xff63
		xkMenu         = 0xff67
		xkModeswitch   = 0xff7e
		xkNumLock      = 0xff7f
		xkKPEnter      = 0xff8d
		xkKPHome       = 0xff95
		xkKPLeft       = 0xff96
		xkKPUp         = 0xff97
		xkKPRight      = 0xff98
		xkKPDown       = 0xff99
		xkKPPageUp     = 0xff9a
		xkKPPageDown   = 0xff9b
		xkKPEnd        = 0xff9c
		xkKPInsert     = 0xff9e
		xkKPDelete     = 0xff9f
		xkKPMultiply   = 0xffaa
		xkKPAdd        = 0xffab
		xkKPSeparator  = 0xffac
		xkKPSubtract   = 0xffad
		xkKPDecimal    = 0xffae
		xkKPDivide     = 0xffaf
		xkKP0          = 0xffb0
		xkKP1          = 0xffb1
		xkKP2          = 0xffb2
		xkKP3          = 0xffb3
		xkKP4          = 0xffb4
		xkKP5          = 0xffb5
		xkKP6          = 0xffb6
		xkKP7          = 0xffb7
		xkKP8          = 0xffb8
		xkKP9          = 0xffb9
		xkKPEqual      = 0xffbd
		xkF1           = 0xffbe
		xkF2           = 0xffbf
		xkF3           = 0xffc0
		xkF4           = 0xffc1
		xkF5           = 0xffc2
		xkF6           = 0xffc3
		xkF7           = 0xffc4
		xkF8           = 0xffc5
		xkF9           = 0xffc6
		xkF10          = 0xffc7
		xkF11          = 0xffc8
		xkF12          = 0xffc9
		xkF13          = 0xffca
		xkF14          = 0xffcb
		xkF15          = 0xffcc
		xkF16          = 0xffcd
		xkF17          = 0xffce
		xkF18          = 0xffcf
		xkF19          = 0xffd0
		xkF20          = 0xffd1
		xkF21          = 0xffd2
		xkF22          = 0xffd3
		xkF23          = 0xffd4
		xkF24          = 0xffd5
		xkF25          = 0xffd6
		xkShiftL       = 0xffe1
		xkShiftR       = 0xffe2
		xkControlL     = 0xffe3
		xkControlR     = 0xffe4
		xkCapsLock     = 0xffe5
		xkMetaL        = 0xffe7
		xkMetaR        = 0xffe8
		xkAltL         = 0xffe9
		xkAltR         = 0xffea
		xkSuperL       = 0xffeb
		xkSuperR       = 0xffec
		xkDelete       = 0xffff
	)
	secondary := map[uint32]KeyCode{
		xkKPEnter:     KeyNumPadEnter,
		xkKPSeparator: KeyNumPadDecimal,
		xkKPDecimal:   KeyNumPadDecimal,
		xkKP0:         KeyNumPad0,
		xkKP1:         KeyNumPad1,
		xkKP2:         KeyNumPad2,
		xkKP3:         KeyNumPad3,
		xkKP4:         KeyNumPad4,
		xkKP5:         KeyNumPad5,
		xkKP6:         KeyNumPad6,
		xkKP7:         KeyNumPad7,
		xkKP8:         KeyNumPad8,
		xkKP9:         KeyNumPad9,
		xkKPEqual:     KeyNumPadEqual,
	}
	primary := map[uint32]KeyCode{
		xkEscape:       KeyEscape,
		xkTab:          KeyTab,
		xkShiftL:       KeyLShift,
		xkShiftR:       KeyRShift,
		xkControlL:     KeyLControl,
		xkControlR:     KeyRControl,
		xkMetaL:        KeyLOption,
		xkAltL:         KeyLOption,
		xkModeswitch:   KeyROption,
		xkMetaR:        KeyROption,
		xkAltR:         KeyROption,
		xkSuperL:       KeyLCommand,
		xkSuperR:       KeyRCommand,
		xkMenu:         KeyMenu,
		xkNumLock:      KeyNumLock,
		xkCapsLock:     KeyCapsLock,
		xkPrint:        KeyPrintScreen,
		xkScrollLock:   KeyScrollLock,
		xkPause:        KeyPause,
		xkDelete:       KeyDelete,
		xkBackSpace:    KeyBackspace,
		xkReturn:       KeyReturn,
		xkHome:         KeyHome,
		xkEnd:          KeyEnd,
		xkPageUp:       KeyPageUp,
		xkPageDown:     KeyPageDown,
		xkInsert:       KeyInsert,
		xkLeft:         KeyLeft,
		xkRight:        KeyRight,
		xkDown:         KeyDown,
		xkUp:           KeyUp,
		xkF1:           KeyF1,
		xkF2:           KeyF2,
		xkF3:           KeyF3,
		xkF4:           KeyF4,
		xkF5:           KeyF5,
		xkF6:           KeyF6,
		xkF7:           KeyF7,
		xkF8:           KeyF8,
		xkF9:           KeyF9,
		xkF10:          KeyF10,
		xkF11:          KeyF11,
		xkF12:          KeyF12,
		xkF13:          KeyF13,
		xkF14:          KeyF14,
		xkF15:          KeyF15,
		xkF16:          KeyF16,
		xkF17:          KeyF17,
		xkF18:          KeyF18,
		xkF19:          KeyF19,
		xkF20:          KeyF20,
		xkF21:          KeyF21,
		xkF22:          KeyF22,
		xkF23:          KeyF23,
		xkF24:          KeyF24,
		xkF25:          KeyF25,
		xkKPDivide:     KeyNumPadDivide,
		xkKPMultiply:   KeyNumPadMultiply,
		xkKPSubtract:   KeyNumPadSubtract,
		xkKPAdd:        KeyNumPadAdd,
		xkKPInsert:     KeyNumPad0,
		xkKPEnd:        KeyNumPad1,
		xkKPDown:       KeyNumPad2,
		xkKPPageDown:   KeyNumPad3,
		xkKPLeft:       KeyNumPad4,
		xkKPRight:      KeyNumPad6,
		xkKPHome:       KeyNumPad7,
		xkKPUp:         KeyNumPad8,
		xkKPPageUp:     KeyNumPad9,
		xkKPDelete:     KeyNumPadDecimal,
		xkKPEqual:      KeyNumPadEqual,
		xkKPEnter:      KeyNumPadEnter,
		xkA:            KeyA,
		xkB:            KeyB,
		xkC:            KeyC,
		xkD:            KeyD,
		xkE:            KeyE,
		xkF:            KeyF,
		xkG:            KeyG,
		xkH:            KeyH,
		xkI:            KeyI,
		xkJ:            KeyJ,
		xkK:            KeyK,
		xkL:            KeyL,
		xkM:            KeyM,
		xkN:            KeyN,
		xkO:            KeyO,
		xkP:            KeyP,
		xkQ:            KeyQ,
		xkR:            KeyR,
		xkS:            KeyS,
		xkT:            KeyT,
		xkU:            KeyU,
		xkV:            KeyV,
		xkW:            KeyW,
		xkX:            KeyX,
		xkY:            KeyY,
		xkZ:            KeyZ,
		xk1:            Key1,
		xk2:            Key2,
		xk3:            Key3,
		xk4:            Key4,
		xk5:            Key5,
		xk6:            Key6,
		xk7:            Key7,
		xk8:            Key8,
		xk9:            Key9,
		xk0:            Key0,
		xkSpace:        KeySpace,
		xkMinus:        KeyMinus,
		xkEqual:        KeyEqual,
		xkBracketLeft:  KeyOpenBracket,
		xkBracketRight: KeyCloseBracket,
		xkBackslash:    KeyBackslash,
		xkSemicolon:    KeySemiColon,
		xkApostrophe:   KeyApostrophe,
		xkGrave:        KeyBackQuote,
		xkComma:        KeyComma,
		xkPeriod:       KeyPeriod,
		xkSlash:        KeySlash,
		xkLess:         KeyWorld1,
	}
	km := x11Conn.GetKeyboardMapping()
	for i := x11Conn.MinKeyCode; i <= x11Conn.MaxKeyCode; i++ {
		pos := int(i-x11Conn.MinKeyCode) * int(km.KeySymsPerKeyCode)
		if km.KeySymsPerKeyCode > 1 {
			if code, ok := secondary[km.KeySyms[pos+1]]; ok {
				rawScanCodeToKeyCodeMap[int(i)] = code
				continue
			}
		}
		if code, ok := primary[km.KeySyms[pos]]; ok {
			rawScanCodeToKeyCodeMap[int(i)] = code
		}
	}
}
