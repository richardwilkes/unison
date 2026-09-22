// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests ask the widgets what they say about themselves and then ask them to do the things they say they can,
// covering the places where the two had drifted apart: something offered that would then be refused, something carried
// out that was reported as having done nothing, and something that changed without anything being told about it.
// Everything goes through the path a screen reader's would. A session owns most of the package's mutable globals while
// it runs, so none of these may call t.Parallel.

// axMenuItemNames returns the name of every menu item in a description, which is what an assistive technology finds
// once a menu has opened.
func axMenuItemNames(tree *accessibility.Tree) []string {
	nodes := axNodesWithRole(tree, role.MenuItem)
	names := make([]string, 0, len(nodes))
	for _, one := range nodes {
		names = append(names, one.Name)
	}
	return names
}

// TestComboFieldAccessibilityMenuNameAndContextMenu verifies three things about a combo field: the menu it opens is
// named after the field, so the choices are not left hanging off something anonymous; a field built with no options
// does not offer to expand, since nothing would come of it; and asking for the contextual menu gets the field's own
// editing commands rather than the choices, which is what a right-click there shows.
func TestComboFieldAccessibilityMenuNameAndContextMenu(t *testing.T) {
	c := check.New(t)
	ready := "Ready"
	waiting := "Waiting"
	var withOptions, withoutOptions *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 400},
		unison.StartupFinishedCallback(func() {
			label := unison.NewLabel()
			label.SetTitle("Choice:")
			withOptions = unison.NewComboField([]*string{&ready, &waiting}, &ready, nil)
			withoutOptions = unison.NewComboField(nil, nil, nil)
			wnd = newHeadlessWindow(t, "combo", geom.NewRect(10, 10, 400, 200),
				axColumn(label, withOptions, withoutOptions))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	screen.AccessibilityTree(wnd)
	empty := screen.AccessibilityNodeFor(withoutOptions)
	c.True(empty != nil)
	if empty == nil {
		return
	}
	c.False(empty.Expandable, "a combo field with nothing to choose from opens nothing")
	c.False(empty.Actions.Has(accessibility.Expand))
	c.False(empty.Actions.Has(accessibility.Collapse))
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   empty.ID,
		Action: accessibility.Expand,
	}), "expanding it cannot be carried out, since nothing would be shown")
	c.Equal(0, len(axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu)), "nothing should have opened")

	node := screen.AccessibilityNodeFor(withOptions)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.Equal(role.ComboBox, node.Role)
	c.Equal("Choice", node.Name, "the label before it names it, minus the colon")
	c.True(node.Expandable)
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}))
	tree := screen.AccessibilityTree(wnd)
	menus := axNodesWithRole(tree, role.Menu)
	c.Equal(1, len(menus), "the choices should have been shown")
	if len(menus) != 1 {
		return
	}
	c.Equal("Choice", menus[0].Name, "the menu is named after the field whose choices it holds")
	c.True(slices.Contains(axMenuItemNames(tree), "Ready"), "the options are what is in it")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Collapse,
	}))
	c.Equal(0, len(axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu)),
		"collapsing should have taken the choices away")

	// The contextual menu belongs to the field: it is the cut, copy, paste and select-all menu a right-click shows,
	// and the choices are what Expand is for.
	c.True(node.Actions.Has(accessibility.ShowContextMenu))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}))
	names := axMenuItemNames(screen.AccessibilityTree(wnd))
	c.True(slices.Contains(names, "Cut"), "the field's editing commands are what its contextual menu holds: %v", names)
	c.False(slices.Contains(names, "Ready"), "the choices are not a contextual menu: %v", names)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestInWindowMenuSeparatorOffersNoPress verifies that a separator in a menu is not described as something that can be
