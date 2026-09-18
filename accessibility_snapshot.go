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
	"unicode"
	"unicode/utf8"

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
// measured by an editable control only while it holds the focus, which is what AccessibilityBuilder.Focused is for:
// measuring every line of every text control in a window on every snapshot would cost far more than anything an
// assistive technology would do with the result. The blocks of a document are the exception, and fill them in whether
// the document holds the focus or not — laying the text out is what drew them, so the advances have already been
// measured and there is nothing left to spend. See accessibility.TextInfo.Lines.

// axPublishThrottle bounds how often a window's tree is rebuilt. A window that redraws continuously — a blinking caret
// is enough — would otherwise rebuild its tree on every frame, and an assistive technology gains nothing from being
// told about changes faster than a person can perceive them. Headless sessions bypass it, since a test that asks for a
// tree must be given the current one rather than one from up to this long ago.
const axPublishThrottle = 50 * time.Millisecond

// axThrottleHeadless makes a headless session honor the publish throttle, which it otherwise bypasses. Nothing but the
// internal tests sets it: the throttled path — the queued re-publish, and what becomes of it when the window or the
// support behind it goes away while it is in flight — has nothing else that could drive it, since every test runs
// headless. UI thread only.
var axThrottleHeadless bool

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
	// generation is the number of the most recent tree built for this window, which is a copy of the window's own
	// Window.axGeneration: this state is dropped when the window is withdrawn and the count is not.
	generation uint64
	// publishQueued reports that a publish was throttled and a task has been scheduled to perform it.
	publishQueued bool
	// adapterFailed reports that the platform's assistive-technology adapter could not be created for this window, so
	// that neither building one nor describing the window is attempted again: there is nothing to publish to, and the
	// reason there is not — on macOS, the accessibility element class failing to register — is a property of the
	// process rather than of this moment, so every later attempt would fail in the same way. A window being described
	// runs through the publish path as often as every axPublishThrottle, which would turn one failure into a steady
	// stream of them. The flag goes away with the rest of this state when the window is taken out of the description,
	// so a fresh activation tries once more.
	adapterFailed bool
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
	cell *axCellContext
	// childrenDescribed holds the nodes whose children the widget itself described, through
	// AccessibilityBuilder.DescribeChildren, which is what keeps visit from describing them a second time. It stays nil
	// until something asks, and the only widgets that do are the pieces of static text holding something an assistive
	// technology has to reach in its own right — a markdown heading with a link in it.
	childrenDescribed map[accessibility.NodeID]bool
	// companionFocus holds the nodes that reported the focus while another node had already claimed it. A document's
	// block does that for the reading caret it holds, and only on the platforms whose screen readers need it; see
	// axCaretBlockReportsFocus and axSnapshot.resolveCompanionFocus, which is what decides whether each of them keeps
	// the claim. It stays nil in every window where nothing claims the focus twice, which is nearly all of them.
	companionFocus []accessibility.NodeID
	focus          accessibility.NodeID
	generation     uint64
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
// panel itself, or — inside a table cell — an id the table's builder allocates for the panel's position within the
// cell, with requests sent to the table.
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
//
// Returns nil, having installed nothing, if the application code the build ran withdrew the window's accessibility
// state partway through. Every ProvideAccessibility and Accessibility.Callback runs from inside the build, and either
// is free to call SetAccessibilityEnabled(false) or Window.Dispose, both of which free that state — see
// deactivateAccessibility and Window.destroy. The registry a build like that would install belongs to a state that is
// gone, and there is nothing left to publish its tree to, so the tree is discarded rather than installed through a
// pointer the withdrawal has nilled. The callbacks are run under SafeCall, but that guards only what the callbacks do,
// not what the build does with the window afterwards.
func (w *Window) buildAccessibilityTree() *accessibility.Tree {
	axSnapshotCount++
	if w.ax == nil {
		w.ax = &windowAccessibility{}
	}
	// Held so that the state this build began with can be told from whatever the window holds once the application
	// code the build runs has returned. Compared by identity rather than against nil, since that code could withdraw
	// the state and then, by turning support back on and describing the window from a callback, have a fresh one
	// installed: a registry built for the old one is no more valid in the new one than in none at all.
	ax := w.ax
	// Counted on the window rather than on the state above, which a withdrawal drops. See Window.axGeneration.
	w.axGeneration++
	ax.generation = w.axGeneration
	// The previous tree's size is the best guess at this one's, since a window rarely changes shape wholesale between
	// snapshots. The margin covers the ordinary case of something having just been added.
	capacity := 16
	if ax.last != nil {
		capacity = len(ax.last.Nodes) + 16
	}
	s := &axSnapshot{
		window: w,
		tree: &accessibility.Tree{
			Nodes:      make(map[accessibility.NodeID]*accessibility.Node, capacity),
			Generation: ax.generation,
		},
		targets:    make(map[accessibility.NodeID]axTarget, capacity),
		focusPanel: w.CurrentFocus(),
		generation: ax.generation,
	}
	s.buildRoot()
	if w.ax != ax {
		return nil
	}
	s.tree.Focus = s.focus
	ax.targets = s.targets
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
	node.Resizable = w.Resizable()
	node.Floating = w.floating
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
	s.markOpenMenuItems()
	s.resolveFocus()
	s.resolveCompanionFocus()
}

