// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"math"
	"runtime"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
	"golang.org/x/sys/windows"
)

// These tests stand in for the UI Automation client the control patterns are built for, the way the ones in
// uia_provider_windows_test.go do: they call the Go functions the vtable slots hold, with the this pointer adjusted to
// the pattern interface each one belongs to. What the answers are supposed to be is settled by the portable tests in
// uia_patterns_test.go; these check the COM around them — the out-parameters, the HRESULTs, the action requests that
// reach the window, and who owns each BSTR and SAFEARRAY.

// uiaRequestAt returns one of the action requests a test window has recorded, or the zero request when it has recorded
// fewer than that, so that a wrong expectation is reported rather than panicking.
func uiaRequestAt(w *uiaTestWindow, i int) accessibility.ActionRequest {
	requests := w.recorded()
	if i < 0 || i >= len(requests) {
		return accessibility.ActionRequest{}
	}
	return requests[i]
}

// uiaSafeArrayUnknowns returns the interface pointers held in a VT_UNKNOWN SAFEARRAY, releasing the reference that
// reading each one added, so that the array is left holding exactly what it was handed.
func uiaSafeArrayUnknowns(c check.Checker, array SAFEARRAY) []uintptr {
	c.True(array != 0, "the array must not be NULL")
	if array == 0 {
		return nil
	}
	bound, hr := SafeArrayGetUBound(array, 1)
	c.True(hresultSucceeded(hr))
	if bound < 0 {
		return nil
	}
	pointers := make([]uintptr, 0, bound+1)
	for i := int32(0); i <= bound; i++ {
		var element uintptr
		c.True(hresultSucceeded(SafeArrayGetElement(array, i, unsafe.Pointer(&element))))
		pointers = append(pointers, element)
		if element != 0 {
			uiaProviderFromThis(element, uiaIfaceSimple).release()
		}
	}
	return pointers
}

// TestUIAInvokePattern verifies the one method of the Invoke pattern: the press a client asks for reaches the window as
// an action, a disabled element refuses, and an element with nowhere to send the request says so rather than reporting
// success.
func TestUIAInvokePattern(t *testing.T) {
	c := check.New(t)
	w := newTestUIAWindow(patternTree())

	c.Equal(COM_S_OK, uiaInvokeInvoke(w.providerFor(2).ifacePtr(uiaIfaceInvoke)))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(2), uiaRequestAt(w, 0).Node)
	c.Equal(accessibility.Press, uiaRequestAt(w, 0).Action)

	// Node 3 is a disabled button. It still supports the pattern — a client has to be able to describe it — but nothing
	// acts on it.
	c.Equal(UIA_E_ELEMENTNOTENABLED, uiaInvokeInvoke(w.providerFor(3).ifacePtr(uiaIfaceInvoke)))
	c.Equal(1, len(w.recorded()))

	plain := NewUIAWindow(UIAConfig{}, patternTree(), UIAGeometry{})
	c.Equal(UIA_E_INVALIDOPERATION, uiaInvokeInvoke(plain.providerFor(2).ifacePtr(uiaIfaceInvoke)))
}

// TestUIATogglePattern verifies the Toggle pattern, including that a toggle button reports whether it is pressed while
// everything else checkable reports its check state.
func TestUIATogglePattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	state, stateAddress := uiaOut[ToggleState](&pin)
	tree := patternTree()
	w := newTestUIAWindow(tree)

	// Node 4 is a check box with a mixed check; node 5 a pressed toggle button.
	c.Equal(COM_S_OK, uiaToggleState(w.providerFor(4).ifacePtr(uiaIfaceToggle), stateAddress))
	c.Equal(ToggleState_Indeterminate, *state)
	c.Equal(COM_S_OK, uiaToggleState(w.providerFor(5).ifacePtr(uiaIfaceToggle), stateAddress))
	c.Equal(ToggleState_On, *state)

	next := patternTree()
	next.Nodes[4].Checked = checkenum.Off
	next.Generation = 2
	w.Publish(next, nil)
	c.Equal(COM_S_OK, uiaToggleState(w.providerFor(4).ifacePtr(uiaIfaceToggle), stateAddress))
	c.Equal(ToggleState_Off, *state)

	c.Equal(COM_S_OK, uiaToggleToggle(w.providerFor(4).ifacePtr(uiaIfaceToggle)))
	c.Equal(accessibility.NodeID(4), uiaRequestAt(w, 0).Node)
	c.Equal(accessibility.Toggle, uiaRequestAt(w, 0).Action)
	c.Equal(1, len(w.recorded()))
}

