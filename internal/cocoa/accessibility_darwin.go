// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

// This file is the NSAccessibility adapter: it turns the immutable accessibility.Tree snapshots the root package
// publishes into a graph of Objective-C objects that VoiceOver and the Accessibility Inspector can walk, and turns the
// requests they make back into accessibility.ActionRequests.
//
// Everything here runs on the main thread. AppKit delivers accessibility queries through the run loop, so they arrive
// inside nativeWaitEvents or a nested modal loop — never part-way through a draw — and the answers are read from the
// snapshot rather than from any live widget. There is no KVO, no polling, no goroutine and no notification observer:
// nothing in this file runs until an assistive technology sends the content view one of the three selectors that can
// activate accessibility support (see view_darwin.go), and nothing runs afterwards except in answer to a query, a
// published snapshot, or a window being torn down.
//
// An assistive technology's request is handed to AccessibilityActionCallback, and the answer given back is optimistic:
// "yes, this element advertises that action" rather than "yes, that has been done". What happens next is the callback's
// to decide. The root package queues anything that may run arbitrary application code — a press, which can open a modal
// dialog — since an assistive technology must never be left waiting on that, but it carries out the requests that
// merely move the focus, the selection or the view on the spot, because VoiceOver reads the result straight after
// asking and would otherwise be handed the state from before its own request.
//
// So an inline request re-enters this file: the callback lays the window out and publishes a new snapshot before
// returning, which runs Publish, and with it postEvents and destroyElement, from inside the AppKit callback the request
// arrived through. Two things make that safe. The adapter holds no state across a callback that the publish could
// invalidate — every answer is read from a.tree at the moment it is asked, and the elements an in-flight
// setAccessibilitySelectedRows: is working through are retained by the NSArray AppKit passed in. And an element whose
// node leaves the tree during such a publish is autoreleased rather than released outright (see releaseElement), so it
// survives until AppKit drains the run loop's pool, well after the accessibility method it is standing in has returned.
//
// The window's root node is represented by the content view rather than by an element of its own: the content view
// answers accessibilityChildren, accessibilityFocusedUIElement and accessibilityHitTest: for it, reports itself as an
// ignored group, and so is spliced out of the hierarchy by AppKit, leaving the NSWindow to stand in as the root
// container that the root node's children hang from. Every other node gets a UnisonAXElement, created the first time
// something asks about it and released when the node leaves the tree.

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// Callbacks invoked by the accessibility adapter. They are invoked on the main thread.
var (
	// AccessibilityActivateCallback is invoked the first time an assistive technology queries a window's content view
	// while that window has no adapter yet. The implementation must turn accessibility support on, build and publish a
	// snapshot of the window synchronously — the query being answered has no other source of truth — and report whether
	// an adapter now exists. Returning false leaves the query to AppKit's own answer.
	AccessibilityActivateCallback func(w Window) bool
	// AccessibilityActionCallback is invoked when an assistive technology asks a node to do something. What happens
	// next is the implementation's to decide, and on this platform the answer is not the same for every request. One
	// that merely moves the focus, the selection or the view is carried out inline, before the callback returns,
	// because the assistive technology reads the result straight after asking and would otherwise be handed the state
	// from before its own request. One that may run arbitrary application code — a press, which can open a modal
	// dialog — is queued for the UI thread instead, since an assistive technology must never be left waiting on that.
	// An inline request re-enters this file, publishing a new snapshot from within the callback; dispatch and
	// releaseElement exist to make that safe, and the file comment above explains how.
	AccessibilityActionCallback func(w Window, req accessibility.ActionRequest)
)

// The AppKit NSString* constants this adapter uses, named by their exported symbols and resolved on first use through
// AppKitString. Roles and subroles come first, then the notification names.
const (
	axRoleButton             = "NSAccessibilityButtonRole"
	axRoleDisclosureTriangle = "NSAccessibilityDisclosureTriangleRole"
	axRoleCell               = "NSAccessibilityCellRole"
	axRoleCheckBox           = "NSAccessibilityCheckBoxRole"
	axRoleColorWell          = "NSAccessibilityColorWellRole"
	axRoleComboBox           = "NSAccessibilityComboBoxRole"
	axRoleGroup              = "NSAccessibilityGroupRole"
	axRoleHelpTag            = "NSAccessibilityHelpTagRole"
	axRoleImage              = "NSAccessibilityImageRole"
	axRoleIncrementor        = "NSAccessibilityIncrementorRole"
	axRoleLink               = "NSAccessibilityLinkRole"
	axRoleList               = "NSAccessibilityListRole"
	axRoleMenu               = "NSAccessibilityMenuRole"
	axRoleMenuBar            = "NSAccessibilityMenuBarRole"
	axRoleMenuItem           = "NSAccessibilityMenuItemRole"
	axRoleOutline            = "NSAccessibilityOutlineRole"
	axRolePopUpButton        = "NSAccessibilityPopUpButtonRole"
	axRoleProgressIndicator  = "NSAccessibilityProgressIndicatorRole"
	axRoleRadioButton        = "NSAccessibilityRadioButtonRole"
	axRoleRow                = "NSAccessibilityRowRole"
	axRoleScrollArea         = "NSAccessibilityScrollAreaRole"
	axRoleScrollBar          = "NSAccessibilityScrollBarRole"
	axRoleSlider             = "NSAccessibilitySliderRole"
	axRoleStaticText         = "NSAccessibilityStaticTextRole"
	axRoleTabGroup           = "NSAccessibilityTabGroupRole"
	axRoleTable              = "NSAccessibilityTableRole"
	axRoleTextArea           = "NSAccessibilityTextAreaRole"
	axRoleTextField          = "NSAccessibilityTextFieldRole"
	axRoleToolbar            = "NSAccessibilityToolbarRole"
	axRoleUnknown            = "NSAccessibilityUnknownRole"
	axRoleWindow             = "NSAccessibilityWindowRole"

	axSubroleOutlineRow      = "NSAccessibilityOutlineRowSubrole"
	axSubroleSecureTextField = "NSAccessibilitySecureTextFieldSubrole"
	axSubroleSortButton      = "NSAccessibilitySortButtonSubrole"
	axSubroleTabButton       = "NSAccessibilityTabButtonSubrole"
	axSubroleToggle          = "NSAccessibilityToggleSubrole"

	axNotifyAnnouncementRequested   = "NSAccessibilityAnnouncementRequestedNotification"
	axNotifyFocusedUIElement        = "NSAccessibilityFocusedUIElementChangedNotification"
	axNotifyLayoutChanged           = "NSAccessibilityLayoutChangedNotification"
	axNotifyRowCollapsed            = "NSAccessibilityRowCollapsedNotification"
	axNotifyRowExpanded             = "NSAccessibilityRowExpandedNotification"
	axNotifySelectedCellsChanged    = "NSAccessibilitySelectedCellsChangedNotification"
	axNotifySelectedChildrenChanged = "NSAccessibilitySelectedChildrenChangedNotification"
	axNotifySelectedRowsChanged     = "NSAccessibilitySelectedRowsChangedNotification"
	axNotifySelectedTextChanged     = "NSAccessibilitySelectedTextChangedNotification"
	axNotifyTitleChanged            = "NSAccessibilityTitleChangedNotification"
	axNotifyUIElementDestroyed      = "NSAccessibilityUIElementDestroyedNotification"
	axNotifyValueChanged            = "NSAccessibilityValueChangedNotification"

	axKeyAnnouncement = "NSAccessibilityAnnouncementKey"
	axKeyPriority     = "NSAccessibilityPriorityKey"
	axKeyUIElements   = "NSAccessibilityUIElementsKey"
)

// The attribute names a menu item's accelerator is reported through. They are spelled out rather than resolved through
// AppKitString because they are constants of ApplicationServices rather than of AppKit — kAXMenuItemCmdCharAttribute
// and kAXMenuItemCmdModifiersAttribute, which are CFStrings with no exported NSString counterpart — and the strings
// themselves have been these since Mac OS X 10.1.
const (
	axAttrMenuItemCmdChar      = "AXMenuItemCmdChar"
	axAttrMenuItemCmdModifiers = "AXMenuItemCmdModifiers"
)

// The bits of AXMenuItemCmdModifiers (AXMenuItemModifiers in ApplicationServices' AXAttributeConstants.h). Zero means
// the command key by itself, which is why a shortcut that does not use it has to say so with the no-command bit.
const (
	axMenuModifierShift     int64 = 1 << 0
	axMenuModifierOption    int64 = 1 << 1
	axMenuModifierControl   int64 = 1 << 2
	axMenuModifierNoCommand int64 = 1 << 3
)

// AppKit enumeration values used by the adapter (verified against the macOS SDK's NSAccessibilityProtocols.h and
// NSAccessibilityConstants.h, which document them as fixed).
const (
	axOrientationUnknown    int64 = 0
	axOrientationVertical   int64 = 1
	axOrientationHorizontal int64 = 2

	axSortDirectionUnknown    int64 = 0
	axSortDirectionAscending  int64 = 1
	axSortDirectionDescending int64 = 2

	// axPriorityHigh is NSAccessibilityPriorityHigh, which is what an announcement the application went out of its way
	// to make deserves: it interrupts whatever is being spoken instead of being dropped.
	axPriorityHigh int64 = 90
)

// axHeadingCandidateRole is the role name macOS uses for headings in web content. There is no public constant for it,
// and whether the accessibility system knows it at all is discovered at runtime by axHeadingRole.
const axHeadingCandidateRole = "AXHeading"

// AXAdapter serves one window's accessibility hierarchy. It holds the most recently published snapshot and the
// Objective-C element for every node an assistive technology has asked about, and answers every query from that
// snapshot without touching a live widget. Main thread only.
type AXAdapter struct {
	tree     *accessibility.Tree
	elements map[accessibility.NodeID]objc.ID
	// deferred holds elements whose release is waiting for the accessibility callback that dropped them to return; see
	// releaseElement.
	deferred []objc.ID
	view     View
	wnd      Window
	// inflight counts the requests being carried out inside an AppKit accessibility callback right now. It is more than
	// zero only between handing a request to AccessibilityActionCallback and that callback returning, which on this
	// platform is when a request may be carried out and a new snapshot published before AppKit is answered.
	inflight int
	// modal is the modality last given to the window, and modalKnown whether it has been given one at all; see
	// publishModal.
	modal      bool
	modalKnown bool
}

// axAdapters holds the adapter for each content view that has one. It is an ordinary map rather than a synchronized
// one because every path that reaches it — AppKit's accessibility queries, publishing a snapshot, tearing a window
// down — runs on the main thread.
var axAdapters map[View]*AXAdapter

var (
	axElementClassOnce sync.Once
	axElementClass     objc.Class
	axElementClassErr  error

	axRoleDescriptionOnce sync.Once
	axRoleDescriptionFunc func(role, subrole objc.ID) objc.ID

	axHeadingOnce             sync.Once
	axHeadingRoleValue        objc.ID
	axHeadingNeedsDescription bool

	axActionNamesOnce       sync.Once
	axActionNameIDs         []objc.ID
	axActionDescriptionFunc func(name objc.ID) objc.ID

	axShortcutAttrsOnce sync.Once
	axShortcutAttrIDs   []objc.ID

	axSelectorRulesOnce sync.Once
	axSelectorRuleMap   map[objc.SEL]func(t *accessibility.Tree, n *accessibility.Node) bool
)

// axActionNames pairs each action name the legacy action protocol deals in with the accessibility.Action it stands
// for, in the order an element advertises them. Press appears twice because VoiceOver sends AXPress for VO-Space and
// AXConfirm for the Return key, and both are a press for everything unison has. The strings are the documented values
// of the NSAccessibility*Action constants, unchanged since 10.1; they are spelled out rather than resolved through
// AppKitString because the symbol for NSAccessibilityScrollToVisibleAction was only exported by the macOS 26 SDK,
// though the action it names is as old as the others and every version of VoiceOver performs it.
var axActionNames = []struct {
	name   string
	action accessibility.Action
}{
	{name: "AXPress", action: accessibility.Press},
	{name: "AXConfirm", action: accessibility.Press},
	{name: "AXIncrement", action: accessibility.Increment},
	{name: "AXDecrement", action: accessibility.Decrement},
	{name: "AXShowMenu", action: accessibility.ShowContextMenu},
	{name: "AXScrollToVisible", action: accessibility.ScrollIntoView},
}

// NewAXAdapter returns the adapter for a content view, creating it if it does not have one yet. It returns nil for a
// nil view. Main thread only.
func NewAXAdapter(v View) *AXAdapter {
	if v == 0 {
		return nil
	}
	if a, ok := axAdapters[v]; ok {
		return a
	}
	axTraceOnce.Do(func() {
		axTraceOn = os.Getenv("UNISON_AX_TRACE") != ""
		axTraceStart = time.Now()
		if axTraceOn {
			purego.RegisterLibFunc(&axSelName, libObjC(), "sel_getName")
		}
	})
	axElementClassOnce.Do(registerAXElementClass)
	if axElementClassErr != nil {
		// Already logged by registerAXElementClass, which runs once: a failure reported again here would be written
		// once per window, and once more for every query that reached this while the same class was still missing.
		return nil
	}
	a := &AXAdapter{
		elements: make(map[accessibility.NodeID]objc.ID),
		view:     v,
		wnd:      viewWindow(objc.ID(v)),
	}
	if axAdapters == nil {
		axAdapters = make(map[View]*AXAdapter)
	}
	axAdapters[v] = a
	return a
}

// Publish installs a freshly built snapshot and tells the accessibility system what changed. The snapshot is swapped in
// before any notification is posted, so an assistive technology that queries back into the adapter while it is being
// notified — which VoiceOver routinely does — sees the tree the notification is about rather than the one before it.
// Main thread only.
func (a *AXAdapter) Publish(tree *accessibility.Tree, events []accessibility.Event) {
	if a == nil {
		return
	}
	a.tree = tree
	if tree == nil {
		return
	}
	if axTraceOn {
		kinds := make(map[string]int)
		for _, e := range events {
			kinds[e.Kind.String()]++
		}
		axTrace("publish generation=%d nodes=%d focus=%d events=%v", tree.Generation, len(tree.Nodes), tree.Focus,
			kinds)
	}
	WithPool(func() {
		a.publishModal(tree)
		a.postEvents(events)
	})
}

