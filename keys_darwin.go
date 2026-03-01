package unison

import (
	"github.com/richardwilkes/unison/internal/mac"
)

func apiFillKeyCodes() {
	rawScanCodeToKeyCodeMap[0x00] = KeyA
	rawScanCodeToKeyCodeMap[0x01] = KeyS
	rawScanCodeToKeyCodeMap[0x02] = KeyD
	rawScanCodeToKeyCodeMap[0x03] = KeyF
	rawScanCodeToKeyCodeMap[0x04] = KeyH
	rawScanCodeToKeyCodeMap[0x05] = KeyG
	rawScanCodeToKeyCodeMap[0x06] = KeyZ
	rawScanCodeToKeyCodeMap[0x07] = KeyX
	rawScanCodeToKeyCodeMap[0x08] = KeyC
	rawScanCodeToKeyCodeMap[0x09] = KeyV
	rawScanCodeToKeyCodeMap[0x0A] = KeyWorld1
	rawScanCodeToKeyCodeMap[0x0B] = KeyB
	rawScanCodeToKeyCodeMap[0x0C] = KeyQ
	rawScanCodeToKeyCodeMap[0x0D] = KeyW
	rawScanCodeToKeyCodeMap[0x0E] = KeyE
	rawScanCodeToKeyCodeMap[0x0F] = KeyR
	rawScanCodeToKeyCodeMap[0x10] = KeyY
	rawScanCodeToKeyCodeMap[0x11] = KeyT
	rawScanCodeToKeyCodeMap[0x12] = Key1
	rawScanCodeToKeyCodeMap[0x13] = Key2
	rawScanCodeToKeyCodeMap[0x14] = Key3
	rawScanCodeToKeyCodeMap[0x15] = Key4
	rawScanCodeToKeyCodeMap[0x16] = Key6
	rawScanCodeToKeyCodeMap[0x17] = Key5
	rawScanCodeToKeyCodeMap[0x18] = KeyEqual
	rawScanCodeToKeyCodeMap[0x19] = Key9
	rawScanCodeToKeyCodeMap[0x1A] = Key7
	rawScanCodeToKeyCodeMap[0x1B] = KeyMinus
	rawScanCodeToKeyCodeMap[0x1C] = Key8
	rawScanCodeToKeyCodeMap[0x1D] = Key0
	rawScanCodeToKeyCodeMap[0x1E] = KeyCloseBracket
	rawScanCodeToKeyCodeMap[0x1F] = KeyO
	rawScanCodeToKeyCodeMap[0x20] = KeyU
	rawScanCodeToKeyCodeMap[0x21] = KeyOpenBracket
	rawScanCodeToKeyCodeMap[0x22] = KeyI
	rawScanCodeToKeyCodeMap[0x23] = KeyP
	rawScanCodeToKeyCodeMap[0x24] = KeyReturn
	rawScanCodeToKeyCodeMap[0x25] = KeyL
	rawScanCodeToKeyCodeMap[0x26] = KeyJ
	rawScanCodeToKeyCodeMap[0x27] = KeyApostrophe
	rawScanCodeToKeyCodeMap[0x28] = KeyK
	rawScanCodeToKeyCodeMap[0x29] = KeySemiColon
	rawScanCodeToKeyCodeMap[0x2A] = KeyBackslash
	rawScanCodeToKeyCodeMap[0x2B] = KeyComma
	rawScanCodeToKeyCodeMap[0x2C] = KeySlash
	rawScanCodeToKeyCodeMap[0x2D] = KeyN
	rawScanCodeToKeyCodeMap[0x2E] = KeyM
	rawScanCodeToKeyCodeMap[0x2F] = KeyPeriod
	rawScanCodeToKeyCodeMap[0x30] = KeyTab
	rawScanCodeToKeyCodeMap[0x31] = KeySpace
	rawScanCodeToKeyCodeMap[0x32] = KeyBackQuote
	rawScanCodeToKeyCodeMap[0x33] = KeyBackspace
	rawScanCodeToKeyCodeMap[0x35] = KeyEscape
	rawScanCodeToKeyCodeMap[0x36] = KeyRCommand
	rawScanCodeToKeyCodeMap[0x37] = KeyLCommand
	rawScanCodeToKeyCodeMap[0x38] = KeyLShift
	rawScanCodeToKeyCodeMap[0x39] = KeyCapsLock
	rawScanCodeToKeyCodeMap[0x3A] = KeyLOption
	rawScanCodeToKeyCodeMap[0x3B] = KeyLControl
	rawScanCodeToKeyCodeMap[0x3C] = KeyRShift
	rawScanCodeToKeyCodeMap[0x3D] = KeyROption
	rawScanCodeToKeyCodeMap[0x3E] = KeyRControl
	rawScanCodeToKeyCodeMap[0x40] = KeyF17
	rawScanCodeToKeyCodeMap[0x41] = KeyNumPadDecimal
	rawScanCodeToKeyCodeMap[0x43] = KeyNumPadMultiply
	rawScanCodeToKeyCodeMap[0x45] = KeyNumPadAdd
	rawScanCodeToKeyCodeMap[0x47] = KeyNumLock
	rawScanCodeToKeyCodeMap[0x4B] = KeyNumPadDivide
	rawScanCodeToKeyCodeMap[0x4C] = KeyNumPadEnter
	rawScanCodeToKeyCodeMap[0x4E] = KeyNumPadSubtract
	rawScanCodeToKeyCodeMap[0x4F] = KeyF18
	rawScanCodeToKeyCodeMap[0x50] = KeyF19
	rawScanCodeToKeyCodeMap[0x51] = KeyNumPadEqual
	rawScanCodeToKeyCodeMap[0x52] = KeyNumPad0
	rawScanCodeToKeyCodeMap[0x53] = KeyNumPad1
	rawScanCodeToKeyCodeMap[0x54] = KeyNumPad2
	rawScanCodeToKeyCodeMap[0x55] = KeyNumPad3
	rawScanCodeToKeyCodeMap[0x56] = KeyNumPad4
	rawScanCodeToKeyCodeMap[0x57] = KeyNumPad5
	rawScanCodeToKeyCodeMap[0x58] = KeyNumPad6
	rawScanCodeToKeyCodeMap[0x59] = KeyNumPad7
	rawScanCodeToKeyCodeMap[0x5A] = KeyF20
	rawScanCodeToKeyCodeMap[0x5B] = KeyNumPad8
	rawScanCodeToKeyCodeMap[0x5C] = KeyNumPad9
	rawScanCodeToKeyCodeMap[0x60] = KeyF5
	rawScanCodeToKeyCodeMap[0x61] = KeyF6
	rawScanCodeToKeyCodeMap[0x62] = KeyF7
	rawScanCodeToKeyCodeMap[0x63] = KeyF3
	rawScanCodeToKeyCodeMap[0x64] = KeyF8
	rawScanCodeToKeyCodeMap[0x65] = KeyF9
	rawScanCodeToKeyCodeMap[0x67] = KeyF11
	rawScanCodeToKeyCodeMap[0x69] = KeyPrintScreen
	rawScanCodeToKeyCodeMap[0x6A] = KeyF16
	rawScanCodeToKeyCodeMap[0x6B] = KeyF14
	rawScanCodeToKeyCodeMap[0x6D] = KeyF10
	rawScanCodeToKeyCodeMap[0x6E] = KeyMenu
	rawScanCodeToKeyCodeMap[0x6F] = KeyF12
	rawScanCodeToKeyCodeMap[0x71] = KeyF15
	rawScanCodeToKeyCodeMap[0x72] = KeyInsert
	rawScanCodeToKeyCodeMap[0x73] = KeyHome
	rawScanCodeToKeyCodeMap[0x74] = KeyPageUp
	rawScanCodeToKeyCodeMap[0x75] = KeyDelete
	rawScanCodeToKeyCodeMap[0x76] = KeyF4
	rawScanCodeToKeyCodeMap[0x77] = KeyEnd
	rawScanCodeToKeyCodeMap[0x78] = KeyF2
	rawScanCodeToKeyCodeMap[0x79] = KeyPageDown
	rawScanCodeToKeyCodeMap[0x7A] = KeyF1
	rawScanCodeToKeyCodeMap[0x7B] = KeyLeft
	rawScanCodeToKeyCodeMap[0x7C] = KeyRight
	rawScanCodeToKeyCodeMap[0x7D] = KeyDown
	rawScanCodeToKeyCodeMap[0x7E] = KeyUp
}

func translateModifiers(flags mac.EventModifierFlags) Modifiers {
	var mods Modifiers
	if flags&mac.EventModifierFlagShift != 0 {
		mods |= ShiftModifier
	}
	if flags&mac.EventModifierFlagControl != 0 {
		mods |= ControlModifier
	}
	if flags&mac.EventModifierFlagOption != 0 {
		mods |= OptionModifier
	}
	if flags&mac.EventModifierFlagCommand != 0 {
		mods |= CommandModifier
	}
	if flags&mac.EventModifierFlagCapsLock != 0 {
		mods |= CapsLockModifier
	}
	return mods
}