// TestUIATogglePressFallback verifies that Toggle presses an element that reports the Toggle pattern but offers only
// the Press action. A sticky or grouped button is reported as a toggle button, because the state a click leaves it in
// is what a client has to hear, and Toggle is then the only way a client can operate it: the role carries no Invoke
// pattern, so dispatching an action the button ignores would answer S_OK while nothing happened.
func TestUIATogglePressFallback(t *testing.T) {
	c := check.New(t)
	tree := patternTree()
	tree.Nodes[5].Actions = accessibility.ActionSet(0).With(accessibility.Press)
	w := newTestUIAWindow(tree)

	c.Equal(COM_S_OK, uiaToggleToggle(w.providerFor(5).ifacePtr(uiaIfaceToggle)))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(5), uiaRequestAt(w, 0).Node)
	c.Equal(accessibility.Press, uiaRequestAt(w, 0).Action)

	// An element that offers neither is still asked to toggle rather than pressed: pressing it is no more likely to
	// work, and the request a client made is the one worth reporting.
	neither := patternTree()
	neither.Nodes[5].Actions = 0
	neither.Generation = 2
	w.Publish(neither, nil)
	c.Equal(COM_S_OK, uiaToggleToggle(w.providerFor(5).ifacePtr(uiaIfaceToggle)))
	c.Equal(2, len(w.recorded()))
	c.Equal(accessibility.Toggle, uiaRequestAt(w, 1).Action)

	// A disabled element refuses either way.
	disabled := patternTree()
	disabled.Nodes[5].Actions = accessibility.ActionSet(0).With(accessibility.Press)
	disabled.Nodes[5].Disabled = true
	disabled.Generation = 3
	w.Publish(disabled, nil)
	c.Equal(UIA_E_ELEMENTNOTENABLED, uiaToggleToggle(w.providerFor(5).ifacePtr(uiaIfaceToggle)))
	c.Equal(2, len(w.recorded()))
}

// TestUIAValuePattern verifies the Value pattern: the text it reports, who owns the BSTR it reports it in, which
// elements refuse a new value, and how the UTF-16 string a client supplies is decoded.
func TestUIAValuePattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	str, strAddress := uiaOut[BSTR](&pin)
	boolean, booleanAddress := uiaOut[int32](&pin)
	tree := patternTree()
	w := newTestUIAWindow(tree)

	value := func(node accessibility.NodeID) string {
		c.Equal(COM_S_OK, uiaValueValue(w.providerFor(node).ifacePtr(uiaIfaceValue), strAddress))
		// The BSTR belongs to the caller, which is UI Automation Core in the real thing and this test here.
		defer str.Free()
		return BSTRToString(*str)
	}
	readOnly := func(node accessibility.NodeID) bool {
		c.Equal(COM_S_OK, uiaValueIsReadOnly(w.providerFor(node).ifacePtr(uiaIfaceValue), booleanAddress))
		return *boolean != 0
	}

	c.Equal("Gandalf", value(8), "a text field reports its content")
	c.Equal("fixed", value(10))
	c.Equal("Red", value(14))
	c.Equal("#ff0000", value(16))
	c.Equal("", value(9), "a password reports nothing")

	c.False(readOnly(8))
	c.False(readOnly(9), "a password may still be typed into")
	c.True(readOnly(10))
	c.True(readOnly(14), "a popup button reports its choice but does not take one")
	c.True(readOnly(16))

	// Setting a value hands over a NUL-terminated UTF-16 string that belongs to UI Automation, so it has to be decoded
	// before the request is queued rather than held onto.
	text, err := windows.UTF16PtrFromString("Frodo")
	c.NoError(err)
	pin.Pin(text)
	c.Equal(COM_S_OK, uiaValueSetValue(w.providerFor(8).ifacePtr(uiaIfaceValue), uintptr(unsafe.Pointer(text))))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(8), uiaRequestAt(w, 0).Node)
	c.Equal(accessibility.SetValue, uiaRequestAt(w, 0).Action)
	c.Equal("Frodo", uiaRequestAt(w, 0).Value)

	// A read-only element refuses; a disabled one refuses differently, so that a client can tell "never" from "not
	// now"; and a NULL string is a broken call rather than a request to empty the value.
	c.Equal(UIA_E_NOTSUPPORTED, uiaValueSetValue(w.providerFor(10).ifacePtr(uiaIfaceValue),
		uintptr(unsafe.Pointer(text))))
	c.Equal(COM_E_INVALIDARG, uiaValueSetValue(w.providerFor(8).ifacePtr(uiaIfaceValue), 0))
	disabled := patternTree()
	disabled.Nodes[8].Disabled = true
	disabled.Generation = 2
	w.Publish(disabled, nil)
	c.Equal(UIA_E_ELEMENTNOTENABLED, uiaValueSetValue(w.providerFor(8).ifacePtr(uiaIfaceValue),
		uintptr(unsafe.Pointer(text))))
	c.Equal(1, len(w.recorded()))
}