// markOpenMenuItems publishes every item of every open menu as something the focus can be moved onto, which for a menu
// item means being what the menu is pointing at.
//
// It is not enough to say so of the item that is already highlighted. All three adapters refuse to pass a focus request
// on to a node that does not offer the focus action — see internal/atspi.Component, w32AccessibilityProvider and the
// Cocoa element's setAccessibilityFocused: — so marking only the highlighted item would advertise the action in the one
// place carrying it out changes nothing and withhold it everywhere it would do something: an assistive technology could
// re-focus the item the person is already on and never move the highlight, which is what moving through a menu with
// anything other than the arrow keys amounts to. See menuItem.performAccessibilityAction, which does the moving.
//
// Nothing is claimed while no menu is open. An item of a closed menu has no panel on the screen at all, and a title on
// the menu bar with nothing open below it is not something a person is choosing from — the pointer merely passing over
// one highlights it — so the bar is included only once something has been opened from it, which is exactly when
// axSnapshot.openMenuFocus will report the focus there. A disabled item is left alone for the reason
// axSnapshot.highlightedMenuItem refuses to report the focus on one: every request that would act on it is refused, and
// the narrowing in axSnapshot.visit has already stripped it back to axDisabledActions.
func (s *axSnapshot) markOpenMenuItems() {
	root := s.window.root
	if len(root.openMenuPanels) == 0 {
		return
	}
	for _, p := range root.openMenuPanels {
		s.markMenuItemsFocusable(p)
	}
	s.markMenuItemsFocusable(root.menuBarPanel)
}

// markMenuItemsFocusable marks each item of one menu panel as something the focus can be moved onto. See
// markOpenMenuItems.
func (s *axSnapshot) markMenuItemsFocusable(panel *menuPanel) {
	if panel == nil || panel.menu == nil {
		return
	}
	for _, item := range panel.menu.items {
		if node := s.menuItemNode(item); node != nil {
			node.Focusable = true
			node.Actions = node.Actions.With(accessibility.Focus)
		}
	}
}

// resolveFocus decides which node, if any, the window reports the keyboard focus on. A node the focus panel was
// described as has already claimed it during the walk; what is left to decide is the three cases where it did not — an
// open menu with something highlighted and an open menu with nothing highlighted, both of which take the focus away
// from wherever it actually is, and a focus panel that is not in the tree or is in it disabled, which has to fall back
// to something that is.
func (s *axSnapshot) resolveFocus() {
	if id := s.openMenuFocus(); id != 0 {
		s.focus = id
		if item := s.tree.Nodes[id]; item != nil {
			// Said here rather than by the item itself, so that exactly one node in the window reports being focused.
			// Being focusable, and offering the action that moves the focus, is said of every item of every open menu
			// rather than only of this one; see axSnapshot.markOpenMenuItems, which has already run and which cannot
			// have passed this item over, since an item it refuses is one openMenuFocus refuses too.
			item.Focused = true
		}
		s.clearDisplacedFocus(id)
		return
	}
	if id := s.openMenuNode(); id != 0 {
		s.focus = id
		if m := s.tree.Nodes[id]; m != nil {
			m.Focused = true
			// Scaffolding is what a tree says about a node nothing needs to reach, and the node the focus is reported
			// on is by definition one an assistive technology must be able to reach. See axSnapshot.fallbackFocus,
			// which clears it for the same reason.
			m.Ignored = false
		}
		s.clearDisplacedFocus(id)
		return
	}
	if s.focus == 0 {
		s.fallbackFocus()
	}
}

