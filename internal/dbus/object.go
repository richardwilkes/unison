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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// The interfaces that every exported object implements without having to declare them.
const (
	introspectableInterface = "org.freedesktop.DBus.Introspectable"
	propertiesInterface     = "org.freedesktop.DBus.Properties"
	peerInterface           = "org.freedesktop.DBus.Peer"
)

// propertiesSignature is the type of a map of property names to values.
const propertiesSignature Signature = "a{sv}"

// introspectPrologue is the header and opening element of an introspection document.
const introspectPrologue = `<!DOCTYPE node PUBLIC "-//freedesktop//DTD D-BUS Object Introspection 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/introspect.dtd">
<node>
`

// Object is something that a connection exports at a path. Its interfaces may be assembled once and returned every
// time, which is what a static object does, or built on demand, which is what an object resolved by
// [Conn.ExportSubtree] typically does. Interfaces is called on the dispatcher goroutine while a call is being answered,
// so it must not block.
//
// The standard org.freedesktop.DBus.Introspectable, org.freedesktop.DBus.Properties and org.freedesktop.DBus.Peer
// interfaces are answered for every object and do not need to be returned. An object that declares one of them anyway
// replaces only the members it declares, since a call is resolved member by member: the rest are still answered by the
// built-in implementation, and introspection describes the two together.
type Object interface {
	Interfaces() []*Interface
}

// Interface is one interface of an [Object].
type Interface struct {
	Name       string
	Methods    []*Method
	Properties []*Property
	Signals    []*Signal
}

// Method is one method of an [Interface]. In and Out are the signatures of the arguments and of the reply; a call whose
// arguments do not match In is refused with an InvalidArgs error. Handle answers the call, either before it returns or
// later from another goroutine; a nil Handle answers NotSupported.
type Method struct {
	Handle func(*Call)
	Name   string
	In     Signature
	Out    Signature
}

// Property is one property of an [Interface]. Sig is the type of the value that Get returns and that Set accepts. A nil
// Set makes the property read-only, which is answered with a PropertyReadOnly error. Both are called on the dispatcher
// goroutine, so neither may block. Returning an [Error] chooses the error name that the caller sees; any other error
// becomes a Failed.
type Property struct {
	Get  func() (any, error)
	Set  func(any) error
	Name string
	Sig  Signature
}

// Signal is one signal that an [Interface] emits. It is only used for introspection, since emitting a signal needs
// nothing more than [Conn.Emit].
type Signal struct {
	Name string
	Sig  Signature
}

// Call is an incoming method call that has been routed to a [Method]. Exactly one of [Call.Reply] and [Call.Error]
// answers it; both may be called from any goroutine, and at any time, so a handler that cannot answer immediately may
// hand the call on and return.
type Call struct {
	// Message is the method call itself, for handlers that need the path, the interface, the member or the sender.
	Message *Message
	conn    *Conn
	obj     Object
	out     Signature
	replied atomic.Bool
}

// Conn returns the connection the call arrived on.
func (call *Call) Conn() *Conn {
	return call.conn
}

// Args unmarshals the arguments of the call. They have already been checked against the method's declared signature.
func (call *Call) Args() ([]any, error) {
	return call.Message.Args()
}

// Reply answers the call, marshaling the values with the method's declared out signature, which lets an empty array or
// dictionary be returned without further ado. Answering a call that has already been answered does nothing.
func (call *Call) Reply(args ...any) {
	call.reply(call.out, args...)
}

// ReplyWithSignature answers the call, marshaling the values with the given signature rather than the method's declared
// one. An empty signature derives the signature from the values.
func (call *Call) ReplyWithSignature(sig Signature, args ...any) {
	call.reply(sig, args...)
}

// reply sends a method return, if the call has not already been answered.
func (call *Call) reply(sig Signature, args ...any) {
	if !call.replied.CompareAndSwap(false, true) {
		return
	}
	if call.Message.Flags&FlagNoReplyExpected != 0 {
		return
	}
	msg := NewReply(call.Message)
	if len(args) != 0 {
		var err error
		if sig == "" {
			err = msg.SetBody(args...)
		} else {
			err = msg.SetBodyWithSignature(sig, args...)
		}
		if err != nil {
			errs.Log(errs.NewWithCause("dbus: unable to marshal a reply", err), "call", call.Message.String())
			call.conn.Enqueue(NewError(call.Message, Failed, err.Error()))
			return
		}
	}
	call.conn.Enqueue(msg)
}

