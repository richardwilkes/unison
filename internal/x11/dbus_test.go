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
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/unison/internal/dbus"
)

// portalTestTimeout is how long a test waits for something that ought to happen at once. It only bounds how long a
// failure takes to report, so it is generous enough to ride out the multi-second stalls a loaded CI runner can suffer.
const portalTestTimeout = 10 * time.Second

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
// nothing behind: the subscription is dropped rather than left waiting for signals that will never arrive. The signal
// the watch would have reported is emitted after the refusal, since a subscription that was never dropped is only
// visible when something arrives for it to answer.
func TestWatchColorSchemeStopsWhenTheBusRefuses(t *testing.T) {
	// probeMember is a signal of this test's own, used to find out when the connection's dispatcher has worked its way
	// past the signals emitted before it.
	const probeMember = "ProbeForOrdering"
	c := check.New(t)
	clientSide, peerSide := net.Pipe()
	client, err := dbus.Connect(clientSide)
	c.NoError(err)
	restore := dbus.SetSessionForTest(client, nil)
	defer restore()
	defer xio.CloseIgnoringErrors(peerSide)
	defer client.Close()
	var serial atomic.Uint32
	write := func(msg *dbus.Message) {
		msg.Serial = serial.Add(1)
		data, encodeErr := msg.Encode()
		if encodeErr != nil {
			t.Errorf("the peer could not encode %s: %v", msg, encodeErr)
			return
		}
		//nolint:errcheck // A failure here is the connection under test going away, which the end of the test does
		_, _ = peerSide.Write(data)
	}
	refused := make(chan struct{})
	go func() {
		defer close(refused)
		in := bufio.NewReader(peerSide)
		msg, decodeErr := dbus.Decode(in)
		if decodeErr != nil {
			return
		}
		write(dbus.NewError(msg, dbus.Failed, "no match rules for you"))
	}()
	reported := make(chan uint32, 1)
	WatchColorScheme(func(value uint32) {
		select {
		case reported <- value:
		default:
		}
	})
	select {
	case <-refused:
	case <-time.After(portalTestTimeout):
		t.Fatal("timed out waiting for the match rule to be refused")
	}

	// The subscription is dropped on a goroutine of the watch's own, which returns the moment it has done so, so the
	// signal below is emitted only once that goroutine is gone. Everything else about the watch is invisible from out
	// here, and a signal emitted while it was still running would be reported by a watch that is about to drop its
	// subscription just as it would by one that never does.
	waitForTheColorSchemeWatchToFinish(t)

	// Signal handlers run one at a time, in order, on the connection's dispatcher goroutine, so a probe that has come
	// back proves everything emitted before it has already been offered to every subscription that wanted it.
	probes := make(chan struct{}, 2)
	defer client.Subscribe(dbus.SignalFilter{Interface: portalSettingsInterface, Member: probeMember},
		func(_ *dbus.Message) { probes <- struct{}{} })()
	probe := func() {
		t.Helper()
		write(dbus.NewSignal(portalPath, portalSettingsInterface, probeMember))
		select {
		case <-probes:
		case <-time.After(portalTestTimeout):
			t.Fatal("timed out waiting for a probe signal to be dispatched")
		}
	}
	probe()
	change := dbus.NewSignal(portalPath, portalSettingsInterface, portalSettingChanged)
	c.NoError(change.SetBodyWithSignature("ssv", colorSchemeNamespace, colorSchemeKey,
		dbus.Variant{Sig: "u", Value: uint32(2)}))
	write(change)
	probe()
	select {
	case value := <-reported:
		t.Errorf("a color scheme of %d was reported by a watch whose match rule the bus refused", value)
	default:
	}
}

// waitForTheColorSchemeWatchToFinish waits until the goroutine [WatchColorScheme] runs its work on has returned, which
// it does as soon as the bus has either taken its match rule or refused it and its subscription has been dropped. A
// stack dump is the only place that goroutine can be seen from outside, and seeing it is what turns the check that
// follows into a check rather than a guess about how long a goroutine takes to notice a reply.
func waitForTheColorSchemeWatchToFinish(t *testing.T) {
	t.Helper()
	// The frame the goroutine runs in. Only its absence is acted on, so a rename here costs the test its certainty
	// rather than its correctness: what follows would simply be racing the watch again, as it would without this.
	const frame = "x11.WatchColorScheme.func"
	deadline := time.Now().Add(portalTestTimeout)
	for strings.Contains(goroutineDump(), frame) {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the color scheme watch to finish with the refused match rule")
		}
		time.Sleep(time.Millisecond)
	}
}

// goroutineDump returns the stacks of every goroutine in the process, whole rather than truncated.
func goroutineDump() string {
	for size := 64 * 1024; ; size *= 2 {
		buf := make([]byte, size)
		if n := runtime.Stack(buf, true); n < size {
			return string(buf[:n])
		}
	}
}