// publishModal tells the window whether the snapshot's root node is modal. It is the one thing about a unison modal
// dialog that AppKit cannot work out for itself: Window.RunModal runs unison's own event loop rather than an NSApp
// modal session, so no window of ours ever becomes NSApp's modal window and AXModal would otherwise read false for
// every dialog, while both other platforms report it. Unlike everything else this adapter answers, the modality is
// state held by the NSWindow rather than an answer read from the snapshot, so it is pushed rather than pulled: on the
// first publish, and afterwards whenever it changes.
func (a *AXAdapter) publishModal(tree *accessibility.Tree) {
	if a.wnd == 0 {
		return
	}
	modal := false
	if root := tree.Node(tree.Root); root != nil {
		modal = root.Modal
	}
	if a.modalKnown && a.modal == modal {
		return
	}
	a.modal = modal
	a.modalKnown = true
	if axTraceOn {
		axTrace("window modal=%v", modal)
	}
	objc.ID(a.wnd).Send(Sel("setAccessibilityModal:"), modal)
}

// Shutdown tells the accessibility system that every element the adapter handed out is gone, releases everything the
// adapter holds and forgets it. It serves both a window being destroyed and support being turned off while the window
// stays on the screen. Main thread only.
func (a *AXAdapter) Shutdown() {
	if a == nil {
		return
	}
	// A pool of its own, like Publish and AXAnnounce, because this is an entry point in its own right: it is reached
	// from a window being destroyed and from support being turned off, either of which an application may call on the
	// UI thread outside the pool an event-loop turn provides, and anything AppKit autoreleases while routing these
	// notifications would otherwise leak with the "autoreleased with no pool in place" warning.
	WithPool(func() {
		// Told to the accessibility system before anything is dropped, since the window may well still be on the
		// screen: support is being turned off while the window lives on, and an assistive technology holding one of
		// these must learn it is gone rather than go on asking it questions it can no longer answer. Every element
		// keeps its place in the map until all of them have been announced, so that a question arriving in the middle
		// of it is answered with the element it has always been answered with rather than a fresh one standing for the
		// same node.
		for _, element := range a.elements {
			NSAccessibilityPostNotification(element, AppKitString(axNotifyUIElementDestroyed))
		}
		for id, element := range a.elements {
			delete(a.elements, id)
			a.releaseElement(element)
		}
		// The adapter is forgotten only once every notification has been routed, which is the order destroyElement
		// works in. An element that has just been announced as destroyed is asked what window and parent it belonged
		// to, and axElementTarget answers that from the adapter registered for the view: unregistering first would
		// leave each of those notifications describing an element with no window, no parent and no role at all.
		delete(axAdapters, a.view)
		a.tree = nil
	})
}

// dispatch hands a request to the action callback with the adapter marked as having one in flight, and lets go of
// whatever that request stranded once it is done.
//
// The mark matters because the callback may carry the request out immediately — that is what this platform does for the
// requests VoiceOver reads the result of straight away — and so may publish a new snapshot, and drop elements, before
// AppKit has been answered. See releaseElement and the file comment.
func (a *AXAdapter) dispatch(req accessibility.ActionRequest) {
	a.inflight++
	defer func() {
		a.inflight--
		if a.inflight == 0 {
			a.drainDeferred()
		}
	}()
	AccessibilityActionCallback(a.wnd, req)
}

// releaseElement lets go of the adapter's reference to an element. While a request is in flight the release is deferred
// instead: the element may be the very one AppKit is calling a method on, and releasing it there would free it under
// AppKit's feet. See drainDeferred.
func (a *AXAdapter) releaseElement(element objc.ID) {
	if a.inflight != 0 {
		a.deferred = append(a.deferred, element)
		return
	}
	Release(element)
}

// drainDeferred lets go of the elements releaseElement held on to, by autoreleasing rather than releasing them. The
// difference is the point of the exercise: autorelease hands them to the enclosing pool, which is AppKit's own run-loop
// pool, so they are not freed until the accessibility method they were dropped underneath has returned and AppKit has
// finished with it.
func (a *AXAdapter) drainDeferred() {
	for _, element := range a.deferred {
		Autorelease(element)
	}
	a.deferred = nil
}

// Element returns the Objective-C element serving the given node, or 0 if nothing has asked about that node yet or it
// has left the tree. Elements are created on demand by the queries that need them, so this reports what the adapter is
// actually holding rather than creating anything; it exists for tests.
func (a *AXAdapter) Element(id accessibility.NodeID) objc.ID {
	if a == nil {
		return 0
	}
	return a.elements[id]
}

// AXAnnounce asks the accessibility system to speak text, for something the user should hear about that no change to a
// window expresses. The request is posted against the application rather than any window, since that is where AppKit
// looks for announcements. Main thread only.
func AXAnnounce(text string) {
	if text == "" {
		return
	}
	WithPool(func() {
		NSAccessibilityPostNotificationWithUserInfo(sharedApp(), AppKitString(axNotifyAnnouncementRequested),
			NSDictionaryFromPairs(
				AppKitString(axKeyAnnouncement), NSStringFromGo(text),
				AppKitString(axKeyPriority), NSNumberFromInt64(axPriorityHigh),
			))
	})
}

// postEvents translates the events describing one publish into NSAccessibility notifications. Every notification is
// gathered rather than posted as its event arrives, so that an element is told about each kind of change once however
// many events named it: a keystroke in a text field changes both the node's value and its text, and a numeric field's
// number besides, and all three mean one value-changed notification to an assistive technology, which re-reads what it
// needs when it arrives. Structural change is coalesced further, into a single layout-changed notification on the
// content view, and a selection change into one notification per container, since a table whose selection moved by one
// row would otherwise produce a notification per row.
func (a *AXAdapter) postEvents(events []accessibility.Event) { //nolint:gocognit // one case per event kind
	var layoutChanged []accessibility.NodeID
	var notices []axNotice
	// flush posts everything gathered so far and forgets it, in the order an assistive technology has to hear it:
	// what changed about each element first, then the one notification that says the shape of the window changed.
	flush := func() {
		a.postNotices(notices)
		notices = nil
		if len(layoutChanged) != 0 {
			a.postLayoutChanged(layoutChanged)
			layoutChanged = nil
		}
	}
	for _, e := range events {
		switch e.Kind {
		case accessibility.NodeRemoved:
			a.destroyElement(e.Node)
		case accessibility.ChildrenChanged, accessibility.NodeAdded, accessibility.BoundsChanged:
			layoutChanged = axAppendUnique(layoutChanged, e.Node)
		case accessibility.RoleChanged:
			// macOS has no role-changed notification, and the element needs no rebuilding for one: it holds nothing but
			// the content view and the node id, so its role, subrole and role description are all read from the current
			// snapshot at the moment they are asked (see axRoleFor), and it answers the same set of selectors whatever
			// the node has become. What has to be prodded is the assistive technology's own idea of the element, which
			// is what a layout-changed notification naming the node's container does: it is the one thing macOS has for
			// "what is in here is no longer what you were told". Destroying the element instead would certainly force a
			// fresh one to be fetched, but it would also pull the VoiceOver cursor out of whatever it was reading,
			// which a control that merely changed what it is does not deserve.
			layoutChanged = axAppendUnique(layoutChanged, a.tree.UnignoredParent(e.Node))
		case accessibility.FocusChanged:
			// Everything gathered so far goes out first — the per-node notices and the layout-changed notification
			// alike: an assistive technology is told where to look only once it has been told what changed, which is
			// why Diff puts this event last of all. Nothing follows it in a published set, so the flush is the whole of
			// what the publish had to say.
			flush()
			if axTraceOn {
				axTrace("notify %s node=%d", axNotifyFocusedUIElement, e.Node)
			}
			NSAccessibilityPostNotification(a.elementOrView(e.Node), AppKitString(axNotifyFocusedUIElement))
		case accessibility.NameChanged:
			// A node's name is given to the accessibility system as the element's label, which AppKit reports as
			// AXDescription (see axHasLabel), and macOS has no notification for a changed description: the
			// title-changed notification is the only one it has for "what this element is called has changed", so that
			// is what stands in for one. A client that re-reads AXTitle when it arrives finds nothing, since the
			// element deliberately does not answer accessibilityTitle — an element that answers both AXTitle and
			// AXDescription with the same string is spoken twice — so what this notification is worth is the prompt to
			// read the element again.
			notices = axAppendNotice(notices, axNotice{node: e.Node, notification: axNotifyTitleChanged})
		case accessibility.ValueChanged, accessibility.NumberChanged, accessibility.TextInserted,
			accessibility.TextDeleted, accessibility.SortChanged:
			notices = axAppendNotice(notices, axNotice{node: e.Node, notification: axNotifyValueChanged})
		case accessibility.TextSelectionChanged:
			notices = axAppendNotice(notices, axNotice{node: e.Node, notification: axNotifySelectedTextChanged})
		case accessibility.StateChanged:
			switch e.State {
			case accessibility.StateChecked, accessibility.StatePressed:
				notices = axAppendNotice(notices, axNotice{node: e.Node, notification: axNotifyValueChanged})
			case accessibility.StateSelected:
				notices = a.appendSelectionNotice(notices, e.Node)
			case accessibility.StateExpanded:
				// The two notifications macOS has for this are about outline rows, so only a row may send one. A
				// disclosure triangle, a pop-up button or a combo box opening would otherwise report a row event on an
				// element that is not a row. Nothing is posted for them: every element answers isAccessibilityExpanded
				// (AXExpanded) from the current snapshot, so an assistive technology that cares reads the new state
				// back when it next asks. A disclosure triangle is also the one of the three whose Pressed flag tracks
				// whether it is open, and that is what it reports as its value (see axNodeValue), so a change to it
				// arrives as a StatePressed event and does post a value-changed notification.
				if n := a.tree.Node(e.Node); n != nil && n.Role.IsRowLike() {
					notification := axNotifyRowCollapsed
					if n.Expanded {
						notification = axNotifyRowExpanded
					}
					notices = axAppendNotice(notices, axNotice{node: e.Node, notification: notification})
				}
			case accessibility.StateIgnored:
				// Whether a node is ignored is exactly what decides whether it appears among its parent's presented
				// children, so a node that has just become ignored — or stopped being — has changed the child list an
				// assistive technology holds for its container, which is what a layout-changed notification naming that
				// container says. A scroll bar is the one this happens to constantly: it is ignored while there is
				// nothing to scroll, so a scroll panel whose content grows past its view port gains a scroll bar in the
				// middle of a publish that changes nothing else about the panel.
				layoutChanged = axAppendUnique(layoutChanged, a.tree.UnignoredParent(e.Node))
			default:
				// Nothing macOS has a notification for. An assistive technology re-reads the element's state when it
				// next needs it. The root's Modal is the one of these the adapter acts on, and it does so on every
				// publish rather than from an event, since the window has to be told even on the first one; see
				// publishModal.
			}
		case accessibility.AttributesChanged:
			// The secondary attributes and the relations: a placeholder, a level, a row or column index or count, an
			// orientation, or one of the LabeledBy, DescribedBy and Controls links. macOS has no notification for any
			// of them, and the title-changed notification is the nearest thing it has to "read this element again" —
			// the same stand-in NameChanged uses, and for the same reason: what it is worth is the prompt, since every
			// one of these is answered from the current snapshot the moment an assistive technology asks.
			notices = axAppendNotice(notices, axNotice{node: e.Node, notification: axNotifyTitleChanged})
		case accessibility.DescriptionChanged, accessibility.WindowActivated, accessibility.WindowDeactivated:
			// NSWindow reports its own activation, and macOS has no notification for a changed help string.
		}
	}
	flush()
}

// axNotice is one notification a publish calls for, and the node it is posted against. See postNotices.
type axNotice struct {
	notification string
	node         accessibility.NodeID
	// container reports that the notification is about what a node holds rather than about the node itself, which is
	// what decides whether it is posted against a node nothing has asked about yet.
	container bool
}

// axAppendNotice records a notification, dropping one that is already there: several events of one publish routinely
// mean the same notification, from the two a keystroke produces for a text field to the one a table's whole selection
// moving produces for every row it touched, and an assistive technology re-reads what it needs when it arrives.
func axAppendNotice(notices []axNotice, notice axNotice) []axNotice {
	if slices.Contains(notices, notice) {
		return notices
	}
	return append(notices, notice)
}

// axAppendUnique records a node id, dropping one that is already there: a node that both moved and gained children
// would otherwise be named twice by the single layout-changed notification they are gathered for.
func axAppendUnique(ids []accessibility.NodeID, id accessibility.NodeID) []accessibility.NodeID {
	if slices.Contains(ids, id) {
		return ids
	}
	return append(ids, id)
}

// appendSelectionNotice records what a change to one node's Selected flag has to be reported as, dropping a notice that
// is already there: selecting a row deselects the one before it, and a table whose selection moved by one row would
// otherwise tell its container twice.
//
// Which notification it is, and what it is posted against, depends on what was selected:
//
//   - A row's selection belongs to the table, outline or list holding it, which is what
//     NSAccessibilitySelectedRowsChangedNotification says changed, and a cell's belongs to its table in the same way.
//   - Anything else whose selection state is what it reports as its value tells the accessibility system its value
//     changed. A tab is the one that matters: accessibilityValue answers whether it is selected (see axNodeValue), so
//     that is the attribute an assistive technology re-reads.
//   - Everything else reports against the container whose selected children changed.
//
// Reporting only the first of them, which is all a table asks for, leaves an assistive technology holding the AXValue
// or AXSelected from before a tab or a cell was selected until something else makes it ask again.
func (a *AXAdapter) appendSelectionNotice(notices []axNotice, id accessibility.NodeID) []axNotice {
	n := a.tree.Node(id)
	if n == nil {
		return notices
	}
	var notice axNotice
	switch {
	case n.Role.IsRowLike():
		notice = axNotice{
			node:         a.tree.UnignoredParent(id),
			notification: axNotifySelectedRowsChanged,
			container:    true,
		}
	case n.Role == role.Cell:
		notice = axNotice{
			node:         axRowContainerOf(a.tree, id),
			notification: axNotifySelectedCellsChanged,
			container:    true,
		}
	case axSelectionIsValue(n):
		notice = axNotice{node: id, notification: axNotifyValueChanged}
	default:
		notice = axNotice{
			node:         axPresentedParent(a.tree, n),
			notification: axNotifySelectedChildrenChanged,
			container:    true,
		}
	}
	return axAppendNotice(notices, notice)
}

// axSelectionIsValue reports whether a node's selection state is what it reports as its value, which is true of a tab
// and of nothing else: see axNodeValue, where a tab's value is whether it is selected.
func axSelectionIsValue(n *accessibility.Node) bool {
	return n.Role == role.Tab
}

// postNotices posts the notifications postEvents gathered, in the order the events called for them. A notice about a
// node itself is posted only if something has already asked about that node, since an assistive technology cannot be
// interested in an element it has never seen, while one about what a container holds is posted whether or not the
// container has an element yet: something looking at a selection has certainly been given the thing holding it.
func (a *AXAdapter) postNotices(notices []axNotice) {
	for _, notice := range notices {
		if !notice.container {
			a.post(notice.node, notice.notification)
			continue
		}
		if axTraceOn {
			axTrace("notify %s node=%d", notice.notification, notice.node)
		}
		NSAccessibilityPostNotification(a.elementOrView(notice.node), AppKitString(notice.notification))
	}
}