// Error answers the call with an error. name should be one of the standard error names, such as [Failed], or one
// specific to the interface. Answering a call that has already been answered does nothing.
func (call *Call) Error(name, message string) {
	if !call.replied.CompareAndSwap(false, true) {
		return
	}
	if call.Message.Flags&FlagNoReplyExpected != 0 {
		return
	}
	call.conn.Enqueue(NewError(call.Message, name, message))
}

// failWith answers the call with the error an object's own code returned, preserving its name if it chose one.
func (call *Call) failWith(err error) {
	var dbusErr *Error
	if errors.As(err, &dbusErr) {
		call.Error(dbusErr.Name, dbusErr.Message)
		return
	}
	call.Error(Failed, err.Error())
}

// builtinInterfaces are the standard interfaces that every exported object answers. The handlers are method expressions
// rather than closures so that the whole list can be built once. It is filled in by init rather than by an initializer
// because Introspect's handler needs the list in order to describe it, which the compiler sees as a cycle.
var builtinInterfaces []*Interface

func init() {
	builtinInterfaces = []*Interface{
		{
			Name: introspectableInterface,
			Methods: []*Method{
				{Name: "Introspect", Out: "s", Handle: (*Call).introspect},
			},
		},
		{
			Name: propertiesInterface,
			Methods: []*Method{
				{Name: "Get", In: "ss", Out: "v", Handle: (*Call).getProperty},
				{Name: "GetAll", In: "s", Out: propertiesSignature, Handle: (*Call).getAllProperties},
				{Name: "Set", In: "ssv", Handle: (*Call).setProperty},
			},
			Signals: []*Signal{
				{Name: "PropertiesChanged", Sig: "sa{sv}as"},
			},
		},
		{
			Name: peerInterface,
			Methods: []*Method{
				{Name: "Ping", Handle: (*Call).ping},
				{Name: "GetMachineId", Out: "s", Handle: (*Call).getMachineID},
			},
		},
	}
}

// findMethod looks for a method in a list of interfaces. An empty name matches any interface, which is what a call that
// omits the interface asks for. ifaceFound reports whether an interface with that name exists at all, so that a caller
// can tell an unknown interface from an unknown method.
func findMethod(ifaces []*Interface, name, member string) (iface *Interface, method *Method, ifaceFound bool) {
	for _, one := range ifaces {
		if name != "" {
			if one.Name != name {
				continue
			}
			ifaceFound = true
		}
		for _, m := range one.Methods {
			if m.Name == member {
				return one, m, true
			}
		}
	}
	return nil, nil, ifaceFound
}

// hasInterface returns true if one of the interfaces has the given name.
func hasInterface(ifaces []*Interface, name string) bool {
	return interfaceNamed(ifaces, name) != nil
}

// interfaceNamed returns the interface with the given name, or nil if there is none.
func interfaceNamed(ifaces []*Interface, name string) *Interface {
	for _, one := range ifaces {
		if one.Name == name {
			return one
		}
	}
	return nil
}

// findProperty looks for a property in a list of interfaces, with the same conventions as [findMethod].
func findProperty(ifaces []*Interface, name, member string) (prop *Property, ifaceFound bool) {
	for _, one := range ifaces {
		if name != "" {
			if one.Name != name {
				continue
			}
			ifaceFound = true
		}
		for _, p := range one.Properties {
			if p.Name == member {
				return p, true
			}
		}
	}
	return nil, ifaceFound
}

// ping implements org.freedesktop.DBus.Peer.Ping.
func (call *Call) ping() {
	call.Reply()
}

// getMachineID implements org.freedesktop.DBus.Peer.GetMachineId.
func (call *Call) getMachineID() {
	call.Reply(machineID())
}

// machineID returns the D-Bus machine id of the host, which identifies the machine rather than the boot or the session.
// The two standard files are read on the platforms that have them; anywhere else, and on a host that has not been
// provisioned with one, a random id is made up once and used for the life of the process, which is enough for the only
// thing anyone does with it, namely telling one machine's peers from another's.
var machineID = sync.OnceValue(func() string {
	for _, path := range []string{"/var/lib/dbus/machine-id", "/etc/machine-id"} {
		if data, err := os.ReadFile(path); err == nil {
			if id := strings.TrimSpace(string(data)); id != "" {
				return id
			}
		}
	}
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return strings.Repeat("0", 2*len(buf))
	}
	return hex.EncodeToString(buf[:])
})

// stringArgs returns the leading string arguments of a call, answering it with an InvalidArgs error and returning false
// if they are not there. Since the arguments have already been checked against the method's declared signature, this
// only fails if the declaration and the handler disagree.
func (call *Call) stringArgs(args []any, count int) ([]string, bool) {
	if len(args) < count {
		call.Error(InvalidArgs, fmt.Sprintf("%d arguments are required", count))
		return nil, false
	}
	result := make([]string, count)
	for i := range count {
		text, ok := args[i].(string)
		if !ok {
			call.Error(InvalidArgs, fmt.Sprintf("argument %d must be a string", i+1))
			return nil, false
		}
		result[i] = text
	}
	return result, true
}

