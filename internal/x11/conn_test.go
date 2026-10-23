// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xio"
)

// newPipeConn returns a Conn wired to the client end of an in-memory pipe with its reader and writer goroutines
// running, plus the server end of the pipe.
func newPipeConn() (conn *Conn, server net.Conn) {
	var client net.Conn
	client, server = net.Pipe()
	conn = &Conn{
		conn:         client,
		events:       make(chan Event, 1),
		requests:     make(chan *request, 128),
		closed:       make(chan struct{}),
		readClosed:   make(chan struct{}),
		eventNewMap:  newEventMap(),
		errorCodeMap: newErrorMap(),
		requestMap:   make(map[uint16]*request),
	}
	go conn.sendRequests()
	go conn.readResponses()
	return conn, server
}

// TestAuthenticateLargeSetupReply verifies that a setup reply of 16384 or more 4-byte words (64 KB+) is read in full.
// The length field used to be multiplied by 4 in uint16 arithmetic, which wraps for such replies and desyncs the
// connection with servers that send very large setup blobs.
func TestAuthenticateLargeSetupReply(t *testing.T) {
	c := check.New(t)
	t.Setenv("XAUTHORITY", filepath.Join(t.TempDir(), "nonexistent"))
	client, server := net.Pipe()
	defer xio.CloseIgnoringErrors(client)
	conn := &Conn{conn: client}
	const words = 16384 // words * 4 == 65536, which wraps to 0 in uint16 arithmetic
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- func() error {
			// Read the client's setup request: a 12-byte header followed by the padded auth name and data.
			header := make([]byte, 12)
			if _, err := io.ReadFull(server, header); err != nil {
				return err
			}
			nameLen := int(binary.LittleEndian.Uint16(header[6:8]))
			dataLen := int(binary.LittleEndian.Uint16(header[8:10]))
			if rest := pad4(nameLen) + pad4(dataLen); rest > 0 {
				if _, err := io.ReadFull(server, make([]byte, rest)); err != nil {
					return err
				}
			}
			// Send a "refused" setup reply whose additional data is 65536 bytes long.
			const reason = "too big to handle"
			reply := make([]byte, 8+words*4)
			reply[0] = 0 // Failed
			reply[1] = byte(len(reason))
			binary.LittleEndian.PutUint16(reply[2:4], 11) // protocol major version
			binary.LittleEndian.PutUint16(reply[4:6], 0)  // protocol minor version
			binary.LittleEndian.PutUint16(reply[6:8], words)
			copy(reply[8:], reason)
			_, err := server.Write(reply)
			return err
		}()
		xio.CloseIgnoringErrors(server)
	}()
	err := conn.authenticate()
	c.NotNil(err)
	c.True(strings.Contains(err.Error(), "too big to handle"),
		"the refusal reason must be read from beyond the 64 KB boundary; got: %v", err)
	if err = <-serverDone; err != nil {
		t.Fatal(err)
	}
}

// setupReplyScreenSize is the size of a SCREEN in a connection setup reply that lists no DEPTHs.
const setupReplyScreenSize = 40

// buildSetupReply returns a complete connection setup success reply, header included, for a server reporting the
// specified number of screens. Screen i is given a root window of 100+i, a root visual of 200+i and a root depth of
// 24, so a test can tell which screen a lookup resolved to.
func buildSetupReply(screens int) []byte {
	data := make([]byte, 32+screens*setupReplyScreenSize)
	binary.LittleEndian.PutUint32(data[0:4], 1)          // release-number
	binary.LittleEndian.PutUint32(data[4:8], 0x00400000) // resource-id-base
	binary.LittleEndian.PutUint32(data[8:12], 0x001fffff)
	binary.LittleEndian.PutUint32(data[12:16], 256)   // motion-buffer-size
	binary.LittleEndian.PutUint16(data[16:18], 0)     // length-of-vendor
	binary.LittleEndian.PutUint16(data[18:20], 65535) // maximum-request-length
	data[20] = byte(screens)                          // number-of-SCREENs
	data[21] = 0                                      // number-of-FORMATs
	data[22] = 0                                      // image-byte-order
	data[23] = 0                                      // bitmap-format-bit-order
	data[24] = 32                                     // bitmap-format-scanline-unit
	data[25] = 32                                     // bitmap-format-scanline-pad
	data[26] = 8                                      // min-keycode
	data[27] = 255                                    // max-keycode
	for i := range screens {
		s := data[32+i*setupReplyScreenSize:]
		binary.LittleEndian.PutUint32(s[0:4], uint32(100+i)) // root
		binary.LittleEndian.PutUint32(s[4:8], uint32(1))     // default-colormap
		binary.LittleEndian.PutUint16(s[20:22], 1920)        // width-in-pixels
		binary.LittleEndian.PutUint16(s[22:24], 1080)        // height-in-pixels
		binary.LittleEndian.PutUint16(s[24:26], 508)         // width-in-millimeters
		binary.LittleEndian.PutUint16(s[26:28], 285)         // height-in-millimeters
		binary.LittleEndian.PutUint16(s[28:30], 1)           // min-installed-maps
		binary.LittleEndian.PutUint16(s[30:32], 1)           // max-installed-maps
		binary.LittleEndian.PutUint32(s[32:36], uint32(200+i))
		s[38] = 24 // root-depth
		s[39] = 0  // number-of-DEPTHs
	}
	reply := make([]byte, 8+len(data))
	reply[0] = 1                                  // Success
	binary.LittleEndian.PutUint16(reply[2:4], 11) // protocol major version
	binary.LittleEndian.PutUint16(reply[4:6], 0)  // protocol minor version
	binary.LittleEndian.PutUint16(reply[6:8], uint16(len(data)/4))
	copy(reply[8:], data)
	return reply
}

// serveSetupReply consumes a connection setup request from server and answers it with reply.
func serveSetupReply(server net.Conn, reply []byte) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- func() error {
			header := make([]byte, 12)
			if _, err := io.ReadFull(server, header); err != nil {
				return err
			}
			nameLen := int(binary.LittleEndian.Uint16(header[6:8]))
			dataLen := int(binary.LittleEndian.Uint16(header[8:10]))
			if rest := pad4(nameLen) + pad4(dataLen); rest > 0 {
				if _, err := io.ReadFull(server, make([]byte, rest)); err != nil {
					return err
				}
			}
			_, err := server.Write(reply)
			return err
		}()
		xio.CloseIgnoringErrors(server)
	}()
	return done
}

// TestAuthenticateValidatesScreenNumber verifies that the screen number taken from DISPLAY (e.g. ":0.1") is checked
// against the number of screens the server actually reports. Parsing only rejects negative values, so without this
// check RootWindow(), DefaultVisual() and DefaultDepth() index Conn.Roots out of range and panic while NewConn
// creates its helper window, rather than the connection failing with a usable error.
func TestAuthenticateValidatesScreenNumber(t *testing.T) {
	for _, tc := range []struct {
		name    string
		screen  int
		screens int
		wantErr bool
	}{
		{name: "the only screen", screen: 0, screens: 1},
		{name: "the second of two screens", screen: 1, screens: 2},
		{name: "one past the last screen", screen: 1, screens: 1, wantErr: true},
		{name: "far past the last screen", screen: 5, screens: 2, wantErr: true},
		{name: "a server with no screens", screen: 0, screens: 0, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			t.Setenv("XAUTHORITY", filepath.Join(t.TempDir(), "nonexistent"))
			t.Setenv("DISPLAY", ":0."+strconv.Itoa(tc.screen))
			client, server := net.Pipe()
			defer xio.CloseIgnoringErrors(client)
			conn := &Conn{conn: client}
			c.NoError(conn.parseDisplayEnv())
			c.Equal(tc.screen, conn.DefaultScreen)
			serverDone := serveSetupReply(server, buildSetupReply(tc.screens))
			err := conn.authenticate()
			if tc.wantErr {
				c.HasError(err)
				c.True(err != nil && strings.Contains(err.Error(), "does not exist"),
					"the failure must name the missing screen; got: %v", err)
			} else {
				c.NoError(err)
				c.Equal(tc.screens, len(conn.Roots))
				// The accessors that index Roots must resolve to the screen DISPLAY asked for.
				c.Equal(WindowID(100+tc.screen), conn.RootWindow())
				c.Equal(VisualID(200+tc.screen), conn.DefaultVisual())
				c.Equal(byte(24), conn.DefaultDepth())
			}
			if err = <-serverDone; err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestWaitEventsUntilFixedDeadline verifies that the timeout is a deadline for the whole wait, not a maximum gap
// between events: a steady stream of non-matching events arriving faster than the timeout must not extend the wait.
func TestWaitEventsUntilFixedDeadline(t *testing.T) {
	conn := &Conn{events: make(chan Event, 1)}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				conn.deliverEvent(&fakeEvent{})
			}
		}
	}()
	const timeout = 100 * time.Millisecond
	type result struct {
		e       Event
		elapsed time.Duration
	}
	done := make(chan result, 1)
	go func() {
		start := time.Now()
		e := conn.WaitEventsUntil(func(Event) bool { return false }, timeout)
		done <- result{e: e, elapsed: time.Since(start)}
	}()
	select {
	case r := <-done:
		if r.e != nil {
			t.Fatalf("expected nil on timeout, got %#v", r.e)
		}
		if r.elapsed < timeout {
			t.Fatalf("returned after %v, before the %v deadline", r.elapsed, timeout)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the event stream extended the deadline indefinitely")
	}
}

// TestProcessRequestRouting verifies that responses are routed to the right channel of the tracked request and that
// the request is removed from the tracking map.
func TestProcessRequestRouting(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1), requestMap: make(map[uint16]*request)}

	// An error must be delivered on the failure channel.
	checked := newCheckedRequest(nil)
	conn.requestMap[3] = checked
	wantErr := errors.New("bad drawable")
	conn.processRequest(3, nil, wantErr)
	c.Equal(wantErr, <-checked.failureChan)

	// A success for a checked request must be delivered as a nil error.
	checked = newCheckedRequest(nil)
	conn.requestMap[4] = checked
	conn.processRequest(4, nil, nil)
	c.Nil(<-checked.failureChan)

	// A reply must be delivered on the reply channel.
	reply := newReplyRequest(nil, nil)
	conn.requestMap[5] = reply
	in := NewReader([]byte{1})
	conn.processRequest(5, in, nil)
	c.Equal(in, <-reply.replyChan)

	c.Equal(0, len(conn.requestMap), "processed requests must be removed from the tracking map")
	c.True(conn.PollEvents(nil) == nil, "routed responses must not also surface as events")
}

