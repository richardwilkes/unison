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
	"strconv"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"golang.org/x/sys/windows"
)

// This file holds the twelve control-pattern interfaces a provider implements: their virtual method tables, the order
// of their slots, and the method bodies. A pattern interface is handed out only when UIAPatterns says the element
// supports that pattern, so a method here can rely on having been reached through an interface the element wanted —
// but only at the moment it was handed out, which is why every method checks again.
//
// Every answer comes from the window's immutable snapshot and every request to change something goes back to the root
// package as an action, to be run on the UI thread; see the comments at the top of uia_provider_windows.go. The
// question each method asks of the snapshot lives in uia_patterns.go, which is portable and tested on any platform, so
// what is left here is the COM: checking the out-parameters, converting to VARIANT-free out-parameter types, and
// getting the ownership of each BSTR and SAFEARRAY handed over right.
//
// The three answers a method can give instead of an answer:
//
//   - UIA_E_ELEMENTNOTAVAILABLE, when the element has left the tree. A client holding an element for something that has
//     been destroyed must be told that rather than given a stale answer.
//   - UIA_E_NOTSUPPORTED, when the element is still there but no longer supports the pattern. Snapshots are published
//     while clients hold interfaces, so a slider that became a label between QueryInterface and the call has to say so.
//     It is also the answer for asking a read-only element to take a new value, which keeps the disabled case — which
//     says nothing about whether the value could otherwise be set — distinguishable.
//   - UIA_E_ELEMENTNOTENABLED, when the element is disabled. Every pattern method that asks for something to happen
//     refuses on a disabled element, ScrollIntoView included: a disabled control is still described, still navigated to
//     and still reported, but nothing acts on it.

// uiaBuildPatternVtbls fills in the virtual method table of every control-pattern interface. The order of the methods
// in each call is the order the interface declares them in; see the interface declarations in the Windows SDK's
// uiautomationcore.idl, or §7.2 of the plan, which lists them.
func uiaBuildPatternVtbls() {
	uiaBuildVtbl(uiaIfaceInvoke, uiaInvokeVtbl[:],
		uiaInvokeInvoke,
	)
	uiaBuildVtbl(uiaIfaceToggle, uiaToggleVtbl[:],
		uiaToggleToggle,
		uiaToggleState,
	)
	uiaBuildVtbl(uiaIfaceValue, uiaValueVtbl[:],
		uiaValueSetValue,
		uiaValueValue,
		uiaValueIsReadOnly,
	)
	// IRangeValueProvider::SetValue takes a double, which syscall.NewCallback cannot describe, so its slot holds an
	// assembly thunk that moves the floating-point register into the integer argument register the callback reads. See
	// uia_thunk_windows.go, including what to do if a thunk ever misbehaves on a target.
	uiaRangeValueSetValueCallback = windows.NewCallback(uiaRangeValueSetValue)
	uiaBuildVtbl(uiaIfaceRangeValue, uiaRangeValueVtbl[:],
		uiaRangeValueSetValueSlot(),
		uiaRangeValueValue,
		uiaRangeValueIsReadOnly,
		uiaRangeValueMaximum,
		uiaRangeValueMinimum,
		uiaRangeValueLargeChange,
		uiaRangeValueSmallChange,
	)
	uiaBuildVtbl(uiaIfaceSelection, uiaSelectionVtbl[:],
		uiaSelectionGetSelection,
		uiaSelectionCanSelectMultiple,
		uiaSelectionIsSelectionRequired,
	)
	uiaBuildVtbl(uiaIfaceSelectionItem, uiaSelectionItemVtbl[:],
		uiaSelectionItemSelect,
		uiaSelectionItemAddToSelection,
		uiaSelectionItemRemoveFromSelection,
		uiaSelectionItemIsSelected,
		uiaSelectionItemSelectionContainer,
	)
	uiaBuildVtbl(uiaIfaceExpandCollapse, uiaExpandCollapseVtbl[:],
		uiaExpandCollapseExpand,
		uiaExpandCollapseCollapse,
		uiaExpandCollapseState,
	)
	uiaBuildVtbl(uiaIfaceScrollItem, uiaScrollItemVtbl[:],
		uiaScrollItemScrollIntoView,
	)
	uiaBuildVtbl(uiaIfaceGrid, uiaGridVtbl[:],
		uiaGridGetItem,
		uiaGridRowCount,
		uiaGridColumnCount,
	)
	uiaBuildVtbl(uiaIfaceGridItem, uiaGridItemVtbl[:],
		uiaGridItemRow,
		uiaGridItemColumn,
		uiaGridItemRowSpan,
		uiaGridItemColumnSpan,
		uiaGridItemContainingGrid,
	)
	uiaBuildVtbl(uiaIfaceTable, uiaTableVtbl[:],
		uiaTableGetRowHeaders,
		uiaTableGetColumnHeaders,
		uiaTableRowOrColumnMajor,
	)
	uiaBuildVtbl(uiaIfaceTableItem, uiaTableItemVtbl[:],
		uiaTableItemGetRowHeaderItems,
		uiaTableItemGetColumnHeaderItems,
	)
}

