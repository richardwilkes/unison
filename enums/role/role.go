// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package role

// IsText returns true if this role presents a body of text that assistive technologies may navigate through rather than
// just a single value: the editable controls TextField, TextArea, SpinButton and ComboBox, the pieces a document is
// read as — Document itself, Paragraph, Heading, Code, Cell and ColumnHeader — and Label, which is the static text the
// rest of a window is written in. These are the only roles whose nodes ever carry text information — content, caret,
// selection, line boundaries, styled runs and the spans other nodes occupy — so asking this question is the cheapest
// way to decide whether that information is worth looking for.
//
// Label is in the set because static text is read the same way everywhere a screen reader reads text at all: by line,
// by word and by character, with a review cursor that has to know where each character was drawn. A label that only
// had a name could be announced but never explored, so it carries its text and the one line it was drawn on as surely
// as a paragraph of a document does.
//
// It is not a promise that the information is there: a Document carries its content as one composed stream in
// accessibility.Node.Document rather than as its own text, a block that was never measured carries nothing, a panel an
// application explicitly gave Label or Heading while it draws nothing has no text to carry, and a Protected field
// carries neither, so accessibility.Node.Text still has to be checked for nil. Heading is in the set because a heading
// carries both — the name an assistive technology announces it by, and the same words as text, so that a reading caret
// can move through them line by line like any other block. ListItem stays out: the words of an item live on the label
// within it, which carries them itself. Link stays out as well: a link is announced by its name and acted on, and
// reading its text a second time as a body of its own would say everything about it twice.
func (e Enum) IsText() bool {
	switch e {
	case TextField, TextArea, SpinButton, ComboBox, Document, Paragraph, Heading, Code, Cell, ColumnHeader, Label:
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
