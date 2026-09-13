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
	"cmp"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"
)

// Marshal marshals values into the D-Bus wire format, using sig to determine the type of each one. sig must be a valid
// sequence of complete types with one type per value. Alignment is relative to the start of the returned bytes, which
// is what a message body requires, since a body always starts on an 8 byte boundary.
func Marshal(sig Signature, values ...any) ([]byte, error) {
	types, err := sig.Types()
	if err != nil {
		return nil, err
	}
	if len(types) != len(values) {
		return nil, fmt.Errorf("dbus: signature %q describes %d values, but %d were given", sig, len(types),
			len(values))
	}
	var e encoder
	for i, one := range values {
		if err = e.value(types[i], one); err != nil {
			return nil, err
		}
	}
	return e.buf, nil
}

// encoder accumulates the marshaled form of a series of values. Alignment is relative to the start of buf.
type encoder struct {
	buf   []byte
	depth int
}

func (e *encoder) align(n int) {
	for len(e.buf)%n != 0 {
		e.buf = append(e.buf, 0)
	}
}

func (e *encoder) putByte(v byte) { e.buf = append(e.buf, v) }

func (e *encoder) putUint16(v uint16) {
	e.align(2)
	e.buf = binary.LittleEndian.AppendUint16(e.buf, v)
}

func (e *encoder) putUint32(v uint32) {
	e.align(4)
	e.buf = binary.LittleEndian.AppendUint32(e.buf, v)
}

func (e *encoder) putUint64(v uint64) {
	e.align(8)
	e.buf = binary.LittleEndian.AppendUint64(e.buf, v)
}

// putString writes a string or object path: a 4 byte length, the bytes, then a terminating NUL.
func (e *encoder) putString(s string) {
	e.putUint32(uint32(len(s)))
	e.buf = append(e.buf, s...)
	e.buf = append(e.buf, 0)
}

// putSignature writes a signature: a 1 byte length, the bytes, then a terminating NUL.
func (e *encoder) putSignature(s Signature) {
	e.buf = append(e.buf, byte(len(s)))
	e.buf = append(e.buf, s...)
	e.buf = append(e.buf, 0)
}

// value marshals one value. sig must be exactly one complete type and must already have been validated.
func (e *encoder) value(sig Signature, v any) error {
	switch sig[0] {
	case 'y':
		b, err := asUint(v, math.MaxUint8, sig)
		if err != nil {
			return err
		}
		e.putByte(byte(b))
	case 'b':
		b, ok := v.(bool)
		if !ok {
			return typeError(sig, v)
		}
		var n uint32
		if b {
			n = 1
		}
		e.putUint32(n)
	case 'n':
		n, err := asInt(v, math.MinInt16, math.MaxInt16, sig)
		if err != nil {
			return err
		}
		e.putUint16(uint16(int16(n)))
	case 'q':
		n, err := asUint(v, math.MaxUint16, sig)
		if err != nil {
			return err
		}
		e.putUint16(uint16(n))
	case 'i':
		n, err := asInt(v, math.MinInt32, math.MaxInt32, sig)
		if err != nil {
			return err
		}
		e.putUint32(uint32(int32(n)))
	case 'u':
		n, err := asUint(v, math.MaxUint32, sig)
		if err != nil {
			return err
		}
		e.putUint32(uint32(n))
	case 'x':
		n, err := asInt(v, math.MinInt64, math.MaxInt64, sig)
		if err != nil {
			return err
		}
		e.putUint64(uint64(n))
	case 't':
		n, err := asUint(v, math.MaxUint64, sig)
		if err != nil {
			return err
		}
		e.putUint64(n)
	case 'd':
		f, err := asFloat(v, sig)
		if err != nil {
			return err
		}
		e.putUint64(math.Float64bits(f))
	case 's':
		s, err := asString(v)
		if err != nil {
			return err
		}
		e.putString(s)
	case 'o':
		p, err := asObjectPath(v)
		if err != nil {
			return err
		}
		e.putString(string(p))
	case 'g':
		s, err := asSignature(v)
		if err != nil {
			return err
		}
		if err = s.Validate(); err != nil {
			return err
		}
		e.putSignature(s)
	case 'v':
		return e.variant(v)
	case 'a':
		if sig[1] == '{' {
			return e.dict(sig[1:], v)
		}
		return e.array(sig[1:], v)
	case '(':
		return e.structure(sig, v)
	default:
		return fmt.Errorf("dbus: cannot marshal type %q", sig)
	}
	return nil
}

