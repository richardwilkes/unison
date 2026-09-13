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
	"strings"
	"sync"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

const (
	brokenInterface = "org.example.Broken"
	greetMember     = "Greet"
	getMember       = "Get"
	nameProperty    = "Name"
	countProperty   = "Count"
	missingMember   = "Missing"
	changedName     = "changed"
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

	reply := b.call(testPath, propertiesInterface, "Set", "ssv", testInterface, nameProperty,
		Variant{Sig: "s", Value: changedName})
	c.Equal(TypeMethodReturn, reply.Type)
	name, err := obj.getName()
	c.NoError(err)
	c.Equal(changedName, name)
	values = replyValues(t, b.call(testPath, propertiesInterface, getMember, "ss", testInterface, nameProperty))
	c.Equal([]any{Variant{Sig: "s", Value: changedName}}, values)

	values = replyValues(t, b.call(testPath, propertiesInterface, "GetAll", "s", testInterface))
	c.Equal([]any{Dict{
		{Key: nameProperty, Value: Variant{Sig: "s", Value: changedName}},
		{Key: countProperty, Value: Variant{Sig: "i", Value: int32(7)}},
	}}, values)

	// An interface with no properties still answers with an empty dictionary.
	values = replyValues(t, b.call(testPath, propertiesInterface, "GetAll", "s", "org.example.Nothing"))
	c.Equal([]any{Dict{}}, values)
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
			member: "Set",
			sig:    "ssv",
			want:   PropertyReadOnly,
			args:   []any{testInterface, countProperty, Variant{Sig: "i", Value: int32(1)}},
		},
		{
			member: "Set",
			sig:    "ssv",
			want:   InvalidArgs,
			args:   []any{testInterface, nameProperty, Variant{Sig: "i", Value: int32(1)}},
		},
		{
			member: "Set",
			sig:    "ssv",
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
	c.Contains(reply.AsError().Message, "cannot be read")
}

func TestEmitPropertiesChanged(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	b := newFakeBus(t)
	b.client.EmitPropertiesChanged(testPath, testInterface, map[string]Variant{
		nameProperty:  {Sig: "s", Value: changedName},
		countProperty: {Sig: "i", Value: int32(3)},
	}, []string{"Other"})
	signal := b.nextSignal()
	c.Equal(propertiesInterface, signal.Interface)
	c.Equal("PropertiesChanged", signal.Member)
	c.Equal(testPath, signal.Path)
	args, err := signal.Args()
	c.NoError(err)
	c.Equal([]any{
		testInterface,
		Dict{ // Sorted by name, so that the encoding is reproducible
			{Key: countProperty, Value: Variant{Sig: "i", Value: int32(3)}},
			{Key: nameProperty, Value: Variant{Sig: "s", Value: changedName}},
		},
		[]string{"Other"},
	}, args)

	b.client.EmitPropertiesChanged(testPath, testInterface, nil, nil)
	args, err = b.nextSignal().Args()
	c.NoError(err)
	c.Equal([]any{testInterface, Dict{}, []string{}}, args)
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
	xml, ok := values[0].(string)
	c.True(ok)
	c.Equal(smallIntrospection, xml)
}
