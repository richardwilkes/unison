// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

import (
	"slices"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// newTestTree assembles a tree from nodes given in any order, filling in each node's Parent from the Children lists so
// that a test only has to state the hierarchy once.
func newTestTree(root, focus accessibility.NodeID, nodes ...*accessibility.Node) *accessibility.Tree {
	tree := &accessibility.Tree{
		Nodes:      make(map[accessibility.NodeID]*accessibility.Node, len(nodes)),
		Root:       root,
		Focus:      focus,
		Generation: 1,
	}
	for _, n := range nodes {
		tree.Nodes[n.ID] = n
	}
	for _, n := range nodes {
		for _, childID := range n.Children {
			if child := tree.Nodes[childID]; child != nil {
				child.Parent = n.ID
			}
		}
	}
	return tree
}

// sampleTree builds the hierarchy the navigation, hit-testing and content-view tests work over:
//
//	1 window                          (0,0 200x200)  focused
//	├─ 2 group   [ignored]            (0,0 200x100)
//	│  ├─ 3 group   [ignored]         (0,0 200x50)
//	│  │  ├─ 4 button                 (0,0 50x20)    focused, pressable
//	│  │  └─ 5 button                 (50,0 50x20)
//	│  └─ 6 label                     (0,50 100x20)   labels 4
//	└─ 7 group                        (0,100 200x100)
//	   ├─ 8 button                     (0,100 60x30)
//	   └─ 9 button                     (0,100 60x30)
//
// The two layers of ignored grouping mean the window's unignored children are 4, 5, 6 and 7, which is what makes this
// tree worth navigating. Nodes 8 and 9 deliberately occupy the same area, with 8 first so that it is the topmost.
func sampleTree() *accessibility.Tree {
	return newTestTree(1, 4,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Window", Focused: true, Bounds: geom.NewRect(0, 0, 200, 200),
			Children: []accessibility.NodeID{2, 7},
		},
		&accessibility.Node{
			ID: 2, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 200, 100),
			Children: []accessibility.NodeID{3, 6},
		},
		&accessibility.Node{
			ID: 3, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 200, 50),
			Children: []accessibility.NodeID{4, 5},
		},
		&accessibility.Node{
			ID: 4, Role: role.Button, Name: "One", Focusable: true, Focused: true,
			Bounds: geom.NewRect(0, 0, 50, 20), LabeledBy: []accessibility.NodeID{6},
			Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.Press),
		},
		&accessibility.Node{ID: 5, Role: role.Button, Name: "Two", Bounds: geom.NewRect(50, 0, 50, 20)},
		&accessibility.Node{ID: 6, Role: role.Label, Name: "One", Bounds: geom.NewRect(0, 50, 100, 20)},
		&accessibility.Node{
			ID: 7, Role: role.Group, Bounds: geom.NewRect(0, 100, 200, 100),
			Children: []accessibility.NodeID{8, 9},
		},
		&accessibility.Node{ID: 8, Role: role.Button, Name: "Three", Bounds: geom.NewRect(0, 100, 60, 30)},
		&accessibility.Node{ID: 9, Role: role.Button, Name: "Four", Bounds: geom.NewRect(0, 100, 60, 30)},
	)
}

// TestControlType verifies the role-to-control-type table, and that it covers every role: a role added to the enum
// without a decision here would silently become Custom, which is the wrong answer for anything UI Automation has a
// control type for.
//
// Every node here is the tree's root, which is what a window-like role needs to be to report the Window control type;
// TestControlTypeNested covers the nested case.
func TestControlType(t *testing.T) {
	c := check.New(t)
	expected := map[role.Enum]ControlTypeID{
		role.Auto:               CustomControlTypeId,
		role.None:               CustomControlTypeId,
		role.Window:             WindowControlTypeId,
		role.Dialog:             WindowControlTypeId,
		role.Group:              GroupControlTypeId,
		role.Button:             ButtonControlTypeId,
		role.ToggleButton:       ButtonControlTypeId,
		role.DisclosureTriangle: ButtonControlTypeId,
		role.CheckBox:           CheckBoxControlTypeId,
		role.RadioButton:        RadioButtonControlTypeId,
		role.Link:               HyperlinkControlTypeId,
		role.Label:              TextControlTypeId,
		role.Heading:            TextControlTypeId,
		role.Paragraph:          TextControlTypeId,
		role.BlockQuote:         GroupControlTypeId,
		role.Code:               TextControlTypeId,
		role.TextField:          EditControlTypeId,
		role.TextArea:           EditControlTypeId,
		role.SpinButton:         SpinnerControlTypeId,
		role.ComboBox:           ComboBoxControlTypeId,
		role.PopupButton:        ComboBoxControlTypeId,
		role.Slider:             SliderControlTypeId,
		role.ProgressBar:        ProgressBarControlTypeId,
		role.ScrollBar:          ScrollBarControlTypeId,
		role.ScrollArea:         PaneControlTypeId,
		role.Separator:          SeparatorControlTypeId,
		role.List:               ListControlTypeId,
		role.ListItem:           ListItemControlTypeId,
		role.Table:              DataGridControlTypeId,
		role.Tree:               DataGridControlTypeId,
		role.Row:                DataItemControlTypeId,
		role.Cell:               DataItemControlTypeId,
		role.ColumnHeader:       HeaderItemControlTypeId,
		role.TableHeader:        HeaderControlTypeId,
		role.TabList:            TabControlTypeId,
		role.Tab:                TabItemControlTypeId,
		role.TabPanel:           PaneControlTypeId,
		role.MenuBar:            MenuBarControlTypeId,
		role.Menu:               MenuControlTypeId,
		role.MenuItem:           MenuItemControlTypeId,
		role.Image:              ImageControlTypeId,
		role.ColorWell:          ButtonControlTypeId,
		role.Tooltip:            ToolTipControlTypeId,
		// A document with no composed stream is a group: the document control type requires the Text pattern, and a
		// document with nothing to read it from would have a client ask for the one pattern every document has and be
		// handed NULL. TestControlTypeDocument covers the other half, where the stream is there.
		role.Document: GroupControlTypeId,
		role.Toolbar:  ToolBarControlTypeId,
		role.Unknown:  CustomControlTypeId,
	}
	c.Equal(len(role.All), len(expected))
	for _, r := range role.All {
		want, ok := expected[r]
		c.True(ok, "no control type expected for role %s", r.Key())
		node := &accessibility.Node{ID: 1, Role: r}
		c.Equal(want, ControlType(newTestTree(1, 0, node), node), "role %s", r.Key())
	}
	c.Equal(CustomControlTypeId, ControlType(nil, nil))
}

// TestControlTypeNested verifies that only the fragment root reports the Window control type. The Window control
// type lists IWindowProvider as a required pattern and the provider hands that interface out for the root alone, so a
// dialog-shaped panel inside a window must report Pane instead of advertising a pattern it refuses.
func TestControlTypeNested(t *testing.T) {
	c := check.New(t)
	tree := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2, 3}},
		&accessibility.Node{ID: 2, Role: role.Dialog},
		&accessibility.Node{ID: 3, Role: role.Window},
	)
	c.Equal(WindowControlTypeId, ControlType(tree, tree.Node(1)))
	c.Equal(PaneControlTypeId, ControlType(tree, tree.Node(2)))
	c.Equal(PaneControlTypeId, ControlType(tree, tree.Node(3)))

	// With no tree to ask, nothing can be said to be the root, so the cautious answer is the nested one.
	c.Equal(PaneControlTypeId, ControlType(nil, tree.Node(1)))
}

// TestControlTypeDocument verifies that a Document reports the document control type exactly when it carries the
// composed stream that makes it readable. The control type lists ITextProvider as a required pattern, so the two have
// to agree: a document element a client could not read text from would be a broken element, and a Document with a
// stream that reported itself a group would never put Narrator into the reading mode this whole pattern exists for.
func TestControlTypeDocument(t *testing.T) {
	c := check.New(t)
	tree := textFixtureTree()
	document := tree.Node(textDocumentID)
	c.Equal(DocumentControlTypeId, ControlType(tree, document))
	c.True(Patterns(document).Has(PatternText | PatternText2))

	document.Document = nil
	c.Equal(GroupControlTypeId, ControlType(tree, document))
	c.Equal(PatternSet(0), Patterns(document)&(PatternText|PatternText2))
}

// TestPatterns verifies the role-to-patterns table, including the handful of roles whose patterns depend on state.
// Every node here is given no actions at all, so that the role's own answer is what is being checked;
// TestPatternsScrollItem covers the one pattern that comes from the action set instead.
func TestPatterns(t *testing.T) {
	c := check.New(t)
	for i, one := range []struct {
		node     *accessibility.Node
		patterns PatternSet
	}{
		{node: &accessibility.Node{Role: role.Window}, patterns: PatternWindow},
		{node: &accessibility.Node{Role: role.Dialog}, patterns: PatternWindow},
		{node: &accessibility.Node{Role: role.Group}},
		{node: &accessibility.Node{Role: role.TabPanel}},
		{node: &accessibility.Node{Role: role.ScrollArea}},
		{node: &accessibility.Node{Role: role.TableHeader}},
		// Static text hands out the Text pattern precisely when it carries text, and never a Value pattern: the same
		// words are already the element's name, and a client reading both would speak them twice. A label showing only
		// an image carries none and hands out nothing.
		{node: &accessibility.Node{Role: role.Label}},
		{
			node:     &accessibility.Node{Role: role.Label, Text: &accessibility.TextInfo{Text: "Some text"}},
			patterns: PatternText | PatternText2,
		},
		{node: &accessibility.Node{Role: role.Heading}},
		{
			node:     &accessibility.Node{Role: role.Heading, Level: 2, Text: &accessibility.TextInfo{Text: "Title"}},
			patterns: PatternText | PatternText2,
		},
		// The blocks a document is composed of answer the same way on their own. Inside a document they lose the
		// pattern again, since the document owns their words; that is ProvidedPatterns' half of the answer, and
		// TestProvidedPatternsTextChild covers it. BlockQuote is outside role.IsText and so carries no text at all.
		{node: &accessibility.Node{Role: role.Paragraph}},
		{node: &accessibility.Node{Role: role.Code}},
		{node: &accessibility.Node{Role: role.BlockQuote}},
		{
			node:     &accessibility.Node{Role: role.Paragraph, Text: &accessibility.TextInfo{Text: "Hello"}},
			patterns: PatternText | PatternText2,
		},
		{
			node:     &accessibility.Node{Role: role.Code, Text: &accessibility.TextInfo{Text: "x = 1"}},
			patterns: PatternText | PatternText2,
		},
		{node: &accessibility.Node{Role: role.BlockQuote, Text: &accessibility.TextInfo{Text: "Quoted"}}},
		{node: &accessibility.Node{Role: role.Image}},
		{node: &accessibility.Node{Role: role.Separator}},
		{node: &accessibility.Node{Role: role.MenuBar}},
		{node: &accessibility.Node{Role: role.Tooltip}},
		{node: &accessibility.Node{Role: role.Toolbar}},
		{node: &accessibility.Node{Role: role.Button}, patterns: PatternInvoke},
		{node: &accessibility.Node{Role: role.Link}, patterns: PatternInvoke},
		// A link that knows where it leads reports the target through a read-only Value, which is where every client
		// looks for a hyperlink's destination: UI Automation has no property of its own for one.
		{
			node:     &accessibility.Node{Role: role.Link, URL: "https://example.com"},
			patterns: PatternInvoke | PatternValue,
		},
		{node: &accessibility.Node{Role: role.ColumnHeader}, patterns: PatternInvoke},
		// A column header drawn as plain text carries that text and reports it, on top of the Invoke that sorts the
		// column. A custom header draws its own content and carries none.
		{
			node:     &accessibility.Node{Role: role.ColumnHeader, Text: &accessibility.TextInfo{Text: "Name"}},
			patterns: PatternInvoke | PatternText | PatternText2,
		},
		{node: &accessibility.Node{Role: role.ColorWell}, patterns: PatternInvoke | PatternValue},
		{node: &accessibility.Node{Role: role.ToggleButton}, patterns: PatternToggle},
		{node: &accessibility.Node{Role: role.CheckBox}, patterns: PatternToggle},
		// A disclosure triangle expands whatever it is attached to, and the state it reports is reachable only through
		// the ExpandCollapse pattern, so the pattern follows the flag exactly as a row's and a menu item's do.
		{node: &accessibility.Node{Role: role.DisclosureTriangle}, patterns: PatternToggle},
		{
			node:     &accessibility.Node{Role: role.DisclosureTriangle, Expandable: true},
			patterns: PatternToggle | PatternExpandCollapse,
		},
		{node: &accessibility.Node{Role: role.RadioButton}, patterns: PatternSelectionItem},
		// A field reports its content twice over: through Value, which is how a client reads the whole of it, and
		// through Text, which is how it reads it by line, word and character and follows the caret. A Protected field
		// carries no text at all, so it keeps Value alone.
		{node: &accessibility.Node{Role: role.TextField}, patterns: PatternValue},
		{node: &accessibility.Node{Role: role.TextArea}, patterns: PatternValue},
		{
			node:     &accessibility.Node{Role: role.TextField, Text: &accessibility.TextInfo{Text: "Hello"}},
			patterns: PatternValue | PatternText | PatternText2,
		},
		{
			node:     &accessibility.Node{Role: role.TextArea, Text: &accessibility.TextInfo{Text: "Hello"}},
			patterns: PatternValue | PatternText | PatternText2,
		},
		{node: &accessibility.Node{Role: role.TextField, Protected: true}, patterns: PatternValue},
		// A node marked Protected after its text was filled in — which only an AccessibilityInfo.Callback can
		// produce — keeps Value alone as well: the pattern that would read the content out by line, word and
		// character is refused on the flag rather than on the text being absent.
		{
			node: &accessibility.Node{
				Role: role.TextField, Protected: true, Text: &accessibility.TextInfo{Text: "hunter2"},
			},
			patterns: PatternValue,
		},
		// A document with no composed stream carries nothing at all: no value, since the pattern would answer a client
		// with an empty string as the whole content of the document, and no Text pattern, since there is no text to
		// read through it. TestControlTypeDocument covers the other half.
		{node: &accessibility.Node{Role: role.Document}},
		{
			node:     &accessibility.Node{Role: role.Document, Document: &accessibility.DocumentInfo{}},
			patterns: PatternText | PatternText2,
		},
		// A spin button has a range only once it has a number, exactly as a progress bar does: an obscured numeric
		// field fills in none of them, and the pattern would report a PIN field as zero.
		{node: &accessibility.Node{Role: role.SpinButton}, patterns: PatternValue},
		{
			node:     &accessibility.Node{Role: role.SpinButton, HasNumber: true},
			patterns: PatternValue | PatternRangeValue,
		},
		{
			node:     &accessibility.Node{Role: role.SpinButton, Text: &accessibility.TextInfo{Text: "3"}},
			patterns: PatternValue | PatternText | PatternText2,
		},
		{
			node: &accessibility.Node{
				Role: role.SpinButton, HasNumber: true, Text: &accessibility.TextInfo{Text: "3"},
			},
			patterns: PatternValue | PatternRangeValue | PatternText | PatternText2,
		},
		{node: &accessibility.Node{Role: role.ComboBox}, patterns: PatternValue | PatternExpandCollapse},
		{
			node:     &accessibility.Node{Role: role.ComboBox, Text: &accessibility.TextInfo{Text: "Red"}},
			patterns: PatternValue | PatternExpandCollapse | PatternText | PatternText2,
		},
		{node: &accessibility.Node{Role: role.PopupButton}, patterns: PatternValue | PatternExpandCollapse},
		// A slider and a scroll bar have a range only once they have a number, for the reason a spin button and a
		// progress bar do: the roles are public API, and RangeValue answering zero for the value, the bounds and the
		// increments reads as "0 percent".
		{node: &accessibility.Node{Role: role.Slider}},
		{node: &accessibility.Node{Role: role.Slider, HasNumber: true}, patterns: PatternRangeValue},
		{node: &accessibility.Node{Role: role.ScrollBar}},
		{node: &accessibility.Node{Role: role.ScrollBar, HasNumber: true}, patterns: PatternRangeValue},
		{node: &accessibility.Node{Role: role.ProgressBar}},
		{node: &accessibility.Node{Role: role.ProgressBar, HasNumber: true}, patterns: PatternRangeValue},
		{node: &accessibility.Node{Role: role.List}, patterns: PatternSelection},
		{node: &accessibility.Node{Role: role.TabList}, patterns: PatternSelection},
		{node: &accessibility.Node{Role: role.ListItem}, patterns: PatternSelectionItem},
		// A list item within a document carries the text of its paragraph, which grants it nothing either: what it
		// hands out is what it is, a thing that can be selected.
		{
			node:     &accessibility.Node{Role: role.ListItem, Text: &accessibility.TextInfo{Text: "one"}},
			patterns: PatternSelectionItem,
		},
		// A row holding one widget with a state reports that state as its own value, which a client can only read
		// through the Value pattern — the same bargain the cell below makes.
		{
			node:     &accessibility.Node{Role: role.ListItem, Value: "checked"},
			patterns: PatternSelectionItem | PatternValue,
		},
		{
			node: &accessibility.Node{
				Role: role.ListItem, Value: "checked", Text: &accessibility.TextInfo{Text: "one"},
			},
			patterns: PatternSelectionItem | PatternValue,
		},
		{node: &accessibility.Node{Role: role.Tab}, patterns: PatternSelectionItem},
		{node: &accessibility.Node{Role: role.Table}, patterns: PatternGrid | PatternTable | PatternSelection},
		{node: &accessibility.Node{Role: role.Tree}, patterns: PatternGrid | PatternTable | PatternSelection},
		{node: &accessibility.Node{Role: role.Row}, patterns: PatternSelectionItem},
		{
			node:     &accessibility.Node{Role: role.Row, Expandable: true},
			patterns: PatternSelectionItem | PatternExpandCollapse,
		},
		{node: &accessibility.Node{Role: role.Cell}, patterns: PatternGridItem | PatternTableItem},
		// A cell holding one widget reports that widget's state as its own value, which a client can only read through
		// the Value pattern.
		{
			node:     &accessibility.Node{Role: role.Cell, Value: "checked"},
			patterns: PatternGridItem | PatternTableItem | PatternValue,
		},
		// A cell carrying text of its own reports it instead, so that a review cursor can read it by word and
		// character rather than only hear it named. The toolkit's own cells are read through the Label within them
		// and never carry text, so this is the shape an application produces by filling a cell's text in from an
		// AccessibilityInfo.Callback.
		{
			node:     &accessibility.Node{Role: role.Cell, Text: &accessibility.TextInfo{Text: "Note"}},
			patterns: PatternGridItem | PatternTableItem | PatternText | PatternText2,
		},
		{node: &accessibility.Node{Role: role.Menu}},
		{node: &accessibility.Node{Role: role.MenuItem}, patterns: PatternInvoke},
		{node: &accessibility.Node{Role: role.MenuItem, HasCheck: true}, patterns: PatternInvoke | PatternToggle},
		{
			node:     &accessibility.Node{Role: role.MenuItem, Expandable: true},
			patterns: PatternInvoke | PatternExpandCollapse,
		},
	} {
		c.Equal(one.patterns, Patterns(one.node), "case %d (%s)", i, one.node.Role.Key())
	}
	c.Equal(PatternSet(0), Patterns(nil))
}

