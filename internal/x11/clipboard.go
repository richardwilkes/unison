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
	"bytes"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/drag"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

// clipboardReplyTimeout is the maximum amount of time to wait for the selection owner to respond to a conversion
// request or to deliver the next chunk of an INCR transfer.
const clipboardReplyTimeout = time.Second

// incrSendTimeout is the maximum amount of time to wait for a requestor to consume a chunk of an outgoing INCR
// transfer before abandoning the transfer.
const incrSendTimeout = 5 * time.Second

// incrTransfer holds the state for an in-progress outgoing INCR transfer.
type incrTransfer struct {
	data      []byte
	requestor WindowID
	property  Atom
	kind      Atom
}

// clipboardEntry holds one representation of the clipboard contents that the helper window will provide to other
// clients when they request the associated target.
type clipboardEntry struct {
	dataType string // The UTI this entry was created from; empty for synthesized alternate representations
	data     []byte
	target   Atom // The selection target this entry is offered as
	kind     Atom // The property type to use when providing the data; usually the same as target
}

// ClipboardDataTypes returns the UTIs for the data types currently available on the clipboard.
func (c *Conn) ClipboardDataTypes() []string {
	if c.helperWindow == 0 {
		return nil
	}
	owner, err := c.getSelectionOwner(c.Atoms.Clipboard)
	if err != nil {
		errs.Log(err)
		return nil
	}
	var result []string
	if owner == c.helperWindow {
		for _, entry := range c.clipboardEntries {
			if entry.dataType != "" && !slices.Contains(result, entry.dataType) {
				result = append(result, entry.dataType)
			}
		}
		return result
	}
	if owner == 0 {
		return nil
	}
	for _, target := range c.requestClipboardTargets() {
		if dataType := c.DataTypeForTarget(target); dataType != "" && !slices.Contains(result, dataType) {
			result = append(result, dataType)
		}
	}
	return result
}

// GetClipboardBytes retrieves the current clipboard data for the given UTI by checking the owner of the CLIPBOARD
// selection and requesting the selection contents if the owner is not the helper window. If the clipboard contents are
// provided incrementally (using the INCR mechanism), it handles that as well by repeatedly requesting the property
// until all data has been received.
func (c *Conn) GetClipboardBytes(dataType string) []byte {
	if c.helperWindow == 0 || dataType == "" {
		return nil
	}
	owner, err := c.getSelectionOwner(c.Atoms.Clipboard)
	if err != nil {
		errs.Log(err)
		return nil
	}
	if owner == c.helperWindow {
		for _, entry := range c.clipboardEntries {
			if entry.dataType == dataType {
				return entry.data
			}
		}
		return nil
	}
	if owner == 0 {
		return nil
	}
	for _, target := range c.TargetsForDataType(dataType) {
		if value, ok := c.convertSelection(c.Atoms.Clipboard, target, 0); ok && len(value) != 0 {
			if target == AtomString {
				value = convertLatin1ToUTF8(value)
			}
			return value
		}
	}
	return nil
}

// SetClipboardData sets the clipboard contents by storing the provided data in the connection and claiming ownership
// of the CLIPBOARD selection with a helper window. When another client requests the clipboard contents, the helper
// window will provide the stored data. Text data is also offered under the conventional X11 text targets (UTF8_STRING,
// TEXT, STRING) in addition to its MIME types so that other applications can find it.
func (c *Conn) SetClipboardData(data ...drag.Data) {
	if c.helperWindow == 0 {
		return
	}
	c.clipboardEntries = c.buildSelectionEntries(data...)
	c.setSelectionOwner(c.helperWindow, c.Atoms.Clipboard)
}

