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
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// This file turns an accessibility snapshot into the answers UI Automation wants. Everything here is a pure function of
// the snapshot: nothing touches the OS, allocates COM memory or depends on which thread it runs on, which is why the
// file carries no build constraint and its tests run on any platform. The Windows-only files translate these answers
// into VARIANTs, SAFEARRAYs and UiaRaise* calls, and nothing more.

// PatternSet is the set of UI Automation control patterns one element supports, held as a bit per pattern. A provider's
// QueryInterface and GetPatternProvider must agree about this set — a client that obtains a pattern interface one way
// and not the other treats the element as broken — so both consult UIAPatterns rather than deciding for themselves.
type PatternSet uint32

// The patterns this package implements. There is deliberately no bit for a pattern the provider does not implement: a
// missing bit and an unimplemented pattern must be the same thing.
const (
	PatternInvoke PatternSet = 1 << iota
	PatternToggle
	PatternValue
	PatternRangeValue
	PatternSelection
	PatternSelectionItem
	PatternExpandCollapse
	PatternScrollItem
	PatternGrid
	PatternGridItem
	PatternTable
	PatternTableItem
	PatternWindow
)

// uiaPatternInfo pairs one PatternSet bit with the UI Automation identifier clients ask for it by and a name for
// diagnostics.
type uiaPatternInfo struct {
	name    string
	id      PatternID
	pattern PatternSet
}

// uiaPatternInfos lists every pattern this package implements, in bit order.
var uiaPatternInfos = []uiaPatternInfo{
	{name: "invoke", id: UIA_InvokePatternId, pattern: PatternInvoke},
	{name: "toggle", id: UIA_TogglePatternId, pattern: PatternToggle},
	{name: "value", id: UIA_ValuePatternId, pattern: PatternValue},
	{name: "range-value", id: UIA_RangeValuePatternId, pattern: PatternRangeValue},
	{name: "selection", id: UIA_SelectionPatternId, pattern: PatternSelection},
	{name: "selection-item", id: UIA_SelectionItemPatternId, pattern: PatternSelectionItem},
	{name: "expand-collapse", id: UIA_ExpandCollapsePatternId, pattern: PatternExpandCollapse},
	{name: "scroll-item", id: UIA_ScrollItemPatternId, pattern: PatternScrollItem},
	{name: "grid", id: UIA_GridPatternId, pattern: PatternGrid},
	{name: "grid-item", id: UIA_GridItemPatternId, pattern: PatternGridItem},
	{name: "table", id: UIA_TablePatternId, pattern: PatternTable},
	{name: "table-item", id: UIA_TableItemPatternId, pattern: PatternTableItem},
	{name: "window", id: UIA_WindowPatternId, pattern: PatternWindow},
}

// Has returns true if the set contains every pattern in patterns. Passing more than one bit therefore asks whether all
// of them are present.
func (s PatternSet) Has(patterns PatternSet) bool {
	return s&patterns == patterns
}

// String implements fmt.Stringer, listing the patterns in the set separated by commas. An empty set yields an empty
// string.
func (s PatternSet) String() string {
	var buffer strings.Builder
	for _, info := range uiaPatternInfos {
		if s&info.pattern != 0 {
			if buffer.Len() != 0 {
				buffer.WriteString(",")
			}
			buffer.WriteString(info.name)
		}
	}
	return buffer.String()
}

// PatternSetForID returns the single-bit PatternSet a UI Automation pattern identifier names, or zero when this package
// does not implement that pattern. A provider asked for an unimplemented pattern must return a nil interface, which is
// exactly what a zero result leads it to do.
func PatternSetForID(id PatternID) PatternSet {
	for _, info := range uiaPatternInfos {
		if info.id == id {
			return info.pattern
		}
	}
	return 0
}

