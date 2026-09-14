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
	"sync/atomic"
	"unsafe"

	"github.com/richardwilkes/unison/accessibility"
	"golang.org/x/sys/windows"
)

// This file holds the per-window half of the UI Automation adapter: the snapshot every provider answers from, the
// window's geometry, the providers themselves, and the two moments that change all of it — a new snapshot being
// published and the window going away.
//
// One UIAWindow serves one HWND, and is created the first time that window is asked for a UI Automation provider. It
// belongs to no thread: the UI thread publishes snapshots and updates the geometry, UI Automation asks for providers on
// whichever thread it likes, and the two meet only through the atomic snapshot pointer and the mutex over the provider
// map.

// UIAConfig holds what the UI Automation adapter for a window needs from the window itself.
type UIAConfig struct {
	// Action receives every request an assistive technology makes of a node in this window: give this the focus, press
	// it, set its value. It is called on whichever thread UI Automation called in on, so the root package's
	// implementation queues the work onto the UI thread and returns at once rather than doing it here. A nil Action
	// makes every such request fail, which is the honest answer for a window that cannot act on them.
	Action func(request accessibility.ActionRequest)
	// HWND is the window whose content area the fragment covers. It is reported as the root's native window handle and
	// is what the root's host provider is obtained from.
	HWND windows.HWND
}

// UIAWindow is the UI Automation adapter for one window: the fragment root, the providers for the nodes beneath it, and
// the snapshot they all answer from.
type UIAWindow struct {
	tree      atomic.Pointer[accessibility.Tree]
	geometry  atomic.Pointer[UIAGeometry]
	action    func(request accessibility.ActionRequest)
	providers map[accessibility.NodeID]*UIAProvider
	root      *UIAProvider
	rootNode  accessibility.NodeID
	hwnd      windows.HWND
	lock      sync.Mutex
	listeners atomic.Int32
}

// NewUIAWindow creates the UI Automation adapter for one window, with the snapshot its providers will answer from and
// the geometry that turns the snapshot's coordinates into screen coordinates. The fragment root is created here, since
// answering the WM_GETOBJECT that led to this needs it; every other provider is created when something first asks about
// its node.
//
// Creating the adapter is the window's first publish, so it is also where the window reports that it opened — which is
// how a screen reader knows to read a dialog out: see raiseEvents, which is given no previous snapshot to compare
// against here.
//
// A nil snapshot leaves the adapter with no fragment root — Root and RootUnknown answer nil, and nothing can be raised
// on it — which is not a state the root package puts it in: a window is never asked for a provider before it has a
// snapshot.
func NewUIAWindow(cfg UIAConfig, tree *accessibility.Tree, geometry UIAGeometry) *UIAWindow {
	w := &UIAWindow{
		action:    cfg.Action,
		providers: make(map[accessibility.NodeID]*UIAProvider),
		hwnd:      cfg.HWND,
	}
	w.geometry.Store(&geometry)
	if tree != nil {
		w.rootNode = tree.Root
		w.tree.Store(tree)
	}
	w.root = w.Provider(w.rootNode)
	if w.root != nil {
		// The reference Provider handed over is given straight back: the provider map's own reference covers this field
		// too, because Destroy clears the field before it gives that one up, so the field can never outlive it.
		w.root.release()
	}
	w.raiseEvents(nil, tree, nil)
	return w
}

// HWND returns the window handle the fragment covers.
func (w *UIAWindow) HWND() windows.HWND {
	return w.hwnd
}

// Tree returns the snapshot every provider currently answers from. It is replaced wholesale by Publish, never modified,
// so a caller that reads it once has a consistent view for as long as it holds the pointer.
func (w *UIAWindow) Tree() *accessibility.Tree {
	return w.tree.Load()
}

// Geometry returns where the window's content area sits on screen and how large its logical units are.
func (w *UIAWindow) Geometry() UIAGeometry {
	if geometry := w.geometry.Load(); geometry != nil {
		return *geometry
	}
	return UIAGeometry{}
}