// pressed and refuses a press rather than reporting one as carried out: a separator is a line drawn between the things
// that can be chosen, there is nothing there for a person to click, and choosing it does nothing.
func TestInWindowMenuSeparatorOffersNoPress(t *testing.T) {
	c := check.New(t)
	const (
		menuID = unison.UserBaseID + iota
		firstID
		secondID
	)
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "separator", geom.NewRect(10, 10, 400, 300), unison.NewPanel())
			if wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(wnd, func(bar unison.Menu) {
				f := bar.Factory()
				m := f.NewMenu(menuID, "Edit", nil)
				m.InsertItem(-1, f.NewItem(firstID, "First", unison.KeyBinding{}, nil, nil))
				m.InsertSeparator(-1, false)
				m.InsertItem(-1, f.NewItem(secondID, "Second", unison.KeyBinding{}, nil, nil))
				bar.InsertMenu(-1, m)
			})
			wnd.ToFront()
		}))
	c.NotNil(wnd)
	screen.Sync()

	tree := screen.AccessibilityTree(wnd)
	title := axNamed(tree, "Edit")
	c.True(title != nil, "the bar carries the one menu that was added to it")
	if title == nil {
		return
	}
	screen.Click(axScreenPoint(screen, wnd, title))
	tree = screen.AccessibilityTree(wnd)
	separators := axNodesWithRole(tree, role.Separator)
	c.Equal(1, len(separators), "the separator in the menu is described as one")
	if len(separators) != 1 {
		return
	}
	c.False(separators[0].Actions.Has(accessibility.Press), "there is nothing in a separator to press")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   separators[0].ID,
		Action: accessibility.Press,
	}), "a press that did nothing must not be reported as carried out")

	// The items around it are unaffected: they are still what a person chooses from.
	item := axNamed(tree, "Second")
	c.True(item != nil)
	if item != nil {
		c.True(item.Actions.Has(accessibility.Press))
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPopupMenuAccessibilityItemsAddedArePublished verifies that adding an item to an empty popup menu reaches an
// assistive technology without waiting for something unrelated to redraw the window. Whether the popup opens anything
// at all is derived from what it holds, and it also decides the popup's preferred size, so the item set has to mark the
// popup for layout as well as for redraw.
func TestPopupMenuAccessibilityItemsAddedArePublished(t *testing.T) {
	c := check.New(t)
	var popup *unison.PopupMenu[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			popup = unison.NewPopupMenu[string]()
			wnd = newHeadlessWindow(t, "popup items", geom.NewRect(10, 10, 300, 200), axColumn(popup))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(popup)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.False(node.Expandable, "a popup with nothing to choose from opens nothing")

	screen.AccessibilityEvents(wnd)
	screen.Do(func() { popup.AddItem("Solo") })
	published := false
	for _, e := range screen.AccessibilityEvents(wnd) {
		if e.Kind == accessibility.StateChanged && e.Node == node.ID &&
			e.State == accessibility.StateExpandable && e.New == "true" {
			published = true
		}
	}
	c.True(published, "the item should have been described without anything else happening")

	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(popup)
	c.True(node != nil)
	if node != nil {
		c.True(node.Expandable)
		c.True(node.Actions.Has(accessibility.Expand))
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPopupMenuAccessibilityExpandRunsTheWillShowCallback verifies that opening a popup that is filled in on demand
// shows what its callback put there, and that one that is still empty afterwards reports that nothing came of it. The
// items are only there once WillShowMenuCallback has run, so nothing can be decided before it — which is also why such
// a popup is opened through the press it advertises rather than through an expansion it does not.
func TestPopupMenuAccessibilityExpandRunsTheWillShowCallback(t *testing.T) {
	c := check.New(t)
	var filled, staysEmpty *unison.PopupMenu[string]
	var wnd *unison.Window
	fills := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			filled = unison.NewPopupMenu[string]()
			filled.WillShowMenuCallback = func(p *unison.PopupMenu[string]) {
				fills++
				p.RemoveAllItems()
				p.AddItem("Fast", "Slow")
				p.SelectIndex(0)
			}
			staysEmpty = unison.NewPopupMenu[string]()
			staysEmpty.WillShowMenuCallback = func(_ *unison.PopupMenu[string]) { fills++ }
			wnd = newHeadlessWindow(t, "on demand popup", geom.NewRect(10, 10, 300, 200),
				axColumn(filled, staysEmpty))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	screen.AccessibilityTree(wnd)
	emptyNode := screen.AccessibilityNodeFor(staysEmpty)
	c.True(emptyNode != nil)
	if emptyNode == nil {
		return
	}
	// A popup that is filled in on demand holds nothing until its callback has run, so it is described as one that
	// opens nothing: it does not advertise an expansion, and an assistive technology therefore uses the press that
	// opens it, which it does advertise. The expansion is refused before it reaches the widget, and is asked of the
	// widget itself here so that what the widget makes of one is still covered — the click it queues runs the callback,
	// and a popup that is still empty afterwards opens nothing.
	c.False(emptyNode.Actions.Has(accessibility.Expand))
	c.True(emptyNode.Actions.Has(accessibility.Press))
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   emptyNode.ID,
		Action: accessibility.Expand,
	}), "an expansion the popup does not advertise is refused before it reaches the widget")
	c.True(screen.Do(func() {
		c.True(staysEmpty.PerformAccessibilityAction(accessibility.ActionRequest{Action: accessibility.Expand}))
	}))
	c.Equal(0, len(axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu)), "nothing should have opened")

	node := screen.AccessibilityNodeFor(filled)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Press,
	}), "the callback fills the popup in, so pressing it shows the choices, exactly as clicking it does")
	tree := screen.AccessibilityTree(wnd)
	c.Equal(1, len(axNodesWithRole(tree, role.Menu)), "its choices should have been shown")
	c.True(slices.Contains(axMenuItemNames(tree), "Slow"), "the choices the callback added are what is in it")
	var count int
	screen.Do(func() { count = fills })
	c.Equal(2, count, "each request runs the callback once, as a click would")

	// Now that the popup holds what its callback put there, it says so.
	node = screen.AccessibilityNodeFor(filled)
	c.True(node != nil)
	if node != nil {
		c.True(node.Expandable)
		c.True(node.Expanded)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestNumericFieldAccessibilityEmptyTextIsNotTakenAsZero verifies that a request carrying an empty text is not taken as
// a request to set the field to zero. UI Automation's value pattern sends only the text, leaving the number at its zero
// value, so a field that cannot hold zero used to end up showing whichever end of its range was nearer to it.
func TestNumericFieldAccessibilityEmptyTextIsNotTakenAsZero(t *testing.T) {
	c := check.New(t)
	f := unison.NewNumericField(5, 1, 10, strconv.Itoa, strconv.Atoi, nil)
	c.False(f.PerformAccessibilityAction(accessibility.ActionRequest{Action: accessibility.SetValue}),
		"a zero the field cannot hold says nothing it can act on")
	c.Equal("5", f.Text(), "the field must be left as it was rather than thrown to the end of its range")

	// A number the field can hold is still taken when it arrives on its own, which is how the AT-SPI value interface
	// and the UI Automation range value pattern send one.
	c.True(f.PerformAccessibilityAction(accessibility.ActionRequest{Action: accessibility.SetValue, Number: 7}))
	c.Equal("7", f.Text())

	// A field whose range holds zero takes it, since that is what the request says and there is nothing to say
	// otherwise.
	g := unison.NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
	c.True(g.PerformAccessibilityAction(accessibility.ActionRequest{Action: accessibility.SetValue}))
	c.Equal("0", g.Text())
}

// TestRadioButtonAccessibilitySelect verifies that a radio button offers to be made the selection of its group and
// does it, which is what a sticky button reported under the same role has always offered: the same request against the
// two widgets must not succeed for one and be refused for the other. A button with no group has no selection to be.
func TestRadioButtonAccessibilitySelect(t *testing.T) {
	c := check.New(t)
	var first, second, loner *unison.RadioButton
	var wnd *unison.Window
	clicks := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			first = unison.NewRadioButton()
			first.ClickAnimationTime = 0
			first.SetTitle("First")
			second = unison.NewRadioButton()
			second.ClickAnimationTime = 0
			second.SetTitle("Second")
			second.ClickCallback = func() { clicks++ }
			unison.NewGroup(first, second).Select(first)
			loner = unison.NewRadioButton()
			loner.ClickAnimationTime = 0
			loner.SetTitle("Alone")
			wnd = newHeadlessWindow(t, "radio buttons", geom.NewRect(10, 10, 300, 200),
				axColumn(first, second, loner))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(second)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.Equal(role.RadioButton, node.Role)
	c.Equal(checkenum.Off, node.Checked)
	c.True(node.Actions.Has(accessibility.Select), "a radio button in a group can be made its selection")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Select,
	}))
	var selected, wasSelected bool
	var count int
	screen.Do(func() {
		selected = second.Group().Selected(second)
		wasSelected = second.Group().Selected(first)
		count = clicks
	})
	c.True(selected, "selecting it should have made it the selection of its group")
	c.False(wasSelected, "which takes the button that was selected out of it")
	c.Equal(1, count, "the application has to be told of a selection it would have been told of from a click")
	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(second)
	c.True(node != nil)
	if node != nil {
		c.Equal(checkenum.On, node.Checked, "the check says which button of the group is selected")
	}

	lonerNode := screen.AccessibilityNodeFor(loner)
	c.True(lonerNode != nil)
	if lonerNode == nil {
		return
	}
	c.False(lonerNode.Actions.Has(accessibility.Select), "a button that belongs to no group has no selection to be")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   lonerNode.ID,
		Action: accessibility.Select,
	}))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilityContextMenuFollowsTheCaret verifies that the contextual menu an assistive technology asks for
