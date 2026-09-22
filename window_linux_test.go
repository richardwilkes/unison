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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/enums/thememode"
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
	// x11SetMinimized marks the window for redraw when an assistive technology is being served, and a window that has
	// been marked is drawn by the next pass of whatever event loop runs after this test. Whether one is being served is
	// package state that an earlier test may have left on, so it is pinned off here rather than trusted.
	priorActive := accessibilityActive.Swap(false)
	t.Cleanup(func() { accessibilityActive.Store(priorActive) })
	w := &Window{valid: true, wnd: &apiWindow{}}
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

// TestX11MinimizeMarksTheWindowForPublish covers what x11SetMinimized does for an assistive technology: a window the
// window manager has iconified, or restored, is marked for redraw so that the event loop withdraws or republishes its
// description, exactly as Window.Minimize does. Without it only a minimized window that held the keyboard focus would be
// withdrawn, by the FocusOut that follows, and a background window a person cannot see would go on being listed.
func TestX11MinimizeMarksTheWindowForPublish(t *testing.T) {
	c := check.New(t)
	priorActive, priorWake := accessibilityActive.Swap(true), redrawWakePending
	w := &Window{valid: true, wnd: &apiWindow{}}
	t.Cleanup(func() {
		accessibilityActive.Store(priorActive)
		redrawWakePending = priorWake
		delete(redrawSet, w)
	})
	marked := func() bool {
		_, ok := redrawSet[w]
		return ok
	}

	w.x11SetMinimized(true)
	c.True(marked(), "a window the window manager iconified must be described again")

	// A repeat of the state already in effect changes nothing and so marks nothing.
	delete(redrawSet, w)
	w.x11SetMinimized(true)
	c.False(marked(), "a repeat of the state in effect must not mark the window")

	w.x11SetMinimized(false)
	c.True(marked(), "a window the window manager restored must be described again")

	// Nothing is marked when no assistive technology is being served: that is the whole cost of the support to an
	// application nothing is watching.
	delete(redrawSet, w)
	accessibilityActive.Store(false)
	w.x11SetMinimized(true)
	c.False(marked(), "without an assistive technology a minimize must not mark the window")
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
	// x11RequestErrorCode is the Request error, which is what the X server answers a request it does not understand
	// with, and what the stand-in below answers everything it was not built to serve with.
	x11RequestErrorCode = 1
)

// x11ChangePropertyOpcode and x11DeletePropertyOpcode are the requests that set a property on a window and remove one
// from it, which the stand-in X server records rather than answers, since the X server sends no reply to either.
const (
	x11ChangePropertyOpcode = 18
	x11DeletePropertyOpcode = 19
)

// x11GetInputFocusOpcode is the request that asks which window has the keyboard focus. The connection sends one after
// each request that can fail without a reply, such as DeleteProperty, so that the reply to it says that the request
// before it did not fail.
const x11GetInputFocusOpcode = 43

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

// x11PropertyChange is one ChangeProperty or DeleteProperty request that the stand-in X server was sent. A deletion
// carries only the window and the property.
type x11PropertyChange struct {
	data         string
	window       x11.WindowID
	property     x11.Atom
	propertyType x11.Atom
	format       byte
	mode         byte
	deleted      bool
}

