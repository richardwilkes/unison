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
	"strconv"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// This file holds the twelve control-pattern interfaces a provider implements through simple answers about one node:
// their virtual method tables, the order of their slots, and the method bodies. The Text, Text2 and TextChild patterns
// are the ones it does not hold — reading a document as text takes a view over its whole composed stream and objects of
// its own for the ranges — and live in text_windows.go and textrange_windows.go instead.
//
// A pattern interface is handed out only when Patterns says the element supports that pattern, so a method here can
// rely on having been reached through an interface the element wanted — but only at the moment it was handed out, which
// is why every method checks again.
//
// Every answer comes from the window's immutable snapshot and every request to change something goes back to the root
// package as an action, to be run on the UI thread; see the comments at the top of provider_windows.go. The
// question each method asks of the snapshot lives in patterns.go, which is portable and tested on any platform, so
// what is left here is the COM: checking the out-parameters, converting to VARIANT-free out-parameter types, and
// getting the ownership of each BSTR and SAFEARRAY handed over right.
//
// The three answers a method can give instead of an answer:
//
//   - E_ELEMENTNOTAVAILABLE, when the element has left the tree. A client holding an element for something that has
//     been destroyed must be told that rather than given a stale answer.
//   - E_NOTSUPPORTED, when the element is still there but no longer supports the pattern. Snapshots are published
//     while clients hold interfaces, so a slider that became a label between QueryInterface and the call has to say so.
//     It is also the answer for asking a read-only element to take a new value, which keeps the disabled case — which
//     says nothing about whether the value could otherwise be set — distinguishable.
//   - E_ELEMENTNOTENABLED, when the element is disabled. Every pattern method that asks for something to happen
//     refuses on a disabled element, with one exception: ScrollIntoView, which is the single action a disabled node
//     goes on offering, since a screen reader stepping through a disabled list must still be able to bring its items on
//     screen. A disabled control is otherwise described, navigated to and reported, but nothing acts on it.
//   - E_INVALIDOPERATION, when the element does not offer the action the method stands for. A pattern says what
//     kind of thing an element is, and the snapshot's action set says what can actually be done to it: a column header
//     that cannot be sorted carries Invoke so that a client describes it as a header, and a container row in a filtered
//     table reports that it is expanded without offering to collapse. Answering S_OK and raising the pattern's event
//     would tell a client that something happened when the widget refuses the request outright.

// buildPatternVtbls fills in the virtual method table of every control-pattern interface. The order of the methods
// in each call is the order the interface declares them in; see the interface declarations in the Windows SDK's
// uiautomationcore.idl, or §7.2 of the plan, which lists them.
func buildPatternVtbls() {
	buildVtbl(
		ifaceInvoke, invokeVtbl[:],
		invokeInvoke,
	)
	buildVtbl(
		ifaceToggle, toggleVtbl[:],
		toggleToggle,
		toggleState,
	)
	buildVtbl(
		ifaceValue, valueVtbl[:],
		valueSetValue,
		valueValue,
		valueIsReadOnly,
	)
	// IRangeValueProvider::SetValue takes a double, which syscall.NewCallback cannot describe, so its slot holds an
	// assembly thunk that moves the floating-point register into the integer argument register the callback reads. See
	// thunk_windows.go, including what to do if a thunk ever misbehaves on a target.
	rangeValueSetValueCallback = windows.NewCallback(rangeValueSetValue)
	buildVtbl(
		ifaceRangeValue, rangeValueVtbl[:],
		rangeValueSetValueSlot(),
		rangeValueValue,
		rangeValueIsReadOnly,
		rangeValueMaximum,
		rangeValueMinimum,
		rangeValueLargeChange,
		rangeValueSmallChange,
	)
	buildVtbl(
		ifaceSelection, selectionVtbl[:],
		selectionGetSelection,
		selectionCanSelectMultiple,
		selectionIsSelectionRequired,
	)
	buildVtbl(
		ifaceSelectionItem, selectionItemVtbl[:],
		selectionItemSelect,
		selectionItemAddToSelection,
		selectionItemRemoveFromSelection,
		selectionItemIsSelected,
		selectionItemSelectionContainer,
	)
	buildVtbl(
		ifaceExpandCollapse, expandCollapseVtbl[:],
		expandCollapseExpand,
		expandCollapseCollapse,
		expandCollapseState,
	)
	buildVtbl(
		ifaceScrollItem, scrollItemVtbl[:],
		scrollItemScrollIntoView,
	)
	buildVtbl(
		ifaceGrid, gridVtbl[:],
		gridGetItem,
		gridRowCount,
		gridColumnCount,
	)
	buildVtbl(
		ifaceGridItem, gridItemVtbl[:],
		gridItemRow,
		gridItemColumn,
		gridItemRowSpan,
		gridItemColumnSpan,
		gridItemContainingGrid,
	)
	buildVtbl(
		ifaceTable, tableVtbl[:],
		tableGetRowHeaders,
		tableGetColumnHeaders,
		tableRowOrColumnMajor,
	)
	buildVtbl(
		ifaceTableItem, tableItemVtbl[:],
		tableItemGetRowHeaderItems,
		tableItemGetColumnHeaderItems,
	)
}

