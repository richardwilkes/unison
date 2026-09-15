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
	"github.com/richardwilkes/unison/accessibility"
)

// This file is the only place in the package that tells UI Automation something. Everywhere else answers a question a
// client asked; these calls start the conversation, and that difference matters in three ways:
//
//   - They run on the UI thread, inside Publish, while a client's questions arrive on whichever thread UI Automation
//     likes. That is why Publish installs the new snapshot before anything is raised: the first thing a client does
//     with an event is ask about the element it names, and it must not be answered from the state the event describes
//     leaving behind. The exception is raiseEvent, which a pattern method that has to report its own outcome calls on
//     whichever thread it was itself called on; UI Automation's raise functions are free-threaded, as the providers
//     here are.
//   - They cost something even when nobody is listening, so every one of them sits behind UiaClientsAreListening. A
//     window whose provider was created by something other than a screen reader — an inspection tool, the touch
//     keyboard — raises nothing at all.
//   - The VARIANTs and BSTRs built for them are ours. A VARIANT handed back from a provider method belongs to UI
//     Automation Core, which clears it; one passed into a UiaRaise* function is copied by the call and stays ours, so
//     every one here is paired with a deferred Clear or Free.
//
// Which calls one publish makes is decided by UIADecideRaises, which is a pure function of the two snapshots and the
// events between them and is tested on any platform. This file does nothing but make them.

// The UI Automation entry points the adapter calls, held in variables so that the Windows tests can record what would
// have been raised instead of needing a live UI Automation client, which no unit test can arrange. The last one is not
// a raise at all but the one call that asks UI Automation for something — the host provider a fragment root reports —
// and is here for the same reason: a test has no window for it to answer about. Nothing but a test ever replaces them,
// and each is restored when that test finishes.
var (
	uiaClientsAreListening                 = UiaClientsAreListening
	uiaRaiseAutomationEvent                = UiaRaiseAutomationEvent
	uiaRaiseAutomationPropertyChangedEvent = UiaRaiseAutomationPropertyChangedEvent
	uiaRaiseStructureChangedEvent          = UiaRaiseStructureChangedEvent
	uiaRaiseNotificationEvent              = UiaRaiseNotificationEvent
	uiaDisconnectProvider                  = UiaDisconnectProvider
	uiaReturnRawElementProvider            = UiaReturnRawElementProvider
	uiaHostProviderFromHwnd                = UiaHostProviderFromHwnd
)

// uiaAnnouncementActivity names the activity every announcement this toolkit makes belongs to. A client that groups or
// suppresses notifications does so by activity, and all of ours are the application speaking rather than some
// long-running operation with an identity of its own.
const uiaAnnouncementActivity = "unison"

// raiseEvents tells UI Automation what one publish changed. It is called from Publish with the snapshot that was
// replaced, the one that replaced it, and the events between them as derived by accessibility.Diff.
//
// Nothing is raised unless a client is listening, and the question is not even asked for a publish with no events
// behind it — which is most of them, since a window republishes its snapshot after every redraw. The exception is the
// first publish, identified by there being no previous snapshot: the window reports that it opened then, and no event
// stands behind that.
func (w *UIAWindow) raiseEvents(old, cur *accessibility.Tree, events []accessibility.Event) {
	if len(events) == 0 && old != nil {
		return
	}
	if !uiaClientsAreListening() {
		return
	}
	for _, raise := range UIADecideRaises(old, cur, events) {
		w.raiseOne(old, cur, raise)
	}
}

