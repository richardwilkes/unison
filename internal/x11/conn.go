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
	"log/slog"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"golang.org/x/text/encoding/charmap"
)

// Event represents a generic X11 event. Specific event types will implement this interface.
type Event interface {
	// ID returns a byte value that identifies the type of the event, which can be used to determine how to process it.
	ID() byte
	// TargetWindow returns the ID of the window that is the target of the event, if applicable. For events that do not
	// have a specific target window, this will return WindowNone.
	TargetWindow() WindowID
	// Process the event using the provided connection. The implementation should perform any necessary actions based on
	// the event type and its data.
	Process(*Conn)
}

// WritableEvent represents an event that can be sent to the X server.
type WritableEvent interface {
	Write(sequence uint16, w *Writer)
	Event
}

type request struct {
	sentChan       chan struct{}
	failureChan    chan error
	replyChan      chan *Reader
	replyProcessor func(*Reader)
	data           *Writer
	event          WritableEvent
	sequence       uint16
}

type extensionInfo struct {
	Present      bool
	majorOpcode  byte
	firstEvent   byte
	firstError   byte
	MajorVersion uint32
	MinorVersion uint32
}

// Conn represents a connection to an X server.
type Conn struct {
	conn                     net.Conn
	events                   chan Event
	requests                 chan *request
	closed                   chan struct{}
	readClosed               chan struct{}
	ExtMisc                  *ExtMisc
	ExtRandr                 *ExtRandr
	eventNewMap              map[byte]func(r *Reader) Event
	errorCodeMap             map[byte]string
	requestMap               map[uint16]*request
	envDisplay               string
	socket                   string
	protocol                 string
	host                     string
	display                  string
	screen                   string
	vendor                   string
	clipboard                string
	pixmapFormats            []*Format
	Roots                    []*Screen
	eventNewMapLock          sync.RWMutex
	errorCodeLock            sync.RWMutex
	requestMapLock           sync.RWMutex
	xid                      xid
	DefaultScreen            int
	displayNum               int
	sequence                 atomic.Uint32
	releaseNumber            uint32
	motionBufferSize         uint32
	helperWindow             WindowID
	AtomPair                 Atom
	AtomClipboard            Atom
	AtomClipboardIncremental Atom
	AtomClipboardManager     Atom
	AtomClipboardMultiple    Atom
	AtomClipboardSaveTargets Atom
	AtomClipboardSelection   Atom
	AtomClipboardTargets     Atom
	AtomNull                 Atom
	AtomUTF8String           Atom
	AtomNetWorkArea          Atom
	AtomNetCurrentDesktop    Atom
	protocolMajorVersion     uint16
	protocolMinorVersion     uint16
	maximumRequestLength     uint16
	imageByteOrder           byte
	bitmapFormatBitOrder     byte
	bitmapFormatScanlineUnit byte
	bitmapFormatScanlinePad  byte
	minKeyCode               byte
	maxKeyCode               byte
}

// NewConn establishes a connection to the X server.
func NewConn() (*Conn, error) {
	var c Conn
	var err error
	if err = c.parseDisplayEnv(); err != nil {
		return nil, err
	}
	if err = c.connect(); err != nil {
		return nil, err
	}
	if err = c.authenticate(); err != nil {
		return nil, err
	}
	c.errorCodeMap = newErrorMap()
	c.eventNewMap = newEventMap()
	c.requestMap = make(map[uint16]*request)
	c.requests = make(chan *request, 128)
	c.events = make(chan Event, 8192)
	c.closed = make(chan struct{})
	c.readClosed = make(chan struct{})
	c.ExtMisc = newExtMisc(&c)
	c.ExtRandr = newExtRandr(&c)
	go c.sendRequests()
	go c.readResponses()
	if err = c.initAtoms(); err != nil {
		return nil, err
	}
	if c.helperWindow, err = c.CreateWindow(c.RootWindow(), 0, 0, 1, 1, 0, WindowClassInputOnly, 0, c.DefaultVisual(),
		WindowBitMaskEventMask, &WindowAttributes{EventMask: EventMaskPropertyChange}); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Conn) initAtoms() error {
	var err error
	if c.AtomPair, err = c.InternAtom("ATOM_PAIR", false); err != nil {
		return err
	}
	if c.AtomClipboard, err = c.InternAtom("CLIPBOARD", false); err != nil {
		return err
	}
	if c.AtomClipboardIncremental, err = c.InternAtom("INCR", false); err != nil {
		return err
	}
	if c.AtomClipboardManager, err = c.InternAtom("CLIPBOARD_MANAGER", false); err != nil {
		return err
	}
	if c.AtomClipboardMultiple, err = c.InternAtom("MULTIPLE", false); err != nil {
		return err
	}
	if c.AtomClipboardSaveTargets, err = c.InternAtom("SAVE_TARGETS", false); err != nil {
		return err
	}
	if c.AtomClipboardSelection, err = c.InternAtom("CLIPBOARD_SELECTION", false); err != nil {
		return err
	}
	if c.AtomClipboardTargets, err = c.InternAtom("TARGETS", false); err != nil {
		return err
	}
	if c.AtomNull, err = c.InternAtom("NULL", false); err != nil {
		return err
	}
	if c.AtomUTF8String, err = c.InternAtom("UTF8_STRING", false); err != nil {
		return err
	}
	if c.AtomNetWorkArea, err = c.InternAtom("_NET_WORKAREA", false); err != nil {
		return err
	}
	if c.AtomNetCurrentDesktop, err = c.InternAtom("_NET_CURRENT_DESKTOP", false); err != nil {
		return err
	}
	return nil
}

