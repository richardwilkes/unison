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
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/dbus"
)

const (
	// noBridgeEnvKey is the environment variable that has meant "do not talk to the accessibility bus" since the GTK
	// AT-SPI bridge introduced it. Any non-zero value disables accessibility, which is how every toolkit reads it.
	noBridgeEnvKey = "NO_AT_BRIDGE"
	// busAddressEnvKey names the environment variable that, when set, is the address of the accessibility bus and ends
	// the search for one.
	busAddressEnvKey = "AT_SPI_BUS_ADDRESS"
	// isEnabledProperty is the property of [StatusInterface] that says whether an assistive technology is running.
	isEnabledProperty = "IsEnabled"
	// nameOwnerChanged is the signal the session bus emits when a name gains or loses its owner.
	nameOwnerChanged = "NameOwnerChanged"
	// propertiesChanged is the signal an object emits when one of its properties changes.
	propertiesChanged = "PropertiesChanged"
)

// The match rules that [WatchEnabled] asks the session bus for. Signals from another connection are only delivered to
// one that has asked for them.
const (
	statusMatchRule = "type='signal',interface='" + dbusPropertiesInterface + "',member='" + propertiesChanged +
		"',path='" + string(BusPath) + "'"
	ownerMatchRule = "type='signal',interface='" + dbusInterface + "',member='" + nameOwnerChanged +
		"',arg0='" + BusDestination + "'"
)

// DisabledByEnvironment returns true if the environment forbids talking to the accessibility bus at all. Nothing else in
// this package, and nothing in the root package, should reach the bus when it does.
func DisabledByEnvironment() bool {
	value := strings.TrimSpace(os.Getenv(noBridgeEnvKey))
	return value != "" && value != "0"
}

// Enabled reports whether an assistive technology is running, which is what the accessibility bus launcher's IsEnabled
// property says. session is the connection to the desktop session bus, which the launcher lives on. A machine with no
// launcher answers with a ServiceUnknown error, and every failure, that one included, means "no": there is nothing for
// Unison to talk to.
//
// The call asks the bus not to start the launcher if it is not already running, since starting it would be a decision
// the user never made.
func Enabled(session *dbus.Conn) bool {
	if session == nil || DisabledByEnvironment() {
		return false
	}
	msg := dbus.NewMethodCall(BusDestination, BusPath, dbusPropertiesInterface, "Get")
	if err := msg.SetBody(StatusInterface, isEnabledProperty); err != nil {
		return false
	}
	reply, err := session.CallWithFlags(msg, dbus.FlagNoAutoStart)
	if err != nil {
		return false
	}
	args, err := reply.Args()
	if err != nil || len(args) == 0 {
		return false
	}
	enabled, _ := boolValue(args[0])
	return enabled
}

// WatchEnabled calls onChange whenever the answer [Enabled] would give changes, and returns a function that stops
// watching. onChange runs on one of the session connection's goroutines, so it must not block; the root package hands
// the answer to the user interface thread.
//
// Two things are watched. The launcher's own PropertiesChanged signal reports IsEnabled being switched on or off while
// it runs, which is what happens when the user starts or stops a screen reader. NameOwnerChanged for the launcher's name
// reports the launcher itself coming or going, which is what happens on the desktops that only start it once something
// needs it; a launcher that has just appeared is asked again, since its signal came too early for anyone to hear.
func WatchEnabled(session *dbus.Conn, onChange func(enabled bool)) (cancel func()) {
	if session == nil || onChange == nil || DisabledByEnvironment() {
		return func() {}
	}
	recheck := func() {
		// Enabled makes a call, and this runs on the dispatcher goroutine, which must not be held up, so the answer is
		// fetched from a goroutine of its own.
		go func() { onChange(Enabled(session)) }()
	}
	cancelStatus := session.Subscribe(dbus.SignalFilter{
		Path:      string(BusPath),
		Interface: dbusPropertiesInterface,
		Member:    propertiesChanged,
	}, func(msg *dbus.Message) {
		switch state := enabledFromPropertiesChanged(msg); state {
		case enabledTrue:
			onChange(true)
		case enabledFalse:
			onChange(false)
		case enabledUnknown:
			if mentionsIsEnabled(msg) {
				recheck()
			}
		}
	})
	cancelOwner := session.Subscribe(dbus.SignalFilter{
		Interface: dbusInterface,
		Member:    nameOwnerChanged,
	}, func(msg *dbus.Message) {
		owner, ok := newOwnerOfBus(msg)
		if !ok {
			return
		}
		if owner == "" {
			onChange(false)
			return
		}
		recheck()
	})
	done := make(chan struct{})
	// One goroutine owns the match rules for the life of the watch, so a rule for a watch that is canceled while it is
	// still being set up cannot be left behind on the bus.
	go func() {
		rules := make([]string, 0, 2)
		for _, rule := range []string{statusMatchRule, ownerMatchRule} {
			if err := session.AddMatch(rule); err != nil {
				errs.Log(errs.NewWithCause("atspi: unable to watch for accessibility status changes", err),
					"rule", rule)
				continue
			}
			rules = append(rules, rule)
		}
		<-done
		for _, rule := range rules {
			if err := session.RemoveMatch(rule); err != nil {
				errs.Log(errs.NewWithCause("atspi: unable to stop watching for accessibility status changes", err),
					"rule", rule)
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			cancelStatus()
			cancelOwner()
			close(done)
		})
	}
}