// getProperty implements org.freedesktop.DBus.Properties.Get.
func (call *Call) getProperty() {
	args, err := call.Args()
	if err != nil {
		call.Error(InvalidArgs, err.Error())
		return
	}
	names, ok := call.stringArgs(args, 2)
	if !ok {
		return
	}
	name, member := names[0], names[1]
	prop, ifaceFound := findProperty(call.obj.Interfaces(), name, member)
	if prop == nil {
		call.propertyError(name, member, !ifaceFound)
		return
	}
	if prop.Get == nil {
		call.Error(UnknownProperty, fmt.Sprintf("%s.%s cannot be read", name, member))
		return
	}
	value, err := prop.Get()
	if err != nil {
		call.failWith(err)
		return
	}
	call.Reply(Variant{Sig: prop.Sig, Value: value})
}

// getAllProperties implements org.freedesktop.DBus.Properties.GetAll. An empty interface name asks for the properties of
// every interface, while one the object does not implement is an UnknownInterface error rather than an empty
// dictionary. A property whose getter fails is left out rather than failing the whole call, which is what the clients
// that use GetAll to fill a cache expect.
func (call *Call) getAllProperties() {
	args, err := call.Args()
	if err != nil {
		call.Error(InvalidArgs, err.Error())
		return
	}
	names, ok := call.stringArgs(args, 1)
	if !ok {
		return
	}
	name := names[0]
	ifaces := call.obj.Interfaces()
	if name != "" && !hasInterface(ifaces, name) && !hasInterface(builtinInterfaces, name) {
		// Answering with an empty dictionary would leave the caller unable to tell "no properties" from "wrong
		// interface", which is the distinction getProperty and setProperty already make.
		call.Error(UnknownInterface, fmt.Sprintf("%s does not implement %s", call.Message.Path, name))
		return
	}
	dict := make(Dict, 0, 8)
	for _, iface := range ifaces {
		if name != "" && iface.Name != name {
			continue
		}
		for _, prop := range iface.Properties {
			if prop.Get == nil {
				continue
			}
			value, getErr := prop.Get()
			if getErr != nil {
				errs.Log(getErr, "path", string(call.Message.Path), "interface", iface.Name, "property", prop.Name)
				continue
			}
			dict = append(dict, DictEntry{Key: prop.Name, Value: Variant{Sig: prop.Sig, Value: value}})
		}
	}
	call.Reply(dict)
}

// setProperty implements org.freedesktop.DBus.Properties.Set.
func (call *Call) setProperty() {
	args, err := call.Args()
	if err != nil {
		call.Error(InvalidArgs, err.Error())
		return
	}
	names, ok := call.stringArgs(args, 2)
	if !ok {
		return
	}
	name, member := names[0], names[1]
	variant, ok := args[2].(Variant)
	if !ok {
		call.Error(InvalidArgs, "the value is not a variant")
		return
	}
	prop, ifaceFound := findProperty(call.obj.Interfaces(), name, member)
	if prop == nil {
		call.propertyError(name, member, !ifaceFound)
		return
	}
	if prop.Set == nil {
		call.Error(PropertyReadOnly, fmt.Sprintf("%s.%s cannot be changed", name, member))
		return
	}
	if variant.Sig != prop.Sig {
		call.Error(InvalidArgs, fmt.Sprintf("%s.%s is a %s, not a %s", name, member, prop.Sig, variant.Sig))
		return
	}
	if err = prop.Set(variant.Value); err != nil {
		call.failWith(err)
		return
	}
	call.Reply()
}

// propertyError answers a property call that named something that does not exist. An empty interface name asks for
// every interface to be searched, so nothing about it can be unknown and only the property can be missing; the error
// then names the object, since there is no interface to name.
func (call *Call) propertyError(name, member string, unknownInterface bool) {
	switch {
	case name == "":
		call.Error(UnknownProperty, fmt.Sprintf("%s has no %s property", call.Message.Path, member))
	case unknownInterface:
		call.Error(UnknownInterface, fmt.Sprintf("%s does not implement %s", call.Message.Path, name))
	default:
		call.Error(UnknownProperty, fmt.Sprintf("%s has no %s property", name, member))
	}
}

