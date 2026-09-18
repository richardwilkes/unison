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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/internal/dbus"
)

// clearAccessibilityEnvironment makes the process look like a plain desktop session, so that a machine that happens to
// have either of these variables set does not change what the tests see.
func clearAccessibilityEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv(noBridgeEnvKey, "")
	t.Setenv(busAddressEnvKey, "")
}

// TestDisabledByEnvironment covers the values NO_AT_BRIDGE is set to in the wild. The rule is the one the C toolkits
// apply, which is atoi: a leading integer, whatever follows it ignored, and zero for anything that does not start with
// one. A value this read as an instruction to stay away while at-spi2-atk and GTK read it as zero would leave an
// application silent on a desktop where every other one still talks.
func TestDisabledByEnvironment(t *testing.T) {
	for _, one := range []struct {
		value    string
		disabled bool
	}{
		{value: "", disabled: false},
		{value: "0", disabled: false},
		{value: "00", disabled: false},
		{value: "-0", disabled: false},
		{value: "1", disabled: true},
		{value: "2", disabled: true},
		{value: " 1 ", disabled: true},
		{value: "01", disabled: true},
		{value: "-1", disabled: true},
		{value: "+1", disabled: true},
		// atoi stops at the first thing that is not a digit, so these are the numbers 1 and 0 respectively.
		{value: "1x", disabled: true},
		{value: "0x1", disabled: false},
		// Nothing that does not begin with a number is one, however much it looks like an answer.
		{value: "false", disabled: false},
		{value: "yes", disabled: false},
		{value: "true", disabled: false},
	} {
		t.Setenv(noBridgeEnvKey, one.value)
		check.New(t).Equal(one.disabled, DisabledByEnvironment(), "%s=%q", noBridgeEnvKey, one.value)
	}
}

func TestEnabled(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	p := newTestPeer(t, statusAnswers)
	c.False(Enabled(p.client), "a launcher that says no means no")
	p.enabled.Store(true)
	c.True(Enabled(p.client))
}

func TestEnabledWhenThereIsNothingToAsk(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	c.False(Enabled(nil), "a machine with no session bus has no accessibility bus either")
	p := newTestPeer(t, unknownServiceAnswers)
	c.False(Enabled(p.client), "a session bus with no launcher on it means no")
}

func TestEnabledObeysTheEnvironment(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	p := newTestPeer(t, statusAnswers)
	p.enabled.Store(true)
	c.True(Enabled(p.client))
	t.Setenv(noBridgeEnvKey, "1")
	c.False(Enabled(p.client), "the environment has the last word")
}

func TestWatchEnabledNeedsSomewhereToReport(t *testing.T) {
	clearAccessibilityEnvironment(t)
	p := newTestPeer(t, statusAnswers)
	// None of these watch anything, but each must still hand back something that can be called.
	WatchEnabled(nil, func(_ bool) {})()
	WatchEnabled(p.client, nil)()
	t.Setenv(noBridgeEnvKey, "1")
	WatchEnabled(p.client, func(_ bool) {})()
	check.New(t).Equal(0, len(p.matchRules()), "a watch that does nothing asks the bus for nothing")
}