// postLayoutChanged tells the accessibility system which elements have moved, appeared or gained children. The
// notification names the elements concerned, since that is what VoiceOver uses to decide which of the frames it holds
// — above all the one under its cursor — have to be read again; a layout-changed notification with no elements in it
// leaves the cursor where the old frame was, one scroll behind. Only elements that already exist are named: an
// assistive technology cannot be holding a frame for an element it has never been given, and creating one for every
// row a scroll moved would be work nobody asked for. The root is represented by the content view.
//
// Nothing is posted when none of the named nodes has an element yet, which is what a change to a node nothing has ever
// asked about comes to: the notification would carry an empty element list, which is the very case this function's
// answer is worth nothing in.
func (a *AXAdapter) postLayoutChanged(ids []accessibility.NodeID) {
	elements := make([]objc.ID, 0, len(ids))
	for _, id := range ids {
		switch {
		case id == a.tree.Root:
			elements = append(elements, objc.ID(a.view))
		case a.elements[id] != 0:
			elements = append(elements, a.elements[id])
		}
	}
	if len(elements) == 0 {
		return
	}
	if axTraceOn {
		axTrace("notify %s nodes=%v", axNotifyLayoutChanged, ids)
	}
	NSAccessibilityPostNotificationWithUserInfo(objc.ID(a.view), AppKitString(axNotifyLayoutChanged),
		NSDictionaryFromPairs(AppKitString(axKeyUIElements), NSArrayFromIDs(elements...)))
}

// post sends a notification about one node, but only if something has actually asked about that node: an assistive
// technology cannot be interested in an element it has never seen.
func (a *AXAdapter) post(id accessibility.NodeID, notification string) {
	if element := a.elements[id]; element != 0 {
		if axTraceOn {
			axTrace("notify %s node=%d", notification, id)
		}
		NSAccessibilityPostNotification(element, AppKitString(notification))
	}
}

// destroyElement tells the accessibility system that a node's element is gone and releases it.
func (a *AXAdapter) destroyElement(id accessibility.NodeID) {
	element, ok := a.elements[id]
	if !ok {
		return
	}
	delete(a.elements, id)
	NSAccessibilityPostNotification(element, AppKitString(axNotifyUIElementDestroyed))
	a.releaseElement(element)
}

// elementFor returns the element serving a node, creating it on first use. The adapter owns the reference it returns,
// which lives until the node leaves the tree or the window is torn down.
func (a *AXAdapter) elementFor(id accessibility.NodeID) objc.ID {
	if id == 0 {
		return 0
	}
	if element, ok := a.elements[id]; ok {
		return element
	}
	if a.tree.Node(id) == nil {
		return 0
	}
	element := objc.ID(axElementClass).Send(Sel("alloc")).Send(Sel("init"))
	if element == 0 {
		return 0
	}
	element.Send(Sel("setAxView:"), objc.ID(a.view))
	element.Send(Sel("setAxNodeID:"), uint64(id))
	a.elements[id] = element
	return element
}

// elementOrView returns the element serving a node, or the content view for the root node and for "no node at all",
// since the content view is what answers for the window's root.
func (a *AXAdapter) elementOrView(id accessibility.NodeID) objc.ID {
	if id == 0 || a.tree == nil || id == a.tree.Root {
		return objc.ID(a.view)
	}
	return a.elementFor(id)
}

// elementsFor returns an autoreleased NSArray of the elements serving the given nodes. It never returns nil, since
// AppKit treats a nil children array as "ask my superclass" rather than "no children".
func (a *AXAdapter) elementsFor(ids []accessibility.NodeID) objc.ID {
	if len(ids) == 0 {
		return NSArrayFromIDs()
	}
	elements := make([]objc.ID, 0, len(ids))
	for _, id := range ids {
		if element := a.elementFor(id); element != 0 {
			elements = append(elements, element)
		}
	}
	return NSArrayFromIDs(elements...)
}

// screenRect converts a node's bounds — window-local, top-left origin, logical units — into the screen coordinates
// NSAccessibility reports frames in: into the content view's own bottom-left-origin space first, then up through the
// view and the window, which is the same path viewFirstRectForCharacterRange takes for the IME.
func (a *AXAdapter) screenRect(bounds geom.Rect) NSRect {
	v := objc.ID(a.view)
	height := objc.Send[NSRect](v, Sel("bounds")).Size.Height
	r := NSRect{
		Origin: NSPoint{X: float64(bounds.X), Y: height - float64(bounds.Bottom())},
		Size:   NSSize{Width: float64(bounds.Width), Height: float64(bounds.Height)},
	}
	r = objc.Send[NSRect](v, Sel("convertRect:toView:"), r, objc.ID(0))
	wnd := v.Send(Sel("window"))
	if wnd == 0 {
		return r
	}
	return objc.Send[NSRect](wnd, Sel("convertRectToScreen:"), r)
}

// windowPoint is the inverse of screenRect for a single point: it converts a screen location into the window-local,
// top-left origin, logical coordinates that Node.Bounds and Tree.HitTest work in. The conversion goes through
// zero-sized rects rather than points because a 32-byte NSRect is always passed in memory, which sidesteps the
// purego amd64 struct-straddle bug documented in objc_darwin.go.
func (a *AXAdapter) windowPoint(screen NSPoint) geom.Point {
	v := objc.ID(a.view)
	r := NSRect{Origin: screen}
	if wnd := v.Send(Sel("window")); wnd != 0 {
		r = objc.Send[NSRect](wnd, Sel("convertRectFromScreen:"), r)
	}
	r = objc.Send[NSRect](v, Sel("convertRect:fromView:"), r, objc.ID(0))
	height := objc.Send[NSRect](v, Sel("bounds")).Size.Height
	return geom.NewPoint(float32(r.Origin.X), float32(height-r.Origin.Y))
}

// hitTest returns the element an assistive technology should be given for a screen location, or 0 if the window has
// nothing there. The deepest node containing the point wins; if it is one of the anonymous grouping nodes an assistive
// technology is told to look past, the nearest reportable ancestor stands in for it, and the content view stands in for
// the window's root.
func (a *AXAdapter) hitTest(screen NSPoint) objc.ID {
	if a.tree == nil {
		return 0
	}
	id := a.tree.HitTest(a.windowPoint(screen))
	if id == 0 {
		return 0
	}
	if n := a.tree.Node(id); n == nil || !axReportable(n) {
		// An anonymous grouping node, or a separator, which is scaffolding rather than content: the nearest ancestor an
		// assistive technology is willing to hear about stands in for it. One step is always enough, since
		// UnignoredParent never lands on an ignored node and a separator never has children.
		id = a.tree.UnignoredParent(id)
		if id == 0 {
			return 0
		}
	}
	id = axStandIn(a.tree, id)
	return a.elementOrView(id)
}

// axReportable reports whether a node is something an assistive technology should be told about at all. A node that
// carries no information of its own is spliced out of what it sees, and a separator is decoration that would only
// get in the way of moving through a window.
func axReportable(n *accessibility.Node) bool {
	return !n.Ignored && n.Role != role.Separator
}

// axHasLabel reports whether a node's name should be given to the accessibility system as the element's label, which is
// the AXDescription an assistive technology speaks alongside the value.
//
// Two roles say their name elsewhere and would otherwise be heard twice. Static text has no name of its own — the
// snapshot puts its content in Name, since that is the accessible name every other platform wants — and reports that
// content as its value instead (see axNodeValue), matching what an NSTextField set to display text does. A disclosure
// triangle is named by its role on this platform, so a label as well would be said after it.
func axHasLabel(n *accessibility.Node) bool {
	return n.Role != role.Label && n.Role != role.DisclosureTriangle
}

// axRoleDescriptionFor returns AppKit's own description of a role and subrole pair, which is what an element that has
// nothing more specific to say reports.
func axRoleDescriptionFor(roleID, subroleID objc.ID) objc.ID {
	axRoleDescriptionOnce.Do(func() {
		purego.RegisterLibFunc(&axRoleDescriptionFunc, LoadFramework("AppKit"), "NSAccessibilityRoleDescription")
	})
	return axRoleDescriptionFunc(roleID, subroleID)
}

// axHeadingRole returns the role to report for a heading, along with whether the adapter has to supply the role
// description itself.
//
// macOS has no public heading role. Web content uses "AXHeading", and the accessibility system does understand it on
// every version this has been tried on — asking it to describe the role yields "heading" rather than echoing the role
// name back — so that is what is reported when the check passes. Where it does not, a heading falls back to static text
// with a role description of "heading", which is the most an assistive technology can be told without a role for it.
func axHeadingRole() (roleID objc.ID, needsDescription bool) {
	axHeadingOnce.Do(func() {
		// Deliberately not autoreleased: the string is consulted for the life of the process.
		candidate := NewNSString(axHeadingCandidateRole)
		honored := false
		WithPool(func() {
			desc := GoStringFromNSString(axRoleDescriptionFor(candidate, 0))
			honored = desc != "" && desc != axHeadingCandidateRole
		})
		if honored {
			axHeadingRoleValue = candidate
			return
		}
		Release(candidate)
		axHeadingRoleValue = AppKitString(axRoleStaticText)
		axHeadingNeedsDescription = true
	})
	return axHeadingRoleValue, axHeadingNeedsDescription
}

// axRoleFor returns the NSAccessibility role and subrole for a node. An empty subrole is reported as nil, which is what
// an element with no subrole answers.
func axRoleFor(t *accessibility.Tree, n *accessibility.Node) (roleID, subroleID objc.ID) { //nolint:gocognit // a table
	switch n.Role {
	case role.Window, role.Dialog:
		return AppKitString(axRoleWindow), 0
	case role.Group, role.TabPanel, role.TableHeader, role.Document:
		return AppKitString(axRoleGroup), 0
	case role.Button:
		return AppKitString(axRoleButton), 0
	case role.ToggleButton:
		return AppKitString(axRoleCheckBox), AppKitString(axSubroleToggle)
	case role.DisclosureTriangle:
		return AppKitString(axRoleDisclosureTriangle), 0
	case role.CheckBox:
		return AppKitString(axRoleCheckBox), 0
	case role.RadioButton:
		return AppKitString(axRoleRadioButton), 0
	case role.Link:
		return AppKitString(axRoleLink), 0
	case role.Label:
		return AppKitString(axRoleStaticText), 0
	case role.Heading:
		roleID, _ = axHeadingRole()
		return roleID, 0
	case role.TextField:
		if n.Protected {
			return AppKitString(axRoleTextField), AppKitString(axSubroleSecureTextField)
		}
		return AppKitString(axRoleTextField), 0
	case role.TextArea:
		return AppKitString(axRoleTextArea), 0
	case role.SpinButton:
		// The incrementor role is what carries the stepper semantics on this platform: told a spin button is a text
		// field, VoiceOver describes it as one and never mentions that its value can be stepped, even though the
		// element implements accessibilityPerformIncrement and Decrement. The element still answers the whole text
		// protocol, so the content is as readable as it was; this only adds what a plain text field cannot say. WebKit
		// maps ARIA's spinbutton the same way, and it is the counterpart of the spinner control type Windows reports
		// and the spin button role AT-SPI reports.
		return AppKitString(axRoleIncrementor), 0
	case role.ComboBox:
		return AppKitString(axRoleComboBox), 0
	case role.PopupButton:
		return AppKitString(axRolePopUpButton), 0
	case role.Slider:
		return AppKitString(axRoleSlider), 0
	case role.ProgressBar:
		return AppKitString(axRoleProgressIndicator), 0
	case role.ScrollBar:
		return AppKitString(axRoleScrollBar), 0
	case role.ScrollArea:
		return AppKitString(axRoleScrollArea), 0
	case role.List:
		return AppKitString(axRoleList), 0
	case role.ListItem:
		return AppKitString(axRoleRow), 0
	case role.Table:
		return AppKitString(axRoleTable), 0
	case role.Tree:
		return AppKitString(axRoleOutline), 0
	case role.Row:
		if parent := t.Node(t.UnignoredParent(n.ID)); parent != nil && parent.Role == role.Tree {
			return AppKitString(axRoleRow), AppKitString(axSubroleOutlineRow)
		}
		return AppKitString(axRoleRow), 0
	case role.Cell:
		return AppKitString(axRoleCell), 0
	case role.ColumnHeader:
		return AppKitString(axRoleButton), AppKitString(axSubroleSortButton)
	case role.TabList:
		return AppKitString(axRoleTabGroup), 0
	case role.Tab:
		return AppKitString(axRoleRadioButton), AppKitString(axSubroleTabButton)
	case role.MenuBar:
		return AppKitString(axRoleMenuBar), 0
	case role.Menu:
		return AppKitString(axRoleMenu), 0
	case role.MenuItem:
		return AppKitString(axRoleMenuItem), 0
	case role.Image:
		return AppKitString(axRoleImage), 0
	case role.ColorWell:
		return AppKitString(axRoleColorWell), 0
	case role.Tooltip:
		return AppKitString(axRoleHelpTag), 0
	case role.Toolbar:
		return AppKitString(axRoleToolbar), 0
	default:
		// role.Separator lands here as well, but it is never reported as an element at all, so what it says its role is
		// does not matter.
		return AppKitString(axRoleUnknown), 0
	}
}

// axElementTarget returns the adapter and node one element speaks for. Either may be nil: the element outlives the
// node whenever an assistive technology holds on to it past a publish that dropped the node, and every getter has to
// answer safely in that case rather than assume the snapshot still has it.
func axElementTarget(self objc.ID, cmd objc.SEL) (a *AXAdapter, n *accessibility.Node) {
	a, ok := axAdapters[View(self.Send(Sel("axView")))]
	if !ok || a == nil {
		return nil, nil
	}
	n = a.tree.Node(accessibility.NodeID(objc.Send[uint64](self, Sel("axNodeID"))))
	if axTraceOn && cmd != 0 {
		axTraceNode("query "+axSelName(cmd), n)
	}
	return a, n
}

// axTraceOn reports whether the trace of accessibility traffic is switched on; see axTrace.
var (
	axTraceOn    bool
	axTraceStart time.Time
	axTraceOnce  sync.Once
	axSelName    func(sel objc.SEL) string
)

// axTrace writes one line of the accessibility trace to standard error. The trace is switched on by setting the
// UNISON_AX_TRACE environment variable to anything at all, and records what an assistive technology asks, what it is
// told, what it asks to have done and what it is notified of, each line stamped with the milliseconds since the first
// adapter was made. It exists to see the order in which VoiceOver does things, which is not documented and not the same
// from one version to the next, and costs nothing unless it is on: the adapter itself only runs once an assistive
// technology is present.
func axTrace(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[ax %6d] "+format+"\n", append([]any{time.Since(axTraceStart).Milliseconds()}, args...)...)
}

