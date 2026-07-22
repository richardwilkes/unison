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
	"fmt"
	"io"
	"math"
	"net"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
)

// newClipboardTestConn creates a Conn whose wire is one end of an in-memory pipe and returns the other end for a fake
// X server to drive. The caller is responsible for starting the sendRequests and readResponses goroutines.
func newClipboardTestConn(helper WindowID) (conn *Conn, server net.Conn) {
	var client net.Conn
	client, server = net.Pipe()
	conn = &Conn{
		conn:                 client,
		events:               make(chan Event, 1),
		requests:             make(chan *request, 128),
		closed:               make(chan struct{}),
		readClosed:           make(chan struct{}),
		eventNewMap:          newEventMap(),
		errorCodeMap:         newErrorMap(),
		requestMap:           make(map[uint16]*request),
		maximumRequestLength: math.MaxUint16,
		helperWindow:         helper,
	}
	conn.Atoms.ClipboardSelection = Atom(200)
	conn.Atoms.ClipboardIncremental = Atom(201)
	return conn, server
}

// readX11Request reads one complete X11 request from the fake server's side of the pipe.
func readX11Request(conn net.Conn) ([]byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	size := int(binary.LittleEndian.Uint16(header[2:4])) * 4
	if size < 4 {
		return nil, fmt.Errorf("invalid request length %d", size)
	}
	buf := make([]byte, size)
	copy(buf, header)
	if _, err := io.ReadFull(conn, buf[4:]); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeSelectionNotify(conn net.Conn, requestor WindowID, selection, target, property Atom) error {
	buf := make([]byte, 32)
	buf[0] = eventCodeSelectionNotify
	binary.LittleEndian.PutUint32(buf[8:12], uint32(requestor))
	binary.LittleEndian.PutUint32(buf[12:16], uint32(selection))
	binary.LittleEndian.PutUint32(buf[16:20], uint32(target))
	binary.LittleEndian.PutUint32(buf[20:24], uint32(property))
	_, err := conn.Write(buf)
	return err
}

func writePropertyNotify(conn net.Conn, window WindowID, property Atom, state byte) error {
	buf := make([]byte, 32)
	buf[0] = eventCodePropertyNotify
	binary.LittleEndian.PutUint32(buf[4:8], uint32(window))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(property))
	buf[16] = state
	_, err := conn.Write(buf)
	return err
}

func writeGetPropertyReply(conn net.Conn, seq uint16, format byte, propertyType Atom, value []byte) error {
	buf := make([]byte, 32+pad4(len(value)))
	buf[0] = 1 // Reply
	buf[1] = format
	binary.LittleEndian.PutUint16(buf[2:4], seq)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(pad4(len(value))/4))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(propertyType))
	if format != 0 {
		binary.LittleEndian.PutUint32(buf[16:20], uint32(len(value)/(int(format)/8)))
	}
	copy(buf[32:], value)
	_, err := conn.Write(buf)
	return err
}

// expectRequest reads the next request and verifies its opcode.
func expectRequest(conn net.Conn, opcode byte) error {
	req, err := readX11Request(conn)
	if err != nil {
		return err
	}
	if req[0] != opcode {
		return fmt.Errorf("expected opcode %d, got %d", opcode, req[0])
	}
	return nil
}

// shutdownClipboardTestConn shuts the connection down and waits for its goroutines to finish.
func shutdownClipboardTestConn(t *testing.T, conn *Conn) {
	t.Helper()
	close(conn.requests)
	select {
	case <-conn.readClosed:
	case <-time.After(10 * time.Second):
		t.Fatal("connection failed to shut down")
	}
}