func TestWatchEnabled(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	p := newTestPeer(t, statusAnswers)
	changes := make(chan bool, 8)
	cancel := WatchEnabled(p.client, func(enabled bool) { changes <- enabled })

	// The state the watch started in is reported first, and every signal below is emitted after it has arrived, so
	// nothing that follows can be confused with it.
	c.False(nextChange(t, changes), "the state the watch started in is reported")
	rules := waitForRules(t, p, 2)
	c.Equal([]string{statusMatchRule, ownerMatchRule}, rules)

	// A launcher that has been switched on or off says so itself.
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{{Key: isEnabledProperty, Value: dbus.Variant{Sig: "b", Value: true}}}, []string{})
	c.True(nextChange(t, changes))
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{{Key: isEnabledProperty, Value: dbus.Variant{Sig: "b", Value: false}}}, []string{})
	c.False(nextChange(t, changes))

	// A launcher that only says the property has changed is asked what it changed to.
	p.enabled.Store(true)
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface, dbus.Dict{},
		[]string{isEnabledProperty})
	c.True(nextChange(t, changes))

	// A launcher that goes away takes accessibility with it, and one that appears is asked.
	p.emitFrom(dbusDestination, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", BusDestination, testPeerName,
		"")
	c.False(nextChange(t, changes))
	p.emitFrom(dbusDestination, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", BusDestination, "",
		testPeerName)
	c.True(nextChange(t, changes))

	cancel()
	waitForRules(t, p, 0)
	cancel() // Canceling twice must not remove anything twice, nor panic
	c.Equal(0, len(p.matchRules()))
}

// TestWatchEnabledReportsWhatItStartedWith covers the gap a watch would otherwise leave: the match rules it needs are
// asked for with calls of their own, so the bus is not yet delivering anything when WatchEnabled returns. Reading the
// property before that point could miss a screen reader that started in between, and the signal that would have
// reported it would have gone nowhere, so the first answer has to be read by the watch itself, once the rules are in
// place.
func TestWatchEnabledReportsWhatItStartedWith(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	p := newTestPeer(t, statusAnswers)
	p.enabled.Store(true)
	changes := make(chan bool, 8)
	defer WatchEnabled(p.client, func(enabled bool) { changes <- enabled })()
	c.True(nextChange(t, changes), "a launcher that was already saying yes is reported without waiting for a signal")
	c.Equal([]string{statusMatchRule, ownerMatchRule}, p.matchRules(),
		"and only once the bus has agreed to deliver the signals that would report a later change")
}

// TestWatchEnabledSaysNothingOnceItIsCanceled covers the other side of that first read. [Enabled] is a round trip to
// the session bus, so a watch can be canceled while the answer is still on its way back, and a caller that has stopped
// listening must hear nothing afterwards: for the root package, canceling before the first answer means accessibility
// support was refused outright, and reporting it anyway would start a bridge nobody asked for.
func TestWatchEnabledSaysNothingOnceItIsCanceled(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	asked := make(chan struct{})
	answer := make(chan struct{})
	p := newTestPeer(t, func(peer *testPeer, msg *dbus.Message) bool {
		if msg.Interface == dbusPropertiesInterface && msg.Member == getMember {
			// The watch is canceled while this reply is still owed, which is the race the second check closes.
			close(asked)
			<-answer
		}
		return statusAnswers(peer, msg)
	})
	p.enabled.Store(true)
	changes := make(chan bool, 8)
	cancel := WatchEnabled(p.client, func(enabled bool) { changes <- enabled })
	<-asked
	cancel()
	close(answer)
	// Taking the match rules back off the bus is the last thing the watch's goroutine does, so once they are gone it
	// has decided what to do with the answer it was waiting for.
	waitForRules(t, p, 0)
	select {
	case reported := <-changes:
		t.Fatalf("a canceled watch reported %v", reported)
	default:
	}
	c.Equal(0, len(p.matchRules()))
}

