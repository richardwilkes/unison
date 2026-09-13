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
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// This file holds the public surface of accessibility support — what a panel exposes, what a widget implements, and how
// the whole subsystem is turned on — along with the small amount of process-wide state that says whether an assistive
// technology is listening.
//
// Nothing here runs unless something has asked for it. A panel carries its AccessibilityInfo whether or not anything
// ever reads it, but no traversal, snapshot, allocation or goroutine happens until activateAccessibility has been
// called, which only the platform adapters (on their first query from an assistive technology), the environment
// override, or a headless test ever do. See accessibility_snapshot.go for the snapshot builder that activation turns
// on and accessibility_actions.go for the path an assistive technology's requests come back through.

// axMaxSelectedRows bounds how many rows a list or table describes solely because they are selected. Rows that can be
// seen are always described, and the selection is worth describing whether or not it can be seen — an assistive
// technology asks what is selected — but "select all" in a table of a million rows must not turn into a million nodes.
const axMaxSelectedRows = 200

// axReach widens the part of a list or table that can be seen by one screenful above and one below it. An assistive
// technology moves through rows one at a time and can only move onto a row that has been described, so the rows just
// past either edge have to be there for it to step onto at all; selecting one scrolls it into view, and the description
// that follows reaches a screenful further still. A screenful in each direction keeps the count bounded while leaving
// room for the pause between one description and the next.
func axReach(visible geom.Rect) geom.Rect {
	if visible.Empty() {
		return visible
	}
	visible.Y -= visible.Height
	visible.Height *= 3
	return visible
}

// AccessibilityEnvKey names the environment variable that forces accessibility support on or off, overriding what the
// platform reports. Set it to a true value, as understood by strconv.ParseBool (e.g. "1"), to build and publish
// snapshots whether or not an assistive technology appears to be running, which is useful for debugging what a screen
// reader would be told. Set it to a false value (e.g. "0") to refuse activation entirely, which is the escape hatch
// for an application that must not pay the cost of snapshots even though something on the machine — an inspection
// tool, or a component that merely happens to use the accessibility APIs — has asked for them.
const AccessibilityEnvKey = "UNISON_ACCESSIBILITY"

var (
	// accessibilityActive reports whether snapshots are being built and published. It is atomic because it is read from
	// the platform adapters' threads and from AnnounceForAccessibility, which may be called from any goroutine, while
	// only the UI thread ever writes it.
	accessibilityActive atomic.Bool
	// accessibilityEnv is what AccessibilityEnvKey asked for: 1 to force activation at startup, -1 to refuse it, and 0
	// when the variable was absent or unparsable. Parsed once during start(), so it is written before anything can read
	// it and never written again.
	accessibilityEnv int8
	// noAccessibility is set by the NoAccessibility startup option and refuses activation just as a false
	// AccessibilityEnvKey does.
	noAccessibility bool
	// axNextID is the process-wide source of node ids. Ids are handed out from one counter rather than one per window,
	// so a NodeID identifies a node without needing to be qualified by the window it belongs to. UI thread only.
	axNextID uint64
	// axSnapshotCount counts the trees built in this process, which is what the zero-cost-when-inactive tests assert on.
	// UI thread only.
	axSnapshotCount uint64
)

// AccessibilityInfo is the information a panel exposes to assistive technologies. Every field is optional: a panel that
// sets none of them is described by whatever its widget reports through AccessibilityProvider, or, for a plain panel,
// by the defaults the snapshot builder derives from the panel itself.
//
// It is held by value in Panel.Accessibility, so it is set in place rather than allocated:
//
//	p.Accessibility.Name = i18n.Text("Search")
//
// The fields are ordered for a compact memory layout rather than by importance, since every panel in every window
// carries one of these whether or not it ever has anything to say.
type AccessibilityInfo struct {
	// LabeledBy is the panel whose text names this one, for a control whose label is not simply the sibling before it.
	// Its text becomes this panel's name when Name is empty, and the association itself is reported either way, since
	// assistive technologies use it to offer the label as a separate element.
	LabeledBy Paneler
	// Callback runs last, after everything else about the node has been decided, and may adjust anything on it. Use it
	// for the occasional fact that has no field of its own, such as a heading's level.
	Callback func(node *accessibility.Node)
	// ActionCallback is consulted before the panel's AccessibilityActor implementation, if it has one, and lets a plain
	// panel handle requests without a type of its own. Return true if the request was handled.
	ActionCallback func(req accessibility.ActionRequest) bool
	// virtual holds the stable node ids of this panel's virtual children, keyed by the key the widget identifies each
	// one by. It is allocated on first use by AccessibilityBuilder.AddVirtualChild and swept when it outgrows what the
	// panel is actually using, so a table whose rows churn cannot grow it without bound.
	virtual map[any]axVirtualEntry
	// Name is what an assistive technology announces for this panel. It overrides whatever name would otherwise be
	// derived, and is the one field most panels that need attention should set.
	Name string
	// Description elaborates on Name. When it is empty, the panel's tooltip text is used instead.
	Description string
	// id is this panel's node id, assigned lazily the first time the panel is described or referred to.
	id accessibility.NodeID
	// Role is what kind of element this panel is. role.Auto, the zero value, derives it from the widget, and role.None
	// hides the panel entirely, promoting its children into its parent.
	Role role.Enum
}

