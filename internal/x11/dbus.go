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
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/dbus"
)

// The XDG Desktop Portal publishes the desktop's appearance preferences on the session bus
// (https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.Settings.html). This file asks it for
// the "color-scheme" preference and watches for changes to it. Everything below rides on the one session bus
// connection that [dbus.Session] hands out, which is shared with the accessibility support, so nothing here dials or
// closes a connection of its own.

const (
	// portalDestination is the well-known bus name of the XDG Desktop Portal.
	portalDestination = "org.freedesktop.portal.Desktop"
	// portalPath is the path of the portal's object.
	portalPath = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	// portalSettingsInterface is the portal interface that publishes the desktop's settings.
	portalSettingsInterface = "org.freedesktop.portal.Settings"
	// portalSettingChanged is the signal the portal emits when one of those settings changes.
	portalSettingChanged = "SettingChanged"
	// colorSchemeNamespace is the settings namespace holding the desktop's appearance preferences.
	colorSchemeNamespace = "org.freedesktop.appearance"
	// colorSchemeKey is the key within that namespace holding the dark mode preference.
	colorSchemeKey = "color-scheme"
	// busDestination is the bus's own well-known name, which is what its NameOwnerChanged signals carry as their
	// sender and what the call that resolves a name is addressed to.
	busDestination = "org.freedesktop.DBus"
	// busPath is the path of the bus's own object.
	busPath = dbus.ObjectPath("/org/freedesktop/DBus")
	// busInterface is the interface the bus itself implements.
	busInterface = "org.freedesktop.DBus"
	// nameOwnerChanged is the signal the bus emits when a name gains or loses its owner.
	nameOwnerChanged = "NameOwnerChanged"
	// getNameOwner is the bus method that says which connection owns a well-known name.
	getNameOwner = "GetNameOwner"
	// colorSchemeMatchRule asks the bus to deliver the portal's setting change signals to us, and portalOwnerMatchRule
	// the bus's own reports of the portal's name changing hands. Signals broadcast by another connection are only
	// delivered to one that has asked for them.
	//
	// Naming the sender narrows what the bus broadcasts to us, and no more than that: a signal that carries a
	// DESTINATION is unicast, which the bus routes to that connection without consulting a match rule at all, and the
	// default session policy lets any peer send one to any other. A match rule is therefore an economy rather than a
	// defense, and what keeps another process on the user's session bus from flipping the application between light and
	// dark with a SettingChanged of its own is [WatchColorScheme] checking the sender of every signal it is handed
	// against the connection the bus says owns the portal's name. The accessibility status watch checks its own signals
	// the same way, for the same reason; see internal/atspi.
	colorSchemeMatchRule = "type='signal',sender='" + portalDestination + "',interface='" + portalSettingsInterface +
		"',member='" + portalSettingChanged + "'"
	portalOwnerMatchRule = "type='signal',sender='" + busDestination + "',interface='" + busInterface +
		"',member='" + nameOwnerChanged + "',arg0='" + portalDestination + "'"
)

// ReadColorScheme queries the XDG Desktop Portal for the current color scheme preference. It returns the raw value
// (0 = no preference, 1 = prefer dark, 2 = prefer light) and whether the query succeeded. A false result means the
// portal or the setting is unavailable.
//
// The call is addressed to the unique name of whichever connection owns the portal's well-known name, rather than to
// the well-known name itself; see [portalDestinationOrItsOwner] for why, and for the one case that cannot be.
func ReadColorScheme() (value uint32, ok bool) {
	conn, err := dbus.Session()
	if err != nil || conn == nil {
		return 0, false
	}
	msg := dbus.NewMethodCall(portalDestinationOrItsOwner(conn), portalPath, portalSettingsInterface, "Read")
	if err = msg.SetBody(colorSchemeNamespace, colorSchemeKey); err != nil {
		return 0, false
	}
	reply, err := conn.Call(msg)
	if err != nil {
		return 0, false // Includes the NameHasNoOwner and ServiceUnknown errors a machine with no portal answers with
	}
	args, err := reply.Args()
	if err != nil || len(args) == 0 {
		return 0, false
	}
	return colorSchemeValue(args[0])
}

