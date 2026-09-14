// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// The node ids the synthetic tree below uses.
const (
	axTestRoot accessibility.NodeID = iota + 1
	axTestButton
	axTestField
	axTestGroup
	axTestLabel
)

// axTestText is the content of the synthetic text field. It is deliberately mixed: a two-byte rune, an astral-plane
// rune that takes two UTF-16 code units, and a line feed, so that the rune-to-UTF-16 conversion cannot be right by
// accident.
const axTestText = "héllo 😀\nwörld"

// newAXTestTree returns a synthetic snapshot holding a button, a focused text field with measured lines, and an ignored
// group wrapping a label — which is what proves an assistive technology is shown the label as a direct child of the
// window rather than the group that happens to hold it.
func newAXTestTree() *accessibility.Tree {
	root := &accessibility.Node{
		ID:       axTestRoot,
		Children: []accessibility.NodeID{axTestButton, axTestField, axTestGroup},
		Name:     "Test Window",
		Role:     role.Window,
		Bounds:   geom.NewRect(0, 0, 320, 240),
		Focused:  true,
	}
	button := &accessibility.Node{
		ID:      axTestButton,
		Parent:  axTestRoot,
		Name:    "OK",
		Role:    role.Button,
		Bounds:  geom.NewRect(10, 20, 80, 24),
		Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
	}
	field := &accessibility.Node{
		ID:          axTestField,
		Parent:      axTestRoot,
		Name:        "Name",
		Description: "Your full name",
		Value:       axTestText,
		Placeholder: "required",
		Role:        role.TextField,
		Bounds:      geom.NewRect(10, 60, 200, 32),
		Focusable:   true,
		Focused:     true,
		Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetValue,
			accessibility.SetTextSelection),
		Text: &accessibility.TextInfo{
			Text:     axTestText,
			SelStart: 2,
			SelEnd:   7,
			Caret:    7,
			Lines: []accessibility.Line{
				{
					Advances: []float32{0, 10, 20, 30, 40, 50, 60, 80, 80},
					Start:    0,
					End:      8,
					Bounds:   geom.NewRect(0, 0, 80, 16),
				},
				{
					Advances: []float32{0, 10, 20, 30, 40, 50},
					Start:    8,
					End:      13,
					Bounds:   geom.NewRect(0, 16, 50, 16),
				},
			},
		},
	}
	group := &accessibility.Node{
		ID:       axTestGroup,
		Parent:   axTestRoot,
		Children: []accessibility.NodeID{axTestLabel},
		Role:     role.Group,
		Bounds:   geom.NewRect(10, 100, 200, 40),
		Ignored:  true,
	}
	label := &accessibility.Node{
		ID:     axTestLabel,
		Parent: axTestGroup,
		Name:   "Ready",
		Role:   role.Label,
		Bounds: geom.NewRect(12, 102, 60, 16),
	}
	return &accessibility.Tree{
		Nodes: map[accessibility.NodeID]*accessibility.Node{
			axTestRoot: root, axTestButton: button, axTestField: field, axTestGroup: group, axTestLabel: label,
		},
		Root:       axTestRoot,
		Focus:      axTestField,
		Generation: 1,
	}
}

// newAXTestAdapter builds a window, a content view and an adapter with the synthetic tree published into it. The
// returned cleanup mirrors nativeDestroy: the adapter lets go of its elements before the view is released.
func newAXTestAdapter(t *testing.T) (view View, adapter *AXAdapter, tree *accessibility.Tree, cleanup func()) {
	t.Helper()
	tree = newAXTestTree()
	view, adapter, cleanup = newAXAdapterWithTree(t, tree)
	return view, adapter, tree, cleanup
}

// newAXAdapterWithTree is newAXTestAdapter for a snapshot the caller built, which is how the tests that need a
// particular shape — a table taller than its view port, a row whose parent is missing — get one.
func newAXAdapterWithTree(t *testing.T, tree *accessibility.Tree) (view View, adapter *AXAdapter, cleanup func()) {
	t.Helper()
	w, v, closeWindow := newTestWindowAndView(t)
	w.MakeKeyAndOrderFront()
	a := NewAXAdapter(v)
	if a == nil {
		t.Error("NewAXAdapter returned nil")
		return 0, nil, closeWindow
	}
	a.Publish(tree, accessibility.Diff(nil, tree))
	return v, a, func() {
		a.Shutdown()
		closeWindow()
	}
}

// axTestChildren returns the elements the content view reports as its children, failing the test if there is not the
// expected number of them.
func axTestChildren(t *testing.T, v View, want int) []objc.ID {
	t.Helper()
	children := IDsFromNSArray(objc.ID(v).Send(Sel("accessibilityChildren")))
	if len(children) != want {
		t.Fatalf("content view has %d children, want %d", len(children), want)
	}
	return children
}

// axTestScreenPoint returns the screen location of a window-local, top-left origin point, using the adapter's own
// conversion so the test is asserting the round trip rather than re-deriving it.
func axTestScreenPoint(a *AXAdapter, pt geom.Point) NSPoint {
	return a.screenRect(geom.Rect{Point: pt}).Origin
}

// TestAXAdapterHierarchy proves the shape an assistive technology is shown: the content view is an ignored group that
// answers for the window's root node, the ignored group in the middle of the tree is spliced out, and each element
// reports its role, subrole, label, help, placeholder and parent from the snapshot.
func TestAXAdapterHierarchy(t *testing.T) {
	runOnMain(func() {
		v, a, _, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		ov := objc.ID(v)
		WithPool(func() {
			if objc.Send[bool](ov, Sel("isAccessibilityElement")) {
				t.Error("the content view reports itself as an accessibility element, want ignored")
			}
			if got := GoStringFromNSString(ov.Send(Sel("accessibilityRole"))); got != "AXGroup" {
				t.Errorf("content view accessibilityRole = %q, want AXGroup", got)
			}
			children := IDsFromNSArray(ov.Send(Sel("accessibilityChildren")))
			if len(children) != 3 {
				t.Fatalf("content view has %d children, want 3 (the ignored group must be flattened)", len(children))
			}
			// Static text is the odd one out: its content is its name in the snapshot, because that is the accessible
			// name every other platform wants, but on this platform it is reported as the element's value and not as
			// its label, so that an assistive technology says it once rather than twice.
			for i, c := range []struct {
				label   string
				value   string
				role    string
				subrole string
			}{
				{label: "OK", role: "AXButton"},
				{label: "Name", value: axTestText, role: "AXTextField"},
				{value: "Ready", role: "AXStaticText"},
			} {
				if got := GoStringFromNSString(children[i].Send(Sel("accessibilityRole"))); got != c.role {
					t.Errorf("child %d role = %q, want %q", i, got, c.role)
				}
				if got := GoStringFromNSString(children[i].Send(Sel("accessibilitySubrole"))); got != c.subrole {
					t.Errorf("child %d subrole = %q, want %q", i, got, c.subrole)
				}
				if got := GoStringFromNSString(children[i].Send(Sel("accessibilityLabel"))); got != c.label {
					t.Errorf("child %d label = %q, want %q", i, got, c.label)
				}
				if got := GoStringFromNSString(children[i].Send(Sel("accessibilityValue"))); got != c.value {
					t.Errorf("child %d value = %q, want %q", i, got, c.value)
				}
				if !objc.Send[bool](children[i], Sel("isAccessibilityElement")) {
					t.Errorf("child %d is not an accessibility element", i)
				}
				if !objc.Send[bool](children[i], Sel("isAccessibilityEnabled")) {
					t.Errorf("child %d is not enabled", i)
				}
				// Every element's parent resolves to the content view: two are children of the root node, and the third
				// is a child of the ignored group, whose reportable ancestor is the root as well.
				if got := children[i].Send(Sel("accessibilityParent")); got != ov {
					t.Errorf("child %d parent = %#x, want the content view %#x", i, got, ov)
				}
				if got := children[i].Send(Sel("accessibilityWindow")); got != objc.ID(a.wnd) {
					t.Errorf("child %d window = %#x, want %#x", i, got, objc.ID(a.wnd))
				}
				if got := children[i].Send(Sel("accessibilityTopLevelUIElement")); got != objc.ID(a.wnd) {
					t.Errorf("child %d top level element = %#x, want %#x", i, got, objc.ID(a.wnd))
				}
			}
			field := children[1]
			if got := GoStringFromNSString(field.Send(Sel("accessibilityHelp"))); got != "Your full name" {
				t.Errorf("field help = %q, want %q", got, "Your full name")
			}
			if got := GoStringFromNSString(field.Send(Sel("accessibilityPlaceholderValue"))); got != "required" {
				t.Errorf("field placeholder = %q, want %q", got, "required")
			}
			if !objc.Send[bool](field, Sel("isAccessibilityFocused")) {
				t.Error("field is not focused")
			}
			if got := field.Send(Sel("accessibilityChildren")); NSArrayCount(got) != 0 {
				t.Errorf("field has %d children, want 0", NSArrayCount(got))
			}
			if got := ov.Send(Sel("accessibilityFocusedUIElement")); got != a.Element(axTestField) {
				t.Errorf("focused element = %#x, want the field %#x", got, a.Element(axTestField))
			}
		})
	})
}

// TestAXAdapterFrameAndHitTest proves the coordinate conversion in both directions: a node's window-local bounds become
// the screen rect NSAccessibility reports, and a screen point comes back as the deepest node containing it.
func TestAXAdapterFrameAndHitTest(t *testing.T) {
	runOnMain(func() {
		v, a, tree, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		ov := objc.ID(v)
		WithPool(func() {
			children := IDsFromNSArray(ov.Send(Sel("accessibilityChildren")))
			if len(children) != 3 {
				t.Fatalf("content view has %d children, want 3", len(children))
			}
			button := children[0]
			want := a.screenRect(tree.Node(axTestButton).Bounds)
			if got := objc.Send[NSRect](button, Sel("accessibilityFrame")); got != want {
				t.Errorf("button accessibilityFrame = %+v, want %+v", got, want)
			}
			if want.Size.Width != 80 || want.Size.Height != 24 {
				t.Errorf("converted button frame size = %v, want {80 24}", want.Size)
			}
			// The content view answers hit tests for the window, and the point is inside the button.
			inButton := axTestScreenPoint(a, geom.NewPoint(50, 30))
			if got := ov.Send(Sel("accessibilityHitTest:"), inButton); got != button {
				t.Errorf("hit test inside the button = %#x, want the button %#x", got, button)
			}
			// A point inside the ignored group but outside the label resolves to the group's reportable ancestor, which
			// is the root node, and so to the content view.
			inGroup := axTestScreenPoint(a, geom.NewPoint(180, 130))
			if got := ov.Send(Sel("accessibilityHitTest:"), inGroup); got != ov {
				t.Errorf("hit test inside the ignored group = %#x, want the content view %#x", got, ov)
			}
			// A point inside the label, which is nested inside the ignored group, resolves to the label itself.
			inLabel := axTestScreenPoint(a, geom.NewPoint(20, 110))
			if got := ov.Send(Sel("accessibilityHitTest:"), inLabel); got != children[2] {
				t.Errorf("hit test inside the label = %#x, want the label %#x", got, children[2])
			}
			// Asking an element itself is the same question, answered the same way.
			if got := button.Send(Sel("accessibilityHitTest:"), inButton); got != button {
				t.Errorf("element hit test = %#x, want the button %#x", got, button)
			}
		})
	})
}

// TestAXAdapterActions proves an assistive technology's requests reach the action callback, that the node's advertised
// action set is what decides whether the request is accepted, and that the value setters carry their payload.
func TestAXAdapterActions(t *testing.T) {
	defer func() { AccessibilityActionCallback = nil }()
	runOnMain(func() {
		v, a, _, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		var requests []accessibility.ActionRequest
		var windows []Window
		AccessibilityActionCallback = func(w Window, req accessibility.ActionRequest) {
			windows = append(windows, w)
			requests = append(requests, req)
		}
		ov := objc.ID(v)
		WithPool(func() {
			children := IDsFromNSArray(ov.Send(Sel("accessibilityChildren")))
			if len(children) != 3 {
				t.Fatalf("content view has %d children, want 3", len(children))
			}
			button, field, label := children[0], children[1], children[2]
			if !objc.Send[bool](button, Sel("accessibilityPerformPress")) {
				t.Error("accessibilityPerformPress on the button = false, want true")
			}
			// The label advertises no actions at all, so pressing it must be refused rather than dispatched.
			if objc.Send[bool](label, Sel("accessibilityPerformPress")) {
				t.Error("accessibilityPerformPress on the label = true, want false")
			}
			// Neither the button nor the field advertises increment, so neither may dispatch one.
			if objc.Send[bool](button, Sel("accessibilityPerformIncrement")) {
				t.Error("accessibilityPerformIncrement on the button = true, want false")
			}
			field.Send(Sel("setAccessibilityFocused:"), true)
			field.Send(Sel("setAccessibilityValue:"), NSStringFromGo("replacement"))
			// UTF-16 {2, 6} covers the runes from index 2 up to but not including 7 — the astral-plane rune counts
			// twice.
			field.Send(Sel("setAccessibilitySelectedTextRange:"), NSRange{Location: 2, Length: 6})
			want := []accessibility.ActionRequest{
				{Node: axTestButton, Action: accessibility.Press},
				{Node: axTestField, Action: accessibility.Focus},
				{Node: axTestField, Action: accessibility.SetValue, Value: "replacement"},
				{Node: axTestField, Action: accessibility.SetTextSelection, Start: 2, End: 7},
			}
			if len(requests) != len(want) {
				t.Fatalf("got %d requests (%+v), want %d", len(requests), requests, len(want))
			}
			for i, req := range want {
				if requests[i] != req {
					t.Errorf("request %d = %+v, want %+v", i, requests[i], req)
				}
				if windows[i] != a.wnd {
					t.Errorf("request %d arrived for window %#x, want %#x", i, windows[i], a.wnd)
				}
			}
		})
	})
}

// TestAXAdapterLegacyActions proves the legacy action protocol, which is the only way an element can offer
// AXScrollToVisible: the names an element advertises come from its node's action set, a name performed through
// accessibilityPerformAction: reaches the action callback as the action it stands for, a name the node does not
// advertise or the adapter does not deal in is ignored, and every name has a description.
func TestAXAdapterLegacyActions(t *testing.T) {
	defer func() { AccessibilityActionCallback = nil }()
	runOnMain(func() {
		tree := newAXTestTree()
		tree.Node(axTestButton).Actions = tree.Node(axTestButton).Actions.With(accessibility.ScrollIntoView)
		tree.Node(axTestLabel).Actions = accessibility.ActionSet(0).With(accessibility.ScrollIntoView,
			accessibility.Increment)
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		var requests []accessibility.ActionRequest
		AccessibilityActionCallback = func(_ Window, req accessibility.ActionRequest) {
			requests = append(requests, req)
		}
		WithPool(func() {
			children := axTestChildren(t, v, 3)
			button, field, label := children[0], children[1], children[2]
			names := func(el objc.ID) []string {
				ids := IDsFromNSArray(el.Send(Sel("accessibilityActionNames")))
				out := make([]string, 0, len(ids))
				for _, id := range ids {
					out = append(out, GoStringFromNSString(id))
				}
				return out
			}
			want := []string{"AXPress", "AXConfirm", "AXScrollToVisible"}
			if got := names(button); !slices.Equal(got, want) {
				t.Errorf("button action names = %v, want %v", got, want)
			}
			want = []string{"AXIncrement", "AXScrollToVisible"}
			if got := names(label); !slices.Equal(got, want) {
				t.Errorf("label action names = %v, want %v", got, want)
			}
			// The field advertises none of the actions the legacy protocol deals in: focus and the value setters are
			// attributes, not actions, on this platform.
			if got := names(field); len(got) != 0 {
				t.Errorf("field action names = %v, want none", got)
			}
			for _, name := range []string{"AXPress", "AXScrollToVisible"} {
				if desc := GoStringFromNSString(button.Send(Sel("accessibilityActionDescription:"),
					NSStringFromGo(name))); desc == "" {
					t.Errorf("accessibilityActionDescription: for %s is empty", name)
				}
			}
			button.Send(Sel("accessibilityPerformAction:"), NSStringFromGo("AXScrollToVisible"))
			button.Send(Sel("accessibilityPerformAction:"), NSStringFromGo("AXConfirm"))
			label.Send(Sel("accessibilityPerformAction:"), NSStringFromGo("AXIncrement"))
			// Neither is dispatched: the label does not advertise a press, and nothing deals in AXRaise.
			label.Send(Sel("accessibilityPerformAction:"), NSStringFromGo("AXPress"))
			button.Send(Sel("accessibilityPerformAction:"), NSStringFromGo("AXRaise"))
			wantRequests := []accessibility.ActionRequest{
				{Node: axTestButton, Action: accessibility.ScrollIntoView},
				{Node: axTestButton, Action: accessibility.Press},
				{Node: axTestLabel, Action: accessibility.Increment},
			}
			if !slices.Equal(requests, wantRequests) {
				t.Errorf("requests = %+v, want %+v", requests, wantRequests)
			}
		})
	})
}

