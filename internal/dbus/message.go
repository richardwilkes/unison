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
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Type is the kind of a message.
type Type byte

// Possible values for [Type]. Other values may legitimately appear on the wire: the specification requires unknown
// message types to be ignored rather than treated as an error, so they are decoded rather than rejected.
const (
	TypeInvalid Type = iota
	TypeMethodCall
	TypeMethodReturn
	TypeError
	TypeSignal
)

// String implements fmt.Stringer.
func (t Type) String() string {
	switch t {
	case TypeInvalid:
		return "invalid"
	case TypeMethodCall:
		return "method call"
	case TypeMethodReturn:
		return "method return"
	case TypeError:
		return "error"
	case TypeSignal:
		return "signal"
	default:
		return "type " + strconv.Itoa(int(t))
	}
}

// Flags alter how the bus and the receiver treat a message.
type Flags byte

// Possible values for [Flags].
const (
	// FlagNoReplyExpected tells the receiver of a method call not to send a reply.
	FlagNoReplyExpected Flags = 1 << iota
	// FlagNoAutoStart tells the bus not to launch an owner for the destination if none is running.
	FlagNoAutoStart
	// FlagAllowInteractiveAuthorization permits the receiver to prompt the user before acting.
	FlagAllowInteractiveAuthorization
)

// The header field codes defined by the specification.
const (
	fieldPath = 1 + iota
	fieldInterface
	fieldMember
	fieldErrorName
	fieldReplySerial
	fieldDestination
	fieldSender
	fieldSignature
	fieldUnixFDs
)

// protocolVersion is the only version of the protocol that exists.
const protocolVersion = 1

// fixedHeaderSize is the size of the part of the header that precedes the header field array.
const fixedHeaderSize = 16

// headerFieldsSignature is the type of the header field array.
const headerFieldsSignature Signature = "a(yv)"

// Message is a single D-Bus message. Which of the fields are required depends on the [Type]: a method call needs Path
// and Member, a signal needs Path, Interface and Member, a method return needs ReplySerial, and an error needs
// ErrorName and ReplySerial. Serial is filled in by the connection when a message is sent. Body holds the marshaled
// arguments described by Signature; use [Message.SetBody] and [Message.Args] rather than setting it directly.
type Message struct {
	Path        ObjectPath
	Interface   string
	Member      string
	ErrorName   string
	Destination string
	Sender      string
	Signature   Signature
	Body        []byte
	Serial      uint32
	ReplySerial uint32
	Type        Type
	Flags       Flags
}

// NewMethodCall creates a method call message. An empty interface is permitted, though it is only unambiguous when the
// destination exports the member on a single interface.
func NewMethodCall(destination string, path ObjectPath, iface, member string) *Message {
	return &Message{
		Type:        TypeMethodCall,
		Path:        path,
		Interface:   iface,
		Member:      member,
		Destination: destination,
	}
}

// NewSignal creates a signal message. Signals are normally broadcast, so Destination is left empty.
func NewSignal(path ObjectPath, iface, member string) *Message {
	return &Message{
		Type:      TypeSignal,
		Path:      path,
		Interface: iface,
		Member:    member,
	}
}

// NewReply creates a reply to a method call.
func NewReply(call *Message) *Message {
	return &Message{
		Type:        TypeMethodReturn,
		Destination: call.Sender,
		ReplySerial: call.Serial,
	}
}

// NewError creates an error reply to a method call. If message is not empty, it becomes the body of the reply, which is
// where D-Bus clients look for the human-readable part of an error.
func NewError(call *Message, name, message string) *Message {
	m := &Message{
		Type:        TypeError,
		ErrorName:   name,
		Destination: call.Sender,
		ReplySerial: call.Serial,
	}
	if message != "" {
		if body, err := Marshal("s", message); err == nil {
			m.Signature = "s"
			m.Body = body
		}
	}
	return m
}

// SetBody marshals args into the body of the message, deriving the signature from the values. Values whose signature
// cannot be derived, such as an untyped integer or an empty []any, are rejected; see [SignatureOf] and
// [Message.SetBodyWithSignature].
func (m *Message) SetBody(args ...any) error {
	if len(args) == 0 {
		m.Signature = ""
		m.Body = nil
		return nil
	}
	sig, err := SignatureOf(args...)
	if err != nil {
		return err
	}
	return m.SetBodyWithSignature(sig, args...)
}

// SetBodyWithSignature marshals args into the body of the message using the given signature, which must describe
// exactly the values passed.
func (m *Message) SetBodyWithSignature(sig Signature, args ...any) error {
	body, err := Marshal(sig, args...)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		m.Signature = ""
		m.Body = nil
		return nil
	}
	m.Signature = sig
	m.Body = body
	return nil
}