// WatchColorScheme subscribes to XDG Desktop Portal "SettingChanged" signals and invokes onChange with the new
// color-scheme value whenever it changes. It returns immediately; watching continues in the background and stops
// silently if the portal is unavailable or the connection drops. onChange is called from one of the session
// connection's goroutines, so it must not block.
//
// Only the portal's own signals are acted on. A match rule cannot establish that on its own — it governs what the bus
// broadcasts to us, while a signal addressed to this connection is delivered whatever the rules say — so the portal's
// well-known name is resolved to the unique name of whichever connection owns it, and the sender of every signal is
// checked against it. That name is not something the application can know in advance and it changes when the portal is
// restarted, which is why the bus's NameOwnerChanged is watched for it as well as being asked once at the start: the
// watch is put in place before the name is resolved, so that a portal that comes or goes while the answer is on its way
// back cannot be missed, and an answer that something newer has overtaken is dropped when it arrives.
func WatchColorScheme(onChange func(value uint32)) {
	// Reaching the bus and asking it for the signals can take as long as a call does, so, as before, none of it
	// happens on the caller's goroutine.
	go func() {
		conn, err := dbus.Session()
		if err != nil || conn == nil {
			return
		}
		owner := &portalOwner{}
		// Subscribe first, so that a signal arriving as soon as the match rule is in place has somewhere to go.
		cancelOwner := conn.Subscribe(dbus.SignalFilter{
			Sender:    busDestination,
			Path:      busPath,
			Interface: busInterface,
			Member:    nameOwnerChanged,
		}, func(msg *dbus.Message) {
			if name, ok := newOwnerOfPortal(msg); ok {
				owner.set(name)
			}
		})
		if err = conn.AddMatch(portalOwnerMatchRule); err != nil {
			// Without this rule the name could change hands without our hearing of it, which would leave the watch
			// holding a unique name that has gone and rejecting the new portal's signals for the life of the process.
			// A watch that cannot tell the portal from anyone else is not worth having, so nothing more is asked for.
			// There is nobody to report that to, the watch having been started and forgotten, so it is logged: a
			// desktop that never follows the system light and dark preference for the life of the process would
			// otherwise leave nothing behind to say why.
			errs.Log(errs.NewWithCause("x11: unable to watch the desktop portal", err), "rule", portalOwnerMatchRule)
			cancelOwner()
			return
		}
		// The bus is reporting changes of ownership now, so nothing that happens from here on can be missed, and the
		// name can safely be resolved.
		owner.resolve(conn)
		cancelChange := conn.Subscribe(dbus.SignalFilter{
			Path:      portalPath,
			Interface: portalSettingsInterface,
			Member:    portalSettingChanged,
		}, func(msg *dbus.Message) {
			if !owner.sent(msg) {
				return
			}
			if value, ok := colorSchemeChange(msg); ok {
				onChange(value)
			}
		})
		// The failure is kept in an error of its own: undoing what has been done overwrites err, which would leave the
		// log with the outcome of the cleanup in place of the reason for it.
		if addErr := conn.AddMatch(colorSchemeMatchRule); addErr != nil {
			errs.Log(errs.NewWithCause("x11: unable to watch the desktop portal", addErr), "rule", colorSchemeMatchRule)
			cancelChange()
			cancelOwner()
			if err = conn.RemoveMatch(portalOwnerMatchRule); err != nil {
				errs.Log(errs.NewWithCause("x11: unable to stop watching for the desktop portal", err))
			}
		}
	}()
}

// portalOwner is the unique name of the connection that owns the portal's well-known name, as the bus last reported it.
// It is written from the session connection's dispatcher goroutine, by the NameOwnerChanged handler, and from the watch
// goroutine that resolves it to begin with, and read from the dispatcher goroutine, so all of it is under the lock.
type portalOwner struct {
	name string
	// reports counts what the bus has said, and is what keeps an answer that had to be asked for from writing over
	// something newer: the question is a round trip, and the name can change hands while one is in flight, so an answer
	// is only recorded while the count is still what it was when the question was asked.
	reports uint64
	lock    sync.Mutex
}

// set records the owner that a NameOwnerChanged signal reports, which is the newest thing the bus has said.
func (o *portalOwner) set(name string) {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.reports++
	o.name = name
}

// resolve asks the bus which connection owns the portal's name and records the answer, unless a signal has reported
// something newer while the question was in flight. An empty name — which is what a desktop with no portal answers
// with, and what the bus says of a name nobody owns — leaves every signal unaccounted for and so acted on by nobody.
func (o *portalOwner) resolve(conn *dbus.Conn) {
	o.lock.Lock()
	o.reports++
	wanted := o.reports
	o.lock.Unlock()
	name := askTheBusWhoOwnsThePortal(conn)
	o.lock.Lock()
	defer o.lock.Unlock()
	if o.reports == wanted {
		o.name = name
	}
}

