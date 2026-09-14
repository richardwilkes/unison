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
	// Lost is called, at most once, when the connection to the accessibility bus ends without [Adapter.Stop] having
	// asked for it, which is what happens when the accessibility bus itself goes away. Nothing published on the adapter
	// reaches anyone afterwards, so the root package throws it away and, if the desktop still wants to be served,
	// starts a fresh one. It is called on one of the connection's own goroutines and must not block. It may be nil, in
	// which case [Adapter.Err] is the only way the loss can be learned of.
	Lost func(err error)
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
	err        error
	cfg        Config
	desktop    dbus.ObjectRef
	name       string
	order      []*windowState
	appID      atomic.Int32
	stopping   atomic.Bool
	// returned reports that [Start] handed this adapter to its caller, which is what makes a lost connection worth
	// reporting: nothing that happens before the caller holds the adapter is its business, and [Start] says so by
	// returning an error instead. It is read and written under lock, so that it and err are decided together.
	returned bool
	lock     sync.RWMutex
	// rejoining reports that a goroutine is inside [Adapter.embed] on behalf of [Adapter.reembed], and rejoinWanted
	// that another registry has appeared since it started. Both are held under rejoinLock, which is a lock of its own
	// so that a rejoin that is waiting for the registry to answer holds nothing a query needs.
	rejoinLock   sync.Mutex
	rejoining    bool
	rejoinWanted bool
}

// Start connects to the accessibility bus, exports the AT-SPI objects and asks the registry to add the application to
// the accessibility tree. Nothing is published yet: the adapter answers for an application with no windows until the
// first [Adapter.Publish].
//
// An error means the application is not on the accessibility bus at all, most often because there is no accessibility
// bus to reach, because the registry is not running, or because the connection ended while the application was still
// joining the tree. Nothing has been left behind when one is returned, and [Config.Lost] is never called for an adapter
// that was not handed back: an adapter that exists reports its own loss, and one that does not is the error.
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
	// Watching for the registry before joining the tree rather than after is what keeps a registry that is restarted
	// between the two from being missed, which would leave the application out of the desktop's tree with nothing to
	// ever put it back.
	a.watchRegistry()
	if err := a.embed(); err != nil {
		conn.Close()
		return nil, err
	}
	// Learning that the connection has ended matters as much as anything published over it: an accessibility bus that
	// is restarted behind a launcher that keeps its name produces no other sign that it has gone, and everything
	// written to a connection that has ended is silently dropped. This is registered last, so that a connection lost
	// while the application was still being exported and embedded is reported by the error returned above instead.
	//
	// A connection that has already ended runs the handler at once, from inside this call, which is what an
	// accessibility bus that dies between the Embed reply and this line does. There is then nothing to hand back: the
	// adapter would answer for an application nobody can reach, and a caller that installed it would wait forever for
	// a report it has already missed. Whether the loss is reported through [Config.Lost] or as the error returned here
	// is settled under the lock, so that it is always exactly one of the two.
	conn.OnDisconnect(a.connectionLost)
	a.lock.Lock()
	err := a.err
	a.returned = err == nil
	a.lock.Unlock()
	if err != nil {
		conn.Close() // Everything exported, subscribed and embedded above goes with the connection
		return nil, err
	}
	return a, nil
}

// Err returns the error that ended the connection to the accessibility bus, or nil while the adapter is still usable.
// A non-nil answer means nothing published since is reaching anyone: the adapter has to be thrown away, and a fresh one
// started if the desktop still wants to be served. [Adapter.Stop] makes it non-nil before it returns, so an adapter
// that has been stopped never reports itself as usable.
func (a *Adapter) Err() error {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.err
}