// TestUIARangeValuePattern verifies the RangeValue pattern, whose value arrives and leaves as a double: the bounds and
// increments a client needs to move the value, and the rules on setting it.
func TestUIARangeValuePattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	number, numberAddress := uiaOut[float64](&pin)
	boolean, booleanAddress := uiaOut[int32](&pin)
	w := newTestUIAWindow(patternTree())
	slider := w.providerFor(11).ifacePtr(uiaIfaceRangeValue)

	double := func(call func(this, out uintptr) uint64, this uintptr) float64 {
		c.Equal(COM_S_OK, call(this, numberAddress))
		return *number
	}
	c.Equal(5.0, double(uiaRangeValueValue, slider))
	c.Equal(0.0, double(uiaRangeValueMinimum, slider))
	c.Equal(10.0, double(uiaRangeValueMaximum, slider))
	c.Equal(1.0, double(uiaRangeValueSmallChange, slider), "one arrow key press")
	c.Equal(10.0, double(uiaRangeValueLargeChange, slider), "one paging key press is ten of them")

	c.Equal(COM_S_OK, uiaRangeValueIsReadOnly(slider, booleanAddress))
	c.Equal(int32(0), *boolean)
	c.Equal(COM_S_OK, uiaRangeValueIsReadOnly(w.providerFor(13).ifacePtr(uiaIfaceRangeValue), booleanAddress))
	c.Equal(int32(1), *boolean, "a progress bar is read-only")

	// The value arrives as the raw bits of a double, delivered by the assembly thunk the vtable slot holds; the thunk
	// itself is covered by TestRangeValueSetValueThunk.
	c.Equal(COM_S_OK, uiaRangeValueSetValue(slider, uintptr(math.Float64bits(7.5))))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(11), uiaRequestAt(w, 0).Node)
	c.Equal(accessibility.SetValue, uiaRequestAt(w, 0).Action)
	c.Equal(7.5, uiaRequestAt(w, 0).Number)
	// The same number as text, for a widget that takes its value that way. A numeric field is a text field underneath
	// and answers SetValue by replacing its text, so a request with only the number in it would blank the field.
	c.Equal("7.5", uiaRequestAt(w, 0).Value)

	// A value the element could never take, and one that is not a number at all, are refused rather than clamped.
	c.Equal(COM_E_INVALIDARG, uiaRangeValueSetValue(slider, uintptr(math.Float64bits(99))))
	c.Equal(COM_E_INVALIDARG, uiaRangeValueSetValue(slider, uintptr(math.Float64bits(-1))))
	c.Equal(COM_E_INVALIDARG, uiaRangeValueSetValue(slider, uintptr(math.Float64bits(math.NaN()))))
	c.Equal(UIA_E_NOTSUPPORTED, uiaRangeValueSetValue(w.providerFor(13).ifacePtr(uiaIfaceRangeValue),
		uintptr(math.Float64bits(5))))
	c.Equal(UIA_E_ELEMENTNOTENABLED, uiaRangeValueSetValue(w.providerFor(12).ifacePtr(uiaIfaceRangeValue),
		uintptr(math.Float64bits(2))))
	c.Equal(1, len(w.recorded()))
}

// TestUIASelectionPattern verifies the Selection pattern a container implements, including who owns the array it hands
// back and the references in it.
func TestUIASelectionPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	array, arrayAddress := uiaOut[SAFEARRAY](&pin)
	boolean, booleanAddress := uiaOut[int32](&pin)
	w := newTestUIAWindow(listTree())
	list := w.providerFor(2).ifacePtr(uiaIfaceSelection)

	// Nodes 3 and 5 are the selected list items, 5 of them through an ignored group.
	c.Equal(COM_S_OK, uiaSelectionGetSelection(list, arrayAddress))
	c.Equal([]uintptr{
		w.providerFor(3).ifacePtr(uiaIfaceSimple),
		w.providerFor(5).ifacePtr(uiaIfaceSimple),
	}, uiaSafeArrayUnknowns(c, *array))

	// Storing an element into the array added a reference, and destroying the array — which UI Automation Core does —
	// gives it back.
	c.Equal(int32(2), atomic.LoadInt32(&w.providerFor(3).refCount))
	c.Equal(int32(2), atomic.LoadInt32(&w.providerFor(5).refCount))
	array.Destroy()
	c.Equal(int32(1), atomic.LoadInt32(&w.providerFor(3).refCount))
	c.Equal(int32(1), atomic.LoadInt32(&w.providerFor(5).refCount))

	c.Equal(COM_S_OK, uiaSelectionCanSelectMultiple(list, booleanAddress))
	c.Equal(int32(1), *boolean)
	c.Equal(COM_S_OK, uiaSelectionIsSelectionRequired(list, booleanAddress))
	c.Equal(int32(0), *boolean, "nothing unison builds insists on a selection")

	tabs := w.providerFor(7).ifacePtr(uiaIfaceSelection)
	c.Equal(COM_S_OK, uiaSelectionGetSelection(tabs, arrayAddress))
	c.Equal([]uintptr{w.providerFor(8).ifacePtr(uiaIfaceSimple)}, uiaSafeArrayUnknowns(c, *array))
	array.Destroy()
	c.Equal(COM_S_OK, uiaSelectionCanSelectMultiple(tabs, booleanAddress))
	c.Equal(int32(0), *boolean, "one tab at a time")

	// A container with nothing selected reports an empty array rather than a NULL one.
	empty := listTree()
	empty.Nodes[3].Selected = false
	empty.Nodes[5].Selected = false
	empty.Generation = 2
	w.Publish(empty, nil)
	c.Equal(COM_S_OK, uiaSelectionGetSelection(list, arrayAddress))
	c.True(*array != 0)
	c.Nil(uiaSafeArrayUnknowns(c, *array))
	array.Destroy()

	// A table's selection is its rows.
	table := newTestUIAWindow(tableTree())
	c.Equal(COM_S_OK, uiaSelectionGetSelection(table.providerFor(6).ifacePtr(uiaIfaceSelection), arrayAddress))
	c.Equal([]uintptr{table.providerFor(7).ifacePtr(uiaIfaceSimple)}, uiaSafeArrayUnknowns(c, *array))
	array.Destroy()
}

