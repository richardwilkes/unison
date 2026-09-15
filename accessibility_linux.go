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
	"runtime/debug"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/atspi"
	"github.com/richardwilkes/unison/internal/dbus"
	"github.com/richardwilkes/unison/internal/x11"
)

// The Linux side of accessibility support: this file connects the snapshots the root package publishes to the AT-SPI2
// server in internal/atspi, and connects the requests that come back from an assistive technology to the UI thread.
//
// Unlike macOS and Windows, where the platform asks a window a question and that question is the thing that turns
// accessibility support on, AT-SPI has to be joined before anything can ask: an application is only reachable once it
// has put objects on the accessibility bus and told the registry about them. What stands in for the first query is the
// accessibility bus launcher's org.a11y.Status.IsEnabled property, which says whether anything on the desktop is
// listening. It is watched, over the session-bus connection the color-scheme watcher already keeps, and read once the
// watch is in place, so that a screen reader started or stopped while the application runs is followed both ways —
// which makes Linux the one platform where support is also torn down again.
//
// The cost to an application nothing is watching is one property read, one name-owner lookup and two match rules, all
// on the session connection that already exists, and nothing whatsoever when there is no session bus. The accessibility
// bus socket, the goroutines that serve it and every object on it exist only while support is enabled.
//
// Everything here runs on the UI thread except four things, each of which hands whatever it has to do straight to the
// UI thread with InvokeTask and touches none of the linuxA11y* globals itself: the action callback and the report that
// the accessibility bus connection has died, which arrive on that connection's own goroutines; the atspi.WatchEnabled
// callback, which arrives on one of the session connection's goroutines; the goroutine linuxStartA11y makes to join the
// accessibility bus, since the join is three blocking round trips; and the goroutine linuxA11yRejoin makes to ask the
// desktop, over the session bus, whether it still wants to be served.

// unisonModulePath is this module's import path, which is how the version to report to an assistive technology is found
// among the modules the running binary was built from.
const unisonModulePath = "github.com/richardwilkes/unison"

// unknownToolkitVersion is reported when the binary carries no build information, which is the case for one built with
// the module system turned off.
const unknownToolkitVersion = "unknown"

// atspiBusAddressLimit bounds how much of the X11 root window's AT_SPI_BUS property is read, in 4-byte units. A bus
// address is a socket path and a guid, so four kilobytes is far more than one can be.
const atspiBusAddressLimit = 1024

// linuxA11yRejoinGrace is how long the connection to the accessibility bus has to have been up for its loss to count as
// a fresh one, which may be recovered from however often it happens. A bus that accepts a connection and immediately
// drops it does so at once, so anything that dies this soon after being installed is that bus rather than one which
// really went away, and gets the single rebuild linuxA11yRejoinAttempted allows and no more. It is a variable only so
// that the tests can lengthen it; nothing changes it at runtime.
var linuxA11yRejoinGrace = 5 * time.Second

