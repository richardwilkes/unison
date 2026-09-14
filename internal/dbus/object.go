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
	"encoding/xml"
	"errors"
	"fmt"
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
	// A handler that answers with no values at all when the method declares a reply is a mismatch, not an empty reply:
	// marshaling nothing against a non-empty signature fails, which is exactly what should happen, since a reply whose
	// body does not match what the method promised is rejected by GDBus and libatspi as having the wrong number of
	// arguments and leaves the caller with no idea why.
	if len(args) != 0 || sig != "" {
		var err error
		if sig == "" {
			err = msg.SetBody(args...)
		} else {
			err = msg.SetBodyWithSignature(sig, args...)
		}
		if err != nil {
			errs.Log(errs.NewWithCause("dbus: unable to marshal a reply", err), "call", call.Message.String())
			call.conn.Enqueue(NewError(call.Message, Failed, publicErrorMessage(err)))
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
	call.Error(Failed, publicErrorMessage(err))
}

// publicErrorMessage returns the part of an error that may be sent to a peer. An error from errs renders as its message
// followed by its call stack, and every path that answers a call with one is reachable by anyone on the bus, so sending
// the whole of it publishes our own source paths and line numbers in a multi-kilobyte string. Everything up to the
// first newline is the message, including whatever wrapped it, and is all that goes out; whoever is answering the call
// logs the rest, which is where it is of some use.
func publicErrorMessage(err error) string {
	message, _, _ := strings.Cut(err.Error(), "\n")
	return message
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
	nodeInterfaces = []*Interface{
		interfaceNamed(builtinInterfaces, introspectableInterface),
		interfaceNamed(builtinInterfaces, peerInterface),
	}
}

// validateInterfaces checks everything an object declares about itself, so that an object that could not be described
// or dispatched is refused when it is exported rather than producing an introspection document that no client can
// parse, a call that can never be answered, or a property that can neither be read nor written.
func validateInterfaces(ifaces []*Interface) error {
	if name := duplicateName(ifaces, func(iface *Interface) string { return iface.Name }); name != "" {
		return fmt.Errorf("dbus: %s is declared more than once", name)
	}
	for _, iface := range ifaces {
		if err := validateInterfaceName("interface name", iface.Name); err != nil {
			return err
		}
		if err := validateMethods(iface); err != nil {
			return err
		}
		if err := validateSignals(iface); err != nil {
			return err
		}
		if err := validateProperties(iface); err != nil {
			return err
		}
	}
	return nil
}

// duplicateName returns the first name that nameOf yields more than once, or an empty string if there is none. A
// duplicate is refused rather than accepted, since neither half of what it would produce is right: dispatch silently
// resolves to whichever was declared first, while the introspection document describes both.
func duplicateName[T any](items []T, nameOf func(T) string) string {
	seen := make(map[string]struct{}, len(items))
	for _, one := range items {
		name := nameOf(one)
		if _, exists := seen[name]; exists {
			return name
		}
		seen[name] = struct{}{}
	}
	return ""
}

// validateMethods checks the names and signatures of one interface's methods.
func validateMethods(iface *Interface) error {
	if name := duplicateName(iface.Methods, func(method *Method) string { return method.Name }); name != "" {
		return fmt.Errorf("dbus: %s declares the method %q more than once", iface.Name, name)
	}
	for _, method := range iface.Methods {
		if validateMemberName(method.Name) != nil {
			return fmt.Errorf("dbus: %s declares the invalid method name %q", iface.Name, method.Name)
		}
		if err := method.In.Validate(); err != nil {
			return fmt.Errorf("dbus: %s.%s declares the invalid argument signature %q", iface.Name, method.Name,
				method.In)
		}
		if err := method.Out.Validate(); err != nil {
			return fmt.Errorf("dbus: %s.%s declares the invalid reply signature %q", iface.Name, method.Name,
				method.Out)
		}
	}
	return nil
}

// validateSignals checks the names and signatures of one interface's signals.
func validateSignals(iface *Interface) error {
	if name := duplicateName(iface.Signals, func(signal *Signal) string { return signal.Name }); name != "" {
		return fmt.Errorf("dbus: %s declares the signal %q more than once", iface.Name, name)
	}
	for _, signal := range iface.Signals {
		if validateMemberName(signal.Name) != nil {
			return fmt.Errorf("dbus: %s declares the invalid signal name %q", iface.Name, signal.Name)
		}
		if err := signal.Sig.Validate(); err != nil {
			return fmt.Errorf("dbus: %s.%s declares the invalid signature %q", iface.Name, signal.Name, signal.Sig)
		}
	}
	return nil
}

// validateProperties checks the names and types of one interface's properties, and that each of them can actually be
// used: one with neither a getter nor a setter could only ever be answered with an error.
func validateProperties(iface *Interface) error {
	if name := duplicateName(iface.Properties, func(prop *Property) string { return prop.Name }); name != "" {
		return fmt.Errorf("dbus: %s declares the property %q more than once", iface.Name, name)
	}
	for _, prop := range iface.Properties {
		if validateMemberName(prop.Name) != nil {
			return fmt.Errorf("dbus: %s declares the invalid property name %q", iface.Name, prop.Name)
		}
		if err := prop.Sig.ValidateSingle(); err != nil {
			return fmt.Errorf("dbus: %s.%s declares the invalid type %q", iface.Name, prop.Name, prop.Sig)
		}
		if prop.Get == nil && prop.Set == nil {
			return fmt.Errorf("dbus: %s.%s can neither be read nor written", iface.Name, prop.Name)
		}
	}
	return nil
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
	ifaces, ok := call.interfaces()
	if !ok {
		return
	}
	prop, ifaceFound := findProperty(ifaces, name, member)
	if prop == nil {
		call.propertyError(name, member, !ifaceFound)
		return
	}
	if prop.Get == nil {
		call.Error(UnknownProperty, call.propertyName(name, member)+" cannot be read")
		return
	}
	value, err := prop.Get()
	if err != nil {
		call.failWith(err)
		return
	}
	call.Reply(Variant{Sig: prop.Sig, Value: value})
}

// getAllProperties implements org.freedesktop.DBus.Properties.GetAll. An empty interface name asks for the properties
// of every interface, while one the object does not implement is an UnknownInterface error rather than an empty
// dictionary. A property whose getter fails, or whose value does not match the type it declares, is left out rather
// than failing the whole call, which is what the clients that use GetAll to fill a cache expect.
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
	ifaces, ok := call.interfaces()
	if !ok {
		return
	}
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
	// A getter that succeeds but hands back something its declared type cannot describe would take the whole reply
	// down with it, losing every sibling property along with the one that is wrong, so the dictionary is marshaled
	// here: the fast path pays one encode, and only a dictionary that will not encode is picked over entry by entry.
	if _, err = Marshal(propertiesSignature, dict); err != nil {
		dict = call.marshalableProperties(dict)
	}
	call.Reply(dict)
}

