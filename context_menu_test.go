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
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/enums/side"
)

// These tests ask for contextual menus by right-click, the Menu key or shift+F10, and an assistive technology's
// request. A headless session uses in-window menus, which appear in the window's description as role.Menu nodes. A
// session owns most of the package's mutable globals while it runs, so none of these may call t.Parallel.

// The one item of the menu these tests give a table, a field or a wrapper around one, telling each apart from the
// menus around it, and the names the table-driven tests give a table and a list.
const (
	cmTableTitle   = "Table"
	cmFieldTitle   = "Field"
	cmWrapperTitle = "Wrapper"
	cmTableName    = "table"
	cmListName     = "list"
)

// cmRequests records the positions a ContextMenuCallback was asked for a menu at. It is only touched on the UI thread;
// read it through cmCount and cmLast.
type cmRequests struct {
	at []geom.Point
}

// cmCount returns how many times the callback has been asked for a menu.
func cmCount(screen *unison.HeadlessScreen, r *cmRequests) int {
	var count int
	screen.Do(func() { count = len(r.at) })
	return count
}

// cmLast returns the position the callback was most recently asked for a menu at.
func cmLast(screen *unison.HeadlessScreen, r *cmRequests) geom.Point {
	var where geom.Point
	screen.Do(func() {
		if len(r.at) != 0 {
			where = r.at[len(r.at)-1]
		}
	})
	return where
}

// cmMenu returns a ContextMenuCallback that records where it was asked for a menu and builds one holding the given
// titles, an empty title standing for a separator.
func cmMenu(r *cmRequests, titles ...string) func(geom.Point) unison.Menu {
	return func(where geom.Point) unison.Menu {
		r.at = append(r.at, where)
		return cmNewMenu(titles...)
	}
}

// cmNewMenu returns a contextual menu holding the given titles, an empty title standing for a separator.
func cmNewMenu(titles ...string) unison.Menu {
	f := unison.DefaultMenuFactory()
	m := f.NewMenu(unison.PopupMenuTemporaryBaseID|unison.ContextMenuIDFlag, "", nil)
	for _, title := range titles {
		if title == "" {
			m.InsertSeparator(-1, false)
			continue
		}
		m.InsertItem(-1, f.NewItem(-1, title, unison.KeyBinding{}, nil, func(unison.MenuItem) {}))
	}
	return m
}

// cmMenuNotingFocus returns a ContextMenuCallback that works as cmMenu's does and also notes whether the given panel
// held the keyboard focus each time it was asked.
func cmMenuNotingFocus(r *cmRequests, p unison.Paneler, focused *[]bool,
	titles ...string,
) func(geom.Point) unison.Menu {
	build := cmMenu(r, titles...)
	return func(where geom.Point) unison.Menu {
		*focused = append(*focused, p.AsPanel().Focused())
		return build(where)
	}
}

// cmMenuAt checks that the one menu open in the window was placed at the given point in the panel's own coordinates.
// The window must be large enough for the menu to fit there, since it is otherwise moved.
func cmMenuAt(c check.Checker, screen *unison.HeadlessScreen, wnd *unison.Window, p unison.Paneler, where geom.Point,
	msgAndArgs ...any,
) {
	c.Helper()
	var expected geom.Point
	screen.Do(func() { expected = p.AsPanel().PointToRoot(where) })
	menus := axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu)
	c.Equal(1, len(menus), "one menu is open")
	if len(menus) == 1 {
		c.Equal(expected, menus[0].Bounds.Point, msgAndArgs...)
	}
}

// cmScreenPoint returns the screen position, which the mouse is driven with, of a point in a panel's own coordinates. A
// panel in no window fails the test, since a click at the zero point would land in no window and a check that nothing
// opened would then pass for the wrong reason.
func cmScreenPoint(c check.Checker, screen *unison.HeadlessScreen, p unison.Paneler, local geom.Point) geom.Point {
	c.Helper()
	var pt geom.Point
	inWindow := false
	screen.Do(func() {
		if w := p.AsPanel().Window(); w != nil {
			inWindow = true
			pt = p.AsPanel().PointToRoot(local).Add(w.ContentRect().Point)
		}
	})
	if !inWindow {
		c.Fatal("the panel is not in a window")
	}
	return pt
}

// cmOpenMenu describes the window afresh and returns how many menus are open in it and the names of their items. A
// window that cannot be described fails the test, rather than reading as one with no menu open.
func cmOpenMenu(c check.Checker, screen *unison.HeadlessScreen, wnd *unison.Window) (menus int, items []string) {
	c.Helper()
	tree := screen.AccessibilityTree(wnd)
	if tree == nil {
		c.Fatal("the window could not be described")
	}
	return len(axNodesWithRole(tree, role.Menu)), axMenuItemNames(tree)
}

// cmCloseMenu presses Escape to take down the open in-window menu and checks that it has gone.
func cmCloseMenu(c check.Checker, screen *unison.HeadlessScreen, wnd *unison.Window) {
	c.Helper()
	screen.KeyPress(unison.KeyEscape, mod.None)
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "Escape should have closed the menu")
}

// cmFixedFocusable makes p focusable and 60×30 whatever it is laid out in.
func cmFixedFocusable(p *unison.Panel) {
	p.SetFocusable(true)
	p.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
		size := geom.NewSize(60, 30)
		return size, size, size
	})
}

// TestContextMenuRightClick verifies that a right-click opens the menu of the panel under the pointer, at the pointer
// in that panel's coordinates, and never the menu of a panel around it; that a release elsewhere or another button
// opens nothing; and that only the separators dangling at either end are trimmed and a menu left empty is not shown.
func TestContextMenuRightClick(t *testing.T) {
	c := check.New(t)
	var outer, inner, refuser, disabled, trimmed, hollow, elsewhere *unison.Panel
	var withholding *cmWithholdingPanel
	var outerCalls, refuserCalls, disabledCalls, withheldCalls, trimmedCalls, hollowCalls cmRequests
	var rightPresses int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			inner = fixedPanel(60, 30)
			refuser = fixedPanel(60, 30)
			refuser.ContextMenuCallback = func(where geom.Point) unison.Menu {
				refuserCalls.at = append(refuserCalls.at, where)
				return nil
			}
			disabled = fixedPanel(60, 30)
			disabled.ContextMenuCallback = cmMenu(&disabledCalls, "Disabled")
			disabled.SetEnabled(false)
			withholding = newCMWithholdingPanel()
			withholding.withhold = true
			withholding.ContextMenuCallback = cmMenu(&withheldCalls, "Withheld")
			trimmed = fixedPanel(60, 30)
			trimmed.ContextMenuCallback = cmMenu(&trimmedCalls, "", "", "A", "", "B", "", "")
			hollow = fixedPanel(60, 30)
			hollow.ContextMenuCallback = cmMenu(&hollowCalls, "", "")
			outer = axColumn(inner, refuser, disabled, withholding, trimmed, hollow)
			// A margin of the outer panel's own, for right-clicks on it rather than on what it holds.
			outer.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(10)))
			outer.ContextMenuCallback = cmMenu(&outerCalls, "Outer One", "Outer Two")
			// A press on a child with no mouse handling of its own is offered to the outer panel, as any press is.
			outer.MouseDownCallback = func(_ geom.Point, button, _ int, _ mod.Modifiers) bool {
				if button == unison.ButtonRight {
					rightPresses++
				}
				return true
			}
			elsewhere = fixedPanel(60, 30)
			wnd = newHeadlessWindow(t, "right-click", geom.NewRect(10, 10, 500, 400), axColumn(outer, elsewhere))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "no menu is open to begin with")
	presses := func() int {
		var count int
		screen.Do(func() { count = rightPresses })
		return count
	}

	// A right-click on the outer panel's own margin opens its menu, asked for at the pointer.
	screen.ClickWith(cmScreenPoint(c, screen, outer, geom.NewPoint(5, 5)), unison.ButtonRight, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click opens the menu")
	c.Equal([]string{"Outer One", "Outer Two"}, items, "and it is the one the panel under the pointer built")
	c.Equal(1, cmCount(screen, &outerCalls))
	c.Equal(geom.NewPoint(5, 5), cmLast(screen, &outerCalls), "the menu is asked for where the pointer is")
	c.Equal(0, presses(), "and the press is the window's rather than the panel's")
	cmCloseMenu(c, screen, wnd)

	// The panel under the pointer has no menu, so nothing opens: the menu of the panel around it is not its own, and
	// the press is delivered as any other.
	screen.ClickWith(cmScreenPoint(c, screen, inner, geom.NewPoint(20, 10)), unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a right-click on a panel without a menu opens nothing")
	c.Equal(1, cmCount(screen, &outerCalls), "the panel around it is not asked")
	c.Equal(1, presses(), "and the press is delivered as an ordinary one")

	// A press released outside the panel is a change of mind.
	screen.MouseDown(cmScreenPoint(c, screen, outer, geom.NewPoint(5, 5)), unison.ButtonRight, mod.None)
	screen.MouseUp(cmScreenPoint(c, screen, elsewhere, geom.NewPoint(20, 10)), unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a right-click released outside the panel opens nothing")
	c.Equal(1, cmCount(screen, &outerCalls), "and does not even ask for the menu")

	// Only the right button asks for a menu.
	for _, button := range []int{unison.ButtonLeft, unison.ButtonMiddle} {
		screen.ClickWith(cmScreenPoint(c, screen, outer, geom.NewPoint(5, 5)), button, mod.None)
		menus, _ = cmOpenMenu(c, screen, wnd)
		c.Equal(0, menus, "button %d opens nothing", button)
	}
	c.Equal(1, cmCount(screen, &outerCalls))

	// A callback that answers nil ends the request; the menu of the panel around it is not shown in its place.
	screen.ClickWith(cmScreenPoint(c, screen, refuser, geom.NewPoint(20, 10)), unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a callback that answers nil opens nothing")
	c.Equal(1, cmCount(screen, &refuserCalls))
	c.Equal(1, cmCount(screen, &outerCalls), "and the request is not handed on to the panel around it")
	c.Equal(1, presses(), "the press was taken for the menu all the same")

	// A disabled panel offers no menu, and the menu of the panel around it is not its own either.
	screen.ClickWith(cmScreenPoint(c, screen, disabled, geom.NewPoint(20, 10)), unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a right-click on a disabled panel opens nothing")
	c.Equal(0, cmCount(screen, &disabledCalls))
	c.Equal(1, cmCount(screen, &outerCalls))
	c.Equal(2, presses(), "and is delivered as an ordinary press")

	// A panel withholding its menu is treated as one without, until it stops withholding.
	screen.ClickWith(cmScreenPoint(c, screen, withholding, geom.NewPoint(20, 10)), unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a right-click on a panel withholding its menu opens nothing")
	c.Equal(0, cmCount(screen, &withheldCalls), "the panel is not asked")
	c.Equal(1, cmCount(screen, &outerCalls), "nor is the panel around it")
	c.Equal(3, presses(), "and the press is delivered as an ordinary one")
	screen.Do(func() { withholding.withhold = false })
	screen.ClickWith(cmScreenPoint(c, screen, withholding, geom.NewPoint(20, 10)), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "once no longer withheld, the panel's menu opens")
	c.Equal([]string{"Withheld"}, items)
	c.Equal(1, cmCount(screen, &withheldCalls))
	c.Equal(3, presses())
	cmCloseMenu(c, screen, wnd)

	// Separators dangling at either end of a menu are taken off, and only those.
	screen.ClickWith(cmScreenPoint(c, screen, trimmed, geom.NewPoint(20, 10)), unison.ButtonRight, mod.None)
	tree := screen.AccessibilityTree(wnd)
	c.Equal(1, len(axNodesWithRole(tree, role.Menu)))
	c.Equal([]string{"A", "B"}, axMenuItemNames(tree))
	c.Equal(1, len(axNodesWithRole(tree, role.Separator)), "the separator between the items is kept")
	cmCloseMenu(c, screen, wnd)

	// A menu holding nothing else is not shown at all.
	screen.ClickWith(cmScreenPoint(c, screen, hollow, geom.NewPoint(20, 10)), unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a menu holding nothing but separators is not shown")
	c.Equal(1, cmCount(screen, &hollowCalls))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuKeyboard verifies that the Menu key and shift+F10 open the menu of the focused panel, and only its
// own, at its ContextMenuAnchor or DefaultContextMenuAnchor; that a focused panel without a menu, or withholding its
// menu, opens nothing, even within a panel with a menu, and is offered the chord as an ordinary key; that a repeat or
// another chord opens nothing; and that the chord reaches the menu of a focused field rather than the field itself.
func TestContextMenuKeyboard(t *testing.T) {
	c := check.New(t)
	var outer, focusable *unison.Panel
	var withholding *cmWithholdingPanel
	var anchored *cmAnchoredPanel
	var field *unison.Field
	var outerCalls, withheldCalls, anchoredCalls cmRequests
	var keys, withheldKeys, fieldKeys int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			focusable = fixedPanel(60, 30)
			focusable.SetFocusable(true)
			focusable.KeyDownCallback = func(_ unison.KeyCode, _ mod.Modifiers, _ bool) bool {
				keys++
				return true
			}
			withholding = newCMWithholdingPanel()
			withholding.withhold = true
			withholding.ContextMenuCallback = cmMenu(&withheldCalls, "Withheld")
			withholding.KeyDownCallback = func(_ unison.KeyCode, _ mod.Modifiers, _ bool) bool {
				withheldKeys++
				return true
			}
			outer = axColumn(focusable, withholding)
			outer.SetFocusable(true)
			outer.ContextMenuCallback = cmMenu(&outerCalls, "Outer")
			anchored = newCMAnchoredPanel(geom.NewPoint(7, 9))
			anchored.ContextMenuCallback = cmMenu(&anchoredCalls, "Anchored")
			field = unison.NewField()
			field.SetText("hello")
			fieldKeyDown := field.KeyDownCallback
			field.KeyDownCallback = func(key unison.KeyCode, mods mod.Modifiers, repeat bool) bool {
				fieldKeys++
				return fieldKeyDown(key, mods, repeat)
			}
			wnd = newHeadlessWindow(t, "keyboard", geom.NewRect(10, 10, 500, 400), axColumn(outer, anchored, field))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { outer.RequestFocus() }))

	// The focused panel says nothing about where its menu belongs, so it opens in the middle of the panel.
	var center geom.Point
	screen.Do(func() { center = outer.ContentRect(false).Center() })
	screen.KeyPress(unison.KeyMenu, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "the Menu key opens the menu")
	c.Equal([]string{"Outer"}, items)
	c.Equal(center, cmLast(screen, &outerCalls), "a panel that says nothing about it has the menu in its middle")
	cmCloseMenu(c, screen, wnd)

	screen.KeyPress(unison.KeyF10, mod.Shift)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "so does shift+F10")
	c.Equal([]string{"Outer"}, items)
	cmCloseMenu(c, screen, wnd)
	c.Equal(2, cmCount(screen, &outerCalls))

	// Holding the key down opens the menu once, not once per repeat.
	screen.KeyDown(unison.KeyMenu, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	cmCloseMenu(c, screen, wnd)
	screen.KeyDown(unison.KeyMenu, mod.None)
	screen.KeyUp(unison.KeyMenu, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a repeat does not open the menu again")
	c.Equal(3, cmCount(screen, &outerCalls))

	// F10 alone, and the Menu key with a modifier held, are not the chord.
	screen.KeyPress(unison.KeyF10, mod.None)
	screen.KeyPress(unison.KeyMenu, mod.Control)
	screen.KeyPress(unison.KeyF10, mod.Shift|mod.Control)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "only the Menu key alone and shift+F10 open the menu")
	c.Equal(3, cmCount(screen, &outerCalls))

	// The focused panel has no menu, so nothing opens: the menu of the panel around it is not its own, and the chord is
	// offered to the panel as any key is.
	c.True(screen.Do(func() { focusable.RequestFocus() }))
	screen.KeyPress(unison.KeyMenu, mod.None)
	screen.KeyPress(unison.KeyF10, mod.Shift)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "the chord opens nothing for a focused panel without a menu")
	c.Equal(3, cmCount(screen, &outerCalls), "and the panel around it is not asked")
	var offered int
	screen.Do(func() { offered = keys })
	c.Equal(2, offered, "the chord is offered to the panel as an ordinary key")

	// A panel withholding its menu opens nothing either, and is offered the chord as an ordinary key, until it stops
	// withholding.
	c.True(screen.Do(func() { withholding.RequestFocus() }))
	screen.KeyPress(unison.KeyMenu, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "the chord opens nothing for a panel withholding its menu")
	c.Equal(0, cmCount(screen, &withheldCalls), "which is not asked")
	c.Equal(3, cmCount(screen, &outerCalls), "nor is the panel around it")
	screen.Do(func() { offered = withheldKeys })
	c.Equal(1, offered, "and the chord is offered to the withholding panel as an ordinary key")
	screen.Do(func() { withholding.withhold = false })
	screen.KeyPress(unison.KeyMenu, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "the chord opens the menu once it is no longer withheld")
	c.Equal([]string{"Withheld"}, items)
	c.Equal(1, cmCount(screen, &withheldCalls))
	screen.Do(func() { offered = withheldKeys })
	c.Equal(1, offered, "and the chord that opened it is not offered to the panel")
	cmCloseMenu(c, screen, wnd)

	// A widget that says where its menu belongs has it opened there.
	c.True(screen.Do(func() { anchored.RequestFocus() }))
	screen.KeyPress(unison.KeyMenu, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{"Anchored"}, items)
	c.Equal(geom.NewPoint(7, 9), cmLast(screen, &anchoredCalls), "the menu opens where ContextMenuAnchor says")
	cmCloseMenu(c, screen, wnd)

	// A field takes nearly every key, so the chord must be handled before the field is offered it.
	c.True(screen.Do(func() { field.RequestFocus() }))
	screen.KeyPress(unison.KeyF10, mod.Shift)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "shift+F10 opens a focused field's menu")
	// Taking the focus selected all of the text and the session's clipboard starts empty, so only Cut and Copy apply.
	c.Equal([]string{"Cut", "Copy"}, items, "the field's own menu is the one shown")
	cmCloseMenu(c, screen, wnd)
	screen.Do(func() { offered = fieldKeys })
	c.Equal(0, offered, "the chord must not have been offered to the field, which would have taken it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// cmWithholdingPanel is a focusable panel with a menu it may withhold.
type cmWithholdingPanel struct {
	unison.Panel
	withhold bool
}

func newCMWithholdingPanel() *cmWithholdingPanel {
	p := &cmWithholdingPanel{}
	p.Self = p
	cmFixedFocusable(p.AsPanel())
	return p
}

// WithholdsContextMenu implements unison.ContextMenuWithholder.
func (p *cmWithholdingPanel) WithholdsContextMenu() bool {
	return p.withhold
}

// cmAnchoredPanel is a focusable panel that says where its contextual menu belongs.
type cmAnchoredPanel struct {
	unison.Panel
	anchor geom.Point
}

func newCMAnchoredPanel(anchor geom.Point) *cmAnchoredPanel {
	p := &cmAnchoredPanel{anchor: anchor}
	p.Self = p
	cmFixedFocusable(p.AsPanel())
	return p
}

// ContextMenuAnchor implements unison.ContextMenuAnchorer.
func (p *cmAnchoredPanel) ContextMenuAnchor() geom.Point {
	return p.anchor
}

// TestContextMenuAccessibility verifies that only an enabled panel with a callback that is not withholding its menu, in
// the active window, offers accessibility.ShowContextMenu, that a request for any other panel's menu is refused without
// asking or focusing it, and that carrying a request out opens the menu in the panel's middle, focusing the panel first
// when it can take the focus.
func TestContextMenuAccessibility(t *testing.T) {
	c := check.New(t)
	var target, passive, plain, disabled, background *unison.Panel
	var withholding *cmWithholdingPanel
	var other *unison.Field
	var targetCalls, passiveCalls, disabledCalls, withheldCalls, backgroundCalls cmRequests
	var wnd, back *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 1000, Height: 600},
		unison.StartupFinishedCallback(func() {
			target = fixedPanel(60, 30)
			target.SetFocusable(true)
			target.ContextMenuCallback = cmMenu(&targetCalls, "Target")
			passive = fixedPanel(60, 30)
			passive.ContextMenuCallback = cmMenu(&passiveCalls, "Passive")
			plain = fixedPanel(60, 30)
			plain.SetFocusable(true)
			disabled = fixedPanel(60, 30)
			disabled.ContextMenuCallback = cmMenu(&disabledCalls, "Disabled")
			disabled.SetEnabled(false)
			withholding = newCMWithholdingPanel()
			withholding.ContextMenuCallback = cmMenu(&withheldCalls, "Withheld")
			other = unison.NewField()
			background = fixedPanel(60, 30)
			background.ContextMenuCallback = cmMenu(&backgroundCalls, "Background")
			back = newHeadlessWindow(t, "background", geom.NewRect(550, 10, 300, 200), axColumn(background))
			wnd = newHeadlessWindow(t, "front", geom.NewRect(10, 10, 500, 400),
				axColumn(target, passive, plain, disabled, withholding, other))
			if wnd != nil {
				wnd.ToFront()
				other.RequestFocus()
			}
		}))
	c.NotNil(wnd)
	c.NotNil(back)

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(target))
	c.True(node.Actions.Has(accessibility.ShowContextMenu), "a panel with a callback offers its menu")
	c.False(node.Ignored, "and a panel that can be acted on is not scaffolding to look past")
	c.False(axMustNode(c, screen.AccessibilityNodeFor(plain)).Actions.Has(accessibility.ShowContextMenu),
		"a panel without one does not")
	disabledNode := axMustNode(c, screen.AccessibilityNodeFor(disabled))
	c.False(disabledNode.Actions.Has(accessibility.ShowContextMenu), "a disabled panel does not offer its menu")

	var center geom.Point
	screen.Do(func() { center = target.ContentRect(false).Center() })
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}))
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "asking for the menu opens it")
	c.Equal([]string{"Target"}, items)
	c.Equal(center, cmLast(screen, &targetCalls), "with no pointer, the menu opens in the middle of the panel")
	cmMenuAt(c, screen, wnd, target, center, "and is put there")
	var focused bool
	screen.Do(func() { focused = target.Focused() })
	c.True(focused, "the panel whose menu was asked for takes the focus its commands act on")
	cmCloseMenu(c, screen, wnd)

	// A panel that cannot hold the focus has its menu opened all the same, leaving the focus where it was.
	screen.AccessibilityTree(wnd)
	passiveNode := axMustNode(c, screen.AccessibilityNodeFor(passive))
	c.True(passiveNode.Actions.Has(accessibility.ShowContextMenu))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   passiveNode.ID,
		Action: accessibility.ShowContextMenu,
	}))
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{"Passive"}, items)
	screen.Do(func() { center = passive.ContentRect(false).Center() })
	c.Equal(center, cmLast(screen, &passiveCalls))
	cmMenuAt(c, screen, wnd, passive, center,
		"the menu is put in the middle of the panel, which is not where the window's origin is")
	screen.Do(func() { focused = target.Focused() })
	c.True(focused, "the focus stays where it was when the panel cannot hold it")
	cmCloseMenu(c, screen, wnd)

	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   disabledNode.ID,
		Action: accessibility.ShowContextMenu,
	}), "a disabled panel is not offered its menu, so a request for it is not performed")
	c.Equal(0, cmCount(screen, &disabledCalls))

	// A panel that starts withholding its menu after it was described as offering it refuses a request made against
	// that description, without being asked or focused, and is not offered the menu once described again, until it
	// stops withholding.
	withholdingNode := axMustNode(c, screen.AccessibilityNodeFor(withholding))
	c.True(withholdingNode.Actions.Has(accessibility.ShowContextMenu), "the panel offers its menu while not withholding")
	screen.Do(func() { withholding.withhold = true })
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   withholdingNode.ID,
		Action: accessibility.ShowContextMenu,
	}), "a request against a description made before the panel withheld its menu is refused")
	c.Equal(0, cmCount(screen, &withheldCalls), "without the panel being asked")
	screen.Do(func() { focused = withholding.Focused() })
	c.False(focused, "or focused")
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus)
	withholdingNode = axMustNode(c, screen.AccessibilityNodeFor(withholding))
	c.False(withholdingNode.Actions.Has(accessibility.ShowContextMenu),
		"a panel withholding its menu does not offer it")
	screen.Do(func() { withholding.withhold = false })
	screen.AccessibilityTree(wnd)
	withholdingNode = axMustNode(c, screen.AccessibilityNodeFor(withholding))
	c.True(withholdingNode.Actions.Has(accessibility.ShowContextMenu), "once no longer withheld, the menu is offered")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   withholdingNode.ID,
		Action: accessibility.ShowContextMenu,
	}), "and a request for it is carried out")
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{"Withheld"}, items)
	c.Equal(1, cmCount(screen, &withheldCalls))
	screen.Do(func() { focused = withholding.Focused() })
	c.True(focused, "with the panel focused first")
	cmCloseMenu(c, screen, wnd)

	// A menu opens in whatever window is active, so a panel in any other window neither offers nor opens one.
	screen.AccessibilityTree(back)
	backNode := axMustNode(c, screen.AccessibilityNodeFor(background))
	c.False(backNode.Actions.Has(accessibility.ShowContextMenu),
		"a panel whose menu would open in some other window does not offer it")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   backNode.ID,
		Action: accessibility.ShowContextMenu,
	}), "so a request for it is not performed")
	c.Equal(0, cmCount(screen, &backgroundCalls))
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "nothing opened in the active window either")

	// Bringing the window to the front brings the action back with it.
	c.True(screen.Do(func() { back.ToFront() }))
	screen.AccessibilityTree(back)
	backNode = axMustNode(c, screen.AccessibilityNodeFor(background))
	c.True(backNode.Actions.Has(accessibility.ShowContextMenu), "the active window's panel offers its menu again")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   backNode.ID,
		Action: accessibility.ShowContextMenu,
	}))
	menus, items = cmOpenMenu(c, screen, back)
	c.Equal(1, menus)
	c.Equal([]string{"Background"}, items)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// cmRowWidget is a Table or a List for the tests that treat the two alike: a menu offered by the rows that acts on the