// axTraceNode writes a trace line about a node, or about the absence of one.
func axTraceNode(what string, n *accessibility.Node) {
	if n == nil {
		axTrace("%s node=<gone>", what)
		return
	}
	axTrace("%s node=%d role=%s name=%q row=%d bounds=%v offscreen=%v selected=%v", what, n.ID, n.Role, n.Name,
		n.RowIndex, n.Bounds, n.Offscreen, n.Selected)
}

// axPerform hands a request from an assistive technology to the action callback, reporting whether the node advertises
// the action at all. The answer is optimistic: the request is carried out later, on the UI thread.
func axPerform(self objc.ID, action accessibility.Action) bool {
	a, n := axElementTarget(self, 0)
	if a == nil || n == nil || !n.Actions.Has(action) || AccessibilityActionCallback == nil {
		if axTraceOn {
			axTraceNode("action "+action.String()+" refused", n)
		}
		return false
	}
	if axTraceOn {
		axTraceNode("action "+action.String(), n)
	}
	a.dispatch(accessibility.ActionRequest{Node: n.ID, Action: action})
	if axTraceOn {
		axTrace("  action %s done", action)
	}
	return true
}

// axRequest hands a fully formed request from an assistive technology to the action callback.
func axRequest(self objc.ID, req accessibility.ActionRequest) {
	a, n := axElementTarget(self, 0)
	if a == nil || n == nil || AccessibilityActionCallback == nil {
		return
	}
	req.Node = n.ID
	if !n.Actions.Has(req.Action) {
		if axTraceOn {
			axTraceNode("request "+req.Action.String()+" refused", n)
		}
		return
	}
	if axTraceOn {
		axTraceNode(fmt.Sprintf("request %s value=%q number=%v start=%d end=%d", req.Action, req.Value, req.Number,
			req.Start, req.End), n)
	}
	a.dispatch(req)
	if axTraceOn {
		axTrace("  request %s done", req.Action)
	}
}

// axNodeValue returns what an assistive technology should be told a node's value is. Which of the many things a node
// might report is the value depends on its role, so the order here is the order of specificity: a state a control is in
// beats the text it holds, which beats a plain string.
func axNodeValue(n *accessibility.Node) objc.ID {
	switch {
	case n.HasCheck:
		return NSNumberFromInt64(axCheckValue(n.Checked))
	case n.Role == role.ToggleButton || n.Role == role.DisclosureTriangle:
		return NSNumberFromInt64(axBoolValue(n.Pressed))
	case n.Role == role.Tab:
		return NSNumberFromInt64(axBoolValue(n.Selected))
	case n.Role == role.Heading:
		return NSNumberFromInt64(int64(n.Level))
	case axValueIsFraction(n):
		return NSNumberFromFloat64(axFractionOf(n))
	case n.Text != nil:
		return NSStringFromGo(n.Text.Text)
	case n.HasNumber:
		return NSNumberFromFloat64(n.Number)
	case n.Value != "":
		return NSStringFromGo(n.Value)
	case n.Role == role.Label:
		// Static text reports its content as its value, not as its label, and the snapshot builder puts a label's text
		// in its name, since that is what every other platform wants.
		return NSStringFromGo(n.Name)
	default:
		return 0
	}
}

// axValueIsFraction reports whether a node's value is given to the accessibility system as a fraction of the range it
// moves in rather than as the number itself, which is what a scroll bar does and nothing else does.
//
// AXScrollBar's value is a position between 0.0 and 1.0, and an AXScrollBar carries no AXMinValue or AXMaxValue at all:
// an NSScroller over a document of any length answers 0.25 for a quarter of the way down and lists neither attribute.
// Reporting the raw offset instead tells VoiceOver a scroll bar sits at "420" of an unstated range, and a client that
// sets the value — which is how an assistive technology scrolls — sends back the fraction the convention promised,
// which as a raw offset means the very top. See axFractionOf and axDenormalize, which are the two halves of it.
func axValueIsFraction(n *accessibility.Node) bool {
	return n.Role == role.ScrollBar && n.HasNumber
}

// axFractionOf maps a node's number onto the 0..1 fraction of its range that axValueIsFraction calls for. An empty
// range — a scroll bar with nothing to scroll — sits at the start of itself.
func axFractionOf(n *accessibility.Node) float64 {
	if n.Max <= n.Min {
		return 0
	}
	return axClampFraction((n.Number - n.Min) / (n.Max - n.Min))
}

// axDenormalize maps a fraction of a node's range back onto the number the node deals in, which is the inverse of
// axFractionOf.
func axDenormalize(n *accessibility.Node, fraction float64) float64 {
	if n.Max <= n.Min {
		return n.Min
	}
	return n.Min + axClampFraction(fraction)*(n.Max-n.Min)
}

// axClampFraction confines a fraction to 0..1, leaving a NaN alone: the widgets refuse one of those for themselves, and
// turning it into a position at either end of the range here would hide what was asked for rather than refuse it.
func axClampFraction(fraction float64) float64 {
	switch {
	case fraction < 0:
		return 0
	case fraction > 1:
		return 1
	default:
		return fraction
	}
}

// axCheckValue maps a check state onto the 0/1/2 an NSAccessibility check box reports.
func axCheckValue(state check.Enum) int64 {
	switch state {
	case check.On:
		return 1
	case check.Mixed:
		return 2
	default:
		return 0
	}
}

// axBoolValue maps a boolean onto the 0/1 an NSAccessibility element reports for an on/off value.
func axBoolValue(on bool) int64 {
	if on {
		return 1
	}
	return 0
}

// axRowsOf returns the row-like children of a node, optionally only the selected ones. A nil node has none, which is
// what a row whose parent is not in the snapshot — the root itself, or a chain of ignored ancestors reaching it —
// resolves to; this runs inside an AppKit callback, where a panic is not survivable.
func axRowsOf(t *accessibility.Tree, n *accessibility.Node, selectedOnly bool) []accessibility.NodeID {
	if n == nil {
		return nil
	}
	var rows []accessibility.NodeID
	for _, id := range t.UnignoredChildren(n.ID) {
		child := t.Node(id)
		if child == nil || !child.Role.IsRowLike() {
			continue
		}
		if selectedOnly && !child.Selected {
			continue
		}
		rows = append(rows, id)
	}
	return rows
}

// axRowCountOf returns how many rows a node holds. What the widget said is preferred over what can be counted, and this
// is why both are worth having: only the rows that can be seen, plus the ones that are selected, are described, so a
// table taller than its view port holds a handful of row nodes and knows perfectly well that it has ten thousand rows.
// Counting the nodes instead would tell an assistive technology the table is as tall as its view port, while
// accessibilityIndex goes on reporting the absolute row number — "row 4,102 of 12" — so the count has to come from the
// same place the index does. Counting is the fallback for a container that reported nothing.
func axRowCountOf(t *accessibility.Tree, n *accessibility.Node) int {
	if n.RowCount > 0 {
		return n.RowCount
	}
	return len(axRowsOf(t, n, false))
}

// axColumnCountOf returns how many columns a node holds, preferring what the widget said for the same reason
// axRowCountOf does. The fallback counts the cells of the first row it has, which is the width of a row-based container
// that did not say.
func axColumnCountOf(t *accessibility.Tree, n *accessibility.Node) int {
	if n.ColumnCount > 0 {
		return n.ColumnCount
	}
	if rows := axRowsOf(t, n, false); len(rows) != 0 {
		if row := t.Node(rows[0]); row != nil {
			return len(axChildrenWithRole(t, row, role.Cell))
		}
	}
	return 0
}

// axStandIn returns the node that is presented in a node's place: for a table cell holding exactly one thing, that
// thing; for anything else, the node itself.
//
// VoiceOver moves through a table by rows and cells, and a cell is a container to it: landing on one, it reads what is
// inside and then explains how to interact with it, and pressing one, it reads the container again rather than any
// change to what is inside. A cell that holds nothing but a check box is, to a person, the check box, and cell-based
// tables have always presented it that way — the row's children are the controls themselves. So the check box stands
// in for its cell: the VoiceOver cursor lands on the control, pressing it presses the control, and the control's own
// value-changed notification is what VoiceOver announces. The cell keeps the control's place in the grid, which the
// control answers for. Other platforms are not told; a cell is what they expect.
func axStandIn(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	if n := t.Node(id); n != nil && n.Role == role.Cell {
		if children := t.UnignoredChildren(id); len(children) == 1 {
			return children[0]
		}
	}
	return id
}

// axCellStoodInFor returns the cell a node stands in for (see axStandIn), or nil if it stands in for nothing.
func axCellStoodInFor(t *accessibility.Tree, n *accessibility.Node) *accessibility.Node {
	if parent := t.Node(t.UnignoredParent(n.ID)); parent != nil && parent.Role == role.Cell &&
		axStandIn(t, parent.ID) == n.ID {
		return parent
	}
	return nil
}

// axPresentedChildren returns the children an assistive technology is shown beneath a node: its unignored children,
// with each cell that holds exactly one thing replaced by that thing.
func axPresentedChildren(t *accessibility.Tree, id accessibility.NodeID) []accessibility.NodeID {
	children := t.UnignoredChildren(id)
	for i, child := range children {
		children[i] = axStandIn(t, child)
	}
	return children
}