// TestPatternsScrollItem verifies that the ScrollItem pattern follows the ScrollIntoView action rather than the
// role. Every node in a snapshot carries that action — a disabled one keeps it when it keeps nothing else — and the
// pattern's only method does nothing but dispatch it, so anything a client may want to talk about can be brought into
// view: a cell scrolled off to the side, a column header, a tab, a menu item.
func TestPatternsScrollItem(t *testing.T) {
	c := check.New(t)
	scrollable := accessibility.ActionSet(0).With(accessibility.ScrollIntoView)
	for _, r := range []role.Enum{
		role.Cell, role.ColumnHeader, role.Tab, role.MenuItem, role.Row, role.ListItem, role.Button, role.Group,
	} {
		with := Patterns(&accessibility.Node{Role: r, Actions: scrollable})
		c.True(with.Has(PatternScrollItem), "role %s", r.Key())
		c.False(Patterns(&accessibility.Node{Role: r}).Has(PatternScrollItem), "role %s without it", r.Key())
	}

	// The pattern is all a disabled row has left, which is what lets a screen reader scroll through a disabled table.
	c.Equal(PatternSelectionItem|PatternScrollItem,
		Patterns(&accessibility.Node{Role: role.Row, Disabled: true, Actions: scrollable}))
}

// TestProvidedPatternsTextChild verifies the one pattern that comes from where a node sits rather than from what it
// is: an element inside a document's stream hands out TextChild, which is how a client that has walked the element tree
// to a link or an image asks which part of the document it is.
//
// It is granted through ProvidedPatterns rather than Patterns, since the answer needs the tree, which is also
// what makes the decider report it appearing and vanishing.
func TestProvidedPatternsTextChild(t *testing.T) {
	c := check.New(t)
	tree := textFixtureTree()
	for _, id := range []accessibility.NodeID{
		textHeadingID, textParagraphID, textLinkID, textImageID, textCodeID, textListItemID,
		textCellAID, textLabelID,
	} {
		n := tree.Node(id)
		c.True(ProvidesPattern(tree, n, PatternTextChild), "node %d", id)
		c.False(Patterns(n).Has(PatternTextChild), "node %d needs the tree to be told", id)
	}

	// The document is the container rather than a child of one, and nothing outside it is either.
	c.False(ProvidesPattern(tree, tree.Node(textDocumentID), PatternTextChild))
	c.False(ProvidesPattern(tree, tree.Node(textButtonID), PatternTextChild))
	c.False(ProvidesPattern(tree, tree.Node(textWindowID), PatternTextChild))

	// A Document that loses its stream takes the pattern away from everything inside it, since there is no longer any
	// text for an element to occupy.
	plain := textFixtureTree()
	plain.Node(textDocumentID).Document = nil
	c.False(ProvidesPattern(plain, plain.Node(textLinkID), PatternTextChild))

	// Gaining TextChild costs the element the Text pattern. The blocks of this document carry text of their own, so
	// Patterns grants them both — a paragraph outside a document is read through its own Text pattern — but inside
	// one the document owns the words, and two providers over the same text with two sets of offsets is not an
	// answer a client can reconcile.
	for _, id := range []accessibility.NodeID{textParagraphID, textCodeID, textCellAID, textCellBID} {
		n := tree.Node(id)
		c.True(Patterns(n).Has(PatternText|PatternText2), "node %d carries its own text", id)
		c.False(ProvidesPattern(tree, n, PatternText), "node %d is claimed by the document", id)
		c.False(ProvidesPattern(tree, n, PatternText2), "node %d is claimed by the document", id)
	}

	// Once the stream is gone nothing claims them, and the blocks answer for their own text again.
	c.True(ProvidesPattern(plain, plain.Node(textParagraphID), PatternText|PatternText2))
}

// TestProvidedPatternsTextOwner verifies the other side of that exchange: an element carrying text that no document
// has claimed hands out the Text pattern itself, wherever in the tree it sits. This is what lets Narrator's scan mode
// read a label, a heading, a plain cell or a column header by line, word and character, and what gives NVDA the caret
// it follows through a field.
func TestProvidedPatternsTextOwner(t *testing.T) {
	c := check.New(t)
	tree := fieldFixtureTree()
	for _, id := range []accessibility.NodeID{fieldID, fieldHeadingID, fieldCellLabelID} {
		n := tree.Node(id)
		c.True(ProvidesPattern(tree, n, PatternText|PatternText2), "node %d", id)
		c.False(ProvidesPattern(tree, n, PatternTextChild), "node %d is in no document", id)
	}

	// Nothing that carries no text hands the pattern out, whatever its role.
	for _, id := range []accessibility.NodeID{
		fieldWindowID, fieldScrollID, fieldTableID, fieldRowID, fieldCellID, fieldButtonID,
	} {
		c.False(ProvidesPattern(tree, tree.Node(id), PatternText), "node %d", id)
	}

	// A Protected field publishes no text, so it keeps its Value pattern and loses the Text ones, which is the second
	// line of the defense the flag is: nothing a client can read the content through is left.
	protected := fieldFixtureTree()
	protected.Node(fieldID).Protected = true
	protected.Node(fieldID).Text = nil
	c.False(ProvidesPattern(protected, protected.Node(fieldID), PatternText))
	c.True(ProvidesPattern(protected, protected.Node(fieldID), PatternValue))

	// The flag is obeyed even when the text was filled in before it was set, which is what an
	// AccessibilityInfo.Callback marking a node Protected produces: the pattern is withheld rather than left to read
	// a password out by line, word and character, exactly as ValueString withholds the value.
	filled := fieldFixtureTree()
	filled.Node(fieldID).Protected = true
	c.NotNil(filled.Node(fieldID).Text)
	c.False(ProvidesPattern(filled, filled.Node(fieldID), PatternText|PatternText2))
	c.Equal("", ValueString(filled.Node(fieldID)))
}

// TestNameString verifies the two places a node's name is not Node.Name: the pieces that are nothing but the text
// drawn in them — a paragraph, a code block, a cell and a plain column header — whose name is their own content when
// the widget gave them none, and the list items and cells that describe what they hold as elements of their own,
// whose name is built from those elements. Narrator's item navigation walks the control view and speaks each
// element's name, and NVDA and JAWS speak the name and the control type and nothing else, so a paragraph with no name
// is announced as a bare "text" and a list row with none as a bare "list item".
//
// The first group needs no tree, so it is asked with none, which is also what proves that a missing snapshot falls
// back to the node's own name rather than panicking.
func TestNameString(t *testing.T) {
	c := check.New(t)
	for i, one := range []struct {
		node     *accessibility.Node
		expected string
	}{
		{
			node:     &accessibility.Node{Role: role.Paragraph, Text: &accessibility.TextInfo{Text: "Body text"}},
			expected: "Body text",
		},
		{
			node:     &accessibility.Node{Role: role.Code, Text: &accessibility.TextInfo{Text: "x = 1"}},
			expected: "x = 1",
		},
		{
			node:     &accessibility.Node{Role: role.Cell, Text: &accessibility.TextInfo{Text: "42"}},
			expected: "42",
		},
		// A column header whose name an application cleared in its Callback — the only shape that reaches this arm,
		// since TableHeader names every column it fills text in for; see namedByItsText — is read as the title it
		// draws rather than as a bare "header".
		{
			node:     &accessibility.Node{Role: role.ColumnHeader, Text: &accessibility.TextInfo{Text: "Name"}},
			expected: "Name",
		},
		{
			node: &accessibility.Node{
				Role: role.ColumnHeader, Name: "Full name", Text: &accessibility.TextInfo{Text: "Name"},
			},
			expected: "Full name",
		},
		{node: &accessibility.Node{Role: role.ColumnHeader}},
		{
			node:     &accessibility.Node{Role: role.Paragraph, Name: "Summary", Text: &accessibility.TextInfo{Text: "Body"}},
			expected: "Summary",
		},
		// A heading folds its fragments into a name of its own, and a label is named by the widget, so neither is
		// renamed by what it draws.
		{
			node:     &accessibility.Node{Role: role.Heading, Name: "Title", Text: &accessibility.TextInfo{Text: "Title"}},
			expected: "Title",
		},
		{node: &accessibility.Node{Role: role.Label, Text: &accessibility.TextInfo{Text: "drawn"}}},
		{node: &accessibility.Node{Role: role.Paragraph}},
		{node: &accessibility.Node{Role: role.Paragraph, Text: &accessibility.TextInfo{}}},
		{node: &accessibility.Node{Role: role.Button, Name: "Close"}, expected: "Close"},
	} {
		c.Equal(one.expected, NameString(nil, one.node), "case %d (%s)", i, one.node.Role.Key())
	}
	c.Equal("", NameString(nil, nil))

	// A list row or a cell the widget left unnamed is read as what it holds. See contentNameTree for the shapes.
	tree := contentNameTree()
	for _, one := range []struct {
		comment  string
		id       accessibility.NodeID
		expected string
	}{
		{comment: "one label", id: 3, expected: "Alpha"},
		{comment: "a label and a check box", id: 5, expected: "Beta checked"},
		{comment: "an anonymous group wrapping a label", id: 9, expected: "Gamma"},
		{comment: "a name of its own wins over the content", id: 13, expected: "Explicit"},
		{comment: "nothing but an ignored child", id: 16, expected: ""},
		// The label names the field beside it, so it is out of the content view and the field reports its text as its
		// own name; saying both would have the row read as "Name Name".
		{comment: "a label naming the field beside it", id: 18, expected: "Name"},
		{comment: "a nested list item", id: 21, expected: "Inner"},
		{comment: "a cell holding a label", id: 27, expected: "Delta"},
		{comment: "a cell carrying text of its own", id: 29, expected: "42"},
	} {
		c.Equal(one.expected, NameString(tree, tree.Node(one.id)), "node %d (%s)", one.id, one.comment)
	}

	// Without a tree there is nothing to build a name from, so the node's own name is all there is to answer with.
	c.Equal("", NameString(nil, tree.Node(3)))
	c.Equal("Explicit", NameString(nil, tree.Node(13)))
}

// contentNameTree builds the rows and cells the content-naming tests read:
//
//	1 window
//	├─ 2 list
//	│  ├─ 3 list-item                                  "Alpha"
//	│  │  └─ 4 label "Alpha"
//	│  ├─ 5 list-item                                  "Beta checked"
//	│  │  ├─ 6 label "Beta"
//	│  │  └─ 7 check-box [value "checked"]
//	│  ├─ 9 list-item                                  "Gamma"
//	│  │  └─ 10 group
//	│  │     └─ 11 label "Gamma"
//	│  ├─ 13 list-item "Explicit"                      "Explicit"
//	│  │  └─ 14 label "Inner"
//	│  ├─ 16 list-item                                 ""
//	│  │  └─ 17 label "Hidden" [ignored]
//	│  ├─ 18 list-item                                 "Name"
//	│  │  ├─ 19 label "Name"
//	│  │  └─ 20 text-field "Name" [labeled by 19]
//	│  └─ 21 list-item                                 "Inner"
//	│     └─ 22 list
//	│        └─ 23 list-item                           "Inner"
//	│           └─ 24 label "Inner"
//	└─ 25 table
//	   └─ 26 row
//	      ├─ 27 cell                                   "Delta"
//	      │  └─ 28 label "Delta"
//	      └─ 29 cell [text "42"]                       "42"
//
// The ignored child of 16 is spliced away by UnignoredChildren, which leaves that row with nothing to be named by;
// the group above 11 is anonymous, so the walk goes through it to the label inside.
func contentNameTree() *accessibility.Tree {
	return newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2, 25}},
		&accessibility.Node{
			ID: 2, Role: role.List, Children: []accessibility.NodeID{3, 5, 9, 13, 16, 18, 21},
		},
		&accessibility.Node{ID: 3, Role: role.ListItem, Children: []accessibility.NodeID{4}},
		&accessibility.Node{ID: 4, Role: role.Label, Name: "Alpha", Text: &accessibility.TextInfo{Text: "Alpha"}},
		&accessibility.Node{ID: 5, Role: role.ListItem, Children: []accessibility.NodeID{6, 7}},
		&accessibility.Node{ID: 6, Role: role.Label, Name: "Beta", Text: &accessibility.TextInfo{Text: "Beta"}},
		&accessibility.Node{ID: 7, Role: role.CheckBox, HasCheck: true, Checked: checkenum.On, Value: "checked"},
		&accessibility.Node{ID: 9, Role: role.ListItem, Children: []accessibility.NodeID{10}},
		&accessibility.Node{ID: 10, Role: role.Group, Children: []accessibility.NodeID{11}},
		&accessibility.Node{ID: 11, Role: role.Label, Name: "Gamma", Text: &accessibility.TextInfo{Text: "Gamma"}},
		&accessibility.Node{ID: 13, Role: role.ListItem, Name: "Explicit", Children: []accessibility.NodeID{14}},
		&accessibility.Node{ID: 14, Role: role.Label, Name: "Inner", Text: &accessibility.TextInfo{Text: "Inner"}},
		&accessibility.Node{ID: 16, Role: role.ListItem, Children: []accessibility.NodeID{17}},
		&accessibility.Node{
			ID: 17, Role: role.Label, Name: "Hidden", Ignored: true, Text: &accessibility.TextInfo{Text: "Hidden"},
		},
		&accessibility.Node{ID: 18, Role: role.ListItem, Children: []accessibility.NodeID{19, 20}},
		&accessibility.Node{ID: 19, Role: role.Label, Name: "Name", Text: &accessibility.TextInfo{Text: "Name"}},
		&accessibility.Node{
			ID: 20, Role: role.TextField, Name: "Name", LabeledBy: []accessibility.NodeID{19},
			Text: &accessibility.TextInfo{Text: "Fred"},
		},
		&accessibility.Node{ID: 21, Role: role.ListItem, Children: []accessibility.NodeID{22}},
		&accessibility.Node{ID: 22, Role: role.List, Children: []accessibility.NodeID{23}},
		&accessibility.Node{ID: 23, Role: role.ListItem, Children: []accessibility.NodeID{24}},
		&accessibility.Node{ID: 24, Role: role.Label, Name: "Inner", Text: &accessibility.TextInfo{Text: "Inner"}},
		&accessibility.Node{ID: 25, Role: role.Table, RowCount: 1, Children: []accessibility.NodeID{26}},
		&accessibility.Node{ID: 26, Role: role.Row, RowIndex: 0, Children: []accessibility.NodeID{27, 29}},
		&accessibility.Node{ID: 27, Role: role.Cell, RowIndex: 0, Children: []accessibility.NodeID{28}},
		&accessibility.Node{ID: 28, Role: role.Label, Name: "Delta", Text: &accessibility.TextInfo{Text: "Delta"}},
		&accessibility.Node{
			ID: 29, Role: role.Cell, RowIndex: 0, ColumnIndex: 1, Text: &accessibility.TextInfo{Text: "42"},
		},
	)
}

// TestPatternSet verifies the bookkeeping around the pattern bitset: that every pattern a provider implements can be
// looked up by the identifier a client asks for it by, that a pattern this package does not implement looks up to
// nothing, and that Has is an all-of test rather than an any-of one.
func TestPatternSet(t *testing.T) {
	c := check.New(t)
	for _, one := range []struct {
		id      PatternID
		pattern PatternSet
	}{
		{id: InvokePatternId, pattern: PatternInvoke},
		{id: TogglePatternId, pattern: PatternToggle},
		{id: ValuePatternId, pattern: PatternValue},
		{id: RangeValuePatternId, pattern: PatternRangeValue},
		{id: SelectionPatternId, pattern: PatternSelection},
		{id: SelectionItemPatternId, pattern: PatternSelectionItem},
		{id: ExpandCollapsePatternId, pattern: PatternExpandCollapse},
		{id: ScrollItemPatternId, pattern: PatternScrollItem},
		{id: GridPatternId, pattern: PatternGrid},
		{id: GridItemPatternId, pattern: PatternGridItem},
		{id: TablePatternId, pattern: PatternTable},
		{id: TableItemPatternId, pattern: PatternTableItem},
		{id: WindowPatternId, pattern: PatternWindow},
		{id: TextPatternId, pattern: PatternText},
		{id: TextPattern2Id, pattern: PatternText2},
		{id: TextChildPatternId, pattern: PatternTextChild},
	} {
		c.Equal(one.pattern, PatternSetForID(one.id))
	}
	c.Equal(PatternSet(0), PatternSetForID(ScrollPatternId))
	c.Equal(PatternSet(0), PatternSetForID(0))

	both := PatternValue | PatternRangeValue
	c.True(both.Has(PatternValue))
	c.True(both.Has(both))
	c.False(both.Has(PatternValue | PatternToggle))
	c.Equal("value,range-value", both.String())
	c.Equal("", PatternSet(0).String())
}