// uiaInvokeInvoke implements IInvokeProvider::Invoke, which is a client asking for the one thing the element does:
// pressing a button, following a link, choosing a menu item, sorting by a column header.
//
// The interface requires UIA_Invoke_InvokedEventId to be raised once the element has carried the action out, and this
// is the only place it can come from: pressing something need not change the snapshot at all — a button that opens a
// menu, or one whose handler does its work somewhere else entirely — so there may be no publish to carry the news, and
// a client waiting on Invoked would wait forever.
//
// It is raised as soon as the request has been accepted rather than once it has been carried out, because the outcome
// cannot be seen from here: the request is queued onto the UI thread and the callback the adapter was handed reports
// nothing back, deliberately, since UI Automation calls in on whichever thread it likes and must not be left waiting
// on a UI thread that may be inside a modal loop. That is the same optimism the S_OK answered here already carries.
func uiaInvokeInvoke(this uintptr) uint64 {
	hr := uiaPatternAction(this, uiaIfaceInvoke, accessibility.Press)
	if hr == COM_S_OK {
		uiaProviderFromThis(this, uiaIfaceInvoke).raiseEvent(UIA_Invoke_InvokedEventId)
	}
	return hr
}

// uiaToggleToggle implements IToggleProvider::Toggle, which moves a checkable element to its next state. Which state
// that is belongs to the widget: a two-state check box has one answer and a three-state check box another, and neither
// is decided here.
//
// Not everything that reports the Toggle pattern offers the Toggle action. A sticky or grouped button is reported as a
// toggle button, because the state a click leaves it in is what a client has to hear, but the only thing it does is be
// pressed. Toggle is the sole way a client can operate such an element on Windows — the role carries no Invoke pattern
// — so a node that does not offer Toggle but does offer Press is pressed instead. Dispatching an action the widget
// ignores would answer S_OK while the button never changed.
func uiaToggleToggle(this uintptr) uint64 {
	p, _, node, hr := uiaPatternNode(this, uiaIfaceToggle)
	if hr != COM_S_OK {
		return hr
	}
	if node.Disabled {
		return UIA_E_ELEMENTNOTENABLED
	}
	action := accessibility.Toggle
	if !node.Actions.Has(accessibility.Toggle) && node.Actions.Has(accessibility.Press) {
		action = accessibility.Press
	}
	return uiaDispatch(p, accessibility.ActionRequest{Action: action})
}

// uiaToggleState implements IToggleProvider::get_ToggleState.
func uiaToggleState(this, out uintptr) uint64 {
	return uiaPatternInt32(this, uiaIfaceToggle, out, func(n *accessibility.Node) int32 {
		return int32(UIAToggleState(n))
	})
}

