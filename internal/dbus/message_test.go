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
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/richardwilkes/toolbox/v2/check"
)

const (
	testSender      = ":1.42"
	testDestination = ":1.7"
	eventInterface  = "org.a11y.atspi.Event.Object"
)

// withoutWireSize checks that a decoded message recorded the number of bytes it arrived as, or zero when it has no body
// and so keeps nothing of the buffer it was read into, then strips that from it so that it can be compared with the
// message it was built from. The wire size is a property of those bytes rather than of the message, and re-encoding
// drops whatever header fields the decoder did not understand, so it is not part of what a round trip has to preserve;
// see [Message.size].
func withoutWireSize(t *testing.T, m *Message, wire int) *Message {
	t.Helper()
	want := 0
	if len(m.Body) != 0 {
		want = wire
	}
	check.New(t).Equal(want, m.wireSize)
	m.wireSize = 0
	return m
}

func TestEncodeHello(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	m, want := helloMessage()
	c.Equal(128, len(want))
	data, err := m.Encode()
	c.NoError(err)
	c.Equal(want, data)
}

func TestDecodeHello(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	want, data := helloMessage()
	m, err := Decode(bytes.NewReader(data))
	c.NoError(err)
	c.Equal(want, withoutWireSize(t, m, len(data)))
	c.Equal("method call #1 to org.freedesktop.DBus /org/freedesktop/DBus org.freedesktop.DBus.Hello", m.String())
}

func TestMessageRoundTrip(t *testing.T) {
	t.Parallel()
	for _, one := range []*Message{
		func() *Message {
			m := NewMethodCall(busName, busPath, busName, "AddMatch")
			m.Serial = 3
			check.New(t).NoError(m.SetBody("type='signal'"))
			return m
		}(),
		func() *Message {
			m := NewSignal(ObjectPath(atspiRoot), eventInterface, "StateChanged")
			m.Serial = 4
			m.Sender = testSender
			c := check.New(t)
			c.NoError(m.SetBody("focused", int32(1), int32(0), Variant{Sig: "i", Value: int32(0)},
				Dict{{Key: toolkitKey, Value: Variant{Sig: "s", Value: toolkitName}}}))
			c.Equal(Signature("siiv"+propertiesSig), m.Signature)
			return m
		}(),
		func() *Message {
			m := NewReply(&Message{Serial: 5, Sender: testDestination})
			m.Serial = 6
			check.New(t).NoError(m.SetBody([]ObjectRef{{Name: testSender, Path: ObjectPath(atspiRoot)}}))
			return m
		}(),
		func() *Message {
			m := NewError(&Message{Serial: 7, Sender: testDestination}, NotSupported, "no can do")
			m.Serial = 8
			m.Flags = FlagNoReplyExpected | FlagNoAutoStart
			return m
		}(),
		func() *Message {
			m := NewMethodCall(busName, "/", "", "NoInterface")
			m.Serial = 9
			return m
		}(),
	} {
		t.Run(one.Member, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			data, err := one.Encode()
			c.NoError(err)
			decoded, err := Decode(bytes.NewReader(data))
			c.NoError(err)
			c.Equal(one, withoutWireSize(t, decoded, len(data)))
			again, err := decoded.Encode()
			c.NoError(err)
			c.Equal(data, again)
			var args []any
			args, err = decoded.Args()
			c.NoError(err)
			var wantArgs []any
			wantArgs, err = one.Args()
			c.NoError(err)
			c.Equal(wantArgs, args)
		})
	}
}

func TestDecodeReadsOnlyOneMessage(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	_, hello := helloMessage()
	stream := bytes.NewReader(concat(hello, hello, []byte{'x'}))
	for range 2 {
		m, err := Decode(stream)
		c.NoError(err)
		c.Equal("Hello", m.Member)
	}
	remaining, err := io.ReadAll(stream)
	c.NoError(err)
	c.Equal([]byte{'x'}, remaining)
}

