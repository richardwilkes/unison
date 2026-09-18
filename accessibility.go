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
	// it and never written again. It is atomic because the goroutine that runs start() is not the one that reads it
	// through AccessibilityEnabled, which documents that it may be called from anywhere.
	accessibilityEnv atomic.Int32
	// noAccessibility is set by the NoAccessibility startup option and by SetAccessibilityEnabled, and refuses
	// activation just as a false AccessibilityEnvKey does. It is atomic because AccessibilityEnabled reports what it
	// holds and, like the SetAccessibilityEnabled that writes it, may be called from any goroutine; only the UI thread
	// ever writes it.
	noAccessibility atomic.Bool
	// axNextID is the process-wide source of node ids. Ids are handed out from one counter rather than one per window,
	// so a NodeID identifies a node without needing to be qualified by the window it belongs to. UI thread only.
	axNextID uint64
	// axSnapshotCount counts the trees built in this process, which is what the zero-cost-when-inactive tests assert
	// on. UI thread only.
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
// Set the fields one at a time, as above, rather than assigning the struct as a whole. Alongside what a panel says
// about itself it carries that panel's identity — the node id an assistive technology knows the panel by, and the ids
// of any virtual children it has handed out — and that identity belongs to the one panel holding it. Assigning a fresh
// struct (p.Accessibility = AccessibilityInfo{Name: "x"}) throws the identity away, and the panel is described under a
// new id, which an assistive technology reads as the old element having been removed and a new one put in its place:
// whatever it was saying about the old one stops, and the focus it was tracking is lost. Copying one panel's
// information onto another (q.Accessibility = p.Accessibility) would hand two live panels the same identity, which is
// caught and repaired — the panel that was copied onto is the one given a fresh id, whichever order the two happen to
// be described in, since the identity is recorded along with the panel it was handed to — but the copy still gains
// nothing, since the ids cannot be shared.
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
	//
	// One thing it cannot decide on its own is the keyboard focus. A window reports the focus on the node of the panel
	// that actually holds it, and a callback that sets accessibility.Node.Focused on any other node — or on a virtual
	// child — is making a second claim on top of that, which is kept only where a screen reader needs a second focused
	// object, and only for a node inside the panel that really holds the focus while that panel's own node is what the
	// focus is reported on. Everywhere else it is taken away again before the tree is published. See
	// accessibility.Node.Focused.
	Callback func(node *accessibility.Node)
	// ActionCallback is consulted before the panel's AccessibilityActor implementation, if it has one, and lets a plain
	// panel handle requests without a type of its own. Return true if the request was handled.
	ActionCallback func(req accessibility.ActionRequest) bool
	// virtual holds the stable node ids of this panel's virtual children, keyed by the key the widget identifies each
	// one by. It is allocated on first use by AccessibilityBuilder.AddVirtualChild and swept when it outgrows what the
	// panel is actually using, so a table whose rows churn cannot grow it without bound.
	virtual map[any]axVirtualEntry
	// owner is the panel id and virtual belong to, recorded when the id was handed out, so that identity copied onto
	// another panel by assigning the struct as a whole is recognized as not being that panel's own. See axIDFor.
	owner *Panel
	// Name is what an assistive technology announces for this panel. It overrides whatever name would otherwise be
	// derived, and is the one field most panels that need attention should set.
	Name string
	// Description elaborates on Name. When it is empty, the panel's tooltip text is used instead.
	Description string
	// URL is where this panel leads, for a panel that is a link. NewLink fills it in from the target it was given, and
	// an application that builds a link of its own sets it so that an assistive technology can say where the link goes
	// and offer it among a document's links. It is copied onto the node as accessibility.Node.URL and means nothing for
	// any other kind of panel.
	URL string
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
// as a real child is. A node left with no Role is described as role.Group, since neither role.Auto nor role.None means
// anything for a child that exists only because a widget described it.
//
// Zero is returned, and nothing is added, when the key has already been used during this description, since a key names
// one child and not two. Reusing one — a table whose rows hand back the same tid.TID, or the same key added under two
// parents — is a mistake in the widget rather than something to paper over.
func (b *AccessibilityBuilder) AddVirtualChild(key any, fill func(n *accessibility.Node)) accessibility.NodeID {
	return b.AddVirtualChildOf(b.node.ID, key, fill)
}

