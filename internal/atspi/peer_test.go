// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"bufio"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/unison/internal/dbus"
)

const (
	// testTimeout is how long a test waits for something that should happen at once. It only bounds how long a failure
	// takes to report, so it is generous enough to ride out the multi-second stalls a loaded CI runner can suffer.
	testTimeout = 10 * time.Second
	// testBusName is the unique name the fake bus hands out in reply to Hello, which is the name that appears in every
	// object reference the adapter builds.
	testBusName = ":1.42"
	// testPeerName is the unique name of the fake peer itself.
	testPeerName = ":1.7"
	// testDesktopPath is the path of the desktop object the fake registry hands back from Embed.
	testDesktopPath dbus.ObjectPath = "/org/a11y/atspi/accessible/desktop"
	// dbusPath is the path of the bus's own object.
	dbusPath dbus.ObjectPath = "/org/freedesktop/DBus"
)

// testPeer is the other end of a connection under test. It answers the calls the bus itself implements, answers the
// calls that whichever service it is standing in for implements, records everything else, and lets a test call into the
// objects the connection exports.
type testPeer struct {
	t       *testing.T
	c       check.Checker
	client  *dbus.Conn
	side    net.Conn
	in      *bufio.Reader
	answer  func(p *testPeer, msg *dbus.Message) bool
	calls   chan *dbus.Message
	signals chan *dbus.Message
	replies map[uint32]chan *dbus.Message
	rules   []string
	enabled atomic.Bool
	serial  uint32
	mu      sync.Mutex
}

// newTestPeer creates a connection under test whose peer answers with the given function, which returns false for a call
// it does not handle so that the test can deal with it instead. Both ends are closed when the test finishes.
func newTestPeer(t *testing.T, answer func(p *testPeer, msg *dbus.Message) bool) *testPeer {
	t.Helper()
	c := check.New(t)
	clientSide, peerSide := net.Pipe()
	client, err := dbus.Connect(clientSide)
	c.NoError(err)
	p := &testPeer{
		t:       t,
		c:       c,
		client:  client,
		side:    peerSide,
		in:      bufio.NewReader(peerSide),
		answer:  answer,
		calls:   make(chan *dbus.Message, 64),
		signals: make(chan *dbus.Message, 256),
		replies: make(map[uint32]chan *dbus.Message),
	}
	go p.run()
	t.Cleanup(func() {
		client.Close()
		xio.CloseIgnoringErrors(peerSide)
	})
	c.NoError(client.Hello())
	return p
}

// run reads messages from the connection under test until it goes away.
func (p *testPeer) run() {
	for {
		msg, err := dbus.Decode(p.in)
		if err != nil {
			return
		}
		switch msg.Type {
		case dbus.TypeMethodCall:
			if msg.Path == dbusPath {
				p.answerBus(msg)
				continue
			}
			if p.answer != nil && p.answer(p, msg) {
				continue
			}
			p.deliver(p.calls, msg)
		case dbus.TypeMethodReturn, dbus.TypeError:
			p.mu.Lock()
			ch := p.replies[msg.ReplySerial]
			delete(p.replies, msg.ReplySerial)
			p.mu.Unlock()
			if ch != nil {
				ch <- msg
			}
		case dbus.TypeSignal:
			p.deliver(p.signals, msg)
		default:
		}
	}
}

// deliver hands a message to the test without ever blocking the peer's reader.
func (p *testPeer) deliver(ch chan *dbus.Message, msg *dbus.Message) {
	select {
	case ch <- msg:
	default:
		p.t.Errorf("the fake peer had nowhere to put %s", msg)
	}
}

// answerBus answers the calls the bus itself implements.
func (p *testPeer) answerBus(msg *dbus.Message) {
	switch msg.Member {
	case "Hello":
		p.replyTo(msg, "s", testBusName)
	case "AddMatch", "RemoveMatch":
		args, err := msg.Args()
		if err != nil || len(args) != 1 {
			p.errorTo(msg, dbus.InvalidArgs, "a match rule is required")
			return
		}
		rule, ok := args[0].(string)
		if !ok {
			p.errorTo(msg, dbus.InvalidArgs, "a match rule is required")
			return
		}
		p.mu.Lock()
		if msg.Member == "AddMatch" {
			p.rules = append(p.rules, rule)
		} else if i := slices.Index(p.rules, rule); i >= 0 {
			p.rules = slices.Delete(p.rules, i, i+1)
		}
		p.mu.Unlock()
		p.replyTo(msg, "")
	default:
		p.errorTo(msg, dbus.UnknownMethod, msg.Member)
	}
}

// matchRules returns the match rules the connection under test has asked the bus for.
func (p *testPeer) matchRules() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.rules)
}

// write sends a message to the connection under test.
func (p *testPeer) write(msg *dbus.Message) {
	p.mu.Lock()
	p.serial++
	msg.Serial = p.serial
	p.mu.Unlock()
	data, err := msg.Encode()
	if err != nil {
		p.t.Errorf("the fake peer could not encode %s: %v", msg, err)
		return
	}
	if _, err = p.side.Write(data); err != nil {
		return // The connection under test has gone away, which some tests do on purpose
	}
}