// axPresentedParent returns the parent an assistive technology is shown above a node: its unignored parent, unless the
// node stands in for a cell, in which case the cell's parent.
func axPresentedParent(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID {
	if cell := axCellStoodInFor(t, n); cell != nil {
		return t.UnignoredParent(cell.ID)
	}
	return t.UnignoredParent(n.ID)
}

// axVisibleRowsOf returns the row-like children of a node that are not scrolled or clipped out of view.
func axVisibleRowsOf(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
	rows := axRowsOf(t, n, false)
	visible := rows[:0]
	for _, id := range rows {
		if row := t.Node(id); row != nil && !row.Offscreen {
			visible = append(visible, id)
		}
	}
	return visible
}

// axSelectedChildrenOf returns the presented children of a node that are selected. It is what an assistive technology
// re-reads when it is told a container's selected children changed, which is how the selection of everything that is
// neither a row nor a cell — a menu item, a tab, a list of anything — is reported.
func axSelectedChildrenOf(t *accessibility.Tree, id accessibility.NodeID) []accessibility.NodeID {
	children := axPresentedChildren(t, id)
	selected := children[:0]
	for _, childID := range children {
		if child := t.Node(childID); child != nil && child.Selected {
			selected = append(selected, childID)
		}
	}
	return selected
}

// axVisibleChildrenOf returns the presented children of a node that are not scrolled or clipped out of view, which is
// what accessibilityVisibleChildren is for: the rows of a table answer accessibilityVisibleRows, and this is the same
// answer for everything that is not a row.
func axVisibleChildrenOf(t *accessibility.Tree, id accessibility.NodeID) []accessibility.NodeID {
	children := axPresentedChildren(t, id)
	visible := children[:0]
	for _, childID := range children {
		if child := t.Node(childID); child != nil && !child.Offscreen {
			visible = append(visible, childID)
		}
	}
	return visible
}

// axDisclosedRowsOf returns the rows an outline row discloses: the rows one level deeper that follow it, up to the next
// row at its own level or shallower. Rows of an outline are published as one flat list with a level apiece, the way an
// outline view orders them, so the rows a row discloses are exactly the deeper ones between it and its next sibling.
func axDisclosedRowsOf(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
	if !n.Role.IsRowLike() {
		return nil
	}
	rows := axRowsOf(t, t.Node(t.UnignoredParent(n.ID)), false)
	var disclosed []accessibility.NodeID
	for i := slices.Index(rows, n.ID) + 1; i > 0 && i < len(rows); i++ {
		row := t.Node(rows[i])
		if row == nil || row.Level <= n.Level {
			break
		}
		if row.Level == n.Level+1 {
			disclosed = append(disclosed, rows[i])
		}
	}
	return disclosed
}

// axDisclosingRowOf returns the outline row that discloses a row: the nearest row before it that is one level
// shallower. It is zero for a top-level row.
func axDisclosingRowOf(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID {
	if !n.Role.IsRowLike() || n.Level < 2 {
		return 0
	}
	rows := axRowsOf(t, t.Node(t.UnignoredParent(n.ID)), false)
	for i := slices.Index(rows, n.ID) - 1; i >= 0; i-- {
		row := t.Node(rows[i])
		if row == nil {
			return 0
		}
		if row.Level < n.Level {
			if row.Level == n.Level-1 {
				return rows[i]
			}
			return 0
		}
	}
	return 0
}

// axScrollBarOf returns the scroll bar child of a scroll area that runs in the given direction, or zero when it has
// none that can scroll.
func axScrollBarOf(t *accessibility.Tree, n *accessibility.Node, o accessibility.Orientation) accessibility.NodeID {
	if n.Role != role.ScrollArea {
		return 0
	}
	for _, id := range t.UnignoredChildren(n.ID) {
		if child := t.Node(id); child != nil && child.Role == role.ScrollBar && child.Orientation == o {
			return id
		}
	}
	return 0
}

// axContentsOf returns the children of a scroll area other than its scroll bars, which is what it scrolls. Anything
// else reports the children it is shown, since AXContents is registered on every element and an element that answered
// the raw children instead would disagree with accessibilityChildren about any container holding a single-occupant
// cell — handing out an element for a cell the presented hierarchy never mentions, which then names a parent that does
// not list it (see axStandIn).
func axContentsOf(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
	if n.Role != role.ScrollArea {
		return axPresentedChildren(t, n.ID)
	}
	children := t.UnignoredChildren(n.ID)
	contents := children[:0:0]
	for _, id := range children {
		if child := t.Node(id); child != nil && child.Role != role.ScrollBar {
			contents = append(contents, id)
		}
	}
	return contents
}

// axShortcutOf splits a node's Shortcut into the character AXMenuItemCmdChar reports and the AXMenuItemCmdModifiers
// bits that go with it, and reports whether the node names a shortcut at all.
//
// What it is given is the string the menus themselves draw, which KeyBinding.String builds as the modifier glyphs in a
// fixed order followed by the key's own name, so the glyphs are taken from the front and whatever is left is the key.
// A key whose name is a word rather than a character — Delete, Space, F1 — is reported as that word, which is what the
// menu shows and what an assistive technology can say: the two attributes that could express such a key exactly,
// AXMenuItemCmdVirtualKey and AXMenuItemCmdGlyph, name a hardware key code and a Carbon menu glyph, and a string built
// for a person to read carries neither.
func axShortcutOf(n *accessibility.Node) (char string, modifiers int64, ok bool) {
	rest := n.Shortcut
	command := false
	for rest != "" {
		ch, size := utf8.DecodeRuneInString(rest)
		bits, isCommand, isModifier := axShortcutModifier(ch)
		if !isModifier {
			break
		}
		modifiers |= bits
		command = command || isCommand
		rest = rest[size:]
	}
	if rest == "" {
		// Either no shortcut at all, or modifiers with no key to go with them, which is not an accelerator any
		// assistive technology can report.
		return "", 0, false
	}
	if !command {
		modifiers |= axMenuModifierNoCommand
	}
	return rest, modifiers, true
}

// axShortcutModifier maps one of the modifier glyphs a shortcut string starts with onto its AXMenuItemCmdModifiers
// bit, reporting separately whether it is the command key — which has no bit of its own, since its absence is what
// the no-command bit says — and whether the glyph is a modifier at all, which is what ends the run of them.
func axShortcutModifier(ch rune) (bits int64, command, isModifier bool) {
	switch ch {
	case '⌃':
		return axMenuModifierControl, false, true
	case '⌥':
		return axMenuModifierOption, false, true
	case '⇧':
		return axMenuModifierShift, false, true
	case '⌘':
		return 0, true, true
	case '⇪', '⇭':
		// Caps lock and num lock. The menus can draw them, but AXMenuItemCmdModifiers has no bit for either and
		// neither is an accelerator modifier on this platform, so they are stripped and forgotten.
		return 0, false, true
	default:
		return 0, false, false
	}
}

// axSetValue turns the value an assistive technology set on an element into a SetValue request.
//
// What arrives is examined before it is messaged. AXValue is a CFType and AXUIElementSetAttributeValue hands on
// whatever a client cares to send, so a script or an assistive technology can perfectly well set it to an
// NSAttributedString, an AXValueRef or a dictionary; sending an NSString's selectors to one of those raises an
// unrecognized-selector exception inside a Go callback, which is uncatchable and takes the process with it. Anything
// that is neither a string nor a number is dropped, which is what an element that cannot be set that way answers.
//
// A node whose value is a fraction of its own range is the one case where what arrives is not what is passed on: the
// fraction is mapped back onto the range first (see axValueIsFraction), so that a scroll bar set to 0.25 scrolls a
// quarter of the way down rather than to the offset 0.25, which would be the very top of anything longer than a pixel.
func axSetValue(self, value objc.ID) {
	if value == 0 {
		return
	}
	req := accessibility.ActionRequest{Action: accessibility.SetValue}
	isNumber := objc.Send[bool](value, Sel("isKindOfClass:"), Cls("NSNumber"))
	switch {
	case isNumber:
		req.Number = Float64FromNSNumber(value)
		req.Value = GoStringFromNSString(value.Send(Sel("stringValue")))
	case objc.Send[bool](value, Sel("isKindOfClass:"), Cls("NSString")):
		req.Value = GoStringFromNSString(value)
	default:
		if axTraceOn {
			axTrace("set value ignored: %#x is neither a string nor a number", value)
		}
		return
	}
	if _, n := axElementTarget(self, 0); n != nil && axValueIsFraction(n) {
		if !isNumber {
			// A fraction of the node's range is the only thing this value can be, and a string is not one of those.
			if axTraceOn {
				axTraceNode("set value ignored: not a fraction", n)
			}
			return
		}
		req.Number = axDenormalize(n, req.Number)
		req.Value = GoStringFromNSString(NSNumberFromFloat64(req.Number).Send(Sel("stringValue")))
	}
	axRequest(self, req)
}

// axSelectRows makes the rows in an NSArray of elements the selection of the table, outline or list they belong to. The
// schema has no request that replaces a selection wholesale, so the first row is selected outright, which clears the
// rest, and each further row is added to it; the requests are carried out later, in that order, on the UI thread.
// Elements that are not rows of this container, or that came from some other window, are ignored, as is an empty
// array, since nothing here can clear a selection. Each row is resolved against the snapshot as its turn comes, since
// selecting one may be carried out on the spot and publish a new one; the rows themselves are held alive by the NSArray
// AppKit passed in.
func axSelectRows(self, rows objc.ID) {
	a, n := axElementTarget(self, 0)
	if a == nil || n == nil || rows == 0 || AccessibilityActionCallback == nil {
		return
	}
	// AXSelectedRows is a CFType like any other attribute, so what a client sets it to is checked before it is
	// counted: asking a dictionary or an AXValueRef how many objects it holds raises an unrecognized-selector
	// exception inside a Go callback, which is uncatchable and takes the process with it.
	if !objc.Send[bool](rows, Sel("isKindOfClass:"), Cls("NSArray")) {
		if axTraceOn {
			axTraceNode(fmt.Sprintf("set selected rows ignored: %#x is not an array", rows), n)
		}
		return
	}
	if axTraceOn {
		axTraceNode(fmt.Sprintf("set selected rows (%d elements)", NSArrayCount(rows)), n)
	}
	action := accessibility.Select
	for _, element := range IDsFromNSArray(rows) {
		if !objc.Send[bool](element, Sel("isKindOfClass:"), axElementClass) {
			continue
		}
		ea, row := axElementTarget(element, 0)
		if ea != a || row == nil || !row.Role.IsRowLike() || a.tree.UnignoredParent(row.ID) != n.ID ||
			!row.Actions.Has(action) {
			continue
		}
		if axTraceOn {
			axTraceNode("  "+action.String(), row)
		}
		a.dispatch(accessibility.ActionRequest{Node: row.ID, Action: action})
		if !n.Multiselectable {
			return
		}
		action = accessibility.AddToSelection
	}
	if axTraceOn {
		axTrace("  set selected rows done")
	}
}

// axChildrenWithRole returns the unignored children of a node that have the given role.
func axChildrenWithRole(t *accessibility.Tree, n *accessibility.Node, want role.Enum) []accessibility.NodeID {
	var ids []accessibility.NodeID
	for _, id := range t.UnignoredChildren(n.ID) {
		if child := t.Node(id); child != nil && child.Role == want {
			ids = append(ids, id)
		}
	}
	return ids
}

// axMaxTreeDepth bounds how far the helpers here will walk a snapshot. Real hierarchies are orders of magnitude
// shallower, so the limit never comes into play; it exists only so that a malformed tree cannot make one of these spin
// inside an AppKit callback, where there is nothing to recover into.
const axMaxTreeDepth = 512

// axRowContainerOf returns the nearest ancestor of a node that holds rows — a table, an outline or a list — or zero
// when it has none. It is how a cell reaches the container its selection and its columns belong to, since a cell's own
// parent is the row rather than the table.
func axRowContainerOf(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	for depth := 0; depth < axMaxTreeDepth; depth++ {
		id = t.UnignoredParent(id)
		if id == 0 {
			return 0
		}
		n := t.Node(id)
		if n == nil {
			return 0
		}
		if n.Role == role.Table || n.Role == role.Tree || n.Role == role.List {
			return id
		}
	}
	return 0
}

// axTableHeaderOf returns the TableHeader node describing the columns of a table or an outline, or zero when the
// snapshot holds none.
//
// Finding it takes a search, because a unison table and its header are separate panels: the header goes into the
// column-header slot of the scroll panel whose content is the table, so it is nowhere inside the table's own subtree
// and the snapshot records no link between the two. The steps are the ones the Windows adapter takes for the same
// question, so that both platforms name the same header for the same window:
//
//  1. A TableHeader the table's Controls name, or one whose own Controls name the table. An explicit link is the right
//     answer whenever a widget records one, and is the only thing that can be right when a window is laid out oddly.
//  2. Proximity: the nearest ancestor of the table that contains exactly one TableHeader and no table but this one.
//     That ancestor is the scroll panel in every layout unison builds. Insisting that it hold only one table is what
//     stops two tables side by side from claiming each other's header; an ancestor that holds more than one ends the
//     search rather than widening it, since widening it can only make the ambiguity worse.
//  3. Nothing, which is reported as having no header.
func axTableHeaderOf(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID {
	if n == nil || (n.Role != role.Table && n.Role != role.Tree) {
		return 0
	}
	for _, id := range n.Controls {
		if header := t.Node(id); header != nil && header.Role == role.TableHeader && !header.Ignored {
			return id
		}
	}
	linked := accessibility.NodeID(0)
	t.Walk(func(candidate *accessibility.Node) bool {
		if candidate.Role != role.TableHeader || candidate.Ignored || !slices.Contains(candidate.Controls, n.ID) {
			return true
		}
		linked = candidate.ID
		return false
	})
	if linked != 0 {
		return linked
	}
	ancestor := t.UnignoredParent(n.ID)
	for depth := 0; ancestor != 0 && depth < axMaxTreeDepth; depth++ {
		headers, tables := axHeadersAndTablesWithin(t, ancestor, 0)
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

// axHeadersAndTablesWithin returns the table headers within the subtree rooted at a node, along with how many tables it
// holds. Ignored nodes are walked through — a header can sit inside a layout panel — but an ignored node is never
// reported as a header, since an assistive technology is never shown one.
func axHeadersAndTablesWithin(t *accessibility.Tree, id accessibility.NodeID, depth int,
) (headers []accessibility.NodeID, tables int) {
	n := t.Node(id)
	if n == nil || depth >= axMaxTreeDepth {
		return nil, 0
	}
	switch n.Role {
	case role.TableHeader:
		if !n.Ignored {
			headers = append(headers, id)
		}
		// A header's own subtree holds nothing but its column headers, so there is no reason to walk into it.
		return headers, 0
	case role.Table, role.Tree:
		// Likewise a table's subtree holds its rows and cells. Counting it and stopping also keeps a table nested
		// inside another table's cell from being counted twice.
		return nil, 1
	default:
	}
	for _, childID := range n.Children {
		childHeaders, childTables := axHeadersAndTablesWithin(t, childID, depth+1)
		headers = append(headers, childHeaders...)
		tables += childTables
	}
	return headers, tables
}

// axColumnHeadersOf returns the ColumnHeader nodes a table's header publishes, in the order it publishes them, or nil
// when there is no header to ask.
func axColumnHeadersOf(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
	header := axTableHeaderOf(t, n)
	if header == 0 {
		return nil
	}
	return axAppendColumnHeaders(t, nil, header, 0)
}

// axAppendColumnHeaders appends the ColumnHeader descendants of a node, in reading order. A column header holds nothing
// that is itself a column header, so the walk stops at each one it finds.
func axAppendColumnHeaders(t *accessibility.Tree, ids []accessibility.NodeID, id accessibility.NodeID, depth int,
) []accessibility.NodeID {
	if depth >= axMaxTreeDepth {
		return ids
	}
	for _, childID := range t.UnignoredChildren(id) {
		child := t.Node(childID)
		if child == nil {
			continue
		}
		if child.Role == role.ColumnHeader {
			ids = append(ids, childID)
			continue
		}
		ids = axAppendColumnHeaders(t, ids, childID, depth+1)
	}
	return ids
}

// axColumnHeadersFor returns the column headers an element reports: all of them for a table or an outline, and the one
// describing its own column for a cell — or for whatever stands in for a cell (see axStandIn). Anything else has none.
//
// A cell's header is the one whose ColumnIndex matches its own. A snapshot that never filled the headers' column
// indexes in — they would all read zero — would then answer only for the first column, so a header is taken by position
// when no index matches, which is right whenever the header publishes one element per column in column order.
func axColumnHeadersFor(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
	if n.Role == role.Table || n.Role == role.Tree {
		return axColumnHeadersOf(t, n)
	}
	if cell := axCellStoodInFor(t, n); cell != nil {
		n = cell
	}
	if n.Role != role.Cell {
		return nil
	}
	headers := axColumnHeadersOf(t, t.Node(axRowContainerOf(t, n.ID)))
	for _, id := range headers {
		if header := t.Node(id); header != nil && header.ColumnIndex == n.ColumnIndex {
			return []accessibility.NodeID{id}
		}
	}
	if n.ColumnIndex >= 0 && n.ColumnIndex < len(headers) {
		return []accessibility.NodeID{headers[n.ColumnIndex]}
	}
	return nil
}

// axSelectedCellsOf returns the selected cells of a table or an outline, with each cell that holds exactly one thing
// replaced by that thing, since that is the element an assistive technology was handed in its place (see axStandIn).
func axSelectedCellsOf(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
	var cells []accessibility.NodeID
	for _, rowID := range axRowsOf(t, n, false) {
		row := t.Node(rowID)
		if row == nil {
			continue
		}
		for _, cellID := range axChildrenWithRole(t, row, role.Cell) {
			if cell := t.Node(cellID); cell != nil && cell.Selected {
				cells = append(cells, axStandIn(t, cellID))
			}
		}
	}
	return cells
}

// registerAXElementClass registers UnisonAXElement, the NSAccessibilityElement subclass that stands in for every node
// of a published snapshot other than the root. An instance holds nothing but the content view it belongs to and the id
// of the node it speaks for; every answer it gives is read from that window's current snapshot at the moment it is
// asked, which is why an element whose node has gone away can still be messaged safely.
//
// Registration is process-global and can only happen once per class name, so it is guarded by axElementClassOnce.
func registerAXElementClass() {
	LoadAppKit()
	cls, err := objc.RegisterClass("UnisonAXElement", Cls("NSAccessibilityElement"), nil, []objc.FieldDef{
		{Name: "axView", Type: reflect.TypeFor[objc.ID](), Attribute: objc.ReadWrite},
		{Name: "axNodeID", Type: reflect.TypeFor[uint64](), Attribute: objc.ReadWrite},
	}, axElementMethods())
	if err != nil {
		axElementClassErr = errs.NewWithCause("NewAXAdapter: unable to register accessibility element class", err)
		// Logged here, inside the once, rather than by each caller: the registration is attempted a single time per
		// process, so this is the only moment there is anything new to say about it.
		errs.Log(axElementClassErr)
		return
	}
	axElementClass = cls
}

// axElementMethods returns UnisonAXElement's method table, in five groups: the attributes that describe a node, the
// states and actions, the text protocol, the accelerator a node may name, and the one method that says which of all of
// them apply to the node an element is standing for.
func axElementMethods() []objc.MethodDef {
	methods := axElementAttributeMethods()
	methods = append(methods, axElementStateMethods()...)
	methods = append(methods, axElementTextMethods()...)
	methods = append(methods, axElementShortcutMethods()...)
	return append(methods, axElementAllowedMethods()...)
}

// axElementAllowedMethods returns the override that trims what each element advertises down to what its node can
// actually do.
//
// The method table above is one flat set shared by every node of every snapshot, so without this a plain label offers
// AXIncrement and AXDecrement, a button offers the whole text protocol, a group offers AXRows and AXDisclosing, and
// every element says its value, selection and expansion can be set — the requests behind all of which the node's own
// action set then refuses (see axPerform and axRequest). isAccessibilitySelectorAllowed: is the facility AppKit
// consults to decide which attributes and actions an element really supports, including the action names it derives
// from the accessibilityPerform* methods a class implements, and answering it from the node is what makes the two ends
// agree: an assistive technology is offered exactly what it will be allowed to do.
//
// A selector axSelectorRules does not name is left to NSAccessibilityElement's own answer, which is not the same as
// "yes if the class responds to it at all": NSAccessibilityElement responds to roughly 150 setAccessibility…:
// selectors this class never overrides, and answering respondsToSelector: for those told every client that every
// attribute an element exposes is writable. A write then landed in NSAccessibilityElement's own property storage,
// was accepted, and either did nothing to the widget — VoiceOver writes AXSelectedText for braille and dictation
// input, which would appear to work — or, for the getters this class does not override, quietly changed what the
// element reports. Super refuses all of those and allows the getters, which is the answer wanted; the setters this
// class does override are all named by axSelectorRules, so they never reach super at all.
func axElementAllowedMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{
			Cmd: Sel("isAccessibilitySelectorAllowed:"),
			Fn: func(self objc.ID, _, selector objc.SEL) bool {
				allowed, restricted := axSelectorRules()[selector]
				if !restricted {
					return axSuperAllowsSelector(self, selector)
				}
				a, n := axElementTarget(self, 0)
				if a == nil || n == nil {
					// An element whose node has left the tree speaks for nothing, so it can support nothing either.
					return false
				}
				if axTraceOn {
					axTraceNode("allowed? "+axSelName(selector), n)
				}
				return allowed(a.tree, n)
			},
		},
	}
}

// axSuperAllowsSelector reports what NSAccessibilityElement's own isAccessibilitySelectorAllowed: answers for a
// selector, which is the answer an element gives for everything axSelectorRules leaves alone. The send has to go
// through SendSuper rather than to self, since self's implementation is the caller.
//
// The result is a BOOL, which the Objective-C runtime returns as a signed char in the low byte of the return
// register, so only that byte may be examined; this is the same conversion purego itself makes for a bool return.
func axSuperAllowsSelector(self objc.ID, selector objc.SEL) bool {
	return byte(SendSuper(self, axElementClass, Sel("isAccessibilitySelectorAllowed:"), selector)) != 0
}

// axSelectorRules returns the rule for each selector an element offers only when its node can back it, keyed by the
// selector. A selector that is absent is left to NSAccessibilityElement's own answer; see axElementAllowedMethods.
//
// The rules answer from the same fields the methods themselves answer from, so what is advertised and what is answered
// cannot drift apart: the performers from the node's action set, the text protocol from whether the node holds text at
// all, the row and disclosure attributes from whether the node is a container of rows or a row itself, and each setter
// from the action it turns into.
func axSelectorRules() map[objc.SEL]func(t *accessibility.Tree, n *accessibility.Node) bool {
	axSelectorRulesOnce.Do(func() {
		hasText := func(_ *accessibility.Tree, n *accessibility.Node) bool { return n.Text != nil }
		isRow := func(_ *accessibility.Tree, n *accessibility.Node) bool { return n.Role.IsRowLike() }
		can := func(actions ...accessibility.Action) func(t *accessibility.Tree, n *accessibility.Node) bool {
			return func(_ *accessibility.Tree, n *accessibility.Node) bool {
				for _, action := range actions {
					if n.Actions.Has(action) {
						return true
					}
				}
				return false
			}
		}
		axSelectorRuleMap = map[objc.SEL]func(t *accessibility.Tree, n *accessibility.Node) bool{
			// The performers, and with them the action names AppKit derives from having them at all.
			Sel("accessibilityPerformPress"):     can(accessibility.Press),
			Sel("accessibilityPerformConfirm"):   can(accessibility.Press),
			Sel("accessibilityPerformIncrement"): can(accessibility.Increment),
			Sel("accessibilityPerformDecrement"): can(accessibility.Decrement),
			Sel("accessibilityPerformShowMenu"):  can(accessibility.ShowContextMenu),
			// The text protocol, which belongs to the nodes that hold text and to no others.
			Sel("accessibilityNumberOfCharacters"):        hasText,
			Sel("accessibilitySelectedText"):              hasText,
			Sel("accessibilitySelectedTextRange"):         hasText,
			Sel("accessibilityVisibleCharacterRange"):     hasText,
			Sel("accessibilityInsertionPointLineNumber"):  hasText,
			Sel("accessibilityStringForRange:"):           hasText,
			Sel("accessibilityAttributedStringForRange:"): hasText,
			Sel("accessibilityRangeForLine:"):             hasText,
			Sel("accessibilityLineForIndex:"):             hasText,
			Sel("accessibilityFrameForRange:"):            hasText,
			Sel("accessibilityRangeForIndex:"):            hasText,
			Sel("accessibilityRangeForPosition:"):         hasText,
			Sel("setAccessibilitySelectedTextRange:"): func(_ *accessibility.Tree, n *accessibility.Node) bool {
				return n.Text != nil && n.Actions.Has(accessibility.SetTextSelection)
			},
			// The rows of a container, and the disclosure of a row.
			Sel("accessibilityRows"):             axHoldsRows,
			Sel("accessibilitySelectedRows"):     axHoldsRows,
			Sel("accessibilityVisibleRows"):      axHoldsRows,
			Sel("accessibilityDisclosedRows"):    isRow,
			Sel("accessibilityDisclosedByRow"):   isRow,
			Sel("isAccessibilityDisclosed"):      isRow,
			Sel("setAccessibilitySelectedRows:"): axHoldsRows,
			Sel("setAccessibilityDisclosed:"): func(_ *accessibility.Tree, n *accessibility.Node) bool {
				return n.Role.IsRowLike() && (n.Actions.Has(accessibility.Expand) ||
					n.Actions.Has(accessibility.Collapse))
			},
			// The settable state. A node whose value is read-only says so by refusing the setter, which is what the
			// other two platforms report as AT-SPI's READ_ONLY and UIA's IsReadOnly.
			Sel("setAccessibilityValue:"): func(_ *accessibility.Tree, n *accessibility.Node) bool {
				return !n.ReadOnly && n.Actions.Has(accessibility.SetValue)
			},
			Sel("setAccessibilitySelected:"): can(accessibility.Select, accessibility.RemoveFromSelection),
			Sel("setAccessibilityExpanded:"): can(accessibility.Expand, accessibility.Collapse),
			// Whether AXFocused can be set is the only way this platform has of saying that a node can be given the
			// keyboard focus, which is what the other two report as AT-SPI's STATE_FOCUSABLE and UIA's
			// IsKeyboardFocusable. The rule is the node's action set, because that is exactly what axPerform consults
			// before it will dispatch the request; see uiaFragmentSetFocus in internal/w32 for the same refusal.
			Sel("setAccessibilityFocused:"): can(accessibility.Focus),
		}
	})
	return axSelectorRuleMap
}

// axHoldsRows reports whether a node is a container whose contents an assistive technology reaches as rows, which is
// what decides whether it may be asked about its rows at all. A table, an outline and a list always are, whatever they
// happen to hold at the moment; anything else is one only while it really does hold rows.
func axHoldsRows(t *accessibility.Tree, n *accessibility.Node) bool {
	switch n.Role {
	case role.Table, role.Tree, role.List:
		return true
	default:
		return len(axRowsOf(t, n, false)) != 0
	}
}

// axElementShortcutMethods returns the two overrides that publish a node's Shortcut as the accelerator that activates
// it, which on this platform is AXMenuItemCmdChar and AXMenuItemCmdModifiers: the attributes VoiceOver speaks after a
// menu item's name, and the counterparts of the key binding AT-SPI reports through Action.GetKeyBinding and the
// accelerator key UIA reports through UIA_AcceleratorKeyPropertyId.
//
// They are published through the informal protocol these two methods belong to because there is nowhere else to put
// them: the NSAccessibility protocol has no property for either, they predate it, and AppKit's own NSMenuItem still
// answers them this way — one lists both among its accessibilityAttributeNames and answers each from
// accessibilityAttributeValue:. Everything else is handed to the superclass untouched, which is what goes on answering
// every other attribute of every other element.
//
// Every node that names a shortcut is reported, not just a menu item: a widget or an application may set Shortcut on
// anything through Accessibility.Callback, and these are the only attributes macOS has to say it with.
func axElementShortcutMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{
			Cmd: Sel("accessibilityAttributeNames"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				names := SendSuper(self, axElementClass, Sel("accessibilityAttributeNames"))
				_, n := axElementTarget(self, cmd)
				if n == nil {
					return names
				}
				if _, _, ok := axShortcutOf(n); !ok {
					return names
				}
				extra := NSArrayFromIDs(axShortcutAttributeStrings()...)
				if names == 0 {
					return extra
				}
				return names.Send(Sel("arrayByAddingObjectsFromArray:"), extra)
			},
		},
		{
			Cmd: Sel("accessibilityAttributeValue:"),
			Fn: func(self objc.ID, cmd objc.SEL, attribute objc.ID) objc.ID {
				name := GoStringFromNSString(attribute)
				if name != axAttrMenuItemCmdChar && name != axAttrMenuItemCmdModifiers {
					return SendSuper(self, axElementClass, Sel("accessibilityAttributeValue:"), attribute)
				}
				_, n := axElementTarget(self, cmd)
				if n == nil {
					return 0
				}
				char, modifiers, ok := axShortcutOf(n)
				if !ok {
					return 0
				}
				if name == axAttrMenuItemCmdChar {
					return NSStringFromGo(char)
				}
				return NSNumberFromInt64(modifiers)
			},
		},
	}
}

// axShortcutAttributeStrings returns the NSString for each of the two accelerator attribute names, in the order an
// element lists them. They are created once and never released, since they are handed out on every attribute-name
// query.
func axShortcutAttributeStrings() []objc.ID {
	axShortcutAttrsOnce.Do(func() {
		axShortcutAttrIDs = []objc.ID{NewNSString(axAttrMenuItemCmdChar), NewNSString(axAttrMenuItemCmdModifiers)}
	})
	return axShortcutAttrIDs
}

// axElementAttributeMethods returns the overrides that describe what a node is, what it is called, what it contains and
// where it sits.
//
//nolint:funlen // the method table is long by nature; splitting it apart would only obscure it
func axElementAttributeMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{
			Cmd: Sel("accessibilityRole"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return AppKitString(axRoleUnknown)
				}
				roleID, _ := axRoleFor(a.tree, n)
				return roleID
			},
		},
		{
			Cmd: Sel("accessibilitySubrole"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				_, subroleID := axRoleFor(a.tree, n)
				return subroleID
			},
		},
		{
			Cmd: Sel("accessibilityRoleDescription"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				if n.Role == role.Heading {
					if _, needsDescription := axHeadingRole(); needsDescription {
						return NSStringFromGo("heading")
					}
				}
				roleID, subroleID := axRoleFor(a.tree, n)
				return axRoleDescriptionFor(roleID, subroleID)
			},
		},
		{
			Cmd: Sel("accessibilityLabel"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				if _, n := axElementTarget(self, cmd); n != nil && n.Name != "" && axHasLabel(n) {
					return NSStringFromGo(n.Name)
				}
				return 0
			},
		},
		{
			Cmd: Sel("accessibilityHelp"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				if _, n := axElementTarget(self, cmd); n != nil && n.Description != "" {
					return NSStringFromGo(n.Description)
				}
				return 0
			},
		},
		{
			Cmd: Sel("accessibilityValue"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				if _, n := axElementTarget(self, cmd); n != nil {
					return axNodeValue(n)
				}
				return 0
			},
		},
		{
			// Answered by every node with a number except the ones whose value is a fraction of their own range, which
			// carry no bounds on this platform because the fraction already expresses them (see axValueIsFraction).
			Cmd: Sel("accessibilityMinValue"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				if _, n := axElementTarget(self, cmd); n != nil && n.HasNumber && !axValueIsFraction(n) {
					return NSNumberFromFloat64(n.Min)
				}
				return 0
			},
		},
		{
			Cmd: Sel("accessibilityMaxValue"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				if _, n := axElementTarget(self, cmd); n != nil && n.HasNumber && !axValueIsFraction(n) {
					return NSNumberFromFloat64(n.Max)
				}
				return 0
			},
		},
		{
			Cmd: Sel("accessibilityPlaceholderValue"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				if _, n := axElementTarget(self, cmd); n != nil && n.Placeholder != "" {
					return NSStringFromGo(n.Placeholder)
				}
				return 0
			},
		},
		{
			Cmd: Sel("accessibilityFrame"),
			Fn: func(self objc.ID, cmd objc.SEL) NSRect {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSRect{}
				}
				r := a.screenRect(n.Bounds)
				if axTraceOn {
					axTrace("  frame answer node=%d screen=(%v,%v %vx%v)", n.ID, r.Origin.X, r.Origin.Y, r.Size.Width,
						r.Size.Height)
				}
				return r
			},
		},
		{
			Cmd: Sel("accessibilityParent"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				return a.elementOrView(axPresentedParent(a.tree, n))
			},
		},
		{
			Cmd: Sel("accessibilityChildren"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axPresentedChildren(a.tree, n.ID))
			},
		},
		{
			Cmd: Sel("accessibilityWindow"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				if a, _ := axElementTarget(self, cmd); a != nil {
					return objc.ID(a.wnd)
				}
				return 0
			},
		},
		{
			Cmd: Sel("accessibilityTopLevelUIElement"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				if a, _ := axElementTarget(self, cmd); a != nil {
					return objc.ID(a.wnd)
				}
				return 0
			},
		},
		{
			Cmd: Sel("accessibilityRows"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axRowsOf(a.tree, n, false))
			},
		},
		{
			// The size of a table is asked for separately from its rows, and has to be answered separately: the rows
			// hold only what can be seen plus what is selected, while this is how many there are altogether. Without it
			// VoiceOver counts the rows it was handed and announces "row 4,102 of 12".
			Cmd: Sel("accessibilityRowCount"),
			Fn: func(self objc.ID, cmd objc.SEL) int64 {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				return int64(axRowCountOf(a.tree, n))
			},
		},
		{
			Cmd: Sel("accessibilityColumnCount"),
			Fn: func(self objc.ID, cmd objc.SEL) int64 {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				return int64(axColumnCountOf(a.tree, n))
			},
		},
		{
			Cmd: Sel("accessibilitySelectedRows"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axRowsOf(a.tree, n, true))
			},
		},
		{
			// A table's header is the group holding its column headers, and answering this is how an assistive
			// technology reaches them at all: a unison table's header is a separate panel sitting beside the table
			// rather than inside it, so nothing in the table's own subtree leads to it (see axTableHeaderOf).
			//
			// There is deliberately no accessibilityColumns beside it. macOS builds a table's column list out of column
			// objects, one element per column, and a snapshot holds none: it describes a table as rows of cells, with
			// each cell recording the column it sits in. The columns would have to be invented, and the elements
			// invented for them would answer for nothing. accessibilityColumnCount, the cells' own column index ranges
			// and the column headers below are what a client is given instead.
			Cmd: Sel("accessibilityHeader"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				return a.elementFor(axTableHeaderOf(a.tree, n))
			},
		},
		{
			// Answered by a table with all of its column headers, and by a cell with the one describing its own column,
			// which is what an assistive technology speaks alongside a cell's contents as it moves across a row.
			// Without it a cell is read as a value with nothing to say which column it came from.
			Cmd: Sel("accessibilityColumnHeaderUIElements"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axColumnHeadersFor(a.tree, n))
			},
		},
		{
			// The tables unison builds select whole rows rather than individual cells, so this is empty for all of
			// them. It is answered anyway because a cell that a widget does mark selected has no other way of being
			// reported: the selected rows name rows, and nothing else in the protocol mentions a cell.
			Cmd: Sel("accessibilitySelectedCells"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axSelectedCellsOf(a.tree, n))
			},
		},
		{
			// The selection of anything that is neither a table nor an outline: a menu's current item, a tab group's
			// tab, a list of anything that is not a row. It is the attribute
			// NSAccessibilitySelectedChildrenChangedNotification tells an assistive technology to read again, so
			// without it that notification names a container holding nothing it can find the selection in.
			Cmd: Sel("accessibilitySelectedChildren"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axSelectedChildrenOf(a.tree, n.ID))
			},
		},
		{
			// VoiceOver moves the selection of a table or outline it is interacting with by setting the selected rows,
			// not by pressing rows one at a time, so without this it has no way to move the selection at all.
			Cmd: Sel("setAccessibilitySelectedRows:"),
			Fn: func(self objc.ID, _ objc.SEL, rows objc.ID) {
				axSelectRows(self, rows)
			},
		},
		{
			Cmd: Sel("accessibilityVisibleRows"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axVisibleRowsOf(a.tree, n))
			},
		},
		{
			// The same answer for the children that are not rows. A scroll panel hands out every child it has,
			// including the ones scrolled clean out of the window, since an assistive technology that wants to reach
			// one needs it to exist; this is how it learns which of them can be seen without scrolling first.
			Cmd: Sel("accessibilityVisibleChildren"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axVisibleChildrenOf(a.tree, n.ID))
			},
		},
		{
			Cmd: Sel("accessibilityDisclosedRows"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axDisclosedRowsOf(a.tree, n))
			},
		},
		{
			Cmd: Sel("accessibilityDisclosedByRow"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				return a.elementFor(axDisclosingRowOf(a.tree, n))
			},
		},
		{
			Cmd: Sel("accessibilityRowIndexRange"),
			Fn: func(self objc.ID, cmd objc.SEL) NSRange {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSRange{}
				}
				if cell := axCellStoodInFor(a.tree, n); cell != nil {
					n = cell
				}
				if !n.Role.IsRowLike() && n.Role != role.Cell {
					return NSRange{}
				}
				return NSRange{Location: uint64(max(n.RowIndex, 0)), Length: 1}
			},
		},
		{
			Cmd: Sel("accessibilityColumnIndexRange"),
			Fn: func(self objc.ID, cmd objc.SEL) NSRange {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSRange{}
				}
				if cell := axCellStoodInFor(a.tree, n); cell != nil {
					n = cell
				}
				switch {
				case n.Role == role.Cell:
					return NSRange{Location: uint64(max(n.ColumnIndex, 0)), Length: 1}
				case n.Role.IsRowLike():
					return NSRange{Length: uint64(len(axChildrenWithRole(a.tree, n, role.Cell)))}
				default:
					return NSRange{}
				}
			},
		},
		{
			// A scroll area's scroll bars are how VoiceOver brings something it has found beyond the area's edge into
			// view, before it reads where that something is; without them it can only draw its cursor where the thing
			// was.
			Cmd: Sel("accessibilityVerticalScrollBar"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				return a.elementFor(axScrollBarOf(a.tree, n, accessibility.OrientationVertical))
			},
		},
		{
			Cmd: Sel("accessibilityHorizontalScrollBar"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				return a.elementFor(axScrollBarOf(a.tree, n, accessibility.OrientationHorizontal))
			},
		},
		{
			Cmd: Sel("accessibilityContents"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axContentsOf(a.tree, n))
			},
		},
		{
			Cmd: Sel("accessibilityIndex"),
			Fn: func(self objc.ID, cmd objc.SEL) int64 {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return 0
				}
				if n.Role.IsRowLike() {
					return int64(n.RowIndex)
				}
				if pos, _ := a.tree.PositionInSet(n.ID); pos > 0 {
					return int64(pos - 1)
				}
				return 0
			},
		},
		{
			Cmd: Sel("accessibilityTabs"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(axChildrenWithRole(a.tree, n, role.Tab))
			},
		},
		{
			Cmd: Sel("accessibilityTitleUIElement"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil || len(n.LabeledBy) == 0 {
					return 0
				}
				return a.elementFor(n.LabeledBy[0])
			},
		},
		{
			Cmd: Sel("accessibilityLinkedUIElements"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				a, n := axElementTarget(self, cmd)
				if a == nil || n == nil {
					return NSArrayFromIDs()
				}
				return a.elementsFor(n.Controls)
			},
		},
		{
			Cmd: Sel("accessibilityHitTest:"),
			Fn: func(self objc.ID, cmd objc.SEL, pt NSPoint) objc.ID {
				a, _ := axElementTarget(self, cmd)
				if a == nil {
					return 0
				}
				return a.hitTest(pt)
			},
		},
	}
}