// AddVirtualChildOf is AddVirtualChild with an explicit parent, which must be a node this panel has already added. Use
// it to build more than one level of virtual children, such as the cells of a table row. As with AddVirtualChild, a key
// that has already been used during this description adds nothing and yields zero.
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
	if existing := b.existingVirtualID(key); existing != 0 && b.snapshot.tree.Nodes[existing] != nil {
		// The key named a node that is already in this tree, so it already has a parent listing it among its children.
		// Describing it a second time would replace what it said the first time and list its id twice among its
		// siblings, which is what every position counted out of that list — the index within the parent, what
		// Tree.PositionInSet answers, and the events the next Diff produces — would then be wrong about. Refusing
		// leaves the mistake where a widget can see it rather than burying it in the tree.
		//
		// The id is looked up rather than allocated, so a refused key neither uses up a virtual child nor renews the
		// generation the sweep of abandoned keys reads: a mistake must not make the panel look busier than it is.
		return 0
	}
	id := b.virtualID(key)
	node := &accessibility.Node{
		ID:     id,
		Parent: parent,
	}
	fill(node)
	if node.Role == role.Auto || node.Role == role.None {
		// A published tree holds neither of these. Auto means "work the role out from the widget", and for a child a
		// widget invented there is nothing to work it out from; None means "do not describe this panel", which is an
		// instruction about a panel rather than about something that exists only because a widget described it. Both
		// become Group, which is what the builder resolves Auto to for a real panel, so that an adapter is never handed
		// a role the schema says cannot occur.
		node.Role = role.Group
	}
	if !b.panel.Enabled() {
		// A virtual child has no existence apart from the panel that described it, and every request aimed at one is
		// carried out by that panel, so a disabled panel's children cannot be acted on either — the rows of a disabled
		// table are no more selectable than the table is. Saying so here keeps what is offered in step with what
		// axDispatchAction will actually do, without every collection widget having to remember it.
		node.Disabled = true
	}
	if node.Disabled {
		node.Actions &= axDisabledActions
	}
	if node.Focused {
		// A virtual child has no panel of its own, so it can never be the panel that holds the keyboard focus: a claim
		// on the focus from one is a second claim on top of wherever the focus actually is, exactly as a claim from a
		// real panel that does not hold it is, and it is decided the same way once the whole window has been described.
		// Without this a widget could publish a second focused node on every platform, whatever the platform's screen
		// reader makes of one. See axSnapshot.resolveCompanionFocus.
		b.snapshot.companionFocus = append(b.snapshot.companionFocus, id)
	}
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
	// Restored with a defer because the description of a cell runs application code — a row building its panels, a
	// widget describing itself — and a panic partway through is caught by the SafeCall around the table's own
	// ProvideAccessibility, well outside this call. Everything described after that would otherwise still be treated as
	// part of this cell: identified by a key of the table's rather than by its own panel, with every request about it
	// sent to the table.
	defer func() { b.snapshot.cell = saved }()
	if persistent {
		b.snapshot.cell = nil
	} else {
		b.snapshot.cell = &axCellContext{builder: b, key: key}
	}
	b.snapshot.visit(p, parent, b.clip)
}

// existingVirtualID returns the node id this panel has already handed out for key, or zero if it has not handed one
// out. Nothing is allocated, nothing is counted and nothing is marked as still in use, so asking about a key that turns
// out not to be usable leaves no trace.
func (b *AccessibilityBuilder) existingVirtualID(key any) accessibility.NodeID {
	return b.panel.Accessibility.virtual[key].id
}

// virtualID returns the stable node id this panel uses for key, allocating one the first time key is seen, and records
// the key as one this snapshot is using, along with counting it among the virtual children this description has added.
func (b *AccessibilityBuilder) virtualID(key any) accessibility.NodeID {
	id := b.virtualIDOf(b.panel, key)
	b.virtualUsed++
	return id
}

