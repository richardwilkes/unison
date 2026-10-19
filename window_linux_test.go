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
	"sync/atomic"
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

// x11GrabPointerOpcode and x11UngrabPointerOpcode are the requests that grab the pointer and release a grab, which the
// stand-in X server records, granting every grab.
const (
	x11GrabPointerOpcode   = 26
	x11UngrabPointerOpcode = 27
)

// x11DestroyWindowOpcode and x11UnmapWindowOpcode are the requests destroying a window sends, which the stand-in X
// server accepts without recording, since the X server sends no reply to either.
const (
	x11DestroyWindowOpcode = 4
	x11UnmapWindowOpcode   = 10
)

// x11ButtonReleaseCode is the ButtonRelease event code.
const x11ButtonReleaseCode = 5

// x11QueryPointerOpcode is the request that asks where the pointer is and which buttons are down, which the stand-in X
// server answers with no position and the button state a test has set, and x11TestButton1Mask is Button1Mask in that
// state.
const (
	x11QueryPointerOpcode = 38
	x11TestButton1Mask    = 0x100
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

// x11Grab is one GrabPointer or UngrabPointer request that the stand-in X server was sent. A release carries nothing
// else.
type x11Grab struct {
	window      x11.WindowID
	cursor      x11.CursorID
	eventMask   uint16
	ownerEvents bool
	released    bool
}

// x11TestServer stands in for the X server for the length of a test: it answers the TranslateCoordinates requests the
// event handling makes by adding the frame's origin to the position it was given, which is what a reparenting window
// manager's frame does to a real one, and records each of them so that a test can see whether one was made at all. It
// records each ChangeProperty and DeleteProperty request too, which need no answer, so that a test can see what was
// done to a window's properties, and answers the GetInputFocus that follows a DeleteProperty with no window, since all
// the connection wants of it is a reply. It records each GrabPointer, granting it unless refuseGrabs is set, and each
// UngrabPointer, and answers QueryPointer with the button state in pointerMask.
// [x11.NewConnForTest] wires the connection the platform code uses to it, and internal/x11's own
// TestNewConnForTestServesRequests is where the bytes below are checked, since nothing in this file builds anywhere but
// Linux.
//
// Those are the only requests it serves, and the tests keep it that way by never marking valid a window whose geometry
// is asked for: ContentRect and BackingScale answer for an invalid window without asking the X server anything, which
// is what keeps the event handling's moved() and the geometry refresh from making the GetGeometry, TranslateCoordinates
// and ContentScale requests they would make for a real window. Any other request is a regression, and is failed and
// answered with an error rather than ignored, since one that expects a reply would otherwise wait for it forever and
// hang the test instead of failing it.
type x11TestServer struct {
	t           *testing.T
	side        net.Conn
	asked       chan x11Translation
	changed     chan x11PropertyChange
	grabbed     chan x11Grab
	pointerMask atomic.Uint32
	refuseGrabs atomic.Bool
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
		grabbed: make(chan x11Grab, 16),
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
		case x11DestroyWindowOpcode, x11UnmapWindowOpcode:
			continue
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
		case x11GrabPointerOpcode:
			// The header's second byte is owner-events. The body is the grab window, the event mask, the pointer and
			// keyboard modes, the window the pointer is confined to, the cursor, then the time.
			s.recordGrab(x11Grab{
				window:      x11.WindowID(binary.LittleEndian.Uint32(body[0:4])),
				cursor:      x11.CursorID(binary.LittleEndian.Uint32(body[12:16])),
				eventMask:   binary.LittleEndian.Uint16(body[4:6]),
				ownerEvents: header[1] != 0,
			})
			// A reply is 32 bytes: the code, the status, which is zero for a grab granted and one for a pointer another
			// client already holds, the sequence, then padding.
			reply := make([]byte, 32)
			reply[0] = 1
			if s.refuseGrabs.Load() {
				reply[1] = 1
			}
			binary.LittleEndian.PutUint16(reply[2:4], seq)
			if _, err := s.side.Write(reply); err != nil {
				return
			}
			continue
		case x11UngrabPointerOpcode:
			s.recordGrab(x11Grab{released: true})
			continue
		case x11QueryPointerOpcode:
			// A reply is 32 bytes: the code, whether on the same screen, the sequence, the count of any additional
			// words, the root, the child, the root and window positions, the button state, then padding.
			reply := make([]byte, 32)
			reply[0] = 1
			reply[1] = 1
			binary.LittleEndian.PutUint16(reply[2:4], seq)
			binary.LittleEndian.PutUint32(reply[8:12], uint32(x11TestRootWindow))
			binary.LittleEndian.PutUint16(reply[24:26], uint16(s.pointerMask.Load()))
			if _, err := s.side.Write(reply); err != nil {
				return
			}
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

// recordGrab keeps a grab or a release for grabs to report.
func (s *x11TestServer) recordGrab(grab x11Grab) {
	select {
	case s.grabbed <- grab:
	default:
		s.t.Errorf("more grabs were made than the stand-in X server can hold, so %+v was lost", grab)
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

// sendButtonRelease sends the code under test a ButtonRelease of button 1 reported to window, and waits until the
// connection has queued it.
func (s *x11TestServer) sendButtonRelease(window x11.WindowID) {
	s.t.Helper()
	// An event is 32 bytes: the code, the button, the sequence, the time, the root, the window the event is reported
	// to, the child, the root and window positions, the state, whether on the same screen, then padding.
	ev := make([]byte, 32)
	ev[0] = x11ButtonReleaseCode
	ev[1] = 1
	binary.LittleEndian.PutUint32(ev[8:12], uint32(x11TestRootWindow))
	binary.LittleEndian.PutUint32(ev[12:16], uint32(window))
	ev[30] = 1
	if _, err := s.side.Write(ev); err != nil {
		s.t.Fatalf("sending a ButtonRelease failed: %v", err)
	}
	s.sync()
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
	s.sync()
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

// grabs returns the grabs and releases the code under test has asked for since the last call, in the order they were
// made, or nil if it made none. See propertyChanges for why that is safe to say.
func (s *x11TestServer) grabs() []x11Grab {
	s.t.Helper()
	s.sync()
	var grabs []x11Grab
	for {
		select {
		case grab := <-s.grabbed:
			grabs = append(grabs, grab)
		default:
			return grabs
		}
	}
}

// sync makes a round trip to the X server, whose reply cannot arrive before every request sent ahead of it has been
// recorded.
func (s *x11TestServer) sync() {
	s.t.Helper()
	if _, _, _, _, err := x11Conn.TranslateCoordinates(x11TestRootWindow, x11TestRootWindow, 0, 0); err != nil {
		s.t.Fatalf("the round trip made to wait for the requests to be recorded failed: %v", err)
	}
	select {
	case <-s.asked: // The round trip's own request, which is not one the code under test made
	default:
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

// x11MenuGrabTestWindows installs an owner holding a menu grab, one of its popup menu windows and another top-level
// window as the window list, putting the previous list back when the test finishes. The owner's content is 400x300.
func x11MenuGrabTestWindows(t *testing.T) (owner, popup, other *Window) {
	t.Helper()
	priorWindows := windowList
	t.Cleanup(func() { windowList = priorWindows })
	owner = &Window{title: "owner", valid: true, wnd: &apiWindow{}, lastWidth: 400, lastHeight: 300}
	owner.wnd.menuGrab = true
	owner.wnd.pointerInside = true
	popup = &Window{title: "popup", valid: true, wnd: &apiWindow{}, owner: owner}
	other = &Window{title: "other", valid: true, wnd: &apiWindow{}}
	windowList = []*Window{owner, popup, other}
	return owner, popup, other
}

// TestX11MenuGrabInterceptsPress covers what becomes of a press while the menus hold the pointer: one on the owner's
// content goes through, one anywhere else closes the menus and is swallowed, and a wheel is swallowed without closing
// them.
func TestX11MenuGrabInterceptsPress(t *testing.T) {
	c := check.New(t)
	owner, popup, other := x11MenuGrabTestWindows(t)
	press := func(button byte, x, y int16) *x11.ButtonPressEvent {
		ev := &x11.ButtonPressEvent{}
		ev.Detail = button
		ev.EventX, ev.EventY = x, y
		ev.RootX, ev.RootY = x+1000, y+500
		return ev
	}
	var asked []geom.Point
	underIs := func(w *Window) func(rootX, rootY int16) *Window {
		return func(rootX, rootY int16) *Window {
			asked = append(asked, geom.NewPoint(float32(rootX), float32(rootY)))
			return w
		}
	}
	type result struct{ intercept, closeStack bool }
	decide := func(w *Window, ev *x11.ButtonPressEvent, under func(rootX, rootY int16) *Window) result {
		intercept, closeStack := x11MenuGrabInterceptsPress(w, ev, under)
		return result{intercept: intercept, closeStack: closeStack}
	}

	c.Equal(result{}, decide(owner, press(1, 10, 20), underIs(owner)), "a press on the owner's content goes through")
	c.Equal([]geom.Point{geom.NewPoint(1010, 520)}, asked, "the window on top is looked up at the root position")
	c.Equal(result{}, decide(owner, press(4, 10, 20), underIs(owner)), "so does a wheel over the owner's content")

	asked = nil
	c.Equal(result{intercept: true, closeStack: true}, decide(owner, press(1, -5, 20), underIs(owner)),
		"a press outside the owner closes the menus")
	c.Equal(result{intercept: true, closeStack: true}, decide(owner, press(3, 400, 20), underIs(owner)),
		"the owner's content ends before its width")
	c.Equal(result{intercept: true, closeStack: true}, decide(owner, press(2, 10, 300), underIs(owner)),
		"and before its height")
	c.Equal(result{intercept: true, closeStack: true}, decide(owner, press(8, 10, -1), underIs(owner)),
		"a press of a button above the wheel closes the menus too")
	c.Equal(0, len(asked), "a press outside the owner's bounds needs no lookup")
	for button := byte(4); button <= 7; button++ {
		c.Equal(result{intercept: true}, decide(owner, press(button, 500, 20), underIs(owner)),
			"a wheel outside the owner is swallowed without closing the menus")
	}

	c.Equal(result{intercept: true, closeStack: true}, decide(owner, press(1, 10, 20), underIs(nil)),
		"a press within the owner's bounds on another client's window on top closes the menus")
	c.Equal(result{intercept: true}, decide(owner, press(5, 10, 20), underIs(nil)),
		"a wheel there is swallowed without closing them")

	c.Equal(result{}, decide(popup, press(1, -5, -5), underIs(nil)), "a popup menu window's presses are its own")
	c.Equal(result{intercept: true, closeStack: true}, decide(other, press(1, 10, 20), underIs(other)),
		"a press on another top-level window closes the menus, since the grab kept it from being activated")
	c.Equal(result{}, decide(other, press(4, 10, 20), underIs(other)), "a wheel there scrolls it")

	owner.wnd.menuGrab = false
	c.Equal(result{}, decide(owner, press(1, -5, 20), underIs(nil)), "without a menu grab, nothing is intercepted")
	c.Equal(result{}, decide(other, press(1, 10, 20), underIs(nil)))
}

// TestX11PointerTargetFollowsThePress covers where a motion or release goes while the menus hold the pointer: a release
// to the window recording its button as down, a motion to the window holding a press, as the press's implicit grab
// would have sent them, and otherwise to the window X delivered it to, except that one reaching the owner from outside
// it is over another client's window and is dropped. One X reported to a window that is gone goes where that window
// would have sent it.
func TestX11PointerTargetFollowsThePress(t *testing.T) {
	c := check.New(t)
	owner, popup, other := x11MenuGrabTestWindows(t)

	c.Equal(owner, x11PointerTarget(owner, true, -1))
	c.Equal(popup, x11PointerTarget(popup, true, -1))
	c.Equal(other, x11PointerTarget(other, true, -1))
	c.True(x11PointerTarget(owner, false, -1) == nil, "with no press held, an event from outside the owner is dropped")
	c.Equal(popup, x11PointerTarget(popup, false, -1), "only the owner is reported events from outside it")
	c.True(x11PointerTarget(nil, false, ButtonLeft) == nil, "a release to a gone window with no press held is dropped")
	owner.wnd.pointerInside = false
	c.True(x11PointerTarget(owner, true, -1) == nil,
		"one within the owner's bounds after the pointer left it is over another client's window on top")
	owner.wnd.pointerInside = true

	popup.inMouseDown = true
	popup.pressedButtons = map[int]bool{ButtonLeft: true}
	c.Equal(popup, x11PointerTarget(owner, false, -1), "a drag off a popup over another client's window stays with it")
	c.Equal(popup, x11PointerTarget(owner, true, -1), "as does one over the owner's content")
	c.Equal(popup, x11PointerTarget(other, true, -1), "and one over another window of ours")
	c.Equal(popup, x11PointerTarget(owner, false, ButtonLeft), "so does the release of the button it holds")
	c.Equal(popup, x11PointerTarget(nil, false, ButtonLeft), "even one reported to a window that is gone")
	c.Equal(popup, x11PointerTarget(nil, false, -1), "as is a motion reported to one")

	owner.inMouseDown = true
	owner.pressedButtons = map[int]bool{ButtonRight: true}
	c.Equal(popup, x11PointerTarget(other, true, -1), "a popup holding a press is preferred")
	c.Equal(owner, x11PointerTarget(owner, false, -1), "a window holding a press keeps the motion it is sent")
	c.Equal(owner, x11PointerTarget(popup, true, ButtonRight),
		"the release of a button the owner holds goes to the owner, whichever window it is sent to")
	c.Equal(popup, x11PointerTarget(owner, true, ButtonLeft), "and that of one the popup holds to the popup")
	c.Equal(owner, x11PointerTarget(owner, true, ButtonMiddle),
		"the release of a button no window records stays where it was sent")
	c.Equal(owner, x11PointerTarget(nil, false, ButtonRight), "a release to a gone window goes to the button's holder")
	popup.inMouseDown = false
	popup.pressedButtons = nil
	c.Equal(owner, x11PointerTarget(popup, true, -1), "a press on the owner keeps the events a popup is sent")

	popup.inMouseDown = true
	popup.pressedButtons = map[int]bool{ButtonLeft: true}
	popup.valid = false
	c.Equal(owner, x11PointerTarget(other, true, -1), "a disposed popup holds nothing")
	c.Equal(owner, x11PointerTarget(other, true, ButtonLeft))

	owner.wnd.menuGrab = false
	c.Equal(other, x11PointerTarget(other, true, -1), "without a menu grab, X already routes the events")
	c.Equal(owner, x11PointerTarget(owner, false, -1))
	c.Equal(popup, x11PointerTarget(popup, true, ButtonRight))
	c.True(x11PointerTarget(nil, false, ButtonRight) == nil, "and a release to a gone window has nowhere to go")
}

// TestX11IgnoreEnter covers which EnterNotify events are ignored: one a grab reports as it takes effect, which enters
// the grab window wherever the pointer is, and, while the menus hold the pointer, one entering a window other than the
// one holding a press. The crossings normal motion and a grab's release report are kept.
func TestX11IgnoreEnter(t *testing.T) {
	c := check.New(t)
	owner, popup, _ := x11MenuGrabTestWindows(t)
	enter := func(mode byte) *x11.EnterNotifyEvent {
		ev := &x11.EnterNotifyEvent{}
		ev.Mode = mode
		return ev
	}
	c.True(x11IgnoreEnter(owner, enter(x11.NotifyGrab)))
	c.False(x11IgnoreEnter(owner, enter(x11.NotifyNormal)))
	c.False(x11IgnoreEnter(owner, enter(x11.NotifyUngrab)))
	c.False(x11IgnoreEnter(popup, enter(x11.NotifyNormal)))

	popup.inMouseDown = true
	c.True(x11IgnoreEnter(owner, enter(x11.NotifyNormal)), "the popup holding a press gets the events, not the owner")
	c.False(x11IgnoreEnter(popup, enter(x11.NotifyNormal)))

	owner.wnd.menuGrab = false
	c.False(x11IgnoreEnter(owner, enter(x11.NotifyNormal)), "without a menu grab, X already routes the crossings")
	c.True(x11IgnoreEnter(owner, enter(x11.NotifyGrab)))
}

// TestX11OtherButtonsDown covers the check that ends the grab standing in for a press's implicit grab: the state a
// release carries is from before it, so the released button's own mask does not count, nor do the modifiers.
func TestX11OtherButtonsDown(t *testing.T) {
	c := check.New(t)
	const (
		shiftMask   = 0x0001
		controlMask = 0x0004
		button1Mask = 0x0100
		button3Mask = 0x0400
		button4Mask = 0x0800
	)
	release := func(button byte, state uint16) *x11.ButtonReleaseEvent {
		ev := &x11.ButtonReleaseEvent{}
		ev.Detail = button
		ev.State = state
		return ev
	}
	c.False(x11OtherButtonsDown(release(1, button1Mask)))
	c.False(x11OtherButtonsDown(release(1, button1Mask|shiftMask|controlMask)))
	c.True(x11OtherButtonsDown(release(1, button1Mask|button3Mask)))
	c.True(x11OtherButtonsDown(release(3, button1Mask|button3Mask)))
	c.True(x11OtherButtonsDown(release(4, button1Mask|button4Mask)), "a wheel released while a button is held")
	c.False(x11OtherButtonsDown(release(8, 0)), "the core protocol has no mask for buttons above five")
	c.True(x11OtherButtonsDown(release(8, button1Mask)))
}

// TestX11MenuStackGrab covers the pointer grab an open menu stack takes: granted with owner events on and the arrow as
// its cursor when the stack opens, released when it empties, and handed, without owner events, to a window still
// holding a press when it empties, until that press is let go or canceled. A menu opened during a drag takes none.
func TestX11MenuStackGrab(t *testing.T) {
	c := check.New(t)
	server := startX11TestServer(t)
	priorDragSource, priorArrow, priorSettings := dragSource, arrowCursor, builtCursorSettings
	t.Cleanup(func() { dragSource, arrowCursor, builtCursorSettings = priorDragSource, priorArrow, priorSettings })
	dragSource = nil
	// The arrow the menu grab shows is given a stand-in, since building a real one would take requests the stand-in X
	// server does not serve. With no record of the settings the built-in cursors were built with, none is rebuilt.
	const arrowID = x11.CursorID(77)
	arrowCursor, builtCursorSettings = &Cursor{cursor: arrowID}, nil
	owner := &Window{valid: true, wnd: &apiWindow{}}
	owner.wnd.id = x11TestWindowID
	windowList = []*Window{owner}
	menuGrab := x11Grab{window: x11TestWindowID, cursor: arrowID, eventMask: x11MenuGrabEventMask, ownerEvents: true}
	released := x11Grab{released: true}
	var nothing []x11Grab

	owner.nativeMenuStackChanged(true)
	c.True(owner.wnd.menuGrab)
	c.Equal([]x11Grab{menuGrab}, server.grabs())
	owner.nativeMenuStackChanged(true)
	c.Equal(nothing, server.grabs(), "a grab already held is not asked for again")
	owner.nativeMenuStackChanged(false)
	c.False(owner.wnd.menuGrab)
	c.Equal([]x11Grab{released}, server.grabs())
	owner.nativeMenuStackChanged(false)
	c.Equal(nothing, server.grabs(), "nothing is released when nothing is held")

	owner.nativeMenuStackChanged(true)
	c.Equal([]x11Grab{menuGrab}, server.grabs())
	owner.inMouseDown = true
	owner.nativeMenuStackChanged(false)
	c.False(owner.wnd.menuGrab)
	c.True(owner.wnd.pressGrab, "a press still down when the menus close keeps the pointer for its window")
	c.Equal([]x11Grab{{
		window:    x11TestWindowID,
		eventMask: x11MenuGrabEventMask | x11.EventMaskEnterWindow | x11.EventMaskLeaveWindow,
	}}, server.grabs())
	owner.nativeMenuStackChanged(true)
	c.False(owner.wnd.pressGrab, "a menu grab replaces the one held for a press")
	c.Equal([]x11Grab{menuGrab}, server.grabs())
	owner.inMouseDown = false
	owner.nativeMenuStackChanged(false)
	c.Equal([]x11Grab{released}, server.grabs())

	owner.wnd.pressGrab = true
	owner.nativeCancelMouseCapture()
	c.False(owner.wnd.pressGrab)
	c.Equal([]x11Grab{released}, server.grabs(), "a canceled press releases the grab held for it")
	owner.nativeCancelMouseCapture()
	c.Equal(nothing, server.grabs())

	// A release ends the grab held for a press once the window has no press left, but not while another button is down.
	owner.wnd.pressGrab = true
	owner.inMouseDown = true
	release := &x11.ButtonReleaseEvent{}
	release.Detail = 1
	release.State = 0x0500 // Button1Mask and Button3Mask, from before the release
	x11EndPressGrabs(release)
	c.True(owner.wnd.pressGrab)
	c.Equal(nothing, server.grabs())
	owner.inMouseDown = false
	x11EndPressGrabs(release)
	c.False(owner.wnd.pressGrab)
	c.Equal([]x11Grab{released}, server.grabs())

	// A held back or forward button, for which the release's state carries nothing, keeps the grab too.
	owner.wnd.pressGrab = true
	owner.inMouseDown = true
	owner.pressedButtons = map[int]bool{x11ButtonFromDetail(8): true}
	release.State = 0
	x11EndPressGrabs(release)
	c.True(owner.wnd.pressGrab)
	c.Equal(nothing, server.grabs())
	owner.inMouseDown = false
	owner.pressedButtons = nil
	x11EndPressGrabs(release)
	c.False(owner.wnd.pressGrab)
	c.Equal([]x11Grab{released}, server.grabs())

	dragSource = owner
	owner.nativeMenuStackChanged(true)
	c.False(owner.wnd.menuGrab, "the drag in progress holds the pointer")
	c.Equal(nothing, server.grabs())
}

// TestX11MenuGrabMovesBetweenWindows covers the one pointer grab the server holds for the client: a menu stack opened
// for a second window takes it from the first, which stops saying it holds the grab, so that the first window's menus
// closing releases nothing and the second window's closing releases the grab.
func TestX11MenuGrabMovesBetweenWindows(t *testing.T) {
	c := check.New(t)
	server := startX11TestServer(t)
	priorDragSource, priorArrow, priorSettings := dragSource, arrowCursor, builtCursorSettings
	t.Cleanup(func() { dragSource, arrowCursor, builtCursorSettings = priorDragSource, priorArrow, priorSettings })
	dragSource = nil
	const arrowID = x11.CursorID(77)
	arrowCursor, builtCursorSettings = &Cursor{cursor: arrowID}, nil
	first := &Window{valid: true, wnd: &apiWindow{}}
	first.wnd.id = x11TestWindowID
	second := &Window{valid: true, wnd: &apiWindow{}}
	second.wnd.id = x11TestWindowID + 1
	windowList = []*Window{first, second}
	var nothing []x11Grab

	first.nativeMenuStackChanged(true)
	c.True(first.wnd.menuGrab)
	c.Equal([]x11Grab{{window: first.wnd.id, cursor: arrowID, eventMask: x11MenuGrabEventMask, ownerEvents: true}},
		server.grabs())
	second.nativeMenuStackChanged(true)
	c.True(second.wnd.menuGrab)
	c.False(first.wnd.menuGrab, "the second window's grab replaced the first's")
	c.Equal([]x11Grab{{window: second.wnd.id, cursor: arrowID, eventMask: x11MenuGrabEventMask, ownerEvents: true}},
		server.grabs())
	c.True(x11MenuGrabOwner() == second)
	first.nativeMenuStackChanged(false)
	c.Equal(nothing, server.grabs(), "the first window's menus closing releases nothing")
	c.True(second.wnd.menuGrab)
	second.nativeMenuStackChanged(false)
	c.False(second.wnd.menuGrab)
	c.Equal([]x11Grab{{released: true}}, server.grabs())
}

// TestX11DestroyLeavesQueuedEventsAlone covers the event handling destroying a window does not do: a popup menu
// window's destroy and a top-level window's both leave the queue to the event loop, since a window is disposed of from
// inside the handling of an event, such as the press that closed a popup's menu, and a release queued behind that press
// must not be handled before it.
func TestX11DestroyLeavesQueuedEventsAlone(t *testing.T) {
	c := check.New(t)
	server := startX11TestServer(t)
	owner, popup, other := x11MenuGrabTestWindows(t)
	owner.wnd.menuGrab = false
	// The release is reported to a window that is not one of ours, so handling it would do nothing but consume it.
	const elsewhere = x11.WindowID(900)
	popup.wnd.id, popup.glCtx = x11.WindowID(43), &apiGLContext{}
	other.wnd.id, other.glCtx = x11.WindowID(44), &apiGLContext{}
	for _, w := range []*Window{popup, other} {
		server.sendButtonRelease(elsewhere)
		w.nativeDestroy()
		c.Equal(x11.WindowID(0), w.wnd.id)
		ev := x11Conn.PollEvents(nil)
		c.NotNil(ev, "the release is still queued for the event loop after the %s window is destroyed", w.title)
		if release, ok := ev.(*x11.ButtonReleaseEvent); ok {
			c.Equal(elsewhere, release.Event)
		} else {
			c.Errorf("queued event = %T, want a ButtonRelease", ev)
		}
	}
}

// TestX11PopupWindowProperties covers the properties a popup window is given: its PID, its type, which is a popup
// menu's for a menu, a tooltip's for a tooltip and a combo box's for a field's suggestions, KWin's request to skip the
// close animation and WM_CLASS, and none of those only a window manager reads.
func TestX11PopupWindowProperties(t *testing.T) {
	c := check.New(t)
	server := startX11TestServer(t)
	// The stand-in connection interns no atoms, so the ones the requests name are given values they can be checked by.
	const (
		pidAtom       = x11.Atom(401)
		typeAtom      = x11.Atom(402)
		popupMenuAtom = x11.Atom(403)
		skipCloseAtom = x11.Atom(404)
		tooltipAtom   = x11.Atom(405)
		comboAtom     = x11.Atom(406)
	)
	x11Conn.Atoms.WMPID = pidAtom
	x11Conn.Atoms.NetWMWindowType = typeAtom
	x11Conn.Atoms.NetWMWindowTypePopupMenu = popupMenuAtom
	x11Conn.Atoms.NetWMWindowTypeTooltip = tooltipAtom
	x11Conn.Atoms.NetWMWindowTypeCombo = comboAtom
	x11Conn.Atoms.KDENetWMSkipCloseAnimation = skipCloseAtom
	cardinal := func(v uint32) string {
		buf := x11.NewWriter(4)
		buf.Uint32(v)
		return string(buf.Retrieve())
	}
	for _, one := range []struct {
		name       string
		kind       WindowKind
		windowType x11.Atom
	}{
		{name: "menu", kind: WindowKindMenu, windowType: popupMenuAtom},
		{name: "tooltip", kind: WindowKindTooltip, windowType: tooltipAtom},
		{name: "dropdown", kind: WindowKindDropdown, windowType: comboAtom},
	} {
		popup := &Window{wnd: &apiWindow{}, owner: &Window{}, kind: one.kind}
		popup.wnd.id = x11TestWindowID
		popup.x11SetPopupProperties()
		c.Equal([]x11PropertyChange{
			{
				data:         cardinal(uint32(os.Getpid())),
				window:       x11TestWindowID,
				property:     pidAtom,
				propertyType: x11.AtomCardinal,
				format:       32,
				mode:         x11.PropModeReplace,
			},
			{
				data:         cardinal(uint32(one.windowType)),
				window:       x11TestWindowID,
				property:     typeAtom,
				propertyType: x11.AtomAtom,
				format:       32,
				mode:         x11.PropModeReplace,
			},
			{
				data:         cardinal(1),
				window:       x11TestWindowID,
				property:     skipCloseAtom,
				propertyType: x11.AtomCardinal,
				format:       32,
				mode:         x11.PropModeReplace,
			},
			{
				data:         string(wmClassData()),
				window:       x11TestWindowID,
				property:     x11.AtomWMClass,
				propertyType: x11.AtomString,
				format:       8,
				mode:         x11.PropModeReplace,
			},
		}, server.propertyChanges(), "the %s window's properties", one.name)
	}
}

// TestX11PointerCrossed covers the record of whether the pointer is in a window: a normal crossing or one a grab's
// release reports sets it, an enter a grab reports as it takes effect leaves it alone, since the grab window is entered
// wherever the pointer is, and every leave clears it.
func TestX11PointerCrossed(t *testing.T) {
	c := check.New(t)
	w := &Window{wnd: &apiWindow{}}
	w.x11PointerCrossed(true, x11.NotifyNormal)
	c.True(w.wnd.pointerInside)
	w.x11PointerCrossed(false, x11.NotifyNormal)
	c.False(w.wnd.pointerInside)
	w.x11PointerCrossed(true, x11.NotifyGrab)
	c.False(w.wnd.pointerInside, "a grab enters its window wherever the pointer is")
	w.x11PointerCrossed(true, x11.NotifyUngrab)
	c.True(w.wnd.pointerInside)
	w.x11PointerCrossed(false, x11.NotifyGrab)
	c.False(w.wnd.pointerInside, "another client's grab takes the pointer away")
}

// TestX11LeaveNotifyTakesDownTheTooltip covers a LeaveNotify, which reports that the pointer has left the window
// itself: the panel under it is exited, the tooltip showing is taken down, keeping its window for the next one, and the
// tooltip asked for is forgotten, so that coming back to the same panel arms it again.
func TestX11LeaveNotifyTakesDownTheTooltip(t *testing.T) {
	c := check.New(t)
	priorWindows := windowList
	t.Cleanup(func() { windowList = priorWindows })
	// A cursor already hidden is the one state in which a crossing asks the X server nothing about the cursor, and with
	// no window valid, hiding the tooltip window and marking a window for redraw do nothing, so no stand-in is needed.
	owner := &Window{wnd: &apiWindow{}, cursorHidden: true}
	owner.wnd.id = x11TestWindowID
	owner.wnd.cursorWasHidden = true
	owner.wnd.pointerInside = true
	hovered := NewPanel()
	exited := false
	hovered.MouseExitCallback = func() bool {
		exited = true
		return true
	}
	owner.lastMouseOverPanel = hovered
	tip := NewPanel()
	owner.lastTooltip = tip
	tooltipWindow := &Window{wnd: &apiWindow{}, owner: owner, kind: WindowKindTooltip}
	tooltipWindow.root = newRootPanel(tooltipWindow)
	tooltipWindow.root.setContent(tip)
	owner.tooltipWindow = tooltipWindow
	windowList = []*Window{owner}
	c.True(owner.tooltipShowing())

	ev := &x11.LeaveNotifyEvent{}
	ev.Event = owner.wnd.id
	ev.Mode = x11.NotifyNormal
	x11ProcessEvent(ev)
	c.False(owner.wnd.pointerInside)
	c.True(exited, "the panel under the pointer is exited")
	c.False(owner.tooltipShowing(), "the tooltip is taken down")
	c.True(owner.tooltipWindow == tooltipWindow, "its window is kept for the next tooltip")
	c.True(owner.lastTooltip == nil, "the tooltip asked for is forgotten, so coming back to its panel arms it again")
}

// TestX11PointerEventPositionUsesTheTargetOrigin covers the position of a pointer event rerouted to another window of
// ours: worked out from the event's root position and the target's own origin, which its last ConfigureNotify
// recorded, rather than asked of the X server on every motion.
func TestX11PointerEventPositionUsesTheTargetOrigin(t *testing.T) {
	c := check.New(t)
	owner, popup, _ := x11MenuGrabTestWindows(t)
	popup.wnd.lastX, popup.wnd.lastY = 1000, 500
	ev := &x11.InputEvent{EventX: 10, EventY: 20, RootX: 1010, RootY: 520}
	x, y := x11PointerEventPosition(owner, owner, ev)
	c.Equal(int16(10), x)
	c.Equal(int16(20), y)
	x, y = x11PointerEventPosition(owner, popup, ev)
	c.Equal(int16(10), x)
	c.Equal(int16(20), y)
	ev.RootX, ev.RootY = 990, 480
	x, y = x11PointerEventPosition(owner, popup, ev)
	c.Equal(int16(-10), x, "a drag off the popup's left edge")
	c.Equal(int16(-20), y)
	x, y = x11PointerEventPosition(nil, popup, ev)
	c.Equal(int16(-10), x, "an event reported to a window that is gone has only its root position")
	c.Equal(int16(-20), y)
}

// TestX11OtherButtonsHeld covers the window's own record of held buttons, which is what tells a held back or forward
// button, for which the core protocol carries no state, from nothing else being down, and the mapping of a press's
// detail to the toolkit's buttons it relies on.
func TestX11OtherButtonsHeld(t *testing.T) {
	c := check.New(t)
	c.Equal(ButtonLeft, x11ButtonFromDetail(1))
	c.Equal(ButtonMiddle, x11ButtonFromDetail(2))
	c.Equal(ButtonRight, x11ButtonFromDetail(3))
	for detail := byte(4); detail <= 7; detail++ {
		c.Equal(-1, x11ButtonFromDetail(detail), "the wheel is no button")
	}
	c.Equal(ButtonMiddle+1, x11ButtonFromDetail(8), "the back button, as x11ProcessEvent reports it")
	c.Equal(ButtonMiddle+2, x11ButtonFromDetail(9))
	w := &Window{wnd: &apiWindow{}, pressedButtons: map[int]bool{}}
	c.False(w.x11OtherButtonsHeld(1))
	w.pressedButtons[ButtonLeft] = true
	c.False(w.x11OtherButtonsHeld(1), "the released button itself does not count")
	c.True(w.x11OtherButtonsHeld(3))
	c.True(w.x11OtherButtonsHeld(4), "a wheel released while a button is held")
	w.pressedButtons = map[int]bool{x11ButtonFromDetail(8): true}
	c.True(w.x11OtherButtonsHeld(1), "a held back button, which the release's state cannot carry")
	c.False(w.x11OtherButtonsHeld(8))
}

// TestX11DragReplacesAndRestoresTheMenuGrab covers the menu grab a drag's own grab replaces: the window that held it
// stops saying so, since the drag's release ends it, and once the drag has released its grab the menu grab is taken
// again if the menus are still open, before the drag is forgotten, and not otherwise. A menu opened during the drag,
// which took no grab, is given one then too.
func TestX11DragReplacesAndRestoresTheMenuGrab(t *testing.T) {
	c := check.New(t)
	server := startX11TestServer(t)
	priorDragSource, priorArrow, priorSettings := dragSource, arrowCursor, builtCursorSettings
	t.Cleanup(func() { dragSource, arrowCursor, builtCursorSettings = priorDragSource, priorArrow, priorSettings })
	const arrowID = x11.CursorID(77)
	arrowCursor, builtCursorSettings = &Cursor{cursor: arrowID}, nil
	owner := &Window{valid: true, wnd: &apiWindow{}}
	owner.wnd.id = x11TestWindowID
	owner.root = newRootPanel(owner)
	windowList = []*Window{owner}
	menuGrab := x11Grab{window: x11TestWindowID, cursor: arrowID, eventMask: x11MenuGrabEventMask, ownerEvents: true}
	var nothing []x11Grab

	c.True(x11DragReplacedMenuGrab() == nil, "no menu grab is held")
	x11RestoreMenuGrab(nil)
	c.Equal(nothing, server.grabs())

	// A drag is in progress while these run, so a menu opened during it takes no grab of its own.
	dragSource = owner
	mp := newTestMenuPanel(0, 0, 100, 100)
	owner.root.insertMenu(mp)
	c.False(owner.wnd.menuGrab)
	c.Equal(nothing, server.grabs())
	x11RestoreMenuGrab(nil)
	c.True(owner.wnd.menuGrab,
		"a menu opened during the drag, which replaced no grab, is given one once the drag is over")
	c.Equal([]x11Grab{menuGrab}, server.grabs())

	owner.wnd.menuGrab = true
	c.True(x11DragReplacedMenuGrab() == owner)
	c.False(owner.wnd.menuGrab, "the drag's grab replaced the menu grab")
	x11RestoreMenuGrab(owner)
	c.True(owner.wnd.menuGrab, "taken again after the drag while the menus are open")
	c.Equal([]x11Grab{menuGrab}, server.grabs())
	x11RestoreMenuGrab(owner)
	c.Equal(nothing, server.grabs(), "a grab already held is not asked for again")

	owner.wnd.menuGrab = false
	owner.root.removeMenu(mp)
	x11RestoreMenuGrab(owner)
	c.False(owner.wnd.menuGrab)
	c.Equal(nothing, server.grabs(), "nothing is taken once the menus have closed")
}

// TestX11PressGrabWaitsForAnUnreadPress covers a press that reached the server while the menu grab was held but has not
// been read yet, as when the event loop lags behind a double-click: it formed no implicit grab, so the menus closing
// keeps the grab in place for it rather than releasing it, a release read before the press leaves the grab alone, and
// the press's own release, after which the server reports no button down, ends it.
func TestX11PressGrabWaitsForAnUnreadPress(t *testing.T) {
	c := check.New(t)
	server := startX11TestServer(t)
	priorDragSource, priorArrow, priorSettings := dragSource, arrowCursor, builtCursorSettings
	t.Cleanup(func() { dragSource, arrowCursor, builtCursorSettings = priorDragSource, priorArrow, priorSettings })
	dragSource = nil
	const arrowID = x11.CursorID(77)
	arrowCursor, builtCursorSettings = &Cursor{cursor: arrowID}, nil
	owner := &Window{valid: true, wnd: &apiWindow{}}
	owner.wnd.id = x11TestWindowID
	windowList = []*Window{owner}
	menuGrab := x11Grab{window: x11TestWindowID, cursor: arrowID, eventMask: x11MenuGrabEventMask, ownerEvents: true}
	var nothing []x11Grab

	server.pointerMask.Store(x11TestButton1Mask)
	owner.nativeMenuStackChanged(true)
	c.Equal([]x11Grab{menuGrab}, server.grabs())
	owner.nativeMenuStackChanged(false)
	c.False(owner.wnd.menuGrab)
	c.True(owner.wnd.pressGrab, "the grab stays for the press the server reports down")
	c.Equal(nothing, server.grabs(), "neither released nor taken again")

	release := &x11.ButtonReleaseEvent{}
	release.Detail = 1
	x11EndPressGrabs(release)
	c.True(owner.wnd.pressGrab, "a release read before the press leaves the grab alone")
	c.Equal(nothing, server.grabs())

	server.pointerMask.Store(0)
	x11EndPressGrabs(release)
	c.False(owner.wnd.pressGrab, "the press's own release ends it")
	c.Equal([]x11Grab{{released: true}}, server.grabs())

	owner.nativeMenuStackChanged(true)
	owner.nativeMenuStackChanged(false)
	c.False(owner.wnd.pressGrab)
	c.Equal([]x11Grab{menuGrab, {released: true}}, server.grabs(), "with no button down, the grab is released at once")
}

// TestX11UnreadPressTakesTheGrabForItsWindow covers a press not yet read when the menus close, made on another of our
// top-level windows and released off it: once read, the window it reached takes the grab, without owner events or the
// menu grab's arrow, so that its release ends its press, whether reported to that window or, queued before the grab
// moved, to the owner. A press on the owner itself takes the grab in the same way, and one on a popup menu window
// leaves it alone.
func TestX11UnreadPressTakesTheGrabForItsWindow(t *testing.T) {
	c := check.New(t)
	server := startX11TestServer(t)
	priorDragSource, priorArrow, priorSettings := dragSource, arrowCursor, builtCursorSettings
	t.Cleanup(func() { dragSource, arrowCursor, builtCursorSettings = priorDragSource, priorArrow, priorSettings })
	dragSource = nil
	const arrowID = x11.CursorID(77)
	arrowCursor, builtCursorSettings = &Cursor{cursor: arrowID}, nil
	// No window is marked valid, so that handling their pointer events asks the stand-in X server nothing about their
	// geometry; see x11TestServer.
	owner := &Window{wnd: &apiWindow{}, pressedButtons: map[int]bool{}, lastWidth: 400, lastHeight: 300}
	owner.wnd.id = x11TestWindowID
	owner.root = newRootPanel(owner)
	other := &Window{wnd: &apiWindow{}, pressedButtons: map[int]bool{}, lastWidth: 400, lastHeight: 300}
	other.wnd.id = x11TestWindowID + 1
	other.root = newRootPanel(other)
	popup := &Window{wnd: &apiWindow{}, owner: owner}
	popup.wnd.id = x11TestWindowID + 2
	windowList = []*Window{owner, other, popup}
	menuGrab := x11Grab{window: owner.wnd.id, cursor: arrowID, eventMask: x11MenuGrabEventMask, ownerEvents: true}
	const pressGrabMask = x11MenuGrabEventMask | x11.EventMaskEnterWindow | x11.EventMaskLeaveWindow
	pressGrab := func(w *Window) x11Grab { return x11Grab{window: w.wnd.id, eventMask: pressGrabMask} }
	released := x11Grab{released: true}
	var nothing []x11Grab
	closeMenusWithPressUnread := func() {
		t.Helper()
		server.pointerMask.Store(x11TestButton1Mask)
		owner.nativeMenuStackChanged(true)
		owner.nativeMenuStackChanged(false)
		c.True(owner.wnd.pressGrab)
		c.Equal([]x11Grab{menuGrab}, server.grabs())
	}
	press := func(w *Window) {
		t.Helper()
		ev := &x11.ButtonPressEvent{}
		ev.Detail = 1
		ev.Event = w.wnd.id
		x11ProcessEvent(ev)
	}
	// release reports to the window given a release of button 1 made outside it, after which the server reports no
	// button down.
	release := func(to *Window) {
		t.Helper()
		server.pointerMask.Store(0)
		ev := &x11.ButtonReleaseEvent{}
		ev.Detail = 1
		ev.State = x11TestButton1Mask
		ev.Event = to.wnd.id
		ev.EventX, ev.EventY = -50, 10
		x11ProcessEvent(ev)
	}

	closeMenusWithPressUnread()
	press(other)
	c.True(other.inMouseDown)
	c.False(owner.wnd.pressGrab)
	c.True(other.wnd.pressGrab, "the window the press reached takes the grab")
	c.Equal([]x11Grab{pressGrab(other)}, server.grabs())
	release(other)
	c.False(other.inMouseDown)
	c.False(other.wnd.pressGrab)
	c.Equal([]x11Grab{released}, server.grabs())

	closeMenusWithPressUnread()
	press(other)
	c.Equal([]x11Grab{pressGrab(other)}, server.grabs())
	release(owner)
	c.False(other.inMouseDown, "a release reported to the owner, which records no press, still ends the press")
	c.False(other.pressedButtons[ButtonLeft])
	c.False(other.wnd.pressGrab)
	c.Equal([]x11Grab{released}, server.grabs())

	closeMenusWithPressUnread()
	popup.x11AdoptPressGrab()
	c.True(owner.wnd.pressGrab, "a press on a popup menu window leaves the grab where it is")
	c.False(popup.wnd.pressGrab)
	c.Equal(nothing, server.grabs())
	press(owner)
	c.True(owner.inMouseDown)
	c.True(owner.wnd.pressGrab)
	c.Equal([]x11Grab{pressGrab(owner)}, server.grabs(), "a press on the owner drops the owner events and the arrow")
	release(owner)
	c.False(owner.inMouseDown)
	c.False(owner.wnd.pressGrab)
	c.Equal([]x11Grab{released}, server.grabs())
}

// TestX11MenuGrabRefusedIsRetried covers a menu grab the server refuses, as it does while another client holds the
// pointer: the grab is asked for again once the retry comes due, and again after that, until it is granted or the
// menus have closed, and not while a drag holds the pointer.
func TestX11MenuGrabRefusedIsRetried(t *testing.T) {
	c := check.New(t)
	server := startX11TestServer(t)
	priorDragSource, priorArrow, priorSettings, priorSchedule := dragSource, arrowCursor, builtCursorSettings,
		x11ScheduleMenuGrabRetry
	t.Cleanup(func() {
		dragSource, arrowCursor, builtCursorSettings, x11ScheduleMenuGrabRetry = priorDragSource, priorArrow,
			priorSettings, priorSchedule
		x11MenuGrabRetryPending = false
	})
	dragSource = nil
	const arrowID = x11.CursorID(77)
	arrowCursor, builtCursorSettings = &Cursor{cursor: arrowID}, nil
	scheduled := 0
	x11ScheduleMenuGrabRetry = func(func()) { scheduled++ }
	owner := &Window{valid: true, wnd: &apiWindow{}}
	owner.wnd.id = x11TestWindowID
	owner.root = newRootPanel(owner)
	windowList = []*Window{owner}
	menuGrab := x11Grab{window: x11TestWindowID, cursor: arrowID, eventMask: x11MenuGrabEventMask, ownerEvents: true}
	var nothing []x11Grab

	server.refuseGrabs.Store(true)
	mp := newTestMenuPanel(0, 0, 100, 100)
	owner.root.insertMenu(mp)
	c.False(owner.wnd.menuGrab)
	c.Equal([]x11Grab{menuGrab}, server.grabs())
	c.Equal(1, scheduled, "a refused grab schedules a retry")
	c.True(x11MenuGrabRetryPending)
	x11RetryMenuGrab()
	c.False(owner.wnd.menuGrab)
	c.Equal([]x11Grab{menuGrab}, server.grabs(), "the retry asks again")
	c.Equal(2, scheduled, "and, refused again, schedules another")
	server.refuseGrabs.Store(false)
	x11RetryMenuGrab()
	c.True(owner.wnd.menuGrab, "granted at last")
	c.Equal([]x11Grab{menuGrab}, server.grabs())
	c.Equal(2, scheduled)
	c.False(x11MenuGrabRetryPending)

	owner.root.removeMenu(mp)
	c.Equal([]x11Grab{{released: true}}, server.grabs())
	x11RetryMenuGrab()
	c.False(owner.wnd.menuGrab)
	c.Equal(nothing, server.grabs(), "a retry coming due after the menus have closed takes nothing")

	server.refuseGrabs.Store(true)
	owner.root.insertMenu(mp)
	c.Equal([]x11Grab{menuGrab}, server.grabs())
	c.Equal(3, scheduled)
	dragSource = owner
	x11RetryMenuGrab()
	c.Equal(nothing, server.grabs(), "a retry coming due during a drag leaves the pointer to it")
	c.False(x11MenuGrabRetryPending)
	owner.root.removeMenu(mp)
}