// TestProcessRequestUncheckedError verifies that a server error for a request that was never tracked (an unchecked
// request) is delivered as an ErrorEvent rather than dropped with a misleading "unknown request" log, while an
// unmatched reply produces no event.
func TestProcessRequestUncheckedError(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1), requestMap: make(map[uint16]*request)}
	wantErr := errors.New("bad window")
	conn.processRequest(9, nil, wantErr)
	ee, ok := conn.PollEvents(nil).(*ErrorEvent)
	c.True(ok, "an unmatched error must surface as an ErrorEvent")
	c.Equal(wantErr, ee.Error)
	conn.processRequest(10, NewReader(make([]byte, 32)), nil)
	c.True(conn.PollEvents(nil) == nil, "an unmatched reply must not produce an event")
}

// TestFlushDoesNotEvictTrackedRequest verifies that a Flush (which never gets a sequence number assigned, leaving it
// at 0) does not evict a tracked request whose 16-bit sequence wrapped to 0, which would strand that request's waiter.
func TestFlushDoesNotEvictTrackedRequest(t *testing.T) {
	c := check.New(t)
	conn, server := newPipeConn()
	defer xio.CloseIgnoringErrors(server)
	wrapped := newCheckedRequest(NewWriter(4))
	conn.requestMapLock.Lock()
	conn.requestMap[0] = wrapped
	conn.requestMapLock.Unlock()
	conn.Flush()
	conn.requestMapLock.RLock()
	_, ok := conn.requestMap[0]
	conn.requestMapLock.RUnlock()
	c.True(ok, "Flush must not evict the tracked request with sequence 0")
	conn.abortStartup()
}

// TestDeadImmediatelyAfterShutdown verifies the shutdown ordering contract: readClosed must be the last thing to close
// during shutdown, so anyone it unblocks (abortStartup, waiters in sendNewRequest) already observes Dead as true. The
// deferred closeEvents in readResponses used to run after readClosed was closed, leaving a window where the connection
// looked fully shut down but Dead still reported false. The loop shuts down many connections because a single pass
// rarely lands in that window. Each pass also checks that abortStartup, which handles a NewConn failure after the
// reader and writer goroutines have started, closes the socket rather than leaking it.
func TestDeadImmediatelyAfterShutdown(t *testing.T) {
	for i := range 200 {
		conn, server := newPipeConn()
		conn.abortStartup()
		if !conn.Dead() {
			t.Fatalf("iteration %d: Dead must report true once abortStartup returns", i)
		}
		// The server side's read ends once the client end shuts down.
		if _, err := server.Read(make([]byte, 1)); err == nil {
			t.Fatalf("iteration %d: abortStartup must close the socket", i)
		}
		xio.CloseIgnoringErrors(server)
	}
}

// TestXSettingsHandleManagerMessage verifies that a MANAGER ClientMessage for the XSETTINGS selection triggers
// re-resolution of the manager window (so a restarted settings daemon is picked up), and that unrelated ClientMessages
// are left alone.
func TestXSettingsHandleManagerMessage(t *testing.T) {
	c := check.New(t)
	const (
		selectionAtom Atom = 601
		settingsAtom  Atom = 602
		managerAtom   Atom = 603
	)
	conn, server := newPipeConn()
	conn.xset = &xSettings{
		selection: selectionAtom,
		settings:  settingsAtom,
		manager:   managerAtom,
		window:    12345,
		dark:      true,
		ok:        true,
	}
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- func() error {
			var seq uint16
			header := make([]byte, 4)
			for {
				if _, err := io.ReadFull(server, header); err != nil {
					return nil // The pipe closes when the Conn shuts down.
				}
				seq++
				size := int(binary.LittleEndian.Uint16(header[2:4])) * 4
				if _, err := io.ReadFull(server, make([]byte, size-4)); err != nil {
					return err
				}
				reply := make([]byte, 32)
				reply[0] = 1
				binary.LittleEndian.PutUint16(reply[2:4], seq)
				switch header[0] {
				case opGetSelectionOwner:
					// Report that the selection has no owner (offset 8 is left 0), the state after a settings daemon
					// exits without a replacement.
				case opGetInputFocus:
					// The Sync performed by checked requests; an all-zero reply suffices.
				default:
					continue // Requests without replies need no response.
				}
				if _, err := server.Write(reply); err != nil {
					return err
				}
			}
		}()
	}()

	// An unrelated ClientMessage must not be consumed.
	handled, changed := conn.XSettingsHandleManagerMessage(&ClientMessageEvent{
		Type:   managerAtom + 1,
		Data32: [5]uint32{0, uint32(selectionAtom)},
	})
	c.False(handled)
	c.False(changed)
	// A MANAGER message for a different selection must not be consumed either.
	handled, changed = conn.XSettingsHandleManagerMessage(&ClientMessageEvent{
		Type:   managerAtom,
		Data32: [5]uint32{0, uint32(selectionAtom + 1)},
	})
	c.False(handled)
	c.False(changed)

	// A MANAGER message for our selection must re-resolve the manager window.
	handled, changed = conn.XSettingsHandleManagerMessage(&ClientMessageEvent{
		Type:   managerAtom,
		Data32: [5]uint32{0, uint32(selectionAtom)},
	})
	c.True(handled)
	c.True(changed, "losing the manager must be reported as a change")
	c.Equal(WindowID(0), conn.xset.window, "the stale manager window must have been discarded")
	_, darkKnown := conn.XSettingsDark()
	c.False(darkKnown, "the dark-mode state must be unavailable without a manager")

	conn.abortStartup()
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

// TestLocateRequestConcurrent verifies that locateRequest can run concurrently with itself and with request
// registration, as happens when the checked-request cleanup on a sending goroutine overlaps the reader goroutine's
// response processing. locateRequest deletes from the request map, so it must take the write lock; under the race
// detector this test fails if it mutates the map while holding only the read lock.
func TestLocateRequestConcurrent(t *testing.T) {
	c := check.New(t)
	conn := &Conn{requestMap: make(map[uint16]*request)}
	const workers = 4
	const perWorker = 512
	for i := range workers * perWorker {
		conn.requestMap[uint16(i)] = newCheckedRequest(nil)
	}
	var located atomic.Int32
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWorker {
				if conn.locateRequest(uint16(w*perWorker+i)) != nil {
					located.Add(1)
				}
			}
		}()
	}
	// Register new requests concurrently, mirroring what sendRequests does while responses are being processed.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range perWorker {
			conn.requestMapLock.Lock()
			conn.requestMap[uint16(workers*perWorker+i)] = newCheckedRequest(nil)
			conn.requestMapLock.Unlock()
		}
	}()
	wg.Wait()
	c.Equal(int32(workers*perWorker), located.Load(), "every tracked request must be located exactly once")
	c.Equal(perWorker, len(conn.requestMap), "only the concurrently registered requests may remain")
}

// TestIconData verifies the _NET_WM_ICON payload layout for multiple images, including a sub-image with a non-zero
// origin and a stride wider than its pixel data, which used to panic.
func TestIconData(t *testing.T) {
	c := check.New(t)
	base := putImageTestImage(8, 6)
	sub, ok := base.SubImage(image.Rect(2, 1, 6, 4)).(*image.NRGBA)
	c.True(ok)
	full := putImageTestImage(2, 2)
	data := IconData([]*image.NRGBA{full, sub})
	c.Equal((8+2*2*4)+(8+4*3*4), len(data))

	c.Equal(uint32(2), binary.LittleEndian.Uint32(data[0:4]))
	c.Equal(uint32(2), binary.LittleEndian.Uint32(data[4:8]))
	c.Equal(premultipliedBGRA(full, 0, 0, 2, 2), data[8:24])

	c.Equal(uint32(4), binary.LittleEndian.Uint32(data[24:28]))
	c.Equal(uint32(3), binary.LittleEndian.Uint32(data[28:32]))
	c.Equal(premultipliedBGRA(sub, 0, 0, 4, 3), data[32:])
}

// TestConfigureWindowDoesNotSync verifies that ConfigureWindow goes out as a single unchecked request with no
// synchronization round-trip: it is issued for every pointer-motion event while dragging (to move the drag-image
// window), so a checked request would cost one server round-trip per motion event. The captureRequests fake server
// never replies to anything, so a regression back to a checked request deadlocks here and is caught by the test
// timeout.
func TestConfigureWindowDoesNotSync(t *testing.T) {
	c := check.New(t)
	requests := captureRequests(t, 1024, func(conn *Conn) {
		conn.ConfigureWindow(WindowID(3), ConfigureWindowMaskX|ConfigureWindowMaskY, &ConfigureWindowRequest{
			X: 10,
			Y: 20,
		})
	})
	c.Equal(1, len(requests))
	c.Equal(byte(opConfigureWindow), requests[0][0])
}

// TestGrabPointerEncodesRequest verifies that GrabPointer sends its owner-events argument in the request's second byte,
// with the window, event mask and cursor where the protocol puts them, and reports the reply's status.
func TestGrabPointerEncodesRequest(t *testing.T) {
	for _, tc := range []struct {
		ownerEvents bool
		status      byte
		granted     bool
	}{
		{ownerEvents: false, status: 0, granted: true},
		{ownerEvents: true, status: 0, granted: true},
		{ownerEvents: true, status: 1, granted: false}, // AlreadyGrabbed
	} {
		c := check.New(t)
		client, server := net.Pipe()
		conn, stop := NewConnForTest(client, []Screen{{Root: 1}})
		sent := make(chan []byte, 1)
		go func() {
			req := make([]byte, 24)
			if _, err := io.ReadFull(server, req); err != nil {
				return
			}
			sent <- req
			reply := make([]byte, 32)
			reply[0] = 1
			reply[1] = tc.status
			binary.LittleEndian.PutUint16(reply[2:4], 1)
			_, _ = server.Write(reply) //nolint:errcheck // A failed write fails the GrabPointer call below
		}()
		mask := uint16(EventMaskButtonPress | EventMaskButtonRelease | EventMaskPointerMotion)
		c.Equal(tc.granted, conn.GrabPointer(WindowID(42), tc.ownerEvents, mask, CursorID(7)))
		req := <-sent
		c.Equal(byte(opGrabPointer), req[0])
		c.Equal(tc.ownerEvents, req[1] == 1)
		c.Equal(uint16(6), binary.LittleEndian.Uint16(req[2:4]))
		c.Equal(uint32(42), binary.LittleEndian.Uint32(req[4:8]))
		c.Equal(mask, binary.LittleEndian.Uint16(req[8:10]))
		c.Equal(uint32(7), binary.LittleEndian.Uint32(req[16:20]))
		xio.CloseIgnoringErrors(server)
		stop()
	}
}