// Args unmarshals the body of the message.
func (m *Message) Args() ([]any, error) {
	if m.Signature == "" {
		if len(m.Body) != 0 {
			return nil, errors.New("dbus: message has a body but no signature")
		}
		return nil, nil
	}
	return Unmarshal(m.Signature, m.Body)
}

// AsError converts an error message into an [Error], returning nil if the message is not an error message. The first
// value in the body, if it is a string, becomes the message text.
func (m *Message) AsError() *Error {
	if m.Type != TypeError {
		return nil
	}
	e := &Error{Name: m.ErrorName}
	if e.Name == "" {
		e.Name = Failed
	}
	if strings.HasPrefix(string(m.Signature), "s") {
		if args, err := m.Args(); err == nil && len(args) > 0 {
			if text, ok := args[0].(string); ok {
				e.Message = text
			}
		}
	}
	return e
}

// String implements fmt.Stringer, producing a one line summary suitable for logging.
func (m *Message) String() string {
	var sb strings.Builder
	sb.WriteString(m.Type.String())
	sb.WriteString(" #")
	sb.WriteString(strconv.FormatUint(uint64(m.Serial), 10))
	if m.ReplySerial != 0 {
		sb.WriteString(" reply-to #")
		sb.WriteString(strconv.FormatUint(uint64(m.ReplySerial), 10))
	}
	if m.Sender != "" {
		sb.WriteString(" from ")
		sb.WriteString(m.Sender)
	}
	if m.Destination != "" {
		sb.WriteString(" to ")
		sb.WriteString(m.Destination)
	}
	if m.Path != "" {
		sb.WriteString(" ")
		sb.WriteString(string(m.Path))
	}
	if m.Interface != "" {
		sb.WriteString(" ")
		sb.WriteString(m.Interface)
	}
	if m.Member != "" {
		sb.WriteString(".")
		sb.WriteString(m.Member)
	}
	if m.ErrorName != "" {
		sb.WriteString(" ")
		sb.WriteString(m.ErrorName)
	}
	if m.Signature != "" {
		sb.WriteString(" (")
		sb.WriteString(string(m.Signature))
		sb.WriteString(")")
	}
	return sb.String()
}

// Encode marshals the message into its wire representation. The header fields are written in the order Path,
// Destination, Interface, Member, ErrorName, ReplySerial, Sender, Signature, which is the order libdbus uses.
func (m *Message) Encode() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	var e encoder
	e.putByte('l') // Little-endian; see the package documentation
	e.putByte(byte(m.Type))
	e.putByte(byte(m.Flags))
	e.putByte(protocolVersion)
	e.putUint32(uint32(len(m.Body)))
	e.putUint32(m.Serial)
	if err := e.value(headerFieldsSignature, m.headerFields()); err != nil {
		return nil, err
	}
	e.align(8)
	if len(e.buf)+len(m.Body) > MaxMessageSize {
		return nil, fmt.Errorf("dbus: message of %d bytes exceeds the %d byte limit", len(e.buf)+len(m.Body),
			MaxMessageSize)
	}
	e.buf = append(e.buf, m.Body...)
	return e.buf, nil
}

func (m *Message) headerFields() []any {
	fields := make([]any, 0, 8)
	add := func(code byte, sig Signature, v any) {
		fields = append(fields, Struct{code, Variant{Sig: sig, Value: v}})
	}
	if m.Path != "" {
		add(fieldPath, "o", m.Path)
	}
	if m.Destination != "" {
		add(fieldDestination, "s", m.Destination)
	}
	if m.Interface != "" {
		add(fieldInterface, "s", m.Interface)
	}
	if m.Member != "" {
		add(fieldMember, "s", m.Member)
	}
	if m.ErrorName != "" {
		add(fieldErrorName, "s", m.ErrorName)
	}
	if m.ReplySerial != 0 {
		add(fieldReplySerial, "u", m.ReplySerial)
	}
	if m.Sender != "" {
		add(fieldSender, "s", m.Sender)
	}
	if m.Signature != "" {
		add(fieldSignature, "g", m.Signature)
	}
	return fields
}

