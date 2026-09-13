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

func TestDisabledByEnvironment(t *testing.T) {
	for _, one := range []struct {
		value    string
		disabled bool
	}{
		{value: "", disabled: false},
		{value: "0", disabled: false},
		{value: "1", disabled: true},
		{value: "2", disabled: true},
		{value: " 1 ", disabled: true},
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
	p.emit(dbusPath, dbusInterface, nameOwnerChanged, "sss", BusDestination, testPeerName, "")
	c.False(nextChange(t, changes))
	p.emit(dbusPath, dbusInterface, nameOwnerChanged, "sss", BusDestination, "", testPeerName)
	c.True(nextChange(t, changes))

	cancel()
	waitForRules(t, p, 0)
	cancel() // Canceling twice must not remove anything twice, nor panic
	c.Equal(0, len(p.matchRules()))
}

func TestWatchEnabledIgnoresWhatIsNotItsBusiness(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	p := newTestPeer(t, statusAnswers)
	changes := make(chan bool, 8)
	defer WatchEnabled(p.client, func(enabled bool) { changes <- enabled })()
	waitForRules(t, p, 2)
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", "org.example.Other",
		dbus.Dict{{Key: isEnabledProperty, Value: dbus.Variant{Sig: "b", Value: true}}}, []string{})
	p.emit(BusPath, dbusPropertiesInterface, propertiesChanged, "sa{sv}as", StatusInterface,
		dbus.Dict{{Key: "SomethingElse", Value: dbus.Variant{Sig: "b", Value: true}}}, []string{})
	p.emit(dbusPath, dbusInterface, nameOwnerChanged, "sss", "org.example.Other", "", testPeerName)
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
