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
	"sync"

	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// This file answers the questions the control patterns ask of an accessibility snapshot: which of a container's items
// are selected, which cell sits at a row and a column, which column header describes a cell, which value a Value
// pattern reports and whether it may be changed, and which ancestor owns a selection item or a cell. Like uia_map.go it
// is a pure function of the snapshot — nothing here touches the OS, allocates COM memory or depends on which thread it
// runs on — so the file carries no build constraint and its tests run on any platform. uia_patterns_windows.go does
// nothing but turn these answers into COM out-parameters.
//
// The one piece of state here is uiaHeaderMemo, which remembers the single most expensive answer for the snapshot it
// was worked out from. It changes no answer, only how often one is worked out, and it is guarded so that the arbitrary
// threads UI Automation calls in on may share it.

// UIASelectionItemRole returns the role of the items the container of the given role selects among, or role.None when
// that role is not a selection container. It is what decides which descendants ISelectionProvider::GetSelection looks
// at: a table selects rows, a list selects list items, and a tab list selects tabs.
func UIASelectionItemRole(container role.Enum) role.Enum {
	switch container {
	case role.List:
		return role.ListItem
	case role.TabList:
		return role.Tab
	case role.Table, role.Tree:
		return role.Row
	default:
		return role.None
	}
}

// UIAIsSelected reports whether a node that supports the SelectionItem pattern is the selected one, which is what
// ISelectionItemProvider::get_IsSelected answers. A radio button has no selected state of its own: being the chosen one
// of its group is being checked, which is also why it reports the Toggle pattern to nobody.
func UIAIsSelected(n *accessibility.Node) bool {
	if n == nil {
		return false
	}
	if n.Role == role.RadioButton {
		return n.Checked == check.On
	}
	return n.Selected
}

// UIASelection returns the ids of the selected items within the container with the given id, in reading order, or nil
// when it holds none or is not a selection container at all.
//
// The search covers the container's whole subtree rather than only its children, since a hierarchical table nests its
// rows, but it stops at a nested container that selects the same kind of item: that container's selection is its own to
// report. Ignored nodes are spliced away, as everywhere else, because they have no provider to hand a client.
func UIASelection(t *accessibility.Tree, id accessibility.NodeID) []accessibility.NodeID {
	container := t.Node(id)
	if container == nil {
		return nil
	}
	itemRole := UIASelectionItemRole(container.Role)
	if itemRole == role.None {
		return nil
	}
	return uiaAppendItems(t, nil, id, itemRole, UIAIsSelected, 0)
}

// uiaAppendItems appends the unignored descendants of the node with the given id whose role is itemRole and which
// accept — nil for all of them — approves, in reading order. It does not look inside a nested container that holds the
// same kind of item, and is bounded in depth so that a malformed tree cannot spin here forever.
func uiaAppendItems(t *accessibility.Tree, ids []accessibility.NodeID, id accessibility.NodeID, itemRole role.Enum,
	accept func(n *accessibility.Node) bool, depth int,
) []accessibility.NodeID {
	if depth >= uiaMaxTreeDepth {
		return ids
	}
	for _, childID := range t.UnignoredChildren(id) {
		child := t.Node(childID)
		if child == nil {
			continue
		}
		if child.Role == itemRole && (accept == nil || accept(child)) {
			ids = append(ids, childID)
		}
		if UIASelectionItemRole(child.Role) == itemRole {
			continue
		}
		ids = uiaAppendItems(t, ids, childID, itemRole, accept, depth+1)
	}
	return ids
}

// UIASelectionContainer returns the id of the element ISelectionItemProvider::get_SelectionContainer reports for the
// node with the given id: the nearest ancestor that supports the Selection pattern, or zero when there is none.
//
// A radio button deliberately has none. Its group is a layout panel that the snapshot marks Ignored and that supports
// no pattern, so there is nothing to point a client at, and inventing one would have it announce the whole window as
// the group.
func UIASelectionContainer(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	n := t.Node(id)
	if n == nil || n.Role == role.RadioButton {
		return 0
	}
	return uiaAncestorWithPattern(t, id, PatternSelection)
}

