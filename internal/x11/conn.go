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
	seq     chan struct{}
	request *Request
	data    *Writer
}

type extensionInfo struct {
	present     bool
	majorOpcode byte
	firstEvent  byte
	firstError  byte
}

// Conn represents a connection to an X server.
type Conn struct {
	conn                     net.Conn
	eventChan                chan Event
	requestChan              chan *Request
	reqChan                  chan *request
	termSend                 chan struct{}
	termRead                 chan struct{}
	ExtMisc                  *ExtMisc
	extensions               map[string]extensionInfo
	eventNewMap              map[byte]func(r *Reader) Event
	errorCodeMap             map[byte]string
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
	extensionsLock           sync.RWMutex
	eventNewMapLock          sync.RWMutex
	errorCodeLock            sync.RWMutex
	resourceIDLock           sync.Mutex
	DefaultScreen            int
	displayNum               int
	sequence                 atomic.Uint32
	releaseNumber            uint32
	resourceIDBase           uint32
	resourceIDMask           uint32
	resourceIDMax            uint32
	resourceIDLast           uint32
	motionBufferSize         uint32
	helperWindow             WindowID
	clipboardAtom            Atom
	clipboardSelectionAtom   Atom
	clipboardIncrementalAtom Atom
	clipboardTargetsAtom     Atom
	clipboardMultipleAtom    Atom
	clipboardManagerAtom     Atom
	clipboardSaveTargetsAtom Atom
	utf8StringAtom           Atom
	atomPairAtom             Atom
	nullAtom                 Atom
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
	c.requestChan = make(chan *Request, 1024)
	c.reqChan = make(chan *request, 128)
	c.eventChan = make(chan Event, 8192)
	c.termSend = make(chan struct{})
	c.termRead = make(chan struct{})
	c.ExtMisc = &ExtMisc{conn: &c}
	go c.sendRequests()
	go c.readResponses()
	if c.clipboardAtom, err = c.InternAtom("CLIPBOARD", false); err != nil {
		return nil, err
	}
	if c.clipboardSelectionAtom, err = c.InternAtom("CLIPBOARD_SELECTION", false); err != nil {
		return nil, err
	}
	if c.clipboardIncrementalAtom, err = c.InternAtom("INCR", false); err != nil {
		return nil, err
	}
	if c.clipboardTargetsAtom, err = c.InternAtom("TARGETS", false); err != nil {
		return nil, err
	}
	if c.clipboardMultipleAtom, err = c.InternAtom("MULTIPLE", false); err != nil {
		return nil, err
	}
	if c.clipboardManagerAtom, err = c.InternAtom("CLIPBOARD_MANAGER", false); err != nil {
		return nil, err
	}
	if c.clipboardSaveTargetsAtom, err = c.InternAtom("SAVE_TARGETS", false); err != nil {
		return nil, err
	}
	if c.utf8StringAtom, err = c.InternAtom("UTF8_STRING", false); err != nil {
		return nil, err
	}
	if c.atomPairAtom, err = c.InternAtom("ATOM_PAIR", false); err != nil {
		return nil, err
	}
	if c.nullAtom, err = c.InternAtom("NULL", false); err != nil {
		return nil, err
	}
	if c.helperWindow, err = c.CreateWindow(c.RootWindow(), 0, 0, 1, 1, 0, WindowClassInputOnly, 0, c.DefaultVisual(),
		WindowBitMaskEventMask, &WindowAttributes{EventMask: EventMaskPropertyChange}); err != nil {
		return nil, err
	}
	return &c, nil
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
		c.resourceIDBase = r.Uint32()
		c.resourceIDMask = r.Uint32()
		c.resourceIDMax = c.resourceIDMask
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
	id, err := c.nextID()
	if err != nil {
		return AtomNone, err
	}
	return Atom(id), nil
}

func (c *Conn) nextWindowID() (WindowID, error) {
	id, err := c.nextID()
	if err != nil {
		return WindowNone, err
	}
	return WindowID(id), nil
}

func (c *Conn) nextID() (uint32, error) {
	inc := c.resourceIDMask & -c.resourceIDMask
	c.resourceIDLock.Lock()
	defer c.resourceIDLock.Unlock()
	switch {
	case c.resourceIDLast < c.resourceIDMax-inc+1:
		c.resourceIDLast += inc
	case c.ExtMisc.Available():
		startID, count, err := c.ExtMisc.GetXIDRange()
		if err != nil {
			return 0, err
		}
		c.resourceIDLast = startID
		c.resourceIDMax = startID + (count-1)*inc
	default:
		return 0, errs.New("no more IDs available")
	}
	return c.resourceIDLast | c.resourceIDBase, nil
}

func (c *Conn) nextSeq() uint16 {
	seq := uint16(c.sequence.Add(1) & 0xFFFF)
	if seq == 0 {
		return c.nextSeq()
	}
	return seq
}