// axElementStateMethods returns the overrides that report and change a node's state, and the ones that carry out an
// assistive technology's requests.
//
//nolint:funlen // the method table is long by nature; splitting it apart would only obscure it
func axElementStateMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{
			Cmd: Sel("isAccessibilityElement"),
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				_, n := axElementTarget(self, cmd)
				return n != nil && axReportable(n)
			},
		},
		{
			Cmd: Sel("isAccessibilityEnabled"),
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				_, n := axElementTarget(self, cmd)
				return n != nil && !n.Disabled
			},
		},
		{
			// A node scrolled or clipped entirely out of view is still part of the hierarchy and is still handed out as
			// a child, since an assistive technology that wants to reach it has to be given it before it can ask for it
			// to be scrolled into view. AXHidden is how it is told that the thing is not on the screen at the moment,
			// which is what stops VoiceOver drawing its cursor around a frame outside the window and what keeps it from
			// reading its way through the part of a list that nobody can see. It is the counterpart of the offscreen
			// property the Windows adapter reports and of the showing and visible states the AT-SPI one drops.
			Cmd: Sel("isAccessibilityHidden"),
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				_, n := axElementTarget(self, cmd)
				return n != nil && n.Offscreen
			},
		},
		{
			Cmd: Sel("isAccessibilityFocused"),
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				_, n := axElementTarget(self, cmd)
				return n != nil && n.Focused
			},
		},
		{
			Cmd: Sel("setAccessibilityFocused:"),
			Fn: func(self objc.ID, _ objc.SEL, focused bool) {
				if focused {
					axPerform(self, accessibility.Focus)
				}
			},
		},
		{
			Cmd: Sel("isAccessibilitySelected"),
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				_, n := axElementTarget(self, cmd)
				return n != nil && n.Selected
			},
		},
		{
			Cmd: Sel("setAccessibilitySelected:"),
			Fn: func(self objc.ID, _ objc.SEL, selected bool) {
				if selected {
					axPerform(self, accessibility.Select)
					return
				}
				axPerform(self, accessibility.RemoveFromSelection)
			},
		},
		{
			Cmd: Sel("isAccessibilityExpanded"),
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				_, n := axElementTarget(self, cmd)
				return n != nil && n.Expanded
			},
		},
		{
			Cmd: Sel("setAccessibilityExpanded:"),
			Fn: func(self objc.ID, _ objc.SEL, expanded bool) {
				if expanded {
					axPerform(self, accessibility.Expand)
					return
				}
				axPerform(self, accessibility.Collapse)
			},
		},
		{
			// VoiceOver opens and closes an outline row by setting whether it is disclosed.
			Cmd: Sel("setAccessibilityDisclosed:"),
			Fn: func(self objc.ID, _ objc.SEL, disclosed bool) {
				if disclosed {
					axPerform(self, accessibility.Expand)
					return
				}
				axPerform(self, accessibility.Collapse)
			},
		},
		{
			Cmd: Sel("isAccessibilityDisclosed"),
			Fn: func(self objc.ID, cmd objc.SEL) bool {
				_, n := axElementTarget(self, cmd)
				return n != nil && n.Expanded
			},
		},
		{
			Cmd: Sel("accessibilityDisclosureLevel"),
			Fn: func(self objc.ID, cmd objc.SEL) int64 {
				_, n := axElementTarget(self, cmd)
				if n == nil || n.Level < 2 {
					return 0
				}
				return int64(n.Level - 1)
			},
		},
		{
			Cmd: Sel("accessibilityOrientation"),
			Fn: func(self objc.ID, cmd objc.SEL) int64 {
				_, n := axElementTarget(self, cmd)
				if n == nil {
					return axOrientationUnknown
				}
				switch n.Orientation {
				case accessibility.OrientationHorizontal:
					return axOrientationHorizontal
				case accessibility.OrientationVertical:
					return axOrientationVertical
				default:
					return axOrientationUnknown
				}
			},
		},
		{
			Cmd: Sel("accessibilitySortDirection"),
			Fn: func(self objc.ID, cmd objc.SEL) int64 {
				_, n := axElementTarget(self, cmd)
				if n == nil {
					return axSortDirectionUnknown
				}
				switch n.Sort {
				case accessibility.SortAscending:
					return axSortDirectionAscending
				case accessibility.SortDescending:
					return axSortDirectionDescending
				default:
					return axSortDirectionUnknown
				}
			},
		},
		{
			Cmd: Sel("accessibilityPerformPress"),
			Fn:  func(self objc.ID, _ objc.SEL) bool { return axPerform(self, accessibility.Press) },
		},
		{
			// Confirm is what VoiceOver sends for the return key, which for everything unison has is the same thing
			// as a press.
			Cmd: Sel("accessibilityPerformConfirm"),
			Fn:  func(self objc.ID, _ objc.SEL) bool { return axPerform(self, accessibility.Press) },
		},
		{
			Cmd: Sel("accessibilityPerformIncrement"),
			Fn:  func(self objc.ID, _ objc.SEL) bool { return axPerform(self, accessibility.Increment) },
		},
		{
			Cmd: Sel("accessibilityPerformDecrement"),
			Fn:  func(self objc.ID, _ objc.SEL) bool { return axPerform(self, accessibility.Decrement) },
		},
		{
			Cmd: Sel("accessibilityPerformShowMenu"),
			Fn:  func(self objc.ID, _ objc.SEL) bool { return axPerform(self, accessibility.ShowContextMenu) },
		},
		{
			// Offered only by a node that advertises the SetValue action and is not ReadOnly, which is what keeps
			// VoiceOver from offering to edit the value of a progress bar, a label or a table row; see
			// axElementAllowedMethods.
			Cmd: Sel("setAccessibilityValue:"),
			Fn: func(self objc.ID, _ objc.SEL, value objc.ID) {
				axSetValue(self, value)
			},
		},
		// The legacy action protocol, alongside the accessibilityPerform* methods above. AppKit derives an element's
		// actions from which of those methods its class implements, and there is no such method for AXScrollToVisible:
		// the action VoiceOver performs on each element its cursor lands on, and the only means it has of bringing
		// that element into view, so without it moving through a list with VO-Down never scrolls the list. When an
		// element answers these three selectors as well, AppKit adds the names they give to the ones it derived rather
		// than replacing them, and delivers an action named this way through accessibilityPerformAction:. Both halves
		// come from the node's own action set — these from it directly, the derived ones through
		// isAccessibilitySelectorAllowed: (see axElementAllowedMethods) — so each action is offered by exactly the
		// nodes that can carry it out, which for scrolling into view is every one the root package describes.
		{
			Cmd: Sel("accessibilityActionNames"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				_, n := axElementTarget(self, cmd)
				if n == nil {
					return NSArrayFromIDs()
				}
				var names []objc.ID
				for i, entry := range axActionNames {
					if n.Actions.Has(entry.action) {
						names = append(names, axActionNameStrings()[i])
					}
				}
				return NSArrayFromIDs(names...)
			},
		},
		{
			Cmd: Sel("accessibilityActionDescription:"),
			Fn: func(_ objc.ID, _ objc.SEL, name objc.ID) objc.ID {
				axActionNameStrings()
				return axActionDescriptionFunc(name)
			},
		},
		{
			Cmd: Sel("accessibilityPerformAction:"),
			Fn: func(self objc.ID, _ objc.SEL, name objc.ID) {
				if action, ok := axActionForName(GoStringFromNSString(name)); ok {
					axPerform(self, action)
				} else if axTraceOn {
					axTrace("action %s unknown", GoStringFromNSString(name))
				}
			},
		},
	}
}