// TestWatchEnabledSaysNothingOnceItIsCanceledAfterASignal covers the same race one round further in. A signal that
// arrives just before the watch is canceled leaves a property read in flight, and the answer must reach nobody: for the
// root package, a yes delivered after the bridge has been torn down builds a fresh adapter against a launcher that may
// no longer be there.
func TestWatchEnabledSaysNothingOnceItIsCanceledAfterASignal(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	var reads atomic.Int32
	asked := make(chan struct{})
	answer := make(chan struct{})
	p := newTestPeer(t, func(peer *testPeer, msg *dbus.Message) bool {
		if msg.Interface == dbusPropertiesInterface && msg.Member == getMember && reads.Add(1) == 2 {
			// The first read is the one the watch takes as it starts; this is the one the signal below asked for, and
			// the watch is canceled while it is still owed.
			close(asked)
			<-answer
		}
		return statusAnswers(peer, msg)
	})
	changes := make(chan bool, 8)
	cancel := WatchEnabled(p.client, func(enabled bool) { changes <- enabled })
	c.False(nextChange(t, changes), "the state the watch started in is reported first")
	waitForRules(t, p, 2)

	// A launcher that has just taken its name is asked what it has to say, since its own signal came too early for
	// anyone to hear it.
	p.enabled.Store(true)
	p.emitFrom(dbusDestination, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", BusDestination, "",
		testPeerName)
	<-asked
	cancel()
	close(answer)
	// Taking the match rules back off the bus is the last thing the watch's own goroutine does, and it is two round
	// trips to the peer, by which time the goroutine that was holding the answer has decided what to do with it.
	waitForRules(t, p, 0)
	select {
	case reported := <-changes:
		t.Fatalf("a canceled watch reported %v", reported)
	default:
	}
}

// TestWatchEnabledReportsOneAnswerAtATime covers the interleaving the generation count cannot close on its own.
// [Enabled] is a round trip, and the launcher's own signal arrives on the session connection's dispatcher goroutine
// while one is in flight: a read that checked the count, was descheduled while the signal bumped it and reported the
// newer answer, and then reported its own would leave the stale answer as the last word the root package heard — a
// bridge left running for a launcher that has just said it is not wanted, or torn down for one that is. Bumping the
// count, checking it and calling all happen under one lock, so no two answers can be reported at once and the order
// they are reported in is the order they were settled in.
func TestWatchEnabledReportsOneAnswerAtATime(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	var reads atomic.Int32
	asked := make(chan struct{})
	answer := make(chan struct{})
	p := newTestPeer(t, func(peer *testPeer, msg *dbus.Message) bool {
		if msg.Interface == dbusPropertiesInterface && msg.Member == getMember && reads.Add(1) == 2 {
			// The first read is the one the watch takes as it starts; this is the one the signal below asked for.
			close(asked)
			<-answer
		}
		return statusAnswers(peer, msg)
	})
	// The count is deliberately left unguarded. Answers that are reported one at a time are ordered by the lock that
	// reports them, so the race detector sees one goroutine hand the count to the next; answers that are not are two
	// goroutines touching it with nothing between them, which is what this test is about and what the race detector
	// reports.
	reported := 0
	var once sync.Once
	entered := make(chan struct{})
	proceed := make(chan struct{})
	changes := make(chan bool, 8)
	defer WatchEnabled(p.client, func(enabled bool) {
		if enabled {
			// This is the answer the held read produced, and it is held here, still being reported, while the
			// launcher's own signal arrives.
			once.Do(func() {
				close(entered)
				<-proceed
			})
		}
		// Counted after the wait above rather than before it, so that the count really is touched from inside the
		// report the signal has to wait for rather than from before it started.
		reported++
		changes <- enabled
	})()
	c.False(nextChange(t, changes), "the state the watch started in is reported first")
	waitForRules(t, p, 2)

	// A launcher that has just taken its name is asked what it has to say, and says yes.
	p.enabled.Store(true)
	p.emitFrom(dbusDestination, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", BusDestination, "",
		testPeerName)
	<-asked
	close(answer)
	<-entered

	// It is switched off again while that answer is still being reported, which is the pair of reports that must not
	// overlap.
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{{Key: isEnabledProperty, Value: dbus.Variant{Sig: "b", Value: false}}}, []string{})
	close(proceed)
	c.True(nextChange(t, changes), "the read's answer is reported first, since it was settled first")
	c.False(nextChange(t, changes), "and the signal that overtook it has the last word")
	c.Equal(3, reported, "every answer was reported, and each of them once")
}

