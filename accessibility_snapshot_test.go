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

// TestAccessibilitySiblingLabelReadsTheAccessibleName pins that the sibling-label convention names a control the way
// the label names itself rather than by the text the label draws. A label drawn in small caps holds its text
// upper-cased, and an application that gives such a label its Accessibility.Name in the original case expects the field
// beside it to be announced in those words, not shouted.
func TestAccessibilitySiblingLabelReadsTheAccessibleName(t *testing.T) {
	c := check.New(t)
	var field *Field
	var label *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 500, Height: 400},
		StartupFinishedCallback(func() {
			label = axTestLabel("STRENGTH (ST):")
			label.Accessibility.Name = "Strength (ST):"
			field = NewField()
			content := NewPanel()
			content.SetLayout(&FlexLayout{Columns: 2, HSpacing: StdHSpacing, VSpacing: StdVSpacing})
			content.AddChild(label)
			content.AddChild(field)
			wnd = axNewTestWindow(t, "accessible label names", geom.NewRect(10, 10, 400, 300), content)
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	fieldNode := screen.AccessibilityNodeFor(field)
	c.True(fieldNode != nil)
	labelNode := screen.AccessibilityNodeFor(label)
	c.True(labelNode != nil)
	if fieldNode != nil && labelNode != nil {
		c.Equal("Strength (ST)", fieldNode.Name, "the field is named as the label names itself, colon trimmed")
		c.Equal([]accessibility.NodeID{labelNode.ID}, fieldNode.LabeledBy)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilitySiblingLabelAcceptsACustomLabelPanel pins which panels other than a *Label the sibling-label
// convention will take a name from: one that declares role.Label and says what it is to be read as, which is how a
// custom widget that draws its own text takes part. A plain panel that merely happens to carry a name is not a caption
// for what follows it, and stays out of it.
func TestAccessibilitySiblingLabelAcceptsACustomLabelPanel(t *testing.T) {
	c := check.New(t)
	var afterCustom, afterNamedGroup *Field
	var custom *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 500, Height: 400},
		StartupFinishedCallback(func() {
			custom = NewPanel()
			custom.Accessibility.Role = role.Label
			custom.Accessibility.Name = "Weight:"
			afterCustom = NewField()
			namedGroup := NewPanel()
			namedGroup.Accessibility.Name = "Not a caption"
			afterNamedGroup = NewField()
			content := NewPanel()
			content.SetLayout(&FlexLayout{Columns: 2, HSpacing: StdHSpacing, VSpacing: StdVSpacing})
			content.AddChild(custom)
			content.AddChild(afterCustom)
			content.AddChild(namedGroup)
			content.AddChild(afterNamedGroup)
			wnd = axNewTestWindow(t, "custom labels", geom.NewRect(10, 10, 400, 300), content)
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	customNode := screen.AccessibilityNodeFor(custom)
	c.True(customNode != nil)
	fieldNode := screen.AccessibilityNodeFor(afterCustom)
	c.True(fieldNode != nil)
	if fieldNode != nil && customNode != nil {
		c.Equal("Weight", fieldNode.Name, "a panel declaring itself a label names the control beside it")
		c.Equal([]accessibility.NodeID{customNode.ID}, fieldNode.LabeledBy)
	}
	groupedNode := screen.AccessibilityNodeFor(afterNamedGroup)
	c.True(groupedNode != nil)
	if groupedNode != nil {
		c.Equal("", groupedNode.Name, "a named group is not a caption for the control after it")
		c.Equal(0, len(groupedNode.LabeledBy))
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityControlNameHonorsLabeledBy pins the order axControlName resolves a popup's name in, which is the
// order the description of the popup itself uses: an outright name, then the label it points at, then the label laid
// out before it. A popup fixed up with LabeledBy used to title its menu after whatever sibling preceded it, or after
// nothing at all, however clearly it had said which label was its own.
func TestAccessibilityControlNameHonorsLabeledBy(t *testing.T) {
	c := check.New(t)
	elsewhere := axTestLabel("Size:")
	preceding := axTestLabel("Ignored:")
	popup := NewPopupMenu[string]()
	content := NewPanel()
	content.AddChild(preceding)
	content.AddChild(popup)
	c.Equal("Ignored", axControlName(popup.AsPanel()), "with nothing else said, the preceding label names it")
	popup.Accessibility.LabeledBy = elsewhere
	c.Equal("Size", axControlName(popup.AsPanel()), "the label it points at outranks the one before it")
	popup.Accessibility.Name = "Font Size"
	c.Equal("Font Size", axControlName(popup.AsPanel()), "an outright name outranks both")
}

// TestAccessibilityMenuPanelStaysPutInTheBackground verifies that a background window takes ShowContextMenu away from a
// nameless panel with a menu and from a list's rows, without the panel becoming scaffolding while it lacks the menu.
func TestAccessibilityMenuPanelStaysPutInTheBackground(t *testing.T) {
	c := check.New(t)
	var content, plain *Panel
	var list *List[string]
	var wnd, other *Window
	noMenu := func(geom.Point) Menu { return nil }
	screen := startHeadlessTest(t, HeadlessConfig{Width: 700, Height: 500},
		StartupFinishedCallback(func() {
			content = NewPanel()
			content.SetLayout(&FlexLayout{Columns: 1})
			content.ContextMenuCallback = noMenu
			content.AddChild(axTestLabel("Inside the menu's panel"))
			list = NewList[string]()
			list.Factory = &DefaultCellFactory{Height: 20}
			list.Append("First", "Second")
			list.ContextMenuCallback = noMenu
			content.AddChild(list)
			plain = NewPanel()
			plain.SetLayout(&FlexLayout{Columns: 1})
			plain.AddChild(axTestLabel("Inside a plain panel"))
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(content)
			column.AddChild(plain)
			wnd = axNewTestWindow(t, "window-wide menu", geom.NewRect(10, 10, 300, 300), column)
			other = newHeadlessTestWindow(t, "other", geom.NewRect(350, 10, 300, 200))
		}))
	c.NotNil(wnd)
	c.NotNil(other)

	// describe returns the nodes of the menu's panel, the plain panel and the list's first row.
	describe := func() (menuPanel, plainPanel, row *accessibility.Node) {
		tree := screen.AccessibilityTree(wnd)
		c.NotNil(tree)
		menuPanel = screen.AccessibilityNodeFor(content)
		plainPanel = screen.AccessibilityNodeFor(plain)
		if listNode := screen.AccessibilityNodeFor(list); tree != nil && listNode != nil {
			for _, id := range listNode.Children {
				if child := tree.Node(id); child != nil && child.RowIndex == 0 {
					row = child
					break
				}
			}
		}
		c.NotNil(menuPanel)
		c.NotNil(plainPanel)
		c.NotNil(row)
		return menuPanel, plainPanel, row
	}

	c.True(screen.Do(func() { wnd.ToFront() }))
	menuPanel, plainPanel, row := describe()
	if menuPanel == nil || plainPanel == nil || row == nil {
		return
	}
	c.True(plainPanel.Ignored, "an anonymous group with nothing to offer is scaffolding")
	c.True(menuPanel.Actions.Has(accessibility.ShowContextMenu), "the panel offers its menu while its window is active")
	c.False(menuPanel.Ignored, "and is part of what the window holds, since it can be acted on")
	c.True(row.Actions.Has(accessibility.ShowContextMenu), "and so do the rows of a list with a menu")

	c.True(screen.Do(func() { other.ToFront() }))
	menuPanel, plainPanel, row = describe()
	if menuPanel == nil || plainPanel == nil || row == nil {
		return
	}
	c.False(menuPanel.Actions.Has(accessibility.ShowContextMenu),
		"a menu that would open in another window is not offered")
	c.False(menuPanel.Ignored, "but the panel is not looked past for that, since the menu comes back with the window")
	c.True(plainPanel.Ignored)
	c.False(row.Actions.Has(accessibility.ShowContextMenu), "nor is it offered on the rows of the list")

	c.True(screen.Do(func() { wnd.ToFront() }))
	menuPanel, _, row = describe()
	if menuPanel == nil || row == nil {
		return
	}
	c.True(menuPanel.Actions.Has(accessibility.ShowContextMenu), "the menu comes back with the window's activation")
	c.False(menuPanel.Ignored)
	c.True(row.Actions.Has(accessibility.ShowContextMenu))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityMarkdownOffersOnlyAnApplicationMenu verifies that a document offers no menu of its own and does not
// move the caret for a request made anyway, but offers a menu the application gives it and opens it at its center while
// the document is not focusable. axReadersFollowFocus is turned off so that the document stays unfocusable while
// accessibility is enabled.
func TestAccessibilityMarkdownOffersOnlyAnApplicationMenu(t *testing.T) {
	c := check.New(t)
	var md *Markdown
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 500},
		StartupFinishedCallback(func() {
			md = NewMarkdown(false)
			md.SetContent("Some words to read.\n", 300)
			wnd = axNewTestWindow(t, "no menu", geom.NewRect(10, 10, 400, 300), md)
		}))
	c.NotNil(wnd)
	saved := axReadersFollowFocus
	// Written on the UI thread, the only thread that reads it.
	defer screen.Do(func() { axReadersFollowFocus = saved })
	c.True(screen.Do(func() {
		axReadersFollowFocus = false
		wnd.ToFront()
	}))
	screen.EnableAccessibility()
	var focusable bool
	screen.Do(func() { focusable = md.Focusable() })
	c.False(focusable, "a document nothing has asked to be read is not focusable here, even while one is being served")

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(md)
	c.NotNil(node)
	if node == nil {
		return
	}
	c.False(node.Actions.Has(accessibility.ShowContextMenu), "a document offers no menu of its own")
	var carried bool
	var caret int
	screen.Do(func() {
		carried = wnd.performAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.ShowContextMenu,
			Start:  5,
			End:    5,
		})
		caret = md.caret
	})
	c.False(carried, "and a request made anyway is refused")
	c.Equal(0, caret, "without the caret being moved to the range it named")

	var asked int
	var at geom.Point
	screen.Do(func() {
		md.ContextMenuCallback = func(where geom.Point) Menu {
			asked++
			at = where
			f := DefaultMenuFactory()
			m := f.NewMenu(PopupMenuTemporaryBaseID|ContextMenuIDFlag, "", nil)
			m.InsertItem(-1, f.NewItem(-1, "Custom", KeyBinding{}, nil, func(MenuItem) {}))
			return m
		}
	})
	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(md)
	c.NotNil(node)
	if node == nil {
		return
	}
	c.True(node.Actions.Has(accessibility.ShowContextMenu), "so the document offers it")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}), "and opens it when asked")
	var center geom.Point
	screen.Do(func() { center = md.DefaultContextMenuAnchor() })
	c.Equal(1, asked, "through the application's callback, once")
	c.Equal(center, at, "in the middle of the document, since a document that is not focusable has no caret to show")
	var menus int
	screen.AccessibilityTree(wnd).Walk(func(n *accessibility.Node) bool {
		if n.Role == role.Menu {
			menus++
		}
		return true
	})
	c.Equal(1, menus, "the menu opened within the window")
	screen.KeyPress(KeyEscape, mod.None)

	screen.Do(func() { md.ContextMenuCallback = nil })
	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(md)
	c.NotNil(node)
	if node != nil {
		c.False(node.Actions.Has(accessibility.ShowContextMenu), "and stops offering it once the callback is gone")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axMenuGroupCellFactory draws each list row as a focusable, nameless group around a label, whose ContextMenuCallback
// returns nil after counting the call in asked, when that is set.
type axMenuGroupCellFactory struct {
	asked *int
}

func (axMenuGroupCellFactory) CellHeight() float32 { return 20 }

func (f axMenuGroupCellFactory) CreateCell(_ Paneler, element any, _ int, _, _ Ink, _, _ bool) Paneler {
	group := NewPanel()
	group.SetLayout(&FlexLayout{Columns: 1})
	group.SetFocusable(true)
	group.ContextMenuCallback = func(geom.Point) Menu {
		if f.asked != nil {
			*f.asked++
		}
		return nil
	}
	if text, ok := element.(string); ok {
		group.AddChild(axTestLabel(text))
	}
	return group
}

// TestAccessibilityMenuInAListRowIsNotOffered verifies that a group with a menu inside a list row is not offered the
// menu in an active or background window, is looked past in favor of its label, and has a request for its menu refused
// without its callback being asked.
func TestAccessibilityMenuInAListRowIsNotOffered(t *testing.T) {
	c := check.New(t)
	var list *List[string]
	var wnd, other *Window
	var asked int
	screen := startHeadlessTest(t, HeadlessConfig{Width: 700, Height: 500},
		StartupFinishedCallback(func() {
			list = NewList[string]()
			list.Factory = axMenuGroupCellFactory{asked: &asked}
			list.Append("First", "Second")
			wnd = axNewTestWindow(t, "grouped rows", geom.NewRect(10, 10, 300, 300), list)
			other = newHeadlessTestWindow(t, "other", geom.NewRect(350, 10, 300, 200))
		}))
	c.NotNil(wnd)
	c.NotNil(other)

	// content returns the unignored nodes beneath the list's first row, and the group's node whether ignored or not.
	content := func() (unignored []*accessibility.Node, group *accessibility.Node) {
		tree := screen.AccessibilityTree(wnd)
		c.NotNil(tree)
		listNode := screen.AccessibilityNodeFor(list)
		c.NotNil(listNode)
		if tree == nil || listNode == nil {
			return nil, nil
		}
		for _, id := range listNode.Children {
			if row := tree.Node(id); row != nil && row.RowIndex == 0 {
				for _, childID := range tree.UnignoredChildren(id) {
					unignored = append(unignored, tree.Node(childID))
				}
				if len(row.Children) == 1 {
					group = tree.Node(row.Children[0])
				}
				break
			}
		}
		c.NotNil(group)
		return unignored, group
	}
	checkRow := func(when string) {
		c.Helper()
		unignored, group := content()
		if group == nil {
			return
		}
		c.False(group.Actions.Has(accessibility.ShowContextMenu), "the group is not offered its menu %s", when)
		c.True(group.Ignored, "and is looked past %s, since nothing is left of it to act on", when)
		c.Equal(1, len(unignored), "the row's content is the label within it %s", when)
		if len(unignored) == 1 && unignored[0] != nil {
			c.Equal(role.Label, unignored[0].Role)
			c.Equal("First", unignored[0].Name)
		}
	}

	c.True(screen.Do(func() { wnd.ToFront() }))
	checkRow("while its window is active")
	c.True(screen.Do(func() { other.ToFront() }))
	checkRow("while its window is in the background")

	// A request naming the group anyway, as from a stale description, is refused.
	c.True(screen.Do(func() { wnd.ToFront() }))
	var carried bool
	screen.Do(func() {
		carried = list.PerformAccessibilityAction(accessibility.ActionRequest{
			Action: accessibility.ShowContextMenu,
			Key:    axCellPanelKey{Cell: 0},
		})
	})
	c.False(carried, "a request for the menu of something within a row is refused")
	c.Equal(0, asked, "without the group being asked for one")
	menus := 0
	if tree := screen.AccessibilityTree(wnd); tree != nil {
		tree.Walk(func(n *accessibility.Node) bool {
			if n.Role == role.Menu {
				menus++
			}
			return true
		})
	}
	c.Equal(0, menus, "and nothing opens")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
