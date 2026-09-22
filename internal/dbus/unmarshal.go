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
	"errors"
	"fmt"
	"math"
)

var (
	errTruncated     = errors.New("dbus: truncated value")
	errPaddingNotNUL = errors.New("dbus: alignment padding is not NUL")
)

// Unmarshal unmarshals the values described by sig from data, which must contain exactly those values and nothing else.
// Alignment is relative to the start of data, which is what a message body requires, since a body always starts on an 8
// byte boundary. The little-endian encoding is assumed, since that is the only one [Message.Encode] produces and the
// only one a [Message] ever holds; see [Decode] for what happens to a message from a big-endian peer.
func Unmarshal(sig Signature, data []byte) ([]any, error) {
	return unmarshal(sig, data, false)
}

// unmarshal is [Unmarshal] with the byte order of the peer that produced the data.
func unmarshal(sig Signature, data []byte, bigEndian bool) ([]any, error) {
	types, err := sig.Types()
	if err != nil {
		return nil, err
	}
	d := decoder{data: data, bigEndian: bigEndian}
	values := make([]any, 0, len(types))
	for _, one := range types {
		var v any
		if v, err = d.value(one); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	if d.pos != len(data) {
		return nil, fmt.Errorf("dbus: %d bytes remain after unmarshaling %q", len(data)-d.pos, sig)
	}
	return values, nil
}

// decoder reads values from the D-Bus wire format. Alignment is relative to the start of data. bigEndian selects the
// byte order that the peer chose for the message being decoded; the zero value is little-endian, which is what every
// peer Unison has ever met uses and what [Marshal] produces.
type decoder struct {
	data      []byte
	pos       int
	bigEndian bool
	depths
}

// align skips forward to the next multiple of n. The specification requires the padding it skips to be NUL, and
// libdbus rejects a message whose padding is not (DBUS_INVALID_ALIGNMENT_PADDING_NOT_NUL), so it is checked here too:
// anything else is a peer that is making things up, and the bytes it writes there are not ours to interpret.
func (d *decoder) align(n int) error {
	for d.pos%n != 0 {
		if d.pos >= len(d.data) {
			return errTruncated
		}
		if d.data[d.pos] != 0 {
			return errPaddingNotNUL
		}
		d.pos++
	}
	return nil
}

func (d *decoder) take(n int) ([]byte, error) {
	if n < 0 || len(d.data)-d.pos < n {
		return nil, errTruncated
	}
	b := d.data[d.pos : d.pos+n]
	d.pos += n
	return b, nil
}

func (d *decoder) getByte() (byte, error) {
	b, err := d.take(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (d *decoder) getUint16() (uint16, error) {
	if err := d.align(2); err != nil {
		return 0, err
	}
	b, err := d.take(2)
	if err != nil {
		return 0, err
	}
	if d.bigEndian {
		return binary.BigEndian.Uint16(b), nil
	}
	return binary.LittleEndian.Uint16(b), nil
}

func (d *decoder) getUint32() (uint32, error) {
	if err := d.align(4); err != nil {
		return 0, err
	}
	b, err := d.take(4)
	if err != nil {
		return 0, err
	}
	if d.bigEndian {
		return binary.BigEndian.Uint32(b), nil
	}
	return binary.LittleEndian.Uint32(b), nil
}

func (d *decoder) getUint64() (uint64, error) {
	if err := d.align(8); err != nil {
		return 0, err
	}
	b, err := d.take(8)
	if err != nil {
		return 0, err
	}
	if d.bigEndian {
		return binary.BigEndian.Uint64(b), nil
	}
	return binary.LittleEndian.Uint64(b), nil
}

// getString reads a string or object path: a 4 byte length, the bytes, then a terminating NUL.
func (d *decoder) getString() (string, error) {
	n, err := d.getUint32()
	if err != nil {
		return "", err
	}
	if n > MaxArraySize {
		return "", fmt.Errorf("dbus: string of %d bytes exceeds the %d byte limit", n, MaxArraySize)
	}
	b, err := d.take(int(n) + 1)
	if err != nil {
		return "", err
	}
	if b[n] != 0 {
		return "", errors.New("dbus: string is not NUL terminated")
	}
	// The content is checked before the copy rather than after it, so that a string a peer had no business sending is
	// never copied at all; a peer may declare one of up to [MaxArraySize] bytes.
	if err = validateStringBytes(b[:n]); err != nil {
		return "", err
	}
	return string(b[:n]), nil
}

// getSignature reads a signature: a 1 byte length, the bytes, then a terminating NUL.
func (d *decoder) getSignature() (Signature, error) {
	n, err := d.getByte()
	if err != nil {
		return "", err
	}
	b, err := d.take(int(n) + 1)
	if err != nil {
		return "", err
	}
	if b[n] != 0 {
		return "", errors.New("dbus: signature is not NUL terminated")
	}
	return Signature(b[:n]), nil
}

// value reads one value. sig must be exactly one complete type and must already have been validated.
func (d *decoder) value(sig Signature) (any, error) {
	switch sig[0] {
	case 'y':
		return d.getByte()
	case 'b':
		n, err := d.getUint32()
		if err != nil {
			return nil, err
		}
		if n > 1 {
			return nil, fmt.Errorf("dbus: %d is not a valid boolean", n)
		}
		return n == 1, nil
	case 'n':
		n, err := d.getUint16()
		if err != nil {
			return nil, err
		}
		return int16(n), nil
	case 'q':
		return d.getUint16()
	case 'i':
		n, err := d.getUint32()
		if err != nil {
			return nil, err
		}
		return int32(n), nil
	case 'u':
		return d.getUint32()
	case 'x':
		n, err := d.getUint64()
		if err != nil {
			return nil, err
		}
		return int64(n), nil
	case 't':
		return d.getUint64()
	case 'd':
		n, err := d.getUint64()
		if err != nil {
			return nil, err
		}
		return math.Float64frombits(n), nil
	case 's':
		return d.getString()
	case 'o':
		s, err := d.getString()
		if err != nil {
			return nil, err
		}
		p := ObjectPath(s)
		if err = p.Validate(); err != nil {
			return nil, err
		}
		return p, nil
	case 'g':
		s, err := d.getSignature()
		if err != nil {
			return nil, err
		}
		if err = s.Validate(); err != nil {
			return nil, err
		}
		return s, nil
	case 'v':
		return d.variant()
	case 'a':
		if sig[1] == '{' {
			return d.dict(sig[1:])
		}
		return d.array(sig[1:])
	case '(':
		return d.structure(sig)
	default:
		return nil, fmt.Errorf("dbus: cannot unmarshal type %q", sig)
	}
}

func (d *decoder) variant() (any, error) {
	sig, err := d.getSignature()
	if err != nil {
		return nil, err
	}
	if err = sig.ValidateSingle(); err != nil {
		return nil, err
	}
	if err = d.enter('v'); err != nil {
		return nil, err
	}
	defer d.leave('v')
	v, err := d.value(sig)
	if err != nil {
		return nil, err
	}
	return Variant{Sig: sig, Value: v}, nil
}

// skip advances past one value without building anything from it. sig must be exactly one complete type and must
// already have been validated. It is what a value that has to be read to find the end of it but must then be thrown
// away costs, which is the case for a header field whose code the specification requires us to ignore: a peer chooses
// both the type and the length of that field, and building its values only to discard them let a 16 MiB field array
// holding an array of empty containers allocate a gigabyte. Only the framing is read: an array declares how many bytes
// it occupies, so its elements are stepped over without being looked at, and a field that must be ignored is no more
// ours to interpret than the padding between values is.
func (d *decoder) skip(sig Signature) error {
	switch sig[0] {
	case 'y':
		_, err := d.take(1)
		return err
	case 'n', 'q':
		_, err := d.getUint16()
		return err
	case 'b', 'i', 'u':
		_, err := d.getUint32()
		return err
	case 'x', 't', 'd':
		_, err := d.getUint64()
		return err
	case 's', 'o':
		n, err := d.getUint32()
		if err != nil {
			return err
		}
		if n > MaxArraySize {
			return fmt.Errorf("dbus: string of %d bytes exceeds the %d byte limit", n, MaxArraySize)
		}
		_, err = d.take(int(n) + 1)
		return err
	case 'g':
		n, err := d.getByte()
		if err != nil {
			return err
		}
		_, err = d.take(int(n) + 1)
		return err
	case 'v':
		nested, err := d.getSignature()
		if err != nil {
			return err
		}
		if err = nested.ValidateSingle(); err != nil {
			return err
		}
		if err = d.enter('v'); err != nil {
			return err
		}
		defer d.leave('v')
		return d.skip(nested)
	case 'a':
		if err := d.enter('a'); err != nil {
			return err
		}
		defer d.leave('a')
		elem := sig[1:]
		end, err := d.startContainer(alignmentOf(elem[0]))
		if err != nil {
			return err
		}
		// The array declares how many bytes its elements occupy, so the elements themselves need not be walked at all.
		d.pos = end
		return nil
	case '(':
		if err := d.enter('('); err != nil {
			return err
		}
		defer d.leave('(')
		types, err := sig[1 : len(sig)-1].Types()
		if err != nil {
			return err
		}
		if err = d.align(8); err != nil {
			return err
		}
		for _, one := range types {
			if err = d.skip(one); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("dbus: cannot unmarshal type %q", sig)
	}
}

// startContainer reads the length of an array and returns the position just past its last element. The caller has
// already entered the array, so that a length that cannot possibly be read is not read at all.
func (d *decoder) startContainer(elemAlign int) (end int, err error) {
	n, err := d.getUint32()
	if err != nil {
		return 0, err
	}
	if n > MaxArraySize {
		return 0, fmt.Errorf("dbus: array of %d bytes exceeds the %d byte limit", n, MaxArraySize)
	}
	if err = d.align(elemAlign); err != nil { // Present even when the array is empty
		return 0, err
	}
	if len(d.data)-d.pos < int(n) {
		return 0, errTruncated
	}
	return d.pos + int(n), nil
}

func (d *decoder) array(elem Signature) (any, error) {
	if err := d.enter('a'); err != nil {
		return nil, err
	}
	defer d.leave('a')
	end, err := d.startContainer(alignmentOf(elem[0]))
	if err != nil {
		return nil, err
	}
	switch elem {
	case "y":
		b, takeErr := d.take(end - d.pos)
		if takeErr != nil {
			return nil, takeErr
		}
		result := make([]byte, len(b))
		copy(result, b)
		return result, nil
	case "s":
		result := make([]string, 0, d.elementCapacity(end, elem))
		for d.pos < end {
			var s string
			if s, err = d.getString(); err != nil {
				return nil, err
			}
			result = append(result, s)
		}
		return result, d.checkEnd(end)
	case "i":
		result := make([]int32, 0, (end-d.pos)/4)
		for d.pos < end {
			var n uint32
			if n, err = d.getUint32(); err != nil {
				return nil, err
			}
			result = append(result, int32(n))
		}
		return result, d.checkEnd(end)
	case "u":
		result := make([]uint32, 0, (end-d.pos)/4)
		for d.pos < end {
			var n uint32
			if n, err = d.getUint32(); err != nil {
				return nil, err
			}
			result = append(result, n)
		}
		return result, d.checkEnd(end)
	case "o":
		result := make([]ObjectPath, 0, d.elementCapacity(end, elem))
		for d.pos < end {
			var s string
			if s, err = d.getString(); err != nil {
				return nil, err
			}
			p := ObjectPath(s)
			if err = p.Validate(); err != nil {
				return nil, err
			}
			result = append(result, p)
		}
		return result, d.checkEnd(end)
	case objectRefSig:
		read, readErr := d.elementDecoder(elem)
		if readErr != nil {
			return nil, readErr
		}
		result := make([]ObjectRef, 0, d.elementCapacity(end, elem))
		for d.pos < end {
			var v any
			if v, err = read(); err != nil {
				return nil, err
			}
			ref, ok := v.(ObjectRef)
			if !ok {
				return nil, fmt.Errorf("dbus: expected an object reference, but got %T", v)
			}
			result = append(result, ref)
		}
		return result, d.checkEnd(end)
	default:
		read, readErr := d.elementDecoder(elem)
		if readErr != nil {
			return nil, readErr
		}
		result := make([]any, 0, d.elementCapacity(end, elem))
		for d.pos < end {
			var v any
			if v, err = read(); err != nil {
				return nil, err
			}
			result = append(result, v)
		}
		return result, d.checkEnd(end)
	}
}

// maxElementCapacity is the capacity an array is decoded into when the bytes it declares could fill that much of it. It
// is small on purpose: append grows the slice for an array that really does hold more, while the initial capacity is
// paid by every array, including the innumerable empty ones that a peer may pack into one message.
const maxElementCapacity = 8

// elementCapacity returns the capacity to give the slice an array is decoded into, which is however many elements of
// the type elem the bytes between here and end could possibly hold, up to [maxElementCapacity]. Sizing it from an
// element count the peer chose is what let one legal message force gigabytes of allocation: an array of empty
// containers amplified the bytes on the wire by as much as seventy times, since each inner container paid for a
// capacity of eight, and an array of empty arrays has no bytes left for its elements at all.
func (d *decoder) elementCapacity(end int, elem Signature) int {
	return min(maxElementCapacity, (end-d.pos)/minimumElementSize(elem))
}

// minimumElementSize returns the fewest bytes that a value of the given type can occupy on the wire. The type must
// already have been validated. A structure or dict entry is measured as the sum of its fields, without the padding
// that separates it from whatever follows, since underestimating only rounds a capacity up.
func minimumElementSize(sig Signature) int {
	switch sig[0] {
	case 'n', 'q', 'g': // A signature is a one byte length and the NUL that terminates it, with nothing between them
		return 2
	case 'b', 'i', 'u', 'a': // An array is its four byte length, which is all that an empty one is
		return 4
	case 'v': // A one byte signature, its NUL, and the smallest value any type has
		return 4
	case 's', 'o': // A four byte length, no bytes at all, and the NUL that terminates them
		return 5
	case 'x', 't', 'd':
		return 8
	case '(', '{':
		types, err := sig[1 : len(sig)-1].Types()
		if err != nil {
			return 1
		}
		total := 0
		for _, one := range types {
			total += minimumElementSize(one)
		}
		return max(total, 1)
	default: // 'y'
		return 1
	}
}

// elementDecoder returns a function that reads one value of the given type, with whatever preparation that type needs
// done once rather than once per element. Splitting a structure's field types out of its signature is the same work for
// every element of an array of structures, and paying for it per element let an 8 MiB body of one byte structures
// allocate seventeen times its own size, all of it driven by an element count the peer chooses.
func (d *decoder) elementDecoder(sig Signature) (func() (any, error), error) {
	if sig[0] != '(' {
		return func() (any, error) { return d.value(sig) }, nil
	}
	types, err := sig[1 : len(sig)-1].Types()
	if err != nil {
		return nil, err
	}
	return func() (any, error) { return d.structureFields(sig, types) }, nil
}

// dict reads an array of dict entries. sig is the dict entry type, including its braces.
func (d *decoder) dict(sig Signature) (any, error) {
	if err := d.enter('a'); err != nil { // One level for the array...
		return nil, err
	}
	defer d.leave('a')
	if err := d.enter('{'); err != nil { // ...and one for the entries it holds, which are all at the same level
		return nil, err
	}
	defer d.leave('{')
	keyEnd, err := scanType(sig, 1, depths{})
	if err != nil {
		return nil, err
	}
	keySig := sig[1:keyEnd]
	valSig := sig[keyEnd : len(sig)-1]
	// A dict entry's key is always a basic type, but its value may be a structure, so the value is read through a
	// decoder that is prepared once rather than once per entry; see elementDecoder.
	readValue, err := d.elementDecoder(valSig)
	if err != nil {
		return nil, err
	}
	end, err := d.startContainer(8)
	if err != nil {
		return nil, err
	}
	result := make(Dict, 0, d.elementCapacity(end, sig))
	for d.pos < end {
		if err = d.align(8); err != nil {
			return nil, err
		}
		var entry DictEntry
		if entry.Key, err = d.value(keySig); err != nil {
			return nil, err
		}
		if entry.Value, err = readValue(); err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, d.checkEnd(end)
}

// structure reads a structure. sig is the structure type, including its parentheses.
func (d *decoder) structure(sig Signature) (any, error) {
	types, err := sig[1 : len(sig)-1].Types()
	if err != nil {
		return nil, err
	}
	return d.structureFields(sig, types)
}

// structureFields reads a structure whose field types have already been split out of its signature, which is how an
// array of structures avoids paying for that split once per element.
func (d *decoder) structureFields(sig Signature, types []Signature) (any, error) {
	if err := d.enter('('); err != nil {
		return nil, err
	}
	defer d.leave('(')
	if err := d.align(8); err != nil {
		return nil, err
	}
	fields := make(Struct, 0, len(types))
	for _, one := range types {
		v, err := d.value(one)
		if err != nil {
			return nil, err
		}
		fields = append(fields, v)
	}
	if sig == objectRefSig {
		name, nameOK := fields[0].(string)
		path, pathOK := fields[1].(ObjectPath)
		if nameOK && pathOK {
			return ObjectRef{Name: name, Path: path}, nil
		}
	}
	return fields, nil
}

func (d *decoder) checkEnd(end int) error {
	if d.pos != end {
		return fmt.Errorf("dbus: array element overran the end of the array by %d bytes", d.pos-end)
	}
	return nil
}
