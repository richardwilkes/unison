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
	"runtime"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// nestedVariantData returns the body of a variant nested the given number of levels deep, where the outermost level is
// the one described by the signature "v" that is handed to Unmarshal.
func nestedVariantData(levels int) []byte {
	var data []byte
	for range levels - 1 {
		data = append(data, sigAt("v")...)
	}
	data = append(data, sigAt("y")...)
	return append(data, 0)
}

func TestUnmarshalTruncated(t *testing.T) {
	t.Parallel()
	for _, one := range goldenCases() {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			for i := range len(one.want) {
				_, err := Unmarshal(one.sig, one.want[:i])
				c.HasError(err, "truncated to %d of %d bytes", i, len(one.want))
			}
		})
	}
}

func TestUnmarshalRejects(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		name string
		sig  Signature
		data []byte
	}{
		{name: "boolean other than 0 or 1", sig: "b", data: u32At(2)},
		{name: "array larger than the limit", sig: "ay", data: concat(u32At(MaxArraySize+1), pad(4))},
		{name: "array longer than the data", sig: "ay", data: concat(u32At(8), pad(4))},
		{name: "array element overrun", sig: "aq", data: concat(u32At(3), pad(4))},
		{name: "string without a terminator", sig: "s", data: concat(u32At(1), []byte{'a', 'b'})},
		{name: "string that is not utf8", sig: "s", data: concat(u32At(1), []byte{0xFF, 0})},
		{name: "string with an embedded nul", sig: "s", data: concat(u32At(3), []byte{'a', 0, 'b', 0})},
		{name: "invalid object path", sig: "o", data: concat(u32At(1), []byte{'x', 0})},
		{name: "variant with two types", sig: "v", data: concat(sigAt("ss"), pad(1), strAt("a"), strAt("b"))},
		{name: "variant with a file descriptor", sig: "v", data: concat(sigAt("h"), u32At(0))},
		{name: "variant with an unknown type", sig: "v", data: concat(sigAt("Z"), u32At(0))},
		{name: "signature without a terminator", sig: "g", data: []byte{1, 'y', 'y'}},
		{name: "trailing bytes", sig: "y", data: []byte{1, 2}},
		{name: "dict entry without a value", sig: stringDictSig, data: concat(u32At(6), pad(4), strAt("a"))},
		{name: "variants nested too deeply", sig: "v", data: nestedVariantData(MaxDepth + 1)},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			_, err := Unmarshal(one.sig, one.data)
			c.HasError(err)
		})
	}
}

func TestUnmarshalNestedVariantsToTheLimit(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	values, err := Unmarshal("v", nestedVariantData(MaxDepth))
	c.NoError(err)
	c.Equal(1, len(values))
	depth := 0
	for v := values[0]; ; depth++ {
		variant, ok := v.(Variant)
		if !ok {
			break
		}
		v = variant.Value
	}
	c.Equal(MaxDepth, depth)
}

func TestUnmarshalObjectPathArray(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	values, err := Unmarshal("ao", concat(u32At(7), strAt("/x")))
	c.NoError(err)
	c.Equal([]any{[]ObjectPath{"/x"}}, values)
	_, err = Unmarshal("ao", concat(u32At(6), strAt("x")))
	c.HasError(err)
}

// bytesAllocated returns how many bytes f allocated. The garbage collector is run first so that nothing left over from
// earlier work is charged to f, and a test that uses this must not be parallel: every goroutine's allocations are
// counted together, so anything else running at the same time would be counted as well.
func bytesAllocated(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// maxCheapAllocation is what "without allocating anything to speak of" is worth in bytes: enough for the handful of
// small values that reaching the error at all requires, and nothing like the lengths these tests declare.
const maxCheapAllocation = 4 << 10

func TestUnmarshalLargeArrayDeclarationIsCheap(t *testing.T) { // Not parallel: it measures allocation
	c := check.New(t)
	// A huge length must be rejected without allocating anything, even though the data supplied is tiny, which is what
	// the measurement here is for: asserting only that an error comes back would pass just as well if the decoder
	// allocated the whole declared length first.
	for _, length := range []uint32{MaxArraySize + 1, 1 << 30, 0xFFFFFFFF} {
		data := u32At(length)
		var err error
		allocated := bytesAllocated(func() { _, err = Unmarshal("au", data) })
		c.HasError(err, "declared length %d", length)
		c.True(allocated < maxCheapAllocation, "a declared length of %d allocated %d bytes", length, allocated)
	}
}

// maxEmptyContainerCost is what one empty container inside an array may cost, in bytes, once it has been decoded into
// an interface value. Each of these is four bytes on the wire, so the cost is what a peer amplifies those four bytes
// by: sizing every one of them for eight elements it does not have cost over 200 bytes apiece, which turned a legal 64
// MiB array into gigabytes.
const maxEmptyContainerCost = 160

func TestUnmarshalDoesNotAmplifyEmptyContainers(t *testing.T) { // Not parallel: it measures allocation
	c := check.New(t)
	const count = 4096
	// An array of empty containers is the cheapest message a peer can write and the most expensive one to decode: the
	// element count is entirely of its choosing, and nothing but the four byte length of each element is on the wire.
	for _, sig := range []Signature{"aas", "aao", "aav", "aaas"} {
		data := u32At(4 * count)
		for range count {
			data = append(data, u32At(0)...)
		}
		var (
			values []any
			err    error
		)
		allocated := bytesAllocated(func() { values, err = Unmarshal(sig, data) })
		c.NoError(err, sig)
		c.Equal(1, len(values), sig)
		c.True(allocated < count*maxEmptyContainerCost, "%s of %d empty elements allocated %d bytes for %d on the "+
			"wire", sig, count, allocated, len(data))
	}
}

func TestUnmarshalRejectsPaddingThatIsNotNUL(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	// libdbus rejects this as DBUS_INVALID_ALIGNMENT_PADDING_NOT_NUL, and so do we: the bytes between the values are
	// not ours to interpret, so a peer that writes anything there is not speaking the protocol.
	_, err := Unmarshal("yi", []byte{1, 0xFF, 0xFF, 0xFF, 2, 0, 0, 0})
	c.HasError(err)
	c.Contains(err.Error(), "padding")
	values, err := Unmarshal("yi", []byte{1, 0, 0, 0, 2, 0, 0, 0})
	c.NoError(err)
	c.Equal([]any{byte(1), int32(2)}, values)
	// The padding that precedes the first element of an array is checked as well, even when the array is empty and so
	// has no elements for the padding to align.
	for _, sig := range []Signature{"at", "a(ii)", propertiesSig} {
		_, err = Unmarshal(sig, concat(u32At(0), []byte{1, 2, 3, 4}))
		c.HasError(err, sig)
		c.Contains(err.Error(), "padding", sig)
	}
}