// axVirtualEntry is the id a panel has handed out for one virtual-child key, along with the snapshot generation that
// last used it. The generation is what the sweep in accessibility_snapshot.go distinguishes live keys from abandoned
// ones by.
type axVirtualEntry struct {
	id   accessibility.NodeID
	used uint64
}

// AccessibilityProvider is implemented by a widget that describes itself to assistive technologies. The snapshot
// builder looks for it on Panel.Self, so it must be implemented by the widget type rather than by an embedded Panel.
//
// ProvideAccessibility is called with the node already filled in with everything that can be derived from the panel
// alone — its bounds, whether it is enabled, focusable and focused, and the actions those imply — so an implementation
// only sets what it knows better. It runs on the UI thread, inside SafeCall, and must not change the panel hierarchy or
// anything else the snapshot it is part of has already looked at.
type AccessibilityProvider interface {
	ProvideAccessibility(b *AccessibilityBuilder)
}

// AccessibilityActor is implemented by a widget that can carry out requests from an assistive technology. The snapshot
// builder looks for it on Panel.Self, so it must be implemented by the widget type rather than by an embedded Panel.
//
// PerformAccessibilityAction runs on the UI thread. Return true if the request was carried out, or false to let the
// default behavior for the action, if there is one, take over.
type AccessibilityActor interface {
	PerformAccessibilityAction(req accessibility.ActionRequest) bool
}

// AccessibilityBuilder is handed to AccessibilityProvider implementations to describe one panel. It is valid only for
// the duration of the ProvideAccessibility call it was passed to and must not be retained.
type AccessibilityBuilder struct {
	snapshot *axSnapshot
	node     *accessibility.Node
	panel    *Panel
	// clip is the region, in window-local coordinates, that this panel's content is visible within. A virtual child
	// lying wholly outside it is Offscreen, exactly as a real child is.
	clip geom.Rect
	// virtualUsed counts the virtual children added during this call, which is what the sweep of abandoned keys is
	// measured against.
	virtualUsed int
}

// Node returns the node being described. Everything on it may be changed.
func (b *AccessibilityBuilder) Node() *accessibility.Node {
	return b.node
}

// Panel returns the panel being described.
func (b *AccessibilityBuilder) Panel() *Panel {
	return b.panel
}

// Window returns the window the panel being described belongs to.
func (b *AccessibilityBuilder) Window() *Window {
	return b.snapshot.window
}

// Focused returns true if the panel being described holds the keyboard focus within its window, whether or not that
// window is the active one.
func (b *AccessibilityBuilder) Focused() bool {
	return b.node.Focused
}

// VisibleRect returns the part of the panel that is actually visible, in the panel's own coordinates, clipped by every
// ancestor that clips it — which includes every ScrollPanel it sits inside. A widget with more content than it can show
// uses this to describe only what can be seen, rather than building a node for every row of a table with a million of
// them. The result is empty when nothing of the panel is visible.
func (b *AccessibilityBuilder) VisibleRect() geom.Rect {
	if b.clip.Empty() {
		return geom.Rect{}
	}
	return b.panel.RectFromRoot(b.clip).Intersect(b.panel.ContentRect(true))
}