// TestParseFrameExtents verifies that a _NET_FRAME_EXTENTS property is only accepted when it actually holds all four
// CARD32s. A property of 8 to 12 bytes used to be accepted by the frame-extents refresh path, which then read past the
// end of the value (logging errors) and cached zeroed top/bottom borders as if the window manager had reported them.
func TestParseFrameExtents(t *testing.T) {
	c := check.New(t)

	extents := func(left, right, top, bottom uint32) []byte {
		w := NewWriter(16)
		w.Uint32(left)
		w.Uint32(right)
		w.Uint32(top)
		w.Uint32(bottom)
		return w.Retrieve()
	}

	// The property is ordered left, right, top, bottom, but the widths are returned top, left, bottom, right.
	value := extents(1, 2, 3, 4)
	top, left, bottom, right, ok := ParseFrameExtents(32, AtomCardinal, value)
	c.True(ok)
	c.Equal(uint32(3), top)
	c.Equal(uint32(1), left)
	c.Equal(uint32(4), bottom)
	c.Equal(uint32(2), right)

	// Trailing data beyond the four values is fine.
	_, _, _, _, ok = ParseFrameExtents(32, AtomCardinal, append(value, make([]byte, 8)...))
	c.True(ok)

	// Anything shorter than the four CARD32s is malformed and must be rejected rather than read past the end.
	for _, size := range []int{0, 4, 8, 12, 15} {
		top, left, bottom, right, ok = ParseFrameExtents(32, AtomCardinal, value[:size])
		c.False(ok, "a %d byte property must be rejected", size)
		c.Equal(uint32(0), top|left|bottom|right, "a rejected property must report no borders")
	}

	// So are the wrong format and the wrong property type.
	_, _, _, _, ok = ParseFrameExtents(16, AtomCardinal, value)
	c.False(ok, "a non-32-bit format must be rejected")
	_, _, _, _, ok = ParseFrameExtents(32, AtomString, value)
	c.False(ok, "a non-CARDINAL property type must be rejected")
}

// TestNewConnForTestServesRequests verifies the connection that the root package's own tests drive the platform code
// against. It is not a real one — nothing is authenticated and no atoms, extensions or helper window exist — so what
// has to hold is that a request written to it reaches the far end whole and that the reply the far end writes comes
// back as the values the caller asked for. TranslateCoordinates is what needed it: the ConfigureNotify handling asks
// the X server where the window manager's frame puts a window, and a test of that decision has to see the request. A
// property change has to arrive whole as well, in one request rather than split into as many as it has bytes, since the
// frame theme test reads the value it asks the window manager for off a single request.
//
// It is also the one place where the bytes that stand in for an X server can be checked, since the test that uses them
// only builds on Linux.
func TestNewConnForTestServesRequests(t *testing.T) {
	c := check.New(t)
	const (
		root   = WindowID(100)
		parent = WindowID(7)
	)
	client, server := net.Pipe()
	conn, stop := NewConnForTest(client, []Screen{{Root: root}})
	defer stop()
	defer xio.CloseIgnoringErrors(server)
	c.Equal(root, conn.RootWindow())
	type translation struct {
		src, dst WindowID
		x, y     int16
	}
	type propertyChange struct {
		data         string
		window       WindowID
		property     Atom
		propertyType Atom
		format       byte
		mode         byte
	}
	asked := make(chan translation, 1)
	changed := make(chan propertyChange, 8)
	go func() {
		var seq uint16
		header := make([]byte, 4)
		for {
			if _, err := io.ReadFull(server, header); err != nil {
				return // The connection under test has gone away, which the end of the test does on purpose
			}
			seq++
			body := make([]byte, int(binary.LittleEndian.Uint16(header[2:4]))*4-len(header))
			if _, err := io.ReadFull(server, body); err != nil {
				return
			}
			if header[0] == opChangeProperty {
				// The body is the window, the property, its type, the format (the number of bits in each unit of the
				// data), three bytes of padding, the number of units, then the data itself, padded to a multiple of four
				// bytes. No reply is sent, since the X server sends none.
				size := int(binary.LittleEndian.Uint32(body[16:20])) * int(body[12]/8)
				changed <- propertyChange{
					data:         string(body[20 : 20+min(size, len(body)-20)]),
					window:       WindowID(binary.LittleEndian.Uint32(body[0:4])),
					property:     Atom(binary.LittleEndian.Uint32(body[4:8])),
					propertyType: Atom(binary.LittleEndian.Uint32(body[8:12])),
					format:       body[12],
					mode:         header[1],
				}
				continue
			}
			if header[0] == opQueryPointer {
				// A reply is 32 bytes: the code, whether on the same screen, the sequence, the count of any additional
				// words, the root, the child, the root and window positions, the button state, then padding. The
				// root package's stand-in X server answers the same way, with the button state its tests set.
				reply := make([]byte, 32)
				reply[0] = 1
				reply[1] = 1
				binary.LittleEndian.PutUint16(reply[2:4], seq)
				binary.LittleEndian.PutUint32(reply[8:12], uint32(root))
				binary.LittleEndian.PutUint16(reply[24:26], 0x100)
				if _, err := server.Write(reply); err != nil {
					return
				}
				continue
			}
			if header[0] != opTranslateCoordinates {
				continue // Nothing else this test sends expects an answer
			}
			asked <- translation{
				src: WindowID(binary.LittleEndian.Uint32(body[0:4])),
				dst: WindowID(binary.LittleEndian.Uint32(body[4:8])),
				x:   int16(binary.LittleEndian.Uint16(body[8:10])),
				y:   int16(binary.LittleEndian.Uint16(body[10:12])),
			}
			// A reply is 32 bytes: the code, the value the request's own reply byte carries — here whether the two
			// windows are on the same screen — the sequence, the count of any additional words, and then the values.
			reply := make([]byte, 32)
			reply[0] = 1
			reply[1] = 1
			binary.LittleEndian.PutUint16(reply[2:4], seq)
			binary.LittleEndian.PutUint32(reply[8:12], 0)
			translatedX, translatedY := int16(311), int16(-222)
			binary.LittleEndian.PutUint16(reply[12:14], uint16(translatedX))
			binary.LittleEndian.PutUint16(reply[14:16], uint16(translatedY))
			if _, err := server.Write(reply); err != nil {
				return
			}
		}
	}()
	const (
		property     = Atom(301)
		propertyType = Atom(302)
	)
	// Sent ahead of the TranslateCoordinates below, whose reply cannot arrive until the far end has read every request
	// sent before it, so that the property change has been recorded, in however many requests it took, by the time the
	// reply is back.
	conn.ChangeProperty(parent, property, propertyType, 8, PropModeReplace, []byte("light"))
	x, y, sameScreen, child, err := conn.TranslateCoordinates(parent, conn.RootWindow(), 10, -20)
	c.NoError(err)
	c.Equal(int16(311), x)
	c.Equal(int16(-222), y)
	c.True(sameScreen)
	c.Equal(WindowID(0), child)
	select {
	case one := <-asked:
		c.Equal(translation{src: parent, dst: root, x: 10, y: -20}, one)
	default:
		t.Fatal("the request never reached the far end")
	}
	select {
	case one := <-changed:
		c.Equal(propertyChange{
			data:         "light",
			window:       parent,
			property:     property,
			propertyType: propertyType,
			format:       8,
			mode:         PropModeReplace,
		}, one, "the property change should have reached the far end whole, in a single request")
	default:
		t.Fatal("the property change never reached the far end")
	}
	select {
	case extra := <-changed:
		t.Errorf("the property change should have taken a single request, but another followed it: %+v", extra)
	default:
	}
	qpr := conn.QueryPointer(root)
	c.NotNil(qpr)
	if qpr != nil {
		c.Equal(root, qpr.Root)
		c.Equal(uint16(0x100), qpr.Mask, "the button state is read from where the reply carries it")
		c.True(qpr.SameScreen)
	}
}

// captureRequests runs fn against a Conn whose wire is an in-memory pipe and returns each raw X11 request written, in
// order.
func captureRequests(t *testing.T, maxRequestWords uint16, fn func(c *Conn)) [][]byte {
	t.Helper()
	client, server := net.Pipe()
	c := &Conn{
		conn:                 client,
		requests:             make(chan *request, 128),
		closed:               make(chan struct{}),
		readClosed:           make(chan struct{}),
		maximumRequestLength: maxRequestWords,
	}
	go c.sendRequests()
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(&buf, server) //nolint:errcheck // Ends with an error when the pipe is closed, which is expected
	}()
	fn(c)
	c.Flush() // Ensure all prior requests have been written to the wire before shutting down
	close(c.readClosed)
	<-c.closed
	<-done
	var requests [][]byte
	data := buf.Bytes()
	for len(data) > 0 {
		if len(data) < 4 {
			t.Fatalf("truncated request header: %d bytes remain", len(data))
		}
		size := int(binary.LittleEndian.Uint16(data[2:4])) * 4
		if size < 4 || size > len(data) {
			t.Fatalf("invalid request length %d with %d bytes remaining", size, len(data))
		}
		requests = append(requests, data[:size])
		data = data[size:]
	}
	return requests
}

type parsedChangeProperty struct {
	data      []byte
	words     int
	unitCount uint32
	mode      byte
	format    byte
}

func parseChangeProperty(t *testing.T, req []byte) parsedChangeProperty {
	t.Helper()
	if req[0] != opChangeProperty {
		t.Fatalf("expected ChangeProperty opcode %d, got %d", opChangeProperty, req[0])
	}
	format := req[16]
	unitCount := binary.LittleEndian.Uint32(req[20:24])
	byteSize := int(unitCount) * int(format) / 8
	if 24+byteSize > len(req) {
		t.Fatalf("request claims %d data bytes, but only %d are present", byteSize, len(req)-24)
	}
	return parsedChangeProperty{
		data:      req[24 : 24+byteSize],
		unitCount: unitCount,
		words:     len(req) / 4,
		mode:      req[1],
		format:    format,
	}
}

func patternedData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i * 31)
	}
	return data
}

