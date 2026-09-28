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
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests cover the keyboard focus standalone static text takes for a screen reader's sake; see
// SetStaticTextFocusableForAccessibility, AccessibilityInfo.FocusForReading and Panel.axStaticTextTakesFocus. They use
// plain labels rather than headings, since a headless session answers as Linux does and lets a heading take the focus
// through an arm of its own; TestStaticTextFocusByRole turns that arm off. A session owns most of the package's mutable
// globals while it runs, so none of these may call t.Parallel.

func staticTextLabel(title string) *Label {
	l := NewLabel()
	l.SetTitle(title)
	return l
}

// staticTextField returns a field whose caret never blinks, since a blink timer outlives the session that armed it and
// would enqueue a stray task into whichever test runs next.
func staticTextField() *Field {
	f := NewField()
	f.BlinkRate = time.Hour
	return f
}

// staticTextNamedField returns a field that names itself, so the label before it stays standalone text rather than
// becoming its caption.
func staticTextNamedField(name string) *Field {
	f := staticTextField()
	f.Accessibility.Name = name
	return f
}

func staticTextColumn(panels ...Paneler) *Panel {
	column := NewPanel()
	column.SetLayout(&FlexLayout{Columns: 1})
	for _, p := range panels {
		column.AddChild(p)
	}
	return column
}

func restoreStaticTextFocus(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { staticTextTakesFocus.Store(false) })
}

func staticTextFocusable(screen *HeadlessScreen, p Paneler) bool {
	var focusable bool
	screen.Do(func() { focusable = p.AsPanel().Focusable() })
	return focusable
}

func staticTextSnapshots(screen *HeadlessScreen) uint64 {
	var count uint64
	screen.Do(func() { count = axSnapshotCount })
	return count
}

// staticTextCheckNode checks, without describing the window again, that the last published node for p has Focusable
// and the focus action both equal to want.
func staticTextCheckNode(c check.Checker, screen *HeadlessScreen, p Paneler, want bool, what string) {
	node := screen.AccessibilityNodeFor(p)
	c.NotNil(node, "%s should have been described", what)
	if node == nil {
		return
	}
	c.Equal(want, node.Focusable, "%s: Focusable", what)
	c.Equal(want, node.Actions.Has(accessibility.Focus), "%s: offers the focus action", what)
}

// TestStaticTextFocusIsOffByDefault verifies that a standalone label is neither a tab stop nor described as one while
// an assistive technology is being served but the switch is off.
func TestStaticTextFocusIsOffByDefault(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var text *Label
	var field *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			text = staticTextLabel("Changes are saved as you type.")
			field = staticTextNamedField("Search")
			wnd = axNewTestWindow(t, "off by default", geom.NewRect(10, 10, 500, 300), staticTextColumn(text, field))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	c.False(StaticTextFocusableForAccessibility(), "the switch is off until the application turns it on")
	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())
	c.False(staticTextFocusable(screen, text), "a standalone label is not a tab stop by default")
	c.NotNil(screen.AccessibilityTree(wnd))
	staticTextCheckNode(c, screen, text, false, "the standalone label")

	screen.Do(func() { wnd.SetFocus(field) })
	screen.KeyPress(KeyTab, mod.None)
	var focus *Panel
	screen.Do(func() { focus = wnd.CurrentFocus() })
	c.True(focus.Is(field), "Tab wraps around onto the field, the only tab stop there is")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextFocusNeedsAnAssistiveTechnology verifies that the switch changes nothing while no assistive technology