// invokeInvoke implements IInvokeProvider::Invoke, which is a client asking for the one thing the element does:
// pressing a button, following a link, choosing a menu item, sorting by a column header.
//
// The interface requires Invoke_InvokedEventId to be raised once the element has carried the action out, and this
// is the only place it can come from: pressing something need not change the snapshot at all — a button that opens a
// menu, or one whose handler does its work somewhere else entirely — so there may be no publish to carry the news, and
// a client waiting on Invoked would wait forever.
//
// It is raised as soon as the request has been accepted rather than once it has been carried out, because the outcome
// cannot be seen from here: the request is queued onto the UI thread and the callback the adapter was handed reports
// nothing back, deliberately, since UI Automation calls in on whichever thread it likes and must not be left waiting
// on a UI thread that may be inside a modal loop. That is the same optimism the S_OK answered here already carries.
//
// Which is why patternAction refusing an element that does not offer the Press action matters here in particular. A
// column header carries the Invoke pattern whether or not its table can be sorted by it — that is how a client knows
// what kind of thing it is — and TableHeader.PerformAccessibilityAction refuses a press on an unsortable one, so
// without that check a client would be answered S_OK and handed an Invoked event while nothing at all had happened.
func invokeInvoke(this uintptr) uint64 {
	hr := patternAction(this, ifaceInvoke, accessibility.Press)
	if hr == w32.COM_S_OK {
		providerFromThis(this, ifaceInvoke).raiseEvent(Invoke_InvokedEventId)
	}
	return hr
}

// toggleToggle implements IToggleProvider::Toggle, which moves a checkable element to its next state. Which state
// that is belongs to the widget: a two-state check box has one answer and a three-state check box another, and neither
// is decided here.
//
// Not everything that reports the Toggle pattern offers the Toggle action. A sticky or grouped button is reported as a
// toggle button, because the state a click leaves it in is what a client has to hear, but the only thing it does is be
// pressed. Toggle is the sole way a client can operate such an element on Windows — the role carries no Invoke pattern
// — so a node that does not offer Toggle but does offer Press is pressed instead. Dispatching an action the widget
// ignores would answer S_OK while the button never changed.
//
// An element that offers neither is refused with E_INVALIDOPERATION, as every other write path refuses one; see the
// list at the top of this file. There is no event to follow a toggle, so a client answered S_OK would be told the
// toggle happened and left with nothing to notice otherwise.
func toggleToggle(this uintptr) uint64 {
	p, _, node, hr := patternNode(this, ifaceToggle)
	if hr != w32.COM_S_OK {
		return hr
	}
	if node.Disabled {
		return E_ELEMENTNOTENABLED
	}
	action := accessibility.Toggle
	switch {
	case node.Actions.Has(accessibility.Toggle):
	case node.Actions.Has(accessibility.Press):
		action = accessibility.Press
	default:
		return E_INVALIDOPERATION
	}
	return dispatch(p, accessibility.ActionRequest{Action: action})
}

// toggleState implements IToggleProvider::get_ToggleState.
func toggleState(this, out uintptr) uint64 {
	return patternInt32(this, ifaceToggle, out, func(n *accessibility.Node) int32 {
		return int32(ToggleStateOf(n))
	})
}

