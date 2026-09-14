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
	"encoding/xml"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

const (
	brokenInterface = "org.example.Broken"
	greetMember     = "Greet"
	getMember       = "Get"
	getAllMember    = "GetAll"
	setMember       = "Set"
	nameProperty    = "Name"
	countProperty   = "Count"
	missingMember   = "Missing"
	// setPropertySig is the signature of org.freedesktop.DBus.Properties.Set.
	setPropertySig = "ssv"
	changedName    = "changed"

	mistypedInterface = "org.example.Mistyped"
)

// testObject is an object with a method of every interesting shape and a few properties.
type testObject struct {
	deferred chan *Call
	name     string
	ifaces   []*Interface
	mu       sync.Mutex
}

func newTestObject() *testObject {
	o := &testObject{deferred: make(chan *Call, 4), name: "original"}
	o.ifaces = []*Interface{
		{
			Name: testInterface,
			Methods: []*Method{
				{Name: greetMember, In: "s", Out: "s", Handle: o.greet},
				{Name: "Nothing", Handle: func(call *Call) { call.Reply() }},
				{Name: "Children", Out: "a(so)", Handle: func(call *Call) { call.Reply([]ObjectRef{}) }},
				{Name: "Attributes", Out: stringDictSig, Handle: func(call *Call) { call.Reply(Dict{}) }},
				{Name: "Boom", Handle: func(_ *Call) { panic("the handler blew up") }},
				{Name: "Unimplemented"},
				{Name: "Later", Out: "s", Handle: func(call *Call) { o.deferred <- call }},
				{Name: "Derived", Handle: func(call *Call) { call.ReplyWithSignature("", "derived", int32(2)) }},
				{Name: "Explicit", Out: "v", Handle: func(call *Call) { call.ReplyWithSignature(stringDictSig, Dict{}) }},
				{Name: "Silent", Out: "s", Handle: func(call *Call) { call.Reply() }},
			},
			Properties: []*Property{
				{Name: nameProperty, Sig: "s", Get: o.getName, Set: o.setName},
				{Name: countProperty, Sig: "i", Get: func() (any, error) { return int32(7), nil }},
			},
			Signals: []*Signal{{Name: "Greeted", Sig: "s"}},
		},
		{
			Name: brokenInterface,
			Properties: []*Property{
				{Name: "Broken", Sig: "s", Get: func() (any, error) { return nil, errors.New("cannot read that") }},
				{Name: "Refused", Sig: "s", Get: func() (any, error) { return nil, Errorf(NotSupported, "nope") }},
			},
		},
	}
	return o
}

func (o *testObject) Interfaces() []*Interface { return o.ifaces }

func (o *testObject) greet(call *Call) {
	args, err := call.Args()
	if err != nil {
		call.Error(InvalidArgs, err.Error())
		return
	}
	text, ok := args[0].(string)
	if !ok {
		call.Error(InvalidArgs, "a string is required")
		return
	}
	call.Reply("hello " + text)
}

func (o *testObject) getName() (any, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.name, nil
}

func (o *testObject) setName(value any) error {
	text, ok := value.(string)
	if !ok {
		return Errorf(InvalidArgs, "a string is required")
	}
	o.mu.Lock()
	o.name = text
	o.mu.Unlock()
	return nil
}

// exportTestObject exports a [testObject] on the connection under test.
func exportTestObject(t *testing.T, b *fakeBus) *testObject {
	t.Helper()
	obj := newTestObject()
	check.New(t).NoError(b.client.Export(testPath, obj))
	return obj
}

// replyValues returns the values in a reply, failing the test if it is an error reply.
func replyValues(t *testing.T, reply *Message) []any {
	t.Helper()
	c := check.New(t)
	c.Equal(TypeMethodReturn, reply.Type, "unexpected reply: %s", reply)
	args, err := reply.Args()
	c.NoError(err)
	return args
}

func TestExportRequiresAValidPathAndObject(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	c.HasError(b.client.Export("no-slash", newTestObject()))
	c.HasError(b.client.Export(testPath, nil))
	c.HasError(b.client.ExportSubtree("no-slash", func(_ ObjectPath) Object { return nil }))
	c.HasError(b.client.ExportSubtree(testPath, nil))
}

