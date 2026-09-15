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
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests cover the decisions the snapshot builder makes on its own: what it does with the children of a widget
// that described them itself, how the pieces of a name are joined, which sibling counts as a label, what an open menu
// may point the focus at, and the layout that has to have happened before any of it is read. A session owns most of the
// package's mutable globals while it runs, so none of these may call t.Parallel.

// axTestHeading is a piece of static text holding something an assistive technology has to be able to reach in its own
// right, which is what a markdown heading with a link in it is: it asks for its children to be described rather than
// letting them be folded into its name. describeChildren says whether it asks, so that a heading that does not is the
// same widget in every other respect.
type axTestHeading struct {
	Panel
	// roleAfterDescribing, when it is not role.Auto, is what the widget decides it is once it has asked, which is the
	// one way the role can move without a callback having done it.
	roleAfterDescribing role.Enum
	describeChildren    bool
}

// newAXTestHeading creates a heading laid out in a single column.
func newAXTestHeading(describeChildren bool) *axTestHeading {
	h := &axTestHeading{describeChildren: describeChildren}
	h.Self = h
	h.SetLayout(&FlexLayout{Columns: 1})
	h.Accessibility.Role = role.Heading
	return h
}

// ProvideAccessibility asks for the children to be described, exactly as markdownHeading does for a heading holding a
// link or an image.
func (h *axTestHeading) ProvideAccessibility(b *AccessibilityBuilder) {
	if h.describeChildren {
		b.DescribeChildren()
	}
	if h.roleAfterDescribing != role.Auto {
		b.Node().Role = h.roleAfterDescribing
	}
}

// axTestLabel returns a label showing the given text, for building the content of a test window.
func axTestLabel(text string) *Label {
	label := NewLabel()
	label.SetTitle(text)
	return label
}