// UIAControlType returns the UI Automation control type to report for a node. It is the single most consequential
// mapping in the adapter: a client derives the spoken control type, the patterns it bothers looking for, and the
// element's treatment in the control and content views from this one value. A role with no reasonable equivalent
// becomes Custom, which tells the client to rely on the name and the patterns instead of on a built-in behavior.
func UIAControlType(n *accessibility.Node) ControlTypeID {
	if n == nil {
		return UIA_CustomControlTypeId
	}
	switch n.Role {
	case role.Window, role.Dialog:
		return UIA_WindowControlTypeId
	case role.Group:
		return UIA_GroupControlTypeId
	case role.TabPanel, role.ScrollArea:
		return UIA_PaneControlTypeId
	case role.TableHeader:
		return UIA_HeaderControlTypeId
	case role.Button, role.ToggleButton, role.DisclosureTriangle, role.ColorWell:
		return UIA_ButtonControlTypeId
	case role.CheckBox:
		return UIA_CheckBoxControlTypeId
	case role.RadioButton:
		return UIA_RadioButtonControlTypeId
	case role.Link:
		return UIA_HyperlinkControlTypeId
	case role.Label, role.Heading:
		return UIA_TextControlTypeId
	case role.TextField, role.TextArea:
		return UIA_EditControlTypeId
	case role.SpinButton:
		return UIA_SpinnerControlTypeId
	case role.ComboBox, role.PopupButton:
		return UIA_ComboBoxControlTypeId
	case role.Slider:
		return UIA_SliderControlTypeId
	case role.ProgressBar:
		return UIA_ProgressBarControlTypeId
	case role.ScrollBar:
		return UIA_ScrollBarControlTypeId
	case role.Separator:
		return UIA_SeparatorControlTypeId
	case role.List:
		return UIA_ListControlTypeId
	case role.ListItem:
		return UIA_ListItemControlTypeId
	case role.Table:
		return UIA_TableControlTypeId
	case role.Tree:
		return UIA_DataGridControlTypeId
	case role.Row, role.Cell:
		return UIA_DataItemControlTypeId
	case role.ColumnHeader:
		return UIA_HeaderItemControlTypeId
	case role.TabList:
		return UIA_TabControlTypeId
	case role.Tab:
		return UIA_TabItemControlTypeId
	case role.MenuBar:
		return UIA_MenuBarControlTypeId
	case role.Menu:
		return UIA_MenuControlTypeId
	case role.MenuItem:
		return UIA_MenuItemControlTypeId
	case role.Image:
		return UIA_ImageControlTypeId
	case role.Tooltip:
		return UIA_ToolTipControlTypeId
	case role.Document:
		return UIA_DocumentControlTypeId
	case role.Toolbar:
		return UIA_ToolBarControlTypeId
	default:
		return UIA_CustomControlTypeId
	}
}

// UIAPatterns returns the patterns a node supports. The answer depends on the node's role and, for a few roles, on a
// state flag that decides whether a pattern is meaningful at all: a row is expandable only in a hierarchical table, a
// progress bar has a range only once it has a number, and a menu item toggles only when it carries a check mark.
//
// The set says which pattern interfaces exist, not which of their methods will work. A disabled button still supports
// Invoke; invoking it fails with UIA_E_ELEMENTNOTENABLED. Declaring the pattern and refusing the operation is what
// lets a client describe a control correctly while it is unusable.
func UIAPatterns(n *accessibility.Node) PatternSet {
	if n == nil {
		return 0
	}
	switch n.Role {
	case role.Window, role.Dialog:
		return PatternWindow
	case role.Button, role.ColorWell, role.Link, role.ColumnHeader:
		// A color well also reports its color through a read-only Value, and a column header sorts when invoked.
		patterns := PatternInvoke
		if n.Role == role.ColorWell {
			patterns |= PatternValue
		}
		return patterns
	case role.ToggleButton, role.DisclosureTriangle, role.CheckBox:
		return PatternToggle
	case role.RadioButton:
		// A radio button is a selection item rather than a toggle, which is how a client knows to announce "n of m"
		// alongside the state.
		return PatternSelectionItem
	case role.TextField, role.TextArea, role.Document:
		return PatternValue
	case role.SpinButton:
		return PatternValue | PatternRangeValue
	case role.ComboBox:
		return PatternValue | PatternExpandCollapse
	case role.PopupButton:
		// A popup button has no editable text, so its Value is read-only, but it is still the way the current choice
		// is reported.
		return PatternValue | PatternExpandCollapse
	case role.Slider, role.ScrollBar:
		return PatternRangeValue
	case role.ProgressBar:
		if n.HasNumber {
			return PatternRangeValue
		}
		return 0
	case role.List, role.TabList:
		return PatternSelection
	case role.ListItem, role.Tab:
		patterns := PatternSelectionItem
		if n.Role == role.ListItem {
			patterns |= PatternScrollItem
		}
		return patterns
	case role.Table, role.Tree:
		return PatternGrid | PatternTable | PatternSelection
	case role.Row:
		patterns := PatternSelectionItem | PatternScrollItem
		if n.Expandable {
			patterns |= PatternExpandCollapse
		}
		return patterns
	case role.Cell:
		return PatternGridItem | PatternTableItem
	case role.MenuItem:
		patterns := PatternInvoke
		if n.HasCheck {
			patterns |= PatternToggle
		}
		if n.Expandable {
			patterns |= PatternExpandCollapse
		}
		return patterns
	default:
		// Group, TabPanel, ScrollArea, TableHeader, Label, Heading, Image, Separator, MenuBar, Menu, Tooltip and
		// Toolbar all present themselves through their properties and their children alone.
		//
		// A Menu is the one of those that looks as though it should expand: the role belongs to the panel of an open
		// menu, which nothing ever collapses and which never reports Expandable, so ExpandCollapse would be a pattern
		// whose state is permanently LeafNode and whose two methods report success without doing anything. The menu
		// item that opened it carries the pattern instead, which is where a client looks for it.
		return 0
	}
}

// UIAIsControlElement reports whether a node belongs to UI Automation's control view, the view a client walks when it
// wants every element a user can perceive or act on. Only the nodes the snapshot marks Ignored are left out, since
// those exist purely to lay other things out and are spliced away by the navigation helpers anyway.
func UIAIsControlElement(n *accessibility.Node) bool {
	return n != nil && !n.Ignored
}