// selection the same way. Everything but selected is UI thread only.
type cmRowWidget struct {
	panel    unison.Paneler
	quiet    unison.Paneler
	calls    *cmRequests
	selectAt func(indexes ...int)
	selected func() []int
	lead     func() int
	rowPoint func(row int) geom.Point
	beneath  func(row int) geom.Point
	rowNode  func(tree *accessibility.Tree, row int) *accessibility.Node
	rowNodes func(tree *accessibility.Tree, p unison.Paneler) []*accessibility.Node
	name     string
}

// cmRowWidgets returns the table and the list of a test as cmRowWidgets.
func cmRowWidgets(c check.Checker, screen *unison.HeadlessScreen, table, quietTable *unison.Table[*tableTestRow],
	tableCalls *cmRequests, list, quietList *unison.List[string], listCalls *cmRequests,
) []cmRowWidget {
	tableRows := func(tree *accessibility.Tree, p unison.Paneler) []*accessibility.Node {
		byName := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(p)))
		rows := make([]*accessibility.Node, 0, len(byName))
		for _, row := range byName {
			rows = append(rows, row)
		}
		return rows
	}
	return []cmRowWidget{
		{
			name:  cmTableName,
			panel: table,
			quiet: quietTable,
			calls: tableCalls,
			selectAt: func(indexes ...int) {
				table.ClearSelection()
				table.SelectByIndex(indexes...)
			},
			selected: func() []int { return axTableSelectedIndexes(screen, table) },
			lead:     table.LeadRowIndex,
			rowPoint: func(row int) geom.Point {
				frame := table.RowFrame(row)
				return geom.NewPoint(frame.X+10, frame.CenterY())
			},
			beneath: func(row int) geom.Point {
				frame := table.RowFrame(row)
				return geom.NewPoint(frame.X, frame.Bottom())
			},
			rowNode: func(tree *accessibility.Tree, row int) *accessibility.Node {
				return axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))["r"+strconv.Itoa(row)]
			},
			rowNodes: tableRows,
		},
		{
			name:     cmListName,
			panel:    list,
			quiet:    quietList,
			calls:    listCalls,
			selectAt: func(indexes ...int) { list.Select(false, indexes...) },
			selected: func() []int { return selectedIndexesOn(screen, list) },
			lead:     list.Lead,
			rowPoint: func(row int) geom.Point {
				rect := list.RowRect(row)
				return geom.NewPoint(rect.X+10, rect.CenterY())
			},
			beneath: func(row int) geom.Point {
				rect := list.RowRect(row)
				return geom.NewPoint(rect.X, rect.Bottom())
			},
			rowNode: func(tree *accessibility.Tree, row int) *accessibility.Node {
				return axNodeWithRowIndex(tree, axMustNode(c, screen.AccessibilityNodeFor(list)), row)
			},
			rowNodes: func(tree *accessibility.Tree, p unison.Paneler) []*accessibility.Node {
				return axChildNodes(tree, axMustNode(c, screen.AccessibilityNodeFor(p)))
			},
		},
	}
}

// TestContextMenuRows verifies, for a table and a list alike, that the rows offer the widget's menu only when it has
// one; that a request or a right-click on an unselected row selects just that row, makes it the lead row and focuses
// the widget, while one on a row of a larger selection keeps it all; and that the menu opens beneath the row.
func TestContextMenuRows(t *testing.T) {
	c := check.New(t)
	var table, quietTable *unison.Table[*tableTestRow]
	var list, quietList *unison.List[string]
	var tableCalls, listCalls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			quietTable = axNewTable(newTableTestRow("q0"), newTableTestRow("q1"))
			table = axNewTable(flatRows(5)...)
			table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			quietList = unison.NewList[string]()
			quietList.Append("Quiet")
			list = unison.NewList[string]()
			list.Append("Zero", "One", "Two", "Three", "Four")
			list.ContextMenuCallback = cmMenu(&listCalls, "List")
			wnd = newHeadlessWindow(t, "rows", geom.NewRect(10, 10, 600, 700),
				axColumn(quietTable, table, quietList, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)

	for _, one := range cmRowWidgets(c, screen, table, quietTable, &tableCalls, list, quietList, &listCalls) {
		tree := screen.AccessibilityTree(wnd)
		rows := one.rowNodes(tree, one.panel)
		c.Equal(5, len(rows), "the %s rows are described", one.name)
		for _, row := range rows {
			c.True(row.Actions.Has(accessibility.ShowContextMenu), "a row of a %s with a menu offers it", one.name)
		}
		for _, row := range one.rowNodes(tree, one.quiet) {
			c.False(row.Actions.Has(accessibility.ShowContextMenu), "a row of a %s without one does not", one.name)
		}

		// A request on a row that is not selected selects that row alone.
		c.True(screen.Do(func() {
			one.quiet.AsPanel().RequestFocus()
			one.selectAt(0, 1)
		}))
		node := axMustNode(c, one.rowNode(screen.AccessibilityTree(wnd), 2), "the %s row is described", one.name)
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.ShowContextMenu,
		}), "asking a %s row for the menu opens it", one.name)
		menus, items := cmOpenMenu(c, screen, wnd)
		c.Equal(1, menus, "asking a %s row for the menu opens it", one.name)
		c.Equal(1, len(items), "the %s menu is the one shown: %v", one.name, items)
		c.Equal([]int{2}, one.selected(), "a %s row that was not selected becomes the selection", one.name)
		var focused bool
		var lead int
		var expected geom.Point
		screen.Do(func() {
			focused = one.panel.AsPanel().Focused()
			lead = one.lead()
			expected = one.beneath(2)
		})
		c.True(focused, "the %s takes the focus its menu's commands act on", one.name)
		c.Equal(2, lead, "and the person is left on the %s row", one.name)
		c.Equal(expected, cmLast(screen, one.calls), "the %s menu opens beneath the row", one.name)
		cmMenuAt(c, screen, wnd, one.panel, expected, "and is put there, in the window's coordinates")
		cmCloseMenu(c, screen, wnd)

		// A request on one row of a larger selection keeps all of it.
		screen.Do(func() { one.selectAt(0, 1, 2) })
		node = axMustNode(c, one.rowNode(screen.AccessibilityTree(wnd), 1), "the %s row is described", one.name)
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.ShowContextMenu,
		}))
		menus, _ = cmOpenMenu(c, screen, wnd)
		c.Equal(1, menus)
		c.Equal([]int{0, 1, 2}, one.selected(), "the %s menu acts on the whole selection", one.name)
		screen.Do(func() { lead = one.lead() })
		c.Equal(1, lead, "with the person on the %s row asked", one.name)
		cmCloseMenu(c, screen, wnd)

		// A right-click on one of several selected rows keeps the whole selection through the release.
		var rowPoint geom.Point
		screen.Do(func() {
			one.selectAt(0, 1, 2)
			rowPoint = one.rowPoint(1)
		})
		screen.ClickWith(cmScreenPoint(c, screen, one.panel, rowPoint), unison.ButtonRight, mod.None)
		menus, _ = cmOpenMenu(c, screen, wnd)
		c.Equal(1, menus, "a right-click on a %s row opens the menu", one.name)
		c.Equal([]int{0, 1, 2}, one.selected(), "and leaves the whole %s selection in place", one.name)
		cmCloseMenu(c, screen, wnd)

		// A right-click on a row outside the selection selects just that row and focuses the widget, though the widget
		// is sent no mouse event for the press.
		screen.Do(func() {
			one.quiet.AsPanel().RequestFocus()
			rowPoint = one.rowPoint(4)
		})
		screen.ClickWith(cmScreenPoint(c, screen, one.panel, rowPoint), unison.ButtonRight, mod.None)
		menus, _ = cmOpenMenu(c, screen, wnd)
		c.Equal(1, menus)
		c.Equal([]int{4}, one.selected(), "only the %s row right-clicked on is selected", one.name)
		screen.Do(func() {
			focused = one.panel.AsPanel().Focused()
			lead = one.lead()
		})
		c.True(focused, "and the %s has the focus", one.name)
		c.Equal(4, lead, "with the person on the %s row", one.name)
		cmCloseMenu(c, screen, wnd)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuTableCells verifies what is particular to a table: its cells offer the menu too, and a request on a
// cell leaves the cell cursor on the cell while a right-click or a request on a row puts it at row level; a right-drag
// is handed back to the table's own mouse handling and opens no menu; and shift+F10 opens the menu beneath the row the
// person is on.
func TestContextMenuTableCells(t *testing.T) {
	c := check.New(t)
	var table, quiet *unison.Table[*tableTestRow]
	var calls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			quiet = axNewTable(newTableTestRow("q0"), newTableTestRow("q1"))
			table = axNewTable(flatRows(5)...)
			table.ContextMenuCallback = cmMenu(&calls, cmTableTitle)
			wnd = newHeadlessWindow(t, "table", geom.NewRect(10, 10, 600, 600), axColumn(quiet, table))
			if wnd != nil {
				wnd.ToFront()
				quiet.RequestFocus()
			}
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	rows := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))
	c.Equal(5, len(rows))
	for name, row := range rows {
		for _, cell := range axChildNodes(tree, row) {
			if cell.Role == role.Cell {
				c.True(cell.Actions.Has(accessibility.ShowContextMenu), "cell of row %s offers the menu", name)
			}
		}
	}
	for _, row := range axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(quiet))) {
		for _, cell := range axChildNodes(tree, row) {
			c.False(cell.Actions.Has(accessibility.ShowContextMenu), "a cell of a table without a menu does not")
		}
	}

	// A request on a cell selects the cell's row and leaves the cell cursor on the cell.
	screen.Do(func() {
		table.ClearSelection()
		table.SelectByIndex(0, 1, 2)
	})
	tree = screen.AccessibilityTree(wnd)
	rows = axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))
	var cellID accessibility.NodeID
	for _, cell := range axChildNodes(tree, axMustNode(c, rows["r3"])) {
		if cell.Role == role.Cell && cell.ColumnIndex == 1 {
			cellID = cell.ID
		}
	}
	c.True(cellID != 0, "the row's second cell is described")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cellID,
		Action: accessibility.ShowContextMenu,
	}))
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]int{3}, axTableSelectedIndexes(screen, table))
	var lead, leadCol int
	var expected geom.Point
	screen.Do(func() {
		lead = table.LeadRowIndex()
		leadCol = table.LeadColumnIndex()
		frame := table.CellFrame(3, 1)
		expected = geom.NewPoint(frame.X, frame.Bottom())
	})
	c.Equal(3, lead)
	c.Equal(1, leadCol, "the cell cursor is on the cell the menu was asked for")
	c.Equal(expected, cmLast(screen, &calls), "the menu opens beneath the cell")
	cmMenuAt(c, screen, wnd, table, expected, "and is put there")
	cmCloseMenu(c, screen, wnd)

	// A right-click on a row, and a request on a row, put the cell cursor back at row level.
	var rowPoint geom.Point
	screen.Do(func() {
		table.SetLeadCell(3, 1)
		frame := table.RowFrame(1)
		rowPoint = geom.NewPoint(frame.X+10, frame.CenterY())
	})
	screen.ClickWith(cmScreenPoint(c, screen, table, rowPoint), unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	screen.Do(func() {
		lead = table.LeadRowIndex()
		leadCol = table.LeadColumnIndex()
	})
	c.Equal(1, lead, "a right-click on a row puts the person on the row")
	c.Equal(-1, leadCol, "at row level")
	cmCloseMenu(c, screen, wnd)
	screen.Do(func() { table.SetLeadCell(1, 0) })
	tree = screen.AccessibilityTree(wnd)
	rows = axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   axMustNode(c, rows["r2"]).ID,
		Action: accessibility.ShowContextMenu,
	}))
	screen.Do(func() {
		lead = table.LeadRowIndex()
		leadCol = table.LeadColumnIndex()
	})
	c.Equal(2, lead, "a request on a row puts the person on the row")
	c.Equal(-1, leadCol, "at row level")
	cmCloseMenu(c, screen, wnd)

	// Once a right press moves beyond the drag threshold, the window hands it to the table's own mouse handling where
	// it was made, followed by the drags and the release, and no menu opens.
	var downs []geom.Point
	var downButtons []int
	var drags, ups int
	restore := func() {
		table.MouseDownCallback = table.DefaultMouseDown
		table.MouseDragCallback = table.DefaultMouseDrag
		table.MouseUpCallback = table.DefaultMouseUp
	}
	screen.Do(func() {
		table.ClearSelection()
		table.SelectByIndex(0)
		table.MouseDownCallback = func(where geom.Point, button, clickCount int, mods mod.Modifiers) bool {
			downs = append(downs, where)
			downButtons = append(downButtons, button)
			return table.DefaultMouseDown(where, button, clickCount, mods)
		}
		table.MouseDragCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
			drags++
			return table.DefaultMouseDrag(where, button, mods)
		}
		table.MouseUpCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
			ups++
			return table.DefaultMouseUp(where, button, mods)
		}
		frame := table.RowFrame(2)
		rowPoint = geom.NewPoint(frame.X+10, frame.CenterY())
	})
	before := cmCount(screen, &calls)
	_, drift := unison.DragGestureParameters()
	start := cmScreenPoint(c, screen, table, rowPoint)
	screen.MouseDown(start, unison.ButtonRight, mod.None)
	c.Equal([]int{2}, axTableSelectedIndexes(screen, table), "the press selects the row it landed on")
	var delivered int
	screen.Do(func() { delivered = len(downs) })
	c.Equal(0, delivered, "and is not delivered while it may still be a right-click")
	screen.MouseMove(start.Add(geom.NewPoint(0, drift*2)), mod.None)
	screen.MouseMove(start.Add(geom.NewPoint(0, drift*4)), mod.None)
	screen.MouseUp(start.Add(geom.NewPoint(0, drift*4)), unison.ButtonRight, mod.None)
	var gotDowns []geom.Point
	var gotButtons []int
	var gotDrags, gotUps int
	screen.Do(func() {
		gotDowns = slices.Clone(downs)
		gotButtons = slices.Clone(downButtons)
		gotDrags = drags
		gotUps = ups
		restore()
	})
	c.Equal([]geom.Point{rowPoint}, gotDowns, "the drag hands the press back where it was made")
	c.Equal([]int{unison.ButtonRight}, gotButtons)
	c.Equal(2, gotDrags, "followed by every move of the drag")
	c.Equal(1, gotUps, "and the release")
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a right-drag opens no menu")
	c.Equal(before, cmCount(screen, &calls), "nor even asks for one")
	c.Equal([]int{2}, axTableSelectedIndexes(screen, table), "and leaves the selection as the press made it")

	// With the table holding the focus, shift+F10 opens the menu beneath the row the person is on.
	screen.Do(func() {
		table.SetLeadCell(2, -1)
		table.RequestFocus()
		frame := table.RowFrame(2)
		expected = geom.NewPoint(frame.X, frame.Bottom())
	})
	before = cmCount(screen, &calls)
	screen.KeyPress(unison.KeyF10, mod.Shift)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "shift+F10 opens the focused table's menu")
	c.Equal(before+1, cmCount(screen, &calls))
	c.Equal(expected, cmLast(screen, &calls), "beneath the row the person is on")
	cmMenuAt(c, screen, wnd, table, expected, "and is put there")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuDockTab verifies that a dock tab offers and opens its menu, for a right-click on its title or on its