// validate checks everything that both the encoder and the decoder require of a message, so that any message that can
// be decoded can also be encoded again.
func (m *Message) validate() error {
	if m.Type == TypeInvalid {
		return errors.New("dbus: message type is missing")
	}
	if m.Serial == 0 {
		return errors.New("dbus: message serial is missing")
	}
	if err := m.Signature.Validate(); err != nil {
		return err
	}
	if m.Signature == "" && len(m.Body) != 0 {
		return errors.New("dbus: message has a body but no signature")
	}
	if m.Signature != "" && len(m.Body) == 0 {
		return errors.New("dbus: message has a signature but no body")
	}
	if m.Path != "" {
		if err := m.Path.Validate(); err != nil {
			return err
		}
	}
	if m.Interface != "" {
		if err := validateInterfaceName("interface name", m.Interface); err != nil {
			return err
		}
	}
	if m.ErrorName != "" {
		if err := validateInterfaceName("error name", m.ErrorName); err != nil {
			return err
		}
	}
	if m.Member != "" {
		if err := validateMemberName(m.Member); err != nil {
			return err
		}
	}
	if m.Destination != "" {
		if err := validateBusName("destination", m.Destination); err != nil {
			return err
		}
	}
	if m.Sender != "" {
		if err := validateBusName("sender", m.Sender); err != nil {
			return err
		}
	}
	return m.validateRequiredFields()
}

func (m *Message) validateRequiredFields() error {
	switch m.Type {
	case TypeMethodCall:
		if m.Path == "" || m.Member == "" {
			return errors.New("dbus: a method call requires a path and a member")
		}
	case TypeMethodReturn:
		if m.ReplySerial == 0 {
			return errors.New("dbus: a method return requires a reply serial")
		}
	case TypeError:
		if m.ErrorName == "" || m.ReplySerial == 0 {
			return errors.New("dbus: an error requires an error name and a reply serial")
		}
	case TypeSignal:
		if m.Path == "" || m.Interface == "" || m.Member == "" {
			return errors.New("dbus: a signal requires a path, an interface and a member")
		}
	default: // An unknown message type has no required fields, since nothing here knows what it means
	}
	return nil
}

// Decode reads one message from r. It reads exactly as many bytes as the message occupies, so r may be a stream
// carrying more messages. A reader that has no more data returns [io.EOF], which is how a peer closing the connection
// cleanly presents itself, while a message that stops part way through returns [io.ErrUnexpectedEOF].
func Decode(r io.Reader) (*Message, error) {
	var fixed [fixedHeaderSize]byte
	if _, err := io.ReadFull(r, fixed[:]); err != nil {
		return nil, err
	}
	switch fixed[0] {
	case 'l':
	case 'B':
		return nil, errors.New("dbus: big-endian messages are not supported")
	default:
		return nil, fmt.Errorf("dbus: %q is not a valid endianness flag", string(fixed[0]))
	}
	if fixed[3] != protocolVersion {
		return nil, fmt.Errorf("dbus: protocol version %d is not supported", fixed[3])
	}
	bodyLength := binary.LittleEndian.Uint32(fixed[4:8])
	fieldsLength := binary.LittleEndian.Uint32(fixed[12:fixedHeaderSize])
	if fieldsLength > MaxArraySize {
		return nil, fmt.Errorf("dbus: header fields of %d bytes exceed the %d byte limit", fieldsLength, MaxArraySize)
	}
	padded := (fieldsLength + 7) & ^uint32(7)
	total := uint64(fixedHeaderSize) + uint64(padded) + uint64(bodyLength)
	if total > MaxMessageSize {
		return nil, fmt.Errorf("dbus: message of %d bytes exceeds the %d byte limit", total, MaxMessageSize)
	}
	var buf bytes.Buffer
	buf.Grow(min(int(total), 64*1024))
	buf.Write(fixed[:])
	if remaining := int64(total) - fixedHeaderSize; remaining > 0 {
		if _, err := io.CopyN(&buf, r, remaining); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
	}
	data := buf.Bytes()
	m := &Message{
		Type:   Type(fixed[1]),
		Flags:  Flags(fixed[2]),
		Serial: binary.LittleEndian.Uint32(fixed[8:12]),
	}
	d := decoder{data: data, pos: 12} // The header field array starts with its length, which is part of the header
	fields, err := d.value(headerFieldsSignature)
	if err != nil {
		return nil, err
	}
	list, ok := fields.([]any)
	if !ok {
		return nil, errors.New("dbus: malformed header fields")
	}
	if err = m.applyHeaderFields(list); err != nil {
		return nil, err
	}
	if bodyLength != 0 {
		m.Body = data[fixedHeaderSize+int(padded):]
	}
	if err = m.validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Message) applyHeaderFields(fields []any) error {
	var seen uint32
	for _, one := range fields {
		st, ok := one.(Struct)
		if !ok || len(st) != 2 {
			return errors.New("dbus: malformed header field")
		}
		code, ok := st[0].(byte)
		if !ok {
			return errors.New("dbus: malformed header field code")
		}
		variant, ok := st[1].(Variant)
		if !ok {
			return errors.New("dbus: malformed header field value")
		}
		if code > 0 && code < 32 {
			if seen&(1<<code) != 0 {
				return fmt.Errorf("dbus: header field %d appears more than once", code)
			}
			seen |= 1 << code
		}
		if err := m.applyHeaderField(code, variant); err != nil {
			return err
		}
	}
	return nil
}

