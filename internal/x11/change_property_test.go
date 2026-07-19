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
	"io"
	"math"
	"net"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

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

// TestChangePropertyIncrChunkIsSingleRequest verifies the contract completeIncrTransfers relies upon: a write of one
// full INCR chunk (the maximum request size minus the 24-byte ChangeProperty header) must go out as a single
// ChangeProperty request, so the requestor sees exactly one PropertyNotify per chunk.
func TestChangePropertyIncrChunkIsSingleRequest(t *testing.T) {
	c := check.New(t)
	var chunkSize int
	requests := captureRequests(t, math.MaxUint16, func(conn *Conn) {
		chunkSize = conn.incrThreshold() - 24
		conn.ChangeProperty(WindowID(1), Atom(2), Atom(3), 8, PropModeReplace, patternedData(chunkSize))
	})
	c.Equal(262116, chunkSize)
	c.Equal(1, len(requests))
	req := parseChangeProperty(t, requests[0])
	c.Equal(byte(PropModeReplace), req.mode)
	c.Equal(uint32(chunkSize), req.unitCount)
	c.Equal(math.MaxUint16, req.words)
	c.Equal(patternedData(chunkSize), req.data)
}

// TestChangePropertySplitsOversizedData verifies that data too large for one request is split into a Replace followed
// by Append requests, each within the maximum request size, and that the payload survives reassembly.
func TestChangePropertySplitsOversizedData(t *testing.T) {
	c := check.New(t)
	const maxData = (math.MaxUint16 - 6) * 4
	data := patternedData(maxData + 5)
	requests := captureRequests(t, math.MaxUint16, func(conn *Conn) {
		conn.ChangeProperty(WindowID(1), Atom(2), Atom(3), 8, PropModeReplace, data)
	})
	c.Equal(2, len(requests))
	first := parseChangeProperty(t, requests[0])
	c.Equal(byte(PropModeReplace), first.mode)
	c.Equal(uint32(maxData), first.unitCount)
	second := parseChangeProperty(t, requests[1])
	c.Equal(byte(PropModeAppend), second.mode)
	c.Equal(uint32(5), second.unitCount)
	c.Equal(data, append(first.data, second.data...))
}

// TestChangePropertyHonorsServerMaximumRequestLength verifies that chunking respects the server's advertised maximum
// request length even when it is smaller than the largest encodable request.
func TestChangePropertyHonorsServerMaximumRequestLength(t *testing.T) {
	c := check.New(t)
	const maxWords = 4096
	const maxDataUnits = (maxWords - 6) * 4 / 4 // format 32: 4 bytes per unit
	data := patternedData((maxDataUnits + 2) * 4)
	requests := captureRequests(t, maxWords, func(conn *Conn) {
		conn.ChangeProperty(WindowID(1), Atom(2), Atom(3), 32, PropModeReplace, data)
	})
	c.Equal(2, len(requests))
	var reassembled []byte
	for i, raw := range requests {
		req := parseChangeProperty(t, raw)
		if req.words > maxWords {
			t.Errorf("request %d is %d words, exceeding the server maximum of %d", i, req.words, maxWords)
		}
		reassembled = append(reassembled, req.data...)
	}
	c.Equal(uint32(maxDataUnits), parseChangeProperty(t, requests[0]).unitCount)
	c.Equal(uint32(2), parseChangeProperty(t, requests[1]).unitCount)
	c.Equal(data, reassembled)
}

// TestChangePropertyFormat16Chunking verifies that chunk boundaries land on format unit boundaries for 16-bit data.
func TestChangePropertyFormat16Chunking(t *testing.T) {
	c := check.New(t)
	const maxWords = 4096
	const maxDataUnits = (maxWords - 6) * 4 / 2 // format 16: 2 bytes per unit
	data := patternedData((maxDataUnits + 1) * 2)
	requests := captureRequests(t, maxWords, func(conn *Conn) {
		conn.ChangeProperty(WindowID(1), Atom(2), Atom(3), 16, PropModeReplace, data)
	})
	c.Equal(2, len(requests))
	first := parseChangeProperty(t, requests[0])
	c.Equal(uint32(maxDataUnits), first.unitCount)
	second := parseChangeProperty(t, requests[1])
	c.Equal(uint32(1), second.unitCount)
	c.Equal(data, append(first.data, second.data...))
}

// TestChangePropertyZeroLengthStillSends verifies that a zero-length write still produces a single request, since the
// INCR protocol uses the resulting PropertyNotify to signal the end of a transfer.
func TestChangePropertyZeroLengthStillSends(t *testing.T) {
	c := check.New(t)
	requests := captureRequests(t, math.MaxUint16, func(conn *Conn) {
		conn.ChangeProperty(WindowID(1), Atom(2), Atom(3), 8, PropModeReplace, nil)
	})
	c.Equal(1, len(requests))
	req := parseChangeProperty(t, requests[0])
	c.Equal(uint32(0), req.unitCount)
	c.Equal(6, req.words)
}
