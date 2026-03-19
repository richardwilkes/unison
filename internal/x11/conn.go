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
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xreflect"
)

// Event represents a generic X11 event. Specific event types will implement this interface.
type Event interface {
	// Process the event using the provided connection. The implementation should perform any necessary actions based on
	// the event type and its data.
	Process(*Conn)
}

type xid struct {
	err error
	id  uint32
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
	xidChan                  chan xid
	seqChan                  chan uint16
	reqChan                  chan *request
	doneSend                 chan struct{}
	doneRead                 chan struct{}
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
	pixmapFormats            []*Format
	Roots                    []*Screen
	extensionsLock           sync.RWMutex
	eventNewMapLock          sync.RWMutex
	errorCodeLock            sync.RWMutex
	DefaultScreen            int
	displayNum               int
	releaseNumber            uint32
	resourceIDBase           uint32
	resourceIDMask           uint32
	motionBufferSize         uint32
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
	if err := c.parseDisplayEnv(); err != nil {
		return nil, err
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	if err := c.authenticate(); err != nil {
		return nil, err
	}
	c.errorCodeMap = map[byte]string{
		1:  "request error",
		2:  "value error",
		3:  "window error",
		4:  "pixmap error",
		5:  "atom error",
		6:  "cursor error",
		7:  "font error",
		8:  "match error",
		9:  "drawable error",
		10: "access error",
		11: "alloc error",
		12: "colormap error",
		13: "gcontext error",
		14: "id choice error",
		15: "name error",
		16: "length error",
		17: "implementation error",
	}
	c.eventNewMap = map[byte]func(r *Reader) Event{
		2:  newKeyPressEvent,
		3:  newKeyReleaseEvent,
		4:  newButtonPressEvent,
		5:  newButtonReleaseEvent,
		6:  newMotionNotifyEvent,
		7:  newEnterNotifyEvent,
		8:  newLeaveNotifyEvent,
		9:  newFocusInEvent,
		10: newFocusOutEvent,
		11: newKeymapNotifyEvent,
		12: newExposeEvent,
		13: newGraphicsExposureEvent,
		14: newNoExposureEvent,
		15: newVisibilityNotifyEvent,
		16: newCreateNotifyEvent,
		17: newDestroyNotifyEvent,
		18: newUnmapNotifyEvent,
		19: newMapNotifyEvent,
		20: newMapRequestEvent,
		21: newReparentNotifyEvent,
		22: newConfigureNotifyEvent,
		23: newConfigureRequestEvent,
		24: newGravityNotifyEvent,
		25: newResizeRequestEvent,
		26: newCirculateNotifyEvent,
		27: newCirculateRequestEvent,
		28: newPropertyNotifyEvent,
		29: newSelectionClearEvent,
		30: newSelectionRequestEvent,
		31: newSelectionNotifyEvent,
		32: newColormapNotifyEvent,
		33: newClientMessageEvent,
		34: newMappingNotifyEvent,
	}
	c.requestChan = make(chan *Request, 1024)
	c.xidChan = make(chan xid, 8)
	c.seqChan = make(chan uint16, 8)
	c.reqChan = make(chan *request, 128)
	c.eventChan = make(chan Event, 8192)
	c.doneSend = make(chan struct{})
	c.doneRead = make(chan struct{})
	c.ExtMisc = &ExtMisc{conn: &c}
	go c.generateXIDs()
	go c.generateSequenceIDs()
	go c.sendRequests()
	go c.readResponses()
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
	id, err := c.newID()
	if err != nil {
		return AtomNone, err
	}
	return Atom(id), nil
}

func (c *Conn) newID() (uint32, error) {
	id, ok := <-c.xidChan
	if !ok {
		return 0, io.EOF
	}
	if id.err != nil {
		return 0, id.err
	}
	return id.id, nil
}

func (c *Conn) generateXIDs() {
	defer close(c.xidChan)
	rangeState := 0
	idInc := c.resourceIDMask & -c.resourceIDMask
	idMax := c.resourceIDMask
	var last uint32
	for {
		var id xid
		if last < idMax-idInc+1 {
			last += idInc
			id = xid{id: last | c.resourceIDBase}
		} else {
			if rangeState == 0 {
				if c.ExtMisc.Available() {
					rangeState = 1
				} else {
					rangeState = 2
				}
			}
			if rangeState == 1 {
				if startID, count, err := c.ExtMisc.GetXIDRange(); err != nil {
					id = xid{err: err}
				} else {
					last = startID
					idMax = startID + (count-1)*idInc
					id = xid{id: last | c.resourceIDBase}
				}
			} else {
				id = xid{err: errs.New("no more IDs available")}
			}
		}
		select {
		case c.xidChan <- id:
		case <-c.doneSend:
			return
		}
	}
}

func (c *Conn) newSequenceID() uint16 {
	return <-c.seqChan
}

func (c *Conn) generateSequenceIDs() {
	defer close(c.seqChan)
	seqid := uint16(1)
	for {
		select {
		case c.seqChan <- seqid:
			seqid++
			if seqid == 0 {
				seqid = 1
			}
		case <-c.doneSend:
			return
		}
	}
}

func (c *Conn) newRequest(data *Writer, req *Request) {
	seq := make(chan struct{})
	select {
	case c.reqChan <- &request{seq: seq, request: req, data: data}:
		select {
		case <-seq:
		case <-c.doneSend:
		}
	case <-c.doneSend:
	}
}

func (c *Conn) sendRequests() {
	defer close(c.requestChan)
	defer xio.CloseIgnoringErrors(c.conn)
	defer close(c.doneSend)
	for {
		select {
		case req := <-c.reqChan:
			if req == nil {
				if err := c.noop(); err != nil {
					xio.CloseIgnoringErrors(c.conn)
					<-c.doneRead
				}
				return
			}
			if len(c.requestChan) == cap(c.requestChan)-1 {
				if err := c.noop(); err != nil {
					xio.CloseIgnoringErrors(c.conn)
					<-c.doneRead
					return
				}
			}
			req.request.setSequenceID(c.newSequenceID())
			c.requestChan <- req.request
			if err := req.data.Send(c.conn); err != nil {
				xio.CloseIgnoringErrors(c.conn)
				<-c.doneRead
				return
			}
			close(req.seq)
		case <-c.doneRead:
			return
		}
	}
}

func (c *Conn) noop() error {
	req := newRequest(c, true, true, nil)
	req.setSequenceID(c.newSequenceID())
	c.requestChan <- req
	if err := c.inputFocusRequestWriter().Send(c.conn); err != nil {
		return err
	}
	req.Reply() //nolint:errcheck // Don't care about errors here
	return nil
}

// Sync causes all outstanding requests to be processed before returning.
func (c *Conn) Sync() {
	c.GetInputFocus() //nolint:errcheck // Don't care about errors here
}

func (c *Conn) readResponses() {
	defer close(c.eventChan)
	defer xio.CloseIgnoringErrors(c.conn)
	defer close(c.doneRead)
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
		default: // Event
			r.Seek(0)
			eventID := r.Byte() & 127
			r.Seek(0)
			c.eventNewMapLock.RLock()
			f, ok := c.eventNewMap[eventID]
			c.eventNewMapLock.RUnlock()
			if ok {
				c.eventChan <- f(r)
			} else {
				slog.Warn("dropped unhandled X11 event", "id", eventID)
			}
			continue
		}
		for one := range c.requestChan {
			if one.processRequest(seq, r, err) {
				break
			}
		}
	}
}