// TestChangePropertySplitsOversizedData verifies that ChangeProperty splits data too large for one request on format
// unit boundaries, each request within the server's advertised maximum request length, and that the payload survives
// reassembly. An oversized Replace becomes a Replace followed by Appends, while every chunk of an oversized Append
// stays an Append. One full INCR chunk (incrThreshold less the 24-byte ChangeProperty header) must go out as a single
// request, since completeIncrTransfers relies on the requestor seeing exactly one PropertyNotify per chunk, and a
// zero-length write must still send a request, since the INCR protocol uses its PropertyNotify to end a transfer.
func TestChangePropertySplitsOversizedData(t *testing.T) {
	const (
		maxBytes      = (math.MaxUint16 - 6) * 4 // Data bytes per request at the largest encodable request.
		smallMaxWords = 4096
		smallMaxBytes = (smallMaxWords - 6) * 4 // Data bytes per request at a smaller server maximum.
	)
	for _, tc := range []struct {
		name     string
		units    []uint32
		modes    []byte
		words    []int
		size     int
		maxWords uint16
		format   byte
		mode     byte
		// incrChunk sizes the data as one full INCR chunk in place of size.
		incrChunk bool
	}{
		{
			name:     "oversized replace continues as appends",
			maxWords: math.MaxUint16,
			format:   8,
			mode:     PropModeReplace,
			size:     maxBytes + 5,
			units:    []uint32{maxBytes, 5},
			modes:    []byte{PropModeReplace, PropModeAppend},
			words:    []int{math.MaxUint16, 8},
		},
		{
			name:      "one full INCR chunk is a single request",
			maxWords:  math.MaxUint16,
			format:    8,
			mode:      PropModeReplace,
			incrChunk: true,
			units:     []uint32{maxBytes},
			modes:     []byte{PropModeReplace},
			words:     []int{math.MaxUint16},
		},
		{
			name:     "format 32 honors a smaller server maximum",
			maxWords: smallMaxWords,
			format:   32,
			mode:     PropModeReplace,
			size:     smallMaxBytes + 2*4,
			units:    []uint32{smallMaxBytes / 4, 2},
			modes:    []byte{PropModeReplace, PropModeAppend},
			words:    []int{smallMaxWords, 8},
		},
		{
			name:     "format 16 splits on unit boundaries",
			maxWords: smallMaxWords,
			format:   16,
			mode:     PropModeReplace,
			size:     smallMaxBytes + 2,
			units:    []uint32{smallMaxBytes / 2, 1},
			modes:    []byte{PropModeReplace, PropModeAppend},
			words:    []int{smallMaxWords, 7},
		},
		{
			name:     "oversized append stays an append",
			maxWords: smallMaxWords,
			format:   8,
			mode:     PropModeAppend,
			size:     smallMaxBytes + 7,
			units:    []uint32{smallMaxBytes, 7},
			modes:    []byte{PropModeAppend, PropModeAppend},
			words:    []int{smallMaxWords, 8},
		},
		{
			name:     "zero-length write still sends a request",
			maxWords: math.MaxUint16,
			format:   8,
			mode:     PropModeReplace,
			units:    []uint32{0},
			modes:    []byte{PropModeReplace},
			words:    []int{6},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			var data []byte
			requests := captureRequests(t, tc.maxWords, func(conn *Conn) {
				size := tc.size
				if tc.incrChunk {
					size = conn.incrThreshold() - 24
				}
				data = patternedData(size)
				conn.ChangeProperty(WindowID(1), Atom(2), Atom(3), tc.format, tc.mode, data)
			})
			if len(requests) != len(tc.units) {
				t.Fatalf("expected %d requests, got %d", len(tc.units), len(requests))
			}
			reassembled := make([]byte, 0, len(data))
			for i, raw := range requests {
				req := parseChangeProperty(t, raw)
				if req.words > int(tc.maxWords) {
					t.Errorf("request %d is %d words, exceeding the server maximum of %d", i, req.words, tc.maxWords)
				}
				c.Equal(tc.words[i], req.words, "request %d", i)
				c.Equal(tc.modes[i], req.mode, "request %d", i)
				c.Equal(tc.units[i], req.unitCount, "request %d", i)
				reassembled = append(reassembled, req.data...)
			}
			c.Equal(data, reassembled)
		})
	}
}

// TestChangePropertyRejectsPartialUnitData verifies that data whose length is not a multiple of the format unit size
// is rejected outright rather than silently truncated to a whole number of units on the wire.
func TestChangePropertyRejectsPartialUnitData(t *testing.T) {
	c := check.New(t)
	requests := captureRequests(t, math.MaxUint16, func(conn *Conn) {
		conn.ChangeProperty(WindowID(1), Atom(2), Atom(3), 16, PropModeReplace, patternedData(5))
		conn.ChangeProperty(WindowID(1), Atom(2), Atom(3), 32, PropModeAppend, patternedData(6))
	})
	c.Equal(0, len(requests))
}

// TestChangePropertyPrependChunking verifies that oversized prepends are split into multiple prepend requests sent in
// reverse order, so that the property ends up holding the data in its original order once all chunks have landed.
func TestChangePropertyPrependChunking(t *testing.T) {
	c := check.New(t)
	const maxWords = 4096
	const maxDataUnits = (maxWords - 6) * 4 / 2 // format 16: 2 bytes per unit
	data := patternedData((maxDataUnits*2 + 3) * 2)
	requests := captureRequests(t, maxWords, func(conn *Conn) {
		conn.ChangeProperty(WindowID(1), Atom(2), Atom(3), 16, PropModePrepend, data)
	})
	c.Equal(3, len(requests))
	// Simulate the property: each prepend lands ahead of the previously written content.
	var property []byte
	for i, raw := range requests {
		req := parseChangeProperty(t, raw)
		c.Equal(byte(PropModePrepend), req.mode)
		if req.words > maxWords {
			t.Errorf("request %d is %d words, exceeding the server maximum of %d", i, req.words, maxWords)
		}
		property = append(append([]byte{}, req.data...), property...)
	}
	c.Equal(uint32(maxDataUnits), parseChangeProperty(t, requests[0]).unitCount)
	c.Equal(uint32(3), parseChangeProperty(t, requests[2]).unitCount)
	c.Equal(data, property)
}

// TestParseDisplayEnv verifies DISPLAY parsing, in particular that the screen separator dot is only recognized after
// the colon, since hostnames routinely contain dots (e.g. DISPLAY=myhost.example.com:0), and that the display field
// always ends up holding just the display number, which readAuthority compares against Xauthority entries.
func TestParseDisplayEnv(t *testing.T) {
	cases := []struct {
		env           string
		socket        string
		protocol      string
		host          string
		display       string
		screen        string
		displayNum    int
		defaultScreen int
		wantErr       bool
	}{
		{env: ":0", display: "0"},
		{env: ":1", display: "1", displayNum: 1},
		{env: ":0.1", display: "0", screen: "1", defaultScreen: 1},
		{env: ":2.0", display: "2", screen: "0", displayNum: 2},
		{env: ":0.", display: "0"},
		{env: "myhost:0", host: "myhost", display: "0"},
		{env: "myhost.example.com:0", host: "myhost.example.com", display: "0"},
		{env: "myhost.example.com:2.1", host: "myhost.example.com", display: "2", screen: "1", displayNum: 2, defaultScreen: 1},
		{env: "localhost:10.0", host: "localhost", display: "10", screen: "0", displayNum: 10},
		{env: "tcp/myhost:1", protocol: "tcp", host: "myhost", display: "1", displayNum: 1},
		{env: "tcp/myhost.example.com:1", protocol: "tcp", host: "myhost.example.com", display: "1", displayNum: 1},
		{env: "/run/user/1000/x11-display:0", socket: "/run/user/1000/x11-display", display: "0"},
		{env: "", wantErr: true},
		{env: "myhost", wantErr: true},
		{env: ":", wantErr: true},
		{env: ":abc", wantErr: true},
		{env: ":0.x", wantErr: true},
		{env: ":-1", wantErr: true},
		{env: ":0.-1", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			c := check.New(t)
			t.Setenv("DISPLAY", tc.env)
			var conn Conn
			err := conn.parseDisplayEnv()
			if tc.wantErr {
				c.HasError(err)
				return
			}
			c.NoError(err)
			c.Equal(tc.socket, conn.socket)
			c.Equal(tc.protocol, conn.protocol)
			c.Equal(tc.host, conn.host)
			c.Equal(tc.display, conn.display)
			c.Equal(tc.screen, conn.screen)
			c.Equal(tc.displayNum, conn.displayNum)
			c.Equal(tc.defaultScreen, conn.DefaultScreen)
		})
	}
}

// fakeEvent is a minimal Event implementation for exercising the delivery machinery without a live X server.
type fakeEvent struct {
	n int
}

func (e *fakeEvent) ID() byte {
	return 255
}

// TestEventDeliveryDoesNotBlockWithoutConsumer verifies that deliverEvent never blocks, no matter how far event
// delivery gets ahead of consumption. readResponses is the sole reader of the connection, so if delivery could block
// (as it did when events were sent through a bounded channel), a reply queued behind a large event backlog would never
// be read and the connection would deadlock.
func TestEventDeliveryDoesNotBlockWithoutConsumer(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	const total = 10000
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range total {
			conn.deliverEvent(&fakeEvent{n: i})
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("deliverEvent blocked with no consumer draining events")
	}
	for i := range total {
		fe, ok := conn.PollEvents(nil).(*fakeEvent)
		if !ok {
			t.Fatalf("event %d: not a *fakeEvent", i)
		}
		c.Equal(i, fe.n)
	}
	c.True(conn.PollEvents(nil) == nil, "queue must be empty after draining")
}