// buildSelectionEntries converts the provided data into the entries that the helper window will offer to other clients
// when they request the contents of a selection it owns.
func (c *Conn) buildSelectionEntries(data ...drag.Data) []clipboardEntry {
	var entries []clipboardEntry
	seen := make(map[Atom]bool)
	add := func(dataType string, target, kind Atom, content []byte) {
		if target == AtomNone || seen[target] {
			return
		}
		seen[target] = true
		entries = append(entries, clipboardEntry{dataType: dataType, data: content, target: target, kind: kind})
	}
	for _, d := range data {
		isText := uti.UTF8PlainText.ConformsTo(d.Type)
		for i, target := range c.TargetsForDataType(d.Type.UTI) {
			var dataType string
			if i == 0 {
				dataType = d.Type.UTI
			}
			content := d.Data
			if isText && target == AtomString {
				content = convertUTF8ToLatin1(d.Data)
			}
			add(dataType, target, target, content)
		}
		if isText {
			add("", c.Atoms.Text, c.Atoms.UTF8String, d.Data)
		}
	}
	return entries
}

// selectionEntries returns the stored entries for the given selection.
func (c *Conn) selectionEntries(selection Atom) []clipboardEntry {
	if selection == c.Atoms.DnDSelection {
		return c.dndEntries
	}
	return c.clipboardEntries
}

// entryForTarget returns the entry offered under the given target, if any.
func entryForTarget(entries []clipboardEntry, target Atom) (entry clipboardEntry, ok bool) {
	for _, entry = range entries {
		if entry.target == target {
			return entry, true
		}
	}
	return clipboardEntry{}, false
}

// requestClipboardTargets asks the current owner of the CLIPBOARD selection for the list of targets it offers.
func (c *Conn) requestClipboardTargets() []Atom {
	value, ok := c.convertSelection(c.Atoms.Clipboard, c.Atoms.ClipboardTargets, 0)
	if !ok || len(value)%4 != 0 {
		return nil
	}
	targets := make([]Atom, 0, len(value)/4)
	r := NewReader(value)
	for range len(value) / 4 {
		targets = append(targets, r.Atom())
	}
	return targets
}

// convertSelection asks the current owner of the given selection to convert its contents to the given target and
// returns the resulting data. If the contents are provided incrementally (using the INCR mechanism), it handles that
// as well by repeatedly requesting the property until all data has been received.
func (c *Conn) convertSelection(selection, target Atom, timestamp uint32) ([]byte, bool) {
	sneFilter := func(e Event) bool {
		sne, valid := e.(*SelectionNotifyEvent)
		return valid && sne.Requestor == c.helperWindow && sne.Selection == selection && sne.Target == target
	}
	// A conversion that previously timed out can leave matching SelectionNotify and PropertyNotify events queued
	// (filtered waits keep non-matching events around forever). Drain all of them before starting a new conversion so
	// a stale event can't be mistaken for a response to this request, which could truncate an INCR transfer by making
	// the loop below read the property before the owner has written the next chunk.
	c.drainEvents(func(e Event) bool {
		if sneFilter(e) {
			return true
		}
		pne, valid := e.(*PropertyNotifyEvent)
		return valid && pne.State == PropertyNewValue && pne.Window == c.helperWindow &&
			pne.Atom == c.Atoms.ClipboardSelection
	})
	c.ConvertSelection(c.helperWindow, selection, target, c.Atoms.ClipboardSelection, timestamp)
	ev := c.WaitEventsUntil(sneFilter, clipboardReplyTimeout)
	sne, ok := ev.(*SelectionNotifyEvent)
	if !ok || sne.Property == AtomNone {
		return nil, false
	}
	filter := func(e Event) bool {
		if pne, valid := e.(*PropertyNotifyEvent); valid && pne.State == PropertyNewValue &&
			pne.Window == sne.Requestor && pne.Atom == sne.Property {
			return true
		}
		return false
	}
	c.drainEvents(filter) // Discard notifications for writes that happened before the SelectionNotify arrived
	_, propertyType, value, _, err := c.GetProperty(sne.Requestor, sne.Property, AtomAny, 0, math.MaxUint32, true)
	if err != nil {
		errs.Log(err)
		return nil, false
	}
	if propertyType != c.Atoms.ClipboardIncremental {
		return value, true
	}
	var buffer bytes.Buffer
	for {
		if xreflect.IsNil(c.WaitEventsUntil(filter, clipboardReplyTimeout)) {
			return nil, false
		}
		if _, _, value, _, err = c.GetProperty(sne.Requestor, sne.Property, AtomAny, 0, math.MaxUint32,
			true); err != nil {
			errs.Log(err)
			return nil, false
		}
		if len(value) == 0 {
			return buffer.Bytes(), true
		}
		buffer.Write(value)
	}
}