func (c *Conn) sendNewRequest(data *Writer, req *Request) {
	seq := make(chan struct{})
	select {
	case c.reqChan <- &request{seq: seq, request: req, data: data}:
		select {
		case <-seq:
		case <-c.termSend:
		}
	case <-c.termSend:
	}
}

func (c *Conn) sendRequests() {
	defer close(c.requestChan)
	defer xio.CloseIgnoringErrors(c.conn)
	defer close(c.termSend)
	for {
		select {
		case req := <-c.reqChan:
			if req == nil {
				if err := c.noop(); err != nil {
					xio.CloseIgnoringErrors(c.conn)
					<-c.termRead
				}
				return
			}
			if len(c.requestChan) == cap(c.requestChan)-1 {
				if err := c.noop(); err != nil {
					xio.CloseIgnoringErrors(c.conn)
					<-c.termRead
					return
				}
			}
			req.request.sequence = c.nextSeq()
			slog.Info("sending request", "sequence", req.request.sequence, "name", req.request.name)
			c.requestChan <- req.request
			if err := req.data.Send(c.conn); err != nil {
				xio.CloseIgnoringErrors(c.conn)
				<-c.termRead
				return
			}
			close(req.seq)
		case <-c.termRead:
			return
		}
	}
}

func (c *Conn) sendEvent(window WindowID, propagate bool, eventMask uint32, event WritableEvent) {
	req := newRequest("sendEvent", c, false, false, nil)
	w := NewWriter(44)
	w.Byte(opcodeSendEvent)
	w.Bool(propagate)
	w.Uint16(11)
	w.WindowID(window)
	w.Uint32(eventMask)
	event.Write(c.nextSeq(), w)
	c.sendNewRequest(w, req)
}

func (c *Conn) noop() error {
	slog.Info("SENDING noop request")
	req := newRequest("noop", c, true, true, nil)
	req.sequence = c.nextSeq()
	c.requestChan <- req
	slog.Info("sending noop request", "sequence", req.sequence, "name", req.name)
	if err := c.inputFocusRequestWriter().Send(c.conn); err != nil {
		return err
	}
	req.Reply() //nolint:errcheck // Don't care about errors here
	return nil
}

// Sync causes all outstanding requests to be processed before returning.
func (c *Conn) Sync() {
	slog.Info("SYNC")
	c.GetInputFocus() //nolint:errcheck // Don't care about errors here
}

func (c *Conn) readResponses() {
	defer close(c.eventChan)
	defer xio.CloseIgnoringErrors(c.conn)
	defer close(c.termRead)
	var err error
	var seq uint16
	for {
		r := NewReader(make([]byte, 32))
		if err = r.Load(c.conn); err != nil {
			c.bail(err)
			return
		}
		switch r.Byte() {
		case 0: // Error
			r.Seek(0)
			xerr := NewError(c, r)
			c.errorCodeLock.RLock()
			err = xerr
			seq = xerr.Sequence
			slog.Info("X11 error received", "sequence", seq, "err", err)
		case 1: // Reply
			r.Skip(1)
			seq = r.Uint16()
			if size := r.Uint32(); size > 0 {
				if err = r.Append(int(size)*4, c.conn); err != nil {
					c.bail(err)
					return
				}
			}
			r.Seek(0)
			slog.Info("X11 reply received", "sequence", seq)
		default: // Event
			r.Seek(0)
			eventID := r.Byte() & 127
			r.Skip(1)
			seq2 := r.Uint16()
			r.Seek(0)
			c.eventNewMapLock.RLock()
			f, ok := c.eventNewMap[eventID]
			c.eventNewMapLock.RUnlock()
			slog.Info("X11 event received", "id", eventID, "sequence", seq2)
			if ok {
				c.eventChan <- f(r)
			} else {
				slog.Warn("dropped unhandled X11 event", "id", eventID)
			}
			continue
		}
		slog.Info("processing the X11 request channel", "sequence", seq)
		for one := range c.requestChan {
			slog.Info("checking request against response", "requestSequence", one.sequence, "responseSequence", seq, "requestName", one.name)
			if one.processRequest(seq, r, err) {
				break
			}
		}
	}
}

func (c *Conn) bail(err error) {
	select {
	case <-c.termSend:
	default:
		errs.Log(err)
		c.eventChan <- &ErrorEvent{Error: err}
	}
}

// PostEmptyEvent posts an empty event to the event channel to wake up the event loop without processing an actual X11
// event.
func (c *Conn) PostEmptyEvent() {
	c.eventChan <- nil
}

// WaitEvents blocks until the next event is available, then processes it.
func (c *Conn) WaitEvents() {
	c.processEvent(<-c.eventChan)
}