func TestDecodeErrors(t *testing.T) {
	t.Parallel()
	_, hello := helloMessage()
	mislabeledEndian := concat(hello) // Little-endian bytes that claim to be big-endian
	mislabeledEndian[0] = 'B'
	badEndian := concat(hello)
	badEndian[0] = '?'
	badVersion := concat(hello)
	badVersion[3] = 2
	zeroSerial := concat(hello)
	copy(zeroSerial[8:12], u32At(0))
	bodyWithoutSignature := concat(hello, pad(8))
	copy(bodyWithoutSignature[4:8], u32At(8))
	bodyLongerThanTheData := concat(hello)
	copy(bodyLongerThanTheData[4:8], u32At(8))
	oversizedBody := concat(hello)
	copy(oversizedBody[4:8], u32At(MaxMessageSize))
	oversizedFields := concat(hello)
	copy(oversizedFields[12:16], u32At(MaxArraySize+1))
	shortFields := concat(hello)
	copy(shortFields[12:16], u32At(109))
	for _, one := range []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "short header", data: hello[:15]},
		{name: "truncated header fields", data: hello[:64]},
		{name: "endianness flag that contradicts the bytes", data: mislabeledEndian},
		{name: "invalid endianness", data: badEndian},
		{name: "unsupported protocol version", data: badVersion},
		{name: "zero serial", data: zeroSerial},
		{name: "body without a signature", data: bodyWithoutSignature},
		{name: "body longer than the data", data: bodyLongerThanTheData},
		{name: "oversized body", data: oversizedBody},
		{name: "oversized header fields", data: oversizedFields},
		{name: "header field array that ends mid-field", data: shortFields},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			_, err := Decode(bytes.NewReader(one.data))
			c.HasError(err)
		})
	}
}

func TestDecodeRejectsMalformedHeaderFields(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		name   string
		fields []any
	}{
		{name: "duplicate path", fields: []any{
			Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}},
			Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/b")}},
			Struct{byte(fieldMember), Variant{Sig: "s", Value: "M"}},
		}},
		{name: "path with the wrong type", fields: []any{
			Struct{byte(fieldPath), Variant{Sig: "s", Value: "/a"}},
			Struct{byte(fieldMember), Variant{Sig: "s", Value: "M"}},
		}},
		{name: "reply serial with the wrong type", fields: []any{
			Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}},
			Struct{byte(fieldMember), Variant{Sig: "s", Value: "M"}},
			Struct{byte(fieldReplySerial), Variant{Sig: "s", Value: "1"}},
		}},
		{name: "zero reply serial", fields: []any{
			Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}},
			Struct{byte(fieldMember), Variant{Sig: "s", Value: "M"}},
			Struct{byte(fieldReplySerial), Variant{Sig: "u", Value: uint32(0)}},
		}},
		{name: "missing member", fields: []any{
			Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}},
		}},
		{name: "invalid member", fields: []any{
			Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}},
			Struct{byte(fieldMember), Variant{Sig: "s", Value: "not.a.member"}},
		}},
		{name: "invalid sender", fields: []any{
			Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}},
			Struct{byte(fieldMember), Variant{Sig: "s", Value: "M"}},
			Struct{byte(fieldSender), Variant{Sig: "s", Value: "no-dots"}},
		}},
		{name: "field code the specification marks invalid", fields: []any{
			Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}},
			Struct{byte(fieldMember), Variant{Sig: "s", Value: "M"}},
			Struct{byte(0), Variant{Sig: "s", Value: "reserved"}},
		}},
		{name: "unix file descriptor count with the wrong type", fields: []any{
			Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}},
			Struct{byte(fieldMember), Variant{Sig: "s", Value: "M"}},
			Struct{byte(fieldUnixFDs), Variant{Sig: "s", Value: "1"}},
		}},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			_, err := Decode(bytes.NewReader(encodeWithFields(t, TypeMethodCall, one.fields)))
			c.HasError(err)
		})
	}
}

func TestDecodeRejectsMessagesMissingRequiredFields(t *testing.T) {
	t.Parallel()
	var (
		path        = Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}}
		iface       = Struct{byte(fieldInterface), Variant{Sig: "s", Value: eventInterface}}
		member      = Struct{byte(fieldMember), Variant{Sig: "s", Value: "M"}}
		replySerial = Struct{byte(fieldReplySerial), Variant{Sig: "u", Value: uint32(1)}}
		errorName   = Struct{byte(fieldErrorName), Variant{Sig: "s", Value: Failed}}
	)
	// Every type but the method call used to reach validateRequiredFields only through Encode, which is the one
	// direction that cannot be driven by a peer; these are the messages that arrive from one.
	for _, one := range []struct {
		name    string
		fields  []any
		msgType Type
	}{
		{name: "method return without a reply serial", msgType: TypeMethodReturn},
		{name: "error without an error name", msgType: TypeError, fields: []any{replySerial}},
		{name: "error without a reply serial", msgType: TypeError, fields: []any{errorName}},
		{name: "signal without an interface", msgType: TypeSignal, fields: []any{path, member}},
		{name: "signal without a path", msgType: TypeSignal, fields: []any{iface, member}},
		{name: "signal without a member", msgType: TypeSignal, fields: []any{path, iface}},
		{name: "method call without a path", msgType: TypeMethodCall, fields: []any{member}},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(bytes.NewReader(encodeWithFields(t, one.msgType, one.fields)))
			check.New(t).HasError(err)
		})
	}
	// The same messages with the fields they require do decode, so each case above is about the missing field rather
	// than about anything else in the message.
	c := check.New(t)
	for _, one := range []struct {
		name    string
		fields  []any
		msgType Type
	}{
		{name: "method return", msgType: TypeMethodReturn, fields: []any{replySerial}},
		{name: "error", msgType: TypeError, fields: []any{replySerial, errorName}},
		{name: "signal", msgType: TypeSignal, fields: []any{path, iface, member}},
		{name: "method call", msgType: TypeMethodCall, fields: []any{path, member}},
	} {
		_, err := Decode(bytes.NewReader(encodeWithFields(t, one.msgType, one.fields)))
		c.NoError(err, one.name)
	}
}