// TestAXAdapterText proves the text protocol, which is where the rune-to-UTF-16 conversion lives: character counts,
// selection, line boundaries and the frame of a range all have to be expressed in UTF-16 code units even though the
// snapshot counts runes.
func TestAXAdapterText(t *testing.T) {
	runOnMain(func() {
		v, a, tree, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		ov := objc.ID(v)
		WithPool(func() {
			children := IDsFromNSArray(ov.Send(Sel("accessibilityChildren")))
			if len(children) != 3 {
				t.Fatalf("content view has %d children, want 3", len(children))
			}
			field, label := children[1], children[2]
			// 13 runes, one of which needs two UTF-16 code units.
			if got := objc.Send[int64](field, Sel("accessibilityNumberOfCharacters")); got != 14 {
				t.Errorf("accessibilityNumberOfCharacters = %d, want 14", got)
			}
			if got := GoStringFromNSString(field.Send(Sel("accessibilityValue"))); got != axTestText {
				t.Errorf("accessibilityValue = %q, want %q", got, axTestText)
			}
			if got := GoStringFromNSString(field.Send(Sel("accessibilitySelectedText"))); got != "llo 😀" {
				t.Errorf("accessibilitySelectedText = %q, want %q", got, "llo 😀")
			}
			wantSel := NSRange{Location: 2, Length: 6}
			if got := objc.Send[NSRange](field, Sel("accessibilitySelectedTextRange")); got != wantSel {
				t.Errorf("accessibilitySelectedTextRange = %+v, want %+v", got, wantSel)
			}
			wantVisible := NSRange{Location: 0, Length: 14}
			if got := objc.Send[NSRange](field, Sel("accessibilityVisibleCharacterRange")); got != wantVisible {
				t.Errorf("accessibilityVisibleCharacterRange = %+v, want %+v", got, wantVisible)
			}
			if got := objc.Send[int64](field, Sel("accessibilityInsertionPointLineNumber")); got != 0 {
				t.Errorf("accessibilityInsertionPointLineNumber = %d, want 0", got)
			}
			// A node with no text has no insertion point, and -1 is how that is said. Answering 0 would have every
			// button, row and group this class serves claim a caret sitting on its first line.
			if got := objc.Send[int64](label, Sel("accessibilityInsertionPointLineNumber")); got != -1 {
				t.Errorf("label accessibilityInsertionPointLineNumber = %d, want -1", got)
			}
			for _, c := range []struct {
				want  string
				given NSRange
			}{
				{given: NSRange{Location: 0, Length: 5}, want: "héllo"},
				{given: NSRange{Location: 6, Length: 2}, want: "😀"},
				{given: NSRange{Location: 9, Length: 5}, want: "wörld"},
				{given: NSRange{Location: 100, Length: 5}, want: ""},
			} {
				if got := GoStringFromNSString(field.Send(Sel("accessibilityStringForRange:"),
					c.given)); got != c.want {
					t.Errorf("accessibilityStringForRange:%+v = %q, want %q", c.given, got, c.want)
				}
			}
			// The attributed form of a range must answer the same content as the plain one. NSAccessibilityElement
			// responds to the selector and answers nil, so without this the element advertises
			// AXAttributedStringForRange and then reports every range as empty while AXStringForRange reports it
			// correctly.
			attributed := field.Send(Sel("accessibilityAttributedStringForRange:"), NSRange{Location: 9, Length: 5})
			if attributed == 0 {
				t.Error("accessibilityAttributedStringForRange: = nil, want an attributed string")
			} else {
				if !objc.Send[bool](attributed, Sel("isKindOfClass:"), Cls("NSAttributedString")) {
					t.Error("accessibilityAttributedStringForRange: did not answer an NSAttributedString")
				}
				if got := GoStringFromNSString(attributed.Send(Sel("string"))); got != "wörld" {
					t.Errorf("accessibilityAttributedStringForRange:{9,5} = %q, want %q", got, "wörld")
				}
			}
			// A node holding no text at all answers nothing rather than an empty attributed string, which is what it
			// answers for the plain form too.
			if got := label.Send(Sel("accessibilityAttributedStringForRange:"),
				NSRange{Location: 0, Length: 1}); got != 0 {
				t.Errorf("label accessibilityAttributedStringForRange: = %#x, want nil", got)
			}
			// The first line owns its trailing line feed, so it runs to UTF-16 offset 9: "héllo " is 6 units, the emoji
			// is 2 and the line feed is 1. A line the field does not have is no range at all, which NSAccessibility
			// spells {NSNotFound, 0}: {0, 0} would read as a real empty range at the start of the content.
			for _, c := range []struct {
				want NSRange
				line int64
			}{
				{line: 0, want: NSRange{Location: 0, Length: 9}},
				{line: 1, want: NSRange{Location: 9, Length: 5}},
				{line: 2, want: emptyRange},
			} {
				if got := objc.Send[NSRange](field, Sel("accessibilityRangeForLine:"), c.line); got != c.want {
					t.Errorf("accessibilityRangeForLine:%d = %+v, want %+v", c.line, got, c.want)
				}
			}
			for _, c := range []struct {
				index int64
				want  int64
			}{{index: 0, want: 0}, {index: 8, want: 0}, {index: 9, want: 1}, {index: 13, want: 1}, {index: 99, want: 1}} {
				if got := objc.Send[int64](field, Sel("accessibilityLineForIndex:"), c.index); got != c.want {
					t.Errorf("accessibilityLineForIndex:%d = %d, want %d", c.index, got, c.want)
				}
			}
			// The astral-plane rune is one character occupying two UTF-16 code units, and asking about either half of
			// it must describe the whole of it.
			for _, index := range []int64{6, 7} {
				want := NSRange{Location: 6, Length: 2}
				if got := objc.Send[NSRange](field, Sel("accessibilityRangeForIndex:"), index); got != want {
					t.Errorf("accessibilityRangeForIndex:%d = %+v, want %+v", index, got, want)
				}
			}
			// The frame of the first character comes from the line's own advances, offset by where the control sits.
			bounds := tree.Node(axTestField).Bounds
			want := a.screenRect(geom.NewRect(bounds.X, bounds.Y, 10, 16))
			if got := objc.Send[NSRect](field, Sel("accessibilityFrameForRange:"),
				NSRange{Location: 0, Length: 1}); got != want {
				t.Errorf("accessibilityFrameForRange:{0,1} = %+v, want %+v", got, want)
			}
			// A position a quarter of the way along the first line lands on the third rune, whose UTF-16 range is
			// {2,1}.
			pt := axTestScreenPoint(a, geom.NewPoint(bounds.X+25, bounds.Y+8))
			wantRange := NSRange{Location: 2, Length: 1}
			if got := objc.Send[NSRange](field, Sel("accessibilityRangeForPosition:"), pt); got != wantRange {
				t.Errorf("accessibilityRangeForPosition: = %+v, want %+v", got, wantRange)
			}
			// A node with no text at all must answer safely rather than reach into a nil TextInfo.
			if got := objc.Send[int64](label, Sel("accessibilityNumberOfCharacters")); got != 0 {
				t.Errorf("label accessibilityNumberOfCharacters = %d, want 0", got)
			}
			// An element holding no text has no lines either: every line is no range at all, and no index sits on one,
			// which -1 says the way accessibilityInsertionPointLineNumber says it. Answering {0, 0} and 0 would have
			// every button, row and group this class serves claim a first line it has no content for.
			if got := objc.Send[NSRange](label, Sel("accessibilityRangeForLine:"), int64(0)); got != emptyRange {
				t.Errorf("label accessibilityRangeForLine:0 = %+v, want %+v", got, emptyRange)
			}
			if got := objc.Send[int64](label, Sel("accessibilityLineForIndex:"), int64(0)); got != -1 {
				t.Errorf("label accessibilityLineForIndex:0 = %d, want -1", got)
			}
			if got := objc.Send[NSRange](label, Sel("accessibilitySelectedTextRange")); got != (NSRange{}) {
				t.Errorf("label accessibilitySelectedTextRange = %+v, want the zero range", got)
			}
			// Static text reports its content as its value, since that is the half of a label macOS reads.
			if got := GoStringFromNSString(label.Send(Sel("accessibilityValue"))); got != "Ready" {
				t.Errorf("label accessibilityValue = %q, want %q", got, "Ready")
			}
			// The caret is the end of the selection that moves as it is extended rather than whichever end is later, so
			// a selection dragged backwards from the second line to the first has its caret on the first line, and that
			// is the line the insertion point sits on. Taking SelEnd for it would name the second.
			backward := newAXTestTree()
			backward.Generation = 2
			info := backward.Nodes[axTestField].Text
			info.SelStart, info.SelEnd, info.Caret = 5, 10, 5
			a.Publish(backward, accessibility.Diff(tree, backward))
			if got := objc.Send[int64](field, Sel("accessibilityInsertionPointLineNumber")); got != 0 {
				t.Errorf("accessibilityInsertionPointLineNumber for a backward selection = %d, want 0", got)
			}
			forward := newAXTestTree()
			forward.Generation = 3
			info = forward.Nodes[axTestField].Text
			info.SelStart, info.SelEnd, info.Caret = 5, 10, 10
			a.Publish(forward, accessibility.Diff(backward, forward))
			if got := objc.Send[int64](field, Sel("accessibilityInsertionPointLineNumber")); got != 1 {
				t.Errorf("accessibilityInsertionPointLineNumber for a forward selection = %d, want 1", got)
			}
			// Either way the selection itself is the whole run between the two ends.
			wantSel = NSRange{Location: 5, Length: 6}
			if got := objc.Send[NSRange](field, Sel("accessibilitySelectedTextRange")); got != wantSel {
				t.Errorf("accessibilitySelectedTextRange after the selection moved = %+v, want %+v", got, wantSel)
			}
		})
	})
}

// TestAXAdapterRemovalAndShutdown proves the element lifetime: an element exists only once something has asked about
// its node, is released when the node leaves the tree, answers safely in between, and the whole adapter lets go of
// everything on shutdown.
func TestAXAdapterRemovalAndShutdown(t *testing.T) {
	runOnMain(func() {
		v, a, tree, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		ov := objc.ID(v)
		WithPool(func() {
			// Nothing has asked about the button yet, so no element exists for it.
			if got := a.Element(axTestButton); got != 0 {
				t.Errorf("Element(button) = %#x before anything asked, want 0", got)
			}
			if NSArrayCount(ov.Send(Sel("accessibilityChildren"))) != 3 {
				t.Error("content view did not report three children")
			}
			label := a.Element(axTestLabel)
			if label == 0 {
				t.Fatal("Element(label) = 0 after the children were listed")
			}
			// Retained by the test so it can still be messaged after the adapter has let go of it, which is exactly the
			// position an assistive technology holding a stale element is in.
			Retain(label)
			defer Release(label)

			// The same tree without the label: the diff has to produce a NodeRemoved, and the adapter has to release
			// the element it was holding.
			next := newAXTestTree()
			next.Generation = 2
			next.Nodes[axTestGroup].Children = nil
			delete(next.Nodes, axTestLabel)
			events := accessibility.Diff(tree, next)
			a.Publish(next, events)
			if got := a.Element(axTestLabel); got != 0 {
				t.Errorf("Element(label) = %#x after the node left the tree, want 0", got)
			}
			if NSArrayCount(ov.Send(Sel("accessibilityChildren"))) != 2 {
				t.Error("content view did not drop to two children after the label was removed")
			}
			// The stale element must still answer, with the defaults for a node that is no longer there.
			if objc.Send[bool](label, Sel("isAccessibilityElement")) {
				t.Error("a stale element reports itself as an accessibility element, want not")
			}
			if got := GoStringFromNSString(label.Send(Sel("accessibilityRole"))); got != "AXUnknown" {
				t.Errorf("stale element role = %q, want AXUnknown", got)
			}
			if got := objc.Send[NSRect](label, Sel("accessibilityFrame")); got != (NSRect{}) {
				t.Errorf("stale element frame = %+v, want the zero rect", got)
			}
			if got := label.Send(Sel("accessibilityParent")); got != 0 {
				t.Errorf("stale element parent = %#x, want nil", got)
			}
		})
		// Every element left is announced as destroyed while the adapter is still registered for the view. AppKit
		// routes each of those notifications by asking the element what window and parent it belonged to, and an
		// element whose adapter has already been forgotten answers nothing at all: the notification an assistive
		// technology must learn the element is gone from would name a thing with no place in any window.
		ensureAXNotifyFuncs()
		realPost := axPostNotify
		defer func() { axPostNotify = realPost }()
		type destroyed struct {
			element objc.ID
			window  objc.ID
			adapted bool
		}
		var destroyedElements []destroyed
		wantDestroyed := GoStringFromNSString(AppKitString(axNotifyUIElementDestroyed))
		axPostNotify = func(element, notification objc.ID) {
			if GoStringFromNSString(notification) == wantDestroyed {
				destroyedElements = append(destroyedElements, destroyed{
					element: element,
					window:  element.Send(Sel("accessibilityWindow")),
					adapted: axViewIsAdapted(v),
				})
			}
			realPost(element, notification)
		}
		a.Shutdown()
		if len(destroyedElements) != 2 {
			t.Errorf("%d elements were announced as destroyed, want 2 (the button and the field)",
				len(destroyedElements))
		}
		for _, d := range destroyedElements {
			if !d.adapted {
				t.Errorf("element %#x was announced as destroyed after the adapter had already been forgotten",
					d.element)
			}
			if d.window != objc.ID(a.wnd) {
				t.Errorf("element %#x reported window %#x while being announced as destroyed, want %#x", d.element,
					d.window, objc.ID(a.wnd))
			}
		}
		if axViewIsAdapted(v) {
			t.Error("the content view still has an adapter after Shutdown")
		}
		if len(axAdapters) != 0 {
			t.Errorf("axAdapters holds %d entries after Shutdown, want 0", len(axAdapters))
		}
		if got := a.Element(axTestField); got != 0 {
			t.Errorf("Element(field) = %#x after Shutdown, want 0", got)
		}
	})
}

// TestAXAdapterActivation proves the lazy activation path: a content view with no adapter asks
// AccessibilityActivateCallback for one, and falls back to AppKit's own answers when there is nothing to activate.
func TestAXAdapterActivation(t *testing.T) {
	defer func() { AccessibilityActivateCallback = nil }()
	runOnMain(func() {
		w, v, closeWindow := newTestWindowAndView(t)
		defer closeWindow()
		w.MakeKeyAndOrderFront()
		ov := objc.ID(v)

		// With no adapter and no callback, the three activating selectors must fall through to AppKit rather than
		// invent an answer. A view with no subviews reports no children.
		AccessibilityActivateCallback = nil
		WithPool(func() {
			if got := NSArrayCount(ov.Send(Sel("accessibilityChildren"))); got != 0 {
				t.Errorf("unadapted content view reported %d children, want 0", got)
			}
			// Whatever AppKit answers with — nil, the view, or the window the ignored view resolves to — it must not be
			// one of the adapter's own elements, since there is no adapter to have made one.
			if got := ov.Send(Sel("accessibilityHitTest:"), NSPoint{}); got != 0 {
				if name := GoStringFromNSString(got.Send(Sel("className"))); name == "UnisonAXElement" {
					t.Error("an unadapted content view answered a hit test with an adapter element")
				}
			}
		})
		if axViewIsAdapted(v) {
			t.Error("querying an unadapted content view created an adapter")
		}

		// A callback that activates: the adapter is created and the query it was created for is answered from it.
		var activated []Window
		AccessibilityActivateCallback = func(macWnd Window) bool {
			activated = append(activated, macWnd)
			a := NewAXAdapter(v)
			if a == nil {
				return false
			}
			tree := newAXTestTree()
			a.Publish(tree, accessibility.Diff(nil, tree))
			return true
		}
		defer func() {
			if a := axAdapters[v]; a != nil {
				a.Shutdown()
			}
		}()
		WithPool(func() {
			if got := NSArrayCount(ov.Send(Sel("accessibilityChildren"))); got != 3 {
				t.Errorf("content view reported %d children after activation, want 3", got)
			}
		})
		if len(activated) != 1 || activated[0] != w {
			t.Errorf("activation callbacks = %v, want one for window %#x", activated, w)
		}
		// A second query must not activate again.
		WithPool(func() { ov.Send(Sel("accessibilityFocusedUIElement")) })
		if len(activated) != 1 {
			t.Errorf("activation callback ran %d times, want 1", len(activated))
		}
	})
}

// TestAXNoActivationFromOrdinaryWindowUse is the zero-cost-when-inactive guard for this platform: creating a window,
// showing it, drawing it, resizing it and tearing it down must never reach AccessibilityActivateCallback, because only
// an assistive technology's own questions may turn snapshot building on. It is skipped while VoiceOver is running,
// since a real assistive technology asking real questions is exactly what the callback is for.
func TestAXNoActivationFromOrdinaryWindowUse(t *testing.T) {
	var voiceOver bool
	runOnMain(func() {
		WithPool(func() {
			voiceOver = objc.Send[bool](objc.ID(Cls("NSWorkspace")).Send(Sel("sharedWorkspace")),
				Sel("isVoiceOverEnabled"))
		})
	})
	if voiceOver {
		t.Skip("VoiceOver is running, so accessibility queries are expected")
	}
	defer func() { AccessibilityActivateCallback = nil }()
	runOnMain(func() {
		activations := 0
		AccessibilityActivateCallback = func(Window) bool {
			activations++
			return false
		}
		w, v, closeWindow := newTestWindowAndView(t)
		w.MakeKeyAndOrderFront()
		WithPool(func() {
			objc.ID(v).Send(Sel("setNeedsDisplay:"), true)
			objc.ID(v).Send(Sel("drawRect:"), NSRect{Size: NSSize{Width: 10, Height: 10}})
			w.SetFrame(geom.NewRect(140, 140, 300, 220))
			objc.ID(v).Send(Sel("updateTrackingAreas"))
			// Give AppKit a turn of the run loop, which is where anything it wanted to ask would arrive.
			date := objc.ID(Cls("NSDate")).Send(Sel("dateWithTimeIntervalSinceNow:"), 0.05)
			objc.ID(Cls("NSRunLoop")).Send(Sel("currentRunLoop")).Send(Sel("runMode:beforeDate:"),
				defaultRunLoopMode(), date)
		})
		closeWindow()
		if activations != 0 {
			t.Errorf("ordinary window use activated accessibility %d times, want 0 (is an assistive technology or the "+
				"Accessibility Inspector running?)", activations)
		}
		if axViewIsAdapted(v) {
			t.Error("ordinary window use created an accessibility adapter")
		}
	})
}

// TestAXHeadingRole records what this version of macOS does with the heading role. macOS has no public one, so the
// adapter tries the "AXHeading" that web content uses and falls back to static text with a role description of
// "heading"; either answer is correct, but the element must never be left without a role description that says what it
// is.
func TestAXHeadingRole(t *testing.T) {
	runOnMain(func() {
		v, a, tree, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		heading := &accessibility.Node{
			ID:     axTestLabel + 1,
			Parent: axTestRoot,
			Name:   "Section",
			Role:   role.Heading,
			Bounds: geom.NewRect(10, 150, 100, 20),
			Level:  2,
		}
		next := newAXTestTree()
		next.Generation = 2
		next.Nodes[heading.ID] = heading
		next.Nodes[axTestRoot].Children = append(next.Nodes[axTestRoot].Children, heading.ID)
		a.Publish(next, accessibility.Diff(tree, next))
		WithPool(func() {
			children := IDsFromNSArray(objc.ID(v).Send(Sel("accessibilityChildren")))
			if len(children) != 4 {
				t.Fatalf("content view has %d children, want 4", len(children))
			}
			element := children[3]
			roleName := GoStringFromNSString(element.Send(Sel("accessibilityRole")))
			description := GoStringFromNSString(element.Send(Sel("accessibilityRoleDescription")))
			t.Logf("heading role = %q, role description = %q", roleName, description)
			if roleName != axHeadingCandidateRole && roleName != "AXStaticText" {
				t.Errorf("heading role = %q, want %q or AXStaticText", roleName, axHeadingCandidateRole)
			}
			if description != "heading" {
				t.Errorf("heading role description = %q, want %q", description, "heading")
			}
			// A heading's level is what it reports as its value, which is how macOS expresses heading depth.
			if got := Int64FromNSNumber(element.Send(Sel("accessibilityValue"))); got != 2 {
				t.Errorf("heading value = %d, want 2", got)
			}
		})
	})
}