// TestAccessibilityDescribedChildrenAreNotDescribedTwice covers the two halves of one disagreement: whether a panel's
// children are described was decided from the role the node held after Accessibility.Callback had run, while
// AccessibilityBuilder.DescribeChildren decided from the role it held while the widget was describing itself.
//
// A callback is entitled to move the role either way. Moving it off Heading or Label made the builder describe the
// children a second time, which listed every one of them beneath its parent twice — the parent's own child list, the
// index of a child within it, and what Tree.PositionInSet counts out of it are all then wrong. Moving it onto either
// made the builder skip children nothing else was going to describe, which took the whole subtree out of the tree.
func TestAccessibilityDescribedChildrenAreNotDescribedTwice(t *testing.T) {
	c := check.New(t)
	var described, plain, switched *axTestHeading
	var promoted *Panel
	var promotedChild *Button
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 500, Height: 400},
		StartupFinishedCallback(func() {
			// A heading that describes its own children and whose callback then decides it is a plain group. The name
			// was gathered from the text while it was still static text, so the children must be listed exactly once.
			described = newAXTestHeading(true)
			described.AddChild(axTestLabel("Described"))
			described.Accessibility.Callback = func(n *accessibility.Node) { n.Role = role.Group }

			// The same widget without the callback, which is the ordinary case and must come out the same way.
			plain = newAXTestHeading(true)
			plain.AddChild(axTestLabel("Plain"))

			// A widget that asks and then decides for itself that it is something else, which is the case no role read
			// back afterwards can tell from one that never asked at all.
			switched = newAXTestHeading(true)
			switched.roleAfterDescribing = role.Group
			switched.AddChild(axTestLabel("Switched"))

			// The mirror case: a group whose callback makes it static text only after the name has been resolved from
			// it. Nothing folded the child into the name, so the child is still the only way to reach the button.
			promoted = NewPanel()
			promoted.SetLayout(&FlexLayout{Columns: 1})
			promotedChild = NewButton()
			promotedChild.SetTitle("Reachable")
			promoted.AddChild(promotedChild)
			promoted.Accessibility.Callback = func(n *accessibility.Node) { n.Role = role.Heading }

			content := NewPanel()
			content.SetLayout(&FlexLayout{Columns: 1, HSpacing: StdHSpacing, VSpacing: StdVSpacing})
			content.AddChild(described)
			content.AddChild(plain)
			content.AddChild(switched)
			content.AddChild(promoted)
			wnd = axNewTestWindow(t, "described children", geom.NewRect(10, 10, 400, 300), content)
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	for _, one := range []struct {
		heading  *axTestHeading
		name     string
		wantName string
	}{
		{name: "Described", heading: described, wantName: "Described"},
		{name: "Plain", heading: plain, wantName: "Plain"},
		// This one stopped being static text before the name was resolved, so it never had one gathered for it.
		{name: "Switched", heading: switched},
	} {
		node := screen.AccessibilityNodeFor(one.heading)
		c.True(node != nil, one.name)
		if node == nil {
			continue
		}
		c.Equal(one.wantName, node.Name, "%s: the name is gathered from the whole of the text", one.name)
		c.Equal(1, len(node.Children), "%s: the one child must be listed once, got %v", one.name, node.Children)
		if len(node.Children) == 1 {
			pos, size := tree.PositionInSet(node.Children[0])
			c.Equal(1, pos, "%s: the only child is the first of one", one.name)
			c.Equal(1, size, "%s: the only child is the first of one", one.name)
		}
	}

	promotedNode := screen.AccessibilityNodeFor(promoted)
	c.True(promotedNode != nil)
	buttonNode := screen.AccessibilityNodeFor(promotedChild)
	c.True(buttonNode != nil, "a callback that makes a group static text must not take the subtree with it")
	if promotedNode != nil && buttonNode != nil {
		c.Equal(role.Heading, promotedNode.Role)
		c.Equal([]accessibility.NodeID{buttonNode.ID}, promotedNode.Children)
		c.Equal(promotedNode.ID, buttonNode.Parent)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAXLabelTextKeepsTheSpacingItIsGiven covers how the pieces of a name are joined. Text that has been broken into
// more than one panel keeps its own spacing — markdown splits "A linked heading" into "A ", the link, and " heading" —
// so a separator added without regard to that doubles every space and leaves a name a screen reader reads with a
// stutter in it.
func TestAXLabelTextKeepsTheSpacingItIsGiven(t *testing.T) {
	c := check.New(t)
	for _, one := range []struct {
		name   string
		want   string
		pieces []string
	}{
		{name: "no spacing of its own", pieces: []string{"First", "Second"}, want: "First Second"},
		{name: "trailing space", pieces: []string{"A ", "linked", " heading"}, want: "A linked heading"},
		{name: "space on both sides", pieces: []string{"A ", " linked"}, want: "A  linked"},
		{name: "one piece", pieces: []string{"Alone"}, want: "Alone"},
		{name: "leading space kept", pieces: []string{" Indented"}, want: " Indented"},
	} {
		parent := NewPanel()
		for _, piece := range one.pieces {
			child := NewPanel()
			child.Accessibility.Name = piece
			parent.AddChild(child)
		}
		c.Equal(one.want, axLabelText(parent), one.name)
	}
}

// TestAccessibilitySiblingLabelRefusesALink pins which sibling the "label, then the thing it labels" convention will
// take a name from. NewLink hands back a *Label that describes itself with role.Link, so a link laid out above a field
// looks exactly like a caption for it: without the check the field is announced by the hyperlink's text and hands an
// assistive technology the link as its LabeledBy, while the link itself stays something a person is expected to press.
func TestAccessibilitySiblingLabelRefusesALink(t *testing.T) {
	c := check.New(t)
	var afterLink, afterLabel *Field
	var link *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 500, Height: 400},
		StartupFinishedCallback(func() {
			link = NewLink("Read more", "", "https://example.com/x", nil, nil)
			afterLink = NewField()
			label := axTestLabel("Name:")
			afterLabel = NewField()
			content := NewPanel()
			content.SetLayout(&FlexLayout{Columns: 1, HSpacing: StdHSpacing, VSpacing: StdVSpacing})
			content.AddChild(link)
			content.AddChild(afterLink)
			content.AddChild(label)
			content.AddChild(afterLabel)
			wnd = axNewTestWindow(t, "sibling labels", geom.NewRect(10, 10, 400, 300), content)
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	linkNode := screen.AccessibilityNodeFor(link)
	c.True(linkNode != nil)
	if linkNode != nil {
		c.Equal(role.Link, linkNode.Role, "a link is a link rather than static text")
	}
	fieldNode := screen.AccessibilityNodeFor(afterLink)
	c.True(fieldNode != nil)
	if fieldNode != nil {
		c.Equal("", fieldNode.Name, "a field must not be named after the link above it")
		c.Equal(0, len(fieldNode.LabeledBy), "nor handed the link as what labels it")
	}

	// The convention itself still works, which is the thing the check must not have cost.
	labeledNode := screen.AccessibilityNodeFor(afterLabel)
	c.True(labeledNode != nil)
	if labeledNode != nil {
		c.Equal("Name", labeledNode.Name, "an ordinary label still names the control beside it")
		c.Equal(1, len(labeledNode.LabeledBy))
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityMenuSeparatorIsNeverTheFocus covers what an open menu may point the focus at. An item is highlighted
// by the pointer merely passing over it, and a separator is highlighted along with everything else the pointer crosses,
// so without skipping it the window reported the focus on a divider — published as focusable besides — every time the
// pointer passed over one. What is left holding it is the menu, since that is where every key goes while one is open.
func TestAccessibilityMenuSeparatorIsNeverTheFocus(t *testing.T) {
	c := check.New(t)
	const (
		menuID = UserBaseID + 700
		itemID = UserBaseID + 701
	)
	var wnd *Window
	var field *Field
	var edit Menu
	screen := startHeadlessTest(t, HeadlessConfig{Width: 500, Height: 400},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "separators", geom.NewRect(10, 10, 400, 300))
			if wnd == nil {
				return
			}
			field = NewField()
			wnd.Content().SetLayout(&FlexLayout{Columns: 1, HSpacing: StdHSpacing, VSpacing: StdVSpacing})
			wnd.Content().AddChild(field)
			factory := DefaultMenuFactory()
			factory.BarForWindow(wnd, func(bar Menu) {
				f := bar.Factory()
				edit = f.NewMenu(menuID, "Edit", nil)
				edit.InsertItem(-1, f.NewItem(itemID, "Cut", KeyBinding{}, nil, nil))
				edit.InsertSeparator(-1, false)
				bar.InsertMenu(-1, edit)
			})
			wnd.ToFront()
			field.RequestFocus()
		}))
	c.NotNil(wnd)
	c.NotNil(edit)
	screen.Sync() // the bar has to have been laid out before there is anywhere to aim at

	var title *Panel
	screen.Do(func() {
		if panels := headlessMenuItemPanels(wnd.root.menuBarPanel); len(panels) != 0 {
			title = panels[0]
		}
	})
	c.NotNil(title, "the menu bar should carry the one title that was added to it")
	if title == nil {
		return
	}
	screen.Click(screen.PanelCenter(title))

	var separator *Panel
	screen.Do(func() {
		if m, ok := edit.(*menu); ok && m.popupPanel != nil {
			if panels := headlessMenuItemPanels(m.popupPanel); len(panels) == 2 {
				separator = panels[1]
			}
		}
	})
	c.NotNil(separator, "clicking the title should have opened the menu with its item and its separator")
	if separator == nil {
		return
	}

	screen.MouseMove(screen.PanelCenter(separator), mod.None)
	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	separatorNode := screen.AccessibilityNodeFor(separator)
	c.True(separatorNode != nil, "the separator is still described as one")
	if separatorNode != nil {
		c.Equal(role.Separator, separatorNode.Role)
		c.False(separatorNode.Focused, "a separator must not report that it holds the focus")
		c.False(separatorNode.Focusable, "nor that it could take it")
		c.NotEqual(separatorNode.ID, tree.Focus, "and the window must not point the focus at it")
	}
	// The focus goes to the menu the pointer is in rather than to the control that holds the keyboard focus: every key
	// goes to the open menu, so saying the person is back in the field would both announce the wrong thing and offer
	// the keys of a control that is not the one they reach. See axSnapshot.openMenuNode.
	var menuNode *accessibility.Node
	tree.Walk(func(n *accessibility.Node) bool {
		if n.Role == role.Menu {
			menuNode = n
			return false
		}
		return true
	})
	c.True(menuNode != nil)
	if menuNode != nil {
		c.Equal(menuNode.ID, tree.Focus, "the open menu holds the focus while nothing in it may be pointed at")
		c.True(menuNode.Focused)
	}
	fieldNode := screen.AccessibilityNodeFor(field)
	c.True(fieldNode != nil)
	if fieldNode != nil {
		c.False(fieldNode.Focused, "the control the keyboard focus is really on must not claim it while a menu is open")
	}
	var stillFocused bool
	screen.Do(func() { stillFocused = field.Is(wnd.CurrentFocus()) })
	c.True(stillFocused, "the keyboard focus itself does not move while a menu is open")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityPublishLaysTheWindowOutFirst covers the geometry a description is built from. A tree asked for while
// a layout invalidation is still pending would otherwise report the frames the panels had before it, which is both what
// an assistive technology draws its highlight from and what the BoundsChanged events of the next diff carry.
func TestAccessibilityPublishLaysTheWindowOutFirst(t *testing.T) {
	c := check.New(t)
	var content *Panel
	var first, second *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 500, Height: 400},
		StartupFinishedCallback(func() {
			first = axTestLabel("First")
			content = NewPanel()
			content.SetLayout(&FlexLayout{Columns: 1, HSpacing: StdHSpacing, VSpacing: StdVSpacing})
			content.AddChild(first)
			wnd = axNewTestWindow(t, "pending layout", geom.NewRect(10, 10, 400, 300), content)
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	firstNode := screen.AccessibilityNodeFor(first)
	c.True(firstNode != nil)
	if firstNode == nil {
		return
	}
	c.False(firstNode.Bounds.Empty(), "the label that was there from the start has been laid out")

	// Adding a child marks the panel as needing layout without marking the window for redraw, so nothing else is going
	// to lay the window out before the description is built. The label added here has never been given a frame.
	screen.Do(func() {
		second = axTestLabel("Second")
		content.AddChild(second)
	})
	screen.AccessibilityTree(wnd)
	secondNode := screen.AccessibilityNodeFor(second)
	c.True(secondNode != nil, "the label that was just added should have been described")
	if secondNode == nil {
		return
	}
	c.False(secondNode.Bounds.Empty(), "a description built with a layout still pending reports a frame of nothing")
	c.True(secondNode.Bounds.Y >= firstNode.Bounds.Bottom(),
		"the second label belongs below the first, got %v after %v", secondNode.Bounds, firstNode.Bounds)

	// What the window really thinks, once it has been asked to lay itself out, has to be what was published.
	var frame geom.Rect
	screen.Do(func() {
		wnd.ValidateLayout()
		frame = second.RectToRoot(second.ContentRect(true))
	})
	c.Equal(frame, secondNode.Bounds, "the published bounds must be where the panel actually is")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
