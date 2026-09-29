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
	"strconv"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/gradienttype"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests cover the keyboard focus a disabled control takes for a screen reader's sake while staying disabled in
// every other respect; see SetFocusForReading and Panel.axDisabledTakesFocus. They share the helpers of
// static_text_focus_internal_test.go. A session owns most of the package's mutable globals while it runs, so none of
// these may call t.Parallel.

// disabledField returns a disabled field holding text that names itself, so that no label is needed to caption it.
func disabledField(name, text string) *Field {
	f := staticTextNamedField(name)
	f.SetText(text)
	f.SetEnabled(false)
	return f
}

func disabledButton(title string, clicked *int) *Button {
	b := NewButton()
	b.SetTitle(title)
	b.ClickCallback = func() { *clicked++ }
	b.SetEnabled(false)
	return b
}

func disabledCheckBox(title string) *CheckBox {
	cb := NewCheckBox()
	cb.SetTitle(title)
	cb.SetEnabled(false)
	return cb
}

func disabledFieldText(screen *HeadlessScreen, f *Field) string {
	var text string
	screen.Do(func() { text = f.Text() })
	return text
}

func disabledFocus(screen *HeadlessScreen, wnd *Window) *Panel {
	var p *Panel
	screen.Do(func() { p = wnd.CurrentFocus() })
	return p
}

// disabledCheckStaysDisabled checks that a panel goes on reporting that it is disabled, which it must whether or not
// it can take the focus.
func disabledCheckStaysDisabled(c check.Checker, screen *HeadlessScreen, p Paneler, what string) {
	var enabled bool
	screen.Do(func() { enabled = p.AsPanel().Enabled() })
	c.False(enabled, "%s: Enabled", what)
}

// disabledNode returns the last published node for p, without describing the window again, failing the test if there
// is none, so that what is then checked of the node cannot be passed over for want of one.
func disabledNode(c check.Checker, screen *HeadlessScreen, p Paneler, what string) *accessibility.Node {
	node := screen.AccessibilityNodeFor(p)
	c.NotNil(node, "%s should have been described", what)
	return node
}

// disabledStdMenus gives a window the standard menus, whose commands and key equivalents are routed to whatever holds
// the keyboard focus ahead of the keys themselves; see RouteActionToFocusExecuteFunc. A test of what reaches a disabled
// control that holds the focus is not complete without them.
func disabledStdMenus(wnd *Window) {
	if wnd != nil {
		DefaultMenuFactory().BarForWindow(wnd, func(m Menu) { InsertStdMenus(m, nil, nil, nil) })
	}
}

// disabledCheckNode checks, without describing the window again, that the last published node for p is disabled, and
// that it can be given the focus, and offers that and nothing else it may not, exactly when reachable says so.
func disabledCheckNode(c check.Checker, screen *HeadlessScreen, p Paneler, reachable bool, what string) {
	node := disabledNode(c, screen, p, what)
	if node == nil {
		return
	}
	c.True(node.Disabled, "%s: Disabled", what)
	c.Equal(reachable, node.Focusable, "%s: Focusable", what)
	want := axDisabledActions
	if reachable {
		want = want.With(accessibility.Focus)
	}
	c.Equal(want, node.Actions, "%s: actions offered", what)
}

// TestDisabledControlsAreOutOfReachByDefault verifies that a disabled control is what it always was while no assistive
// technology is being served, whatever the switch says, and while one is being served but the switch is off.
func TestDisabledControlsAreOutOfReachByDefault(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var editable, field *Field
	var button *Button
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			field = disabledField("Total", "42")
			button = disabledButton("Apply", &clicked)
			wnd = axNewTestWindow(t, "out of reach", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(editable, field, button))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	c.False(staticTextFocusable(screen, field), "a disabled field is not a tab stop")
	c.False(staticTextFocusable(screen, button), "nor is a disabled button")

	SetFocusForReading(true)
	screen.Sync()
	c.False(IsAccessibilityActive(), "turning the switch on must not start accessibility support")
	c.False(staticTextFocusable(screen, field), "the switch alone changes nothing while nothing is listening")
	c.False(staticTextFocusable(screen, button))
	screen.Do(func() { wnd.SetFocus(editable) })
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(editable), "and Tab never stops at either")
	c.Equal(uint64(0), staticTextSnapshots(screen), "nothing may be described while nothing is listening")
	SetFocusForReading(false)
	screen.Sync()

	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	c.False(staticTextFocusable(screen, field), "with the switch off, being listened to changes nothing either")
	c.False(staticTextFocusable(screen, button))
	disabledCheckNode(c, screen, field, false, "the disabled field with the switch off")
	disabledCheckNode(c, screen, button, false, "the disabled button with the switch off")
	if node := disabledNode(c, screen, field, "the disabled field"); node != nil {
		c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.Focus,
		}), "a request for the focus is refused")
	}
	c.True(disabledFocus(screen, wnd).Is(editable))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledControlsTakeTheFocusForReading verifies the switch while an assistive technology is being served: each
