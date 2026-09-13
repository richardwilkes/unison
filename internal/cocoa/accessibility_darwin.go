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
	// AccessibilityActionCallback is invoked when an assistive technology asks a node to do something. It must not
	// block: the implementation is expected to hand the request to the UI thread and return.
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

	axNotifyAnnouncementRequested = "NSAccessibilityAnnouncementRequestedNotification"
	axNotifyFocusedUIElement      = "NSAccessibilityFocusedUIElementChangedNotification"
	axNotifyLayoutChanged         = "NSAccessibilityLayoutChangedNotification"
	axNotifyRowCollapsed          = "NSAccessibilityRowCollapsedNotification"
	axNotifyRowExpanded           = "NSAccessibilityRowExpandedNotification"
	axNotifySelectedRowsChanged   = "NSAccessibilitySelectedRowsChangedNotification"
	axNotifySelectedTextChanged   = "NSAccessibilitySelectedTextChangedNotification"
	axNotifyTitleChanged          = "NSAccessibilityTitleChangedNotification"
	axNotifyUIElementDestroyed    = "NSAccessibilityUIElementDestroyedNotification"
	axNotifyValueChanged          = "NSAccessibilityValueChangedNotification"

	axKeyAnnouncement = "NSAccessibilityAnnouncementKey"
	axKeyPriority     = "NSAccessibilityPriorityKey"
	axKeyUIElements   = "NSAccessibilityUIElementsKey"
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
)

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
		errs.Log(axElementClassErr)
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
	WithPool(func() { a.postEvents(events) })
}