// replyTo answers a method call the connection under test made.
func (p *testPeer) replyTo(call *dbus.Message, sig dbus.Signature, args ...any) {
	reply := dbus.NewReply(call)
	if len(args) != 0 {
		p.c.NoError(reply.SetBodyWithSignature(sig, args...))
	}
	p.write(reply)
}

// errorTo answers a method call the connection under test made with an error.
func (p *testPeer) errorTo(call *dbus.Message, name, message string) {
	p.write(dbus.NewError(call, name, message))
}

// emit sends a signal to the connection under test.
func (p *testPeer) emit(path dbus.ObjectPath, iface, member string, sig dbus.Signature, args ...any) {
	msg := dbus.NewSignal(path, iface, member)
	msg.Sender = testPeerName
	if len(args) != 0 {
		p.c.NoError(msg.SetBodyWithSignature(sig, args...))
	}
	p.write(msg)
}

// call makes a method call on an object the connection under test exports and returns its reply.
func (p *testPeer) call(path dbus.ObjectPath, iface, member string, sig dbus.Signature, args ...any) *dbus.Message {
	p.t.Helper()
	msg := dbus.NewMethodCall("", path, iface, member)
	msg.Sender = testPeerName
	if len(args) != 0 {
		p.c.NoError(msg.SetBodyWithSignature(sig, args...))
	}
	ch := make(chan *dbus.Message, 1)
	p.mu.Lock()
	p.serial++
	msg.Serial = p.serial
	p.replies[msg.Serial] = ch
	p.mu.Unlock()
	data, err := msg.Encode()
	p.c.NoError(err)
	_, err = p.side.Write(data)
	p.c.NoError(err)
	select {
	case reply := <-ch:
		return reply
	case <-time.After(testTimeout):
		p.t.Fatalf("timed out waiting for a reply to %s", msg)
		return nil
	}
}

// getProperty reads one property of an object the connection under test exports, unwrapping the variant it comes back in.
func (p *testPeer) getProperty(path dbus.ObjectPath, iface, name string) any {
	p.t.Helper()
	args := p.replyValues(p.call(path, dbusPropertiesInterface, "Get", "ss", iface, name))
	p.c.Equal(1, len(args))
	variant, ok := args[0].(dbus.Variant)
	p.c.True(ok, "a property must come back as a variant")
	return variant.Value
}

// setProperty writes one property of an object the connection under test exports, returning the reply so that a test can
// look at the error a refusal produces.
func (p *testPeer) setProperty(path dbus.ObjectPath, iface, name string, value dbus.Variant) *dbus.Message {
	p.t.Helper()
	return p.call(path, dbusPropertiesInterface, "Set", "ssv", iface, name, value)
}

// replyValues returns the values in a reply, failing the test if it is an error reply instead.
func (p *testPeer) replyValues(reply *dbus.Message) []any {
	p.t.Helper()
	p.c.Equal(dbus.TypeMethodReturn, reply.Type, "unexpected reply: %s", reply)
	args, err := reply.Args()
	p.c.NoError(err)
	return args
}

// nextCall returns the next method call the connection under test made that the peer did not answer itself.
func (p *testPeer) nextCall() *dbus.Message {
	p.t.Helper()
	select {
	case msg := <-p.calls:
		return msg
	case <-time.After(testTimeout):
		p.t.Fatal("timed out waiting for a method call")
		return nil
	}
}

// registryAnswers stands in for the AT-SPI registry: it lets the application into the accessibility tree and lets it
// leave again.
func registryAnswers(p *testPeer, msg *dbus.Message) bool {
	if msg.Interface != InterfaceSocket {
		return false
	}
	switch msg.Member {
	case "Embed":
		p.deliver(p.calls, msg)
		p.replyTo(msg, objectRefSignature, dbus.ObjectRef{Name: testPeerName, Path: testDesktopPath})
		return true
	case "Unembed":
		p.deliver(p.calls, msg)
		p.replyTo(msg, "")
		return true
	default:
		return false
	}
}

// statusAnswers stands in for the accessibility bus launcher on the session bus, reporting whatever the test has made
// the peer's enabled flag say.
func statusAnswers(p *testPeer, msg *dbus.Message) bool {
	switch {
	case msg.Interface == dbusPropertiesInterface && msg.Member == "Get":
		args, err := msg.Args()
		if err != nil || len(args) != 2 {
			return false
		}
		iface, _ := args[0].(string) //nolint:errcheck // The signature has already been checked
		name, _ := args[1].(string)  //nolint:errcheck // The signature has already been checked
		if iface != StatusInterface || name != isEnabledProperty {
			return false
		}
		p.replyTo(msg, "v", dbus.Variant{Sig: "b", Value: p.enabled.Load()})
		return true
	case msg.Interface == BusInterface && msg.Member == "GetAddress":
		p.replyTo(msg, "s", "unix:path=/tmp/at-spi-bus-for-a-test")
		return true
	default:
		return false
	}
}

// unknownServiceAnswers stands in for a session bus that has no accessibility bus launcher on it at all.
func unknownServiceAnswers(p *testPeer, msg *dbus.Message) bool {
	p.errorTo(msg, dbus.ServiceUnknown, "there is no such service")
	return true
}
