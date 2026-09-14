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
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Limits imposed by the D-Bus specification.
const (
	// MaxMessageSize is the largest permitted size, in bytes, of a complete message.
	MaxMessageSize = 1 << 27
	// MaxArraySize is the largest permitted size, in bytes, of the marshaled body of an array.
	MaxArraySize = 1 << 26
	// MaxSignatureLength is the largest permitted length, in bytes, of a signature.
	MaxSignatureLength = 255
	// MaxNameLength is the largest permitted length, in bytes, of a bus, interface, member or error name.
	MaxNameLength = 255
	// MaxDepth is the deepest permitted nesting of containers. It applies separately to arrays, to structures and to
	// variants; see [depths].
	MaxDepth = 32
)

// ObjectPath is the path to an object exported on a D-Bus connection (wire type code 'o').
type ObjectPath string

// Signature is a sequence of zero or more complete D-Bus types (wire type code 'g').
type Signature string

// Variant is a value together with its own signature (wire type code 'v').
type Variant struct {
	Value any
	Sig   Signature
}

// DictEntry is one key and value pair within a [Dict].
type DictEntry struct {
	Key   any
	Value any
}

// Dict is a D-Bus dictionary, i.e. an array of dict entries. Unlike a Go map, it preserves the order of its entries,
// which matters both for reproducible encoding and for the peers that care about the order they published.
type Dict []DictEntry

// Struct is a D-Bus structure. A plain []any is accepted anywhere a Struct is when marshaling, and unmarshaling
// produces a Struct rather than a []any, so that a structure can be told apart from an array. The one exception is the
// (so) structure that AT-SPI uses everywhere, which unmarshals to an [ObjectRef].
type Struct []any

// Array is a D-Bus array with an explicit element type. It is only needed when the element type cannot be derived from
// the values, i.e. for an empty array in a context where the signature is derived rather than supplied, such as
// [Message.SetBody] or the value of a [Variant]. Elem may be a dict entry type, which is the one place a dict entry may
// appear: Array{Elem: "{sv}"} is an empty a{sv}, which an empty [Dict] cannot be, since nothing about it says what its
// keys and values would have been.
type Array struct {
	Elem   Signature
	Values []any
}

// ObjectRef is the (so) structure that AT-SPI uses to refer to an object on another connection: the unique bus name of
// the connection that exports it plus the path of the object itself.
type ObjectRef struct {
	Name string
	Path ObjectPath
}

// objectRefSig is the signature of the structure that [ObjectRef] represents.
const objectRefSig Signature = "(so)"

var (
	errEmptySignature     = errors.New("dbus: empty signature")
	errIncompleteType     = errors.New("dbus: incomplete type in signature")
	errUnixFDUnsupported  = errors.New("dbus: unix file descriptors (h) are not supported")
	errTooManyArrays      = fmt.Errorf("dbus: arrays nested more than %d deep", MaxDepth)
	errTooManyStructures  = fmt.Errorf("dbus: structures nested more than %d deep", MaxDepth)
	errTooManyVariants    = fmt.Errorf("dbus: variants nested more than %d deep", MaxDepth)
	errDictEntryPlacement = errors.New("dbus: a dict entry may only be used as the element type of an array")
	errEmptyArrayNoType   = errors.New("dbus: cannot derive the element type of an empty array; use dbus.Array")
	errEmptyDictNoType    = errors.New("dbus: cannot derive the key and value types of an empty dictionary")
)

// basicTypes holds the type codes of the basic (non-container) types, which are the only types permitted as the key of
// a dict entry. Note that 'v' is not basic and that 'h' is deliberately absent, since it is not supported.
const basicTypes = "ybnqiuxtdsog"

// depths counts how deeply the containers being scanned, encoded or decoded are nested. The specification allows 32
// nested arrays and 32 nested structures, and the two limits are independent of each other, so a signature such as
// "a(a(…a(y)…))" with 17 array and structure pairs is within both even though it opens 34 containers in all.
// Variants are counted on their own as well: nothing in the specification bounds them, but a variant carries its own
// type on the wire rather than in the enclosing signature, so the recursion a nest of them causes has to be bounded by
// something.
type depths struct {
	arrays     int
	structures int
	variants   int
}