// UIAIsContentElement reports whether a node belongs to UI Automation's content view, the narrower view a client walks
// when it wants the information on screen rather than the machinery presenting it. Everything in the control view is in
// the content view except:
//
//   - separators, scroll bars and tooltips, which are presentation or transient commentary rather than content;
//   - a label that names another element, because that element already reports the label's text as its own name, so
//     leaving the label in the content view makes a screen reader say it twice.
//
// Finding out whether a label names something means looking at every node's LabeledBy, so this costs a walk of the tree
// for label nodes and nothing at all for the rest. A caller answering the property for many labels at once should
// remember the answers.
func UIAIsContentElement(t *accessibility.Tree, n *accessibility.Node) bool {
	if !UIAIsControlElement(n) {
		return false
	}
	switch n.Role {
	case role.Separator, role.ScrollBar, role.Tooltip:
		return false
	case role.Label:
		return !uiaNamesAnother(t, n.ID)
	default:
		return true
	}
}

// uiaNamesAnother reports whether any node in the tree says it is labeled by the node with the given id.
func uiaNamesAnother(t *accessibility.Tree, id accessibility.NodeID) bool {
	if t == nil {
		return false
	}
	for _, n := range t.Nodes {
		for _, labelID := range n.LabeledBy {
			if labelID == id {
				return true
			}
		}
	}
	return false
}

// UIAHeadingLevel returns the value of the HeadingLevel property for a node. Only a heading has one; everything else
// reports HeadingLevel_None, and so does a heading whose level the snapshot never filled in. Levels beyond nine clamp
// to the ninth, since UI Automation defines no more than that.
func UIAHeadingLevel(n *accessibility.Node) HeadingLevelID {
	if n == nil || n.Role != role.Heading || n.Level < 1 {
		return HeadingLevel_None
	}
	if n.Level > 9 {
		return HeadingLevel9
	}
	return HeadingLevel1 + HeadingLevelID(n.Level-1)
}

// UIAOrientation returns the value of the Orientation property for a node.
func UIAOrientation(n *accessibility.Node) OrientationType {
	if n == nil {
		return OrientationType_None
	}
	switch n.Orientation {
	case accessibility.OrientationHorizontal:
		return OrientationType_Horizontal
	case accessibility.OrientationVertical:
		return OrientationType_Vertical
	default:
		return OrientationType_None
	}
}

// UIAToggleState returns the state an element that supports the Toggle pattern reports. A toggle button's state is
// whether it is pressed; everything else checkable reports its check state, where a mixed check becomes
// ToggleState_Indeterminate.
func UIAToggleState(n *accessibility.Node) ToggleState {
	if n == nil {
		return ToggleState_Off
	}
	if n.Role == role.ToggleButton || !n.HasCheck {
		if n.Pressed {
			return ToggleState_On
		}
		return ToggleState_Off
	}
	switch n.Checked {
	case check.On:
		return ToggleState_On
	case check.Mixed:
		return ToggleState_Indeterminate
	default:
		return ToggleState_Off
	}
}

// UIAExpandCollapseState returns the state an element that supports the ExpandCollapse pattern reports. A node that
// cannot be expanded at all is a leaf, which is a different answer from being collapsed: a client announces a leaf
// silently and a collapsed node as collapsed.
func UIAExpandCollapseState(n *accessibility.Node) ExpandCollapseState {
	if n == nil || !n.Expandable {
		return ExpandCollapseState_LeafNode
	}
	if n.Expanded {
		return ExpandCollapseState_Expanded
	}
	return ExpandCollapseState_Collapsed
}

// UIAItemStatus returns the value of the ItemStatus property for a node, which is how a sorted column header tells a
// client which way it is sorted. A node that is not a sort key has no item status, reported as the empty string so that
// the provider answers VT_EMPTY.
func UIAItemStatus(n *accessibility.Node) string {
	if n == nil || n.Sort == accessibility.SortNone {
		return ""
	}
	return n.Sort.String()
}

// UIAWindowInteractionState returns the value of the WindowInteractionState property for a fragment root. A window the
// snapshot reports as disabled is disabled because something modal is in front of it, which is the one distinction UI
// Automation cares about here.
func UIAWindowInteractionState(root *accessibility.Node) WindowInteractionState {
	if root != nil && root.Disabled {
		return WindowInteractionState_BlockedByModalWindow
	}
	return WindowInteractionState_ReadyForUserInteraction
}

// UIASmallChange returns the value of the RangeValue pattern's SmallChange property: how far one press of an arrow key
// moves the value.
func UIASmallChange(n *accessibility.Node) float64 {
	if n == nil {
		return 0
	}
	return n.Step
}

// UIALargeChange returns the value of the RangeValue pattern's LargeChange property: how far one press of a paging key
// moves the value. Ten small steps is the conventional answer for a control that does not say.
func UIALargeChange(n *accessibility.Node) float64 {
	if n == nil {
		return 0
	}
	return n.Step * 10
}

