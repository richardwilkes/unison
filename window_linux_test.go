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
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/atspi"
	"github.com/richardwilkes/unison/internal/dbus"
	"github.com/richardwilkes/unison/internal/x11"
)

func TestWMClassData(t *testing.T) {
	c := check.New(t)

	savedCmd := xos.AppCmdName
	savedName := xos.AppName
	savedID := xos.AppIdentifier
	defer func() {
		xos.AppCmdName = savedCmd
		xos.AppName = savedName
		xos.AppIdentifier = savedID
	}()

	// Normal case: instance is the command name, class is the identifier. WM_CLASS is a pair of null-terminated
	// strings, so a .desktop file's StartupWMClass entry (which matches the class name) can associate with the window.
	xos.AppCmdName = "gcs"
	xos.AppName = "GCS"
	xos.AppIdentifier = "com.trollworks.gcs"
	c.Equal("gcs\x00com.trollworks.gcs\x00", string(wmClassData()))

	// Falls back to the application name when the command name is empty.
	xos.AppCmdName = ""
	c.Equal("GCS\x00com.trollworks.gcs\x00", string(wmClassData()))

	// Falls back to the instance name when the identifier is empty.
	xos.AppCmdName = "gcs"
	xos.AppIdentifier = ""
	c.Equal("gcs\x00gcs\x00", string(wmClassData()))
}

// TestX11WindowStateIsVisibleToPublicAPI verifies that the WM_STATE and _NET_WM_STATE property handlers record the
// state where IsMinimized and IsMaximized read it. The handlers used to write a platform-private copy of the flags
// instead, so those public methods always reported false on Linux no matter what the window manager did.
func TestX11WindowStateIsVisibleToPublicAPI(t *testing.T) {
	c := check.New(t)
	w := &Window{valid: true}
	var minimizedCalls, maximizedCalls []bool
	w.MinimizedCallback = func(minimized bool) { minimizedCalls = append(minimizedCalls, minimized) }
	w.MaximizedCallback = func(maximized bool) { maximizedCalls = append(maximizedCalls, maximized) }
	c.False(w.IsMinimized())
	c.False(w.IsMaximized())

	w.x11SetMinimized(true)
	c.True(w.IsMinimized())
	c.Equal([]bool{true}, minimizedCalls)

	// A repeat of the state already in effect must not re-notify.
	w.x11SetMinimized(true)
	c.Equal([]bool{true}, minimizedCalls)

	w.x11SetMinimized(false)
	c.False(w.IsMinimized())
	c.Equal([]bool{true, false}, minimizedCalls)

	w.x11SetMaximized(true)
	c.True(w.IsMaximized())
	c.Equal([]bool{true}, maximizedCalls)
	w.x11SetMaximized(true)
	c.Equal([]bool{true}, maximizedCalls)
	w.x11SetMaximized(false)
	c.False(w.IsMaximized())
	c.Equal([]bool{true, false}, maximizedCalls)
}

// The X protocol constants the ConfigureNotify test needs. They are the wire's own numbers rather than anything
// internal/x11 decides, and that package keeps its copies of them to itself: [x11.Synthetic] is the only way in from
// out here to what the high bit of an event code means, which is why [x11ConfigureNotify] checks the event it builds
// against it rather than trusting the two below to still agree.
const (
	// x11TranslateCoordinatesOpcode is the request that asks the X server where one window's coordinates land in
	// another, which is what a window whose parent is a window manager's frame has to ask to know where it is.
	x11TranslateCoordinatesOpcode = 40
	// x11ConfigureNotifyCode is the ConfigureNotify event code, and x11SyntheticEventFlag the high bit the X server
	// sets on an event another client sent rather than the server generating it.
	x11ConfigureNotifyCode = 22
	x11SyntheticEventFlag  = 0x80
)

// The windows the ConfigureNotify test works with, and where the window manager's frame sits on the root window: a
// real ConfigureNotify carries a position relative to the frame, so the frame's own origin is what translating one
// adds to it.
const (
	x11TestRootWindow   = x11.WindowID(100)
	x11TestFrameWindow  = x11.WindowID(7)
	x11TestWindowID     = x11.WindowID(42)
	x11TestFrameOriginX = 60
	x11TestFrameOriginY = 30
)

// x11Translation is one TranslateCoordinates request that the stand-in X server was asked to answer.
type x11Translation struct {
	src, dst x11.WindowID
	x, y     int16
}