func TestDecodeIgnoresUnknownHeaderFields(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	data := encodeWithFields(t, TypeMethodCall, []any{
		Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}},
		Struct{byte(fieldMember), Variant{Sig: "s", Value: "M"}},
		Struct{byte(fieldUnixFDs), Variant{Sig: "u", Value: uint32(0)}},
		Struct{byte(42), Variant{Sig: "s", Value: "from the future"}},
	})
	m, err := Decode(bytes.NewReader(data))
	c.NoError(err)
	c.Equal(ObjectPath("/a"), m.Path)
	c.Equal("M", m.Member)
}

func TestDecodeRejectsNonNULPaddingBeforeTheBody(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	data := encodeWithFields(t, TypeMethodCall, []any{
		Struct{byte(fieldPath), Variant{Sig: "o", Value: ObjectPath("/a")}},
		Struct{byte(fieldMember), Variant{Sig: "s", Value: "M"}},
	})
	fieldsEnd := fixedHeaderSize + int(binary.LittleEndian.Uint32(data[12:fixedHeaderSize]))
	c.True(fieldsEnd < len(data), "the header fields must not already end on an 8 byte boundary")
	_, err := Decode(bytes.NewReader(data))
	c.NoError(err)
	// The run of padding that puts the body on an 8 byte boundary is the one run the decoder never has to walk over, so
	// it is checked on its own; libdbus rejects the same thing as DBUS_INVALID_ALIGNMENT_PADDING_NOT_NUL.
	data[fieldsEnd] = 0xFF
	_, err = Decode(bytes.NewReader(data))
	c.True(errors.Is(err, errPaddingNotNUL))
}

// encodeWithFields builds a message of the given type whose header field array holds exactly the given fields,
// bypassing the checks that [Message.Encode] applies.
func encodeWithFields(t *testing.T, msgType Type, fields []any) []byte {
	t.Helper()
	c := check.New(t)
	var e encoder
	e.putByte('l')
	e.putByte(byte(msgType))
	e.putByte(0)
	e.putByte(protocolVersion)
	e.putUint32(0)
	e.putUint32(1)
	c.NoError(e.value(headerFieldsSignature, fields))
	e.align(8)
	return e.buf
}

// paddedMember is the member name of the signals that [paddedSignal] builds.
const paddedMember = "Padded"

// paddedSignal encodes a signal that carries padding bytes of the given size in a header field that the specification
// requires the decoder to ignore, plus a body of a few bytes. Nothing ever looks at the padding, but the message keeps
// every byte of it alive: the body is a slice of the buffer the whole message was read into.
func paddedSignal(t *testing.T, serial uint32, padding int) []byte {
	t.Helper()
	return signalWithFields(t, serial, true, ignorableField(make([]byte, padding)))
}

// ignorableField returns a header field whose code is one that the specification requires the decoder to ignore, which
// is what makes it somewhere for a peer to put bytes that no part of the decoded message will ever account for.
// Everything from 32 up is unknown, and only the codes from 1 to 9 mean anything at all.
func ignorableField(value any) any {
	sig, err := SignatureOf(value)
	if err != nil {
		panic(err) // The tests only ever hand this values whose signature can be derived
	}
	return Struct{byte(42), Variant{Sig: sig, Value: value}}
}