// resolveCompanionFocus decides what becomes of a second node reporting the focus while another node already holds it,
// which is what a document's block does while it holds the reading caret: the document is the focus, and the block says
// it is focused too so that a screen reader following the caret is told where the caret went. It runs once
// axSnapshot.resolveFocus has settled where the focus actually is.
//
// The claim stands only where the platform's screen reader needs it and only for a node inside the panel that really
// holds the keyboard focus, and only while that panel's own node is what the window reports the focus on. AT-SPI
// carries the state per object, so Orca finding the block focused is how it presents an application-driven caret move
// at all — see axCaretBlockReportsFocus — while UI Automation and AppKit have one focused element apiece and would
// follow the wrong one. A claim from anywhere else in the window is a mistake, since the node an assistive technology
// is told to look at is not the node the person is actually in.
//
// This runs after the open-menu cases have had their say, and a window whose focus a menu has taken over loses the
// companion claim with it: axSnapshot.clearDisplacedFocus takes Focused away from the panel that holds the keyboard
// focus so that exactly one node in the window reports it — the item the person is choosing from — and a caret block
// left claiming the focus inside a document the menu displaced would be the second focused object that clearing exists
// to prevent, which on AT-SPI is what Orca would announce instead of the item. Nothing is lost by dropping it: the
// caret has not moved while the menu is up, so the claim comes back with the focus when the menu closes. A focus that
// fell back to an ancestor of the focus panel — because the panel is disabled or was never described — clears it for
// the same reason.
//
// Tree.Focus is never moved here. It names the node the focus is reported on, which is the document rather than the
// block inside it, and the first claim is what set it.
func (s *axSnapshot) resolveCompanionFocus() {
	if len(s.companionFocus) == 0 {
		return
	}
	var within accessibility.NodeID
	if s.focusPanel != nil && s.focusPanel.Accessibility.owner == s.focusPanel {
		within = s.focusPanel.Accessibility.id
	}
	// The focus panel's own node is what a companion claim sits beneath, so a window reporting the focus anywhere else
	// has nothing for one to be a companion to.
	reported := within != 0 && s.focus == within
	for _, id := range s.companionFocus {
		node := s.tree.Nodes[id]
		if node == nil || node.ID == s.focus {
			continue
		}
		if !axCaretBlockReportsFocus || !reported || !s.isWithin(node, within) {
			node.Focused = false
		}
	}
}

// isWithin reports whether a node is a descendant of the given one. Zero is nothing's ancestor, which is the answer for
// a focus panel that was never described.
func (s *axSnapshot) isWithin(node *accessibility.Node, ancestor accessibility.NodeID) bool {
	if ancestor == 0 {
		return false
	}
	for n := node; n != nil && n.Parent != 0; n = s.tree.Nodes[n.Parent] {
		if n.Parent == ancestor {
			return true
		}
	}
	return false
}