// TargetsForDataType returns the selection targets to use for the given UTI, in order of preference.
func (c *Conn) TargetsForDataType(dataType string) []Atom {
	var result []Atom
	add := func(target Atom) {
		if target != AtomNone && !slices.Contains(result, target) {
			result = append(result, target)
		}
	}
	dt := uti.ByUTI(dataType)
	isText := dt != nil && uti.UTF8PlainText.ConformsTo(dt)
	if isText {
		add(c.Atoms.UTF8String)
	}
	if dt != nil {
		for _, mimeType := range dt.MimeTypes {
			add(c.lookupDataTypeAtom(mimeType))
		}
	}
	if isText {
		add(AtomString)
	}
	add(c.lookupDataTypeAtom(dataType))
	return result
}

// DataTypeForTarget returns the UTI for the given selection target, or "" if the target does not represent selection
// data. Targets that aren't recognized are returned as-is, since they may be UTIs provided by another unison-based
// application or types that the caller knows how to interpret.
func (c *Conn) DataTypeForTarget(target Atom) string {
	switch target {
	case AtomNone:
		return ""
	case c.Atoms.UTF8String, AtomString, c.Atoms.Text:
		return uti.UTF8PlainText.UTI
	case c.Atoms.ClipboardTargets, c.Atoms.ClipboardMultiple, c.Atoms.ClipboardSaveTargets:
		return ""
	}
	c.dataTypeMapLock.RLock()
	dataType, ok := c.reverseDataTypeMap[target]
	c.dataTypeMapLock.RUnlock()
	if ok {
		return dataType
	}
	name, err := c.GetAtomName(target)
	if err != nil {
		errs.Log(err)
		return ""
	}
	switch {
	case strings.HasPrefix(name, "text/plain"):
		dataType = uti.UTF8PlainText.UTI
	case slices.Contains([]string{"TIMESTAMP", "COMPOUND_TEXT", "DELETE", "INSERT_PROPERTY", "INSERT_SELECTION"}, name):
		dataType = "" // Protocol side-effect targets and encodings we can't decode, not actual data
	default:
		if matches := uti.ByMimeType(name); len(matches) != 0 {
			dataType = matches[0].UTI
		} else {
			dataType = name
		}
	}
	c.dataTypeMapLock.Lock()
	c.reverseDataTypeMap[target] = dataType
	c.dataTypeMapLock.Unlock()
	return dataType
}

// lookupDataTypeAtom returns the Atom for the given data type name, interning it if necessary.
func (c *Conn) lookupDataTypeAtom(dataType string) Atom {
	c.dataTypeMapLock.RLock()
	a, ok := c.dataTypeMap[dataType]
	c.dataTypeMapLock.RUnlock()
	if ok {
		return a
	}
	var err error
	if a, err = c.InternAtom(dataType, false); err != nil {
		errs.Log(err)
		return AtomNone
	}
	c.dataTypeMapLock.Lock()
	c.dataTypeMap[dataType] = a
	c.dataTypeMapLock.Unlock()
	return a
}

// incrThreshold returns the maximum number of bytes that will be written to a selection property in one shot. Per the
// ICCCM, selections larger than the maximum request size should be transferred incrementally (using the INCR
// mechanism).
func (c *Conn) incrThreshold() int {
	return int(c.maximumRequestLength) * 4
}

