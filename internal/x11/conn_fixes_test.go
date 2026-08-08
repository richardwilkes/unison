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
	"encoding/binary"
	"errors"
	"image"
	"io"
	"net"
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

// TestAbortStartupReleasesResources verifies that a NewConn failure after the reader and writer goroutines have been
// started closes the socket and terminates both goroutines instead of leaking them.
func TestAbortStartupReleasesResources(t *testing.T) {
	c := check.New(t)
	conn, server := newPipeConn()
	done := make(chan struct{})
	go func() {
		conn.abortStartup()
		close(done)
	}()
	// The socket must be closed: the server side's read ends once the client end shuts down.
	buffer := make([]byte, 1)
	_, err := server.Read(buffer)
	c.NotNil(err)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("abortStartup failed to terminate the connection goroutines")
	}
	c.True(conn.Dead(), "the event stream must be shut down after an aborted startup")
}

// TestDeadImmediatelyAfterShutdown verifies the shutdown ordering contract: readClosed must be the last thing to close
// during shutdown, so anyone it unblocks (abortStartup, waiters in sendNewRequest) already observes Dead as true. The
// deferred closeEvents in readResponses used to run after readClosed was closed, leaving a window where the connection
// looked fully shut down but Dead still reported false. The loop shuts down many connections because a single pass
// rarely lands in that window.
func TestDeadImmediatelyAfterShutdown(t *testing.T) {
	for i := range 200 {
		conn, server := newPipeConn()
		conn.abortStartup()
		if !conn.Dead() {
			t.Fatalf("iteration %d: Dead must report true once abortStartup returns", i)
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