func (c *Conn) parseDisplayEnv() error {
	const invalidDisplayErr = "invalid DISPLAY environment variable: "
	c.envDisplay = os.Getenv("DISPLAY")
	colon := strings.LastIndex(c.envDisplay, ":")
	if colon < 0 {
		return errs.New(invalidDisplayErr + c.envDisplay)
	}
	if c.envDisplay[0] == '/' {
		c.socket = c.envDisplay[0:colon]
	} else {
		if slash := strings.LastIndex(c.envDisplay, "/"); slash >= 0 {
			c.protocol = c.envDisplay[0:slash]
			c.host = c.envDisplay[slash+1 : colon]
		} else {
			c.host = c.envDisplay[0:colon]
		}
	}
	id := c.envDisplay[colon+1:]
	if id == "" {
		return errs.New(invalidDisplayErr + c.envDisplay)
	}
	dot := strings.LastIndex(c.envDisplay, ".")
	if dot < 0 {
		c.display = c.envDisplay[0:]
	} else {
		c.display = c.envDisplay[0:dot]
		if c.screen = c.envDisplay[dot+1:]; c.screen != "" {
			var err error
			if c.DefaultScreen, err = strconv.Atoi(c.screen); err != nil {
				return errs.New(invalidDisplayErr + c.envDisplay)
			}
		}
	}
	var err error
	if c.displayNum, err = strconv.Atoi(id); err != nil || c.displayNum < 0 {
		return errs.New(invalidDisplayErr + c.envDisplay)
	}
	return nil
}

func (c *Conn) connect() error {
	var err error
	switch {
	case c.socket != "":
		c.conn, err = net.Dial("unix", c.socket+":"+strconv.Itoa(c.displayNum))
	case c.host != "" && c.host != "unix":
		if c.protocol == "" {
			c.protocol = "tcp"
		}
		c.conn, err = net.Dial(c.protocol, c.host+":"+strconv.Itoa(6000+c.displayNum))
	default:
		c.conn, err = net.Dial("unix", "/tmp/.X11-unix/X"+strconv.Itoa(c.displayNum))
	}
	if err != nil {
		return errs.NewWithCause("unable to connect to X server with DISPLAY "+c.envDisplay, err)
	}
	return nil
}

