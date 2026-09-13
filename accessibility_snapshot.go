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
	"strconv"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// This file turns a window's live panel hierarchy into an immutable accessibility.Tree and publishes it, along with the
// events that describe how it differs from the tree published before it, to the platform adapter.
//
// The model is push rather than pull. Unison has no per-panel change notification — MarkForRedraw invalidates a whole
// window — so there is nothing to hang "the name of this control changed" on. Instead, a window that has just been
// redrawn is walked in full, and accessibility.Diff derives the events from the two snapshots. That also decouples the
// assistive technology's thread from the UI thread: an adapter answers every query from the snapshot it was handed and
// never touches a live panel.
//
// None of this runs unless accessibilityActive is set. See accessibility.go.
//
// Text controls fill in accessibility.Node.Text from their own ProvideAccessibility implementation, since only the
// widget knows what its content and selection are. The laid-out lines within that — accessibility.TextInfo.Lines — are
// measured only for the control that holds the focus, which is what AccessibilityBuilder.Focused is for: measuring every
// line of every text control in a window on every snapshot would cost far more than anything an assistive technology
// would do with the result.

// axPublishThrottle bounds how often a window's tree is rebuilt. A window that redraws continuously — a blinking caret
// is enough — would otherwise rebuild its tree on every frame, and an assistive technology gains nothing from being told
// about changes faster than a person can perceive them. Headless sessions bypass it, since a test that asks for a tree
// must be given the current one rather than one from up to this long ago.
const axPublishThrottle = 50 * time.Millisecond

// windowAccessibility is a window's accessibility state. It is nil until the window is first described, and is dropped
// again when accessibility support is deactivated or the window is destroyed, so nothing here exists for a window that
// no assistive technology has ever asked about.
type windowAccessibility struct {
	// last is the most recently published tree, which is what the next one is diffed against.
	last *accessibility.Tree
	// targets maps each node in the most recently built tree back to what it was built from, which is how a request
	// arriving from an assistive technology finds the panel to act on. It is rebuilt with every snapshot.
	targets map[accessibility.NodeID]axTarget
	// lastPublish is when the most recent publish happened, for the throttle.
	lastPublish time.Time
	// generation counts the trees built for this window and becomes accessibility.Tree.Generation.
	generation uint64
	// publishQueued reports that a publish was throttled and a task has been scheduled to perform it.
	publishQueued bool
}

// axTarget is what one node was built from: the panel that described it and, for a virtual child such as a table row,
// the key that panel identifies the child by. key is nil for a node that is a panel in its own right.
type axTarget struct {
	panel *Panel
	key   any
}

// axSnapshot is the state of one tree being built. It lives only for the duration of buildAccessibilityTree.
type axSnapshot struct {
	window     *Window
	tree       *accessibility.Tree
	targets    map[accessibility.NodeID]axTarget
	focusPanel *Panel
	// cell is set while the panels inside one cell of a table are being described; see axCellContext.
	cell       *axCellContext
	focus      accessibility.NodeID
	generation uint64
}

// axCellContext is in force while the panel a table row handed back for one of its cells, and everything inside it,
// is being described. Such panels are not part of the window: a row may build them afresh every time it is asked, and
// the table attaches one only for as long as it takes to draw it or hand it an event. So the nodes describing them
// cannot be keyed by the panels themselves, which may not exist a moment later, nor can requests be sent to those
// panels. Instead each node is keyed by the cell and the position of its panel within the cell — the table's builder
// allocates the ids, so they hold from one description to the next for as long as the row builds the same thing — and
// requests are sent to the table, which builds the cell again, attaches it, and finds the panel at that position.
type axCellContext struct {
	builder *AccessibilityBuilder
	key     accessibility.CellKey
	path    []int
}

// axCellPanelKey is the key under which a panel inside a table cell is registered: the cell, and the panel's position
// within it as the child indexes on the way down from the cell's own panel, joined with dots.
type axCellPanelKey struct {
	Path string
	Cell accessibility.CellKey
}