// TestAXAdapterNotifications proves publishing does not fall over while posting the notification set a real change
// produces, and that AXAnnounce reaches AppKit. The accessibility system swallows notifications when nothing is
// listening, so what is asserted is that the adapter's state is consistent afterwards rather than what VoiceOver heard.
func TestAXAdapterNotifications(t *testing.T) {
	runOnMain(func() {
		v, a, tree, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			// Create the elements first, so the notifications have something to be posted against.
			if NSArrayCount(objc.ID(v).Send(Sel("accessibilityChildren"))) != 3 {
				t.Error("content view did not report three children")
			}
		})
		next := newAXTestTree()
		next.Generation = 2
		next.Focus = axTestButton
		next.Nodes[axTestButton].Name = "Cancel"
		next.Nodes[axTestButton].Focused = true
		next.Nodes[axTestField].Focused = false
		next.Nodes[axTestField].Value = "changed"
		next.Nodes[axTestField].Text.Text = "changed"
		next.Nodes[axTestField].Text.SelStart = 1
		next.Nodes[axTestField].Text.SelEnd = 1
		next.Nodes[axTestField].Text.Caret = 1
		next.Nodes[axTestField].Text.Lines = nil
		next.Nodes[axTestLabel].Bounds = geom.NewRect(12, 104, 60, 16)
		events := accessibility.Diff(tree, next)
		if len(events) == 0 {
			t.Error("the diff produced no events")
		}
		// Record what is posted with additional information, which is how the layout-changed notification names the
		// elements whose frames moved.
		ensureAXNotifyFuncs()
		realPostInfo := axPostNotifyInfo
		defer func() { axPostNotifyInfo = realPostInfo }()
		// The user info is autoreleased, so what it names is read out while it is still alive.
		type posted struct {
			name         string
			announcement string
			elements     []objc.ID
			element      objc.ID
			priority     int64
		}
		var postedInfo []posted
		axPostNotifyInfo = func(element, notification, userInfo objc.ID) {
			postedInfo = append(postedInfo, posted{
				element:  element,
				name:     GoStringFromNSString(notification),
				elements: IDsFromNSArray(userInfo.Send(Sel("objectForKey:"), AppKitString(axKeyUIElements))),
				announcement: GoStringFromNSString(userInfo.Send(Sel("objectForKey:"),
					AppKitString(axKeyAnnouncement))),
				priority: Int64FromNSNumber(userInfo.Send(Sel("objectForKey:"), AppKitString(axKeyPriority))),
			})
			realPostInfo(element, notification, userInfo)
		}
		a.Publish(next, events)
		if a.tree != next {
			t.Error("Publish did not install the new tree")
		}
		layoutChanged := 0
		wantName := GoStringFromNSString(AppKitString(axNotifyLayoutChanged))
		for _, p := range postedInfo {
			if p.element != objc.ID(v) || p.name != wantName {
				continue
			}
			layoutChanged++
			if len(p.elements) != 1 || p.elements[0] != a.Element(axTestLabel) {
				t.Errorf("layout-changed notification named %v, want just the label %#x", p.elements,
					a.Element(axTestLabel))
			}
		}
		if layoutChanged != 1 {
			t.Errorf("%d layout-changed notifications were posted, want 1", layoutChanged)
		}
		WithPool(func() {
			button := a.Element(axTestButton)
			if got := GoStringFromNSString(button.Send(Sel("accessibilityLabel"))); got != "Cancel" {
				t.Errorf("button label after publish = %q, want Cancel", got)
			}
			if !objc.Send[bool](button, Sel("isAccessibilityFocused")) {
				t.Error("button is not focused after publish")
			}
			// Without measured lines the whole content is one line, which is what an unfocused control reports.
			field := a.Element(axTestField)
			want := NSRange{Location: 0, Length: 7}
			if got := objc.Send[NSRange](field, Sel("accessibilityRangeForLine:"), int64(0)); got != want {
				t.Errorf("accessibilityRangeForLine:0 without measured lines = %+v, want %+v", got, want)
			}
			if got := objc.Send[int64](field, Sel("accessibilityLineForIndex:"), int64(5)); got != 0 {
				t.Errorf("accessibilityLineForIndex:5 without measured lines = %d, want 0", got)
			}
			// The frame of a range falls back to the whole control when there are no measured lines.
			wantFrame := a.screenRect(next.Nodes[axTestField].Bounds)
			if got := objc.Send[NSRect](field, Sel("accessibilityFrameForRange:"),
				NSRange{Location: 0, Length: 1}); got != wantFrame {
				t.Errorf("accessibilityFrameForRange without measured lines = %+v, want %+v", got, wantFrame)
			}
		})
		// An announcement reaches this adapter through AXAnnounce alone: it is something an application asks to have
		// spoken rather than anything a change to a window expresses, so no published event carries one.
		AXAnnounce("all done")
		AXAnnounce("spoken directly")
		AXAnnounce("") // must be a no-op rather than an exception
		// An announcement is the one notification whose whole content is in the user info: the text to speak and the
		// priority that decides whether it interrupts what is being said. It is posted against the application rather
		// than against any window or element, since that is where AppKit looks for announcements.
		var announcements []posted
		wantAnnouncement := GoStringFromNSString(AppKitString(axNotifyAnnouncementRequested))
		for _, p := range postedInfo {
			if p.name == wantAnnouncement {
				announcements = append(announcements, p)
			}
		}
		if len(announcements) != 2 {
			t.Fatalf("%d announcements were posted, want 2 (the empty one must post nothing at all)",
				len(announcements))
		}
		for i, want := range []string{"all done", "spoken directly"} {
			if announcements[i].announcement != want {
				t.Errorf("announcement %d spoke %q, want %q", i, announcements[i].announcement, want)
			}
			if announcements[i].priority != axPriorityHigh {
				t.Errorf("announcement %d priority = %d, want %d", i, announcements[i].priority, axPriorityHigh)
			}
			if announcements[i].element != sharedApp() {
				t.Errorf("announcement %d was posted on %#x, want the application %#x", i, announcements[i].element,
					sharedApp())
			}
		}
	})
}

// TestAXAdapterStateAndCollections proves the remaining attribute answers — the check, toggle, numeric, orientation,
// sort, row and tab reports — against nodes built for each of them.
func TestAXAdapterStateAndCollections(t *testing.T) {
	defer func() { AccessibilityActionCallback = nil }()
	runOnMain(func() {
		w, v, closeWindow := newTestWindowAndView(t)
		defer closeWindow()
		w.MakeKeyAndOrderFront()
		a := NewAXAdapter(v)
		if a == nil {
			t.Error("NewAXAdapter returned nil")
			return
		}
		defer a.Shutdown()
		const (
			rootID accessibility.NodeID = 100 + iota
			checkID
			sliderID
			treeID
			row1ID
			row2ID
			tabListID
			tabID
			headerID
			separatorID
			scrollID
			vBarID
			hBarID
			contentID
			cellAID
			boxID
			cellBID
			labelAID
			labelBID
			discID
			panelID
		)
		nodes := map[accessibility.NodeID]*accessibility.Node{
			rootID: {
				ID: rootID,
				Children: []accessibility.NodeID{
					checkID, sliderID, treeID, tabListID, headerID, separatorID, scrollID, panelID,
				},
				Role:   role.Window,
				Bounds: geom.NewRect(0, 0, 320, 240),
			},
			checkID: {
				ID: checkID, Parent: rootID, Role: role.CheckBox, Name: "Mixed", Bounds: geom.NewRect(0, 0, 20, 20),
				HasCheck: true, Checked: 2, // check.Mixed
			},
			sliderID: {
				ID: sliderID, Parent: rootID, Role: role.Slider, Name: "Volume", Bounds: geom.NewRect(0, 20, 100, 20),
				HasNumber: true, Number: 3, Min: 1, Max: 9, Step: 1,
				Orientation: accessibility.OrientationVertical,
				Actions:     accessibility.ActionSet(0).With(accessibility.Increment, accessibility.Decrement),
			},
			treeID: {
				ID: treeID, Parent: rootID, Children: []accessibility.NodeID{row1ID, row2ID}, Role: role.Tree,
				Name: "Items", Bounds: geom.NewRect(0, 40, 200, 60), Multiselectable: true, RowCount: 2,
			},
			row1ID: {
				ID: row1ID, Parent: treeID, Children: []accessibility.NodeID{discID, cellAID, cellBID}, Role: role.Row,
				Name: "One", Bounds: geom.NewRect(0, 40, 200, 20), RowIndex: 0, Level: 1, Selectable: true,
				Expandable: true, Expanded: true,
				Actions: accessibility.ActionSet(0).With(accessibility.Select, accessibility.AddToSelection,
					accessibility.Expand, accessibility.ShowContextMenu),
			},
			discID: {
				ID: discID, Parent: row1ID, Role: role.DisclosureTriangle, Name: "Disclosure Triangle",
				Bounds: geom.NewRect(0, 42, 16, 16), Pressed: true, Expandable: true, Expanded: true,
				Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Toggle),
			},
			// A cell holding one check box is presented as the check box itself, in the cell's place in the grid.
			cellAID: {
				ID: cellAID, Parent: row1ID, Children: []accessibility.NodeID{boxID}, Role: role.Cell,
				Bounds: geom.NewRect(0, 40, 40, 20), RowIndex: 0, ColumnIndex: 0,
			},
			boxID: {
				ID: boxID, Parent: cellAID, Role: role.CheckBox, Name: "Done", Bounds: geom.NewRect(2, 42, 16, 16),
				HasCheck: true, Actions: accessibility.ActionSet(0).With(accessibility.Press),
			},
			// A cell holding more than one thing stays a cell.
			cellBID: {
				ID: cellBID, Parent: row1ID, Children: []accessibility.NodeID{labelAID, labelBID}, Role: role.Cell,
				Bounds: geom.NewRect(40, 40, 160, 20), RowIndex: 0, ColumnIndex: 1,
			},
			labelAID: {
				ID: labelAID, Parent: cellBID, Role: role.Label, Name: "First", Bounds: geom.NewRect(40, 40, 80, 20),
			},
			labelBID: {
				ID: labelBID, Parent: cellBID, Role: role.Label, Name: "Second", Bounds: geom.NewRect(120, 40, 80, 20),
			},
			row2ID: {
				ID: row2ID, Parent: treeID, Role: role.Row, Name: "Two", Bounds: geom.NewRect(0, 60, 200, 20),
				RowIndex: 1, Level: 2, Selectable: true, Selected: true, Expandable: true, Expanded: true,
				Actions: accessibility.ActionSet(0).With(accessibility.Collapse, accessibility.Select,
					accessibility.AddToSelection, accessibility.RemoveFromSelection),
			},
			tabListID: {
				ID: tabListID, Parent: rootID, Children: []accessibility.NodeID{tabID}, Role: role.TabList,
				Bounds: geom.NewRect(0, 100, 200, 20),
			},
			tabID: {
				ID: tabID, Parent: tabListID, Role: role.Tab, Name: "First", Bounds: geom.NewRect(0, 100, 60, 20),
				Selected: true, Controls: []accessibility.NodeID{panelID},
			},
			// The panel a tab shows, named by the tab rather than by any text of its own, which is what
			// AXTitleUIElement reports, and what the tab's AXLinkedUIElements points at from the other end.
			panelID: {
				ID: panelID, Parent: rootID, Role: role.TabPanel, Bounds: geom.NewRect(0, 120, 200, 20),
				LabeledBy: []accessibility.NodeID{tabID},
			},
			headerID: {
				ID: headerID, Parent: rootID, Role: role.ColumnHeader, Name: "Size",
				Bounds: geom.NewRect(0, 120, 60, 20), Sort: accessibility.SortDescending,
			},
			separatorID: {
				ID: separatorID, Parent: rootID, Role: role.Separator, Bounds: geom.NewRect(0, 140, 200, 1),
			},
			scrollID: {
				ID: scrollID, Parent: rootID, Children: []accessibility.NodeID{hBarID, vBarID, contentID},
				Role: role.ScrollArea, Bounds: geom.NewRect(0, 150, 200, 80),
			},
			// The horizontal bar has nothing to scroll, so it is ignored and must not be offered.
			hBarID: {
				ID: hBarID, Parent: scrollID, Role: role.ScrollBar, Orientation: accessibility.OrientationHorizontal,
				Bounds: geom.NewRect(0, 220, 190, 10), Ignored: true,
			},
			vBarID: {
				ID: vBarID, Parent: scrollID, Role: role.ScrollBar, Orientation: accessibility.OrientationVertical,
				Bounds: geom.NewRect(190, 150, 10, 80), HasNumber: true, Number: 75, Max: 300, Step: 8,
				Actions: accessibility.ActionSet(0).With(accessibility.Increment, accessibility.Decrement,
					accessibility.SetValue),
			},
			contentID: {
				ID: contentID, Parent: scrollID, Role: role.Group, Name: "Long form",
				Bounds: geom.NewRect(0, 150, 190, 380),
			},
		}
		tree := &accessibility.Tree{Nodes: nodes, Root: rootID, Generation: 1}
		a.Publish(tree, accessibility.Diff(nil, tree))
		var requests []accessibility.ActionRequest
		AccessibilityActionCallback = func(_ Window, req accessibility.ActionRequest) {
			requests = append(requests, req)
		}
		WithPool(func() {
			children := IDsFromNSArray(objc.ID(v).Send(Sel("accessibilityChildren")))
			if len(children) != 8 {
				t.Fatalf("content view has %d children, want 8", len(children))
			}
			checkBox, slider, outline := children[0], children[1], children[2]
			tabList, header, separator, scrollArea := children[3], children[4], children[5], children[6]
			panel := children[7]
			if got := Int64FromNSNumber(checkBox.Send(Sel("accessibilityValue"))); got != 2 {
				t.Errorf("mixed check box value = %d, want 2", got)
			}
			if got := GoStringFromNSString(slider.Send(Sel("accessibilityRole"))); got != "AXSlider" {
				t.Errorf("slider role = %q, want AXSlider", got)
			}
			if got := Float64FromNSNumber(slider.Send(Sel("accessibilityValue"))); got != 3 {
				t.Errorf("slider value = %v, want 3", got)
			}
			if got := Float64FromNSNumber(slider.Send(Sel("accessibilityMinValue"))); got != 1 {
				t.Errorf("slider min = %v, want 1", got)
			}
			if got := Float64FromNSNumber(slider.Send(Sel("accessibilityMaxValue"))); got != 9 {
				t.Errorf("slider max = %v, want 9", got)
			}
			if got := objc.Send[int64](slider, Sel("accessibilityOrientation")); got != axOrientationVertical {
				t.Errorf("slider orientation = %d, want %d", got, axOrientationVertical)
			}
			if got := GoStringFromNSString(outline.Send(Sel("accessibilityRole"))); got != "AXOutline" {
				t.Errorf("tree role = %q, want AXOutline", got)
			}
			rows := IDsFromNSArray(outline.Send(Sel("accessibilityRows")))
			if len(rows) != 2 {
				t.Fatalf("tree reported %d rows, want 2", len(rows))
			}
			selected := IDsFromNSArray(outline.Send(Sel("accessibilitySelectedRows")))
			if len(selected) != 1 || selected[0] != rows[1] {
				t.Errorf("tree selected rows = %v, want just the second row", selected)
			}
			if got := GoStringFromNSString(rows[1].Send(Sel("accessibilitySubrole"))); got != "AXOutlineRow" {
				t.Errorf("row subrole under a tree = %q, want AXOutlineRow", got)
			}
			// VoiceOver moves an outline's selection by setting its selected rows: the first replaces the selection
			// and the rest join it, so that is what must be asked of the rows, in that order.
			requests = nil
			outline.Send(Sel("setAccessibilitySelectedRows:"), NSArrayFromIDs(rows[0], rows[1]))
			wantRequests := []accessibility.ActionRequest{
				{Node: row1ID, Action: accessibility.Select},
				{Node: row2ID, Action: accessibility.AddToSelection},
			}
			if !slices.Equal(requests, wantRequests) {
				t.Errorf("setting the selected rows asked for %v, want %v", requests, wantRequests)
			}
			// Something that is not one of this outline's rows is passed over rather than acted on.
			requests = nil
			outline.Send(Sel("setAccessibilitySelectedRows:"), NSArrayFromIDs(checkBox, rows[1]))
			wantRequests = []accessibility.ActionRequest{{Node: row2ID, Action: accessibility.Select}}
			if !slices.Equal(requests, wantRequests) {
				t.Errorf("setting the selected rows with a stray element asked for %v, want %v", requests,
					wantRequests)
			}
			if visible := IDsFromNSArray(outline.Send(Sel("accessibilityVisibleRows"))); len(visible) != 2 {
				t.Errorf("tree reported %d visible rows, want 2", len(visible))
			}
			if disclosed := IDsFromNSArray(rows[0].Send(Sel("accessibilityDisclosedRows"))); len(disclosed) != 1 ||
				disclosed[0] != rows[1] {
				t.Errorf("first row discloses %v, want just the second row %#x", disclosed, rows[1])
			}
			if got := rows[1].Send(Sel("accessibilityDisclosedByRow")); got != rows[0] {
				t.Errorf("second row is disclosed by %#x, want the first row %#x", got, rows[0])
			}
			if got := rows[0].Send(Sel("accessibilityDisclosedByRow")); got != 0 {
				t.Errorf("top-level row is disclosed by %#x, want nothing", got)
			}
			if got := objc.Send[NSRange](rows[1], Sel("accessibilityRowIndexRange")); got != (NSRange{Location: 1, Length: 1}) {
				t.Errorf("second row index range = %v, want {1 1}", got)
			}
			if got := objc.Send[int64](rows[1], Sel("accessibilityIndex")); got != 1 {
				t.Errorf("second row index = %d, want 1", got)
			}
			if !objc.Send[bool](rows[1], Sel("isAccessibilityDisclosed")) {
				t.Error("expanded row is not disclosed")
			}
			if got := objc.Send[int64](rows[1], Sel("accessibilityDisclosureLevel")); got != 1 {
				t.Errorf("second row disclosure level = %d, want 1", got)
			}
			if got := objc.Send[int64](rows[0], Sel("accessibilityDisclosureLevel")); got != 0 {
				t.Errorf("first row disclosure level = %d, want 0", got)
			}
			if got := GoStringFromNSString(tabList.Send(Sel("accessibilityRole"))); got != "AXTabGroup" {
				t.Errorf("tab list role = %q, want AXTabGroup", got)
			}
			tabs := IDsFromNSArray(tabList.Send(Sel("accessibilityTabs")))
			if len(tabs) != 1 {
				t.Fatalf("tab list reported %d tabs, want 1", len(tabs))
			}
			if got := GoStringFromNSString(tabs[0].Send(Sel("accessibilitySubrole"))); got != "AXTabButton" {
				t.Errorf("tab subrole = %q, want AXTabButton", got)
			}
			if got := Int64FromNSNumber(tabs[0].Send(Sel("accessibilityValue"))); got != 1 {
				t.Errorf("selected tab value = %d, want 1", got)
			}
			if got := GoStringFromNSString(header.Send(Sel("accessibilitySubrole"))); got != "AXSortButton" {
				t.Errorf("column header subrole = %q, want AXSortButton", got)
			}
			if got := objc.Send[int64](header, Sel("accessibilitySortDirection")); got != axSortDirectionDescending {
				t.Errorf("column header sort direction = %d, want %d", got, axSortDirectionDescending)
			}
			if objc.Send[bool](separator, Sel("isAccessibilityElement")) {
				t.Error("a separator reports itself as an accessibility element, want not")
			}
			// The first row shows its check box in place of the cell holding it, and the check box answers for the
			// cell's place in the grid; the cell holding two labels is shown as a cell.
			rowChildren := IDsFromNSArray(rows[0].Send(Sel("accessibilityChildren")))
			if len(rowChildren) != 3 {
				t.Fatalf("first row has %d children, want 3", len(rowChildren))
			}
			disclosure, box, cellB := rowChildren[0], rowChildren[1], rowChildren[2]
			// The disclosure triangle is named by its role alone and reports whether it is open as its value.
			if got := GoStringFromNSString(disclosure.Send(Sel("accessibilityRole"))); got != "AXDisclosureTriangle" {
				t.Errorf("disclosure role = %q, want AXDisclosureTriangle", got)
			}
			if got := disclosure.Send(Sel("accessibilityLabel")); got != 0 {
				t.Errorf("disclosure label = %q, want none", GoStringFromNSString(got))
			}
			if got := Int64FromNSNumber(disclosure.Send(Sel("accessibilityValue"))); got != 1 {
				t.Errorf("open disclosure value = %d, want 1", got)
			}
			// VoiceOver opens and closes a row by setting whether it is disclosed.
			requests = nil
			rows[1].Send(Sel("setAccessibilityDisclosed:"), false)
			wantRequests = []accessibility.ActionRequest{{Node: row2ID, Action: accessibility.Collapse}}
			if !slices.Equal(requests, wantRequests) {
				t.Errorf("setting the row undisclosed asked for %v, want %v", requests, wantRequests)
			}
			if got := GoStringFromNSString(box.Send(Sel("accessibilityRole"))); got != "AXCheckBox" {
				t.Errorf("first child of the row = %q, want AXCheckBox standing in for its cell", got)
			}
			if got := box.Send(Sel("accessibilityParent")); got != rows[0] {
				t.Errorf("the check box's parent = %#x, want the row %#x", got, rows[0])
			}
			if got := objc.Send[NSRange](box, Sel("accessibilityColumnIndexRange")); got != (NSRange{Length: 1}) {
				t.Errorf("the check box's column range = %v, want {0 1}", got)
			}
			if got := objc.Send[NSRange](box, Sel("accessibilityRowIndexRange")); got != (NSRange{Length: 1}) {
				t.Errorf("the check box's row range = %v, want {0 1}", got)
			}
			if got := GoStringFromNSString(cellB.Send(Sel("accessibilityRole"))); got != "AXCell" {
				t.Errorf("second child of the row = %q, want AXCell", got)
			}
			if got := objc.Send[NSRange](cellB, Sel("accessibilityColumnIndexRange")); got != (NSRange{
				Location: 1,
				Length:   1,
			}) {
				t.Errorf("the second cell's column range = %v, want {1 1}", got)
			}
			// A hit on the cell's own area, beside the check box, lands on the check box that stands in for it.
			onCell := axTestScreenPoint(a, geom.NewPoint(30, 50))
			if got := objc.ID(v).Send(Sel("accessibilityHitTest:"), onCell); got != box {
				t.Errorf("hit test on the cell = %#x, want the check box %#x", got, box)
			}
			// A scroll area offers the scroll bar that can scroll, and nothing for the one that cannot, so that
			// VoiceOver can bring what lies past the edge into view; its contents are what it scrolls.
			vBar := scrollArea.Send(Sel("accessibilityVerticalScrollBar"))
			if vBar == 0 || vBar != a.Element(vBarID) {
				t.Errorf("vertical scroll bar = %#x, want the bar's element %#x", vBar, a.Element(vBarID))
			}
			if got := scrollArea.Send(Sel("accessibilityHorizontalScrollBar")); got != 0 {
				t.Errorf("horizontal scroll bar = %#x, want none", got)
			}
			if contents := IDsFromNSArray(scrollArea.Send(Sel("accessibilityContents"))); len(contents) != 1 ||
				contents[0] != a.Element(contentID) {
				t.Errorf("scroll area contents = %v, want just the content %#x", contents, a.Element(contentID))
			}
			// A scroll bar's value is a fraction of the range it moves in, and it carries no bounds at all: an
			// NSScroller a quarter of the way down a document answers 0.25 for AXValue and lists neither AXMinValue
			// nor AXMaxValue. Reported raw, VoiceOver would speak an offset with nothing to measure it against.
			if got := Float64FromNSNumber(vBar.Send(Sel("accessibilityValue"))); got != 0.25 {
				t.Errorf("scroll bar value = %v, want 0.25 (75 of 0..300)", got)
			}
			if got := vBar.Send(Sel("accessibilityMinValue")); got != 0 {
				t.Errorf("scroll bar min value = %v, want none at all", Float64FromNSNumber(got))
			}
			if got := vBar.Send(Sel("accessibilityMaxValue")); got != 0 {
				t.Errorf("scroll bar max value = %v, want none at all", Float64FromNSNumber(got))
			}
			// The other half of the convention: what a client sets is a fraction too, so it has to be mapped back onto
			// the range before the widget is asked to scroll. Taken literally, 0.4 would scroll to the very top.
			requests = nil
			vBar.Send(Sel("setAccessibilityValue:"), NSNumberFromFloat64(0.4))
			wantRequests = []accessibility.ActionRequest{{
				Node: vBarID, Action: accessibility.SetValue, Number: 120,
				Value: "120",
			}}
			if !slices.Equal(requests, wantRequests) {
				t.Errorf("setting the scroll bar's value asked for %v, want %v", requests, wantRequests)
			}
			// A fraction beyond either end is confined to the range rather than sent on to scroll past it.
			requests = nil
			vBar.Send(Sel("setAccessibilityValue:"), NSNumberFromFloat64(2))
			wantRequests = []accessibility.ActionRequest{{
				Node: vBarID, Action: accessibility.SetValue, Number: 300,
				Value: "300",
			}}
			if !slices.Equal(requests, wantRequests) {
				t.Errorf("setting the scroll bar past its end asked for %v, want %v", requests, wantRequests)
			}
			// Anything a client sets AXValue to that is neither a string nor a number is dropped rather than
			// messaged: sending an NSString's selectors to one of those raises inside a Go callback, which is
			// uncatchable. The same goes for the array of AXSelectedRows.
			requests = nil
			vBar.Send(Sel("setAccessibilityValue:"), Autorelease(objc.ID(Cls("NSAttributedString")).
				Send(Sel("alloc")).Send(Sel("initWithString:"), NSStringFromGo("halfway"))))
			vBar.Send(Sel("setAccessibilityValue:"), objc.ID(Cls("NSDictionary")).Send(Sel("dictionary")))
			vBar.Send(Sel("setAccessibilityValue:"), objc.ID(0))
			outline.Send(Sel("setAccessibilitySelectedRows:"), NSStringFromGo("not an array"))
			if len(requests) != 0 {
				t.Errorf("setting a value that is not a string or a number asked for %v, want nothing", requests)
			}
			// A tab panel is named by its tab, which is what AXTitleUIElement reports, and the tab points back at what
			// it controls through AXLinkedUIElements. An element with neither relationship reports neither.
			if got := panel.Send(Sel("accessibilityTitleUIElement")); got != tabs[0] {
				t.Errorf("the panel's title element = %#x, want the tab %#x", got, tabs[0])
			}
			if got := checkBox.Send(Sel("accessibilityTitleUIElement")); got != 0 {
				t.Errorf("the check box's title element = %#x, want nothing", got)
			}
			if linked := IDsFromNSArray(tabs[0].Send(Sel("accessibilityLinkedUIElements"))); len(linked) != 1 ||
				linked[0] != panel {
				t.Errorf("the tab is linked to %v, want just the panel %#x", linked, panel)
			}
			if got := NSArrayCount(checkBox.Send(Sel("accessibilityLinkedUIElements"))); got != 0 {
				t.Errorf("the check box is linked to %d elements, want 0", got)
			}
			// Expanding and collapsing by setting the state: each direction maps onto the action the node advertises,
			// and onto nothing at all when it advertises none.
			requests = nil
			rows[0].Send(Sel("setAccessibilityExpanded:"), true)
			rows[1].Send(Sel("setAccessibilityExpanded:"), false)
			rows[1].Send(Sel("setAccessibilityExpanded:"), true) // the second row advertises no Expand
			wantRequests = []accessibility.ActionRequest{
				{Node: row1ID, Action: accessibility.Expand},
				{Node: row2ID, Action: accessibility.Collapse},
			}
			if !slices.Equal(requests, wantRequests) {
				t.Errorf("setting the expanded state asked for %v, want %v", requests, wantRequests)
			}
			// Selecting one element at a time is the other half of setAccessibilitySelectedRows:, and deselecting is a
			// request of its own rather than the absence of one.
			requests = nil
			rows[0].Send(Sel("setAccessibilitySelected:"), true)
			rows[1].Send(Sel("setAccessibilitySelected:"), false)
			rows[0].Send(Sel("setAccessibilitySelected:"), false) // the first row advertises no RemoveFromSelection
			wantRequests = []accessibility.ActionRequest{
				{Node: row1ID, Action: accessibility.Select},
				{Node: row2ID, Action: accessibility.RemoveFromSelection},
			}
			if !slices.Equal(requests, wantRequests) {
				t.Errorf("setting the selected state asked for %v, want %v", requests, wantRequests)
			}
			// The two remaining performers: showing a context menu, and the return key, which for everything unison has
			// is the same thing as a press.
			requests = nil
			if !objc.Send[bool](rows[0], Sel("accessibilityPerformShowMenu")) {
				t.Error("accessibilityPerformShowMenu on the first row = false, want true")
			}
			if objc.Send[bool](rows[1], Sel("accessibilityPerformShowMenu")) {
				t.Error("accessibilityPerformShowMenu on the second row = true, want false")
			}
			if !objc.Send[bool](box, Sel("accessibilityPerformConfirm")) {
				t.Error("accessibilityPerformConfirm on the check box = false, want true")
			}
			if objc.Send[bool](tabs[0], Sel("accessibilityPerformConfirm")) {
				t.Error("accessibilityPerformConfirm on the tab = true, want false")
			}
			wantRequests = []accessibility.ActionRequest{
				{Node: row1ID, Action: accessibility.ShowContextMenu},
				{Node: boxID, Action: accessibility.Press},
			}
			if !slices.Equal(requests, wantRequests) {
				t.Errorf("the performers asked for %v, want %v", requests, wantRequests)
			}
			// A hit test landing on the separator must resolve past it, to the content view standing in for the root.
			onSeparator := axTestScreenPoint(a, geom.NewPoint(100, 140))
			if got := objc.ID(v).Send(Sel("accessibilityHitTest:"), onSeparator); got != objc.ID(v) {
				t.Errorf("hit test on the separator = %#x, want the content view %#x", got, objc.ID(v))
			}
		})
	})
}