func (c *Conn) authenticate() error {
	host := c.host
	if host == "" || host == "localhost" {
		var err error
		if host, err = os.Hostname(); err != nil {
			return errs.NewWithCause("cannot determine hostname", err)
		}
	}
	authName, authData := c.readAuthority(host)
	w := NewWriter(18 + len(authName) + len(authData))
	w.Byte(0x6C) // Use little endian
	w.Zero(1)
	w.Uint16(11) // Major version
	w.Uint16(0)  // Minor version
	w.Uint16(uint16(len(authName)))
	w.Uint16(uint16(len(authData)))
	w.Zero(2)
	w.String(authName)
	w.ZeroTo4ByteAlignment()
	w.Bytes(authData)
	w.ZeroTo4ByteAlignment()
	if err := w.Send(c.conn); err != nil {
		return errs.NewWithCause("failed to send authentication data", err)
	}
	header := NewReader(make([]byte, 8))
	if err := header.Load(c.conn); err != nil {
		return errs.NewWithCause("failed to read authentication response header", err)
	}
	code := header.Byte()
	reasonLen := header.Byte()
	c.protocolMajorVersion = header.Uint16()
	c.protocolMinorVersion = header.Uint16()
	dataLen := header.Uint16() * 4
	if c.protocolMajorVersion != 11 || c.protocolMinorVersion != 0 {
		return errs.Newf("unsupported X protocol version: %d.%d", c.protocolMajorVersion, c.protocolMinorVersion)
	}
	r := NewReader(make([]byte, int(dataLen)))
	if err := r.Load(c.conn); err != nil {
		return errs.NewWithCause("failed to read authentication response data", err)
	}
	switch code {
	case 0:
		return errs.New("authentication refused: " + r.String(int(reasonLen)))
	case 1:
		c.releaseNumber = r.Uint32()
		c.xid.init(r)
		c.motionBufferSize = r.Uint32()
		vendorLen := r.Uint16()
		c.maximumRequestLength = r.Uint16()
		rootsLen := r.Byte()
		pixmapFormatsLen := r.Byte()
		c.imageByteOrder = r.Byte()
		c.bitmapFormatBitOrder = r.Byte()
		c.bitmapFormatScanlineUnit = r.Byte()
		c.bitmapFormatScanlinePad = r.Byte()
		c.minKeyCode = r.Byte()
		c.maxKeyCode = r.Byte()
		r.Skip(4)
		c.vendor = r.String(int(vendorLen))
		r.SkipTo4ByteAlignment()
		c.pixmapFormats = ReadList(int(pixmapFormatsLen), r, NewFormat)
		c.Roots = ReadList(int(rootsLen), r, NewScreen)
		return nil
	case 2:
		return errs.New("further authentication required: " + r.ZeroedString(int(dataLen)))
	default:
		return errs.Newf("unexpected response code: %d", code)
	}
}

func (c *Conn) readAuthority(host string) (name string, data []byte) {
	fileName := os.Getenv("XAUTHORITY")
	if fileName == "" {
		if fileName = os.Getenv("HOME"); fileName == "" {
			return "", nil
		}
		fileName += "/.Xauthority"
	}
	root, err := os.OpenRoot(filepath.Dir(fileName))
	if err != nil {
		return "", nil
	}
	defer xio.CloseIgnoringErrors(root)
	var fileData []byte
	if fileData, err = root.ReadFile(filepath.Base(fileName)); err != nil {
		return "", nil
	}
	r := NewReaderWithByteOrder(binary.BigEndian, fileData)
	for r.Remaining() != 0 {
		family := r.Uint16()
		addr := r.SizePrefixedString()
		disp := r.SizePrefixedString()
		name = r.SizePrefixedString()
		data = r.SizePrefixedBytes()
		if ((family == 65535) || (family == 256 && addr == host)) &&
			((disp == "") || (disp == c.display)) {
			return name, data
		}
	}
	return "", nil
}

// NewAtom generates a new Atom ID.
func (c *Conn) NewAtom() (Atom, error) {
	id, err := c.xid.next(c)
	if err != nil {
		return AtomNone, err
	}
	return Atom(id), nil
}

func (c *Conn) nextWindowID() (WindowID, error) {
	id, err := c.xid.next(c)
	if err != nil {
		return WindowNone, err
	}
	return WindowID(id), nil
}

func (c *Conn) nextSeq() uint16 {
	seq := uint16(c.sequence.Add(1) & 0xFFFF)
	if seq == 0 {
		return c.nextSeq()
	}
	return seq
}

func newUncheckedRequest(data *Writer) *request {
	return &request{
		sentChan: make(chan struct{}),
		data:     data,
	}
}

func newCheckedRequest(data *Writer) *request {
	return &request{
		sentChan:    make(chan struct{}),
		failureChan: make(chan error, 1),
		data:        data,
	}
}

func newReplyRequest(data *Writer, replyProcessor func(*Reader)) *request {
	return &request{
		sentChan:       make(chan struct{}),
		failureChan:    make(chan error, 1),
		replyChan:      make(chan *Reader, 1),
		replyProcessor: replyProcessor,
		data:           data,
	}
}

