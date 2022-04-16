// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "sync/atomic"

// NoUndoID represents an empty undo ID value.
const NoUndoID = 0

var (
	_          Undoable = &UndoEdit[int]{}
	nextUndoID          = int64(NoUndoID + 1)
)

// Undoable defines the required methods an undoable edit must implement.
type Undoable interface {
	// Name returns the localized name of the edit, suitable for displaying in a user interface menu. Note that no
	// leading "Undo " or "Redo " should be part of this name, as the UndoManager will add this.
	Name() string
	// Cost returns a cost factor for this edit. When the cost values of the edits within a given UndoManager exceed the
	// UndoManager's defined cost limit, the oldest edits will be discarded until the cost values are less than or equal
	// to the UndoManager's defined limit. Note that if this method returns a value less than 1, it will be set to 1 for
	// purposes of this calculation.
	Cost() int
	// Undo the state.
	Undo()
	// Redo the state.
	Redo()
	// Absorb gives this edit a chance to absorb a new edit that is about to be added to the manager. If this method
	// returns true, it is assumed this edit has incorporated any necessary state into itself to perform an undo/redo
	// and the other edit will be discarded.
	Absorb(other Undoable) bool
	// Release is called when this edit is no longer needed by the UndoManager.
	Release()
}

// UndoEdit provides a standard Undoable.
type UndoEdit[T any] struct {
	ID          int64
	EditName    string
	EditCost    int
	UndoFunc    func(*UndoEdit[T])
	RedoFunc    func(*UndoEdit[T])
	AbsorbFunc  func(*UndoEdit[T], Undoable) bool
	ReleaseFunc func(*UndoEdit[T])
	BeforeData  T
	AfterData   T
}

// NextUndoID returns the next available undo ID.
func NextUndoID() int64 {
	return atomic.AddInt64(&nextUndoID, 1)
}

// Name implements Undoable
func (e *UndoEdit[T]) Name() string {
	return e.EditName
}

// Cost implements Undoable
func (e *UndoEdit[T]) Cost() int {
	return e.EditCost
}

// Undo implements Undoable
func (e *UndoEdit[T]) Undo() {
	if e.UndoFunc != nil {
		e.UndoFunc(e)
	}
}

// Redo implements Undoable
func (e *UndoEdit[T]) Redo() {
	if e.RedoFunc != nil {
		e.RedoFunc(e)
	}
}

// Absorb implements Undoable
func (e *UndoEdit[T]) Absorb(other Undoable) bool {
	if e.AbsorbFunc != nil {
		return e.AbsorbFunc(e, other)
	}
	return false
}

// Release implements Undoable
func (e *UndoEdit[T]) Release() {
	if e.ReleaseFunc != nil {
		e.ReleaseFunc(e)
	}
}