func (m *Message) applyHeaderField(code byte, variant Variant) error {
	switch code {
	case fieldPath:
		p, ok := variant.Value.(ObjectPath)
		if !ok {
			return headerFieldTypeError(code, variant.Sig)
		}
		m.Path = p
	case fieldInterface, fieldMember, fieldErrorName, fieldDestination, fieldSender:
		s, ok := variant.Value.(string)
		if !ok {
			return headerFieldTypeError(code, variant.Sig)
		}
		switch code {
		case fieldInterface:
			m.Interface = s
		case fieldMember:
			m.Member = s
		case fieldErrorName:
			m.ErrorName = s
		case fieldDestination:
			m.Destination = s
		default:
			m.Sender = s
		}
	case fieldReplySerial:
		n, ok := variant.Value.(uint32)
		if !ok {
			return headerFieldTypeError(code, variant.Sig)
		}
		if n == 0 {
			return errors.New("dbus: reply serial is zero")
		}
		m.ReplySerial = n
	case fieldSignature:
		s, ok := variant.Value.(Signature)
		if !ok {
			return headerFieldTypeError(code, variant.Sig)
		}
		m.Signature = s
	case fieldUnixFDs: // Unix file descriptors are not supported, but the count itself is harmless
	default: // Unknown header fields must be ignored
	}
	return nil
}

func headerFieldTypeError(code byte, sig Signature) error {
	return fmt.Errorf("dbus: header field %d has the wrong type (%q)", code, sig)
}

// validateInterfaceName validates an interface or error name, which share the same rules. kind names what is being
// validated, for the error message.
func validateInterfaceName(kind, name string) error {
	if len(name) > MaxNameLength {
		return fmt.Errorf("dbus: %s is longer than %d bytes", kind, MaxNameLength)
	}
	elements := strings.Split(name, ".")
	if len(elements) < 2 {
		return fmt.Errorf("dbus: %s %q has fewer than two elements", kind, name)
	}
	for _, element := range elements {
		if element == "" {
			return fmt.Errorf("dbus: %s %q has an empty element", kind, name)
		}
		if element[0] >= '0' && element[0] <= '9' {
			return fmt.Errorf("dbus: %s %q has an element starting with a digit", kind, name)
		}
		for i := 0; i < len(element); i++ {
			if !isNameChar(element[i]) {
				return fmt.Errorf("dbus: %s %q contains the invalid character %q", kind, name, string(element[i]))
			}
		}
	}
	return nil
}

func validateMemberName(name string) error {
	if name == "" {
		return errors.New("dbus: member name is empty")
	}
	if len(name) > MaxNameLength {
		return fmt.Errorf("dbus: member name is longer than %d bytes", MaxNameLength)
	}
	if name[0] >= '0' && name[0] <= '9' {
		return fmt.Errorf("dbus: member name %q starts with a digit", name)
	}
	for i := 0; i < len(name); i++ {
		if !isNameChar(name[i]) {
			return fmt.Errorf("dbus: member name %q contains the invalid character %q", name, string(name[i]))
		}
	}
	return nil
}

// validateBusName validates a well-known or unique bus name. kind names what is being validated, for the error message.
func validateBusName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("dbus: %s is empty", kind)
	}
	if len(name) > MaxNameLength {
		return fmt.Errorf("dbus: %s is longer than %d bytes", kind, MaxNameLength)
	}
	unique := name[0] == ':'
	elements := strings.Split(name, ".")
	if unique {
		elements[0] = elements[0][1:]
	}
	if len(elements) < 2 {
		return fmt.Errorf("dbus: %s %q has fewer than two elements", kind, name)
	}
	for _, element := range elements {
		if element == "" {
			return fmt.Errorf("dbus: %s %q has an empty element", kind, name)
		}
		if !unique && element[0] >= '0' && element[0] <= '9' {
			return fmt.Errorf("dbus: %s %q has an element starting with a digit", kind, name)
		}
		for i := 0; i < len(element); i++ {
			if c := element[i]; !isNameChar(c) && c != '-' {
				return fmt.Errorf("dbus: %s %q contains the invalid character %q", kind, name, string(c))
			}
		}
	}
	return nil
}