var (
	// linuxA11y is the AT-SPI2 server, which exists only while an assistive technology is being served. UI thread only.
	linuxA11y *atspi.Adapter
	// linuxA11yCancelWatch stops watching the accessibility bus launcher's IsEnabled property. UI thread only.
	linuxA11yCancelWatch func()
	// linuxA11yAttempt counts the attempts made to join the accessibility bus. Each attempt captures the number it was
	// begun as, which is what identifies it afterwards: the adapter itself cannot, since the goroutine that dials is
	// still inside atspi.Start when the first report of its connection dying may arrive. Bumping it disowns whatever is
	// in flight, so an adapter that arrives after support was turned off, or after a later attempt overtook it, is
	// stopped rather than installed. UI thread only, apart from the value each attempt's own goroutine holds.
	linuxA11yAttempt uint64
	// linuxA11yCurrentAttempt is the attempt the adapter in linuxA11y came from, and zero when there is none. UI thread
	// only.
	linuxA11yCurrentAttempt uint64
	// linuxA11yJoining reports that an attempt to join the accessibility bus is in flight, so that a second one is not
	// begun alongside it. UI thread only.
	linuxA11yJoining bool
	// linuxA11yRejoinAttempted records that the accessibility bus connection has already been lost once and rebuilt
	// without the desktop having said anything in between, so that a bus which accepts a connection and immediately
	// drops it is dialed once rather than forever. It is cleared when the atspi.WatchEnabled callback reports the
	// status, and when the connection that died had been up for longer than linuxA11yRejoinGrace, since neither of
	// those is the bus the guard is aimed at.
	//
	// Only the watch clears it, and deliberately not the answer linuxA11yRejoin fetches for itself: that answer is a
	// property read this code asked for rather than the desktop volunteering anything, and a bus that accepts a
	// connection and drops it while the launcher goes on saying an assistive technology is there would have every
	// rejoin clear the guard that the next loss is about to consult, which is the endless dial loop the guard exists to
	// stop. The asymmetry is the invariant; making the two paths agree breaks it.
	//
	// UI thread only.
	linuxA11yRejoinAttempted bool
	// linuxA11yInstalledAt is when the adapter in linuxA11y was installed, which is what says how long a connection had
	// been up for when it dies. It deliberately outlives the adapter it describes: linuxA11yRejoin is reached only
	// after linuxStopA11y has thrown that adapter away, and what it has to know is how the connection that just ended
	// behaved. UI thread only.
	linuxA11yInstalledAt time.Time
)

// linuxA11yStartMode is what the environment and the startup options have decided about accessibility support before
// the session bus is consulted at all.
type linuxA11yStartMode uint8

// The possible decisions.
const (
	// linuxA11yRefused means nothing may talk to the accessibility bus: NO_AT_BRIDGE is set, AccessibilityEnvKey asked
	// for support to be refused, or the NoAccessibility startup option was used. Nothing is read, nothing is watched
	// and nothing is retried.
	linuxA11yRefused linuxA11yStartMode = iota
	// linuxA11yForced means AccessibilityEnvKey asked for support whatever the desktop reports, so the server is
	// started without asking whether anything is listening.
	linuxA11yForced
	// linuxA11yAsk means the desktop decides, which is the ordinary case.
	linuxA11yAsk
)

// linuxA11yMode returns what the environment and the startup options have decided. NO_AT_BRIDGE is honored even when
// AccessibilityEnvKey asks for support, since it is the whole desktop's instruction rather than this application's.
func linuxA11yMode() linuxA11yStartMode {
	if noAccessibility.Load() || accessibilityEnv.Load() < 0 || atspi.DisabledByEnvironment() {
		return linuxA11yRefused
	}
	if accessibilityEnv.Load() > 0 {
		return linuxA11yForced
	}
	return linuxA11yAsk
}

// linuxA11yStatusInit decides whether an assistive technology is being served and arranges to be told when that
// changes. It is called at the end of nativeLateInit, and again whenever SetAccessibilityEnabled lifts a refusal, and
// is the only thing on this platform that turns snapshot building on by itself.
//
// The property is read by the watch rather than here, once the bus has accepted the match rules it needs: those rules
// are asked for with calls of their own, so the bus is not yet delivering anything when atspi.WatchEnabled returns, and
// a read taken at that point could miss a screen reader that started in between and then never hear the signal that
// would have reported it. Every answer, the first one included, therefore arrives through the watch, on the UI thread
// behind this call, and ends up in linuxSetA11yEnabled, which is idempotent.
func linuxA11yStatusInit() {
	if linuxA11yCancelWatch != nil {
		return
	}
	switch linuxA11yMode() {
	case linuxA11yRefused:
		return
	case linuxA11yForced:
		linuxSetA11yEnabled(true)
		return
	default:
	}
	session, err := dbus.Session()
	if err != nil || session == nil {
		// There is no session bus, so there is no launcher to ask and nothing that could ever answer. Nothing is
		// retried: a session bus does not appear part way through the life of a process.
		return
	}
	linuxA11yCancelWatch = atspi.WatchEnabled(session, func(enabled bool) {
		// This arrives on one of the session connection's goroutines, so the answer is handed to the UI thread, which
		// is the only place the adapter and the window list may be touched.
		InvokeTask(func() {
			// The desktop has spoken for itself, which makes whatever happened to an earlier connection to the
			// accessibility bus history: a loss after this may be recovered from again.
			linuxA11yRejoinAttempted = false
			linuxSetA11yEnabled(enabled)
		})
	})
}