func (e *encoder) variant(v any) error {
	sig, val := Signature(""), v
	if variant, ok := v.(Variant); ok {
		sig, val = variant.Sig, variant.Value
		if err := sig.ValidateSingle(); err != nil {
			return err
		}
	} else {
		derived, err := SignatureOf(v)
		if err != nil {
			return err
		}
		if err = derived.ValidateSingle(); err != nil {
			return err
		}
		sig = derived
	}
	if e.depth >= MaxDepth {
		return errTooDeep
	}
	e.depth++
	e.putSignature(sig)
	err := e.value(sig, val)
	e.depth--
	return err
}

// startArray writes the placeholder for an array's length and returns the position of that placeholder along with the
// position at which the elements start.
func (e *encoder) startArray(elemAlign int) (lengthPos, start int) {
	e.align(4)
	lengthPos = len(e.buf)
	e.buf = append(e.buf, 0, 0, 0, 0)
	e.align(elemAlign) // Present even when the array is empty
	return lengthPos, len(e.buf)
}

// finishArray back-fills the length of an array whose placeholder is at lengthPos and whose elements start at start.
func (e *encoder) finishArray(lengthPos, start int) error {
	n := len(e.buf) - start
	if n > MaxArraySize {
		return fmt.Errorf("dbus: array of %d bytes exceeds the %d byte limit", n, MaxArraySize)
	}
	binary.LittleEndian.PutUint32(e.buf[lengthPos:], uint32(n))
	return nil
}

func (e *encoder) array(elem Signature, v any) error {
	if e.depth >= MaxDepth {
		return errTooDeep
	}
	e.depth++
	defer func() { e.depth-- }()
	lengthPos, start := e.startArray(alignmentOf(elem[0]))
	if err := e.arrayElements(elem, v); err != nil {
		return err
	}
	return e.finishArray(lengthPos, start)
}

func (e *encoder) arrayElements(elem Signature, v any) error {
	switch elem {
	case "y":
		if b, ok := v.([]byte); ok {
			e.buf = append(e.buf, b...)
			return nil
		}
	case "s":
		if s, ok := v.([]string); ok {
			for _, one := range s {
				if err := e.value(elem, one); err != nil {
					return err
				}
			}
			return nil
		}
	case "i":
		if s, ok := v.([]int32); ok {
			for _, one := range s {
				e.putUint32(uint32(one))
			}
			return nil
		}
	case "u":
		if s, ok := v.([]uint32); ok {
			for _, one := range s {
				e.putUint32(one)
			}
			return nil
		}
	case "o":
		if s, ok := v.([]ObjectPath); ok {
			for _, one := range s {
				if err := e.value(elem, one); err != nil {
					return err
				}
			}
			return nil
		}
	case objectRefSig:
		if s, ok := v.([]ObjectRef); ok {
			for _, one := range s {
				if err := e.value(elem, one); err != nil {
					return err
				}
			}
			return nil
		}
	}
	switch tv := v.(type) {
	case []any:
		for _, one := range tv {
			if err := e.value(elem, one); err != nil {
				return err
			}
		}
		return nil
	case Array:
		if tv.Elem != "" && tv.Elem != elem {
			return fmt.Errorf("dbus: array of %q cannot hold elements of type %q", elem, tv.Elem)
		}
		for _, one := range tv.Values {
			if err := e.value(elem, one); err != nil {
				return err
			}
		}
		return nil
	default:
		rv := reflect.ValueOf(v)
		if k := rv.Kind(); k != reflect.Slice && k != reflect.Array {
			return typeError("a"+elem, v)
		}
		for i := 0; i < rv.Len(); i++ {
			if err := e.value(elem, rv.Index(i).Interface()); err != nil {
				return err
			}
		}
		return nil
	}
}