func TestExportedMethodCalls(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	exportTestObject(t, b)

	c.Equal([]any{"hello you"}, replyValues(t, b.call(testPath, testInterface, greetMember, "s", "you")))

	// A method with no reply arguments answers with an empty body.
	reply := b.call(testPath, testInterface, "Nothing", "")
	c.Equal(TypeMethodReturn, reply.Type)
	c.Equal(Signature(""), reply.Signature)

	// The declared out signature lets an empty array and an empty dictionary be returned.
	reply = b.call(testPath, testInterface, "Children", "")
	c.Equal(Signature("a(so)"), reply.Signature)
	c.Equal([]any{[]ObjectRef{}}, replyValues(t, reply))
	reply = b.call(testPath, testInterface, "Attributes", "")
	c.Equal(Signature(stringDictSig), reply.Signature)
	c.Equal([]any{Dict{}}, replyValues(t, reply))

	// A call that leaves the interface out is resolved by member name alone.
	c.Equal([]any{"hello nobody"}, replyValues(t, b.call(testPath, "", greetMember, "s", "nobody")))
}

func TestExportedMethodCallErrors(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	exportTestObject(t, b)
	for i, one := range []struct {
		path   ObjectPath
		iface  string
		member string
		sig    Signature
		want   string
		args   []any
	}{
		{
			path:   "/org/example/nothing",
			iface:  testInterface,
			member: greetMember,
			sig:    "s",
			want:   UnknownObject,
			args:   []any{"x"},
		},
		{
			path:   testPath,
			iface:  "org.example.Missing",
			member: greetMember,
			sig:    "s",
			want:   UnknownInterface,
			args:   []any{"x"},
		},
		{path: testPath, iface: testInterface, member: missingMember, want: UnknownMethod},
		{path: testPath, iface: "", member: missingMember, want: UnknownMethod},
		{
			path:   testPath,
			iface:  testInterface,
			member: greetMember,
			sig:    "i",
			want:   InvalidArgs,
			args:   []any{int32(1)},
		},
		{path: testPath, iface: testInterface, member: greetMember, want: InvalidArgs},
		{path: testPath, iface: testInterface, member: "Unimplemented", want: NotSupported},
		{path: testPath, iface: testInterface, member: "Boom", want: Failed},
		// A handler that answers with nothing at all when its method declares a reply has not answered the call: a
		// METHOD_RETURN with no signature and an empty body is rejected by GDBus and libatspi as having the wrong
		// number of arguments, which tells the caller far less than an error does.
		{path: testPath, iface: testInterface, member: "Silent", want: Failed},
	} {
		reply := b.call(one.path, one.iface, one.member, one.sig, one.args...)
		c.Equal(TypeError, reply.Type, "case %d", i)
		c.Equal(one.want, reply.ErrorName, "case %d", i)
		c.NotNil(reply.AsError(), "case %d", i)
	}
}

func TestExportSubtree(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	const prefix = ObjectPath("/org/example/tree")
	var resolved []ObjectPath
	var mu sync.Mutex
	obj := newTestObject()
	c.NoError(b.client.ExportSubtree(prefix, func(path ObjectPath) Object {
		mu.Lock()
		resolved = append(resolved, path)
		mu.Unlock()
		if strings.HasSuffix(string(path), "/known") {
			return obj
		}
		return nil
	}))

	c.Equal([]any{"hello tree"}, replyValues(t, b.call(prefix+"/known", testInterface, greetMember, "s", "tree")))
	c.Equal(UnknownObject, b.call(prefix+"/unknown", testInterface, greetMember, "s", "x").ErrorName)
	mu.Lock()
	c.Equal([]ObjectPath{prefix + "/known", prefix + "/unknown"}, resolved)
	mu.Unlock()

	// An object exported at an exact path wins over the subtree that covers it.
	exact := newTestObject()
	c.NoError(exact.setName("exact"))
	c.NoError(b.client.Export(prefix+"/unknown", exact))
	c.Equal([]any{"hello direct"}, replyValues(t, b.call(prefix+"/unknown", testInterface, greetMember, "s", "direct")))

	b.client.Unexport(prefix)
	b.client.Unexport(prefix + "/unknown")
	c.Equal(UnknownObject, b.call(prefix+"/known", testInterface, greetMember, "s", "x").ErrorName)
	c.Equal(UnknownObject, b.call(prefix+"/unknown", testInterface, greetMember, "s", "x").ErrorName)
}