// virtualIDOf returns the stable node id another panel uses for one of its virtual-child keys, allocating one if that
// panel has not handed one out yet. It is how a widget refers to a virtual child of a panel that has not been described
// yet: a document composing its content into one stream records the node occupying each part of it, and the rows of a
// table inside it exist only as virtual children the table will invent when its own turn comes.
//
// The id is recorded as one this snapshot is using, so a key a document has pointed at is not swept away as abandoned
// before the panel that owns it has been asked about it. What is not counted is the panel's own tally of virtual
// children: that tally is what the sweep measures the map against, and it belongs to the panel being described rather
// than to whoever asked about one of its keys.
//
// The panel's own identity is established first, because that is what its map of virtual keys hangs from: axIDFor
// throws the map away whenever it hands a panel a new id — a panel that has never been described, or one whose id was
// handed out while it belonged to something else — so a key recorded before the panel had an id would be discarded the
// moment the panel was visited, after which the same key would be allocated a second, different id. Everything that
// pointed at the first one — the spans of a document's stream, say — would then name a node that is not in the tree.
func (b *AccessibilityBuilder) virtualIDOf(p *Panel, key any) accessibility.NodeID {
	if p == nil {
		return 0
	}
	axIDFor(p)
	if p.Accessibility.virtual == nil {
		p.Accessibility.virtual = make(map[any]axVirtualEntry)
	}
	entry := p.Accessibility.virtual[key]
	if entry.id == 0 {
		entry.id = axAllocID()
	}
	entry.used = b.snapshot.generation
	p.Accessibility.virtual[key] = entry
	return entry.id
}

// NoAccessibility returns a startup option that refuses accessibility support from the start. No snapshot is built and
// no platform adapter is created, whatever an assistive technology asks for, until SetAccessibilityEnabled(true) lifts
// the refusal. It is the programmatic equivalent of setting AccessibilityEnvKey to a false value.
func NoAccessibility() StartupOption {
	return func(_ startupOption) error {
		noAccessibility.Store(true)
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
	if noAccessibility.Load() == !enabled {
		return
	}
	noAccessibility.Store(!enabled)
	if !enabled {
		deactivateAccessibility()
	}
	apiAccessibilityEnabledChanged(enabled)
}

// AccessibilityEnabled reports whether accessibility support is permitted: neither the NoAccessibility startup option,
// SetAccessibilityEnabled(false) nor a false AccessibilityEnvKey has refused it. It says nothing about whether an
// assistive technology is actually being served; IsAccessibilityActive does that.
//
// It is safe to call from any goroutine, as is SetAccessibilityEnabled.
func AccessibilityEnabled() bool {
	return !noAccessibility.Load() && accessibilityEnv.Load() >= 0
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
	// The task tests the flag again because it runs later: an announcement made from another goroutine just as the
	// assistive technology goes away, or just as SetAccessibilityEnabled(false) tears everything down, would otherwise
	// be spoken after there was anything left to speak it. apiAccessibilityAnnounce tests it as well, since it is the
	// entry every path arrives through, but saying so here keeps what the deferral costs visible where the deferral is
	// made.
	InvokeTask(func() {
		if accessibilityActive.Load() {
			apiAccessibilityAnnounce(text)
		}
	})
}

// applyAccessibilityEnvRequest records what AccessibilityEnvKey asks for. This runs during startup, before any window
// can exist, so that what it decides is in place before the first thing that could consult it.
func applyAccessibilityEnvRequest() {
	accessibilityEnv.Store(0)
	v, ok := os.LookupEnv(AccessibilityEnvKey)
	if !ok {
		return
	}
	on, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return
	}
	if on {
		accessibilityEnv.Store(1)
		slog.Info("accessibility support was requested via the environment", "var", AccessibilityEnvKey)
	} else {
		accessibilityEnv.Store(-1)
		slog.Info("accessibility support was refused via the environment", "var", AccessibilityEnvKey)
	}
}