// valueSetValue implements IValueProvider::SetValue. The string arrives as a NUL-terminated UTF-16 sequence that
// belongs to UI Automation, so it is decoded into a Go string before the request is queued: the pointer is not valid
// once this method has returned, and the action runs later, on the UI thread.
func valueSetValue(this, str uintptr) uint64 {
	p, _, node, hr := patternNode(this, ifaceValue)
	if hr != w32.COM_S_OK {
		return hr
	}
	if str == 0 {
		return w32.COM_E_INVALIDARG
	}
	if node.Disabled {
		return E_ELEMENTNOTENABLED
	}
	if IsValueReadOnly(node) {
		return E_NOTSUPPORTED
	}
	return dispatch(p, accessibility.ActionRequest{
		Action: accessibility.SetValue,
		Value:  windows.UTF16PtrToString(xruntime.PtrFromUintptr[uint16](str)),
	})
}

// valueValue implements IValueProvider::get_Value. The BSTR handed back belongs to UI Automation Core, which frees
// it, so nothing here does. An element with nothing to report answers with an empty string rather than a NULL BSTR:
// NULL is indistinguishable from an allocation failure to most clients.
func valueValue(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	_, _, node, hr := patternNode(this, ifaceValue)
	if hr != w32.COM_S_OK {
		return hr
	}
	str := NewBSTR(ValueString(node))
	if str == 0 {
		return w32.COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[BSTR](out) = str
	return w32.COM_S_OK
}

// valueIsReadOnly implements IValueProvider::get_IsReadOnly.
func valueIsReadOnly(this, out uintptr) uint64 {
	return patternBOOL(this, ifaceValue, out, IsValueReadOnly)
}

// rangeValueSetValue implements IRangeValueProvider::SetValue. It is reached through an assembly thunk, so the value
// arrives as the raw bits of the double rather than as a double; see thunk_windows.go. A value outside the
// element's own range, or one that is not a number at all, is refused here rather than clamped by the widget, so that a
// client is told its request was wrong instead of silently getting something else.
//
// The request carries the number both ways: as Number, and as the text the same number reads as. A widget that takes
// its value as text — a numeric field is a text field underneath, and answers SetValue by replacing its text — would
// otherwise be handed an empty string and blank itself. The Cocoa adapter fills in both for the same reason.
func rangeValueSetValue(this, valueBits uintptr) uint64 {
	p, _, node, hr := patternNode(this, ifaceRangeValue)
	if hr != w32.COM_S_OK {
		return hr
	}
	if node.Disabled {
		return E_ELEMENTNOTENABLED
	}
	if IsRangeValueReadOnly(node) {
		return E_NOTSUPPORTED
	}
	number := math.Float64frombits(uint64(valueBits))
	if math.IsNaN(number) || (node.HasNumber && node.Max > node.Min && (number < node.Min || number > node.Max)) {
		return w32.COM_E_INVALIDARG
	}
	return dispatch(p, accessibility.ActionRequest{
		Action: accessibility.SetValue,
		Number: number,
		Value:  strconv.FormatFloat(number, 'g', -1, 64),
	})
}

// rangeValueValue implements IRangeValueProvider::get_Value.
func rangeValueValue(this, out uintptr) uint64 {
	return patternFloat64(this, ifaceRangeValue, out, func(n *accessibility.Node) float64 { return n.Number })
}

// rangeValueIsReadOnly implements IRangeValueProvider::get_IsReadOnly.
func rangeValueIsReadOnly(this, out uintptr) uint64 {
	return patternBOOL(this, ifaceRangeValue, out, IsRangeValueReadOnly)
}

// rangeValueMaximum implements IRangeValueProvider::get_Maximum.
func rangeValueMaximum(this, out uintptr) uint64 {
	return patternFloat64(this, ifaceRangeValue, out, func(n *accessibility.Node) float64 { return n.Max })
}

// rangeValueMinimum implements IRangeValueProvider::get_Minimum.
func rangeValueMinimum(this, out uintptr) uint64 {
	return patternFloat64(this, ifaceRangeValue, out, func(n *accessibility.Node) float64 { return n.Min })
}

// rangeValueLargeChange implements IRangeValueProvider::get_LargeChange.
func rangeValueLargeChange(this, out uintptr) uint64 {
	return patternFloat64(this, ifaceRangeValue, out, LargeChange)
}

// rangeValueSmallChange implements IRangeValueProvider::get_SmallChange.
func rangeValueSmallChange(this, out uintptr) uint64 {
	return patternFloat64(this, ifaceRangeValue, out, SmallChange)
}

// selectionGetSelection implements ISelectionProvider::GetSelection. A container with nothing selected answers with
// an empty array rather than a NULL one, which is what says "nothing is selected" as opposed to "I cannot tell you".
func selectionGetSelection(this, out uintptr) uint64 {
	return patternProviderArray(this, ifaceSelection, out,
		func(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
			return Selection(t, n.ID)
		})
}

// selectionCanSelectMultiple implements ISelectionProvider::get_CanSelectMultiple.
func selectionCanSelectMultiple(this, out uintptr) uint64 {
	return patternBOOL(this, ifaceSelection, out, func(n *accessibility.Node) bool {
		return n.Multiselectable
	})
}

// selectionIsSelectionRequired implements ISelectionProvider::get_IsSelectionRequired. No container unison builds
// insists on holding a selection: a list, a table and a tab list can all be left with nothing selected, and a client
// that was told otherwise would report an empty one as a broken element.
func selectionIsSelectionRequired(this, out uintptr) uint64 {
	return patternBOOL(this, ifaceSelection, out, func(_ *accessibility.Node) bool { return false })
}

// selectionItemSelect implements ISelectionItemProvider::Select, which makes this element the only selection in its
// container.
func selectionItemSelect(this uintptr) uint64 {
	return selectionItemAction(this, accessibility.Select)
}

// selectionItemAddToSelection implements ISelectionItemProvider::AddToSelection.
func selectionItemAddToSelection(this uintptr) uint64 {
	return selectionItemAction(this, accessibility.AddToSelection)
}

// selectionItemRemoveFromSelection implements ISelectionItemProvider::RemoveFromSelection.
func selectionItemRemoveFromSelection(this uintptr) uint64 {
	return selectionItemAction(this, accessibility.RemoveFromSelection)
}

// selectionItemAction answers one of the three SelectionItem methods that changes what is selected.
//
// A radio button is the exception the pattern exists to describe: it is a selection item whose container is a layout
// panel with no pattern of its own, so choosing it is pressing it, and it cannot be unchosen at all — a radio group is
// left without a selection only by the application. Select and AddToSelection therefore become Press, and
// RemoveFromSelection is refused.
//
// Everything else is measured against what the snapshot says the element offers, as every other write path here is.
// Adding to or removing from a selection means nothing in a container that holds one selection at a time: the widget
// would replace the selection instead, so a client that was answered S_OK would be told the opposite of what happened.
// Neither does an action the node does not list. UI Automation's answer for both is E_INVALIDOPERATION — the
// operation cannot be performed on this element — rather than E_NOTSUPPORTED, which would deny the whole pattern.
func selectionItemAction(this uintptr, action accessibility.Action) uint64 {
	p, tree, node, hr := patternNode(this, ifaceSelectionItem)
	if hr != w32.COM_S_OK {
		return hr
	}
	if node.Disabled {
		return E_ELEMENTNOTENABLED
	}
	switch {
	case node.Role == role.RadioButton:
		if action == accessibility.RemoveFromSelection {
			return E_INVALIDOPERATION
		}
		action = accessibility.Press
	case action != accessibility.Select && !multipleSelection(tree, node):
		return E_INVALIDOPERATION
	default:
	}
	if !node.Actions.Has(action) {
		return E_INVALIDOPERATION
	}
	return dispatch(p, accessibility.ActionRequest{Action: action})
}

// multipleSelection reports whether the container a selection item belongs to holds more than one selection at a
// time, which is what decides whether AddToSelection and RemoveFromSelection mean anything for that item. The container
// is the one ISelectionItemProvider::get_SelectionContainer reports, so a client asking about it and a client acting on
// it are looking at the same element.
func multipleSelection(t *accessibility.Tree, n *accessibility.Node) bool {
	container := t.Node(SelectionContainer(t, n.ID))
	return container != nil && container.Multiselectable
}

// selectionItemIsSelected implements ISelectionItemProvider::get_IsSelected.
func selectionItemIsSelected(this, out uintptr) uint64 {
	return patternBOOL(this, ifaceSelectionItem, out, IsSelected)
}

// selectionItemSelectionContainer implements ISelectionItemProvider::get_SelectionContainer, answering with the
// nearest ancestor that supports the Selection pattern, or NULL when there is none — which is the answer for a radio
// button. See SelectionContainer.
func selectionItemSelectionContainer(this, out uintptr) uint64 {
	return patternProvider(this, ifaceSelectionItem, out,
		func(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID {
			return SelectionContainer(t, n.ID)
		})
}

// expandCollapseExpand implements IExpandCollapseProvider::Expand.
func expandCollapseExpand(this uintptr) uint64 {
	return patternAction(this, ifaceExpandCollapse, accessibility.Expand)
}

// expandCollapseCollapse implements IExpandCollapseProvider::Collapse.
func expandCollapseCollapse(this uintptr) uint64 {
	return patternAction(this, ifaceExpandCollapse, accessibility.Collapse)
}

// expandCollapseState implements IExpandCollapseProvider::get_ExpandCollapseState.
func expandCollapseState(this, out uintptr) uint64 {
	return patternInt32(this, ifaceExpandCollapse, out, func(n *accessibility.Node) int32 {
		return int32(ExpandCollapseStateOf(n))
	})
}

// scrollItemScrollIntoView implements IScrollItemProvider::ScrollIntoView, which a client calls to bring an element
// it is about to talk about into view. It is the one thing a disabled element still does: every row and item of a
// disabled table or list is published with ScrollIntoView as its only action, and a screen reader that could not scroll
// them would be reading out elements the user cannot see.
func scrollItemScrollIntoView(this uintptr) uint64 {
	return patternAction(this, ifaceScrollItem, accessibility.ScrollIntoView)
}

// gridGetItem implements IGridProvider::GetItem. A row or column outside the grid is an error, as UI Automation
// defines it, but a cell the snapshot simply does not hold is not: a table publishes only the rows in its viewport, so
// a client walking a large grid is answered with a NULL element for most of it.
func gridGetItem(this, row, column, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	p, tree, node, hr := patternNode(this, ifaceGrid)
	if hr != w32.COM_S_OK {
		return hr
	}
	wantRow, wantColumn := int(int32(uint32(row))), int(int32(uint32(column)))
	if wantRow < 0 || wantColumn < 0 || (node.RowCount > 0 && wantRow >= node.RowCount) ||
		(node.ColumnCount > 0 && wantColumn >= node.ColumnCount) {
		return w32.COM_E_INVALIDARG
	}
	if cell := p.window.Provider(GridItem(tree, p.node, wantRow, wantColumn)); cell != nil {
		// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
		*xruntime.PtrFromUintptr[uintptr](out) = cell.ifacePtr(ifaceSimple)
	}
	return w32.COM_S_OK
}

// gridRowCount implements IGridProvider::get_RowCount. It is the number of rows the grid holds, not the number the
// snapshot published: a client asking how large a table is wants the answer the scroll bar reflects.
func gridRowCount(this, out uintptr) uint64 {
	return patternInt32(this, ifaceGrid, out, func(n *accessibility.Node) int32 { return int32(n.RowCount) })
}

// gridColumnCount implements IGridProvider::get_ColumnCount.
func gridColumnCount(this, out uintptr) uint64 {
	return patternInt32(this, ifaceGrid, out, func(n *accessibility.Node) int32 { return int32(n.ColumnCount) })
}

// gridItemRow implements IGridItemProvider::get_Row.
func gridItemRow(this, out uintptr) uint64 {
	return patternInt32(this, ifaceGridItem, out, func(n *accessibility.Node) int32 { return int32(n.RowIndex) })
}

// gridItemColumn implements IGridItemProvider::get_Column.
func gridItemColumn(this, out uintptr) uint64 {
	return patternInt32(this, ifaceGridItem, out, func(n *accessibility.Node) int32 {
		return int32(n.ColumnIndex)
	})
}

// gridItemRowSpan implements IGridItemProvider::get_RowSpan. Nothing unison draws merges cells, so every cell
// occupies exactly one row and one column.
func gridItemRowSpan(this, out uintptr) uint64 {
	return patternInt32(this, ifaceGridItem, out, func(_ *accessibility.Node) int32 { return 1 })
}

// gridItemColumnSpan implements IGridItemProvider::get_ColumnSpan.
func gridItemColumnSpan(this, out uintptr) uint64 {
	return patternInt32(this, ifaceGridItem, out, func(_ *accessibility.Node) int32 { return 1 })
}

// gridItemContainingGrid implements IGridItemProvider::get_ContainingGrid, answering with the nearest ancestor that
// supports the Grid pattern.
func gridItemContainingGrid(this, out uintptr) uint64 {
	return patternProvider(this, ifaceGridItem, out,
		func(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID {
			return ContainingGrid(t, n.ID)
		})
}

// tableGetRowHeaders implements ITableProvider::GetRowHeaders. A unison table has no row headers — its first column
// is an ordinary column — so the answer is an empty array.
func tableGetRowHeaders(this, out uintptr) uint64 {
	return patternProviderArray(this, ifaceTable, out, noNodes)
}

// tableGetColumnHeaders implements ITableProvider::GetColumnHeaders. See TableColumnHeaders for how the header of
// a table is found, given that the two are separate panels.
func tableGetColumnHeaders(this, out uintptr) uint64 {
	return patternProviderArray(this, ifaceTable, out,
		func(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
			return TableColumnHeaders(t, n.ID)
		})
}

// tableRowOrColumnMajor implements ITableProvider::get_RowOrColumnMajor. A unison table is a list of rows, each of
// which holds one cell per column, which is what row-major means here.
func tableRowOrColumnMajor(this, out uintptr) uint64 {
	return patternInt32(this, ifaceTable, out, func(_ *accessibility.Node) int32 {
		return int32(RowOrColumnMajor_RowMajor)
	})
}

// tableItemGetRowHeaderItems implements ITableItemProvider::GetRowHeaderItems. There are no row headers for a cell
// to belong to, so the answer is an empty array.
func tableItemGetRowHeaderItems(this, out uintptr) uint64 {
	return patternProviderArray(this, ifaceTableItem, out, noNodes)
}

// tableItemGetColumnHeaderItems implements ITableItemProvider::GetColumnHeaderItems, answering with the header of
// the cell's own column, which is what lets a screen reader say the column name before the cell's contents.
func tableItemGetColumnHeaderItems(this, out uintptr) uint64 {
	return patternProviderArray(this, ifaceTableItem, out,
		func(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
			if header := ColumnHeaderItem(t, n.ID); header != 0 {
				return []accessibility.NodeID{header}
			}
			return nil
		})
}

// noNodes is the answer of a pattern method that reports a set of elements unison never has any of.
func noNodes(_ *accessibility.Tree, _ *accessibility.Node) []accessibility.NodeID {
	return nil
}

// patternNode recovers what a control-pattern method needs to answer: the provider it was called on, the snapshot
// that provider must answer from, and its own node within it. hr is w32.COM_S_OK when the method may go ahead, and
// otherwise is what it must return instead — see the list at the top of this file. tree and node are nil in that case,
// so a caller that ignores hr dereferences nil rather than answering from nothing.
//
// The pattern is checked on every call rather than trusted from the QueryInterface that handed out the interface: the
// snapshot the element supported the pattern in may have been replaced since, and a method whose node no longer has
// anything to say about a pattern must say so rather than invent an answer from a node of a different role. It is
// checked through the same supports the two ways of handing out an interface use, so the three cannot disagree.
func patternNode(this uintptr, which iface) (p *Provider, tree *accessibility.Tree,
	node *accessibility.Node, hr uint64,
) {
	p = providerFromThis(this, which)
	var ok bool
	if tree, node, ok = p.current(); !ok {
		return p, nil, nil, E_ELEMENTNOTAVAILABLE
	}
	if !p.supports(which) {
		return p, nil, nil, E_NOTSUPPORTED
	}
	return p, tree, node, w32.COM_S_OK
}

// dispatch hands one action request to the window on the provider's behalf, filling in the node it is about. A
// window with nowhere to send the request reports the operation as impossible rather than as done; anything else would
// have a client waiting for a change that is never coming.
func dispatch(p *Provider, request accessibility.ActionRequest) uint64 {
	request.Node = p.node
	if !p.window.dispatch(request) {
		return E_INVALIDOPERATION
	}
	return w32.COM_S_OK
}

// patternAction answers one of the pattern methods whose whole job is to ask for something to happen: it checks that
// the element is still there and still supports the pattern, refuses while the element is disabled or does not offer
// the action, and queues the request. It returns as soon as the request is queued, which is what UI Automation expects:
// the outcome arrives later, as an event.
//
// ScrollIntoView is the one action a disabled element still takes; see the list at the top of this file for why, and
// axDisabledActions in accessibility_actions.go for the snapshot's half of the same rule.
func patternAction(this uintptr, which iface, action accessibility.Action) uint64 {
	p, _, node, hr := patternNode(this, which)
	if hr != w32.COM_S_OK {
		return hr
	}
	if node.Disabled && action != accessibility.ScrollIntoView {
		return E_ELEMENTNOTENABLED
	}
	if !node.Actions.Has(action) {
		return E_INVALIDOPERATION
	}
	return dispatch(p, accessibility.ActionRequest{Action: action})
}

// patternInt32 answers one of the pattern methods that reports a 32-bit integer: a count, an index, or one of the
// enumerations UI Automation defines, every one of which is a 32-bit integer across the ABI.
func patternInt32(this uintptr, which iface, out uintptr, pick func(n *accessibility.Node) int32) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearInt32(out)
	_, _, node, hr := patternNode(this, which)
	if hr != w32.COM_S_OK {
		return hr
	}
	*xruntime.PtrFromUintptr[int32](out) = pick(node)
	return w32.COM_S_OK
}

// patternBOOL answers one of the pattern methods that reports a Win32 BOOL.
func patternBOOL(this uintptr, which iface, out uintptr, pick func(n *accessibility.Node) bool) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	setBOOL(out, false)
	_, _, node, hr := patternNode(this, which)
	if hr != w32.COM_S_OK {
		return hr
	}
	setBOOL(out, pick(node))
	return w32.COM_S_OK
}

// patternFloat64 answers one of the pattern methods that reports a double, which is every measurement the RangeValue
// pattern has.
func patternFloat64(this uintptr, which iface, out uintptr, pick func(n *accessibility.Node) float64) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearFloat64(out)
	_, _, node, hr := patternNode(this, which)
	if hr != w32.COM_S_OK {
		return hr
	}
	*xruntime.PtrFromUintptr[float64](out) = pick(node)
	return w32.COM_S_OK
}