// TestAXSpinButtonRole proves a spin button is presented with the stepper semantics it advertises. A NumericField
// advertises Increment and Decrement and this element implements both, but told the element is a plain text field
// VoiceOver describes it as one and never mentions that its value can be stepped. The incrementor role is what carries
// that on this platform, and is the counterpart of the spinner control type Windows reports and the spin button role
// AT-SPI reports; the text protocol still answers, so nothing about reading the content is given up for it.
func TestAXSpinButtonRole(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 200 + iota
			spinID
		)
		tree := &accessibility.Tree{
			Nodes: map[accessibility.NodeID]*accessibility.Node{
				rootID: {
					ID: rootID, Children: []accessibility.NodeID{spinID}, Role: role.Window,
					Bounds: geom.NewRect(0, 0, 320, 240),
				},
				spinID: {
					ID: spinID, Parent: rootID, Role: role.SpinButton, Name: "Count",
					Bounds: geom.NewRect(10, 10, 80, 24), HasNumber: true, Number: 3, Min: 1, Max: 9, Step: 1,
					Text:    &accessibility.TextInfo{Text: "3", SelStart: 1, SelEnd: 1, Caret: 1},
					Actions: accessibility.ActionSet(0).With(accessibility.Increment, accessibility.Decrement),
				},
			},
			Root:       rootID,
			Generation: 1,
		}
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			spin := axTestChildren(t, v, 1)[0]
			if got := GoStringFromNSString(spin.Send(Sel("accessibilityRole"))); got != "AXIncrementor" {
				t.Errorf("spin button role = %q, want AXIncrementor", got)
			}
			// The role description is what an assistive technology actually says, so it must no longer be the one a
			// text field gets.
			textFieldDescription := GoStringFromNSString(axRoleDescriptionFor(AppKitString(axRoleTextField), 0))
			if described := GoStringFromNSString(spin.Send(Sel("accessibilityRoleDescription"))); described == "" ||
				described == textFieldDescription {
				t.Errorf("spin button role description = %q, want the platform's own stepper description", described)
			}
			if got := GoStringFromNSString(spin.Send(Sel("accessibilityLabel"))); got != "Count" {
				t.Errorf("spin button label = %q, want Count", got)
			}
			if got := objc.Send[int64](spin, Sel("accessibilityNumberOfCharacters")); got != 1 {
				t.Errorf("spin button character count = %d, want 1", got)
			}
			if got := Float64FromNSNumber(spin.Send(Sel("accessibilityMaxValue"))); got != 9 {
				t.Errorf("spin button max = %v, want 9", got)
			}
		})
	})
}

// TestAXTableCounts proves a container reports how many rows and columns it holds rather than how many it happened to
// describe. Only the rows that can be seen, plus the ones that are selected, are described, so counting the row
// elements would tell an assistive technology a table is as tall as its view port while accessibilityIndex goes on
// reporting the absolute row number — "row 4,101 of 2". A container that reported no counts falls back to what can be
// counted, which is all a container of that kind has.
func TestAXTableCounts(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 300 + iota
			tableID
			rowAID
			rowBID
			cellA1ID
			cellA2ID
			gridID
			gridRowID
			gridCellID
		)
		tree := &accessibility.Tree{
			Nodes: map[accessibility.NodeID]*accessibility.Node{
				rootID: {
					ID: rootID, Children: []accessibility.NodeID{tableID, gridID}, Role: role.Window,
					Bounds: geom.NewRect(0, 0, 320, 240),
				},
				// A table of ten thousand rows showing two of them, which is what a scrolled table looks like.
				tableID: {
					ID: tableID, Parent: rootID, Children: []accessibility.NodeID{rowAID, rowBID}, Role: role.Table,
					Name: "Big", Bounds: geom.NewRect(0, 0, 200, 40), RowCount: 10000, ColumnCount: 3,
				},
				rowAID: {
					ID: rowAID, Parent: tableID, Children: []accessibility.NodeID{cellA1ID, cellA2ID}, Role: role.Row,
					Bounds: geom.NewRect(0, 0, 200, 20), RowIndex: 4100, Level: 1,
				},
				cellA1ID: {
					ID: cellA1ID, Parent: rowAID, Role: role.Cell, Bounds: geom.NewRect(0, 0, 100, 20),
					RowIndex: 4100, ColumnIndex: 0,
				},
				cellA2ID: {
					ID: cellA2ID, Parent: rowAID, Role: role.Cell, Bounds: geom.NewRect(100, 0, 100, 20),
					RowIndex: 4100, ColumnIndex: 1,
				},
				rowBID: {
					ID: rowBID, Parent: tableID, Role: role.Row, Bounds: geom.NewRect(0, 20, 200, 20),
					RowIndex: 4101, Level: 1,
				},
				// A container that reported neither count: both are counted from what it holds.
				gridID: {
					ID: gridID, Parent: rootID, Children: []accessibility.NodeID{gridRowID}, Role: role.Table,
					Name: "Small", Bounds: geom.NewRect(0, 40, 200, 20),
				},
				gridRowID: {
					ID: gridRowID, Parent: gridID, Children: []accessibility.NodeID{gridCellID}, Role: role.Row,
					Bounds: geom.NewRect(0, 40, 200, 20), RowIndex: 0, Level: 1,
				},
				gridCellID: {
					ID: gridCellID, Parent: gridRowID, Role: role.Cell, Name: "Only",
					Bounds: geom.NewRect(0, 40, 200, 20), RowIndex: 0, ColumnIndex: 0,
				},
			},
			Root:       rootID,
			Generation: 1,
		}
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			children := axTestChildren(t, v, 2)
			table, grid := children[0], children[1]
			if got := objc.Send[int64](table, Sel("accessibilityRowCount")); got != 10000 {
				t.Errorf("table row count = %d, want 10000", got)
			}
			if got := objc.Send[int64](table, Sel("accessibilityColumnCount")); got != 3 {
				t.Errorf("table column count = %d, want 3", got)
			}
			// The described subset is unchanged: the count is the whole point, since the rows are not all there.
			if rows := IDsFromNSArray(table.Send(Sel("accessibilityRows"))); len(rows) != 2 {
				t.Errorf("table reported %d rows, want the 2 that are described", len(rows))
			}
			// The index and the count have to be measured against the same thing for "row N of M" to make sense.
			if got := objc.Send[int64](a.Element(rowAID), Sel("accessibilityIndex")); got != 4100 {
				t.Errorf("first described row index = %d, want 4100", got)
			}
			if got := objc.Send[int64](grid, Sel("accessibilityRowCount")); got != 1 {
				t.Errorf("uncounted container row count = %d, want the 1 row it holds", got)
			}
			if got := objc.Send[int64](grid, Sel("accessibilityColumnCount")); got != 1 {
				t.Errorf("uncounted container column count = %d, want the 1 cell its row holds", got)
			}
			// Something that is not a container holds no rows and no columns.
			if got := objc.Send[int64](a.Element(cellA1ID), Sel("accessibilityRowCount")); got != 0 {
				t.Errorf("cell row count = %d, want 0", got)
			}
		})
	})
}