// UIAContainingGrid returns the id of the element IGridItemProvider::get_ContainingGrid and
// ITableItemProvider::GetColumnHeaderItems work from: the nearest ancestor of the node with the given id that supports
// the Grid pattern, or zero when there is none.
func UIAContainingGrid(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	return uiaAncestorWithPattern(t, id, PatternGrid)
}

// uiaAncestorWithPattern returns the id of the nearest unignored ancestor of the node with the given id that supports
// every pattern in pattern, or zero when there is none. The walk is bounded so that a malformed tree — one whose Parent
// links form a cycle — cannot spin here forever.
func uiaAncestorWithPattern(t *accessibility.Tree, id accessibility.NodeID, pattern PatternSet,
) accessibility.NodeID {
	for depth := 0; depth < uiaMaxTreeDepth; depth++ {
		parentID := t.UnignoredParent(id)
		if parentID == 0 {
			return 0
		}
		if UIAPatterns(t.Node(parentID)).Has(pattern) {
			return parentID
		}
		id = parentID
	}
	return 0
}

// UIAGridItem returns the id of the cell IGridProvider::GetItem reports for a row and a column of the grid with the
// given id, or zero when the snapshot does not hold it.
//
// Zero is the common answer rather than an error: a table publishes only the rows in its viewport, so most of a large
// table's cells are genuinely not there to be handed over, and UI Automation is told so with a NULL element rather
// than with a failure. The row is found by the RowIndex it recorded rather than by its position among the rows
// published, which is what makes the answer right while scrolled.
func UIAGridItem(t *accessibility.Tree, id accessibility.NodeID, row, column int) accessibility.NodeID {
	if row < 0 || column < 0 {
		return 0
	}
	if t.Node(id) == nil {
		return 0
	}
	// Only a table and a tree support the Grid pattern, and the rows of both are Row nodes.
	rowID := accessibility.NodeID(0)
	for _, candidate := range uiaAppendItems(t, nil, id, role.Row, nil, 0) {
		if n := t.Node(candidate); n != nil && n.RowIndex == row {
			rowID = candidate
			break
		}
	}
	if rowID == 0 {
		return 0
	}
	for _, cellID := range t.UnignoredChildren(rowID) {
		if n := t.Node(cellID); n != nil && n.Role == role.Cell && n.ColumnIndex == column {
			return cellID
		}
	}
	return 0
}

// UIATableColumnHeaders returns the ids of the column headers ITableProvider::GetColumnHeaders reports for the table
// with the given id, in the order the header publishes them, or nil when the snapshot holds none.
//
// Finding them takes a search, because a unison table and its header are separate panels: the header goes into the
// column-header slot of the scroll panel whose content is the table, so it is nowhere inside the table's own subtree
// and the snapshot records no link between the two. Three things are tried, in this order:
//
//  1. A TableHeader the table's Controls name, or one whose own Controls name the table. An explicit link is the right
//     answer whenever a widget records one, and is the only thing that can be right when a window is laid out oddly.
//  2. Proximity: the nearest ancestor of the table that contains exactly one TableHeader and no table but this one.
//     That ancestor is the scroll panel in every layout unison builds. Insisting that it hold only one table is what
//     stops two tables side by side from claiming each other's header; an ancestor that holds more than one ends the
//     search rather than widening it, since widening it can only make the ambiguity worse. The search also stops the
//     moment it reaches another table or tree, since everything beyond that belongs to the outer table: a table nested
//     in a cell of another one would otherwise walk out past its own container and announce the outer table's column
//     names over the inner table's cells.
//  3. Nothing, which the provider reports as an empty array.
//
// Which header a table's columns come from is remembered for the snapshot it was worked out from, since a client asks
// for it once per cell; see uiaHeaderMemo.
func UIATableColumnHeaders(t *accessibility.Tree, id accessibility.NodeID) []accessibility.NodeID {
	header := uiaMemoizedTableHeaderFor(t, id)
	if header == 0 {
		return nil
	}
	return uiaAppendItems(t, nil, header, role.ColumnHeader, nil, 0)
}

