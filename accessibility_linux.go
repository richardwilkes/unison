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
// listening. It is read once at startup, over the session-bus connection the color-scheme watcher already keeps, and
// then watched, so that a screen reader started or stopped while the application runs is followed both ways — which
// makes Linux the one platform where support is also torn down again.
//
// The cost to an application nothing is watching is that one property read plus two match rules on a connection that
// already exists, and nothing whatsoever when there is no session bus. The accessibility bus socket, the goroutines
// that serve it and every object on it exist only while support is enabled.
//
// Everything here runs on the UI thread except the action callback, which arrives on the accessibility connection's
// dispatcher goroutine and hands its work straight to the UI thread.

// unisonModulePath is this module's import path, which is how the version to report to an assistive technology is found
// among the modules the running binary was built from.
const unisonModulePath = "github.com/richardwilkes/unison"

// unknownToolkitVersion is reported when the binary carries no build information, which is the case for one built with
// the module system turned off.
const unknownToolkitVersion = "unknown"

// atspiBusAddressLimit bounds how much of the X11 root window's AT_SPI_BUS property is read, in 4-byte units. A bus
// address is a socket path and a guid, so four kilobytes is far more than one can be.
const atspiBusAddressLimit = 1024

var (
	// linuxA11y is the AT-SPI2 server, which exists only while an assistive technology is being served. UI thread only.
	linuxA11y *atspi.Adapter
	// linuxA11yCancelWatch stops watching the accessibility bus launcher's IsEnabled property. UI thread only.
	linuxA11yCancelWatch func()
)

// linuxA11yStartMode is what the environment and the startup options have decided about accessibility support before
// the session bus is consulted at all.
type linuxA11yStartMode uint8

// The possible decisions.
const (
	// linuxA11yRefused means nothing may talk to the accessibility bus: NO_AT_BRIDGE is set, AccessibilityEnvKey asked
	// for support to be refused, or the NoAccessibility startup option was used. Nothing is read, nothing is watched and
	// nothing is retried.
	linuxA11yRefused linuxA11yStartMode = iota
	// linuxA11yForced means AccessibilityEnvKey asked for support whatever the desktop reports, so the server is started
	// without asking whether anything is listening.
	linuxA11yForced
	// linuxA11yAsk means the desktop decides, which is the ordinary case.
	linuxA11yAsk
)

// linuxA11yMode returns what the environment and the startup options have decided. NO_AT_BRIDGE is honored even when
// AccessibilityEnvKey asks for support, since it is the whole desktop's instruction rather than this application's.
func linuxA11yMode() linuxA11yStartMode {
	if noAccessibility || accessibilityEnv < 0 || atspi.DisabledByEnvironment() {
		return linuxA11yRefused
	}
	if accessibilityEnv > 0 {
		return linuxA11yForced
	}
	return linuxA11yAsk
}

// linuxA11yStatusInit decides whether an assistive technology is being served and arranges to be told when that
// changes. It is called at the end of nativeLateInit, and again whenever SetAccessibilityEnabled lifts a refusal, and
// is the only thing on this platform that turns snapshot building on by itself.
//
// The watch is installed before the one property read, so that a screen reader starting up in the moment between the
// two cannot go unnoticed. Both paths end up in linuxSetA11yEnabled, which is idempotent, and the watch's answers
// arrive on the UI thread behind this call, so they cannot interleave with it.
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
		// This arrives on one of the session connection's goroutines, so the answer is handed to the UI thread, which is
		// the only place the adapter and the window list may be touched.
		InvokeTask(func() { linuxSetA11yEnabled(enabled) })
	})
	if atspi.Enabled(session) {
		linuxSetA11yEnabled(true)
	}
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

// linuxStartA11y joins the accessibility bus, turns snapshot building on and describes every window there already is.
// Nothing is left behind when joining fails: the desktop said something was listening, but there was nothing to reach,
// and the watch will ask again the next time it is told the answer has changed.
func linuxStartA11y() {
	if linuxA11y != nil || linuxA11yMode() == linuxA11yRefused {
		return
	}
	// The session connection is what the bus address is normally found through. It may be nil, which happens only when
	// the environment forced support on without a session bus to ask; the address is then looked for in the environment
	// and on the X11 root window instead.
	session, _ := dbus.Session() //nolint:errcheck // A failure here leaves session nil, which is what Config allows for
	adapter, err := atspi.Start(atspi.Config{
		Session:        session,
		X11Address:     linuxX11A11yAddress,
		Action:         linuxA11yAction,
		ToolkitVersion: linuxToolkitVersion(),
	})
	if err != nil {
		errs.Log(errs.NewWithCause("unable to join the accessibility bus", err))
		return
	}
	linuxA11y = adapter
	if !activateAccessibility() {
		// Refused after all, which linuxA11yMode above should have caught; leave nothing behind.
		linuxA11y = nil
		adapter.Stop()
		return
	}
	// activateAccessibility has marked every window for redraw, which would describe them all after the next pass of
	// the event loop, but a window that is already on the screen should be reachable as soon as the registry knows about
	// us rather than one frame later.
	for _, wnd := range windowList {
		if !wnd.IsValid() || wnd.root == nil || !wnd.IsVisible() {
			continue
		}
		if wnd.ax == nil {
			wnd.ax = &windowAccessibility{}
		}
		wnd.publishAccessibilityNow()
	}
}