// connectionLost records why the connection to the accessibility bus ended and, unless it ended because [Adapter.Stop]
// closed it on purpose, tells whoever asked to be told. It runs on one of the connection's own goroutines, and may run
// from inside [Start] when the connection had already ended by the time the handler was registered; a loss that early
// is reported by Start returning an error rather than through the callback, since the caller does not yet hold the
// adapter.
func (a *Adapter) connectionLost(err error) {
	a.lock.Lock()
	if a.err == nil {
		a.err = err
	}
	report := a.returned
	a.lock.Unlock()
	if !report || a.stopping.Load() || a.cfg.Lost == nil {
		return
	}
	a.cfg.Lost(err)
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

// registryMatchRule is what the accessibility bus is asked for so that it delivers the signal that says the registry
// has come or gone. Only the bus itself sends NameOwnerChanged, and only its own name can be the sender, so a rule that
// names both cannot be satisfied by anything another peer on the bus emits.
const registryMatchRule = "type='signal',sender='" + dbusDestination + "',interface='" + dbusInterface +
	"',member='" + nameOwnerChanged + "',arg0='" + RegistryDestination + "'"

// watchRegistry asks the accessibility bus to say when the registry comes back and joins the accessibility tree again
// when it does.
//
// at-spi2-registryd can be restarted, and is on any desktop where the session is left running while the assistive
// technology stack is not. The connection to the accessibility bus survives that, so nothing else reports it: the
// application is simply no longer among the desktop's children, every event it sends afterwards reaches nobody, and the
// desktop reference it holds names a bus name that no longer exists. at-spi2-atk registers itself again on this signal,
// and so does this.
func (a *Adapter) watchRegistry() {
	a.conn.Subscribe(dbus.SignalFilter{
		Sender:    dbusDestination,
		Path:      dbusObjectPath,
		Interface: dbusInterface,
		Member:    nameOwnerChanged,
	}, func(msg *dbus.Message) {
		if !registryIsBack(msg) {
			return
		}
		// This runs on the connection's dispatcher goroutine, which answers every query an assistive technology makes,
		// so the call that rejoins the tree is made on a goroutine of its own.
		go a.reembed()
	})
	if err := a.conn.AddMatch(registryMatchRule); err != nil {
		errs.Log(errs.NewWithCause("atspi: unable to watch for the accessibility registry restarting", err))
	}
}

// registryIsBack reports whether a NameOwnerChanged signal says that the registry's name has gained an owner, which is
// what a registry that has just started up does.
func registryIsBack(msg *dbus.Message) bool {
	args, err := msg.Args()
	if err != nil || len(args) < 3 {
		return false
	}
	name, ok := args[0].(string)
	if !ok || name != RegistryDestination {
		return false
	}
	owner, ok := args[2].(string)
	return ok && owner != ""
}

// reembed joins the accessibility tree again, replacing the desktop reference that the registry which has just gone
// away handed out. Nothing is retried: a registry that cannot be talked to now will be talked to when it appears again,
// which is the only thing that would make a retry work anyway.
//
// One rejoin runs at a time. A registry that crash-loops takes its name several times in quick succession, and an Embed
// holds a pending call for as long as the connection's call timeout allows, so starting one per signal would leave
// several in flight at once with nothing to order their replies: whichever landed last would decide the desktop the
// application root reports as its parent, and that may well be the one an older registry incarnation handed back. A
// signal that arrives while a rejoin is under way therefore only asks the goroutine already doing it to go again once
// it is done, which is both what the newest registry needs and the last word on which desktop is current.
func (a *Adapter) reembed() {
	a.rejoinLock.Lock()
	if a.rejoining {
		a.rejoinWanted = true
		a.rejoinLock.Unlock()
		return
	}
	a.rejoining = true
	a.rejoinLock.Unlock()
	for {
		if !a.stopping.Load() && a.Err() == nil {
			if err := a.embed(); err != nil {
				errs.Log(errs.NewWithCause("atspi: unable to rejoin the accessibility tree", err))
			}
		}
		a.rejoinLock.Lock()
		if !a.rejoinWanted {
			a.rejoining = false
			a.rejoinLock.Unlock()
			return
		}
		a.rejoinWanted = false
		a.rejoinLock.Unlock()
	}
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
// goes away with it, and the adapter must not be used afterwards. It never waits for the bus: it is called from the
// user interface thread, and what is left to say is said on a goroutine of its own.
//
// The [Config.Lost] callback is not called for a connection that ends this way: the caller is the one that ended it, so
// there is nothing to report.
func (a *Adapter) Stop() {
	a.stopping.Store(true)
	a.lock.Lock()
	// The connection ends on a goroutine of its own, so a caller that asked whether the adapter was still usable the
	// moment after this returned would otherwise be told that it was. Whether it was still usable before that is what
	// decides whether there is anyone left to tell that the application is going.
	alive := a.err == nil
	if alive {
		a.err = dbus.ErrClosed
	}
	a.lock.Unlock()
	go a.leave(alive)
}

// leave tells the registry the application is going and closes the connection.
//
// It runs on a goroutine of its own because [Adapter.Stop] is called from the user interface thread, while the message
// has to reach the transport before the connection is closed, since closing it throws away whatever is still queued. An
// assistive technology or a bus that has stopped reading — exactly the state switching a screen reader off can leave
// things in — would otherwise hold the whole user interface up for the length of a write timeout. Nothing waits for
// this: the adapter is unusable from the moment Stop was called, and everything it answered for goes with it.
func (a *Adapter) leave(tell bool) {
	if tell {
		a.unembed()
	}
	a.conn.Close()
}

// unembed tells the registry the application is leaving. No reply is asked for: the registry has nothing to say, and
// waiting for one would hold up a shutdown that may be happening because the bus has already gone.
func (a *Adapter) unembed() {
	msg := dbus.NewMethodCall(RegistryDestination, RootPath, InterfaceSocket, "Unembed")
	msg.Flags |= dbus.FlagNoReplyExpected
	if err := msg.SetBodyWithSignature(objectRefSignature, a.rootReference()); err != nil {
		errs.Log(err)
		return
	}
	if err := a.conn.Send(msg); err != nil {
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
			// Only an entry that still points at this window is dropped. A panel can be reparented into another
			// window, and that window may well have published the node before the one it left publishes a snapshot
			// without it; taking the entry away then would leave the node's object answering UnknownObject until the
			// window that actually holds it published again.
			if _, exists := tree.Nodes[id]; !exists && a.nodeWindow[id] == ws {
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
			a.emitBoundsChanged(nil, data, data.tree.Root)
			return
		}
	}
}

// RemoveWindow takes a window out of the accessibility tree. Its nodes stop being reachable at once; the snapshot
// itself is kept a little longer, since the signals that announce the window's departure are built from it. A node that
// has been reparented into another window that is still published belongs to that window now, and neither stops
// answering nor is announced as gone.
//
// The same key may be published again afterwards, which is what a window that is hidden and shown again does: it joins
// the application's children once more, as a window nothing has been told about yet.
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
				// As in [Adapter.Publish], a node that has since moved to another window that is still published
				// belongs to that window now, and closing the one it came from must not un-register it.
				if a.nodeWindow[id] == ws {
					delete(a.nodeWindow, id)
				}
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