// uiaValueSetValue implements IValueProvider::SetValue. The string arrives as a NUL-terminated UTF-16 sequence that
// belongs to UI Automation, so it is decoded into a Go string before the request is queued: the pointer is not valid
// once this method has returned, and the action runs later, on the UI thread.
func uiaValueSetValue(this, str uintptr) uint64 {
	p, _, node, hr := uiaPatternNode(this, uiaIfaceValue)
	if hr != COM_S_OK {
		return hr
	}
	if str == 0 {
		return COM_E_INVALIDARG
	}
	if node.Disabled {
		return UIA_E_ELEMENTNOTENABLED
	}
	if UIAIsValueReadOnly(node) {
		return UIA_E_NOTSUPPORTED
	}
	return uiaDispatch(p, accessibility.ActionRequest{
		Action: accessibility.SetValue,
		Value:  windows.UTF16PtrToString(xruntime.PtrFromUintptr[uint16](str)),
	})
}

// uiaValueValue implements IValueProvider::get_Value. The BSTR handed back belongs to UI Automation Core, which frees
// it, so nothing here does. An element with nothing to report answers with an empty string rather than a NULL BSTR:
// NULL is indistinguishable from an allocation failure to most clients.
func uiaValueValue(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	_, _, node, hr := uiaPatternNode(this, uiaIfaceValue)
	if hr != COM_S_OK {
		return hr
	}
	str := NewBSTR(UIAValueString(node))
	if str == 0 {
		return COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[BSTR](out) = str
	return COM_S_OK
}

// uiaValueIsReadOnly implements IValueProvider::get_IsReadOnly.
func uiaValueIsReadOnly(this, out uintptr) uint64 {
	return uiaPatternBOOL(this, uiaIfaceValue, out, UIAIsValueReadOnly)
}

// uiaRangeValueSetValue implements IRangeValueProvider::SetValue. It is reached through an assembly thunk, so the value
// arrives as the raw bits of the double rather than as a double; see uia_thunk_windows.go. A value outside the
// element's own range, or one that is not a number at all, is refused here rather than clamped by the widget, so that a
// client is told its request was wrong instead of silently getting something else.
//
// The request carries the number both ways: as Number, and as the text the same number reads as. A widget that takes
// its value as text — a numeric field is a text field underneath, and answers SetValue by replacing its text — would
// otherwise be handed an empty string and blank itself. The Cocoa adapter fills in both for the same reason.
func uiaRangeValueSetValue(this, valueBits uintptr) uint64 {
	p, _, node, hr := uiaPatternNode(this, uiaIfaceRangeValue)
	if hr != COM_S_OK {
		return hr
	}
	if node.Disabled {
		return UIA_E_ELEMENTNOTENABLED
	}
	if UIAIsRangeValueReadOnly(node) {
		return UIA_E_NOTSUPPORTED
	}
	number := math.Float64frombits(uint64(valueBits))
	if math.IsNaN(number) || (node.HasNumber && node.Max > node.Min && (number < node.Min || number > node.Max)) {
		return COM_E_INVALIDARG
	}
	return uiaDispatch(p, accessibility.ActionRequest{
		Action: accessibility.SetValue,
		Number: number,
		Value:  strconv.FormatFloat(number, 'g', -1, 64),
	})
}

// uiaRangeValueValue implements IRangeValueProvider::get_Value.
func uiaRangeValueValue(this, out uintptr) uint64 {
	return uiaPatternFloat64(this, uiaIfaceRangeValue, out, func(n *accessibility.Node) float64 { return n.Number })
}

// uiaRangeValueIsReadOnly implements IRangeValueProvider::get_IsReadOnly.
func uiaRangeValueIsReadOnly(this, out uintptr) uint64 {
	return uiaPatternBOOL(this, uiaIfaceRangeValue, out, UIAIsRangeValueReadOnly)
}

// uiaRangeValueMaximum implements IRangeValueProvider::get_Maximum.
func uiaRangeValueMaximum(this, out uintptr) uint64 {
	return uiaPatternFloat64(this, uiaIfaceRangeValue, out, func(n *accessibility.Node) float64 { return n.Max })
}

// uiaRangeValueMinimum implements IRangeValueProvider::get_Minimum.
func uiaRangeValueMinimum(this, out uintptr) uint64 {
	return uiaPatternFloat64(this, uiaIfaceRangeValue, out, func(n *accessibility.Node) float64 { return n.Min })
}

// uiaRangeValueLargeChange implements IRangeValueProvider::get_LargeChange.
func uiaRangeValueLargeChange(this, out uintptr) uint64 {
	return uiaPatternFloat64(this, uiaIfaceRangeValue, out, UIALargeChange)
}

// uiaRangeValueSmallChange implements IRangeValueProvider::get_SmallChange.
func uiaRangeValueSmallChange(this, out uintptr) uint64 {
	return uiaPatternFloat64(this, uiaIfaceRangeValue, out, UIASmallChange)
}

// uiaSelectionGetSelection implements ISelectionProvider::GetSelection. A container with nothing selected answers with
// an empty array rather than a NULL one, which is what says "nothing is selected" as opposed to "I cannot tell you".
func uiaSelectionGetSelection(this, out uintptr) uint64 {
	return uiaPatternProviderArray(this, uiaIfaceSelection, out,
		func(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
			return UIASelection(t, n.ID)
		})
}

// uiaSelectionCanSelectMultiple implements ISelectionProvider::get_CanSelectMultiple.
func uiaSelectionCanSelectMultiple(this, out uintptr) uint64 {
	return uiaPatternBOOL(this, uiaIfaceSelection, out, func(n *accessibility.Node) bool {
		return n.Multiselectable
	})
}

// uiaSelectionIsSelectionRequired implements ISelectionProvider::get_IsSelectionRequired. No container unison builds
// insists on holding a selection: a list, a table and a tab list can all be left with nothing selected, and a client
// that was told otherwise would report an empty one as a broken element.
func uiaSelectionIsSelectionRequired(this, out uintptr) uint64 {
	return uiaPatternBOOL(this, uiaIfaceSelection, out, func(_ *accessibility.Node) bool { return false })
}

// uiaSelectionItemSelect implements ISelectionItemProvider::Select, which makes this element the only selection in its
// container.
func uiaSelectionItemSelect(this uintptr) uint64 {
	return uiaSelectionItemAction(this, accessibility.Select)
}

// uiaSelectionItemAddToSelection implements ISelectionItemProvider::AddToSelection.
func uiaSelectionItemAddToSelection(this uintptr) uint64 {
	return uiaSelectionItemAction(this, accessibility.AddToSelection)
}

// uiaSelectionItemRemoveFromSelection implements ISelectionItemProvider::RemoveFromSelection.
func uiaSelectionItemRemoveFromSelection(this uintptr) uint64 {
	return uiaSelectionItemAction(this, accessibility.RemoveFromSelection)
}

// uiaSelectionItemAction answers one of the three SelectionItem methods that changes what is selected.
//
// A radio button is the exception the pattern exists to describe: it is a selection item whose container is a layout
// panel with no pattern of its own, so choosing it is pressing it, and it cannot be unchosen at all — a radio group is
// left without a selection only by the application. Select and AddToSelection therefore become Press, and
// RemoveFromSelection is refused.
//
// Everything else is measured against what the snapshot says the element offers, as every other write path here is.
// Adding to or removing from a selection means nothing in a container that holds one selection at a time: the widget
// would replace the selection instead, so a client that was answered S_OK would be told the opposite of what happened.
// Neither does an action the node does not list. UI Automation's answer for both is UIA_E_INVALIDOPERATION — the
// operation cannot be performed on this element — rather than UIA_E_NOTSUPPORTED, which would deny the whole pattern.
func uiaSelectionItemAction(this uintptr, action accessibility.Action) uint64 {
	p, tree, node, hr := uiaPatternNode(this, uiaIfaceSelectionItem)
	if hr != COM_S_OK {
		return hr
	}
	if node.Disabled {
		return UIA_E_ELEMENTNOTENABLED
	}
	switch {
	case node.Role == role.RadioButton:
		if action == accessibility.RemoveFromSelection {
			return UIA_E_INVALIDOPERATION
		}
		action = accessibility.Press
	case action != accessibility.Select && !uiaMultipleSelection(tree, node):
		return UIA_E_INVALIDOPERATION
	default:
	}
	if !node.Actions.Has(action) {
		return UIA_E_INVALIDOPERATION
	}
	return uiaDispatch(p, accessibility.ActionRequest{Action: action})
}

// uiaMultipleSelection reports whether the container a selection item belongs to holds more than one selection at a
// time, which is what decides whether AddToSelection and RemoveFromSelection mean anything for that item. The container
// is the one ISelectionItemProvider::get_SelectionContainer reports, so a client asking about it and a client acting on
// it are looking at the same element.
func uiaMultipleSelection(t *accessibility.Tree, n *accessibility.Node) bool {
	container := t.Node(UIASelectionContainer(t, n.ID))
	return container != nil && container.Multiselectable
}

// uiaSelectionItemIsSelected implements ISelectionItemProvider::get_IsSelected.
func uiaSelectionItemIsSelected(this, out uintptr) uint64 {
	return uiaPatternBOOL(this, uiaIfaceSelectionItem, out, UIAIsSelected)
}

// uiaSelectionItemSelectionContainer implements ISelectionItemProvider::get_SelectionContainer, answering with the
// nearest ancestor that supports the Selection pattern, or NULL when there is none — which is the answer for a radio
// button. See UIASelectionContainer.
func uiaSelectionItemSelectionContainer(this, out uintptr) uint64 {
	return uiaPatternProvider(this, uiaIfaceSelectionItem, out,
		func(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID {
			return UIASelectionContainer(t, n.ID)
		})
}

// uiaExpandCollapseExpand implements IExpandCollapseProvider::Expand.
func uiaExpandCollapseExpand(this uintptr) uint64 {
	return uiaPatternAction(this, uiaIfaceExpandCollapse, accessibility.Expand)
}

// uiaExpandCollapseCollapse implements IExpandCollapseProvider::Collapse.
func uiaExpandCollapseCollapse(this uintptr) uint64 {
	return uiaPatternAction(this, uiaIfaceExpandCollapse, accessibility.Collapse)
}

// uiaExpandCollapseState implements IExpandCollapseProvider::get_ExpandCollapseState.
func uiaExpandCollapseState(this, out uintptr) uint64 {
	return uiaPatternInt32(this, uiaIfaceExpandCollapse, out, func(n *accessibility.Node) int32 {
		return int32(UIAExpandCollapseState(n))
	})
}

// uiaScrollItemScrollIntoView implements IScrollItemProvider::ScrollIntoView, which a client calls to bring an element
// it is about to talk about into view.
func uiaScrollItemScrollIntoView(this uintptr) uint64 {
	return uiaPatternAction(this, uiaIfaceScrollItem, accessibility.ScrollIntoView)
}

// uiaGridGetItem implements IGridProvider::GetItem. A row or column outside the grid is an error, as UI Automation
// defines it, but a cell the snapshot simply does not hold is not: a table publishes only the rows in its viewport, so
// a client walking a large grid is answered with a NULL element for most of it.
func uiaGridGetItem(this, row, column, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	p, tree, node, hr := uiaPatternNode(this, uiaIfaceGrid)
	if hr != COM_S_OK {
		return hr
	}
	wantRow, wantColumn := int(int32(uint32(row))), int(int32(uint32(column)))
	if wantRow < 0 || wantColumn < 0 || (node.RowCount > 0 && wantRow >= node.RowCount) ||
		(node.ColumnCount > 0 && wantColumn >= node.ColumnCount) {
		return COM_E_INVALIDARG
	}
	if cell := p.window.Provider(UIAGridItem(tree, p.node, wantRow, wantColumn)); cell != nil {
		// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
		*xruntime.PtrFromUintptr[uintptr](out) = cell.ifacePtr(uiaIfaceSimple)
	}
	return COM_S_OK
}

// uiaGridRowCount implements IGridProvider::get_RowCount. It is the number of rows the grid holds, not the number the
// snapshot published: a client asking how large a table is wants the answer the scroll bar reflects.
func uiaGridRowCount(this, out uintptr) uint64 {
	return uiaPatternInt32(this, uiaIfaceGrid, out, func(n *accessibility.Node) int32 { return int32(n.RowCount) })
}

// uiaGridColumnCount implements IGridProvider::get_ColumnCount.
func uiaGridColumnCount(this, out uintptr) uint64 {
	return uiaPatternInt32(this, uiaIfaceGrid, out, func(n *accessibility.Node) int32 { return int32(n.ColumnCount) })
}

// uiaGridItemRow implements IGridItemProvider::get_Row.
func uiaGridItemRow(this, out uintptr) uint64 {
	return uiaPatternInt32(this, uiaIfaceGridItem, out, func(n *accessibility.Node) int32 { return int32(n.RowIndex) })
}

// uiaGridItemColumn implements IGridItemProvider::get_Column.
func uiaGridItemColumn(this, out uintptr) uint64 {
	return uiaPatternInt32(this, uiaIfaceGridItem, out, func(n *accessibility.Node) int32 {
		return int32(n.ColumnIndex)
	})
}

// uiaGridItemRowSpan implements IGridItemProvider::get_RowSpan. Nothing unison draws merges cells, so every cell
// occupies exactly one row and one column.
func uiaGridItemRowSpan(this, out uintptr) uint64 {
	return uiaPatternInt32(this, uiaIfaceGridItem, out, func(_ *accessibility.Node) int32 { return 1 })
}

// uiaGridItemColumnSpan implements IGridItemProvider::get_ColumnSpan.
func uiaGridItemColumnSpan(this, out uintptr) uint64 {
	return uiaPatternInt32(this, uiaIfaceGridItem, out, func(_ *accessibility.Node) int32 { return 1 })
}

// uiaGridItemContainingGrid implements IGridItemProvider::get_ContainingGrid, answering with the nearest ancestor that
// supports the Grid pattern.
func uiaGridItemContainingGrid(this, out uintptr) uint64 {
	return uiaPatternProvider(this, uiaIfaceGridItem, out,
		func(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID {
			return UIAContainingGrid(t, n.ID)
		})
}

// uiaTableGetRowHeaders implements ITableProvider::GetRowHeaders. A unison table has no row headers — its first column
// is an ordinary column — so the answer is an empty array.
func uiaTableGetRowHeaders(this, out uintptr) uint64 {
	return uiaPatternProviderArray(this, uiaIfaceTable, out, uiaNoNodes)
}

// uiaTableGetColumnHeaders implements ITableProvider::GetColumnHeaders. See UIATableColumnHeaders for how the header of
// a table is found, given that the two are separate panels.
func uiaTableGetColumnHeaders(this, out uintptr) uint64 {
	return uiaPatternProviderArray(this, uiaIfaceTable, out,
		func(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
			return UIATableColumnHeaders(t, n.ID)
		})
}

// uiaTableRowOrColumnMajor implements ITableProvider::get_RowOrColumnMajor. A unison table is a list of rows, each of
// which holds one cell per column, which is what row-major means here.
func uiaTableRowOrColumnMajor(this, out uintptr) uint64 {
	return uiaPatternInt32(this, uiaIfaceTable, out, func(_ *accessibility.Node) int32 {
		return int32(RowOrColumnMajor_RowMajor)
	})
}

// uiaTableItemGetRowHeaderItems implements ITableItemProvider::GetRowHeaderItems. There are no row headers for a cell
// to belong to, so the answer is an empty array.
func uiaTableItemGetRowHeaderItems(this, out uintptr) uint64 {
	return uiaPatternProviderArray(this, uiaIfaceTableItem, out, uiaNoNodes)
}

// uiaTableItemGetColumnHeaderItems implements ITableItemProvider::GetColumnHeaderItems, answering with the header of
// the cell's own column, which is what lets a screen reader say the column name before the cell's contents.
func uiaTableItemGetColumnHeaderItems(this, out uintptr) uint64 {
	return uiaPatternProviderArray(this, uiaIfaceTableItem, out,
		func(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
			if header := UIAColumnHeaderItem(t, n.ID); header != 0 {
				return []accessibility.NodeID{header}
			}
			return nil
		})
}

// uiaNoNodes is the answer of a pattern method that reports a set of elements unison never has any of.
func uiaNoNodes(_ *accessibility.Tree, _ *accessibility.Node) []accessibility.NodeID {
	return nil
}

// uiaPatternNode recovers what a control-pattern method needs to answer: the provider it was called on, the snapshot
// that provider must answer from, and its own node within it. hr is COM_S_OK when the method may go ahead, and
// otherwise is what it must return instead — see the list at the top of this file. tree and node are nil in that case,
// so a caller that ignores hr dereferences nil rather than answering from nothing.
//
// The pattern is checked on every call rather than trusted from the QueryInterface that handed out the interface: the
// snapshot the element supported the pattern in may have been replaced since, and a method whose node no longer has
// anything to say about a pattern must say so rather than invent an answer from a node of a different role. It is
// checked through the same supports the two ways of handing out an interface use, so the three cannot disagree.
func uiaPatternNode(this uintptr, iface uiaIface) (p *UIAProvider, tree *accessibility.Tree,
	node *accessibility.Node, hr uint64,
) {
	p = uiaProviderFromThis(this, iface)
	var ok bool
	if tree, node, ok = p.current(); !ok {
		return p, nil, nil, UIA_E_ELEMENTNOTAVAILABLE
	}
	if !p.supports(iface) {
		return p, nil, nil, UIA_E_NOTSUPPORTED
	}
	return p, tree, node, COM_S_OK
}

// uiaDispatch hands one action request to the window on the provider's behalf, filling in the node it is about. A
// window with nowhere to send the request reports the operation as impossible rather than as done; anything else would
// have a client waiting for a change that is never coming.
func uiaDispatch(p *UIAProvider, request accessibility.ActionRequest) uint64 {
	request.Node = p.node
	if !p.window.dispatch(request) {
		return UIA_E_INVALIDOPERATION
	}
	return COM_S_OK
}

// uiaPatternAction answers one of the pattern methods whose whole job is to ask for something to happen: it checks that
// the element is still there and still supports the pattern, refuses while the element is disabled, and queues the
// request. It returns as soon as the request is queued, which is what UI Automation expects: the outcome arrives later,
// as an event.
func uiaPatternAction(this uintptr, iface uiaIface, action accessibility.Action) uint64 {
	p, _, node, hr := uiaPatternNode(this, iface)
	if hr != COM_S_OK {
		return hr
	}
	if node.Disabled {
		return UIA_E_ELEMENTNOTENABLED
	}
	return uiaDispatch(p, accessibility.ActionRequest{Action: action})
}

// uiaPatternInt32 answers one of the pattern methods that reports a 32-bit integer: a count, an index, or one of the
// enumerations UI Automation defines, every one of which is a 32-bit integer across the ABI.
func uiaPatternInt32(this uintptr, iface uiaIface, out uintptr, pick func(n *accessibility.Node) int32) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearInt32(out)
	_, _, node, hr := uiaPatternNode(this, iface)
	if hr != COM_S_OK {
		return hr
	}
	*xruntime.PtrFromUintptr[int32](out) = pick(node)
	return COM_S_OK
}

// uiaPatternBOOL answers one of the pattern methods that reports a Win32 BOOL.
func uiaPatternBOOL(this uintptr, iface uiaIface, out uintptr, pick func(n *accessibility.Node) bool) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaSetBOOL(out, false)
	_, _, node, hr := uiaPatternNode(this, iface)
	if hr != COM_S_OK {
		return hr
	}
	uiaSetBOOL(out, pick(node))
	return COM_S_OK
}

// uiaPatternFloat64 answers one of the pattern methods that reports a double, which is every measurement the RangeValue
// pattern has.
func uiaPatternFloat64(this uintptr, iface uiaIface, out uintptr, pick func(n *accessibility.Node) float64) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearFloat64(out)
	_, _, node, hr := uiaPatternNode(this, iface)
	if hr != COM_S_OK {
		return hr
	}
	*xruntime.PtrFromUintptr[float64](out) = pick(node)
	return COM_S_OK
}