// openMenuNode returns the node id of the newest in-window menu that is open, or zero if none is open or none of them
// is in the tree. It is what the focus is reported on while a menu is open with nothing in it highlighted.
//
// That state is ordinary and is reached with the mouse alone: an item is highlighted by the pointer passing over it and
// menuItem.mouseExit clears the highlight when the pointer leaves, with nothing restoring a last-highlighted item, so
// moving the pointer off an open menu without closing it leaves every open menu and the bar with nothing highlighted.
// The keys still go to the newest open menu — rootPanel.preKeyDown, preRuneTyped and preKeyUp all hand them there ahead
// of the panel that holds the keyboard focus — so the menu is where the person is. Leaving the focus on the control
// underneath instead would have a screen reader announce, say, the text field the person was in before the menu opened,
// and offer the arrow and Return keys there while those keys actually drove the menu; every pass of the pointer across
// the menu would flip the reported focus between the item and that control.
//
// Focusable is not invented for the menu, and it is not given the focus action: a menu panel cannot take the keyboard
// focus — Window.SetFocus would not leave it there and Panel.axDispatchAction refuses a focus request on a panel that
// is not focusable — so advertising either would offer an assistive technology something it would then be refused. That
// is the answer axSnapshot.fallbackFocus gives for the same reason, and the arrow keys are how a person moves from here
// onto an item in any case: the first of them highlights one, and the highlighted item is what the focus is reported on
// from then on.
func (s *axSnapshot) openMenuNode() accessibility.NodeID {
	root := s.window.root
	for i := len(root.openMenuPanels) - 1; i >= 0; i-- {
		p := root.openMenuPanels[i].AsPanel()
		// A menu that hid itself with role.None, or that was hidden while open, was never described, so there is no
		// node to report the focus on and the menu below it in the stack is the newest one there is.
		if p.Accessibility.id == 0 || p.Accessibility.owner != p {
			continue
		}
		if node := s.tree.Nodes[p.Accessibility.id]; node != nil && !node.Disabled {
			return node.ID
		}
	}
	return 0
}

// fallbackFocus reports the focus on the nearest ancestor of the focus panel that an assistive technology can be shown,
// for a window whose focus panel could not claim it for itself.
//
// There are several ways that happens, and they amount to the same thing. The panel may not be in the tree at all: it
// hides itself with role.None, it is inside a Heading or a Label, whose children are folded into the name and never
// visited, or it was hidden while holding the focus. Or it may be in the tree but disabled, which is published without
// the focus. Either way something in the window really does hold the keyboard focus, and a tree that says the focus is
// nowhere makes an assistive technology stop following the window: it has been told the person is not anywhere.
//
// The root is not an answer. It reports whether the window is active, which is a different question, and naming it
// would make the focus appear to jump out to the window itself. A disabled ancestor is not one either, for the reason
// the disabled focus panel was refused. Focusable is not invented for the node that is chosen — a person cannot tab to
// it, and an assistive technology that offered to move the focus there would be refused — but Ignored is cleared:
// scaffolding is what a tree says about a node nothing needs to reach, and the node the focus is reported on is by
// definition one an assistive technology must be able to reach.
func (s *axSnapshot) fallbackFocus() {
	if s.focusPanel == nil {
		return
	}
	for p := s.focusPanel.Parent(); p != nil; p = p.Parent() {
		if p.Accessibility.id == 0 || p.Accessibility.owner != p {
			continue
		}
		node := s.tree.Nodes[p.Accessibility.id]
		if node == nil || node.ID == s.tree.Root || node.Disabled {
			continue
		}
		node.Focused = true
		node.Ignored = false
		s.focus = node.ID
		return
	}
}

// clearDisplacedFocus clears Focused on the panel that still holds the keyboard focus when an open menu has taken the
// effective focus away from it, leaving keep as the only node in the window that reports being focused.
//
// A tree that says two of its nodes are focused is a tree an assistive technology cannot make sense of: AT-SPI maps
// Node.Focused straight onto ATSPI_STATE_FOCUSED, so Orca would find two focused objects in one window and announce
// whichever it came across, and only one of them is what the person is actually choosing from. The keyboard focus does
// stay where it was while a menu is open — the menu simply handles keys ahead of it — but that is an implementation
// detail of how menus work, not something to describe.
//
// Only the panel that holds the keyboard focus is cleared. A node inside it that reports the focus for a reading caret
// it holds is not something a menu displaced, and what becomes of that claim is decided by
// axSnapshot.resolveCompanionFocus.
func (s *axSnapshot) clearDisplacedFocus(keep accessibility.NodeID) {
	if s.focusPanel == nil {
		return
	}
	if node := s.tree.Nodes[s.focusPanel.Accessibility.id]; node != nil && node.ID != keep {
		node.Focused = false
	}
}