// TestPostEmptyEventWakesBlockedWaiter verifies that a wake-up posted while WaitEvents is blocked causes it to return
// nil, which is what the event loop relies on to run tasks posted from other goroutines.
func TestPostEmptyEventWakesBlockedWaiter(t *testing.T) {
	conn := &Conn{events: make(chan Event, 1)}
	got := make(chan Event, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		got <- conn.WaitEvents(nil)
	}()
	<-started
	conn.PostEmptyEvent()
	select {
	case e := <-got:
		if e != nil {
			t.Fatalf("expected nil from a wake-up, got %#v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("PostEmptyEvent failed to wake WaitEvents")
	}
}

// TestWaitEventsFilterLeavesNonMatchingQueued verifies that filtered waits return the first matching event while
// leaving non-matching events queued, in order, for later retrieval.
func TestWaitEventsFilterLeavesNonMatchingQueued(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	conn.deliverEvent(&fakeEvent{n: 1})
	conn.deliverEvent(&fakeEvent{n: 2})
	fe, ok := conn.WaitEvents(func(e Event) bool {
		f, ok2 := e.(*fakeEvent)
		return ok2 && f.n == 2
	}).(*fakeEvent)
	c.True(ok)
	c.Equal(2, fe.n)
	fe, ok = conn.PollEvents(nil).(*fakeEvent)
	c.True(ok)
	c.Equal(1, fe.n)
	c.True(conn.PollEvents(nil) == nil, "queue must be empty after draining")
}

// TestShutdownDrainsQueueThenReturnsNil verifies the shutdown contract: once the connection is dead, already-queued
// events are still retrievable, after which every wait returns nil immediately and Dead reports true so callers can
// distinguish a lost connection from an ordinary wake-up.
func TestShutdownDrainsQueueThenReturnsNil(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	conn.deliverEvent(&fakeEvent{n: 1})
	conn.deliverEvent(&fakeEvent{n: 2})
	c.False(conn.Dead())
	conn.closeEvents() // What readResponses does when the connection shuts down.
	c.True(conn.Dead())
	for i := 1; i <= 2; i++ {
		fe, ok := conn.WaitEvents(nil).(*fakeEvent)
		c.True(ok)
		c.Equal(i, fe.n)
	}
	c.True(conn.WaitEvents(nil) == nil, "WaitEvents must return nil once dead and drained")
	c.True(conn.WaitEvents(nil) == nil, "WaitEvents must keep returning nil, not block")
	c.True(conn.WaitEventsUntil(nil, time.Second) == nil, "WaitEventsUntil must return nil once dead and drained")
	c.True(conn.PollEvents(nil) == nil, "PollEvents must return nil once dead and drained")
}

// TestWaitEventsUntilTimesOut verifies that a filtered timed wait with nothing pending returns nil once the timeout
// elapses.
func TestWaitEventsUntilTimesOut(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	start := time.Now()
	c.True(conn.WaitEventsUntil(nil, 50*time.Millisecond) == nil)
	c.True(time.Since(start) >= 50*time.Millisecond, "must not return before the timeout")
}

// TestWaitForWindowVisibilityReturnsOnEvent verifies that WaitForWindowVisibility consumes the VisibilityNotify event
// for the requested window and reports success, leaving events for other windows queued.
func TestWaitForWindowVisibilityReturnsOnEvent(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	conn.deliverEvent(&VisibilityNotifyEvent{Code: eventCodeVisibilityNotify, Window: WindowID(9)})
	conn.deliverEvent(&VisibilityNotifyEvent{Code: eventCodeVisibilityNotify, Window: WindowID(5)})
	c.True(conn.WaitForWindowVisibility(WindowID(5), time.Minute), "must report that the event arrived")
	ev, ok := conn.PollEvents(nil).(*VisibilityNotifyEvent)
	c.True(ok, "the other window's event must remain queued")
	c.Equal(WindowID(9), ev.Window)
	c.True(conn.PollEvents(nil) == nil, "queue must be empty after draining")
}

// TestWaitForWindowVisibilityBounded is the regression test for nativeShow hanging forever when the window manager
// never maps a window: the wait is filtered, so PostEmptyEvent wake-ups cannot unstick it, and MapWindow is intercepted
// via SubstructureRedirect, so a hung or misbehaving window manager may never produce the awaited VisibilityNotify. The
// wait must therefore give up on its own once the timeout elapses, even while wake-ups and non-matching events keep
// arriving.
func TestWaitForWindowVisibilityBounded(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	conn.deliverEvent(&VisibilityNotifyEvent{Code: eventCodeVisibilityNotify, Window: WindowID(9)})
	stop := make(chan struct{})
	defer close(stop)
	go func() { // Keep posting wake-ups, mimicking InvokeTask trying to unstick the UI thread.
		for {
			select {
			case <-stop:
				return
			default:
				conn.PostEmptyEvent()
				time.Sleep(time.Millisecond)
			}
		}
	}()
	start := time.Now()
	done := make(chan bool, 1)
	go func() {
		done <- conn.WaitForWindowVisibility(WindowID(5), 50*time.Millisecond)
	}()
	select {
	case got := <-done:
		c.False(got, "must report that the event never arrived")
		c.True(time.Since(start) >= 50*time.Millisecond, "must not return before the timeout")
	case <-time.After(10 * time.Second):
		t.Fatal("WaitForWindowVisibility failed to time out")
	}
}

// TestReplyNotStarvedByEventBacklog is the regression test for the reply-starvation deadlock: with the main thread
// parked in sendNewRequest waiting for a reply and nothing draining events, readResponses must still be able to work
// through an event backlog larger than any fixed buffer to reach the reply. Before the fix, events were delivered
// through a bounded channel, so a backlog exceeding its capacity blocked readResponses forever and the connection
// deadlocked.
func TestReplyNotStarvedByEventBacklog(t *testing.T) {
	c := check.New(t)
	client, server := net.Pipe()
	conn := &Conn{
		conn:         client,
		events:       make(chan Event, 1),
		requests:     make(chan *request, 128),
		closed:       make(chan struct{}),
		readClosed:   make(chan struct{}),
		eventNewMap:  newEventMap(),
		errorCodeMap: newErrorMap(),
		requestMap:   make(map[uint16]*request),
	}
	go conn.sendRequests()
	go conn.readResponses()
	const backlog = 10000
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- func() error {
			buf := make([]byte, 4) // The GetInputFocus request is a single 4-byte packet.
			if _, err := io.ReadFull(server, buf); err != nil {
				return err
			}
			event := make([]byte, 32)
			event[0] = eventCodeKeyPress
			for range backlog {
				if _, err := server.Write(event); err != nil {
					return err
				}
			}
			reply := make([]byte, 32)
			reply[0] = 1                                 // Reply
			binary.LittleEndian.PutUint16(reply[2:4], 1) // Sequence number of the first request sent
			_, err := server.Write(reply)
			return err
		}()
	}()
	requestDone := make(chan error, 1)
	go func() {
		_, _, err := conn.GetInputFocus()
		requestDone <- err
	}()
	select {
	case err := <-requestDone:
		c.NoError(err)
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock: reply starved behind event backlog")
	}
	c.NoError(<-serverErr)
	count := 0
	for {
		e := conn.PollEvents(nil)
		if e == nil {
			break
		}
		if _, ok := e.(*KeyPressEvent); !ok {
			t.Fatalf("event %d: not a *KeyPressEvent", count)
		}
		count++
	}
	c.Equal(backlog, count)
	close(conn.requests) // Shut the connection down and wait for both goroutines to finish.
	select {
	case <-conn.readClosed:
	case <-time.After(10 * time.Second):
		t.Fatal("connection failed to shut down")
	}
	c.True(conn.Dead())
}

func isMotion(e Event) bool {
	_, ok := e.(*MotionNotifyEvent)
	return ok
}

func isButtonRelease(e Event) bool {
	_, ok := e.(*ButtonReleaseEvent)
	return ok
}

// queueEvents fills a connection's event queue directly, in order.
func queueEvents(events ...Event) *Conn {
	conn := &Conn{events: make(chan Event, 1)}
	for _, e := range events {
		conn.deliverEvent(e)
	}
	return conn
}

// remaining returns the events still queued, in order, draining the queue in the process.
func remaining(conn *Conn) []Event {
	var events []Event
	for {
		e := conn.PollEvents(nil)
		if e == nil {
			return events
		}
		events = append(events, e)
	}
}

// TestPollEventsBeforeReturnsMatchAheadOfBarrier verifies that matching events queued ahead of the barrier are still
// returned, one call at a time, until the barrier is reached, and that a matching event behind it is left alone. Motion
// coalescing during a drag relies on the latter: pulling the motion that arrived after a ButtonRelease out from behind
// it would report the pointer's post-release position as the drop position.
func TestPollEventsBeforeReturnsMatchAheadOfBarrier(t *testing.T) {
	c := check.New(t)
	first := &MotionNotifyEvent{}
	second := &MotionNotifyEvent{}
	release := &ButtonReleaseEvent{}
	behind := &MotionNotifyEvent{}
	conn := queueEvents(first, second, release, behind)

	c.Equal(Event(first), conn.PollEventsBefore(isMotion, isButtonRelease))
	c.Equal(Event(second), conn.PollEventsBefore(isMotion, isButtonRelease))

	// The barrier is now at the head, so the motion behind it stays put.
	c.Nil(conn.PollEventsBefore(isMotion, isButtonRelease))
	c.Equal([]Event{release, behind}, remaining(conn))
}

// TestPollEventsBeforeSkipsUnrelatedEvents verifies that events matching neither the filter nor the barrier are
// skipped over and left queued, as PollEvents does.
func TestPollEventsBeforeSkipsUnrelatedEvents(t *testing.T) {
	c := check.New(t)
	expose := &ExposeEvent{}
	press := &ButtonPressEvent{}
	motion := &MotionNotifyEvent{}
	release := &ButtonReleaseEvent{}
	conn := queueEvents(expose, press, motion, release)

	c.Equal(Event(motion), conn.PollEventsBefore(isMotion, isButtonRelease))
	c.Equal([]Event{expose, press, release}, remaining(conn))
}

// TestPollEventsBeforeWithoutBarrier verifies that a nil barrier makes the call behave like PollEvents, that a nil
// filter takes the event at the head, and that a queue with nothing to return yields nil without consuming anything.
func TestPollEventsBeforeWithoutBarrier(t *testing.T) {
	c := check.New(t)

	c.Nil((&Conn{events: make(chan Event, 1)}).PollEventsBefore(isMotion, isButtonRelease), "an empty queue yields nil")

	release := &ButtonReleaseEvent{}
	motion := &MotionNotifyEvent{}
	conn := queueEvents(release, motion)
	c.Equal(Event(motion), conn.PollEventsBefore(isMotion, nil), "with no barrier, motion behind a release is returned")
	c.Equal([]Event{release}, remaining(conn))

	// A nil filter returns the event at the head, so long as it isn't the barrier itself.
	expose := &ExposeEvent{}
	release = &ButtonReleaseEvent{}
	conn = queueEvents(expose, release)
	c.Equal(Event(expose), conn.PollEventsBefore(nil, isButtonRelease))
	c.Nil(conn.PollEventsBefore(nil, isButtonRelease), "the barrier itself is never returned")
	c.Equal([]Event{release}, remaining(conn))

	// A queue holding nothing that matches yields nil without consuming anything.
	press := &ButtonPressEvent{}
	conn = queueEvents(expose, press)
	c.Nil(conn.PollEventsBefore(isMotion, isButtonRelease))
	c.Equal([]Event{expose, press}, remaining(conn))
}

// TestPostEmptyEventDoesNotBlockWhenFull verifies that posting places a nil wake-up event on the events channel and
// that posting to a full channel returns immediately instead of blocking. Blocking there while holding postEventLock
// would deadlock a concurrent closeEvents during shutdown, and the wake-up is redundant anyway: a full channel already
// guarantees any waiter will wake.
func TestPostEmptyEventDoesNotBlockWhenFull(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	conn.PostEmptyEvent()
	conn.PostEmptyEvent() // Would block forever before the fix's non-blocking send if delivery were attempted.
	c.Equal(1, len(conn.events))
	c.True(<-conn.events == nil, "the wake-up event must be nil")
}

// TestPostEmptyEventSafeOnUnconnectedConn verifies that posting on a zero-value Conn (nil events channel) is a no-op
// rather than a permanent block or panic. app_linux.go's shutdown race test relies on this to exercise the wake path
// without a live X server.
func TestPostEmptyEventSafeOnUnconnectedConn(_ *testing.T) {
	var conn Conn
	conn.PostEmptyEvent() // Must return without blocking or panicking.
}

// TestPostEmptyEventRacesClose is the regression test for the shutdown race: PostEmptyEvent may be called from any
// goroutine while readResponses closes the events channel during Conn.Close. Before the fix, PostEmptyEvent did a
// bare send on the channel, so a post racing the close panicked with "send on closed channel". Run with -race, this
// also proves the eventsClosed flag itself is properly synchronized. Posts after the close must be silent no-ops.
func TestPostEmptyEventRacesClose(_ *testing.T) {
	for range 100 {
		conn := &Conn{events: make(chan Event, 2)}
		start := make(chan struct{})
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				for range 8 {
					conn.PostEmptyEvent()
				}
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			conn.closeEvents() // What readResponses does when the connection shuts down.
		}()
		close(start)
		wg.Wait()
		conn.PostEmptyEvent() // After the close, posting must be a no-op, not a panic.
	}
}