// sent reports whether a signal came from the connection that owns the portal's name. A signal that carries no sender
// at all, which is what a peer to peer connection with no bus on the other end produces, is not from the portal either:
// there is no portal on such a connection, and the only thing that could have sent it is whatever is on the far end.
func (o *portalOwner) sent(msg *dbus.Message) bool {
	o.lock.Lock()
	defer o.lock.Unlock()
	return o.name != "" && msg.Sender == o.name
}

// portalDestinationOrItsOwner returns the name to address a call to the portal to: the unique name of whichever
// connection owns its well-known name, when the bus says something owns it, and the well-known name itself when
// nothing does.
//
// Addressing the unique name is what makes the reply worth something. The bus fills in the SENDER of every message it
// delivers, overwriting whatever the sending connection put there, so a call addressed to a unique name can only be
// completed by that connection or by the bus answering on its behalf; a call addressed to a well-known name is answered
// under a unique name the caller has no way of knowing, so a reply from anyone at all has to be accepted and any peer
// on the session bus that guesses the serial can hand us forged values. See [dbus.Conn.Call].
//
// The fallback is the case where the bus says nothing owns the name. The portal is bus-activatable, and a call to its
// well-known name is what starts it, so refusing to make one would leave a desktop whose portal has not been started
// yet with no color scheme at all. Such a call is exactly as protected as every call made here before, which is to say
// not at all, and it is the only one left in that position.
func portalDestinationOrItsOwner(conn *dbus.Conn) string {
	if owner := askTheBusWhoOwnsThePortal(conn); owner != "" {
		return owner
	}
	return portalDestination
}

// askTheBusWhoOwnsThePortal returns the unique name of the connection that owns the portal's well-known name, or an
// empty string if nothing does or the bus will not say. The call is addressed to the bus's own name, which only the bus
// answers to, so the answer is not something another peer can put words into; see [dbus.Conn.Call].
func askTheBusWhoOwnsThePortal(conn *dbus.Conn) string {
	msg := dbus.NewMethodCall(busDestination, busPath, busInterface, getNameOwner)
	if err := msg.SetBody(portalDestination); err != nil {
		return ""
	}
	reply, err := conn.Call(msg)
	if err != nil {
		return "" // Includes the NameHasNoOwner error a desktop with no portal answers with
	}
	args, err := reply.Args()
	if err != nil || len(args) == 0 {
		return ""
	}
	name, _ := args[0].(string) //nolint:errcheck // Anything but a string means the bus answered something else
	return name
}

// newOwnerOfPortal returns the new owner of the portal's well-known name that a NameOwnerChanged signal reports. ok is
// false for a signal about any other name. The signal's arguments are the name, the owner it had, and the owner it has
// now, which is empty when the name has been given up.
func newOwnerOfPortal(msg *dbus.Message) (owner string, ok bool) {
	args, err := msg.Args()
	if err != nil || len(args) < 3 {
		return "", false
	}
	name, isString := args[0].(string)
	if !isString || name != portalDestination {
		return "", false
	}
	owner, isString = args[2].(string)
	if !isString {
		return "", false
	}
	return owner, true
}

// colorSchemeChange returns the new color-scheme value carried by a SettingChanged signal, or false if the signal is
// for some other setting or is not shaped the way the portal documents (namespace, key, value).
func colorSchemeChange(msg *dbus.Message) (value uint32, ok bool) {
	args, err := msg.Args()
	if err != nil || len(args) < 3 {
		return 0, false
	}
	if namespace, isStr := args[0].(string); !isStr || namespace != colorSchemeNamespace {
		return 0, false
	}
	if key, isStr := args[1].(string); !isStr || key != colorSchemeKey {
		return 0, false
	}
	return colorSchemeValue(args[2])
}

// colorSchemeValue extracts the color scheme from the variant the portal reports it in, transparently unwrapping the
// extra layer of variant that org.freedesktop.portal.Settings.Read is known to wrap it in.
func colorSchemeValue(v any) (value uint32, ok bool) {
	for {
		variant, isVariant := v.(dbus.Variant)
		if !isVariant {
			break
		}
		v = variant.Value
	}
	value, ok = v.(uint32)
	return value, ok
}