// marshalableProperties returns the entries of a property dictionary that can actually be marshaled, logging the ones
// that cannot, which are the ones whose value does not match the type their property declares.
func (call *Call) marshalableProperties(dict Dict) Dict {
	result := make(Dict, 0, len(dict))
	for _, entry := range dict {
		if _, err := Marshal(propertiesSignature, Dict{entry}); err != nil {
			errs.Log(errs.NewWithCause("dbus: unable to marshal a property", err), "path",
				string(call.Message.Path), "property", entry.Key)
			continue
		}
		result = append(result, entry)
	}
	return result
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
	// The declared "ssv" signature has already been matched against the call, so the value is always there; the guard
	// is what keeps a later change to that declaration from turning this into a panic on the dispatcher goroutine.
	if len(args) < 3 {
		call.Error(InvalidArgs, "3 arguments are required")
		return
	}
	variant, ok := args[2].(Variant)
	if !ok {
		call.Error(InvalidArgs, "the value is not a variant")
		return
	}
	ifaces, ok := call.interfaces()
	if !ok {
		return
	}
	prop, ifaceFound := findProperty(ifaces, name, member)
	if prop == nil {
		call.propertyError(name, member, !ifaceFound)
		return
	}
	if prop.Set == nil {
		call.Error(PropertyReadOnly, call.propertyName(name, member)+" cannot be changed")
		return
	}
	if variant.Sig != prop.Sig {
		call.Error(InvalidArgs, fmt.Sprintf("%s is a %s, not a %s", call.propertyName(name, member), prop.Sig,
			variant.Sig))
		return
	}
	if err = prop.Set(variant.Value); err != nil {
		call.failWith(err)
		return
	}
	call.Reply()
}

// propertyName names a property for an error message. An empty interface name asks for every interface to be searched,
// so there is no interface to name and the object path stands in for it, exactly as [Call.propertyError] does; without
// it a property that the caller reached without naming an interface is reported as ".WriteOnly cannot be read".
func (call *Call) propertyName(iface, member string) string {
	if iface == "" {
		return string(call.Message.Path) + "." + member
	}
	return iface + "." + member
}

// propertyError answers a property call that named something that does not exist. An empty interface name asks for
// every interface to be searched, so nothing about it can be unknown and only the property can be missing; the error
// then names the object, since there is no interface to name. One of the standard interfaces that every object
// implements is not unknown either, even though the object's own list does not hold it: it simply has no properties,
// so it answers UnknownProperty, which is what any other property-less interface would say and what getAllProperties
// already assumes for the same name.
func (call *Call) propertyError(name, member string, unknownInterface bool) {
	switch {
	case name == "":
		call.Error(UnknownProperty, fmt.Sprintf("%s has no %s property", call.Message.Path, member))
	case unknownInterface && !hasInterface(builtinInterfaces, name):
		call.Error(UnknownInterface, fmt.Sprintf("%s does not implement %s", call.Message.Path, name))
	default:
		call.Error(UnknownProperty, fmt.Sprintf("%s has no %s property", name, member))
	}
}