// writeClipboardProperty writes the given clipboard entry to a property on the requestor's window. If the data is
// small enough, it is written directly and nil is returned. Otherwise, an INCR transfer is initiated by writing the
// total size to the property and selecting for property change events on the requestor's window, and the returned
// transfer must be completed with completeIncrTransfers after the SelectionNotify event has been sent.
func (c *Conn) writeClipboardProperty(requestor WindowID, property Atom, entry clipboardEntry) *incrTransfer {
	if len(entry.data) <= c.incrThreshold() {
		c.ChangeProperty(requestor, property, entry.kind, 8, PropModeReplace, entry.data)
		return nil
	}
	// Mirror the receive-side drain in convertSelection: an earlier transfer to the same requestor and property that
	// was abandoned after incrSendTimeout can leave a matching PropertyDelete queued (filtered waits keep non-matching
	// events around forever). If it survived into completeIncrTransfers, the first wait there would consume it and
	// write chunk 1 immediately, replacing the INCR size marker before the requestor has read it and corrupting the
	// transfer. The drain must happen here, before the size marker is written, since once the marker is on the wire a
	// matching PropertyDelete may be the requestor legitimately consuming it.
	c.drainEvents(func(e Event) bool {
		pne, ok := e.(*PropertyNotifyEvent)
		return ok && pne.State == PropertyDelete && pne.Window == requestor && pne.Atom == property
	})
	c.ChangeWindowAttributes(requestor, WindowMaskEventMask,
		&WindowCreationAttributes{EventMask: EventMaskPropertyChange})
	w := NewWriter(4)
	w.Uint32(uint32(len(entry.data)))
	c.ChangeProperty(requestor, property, c.Atoms.ClipboardIncremental, 32, PropModeReplace, w.Retrieve())
	return &incrTransfer{
		data:      entry.data,
		requestor: requestor,
		property:  property,
		kind:      entry.kind,
	}
}

// completeIncrTransfers performs the chunked data transfers for any INCR transfers begun by writeClipboardProperty.
// Each time the requestor consumes (deletes) the property, the next chunk is written to it, with a final zero-length
// write signaling the end of the transfer.
func (c *Conn) completeIncrTransfers(transfers []*incrTransfer) {
	if len(transfers) == 0 {
		return
	}
	// Each chunk must be written with a single ChangeProperty request, since the requestor treats every property
	// change as a complete chunk. Larger writes would be split into multiple requests by ChangeProperty, allowing the
	// requestor to read and delete a partially written chunk. Bound the chunk to the server's maximum request length
	// (the same limit that triggers INCR in the first place), leaving room for the ChangeProperty request header.
	chunkSize := c.incrThreshold() - 24
	for _, t := range transfers {
		offset := 0
		for {
			ev := c.WaitEventsUntil(func(e Event) bool {
				pne, ok := e.(*PropertyNotifyEvent)
				return ok && pne.State == PropertyDelete && pne.Window == t.requestor && pne.Atom == t.property
			}, incrSendTimeout)
			if xreflect.IsNil(ev) {
				break // The requestor stopped responding, so abandon the transfer
			}
			size := min(chunkSize, len(t.data)-offset)
			c.ChangeProperty(t.requestor, t.property, t.kind, 8, PropModeReplace, t.data[offset:offset+size])
			if size == 0 {
				break
			}
			offset += size
		}
	}
	// Stop listening for property change events on the requestor windows
	notified := make(map[WindowID]bool)
	for _, t := range transfers {
		if !notified[t.requestor] {
			notified[t.requestor] = true
			c.ChangeWindowAttributes(t.requestor, WindowMaskEventMask,
				&WindowCreationAttributes{EventMask: EventMaskNone})
		}
	}
}

func convertLatin1ToUTF8(latin1 []byte) []byte {
	s, err := charmap.ISO8859_1.NewDecoder().Bytes(latin1)
	if err != nil {
		errs.Log(err)
		return latin1
	}
	return s
}

func convertUTF8ToLatin1(utf8 []byte) []byte {
	s, err := encoding.ReplaceUnsupported(charmap.ISO8859_1.NewEncoder()).Bytes(utf8)
	if err != nil {
		errs.Log(err)
		return utf8
	}
	return s
}