// dict marshals an array of dict entries. sig is the dict entry type, including its braces.
func (e *encoder) dict(sig Signature, v any) error {
	if e.depth >= MaxDepth {
		return errTooDeep
	}
	keyEnd, err := scanType(sig, 1, 0)
	if err != nil {
		return err
	}
	keySig := sig[1:keyEnd]
	valSig := sig[keyEnd : len(sig)-1]
	e.depth++
	defer func() { e.depth-- }()
	lengthPos, start := e.startArray(8)
	switch tv := v.(type) {
	case Dict:
		err = e.dictEntries(keySig, valSig, tv)
	case []DictEntry:
		err = e.dictEntries(keySig, valSig, tv)
	case []any:
		for _, one := range tv {
			if err = e.dictEntry(keySig, valSig, one); err != nil {
				break
			}
		}
	case Array:
		for _, one := range tv.Values {
			if err = e.dictEntry(keySig, valSig, one); err != nil {
				break
			}
		}
	default:
		err = e.mapEntries(keySig, valSig, v)
	}
	if err != nil {
		return err
	}
	return e.finishArray(lengthPos, start)
}

func (e *encoder) dictEntries(keySig, valSig Signature, entries Dict) error {
	for _, entry := range entries {
		if err := e.pair(keySig, valSig, entry.Key, entry.Value); err != nil {
			return err
		}
	}
	return nil
}

// dictEntry marshals a single dict entry supplied as a [DictEntry], a [Struct] or a []any with exactly two elements.
func (e *encoder) dictEntry(keySig, valSig Signature, v any) error {
	switch tv := v.(type) {
	case DictEntry:
		return e.pair(keySig, valSig, tv.Key, tv.Value)
	case Struct:
		if len(tv) != 2 {
			return fmt.Errorf("dbus: a dict entry requires 2 values, but %d were given", len(tv))
		}
		return e.pair(keySig, valSig, tv[0], tv[1])
	case []any:
		if len(tv) != 2 {
			return fmt.Errorf("dbus: a dict entry requires 2 values, but %d were given", len(tv))
		}
		return e.pair(keySig, valSig, tv[0], tv[1])
	default:
		return typeError("{"+keySig+valSig+"}", v)
	}
}

func (e *encoder) pair(keySig, valSig Signature, key, value any) error {
	if e.depth >= MaxDepth {
		return errTooDeep
	}
	e.depth++
	defer func() { e.depth-- }()
	e.align(8)
	if err := e.value(keySig, key); err != nil {
		return err
	}
	return e.value(valSig, value)
}

// mapEntries marshals a Go map as a series of dict entries, ordered by key so that the result is deterministic.
func (e *encoder) mapEntries(keySig, valSig Signature, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Map {
		return typeError("a{"+keySig+valSig+"}", v)
	}
	keys := rv.MapKeys()
	slices.SortStableFunc(keys, compareMapKeys)
	for _, key := range keys {
		if err := e.pair(keySig, valSig, key.Interface(), rv.MapIndex(key).Interface()); err != nil {
			return err
		}
	}
	return nil
}