// TestIdentifierValues pins every hand-written number in constants.go: the identifiers UI Automation defines,
// the provider options, the VARIANT type tags and VARIANT_BOOL values from wtypes.h, and the HRESULTs a provider
// refuses with. Nothing else does: the mapping tests compare one symbol against another, so a transposed value —
// DataGrid's 50028 written where DataItem's 50029 belongs, or one property id given another's number — would pass the
// whole suite while making a screen reader describe elements as the wrong kind of thing, or read a property no client
// asked for.
//
// A client resolves none of these by name, so the numbers are the entire interface. They are the values in the Windows
// SDK's uiautomationcoreapi.h, uiautomationcore.idl and wtypes.h, which is where a reader checks them.
func TestIdentifierValues(t *testing.T) {
	c := check.New(t)

	// Control types, the value of ControlTypePropertyId.
	c.Equal(ControlTypeID(50000), ButtonControlTypeId)
	c.Equal(ControlTypeID(50002), CheckBoxControlTypeId)
	c.Equal(ControlTypeID(50003), ComboBoxControlTypeId)
	c.Equal(ControlTypeID(50004), EditControlTypeId)
	c.Equal(ControlTypeID(50005), HyperlinkControlTypeId)
	c.Equal(ControlTypeID(50006), ImageControlTypeId)
	c.Equal(ControlTypeID(50007), ListItemControlTypeId)
	c.Equal(ControlTypeID(50008), ListControlTypeId)
	c.Equal(ControlTypeID(50009), MenuControlTypeId)
	c.Equal(ControlTypeID(50010), MenuBarControlTypeId)
	c.Equal(ControlTypeID(50011), MenuItemControlTypeId)
	c.Equal(ControlTypeID(50012), ProgressBarControlTypeId)
	c.Equal(ControlTypeID(50013), RadioButtonControlTypeId)
	c.Equal(ControlTypeID(50014), ScrollBarControlTypeId)
	c.Equal(ControlTypeID(50015), SliderControlTypeId)
	c.Equal(ControlTypeID(50016), SpinnerControlTypeId)
	c.Equal(ControlTypeID(50018), TabControlTypeId)
	c.Equal(ControlTypeID(50019), TabItemControlTypeId)
	c.Equal(ControlTypeID(50020), TextControlTypeId)
	c.Equal(ControlTypeID(50021), ToolBarControlTypeId)
	c.Equal(ControlTypeID(50022), ToolTipControlTypeId)
	c.Equal(ControlTypeID(50023), TreeControlTypeId)
	c.Equal(ControlTypeID(50024), TreeItemControlTypeId)
	c.Equal(ControlTypeID(50025), CustomControlTypeId)
	c.Equal(ControlTypeID(50026), GroupControlTypeId)
	c.Equal(ControlTypeID(50028), DataGridControlTypeId)
	c.Equal(ControlTypeID(50029), DataItemControlTypeId)
	c.Equal(ControlTypeID(50030), DocumentControlTypeId)
	c.Equal(ControlTypeID(50032), WindowControlTypeId)
	c.Equal(ControlTypeID(50033), PaneControlTypeId)
	c.Equal(ControlTypeID(50034), HeaderControlTypeId)
	c.Equal(ControlTypeID(50035), HeaderItemControlTypeId)
	c.Equal(ControlTypeID(50036), TableControlTypeId)
	c.Equal(ControlTypeID(50038), SeparatorControlTypeId)

	// Control patterns, which a client asks for by identifier through GetPatternProvider.
	c.Equal(PatternID(10000), InvokePatternId)
	c.Equal(PatternID(10001), SelectionPatternId)
	c.Equal(PatternID(10002), ValuePatternId)
	c.Equal(PatternID(10003), RangeValuePatternId)
	c.Equal(PatternID(10004), ScrollPatternId)
	c.Equal(PatternID(10005), ExpandCollapsePatternId)
	c.Equal(PatternID(10006), GridPatternId)
	c.Equal(PatternID(10007), GridItemPatternId)
	c.Equal(PatternID(10009), WindowPatternId)
	c.Equal(PatternID(10010), SelectionItemPatternId)
	c.Equal(PatternID(10012), TablePatternId)
	c.Equal(PatternID(10013), TableItemPatternId)
	c.Equal(PatternID(10014), TextPatternId)
	c.Equal(PatternID(10015), TogglePatternId)
	c.Equal(PatternID(10017), ScrollItemPatternId)
	c.Equal(PatternID(10024), TextPattern2Id)
	c.Equal(PatternID(10029), TextChildPatternId)

	// Events, every one of which is raised by identifier.
	c.Equal(EventID(20000), ToolTipOpenedEventId)
	c.Equal(EventID(20001), ToolTipClosedEventId)
	c.Equal(EventID(20002), StructureChangedEventId)
	c.Equal(EventID(20003), MenuOpenedEventId)
	c.Equal(EventID(20004), AutomationPropertyChangedEventId)
	c.Equal(EventID(20005), AutomationFocusChangedEventId)
	c.Equal(EventID(20007), MenuClosedEventId)
	c.Equal(EventID(20009), Invoke_InvokedEventId)
	c.Equal(EventID(20010), SelectionItem_ElementAddedToSelectionEventId)
	c.Equal(EventID(20011), SelectionItem_ElementRemovedFromSelectionEventId)
	c.Equal(EventID(20012), SelectionItem_ElementSelectedEventId)
	c.Equal(EventID(20014), Text_TextSelectionChangedEventId)
	c.Equal(EventID(20015), Text_TextChangedEventId)
	c.Equal(EventID(20016), Window_WindowOpenedEventId)
	c.Equal(EventID(20017), Window_WindowClosedEventId)
	c.Equal(EventID(20024), LiveRegionChangedEventId)
	c.Equal(EventID(20035), NotificationEventId)

	// Properties, both the element-wide ones and the ones belonging to a control pattern.
	c.Equal(PropertyID(30000), RuntimeIdPropertyId)
	c.Equal(PropertyID(30001), BoundingRectanglePropertyId)
	c.Equal(PropertyID(30002), ProcessIdPropertyId)
	c.Equal(PropertyID(30003), ControlTypePropertyId)
	c.Equal(PropertyID(30004), LocalizedControlTypePropertyId)
	c.Equal(PropertyID(30005), NamePropertyId)
	c.Equal(PropertyID(30006), AcceleratorKeyPropertyId)
	c.Equal(PropertyID(30007), AccessKeyPropertyId)
	c.Equal(PropertyID(30008), HasKeyboardFocusPropertyId)
	c.Equal(PropertyID(30009), IsKeyboardFocusablePropertyId)
	c.Equal(PropertyID(30010), IsEnabledPropertyId)
	c.Equal(PropertyID(30011), AutomationIdPropertyId)
	c.Equal(PropertyID(30012), ClassNamePropertyId)
	c.Equal(PropertyID(30013), HelpTextPropertyId)
	c.Equal(PropertyID(30016), IsControlElementPropertyId)
	c.Equal(PropertyID(30017), IsContentElementPropertyId)
	c.Equal(PropertyID(30018), LabeledByPropertyId)
	c.Equal(PropertyID(30019), IsPasswordPropertyId)
	c.Equal(PropertyID(30020), NativeWindowHandlePropertyId)
	c.Equal(PropertyID(30022), IsOffscreenPropertyId)
	c.Equal(PropertyID(30023), OrientationPropertyId)
	c.Equal(PropertyID(30024), FrameworkIdPropertyId)
	c.Equal(PropertyID(30026), ItemStatusPropertyId)
	c.Equal(PropertyID(30103), IsDataValidForFormPropertyId)
	c.Equal(PropertyID(30104), ControllerForPropertyId)
	c.Equal(PropertyID(30105), DescribedByPropertyId)
	c.Equal(PropertyID(30107), ProviderDescriptionPropertyId)
	c.Equal(PropertyID(30135), LiveSettingPropertyId)
	c.Equal(PropertyID(30152), PositionInSetPropertyId)
	c.Equal(PropertyID(30153), SizeOfSetPropertyId)
	c.Equal(PropertyID(30154), LevelPropertyId)
	c.Equal(PropertyID(30159), FullDescriptionPropertyId)
	c.Equal(PropertyID(30173), HeadingLevelPropertyId)
	c.Equal(PropertyID(30174), IsDialogPropertyId)
	c.Equal(PropertyID(30045), ValueValuePropertyId)
	c.Equal(PropertyID(30046), ValueIsReadOnlyPropertyId)
	c.Equal(PropertyID(30047), RangeValueValuePropertyId)
	c.Equal(PropertyID(30048), RangeValueIsReadOnlyPropertyId)
	c.Equal(PropertyID(30049), RangeValueMinimumPropertyId)
	c.Equal(PropertyID(30050), RangeValueMaximumPropertyId)
	c.Equal(PropertyID(30051), RangeValueLargeChangePropertyId)
	c.Equal(PropertyID(30052), RangeValueSmallChangePropertyId)
	c.Equal(PropertyID(30059), SelectionSelectionPropertyId)
	c.Equal(PropertyID(30060), SelectionCanSelectMultiplePropertyId)
	c.Equal(PropertyID(30061), SelectionIsSelectionRequiredPropertyId)
	c.Equal(PropertyID(30062), GridRowCountPropertyId)
	c.Equal(PropertyID(30063), GridColumnCountPropertyId)
	c.Equal(PropertyID(30064), GridItemRowPropertyId)
	c.Equal(PropertyID(30065), GridItemColumnPropertyId)
	c.Equal(PropertyID(30066), GridItemRowSpanPropertyId)
	c.Equal(PropertyID(30067), GridItemColumnSpanPropertyId)
	c.Equal(PropertyID(30068), GridItemContainingGridPropertyId)
	c.Equal(PropertyID(30070), ExpandCollapseExpandCollapseStatePropertyId)
	c.Equal(PropertyID(30073), WindowCanMaximizePropertyId)
	c.Equal(PropertyID(30074), WindowCanMinimizePropertyId)
	c.Equal(PropertyID(30075), WindowWindowVisualStatePropertyId)
	c.Equal(PropertyID(30076), WindowWindowInteractionStatePropertyId)
	c.Equal(PropertyID(30077), WindowIsModalPropertyId)
	c.Equal(PropertyID(30078), WindowIsTopmostPropertyId)
	c.Equal(PropertyID(30079), SelectionItemIsSelectedPropertyId)
	c.Equal(PropertyID(30080), SelectionItemSelectionContainerPropertyId)
	c.Equal(PropertyID(30081), TableRowHeadersPropertyId)
	c.Equal(PropertyID(30082), TableColumnHeadersPropertyId)
	c.Equal(PropertyID(30083), TableRowOrColumnMajorPropertyId)
	c.Equal(PropertyID(30084), TableItemRowHeaderItemsPropertyId)
	c.Equal(PropertyID(30085), TableItemColumnHeaderItemsPropertyId)
	c.Equal(PropertyID(30086), ToggleToggleStatePropertyId)

	// The pattern availability properties, which is how a pattern appearing or vanishing is reported.
	c.Equal(PropertyID(30028), IsExpandCollapsePatternAvailablePropertyId)
	c.Equal(PropertyID(30029), IsGridItemPatternAvailablePropertyId)
	c.Equal(PropertyID(30030), IsGridPatternAvailablePropertyId)
	c.Equal(PropertyID(30031), IsInvokePatternAvailablePropertyId)
	c.Equal(PropertyID(30033), IsRangeValuePatternAvailablePropertyId)
	c.Equal(PropertyID(30035), IsScrollItemPatternAvailablePropertyId)
	c.Equal(PropertyID(30036), IsSelectionItemPatternAvailablePropertyId)
	c.Equal(PropertyID(30037), IsSelectionPatternAvailablePropertyId)
	c.Equal(PropertyID(30038), IsTablePatternAvailablePropertyId)
	c.Equal(PropertyID(30039), IsTableItemPatternAvailablePropertyId)
	c.Equal(PropertyID(30041), IsTogglePatternAvailablePropertyId)
	c.Equal(PropertyID(30043), IsValuePatternAvailablePropertyId)
	c.Equal(PropertyID(30044), IsWindowPatternAvailablePropertyId)
	c.Equal(PropertyID(30040), IsTextPatternAvailablePropertyId)
	c.Equal(PropertyID(30119), IsTextPattern2AvailablePropertyId)
	c.Equal(PropertyID(30136), IsTextChildPatternAvailablePropertyId)

	// Heading levels, which are identifiers of their own rather than plain integers.
	c.Equal(HeadingLevelID(80050), HeadingLevel_None)
	c.Equal(HeadingLevelID(80051), HeadingLevel1)
	c.Equal(HeadingLevelID(80052), HeadingLevel2)
	c.Equal(HeadingLevelID(80053), HeadingLevel3)
	c.Equal(HeadingLevelID(80054), HeadingLevel4)
	c.Equal(HeadingLevelID(80055), HeadingLevel5)
	c.Equal(HeadingLevelID(80056), HeadingLevel6)
	c.Equal(HeadingLevelID(80057), HeadingLevel7)
	c.Equal(HeadingLevelID(80058), HeadingLevel8)
	c.Equal(HeadingLevelID(80059), HeadingLevel9)

	// The Text pattern's enumerations. The units and the endpoints are sequences a client passes in, so a value out of
	// place would have a range expanded by the wrong unit or moved at the wrong end; the selection kinds are what
	// get_SupportedTextSelection answers with.
	c.Equal(TextUnit(0), TextUnit_Character)
	c.Equal(TextUnit(1), TextUnit_Format)
	c.Equal(TextUnit(2), TextUnit_Word)
	c.Equal(TextUnit(3), TextUnit_Line)
	c.Equal(TextUnit(4), TextUnit_Paragraph)
	c.Equal(TextUnit(5), TextUnit_Page)
	c.Equal(TextUnit(6), TextUnit_Document)
	c.Equal(6, int(TextUnit_Document), "textUnitCount depends on the document being the last unit")
	c.Equal(textUnitCount, int(TextUnit_Document)+1)
	c.Equal(TextPatternRangeEndpoint(0), TextPatternRangeEndpoint_Start)
	c.Equal(TextPatternRangeEndpoint(1), TextPatternRangeEndpoint_End)
	c.Equal(SupportedTextSelection(0), SupportedTextSelection_None)
	c.Equal(SupportedTextSelection(1), SupportedTextSelection_Single)
	c.Equal(SupportedTextSelection(2), SupportedTextSelection_Multiple)

	// The text attributes, which a client asks for by identifier and searches by.
	c.Equal(TextAttributeID(40004), CultureAttributeId)
	c.Equal(TextAttributeID(40005), FontNameAttributeId)
	c.Equal(TextAttributeID(40006), FontSizeAttributeId)
	c.Equal(TextAttributeID(40007), FontWeightAttributeId)
	c.Equal(TextAttributeID(40013), IsHiddenAttributeId)
	c.Equal(TextAttributeID(40014), IsItalicAttributeId)
	c.Equal(TextAttributeID(40015), IsReadOnlyAttributeId)
	c.Equal(TextAttributeID(40026), StrikethroughStyleAttributeId)
	c.Equal(TextAttributeID(40030), UnderlineStyleAttributeId)
	c.Equal(TextAttributeID(40031), AnnotationTypesAttributeId)
	c.Equal(TextAttributeID(40033), StyleNameAttributeId)
	c.Equal(TextAttributeID(40034), StyleIdAttributeId)
	c.Equal(TextAttributeID(40035), LinkAttributeId)
	c.Equal(TextAttributeID(40036), IsActiveAttributeId)

	// The decoration styles and the style identifiers, which are the values of four of those attributes.
	c.Equal(TextDecorationLineStyle(0), TextDecorationLineStyle_None)
	c.Equal(TextDecorationLineStyle(1), TextDecorationLineStyle_Single)
	c.Equal(StyleID(70000), StyleId_Custom)
	c.Equal(StyleID(70001), StyleId_Heading1)
	c.Equal(StyleID(70002), StyleId_Heading2)
	c.Equal(StyleID(70003), StyleId_Heading3)
	c.Equal(StyleID(70004), StyleId_Heading4)
	c.Equal(StyleID(70005), StyleId_Heading5)
	c.Equal(StyleID(70006), StyleId_Heading6)
	c.Equal(StyleID(70007), StyleId_Heading7)
	c.Equal(StyleID(70008), StyleId_Heading8)
	c.Equal(StyleID(70009), StyleId_Heading9)
	c.Equal(StyleID(70010), StyleId_Title)
	c.Equal(StyleID(70011), StyleId_Subtitle)
	c.Equal(StyleID(70012), StyleId_Normal)
	c.Equal(StyleID(70013), StyleId_Emphasis)
	c.Equal(StyleID(70014), StyleId_Quote)
	c.Equal(StyleID(70015), StyleId_BulletedList)
	c.Equal(StyleID(70016), StyleId_NumberedList)

	// The two bare integers UI Automation defines: the WM_GETOBJECT lParam and the runtime-id prefix.
	c.Equal(int32(-25), RootObjectId)
	c.Equal(int32(3), AppendRuntimeId)

	// The provider options, which are bit values rather than a sequence: get_ProviderOptions reports
	// ServerSideProvider, and reporting ClientSideProvider or UseComThreading by accident would have UI Automation
	// treat a free-threaded server-side provider as something else entirely.
	c.Equal(ProviderOptions(0x1), ProviderOptions_ClientSideProvider)
	c.Equal(ProviderOptions(0x2), ProviderOptions_ServerSideProvider)
	c.Equal(ProviderOptions(0x4), ProviderOptions_NonClientAreaProvider)
	c.Equal(ProviderOptions(0x8), ProviderOptions_OverrideProvider)
	c.Equal(ProviderOptions(0x10), ProviderOptions_ProviderOwnsSetFocus)
	c.Equal(ProviderOptions(0x20), ProviderOptions_UseComThreading)

	// The VARIANT type tags and the VARIANT_BOOL values, which are wtypes.h's rather than UI Automation's. A wrong tag
	// has a client read a property's bits as the wrong type, and a VARIANT_BOOL of one rather than every bit set is
	// neither true nor false to most COM clients.
	c.Equal(VARTYPE(0), VT_EMPTY)
	c.Equal(VARTYPE(3), VT_I4)
	c.Equal(VARTYPE(5), VT_R8)
	c.Equal(VARTYPE(8), VT_BSTR)
	c.Equal(VARTYPE(11), VT_BOOL)
	c.Equal(VARTYPE(13), VT_UNKNOWN)
	c.Equal(VARTYPE(0x2000), VT_ARRAY)
	c.Equal(int16(-1), VARIANT_TRUE)
	c.Equal(int16(0), VARIANT_FALSE)

	// The UI Automation HRESULTs a provider refuses with. A client turns each into a different error, and the whole
	// distinction between "gone", "never did that" and "not just now" is these four numbers.
	c.Equal(uint64(0x80040200), E_ELEMENTNOTENABLED)
	c.Equal(uint64(0x80040201), E_ELEMENTNOTAVAILABLE)
	c.Equal(uint64(0x80040204), E_NOTSUPPORTED)
	c.Equal(uint64(0x80131509), E_INVALIDOPERATION)
}