// enter records that a container whose type code is c is being entered, returning an error if that would nest more
// deeply than the specification permits. A dict entry counts as a structure, which is what it is on the wire. Every
// successful call must be matched with a call to [depths.leave], except while scanning a signature, where the depths
// are passed by value and unwind on their own.
func (d *depths) enter(c byte) error {
	switch c {
	case 'a':
		if d.arrays >= MaxDepth {
			return errTooManyArrays
		}
		d.arrays++
	case '(', '{':
		if d.structures >= MaxDepth {
			return errTooManyStructures
		}
		d.structures++
	default: // 'v'
		if d.variants >= MaxDepth {
			return errTooManyVariants
		}
		d.variants++
	}
	return nil
}

// leave undoes one call to [depths.enter].
func (d *depths) leave(c byte) {
	switch c {
	case 'a':
		d.arrays--
	case '(', '{':
		d.structures--
	default: // 'v'
		d.variants--
	}
}

// String returns the path as a string.
func (p ObjectPath) String() string { return string(p) }

// Validate returns an error if the path is not a valid D-Bus object path. The elements are walked in place rather than
// split out of the path: the decoder calls this on untrusted input of up to [MaxArraySize] bytes, both for the values
// in a body and for the PATH header field, and materializing a slice holding every element of a 16 MiB path made of
// two byte elements is an order of magnitude more memory than the path itself.
func (p ObjectPath) Validate() error {
	if p == "" || p[0] != '/' {
		return fmt.Errorf("dbus: object path %q does not start with '/'", abbreviate(string(p)))
	}
	if p == "/" {
		return nil
	}
	if p[len(p)-1] == '/' {
		return fmt.Errorf("dbus: object path %q has an empty element", abbreviate(string(p)))
	}
	for i := 1; i < len(p); i++ {
		switch c := p[i]; {
		case c == '/':
			if p[i-1] == '/' {
				return fmt.Errorf("dbus: object path %q has an empty element", abbreviate(string(p)))
			}
		case !isNameChar(c):
			return fmt.Errorf("dbus: object path %q contains the invalid character %q", abbreviate(string(p)),
				string(c))
		}
	}
	return nil
}

// abbreviate shortens a string that came off the wire for an error message. A peer chooses how long an object path is,
// and repeating a 64 MiB one back word for word costs far more than knowing the whole of it is worth.
func abbreviate(s string) string {
	const maxLength = 64
	if len(s) > maxLength {
		return s[:maxLength] + "..."
	}
	return s
}

// isNameChar returns true if c is one of the characters permitted in an object path element or a name element.
func isNameChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// String returns the signature as a string.
func (s Signature) String() string { return string(s) }

// Validate returns an error if the signature is not a valid sequence of zero or more complete types.
func (s Signature) Validate() error {
	if len(s) > MaxSignatureLength {
		return fmt.Errorf("dbus: signature is longer than %d bytes", MaxSignatureLength)
	}
	for i := 0; i < len(s); {
		next, err := scanType(s, i, depths{})
		if err != nil {
			return err
		}
		i = next
	}
	return nil
}

// ValidateSingle returns an error unless the signature is exactly one complete type.
func (s Signature) ValidateSingle() error {
	if len(s) > MaxSignatureLength {
		return fmt.Errorf("dbus: signature is longer than %d bytes", MaxSignatureLength)
	}
	if s == "" {
		return errEmptySignature
	}
	next, err := scanType(s, 0, depths{})
	if err != nil {
		return err
	}
	if next != len(s) {
		return fmt.Errorf("dbus: signature %q is more than one complete type", s)
	}
	return nil
}

// Types splits the signature into its top-level complete types.
func (s Signature) Types() ([]Signature, error) {
	if len(s) > MaxSignatureLength {
		return nil, fmt.Errorf("dbus: signature is longer than %d bytes", MaxSignatureLength)
	}
	var types []Signature
	for i := 0; i < len(s); {
		next, err := scanType(s, i, depths{})
		if err != nil {
			return nil, err
		}
		types = append(types, s[i:next])
		i = next
	}
	return types, nil
}

