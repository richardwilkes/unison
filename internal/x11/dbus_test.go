// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import (
	"bufio"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/unison/internal/dbus"
)

// portalTestTimeout is how long a test waits for something that ought to happen at once.
const portalTestTimeout = 2 * time.Second

// portalPeer is the far end of a session bus connection: it answers the bus's own match rule calls, hands everything
// else to a responder the test supplies, records what it was asked, and emits signals on demand. The connection it
// serves is installed as the one [dbus.Session] hands out for the duration of the test.
type portalPeer struct {
	t       *testing.T
	side    net.Conn
	respond func(call *dbus.Message) *dbus.Message
	calls   chan *dbus.Message
	rules   chan string
	serial  atomic.Uint32
}

// newPortalPeer creates a session bus connection whose peer answers portal calls with respond, which may return nil to
// leave a call unanswered. Both ends are closed, and the prior session connection restored, when the test finishes.
func newPortalPeer(t *testing.T, respond func(call *dbus.Message) *dbus.Message) *portalPeer {
	t.Helper()
	clientSide, peerSide := net.Pipe()
	client, err := dbus.Connect(clientSide)
	check.New(t).NoError(err)
	p := &portalPeer{
		t:       t,
		side:    peerSide,
		respond: respond,
		calls:   make(chan *dbus.Message, 16),
		rules:   make(chan string, 16),
	}
	restore := dbus.SetSessionForTest(client, nil)
	t.Cleanup(func() {
		restore()
		client.Close()
		xio.CloseIgnoringErrors(peerSide)
	})
	go p.run()
	return p
}

// run reads messages from the connection under test until it goes away.
func (p *portalPeer) run() {
	in := bufio.NewReader(p.side)
	for {
		msg, err := dbus.Decode(in)
		if err != nil {
			return
		}
		if msg.Type != dbus.TypeMethodCall {
			continue
		}
		if msg.Member == "AddMatch" || msg.Member == "RemoveMatch" {
			p.recordRule(msg)
			p.write(dbus.NewReply(msg))
			continue
		}
		p.calls <- msg
		if reply := p.respond(msg); reply != nil {
			p.write(reply)
		}
	}
}

// recordRule notes the match rule of an AddMatch or RemoveMatch call.
func (p *portalPeer) recordRule(msg *dbus.Message) {
	args, err := msg.Args()
	if err != nil || len(args) != 1 {
		p.t.Errorf("%s did not carry a match rule", msg)
		return
	}
	rule, ok := args[0].(string)
	if !ok {
		p.t.Errorf("%s carried a %T rather than a match rule", msg, args[0])
		return
	}
	p.rules <- rule
}

// write sends a message to the connection under test.
func (p *portalPeer) write(msg *dbus.Message) {
	msg.Serial = p.serial.Add(1)
	data, err := msg.Encode()
	if err != nil {
		p.t.Errorf("the peer could not encode %s: %v", msg, err)
		return
	}
	if _, err = p.side.Write(data); err != nil {
		return // The connection under test has gone away, which the end of a test does on purpose
	}
}

// emit sends a SettingChanged signal to the connection under test.
func (p *portalPeer) emit(namespace, key string, value any) {
	p.t.Helper()
	msg := dbus.NewSignal(portalPath, portalSettingsInterface, portalSettingChanged)
	check.New(p.t).NoError(msg.SetBodyWithSignature("ssv", namespace, key, value))
	p.write(msg)
}

// nextCall returns the next call the connection under test made to something other than the bus itself.
func (p *portalPeer) nextCall() *dbus.Message {
	p.t.Helper()
	select {
	case msg := <-p.calls:
		return msg
	case <-time.After(portalTestTimeout):
		p.t.Fatal("timed out waiting for a method call")
		return nil
	}
}

// nextRule returns the next match rule the connection under test asked the bus for.
func (p *portalPeer) nextRule() string {
	p.t.Helper()
	select {
	case rule := <-p.rules:
		return rule
	case <-time.After(portalTestTimeout):
		p.t.Fatal("timed out waiting for a match rule")
		return ""
	}
}

// bodyReply answers a call with a body marshaled from sig and args. Being unable to marshal it means the test itself is
// wrong, and it happens on the peer's goroutine, where nothing may be reported through the testing.T, so it panics.
func bodyReply(call *dbus.Message, sig dbus.Signature, args ...any) *dbus.Message {
	reply := dbus.NewReply(call)
	if err := reply.SetBodyWithSignature(sig, args...); err != nil {
		panic(err)
	}
	return reply
}

// variantReply answers a call with a single variant whose value is v.
func variantReply(call *dbus.Message, v any) *dbus.Message {
	return bodyReply(call, "v", v)
}