// TestViews verifies which nodes belong to the control and content views. The interesting case is a label: it stays
// in the content view while it names nothing, and drops out once another node says it is labeled by it, because that
// node already reports the label's text as its own name.
func TestViews(t *testing.T) {
	c := check.New(t)
	tree := sampleTree()

	c.False(IsControlElement(nil))
	c.False(IsControlElement(tree.Node(2)))
	c.True(IsControlElement(tree.Node(4)))
	c.True(IsControlElement(tree.Node(7)))

	c.False(IsContentElement(tree, tree.Node(2)))
	c.True(IsContentElement(tree, tree.Node(4)))
	c.False(IsContentElement(tree, tree.Node(6)), "a label that names node 4 is not content")

	lone := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2, 3, 4, 5}},
		&accessibility.Node{ID: 2, Role: role.Label, Name: "Standing alone"},
		&accessibility.Node{ID: 3, Role: role.Separator},
		&accessibility.Node{ID: 4, Role: role.ScrollBar},
		&accessibility.Node{ID: 5, Role: role.Tooltip},
	)
	c.True(IsContentElement(lone, lone.Node(2)), "a label that names nothing is content")
	c.False(IsContentElement(lone, lone.Node(3)))
	c.False(IsContentElement(lone, lone.Node(4)))
	c.False(IsContentElement(lone, lone.Node(5)))
}

// TestHasKeyboardFocus verifies that at most one element of a fragment claims the keyboard focus, and only while the
// window is the active one.
//
// The root is the trap: its Focused flag says the window is active rather than that the window itself is where typing
// goes, so answering the property from that flag would have the root claim the focus alongside the control that really
// has it — while reporting that it cannot be focused at all, since a root is never focusable. It claims nothing for an
// empty focus either, which is the answer GetFocus gives for one.
func TestHasKeyboardFocus(t *testing.T) {
	c := check.New(t)
	tree := sampleTree()
	c.True(HasKeyboardFocus(tree, tree.Node(4)), "the node the snapshot's Focus names")
	c.False(HasKeyboardFocus(tree, tree.Node(1)), "the root, while something inside the window has the focus")
	c.False(HasKeyboardFocus(tree, tree.Node(5)))

	// An inactive window holds no keyboard focus anywhere, however its nodes are marked.
	inactive := sampleTree()
	inactive.Node(1).Focused = false
	c.False(HasKeyboardFocus(inactive, inactive.Node(4)))
	c.False(HasKeyboardFocus(inactive, inactive.Node(1)))

	// With nothing inside the window focused, no element claims the keyboard: the root would have to report
	// HasKeyboardFocus true alongside IsKeyboardFocusable false, which is a pair a client cannot make sense of, and
	// IRawElementProviderFragmentRoot::GetFocus answers with a NULL element in exactly this case.
	bare := sampleTree()
	bare.Focus = 0
	bare.Node(4).Focused = false
	c.False(HasKeyboardFocus(bare, bare.Node(1)))
	c.False(HasKeyboardFocus(bare, bare.Node(4)), "a stale Focused flag does not decide it; the tree's Focus does")

	// A stale Focused flag on the node the tree still names is not what decides it either.
	stale := sampleTree()
	stale.Focus = 0
	c.False(HasKeyboardFocus(stale, stale.Node(4)))

	c.False(HasKeyboardFocus(nil, tree.Node(4)))
	c.False(HasKeyboardFocus(tree, nil))
}

// TestHeadingLevel verifies that only headings report a level, that the nine UI Automation levels are numbered from
// the right place, and that a deeper heading clamps rather than running off the end of the enumeration.
func TestHeadingLevel(t *testing.T) {
	c := check.New(t)
	c.Equal(HeadingLevel_None, HeadingLevel(nil))
	c.Equal(HeadingLevel_None, HeadingLevel(&accessibility.Node{Role: role.Heading}))
	c.Equal(HeadingLevel_None, HeadingLevel(&accessibility.Node{Role: role.Label, Level: 2}))
	c.Equal(HeadingLevel1, HeadingLevel(&accessibility.Node{Role: role.Heading, Level: 1}))
	c.Equal(HeadingLevel3, HeadingLevel(&accessibility.Node{Role: role.Heading, Level: 3}))
	c.Equal(HeadingLevel9, HeadingLevel(&accessibility.Node{Role: role.Heading, Level: 9}))
	c.Equal(HeadingLevel9, HeadingLevel(&accessibility.Node{Role: role.Heading, Level: 12}))
	c.Equal(HeadingLevelID(80053), HeadingLevel3)
}

// TestOrientation verifies the orientation mapping.
func TestOrientation(t *testing.T) {
	c := check.New(t)
	c.Equal(OrientationType_None, Orientation(nil))
	c.Equal(OrientationType_None, Orientation(&accessibility.Node{Role: role.Slider}))
	c.Equal(OrientationType_Horizontal, Orientation(&accessibility.Node{
		Role:        role.Slider,
		Orientation: accessibility.OrientationHorizontal,
	}))
	c.Equal(OrientationType_Vertical, Orientation(&accessibility.Node{
		Role:        role.ScrollBar,
		Orientation: accessibility.OrientationVertical,
	}))
}

// TestToggleState verifies that a toggle button reports its pressed state while everything else checkable reports
// its check state, and that a mixed check becomes indeterminate rather than on.
func TestToggleState(t *testing.T) {
	c := check.New(t)
	c.Equal(ToggleState_Off, ToggleStateOf(nil))
	c.Equal(ToggleState_Off, ToggleStateOf(&accessibility.Node{Role: role.ToggleButton}))
	c.Equal(ToggleState_On, ToggleStateOf(&accessibility.Node{Role: role.ToggleButton, Pressed: true}))
	c.Equal(ToggleState_Off, ToggleStateOf(&accessibility.Node{
		Role: role.CheckBox, HasCheck: true, Checked: checkenum.Off,
	}))
	c.Equal(ToggleState_On, ToggleStateOf(&accessibility.Node{
		Role: role.CheckBox, HasCheck: true, Checked: checkenum.On,
	}))
	c.Equal(ToggleState_Indeterminate, ToggleStateOf(&accessibility.Node{
		Role: role.CheckBox, HasCheck: true, Checked: checkenum.Mixed,
	}))
	c.Equal(ToggleState_On, ToggleStateOf(&accessibility.Node{
		Role: role.MenuItem, HasCheck: true, Checked: checkenum.On,
	}))
}

// TestExpandCollapseState verifies that a node which cannot expand is reported as a leaf, which is a different
// answer from being collapsed.
func TestExpandCollapseState(t *testing.T) {
	c := check.New(t)
	c.Equal(ExpandCollapseState_LeafNode, ExpandCollapseStateOf(nil))
	c.Equal(ExpandCollapseState_LeafNode, ExpandCollapseStateOf(&accessibility.Node{Role: role.Row}))
	c.Equal(ExpandCollapseState_Collapsed, ExpandCollapseStateOf(&accessibility.Node{
		Role: role.Row, Expandable: true,
	}))
	c.Equal(ExpandCollapseState_Expanded, ExpandCollapseStateOf(&accessibility.Node{
		Role: role.Row, Expandable: true, Expanded: true,
	}))
}

// TestItemStatus verifies that the item status reports a column header's sort direction, that a node which is
// working says so, and nothing otherwise.
func TestItemStatus(t *testing.T) {
	c := check.New(t)
	c.Equal("", ItemStatus(nil))
	c.Equal("", ItemStatus(&accessibility.Node{Role: role.ColumnHeader}))
	// The value is free text a screen reader speaks exactly as it is given, so it is a translated phrase rather than
	// the name the SortDirection enumeration goes by.
	c.Equal("Sorted ascending", ItemStatus(&accessibility.Node{
		Role: role.ColumnHeader, Sort: accessibility.SortAscending,
	}))
	c.Equal("Sorted descending", ItemStatus(&accessibility.Node{
		Role: role.ColumnHeader, Sort: accessibility.SortDescending,
	}))

	// An indeterminate progress bar is the reason Busy is reported here at all: it fills in no number, so this is the
	// only thing it has to say for itself.
	indeterminate := &accessibility.Node{Role: role.ProgressBar, Busy: true, ReadOnly: true}
	c.Equal("Busy", ItemStatus(indeterminate))
	c.Equal(PatternSet(0), Patterns(indeterminate), "and it has no range for a client to read instead")
	c.Equal("", ItemStatus(&accessibility.Node{
		Role: role.ProgressBar, HasNumber: true, Number: 3, Max: 10, ReadOnly: true,
	}), "while a determinate one says nothing, since its value carries the news")

	// Both at once is reachable — Node.Busy is public API, and nothing stops a header that is sorting from setting it —
	// and a client speaks the property as one piece of text.
	c.Equal("Sorted ascending, Busy", ItemStatus(&accessibility.Node{
		Role: role.ColumnHeader, Sort: accessibility.SortAscending, Busy: true,
	}))
}

// TestWindowState verifies the window interaction state and the RangeValue increments.
func TestWindowState(t *testing.T) {
	c := check.New(t)
	c.Equal(WindowInteractionState_ReadyForUserInteraction, WindowInteractionStateOf(nil))
	c.Equal(WindowInteractionState_ReadyForUserInteraction, WindowInteractionStateOf(&accessibility.Node{
		Role: role.Window,
	}))
	c.Equal(WindowInteractionState_BlockedByModalWindow, WindowInteractionStateOf(&accessibility.Node{
		Role: role.Window, Disabled: true,
	}))

	c.Equal(float64(0), SmallChange(nil))
	c.Equal(float64(0), LargeChange(nil))
	slider := &accessibility.Node{Role: role.Slider, HasNumber: true, Step: 0.5}
	c.Equal(0.5, SmallChange(slider))
	c.Equal(5.0, LargeChange(slider))
}

// TestRuntimeID verifies that a node id survives being split across the two 32-bit halves a runtime identifier is
// made of, including an id large enough to need the high half.
func TestRuntimeID(t *testing.T) {
	c := check.New(t)
	c.Equal([]int32{AppendRuntimeId, 1, 0}, RuntimeID(1))
	c.Equal([]int32{AppendRuntimeId, -1, 0}, RuntimeID(0xFFFFFFFF))
	c.Equal([]int32{AppendRuntimeId, 0, 1}, RuntimeID(0x100000000))
	c.Equal([]int32{AppendRuntimeId, 2, 3}, RuntimeID(0x300000002))
}

// TestNavigate verifies that navigation runs over the unignored tree: the two layers of ignored grouping in
// sampleTree must be invisible, so the window's children are the controls inside them.
func TestNavigate(t *testing.T) {
	c := check.New(t)
	tree := sampleTree()
	for i, one := range []struct {
		from      accessibility.NodeID
		direction NavigateDirection
		want      accessibility.NodeID
	}{
		{from: 1, direction: NavigateDirection_FirstChild, want: 4},
		{from: 1, direction: NavigateDirection_LastChild, want: 7},
		{from: 1, direction: NavigateDirection_Parent, want: 0},
		{from: 1, direction: NavigateDirection_NextSibling, want: 0},
		{from: 1, direction: NavigateDirection_PreviousSibling, want: 0},
		{from: 4, direction: NavigateDirection_Parent, want: 1},
		{from: 4, direction: NavigateDirection_PreviousSibling, want: 0},
		{from: 4, direction: NavigateDirection_NextSibling, want: 5},
		{from: 5, direction: NavigateDirection_NextSibling, want: 6},
		{from: 6, direction: NavigateDirection_NextSibling, want: 7},
		{from: 6, direction: NavigateDirection_PreviousSibling, want: 5},
		{from: 7, direction: NavigateDirection_NextSibling, want: 0},
		{from: 7, direction: NavigateDirection_FirstChild, want: 8},
		{from: 7, direction: NavigateDirection_LastChild, want: 9},
		{from: 8, direction: NavigateDirection_Parent, want: 7},
		{from: 8, direction: NavigateDirection_NextSibling, want: 9},
		{from: 9, direction: NavigateDirection_NextSibling, want: 0},
		{from: 4, direction: NavigateDirection_FirstChild, want: 0},
		{from: 0, direction: NavigateDirection_FirstChild, want: 0},
		{from: 99, direction: NavigateDirection_Parent, want: 0},
	} {
		c.Equal(one.want, Navigate(tree, one.from, one.direction), "case %d", i)
	}
	c.Equal(accessibility.NodeID(0), Navigate(nil, 1, NavigateDirection_FirstChild))
	c.Equal(accessibility.NodeID(0), Navigate(tree, 1, NavigateDirection(99)))
}

// TestHitTest verifies that a point resolves to the deepest node covering it, that overlapping siblings resolve to
// the first of them, that an offscreen node is passed over, and that a hit on a node with no provider resolves to the
// nearest ancestor that has one.
func TestHitTest(t *testing.T) {
	c := check.New(t)
	tree := sampleTree()
	c.Equal(accessibility.NodeID(4), HitTest(tree, geom.NewPoint(10, 10)))
	c.Equal(accessibility.NodeID(5), HitTest(tree, geom.NewPoint(60, 10)))
	c.Equal(accessibility.NodeID(6), HitTest(tree, geom.NewPoint(10, 60)))
	c.Equal(accessibility.NodeID(8), HitTest(tree, geom.NewPoint(10, 110)), "the first of two stacked siblings wins")
	c.Equal(accessibility.NodeID(7), HitTest(tree, geom.NewPoint(150, 150)))
	c.Equal(accessibility.NodeID(1), HitTest(tree, geom.NewPoint(150, 10)),
		"a hit on an ignored group reports the nearest unignored ancestor")
	c.Equal(accessibility.NodeID(0), HitTest(tree, geom.NewPoint(500, 500)))
	c.Equal(accessibility.NodeID(0), HitTest(nil, geom.NewPoint(10, 10)))

	tree.Node(8).Offscreen = true
	c.Equal(accessibility.NodeID(9), HitTest(tree, geom.NewPoint(10, 110)), "an offscreen node is passed over")
}

// TestPositionInSet verifies that rows are numbered from what the snapshot recorded rather than from what is in the
// tree, since a table publishes only the rows in its viewport, while tabs and menu items are numbered by counting
// siblings.
func TestPositionInSet(t *testing.T) {
	c := check.New(t)
	table := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{
			ID: 2, Role: role.Table, RowCount: 500, ColumnCount: 2,
			Children: []accessibility.NodeID{3, 4},
		},
		&accessibility.Node{ID: 3, Role: role.Row, RowIndex: 120, Children: []accessibility.NodeID{5}},
		&accessibility.Node{ID: 4, Role: role.Row, RowIndex: 121},
		&accessibility.Node{ID: 5, Role: role.Cell, RowIndex: 120, ColumnIndex: 0},
	)
	position, size := PositionInSet(table, table.Node(3))
	c.Equal(121, position)
	c.Equal(500, size)
	position, size = PositionInSet(table, table.Node(4))
	c.Equal(122, position)
	c.Equal(500, size)
	position, size = PositionInSet(table, table.Node(5))
	c.Equal(0, position, "a cell is not one of a numbered set")
	c.Equal(0, size)
	position, size = PositionInSet(table, table.Node(2))
	c.Equal(0, position)
	c.Equal(0, size)

	// A snapshot that reports a row past the end of its container — or before the start of it — is not trusted to
	// number it: a stale RowIndex would otherwise have a client announce "row 700 of 500", which is worse than no
	// position at all. Counting siblings is what is left, and the row is the only one of its kind published here.
	for _, rowIndex := range []int{500, 700, -1} {
		stale := newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Table, RowCount: 500, Children: []accessibility.NodeID{3}},
			&accessibility.Node{ID: 3, Role: role.Row, RowIndex: rowIndex},
		)
		position, size = PositionInSet(stale, stale.Node(3))
		c.Equal(1, position, "row index %d", rowIndex)
		c.Equal(1, size, "row index %d", rowIndex)
	}

	// A row whose container records no count at all falls back to counting siblings too, which is also what a list
	// does.
	list := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.List, Children: []accessibility.NodeID{3, 4, 5}},
		&accessibility.Node{ID: 3, Role: role.ListItem},
		&accessibility.Node{ID: 4, Role: role.Separator},
		&accessibility.Node{ID: 5, Role: role.ListItem},
	)
	position, size = PositionInSet(list, list.Node(5))
	c.Equal(2, position, "the separator between the items is not counted")
	c.Equal(2, size)

	tabs := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.TabList, Children: []accessibility.NodeID{3, 4}},
		&accessibility.Node{ID: 3, Role: role.Tab, Selected: true},
		&accessibility.Node{ID: 4, Role: role.Tab},
	)
	position, size = PositionInSet(tabs, tabs.Node(4))
	c.Equal(2, position)
	c.Equal(2, size)

	// A radio button is numbered the same way, which is the "n of m" a client announces alongside the state and the
	// whole reason the role is given the SelectionItem pattern. Its group is an ignored layout panel, so the siblings
	// counted are the ones the window sees.
	radios := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.Group, Ignored: true, Children: []accessibility.NodeID{3, 4, 5}},
		&accessibility.Node{ID: 3, Role: role.RadioButton, HasCheck: true, Checked: checkenum.On},
		&accessibility.Node{ID: 4, Role: role.RadioButton, HasCheck: true},
		&accessibility.Node{ID: 5, Role: role.RadioButton, HasCheck: true},
	)
	position, size = PositionInSet(radios, radios.Node(4))
	c.Equal(2, position)
	c.Equal(3, size)

	position, size = PositionInSet(nil, nil)
	c.Equal(0, position)
	c.Equal(0, size)
}

// eventTree builds the hierarchy the event-decision tests work over. Each call produces fresh nodes, so a test may
// mutate one side of a comparison freely.
//
//	1 window [focused]
//	├─ 2 text-field [focused]
//	├─ 3 check-box
//	├─ 4 list [multiselectable]
//	│  ├─ 5 list-item [selected]
//	│  └─ 6 list-item
//	├─ 7 radio-button [checked]
//	├─ 8 slider
//	└─ 9 group [ignored]
//	   └─ 10 button
//
// The radio button hangs directly off the window, which is not multiselectable, so it exercises the single-selection
// path while the list items exercise the multiple-selection one.
func eventTree() *accessibility.Tree {
	return newTestTree(1, 2,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Window", Focused: true,
			Children: []accessibility.NodeID{2, 3, 4, 7, 8, 9},
		},
		&accessibility.Node{
			ID: 2, Role: role.TextField, Focusable: true, Focused: true, Value: "hello",
			Text: &accessibility.TextInfo{Text: "hello", SelStart: 5, SelEnd: 5},
		},
		&accessibility.Node{ID: 3, Role: role.CheckBox, HasCheck: true, Checked: checkenum.On},
		&accessibility.Node{
			ID: 4, Role: role.List, Multiselectable: true,
			Children: []accessibility.NodeID{5, 6},
		},
		&accessibility.Node{ID: 5, Role: role.ListItem, Selectable: true, Selected: true},
		&accessibility.Node{ID: 6, Role: role.ListItem, Selectable: true},
		&accessibility.Node{ID: 7, Role: role.RadioButton, HasCheck: true, Checked: checkenum.On},
		&accessibility.Node{ID: 8, Role: role.Slider, HasNumber: true, Number: 5, Max: 10, Step: 1},
		&accessibility.Node{ID: 9, Role: role.Group, Ignored: true, Children: []accessibility.NodeID{10}},
		&accessibility.Node{ID: 10, Role: role.Button, Name: "Deep"},
	)
}

