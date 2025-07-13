// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"fmt"

	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// UndoManagerProvider defines the method a provider of UndoManager should define.
type UndoManagerProvider interface {
	UndoManager() *UndoManager
}

// UndoManager provides management of an undo/redo stack.
type UndoManager struct {
	recoveryHandler func(error)
	edits           []Undoable
	costLimit       int
	index           int // points to the currently applied edit
}

// NewUndoManager creates a new undo/redo manager.
func NewUndoManager(costLimit int, recoveryHandler func(error)) *UndoManager {
	if costLimit < 1 {
		costLimit = 1
	}
	return &UndoManager{
		recoveryHandler: recoveryHandler,
		costLimit:       costLimit,
		index:           -1,
	}
}

// UndoManagerFor returns the UndoManager for a given Paneler. May return nil.
func UndoManagerFor(paneler Paneler) *UndoManager {
	provider := AncestorOrSelf[UndoManagerProvider](paneler)
	for provider != nil {
		if manager := provider.UndoManager(); manager != nil {
			return manager
		}
		p, ok := provider.(Paneler)
		if !ok {
			return nil
		}
		provider = Ancestor[UndoManagerProvider](p)
	}
	return nil
}

// CostLimit returns the current cost limit permitted by this undo manager.
func (m *UndoManager) CostLimit() int {
	return m.costLimit
}

// SetCostLimit sets a new cost limit, potentially trimming existing edits to fit within the new limit. Note that if the
// most recent edit has a cost larger than the new limit, that last edit (and only that last edit) will still be
// retained.
func (m *UndoManager) SetCostLimit(limit int) {
	old := m.CostLimit()
	if limit < 1 {
		limit = 1
	}
	m.costLimit = limit
	if old > limit {
		m.trimForLimit()
	}
}

// Add an edit. If one or more undos have been performed, this will cause any redo capability beyond this point to be
// lost.
func (m *UndoManager) Add(edit Undoable) {
	for i := m.index + 1; i < len(m.edits); i++ {
		m.release(m.edits[i])
	}
	add := m.index < 0
	if !add {
		absorb := m.edits[m.index].Absorb(edit)
		if absorb {
			m.release(edit)
		}
		add = !absorb
	}
	if add {
		m.index++
	}
	edits := make([]Undoable, m.index+1)
	copy(edits, m.edits)
	if add {
		edits[m.index] = edit
	}
	m.edits = edits
	m.trimForLimit()
}

// CanUndo returns true if Undo() can be called successfully.
func (m *UndoManager) CanUndo() bool {
	return m.index >= 0 && len(m.edits) > 0
}

// Undo rewinds the current state by one edit.
func (m *UndoManager) Undo() {
	if m.CanUndo() {
		xos.SafeCall(m.undo, m.recoveryHandler)
	}
}

func (m *UndoManager) undo() {
	m.edits[m.index].Undo()
	m.index--
}

// UndoTitle returns the title for the current undo state.
func (m *UndoManager) UndoTitle() string {
	if m.CanUndo() {
		return fmt.Sprintf(i18n.Text("Undo %s"), m.edits[m.index].Name())
	}
	return CannotUndoTitle()
}

// CannotUndoTitle returns the Cannot Undo title.
func CannotUndoTitle() string {
	return i18n.Text("Cannot Undo")
}

// CanRedo returns true if Redo() can be called successfully.
func (m *UndoManager) CanRedo() bool {
	return m.index < len(m.edits)-1
}

// Redo re-applies the current state by one edit.
func (m *UndoManager) Redo() {
	if m.CanRedo() {
		xos.SafeCall(m.redo, m.recoveryHandler)
	}
}

func (m *UndoManager) redo() {
	m.index++
	m.edits[m.index].Redo()
}

// RedoTitle returns the title for the current redo state.
func (m *UndoManager) RedoTitle() string {
	if m.CanRedo() {
		return fmt.Sprintf(i18n.Text("Redo %s"), m.edits[m.index+1].Name())
	}
	return CannotRedoTitle()
}

// CannotRedoTitle returns the Cannot Redo title.
func CannotRedoTitle() string {
	return i18n.Text("Cannot Redo")
}

// Clear removes all edits from this UndoManager.
func (m *UndoManager) Clear() {
	for i := range m.edits {
		m.release(m.edits[i])
	}
	m.edits = nil
	m.index = -1
}

func (m *UndoManager) release(edit Undoable) {
	xos.SafeCall(edit.Release, m.recoveryHandler)
}

func (m *UndoManager) cost(edit Undoable) int {
	cost := edit.Cost()
	if cost < 1 {
		return 1
	}
	return cost
}

func (m *UndoManager) trimForLimit() {
	// Start at current index and tally cost moving to beginning. If we run out before reaching the start, then keep
	// just the edits from index to the point we ran out.
	i := m.index
	remaining := m.CostLimit()
	for ; i >= 0; i-- {
		if remaining -= m.cost(m.edits[i]); remaining >= 0 {
			continue
		}
		if i == m.index {
			// If even the current index doesn't fit, retain just the current index.
			for j := range m.edits {
				if j != i {
					m.release(m.edits[j])
				}
			}
			m.edits = []Undoable{m.edits[i]}
			m.index = 0
			return
		}
		// Trim out the edits from this point to the start, plus those after the current index.
		for j := 0; j <= i; j++ {
			m.release(m.edits[j])
		}
		for j := m.index + 1; j < len(m.edits); j++ {
			m.release(m.edits[j])
		}
		edits := make([]Undoable, m.index-i)
		copy(edits, m.edits[i+1:m.index+1])
		m.edits = edits
		m.index -= i + 1
		return
	}
	// If we get here, then all edits up to the current index fit within the cost limit. Look at those beyond the
	// current index and trim out any that go over the limit.
	for i = m.index + 1; i < len(m.edits); i++ {
		if remaining -= m.cost(m.edits[i]); remaining >= 0 {
			continue
		}
		for j := i; j < len(m.edits); j++ {
			m.release(m.edits[j])
		}
		edits := make([]Undoable, i)
		copy(edits, m.edits)
		m.edits = edits
		return
	}
}
