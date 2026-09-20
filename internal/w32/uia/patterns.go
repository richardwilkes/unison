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
	"sync"

	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// This file answers the questions the control patterns ask of an accessibility snapshot: which of a container's items
// are selected, which cell sits at a row and a column, which column header describes a cell, which value a Value
// pattern reports and whether it may be changed, and which ancestor owns a selection item or a cell. Like map.go it
// is a pure function of the snapshot — nothing here touches the OS, allocates COM memory or depends on which thread it
// runs on — so the file carries no build constraint and its tests run on any platform. patterns_windows.go does
// nothing but turn these answers into COM out-parameters.
//
// The one piece of state here is snapshotMemo, which remembers the most expensive answers this package gives — the
// ones from map.go included — for the snapshot they were worked out from. It changes no answer, only how often one
// is worked out, and it is guarded so that the arbitrary threads UI Automation calls in on may share it.

// SelectionItemRole returns the role of the items the container of the given role selects among, or role.None when
// that role is not a selection container. It is what decides which descendants ISelectionProvider::GetSelection looks
// at: a table selects rows, a list selects list items, and a tab list selects tabs.
func SelectionItemRole(container role.Enum) role.Enum {
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

// IsSelected reports whether a node that supports the SelectionItem pattern is the selected one, which is what
// ISelectionItemProvider::get_IsSelected answers. A radio button has no selected state of its own: being the chosen one
// of its group is being checked, which is also why it reports the Toggle pattern to nobody.
func IsSelected(n *accessibility.Node) bool {
	if n == nil {
		return false
	}
	if n.Role == role.RadioButton {
		return n.Checked == check.On
	}
	return n.Selected
}

// Selection returns the ids of the selected items within the container with the given id, in reading order, or nil
// when it holds none or is not a selection container at all.
//
// The search covers the container's whole subtree rather than only its children, since a hierarchical table nests its
// rows, but it stops at a nested container that selects the same kind of item: that container's selection is its own to
// report. Ignored nodes are spliced away, as everywhere else, because they have no provider to hand a client.
func Selection(t *accessibility.Tree, id accessibility.NodeID) []accessibility.NodeID {
	container := t.Node(id)
	if container == nil {
		return nil
	}
	itemRole := SelectionItemRole(container.Role)
	if itemRole == role.None {
		return nil
	}
	return itemsWithin(t, id, itemRole, IsSelected)
}

// itemsWithin returns the unignored descendants of the node with the given id whose role is itemRole and which
// accept — nil for all of them — approves, in reading order. It does not look inside a nested container that holds the
// same kind of item. See appendItems, which it starts.
func itemsWithin(t *accessibility.Tree, id accessibility.NodeID, itemRole role.Enum,
	accept func(n *accessibility.Node) bool,
) []accessibility.NodeID {
	return appendItems(t, nil, id, itemRole, accept, make(map[accessibility.NodeID]bool), 0)
}

// appendItems appends the unignored descendants of the node with the given id whose role is itemRole and which
// accept — nil for all of them — approves, in reading order. It does not look inside a nested container that holds the
// same kind of item.
//
// visited holds the ids already reached, which is what keeps a malformed tree from being descended forever, and it is
// what bounds this rather than depth: a walk downwards follows Children links that may branch and revisit, so a depth
// bound alone would let a chain of nodes that each list the same child twice take 2^depth visits, and a Children link
// that points back at an ancestor would never end at all. Each node is therefore both reported and descended into at
// most once, exactly as accessibility.Tree's own walk does it. The depth bound stays as a second line of defense, since
// the recursion is on the stack. UI Automation asks these questions from arbitrary threads — GetSelection, GetItem,
// GetColumnHeaders — so a snapshot with a duplicated or cyclic child id must answer rather than hang the client.
func appendItems(t *accessibility.Tree, ids []accessibility.NodeID, id accessibility.NodeID, itemRole role.Enum,
	accept func(n *accessibility.Node) bool, visited map[accessibility.NodeID]bool, depth int,
) []accessibility.NodeID {
	if depth >= maxTreeDepth {
		return ids
	}
	for _, childID := range t.UnignoredChildren(id) {
		child := t.Node(childID)
		if child == nil || visited[childID] {
			continue
		}
		visited[childID] = true
		if child.Role == itemRole && (accept == nil || accept(child)) {
			ids = append(ids, childID)
		}
		if SelectionItemRole(child.Role) == itemRole {
			continue
		}
		ids = appendItems(t, ids, childID, itemRole, accept, visited, depth+1)
	}
	return ids
}

// SelectionContainer returns the id of the element ISelectionItemProvider::get_SelectionContainer reports for the
// node with the given id: the nearest ancestor that supports the Selection pattern, or zero when there is none.
//
// A radio button deliberately has none. Its group is a layout panel that the snapshot marks Ignored and that supports
// no pattern, so there is nothing to point a client at, and inventing one would have it announce the whole window as
// the group.
func SelectionContainer(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	n := t.Node(id)
	if n == nil || n.Role == role.RadioButton {
		return 0
	}
	return ancestorWithPattern(t, id, PatternSelection)
}

// ContainingGrid returns the id of the element IGridItemProvider::get_ContainingGrid and
// ITableItemProvider::GetColumnHeaderItems work from: the nearest ancestor of the node with the given id that supports
// the Grid pattern, or zero when there is none.
func ContainingGrid(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	return ancestorWithPattern(t, id, PatternGrid)
}

// ancestorWithPattern returns the id of the nearest unignored ancestor of the node with the given id that supports
// every pattern in pattern, or zero when there is none. The walk is bounded so that a malformed tree — one whose Parent
// links form a cycle — cannot spin here forever.
func ancestorWithPattern(t *accessibility.Tree, id accessibility.NodeID, pattern PatternSet,
) accessibility.NodeID {
	for depth := 0; depth < maxTreeDepth; depth++ {
		parentID := t.UnignoredParent(id)
		if parentID == 0 {
			return 0
		}
		if Patterns(t.Node(parentID)).Has(pattern) {
			return parentID
		}
		id = parentID
	}
	return 0
}

// GridItem returns the id of the cell IGridProvider::GetItem reports for a row and a column of the grid with the
// given id, or zero when the snapshot does not hold it.
//
// Zero is the common answer rather than an error: a table publishes only the rows in its viewport, so most of a large
// table's cells are genuinely not there to be handed over, and UI Automation is told so with a NULL element rather
// than with a failure. The row is found by the RowIndex it recorded rather than by its position among the rows
// published, which is what makes the answer right while scrolled.
func GridItem(t *accessibility.Tree, id accessibility.NodeID, row, column int) accessibility.NodeID {
	if row < 0 || column < 0 {
		return 0
	}
	if t.Node(id) == nil {
		return 0
	}
	// Only a table and a tree support the Grid pattern, and the rows of both are Row nodes.
	rowID := accessibility.NodeID(0)
	for _, candidate := range itemsWithin(t, id, role.Row, nil) {
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

// TableColumnHeaders returns the ids of the column headers ITableProvider::GetColumnHeaders reports for the table
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
// for it once per cell; see snapshotMemo.
func TableColumnHeaders(t *accessibility.Tree, id accessibility.NodeID) []accessibility.NodeID {
	header := memoizedTableHeaderFor(t, id)
	if header == 0 {
		return nil
	}
	return itemsWithin(t, header, role.ColumnHeader, nil)
}

// setPosition is one node's answer to PositionInSet: where in its set it sits, and how large that set is.
type setPosition struct {
	position int
	size     int
}

// snapshotMemo remembers, for one snapshot at a time, the answers that cost a walk of that snapshot to work out.
// Three questions are asked often enough, and cost enough, to be worth remembering:
//
//   - Which TableHeader describes which table. Working it out takes a walk of the whole tree plus a scan of one subtree
//     per ancestor, and ITableItemProvider::GetColumnHeaderItems asks for it once per cell, so a screen reader stepping
//     through a table would otherwise pay that cost for every cell it reads.
//   - Which nodes some other node says it is labeled by, which is what decides whether a label belongs to the content
//     view. Answering it takes a scan of every node in the snapshot, and IsContentElement is asked it of every
//     label, so a client walking a form would otherwise pay labels × nodes per pass. See memoizedNamesAnother.
//   - Where a node sits in its set, which takes a walk up to the container that holds the count plus a scan of the
//     node's siblings. PositionInSet and SizeOfSet are two properties carrying the two halves of that one answer, and a
//     client reading an element reads both.
//   - How the text an element hands the Text pattern out over divides into the units a screen reader reads it in.
//     Working the divisions out costs several passes over the whole of it, and a client reading asks for them once
//     per line, word or character it moves the caret over — a say-all over a long document is thousands of calls,
//     every one of which would otherwise re-divide the text. See memoizedTextDocument.
//   - Which stretch of which document's stream each element occupies. ProvidedPatterns asks it of every element a
//     client so much as looks at, in both of the snapshots a publish compares, and answering it by scanning a
//     document's span list costs a pass over as many spans as the document has blocks and inline elements — which over
//     a whole tree is quadratic. See memoizedTextSpans.
//
// Remembering them is correct only because a published snapshot is immutable: a publish swaps a whole new tree in
// rather than editing the one providers are answering from, so an answer worked out from a tree cannot go out of date
// while that tree is still the one being asked about. Only one tree is kept, which is what a client walking one window
// needs and which keeps this from holding every snapshot a window ever published alive; a caller that alternates
// between two snapshots — raisePropertyChanged, which reports a property's value before and after — works the older
// tree's answers out every time, exactly as it did before anything was remembered. The one tree kept is a strong
// reference all the same, so Window.Destroy drops it: otherwise the last snapshot of a window, and every node in it,
// would stay reachable for the rest of the process.
//
// Every answer is worked out the first time it is asked for rather than when a snapshot arrives, and no map is built
// until there is something to put in it, so a window nothing asks these questions of allocates nothing here.
//
// UI Automation calls providers on whichever thread it likes, so every access is under the lock, the computation
// included: it is short, and several threads working the same answer out at once is the thing being avoided.
var snapshotMemo struct {
	tree          *accessibility.Tree
	headers       map[accessibility.NodeID]accessibility.NodeID
	named         map[accessibility.NodeID]bool
	positions     map[accessibility.NodeID]setPosition
	documents     map[accessibility.NodeID]*textDocument
	textSpans     map[accessibility.NodeID]documentSpan
	namedDone     bool
	textSpansDone bool
	lock          sync.Mutex
}

// forgetSnapshotMemo drops whatever snapshotMemo is holding, so that a snapshot nothing is answering from any
// more is not kept alive by it. A window being destroyed calls it; the next question asked of any window works its
// answer out again, which costs one walk of that window's tree.
func forgetSnapshotMemo() {
	snapshotMemo.lock.Lock()
	defer snapshotMemo.lock.Unlock()
	memoSwitchTo(nil)
}

// memoSwitchTo prepares the memo to answer questions about t and reports whether it will, emptying it unless it is
// already holding the answers worked out from t. The lock must be held. A nil tree empties it, is never remembered and
// is never served, since nothing answers from one.
//
// A snapshot the memo has already moved past is refused rather than let in, which is what keeps the memo on the
// snapshot a client is actually walking. The publish path asks about the previous tree and then about the current one
// for every property it raises — see raisePropertyChanged, and attributeProperties, of which a single
// AttributesChanged node raises nine — so an older tree allowed in here would throw away the headers, labels and
// positions a client's own thread had built for the current snapshot, once per property, to remember answers nothing
// will ask for a second time. The caller works those out for itself instead.
//
// Two snapshots are of the same window when they name the same root: node ids come from one process-wide counter and a
// panel keeps its id for life, so no two windows share one. Generation orders the snapshots of that window, and a lower
// one is therefore a tree this memo has already been asked to move on from. A tree from another window is not compared
// at all and takes the memo over as it always did, which is what keeps a newly opened dialog from being starved of the
// memo by a window that has been publishing for hours.
func memoSwitchTo(t *accessibility.Tree) bool {
	held := snapshotMemo.tree
	if t != nil && held == t {
		return true
	}
	if t != nil && held != nil && held.Root == t.Root && held.Generation > t.Generation {
		return false
	}
	snapshotMemo.tree = t
	snapshotMemo.headers = nil
	snapshotMemo.named = nil
	snapshotMemo.namedDone = false
	snapshotMemo.positions = nil
	snapshotMemo.documents = nil
	snapshotMemo.textSpans = nil
	snapshotMemo.textSpansDone = false
	return t != nil
}

// memoizedTableHeaderFor answers tableHeaderFor from snapshotMemo, working the answer out and remembering it
// whenever the memo does not already hold one for this tree. A snapshot the memo will not serve is answered from
// itself, outside the lock, since working one answer out there would hold every other thread up for the length of a
// walk of a whole tree.
func memoizedTableHeaderFor(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	if t == nil {
		return 0
	}
	snapshotMemo.lock.Lock()
	if !memoSwitchTo(t) {
		snapshotMemo.lock.Unlock()
		return tableHeaderFor(t, id)
	}
	defer snapshotMemo.lock.Unlock()
	if header, ok := snapshotMemo.headers[id]; ok {
		return header
	}
	header := tableHeaderFor(t, id)
	if snapshotMemo.headers == nil {
		snapshotMemo.headers = make(map[accessibility.NodeID]accessibility.NodeID)
	}
	snapshotMemo.headers[id] = header
	return header
}

// memoizedNamesAnother reports whether any node in the tree says it is labeled by the node with the given id,
// answering from snapshotMemo.
//
// The whole snapshot's answer is worked out at once rather than one label at a time, because the scan is the expensive
// part and one of them collects every named node there is: the question IsContentElement asks of each label is
// whether that label is among them. A snapshot in which nothing names anything leaves the set nil, which answers every
// label without having allocated anything at all — and a window with no LabeledBy relation in it is the common case.
//
// A snapshot the memo will not serve is scanned for the one answer, outside the lock and remembering nothing. The
// decider asks this of the previous snapshot for every label whose LabeledBy changed; see labelContent.
func memoizedNamesAnother(t *accessibility.Tree, id accessibility.NodeID) bool {
	if t == nil {
		return false
	}
	snapshotMemo.lock.Lock()
	if !memoSwitchTo(t) {
		snapshotMemo.lock.Unlock()
		return namedNodes(t)[id]
	}
	defer snapshotMemo.lock.Unlock()
	if !snapshotMemo.namedDone {
		snapshotMemo.named = namedNodes(t)
		snapshotMemo.namedDone = true
	}
	return snapshotMemo.named[id]
}

// namedNodes returns the set of ids that some node in the tree says it is labeled by, or nil when no node names
// another.
func namedNodes(t *accessibility.Tree) map[accessibility.NodeID]bool {
	var named map[accessibility.NodeID]bool
	for _, n := range t.Nodes {
		for _, labelID := range n.LabeledBy {
			if named == nil {
				named = make(map[accessibility.NodeID]bool)
			}
			named[labelID] = true
		}
	}
	return named
}

// memoizedPositionInSet answers positionInSet from snapshotMemo, working the answer out and remembering it
// whenever the memo does not already hold one for this node. The answer is remembered even when it is that the node is
// not one of a numbered set, since a client asks for both halves of it either way.
//
// PositionInSet is the only caller, and has already established that both the tree and the node are there.
//
// A snapshot the memo will not serve is answered from itself, outside the lock and remembering nothing. The publish
// path asks for both halves of this of both snapshots for every AttributesChanged node, the previous one included; see
// attributeProperties.
func memoizedPositionInSet(t *accessibility.Tree, n *accessibility.Node) (position, size int) {
	snapshotMemo.lock.Lock()
	if !memoSwitchTo(t) {
		snapshotMemo.lock.Unlock()
		return positionInSet(t, n)
	}
	defer snapshotMemo.lock.Unlock()
	if answer, ok := snapshotMemo.positions[n.ID]; ok {
		return answer.position, answer.size
	}
	position, size = positionInSet(t, n)
	if snapshotMemo.positions == nil {
		snapshotMemo.positions = make(map[accessibility.NodeID]setPosition)
	}
	snapshotMemo.positions[n.ID] = setPosition{position: position, size: size}
	return position, size
}

// tableHeaderFor returns the id of the TableHeader node that describes the columns of the table with the given id,
// or zero when there is none to be found. See TableColumnHeaders for how it is looked for and why.
func tableHeaderFor(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	table := t.Node(id)
	if table == nil || !Patterns(table).Has(PatternTable) {
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
	for depth := 0; ancestor != 0 && depth < maxTreeDepth; depth++ {
		if n := t.Node(ancestor); n != nil && (n.Role == role.Table || n.Role == role.Tree) {
			// Another table contains this one, which a table nested in a cell really is. Its header describes its own
			// columns, not this table's, and everything further out belongs to it as well, so the search ends here
			// rather than climbing out of the container and claiming the outer table's column names.
			return 0
		}
		headers, tables := headersAndTablesWithin(t, ancestor, make(map[accessibility.NodeID]bool), 0)
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

// headersAndTablesWithin returns the ids of the table headers within the subtree rooted at the node with the given
// id, along with how many tables it holds. Ignored nodes are walked through — a header could sit inside a layout panel
// — but an ignored node is never reported as a header, since it has no provider to hand a client.
//
// visited holds the ids already reached, and bounds this the way it bounds appendItems: each node is looked at once,
// so a duplicated child cannot count the same table twice — which would end the search for a header that is really
// there — and a cyclic Children link cannot spin here forever. The depth bound stays as a second line of defense, since
// the recursion is on the stack.
func headersAndTablesWithin(t *accessibility.Tree, id accessibility.NodeID, visited map[accessibility.NodeID]bool,
	depth int,
) (headers []accessibility.NodeID, tables int) {
	n := t.Node(id)
	if n == nil || depth >= maxTreeDepth || visited[id] {
		return nil, 0
	}
	visited[id] = true
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
		childHeaders, childTables := headersAndTablesWithin(t, childID, visited, depth+1)
		headers = append(headers, childHeaders...)
		tables += childTables
	}
	return headers, tables
}

// ColumnHeaderItem returns the id of the column header ITableItemProvider::GetColumnHeaderItems reports for the cell
// with the given id, or zero when there is none.
//
// The header of a cell's column is the one whose ColumnIndex matches the cell's. A snapshot that never filled in the
// headers' column indexes — they all read zero — would then answer only for the first column, so a header is taken by
// position when no index matches, which is right whenever the header publishes one element per column in column order.
func ColumnHeaderItem(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	cell := t.Node(id)
	if cell == nil {
		return 0
	}
	grid := ContainingGrid(t, id)
	if grid == 0 {
		return 0
	}
	headers := TableColumnHeaders(t, grid)
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

// ValueString returns the text IValueProvider::get_Value reports for a node. A text control's content is its value;
// everything else with a value reports it in textual form already. A password field reports nothing at all, which is
// the whole point of the Protected flag: the snapshot never fills in its value or its text either, and this is the
// second line of that defense.
//
// A link's value is where it leads. UI Automation has no property for a hyperlink's destination, so the Value pattern
// is where every client looks for one, and the pattern is handed out precisely when there is a URL to report; see
// rolePatterns. It wins over Value for that one role, since the URL is what the pattern was granted for — a link's
// text is its name, and reporting the text twice would tell a client nothing about where following it would go.
//
// Falling back to the text reaches only the nodes that hand out both patterns, which is a field, a spin button or a
// combo box: a label, a heading and a plain column header carry text and no Value pattern at all, so nothing ever asks
// this of them, and a cell's Value pattern is gated on the widget having filled a value in. See rolePatterns.
func ValueString(n *accessibility.Node) string {
	if n == nil || n.Protected {
		return ""
	}
	if n.Role == role.Link && n.URL != "" {
		return n.URL
	}
	if n.Value != "" {
		return n.Value
	}
	if n.Role.IsText() && n.Text != nil {
		return n.Text.Text
	}
	return ""
}

// IsValueReadOnly reports whether IValueProvider::get_IsReadOnly says the value cannot be changed. Three things make
// it read-only:
//
//   - the role. A color well and a popup button report a value so that a client can say what is currently chosen, but
//     neither takes a new one through it, a link reports where it leads and nothing retargets a link through
//     accessibility, and a document is there to be read.
//   - the node saying so, through ReadOnly.
//   - the node not offering the SetValue action, which is the snapshot's own statement that nothing will happen if a
//     client tries. Declaring a value writable and then refusing every attempt is worse than saying so up front.
//
// Being disabled is deliberately not one of them. Read-only says the value could never be set through this element;
// disabled says nothing about the value at all, only that the element cannot be used just now, and UI Automation has
// IsEnabled for that. The SetValue paths refuse a disabled element with E_ELEMENTNOTENABLED before they ever ask
// this, so nothing is let through by the distinction.
func IsValueReadOnly(n *accessibility.Node) bool {
	if n == nil {
		return true
	}
	switch n.Role {
	case role.ColorWell, role.PopupButton, role.Document, role.Link:
		return true
	default:
		return n.ReadOnly || !n.Actions.Has(accessibility.SetValue)
	}
}

// IsRangeValueReadOnly reports whether IRangeValueProvider::get_IsReadOnly says the value cannot be changed. A
// progress bar is read-only by definition — it reports what the application is doing, and nothing outside the
// application decides that — and everything else follows the same rules as IsValueReadOnly, the disabled case
// included.
func IsRangeValueReadOnly(n *accessibility.Node) bool {
	if n == nil {
		return true
	}
	if n.Role == role.ProgressBar {
		return true
	}
	return n.ReadOnly || !n.Actions.Has(accessibility.SetValue)
}