func TestWatchEnabledIgnoresWhatIsNotItsBusiness(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	p := newTestPeer(t, statusAnswers)
	changes := make(chan bool, 8)
	defer WatchEnabled(p.client, func(enabled bool) { changes <- enabled })()
	c.False(nextChange(t, changes), "the state the watch started in is reported before anything else")
	waitForRules(t, p, 2)
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", "org.example.Other",
		dbus.Dict{{Key: isEnabledProperty, Value: dbus.Variant{Sig: "b", Value: true}}}, []string{})
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{{Key: "SomethingElse", Value: dbus.Variant{Sig: "b", Value: true}}}, []string{})
	p.emitFrom(dbusDestination, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", "org.example.Other", "",
		testPeerName)
	// Only the bus itself says who owns a name. A NameOwnerChanged from anything else is another process on the session
	// bus trying to decide whether this one talks to an assistive technology, and taking it at its word would let any
	// peer cut a screen-reader user off from the application.
	p.emitFrom(testPeerName, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", BusDestination, testPeerName, "")
	// The signal that does matter arrives last, so anything reported before it would be one of the others.
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{{Key: isEnabledProperty, Value: dbus.Variant{Sig: "b", Value: true}}}, []string{})
	c.True(nextChange(t, changes))
	select {
	case reported := <-changes:
		t.Fatalf("nothing else should have been reported, but %v was", reported)
	default:
	}
}

// TestWatchEnabledOnlyListensToTheLauncher covers the hole a match rule cannot close. The bus consults the rules only
// for the signals it broadcasts: one addressed to this connection is delivered whatever they say, so any peer on the
// session bus could otherwise unicast a PropertiesChanged for /org/a11y/bus saying IsEnabled=false and have the root
// package tear the bridge down, leaving a screen-reader user with no way into the application. What the launcher's
// signals carry, and what nothing else can, is the unique name of the connection that owns the launcher's well-known
// name, which the watch resolves for itself and keeps current from NameOwnerChanged.
func TestWatchEnabledOnlyListensToTheLauncher(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	p := newTestPeer(t, statusAnswers)
	changes := make(chan bool, 8)
	defer WatchEnabled(p.client, func(enabled bool) { changes <- enabled })()
	c.False(nextChange(t, changes), "the state the watch started in is reported before anything else")
	waitForRules(t, p, 2)

	// Another peer saying the status has changed, both of the ways a PropertiesChanged can say it: with the new value,
	// and by naming the property as one to be asked about again. Neither is the launcher's word.
	p.emitFrom(testOtherName, BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{{Key: isEnabledProperty, Value: dbus.Variant{Sig: "b", Value: true}}}, []string{})
	p.emitFrom(testOtherName, BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{}, []string{isEnabledProperty})
	// The launcher itself is still listened to, and its signal arrives last, so anything reported before it would be
	// one of the forgeries.
	p.enabled.Store(true)
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{{Key: isEnabledProperty, Value: dbus.Variant{Sig: "b", Value: true}}}, []string{})
	c.True(nextChange(t, changes))
	select {
	case reported := <-changes:
		t.Fatalf("nothing else should have been reported, but %v was", reported)
	default:
	}

	// The name moving to another connection moves the trust with it: the launcher that owns it now is asked what it has
	// to say, and the connection that used to own it is just another peer from then on.
	p.enabled.Store(false)
	p.emitFrom(dbusDestination, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", BusDestination, testPeerName,
		testOtherName)
	c.False(nextChange(t, changes), "a launcher that has just taken the name is asked what it has to say")
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{{Key: isEnabledProperty, Value: dbus.Variant{Sig: "b", Value: true}}}, []string{})
	p.emitFrom(testOtherName, BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{{Key: isEnabledProperty, Value: dbus.Variant{Sig: "b", Value: true}}}, []string{})
	c.True(nextChange(t, changes), "the owner's word is taken, and the connection that lost the name is not")
	select {
	case reported := <-changes:
		t.Fatalf("nothing else should have been reported, but %v was", reported)
	default:
	}
}

