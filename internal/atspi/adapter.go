// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"slices"
	"sync"
	"sync/atomic"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// WindowKey identifies one window to an [Adapter]. The root package uses whatever it already has that is unique per
// window and does not change while the window exists.
type WindowKey uint32

// Geometry is what turns a window's logical, window-local coordinates into the physical pixels AT-SPI reports.
type Geometry struct {
	// Origin is the position, in physical pixels, of the top left corner of the window's content area on the screen.
	Origin geom.Point
	// Scale is how many physical pixels there are per logical unit, which is the window's backing scale.
	Scale geom.Point
}

// Config is what an [Adapter] needs in order to run.
type Config struct {
	// Session is the connection to the desktop session bus, used to find the accessibility bus. It may be nil if
	// BusAddress is set.
	Session *dbus.Conn
	// X11Address is asked for the address of the accessibility bus if neither the environment nor the session bus can
	// say what it is. It may be nil. See [BusAddress].
	X11Address func() string
	// Action is called with every request an assistive technology makes. It is called on the connection's dispatcher
	// goroutine and must not block: the root package hands the request to the user interface thread and returns.
	Action func(req accessibility.ActionRequest)
	// conn is the connection to use instead of dialing the accessibility bus. It is unexported, so only this package
	// can supply one, which is what the tests do: they hand [Start] a connection to a fake registry over a net.Pipe.
	// It must already be authenticated and have said Hello, as a connection from [dbus.Dial] has, since the unique
	// name the bus assigned is part of every object reference the adapter hands out.
	conn *dbus.Conn
	// BusAddress is the address of the accessibility bus. When it is empty, [BusAddress] is asked to find it.
	BusAddress string
	// ToolkitVersion is the version of Unison, which is reported as both the application's version and the toolkit's.
	ToolkitVersion string
}

// Adapter publishes Unison's accessibility snapshots on the accessibility bus. It is created by [Start] and everything
// else about its lifetime — [Adapter.Publish], [Adapter.SetGeometry], [Adapter.RemoveWindow], [Adapter.Announce] and
// [Adapter.Stop] — is called from the user interface thread, while the queries it answers arrive on the connection's
// dispatcher goroutine.
type Adapter struct {
	conn       *dbus.Conn
	windows    map[WindowKey]*windowState
	nodeWindow map[accessibility.NodeID]*windowState
	cfg        Config
	desktop    dbus.ObjectRef
	name       string
	order      []*windowState
	appID      atomic.Int32
	lock       sync.RWMutex
}

// Start connects to the accessibility bus, exports the AT-SPI objects and asks the registry to add the application to
// the accessibility tree. Nothing is published yet: the adapter answers for an application with no windows until the
// first [Adapter.Publish].
//
// An error means the application is not on the accessibility bus at all, most often because there is no accessibility
// bus to reach or because the registry is not running. Nothing has been left behind when one is returned.
func Start(cfg Config) (*Adapter, error) {
	conn := cfg.conn
	if conn == nil {
		address := cfg.BusAddress
		if address == "" {
			var err error
			if address, err = BusAddress(cfg.Session, cfg.X11Address); err != nil {
				return nil, err
			}
		}
		dialed, err := dbus.Dial(address)
		if err != nil {
			return nil, err
		}
		conn = dialed
	}
	a := &Adapter{
		conn:       conn,
		windows:    make(map[WindowKey]*windowState),
		nodeWindow: make(map[accessibility.NodeID]*windowState),
		cfg:        cfg,
		desktop:    nullReference(),
		name:       conn.Name(),
	}
	if err := a.export(); err != nil {
		conn.Close()
		return nil, err
	}
	if err := a.embed(); err != nil {
		conn.Close()
		return nil, err
	}
	return a, nil
}

// export publishes the three kinds of object the adapter answers for: the application root, the cache, and one object
// per node, resolved as it is asked for.
func (a *Adapter) export() error {
	if err := a.conn.Export(RootPath, &rootObject{a: a}); err != nil {
		return err
	}
	if err := a.conn.Export(CachePath, &cacheObject{a: a}); err != nil {
		return err
	}
	return a.conn.ExportSubtree(AccessiblePrefix, a.resolve)
}

// embed asks the registry to add the application to the accessibility tree. The reply is the desktop object, which
// becomes the parent of the application root.
func (a *Adapter) embed() error {
	msg := dbus.NewMethodCall(RegistryDestination, RootPath, InterfaceSocket, "Embed")
	if err := msg.SetBodyWithSignature(objectRefSignature, a.rootReference()); err != nil {
		return err
	}
	reply, err := a.conn.Call(msg)
	if err != nil {
		return err
	}
	args, err := reply.Args()
	if err != nil {
		return err
	}
	desktop, ok := dbus.ObjectRef{}, false
	if len(args) == 1 {
		desktop, ok = args[0].(dbus.ObjectRef)
	}
	if !ok {
		return errs.Newf("atspi: the registry did not return the desktop in reply to Embed (%s)", reply)
	}
	a.lock.Lock()
	a.desktop = desktop
	a.lock.Unlock()
	return nil
}

// Stop tells the registry the application is leaving the accessibility tree and closes the connection. Every object
// goes away with it, and the adapter must not be used afterwards.
func (a *Adapter) Stop() {
	a.unembed()
	a.conn.Close()
}