// x11TestServer stands in for the X server for the length of a test: it answers the TranslateCoordinates requests the
// event handling makes by adding the frame's origin to the position it was given, which is what a reparenting window
// manager's frame does to a real one, and records each of them so that a test can see whether one was made at all.
// [x11.NewConnForTest] wires the connection the platform code uses to it, and internal/x11's own
// TestNewConnForTestServesRequests is where the bytes below are checked, since nothing in this file builds anywhere but
// Linux.
type x11TestServer struct {
	t     *testing.T
	side  net.Conn
	asked chan x11Translation
}

// startX11TestServer installs a connection to a stand-in X server as the one the platform code uses, putting the
// previous connection and window list back when the test finishes.
func startX11TestServer(t *testing.T) *x11TestServer {
	t.Helper()
	clientSide, serverSide := net.Pipe()
	conn, stop := x11.NewConnForTest(clientSide, []x11.Screen{{Root: x11TestRootWindow}})
	priorConn, priorWindows := x11Conn, windowList
	x11Conn = conn
	t.Cleanup(func() {
		x11Conn, windowList = priorConn, priorWindows
		stop()
		xio.CloseIgnoringErrors(serverSide)
	})
	s := &x11TestServer{t: t, side: serverSide, asked: make(chan x11Translation, 16)}
	go s.run()
	return s
}

// run answers requests until the connection under test goes away.
func (s *x11TestServer) run() {
	var seq uint16
	header := make([]byte, 4)
	for {
		if _, err := io.ReadFull(s.side, header); err != nil {
			return // The connection under test has gone away, which the end of a test does on purpose
		}
		seq++
		body := make([]byte, int(binary.LittleEndian.Uint16(header[2:4]))*4-len(header))
		if _, err := io.ReadFull(s.side, body); err != nil {
			return
		}
		if header[0] != x11TranslateCoordinatesOpcode {
			continue // Nothing else the code under test sends here expects an answer
		}
		asked := x11Translation{
			src: x11.WindowID(binary.LittleEndian.Uint32(body[0:4])),
			dst: x11.WindowID(binary.LittleEndian.Uint32(body[4:8])),
			x:   int16(binary.LittleEndian.Uint16(body[8:10])),
			y:   int16(binary.LittleEndian.Uint16(body[10:12])),
		}
		select {
		case s.asked <- asked:
		default:
		}
		// A reply is 32 bytes: the code, the byte the reply's own first value occupies — here whether the two windows
		// are on the same screen — the sequence, the count of any additional words, then the child the position falls
		// in and the position itself.
		reply := make([]byte, 32)
		reply[0] = 1
		reply[1] = 1
		binary.LittleEndian.PutUint16(reply[2:4], seq)
		translatedX, translatedY := asked.x+x11TestFrameOriginX, asked.y+x11TestFrameOriginY
		binary.LittleEndian.PutUint16(reply[12:14], uint16(translatedX))
		binary.LittleEndian.PutUint16(reply[14:16], uint16(translatedY))
		if _, err := s.side.Write(reply); err != nil {
			return
		}
	}
}

// nextTranslation returns the request the code under test made of the X server.
func (s *x11TestServer) nextTranslation() x11Translation {
	s.t.Helper()
	select {
	case asked := <-s.asked:
		return asked
	case <-time.After(a11yTestTimeout):
		s.t.Fatal("timed out waiting for the X server to be asked to translate a position")
		return x11Translation{}
	}
}

// nothingTranslated fails the test if the X server was asked to translate anything. It is safe to check without
// waiting: x11ProcessEvent makes the request, if it makes one at all, before it returns, and the pipe the request would
// travel on is read by the server before anything else could happen.
func (s *x11TestServer) nothingTranslated() {
	s.t.Helper()
	select {
	case asked := <-s.asked:
		s.t.Errorf("the X server was asked to translate %v, which a synthetic ConfigureNotify must not do", asked)
	default:
	}
}

// x11ConfigureNotify builds a ConfigureNotify for the test window at the given position. synthetic selects the one a
// window manager sends over the one the X server generates.
func x11ConfigureNotify(t *testing.T, x, y int16, synthetic bool) *x11.ConfigureNotifyEvent {
	t.Helper()
	ev := &x11.ConfigureNotifyEvent{
		Code:   x11ConfigureNotifyCode,
		Event:  x11TestWindowID,
		Window: x11TestWindowID,
		X:      x,
		Y:      y,
		Width:  400,
		Height: 300,
	}
	if synthetic {
		ev.Code |= x11SyntheticEventFlag
	}
	check.New(t).Equal(synthetic, x11.Synthetic(ev), "the event code must say what the test means it to")
	return ev
}

