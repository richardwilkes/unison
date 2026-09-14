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
	"math"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

func TestMarshalGolden(t *testing.T) {
	t.Parallel()
	for _, one := range goldenCases() {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			data, err := Marshal(one.sig, one.in...)
			c.NoError(err)
			c.Equal(one.want, data)
		})
	}
}

func TestUnmarshalGolden(t *testing.T) {
	t.Parallel()
	for _, one := range goldenCases() {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			values, err := Unmarshal(one.sig, one.want)
			c.NoError(err)
			want := one.out
			if want == nil {
				want = one.in
			}
			c.Equal(want, values)
		})
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	for i, one := range []struct {
		sig    Signature
		values []any
	}{
		{sig: "y", values: []any{byte(255)}},
		{sig: "b", values: []any{false}},
		{sig: "n", values: []any{int16(math.MinInt16)}},
		{sig: "q", values: []any{uint16(math.MaxUint16)}},
		{sig: "i", values: []any{int32(math.MinInt32)}},
		{sig: "u", values: []any{uint32(math.MaxUint32)}},
		{sig: "x", values: []any{int64(math.MinInt64)}},
		{sig: "t", values: []any{uint64(math.MaxUint64)}},
		{sig: "d", values: []any{math.Inf(-1)}},
		{sig: "d", values: []any{math.Copysign(0, -1)}},
		{sig: "s", values: []any{""}},
		{sig: "s", values: []any{"a string with a ☃ in it"}},
		{sig: "o", values: []any{ObjectPath("/")}},
		{sig: "o", values: []any{ObjectPath(atspiRoot)}},
		{sig: "g", values: []any{Signature("")}},
		{sig: "g", values: []any{Signature("a(ii)a{sv}")}},
		{sig: "v", values: []any{Variant{Sig: "y", Value: byte(1)}}},
		{sig: "v", values: []any{Variant{Sig: "ai", Value: []int32{1, 2}}}},
		{sig: "ay", values: []any{[]byte{0, 1, 2, 3, 4}}},
		{sig: "as", values: []any{[]string{"alpha", "beta", "gamma"}}},
		{sig: "ai", values: []any{[]int32{-1, 0, 1}}},
		{sig: "au", values: []any{[]uint32{1, 2, 3}}},
		{sig: "ao", values: []any{[]ObjectPath{"/a", "/b"}}},
		{sig: "a(so)", values: []any{[]ObjectRef{{Name: "n", Path: "/p"}}}},
		{sig: "aay", values: []any{[]any{[]byte{1}, []byte{2, 3}}}},
		{sig: "aai", values: []any{[]any{[]int32{1}, []int32{}}}},
		{sig: stringDictSig, values: []any{Dict{{Key: "k", Value: "v"}}}},
		{sig: "a{sv}", values: []any{Dict{{Key: "k", Value: Variant{Sig: "b", Value: true}}}}},
		{sig: "a{ya{ss}}", values: []any{Dict{{Key: byte(1), Value: Dict{{Key: "a", Value: "b"}}}}}},
		{sig: "(ii)", values: []any{Struct{int32(1), int32(2)}}},
		{sig: "(y(yy))", values: []any{Struct{byte(1), Struct{byte(2), byte(3)}}}},
		{sig: "((so)v)", values: []any{Struct{ObjectRef{Name: "n", Path: "/p"}, Variant{Sig: "s", Value: "x"}}}},
		{sig: "a(sa{sv})", values: []any{[]any{Struct{"n", Dict{{Key: "k", Value: Variant{Sig: "i", Value: int32(7)}}}}}}},
		{sig: "sv", values: []any{"", Variant{Sig: stringDictSig, Value: Dict{}}}},
		{sig: "", values: nil},
	} {
		t.Run(strings.ReplaceAll(string(one.sig), "/", "_"), func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			data, err := Marshal(one.sig, one.values...)
			c.NoError(err, i)
			values, err := Unmarshal(one.sig, data)
			c.NoError(err, i)
			if one.values == nil {
				c.Equal(0, len(values), i)
			} else {
				c.Equal(one.values, values, i)
			}
			again, err := Marshal(one.sig, values...)
			c.NoError(err, i)
			c.Equal(data, again, i)
		})
	}
}

func TestMarshalAcceptsMapsSortedByKey(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	fromDict, err := Marshal(stringDictSig, Dict{{Key: "a", Value: "b"}, {Key: "c", Value: "d"}})
	c.NoError(err)
	fromMap, err := Marshal(stringDictSig, map[string]string{"c": "d", "a": "b"})
	c.NoError(err)
	c.Equal(fromDict, fromMap)
	fromIntDict, err := Marshal("a{yv}", Dict{
		{Key: byte(1), Value: Variant{Sig: "s", Value: "one"}},
		{Key: byte(2), Value: Variant{Sig: "s", Value: "two"}},
	})
	c.NoError(err)
	fromIntMap, err := Marshal("a{yv}", map[byte]any{2: "two", 1: "one"})
	c.NoError(err)
	c.Equal(fromIntDict, fromIntMap)
}