// signalWithFields encodes a signal whose header field array holds the fields that every signal needs plus whatever
// extra ones it is given, bypassing the checks that [Message.Encode] applies. A signal without a body keeps nothing of
// the buffer it was read into, since the body is the only part of a message that aliases that buffer.
func signalWithFields(t *testing.T, serial uint32, withBody bool, extra ...any) []byte {
	t.Helper()
	c := check.New(t)
	var body []byte
	fields := []any{
		Struct{byte(fieldPath), Variant{Sig: "o", Value: testPath}},
		Struct{byte(fieldInterface), Variant{Sig: "s", Value: eventInterface}},
		Struct{byte(fieldMember), Variant{Sig: "s", Value: paddedMember}},
		Struct{byte(fieldSender), Variant{Sig: "s", Value: testDestination}},
	}
	if withBody {
		var err error
		body, err = Marshal("s", "x")
		c.NoError(err)
		fields = append(fields, Struct{byte(fieldSignature), Variant{Sig: "g", Value: Signature("s")}})
	}
	fields = append(fields, extra...)
	var e encoder
	e.putByte('l')
	e.putByte(byte(TypeSignal))
	e.putByte(0)
	e.putByte(protocolVersion)
	e.putUint32(uint32(len(body)))
	e.putUint32(serial)
	c.NoError(e.value(headerFieldsSignature, fields))
	e.align(8)
	return append(e.buf, body...)
}

func TestADecodedMessageIsChargedWhatItKeepsAlive(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	const padding = 1 << 20
	data := paddedSignal(t, 1, padding)
	m, err := Decode(bytes.NewReader(data))
	c.NoError(err)
	c.Equal(paddedMember, m.Member)
	// The decoded parts of the message add up to a few dozen bytes, but its body is a slice of the buffer the whole
	// message was read into, so the megabyte the decoder ignored stays alive for as long as the message does. Charging
	// a queue only what the parts add up to is what let a peer pin megabytes per message while the bound that is meant
	// to stop exactly that counted a few dozen bytes.
	c.True(len(m.Body) < 16, "expected a small body, but it was %d bytes", len(m.Body))
	c.Equal(len(data), m.wireSize)
	c.True(m.size() >= len(data), "expected a message of %d bytes to be charged at least that, but it was charged %d",
		len(data), m.size())
	// A message that was built rather than decoded holds nothing but its own parts, so it is charged for those alone.
	built := NewSignal(testPath, eventInterface, paddedMember)
	c.NoError(built.SetBody("x"))
	c.True(built.size() < 128, "expected a small message to be charged a small size, but it was %d", built.size())
	// So is a message that was decoded but has no body, since the body is the only part of it that aliases the buffer
	// it was read into: charging one the whole megabyte it does not keep alive dropped the later messages that the
	// queue could perfectly well have held.
	bodyless, err := Decode(bytes.NewReader(signalWithFields(t, 2, false, ignorableField(make([]byte, padding)))))
	c.NoError(err)
	c.Equal(paddedMember, bodyless.Member)
	c.Equal(0, len(bodyless.Body))
	c.Equal(0, bodyless.wireSize)
	c.True(bodyless.size() < 128, "expected a bodyless message to be charged a small size, but it was %d",
		bodyless.size())
}

func TestDecodeDoesNotBuildTheValueOfAnUnknownHeaderField(t *testing.T) { // Not parallel: it measures allocation
	c := check.New(t)
	const count = 4096
	empties := make([]any, count)
	for i := range empties {
		empties[i] = []string{}
	}
	// A field whose code the specification requires the decoder to ignore is the cheapest place on the wire for a peer
	// to put an array of empty containers, since the value is thrown away the moment it has been built: four bytes
	// apiece bought hundreds, and the 64 MiB such a field may hold scaled to gigabytes on the reader goroutine. The
	// value is stepped over instead, so all that a message like this costs is the buffer it was read into.
	data := signalWithFields(t, 1, true, ignorableField(empties))
	var (
		m   *Message
		err error
	)
	allocated := bytesAllocated(func() { m, err = Decode(bytes.NewReader(data)) })
	c.NoError(err)
	c.Equal(paddedMember, m.Member)
	c.True(allocated < 4*uint64(len(data)), "a %d byte message allocated %d bytes", len(data), allocated)
}

// malformedBodySignal encodes a signal whose body declares a string one byte longer than the body actually holds, so
// that the body does not match the signature the message declares. bigEndian selects the byte order of the whole
// message, which is swapped while the body is still well formed, since [swapEndianness] walks the values themselves.
func malformedBodySignal(t *testing.T, serial uint32, bigEndian bool) []byte {
	t.Helper()
	c := check.New(t)
	m := NewSignal(testPath, eventInterface, malformedMember)
	m.Serial = serial
	m.Sender = testDestination
	c.NoError(m.SetBody("hello"))
	data, err := m.Encode()
	c.NoError(err)
	bodyStart := len(data) - len(m.Body)
	var order binary.ByteOrder = binary.LittleEndian
	if bigEndian {
		data = toBigEndian(data, "s")
		order = binary.BigEndian
	}
	order.PutUint32(data[bodyStart:], uint32(len("hello")+1))
	return data
}