// UIARuntimeID returns the runtime identifier for a node other than a fragment root. It begins with
// UiaAppendRuntimeId, which tells UI Automation to prepend the fragment root's own identifier, and continues with the
// two halves of the node id — runtime identifiers are arrays of 32-bit integers, and a node id is 64 bits wide.
//
// A fragment root has no runtime identifier of its own to report: it answers GetRuntimeId with a NULL array and lets UI
// Automation use the window handle.
func UIARuntimeID(id accessibility.NodeID) []int32 {
	return []int32{UiaAppendRuntimeId, int32(uint32(id)), int32(uint32(id >> 32))}
}

// UIANavigate returns the id of the node IRawElementProviderFragment::Navigate should move to from the node with the
// given id, or zero when there is nothing in that direction.
//
// Navigation runs over the unignored tree, so the layout panels the snapshot marks Ignored are invisible here: their
// children take their place as children of the nearest unignored ancestor. The fragment root has no parent and no
// siblings, so those three directions report nothing for it, and an Ignored node — which has no provider to navigate
// from in the first place — reports nothing for either sibling direction, since it is not among the unignored children
// of its own unignored parent. Its children and its parent are still answered, from the unignored tree like everything
// else, so a caller that asks anyway is told something consistent rather than nothing.
func UIANavigate(t *accessibility.Tree, id accessibility.NodeID, direction NavigateDirection) accessibility.NodeID {
	if t == nil || t.Node(id) == nil {
		return 0
	}
	switch direction {
	case NavigateDirection_Parent:
		if id == t.Root {
			return 0
		}
		return t.UnignoredParent(id)
	case NavigateDirection_FirstChild:
		if children := t.UnignoredChildren(id); len(children) != 0 {
			return children[0]
		}
		return 0
	case NavigateDirection_LastChild:
		if children := t.UnignoredChildren(id); len(children) != 0 {
			return children[len(children)-1]
		}
		return 0
	case NavigateDirection_NextSibling, NavigateDirection_PreviousSibling:
		return uiaSibling(t, id, direction == NavigateDirection_NextSibling)
	default:
		return 0
	}
}

// uiaSibling returns the id of the node before or after the given one among the unignored children of its unignored
// parent, or zero when there is none.
func uiaSibling(t *accessibility.Tree, id accessibility.NodeID, next bool) accessibility.NodeID {
	if id == t.Root {
		return 0
	}
	siblings := t.UnignoredChildren(t.UnignoredParent(id))
	for i, siblingID := range siblings {
		if siblingID != id {
			continue
		}
		if next {
			if i+1 < len(siblings) {
				return siblings[i+1]
			}
			return 0
		}
		if i > 0 {
			return siblings[i-1]
		}
		return 0
	}
	return 0
}

// UIAHitTest returns the id of the node IRawElementProviderFragmentRoot::ElementProviderFromPoint should report for a
// point, or zero when the point is outside the window. pt is in the same window-local, top-left origin, logical
// coordinate space as Node.Bounds, so the caller converts from the screen coordinates UI Automation supplies first.
//
// The answer is always an unignored node, since an Ignored one has no provider: a hit on a layout panel is reported as
// a hit on the nearest unignored ancestor.
func UIAHitTest(t *accessibility.Tree, pt geom.Point) accessibility.NodeID {
	hit := t.HitTest(pt)
	if hit == 0 {
		return 0
	}
	if n := t.Node(hit); n != nil && n.Ignored {
		return t.UnignoredParent(hit)
	}
	return hit
}

// UIAPositionInSet returns the values of the PositionInSet and SizeOfSet properties for a node, or zero for both when
// the node is not one of a numbered set and the properties should be left unreported.
//
// Rows and list items are numbered from what the snapshot recorded, not from what is in the tree: a table exposes only
// the rows in its viewport, so counting siblings would tell the user they are on row 3 of 12 while scrolled to the
// bottom of a thousand. Everything else numbered — tabs and menu items — is fully present in the tree, so its position
// comes from counting siblings that share its role.
func UIAPositionInSet(t *accessibility.Tree, n *accessibility.Node) (position, size int) {
	if t == nil || n == nil || n.Ignored {
		return 0, 0
	}
	if n.Role.IsRowLike() {
		if count := uiaContainerRowCount(t, n); count > 0 && n.RowIndex >= 0 && n.RowIndex < count {
			return n.RowIndex + 1, count
		}
	}
	switch n.Role {
	case role.Row, role.ListItem, role.Tab, role.MenuItem:
		return t.PositionInSet(n.ID)
	default:
		return 0, 0
	}
}

// uiaMaxTreeDepth bounds how far the helpers here will walk up a tree.
const uiaMaxTreeDepth = 512

// uiaContainerRowCount returns the row count of the nearest ancestor of n that reports one, or zero when no ancestor
// does. This is the total number of rows the container holds, which is not the number the snapshot exposes whenever the
// container only publishes the rows in its viewport.
//
// The walk is bounded so that a malformed tree — one whose Parent links form a cycle — cannot spin here forever. Real
// hierarchies are far shallower than the limit.
func uiaContainerRowCount(t *accessibility.Tree, n *accessibility.Node) int {
	parent := t.Node(n.Parent)
	for depth := 0; parent != nil && depth < uiaMaxTreeDepth; depth++ {
		if parent.RowCount > 0 {
			return parent.RowCount
		}
		parent = t.Node(parent.Parent)
	}
	return 0
}