func TestMarshalSortsMapKeysHeldInAnInterface(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	// Every key of a map whose key type is an interface has the same reflect kind, so comparing them without unwrapping
	// what they hold left the map unsorted and made the same map encode differently from one call to the next.
	want, err := Marshal(stringDictSig, Dict{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}, {Key: "c", Value: "3"}})
	c.NoError(err)
	sorted := map[any]string{"c": "3", "a": "1", "b": "2"}
	for i := range 50 {
		var data []byte
		data, err = Marshal(stringDictSig, sorted)
		c.NoError(err, i)
		c.Equal(want, data, i)
	}
	// Keys of different types cannot be ordered against one another at all, so the comparison has to keep its hands off
	// them rather than asking a string for its integer value; the mixture is then refused by the encoder.
	_, err = Marshal(stringDictSig, map[any]string{"a": "1", int32(2): "2"})
	c.HasError(err)
}

func TestMarshalAcceptsLooseTypes(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	strict, err := Marshal("iud", int32(-3), uint32(4), 5.0)
	c.NoError(err)
	loose, err := Marshal("iud", -3, 4, 5)
	c.NoError(err)
	c.Equal(strict, loose)
	fromStruct, err := Marshal("(ss)", Struct{"a", "b"})
	c.NoError(err)
	fromSlice, err := Marshal("(ss)", []any{"a", "b"})
	c.NoError(err)
	c.Equal(fromStruct, fromSlice)
	fromArray, err := Marshal("as", Array{Elem: "s", Values: []any{"a"}})
	c.NoError(err)
	fromStrings, err := Marshal("as", []string{"a"})
	c.NoError(err)
	c.Equal(fromStrings, fromArray)
	derived, err := Marshal("v", "derived")
	c.NoError(err)
	explicit, err := Marshal("v", Variant{Sig: "s", Value: "derived"})
	c.NoError(err)
	c.Equal(explicit, derived)
}

func TestMarshalErrors(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		name   string
		sig    Signature
		values []any
	}{
		{name: "wrong count", sig: "ss", values: []any{"only one"}},
		{name: "wrong type", sig: "s", values: []any{42}},
		{name: "bool from int", sig: "b", values: []any{1}},
		{name: "byte overflow", sig: "y", values: []any{256}},
		{name: "int16 overflow", sig: "n", values: []any{32768}},
		{name: "uint32 from negative", sig: "u", values: []any{-1}},
		{name: "int32 overflow", sig: "i", values: []any{int64(math.MaxInt32) + 1}},
		{name: "invalid utf8", sig: "s", values: []any{"\xff"}},
		{name: "embedded nul", sig: "s", values: []any{"a\x00b"}},
		{name: "relative path", sig: "o", values: []any{ObjectPath("org/freedesktop")}},
		{name: "empty path element", sig: "o", values: []any{ObjectPath("/a//b")}},
		{name: "invalid path character", sig: "o", values: []any{ObjectPath("/a-b")}},
		{name: "invalid signature value", sig: "g", values: []any{Signature("a")}},
		{name: "unsupported fd", sig: "h", values: []any{int32(0)}},
		{name: "struct field count", sig: "(ss)", values: []any{Struct{"a"}}},
		{name: "array of non-slice", sig: "as", values: []any{"not a slice"}},
		{name: "dict from non-map", sig: stringDictSig, values: []any{"not a map"}},
		{name: "dict entry outside array", sig: stringEntrySig, values: []any{DictEntry{Key: "a", Value: "b"}}},
		{name: "empty struct", sig: "()", values: []any{Struct{}}},
		{name: "variant of undecidable value", sig: "v", values: []any{1}},
		{name: "variant with bad signature", sig: "v", values: []any{Variant{Sig: "ss", Value: "x"}}},
		{name: "array element type mismatch", sig: "as", values: []any{Array{Elem: "i", Values: []any{int32(1)}}}},
		{name: "object reference as another structure", sig: "(ss)", values: []any{ObjectRef{Name: "n", Path: "/p"}}},
		{name: "dict entry as a three field structure", sig: "(sss)", values: []any{DictEntry{Key: "a", Value: "b"}}},
		{name: "structure from a map", sig: "(ss)", values: []any{map[string]string{}}},
		{name: "bad value inside an array", sig: "as", values: []any{[]any{1}}},
		{name: "bad value inside a struct", sig: "(s)", values: []any{Struct{1}}},
		{name: "bad key inside a dict", sig: stringDictSig, values: []any{Dict{{Key: 1, Value: "a"}}}},
		{name: "bad value inside a map", sig: stringDictSig, values: []any{map[string]any{"a": 1}}},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			_, err := Marshal(one.sig, one.values...)
			c.HasError(err)
		})
	}
}

