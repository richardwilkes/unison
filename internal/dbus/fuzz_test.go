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
	"bytes"
	"reflect"
	"testing"
)

// FuzzDecode checks that arbitrary input never panics and that anything that does decode successfully can be encoded
// again and decoded back to the same message.
func FuzzDecode(f *testing.F) {
	_, hello := helloMessage()
	f.Add(hello)
	f.Add(concat(hello, hello))
	for _, one := range goldenCases() {
		m := NewSignal("/a", "org.a11y.atspi.Event.Object", "Fuzz")
		m.Serial = 1
		m.Sender = ":1.1"
		if err := m.SetBodyWithSignature(one.sig, one.in...); err != nil {
			f.Fatal(err)
		}
		data, err := m.Encode()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		for {
			m, err := Decode(r)
			if err != nil {
				return
			}
			if _, err = m.Args(); err != nil {
				// An invalid body is fine; it just must not be fatal.
				_ = err
			}
			encoded, err := m.Encode()
			if err != nil {
				t.Fatalf("a decoded message failed to encode: %v (%s)", err, m)
			}
			again, err := Decode(bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("a re-encoded message failed to decode: %v (%s)", err, m)
			}
			if !reflect.DeepEqual(m, again) {
				t.Fatalf("a re-encoded message decoded differently: %s vs %s", m, again)
			}
		}
	})
}

// FuzzUnmarshal checks that arbitrary bodies never panic and that unmarshaling then marshaling is stable.
func FuzzUnmarshal(f *testing.F) {
	for _, one := range goldenCases() {
		f.Add(string(one.sig), one.want)
	}
	f.Fuzz(func(t *testing.T, sig string, data []byte) {
		values, err := Unmarshal(Signature(sig), data)
		if err != nil {
			return
		}
		first, err := Marshal(Signature(sig), values...)
		if err != nil {
			t.Fatalf("unmarshaled values failed to marshal: %v (%q)", err, sig)
		}
		values, err = Unmarshal(Signature(sig), first)
		if err != nil {
			t.Fatalf("marshaled values failed to unmarshal: %v (%q)", err, sig)
		}
		second, err := Marshal(Signature(sig), values...)
		if err != nil {
			t.Fatalf("unmarshaled values failed to marshal a second time: %v (%q)", err, sig)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("marshaling is not stable for %q", sig)
		}
	})
}