func TestBusAddress(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	p := newTestPeer(t, statusAnswers)

	t.Setenv(busAddressEnvKey, " unix:path=/tmp/from-the-environment ")
	address, err := BusAddress(p.client, func() string { return "unix:path=/tmp/from-x11" })
	c.NoError(err)
	c.Equal("unix:path=/tmp/from-the-environment", address, "the environment is asked first")

	t.Setenv(busAddressEnvKey, "")
	address, err = BusAddress(p.client, func() string { return "unix:path=/tmp/from-x11" })
	c.NoError(err)
	c.Equal("unix:path=/tmp/at-spi-bus-for-a-test", address, "then the launcher on the session bus")

	address, err = BusAddress(nil, func() string { return " unix:path=/tmp/from-x11 " })
	c.NoError(err)
	c.Equal("unix:path=/tmp/from-x11", address, "and finally the X11 root window")

	_, err = BusAddress(nil, nil)
	c.HasError(err)
	_, err = BusAddress(nil, func() string { return "" })
	c.HasError(err)

	quiet := newTestPeer(t, unknownServiceAnswers)
	_, err = BusAddress(quiet.client, nil)
	c.HasError(err, "a session bus with no launcher cannot say where the accessibility bus is")
}

// TestTheLaunchersAddressIsAskedOfItsOwner covers where the question that finds the accessibility bus is sent. A reply
// to a call addressed to a well-known name carries the unique name of whichever connection owns it, which the caller
// cannot know in advance, so such a reply is accepted from any sender: a peer on the session bus that guesses the
// serial can answer in the launcher's place, and the answer is the address of the bus that everything the application
// says to an assistive technology, and every keystroke one sends back, travels over. Addressing the call to the unique
// name the bus resolved instead is what makes the sender check possible.
func TestTheLaunchersAddressIsAskedOfItsOwner(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	asked := make(chan *dbus.Message, 4)
	forge := true
	p := newTestPeer(t, func(peer *testPeer, msg *dbus.Message) bool {
		if msg.Interface != BusInterface || msg.Member != "GetAddress" {
			return statusAnswers(peer, msg)
		}
		peer.deliver(asked, msg)
		if forge {
			// Another peer answers first, with a bus of its own. It has the serial in front of it, which is no more
			// than one that guesses it has.
			peer.replyFrom(testOtherName, msg, "s", "unix:path=/tmp/a-bus-of-somebody-elses")
		}
		peer.replyFrom(testPeerName, msg, "s", "unix:path=/tmp/at-spi-bus-for-a-test")
		return true
	})

	address, err := BusAddress(p.client, nil)
	c.NoError(err)
	c.Equal("unix:path=/tmp/at-spi-bus-for-a-test", address,
		"a reply from anyone but the launcher must be discarded, leaving the launcher's own to answer the call")
	c.Equal(testPeerName, (<-asked).Destination,
		"the connection that owns the launcher's name is what the call is addressed to")

	// A name nobody owns is a launcher that is not running, and a call to the well-known name is what starts one. The
	// reply to that call cannot be checked, which is the price of there being anything to ask at all.
	forge = false
	p.noOwners.Store(true)
	address, err = BusAddress(p.client, nil)
	c.NoError(err)
	c.Equal("unix:path=/tmp/at-spi-bus-for-a-test", address)
	c.Equal(BusDestination, (<-asked).Destination, "which is the one thing that can start a launcher on demand")
}

// waitForRules waits until the connection under test has asked the bus for exactly count match rules and returns them.
func waitForRules(t *testing.T, p *testPeer, count int) []string {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for {
		rules := p.matchRules()
		if len(rules) == count {
			return rules
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d match rules; there are %d", count, len(rules))
			return nil
		}
		time.Sleep(time.Millisecond)
	}
}

// nextChange returns the next change a watch reported.
func nextChange(t *testing.T, changes chan bool) bool {
	t.Helper()
	select {
	case enabled := <-changes:
		return enabled
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for a change to be reported")
		return false
	}
}