// uiaHeaderMemo remembers which TableHeader describes which table, for one snapshot at a time. Working the answer out
// takes a walk of the whole tree plus a scan of one subtree per ancestor, and ITableItemProvider::GetColumnHeaderItems
// asks for it once per cell, so a screen reader stepping through a table would otherwise pay that cost for every cell
// it reads.
//
// Remembering it is correct only because a published snapshot is immutable: a publish swaps a whole new tree in rather
// than editing the one providers are answering from, so an answer worked out from a tree cannot go out of date while
// that tree is still the one being asked about. Only the most recently asked-about tree is kept, which is all a client
// walking one table needs and which keeps this from holding every snapshot a window ever published alive. It is a
// strong reference all the same, so UIAWindow.Destroy drops it: otherwise the last snapshot of a window with a table
// in it, and every node in that snapshot, would stay reachable for the rest of the process.
//
// UI Automation calls providers on whichever thread it likes, so every access is under the lock, the computation
// included: it is short, and several threads working the same answer out at once is the thing being avoided.
var uiaHeaderMemo struct {
	tree    *accessibility.Tree
	headers map[accessibility.NodeID]accessibility.NodeID
	lock    sync.Mutex
}

// uiaForgetHeaderMemo drops whatever uiaHeaderMemo is holding, so that a snapshot nothing is answering from any more
// is not kept alive by it. A window being destroyed calls it; the next question asked of any window works its answer
// out again, which costs one walk of that window's tree.
func uiaForgetHeaderMemo() {
	uiaHeaderMemo.lock.Lock()
	defer uiaHeaderMemo.lock.Unlock()
	uiaHeaderMemo.tree = nil
	uiaHeaderMemo.headers = nil
}

// uiaMemoizedTableHeaderFor answers uiaTableHeaderFor from uiaHeaderMemo, working the answer out and remembering it
// whenever the memo does not already hold one for this tree.
func uiaMemoizedTableHeaderFor(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	uiaHeaderMemo.lock.Lock()
	defer uiaHeaderMemo.lock.Unlock()
	if uiaHeaderMemo.tree != t || uiaHeaderMemo.headers == nil {
		uiaHeaderMemo.tree = t
		uiaHeaderMemo.headers = make(map[accessibility.NodeID]accessibility.NodeID)
	}
	if header, ok := uiaHeaderMemo.headers[id]; ok {
		return header
	}
	header := uiaTableHeaderFor(t, id)
	uiaHeaderMemo.headers[id] = header
	return header
}

// uiaTableHeaderFor returns the id of the TableHeader node that describes the columns of the table with the given id,
// or zero when there is none to be found. See UIATableColumnHeaders for how it is looked for and why.
func uiaTableHeaderFor(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	table := t.Node(id)
	if table == nil || !UIAPatterns(table).Has(PatternTable) {
		return 0
	}
	for _, controlled := range table.Controls {
		if n := t.Node(controlled); n != nil && n.Role == role.TableHeader && !n.Ignored {
			return controlled
		}
	}
	header := accessibility.NodeID(0)
	t.Walk(func(n *accessibility.Node) bool {
		if n.Role != role.TableHeader || n.Ignored {
			return true
		}
		for _, controlled := range n.Controls {
			if controlled == id {
				header = n.ID
				return false
			}
		}
		return true
	})
	if header != 0 {
		return header
	}
	ancestor := t.UnignoredParent(id)
	for depth := 0; ancestor != 0 && depth < uiaMaxTreeDepth; depth++ {
		if n := t.Node(ancestor); n != nil && n.ID != id && (n.Role == role.Table || n.Role == role.Tree) {
			// Another table contains this one, which a table nested in a cell really is. Its header describes its own
			// columns, not this table's, and everything further out belongs to it as well, so the search ends here
			// rather than climbing out of the container and claiming the outer table's column names.
			return 0
		}
		headers, tables := uiaHeadersAndTablesWithin(t, ancestor, 0)
		if tables > 1 {
			return 0
		}
		if len(headers) == 1 {
			return headers[0]
		}
		ancestor = t.UnignoredParent(ancestor)
	}
	return 0
}

