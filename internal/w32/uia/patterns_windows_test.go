// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

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
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// These tests stand in for the UI Automation client the control patterns are built for, the way the ones in
// provider_windows_test.go do: they call the Go functions the vtable slots hold, with the this pointer adjusted to
// the pattern interface each one belongs to. What the answers are supposed to be is settled by the portable tests in
// patterns_test.go; these check the COM around them — the out-parameters, the HRESULTs, the action requests that
// reach the window, and who owns each BSTR and SAFEARRAY.

// requestAt returns one of the action requests a test window has recorded, or the zero request when it has recorded
// fewer than that, so that a wrong expectation is reported rather than panicking.
func requestAt(w *testWindow, i int) accessibility.ActionRequest {
	requests := w.recorded()
	if i < 0 || i >= len(requests) {
		return accessibility.ActionRequest{}
	}
	return requests[i]
}

// safeArrayUnknowns returns the interface pointers held in a VT_UNKNOWN SAFEARRAY, releasing the reference that
// reading each one added, so that the array is left holding exactly what it was handed.
func safeArrayUnknowns(c check.Checker, array SAFEARRAY) []uintptr {
	c.True(array != 0, "the array must not be NULL")
	if array == 0 {
		return nil
	}
	bound, hr := SafeArrayGetUBound(array, 1)
	c.True(w32.HResultSucceeded(hr))
	if bound < 0 {
		return nil
	}
	pointers := make([]uintptr, 0, bound+1)
	for i := int32(0); i <= bound; i++ {
		var element uintptr
		c.True(w32.HResultSucceeded(SafeArrayGetElement(array, i, unsafe.Pointer(&element))))
		pointers = append(pointers, element)
		if element != 0 {
			providerFromThis(element, ifaceSimple).release()
		}
	}
	return pointers
}

// TestPatternVtblSlotOrder verifies that method N of each control-pattern interface really sits in slot N of that
// interface's virtual method table. No other test here can: every other one calls the Go functions the slots were built
// from, and those answer the same however the slots are ordered, so two methods of the same shape swapped —
// gridItemRow for gridItemColumn, say — would pass the whole suite while a real client got one answer where it
// asked for the other. These call through the table with syscall.SyscallN, the way UI Automation does.
//
// See TestVtblSlotOrder, which does the same for the provider and window-level interfaces and explains why the two
// slots holding assembly thunks — IRangeValueProvider::SetValue among them — are left out.
func TestPatternVtblSlotOrder(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	w := newTestWindow(t, patternTree())

	// IInvokeProvider: Invoke.
	c.Equal(w32.COM_S_OK, callSlot(w.providerFor(2), ifaceInvoke, 0))
	c.Equal(1, len(w.recorded()), "Invoke")
	c.Equal(accessibility.Press, requestAt(w, 0).Action)

	// IToggleProvider: Toggle, get_ToggleState. Node 4 is a check box with a mixed check.
	toggle := w.providerFor(4)
	c.Equal(w32.COM_S_OK, callSlot(toggle, ifaceToggle, 0))
	c.Equal(2, len(w.recorded()), "Toggle")
	c.Equal(accessibility.Toggle, requestAt(w, 1).Action)
	c.Equal(w32.COM_S_OK, callSlot(toggle, ifaceToggle, 1, out.fresh()))
	c.Equal(ToggleState_Indeterminate, ToggleState(out.i32()), "get_ToggleState")

	// IValueProvider: SetValue, get_Value, get_IsReadOnly. All three take one argument beyond the this pointer, so the
	// two getters are read first, while what the field holds is still what the snapshot says.
	value := w.providerFor(8)
	c.Equal(w32.COM_S_OK, callSlot(value, ifaceValue, 1, out.fresh()))
	c.Equal("Gandalf", BSTRToString(BSTR(out.ptr())), "get_Value")
	BSTR(out.ptr()).Free()
	c.Equal(w32.COM_S_OK, callSlot(value, ifaceValue, 2, out.fresh()))
	c.Equal(int32(0), out.i32(), "get_IsReadOnly")
	text, err := windows.UTF16PtrFromString("Frodo")
	c.NoError(err)
	pin.Pin(text)
	c.Equal(w32.COM_S_OK, callSlot(value, ifaceValue, 0, uintptr(unsafe.Pointer(text))))
	c.Equal(3, len(w.recorded()), "SetValue")
	c.Equal("Frodo", requestAt(w, 2).Value)

	// IExpandCollapseProvider: Expand, Collapse, get_ExpandCollapseState. Node 14 is a collapsed popup button, and the
	// two methods that take nothing but the this pointer are told apart by the action each asks the window for.
	popup := w.providerFor(14)
	c.Equal(w32.COM_S_OK, callSlot(popup, ifaceExpandCollapse, 0))
	c.Equal(accessibility.Expand, requestAt(w, 3).Action, "Expand")
	c.Equal(w32.COM_S_OK, callSlot(popup, ifaceExpandCollapse, 1))
	c.Equal(accessibility.Collapse, requestAt(w, 4).Action, "Collapse")
	c.Equal(5, len(w.recorded()))
	c.Equal(w32.COM_S_OK, callSlot(popup, ifaceExpandCollapse, 2, out.fresh()))
	c.Equal(ExpandCollapseState_Collapsed, ExpandCollapseState(out.i32()), "get_ExpandCollapseState")

	checkRangeValueSlotOrder(t, c, out)
	checkSelectionSlotOrder(t, c, out)
	checkTableSlotOrder(t, c, out)
}

// checkRangeValueSlotOrder verifies the slot order of IRangeValueProvider: SetValue, get_Value, get_IsReadOnly,
// get_Maximum, get_Minimum, get_LargeChange, get_SmallChange. Every one of the six getters takes nothing but an
// out-parameter, so the slider is given a range whose six answers are all different from one another: a value of 5 over
// -1 to 10, a step of 2 — which makes the large change 20 — and no SetValue action, which makes it read-only.
func checkRangeValueSlotOrder(t *testing.T, c check.Checker, out *slotOut) {
	t.Helper()
	tree := patternTree()
	tree.Nodes[11].Min = -1
	tree.Nodes[11].Step = 2
	tree.Nodes[11].Actions = accessibility.ActionSet(0).With(accessibility.Increment, accessibility.Decrement)
	slider := newTestWindow(t, tree).providerFor(11)
	c.Equal(w32.COM_S_OK, callSlot(slider, ifaceRangeValue, 1, out.fresh()))
	c.Equal(5.0, out.f64(), "get_Value")
	c.Equal(w32.COM_S_OK, callSlot(slider, ifaceRangeValue, 2, out.fresh()))
	c.Equal(int32(1), out.i32(), "get_IsReadOnly")
	c.Equal(w32.COM_S_OK, callSlot(slider, ifaceRangeValue, 3, out.fresh()))
	c.Equal(10.0, out.f64(), "get_Maximum")
	c.Equal(w32.COM_S_OK, callSlot(slider, ifaceRangeValue, 4, out.fresh()))
	c.Equal(-1.0, out.f64(), "get_Minimum")
	c.Equal(w32.COM_S_OK, callSlot(slider, ifaceRangeValue, 5, out.fresh()))
	c.Equal(20.0, out.f64(), "get_LargeChange")
	c.Equal(w32.COM_S_OK, callSlot(slider, ifaceRangeValue, 6, out.fresh()))
	c.Equal(2.0, out.f64(), "get_SmallChange")
}