// is being served: no label becomes a tab stop, the window still opens in its field, and nothing is described.
func TestStaticTextFocusNeedsAnAssistiveTechnology(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var text *Label
	var field *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			text = staticTextLabel("Changes are saved as you type.")
			field = staticTextNamedField("Search")
			wnd = axNewTestWindow(t, "no assistive technology", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(text, field))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	SetStaticTextFocusableForAccessibility(true)
	screen.Sync()
	c.True(StaticTextFocusableForAccessibility(), "the switch reports what it was set to")
	c.False(IsAccessibilityActive(), "turning the switch on must not start accessibility support")
	c.False(staticTextFocusable(screen, text), "with nothing listening, a label is not a tab stop")

	var seeded, next *Panel
	screen.Do(func() { wnd.SetFocus(nil) })
	screen.KeyPress(KeyTab, mod.None)
	screen.Do(func() { seeded = wnd.CurrentFocus() })
	screen.KeyPress(KeyTab, mod.None)
	screen.Do(func() { next = wnd.CurrentFocus() })
	c.True(seeded.Is(field), "the window opens in its field")
	c.True(next.Is(field), "and Tab never stops at the label")
	c.Equal(uint64(0), staticTextSnapshots(screen), "nothing may be described while nothing is listening")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextTakesFocusExceptCaptions verifies the switch while an assistive technology is being served: standalone
// labels become tab stops, are described as such and are given the focus when a screen reader asks, while a caption,
// whether placed before a field or pointed at through LabeledBy, is neither. The description published right after the
// switch is turned on is checked without describing the window again, since the caption before its field is described
// before the field makes it one; see axSnapshot.settleCaptionFocus.
func TestStaticTextTakesFocusExceptCaptions(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var first, caption, trailingCaption, last *Label
	var named, pointedAt *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			first = staticTextLabel("Both names are optional.")
			caption = staticTextLabel("Name:")
			named = staticTextField()
			pointedAt = staticTextField()
			// Named by a label that comes after it, so the builder records the caption before describing it.
			trailingCaption = staticTextLabel("Nickname")
			pointedAt.Accessibility.LabeledBy = trailingCaption
			last = staticTextLabel("Changes are saved when the window closes.")
			wnd = axNewTestWindow(t, "captions", geom.NewRect(10, 10, 500, 400),
				staticTextColumn(first, caption, named, pointedAt, trailingCaption, last))
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

	// Described once before the switch, so the captions were recorded in an earlier description than the one the
	// switch causes.
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	before := staticTextSnapshots(screen)
	SetStaticTextFocusableForAccessibility(true)
	screen.Sync()
	c.True(staticTextSnapshots(screen) > before, "turning the switch on describes the window again")

	staticTextCheckNode(c, screen, first, true, "the standalone label at the top")
	staticTextCheckNode(c, screen, last, true, "the standalone label at the bottom")
	staticTextCheckNode(c, screen, caption, false, "the caption described before its field")
	staticTextCheckNode(c, screen, trailingCaption, false, "the caption described after its field")
	c.True(staticTextFocusable(screen, first))
	c.True(staticTextFocusable(screen, last))
	c.False(staticTextFocusable(screen, caption), "a caption is spoken with its field and is not a tab stop")
	c.False(staticTextFocusable(screen, trailingCaption), "whichever way it names the field")

	screen.Do(func() { wnd.SetFocus(pointedAt) })
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(last), "Tab passes over the caption after the field and lands on the standalone label")
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(first), "and wraps around onto the label at the top of the window")
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(named), "then passes over the caption that names the first field")
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(focus().Is(first), "and Shift-Tab passes over it again on the way back")

	captionNode := screen.AccessibilityNodeFor(caption)
	firstNode := screen.AccessibilityNodeFor(first)
	c.NotNil(captionNode)
	c.NotNil(firstNode)
	if captionNode == nil || firstNode == nil {
		return
	}
	screen.Do(func() { wnd.SetFocus(named) })
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   captionNode.ID,
		Action: accessibility.Focus,
	}), "a caption does not offer the focus, so a request for it is refused")
	c.True(focus().Is(named), "and the focus stays where it was")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   firstNode.ID,
		Action: accessibility.Focus,
	}), "a screen reader asking for the focus on standalone text gets it")
	c.True(focus().Is(first))
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree != nil {
		c.Equal(firstNode.ID, tree.Focus, "and the description reports the focus there")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextFocusExclusions verifies that, with the switch on, text outside the window's content stays out of the
// tab order: a label inside a document or a control, one hidden from assistive technologies, one holding only a
// drawable, one installed as a cell of a Table, List or TableHeader, and one in the tooltip. Labels inside a group and
// a scroll panel are the window's text, and are tab stops.
func TestStaticTextFocusExclusions(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var inReader, inButton, hidden, icon, inGroup, inScroller *Label
	var md *Markdown
	var table *Table[*axHeaderRow]
	var header *TableHeader[*axHeaderRow]
	var list *List[string]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			md = NewMarkdown(false)
			md.SetContent("# Title\n\nSome body text.\n", 400)
			// A stand-in for a document holding a plain label, which a real Markdown never does, so that the ancestor
			// rule is the only thing keeping the label out.
			reader := NewPanel()
			reader.SetLayout(&FlexLayout{Columns: 1})
			reader.axFocusable = true
			inReader = staticTextLabel("Inside a document")
			reader.AddChild(inReader)

			button := NewPanel()
			button.SetLayout(&FlexLayout{Columns: 1})
			button.Accessibility.Role = role.Button
			inButton = staticTextLabel("Go")
			button.AddChild(inButton)

			hidden = staticTextLabel("Hidden from screen readers")
			hidden.Accessibility.Role = role.None

			icon = NewLabel()
			icon.Drawable = &axLabelTestDrawable{size: geom.NewSize(16, 16)}
			icon.Tooltip = NewTooltipWithText("Printer")

			group := NewPanel()
			group.SetLayout(&FlexLayout{Columns: 1})
			group.Accessibility.Role = role.Group
			inGroup = staticTextLabel("In a group")
			group.AddChild(inGroup)

			scroller := NewScrollPanel()
			inScroller = staticTextLabel("In a scroll panel")
			scroller.SetContent(inScroller, behavior.Fill, behavior.Fill)

			model := &SimpleTableModel[*axHeaderRow]{}
			model.SetRootRows([]*axHeaderRow{{id: "a"}})
			table = NewTable[*axHeaderRow](model)
			table.Columns = []ColumnInfo{{ID: 0, Current: 100}}
			header = NewTableHeader(table, NewTableColumnHeader[*axHeaderRow]("Column", "", nil))
			table.SyncToModel()
			list = NewList[string]()
			list.Append("row")

			wnd = axNewTestWindow(t, "exclusions", geom.NewRect(10, 10, 700, 700),
				staticTextColumn(md, reader, button, hidden, icon, group, scroller, header, table, list))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	SetStaticTextFocusableForAccessibility(true)
	screen.Sync()

	c.False(staticTextFocusable(screen, inReader), "a document reads its own content")
	c.False(staticTextFocusable(screen, inButton), "text inside a control is that control's")
	c.False(staticTextFocusable(screen, hidden), "text hidden from assistive technologies is not somewhere to land")
	c.False(staticTextFocusable(screen, icon), "a label holding only a drawable is an image rather than text")
	c.True(staticTextFocusable(screen, inGroup), "a grouping panel leaves its text as the window's")
	c.True(staticTextFocusable(screen, inScroller), "and so does a scroll panel")

	var inDocument []*Panel
	var inTable, inHeader, inList, inTooltip, documentFocusable bool
	screen.Do(func() {
		documentFocusable = md.AsPanel().Focusable()
		var walk func(p *Panel)
		walk = func(p *Panel) {
			for _, child := range p.Children() {
				if child.Focusable() {
					inDocument = append(inDocument, child)
				}
				walk(child)
			}
		}
		walk(md.AsPanel())

		// A cell is attached only while drawn or described, so each is asked while installed.
		rect := geom.NewRect(0, 0, 100, 20)
		cellFocusable := func(install, uninstall func(p *Panel)) bool {
			cell := staticTextLabel("In a cell").AsPanel()
			install(cell)
			defer uninstall(cell)
			return cell.Focusable()
		}
		inTable = cellFocusable(func(p *Panel) { table.installCell(p, rect) }, func(p *Panel) { p.parent = nil })
		inHeader = cellFocusable(func(p *Panel) { header.installCell(p, rect) }, header.uninstallCell)
		inList = cellFocusable(func(p *Panel) { list.installCell(p, rect) }, list.uninstallCell)

		tip := staticTextColumn()
		tipText := staticTextLabel("Tip text")
		tip.AddChild(tipText)
		wnd.root.setTooltip(tip)
		inTooltip = tipText.AsPanel().Focusable()
		wnd.root.setTooltip(nil)
	})
	c.True(documentFocusable, "the document itself takes the focus for a screen reader's sake")
	c.Equal(0, len(inDocument), "but nothing inside it does: %v", inDocument)
	c.False(inTable, "a table cell is reached through the table's cell cursor")
	c.False(inHeader, "a column header is part of the table's description")
	c.False(inList, "a list row is reached through the list's current row")
	c.False(inTooltip, "a tooltip comes and goes and is never in the tab order")

	c.NotNil(screen.AccessibilityTree(wnd))
	staticTextCheckNode(c, screen, inButton, false, "the label inside the button")
	staticTextCheckNode(c, screen, icon, false, "the drawable-only label")
	staticTextCheckNode(c, screen, inGroup, true, "the label inside the group")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextSeedsTheFocusLast verifies that static text is the last choice when a window with nothing focused
// chooses where to put the person: a window holding a field opens in the field from either end, while a window of
// nothing but text opens in its first label.
func TestStaticTextSeedsTheFocusLast(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var lead, trailing, firstText, lastText *Label
	var field *Field
	var wnd, textOnly *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			lead = staticTextLabel("Search the catalog.")
			field = staticTextNamedField("Search")
			trailing = staticTextLabel("Results appear below.")
			wnd = axNewTestWindow(t, "text and a field", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(lead, field, trailing))

			firstText = staticTextLabel("The operation finished.")
			lastText = staticTextLabel("Nothing was changed.")
			textOnly = axNewTestWindow(t, "text only", geom.NewRect(60, 60, 300, 200),
				staticTextColumn(firstText, lastText))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	c.NotNil(textOnly)
	screen.Sync()
	screen.EnableAccessibility()
	SetStaticTextFocusableForAccessibility(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, lead), "the label before the field is standalone text, and a tab stop")
	c.True(staticTextFocusable(screen, trailing))

	var seeded, seededBackward, seededText, seededTextBackward *Panel
	screen.Do(func() { wnd.SetFocus(nil) })
	screen.KeyPress(KeyTab, mod.None)
	screen.Do(func() {
		seeded = wnd.CurrentFocus()
		wnd.SetFocus(nil)
	})
	screen.KeyPress(KeyTab, mod.Shift)
	screen.Do(func() {
		seededBackward = wnd.CurrentFocus()
		textOnly.SetFocus(nil)
		textOnly.FocusNext()
		seededText = textOnly.CurrentFocus()
		textOnly.SetFocus(nil)
		textOnly.FocusPrevious()
		seededTextBackward = textOnly.CurrentFocus()
	})
	c.True(seeded.Is(field), "the window opens in its field rather than on the label above it")
	c.True(seededBackward.Is(field), "and Shift-Tab passes over the label below it just the same")
	c.True(seededText.Is(firstText), "a window of nothing but text opens in its first label")
	c.True(seededTextBackward.Is(lastText), "or its last, moving the other way")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextFocusRuntimeToggle verifies that the switch may be flipped while the application runs, from a goroutine
// other than the UI thread: turning it off leaves the focus on the label that holds it, stops the label being a tab
// stop and describes the window again, and turning it back on restores both. The label sits between two fields so that
// the next Tab landing on the window's first tab stop, and Shift-Tab on its last, is told apart from the stops beside
// it.
func TestStaticTextFocusRuntimeToggle(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var text *Label
	var before, after *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			before = staticTextNamedField("Search")
			text = staticTextLabel("Changes are saved as you type.")
			after = staticTextNamedField("Replace")
			wnd = axNewTestWindow(t, "toggle", geom.NewRect(10, 10, 500, 300), staticTextColumn(before, text, after))
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

	screen.EnableAccessibility()
	SetStaticTextFocusableForAccessibility(true)
	screen.Sync()
	screen.Do(func() { wnd.SetFocus(text) })
	c.True(focus().Is(text), "the label takes the focus while the switch is on")
	c.NotNil(screen.AccessibilityTree(wnd))
	staticTextCheckNode(c, screen, text, true, "the label with the switch on")

	// Called off the UI thread, as a worker goroutine would.
	snapshots := staticTextSnapshots(screen)
	SetStaticTextFocusableForAccessibility(false)
	screen.Sync()
	c.False(StaticTextFocusableForAccessibility())
	c.True(focus().Is(text), "turning the switch off leaves the focus where it is")
	c.False(staticTextFocusable(screen, text), "but the label is no longer a tab stop")
	c.True(staticTextSnapshots(screen) > snapshots, "the window is described again to say so")
	staticTextCheckNode(c, screen, text, false, "the label with the switch off")
	// The description keeps reporting the focus on the label, which still holds it, rather than falling back to an
	// ancestor as it does for a disabled focus panel; see axSnapshot.settleCaptionFocus.
	node := screen.AccessibilityNodeFor(text)
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(node)
	c.NotNil(tree)
	if node != nil && tree != nil {
		c.True(node.Focused, "the label still says it holds the focus")
		c.Equal(node.ID, tree.Focus, "and the window reports the focus there")
	}
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(before), "so Tab lands on the window's first tab stop, not the field after the label")

	snapshots = staticTextSnapshots(screen)
	SetStaticTextFocusableForAccessibility(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, text), "turning it back on makes the label a tab stop again")
	c.True(staticTextSnapshots(screen) > snapshots, "and describes the window again")
	staticTextCheckNode(c, screen, text, true, "the label with the switch back on")

	screen.Do(func() { wnd.SetFocus(text) })
	SetStaticTextFocusableForAccessibility(false)
	screen.Sync()
	c.True(focus().Is(text))
	screen.KeyPress(KeyTab, mod.Shift)
	c.True(focus().Is(after), "and Shift-Tab lands on the window's last tab stop, not the field before the label")
	SetStaticTextFocusableForAccessibility(true)
	screen.Sync()

	snapshots = staticTextSnapshots(screen)
	SetStaticTextFocusableForAccessibility(true)
	screen.Sync()
	c.Equal(snapshots, staticTextSnapshots(screen), "setting the value it already has describes nothing")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextFocusByRole verifies the arm of Panel.axIsReadableText the Windows and macOS builds rely on, where a
// heading never takes the focus for its own sake: a panel marked up as role.Label takes the focus when it has a name
// and not otherwise, and a heading takes it as static text once the heading arm, which a headless session answers as
// Linux does, is turned off.
func TestStaticTextFocusByRole(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var named, nameless *Panel
	var heading *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			named = NewPanel()
			named.Accessibility.Role = role.Label
			named.Accessibility.Name = "Built from a plain panel"
			nameless = NewPanel()
			nameless.Accessibility.Role = role.Label
			heading = staticTextLabel("Section")
			heading.Accessibility.Role = role.Heading
			heading.Accessibility.Level = 1
			wnd = axNewTestWindow(t, "by role", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(named, nameless, heading, staticTextNamedField("Search")))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	var saved bool
	screen.Do(func() {
		saved = axHeadingsTakeFocus
		axHeadingsTakeFocus = false
	})
	defer screen.Do(func() { axHeadingsTakeFocus = saved })

	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	c.False(staticTextFocusable(screen, heading), "where headings do not take the focus, one is not a tab stop by default")
	c.False(staticTextFocusable(screen, named), "nor is a panel marked up as a label")

	SetStaticTextFocusableForAccessibility(true)
	screen.Sync()
	c.True(staticTextFocusable(screen, named), "a panel marked up as a label takes the focus on the strength of its name")
	c.False(staticTextFocusable(screen, nameless), "one with nothing to announce is no place to land")
	c.True(staticTextFocusable(screen, heading), "a heading takes the focus as static text where the heading arm is off")
	staticTextCheckNode(c, screen, named, true, "the named label panel")
	staticTextCheckNode(c, screen, heading, true, "the heading")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFocusForReadingOptsInOnePanel verifies the per-panel opt-in: with the switch off, a label that sets
// FocusForReading takes the focus while an assistive technology is being served, the label beside it does not, and a
// caption that sets it is still not a tab stop.
func TestFocusForReadingOptsInOnePanel(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	var reading, plain, caption *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			reading = staticTextLabel("Read this before continuing.")
			reading.Accessibility.FocusForReading = true
			plain = staticTextLabel("Some other text.")
			caption = staticTextLabel("Name:")
			caption.Accessibility.FocusForReading = true
			wnd = axNewTestWindow(t, "opt in", geom.NewRect(10, 10, 500, 300),
				staticTextColumn(reading, plain, caption, staticTextField()))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	c.False(staticTextFocusable(screen, reading), "with nothing listening, even an opted-in label is not a tab stop")
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))
	c.False(StaticTextFocusableForAccessibility(), "the switch itself is still off")
	c.True(staticTextFocusable(screen, reading), "the label that asked takes the focus")
	c.False(staticTextFocusable(screen, plain), "the one beside it, which did not ask, does not")
	c.False(staticTextFocusable(screen, caption), "and a caption that asked is still spoken with its field instead")
	staticTextCheckNode(c, screen, reading, true, "the opted-in label")
	staticTextCheckNode(c, screen, plain, false, "the plain label")
	staticTextCheckNode(c, screen, caption, false, "the opted-in caption")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestStaticTextFocusDoesNotOutliveTheSession verifies that a headless session puts the switch back when it ends.
func TestStaticTextFocusDoesNotOutliveTheSession(t *testing.T) {
	c := check.New(t)
	restoreStaticTextFocus(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200})
	SetStaticTextFocusableForAccessibility(true)
	c.True(StaticTextFocusableForAccessibility())
	c.True(screen.Quit(), "the session should end")
	next := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200})
	c.True(next.Running())
	c.False(StaticTextFocusableForAccessibility(), "the next session starts with the switch off")
}
