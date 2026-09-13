// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import "github.com/richardwilkes/unison/internal/dbus"

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
	// colorSchemeMatchRule asks the bus to deliver the portal's setting change signals to us. Signals from another
	// connection are only delivered to one that has asked for them.
	colorSchemeMatchRule = "type='signal',interface='" + portalSettingsInterface +
		"',member='" + portalSettingChanged + "'"
)

// ReadColorScheme queries the XDG Desktop Portal for the current color scheme preference. It returns the raw value
// (0 = no preference, 1 = prefer dark, 2 = prefer light) and whether the query succeeded. A false result means the
// portal or the setting is unavailable.
func ReadColorScheme() (value uint32, ok bool) {
	conn, err := dbus.Session()
	if err != nil {
		return 0, false
	}
	msg := dbus.NewMethodCall(portalDestination, portalPath, portalSettingsInterface, "Read")
	if err = msg.SetBody(colorSchemeNamespace, colorSchemeKey); err != nil {
		return 0, false
	}
	reply, err := conn.Call(msg)
	if err != nil {
		return 0, false // Includes the ServiceUnknown error a machine with no portal answers with
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
func WatchColorScheme(onChange func(value uint32)) {
	// Reaching the bus and asking it for the signals can take as long as a call does, so, as before, none of it
	// happens on the caller's goroutine.
	go func() {
		conn, err := dbus.Session()
		if err != nil {
			return
		}
		// Subscribe first, so that a signal arriving as soon as the match rule is in place has somewhere to go.
		cancel := conn.Subscribe(dbus.SignalFilter{
			Interface: portalSettingsInterface,
			Member:    portalSettingChanged,
		}, func(msg *dbus.Message) {
			if value, ok := colorSchemeChange(msg); ok {
				onChange(value)
			}
		})
		if err = conn.AddMatch(colorSchemeMatchRule); err != nil {
			cancel()
		}
	}()
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