// appears at the caret. A right-click would have put the menu under the pointer; there is none here, so it goes where
// the person's attention is, which after a backward selection made with shift+Home is the start of the selection rather
// than its end.
func TestFieldAccessibilityContextMenuFollowsTheCaret(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 400},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText("hello world")
			wnd = newHeadlessWindow(t, "caret menu", geom.NewRect(10, 10, 400, 200), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))
	c.True(screen.Do(func() {
		field.RequestFocus()
		field.SetSelectionToEnd()
	}))
	screen.KeyPress(unison.KeyHome, mod.Shift)
	var start, end int
	screen.Do(func() { start, end = field.Selection() })
	c.Equal(0, start, "shift+Home selects back to the start of the text")
	c.Equal(11, end)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(field)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}))
	menus := axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu)
	c.Equal(1, len(menus), "the field's contextual menu should have opened within the window")
	if len(menus) != 1 {
		return
	}
	var atCaret, atOtherEnd geom.Point
	screen.Do(func() {
		atCaret = field.PointToRoot(field.FromSelectionIndex(0))
		atOtherEnd = field.PointToRoot(field.FromSelectionIndex(11))
	})
	c.True(atOtherEnd.X-atCaret.X > 20,
		"the two ends of the selection have to be well apart for this to mean anything")
	c.True(xmath.Abs(menus[0].Bounds.X-atCaret.X) < xmath.Abs(menus[0].Bounds.X-atOtherEnd.X),
		"the menu goes where the caret is, not where the selection happens to end")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestListAccessibilityAddToSelectionNeedsMultipleSelection verifies that a list holding one row at a time refuses to