func newEventRequest(data *Writer, event WritableEvent) *request {
	return &request{
		sentChan: make(chan struct{}),
		data:     data,
		event:    event,
	}
}

func (c *Conn) sendNewRequest(req *request) error {
	select {
	case c.requests <- req:
		select {
		case <-req.sentChan:
			switch {
			case req.replyChan != nil:
				select {
				case in := <-req.replyChan:
					if in != nil && req.replyProcessor != nil {
						req.replyProcessor(in)
					}
					return nil
				case err := <-req.failureChan:
					return err
				case <-c.readClosed:
					return io.EOF
				}
			case req.failureChan != nil:
				select {
				case err := <-req.failureChan:
					return err
				default:
					c.Sync()
					select {
					case err := <-req.failureChan:
						return err
					case <-c.readClosed:
						return io.EOF
					default:
						c.locateRequest(req.sequence)
						return nil
					}
				}
			default:
				c.locateRequest(req.sequence)
				return nil
			}
		case <-c.closed:
			return io.EOF
		}
	case <-c.closed:
		return io.EOF
	}
}

func (c *Conn) sendRequests() {
	defer xio.CloseIgnoringErrors(c.conn)
	defer close(c.closed)
	for {
		select {
		case req := <-c.requests:
			if req == nil {
				return
			}
			req.sequence = c.nextSeq()
			if req.event != nil {
				req.event.Write(req.sequence, req.data)
			}
			if req.replyChan != nil || req.failureChan != nil {
				c.requestMapLock.Lock()
				c.requestMap[req.sequence] = req
				c.requestMapLock.Unlock()
			}
			close(req.sentChan)
			if err := req.data.Send(c.conn); err != nil {
				errs.Log(err)
				xio.CloseIgnoringErrors(c.conn)
				<-c.readClosed
				return
			}
		case <-c.readClosed:
			return
		}
	}
}

func (c *Conn) sendEvent(window WindowID, propagate bool, eventMask uint32, event WritableEvent) error {
	w := NewWriter(44)
	w.Byte(opcodeSendEvent)
	w.Bool(propagate)
	w.Uint16(11)
	w.WindowID(window)
	w.Uint32(eventMask)
	return c.sendNewRequest(newEventRequest(w, event))
}

// Sync causes all outstanding requests to be processed before returning.
func (c *Conn) Sync() {
	c.GetInputFocus() //nolint:errcheck // Don't care about errors here
}

func (c *Conn) readResponses() {
	defer close(c.events)
	defer xio.CloseIgnoringErrors(c.conn)
	defer close(c.readClosed)
	for {
		var err error
		r := NewReader(make([]byte, 32))
		if err = r.Load(c.conn); err != nil {
			c.bail(err)
			return
		}
		code := r.Byte()
		r.Skip(1)
		seq := r.Uint16()
		size := r.Uint32()
		r.Seek(0)
		switch code {
		case 0: // Error
			xerr := NewError(c, r)
			err = xerr
			seq = xerr.Sequence
			c.processRequest(seq, r, err)
		case 1: // Reply
			if size > 0 {
				if err = r.Append(int(size)*4, c.conn); err != nil {
					c.bail(err)
					return
				}
			}
			c.processRequest(seq, r, nil)
		default: // Event
			eventID := code & 127
			c.eventNewMapLock.RLock()
			f, ok := c.eventNewMap[eventID]
			c.eventNewMapLock.RUnlock()
			if ok {
				c.events <- f(r)
			} else {
				slog.Warn("dropped unhandled X11 event", "id", eventID, "sequence", seq)
			}
		}
	}
}

func (c *Conn) processRequest(seq uint16, in *Reader, err error) {
	if req := c.locateRequest(seq); req != nil {
		switch {
		case err != nil:
			if req.failureChan != nil {
				req.failureChan <- err
			} else {
				c.events <- &ErrorEvent{Error: err}
				if req.replyChan != nil {
					req.replyChan <- nil
				}
			}
		case req.replyChan != nil:
			req.replyChan <- in
		case req.failureChan != nil:
			req.failureChan <- nil
		}
	} else {
		slog.Warn("received response for unknown request", "sequence", seq, "error", err)
	}
}