// pathString returns the current position within the cell in the form axCellPanelKey.Path uses.
func (c *axCellContext) pathString() string {
	var buffer strings.Builder
	for i, index := range c.path {
		if i != 0 {
			buffer.WriteByte('.')
		}
		buffer.WriteString(strconv.Itoa(index))
	}
	return buffer.String()
}

// identify returns the id a panel is described under and where requests about it are sent: the panel's own id and the
// panel itself, or — inside a table cell — an id the table's builder allocates for the panel's position within the cell,
// with requests sent to the table.
func (s *axSnapshot) identify(p *Panel) (accessibility.NodeID, axTarget) {
	if s.cell == nil {
		return axIDFor(p), axTarget{panel: p}
	}
	key := axCellPanelKey{Cell: s.cell.key, Path: s.cell.pathString()}
	return s.cell.builder.virtualID(key), axTarget{panel: s.cell.builder.panel, key: key}
}

// buildAccessibilityTree captures the window's hierarchy as an immutable tree. The window's node registry is replaced
// with the one this tree was built with, so a request arriving afterwards is resolved against what the assistive
// technology was last told about.
func (w *Window) buildAccessibilityTree() *accessibility.Tree {
	axSnapshotCount++
	if w.ax == nil {
		w.ax = &windowAccessibility{}
	}
	w.ax.generation++
	// The previous tree's size is the best guess at this one's, since a window rarely changes shape wholesale between
	// snapshots. The margin covers the ordinary case of something having just been added.
	capacity := 16
	if w.ax.last != nil {
		capacity = len(w.ax.last.Nodes) + 16
	}
	s := &axSnapshot{
		window: w,
		tree: &accessibility.Tree{
			Nodes:      make(map[accessibility.NodeID]*accessibility.Node, capacity),
			Generation: w.ax.generation,
		},
		targets:    make(map[accessibility.NodeID]axTarget, capacity),
		focusPanel: w.CurrentFocus(),
		generation: w.ax.generation,
	}
	s.buildRoot()
	s.tree.Focus = s.focus
	w.ax.targets = s.targets
	return s.tree
}

// buildRoot describes the window itself and then its children. The root's children are taken from the root panel's own
// fields rather than from its child list, so the order an assistive technology reads them in is fixed by what they are
// — the menus that are open, then the menu bar, then the tooltip, then the content — rather than by the order in which
// they happened to be inserted. The open menus are visited newest first, which is both their z-order and the order the
// root panel actually holds them in.
func (s *axSnapshot) buildRoot() {
	w := s.window
	root := w.root
	node := &accessibility.Node{
		ID:      axIDFor(root.AsPanel()),
		Name:    w.Title(),
		Bounds:  w.LocalContentRect(),
		Role:    role.Window,
		Focused: w.Focused(),
		Modal:   len(modalStack) != 0 && modalStack[len(modalStack)-1] == w,
	}
	if w.kind == WindowKindDialog || w.data[DialogClientDataKey] != nil {
		node.Role = role.Dialog
	}
	s.tree.Root = node.ID
	s.tree.Nodes[node.ID] = node
	s.targets[node.ID] = axTarget{panel: root.AsPanel()}
	clip := node.Bounds
	for i := len(root.openMenuPanels) - 1; i >= 0; i-- {
		s.visit(root.openMenuPanels[i].AsPanel(), node.ID, clip)
	}
	if root.menuBarPanel != nil {
		s.visit(root.menuBarPanel.AsPanel(), node.ID, clip)
	}
	s.visit(root.tooltipPanel, node.ID, clip)
	s.visit(root.contentPanel, node.ID, clip)
	if id := s.openMenuFocus(); id != 0 {
		s.focus = id
	}
}