// UIARaiseKind says which UI Automation call a UIARaise stands for.
type UIARaiseKind uint8

// Possible UIARaiseKind values.
const (
	// UIARaiseEvent is a UiaRaiseAutomationEvent call: raise UIARaise.Event on UIARaise.Node.
	UIARaiseEvent UIARaiseKind = iota
	// UIARaiseProperty is a UiaRaiseAutomationPropertyChangedEvent call: report UIARaise.Property as changed on
	// UIARaise.Node. The new value is read from the node itself, so that the value a client asks for and the value it
	// was told about cannot disagree.
	UIARaiseProperty
	// UIARaiseStructure is a UiaRaiseStructureChangedEvent call: report UIARaise.Change on UIARaise.Node, naming
	// UIARaise.Child when the change is a child being added or removed.
	UIARaiseStructure
	// UIARaiseNotify is a UiaRaiseNotificationEvent call: announce UIARaise.Text from UIARaise.Node.
	UIARaiseNotify
	// UIARaiseDisconnect is not an event at all: it says that UIARaise.Node has left the tree, so its provider should
	// be marked stale, disconnected from UI Automation and released. It always follows any structure change that
	// reported the removal.
	UIARaiseDisconnect
)

// String implements fmt.Stringer.
func (k UIARaiseKind) String() string {
	switch k {
	case UIARaiseEvent:
		return "event"
	case UIARaiseProperty:
		return "property"
	case UIARaiseStructure:
		return "structure"
	case UIARaiseNotify:
		return "notify"
	case UIARaiseDisconnect:
		return "disconnect"
	default:
		return "UIARaiseKind(" + strconv.FormatUint(uint64(k), 10) + ")"
	}
}

// UIARaise is one thing the adapter should tell UI Automation about, as decided by UIADecideRaises. Which fields carry
// information depends on Kind; the rest are left at their zero values.
type UIARaise struct {
	// Text is what to announce, for UIARaiseNotify.
	Text string
	// Node is the element the call is made on.
	Node accessibility.NodeID
	// Child is the element a structure change is about, when the change names one.
	Child accessibility.NodeID
	// Event is the automation event to raise, for UIARaiseEvent.
	Event EventID
	// Property is the property to report as changed, for UIARaiseProperty.
	Property PropertyID
	// Change is the kind of structure change, for UIARaiseStructure.
	Change StructureChangeType
	// Kind says which call this is.
	Kind UIARaiseKind
}

// String implements fmt.Stringer, in a form meant for test failures rather than for users.
func (r UIARaise) String() string {
	var buffer strings.Builder
	buffer.WriteString(r.Kind.String())
	buffer.WriteString("{node:")
	buffer.WriteString(strconv.FormatUint(uint64(r.Node), 10))
	switch r.Kind {
	case UIARaiseEvent:
		buffer.WriteString(",event:")
		buffer.WriteString(strconv.FormatInt(int64(r.Event), 10))
	case UIARaiseProperty:
		buffer.WriteString(",property:")
		buffer.WriteString(strconv.FormatInt(int64(r.Property), 10))
	case UIARaiseStructure:
		buffer.WriteString(",change:")
		buffer.WriteString(strconv.FormatInt(int64(r.Change), 10))
		if r.Child != 0 {
			buffer.WriteString(",child:")
			buffer.WriteString(strconv.FormatUint(uint64(r.Child), 10))
		}
	case UIARaiseNotify:
		buffer.WriteString(",text:")
		buffer.WriteString(strconv.Quote(r.Text))
	default:
	}
	buffer.WriteString("}")
	return buffer.String()
}