// SetGeometry records where the window's content area now sits on screen. The root package calls it when the window is
// moved or resized and when its display scale changes, since a bounding rectangle answered from a stale origin points a
// screen reader's highlight at the wrong part of the screen.
func (w *UIAWindow) SetGeometry(geometry UIAGeometry) {
	w.geometry.Store(&geometry)
}

// Listeners reports how many event listeners UI Automation has advised this window's fragment root of. It is for
// diagnostics and tests only: whether to raise an event is decided by UiaClientsAreListening, which knows about clients
// this fragment was never told about.
func (w *UIAWindow) Listeners() int32 {
	return w.listeners.Load()
}

// Root returns the window's fragment root provider, or nil once the window has been destroyed. The reference that comes
// back is the caller's to release, for the reason Provider gives.
func (w *UIAWindow) Root() *UIAProvider {
	w.lock.Lock()
	defer w.lock.Unlock()
	if w.root == nil {
		return nil
	}
	w.root.addRef()
	return w.root
}

// RootUnknown returns the fragment root's IRawElementProviderSimple pointer, which is what
// UiaReturnRawElementProvider answers a WM_GETOBJECT with, or nil once the window has been destroyed. The pointer is
// unowned: UiaReturnRawElementProvider takes a reference of its own, and until it does, the provider map's reference
// keeps the root alive. That is safe only because both this and Destroy run on the UI thread, so the root cannot be
// given up between the two.
func (w *UIAWindow) RootUnknown() unsafe.Pointer {
	root := w.Root()
	if root == nil {
		return nil
	}
	defer root.release()
	return root.Unknown()
}

// Provider returns the provider for one node, creating it if this is the first time anything has asked. It may be
// called from any thread.
//
// The reference that comes back is the caller's, and must be released once the caller is done with the pointer — with
// release, or by handing it to UI Automation as the out-parameter of a method that transfers ownership. Handing back an
// unowned pointer would be a use-after-free waiting to happen: the reference taken here is taken while the provider map
// still holds one, and a caller that instead took its own after the lock was dropped could find that a publish had
// retired the provider in between, taking the count to zero, unpinning the object and leaving the caller to resurrect
// memory Go no longer keeps alive.
//
// It returns nil for the zero id, for a node the current snapshot does not hold, for a node the snapshot marks Ignored
// — such a node is spliced out of the tree a client sees, so nothing can ask about it — and for every node once the
// window has been destroyed. A provider that already exists is returned even after its node leaves the tree, up until
// the publish that retires it.
func (w *UIAWindow) Provider(id accessibility.NodeID) *UIAProvider {
	if id == 0 {
		return nil
	}
	w.lock.Lock()
	defer w.lock.Unlock()
	if w.providers == nil {
		return nil
	}
	if p := w.providers[id]; p != nil {
		p.addRef()
		return p
	}
	node := w.Tree().Node(id)
	if node == nil || node.Ignored {
		return nil
	}
	p := newUIAProvider(w, id)
	w.providers[id] = p
	// One reference for the map, which newUIAProvider left behind, and one for the caller.
	p.addRef()
	return p
}

// Publish installs a new snapshot, tells UI Automation what changed, and retires the providers of the nodes that have
// left the tree. events describes how the new snapshot differs from the old one, as derived by accessibility.Diff.
//
// The order is the whole of the contract with a client. The snapshot is installed first, so that a client answering an
// event it is about to be told about sees the new state rather than the old; the events go out next, while every
// provider they can name still exists; and the providers of the departed nodes are let go last, since a removal is
// reported on the parent while the child is still there to be disconnected. See raiseEvents in uia_events_windows.go.
func (w *UIAWindow) Publish(tree *accessibility.Tree, events []accessibility.Event) {
	if tree == nil {
		return
	}
	old := w.tree.Swap(tree)
	w.raiseEvents(old, tree, events)
	w.retireRemovedProviders(tree)
}

