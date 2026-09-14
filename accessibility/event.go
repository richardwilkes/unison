// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package accessibility

import "strconv"

// EventKind identifies what an Event reports.
type EventKind uint8

// Possible EventKind values.
const (
	FocusChanged         EventKind = iota // The keyboard focus moved to Event.Node, or off everything when it is zero
	NameChanged                           // The node's Name changed
	DescriptionChanged                    // The node's Description changed
	ValueChanged                          // The node's Value changed
	NumberChanged                         // The node's Number, Min or Max changed
	StateChanged                          // The state named by Event.State changed
	TextInserted                          // Length runes were inserted at Start; Event.New holds them
	TextDeleted                           // Length runes were deleted at Start; Event.Old holds them
	TextSelectionChanged                  // The caret or selection moved to Start for Length runes
	ChildrenChanged                       // The node's list of children changed
	BoundsChanged                         // The node's Bounds changed
	NodeAdded                             // The node joined the tree
	NodeRemoved                           // The node left the tree
	SortChanged                           // The node's Sort direction changed
	RoleChanged                           // The node's Role changed; Old and New hold the role keys
	WindowActivated                       // The window became the active one
	WindowDeactivated                     // The window stopped being the active one
)

// String implements fmt.Stringer.
func (e EventKind) String() string {
	switch e {
	case FocusChanged:
		return "focus-changed"
	case NameChanged:
		return "name-changed"
	case DescriptionChanged:
		return "description-changed"
	case ValueChanged:
		return "value-changed"
	case NumberChanged:
		return "number-changed"
	case StateChanged:
		return "state-changed"
	case TextInserted:
		return "text-inserted"
	case TextDeleted:
		return "text-deleted"
	case TextSelectionChanged:
		return "text-selection-changed"
	case ChildrenChanged:
		return "children-changed"
	case BoundsChanged:
		return "bounds-changed"
	case NodeAdded:
		return "node-added"
	case NodeRemoved:
		return "node-removed"
	case SortChanged:
		return "sort-changed"
	case RoleChanged:
		return "role-changed"
	case WindowActivated:
		return "window-activated"
	case WindowDeactivated:
		return "window-deactivated"
	default:
		return "EventKind(" + strconv.FormatUint(uint64(e), 10) + ")"
	}
}

// State identifies which of a Node's state flags changed in a StateChanged event.
//
// Node.Focused is deliberately absent. A change to the root's Focused means the window became active or stopped being
// active, which is reported as WindowActivated or WindowDeactivated, and a change to any other node's Focused always
// accompanies a change to Tree.Focus, which is reported as FocusChanged. Reporting it here as well would only make
// every focus move produce a second, redundant event.
type State uint8

// Possible State values.
const (
	StateNone State = iota // No state; the event is not a StateChanged
	StateDisabled
	StateFocusable
	StateSelectable
	StateSelected
	StateMultiselectable
	StatePressed
	StateReadOnly
	StateModal
	StateBusy
	StateInvalid
	StateOffscreen
	StateExpandable
	StateExpanded
	StateIgnored
	StateProtected
	StateChecked // Node.Checked or Node.HasCheck changed
)

// String implements fmt.Stringer.
func (s State) String() string {
	switch s {
	case StateNone:
		return "none"
	case StateDisabled:
		return "disabled"
	case StateFocusable:
		return "focusable"
	case StateSelectable:
		return "selectable"
	case StateSelected:
		return "selected"
	case StateMultiselectable:
		return "multiselectable"
	case StatePressed:
		return "pressed"
	case StateReadOnly:
		return "read-only"
	case StateModal:
		return "modal"
	case StateBusy:
		return "busy"
	case StateInvalid:
		return "invalid"
	case StateOffscreen:
		return "offscreen"
	case StateExpandable:
		return "expandable"
	case StateExpanded:
		return "expanded"
	case StateIgnored:
		return "ignored"
	case StateProtected:
		return "protected"
	case StateChecked:
		return "checked"
	default:
		return "State(" + strconv.FormatUint(uint64(s), 10) + ")"
	}
}

// Event is one change an assistive technology should be told about. Which fields carry information depends on Kind; the
// rest are left at their zero values.
type Event struct {
	// Old is the previous value, for the kinds that report one. StateChanged reports "true" or "false", except for
	// StateChecked, which reports the check state's key.
	Old string
	// New is the current value, for the kinds that report one.
	New string
	// Node is the node the event concerns. For NodeRemoved it names a node that is no longer in the new tree.
	Node NodeID
	// Start is the first rune index the event covers, for the text kinds.
	Start int
	// Length is how many runes the event covers, for the text kinds.
	Length int
	// Kind is what the event reports.
	Kind EventKind
	// State names the flag that changed, and is meaningful only when Kind is StateChanged.
	State State
}