// TestReadColorScheme verifies that the color scheme is read from the portal with the call the portal documents, that
// the extra layer of variant the Read method wraps the value in is unwrapped, and that anything else the portal might
// answer with, including not being there at all, reports that the value is unavailable.
func TestReadColorScheme(t *testing.T) {
	cases := []struct {
		reply func(call *dbus.Message) *dbus.Message
		name  string
		want  uint32
		ok    bool
	}{
		{
			name: "a nested variant, which is what Read actually returns",
			reply: func(call *dbus.Message) *dbus.Message {
				return variantReply(call, dbus.Variant{Sig: "v", Value: dbus.Variant{Sig: "u", Value: uint32(1)}})
			},
			want: 1,
			ok:   true,
		},
		{
			name: "a plain variant",
			reply: func(call *dbus.Message) *dbus.Message {
				return variantReply(call, dbus.Variant{Sig: "u", Value: uint32(2)})
			},
			want: 2,
			ok:   true,
		},
		{
			name: "no portal at all",
			reply: func(call *dbus.Message) *dbus.Message {
				return dbus.NewError(call, dbus.ServiceUnknown, "no such service")
			},
		},
		{
			name: "the setting is not published",
			reply: func(call *dbus.Message) *dbus.Message {
				return dbus.NewError(call, "org.freedesktop.portal.Error.NotFound", "unknown key")
			},
		},
		{
			name: "a value of the wrong type",
			reply: func(call *dbus.Message) *dbus.Message {
				return variantReply(call, dbus.Variant{Sig: "s", Value: "prefer-dark"})
			},
		},
		{
			name: "a reply that is not a variant",
			reply: func(call *dbus.Message) *dbus.Message {
				return bodyReply(call, "s", "prefer-dark")
			},
		},
		{
			name:  "a reply with no value at all",
			reply: dbus.NewReply,
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			peer := newPortalPeer(t, one.reply)
			value, ok := ReadColorScheme()
			c.Equal(one.ok, ok)
			c.Equal(one.want, value)
			call := peer.nextCall()
			c.Equal(portalDestination, call.Destination)
			c.Equal(portalPath, call.Path)
			c.Equal(portalSettingsInterface, call.Interface)
			c.Equal("Read", call.Member)
			args, err := call.Args()
			c.NoError(err)
			c.Equal([]any{colorSchemeNamespace, colorSchemeKey}, args)
		})
	}
}

// TestColorSchemeWithoutSessionBus verifies that a machine with no session bus, which includes every machine that is
// not running a Linux desktop, simply reports that the value is unavailable and never reports a change.
func TestColorSchemeWithoutSessionBus(t *testing.T) {
	c := check.New(t)
	restore := dbus.SetSessionForTest(nil, errors.New("no session bus"))
	defer restore()
	value, ok := ReadColorScheme()
	c.False(ok)
	c.Equal(uint32(0), value)
	// The watch cannot report anything for as long as there is no bus to report from, which is until this test puts the
	// session connection back, so nothing may arrive within the window below. Nothing is reported from the callback
	// itself, since the watch's goroutine outlives the test: it has no way to be stopped.
	changes := make(chan uint32, 1)
	WatchColorScheme(func(value uint32) {
		select {
		case changes <- value:
		default:
		}
	})
	select {
	case reported := <-changes:
		t.Errorf("a color scheme change of %d was reported without a session bus", reported)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestWatchColorScheme verifies that the watch asks the bus for the portal's setting change signals and reports the
// new color scheme from the ones that carry it, ignoring the signals for other settings and the malformed ones.
func TestWatchColorScheme(t *testing.T) {
	c := check.New(t)
	peer := newPortalPeer(t, func(_ *dbus.Message) *dbus.Message { return nil })
	changes := make(chan uint32, 16)
	WatchColorScheme(func(value uint32) {
		select {
		case changes <- value:
		default:
		}
	})
	c.Equal("type='signal',interface='org.freedesktop.portal.Settings',member='SettingChanged'", peer.nextRule())

	// Everything but the last of these is for something other than the color scheme, or is not shaped the way the
	// portal documents, so only the last one may be reported. Signals are delivered in order, so receiving it proves
	// the others were ignored.
	peer.emit("org.example.Other", colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(1)})
	peer.emit(colorSchemeNamespace, "contrast", dbus.Variant{Sig: "u", Value: uint32(1)})
	peer.emit(colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "s", Value: "prefer-dark"})
	peer.emit(colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(2)})
	select {
	case value := <-changes:
		c.Equal(uint32(2), value)
	case <-time.After(portalTestTimeout):
		t.Fatal("timed out waiting for a color scheme change")
	}
	select {
	case value := <-changes:
		t.Errorf("an unexpected color scheme change of %d was reported", value)
	default:
	}
}

// TestWatchColorSchemeStopsWhenTheBusRefuses verifies that a bus that will not deliver the portal's signals leaves
// nothing behind: the subscription is dropped rather than left waiting for signals that will never arrive.
func TestWatchColorSchemeStopsWhenTheBusRefuses(t *testing.T) {
	c := check.New(t)
	clientSide, peerSide := net.Pipe()
	client, err := dbus.Connect(clientSide)
	c.NoError(err)
	restore := dbus.SetSessionForTest(client, nil)
	defer restore()
	defer xio.CloseIgnoringErrors(peerSide)
	defer client.Close()
	refused := make(chan struct{})
	go func() {
		defer close(refused)
		in := bufio.NewReader(peerSide)
		msg, decodeErr := dbus.Decode(in)
		if decodeErr != nil {
			return
		}
		reply := dbus.NewError(msg, dbus.Failed, "no match rules for you")
		reply.Serial = 1
		data, encodeErr := reply.Encode()
		if encodeErr != nil {
			t.Errorf("the peer could not encode %s: %v", reply, encodeErr)
			return
		}
		if _, writeErr := peerSide.Write(data); writeErr != nil {
			return
		}
	}()
	WatchColorScheme(func(_ uint32) {
		t.Error("the callback must not be invoked when the bus refuses the match rule")
	})
	select {
	case <-refused:
	case <-time.After(portalTestTimeout):
		t.Fatal("timed out waiting for the match rule to be refused")
	}
}