// introspect implements org.freedesktop.DBus.Introspectable.Introspect. [Conn.Export] refuses an object whose names or
// signatures would need escaping, and path elements are limited to letters, digits and underscores by
// [ObjectPath.Validate], but an object handed out by a [Conn.ExportSubtree] resolver is never seen until it is used, so
// everything that goes into the document is escaped as well: a document that cannot be parsed would be worse than one
// with an odd name in it.
func (call *Call) introspect() {
	own, ok := call.interfaces()
	if !ok {
		return
	}
	var sb strings.Builder
	sb.WriteString(introspectPrologue)
	// A placeholder for a path that has objects only below it is described by that list of children alone: it has no
	// interfaces of its own, and the two it answers exist only so that a client can walk down to the real objects.
	if _, placeholder := call.obj.(nodeObject); !placeholder {
		for _, iface := range own {
			writeInterfaceXML(&sb, effectiveInterface(iface))
		}
		for _, iface := range builtinInterfaces {
			if !hasInterface(own, iface.Name) {
				writeInterfaceXML(&sb, iface)
			}
		}
	}
	for _, name := range call.conn.childNodes(call.Message.Path) {
		sb.WriteString(`  <node name="` + escapeXML(name) + "\"/>\n")
	}
	sb.WriteString("</node>\n")
	call.Reply(sb.String())
}

// nodeObject stands in for a path that has no object of its own but does have objects exported below it, which is what
// every path above a subtree looks like: with only /org/a11y/atspi/accessible exported, /, /org, /org/a11y and
// /org/a11y/atspi are all such paths, and answering them with UnknownObject leaves busctl and d-feet unable to walk
// down to anything at all. It answers only Introspectable, whose document is the list of its children, and Peer, which
// is about the connection rather than the object; anything else is honestly reported as not being there.
type nodeObject struct{}

// Interfaces implements [Object].
func (nodeObject) Interfaces() []*Interface { return nil }

// nodeInterfaces are the standard interfaces that a [nodeObject] answers.
var nodeInterfaces []*Interface

// builtins returns the standard interfaces the call's object answers, which is all of them for a real object and only
// the ones that make sense for a placeholder.
func (call *Call) builtins() []*Interface {
	if _, placeholder := call.obj.(nodeObject); placeholder {
		return nodeInterfaces
	}
	return builtinInterfaces
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

// escapeXML returns the text as it may appear inside an XML attribute value, so that no name or signature can end an
// attribute early and inject markup of its own. It is [xml.EscapeText] rather than a replacer of the five
// metacharacters because XML 1.0 forbids most control characters outright: escaping only the metacharacters would
// leave a name holding one of those in a document that no parser will accept, and only [Conn.Export] rejects such a
// name, so an object handed out by a [Conn.ExportSubtree] resolver could still produce one. Those are replaced with
// U+FFFD instead.
func escapeXML(text string) string {
	var sb strings.Builder
	if err := xml.EscapeText(&sb, []byte(text)); err != nil {
		return "" // A strings.Builder never fails to be written to, but a name that will not escape has no place here
	}
	return sb.String()
}

// writeInterfaceXML writes the introspection document's description of one interface.
func writeInterfaceXML(sb *strings.Builder, iface *Interface) {
	sb.WriteString(`  <interface name="` + escapeXML(iface.Name) + "\">\n")
	for _, method := range iface.Methods {
		name := escapeXML(method.Name)
		if method.In == "" && method.Out == "" {
			sb.WriteString(`    <method name="` + name + "\"/>\n")
			continue
		}
		sb.WriteString(`    <method name="` + name + "\">\n")
		writeArgsXML(sb, method.In, "in")
		writeArgsXML(sb, method.Out, "out")
		sb.WriteString("    </method>\n")
	}
	for _, signal := range iface.Signals {
		name := escapeXML(signal.Name)
		if signal.Sig == "" {
			sb.WriteString(`    <signal name="` + name + "\"/>\n")
			continue
		}
		sb.WriteString(`    <signal name="` + name + "\">\n")
		writeArgsXML(sb, signal.Sig, "")
		sb.WriteString("    </signal>\n")
	}
	for _, prop := range iface.Properties {
		access := "read"
		switch {
		case prop.Get == nil && prop.Set == nil:
			// Export refuses such a property, so only one resolved by a subtree can get this far. Advertising it as
			// writable would promise a write that then fails with PropertyReadOnly, so it is left out instead.
			continue
		case prop.Get == nil:
			access = "write"
		case prop.Set != nil:
			access = "readwrite"
		}
		sb.WriteString(`    <property name="` + escapeXML(prop.Name) + `" type="` + escapeXML(string(prop.Sig)) +
			`" access="` + access + "\"/>\n")
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
		sb.WriteString(`      <arg type="` + escapeXML(string(one)) + `"`)
		if direction != "" {
			sb.WriteString(` direction="` + direction + `"`)
		}
		sb.WriteString("/>\n")
	}
}