// kind of disabled control becomes a tab stop in the order it sits in the window, is published as disabled but able to
// take the focus, reports the focus when it holds it, and is given the focus when a screen reader asks. None of them
// stops reporting that it is disabled.
func TestDisabledControlsTakeTheFocusForReading(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var editable, field, combo *Field
	var numeric *NumericField[int]
	var button *Button
	var checkBox *CheckBox
	var radio *RadioButton
	var popup *PopupMenu[string]
	var list *List[string]
	var wnd *Window
	first := "First"
	second := "Second"
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			field = disabledField("Total", "42")
			numeric = NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			numeric.BlinkRate = editable.BlinkRate
			numeric.Accessibility.Name = "Count"
			numeric.SetEnabled(false)
			combo = NewComboField([]*string{&first, &second}, &first, nil)
			combo.BlinkRate = editable.BlinkRate
			combo.Accessibility.Name = "Choice"
			combo.SetEnabled(false)
			button = disabledButton("Apply", &clicked)
			checkBox = disabledCheckBox("Remember")
			radio = NewRadioButton()
			radio.SetTitle("Always")
			radio.SetEnabled(false)
			popup = NewPopupMenu[string]()
			popup.AddItem("One", "Two")
			popup.SelectIndex(0)
			popup.Accessibility.Name = "Number"
			popup.SetEnabled(false)
			list = NewList[string]()
			list.Append("Row one", "Row two")
			list.Accessibility.Name = "Entries"
			list.SetEnabled(false)
			wnd = axNewTestWindow(t, "for reading", geom.NewRect(10, 10, 700, 700),
				staticTextColumn(editable, field, numeric, combo, button, checkBox, radio, popup, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	controls := []struct {
		panel Paneler
		what  string
		role  role.Enum
	}{
		{panel: field, what: "the disabled field", role: role.TextField},
		{panel: numeric, what: "the disabled numeric field", role: role.SpinButton},
		{panel: combo, what: "the disabled combo field", role: role.ComboBox},
		{panel: button, what: "the disabled button", role: role.Button},
		{panel: checkBox, what: "the disabled check box", role: role.CheckBox},
		{panel: radio, what: "the disabled radio button", role: role.RadioButton},
		{panel: popup, what: "the disabled popup menu", role: role.PopupButton},
		{panel: list, what: "the disabled list", role: role.List},
	}

	// Described once before the switch, so that turning it on is what describes the window again.
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	before := staticTextSnapshots(screen)
	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextSnapshots(screen) > before, "turning the switch on describes the window again")

	for _, one := range controls {
		c.True(staticTextFocusable(screen, one.panel), "%s can take the focus", one.what)
		disabledCheckStaysDisabled(c, screen, one.panel, one.what)
		disabledCheckNode(c, screen, one.panel, true, one.what)
		if node := disabledNode(c, screen, one.panel, one.what); node != nil {
			c.Equal(one.role, node.Role, "%s: Role", one.what)
			c.False(node.Focused, "%s does not hold the focus yet", one.what)
		}
	}
	if node := disabledNode(c, screen, field, "the disabled field"); node != nil {
		c.Equal("42", node.Value, "what the field holds is there to be read")
	}

	screen.Do(func() { wnd.SetFocus(editable) })
	for _, one := range controls {
		screen.KeyPress(KeyTab, mod.None)
		c.True(disabledFocus(screen, wnd).Is(one.panel), "Tab stops at %s", one.what)
		tree := screen.AccessibilityTree(wnd)
		node := screen.AccessibilityNodeFor(one.panel)
		c.NotNil(tree)
		c.NotNil(node)
		if tree != nil && node != nil {
			c.True(node.Focused, "%s says it holds the focus", one.what)
			c.Equal(node.ID, tree.Focus, "and the window reports the focus on %s", one.what)
			c.True(node.Disabled, "%s is still published as disabled", one.what)
		}
	}
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(editable), "and Tab wraps around onto the field that can be typed into")
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(disabledFocus(screen, wnd).Is(list), "Shift-Tab goes back the other way")

	screen.Do(func() { wnd.SetFocus(editable) })
	c.NotNil(screen.AccessibilityTree(wnd))
	if node := disabledNode(c, screen, button, "the disabled button"); node != nil {
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.Focus,
		}), "a screen reader asking for the focus on a disabled control gets it")
		c.True(disabledFocus(screen, wnd).Is(button))
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.ScrollIntoView,
		}), "and may scroll it into view, as it always could")
	}
	c.Equal(0, clicked)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledControlsStayDisabled verifies that holding the focus changes nothing else about a disabled control:
// nothing typed, pressed or clicked reaches it, the keys pressed while it holds the focus go to the panel it is inside,
// the commands of the menus and their key equivalents pass over it, a click does not move the focus onto it, and an
// assistive technology is refused everything other than the focus. The window has the standard menus, since their key
// equivalents are run ahead of the keys.
func TestDisabledControlsStayDisabled(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	const original = "forty-two"
	clicked := 0
	modified := 0
	var keys []KeyCode
	var runes []rune
	var editable, field *Field
	var numeric *NumericField[int]
	var button *Button
	var checkBox *CheckBox
	var list *List[string]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			field = disabledField("Total", original)
			field.ModifiedCallback = func(_, _ *FieldState) { modified++ }
			numeric = NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			numeric.BlinkRate = editable.BlinkRate
			numeric.Accessibility.Name = "Count"
			numeric.SetEnabled(false)
			button = disabledButton("Apply", &clicked)
			checkBox = disabledCheckBox("Remember")
			list = NewList[string]()
			list.Append("Row one", "Row two")
			list.Accessibility.Name = "Entries"
			list.SetEnabled(false)
			column := staticTextColumn(editable, field, numeric, button, checkBox, list)
			column.KeyDownCallback = func(keyCode KeyCode, _ mod.Modifiers, _ bool) bool {
				if keyCode == KeyTab {
					return false
				}
				keys = append(keys, keyCode)
				return true
			}
			column.RuneTypedCallback = func(ch rune) bool {
				runes = append(runes, ch)
				return true
			}
			wnd = axNewTestWindow(t, "stays disabled", geom.NewRect(10, 10, 700, 700), column)
			disabledStdMenus(wnd)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	screen.Do(func() { wnd.SetFocus(field) })
	c.True(disabledFocus(screen, wnd).Is(field))
	var start, end int
	screen.Do(func() { start, end = field.Selection() })
	c.Equal(len(original), start, "taking the focus selects nothing in a disabled field")
	c.Equal(len(original), end)

	screen.Type("x")
	screen.KeyPress(KeyBackspace, mod.None)
	screen.KeyPress(KeyLeft, mod.None)
	screen.Do(func() { ClipboardSetText("pasted") })
	screen.KeyPress(KeyV, mod.OSMenuCommand())
	c.Equal(original, disabledFieldText(screen, field), "nothing typed or pressed changes the field")
	screen.Do(func() { start, end = field.Selection() })
	c.Equal(len(original), start, "or moves its caret")
	c.Equal(len(original), end)
	c.Equal([]rune{'x'}, runes, "what is typed goes to the panel the field is inside, as for any disabled focus")
	c.Equal([]KeyCode{KeyX, KeyBackspace, KeyLeft, KeyV}, keys, "and so do the keys")
	c.Equal(0, modified)

	// The commands of the menus are routed to the focus, and their key equivalents are run by the menu bar ahead of the
	// keys. Each passes over the disabled field, which has handlers installed for every one of them.
	var can []bool
	var clipboard string
	screen.Do(func() {
		for _, action := range []*Action{CutAction(), CopyAction(), PasteAction(), DeleteAction(), SelectAllAction()} {
			can = append(can, action.Enabled(nil))
		}
	})
	c.Equal([]bool{false, false, false, false, false}, can, "no command is offered for a disabled field")
	screen.KeyPress(KeyA, mod.OSMenuCommand())
	screen.Do(func() { start, end = field.Selection() })
	c.Equal(len(original), start, "select all selects nothing in it")
	c.Equal(len(original), end)
	screen.Do(func() {
		field.SetSelection(0, len(original))
		modified = 0
	})
	screen.KeyPress(KeyX, mod.OSMenuCommand())
	screen.KeyPress(KeyV, mod.OSMenuCommand())
	screen.Do(func() {
		DeleteAction().Execute(nil)
		PasteAction().Execute(nil)
		CutAction().Execute(nil)
		field.PerformCmd(nil, PasteItemID)
		clipboard = ClipboardGetText()
		field.SetSelectionToEnd()
	})
	c.Equal(original, disabledFieldText(screen, field), "nothing is cut from, pasted into or deleted from the field")
	c.Equal("pasted", clipboard, "and nothing of it was put on the clipboard")
	c.Equal(0, modified)

	screen.Do(func() {
		list.Select(false, 0)
		wnd.SetFocus(list)
	})
	c.True(disabledFocus(screen, wnd).Is(list))
	screen.KeyPress(KeyA, mod.OSMenuCommand())
	var selected int
	screen.Do(func() {
		SelectAllAction().Execute(nil)
		selected = list.Selection.Count()
	})
	c.Equal(1, selected, "select all does not select the rows of a disabled list")

	screen.Do(func() { wnd.SetFocus(button) })
	screen.KeyPress(KeySpace, mod.None)
	c.Equal(0, clicked, "the key that clicks a button does nothing for a disabled one that holds the focus")
	screen.Do(func() { wnd.SetFocus(editable) })
	screen.Click(screen.PanelCenter(button))
	c.Equal(0, clicked, "nor does a click")
	c.True(disabledFocus(screen, wnd).Is(editable), "which does not move the focus there either")
	screen.Click(screen.PanelCenter(checkBox))
	var state checkenum.Enum
	screen.Do(func() { state = checkBox.State })
	c.Equal(checkenum.Off, state, "a disabled check box is not checked by a click")
	screen.Click(screen.PanelCenter(field))
	c.True(disabledFocus(screen, wnd).Is(editable), "and a click on a disabled field leaves the focus where it was")

	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	refused := func(p Paneler, what string, requests ...accessibility.ActionRequest) {
		node := screen.AccessibilityNodeFor(p)
		c.NotNil(node, "%s should have been described", what)
		if node == nil {
			return
		}
		for _, req := range requests {
			req.Node = node.ID
			c.False(screen.PerformAccessibilityAction(req), "%s refuses %s", what, req.Action)
		}
	}
	refused(field, "the disabled field",
		accessibility.ActionRequest{Action: accessibility.SetValue, Value: "replaced"},
		accessibility.ActionRequest{Action: accessibility.ReplaceText, Start: 0, End: 2, Value: "replaced"},
		accessibility.ActionRequest{Action: accessibility.SetTextSelection, Start: 1, End: 4},
		accessibility.ActionRequest{Action: accessibility.ShowContextMenu})
	refused(numeric, "the disabled numeric field",
		accessibility.ActionRequest{Action: accessibility.Increment},
		accessibility.ActionRequest{Action: accessibility.Decrement},
		accessibility.ActionRequest{Action: accessibility.SetValue, Value: "7", Number: 7})
	refused(button, "the disabled button", accessibility.ActionRequest{Action: accessibility.Press})
	refused(checkBox, "the disabled check box",
		accessibility.ActionRequest{Action: accessibility.Press},
		accessibility.ActionRequest{Action: accessibility.Toggle})
	c.Equal(original, disabledFieldText(screen, field))
	c.Equal("5", disabledFieldText(screen, numeric.Field))
	c.Equal(0, clicked)
	screen.Do(func() { state = checkBox.State })
	c.Equal(checkenum.Off, state)

	// The rows of a disabled list are no more reachable than they were: the list itself is what takes the focus.
	if tree != nil {
		rows := 0
		tree.Walk(func(n *accessibility.Node) bool {
			if n.Role == role.ListItem {
				rows++
				c.True(n.Disabled, "a row of a disabled list is disabled")
				c.False(n.Actions.Has(accessibility.Focus), "and does not offer the focus")
				c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
					Node:   n.ID,
					Action: accessibility.Focus,
				}), "which it refuses")
			}
			return true
		})
		c.Equal(2, rows, "both rows of the list should have been described")
	}
	c.True(disabledFocus(screen, wnd).Is(editable))

	// Out of range, which a field being edited would bring back into range as it lost the focus.
	screen.Do(func() {
		numeric.SetText("12")
		wnd.SetFocus(numeric)
	})
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(button))
	c.Equal("12", disabledFieldText(screen, numeric.Field), "losing the focus leaves what the application put there")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledControlsSeedTheFocusLast verifies that a disabled control is the last choice when a window with nothing
