// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package role

// IsText returns true if this role presents a body of text that assistive technologies may navigate through rather
// than just a single value: TextField, TextArea, SpinButton, ComboBox and Document. These are exactly the roles for
// which the snapshot builder fills in the text information of a node — caret, selection and line boundaries — so
// asking this question is the cheapest way to decide whether that information is worth looking for. Label and Heading
// are deliberately excluded: their text is static and is reported as the node's name instead.
func (e Enum) IsText() bool {
	switch e {
	case TextField, TextArea, SpinButton, ComboBox, Document:
		return true
	default:
		return false
	}
}

// IsContainer returns true if this role exists primarily to hold other elements rather than to present a value of its
// own: Group, List, Table, Tree, TabList, Menu, MenuBar, Toolbar, ScrollArea, TableHeader, TabPanel, Window, Dialog
// and Document. Roles that hold children only incidentally — a Row and its cells, a MenuItem with a submenu, a
// ComboBox with its dropdown button — are not containers in this sense, because what they report to an assistive
// technology is their own value, not their contents.
func (e Enum) IsContainer() bool {
	switch e {
	case Group, List, Table, Tree, TabList, Menu, MenuBar, Toolbar, ScrollArea, TableHeader, TabPanel, Window, Dialog,
		Document:
		return true
	default:
		return false
	}
}

// IsRowLike returns true if this role is one entry in a collection that is addressed by position: Row, used by Table
// and Tree, and ListItem, used by List. These are the roles for which the position within the set and the total set
// size are worth reporting, and the ones a platform adapter exposes through its row-oriented interfaces.
func (e Enum) IsRowLike() bool {
	return e == Row || e == ListItem
}

// IsWindow returns true if this role is a top-level window: Window or Dialog. Only the root of a snapshot has one of
// these roles, so this is also the test for "is this node the root".
func (e Enum) IsWindow() bool {
	return e == Window || e == Dialog
}