// TestConvertSelectionDrainsStaleEventsBeforeIncrPaste verifies that stale SelectionNotify and PropertyNotify events
// left queued by a previously timed-out conversion (filtered waits keep non-matching events queued forever) are all
// drained before a new conversion starts. Before the fix, convertSelection drained at most one stale PropertyNotify
// and no stale SelectionNotify, so the INCR loop would consume a stale notification, read the property before the
// owner had written the next chunk, and return a truncated transfer as complete.
func TestConvertSelectionDrainsStaleEventsBeforeIncrPaste(t *testing.T) {
	c := check.New(t)
	const helper = WindowID(7)
	const selection = Atom(300)
	const target = Atom(301)
	conn, server := newClipboardTestConn(helper)
	prop := conn.Atoms.ClipboardSelection
	// Queue the leftovers of an earlier conversion that timed out partway through an INCR transfer.
	for range 2 {
		conn.deliverEvent(&SelectionNotifyEvent{
			Code:      eventCodeSelectionNotify,
			Requestor: helper,
			Selection: selection,
			Target:    target,
			Property:  prop,
		})
	}
	for range 3 {
		conn.deliverEvent(&PropertyNotifyEvent{
			Code:   eventCodePropertyNotify,
			Window: helper,
			Atom:   prop,
			State:  PropertyNewValue,
		})
	}
	go conn.sendRequests()
	go conn.readResponses()
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- func() error {
			// The owner writes the INCR size to the property and then announces the conversion.
			if err := expectRequest(server, opConvertSelection); err != nil {
				return err
			}
			if err := writePropertyNotify(server, helper, prop, PropertyNewValue); err != nil {
				return err
			}
			if err := writeSelectionNotify(server, helper, selection, target, prop); err != nil {
				return err
			}
			// The requestor reads and deletes the INCR marker (sequence 2; ConvertSelection was sequence 1).
			if err := expectRequest(server, opGetProperty); err != nil {
				return err
			}
			total := make([]byte, 4)
			binary.LittleEndian.PutUint32(total, 8)
			if err := writeGetPropertyReply(server, 2, 32, conn.Atoms.ClipboardIncremental, total); err != nil {
				return err
			}
			// The owner writes each chunk after the prior one has been consumed, ending with a zero-length write.
			seq := uint16(3)
			for _, chunk := range [][]byte{[]byte("AAAA"), []byte("BBBB"), nil} {
				if err := writePropertyNotify(server, helper, prop, PropertyNewValue); err != nil {
					return err
				}
				if err := expectRequest(server, opGetProperty); err != nil {
					return err
				}
				if err := writeGetPropertyReply(server, seq, 8, target, chunk); err != nil {
					return err
				}
				seq++
			}
			return nil
		}()
	}()
	type result struct {
		value []byte
		ok    bool
	}
	done := make(chan result, 1)
	go func() {
		value, ok := conn.convertSelection(selection, target, 0)
		done <- result{value: value, ok: ok}
	}()
	select {
	case res := <-done:
		c.True(res.ok, "conversion must succeed")
		c.Equal([]byte("AAAABBBB"), res.value)
	case <-time.After(10 * time.Second):
		t.Fatal("convertSelection did not complete")
	}
	c.NoError(<-serverErr)
	// Every stale event must have been drained and every live event consumed; anything left queued means a stale
	// event was mistaken for part of the transfer, or a live event was displaced by one.
	c.True(conn.PollEvents(nil) == nil, "event queue must be empty after the transfer")
	shutdownClipboardTestConn(t, conn)
}