// malformedMember is the member name of the signals that [malformedBodySignal] builds.
const malformedMember = "Malformed"

// interfacelessSignal encodes a signal that carries PATH and MEMBER but no INTERFACE, bypassing the checks that
// [Message.Encode] applies. Every byte of it is correctly framed, yet no signal may be missing its interface, so it is
// the cheapest thing a peer can send that is wrong about itself and nothing else; see [ErrMessageContent].
func interfacelessSignal(t *testing.T, serial uint32) []byte {
	t.Helper()
	c := check.New(t)
	var e encoder
	e.putByte('l')
	e.putByte(byte(TypeSignal))
	e.putByte(0)
	e.putByte(protocolVersion)
	e.putUint32(0)
	e.putUint32(serial)
	c.NoError(e.value(headerFieldsSignature, []any{
		Struct{byte(fieldPath), Variant{Sig: "o", Value: testPath}},
		Struct{byte(fieldMember), Variant{Sig: "s", Value: paddedMember}},
		Struct{byte(fieldSender), Variant{Sig: "s", Value: testDestination}},
	}))
	e.align(8)
	return e.buf
}

func TestDecodeSeparatesAMessageFaultFromAStreamFault(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	_, hello := helloMessage()
	// A message that arrived whole but is not one the specification permits leaves the reader exactly where the next
	// message starts, so reading can carry on. Treating it as a fault in the stream, which is what any error from
	// Decode used to amount to, cost every pending call and every subscription over a single message.
	for _, one := range []struct {
		name string
		text string
		data []byte
	}{
		{
			name: "signal with no interface",
			text: "a signal requires a path, an interface and a member",
			data: interfacelessSignal(t, 1),
		},
		{
			name: "repeated header field",
			text: "appears more than once",
			data: signalWithFields(t, 1, false, Struct{byte(fieldMember), Variant{Sig: "s", Value: paddedMember}}),
		},
	} {
		stream := bytes.NewReader(concat(one.data, hello))
		_, err := Decode(stream)
		c.HasError(err, one.name)
		c.True(errors.Is(err, ErrMessageContent), "%s: %v", one.name, err)
		c.Contains(err.Error(), one.text, one.name)
		m, err := Decode(stream)
		c.NoError(err, one.name)
		c.Equal("Hello", m.Member, one.name) // The next message was found exactly where it should have been
	}
	// A fault in the framing is not confined to one message: the length that says where the next message starts is
	// part of what could not be believed, so there is nothing left to resynchronize to.
	for _, one := range []struct {
		name string
		data []byte
	}{
		{name: "end of stream", data: nil},
		{name: "endianness flag", data: []byte("Xnot a message at all, whatever else it may be")},
		{name: "truncated", data: hello[:100]},
		{name: "protocol version", data: append([]byte{'l', byte(TypeSignal), 0, 2}, hello[4:]...)},
	} {
		_, err := Decode(bytes.NewReader(one.data))
		c.HasError(err, one.name)
		c.False(errors.Is(err, ErrMessageContent), "%s: %v", one.name, err)
	}
}

func TestDecodeMalformedBodyIsAPerMessageError(t *testing.T) {
	t.Parallel()
	// A body that does not match the signature its message declares is a fault in that one message. Converting a
	// big-endian body inside Decode made it a fault in the stream instead, since a connection's reader ends the
	// connection for any error Decode reports: one such message from a big-endian peer cost every message that would
	// have followed it, while the identical message from a little-endian peer cost only itself.
	for _, one := range []struct {
		name      string
		bigEndian bool
	}{
		{name: "little-endian"},
		{name: "big-endian", bigEndian: true},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			m, err := Decode(bytes.NewReader(malformedBodySignal(t, 1, one.bigEndian)))
			c.NoError(err)
			c.Equal(malformedMember, m.Member)
			_, err = m.Args()
			c.HasError(err)
			// A big-endian body that will not convert is the one body that cannot be written out again, since what a
			// Message holds is always the little-endian encoding; the same fault is reported rather than bytes that
			// claim to be little-endian and are not.
			_, err = m.Encode()
			if one.bigEndian {
				c.HasError(err)
			} else {
				c.NoError(err)
			}
		})
	}
}