// TestAXExpandedNotificationIsRowOnly proves only a row sends the outline-row expansion notifications. macOS has
// nothing but NSAccessibilityRowExpanded/RowCollapsedNotification for a state change of this kind, and they say "an
// outline row opened": a disclosure triangle, a pop-up button or a combo box opening must not claim to be one, the way
// every other row-oriented path in the adapter already refuses to.
func TestAXExpandedNotificationIsRowOnly(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 400 + iota
			outlineID
			rowID
			popupID
			discID
		)
		build := func(expanded bool) *accessibility.Tree {
			return &accessibility.Tree{
				Nodes: map[accessibility.NodeID]*accessibility.Node{
					rootID: {
						ID: rootID, Children: []accessibility.NodeID{outlineID, popupID, discID}, Role: role.Window,
						Bounds: geom.NewRect(0, 0, 320, 240),
					},
					outlineID: {
						ID: outlineID, Parent: rootID, Children: []accessibility.NodeID{rowID}, Role: role.Tree,
						Name: "Items", Bounds: geom.NewRect(0, 0, 200, 20), RowCount: 1,
					},
					rowID: {
						ID: rowID, Parent: outlineID, Role: role.Row, Name: "One",
						Bounds: geom.NewRect(0, 0, 200, 20), RowIndex: 0, Level: 1, Expandable: true,
						Expanded: expanded,
					},
					popupID: {
						ID: popupID, Parent: rootID, Role: role.PopupButton, Name: "Choose",
						Bounds: geom.NewRect(0, 20, 100, 20), Expandable: true, Expanded: expanded,
					},
					discID: {
						ID: discID, Parent: rootID, Role: role.DisclosureTriangle,
						Bounds: geom.NewRect(0, 40, 16, 16), Expandable: true, Expanded: expanded,
					},
				},
				Root:       rootID,
				Generation: 1,
			}
		}
		closed := build(false)
		v, a, cleanup := newAXAdapterWithTree(t, closed)
		defer cleanup()
		if a == nil {
			return
		}
		// Nothing is posted about an element nothing has asked about, so every element has to exist first.
		WithPool(func() {
			for _, child := range axTestChildren(t, v, 3) {
				child.Send(Sel("accessibilityChildren"))
			}
		})
		if a.Element(rowID) == 0 {
			t.Fatal("the row's element was never created")
		}
		ensureAXNotifyFuncs()
		realPost := axPostNotify
		defer func() { axPostNotify = realPost }()
		type posted struct {
			name    string
			element objc.ID
		}
		var notifications []posted
		axPostNotify = func(element, notification objc.ID) {
			notifications = append(notifications, posted{element: element, name: GoStringFromNSString(notification)})
			realPost(element, notification)
		}
		opened := build(true)
		opened.Generation = 2
		a.Publish(opened, accessibility.Diff(closed, opened))
		wantExpanded := GoStringFromNSString(AppKitString(axNotifyRowExpanded))
		wantCollapsed := GoStringFromNSString(AppKitString(axNotifyRowCollapsed))
		var expandedOn []objc.ID
		for _, p := range notifications {
			if p.name == wantExpanded || p.name == wantCollapsed {
				expandedOn = append(expandedOn, p.element)
			}
		}
		if len(expandedOn) != 1 || expandedOn[0] != a.Element(rowID) {
			t.Errorf("row expansion was reported on %v, want just the row %#x", expandedOn, a.Element(rowID))
		}
		// Closing everything again is the same story from the other direction.
		notifications = nil
		reclosed := build(false)
		reclosed.Generation = 3
		a.Publish(reclosed, accessibility.Diff(opened, reclosed))
		expandedOn = nil
		for _, p := range notifications {
			if p.name == wantExpanded || p.name == wantCollapsed {
				expandedOn = append(expandedOn, p.element)
			}
		}
		if len(expandedOn) != 1 || expandedOn[0] != a.Element(rowID) {
			t.Errorf("row collapse was reported on %v, want just the row %#x", expandedOn, a.Element(rowID))
		}
	})
}

// TestAXRowWithoutParentNode proves the row helpers tolerate a row whose parent is not in the snapshot. Both
// axDisclosedRowsOf and axDisclosingRowOf look their row's siblings up through the unignored parent, which is zero for
// a row that is the root or whose ancestor chain holds nothing the snapshot knows; asking the snapshot for node zero
// yields nil, and this runs inside an AppKit accessibility callback, where a nil dereference takes the process with it.
func TestAXRowWithoutParentNode(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 500 + iota
			orphanID
		)
		tree := &accessibility.Tree{
			Nodes: map[accessibility.NodeID]*accessibility.Node{
				rootID: {
					ID: rootID, Children: []accessibility.NodeID{orphanID}, Role: role.Window,
					Bounds: geom.NewRect(0, 0, 320, 240),
				},
				// A row the snapshot reaches as a child of the root, but which names no parent of its own.
				orphanID: {
					ID: orphanID, Role: role.Row, Name: "Stray", Bounds: geom.NewRect(0, 0, 200, 20),
					RowIndex: 0, Level: 2, Expandable: true, Expanded: true,
				},
			},
			Root:       rootID,
			Generation: 1,
		}
		if got := tree.UnignoredParent(orphanID); got != 0 {
			t.Fatalf("the orphaned row's unignored parent = %d, want 0 for this test to mean anything", got)
		}
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			orphan := axTestChildren(t, v, 1)[0]
			if got := NSArrayCount(orphan.Send(Sel("accessibilityDisclosedRows"))); got != 0 {
				t.Errorf("the orphaned row discloses %d rows, want 0", got)
			}
			if got := orphan.Send(Sel("accessibilityDisclosedByRow")); got != 0 {
				t.Errorf("the orphaned row is disclosed by %#x, want nothing", got)
			}
			if got := NSArrayCount(orphan.Send(Sel("accessibilityRows"))); got != 0 {
				t.Errorf("the orphaned row reported %d rows of its own, want 0", got)
			}
		})
	})
}

// TestAXInlineActionPublishDefersElementRelease proves the adapter survives being re-entered by the request it is
// carrying out. The macOS binding runs the requests an assistive technology reads the result of immediately —
// focusing, selecting, scrolling — which lays the window out and publishes a new snapshot before the adapter has
// returned to AppKit, so Publish, postEvents and destroyElement all run inside the accessibility callback. The element
// whose node that publish drops is the element AppKit is calling the method on: releasing it there would free it under
// AppKit's feet, so it is held until the request is done and then autoreleased, which leaves it alive until the run
// loop's own pool is drained.
func TestAXInlineActionPublishDefersElementRelease(t *testing.T) {
	defer func() { AccessibilityActionCallback = nil }()
	runOnMain(func() {
		v, a, tree, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			button := axTestChildren(t, v, 3)[0]
			if button != a.Element(axTestButton) {
				t.Fatalf("the first child %#x is not the button's element %#x", button, a.Element(axTestButton))
			}
			// The button goes away as a result of being pressed, which is what a button that dismisses its own panel
			// does, and the new snapshot is published from inside the callback the way performAccessibilityAction does.
			next := newAXTestTree()
			next.Generation = 2
			delete(next.Nodes, axTestButton)
			next.Nodes[axTestRoot].Children = []accessibility.NodeID{axTestField, axTestGroup}
			var inflight int
			var deferredDuring []objc.ID
			AccessibilityActionCallback = func(_ Window, _ accessibility.ActionRequest) {
				a.Publish(next, accessibility.Diff(tree, next))
				inflight = a.inflight
				deferredDuring = slices.Clone(a.deferred)
			}
			if !objc.Send[bool](button, Sel("accessibilityPerformPress")) {
				t.Fatal("accessibilityPerformPress on the button = false, want true")
			}
			if inflight != 1 {
				t.Errorf("the adapter counted %d requests in flight while carrying one out, want 1", inflight)
			}
			if len(deferredDuring) != 1 || deferredDuring[0] != button {
				t.Errorf("the element dropped mid-request was %v, want it held back as just the button %#x",
					deferredDuring, button)
			}
			if len(a.deferred) != 0 {
				t.Errorf("%d elements were still held back after the request finished, want 0", len(a.deferred))
			}
			if a.inflight != 0 {
				t.Errorf("the adapter still counts %d requests in flight, want 0", a.inflight)
			}
			if a.Element(axTestButton) != 0 {
				t.Error("the adapter still hands out an element for the node that left the tree")
			}
			// The element AppKit was standing on is still answerable: it outlives the request and reports, safely, that
			// it no longer speaks for anything.
			if objc.Send[bool](button, Sel("isAccessibilityElement")) {
				t.Error("the released element still reports itself as an accessibility element")
			}
			if got := button.Send(Sel("accessibilityParent")); got != 0 {
				t.Errorf("the released element's parent = %#x, want nothing", got)
			}
		})
	})
}

// TestAXTableColumnHeaders proves a table hands an assistive technology the header describing its columns, and a cell
// the header of the column it sits in, which is what VoiceOver speaks alongside the cell's contents. A unison table and
// its header are separate panels — the header goes into the column-header slot of the scroll panel whose content is the
// table — so neither is inside the other and the snapshot records no link between them: the header has to be searched
// for, by the explicit link a widget recorded or by proximity, exactly as the Windows adapter searches for it.
func TestAXTableColumnHeaders(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 700 + iota
			scrollID
			headerID
			col0ID
			col1ID
			tableID
			rowID
			cell0ID
			cell1ID
			labelID
			strayHeaderID
			strayColID
			strayTableID
			lonelyTableID
		)
		tree := &accessibility.Tree{
			Nodes: map[accessibility.NodeID]*accessibility.Node{
				rootID: {
					ID:       rootID,
					Children: []accessibility.NodeID{scrollID, strayHeaderID, strayTableID, lonelyTableID},
					Role:     role.Window,
					Bounds:   geom.NewRect(0, 0, 320, 240),
				},
				// The scroll panel, holding the header above and the table below, neither inside the other.
				scrollID: {
					ID: scrollID, Parent: rootID, Children: []accessibility.NodeID{headerID, tableID},
					Role: role.Group, Bounds: geom.NewRect(0, 0, 200, 60),
				},
				headerID: {
					ID: headerID, Parent: scrollID, Children: []accessibility.NodeID{col0ID, col1ID},
					Role: role.TableHeader, Bounds: geom.NewRect(0, 0, 200, 20), ColumnCount: 2,
				},
				col0ID: {
					ID: col0ID, Parent: headerID, Role: role.ColumnHeader, Name: "Name",
					Bounds: geom.NewRect(0, 0, 100, 20), ColumnIndex: 0,
				},
				col1ID: {
					ID: col1ID, Parent: headerID, Role: role.ColumnHeader, Name: "Size",
					Bounds: geom.NewRect(100, 0, 100, 20), ColumnIndex: 1,
				},
				tableID: {
					ID: tableID, Parent: scrollID, Children: []accessibility.NodeID{rowID}, Role: role.Table,
					Name: "Files", Bounds: geom.NewRect(0, 20, 200, 40), RowCount: 1, ColumnCount: 2,
				},
				rowID: {
					ID: rowID, Parent: tableID, Children: []accessibility.NodeID{cell0ID, cell1ID}, Role: role.Row,
					Bounds: geom.NewRect(0, 20, 200, 20), RowIndex: 0, Level: 1,
				},
				cell0ID: {
					ID: cell0ID, Parent: rowID, Role: role.Cell, Name: "notes.txt",
					Bounds: geom.NewRect(0, 20, 100, 20), RowIndex: 0, ColumnIndex: 0, Selectable: true, Selected: true,
				},
				// A cell holding one thing is presented as that thing, which has to answer for the cell's column too.
				cell1ID: {
					ID: cell1ID, Parent: rowID, Children: []accessibility.NodeID{labelID}, Role: role.Cell,
					Bounds: geom.NewRect(100, 20, 100, 20), RowIndex: 0, ColumnIndex: 1,
				},
				labelID: {
					ID: labelID, Parent: cell1ID, Role: role.Label, Name: "12 KB",
					Bounds: geom.NewRect(100, 20, 100, 20),
				},
				// A header that says which table it describes, for a table it sits nowhere near.
				strayHeaderID: {
					ID: strayHeaderID, Parent: rootID, Children: []accessibility.NodeID{strayColID},
					Role: role.TableHeader, Bounds: geom.NewRect(0, 60, 200, 20),
					Controls: []accessibility.NodeID{strayTableID},
				},
				strayColID: {
					ID: strayColID, Parent: strayHeaderID, Role: role.ColumnHeader, Name: "Only",
					Bounds: geom.NewRect(0, 60, 200, 20), ColumnIndex: 0,
				},
				strayTableID: {
					ID: strayTableID, Parent: rootID, Role: role.Table, Name: "Elsewhere",
					Bounds: geom.NewRect(0, 80, 200, 20),
				},
				// A table with neither a link nor an ancestor holding it alone: the window holds three tables, so there
				// is nothing to pair it with that is not a guess.
				lonelyTableID: {
					ID: lonelyTableID, Parent: rootID, Role: role.Table, Name: "Nobody's",
					Bounds: geom.NewRect(0, 100, 200, 20),
				},
			},
			Root:       rootID,
			Generation: 1,
		}
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			children := axTestChildren(t, v, 4)
			scrollChildren := IDsFromNSArray(children[0].Send(Sel("accessibilityChildren")))
			if len(scrollChildren) != 2 {
				t.Fatalf("the scroll panel has %d children, want 2", len(scrollChildren))
			}
			header, table := scrollChildren[0], scrollChildren[1]
			if got := table.Send(Sel("accessibilityHeader")); got != header {
				t.Errorf("the table's header = %#x, want the header group %#x", got, header)
			}
			columnHeaders := IDsFromNSArray(table.Send(Sel("accessibilityColumnHeaderUIElements")))
			if len(columnHeaders) != 2 || columnHeaders[0] != a.Element(col0ID) ||
				columnHeaders[1] != a.Element(col1ID) {
				t.Errorf("the table's column headers = %v, want %#x and %#x", columnHeaders, a.Element(col0ID),
					a.Element(col1ID))
			}
			rows := IDsFromNSArray(table.Send(Sel("accessibilityRows")))
			if len(rows) != 1 {
				t.Fatalf("the table reported %d rows, want 1", len(rows))
			}
			cells := IDsFromNSArray(rows[0].Send(Sel("accessibilityChildren")))
			if len(cells) != 2 {
				t.Fatalf("the row has %d children, want 2", len(cells))
			}
			if cells[1] != a.Element(labelID) {
				t.Errorf("the second cell is presented as %#x, want the label %#x standing in for it", cells[1],
					a.Element(labelID))
			}
			// Each cell names the header of its own column, the label standing in for the second cell included.
			for i, c := range []struct {
				want    accessibility.NodeID
				element objc.ID
			}{
				{element: cells[0], want: col0ID},
				{element: cells[1], want: col1ID},
			} {
				got := IDsFromNSArray(c.element.Send(Sel("accessibilityColumnHeaderUIElements")))
				if len(got) != 1 || got[0] != a.Element(c.want) {
					t.Errorf("cell %d named %v as its column header, want just %#x", i, got, a.Element(c.want))
				}
			}
			if selected := IDsFromNSArray(table.Send(Sel("accessibilitySelectedCells"))); len(selected) != 1 ||
				selected[0] != cells[0] {
				t.Errorf("the table's selected cells = %v, want just the first cell %#x", selected, cells[0])
			}
			// A header that says which table it describes is found even though it is nowhere near it, and a table with
			// neither a link nor an ancestor of its own is left without one rather than paired with a guess.
			if got := children[2].Send(Sel("accessibilityHeader")); got != a.Element(strayHeaderID) {
				t.Errorf("the stray table's header = %#x, want the header naming it %#x", got,
					a.Element(strayHeaderID))
			}
			if got := children[3].Send(Sel("accessibilityHeader")); got != 0 {
				t.Errorf("the unpaired table's header = %#x, want nothing", got)
			}
			// Something that is not a table has neither a header nor columns of its own.
			if got := header.Send(Sel("accessibilityHeader")); got != 0 {
				t.Errorf("the header group's own header = %#x, want nothing", got)
			}
			if got := NSArrayCount(children[0].Send(Sel("accessibilityColumnHeaderUIElements"))); got != 0 {
				t.Errorf("the scroll panel named %d column headers, want 0", got)
			}
		})
	})
}