// IDFor returns the node id of another panel, assigning one if it does not have it yet. Use it to point at a node from
// a relationship such as Node.LabeledBy or Node.Controls. The id is valid whether or not that panel has been described
// yet, and whether or not it ends up in the tree at all, so a relationship may be recorded without regard to the order
// the panels happen to be visited in.
func (b *AccessibilityBuilder) IDFor(p Paneler) accessibility.NodeID {
	if xreflect.IsNil(p) {
		return 0
	}
	return axIDFor(p.AsPanel())
}

// AddVirtualChild adds a node that has no panel of its own as a child of the node being described, and returns its id.
// Rows of a list or table, and the cells within them, exist this way: there is no panel per row to describe, so the
// widget describes each one directly.
//
// key identifies the child within this panel and must be comparable. The same key always yields the same node id, so
// an assistive technology's notion of a row survives the rows around it being added, removed or reordered. Tables key
// rows by their tid.TID, lists key them by index, and cells use accessibility.CellKey.
//
// fill is called with a node that has nothing but its identity filled in. Set its Bounds in the panel's own
// coordinates; they are converted afterwards, and the child is marked Offscreen when none of it can be seen, exactly
// as a real child is.
func (b *AccessibilityBuilder) AddVirtualChild(key any, fill func(n *accessibility.Node)) accessibility.NodeID {
	return b.AddVirtualChildOf(b.node.ID, key, fill)
}

// AddVirtualChildOf is AddVirtualChild with an explicit parent, which must be a node this panel has already added. Use
// it to build more than one level of virtual children, such as the cells of a table row.
func (b *AccessibilityBuilder) AddVirtualChildOf(parent accessibility.NodeID, key any,
	fill func(n *accessibility.Node),
) accessibility.NodeID {
	if fill == nil {
		return 0
	}
	parentNode := b.snapshot.tree.Nodes[parent]
	if parentNode == nil {
		return 0
	}
	id := b.virtualID(key)
	node := &accessibility.Node{
		ID:     id,
		Parent: parent,
	}
	fill(node)
	raw := b.panel.RectToRoot(node.Bounds)
	node.Bounds = raw
	node.Offscreen = !raw.Intersects(b.clip) && !raw.Empty()
	b.snapshot.tree.Nodes[id] = node
	b.snapshot.targets[id] = axTarget{panel: b.panel, key: key}
	parentNode.Children = append(parentNode.Children, id)
	return id
}

// addCellPanel describes the panel a table row handed back for one of its cells, and everything inside it, beneath the
// node for that cell. The panel must be attached to the table and laid out at the cell's frame, as it is for drawing,
// for the duration of the call. See axCellContext for how the resulting nodes are identified and how requests about
// them find their way back. A cell that stays attached to the table beyond this call — the one holding the keyboard
// focus — is described as the real panels it is made of, under their own ids, since they persist and can be reached.
func (b *AccessibilityBuilder) addCellPanel(parent accessibility.NodeID, key accessibility.CellKey, p *Panel,
	persistent bool,
) {
	if p == nil || b.snapshot.tree.Nodes[parent] == nil {
		return
	}
	saved := b.snapshot.cell
	if persistent {
		b.snapshot.cell = nil
	} else {
		b.snapshot.cell = &axCellContext{builder: b, key: key}
	}
	b.snapshot.visit(p, parent, b.clip)
	b.snapshot.cell = saved
}

// virtualID returns the stable node id this panel uses for key, allocating one the first time key is seen.
func (b *AccessibilityBuilder) virtualID(key any) accessibility.NodeID {
	if b.panel.Accessibility.virtual == nil {
		b.panel.Accessibility.virtual = make(map[any]axVirtualEntry)
	}
	entry := b.panel.Accessibility.virtual[key]
	if entry.id == 0 {
		entry.id = axAllocID()
	}
	entry.used = b.snapshot.generation
	b.panel.Accessibility.virtual[key] = entry
	b.virtualUsed++
	return entry.id
}

// NoAccessibility returns a startup option that refuses accessibility support from the start. No snapshot is built and
// no platform adapter is created, whatever an assistive technology asks for, until SetAccessibilityEnabled(true) lifts
// the refusal. It is the programmatic equivalent of setting AccessibilityEnvKey to a false value.
func NoAccessibility() StartupOption {
	return func(_ startupOption) error {
		noAccessibility = true
		return nil
	}
}

