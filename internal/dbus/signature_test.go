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
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

func TestSignatureValidate(t *testing.T) {
	t.Parallel()
	for _, one := range []Signature{
		"",
		"y", "b", "n", "q", "i", "u", "x", "t", "d", "s", "o", "g", "v",
		"ybnqiuxtdsogv",
		"ay", "as", "av", "aay", "a(ii)", "aa{sv}",
		"(i)", "(ii)", "(i(ii))", "((so)(so))", "(a{sv})",
		"a{sv}", stringDictSig, "a{ya{sv}}", "a{s(ii)}", "a{oas}",
		"siiv" + propertiesSig,
		"a((so)(so)(so)iiassusau)",
		Signature(strings.Repeat("a", MaxDepth-1) + "y"),
		Signature(strings.Repeat("(", MaxDepth-1) + "y" + strings.Repeat(")", MaxDepth-1)),
	} {
		t.Run(string(one), func(t *testing.T) {
			t.Parallel()
			check.New(t).NoError(one.Validate(), "%q should be valid", one)
		})
	}
}

func TestSignatureValidateRejects(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		name string
		sig  Signature
	}{
		{name: "unknown code", sig: "Z"},
		{name: "file descriptor", sig: "h"},
		{name: "file descriptor in a container", sig: "a(ih)"},
		{name: "array without an element type", sig: "a"},
		{name: "array of nothing but an array", sig: "aa"},
		{name: "unterminated struct", sig: "(i"},
		{name: "unopened struct", sig: "i)"},
		{name: "empty struct", sig: "()"},
		{name: "dict entry outside an array", sig: stringEntrySig},
		{name: "dict entry with a container key", sig: "a{(i)s}"},
		{name: "dict entry with a variant key", sig: "a{vs}"},
		{name: "dict entry with one type", sig: "a{s}"},
		{name: "dict entry with three types", sig: "a{sss}"},
		{name: "unterminated dict entry", sig: "a{ss"},
		{name: "reversed braces", sig: "a}ss{"},
		{name: "too long", sig: Signature(strings.Repeat("y", MaxSignatureLength+1))},
		{name: "arrays nested too deeply", sig: Signature(strings.Repeat("a", MaxDepth+1) + "y")},
		{
			name: "structs nested too deeply",
			sig:  Signature(strings.Repeat("(", MaxDepth+1) + "y" + strings.Repeat(")", MaxDepth+1)),
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			check.New(t).HasError(one.sig.Validate(), "%q should be invalid", one.sig)
		})
	}
}

func TestSignatureValidateSingle(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.NoError(Signature("a{sv}").ValidateSingle())
	c.NoError(Signature("(so)").ValidateSingle())
	c.HasError(Signature("").ValidateSingle())
	c.HasError(Signature("ss").ValidateSingle())
	c.HasError(Signature("a{sv}i").ValidateSingle())
}

func TestSignatureTypes(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	types, err := Signature("siiv" + propertiesSig).Types()
	c.NoError(err)
	c.Equal([]Signature{"s", "i", "i", "v", propertiesSig}, types)
	types, err = Signature("").Types()
	c.NoError(err)
	c.Equal(0, len(types))
	types, err = Signature("a((so)i)(y)").Types()
	c.NoError(err)
	c.Equal([]Signature{"a((so)i)", "(y)"}, types)
	_, err = Signature("(i").Types()
	c.HasError(err)
}

func TestObjectPathValidate(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []ObjectPath{"/", "/a", "/org/a11y/atspi/accessible/root", "/A_1"} {
		c.NoError(one.Validate(), "%q should be valid", one)
		c.Equal(string(one), one.String())
	}
	for _, one := range []ObjectPath{
		"", "a", "a/b", "/a/", "//", "///", "/a//b", "/a/b//", "/a-b", "/a.b", "/a b", "/a/b-c/d",
	} {
		c.HasError(one.Validate(), "%q should be invalid", one)
	}
	// The path is validated in place rather than split into its elements, since the decoder calls this on as much as
	// MaxArraySize bytes of whatever a peer chose to send, and an error message repeats only the start of it.
	long := ObjectPath("/" + strings.Repeat("a", 200) + "-")
	err := long.Validate()
	c.HasError(err)
	c.True(len(err.Error()) < 150, "expected the error to name only the start of a long path, but it was %d bytes",
		len(err.Error()))
}

// namedDict is a dictionary under a name of its own, which is what a package that wants to hang methods off one
// declares; nothing about it says it is anything other than a slice of dict entries.
type namedDict []DictEntry