// add a row to its selection rather than replacing the selection and reporting that it had added to it. The rows of
// such a list do not offer the action, but PerformAccessibilityAction is exported and nothing checks what was
// advertised before dispatching.
func TestListAccessibilityAddToSelectionNeedsMultipleSelection(t *testing.T) {
	c := check.New(t)
	var list *unison.List[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			list = unison.NewList[string]()
			list.Factory = &unison.DefaultCellFactory{Height: 20}
			for i := range 3 {
				list.Append("Row " + strconv.Itoa(i))
			}
			list.SetAllowMultipleSelection(false)
			list.Select(false, 0)
			wnd = newHeadlessWindow(t, "list selection", geom.NewRect(10, 10, 300, 200), axColumn(list))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(list)
	c.True(node != nil)
	second := axNodeWithRowIndex(tree, node, 1)
	c.True(second != nil)
	if second == nil {
		return
	}
	c.False(second.Actions.Has(accessibility.AddToSelection), "a list that holds one row has nothing to add to")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   second.ID,
		Action: accessibility.AddToSelection,
	}), "adding to a selection that would be replaced instead must be refused")
	var count int
	var selected bool
	screen.Do(func() {
		count = list.Selection.Count()
		selected = list.Selection.State(0)
	})
	c.Equal(1, count, "the selection must have been left alone")
	c.True(selected)

	// Once more than one row may be selected there is something to add to, and the row says so.
	screen.Do(func() { list.SetAllowMultipleSelection(true) })
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(list)
	second = axNodeWithRowIndex(tree, node, 1)
	c.True(second != nil)
	if second == nil {
		return
	}
	c.True(second.Actions.Has(accessibility.AddToSelection))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   second.ID,
		Action: accessibility.AddToSelection,
	}))
	screen.Do(func() { count = list.Selection.Count() })
	c.Equal(2, count, "the row should have been added to the selection")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityFlatFilterHasNoHierarchy verifies that a table showing the rows a flat filter kept describes