// linuxSetA11yEnabled starts or stops serving assistive technologies. UI thread only, and idempotent, so the startup
// read and every answer the watch produces may call it without checking what the state already is.
func linuxSetA11yEnabled(enabled bool) {
	if enabled {
		linuxStartA11y()
		return
	}
	linuxStopA11y()
}

// linuxStartA11y begins joining the accessibility bus. The join itself happens on a goroutine, since it is three
// blocking round trips — asking the launcher for the bus address, dialing it, and asking the registry to embed the
// application — each of which may take the five seconds its timeout allows on a desktop whose launcher or registry is
// wedged. The UI thread must not be made to wait fifteen seconds for a screen reader starting up, so everything that
// can only be done here is done here and the rest is handed over; linuxFinishStartA11y installs the result.
//
// Nothing is left behind when joining fails: the desktop said something was listening, but there was nothing to reach,
// and the watch will ask again the next time it is told the answer has changed.
func linuxStartA11y() {
	if linuxA11y != nil || linuxA11yJoining || linuxA11yMode() == linuxA11yRefused {
		return
	}
	// The session connection is what the bus address is normally found through. It may be nil, which happens only when
	// the environment forced support on without a session bus to ask; the address is then looked for in the environment
	// and on the X11 root window instead. The connection is safe to use from the goroutine below, which is what every
	// call on it already does.
	session, _ := dbus.Session() //nolint:errcheck // A failure here leaves session nil, which is what Config allows for
	// The X11 fallback is read here rather than being handed over as the function that reads it: the X11 connection is
	// the UI thread's, and the property is one round trip to the X server whether or not the two places looked at
	// before it have the answer.
	x11Address := linuxX11A11yAddress()
	version := linuxToolkitVersion()
	linuxA11yJoining = true
	linuxA11yAttempt++
	attempt := linuxA11yAttempt
	go func() {
		// The attempt number, rather than the adapter, is what the report of a dead connection names itself by: this
		// goroutine is still inside Start when that report first becomes possible, so anything it has yet to assign
		// cannot be read from the connection's own goroutines.
		adapter, err := atspi.Start(atspi.Config{
			Session:        session,
			X11Address:     func() string { return x11Address },
			Action:         linuxA11yAction,
			Lost:           func(lost error) { InvokeTask(func() { linuxA11yConnectionLost(attempt, lost) }) },
			ToolkitVersion: version,
		})
		InvokeTask(func() { linuxFinishStartA11y(attempt, adapter, err) })
	}()
}

