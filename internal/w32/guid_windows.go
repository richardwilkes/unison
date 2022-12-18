// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

const hexTable = "0123456789ABCDEF"

var NullGUID GUID

// GUID holds a Windows universal ID.
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// NewGUID creates a GUID from a string. The string may be in one of these formats:
//
//	{XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}
//	XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX
//	XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
func NewGUID(guid string) GUID {
	d := []byte(guid)
	var d1, d2, d3, d4a, d4b []byte
	switch len(d) {
	case 38:
		if d[0] != '{' || d[37] != '}' {
			return NullGUID
		}
		d = d[1:37]
		fallthrough
	case 36:
		if d[8] != '-' || d[13] != '-' || d[18] != '-' || d[23] != '-' {
			return NullGUID
		}
		d1 = d[0:8]
		d2 = d[9:13]
		d3 = d[14:18]
		d4a = d[19:23]
		d4b = d[24:36]
	case 32:
		d1 = d[0:8]
		d2 = d[8:12]
		d3 = d[12:16]
		d4a = d[16:20]
		d4b = d[20:32]
	default:
		return NullGUID
	}
	var g GUID
	var ok1, ok2, ok3, ok4 bool
	g.Data1, ok1 = decodeHexUint32(d1)
	g.Data2, ok2 = decodeHexUint16(d2)
	g.Data3, ok3 = decodeHexUint16(d3)
	g.Data4, ok4 = decodeHexByte64(d4a, d4b)
	if ok1 && ok2 && ok3 && ok4 {
		return g
	}
	return NullGUID
}

// String returns the string representation of the GUID in its canonical format of
// {XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}.
func (guid GUID) String() string {
	var c [38]byte
	c[0] = '{'
	putUint32Hex(c[1:9], guid.Data1)
	c[9] = '-'
	putUint16Hex(c[10:14], guid.Data2)
	c[14] = '-'
	putUint16Hex(c[15:19], guid.Data3)
	c[19] = '-'
	putByteHex(c[20:24], guid.Data4[0:2])
	c[24] = '-'
	putByteHex(c[25:37], guid.Data4[2:8])
	c[37] = '}'
	return string(c[:])
}

func decodeHexUint32(src []byte) (uint32, bool) {
	b1, ok1 := decodeHexByte(src[0], src[1])
	b2, ok2 := decodeHexByte(src[2], src[3])
	b3, ok3 := decodeHexByte(src[4], src[5])
	b4, ok4 := decodeHexByte(src[6], src[7])
	return (uint32(b1) << 24) | (uint32(b2) << 16) | (uint32(b3) << 8) | uint32(b4), ok1 && ok2 && ok3 && ok4
}

func decodeHexUint16(src []byte) (uint16, bool) {
	b1, ok1 := decodeHexByte(src[0], src[1])
	b2, ok2 := decodeHexByte(src[2], src[3])
	return (uint16(b1) << 8) | uint16(b2), ok1 && ok2
}

func decodeHexByte64(s1 []byte, s2 []byte) (value [8]byte, ok bool) {
	var ok1, ok2, ok3, ok4, ok5, ok6, ok7, ok8 bool
	value[0], ok1 = decodeHexByte(s1[0], s1[1])
	value[1], ok2 = decodeHexByte(s1[2], s1[3])
	value[2], ok3 = decodeHexByte(s2[0], s2[1])
	value[3], ok4 = decodeHexByte(s2[2], s2[3])
	value[4], ok5 = decodeHexByte(s2[4], s2[5])
	value[5], ok6 = decodeHexByte(s2[6], s2[7])
	value[6], ok7 = decodeHexByte(s2[8], s2[9])
	value[7], ok8 = decodeHexByte(s2[10], s2[11])
	return value, ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7 && ok8
}

func decodeHexByte(c1, c2 byte) (byte, bool) {
	n1, ok1 := decodeHexChar(c1)
	n2, ok2 := decodeHexChar(c2)
	return (n1 << 4) | n2, ok1 && ok2
}

func decodeHexChar(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func putUint32Hex(b []byte, v uint32) {
	b[0] = hexTable[byte(v>>24)>>4]
	b[1] = hexTable[byte(v>>24)&0x0f]
	b[2] = hexTable[byte(v>>16)>>4]
	b[3] = hexTable[byte(v>>16)&0x0f]
	b[4] = hexTable[byte(v>>8)>>4]
	b[5] = hexTable[byte(v>>8)&0x0f]
	b[6] = hexTable[byte(v)>>4]
	b[7] = hexTable[byte(v)&0x0f]
}

func putUint16Hex(b []byte, v uint16) {
	b[0] = hexTable[byte(v>>8)>>4]
	b[1] = hexTable[byte(v>>8)&0x0f]
	b[2] = hexTable[byte(v)>>4]
	b[3] = hexTable[byte(v)&0x0f]
}

func putByteHex(dst, src []byte) {
	for i := 0; i < len(src); i++ {
		dst[i*2] = hexTable[src[i]>>4]
		dst[i*2+1] = hexTable[src[i]&0x0f]
	}
}