// TestUIASelectionItemPattern verifies the SelectionItem pattern an item implements, including the radio button, which
// is a selection item with no container to point at and no way to be unchosen.
func TestUIASelectionItemPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	boolean, booleanAddress := uiaOut[int32](&pin)
	w := newTestUIAWindow(listTree())
	item := w.providerFor(3).ifacePtr(uiaIfaceSelectionItem)
	other := w.providerFor(6).ifacePtr(uiaIfaceSelectionItem)

	c.Equal(COM_S_OK, uiaSelectionItemIsSelected(item, booleanAddress))
	c.Equal(int32(1), *boolean)
	c.Equal(COM_S_OK, uiaSelectionItemIsSelected(other, booleanAddress))
	c.Equal(int32(0), *boolean)

	c.Equal(COM_S_OK, uiaSelectionItemSelectionContainer(item, outAddress))
	c.Equal(w.providerFor(2).ifacePtr(uiaIfaceSimple), *out)
	c.Equal(uintptr(1), w.providerFor(2).release(), "the container handed out is AddRef'd")

	c.Equal(COM_S_OK, uiaSelectionItemSelect(other))
	c.Equal(COM_S_OK, uiaSelectionItemAddToSelection(other))
	c.Equal(COM_S_OK, uiaSelectionItemRemoveFromSelection(item))
	c.Equal(3, len(w.recorded()))
	c.Equal(accessibility.Select, uiaRequestAt(w, 0).Action)
	c.Equal(accessibility.NodeID(6), uiaRequestAt(w, 0).Node)
	c.Equal(accessibility.AddToSelection, uiaRequestAt(w, 1).Action)
	c.Equal(accessibility.RemoveFromSelection, uiaRequestAt(w, 2).Action)
	c.Equal(accessibility.NodeID(3), uiaRequestAt(w, 2).Node)

	// A radio button reports its check state as its selected state, is chosen by being pressed, cannot be unchosen, and
	// has no container to name: its group is a layout panel with no pattern of its own.
	radios := newTestUIAWindow(patternTree())
	first := radios.providerFor(6).ifacePtr(uiaIfaceSelectionItem)
	second := radios.providerFor(7).ifacePtr(uiaIfaceSelectionItem)
	c.Equal(COM_S_OK, uiaSelectionItemIsSelected(first, booleanAddress))
	c.Equal(int32(1), *boolean)
	c.Equal(COM_S_OK, uiaSelectionItemIsSelected(second, booleanAddress))
	c.Equal(int32(0), *boolean)
	c.Equal(COM_S_OK, uiaSelectionItemSelectionContainer(first, outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(COM_S_OK, uiaSelectionItemSelect(second))
	c.Equal(COM_S_OK, uiaSelectionItemAddToSelection(second))
	c.Equal(UIA_E_INVALIDOPERATION, uiaSelectionItemRemoveFromSelection(first))
	c.Equal(2, len(radios.recorded()))
	c.Equal(accessibility.Press, uiaRequestAt(radios, 0).Action)
	c.Equal(accessibility.NodeID(7), uiaRequestAt(radios, 0).Node)
	c.Equal(accessibility.Press, uiaRequestAt(radios, 1).Action)

	// A row is a selection item too, and a disabled one is not acted on.
	rows := newTestUIAWindow(tableTree())
	c.Equal(COM_S_OK, uiaSelectionItemSelectionContainer(rows.providerFor(7).ifacePtr(uiaIfaceSelectionItem),
		outAddress))
	c.Equal(rows.providerFor(6).ifacePtr(uiaIfaceSimple), *out)
	c.Equal(uintptr(1), rows.providerFor(6).release())
	disabled := tableTree()
	disabled.Nodes[7].Disabled = true
	disabled.Generation = 2
	rows.Publish(disabled, nil)
	c.Equal(UIA_E_ELEMENTNOTENABLED, uiaSelectionItemSelect(rows.providerFor(7).ifacePtr(uiaIfaceSelectionItem)))
	c.Equal(0, len(rows.recorded()))
}

// TestUIAExpandCollapsePattern verifies the ExpandCollapse pattern, including that an element that cannot be expanded
// at all reports itself a leaf rather than collapsed: a client announces a leaf silently.
func TestUIAExpandCollapsePattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	state, stateAddress := uiaOut[ExpandCollapseState](&pin)
	w := newTestUIAWindow(patternTree())
	popup := w.providerFor(14).ifacePtr(uiaIfaceExpandCollapse)

	c.Equal(COM_S_OK, uiaExpandCollapseState(popup, stateAddress))
	c.Equal(ExpandCollapseState_Collapsed, *state)
	c.Equal(COM_S_OK, uiaExpandCollapseState(w.providerFor(15).ifacePtr(uiaIfaceExpandCollapse), stateAddress))
	c.Equal(ExpandCollapseState_LeafNode, *state, "a combo box with nothing to drop down is a leaf")

	open := patternTree()
	open.Nodes[14].Expanded = true
	open.Generation = 2
	w.Publish(open, nil)
	c.Equal(COM_S_OK, uiaExpandCollapseState(popup, stateAddress))
	c.Equal(ExpandCollapseState_Expanded, *state)

	c.Equal(COM_S_OK, uiaExpandCollapseExpand(popup))
	c.Equal(COM_S_OK, uiaExpandCollapseCollapse(popup))
	c.Equal(2, len(w.recorded()))
	c.Equal(accessibility.Expand, uiaRequestAt(w, 0).Action)
	c.Equal(accessibility.NodeID(14), uiaRequestAt(w, 0).Node)
	c.Equal(accessibility.Collapse, uiaRequestAt(w, 1).Action)

	// An expanded row in a hierarchical table is the other user of the pattern.
	rows := newTestUIAWindow(tableTree())
	c.Equal(COM_S_OK, uiaExpandCollapseState(rows.providerFor(7).ifacePtr(uiaIfaceExpandCollapse), stateAddress))
	c.Equal(ExpandCollapseState_Expanded, *state)
	c.Equal(COM_S_OK, uiaExpandCollapseCollapse(rows.providerFor(7).ifacePtr(uiaIfaceExpandCollapse)))
	c.Equal(accessibility.Collapse, uiaRequestAt(rows, 0).Action)
	c.Equal(accessibility.NodeID(7), uiaRequestAt(rows, 0).Node)
}

