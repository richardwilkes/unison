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
// than just a single value: TextField, TextArea, SpinButton, ComboBox and Document. These are the only roles whose
// nodes ever carry text information — content, caret, selection and line boundaries — so asking this question is the
// cheapest way to decide whether that information is worth looking for. It is not a promise that the information is
// there: a Document never carries it, since its content is described by the nodes beneath it, and neither does a
// Protected field, so accessibility.Node.Text still has to be checked for nil. Label and Heading are deliberately
// excluded: their text is static and is reported as the node's name instead.
func (e Enum) IsText() bool {
	switch e {
	case TextField, TextArea, SpinButton, ComboBox, Document:
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
