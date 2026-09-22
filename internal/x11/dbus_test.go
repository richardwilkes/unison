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

// portalTestTimeout is how long a test waits for something that ought to happen at once. It only bounds how long a
// failure takes to report, so it is generous enough to ride out the multi-second stalls a loaded CI runner can suffer.
const portalTestTimeout = 10 * time.Second

const (
	// portalOwnerName is the unique name the test bus hands out as the owner of the portal's well-known name, which is
	// what the portal's own signals are stamped with.
	portalOwnerName = ":1.7"
	// otherConnectionName is another connection on the same session bus, which is what a signal that is not the
	// portal's comes from.
	otherConnectionName = ":1.99"
)

// portalPeer is the far end of a session bus connection: it answers the bus's own match rule and name resolution calls,
// hands everything else to a responder the test supplies, records what it was asked, and emits signals on demand. The
// connection it serves is installed as the one [dbus.Session] hands out for the duration of the test.
type portalPeer struct {
	t       *testing.T
	side    net.Conn
	respond func(call *dbus.Message) *dbus.Message
	calls   chan *dbus.Message
	rules   chan string
	// owner is what the bus answers GetNameOwner for the portal's name with; empty means nobody owns it.
	owner string
	// refuse is a match rule that the bus will not accept, which is how a test reaches the path a refusal takes.
	refuse string
	serial atomic.Uint32
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
		owner:   portalOwnerName,
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
		switch msg.Member {
		case "AddMatch", "RemoveMatch":
			rule := p.recordRule(msg)
			if msg.Member == "AddMatch" && p.refuse != "" && rule == p.refuse {
				p.write(dbus.NewError(msg, dbus.Failed, "not that rule"))
				continue
			}
			p.write(dbus.NewReply(msg))
			continue
		case getNameOwner:
			p.answerNameOwner(msg)
			continue
		default:
		}
		p.calls <- msg
		if reply := p.respond(msg); reply != nil {
			p.write(reply)
		}
	}
}

// answerNameOwner answers the bus call that resolves a well-known name to the unique name of whichever connection owns
// it, exactly as the bus does: a name nobody owns is an error rather than an empty answer.
func (p *portalPeer) answerNameOwner(msg *dbus.Message) {
	if p.owner == "" {
		p.write(dbus.NewError(msg, "org.freedesktop.DBus.Error.NameHasNoOwner", "no such name"))
		return
	}
	reply := dbus.NewReply(msg)
	if err := reply.SetBodyWithSignature("s", p.owner); err != nil {
		p.t.Errorf("the peer could not encode the owner of %s: %v", portalDestination, err)
		return
	}
	p.write(reply)
}