// uiaHeadersAndTablesWithin returns the ids of the table headers within the subtree rooted at the node with the given
// id, along with how many tables it holds. Ignored nodes are walked through — a header could sit inside a layout panel
// — but an ignored node is never reported as a header, since it has no provider to hand a client.
func uiaHeadersAndTablesWithin(t *accessibility.Tree, id accessibility.NodeID, depth int,
) (headers []accessibility.NodeID, tables int) {
	n := t.Node(id)
	if n == nil || depth >= uiaMaxTreeDepth {
		return nil, 0
	}
	switch {
	case n.Role == role.TableHeader:
		if !n.Ignored {
			headers = append(headers, id)
		}
		// A header's own subtree holds nothing but its column headers, so there is no reason to walk into it.
		return headers, 0
	case n.Role == role.Table || n.Role == role.Tree:
		// Likewise a table's subtree holds its rows and cells. Counting it and stopping also keeps a table nested
		// inside another table's cell from being counted twice.
		return nil, 1
	default:
	}
	for _, childID := range n.Children {
		childHeaders, childTables := uiaHeadersAndTablesWithin(t, childID, depth+1)
		headers = append(headers, childHeaders...)
		tables += childTables
	}
	return headers, tables
}

// UIAColumnHeaderItem returns the id of the column header ITableItemProvider::GetColumnHeaderItems reports for the cell
// with the given id, or zero when there is none.
//
// The header of a cell's column is the one whose ColumnIndex matches the cell's. A snapshot that never filled in the
// headers' column indexes — they all read zero — would then answer only for the first column, so a header is taken by
// position when no index matches, which is right whenever the header publishes one element per column in column order.
func UIAColumnHeaderItem(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	cell := t.Node(id)
	if cell == nil {
		return 0
	}
	grid := UIAContainingGrid(t, id)
	if grid == 0 {
		return 0
	}
	headers := UIATableColumnHeaders(t, grid)
	for _, headerID := range headers {
		if n := t.Node(headerID); n != nil && n.ColumnIndex == cell.ColumnIndex {
			return headerID
		}
	}
	if cell.ColumnIndex >= 0 && cell.ColumnIndex < len(headers) {
		return headers[cell.ColumnIndex]
	}
	return 0
}

// UIAValueString returns the text IValueProvider::get_Value reports for a node. A text control's content is its value;
// everything else with a value reports it in textual form already. A password field reports nothing at all, which is
// the whole point of the Protected flag: the snapshot never fills in its value or its text either, and this is the
// second line of that defence.
func UIAValueString(n *accessibility.Node) string {
	if n == nil || n.Protected {
		return ""
	}
	if n.Value != "" {
		return n.Value
	}
	if n.Role.IsText() && n.Text != nil {
		return n.Text.Text
	}
	return ""
}

// UIAIsValueReadOnly reports whether IValueProvider::get_IsReadOnly says the value cannot be changed. Three things make
// it read-only:
//
//   - the role. A color well and a popup button report a value so that a client can say what is currently chosen, but
//     neither takes a new one through it, and a document is there to be read.
//   - the node saying so, through ReadOnly.
//   - the node not offering the SetValue action, which is the snapshot's own statement that nothing will happen if a
//     client tries. Declaring a value writable and then refusing every attempt is worse than saying so up front.
//
// Being disabled is deliberately not one of them. Read-only says the value could never be set through this element;
// disabled says nothing about the value at all, only that the element cannot be used just now, and UI Automation has
// IsEnabled for that. The SetValue paths refuse a disabled element with UIA_E_ELEMENTNOTENABLED before they ever ask
// this, so nothing is let through by the distinction.
func UIAIsValueReadOnly(n *accessibility.Node) bool {
	if n == nil {
		return true
	}
	switch n.Role {
	case role.ColorWell, role.PopupButton, role.Document:
		return true
	default:
		return n.ReadOnly || !n.Actions.Has(accessibility.SetValue)
	}
}

// UIAIsRangeValueReadOnly reports whether IRangeValueProvider::get_IsReadOnly says the value cannot be changed. A
// progress bar is read-only by definition — it reports what the application is doing, and nothing outside the
// application decides that — and everything else follows the same rules as UIAIsValueReadOnly, the disabled case
// included.
func UIAIsRangeValueReadOnly(n *accessibility.Node) bool {
	if n == nil {
		return true
	}
	if n.Role == role.ProgressBar {
		return true
	}
	return n.ReadOnly || !n.Actions.Has(accessibility.SetValue)
}