// them as the flat list they are. Nothing is shown beneath any row while such a filter is applied, so a container that
// passed it is not a thing that opens: it used to be reported as expandable and expanded, offer to be collapsed, and
// report that it had collapsed while the rows stayed exactly as they were.
func TestTableAccessibilityFlatFilterHasNoHierarchy(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var parent *tableTestRow
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			parent = newTableTestRow("p")
			parent.SetChildren([]*tableTestRow{newTableTestRow("c0"), newTableTestRow("c1")})
			parent.SetOpen(true)
			table = axNewTable(parent, newTableTestRow("s"))
			wnd = newHeadlessWindow(t, "filtered table", geom.NewRect(10, 10, 500, 300), axColumn(table))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.Equal(role.Tree, node.Role, "a table whose rows can have children is a tree")
	row := axTableRows(tree, node)["p"]
	c.True(row != nil)
	if row == nil {
		return
	}
	c.True(row.Expandable, "the container is something that opens while the hierarchy is being shown")
	c.True(row.Actions.Has(accessibility.Collapse))

	// The filter keeps the container and the row below it, and shows them as a flat list.
	c.True(screen.Do(func() {
		table.ApplyFilter(func(r *tableTestRow) bool { return r.ID() != "p" && r.ID() != "s" })
	}))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.Equal(unison.FlatTableRoleForTest(), node.Role, "a filter that shows a flat list is not a tree")
	c.Equal(2, node.RowCount, "the rows that passed are all there is")
	row = axTableRows(tree, node)["p"]
	c.True(row != nil)
	if row == nil {
		return
	}
	c.False(row.Expandable, "nothing is shown beneath a row while a flat filter is applied")
	c.False(row.Expanded)
	c.False(row.Actions.Has(accessibility.Expand))
	c.False(row.Actions.Has(accessibility.Collapse))
	c.Equal(0, len(axNodesWithRole(tree, role.DisclosureTriangle)),
		"no row is drawn with a disclosure triangle, so none is described with one")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   row.ID,
		Action: accessibility.Collapse,
	}), "a row that cannot be opened or closed must not report that it was")
	var open bool
	screen.Do(func() { open = parent.IsOpen() })
	c.True(open, "the row's own open state is left exactly as it was")

	// Taking the filter away brings the hierarchy back.
	c.True(screen.Do(func() { table.ApplyFilter(nil) }))
	tree = screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.Equal(role.Tree, node.Role)
	row = axTableRows(tree, node)["p"]
	c.True(row != nil)
	if row != nil {
		c.True(row.Expandable)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilitySelectionCallbacksOnlyOnChange verifies that the selection an assistive technology makes tells
// the application only when it actually changed something. An assistive technology acts on the row it is already on
// constantly — landing on it, then acting on it — and an application told its selection had changed sets about whatever
// it does when that happens.
func TestTableAccessibilitySelectionCallbacksOnlyOnChange(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(3)...)
	table.Columns = append(table.Columns, unison.ColumnInfo{ID: 0, Current: 100})
	table.SyncToModel()
	changes := 0
	table.SelectionChangedCallback = func() { changes++ }
	first := table.RowFromIndex(0).ID()
	second := table.RowFromIndex(1).ID()
	act := func(key any, action accessibility.Action) bool {
		return table.PerformAccessibilityAction(accessibility.ActionRequest{Key: key, Action: action})
	}

	c.True(act(first, accessibility.Select))
	c.Equal(1, changes, "selecting a row that was not selected is a change of selection")
	c.True(act(first, accessibility.Select))
	c.Equal(1, changes, "re-selecting the row that is already the whole of the selection is not")
	c.Equal(first, table.RowFromIndex(table.FirstSelectedRowIndex()).ID())

	c.True(act(second, accessibility.AddToSelection))
	c.Equal(2, changes, "adding a row that was not selected is")
	c.True(act(second, accessibility.AddToSelection))
	c.Equal(2, changes, "a row already in the selection cannot be added to it again")
	c.Equal(2, table.SelectionCount())

	c.True(act(second, accessibility.RemoveFromSelection))
	c.Equal(3, changes, "taking a selected row out of the selection is")
	c.True(act(second, accessibility.RemoveFromSelection))
	c.Equal(3, changes, "a row that is not selected cannot be taken out of it again")
	c.Equal(1, table.SelectionCount())
	c.True(table.IsRowSelected(0))
	c.False(table.IsRowSelected(1))
}