// x11TestServer stands in for the X server for the length of a test: it answers the TranslateCoordinates requests the
// event handling makes by adding the frame's origin to the position it was given, which is what a reparenting window
// manager's frame does to a real one, and records each of them so that a test can see whether one was made at all. It
// records each ChangeProperty and DeleteProperty request too, which need no answer, so that a test can see what was
// done to a window's properties, and answers the GetInputFocus that follows a DeleteProperty with no window, since all
// the connection wants of it is a reply. [x11.NewConnForTest] wires the connection the platform code uses to it, and internal/x11's own
// TestNewConnForTestServesRequests is where the bytes below are checked, since nothing in this file builds anywhere but
// Linux.
//
// Those are the only requests it serves, and the tests keep it that way by never marking a window valid:
// ContentRect and BackingScale answer for an invalid window without asking the X server anything, which is what keeps
// the event handling's moved() and the geometry refresh from making the GetGeometry, TranslateCoordinates and
// ContentScale requests they would make for a real window. Any other request is a regression, and is failed and
// answered with an error rather than ignored, since one that expects a reply would otherwise wait for it forever and
// hang the test instead of failing it.
type x11TestServer struct {
	t       *testing.T
	side    net.Conn
	asked   chan x11Translation
	changed chan x11PropertyChange
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
	s := &x11TestServer{
		t:       t,
		side:    serverSide,
		asked:   make(chan x11Translation, 16),
		changed: make(chan x11PropertyChange, 16),
	}
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
		switch header[0] {
		case x11ChangePropertyOpcode:
			s.recordPropertyChange(header[1], body)
			continue
		case x11DeletePropertyOpcode:
			// The body is the window and then the property.
			s.record(x11PropertyChange{
				window:   x11.WindowID(binary.LittleEndian.Uint32(body[0:4])),
				property: x11.Atom(binary.LittleEndian.Uint32(body[4:8])),
				deleted:  true,
			})
			continue
		case x11GetInputFocusOpcode:
			// A reply is 32 bytes: the code, the byte the reply's own first value occupies — here what the focus reverts
			// to — the sequence, the count of any additional words, then the window with the focus, which is none.
			reply := make([]byte, 32)
			reply[0] = 1
			binary.LittleEndian.PutUint16(reply[2:4], seq)
			if _, err := s.side.Write(reply); err != nil {
				return
			}
			continue
		}
		if header[0] != x11TranslateCoordinatesOpcode {
			s.t.Errorf("the X server was sent request %d, which nothing in these tests expects it to serve", header[0])
			if !s.writeError(seq, header[0]) {
				return
			}
			continue
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

// recordPropertyChange records a ChangeProperty request from the mode its header carries and its body: the window, the
// property, its type, the format (the number of bits in each unit of the data), three bytes of padding, the number of
// units, then the data itself, padded to a multiple of four bytes.
func (s *x11TestServer) recordPropertyChange(mode byte, body []byte) {
	change := x11PropertyChange{
		window:       x11.WindowID(binary.LittleEndian.Uint32(body[0:4])),
		property:     x11.Atom(binary.LittleEndian.Uint32(body[4:8])),
		propertyType: x11.Atom(binary.LittleEndian.Uint32(body[8:12])),
		format:       body[12],
		mode:         mode,
	}
	size := int(binary.LittleEndian.Uint32(body[16:20])) * int(change.format/8)
	if size > len(body)-20 {
		s.t.Errorf("a ChangeProperty request claimed %d bytes of data but carried only %d", size, len(body)-20)
		return
	}
	change.data = string(body[20 : 20+size])
	s.record(change)
}

// record keeps a property change for propertyChanges to report.
func (s *x11TestServer) record(change x11PropertyChange) {
	select {
	case s.changed <- change:
	default:
		s.t.Errorf("more property changes were made than the stand-in X server can hold, so %+v was lost", change)
	}
}

// writeError answers a request with a Request error, which is what a real X server sends for a request it does not
// understand. The connection under test hands it to whoever is waiting on the request, so one that expects a reply
// returns an error at once rather than waiting for a reply that will never come; one that expects nothing has the error
// queued as an event nobody reads. It returns false when the connection under test has gone away.
func (s *x11TestServer) writeError(seq uint16, opcode byte) bool {
	// An error is 32 bytes: zero, the error code, the sequence, the value the error is about, the minor opcode, the
	// major opcode, then padding.
	e := make([]byte, 32)
	e[1] = x11RequestErrorCode
	binary.LittleEndian.PutUint16(e[2:4], seq)
	e[10] = opcode
	_, err := s.side.Write(e)
	return err == nil
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

// propertyChanges returns the property changes the code under test has made since the last call, in the order they
// were made, or nil if it made none. The X server takes requests in the order they were sent, and the stand-in records
// each one before it reads the next, so the reply to a round trip made after them cannot arrive before every one of
// them has been recorded. That is what makes it safe to say that nothing else was sent.
func (s *x11TestServer) propertyChanges() []x11PropertyChange {
	s.t.Helper()
	if _, _, _, _, err := x11Conn.TranslateCoordinates(x11TestRootWindow, x11TestRootWindow, 0, 0); err != nil {
		s.t.Fatalf("the round trip made to wait for the property changes to be recorded failed: %v", err)
	}
	select {
	case <-s.asked: // The round trip's own request, which is not one the code under test made
	default:
	}
	var changes []x11PropertyChange
	for {
		select {
		case change := <-s.changed:
			changes = append(changes, change)
		default:
			return changes
		}
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
	w := &Window{wnd: &apiWindow{}}
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
	if len(args) != 5 {
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
	w := &Window{wnd: &apiWindow{}}
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

// TestX11FrameThemeFollowsDarkMode verifies the requests that ask the window manager for a frame that matches the
// application's dark mode state. _GTK_THEME_VARIANT is set to "dark" or "light" whenever the state changes, with light
// named rather than left out, since a window manager gives a window without the property the desktop's preferred
// variant. _KDE_NET_WM_COLOR_SCHEME is set to the path of Breeze's matching scheme only while the application's state
// differs from the desktop's, or the desktop's cannot be determined, and is removed once they match again, since KWin
// draws a frame without it in the desktop's own scheme. A window with no frame to theme, and one with no X window, must
// be sent nothing.
func TestX11FrameThemeFollowsDarkMode(t *testing.T) {
	c := check.New(t)
	server := startX11TestServer(t)
	priorMode, priorSchemes := CurrentThemeMode(), x11KDEColorSchemes
	priorDark, priorTrackable := linuxDarkModeEnabled.Load(), linuxColorModeTrackable.Load()
	t.Cleanup(func() {
		currentThemeMode.Store(int32(priorMode))
		x11KDEColorSchemes = priorSchemes
		linuxDarkModeEnabled.Store(priorDark)
		linuxColorModeTrackable.Store(priorTrackable)
	})
	// The stand-in connection interns no atoms, so the ones the requests name are given values they can be checked by.
	const (
		themeVariantAtom = x11.Atom(301)
		colorSchemeAtom  = x11.Atom(302)
		utf8StringAtom   = x11.Atom(303)
		lightScheme      = "/usr/share/color-schemes/BreezeLight.colors"
		darkScheme       = "/usr/share/color-schemes/BreezeDark.colors"
	)
	x11Conn.Atoms.GTKThemeVariant = themeVariantAtom
	x11Conn.Atoms.KDENetWMColorScheme = colorSchemeAtom
	x11Conn.Atoms.UTF8String = utf8StringAtom
	x11KDEColorSchemes = func() (light, dark string) { return lightScheme, darkScheme }
	variantSet := func(value string) x11PropertyChange {
		return x11PropertyChange{
			data:         value,
			window:       x11TestWindowID,
			property:     themeVariantAtom,
			propertyType: utf8StringAtom,
			format:       8,
			mode:         x11.PropModeReplace,
		}
	}
	schemeSet := func(path string) x11PropertyChange {
		return x11PropertyChange{
			data:         path,
			window:       x11TestWindowID,
			property:     colorSchemeAtom,
			propertyType: x11.AtomString,
			format:       8,
			mode:         x11.PropModeReplace,
		}
	}
	schemeRemoved := x11PropertyChange{window: x11TestWindowID, property: colorSchemeAtom, deleted: true}
	var nothing []x11PropertyChange
	// update puts the application in mode and the desktop in the given state, and returns what the window's frame theme
	// update asked of the X server.
	update := func(w *Window, mode thememode.Enum, desktopDark, desktopKnown bool) []x11PropertyChange {
		currentThemeMode.Store(int32(mode))
		linuxDarkModeEnabled.Store(desktopDark)
		linuxColorModeTrackable.Store(desktopKnown)
		w.nativeUpdateFrameTheme()
		return server.propertyChanges()
	}
	w := &Window{wnd: &apiWindow{}}
	w.wnd.id = x11TestWindowID

	c.Equal([]x11PropertyChange{variantSet("light")}, update(w, thememode.Light, false, true),
		"a light window on a light desktop needs only the theme variant, since KWin's default scheme already matches")
	c.Equal([]x11PropertyChange{variantSet("dark"), schemeSet(darkScheme)}, update(w, thememode.Dark, false, true),
		"a dark window on a light desktop must ask KWin for a dark scheme as well")
	c.Equal(nothing, update(w, thememode.Dark, false, true), "nothing changed, so nothing must be sent")
	c.Equal([]x11PropertyChange{schemeRemoved}, update(w, thememode.Dark, true, true),
		"once the desktop is dark too, its own scheme matches, so the one asked for must be removed")
	c.Equal([]x11PropertyChange{variantSet("light"), schemeSet(lightScheme)}, update(w, thememode.Light, true, true),
		"a light window on a dark desktop must ask KWin for a light scheme as well")
	c.Equal([]x11PropertyChange{schemeRemoved}, update(w, thememode.Light, false, true),
		"once the desktop is light too, the scheme asked for must be removed")
	c.Equal([]x11PropertyChange{schemeSet(lightScheme)}, update(w, thememode.Light, false, false),
		"when the desktop's state is unknown, nothing says its scheme matches, so the scheme must be asked for")

	// Without Breeze's schemes there is nothing to ask KWin for, so the frame is left to the desktop's scheme.
	x11KDEColorSchemes = func() (light, dark string) { return "", "" }
	plain := &Window{wnd: &apiWindow{}}
	plain.wnd.id = x11TestWindowID
	c.Equal([]x11PropertyChange{variantSet("dark")}, update(plain, thememode.Dark, false, true),
		"without a scheme to name, only the theme variant can be set")

	// A light window on a dark desktop would need both properties, so these show that nothing at all is sent for a
	// window without a frame, or one that has no X window.
	x11KDEColorSchemes = func() (light, dark string) { return lightScheme, darkScheme }
	undecorated := &Window{wnd: &apiWindow{}, undecorated: true}
	undecorated.wnd.id = x11TestWindowID
	c.Equal(nothing, update(undecorated, thememode.Light, true, true), "a window without a frame must be sent nothing")
	c.Equal(nothing, update(&Window{wnd: &apiWindow{}}, thememode.Light, true, true),
		"a window that has no X window must be sent nothing")
}

// TestLinuxXDGDataDirs verifies the directories a data file is looked for in: the user's own and then the system's,
// with the defaults the XDG Base Directory Specification gives for either that is not set, and without any relative
// path, which the specification says to ignore.
func TestLinuxXDGDataDirs(t *testing.T) {
	c := check.New(t)
	t.Setenv("XDG_DATA_HOME", "/home/someone/data")
	t.Setenv("XDG_DATA_DIRS", "/opt/share:relative/share::/usr/share")
	c.Equal([]string{"/home/someone/data", "/opt/share", "/usr/share"}, linuxXDGDataDirs())

	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_DATA_DIRS", "")
	c.Equal([]string{filepath.Join(xos.HomeDir(), ".local", "share"), "/usr/local/share", "/usr/share"},
		linuxXDGDataDirs())

	t.Setenv("XDG_DATA_HOME", "relative/data")
	c.Equal([]string{"/usr/local/share", "/usr/share"}, linuxXDGDataDirs())
}

// TestLinuxFindDataFile verifies that a data file is taken from the first directory that holds it, so that a user's own
// copy of a file takes the place of the system's, and that a file none of them holds is not found.
func TestLinuxFindDataFile(t *testing.T) {
	c := check.New(t)
	const name = "color-schemes/BreezeDark.colors"
	empty, user, system := t.TempDir(), t.TempDir(), t.TempDir()
	dirs := []string{empty, user, system}
	install := func(dir string) string {
		p := filepath.Join(dir, name)
		c.NoError(os.MkdirAll(filepath.Dir(p), 0o755))
		c.NoError(os.WriteFile(p, []byte("[General]\n"), 0o644))
		return p
	}
	c.Equal("", linuxFindDataFile(dirs, name), "a file none of the directories holds must not be found")
	systemPath := install(system)
	c.Equal(systemPath, linuxFindDataFile(dirs, name))
	userPath := install(user)
	c.Equal(userPath, linuxFindDataFile(dirs, name), "the user's own copy must take the place of the system's")
	c.Equal("", linuxFindDataFile(dirs, "color-schemes/BreezeLight.colors"))
	// A directory by the name is not the file.
	c.NoError(os.MkdirAll(filepath.Join(empty, "color-schemes", "BreezeLight.colors"), 0o755))
	c.Equal("", linuxFindDataFile(dirs, "color-schemes/BreezeLight.colors"), "a directory must not be taken for the file")
}