// own padding, while its container holds several dockables, and withholds it when alone, even when the other tabs go
// between the press and the release: a right-click on the lone tab or its close button is then an ordinary press that
// opens no enclosing panel's menu and never closes the dockable, an assistive technology's request is refused, and a
// right press chorded onto a left press held on the tab does not leave the tab pressed.
func TestContextMenuDockTab(t *testing.T) {
	c := check.New(t)
	var first, second *axDockable
	var container *unison.DockContainer
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			first = newAxDockable("First", "")
			second = newAxDockable("Second", "")
			dock := unison.NewDock()
			dock.DockTo(first, nil, side.Left)
			container = unison.Ancestor[*unison.DockContainer](first)
			if container != nil {
				container.Stack(second, -1)
				container.SetCurrentDockable(first)
			}
			wnd = newHeadlessWindow(t, "dock", geom.NewRect(10, 10, 600, 400), dock)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	c.True(container != nil)

	tree := screen.AccessibilityTree(wnd)
	tabs := axNodesWithRole(tree, role.Tab)
	c.Equal(2, len(tabs))
	if len(tabs) != 2 {
		return
	}
	c.True(tabs[0].Actions.Has(accessibility.ShowContextMenu), "a tab among several offers its menu")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   tabs[1].ID,
		Action: accessibility.ShowContextMenu,
	}))
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{"Close Other Tabs", "Close All Tabs"}, items)
	cmCloseMenu(c, screen, wnd)

	// tabParts returns the tab panel described by the given node and its title, which is not described itself: the
	// title is the panel under the middle of the tab, and the tab is the panel holding it.
	tabParts := func(node *accessibility.Node) (tab, title *unison.Panel) {
		screen.Do(func() {
			content := wnd.Content()
			title = content.PanelAt(content.PointFromRoot(node.Bounds.Center()))
			if title != nil {
				tab = title.Parent()
			}
		})
		c.NotNil(tab)
		return tab, title
	}
	// onTitle and onPadding return the screen positions of the middle of a tab's title and of a point within the tab's
	// own padding, where no child of the tab is.
	onTitle := func(title *unison.Panel) geom.Point {
		var center geom.Point
		screen.Do(func() { center = title.ContentRect(false).Center() })
		return cmScreenPoint(c, screen, title, center)
	}
	onPadding := func(tab *unison.Panel) geom.Point {
		pt := geom.NewPoint(1, 1)
		var own bool
		screen.Do(func() { own = tab.PanelAt(pt) == tab && !pt.In(tab.ContentRect(false)) })
		c.True(own, "the test needs the point to be in the tab's padding")
		return cmScreenPoint(c, screen, tab, pt)
	}

	// A right-click on the title of the second tab, which is not the current one, or on the tab's own padding, opens
	// its menu without bringing its dockable to the front.
	secondTab, secondTitle := tabParts(tabs[1])
	var current unison.Dockable
	for _, one := range []struct {
		name string
		at   geom.Point
	}{
		{at: onTitle(secondTitle), name: "title"},
		{at: onPadding(secondTab), name: "padding"},
	} {
		screen.ClickWith(one.at, unison.ButtonRight, mod.None)
		menus, items = cmOpenMenu(c, screen, wnd)
		c.Equal(1, menus, "a right-click on a tab's %s opens its menu", one.name)
		c.Equal([]string{"Close Other Tabs", "Close All Tabs"}, items)
		cmCloseMenu(c, screen, wnd)
		screen.Do(func() { current = container.CurrentDockable() })
		c.True(current == unison.Dockable(first), "a right-click on the %s does not bring the dockable to the front",
			one.name)
	}

	// The other tab going away between the press and the release leaves the tab alone, withholding its menu, so the
	// release opens nothing.
	firstTab, firstTitle := tabParts(tabs[0])
	firstPoint := onTitle(firstTitle)
	screen.MouseDown(firstPoint, unison.ButtonRight, mod.None)
	screen.Do(func() { container.Close(second) })
	screen.MouseUp(firstPoint, unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a tab whose menu was withheld by the time of the release opens nothing")

	// With one dockable left there are no other tabs to close, so the tab has no menu to offer: a right-click on it, or
	// on its close button, is an ordinary press, which the tab leaves to the panels around it. The menu of the window's
	// content is not the tab's own, so it does not open in the tab's place.
	tree = screen.AccessibilityTree(wnd)
	tabs = axNodesWithRole(tree, role.Tab)
	c.Equal(1, len(tabs))
	if len(tabs) != 1 {
		return
	}
	c.False(tabs[0].Actions.Has(accessibility.ShowContextMenu), "a tab alone in its container has no menu")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   tabs[0].ID,
		Action: accessibility.ShowContextMenu,
	}), "and refuses an assistive technology's request for it")
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus)
	closers := axChildNodes(tree, tabs[0])
	c.Equal(1, len(closers), "the tab still holds its close button")
	var contentCalls cmRequests
	var rightPresses int
	screen.Do(func() {
		content := wnd.Content()
		content.ContextMenuCallback = cmMenu(&contentCalls, "Window")
		content.MouseDownCallback = func(_ geom.Point, button, _ int, _ mod.Modifiers) bool {
			if button == unison.ButtonRight {
				rightPresses++
			}
			return false
		}
	})
	var origin geom.Point
	screen.Do(func() { origin = wnd.ContentRect().Point })
	for _, one := range []struct {
		name string
		at   geom.Point
	}{
		{at: onTitle(firstTitle), name: "title"},
		{at: onPadding(firstTab), name: "padding"},
	} {
		screen.ClickWith(one.at, unison.ButtonRight, mod.None)
		menus, _ = cmOpenMenu(c, screen, wnd)
		c.Equal(0, menus, "a right-click on a lone tab's %s opens nothing", one.name)
		c.Equal(0, cmCount(screen, &contentCalls), "not even the menu of the panel the dock sits in")
	}
	var presses int
	screen.Do(func() { presses = rightPresses })
	c.Equal(2, presses, "and the presses are passed on to the tab's ancestors rather than swallowed by the tab")
	var closed bool
	if len(closers) == 1 {
		// The close button ignores a right press, so it is passed on to the tab's ancestors too.
		screen.ClickWith(closers[0].Bounds.Center().Add(origin), unison.ButtonRight, mod.None)
		menus, _ = cmOpenMenu(c, screen, wnd)
		c.Equal(0, menus, "a right-click on the close button of a lone tab opens nothing")
		c.Equal(0, cmCount(screen, &contentCalls))
		screen.Do(func() {
			presses = rightPresses
			closed = first.closed
		})
		c.Equal(3, presses, "and is passed on to the tab's ancestors as well")
		c.False(closed, "rather than closing the dockable")
	}

	// A right press made while the left button is held on the lone tab, and let go in either order, does not leave the
	// tab pressed: the tab shows the open hand again once every button is up.
	screen.MouseMove(firstPoint, mod.None)
	c.True(screen.Cursor() == unison.OpenHandCursor(), "the test needs the tab's own cursor over it")
	for _, first := range []int{unison.ButtonLeft, unison.ButtonRight} {
		second := unison.ButtonRight
		if first == unison.ButtonRight {
			second = unison.ButtonLeft
		}
		screen.MouseDown(firstPoint, unison.ButtonLeft, mod.None)
		c.True(screen.Cursor() == unison.ClosedHandCursor(), "a left press takes hold of the tab")
		screen.MouseDown(firstPoint, unison.ButtonRight, mod.None)
		screen.MouseUp(firstPoint, first, mod.None)
		if first == unison.ButtonRight {
			c.True(screen.Cursor() == unison.ClosedHandCursor(),
				"the right release does not let go of the tab while the left button is still held")
		} else {
			c.True(screen.Cursor() == unison.OpenHandCursor(), "the left release lets go of the tab")
		}
		screen.MouseUp(firstPoint, second, mod.None)
		c.True(screen.Cursor() == unison.OpenHandCursor(),
			"the tab lets go once every button is up, button %d released first", first)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuTableCellWidget verifies that a right-click on a widget with a menu in a table cell opens that menu
// rather than the table's and focuses the widget, after which shift+F10 asks the widget too; and that a right-click on
// a cell without one, on a widget that cannot take the focus (which leaves the focus where it is), in a hidden cell,
// or outside the cell's frame is for the table's menu.
func TestContextMenuTableCellWidget(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "abc", false)
			wnd, _ = newEditWindow(t, e, false)
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	// The table has no menu yet; the field's own opens.
	screen.ClickWith(cellCenter(screen, e.table, 1, 0), unison.ButtonRight, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click on a field in a cell opens the field's menu")
	c.True(slices.Contains(items, "Copy"), "which offers to copy the field's text, not %v", items)
	s := e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused, "the field takes the focus its menu's commands act on")
	c.True(s.installed, "and its cell stays attached to the table")
	c.Equal(1, s.focusRow)
	c.Equal(0, s.focusCol)
	c.Equal([]int{1}, axTableSelectedIndexes(screen, e.table), "the person is put on the field's row")
	cmCloseMenu(c, screen, wnd)

	// With a menu on the table too, a right-click on a field still opens the field's.
	var tableCalls, fieldCalls, stuckCalls cmRequests
	screen.Do(func() {
		e.table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
		e.fields[2][1].ContextMenuCallback = cmMenu(&fieldCalls, cmFieldTitle)
		e.fields[0][0].ContextMenuCallback = cmMenu(&stuckCalls, "Stuck")
		e.fields[0][0].SetFocusable(false)
	})
	screen.ClickWith(cellCenter(screen, e.table, 2, 1), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{cmFieldTitle}, items, "the field's menu opens rather than the table's")
	c.Equal(0, cmCount(screen, &tableCalls), "and the table is not asked")
	var expected geom.Point
	screen.Do(func() {
		expected = e.fields[2][1].PointFromRoot(e.table.PointToRoot(e.table.CellFrame(2, 1).Center()))
	})
	c.Equal(expected, cmLast(screen, &fieldCalls), "the menu opens under the pointer, in the field's coordinates")
	s = e.snapshot(c, screen, 2, 1)
	c.True(s.fieldFocused, "the focus moves to the field that was right-clicked")
	c.Equal(2, s.focusRow)
	c.Equal(1, s.focusCol)
	c.Equal([]int{2}, axTableSelectedIndexes(screen, e.table))
	cmCloseMenu(c, screen, wnd)

	// With the focus in the field, the keyboard asks the field too.
	screen.KeyPress(unison.KeyF10, mod.Shift)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "shift+F10 with the focus in the field opens its menu")
	c.Equal([]string{cmFieldTitle}, items)
	c.Equal(2, cmCount(screen, &fieldCalls))
	c.Equal(0, cmCount(screen, &tableCalls))
	cmCloseMenu(c, screen, wnd)

	// A right-click on a cell holding no widget with a menu is for the table's, and selects that row.
	screen.ClickWith(cellCenter(screen, e.table, 1, 2), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{cmTableTitle}, items, "a right-click on a cell with no menu of its own opens the table's")
	c.Equal(1, cmCount(screen, &tableCalls))
	c.Equal([]int{1}, axTableSelectedIndexes(screen, e.table), "which selects the row under the pointer")
	s = e.snapshot(c, screen, 2, 1)
	c.False(s.fieldFocused, "the table takes the focus back from the field")
	c.True(s.tableFocused)
	c.Equal(-1, s.focusRow)
	cmCloseMenu(c, screen, wnd)

	// A widget that cannot take the focus cannot keep its cell attached until the release, so it is passed over, and
	// nothing is focused in its place: the table, which holds the focus, keeps it throughout rather than losing it to
	// nothing and taking it back for its menu.
	var tableLostFocus int
	screen.Do(func() {
		lost := e.table.LostFocusCallback
		e.table.LostFocusCallback = func() {
			tableLostFocus++
			if lost != nil {
				lost()
			}
		}
	})
	screen.ClickWith(cellCenter(screen, e.table, 0, 0), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{cmTableTitle}, items, "a widget that cannot take the focus is passed over for the table's menu")
	c.Equal(0, cmCount(screen, &stuckCalls), "and is not asked")
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table))
	var lostCount int
	screen.Do(func() { lostCount = tableLostFocus })
	c.Equal(0, lostCount, "and the table keeps the focus throughout")
	cmCloseMenu(c, screen, wnd)

	// The same in a table that cannot hold the focus while a field in another cell holds it: the field keeps the
	// focus, with its cell attached, since nothing is focused in the passed-over widget's place.
	screen.Do(func() { e.table.SetFocusable(false) })
	screen.Click(cellCenter(screen, e.table, 2, 1))
	s = e.snapshot(c, screen, 2, 1)
	c.True(s.fieldFocused, "the test needs a field in another cell focused")
	c.True(s.installed, "with its cell attached")
	screen.ClickWith(cellCenter(screen, e.table, 0, 0), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{cmTableTitle}, items,
		"a widget that cannot take the focus is passed over for the table's menu in a table that cannot hold it too")
	c.Equal(0, cmCount(screen, &stuckCalls), "and is not asked")
	s = e.snapshot(c, screen, 2, 1)
	c.True(s.fieldFocused, "the field in the other cell keeps the focus")
	c.True(s.installed, "with its cell still attached")
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table), "and the row under the pointer is selected for the menu")
	cmCloseMenu(c, screen, wnd)
	screen.Do(func() {
		e.table.SetFocusable(true)
		e.table.RequestFocus()
	})
	s = e.snapshot(c, screen, 2, 1)
	c.True(s.tableFocused, "the test needs the table focused again")

	// A hidden cell is not drawn, so a right-click where it would be is for the table's menu, though the widget with
	// a menu within it is not hidden itself and would be found under the pointer were the cell drawn: the widget is
	// neither asked nor focused.
	var hiddenCalls cmRequests
	var hiddenWrapper *unison.Panel
	var hiddenBuild func(row, col int) unison.Paneler
	screen.Do(func() {
		e.fields[1][1].ContextMenuCallback = cmMenu(&hiddenCalls, "Hidden")
		hiddenWrapper = unison.NewPanel()
		hiddenWrapper.SetLayout(&unison.FlexLayout{Columns: 1})
		hiddenWrapper.AddChild(e.fields[1][1])
		hiddenWrapper.Hidden = true
		hiddenBuild = e.rows[1].cellFactory
		e.rows[1].cellFactory = func(r, col int) unison.Paneler {
			if col == 1 {
				return hiddenWrapper
			}
			return hiddenBuild(r, col)
		}
	})
	screen.ClickWith(cellCenter(screen, e.table, 1, 1), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{cmTableTitle}, items, "a widget in a hidden cell is passed over for the table's menu")
	c.Equal(0, cmCount(screen, &hiddenCalls), "and is not asked")
	s = e.snapshot(c, screen, 1, 1)
	c.False(s.fieldFocused, "nor focused")
	c.Equal([]int{1}, axTableSelectedIndexes(screen, e.table))
	cmCloseMenu(c, screen, wnd)
	screen.Do(func() {
		e.rows[1].cellFactory = hiddenBuild
		e.fields[1][1].RemoveFromParent()
	})

	// A right-click in the padding 2px left of a field's cell frame is for the table's menu; the field is neither asked
	// nor focused.
	cellPoint := func(row, col int, dx float32) geom.Point {
		var offset geom.Point
		screen.Do(func() {
			frame := e.table.CellFrame(row, col)
			offset = geom.NewPoint(frame.X+dx, frame.CenterY()).Sub(e.table.ContentRect(false).Point)
		})
		return screen.PanelPoint(e.table, offset)
	}
	tableAsked := cmCount(screen, &tableCalls)
	screen.ClickWith(cellPoint(1, 0, -2), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click in the padding beside a field opens a menu")
	c.Equal([]string{cmTableTitle}, items, "the table's, since the point is not within the field")
	c.Equal(tableAsked+1, cmCount(screen, &tableCalls))
	s = e.snapshot(c, screen, 1, 0)
	c.False(s.fieldFocused, "and the field is not focused")
	c.True(s.tableFocused)
	c.Equal(-1, s.focusRow)
	c.Equal([]int{1}, axTableSelectedIndexes(screen, e.table), "the row under the pointer is selected for it")
	cmCloseMenu(c, screen, wnd)

	// The indent of a child row in the hierarchy column is the table's own: a right-click 8px left of the child's cell
	// frame is for the table's menu.
	var child *unison.Field
	screen.Do(func() {
		child = unison.NewField()
		child.SetText("child")
		row := newTableTestRow("child")
		row.cellFactory = func(_, col int) unison.Paneler {
			if col == 0 {
				return child
			}
			return unison.NewPanel()
		}
		e.rows[2].SetChildren([]*tableTestRow{row})
		e.rows[2].open = true
		e.table.SyncToModel()
	})
	screen.Sync()
	var childFrame, parentFrame geom.Rect
	screen.Do(func() {
		childFrame = e.table.CellFrame(3, 0)
		parentFrame = e.table.CellFrame(2, 0)
	})
	c.True(childFrame.X > parentFrame.X, "the child's cell is indented")
	tableAsked = cmCount(screen, &tableCalls)
	screen.ClickWith(cellPoint(3, 0, -8), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click in the indent of a child row opens a menu")
	c.Equal([]string{cmTableTitle}, items, "the table's")
	c.Equal(tableAsked+1, cmCount(screen, &tableCalls))
	var childFocused bool
	screen.Do(func() { childFocused = child.Focused() })
	c.False(childFocused, "and the child's field is not focused")
	c.Equal([]int{3}, axTableSelectedIndexes(screen, e.table), "the child row is selected for it")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuTableCellFreshWidget verifies that a right-click on a widget with a menu that the row builds afresh on
// every call is passed over for the table's menu, without the widget being focused or asked, since the draw between
// the press and the release would replace the widget and detach it before the release.
func TestContextMenuTableCellFreshWidget(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	var freshCalls, tableCalls cmRequests
	var built, gained int
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			row := newTableTestRow("fresh")
			row.cellFactory = func(_, _ int) unison.Paneler {
				built++
				field := unison.NewField()
				field.SetText("abc")
				field.ContextMenuCallback = cmMenu(&freshCalls, "Fresh")
				gainedFocus := field.GainedFocusCallback
				field.GainedFocusCallback = func() {
					gained++
					gainedFocus()
				}
				return field
			}
			model := &unison.SimpleTableModel[*tableTestRow]{}
			model.SetRootRows([]*tableTestRow{row})
			table = unison.NewTable[*tableTestRow](model)
			table.Columns = []unison.ColumnInfo{{ID: 0, Current: 120}}
			table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			table.SyncToModel()
			table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			wnd = newHeadlessWindow(t, "fresh widget", geom.NewRect(20, 20, 400, 300), axColumn(table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(table)
	c.NotNil(wnd)
	var builtBefore int
	screen.Do(func() { builtBefore = built })
	screen.Sync()
	var builtAfter int
	screen.Do(func() { builtAfter = built })
	c.True(builtAfter > builtBefore || builtBefore > 1,
		"the test needs a row that builds the widget afresh on every call")

	at := cellCenter(screen, table, 0, 0)
	screen.MouseDown(at, unison.ButtonRight, mod.None)
	// The draw between the press and the release asks the row for the cell again, as a draw does.
	screen.Do(func() { wnd.FlushDrawing() })
	screen.MouseUp(at, unison.ButtonRight, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click on a widget the row builds afresh on every call opens a menu")
	c.Equal([]string{cmTableTitle}, items, "the table's, since the widget would be gone by the release")
	c.Equal(0, cmCount(screen, &freshCalls), "and the widget is not asked")
	var focus *unison.Panel
	var gainedCount int
	screen.Do(func() {
		focus = wnd.CurrentFocus()
		gainedCount = gained
	})
	c.True(focus == table.AsPanel(), "the table takes the focus for its menu")
	c.Equal(0, gainedCount, "no widget having been given it only to lose it")
	c.Equal([]int{0}, axTableSelectedIndexes(screen, table), "and the row is selected for the menu")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuTableCellWrapper verifies, for a wrapper with a menu that the row builds afresh around a memoized
// field on every call, that a right-click on the field without a menu or on the wrapper's own area opens the table's
// menu, since the wrapper is not under the pointer or would be detached before the release; that the field's own menu
// still opens, since the field moves into each new wrapper; and that an assistive technology is offered the wrapper's
// menu and the field's alike, each opening where its panel is drawn.
func TestContextMenuTableCellWrapper(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	var wrapperCalls, tableCalls, fieldCalls cmRequests
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(2, 1, "abc", true)
			for _, row := range e.rows {
				build := row.cellFactory
				row.cellFactory = func(r, col int) unison.Paneler {
					p := build(r, col)
					if col == 0 {
						p.AsPanel().ContextMenuCallback = cmMenu(&wrapperCalls, cmWrapperTitle)
					}
					return p
				}
			}
			for _, fields := range e.fields {
				for _, field := range fields {
					field.ContextMenuCallback = nil
					// The field keeps to its own width at the left of the wrapper, leaving the wrapper's own area
					// beside it.
					field.SetLayoutData(&unison.FlexLayoutData{VAlign: align.Fill, VGrab: true})
				}
			}
			e.table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			wnd, _ = newEditWindow(t, e, false)
		}))
	c.NotNil(e)
	c.NotNil(wnd)
	// cellPoint returns the screen position of a point in a cell's own coordinates.
	cellPoint := func(row, col int, local geom.Point) geom.Point {
		var offset geom.Point
		screen.Do(func() {
			offset = e.table.CellFrame(row, col).Point.Add(local).Sub(e.table.ContentRect(false).Point)
		})
		return screen.PanelPoint(e.table, offset)
	}
	var fieldFrame, cellFrame geom.Rect
	screen.Do(func() {
		fieldFrame = e.fields[1][0].FrameRect()
		cellFrame = e.table.CellFrame(1, 0)
	})
	c.True(fieldFrame.Right()+4 < cellFrame.Width, "the test needs the wrapper to show beside the field: %v in %v",
		fieldFrame, cellFrame)
	onField := fieldFrame.Center()
	besideField := geom.NewPoint((fieldFrame.Right()+cellFrame.Width)/2, fieldFrame.CenterY())

	screen.ClickWith(cellPoint(1, 0, onField), unison.ButtonRight, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click on a field without a menu, in a wrapper with one, opens a menu")
	c.Equal([]string{cmTableTitle}, items, "the table's rather than the wrapper's, which is not under the pointer")
	c.Equal(0, cmCount(screen, &wrapperCalls), "so the wrapper is not asked")
	s := e.snapshot(c, screen, 1, 0)
	c.False(s.fieldFocused, "and the field is not focused")
	c.True(s.tableFocused)
	c.Equal(-1, s.focusRow)
	c.Equal([]int{1}, axTableSelectedIndexes(screen, e.table), "the row is selected for the table's menu")
	cmCloseMenu(c, screen, wnd)

	screen.ClickWith(cellPoint(1, 0, besideField), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click on the wrapper itself opens a menu")
	c.Equal([]string{cmTableTitle}, items, "the table's, since the wrapper would be detached before the release")
	c.Equal(0, cmCount(screen, &wrapperCalls), "so the wrapper is not asked")
	s = e.snapshot(c, screen, 1, 0)
	c.False(s.fieldFocused, "and the field is not focused")
	c.True(s.tableFocused)
	c.Equal(-1, s.focusRow)
	cmCloseMenu(c, screen, wnd)

	screen.Do(func() { e.fields[0][0].ContextMenuCallback = cmMenu(&fieldCalls, cmFieldTitle) })
	screen.ClickWith(cellPoint(0, 0, onField), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click on a field with a menu of its own opens a menu")
	c.Equal([]string{cmFieldTitle}, items, "the field's, though its wrapper is built afresh every time")
	c.Equal(1, cmCount(screen, &fieldCalls))
	c.Equal(0, cmCount(screen, &wrapperCalls))
	s = e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the field takes the focus")
	c.True(s.wrapped, "and sits in a wrapper attached to the table")
	c.Equal(0, s.focusRow)
	c.Equal(0, s.focusCol)
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table))
	cmCloseMenu(c, screen, wnd)

	// An assistive technology is offered the wrapper's menu too, since a request opens it at once, and the menu is put
	// where the wrapper is drawn, though drawing replaces the wrapper before the menu pops up. The table is moved away
	// from the window's origin first, since a menu put at the replaced wrapper's position within the table would
	// otherwise be in the right place by chance.
	screen.Do(func() {
		wnd.Content().SetBorder(unison.NewEmptyBorder(geom.Insets{Top: 50, Left: 30}))
		wnd.Content().MarkForLayoutAndRedraw()
		e.table.RequestFocus()
	})
	screen.Sync()
	var tableOrigin geom.Point
	screen.Do(func() { tableOrigin = e.table.PointToRoot(geom.Point{}) })
	c.True(tableOrigin.X >= 30 && tableOrigin.Y >= 50, "the test needs the table away from the window's origin: %v",
		tableOrigin)
	tree := screen.AccessibilityTree(wnd)
	rows := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	wrapped := cmCellContent(tree, axMustNode(c, rows["r1"]), 0)
	withMenu := cmCellContent(tree, axMustNode(c, rows["r0"]), 0)
	c.Equal(1, len(wrapped))
	c.Equal(1, len(withMenu))
	if len(wrapped) != 1 || len(withMenu) != 1 {
		return
	}
	c.Equal(role.Group, wrapped[0].Role, "the cell is described as its wrapper")
	c.True(wrapped[0].Actions.Has(accessibility.ShowContextMenu),
		"a wrapper built afresh offers its menu to an assistive technology")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   wrapped[0].ID,
		Action: accessibility.ShowContextMenu,
	}), "which opens on request")
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{cmWrapperTitle}, items)
	c.Equal(1, cmCount(screen, &wrapperCalls))
	s = e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused, "the field within the wrapper takes the focus")
	c.True(s.wrapped, "and sits in a wrapper attached to the table")
	var expected geom.Point
	screen.Do(func() {
		wrapper := e.fields[1][0].Parent()
		expected = wrapper.PointToRoot(wrapper.DefaultContextMenuAnchor())
	})
	menuNodes := axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu)
	c.Equal(1, len(menuNodes))
	if len(menuNodes) == 1 {
		c.Equal(expected, menuNodes[0].Bounds.Point,
			"the menu is put where the wrapper is drawn, in the window's coordinates")
	}
	cmCloseMenu(c, screen, wnd)

	// The field with a menu of its own is described within its wrapper, and its own menu opens beneath its caret.
	c.Equal(role.Group, withMenu[0].Role)
	fieldNodes := axChildNodes(tree, withMenu[0])
	c.Equal(1, len(fieldNodes), "the wrapper holds the field")
	if len(fieldNodes) != 1 {
		return
	}
	c.True(fieldNodes[0].Actions.Has(accessibility.ShowContextMenu), "which offers its own menu")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   fieldNodes[0].ID,
		Action: accessibility.ShowContextMenu,
	}), "and opens it on request")
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{cmFieldTitle}, items)
	c.Equal(2, cmCount(screen, &fieldCalls))
	s = e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the field takes the focus")
	c.True(s.wrapped, "and sits in a wrapper attached to the table")
	var anchor geom.Point
	screen.Do(func() { anchor = e.fields[0][0].ContextMenuAnchor() })
	c.Equal(anchor, cmLast(screen, &fieldCalls), "the menu opens beneath the field's caret")
	cmMenuAt(c, screen, wnd, e.fields[0][0], anchor, "and is put there, with the field where it is drawn")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// cmCellContent returns the nodes described within the given column's cell of a table row.