// TestAXSelectionNotifications proves every kind of selection change is reported rather than only a row's. A tab
// reports whether it is selected as its value, a cell's selection belongs to the table holding it, and anything else
// selectable belongs to its container: before this, selecting a tab or a cell posted nothing at all, leaving an
// assistive technology with the state from before the change until something else made it ask again.
func TestAXSelectionNotifications(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 800 + iota
			tabListID
			tabAID
			tabBID
			tableID
			rowID
			cellAID
			cellBID
			listID
			itemID
			menuID
			menuItemID
		)
		// moved reports the tree after the selection has moved on in every container at once.
		build := func(moved bool) *accessibility.Tree {
			return &accessibility.Tree{
				Nodes: map[accessibility.NodeID]*accessibility.Node{
					rootID: {
						ID: rootID, Children: []accessibility.NodeID{tabListID, tableID, listID, menuID},
						Role: role.Window, Bounds: geom.NewRect(0, 0, 320, 240),
					},
					tabListID: {
						ID: tabListID, Parent: rootID, Children: []accessibility.NodeID{tabAID, tabBID},
						Role: role.TabList, Bounds: geom.NewRect(0, 0, 200, 20),
					},
					tabAID: {
						ID: tabAID, Parent: tabListID, Role: role.Tab, Name: "First",
						Bounds: geom.NewRect(0, 0, 100, 20), Selectable: true, Selected: !moved,
					},
					tabBID: {
						ID: tabBID, Parent: tabListID, Role: role.Tab, Name: "Second",
						Bounds: geom.NewRect(100, 0, 100, 20), Selectable: true, Selected: moved,
					},
					tableID: {
						ID: tableID, Parent: rootID, Children: []accessibility.NodeID{rowID}, Role: role.Table,
						Name: "Grid", Bounds: geom.NewRect(0, 20, 200, 20), RowCount: 1, ColumnCount: 2,
					},
					rowID: {
						ID: rowID, Parent: tableID, Children: []accessibility.NodeID{cellAID, cellBID}, Role: role.Row,
						Bounds: geom.NewRect(0, 20, 200, 20), RowIndex: 0, Level: 1,
					},
					// Both cells change together, which has to produce one notification rather than two.
					cellAID: {
						ID: cellAID, Parent: rowID, Role: role.Cell, Name: "Left",
						Bounds: geom.NewRect(0, 20, 100, 20), RowIndex: 0, ColumnIndex: 0, Selectable: true,
						Selected: moved,
					},
					cellBID: {
						ID: cellBID, Parent: rowID, Role: role.Cell, Name: "Right",
						Bounds: geom.NewRect(100, 20, 100, 20), RowIndex: 0, ColumnIndex: 1, Selectable: true,
						Selected: moved,
					},
					listID: {
						ID: listID, Parent: rootID, Children: []accessibility.NodeID{itemID}, Role: role.List,
						Name: "Items", Bounds: geom.NewRect(0, 40, 200, 20), RowCount: 1,
					},
					itemID: {
						ID: itemID, Parent: listID, Role: role.ListItem, Name: "Only",
						Bounds: geom.NewRect(0, 40, 200, 20), RowIndex: 0, Level: 1, Selectable: true, Selected: moved,
					},
					menuID: {
						ID: menuID, Parent: rootID, Children: []accessibility.NodeID{menuItemID}, Role: role.Menu,
						Name: "File", Bounds: geom.NewRect(0, 60, 200, 20),
					},
					menuItemID: {
						ID: menuItemID, Parent: menuID, Role: role.MenuItem, Name: "Open",
						Bounds: geom.NewRect(0, 60, 200, 20), Selectable: true, Selected: moved,
					},
				},
				Root:       rootID,
				Generation: 1,
			}
		}
		before := build(false)
		v, a, cleanup := newAXAdapterWithTree(t, before)
		defer cleanup()
		if a == nil {
			return
		}
		// Nothing is posted about an element nothing has asked about, so every element has to exist first.
		WithPool(func() {
			for _, child := range axTestChildren(t, v, 4) {
				for _, grandchild := range IDsFromNSArray(child.Send(Sel("accessibilityChildren"))) {
					grandchild.Send(Sel("accessibilityChildren"))
				}
			}
		})
		ensureAXNotifyFuncs()
		realPost := axPostNotify
		defer func() { axPostNotify = realPost }()
		type posted struct {
			name    string
			element objc.ID
		}
		var notifications []posted
		axPostNotify = func(element, notification objc.ID) {
			notifications = append(notifications, posted{element: element, name: GoStringFromNSString(notification)})
			realPost(element, notification)
		}
		after := build(true)
		after.Generation = 2
		a.Publish(after, accessibility.Diff(before, after))
		want := []posted{
			// A tab's value is whether it is selected, so both the tab that lost the selection and the one that gained
			// it report a changed value.
			{element: a.Element(tabAID), name: GoStringFromNSString(AppKitString(axNotifyValueChanged))},
			{element: a.Element(tabBID), name: GoStringFromNSString(AppKitString(axNotifyValueChanged))},
			// Two cells, one notification, against the table rather than the row.
			{element: a.Element(tableID), name: GoStringFromNSString(AppKitString(axNotifySelectedCellsChanged))},
			{element: a.Element(listID), name: GoStringFromNSString(AppKitString(axNotifySelectedRowsChanged))},
			{element: a.Element(menuID), name: GoStringFromNSString(AppKitString(axNotifySelectedChildrenChanged))},
		}
		if !slices.Equal(notifications, want) {
			t.Errorf("the selection change posted %v, want %v", notifications, want)
		}
		// The selected-children notification tells an assistive technology to read the container's selection again, so
		// there has to be something there for it to read: an element that answers nothing for AXSelectedChildren leaves
		// it with the notification and no way to act on it.
		WithPool(func() {
			selected := IDsFromNSArray(a.Element(menuID).Send(Sel("accessibilitySelectedChildren")))
			if len(selected) != 1 || selected[0] != a.Element(menuItemID) {
				t.Errorf("the menu's selected children = %v, want just the menu item %#x", selected,
					a.Element(menuItemID))
			}
			selected = IDsFromNSArray(a.Element(tabListID).Send(Sel("accessibilitySelectedChildren")))
			if len(selected) != 1 || selected[0] != a.Element(tabBID) {
				t.Errorf("the tab group's selected children = %v, want just the second tab %#x", selected,
					a.Element(tabBID))
			}
			// A container holding nothing selected answers an empty array rather than nothing at all, which is what
			// AppKit takes for "ask my superclass".
			if got := NSArrayCount(a.Element(rowID).Send(Sel("accessibilitySelectedChildren"))); got != 2 {
				t.Errorf("the row reported %d selected children, want its two cells", got)
			}
		})
	})
}

// TestAXShutdownDuringInflightRequest proves the adapter survives being shut down by the request it is carrying out,
// which is what a press that closes its own window does. The macOS binding runs the requests an assistive technology
// reads the result of immediately, so Shutdown runs inside the AppKit accessibility callback the press arrived
// through, and the element AppKit is standing on is one of the ones it lets go of. It is the only path that reaches
// releaseElement from Shutdown rather than from destroyElement.
func TestAXShutdownDuringInflightRequest(t *testing.T) {
	defer func() { AccessibilityActionCallback = nil }()
	runOnMain(func() {
		v, a, _, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			button := axTestChildren(t, v, 3)[0]
			if button != a.Element(axTestButton) {
				t.Fatalf("the first child %#x is not the button's element %#x", button, a.Element(axTestButton))
			}
			var inflight int
			var adaptedDuring bool
			var deferredDuring []objc.ID
			AccessibilityActionCallback = func(_ Window, _ accessibility.ActionRequest) {
				a.Shutdown()
				inflight = a.inflight
				adaptedDuring = axViewIsAdapted(v)
				deferredDuring = slices.Clone(a.deferred)
			}
			if !objc.Send[bool](button, Sel("accessibilityPerformPress")) {
				t.Fatal("accessibilityPerformPress on the button = false, want true")
			}
			if inflight != 1 {
				t.Errorf("the adapter counted %d requests in flight while carrying one out, want 1", inflight)
			}
			if adaptedDuring {
				t.Error("the content view still had an adapter once Shutdown had returned")
			}
			if len(deferredDuring) != 3 {
				t.Errorf("Shutdown held back %d elements while the request was in flight, want all 3",
					len(deferredDuring))
			}
			if !slices.Contains(deferredDuring, button) {
				t.Errorf("the element AppKit was standing on, %#x, was not among the %v held back", button,
					deferredDuring)
			}
			if len(a.deferred) != 0 {
				t.Errorf("%d elements were still held back after the request finished, want 0", len(a.deferred))
			}
			if a.inflight != 0 {
				t.Errorf("the adapter still counts %d requests in flight, want 0", a.inflight)
			}
			// The element AppKit was standing on outlives the request: it was autoreleased into the enclosing pool
			// rather than freed under AppKit's feet, and reports, safely, that it speaks for nothing.
			if objc.Send[bool](button, Sel("isAccessibilityElement")) {
				t.Error("the released element still reports itself as an accessibility element")
			}
			if got := button.Send(Sel("accessibilityParent")); got != 0 {
				t.Errorf("the released element's parent = %#x, want nothing", got)
			}
			if got := a.Element(axTestButton); got != 0 {
				t.Errorf("the adapter still hands out %#x for a node it has let go of", got)
			}
		})
	})
}

// TestAXValueChangedCoalescing proves one edit produces one value-changed notification. A field fills both its node's
// Value and its Text from the same string, so a single keystroke publishes a changed value, an inserted run of text
// and — in a numeric field — a changed number as well, and a notification apiece would have VoiceOver speak the new
// contents three times over.
func TestAXValueChangedCoalescing(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 1000 + iota
			fieldID
		)
		build := func(text string, number float64) *accessibility.Tree {
			return &accessibility.Tree{
				Nodes: map[accessibility.NodeID]*accessibility.Node{
					rootID: {
						ID: rootID, Children: []accessibility.NodeID{fieldID}, Role: role.Window,
						Bounds: geom.NewRect(0, 0, 320, 240),
					},
					fieldID: {
						ID: fieldID, Parent: rootID, Role: role.SpinButton, Name: "Count", Value: text,
						Bounds: geom.NewRect(10, 10, 80, 24), HasNumber: true, Number: number, Max: 99, Step: 1,
						Text: &accessibility.TextInfo{
							Text:     text,
							SelStart: len(text),
							SelEnd:   len(text),
							Caret:    len(text),
						},
					},
				},
				Root:       rootID,
				Generation: 1,
			}
		}
		before := build("4", 4)
		v, a, cleanup := newAXAdapterWithTree(t, before)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() { axTestChildren(t, v, 1) })
		field := a.Element(fieldID)
		if field == 0 {
			t.Fatal("the field's element was never created")
		}
		after := build("45", 45)
		after.Generation = 2
		// The edit also brings the focus to the field, which is how the order the notifications go out in is asserted
		// below: an assistive technology is told where to look only after it has been told what changed.
		after.Focus = fieldID
		after.Nodes[fieldID].Focused = true
		events := accessibility.Diff(before, after)
		// The one edit really does produce the several events this is about.
		kinds := make(map[accessibility.EventKind]int)
		for _, e := range events {
			kinds[e.Kind]++
		}
		for _, kind := range []accessibility.EventKind{
			accessibility.ValueChanged,
			accessibility.NumberChanged,
			accessibility.TextInserted,
			accessibility.TextSelectionChanged,
		} {
			if kinds[kind] != 1 {
				t.Errorf("the edit produced %d %v events, want 1", kinds[kind], kind)
			}
		}
		ensureAXNotifyFuncs()
		realPost := axPostNotify
		defer func() { axPostNotify = realPost }()
		var notifications []string
		axPostNotify = func(element, notification objc.ID) {
			if element == field {
				notifications = append(notifications, GoStringFromNSString(notification))
			}
			realPost(element, notification)
		}
		a.Publish(after, events)
		want := []string{
			GoStringFromNSString(AppKitString(axNotifyValueChanged)),
			GoStringFromNSString(AppKitString(axNotifySelectedTextChanged)),
			GoStringFromNSString(AppKitString(axNotifyFocusedUIElement)),
		}
		if !slices.Equal(notifications, want) {
			t.Errorf("the edit posted %v, want %v (one value-changed, and the focus last of all)", notifications, want)
		}
	})
}

// TestAXIgnoredChangeReportsLayoutChange proves a node that joins the presented tree by having its Ignored flag
// cleared is reported. A scroll bar is ignored while there is nothing to scroll, so a scroll panel whose content grows
// past its view port gains one in a publish that changes nothing else about the panel: without a layout-changed
// notification naming the scroll area, VoiceOver goes on holding the child list from before the bar appeared, and
// never asks for the bar it now needs to bring anything into view.
func TestAXIgnoredChangeReportsLayoutChange(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 1100 + iota
			scrollID
			barID
			contentID
		)
		build := func(maximum float64) *accessibility.Tree {
			return &accessibility.Tree{
				Nodes: map[accessibility.NodeID]*accessibility.Node{
					rootID: {
						ID: rootID, Children: []accessibility.NodeID{scrollID}, Role: role.Window,
						Bounds: geom.NewRect(0, 0, 320, 240),
					},
					scrollID: {
						ID: scrollID, Parent: rootID, Children: []accessibility.NodeID{barID, contentID},
						Role: role.ScrollArea, Bounds: geom.NewRect(0, 0, 200, 80),
					},
					barID: {
						ID: barID, Parent: scrollID, Role: role.ScrollBar, Bounds: geom.NewRect(190, 0, 10, 80),
						Orientation: accessibility.OrientationVertical, HasNumber: true, Max: maximum, Step: 8,
						Ignored: maximum == 0,
					},
					contentID: {
						ID: contentID, Parent: scrollID, Role: role.Group, Name: "Long form",
						Bounds: geom.NewRect(0, 0, 190, 80),
					},
				},
				Root:       rootID,
				Generation: 1,
			}
		}
		before := build(0)
		v, a, cleanup := newAXAdapterWithTree(t, before)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			scrollArea := axTestChildren(t, v, 1)[0]
			if got := NSArrayCount(scrollArea.Send(Sel("accessibilityChildren"))); got != 1 {
				t.Errorf("the scroll area presented %d children while the bar was ignored, want 1", got)
			}
			if got := scrollArea.Send(Sel("accessibilityVerticalScrollBar")); got != 0 {
				t.Errorf("the scroll area offered the scroll bar %#x while there was nothing to scroll", got)
			}
		})
		ensureAXNotifyFuncs()
		realPostInfo := axPostNotifyInfo
		defer func() { axPostNotifyInfo = realPostInfo }()
		var named [][]objc.ID
		wantName := GoStringFromNSString(AppKitString(axNotifyLayoutChanged))
		axPostNotifyInfo = func(element, notification, userInfo objc.ID) {
			if element == objc.ID(v) && GoStringFromNSString(notification) == wantName {
				named = append(named, IDsFromNSArray(userInfo.Send(Sel("objectForKey:"),
					AppKitString(axKeyUIElements))))
			}
			realPostInfo(element, notification, userInfo)
		}
		after := build(300)
		after.Generation = 2
		a.Publish(after, accessibility.Diff(before, after))
		if len(named) != 1 {
			t.Fatalf("%d layout-changed notifications were posted for the scroll bar appearing, want 1", len(named))
		}
		if len(named[0]) != 1 || named[0][0] != a.Element(scrollID) {
			t.Errorf("the layout-changed notification named %v, want just the scroll area %#x", named[0],
				a.Element(scrollID))
		}
		WithPool(func() {
			scrollArea := a.Element(scrollID)
			if got := NSArrayCount(scrollArea.Send(Sel("accessibilityChildren"))); got != 2 {
				t.Errorf("the scroll area presented %d children once the bar could scroll, want 2", got)
			}
			if got := scrollArea.Send(Sel("accessibilityVerticalScrollBar")); got != a.Element(barID) {
				t.Errorf("the scroll area's vertical scroll bar = %#x, want the bar %#x", got, a.Element(barID))
			}
		})
	})
}

// TestAXOffscreenChildrenAreHidden proves a node scrolled clean out of view is marked hidden rather than quietly handed
// over as if it were on the screen. It is still a child — an assistive technology has to be given an element before it
// can ask for it to be scrolled into view — but its frame lies outside the window, and AXHidden is what stops VoiceOver
// drawing its cursor around it. It is the counterpart of the offscreen property the Windows adapter reports and of the
// showing and visible states the AT-SPI one drops.
func TestAXOffscreenChildrenAreHidden(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 1200 + iota
			scrollID
			seenID
			goneID
		)
		tree := &accessibility.Tree{
			Nodes: map[accessibility.NodeID]*accessibility.Node{
				rootID: {
					ID: rootID, Children: []accessibility.NodeID{scrollID}, Role: role.Window,
					Bounds: geom.NewRect(0, 0, 320, 240),
				},
				scrollID: {
					ID: scrollID, Parent: rootID, Children: []accessibility.NodeID{seenID, goneID},
					Role: role.ScrollArea, Bounds: geom.NewRect(0, 0, 200, 40),
				},
				seenID: {
					ID: seenID, Parent: scrollID, Role: role.Button, Name: "Here",
					Bounds: geom.NewRect(0, 0, 80, 24), Actions: accessibility.ActionSet(0).With(accessibility.Press),
				},
				goneID: {
					ID: goneID, Parent: scrollID, Role: role.Button, Name: "Below",
					Bounds: geom.NewRect(0, 400, 80, 24), Offscreen: true,
					Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.ScrollIntoView),
				},
			},
			Root:       rootID,
			Generation: 1,
		}
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			scrollArea := axTestChildren(t, v, 1)[0]
			children := IDsFromNSArray(scrollArea.Send(Sel("accessibilityChildren")))
			if len(children) != 2 {
				t.Fatalf("the scroll area presented %d children, want 2 (the one out of view is still reachable)",
					len(children))
			}
			if objc.Send[bool](children[0], Sel("isAccessibilityHidden")) {
				t.Error("the child in view reports itself as hidden")
			}
			if !objc.Send[bool](children[1], Sel("isAccessibilityHidden")) {
				t.Error("the child scrolled out of view does not report itself as hidden")
			}
			visible := IDsFromNSArray(scrollArea.Send(Sel("accessibilityVisibleChildren")))
			if len(visible) != 1 || visible[0] != children[0] {
				t.Errorf("the scroll area's visible children = %v, want just the one in view %#x", visible,
					children[0])
			}
		})
	})
}

// TestAXLayoutChangedNamesEachElementOnce proves the one layout-changed notification a publish posts names each
// element once. A node that both moved and gained a child produces two of the events gathered into it, and naming it
// twice has VoiceOver re-read the same frame twice for nothing.
func TestAXLayoutChangedNamesEachElementOnce(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 1300 + iota
			groupID
			firstID
			secondID
		)
		build := func(grown bool) *accessibility.Tree {
			group := &accessibility.Node{
				ID: groupID, Parent: rootID, Children: []accessibility.NodeID{firstID}, Role: role.Group,
				Name: "Box", Bounds: geom.NewRect(0, 0, 200, 40),
			}
			nodes := map[accessibility.NodeID]*accessibility.Node{
				rootID: {
					ID: rootID, Children: []accessibility.NodeID{groupID}, Role: role.Window,
					Bounds: geom.NewRect(0, 0, 320, 240),
				},
				groupID: group,
				firstID: {
					ID: firstID, Parent: groupID, Role: role.Label, Name: "One",
					Bounds: geom.NewRect(0, 0, 80, 20),
				},
			}
			if grown {
				// The group both moved and gained a child, which is one BoundsChanged and one ChildrenChanged for the
				// same node.
				group.Bounds = geom.NewRect(0, 10, 200, 60)
				group.Children = append(group.Children, secondID)
				nodes[secondID] = &accessibility.Node{
					ID: secondID, Parent: groupID, Role: role.Label, Name: "Two",
					Bounds: geom.NewRect(0, 30, 80, 20),
				}
			}
			return &accessibility.Tree{Nodes: nodes, Root: rootID, Generation: 1}
		}
		before := build(false)
		v, a, cleanup := newAXAdapterWithTree(t, before)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() { axTestChildren(t, v, 1) })
		if a.Element(groupID) == 0 {
			t.Fatal("the group's element was never created")
		}
		ensureAXNotifyFuncs()
		realPostInfo := axPostNotifyInfo
		defer func() { axPostNotifyInfo = realPostInfo }()
		var named [][]objc.ID
		wantName := GoStringFromNSString(AppKitString(axNotifyLayoutChanged))
		axPostNotifyInfo = func(element, notification, userInfo objc.ID) {
			if element == objc.ID(v) && GoStringFromNSString(notification) == wantName {
				named = append(named, IDsFromNSArray(userInfo.Send(Sel("objectForKey:"),
					AppKitString(axKeyUIElements))))
			}
			realPostInfo(element, notification, userInfo)
		}
		after := build(true)
		after.Generation = 2
		a.Publish(after, accessibility.Diff(before, after))
		if len(named) != 1 {
			t.Fatalf("%d layout-changed notifications were posted, want 1", len(named))
		}
		if len(named[0]) != 1 || named[0][0] != a.Element(groupID) {
			t.Errorf("the layout-changed notification named %v, want the group %#x exactly once", named[0],
				a.Element(groupID))
		}
	})
}