// Shutdown tells the accessibility system that every element the adapter handed out is gone, releases everything the
// adapter holds and forgets it. It serves both a window being destroyed and support being turned off while the window
// stays on the screen. Main thread only.
func (a *AXAdapter) Shutdown() {
	if a == nil {
		return
	}
	delete(axAdapters, a.view)
	for id, element := range a.elements {
		// Told to the accessibility system before the reference is dropped, since the window may well still be on the
		// screen: support is being turned off while the window lives on, and an assistive technology holding one of
		// these must learn it is gone rather than go on asking it questions it can no longer answer.
		delete(a.elements, id)
		NSAccessibilityPostNotification(element, AppKitString(axNotifyUIElementDestroyed))
		a.releaseElement(element)
	}
	a.tree = nil
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

// postEvents translates the events describing one publish into NSAccessibility notifications. Structural change is
// coalesced into a single layout-changed notification on the content view, and a row selection change into one
// selected-rows-changed notification per container, since an assistive technology re-reads what it needs either way and
// a table whose selection moved by one row would otherwise produce a notification per row.
func (a *AXAdapter) postEvents(events []accessibility.Event) { //nolint:gocognit // one case per event kind
	var layoutChanged, selectionChanged []accessibility.NodeID
	for _, e := range events {
		switch e.Kind {
		case accessibility.NodeRemoved:
			a.destroyElement(e.Node)
		case accessibility.ChildrenChanged, accessibility.NodeAdded, accessibility.BoundsChanged:
			layoutChanged = append(layoutChanged, e.Node)
		case accessibility.FocusChanged:
			if axTraceOn {
				axTrace("notify %s node=%d", axNotifyFocusedUIElement, e.Node)
			}
			NSAccessibilityPostNotification(a.elementOrView(e.Node), AppKitString(axNotifyFocusedUIElement))
		case accessibility.NameChanged:
			a.post(e.Node, axNotifyTitleChanged)
		case accessibility.ValueChanged, accessibility.NumberChanged, accessibility.TextInserted,
			accessibility.TextDeleted, accessibility.SortChanged:
			a.post(e.Node, axNotifyValueChanged)
		case accessibility.TextSelectionChanged:
			a.post(e.Node, axNotifySelectedTextChanged)
		case accessibility.StateChanged:
			switch e.State {
			case accessibility.StateChecked, accessibility.StatePressed:
				a.post(e.Node, axNotifyValueChanged)
			case accessibility.StateSelected:
				if n := a.tree.Node(e.Node); n != nil && n.Role.IsRowLike() {
					if parent := a.tree.UnignoredParent(e.Node); !slices.Contains(selectionChanged, parent) {
						selectionChanged = append(selectionChanged, parent)
					}
				}
			case accessibility.StateExpanded:
				// The two notifications macOS has for this are about outline rows, so only a row may send one. A
				// disclosure triangle, a pop-up button or a combo box opening would otherwise report a row event on an
				// element that is not a row; each of them reports its state as its value instead, which is what an
				// assistive technology reads back when it asks.
				if n := a.tree.Node(e.Node); n != nil && n.Role.IsRowLike() {
					if n.Expanded {
						a.post(e.Node, axNotifyRowExpanded)
					} else {
						a.post(e.Node, axNotifyRowCollapsed)
					}
				}
			default:
				// Nothing macOS has a notification for. An assistive technology re-reads the element's state when it
				// next needs it.
			}
		case accessibility.Announcement:
			AXAnnounce(e.New)
		case accessibility.DescriptionChanged, accessibility.WindowActivated, accessibility.WindowDeactivated:
			// NSWindow reports its own activation, and macOS has no notification for a changed help string.
		}
	}
	for _, parent := range selectionChanged {
		if axTraceOn {
			axTrace("notify %s node=%d", axNotifySelectedRowsChanged, parent)
		}
		NSAccessibilityPostNotification(a.elementOrView(parent), AppKitString(axNotifySelectedRowsChanged))
	}
	if len(layoutChanged) != 0 {
		if axTraceOn {
			axTrace("notify %s nodes=%v", axNotifyLayoutChanged, layoutChanged)
		}
		a.postLayoutChanged(layoutChanged)
	}
}

// postLayoutChanged tells the accessibility system which elements have moved, appeared or gained children. The
// notification names the elements concerned, since that is what VoiceOver uses to decide which of the frames it holds
// — above all the one under its cursor — have to be read again; a layout-changed notification with no elements in it
// leaves the cursor where the old frame was, one scroll behind. Only elements that already exist are named: an
// assistive technology cannot be holding a frame for an element it has never been given, and creating one for every
// row a scroll moved would be work nobody asked for. The root is represented by the content view.
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
		// field, VoiceOver describes it as one and never mentions that its value can be stepped, even though the element
		// implements accessibilityPerformIncrement and Decrement. The element still answers the whole text protocol, so
		// the content is as readable as it was; this only adds what a plain text field cannot say. WebKit maps ARIA's
		// spinbutton the same way, and it is the counterpart of the spinner control type Windows reports and the spin
		// button role AT-SPI reports.
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

// axDisclosedRowsOf returns the rows an outline row discloses: the rows one level deeper that follow it, up to the next
// row at its own level or shallower. Rows of an outline are published as one flat list with a level apiece, the way an
// outline view orders them, so the rows a row discloses are exactly the deeper ones between it and its next sibling.
func axDisclosedRowsOf(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
	if !n.Role.IsRowLike() {
		return nil
	}
	rows := axRowsOf(t, t.Node(t.UnignoredParent(n.ID)), false)
	var disclosed []accessibility.NodeID
	for i := axIndexOfID(rows, n.ID) + 1; i > 0 && i < len(rows); i++ {
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
	for i := axIndexOfID(rows, n.ID) - 1; i >= 0; i-- {
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

// axContentsOf returns the children of a scroll area other than its scroll bars, which is what it scrolls.
func axContentsOf(t *accessibility.Tree, n *accessibility.Node) []accessibility.NodeID {
	children := t.UnignoredChildren(n.ID)
	if n.Role != role.ScrollArea {
		return children
	}
	contents := children[:0:0]
	for _, id := range children {
		if child := t.Node(id); child != nil && child.Role != role.ScrollBar {
			contents = append(contents, id)
		}
	}
	return contents
}

// axIndexOfID returns the position of an id within a list of ids, or -1 if it is absent.
func axIndexOfID(ids []accessibility.NodeID, id accessibility.NodeID) int {
	for i, candidate := range ids {
		if candidate == id {
			return i
		}
	}
	return -1
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
		return
	}
	axElementClass = cls
}

// axElementMethods returns UnisonAXElement's method table, in three groups: the attributes that describe a node, the
// states and actions, and the text protocol.
func axElementMethods() []objc.MethodDef {
	methods := axElementAttributeMethods()
	methods = append(methods, axElementStateMethods()...)
	return append(methods, axElementTextMethods()...)
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
			Cmd: Sel("accessibilityMinValue"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				if _, n := axElementTarget(self, cmd); n != nil && n.HasNumber {
					return NSNumberFromFloat64(n.Min)
				}
				return 0
			},
		},
		{
			Cmd: Sel("accessibilityMaxValue"),
			Fn: func(self objc.ID, cmd objc.SEL) objc.ID {
				if _, n := axElementTarget(self, cmd); n != nil && n.HasNumber {
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
			Cmd: Sel("setAccessibilityValue:"),
			Fn: func(self objc.ID, _ objc.SEL, value objc.ID) {
				req := accessibility.ActionRequest{Action: accessibility.SetValue}
				if objc.Send[bool](value, Sel("isKindOfClass:"), Cls("NSNumber")) {
					req.Number = Float64FromNSNumber(value)
					req.Value = GoStringFromNSString(value.Send(Sel("stringValue")))
				} else {
					req.Value = GoStringFromNSString(value)
				}
				axRequest(self, req)
			},
		},
	}
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
			Cmd: Sel("accessibilityInsertionPointLineNumber"),
			Fn: func(self objc.ID, _ objc.SEL) int64 {
				_, _, info := axTextTarget(self)
				if info == nil {
					return 0
				}
				return int64(axLineForRune(info, info.SelEnd))
			},
		},
		{
			Cmd: Sel("accessibilityStringForRange:"),
			Fn: func(self objc.ID, _ objc.SEL, r NSRange) objc.ID {
				_, _, info := axTextTarget(self)
				if info == nil {
					return 0
				}
				return NSStringFromGo(axRuneSlice(info.Text, axRuneFromUTF16(info.Text, int(r.Location)),
					axRuneFromUTF16(info.Text, int(r.Location+r.Length))))
			},
		},
		{
			Cmd: Sel("accessibilityRangeForLine:"),
			Fn: func(self objc.ID, _ objc.SEL, line int64) NSRange {
				_, _, info := axTextTarget(self)
				if info == nil {
					return NSRange{}
				}
				start, end, ok := axLineRange(info, int(line))
				if !ok {
					return NSRange{}
				}
				loc := axUTF16FromRune(info.Text, start)
				return NSRange{Location: uint64(loc), Length: uint64(axUTF16FromRune(info.Text, end) - loc)}
			},
		},
		{
			Cmd: Sel("accessibilityLineForIndex:"),
			Fn: func(self objc.ID, _ objc.SEL, index int64) int64 {
				_, _, info := axTextTarget(self)
				if info == nil {
					return 0
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
func axRectForRuneRange(n *accessibility.Node, info *accessibility.TextInfo, start, end int) geom.Rect {
	if len(info.Lines) == 0 {
		return n.Bounds
	}
	if start > end {
		start, end = end, start
	}
	line := &info.Lines[axLineForRune(info, start)]
	x := axAdvance(line, start)
	right := axAdvance(line, end)
	if right < x {
		right = x
	}
	r := geom.NewRect(line.Bounds.X+x, line.Bounds.Y, right-x, line.Bounds.Height)
	r.Point = r.Point.Add(n.Bounds.Point)
	return r
}

// axRuneForLocalPoint returns the rune index nearest a point expressed in the control's own coordinates. Without
// measured lines there is nothing to search, so the caret is reported.
func axRuneForLocalPoint(info *accessibility.TextInfo, pt geom.Point) int {
	if len(info.Lines) == 0 {
		return info.SelEnd
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

// axViewIsAdapted reports whether the content view already has an adapter, which is what tells the view's three
// activating selectors whether they have anything to answer from yet.
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