func TestSignatureOf(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		name  string
		value any
		want  Signature
	}{
		{name: "byte", value: byte(1), want: "y"},
		{name: "bool", value: true, want: "b"},
		{name: "int16", value: int16(1), want: "n"},
		{name: "uint16", value: uint16(1), want: "q"},
		{name: "int32", value: int32(1), want: "i"},
		{name: "uint32", value: uint32(1), want: "u"},
		{name: "int64", value: int64(1), want: "x"},
		{name: "uint64", value: uint64(1), want: "t"},
		{name: "float64", value: 1.0, want: "d"},
		{name: "string", value: "s", want: "s"},
		{name: "object path", value: ObjectPath("/a"), want: "o"},
		{name: "signature", value: Signature("a{sv}"), want: "g"},
		{name: "variant", value: Variant{Sig: "s", Value: "a"}, want: "v"},
		{name: "bytes", value: []byte{1}, want: "ay"},
		{name: "strings", value: []string{"a"}, want: "as"},
		{name: "int32s", value: []int32{1}, want: "ai"},
		{name: "uint32s", value: []uint32{1}, want: "au"},
		{name: "object paths", value: []ObjectPath{"/a"}, want: "ao"},
		{name: "object reference", value: ObjectRef{Name: "n", Path: "/p"}, want: "(so)"},
		{name: "object references", value: []ObjectRef{}, want: "a(so)"},
		{name: "explicit array", value: Array{Elem: "(ii)"}, want: "a(ii)"},
		{name: "struct", value: Struct{"a", int32(1)}, want: "(si)"},
		{name: "nested struct", value: Struct{Struct{byte(1)}}, want: "((y))"},
		{name: "slice", value: []any{"a", "b"}, want: "as"},
		{name: "slice of structs", value: []any{Struct{int32(1)}}, want: "a(i)"},
		{name: "dict", value: Dict{{Key: "k", Value: "v"}}, want: stringDictSig},
		{name: "dict of variants", value: Dict{{Key: "k", Value: Variant{Sig: "i", Value: int32(1)}}}, want: "a{sv}"},
		{name: "multi-entry dict", value: Dict{{Key: "j", Value: "v"}, {Key: "k", Value: "w"}}, want: stringDictSig},
		{name: "map", value: map[string]string{}, want: stringDictSig},
		{name: "map of variants", value: map[string]Variant{}, want: propertiesSig},
		{name: "map of slices", value: map[byte][]string{}, want: "a{yas}"},
		{name: "map of maps", value: map[string]map[string]string{}, want: "a{sa{ss}}"},
		{name: "map with interface values", value: map[string]any{"a": "b"}, want: stringDictSig},
		{name: "named slice", value: []Variant{}, want: "av"},
		// A dict entry is a complete type only as the element type of an array, which is the one place Array accepts
		// it: an empty a{sv} is the most common empty container AT-SPI sends, and an empty Dict cannot say what its
		// keys and values would have been.
		{name: "explicit empty dictionary", value: Array{Elem: "{sv}"}, want: propertiesSig},
		{
			name:  "explicit dictionary",
			value: Array{Elem: stringEntrySig, Values: []any{DictEntry{Key: "a", Value: "b"}}},
			want:  stringDictSig,
		},
		// A named slice of dict entries is a dictionary however it was named, and the key and value types come from
		// the entries, since the Go type alone cannot supply them.
		{name: "named dictionary type", value: namedDict{{Key: "a", Value: "b"}}, want: stringDictSig},
		{name: "array of dict entries", value: [1]DictEntry{{Key: "a", Value: "b"}}, want: stringDictSig},
		// A Dict is a slice of dict entries, so its element type cannot be derived without a value to look at. An
		// empty want means that an error is expected.
		{name: "slice of dicts", value: []Dict{}, want: ""},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			sig, err := SignatureOf(one.value)
			if one.want == "" {
				c.HasError(err)
				return
			}
			c.NoError(err)
			c.Equal(one.want, sig)
		})
	}
}

func TestEmptyDictionaryWhereTheSignatureIsDerived(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	// An empty a{sv} has no values to derive a signature from, so the element type has to be stated; this is the shape
	// that says it, and it has to work everywhere a signature is derived rather than supplied.
	m := NewSignal(testPath, eventInterface, "Empty")
	m.Serial = 1
	c.NoError(m.SetBody("insert", Array{Elem: "{sv}"}))
	c.Equal(Signature("s"+propertiesSig), m.Signature)
	args, err := m.Args()
	c.NoError(err)
	c.Equal([]any{"insert", Dict{}}, args)
	data, err := Marshal("v", Array{Elem: "{sv}"})
	c.NoError(err)
	values, err := Unmarshal("v", data)
	c.NoError(err)
	c.Equal([]any{Variant{Sig: propertiesSig, Value: Dict{}}}, values)
}