// TestAXRoleChange proves a node that changes role is reported, which macOS has no notification for. A live node really
// can change role — a label becomes an image when its text is swapped for a drawable, a button becomes a toggle button
// when it is made sticky — and the element answering for it goes on answering, since it holds nothing but the node id
// and reads the role from the current snapshot every time it is asked. What has to happen is that the assistive
// technology is told to look again, which is the layout-changed notification against the node's container.
func TestAXRoleChange(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 900 + iota
			groupID
			buttonID
		)
		build := func(r role.Enum) *accessibility.Tree {
			return &accessibility.Tree{
				Nodes: map[accessibility.NodeID]*accessibility.Node{
					rootID: {
						ID: rootID, Children: []accessibility.NodeID{groupID}, Role: role.Window,
						Bounds: geom.NewRect(0, 0, 320, 240),
					},
					groupID: {
						ID: groupID, Parent: rootID, Children: []accessibility.NodeID{buttonID}, Role: role.Group,
						Name: "Box", Bounds: geom.NewRect(0, 0, 200, 40),
					},
					buttonID: {
						ID: buttonID, Parent: groupID, Role: r, Name: "Sticky", Bounds: geom.NewRect(0, 0, 80, 24),
						Pressed: true, Actions: accessibility.ActionSet(0).With(accessibility.Press),
					},
				},
				Root:       rootID,
				Generation: 1,
			}
		}
		before := build(role.Button)
		v, a, cleanup := newAXAdapterWithTree(t, before)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			axTestChildren(t, v, 1)[0].Send(Sel("accessibilityChildren"))
		})
		button := a.Element(buttonID)
		if button == 0 {
			t.Fatal("the button's element was never created")
		}
		ensureAXNotifyFuncs()
		realPostInfo := axPostNotifyInfo
		defer func() { axPostNotifyInfo = realPostInfo }()
		type posted struct {
			name     string
			elements []objc.ID
			element  objc.ID
		}
		var postedInfo []posted
		axPostNotifyInfo = func(element, notification, userInfo objc.ID) {
			postedInfo = append(postedInfo, posted{
				element:  element,
				name:     GoStringFromNSString(notification),
				elements: IDsFromNSArray(userInfo.Send(Sel("objectForKey:"), AppKitString(axKeyUIElements))),
			})
			realPostInfo(element, notification, userInfo)
		}
		after := build(role.ToggleButton)
		after.Generation = 2
		events := accessibility.Diff(before, after)
		if len(events) == 0 || events[0].Kind != accessibility.RoleChanged || events[0].Node != buttonID {
			t.Fatalf("the diff produced %v, want a role-changed event for the button first", events)
		}
		a.Publish(after, events)
		// The element is kept rather than destroyed: it caches nothing about the role, and destroying it would pull the
		// VoiceOver cursor out of whatever it was reading.
		if got := a.Element(buttonID); got != button {
			t.Errorf("the button's element after the role change = %#x, want the one it had %#x", got, button)
		}
		WithPool(func() {
			if got := GoStringFromNSString(button.Send(Sel("accessibilityRole"))); got != "AXCheckBox" {
				t.Errorf("the button's role after the change = %q, want AXCheckBox", got)
			}
			if got := GoStringFromNSString(button.Send(Sel("accessibilitySubrole"))); got != "AXToggle" {
				t.Errorf("the button's subrole after the change = %q, want AXToggle", got)
			}
		})
		layoutChanged := 0
		wantName := GoStringFromNSString(AppKitString(axNotifyLayoutChanged))
		for _, p := range postedInfo {
			if p.element != objc.ID(v) || p.name != wantName {
				continue
			}
			layoutChanged++
			if len(p.elements) != 1 || p.elements[0] != a.Element(groupID) {
				t.Errorf("the layout-changed notification named %v, want just the group %#x", p.elements,
					a.Element(groupID))
			}
		}
		if layoutChanged != 1 {
			t.Errorf("%d layout-changed notifications were posted for the role change, want 1", layoutChanged)
		}
	})
}

// axSelectorAllowed reports what an element answers when AppKit asks whether an accessibility client may invoke a
// selector on it, which is how the advertised surface of each element is decided.
func axSelectorAllowed(element objc.ID, selector string) bool {
	return objc.Send[bool](element, Sel("isAccessibilitySelectorAllowed:"), Sel(selector))
}

// TestAXSelectorAllowed proves each element advertises only what its node can actually do. The method table is one
// flat set shared by every node, so without isAccessibilitySelectorAllowed: a label offers AXIncrement and
// AXDecrement, a button offers the whole text protocol, a group offers AXRows and AXDisclosing, and every element
// says its value can be set — including a progress bar, which is exactly the read-only node the other two platforms
// report as READ_ONLY and IsReadOnly. What the element advertises now comes from the same fields the answers come
// from, so an assistive technology is offered exactly what it will be allowed to do.
func TestAXSelectorAllowed(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 1400 + iota
			labelID
			buttonID
			sliderID
			fieldID
			tableID
			rowID
			cellID
			progressID
		)
		tree := &accessibility.Tree{
			Nodes: map[accessibility.NodeID]*accessibility.Node{
				rootID: {
					ID: rootID, Role: role.Window, Bounds: geom.NewRect(0, 0, 320, 240),
					Children: []accessibility.NodeID{labelID, buttonID, sliderID, fieldID, tableID, progressID},
				},
				labelID: {
					ID: labelID, Parent: rootID, Role: role.Label, Name: "Status",
					Bounds: geom.NewRect(0, 0, 80, 20),
				},
				buttonID: {
					ID: buttonID, Parent: rootID, Role: role.Button, Name: "OK", Bounds: geom.NewRect(0, 20, 80, 24),
					Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
				},
				sliderID: {
					ID: sliderID, Parent: rootID, Role: role.Slider, Name: "Volume",
					Bounds: geom.NewRect(0, 44, 100, 20), HasNumber: true, Number: 3, Min: 1, Max: 9, Step: 1,
					Actions: accessibility.ActionSet(0).With(accessibility.Increment, accessibility.Decrement,
						accessibility.SetValue),
				},
				fieldID: {
					ID: fieldID, Parent: rootID, Role: role.TextField, Name: "Full name", Value: "Ada",
					Bounds: geom.NewRect(0, 64, 200, 24),
					Text:   &accessibility.TextInfo{Text: "Ada", SelStart: 3, SelEnd: 3, Caret: 3},
					Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetValue,
						accessibility.SetTextSelection),
				},
				tableID: {
					ID: tableID, Parent: rootID, Children: []accessibility.NodeID{rowID}, Role: role.Table,
					Name: "Files", Bounds: geom.NewRect(0, 88, 200, 20), RowCount: 1, ColumnCount: 1,
				},
				rowID: {
					ID: rowID, Parent: tableID, Children: []accessibility.NodeID{cellID}, Role: role.Row,
					Bounds: geom.NewRect(0, 88, 200, 20), RowIndex: 0, Level: 1, Selectable: true, Expandable: true,
					Actions: accessibility.ActionSet(0).With(accessibility.Select, accessibility.Expand),
				},
				cellID: {
					ID: cellID, Parent: rowID, Role: role.Cell, Name: "notes.txt",
					Bounds: geom.NewRect(0, 88, 200, 20), RowIndex: 0, ColumnIndex: 0,
				},
				// A progress bar is the read-only node: it has a number and no way to change it.
				progressID: {
					ID: progressID, Parent: rootID, Role: role.ProgressBar, Name: "Copying",
					Bounds: geom.NewRect(0, 108, 200, 20), HasNumber: true, Number: 40, Max: 100, ReadOnly: true,
				},
			},
			Root:       rootID,
			Generation: 1,
		}
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			children := axTestChildren(t, v, 6)
			rows := IDsFromNSArray(children[4].Send(Sel("accessibilityRows")))
			if len(rows) != 1 {
				t.Fatalf("the table reported %d rows, want 1", len(rows))
			}
			// The elements the expectation rows below answer for, in the order their characters are read.
			elements := []struct {
				name    string
				element objc.ID
			}{
				{name: "label", element: children[0]},
				{name: "button", element: children[1]},
				{name: "slider", element: children[2]},
				{name: "text field", element: children[3]},
				{name: "table", element: children[4]},
				{name: "row", element: rows[0]},
				{name: "progress bar", element: children[5]},
			}
			// The expectation rows are made of these: one character per element in the order above, "y" for an
			// element that offers the selector and "n" for one that refuses it.
			const (
				axByEveryNode    = "yyyyyyy"
				axByNoNode       = "nnnnnnn"
				axByButton       = "nynnnnn"
				axBySlider       = "nnynnnn"
				axByTextField    = "nnnynnn"
				axBySettable     = "nnyynnn"
				axByRowContainer = "nnnnynn"
				axByRow          = "nnnnnyn"
			)
			for _, c := range []struct {
				selector string
				allowed  string
			}{
				// Everything the class answers for every node stays on offer, whatever the node is.
				{selector: "accessibilityRole", allowed: axByEveryNode},
				{selector: "accessibilityLabel", allowed: axByEveryNode},
				{selector: "accessibilityValue", allowed: axByEveryNode},
				{selector: "accessibilityFrame", allowed: axByEveryNode},
				{selector: "accessibilityParent", allowed: axByEveryNode},
				{selector: "accessibilityChildren", allowed: axByEveryNode},
				{selector: "accessibilityMaxValue", allowed: axByEveryNode},
				{selector: "isAccessibilityFocused", allowed: axByEveryNode},
				{selector: "setAccessibilityFocused:", allowed: axByEveryNode},
				// The performers, each offered by exactly the nodes whose action set has it. AppKit derives the
				// AXIncrement and AXDecrement action names from these two being implemented at all, so a label that
				// answers them advertises stepping it cannot do.
				{selector: "accessibilityPerformPress", allowed: axByButton},
				{selector: "accessibilityPerformConfirm", allowed: axByButton},
				{selector: "accessibilityPerformIncrement", allowed: axBySlider},
				{selector: "accessibilityPerformDecrement", allowed: axBySlider},
				{selector: "accessibilityPerformShowMenu", allowed: axByNoNode},
				// The text protocol belongs to the nodes that hold text and to no others.
				{selector: "accessibilityNumberOfCharacters", allowed: axByTextField},
				{selector: "accessibilitySelectedText", allowed: axByTextField},
				{selector: "accessibilitySelectedTextRange", allowed: axByTextField},
				{selector: "accessibilityStringForRange:", allowed: axByTextField},
				{selector: "accessibilityAttributedStringForRange:", allowed: axByTextField},
				{selector: "accessibilityRangeForLine:", allowed: axByTextField},
				{selector: "accessibilityLineForIndex:", allowed: axByTextField},
				{selector: "accessibilityInsertionPointLineNumber", allowed: axByTextField},
				{selector: "setAccessibilitySelectedTextRange:", allowed: axByTextField},
				// The rows of a container, and the disclosure of a row: a group offering either is the symptom.
				{selector: "accessibilityRows", allowed: axByRowContainer},
				{selector: "accessibilitySelectedRows", allowed: axByRowContainer},
				{selector: "accessibilityVisibleRows", allowed: axByRowContainer},
				{selector: "setAccessibilitySelectedRows:", allowed: axByRowContainer},
				{selector: "isAccessibilityDisclosed", allowed: axByRow},
				{selector: "accessibilityDisclosedRows", allowed: axByRow},
				{selector: "accessibilityDisclosedByRow", allowed: axByRow},
				{selector: "setAccessibilityDisclosed:", allowed: axByRow},
				// The settable state. The progress bar is the read-only node: it reports a number that nobody may
				// set, which is what ReadOnly says and what the other two platforms report as READ_ONLY and
				// IsReadOnly.
				{selector: "setAccessibilityValue:", allowed: axBySettable},
				{selector: "setAccessibilitySelected:", allowed: axByRow},
				{selector: "setAccessibilityExpanded:", allowed: axByRow},
			} {
				if len(c.allowed) != len(elements) {
					t.Fatalf("the row for %s covers %d elements, want %d", c.selector, len(c.allowed), len(elements))
				}
				for i, e := range elements {
					want := c.allowed[i] == 'y'
					if got := axSelectorAllowed(e.element, c.selector); got != want {
						t.Errorf("the %s allows %s = %v, want %v", e.name, c.selector, got, want)
					}
				}
			}
		})
		// An element whose node has left the tree speaks for nothing, so it may be asked to do nothing either.
		stale := a.Element(labelID)
		if stale == 0 {
			t.Fatal("the label's element was never created")
		}
		Retain(stale)
		defer Release(stale)
		next := &accessibility.Tree{
			Nodes: make(map[accessibility.NodeID]*accessibility.Node), Root: rootID,
			Generation: 2,
		}
		for id, n := range tree.Nodes {
			next.Nodes[id] = n
		}
		delete(next.Nodes, labelID)
		next.Nodes[rootID] = &accessibility.Node{
			ID: rootID, Role: role.Window, Bounds: geom.NewRect(0, 0, 320, 240),
			Children: []accessibility.NodeID{buttonID, sliderID, fieldID, tableID, progressID},
		}
		a.Publish(next, accessibility.Diff(tree, next))
		WithPool(func() {
			if axSelectorAllowed(stale, "accessibilityPerformPress") {
				t.Error("a stale element still advertises that it can be pressed")
			}
			if axSelectorAllowed(stale, "setAccessibilityValue:") {
				t.Error("a stale element still advertises that its value can be set")
			}
		})
	})
}

// TestAXShortcutParser proves the accelerator parser against the strings the menus themselves draw. KeyBinding.String
// builds them as the modifier glyphs in a fixed order followed by the key's own name, and AXMenuItemCmdModifiers
// counts the command key by its absence: zero means command alone, and a shortcut without it has to set the
// no-command bit.
func TestAXShortcutParser(t *testing.T) {
	for _, c := range []struct {
		shortcut  string
		char      string
		modifiers int64
		ok        bool
	}{
		{shortcut: "⌘S", char: "S", modifiers: 0, ok: true},
		{shortcut: "⇧⌘Z", char: "Z", modifiers: axMenuModifierShift, ok: true},
		{
			shortcut:  "⌃⌥⇧⌘A",
			char:      "A",
			modifiers: axMenuModifierControl | axMenuModifierOption | axMenuModifierShift,
			ok:        true,
		},
		{shortcut: "⌘,", char: ",", modifiers: 0, ok: true},
		// A key whose name is a word is reported as the word, which is what the menu shows and what an assistive
		// technology can say.
		{shortcut: "⌘Delete", char: "Delete", modifiers: 0, ok: true},
		{shortcut: "⌘NumPad-0", char: "NumPad-0", modifiers: 0, ok: true},
		// No command key at all, which is the one thing the modifier bits have to say outright.
		{shortcut: "F1", char: "F1", modifiers: axMenuModifierNoCommand, ok: true},
		{
			shortcut:  "⌃Space",
			char:      "Space",
			modifiers: axMenuModifierControl | axMenuModifierNoCommand,
			ok:        true,
		},
		// Caps lock and num lock can be drawn but have no bit of their own, so they are dropped.
		{shortcut: "⇪⌘P", char: "P", modifiers: 0, ok: true},
		{shortcut: "⇭⌘1", char: "1", modifiers: 0, ok: true},
		// Nothing to report: no shortcut, and modifiers with no key to go with them.
		{shortcut: ""},
		{shortcut: "⌘"},
		{shortcut: "⇧⌘"},
	} {
		char, modifiers, ok := axShortcutOf(&accessibility.Node{Shortcut: c.shortcut})
		if ok != c.ok {
			t.Errorf("axShortcutOf(%q) reported ok = %v, want %v", c.shortcut, ok, c.ok)
			continue
		}
		if char != c.char || modifiers != c.modifiers {
			t.Errorf("axShortcutOf(%q) = %q, %d, want %q, %d", c.shortcut, char, modifiers, c.char, c.modifiers)
		}
	}
}

// TestAXShortcutAttributes proves a node's Shortcut reaches the accessibility system as the accelerator that
// activates it. AXMenuItemCmdChar and AXMenuItemCmdModifiers are the only attributes macOS has for it, they predate
// the NSAccessibility protocol and have no property in it, so they are published through the informal protocol the
// way AppKit's own NSMenuItem publishes them — and everything else has to go on being answered by the superclass.
func TestAXShortcutAttributes(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 1500 + iota
			menuID
			itemID
			plainID
		)
		tree := &accessibility.Tree{
			Nodes: map[accessibility.NodeID]*accessibility.Node{
				rootID: {
					ID: rootID, Children: []accessibility.NodeID{menuID}, Role: role.Window,
					Bounds: geom.NewRect(0, 0, 320, 240),
				},
				menuID: {
					ID: menuID, Parent: rootID, Children: []accessibility.NodeID{itemID, plainID}, Role: role.Menu,
					Name: "File", Bounds: geom.NewRect(0, 0, 200, 40),
				},
				itemID: {
					ID: itemID, Parent: menuID, Role: role.MenuItem, Name: "Save", Shortcut: "⇧⌘S",
					Bounds:  geom.NewRect(0, 0, 200, 20),
					Actions: accessibility.ActionSet(0).With(accessibility.Press),
				},
				// An item with no key binding of its own, which must advertise neither attribute.
				plainID: {
					ID: plainID, Parent: menuID, Role: role.MenuItem, Name: "Save As…",
					Bounds:  geom.NewRect(0, 20, 200, 20),
					Actions: accessibility.ActionSet(0).With(accessibility.Press),
				},
			},
			Root:       rootID,
			Generation: 1,
		}
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			items := IDsFromNSArray(axTestChildren(t, v, 1)[0].Send(Sel("accessibilityChildren")))
			if len(items) != 2 {
				t.Fatalf("the menu has %d children, want 2", len(items))
			}
			item, plain := items[0], items[1]
			names := func(element objc.ID) []string {
				ids := IDsFromNSArray(element.Send(Sel("accessibilityAttributeNames")))
				out := make([]string, 0, len(ids))
				for _, id := range ids {
					out = append(out, GoStringFromNSString(id))
				}
				return out
			}
			itemNames := names(item)
			for _, want := range []string{axAttrMenuItemCmdChar, axAttrMenuItemCmdModifiers} {
				if !slices.Contains(itemNames, want) {
					t.Errorf("the item's attributes %v do not include %s", itemNames, want)
				}
			}
			plainNames := names(plain)
			for _, unwanted := range []string{axAttrMenuItemCmdChar, axAttrMenuItemCmdModifiers} {
				if slices.Contains(plainNames, unwanted) {
					t.Errorf("the item with no key binding advertises %s", unwanted)
				}
			}
			// Whatever the superclass answers for every other attribute, it has to go on answering it: the names an
			// element lists are its own plus the two, never only the two.
			if len(plainNames) == 0 || len(itemNames) != len(plainNames)+2 {
				t.Errorf("the item lists %d attributes and the plain item %d, want the item's to be the plain "+
					"item's two longer", len(itemNames), len(plainNames))
			}
			if got := GoStringFromNSString(item.Send(Sel("accessibilityAttributeValue:"),
				NSStringFromGo(axAttrMenuItemCmdChar))); got != "S" {
				t.Errorf("the item's %s = %q, want S", axAttrMenuItemCmdChar, got)
			}
			if got := Int64FromNSNumber(item.Send(Sel("accessibilityAttributeValue:"),
				NSStringFromGo(axAttrMenuItemCmdModifiers))); got != axMenuModifierShift {
				t.Errorf("the item's %s = %d, want %d", axAttrMenuItemCmdModifiers, got, axMenuModifierShift)
			}
			for _, attribute := range []string{axAttrMenuItemCmdChar, axAttrMenuItemCmdModifiers} {
				if got := plain.Send(Sel("accessibilityAttributeValue:"), NSStringFromGo(attribute)); got != 0 {
					t.Errorf("the item with no key binding answered %s with %#x, want nothing", attribute, got)
				}
			}
			// The protocol path is untouched by the informal one being answered as well.
			if got := GoStringFromNSString(item.Send(Sel("accessibilityRole"))); got != "AXMenuItem" {
				t.Errorf("the item's role = %q, want AXMenuItem", got)
			}
			if got := GoStringFromNSString(item.Send(Sel("accessibilityLabel"))); got != "Save" {
				t.Errorf("the item's label = %q, want Save", got)
			}
			if got := item.Send(Sel("accessibilityParent")); got != a.Element(menuID) {
				t.Errorf("the item's parent = %#x, want the menu %#x", got, a.Element(menuID))
			}
		})
	})
}