// focused chooses where to put the person, as static text is: a window that also holds a control that can be used
// opens there from either end, a container is resolved the same way, and a window holding nothing else opens in its
// first disabled control.
func TestDisabledControlsSeedTheFocusLast(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var lead, editable, only *Field
	var trailing, last *Button
	var column *Panel
	var wnd, disabledWindow *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			lead = disabledField("Subtotal", "40")
			editable = staticTextNamedField("Discount")
			trailing = disabledButton("Apply", &clicked)
			column = staticTextColumn(lead, editable, trailing)
			wnd = axNewTestWindow(t, "disabled and enabled", geom.NewRect(10, 10, 500, 300), column)

			only = disabledField("Total", "42")
			last = disabledButton("Recalculate", &clicked)
			disabledWindow = axNewTestWindow(t, "disabled alone", geom.NewRect(60, 60, 300, 200),
				staticTextColumn(only, last))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	c.NotNil(disabledWindow)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, lead))
	c.True(staticTextFocusable(screen, trailing))

	var seeded, seededBackward, firstChild, lastChild, seededAlone, seededAloneBackward *Panel
	screen.Do(func() {
		wnd.SetFocus(nil)
		wnd.FocusNext()
		seeded = wnd.CurrentFocus()
		wnd.SetFocus(nil)
		wnd.FocusPrevious()
		seededBackward = wnd.CurrentFocus()
		firstChild = column.FirstFocusableChild()
		lastChild = column.LastFocusableChild()
		disabledWindow.SetFocus(nil)
		disabledWindow.FocusNext()
		seededAlone = disabledWindow.CurrentFocus()
		disabledWindow.SetFocus(nil)
		disabledWindow.FocusPrevious()
		seededAloneBackward = disabledWindow.CurrentFocus()
	})
	c.True(seeded.Is(editable), "the window opens in the field that can be typed into")
	c.True(seededBackward.Is(editable), "from either end")
	c.True(firstChild.Is(editable), "and a container is resolved the same way")
	c.True(lastChild.Is(editable))
	c.True(seededAlone.Is(only), "a window of nothing but disabled controls opens in its first")
	c.True(seededAloneBackward.Is(last), "or its last, moving the other way")

	screen.Do(func() { wnd.SetFocus(editable) })
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(trailing), "Tab from a real focus stops at the disabled control after it")
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(lead), "and wraps around onto the one at the top")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledControlFocusRuntimeChanges verifies that whether a disabled control takes the focus follows the switch,
// the control and the assistive technology as they change: turning the switch off leaves the focus on the control but
// takes it out of the tab order and stops the focus being reported there, enabling the control gives back one that can
// be used, a hidden one takes no focus, and the assistive technology going away puts it out of reach again.
func TestDisabledControlFocusRuntimeChanges(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var before, field, after *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			before = staticTextNamedField("Search")
			field = disabledField("Total", "42")
			after = staticTextNamedField("Replace")
			wnd = axNewTestWindow(t, "runtime", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(before, field, after))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	screen.Do(func() { wnd.SetFocus(field) })
	c.True(disabledFocus(screen, wnd).Is(field))

	// Called off the UI thread, as a worker goroutine would.
	snapshots := staticTextSnapshots(screen)
	SetFocusForReading(false)
	screen.Sync()
	c.True(disabledFocus(screen, wnd).Is(field), "turning the switch off leaves the focus where it is")
	c.False(staticTextFocusable(screen, field), "but the field is no longer a tab stop")
	c.True(staticTextSnapshots(screen) > snapshots, "the window is described again to say so")
	disabledCheckNode(c, screen, field, false, "the field with the switch turned off")
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if node := disabledNode(c, screen, field, "the field with the switch turned off"); node != nil && tree != nil {
		c.False(node.Focused, "a disabled control that cannot take the focus is published without it")
		c.NotEqual(node.ID, tree.Focus, "and the focus is reported on something that can be shown")
	}
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(before), "so Tab lands on the window's first tab stop")
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(after), "and passes over the field from then on")

	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, field), "turning it back on makes the field a tab stop again")
	disabledCheckNode(c, screen, field, true, "the field with the switch back on")

	screen.Do(func() { field.Hidden = true })
	c.False(staticTextFocusable(screen, field), "a hidden control takes no focus")
	screen.Do(func() { field.Hidden = false })
	c.True(staticTextFocusable(screen, field))

	screen.Do(func() {
		field.SetEnabled(true)
		wnd.SetFocus(field)
		field.SetSelectionToEnd()
	})
	screen.Type("0")
	c.Equal("420", disabledFieldText(screen, field), "a control that is enabled again is used as any other")
	c.NotNil(screen.AccessibilityTree(wnd))
	if node := disabledNode(c, screen, field, "the field that was enabled again"); node != nil {
		c.False(node.Disabled)
		c.True(node.Focusable)
		c.True(node.Actions.Has(accessibility.SetValue))
	}

	screen.Do(func() {
		field.SetEnabled(false)
		SetAccessibilityEnabled(false)
	})
	c.False(IsAccessibilityActive())
	c.True(FocusForReading(), "the switch is still on")
	c.False(staticTextFocusable(screen, field), "but with nothing listening any more, the field is out of reach")
	screen.Do(func() { SetAccessibilityEnabled(true) })
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFocusForReadingOptsInOneDisabledControl verifies the per-panel opt-in: with the switch off, a disabled control
// that sets FocusForReading takes the focus while an assistive technology is being served, the one beside it does not,
// and the label before the first still captions it rather than becoming a tab stop.
func TestFocusForReadingOptsInOneDisabledControl(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var reading, plain *Field
	var caption *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			caption = staticTextLabel("Total:")
			reading = staticTextField()
			reading.SetText("42")
			reading.SetEnabled(false)
			reading.Accessibility.FocusForReading = true
			plain = disabledField("Subtotal", "40")
			wnd = axNewTestWindow(t, "opt in", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(caption, reading, plain, staticTextNamedField("Discount")))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	c.False(staticTextFocusable(screen, reading), "with nothing listening, even an opted-in control is out of reach")
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	c.False(FocusForReading(), "the switch itself is still off")
	c.True(staticTextFocusable(screen, reading), "the control that asked takes the focus")
	c.False(staticTextFocusable(screen, plain), "the one beside it, which did not ask, does not")
	disabledCheckNode(c, screen, reading, true, "the opted-in field")
	disabledCheckNode(c, screen, plain, false, "the field that did not opt in")
	if node := disabledNode(c, screen, reading, "the opted-in field"); node != nil {
		c.Equal("Total", node.Name, "the label before the field still captions it")
	}
	c.False(staticTextFocusable(screen, caption), "and is spoken with it rather than being a tab stop")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledControlFocusExclusions verifies what the switch leaves alone: a disabled panel that would not be a tab
// stop were it enabled, a disabled control in a cell of a Table or List, which those reach through a cursor of their
// own, one inside a document, one inside a heading or a label, and one that is in no window. A disabled control inside
// a panel marked up as any other kind of element is its own element and takes the focus, and so does standalone static
// text that is disabled.
func TestDisabledControlFocusExclusions(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var plain, reader *Panel
	var inReader, inToolbar, detached *Field
	var inHeading, inLabel *Button
	var text, caption *Label
	var captioned *Field
	var table *Table[*axHeaderRow]
	var list *List[string]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			plain = NewPanel()
			plain.Accessibility.Name = "Just a panel"
			plain.SetEnabled(false)

			// A stand-in for a document holding a field, which a real Markdown never does, so that the ancestor rule is
			// the only thing keeping the field out.
			reader = NewPanel()
			reader.SetLayout(&FlexLayout{Columns: 1})
			reader.axFocusable = true
			inReader = disabledField("Inside a document", "1")
			reader.AddChild(inReader)

			toolbar := NewPanel()
			toolbar.SetLayout(&FlexLayout{Columns: 1})
			toolbar.Accessibility.Role = role.Toolbar
			inToolbar = disabledField("Inside a toolbar", "2")
			toolbar.AddChild(inToolbar)

			// What is inside static text is folded into its name and never described, so a control there has no element
			// for the focus to be reported on.
			heading := NewPanel()
			heading.SetLayout(&FlexLayout{Columns: 2})
			heading.Accessibility.Role = role.Heading
			heading.Accessibility.Level = 1
			heading.AddChild(staticTextLabel("Section"))
			inHeading = disabledButton("Edit", &clicked)
			heading.AddChild(inHeading)
			marked := NewPanel()
			marked.SetLayout(&FlexLayout{Columns: 2})
			marked.Accessibility.Role = role.Label
			marked.AddChild(staticTextLabel("Status"))
			inLabel = disabledButton("Refresh", &clicked)
			marked.AddChild(inLabel)

			text = staticTextLabel("This section cannot be changed.")
			text.SetEnabled(false)
			caption = staticTextLabel("Total:")
			caption.SetEnabled(false)
			captioned = staticTextField()
			captioned.SetText("42")
			captioned.SetEnabled(false)
			detached = disabledField("Elsewhere", "7")

			model := &SimpleTableModel[*axHeaderRow]{}
			model.SetRootRows([]*axHeaderRow{{id: "a"}})
			table = NewTable(model)
			table.Columns = []ColumnInfo{{ID: 0, Current: 100}}
			table.SyncToModel()
			list = NewList[string]()
			list.Append("row")

			wnd = axNewTestWindow(t, "exclusions", geom.NewRect(10, 10, 700, 700),
				staticTextColumn(plain, reader, toolbar, heading, marked, text, caption, captioned, table, list,
					disabledButton("Apply", &clicked)))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	// Described again, so that the caption has been recorded as one; see Panel.axStaticTextTakesFocus.
	c.NotNil(screen.AccessibilityTree(wnd))

	c.False(staticTextFocusable(screen, plain), "a panel that is no tab stop when enabled is none when disabled")
	c.False(staticTextFocusable(screen, inReader), "a document reads its own content")
	c.False(staticTextFocusable(screen, detached), "a control in no window has nothing to take the focus in")
	c.True(staticTextFocusable(screen, inToolbar), "a control inside some other kind of element is still its own")
	c.False(staticTextFocusable(screen, inHeading), "one inside a heading is folded into it and never described")
	c.False(staticTextFocusable(screen, inLabel), "and so is one inside a panel marked up as a label")
	c.Nil(screen.AccessibilityNodeFor(inHeading), "which is why it must not be a tab stop")
	c.Nil(screen.AccessibilityNodeFor(inLabel))
	c.True(staticTextFocusable(screen, text), "standalone static text takes the focus whether or not it is disabled")
	c.False(staticTextFocusable(screen, caption), "a caption is spoken with its control, disabled or not")
	c.True(staticTextFocusable(screen, captioned), "and its control is what takes the focus")
	disabledCheckNode(c, screen, inToolbar, true, "the field inside the toolbar")
	disabledCheckNode(c, screen, text, true, "the disabled label")
	disabledCheckNode(c, screen, caption, false, "the disabled caption")
	disabledCheckNode(c, screen, plain, false, "the disabled plain panel")
	if node := disabledNode(c, screen, captioned, "the captioned field"); node != nil {
		c.Equal("Total", node.Name, "the disabled caption names the disabled field")
	}

	var inTable, inList bool
	screen.Do(func() {
		// A cell is attached only while drawn or described, so each is asked while installed.
		rect := geom.NewRect(0, 0, 100, 20)
		cellFocusable := func(install, uninstall func(p *Panel)) bool {
			cell := disabledField("In a cell", "9").AsPanel()
			install(cell)
			defer uninstall(cell)
			return cell.Focusable()
		}
		inTable = cellFocusable(func(p *Panel) { table.installCell(p, rect) }, func(p *Panel) { p.parent = nil })
		inList = cellFocusable(func(p *Panel) { list.installCell(p, rect) }, list.uninstallCell)
	})
	c.False(inTable, "a disabled control in a table cell is reached through the table's cell cursor")
	c.False(inList, "and one in a list row through the list's current row")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledFocusGetsNoKeyUps verifies that the release of a key is not delivered to a disabled control that holds
// the focus, which was passed over for the press, nor to a control that was disabled between the two.
func TestDisabledFocusGetsNoKeyUps(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	disabledUps := 0
	enabledUps := 0
	var editable, field *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			editable.KeyUpCallback = func(_ KeyCode, _ mod.Modifiers) bool {
				enabledUps++
				return true
			}
			field = disabledField("Total", "42")
			field.KeyUpCallback = func(_ KeyCode, _ mod.Modifiers) bool {
				disabledUps++
				return true
			}
			wnd = axNewTestWindow(t, "key ups", geom.NewRect(10, 10, 500, 300), staticTextColumn(editable, field))
			disabledStdMenus(wnd)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	screen.Do(func() { wnd.SetFocus(field) })
	c.True(disabledFocus(screen, wnd).Is(field))
	screen.KeyPress(KeyX, mod.None)
	screen.KeyPress(KeyLeft, mod.None)
	c.Equal(0, disabledUps, "no key release is delivered to a disabled control that holds the focus")

	screen.Do(func() { wnd.SetFocus(editable) })
	screen.KeyPress(KeyLeft, mod.None)
	c.Equal(1, enabledUps, "a control that can be used still hears of them")
	screen.KeyDown(KeyLeft, mod.None)
	screen.Do(func() { editable.SetEnabled(false) })
	screen.KeyUp(KeyLeft, mod.None)
	c.Equal(1, enabledUps, "but not of the release of a key it was disabled while holding down")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// disabledPixels returns the pixels the window last presented within the frame of p, drawing the window first.
func disabledPixels(screen *HeadlessScreen, wnd *Window, p Paneler) []byte {
	screen.Sync()
	img := screen.CaptureWindow(wnd)
	if img == nil {
		return nil
	}
	var rect geom.Rect
	screen.Do(func() { rect = p.AsPanel().RectToRoot(p.AsPanel().ContentRect(true)) })
	scale := screen.Scale()
	var pixels []byte
	for y := int(rect.Y * scale); y < int(rect.Bottom()*scale); y++ {
		for x := int(rect.X * scale); x < int(rect.Right()*scale); x++ {
			px := img.NRGBAAt(x, y)
			pixels = append(pixels, px.R, px.G, px.B, px.A)
		}
	}
	return pixels
}

// TestDisabledControlsShowTheFocus verifies that each kind of disabled control is drawn differently while it holds the
// focus, so that a person who can see the window can tell where a screen reader is, and is drawn as it was once the
// focus has gone.
func TestDisabledControlsShowTheFocus(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var editable, field, combo *Field
	var numeric *NumericField[int]
	var button *Button
	var checkBox *CheckBox
	var radio *RadioButton
	var popup *PopupMenu[string]
	var list *List[string]
	var link *Label
	var wnd *Window
	first := "Earliest"
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			field = disabledField("Total", "42")
			numeric = NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			numeric.BlinkRate = editable.BlinkRate
			numeric.Accessibility.Name = "Amount"
			numeric.SetEnabled(false)
			combo = NewComboField([]*string{&first}, &first, nil)
			combo.BlinkRate = editable.BlinkRate
			combo.Accessibility.Name = "Choice"
			combo.SetEnabled(false)
			button = disabledButton("Apply", &clicked)
			checkBox = disabledCheckBox("Remember")
			radio = NewRadioButton()
			radio.SetTitle("Always")
			radio.SetEnabled(false)
			popup = NewPopupMenu[string]()
			popup.AddItem("One", "Two")
			popup.SelectIndex(0)
			popup.Accessibility.Name = "Number"
			popup.SetEnabled(false)
			list = NewList[string]()
			list.Append("Row one", "Row two")
			list.Select(false, 0)
			list.Accessibility.Name = "Records"
			list.SetEnabled(false)
			link = NewLink("Documentation", "", "https://example.com/docs", nil, nil)
			link.SetEnabled(false)
			wnd = axNewTestWindow(t, "shows the focus", geom.NewRect(10, 10, 700, 700),
				staticTextColumn(editable, field, numeric, combo, button, checkBox, radio, popup, list, link))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	for _, one := range []struct {
		panel Paneler
		what  string
	}{
		{panel: field, what: "the disabled field"},
		{panel: numeric, what: "the disabled numeric field"},
		{panel: combo, what: "the disabled combo field"},
		{panel: button, what: "the disabled button"},
		{panel: checkBox, what: "the disabled check box"},
		{panel: radio, what: "the disabled radio button"},
		{panel: popup, what: "the disabled popup menu"},
		{panel: list, what: "the disabled list"},
		{panel: link, what: "the disabled link"},
	} {
		screen.Do(func() { wnd.SetFocus(editable) })
		unfocused := disabledPixels(screen, wnd, one.panel)
		c.True(len(unfocused) > 0, "%s should have been drawn", one.what)
		screen.Do(func() { wnd.SetFocus(one.panel) })
		c.True(disabledFocus(screen, wnd).Is(one.panel), "%s takes the focus", one.what)
		c.NotEqual(unfocused, disabledPixels(screen, wnd, one.panel), "%s shows that it holds the focus", one.what)
		screen.Do(func() { wnd.SetFocus(editable) })
		c.Equal(unfocused, disabledPixels(screen, wnd, one.panel), "%s is drawn as it was once the focus has gone",
			one.what)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledComboFieldButtonDoesNothing verifies that the dropdown button of a disabled combo field, which is a panel
// of its own that is not disabled along with the field, opens no menu and does not move the focus onto the field.
func TestDisabledComboFieldButtonDoesNothing(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	changed := 0
	first := "Primary"
	second := "Secondary"
	var editable, combo *Field
	var dropdown *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			combo = NewComboField([]*string{&first, &second}, &second, func(_ *string) { changed++ })
			combo.BlinkRate = editable.BlinkRate
			combo.Accessibility.Name = "Choice"
			combo.SetEnabled(false)
			if children := combo.Children(); len(children) == 1 {
				dropdown = children[0]
			}
			wnd = axNewTestWindow(t, "combo", geom.NewRect(10, 10, 500, 300), staticTextColumn(editable, combo))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	c.NotNil(dropdown, "the combo field should hold its dropdown button")
	screen.Sync()
	if dropdown == nil {
		return
	}

	click := func(what string) {
		screen.Do(func() { wnd.SetFocus(editable) })
		screen.Click(screen.PanelCenter(dropdown))
		var open int
		screen.Do(func() { open = len(wnd.root.openMenuPanels) })
		c.Equal(0, open, "%s: no menu is opened", what)
		c.True(disabledFocus(screen, wnd).Is(editable), "%s: the focus stays where it was", what)
		c.Equal(second, disabledFieldText(screen, combo), "%s: the value is left alone", what)
		c.Equal(0, changed, "%s: nothing is reported as changed", what)
	}
	click("with nothing listening")
	screen.EnableAccessibility()
	click("with the switch off")
	SetFocusForReading(true)
	screen.Sync()
	click("with the switch on")

	// The key that opens the choices goes to the panel the field is inside, as every key does.
	screen.Do(func() { wnd.SetFocus(combo) })
	c.True(disabledFocus(screen, wnd).Is(combo))
	screen.KeyPress(KeyDown, mod.None)
	var open int
	screen.Do(func() { open = len(wnd.root.openMenuPanels) })
	c.Equal(0, open)

	screen.Do(func() {
		combo.SetEnabled(true)
		wnd.SetFocus(editable)
	})
	screen.Click(screen.PanelCenter(dropdown))
	screen.Do(func() { open = len(wnd.root.openMenuPanels) })
	c.Equal(1, open, "the button of a combo field that is enabled again opens its choices")
	c.True(disabledFocus(screen, wnd).Is(combo), "and gives the field the focus")
	screen.KeyPress(KeyEscape, mod.None)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledDocumentTakesTheFocusLast verifies that a disabled document is a disabled control like any other: it
// takes the focus so that it can be read, answers no keys, is published as disabled, and is passed over for a document
// that can be read from when a window or a container chooses where to put the focus.
func TestDisabledDocumentTakesTheFocusLast(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	clicked := 0
	var button *Button
	var disabled, enabled *Markdown
	var column *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			button = disabledButton("Apply", &clicked)
			disabled = NewMarkdown(false)
			disabled.SetContent("This part cannot be used.\n", 400)
			disabled.SetEnabled(false)
			enabled = NewMarkdown(false)
			enabled.SetContent("This part can be read.\n", 400)
			column = staticTextColumn(button, disabled, enabled)
			wnd = axNewTestWindow(t, "documents", geom.NewRect(10, 10, 500, 400), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	c.True(staticTextFocusable(screen, enabled), "a document takes the focus while a screen reader is listening")
	c.False(staticTextFocusable(screen, disabled), "a disabled one does not while the switch is off")

	SetFocusForReading(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, disabled), "with the switch on, a disabled document takes the focus")
	disabledCheckStaysDisabled(c, screen, disabled, "the disabled document")
	disabledCheckNode(c, screen, disabled, true, "the disabled document")

	var seeded, seededBackward, firstChild, lastChild *Panel
	screen.Do(func() {
		wnd.SetFocus(nil)
		wnd.FocusNext()
		seeded = wnd.CurrentFocus()
		wnd.SetFocus(nil)
		wnd.FocusPrevious()
		seededBackward = wnd.CurrentFocus()
		firstChild = column.FirstFocusableChild()
		lastChild = column.LastFocusableChild()
	})
	c.True(seeded.Is(enabled), "the window opens in the document that can be read from")
	c.True(seededBackward.Is(enabled), "from either end")
	c.True(firstChild.Is(enabled), "and a container is resolved the same way")
	c.True(lastChild.Is(enabled))

	screen.Do(func() { wnd.SetFocus(disabled) })
	c.True(disabledFocus(screen, wnd).Is(disabled))
	var before, after int
	screen.Do(func() { before = disabled.axCaret() })
	screen.KeyPress(KeyRight, mod.None)
	screen.KeyPress(KeyEnd, mod.None)
	screen.Do(func() { after = disabled.axCaret() })
	c.Equal(before, after, "the keys that move a document's reading caret do nothing in a disabled one")
	screen.Click(screen.PanelCenter(enabled))
	screen.Click(screen.PanelCenter(disabled))
	c.True(disabledFocus(screen, wnd).Is(enabled), "and a click does not move the focus onto it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestNumericFieldBringsIntoRangeOnlyWhatWasTyped verifies that whether a numeric field brings its text into range as
// the focus leaves is decided by what happened while it held the focus, not by what is being asked for at that moment:
// a disabled field that was only read is left showing what the application put there even though the switch was
// turned off, or the assistive technology went away, before the focus left, and a field that was typed into and then
// disabled is brought into range whether the switch is on or off.
func TestNumericFieldBringsIntoRangeOnlyWhatWasTyped(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	modified := 0
	var editable *Field
	var reading, typing *NumericField[int]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			reading = NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			reading.BlinkRate = editable.BlinkRate
			reading.Accessibility.Name = "Quantity"
			reading.SetText("12")
			reading.SetEnabled(false)
			reading.ModifiedCallback = func(_, _ *FieldState) { modified++ }
			typing = NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			typing.BlinkRate = editable.BlinkRate
			typing.Accessibility.Name = "Copies"
			wnd = axNewTestWindow(t, "numeric", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(editable, reading, typing))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	screen.Do(func() { wnd.SetFocus(reading) })
	c.True(disabledFocus(screen, wnd).Is(reading))
	SetFocusForReading(false)
	screen.Sync()
	c.True(disabledFocus(screen, wnd).Is(reading), "turning the switch off leaves the focus where it is")
	screen.KeyPress(KeyTab, mod.None)
	c.False(disabledFocus(screen, wnd).Is(reading))
	c.Equal("12", disabledFieldText(screen, reading.Field), "a field that was only read is left as it was")
	c.Equal(0, modified, "and reports no modification")

	SetFocusForReading(true)
	screen.Sync()
	screen.Do(func() {
		wnd.SetFocus(reading)
		SetAccessibilityEnabled(false)
	})
	c.True(disabledFocus(screen, wnd).Is(reading))
	screen.KeyPress(KeyTab, mod.None)
	c.False(disabledFocus(screen, wnd).Is(reading))
	c.Equal("12", disabledFieldText(screen, reading.Field), "as it is when the assistive technology has gone away")
	c.Equal(0, modified)
	screen.Do(func() { SetAccessibilityEnabled(true) })
	screen.EnableAccessibility()

	for _, on := range []bool{true, false} {
		SetFocusForReading(on)
		screen.Sync()
		screen.Do(func() {
			typing.SetEnabled(true)
			typing.SetText("5")
			wnd.SetFocus(typing)
			typing.SelectAll()
		})
		screen.Type("12")
		c.Equal("12", disabledFieldText(screen, typing.Field))
		screen.Do(func() { typing.SetEnabled(false) })
		c.True(disabledFocus(screen, wnd).Is(typing), "disabling a field leaves the focus on it")
		screen.Do(func() { wnd.SetFocus(editable) })
		c.Equal("10", disabledFieldText(screen, typing.Field),
			"what was typed is brought into range though the field was disabled since (switch on: %v)", on)
	}

	// A field that was disabled when the focus arrived and enabled while it held it may have been typed into.
	SetFocusForReading(true)
	screen.Sync()
	screen.Do(func() {
		typing.SetEnabled(false)
		typing.SetText("12")
		wnd.SetFocus(typing)
		typing.SetEnabled(true)
		wnd.SetFocus(editable)
	})
	c.Equal("10", disabledFieldText(screen, typing.Field))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledListRowRefusesAStaleFocusRequest verifies that a request for the focus aimed at a row is refused once the
// list has been disabled, even though the row was published, while the list was enabled, as offering it and the window
// has not been described since. Giving the list the focus instead would report the request as carried out for the row
// while the focus went somewhere else.
func TestDisabledListRowRefusesAStaleFocusRequest(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var editable *Field
	var list *List[string]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			list = NewList[string]()
			list.Append("Row one", "Row two")
			list.Accessibility.Name = "Items"
			wnd = axNewTestWindow(t, "stale row", geom.NewRect(10, 10, 500, 300), staticTextColumn(editable, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	screen.Do(func() { wnd.SetFocus(editable) })

	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree == nil {
		return
	}
	var row *accessibility.Node
	tree.Walk(func(n *accessibility.Node) bool {
		if row == nil && n.Role == role.ListItem {
			row = n
		}
		return true
	})
	c.NotNil(row, "the rows of the list should have been described")
	if row == nil {
		return
	}
	c.False(row.Disabled)
	c.True(row.Actions.Has(accessibility.Focus), "a row of a list that can be used offers the focus")

	// Disabled and asked within the one task, so that the window cannot have been described in between.
	var carriedOut, listTakesFocus bool
	var focus *Panel
	screen.Do(func() {
		list.SetEnabled(false)
		listTakesFocus = list.Focusable()
		carriedOut = wnd.performAccessibilityAction(accessibility.ActionRequest{
			Node:   row.ID,
			Action: accessibility.Focus,
		})
		focus = wnd.CurrentFocus()
	})
	c.True(listTakesFocus, "the disabled list itself can take the focus for reading")
	c.False(carriedOut, "but a request for the focus aimed at one of its rows is refused")
	c.True(focus.Is(editable), "and the focus stays where it was")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledCaptionGivesUpTheFocus verifies that a disabled label that holds the focus and then becomes the caption
// of a control is published without the focus, as every disabled node that cannot take the focus is, with the focus
// reported on something that can be shown in its place.
func TestDisabledCaptionGivesUpTheFocus(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var editable *Field
	var label *Label
	var column *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			label = staticTextLabel("Total:")
			label.SetEnabled(false)
			column = staticTextColumn(editable, label)
			wnd = axNewTestWindow(t, "disabled caption", geom.NewRect(10, 10, 500, 300), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	c.NotNil(screen.AccessibilityTree(wnd))

	screen.Do(func() { wnd.SetFocus(label) })
	c.True(disabledFocus(screen, wnd).Is(label), "a disabled standalone label takes the focus")
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if node := disabledNode(c, screen, label, "the disabled label"); node != nil && tree != nil {
		c.True(node.Focused)
		c.Equal(node.ID, tree.Focus)
	}

	screen.Do(func() {
		captioned := staticTextField()
		captioned.SetText("42")
		column.AddChild(captioned)
		column.MarkForLayoutAndRedraw()
	})
	tree = screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	c.True(disabledFocus(screen, wnd).Is(label), "the window leaves the focus where it is")
	if node := disabledNode(c, screen, label, "the disabled caption"); node != nil && tree != nil {
		c.True(node.Disabled)
		c.False(node.Focusable, "a caption cannot take the focus")
		c.False(node.Actions.Has(accessibility.Focus))
		c.False(node.Focused, "and a disabled node that cannot take the focus is published without it")
		c.NotEqual(node.ID, tree.Focus)
		c.NotEqual(accessibility.NodeID(0), tree.Focus, "the focus is reported on something that can be shown")
		focused := 0
		tree.Walk(func(n *accessibility.Node) bool {
			if n.Focused && n.ID != tree.Root {
				focused++
				c.Equal(tree.Focus, n.ID)
			}
			return true
		})
		c.Equal(1, focused, "exactly one node reports the focus")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledContainerHandsTheFocusOn verifies that a disabled panel that is focusable and holds a tab stop hands the
// focus an application gives it to that tab stop, as it does while the switch is off, and is still a tab stop of its
// own that Tab lands on and leaves and that a screen reader can be taken to.
func TestDisabledContainerHandsTheFocusOn(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var editable, inner *Field
	var container *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			container = NewPanel()
			container.SetLayout(&FlexLayout{Columns: 1})
			container.Accessibility.Name = "Details"
			container.SetFocusable(true)
			inner = staticTextNamedField("Inner")
			container.AddChild(inner)
			container.SetEnabled(false)
			wnd = axNewTestWindow(t, "container", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(editable, container))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()

	for _, on := range []bool{false, true} {
		SetFocusForReading(on)
		screen.Sync()
		screen.Do(func() {
			wnd.SetFocus(editable)
			wnd.SetFocus(container)
		})
		c.True(disabledFocus(screen, wnd).Is(inner),
			"the focus given to the container goes to the field inside it (switch on: %v)", on)
	}

	c.True(staticTextFocusable(screen, container))
	screen.Do(func() { wnd.SetFocus(editable) })
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(container), "Tab stops at the disabled container itself")
	screen.KeyPress(KeyTab, mod.None)
	c.True(disabledFocus(screen, wnd).Is(inner), "and goes on to the field inside it")
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(disabledFocus(screen, wnd).Is(container), "Shift-Tab goes back onto the container")
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(disabledFocus(screen, wnd).Is(editable), "and past it")

	c.NotNil(screen.AccessibilityTree(wnd))
	if node := disabledNode(c, screen, container, "the disabled container"); node != nil {
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.Focus,
		}), "a screen reader asking for the focus on the container gets it")
		c.True(disabledFocus(screen, wnd).Is(container), "on the container itself")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledFieldIsNotScrolledByTheFocus verifies that a disabled field whose text does not fit is left showing the
// start of it when it takes the focus, since nothing would scroll it back once the focus had left, while a field that
// can be typed into is scrolled to its caret as it always was.
func TestDisabledFieldIsNotScrolledByTheFocus(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	const long = "A sentence that is a good deal longer than the field it has been put into can show"
	var editable, field, typed *Field
	var wnd *Window
	narrow := func(f *Field) *Field {
		f.SetText(long)
		f.NoSelectAllOnFocus = true
		f.SetLayoutData(&FlexLayoutData{SizeHint: geom.NewSize(80, 0)})
		return f
	}
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			editable = staticTextNamedField("Search")
			field = narrow(disabledField("Note", ""))
			typed = narrow(staticTextNamedField("Comment"))
			wnd = axNewTestWindow(t, "scroll", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(editable, field, typed))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	offsets := func(f *Field) (before, during, after geom.Point, width float32) {
		screen.Do(func() {
			wnd.SetFocus(editable)
			f.SetScrollOffset(geom.Point{})
			before = f.ScrollOffset()
			width = f.ContentRect(false).Width
			wnd.SetFocus(f)
			during = f.ScrollOffset()
			wnd.SetFocus(editable)
			after = f.ScrollOffset()
		})
		return before, during, after, width
	}
	before, during, after, width := offsets(field)
	c.True(width < 200, "the field should be too narrow for its text, but is %v wide", width)
	c.Equal(geom.Point{}, before)
	c.Equal(geom.Point{}, during, "taking the focus does not scroll the text of a disabled field")
	c.Equal(geom.Point{}, after)
	_, during, _, _ = offsets(typed)
	c.True(during.X < 0, "while a field that can be typed into is scrolled to its caret, not left at %v", during)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestGradientEditorUpdatesADisabledFieldThatHoldsTheFocus verifies that a field of the gradient editor that holds the
// focus while disabled, which is one being read rather than typed into, is brought up to date with the gradient.
func TestGradientEditorUpdatesADisabledFieldThatHoldsTheFocus(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var editor *GradientEditor
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			editor = NewGradientEditor(&Gradient{
				Kind:  gradienttype.Linear,
				EndPt: geom.NewPoint(1, 0),
				Stops: NewEvenlySpacedGradientStopsForColors(Black, White),
			})
			for _, f := range []*Field{
				editor.posField, editor.startXField, editor.startYField, editor.endXField, editor.endYField,
				editor.startRadiusField, editor.endRadiusField, editor.startAngleField, editor.endAngleField,
			} {
				if f != nil {
					f.BlinkRate = time.Hour
				}
			}
			wnd = axNewTestWindow(t, "gradient", geom.NewRect(10, 10, 700, 700), editor)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()

	var enabled bool
	screen.Do(func() {
		enabled = editor.startRadiusField.Enabled()
		wnd.SetFocus(editor.startRadiusField)
	})
	c.False(enabled, "a linear gradient has no radius, so the field is disabled")
	c.True(disabledFocus(screen, wnd).Is(editor.startRadiusField), "and takes the focus so that it can be read")
	c.Equal("0", disabledFieldText(screen, editor.startRadiusField))
	screen.Do(func() {
		g := editor.Gradient()
		g.Radius.Start = 42
		editor.SetGradient(g)
	})
	c.Equal("42", disabledFieldText(screen, editor.startRadiusField),
		"a disabled field that holds the focus is not being edited, so it is brought up to date")

	// A field being typed into is still left alone.
	screen.Do(func() {
		wnd.SetFocus(editor.startXField)
		editor.startXField.SelectAll()
	})
	screen.Type("7")
	screen.Do(func() {
		g := editor.Gradient()
		g.StartPt.X = 0.5
		editor.SetGradient(g)
	})
	c.Equal("7", disabledFieldText(screen, editor.startXField))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestEditorHintsAreNotTabStops verifies that the dimmed text the color and gradient editors put after a field to say
// what may be typed into it is said as that field's description, and is neither an element of its own nor a tab stop.
func TestEditorHintsAreNotTabStops(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var colors *ColorEditor
	var gradients *GradientEditor
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 900, Height: 1200},
		StartupFinishedCallback(func() {
			colors = NewColorEditor(Red)
			gradients = NewGradientEditor(&Gradient{
				Kind:   gradienttype.Radial,
				Radius: StartEnd{Start: 1, End: 32},
				Stops:  NewEvenlySpacedGradientStopsForColors(Black, White),
			})
			wnd = axNewTestWindow(t, "editors", geom.NewRect(10, 10, 800, 1100), staticTextColumn(colors, gradients))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetFocusForReading(true)
	screen.Sync()
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree == nil {
		return
	}

	hints := map[string]bool{"0-255 or 0-100%": true, "0-359": true, "0-100%": true, "px": true}
	found := 0
	var tabStops []string
	screen.Do(func() {
		var walk func(p *Panel)
		walk = func(p *Panel) {
			if label, ok := p.Self.(*Label); ok && hints[label.String()] {
				found++
				if p.Focusable() {
					tabStops = append(tabStops, label.String())
				}
			}
			for _, child := range p.Children() {
				walk(child)
			}
		}
		walk(wnd.Content())
	})
	c.Equal(8, found, "the editors should hold seven hints and a unit")
	c.Equal(0, len(tabStops), "none of which may be a tab stop, but these are: %v", tabStops)
	described := 0
	tree.Walk(func(n *accessibility.Node) bool {
		if hints[n.Name] {
			described++
		}
		return true
	})
	c.Equal(0, described, "nor described as an element of its own")
	if node := disabledNode(c, screen, colors.hueField, "the hue field"); node != nil {
		c.Equal("0-359", node.Description, "each is said with the field it follows")
		c.Equal("Hue", node.Name)
	}
	if node := disabledNode(c, screen, gradients.startRadiusField, "the start radius field"); node != nil {
		c.Equal("Pixels", node.Description)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
