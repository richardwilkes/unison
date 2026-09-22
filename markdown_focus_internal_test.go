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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
)

// These tests cover the keyboard focus a Markdown takes for an assistive technology's sake — see Panel.axTakesFocus:
// that it takes it only while one is being served, and only on the platforms whose screen readers start from the focus;
// that a window in which nothing held the focus hands it to the document as support starts, while a focus that was
// already held is left alone; and that a keyboard user nothing is listening to never finds the document in the tab
// order. A session owns most of the package's mutable globals while it runs, so none of these may call t.Parallel.

// markdownFocusContent is the document the tests below show. What it says does not matter; that it is a Markdown does.
const markdownFocusContent = "# Title\n\nSome body text.\n"

// TestMarkdownTakesFocusOnlyWhileAccessibilityIsActive shows a window holding nothing but a document, which is the
// window a screen reader was stranded at the edge of: with nothing inside it holding the focus, Narrator's cursor
// stayed on the window itself. Once an assistive technology is being served, the document takes the focus and the
// description reports it there; once none is, the document is no longer focusable and the next move of the focus
// leaves it.
func TestMarkdownTakesFocusOnlyWhileAccessibilityIsActive(t *testing.T) {
	c := check.New(t)
	var md *Markdown
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			md = NewMarkdown(false)
			md.SetContent(markdownFocusContent, 400)
			scroller := NewScrollPanel()
			scroller.SetContent(md, behavior.Fill, behavior.Fill)
			wnd = axNewTestWindow(t, "document focus", geom.NewRect(10, 10, 500, 500), scroller)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	focusable := func() bool {
		var f bool
		screen.Do(func() { f = md.Focusable() })
		return f
	}
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}
	c.False(focusable(), "with no assistive technology, a document is not focusable")
	c.True(focus() == nil, "and a window holding nothing but one has no focus")

	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	node := screen.AccessibilityNodeFor(md)
	c.NotNil(node)
	if tree == nil || node == nil {
		return
	}
	if axReadersFollowFocus {
		c.True(focusable(), "while one is being served, the document takes the focus")
		c.True(focus().Is(md), "a window in which nothing held the focus hands it to the document as support starts")
		c.Equal(node.ID, tree.Focus, "and the description reports the focus there")
		c.True(node.Focusable, "the node says it can take the focus")
		c.True(node.Actions.Has(accessibility.Focus), "and offers to")
	} else {
		c.False(focusable(), "on a platform whose screen reader does not start from the focus, nothing changes")
		c.True(focus() == nil, "so the window still has no focus")
		c.Equal(accessibility.NodeID(0), tree.Focus, "and the description reports none")
		c.False(node.Focusable)
		c.False(node.Actions.Has(accessibility.Focus))
	}

	screen.Do(deactivateAccessibility)
	c.False(IsAccessibilityActive())
	c.False(focusable(), "once none is being served, the document is not focusable again")
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus() == nil, "and the next move of the focus leaves it, with nothing else to land on")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownJoinsTabOrderOnlyWhileAccessibilityIsActive shows a document between two text fields. With no assistive
// technology, Tab moves from one field to the other and never lands on the document, as it never has. Once one is being
// served, on the platforms where a document takes the focus, the document is a tab stop between the two — while the
// field that held the focus as support started keeps it, since only a window with no focus at all is given one.
func TestMarkdownJoinsTabOrderOnlyWhileAccessibilityIsActive(t *testing.T) {
	c := check.New(t)
	var first, second *Field
	var md *Markdown
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			first = NewField()
			md = NewMarkdown(false)
			md.SetContent(markdownFocusContent, 400)
			second = NewField()
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(first)
			column.AddChild(md)
			column.AddChild(second)
			wnd = axNewTestWindow(t, "document tab order", geom.NewRect(10, 10, 500, 500), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}
	focusFirst := func() {
		screen.Do(func() { first.RequestFocus() })
		c.True(focus().Is(first))
	}
	focusFirst()
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(second), "with no assistive technology, the tab order skips the document")

	focusFirst()
	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())
	c.True(focus().Is(first), "support starting leaves a focus that was already held where it is")
	screen.KeyPress(KeyTab, mod.None)
	if axReadersFollowFocus {
		c.True(focus().Is(md), "while one is being served, the document is a tab stop")
		screen.KeyPress(KeyTab, mod.None)
	}
	c.True(focus().Is(second), "and the field after it is the next stop")

	screen.Do(deactivateAccessibility)
	c.False(IsAccessibilityActive())
	focusFirst()
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(second), "once none is being served, the tab order skips the document again")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
