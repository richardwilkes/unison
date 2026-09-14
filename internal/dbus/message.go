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
	// bigEndianBody records that Body still holds the big-endian encoding the peer sent, which is the case only for a
	// body that would not convert; see [Message.convertBody]. [Message.Args] reads it in that encoding, and
	// [Message.Encode] refuses to write it out, so nothing outside this file has to know about it.
	bigEndianBody bool
	// wireSize is how many bytes a decoded message arrived as, which is more than its parts add up to: Body is a slice
	// of the buffer the whole message was read into, so that buffer, header field array and all, stays alive for as
	// long as the message does. It is zero for a message that was built rather than decoded, and for one that was
	// decoded but kept nothing of that buffer. See [Message.size].
	wireSize int
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
// where D-Bus clients look for the human-readable part of an error. A name that is not a valid error name, which
// includes the empty one, becomes [Failed]: a reply that will not encode is dropped rather than sent, leaving the peer
// waiting for an answer that never comes, and a generic error name says far more than that does.
func NewError(call *Message, name, message string) *Message {
	if validateInterfaceName("error name", name) != nil {
		name = Failed
	}
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

// Args unmarshals the body of the message. A body that does not match the signature the message declares is an error
// here, which is where a malformed one belongs: it is a fault in the one message rather than in the stream it arrived
// on, no matter which byte order the peer that sent it chose.
func (m *Message) Args() ([]any, error) {
	if m.Signature == "" {
		if len(m.Body) != 0 {
			return nil, errors.New("dbus: message has a body but no signature")
		}
		return nil, nil
	}
	return unmarshal(m.Signature, m.Body, m.bigEndianBody)
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
		if m.Interface != "" {
			sb.WriteString(".") // Only an interface that is actually there has a member separated from it by a dot
		} else {
			sb.WriteString(" ")
		}
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

// size is roughly how many bytes the message occupies, which is what a connection's queues are bounded by. The body
// dominates everything else a message holds, so the fixed header stands in for the parts of the header that are not
// worth adding up exactly. A decoded message whose body aliases the buffer it arrived in is charged that whole buffer
// rather than the length of its body, since that is what it actually keeps alive: a peer that pads the header field
// array with an ignorable unknown field would otherwise be accounted a few dozen bytes while pinning megabytes, and the
// byte bound on the queues would stop bounding anything. A message with no body keeps none of that buffer, so it is
// charged for its parts alone; see [Decode].
func (m *Message) size() int {
	return fixedHeaderSize + len(m.Path) + len(m.Interface) + len(m.Member) + len(m.ErrorName) + len(m.Destination) +
		len(m.Sender) + len(m.Signature) + max(len(m.Body), m.wireSize)
}

// Encode marshals the message into its wire representation. The header fields are written in the order Path,
// Destination, Interface, Member, ErrorName, ReplySerial, Sender, Signature, which is the order libdbus uses.
func (m *Message) Encode() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	if m.bigEndianBody {
		// The body arrived in the big-endian encoding and would not convert, so there is nothing here that could
		// honestly be written out: the error [Message.Args] reports is reported here too, rather than emitting bytes
		// labeled little-endian that are not.
		if err := m.convertBody(); err != nil {
			return nil, err
		}
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
//
// Either byte order is accepted, since the specification lets every peer choose the one that suits it. A big-endian
// message is converted as it is decoded, so the [Message] that comes back is indistinguishable from one a little-endian
// peer sent: its Body holds the little-endian encoding of the same values, which is what [Message.Args] and
// [Message.Encode] expect and the only form a Message ever holds. A body that will not convert, because it does not
// match the signature the message declares, is left as it arrived rather than failing the decode: a malformed body is a
// fault in the one message, and the caller learns of it from [Message.Args], exactly as it does for a little-endian
// peer's malformed body. Tolerating that rather than converting on demand is what keeps what [Decode] costs where it
// has always been: a little-endian body is never walked, and a big-endian one is walked exactly once.
func Decode(r io.Reader) (*Message, error) {
	var fixed [fixedHeaderSize]byte
	if _, err := io.ReadFull(r, fixed[:]); err != nil {
		return nil, err
	}
	var order binary.ByteOrder = binary.LittleEndian
	var bigEndian bool
	switch fixed[0] {
	case 'l':
	case 'B':
		order, bigEndian = binary.BigEndian, true
	default:
		return nil, fmt.Errorf("dbus: %q is not a valid endianness flag", string(fixed[0]))
	}
	if fixed[3] != protocolVersion {
		return nil, fmt.Errorf("dbus: protocol version %d is not supported", fixed[3])
	}
	bodyLength := order.Uint32(fixed[4:8])
	fieldsLength := order.Uint32(fixed[12:fixedHeaderSize])
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
		Serial: order.Uint32(fixed[8:12]),
	}
	// The header field array starts with its length, which is part of the fixed header.
	d := decoder{data: data, pos: 12, bigEndian: bigEndian}
	if err := m.decodeHeaderFields(&d); err != nil {
		return nil, err
	}
	// The body starts on an 8 byte boundary, and the padding that puts it there must be NUL like every other run of
	// padding, which the decoder has no reason to look at because it stops at the end of the header field array.
	for _, b := range data[fixedHeaderSize+int(fieldsLength) : fixedHeaderSize+int(padded)] {
		if b != 0 {
			return nil, errPaddingNotNUL
		}
	}
	if bodyLength != 0 {
		m.Body = data[fixedHeaderSize+int(padded):]
		// The body is a slice of the buffer the whole message was read into, so that buffer stays alive for as long as
		// the message does; a message with no body keeps none of it. See [Message.size].
		m.wireSize = int(total)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	if bigEndian {
		// A body that will not convert is left in the encoding it arrived in, which [Message.Args] then reads it in.
		if m.convertBody() != nil {
			m.bigEndianBody = true
		}
	}
	return m, nil
}

// convertBody rewrites a body that arrived in the big-endian encoding as the little-endian one, which is the only
// encoding a [Message] holds. It goes through the values rather than swapping bytes in place because only the values
// know where the multi-byte fields are, and re-marshaling them normalizes the padding at the same time.
func (m *Message) convertBody() error {
	if m.Signature == "" || len(m.Body) == 0 {
		return nil
	}
	values, err := unmarshal(m.Signature, m.Body, true)
	if err != nil {
		return err
	}
	body, err := Marshal(m.Signature, values...)
	if err != nil {
		return err
	}
	m.Body = body
	// The buffer the message was read into is no longer referenced by anything, so the body is all that is left to
	// account for; see [Message.size].
	m.wireSize = 0
	return nil
}

// decodeHeaderFields reads the header field array, which is an "a(yv)", and applies each field to the message. The
// array is walked here rather than decoded as one value so that the value of a field whose code the specification
// requires us to ignore can be stepped over instead of built: a peer chooses the type and the length of such a field,
// and a 16 MiB one holding an array of empty containers made decoding a single message allocate a gigabyte. See
// [decoder.skip].
func (m *Message) decodeHeaderFields(d *decoder) error {
	// The array, the structure that each field is and the variant that each of those holds are entered once rather
	// than once per field: the fields are siblings, so one level of each is all that any of them is nested by.
	if err := d.enter('a'); err != nil {
		return err
	}
	defer d.leave('a')
	if err := d.enter('('); err != nil {
		return err
	}
	defer d.leave('(')
	if err := d.enter('v'); err != nil {
		return err
	}
	defer d.leave('v')
	end, err := d.startContainer(8)
	if err != nil {
		return err
	}
	var seen uint32
	for d.pos < end {
		if err = d.align(8); err != nil {
			return err
		}
		var code byte
		if code, err = d.getByte(); err != nil {
			return err
		}
		if code == 0 {
			return errors.New("dbus: header field code 0 is not valid")
		}
		// Only the codes the specification has assigned are worth tracking: everything from 32 up is unknown and
		// ignored, so a repeat of one costs nothing.
		if code < 32 {
			if seen&(1<<code) != 0 {
				return fmt.Errorf("dbus: header field %d appears more than once", code)
			}
			seen |= 1 << code
		}
		var sig Signature
		if sig, err = d.getSignature(); err != nil {
			return err
		}
		if err = sig.ValidateSingle(); err != nil {
			return err
		}
		if code > fieldUnixFDs { // Unknown header fields must be ignored, so nothing is built from their values
			if err = d.skip(sig); err != nil {
				return err
			}
			continue
		}
		var value any
		if value, err = d.value(sig); err != nil {
			return err
		}
		if err = m.applyHeaderField(code, Variant{Sig: sig, Value: value}); err != nil {
			return err
		}
	}
	return d.checkEnd(end)
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
	case fieldUnixFDs:
		// Unix file descriptors are not supported, so the count is ignored, but a field of the wrong type is still a
		// peer that is making things up, exactly as it is for every other field whose type the specification fixes.
		if _, ok := variant.Value.(uint32); !ok {
			return headerFieldTypeError(code, variant.Sig)
		}
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