func TestReplyFromAnotherGoroutine(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	obj := exportTestObject(t, b)
	go func() {
		call := <-obj.deferred
		call.Reply("answered later")
		call.Error(Failed, "this second answer is ignored")
	}()
	c.Equal([]any{"answered later"}, replyValues(t, b.call(testPath, testInterface, "Later", "")))
}

func TestReplyWithSignature(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	exportTestObject(t, b)
	// An empty signature derives one from the values...
	reply := b.call(testPath, testInterface, "Derived", "")
	c.Equal(Signature("si"), reply.Signature)
	c.Equal([]any{"derived", int32(2)}, replyValues(t, reply))
	// ...while an explicit one replaces whatever the method declared, which is how a handler returns an empty
	// dictionary from a method whose declared reply type cannot describe it.
	reply = b.call(testPath, testInterface, "Explicit", "")
	c.Equal(Signature(stringDictSig), reply.Signature)
	c.Equal([]any{Dict{}}, replyValues(t, reply))
}

func TestNoReplyExpectedSuppressesAnswers(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	exportTestObject(t, b)
	// The specification requires a call that says no reply is expected to get none, whether the answer would have been
	// a value or an error, and whether the error came from the handler or from dispatch itself.
	answered := map[string]func() bool{
		"a reply":              b.callNoReply(testPath, testInterface, greetMember, "s", "quiet"),
		"a panicking handler":  b.callNoReply(testPath, testInterface, "Boom", ""),
		"an unknown method":    b.callNoReply(testPath, testInterface, missingMember, ""),
		"an unknown interface": b.callNoReply(testPath, "org.example.Missing", greetMember, "s", "x"),
		"an unknown object":    b.callNoReply("/org/example/nothing", testInterface, greetMember, "s", "x"),
		"invalid arguments":    b.callNoReply(testPath, testInterface, greetMember, ""),
	}
	// A later call that is answered proves the earlier ones would have been: the dispatcher answers in the order the
	// calls arrived, so anything it was going to send for them has been sent by the time this one comes back.
	c.Equal([]any{"hello after"}, replyValues(t, b.call(testPath, testInterface, greetMember, "s", "after")))
	for name, wasAnswered := range answered {
		c.False(wasAnswered(), "%s was answered", name)
	}
}

func TestErrorRepliesDoNotCarryAStackTrace(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	exportTestObject(t, b)
	// Any peer on the bus can reach a handler that panics, so the reply carries the message alone; the detail, which
	// names our own source files and line numbers, is logged here instead.
	reply := b.call(testPath, testInterface, "Boom", "")
	c.Equal(TypeError, reply.Type)
	c.Equal(Failed, reply.ErrorName)
	message := reply.AsError().Message
	c.Equal("recovered from panic", message)
	c.NotContains(message, "internal/dbus")
	// The same goes for a panic in the object's own Interfaces, which is reachable the same way. Only a subtree can
	// hand out such an object, since Export asks an object what it implements before it accepts it.
	c.NoError(b.client.ExportSubtree(testPath+"/panics", func(_ ObjectPath) Object { return panickingObject{} }))
	reply = b.call(testPath+"/panics", testInterface, greetMember, "s", "x")
	c.Equal(Failed, reply.ErrorName)
	c.Equal("recovered from panic", reply.AsError().Message)
}

// mistypedObject has a property whose getter succeeds but hands back a value that its declared type cannot describe,
// which is the other way a property can fail to be part of a GetAll reply.
type mistypedObject struct{}

func (mistypedObject) Interfaces() []*Interface {
	return []*Interface{
		{
			Name: mistypedInterface,
			Properties: []*Property{
				{Name: "First", Sig: "s", Get: func() (any, error) { return "first", nil }},
				{Name: "Bad", Sig: "i", Get: func() (any, error) { return "not an int", nil }},
				{Name: "Last", Sig: "s", Get: func() (any, error) { return "last", nil }},
			},
		},
	}
}

func TestGetAllLeavesOutAPropertyThatWillNotMarshal(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	const path = ObjectPath("/org/example/mistyped")
	c.NoError(b.client.Export(path, mistypedObject{}))
	// GetAll is the call a client makes to fill its cache, so one property whose value does not match the type it
	// declares costs that property rather than every property the object has.
	c.Equal([]any{Dict{
		{Key: "First", Value: Variant{Sig: "s", Value: "first"}},
		{Key: "Last", Value: Variant{Sig: "s", Value: "last"}},
	}}, replyValues(t, b.call(path, propertiesInterface, getAllMember, "s", mistypedInterface)))
	// Get on the same property still fails, since there is nothing else it could answer.
	c.Equal(Failed, b.call(path, propertiesInterface, getMember, "ss", mistypedInterface, "Bad").ErrorName)
}