// TestUIAScrollItemPattern verifies the one method of the ScrollItem pattern, which a client calls to bring an element
// it is about to talk about into view.
func TestUIAScrollItemPattern(t *testing.T) {
	c := check.New(t)
	w := newTestUIAWindow(listTree())

	c.Equal(COM_S_OK, uiaScrollItemScrollIntoView(w.providerFor(3).ifacePtr(uiaIfaceScrollItem)))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(3), uiaRequestAt(w, 0).Node)
	c.Equal(accessibility.ScrollIntoView, uiaRequestAt(w, 0).Action)

	// A disabled element is not acted on, here as everywhere else: a client that wants it described still gets that.
	disabled := listTree()
	disabled.Nodes[3].Disabled = true
	disabled.Generation = 2
	w.Publish(disabled, nil)
	c.Equal(UIA_E_ELEMENTNOTENABLED, uiaScrollItemScrollIntoView(w.providerFor(3).ifacePtr(uiaIfaceScrollItem)))
	c.Equal(1, len(w.recorded()))
}

// TestUIAGridPattern verifies the Grid pattern a table implements, including the difference between a cell outside the
// grid — an error — and one the snapshot simply does not hold, which most of a large table's cells are.
func TestUIAGridPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	count, countAddress := uiaOut[int32](&pin)
	w := newTestUIAWindow(tableTree())
	grid := w.providerFor(6).ifacePtr(uiaIfaceGrid)

	c.Equal(COM_S_OK, uiaGridRowCount(grid, countAddress))
	c.Equal(int32(5), *count, "every row, not only the ones published")
	c.Equal(COM_S_OK, uiaGridColumnCount(grid, countAddress))
	c.Equal(int32(2), *count)

	c.Equal(COM_S_OK, uiaGridGetItem(grid, 1, 0, outAddress))
	c.Equal(w.providerFor(8).ifacePtr(uiaIfaceSimple), *out)
	c.Equal(uintptr(1), w.providerFor(8).release(), "the cell handed out is AddRef'd")
	c.Equal(COM_S_OK, uiaGridGetItem(grid, 2, 1, outAddress))
	c.Equal(w.providerFor(12).ifacePtr(uiaIfaceSimple), *out)
	c.Equal(uintptr(1), w.providerFor(12).release())

	// Row 0 is outside the viewport, so it was never published; asking for it is answered with nothing rather than with
	// an error.
	c.Equal(COM_S_OK, uiaGridGetItem(grid, 0, 0, outAddress))
	c.Equal(uintptr(0), *out)

	// A row or column the grid does not have at all is a broken request. A negative index arrives as every bit set,
	// which is what a client passing -1 looks like across the ABI.
	negative := ^uintptr(0)
	c.Equal(COM_E_INVALIDARG, uiaGridGetItem(grid, 5, 0, outAddress))
	c.Equal(COM_E_INVALIDARG, uiaGridGetItem(grid, 0, 2, outAddress))
	c.Equal(COM_E_INVALIDARG, uiaGridGetItem(grid, negative, 0, outAddress))
	c.Equal(COM_E_INVALIDARG, uiaGridGetItem(grid, 0, negative, outAddress))
	c.Equal(COM_E_POINTER, uiaGridGetItem(grid, 0, 0, 0))
}