// linuxFinishStartA11y installs the adapter one attempt at joining the accessibility bus produced, or reports why there
// is none. UI thread only.
//
// Everything may have moved on while the join was in flight: support may have been turned off, the desktop may have
// said the assistive technology went away, or a later attempt may have installed an adapter of its own. The attempt
// number is what tells an answer that is still wanted from one that has been superseded, and an adapter that arrives
// too late is stopped rather than installed.
func linuxFinishStartA11y(attempt uint64, adapter *atspi.Adapter, err error) {
	current := attempt == linuxA11yAttempt
	if current {
		linuxA11yJoining = false
	}
	if current && err == nil && adapter != nil {
		// The connection may have died while the join was still in flight, which atspi.Start reports through
		// Config.Lost rather than by returning an error, from the moment it decides to hand the adapter back — before
		// this has been queued, let alone run. linuxA11yConnectionLost then rejects that report as belonging to no
		// current attempt, since there is nothing for it to name until the install below, and Lost is called at most
		// once, so an adapter that is not asked here would sit in linuxA11y for the rest of the process: everything
		// published on it would be silently dropped, linuxStartA11y would refuse to build a replacement, and nothing
		// would ever report the loss again. Asking it makes such a join a failed one, which the watch and
		// linuxA11yRejoin can try again after.
		err = adapter.Err()
	}
	if adapter == nil || err != nil || !current || linuxA11y != nil || linuxA11yMode() == linuxA11yRefused {
		if adapter != nil {
			adapter.Stop()
		}
		if err != nil {
			errs.Log(errs.NewWithCause("unable to join the accessibility bus", err))
		}
		if current && linuxA11y == nil && linuxA11yMode() == linuxA11yForced {
			// There is no bus to publish on, but the environment asked for snapshots whatever the desktop reports, and
			// that promise is separate from anything being there to receive them: they are built, and go nowhere. Said
			// here as well as in finishStartup so that support turned off and back on, or a connection lost and not
			// recovered, leaves such an application where it started rather than with snapshots off forever.
			activateAccessibility()
		}
		return
	}
	linuxA11y = adapter
	linuxA11yCurrentAttempt = attempt
	linuxA11yInstalledAt = time.Now()
	if !activateAccessibility() {
		// Refused after all, which linuxA11yMode above should have caught; leave nothing behind.
		linuxA11y = nil
		linuxA11yCurrentAttempt = 0
		linuxA11yInstalledAt = time.Time{}
		adapter.Stop()
		return
	}
	// activateAccessibility has marked every window for redraw, which would describe them all after the next pass of
	// the event loop, but a window that is already on the screen should be reachable as soon as the registry knows
	// about us rather than one frame later.
	for _, wnd := range windowList {
		if !wnd.IsValid() || wnd.root == nil || !wnd.IsVisible() {
			continue
		}
		if wnd.ax == nil {
			wnd.ax = &windowAccessibility{}
		}
		// Laid out first, since a window whose layout invalidation is still pending would otherwise be described from
		// the frames its panels had before it — the very first bounds the registry is given, and the ones a screen
		// reader draws its highlight from. Window.performAccessibilityAction lays the window out ahead of its publish
		// for the same reason.
		wnd.ValidateLayout()
		wnd.publishAccessibilityNow()
	}
}

// linuxStopA11y stops serving assistive technologies and frees everything that was serving them. Snapshot building is
// turned off first, which takes each window out of the accessibility tree through nativeAccessibilityShutdown, so that
// the registry is told the windows have gone before the application itself does.
func linuxStopA11y() {
	// Whatever is in flight is disowned first, so that an adapter which arrives after this is stopped rather than
	// installed behind the back of the decision to stop.
	linuxA11yAttempt++
	linuxA11yJoining = false
	linuxA11yCurrentAttempt = 0
	if linuxA11y == nil {
		return
	}
	deactivateAccessibility()
	linuxA11y.Stop()
	linuxA11y = nil
}

// linuxA11yConnectionLost throws away an adapter whose connection to the accessibility bus has died, and asks the
// desktop whether it wants a fresh one. UI thread only; the report itself arrives on one of the dead connection's
// goroutines and is handed here by linuxStartA11y, which names the attempt at joining that the dead connection came
// from so that a report for an adapter that has since been stopped or replaced cannot tear its replacement down.
//
// Nothing else notices such a death. The accessibility bus is a dbus-daemon of its own that at-spi-bus-launcher starts
// and can restart without ever giving up the org.a11y.Bus name, so the watch stays silent, while every snapshot
// published on the dead connection is quietly dropped and linuxStartA11y refuses to build a replacement for as long as
// linuxA11y is non-nil. Dropping the adapter is what turns snapshot building off again and makes a replacement
// possible; asking the desktop once more is what gets one built, since the ordinary answer is still yes and the
// launcher will have a new bus up by the time the question reaches it. What that comes to is linuxA11yRejoin's
// business.
func linuxA11yConnectionLost(attempt uint64, err error) {
	if attempt == 0 || linuxA11y == nil || attempt != linuxA11yCurrentAttempt {
		// The adapter has already been stopped or replaced, so its connection ending is of no consequence to anyone.
		return
	}
	errs.Log(errs.NewWithCause("the connection to the accessibility bus was lost", err))
	linuxStopA11y()
	linuxA11yRejoin()
}