// openMenuFocus returns the node id of the item the in-window menus are pointing at, or zero if no menu is open or
// nothing is being pointed at. While a menu is open the keyboard focus stays wherever it was, since the menu handles
// keys ahead of it, but what a person is choosing from is the item that is highlighted, so that is what an assistive
// technology must be told the focus is. The open menus are searched from the newest down, since a sub-menu that has
// just opened has nothing highlighted yet while the item that opened it still does, and the menu bar is searched last,
// which is where the highlight is in the moment between a title being clicked and something in the menu it opened being
// pointed at.
//
// Nothing counts while no menu is open. An item is highlighted by the pointer merely passing over it, and on the menu
// bar that happens whenever the pointer crosses a title: reporting it as the focus would name an item the person has
// not chosen anything from, alongside the control that actually holds the keyboard focus, on every such pass.
func (s *axSnapshot) openMenuFocus() accessibility.NodeID {
	root := s.window.root
	if len(root.openMenuPanels) == 0 {
		return 0
	}
	for i := len(root.openMenuPanels) - 1; i >= 0; i-- {
		if id := s.highlightedMenuItem(root.openMenuPanels[i]); id != 0 {
			return id
		}
	}
	return s.highlightedMenuItem(root.menuBarPanel)
}

// highlightedMenuItem returns the node id of the item a menu panel has highlighted, or zero when it has none, holds no
// menu, or the item it has highlighted is not one the focus can be reported on.
//
// A disabled item is not one. An item is highlighted by the pointer merely passing over it, whether or not it can be
// chosen — the item's panel is never disabled, so nothing stops the highlight; it is the node that is published as
// disabled — so pointing the focus at one would publish a node that says it holds the focus while saying it cannot
// take it, which is the pair every other path here goes out of its way not to produce. See axSnapshot.visit and
// axSnapshot.fallbackFocus, which both refuse a disabled node for the same reason. The search carries on to the menu
// that item was opened from, exactly as it does when nothing is highlighted at all.
//
// Neither is a separator. It is highlighted like anything else the pointer crosses — menuItem.newPanel installs the
// same enter and exit callbacks on one, since the highlight has to be taken off whatever the pointer left — but a
// separator is the line drawn between the things that can be chosen rather than one of them, and it is published with
// role.Separator. Reporting the focus on it would tell a screen reader the person had moved onto a divider, and would
// say a divider is focusable besides, every time the pointer merely passed over one.
func (s *axSnapshot) highlightedMenuItem(panel *menuPanel) accessibility.NodeID {
	if panel == nil || panel.menu == nil {
		return 0
	}
	for _, item := range panel.menu.items {
		if !item.over {
			continue
		}
		if node := s.menuItemNode(item); node != nil {
			return node.ID
		}
	}
	return 0
}