// TestIncrSendDrainsStalePropertyDeleteBeforeTransfer is the send-side companion to the receive-side drain test above:
// a PropertyDelete for the same (requestor, property) pair left queued by an earlier abandoned INCR transfer (a delete
// arriving after incrSendTimeout stays queued while inside filtered event loops) must be drained before a new transfer
// writes its INCR size marker. Before the fix, the first wait in completeIncrTransfers consumed the stale delete and
// wrote chunk 1 immediately, replacing the size marker before the requestor had read it and corrupting the handoff —
// notably to the clipboard manager at app quit. The drain leaves exactly one wait per requestor-consumed delete, so a
// stale event would surface as a leftover in the queue at the end of the transfer.
func TestIncrSendDrainsStalePropertyDeleteBeforeTransfer(t *testing.T) {
	c := check.New(t)
	const requestor = WindowID(33)
	const property = Atom(44)
	const kind = Atom(45)
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
		maximumRequestLength: 16, // incrThreshold is 64 bytes, so chunks carry 40 bytes each.
	}
	conn.Atoms.ClipboardIncremental = Atom(201)
	data := patternedData(100) // Splits into chunks of 40, 40, and 20 bytes plus the final zero-length write.
	// The leftover of an earlier transfer to the same requestor and property that was abandoned after its timeout.
	conn.deliverEvent(&PropertyNotifyEvent{
		Code:   eventCodePropertyNotify,
		Window: requestor,
		Atom:   property,
		State:  PropertyDelete,
	})
	go conn.sendRequests()
	go conn.readResponses()
	type serverResult struct {
		err    error
		chunks [][]byte
	}
	serverDone := make(chan serverResult, 1)
	go func() {
		serverDone <- func() serverResult {
			// A fake requestor: it "consumes" the INCR size marker and then each non-empty chunk by deleting the
			// property, which arrives at the sender as a PropertyDelete event. GetInputFocus round-trips (the Sync
			// inside checked requests) are answered so ChangeWindowAttributes can complete.
			var result serverResult
			var seq uint16
			for {
				req, err := readX11Request(server)
				if err != nil {
					return result // The pipe closes when the Conn shuts down, ending the read with an error.
				}
				seq++
				switch req[0] {
				case opGetInputFocus:
					reply := make([]byte, 32)
					reply[0] = 1
					binary.LittleEndian.PutUint16(reply[2:4], seq)
					if _, err = server.Write(reply); err != nil {
						result.err = err
						return result
					}
				case opChangeWindowAttributes: // Subscribing to or unsubscribing from property change events.
				case opChangeProperty:
					format := req[16]
					size := int(binary.LittleEndian.Uint32(req[20:24])) * int(format) / 8
					if format == 32 { // The INCR size marker that starts the transfer.
						if err = writePropertyNotify(server, requestor, property, PropertyDelete); err != nil {
							result.err = err
							return result
						}
						continue
					}
					if size > 0 {
						result.chunks = append(result.chunks, req[24:24+size])
						if err = writePropertyNotify(server, requestor, property, PropertyDelete); err != nil {
							result.err = err
							return result
						}
					}
				default:
					result.err = fmt.Errorf("unexpected opcode %d", req[0])
					return result
				}
			}
		}()
	}()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if transfer := conn.writeClipboardProperty(requestor, property,
			clipboardEntry{data: data, target: kind, kind: kind}); transfer != nil {
			conn.completeIncrTransfers([]*incrTransfer{transfer})
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("INCR transfer did not complete")
	}
	// Round-trip once more so any event the fake requestor sent before this point is guaranteed to have been
	// delivered, making the queue-empty check below deterministic.
	_, _, err := conn.GetInputFocus()
	c.NoError(err)
	// Every requestor delete must have been matched by exactly one wait; anything left queued means the stale delete
	// was consumed in place of a real one, letting a chunk overwrite data the requestor had not read yet.
	c.True(conn.PollEvents(nil) == nil, "event queue must be empty after the transfer")
	shutdownClipboardTestConn(t, conn)
	res := <-serverDone
	c.NoError(res.err)
	c.Equal(3, len(res.chunks))
	var reassembled []byte
	for _, chunk := range res.chunks {
		reassembled = append(reassembled, chunk...)
	}
	c.Equal(data, reassembled)
}

// TestConvertSelectionIgnoresStaleSelectionNotify verifies that a stale SelectionNotify from a timed-out conversion in
// which the owner refused the request (Property == AtomNone) does not fail a later conversion. Before the fix, the
// stale event was consumed as the response to the new request, reporting failure even though the owner responds.
func TestConvertSelectionIgnoresStaleSelectionNotify(t *testing.T) {
	c := check.New(t)
	const helper = WindowID(9)
	const selection = Atom(400)
	const target = Atom(401)
	conn, server := newClipboardTestConn(helper)
	prop := conn.Atoms.ClipboardSelection
	conn.deliverEvent(&SelectionNotifyEvent{
		Code:      eventCodeSelectionNotify,
		Requestor: helper,
		Selection: selection,
		Target:    target,
		Property:  AtomNone,
	})
	go conn.sendRequests()
	go conn.readResponses()
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- func() error {
			if err := expectRequest(server, opConvertSelection); err != nil {
				return err
			}
			if err := writeSelectionNotify(server, helper, selection, target, prop); err != nil {
				return err
			}
			if err := expectRequest(server, opGetProperty); err != nil {
				return err
			}
			return writeGetPropertyReply(server, 2, 8, target, []byte("hello"))
		}()
	}()
	type result struct {
		value []byte
		ok    bool
	}
	done := make(chan result, 1)
	go func() {
		value, ok := conn.convertSelection(selection, target, 0)
		done <- result{value: value, ok: ok}
	}()
	select {
	case res := <-done:
		c.True(res.ok, "conversion must succeed despite the stale refusal")
		c.Equal([]byte("hello"), res.value)
	case <-time.After(10 * time.Second):
		t.Fatal("convertSelection did not complete")
	}
	c.NoError(<-serverErr)
	c.True(conn.PollEvents(nil) == nil, "event queue must be empty after the transfer")
	shutdownClipboardTestConn(t, conn)
}