// recordRule notes the match rule of an AddMatch or RemoveMatch call and returns it.
func (p *portalPeer) recordRule(msg *dbus.Message) string {
	args, err := msg.Args()
	if err != nil || len(args) != 1 {
		p.t.Errorf("%s did not carry a match rule", msg)
		return ""
	}
	rule, ok := args[0].(string)
	if !ok {
		p.t.Errorf("%s carried a %T rather than a match rule", msg, args[0])
		return ""
	}
	p.rules <- rule
	return rule
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

// emit sends a SettingChanged signal to the connection under test, from the portal itself.
func (p *portalPeer) emit(namespace, key string, value any) {
	p.t.Helper()
	p.emitFrom(portalOwnerName, namespace, key, value)
}

// emitFrom sends a SettingChanged signal to the connection under test, stamped with the sender the bus would have
// filled in for whichever connection sent it. A peer cannot choose that field for itself on a real bus, which is what
// makes checking it worth anything.
func (p *portalPeer) emitFrom(sender, namespace, key string, value any) {
	p.t.Helper()
	msg := dbus.NewSignal(portalPath, portalSettingsInterface, portalSettingChanged)
	msg.Sender = sender
	check.New(p.t).NoError(msg.SetBodyWithSignature("ssv", namespace, key, value))
	p.write(msg)
}

// emitNameOwnerChanged tells the connection under test that a well-known name has changed hands, as the bus does.
func (p *portalPeer) emitNameOwnerChanged(name, from, to string) {
	p.t.Helper()
	msg := dbus.NewSignal(busPath, busInterface, nameOwnerChanged)
	msg.Sender = busDestination
	check.New(p.t).NoError(msg.SetBodyWithSignature("sss", name, from, to))
	p.write(msg)
}

// startWatch begins watching for color scheme changes and returns the channel the changes are reported on, once both of
// the match rules the watch asks for are in place. Nothing may be emitted before that: the subscription for the
// portal's signals is made just before the rule that asks for them, so a signal sent earlier has nowhere to go.
func (p *portalPeer) startWatch() chan uint32 {
	p.t.Helper()
	c := check.New(p.t)
	changes := make(chan uint32, 16)
	WatchColorScheme(func(value uint32) {
		select {
		case changes <- value:
		default:
		}
	})
	c.Equal(portalOwnerMatchRule, p.nextRule(), "the portal's name must be watched for changes of ownership")
	c.Equal(colorSchemeMatchRule, p.nextRule(), "the portal's setting changes must be asked for")
	return changes
}

// probeMember is a signal of the tests' own, used to find out when the connection's dispatcher has worked its way past
// the signals emitted before it.
const probeMember = "ProbeForOrdering"

// waitForDispatch emits a signal of the test's own and waits for it to come back out of the connection's dispatcher.
// Signal handlers run one at a time, in the order the signals arrived, on that one goroutine, so a probe that has come
// back proves everything emitted before it has already been offered to every subscription that wanted it. It is what
// turns "nothing was reported" into a check rather than a guess about how long a dispatcher takes.
func (p *portalPeer) waitForDispatch() {
	p.t.Helper()
	conn, err := dbus.Session()
	check.New(p.t).NoError(err)
	probes := make(chan struct{}, 1)
	defer conn.Subscribe(dbus.SignalFilter{Interface: portalSettingsInterface, Member: probeMember},
		func(_ *dbus.Message) { probes <- struct{}{} })()
	msg := dbus.NewSignal(portalPath, portalSettingsInterface, probeMember)
	msg.Sender = portalOwnerName
	p.write(msg)
	select {
	case <-probes:
	case <-time.After(portalTestTimeout):
		p.t.Fatal("timed out waiting for a probe signal to be dispatched")
	}
}

// awaitChange returns the next color scheme reported by a watch.
func awaitChange(t *testing.T, changes <-chan uint32) uint32 {
	t.Helper()
	select {
	case value := <-changes:
		return value
	case <-time.After(portalTestTimeout):
		t.Fatal("timed out waiting for a color scheme change")
		return 0
	}
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
			// The portal's well-known name is resolved before the call is made, and the call goes to the unique name
			// of whichever connection owns it; see TestReadColorSchemeIgnoresAReplyFromAnyoneButThePortal.
			c.Equal(portalOwnerName, call.Destination)
			c.Equal(portalPath, call.Path)
			c.Equal(portalSettingsInterface, call.Interface)
			c.Equal("Read", call.Member)
			args, err := call.Args()
			c.NoError(err)
			c.Equal([]any{colorSchemeNamespace, colorSchemeKey}, args)
		})
	}
}

// TestReadColorSchemeIgnoresAReplyFromAnyoneButThePortal covers what addressing the call to the portal's unique name
// buys. A call to a well-known name is answered under a unique name the caller cannot know, so any reply at all has to
// be accepted for it, and serials are small integers that start at one: a peer that guesses one and unicasts a
// METHOD_RETURN to us would otherwise choose the desktop's color scheme, the genuine reply arriving afterwards to be
// discarded as unmatched. Asking the bus who owns the name first and calling that name instead is what makes the reply
// checkable, since the bus stamps the SENDER of every message itself and a connection cannot claim another's name.
//
// The forged reply is followed by the genuine one, which is what turns "it was ignored" into something to wait for
// rather than a five second timeout, and what proves the call was left pending rather than failed.
func TestReadColorSchemeIgnoresAReplyFromAnyoneButThePortal(t *testing.T) {
	c := check.New(t)
	var peer *portalPeer
	peer = newPortalPeer(t, func(call *dbus.Message) *dbus.Message {
		forged := variantReply(call, dbus.Variant{Sig: "u", Value: uint32(1)})
		forged.Sender = otherConnectionName
		peer.write(forged)
		genuine := variantReply(call, dbus.Variant{Sig: "u", Value: uint32(2)})
		genuine.Sender = portalOwnerName
		return genuine
	})
	value, ok := ReadColorScheme()
	c.True(ok)
	c.Equal(uint32(2), value)
	c.Equal(portalOwnerName, peer.nextCall().Destination)
}

// TestReadColorSchemeFallsBackToTheWellKnownName covers the one call that cannot be pinned to a unique name: the portal
// is bus-activatable, so on a desktop where it has not been started yet nothing owns its name, and a call to the
// well-known name is what starts it. Insisting on an owner would mean never reading the color scheme at all there.
func TestReadColorSchemeFallsBackToTheWellKnownName(t *testing.T) {
	c := check.New(t)
	peer := newPortalPeer(t, func(call *dbus.Message) *dbus.Message {
		return variantReply(call, dbus.Variant{Sig: "u", Value: uint32(1)})
	})
	peer.owner = ""
	value, ok := ReadColorScheme()
	c.True(ok)
	c.Equal(uint32(1), value)
	c.Equal(portalDestination, peer.nextCall().Destination)
}