// menuItemNode returns the node describing a menu item, or nil when the item is not one the focus may be reported on or
// moved to: a separator, one that was never described — it holds no panel yet, or the panel it holds hides itself — or
// one that was published as disabled. See highlightedMenuItem and markOpenMenuItems, which both ask.
func (s *axSnapshot) menuItemNode(item *menuItem) *accessibility.Node {
	if item.isSeparator || item.panel == nil {
		return nil
	}
	id := item.panel.Accessibility.id
	if id == 0 {
		return nil
	}
	if node := s.tree.Nodes[id]; node != nil && !node.Disabled {
		return node
	}
	return nil
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
		URL:         p.Accessibility.URL,
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
		// Resolved before the name is, since which conventions apply — the sibling label, the text of a piece of static
		// text — is decided by the role. Whatever the callback leaves behind is resolved again after it runs.
		node.Role = role.Group
	}
	// Held here because this is the role the name is about to be resolved from, and the two halves of what being static
	// text means — the children's text folded into the name, the children themselves not described — have to be decided
	// by the same role. The callback runs after the name has been resolved, so a callback that moves the node off
	// Heading or Label has not unfolded the name it already has, and one that moves it onto either has not folded
	// anything into it. See the end of this function.
	staticText := node.Role == role.Heading || node.Role == role.Label
	s.resolveName(p, node)
	if node.Description == "" {
		// A widget with nothing but a tooltip to go on — an icon button, say — takes its name from that tooltip, and
		// repeating it as the description would have an assistive technology say the same thing twice.
		if tip := axTooltipText(p); tip != node.Name {
			node.Description = tip
		}
	}
	// Whether the node is scaffolding is decided after the callback has run, since a callback that gives an otherwise
	// anonymous group the name it was missing — which is one of the things callbacks are for — would otherwise leave it
	// marked for every assistive technology to skip.
	//
	// Both the widget and the callback outrank the heuristic. The node reaches the callback carrying whatever the
	// widget asked for, so a callback that changes the flag has said something the heuristic must not undo, while one
	// that leaves it alone has said nothing about it at all. Setting it to the value the widget had already chosen is
	// the one case the two cannot be told apart, and reading that as agreement rather than as an instruction costs
	// nothing: the heuristic can only turn the flag on, and the widget had already turned it on.
	widgetIgnored := node.Ignored
	if p.Accessibility.Callback != nil {
		SafeCall(func() { p.Accessibility.Callback(node) })
	}
	if node.Role == role.Auto || node.Role == role.None {
		// A published tree holds neither of these, and the callback is entitled to have set either: Panel.Accessibility
		// takes both, so a callback that decides a panel is a plain group is as likely to write role.None as it is to
		// leave role.Auto in place. Neither can be acted on this late — the panel has been described and its children
		// already point at this node as their parent — so both become Group, which is what Auto resolves to anyway and
		// what a virtual child with no role of its own is given. A panel that means not to be described at all says
		// so through Accessibility.Role, which is read before any of this runs. See
		// AccessibilityBuilder.AddVirtualChildOf.
		node.Role = role.Group
	}
	if node.Disabled {
		// Every request that would act on a disabled node is refused, so advertising one would offer an assistive
		// technology something it cannot have, and a screen reader that says a greyed-out control can be pressed is
		// worse than one that does not mention it. Applied after the widget and the callback have had their say, since
		// either may be the only one that knows the node is disabled. See axDisabledActions.
		node.Actions &= axDisabledActions
		// The window does not move the keyboard focus when the panel holding it is disabled, so the panel really can
		// be both. What is published is not: a node that says it holds the focus while saying it cannot take it is the
		// pair an assistive technology cannot make sense of, and there is nothing a person could do with the control
		// anyway. The focus falls back to the nearest ancestor that can be used; see axSnapshot.fallbackFocus.
		node.Focused = false
	}
	s.narrowMenuActions(p, node)
	// Decided after the three above, so that what the node is judged on is what it finally offers and says rather than
	// what it offered before being disabled stripped it back.
	if node.Ignored == widgetIgnored {
		node.Ignored = widgetIgnored || axIsScaffolding(node)
	}
	switch {
	case !node.Focused:
	case p.Is(s.focusPanel):
		// The panel really does hold the keyboard focus, so this is the node the window reports it on.
		if s.focus == 0 {
			s.focus = node.ID
		}
	default:
		// The panel does not hold the keyboard focus and the node says it is focused anyway, which only the widget or
		// the Accessibility.Callback can have said. It is a second claim on top of wherever the focus actually is, and
		// what becomes of it is decided once the whole window has been described; see
		// axSnapshot.resolveCompanionFocus.
		s.companionFocus = append(s.companionFocus, node.ID)
	}
	if s.childrenDescribed[node.ID] || staticText {
		// Either the widget described the children itself, from ProvideAccessibility — a heading holding a link asks for
		// that, since a link has to be reachable in its own right — or the node is static text, which is one element
		// however many panels it is actually built from, so its children have already been folded into its name.
		// Describing them here as well would list every one of them beneath this node twice, which is what every
		// position counted out of that list would then be wrong about.
		return
	}
	s.visitChildren(p, node.ID, visible)
}

// narrowMenuActions takes away the actions a widget advertises that would open a menu, for a widget whose window is not
// the one a menu would open in.
//
// A menu does not open where the widget that opened it is: menu.createPopup inserts the popup into ActiveWindow()'s
// root, so every request that would open one from a background window is refused — see axMayPopupMenu, which
// PopupMenu.PerformAccessibilityAction, Field.PerformAccessibilityAction, Markdown.PerformAccessibilityAction, the
// action callback NewComboField installs and Panel.axSynthesizeClick all ask. A node that went on advertising them
// would offer a screen-reader user an expand or a contextual menu that silently does nothing, which is the same drift
// the narrowing of a disabled node's actions exists to prevent.
//
// It is done here rather than in each of those widgets so that the rule is stated once, and after the widget and the
// Accessibility.Callback have had their say, since a combo box is a Field that is given its Expand by the callback
// NewComboField installs. Nothing has to invalidate anything when a window's activation changes: Window.gainedFocus and
// Window.lostFocus both mark their window for publishing, so the actions go and come back with the focus.
func (s *axSnapshot) narrowMenuActions(p *Panel, node *accessibility.Node) {
	opener, ok := p.Self.(axMenuActions)
	if !ok || axMayPopupMenu(p) {
		return
	}
	node.Actions &^= opener.axMenuOpeningActions(node)
}