// captureCheckedRequests runs fn against a Conn wired to an in-memory fake X server and returns each raw X11 request
// written, in order. Unlike captureRequests, the fake server replies to the GetInputFocus request that the Sync inside
// every checked request performs (and omits it from the returned requests), so functions built on checked requests can
// run to completion.
func captureCheckedRequests(t *testing.T, maxRequestWords uint16, fn func(c *Conn)) [][]byte {
	t.Helper()
	client, server := net.Pipe()
	conn := &Conn{
		conn:                 client,
		events:               make(chan Event, 1),
		requests:             make(chan *request, 128),
		closed:               make(chan struct{}),
		readClosed:           make(chan struct{}),
		eventNewMap:          newEventMap(),
		errorCodeMap:         newErrorMap(),
		requestMap:           make(map[uint16]*request),
		maximumRequestLength: maxRequestWords,
	}
	go conn.sendRequests()
	go conn.readResponses()
	var captured [][]byte
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- func() error {
			var seq uint16
			header := make([]byte, 4)
			for {
				if _, err := io.ReadFull(server, header); err != nil {
					return nil // The pipe closes when the Conn shuts down, ending the read with an error.
				}
				seq++
				size := int(binary.LittleEndian.Uint16(header[2:4])) * 4
				if size < 4 {
					return fmt.Errorf("invalid request length %d", size)
				}
				req := make([]byte, size)
				copy(req, header)
				if _, err := io.ReadFull(server, req[4:]); err != nil {
					return err
				}
				if req[0] == opGetInputFocus {
					reply := make([]byte, 32)
					reply[0] = 1
					binary.LittleEndian.PutUint16(reply[2:4], seq)
					if _, err := server.Write(reply); err != nil {
						return err
					}
					continue
				}
				captured = append(captured, req)
			}
		}()
	}()
	fn(conn)
	close(conn.requests) // Shut the connection down; sendRequests closes the pipe, unblocking the other goroutines.
	select {
	case <-conn.readClosed:
	case <-time.After(10 * time.Second):
		t.Fatal("connection failed to shut down")
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
	return captured
}

type parsedPutImage struct {
	data          []byte
	words         int
	width, height int
	dstX, dstY    int
	format, depth byte
	leftPad       byte
}

func parsePutImage(t *testing.T, req []byte) parsedPutImage {
	t.Helper()
	if req[0] != opPutImage {
		t.Fatalf("expected PutImage opcode %d, got %d", opPutImage, req[0])
	}
	p := parsedPutImage{
		data:    req[24:],
		words:   len(req) / 4,
		width:   int(binary.LittleEndian.Uint16(req[12:14])),
		height:  int(binary.LittleEndian.Uint16(req[14:16])),
		dstX:    int(int16(binary.LittleEndian.Uint16(req[16:18]))),
		dstY:    int(int16(binary.LittleEndian.Uint16(req[18:20]))),
		leftPad: req[20],
		format:  req[1],
		depth:   req[21],
	}
	if len(p.data) != p.width*p.height*4 {
		t.Fatalf("request carries %d data bytes for a %dx%d image", len(p.data), p.width, p.height)
	}
	return p
}

// putImageTestImage returns a w x h image where every pixel has a distinct NRGBA value, including a mix of alphas.
func putImageTestImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	i := 0
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(i * 7), G: uint8(i*13 + 5), B: uint8(i*29 + 11), A: uint8(i*31 + 100)})
			i++
		}
	}
	return img
}

// premultipliedBGRA returns the expected wire form of the given rectangle of img: rows of pixels converted to
// pre-multiplied BGRA order. The rectangle is relative to img's bounds.
func premultipliedBGRA(img *image.NRGBA, x, y, w, h int) []byte {
	out := make([]byte, 0, w*h*4)
	for row := y; row < y+h; row++ {
		si := img.PixOffset(img.Rect.Min.X+x, img.Rect.Min.Y+row)
		for range w {
			a := uint16(img.Pix[si+3])
			out = append(out,
				uint8(uint16(img.Pix[si+2])*a/0xff),
				uint8(uint16(img.Pix[si+1])*a/0xff),
				uint8(uint16(img.Pix[si])*a/0xff),
				img.Pix[si+3])
			si += 4
		}
	}
	return out
}

// TestPutImageChunksRowsToServerMax verifies the PutImage wire form, a depth 32 ZPixmap with no left pad carrying
// pre-multiplied BGRA pixels, and that a tall image is split into row chunks sized by the server-advertised maximum
// request length (not the compile-time maximum encodable request) with the payload surviving reassembly. A sub-image,
// whose bounds have a non-zero origin and whose stride is wider than its pixel rows, must be read through its own
// geometry rather than as packed, zero-origin pixel data, so its expected bytes come from the parent's coordinates.
func TestPutImageChunksRowsToServerMax(t *testing.T) {
	pixel := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	pixel.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 40, A: 128})
	tall := putImageTestImage(100, 10)
	base := putImageTestImage(8, 8)
	sub, ok := base.SubImage(image.Rect(2, 3, 6, 7)).(*image.NRGBA)
	if !ok {
		t.Fatal("SubImage did not return an *image.NRGBA")
	}
	for _, tc := range []struct {
		name     string
		img      *image.NRGBA
		want     []byte
		heights  []int
		width    int
		dstX     int16
		dstY     int16
		maxWords uint16
	}{
		{
			name:     "single pixel",
			img:      pixel,
			maxWords: math.MaxUint16,
			dstX:     30,
			dstY:     -40,
			width:    1,
			heights:  []int{1},
			// (B, G, R) of (40, 100, 200) pre-multiplied by alpha 128/255, then the original alpha.
			want: []byte{20, 50, 100, 128},
		},
		{
			name:     "tall image splits into row chunks",
			img:      tall,
			maxWords: 306, // Fits 300 pixels per request, which is 3 rows of 100.
			dstX:     5,
			dstY:     7,
			width:    100,
			heights:  []int{3, 3, 3, 1},
			want:     premultipliedBGRA(tall, 0, 0, 100, 10),
		},
		{
			name:     "sub-image respects its stride and origin",
			img:      sub,
			maxWords: math.MaxUint16,
			width:    4,
			heights:  []int{4},
			want:     premultipliedBGRA(base, 2, 3, 4, 4),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			requests := captureCheckedRequests(t, tc.maxWords, func(conn *Conn) {
				conn.PutImage(DrawableID(1), GCID(2), tc.dstX, tc.dstY, tc.img)
			})
			if len(requests) != len(tc.heights) {
				t.Fatalf("expected %d requests, got %d", len(tc.heights), len(requests))
			}
			reassembled := make([]byte, 0, len(tc.want))
			dstY := int(tc.dstY)
			for i, raw := range requests {
				req := parsePutImage(t, raw)
				if req.words > int(tc.maxWords) {
					t.Errorf("request %d is %d words, exceeding the server maximum of %d", i, req.words, tc.maxWords)
				}
				c.Equal(byte(ImageFormatZPixmap), req.format)
				c.Equal(byte(32), req.depth)
				c.Equal(byte(0), req.leftPad)
				c.Equal(tc.width, req.width)
				c.Equal(tc.heights[i], req.height, "request %d", i)
				c.Equal(int(tc.dstX), req.dstX)
				c.Equal(dstY, req.dstY, "request %d", i)
				dstY += tc.heights[i]
				reassembled = append(reassembled, req.data...)
			}
			c.Equal(tc.want, reassembled)
		})
	}
}

// TestPutImageSplitsRowsWiderThanMaxRequest verifies that an image whose single row exceeds the maximum request size
// is split into horizontal spans. Before this was handled, such an image made the row count per request zero and the
// send loop never advanced.
func TestPutImageSplitsRowsWiderThanMaxRequest(t *testing.T) {
	c := check.New(t)
	const maxWords = 106 // Fits 100 pixels per request, less than one 250-pixel row.
	img := putImageTestImage(250, 2)
	requests := captureCheckedRequests(t, maxWords, func(conn *Conn) {
		conn.PutImage(DrawableID(1), GCID(2), 10, 20, img)
	})
	c.Equal(6, len(requests))
	spans := [][2]int{{0, 100}, {100, 100}, {200, 50}}
	for i, raw := range requests {
		req := parsePutImage(t, raw)
		if req.words > maxWords {
			t.Errorf("request %d is %d words, exceeding the server maximum of %d", i, req.words, maxWords)
		}
		row := i / len(spans)
		span := spans[i%len(spans)]
		c.Equal(1, req.height)
		c.Equal(20+row, req.dstY)
		c.Equal(10+span[0], req.dstX)
		c.Equal(span[1], req.width)
		c.Equal(premultipliedBGRA(img, span[0], row, span[1], 1), req.data)
	}
}