// UIADecideRaises turns the events describing how cur differs from old into the UI Automation calls the adapter should
// make, in the order it should make them. It is pure: neither tree is touched, nothing outside them and the events is
// consulted, and the same inputs always produce the same output. The caller supplies the events rather than having them
// derived here, so that one diff can feed every platform's adapter.
//
// The translation is mostly one event to one call, with these rules on top:
//
//   - A focus event is raised only while the root reports itself focused. Telling a client that the focus moved inside
//     a window that is not the active one makes a screen reader jump to a window the user is not looking at.
//   - A window that becomes active re-raises the focus, since the client needs to be pointed back at whatever inside
//     the window has it.
//   - A bounding-rectangle change is raised only for the focused node and for the root. Every node's bounds change when
//     a window is resized, and a client that wanted them all would ask.
//   - A property change is raised only when the element actually supports the property. A slider reports its value
//     through the RangeValue pattern and a text field through the Value pattern, so the same ValueChanged event becomes
//     a different property on each, and becomes nothing at all on an element with neither pattern.
//   - When a node's children changed, the client is told once, with ChildrenInvalidated, and the individual additions
//     and removals anywhere beneath that node are dropped. A client responds to ChildrenInvalidated by reading the
//     children again, so the per-child events would only make it do the same work repeatedly — and a whole subtree
//     appearing at once would otherwise report every element in it.
//   - A removal whose parent also left the tree reports no structure change either, since there is no provider left to
//     raise it on, but it still reports the disconnect that releases the removed node's provider.
//   - Nodes the snapshot marks Ignored never appear: they have no provider, so there is nothing to raise an event on.
//     The exception is the moment the flag itself flips, which is the node joining or leaving the tree a client sees
//     while nobody's list of children changed — a scroll bar appearing as its content outgrows its view port is exactly
//     this — so the nearest unignored parent is told to read its children again.
//   - UI Automation's two text events are never raised. Both belong to the Text control pattern, which this package
//     does not implement and which every element here answers NULL for, so a client that responded to one by asking for
//     ITextProvider would have nothing to read. An edit reports the Value pattern's property instead, which is where
//     the text a client reads comes from.
//   - Duplicates are dropped. One text edit arrives as a value change and as the deletion and the insertion that made
//     it, and all three ask for the same value property, so without this a client would hear about one edit three
//     times.
//
// The first publish of a window is the one case that produces a call with no event behind it: a dialog announces
// itself with Window_WindowOpened, which is how a screen reader knows to read the whole dialog out. Diff reports
// nothing but the focus for a first publish, since there is no previous tree to compare against, so old being nil is
// what identifies that moment.
func UIADecideRaises(old, cur *accessibility.Tree, events []accessibility.Event) []UIARaise {
	if cur == nil {
		return nil
	}
	d := &uiaDecider{
		old:         old,
		cur:         cur,
		invalidated: make(map[accessibility.NodeID]bool),
		seen:        make(map[UIARaise]bool),
	}
	if root := cur.Node(cur.Root); root != nil {
		d.rootFocused = root.Focused
		if old == nil && root.Role == role.Dialog {
			d.add(UIARaise{Kind: UIARaiseEvent, Node: cur.Root, Event: UIA_Window_WindowOpenedEventId})
		}
	}
	for _, event := range events {
		if target := d.invalidationTarget(event); target != 0 {
			d.invalidated[target] = true
		}
	}
	for _, event := range events {
		d.translate(event)
	}
	return d.raises
}

// uiaDecider holds the state UIADecideRaises threads through the translation of one batch of events.
type uiaDecider struct {
	old         *accessibility.Tree
	cur         *accessibility.Tree
	invalidated map[accessibility.NodeID]bool
	seen        map[UIARaise]bool
	raises      []UIARaise
	rootFocused bool
}

// add records one call to make, dropping it when it would duplicate one already recorded or when it names a node with
// no provider behind it. Announcements are never treated as duplicates: saying the same thing twice is a legitimate
// request.
func (d *uiaDecider) add(raise UIARaise) {
	if raise.Node == 0 {
		return
	}
	switch raise.Kind {
	case UIARaiseEvent, UIARaiseProperty:
		if n := d.cur.Node(raise.Node); n == nil || n.Ignored {
			return
		}
	default:
	}
	if raise.Kind != UIARaiseNotify {
		if d.seen[raise] {
			return
		}
		d.seen[raise] = true
	}
	d.raises = append(d.raises, raise)
}

// invalidationTarget returns the node a client must be told to read the children of again because of one event, or
// zero when the event asks for no such thing. Two kinds do: a node whose list of children changed, and a node whose
// Ignored flag flipped, which joins it to or removes it from the tree a client sees without any list of children
// having changed at all.
func (d *uiaDecider) invalidationTarget(event accessibility.Event) accessibility.NodeID {
	switch {
	case event.Kind == accessibility.ChildrenChanged:
		return d.target(d.cur, event.Node)
	case event.Kind == accessibility.StateChanged && event.State == accessibility.StateIgnored:
		return d.cur.UnignoredParent(event.Node)
	default:
		return 0
	}
}

// target returns the node an event about the given id should be raised on: the node itself when it has a provider, or
// its nearest unignored ancestor when it does not.
func (d *uiaDecider) target(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	n := t.Node(id)
	if n == nil {
		return 0
	}
	if !n.Ignored {
		return id
	}
	return t.UnignoredParent(id)
}