// checkSelectionSlotOrder verifies the slot order of ISelectionProvider — GetSelection, get_CanSelectMultiple,
// get_IsSelectionRequired — of ISelectionItemProvider — Select, AddToSelection, RemoveFromSelection, get_IsSelected,
// get_SelectionContainer — and of IScrollItemProvider's one method. The three that take nothing but the this pointer
// are told apart by the action each asks the window for.
func checkSelectionSlotOrder(t *testing.T, c check.Checker, out *slotOut) {
	t.Helper()
	w := newTestWindow(t, listTree())
	list := w.providerFor(2)
	c.Equal(w32.COM_S_OK, callSlot(list, ifaceSelection, 0, out.fresh()))
	c.Equal(2, len(safeArrayUnknowns(c, out.array())), "GetSelection")
	out.array().Destroy()
	c.Equal(w32.COM_S_OK, callSlot(list, ifaceSelection, 1, out.fresh()))
	c.Equal(int32(1), out.i32(), "get_CanSelectMultiple")
	c.Equal(w32.COM_S_OK, callSlot(list, ifaceSelection, 2, out.fresh()))
	c.Equal(int32(0), out.i32(), "get_IsSelectionRequired")

	selected := w.providerFor(3)
	c.Equal(w32.COM_S_OK, callSlot(selected, ifaceSelectionItem, 3, out.fresh()))
	c.Equal(int32(1), out.i32(), "get_IsSelected")
	c.Equal(w32.COM_S_OK, callSlot(selected, ifaceSelectionItem, 4, out.fresh()))
	c.Equal(list.ifacePtr(ifaceSimple), out.ptr(), "get_SelectionContainer")
	c.Equal(uintptr(1), list.release())

	other := w.providerFor(6)
	for i, action := range []accessibility.Action{
		accessibility.Select, accessibility.AddToSelection, accessibility.RemoveFromSelection,
	} {
		c.Equal(w32.COM_S_OK, callSlot(other, ifaceSelectionItem, i))
		c.Equal(i+1, len(w.recorded()))
		c.Equal(action, requestAt(w, i).Action, "selection item method %d", i)
	}

	c.Equal(w32.COM_S_OK, callSlot(selected, ifaceScrollItem, 0))
	c.Equal(4, len(w.recorded()))
	c.Equal(accessibility.ScrollIntoView, requestAt(w, 3).Action, "ScrollIntoView")
}

// checkTableSlotOrder verifies the slot order of IGridProvider — GetItem, get_RowCount, get_ColumnCount — of
// IGridItemProvider — get_Row, get_Column, get_RowSpan, get_ColumnSpan, get_ContainingGrid — of ITableProvider —
// GetRowHeaders, GetColumnHeaders, get_RowOrColumnMajor — and of ITableItemProvider's GetRowHeaderItems and
// GetColumnHeaderItems.
//
// Cell 11 sits at row 2 of column 0, so get_Row and get_Column answer differently. The two spans are both the constant
// one, so nothing can tell them apart from each other — and nothing would go wrong if they were swapped.
func checkTableSlotOrder(t *testing.T, c check.Checker, out *slotOut) {
	t.Helper()
	w := newTestWindow(t, tableTree())
	grid := w.providerFor(6)
	c.Equal(w32.COM_S_OK, callSlot(grid, ifaceGrid, 0, 1, 0, out.fresh()))
	cell := w.providerFor(8)
	c.Equal(cell.ifacePtr(ifaceSimple), out.ptr(), "GetItem")
	c.Equal(uintptr(1), cell.release())
	c.Equal(w32.COM_S_OK, callSlot(grid, ifaceGrid, 1, out.fresh()))
	c.Equal(int32(5), out.i32(), "get_RowCount")
	c.Equal(w32.COM_S_OK, callSlot(grid, ifaceGrid, 2, out.fresh()))
	c.Equal(int32(2), out.i32(), "get_ColumnCount")

	item := w.providerFor(11)
	c.Equal(w32.COM_S_OK, callSlot(item, ifaceGridItem, 0, out.fresh()))
	c.Equal(int32(2), out.i32(), "get_Row")
	c.Equal(w32.COM_S_OK, callSlot(item, ifaceGridItem, 1, out.fresh()))
	c.Equal(int32(0), out.i32(), "get_Column")
	c.Equal(w32.COM_S_OK, callSlot(item, ifaceGridItem, 2, out.fresh()))
	c.Equal(int32(1), out.i32(), "get_RowSpan")
	c.Equal(w32.COM_S_OK, callSlot(item, ifaceGridItem, 3, out.fresh()))
	c.Equal(int32(1), out.i32(), "get_ColumnSpan")
	c.Equal(w32.COM_S_OK, callSlot(item, ifaceGridItem, 4, out.fresh()))
	c.Equal(grid.ifacePtr(ifaceSimple), out.ptr(), "get_ContainingGrid")
	c.Equal(uintptr(1), grid.release())

	c.Equal(w32.COM_S_OK, callSlot(grid, ifaceTable, 0, out.fresh()))
	c.Nil(safeArrayUnknowns(c, out.array()), "GetRowHeaders: a unison table has none")
	out.array().Destroy()
	c.Equal(w32.COM_S_OK, callSlot(grid, ifaceTable, 1, out.fresh()))
	c.Equal(2, len(safeArrayUnknowns(c, out.array())), "GetColumnHeaders")
	out.array().Destroy()
	c.Equal(w32.COM_S_OK, callSlot(grid, ifaceTable, 2, out.fresh()))
	c.Equal(RowOrColumnMajor_RowMajor, RowOrColumnMajor(out.i32()), "get_RowOrColumnMajor")

	c.Equal(w32.COM_S_OK, callSlot(item, ifaceTableItem, 0, out.fresh()))
	c.Nil(safeArrayUnknowns(c, out.array()), "GetRowHeaderItems: a unison cell belongs to none")
	out.array().Destroy()
	c.Equal(w32.COM_S_OK, callSlot(item, ifaceTableItem, 1, out.fresh()))
	c.Equal(1, len(safeArrayUnknowns(c, out.array())), "GetColumnHeaderItems")
	out.array().Destroy()
}