// TestUIAGridItemPattern verifies the GridItem pattern a cell implements.
func TestUIAGridItemPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	index, indexAddress := uiaOut[int32](&pin)
	w := newTestUIAWindow(tableTree())
	cell := w.providerFor(9).ifacePtr(uiaIfaceGridItem)

	c.Equal(COM_S_OK, uiaGridItemRow(cell, indexAddress))
	c.Equal(int32(1), *index)
	c.Equal(COM_S_OK, uiaGridItemColumn(cell, indexAddress))
	c.Equal(int32(1), *index)
	c.Equal(COM_S_OK, uiaGridItemRowSpan(cell, indexAddress))
	c.Equal(int32(1), *index, "nothing unison draws merges cells")
	c.Equal(COM_S_OK, uiaGridItemColumnSpan(cell, indexAddress))
	c.Equal(int32(1), *index)

	c.Equal(COM_S_OK, uiaGridItemContainingGrid(cell, outAddress))
	c.Equal(w.providerFor(6).ifacePtr(uiaIfaceSimple), *out)
	c.Equal(uintptr(1), w.providerFor(6).release())
}

// TestUIATablePattern verifies the Table pattern a table implements on top of Grid, which is what adds the headers.
func TestUIATablePattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	array, arrayAddress := uiaOut[SAFEARRAY](&pin)
	major, majorAddress := uiaOut[RowOrColumnMajor](&pin)
	w := newTestUIAWindow(tableTree())
	table := w.providerFor(6).ifacePtr(uiaIfaceTable)

	c.Equal(COM_S_OK, uiaTableRowOrColumnMajor(table, majorAddress))
	c.Equal(RowOrColumnMajor_RowMajor, *major)

	// A unison table has no row headers, which is an empty array rather than a NULL one.
	c.Equal(COM_S_OK, uiaTableGetRowHeaders(table, arrayAddress))
	c.Nil(uiaSafeArrayUnknowns(c, *array))
	array.Destroy()

	// The column headers come from the TableHeader panel the table sits with, which is nowhere inside it.
	c.Equal(COM_S_OK, uiaTableGetColumnHeaders(table, arrayAddress))
	c.Equal([]uintptr{
		w.providerFor(4).ifacePtr(uiaIfaceSimple),
		w.providerFor(5).ifacePtr(uiaIfaceSimple),
	}, uiaSafeArrayUnknowns(c, *array))
	c.Equal(int32(2), atomic.LoadInt32(&w.providerFor(4).refCount))
	array.Destroy()
	c.Equal(int32(1), atomic.LoadInt32(&w.providerFor(4).refCount))

	// A table whose header has left the window reports no columns rather than something unrelated.
	headerless := tableTree()
	delete(headerless.Nodes, 3)
	delete(headerless.Nodes, 4)
	delete(headerless.Nodes, 5)
	headerless.Nodes[2].Children = []accessibility.NodeID{6}
	headerless.Generation = 2
	w.Publish(headerless, nil)
	c.Equal(COM_S_OK, uiaTableGetColumnHeaders(table, arrayAddress))
	c.Nil(uiaSafeArrayUnknowns(c, *array))
	array.Destroy()
	c.Equal(COM_E_POINTER, uiaTableGetColumnHeaders(table, 0))
}