// EmitPropertiesChanged emits the org.freedesktop.DBus.Properties.PropertiesChanged signal for an object. changed holds
// the properties whose new values are being announced, and invalidated names the ones whose values have changed but are
// not being sent. Either may be empty. It never blocks.
func (c *Conn) EmitPropertiesChanged(path ObjectPath, iface string, changed map[string]Variant, invalidated []string) {
	dict := make(Dict, 0, len(changed))
	for _, name := range slices.Sorted(maps.Keys(changed)) {
		dict = append(dict, DictEntry{Key: name, Value: changed[name]})
	}
	if invalidated == nil {
		invalidated = []string{}
	}
	c.EmitWithSignature(path, propertiesInterface, "PropertiesChanged", "sa{sv}as", iface, dict, invalidated)
}

// introspect implements org.freedesktop.DBus.Introspectable.Introspect. Nothing that goes into the document needs to be
// escaped: interface, method, property, signal and path element names are all limited to letters, digits, underscores
// and, for the names, dots, and signatures to the type codes.
func (call *Call) introspect() {
	var sb strings.Builder
	sb.WriteString(introspectPrologue)
	own := call.obj.Interfaces()
	for _, iface := range own {
		writeInterfaceXML(&sb, effectiveInterface(iface))
	}
	for _, iface := range builtinInterfaces {
		if !hasInterface(own, iface.Name) {
			writeInterfaceXML(&sb, iface)
		}
	}
	for _, name := range call.conn.childNodes(call.Message.Path) {
		sb.WriteString(`  <node name="` + name + "\"/>\n")
	}
	sb.WriteString("</node>\n")
	call.Reply(sb.String())
}

// effectiveInterface returns the interface as it actually behaves. An object that declares one of the standard
// interfaces replaces only the members it declares, since dispatchMethodCall resolves a call member by member and falls
// through to the built-in implementation for anything the object left out, so those members belong in the introspection
// document as well. An interface that is not one of the standard ones is returned unchanged.
func effectiveInterface(iface *Interface) *Interface {
	builtin := interfaceNamed(builtinInterfaces, iface.Name)
	if builtin == nil {
		return iface
	}
	return &Interface{
		Name:       iface.Name,
		Methods:    withMissingMembers(iface.Methods, builtin.Methods, func(m *Method) string { return m.Name }),
		Properties: withMissingMembers(iface.Properties, builtin.Properties, func(p *Property) string { return p.Name }),
		Signals:    withMissingMembers(iface.Signals, builtin.Signals, func(s *Signal) string { return s.Name }),
	}
}

// withMissingMembers returns own with each member of builtin whose name own does not already use appended to it,
// without modifying own.
func withMissingMembers[T any](own, builtin []T, nameOf func(T) string) []T {
	result := slices.Clip(own)
	for _, one := range builtin {
		if !slices.ContainsFunc(own, func(other T) bool { return nameOf(other) == nameOf(one) }) {
			result = append(result, one)
		}
	}
	return result
}

// writeInterfaceXML writes the introspection document's description of one interface.
func writeInterfaceXML(sb *strings.Builder, iface *Interface) {
	sb.WriteString(`  <interface name="` + iface.Name + "\">\n")
	for _, method := range iface.Methods {
		if method.In == "" && method.Out == "" {
			sb.WriteString(`    <method name="` + method.Name + "\"/>\n")
			continue
		}
		sb.WriteString(`    <method name="` + method.Name + "\">\n")
		writeArgsXML(sb, method.In, "in")
		writeArgsXML(sb, method.Out, "out")
		sb.WriteString("    </method>\n")
	}
	for _, signal := range iface.Signals {
		if signal.Sig == "" {
			sb.WriteString(`    <signal name="` + signal.Name + "\"/>\n")
			continue
		}
		sb.WriteString(`    <signal name="` + signal.Name + "\">\n")
		writeArgsXML(sb, signal.Sig, "")
		sb.WriteString("    </signal>\n")
	}
	for _, prop := range iface.Properties {
		access := "read"
		switch {
		case prop.Get == nil:
			access = "write"
		case prop.Set != nil:
			access = "readwrite"
		}
		sb.WriteString(`    <property name="` + prop.Name + `" type="` + string(prop.Sig) + `" access="` + access +
			"\"/>\n")
	}
	sb.WriteString("  </interface>\n")
}

// writeArgsXML writes one argument element per complete type in the signature. direction is empty for a signal's
// arguments, which have none.
func writeArgsXML(sb *strings.Builder, sig Signature, direction string) {
	types, err := sig.Types()
	if err != nil {
		return // A declared signature that will not parse is a programming error, and there is nothing to say here
	}
	for _, one := range types {
		sb.WriteString(`      <arg type="` + string(one) + `"`)
		if direction != "" {
			sb.WriteString(` direction="` + direction + `"`)
		}
		sb.WriteString("/>\n")
	}
}
