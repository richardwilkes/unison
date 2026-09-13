// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package dbus

import (
	"encoding/binary"
	"math"
	"slices"
)

// The helpers below assemble expected byte sequences by hand so that the golden tests do not lean on any of the code
// they are checking. Every case spells out its padding explicitly, with the byte offsets in comments.

func u16At(v uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return b
}

func u32At(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

func u64At(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

// strAt returns the wire form of a string or object path: a 4 byte length, the bytes, then a NUL.
func strAt(s string) []byte {
	return append(append(u32At(uint32(len(s))), s...), 0)
}

// sigAt returns the wire form of a signature: a 1 byte length, the bytes, then a NUL.
func sigAt(s string) []byte {
	return append(append([]byte{byte(len(s))}, s...), 0)
}

// pad returns n bytes of alignment padding.
func pad(n int) []byte { return make([]byte, n) }

func concat(parts ...[]byte) []byte {
	var result []byte
	for _, one := range parts {
		result = append(result, one...)
	}
	return result
}

// goldenCase is one hand-encoded value sequence.
type goldenCase struct {
	name string
	sig  Signature
	in   []any // What is marshaled
	out  []any // What unmarshaling produces; nil means the same as in
	want []byte
}

const (
	atspiRoot     = "/org/a11y/atspi/accessible/root"
	busName       = "org.freedesktop.DBus"
	busPath       = "/org/freedesktop/DBus"
	toolkitName   = "unison"
	toolkitKey    = "toolkit"
	propertiesSig = "a{sv}"
	stringDictSig = "a{ss}"
)

func goldenCases() []goldenCase {
	return []goldenCase{
		{
			name: "fixed types and a string",
			sig:  "ybnqiuxtds",
			in: []any{
				byte(0x12), true, int16(-2), uint16(3), int32(-4), uint32(5), int64(-6), uint64(7), 1.5, "hi",
			},
			want: concat(
				[]byte{0x12},                 // 0     y
				pad(3),                       // 1-3   align the boolean to 4
				u32At(1),                     // 4-7   b
				u16At(0xFFFE),                // 8-9   n (-2)
				u16At(3),                     // 10-11 q
				u32At(0xFFFFFFFC),            // 12-15 i (-4)
				u32At(5),                     // 16-19 u
				pad(4),                       // 20-23 align the int64 to 8
				u64At(0xFFFFFFFFFFFFFFFA),    // 24-31 x (-6)
				u64At(7),                     // 32-39 t
				u64At(math.Float64bits(1.5)), // 40-47 d
				strAt("hi"),                  // 48-54 s
			),
		},
		{
			name: "object path and signature",
			sig:  "og",
			in:   []any{ObjectPath(atspiRoot), Signature(propertiesSig)},
			want: concat(
				strAt(atspiRoot),     // 0-35  o (4 + 31 + 1)
				sigAt(propertiesSig), // 36-42 g (1 + 5 + 1)
			),
		},
		{
			name: "at-spi event body",
			sig:  "siiv" + propertiesSig,
			in: []any{
				"focused", int32(1), int32(0),
				Variant{Sig: "i", Value: int32(0)},
				Dict{{Key: toolkitKey, Value: Variant{Sig: "s", Value: toolkitName}}},
			},
			want: concat(
				strAt("focused"),   // 0-11  s
				u32At(1),           // 12-15 i detail1
				u32At(0),           // 16-19 i detail2
				sigAt("i"),         // 20-22 v signature
				pad(1),             // 23    align the int32 to 4
				u32At(0),           // 24-27 v value
				u32At(27),          // 28-31 a{sv} length in bytes
				strAt(toolkitKey),  // 32-43 entry key (8 aligned already)
				sigAt("s"),         // 44-46 entry value signature
				pad(1),             // 47    align the string to 4
				strAt(toolkitName), // 48-58 entry value
			),
		},
		{
			name: "at-spi cache item",
			sig:  "a((so)(so)(so)iiassusau)",
			in: []any{[]any{Struct{
				ObjectRef{Name: "a", Path: "/a"},
				ObjectRef{Name: "b", Path: "/b"},
				ObjectRef{Name: "c", Path: "/c"},
				int32(1), int32(2),
				[]string{"x"},
				"y", uint32(3), "z",
				[]uint32{4},
			}}},
			want: concat(
				u32At(96),   // 0-3     array length in bytes
				pad(4),      // 4-7     align the first structure to 8
				strAt("a"),  // 8-13    (so) #1 name
				pad(2),      // 14-15   align the path to 4
				strAt("/a"), // 16-22   (so) #1 path
				pad(1),      // 23      align (so) #2 to 8
				strAt("b"),  // 24-29
				pad(2),      // 30-31
				strAt("/b"), // 32-38
				pad(1),      // 39      align (so) #3 to 8
				strAt("c"),  // 40-45
				pad(2),      // 46-47
				strAt("/c"), // 48-54
				pad(1),      // 55      align the int32 to 4
				u32At(1),    // 56-59   i
				u32At(2),    // 60-63   i
				u32At(6),    // 64-67   as length in bytes
				strAt("x"),  // 68-73   as[0]
				pad(2),      // 74-75   align the string to 4
				strAt("y"),  // 76-81   s
				pad(2),      // 82-83   align the uint32 to 4
				u32At(3),    // 84-87   u
				strAt("z"),  // 88-93   s
				pad(2),      // 94-95   align the array length to 4
				u32At(4),    // 96-99   au length in bytes
				u32At(4),    // 100-103 au[0]
			),
		},
		{
			name: "string dictionary",
			sig:  stringDictSig,
			in:   []any{Dict{{Key: "a", Value: "b"}, {Key: "c", Value: "d"}}},
			want: concat(
				u32At(30),  // 0-3   array length in bytes
				pad(4),     // 4-7   align the first entry to 8
				strAt("a"), // 8-13  entry #1 key
				pad(2),     // 14-15 align the value to 4
				strAt("b"), // 16-21 entry #1 value
				pad(2),     // 22-23 align entry #2 to 8
				strAt("c"), // 24-29
				pad(2),     // 30-31
				strAt("d"), // 32-37
			),
		},
		{
			name: "nested variants",
			sig:  "v",
			in:   []any{Variant{Sig: "v", Value: Variant{Sig: "s", Value: "hi"}}},
			want: concat(
				sigAt("v"),  // 0-2  outer variant signature
				sigAt("s"),  // 3-5  inner variant signature
				pad(2),      // 6-7  align the string to 4
				strAt("hi"), // 8-14 inner variant value
			),
		},
		{
			name: "empty arrays",
			sig:  "asay" + propertiesSig + "ata(ii)",
			in:   []any{[]string{}, []byte{}, Dict{}, []uint64{}, []any{}},
			out:  []any{[]string{}, []byte{}, Dict{}, []any{}, []any{}},
			want: concat(
				u32At(0), // 0-3   as length, elements are 4 aligned so no padding follows
				u32At(0), // 4-7   ay length, elements are 1 aligned so no padding follows
				u32At(0), // 8-11  a{sv} length
				pad(4),   // 12-15 dict entries are 8 aligned even when there are none
				u32At(0), // 16-19 at length
				pad(4),   // 20-23 uint64 elements are 8 aligned even when there are none
				u32At(0), // 24-27 a(ii) length
				pad(4),   // 28-31 structures are 8 aligned even when there are none
			),
		},
		{
			name: "array fast paths",
			sig:  "ayasaiauao",
			in: []any{
				[]byte{1, 2, 3},
				[]string{"ab"},
				[]int32{-1},
				[]uint32{2},
				[]ObjectPath{"/x"},
			},
			want: concat(
				u32At(3),          // 0-3   ay length
				[]byte{1, 2, 3},   // 4-6
				pad(1),            // 7     align the as length to 4
				u32At(7),          // 8-11  as length
				strAt("ab"),       // 12-18
				pad(1),            // 19    align the ai length to 4
				u32At(4),          // 20-23 ai length
				u32At(0xFFFFFFFF), // 24-27 ai[0] (-1)
				u32At(4),          // 28-31 au length
				u32At(2),          // 32-35 au[0]
				u32At(7),          // 36-39 ao length
				strAt("/x"),       // 40-46
			),
		},
		{
			name: "object references",
			sig:  "(so)a(so)",
			in: []any{
				ObjectRef{Name: "n", Path: "/p"},
				[]ObjectRef{{Name: "q", Path: "/r"}},
			},
			want: concat(
				strAt("n"),  // 0-5   (so) name
				pad(2),      // 6-7   align the path to 4
				strAt("/p"), // 8-14  (so) path
				pad(1),      // 15    align the array length to 4
				u32At(15),   // 16-19 a(so) length in bytes
				pad(4),      // 20-23 align the first structure to 8
				strAt("q"),  // 24-29
				pad(2),      // 30-31
				strAt("/r"), // 32-38
			),
		},
	}
}

// swapper rewrites the little-endian encoding of a value sequence as the big-endian one, in place. It walks the
// signature itself rather than leaning on the decoder, so that the big-endian tests do not check the code under test
// against itself. Only values that [Marshal] produced are handed to it, so anything malformed is a bug in the test and
// is reported by panicking.
type swapper struct {
	data []byte
	pos  int
}

// swapEndianness returns a copy of the marshaled form of sig with every multi-byte field written the other way round.
func swapEndianness(sig Signature, data []byte) []byte {
	s := &swapper{data: slices.Clone(data)}
	types, err := sig.Types()
	if err != nil {
		panic(err)
	}
	for _, one := range types {
		s.value(one)
	}
	if s.pos != len(s.data) {
		panic("the swapper did not consume everything")
	}
	return s.data
}

// align skips the padding that precedes a value of the given alignment, which is already NUL and so needs no swapping.
func (s *swapper) align(n int) {
	for s.pos%n != 0 {
		s.pos++
	}
}

// fixed swaps the n byte value at the current position, having first skipped its padding.
func (s *swapper) fixed(n int) {
	s.align(n)
	slices.Reverse(s.data[s.pos : s.pos+n])
	s.pos += n
}

// length swaps the 4 byte length at the current position and returns the value it had.
func (s *swapper) length() int {
	s.align(4)
	n := binary.LittleEndian.Uint32(s.data[s.pos:])
	binary.BigEndian.PutUint32(s.data[s.pos:], n)
	s.pos += 4
	return int(n)
}

// value swaps one value, whose type is the single complete type sig.
func (s *swapper) value(sig Signature) {
	switch sig[0] {
	case 'y':
		s.pos++
	case 'n', 'q':
		s.fixed(2)
	case 'b', 'i', 'u':
		s.fixed(4)
	case 'x', 't', 'd':
		s.fixed(8)
	case 's', 'o':
		s.pos += s.length() + 1 // The bytes and the NUL that follows them are untouched
	case 'g':
		s.pos += int(s.data[s.pos]) + 2 // The length is a single byte, so only the NUL is added to it
	case 'v':
		n := int(s.data[s.pos])
		nested := Signature(s.data[s.pos+1 : s.pos+1+n])
		s.pos += n + 2
		s.value(nested)
	case 'a':
		s.array(sig[1:])
	case '(':
		s.structure(sig)
	default:
		panic("cannot swap the type " + string(sig))
	}
}

// array swaps every element of an array whose element type is elem, which is a dict entry when it starts with a brace.
func (s *swapper) array(elem Signature) {
	n := s.length()
	s.align(alignmentOf(elem[0]))
	end := s.pos + n
	for s.pos < end {
		if elem[0] == '{' {
			s.structure(elem)
			continue
		}
		s.value(elem)
	}
	if s.pos != end {
		panic("an array element overran the end of its array")
	}
}

// structure swaps every field of a structure or dict entry, whose type sig includes its parentheses or braces.
func (s *swapper) structure(sig Signature) {
	types, err := sig[1 : len(sig)-1].Types()
	if err != nil {
		panic(err)
	}
	s.align(8)
	for _, one := range types {
		s.value(one)
	}
}

// toBigEndian returns the big-endian form of a complete message that was encoded little-endian. bodySig is the
// signature of the body, which may be empty when there is none. The serial and the header field array are swapped
// together, starting from an offset that is a multiple of 8, so that the alignment inside the array is measured from
// the same place the encoder measured it from.
func toBigEndian(data []byte, bodySig Signature) []byte {
	result := slices.Clone(data)
	fieldsLength := int(binary.LittleEndian.Uint32(result[12:fixedHeaderSize]))
	copy(result[8:], swapEndianness("ua(yv)", result[8:fixedHeaderSize+fieldsLength]))
	if bodySig != "" {
		bodyStart := fixedHeaderSize + (fieldsLength+7)&^7
		copy(result[bodyStart:], swapEndianness(bodySig, result[bodyStart:]))
	}
	slices.Reverse(result[4:8]) // The body length
	result[0] = 'B'
	return result
}

// helloMessage returns the canonical Hello method call that every client sends to the bus as its first message, along
// with the 128 bytes libdbus produces for it.
func helloMessage() (m *Message, data []byte) {
	m = NewMethodCall(busName, busPath, busName, "Hello")
	m.Serial = 1
	return m, concat(
		[]byte{'l', 1, 0, 1}, // 0-3     little-endian, method call, no flags, protocol version 1
		u32At(0),             // 4-7     body length
		u32At(1),             // 8-11    serial
		u32At(110),           // 12-15   header field array length in bytes
		[]byte{1},            // 16      PATH
		sigAt("o"),           // 17-19
		strAt(busPath),       // 20-45
		pad(2),               // 46-47   align the next field to 8
		[]byte{6},            // 48      DESTINATION
		sigAt("s"),           // 49-51
		strAt(busName),       // 52-76
		pad(3),               // 77-79
		[]byte{2},            // 80      INTERFACE
		sigAt("s"),           // 81-83
		strAt(busName),       // 84-108
		pad(3),               // 109-111
		[]byte{3},            // 112     MEMBER
		sigAt("s"),           // 113-115
		strAt("Hello"),       // 116-125
		pad(2),               // 126-127 the header is padded to 8 before the body
	)
}

// helloMessageBigEndian returns the same Hello method call that [helloMessage] does, in the encoding a peer on a
// big-endian machine would send. It is assembled by hand, byte for byte, rather than by swapping the little-endian
// bytes, so that it also checks the swapper that [toBigEndian] uses for the cases that are too large to spell out.
func helloMessageBigEndian() []byte {
	u32 := func(v uint32) []byte {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, v)
		return b
	}
	str := func(s string) []byte { return append(append(u32(uint32(len(s))), s...), 0) }
	return concat(
		[]byte{'B', 1, 0, 1}, // 0-3     big-endian, method call, no flags, protocol version 1
		u32(0),               // 4-7     body length
		u32(1),               // 8-11    serial
		u32(110),             // 12-15   header field array length in bytes
		[]byte{1},            // 16      PATH
		sigAt("o"),           // 17-19
		str(busPath),         // 20-45
		pad(2),               // 46-47   align the next field to 8
		[]byte{6},            // 48      DESTINATION
		sigAt("s"),           // 49-51
		str(busName),         // 52-76
		pad(3),               // 77-79
		[]byte{2},            // 80      INTERFACE
		sigAt("s"),           // 81-83
		str(busName),         // 84-108
		pad(3),               // 109-111
		[]byte{3},            // 112     MEMBER
		sigAt("s"),           // 113-115
		str("Hello"),         // 116-125
		pad(2),               // 126-127 the header is padded to 8 before the body
	)
}