// raiseEvent, raiseProperty, raiseStructure and raiseDisconnect build the expected results of DecideRaises, so that
// a test reads as a list of calls rather than as a list of struct literals.
func raiseEvent(node accessibility.NodeID, id EventID) Raise {
	return Raise{Kind: RaiseEvent, Node: node, Event: id}
}

func raiseProperty(node accessibility.NodeID, id PropertyID) Raise {
	return Raise{Kind: RaiseProperty, Node: node, Property: id}
}

func raiseStructure(node, child accessibility.NodeID, change StructureChangeType) Raise {
	return Raise{Kind: RaiseStructure, Node: node, Child: child, Change: change}
}

func raiseDisconnect(node accessibility.NodeID) Raise {
	return Raise{Kind: RaiseDisconnect, Node: node}
}

// TestDecideRaisesNothing verifies the empty cases: no current tree at all, and a batch with no events in it.
func TestDecideRaisesNothing(t *testing.T) {
	c := check.New(t)
	c.Nil(DecideRaises(nil, nil, nil))
	c.Nil(DecideRaises(eventTree(), nil, nil))
	c.Nil(DecideRaises(eventTree(), eventTree(), nil))
}

// TestDecideRaisesFocus verifies that a focus change is reported only while the window itself is active. A screen
// reader follows a focus event by moving its cursor, so raising one for a window the user is not looking at pulls them
// away from the one they are.
func TestDecideRaisesFocus(t *testing.T) {
	c := check.New(t)
	events := []accessibility.Event{{Kind: accessibility.FocusChanged, Node: 2}}

	cur := eventTree()
	c.Equal([]Raise{raiseEvent(2, AutomationFocusChangedEventId)}, DecideRaises(eventTree(), cur, events))

	cur = eventTree()
	cur.Node(1).Focused = false
	c.Nil(DecideRaises(eventTree(), cur, events))
}

// TestDecideRaisesFocusCleared verifies that the focus moving to nothing is reported on the fragment root. A client
// answers a focus event by moving its cursor to the element the event names, so saying nothing at all would leave it on
// an element that has just stopped being focused; the root is the only element left to name, and a client that follows
// the event up is told by GetFocus that the fragment holds no focus.
func TestDecideRaisesFocusCleared(t *testing.T) {
	c := check.New(t)
	cleared := eventTree()
	cleared.Focus = 0
	cleared.Node(2).Focused = false
	c.Equal([]Raise{raiseEvent(1, AutomationFocusChangedEventId)},
		DecideRaises(eventTree(), cleared, accessibility.Diff(eventTree(), cleared)))

	// A window becoming active with nothing inside it focused points the client at the window itself for the same
	// reason.
	c.Equal([]Raise{raiseEvent(1, AutomationFocusChangedEventId)},
		DecideRaises(eventTree(), cleared, []accessibility.Event{
			{Kind: accessibility.WindowActivated, Node: 1},
		}))

	// An inactive window still says nothing, however its focus moved.
	background := eventTree()
	background.Focus = 0
	background.Node(1).Focused = false
	background.Node(2).Focused = false
	c.Nil(DecideRaises(eventTree(), background, []accessibility.Event{
		{Kind: accessibility.FocusChanged, Node: 0},
	}))
}

// TestDecideRaisesWindowOpened verifies that the first publish of a window announces that it opened, which is how a
// screen reader knows to read a dialog out as it appears.
//
// Every root raises it, not only a dialog: Window.Destroy raises Window_WindowClosed for every window, and the
// Window control type lists both events as required, so a client tracking window lifetimes must not be told that a
// window it was never told about has closed.
func TestDecideRaisesWindowOpened(t *testing.T) {
	c := check.New(t)
	dialog := eventTree()
	dialog.Node(1).Role = role.Dialog
	c.Equal([]Raise{
		raiseEvent(1, Window_WindowOpenedEventId),
		raiseEvent(2, AutomationFocusChangedEventId),
	}, DecideRaises(nil, dialog, accessibility.Diff(nil, dialog)))

	window := eventTree()
	c.Equal([]Raise{
		raiseEvent(1, Window_WindowOpenedEventId),
		raiseEvent(2, AutomationFocusChangedEventId),
	}, DecideRaises(nil, window, accessibility.Diff(nil, window)))

	// Every publish after the first one has a previous snapshot to compare against, and announces nothing: a window
	// that said it had opened on every redraw would be read out again each time.
	next := eventTree()
	next.Node(1).Role = role.Dialog
	next.Node(2).Name = "Renamed"
	c.Equal([]Raise{raiseProperty(2, NamePropertyId)},
		DecideRaises(dialog, next, accessibility.Diff(dialog, next)))
}

// TestDecideRaisesInvalidationOrder verifies that a parent reporting all of its children invalidated drops the
// per-child events under it however the batch is ordered. The diff reports the invalidation first, but the decision
// must not depend on that: it is the presence of the invalidation in the batch that makes the child events redundant,
// not its position.
func TestDecideRaisesInvalidationOrder(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(4).Children = []accessibility.NodeID{5, 11}
	cur.Nodes[11] = &accessibility.Node{ID: 11, Parent: 4, Role: role.ListItem, Selectable: true}
	delete(cur.Nodes, 6)

	expected := []Raise{
		raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated),
		raiseDisconnect(6),
	}
	c.Equal(expected, DecideRaises(old, cur, []accessibility.Event{
		{Kind: accessibility.ChildrenChanged, Node: 4},
		{Kind: accessibility.NodeAdded, Node: 11},
		{Kind: accessibility.NodeRemoved, Node: 6},
	}))
	c.Equal([]Raise{
		raiseDisconnect(6),
		raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated),
	}, DecideRaises(old, cur, []accessibility.Event{
		{Kind: accessibility.NodeRemoved, Node: 6},
		{Kind: accessibility.NodeAdded, Node: 11},
		{Kind: accessibility.ChildrenChanged, Node: 4},
	}), "the invalidation still drops the child events when it arrives last")
}

// TestDecideRaisesTextProperties verifies the straightforward one-event-to-one-property translations, and the one
// event that reports two properties: the provider answers both HelpText and FullDescription from Node.Description, so
// a client that cached FullDescription and heard only about HelpText would keep a stale value forever.
func TestDecideRaisesTextProperties(t *testing.T) {
	c := check.New(t)
	cur := eventTree()
	c.Equal([]Raise{
		raiseProperty(2, NamePropertyId),
		raiseProperty(2, HelpTextPropertyId),
		raiseProperty(2, FullDescriptionPropertyId),
		raiseProperty(3, ItemStatusPropertyId),
	}, DecideRaises(eventTree(), cur, []accessibility.Event{
		{Kind: accessibility.NameChanged, Node: 2, Old: "a", New: "b"},
		{Kind: accessibility.DescriptionChanged, Node: 2},
		{Kind: accessibility.SortChanged, Node: 3},
	}))
}

// TestDecideRaisesAttributes verifies that an attributes change reports every property the provider derives from the
// fields that event covers. The event does not say which of them changed — a watermark, a level, a row index, an
// orientation, a link's target or one of the three relations — so each of the properties GetPropertyValue answers from
// them is reported once, in a fixed order, the value of a node that hands out the Value pattern last.
func TestDecideRaisesAttributes(t *testing.T) {
	c := check.New(t)
	c.Equal([]Raise{
		raiseProperty(2, HelpTextPropertyId),
		raiseProperty(2, LevelPropertyId),
		raiseProperty(2, HeadingLevelPropertyId),
		raiseProperty(2, PositionInSetPropertyId),
		raiseProperty(2, SizeOfSetPropertyId),
		raiseProperty(2, OrientationPropertyId),
		raiseProperty(2, LabeledByPropertyId),
		raiseProperty(2, DescribedByPropertyId),
		raiseProperty(2, ControllerForPropertyId),
		raiseProperty(2, ValueValuePropertyId),
	}, DecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.AttributesChanged, Node: 2},
	}))

	// An ignored node has no provider, so nothing is raised for it at all.
	c.Nil(DecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.AttributesChanged, Node: 9},
	}))

	// A description change alongside it reports HelpText once: the two events ask for the same property, and a client
	// hearing about it twice would read the same value twice.
	c.Equal([]Raise{
		raiseProperty(2, HelpTextPropertyId),
		raiseProperty(2, FullDescriptionPropertyId),
		raiseProperty(2, LevelPropertyId),
		raiseProperty(2, HeadingLevelPropertyId),
		raiseProperty(2, PositionInSetPropertyId),
		raiseProperty(2, SizeOfSetPropertyId),
		raiseProperty(2, OrientationPropertyId),
		raiseProperty(2, LabeledByPropertyId),
		raiseProperty(2, DescribedByPropertyId),
		raiseProperty(2, ControllerForPropertyId),
		raiseProperty(2, ValueValuePropertyId),
	}, DecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.DescriptionChanged, Node: 2},
		{Kind: accessibility.AttributesChanged, Node: 2},
	}))

	// A node that hands out no value pattern reports no value: a check box's state is its Toggle pattern's, and there
	// is no value property on it for a client to read.
	c.Equal([]Raise{
		raiseProperty(3, HelpTextPropertyId),
		raiseProperty(3, LevelPropertyId),
		raiseProperty(3, HeadingLevelPropertyId),
		raiseProperty(3, PositionInSetPropertyId),
		raiseProperty(3, SizeOfSetPropertyId),
		raiseProperty(3, OrientationPropertyId),
		raiseProperty(3, LabeledByPropertyId),
		raiseProperty(3, DescribedByPropertyId),
		raiseProperty(3, ControllerForPropertyId),
	}, DecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.AttributesChanged, Node: 3},
	}))
}

// TestDecideRaisesHeadingLevel verifies that a heading whose depth changed reports HeadingLevel. Node.Level answers
// two properties — Level and, through HeadingLevel, HeadingLevel — and a client reads a heading's depth from the
// second of them, so a change that reported only the first would leave it announcing the wrong depth.
func TestDecideRaisesHeadingLevel(t *testing.T) {
	c := check.New(t)
	headings := func(level int) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Heading, Name: "Section", Level: level},
		)
	}
	old, cur := headings(2), headings(3)
	c.NotEqual(HeadingLevel(old.Node(2)), HeadingLevel(cur.Node(2)), "the two snapshots must differ")
	c.Equal([]Raise{
		raiseProperty(2, HelpTextPropertyId),
		raiseProperty(2, LevelPropertyId),
		raiseProperty(2, HeadingLevelPropertyId),
		raiseProperty(2, PositionInSetPropertyId),
		raiseProperty(2, SizeOfSetPropertyId),
		raiseProperty(2, OrientationPropertyId),
		raiseProperty(2, LabeledByPropertyId),
		raiseProperty(2, DescribedByPropertyId),
		raiseProperty(2, ControllerForPropertyId),
	}, DecideRaises(old, cur, []accessibility.Event{{Kind: accessibility.AttributesChanged, Node: 2}}))
}

// TestDecideRaisesLabelContent verifies that a change to one node's LabeledBy relation reports the content-view
// change it makes to the label at the other end of it. A label that names another element is left out of the content
// view, so the label's own IsContentElement moves when something starts or stops naming it — and the event names the
// node whose attributes changed rather than the label, so nothing else would report it.
func TestDecideRaisesLabelContent(t *testing.T) {
	c := check.New(t)

	// Nodes 3 and 4 are labels; nodes 2 and 5 are fields that may name them.
	labeled := func(first, second []accessibility.NodeID) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{
				ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2, 3, 4, 5},
			},
			&accessibility.Node{ID: 2, Role: role.TextField, LabeledBy: first},
			&accessibility.Node{ID: 3, Role: role.Label, Name: "Name"},
			&accessibility.Node{ID: 4, Role: role.Label, Name: "Other"},
			&accessibility.Node{ID: 5, Role: role.TextField, LabeledBy: second},
		)
	}
	// The attribute properties themselves are pinned by TestDecideRaisesAttributes; what matters here is what
	// follows them. The field hands out the Value pattern, so its value comes after them; see attributes.
	expected := func(extra ...Raise) []Raise {
		raises := make([]Raise, 0, len(attributeProperties)+len(extra)+1)
		for _, propertyID := range attributeProperties {
			raises = append(raises, raiseProperty(2, propertyID))
		}
		raises = append(raises, raiseProperty(2, ValueValuePropertyId))
		return append(raises, extra...)
	}
	for i, one := range []struct {
		old      *accessibility.Tree
		cur      *accessibility.Tree
		expected []Raise
		name     string
	}{
		{
			name:     "a label that has just been given something to name leaves the content view",
			old:      labeled(nil, nil),
			cur:      labeled([]accessibility.NodeID{3}, nil),
			expected: expected(raiseProperty(3, IsContentElementPropertyId)),
		},
		{
			name:     "a label nothing names any more rejoins it",
			old:      labeled([]accessibility.NodeID{3}, nil),
			cur:      labeled(nil, nil),
			expected: expected(raiseProperty(3, IsContentElementPropertyId)),
		},
		{
			name: "a field that changed which label names it moves both of them",
			old:  labeled([]accessibility.NodeID{3}, nil),
			cur:  labeled([]accessibility.NodeID{4}, nil),
			expected: expected(raiseProperty(3, IsContentElementPropertyId),
				raiseProperty(4, IsContentElementPropertyId)),
		},
		{
			name:     "a label another field still names has not moved, so nothing is said about it",
			old:      labeled([]accessibility.NodeID{3}, []accessibility.NodeID{3}),
			cur:      labeled(nil, []accessibility.NodeID{3}),
			expected: expected(),
		},
		{
			name:     "an attributes change that leaves the relation alone reports no label at all",
			old:      labeled([]accessibility.NodeID{3}, nil),
			cur:      labeled([]accessibility.NodeID{3}, nil),
			expected: expected(),
		},
	} {
		c.Equal(one.expected, DecideRaises(one.old, one.cur, []accessibility.Event{
			{Kind: accessibility.AttributesChanged, Node: 2},
		}), "case %d (%s)", i, one.name)
	}
}

// TestDecideRaisesPatternAvailability verifies that a change which takes a state-gated pattern away, or grants one,
// is reported through that pattern's availability property — and that the pattern's own properties are reported only
// while the current snapshot still hands the pattern out.
//
// Patterns gates ExpandCollapse on Expandable, RangeValue on HasNumber, a menu item's Toggle on HasCheck and a
// cell's Value on there being a value, so a snapshot that has just lost one of those states no longer supports the
// pattern whose property would carry the news. Raising it anyway is what constants.go says must never happen;
// saying nothing at all would leave a client announcing rows as expanded forever after they had become leaves. The
// availability property is the one thing that may be raised on an element without the pattern, so that is what goes
// out, ahead of anything else about the element. Both directions are checked.
func TestDecideRaisesPatternAvailability(t *testing.T) {
	c := check.New(t)
	rows := func(expandable bool) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Tree, RowCount: 1, Children: []accessibility.NodeID{3}},
			&accessibility.Node{ID: 3, Role: role.Row, RowIndex: 0, Expandable: expandable, Expanded: expandable},
		)
	}
	scrollableRows := func(expandable, scrollable bool) *accessibility.Tree {
		tree := rows(expandable)
		if scrollable {
			tree.Nodes[3].Actions = tree.Nodes[3].Actions.With(accessibility.ScrollIntoView)
		}
		return tree
	}
	cells := func(value string) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Table, RowCount: 1, Children: []accessibility.NodeID{3}},
			&accessibility.Node{ID: 3, Role: role.Row, RowIndex: 0, Children: []accessibility.NodeID{4}},
			&accessibility.Node{ID: 4, Role: role.Cell, RowIndex: 0, ColumnIndex: 0, Value: value},
		)
	}
	sliders := func(hasNumber bool) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Slider, HasNumber: hasNumber, Number: 5, Max: 10, Step: 1},
		)
	}
	menuItems := func(hasCheck bool) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Menu, Children: []accessibility.NodeID{3}},
			&accessibility.Node{ID: 3, Role: role.MenuItem, Name: "Wrap", HasCheck: hasCheck},
		)
	}
	stateChanged := func(id accessibility.NodeID, state accessibility.State) accessibility.Event {
		return accessibility.Event{Kind: accessibility.StateChanged, Node: id, State: state}
	}
	for i, one := range []struct {
		old      *accessibility.Tree
		cur      *accessibility.Tree
		expected []Raise
		event    accessibility.Event
		name     string
	}{
		{
			name:     "a row that stopped being expandable",
			old:      rows(true),
			cur:      rows(false),
			event:    stateChanged(3, accessibility.StateExpandable),
			expected: []Raise{raiseProperty(3, IsExpandCollapsePatternAvailablePropertyId)},
		},
		{
			name:  "a row that became expandable",
			old:   rows(false),
			cur:   rows(true),
			event: stateChanged(3, accessibility.StateExpandable),
			expected: []Raise{
				raiseProperty(3, IsExpandCollapsePatternAvailablePropertyId),
				raiseProperty(3, ExpandCollapseExpandCollapseStatePropertyId),
			},
		},
		{
			// Every pattern the node gained or lost is reported, in the order the patterns are defined in, and all of
			// them before whatever else the event asks for.
			name:  "a row that lost one pattern and gained another",
			old:   scrollableRows(true, false),
			cur:   scrollableRows(false, true),
			event: stateChanged(3, accessibility.StateExpandable),
			expected: []Raise{
				raiseProperty(3, IsExpandCollapsePatternAvailablePropertyId),
				raiseProperty(3, IsScrollItemPatternAvailablePropertyId),
			},
		},
		{
			name:     "a cell whose value became empty",
			old:      cells("Checked"),
			cur:      cells(""),
			event:    accessibility.Event{Kind: accessibility.ValueChanged, Node: 4},
			expected: []Raise{raiseProperty(4, IsValuePatternAvailablePropertyId)},
		},
		{
			name:  "a cell that gained a value",
			old:   cells(""),
			cur:   cells("Checked"),
			event: accessibility.Event{Kind: accessibility.ValueChanged, Node: 4},
			expected: []Raise{
				raiseProperty(4, IsValuePatternAvailablePropertyId),
				raiseProperty(4, ValueValuePropertyId),
			},
		},
		{
			name:     "a slider that stopped reporting a number",
			old:      sliders(true),
			cur:      sliders(false),
			event:    accessibility.Event{Kind: accessibility.NumberChanged, Node: 2},
			expected: []Raise{raiseProperty(2, IsRangeValuePatternAvailablePropertyId)},
		},
		{
			name:  "a slider that started reporting one",
			old:   sliders(false),
			cur:   sliders(true),
			event: accessibility.Event{Kind: accessibility.NumberChanged, Node: 2},
			expected: []Raise{
				raiseProperty(2, IsRangeValuePatternAvailablePropertyId),
				raiseProperty(2, RangeValueValuePropertyId),
			},
		},
		{
			name:     "a menu item that stopped being checkable",
			old:      menuItems(true),
			cur:      menuItems(false),
			event:    stateChanged(3, accessibility.StateChecked),
			expected: []Raise{raiseProperty(3, IsTogglePatternAvailablePropertyId)},
		},
		{
			name:  "a menu item that became checkable",
			old:   menuItems(false),
			cur:   menuItems(true),
			event: stateChanged(3, accessibility.StateChecked),
			expected: []Raise{
				raiseProperty(3, IsTogglePatternAvailablePropertyId),
				raiseProperty(3, ToggleToggleStatePropertyId),
			},
		},
		{
			name:  "a node that has never had the pattern still reports nothing",
			old:   sliders(false),
			cur:   sliders(false),
			event: accessibility.Event{Kind: accessibility.NumberChanged, Node: 2},
		},
		{
			// An attributes change is the only event that reports a change to the action set, and ScrollItem is gated
			// on the ScrollIntoView action alone, so this is the one place the pattern's arrival can be reported.
			name:  "a row that became scrollable",
			old:   scrollableRows(false, false),
			cur:   scrollableRows(false, true),
			event: accessibility.Event{Kind: accessibility.AttributesChanged, Node: 3},
			expected: []Raise{
				raiseProperty(3, IsScrollItemPatternAvailablePropertyId),
				raiseProperty(3, HelpTextPropertyId),
				raiseProperty(3, LevelPropertyId),
				raiseProperty(3, HeadingLevelPropertyId),
				raiseProperty(3, PositionInSetPropertyId),
				raiseProperty(3, SizeOfSetPropertyId),
				raiseProperty(3, OrientationPropertyId),
				raiseProperty(3, LabeledByPropertyId),
				raiseProperty(3, DescribedByPropertyId),
				raiseProperty(3, ControllerForPropertyId),
			},
		},
		{
			// A node only one of the snapshots holds has nothing to compare: its arrival is structural, and a client
			// that has never seen it has nothing cached about it to correct.
			name:  "a node the previous snapshot did not hold",
			old:   newTestTree(1, 0, &accessibility.Node{ID: 1, Role: role.Window, Name: "Window"}),
			cur:   sliders(true),
			event: accessibility.Event{Kind: accessibility.NumberChanged, Node: 2},
			expected: []Raise{
				raiseProperty(2, RangeValueValuePropertyId),
			},
		},
	} {
		raises := DecideRaises(one.old, one.cur, []accessibility.Event{one.event})
		if one.expected == nil {
			c.Nil(raises, "case %d (%s)", i, one.name)
			continue
		}
		c.Equal(one.expected, raises, "case %d (%s)", i, one.name)
	}
}