func TestPropertiesOnABuiltinInterface(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	exportTestObject(t, b)
	// Every object implements the standard interfaces, so naming one of them is not an unknown interface; none of them
	// has any properties, so asking for one is an unknown property, which is exactly what GetAll already assumes when
	// it answers the same name with an empty dictionary.
	for _, iface := range []string{peerInterface, propertiesInterface, introspectableInterface} {
		reply := b.call(testPath, propertiesInterface, getMember, "ss", iface, nameProperty)
		c.Equal(UnknownProperty, reply.ErrorName, iface)
		reply = b.call(testPath, propertiesInterface, setMember, "ssv", iface, nameProperty,
			Variant{Sig: "s", Value: changedName})
		c.Equal(UnknownProperty, reply.ErrorName, iface)
		c.Equal([]any{Dict{}}, replyValues(t, b.call(testPath, propertiesInterface, getAllMember, "s", iface)), iface)
	}
}

// declaredObject returns whatever interfaces a test gives it, valid or not.
type declaredObject struct {
	ifaces []*Interface
}

func (o declaredObject) Interfaces() []*Interface { return o.ifaces }

// readable is a property getter, for the declarations that need one in order to be about something else.
func readable() (any, error) { return "", nil }

func TestExportRejectsInvalidDeclarations(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	for i, one := range []*Interface{
		{Name: "single"},
		{Name: `org.example.X"><node name="pwned`},
		{Name: testInterface, Methods: []*Method{{Name: "has.a.dot"}}},
		{Name: testInterface, Methods: []*Method{{Name: `M"`}}},
		{Name: testInterface, Methods: []*Method{{Name: "M", In: "a"}}},
		{Name: testInterface, Methods: []*Method{{Name: "M", Out: "(i"}}},
		{Name: testInterface, Signals: []*Signal{{Name: "not a name"}}},
		{Name: testInterface, Signals: []*Signal{{Name: "S", Sig: "h"}}},
		{Name: testInterface, Properties: []*Property{{Name: "P<", Sig: "s", Get: readable}}},
		{Name: testInterface, Properties: []*Property{{Name: "P", Sig: "ss", Get: readable}}},
		{Name: testInterface, Properties: []*Property{{Name: "P", Sig: "", Get: readable}}},
		{Name: testInterface, Properties: []*Property{{Name: "P", Sig: "s"}}}, // Neither readable nor writable
	} {
		c.HasError(b.client.Export(testPath, declaredObject{ifaces: []*Interface{one}}), "case %d: %s", i, one.Name)
	}
	c.NoError(b.client.Export(testPath, declaredObject{ifaces: []*Interface{{
		Name:       testInterface,
		Methods:    []*Method{{Name: "M", In: "s", Out: stringDictSig}},
		Signals:    []*Signal{{Name: "S", Sig: "s"}},
		Properties: []*Property{{Name: "P", Sig: "s", Get: readable}},
	}}}))
}

func TestExportRejectsNilDeclarations(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	// Export promises that the names and signatures an object declares are checked when it is exported rather than
	// being left to fail one call at a time, and a panic part way through that check is neither of those things.
	c.HasError(b.client.Export(testPath, declaredObject{ifaces: []*Interface{nil}}))
	for i, one := range []*Interface{
		{Name: testInterface, Methods: []*Method{nil}},
		{Name: testInterface, Signals: []*Signal{nil}},
		{Name: testInterface, Properties: []*Property{nil}},
	} {
		c.HasError(b.client.Export(testPath, declaredObject{ifaces: []*Interface{one}}), "case %d", i)
	}
}