// TestInvokePattern verifies the one method of the Invoke pattern: the press a client asks for reaches the window as
// an action, a disabled element refuses, and an element with nowhere to send the request says so rather than reporting
// success.
func TestInvokePattern(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, patternTree())

	c.Equal(w32.COM_S_OK, invokeInvoke(w.providerFor(2).ifacePtr(ifaceInvoke)))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(2), requestAt(w, 0).Node)
	c.Equal(accessibility.Press, requestAt(w, 0).Action)

	// Node 3 is a disabled button. It still supports the pattern — a client has to be able to describe it — but nothing
	// acts on it.
	c.Equal(E_ELEMENTNOTENABLED, invokeInvoke(w.providerFor(3).ifacePtr(ifaceInvoke)))
	c.Equal(1, len(w.recorded()))

	plain := newActionlessWindow(t, patternTree())
	c.Equal(E_INVALIDOPERATION, invokeInvoke(plain.providerFor(2).ifacePtr(ifaceInvoke)))

	// An element that carries the pattern but offers no Press refuses too. A column header is the real case: it is
	// given the Invoke pattern whether or not its table can be sorted by it, so that a client describes it as a header
	// either way, and TableHeader.PerformAccessibilityAction turns down a press on one that cannot sort. Answering
	// S_OK would tell the client the header had been activated while nothing happened at all.
	headers := newTestWindow(t, tableTree())
	c.Equal(w32.COM_S_OK, invokeInvoke(headers.providerFor(4).ifacePtr(ifaceInvoke)))
	c.Equal(1, len(headers.recorded()))
	unsortable := tableTree()
	unsortable.Nodes[4].Actions = 0
	unsortable.Generation = 2
	headers.Publish(unsortable, nil)
	c.Equal(E_INVALIDOPERATION, invokeInvoke(headers.providerFor(4).ifacePtr(ifaceInvoke)))
	c.Equal(1, len(headers.recorded()))
}

// TestInvokeRaisesInvoked verifies that Invoke reports the invocation with Invoke_InvokedEventId on the element
// it pressed. The interface requires it, and this is the only place it can come from: pressing something need not
// change the snapshot at all, so there may be no publish to carry the news and a client waiting on Invoked would wait
// forever.
func TestInvokeRaisesInvoked(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, patternTree())
	// Both windows are created before the recorder, since each of them silences UI Automation for the rest of the test:
	// the recorder has to be the last hook installed for anything raised below to reach it.
	plain := newActionlessWindow(t, patternTree())
	r := record(t, true) // Installed after the windows, whose own hook says nobody is listening.
	button := w.providerFor(2)
	c.NotNil(button)

	c.Equal(w32.COM_S_OK, invokeInvoke(button.ifacePtr(ifaceInvoke)))
	c.Equal(1, len(w.recorded()))
	c.Equal(1, len(r.raises))
	c.Equal(RaiseEvent, r.raises[0].Kind)
	c.Equal(Invoke_InvokedEventId, r.raises[0].Event)
	c.Equal(button.Unknown(), r.raises[0].Provider, "the event names the element that was invoked")

	// A refused invocation reports nothing. Node 3 is disabled, a window with nowhere to send the request cannot have
	// carried it out either, and an element that offers no Press never had anything to invoke — which is the case the
	// event matters most for, since a client waiting on Invoked would otherwise be told the press had happened.
	c.Equal(E_ELEMENTNOTENABLED, invokeInvoke(w.providerFor(3).ifacePtr(ifaceInvoke)))
	c.Equal(E_INVALIDOPERATION, invokeInvoke(plain.providerFor(2).ifacePtr(ifaceInvoke)))
	pressless := patternTree()
	pressless.Nodes[2].Actions = 0
	pressless.Generation = 2
	w.Publish(pressless, nil)
	c.Equal(E_INVALIDOPERATION, invokeInvoke(button.ifacePtr(ifaceInvoke)))
	c.Equal(1, len(r.raises))
	w.Publish(patternTree(), nil)

	// Like every other raise this package makes, it costs nothing while no client is listening.
	quiet := record(t, false)
	c.Equal(w32.COM_S_OK, invokeInvoke(button.ifacePtr(ifaceInvoke)))
	c.Equal(2, len(w.recorded()))
	c.Nil(quiet.raises)
}

// TestTogglePattern verifies the Toggle pattern, including that a toggle button reports whether it is pressed while
// everything else checkable reports its check state.
func TestTogglePattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	state, stateAddress := pinnedOut[ToggleState](&pin)
	tree := patternTree()
	w := newTestWindow(t, tree)

	// Node 4 is a check box with a mixed check; node 5 a pressed toggle button.
	c.Equal(w32.COM_S_OK, toggleState(w.providerFor(4).ifacePtr(ifaceToggle), stateAddress))
	c.Equal(ToggleState_Indeterminate, *state)
	c.Equal(w32.COM_S_OK, toggleState(w.providerFor(5).ifacePtr(ifaceToggle), stateAddress))
	c.Equal(ToggleState_On, *state)

	next := patternTree()
	next.Nodes[4].Checked = checkenum.Off
	next.Generation = 2
	w.Publish(next, nil)
	c.Equal(w32.COM_S_OK, toggleState(w.providerFor(4).ifacePtr(ifaceToggle), stateAddress))
	c.Equal(ToggleState_Off, *state)

	c.Equal(w32.COM_S_OK, toggleToggle(w.providerFor(4).ifacePtr(ifaceToggle)))
	c.Equal(accessibility.NodeID(4), requestAt(w, 0).Node)
	c.Equal(accessibility.Toggle, requestAt(w, 0).Action)
	c.Equal(1, len(w.recorded()))
}