// scanType validates the complete type that starts at index i and returns the index just past it. d is passed by value,
// so each branch of the type tree is measured on its own rather than against whatever a sibling reached.
func scanType(s Signature, i int, d depths) (next int, err error) {
	if i >= len(s) {
		return 0, errIncompleteType
	}
	switch c := s[i]; c {
	case 'y', 'b', 'n', 'q', 'i', 'u', 'x', 't', 'd', 's', 'o', 'g', 'v':
		return i + 1, nil
	case 'h':
		return 0, errUnixFDUnsupported
	case 'a':
		if err = d.enter('a'); err != nil {
			return 0, err
		}
		if i+1 < len(s) && s[i+1] == '{' {
			return scanDictEntry(s, i+1, d)
		}
		return scanType(s, i+1, d)
	case '(':
		if err = d.enter('('); err != nil {
			return 0, err
		}
		j := i + 1
		for j < len(s) && s[j] != ')' {
			if j, err = scanType(s, j, d); err != nil {
				return 0, err
			}
		}
		if j >= len(s) {
			return 0, fmt.Errorf("dbus: unterminated structure in signature %q", s)
		}
		if j == i+1 {
			return 0, fmt.Errorf("dbus: empty structure in signature %q", s)
		}
		return j + 1, nil
	case '{':
		return 0, errDictEntryPlacement
	case ')', '}':
		return 0, fmt.Errorf("dbus: unexpected %q in signature %q", string(c), s)
	default:
		return 0, fmt.Errorf("dbus: unknown type code %q in signature %q", string(c), s)
	}
}

// scanDictEntry validates the dict entry that starts at index i and returns the index just past it.
func scanDictEntry(s Signature, i int, d depths) (next int, err error) {
	if err = d.enter('{'); err != nil {
		return 0, err
	}
	if i+1 >= len(s) {
		return 0, errIncompleteType
	}
	if !strings.ContainsRune(basicTypes, rune(s[i+1])) {
		return 0, fmt.Errorf("dbus: dict entry key type %q is not a basic type", string(s[i+1]))
	}
	if next, err = scanType(s, i+2, d); err != nil {
		return 0, err
	}
	if next >= len(s) || s[next] != '}' {
		return 0, fmt.Errorf("dbus: malformed dict entry in signature %q", s)
	}
	return next + 1, nil
}

// alignmentOf returns the alignment, in bytes, required by the type whose type code is c.
func alignmentOf(c byte) int {
	switch c {
	case 'n', 'q':
		return 2
	case 'b', 'i', 'u', 's', 'o', 'a':
		return 4
	case 'x', 't', 'd', '(', '{':
		return 8
	default: // 'y', 'g', 'v'
		return 1
	}
}

// SignatureOf derives the signature of the given values. Integer types other than the sized ones, and empty arrays and
// dictionaries whose element types cannot be determined from their Go type, are rejected, since their D-Bus type would
// be a guess; use an explicitly typed value or [Array] for those.
func SignatureOf(values ...any) (Signature, error) {
	var sb strings.Builder
	for _, v := range values {
		sig, err := signatureOfValue(v)
		if err != nil {
			return "", err
		}
		sb.WriteString(string(sig))
	}
	sig := Signature(sb.String())
	if err := sig.Validate(); err != nil {
		return "", err
	}
	return sig, nil
}

func signatureOfValue(v any) (Signature, error) {
	switch tv := v.(type) {
	case byte:
		return "y", nil
	case bool:
		return "b", nil
	case int16:
		return "n", nil
	case uint16:
		return "q", nil
	case int32:
		return "i", nil
	case uint32:
		return "u", nil
	case int64:
		return "x", nil
	case uint64:
		return "t", nil
	case float64:
		return "d", nil
	case string:
		return "s", nil
	case ObjectPath:
		return "o", nil
	case Signature:
		return "g", nil
	case Variant:
		return "v", nil
	case []byte:
		return "ay", nil
	case []string:
		return "as", nil
	case []int32:
		return "ai", nil
	case []uint32:
		return "au", nil
	case []ObjectPath:
		return "ao", nil
	case ObjectRef:
		return objectRefSig, nil
	case []ObjectRef:
		return "a(so)", nil
	case Array:
		// The element type is validated as part of the array rather than on its own, since a dict entry is a complete
		// type only in that position: Array{Elem: "{sv}"} is how an empty a{sv}, the most common empty container
		// AT-SPI sends, is written where the signature has to be derived.
		sig := "a" + tv.Elem
		if err := sig.ValidateSingle(); err != nil {
			return "", err
		}
		return sig, nil
	case Dict:
		return signatureOfDict(tv)
	case []DictEntry:
		return signatureOfDict(tv)
	case DictEntry:
		return "", errDictEntryPlacement
	case Struct:
		return signatureOfStruct(tv)
	case []any:
		return signatureOfSlice(tv)
	default:
		return signatureOfOther(v)
	}
}