func cmCellContent(tree *accessibility.Tree, row *accessibility.Node, col int) []*accessibility.Node {
	for _, cell := range axChildNodes(tree, row) {
		if cell.Role == role.Cell && cell.ColumnIndex == col {
			return axChildNodes(tree, cell)
		}
	}
	return nil
}

// TestContextMenuTableCellAccessibility verifies that a widget in a table cell offers its menu to an assistive
// technology only when it, or something within it, can take the focus; that a request focuses it before the menu opens
// unless the focus is already within it, as a right-click does; that a request for a widget that cannot take the focus,
// or whose focus does not stay, is refused; and that nothing within a list row offers a menu.
func TestContextMenuTableCellAccessibility(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var list *unison.List[string]
	var listFields []*unison.Field
	var wrapper *unison.Panel
	var extra *unison.Field
	var tableCalls, stuckCalls, wrapperCalls, fleetingCalls, listCalls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "abc", false)
			e.table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			// A field that can take the focus while the window is described, and then no longer can, so that the
			// request made against its description reaches the table, which must refuse it itself.
			e.fields[0][0].ContextMenuCallback = cmMenu(&stuckCalls, "Stuck")
			// A field whose focus does not stay: it hands the focus to the table as soon as it gains it.
			e.fields[1][1].ContextMenuCallback = cmMenu(&fleetingCalls, "Fleeting")
			fleetingGained := e.fields[1][1].GainedFocusCallback
			e.fields[1][1].GainedFocusCallback = func() {
				fleetingGained()
				e.table.RequestFocus()
			}
			// A wrapper holding two fields, the second keeping to its own width so that the wrapper has an area of its
			// own beside it.
			extra = unison.NewField()
			extra.SetText("extra")
			wrapper = unison.NewPanel()
			wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
			wrapper.AddChild(e.fields[2][1])
			wrapper.AddChild(extra)
			wrapper.ContextMenuCallback = cmMenu(&wrapperCalls, cmWrapperTitle)
			e.fields[2][1].ContextMenuCallback = nil
			build := e.rows[2].cellFactory
			e.rows[2].cellFactory = func(r, col int) unison.Paneler {
				if col == 1 {
					return wrapper
				}
				return build(r, col)
			}
			list = unison.NewList[string]()
			list.ContextMenuCallback = cmMenu(&listCalls, "List")
			factory := &axListCellFactory{height: 24}
			for range 2 {
				field := unison.NewField()
				field.SetText("row")
				listFields = append(listFields, field)
				factory.cells = append(factory.cells, field)
			}
			list.Factory = factory
			list.Append("Zero", "One")
			e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			wnd = newHeadlessWindow(t, "cells", geom.NewRect(10, 10, 500, 500), axColumn(e.table, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	screen.Do(func() { e.table.RequestFocus() })
	tree := screen.AccessibilityTree(wnd)
	rows := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	stuck := cmCellContent(tree, axMustNode(c, rows["r0"]), 0)
	field := cmCellContent(tree, axMustNode(c, rows["r1"]), 0)
	fleeting := cmCellContent(tree, axMustNode(c, rows["r1"]), 1)
	wrapped := cmCellContent(tree, axMustNode(c, rows["r2"]), 1)
	c.Equal(1, len(stuck))
	c.Equal(1, len(field))
	c.Equal(1, len(fleeting))
	c.Equal(1, len(wrapped))
	if len(stuck) != 1 || len(field) != 1 || len(fleeting) != 1 || len(wrapped) != 1 {
		return
	}
	c.True(stuck[0].Actions.Has(accessibility.ShowContextMenu), "a field in a cell offers its menu while it can take "+
		"the focus")
	c.True(field[0].Actions.Has(accessibility.ShowContextMenu), "a field in a cell offers its menu")
	c.Equal(role.Group, wrapped[0].Role)
	c.True(wrapped[0].Actions.Has(accessibility.ShowContextMenu),
		"and so does a wrapper that cannot take the focus around a field that can")
	c.Equal(2, len(axChildNodes(tree, wrapped[0])), "the wrapper holds both fields")

	// A request for the menu of a widget that cannot take the focus is refused, and so is one for a widget whose focus
	// does not stay, since neither can keep its cell attached; neither is asked for its menu. The first is made
	// against the description from when the widget could still take the focus, since the driver forwards only what
	// the description offers, and taking the focus away describes nothing again, so the table refuses it itself.
	screen.Do(func() { e.fields[0][0].SetFocusable(false) })
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   stuck[0].ID,
		Action: accessibility.ShowContextMenu,
	}), "a widget that cannot take the focus refuses")
	c.Equal(0, cmCount(screen, &stuckCalls), "and is not asked")
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.tableFocused, "the table keeps the focus")
	c.False(s.fieldFocused)
	c.False(s.installed, "and the cell was handed back")
	tree = screen.AccessibilityTree(wnd)
	rows = axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	stuck = cmCellContent(tree, axMustNode(c, rows["r0"]), 0)
	c.Equal(1, len(stuck))
	if len(stuck) == 1 {
		c.False(stuck[0].Actions.Has(accessibility.ShowContextMenu),
			"a widget that cannot take the focus and holds nothing that can does not offer its menu")
	}
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   fleeting[0].ID,
		Action: accessibility.ShowContextMenu,
	}), "a widget whose focus does not stay refuses")
	c.Equal(0, cmCount(screen, &fleetingCalls))
	s = e.snapshot(c, screen, 1, 1)
	c.True(s.tableFocused, "the focus is where the widget sent it")
	c.False(s.installed, "and the cell was handed back")
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus)

	// A right-click on the widget whose focus does not stay is left to the table: its menu opens, for the row under
	// the pointer, and the widget is not asked.
	screen.ClickWith(cellCenter(screen, e.table, 1, 1), unison.ButtonRight, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click on a widget whose focus does not stay opens a menu")
	c.Equal([]string{cmTableTitle}, items, "the table's")
	c.Equal(1, cmCount(screen, &tableCalls))
	c.Equal(0, cmCount(screen, &fleetingCalls), "and the widget is not asked")
	c.Equal([]int{1}, axTableSelectedIndexes(screen, e.table), "the row under the pointer is selected for it")
	s = e.snapshot(c, screen, 1, 1)
	c.True(s.tableFocused, "the focus is where the widget sent it")
	c.False(s.installed, "and the cell was handed back")
	cmCloseMenu(c, screen, wnd)

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   field[0].ID,
		Action: accessibility.ShowContextMenu,
	}), "asking a field in a cell for its menu opens it")
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.True(slices.Contains(items, "Copy"), "the field's own menu, not %v", items)
	c.Equal(1, cmCount(screen, &tableCalls), "and the table is not asked again")
	s = e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused, "the field takes the focus its menu's commands act on")
	c.True(s.installed, "and its cell stays attached to the table")
	c.Equal(1, s.focusRow)
	c.Equal(0, s.focusCol)
	c.Equal([]int{1}, axTableSelectedIndexes(screen, e.table), "the person is put on the field's row")
	cmCloseMenu(c, screen, wnd)

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   wrapped[0].ID,
		Action: accessibility.ShowContextMenu,
	}), "asking the wrapper for its menu opens it")
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{cmWrapperTitle}, items)
	c.Equal(1, cmCount(screen, &wrapperCalls))
	c.Equal(1, cmCount(screen, &tableCalls))
	s = e.snapshot(c, screen, 2, 1)
	c.True(s.fieldFocused, "the first field within the wrapper takes the focus")
	c.True(s.wrapped, "and the wrapper stays attached to the table")
	c.Equal(2, s.focusRow)
	c.Equal(1, s.focusCol)
	c.Equal([]int{2}, axTableSelectedIndexes(screen, e.table), "the person is put on the wrapper's row")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, cmCount(screen, &stuckCalls))

	// With the focus already within the wrapper, on its second field, a request for the wrapper's menu and a
	// right-click on the wrapper's own area both leave the focus there.
	screen.AccessibilityTree(wnd)
	extraNode := axMustNode(c, screen.AccessibilityNodeFor(extra))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   extraNode.ID,
		Action: accessibility.Focus,
	}), "the test needs the focus on the wrapper's second field")
	extraFocused := func() bool {
		var focused bool
		screen.Do(func() { focused = extra.Focused() })
		return focused
	}
	c.True(extraFocused())
	var besideExtra geom.Point
	var room bool
	screen.Do(func() {
		frame := extra.FrameRect()
		room = frame.Right()+4 < wrapper.ContentRect(false).Width
		besideExtra = geom.NewPoint((frame.Right()+wrapper.ContentRect(false).Width)/2, frame.CenterY())
	})
	c.True(room, "the test needs the wrapper to show beside its second field")
	tree = screen.AccessibilityTree(wnd)
	wrapped = cmCellContent(tree, axMustNode(c, axTableRows(tree,
		axMustNode(c, screen.AccessibilityNodeFor(e.table)))["r2"]), 1)
	c.Equal(1, len(wrapped))
	if len(wrapped) != 1 {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   wrapped[0].ID,
		Action: accessibility.ShowContextMenu,
	}), "asking the wrapper for its menu opens it")
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.Equal([]string{cmWrapperTitle}, items)
	c.True(extraFocused(), "and leaves the focus on the second field, since it is already within the wrapper")
	cmCloseMenu(c, screen, wnd)
	screen.ClickWith(cmScreenPoint(c, screen, wrapper, besideExtra), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click on the wrapper's own area opens its menu")
	c.Equal([]string{cmWrapperTitle}, items)
	c.True(extraFocused(), "and leaves the focus on the second field as well")
	c.Equal(3, cmCount(screen, &wrapperCalls))
	cmCloseMenu(c, screen, wnd)

	listRows := axChildNodes(tree, axMustNode(c, screen.AccessibilityNodeFor(list)))
	c.Equal(2, len(listRows))
	for _, row := range listRows {
		c.True(row.Actions.Has(accessibility.ShowContextMenu), "a row of a list with a menu offers the list's menu")
		content := axChildNodes(tree, row)
		c.Equal(1, len(content), "the row is drawn with a field")
		for _, node := range content {
			c.False(node.Actions.Has(accessibility.ShowContextMenu), "which does not offer its own menu")
		}
	}

	// A field in a cell of a disabled table does not offer its menu, since the table refuses every request on its
	// behalf.
	screen.Do(func() {
		e.table.RequestFocus()
		e.table.SetEnabled(false)
	})
	tree = screen.AccessibilityTree(wnd)
	rows = axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	field = cmCellContent(tree, axMustNode(c, rows["r1"]), 0)
	c.Equal(1, len(field))
	if len(field) == 1 {
		c.False(field[0].Actions.Has(accessibility.ShowContextMenu),
			"a field in a cell of a disabled table does not offer its menu")
		c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   field[0].ID,
			Action: accessibility.ShowContextMenu,
		}), "and a request for it is refused")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuRightClickKeepsSelectionWithoutMenu verifies that, on a Table and a List with no contextual menu, a
