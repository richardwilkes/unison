// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"bytes"
	"slices"

	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
)

// The odds and ends of the platform contract: cursors, the clipboard, the display, menus and file dialogs. Each has a
// pure-Go stand-in that behaves the way the real one does as far as anything above this layer can tell.

// newCursor returns an inert cursor. Nothing consumes cursor pixels in a headless session, and what a test can
// meaningfully assert is which *Cursor a window resolved to rather than what it looks like, so the rasterization is
// skipped entirely. This is the same answer cursor_linux.go gives when there is no X11 connection to build a cursor
// with: a non-nil, distinct Cursor with nothing behind it, deliberately not recorded in cursorList, since there is
// nothing for the teardown to release. The flag is what Destroy dispatches on, so it stays inert even if it outlives
// the session that made it.
func (s *headlessState) newCursor(_ *cursorSource) *Cursor {
	return &Cursor{headless: true}
}

// The clipboard is a plain in-memory list. It belongs to the session, so it starts empty and its contents cannot leak
// into, or out of, whatever the machine running the test happens to have on its real clipboard.

func (s *headlessState) clipboardAvailableDataTypes() []string {
	types := make([]string, 0, len(s.clipboard))
	for _, one := range s.clipboard {
		if one.Type != nil {
			types = append(types, one.Type.UTI)
		}
	}
	return types
}

func (s *headlessState) clipboardHasDataType(dataType *uti.DataType) bool {
	return s.clipboardIndex(dataType) >= 0
}

// clipboardGetData hands back a copy of the bytes rather than the clipboard's own, as the platform clipboards do, so a
// caller writing into what it was given alters nothing that a later reader will see.
func (s *headlessState) clipboardGetData(dataType *uti.DataType) []byte {
	if i := s.clipboardIndex(dataType); i >= 0 {
		return bytes.Clone(s.clipboard[i].Data)
	}
	return nil
}

func (s *headlessState) clipboardSetData(data ...drag.Data) {
	// Replace the content wholesale, as the platform clipboards do, and copy the bytes so a caller that reuses its
	// buffer cannot alter what was placed on the clipboard.
	s.clipboard = make([]drag.Data, 0, len(data))
	for _, one := range data {
		s.clipboard = append(s.clipboard, drag.Data{Type: one.Type, Data: bytes.Clone(one.Data)})
	}
}

// clipboardIndex returns the position of dataType in the clipboard, or -1 if it is not present. An exact match on the
// UTI is enough: clipboard.go's selectDataType has already resolved conformance against clipboardAvailableDataTypes
// before any of these are called.
func (s *headlessState) clipboardIndex(dataType *uti.DataType) int {
	if dataType == nil {
		return -1
	}
	return slices.IndexFunc(s.clipboard, func(one drag.Data) bool {
		return one.Type != nil && one.Type.UTI == dataType.UTI
	})
}

// The session has exactly one display, which is the screen it was configured with. Multi-display configurations are
// out of scope.

func (s *headlessState) primaryDisplay() *Display {
	return s.display
}

func (s *headlessState) allDisplays() []*Display {
	return []*Display{s.display}
}

// Menus use the pure-Go in-window implementation, which is also what the non-macOS platforms use.

func (s *headlessState) newDefaultMenuFactory() MenuFactory {
	return NewInWindowMenuFactory()
}

func (s *headlessState) quitMenuTitle() string {
	return i18n.Text("Quit")
}

// addAppMenuEntries does nothing: the application menu it would add to is a macOS construct.
func (s *headlessState) addAppMenuEntries(_ Menu) {
}

// File dialogs use the pure-Go in-window implementations, so a test can drive them like any other window.

func (s *headlessState) newOpenDialog() OpenDialog {
	return NewCommonOpenDialog()
}

func (s *headlessState) newSaveDialog() SaveDialog {
	return NewCommonSaveDialog()
}