// TestTableAccessibilityForwardedRequestsCarryNoKey verifies that a request handed on to a real panel — one inside a
// table cell, or inside a column header — arrives without the virtual-child key that brought it there. An application's
// own action callback would otherwise be given a key it never handed out, and a widget that keys virtual children of
// its own would take the table's key for one of them and act at whatever position it matched.
func TestTableAccessibilityForwardedRequestsCarryNoKey(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var header *unison.TableHeader[*tableTestRow]
	var wnd *unison.Window
	var keys []any
	clicks := 0
	record := func(p *unison.Panel) {
		p.Accessibility.ActionCallback = func(req accessibility.ActionRequest) bool {
			keys = append(keys, req.Key)
			return false
		}
	}
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			row := newTableTestRow("r0")
			row.cellFactory = func(_, _ int) unison.Paneler {
				wrapper := unison.NewPanel()
				wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
				button := unison.NewButton()
				button.ClickAnimationTime = 0
				button.SetTitle("Go")
				button.ClickCallback = func() { clicks++ }
				button.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Middle})
				record(button.AsPanel())
				wrapper.AddChild(button)
				return wrapper
			}
			table = axNewTable(row)
			colHeader := newAxButtonHeader("Pick", &clicks)
			record(colHeader.AsPanel())
			header = unison.NewTableHeader[*tableTestRow](table, colHeader,
				unison.NewTableColumnHeader[*tableTestRow]("Value", "", nil))
			scroller := axScroller(table, geom.NewSize(400, 200))
			scroller.SetColumnHeader(header)
			wnd = newHeadlessWindow(t, "forwarded keys", geom.NewRect(10, 10, 500, 400), axColumn(scroller))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	rows := axChildNodes(tree, node)
	c.Equal(1, len(rows))
	if len(rows) != 1 {
		return
	}
	cells := axChildNodes(tree, rows[0])
	c.True(len(cells) > 0)
	if len(cells) == 0 {
		return
	}
	inside := axUnignoredNodes(tree, cells[0])
	c.Equal(1, len(inside), "the button in the cell should have been described")
	if len(inside) != 1 {
		return
	}

	// A request aimed at the button itself, which the table forwards into the cell it was described in.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   inside[0].ID,
		Action: accessibility.Press,
	}))
	// A request aimed at the cell, which the table passes on to the content that can carry it out.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cells[0].ID,
		Action: accessibility.Press,
	}))

	headerNode := screen.AccessibilityNodeFor(header)
	c.True(headerNode != nil)
	columns := axChildNodes(tree, headerNode)
	c.True(len(columns) > 0)
	if len(columns) == 0 {
		return
	}
	inHeader := axUnignoredNodes(tree, columns[0])
	c.Equal(1, len(inHeader), "the button the column header is built around should have been described")
	if len(inHeader) != 1 {
		return
	}
	// A request aimed at something within a column header, which the header forwards into that header.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   inHeader[0].ID,
		Action: accessibility.Press,
	}))

	var forwarded []any
	var count int
	screen.Do(func() {
		forwarded = slices.Clone(keys)
		count = clicks
	})
	c.Equal(3, len(forwarded), "every one of the three requests should have reached a panel")
	for i, key := range forwarded {
		c.True(key == nil, "request %d arrived with a key a real panel never handed out: %v", i, key)
	}
	c.Equal(3, count, "each request should have clicked what it reached")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableHeaderAccessibilityActionBoundedByTheColumns verifies that the column a request names is bounded the way the
// description is. A header may hold more column headers than the table has columns, and only the ones that stand for a
// column are described; asking one of the others to sort would hand the model a column index that does not exist.
func TestTableHeaderAccessibilityActionBoundedByTheColumns(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(2)...)
	table.Columns = append(table.Columns, unison.ColumnInfo{ID: 0, Current: 100})
	table.SyncToModel()
	clicks := 0
	header := unison.NewTableHeader[*tableTestRow](table, newAxButtonHeader("First", &clicks),
		newAxButtonHeader("Extra", &clicks))

	c.True(header.PerformAccessibilityAction(accessibility.ActionRequest{
		Key:    0,
		Action: accessibility.Press,
	}), "the column header that stands for the table's one column sorts it")
	c.False(header.PerformAccessibilityAction(accessibility.ActionRequest{
		Key:    1,
		Action: accessibility.Press,
	}), "a column header the table has no column for cannot be sorted on")
	c.False(header.PerformAccessibilityAction(accessibility.ActionRequest{
		Key:    1,
		Action: accessibility.ScrollIntoView,
	}), "nor brought into view, since no such column is described")
}