// TestUIATableItemPattern verifies the TableItem pattern a cell implements on top of GridItem, which is what lets a
// screen reader say the column name before the cell's contents.
func TestUIATableItemPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	array, arrayAddress := uiaOut[SAFEARRAY](&pin)
	w := newTestUIAWindow(tableTree())

	c.Equal(COM_S_OK, uiaTableItemGetColumnHeaderItems(w.providerFor(8).ifacePtr(uiaIfaceTableItem), arrayAddress))
	c.Equal([]uintptr{w.providerFor(4).ifacePtr(uiaIfaceSimple)}, uiaSafeArrayUnknowns(c, *array))
	array.Destroy()
	c.Equal(COM_S_OK, uiaTableItemGetColumnHeaderItems(w.providerFor(9).ifacePtr(uiaIfaceTableItem), arrayAddress))
	c.Equal([]uintptr{w.providerFor(5).ifacePtr(uiaIfaceSimple)}, uiaSafeArrayUnknowns(c, *array))
	array.Destroy()

	// There are no row headers for a cell to belong to.
	c.Equal(COM_S_OK, uiaTableItemGetRowHeaderItems(w.providerFor(8).ifacePtr(uiaIfaceTableItem), arrayAddress))
	c.Nil(uiaSafeArrayUnknowns(c, *array))
	array.Destroy()
	c.Equal(COM_E_POINTER, uiaTableItemGetColumnHeaderItems(w.providerFor(8).ifacePtr(uiaIfaceTableItem), 0))
}

// TestUIAPatternStale verifies what a client holding a pattern interface for something that has been destroyed is told.
// Answering from the snapshot the element was last in would have a screen reader describing something that is no longer
// on screen.
func TestUIAPatternStale(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	str, strAddress := uiaOut[BSTR](&pin)
	state, stateAddress := uiaOut[ToggleState](&pin)
	w := newTestUIAWindow(patternTree())
	button := w.providerFor(2)
	button.addRef() // Stand in for the reference a client would be holding.
	field := w.providerFor(8)
	field.addRef()

	without := patternTree()
	delete(without.Nodes, 2)
	delete(without.Nodes, 8)
	without.Nodes[1].Children = []accessibility.NodeID{3, 4, 5, 6, 7, 9, 10, 11, 12, 13, 14, 15, 16, 17}
	without.Focus = 0
	without.Generation = 2
	w.Publish(without, nil)
	c.True(button.Stale())
	c.True(field.Stale())

	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaInvokeInvoke(button.ifacePtr(uiaIfaceInvoke)))
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaValueValue(field.ifacePtr(uiaIfaceValue), strAddress))
	c.Equal(BSTR(0), *str)
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaToggleState(button.ifacePtr(uiaIfaceToggle), stateAddress))
	c.Equal(ToggleState_Off, *state)
	c.Equal(0, len(w.recorded()))
	c.Equal(uintptr(0), button.release())
	c.Equal(uintptr(0), field.release())
}

// TestUIAPatternUnsupported verifies that a pattern method refuses on an element that does not support the pattern. It
// happens for real when a snapshot is published between the QueryInterface that handed a client the interface and the
// call it makes on it: a slider that became a label has nothing to say about a range, and saying something anyway would
// mean answering from a node of another role entirely.
func TestUIAPatternUnsupported(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	number, numberAddress := uiaOut[float64](&pin)
	str, strAddress := uiaOut[BSTR](&pin)
	w := newTestUIAWindow(patternTree())
	slider := w.providerFor(11).ifacePtr(uiaIfaceRangeValue)
	c.Equal(COM_S_OK, uiaRangeValueValue(slider, numberAddress))
	c.Equal(5.0, *number)

	relabeled := patternTree()
	relabeled.Nodes[11].Role = role.Label
	relabeled.Nodes[11].HasNumber = false
	relabeled.Generation = 2
	w.Publish(relabeled, nil)

	c.Equal(UIA_E_NOTSUPPORTED, uiaRangeValueValue(slider, numberAddress))
	c.Equal(0.0, *number)
	c.Equal(UIA_E_NOTSUPPORTED, uiaRangeValueSetValue(slider, uintptr(math.Float64bits(3))))
	c.Equal(0, len(w.recorded()))

	// The same guard is what a method reached through the wrong interface runs into, since the interfaces a provider
	// hands out are decided from the same table.
	c.Equal(UIA_E_NOTSUPPORTED, uiaValueValue(w.providerFor(2).ifacePtr(uiaIfaceValue), strAddress))
	c.Equal(BSTR(0), *str)
	c.Equal(UIA_E_NOTSUPPORTED, uiaInvokeInvoke(w.providerFor(17).ifacePtr(uiaIfaceInvoke)))
}