// TestTogglePressFallback verifies that Toggle presses an element that reports the Toggle pattern but offers only
// the Press action, and refuses one that offers neither. A sticky or grouped button is reported as a toggle button,
// because the state a click leaves it in is what a client has to hear, and Toggle is then the only way a client can
// operate it: the role carries no Invoke pattern, so dispatching an action the button ignores would answer S_OK while
// nothing happened.
func TestTogglePressFallback(t *testing.T) {
	c := check.New(t)
	tree := patternTree()
	tree.Nodes[5].Actions = accessibility.ActionSet(0).With(accessibility.Press)
	w := newTestWindow(t, tree)

	c.Equal(w32.COM_S_OK, toggleToggle(w.providerFor(5).ifacePtr(ifaceToggle)))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(5), requestAt(w, 0).Node)
	c.Equal(accessibility.Press, requestAt(w, 0).Action)

	// An element that offers neither is refused rather than asked to do something it has said it will not do, which is
	// what every other write path does with one. Nothing follows a toggle for a client to notice, so answering S_OK
	// would leave it believing the element had changed.
	neither := patternTree()
	neither.Nodes[5].Actions = 0
	neither.Generation = 2
	w.Publish(neither, nil)
	c.Equal(E_INVALIDOPERATION, toggleToggle(w.providerFor(5).ifacePtr(ifaceToggle)))
	c.Equal(1, len(w.recorded()))

	// A disabled element refuses either way, and says so as not enabled rather than as unsupported: being unusable now
	// is a different thing from never offering the action.
	disabled := patternTree()
	disabled.Nodes[5].Actions = accessibility.ActionSet(0).With(accessibility.Press)
	disabled.Nodes[5].Disabled = true
	disabled.Generation = 3
	w.Publish(disabled, nil)
	c.Equal(E_ELEMENTNOTENABLED, toggleToggle(w.providerFor(5).ifacePtr(ifaceToggle)))
	c.Equal(1, len(w.recorded()))
}

// TestValuePattern verifies the Value pattern: the text it reports, who owns the BSTR it reports it in, which
// elements refuse a new value, and how the UTF-16 string a client supplies is decoded.
func TestValuePattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	str, strAddress := pinnedOut[BSTR](&pin)
	boolean, booleanAddress := pinnedOut[int32](&pin)
	tree := patternTree()
	w := newTestWindow(t, tree)

	value := func(node accessibility.NodeID) string {
		c.Equal(w32.COM_S_OK, valueValue(w.providerFor(node).ifacePtr(ifaceValue), strAddress))
		// The BSTR belongs to the caller, which is UI Automation Core in the real thing and this test here.
		defer str.Free()
		return BSTRToString(*str)
	}
	readOnly := func(node accessibility.NodeID) bool {
		c.Equal(w32.COM_S_OK, valueIsReadOnly(w.providerFor(node).ifacePtr(ifaceValue), booleanAddress))
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
	c.Equal(w32.COM_S_OK, valueSetValue(w.providerFor(8).ifacePtr(ifaceValue), uintptr(unsafe.Pointer(text))))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(8), requestAt(w, 0).Node)
	c.Equal(accessibility.SetValue, requestAt(w, 0).Action)
	c.Equal("Frodo", requestAt(w, 0).Value)

	// A read-only element refuses; a disabled one refuses differently, so that a client can tell "never" from "not
	// now"; and a NULL string is a broken call rather than a request to empty the value.
	c.Equal(E_NOTSUPPORTED, valueSetValue(w.providerFor(10).ifacePtr(ifaceValue),
		uintptr(unsafe.Pointer(text))))
	c.Equal(w32.COM_E_INVALIDARG, valueSetValue(w.providerFor(8).ifacePtr(ifaceValue), 0))
	disabled := patternTree()
	disabled.Nodes[8].Disabled = true
	disabled.Generation = 2
	w.Publish(disabled, nil)
	c.Equal(E_ELEMENTNOTENABLED, valueSetValue(w.providerFor(8).ifacePtr(ifaceValue),
		uintptr(unsafe.Pointer(text))))
	c.Equal(1, len(w.recorded()))
}

// TestRangeValuePattern verifies the RangeValue pattern, whose value arrives and leaves as a double: the bounds and
// increments a client needs to move the value, and the rules on setting it.
func TestRangeValuePattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	number, numberAddress := pinnedOut[float64](&pin)
	boolean, booleanAddress := pinnedOut[int32](&pin)
	w := newTestWindow(t, patternTree())
	slider := w.providerFor(11).ifacePtr(ifaceRangeValue)

	double := func(call func(this, out uintptr) uint64, this uintptr) float64 {
		c.Equal(w32.COM_S_OK, call(this, numberAddress))
		return *number
	}
	c.Equal(5.0, double(rangeValueValue, slider))
	c.Equal(0.0, double(rangeValueMinimum, slider))
	c.Equal(10.0, double(rangeValueMaximum, slider))
	c.Equal(1.0, double(rangeValueSmallChange, slider), "one arrow key press")
	c.Equal(10.0, double(rangeValueLargeChange, slider), "one paging key press is ten of them")

	c.Equal(w32.COM_S_OK, rangeValueIsReadOnly(slider, booleanAddress))
	c.Equal(int32(0), *boolean)
	c.Equal(w32.COM_S_OK, rangeValueIsReadOnly(w.providerFor(13).ifacePtr(ifaceRangeValue), booleanAddress))
	c.Equal(int32(1), *boolean, "a progress bar is read-only")

	// The value arrives as the raw bits of a double, delivered by the assembly thunk the vtable slot holds; the thunk
	// itself is covered by TestRangeValueSetValueThunk.
	c.Equal(w32.COM_S_OK, rangeValueSetValue(slider, uintptr(math.Float64bits(7.5))))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(11), requestAt(w, 0).Node)
	c.Equal(accessibility.SetValue, requestAt(w, 0).Action)
	c.Equal(7.5, requestAt(w, 0).Number)
	// The same number as text, for a widget that takes its value that way. A numeric field is a text field underneath
	// and answers SetValue by replacing its text, so a request with only the number in it would blank the field.
	c.Equal("7.5", requestAt(w, 0).Value)

	// A value the element could never take, and one that is not a number at all, are refused rather than clamped.
	c.Equal(w32.COM_E_INVALIDARG, rangeValueSetValue(slider, uintptr(math.Float64bits(99))))
	c.Equal(w32.COM_E_INVALIDARG, rangeValueSetValue(slider, uintptr(math.Float64bits(-1))))
	c.Equal(w32.COM_E_INVALIDARG, rangeValueSetValue(slider, uintptr(math.Float64bits(math.NaN()))))
	c.Equal(E_NOTSUPPORTED, rangeValueSetValue(w.providerFor(13).ifacePtr(ifaceRangeValue),
		uintptr(math.Float64bits(5))))
	c.Equal(E_ELEMENTNOTENABLED, rangeValueSetValue(w.providerFor(12).ifacePtr(ifaceRangeValue),
		uintptr(math.Float64bits(2))))
	c.Equal(1, len(w.recorded()))
}