// linuxA11yRejoin decides what becomes of an application whose connection to the accessibility bus has died and whose
// adapter has just been thrown away. UI thread only; it is reached from linuxA11yConnectionLost alone.
//
// Snapshot building went with the adapter, since linuxStopA11y turns it off. That is the right answer when the desktop
// decides, but not when the environment asked for support whatever the desktop reports: such an application builds
// snapshots whether or not there is a bus to publish them on, so they are turned straight back on here. It has to
// happen before the rebuild guard rather than only in linuxFinishStartA11y, since a loss the guard stops short of
// rebuilding from never reaches that function at all, and would otherwise leave such an application with snapshots off
// for the rest of its life.
//
// One rebuild is attempted for each thing the desktop says and for each connection that lasted longer than
// linuxA11yRejoinGrace, so a bus that accepts a connection and immediately drops it costs one dial rather than an
// endless succession of them, while an accessibility bus restarted under an application it had been serving is
// recovered from every time it happens. Telling the two apart by how long the connection lasted is what there is to go
// on: nothing reports either, and a restart behind a launcher that keeps org.a11y.Bus is exactly the case this
// machinery exists for.
func linuxA11yRejoin() {
	mode := linuxA11yMode()
	if mode == linuxA11yForced {
		activateAccessibility()
	}
	if !linuxA11yInstalledAt.IsZero() && time.Since(linuxA11yInstalledAt) >= linuxA11yRejoinGrace {
		// The connection that has just died was a working one, served over for longer than a bus that drops what it
		// accepts would ever have allowed, so its loss is a fresh one rather than the second half of the case the guard
		// below is aimed at, and whatever an earlier loss left set is history.
		linuxA11yRejoinAttempted = false
	}
	if linuxA11yRejoinAttempted {
		return
	}
	linuxA11yRejoinAttempted = true
	switch mode {
	case linuxA11yRefused:
		return
	case linuxA11yForced:
		// Nothing is being asked on this path, so there is nothing to wait for the answer to.
		linuxStartA11y()
		return
	default:
	}
	session, sessionErr := dbus.Session()
	if sessionErr != nil || session == nil {
		return
	}
	// Asking is a call on the session bus, which must not be made from the UI thread, so the answer comes back the same
	// way every answer the watch produces does.
	go func() {
		enabled := atspi.Enabled(session)
		InvokeTask(func() { linuxSetA11yEnabled(enabled) })
	}()
}

// linuxA11yTerminate leaves the accessibility bus. It is called first of all from nativeTerminate, so that the registry
// is told the application is going before the rest of the platform is taken apart.
//
// Nothing here touches the X11 connection, and nothing has to: the last of the three places the bus address is looked
// for is the root window's AT_SPI_BUS property, which linuxStartA11y reads on the UI thread when a join begins and
// hands atspi.Config a closure over, precisely so that neither the join goroutine nor this has any use for the
// connection.
func linuxA11yTerminate() {
	linuxA11yRejoinAttempted = false
	linuxA11yInstalledAt = time.Time{}
	if linuxA11yCancelWatch != nil {
		linuxA11yCancelWatch()
		linuxA11yCancelWatch = nil
	}
	// As in linuxStopA11y: an attempt at joining that is still in flight is disowned, so that the adapter it produces
	// is stopped on arrival rather than left running past the application's own shutdown.
	linuxA11yAttempt++
	linuxA11yJoining = false
	linuxA11yCurrentAttempt = 0
	if linuxA11y != nil {
		linuxA11y.Stop()
		linuxA11y = nil
	}
}