func TestEncodeErrors(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		msg  *Message
		name string
	}{
		{name: "no type", msg: &Message{Serial: 1, Path: "/a", Member: "M"}},
		{name: "no serial", msg: &Message{Type: TypeMethodCall, Path: "/a", Member: "M"}},
		{name: "no path", msg: &Message{Type: TypeMethodCall, Serial: 1, Member: "M"}},
		{name: "no member", msg: &Message{Type: TypeMethodCall, Serial: 1, Path: "/a"}},
		{name: "signal without an interface", msg: &Message{Type: TypeSignal, Serial: 1, Path: "/a", Member: "M"}},
		{name: "reply without a reply serial", msg: &Message{Type: TypeMethodReturn, Serial: 1}},
		{
			name: "error without a name",
			msg:  &Message{Type: TypeError, Serial: 1, ReplySerial: 2},
		},
		{
			name: "invalid path",
			msg:  &Message{Type: TypeMethodCall, Serial: 1, Path: "a", Member: "M"},
		},
		{
			name: "invalid interface",
			msg:  &Message{Type: TypeMethodCall, Serial: 1, Path: "/a", Interface: "single", Member: "M"},
		},
		{
			name: "interface element starting with a digit",
			msg:  &Message{Type: TypeMethodCall, Serial: 1, Path: "/a", Interface: "a.1b", Member: "M"},
		},
		{
			name: "interface with a hyphen",
			msg:  &Message{Type: TypeMethodCall, Serial: 1, Path: "/a", Interface: "a.b-c", Member: "M"},
		},
		{
			name: "invalid member",
			msg:  &Message{Type: TypeMethodCall, Serial: 1, Path: "/a", Member: "M.N"},
		},
		{
			name: "invalid destination",
			msg:  &Message{Type: TypeMethodCall, Serial: 1, Path: "/a", Member: "M", Destination: "nodots"},
		},
		{
			name: "invalid error name",
			msg:  &Message{Type: TypeError, Serial: 1, ReplySerial: 2, ErrorName: "nodots"},
		},
		{
			name: "body without a signature",
			msg:  &Message{Type: TypeMethodCall, Serial: 1, Path: "/a", Member: "M", Body: []byte{1}},
		},
		{
			name: "signature without a body",
			msg:  &Message{Type: TypeMethodCall, Serial: 1, Path: "/a", Member: "M", Signature: "y"},
		},
		{
			name: "invalid signature",
			msg:  &Message{Type: TypeMethodCall, Serial: 1, Path: "/a", Member: "M", Signature: "a", Body: []byte{1}},
		},
		{
			name: "name that is too long",
			msg: &Message{
				Type: TypeMethodCall, Serial: 1, Path: "/a",
				Member: strings.Repeat("M", MaxNameLength+1),
			},
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			_, err := one.msg.Encode()
			c.HasError(err)
		})
	}
}

func TestEncodeAcceptsUniqueAndWellKnownNames(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	m := NewMethodCall(testDestination, "/a", "org.a11y.atspi.Socket", "Embed")
	m.Serial = 1
	m.Sender = testSender
	_, err := m.Encode()
	c.NoError(err)
	m.Destination = "org.a11y.Bus"
	_, err = m.Encode()
	c.NoError(err)
	m.Destination = ":1.a-b"
	_, err = m.Encode()
	c.NoError(err)
}

func TestSetBodyAndArgs(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	m := NewSignal("/a", eventInterface, "TextChanged")
	m.Serial = 1
	c.NoError(m.SetBody())
	c.Equal(Signature(""), m.Signature)
	args, err := m.Args()
	c.NoError(err)
	c.Equal(0, len(args))
	c.NoError(m.SetBody("insert", int32(3), int32(2), Variant{Sig: "s", Value: "ab"}))
	c.Equal(Signature("siiv"), m.Signature)
	args, err = m.Args()
	c.NoError(err)
	c.Equal([]any{"insert", int32(3), int32(2), Variant{Sig: "s", Value: "ab"}}, args)
	c.HasError(m.SetBody(1))
	c.NoError(m.SetBodyWithSignature("i", 1))
	c.Equal(Signature("i"), m.Signature)
	c.HasError(m.SetBodyWithSignature("ii", int32(1)))
	c.NoError(m.SetBodyWithSignature("", []any{}...))
	c.Equal(Signature(""), m.Signature)
	c.Nil(m.Body)
	m.Signature = "y"
	m.Body = nil
	_, err = m.Args()
	c.HasError(err)
	m.Signature = ""
	m.Body = []byte{1}
	_, err = m.Args()
	c.HasError(err)
}