// uiaPatternProvider answers one of the pattern methods that reports another element, as an IRawElementProviderSimple
// pointer. A node with no provider — one that is not in the snapshot, or that the snapshot marks Ignored — is reported
// as NULL with S_OK, since "there is no such element" is an answer rather than a failure. The reference handed over is
// the caller's to release, as an interface out-parameter always is.
func uiaPatternProvider(this uintptr, iface uiaIface, out uintptr,
	pick func(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID,
) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	p, tree, node, hr := uiaPatternNode(this, iface)
	if hr != COM_S_OK {
		return hr
	}
	if other := p.window.Provider(pick(tree, node)); other != nil {
		// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
		*xruntime.PtrFromUintptr[uintptr](out) = other.ifacePtr(uiaIfaceSimple)
	}
	return COM_S_OK
}

// uiaPatternProviderArray answers one of the pattern methods that reports a set of elements, as a SAFEARRAY of
// IUnknown pointers. The array and the references in it belong to UI Automation Core, which destroys the array and
// releases them.
func uiaPatternProviderArray(this uintptr, iface uiaIface, out uintptr,
	pick func(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID,
) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	p, tree, node, hr := uiaPatternNode(this, iface)
	if hr != COM_S_OK {
		return hr
	}
	array := uiaProviderArray(p.window, pick(tree, node))
	if array == 0 {
		return COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[SAFEARRAY](out) = array
	return COM_S_OK
}

// uiaProviderArray builds the SAFEARRAY of interface pointers a pattern method reports a set of elements through. A
// node with no provider is left out rather than reported as a NULL element, which a client walking the array would
// have to guard against. An empty set yields a valid empty array, not a zero one.
//
// Storing an element into a VT_UNKNOWN array adds a reference of its own, so what the array ends up holding is exactly
// the one reference per element that UI Automation Core will release; the references Provider handed over are given
// back once the array has been built.
func uiaProviderArray(w *UIAWindow, ids []accessibility.NodeID) SAFEARRAY {
	providers := make([]*UIAProvider, 0, len(ids))
	defer func() {
		for _, p := range providers {
			p.release()
		}
	}()
	pointers := make([]unsafe.Pointer, 0, len(ids))
	for _, id := range ids {
		if p := w.Provider(id); p != nil {
			providers = append(providers, p)
			pointers = append(pointers, p.Unknown())
		}
	}
	return NewSafeArrayUnknown(pointers)
}

// uiaClearPointer zeroes a pointer-sized out-parameter, which is what an interface pointer, a BSTR and a SAFEARRAY all
// are. A NULL out-parameter is ignored rather than treated as an error, since the caller has already decided what to
// answer in that case.
func uiaClearPointer(out uintptr) {
	if out != 0 {
		*xruntime.PtrFromUintptr[uintptr](out) = 0
	}
}

// uiaClearInt32 zeroes a 32-bit out-parameter, which is what a Win32 BOOL, a count, an index and every
// enumeration-valued property are.
func uiaClearInt32(out uintptr) {
	if out != 0 {
		*xruntime.PtrFromUintptr[int32](out) = 0
	}
}

// uiaClearFloat64 zeroes a double out-parameter, which is what every measurement the RangeValue pattern reports is.
func uiaClearFloat64(out uintptr) {
	if out != 0 {
		*xruntime.PtrFromUintptr[float64](out) = 0
	}
}