// markChildrenDescribed records that the children of a node have been described by the widget itself rather than being
// left to visit. See AccessibilityBuilder.DescribeChildren.
func (s *axSnapshot) markChildrenDescribed(id accessibility.NodeID) {
	if s.childrenDescribed == nil {
		s.childrenDescribed = make(map[accessibility.NodeID]bool)
	}
	s.childrenDescribed[id] = true
}

// visitChildren describes each of p's children as children of parent, in index order — which is reading order, and also
// front-to-back order, since index 0 is drawn last and so sits on top.
func (s *axSnapshot) visitChildren(p *Panel, parent accessibility.NodeID, clip geom.Rect) {
	for i, child := range p.Children() {
		s.visitChild(child, i, parent, clip)
	}
}

// visitChild describes one child, at the given index among its parent's children, as a child of parent.
//
// It is a function of its own so that the position within a table cell, which is part of how the nodes describing the
// panels inside one are identified, can be given back with a defer. Describing a panel runs application code, and a
// panic partway through is caught well outside this call, by the SafeCall around the ProvideAccessibility of whatever
// is being described; without the defer everything described afterwards would be keyed by a position several levels too
// deep.
func (s *axSnapshot) visitChild(child *Panel, index int, parent accessibility.NodeID, clip geom.Rect) {
	// Held rather than read back through s.cell, which the description of a nested cell replaces and restores.
	if cell := s.cell; cell != nil {
		cell.path = append(cell.path, index)
		defer func() { cell.path = cell.path[:len(cell.path)-1] }()
	}
	s.visit(child, parent, clip)
}

// axIsScaffolding reports whether a node is a grouping panel with nothing to say about itself, which makes it part of
// how the window is put together rather than part of what it holds. Such a node stays in the tree so that hit testing
// and coordinate clipping still work, but an assistive technology is told to look past it.
//
// Anything that can be focused or acted on is part of what the window holds however anonymous it is. A custom canvas
// that takes the keyboard focus and responds to a click, and that sets neither a role nor a name, is precisely an
// element an assistive technology has to be able to reach — and a node it is told to look past is one it is not shown
// at all, which would leave the focus landing on something it does not have. Scrolling into view does not count, since
// every node offers that.
func axIsScaffolding(node *accessibility.Node) bool {
	if node.Focusable || node.Actions.Without(accessibility.ScrollIntoView) != 0 {
		return false
	}
	return node.Role == role.Group && node.Name == "" && node.Description == ""
}