// right-click on one of several selected rows keeps the whole selection and a right double-click is not a double-click.
func TestContextMenuRightClickKeepsSelectionWithoutMenu(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var list *unison.List[string]
	var tableDoubles, listDoubles int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(4)...)
			table.DoubleClickCallback = func() { tableDoubles++ }
			list = unison.NewList[string]()
			list.Append("Zero", "One", "Two", "Three")
			list.DoubleClickCallback = func() { listDoubles++ }
			wnd = newHeadlessWindow(t, "no menu", geom.NewRect(10, 10, 600, 600), axColumn(table, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)

	screen.Do(func() {
		table.SelectByIndex(0, 1, 2)
		list.Select(false, 0, 1, 2)
	})
	tablePoint := axTableRowPoint(screen, table, 1)
	var listPoint geom.Point
	screen.Do(func() {
		rect := list.RowRect(1)
		listPoint = geom.NewPoint(rect.X+10, rect.CenterY())
	})
	listPoint = cmScreenPoint(c, screen, list, listPoint)

	screen.ClickWith(tablePoint, unison.ButtonRight, mod.None)
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a right-click on a table with no menu opens nothing")
	c.Equal([]int{0, 1, 2}, axTableSelectedIndexes(screen, table),
		"and one on a selected row keeps the whole selection once released")
	screen.ClickWith(listPoint, unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a right-click on a list with no menu opens nothing")
	c.Equal([]int{0, 1, 2}, selectedIndexesOn(screen, list), "and keeps the whole selection as well")

	// A left click on the same row narrows the selection to it.
	screen.Click(tablePoint)
	c.Equal([]int{1}, axTableSelectedIndexes(screen, table), "a left click narrows the table's selection to the row")
	screen.Click(listPoint)
	c.Equal([]int{1}, selectedIndexesOn(screen, list), "and the list's")

	// A right double-click on a selected row is not a double-click.
	for _, pt := range []geom.Point{tablePoint, listPoint} {
		for range 2 {
			screen.MouseDown(pt, unison.ButtonRight, mod.None)
			screen.MouseUp(pt, unison.ButtonRight, mod.None)
		}
	}
	var tableCount, listCount int
	screen.Do(func() {
		tableCount = tableDoubles
		listCount = listDoubles
	})
	c.Equal(0, tableCount, "a right double-click on a selected row does not call the table's DoubleClickCallback")
	c.Equal(0, listCount, "nor the list's")
	c.Equal([]int{1}, axTableSelectedIndexes(screen, table))
	c.Equal([]int{1}, selectedIndexesOn(screen, list))
	screen.DoubleClick(tablePoint)
	screen.DoubleClick(listPoint)
	screen.Do(func() {
		tableCount = tableDoubles
		listCount = listDoubles
	})
	c.Equal(1, tableCount, "a left double-click does")
	c.Equal(1, listCount)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuListRightDragFromSelection verifies that a right-drag from an already selected row extends the
// selection from that row, with or without a menu, and that a right-click on one row of a larger selection keeps the
// selection but makes that row the anchor.
func TestContextMenuListRightDragFromSelection(t *testing.T) {
	c := check.New(t)
	var withMenu, without *unison.List[string]
	var calls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			withMenu = unison.NewList[string]()
			withMenu.Append("Zero", "One", "Two", "Three", "Four", "Five")
			withMenu.ContextMenuCallback = cmMenu(&calls, "List")
			without = unison.NewList[string]()
			without.Append("Zero", "One", "Two", "Three", "Four", "Five")
			wnd = newHeadlessWindow(t, "list right-drag", geom.NewRect(10, 10, 400, 500), axColumn(withMenu, without))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)

	// rowPoint returns a point within a row, in the list's own coordinates. UI thread only.
	rowPoint := func(l *unison.List[string], row int) geom.Point {
		rect := l.RowRect(row)
		return geom.NewPoint(rect.X+10, rect.CenterY())
	}
	// dragFromSelection selects rows 0-2, leaving the anchor on row 0, drags with the given button from row 2 to row 4,
	// and returns the resulting selection.
	dragFromSelection := func(l *unison.List[string], button int) []int {
		var from, over, to geom.Point
		var anchor int
		screen.Do(func() {
			l.Select(false, 0, 1, 2)
			anchor = l.Anchor()
			from = rowPoint(l, 2)
			over = rowPoint(l, 3)
			to = rowPoint(l, 4)
		})
		c.Equal(0, anchor, "selecting the rows leaves the anchor on the first of them")
		screen.MouseDown(cmScreenPoint(c, screen, l, from), button, mod.None)
		screen.MouseMove(cmScreenPoint(c, screen, l, over), mod.None)
		screen.MouseMove(cmScreenPoint(c, screen, l, to), mod.None)
		screen.MouseUp(cmScreenPoint(c, screen, l, to), button, mod.None)
		return selectedIndexesOn(screen, l)
	}

	c.Equal([]int{2, 3, 4}, dragFromSelection(without, unison.ButtonLeft),
		"a left-drag from a selected row selects from that row to where it ends")
	c.Equal([]int{2, 3, 4}, dragFromSelection(without, unison.ButtonRight),
		"and so does a right-drag on a list without a menu")
	c.Equal([]int{2, 3, 4}, dragFromSelection(withMenu, unison.ButtonRight),
		"and one on a list with a menu, which has the press handed back to it")
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a right-drag opens no menu")
	c.Equal(0, cmCount(screen, &calls), "nor even asks for one")

	var point geom.Point
	var anchor int
	screen.Do(func() {
		withMenu.Select(false, 0, 1, 2)
		point = rowPoint(withMenu, 1)
	})
	screen.ClickWith(cmScreenPoint(c, screen, withMenu, point), unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click on a row opens the menu")
	c.Equal([]int{0, 1, 2}, selectedIndexesOn(screen, withMenu), "and keeps the whole selection")
	screen.Do(func() { anchor = withMenu.Anchor() })
	c.Equal(1, anchor, "while making the row right-clicked on the anchor")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuRowRequestWithoutMenu verifies that a request for the menu of a table or list row, made against a
// description published before the widget's menu was removed, is refused without changing the selection, notifying the
// application or moving the focus, and that the same requests are carried out once the menu is back.
func TestContextMenuRowRequestWithoutMenu(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var list, quiet *unison.List[string]
	var tableCalls, listCalls cmRequests
	var tableChanges, listChanges int
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			quiet = unison.NewList[string]()
			quiet.Append("Quiet")
			table = axNewTable(flatRows(4)...)
			table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			table.SelectByIndex(0)
			table.SelectionChangedCallback = func() { tableChanges++ }
			list = unison.NewList[string]()
			list.Append("Zero", "One", "Two", "Three")
			list.ContextMenuCallback = cmMenu(&listCalls, "List")
			list.Select(false, 0)
			list.NewSelectionCallback = func() { listChanges++ }
			wnd = newHeadlessWindow(t, "no menu", geom.NewRect(10, 10, 600, 600), axColumn(quiet, table, list))
			if wnd != nil {
				wnd.ToFront()
				quiet.RequestFocus()
			}
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	tableRow := axMustNode(c, axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))["r2"])
	listRows := axChildNodes(tree, axMustNode(c, screen.AccessibilityNodeFor(list)))
	c.Equal(4, len(listRows))
	if len(listRows) != 4 {
		return
	}
	listRow := listRows[2]
	c.True(tableRow.Actions.Has(accessibility.ShowContextMenu), "the table's rows were described while it had a menu")
	c.True(listRow.Actions.Has(accessibility.ShowContextMenu), "and so were the list's")

	var tableBefore, listBefore int
	screen.Do(func() {
		table.ContextMenuCallback = nil
		list.ContextMenuCallback = nil
		tableBefore = tableChanges
		listBefore = listChanges
	})
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   tableRow.ID,
		Action: accessibility.ShowContextMenu,
	}), "a table with no menu refuses a request for one")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   listRow.ID,
		Action: accessibility.ShowContextMenu,
	}), "and so does a list")
	var tableAfter, listAfter int
	var quietFocused bool
	screen.Do(func() {
		tableAfter = tableChanges
		listAfter = listChanges
		quietFocused = quiet.Focused()
	})
	c.Equal([]int{0}, axTableSelectedIndexes(screen, table), "the table's selection is left alone")
	c.Equal(tableBefore, tableAfter, "and the application is not told it changed")
	c.Equal([]int{0}, selectedIndexesOn(screen, list), "the list's selection is left alone")
	c.Equal(listBefore, listAfter, "and the application is not told it changed")
	c.True(quietFocused, "the focus stays where it was")

	// With the menus back, the very same requests are carried out.
	screen.Do(func() {
		table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
		list.ContextMenuCallback = cmMenu(&listCalls, "List")
	})
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   tableRow.ID,
		Action: accessibility.ShowContextMenu,
	}), "the table's row reaches the table")
	c.Equal([]int{2}, axTableSelectedIndexes(screen, table))
	cmCloseMenu(c, screen, wnd)
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   listRow.ID,
		Action: accessibility.ShowContextMenu,
	}), "and the list's row reaches the list")
	c.Equal([]int{2}, selectedIndexesOn(screen, list))
	cmCloseMenu(c, screen, wnd)
	c.Equal(1, cmCount(screen, &tableCalls))
	c.Equal(1, cmCount(screen, &listCalls))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuNotFocusable verifies that a right-click or an assistive technology's request on a row of a table or
// list that cannot take the focus selects the row and opens the menu, leaving the focus where it was.
func TestContextMenuNotFocusable(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var list *unison.List[string]
	var field *unison.Field
	var tableCalls, listCalls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText("hello")
			table = axNewTable(flatRows(4)...)
			table.SetFocusable(false)
			table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			list = unison.NewList[string]()
			list.Append("Zero", "One", "Two", "Three")
			list.SetFocusable(false)
			list.ContextMenuCallback = cmMenu(&listCalls, "List")
			wnd = newHeadlessWindow(t, "not focusable", geom.NewRect(10, 10, 600, 600), axColumn(field, table, list))
			if wnd != nil {
				wnd.ToFront()
				field.RequestFocus()
			}
		}))
	c.NotNil(wnd)
	fieldFocused := func() bool {
		var focused bool
		screen.Do(func() { focused = field.Focused() })
		return focused
	}
	c.True(fieldFocused(), "the field holds the focus to begin with")

	var tablePoint, listPoint geom.Point
	screen.Do(func() {
		frame := table.RowFrame(2)
		tablePoint = geom.NewPoint(frame.X+10, frame.CenterY())
		rect := list.RowRect(2)
		listPoint = geom.NewPoint(rect.X+10, rect.CenterY())
	})
	screen.ClickWith(cmScreenPoint(c, screen, table, tablePoint), unison.ButtonRight, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click on a row of a table that cannot take the focus opens its menu")
	c.Equal([]string{cmTableTitle}, items)
	c.Equal([]int{2}, axTableSelectedIndexes(screen, table), "for the row right-clicked on")
	c.True(fieldFocused(), "and leaves the focus in the field")
	cmCloseMenu(c, screen, wnd)

	screen.ClickWith(cmScreenPoint(c, screen, list, listPoint), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "so does one on a row of such a list")
	c.Equal([]string{"List"}, items)
	c.Equal([]int{2}, selectedIndexesOn(screen, list))
	c.True(fieldFocused(), "which leaves the focus in the field as well")
	cmCloseMenu(c, screen, wnd)

	tree := screen.AccessibilityTree(wnd)
	tableRow := axMustNode(c, axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))["r1"])
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   tableRow.ID,
		Action: accessibility.ShowContextMenu,
	}), "an assistive technology asking a row of the table opens its menu")
	c.Equal([]int{1}, axTableSelectedIndexes(screen, table))
	c.True(fieldFocused(), "and leaves the focus in the field")
	cmCloseMenu(c, screen, wnd)

	tree = screen.AccessibilityTree(wnd)
	listRows := axChildNodes(tree, axMustNode(c, screen.AccessibilityNodeFor(list)))
	c.Equal(4, len(listRows))
	if len(listRows) == 4 {
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   listRows[1].ID,
			Action: accessibility.ShowContextMenu,
		}), "as does one asking a row of the list")
		c.Equal([]int{1}, selectedIndexesOn(screen, list))
		c.True(fieldFocused(), "which leaves the focus in the field as well")
		cmCloseMenu(c, screen, wnd)
	}
	c.Equal(2, cmCount(screen, &tableCalls))
	c.Equal(2, cmCount(screen, &listCalls))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// cmNamedPanel is a panel with a menu callback, the requests it records and a name for the assertions about it.
type cmNamedPanel struct {
	panel *unison.Panel
	calls *cmRequests
	name  string
}

// TestContextMenuNothingToShow verifies that ShowContextMenu reports false for a nil menu, one empty once its dangling
// separators are trimmed, a panel with no callback, a disabled panel and one in no window, the last two without asking
// the callback; and that the Menu key and shift+F10 then go on to the focused panel, while an assistive technology's
// request is refused.
func TestContextMenuNothingToShow(t *testing.T) {
	c := check.New(t)
	var refuser, hollow, empty, disabled, plain, loose, shown *unison.Panel
	var refuserCalls, hollowCalls, emptyCalls, disabledCalls, looseCalls, shownCalls cmRequests
	var keys []unison.KeyCode
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			// takeKeys makes the panel focusable and has it take every key it is offered, noting which.
			takeKeys := func(p *unison.Panel) {
				p.SetFocusable(true)
				p.KeyDownCallback = func(key unison.KeyCode, _ mod.Modifiers, _ bool) bool {
					keys = append(keys, key)
					return true
				}
			}
			refuser = fixedPanel(60, 30)
			takeKeys(refuser)
			refuser.ContextMenuCallback = func(where geom.Point) unison.Menu {
				refuserCalls.at = append(refuserCalls.at, where)
				return nil
			}
			hollow = fixedPanel(60, 30)
			takeKeys(hollow)
			hollow.ContextMenuCallback = cmMenu(&hollowCalls, "", "", "")
			empty = fixedPanel(60, 30)
			empty.ContextMenuCallback = cmMenu(&emptyCalls)
			disabled = fixedPanel(60, 30)
			disabled.ContextMenuCallback = cmMenu(&disabledCalls, "Disabled")
			disabled.SetEnabled(false)
			plain = fixedPanel(60, 30)
			shown = fixedPanel(60, 30)
			shown.ContextMenuCallback = cmMenu(&shownCalls, "Shown")
			loose = fixedPanel(60, 30)
			loose.ContextMenuCallback = cmMenu(&looseCalls, "Loose")
			wnd = newHeadlessWindow(t, "nothing to show", geom.NewRect(10, 10, 500, 400),
				axColumn(refuser, hollow, empty, disabled, plain, shown))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)

	// With nothing to show, the chord goes on to the panel holding the focus.
	for _, one := range []cmNamedPanel{
		{panel: refuser, calls: &refuserCalls, name: "a callback that answers nil"},
		{panel: hollow, calls: &hollowCalls, name: "a menu of nothing but separators"},
	} {
		c.True(screen.Do(func() {
			one.panel.RequestFocus()
			keys = nil
		}))
		screen.KeyPress(unison.KeyMenu, mod.None)
		screen.KeyPress(unison.KeyF10, mod.Shift)
		c.Equal(2, cmCount(screen, one.calls), "with %s, each chord asks for the menu", one.name)
		var got []unison.KeyCode
		screen.Do(func() { got = slices.Clone(keys) })
		c.Equal([]unison.KeyCode{unison.KeyMenu, unison.KeyF10}, got,
			"with %s, each chord then reaches the panel holding the focus", one.name)
		menus, _ := cmOpenMenu(c, screen, wnd)
		c.Equal(0, menus, "with %s, no menu opens", one.name)
	}

	// An assistive technology's request is refused.
	screen.AccessibilityTree(wnd)
	for _, one := range []cmNamedPanel{
		{panel: refuser, calls: &refuserCalls, name: "a callback that answers nil"},
		{panel: hollow, calls: &hollowCalls, name: "a menu of nothing but separators"},
		{panel: empty, calls: &emptyCalls, name: "a menu with nothing in it"},
	} {
		before := cmCount(screen, one.calls)
		node := axMustNode(c, screen.AccessibilityNodeFor(one.panel))
		c.True(node.Actions.Has(accessibility.ShowContextMenu), "with %s, the menu is offered", one.name)
		c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.ShowContextMenu,
		}), "with %s, the request is refused", one.name)
		c.Equal(before+1, cmCount(screen, one.calls), "with %s, after the menu was asked for", one.name)
		menus, _ := cmOpenMenu(c, screen, wnd)
		c.Equal(0, menus, "with %s, no menu opens", one.name)
	}

	// ShowContextMenu itself reports false in every case.
	var results []bool
	screen.Do(func() {
		for _, p := range []*unison.Panel{refuser, hollow, empty, disabled, plain, loose} {
			results = append(results, p.ShowContextMenu(geom.NewPoint(5, 5)))
		}
	})
	c.Equal([]bool{false, false, false, false, false, false}, results, "nothing is shown, and each call says so")
	c.Equal(0, cmCount(screen, &disabledCalls), "a disabled panel's callback is not asked")
	c.Equal(0, cmCount(screen, &looseCalls), "nor is that of a panel in no window")
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus)
	var ok bool
	screen.Do(func() { ok = shown.ShowContextMenu(geom.NewPoint(5, 5)) })
	c.True(ok, "a menu with something in it is shown, and the call says so")
	c.Equal(1, cmCount(screen, &shownCalls))
	cmMenuAt(c, screen, wnd, shown, geom.NewPoint(5, 5))
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuDefaultAnchorInAScrollPanel verifies that DefaultContextMenuAnchor, used by the keyboard and assistive
// technologies, is the middle of the visible part of a panel in a scroll panel, or of the whole panel when none of it
// is visible.
func TestContextMenuDefaultAnchorInAScrollPanel(t *testing.T) {
	c := check.New(t)
	var tall, below *unison.Panel
	var scroller *unison.ScrollPanel
	var tallCalls, belowCalls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			// Narrower than the view port, so that only the scrolling clips it.
			tall = fixedPanel(100, 300)
			tall.SetFocusable(true)
			tall.ContextMenuCallback = cmMenu(&tallCalls, "Tall")
			below = fixedPanel(100, 60)
			below.ContextMenuCallback = cmMenu(&belowCalls, "Below")
			scroller = axScroller(axColumn(tall, below), geom.NewSize(200, 100))
			wnd = newHeadlessWindow(t, "scrolled", geom.NewRect(10, 10, 500, 400), axColumn(scroller))
			if wnd != nil {
				wnd.ToFront()
				tall.RequestFocus()
			}
		}))
	c.NotNil(wnd)

	// Scrolled down a little, both ends of the tall panel are out of sight, and all of the panel below it.
	var visible, whole, belowCenter, anchor, belowAnchor geom.Point
	var v float32
	var belowHidden bool
	screen.Do(func() {
		scroller.SetPosition(0, 30)
		_, v = scroller.Position()
		view := scroller.ContentView()
		inView := view.RectToRoot(view.ContentRect(false))
		visible = tall.RectFromRoot(inView).Intersect(tall.ContentRect(false)).Center()
		whole = tall.ContentRect(false).Center()
		belowHidden = below.RectFromRoot(inView).Intersect(below.ContentRect(false)).Empty()
		belowCenter = below.ContentRect(false).Center()
		anchor = tall.DefaultContextMenuAnchor()
		belowAnchor = below.DefaultContextMenuAnchor()
	})
	c.Equal(float32(30), v, "the test needs the view scrolled")
	c.True(belowHidden, "the test needs the panel below scrolled wholly out of sight")
	c.True(visible != whole, "the test needs the middle of what can be seen to differ from the middle of the whole")
	c.Equal(visible, anchor, "the default is the middle of what can be seen")
	c.Equal(belowCenter, belowAnchor, "and the middle of the whole when none of it can be seen")

	screen.KeyPress(unison.KeyMenu, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "the Menu key opens the focused panel's menu")
	c.Equal([]string{"Tall"}, items)
	c.Equal(visible, cmLast(screen, &tallCalls), "in the middle of what can be seen of it")
	cmMenuAt(c, screen, wnd, tall, visible)
	cmCloseMenu(c, screen, wnd)

	screen.AccessibilityTree(wnd)
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   axMustNode(c, screen.AccessibilityNodeFor(below)).ID,
		Action: accessibility.ShowContextMenu,
	}), "an assistive technology can ask a panel scrolled out of sight for its menu")
	c.Equal(belowCenter, cmLast(screen, &belowCalls), "which opens in the middle of the whole of it")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// cmRowShowing reports whether the span from top to bottom, in the widget's own coordinates, is wholly visible in the
// scroll panel. UI thread only.
func cmRowShowing(scroller *unison.ScrollPanel, p unison.Paneler, top, bottom float32) bool {
	view := scroller.ContentView()
	shown := p.AsPanel().RectFromRoot(view.RectToRoot(view.ContentRect(false)))
	return top >= shown.Y && bottom <= shown.Bottom()
}

