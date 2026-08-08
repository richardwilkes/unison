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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
)

// newEntriesTestConn returns a Conn that can build selection entries without a server: the atoms the text targets need
// are assigned directly and every data type atom the test asks for is pre-seeded, so lookupDataTypeAtom never has to
// intern anything over the wire.
func newEntriesTestConn(dataTypes ...string) *Conn {
	conn := &Conn{dataTypeMap: make(map[string]Atom)}
	conn.Atoms.UTF8String = Atom(100)
	conn.Atoms.Text = Atom(101)
	for i, dataType := range dataTypes {
		conn.dataTypeMap[dataType] = Atom(200 + i)
	}
	return conn
}

// dataTypeForEntries returns the UTI the entry offered under the given target was labeled with.
func dataTypeForEntries(entries []clipboardEntry, target Atom) string {
	for _, entry := range entries {
		if entry.target == target {
			return entry.dataType
		}
	}
	return ""
}

// dataForEntries returns the bytes the entry offered under the given target serves.
func dataForEntries(entries []clipboardEntry, target Atom) []byte {
	for _, entry := range entries {
		if entry.target == target {
			return entry.data
		}
	}
	return nil
}

// TestBuildSelectionEntriesLabelsEveryDataType verifies that each data item's UTI reaches an entry the item actually
// contributes. The label used to be pinned to the item's first target, which is UTF8_STRING for every text-conforming
// flavor, so a second such flavor lost its label to the first item's claim on that target. The helper window still
// owned the selection and served the bytes, but ClipboardDataTypes omitted the UTI and an in-process GetClipboardBytes
// for it returned nil, even though the data had just been placed on the clipboard.
func TestBuildSelectionEntriesLabelsEveryDataType(t *testing.T) {
	c := check.New(t)
	const plainMime = "text/plain"
	conn := newEntriesTestConn(uti.UTF8PlainText.UTI, uti.PlainText.UTI, plainMime,
		"text/plain;charset=utf-8", `text/plain;charset="utf-8"`)
	utf8Data := []byte("utf8 flavor")
	plainData := []byte("plain flavor")
	entries := conn.buildSelectionEntries(
		drag.Data{Type: uti.UTF8PlainText, Data: utf8Data},
		drag.Data{Type: uti.PlainText, Data: plainData},
	)

	// The first item leads with UTF8_STRING, so it takes both that target and the label there.
	c.Equal(uti.UTF8PlainText.UTI, dataTypeForEntries(entries, conn.Atoms.UTF8String))

	// The second item's first target is that same already-claimed UTF8_STRING, so its label has to move to the first
	// target it does contribute.
	labeled := make(map[string][]byte)
	for _, entry := range entries {
		if entry.dataType != "" {
			if _, exists := labeled[entry.dataType]; !exists {
				labeled[entry.dataType] = entry.data
			}
		}
	}
	c.Equal(2, len(labeled), "every data item must contribute a labeled entry")
	c.Equal(utf8Data, labeled[uti.UTF8PlainText.UTI])
	c.Equal(plainData, labeled[uti.PlainText.UTI])

	// The label must land on a target that actually serves the second item's bytes.
	for _, entry := range entries {
		if entry.dataType == uti.PlainText.UTI {
			c.Equal(plainData, entry.data, "the labeled entry must carry the item's data")
		}
	}

	// Each UTI is labeled exactly once, so ClipboardDataTypes reports no duplicates.
	counts := make(map[string]int)
	for _, entry := range entries {
		if entry.dataType != "" {
			counts[entry.dataType]++
		}
	}
	c.Equal(1, counts[uti.UTF8PlainText.UTI])
	c.Equal(1, counts[uti.PlainText.UTI])

	// A target is still only ever offered once, by whichever item claimed it first.
	seen := make(map[Atom]int)
	for _, entry := range entries {
		seen[entry.target]++
	}
	for target, count := range seen {
		c.Equal(1, count, "target %d must be offered exactly once", target)
	}
}

// TestBuildSelectionEntriesSingleTextFlavor verifies the ordinary single-item case still labels its leading target and
// offers the conventional text aliases.
func TestBuildSelectionEntriesSingleTextFlavor(t *testing.T) {
	c := check.New(t)
	conn := newEntriesTestConn(uti.UTF8PlainText.UTI, "text/plain;charset=utf-8", `text/plain;charset="utf-8"`)
	data := []byte("hello")
	entries := conn.buildSelectionEntries(drag.Data{Type: uti.UTF8PlainText, Data: data})

	c.Equal(uti.UTF8PlainText.UTI, dataTypeForEntries(entries, conn.Atoms.UTF8String))
	// STRING carries the Latin-1 form and TEXT is a synthesized alias, so neither one is labeled.
	c.Equal("", dataTypeForEntries(entries, AtomString))
	c.Equal("", dataTypeForEntries(entries, conn.Atoms.Text))
	c.Equal(data, dataForEntries(entries, conn.Atoms.Text))
}