func (c *Conn) locateRequest(seq uint16) *request {
	c.requestMapLock.RLock()
	defer c.requestMapLock.RUnlock()
	req, ok := c.requestMap[seq]
	if !ok {
		return nil
	}
	delete(c.requestMap, seq)
	return req
}

func (c *Conn) bail(err error) {
	select {
	case <-c.closed:
	default:
		errs.Log(err)
		c.events <- &ErrorEvent{Error: err}
	}
}

// PostEmptyEvent posts an empty event to the event channel to wake up the event loop without processing an actual X11
// event.
func (c *Conn) PostEmptyEvent() {
	c.events <- nil
}

// WaitEvents blocks until the next event is available, then processes it.
func (c *Conn) WaitEvents() {
	c.processEvent(<-c.events)
}

func waitForEvent[T Event](c *Conn, f func(Event) T) T {
	for {
		ev := <-c.events
		if xreflect.IsNil(ev) {
			var zero T
			return zero
		}
		if evt := f(ev); !xreflect.IsNil(evt) {
			return evt
		}
		c.processEvent(ev)
	}
}

// PollEvents processes the next event if one is available.
func (c *Conn) PollEvents() {
	select {
	case ev := <-c.events:
		c.processEvent(ev)
	default:
	}
}

func pollForEvent[T Event](c *Conn, f func(Event) T) T {
	for {
		select {
		case ev := <-c.events:
			if xreflect.IsNil(ev) {
				var zero T
				return zero
			}
			if evt := f(ev); !xreflect.IsNil(evt) {
				return evt
			}
			c.processEvent(ev)
		default:
			var zero T
			return zero
		}
	}
}

func (c *Conn) processEvent(ev Event) {
	if xreflect.IsNil(ev) {
		return
	}
	ev.Process(c)
}

func (c *Conn) hasExtension(name string, majorMax, minorMax uint32) extensionInfo {
	size := 8 + pad4(len(name))
	w := NewWriter(size)
	w.Byte(opcodeQueryExtension)
	w.Zero(1)
	w.Uint16(uint16(size / 4))
	w.Uint16(uint16(len(name)))
	w.Zero(2)
	w.String(name)
	w.ZeroTo4ByteAlignment()
	var info extensionInfo
	if err := c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(8)
		info.Present = r.Bool()
		info.majorOpcode = r.Byte()
		info.firstEvent = r.Byte()
		info.firstError = r.Byte()
		r.Skip(24)
	})); err != nil {
		errs.Log(err, "name", name)
	}
	if info.Present {
		w = NewWriter(12)
		w.Byte(info.majorOpcode)
		w.Byte(0) // Version query is always opcode 0 within the extension
		w.Uint16(2)
		w.Uint32(majorMax)
		w.Uint32(minorMax)
		if err := c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
			r.Skip(8)
			info.MajorVersion = uint32(r.Uint16())
			info.MinorVersion = uint32(r.Uint16())
			r.Skip(20)
		})); err != nil {
			errs.Log(err, "name", name)
		}
	}
	return info
}

// InternAtom returns the Atom ID for the specified name, creating a new Atom if onlyIfExists is false and no existing
// Atom has the specified name.
func (c *Conn) InternAtom(name string, onlyIfExists bool) (Atom, error) {
	size := 8 + pad4(len(name))
	w := NewWriter(size)
	w.Byte(opcodeInternAtom)
	w.Bool(onlyIfExists)
	w.Uint16(uint16(size / 4))
	w.Uint16(uint16(len(name)))
	w.Zero(2)
	w.String(name)
	w.ZeroTo4ByteAlignment()
	var atom Atom
	err := c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(8)
		atom = Atom(r.Uint32())
		r.Skip(20)
	}))
	return atom, err
}

// CreateWindow creates a new window with the specified parameters and attributes.
func (c *Conn) CreateWindow(parent WindowID, x, y int16, width, height, borderWidth, windowClass uint16, depth byte, visual VisualID, valueMask uint32, attributes *WindowAttributes) (WindowID, error) {
	windowID, err := c.nextWindowID()
	if err != nil {
		return WindowNone, err
	}
	valueList := attributes.toValues(valueMask)
	size := 32 + 4*len(valueList)
	w := NewWriter(size)
	w.Byte(opcodeCreateWindow)
	w.Byte(depth)
	w.Uint16(uint16(size / 4))
	w.WindowID(windowID)
	w.WindowID(parent)
	w.Int16(x)
	w.Int16(y)
	w.Uint16(width)
	w.Uint16(height)
	w.Uint16(borderWidth)
	w.Uint16(windowClass)
	w.VisualID(visual)
	w.Uint32(valueMask)
	for _, v := range valueList {
		w.Uint32(v)
	}
	w.ZeroTo4ByteAlignment()
	err = c.sendNewRequest(newCheckedRequest(w))
	return windowID, err
}

