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
	w, v, closeWindow := newTestWindowAndView(t)
	w.MakeKeyAndOrderFront()
	a := NewAXAdapter(v)
	if a == nil {
		t.Error("NewAXAdapter returned nil")
		return 0, nil, nil, closeWindow
	}
	tree = newAXTestTree()
	a.Publish(tree, accessibility.Diff(nil, tree))
	return v, a, tree, func() {
		a.Shutdown()
		closeWindow()
	}
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
			for i, c := range []struct {
				label   string
				role    string
				subrole string
			}{
				{label: "OK", role: "AXButton"},
				{label: "Name", role: "AXTextField"},
				{label: "Ready", role: "AXStaticText"},
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
			// UTF-16 {2, 6} covers the runes from index 2 up to but not including 7 — the astral-plane rune counts twice.
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
			// The first line owns its trailing line feed, so it runs to UTF-16 offset 9: "héllo " is 6 units, the emoji
			// is 2 and the line feed is 1.
			for _, c := range []struct {
				want NSRange
				line int64
			}{
				{line: 0, want: NSRange{Location: 0, Length: 9}},
				{line: 1, want: NSRange{Location: 9, Length: 5}},
				{line: 2, want: NSRange{}},
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
			// The astral-plane rune is one character occupying two UTF-16 code units, and asking about either half of it
			// must describe the whole of it.
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
			// A position a quarter of the way along the first line lands on the third rune, whose UTF-16 range is {2,1}.
			pt := axTestScreenPoint(a, geom.NewPoint(bounds.X+25, bounds.Y+8))
			wantRange := NSRange{Location: 2, Length: 1}
			if got := objc.Send[NSRange](field, Sel("accessibilityRangeForPosition:"), pt); got != wantRange {
				t.Errorf("accessibilityRangeForPosition: = %+v, want %+v", got, wantRange)
			}
			// A node with no text at all must answer safely rather than reach into a nil TextInfo.
			if got := objc.Send[int64](label, Sel("accessibilityNumberOfCharacters")); got != 0 {
				t.Errorf("label accessibilityNumberOfCharacters = %d, want 0", got)
			}
			if got := objc.Send[NSRange](label, Sel("accessibilitySelectedTextRange")); got != (NSRange{}) {
				t.Errorf("label accessibilitySelectedTextRange = %+v, want the zero range", got)
			}
			// Static text reports its content as its value, since that is the half of a label macOS reads.
			if got := GoStringFromNSString(label.Send(Sel("accessibilityValue"))); got != "Ready" {
				t.Errorf("label accessibilityValue = %q, want %q", got, "Ready")
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

			// The same tree without the label: the diff has to produce a NodeRemoved, and the adapter has to release the
			// element it was holding.
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
		a.Shutdown()
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
		a.Publish(next, []accessibility.Event{{Kind: accessibility.Announcement, New: "all done"}})
		AXAnnounce("spoken directly")
		AXAnnounce("") // must be a no-op rather than an exception
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
		)
		nodes := map[accessibility.NodeID]*accessibility.Node{
			rootID: {
				ID: rootID,
				Children: []accessibility.NodeID{
					checkID, sliderID, treeID, tabListID, headerID, separatorID, scrollID,
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
				Actions: accessibility.ActionSet(0).With(accessibility.Select, accessibility.AddToSelection),
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
					accessibility.AddToSelection),
			},
			tabListID: {
				ID: tabListID, Parent: rootID, Children: []accessibility.NodeID{tabID}, Role: role.TabList,
				Bounds: geom.NewRect(0, 100, 200, 20),
			},
			tabID: {
				ID: tabID, Parent: tabListID, Role: role.Tab, Name: "First", Bounds: geom.NewRect(0, 100, 60, 20),
				Selected: true,
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
				Bounds: geom.NewRect(190, 150, 10, 80), HasNumber: true, Number: 0, Max: 300, Step: 8,
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
			if len(children) != 7 {
				t.Fatalf("content view has %d children, want 7", len(children))
			}
			checkBox, slider, outline := children[0], children[1], children[2]
			tabList, header, separator, scrollArea := children[3], children[4], children[5], children[6]
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
			requests = nil
			vBar.Send(Sel("setAccessibilityValue:"), NSNumberFromFloat64(120))
			wantRequests = []accessibility.ActionRequest{{
				Node: vBarID, Action: accessibility.SetValue, Number: 120,
				Value: "120",
			}}
			if !slices.Equal(requests, wantRequests) {
				t.Errorf("setting the scroll bar's value asked for %v, want %v", requests, wantRequests)
			}
			// A hit test landing on the separator must resolve past it, to the content view standing in for the root.
			onSeparator := axTestScreenPoint(a, geom.NewPoint(100, 140))
			if got := objc.ID(v).Send(Sel("accessibilityHitTest:"), onSeparator); got != objc.ID(v) {
				t.Errorf("hit test on the separator = %#x, want the content view %#x", got, objc.ID(v))
			}
		})
	})
}