func TestIntrospectionEscapesWhatItIsGiven(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	const prefix = ObjectPath("/org/example/unchecked")
	// Only Export checks what an object declares, so a resolver may still hand out a name that would end the attribute
	// it lands in and add markup of its own. Escaping is what keeps the document parseable no matter what it describes.
	c.NoError(b.client.ExportSubtree(prefix, func(_ ObjectPath) Object {
		return declaredObject{ifaces: []*Interface{{
			Name:    `org.example.X"><node name="pwned`,
			Methods: []*Method{{Name: `M"`, In: "s"}, {Name: "Bell\a"}},
			Signals: []*Signal{{Name: "S&"}},
			Properties: []*Property{
				{Name: "P<", Sig: "s", Get: readable},
				{Name: "Useless", Sig: "s"},
			},
		}}}
	}))
	values := replyValues(t, b.call(prefix+"/x", introspectableInterface, "Introspect", ""))
	doc, ok := values[0].(string)
	c.True(ok)
	c.Contains(doc, `<interface name="org.example.X&#34;&gt;&lt;node name=&#34;pwned">`)
	c.Contains(doc, `<method name="M&#34;">`)
	c.Contains(doc, `<signal name="S&amp;"/>`)
	c.Contains(doc, `<property name="P&lt;" type="s" access="read"/>`)
	// XML 1.0 forbids most control characters outright, so escaping only the five metacharacters would still leave a
	// document that no parser accepts; they are replaced instead.
	c.Contains(doc, `<method name="Bell`+"\uFFFD"+`"/>`)
	c.NotContains(doc, "Bell\a")
	// A property that can neither be read nor written is left out rather than advertised as one that can be written.
	c.NotContains(doc, "Useless")
	var node struct {
		XMLName  xml.Name `xml:"node"`
		Children []struct {
			Name string `xml:"name,attr"`
		} `xml:"node"`
	}
	c.NoError(xml.Unmarshal([]byte(doc), &node))
	c.Equal(0, len(node.Children), "nothing may inject a node of its own")
}

// unnamedErrorObject answers with errors whose names would keep the reply from being encoded at all.
type unnamedErrorObject struct{}

func (unnamedErrorObject) Interfaces() []*Interface {
	return []*Interface{
		{
			Name: "org.example.Unnamed",
			Methods: []*Method{
				{Name: "NoName", Handle: func(call *Call) { call.failWith(&Error{Message: "no name"}) }},
				{Name: "BadName", Handle: func(call *Call) { call.Error("not a name", "malformed") }},
			},
		},
	}
}

func TestErrorRepliesAlwaysHaveAValidName(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	c.NoError(b.client.Export(testPath, unnamedErrorObject{}))
	// An object that chooses an error name that will not encode used to produce a reply that was logged and dropped on
	// its way out, leaving the caller waiting for an answer that never came.
	for _, one := range []struct {
		member string
		want   string
	}{
		{member: "NoName", want: "no name"},
		{member: "BadName", want: "malformed"},
	} {
		reply := b.call(testPath, "org.example.Unnamed", one.member, "")
		c.Equal(TypeError, reply.Type, one.member)
		c.Equal(Failed, reply.ErrorName, one.member)
		c.Equal(one.want, reply.AsError().Message, one.member)
	}
}

func TestPeerInterface(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	exportTestObject(t, b)
	reply := b.call(testPath, peerInterface, "Ping", "")
	c.Equal(TypeMethodReturn, reply.Type)
	c.Equal(Signature(""), reply.Signature)
	values := replyValues(t, b.call(testPath, peerInterface, "GetMachineId", ""))
	c.Equal(1, len(values))
	id, ok := values[0].(string)
	c.True(ok)
	c.NotEqual("", id)
	c.Equal(machineID(), id)
}

func TestProperties(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	obj := exportTestObject(t, b)

	values := replyValues(t, b.call(testPath, propertiesInterface, getMember, "ss", testInterface, nameProperty))
	c.Equal([]any{Variant{Sig: "s", Value: "original"}}, values)

	reply := b.call(testPath, propertiesInterface, setMember, "ssv", testInterface, nameProperty,
		Variant{Sig: "s", Value: changedName})
	c.Equal(TypeMethodReturn, reply.Type)
	name, err := obj.getName()
	c.NoError(err)
	c.Equal(changedName, name)
	values = replyValues(t, b.call(testPath, propertiesInterface, getMember, "ss", testInterface, nameProperty))
	c.Equal([]any{Variant{Sig: "s", Value: changedName}}, values)

	values = replyValues(t, b.call(testPath, propertiesInterface, getAllMember, "s", testInterface))
	c.Equal([]any{Dict{
		{Key: nameProperty, Value: Variant{Sig: "s", Value: changedName}},
		{Key: countProperty, Value: Variant{Sig: "i", Value: int32(7)}},
	}}, values)

	// An interface that the object does implement, but which has no properties, answers with an empty dictionary,
	// while one it does not implement is an error, just as it is for Get and Set.
	values = replyValues(t, b.call(testPath, propertiesInterface, getAllMember, "s", peerInterface))
	c.Equal([]any{Dict{}}, values)
	c.Equal(UnknownInterface, b.call(testPath, propertiesInterface, getAllMember, "s", "org.example.Nothing").ErrorName)

	// An empty interface name asks for the properties of every interface, and reports a property that no interface has
	// as unknown rather than blaming the interface that was not named.
	values = replyValues(t, b.call(testPath, propertiesInterface, getAllMember, "s", ""))
	c.Equal([]any{Dict{
		{Key: nameProperty, Value: Variant{Sig: "s", Value: changedName}},
		{Key: countProperty, Value: Variant{Sig: "i", Value: int32(7)}},
	}}, values)
	reply = b.call(testPath, propertiesInterface, getMember, "ss", "", missingMember)
	c.Equal(UnknownProperty, reply.ErrorName)
	c.Equal(string(testPath)+" has no "+missingMember+" property", reply.AsError().Message)
	reply = b.call(testPath, propertiesInterface, setMember, "ssv", "", missingMember, Variant{Sig: "s", Value: "x"})
	c.Equal(UnknownProperty, reply.ErrorName)
}