// TestSelectionPattern verifies the Selection pattern a container implements, including who owns the array it hands
// back and the references in it.
func TestSelectionPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	array, arrayAddress := pinnedOut[SAFEARRAY](&pin)
	boolean, booleanAddress := pinnedOut[int32](&pin)
	w := newTestWindow(t, listTree())
	list := w.providerFor(2).ifacePtr(ifaceSelection)

	// Nodes 3 and 5 are the selected list items, 5 of them through an ignored group.
	c.Equal(w32.COM_S_OK, selectionGetSelection(list, arrayAddress))
	c.Equal([]uintptr{
		w.providerFor(3).ifacePtr(ifaceSimple),
		w.providerFor(5).ifacePtr(ifaceSimple),
	}, safeArrayUnknowns(c, *array))

	// Storing an element into the array added a reference, and destroying the array — which UI Automation Core does —
	// gives it back.
	c.Equal(int32(2), atomic.LoadInt32(&w.providerFor(3).refCount))
	c.Equal(int32(2), atomic.LoadInt32(&w.providerFor(5).refCount))
	array.Destroy()
	c.Equal(int32(1), atomic.LoadInt32(&w.providerFor(3).refCount))
	c.Equal(int32(1), atomic.LoadInt32(&w.providerFor(5).refCount))

	c.Equal(w32.COM_S_OK, selectionCanSelectMultiple(list, booleanAddress))
	c.Equal(int32(1), *boolean)
	c.Equal(w32.COM_S_OK, selectionIsSelectionRequired(list, booleanAddress))
	c.Equal(int32(0), *boolean, "nothing unison builds insists on a selection")

	tabs := w.providerFor(7).ifacePtr(ifaceSelection)
	c.Equal(w32.COM_S_OK, selectionGetSelection(tabs, arrayAddress))
	c.Equal([]uintptr{w.providerFor(8).ifacePtr(ifaceSimple)}, safeArrayUnknowns(c, *array))
	array.Destroy()
	c.Equal(w32.COM_S_OK, selectionCanSelectMultiple(tabs, booleanAddress))
	c.Equal(int32(0), *boolean, "one tab at a time")

	// A container with nothing selected reports an empty array rather than a NULL one.
	empty := listTree()
	empty.Nodes[3].Selected = false
	empty.Nodes[5].Selected = false
	empty.Generation = 2
	w.Publish(empty, nil)
	c.Equal(w32.COM_S_OK, selectionGetSelection(list, arrayAddress))
	c.True(*array != 0)
	c.Nil(safeArrayUnknowns(c, *array))
	array.Destroy()

	// A table's selection is its rows.
	table := newTestWindow(t, tableTree())
	c.Equal(w32.COM_S_OK, selectionGetSelection(table.providerFor(6).ifacePtr(ifaceSelection), arrayAddress))
	c.Equal([]uintptr{table.providerFor(7).ifacePtr(ifaceSimple)}, safeArrayUnknowns(c, *array))
	array.Destroy()
}

// TestSelectionItemPattern verifies the SelectionItem pattern an item implements, including the radio button, which
// is a selection item with no container to point at and no way to be unchosen.
func TestSelectionItemPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	boolean, booleanAddress := pinnedOut[int32](&pin)
	w := newTestWindow(t, listTree())
	item := w.providerFor(3).ifacePtr(ifaceSelectionItem)
	other := w.providerFor(6).ifacePtr(ifaceSelectionItem)

	c.Equal(w32.COM_S_OK, selectionItemIsSelected(item, booleanAddress))
	c.Equal(int32(1), *boolean)
	c.Equal(w32.COM_S_OK, selectionItemIsSelected(other, booleanAddress))
	c.Equal(int32(0), *boolean)

	c.Equal(w32.COM_S_OK, selectionItemSelectionContainer(item, outAddress))
	c.Equal(w.providerFor(2).ifacePtr(ifaceSimple), *out)
	c.Equal(uintptr(1), w.providerFor(2).release(), "the container handed out is AddRef'd")

	c.Equal(w32.COM_S_OK, selectionItemSelect(other))
	c.Equal(w32.COM_S_OK, selectionItemAddToSelection(other))
	c.Equal(w32.COM_S_OK, selectionItemRemoveFromSelection(item))
	c.Equal(3, len(w.recorded()))
	c.Equal(accessibility.Select, requestAt(w, 0).Action)
	c.Equal(accessibility.NodeID(6), requestAt(w, 0).Node)
	c.Equal(accessibility.AddToSelection, requestAt(w, 1).Action)
	c.Equal(accessibility.RemoveFromSelection, requestAt(w, 2).Action)
	c.Equal(accessibility.NodeID(3), requestAt(w, 2).Node)

	// A radio button reports its check state as its selected state, is chosen by being pressed, cannot be unchosen, and
	// has no container to name: its group is a layout panel with no pattern of its own.
	radios := newTestWindow(t, patternTree())
	first := radios.providerFor(6).ifacePtr(ifaceSelectionItem)
	second := radios.providerFor(7).ifacePtr(ifaceSelectionItem)
	c.Equal(w32.COM_S_OK, selectionItemIsSelected(first, booleanAddress))
	c.Equal(int32(1), *boolean)
	c.Equal(w32.COM_S_OK, selectionItemIsSelected(second, booleanAddress))
	c.Equal(int32(0), *boolean)
	c.Equal(w32.COM_S_OK, selectionItemSelectionContainer(first, outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(w32.COM_S_OK, selectionItemSelect(second))
	c.Equal(w32.COM_S_OK, selectionItemAddToSelection(second))
	c.Equal(E_INVALIDOPERATION, selectionItemRemoveFromSelection(first))
	c.Equal(2, len(radios.recorded()))
	c.Equal(accessibility.Press, requestAt(radios, 0).Action)
	c.Equal(accessibility.NodeID(7), requestAt(radios, 0).Node)
	c.Equal(accessibility.Press, requestAt(radios, 1).Action)

	// A row is a selection item too, and a disabled one is not acted on.
	rows := newTestWindow(t, tableTree())
	c.Equal(w32.COM_S_OK, selectionItemSelectionContainer(rows.providerFor(7).ifacePtr(ifaceSelectionItem),
		outAddress))
	c.Equal(rows.providerFor(6).ifacePtr(ifaceSimple), *out)
	c.Equal(uintptr(1), rows.providerFor(6).release())
	disabled := tableTree()
	disabled.Nodes[7].Disabled = true
	disabled.Generation = 2
	rows.Publish(disabled, nil)
	c.Equal(E_ELEMENTNOTENABLED, selectionItemSelect(rows.providerFor(7).ifacePtr(ifaceSelectionItem)))
	c.Equal(0, len(rows.recorded()))
}

// TestSelectionItemRefusesImpossibleChanges verifies that the three methods that change what is selected are
// measured against what the snapshot says the element offers, as every other write path here is.
//
// Adding to or removing from a selection means nothing in a container that holds one selection at a time: the widget
// replaces the selection instead, so a client answered S_OK would have been told the opposite of what happened. UI
// Automation defines E_INVALIDOPERATION for that, and for an item that does not offer the action at all.
func TestSelectionItemRefusesImpossibleChanges(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, listTree())

	// Node 9 is a tab, and a tab list holds one selection at a time.
	tab := w.providerFor(9).ifacePtr(ifaceSelectionItem)
	c.Equal(E_INVALIDOPERATION, selectionItemAddToSelection(tab))
	c.Equal(E_INVALIDOPERATION, selectionItemRemoveFromSelection(tab))
	c.Equal(0, len(w.recorded()))
	c.Equal(w32.COM_S_OK, selectionItemSelect(tab), "choosing it outright is what a tab list does allow")
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(9), requestAt(w, 0).Node)
	c.Equal(accessibility.Select, requestAt(w, 0).Action)

	// Node 6 is a list item whose container allows several selections, so only the actions it does not offer are
	// refused.
	limited := listTree()
	limited.Nodes[6].Actions = accessibility.ActionSet(0).With(accessibility.Select)
	limited.Generation = 2
	w.Publish(limited, nil)
	item := w.providerFor(6).ifacePtr(ifaceSelectionItem)
	c.Equal(E_INVALIDOPERATION, selectionItemAddToSelection(item))
	c.Equal(E_INVALIDOPERATION, selectionItemRemoveFromSelection(item))
	c.Equal(1, len(w.recorded()))
	c.Equal(w32.COM_S_OK, selectionItemSelect(item))
	c.Equal(2, len(w.recorded()))
	c.Equal(accessibility.NodeID(6), requestAt(w, 1).Node)
}