// TestX11ConfigureNotifyTranslatesOnlyRealEvents covers the decision the X11 event loop makes about every
// ConfigureNotify. The position one the X server generated carries is relative to the window's parent, which under a
// reparenting window manager is the frame it put around the window, so it has to be translated; a synthetic one — what
// the window manager sends when it has moved a window without resizing it — already carries the position relative to
// the root and must not be, or the frame's origin is added to it a second time and the window is reported somewhere it
// is not. An assistive technology draws its highlight from that origin, and so does everything the application is told
// about where it sits on the screen.
//
// The window is given a live adapter with a window in it, so that each event is followed all the way to the assistive
// technology rather than only as far as the move callback. The two halves are what the origin is for and where it
// comes from: without the adapter, deleting the call that hands it over leaves everything here green, and without the
// event loop, nothing says the origin handed over is the translated one rather than the position the event carried.
func TestX11ConfigureNotifyTranslatesOnlyRealEvents(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	server := startX11TestServer(t)
	bus := newFakeA11yBus(t)
	adapter, err := atspi.Start(atspi.Config{BusAddress: bus.address, ToolkitVersion: "test"})
	c.NoError(err)
	if adapter == nil {
		t.Fatal("the fake accessibility bus should have produced an adapter")
	}
	t.Cleanup(adapter.Stop)
	linuxA11y = adapter
	w := &Window{}
	w.root = &rootPanel{}
	w.wnd.id = x11TestWindowID
	w.wnd.parent = x11TestFrameWindow
	w.ax = &windowAccessibility{}
	adapter.Publish(atspi.WindowKey(x11TestWindowID), &accessibility.Tree{
		Root:       1,
		Generation: 1,
		Nodes: map[accessibility.NodeID]*accessibility.Node{
			1: {ID: 1, Role: role.Window, Name: "configured", Bounds: geom.NewRect(0, 0, 400, 300)},
		},
	}, nil, atspi.Geometry{Scale: geom.NewPoint(1, 1)})
	// The size the events below carry, so that nothing is resized and this is only about the position.
	w.lastWidth, w.lastHeight = 400, 300
	var moves []geom.Point
	w.MovedCallback = func() { moves = append(moves, geom.NewPoint(w.wnd.lastX, w.wnd.lastY)) }
	windowList = []*Window{w}

	// A real one is translated, so the window ends up at the frame's origin plus the position the event carried.
	w.wnd.awaitingConfigure = true
	x11ProcessEvent(x11ConfigureNotify(t, 10, 20, false))
	c.False(w.wnd.awaitingConfigure, "a ConfigureNotify is the answer that was being waited for")
	c.Equal(x11Translation{src: x11TestFrameWindow, dst: x11TestRootWindow, x: 10, y: 20}, server.nextTranslation())
	c.Equal([]geom.Point{geom.NewPoint(x11TestFrameOriginX+10, x11TestFrameOriginY+20)}, moves,
		"a real ConfigureNotify must report the position the X server translated")
	c.Equal(x11Extents(x11TestFrameOriginX+10, x11TestFrameOriginY+20), x11AnnouncedExtents(t, bus),
		"the translated origin is what the assistive technology must be given")

	// A synthetic one is not, so the window ends up exactly where the event said.
	x11ProcessEvent(x11ConfigureNotify(t, 11, 21, true))
	server.nothingTranslated()
	c.Equal(2, len(moves), "the synthetic event moved the window somewhere else, so it must have been reported")
	c.Equal(geom.NewPoint(11, 21), moves[1], "a synthetic ConfigureNotify must be taken at face value")
	c.Equal(x11Extents(11, 21), x11AnnouncedExtents(t, bus),
		"and the position a synthetic event carried must reach the assistive technology untranslated")

	// A window whose parent is the root window has nothing to translate either way, since the position is already
	// relative to the root: that is the unreparented case, and it must not cost a round trip.
	w.wnd.parent = x11TestRootWindow
	x11ProcessEvent(x11ConfigureNotify(t, 12, 22, false))
	server.nothingTranslated()
	c.Equal(geom.NewPoint(12, 22), moves[2], "a window parented to the root reports the position it was given")
	c.Equal(x11Extents(12, 22), x11AnnouncedExtents(t, bus))
}

