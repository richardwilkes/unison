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
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

const (
	testSender      = ":1.42"
	testDestination = ":1.7"
	eventInterface  = "org.a11y.atspi.Event.Object"
)

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
	c.Equal(want, m)
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
			c.Equal(one, decoded)
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
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()
			c := check.New(t)
			_, err := Decode(bytes.NewReader(encodeWithFields(t, one.fields)))
			c.HasError(err)
		})
	}
}

func TestDecodeIgnoresUnknownHeaderFields(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	data := encodeWithFields(t, []any{
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

// encodeWithFields builds a method call whose header field array holds exactly the given fields, bypassing the checks
// that [Message.Encode] applies.
func encodeWithFields(t *testing.T, fields []any) []byte {
	t.Helper()
	c := check.New(t)
	var e encoder
	e.putByte('l')
	e.putByte(byte(TypeMethodCall))
	e.putByte(0)
	e.putByte(protocolVersion)
	e.putUint32(0)
	e.putUint32(1)
	c.NoError(e.value(headerFieldsSignature, fields))
	e.align(8)
	return e.buf
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
	c.Equal(m, decoded)
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
	c.Equal(want, m)
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