// openMenuFocus returns the node id of the item an open in-window menu is pointing at, or zero if no menu is open or
// none of the open menus is pointing at anything. While a menu is open the keyboard focus stays wherever it was, since
// the menu handles keys ahead of it, but what a person is choosing from is the item the menu has highlighted, so that
// is what an assistive technology must be told the focus is. The stack is searched from the newest menu down, since a
// sub-menu that has just opened has nothing highlighted yet while the item that opened it still does.
func (s *axSnapshot) openMenuFocus() accessibility.NodeID {
	panels := s.window.root.openMenuPanels
	for i := len(panels) - 1; i >= 0; i-- {
		if panels[i].menu == nil {
			continue
		}
		for _, item := range panels[i].menu.items {
			if !item.over || item.panel == nil {
				continue
			}
			if id := item.panel.Accessibility.id; id != 0 && s.tree.Nodes[id] != nil {
				return id
			}
		}
	}
	return 0
}

// visit describes p and everything beneath it, adding the result as a child of parent. clip is the region, in
// window-local coordinates, that p's ancestors leave visible.
func (s *axSnapshot) visit(p *Panel, parent accessibility.NodeID, clip geom.Rect) {
	if p == nil || p.Hidden {
		return
	}
	raw := p.RectToRoot(p.ContentRect(true))
	// A panel clips its children to its frame when it draws them, so what its ancestors leave visible of it is the
	// region its own children are in turn confined to. Its bounds are reported whole, though: an assistive technology
	// needs to know where the panel is and how big it is even when part of it has been scrolled out of sight, since
	// that is how it knows how far to scroll to reveal the rest; Offscreen says when none of it can be seen.
	visible := raw.Intersect(clip)
	if p.Accessibility.Role == role.None {
		// The panel is not exposed at all, so its children take its place as children of its own parent. It still clips
		// them, since it is still a real panel drawn on the screen.
		s.visitChildren(p, parent, visible)
		return
	}
	id, target := s.identify(p)
	node := &accessibility.Node{
		ID:          id,
		Parent:      parent,
		Name:        p.Accessibility.Name,
		Description: p.Accessibility.Description,
		Bounds:      raw,
		Role:        p.Accessibility.Role,
		Disabled:    !p.Enabled(),
		Focusable:   p.Focusable(),
		Focused:     s.focusPanel != nil && p.Is(s.focusPanel),
		Offscreen:   visible.Empty() && !raw.Empty(),
	}
	node.Actions = node.Actions.With(accessibility.ScrollIntoView)
	if node.Focusable {
		node.Actions = node.Actions.With(accessibility.Focus)
	}
	if p.MouseDownCallback != nil && p.MouseUpCallback != nil {
		node.Actions = node.Actions.With(accessibility.Press)
	}
	// Registered before the widget is consulted, so that virtual children it adds can find their parent and so that a
	// panic partway through still leaves a usable node behind.
	s.tree.Nodes[node.ID] = node
	s.targets[node.ID] = target
	if parentNode := s.tree.Nodes[parent]; parentNode != nil {
		parentNode.Children = append(parentNode.Children, node.ID)
	}
	if provider, ok := p.Self.(AccessibilityProvider); ok {
		b := &AccessibilityBuilder{
			snapshot: s,
			node:     node,
			panel:    p,
			clip:     visible,
		}
		SafeCall(func() { provider.ProvideAccessibility(b) })
		s.sweepVirtualIDs(p, b.virtualUsed)
	}
	if node.Role == role.Auto {
		node.Role = role.Group
	}
	s.resolveName(p, node)
	if node.Description == "" {
		// A widget with nothing but a tooltip to go on — an icon button, say — takes its name from that tooltip, and
		// repeating it as the description would have an assistive technology say the same thing twice.
		if tip := axTooltipText(p); tip != node.Name {
			node.Description = tip
		}
	}
	if node.Role == role.Group && node.Name == "" && node.Description == "" {
		// A grouping panel with nothing to say about itself is scaffolding rather than content. It stays in the tree so
		// that hit testing and coordinate clipping still work, but an assistive technology is told to look past it.
		node.Ignored = true
	}
	if p.Accessibility.Callback != nil {
		SafeCall(func() { p.Accessibility.Callback(node) })
	}
	if node.Focused && s.focus == 0 {
		s.focus = node.ID
	}
	if node.Role == role.Heading || node.Role == role.Label {
		// Static text is one element, however many panels it is actually built from, so its children have already been
		// folded into its name and are not visited.
		return
	}
	s.visitChildren(p, node.ID, visible)
}