// translate records the calls one event asks for.
func (d *uiaDecider) translate(event accessibility.Event) {
	switch event.Kind {
	case accessibility.FocusChanged:
		d.focus(event.Node)
	case accessibility.NameChanged:
		d.property(event.Node, UIA_NamePropertyId)
	case accessibility.DescriptionChanged:
		d.property(event.Node, UIA_HelpTextPropertyId)
	case accessibility.ValueChanged:
		d.valueProperty(event.Node)
	case accessibility.NumberChanged:
		if d.patterns(event.Node).Has(PatternRangeValue) {
			d.property(event.Node, UIA_RangeValueValuePropertyId)
		}
	case accessibility.StateChanged:
		d.state(event)
	case accessibility.TextInserted, accessibility.TextDeleted:
		// UIA_Text_TextChangedEventId belongs to the Text pattern, which nothing here implements, so the edit is
		// reported through the value property alone. See the rule in the doc comment on UIADecideRaises.
		d.valueProperty(event.Node)
	case accessibility.TextSelectionChanged:
		// Nothing. UIA_Text_TextSelectionChangedEventId is the other half of the Text pattern, and a caret move changes
		// no property this package answers, so there is nothing a client could read even if it were told.
	case accessibility.ChildrenChanged:
		d.add(UIARaise{
			Kind:   UIARaiseStructure,
			Node:   d.target(d.cur, event.Node),
			Change: StructureChangeType_ChildrenInvalidated,
		})
	case accessibility.NodeAdded:
		d.added(event.Node)
	case accessibility.NodeRemoved:
		d.removed(event.Node)
	case accessibility.BoundsChanged:
		if event.Node == d.cur.Focus || event.Node == d.cur.Root {
			d.property(event.Node, UIA_BoundingRectanglePropertyId)
		}
	case accessibility.SortChanged:
		d.property(event.Node, UIA_ItemStatusPropertyId)
	case accessibility.RoleChanged:
		// A role change is a control-type change: UIAControlType is a pure function of the role, and the control type
		// is what a client derives the spoken name of the element and the patterns it bothers looking for from. The
		// localized control type is not raised alongside it, since this package never answers that property — UI
		// Automation derives it from the control type, which it has just been told about.
		d.property(event.Node, UIA_ControlTypePropertyId)
	case accessibility.WindowActivated:
		d.focus(d.cur.Focus)
	case accessibility.Announcement:
		if event.New != "" {
			d.add(UIARaise{Kind: UIARaiseNotify, Node: d.cur.Root, Text: event.New})
		}
	default:
		// WindowDeactivated has no UI Automation equivalent: the window that became active raises the events that
		// matter.
	}
}

// patterns returns the patterns the node with the given id supports in the current tree.
func (d *uiaDecider) patterns(id accessibility.NodeID) PatternSet {
	return UIAPatterns(d.cur.Node(id))
}

// event records an automation event.
func (d *uiaDecider) event(id accessibility.NodeID, eventID EventID) {
	d.add(UIARaise{Kind: UIARaiseEvent, Node: id, Event: eventID})
}

// property records a property change.
func (d *uiaDecider) property(id accessibility.NodeID, propertyID PropertyID) {
	d.add(UIARaise{Kind: UIARaiseProperty, Node: id, Property: propertyID})
}

// focus records a focus change, which is worth reporting only while the window itself is active.
func (d *uiaDecider) focus(id accessibility.NodeID) {
	if d.rootFocused {
		d.event(id, UIA_AutomationFocusChangedEventId)
	}
}

// valueProperty records the change of whichever value property the node actually has, and nothing when it has neither.
func (d *uiaDecider) valueProperty(id accessibility.NodeID) {
	patterns := d.patterns(id)
	switch {
	case patterns.Has(PatternValue):
		d.property(id, UIA_ValueValuePropertyId)
	case patterns.Has(PatternRangeValue):
		d.property(id, UIA_RangeValueValuePropertyId)
	default:
	}
}

// state records the calls a state change asks for. A flag with no UI Automation property behind it, or one belonging to
// a pattern this node does not support, records nothing.
func (d *uiaDecider) state(event accessibility.Event) {
	n := d.cur.Node(event.Node)
	if n == nil {
		return
	}
	patterns := UIAPatterns(n)
	switch event.State {
	case accessibility.StateDisabled:
		d.property(n.ID, UIA_IsEnabledPropertyId)
	case accessibility.StateFocusable:
		d.property(n.ID, UIA_IsKeyboardFocusablePropertyId)
	case accessibility.StateOffscreen:
		d.property(n.ID, UIA_IsOffscreenPropertyId)
	case accessibility.StateInvalid:
		d.property(n.ID, UIA_IsDataValidForFormPropertyId)
	case accessibility.StateSelected:
		if patterns.Has(PatternSelectionItem) {
			d.selectionItem(n, n.Selected)
		}
	case accessibility.StatePressed, accessibility.StateChecked:
		switch {
		case patterns.Has(PatternToggle):
			d.property(n.ID, UIA_ToggleToggleStatePropertyId)
		case patterns.Has(PatternSelectionItem):
			// A radio button has no toggle state: being checked is being the selected one of its group.
			d.selectionItem(n, n.Checked == check.On)
		default:
		}
	case accessibility.StateReadOnly:
		switch {
		case patterns.Has(PatternValue):
			d.property(n.ID, UIA_ValueIsReadOnlyPropertyId)
		case patterns.Has(PatternRangeValue):
			d.property(n.ID, UIA_RangeValueIsReadOnlyPropertyId)
		default:
		}
	case accessibility.StateExpandable, accessibility.StateExpanded:
		if patterns.Has(PatternExpandCollapse) {
			d.property(n.ID, UIA_ExpandCollapseExpandCollapseStatePropertyId)
		}
	case accessibility.StateProtected:
		// A field that became, or stopped being, a password field changes what every client may read from it, and the
		// provider answers the property for every element, so it is reported for every element too.
		d.property(n.ID, UIA_IsPasswordPropertyId)
	case accessibility.StateModal:
		// Modality is the Window pattern's to report, and the fragment root is the only element that hands that pattern
		// out — a nested node with a window-like role is a panel rather than a window of its own, as
		// UIAProvider.supports explains. A client watches the property to know whether to keep the user inside this
		// window until it is dealt with.
		if n.ID == d.cur.Root && patterns.Has(PatternWindow) {
			d.property(n.ID, UIA_WindowIsModalPropertyId)
		}
	case accessibility.StateIgnored:
		// An ignored node has no provider, so there is no property to report and nothing to report it on. What has
		// happened is structural: the node joined or left the tree a client sees, and no node's list of children
		// changed to say so, since an ignored node stays in the snapshot for hit testing and coordinate clipping. The
		// nearest unignored parent is therefore told to read its children again, which is the only thing that brings a
		// client's cached hierarchy back in line with what UIANavigate answers. It reaches a real window often:
		// ScrollBar.ProvideAccessibility ignores a scroll bar with nothing to scroll, so one appears and disappears as
		// its content grows and shrinks. The Linux adapter reports the same flip the same way; see emitIgnoredChanged
		// in internal/atspi/events.go.
		d.add(UIARaise{
			Kind:   UIARaiseStructure,
			Node:   d.invalidationTarget(event),
			Change: StructureChangeType_ChildrenInvalidated,
		})
	default:
		// Selectable, Multiselectable and Busy have no property a client watches for.
	}
}