// nativeAccessibilityPublish hands a freshly built snapshot, and the events describing how it differs from the one
// before it, to the platform's assistive-technology adapter.
//
// A window is identified on the accessibility bus by its X11 window id, which is unique for as long as the window
// exists and is what every other part of this platform's code already knows a window by.
func (w *Window) nativeAccessibilityPublish(tree *accessibility.Tree, events []accessibility.Event) {
	// The adapter is nil whenever nothing is listening. That includes the case where AccessibilityEnvKey forced
	// snapshot building on but the accessibility bus could not be reached, which is exactly what the environment
	// override is for: the snapshots are built, and go nowhere.
	if linuxA11y == nil || tree == nil || w.wnd.id == 0 {
		return
	}
	linuxA11y.Publish(atspi.WindowKey(w.wnd.id), tree, events, w.linuxA11yGeometry())
}

// nativeAccessibilityGeometryChanged is a no-op on Linux: nothing on this platform calls the wrapper it sits behind. A
// window that has moved, been resized or changed backing scale is described to the adapter by
// x11RefreshAccessibilityGeometry instead, which the X11 event loop calls from the ConfigureNotify it is already
// handling and hands the new origin it already has, precisely so that an interactive move or resize does not cost an
// assistive technology the two X round trips Window.ContentRect would pay on every one of the events it produces.
func (*Window) nativeAccessibilityGeometryChanged() {
}

// nativeAccessibilityShutdown releases everything the adapter holds for this window. It is reached from Window.destroy
// and again from nativeDestroy, and from deactivateAccessibility when the last assistive technology goes away; taking a
// window that is not in the accessibility tree out of it does nothing, so it does not matter how often it happens.
func (w *Window) nativeAccessibilityShutdown() {
	if linuxA11y == nil || w.wnd.id == 0 {
		return
	}
	linuxA11y.RemoveWindow(atspi.WindowKey(w.wnd.id))
}

// nativeAccessibilityWindowHidden takes a window that has been hidden or minimized out of the accessibility tree. The
// application tells the registry for itself what windows it has, so a window that is no longer on the screen has to be
// withdrawn or it would go on being listed as showing and visible; it is added back when it is next drawn.
func (w *Window) nativeAccessibilityWindowHidden() bool {
	w.nativeAccessibilityShutdown()
	return true
}

// nativeAccessibilityEnabledChanged stops watching and serving when support is refused, and asks the desktop again
// whether an assistive technology is there when the refusal is lifted, since on this platform nothing else would ask.
//
// An application the environment forced support on for has snapshot building turned back on here and now, as it is on
// macOS and Windows, rather than waiting for the join to succeed: the promise SetAccessibilityEnabled makes is that the
// three platforms end up where they started, and where a forced-on application started is building snapshots whether or
// not there is an accessibility bus to publish them on.
func nativeAccessibilityEnabledChanged(enabled bool) {
	if enabled {
		if accessibilityEnv.Load() > 0 {
			activateAccessibility()
		}
		linuxA11yStatusInit()
		return
	}
	linuxA11yTerminate()
}

// nativeAccessibilityAnnounce asks the platform's assistive technology to speak text. Nothing is said when nothing is
// listening.
func nativeAccessibilityAnnounce(text string) {
	if linuxA11y != nil {
		linuxA11y.Announce(text)
	}
}

// x11RefreshAccessibilityGeometry tells the window's assistive-technology adapter, if it has one, where its content
// area now sits: origin is its upper left corner in the X server's pixels, relative to the root window, which is the
// space AT-SPI reports coordinates in. A window that is not being described pays one nil check.
//
// The origin is handed in rather than looked up because the X11 event loop already has it, from the ConfigureNotify it
// is handling, while Window.ContentRect would ask the X server for it all over again: nativeContentRect issues a
// GetGeometry and a TranslateCoordinates, two synchronous round trips, and they would be paid on every one of the
// stream of events an interactive move or resize produces — before Adapter.SetGeometry even had the chance to discard
// a geometry that has not changed. The adapter is called directly for the same reason: there is no window here that
// could be a headless one, since only the X11 event loop calls this.
func (w *Window) x11RefreshAccessibilityGeometry(origin geom.Point) {
	if w.ax == nil || linuxA11y == nil || w.wnd.id == 0 {
		return
	}
	linuxA11y.SetGeometry(atspi.WindowKey(w.wnd.id), atspi.Geometry{Origin: origin, Scale: w.BackingScale()})
}