func TestPropertyErrors(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	exportTestObject(t, b)
	for i, one := range []struct {
		member string
		sig    Signature
		want   string
		args   []any
	}{
		{member: getMember, sig: "ss", want: UnknownProperty, args: []any{testInterface, missingMember}},
		{member: getMember, sig: "ss", want: UnknownInterface, args: []any{"org.example.Missing", nameProperty}},
		{member: getMember, sig: "ss", want: Failed, args: []any{brokenInterface, "Broken"}},
		{member: getMember, sig: "ss", want: NotSupported, args: []any{brokenInterface, "Refused"}},
		{
			member: setMember,
			sig:    setPropertySig,
			want:   PropertyReadOnly,
			args:   []any{testInterface, countProperty, Variant{Sig: "i", Value: int32(1)}},
		},
		{
			member: setMember,
			sig:    setPropertySig,
			want:   InvalidArgs,
			args:   []any{testInterface, nameProperty, Variant{Sig: "i", Value: int32(1)}},
		},
		{
			member: setMember,
			sig:    setPropertySig,
			want:   UnknownProperty,
			args:   []any{testInterface, missingMember, Variant{Sig: "s", Value: "x"}},
		},
	} {
		reply := b.call(testPath, propertiesInterface, one.member, one.sig, one.args...)
		c.Equal(TypeError, reply.Type, "case %d", i)
		c.Equal(one.want, reply.ErrorName, "case %d", i)
	}
	reply := b.call(testPath, propertiesInterface, getMember, "ss", brokenInterface, "Broken")
	c.Equal("cannot read that", reply.AsError().Message)

	// A property that can only be written cannot be read.
	c.NoError(b.client.Export(testPath+"/small", smallObject{}))
	reply = b.call(testPath+"/small", propertiesInterface, getMember, "ss", "org.example.Small", "WriteOnly")
	c.Equal(UnknownProperty, reply.ErrorName)
	c.Equal("org.example.Small.WriteOnly cannot be read", reply.AsError().Message)
	// An empty interface name asks for every interface to be searched, so there is no interface to name and the object
	// path stands in for it, exactly as it does when the property itself is missing; without that the message read
	// ".WriteOnly cannot be read".
	reply = b.call(testPath+"/small", propertiesInterface, getMember, "ss", "", "WriteOnly")
	c.Equal(UnknownProperty, reply.ErrorName)
	c.Equal(string(testPath)+"/small.WriteOnly cannot be read", reply.AsError().Message)
	reply = b.call(testPath+"/small", propertiesInterface, setMember, "ssv", "", "ReadOnly",
		Variant{Sig: "s", Value: "x"})
	c.Equal(PropertyReadOnly, reply.ErrorName)
	c.Equal(string(testPath)+"/small.ReadOnly cannot be changed", reply.AsError().Message)
	reply = b.call(testPath+"/small", propertiesInterface, setMember, "ssv", "", "ReadWrite",
		Variant{Sig: "s", Value: "x"})
	c.Equal(InvalidArgs, reply.ErrorName)
	c.Equal(string(testPath)+"/small.ReadWrite is a i, not a s", reply.AsError().Message)
}

// smallObject is an object whose introspection document is short enough to check in full.
type smallObject struct{}