func compareMapKeys(a, b reflect.Value) int {
	switch a.Kind() {
	case reflect.String:
		return strings.Compare(a.String(), b.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return cmp.Compare(a.Int(), b.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return cmp.Compare(a.Uint(), b.Uint())
	case reflect.Float32, reflect.Float64:
		return cmp.Compare(a.Float(), b.Float())
	case reflect.Bool:
		if a.Bool() == b.Bool() {
			return 0
		}
		if a.Bool() {
			return 1
		}
		return -1
	default:
		return 0
	}
}

// structure marshals a structure. sig is the structure type, including its parentheses.
func (e *encoder) structure(sig Signature, v any) error {
	if e.depth >= MaxDepth {
		return errTooDeep
	}
	var fields []any
	switch tv := v.(type) {
	case Struct:
		fields = tv
	case []any:
		fields = tv
	case ObjectRef:
		if sig != objectRefSig {
			return typeError(sig, v)
		}
		fields = []any{tv.Name, tv.Path}
	case DictEntry:
		fields = []any{tv.Key, tv.Value}
	default:
		return typeError(sig, v)
	}
	types, err := sig[1 : len(sig)-1].Types()
	if err != nil {
		return err
	}
	if len(types) != len(fields) {
		return fmt.Errorf("dbus: structure %q requires %d fields, but %d were given", sig, len(types), len(fields))
	}
	e.depth++
	defer func() { e.depth-- }()
	e.align(8)
	for i, one := range types {
		if err = e.value(one, fields[i]); err != nil {
			return err
		}
	}
	return nil
}

func typeError(sig Signature, v any) error {
	return fmt.Errorf("dbus: cannot marshal %T as %q", v, sig)
}

func asInt(v any, minimum, maximum int64, sig Signature) (int64, error) {
	var n int64
	switch tv := v.(type) {
	case int16:
		n = int64(tv)
	case int32:
		n = int64(tv)
	case int64:
		n = tv
	case int:
		n = int64(tv)
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n = rv.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			u := rv.Uint()
			if u > uint64(maximum) {
				return 0, rangeError(v, sig)
			}
			n = int64(u)
		default:
			return 0, typeError(sig, v)
		}
	}
	if n < minimum || n > maximum {
		return 0, rangeError(v, sig)
	}
	return n, nil
}

func asUint(v any, maximum uint64, sig Signature) (uint64, error) {
	var n uint64
	switch tv := v.(type) {
	case byte:
		n = uint64(tv)
	case uint16:
		n = uint64(tv)
	case uint32:
		n = uint64(tv)
	case uint64:
		n = tv
	case uint:
		n = uint64(tv)
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n = rv.Uint()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i := rv.Int()
			if i < 0 {
				return 0, rangeError(v, sig)
			}
			n = uint64(i)
		default:
			return 0, typeError(sig, v)
		}
	}
	if n > maximum {
		return 0, rangeError(v, sig)
	}
	return n, nil
}

func asFloat(v any, sig Signature) (float64, error) {
	switch tv := v.(type) {
	case float64:
		return tv, nil
	case float32:
		return float64(tv), nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Float32, reflect.Float64:
		return rv.Float(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint()), nil
	default:
		return 0, typeError(sig, v)
	}
}

func asString(v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.String {
			return "", typeError("s", v)
		}
		s = rv.String()
	}
	if err := validateStringContent(s); err != nil {
		return "", err
	}
	return s, nil
}

func asObjectPath(v any) (ObjectPath, error) {
	p, ok := v.(ObjectPath)
	if !ok {
		s, err := asString(v)
		if err != nil {
			return "", typeError("o", v)
		}
		p = ObjectPath(s)
	}
	if err := p.Validate(); err != nil {
		return "", err
	}
	return p, nil
}

func asSignature(v any) (Signature, error) {
	s, ok := v.(Signature)
	if !ok {
		str, err := asString(v)
		if err != nil {
			return "", typeError("g", v)
		}
		s = Signature(str)
	}
	return s, nil
}

func validateStringContent(s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("dbus: string %q is not valid UTF-8", s)
	}
	if strings.IndexByte(s, 0) >= 0 {
		return fmt.Errorf("dbus: string %q contains a NUL", s)
	}
	return nil
}

func rangeError(v any, sig Signature) error {
	return fmt.Errorf("dbus: %v is out of range for %q", v, sig)
}