// retireRemovedProviders retires the provider of every node the new snapshot no longer holds, or that it now marks
// Ignored, which is the same thing to a client. Retiring means marking the provider stale so that it answers
// UIA_E_ELEMENTNOTAVAILABLE, telling UI Automation to drop the references it holds to it, and giving up the reference
// the provider map held; the provider itself lives on until any client holding it releases it too.
//
// The fragment root is never retired here. It exists for as long as the window does, and Destroy is what ends it.
func (w *UIAWindow) retireRemovedProviders(tree *accessibility.Tree) {
	var retired []*UIAProvider
	w.lock.Lock()
	for id, p := range w.providers {
		if id == w.rootNode {
			continue
		}
		if node := tree.Node(id); node == nil || node.Ignored {
			retired = append(retired, p)
			delete(w.providers, id)
		}
	}
	w.lock.Unlock()
	// Deliberately outside the lock: UiaDisconnectProvider calls into UI Automation, which may call back into a
	// provider, and a provider method that needed this lock would deadlock.
	for _, p := range retired {
		p.retire()
	}
}

// Destroy tells UI Automation that the window is going away and gives up the adapter's hold on every provider. The root
// package calls it from nativeAccessibilityShutdown, before DestroyWindow.
//
// The order matters. Window_WindowClosed goes out while the fragment root is still connected and the provider map still
// answers, since a client answers that event by asking about the window it names: one that walks the fragment to
// describe the window one last time has to find the window's content still there. UiaReturnRawElementProvider with a
// nil provider then withdraws the window's provider, so that a WM_GETOBJECT arriving between here and DestroyWindow is
// not answered with a fragment whose providers are being released. Only then is each provider disconnected and
// released, and the fragment root last of all.
//
// Providers a client still holds stay alive but answer UIA_E_ELEMENTNOTAVAILABLE, and the adapter itself answers
// nothing further: Root and Provider return nil from here on. A second call does nothing, so a window destroyed twice —
// or one shut down and then destroyed — is not a problem.
func (w *UIAWindow) Destroy() {
	// The snapshot memo holds a strong reference to whichever snapshot was asked about last, which may well be this
	// window's. Nothing will be answering from it once this returns, so it is dropped rather than left holding a whole
	// tree, and every node in it, until some other window happens to be asked one of the questions it remembers.
	defer uiaForgetSnapshotMemo()
	root := w.Root()
	if root != nil {
		if uiaClientsAreListening() {
			uiaRaiseAutomationEvent(root.Unknown(), UIA_Window_WindowClosedEventId)
		}
		if w.hwnd != 0 {
			// A zero handle belongs to an adapter built by a test. There is no window for UI Automation to withdraw a
			// provider from, and the call would name one that never received a WM_GETOBJECT.
			uiaReturnRawElementProvider(w.hwnd, 0, 0, nil)
		}
	}
	w.lock.Lock()
	retired := make([]*UIAProvider, 0, len(w.providers))
	for id, p := range w.providers {
		if id == w.rootNode {
			// The fragment root is retired below, after everything else.
			continue
		}
		retired = append(retired, p)
	}
	w.providers = nil
	w.lock.Unlock()
	// Deliberately outside the lock, for the reason retireRemovedProviders gives: disconnecting a provider calls back
	// into it, and a provider method that needed this lock would deadlock.
	for _, p := range retired {
		p.retire()
	}
	if root == nil {
		return
	}
	// The fragment root is given up only after the disconnects, not with the rest of the map: disconnecting a provider
	// calls back into it, and get_FragmentRoot is one of the calls that can arrive. Clearing the field under the lock
	// before the provider map's reference is given up is what keeps a caller that is inside Root at this moment from
	// being handed a provider that is about to be unpinned.
	w.lock.Lock()
	held := w.root == root
	w.root = nil
	w.lock.Unlock()
	if held {
		root.retireRoot()
	}
	root.release()
}

// adviseEvents adjusts the count of event listeners UI Automation has told the fragment root about.
func (w *UIAWindow) adviseEvents(delta int32) {
	w.listeners.Add(delta)
}

// dispatch hands one action request to the window that owns this adapter, reporting whether there was anywhere to send
// it. The window queues the work onto the UI thread, so this returns long before the action has happened; UI Automation
// expects exactly that, and learns the outcome from the events the next snapshot produces.
func (w *UIAWindow) dispatch(request accessibility.ActionRequest) bool {
	if w.action == nil {
		return false
	}
	w.action(request)
	return true
}