// visitChildren describes each of p's children as children of parent, in index order — which is reading order, and also
// front-to-back order, since index 0 is drawn last and so sits on top.
func (s *axSnapshot) visitChildren(p *Panel, parent accessibility.NodeID, clip geom.Rect) {
	for i, child := range p.Children() {
		if s.cell != nil {
			s.cell.path = append(s.cell.path, i)
		}
		s.visit(child, parent, clip)
		if s.cell != nil {
			s.cell.path = s.cell.path[:len(s.cell.path)-1]
		}
	}
}

// resolveName fills in the node's name and label associations, for a node whose widget did not name it outright. The
// order is: an explicit Accessibility.LabeledBy panel, then the sibling-label convention, then, for static text, the
// text of the panel and its descendants.
func (s *axSnapshot) resolveName(p *Panel, node *accessibility.Node) {
	if !xreflect.IsNil(p.Accessibility.LabeledBy) {
		if labeler := p.Accessibility.LabeledBy.AsPanel(); labeler != nil {
			node.LabeledBy = append(node.LabeledBy, axIDFor(labeler))
			if node.Name == "" {
				node.Name = axLabelText(labeler)
			}
		}
	}
	if node.Name == "" && axRoleUsesSiblingLabel(node.Role) {
		if labeler := axPrecedingLabel(p); labeler != nil {
			node.Name = axTrimLabelText(labeler.String())
			node.LabeledBy = append(node.LabeledBy, axIDFor(labeler.AsPanel()))
		}
	}
	if node.Name == "" && (node.Role == role.Heading || node.Role == role.Label) {
		node.Name = axLabelText(p)
	}
}

// sweepVirtualIDs discards the virtual-child ids a panel is no longer using, once it is holding appreciably more of them
// than it needs. The threshold leaves room for a control whose set of children shifts a little from snapshot to snapshot
// — a table scrolling by a row — to keep reusing its ids, while a table whose rows are replaced wholesale, over and over,
// cannot accumulate them without bound.
func (s *axSnapshot) sweepVirtualIDs(p *Panel, used int) {
	entries := p.Accessibility.virtual
	if len(entries) <= 2*used+16 {
		return
	}
	for key, entry := range entries {
		if entry.used != s.generation {
			delete(entries, key)
		}
	}
}

// axRoleUsesSiblingLabel reports whether the sibling-label convention applies to a role. It is restricted to the roles
// that present a value the user is expected to read or change, since those are the ones a label is placed beside. A
// button or a check box carries its own text, and applying the convention to a container would attach the label of
// whatever came before it to everything inside.
func axRoleUsesSiblingLabel(r role.Enum) bool {
	switch r {
	case role.TextField, role.TextArea, role.SpinButton, role.ComboBox, role.PopupButton, role.Slider, role.ProgressBar,
		role.ColorWell, role.List, role.Table, role.Tree:
		return true
	default:
		return false
	}
}

// axPrecedingLabel returns the Label immediately before p among its parent's children, or nil if there is not one. This
// is the "label, then the thing it labels" convention that laying controls out in a two-column grid produces, and it is
// used only as a last resort: a widget that knows its own label sets Accessibility.LabeledBy and never depends on it.
func axPrecedingLabel(p *Panel) *Label {
	parent := p.Parent()
	if parent == nil {
		return nil
	}
	i := parent.IndexOfChild(p)
	if i <= 0 {
		return nil
	}
	previous := parent.Children()[i-1]
	if previous.Hidden || previous.Accessibility.Role == role.None {
		return nil
	}
	label, ok := previous.Self.(*Label)
	if !ok || label.String() == "" {
		return nil
	}
	return label
}

