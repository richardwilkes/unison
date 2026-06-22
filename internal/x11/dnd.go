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
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
)

// DnDVersion is the version of the XDND protocol that is implemented here.
const DnDVersion = 5

// SetDnDData stores the data for an outgoing drag and claims ownership of the XdndSelection with the helper window.
// When the drop target requests the drag contents, the helper window will provide the stored data. Data whose type
// conforms to a URL is also offered under the conventional text/uri-list target so that other applications can find
// it. The data remains available until the next call to SetDnDData, so that requestors that are slow to ask for it
// after a drop are still able to retrieve it.
func (c *Conn) SetDnDData(data ...drag.Data) {
	if c.helperWindow == 0 {
		return
	}
	entries := c.buildSelectionEntries(data...)
	if _, ok := entryForTarget(entries, c.Atoms.TextURIList); !ok {
		for _, d := range data {
			if d.Type.ConformsTo(uti.URL) {
				entries = append(entries, clipboardEntry{
					data:   d.Data,
					target: c.Atoms.TextURIList,
					kind:   c.Atoms.TextURIList,
				})
				break
			}
		}
	}
	c.dndEntries = entries
	c.setSelectionOwner(c.helperWindow, c.Atoms.DnDSelection)
}

// DnDTargets returns the targets being offered for the current outgoing drag.
func (c *Conn) DnDTargets() []Atom {
	targets := make([]Atom, 0, len(c.dndEntries))
	for _, entry := range c.dndEntries {
		targets = append(targets, entry.target)
	}
	return targets
}

// DnDSelectionBytes returns the data for the given XdndSelection target. If this application is the source of the
// drag, the data is returned directly from the stored drag data. Otherwise, the selection is converted by asking the
// drag source for the data. Data for the STRING target is converted from Latin-1 to UTF-8.
func (c *Conn) DnDSelectionBytes(target Atom, timestamp uint32) ([]byte, bool) {
	if c.helperWindow == 0 || target == AtomNone {
		return nil, false
	}
	owner, err := c.getSelectionOwner(c.Atoms.DnDSelection)
	if err != nil {
		errs.Log(err)
		return nil, false
	}
	var value []byte
	var ok bool
	switch owner {
	case c.helperWindow:
		var entry clipboardEntry
		if entry, ok = entryForTarget(c.dndEntries, target); ok {
			value = entry.data
		}
	case 0:
		return nil, false
	default:
		value, ok = c.convertSelection(c.Atoms.DnDSelection, target, timestamp)
	}
	if ok && target == AtomString {
		value = convertLatin1ToUTF8(value)
	}
	return value, ok
}

// SendDnDEnter sends an XdndEnter message to the destination window, announcing the start of a drag over it and the
// targets being offered. If more than 3 targets are offered, the full list is stored in the XdndTypeList property on
// the source window, as required by the XDND protocol.
func (c *Conn) SendDnDEnter(src, dst WindowID, version uint32, targets []Atom) {
	msg := ClientMessageEvent{
		Window: dst,
		Type:   c.Atoms.DnDEnter,
		Format: 32,
		Data32: [5]uint32{uint32(src), version << 24, 0, 0, 0},
	}
	if len(targets) > 3 {
		msg.Data32[1] |= 1
		w := NewWriter(4 * len(targets))
		for _, target := range targets {
			w.Atom(target)
		}
		c.ChangeProperty(src, c.Atoms.DnDTypeList, AtomAtom, 32, PropModeReplace, w.Retrieve())
	}
	for i, target := range targets {
		if i > 2 {
			break
		}
		msg.Data32[2+i] = uint32(target)
	}
	if err := c.sendEvent(dst, false, 0, &msg); err != nil {
		errs.Log(err)
	}
}

// SendDnDPosition sends an XdndPosition message to the destination window, providing the current pointer position in
// root coordinates and the suggested action.
func (c *Conn) SendDnDPosition(src, dst WindowID, rootX, rootY int16, timestamp uint32, action Atom) {
	if err := c.sendEvent(dst, false, 0, &ClientMessageEvent{
		Window: dst,
		Type:   c.Atoms.DnDPosition,
		Format: 32,
		Data32: [5]uint32{
			uint32(src),
			0,
			uint32(uint16(rootX))<<16 | uint32(uint16(rootY)),
			timestamp,
			uint32(action),
		},
	}); err != nil {
		errs.Log(err)
	}
}

// SendDnDLeave sends an XdndLeave message to the destination window, indicating that the drag has left it or has been
// canceled.
func (c *Conn) SendDnDLeave(src, dst WindowID) {
	if err := c.sendEvent(dst, false, 0, &ClientMessageEvent{
		Window: dst,
		Type:   c.Atoms.DnDLeave,
		Format: 32,
		Data32: [5]uint32{uint32(src), 0, 0, 0, 0},
	}); err != nil {
		errs.Log(err)
	}
}

// SendDnDDrop sends an XdndDrop message to the destination window, indicating that the drag data was released over it.
func (c *Conn) SendDnDDrop(src, dst WindowID, timestamp uint32) {
	if err := c.sendEvent(dst, false, 0, &ClientMessageEvent{
		Window: dst,
		Type:   c.Atoms.DnDDrop,
		Format: 32,
		Data32: [5]uint32{uint32(src), 0, timestamp, 0, 0},
	}); err != nil {
		errs.Log(err)
	}
}

// SendDnDStatus sends an XdndStatus message to the source window in response to an XdndPosition message, indicating
// whether a drop would be accepted at the current position and, if so, the action that would be taken.
func (c *Conn) SendDnDStatus(src, dst WindowID, accept bool, action Atom) {
	var flags uint32 = 2 // Request XdndPosition messages on every pointer move
	if accept {
		flags |= 1
	}
	if err := c.sendEvent(src, false, 0, &ClientMessageEvent{
		Window: src,
		Type:   c.Atoms.DnDStatus,
		Format: 32,
		Data32: [5]uint32{uint32(dst), flags, 0, 0, uint32(action)},
	}); err != nil {
		errs.Log(err)
	}
}

// SendDnDFinished sends an XdndFinished message to the source window in response to an XdndDrop message, indicating
// whether the drop was accepted and, if so, the action that was taken.
func (c *Conn) SendDnDFinished(src, dst WindowID, accepted bool, action Atom) {
	var flags uint32
	if accepted {
		flags = 1
	} else {
		action = AtomNone
	}
	if err := c.sendEvent(src, false, 0, &ClientMessageEvent{
		Window: src,
		Type:   c.Atoms.DnDFinished,
		Format: 32,
		Data32: [5]uint32{uint32(dst), flags, uint32(action), 0, 0},
	}); err != nil {
		errs.Log(err)
	}
}