// axModalTestTree returns a window whose root reports the given modality, with one button in it.
func axModalTestTree(modal bool, generation uint64) *accessibility.Tree {
	const (
		rootID   accessibility.NodeID = 1601
		buttonID accessibility.NodeID = 1602
	)
	return &accessibility.Tree{
		Nodes: map[accessibility.NodeID]*accessibility.Node{
			rootID: {
				ID: rootID, Children: []accessibility.NodeID{buttonID}, Role: role.Dialog, Name: "Confirm",
				Bounds: geom.NewRect(0, 0, 320, 240), Modal: modal,
			},
			buttonID: {
				ID: buttonID, Parent: rootID, Role: role.Button, Name: "OK", Bounds: geom.NewRect(0, 0, 80, 24),
				Actions: accessibility.ActionSet(0).With(accessibility.Press),
			},
		},
		Root:       rootID,
		Generation: generation,
	}
}

// TestAXModalWindow proves a modal dialog is reported as modal. Window.RunModal runs unison's own event loop rather
// than an NSApp modal session, so no window of ours ever becomes NSApp's modal window and AppKit cannot derive
// AXModal for itself the way the style mask and the window level give it AXResizable and AXFloating. The adapter sets
// the NSWindow's own accessibilityModal instead, on the first publish and whenever it changes.
func TestAXModalWindow(t *testing.T) {
	runOnMain(func() {
		modal := axModalTestTree(true, 1)
		_, a, cleanup := newAXAdapterWithTree(t, modal)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			if !objc.Send[bool](objc.ID(a.wnd), Sel("isAccessibilityModal")) {
				t.Error("the window of a modal dialog does not report itself as modal")
			}
		})
		// Dismissing the modality tells the window again, so a window reused for something else does not stay modal.
		ordinary := axModalTestTree(false, 2)
		a.Publish(ordinary, accessibility.Diff(modal, ordinary))
		WithPool(func() {
			if objc.Send[bool](objc.ID(a.wnd), Sel("isAccessibilityModal")) {
				t.Error("the window still reports itself as modal after the root node stopped being modal")
			}
		})
		// And a publish that changes nothing about the modality leaves it where it is.
		again := axModalTestTree(true, 3)
		a.Publish(again, accessibility.Diff(ordinary, again))
		if !a.modal || !a.modalKnown {
			t.Errorf("the adapter holds modal = %v, known = %v, want both true", a.modal, a.modalKnown)
		}
		WithPool(func() {
			if !objc.Send[bool](objc.ID(a.wnd), Sel("isAccessibilityModal")) {
				t.Error("the window does not report itself as modal after the root node became modal again")
			}
		})
	})
}

// TestAXOrdinaryWindowIsNotModal proves the ordinary case says so: a window whose root is not modal is told as much
// on the first publish, so nothing is left to whatever the NSWindow happened to hold.
func TestAXOrdinaryWindowIsNotModal(t *testing.T) {
	runOnMain(func() {
		_, a, _, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		if !a.modalKnown || a.modal {
			t.Errorf("the adapter holds modal = %v, known = %v, want false and true", a.modal, a.modalKnown)
		}
		WithPool(func() {
			if objc.Send[bool](objc.ID(a.wnd), Sel("isAccessibilityModal")) {
				t.Error("an ordinary window reports itself as modal")
			}
		})
	})
}

// axRecordedNotification is one notification a publish posted, in the order it went out; see axRecordNotifications.
type axRecordedNotification struct {
	name    string
	element objc.ID
}

// axRecordNotifications records every notification posted, with and without user information, into one list in the
// order they go out, and returns the function that stops recording. It is what the tests about ordering need, since
// the layout-changed notification carries user information and the focus one does not, so watching either function
// alone cannot say which came first.
func axRecordNotifications(recorded *[]axRecordedNotification) func() {
	ensureAXNotifyFuncs()
	realPost := axPostNotify
	realPostInfo := axPostNotifyInfo
	axPostNotify = func(element, notification objc.ID) {
		*recorded = append(*recorded, axRecordedNotification{
			element: element,
			name:    GoStringFromNSString(notification),
		})
		realPost(element, notification)
	}
	axPostNotifyInfo = func(element, notification, userInfo objc.ID) {
		*recorded = append(*recorded, axRecordedNotification{
			element: element,
			name:    GoStringFromNSString(notification),
		})
		realPostInfo(element, notification, userInfo)
	}
	return func() {
		axPostNotify = realPost
		axPostNotifyInfo = realPostInfo
	}
}

// The node ids axMovedAndFocusedTree builds.
const (
	axMovedRootID accessibility.NodeID = 1700 + iota
	axMovedGroupID
	axMovedButtonID
)

// axMovedAndFocusedTree returns a window holding a group and a button, with the button moved and holding the focus
// once moved is true.
func axMovedAndFocusedTree(moved bool) *accessibility.Tree {
	const (
		rootID   = axMovedRootID
		groupID  = axMovedGroupID
		buttonID = axMovedButtonID
	)
	bounds := geom.NewRect(0, 0, 80, 24)
	if moved {
		bounds = geom.NewRect(0, 10, 80, 24)
	}
	tree := &accessibility.Tree{
		Nodes: map[accessibility.NodeID]*accessibility.Node{
			rootID: {
				ID: rootID, Children: []accessibility.NodeID{groupID}, Role: role.Window,
				Bounds: geom.NewRect(0, 0, 320, 240),
			},
			groupID: {
				ID: groupID, Parent: rootID, Children: []accessibility.NodeID{buttonID}, Role: role.Group,
				Name: "Box", Bounds: geom.NewRect(0, 0, 200, 40),
			},
			buttonID: {
				ID: buttonID, Parent: groupID, Role: role.Button, Name: "OK", Bounds: bounds, Focusable: true,
				Focused: moved,
				Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
			},
		},
		Root:       rootID,
		Generation: 1,
	}
	if moved {
		tree.Focus = buttonID
	}
	return tree
}

// TestAXFocusChangeFlushesLayoutChangedFirst proves everything a publish gathered goes out before the focus does. An
// assistive technology is told where to look only once it has been told what changed — which is why Diff puts the
// focus event last of all — and that has to hold for the layout-changed notification too: posted after the focus
// moved, it tells VoiceOver to re-read the frame of an element its cursor has already left.
func TestAXFocusChangeFlushesLayoutChangedFirst(t *testing.T) {
	runOnMain(func() {
		before := axMovedAndFocusedTree(false)
		v, a, cleanup := newAXAdapterWithTree(t, before)
		defer cleanup()
		if a == nil {
			return
		}
		// Only elements that exist are named, so the button has to have been asked about first.
		var button objc.ID
		WithPool(func() {
			group := axTestChildren(t, v, 1)[0]
			children := IDsFromNSArray(group.Send(Sel("accessibilityChildren")))
			if len(children) != 1 {
				t.Fatalf("the group has %d children, want 1", len(children))
			}
			button = children[0]
		})
		var recorded []axRecordedNotification
		defer axRecordNotifications(&recorded)()
		after := axMovedAndFocusedTree(true)
		after.Generation = 2
		events := accessibility.Diff(before, after)
		if len(events) == 0 || events[len(events)-1].Kind != accessibility.FocusChanged {
			t.Fatalf("the diff produced %v, want the focus change last of all", events)
		}
		a.Publish(after, events)
		want := []axRecordedNotification{
			{element: objc.ID(v), name: GoStringFromNSString(AppKitString(axNotifyLayoutChanged))},
			{element: button, name: GoStringFromNSString(AppKitString(axNotifyFocusedUIElement))},
		}
		if !slices.Equal(recorded, want) {
			t.Errorf("the publish posted %v, want %v (the layout change before the focus)", recorded, want)
		}
	})
}

// TestAXLayoutChangedSkippedWhenNothingHasAnElement proves a layout-changed notification naming nothing is not posted
// at all. The notification is worth something only for the elements it names — VoiceOver re-reads their frames, and
// one with an empty list leaves its cursor where the old frame was — and every gathered id lacking an element is the
// ordinary case of something moving that no assistive technology has ever asked about.
func TestAXLayoutChangedSkippedWhenNothingHasAnElement(t *testing.T) {
	runOnMain(func() {
		before := axMovedAndFocusedTree(false)
		v, a, cleanup := newAXAdapterWithTree(t, before)
		defer cleanup()
		if a == nil {
			return
		}
		var recorded []axRecordedNotification
		stop := axRecordNotifications(&recorded)
		defer stop()
		// Nothing has asked about anything yet, so the moved button has no element to name.
		moved := axMovedAndFocusedTree(false)
		moved.Generation = 2
		moved.Nodes[axMovedButtonID].Bounds = geom.NewRect(0, 10, 80, 24)
		a.Publish(moved, accessibility.Diff(before, moved))
		if len(recorded) != 0 {
			t.Errorf("a publish naming only elements that do not exist posted %v, want nothing", recorded)
		}
		// Once the button has been asked about, the same change is worth reporting.
		WithPool(func() { axTestChildren(t, v, 1)[0].Send(Sel("accessibilityChildren")) })
		back := axMovedAndFocusedTree(false)
		back.Generation = 3
		a.Publish(back, accessibility.Diff(moved, back))
		want := []axRecordedNotification{
			{element: objc.ID(v), name: GoStringFromNSString(AppKitString(axNotifyLayoutChanged))},
		}
		if !slices.Equal(recorded, want) {
			t.Errorf("the publish posted %v, want %v", recorded, want)
		}
	})
}

// TestAXContentsMatchPresentedChildren proves AXContents and AXChildren agree. Both are registered on every element,
// so a container holding a cell with a single occupant answered its raw children for one and its presented children
// for the other: an element was created for a cell the presented hierarchy never hands out, and that element then
// reported the row as its parent while the row did not list it among its children. A scroll area is the one node that
// answers something different, since what it scrolls is its children other than its scroll bars.
func TestAXContentsMatchPresentedChildren(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 1800 + iota
			tableID
			rowID
			cellAID
			boxID
			cellBID
			labelAID
			labelBID
			scrollID
			barID
			contentID
		)
		tree := &accessibility.Tree{
			Nodes: map[accessibility.NodeID]*accessibility.Node{
				rootID: {
					ID: rootID, Children: []accessibility.NodeID{tableID, scrollID}, Role: role.Window,
					Bounds: geom.NewRect(0, 0, 320, 240),
				},
				tableID: {
					ID: tableID, Parent: rootID, Children: []accessibility.NodeID{rowID}, Role: role.Table,
					Name: "Grid", Bounds: geom.NewRect(0, 0, 200, 20), RowCount: 1, ColumnCount: 2,
				},
				rowID: {
					ID: rowID, Parent: tableID, Children: []accessibility.NodeID{cellAID, cellBID}, Role: role.Row,
					Bounds: geom.NewRect(0, 0, 200, 20), RowIndex: 0, Level: 1,
				},
				// One cell holding a single check box, which is presented in the cell's place.
				cellAID: {
					ID: cellAID, Parent: rowID, Children: []accessibility.NodeID{boxID}, Role: role.Cell,
					Bounds: geom.NewRect(0, 0, 40, 20), RowIndex: 0, ColumnIndex: 0,
				},
				boxID: {
					ID: boxID, Parent: cellAID, Role: role.CheckBox, Name: "Done", Bounds: geom.NewRect(2, 2, 16, 16),
					HasCheck: true, Actions: accessibility.ActionSet(0).With(accessibility.Press),
				},
				// One cell holding two labels, which stays a cell.
				cellBID: {
					ID: cellBID, Parent: rowID, Children: []accessibility.NodeID{labelAID, labelBID}, Role: role.Cell,
					Bounds: geom.NewRect(40, 0, 160, 20), RowIndex: 0, ColumnIndex: 1,
				},
				labelAID: {
					ID: labelAID, Parent: cellBID, Role: role.Label, Name: "Left",
					Bounds: geom.NewRect(40, 0, 80, 20),
				},
				labelBID: {
					ID: labelBID, Parent: cellBID, Role: role.Label, Name: "Right",
					Bounds: geom.NewRect(120, 0, 80, 20),
				},
				scrollID: {
					ID: scrollID, Parent: rootID, Children: []accessibility.NodeID{barID, contentID},
					Role: role.ScrollArea, Bounds: geom.NewRect(0, 20, 200, 80),
				},
				barID: {
					ID: barID, Parent: scrollID, Role: role.ScrollBar, Bounds: geom.NewRect(190, 20, 10, 80),
					Orientation: accessibility.OrientationVertical, HasNumber: true, Max: 300, Step: 8,
				},
				contentID: {
					ID: contentID, Parent: scrollID, Role: role.Group, Name: "Long form",
					Bounds: geom.NewRect(0, 20, 190, 380),
				},
			},
			Root:       rootID,
			Generation: 1,
		}
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() {
			children := axTestChildren(t, v, 2)
			table, scrollArea := children[0], children[1]
			rows := IDsFromNSArray(table.Send(Sel("accessibilityRows")))
			if len(rows) != 1 {
				t.Fatalf("the table reported %d rows, want 1", len(rows))
			}
			rowChildren := IDsFromNSArray(rows[0].Send(Sel("accessibilityChildren")))
			rowContents := IDsFromNSArray(rows[0].Send(Sel("accessibilityContents")))
			if !slices.Equal(rowChildren, rowContents) {
				t.Errorf("the row's contents %v are not its children %v", rowContents, rowChildren)
			}
			if len(rowChildren) != 2 || rowChildren[0] != a.Element(boxID) || rowChildren[1] != a.Element(cellBID) {
				t.Errorf("the row is shown holding %v, want the check box %#x and the second cell %#x", rowChildren,
					a.Element(boxID), a.Element(cellBID))
			}
			// The cell the check box stands in for is never handed out, so nothing ever creates an element for it.
			if got := a.Element(cellAID); got != 0 {
				t.Errorf("an element %#x exists for the cell the check box stands in for", got)
			}
			// A scroll area is the exception: its contents are what it scrolls, which is everything but its bars.
			if contents := IDsFromNSArray(scrollArea.Send(Sel("accessibilityContents"))); len(contents) != 1 ||
				contents[0] != a.Element(contentID) {
				t.Errorf("the scroll area's contents = %v, want just the content %#x", contents,
					a.Element(contentID))
			}
			if got := NSArrayCount(scrollArea.Send(Sel("accessibilityChildren"))); got != 2 {
				t.Errorf("the scroll area is shown holding %d children, want 2 (the bar included)", got)
			}
		})
	})
}

// TestAXAttributesChangedNotification proves the secondary attributes and relations are reported. A placeholder, a
// level, a row or column index or count, an orientation and the LabeledBy, DescribedBy and Controls links all reach
// an adapter as one AttributesChanged event, and macOS has no notification for any of them: the title-changed
// notification is the nearest thing it has to "read this element again", which is the same stand-in a changed name
// uses and all any of these needs, since every one of them is answered from the current snapshot.
func TestAXAttributesChangedNotification(t *testing.T) {
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = 1900 + iota
			fieldID
		)
		build := func(placeholder string) *accessibility.Tree {
			return &accessibility.Tree{
				Nodes: map[accessibility.NodeID]*accessibility.Node{
					rootID: {
						ID: rootID, Children: []accessibility.NodeID{fieldID}, Role: role.Window,
						Bounds: geom.NewRect(0, 0, 320, 240),
					},
					fieldID: {
						ID: fieldID, Parent: rootID, Role: role.TextField, Name: "Nickname",
						Bounds: geom.NewRect(0, 0, 200, 24), Placeholder: placeholder,
						Text: &accessibility.TextInfo{},
					},
				},
				Root:       rootID,
				Generation: 1,
			}
		}
		before := build("required")
		v, a, cleanup := newAXAdapterWithTree(t, before)
		defer cleanup()
		if a == nil {
			return
		}
		WithPool(func() { axTestChildren(t, v, 1) })
		field := a.Element(fieldID)
		if field == 0 {
			t.Fatal("the field's element was never created")
		}
		var recorded []axRecordedNotification
		defer axRecordNotifications(&recorded)()
		after := build("optional")
		after.Generation = 2
		events := accessibility.Diff(before, after)
		if !slices.ContainsFunc(events, func(e accessibility.Event) bool {
			return e.Kind == accessibility.AttributesChanged && e.Node == fieldID
		}) {
			t.Fatalf("the diff produced %v, want an attributes-changed event for the field", events)
		}
		a.Publish(after, events)
		want := []axRecordedNotification{
			{element: field, name: GoStringFromNSString(AppKitString(axNotifyTitleChanged))},
		}
		if !slices.Equal(recorded, want) {
			t.Errorf("the attribute change posted %v, want %v", recorded, want)
		}
		WithPool(func() {
			if got := GoStringFromNSString(field.Send(Sel("accessibilityPlaceholderValue"))); got != "optional" {
				t.Errorf("the field's placeholder after the change = %q, want optional", got)
			}
		})
	})
}