// unembed tells the registry the application is leaving. No reply is asked for: the registry has nothing to say, and
// waiting for it would hold up a shutdown that may be happening because the bus has already gone.
func (a *Adapter) unembed() {
	msg := dbus.NewMethodCall(RegistryDestination, RootPath, InterfaceSocket, "Unembed")
	if err := msg.SetBodyWithSignature(objectRefSignature, a.rootReference()); err != nil {
		errs.Log(err)
		return
	}
	if _, err := a.conn.CallWithFlags(msg, dbus.FlagNoReplyExpected); err != nil {
		errs.Log(errs.NewWithCause("atspi: unable to tell the registry the application is leaving", err))
	}
}

// Publish makes a window's newly built snapshot the one every query is answered from, and reports the events that led
// to it. A window that has not been published before joins the application root's children.
//
// The tree and the events must not be modified afterwards. The write lock is held only long enough to swap the pointers
// and update the index from node to window, so a query being answered on the dispatcher goroutine is never held up for
// longer than that, and never at all once it has resolved the object it is answering for.
func (a *Adapter) Publish(key WindowKey, tree *accessibility.Tree, events []accessibility.Event, g Geometry) {
	if tree == nil {
		return
	}
	data := newWindowData(tree, g)
	a.lock.Lock()
	ws := a.windows[key]
	if ws == nil {
		ws = &windowState{}
		a.windows[key] = ws
		a.order = append(a.order, ws)
	}
	prior := ws.data.Swap(data)
	if prior != nil {
		for id := range prior.tree.Nodes {
			if _, exists := tree.Nodes[id]; !exists {
				delete(a.nodeWindow, id)
			}
		}
	}
	for id := range tree.Nodes {
		a.nodeWindow[id] = ws
	}
	a.lock.Unlock()
	a.emitEvents(ws, prior, data, events)
}

// SetGeometry records where a window's content area now is on the screen and how many physical pixels there are per
// logical unit. The published tree is unaffected: only the conversion of its coordinates changes.
//
// The window's new area on the screen is announced, since a window's own extents are the one set of coordinates an
// assistive technology is told about rather than asked for. Nothing is sent when the geometry is the one already in
// effect, so the refresh that comes with every publish costs nothing for a window that has not moved.
func (a *Adapter) SetGeometry(key WindowKey, g Geometry) {
	a.lock.RLock()
	ws := a.windows[key]
	a.lock.RUnlock()
	if ws == nil {
		return
	}
	for {
		prior := ws.data.Load()
		if prior == nil || prior.geometry == g {
			return
		}
		data := prior.withGeometry(g)
		if ws.data.CompareAndSwap(prior, data) {
			a.emitBoundsChanged(data, data.tree.Root)
			return
		}
	}
}

// RemoveWindow takes a window out of the accessibility tree. Its nodes stop being reachable at once; the snapshot
// itself is kept a little longer, since the signals that announce the window's departure are built from it.
func (a *Adapter) RemoveWindow(key WindowKey) {
	a.lock.Lock()
	ws := a.windows[key]
	index := -1
	if ws != nil {
		delete(a.windows, key)
		if index = slices.Index(a.order, ws); index >= 0 {
			a.order = slices.Delete(a.order, index, index+1)
		}
		if data := ws.data.Load(); data != nil {
			for id := range data.tree.Nodes {
				delete(a.nodeWindow, id)
			}
		}
	}
	a.lock.Unlock()
	if ws == nil {
		return
	}
	a.emitWindowRemoved(ws, index)
}

// Announce asks an assistive technology to say something that is not tied to any object, such as the result of an
// operation that changed nothing on the screen.
func (a *Adapter) Announce(text string) {
	if text == "" {
		return
	}
	a.emitAnnouncement(text)
}

// dispatch hands an action request to the callback the root package supplied and returns true if there was one to hand
// it to. It never waits for the request to be carried out: the user interface thread may be inside a modal loop or a
// drag, and the assistive technology's client-side timeout is only two seconds, so every answer is optimistic.
func (a *Adapter) dispatch(req accessibility.ActionRequest) bool {
	if a.cfg.Action == nil {
		return false
	}
	a.cfg.Action(req)
	return true
}

// rootReference returns the reference to the application root object.
func (a *Adapter) rootReference() dbus.ObjectRef {
	return dbus.ObjectRef{Name: a.name, Path: RootPath}
}

// reference returns the reference to the object that represents a node, or the null reference for node zero, which is
// how the schema says "no node".
func (a *Adapter) reference(id accessibility.NodeID) dbus.ObjectRef {
	if id == 0 {
		return nullReference()
	}
	return dbus.ObjectRef{Name: a.name, Path: NodePath(id)}
}

// references returns the references to the objects that represent a list of nodes.
func (a *Adapter) references(ids []accessibility.NodeID) []dbus.ObjectRef {
	refs := make([]dbus.ObjectRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, a.reference(id))
	}
	return refs
}

// desktopReference returns the reference to the desktop object the registry handed back, which is the parent of the
// application root. It is the null reference until the registry has answered.
func (a *Adapter) desktopReference() dbus.ObjectRef {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.desktop
}

// nullReference returns the reference that means "nothing". AT-SPI spells it as an empty bus name and the null path.
func nullReference() dbus.ObjectRef {
	return dbus.ObjectRef{Path: NullPath}
}

// windowOrder returns the windows in the order they were first published, which is the order the application root
// reports its children in.
func (a *Adapter) windowOrder() []*windowState {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return slices.Clone(a.order)
}

// windowIndex returns where a window sits among the application root's children, or -1 if it is no longer there.
func (a *Adapter) windowIndex(ws *windowState) int {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return slices.Index(a.order, ws)
}

// windowFor returns the window that holds a node, or nil if no published window does.
func (a *Adapter) windowFor(id accessibility.NodeID) *windowState {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.nodeWindow[id]
}