// TestReportsProperty verifies what a raised property change is allowed to carry: a pattern's property only from a
// snapshot in which the element hands that pattern out, and — for the Window pattern — only on the fragment root, which
// is the only element the provider hands IWindowProvider to. Provider.raisedPropertyValue is what consults this, and
// it is the only guard there is: a pattern property has no GetPropertyValue answer to agree with, so nothing else would
// catch a value invented for an element that does not implement the pattern.
func TestReportsProperty(t *testing.T) {
	c := check.New(t)

	// The case that made this necessary. A spin button that becomes a password field stops reporting a number, which
	// takes the RangeValue pattern with it, and Diff still reports the number as changed: the raise goes out, and the
	// new snapshot must hand a client nothing rather than a value for a pattern the element no longer implements. The
	// protected node keeps its Number so that a report worked out from the field rather than from the pattern would
	// show up here as a value instead of as nothing; a real snapshot of a protected field leaves it at zero, which
	// would reach a client as the field's value having become "0".
	spinButtons := func(protected bool) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{
				ID: 2, Role: role.SpinButton, Name: "PIN", HasNumber: !protected, Number: 4, Max: 9,
				Protected: protected,
			},
		)
	}
	open := spinButtons(false)
	hidden := spinButtons(true)
	c.Equal([]Raise{raiseProperty(2, IsRangeValuePatternAvailablePropertyId)},
		DecideRaises(open, hidden, []accessibility.Event{{Kind: accessibility.NumberChanged, Node: 2}}),
		"the loss is still reported, since a client holding the old value has to be told it is gone, but through the"+
			" one property that may be raised on an element without the pattern")
	c.True(ReportsProperty(open, open.Node(2), RangeValueValuePropertyId))
	c.False(ReportsProperty(hidden, hidden.Node(2), RangeValueValuePropertyId),
		"while the snapshot that lost the pattern has nothing to report for its property")
	c.True(ReportsProperty(hidden, hidden.Node(2), ValueValuePropertyId),
		"the pattern it kept still answers, with the empty string every protected node gives")
	c.True(ReportsProperty(hidden, hidden.Node(2), NamePropertyId),
		"and a property no pattern owns is answered by every element")
	c.True(ReportsProperty(hidden, hidden.Node(2), IsRangeValuePatternAvailablePropertyId),
		"as is a pattern's availability, which is answered precisely by the element that no longer has the pattern")

	// Modality is the Window pattern's, and the fragment root is the only element that hands that pattern out: a nested
	// node with a window-like role is a dialog-shaped panel that the provider refuses IWindowProvider, so it must be
	// refused a modality to report through it as well. Nothing raises one on such a panel either — the decider asks the
	// same question of the same helper — and both halves are pinned here, since the two used to be independent and only
	// the raising half recorded the rule.
	dialogs := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Modal: true, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.Dialog, Name: "Sheet", Modal: true},
	)
	c.True(ReportsProperty(dialogs, dialogs.Node(1), WindowIsModalPropertyId),
		"the fragment root is a window of its own")
	c.False(ReportsProperty(dialogs, dialogs.Node(2), WindowIsModalPropertyId),
		"a nested dialog-shaped panel is not, however modal the snapshot says it is")
	c.False(ReportsProperty(nil, dialogs.Node(1), WindowIsModalPropertyId),
		"and with no tree to ask, nothing can be said to be the root")
	c.Equal([]Raise{raiseProperty(1, WindowIsModalPropertyId)},
		DecideRaises(dialogs, dialogs, []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: 1, State: accessibility.StateModal},
		}), "so the root reports a change to it")
	c.Nil(DecideRaises(dialogs, dialogs, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 2, State: accessibility.StateModal},
	}), "and the panel reports nothing")

	// Which pattern owns which property. A property answered through a pattern interface is the pattern's to report and
	// nobody else's; everything a client reads through GetPropertyValue is answered by every element.
	for i, one := range []struct {
		property PropertyID
		expected PatternSet
	}{
		{property: ValueValuePropertyId, expected: PatternValue},
		{property: ValueIsReadOnlyPropertyId, expected: PatternValue},
		{property: RangeValueValuePropertyId, expected: PatternRangeValue},
		{property: RangeValueIsReadOnlyPropertyId, expected: PatternRangeValue},
		{property: ToggleToggleStatePropertyId, expected: PatternToggle},
		{property: ExpandCollapseExpandCollapseStatePropertyId, expected: PatternExpandCollapse},
		{property: SelectionItemIsSelectedPropertyId, expected: PatternSelectionItem},
		{property: SelectionCanSelectMultiplePropertyId, expected: PatternSelection},
		{property: WindowIsModalPropertyId, expected: PatternWindow},
		{property: NamePropertyId},
		{property: ItemStatusPropertyId},
		{property: BoundingRectanglePropertyId},
		{property: IsPasswordPropertyId},
	} {
		c.Equal(one.expected, PropertyPattern(one.property), "case %d (property %d)", i, one.property)
	}

	// Every property an attributes change reports is element-wide, so none of them may be gated: a node with nothing to
	// say for one answers it empty on both sides, which a client reads as no change, and gating them would instead have
	// a node that never had the pattern drop a property it really does answer.
	for _, propertyID := range attributeProperties {
		c.Equal(PatternSet(0), PropertyPattern(propertyID), "property %d", propertyID)
	}
}

// TestPatternAvailableProperty verifies that every pattern this package implements has an availability property,
// that the two directions of the mapping agree, and that no such property is owned by the pattern it describes — an
// element without the pattern is exactly the element that has to answer one, so gating it would silence the only thing
// that can tell a client the pattern has gone.
func TestPatternAvailableProperty(t *testing.T) {
	c := check.New(t)
	seen := make(map[PropertyID]bool, len(patternInfos))
	for _, info := range patternInfos {
		c.NotEqual(PropertyID(0), info.available, "pattern %s has no availability property", info.name)
		c.False(seen[info.available], "pattern %s shares an availability property", info.name)
		seen[info.available] = true
		c.Equal(info.available, PatternAvailableProperty(info.pattern), "pattern %s", info.name)
		c.Equal(info.pattern, AvailabilityPattern(info.available), "pattern %s", info.name)
		c.Equal(PatternSet(0), PropertyPattern(info.available), "pattern %s", info.name)
		c.True(ReportsProperty(nil, nil, info.available), "pattern %s", info.name)
	}
	c.Equal(PropertyID(0), PatternAvailableProperty(0))
	c.Equal(PropertyID(0), PatternAvailableProperty(PatternValue|PatternRangeValue),
		"the availability of two patterns at once is not a property")
	c.Equal(PatternSet(0), AvailabilityPattern(NamePropertyId))
	c.Equal(PatternSet(0), AvailabilityPattern(ValueValuePropertyId))
}

// TestDecideRaisesValue verifies that a value change becomes whichever value property the element actually has. A
// text field reports its value through the Value pattern and a slider through RangeValue, so raising the Value
// pattern's property on a slider would be telling a client about a property the element does not support; a label has
// neither, so its value change is dropped.
func TestDecideRaisesValue(t *testing.T) {
	c := check.New(t)
	cur := eventTree()
	c.Equal([]Raise{raiseProperty(2, ValueValuePropertyId)}, DecideRaises(eventTree(), cur,
		[]accessibility.Event{{Kind: accessibility.ValueChanged, Node: 2}}))
	c.Equal([]Raise{raiseProperty(8, RangeValueValuePropertyId)}, DecideRaises(eventTree(), cur,
		[]accessibility.Event{{Kind: accessibility.ValueChanged, Node: 8}}))
	c.Equal([]Raise{raiseProperty(8, RangeValueValuePropertyId)}, DecideRaises(eventTree(), cur,
		[]accessibility.Event{{Kind: accessibility.NumberChanged, Node: 8}}))
	c.Nil(DecideRaises(eventTree(), cur, []accessibility.Event{{Kind: accessibility.NumberChanged, Node: 3}}))

	// A slider reporting both a textual and a numeric change still tells the client once.
	c.Equal([]Raise{raiseProperty(8, RangeValueValuePropertyId)}, DecideRaises(eventTree(), cur,
		[]accessibility.Event{
			{Kind: accessibility.ValueChanged, Node: 8},
			{Kind: accessibility.NumberChanged, Node: 8},
		}))

	// A cell whose one widget changed state reports its value, which is the Value pattern's property: a screen reader
	// reading across a row asks the cell, so a change to the widget has to arrive as a change to the cell.
	cells := func(value string) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Focused: true, Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Table, RowCount: 1, Children: []accessibility.NodeID{3}},
			&accessibility.Node{ID: 3, Role: role.Row, RowIndex: 0, Children: []accessibility.NodeID{4}},
			&accessibility.Node{ID: 4, Role: role.Cell, RowIndex: 0, ColumnIndex: 0, Value: value},
		)
	}
	c.Equal([]Raise{raiseProperty(4, ValueValuePropertyId)}, DecideRaises(cells("off"), cells("on"),
		[]accessibility.Event{{Kind: accessibility.ValueChanged, Node: 4}}))

	// A cell whose content is its name has no value to report, so there is no property to raise.
	c.Nil(DecideRaises(cells(""), cells(""), []accessibility.Event{
		{Kind: accessibility.ValueChanged, Node: 4},
	}))
}

// TestDecideRaisesText verifies what one edit to a field turns into, which is every channel the element has and each
// of them once: the value property a client reads the whole field through, UI Automation's text event that tells one
// reading through ITextProvider to read again, and the selection event that moves its caret. The diff describes that
// one edit as a value change plus the deletion and the insertion that made it plus the caret that followed, and all
// of that collapses to three raises.
//
// An element carrying no text has only the value to report, which is the answer this used to give for every element:
// both text events belong to the Text control pattern, and a client that responded to one by asking an element that
// answers NULL for it would have nothing to read. TestDecideRaisesDocumentText covers a document, and
// TestDecideRaisesLabelText an element with text and no value.
func TestDecideRaisesText(t *testing.T) {
	c := check.New(t)
	c.Equal([]Raise{
		raiseProperty(2, ValueValuePropertyId),
		raiseEvent(2, Text_TextChangedEventId),
		raiseEvent(2, Text_TextSelectionChangedEventId),
	}, DecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.ValueChanged, Node: 2, Old: "hello", New: "help"},
		{Kind: accessibility.TextDeleted, Node: 2, Start: 3, Length: 2, Old: "lo"},
		{Kind: accessibility.TextInserted, Node: 2, Start: 3, Length: 1, New: "p"},
		{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 4},
	}))

	// An edit with no value change of its own still reports the value, since that is where a client reading the field
	// whole looks for the text.
	c.Equal([]Raise{raiseEvent(2, Text_TextChangedEventId), raiseProperty(2, ValueValuePropertyId)},
		DecideRaises(eventTree(), eventTree(), []accessibility.Event{
			{Kind: accessibility.TextInserted, Node: 2, Start: 5, Length: 1, New: "!"},
		}))

	// A field carrying no text at all — which is what a Protected one publishes — has only the Value pattern, so the
	// same edit reports the value once and nothing else. The caret move reports nothing, since there is no selection
	// a client could read back.
	textless := func() *accessibility.Tree {
		tree := eventTree()
		tree.Node(2).Text = nil
		return tree
	}
	c.Equal([]Raise{raiseProperty(2, ValueValuePropertyId)},
		DecideRaises(textless(), textless(), []accessibility.Event{
			{Kind: accessibility.ValueChanged, Node: 2, Old: "hello", New: "help"},
			{Kind: accessibility.TextDeleted, Node: 2, Start: 3, Length: 2, Old: "lo"},
			{Kind: accessibility.TextInserted, Node: 2, Start: 3, Length: 1, New: "p"},
			{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 4},
		}))

	// A button has no value pattern and no text, so an edit to it reports nothing rather than an event a client cannot
	// follow up.
	c.Nil(DecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.TextInserted, Node: 10, Start: 0, Length: 1, New: "x"},
		{Kind: accessibility.TextSelectionChanged, Node: 10},
	}))
}

// TestDecideRaisesFieldText covers the two things a field's Text pattern adds beyond the edit itself: the caret event
// NVDA reads its arrow keys from, and the pattern arriving and departing as the field's text does.
func TestDecideRaisesFieldText(t *testing.T) {
	c := check.New(t)

	// A caret move with no edit behind it is the whole of NVDA's arrow-key reading: it answers the event by reading
	// the new line, word or character through ITextProvider from the selection. Before the field had the pattern this
	// reported nothing at all.
	c.Equal([]Raise{raiseEvent(2, Text_TextSelectionChangedEventId)},
		DecideRaises(eventTree(), eventTree(), []accessibility.Event{
			{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 2},
		}))

	// A field that becomes Protected publishes no text, which takes both Text patterns away. The availability
	// properties are the only thing that may be raised on an element that no longer has a pattern, and they come
	// before the state's own property.
	protected := eventTree()
	protected.Node(2).Protected = true
	protected.Node(2).Text = nil
	protected.Node(2).Value = ""
	c.Equal([]Raise{
		raiseProperty(2, IsTextPatternAvailablePropertyId),
		raiseProperty(2, IsTextPattern2AvailablePropertyId),
		raiseProperty(2, IsPasswordPropertyId),
	}, DecideRaises(eventTree(), protected, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 2, State: accessibility.StateProtected},
	}))

	// And back again, which grants them: a client holding the interface has to be told both ways.
	c.Equal([]Raise{
		raiseProperty(2, IsTextPatternAvailablePropertyId),
		raiseProperty(2, IsTextPattern2AvailablePropertyId),
		raiseProperty(2, IsPasswordPropertyId),
	}, DecideRaises(protected, eventTree(), []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 2, State: accessibility.StateProtected},
	}))

	// Measuring the field's lines changes nothing about which patterns it hands out — it carried text before and
	// carries text now — so a publish that fills them in as the field takes the focus reports no availability at all.
	// A spurious pattern-gained raise would have every client re-read an element it already knows.
	unmeasured := eventTree()
	unmeasured.Node(2).Text = &accessibility.TextInfo{Text: "hello", SelStart: 5, SelEnd: 5}
	measured := eventTree()
	measured.Node(2).Text = &accessibility.TextInfo{
		Text: "hello", SelStart: 5, SelEnd: 5,
		Lines: []accessibility.Line{{Advances: []float32{0, 1, 2, 3, 4, 5}, Start: 0, End: 5}},
	}
	for _, raise := range DecideRaises(unmeasured, measured, []accessibility.Event{
		{Kind: accessibility.FocusChanged, Node: 2},
		{Kind: accessibility.AttributesChanged, Node: 2},
	}) {
		c.NotEqual(raiseProperty(2, IsTextPatternAvailablePropertyId), raise)
		c.NotEqual(raiseProperty(2, IsTextPattern2AvailablePropertyId), raise)
	}
}