// resolveName fills in the node's name and label associations, for a node whose widget did not name it outright. The
// order is: an explicit Accessibility.LabeledBy panel, then the sibling-label convention, then, for static text, the
// text of the panel and its descendants.
func (s *axSnapshot) resolveName(p *Panel, node *accessibility.Node) {
	if !xreflect.IsNil(p.Accessibility.LabeledBy) {
		if labeler := p.Accessibility.LabeledBy.AsPanel(); labeler != nil {
			node.LabeledBy = append(node.LabeledBy, axIDFor(labeler))
			if node.Name == "" {
				// Trimmed exactly as the sibling-label convention below trims it, so that the same "Name:" label is
				// spoken the same way whether the association was stated outright or merely inferred.
				node.Name = axTrimLabelText(axLabelText(labeler))
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

// sweepVirtualIDs discards the virtual-child ids a panel is no longer using, once it is holding appreciably more of
// them than it needs. The threshold leaves room for a control whose set of children shifts a little from snapshot to
// snapshot — a table scrolling by a row — to keep reusing its ids, while a table whose rows are replaced wholesale,
// over and over, cannot accumulate them without bound.
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
//
// Only a sibling that is still going to be published as static text counts. A *Label is the type several things in this
// package are built out of rather than a promise about what any of them is — NewLink hands back one that describes
// itself with role.Link — and a sibling that says it is something else is something a person acts on in its own right,
// not a caption for what follows it. Naming the control after it would have an assistive technology read a hyperlink's
// text as the name of the field beside it and hand back the link as the field's LabeledBy.
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
	if previous.Hidden {
		return nil
	}
	if r := previous.Accessibility.Role; r != role.Auto && r != role.Label {
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
		if axNeedsSeparatingSpace(buffer.String(), text) {
			buffer.WriteByte(' ')
		}
		buffer.WriteString(text)
		return
	}
	for _, child := range p.Children() {
		axAppendLabelText(buffer, child)
	}
}

// axNeedsSeparatingSpace reports whether a space has to be put between two pieces of a name so that the words on either
// side of the join are not run together.
//
// One is needed only when neither side already carries whitespace there. A run of text that has been broken into more
// than one panel keeps its own spacing — markdown splits "A linked heading" into "A ", the link, and " heading", each
// with the spaces that separate them still attached — so adding a space unconditionally would double every one of them
// and leave a name an assistive technology reads with a stutter in it.
func axNeedsSeparatingSpace(before, after string) bool {
	if before == "" || after == "" {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(before)
	first, _ := utf8.DecodeRuneInString(after)
	return !unicode.IsSpace(last) && !unicode.IsSpace(first)
}

// axTrimLabelText turns the text of a label into a name by dropping the surrounding whitespace and the trailing colon
// that a label placed beside a control conventionally ends with.
func axTrimLabelText(text string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), ":"))
}

// axTooltipText returns the text of a panel's tooltip, which is what a node's description falls back to. A tooltip
// built by NewTooltipWithText names itself; anything else is read from the labels it is built out of.
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
	if activeHeadless() == nil || axThrottleHeadless {
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
//
// The window is laid out first, since a description is built from where the panels are: one built while a layout
// invalidation is still pending reports the frames they had before it, which is both the geometry an assistive
// technology draws its highlight from and what the BoundsChanged events of the next diff carry. The publish that
// follows a draw is safe without this — Window.Draw validates the layout itself — but the throttled one runs from the
// task queue ahead of that pass's redraws, and HeadlessScreen.AccessibilityTree asks for a tree without drawing at all.
// It is a no-op when the layout is already valid, so the callers that lay the window out before reaching here pay
// nothing for saying so.
func (w *Window) publishAccessibilityNow() {
	if w.ax.adapterFailed {
		// There is nothing to publish to and there never will be, so describing the window would be work with nowhere
		// to go. See windowAccessibility.adapterFailed. Checked ahead of the layout below, since a window that publishes
		// nowhere has no reason to be laid out either.
		return
	}
	w.ValidateLayout()
	if w.ax == nil {
		// The layout ran application code — sizers and layout callbacks — which may have turned support off or
		// disposed of the window. Re-checked after running it, as Window.performAccessibilityAction re-checks after
		// carrying out a request, since what follows reaches through the state a withdrawal frees.
		return
	}
	w.ax.lastPublish = time.Now()
	tree := w.buildAccessibilityTree()
	if tree == nil {
		// The build ran application code that withdrew the window's accessibility state; see buildAccessibilityTree.
		// There is no state left to diff against and nothing left to publish to, and the dereferences below sit
		// outside every SafeCall, so a publish that pressed on would take the event loop down with it.
		return
	}
	events := accessibility.Diff(w.ax.last, tree)
	w.ax.last = tree
	w.apiAccessibilityPublish(tree, events)
}

// accessibilityGeometry returns where the window's content area sits on the screen, in physical pixels, along with the
// backing scale that node bounds — which are window-local and in logical units — must be multiplied by to reach that
// space.
//
// The Linux adapter is the one that converts with these two values. macOS needs none of it, since its adapter converts
// each node's bounds through the content view and the window every time it is asked, and Windows asks the platform
// where the client area is instead: a window rect there already holds its origin in the raw global pixel space, so
// scaling that origin would be wrong. See w32AccessibilityGeometry.
func (w *Window) accessibilityGeometry() (origin, scale geom.Point) {
	scale = w.BackingScale()
	return w.ContentRect().Point.MulPt(scale), scale
}