// linuxA11yAction receives one request an assistive technology has made of a node.
//
// It arrives on the accessibility connection's dispatcher goroutine, and must not wait on the UI thread: that thread
// may be inside a modal loop or a drag, and libatspi's client-side timeout is two seconds. The request is therefore
// queued and returned from at once. The assistive technology has already been told, optimistically, that the request
// will be carried out, and learns what actually happened from the events the next snapshot produces.
func linuxA11yAction(req accessibility.ActionRequest) {
	InvokeTask(func() {
		// AT-SPI names an object without saying which window it is in — node ids are unique across the whole process,
		// so it does not have to — which leaves the window to be found from what was most recently published.
		for _, wnd := range windowList {
			if wnd.ax == nil {
				continue
			}
			if _, ok := wnd.ax.targets[req.Node]; !ok {
				continue
			}
			if wnd.performAccessibilityAction(req) {
				return
			}
			// The window that described the node refused to act on it, which is what a window says about a panel that
			// has since been reparented into another one and that it has not been described without since. The search
			// goes on rather than dropping the request on the floor.
		}
	})
}

// linuxA11yGeometry returns where the window's content area sits on the screen, in physical pixels, along with the
// backing scale the window-local logical bounds in a snapshot must be multiplied by to reach that space.
//
// Unlike Windows, this platform is served correctly by Window.accessibilityGeometry, which multiplies the content
// rect's origin by the backing scale: an X11 window's position comes from the server in device pixels and is divided by
// the scale on the way out of nativeContentRect, so multiplying it back is what recovers the pixels the X server —
// and therefore AT-SPI — works in.
func (w *Window) linuxA11yGeometry() atspi.Geometry {
	origin, scale := w.accessibilityGeometry()
	return atspi.Geometry{Origin: origin, Scale: scale}
}

// linuxX11A11yAddress returns the address of the accessibility bus as published in the AT_SPI_BUS property of the X11
// root window. This is the last of the three places the address is looked for, and matters on desktops whose launcher
// only publishes it there. Unison never sets the property itself, so an empty answer simply means nobody has.
func linuxX11A11yAddress() string {
	if x11Conn == nil {
		return ""
	}
	format, actualType, value, _, err := x11Conn.GetProperty(x11Conn.RootWindow(), x11Conn.Atoms.ATSPIBus, x11.AtomAny,
		0, atspiBusAddressLimit, false)
	if err != nil || format != 8 {
		return ""
	}
	// The launcher writes the property as a latin-1 STRING, which is what at-spi2-core asks for when it reads it back,
	// but a UTF8_STRING is accepted too rather than being thrown away over its type.
	if actualType != x11.AtomString && actualType != x11Conn.Atoms.UTF8String {
		return ""
	}
	// Properties are not required to be terminated, and are sometimes written with the terminator included.
	return strings.TrimSpace(strings.TrimRight(string(value), "\x00"))
}

// linuxToolkitVersion returns the version of Unison to report to an assistive technology. It is only asked for while
// joining the accessibility bus, so a binary nothing is watching never reads its own build information.
func linuxToolkitVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return unknownToolkitVersion
	}
	return linuxToolkitVersionFrom(info)
}

// linuxToolkitVersionFrom picks Unison's version out of a binary's build information. Unison is normally one of the
// modules the application depends on, but the version of the application itself is the right answer when the binary
// being run is Unison's own — one of its examples, or a test.
func linuxToolkitVersionFrom(info *debug.BuildInfo) string {
	if info == nil {
		return unknownToolkitVersion
	}
	for _, dep := range info.Deps {
		if dep == nil || dep.Path != unisonModulePath {
			continue
		}
		if dep.Replace != nil && dep.Replace.Version != "" {
			return dep.Replace.Version
		}
		if dep.Version != "" {
			return dep.Version
		}
	}
	if info.Main.Path == unisonModulePath && info.Main.Version != "" {
		return info.Main.Version
	}
	return unknownToolkitVersion
}