func (smallObject) Interfaces() []*Interface {
	return []*Interface{
		{
			Name: "org.example.Small",
			Methods: []*Method{
				{Name: "Nothing", Handle: func(call *Call) { call.Reply() }},
				{Name: "Echo", In: "s", Out: "s", Handle: func(call *Call) { call.Reply("echo") }},
			},
			Properties: []*Property{
				{Name: "ReadOnly", Sig: "s", Get: func() (any, error) { return "x", nil }},
				{
					Name: "ReadWrite", Sig: "i", Get: func() (any, error) { return int32(1), nil },
					Set: func(_ any) error { return nil },
				},
				{Name: "WriteOnly", Sig: "b", Set: func(_ any) error { return nil }},
			},
			Signals: []*Signal{
				{Name: "Changed", Sig: "sv"},
				{Name: "Bare"},
			},
		},
	}
}

const smallIntrospection = `<!DOCTYPE node PUBLIC "-//freedesktop//DTD D-BUS Object Introspection 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/introspect.dtd">
<node>
  <interface name="org.example.Small">
    <method name="Nothing"/>
    <method name="Echo">
      <arg type="s" direction="in"/>
      <arg type="s" direction="out"/>
    </method>
    <signal name="Changed">
      <arg type="s"/>
      <arg type="v"/>
    </signal>
    <signal name="Bare"/>
    <property name="ReadOnly" type="s" access="read"/>
    <property name="ReadWrite" type="i" access="readwrite"/>
    <property name="WriteOnly" type="b" access="write"/>
  </interface>
  <interface name="org.freedesktop.DBus.Introspectable">
    <method name="Introspect">
      <arg type="s" direction="out"/>
    </method>
  </interface>
  <interface name="org.freedesktop.DBus.Properties">
    <method name="Get">
      <arg type="s" direction="in"/>
      <arg type="s" direction="in"/>
      <arg type="v" direction="out"/>
    </method>
    <method name="GetAll">
      <arg type="s" direction="in"/>
      <arg type="a{sv}" direction="out"/>
    </method>
    <method name="Set">
      <arg type="s" direction="in"/>
      <arg type="s" direction="in"/>
      <arg type="v" direction="in"/>
    </method>
    <signal name="PropertiesChanged">
      <arg type="s"/>
      <arg type="a{sv}"/>
      <arg type="as"/>
    </signal>
  </interface>
  <interface name="org.freedesktop.DBus.Peer">
    <method name="Ping"/>
    <method name="GetMachineId">
      <arg type="s" direction="out"/>
    </method>
  </interface>
  <node name="deeper"/>
  <node name="direct"/>
</node>
`

func TestIntrospect(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	const root = ObjectPath("/org/example/small")
	c.NoError(b.client.Export(root, smallObject{}))
	c.NoError(b.client.Export(root+"/direct", smallObject{}))
	c.NoError(b.client.ExportSubtree(root+"/deeper/below", func(_ ObjectPath) Object { return smallObject{} }))
	c.NoError(b.client.Export("/org/example/elsewhere", smallObject{}))
	values := replyValues(t, b.call(root, introspectableInterface, "Introspect", ""))
	c.Equal(1, len(values))
	doc, ok := values[0].(string)
	c.True(ok)
	c.Equal(smallIntrospection, doc)
}

func TestIntrospectAPathThatOnlyHasChildren(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	c.NoError(b.client.Export("/org/example/deep/down/here", smallObject{}))
	// Nothing is exported at any of the paths above it, which is what every path above a subtree looks like on the
	// real adapter. Introspection is the only way to find what is below, so answering these with UnknownObject left
	// busctl and d-feet unable to walk down to anything at all.
	for _, one := range []struct {
		path  ObjectPath
		child string
	}{
		{path: "/", child: "org"},
		{path: "/org", child: "example"},
		{path: "/org/example", child: "deep"},
		{path: "/org/example/deep", child: "down"},
		{path: "/org/example/deep/down", child: "here"},
	} {
		values := replyValues(t, b.call(one.path, introspectableInterface, "Introspect", ""))
		doc, ok := values[0].(string)
		c.True(ok)
		// A path with no object of its own is described by the list of its children and nothing else: the interfaces
		// it answers are there so that a client can walk down, not because there is anything at the path.
		c.Equal(introspectPrologue+`  <node name="`+one.child+"\"/>\n</node>\n", doc, string(one.path))
	}
	// Peer answers too, since it is about the connection rather than the object...
	c.Equal(TypeMethodReturn, b.call("/org/example", peerInterface, "Ping", "").Type)
	// ...while everything else is honestly reported as not being there.
	c.Equal(UnknownInterface, b.call("/org/example", propertiesInterface, getAllMember, "").ErrorName)
	c.Equal(UnknownMethod, b.call("/org/example", "", greetMember, "").ErrorName)
	// A path with nothing at or below it is still an unknown object, and so is one that only looks like a prefix.
	c.Equal(UnknownObject, b.call("/org/nothing", introspectableInterface, "Introspect", "").ErrorName)
	c.Equal(UnknownObject, b.call("/org/example/deep/down/here/below", introspectableInterface, "Introspect",
		"").ErrorName)
	// The object itself still describes its interfaces, since it has some.
	values := replyValues(t, b.call("/org/example/deep/down/here", introspectableInterface, "Introspect", ""))
	doc, ok := values[0].(string)
	c.True(ok)
	c.Contains(doc, `<interface name="org.example.Small">`)
}