// TestContextMenuRowsInAScrollPanel verifies that, for a table and a list in scroll panels, a right-click on a partly
// visible row selects it and focuses the widget without scrolling, while shift+F10 and an assistive technology's
// request scroll the row into view and open the menu beneath it.
func TestContextMenuRowsInAScrollPanel(t *testing.T) {
	c := check.New(t)
	const rowCount = 30
	const row = 20
	var table *unison.Table[*tableTestRow]
	var list *unison.List[string]
	var tableScroller, listScroller *unison.ScrollPanel
	var field *unison.Field
	var tableCalls, listCalls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 900, Height: 700},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			// Named as flatRows names them, which it can do only up to r9.
			rows := make([]*tableTestRow, rowCount)
			for i := range rows {
				rows[i] = newTableTestRow("r" + strconv.Itoa(i))
			}
			table = axNewTable(rows...)
			table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			tableScroller = axScroller(table, geom.NewSize(300, 100))
			list = unison.NewList[string]()
			list.Factory = &unison.DefaultCellFactory{Height: 20}
			for i := range rowCount {
				list.Append("Row " + strconv.Itoa(i))
			}
			list.ContextMenuCallback = cmMenu(&listCalls, "List")
			listScroller = axScroller(list, geom.NewSize(300, 100))
			wnd = newHeadlessWindow(t, "scrolled rows", geom.NewRect(10, 10, 600, 500),
				axColumn(field, tableScroller, listScroller))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)

	type widget struct {
		scroller *unison.ScrollPanel
		panel    unison.Paneler
		calls    *cmRequests
		rowFrame func() geom.Rect
		selectAt func(index int)
		selected func() []int
		rowNode  func(tree *accessibility.Tree) *accessibility.Node
		name     string
	}
	for _, one := range []widget{
		{
			name:     cmTableName,
			scroller: tableScroller,
			panel:    table,
			calls:    &tableCalls,
			rowFrame: func() geom.Rect { return table.RowFrame(row) },
			selectAt: func(index int) { table.SetLeadCell(index, -1) },
			selected: func() []int { return axTableSelectedIndexes(screen, table) },
			rowNode: func(tree *accessibility.Tree) *accessibility.Node {
				return axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))["r"+strconv.Itoa(row)]
			},
		},
		{
			name:     cmListName,
			scroller: listScroller,
			panel:    list,
			calls:    &listCalls,
			rowFrame: func() geom.Rect { return list.RowRect(row) },
			selectAt: func(index int) { list.Select(false, index) },
			selected: func() []int { return selectedIndexesOn(screen, list) },
			rowNode: func(tree *accessibility.Tree) *accessibility.Node {
				return axNodeWithRowIndex(tree, axMustNode(c, screen.AccessibilityNodeFor(list)), row)
			},
		},
	} {
		// Select the first row, focus elsewhere, and scroll until only the lower part of the target row can be seen.
		var frame geom.Rect
		var h, v float32
		screen.Do(func() {
			one.selectAt(0)
			field.RequestFocus()
			frame = one.rowFrame()
			one.scroller.SetPosition(0, frame.Y+5)
			h, v = one.scroller.Position()
		})
		top, bottom := frame.Y, frame.Bottom()
		beneath := geom.NewPoint(frame.X, bottom)
		c.Equal(top+5, v, "the %s test needs the view scrolled", one.name)
		var partly bool
		screen.Do(func() { partly = !cmRowShowing(one.scroller, one.panel, top, bottom) })
		c.True(partly, "the %s test needs the row only partly in view", one.name)
		point := cmScreenPoint(c, screen, one.panel, geom.NewPoint(frame.X+10, bottom-2))
		screen.ClickWith(point, unison.ButtonRight, mod.None)
		menus, _ := cmOpenMenu(c, screen, wnd)
		c.Equal(1, menus, "a right-click on the part of a %s row that can be seen opens the menu", one.name)
		var afterH, afterV float32
		var focused bool
		screen.Do(func() {
			afterH, afterV = one.scroller.Position()
			focused = one.panel.AsPanel().Focused()
		})
		c.Equal([]int{row}, one.selected(), "for the %s row right-clicked on", one.name)
		c.True(focused, "the %s takes the focus", one.name)
		c.Equal(h, afterH, "without the %s view moving", one.name)
		c.Equal(v, afterV, "without the %s view moving", one.name)
		cmCloseMenu(c, screen, wnd)

		// With the row out of sight again, shift+F10 brings it into view and opens the menu beneath it.
		var showing bool
		screen.Do(func() {
			one.scroller.SetPosition(0, 0)
			showing = cmRowShowing(one.scroller, one.panel, top, bottom)
		})
		c.False(showing, "the %s test needs the row out of sight", one.name)
		screen.KeyPress(unison.KeyF10, mod.Shift)
		menus, _ = cmOpenMenu(c, screen, wnd)
		c.Equal(1, menus, "shift+F10 opens the %s menu", one.name)
		screen.Do(func() { showing = cmRowShowing(one.scroller, one.panel, top, bottom) })
		c.True(showing, "the %s row is brought into view", one.name)
		c.Equal(beneath, cmLast(screen, one.calls), "the %s menu opens beneath the row", one.name)
		cmMenuAt(c, screen, wnd, one.panel, beneath)
		cmCloseMenu(c, screen, wnd)

		// The same for an assistive technology's request; the row is described while out of sight because it is
		// selected.
		screen.Do(func() { one.scroller.SetPosition(0, 0) })
		node := axMustNode(c, one.rowNode(screen.AccessibilityTree(wnd)), "the selected row is described:", one.name)
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.ShowContextMenu,
		}), "an assistive technology asking the %s row opens the menu", one.name)
		screen.Do(func() { showing = cmRowShowing(one.scroller, one.panel, top, bottom) })
		c.True(showing, "the %s row is brought into view", one.name)
		c.Equal(beneath, cmLast(screen, one.calls))
		cmCloseMenu(c, screen, wnd)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuAccessibilityFocusesFirst verifies that an assistive technology's request for the menu of a plain
// panel, a table row or a list row moves the focus to the panel before its callback builds the menu.
func TestContextMenuAccessibilityFocusesFirst(t *testing.T) {
	c := check.New(t)
	var panel *unison.Panel
	var table *unison.Table[*tableTestRow]
	var list *unison.List[string]
	var field *unison.Field
	var panelCalls, tableCalls, listCalls cmRequests
	var panelFocus, tableFocus, listFocus []bool
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			panel = fixedPanel(60, 30)
			panel.SetFocusable(true)
			panel.ContextMenuCallback = cmMenuNotingFocus(&panelCalls, panel, &panelFocus, "Panel")
			table = axNewTable(flatRows(4)...)
			table.ContextMenuCallback = cmMenuNotingFocus(&tableCalls, table, &tableFocus, cmTableTitle)
			list = unison.NewList[string]()
			list.Append("Zero", "One", "Two", "Three")
			list.ContextMenuCallback = cmMenuNotingFocus(&listCalls, list, &listFocus, "List")
			wnd = newHeadlessWindow(t, "focus first", geom.NewRect(10, 10, 600, 600),
				axColumn(field, panel, table, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)

	for _, one := range []struct {
		node    func(tree *accessibility.Tree) *accessibility.Node
		focused *[]bool
		name    string
	}{
		{
			name:    "a plain panel",
			focused: &panelFocus,
			node: func(*accessibility.Tree) *accessibility.Node {
				return screen.AccessibilityNodeFor(panel)
			},
		},
		{
			name:    "a table row",
			focused: &tableFocus,
			node: func(tree *accessibility.Tree) *accessibility.Node {
				return axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))["r2"]
			},
		},
		{
			name:    "a list row",
			focused: &listFocus,
			node: func(tree *accessibility.Tree) *accessibility.Node {
				return axNodeWithRowIndex(tree, axMustNode(c, screen.AccessibilityNodeFor(list)), 2)
			},
		},
	} {
		c.True(screen.Do(func() { field.RequestFocus() }))
		node := axMustNode(c, one.node(screen.AccessibilityTree(wnd)), "the node for", one.name)
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.ShowContextMenu,
		}), "asking %s for its menu opens it", one.name)
		var focused []bool
		screen.Do(func() { focused = slices.Clone(*one.focused) })
		c.Equal([]bool{true}, focused, "for %s, the focus had moved there by the time the menu was asked for", one.name)
		cmCloseMenu(c, screen, wnd)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuRowsInTheBackground verifies that the rows and cells of a table with a menu, and the rows of a list
// with one, stop offering it while their window is in the background and offer it again once it is back in front.
func TestContextMenuRowsInTheBackground(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var list *unison.List[string]
	var tableCalls, listCalls cmRequests
	var wnd, other *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 1000, Height: 700},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(3)...)
			table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			list = unison.NewList[string]()
			list.Append("Zero", "One", "Two")
			list.ContextMenuCallback = cmMenu(&listCalls, "List")
			other = newHeadlessWindow(t, "other", geom.NewRect(620, 10, 300, 200), axColumn(fixedPanel(60, 30)))
			wnd = newHeadlessWindow(t, "rows", geom.NewRect(10, 10, 600, 600), axColumn(table, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	c.NotNil(other)

	// offering counts the table's rows and cells and the list's rows, and how many of each offer the menu.
	offering := func() (rows, rowsOffering, cells, cellsOffering, items, itemsOffering int) {
		tree := screen.AccessibilityTree(wnd)
		for _, row := range axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table))) {
			rows++
			if row.Actions.Has(accessibility.ShowContextMenu) {
				rowsOffering++
			}
			for _, cell := range axChildNodes(tree, row) {
				if cell.Role == role.Cell {
					cells++
					if cell.Actions.Has(accessibility.ShowContextMenu) {
						cellsOffering++
					}
				}
			}
		}
		for _, item := range axChildNodes(tree, axMustNode(c, screen.AccessibilityNodeFor(list))) {
			items++
			if item.Actions.Has(accessibility.ShowContextMenu) {
				itemsOffering++
			}
		}
		return rows, rowsOffering, cells, cellsOffering, items, itemsOffering
	}

	rows, rowsOffering, cells, cellsOffering, items, itemsOffering := offering()
	c.Equal(3, rows)
	c.Equal(6, cells)
	c.Equal(3, items)
	c.Equal(rows, rowsOffering, "every row of the table offers its menu while the window is active")
	c.Equal(cells, cellsOffering, "and so does every cell")
	c.Equal(items, itemsOffering, "and every row of the list")

	c.True(screen.Do(func() { other.ToFront() }))
	_, rowsOffering, _, cellsOffering, _, itemsOffering = offering()
	c.Equal(0, rowsOffering, "no row of the table offers it while the window is in the background")
	c.Equal(0, cellsOffering, "nor does any cell")
	c.Equal(0, itemsOffering, "nor any row of the list")

	c.True(screen.Do(func() { wnd.ToFront() }))
	_, rowsOffering, _, cellsOffering, _, itemsOffering = offering()
	c.Equal(rows, rowsOffering, "the rows offer it again once the window is back in front")
	c.Equal(cells, cellsOffering)
	c.Equal(items, itemsOffering)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuTableAndListAnchors verifies that a table or list menu asked for from the keyboard, or by an assistive
// technology asking the widget itself, opens beneath the row the person is on, or beneath the table cell under the cell
// cursor, and at DefaultContextMenuAnchor when nothing is selected.
func TestContextMenuTableAndListAnchors(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var list *unison.List[string]
	var tableCalls, listCalls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(5)...)
			table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			list = unison.NewList[string]()
			list.Append("Zero", "One", "Two", "Three")
			list.ContextMenuCallback = cmMenu(&listCalls, "List")
			wnd = newHeadlessWindow(t, "anchors", geom.NewRect(10, 10, 600, 600), axColumn(table, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)

	// fromKeyboard runs prepare, focuses the widget, presses shift+F10 and checks that the menu opened at where().
	fromKeyboard := func(p unison.Paneler, calls *cmRequests, prepare func(), where func() geom.Point, msg string) {
		var expected geom.Point
		screen.Do(func() {
			prepare()
			p.AsPanel().RequestFocus()
			expected = where()
		})
		screen.KeyPress(unison.KeyF10, mod.Shift)
		c.Equal(expected, cmLast(screen, calls), msg)
		cmMenuAt(c, screen, wnd, p, expected, msg)
		cmCloseMenu(c, screen, wnd)
	}
	// fromAccessibility runs prepare, asks the widget's own node for its menu and checks that it opened at where().
	fromAccessibility := func(p unison.Paneler, calls *cmRequests, prepare func(), where func() geom.Point,
		msg string,
	) {
		var expected geom.Point
		screen.Do(func() {
			prepare()
			expected = where()
		})
		screen.AccessibilityTree(wnd)
		node := axMustNode(c, screen.AccessibilityNodeFor(p))
		c.True(node.Actions.Has(accessibility.ShowContextMenu), "the widget itself offers its menu")
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.ShowContextMenu,
		}), msg)
		c.Equal(expected, cmLast(screen, calls), msg)
		cmMenuAt(c, screen, wnd, p, expected, msg)
		cmCloseMenu(c, screen, wnd)
	}
	beneathCell := func(row, col int) func() geom.Point {
		return func() geom.Point {
			frame := table.CellFrame(row, col)
			return geom.NewPoint(frame.X, frame.Bottom())
		}
	}
	beneathTableRow := func(row int) func() geom.Point {
		return func() geom.Point {
			frame := table.RowFrame(row)
			return geom.NewPoint(frame.X, frame.Bottom())
		}
	}
	beneathListRow := func(row int) func() geom.Point {
		return func() geom.Point {
			rect := list.RowRect(row)
			return geom.NewPoint(rect.X, rect.Bottom())
		}
	}

	var cellX, rowX float32
	screen.Do(func() {
		cellX = table.CellFrame(2, 1).X
		rowX = table.RowFrame(2).X
	})
	c.True(cellX != rowX, "the test needs the cell to begin somewhere other than its row")
	fromKeyboard(table, &tableCalls, func() { table.SetLeadCell(2, 1) }, beneathCell(2, 1),
		"with the cell cursor on a cell, the table's menu opens beneath the cell")
	fromKeyboard(table, &tableCalls, func() { table.SetLeadCell(3, -1) }, beneathTableRow(3),
		"with the person on a row, the table's menu opens beneath the row")
	fromAccessibility(table, &tableCalls, func() { table.SetLeadCell(1, -1) }, beneathTableRow(1),
		"asked for by an assistive technology, the table's menu opens beneath the row as well")
	fromAccessibility(table, &tableCalls, func() { table.SetLeadCell(4, 0) }, beneathCell(4, 0),
		"or beneath the cell the cell cursor is on")
	fromKeyboard(table, &tableCalls, table.ClearSelection, table.DefaultContextMenuAnchor,
		"with nothing selected, the table's menu opens in the middle of what can be seen of it")

	fromKeyboard(list, &listCalls, func() { list.Select(false, 2) }, beneathListRow(2),
		"with a row selected, the list's menu opens beneath the row")
	fromAccessibility(list, &listCalls, func() { list.Select(false, 1) }, beneathListRow(1),
		"asked for by an assistive technology, the list's menu opens beneath the row as well")
	fromKeyboard(list, &listCalls, func() { list.Select(false) }, list.DefaultContextMenuAnchor,
		"with nothing selected, the list's menu opens in the middle of what can be seen of it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuReportsSelectionChanges verifies that a right-click or an assistive technology's request on an
// unselected row of a table or list reports the selection change once, before the menu is asked for, and that one on a
// row of a larger selection reports nothing.
func TestContextMenuReportsSelectionChanges(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var list *unison.List[string]
	var events []string
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(5)...)
			table.SelectionChangedCallback = func() { events = append(events, "selection") }
			table.ContextMenuCallback = func(geom.Point) unison.Menu {
				events = append(events, "menu")
				return cmNewMenu(cmTableTitle)
			}
			list = unison.NewList[string]()
			list.Append("Zero", "One", "Two", "Three", "Four")
			list.NewSelectionCallback = func() { events = append(events, "selection") }
			list.ContextMenuCallback = func(geom.Point) unison.Menu {
				events = append(events, "menu")
				return cmNewMenu("List")
			}
			wnd = newHeadlessWindow(t, "selection changes", geom.NewRect(10, 10, 600, 600), axColumn(table, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)

	type widget struct {
		panel    unison.Paneler
		selectAt func(indexes ...int)
		rowPoint func(row int) geom.Point
		rowNode  func(tree *accessibility.Tree, row int) *accessibility.Node
		name     string
	}
	for _, one := range []widget{
		{
			name:  cmTableName,
			panel: table,
			selectAt: func(indexes ...int) {
				table.ClearSelection()
				table.SelectByIndex(indexes...)
			},
			rowPoint: func(row int) geom.Point {
				frame := table.RowFrame(row)
				return geom.NewPoint(frame.X+10, frame.CenterY())
			},
			rowNode: func(tree *accessibility.Tree, row int) *accessibility.Node {
				return axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))["r"+strconv.Itoa(row)]
			},
		},
		{
			name:     cmListName,
			panel:    list,
			selectAt: func(indexes ...int) { list.Select(false, indexes...) },
			rowPoint: func(row int) geom.Point {
				rect := list.RowRect(row)
				return geom.NewPoint(rect.X+10, rect.CenterY())
			},
			rowNode: func(tree *accessibility.Tree, row int) *accessibility.Node {
				return axNodeWithRowIndex(tree, axMustNode(c, screen.AccessibilityNodeFor(list)), row)
			},
		},
	} {
		// prepare selects the given rows and forgets what that reported.
		prepare := func(indexes ...int) {
			screen.Do(func() {
				one.selectAt(indexes...)
				events = nil
			})
		}
		reported := func() []string {
			var got []string
			screen.Do(func() { got = slices.Clone(events) })
			return got
		}
		byClick := func(row int) {
			var point geom.Point
			screen.Do(func() { point = one.rowPoint(row) })
			screen.ClickWith(cmScreenPoint(c, screen, one.panel, point), unison.ButtonRight, mod.None)
		}
		byRequest := func(row int) {
			node := axMustNode(c, one.rowNode(screen.AccessibilityTree(wnd), row), "the row is described:", one.name)
			c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
				Node:   node.ID,
				Action: accessibility.ShowContextMenu,
			}))
		}
		for _, ask := range []struct {
			how  func(row int)
			name string
		}{
			{how: byClick, name: "a right-click"},
			{how: byRequest, name: "an assistive technology's request"},
		} {
			prepare(0)
			ask.how(2)
			c.Equal([]string{"selection", "menu"}, reported(),
				"%s on a %s row outside the selection reports the change once, before the menu is asked for",
				ask.name, one.name)
			cmCloseMenu(c, screen, wnd)
			prepare(0, 1, 2)
			ask.how(1)
			c.Equal([]string{"menu"}, reported(),
				"%s on a %s row of a larger selection, which keeps it, reports nothing", ask.name, one.name)
			cmCloseMenu(c, screen, wnd)
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// cmWrappedMenu lets cmRecordingMenu embed a unison.Menu without the embedded field's name colliding with Menu.Menu.
type cmWrappedMenu = unison.Menu

// cmRecordingMenu logs each Popup and Dispose made on the menu it wraps, and records the rect it is popped up at. UI
// thread only.
type cmRecordingMenu struct {
	cmWrappedMenu
	log  *[]string
	rect *geom.Rect
}

// Popup implements unison.Menu.
func (m *cmRecordingMenu) Popup(where geom.Rect, itemIndex int) {
	*m.log = append(*m.log, "popup")
	*m.rect = where
	m.cmWrappedMenu.Popup(where, itemIndex)
}

// Dispose implements unison.Menu.
func (m *cmRecordingMenu) Dispose() {
	*m.log = append(*m.log, "dispose")
	m.cmWrappedMenu.Dispose()
}

// TestContextMenuPopupAndDispose verifies that ShowContextMenu draws pending changes before popping the menu up at the
// requested point in window coordinates, disposes of the menu after popping it up, or instead of doing so when it holds
// only separators, and takes down a showing tooltip.
func TestContextMenuPopupAndDispose(t *testing.T) {
	c := check.New(t)
	var owner, hollow *unison.Panel
	var log []string
	var rect geom.Rect
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			owner = fixedPanel(80, 40)
			owner.Tooltip = unison.NewTooltipWithText("What the owner is")
			owner.TooltipImmediate = true
			owner.DrawCallback = func(*unison.Canvas, geom.Rect) { log = append(log, "draw") }
			owner.ContextMenuCallback = func(geom.Point) unison.Menu {
				log = append(log, "asked")
				// Stands in for a change the request made, such as a right-click selecting a table row.
				owner.MarkForRedraw()
				return &cmRecordingMenu{cmWrappedMenu: cmNewMenu("Item"), log: &log, rect: &rect}
			}
			hollow = fixedPanel(80, 40)
			hollow.ContextMenuCallback = func(geom.Point) unison.Menu {
				log = append(log, "asked")
				return &cmRecordingMenu{cmWrappedMenu: cmNewMenu("", ""), log: &log, rect: &rect}
			}
			wnd = newHeadlessWindow(t, "popup", geom.NewRect(10, 10, 500, 400),
				axColumn(fixedPanel(80, 40), owner, hollow))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	tooltips := func() int { return len(axNodesWithRole(screen.AccessibilityTree(wnd), role.Tooltip)) }
	logged := func() []string {
		var got []string
		screen.Do(func() { got = slices.Clone(log) })
		return got
	}

	screen.MouseMove(screen.PanelCenter(owner), mod.None)
	c.Equal(1, tooltips(), "the test needs the owner's tooltip showing")
	var center geom.Point
	screen.Do(func() {
		center = owner.ContentRect(false).Center()
		log = nil
		rect = geom.Rect{}
	})
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   axMustNode(c, screen.AccessibilityNodeFor(owner)).ID,
		Action: accessibility.ShowContextMenu,
	}))
	got := logged()
	popup := slices.Index(got, "popup")
	c.True(popup > 0, "the menu pops up: %v", got)
	if popup > 0 {
		c.Equal("asked", got[0], "once it has been asked for: %v", got)
		c.True(slices.Contains(got[:popup], "draw"), "the owner is drawn before the menu pops up: %v", got)
		c.Equal([]string{"popup", "dispose"}, got[popup:min(popup+2, len(got))],
			"and the menu is disposed of once it has popped up: %v", got)
		c.Equal(1, len(slices.DeleteFunc(slices.Clone(got), func(s string) bool { return s != "dispose" })),
			"and only then: %v", got)
	}
	var expected geom.Rect
	screen.Do(func() { expected = geom.Rect{Point: owner.PointToRoot(center), Size: geom.NewSize(1, 1)} })
	c.Equal(expected, rect, "the menu pops up at the position asked for, in the window's coordinates")
	cmMenuAt(c, screen, wnd, owner, center)
	c.Equal(0, tooltips(), "the tooltip is taken down, since it would be drawn over the menu")
	cmCloseMenu(c, screen, wnd)

	screen.Do(func() { log = nil })
	screen.AccessibilityTree(wnd)
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   axMustNode(c, screen.AccessibilityNodeFor(hollow)).ID,
		Action: accessibility.ShowContextMenu,
	}))
	c.Equal([]string{"asked", "dispose"}, logged(),
		"a menu holding nothing but separators is disposed of without being popped up")

	// Away and back, since a tooltip is shown when the pointer arrives over its panel.
	screen.MouseMove(screen.PanelCenter(hollow), mod.None)
	screen.MouseMove(screen.PanelCenter(owner), mod.None)
	c.Equal(1, tooltips(), "the test needs the owner's tooltip showing again")
	screen.ClickWith(screen.PanelCenter(owner), unison.ButtonRight, mod.None)
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click opens the menu")
	c.Equal(0, tooltips(), "and the tooltip is gone by the time it has")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// cmPanickingAnchorPanel is a focusable panel whose ContextMenuAnchor panics.