// TestExpandCollapsePattern verifies the ExpandCollapse pattern, including that an element that cannot be expanded
// at all reports itself a leaf rather than collapsed: a client announces a leaf silently.
func TestExpandCollapsePattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	state, stateAddress := pinnedOut[ExpandCollapseState](&pin)
	w := newTestWindow(t, patternTree())
	popup := w.providerFor(14).ifacePtr(ifaceExpandCollapse)

	c.Equal(w32.COM_S_OK, expandCollapseState(popup, stateAddress))
	c.Equal(ExpandCollapseState_Collapsed, *state)
	c.Equal(w32.COM_S_OK, expandCollapseState(w.providerFor(15).ifacePtr(ifaceExpandCollapse), stateAddress))
	c.Equal(ExpandCollapseState_LeafNode, *state, "a combo box with nothing to drop down is a leaf")

	open := patternTree()
	open.Nodes[14].Expanded = true
	open.Generation = 2
	w.Publish(open, nil)
	c.Equal(w32.COM_S_OK, expandCollapseState(popup, stateAddress))
	c.Equal(ExpandCollapseState_Expanded, *state)

	c.Equal(w32.COM_S_OK, expandCollapseExpand(popup))
	c.Equal(w32.COM_S_OK, expandCollapseCollapse(popup))
	c.Equal(2, len(w.recorded()))
	c.Equal(accessibility.Expand, requestAt(w, 0).Action)
	c.Equal(accessibility.NodeID(14), requestAt(w, 0).Node)
	c.Equal(accessibility.Collapse, requestAt(w, 1).Action)

	// An expanded row in a hierarchical table is the other user of the pattern.
	rows := newTestWindow(t, tableTree())
	c.Equal(w32.COM_S_OK, expandCollapseState(rows.providerFor(7).ifacePtr(ifaceExpandCollapse), stateAddress))
	c.Equal(ExpandCollapseState_Expanded, *state)
	c.Equal(w32.COM_S_OK, expandCollapseCollapse(rows.providerFor(7).ifacePtr(ifaceExpandCollapse)))
	c.Equal(accessibility.Collapse, requestAt(rows, 0).Action)
	c.Equal(accessibility.NodeID(7), requestAt(rows, 0).Node)

	// A row behind a hierarchical filter reports that it is expanded — that is what the user sees — while offering
	// neither action, since the filter decides what is open and Table.axSetRowOpen refuses to change it. The pattern
	// still has to be there for the state to be read through, so the methods are what must refuse.
	filtered := tableTree()
	filtered.Nodes[7].Actions = filtered.Nodes[7].Actions.Without(accessibility.Expand, accessibility.Collapse)
	filtered.Generation = 2
	rows.Publish(filtered, nil)
	row := rows.providerFor(7).ifacePtr(ifaceExpandCollapse)
	c.Equal(w32.COM_S_OK, expandCollapseState(row, stateAddress))
	c.Equal(ExpandCollapseState_Expanded, *state)
	c.Equal(E_INVALIDOPERATION, expandCollapseCollapse(row))
	c.Equal(E_INVALIDOPERATION, expandCollapseExpand(row))
	c.Equal(1, len(rows.recorded()))
}

// TestScrollItemPattern verifies the one method of the ScrollItem pattern, which a client calls to bring an element
// it is about to talk about into view.
func TestScrollItemPattern(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, listTree())

	c.Equal(w32.COM_S_OK, scrollItemScrollIntoView(w.providerFor(3).ifacePtr(ifaceScrollItem)))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.NodeID(3), requestAt(w, 0).Node)
	c.Equal(accessibility.ScrollIntoView, requestAt(w, 0).Action)

	// A disabled element is the exception to the rule that nothing acts on one: ScrollIntoView is the single action a
	// disabled node goes on offering, and every row and item of a disabled table or list is published with it and
	// nothing else, so a screen reader stepping through one must still be able to bring its items on screen.
	disabled := listTree()
	disabled.Nodes[3].Disabled = true
	disabled.Nodes[3].Actions = accessibility.ActionSet(0).With(accessibility.ScrollIntoView)
	disabled.Generation = 2
	w.Publish(disabled, nil)
	c.Equal(w32.COM_S_OK, scrollItemScrollIntoView(w.providerFor(3).ifacePtr(ifaceScrollItem)))
	c.Equal(2, len(w.recorded()))
	c.Equal(accessibility.ScrollIntoView, requestAt(w, 1).Action)

	// A node that does not offer the action does not have the pattern either, so the interface is refused outright
	// rather than answering for something that cannot happen.
	fixed := listTree()
	fixed.Nodes[3].Actions = accessibility.ActionSet(0).With(accessibility.Select)
	fixed.Generation = 3
	w.Publish(fixed, nil)
	c.Equal(E_NOTSUPPORTED, scrollItemScrollIntoView(w.providerFor(3).ifacePtr(ifaceScrollItem)))
	c.Equal(2, len(w.recorded()))
}