func TestExportRejectsDuplicateDeclarations(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	// Dispatch resolves a duplicate to whichever was declared first while introspection describes both, so neither
	// half of what a duplicate produces is right and it is refused instead.
	c.HasError(b.client.Export(testPath, declaredObject{ifaces: []*Interface{
		{Name: testInterface, Methods: []*Method{{Name: "M", Handle: func(call *Call) { call.Reply() }}}},
		{Name: testInterface, Properties: []*Property{{Name: "P", Sig: "s", Get: readable}}},
	}}))
	for i, one := range []*Interface{
		{Name: testInterface, Methods: []*Method{{Name: "M", In: "s"}, {Name: "M", In: "i"}}},
		{Name: testInterface, Signals: []*Signal{{Name: "S"}, {Name: "S", Sig: "s"}}},
		{Name: testInterface, Properties: []*Property{
			{Name: "P", Sig: "s", Get: readable},
			{Name: "P", Sig: "i", Get: readable},
		}},
	} {
		c.HasError(b.client.Export(testPath, declaredObject{ifaces: []*Interface{one}}), "case %d", i)
	}
	// A method and a signal may share a name, since they are described by different elements and dispatched from
	// different lists.
	c.NoError(b.client.Export(testPath, declaredObject{ifaces: []*Interface{{
		Name:       testInterface,
		Methods:    []*Method{{Name: "Same"}},
		Signals:    []*Signal{{Name: "Same"}},
		Properties: []*Property{{Name: "Same", Sig: "s", Get: readable}},
	}}}))
}

// partialPropertiesObject declares just one member of a standard interface, which is all the specification requires an
// object to do in order to answer that member itself.
type partialPropertiesObject struct{}

func (partialPropertiesObject) Interfaces() []*Interface {
	return []*Interface{
		{
			Name: propertiesInterface,
			Methods: []*Method{
				{Name: getMember, In: "ss", Out: "v", Handle: func(call *Call) {
					call.Reply(Variant{Sig: "s", Value: "mine"})
				}},
			},
		},
	}
}

func TestPartiallyDeclaredStandardInterface(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	const path = ObjectPath("/org/example/partial")
	c.NoError(b.client.Export(path, partialPropertiesObject{}))

	// The declared member wins over the built-in one...
	values := replyValues(t, b.call(path, propertiesInterface, getMember, "ss", testInterface, nameProperty))
	c.Equal([]any{Variant{Sig: "s", Value: "mine"}}, values)
	// ...while the members it left out are still answered by the built-in implementation.
	c.Equal(TypeMethodReturn, b.call(path, propertiesInterface, getAllMember, "s", "").Type)
	c.Equal(TypeError, b.call(path, propertiesInterface, setMember, "ssv", testInterface, nameProperty,
		Variant{Sig: "s", Value: "x"}).Type)

	// Introspection has to describe what dispatch actually does, so the members that fell through to the built-in
	// implementation appear alongside the declared one rather than being left out.
	values = replyValues(t, b.call(path, introspectableInterface, "Introspect", ""))
	doc, ok := values[0].(string)
	c.True(ok)
	c.Equal(1, strings.Count(doc, `<interface name="`+propertiesInterface+`">`))
	for _, member := range []string{getMember, getAllMember, setMember} {
		c.Contains(doc, `<method name="`+member+`">`, member)
	}
	c.Contains(doc, `<signal name="PropertiesChanged">`)
}