// SetAccessibilityEnabled turns accessibility support off, or back on, while the application is running. Turning it
// off shuts down whatever is currently serving an assistive technology, frees everything that was built for it, and
// refuses every request to start again, so that an application which does not want the cost of being described — or
// wants to leave that to a preference — is not made to pay it by something on the desktop deciding to ask. Turning it
// back on lifts the refusal: nothing starts until an assistive technology next asks, exactly as at startup, except on
// Linux, where the desktop is asked again whether one is already there. The environment still has the last word: a
// false AccessibilityEnvKey refuses support whatever this is told.
//
// May be called from any goroutine; the change is made on the UI thread. Use NoAccessibility to refuse support before
// the application has started.
func SetAccessibilityEnabled(enabled bool) {
	if !onUIThread() {
		InvokeTask(func() { SetAccessibilityEnabled(enabled) })
		return
	}
	if noAccessibility == !enabled {
		return
	}
	noAccessibility = !enabled
	if !enabled {
		deactivateAccessibility()
	}
	apiAccessibilityEnabledChanged(enabled)
}

// AccessibilityEnabled reports whether accessibility support is permitted: neither the NoAccessibility startup option,
// SetAccessibilityEnabled(false) nor a false AccessibilityEnvKey has refused it. It says nothing about whether an
// assistive technology is actually being served; IsAccessibilityActive does that.
func AccessibilityEnabled() bool {
	return !noAccessibility && accessibilityEnv >= 0
}

// IsAccessibilityActive returns true if an assistive technology is being served, which is when snapshots of each
// window are being built and published. It is safe to call from any goroutine.
func IsAccessibilityActive() bool {
	return accessibilityActive.Load()
}

// AnnounceForAccessibility asks the platform's assistive technology to speak text, for something the user should hear
// about that no change to a window expresses — a background task that finished, say. It does nothing when no assistive
// technology is being served, so an application may call it unconditionally.
//
// It is safe to call from any goroutine; work that has to happen on the UI thread is handed to it.
func AnnounceForAccessibility(text string) {
	if text == "" || !accessibilityActive.Load() {
		return
	}
	if onUIThread() {
		apiAccessibilityAnnounce(text)
		return
	}
	InvokeTask(func() { apiAccessibilityAnnounce(text) })
}

// applyAccessibilityEnvRequest records what AccessibilityEnvKey asks for. This runs during startup, before any window
// can exist, so that what it decides is in place before the first thing that could consult it.
func applyAccessibilityEnvRequest() {
	accessibilityEnv = 0
	v, ok := os.LookupEnv(AccessibilityEnvKey)
	if !ok {
		return
	}
	on, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return
	}
	if on {
		accessibilityEnv = 1
		slog.Info("accessibility support was requested via the environment", "var", AccessibilityEnvKey)
	} else {
		accessibilityEnv = -1
		slog.Info("accessibility support was refused via the environment", "var", AccessibilityEnvKey)
	}
}

// activateAccessibility turns snapshot building on and reports whether accessibility support is now available. It is
// idempotent, so a platform adapter may call it on every query without checking first, and returns false — having done
// nothing — when the environment or the NoAccessibility startup option has refused it.
//
// Every window is marked for redraw, since publishing happens after a window is drawn: without this, a window that is
// sitting idle when an assistive technology starts up would not be described until something else happened to make it
// redraw.
func activateAccessibility() bool {
	if noAccessibility || accessibilityEnv < 0 {
		return false
	}
	if !accessibilityActive.Swap(true) {
		for _, wnd := range windowList {
			wnd.MarkForRedraw()
		}
	}
	return true
}

// deactivateAccessibility turns snapshot building off, shuts down every window's platform adapter and frees the
// snapshots and registries those were answering from. Nothing is built again until activateAccessibility is called
// once more.
func deactivateAccessibility() {
	if !accessibilityActive.Swap(false) {
		return
	}
	for _, wnd := range windowList {
		if wnd.ax != nil {
			wnd.apiAccessibilityShutdown()
			wnd.ax = nil
		}
	}
}

// axIDFor returns the node id of a panel, assigning one if it does not have it yet. A panel keeps its id for life, so
// an assistive technology's notion of an element survives the snapshots it appears in.
func axIDFor(p *Panel) accessibility.NodeID {
	if p == nil {
		return 0
	}
	if p.Accessibility.id == 0 {
		p.Accessibility.id = axAllocID()
	}
	return p.Accessibility.id
}

// axAllocID returns the next unused node id.
func axAllocID() accessibility.NodeID {
	axNextID++
	return accessibility.NodeID(axNextID)
}