// TestPutImageZeroSizedImage verifies that empty images produce no requests. A zero-width image used to divide by
// zero when computing the rows per request.
func TestPutImageZeroSizedImage(t *testing.T) {
	c := check.New(t)
	requests := captureCheckedRequests(t, math.MaxUint16, func(conn *Conn) {
		conn.PutImage(DrawableID(1), GCID(2), 0, 0, image.NewNRGBA(image.Rect(0, 0, 0, 5)))
		conn.PutImage(DrawableID(1), GCID(2), 0, 0, image.NewNRGBA(image.Rect(0, 0, 5, 0)))
	})
	c.Equal(0, len(requests))
}

// premulTestPixels returns a pixel buffer for PutImageRGBAPremul: rowPixels words per row for height rows, where the
// leftmost width words of each row hold distinct premultiplied RGBA values and any remaining stride padding is filled
// with a sentinel that must never appear on the wire.
func premulTestPixels(width, height, rowPixels int) []uint32 {
	pixels := make([]uint32, rowPixels*height)
	i := 0
	for y := range height {
		for x := range rowPixels {
			if x < width {
				pixels[y*rowPixels+x] = uint32(uint8(i*7)) | uint32(uint8(i*13+5))<<8 |
					uint32(uint8(i*29+11))<<16 | uint32(uint8(i*31+100))<<24
				i++
			} else {
				pixels[y*rowPixels+x] = 0xdeadbeef
			}
		}
	}
	return pixels
}

// premulBGRAWire returns the expected wire form of the given rectangle of a premulTestPixels buffer: rows of pixel
// words serialized in BGRA byte order.
func premulBGRAWire(pixels []uint32, rowPixels, x, y, w, h int) []byte {
	out := make([]byte, 0, w*h*4)
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			v := pixels[row*rowPixels+col]
			out = append(out, byte(v>>16), byte(v>>8), byte(v), byte(v>>24))
		}
	}
	return out
}

// TestPutImageRGBAPremulChunksRowsAndRespectsStride verifies the PutImageRGBAPremul wire form, a ZPixmap at the
// caller-supplied depth with no left pad carrying each device word in BGRA byte order, and that a tall upload is split
// into row chunks sized by the server-advertised maximum request length, with rows read through the pixel stride,
// never leaking padding words onto the wire.
func TestPutImageRGBAPremulChunksRowsAndRespectsStride(t *testing.T) {
	const tallWidth, tallHeight, tallRowPixels = 100, 10, 128
	tall := premulTestPixels(tallWidth, tallHeight, tallRowPixels)
	for _, tc := range []struct {
		name      string
		pixels    []uint32
		want      []byte
		heights   []int
		width     int32
		height    int32
		rowPixels int32
		dstX      int16
		dstY      int16
		maxWords  uint16
		depth     byte
	}{
		{
			name:      "single pixel at depth 24",
			pixels:    []uint32{100 | 50<<8 | 20<<16 | 128<<24},
			maxWords:  math.MaxUint16,
			dstX:      30,
			dstY:      -40,
			width:     1,
			height:    1,
			rowPixels: 1,
			depth:     24,
			heights:   []int{1},
			// The (R, G, B, A) device word (100, 50, 20, 128) reordered to wire (B, G, R, A).
			want: []byte{20, 50, 100, 128},
		},
		{
			name:      "tall upload splits into row chunks",
			pixels:    tall,
			maxWords:  306, // Fits 300 pixels per request, which is 3 rows of 100.
			dstX:      5,
			dstY:      7,
			width:     tallWidth,
			height:    tallHeight,
			rowPixels: tallRowPixels,
			depth:     32,
			heights:   []int{3, 3, 3, 1},
			want:      premulBGRAWire(tall, tallRowPixels, 0, 0, tallWidth, tallHeight),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			requests := captureCheckedRequests(t, tc.maxWords, func(conn *Conn) {
				conn.PutImageRGBAPremul(DrawableID(1), GCID(2), tc.dstX, tc.dstY, tc.width, tc.height, tc.rowPixels,
					tc.pixels, tc.depth)
			})
			if len(requests) != len(tc.heights) {
				t.Fatalf("expected %d requests, got %d", len(tc.heights), len(requests))
			}
			reassembled := make([]byte, 0, len(tc.want))
			dstY := int(tc.dstY)
			for i, raw := range requests {
				req := parsePutImage(t, raw)
				if req.words > int(tc.maxWords) {
					t.Errorf("request %d is %d words, exceeding the server maximum of %d", i, req.words, tc.maxWords)
				}
				c.Equal(byte(ImageFormatZPixmap), req.format)
				c.Equal(tc.depth, req.depth)
				c.Equal(byte(0), req.leftPad)
				c.Equal(int(tc.width), req.width)
				c.Equal(tc.heights[i], req.height, "request %d", i)
				c.Equal(int(tc.dstX), req.dstX)
				c.Equal(dstY, req.dstY, "request %d", i)
				dstY += tc.heights[i]
				reassembled = append(reassembled, req.data...)
			}
			c.Equal(tc.want, reassembled)
		})
	}
}

// TestPutImageRGBAPremulSplitsRowsWiderThanMaxRequest verifies that an upload whose single row exceeds the maximum
// request size is split into horizontal spans.
func TestPutImageRGBAPremulSplitsRowsWiderThanMaxRequest(t *testing.T) {
	c := check.New(t)
	const maxWords = 106 // Fits 100 pixels per request, less than one 250-pixel row.
	const width, height = 250, 2
	pixels := premulTestPixels(width, height, width)
	requests := captureCheckedRequests(t, maxWords, func(conn *Conn) {
		conn.PutImageRGBAPremul(DrawableID(1), GCID(2), 10, 20, width, height, width, pixels, 24)
	})
	c.Equal(6, len(requests))
	spans := [][2]int{{0, 100}, {100, 100}, {200, 50}}
	for i, raw := range requests {
		req := parsePutImage(t, raw)
		if req.words > maxWords {
			t.Errorf("request %d is %d words, exceeding the server maximum of %d", i, req.words, maxWords)
		}
		row := i / len(spans)
		span := spans[i%len(spans)]
		c.Equal(1, req.height)
		c.Equal(20+row, req.dstY)
		c.Equal(10+span[0], req.dstX)
		c.Equal(span[1], req.width)
		c.Equal(premulBGRAWire(pixels, width, span[0], row, span[1], 1), req.data)
	}
}

// TestPutImageRGBAPremulZeroSized verifies that empty uploads produce no requests.
func TestPutImageRGBAPremulZeroSized(t *testing.T) {
	c := check.New(t)
	requests := captureCheckedRequests(t, math.MaxUint16, func(conn *Conn) {
		conn.PutImageRGBAPremul(DrawableID(1), GCID(2), 0, 0, 0, 5, 1, nil, 24)
		conn.PutImageRGBAPremul(DrawableID(1), GCID(2), 0, 0, 5, 0, 5, nil, 24)
	})
	c.Equal(0, len(requests))
}

// reversedWordOrder returns the wire data with each 32-bit word's bytes reversed, turning the LSBFirst BGRA byte
// sequences the expectation helpers produce into their MSBFirst ARGB equivalents.
func reversedWordOrder(t *testing.T, data []byte) []byte {
	t.Helper()
	if len(data)%4 != 0 {
		t.Fatalf("data length %d is not a multiple of 4", len(data))
	}
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += 4 {
		out[i] = data[i+3]
		out[i+1] = data[i+2]
		out[i+2] = data[i+1]
		out[i+3] = data[i]
	}
	return out
}

// TestPutImageHonorsServerImageByteOrder verifies that pixel words are emitted in the server's advertised
// image-byte-order. Unlike property data, ZPixmap image data is not byte-swapped by the server, so against an
// MSBFirst server the previously hard-coded little-endian BGRA byte sequences rendered with scrambled channels.
func TestPutImageHonorsServerImageByteOrder(t *testing.T) {
	c := check.New(t)
	img := putImageTestImage(3, 2)
	requests := captureRequests(t, math.MaxUint16, func(conn *Conn) {
		conn.imageByteOrder = imageByteOrderMSBFirst
		conn.PutImage(DrawableID(1), GCID(2), 0, 0, img)
	})
	c.Equal(1, len(requests))
	req := parsePutImage(t, requests[0])
	c.Equal(reversedWordOrder(t, premultipliedBGRA(img, 0, 0, 3, 2)), req.data)
}

// TestPutImageRGBAPremulHonorsServerImageByteOrder is the premultiplied-RGBA companion to
// TestPutImageHonorsServerImageByteOrder.
func TestPutImageRGBAPremulHonorsServerImageByteOrder(t *testing.T) {
	c := check.New(t)
	const width, height = 3, 2
	pixels := premulTestPixels(width, height, width)
	requests := captureRequests(t, math.MaxUint16, func(conn *Conn) {
		conn.imageByteOrder = imageByteOrderMSBFirst
		conn.PutImageRGBAPremul(DrawableID(1), GCID(2), 0, 0, width, height, width, pixels, 24)
	})
	c.Equal(1, len(requests))
	req := parsePutImage(t, requests[0])
	c.Equal(reversedWordOrder(t, premulBGRAWire(pixels, width, 0, 0, width, height)), req.data)
}

// TestPutImagePipelinesChunksWithoutSync verifies that a chunked upload goes out as a pure stream of PutImage
// requests with no interleaved synchronization round-trips. The captureRequests fake server never replies to
// anything, so a regression back to one checked request (and its GetInputFocus round-trip) per chunk deadlocks here
// and is caught by the test timeout; the opcode check additionally documents that nothing but image data goes on the
// wire in the presentation hot path.
func TestPutImagePipelinesChunksWithoutSync(t *testing.T) {
	c := check.New(t)
	const maxWords = 306 // Fits 300 pixels per request, which is 3 rows of 100.
	img := putImageTestImage(100, 10)
	requests := captureRequests(t, maxWords, func(conn *Conn) {
		conn.PutImage(DrawableID(1), GCID(2), 0, 0, img)
	})
	c.Equal(4, len(requests))
	for i, raw := range requests {
		if raw[0] != opPutImage {
			t.Errorf("request %d has opcode %d; the chunk stream must contain only PutImage requests", i, raw[0])
		}
	}
	requests = captureRequests(t, maxWords, func(conn *Conn) {
		conn.PutImageRGBAPremul(DrawableID(1), GCID(2), 0, 0, 100, 10, 100, premulTestPixels(100, 10, 100), 24)
	})
	c.Equal(4, len(requests))
	for i, raw := range requests {
		if raw[0] != opPutImage {
			t.Errorf("request %d has opcode %d; the chunk stream must contain only PutImage requests", i, raw[0])
		}
	}
}