// DestroyWindow destroys the specified window.
func (c *Conn) DestroyWindow(window WindowID) error {
	w := NewWriter(8)
	w.Byte(opcodeDestroyWindow)
	w.Zero(1)
	w.Uint16(2)
	w.WindowID(window)
	return c.sendNewRequest(newCheckedRequest(w))
}

// GetInputFocus returns the current input focus window and the revert-to value.
func (c *Conn) GetInputFocus() (focus WindowID, revertTo byte, err error) {
	w := NewWriter(4)
	w.Byte(opcodeGetInputFocus)
	w.Zero(1)
	w.Uint16(1)
	err = c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(1)
		revertTo = r.Byte()
		r.Skip(6)
		focus = WindowID(r.Uint32())
	}))
	return focus, revertTo, err
}

// GetProperty returns information about the specified property.
func (c *Conn) GetProperty(window WindowID, property, propertyType Atom, offset, length uint32, remove bool) (format byte, actualPropertyType Atom, value []byte, err error) {
	w := NewWriter(24)
	w.Byte(opcodeGetProperty)
	w.Bool(remove)
	w.Uint16(6)
	w.WindowID(window)
	w.Atom(property)
	w.Atom(propertyType)
	w.Uint32(offset)
	w.Uint32(length)
	err = c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(1)
		format = r.Byte()
		r.Skip(6)
		actualPropertyType = Atom(r.Uint32())
		r.Skip(4)
		lengthInFormatUnits := r.Uint32()
		r.Skip(12)
		if format != 0 {
			size := int(lengthInFormatUnits)
			if actualPropertyType == propertyType {
				size *= int(format / 8)
			}
			value = r.Bytes(size)
			r.SkipTo4ByteAlignment()
		}
	}))
	return format, actualPropertyType, value, err
}

// Possible modes for ChangeProperty requests
const (
	PropModeReplace = iota
	PropModePrepend
	PropModeAppend
)