func TestMarshalDepthLimit(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	sig := Signature(strings.Repeat("a", MaxDepth-1) + "y")
	value := any([]byte{1})
	for range MaxDepth - 2 {
		value = []any{value}
	}
	_, err := Marshal(sig, value)
	c.NoError(err)
	c.HasError(Signature(strings.Repeat("a", MaxDepth+1) + "y").Validate())
	deep := any(Variant{Sig: "y", Value: byte(1)})
	for range MaxDepth {
		deep = Variant{Sig: "v", Value: deep}
	}
	_, err = Marshal("v", deep)
	c.HasError(err)
}

func TestMarshalArraySizeLimit(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	_, err := Marshal("ay", make([]byte, MaxArraySize+1))
	c.HasError(err)
	// A string is bounded by the same limit, since the decoder rejects one over it: marshaling a longer one produced a
	// body that this package could not itself decode and that no peer would accept.
	oversized := strings.Repeat("a", MaxArraySize+1)
	_, err = Marshal("s", oversized)
	c.HasError(err)
	_, err = Marshal("o", ObjectPath("/"+oversized))
	c.HasError(err)
	_, err = Unmarshal("s", concat(u32At(MaxArraySize+1), []byte{'a', 0}))
	c.HasError(err)
}

// maxStringErrorLength is what an error about the content of a string may cost, in bytes. The string is abbreviated to
// 64 bytes, and %q may expand each of those to four characters, so everything beyond a few hundred bytes is the error
// message repeating a peer's own bytes back to it.
const maxStringErrorLength = 512

func TestInvalidStringErrorsAreAbbreviated(t *testing.T) {
	t.Parallel()
	const size = 1 << 20
	for _, one := range []struct {
		name string
		text string
	}{
		{name: "invalid UTF-8", text: strings.Repeat("\xff", size)},
		{name: "embedded NUL", text: strings.Repeat("a", size-1) + "\x00"},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			// A string that a message body or a header field declares may be as large as MaxArraySize, and %q expands
			// each invalid byte to four characters, so naming the whole of one amplified a single message into
			// hundreds of megabytes of error text that a connection then held for as long as it lasted.
			_, err := Marshal("s", one.text)
			c.HasError(err)
			c.True(len(err.Error()) < maxStringErrorLength, "a %d byte string produced a %d byte error", len(one.text),
				len(err.Error()))
			_, err = Unmarshal("s", strAt(one.text))
			c.HasError(err)
			c.True(len(err.Error()) < maxStringErrorLength, "a %d byte string produced a %d byte error", len(one.text),
				len(err.Error()))
		})
	}
}

func TestDecodingAnInvalidStringDoesNotCopyIt(t *testing.T) { // Not parallel: it measures allocation
	c := check.New(t)
	// The content of a string off the wire is checked before it is copied out of the buffer it arrived in, so a string
	// that a peer had no business sending costs nothing but the check.
	data := strAt(strings.Repeat("\xff", 1<<20))
	var err error
	allocated := bytesAllocated(func() { _, err = Unmarshal("s", data) })
	c.HasError(err)
	// The bound is a fraction of the string itself rather than nothing at all, since building the error message does
	// allocate a few kilobytes; what it rules out is the megabyte copy that reaching the check required.
	c.True(allocated < uint64(len(data))/16, "a %d byte invalid string allocated %d bytes", len(data), allocated)
}

// The named types below exercise the reflection-based fallbacks, which is how the AT-SPI layer's own enumerated types
// will arrive here.
type (
	myLevel   int32
	myFlags   uint16
	myText    string
	myEnabled bool
)