// TestUIAPatternNullOutParameters verifies that every pattern method that reports something checks its out-parameter
// before anything else. UI Automation does not pass NULL, but a provider that trusts that crashes the client's process
// rather than its own.
func TestUIAPatternNullOutParameters(t *testing.T) {
	c := check.New(t)
	w := newTestUIAWindow(tableTree())
	root := w.rootProvider()
	c.NotNil(root)
	for _, method := range []struct {
		call  func(this, out uintptr) uint64
		name  string
		iface uiaIface
	}{
		{name: "IToggleProvider::get_ToggleState", iface: uiaIfaceToggle, call: uiaToggleState},
		{name: "IValueProvider::get_Value", iface: uiaIfaceValue, call: uiaValueValue},
		{name: "IValueProvider::get_IsReadOnly", iface: uiaIfaceValue, call: uiaValueIsReadOnly},
		{name: "IRangeValueProvider::get_Value", iface: uiaIfaceRangeValue, call: uiaRangeValueValue},
		{name: "IRangeValueProvider::get_IsReadOnly", iface: uiaIfaceRangeValue, call: uiaRangeValueIsReadOnly},
		{name: "IRangeValueProvider::get_Maximum", iface: uiaIfaceRangeValue, call: uiaRangeValueMaximum},
		{name: "IRangeValueProvider::get_Minimum", iface: uiaIfaceRangeValue, call: uiaRangeValueMinimum},
		{name: "IRangeValueProvider::get_LargeChange", iface: uiaIfaceRangeValue, call: uiaRangeValueLargeChange},
		{name: "IRangeValueProvider::get_SmallChange", iface: uiaIfaceRangeValue, call: uiaRangeValueSmallChange},
		{name: "ISelectionProvider::GetSelection", iface: uiaIfaceSelection, call: uiaSelectionGetSelection},
		{
			name: "ISelectionProvider::get_CanSelectMultiple", iface: uiaIfaceSelection,
			call: uiaSelectionCanSelectMultiple,
		},
		{
			name: "ISelectionProvider::get_IsSelectionRequired", iface: uiaIfaceSelection,
			call: uiaSelectionIsSelectionRequired,
		},
		{
			name: "ISelectionItemProvider::get_IsSelected", iface: uiaIfaceSelectionItem,
			call: uiaSelectionItemIsSelected,
		},
		{
			name: "ISelectionItemProvider::get_SelectionContainer", iface: uiaIfaceSelectionItem,
			call: uiaSelectionItemSelectionContainer,
		},
		{
			name: "IExpandCollapseProvider::get_ExpandCollapseState", iface: uiaIfaceExpandCollapse,
			call: uiaExpandCollapseState,
		},
		{name: "IGridProvider::get_RowCount", iface: uiaIfaceGrid, call: uiaGridRowCount},
		{name: "IGridProvider::get_ColumnCount", iface: uiaIfaceGrid, call: uiaGridColumnCount},
		{
			name: "IGridProvider::GetItem", iface: uiaIfaceGrid,
			call: func(this, out uintptr) uint64 { return uiaGridGetItem(this, 0, 0, out) },
		},
		{name: "IGridItemProvider::get_Row", iface: uiaIfaceGridItem, call: uiaGridItemRow},
		{name: "IGridItemProvider::get_Column", iface: uiaIfaceGridItem, call: uiaGridItemColumn},
		{name: "IGridItemProvider::get_RowSpan", iface: uiaIfaceGridItem, call: uiaGridItemRowSpan},
		{name: "IGridItemProvider::get_ColumnSpan", iface: uiaIfaceGridItem, call: uiaGridItemColumnSpan},
		{name: "IGridItemProvider::get_ContainingGrid", iface: uiaIfaceGridItem, call: uiaGridItemContainingGrid},
		{name: "ITableProvider::GetRowHeaders", iface: uiaIfaceTable, call: uiaTableGetRowHeaders},
		{name: "ITableProvider::GetColumnHeaders", iface: uiaIfaceTable, call: uiaTableGetColumnHeaders},
		{name: "ITableProvider::get_RowOrColumnMajor", iface: uiaIfaceTable, call: uiaTableRowOrColumnMajor},
		{name: "ITableItemProvider::GetRowHeaderItems", iface: uiaIfaceTableItem, call: uiaTableItemGetRowHeaderItems},
		{
			name: "ITableItemProvider::GetColumnHeaderItems", iface: uiaIfaceTableItem,
			call: uiaTableItemGetColumnHeaderItems,
		},
	} {
		c.Equal(COM_E_POINTER, method.call(root.ifacePtr(method.iface), 0), method.name)
	}
}