// ChangeProperty changes the specified property on the given window to the provided data, using the specified mode
// (PropModeReplace, PropModePrepend, or PropModeAppend). The propertyType and format parameters specify the type and
// format of the property data, respectively. The data is provided as a byte slice, and its length should be consistent
// with the specified format (8, 16, or 32 bits per unit).
func (c *Conn) ChangeProperty(window WindowID, property, propertyType Atom, format, mode byte, data []byte) {
	w := NewWriter(24 + pad4(len(data)))
	w.Byte(opcodeChangeProperty)
	w.Byte(mode)
	w.Uint16(uint16((24 + pad4(len(data))) / 4))
	w.WindowID(window)
	w.Atom(property)
	w.Atom(propertyType)
	w.Byte(format)
	w.Zero(3)
	w.Uint32(uint32(len(data) / int(format/8)))
	w.Bytes(data)
	w.ZeroTo4ByteAlignment()
	if err := c.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// Bell causes the server to emit a bell sound with the specified volume as a percentage relative to the base volume,
// from -100 to 100, inclusive.
func (c *Conn) Bell(percent int8) {
	w := NewWriter(4)
	w.Byte(opcodeBell)
	w.Int8(percent)
	w.Uint16(1)
	if err := c.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// GetClipboardText retrieves the current clipboard text by checking the owner of the CLIPBOARD selection and requesting
// the selection contents if the owner is not the helper window. It tries to retrieve the clipboard text in UTF8_STRING
// format first, then falls back to STRING format if UTF8_STRING is not available. If the clipboard contents are
// provided incrementally (using the INCR mechanism), it handles that as well by repeatedly requesting the property
// until all data has been received.
func (c *Conn) GetClipboardText() string {
	if c.helperWindow == WindowNone {
		return ""
	}
	owner, err := c.getSelectionOwner(c.AtomClipboard)
	if err != nil {
		errs.Log(err)
		return ""
	}
	if owner == c.helperWindow {
		return c.clipboard
	}
	c.clipboard = ""
	for _, kind := range []Atom{c.AtomUTF8String, AtomString} {
		c.convertSelection(c.helperWindow, c.AtomClipboard, kind, c.AtomClipboardSelection, 0)
		sne := waitForEvent(c, func(evt Event) *SelectionNotifyEvent {
			if e, ok := evt.(*SelectionNotifyEvent); ok && e.Requestor == c.helperWindow {
				return e
			}
			return nil
		})
		if sne != nil && sne.Property != AtomNone {
			filter := func(evt Event) *PropertyNotifyEvent {
				if e, ok := evt.(*PropertyNotifyEvent); ok && e.State == propertyNewValue &&
					e.Window == sne.Requestor && e.Atom == sne.Property {
					return e
				}
				return nil
			}
			pollForEvent(c, filter) // Ensure no existing PropertyNotifyEvent is already pending
			var propertyType Atom
			var value []byte
			if _, propertyType, value, err = c.GetProperty(sne.Requestor, sne.Property, AtomAny, 0, math.MaxUint32,
				true); err != nil {
				errs.Log(err)
				continue
			}
			switch propertyType {
			case c.AtomClipboardIncremental:
				var buffer bytes.Buffer
				for {
					waitForEvent(c, filter)
					if _, _, value, err = c.GetProperty(sne.Requestor, sne.Property, AtomAny, 0,
						math.MaxUint32, true); err != nil {
						errs.Log(err)
						break
					}
					if len(value) == 0 {
						break
					}
					buffer.Write(value)
				}
				if kind == c.AtomUTF8String {
					c.clipboard = buffer.String()
				} else {
					c.clipboard = convertLatin1ToUTF8(buffer.Bytes())
				}
			case c.AtomUTF8String:
				c.clipboard = string(value)
			case AtomString:
				c.clipboard = convertLatin1ToUTF8(value)
			}
		}
		if c.clipboard != "" {
			break
		}
	}
	return c.clipboard
}

func convertLatin1ToUTF8(latin1 []byte) string {
	s, err := charmap.ISO8859_1.NewDecoder().Bytes(latin1)
	if err != nil {
		errs.Log(err)
		return string(latin1)
	}
	return string(s)
}

// SetClipboardText sets the clipboard text by storing it in the connection and claiming ownership of the CLIPBOARD
// selection with a helper window. When another client requests the clipboard contents, the helper window will provide
// the stored text.
func (c *Conn) SetClipboardText(str string) {
	if c.helperWindow == WindowNone {
		return
	}
	c.clipboard = str
	c.setSelectionOwner(c.helperWindow, c.AtomClipboard)
}

func (c *Conn) setSelectionOwner(owner WindowID, selection Atom) {
	w := NewWriter(16)
	w.Byte(opcodeSetSelectionOwner)
	w.Zero(1)
	w.Uint16(4)
	w.WindowID(owner)
	w.Atom(selection)
	w.Uint32(0)
	if err := c.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

func (c *Conn) getSelectionOwner(selection Atom) (owner WindowID, err error) {
	w := NewWriter(8)
	w.Byte(opcodeGetSelectionOwner)
	w.Zero(1)
	w.Uint16(2)
	w.Atom(selection)
	err = c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(8)
		owner = WindowID(r.Uint32())
		r.Skip(20)
	}))
	return owner, err
}