// TestGridPattern verifies the Grid pattern a table implements, including the difference between a cell outside the
// grid — an error — and one the snapshot simply does not hold, which most of a large table's cells are.
func TestGridPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	count, countAddress := pinnedOut[int32](&pin)
	w := newTestWindow(t, tableTree())
	grid := w.providerFor(6).ifacePtr(ifaceGrid)

	c.Equal(w32.COM_S_OK, gridRowCount(grid, countAddress))
	c.Equal(int32(5), *count, "every row, not only the ones published")
	c.Equal(w32.COM_S_OK, gridColumnCount(grid, countAddress))
	c.Equal(int32(2), *count)

	c.Equal(w32.COM_S_OK, gridGetItem(grid, 1, 0, outAddress))
	c.Equal(w.providerFor(8).ifacePtr(ifaceSimple), *out)
	c.Equal(uintptr(1), w.providerFor(8).release(), "the cell handed out is AddRef'd")
	c.Equal(w32.COM_S_OK, gridGetItem(grid, 2, 1, outAddress))
	c.Equal(w.providerFor(12).ifacePtr(ifaceSimple), *out)
	c.Equal(uintptr(1), w.providerFor(12).release())

	// Row 0 is outside the viewport, so it was never published; asking for it is answered with nothing rather than with
	// an error.
	c.Equal(w32.COM_S_OK, gridGetItem(grid, 0, 0, outAddress))
	c.Equal(uintptr(0), *out)

	// A row or column the grid does not have at all is a broken request. A negative index arrives as every bit set,
	// which is what a client passing -1 looks like across the ABI.
	negative := ^uintptr(0)
	c.Equal(w32.COM_E_INVALIDARG, gridGetItem(grid, 5, 0, outAddress))
	c.Equal(w32.COM_E_INVALIDARG, gridGetItem(grid, 0, 2, outAddress))
	c.Equal(w32.COM_E_INVALIDARG, gridGetItem(grid, negative, 0, outAddress))
	c.Equal(w32.COM_E_INVALIDARG, gridGetItem(grid, 0, negative, outAddress))
	c.Equal(w32.COM_E_POINTER, gridGetItem(grid, 0, 0, 0))
}

// TestGridItemPattern verifies the GridItem pattern a cell implements.
func TestGridItemPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	index, indexAddress := pinnedOut[int32](&pin)
	w := newTestWindow(t, tableTree())
	cell := w.providerFor(9).ifacePtr(ifaceGridItem)

	c.Equal(w32.COM_S_OK, gridItemRow(cell, indexAddress))
	c.Equal(int32(1), *index)
	c.Equal(w32.COM_S_OK, gridItemColumn(cell, indexAddress))
	c.Equal(int32(1), *index)
	c.Equal(w32.COM_S_OK, gridItemRowSpan(cell, indexAddress))
	c.Equal(int32(1), *index, "nothing unison draws merges cells")
	c.Equal(w32.COM_S_OK, gridItemColumnSpan(cell, indexAddress))
	c.Equal(int32(1), *index)

	c.Equal(w32.COM_S_OK, gridItemContainingGrid(cell, outAddress))
	c.Equal(w.providerFor(6).ifacePtr(ifaceSimple), *out)
	c.Equal(uintptr(1), w.providerFor(6).release())
}

// TestTablePattern verifies the Table pattern a table implements on top of Grid, which is what adds the headers.
func TestTablePattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	array, arrayAddress := pinnedOut[SAFEARRAY](&pin)
	major, majorAddress := pinnedOut[RowOrColumnMajor](&pin)
	w := newTestWindow(t, tableTree())
	table := w.providerFor(6).ifacePtr(ifaceTable)

	c.Equal(w32.COM_S_OK, tableRowOrColumnMajor(table, majorAddress))
	c.Equal(RowOrColumnMajor_RowMajor, *major)

	// A unison table has no row headers, which is an empty array rather than a NULL one.
	c.Equal(w32.COM_S_OK, tableGetRowHeaders(table, arrayAddress))
	c.Nil(safeArrayUnknowns(c, *array))
	array.Destroy()

	// The column headers come from the TableHeader panel the table sits with, which is nowhere inside it.
	c.Equal(w32.COM_S_OK, tableGetColumnHeaders(table, arrayAddress))
	c.Equal([]uintptr{
		w.providerFor(4).ifacePtr(ifaceSimple),
		w.providerFor(5).ifacePtr(ifaceSimple),
	}, safeArrayUnknowns(c, *array))
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
	c.Equal(w32.COM_S_OK, tableGetColumnHeaders(table, arrayAddress))
	c.Nil(safeArrayUnknowns(c, *array))
	array.Destroy()
	c.Equal(w32.COM_E_POINTER, tableGetColumnHeaders(table, 0))
}

// TestTableItemPattern verifies the TableItem pattern a cell implements on top of GridItem, which is what lets a
// screen reader say the column name before the cell's contents.
func TestTableItemPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	array, arrayAddress := pinnedOut[SAFEARRAY](&pin)
	w := newTestWindow(t, tableTree())

	c.Equal(w32.COM_S_OK, tableItemGetColumnHeaderItems(w.providerFor(8).ifacePtr(ifaceTableItem), arrayAddress))
	c.Equal([]uintptr{w.providerFor(4).ifacePtr(ifaceSimple)}, safeArrayUnknowns(c, *array))
	array.Destroy()
	c.Equal(w32.COM_S_OK, tableItemGetColumnHeaderItems(w.providerFor(9).ifacePtr(ifaceTableItem), arrayAddress))
	c.Equal([]uintptr{w.providerFor(5).ifacePtr(ifaceSimple)}, safeArrayUnknowns(c, *array))
	array.Destroy()

	// There are no row headers for a cell to belong to.
	c.Equal(w32.COM_S_OK, tableItemGetRowHeaderItems(w.providerFor(8).ifacePtr(ifaceTableItem), arrayAddress))
	c.Nil(safeArrayUnknowns(c, *array))
	array.Destroy()
	c.Equal(w32.COM_E_POINTER, tableItemGetColumnHeaderItems(w.providerFor(8).ifacePtr(ifaceTableItem), 0))
}