// linuxStopA11y stops serving assistive technologies and frees everything that was serving them. Snapshot building is
// turned off first, which takes each window out of the accessibility tree through nativeAccessibilityShutdown, so that
// the registry is told the windows have gone before the application itself does.
func linuxStopA11y() {
	if linuxA11y == nil {
		return
	}
	deactivateAccessibility()
	linuxA11y.Stop()
	linuxA11y = nil
}

// linuxA11yTerminate leaves the accessibility bus. It is called from nativeTerminate, before the X11 connection is
// closed, since the fallback that finds the bus address reads a property from the root window.
func linuxA11yTerminate() {
	if linuxA11yCancelWatch != nil {
		linuxA11yCancelWatch()
		linuxA11yCancelWatch = nil
	}
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
	// The adapter is nil whenever nothing is listening. That includes the case where AccessibilityEnvKey forced snapshot
	// building on but the accessibility bus could not be reached, which is exactly what the environment override is for:
	// the snapshots are built, and go nowhere.
	if linuxA11y == nil || tree == nil || w.wnd.id == 0 {
		return
	}
	linuxA11y.Publish(atspi.WindowKey(w.wnd.id), tree, events, w.linuxA11yGeometry())
}

// nativeAccessibilityGeometryChanged tells the adapter that the window has moved, resized or changed backing scale, so
// that the screen coordinates it reports are recomputed.
func (w *Window) nativeAccessibilityGeometryChanged() {
	if linuxA11y == nil || w.wnd.id == 0 {
		return
	}
	linuxA11y.SetGeometry(atspi.WindowKey(w.wnd.id), w.linuxA11yGeometry())
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

// nativeAccessibilityAnnounce asks the platform's assistive technology to speak text.
// nativeAccessibilityEnabledChanged stops watching and serving when support is refused, and asks the desktop again
// whether an assistive technology is there when the refusal is lifted, since on this platform nothing else would ask.
func nativeAccessibilityEnabledChanged(enabled bool) {
	if enabled {
		linuxA11yStatusInit()
		return
	}
	linuxA11yTerminate()
}

func nativeAccessibilityAnnounce(text string) {
	if linuxA11y != nil {
		linuxA11y.Announce(text)
	}
}

// x11RefreshAccessibilityGeometry tells the window's assistive-technology adapter, if it has one, that the screen
// position, size or scale of its content area has changed. A window that is not being described pays one nil check.
func (w *Window) x11RefreshAccessibilityGeometry() {
	if w.ax != nil {
		w.apiAccessibilityGeometryChanged()
	}
}

// linuxA11yAction receives one request an assistive technology has made of a node.
//
// It arrives on the accessibility connection's dispatcher goroutine, and must not wait on the UI thread: that thread
// may be inside a modal loop or a drag, and libatspi's client-side timeout is two seconds. The request is therefore
// queued and returned from at once. The assistive technology has already been told, optimistically, that the request
// will be carried out, and learns what actually happened from the events the next snapshot produces.
func linuxA11yAction(req accessibility.ActionRequest) {
	InvokeTask(func() {
		// AT-SPI names an object without saying which window it is in — node ids are unique across the whole process, so
		// it does not have to — which leaves the window to be found from what was most recently published.
		for _, wnd := range windowList {
			if wnd.ax == nil {
				continue
			}
			if _, ok := wnd.ax.targets[req.Node]; ok {
				wnd.performAccessibilityAction(req)
				return
			}
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
	return linuxA11yGeometryFor(w.accessibilityGeometry())
}

// linuxA11yGeometryFor turns the screen origin and backing scale of a window's content area into the geometry the
// adapter converts node bounds with. It is separate from linuxA11yGeometry so that the conversion can be exercised
// without a window.
func linuxA11yGeometryFor(origin, scale geom.Point) atspi.Geometry {
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