// TestDecideRaisesLabelText covers an element that carries text and no value: a label. An edit to one reports the
// text event, which is what a client reading it through ITextProvider answers by reading again, and the name change
// the snapshot raises in its own right — and nothing through a Value pattern, since a label hands out none.
func TestDecideRaisesLabelText(t *testing.T) {
	c := check.New(t)
	labels := func(text string) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Focused: true, Children: []accessibility.NodeID{2, 3}},
			&accessibility.Node{ID: 2, Role: role.Label, Name: text, Text: &accessibility.TextInfo{Text: text}},
			&accessibility.Node{ID: 3, Role: role.Label, Name: "Image only"},
		)
	}
	c.Equal([]Raise{raiseProperty(2, NamePropertyId), raiseEvent(2, Text_TextChangedEventId)},
		DecideRaises(labels("Name"), labels("Full name"), []accessibility.Event{
			{Kind: accessibility.NameChanged, Node: 2},
			{Kind: accessibility.TextDeleted, Node: 2, Start: 0, Length: 4},
			{Kind: accessibility.TextInserted, Node: 2, Start: 0, Length: 9},
		}))

	// A label showing only an image carries no text, so it hands out no pattern and an edit to it says nothing.
	c.Nil(DecideRaises(labels("Name"), labels("Name"), []accessibility.Event{
		{Kind: accessibility.TextInserted, Node: 3, Start: 0, Length: 1},
	}))

	// A label that stops carrying text loses both patterns, and the availability properties are what say so. The whole
	// point of the case is what the production path produces for it, so the events are the ones accessibility.Diff
	// really emits rather than a hand-made list: the text events describe an edit to text present on both sides, so
	// they say nothing at all here, and what carries the news is the attributes change the diff fires when one
	// snapshot carries text and the other does not.
	//
	// The label is a status label whose drawn title is cleared while the name the application set for it keeps it in
	// the tree, which is why the name changes along with the text.
	status := func(name, text string) *accessibility.Tree {
		n := &accessibility.Node{ID: 2, Role: role.Label, Name: name}
		if text != "" {
			n.Text = &accessibility.TextInfo{Text: text}
		}
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Focused: true, Children: []accessibility.NodeID{2}},
			n)
	}
	withText := status("Ready", "Ready")
	withoutText := status("Status", "")
	c.True(ProvidesPattern(withText, withText.Node(2), PatternText|PatternText2))
	c.False(ProvidedPatterns(withoutText, withoutText.Node(2)).Has(PatternText | PatternText2))

	lost := DecideRaises(withText, withoutText, accessibility.Diff(withText, withoutText))
	c.True(slices.Contains(lost, raiseProperty(2, IsTextPatternAvailablePropertyId)),
		"the pattern's departure is reported")
	c.True(slices.Contains(lost, raiseProperty(2, IsTextPattern2AvailablePropertyId)))
	c.True(slices.Contains(lost, raiseProperty(2, NamePropertyId)))

	// And the other way about: a label that starts carrying text gains both patterns, which a client that cached
	// IsTextPatternAvailable has to be told or it will never ask for ITextProvider again.
	gained := DecideRaises(withoutText, withText, accessibility.Diff(withoutText, withText))
	c.True(slices.Contains(gained, raiseProperty(2, IsTextPatternAvailablePropertyId)),
		"the pattern's arrival is reported")
	c.True(slices.Contains(gained, raiseProperty(2, IsTextPattern2AvailablePropertyId)))
	c.True(slices.Contains(gained, raiseProperty(2, NamePropertyId)))
}

// contentRowTree builds a list of two rows, one named by what it holds and one the widget named itself:
//
//	1 window [focused]
//	└─ 2 list
//	   ├─ 3 list-item            named "Alpha" by the label within it
//	   │  └─ 4 label "Alpha"
//	   └─ 5 list-item "Kept"     named by the widget, whatever it holds
//	      └─ 6 label "Beta"
func contentRowTree() *accessibility.Tree {
	return newTestTree(1, 0,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Window", Focused: true, Children: []accessibility.NodeID{2},
		},
		&accessibility.Node{ID: 2, Role: role.List, Children: []accessibility.NodeID{3, 5}},
		&accessibility.Node{ID: 3, Role: role.ListItem, Selectable: true, Children: []accessibility.NodeID{4}},
		&accessibility.Node{ID: 4, Role: role.Label, Name: "Alpha", Text: &accessibility.TextInfo{Text: "Alpha"}},
		&accessibility.Node{
			ID: 5, Role: role.ListItem, Name: "Kept", Selectable: true, Children: []accessibility.NodeID{6},
		},
		&accessibility.Node{ID: 6, Role: role.Label, Name: "Beta", Text: &accessibility.TextInfo{Text: "Beta"}},
	)
}

// TestDecideRaisesContentName verifies that a list item named by the elements it holds is reported as renamed whenever
// those elements say something different. No event names the item — the events name the label or the child that
// changed — so a client that cached the name would go on speaking the old row for the life of the window, which is
// what a screen reader reads a list with.
func TestDecideRaisesContentName(t *testing.T) {
	c := check.New(t)

	// A label's text edited beneath the row. The label reports its own rename and its own text change; the row
	// reports the name it now makes.
	edited := contentRowTree()
	edited.Node(4).Name = "Renamed"
	edited.Node(4).Text = &accessibility.TextInfo{Text: "Renamed"}
	c.Equal([]Raise{
		raiseProperty(4, NamePropertyId),
		raiseEvent(4, Text_TextChangedEventId),
		raiseProperty(3, NamePropertyId),
	}, DecideRaises(contentRowTree(), edited, accessibility.Diff(contentRowTree(), edited)))

	// The row the widget named keeps that name however its label reads, so nothing is reported about it.
	other := contentRowTree()
	other.Node(6).Name = "Changed"
	other.Node(6).Text = &accessibility.TextInfo{Text: "Changed"}
	raises := DecideRaises(contentRowTree(), other, accessibility.Diff(contentRowTree(), other))
	c.False(slices.Contains(raises, raiseProperty(5, NamePropertyId)))

	// A child arriving beneath the row adds what it says to the name. The structure change tells a client to read the
	// children again, which says nothing at all about what the row is called.
	grown := contentRowTree()
	grown.Node(3).Children = []accessibility.NodeID{4, 7}
	grown.Nodes[7] = &accessibility.Node{
		ID: 7, Parent: 3, Role: role.Label, Name: "Extra", Text: &accessibility.TextInfo{Text: "Extra"},
	}
	c.Equal("Alpha Extra", NameString(grown, grown.Node(3)))
	c.True(slices.Contains(DecideRaises(contentRowTree(), grown, accessibility.Diff(contentRowTree(), grown)),
		raiseProperty(3, NamePropertyId)), "a child arriving renames the row")

	// A publish that changed nothing beneath either row reports nothing about them, even though the batch holds an
	// event that could have.
	renamed := contentRowTree()
	renamed.Node(1).Name = "Retitled"
	c.Equal([]Raise{raiseProperty(1, NamePropertyId)},
		DecideRaises(contentRowTree(), renamed, accessibility.Diff(contentRowTree(), renamed)))

	// A row that gains a value gains the Value pattern with it, and a client that cached the availability has to be
	// told before it will ask for IValueProvider at all. The name is unchanged, so nothing is said about it.
	valued := contentRowTree()
	valued.Node(3).Value = "checked"
	c.Equal([]Raise{
		raiseProperty(3, IsValuePatternAvailablePropertyId),
		raiseProperty(3, ValueValuePropertyId),
	}, DecideRaises(contentRowTree(), valued, accessibility.Diff(contentRowTree(), valued)))
}

// TestDecideRaisesDocumentText verifies the two events UI Automation has for text, which are raised on the elements
// that hand out the Text pattern and nowhere else: a client answers TextChanged by reading the document again and
// TextSelectionChanged by reading the selection, both of which it can only do through ITextProvider.
//
// The blocks inside a document are the other half. They have no Text pattern of their own — a client reads them as
// elements — so an edit to one reports its value, and its name as well when the name is the text itself, which is what
// Narrator's item navigation speaks.
func TestDecideRaisesDocumentText(t *testing.T) {
	c := check.New(t)
	c.Equal([]Raise{
		raiseEvent(textDocumentID, Text_TextChangedEventId),
		raiseEvent(textDocumentID, Text_TextSelectionChangedEventId),
	}, DecideRaises(textFixtureTree(), textFixtureTree(), []accessibility.Event{
		{Kind: accessibility.TextDeleted, Node: textDocumentID, Start: 3, Length: 2},
		{Kind: accessibility.TextInserted, Node: textDocumentID, Start: 3, Length: 1},
		{Kind: accessibility.TextSelectionChanged, Node: textDocumentID, Start: 12},
	}), "the edit is one event however many pieces the diff describes it in")

	// A caret move on its own is the event NVDA reads as the caret having moved, which is the whole of its arrow-key
	// reading.
	c.Equal([]Raise{raiseEvent(textDocumentID, Text_TextSelectionChangedEventId)},
		DecideRaises(textFixtureTree(), textFixtureTree(), []accessibility.Event{
			{Kind: accessibility.TextSelectionChanged, Node: textDocumentID, Start: 20},
		}))

	// A paragraph within the document is named by its own content, so an edit to it renames it.
	c.Equal([]Raise{raiseProperty(textParagraphID, NamePropertyId)},
		DecideRaises(textFixtureTree(), textFixtureTree(), []accessibility.Event{
			{Kind: accessibility.TextInserted, Node: textParagraphID, Start: 0, Length: 1},
		}), "and it has no value pattern to report the text through")

	// Emptying such a block renames it just as much: its name was its text and is now nothing at all, and a client that
	// cached the old name would go on speaking text that has been deleted.
	emptied := textFixtureTree()
	emptied.Node(textParagraphID).Text = &accessibility.TextInfo{}
	c.Equal([]Raise{raiseProperty(textParagraphID, NamePropertyId)},
		DecideRaises(textFixtureTree(), emptied, []accessibility.Event{
			{Kind: accessibility.TextDeleted, Node: textParagraphID, Start: 0, Length: 16},
		}))

	// An edit that arrives on the same node as a change to the patterns it hands out reports both. A block whose
	// words a document has just composed into its stream stops answering the Text pattern and answers TextChild
	// instead, and nothing else in the diff says so: the stream arriving is an attributes change on the document, not
	// on the block, so this edit is the block's only event and the availability raises have to ride along with it.
	// Without them a client goes on reading the block through a pattern it no longer hands out.
	unclaimed := textFixtureTree()
	unclaimed.Node(textDocumentID).Document = nil
	claimed := textFixtureTree()
	claimed.Node(textParagraphID).Text = &accessibility.TextInfo{Text: "Hello link \ufffc ends"}
	c.Equal(PatternText|PatternText2, ProvidedPatterns(unclaimed, unclaimed.Node(textParagraphID)))
	c.Equal(PatternTextChild, ProvidedPatterns(claimed, claimed.Node(textParagraphID)))
	c.Equal([]Raise{
		raiseProperty(textParagraphID, IsTextPatternAvailablePropertyId),
		raiseProperty(textParagraphID, IsTextPattern2AvailablePropertyId),
		raiseProperty(textParagraphID, IsTextChildPatternAvailablePropertyId),
		raiseProperty(textParagraphID, NamePropertyId),
	}, DecideRaises(unclaimed, claimed, []accessibility.Event{
		{Kind: accessibility.TextInserted, Node: textParagraphID, Start: 15, Length: 1},
	}), "the edit is the only event the block gets, so it carries the pattern flip too")

	// A caret move within a block reports nothing: the block has no Text pattern, so there is no selection a client
	// could read from it, and the document's own event is what carries the news.
	c.Nil(DecideRaises(textFixtureTree(), textFixtureTree(), []accessibility.Event{
		{Kind: accessibility.TextSelectionChanged, Node: textParagraphID, Start: 4},
	}))

	// A Document that loses its stream loses both patterns, which is reported through the availability properties — the
	// only ones that may be raised on an element without the pattern — and arrives as an attributes change.
	before := textFixtureTree()
	after := textFixtureTree()
	after.Node(textDocumentID).Document = nil
	raises := DecideRaises(before, after, []accessibility.Event{
		{Kind: accessibility.AttributesChanged, Node: textDocumentID},
	})
	c.Equal(raiseProperty(textDocumentID, IsTextPatternAvailablePropertyId), raises[0])
	c.Equal(raiseProperty(textDocumentID, IsTextPattern2AvailablePropertyId), raises[1])

	// The same flip changes what the element is: a Document with a stream is a Document and one without is a Group, and
	// nothing but this event reports it. Narrator keys its document reading mode off the control type, so a client that
	// cached it would go on reading a group as a document.
	c.Equal(raiseProperty(textDocumentID, ControlTypePropertyId), raises[2])
	c.Equal(DocumentControlTypeId, ControlType(before, before.Node(textDocumentID)))
	c.Equal(GroupControlTypeId, ControlType(after, after.Node(textDocumentID)))

	// A change that leaves the control type alone reports none, which is what keeps the raise to the elements it is
	// true of.
	raises = DecideRaises(textFixtureTree(), textFixtureTree(), []accessibility.Event{
		{Kind: accessibility.AttributesChanged, Node: textDocumentID},
	})
	for _, raise := range raises {
		c.NotEqual(raiseProperty(textDocumentID, ControlTypePropertyId), raise)
	}

	// And so does a link that gains a target, which is what grants it the Value pattern.
	plain := textFixtureTree()
	plain.Node(textLinkID).URL = ""
	raises = DecideRaises(plain, textFixtureTree(), []accessibility.Event{
		{Kind: accessibility.AttributesChanged, Node: textLinkID},
	})
	c.Equal(raiseProperty(textLinkID, IsValuePatternAvailablePropertyId), raises[0])

	// A link re-pointed from one target to another keeps the pattern, so no availability raise says anything: the value
	// itself is the only thing that can tell a client the destination changed, and without it Narrator and Inspect go
	// on reading the old URL for the life of the window.
	repointed := textFixtureTree()
	repointed.Node(textLinkID).URL = "https://example.com/elsewhere"
	expected := make([]Raise, 0, len(attributeProperties)+1)
	for _, propertyID := range attributeProperties {
		expected = append(expected, raiseProperty(textLinkID, propertyID))
	}
	expected = append(expected, raiseProperty(textLinkID, ValueValuePropertyId))
	c.Equal(expected, DecideRaises(textFixtureTree(), repointed, []accessibility.Event{
		{Kind: accessibility.AttributesChanged, Node: textLinkID},
	}))
}

// TestDecideRaisesStates verifies the state flags that map onto a property of their own, and that a flag belonging
// to a pattern the element does not support raises nothing.
func TestDecideRaisesStates(t *testing.T) {
	c := check.New(t)
	for i, one := range []struct {
		expected []Raise
		node     accessibility.NodeID
		state    accessibility.State
	}{
		{state: accessibility.StateDisabled, node: 2, expected: []Raise{
			raiseProperty(2, IsEnabledPropertyId),
		}},
		{state: accessibility.StateFocusable, node: 2, expected: []Raise{
			raiseProperty(2, IsKeyboardFocusablePropertyId),
		}},
		{state: accessibility.StateOffscreen, node: 2, expected: []Raise{
			raiseProperty(2, IsOffscreenPropertyId),
		}},
		{state: accessibility.StateInvalid, node: 2, expected: []Raise{
			raiseProperty(2, IsDataValidForFormPropertyId),
		}},
		{state: accessibility.StateReadOnly, node: 2, expected: []Raise{
			raiseProperty(2, ValueIsReadOnlyPropertyId),
		}},
		{state: accessibility.StateReadOnly, node: 8, expected: []Raise{
			raiseProperty(8, RangeValueIsReadOnlyPropertyId),
		}},
		{state: accessibility.StateChecked, node: 3, expected: []Raise{
			raiseProperty(3, ToggleToggleStatePropertyId),
		}},
		{state: accessibility.StatePressed, node: 3, expected: []Raise{
			raiseProperty(3, ToggleToggleStatePropertyId),
		}},
		{state: accessibility.StateExpanded, node: 3},
		// Modality belongs to the Window pattern, so a window reports it and an element with no such pattern does not.
		{state: accessibility.StateModal, node: 1, expected: []Raise{
			raiseProperty(1, WindowIsModalPropertyId),
		}},
		{state: accessibility.StateModal, node: 3},
		// Whether a field hides what is typed into it is answered for every element, so it is reported for every one.
		{state: accessibility.StateProtected, node: 2, expected: []Raise{
			raiseProperty(2, IsPasswordPropertyId),
		}},
		// CanSelectMultiple belongs to the Selection pattern, so a container reports it and an element with no such
		// pattern does not.
		{state: accessibility.StateMultiselectable, node: 4, expected: []Raise{
			raiseProperty(4, SelectionCanSelectMultiplePropertyId),
		}},
		{state: accessibility.StateMultiselectable, node: 2},
		// Being busy is part of the item status, which every element answers, so it is reported for every one. An
		// indeterminate progress bar is the element that needs it: it reports no number at all, so this is the only
		// thing that changes as it starts and stops working.
		{state: accessibility.StateBusy, node: 1, expected: []Raise{
			raiseProperty(1, ItemStatusPropertyId),
		}},
		{state: accessibility.StateReadOnly, node: 1},
	} {
		raises := DecideRaises(eventTree(), eventTree(), []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: one.node, State: one.state},
		})
		if one.expected == nil {
			c.Nil(raises, "case %d (%s)", i, one.state)
			continue
		}
		c.Equal(one.expected, raises, "case %d (%s)", i, one.state)
	}
}

// TestDecideRaisesExpandCollapse verifies that a change to either half of the expandable state reports the combined
// ExpandCollapseState property, since that one property carries both.
func TestDecideRaisesExpandCollapse(t *testing.T) {
	c := check.New(t)
	tree := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Focused: true, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.Tree, RowCount: 1, Children: []accessibility.NodeID{3}},
		&accessibility.Node{ID: 3, Role: role.Row, RowIndex: 0, Expandable: true, Expanded: true},
	)
	for _, state := range []accessibility.State{accessibility.StateExpanded, accessibility.StateExpandable} {
		c.Equal([]Raise{raiseProperty(3, ExpandCollapseExpandCollapseStatePropertyId)},
			DecideRaises(tree, tree, []accessibility.Event{
				{Kind: accessibility.StateChanged, Node: 3, State: state},
			}), "%s", state)
	}
}