// raiseOne makes one of the calls UIADecideRaises asked for. The provider it names is created here if nothing has
// needed it before, which is how a window whose client only listens for events comes to have providers at all; a node
// with no provider — one that has left the tree, or that the snapshot ignores — is skipped rather than reported on.
func (w *UIAWindow) raiseOne(old, cur *accessibility.Tree, raise UIARaise) {
	if raise.Kind == UIARaiseDisconnect {
		// Retiring a provider is the provider map's business rather than an event, and has to happen whether or not
		// anyone was listening, so Publish does it immediately after this rather than here. That is also the order the
		// decision asked for: the removal is reported while the departing provider still exists to be disconnected.
		return
	}
	p := w.Provider(raise.Node)
	if p == nil {
		return
	}
	defer p.release()
	switch raise.Kind {
	case UIARaiseEvent:
		uiaRaiseAutomationEvent(p.Unknown(), raise.Event)
	case UIARaiseProperty:
		p.raisePropertyChanged(old, cur, raise.Property)
	case UIARaiseStructure:
		uiaRaiseStructureChangedEvent(p.Unknown(), raise.Change, uiaStructureRuntimeID(raise))
	default:
	}
}

// Announce asks any listening assistive technology to speak a piece of text that no change in the tree accounts for: a
// background task finished, a search found nothing, a shortcut did something with no visible result. The root package
// calls it for AnnounceForAccessibility.
//
// It is a notification event on the fragment root rather than a change to any element, so nothing about the snapshot
// changes and nothing is remembered — a client that is not listening at this moment never hears it. Saying the same
// thing twice really does say it twice, which is what an application asking twice means.
func (w *UIAWindow) Announce(text string) {
	if text == "" || !uiaClientsAreListening() {
		return
	}
	if root := w.Root(); root != nil {
		defer root.release()
		root.raiseNotification(text)
	}
}

// raiseEvent tells listening clients that something happened to this provider's element. Everything else in this file
// is reached from a publish, where UIADecideRaises has already decided what to say; this is for the one pattern method
// that has to report its own outcome, IInvokeProvider::Invoke, whose event no change to the snapshot need follow.
func (p *UIAProvider) raiseEvent(eventID EventID) {
	if !uiaClientsAreListening() {
		return
	}
	uiaRaiseAutomationEvent(p.Unknown(), eventID)
}

// uiaStructureRuntimeID returns the runtime identifier a structure change carries: the added or removed child's, since
// a removal is reported on a parent that has to say which child left. An invalidation names no child and carries none.
func uiaStructureRuntimeID(raise UIARaise) []int32 {
	if raise.Child == 0 {
		return nil
	}
	return UIARuntimeID(raise.Child)
}

// raisePropertyChanged reports one property of this provider's node as having changed, with the value it had in the
// previous snapshot and the value it has in the current one. Both VARIANTs are ours: the call copies them, so what UI
// Automation ends up holding is its own, and whatever ours own — a BSTR, a SAFEARRAY — is released here.
//
// A node the previous snapshot did not hold has no old value, and an empty VARIANT is how that is said. A client uses
// the old value only to phrase what it says ("changed from 3 to 4") and falls back to the new value alone.
func (p *UIAProvider) raisePropertyChanged(old, cur *accessibility.Tree, propertyID PropertyID) {
	var oldValue, newValue VARIANT
	defer oldValue.Clear()
	defer newValue.Clear()
	p.raisedPropertyValue(old, propertyID, &oldValue)
	p.raisedPropertyValue(cur, propertyID, &newValue)
	uiaRaiseAutomationPropertyChangedEvent(p.Unknown(), propertyID, oldValue, newValue)
}

