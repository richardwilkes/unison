// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import "github.com/richardwilkes/unison/drag"

// DropEffect represents the effect of a drag-and-drop operation.
type DropEffect uint32

// Possible values for DropEffect.
const (
	DropEffectCopy DropEffect = 1 << iota
	DropEffectMove
	DropEffectLink
	DropEffectNone   DropEffect = 0
	DropEffectScroll DropEffect = 0x80000000
)

func dropEffectToOp(effect DropEffect) drag.Op {
	var op drag.Op
	if effect&DropEffectCopy != 0 {
		op |= drag.Copy
	}
	if effect&DropEffectMove != 0 {
		op |= drag.Move
	}
	return op
}

func opToDropEffect(op drag.Op) DropEffect {
	switch {
	case op&drag.Copy != 0:
		return DropEffectCopy
	case op&drag.Move != 0:
		return DropEffectMove
	default:
		return DropEffectNone
	}
}

// dropResultEffect determines the effect to report back to the drag source from IDropTarget::Drop. When the drop
// handler accepted the drop, the effect must reflect the operation that was in force from the last DragEnter/DragOver,
// since a source performing a Move relies on it to know whether to delete the original.
func dropResultEffect(accepted bool, lastOp drag.Op) DropEffect {
	if !accepted {
		return DropEffectNone
	}
	return opToDropEffect(lastOp)
}

// OpMaskToDropEffect converts a drag.Op mask to a Windows DropEffect bitmask.
func OpMaskToDropEffect(op drag.Op) DropEffect {
	var effect DropEffect
	if op&drag.Copy != 0 {
		effect |= DropEffectCopy
	}
	if op&drag.Move != 0 {
		effect |= DropEffectMove
	}
	return effect
}
