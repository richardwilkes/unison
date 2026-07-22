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
	"image"
	"image/color"
	"io"
	"math"
	"net"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
)

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

// TestPutImageSingleRequest verifies the wire form of an image that fits in one request, including a hand-computed
// pre-multiplied BGRA pixel.
func TestPutImageSingleRequest(t *testing.T) {
	c := check.New(t)
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 40, A: 128})
	requests := captureCheckedRequests(t, math.MaxUint16, func(conn *Conn) {
		conn.PutImage(DrawableID(1), GCID(2), 30, -40, img)
	})
	c.Equal(1, len(requests))
	req := parsePutImage(t, requests[0])
	c.Equal(byte(ImageFormatZPixmap), req.format)
	c.Equal(byte(32), req.depth)
	c.Equal(byte(0), req.leftPad)
	c.Equal(1, req.width)
	c.Equal(1, req.height)
	c.Equal(30, req.dstX)
	c.Equal(-40, req.dstY)
	c.Equal(7, req.words)
	// (B, G, R) of (40, 100, 200) pre-multiplied by alpha 128/255, then the original alpha.
	c.Equal([]byte{20, 50, 100, 128}, req.data)
}

// TestPutImageChunksRowsToServerMax verifies that a tall image is split into row chunks sized by the server-advertised
// maximum request length (not the compile-time maximum encodable request) and that the payload survives reassembly.
func TestPutImageChunksRowsToServerMax(t *testing.T) {
	c := check.New(t)
	const maxWords = 306 // Fits 300 pixels per request, which is 3 rows of 100.
	img := putImageTestImage(100, 10)
	requests := captureCheckedRequests(t, maxWords, func(conn *Conn) {
		conn.PutImage(DrawableID(1), GCID(2), 5, 7, img)
	})
	c.Equal(4, len(requests))
	var reassembled []byte
	for i, raw := range requests {
		req := parsePutImage(t, raw)
		if req.words > maxWords {
			t.Errorf("request %d is %d words, exceeding the server maximum of %d", i, req.words, maxWords)
		}
		c.Equal(100, req.width)
		c.Equal(5, req.dstX)
		c.Equal(7+i*3, req.dstY)
		reassembled = append(reassembled, req.data...)
	}
	c.Equal(3, parsePutImage(t, requests[0]).height)
	c.Equal(1, parsePutImage(t, requests[3]).height)
	c.Equal(premultipliedBGRA(img, 0, 0, 100, 10), reassembled)
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

// TestPutImageSubImageRespectsStrideAndOrigin verifies that a sub-image — whose bounds have a non-zero origin and
// whose stride is wider than its pixel rows — is read through its own geometry rather than assuming packed,
// zero-origin pixel data. The expected bytes are computed from the parent image's coordinates.
func TestPutImageSubImageRespectsStrideAndOrigin(t *testing.T) {
	c := check.New(t)
	base := putImageTestImage(8, 8)
	sub, ok := base.SubImage(image.Rect(2, 3, 6, 7)).(*image.NRGBA)
	if !ok {
		t.Fatal("SubImage did not return an *image.NRGBA")
	}
	requests := captureCheckedRequests(t, math.MaxUint16, func(conn *Conn) {
		conn.PutImage(DrawableID(1), GCID(2), 0, 0, sub)
	})
	c.Equal(1, len(requests))
	req := parsePutImage(t, requests[0])
	c.Equal(4, req.width)
	c.Equal(4, req.height)
	c.Equal(premultipliedBGRA(base, 2, 3, 4, 4), req.data)
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

// TestPutImageRGBAPremulSingleRequest verifies the wire form of a premultiplied-RGBA upload that fits in one request,
// including a hand-computed BGRA pixel and the caller-supplied drawable depth.
func TestPutImageRGBAPremulSingleRequest(t *testing.T) {
	c := check.New(t)
	pixels := []uint32{100 | 50<<8 | 20<<16 | 128<<24}
	requests := captureCheckedRequests(t, math.MaxUint16, func(conn *Conn) {
		conn.PutImageRGBAPremul(DrawableID(1), GCID(2), 30, -40, 1, 1, 1, pixels, 24)
	})
	c.Equal(1, len(requests))
	req := parsePutImage(t, requests[0])
	c.Equal(byte(ImageFormatZPixmap), req.format)
	c.Equal(byte(24), req.depth)
	c.Equal(byte(0), req.leftPad)
	c.Equal(1, req.width)
	c.Equal(1, req.height)
	c.Equal(30, req.dstX)
	c.Equal(-40, req.dstY)
	c.Equal(7, req.words)
	// The (R, G, B, A) device word (100, 50, 20, 128) reordered to wire (B, G, R, A).
	c.Equal([]byte{20, 50, 100, 128}, req.data)
}

// TestPutImageRGBAPremulChunksRowsAndRespectsStride verifies that a tall upload is split into row chunks sized by the
// server-advertised maximum request length and that rows are read through the pixel stride, never leaking padding
// words onto the wire.
func TestPutImageRGBAPremulChunksRowsAndRespectsStride(t *testing.T) {
	c := check.New(t)
	const maxWords = 306 // Fits 300 pixels per request, which is 3 rows of 100.
	const width, height, rowPixels = 100, 10, 128
	pixels := premulTestPixels(width, height, rowPixels)
	requests := captureCheckedRequests(t, maxWords, func(conn *Conn) {
		conn.PutImageRGBAPremul(DrawableID(1), GCID(2), 5, 7, width, height, rowPixels, pixels, 32)
	})
	c.Equal(4, len(requests))
	var reassembled []byte
	for i, raw := range requests {
		req := parsePutImage(t, raw)
		if req.words > maxWords {
			t.Errorf("request %d is %d words, exceeding the server maximum of %d", i, req.words, maxWords)
		}
		c.Equal(byte(32), req.depth)
		c.Equal(width, req.width)
		c.Equal(5, req.dstX)
		c.Equal(7+i*3, req.dstY)
		reassembled = append(reassembled, req.data...)
	}
	c.Equal(3, parsePutImage(t, requests[0]).height)
	c.Equal(1, parsePutImage(t, requests[3]).height)
	c.Equal(premulBGRAWire(pixels, rowPixels, 0, 0, width, height), reassembled)
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