// raisedPropertyValue fills in the VARIANT holding one property of this provider's node as of one snapshot, leaving it
// empty when that snapshot does not hold the node at all, or holds it without the pattern the property belongs to.
//
// The properties a control pattern answers — the value, the toggle state, whether the value may be changed — are filled
// in here, because a client reads them through the pattern interface rather than through GetPropertyValue and so
// propertyValue has nothing to say about them. Everything else defers to propertyValue, so that the value a client is
// told about and the value it reads back cannot disagree.
//
// UIAReportsProperty is what keeps that promise for the pattern properties, which have no GetPropertyValue answer to
// agree with: the two snapshots a change is reported between need not both support the pattern, since a state flag can
// take one away, and the side that does not support it has nothing to report. Filling it in anyway would hand a client
// a concrete value for a pattern the element does not implement, which is the one thing the control pattern properties
// must never do. UIADecideRaises no longer raises a pattern's property on an element that has lost the pattern — the
// loss goes out as the pattern's availability property instead — so what reaches here is the granting direction, where
// it is the previous snapshot that has nothing to say.
//
// The availability properties themselves are answered here too, from the patterns the snapshot's node hands out. They
// are the one kind of pattern property every element answers, false being the answer that says the pattern is gone.
func (p *UIAProvider) raisedPropertyValue(tree *accessibility.Tree, propertyID PropertyID, value *VARIANT) {
	node := tree.Node(p.node)
	if node == nil || !UIAReportsProperty(tree, node, propertyID) {
		return
	}
	if pattern := UIAAvailabilityPattern(propertyID); pattern != 0 {
		value.SetBool(UIAProvidesPattern(tree, node, pattern))
		return
	}
	switch propertyID {
	case UIA_ValueValuePropertyId:
		// Always a BSTR, even for an empty value, which is what IValueProvider::get_Value answers with.
		value.SetBSTR(UIAValueString(node))
	case UIA_RangeValueValuePropertyId:
		value.SetR8(node.Number)
	case UIA_ValueIsReadOnlyPropertyId:
		value.SetBool(UIAIsValueReadOnly(node))
	case UIA_RangeValueIsReadOnlyPropertyId:
		value.SetBool(UIAIsRangeValueReadOnly(node))
	case UIA_ToggleToggleStatePropertyId:
		value.SetI4(int32(UIAToggleState(node)))
	case UIA_ExpandCollapseExpandCollapseStatePropertyId:
		value.SetI4(int32(UIAExpandCollapseState(node)))
	case UIA_SelectionItemIsSelectedPropertyId:
		value.SetBool(UIAIsSelected(node))
	case UIA_SelectionCanSelectMultiplePropertyId:
		// The Selection pattern's, read through ISelectionProvider::get_CanSelectMultiple rather than through
		// GetPropertyValue, so propertyValue has nothing to say about it either.
		value.SetBool(node.Multiselectable)
	case UIA_WindowIsModalPropertyId:
		// The Window pattern's, so it is answered here rather than by propertyValue, which a client never reads it
		// through: it asks IWindowProvider::get_IsModal instead. Reaching this means UIAReportsProperty has already
		// established that this element hands that pattern out, which the fragment root alone does — a dialog-shaped
		// panel is refused IWindowProvider, so it is refused a modality to go with it.
		value.SetBool(node.Modal)
	case UIA_BoundingRectanglePropertyId:
		p.setBoundingRectangle(value, node)
	default:
		p.propertyValue(tree, node, propertyID, value)
	}
}

// setBoundingRectangle stores a node's screen rectangle in the shape UI Automation reports the bounding rectangle
// property in: an array of four doubles, rather than the UiaRect that
// IRawElementProviderFragment::get_BoundingRectangle answers with. An element with no meaningful rectangle — one
// scrolled or clipped out of view — leaves the VARIANT empty, which is what pairs with the IsOffscreen property.
//
// The rectangle is always converted through the window's current geometry, including the one reported as the old value:
// the geometry a window had before it moved is not kept, and a client uses the old rectangle to erase a highlight
// rather than to describe anything.
func (p *UIAProvider) setBoundingRectangle(value *VARIANT, node *accessibility.Node) {
	if node.Offscreen || node.Bounds.Empty() {
		return
	}
	left, top, width, height := p.window.Geometry().ScreenRect(node.Bounds)
	if array := NewSafeArrayFloat64([]float64{left, top, width, height}); array != 0 {
		value.SetArray(VT_R8, array)
	}
}

// raiseNotification asks a client to speak one piece of text on this provider's behalf. Both BSTRs stay ours, so both
// are freed once the call returns; the export itself is absent before Windows 10 1709, in which case the function
// reports that and nothing is announced.
func (p *UIAProvider) raiseNotification(text string) {
	display := NewBSTR(text)
	defer display.Free()
	activity := NewBSTR(uiaAnnouncementActivity)
	defer activity.Free()
	uiaRaiseNotificationEvent(p.Unknown(), NotificationKind_Other, NotificationProcessing_All, display, activity)
}