// TestDecideRaisesSelection verifies the three shapes a selection change takes: a container that allows several
// selections reports each element joining and leaving, while one that does not reports only the element that became the
// selection, since that implicitly deselects whatever was selected before.
func TestDecideRaisesSelection(t *testing.T) {
	c := check.New(t)

	multi := eventTree()
	c.Equal([]Raise{
		raiseProperty(5, SelectionItemIsSelectedPropertyId),
		raiseEvent(5, SelectionItem_ElementAddedToSelectionEventId),
	}, DecideRaises(eventTree(), multi, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateSelected},
	}))

	deselected := eventTree()
	deselected.Node(5).Selected = false
	c.Equal([]Raise{
		raiseProperty(5, SelectionItemIsSelectedPropertyId),
		raiseEvent(5, SelectionItem_ElementRemovedFromSelectionEventId),
	}, DecideRaises(eventTree(), deselected, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateSelected},
	}))

	single := eventTree()
	single.Node(4).Multiselectable = false
	c.Equal([]Raise{
		raiseProperty(5, SelectionItemIsSelectedPropertyId),
		raiseEvent(5, SelectionItem_ElementSelectedEventId),
	}, DecideRaises(eventTree(), single, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateSelected},
	}))

	singleOff := eventTree()
	singleOff.Node(4).Multiselectable = false
	singleOff.Node(5).Selected = false
	c.Equal([]Raise{raiseProperty(5, SelectionItemIsSelectedPropertyId)},
		DecideRaises(eventTree(), singleOff, []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateSelected},
		}))
}

// TestDecideRaisesSelectionNested verifies that a nested row's selection is judged by the container the provider
// reports — the nearest ancestor supporting the Selection pattern — rather than by its immediate parent. A child row's
// parent is another row, which is never multiselectable, so asking the parent would report a row joining a multiple
// selection with ElementSelected, which tells the client everything else was just deselected.
func TestDecideRaisesSelectionNested(t *testing.T) {
	c := check.New(t)
	nested := func() *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Focused: true, Children: []accessibility.NodeID{2}},
			&accessibility.Node{
				ID: 2, Role: role.Tree, Multiselectable: true, RowCount: 2,
				Children: []accessibility.NodeID{3},
			},
			&accessibility.Node{
				ID: 3, Role: role.Row, RowIndex: 0, Selectable: true, Selected: true, Expandable: true,
				Expanded: true, Children: []accessibility.NodeID{4},
			},
			&accessibility.Node{ID: 4, Role: role.Row, RowIndex: 1, Selectable: true, Selected: true},
		)
	}
	c.Equal([]Raise{
		raiseProperty(4, SelectionItemIsSelectedPropertyId),
		raiseEvent(4, SelectionItem_ElementAddedToSelectionEventId),
	}, DecideRaises(nested(), nested(), []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 4, State: accessibility.StateSelected},
	}))

	// The same row in a table that holds one selection at a time reports becoming the selection instead.
	single := nested()
	single.Node(2).Multiselectable = false
	c.Equal([]Raise{
		raiseProperty(4, SelectionItemIsSelectedPropertyId),
		raiseEvent(4, SelectionItem_ElementSelectedEventId),
	}, DecideRaises(nested(), single, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 4, State: accessibility.StateSelected},
	}))
}

// TestDecideRaisesRadioButton verifies that a radio button reports being checked as being selected. It has no Toggle
// pattern, so a toggle-state property change would name a property the element does not support.
func TestDecideRaisesRadioButton(t *testing.T) {
	c := check.New(t)
	c.Equal([]Raise{
		raiseProperty(7, SelectionItemIsSelectedPropertyId),
		raiseEvent(7, SelectionItem_ElementSelectedEventId),
	}, DecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 7, State: accessibility.StateChecked},
	}))

	unchecked := eventTree()
	unchecked.Node(7).Checked = checkenum.Off
	c.Equal([]Raise{raiseProperty(7, SelectionItemIsSelectedPropertyId)},
		DecideRaises(eventTree(), unchecked, []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: 7, State: accessibility.StateChecked},
		}))

	// Its Selected flag is not what decides it either: ISelectionItemProvider::get_IsSelected answers from the check
	// state for a radio button, so a selected-but-unchecked one must not be reported as having become the selection —
	// the client would be told something the provider then denies. The builder produces no such node today; the two
	// answers are kept in step here rather than relying on that.
	odd := eventTree()
	odd.Node(7).Checked = checkenum.Off
	odd.Node(7).Selected = true
	c.Equal([]Raise{raiseProperty(7, SelectionItemIsSelectedPropertyId)},
		DecideRaises(eventTree(), odd, []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: 7, State: accessibility.StateSelected},
		}))
}

// TestDecideRaisesBounds verifies that only the focused node and the root report a new bounding rectangle. Resizing
// a window moves everything in it, and a client that wanted every rectangle would ask for them.
func TestDecideRaisesBounds(t *testing.T) {
	c := check.New(t)
	c.Equal([]Raise{
		raiseProperty(1, BoundingRectanglePropertyId),
		raiseProperty(2, BoundingRectanglePropertyId),
	}, DecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.BoundsChanged, Node: 1},
		{Kind: accessibility.BoundsChanged, Node: 2},
		{Kind: accessibility.BoundsChanged, Node: 3},
		{Kind: accessibility.BoundsChanged, Node: 8},
	}))
}

// TestDecideRaisesIgnored verifies that a node the snapshot marks Ignored never appears, since it has no provider
// for an event to be raised on.
func TestDecideRaisesIgnored(t *testing.T) {
	c := check.New(t)
	c.Nil(DecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.NameChanged, Node: 9},
		{Kind: accessibility.StateChanged, Node: 9, State: accessibility.StateDisabled},
		{Kind: accessibility.BoundsChanged, Node: 9},
	}))
}

// TestDecideRaisesAdded verifies that a new node reports itself as added, and that it says nothing once its parent
// has already reported all of its children invalidated — which is what the diff produces for a real addition, since the
// parent's list of children changed too.
func TestDecideRaisesAdded(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(4).Children = append(cur.Node(4).Children, 11)
	cur.Nodes[11] = &accessibility.Node{ID: 11, Parent: 4, Role: role.ListItem, Selectable: true}

	c.Equal([]Raise{raiseStructure(11, 11, StructureChangeType_ChildAdded)},
		DecideRaises(old, cur, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 11}}))

	c.Equal([]Raise{raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated)},
		DecideRaises(old, cur, accessibility.Diff(old, cur)))
}

// TestDecideRaisesAddedSubtree verifies that a whole subtree arriving at once is reported as one invalidation of the
// parent it hangs off and nothing else.
//
// The diff invalidates that parent and then reports every node of the subtree as added, so looking only at a new node's
// immediate parent drops the subtree's top node — whose parent is the invalidated one — and then reports every node
// beneath it, whose parents are the new nodes themselves. A client answers the invalidation by reading the children
// again, so those are events it would only make it do the same work over.
func TestDecideRaisesAddedSubtree(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(1).Children = append(cur.Node(1).Children, 11)
	cur.Nodes[11] = &accessibility.Node{ID: 11, Parent: 1, Role: role.Group, Children: []accessibility.NodeID{12, 13}}
	cur.Nodes[12] = &accessibility.Node{ID: 12, Parent: 11, Role: role.Button, Name: "One"}
	cur.Nodes[13] = &accessibility.Node{ID: 13, Parent: 11, Role: role.Button, Name: "Two"}

	c.Equal([]Raise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		DecideRaises(old, cur, accessibility.Diff(old, cur)))

	// A node added under an invalidated ancestor several levels up is dropped just the same, which is what the walk up
	// the chain is for.
	c.Equal([]Raise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		DecideRaises(old, cur, []accessibility.Event{
			{Kind: accessibility.ChildrenChanged, Node: 1},
			{Kind: accessibility.NodeAdded, Node: 12},
		}))
}

// TestDecideRaisesRemoved verifies that a departed node reports its removal on the parent it left, since it no
// longer exists to raise anything itself, and that the disconnect releasing its provider happens either way.
func TestDecideRaisesRemoved(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(4).Children = []accessibility.NodeID{5}
	delete(cur.Nodes, 6)

	c.Equal([]Raise{
		raiseStructure(4, 6, StructureChangeType_ChildRemoved),
		raiseDisconnect(6),
	}, DecideRaises(old, cur, []accessibility.Event{{Kind: accessibility.NodeRemoved, Node: 6}}))

	c.Equal([]Raise{
		raiseDisconnect(6),
		raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated),
	}, DecideRaises(old, cur, accessibility.Diff(old, cur)))
}

// TestDecideRaisesRemovedSubtree verifies that removing a whole subtree reports the invalidation once on the
// surviving parent and disconnects every node that left, rather than trying to raise a removal on a parent that is
// gone too.
func TestDecideRaisesRemovedSubtree(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(1).Children = []accessibility.NodeID{2, 3, 7, 8, 9}
	delete(cur.Nodes, 4)
	delete(cur.Nodes, 5)
	delete(cur.Nodes, 6)

	c.Equal([]Raise{
		raiseDisconnect(4),
		raiseDisconnect(5),
		raiseDisconnect(6),
		raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated),
	}, DecideRaises(old, cur, accessibility.Diff(old, cur)))

	// A removal under an ancestor the same batch invalidated higher up is dropped too. A diff never produces that on
	// its own — a real removal changes the immediate parent's list of children as well — so the events are written out
	// here to reach the walk up the chain that removals share with additions.
	gone := eventTree()
	gone.Node(4).Children = []accessibility.NodeID{5}
	delete(gone.Nodes, 6)
	c.Equal([]Raise{
		raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated),
		raiseDisconnect(6),
	}, DecideRaises(old, gone, []accessibility.Event{
		{Kind: accessibility.ChildrenChanged, Node: 1},
		{Kind: accessibility.NodeRemoved, Node: 6},
	}))
}

// TestDecideRaisesIgnoredFlip verifies that a node whose Ignored flag flips has the nearest unignored parent told to
// read its children again.
//
// Nothing else says it happened: an ignored node stays in the snapshot so that hit testing and coordinate clipping go
// on working, so no list of children changed, while to a client the node has just joined or left the tree entirely. It
// reaches a real window — ScrollBar.ProvideAccessibility ignores a scroll bar with nothing to scroll — and a client
// that was told nothing would keep a hierarchy that permanently disagrees with what Navigate answers.
func TestDecideRaisesIgnoredFlip(t *testing.T) {
	c := check.New(t)
	shown := eventTree()
	hidden := eventTree()
	hidden.Node(3).Ignored = true

	// The check box directly under the window leaves the tree a client sees, and then comes back.
	c.Equal([]Raise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		DecideRaises(shown, hidden, accessibility.Diff(shown, hidden)))
	c.Equal([]Raise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		DecideRaises(hidden, shown, accessibility.Diff(hidden, shown)))

	// Node 10 sits under an ignored group, so the invalidation lands on the window rather than on a parent with no
	// provider to raise it on.
	buried := eventTree()
	buried.Node(10).Ignored = true
	c.Equal([]Raise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		DecideRaises(shown, buried, accessibility.Diff(shown, buried)))

	// Having said the parent's children are all invalid, the additions and removals under it are dropped, exactly as
	// they are for a list of children that changed.
	c.Equal([]Raise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		DecideRaises(shown, hidden, []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: 3, State: accessibility.StateIgnored, Old: "false", New: "true"},
			{Kind: accessibility.NodeAdded, Node: 2},
		}))
}

// menuTree builds a window with a button, optionally with an open menu or a tooltip hanging off the root the way the
// root panel really holds them: both are children of the window itself rather than of whatever they belong to.
func menuTree(extra ...*accessibility.Node) *accessibility.Tree {
	nodes := []*accessibility.Node{
		{ID: 1, Role: role.Window, Name: "Window", Focused: true},
		{ID: 2, Role: role.Button, Name: "File"},
	}
	children := []accessibility.NodeID{2}
	for _, n := range extra {
		nodes = append(nodes, n)
		children = append(children, n.ID)
	}
	nodes[0].Children = children
	return newTestTree(1, 2, nodes...)
}

// TestDecideRaisesMenu verifies that a menu appearing and disappearing raises the two events UI Automation has for
// exactly that, over and above whatever structure change the node asks for. They are what tell a screen reader to enter
// and leave menu mode; a structure change and a focus move say nothing of the kind.
func TestDecideRaisesMenu(t *testing.T) {
	c := check.New(t)
	menu := func() []*accessibility.Node {
		return []*accessibility.Node{
			{ID: 3, Role: role.Menu, Children: []accessibility.NodeID{4}},
			{ID: 4, Role: role.MenuItem, Name: "Open"},
		}
	}
	closed := menuTree()
	open := menuTree(menu()...)

	c.Equal([]Raise{
		raiseStructure(3, 3, StructureChangeType_ChildAdded),
		raiseEvent(3, MenuOpenedEventId),
	}, DecideRaises(closed, open, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 3}}))

	// The menu closing is raised on the window: the menu itself has left the tree, so there is no provider of its own
	// left for a client to be told about it through.
	c.Equal([]Raise{
		raiseEvent(1, MenuClosedEventId),
		raiseStructure(1, 3, StructureChangeType_ChildRemoved),
		raiseDisconnect(3),
	}, DecideRaises(open, closed, []accessibility.Event{{Kind: accessibility.NodeRemoved, Node: 3}}))

	// The same holds for the real batch a diff produces, where the structure changes collapse into one invalidation of
	// the window but the menu events do not: a client answers an invalidation by reading the children again, which
	// tells it nothing about a menu having opened.
	c.Equal([]Raise{
		raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated),
		raiseEvent(3, MenuOpenedEventId),
	}, DecideRaises(closed, open, accessibility.Diff(closed, open)))

	// The menu item inside it is not a menu, and neither is anything else that comes and goes.
	c.Equal([]Raise{raiseStructure(4, 4, StructureChangeType_ChildAdded)},
		DecideRaises(closed, open, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 4}}))
}

// TestDecideRaisesTooltip verifies that a tooltip appearing is announced. Nothing else says it happened: a tip takes
// no focus, names nothing and changes no property, so without the event a client never mentions it.
func TestDecideRaisesTooltip(t *testing.T) {
	c := check.New(t)
	without := menuTree()
	with := menuTree(&accessibility.Node{ID: 3, Role: role.Tooltip, Name: "Open a file"})

	c.Equal([]Raise{
		raiseStructure(3, 3, StructureChangeType_ChildAdded),
		raiseEvent(3, ToolTipOpenedEventId),
	}, DecideRaises(without, with, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 3}}))

	c.Equal([]Raise{
		raiseEvent(1, ToolTipClosedEventId),
		raiseStructure(1, 3, StructureChangeType_ChildRemoved),
		raiseDisconnect(3),
	}, DecideRaises(with, without, []accessibility.Event{{Kind: accessibility.NodeRemoved, Node: 3}}))

	// An ignored node has no provider, so nothing is raised for it at all.
	hidden := menuTree(&accessibility.Node{ID: 3, Role: role.Tooltip, Name: "Open a file", Ignored: true})
	c.Nil(DecideRaises(without, hidden, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 3}}))
}

// TestDecideRaisesRole verifies that a node whose role changed reports its control type as changed. A live node
// really can change role — a label becomes an image when its text is swapped for a drawable, a button becomes a toggle
// button when it is made sticky — and the control type is what a client derives the spoken kind of the element, and the
// patterns it bothers looking for, from.
//
// The localized control type is deliberately not reported alongside it: this package never answers that property, since
// UI Automation has a localized name for every control type and ours would be in English only, so it derives that one
// from the control type it has just been told about.
func TestDecideRaisesRole(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(3).Role = role.ToggleButton

	events := accessibility.Diff(old, cur)
	c.Equal(1, len(events))
	c.Equal(accessibility.RoleChanged, events[0].Kind)
	c.Equal([]Raise{raiseProperty(3, ControlTypePropertyId)}, DecideRaises(old, cur, events))

	// An ignored node has no provider, so its role change is dropped like everything else about it.
	c.Nil(DecideRaises(old, cur, []accessibility.Event{
		{Kind: accessibility.RoleChanged, Node: 9, Old: "group", New: "tool-bar"},
	}))
}

// TestDecideRaisesWindowActivation verifies that a window becoming active points the client back at whatever inside
// it has the focus, and that a window losing it says nothing at all.
func TestDecideRaisesWindowActivation(t *testing.T) {
	c := check.New(t)
	c.Equal([]Raise{raiseEvent(2, AutomationFocusChangedEventId)},
		DecideRaises(eventTree(), eventTree(), []accessibility.Event{
			{Kind: accessibility.WindowActivated, Node: 1},
		}))

	deactivated := eventTree()
	deactivated.Node(1).Focused = false
	c.Nil(DecideRaises(eventTree(), deactivated, []accessibility.Event{
		{Kind: accessibility.WindowDeactivated, Node: 1},
	}))
}

// TestDecideRaisesDeterminism verifies that the same trees and events always produce the same calls in the same
// order, which the map iteration inside the decision logic could otherwise break.
func TestDecideRaisesDeterminism(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(2).Value = "help"
	cur.Node(2).Text = &accessibility.TextInfo{Text: "help", SelStart: 4, SelEnd: 4}
	cur.Node(3).Checked = checkenum.Off
	cur.Node(5).Selected = false
	cur.Node(8).Number = 6
	cur.Node(1).Children = []accessibility.NodeID{2, 3, 4, 7, 8}
	delete(cur.Nodes, 9)
	delete(cur.Nodes, 10)
	events := accessibility.Diff(old, cur)
	c.True(len(events) > 4)
	first := DecideRaises(old, cur, events)
	c.True(len(first) > 4)
	for range 8 {
		c.Equal(first, DecideRaises(old, cur, events))
	}
}

// TestRaiseStrings verifies that the diagnostic strings name each kind and carry the field that matters for it,
// since a failing provider test is read through them.
func TestRaiseStrings(t *testing.T) {
	c := check.New(t)
	c.Equal("event", RaiseEvent.String())
	c.Equal("property", RaiseProperty.String())
	c.Equal("structure", RaiseStructure.String())
	c.Equal("disconnect", RaiseDisconnect.String())
	c.Equal("RaiseKind(9)", RaiseKind(9).String())
	c.Equal("event{node:2,event:20005}", raiseEvent(2, AutomationFocusChangedEventId).String())
	c.Equal("property{node:3,property:30005}", raiseProperty(3, NamePropertyId).String())
	c.Equal("structure{node:4,change:1,child:6}",
		raiseStructure(4, 6, StructureChangeType_ChildRemoved).String())
	c.Equal("structure{node:4,change:2}", raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated).String())
	c.Equal("disconnect{node:6}", raiseDisconnect(6).String())
}