// TestPatternStale verifies what a client holding a pattern interface for something that has been destroyed is told.
// Answering from the snapshot the element was last in would have a screen reader describing something that is no longer
// on screen.
func TestPatternStale(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	str, strAddress := pinnedOut[BSTR](&pin)
	state, stateAddress := pinnedOut[ToggleState](&pin)
	// Both buffers are seeded with something a client would notice, so that "the out-parameter was cleared" is a
	// claim that can fail: an out-parameter UI Automation hands a provider holds whatever was last in it.
	*str = BSTR(0xDEAD)
	*state = ToggleState_Indeterminate
	w := newTestWindow(t, patternTree())
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

	c.Equal(E_ELEMENTNOTAVAILABLE, invokeInvoke(button.ifacePtr(ifaceInvoke)))
	c.Equal(E_ELEMENTNOTAVAILABLE, valueValue(field.ifacePtr(ifaceValue), strAddress))
	c.Equal(BSTR(0), *str)
	c.Equal(E_ELEMENTNOTAVAILABLE, toggleState(button.ifacePtr(ifaceToggle), stateAddress))
	c.Equal(ToggleState_Off, *state)
	c.Equal(0, len(w.recorded()))
	c.Equal(uintptr(0), button.release())
	c.Equal(uintptr(0), field.release())
}

// TestPatternUnsupported verifies that a pattern method refuses on an element that does not support the pattern. It
// happens for real when a snapshot is published between the QueryInterface that handed a client the interface and the
// call it makes on it: a slider that became a label has nothing to say about a range, and saying something anyway would
// mean answering from a node of another role entirely.
func TestPatternUnsupported(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	number, numberAddress := pinnedOut[float64](&pin)
	str, strAddress := pinnedOut[BSTR](&pin)
	*str = BSTR(0xDEAD) // Seeded for the reason TestPatternStale gives.
	w := newTestWindow(t, patternTree())
	slider := w.providerFor(11).ifacePtr(ifaceRangeValue)
	c.Equal(w32.COM_S_OK, rangeValueValue(slider, numberAddress))
	c.Equal(5.0, *number)

	relabeled := patternTree()
	relabeled.Nodes[11].Role = role.Label
	relabeled.Nodes[11].HasNumber = false
	relabeled.Generation = 2
	w.Publish(relabeled, nil)

	c.Equal(E_NOTSUPPORTED, rangeValueValue(slider, numberAddress))
	c.Equal(0.0, *number)
	c.Equal(E_NOTSUPPORTED, rangeValueSetValue(slider, uintptr(math.Float64bits(3))))
	c.Equal(0, len(w.recorded()))

	// The same guard is what a method reached through the wrong interface runs into, since the interfaces a provider
	// hands out are decided from the same table.
	c.Equal(E_NOTSUPPORTED, valueValue(w.providerFor(2).ifacePtr(ifaceValue), strAddress))
	c.Equal(BSTR(0), *str)
	c.Equal(E_NOTSUPPORTED, invokeInvoke(w.providerFor(17).ifacePtr(ifaceInvoke)))
}

// TestPatternNullOutParameters verifies that every pattern method that reports something checks its out-parameter
// before anything else. UI Automation does not pass NULL, but a provider that trusts that crashes the client's process
// rather than its own.
func TestPatternNullOutParameters(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, tableTree())
	root := w.rootProvider()
	c.NotNil(root)
	for _, method := range []struct {
		call  func(this, out uintptr) uint64
		name  string
		iface iface
	}{
		{name: "IToggleProvider::get_ToggleState", iface: ifaceToggle, call: toggleState},
		{name: "IValueProvider::get_Value", iface: ifaceValue, call: valueValue},
		{name: "IValueProvider::get_IsReadOnly", iface: ifaceValue, call: valueIsReadOnly},
		{name: "IRangeValueProvider::get_Value", iface: ifaceRangeValue, call: rangeValueValue},
		{name: "IRangeValueProvider::get_IsReadOnly", iface: ifaceRangeValue, call: rangeValueIsReadOnly},
		{name: "IRangeValueProvider::get_Maximum", iface: ifaceRangeValue, call: rangeValueMaximum},
		{name: "IRangeValueProvider::get_Minimum", iface: ifaceRangeValue, call: rangeValueMinimum},
		{name: "IRangeValueProvider::get_LargeChange", iface: ifaceRangeValue, call: rangeValueLargeChange},
		{name: "IRangeValueProvider::get_SmallChange", iface: ifaceRangeValue, call: rangeValueSmallChange},
		{name: "ISelectionProvider::GetSelection", iface: ifaceSelection, call: selectionGetSelection},
		{
			name: "ISelectionProvider::get_CanSelectMultiple", iface: ifaceSelection,
			call: selectionCanSelectMultiple,
		},
		{
			name: "ISelectionProvider::get_IsSelectionRequired", iface: ifaceSelection,
			call: selectionIsSelectionRequired,
		},
		{
			name: "ISelectionItemProvider::get_IsSelected", iface: ifaceSelectionItem,
			call: selectionItemIsSelected,
		},
		{
			name: "ISelectionItemProvider::get_SelectionContainer", iface: ifaceSelectionItem,
			call: selectionItemSelectionContainer,
		},
		{
			name: "IExpandCollapseProvider::get_ExpandCollapseState", iface: ifaceExpandCollapse,
			call: expandCollapseState,
		},
		{name: "IGridProvider::get_RowCount", iface: ifaceGrid, call: gridRowCount},
		{name: "IGridProvider::get_ColumnCount", iface: ifaceGrid, call: gridColumnCount},
		{
			name: "IGridProvider::GetItem", iface: ifaceGrid,
			call: func(this, out uintptr) uint64 { return gridGetItem(this, 0, 0, out) },
		},
		{name: "IGridItemProvider::get_Row", iface: ifaceGridItem, call: gridItemRow},
		{name: "IGridItemProvider::get_Column", iface: ifaceGridItem, call: gridItemColumn},
		{name: "IGridItemProvider::get_RowSpan", iface: ifaceGridItem, call: gridItemRowSpan},
		{name: "IGridItemProvider::get_ColumnSpan", iface: ifaceGridItem, call: gridItemColumnSpan},
		{name: "IGridItemProvider::get_ContainingGrid", iface: ifaceGridItem, call: gridItemContainingGrid},
		{name: "ITableProvider::GetRowHeaders", iface: ifaceTable, call: tableGetRowHeaders},
		{name: "ITableProvider::GetColumnHeaders", iface: ifaceTable, call: tableGetColumnHeaders},
		{name: "ITableProvider::get_RowOrColumnMajor", iface: ifaceTable, call: tableRowOrColumnMajor},
		{name: "ITableItemProvider::GetRowHeaderItems", iface: ifaceTableItem, call: tableItemGetRowHeaderItems},
		{
			name: "ITableItemProvider::GetColumnHeaderItems", iface: ifaceTableItem,
			call: tableItemGetColumnHeaderItems,
		},
	} {
		c.Equal(w32.COM_E_POINTER, method.call(root.ifacePtr(method.iface), 0), method.name)
	}
}