func TestSignatureOfDictChecksEveryEntry(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	// A dictionary has one key type and one value type, so deriving them from the first entry alone turned a mixture
	// into a signature that was silently wrong and a confusing "cannot marshal" failure much further along.
	_, err := SignatureOf(Dict{{Key: "a", Value: "b"}, {Key: "c", Value: int32(1)}})
	c.HasError(err)
	c.Contains(err.Error(), "values have differing types")
	_, err = SignatureOf(Dict{{Key: "a", Value: "b"}, {Key: int32(1), Value: "c"}})
	c.HasError(err)
	c.Contains(err.Error(), "keys have differing types")
}

func TestSignatureOfMultipleValues(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	sig, err := SignatureOf("a", int32(1), Variant{Sig: "b", Value: true})
	c.NoError(err)
	c.Equal(Signature("siv"), sig)
	sig, err = SignatureOf()
	c.NoError(err)
	c.Equal(Signature(""), sig)
}

func TestSignatureOfRejects(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		value any
		name  string
	}{
		{name: "nil", value: nil},
		{name: "untyped int", value: 1},
		{name: "float32", value: float32(1)},
		{name: "empty slice", value: []any{}},
		{name: "mixed slice", value: []any{"a", int32(1)}},
		{name: "empty dict", value: Dict{}},
		{name: "empty map with interface values", value: map[string]any{}},
		{name: "map with mixed values", value: map[string]any{"a": "b", "c": int32(1)}},
		{name: "map with a container key", value: map[[2]byte]string{}},
		{name: "bare dict entry", value: DictEntry{Key: "a", Value: "b"}},
		{name: "dict with a container key", value: Dict{{Key: Struct{byte(1)}, Value: "x"}}},
		{name: "dict with an undecidable value", value: Dict{{Key: "a", Value: 1}}},
		{name: "dict with an undecidable key", value: Dict{{Key: 1, Value: "a"}}},
		{name: "dict with mixed values", value: Dict{{Key: "a", Value: "b"}, {Key: "c", Value: int32(1)}}},
		{name: "dict with mixed keys", value: Dict{{Key: "a", Value: "b"}, {Key: int32(1), Value: "c"}}},
		{name: "dict whose later value cannot be described", value: Dict{{Key: "a", Value: "b"}, {Key: "c", Value: 1}}},
		{name: "slice with an undecidable element", value: []any{1}},
		{name: "struct with an undecidable field", value: Struct{1}},
		{name: "empty struct", value: Struct{}},
		{name: "channel", value: make(chan int)},
		{name: "struct type", value: struct{ A int32 }{}},
		{name: "array with a bad element type", value: Array{Elem: "ss"}},
		{name: "array with no element type", value: Array{}},
		{name: "array with a malformed dict entry element type", value: Array{Elem: "{s}"}},
		{name: "empty named dictionary type", value: namedDict{}},
		{name: "named dictionary with mixed values", value: namedDict{{Key: "a", Value: "b"}, {Key: "c", Value: 1}}},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			_, err := SignatureOf(one.value)
			check.New(t).HasError(err)
		})
	}
}

func TestSignatureDepthLimitsAreIndependent(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	// The specification allows 32 nested arrays and 32 nested structures, and the two limits do not share a budget, so
	// 32 array and structure pairs are valid even though they open 64 containers between them.
	pairs := Signature(strings.Repeat("a(", MaxDepth) + "y" + strings.Repeat(")", MaxDepth))
	c.NoError(pairs.Validate(), "%q should be valid", pairs)
	c.NoError(pairs.ValidateSingle(), "%q should be valid", pairs)
	// Dict entries count towards the structure limit, since that is what they are on the wire, so a dictionary nested
	// 32 deep uses 32 of each and is valid too.
	dicts := Signature(strings.Repeat("a{s", MaxDepth) + "y" + strings.Repeat("}", MaxDepth))
	c.NoError(dicts.Validate(), "%q should be valid", dicts)
	// Each limit is still enforced on its own.
	c.HasError(Signature(strings.Repeat("a", MaxDepth+1) + "y").Validate())
	c.HasError(Signature(strings.Repeat("(", MaxDepth+1) + "y" + strings.Repeat(")", MaxDepth+1)).Validate())
	// A sibling's depth does not count against its neighbors, since each branch is measured on its own.
	c.NoError(Signature(strings.Repeat("a", MaxDepth) + "y" + strings.Repeat("a", MaxDepth) + "y").Validate())
}

func TestDepthLimitsAreIndependentWhenMarshaling(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	sig := Signature(strings.Repeat("a(", MaxDepth) + "y" + strings.Repeat(")", MaxDepth))
	value := any(byte(1))
	for range MaxDepth {
		value = []any{Struct{value}}
	}
	data, err := Marshal(sig, value)
	c.NoError(err)
	values, err := Unmarshal(sig, data)
	c.NoError(err)
	again, err := Marshal(sig, values...)
	c.NoError(err)
	c.Equal(data, again)
}