type cmPanickingAnchorPanel struct {
	unison.Panel
}

func newCMPanickingAnchorPanel() *cmPanickingAnchorPanel {
	p := &cmPanickingAnchorPanel{}
	p.Self = p
	cmFixedFocusable(p.AsPanel())
	return p
}

// ContextMenuAnchor implements unison.ContextMenuAnchorer.
func (p *cmPanickingAnchorPanel) ContextMenuAnchor() geom.Point {
	panic("no idea where the menu belongs")
}

// TestContextMenuPanics verifies that a panicking ContextMenuCallback is recorded and shows nothing, so the Menu key
// reaches the focused panel and an assistive technology's request is refused; that a panicking ContextMenuAnchor is
// recorded and the menu opens at DefaultContextMenuAnchor; and that a panicking ContextMenuPressHandler is recorded and
// the menu still opens on the release.
func TestContextMenuPanics(t *testing.T) {
	c := check.New(t)
	var panicky *unison.Panel
	var lost *cmPanickingAnchorPanel
	var pressy *cmPanickingPressPanel
	var lostCalls, pressyCalls cmRequests
	var keys []unison.KeyCode
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			panicky = fixedPanel(60, 30)
			panicky.SetFocusable(true)
			panicky.KeyDownCallback = func(key unison.KeyCode, _ mod.Modifiers, _ bool) bool {
				keys = append(keys, key)
				return true
			}
			panicky.ContextMenuCallback = func(geom.Point) unison.Menu { panic("no menu to be had") }
			lost = newCMPanickingAnchorPanel()
			lost.ContextMenuCallback = cmMenu(&lostCalls, "Lost")
			pressy = newCMPanickingPressPanel()
			pressy.ContextMenuCallback = cmMenu(&pressyCalls, "Pressy")
			wnd = newHeadlessWindow(t, "panics", geom.NewRect(10, 10, 500, 400),
				axColumn(fixedPanel(60, 30), panicky, lost, pressy))
			if wnd != nil {
				wnd.ToFront()
				panicky.RequestFocus()
			}
		}))
	c.NotNil(wnd)

	screen.KeyPress(unison.KeyMenu, mod.None)
	var got []unison.KeyCode
	screen.Do(func() { got = slices.Clone(keys) })
	c.Equal([]unison.KeyCode{unison.KeyMenu}, got,
		"a callback that panicked showed nothing, so the key reaches the panel holding the focus")
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus)
	c.Equal(1, len(screen.Errors()), "and the panic is recorded")

	screen.AccessibilityTree(wnd)
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   axMustNode(c, screen.AccessibilityNodeFor(panicky)).ID,
		Action: accessibility.ShowContextMenu,
	}), "an assistive technology's request is refused")
	c.Equal(2, len(screen.Errors()), "with the panic recorded")
	var shown bool
	screen.Do(func() { shown = panicky.ShowContextMenu(geom.NewPoint(5, 5)) })
	c.False(shown, "ShowContextMenu says nothing was shown")
	c.Equal(3, len(screen.Errors()))
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus)

	var center geom.Point
	screen.Do(func() {
		lost.RequestFocus()
		center = lost.DefaultContextMenuAnchor()
	})
	c.True(center != geom.Point{}, "the test needs the default anchor to be somewhere other than the origin")
	screen.KeyPress(unison.KeyMenu, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "an anchor that panicked does not keep the menu from opening")
	c.Equal([]string{"Lost"}, items)
	c.Equal(center, cmLast(screen, &lostCalls), "which opens where a panel that says nothing about it has it")
	cmMenuAt(c, screen, wnd, lost, center)
	c.Equal(4, len(screen.Errors()), "and the panic is recorded")
	cmCloseMenu(c, screen, wnd)

	var onPressy geom.Point
	screen.Do(func() { onPressy = pressy.ContentRect(false).Center() })
	screen.ClickWith(cmScreenPoint(c, screen, pressy, onPressy), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a press handler that panicked does not keep the menu from opening on the release")
	c.Equal([]string{"Pressy"}, items)
	c.Equal(onPressy, cmLast(screen, &pressyCalls), "under the pointer")
	c.Equal(5, len(screen.Errors()), "and the panic is recorded")
	var focused bool
	screen.Do(func() { focused = pressy.Focused() })
	c.True(focused, "the panel was still given the focus")
	cmCloseMenu(c, screen, wnd)
}

// TestContextMenuModifiedRightDrag verifies that a right-drag with a modifier held, shift here, on a table or list with
// a menu is not handed back to the widget: it is sent no mouse events, the selection stays as the press made it, and no
// menu opens.
func TestContextMenuModifiedRightDrag(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var list *unison.List[string]
	var tableCalls, listCalls cmRequests
	var events []string
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 800},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(5)...)
			table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			table.MouseDownCallback = func(where geom.Point, button, clickCount int, mods mod.Modifiers) bool {
				events = append(events, "table down")
				return table.DefaultMouseDown(where, button, clickCount, mods)
			}
			table.MouseDragCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
				events = append(events, "table drag")
				return table.DefaultMouseDrag(where, button, mods)
			}
			table.MouseUpCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
				events = append(events, "table up")
				return table.DefaultMouseUp(where, button, mods)
			}
			list = unison.NewList[string]()
			list.Append("Zero", "One", "Two", "Three", "Four")
			list.ContextMenuCallback = cmMenu(&listCalls, "List")
			list.MouseDownCallback = func(where geom.Point, button, clickCount int, mods mod.Modifiers) bool {
				events = append(events, "list down")
				return list.DefaultMouseDown(where, button, clickCount, mods)
			}
			list.MouseDragCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
				events = append(events, "list drag")
				return list.DefaultMouseDrag(where, button, mods)
			}
			list.MouseUpCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
				events = append(events, "list up")
				return list.DefaultMouseUp(where, button, mods)
			}
			wnd = newHeadlessWindow(t, "modified right-drag", geom.NewRect(10, 10, 600, 600), axColumn(table, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)

	for _, one := range []struct {
		panel    unison.Paneler
		calls    *cmRequests
		prepare  func()
		rowPoint func(row int) geom.Point
		selected func() []int
		name     string
	}{
		{
			name:  cmTableName,
			panel: table,
			calls: &tableCalls,
			prepare: func() {
				table.ClearSelection()
				table.SelectByIndex(0)
			},
			rowPoint: func(row int) geom.Point {
				frame := table.RowFrame(row)
				return geom.NewPoint(frame.X+10, frame.CenterY())
			},
			selected: func() []int { return axTableSelectedIndexes(screen, table) },
		},
		{
			name:    cmListName,
			panel:   list,
			calls:   &listCalls,
			prepare: func() { list.Select(false, 0) },
			rowPoint: func(row int) geom.Point {
				rect := list.RowRect(row)
				return geom.NewPoint(rect.X+10, rect.CenterY())
			},
			selected: func() []int { return selectedIndexesOn(screen, list) },
		},
	} {
		var from, over, to geom.Point
		screen.Do(func() {
			one.prepare()
			events = nil
			from = one.rowPoint(1)
			over = one.rowPoint(2)
			to = one.rowPoint(3)
		})
		screen.MouseDown(cmScreenPoint(c, screen, one.panel, from), unison.ButtonRight, mod.Shift)
		c.Equal([]int{1}, one.selected(), "the shift-right press selects the %s row it landed on", one.name)
		screen.MouseMove(cmScreenPoint(c, screen, one.panel, over), mod.Shift)
		screen.MouseMove(cmScreenPoint(c, screen, one.panel, to), mod.Shift)
		screen.MouseUp(cmScreenPoint(c, screen, one.panel, to), unison.ButtonRight, mod.Shift)
		var got []string
		screen.Do(func() { got = slices.Clone(events) })
		c.Equal(0, len(got), "the %s is sent nothing of the drag: %v", one.name, got)
		c.Equal([]int{1}, one.selected(), "and its selection stays as the press made it")
		menus, _ := cmOpenMenu(c, screen, wnd)
		c.Equal(0, menus, "a right-drag opens no menu")
		c.Equal(0, cmCount(screen, one.calls), "nor even asks for one")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuUnfocusableField verifies that a field that cannot take the focus offers no default menu: a
// right-click on it opens nothing and leaves the focus, and the selection, of the field holding it alone, since the
// menu's commands act on whatever holds the focus; and that the field's menu opens once it can take the focus.
func TestContextMenuUnfocusableField(t *testing.T) {
	c := check.New(t)
	var holder, unfocusable *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			holder = unison.NewField()
			holder.SetText("secret")
			unfocusable = unison.NewField()
			unfocusable.SetText("other")
			unfocusable.SetFocusable(false)
			wnd = newHeadlessWindow(t, "unfocusable field", geom.NewRect(10, 10, 500, 400),
				axColumn(holder, unfocusable))
			if wnd != nil {
				wnd.ToFront()
				holder.RequestFocus()
			}
		}))
	c.NotNil(wnd)
	var holderFocused bool
	var selected string
	screen.Do(func() {
		holderFocused = holder.Focused()
		selected = holder.SelectedText()
	})
	c.True(holderFocused, "the test needs the other field holding the focus")
	c.Equal("secret", selected, "with its text selected")

	screen.ClickWith(cmScreenPoint(c, screen, unfocusable, geom.NewPoint(10, 5)), unison.ButtonRight, mod.None)
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "a right-click on a field that cannot take the focus opens no menu")
	screen.Do(func() {
		holderFocused = holder.Focused()
		selected = holder.SelectedText()
	})
	c.True(holderFocused, "the focus stays where it was")
	c.Equal("secret", selected, "and so does the selection the menu would have acted on")
	var shown bool
	screen.Do(func() { shown = unfocusable.ShowContextMenu(geom.NewPoint(10, 5)) })
	c.False(shown, "and ShowContextMenu shows nothing either")

	screen.Do(func() { unfocusable.SetFocusable(true) })
	screen.ClickWith(cmScreenPoint(c, screen, unfocusable, geom.NewPoint(10, 5)), unison.ButtonRight, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "once the field can take the focus, a right-click opens its menu")
	c.True(slices.Contains(items, "Copy"), "which acts on the field's own text: %v", items)
	var focused bool
	screen.Do(func() { focused = unfocusable.Focused() })
	c.True(focused, "with the field holding the focus")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuCellFieldRangeRequest verifies that an assistive technology's request for the menu of a Field in a
// table cell that names a range places the caret on that range although the table gives the field the focus for the
// request, so the select-all a field makes on gaining the focus is not mistaken for the person's own selection.
func TestContextMenuCellFieldRangeRequest(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "abcdef", false)
			e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			wnd = newHeadlessWindow(t, "cell range", geom.NewRect(10, 10, 500, 500), axColumn(e.table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	// The table holds the focus and the field has a caret at its end, with nothing selected.
	c.True(screen.Do(func() {
		e.table.RequestFocus()
		e.fields[1][0].SetSelection(6, 6)
	}))
	s := e.snapshot(c, screen, 1, 0)
	c.True(s.tableFocused, "the test needs the table focused")
	c.False(s.hasRange, "and the field without a selection")
	tree := screen.AccessibilityTree(wnd)
	rows := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	field := cmCellContent(tree, axMustNode(c, rows["r1"]), 0)
	c.Equal(1, len(field))
	if len(field) != 1 {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   field[0].ID,
		Action: accessibility.ShowContextMenu,
		Start:  3,
		End:    3,
	}), "asking for the menu at a position in the field opens it")
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	s = e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused, "the field takes the focus")
	c.True(s.installed, "and its cell stays attached")
	c.Equal(3, s.selStart, "the caret is at the position asked for")
	c.Equal(3, s.selEnd, "with nothing selected, the select-all made on gaining the focus not being the person's")
	cmCloseMenu(c, screen, wnd)

	// Back on the table, a request naming a range selects it.
	c.True(screen.Do(func() { e.table.RequestFocus() }))
	tree = screen.AccessibilityTree(wnd)
	rows = axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	field = cmCellContent(tree, axMustNode(c, rows["r2"]), 1)
	c.Equal(1, len(field))
	if len(field) != 1 {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   field[0].ID,
		Action: accessibility.ShowContextMenu,
		Start:  1,
		End:    4,
	}), "asking for the menu on a range in another field opens it")
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	s = e.snapshot(c, screen, 2, 1)
	c.True(s.fieldFocused)
	c.Equal(1, s.selStart, "the range asked for is selected")
	c.Equal(4, s.selEnd)
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuRowRequestAfterAFieldCommitReordersTheRows verifies that an assistive technology's request for the
// menu of a table row is carried out on that row even when the table taking the focus for it, away from a Field being
// edited in a cell, has the field commit and rebuild the rows in a different order: the row is looked up once the focus
// has moved, so the row named is the one selected and the menu opens beneath it.
func TestContextMenuRowRequestAfterAFieldCommitReordersTheRows(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var calls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 1, "abc", false)
			e.table.ContextMenuCallback = cmMenu(&calls, cmTableTitle)
			e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			wnd = newHeadlessWindow(t, "reordered rows", geom.NewRect(10, 10, 500, 500), axColumn(e.table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	// The field in the first row is being edited; losing the focus, it commits and the rows come back reversed.
	var inCell geom.Point
	c.True(screen.Do(func() { inCell = e.table.CellFrame(0, 0).Center() }))
	screen.Click(cmScreenPoint(c, screen, e.table, inCell))
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the test needs the field focused")
	c.True(screen.Do(func() {
		lost := e.fields[0][0].LostFocusCallback
		e.fields[0][0].LostFocusCallback = func() {
			if lost != nil {
				lost()
			}
			rows := slices.Clone(e.model.RootRows())
			slices.Reverse(rows)
			e.model.SetRootRows(rows)
			e.table.SyncToModel()
		}
	}))
	tree := screen.AccessibilityTree(wnd)
	rows := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   axMustNode(c, rows["r2"]).ID,
		Action: accessibility.ShowContextMenu,
	}), "asking the last row for its menu opens it")
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	var index int
	var selected bool
	var want geom.Point
	c.True(screen.Do(func() {
		index = e.table.RowToIndex(e.rows[2])
		selected = e.table.IsRowSelected(index)
		frame := e.table.RowFrame(index)
		want = geom.NewPoint(frame.X, frame.Bottom())
	}))
	c.Equal(0, index, "the test needs the rows reversed by the commit")
	c.True(selected, "the row asked for is the one selected, at the index the commit left it at")
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table))
	c.Equal(want, cmLast(screen, &calls), "and the menu opens beneath it")
	s = e.snapshot(c, screen, 0, 0)
	c.True(s.tableFocused, "the table holds the focus")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// cmRotateRows moves the last of the table's rows to the front, so that each row is at another index afterwards, and
// brings the table up to date.
func cmRotateRows(e *editTable) {
	rows := slices.Clone(e.model.RootRows())
	rows = append(rows[len(rows)-1:], rows[:len(rows)-1]...)
	e.model.SetRootRows(rows)
	e.table.SyncToModel()
}

// cmCommitRotatesRows makes the field in the given cell rotate the rows, as cmRotateRows does, when it loses the focus,
// as a field committing an edit that the table is sorted on would.
func cmCommitRotatesRows(e *editTable, row, col int) {
	lost := e.fields[row][col].LostFocusCallback
	e.fields[row][col].LostFocusCallback = func() {
		if lost != nil {
			lost()
		}
		cmRotateRows(e)
	}
}

// TestContextMenuCellWidgetRightClickAfterAFieldCommitReordersTheRows verifies that a right-click on a field in a cell
// while a field in another cell is being edited looks the row up only once the edited field has committed on losing
// the focus and reordered the rows: the field of the row then under the pointer is the one focused, adopted at its
// index, selected and asked for its menu. In a table that cannot hold the focus, where the edited field commits only as
// the right-clicked field takes the focus, that field's cell is adopted at the index its row has once the rows are
// reordered.
func TestContextMenuCellWidgetRightClickAfterAFieldCommitReordersTheRows(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 1, "abc", false)
			e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			wnd = newHeadlessWindow(t, "reordered rows", geom.NewRect(10, 10, 500, 500), axColumn(e.table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	// The field in the first row is being edited; losing the focus, it commits and the rows come back rotated, so
	// that the last row's cell, under the pointer, is where the second row's cell then is.
	screen.Click(cellCenter(screen, e.table, 0, 0))
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the test needs the field focused")
	screen.Do(func() { cmCommitRotatesRows(e, 0, 0) })
	screen.ClickWith(cellCenter(screen, e.table, 2, 0), unison.ButtonRight, mod.None)
	var order []int
	screen.Do(func() {
		for _, row := range e.rows {
			order = append(order, e.table.RowToIndex(row))
		}
	})
	c.Equal([]int{1, 2, 0}, order, "the test needs the commit to have rotated the rows")
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "the menu of the field under the pointer opens")
	c.True(slices.Contains(items, "Copy"), "a field's menu, not %v", items)
	s = e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused, "the field of the row under the pointer once the rows were reordered takes the focus")
	c.True(s.installed, "and its cell stays attached to the table")
	c.Equal(2, s.focusRow, "at the index its row now has")
	c.Equal(0, s.focusCol)
	c.Equal([]int{2}, axTableSelectedIndexes(screen, e.table), "and its row is selected")
	s = e.snapshot(c, screen, 2, 0)
	c.False(s.fieldFocused, "the field that was under the pointer before the commit is not focused")
	c.True(s.detached, "and its cell was handed back")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())

	// The same in a table that cannot hold the focus: the edited field commits only as the right-clicked field takes
	// the focus, so the rows are reordered once that field's cell is already built, at the index its row had.
	screen.Do(func() {
		e.model.SetRootRows(slices.Clone(e.rows))
		e.table.SyncToModel()
		e.table.SetFocusable(false)
		e.table.FocusCell(0, 0)
	})
	s = e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the test needs the first row's field focused again")
	c.Equal(0, s.focusRow)
	screen.MouseDown(cellCenter(screen, e.table, 2, 0), unison.ButtonRight, mod.None)
	order = nil
	screen.Do(func() {
		for _, row := range e.rows {
			order = append(order, e.table.RowToIndex(row))
		}
	})
	c.Equal([]int{1, 2, 0}, order, "the test needs the commit to have rotated the rows")
	s = e.snapshot(c, screen, 2, 0)
	c.True(s.fieldFocused, "the right-clicked field takes the focus")
	c.True(s.installed, "and its cell stays attached to the table")
	c.Equal(0, s.focusRow, "at the index its row has once the rows are reordered")
	c.Equal(0, s.focusCol)
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table), "with its row selected")
	s = e.snapshot(c, screen, 1, 0)
	c.False(s.fieldFocused, "and not the field of the row that took its old index")
	screen.MouseUp(cellCenter(screen, e.table, 2, 0), unison.ButtonRight, mod.None)
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus, "the release lands where the row no longer is, so no menu opens")
	s = e.snapshot(c, screen, 2, 0)
	c.True(s.fieldFocused, "and the field keeps the focus")
	c.Equal(0, s.focusRow)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuCellWidgetRequestAfterAFieldCommitReordersTheRows verifies that an assistive technology's request for