func TestSetBodyWithAnEmptyArray(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	m := NewReply(&Message{Serial: 1})
	m.Serial = 2
	c.HasError(m.SetBody([]any{}))
	c.NoError(m.SetBody(Array{Elem: "(so)"}))
	c.Equal(Signature("a(so)"), m.Signature)
	args, err := m.Args()
	c.NoError(err)
	c.Equal([]any{[]ObjectRef{}}, args)
}

func TestNewErrorAndAsError(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	call := NewMethodCall(busName, "/a", busName, "M")
	call.Serial = 11
	call.Sender = testSender
	m := NewError(call, InvalidArgs, "bad")
	c.Equal(TypeError, m.Type)
	c.Equal(uint32(11), m.ReplySerial)
	c.Equal(testSender, m.Destination)
	c.Equal(InvalidArgs, m.ErrorName)
	err := m.AsError()
	c.NotNil(err)
	c.Equal(InvalidArgs, err.Name)
	c.Equal("bad", err.Message)
	c.Equal("org.freedesktop.DBus.Error.InvalidArgs: bad", err.Error())
	withoutText := NewError(call, Failed, "")
	c.Equal(Signature(""), withoutText.Signature)
	err = withoutText.AsError()
	c.NotNil(err)
	c.Equal("", err.Message)
	c.Equal(Failed, err.Error())
	c.Nil(call.AsError())
	unnamed := &Message{Type: TypeError}
	c.Equal(Failed, unnamed.AsError().Name)
	c.Equal(ServiceUnknown+": nope", Errorf(ServiceUnknown, "%s", "nope").Error())
}

func TestNewErrorFallsBackToFailed(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	call := NewMethodCall(busName, "/a", busName, "M")
	call.Serial = 31
	call.Sender = testSender
	// An error name that will not encode would be dropped on its way out, leaving the caller with no reply at all, so
	// anything that would not survive validation becomes the generic name instead.
	for _, name := range []string{"", "nodots", "org.example.", "1.bad", "has a space", strings.Repeat("a.b", 200)} {
		m := NewError(call, name, "explanation")
		c.Equal(Failed, m.ErrorName, name)
		m.Serial = 32
		_, err := m.Encode()
		c.NoError(err, name)
		c.Equal("explanation", m.AsError().Message, name)
	}
	c.Equal(NotSupported, NewError(call, NotSupported, "").ErrorName)
}

func TestNewErrorSanitizesItsMessage(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	call := NewMethodCall(busName, "/a", busName, "M")
	call.Serial = 41
	call.Sender = testSender
	// The text comes straight from whoever answered the call, and a D-Bus string holds neither invalid UTF-8 nor a
	// NUL. Dropping the body when it would not marshal left the peer with an error name and nothing else, which is
	// precisely when the human-readable part is worth the most.
	m := NewError(call, Failed, "bad \xff text\x00 here")
	c.Equal(Signature("s"), m.Signature)
	m.Serial = 42
	_, err := m.Encode()
	c.NoError(err)
	message := m.AsError().Message
	c.True(utf8.ValidString(message), "%q is not valid UTF-8", message)
	c.False(strings.Contains(message, "\x00"), "%q still holds a NUL", message)
	c.Contains(message, "bad ")
	c.Contains(message, " text here")
	// Nothing else bounds what a handler passes, since publicErrorMessage only cuts at the first newline, so the text
	// is truncated as well. A three byte rune straddles the cut here, and half of one would be exactly the invalid
	// UTF-8 that was just repaired, so what is left of it is dropped.
	long := NewError(call, Failed, strings.Repeat("€", maxErrorMessageLength))
	message = long.AsError().Message
	c.True(len(message) <= maxErrorMessageLength, "%d bytes survived a limit of %d", len(message),
		maxErrorMessageLength)
	c.True(len(message) > maxErrorMessageLength-3, "%d bytes is less than the limit of %d is worth", len(message),
		maxErrorMessageLength)
	c.True(utf8.ValidString(message), "the truncated message is not valid UTF-8")
	// Text that is nothing but what a D-Bus string cannot hold still produces a body, since a reply that says nothing
	// at all is what this exists to avoid.
	c.Equal(Signature("s"), NewError(call, Failed, "\x00\x00").Signature)
}

func TestNewReply(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	call := NewMethodCall(busName, "/a", busName, "M")
	call.Serial = 21
	call.Sender = testSender
	reply := NewReply(call)
	c.Equal(TypeMethodReturn, reply.Type)
	c.Equal(uint32(21), reply.ReplySerial)
	c.Equal(testSender, reply.Destination)
	c.Equal("method return #0 reply-to #21 to :1.42", reply.String())
}