// patternProvider answers one of the pattern methods that reports another element, as an IRawElementProviderSimple
// pointer. A node with no provider — one that is not in the snapshot, or that the snapshot marks Ignored — is reported
// as NULL with S_OK, since "there is no such element" is an answer rather than a failure. The reference handed over is
// the caller's to release, as an interface out-parameter always is.
func patternProvider(this uintptr, which iface, out uintptr,
	pick func(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID,
) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	p, tree, node, hr := patternNode(this, which)
	if hr != w32.COM_S_OK {
		return hr
	}
	if other := p.window.Provider(pick(tree, node)); other != nil {
		// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
		*xruntime.PtrFromUintptr[uintptr](out) = other.ifacePtr(ifaceSimple)
	}
	return w32.COM_S_OK
}

// patternProviderArray answers one of the pattern methods that reports a set of elements, as a SAFEARRAY of
// IUnknown pointers. The array and the references in it belong to UI Automation Core, which destroys the array and
// releases them.
func patternProviderArray(this uintptr, which iface, out uintptr,
	pick func(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID,
) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	p, tree, node, hr := patternNode(this, which)
	if hr != w32.COM_S_OK {
		return hr
	}
	array := providerArray(p.window, pick(tree, node))
	if array == 0 {
		return w32.COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[SAFEARRAY](out) = array
	return w32.COM_S_OK
}

// providerArray builds the SAFEARRAY of interface pointers a pattern method reports a set of elements through. A
// node with no provider is left out rather than reported as a NULL element, which a client walking the array would
// have to guard against. An empty set yields a valid empty array, not a zero one.
//
// Storing an element into a VT_UNKNOWN array adds a reference of its own, so what the array ends up holding is exactly
// the one reference per element that UI Automation Core will release; the references Provider handed over are given
// back once the array has been built.
func providerArray(w *Window, ids []accessibility.NodeID) SAFEARRAY {
	providers := make([]*Provider, 0, len(ids))
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

// clearPointer zeroes a pointer-sized out-parameter, which is what an interface pointer, a BSTR and a SAFEARRAY all
// are. A NULL out-parameter is ignored rather than treated as an error, since the caller has already decided what to
// answer in that case.
func clearPointer(out uintptr) {
	if out != 0 {
		*xruntime.PtrFromUintptr[uintptr](out) = 0
	}
}

// clearInt32 zeroes a 32-bit out-parameter, which is what a Win32 BOOL, a count, an index and every
// enumeration-valued property are.
func clearInt32(out uintptr) {
	if out != 0 {
		*xruntime.PtrFromUintptr[int32](out) = 0
	}
}

// clearFloat64 zeroes a double out-parameter, which is what every measurement the RangeValue pattern reports is.
func clearFloat64(out uintptr) {
	if out != 0 {
		*xruntime.PtrFromUintptr[float64](out) = 0
	}
}