// activateAccessibility turns snapshot building on and reports whether accessibility support is now available. It is
// idempotent, so a platform adapter may call it on every query without checking first, and returns false — having done
// nothing — when the environment or the NoAccessibility startup option has refused it.
//
// Every window is marked for redraw, since publishing happens after a window is drawn: without this, a window that is
// sitting idle when an assistive technology starts up would not be described until something else happened to make it
// redraw. On the platforms whose screen readers start from the keyboard focus, the active window is also given the
// chance to choose a focus if nothing in it holds one, since the panels that take the focus only for an assistive
// technology's sake have just become able to; see Panel.axTakesFocus. A window that already holds a focus keeps it,
// and any other window chooses one as it is next activated, exactly as it always has.
func activateAccessibility() bool {
	if noAccessibility.Load() || accessibilityEnv.Load() < 0 {
		return false
	}
	if !accessibilityActive.Swap(true) {
		for _, wnd := range windowList {
			wnd.MarkForRedraw()
			if axReadersFollowFocus && wnd.Focused() && wnd.focus == nil {
				wnd.FocusNext()
			}
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

// axMarkForPublish marks a window for redraw when an assistive technology is being served, for a change that alters
// what the window's description says without necessarily changing anything that is drawn. A description is published
// after a window has been drawn, so a change nothing repaints for — the focus moving between two panels that do not
// draw themselves any differently for holding it, or a window becoming the active one while its focus is empty — would
// otherwise sit unreported until something unrelated happened to redraw the window, which may be never.
//
// An application nothing is listening to pays one atomic load for each of these.
func (w *Window) axMarkForPublish() {
	if accessibilityActive.Load() {
		w.MarkForRedraw()
	}
}

// axFocusOnClick moves the keyboard focus to a control a person has just clicked, for the controls that do not
// otherwise take it — a check box, a radio button, a button, a popup menu and a color well — and only while an
// assistive technology is being served. A screen reader speaks a change of state only for the control that holds the
// focus, so a click that toggles a check box the focus is not on goes unspoken however faithfully the change is
// published: the notification arrives for an element the screen reader is not watching. Taking the focus puts the two
// together, and since accessibility.Diff reports the focus move after every other change in the same publish, what is
// spoken is the control in its new state. Windows and GTK controls take the focus on a click regardless, so a person
// using a screen reader there is given nothing they were not already used to.
//
// Nothing changes when no assistive technology is being served. These controls have never taken the focus on a click,
// so that clicking a check box does not pull the focus out of the text field a person is typing in, and an application
// nothing is listening to pays one atomic load for each click.
func (p *Panel) axFocusOnClick() {
	if accessibilityActive.Load() {
		p.RequestFocus()
	}
}

// axWindowHidden takes a window that is no longer on the screen out of the description an assistive technology holds,
// where the platform calls for that.
//
// A hidden or minimized window is not drawn, so nothing is published for it and the last description of it would
// otherwise stand for as long as it existed — on AT-SPI, where the application says for itself what windows it has,
// that leaves a window a person cannot see listed as showing and visible. There everything built for the window is
// released rather than merely suspended, since a window that is hidden may never be shown again; showing it again draws
// it, which publishes it afresh, and an adapter takes a window it has been told about before back exactly as it took it
// the first time. On macOS and Windows the system lists the application's windows itself and a hidden one simply drops
// out of the list, so what was built is kept and the assistive technology finds the elements it already knows when the
// window comes back.
//
// What is not released is the count of snapshots taken of the window, which goes on where it left off when the window
// is next described. See Window.axGeneration.
func (w *Window) axWindowHidden() {
	if w.ax == nil {
		return
	}
	if w.apiAccessibilityWindowHidden() {
		w.ax = nil
	}
}

// axIDFor returns the node id of a panel, assigning one if it does not have it yet. A panel keeps its id for life, so
// an assistive technology's notion of an element survives the snapshots it appears in.
//
// An id that arrived by having another panel's AccessibilityInfo assigned onto this one is not this panel's to use, and
// is replaced here along with the virtual-child ids that came with it. Two live panels sharing an id would otherwise
// describe themselves into the same entry of the tree and appear as a child of two different parents, which is worse
// than the lost identity the copy has already cost.
func axIDFor(p *Panel) accessibility.NodeID {
	if p == nil {
		return 0
	}
	if p.Accessibility.id == 0 || p.Accessibility.owner != p {
		p.Accessibility.id = axAllocID()
		p.Accessibility.owner = p
		p.Accessibility.virtual = nil
	}
	return p.Accessibility.id
}

// axAllocID returns the next unused node id.
func axAllocID() accessibility.NodeID {
	axNextID++
	return accessibility.NodeID(axNextID)
}

// axTakesFocus reports whether a panel that takes the keyboard focus only for an assistive technology's sake — a
// Markdown, whose content is read rather than acted on — takes it now. It does so while an assistive technology is
// being served, and only on the platforms whose screen readers start from the keyboard focus; see axReadersFollowFocus.
//
// Such a panel has no use for the focus itself: it draws no differently for holding it and handles no keys of its
// own. What it holds the focus for is where a screen reader begins. Narrator keeps its cursor on the focused element,
// and a window in which nothing holds the focus leaves that cursor on the window's own element, from which Narrator's
// scan mode, heading and link navigation all refuse to move — while from any element inside the window they move
// through the rest of it freely. Giving the document the focus puts the cursor inside the content, where the person can
// read it. A keyboard user nothing is listening to is left alone, since these panels have never been tab stops, and an
// application pays one atomic load per Focusable call on a panel marked this way, and nothing at all for the rest.
func (p *Panel) axTakesFocus() bool {
	return p.axFocusable && axReadersFollowFocus && accessibilityActive.Load()
}