func (c *Conn) bail(err error) {
	select {
	case <-c.doneSend:
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

// PollEvents processes the next event if one is available.
func (c *Conn) PollEvents() {
	select {
	case ev := <-c.eventChan:
		c.processEvent(ev)
	default:
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
	req := newRequest(c, true, true, func(r *Reader) {
		r.Skip(8)
		info.present = r.Bool()
		info.majorOpcode = r.Byte()
		info.firstEvent = r.Byte()
		info.firstError = r.Byte()
	})
	size := 8 + pad4(len(name)) - len(name)
	w := NewWriter(size)
	w.Byte(98)
	w.Zero(1)
	w.Uint16(uint16(size / 4))
	w.Uint16(uint16(len(name)))
	w.Zero(2)
	w.String(name)
	w.ZeroTo4ByteAlignment()
	c.newRequest(w, req)
	req.Reply() //nolint:errcheck // Ignore errors here since we'll just return info.present=false
	return info
}

// GetInputFocus returns the current input focus window and the revert-to value.
func (c *Conn) GetInputFocus() (focus WindowID, revertTo byte, err error) {
	req := newRequest(c, true, true, func(r *Reader) {
		r.Skip(1)
		revertTo = r.Byte()
		r.Skip(6)
		focus = WindowID(r.Uint32())
	})
	c.newRequest(c.inputFocusRequestWriter(), req)
	err = req.Reply()
	return focus, revertTo, err
}

func (c *Conn) inputFocusRequestWriter() *Writer {
	w := NewWriter(4)
	w.Byte(43)
	w.Zero(1)
	w.Uint16(1)
	return w
}

// GetProperty returns information about the specified property.
func (c *Conn) GetProperty(window WindowID, property, propertyType Atom, offset, length uint32, remove bool) (format byte, actualPropertyType Atom, value []byte, err error) {
	req := newRequest(c, true, true, func(r *Reader) {
		r.Skip(1)
		format = r.Byte()
		r.Skip(6)
		actualPropertyType = Atom(r.Uint32())
		r.Skip(4)
		lengthInFormatUnits := r.Uint32()
		r.Skip(12)
		if format != 0 {
			value = r.Bytes(int(lengthInFormatUnits * uint32(format/8)))
			r.Skip(pad4(len(value)))
		}
	})
	w := NewWriter(24)
	w.Byte(20)
	w.Bool(remove)
	w.Uint16(6)
	w.Uint32(uint32(window))
	w.Uint32(uint32(property))
	w.Uint32(uint32(propertyType))
	w.Uint32(offset)
	w.Uint32(length)
	c.newRequest(w, req)
	err = req.Reply()
	return format, actualPropertyType, value, err
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

// Close the connection after finishing any in-flight requests.
func (c *Conn) Close() {
	select {
	case c.reqChan <- nil:
	case <-c.doneSend:
	}
}
