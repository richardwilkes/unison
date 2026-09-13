// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package accessibility

import (
	"strconv"
	"strings"
)

// Action identifies something an assistive technology may ask a node to do. A node advertises the ones it supports in
// its ActionSet, and a request to perform one arrives as an ActionRequest.
type Action uint32

// Possible Action values.
const (
	Focus               Action = iota // Give this node the keyboard focus
	Press                             // Activate this node, as a click would
	Increment                         // Raise the numeric value by one step
	Decrement                         // Lower the numeric value by one step
	SetValue                          // Replace the value with ActionRequest.Value or Number
	Expand                            // Reveal what this node contains
	Collapse                          // Hide what this node contains
	Select                            // Make this node the only selection in its container
	AddToSelection                    // Add this node to its container's selection
	RemoveFromSelection               // Take this node out of its container's selection
	Toggle                            // Move a checkable node to its next state
	ScrollIntoView                    // Scroll ancestors so this node becomes visible
	ShowContextMenu                   // Show the contextual menu for this node
	SetTextSelection                  // Move the caret or selection to ActionRequest.Start and End
	ReplaceText                       // Replace the runes from ActionRequest.Start to End with Value
)

// lastAction is the highest valid Action. Since an ActionSet holds one bit per Action, this is also the highest bit an
// ActionSet can use, and the bound every ActionSet method range-checks against.
const lastAction = ReplaceText

// String implements fmt.Stringer.
func (a Action) String() string {
	switch a {
	case Focus:
		return "focus"
	case Press:
		return "press"
	case Increment:
		return "increment"
	case Decrement:
		return "decrement"
	case SetValue:
		return "set-value"
	case Expand:
		return "expand"
	case Collapse:
		return "collapse"
	case Select:
		return "select"
	case AddToSelection:
		return "add-to-selection"
	case RemoveFromSelection:
		return "remove-from-selection"
	case Toggle:
		return "toggle"
	case ScrollIntoView:
		return "scroll-into-view"
	case ShowContextMenu:
		return "show-context-menu"
	case SetTextSelection:
		return "set-text-selection"
	case ReplaceText:
		return "replace-text"
	default:
		return "Action(" + strconv.FormatUint(uint64(a), 10) + ")"
	}
}

// ActionSet is the set of Actions a node supports, held as a bit per Action.
type ActionSet uint32

// Has returns true if the set contains the given action.
func (s ActionSet) Has(a Action) bool {
	if a > lastAction {
		return false
	}
	return s&(1<<a) != 0
}

// With returns the set with the given actions added.
func (s ActionSet) With(actions ...Action) ActionSet {
	for _, a := range actions {
		if a <= lastAction {
			s |= 1 << a
		}
	}
	return s
}

// Without returns the set with the given actions removed.
func (s ActionSet) Without(actions ...Action) ActionSet {
	for _, a := range actions {
		if a <= lastAction {
			s &= ^(ActionSet(1) << a)
		}
	}
	return s
}

// String implements fmt.Stringer, listing the actions in the set separated by commas. An empty set yields an empty
// string.
func (s ActionSet) String() string {
	var buffer strings.Builder
	for a := Focus; a <= lastAction; a++ {
		if s.Has(a) {
			if buffer.Len() != 0 {
				buffer.WriteString(",")
			}
			buffer.WriteString(a.String())
		}
	}
	return buffer.String()
}

// ActionRequest asks a node to perform an Action. Adapters fill in everything except Key, which the root package
// supplies from its registry when the target turns out to be a virtual child such as a table row or cell.
type ActionRequest struct {
	// Key is the widget-defined key of the virtual child being acted on, or nil when the target is a real panel. Table
	// rows use a tid.TID, list rows an int, and cells a CellKey.
	Key any
	// Value is the replacement text for SetValue and ReplaceText.
	Value string
	// Number is the replacement numeric value for SetValue on a node that has one.
	Number float64
	// Node is the node being asked to act.
	Node NodeID
	// Action is what the node is being asked to do.
	Action Action
	// Start is the first rune index of the range SetTextSelection or ReplaceText applies to.
	Start int
	// End is the rune index just past the range SetTextSelection or ReplaceText applies to.
	End int
}