func signatureOfDict(d Dict) (Signature, error) {
	if len(d) == 0 {
		return "", errEmptyDictNoType
	}
	keySig, err := signatureOfValue(d[0].Key)
	if err != nil {
		return "", err
	}
	if len(keySig) != 1 || !strings.ContainsRune(basicTypes, rune(keySig[0])) {
		return "", fmt.Errorf("dbus: dict entry key type %q is not a basic type", keySig)
	}
	valSig, err := signatureOfValue(d[0].Value)
	if err != nil {
		return "", err
	}
	// Every remaining entry has to agree with the first, exactly as the elements of an array do: a dictionary has one
	// key type and one value type, and deriving them from the first entry alone would turn a mixture into a signature
	// that is silently wrong and a confusing "cannot marshal" failure much later on.
	for _, entry := range d[1:] {
		var otherSig Signature
		if otherSig, err = signatureOfValue(entry.Key); err != nil {
			return "", err
		}
		if otherSig != keySig {
			return "", fmt.Errorf("dbus: dict entry keys have differing types %q and %q", keySig, otherSig)
		}
		if otherSig, err = signatureOfValue(entry.Value); err != nil {
			return "", err
		}
		if otherSig != valSig {
			return "", fmt.Errorf("dbus: dict entry values have differing types %q and %q", valSig, otherSig)
		}
	}
	return "a{" + keySig + valSig + "}", nil
}

func signatureOfStruct(s Struct) (Signature, error) {
	if len(s) == 0 {
		return "", errors.New("dbus: a structure must have at least one field")
	}
	var sb strings.Builder
	sb.WriteByte('(')
	for _, field := range s {
		sig, err := signatureOfValue(field)
		if err != nil {
			return "", err
		}
		sb.WriteString(string(sig))
	}
	sb.WriteByte(')')
	return Signature(sb.String()), nil
}

func signatureOfSlice(s []any) (Signature, error) {
	if len(s) == 0 {
		return "", errEmptyArrayNoType
	}
	elemSig, err := signatureOfValue(s[0])
	if err != nil {
		return "", err
	}
	for _, one := range s[1:] {
		var otherSig Signature
		if otherSig, err = signatureOfValue(one); err != nil {
			return "", err
		}
		if otherSig != elemSig {
			return "", fmt.Errorf("dbus: array elements have differing types %q and %q", elemSig, otherSig)
		}
	}
	return "a" + elemSig, nil
}

// signatureOfOther derives the signature of values whose Go type is not one of those handled directly, which in
// practice means Go maps and named or unusual slice types.
func signatureOfOther(v any) (Signature, error) {
	if v == nil {
		return "", errors.New("dbus: cannot derive the signature of a nil value")
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map:
		return signatureOfMap(rv)
	case reflect.Slice, reflect.Array:
		if sig, ok := signatureOfType(rv.Type()); ok {
			return sig, nil
		}
		// A slice or array of dict entries is a dictionary however it was named, and signatureOfType deliberately
		// refuses one so that the key and value types can be taken from the entries themselves, which is what happens
		// here; without this a named dictionary type ends at signatureOfValue(DictEntry), which is an error. The
		// element type is already known to be a dict entry, so the entries are copied out wholesale rather than
		// asserted one at a time.
		if rv.Type().Elem() == dictEntryType {
			entries := make(Dict, rv.Len())
			reflect.Copy(reflect.ValueOf(entries), rv)
			return signatureOfDict(entries)
		}
		values := make([]any, rv.Len())
		for i := range values {
			values[i] = rv.Index(i).Interface()
		}
		return signatureOfSlice(values)
	default:
		if sig, ok := signatureOfType(rv.Type()); ok {
			return sig, nil
		}
		return "", fmt.Errorf("dbus: cannot derive the signature of %T", v)
	}
}