// axLabelText returns the text of p and its descendants, joined with spaces, for a panel whose whole content is static
// text. A descendant that names itself contributes that name and is not descended into; otherwise a Label contributes
// its text. Hidden panels contribute nothing.
func axLabelText(p *Panel) string {
	var buffer strings.Builder
	axAppendLabelText(&buffer, p)
	return buffer.String()
}

// axAppendLabelText appends the text of p and its descendants to buffer.
func axAppendLabelText(buffer *strings.Builder, p *Panel) {
	if p == nil || p.Hidden {
		return
	}
	text := p.Accessibility.Name
	if text == "" {
		if label, ok := p.Self.(*Label); ok {
			text = label.String()
		}
	}
	if text != "" {
		if buffer.Len() != 0 {
			buffer.WriteByte(' ')
		}
		buffer.WriteString(text)
		return
	}
	for _, child := range p.Children() {
		axAppendLabelText(buffer, child)
	}
}

// axTrimLabelText turns the text of a label into a name by dropping the surrounding whitespace and the trailing colon
// that a label placed beside a control conventionally ends with.
func axTrimLabelText(text string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), ":"))
}

// axTooltipText returns the text of a panel's tooltip, which is what a node's description falls back to. A tooltip built
// by NewTooltipWithText names itself; anything else is read from the labels it is built out of.
func axTooltipText(p *Panel) string {
	tip := p.Tooltip
	if tip == nil {
		return ""
	}
	if tip.Accessibility.Name != "" {
		return tip.Accessibility.Name
	}
	return axLabelText(tip)
}

// publishAccessibility describes the window and hands the result, with the events that say how it differs from the
// previously published description, to the platform adapter. It is called after the window has been drawn, and does
// nothing unless an assistive technology is being served.
//
// Publishing is throttled: a window that redraws continuously is described at most once per axPublishThrottle, with a
// task scheduled to catch the final state of a burst. A headless session is not throttled, since a test asking for a
// tree must be given the state of the window as it is now. Neither is the first publish of a window, whatever the
// session, so a platform adapter that activates on its first query and then publishes has something to answer with
// before it returns.
func (w *Window) publishAccessibility() {
	// The root panel is what a description is built from, so a window that has none — a bare Window a test assembled
	// itself — has nothing to describe.
	if !accessibilityActive.Load() || !w.IsValid() || w.root == nil {
		return
	}
	if w.ax == nil {
		w.ax = &windowAccessibility{}
	}
	if activeHeadless() == nil {
		if elapsed := time.Since(w.ax.lastPublish); elapsed < axPublishThrottle {
			if !w.ax.publishQueued {
				w.ax.publishQueued = true
				InvokeTaskAfter(w.publishThrottledAccessibility, axPublishThrottle-elapsed)
			}
			return
		}
	}
	w.publishAccessibilityNow()
}

// publishThrottledAccessibility performs the publish that publishAccessibility deferred.
func (w *Window) publishThrottledAccessibility() {
	if w.ax == nil {
		return
	}
	w.ax.publishQueued = false
	if accessibilityActive.Load() && w.IsValid() && w.root != nil {
		w.publishAccessibilityNow()
	}
}

// publishAccessibilityNow describes the window and publishes it, without regard to the throttle.
func (w *Window) publishAccessibilityNow() {
	w.ax.lastPublish = time.Now()
	tree := w.buildAccessibilityTree()
	events := accessibility.Diff(w.ax.last, tree)
	w.ax.last = tree
	w.apiAccessibilityPublish(tree, events)
}

// accessibilityGeometry returns where the window's content area sits on the screen, in physical pixels, along with the
// backing scale that node bounds — which are window-local and in logical units — must be multiplied by to reach that
// space. The platforms whose accessibility APIs work in physical screen pixels convert with these two values.
func (w *Window) accessibilityGeometry() (origin, scale geom.Point) {
	scale = w.BackingScale()
	return w.ContentRect().Point.MulPt(scale), scale
}