// axActionNameStrings returns the NSString for each entry of axActionNames, in the same order. They are created once
// and never released, since they are handed out on every accessibilityActionNames query, and the function that
// describes an action is resolved along with them.
func axActionNameStrings() []objc.ID {
	axActionNamesOnce.Do(func() {
		axActionNameIDs = make([]objc.ID, len(axActionNames))
		for i, entry := range axActionNames {
			axActionNameIDs[i] = NewNSString(entry.name)
		}
		purego.RegisterLibFunc(&axActionDescriptionFunc, LoadFramework("AppKit"), "NSAccessibilityActionDescription")
	})
	return axActionNameIDs
}

// axActionForName returns the accessibility.Action an action name of the legacy protocol stands for, and false for a
// name this adapter does not deal in.
func axActionForName(name string) (accessibility.Action, bool) {
	for _, entry := range axActionNames {
		if entry.name == name {
			return entry.action, true
		}
	}
	return 0, false
}

// axElementTextMethods returns the overrides that make a text control's content, caret, selection and line structure
// navigable. NSAccessibility works in UTF-16 code units while the snapshot works in runes, so every offset crossing
// this boundary is converted.
//
//nolint:funlen // the method table is long by nature; splitting it apart would only obscure it
func axElementTextMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{
			Cmd: Sel("accessibilityNumberOfCharacters"),
			Fn: func(self objc.ID, _ objc.SEL) int64 {
				_, _, info := axTextTarget(self)
				if info == nil {
					return 0
				}
				return int64(axUTF16Len(info.Text))
			},
		},
		{
			Cmd: Sel("accessibilitySelectedText"),
			Fn: func(self objc.ID, _ objc.SEL) objc.ID {
				_, _, info := axTextTarget(self)
				if info == nil {
					return 0
				}
				return NSStringFromGo(axRuneSlice(info.Text, info.SelStart, info.SelEnd))
			},
		},
		{
			Cmd: Sel("accessibilitySelectedTextRange"),
			Fn: func(self objc.ID, _ objc.SEL) NSRange {
				_, _, info := axTextTarget(self)
				if info == nil {
					return NSRange{}
				}
				start := axUTF16FromRune(info.Text, info.SelStart)
				end := axUTF16FromRune(info.Text, info.SelEnd)
				if end < start {
					start, end = end, start
				}
				return NSRange{Location: uint64(start), Length: uint64(end - start)}
			},
		},
		{
			Cmd: Sel("setAccessibilitySelectedTextRange:"),
			Fn: func(self objc.ID, _ objc.SEL, r NSRange) {
				_, _, info := axTextTarget(self)
				if info == nil {
					return
				}
				start := axRuneFromUTF16(info.Text, int(r.Location))
				axRequest(self, accessibility.ActionRequest{
					Action: accessibility.SetTextSelection,
					Start:  start,
					End:    axRuneFromUTF16(info.Text, int(r.Location+r.Length)),
				})
			},
		},
		{
			Cmd: Sel("accessibilityVisibleCharacterRange"),
			Fn: func(self objc.ID, _ objc.SEL) NSRange {
				_, _, info := axTextTarget(self)
				if info == nil {
					return NSRange{}
				}
				return NSRange{Length: uint64(axUTF16Len(info.Text))}
			},
		},
		{
			// The line the caret sits on, which is the end of the selection that moves as it is extended rather than
			// whichever end happens to be later (see accessibility.TextInfo.Caret). A node with no text at all has no
			// insertion point, and -1 is how AXInsertionPointLineNumber says so: answering 0 instead would have every
			// button, row and group this class serves claim a caret on its first line.
			Cmd: Sel("accessibilityInsertionPointLineNumber"),
			Fn: func(self objc.ID, _ objc.SEL) int64 {
				_, _, info := axTextTarget(self)
				if info == nil {
					return -1
				}
				return int64(axLineForRune(info, info.Caret))
			},
		},
		{
			Cmd: Sel("accessibilityStringForRange:"),
			Fn: func(self objc.ID, _ objc.SEL, r NSRange) objc.ID {
				text, ok := axTextForRange(self, r)
				if !ok {
					return 0
				}
				return NSStringFromGo(text)
			},
		},
		{
			// The same content, wrapped in an NSAttributedString. NSAccessibilityElement responds to this selector and
			// answers nil, so a text element advertises AXAttributedStringForRange and then never answers it, while
			// AXStringForRange above is answered properly — a client that asks for the attributed form, as VoiceOver
			// does when it wants to know about misspellings or links, is told the range holds nothing. The snapshot
			// carries no attributes to report, so the plain string carrying none is the whole of the right answer, and
			// the two questions agree.
			Cmd: Sel("accessibilityAttributedStringForRange:"),
			Fn: func(self objc.ID, _ objc.SEL, r NSRange) objc.ID {
				text, ok := axTextForRange(self, r)
				if !ok {
					return 0
				}
				return Autorelease(objc.ID(Cls("NSAttributedString")).Send(Sel("alloc")).Send(Sel("initWithString:"),
					NSStringFromGo(text)))
			},
		},
		{
			// A line the element does not have, and an element with no text at all, are both "there is no such range",
			// which NSAccessibility spells {NSNotFound, 0} (see emptyRange). Answering {0, 0} instead reads as a real
			// empty range at the very start of the content, which is a position rather than an absence.
			Cmd: Sel("accessibilityRangeForLine:"),
			Fn: func(self objc.ID, _ objc.SEL, line int64) NSRange {
				_, _, info := axTextTarget(self)
				if info == nil {
					return emptyRange
				}
				start, end, ok := axLineRange(info, int(line))
				if !ok {
					return emptyRange
				}
				loc := axUTF16FromRune(info.Text, start)
				return NSRange{Location: uint64(loc), Length: uint64(axUTF16FromRune(info.Text, end) - loc)}
			},
		},
		{
			// An element with no text has no lines either, and -1 is how a line number says so, exactly as
			// accessibilityInsertionPointLineNumber above says it: answering 0 would have every button, row and group
			// this class serves place the index on its first line.
			Cmd: Sel("accessibilityLineForIndex:"),
			Fn: func(self objc.ID, _ objc.SEL, index int64) int64 {
				_, _, info := axTextTarget(self)
				if info == nil {
					return -1
				}
				return int64(axLineForRune(info, axRuneFromUTF16(info.Text, int(index))))
			},
		},
		{
			Cmd: Sel("accessibilityFrameForRange:"),
			Fn: func(self objc.ID, _ objc.SEL, r NSRange) NSRect {
				a, n, info := axTextTarget(self)
				if info == nil {
					return NSRect{}
				}
				return a.screenRect(axRectForRuneRange(n, info, axRuneFromUTF16(info.Text, int(r.Location)),
					axRuneFromUTF16(info.Text, int(r.Location+r.Length))))
			},
		},
		{
			Cmd: Sel("accessibilityRangeForIndex:"),
			Fn: func(self objc.ID, _ objc.SEL, index int64) NSRange {
				_, _, info := axTextTarget(self)
				if info == nil {
					return NSRange{}
				}
				return axRangeForRune(info.Text, axRuneFromUTF16(info.Text, int(index)))
			},
		},
		{
			Cmd: Sel("accessibilityRangeForPosition:"),
			Fn: func(self objc.ID, _ objc.SEL, pt NSPoint) NSRange {
				a, n, info := axTextTarget(self)
				if info == nil {
					return NSRange{}
				}
				local := a.windowPoint(pt).Sub(n.Bounds.Point)
				return axRangeForRune(info.Text, axRuneForLocalPoint(info, local))
			},
		},
	}
}