// TestColorSchemeWithoutSessionBus verifies that a machine with no session bus, which includes every machine that is
// not running a Linux desktop, simply reports that the value is unavailable and never reports a change. The second case
// is the one an error alone does not cover: a nil connection with a nil error, which is what the accessibility path
// guards against as well, would be a nil dereference rather than an answer if only the error were looked at.
func TestColorSchemeWithoutSessionBus(t *testing.T) {
	for _, one := range []struct {
		err  error
		name string
	}{
		{name: "a bus that could not be reached", err: errors.New("no session bus")},
		{name: "no connection and no error either"},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			restore := dbus.SetSessionForTest(nil, one.err)
			defer restore()
			value, ok := ReadColorScheme()
			c.False(ok)
			c.Equal(uint32(0), value)
			// The watch cannot report anything for as long as there is no bus to report from, which is until this test
			// puts the session connection back, so nothing may arrive within the window below. Nothing is reported from
			// the callback itself, since the watch's goroutine outlives the test: it has no way to be stopped.
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
		})
	}
}

// TestWatchColorScheme verifies that the watch asks the bus for the portal's setting change signals and reports the
// new color scheme from the ones that carry it, ignoring the signals for other settings and the malformed ones.
func TestWatchColorScheme(t *testing.T) {
	c := check.New(t)
	peer := newPortalPeer(t, func(_ *dbus.Message) *dbus.Message { return nil })
	changes := peer.startWatch()

	// Everything but the last of these is for something other than the color scheme, or is not shaped the way the
	// portal documents, so only the last one may be reported. Signals are delivered in order, so receiving it proves
	// the others were ignored.
	peer.emit("org.example.Other", colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(1)})
	peer.emit(colorSchemeNamespace, "contrast", dbus.Variant{Sig: "u", Value: uint32(1)})
	peer.emit(colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "s", Value: "prefer-dark"})
	peer.emit(colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(2)})
	c.Equal(uint32(2), awaitChange(t, changes))
	select {
	case value := <-changes:
		t.Errorf("an unexpected color scheme change of %d was reported", value)
	default:
	}
}

// TestWatchColorSchemeIgnoresASignalFromAnotherConnection covers what the match rule cannot do for itself. Naming the
// sender in a match rule narrows what the bus broadcasts to us and nothing else: a signal that carries a DESTINATION is
// routed straight to that connection, no rule consulted, and the default session policy lets any peer send one to any
// other. Without the check in the handler, any process on the user's session bus could flip the application between
// light and dark by unicasting a SettingChanged of its own.
func TestWatchColorSchemeIgnoresASignalFromAnotherConnection(t *testing.T) {
	c := check.New(t)
	peer := newPortalPeer(t, func(_ *dbus.Message) *dbus.Message { return nil })
	changes := peer.startWatch()
	// The first of these is perfectly well formed and says something the watch would otherwise report; it is only the
	// sender that is wrong. Signals are delivered in order, so the second one arriving proves the first was dropped.
	peer.emitFrom(otherConnectionName, colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(1)})
	peer.emitFrom(portalOwnerName, colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(2)})
	c.Equal(uint32(2), awaitChange(t, changes))
	select {
	case value := <-changes:
		t.Errorf("an unexpected color scheme change of %d was reported", value)
	default:
	}
}

// TestWatchColorSchemeFollowsThePortalToItsNewName covers the other half of checking the sender: the unique name the
// portal answers to is not fixed. A portal that is restarted comes back under a new one, and a watch that went on
// insisting on the old one would ignore every setting change for the life of the process, which is why the bus's own
// NameOwnerChanged is watched for the portal's name.
func TestWatchColorSchemeFollowsThePortalToItsNewName(t *testing.T) {
	const restartedPortalName = ":1.31"
	c := check.New(t)
	peer := newPortalPeer(t, func(_ *dbus.Message) *dbus.Message { return nil })
	changes := peer.startWatch()
	peer.emitNameOwnerChanged(portalDestination, portalOwnerName, restartedPortalName)
	// The name the portal had before it was restarted is somebody else's now, or nobody's, and is no more to be
	// believed than any other connection on the bus.
	peer.emitFrom(portalOwnerName, colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(1)})
	peer.emitFrom(restartedPortalName, colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(2)})
	c.Equal(uint32(2), awaitChange(t, changes))
	// A name that has been given up leaves nothing to believe at all, so the setting changes stop being acted on until
	// something owns it again.
	peer.emitNameOwnerChanged(portalDestination, restartedPortalName, "")
	peer.emitFrom(restartedPortalName, colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(1)})
	peer.emitNameOwnerChanged(portalDestination, "", portalOwnerName)
	peer.emitFrom(portalOwnerName, colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(2)})
	c.Equal(uint32(2), awaitChange(t, changes))
	select {
	case value := <-changes:
		t.Errorf("an unexpected color scheme change of %d was reported", value)
	default:
	}
}