// x11Extents is the value a BoundsChanged signal carries for the test window at the given origin: the position and the
// size of the window's content, in the X server's pixels.
func x11Extents(x, y int32) dbus.Variant {
	return dbus.Variant{Sig: "(iiii)", Value: dbus.Struct{x, y, int32(400), int32(300)}}
}

// x11AnnouncedExtents returns the extents the next BoundsChanged the adapter sent carries, which is where an assistive
// technology has just been told the window's content sits on the screen. Nothing else in these tests sends that
// signal — it comes from SetGeometry and from a bounds change in a published snapshot, and none is published after the
// first — so the next one is always the one the event under test produced.
func x11AnnouncedExtents(t *testing.T, bus *fakeA11yBus) any {
	t.Helper()
	args := bus.awaitSignal(atspi.InterfaceEventObject, "BoundsChanged")
	if len(args) < 4 {
		t.Fatalf("a BoundsChanged signal carries five values, but this one carried %d", len(args))
	}
	return args[3]
}

// TestX11RefreshAccessibilityGeometryReachesTheAdapter covers what the ConfigureNotify handling does with the origin it
// worked out: it hands it to the window's assistive-technology adapter, which is where the coordinates an assistive
// technology draws its highlight from come from. Only a live adapter can show that, so one is built on a fake
// accessibility bus and the signal it sends is what says the geometry arrived; what SetGeometry then makes of it is
// internal/atspi's own business, and tested there.
func TestX11RefreshAccessibilityGeometryReachesTheAdapter(t *testing.T) {
	c := check.New(t)
	saveA11yState(t)
	bus := newFakeA11yBus(t)
	adapter, err := atspi.Start(atspi.Config{BusAddress: bus.address, ToolkitVersion: "test"})
	c.NoError(err)
	if adapter == nil {
		t.Fatal("the fake accessibility bus should have produced an adapter")
	}
	t.Cleanup(adapter.Stop)
	linuxA11y = adapter
	key := atspi.WindowKey(x11TestWindowID)
	adapter.Publish(key, &accessibility.Tree{
		Root:       1,
		Generation: 1,
		Nodes: map[accessibility.NodeID]*accessibility.Node{
			1: {ID: 1, Role: role.Window, Name: "geometry", Bounds: geom.NewRect(0, 0, 200, 150)},
		},
	}, nil, atspi.Geometry{Scale: geom.NewPoint(1, 1)})
	w := &Window{}
	w.wnd.id = x11TestWindowID
	w.ax = &windowAccessibility{}

	// The window's own extents are the one set of coordinates an assistive technology is told rather than asked for, so
	// a window that has moved announces them. Nothing else ever sends this signal — it comes from SetGeometry and from
	// a bounds change in a published snapshot, and none is published here — so the next one is the one this produced.
	w.x11RefreshAccessibilityGeometry(geom.NewPoint(300, 200))
	c.Equal(dbus.Variant{Sig: "(iiii)", Value: dbus.Struct{int32(300), int32(200), int32(200), int32(150)}},
		x11AnnouncedExtents(t, bus),
		"the window must be reported at the origin the ConfigureNotify handling worked out")

	// Each of the three things that make the call a no-op has to actually make it one: a window nothing is describing,
	// no adapter at all, and a window the X server has no id for. Signals arrive in the order they were sent, so the
	// one that follows them being the last origin is what says none of them announced anything.
	w.ax = nil
	w.x11RefreshAccessibilityGeometry(geom.NewPoint(11, 11))
	w.ax = &windowAccessibility{}
	linuxA11y = nil
	w.x11RefreshAccessibilityGeometry(geom.NewPoint(22, 22))
	linuxA11y = adapter
	w.wnd.id = 0
	w.x11RefreshAccessibilityGeometry(geom.NewPoint(33, 33))
	w.wnd.id = x11TestWindowID
	w.x11RefreshAccessibilityGeometry(geom.NewPoint(44, 44))
	c.Equal(dbus.Variant{Sig: "(iiii)", Value: dbus.Struct{int32(44), int32(44), int32(200), int32(150)}},
		x11AnnouncedExtents(t, bus),
		"a window that is not being described, or has no adapter or id, must announce nothing")
}