// axTextForRange returns the content of the UTF-16 range an assistive technology asked about, along with whether the
// element holds any text at all. Both accessibilityStringForRange: and its attributed counterpart answer from it, so
// what the two report can never drift apart.
func axTextForRange(self objc.ID, r NSRange) (text string, ok bool) {
	_, _, info := axTextTarget(self)
	if info == nil {
		return "", false
	}
	return axRuneSlice(info.Text, axRuneFromUTF16(info.Text, int(r.Location)),
		axRuneFromUTF16(info.Text, int(r.Location+r.Length))), true
}

// axTextTarget returns the adapter, node and text information one element speaks for, with a nil text information for
// anything that is not a text control — or whose node has left the tree.
func axTextTarget(self objc.ID) (a *AXAdapter, n *accessibility.Node, info *accessibility.TextInfo) {
	a, n = axElementTarget(self, 0)
	if a == nil || n == nil {
		return nil, nil, nil
	}
	return a, n, n.Text
}

// axUTF16Len returns the number of UTF-16 code units the string occupies, which is the length NSAccessibility works in.
func axUTF16Len(s string) int {
	count := 0
	for _, ch := range s {
		count++
		if ch > 0xffff {
			count++
		}
	}
	return count
}

// axUTF16FromRune returns the UTF-16 offset of a rune index, clamped to the string.
func axUTF16FromRune(s string, index int) int {
	if index <= 0 {
		return 0
	}
	count := 0
	runes := 0
	for _, ch := range s {
		if runes >= index {
			return count
		}
		runes++
		count++
		if ch > 0xffff {
			count++
		}
	}
	return count
}

// axRuneFromUTF16 returns the rune index a UTF-16 offset falls at, clamped to the string. An offset that lands inside a
// surrogate pair resolves to the rune that pair encodes.
func axRuneFromUTF16(s string, offset int) int {
	if offset <= 0 {
		return 0
	}
	count := 0
	runes := 0
	for _, ch := range s {
		if count >= offset {
			return runes
		}
		size := 1
		if ch > 0xffff {
			size = 2
		}
		if count+size > offset {
			// The offset falls between the two halves of a surrogate pair, so the rune that pair encodes is the answer.
			return runes
		}
		count += size
		runes++
	}
	return runes
}

// axRuneSlice returns the substring between two rune indexes, clamping both and tolerating them being the wrong way
// around.
func axRuneSlice(s string, start, end int) string {
	if start > end {
		start, end = end, start
	}
	runes := []rune(s)
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start >= end {
		return ""
	}
	return string(runes[start:end])
}

// axRangeForRune returns the UTF-16 range covering the single character at a rune index. A rune index at or past the
// end of the content yields an empty range there, which is what an assistive technology asking about the position just
// past the last character expects.
func axRangeForRune(s string, index int) NSRange {
	runes := []rune(s)
	if index < 0 {
		index = 0
	}
	if index >= len(runes) {
		return NSRange{Location: uint64(axUTF16Len(s))}
	}
	length := 1
	if runes[index] > 0xffff {
		length = 2
	}
	return NSRange{Location: uint64(axUTF16FromRune(s, index)), Length: uint64(length)}
}

// axLineRange returns the rune range of one line. When the snapshot did not measure the lines — it does that only for
// the control holding the keyboard focus — the whole content is treated as a single line.
func axLineRange(info *accessibility.TextInfo, index int) (start, end int, ok bool) {
	if len(info.Lines) == 0 {
		if index != 0 {
			return 0, 0, false
		}
		return 0, len([]rune(info.Text)), true
	}
	if index < 0 || index >= len(info.Lines) {
		return 0, 0, false
	}
	return info.Lines[index].Start, info.Lines[index].End, true
}

// axLineForRune returns the index of the line a rune index sits on. A line owns the line feed that ends it, so a caret
// at the end of a line is reported on that line rather than at the start of the next one.
func axLineForRune(info *accessibility.TextInfo, index int) int {
	if len(info.Lines) == 0 {
		return 0
	}
	for i := range info.Lines {
		if index < info.Lines[i].End {
			return i
		}
	}
	return len(info.Lines) - 1
}

// axAdvance returns the horizontal offset of a rune boundary within a line, measured from the line's own left edge.
func axAdvance(line *accessibility.Line, index int) float32 {
	if len(line.Advances) == 0 {
		return 0
	}
	i := index - line.Start
	if i < 0 {
		i = 0
	}
	if i >= len(line.Advances) {
		i = len(line.Advances) - 1
	}
	return line.Advances[i]
}

// axRectForRuneRange returns where a rune range sits, in the same window-local, top-left origin coordinates as
// Node.Bounds. Line bounds are in the control's own coordinates, so the node's origin — which is where the control's
// coordinate system starts within the window — is added. Without measured lines the best available answer is the whole
// control.
//
// A range that spans more than one line answers with the bounding box of every line it touches, which is what
// VoiceOver draws its selection highlight from. Each line contributes only the part of itself the range covers, and
// axAdvance is what trims it: an index before a line's first rune clamps to its left edge and one past its last rune
// clamps to its right edge, so the first and last lines contribute their fragments and the lines between them
// contribute their whole width. A collapsed range still answers with the zero-width caret rectangle on its own line.
func axRectForRuneRange(n *accessibility.Node, info *accessibility.TextInfo, start, end int) geom.Rect {
	if len(info.Lines) == 0 {
		return n.Bounds
	}
	if start > end {
		start, end = end, start
	}
	first := axLineForRune(info, start)
	last := axLineForRune(info, end)
	line := &info.Lines[first]
	left := line.Bounds.X + axAdvance(line, start)
	right := line.Bounds.X + axAdvance(line, end)
	top := line.Bounds.Y
	bottom := line.Bounds.Bottom()
	for i := first + 1; i <= last; i++ {
		line = &info.Lines[i]
		left = min(left, line.Bounds.X+axAdvance(line, start))
		right = max(right, line.Bounds.X+axAdvance(line, end))
		top = min(top, line.Bounds.Y)
		bottom = max(bottom, line.Bounds.Bottom())
	}
	if right < left {
		right = left
	}
	r := geom.NewRect(left, top, right-left, bottom-top)
	r.Point = r.Point.Add(n.Bounds.Point)
	return r
}

// axRuneForLocalPoint returns the rune index nearest a point expressed in the control's own coordinates. Without
// measured lines there is nothing to search, so the caret is reported.
func axRuneForLocalPoint(info *accessibility.TextInfo, pt geom.Point) int {
	if len(info.Lines) == 0 {
		return info.Caret
	}
	index := len(info.Lines) - 1
	for i := range info.Lines {
		if pt.Y < info.Lines[i].Bounds.Bottom() {
			index = i
			break
		}
	}
	line := &info.Lines[index]
	x := pt.X - line.Bounds.X
	best := 0
	for i := range line.Advances {
		if line.Advances[i] > x {
			break
		}
		best = i
	}
	return line.Start + best
}

// axViewIsAdapted reports whether the content view already has an adapter. Nothing in the adapter asks: the view's
// three activating selectors all go through axViewAdapter, which does its own lookup. It exists for the tests, which
// use it to watch accessibility turn on — and stay off — without reaching into axAdapters themselves.
func axViewIsAdapted(v View) bool {
	_, ok := axAdapters[v]
	return ok
}

// axViewAdapter returns the adapter for a content view, asking AccessibilityActivateCallback to create one if there is
// not one yet. This is the only path that turns accessibility support on, and it is reached only from the three content
// view selectors an assistive technology has to send before it can learn anything about the window.
func axViewAdapter(v View) *AXAdapter {
	if a, ok := axAdapters[v]; ok {
		return a
	}
	if AccessibilityActivateCallback == nil {
		return nil
	}
	if !AccessibilityActivateCallback(viewWindow(objc.ID(v))) {
		return nil
	}
	return axAdapters[v]
}

// axViewChildren answers the content view's accessibilityChildren from the window's root node, or reports that the view
// has nothing to say and AppKit's own answer should stand.
func axViewChildren(v View) (children objc.ID, ok bool) {
	a := axViewAdapter(v)
	if a == nil || a.tree == nil {
		return 0, false
	}
	return a.elementsFor(axPresentedChildren(a.tree, a.tree.Root)), true
}

// axViewFocusedElement answers the content view's accessibilityFocusedUIElement. The view itself stands in when nothing
// within the window holds the focus, since the view is what represents the window's root.
func axViewFocusedElement(v View) (element objc.ID, ok bool) {
	a := axViewAdapter(v)
	if a == nil || a.tree == nil {
		return 0, false
	}
	return a.elementOrView(a.tree.Focus), true
}

// axViewHitTest answers the content view's accessibilityHitTest:.
func axViewHitTest(v View, pt NSPoint) (element objc.ID, ok bool) {
	a := axViewAdapter(v)
	if a == nil || a.tree == nil {
		return 0, false
	}
	element = a.hitTest(pt)
	return element, element != 0
}