func (c *Conn) convertSelection(requestor WindowID, selection, target, property Atom, timestamp uint32) {
	w := NewWriter(8)
	w.Byte(opcodeConvertSelection)
	w.Zero(1)
	w.Uint16(6)
	w.WindowID(requestor)
	w.Atom(selection)
	w.Atom(target)
	w.Atom(property)
	w.Uint32(timestamp)
	if err := c.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

func (c *Conn) setEventNewFunc(eventID byte, f func(r *Reader) Event) {
	c.eventNewMapLock.Lock()
	c.eventNewMap[eventID] = f
	c.eventNewMapLock.Unlock()
}

func (c *Conn) setErrorCodeName(code byte, name string) {
	c.errorCodeLock.Lock()
	c.errorCodeMap[code] = name
	c.errorCodeLock.Unlock()
}

// RootWindow returns the ID of the root window for the default screen.
func (c *Conn) RootWindow() WindowID {
	return c.Roots[c.DefaultScreen].Root
}

// DefaultVisual returns the ID of the default visual for the default screen.
func (c *Conn) DefaultVisual() VisualID {
	return c.Roots[c.DefaultScreen].RootVisual
}

// ContentScale returns the content scale factor for the default screen.
func (c *Conn) ContentScale() (float32, error) {
	format, actualPropertyType, value, err := c.GetProperty(c.RootWindow(), AtomResourceManager, AtomString, 0, 100_000_000, false)
	if err != nil {
		return 1, err
	}
	if format == 8 && actualPropertyType == AtomString {
		for _, line := range strings.Split(string(value), "\n") {
			const xftDPI = "Xft.dpi:"
			if strings.HasPrefix(line, xftDPI) {
				var dpi int
				if dpi, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, xftDPI))); err == nil {
					return float32(dpi) / 96, nil
				}
			}
		}
	}
	return 1, nil
}

// MonitorWorkArea returns the work area of the monitor containing the specified root window.
func (c *Conn) MonitorWorkArea(root WindowID, area geom.Rect) geom.Rect {
	_, _, extentsBytes, err := c.GetProperty(root, c.AtomNetWorkArea, AtomCardinal, 0, math.MaxUint32, false)
	if err != nil {
		return area
	}
	r := NewReader(extentsBytes)
	extents := r.Uint32Slice(len(extentsBytes) / 4)
	var desktopBytes []byte
	if _, _, desktopBytes, err = c.GetProperty(root, c.AtomNetCurrentDesktop, AtomCardinal, 0, math.MaxUint32, false); err != nil {
		return area
	}
	r = NewReader(desktopBytes)
	desktop := r.Uint32Slice(len(desktopBytes) / 4)
	if len(extents) >= 4 && len(desktop) != 0 && desktop[0] < uint32(len(extents)/4) {
		x := float32(extents[desktop[0]*4])
		y := float32(extents[desktop[0]*4+1])
		width := float32(extents[desktop[0]*4+2])
		height := float32(extents[desktop[0]*4+3])
		if area.X < x {
			area.Width -= x - area.X
			area.X = x
		}
		if area.Y < y {
			area.Height -= y - area.Y
			area.Y = y
		}
		if area.Right() > x+width {
			area.Width = x - area.X + width
		}
		if area.Bottom() > y+height {
			area.Height = y - area.Y + height
		}
	}
	return area
}

// pushClipboardToManager checks if the helper window is currently the owner of the CLIPBOARD selection, and if so, it
// converts the selection to the CLIPBOARD_MANAGER with the SAVE_TARGETS property. It then waits for events related to
// this conversion, processing any SelectionRequestEvent or SelectionClearEvent that may occur during this time.
// Finally, it destroys the helper window and resets its ID to WindowNone.
func (c *Conn) pushClipboardToManager() {
	if c.helperWindow == WindowNone {
		return
	}
	if owner, err := c.getSelectionOwner(c.AtomClipboard); err == nil && owner == c.helperWindow {
		c.convertSelection(c.helperWindow, c.AtomClipboardManager, c.AtomClipboardSaveTargets, AtomNone, 0)
		again := true
		for again {
			evt := waitForEvent(c, func(ev Event) Event {
				switch e := ev.(type) {
				case *SelectionNotifyEvent:
					if e.Requestor == c.helperWindow {
						return e
					}
				case *SelectionRequestEvent:
					if e.Owner == c.helperWindow {
						return e
					}
				case *SelectionClearEvent:
					if e.Owner == c.helperWindow {
						return e
					}
				}
				return nil
			})
			switch e := evt.(type) {
			case *SelectionNotifyEvent:
				if e.Target == c.AtomClipboardSaveTargets {
					again = false
				}
			case *SelectionRequestEvent:
				c.processEvent(e)
			case *SelectionClearEvent:
			default:
				again = false
			}
		}
	}
	if err := c.DestroyWindow(c.helperWindow); err != nil {
		errs.Log(err)
	}
	c.helperWindow = WindowNone
}

// Close the connection after finishing any in-flight requests.
func (c *Conn) Close() {
	c.pushClipboardToManager()
	c.Sync()
	close(c.requests)
	<-c.closed
}