// TestWatchColorSchemeIgnoresEverythingWhenNothingOwnsThePortalName covers the desktop with no portal at all: the bus
// answers GetNameOwner with an error, so there is no connection whose signals are the portal's, and a SettingChanged
// from anyone is a SettingChanged from somebody who is not the portal.
func TestWatchColorSchemeIgnoresEverythingWhenNothingOwnsThePortalName(t *testing.T) {
	peer := newPortalPeer(t, func(_ *dbus.Message) *dbus.Message { return nil })
	peer.owner = ""
	changes := peer.startWatch()
	peer.emitFrom(portalOwnerName, colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(2)})
	peer.waitForDispatch()
	select {
	case value := <-changes:
		t.Errorf("a color scheme of %d was reported with nothing owning the portal's name", value)
	default:
	}
}

// TestWatchColorSchemeStopsWhenTheBusRefuses verifies that a bus that will not deliver the portal's signals leaves
// nothing behind: the subscription is dropped, and the rule that was accepted before it is taken off the bus again,
// rather than being left waiting for signals that will never arrive. The signal the watch would have reported is
// emitted after the refusal, since a subscription that was never dropped is only visible when something arrives for it
// to answer.
func TestWatchColorSchemeStopsWhenTheBusRefuses(t *testing.T) {
	c := check.New(t)
	peer := newPortalPeer(t, func(_ *dbus.Message) *dbus.Message { return nil })
	peer.refuse = colorSchemeMatchRule
	reported := make(chan uint32, 1)
	WatchColorScheme(func(value uint32) {
		select {
		case reported <- value:
		default:
		}
	})
	c.Equal(portalOwnerMatchRule, peer.nextRule())
	c.Equal(colorSchemeMatchRule, peer.nextRule())
	// The watch drops its subscription and then takes the rule it did get back off the bus, so seeing that removal is
	// proof that the subscription is already gone: a signal emitted while the watch was still running would be reported
	// by one that is about to drop its subscription just as it would by one that never does.
	c.Equal(portalOwnerMatchRule, peer.nextRule())
	peer.emitFrom(portalOwnerName, colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(2)})
	peer.waitForDispatch()
	select {
	case value := <-reported:
		t.Errorf("a color scheme of %d was reported by a watch whose match rule the bus refused", value)
	default:
	}
}

// TestWatchColorSchemeStopsWhenTheOwnerRuleIsRefused covers the first of the two rules the watch depends on. Without
// the bus's own reports of the portal's name changing hands, the watch would be left holding a unique name that may
// have gone and rejecting the new portal's signals for the life of the process, so nothing further is asked for and
// nothing is left behind: the rule was never added, so there is nothing to take off the bus again, and the
// subscription that was made before it is dropped.
func TestWatchColorSchemeStopsWhenTheOwnerRuleIsRefused(t *testing.T) {
	c := check.New(t)
	peer := newPortalPeer(t, func(_ *dbus.Message) *dbus.Message { return nil })
	peer.refuse = portalOwnerMatchRule
	reported := make(chan uint32, 1)
	WatchColorScheme(func(value uint32) {
		select {
		case reported <- value:
		default:
		}
	})
	c.Equal(portalOwnerMatchRule, peer.nextRule())
	// The portal's setting changes are never asked for, so the signal that would have been reported by a watch that
	// carried on regardless is not, and the bus is asked for nothing more, the rule that was refused included.
	peer.emitFrom(portalOwnerName, colorSchemeNamespace, colorSchemeKey, dbus.Variant{Sig: "u", Value: uint32(2)})
	peer.waitForDispatch()
	select {
	case rule := <-peer.rules:
		t.Errorf("the watch asked the bus for %q after the rule it depends on was refused", rule)
	default:
	}
	select {
	case value := <-reported:
		t.Errorf("a color scheme of %d was reported by a watch whose first match rule the bus refused", value)
	default:
	}
}