// selectionItem records the calls a change to a selection item's selected state asks for. Which event that is depends
// on the container: one that allows several selections at once reports each element joining and leaving the selection,
// while one that does not reports only the element that became the selection, since that implicitly deselects the
// previous one.
//
// The container is the one the provider reports through ISelectionItemProvider::get_SelectionContainer — the nearest
// ancestor supporting the Selection pattern — rather than the immediate parent, which in a hierarchical table is
// another Row. A row is never multiselectable, so asking the parent would report a child row joining a multiple
// selection with ElementSelected, which tells the client everything else was just deselected.
func (d *uiaDecider) selectionItem(n *accessibility.Node, selected bool) {
	d.property(n.ID, UIA_SelectionItemIsSelectedPropertyId)
	container := d.cur.Node(UIASelectionContainer(d.cur, n.ID))
	if container != nil && container.Multiselectable {
		if selected {
			d.event(n.ID, UIA_SelectionItem_ElementAddedToSelectionEventId)
			return
		}
		d.event(n.ID, UIA_SelectionItem_ElementRemovedFromSelectionEventId)
		return
	}
	if selected {
		d.event(n.ID, UIA_SelectionItem_ElementSelectedEventId)
	}
}

// added records the structure change a new node asks for, unless something at or above its parent has already been
// reported as having all of its children invalidated.
//
// The whole chain has to be looked at rather than only the parent. A subtree arrives as one addition of its top node
// plus one of every node under it, and the invalidation the diff produces names the surviving parent the subtree hangs
// off: testing only the immediate parent would drop the top node, whose parent is that one, and then report every
// descendant individually, whose parents are the new nodes themselves.
func (d *uiaDecider) added(id accessibility.NodeID) {
	n := d.cur.Node(id)
	if n == nil || n.Ignored {
		return
	}
	if d.invalidatedAt(d.cur, n.Parent) {
		return
	}
	d.add(UIARaise{
		Kind:   UIARaiseStructure,
		Node:   id,
		Child:  id,
		Change: StructureChangeType_ChildAdded,
	})
}

// removed records the structure change a departed node asks for, followed by the disconnect that releases its provider.
// The structure change is raised on the parent, since the node itself is gone, and is skipped when something at or
// above that parent has already been reported as having all of its children invalidated, or when the parent has left
// the tree too. The chain is walked over the previous tree, which is the only one that still holds the departed node.
func (d *uiaDecider) removed(id accessibility.NodeID) {
	if n := d.old.Node(id); n != nil && !n.Ignored {
		parent := d.target(d.old, n.Parent)
		if parent != 0 && !d.invalidatedAt(d.old, n.Parent) && d.cur.Node(parent) != nil {
			d.add(UIARaise{
				Kind:   UIARaiseStructure,
				Node:   parent,
				Child:  id,
				Change: StructureChangeType_ChildRemoved,
			})
		}
	}
	d.add(UIARaise{Kind: UIARaiseDisconnect, Node: id})
}

// invalidatedAt reports whether the node with the given id, or any ancestor of it within t, has already been reported
// as having all of its children invalidated. The walk is bounded so that a malformed tree — one whose Parent links form
// a cycle — cannot spin here forever.
func (d *uiaDecider) invalidatedAt(t *accessibility.Tree, id accessibility.NodeID) bool {
	for depth := 0; id != 0 && depth < uiaMaxTreeDepth; depth++ {
		if d.invalidated[d.target(t, id)] {
			return true
		}
		n := t.Node(id)
		if n == nil {
			return false
		}
		id = n.Parent
	}
	return false
}