// signatureOfMap derives the signature of a Go map. A key or value type that implies no signature on its own falls back
// to the keys or values the map actually holds, which every entry then has to agree on, since a dictionary has one key
// type and one value type. Such a type is not only an interface: [signatureOfType] also declines an unsized integer, a
// float32, a Go structure and a named dictionary type, so map[string]int reaches the fallback as surely as
// map[any]string does, and the latter marshals perfectly well once its keys have been looked at.
func signatureOfMap(rv reflect.Value) (Signature, error) {
	keySig, keyKnown := signatureOfType(rv.Type().Key())
	valSig, valKnown := signatureOfType(rv.Type().Elem())
	if !keyKnown || !valKnown {
		if rv.Len() == 0 {
			return "", errEmptyDictNoType
		}
		for iter := rv.MapRange(); iter.Next(); {
			if !keyKnown {
				sig, err := signatureOfValue(iter.Key().Interface())
				if err != nil {
					return "", err
				}
				if keySig == "" {
					keySig = sig
				} else if keySig != sig {
					return "", fmt.Errorf("dbus: map keys have differing types %q and %q", keySig, sig)
				}
			}
			if !valKnown {
				sig, err := signatureOfValue(iter.Value().Interface())
				if err != nil {
					return "", err
				}
				if valSig == "" {
					valSig = sig
				} else if valSig != sig {
					return "", fmt.Errorf("dbus: map values have differing types %q and %q", valSig, sig)
				}
			}
		}
	}
	if len(keySig) != 1 || !strings.ContainsRune(basicTypes, rune(keySig[0])) {
		if keyKnown {
			return "", fmt.Errorf("dbus: map key type %s cannot be a dict entry key", rv.Type().Key())
		}
		return "", fmt.Errorf("dbus: map keys of type %q cannot be dict entry keys", keySig)
	}
	return "a{" + keySig + valSig + "}", nil
}

var (
	objectPathType = reflect.TypeFor[ObjectPath]()
	signatureType  = reflect.TypeFor[Signature]()
	variantType    = reflect.TypeFor[Variant]()
	objectRefType  = reflect.TypeFor[ObjectRef]()
	dictEntryType  = reflect.TypeFor[DictEntry]()
)

// signatureOfType derives the signature implied by a Go type, without reference to any value of that type. The second
// return value is false if the type does not imply a signature on its own.
func signatureOfType(t reflect.Type) (sig Signature, ok bool) {
	switch t {
	case objectPathType:
		return "o", true
	case signatureType:
		return "g", true
	case variantType:
		return "v", true
	case objectRefType:
		return objectRefSig, true
	}
	switch t.Kind() {
	case reflect.Bool:
		return "b", true
	case reflect.Uint8:
		return "y", true
	case reflect.Int16:
		return "n", true
	case reflect.Uint16:
		return "q", true
	case reflect.Int32:
		return "i", true
	case reflect.Uint32:
		return "u", true
	case reflect.Int64:
		return "x", true
	case reflect.Uint64:
		return "t", true
	case reflect.Float64:
		return "d", true
	case reflect.String:
		return "s", true
	case reflect.Slice, reflect.Array:
		if t.Elem() == dictEntryType {
			return "", false
		}
		if elem, elemOK := signatureOfType(t.Elem()); elemOK {
			return "a" + elem, true
		}
		return "", false
	case reflect.Map:
		key, keyOK := signatureOfType(t.Key())
		if !keyOK || len(key) != 1 || !strings.ContainsRune(basicTypes, rune(key[0])) {
			return "", false
		}
		elem, elemOK := signatureOfType(t.Elem())
		if !elemOK {
			return "", false
		}
		return "a{" + key + elem + "}", true
	default:
		return "", false
	}
}