func TestMarshalNamedTypes(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		named any
		plain any
		sig   Signature
	}{
		{sig: "b", named: myEnabled(true), plain: true},
		{sig: "i", named: myLevel(-5), plain: int32(-5)},
		{sig: "q", named: myFlags(7), plain: uint16(7)},
		{sig: "s", named: myText("x"), plain: "x"},
		{sig: "o", named: myText("/x"), plain: ObjectPath("/x")},
		{sig: "g", named: myText("ii"), plain: Signature("ii")},
		{sig: "d", named: float32(1.5), plain: 1.5},
		{sig: "x", named: uint64(5), plain: int64(5)},
		{sig: "t", named: 5, plain: uint64(5)},
		{sig: "n", named: myLevel(-1), plain: int16(-1)},
		{sig: "as", named: []myText{"a"}, plain: []string{"a"}},
		{sig: "av", named: []myText{"a"}, plain: []any{Variant{Sig: "s", Value: "a"}}},
	} {
		named, err := Marshal(one.sig, one.named)
		c.NoError(err, one.sig)
		plain, err := Marshal(one.sig, one.plain)
		c.NoError(err, one.sig)
		c.Equal(plain, named, one.sig)
	}
	sig, err := SignatureOf(myLevel(1), myFlags(2), myText("3"), []myText{"4"}, myEnabled(false))
	c.NoError(err)
	c.Equal(Signature("iqsasb"), sig)
	// Whatever SignatureOf derives a signature for must also marshal with it, which a named bool did not always do.
	_, err = Marshal(sig, myLevel(1), myFlags(2), myText("3"), []myText{"4"}, myEnabled(false))
	c.NoError(err)
	_, err = Marshal("b", "not a boolean")
	c.HasError(err)
	_, err = Marshal("y", myFlags(300))
	c.HasError(err)
	_, err = Marshal("d", "not a number")
	c.HasError(err)
	_, err = Marshal("o", myLevel(1))
	c.HasError(err)
	_, err = Marshal("g", myLevel(1))
	c.HasError(err)
}

func TestMarshalDictEntryAsAStructure(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	want, err := Marshal(objectRefSig, ObjectRef{Name: "n", Path: "/p"})
	c.NoError(err)
	data, err := Marshal(objectRefSig, DictEntry{Key: "n", Value: ObjectPath("/p")})
	c.NoError(err)
	c.Equal(want, data)
}

func TestMarshalDictEntryForms(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	want, err := Marshal(stringDictSig, Dict{{Key: "a", Value: "b"}})
	c.NoError(err)
	for _, one := range []any{
		[]DictEntry{{Key: "a", Value: "b"}},
		[]any{DictEntry{Key: "a", Value: "b"}},
		[]any{Struct{"a", "b"}},
		[]any{[]any{"a", "b"}},
		Array{Elem: stringEntrySig, Values: []any{DictEntry{Key: "a", Value: "b"}}},
		map[myText]myText{"a": "b"},
		// A slice or an array of entries marshals into a dictionary exactly as it does into an array of structures,
		// whatever its Go type is named.
		[]Struct{{"a", "b"}},
		[1]DictEntry{{Key: "a", Value: "b"}},
		namedDict{{Key: "a", Value: "b"}},
	} {
		data, marshalErr := Marshal(stringDictSig, one)
		c.NoError(marshalErr, "%T", one)
		c.Equal(want, data, "%T", one)
	}
	for _, one := range []any{
		[]any{Struct{"a"}},
		[]any{[]any{"a", "b", "c"}},
		[]any{"not an entry"},
		// An explicitly typed array that contradicts the dictionary it is being marshaled into is refused, just as it
		// is for any other array.
		Array{Elem: "{si}", Values: []any{DictEntry{Key: "a", Value: "b"}}},
		Array{Elem: "ss", Values: []any{DictEntry{Key: "a", Value: "b"}}},
		// Something that is neither a series of entries nor a map has no shape a dictionary could take.
		"not a container",
		[]Struct{{"a", "b", "c"}},
	} {
		_, marshalErr := Marshal(stringDictSig, one)
		c.HasError(marshalErr, "%T", one)
	}
}

func TestMarshalMapKeyOrdering(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	fromMap, err := Marshal("a{ns}", map[int16]string{2: "c", -1: "a", 1: "b"})
	c.NoError(err)
	fromDict, err := Marshal("a{ns}", Dict{
		{Key: int16(-1), Value: "a"},
		{Key: int16(1), Value: "b"},
		{Key: int16(2), Value: "c"},
	})
	c.NoError(err)
	c.Equal(fromDict, fromMap)
	fromMap, err = Marshal("a{bs}", map[bool]string{true: "t", false: "f"})
	c.NoError(err)
	fromDict, err = Marshal("a{bs}", Dict{{Key: false, Value: "f"}, {Key: true, Value: "t"}})
	c.NoError(err)
	c.Equal(fromDict, fromMap)
	fromMap, err = Marshal("a{ds}", map[float64]string{2.5: "b", 1.5: "a"})
	c.NoError(err)
	fromDict, err = Marshal("a{ds}", Dict{{Key: 1.5, Value: "a"}, {Key: 2.5, Value: "b"}})
	c.NoError(err)
	c.Equal(fromDict, fromMap)
	fromMap, err = Marshal("a{ts}", map[uint64]string{2: "b", 1: "a"})
	c.NoError(err)
	fromDict, err = Marshal("a{ts}", Dict{{Key: uint64(1), Value: "a"}, {Key: uint64(2), Value: "b"}})
	c.NoError(err)
	c.Equal(fromDict, fromMap)
}