func waitForEvent[T Event](c *Conn, f func(Event) T) T {
	for {
		ev := <-c.eventChan
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
	case ev := <-c.eventChan:
		c.processEvent(ev)
	default:
	}
}

func pollForEvent[T Event](c *Conn, f func(Event) T) T {
	for {
		select {
		case ev := <-c.eventChan:
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

func (c *Conn) hasExtension(name string) extensionInfo {
	c.extensionsLock.RLock()
	info, ok := c.extensions[name]
	c.extensionsLock.RUnlock()
	if ok {
		return info
	}
	c.extensionsLock.Lock()
	defer c.extensionsLock.Unlock()
	info = c.queryExtension(name)
	if c.extensions == nil {
		c.extensions = make(map[string]extensionInfo)
	}
	c.extensions[name] = info
	return info
}

func (c *Conn) queryExtension(name string) extensionInfo {
	var info extensionInfo
	req := newRequest("queryExtension", c, true, true, func(r *Reader) {
		r.Skip(8)
		info.present = r.Bool()
		info.majorOpcode = r.Byte()
		info.firstEvent = r.Byte()
		info.firstError = r.Byte()
	})
	size := 8 + pad4(len(name))
	w := NewWriter(size)
	w.Byte(opcodeQueryExtension)
	w.Zero(1)
	w.Uint16(uint16(size / 4))
	w.Uint16(uint16(len(name)))
	w.Zero(2)
	w.String(name)
	w.ZeroTo4ByteAlignment()
	c.sendNewRequest(w, req)
	req.Reply() //nolint:errcheck // Ignore errors here since we'll just return info.present=false
	return info
}

// InternAtom returns the Atom ID for the specified name, creating a new Atom if onlyIfExists is false and no existing
// Atom has the specified name.
func (c *Conn) InternAtom(name string, onlyIfExists bool) (Atom, error) {
	var atom Atom
	req := newRequest("internAtom", c, true, true, func(r *Reader) {
		r.Skip(8)
		atom = Atom(r.Uint32())
		r.Skip(20)
	})
	size := 8 + pad4(len(name))
	w := NewWriter(size)
	w.Byte(opcodeInternAtom)
	w.Bool(onlyIfExists)
	w.Uint16(uint16(size / 4))
	w.Uint16(uint16(len(name)))
	w.Zero(2)
	w.String(name)
	w.ZeroTo4ByteAlignment()
	c.sendNewRequest(w, req)
	err := req.Reply()
	return atom, err
}

// CreateWindow creates a new window with the specified parameters and attributes, returning a CreateNotifyEvent
// containing the ID of the newly created window if successful.
func (c *Conn) CreateWindow(parent WindowID, x, y int16, width, height, borderWidth, windowClass uint16, depth byte, visual VisualID, valueMask uint32, attributes *WindowAttributes) (WindowID, error) {
	windowID, err := c.nextWindowID()
	if err != nil {
		return WindowNone, err
	}
	req := newRequest("createWindow", c, true, false, nil)
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
	c.sendNewRequest(w, req)
	return windowID, req.Check()
}

// DestroyWindow destroys the specified window.
func (c *Conn) DestroyWindow(window WindowID) error {
	req := newRequest("destroyWindow", c, true, false, nil)
	w := NewWriter(8)
	w.Byte(opcodeDestroyWindow)
	w.Zero(1)
	w.Uint16(2)
	w.WindowID(window)
	c.sendNewRequest(w, req)
	return req.Check()
}

// GetInputFocus returns the current input focus window and the revert-to value.
func (c *Conn) GetInputFocus() (focus WindowID, revertTo byte, err error) {
	req := newRequest("getInputFocus", c, true, true, func(r *Reader) {
		r.Skip(1)
		revertTo = r.Byte()
		r.Skip(6)
		focus = WindowID(r.Uint32())
	})
	c.sendNewRequest(c.inputFocusRequestWriter(), req)
	err = req.Reply()
	return focus, revertTo, err
}

func (c *Conn) inputFocusRequestWriter() *Writer {
	w := NewWriter(4)
	w.Byte(opcodeGetInputFocus)
	w.Zero(1)
	w.Uint16(1)
	return w
}

// GetProperty returns information about the specified property.
func (c *Conn) GetProperty(window WindowID, property, propertyType Atom, offset, length uint32, remove bool) (format byte, actualPropertyType Atom, value []byte, err error) {
	req := newRequest("getProperty", c, true, true, func(r *Reader) {
		r.Skip(1)
		format = r.Byte()
		r.Skip(6)
		actualPropertyType = Atom(r.Uint32())
		r.Skip(4)
		lengthInFormatUnits := r.Uint32()
		r.Skip(12)
		if format != 0 {
			value = r.Bytes(int(lengthInFormatUnits * uint32(format/8)))
			r.SkipTo4ByteAlignment()
		}
	})
	w := NewWriter(24)
	w.Byte(opcodeGetProperty)
	w.Bool(remove)
	w.Uint16(6)
	w.WindowID(window)
	w.Atom(property)
	w.Atom(propertyType)
	w.Uint32(offset)
	w.Uint32(length)
	c.sendNewRequest(w, req)
	err = req.Reply()
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
func (c *Conn) ChangeProperty(window WindowID, property, propertyType Atom, format, mode byte, data []byte) error {
	slog.Info("ChangeProperty")
	req := newRequest("changeProperty", c, true, false, nil)
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
	c.sendNewRequest(w, req)
	return req.Check()
}

// Bell causes the server to emit a bell sound with the specified volume as a percentage relative to the base volume,
// from -100 to 100, inclusive.
func (c *Conn) Bell(percent int8) {
	req := newRequest("bell", c, false, false, nil)
	w := NewWriter(4)
	w.Byte(opcodeBell)
	w.Int8(percent)
	w.Uint16(1)
	c.sendNewRequest(w, req)
}

// GetClipboardText retrieves the current clipboard text by checking the owner of the CLIPBOARD selection and requesting the selection contents if the owner is not the helper window. It tries to retrieve the clipboard text in UTF8_STRING format first, then falls back to STRING format if UTF8_STRING is not available. If the clipboard contents are provided incrementally (using the INCR mechanism), it handles that as well by repeatedly requesting the property until all data has been received. The retrieved clipboard text is stored in the connection for future retrievals until it changes.
func (c *Conn) GetClipboardText() string {
	slog.Info("GetClipboardText")
	owner, err := c.getSelectionOwner(c.clipboardAtom)
	if err != nil {
		errs.Log(err)
		return ""
	}
	if owner == c.helperWindow {
		return c.clipboard
	}
	c.clipboard = ""
	for _, kind := range []Atom{c.utf8StringAtom, AtomString} {
		c.convertSelection(c.helperWindow, c.clipboardAtom, kind, c.clipboardSelectionAtom, 0)
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
			case c.clipboardIncrementalAtom:
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
				if kind == c.utf8StringAtom {
					c.clipboard = buffer.String()
				} else {
					c.clipboard = convertLatin1ToUTF8(buffer.Bytes())
				}
			case c.utf8StringAtom:
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
	slog.Info("SetClipboardText")
	c.clipboard = str
	req := newRequest("setClipboardText", c, false, false, nil)
	w := NewWriter(16)
	w.Byte(opcodeSetSelectionOwner)
	w.Zero(1)
	w.Uint16(4)
	w.WindowID(c.helperWindow)
	w.Atom(c.clipboardAtom)
	w.Uint32(0)
	c.sendNewRequest(w, req)
}

func (c *Conn) getSelectionOwner(selection Atom) (owner WindowID, err error) {
	slog.Info("getSelectionOwner")
	req := newRequest("getSelectionOwner", c, true, true, func(r *Reader) {
		r.Skip(8)
		owner = WindowID(r.Uint32())
		r.Skip(20)
	})
	w := NewWriter(8)
	w.Byte(opcodeGetSelectionOwner)
	w.Zero(1)
	w.Uint16(2)
	w.Atom(selection)
	c.sendNewRequest(w, req)
	err = req.Reply()
	return owner, err
}

func (c *Conn) convertSelection(requestor WindowID, selection, target, property Atom, timestamp uint32) {
	slog.Info("convertSelection")
	req := newRequest("convertSelection", c, false, false, nil)
	w := NewWriter(8)
	w.Byte(opcodeConvertSelection)
	w.Zero(1)
	w.Uint16(6)
	w.WindowID(requestor)
	w.Atom(selection)
	w.Atom(target)
	w.Atom(property)
	w.Uint32(timestamp)
	c.sendNewRequest(w, req)
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

// PushClipboardToManager checks if the helper window is currently the owner of the CLIPBOARD selection, and if so, it
// converts the selection to the CLIPBOARD_MANAGER with the SAVE_TARGETS property. It then waits for events related to
// this conversion, processing any SelectionRequestEvent or SelectionClearEvent that may occur during this time.
// Finally, it destroys the helper window and resets its ID to WindowNone.
func (c *Conn) PushClipboardToManager() {
	slog.Info("PushClipboardToManager")
	if c.helperWindow != WindowNone {
		if owner, err := c.getSelectionOwner(c.clipboardAtom); err == nil && owner == c.helperWindow {
			c.convertSelection(c.helperWindow, c.clipboardManagerAtom, c.clipboardSaveTargetsAtom, AtomNone, 0)
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
					if e.Target == c.clipboardSaveTargetsAtom {
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
}

// Close the connection after finishing any in-flight requests.
func (c *Conn) Close() {
	select {
	case c.reqChan <- nil:
	case <-c.termSend:
	}
}