// the menu of a field in a cell, made while a field in another cell is being edited, adopts the field's cell at the
// index its row has once the edited field has committed and reordered the rows, and that a request for a field whose
// row the commit removed is refused, with the focus handed back to the table.
func TestContextMenuCellWidgetRequestAfterAFieldCommitReordersTheRows(t *testing.T) {
	testCellWidgetRequestAfterAFieldCommitReordersTheRows(t, 0, 0)
}

// TestContextMenuCellWidgetRequestWithARangeAfterAFieldCommitReordersTheRows verifies the same for a request that names
// a range, as UI Automation's text-range ShowContextMenu does: placing the range gives the field the focus before the
// table does, so the edited field commits then, and the row must still be the one looked up afterwards.
func TestContextMenuCellWidgetRequestWithARangeAfterAFieldCommitReordersTheRows(t *testing.T) {
	testCellWidgetRequestAfterAFieldCommitReordersTheRows(t, 1, 2)
}

// testCellWidgetRequestAfterAFieldCommitReordersTheRows makes the requests with the given range, which names none when
// start and end are both zero.
func testCellWidgetRequestAfterAFieldCommitReordersTheRows(t *testing.T, start, end int) {
	t.Helper()
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 1, "abc", false)
			e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			wnd = newHeadlessWindow(t, "reordered rows", geom.NewRect(10, 10, 500, 500), axColumn(e.table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	screen.Click(cellCenter(screen, e.table, 0, 0))
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the test needs the field focused")
	var commit func()
	screen.Do(func() {
		commit = e.fields[0][0].LostFocusCallback
		cmCommitRotatesRows(e, 0, 0)
	})
	tree := screen.AccessibilityTree(wnd)
	rows := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	field := cmCellContent(tree, axMustNode(c, rows["r2"]), 0)
	c.Equal(1, len(field))
	if len(field) != 1 {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   field[0].ID,
		Action: accessibility.ShowContextMenu,
		Start:  start,
		End:    end,
	}), "asking the last row's field for its menu opens it")
	var order []int
	screen.Do(func() {
		for _, row := range e.rows {
			order = append(order, e.table.RowToIndex(row))
		}
	})
	c.Equal([]int{1, 2, 0}, order, "the test needs the commit to have rotated the rows")
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.True(slices.Contains(items, "Copy"), "the field's menu, not %v", items)
	s = e.snapshot(c, screen, 2, 0)
	c.True(s.fieldFocused, "the field asked for takes the focus")
	c.True(s.installed, "and its cell stays attached to the table")
	c.Equal(0, s.focusRow, "at the index its row has once the rows are reordered")
	c.Equal(0, s.focusCol)
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table), "with its row selected")
	if start != end {
		c.Equal(start, s.selStart, "and the range asked for selected")
		c.Equal(end, s.selEnd)
	}
	var fieldFrame, cellFrame geom.Rect
	screen.Do(func() {
		fieldFrame = e.fields[2][0].RectToRoot(e.fields[2][0].ContentRect(false))
		cellFrame = e.table.RectToRoot(e.table.CellFrame(0, 0))
	})
	c.True(fieldFrame.Intersects(cellFrame), "the field is drawn where its cell now is, at %v, not %v", cellFrame,
		fieldFrame)
	menuNodes := axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu)
	c.Equal(1, len(menuNodes))
	if len(menuNodes) == 1 {
		c.True(menuNodes[0].Bounds.Y >= cellFrame.Y && menuNodes[0].Bounds.Y <= cellFrame.Bottom()+1,
			"and the menu opens beneath the field there, at %v rather than within %v", menuNodes[0].Bounds,
			cellFrame)
	}
	cmCloseMenu(c, screen, wnd)

	// A commit that removes the row instead: the request is refused, and the focus goes back to the table.
	screen.Do(func() {
		e.model.SetRootRows(slices.Clone(e.rows))
		e.table.SyncToModel()
		e.table.FocusCell(0, 0)
		e.fields[0][0].LostFocusCallback = func() {
			if commit != nil {
				commit()
			}
			e.model.SetRootRows(e.rows[:2])
			e.table.SyncToModel()
		}
	})
	s = e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the test needs the first row's field focused again")
	tree = screen.AccessibilityTree(wnd)
	rows = axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	field = cmCellContent(tree, axMustNode(c, rows["r2"]), 0)
	c.Equal(1, len(field))
	if len(field) != 1 {
		return
	}
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   field[0].ID,
		Action: accessibility.ShowContextMenu,
		Start:  start,
		End:    end,
	}), "a field whose row the commit removed is refused")
	var count int
	screen.Do(func() { count = e.table.LastRowIndex() + 1 })
	c.Equal(2, count, "the test needs the commit to have removed the row")
	menus, _ = cmOpenMenu(c, screen, wnd)
	c.Equal(0, menus)
	s = e.snapshot(c, screen, 2, 0)
	c.False(s.fieldFocused, "the field no row shows any more does not keep the focus")
	c.True(s.detached, "and is detached")
	c.True(s.tableFocused, "the table holds the focus instead")
	c.Equal(-1, s.focusRow)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuRowRequestAfterASelectionChangeReordersTheRows verifies that an assistive technology's request for a
// row's menu opens it beneath the row at the index it has once the row was selected for it, since a
// SelectionChangedCallback may reorder the rows.
func TestContextMenuRowRequestAfterASelectionChangeReordersTheRows(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var calls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 1, "abc", false)
			e.table.ContextMenuCallback = cmMenu(&calls, cmTableTitle)
			e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			rotated := false
			e.table.SelectionChangedCallback = func() {
				if !rotated {
					rotated = true
					cmRotateRows(e)
				}
			}
			wnd = newHeadlessWindow(t, "reordered rows", geom.NewRect(10, 10, 500, 500), axColumn(e.table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	rows := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   axMustNode(c, rows["r2"]).ID,
		Action: accessibility.ShowContextMenu,
	}), "asking the last row for its menu opens it")
	menus, _ := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	var index int
	var want geom.Point
	c.True(screen.Do(func() {
		index = e.table.RowToIndex(e.rows[2])
		frame := e.table.RowFrame(index)
		want = geom.NewPoint(frame.X, frame.Bottom())
	}))
	c.Equal(0, index, "the test needs the rows rotated by the selection change")
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table), "the row asked for is selected, at its new index")
	c.Equal(want, cmLast(screen, &calls), "and the menu opens beneath it there")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuCellWidgetRequestAfterAFieldCommitWithWrappedFields verifies that a row which builds a fresh wrapper
// around its memoized field on every call has the newest wrapper adopted as the focused cell when the field takes the
// focus for its menu while a field in another cell is being edited, whose commit moves the field into a new wrapper,
// for an assistive technology's request and for a right-click in a table that cannot hold the focus alike.
func TestContextMenuCellWidgetRequestAfterAFieldCommitWithWrappedFields(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var calls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 1, "abc", true)
			e.table.ContextMenuCallback = cmMenu(&calls, cmTableTitle)
			e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			wnd = newHeadlessWindow(t, "wrapped rows", geom.NewRect(10, 10, 500, 500), axColumn(e.table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	screen.Click(cellCenter(screen, e.table, 0, 0))
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the test needs the field focused")
	c.True(s.wrapped, "in a wrapper of its own")
	screen.Do(func() { cmCommitRotatesRows(e, 0, 0) })
	tree := screen.AccessibilityTree(wnd)
	rows := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	wrapper := cmCellContent(tree, axMustNode(c, rows["r2"]), 0)
	c.Equal(1, len(wrapper), "the cell holds the wrapper")
	if len(wrapper) != 1 {
		return
	}
	field := axChildNodes(tree, wrapper[0])
	c.Equal(1, len(field), "and the wrapper the field")
	if len(field) != 1 {
		return
	}
	c.Equal(role.TextField, field[0].Role)
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   field[0].ID,
		Action: accessibility.ShowContextMenu,
	}), "asking the last row's field for its menu opens it")
	var order []int
	screen.Do(func() {
		for _, row := range e.rows {
			order = append(order, e.table.RowToIndex(row))
		}
	})
	c.Equal([]int{1, 2, 0}, order, "the test needs the commit to have rotated the rows")
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.True(slices.Contains(items, "Copy"), "the field's menu, not %v", items)
	s = e.snapshot(c, screen, 2, 0)
	c.True(s.fieldFocused, "the field asked for takes the focus")
	c.True(s.wrapped, "and the newest wrapper around it is the cell attached to the table")
	c.Equal(0, s.focusRow, "at the index its row has once the rows are reordered")
	c.Equal(0, s.focusCol)
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table), "with its row selected")
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())

	// A right-click in a table that cannot hold the focus: the edited field commits only as the right-clicked field
	// takes the focus, so the wrapper built for the press is replaced while the press is being resolved.
	screen.Do(func() {
		e.model.SetRootRows(slices.Clone(e.rows))
		e.table.SyncToModel()
		e.table.SetFocusable(false)
		e.table.FocusCell(0, 0)
	})
	s = e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the test needs the first row's field focused again")
	c.Equal(0, s.focusRow)
	screen.MouseDown(cellCenter(screen, e.table, 2, 0), unison.ButtonRight, mod.None)
	order = nil
	screen.Do(func() {
		for _, row := range e.rows {
			order = append(order, e.table.RowToIndex(row))
		}
	})
	c.Equal([]int{1, 2, 0}, order, "the test needs the commit to have rotated the rows")
	s = e.snapshot(c, screen, 2, 0)
	c.True(s.fieldFocused, "the right-clicked field takes the focus")
	c.True(s.wrapped, "and the newest wrapper around it is the cell attached to the table")
	c.Equal(0, s.focusRow, "at the index its row has once the rows are reordered")
	c.Equal(0, s.focusCol)
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table), "with its row selected")
	s = e.snapshot(c, screen, 1, 0)
	c.False(s.fieldFocused, "and not the field of the row that took its old index")
	screen.MouseUp(cellCenter(screen, e.table, 0, 0), unison.ButtonRight, mod.None)
	menus, items = cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "the release over the field where its row now is opens its menu")
	c.True(slices.Contains(items, "Copy"), "the field's menu, not the table's: %v", items)
	c.Equal(0, cmCount(screen, &calls), "and the table was not asked for its own")
	s = e.snapshot(c, screen, 2, 0)
	c.True(s.fieldFocused, "the field keeps the focus")
	c.True(s.wrapped)
	c.Equal(0, s.focusRow)
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuCellWidgetRequestAfterASelectionChangeReordersTheRows verifies that an assistive technology's request
// for the menu of a field in a cell opens the menu beneath the field where its cell is drawn once the row was selected
// for it, since adopting the cell selects its row and a SelectionChangedCallback may reorder the rows: the cell is
// placed at the frame its row has by then rather than at the one it had before.
func TestContextMenuCellWidgetRequestAfterASelectionChangeReordersTheRows(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 1, "abc", false)
			e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			rotated := false
			e.table.SelectionChangedCallback = func() {
				if !rotated {
					rotated = true
					cmRotateRows(e)
				}
			}
			wnd = newHeadlessWindow(t, "reordered rows", geom.NewRect(10, 10, 500, 500), axColumn(e.table))
			if wnd != nil {
				wnd.ToFront()
				e.table.RequestFocus()
			}
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	s := e.snapshot(c, screen, 2, 0)
	c.True(s.tableFocused, "the test needs the table focused")
	tree := screen.AccessibilityTree(wnd)
	rows := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	field := cmCellContent(tree, axMustNode(c, rows["r2"]), 0)
	c.Equal(1, len(field))
	if len(field) != 1 {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   field[0].ID,
		Action: accessibility.ShowContextMenu,
	}), "asking the last row's field for its menu opens it")
	var order []int
	screen.Do(func() {
		for _, row := range e.rows {
			order = append(order, e.table.RowToIndex(row))
		}
	})
	c.Equal([]int{1, 2, 0}, order, "the test needs the selection change to have rotated the rows")
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus)
	c.True(slices.Contains(items, "Copy"), "the field's menu, not %v", items)
	s = e.snapshot(c, screen, 2, 0)
	c.True(s.fieldFocused, "the field asked for takes the focus")
	c.True(s.installed, "and its cell stays attached to the table")
	c.Equal(0, s.focusRow, "at the index its row has once the rows are reordered")
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table), "with its row selected")
	var cellFrame geom.Rect
	screen.Do(func() { cellFrame = e.table.RectToRoot(e.table.CellFrame(0, 0)) })
	menuNodes := axNodesWithRole(screen.AccessibilityTree(wnd), role.Menu)
	c.Equal(1, len(menuNodes))
	if len(menuNodes) == 1 {
		c.True(menuNodes[0].Bounds.Y >= cellFrame.Y && menuNodes[0].Bounds.Y <= cellFrame.Bottom()+1,
			"the menu opens beneath the field where its cell now is, at %v rather than within %v", menuNodes[0].Bounds,
			cellFrame)
	}
	cmCloseMenu(c, screen, wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityCellFocusRequestAfterAFieldCommitReordersTheRows verifies that an assistive technology's request to
// focus a field in a cell, made while a field in another cell is being edited, adopts the field's cell at the index its
// row has once the edited field has committed and reordered the rows, and selects that row, as a request for the
// field's menu does; the row that took the field's old index is left alone, and the field keeps the focus across the
// draw that follows.
func TestAccessibilityCellFocusRequestAfterAFieldCommitReordersTheRows(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 1, "abc", false)
			e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			wnd = newHeadlessWindow(t, "reordered rows", geom.NewRect(10, 10, 500, 500), axColumn(e.table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	screen.Click(cellCenter(screen, e.table, 0, 0))
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the test needs the field focused")
	screen.Do(func() { cmCommitRotatesRows(e, 0, 0) })
	tree := screen.AccessibilityTree(wnd)
	rows := axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(e.table)))
	field := cmCellContent(tree, axMustNode(c, rows["r2"]), 0)
	c.Equal(1, len(field))
	if len(field) != 1 {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   field[0].ID,
		Action: accessibility.Focus,
	}), "asking to focus the last row's field is carried out")
	var order []int
	screen.Do(func() {
		for _, row := range e.rows {
			order = append(order, e.table.RowToIndex(row))
		}
		wnd.FlushDrawing()
	})
	c.Equal([]int{1, 2, 0}, order, "the test needs the commit to have rotated the rows")
	s = e.snapshot(c, screen, 2, 0)
	c.True(s.fieldFocused, "the field asked for holds the focus, across the draw that followed")
	c.True(s.installed, "and its cell stays attached to the table")
	c.Equal(0, s.focusRow, "at the index its row has once the rows are reordered")
	c.Equal(0, s.focusCol)
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table), "with its row selected")
	s = e.snapshot(c, screen, 1, 0)
	c.False(s.fieldFocused, "and not the field of the row that took its old index")
	c.True(s.detached, "whose cell was handed back")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestContextMenuFocusedCellFollowsTheCellRule verifies that the panels of the cell holding the keyboard focus, which
// are described under their own ids, are offered accessibility.ShowContextMenu by the table's rule for every cell: a
// panel that cannot take the focus and holds nothing that can is not offered it, and nothing in the cell is offered it
// while the table is disabled.
func TestContextMenuFocusedCellFollowsTheCellRule(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var label *unison.Label
	var labelCalls, tableCalls cmRequests
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(2, 1, "abc", false)
			e.table.ContextMenuCallback = cmMenu(&tableCalls, cmTableTitle)
			label = unison.NewLabel()
			label.SetTitle("beside")
			label.ContextMenuCallback = cmMenu(&labelCalls, "Label")
			wrapper := unison.NewPanel()
			wrapper.SetLayout(&unison.FlexLayout{Columns: 2, HSpacing: 4})
			wrapper.AddChild(e.fields[1][0])
			wrapper.AddChild(label)
			build := e.rows[1].cellFactory
			e.rows[1].cellFactory = func(r, col int) unison.Paneler {
				if col == 0 {
					return wrapper
				}
				return build(r, col)
			}
			e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
			wnd = newHeadlessWindow(t, "focused cell", geom.NewRect(10, 10, 500, 500), axColumn(e.table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	var labelCenter geom.Point
	screen.Do(func() {
		e.table.FocusCell(1, 0)
		labelCenter = label.ContentRect(false).Center()
	})
	s := e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused, "the test needs the field focused")
	c.True(s.wrapped, "with its cell attached to the table")
	screen.AccessibilityTree(wnd)
	fieldNode := axMustNode(c, screen.AccessibilityNodeFor(e.fields[1][0]), "the field is described under its own id")
	labelNode := axMustNode(c, screen.AccessibilityNodeFor(label), "and so is the label beside it")
	c.True(fieldNode.Actions.Has(accessibility.ShowContextMenu), "the field in the focused cell offers its menu")
	c.False(labelNode.Actions.Has(accessibility.ShowContextMenu),
		"the label, which cannot take the focus, does not offer its own, as it would not in any other cell")
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   labelNode.ID,
		Action: accessibility.ShowContextMenu,
	}), "so a request for it is refused")
	c.Equal(0, cmCount(screen, &labelCalls), "and the label is not asked")
	screen.ClickWith(cmScreenPoint(c, screen, label, labelCenter), unison.ButtonRight, mod.None)
	menus, items := cmOpenMenu(c, screen, wnd)
	c.Equal(1, menus, "a right-click on the label opens a menu")
	c.Equal([]string{cmTableTitle}, items, "the table's, as it is for a panel that cannot take the focus")
	c.Equal(0, cmCount(screen, &labelCalls))
	cmCloseMenu(c, screen, wnd)

	screen.Do(func() {
		e.table.FocusCell(1, 0)
		e.table.SetEnabled(false)
	})
	s = e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused, "the test needs the field focused in the disabled table")
	c.True(s.wrapped)
	screen.AccessibilityTree(wnd)
	fieldNode = axMustNode(c, screen.AccessibilityNodeFor(e.fields[1][0]))
	c.False(fieldNode.Actions.Has(accessibility.ShowContextMenu),
		"a field in the focused cell of a disabled table does not offer its menu, as it would not in any other cell")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// cmPanickingPressPanel is a focusable panel with a menu whose ContextMenuPressHandler panics.
type cmPanickingPressPanel struct {
	unison.Panel
}

func newCMPanickingPressPanel() *cmPanickingPressPanel {
	p := &cmPanickingPressPanel{}
	p.Self = p
	cmFixedFocusable(p.AsPanel())
	return p
}

// ContextMenuPressed implements unison.ContextMenuPressHandler.
func (p *cmPanickingPressPanel) ContextMenuPressed(_ geom.Point, _ mod.Modifiers) bool {
	panic("no idea what to do with the press")
}
