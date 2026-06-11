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
		if dataType := c.dataTypeForTarget(target); dataType != "" && !slices.Contains(result, dataType) {
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
	for _, target := range c.targetsForDataType(dataType) {
		if value, ok := c.convertClipboardSelection(target); ok && len(value) != 0 {
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
		for i, target := range c.targetsForDataType(d.Type.UTI) {
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
	c.clipboardEntries = entries
	c.setSelectionOwner(c.helperWindow, c.Atoms.Clipboard)
}

// clipboardEntryForTarget returns the stored clipboard entry offered under the given target, if any.
func (c *Conn) clipboardEntryForTarget(target Atom) (entry clipboardEntry, ok bool) {
	for _, entry = range c.clipboardEntries {
		if entry.target == target {
			return entry, true
		}
	}
	return clipboardEntry{}, false
}

// requestClipboardTargets asks the current owner of the CLIPBOARD selection for the list of targets it offers.
func (c *Conn) requestClipboardTargets() []Atom {
	value, ok := c.convertClipboardSelection(c.Atoms.ClipboardTargets)
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

// convertClipboardSelection asks the current owner of the CLIPBOARD selection to convert its contents to the given
// target and returns the resulting data. If the contents are provided incrementally (using the INCR mechanism), it
// handles that as well by repeatedly requesting the property until all data has been received.
func (c *Conn) convertClipboardSelection(target Atom) ([]byte, bool) {
	c.ConvertSelection(c.helperWindow, c.Atoms.Clipboard, target, c.Atoms.ClipboardSelection, 0)
	ev := c.WaitEventsUntil(func(e Event) bool {
		if sne, ok := e.(*SelectionNotifyEvent); ok && sne.Requestor == c.helperWindow && sne.Target == target {
			return true
		}
		return false
	}, clipboardReplyTimeout)
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
	c.PollEvents(filter) // Ensure no existing PropertyNotifyEvent is already pending
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

// targetsForDataType returns the selection targets to use for the given UTI, in order of preference.
func (c *Conn) targetsForDataType(dataType string) []Atom {
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

// dataTypeForTarget returns the UTI for the given selection target, or "" if the target does not represent clipboard
// data. Targets that aren't recognized are returned as-is, since they may be UTIs provided by another unison-based
// application or types that the caller knows how to interpret.
func (c *Conn) dataTypeForTarget(target Atom) string {
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