func TestMessageStringIncludesEveryField(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	m := &Message{
		Type:        TypeError,
		Serial:      2,
		ReplySerial: 1,
		Sender:      testSender,
		Destination: testDestination,
		Path:        "/a",
		Interface:   eventInterface,
		Member:      "M",
		ErrorName:   Failed,
		Signature:   "s",
	}
	c.Equal("error #2 reply-to #1 from :1.42 to :1.7 /a org.a11y.atspi.Event.Object.M "+Failed+" (s)", m.String())
}

func TestMessageStringWithoutAnInterface(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	// A member is only ever separated from an interface by a dot, so a message that has no interface must not be made
	// to look as though its path or its serial were one.
	m := NewMethodCall(busName, "/", "", "NoInterface")
	m.Serial = 3
	c.Equal("method call #3 to org.freedesktop.DBus / NoInterface", m.String())
	c.Equal("method return #4 reply-to #3 M", (&Message{
		Type:        TypeMethodReturn,
		Serial:      4,
		ReplySerial: 3,
		Member:      "M",
	}).String())
}

func TestTypeString(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal("invalid", TypeInvalid.String())
	c.Equal("method call", TypeMethodCall.String())
	c.Equal("method return", TypeMethodReturn.String())
	c.Equal("error", TypeError.String())
	c.Equal("signal", TypeSignal.String())
	c.Equal("type 99", Type(99).String())
}

func TestDecodeUnknownMessageType(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	m := &Message{Type: Type(99), Serial: 1}
	data, err := m.Encode()
	c.NoError(err)
	decoded, err := Decode(bytes.NewReader(data))
	c.NoError(err)
	c.Equal(m, withoutWireSize(t, decoded, len(data)))
}

func TestDecodeDistinguishesEndOfStreamFromTruncation(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	_, hello := helloMessage()
	_, err := Decode(bytes.NewReader(nil))
	c.True(errors.Is(err, io.EOF))
	_, err = Decode(bytes.NewReader(hello[:8]))
	c.True(errors.Is(err, io.ErrUnexpectedEOF))
	_, err = Decode(bytes.NewReader(hello[:100]))
	c.True(errors.Is(err, io.ErrUnexpectedEOF))
}

func TestDecodeBigEndianHeader(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	want, littleEndian := helloMessage()
	data := helloMessageBigEndian()
	c.Equal(128, len(data))
	c.Equal(data, toBigEndian(littleEndian, ""))
	m, err := Decode(bytes.NewReader(data))
	c.NoError(err)
	c.Equal(want, withoutWireSize(t, m, len(data)))
	// Re-encoding a message that arrived big-endian produces the little-endian form, since that is the only encoding
	// this package emits.
	encoded, err := m.Encode()
	c.NoError(err)
	c.Equal(littleEndian, encoded)
}

func TestDecodeBigEndianBody(t *testing.T) {
	t.Parallel()
	for _, one := range goldenCases() {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			m := NewSignal(ObjectPath(atspiRoot), eventInterface, "BigEndian")
			m.Serial = 7
			m.Sender = testSender
			c.NoError(m.SetBodyWithSignature(one.sig, one.in...))
			littleEndian, err := m.Encode()
			c.NoError(err)
			decoded, err := Decode(bytes.NewReader(toBigEndian(littleEndian, one.sig)))
			c.NoError(err)
			// The body is converted as it is decoded, so nothing downstream can tell where the message came from.
			c.Equal(m, decoded)
			args, err := decoded.Args()
			c.NoError(err)
			want := one.out
			if want == nil {
				want = one.in
			}
			c.Equal(want, args)
			encoded, err := decoded.Encode()
			c.NoError(err)
			c.Equal(littleEndian, encoded)
		})
	}
}

func TestDecodeBigEndianRejectsTheSameThingsAsLittleEndian(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	_, hello := helloMessage()
	truncated := toBigEndian(hello, "")
	_, err := Decode(bytes.NewReader(truncated[:100]))
	c.True(errors.Is(err, io.ErrUnexpectedEOF))
	badVersion := toBigEndian(hello, "")
	badVersion[3] = 2
	_, err = Decode(bytes.NewReader(badVersion))
	c.HasError(err)
	oversizedFields := toBigEndian(hello, "")
	copy(oversizedFields[12:16], []byte{0xFF, 0xFF, 0xFF, 0xFF})
	_, err = Decode(bytes.NewReader(oversizedFields))
	c.HasError(err)
}
