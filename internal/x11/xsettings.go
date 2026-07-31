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
	"math"
	"strconv"
	"strings"
)

// XSETTINGS (https://specifications.freedesktop.org/xsettings-spec/xsettings-spec-0.5.html) is the mechanism GTK-based
// desktops (GNOME on X11, Cinnamon, MATE, XFCE, Budgie, ...) use to publish the active GTK theme to applications. It is
// consulted as a fallback for dark-mode detection on desktops that do not implement the newer XDG Desktop Portal
// "color-scheme" appearance setting. A theme is considered dark when "Gtk/ApplicationPreferDarkTheme" is set or when
// the theme name contains "dark".

const (
	xSettingsTypeInteger = iota
	xSettingsTypeString
	xSettingsTypeColor
)

// xSettings holds the state needed to read and watch the XSETTINGS manager.
type xSettings struct {
	selection Atom
	settings  Atom
	manager   Atom
	window    WindowID
	dark      bool
	ok        bool
}

// InitXSettings locates the XSETTINGS manager, subscribes to changes on it, and reads the initial value. It is safe to
// call when no manager is present; XSettingsDark will simply report that the value is unavailable.
func (c *Conn) InitXSettings() {
	xs := &xSettings{}
	c.xset = xs
	var err error
	if xs.selection, err = c.InternAtom("_XSETTINGS_S"+strconv.Itoa(c.DefaultScreen), false); err != nil {
		return
	}
	if xs.settings, err = c.InternAtom("_XSETTINGS_SETTINGS", false); err != nil {
		return
	}
	if xs.manager, err = c.InternAtom("MANAGER", false); err == nil {
		// Watch the root window for MANAGER ClientMessages so that a restarted settings daemon (a new selection owner)
		// is picked up; without this, dark-mode tracking would silently stop working until the application restarted.
		// Per the XSETTINGS spec, this must be selected before the selection owner is queried so an ownership change
		// cannot slip between the two.
		c.ChangeWindowAttributes(c.RootWindow(), WindowMaskEventMask,
			&WindowCreationAttributes{EventMask: EventMaskStructureNotify})
	}
	c.resolveXSettingsManager()
}

// XSettingsHandleManagerMessage processes a MANAGER ClientMessage broadcast on the root window. When the message
// announces a new owner for this screen's XSETTINGS selection (data32[1] holds the selection atom), the manager window
// is re-resolved and the settings re-read. It returns whether the message was consumed and whether the dark-mode state
// changed as a result.
func (c *Conn) XSettingsHandleManagerMessage(ev *ClientMessageEvent) (handled, changed bool) {
	xs := c.xset
	if xs == nil || xs.manager == AtomNone || ev.Type != xs.manager || Atom(ev.Data32[1]) != xs.selection {
		return false, false
	}
	prevDark, prevOK := xs.dark, xs.ok
	c.resolveXSettingsManager()
	return true, xs.dark != prevDark || xs.ok != prevOK
}

// resolveXSettingsManager finds the current manager window, watches it for property changes, and reads its value.
func (c *Conn) resolveXSettingsManager() {
	xs := c.xset
	if xs == nil || xs.selection == AtomNone {
		return
	}
	owner, err := c.getSelectionOwner(xs.selection)
	if err != nil || owner == 0 {
		xs.window = 0
		xs.ok = false
		return
	}
	xs.window = owner
	// Select for property changes on the manager window so we are notified when the theme changes.
	c.ChangeWindowAttributes(owner, WindowMaskEventMask, &WindowCreationAttributes{EventMask: EventMaskPropertyChange})
	c.readXSettings()
}

// XSettingsManagerWindow returns the current XSETTINGS manager window, or 0 if none is present.
func (c *Conn) XSettingsManagerWindow() WindowID {
	if c.xset == nil {
		return 0
	}
	return c.xset.window
}

// XSettingsDark reports the dark-mode state derived from XSETTINGS and whether it could be determined.
func (c *Conn) XSettingsDark() (dark, ok bool) {
	if c.xset == nil {
		return false, false
	}
	return c.xset.dark, c.xset.ok
}

// RefreshXSettings re-reads the XSETTINGS property and reports whether the dark-mode state changed. Call this when a
// PropertyNotify is received for the manager window.
func (c *Conn) RefreshXSettings() (changed bool) {
	if c.xset == nil {
		return false
	}
	prevDark, prevOK := c.xset.dark, c.xset.ok
	c.readXSettings()
	return c.xset.dark != prevDark || c.xset.ok != prevOK
}

func (c *Conn) readXSettings() {
	xs := c.xset
	if xs == nil || xs.window == 0 {
		if xs != nil {
			xs.ok = false
		}
		return
	}
	format, _, value, _, err := c.GetProperty(xs.window, xs.settings, xs.settings, 0, math.MaxUint32, false)
	if err != nil || format != 8 {
		xs.ok = false
		return
	}
	xs.dark, xs.ok = parseXSettingsDark(value)
}

// parseXSettingsDark decodes an XSETTINGS property blob and determines whether a dark theme is active.
func parseXSettingsDark(b []byte) (dark, ok bool) {
	if len(b) < 12 {
		return false, false
	}
	littleEndian := b[0] == 0 // CARD8 byte-order: 0 = LSBFirst, 1 = MSBFirst.
	u16 := func(p int) uint16 {
		if littleEndian {
			return uint16(b[p]) | uint16(b[p+1])<<8
		}
		return uint16(b[p])<<8 | uint16(b[p+1])
	}
	u32 := func(p int) uint32 {
		if littleEndian {
			return uint32(b[p]) | uint32(b[p+1])<<8 | uint32(b[p+2])<<16 | uint32(b[p+3])<<24
		}
		return uint32(b[p])<<24 | uint32(b[p+1])<<16 | uint32(b[p+2])<<8 | uint32(b[p+3])
	}

	count := u32(8)
	pos := 12
	var preferDark, themeDark, found bool
loop:
	for range count {
		if pos+4 > len(b) {
			break
		}
		settingType := b[pos]
		nameLen := int(u16(pos + 2))
		pos += 4
		if pos+nameLen > len(b) {
			break
		}
		name := string(b[pos : pos+nameLen])
		pos = align4(pos + nameLen)
		if pos+4 > len(b) { // Skip the last-change-serial.
			break
		}
		pos += 4
		switch settingType {
		case xSettingsTypeInteger:
			if pos+4 > len(b) {
				break loop
			}
			if name == "Gtk/ApplicationPreferDarkTheme" {
				preferDark = u32(pos) != 0
				found = true
			}
			pos += 4
		case xSettingsTypeString:
			if pos+4 > len(b) {
				break loop
			}
			valLen := int(u32(pos))
			pos += 4
			if pos+valLen > len(b) {
				break loop
			}
			if name == "Net/ThemeName" || name == "Gtk/ThemeName" {
				s := strings.ToLower(string(b[pos : pos+valLen]))
				if strings.Contains(s, "dark") || strings.Contains(s, "black") {
					themeDark = true
				}
				found = true
			}
			pos = align4(pos + valLen)
		case xSettingsTypeColor:
			pos += 8
		default:
			return false, false // Unknown setting type; we cannot safely continue parsing.
		}
	}
	if !found {
		return false, false
	}
	return preferDark || themeDark, true
}

func align4(p int) int {
	return (p + 3) &^ 3
}