// enabledState is what a PropertiesChanged signal had to say about IsEnabled.
type enabledState uint8

// The possible answers.
const (
	enabledUnknown enabledState = iota // The signal did not carry a new value
	enabledTrue
	enabledFalse
)

// enabledFromPropertiesChanged returns the new value of IsEnabled that a PropertiesChanged signal carries, if it carries
// one. The signal's arguments are the interface name, the properties that changed, and the names of the ones that have
// changed without a value being sent.
func enabledFromPropertiesChanged(msg *dbus.Message) enabledState {
	args, err := msg.Args()
	if err != nil || len(args) < 2 {
		return enabledUnknown
	}
	if name, ok := args[0].(string); !ok || name != StatusInterface {
		return enabledUnknown
	}
	changed, ok := args[1].(dbus.Dict)
	if !ok {
		return enabledUnknown
	}
	for _, entry := range changed {
		if key, isString := entry.Key.(string); !isString || key != isEnabledProperty {
			continue
		}
		if enabled, valid := boolValue(entry.Value); valid {
			if enabled {
				return enabledTrue
			}
			return enabledFalse
		}
	}
	return enabledUnknown
}

// mentionsIsEnabled returns true if a PropertiesChanged signal for the status interface says that IsEnabled has changed
// without saying what to, which is an invitation to ask.
func mentionsIsEnabled(msg *dbus.Message) bool {
	args, err := msg.Args()
	if err != nil || len(args) < 3 {
		return false
	}
	if name, ok := args[0].(string); !ok || name != StatusInterface {
		return false
	}
	invalidated, ok := args[2].([]string)
	if !ok {
		return false
	}
	return slices.Contains(invalidated, isEnabledProperty)
}

// newOwnerOfBus returns the new owner of the accessibility bus launcher's name that a NameOwnerChanged signal reports.
// ok is false for a signal about any other name.
func newOwnerOfBus(msg *dbus.Message) (owner string, ok bool) {
	args, err := msg.Args()
	if err != nil || len(args) < 3 {
		return "", false
	}
	name, isString := args[0].(string)
	if !isString || name != BusDestination {
		return "", false
	}
	owner, isString = args[2].(string)
	if !isString {
		return "", false
	}
	return owner, true
}

// BusAddress returns the address of the accessibility bus. The environment is asked first, since a session that has one
// set means it; then the launcher on the session bus, which is where it comes from on a normal desktop; and finally the
// caller's fallback, which the root package fills in by reading the AT_SPI_BUS property of the X11 root window. Unison
// never sets that property itself.
func BusAddress(session *dbus.Conn, x11Address func() string) (string, error) {
	if address := strings.TrimSpace(os.Getenv(busAddressEnvKey)); address != "" {
		return address, nil
	}
	if session != nil {
		if address := addressFromLauncher(session); address != "" {
			return address, nil
		}
	}
	if x11Address != nil {
		if address := strings.TrimSpace(x11Address()); address != "" {
			return address, nil
		}
	}
	return "", errs.New("atspi: unable to determine the address of the accessibility bus")
}

// addressFromLauncher asks the accessibility bus launcher where its bus is, returning an empty string if it cannot say.
func addressFromLauncher(session *dbus.Conn) string {
	reply, err := session.Call(dbus.NewMethodCall(BusDestination, BusPath, BusInterface, "GetAddress"))
	if err != nil {
		return ""
	}
	args, err := reply.Args()
	if err != nil || len(args) == 0 {
		return ""
	}
	address, ok := args[0].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(address)
}

// boolValue extracts a boolean from a value that may be wrapped in any number of variants, which is how a property
// arrives.
func boolValue(v any) (value, ok bool) {
	for {
		variant, isVariant := v.(dbus.Variant)
		if !isVariant {
			break
		}
		v = variant.Value
	}
	value, ok = v.(bool)
	return value, ok
}