// TestPutImageRGBAPremulReusesScratchBuffer verifies that the wire-format staging buffer lives on the connection and
// only grows, so neither the chunks of one frame nor the frames after it allocate a buffer of their own. The payload
// comparisons guard the other half of that bargain: reuse is only safe because the request buffer copies the staged
// bytes as it is handed them, so a regression there would show up as chunks carrying a later chunk's pixels, and the
// buffer the large frame leaves oversized must not leak stale bytes into the small frame that follows.
func TestPutImageRGBAPremulReusesScratchBuffer(t *testing.T) {
	c := check.New(t)
	const maxWords = 306 // Fits 300 pixels per request, which is 3 rows of 100.
	const width, height, rowPixels = 100, 10, 128
	pixels := premulTestPixels(width, height, rowPixels)
	small := premulTestPixels(4, 2, 4)
	// base identifies the scratch buffer's storage, and tolerates a buffer that was never allocated so that a
	// regression to per-chunk buffers reports a failure rather than panicking.
	base := func(conn *Conn) *byte {
		if len(conn.putImageScratch) == 0 {
			return nil
		}
		return &conn.putImageScratch[0]
	}
	requests := captureCheckedRequests(t, maxWords, func(conn *Conn) {
		c.Equal(0, cap(conn.putImageScratch)) // Nothing is staged until a chunk needs it.
		conn.PutImageRGBAPremul(DrawableID(1), GCID(2), 0, 0, width, height, rowPixels, pixels, 32)
		// The chunks are 3, 3, 3 then 1 rows, so the buffer sized for the largest of them is what remains at the end
		// of the frame; a per-chunk allocation would have left the trailing chunk's smaller one instead.
		c.Equal(width*3*4, cap(conn.putImageScratch))
		first := base(conn)
		conn.PutImageRGBAPremul(DrawableID(1), GCID(2), 0, 0, width, height, rowPixels, pixels, 32)
		c.True(first != nil && first == base(conn), "the next frame must stage into the same buffer")
		conn.PutImageRGBAPremul(DrawableID(1), GCID(2), 0, 0, 4, 2, 4, small, 32)
		c.True(first != nil && first == base(conn), "a smaller frame must reuse the buffer rather than replace it")
		c.Equal(width*3*4, cap(conn.putImageScratch))
	})
	c.Equal(9, len(requests))
	for _, frame := range [][][]byte{requests[:4], requests[4:8]} {
		reassembled := make([]byte, 0, int(width)*int(height)*4)
		for _, raw := range frame {
			reassembled = append(reassembled, parsePutImage(t, raw).data...)
		}
		c.Equal(premulBGRAWire(pixels, rowPixels, 0, 0, width, height), reassembled)
	}
	c.Equal(premulBGRAWire(small, 4, 0, 0, 4, 2), parsePutImage(t, requests[8]).data)
}

// fakeAddrConn is a net.Conn stand-in whose only working method is RemoteAddr, which is all readAuthority needs.
type fakeAddrConn struct {
	net.Conn
	remote net.Addr
}

func (f *fakeAddrConn) RemoteAddr() net.Addr {
	return f.remote
}

// xauthEntry serializes one Xauthority file entry: a big-endian uint16 family followed by size-prefixed address,
// display, name, and data fields.
func xauthEntry(family uint16, addr []byte, display, name string, data []byte) []byte {
	var buf bytes.Buffer
	field := func(b []byte) {
		var size [2]byte
		binary.BigEndian.PutUint16(size[:], uint16(len(b)))
		buf.Write(size[:])
		buf.Write(b)
	}
	var fam [2]byte
	binary.BigEndian.PutUint16(fam[:], family)
	buf.Write(fam[:])
	field(addr)
	field([]byte(display))
	field([]byte(name))
	field(data)
	return buf.Bytes()
}

// writeXauthorityFile writes the given entries to a temporary Xauthority file and points XAUTHORITY at it for the
// duration of the test.
func writeXauthorityFile(t *testing.T, entries ...[]byte) {
	t.Helper()
	fileName := filepath.Join(t.TempDir(), "Xauthority")
	if err := os.WriteFile(fileName, bytes.Join(entries, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XAUTHORITY", fileName)
}

// TestXauthAddressMatches exercises the per-family Xauthority address matching, including the FamilyInternet and
// FamilyInternet6 binary-IP forms that remote TCP displays store their cookies under.
func TestXauthAddressMatches(t *testing.T) {
	c := check.New(t)
	v4 := net.ParseIP("192.168.1.10")
	v6 := net.ParseIP("fd00::1234")
	// Wild entries match any connection.
	c.True(xauthAddressMatches(xauthFamilyWild, "ignored", "myhost", nil))
	c.True(xauthAddressMatches(xauthFamilyWild, "", "myhost", v4))
	// Local entries match on the hostname string.
	c.True(xauthAddressMatches(xauthFamilyLocal, "myhost", "myhost", nil))
	c.False(xauthAddressMatches(xauthFamilyLocal, "otherhost", "myhost", nil))
	// Internet entries match on the connection's remote IPv4 address, in binary form.
	c.True(xauthAddressMatches(xauthFamilyInternet, string(v4.To4()), "myhost", v4))
	c.False(xauthAddressMatches(xauthFamilyInternet, string(net.ParseIP("10.0.0.1").To4()), "myhost", v4))
	c.False(xauthAddressMatches(xauthFamilyInternet, string(v4.To4()), "myhost", nil), "local connections have no remote IP")
	c.False(xauthAddressMatches(xauthFamilyInternet, string(v4.To4()), "myhost", v6))
	// Internet6 entries match on the connection's remote IPv6 address, in binary form.
	c.True(xauthAddressMatches(xauthFamilyInternet6, string(v6.To16()), "myhost", v6))
	c.False(xauthAddressMatches(xauthFamilyInternet6, string(net.ParseIP("fd00::5678").To16()), "myhost", v6))
	c.False(xauthAddressMatches(xauthFamilyInternet6, string(v6.To16()), "myhost", v4), "an IPv4 connection must not match an IPv6 entry")
	c.False(xauthAddressMatches(xauthFamilyInternet6, string(v6.To16()), "myhost", nil))
}

// TestReadAuthorityMatchesInternetFamily is the regression test for remote TCP displays being unable to authenticate:
// their Xauthority entries are stored as FamilyInternet/FamilyInternet6 with a binary IP address, which the scan
// previously could never match (it only accepted FamilyWild and FamilyLocal), so any remote server requiring
// MIT-MAGIC-COOKIE-1 refused the connection. An IPv6 connection must take its FamilyInternet6 entry and never the
// FamilyInternet form, a local connection must match no Internet entry, and FamilyLocal and FamilyWild entries must
// still match.
func TestReadAuthorityMatchesInternetFamily(t *testing.T) {
	const authName = "MIT-MAGIC-COOKIE-1"
	v4 := net.ParseIP("192.168.1.10")
	v4Cookie := []byte{0xde, 0xad, 0xbe, 0xef, 1, 2, 3, 4}
	v4Entries := [][]byte{
		xauthEntry(xauthFamilyLocal, []byte("otherhost"), "0", authName, []byte("wrong-local")),
		xauthEntry(xauthFamilyInternet, net.ParseIP("10.0.0.1").To4(), "0", authName, []byte("wrong-address")),
		xauthEntry(xauthFamilyInternet, v4.To4(), "1", authName, []byte("wrong-display")),
		xauthEntry(xauthFamilyInternet, v4.To4(), "0", authName, v4Cookie),
	}
	v6 := net.ParseIP("fd00::1234")
	v6Cookie := []byte{9, 8, 7, 6}
	localCookie := []byte{1, 1, 2, 2}
	wildCookie := []byte{3, 3, 4, 4}
	const localHost = "myhost"
	localAndWildEntries := [][]byte{
		xauthEntry(xauthFamilyLocal, []byte(localHost), "0", authName, localCookie),
		xauthEntry(xauthFamilyWild, nil, "", authName, wildCookie),
	}
	for _, tc := range []struct {
		name     string
		host     string
		wantName string
		entries  [][]byte
		// remote is the server's address for a TCP connection; nil means a local (unix socket) connection.
		remote   net.IP
		wantData []byte
	}{
		{
			name:     "IPv4 connection takes the Internet entry for its address and display",
			entries:  v4Entries,
			remote:   v4,
			host:     "192.168.1.10",
			wantName: authName,
			wantData: v4Cookie,
		},
		{
			name:    "local connection has no remote IP to match an Internet entry",
			entries: v4Entries,
			host:    localHost,
		},
		{
			name: "IPv6 connection takes the Internet6 entry",
			entries: [][]byte{
				xauthEntry(xauthFamilyInternet, v6.To16()[:4], "0", authName, []byte("wrong-family")),
				xauthEntry(xauthFamilyInternet6, v6.To16(), "0", authName, v6Cookie),
			},
			remote:   v6,
			host:     "fd00::1234",
			wantName: authName,
			wantData: v6Cookie,
		},
		{
			name:     "FamilyLocal entry matches the hostname",
			entries:  localAndWildEntries,
			host:     localHost,
			wantName: authName,
			wantData: localCookie,
		},
		{
			name:     "FamilyWild entry with no display matches any other host",
			entries:  localAndWildEntries,
			host:     "someotherhost",
			wantName: authName,
			wantData: wildCookie,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			writeXauthorityFile(t, tc.entries...)
			conn := &Conn{display: "0"}
			if tc.remote != nil {
				conn.conn = &fakeAddrConn{remote: &net.TCPAddr{IP: tc.remote, Port: 6000}}
			}
			name, data := conn.readAuthority(tc.host)
			c.Equal(tc.wantName, name)
			c.True(bytes.Equal(tc.wantData, data), "expected cookie %v, got %v", tc.wantData, data)
		})
	}
}
