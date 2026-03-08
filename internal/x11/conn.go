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
)

var _ protoReader = &Conn{}

type xid struct {
	err error
	id  uint32
}

type request struct {
	seq     chan struct{}
	request requestProcessor
	data    *Writer
}

// Conn represents a connection to an X server.
type Conn struct {
	conn                     net.Conn
	eventChan                chan Event
	requestChan              chan requestProcessor
	xidChan                  chan xid
	seqChan                  chan uint16
	reqChan                  chan *request
	doneSend                 chan struct{}
	doneRead                 chan struct{}
	ExtMisc                  *ExtMisc
	extensions               map[string]*QueryExtensionReply
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
	roots                    []*Screen
	extensionsLock           sync.RWMutex
	eventNewMapLock          sync.RWMutex
	errorCodeLock            sync.RWMutex
	defaultScreen            int
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
	minKeycode               byte
	maxKeycode               byte
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
		2:  c.newKeyPressEvent,
		3:  c.newKeyReleaseEvent,
		4:  c.newButtonPressEvent,
		5:  c.newButtonReleaseEvent,
		6:  c.newMotionNotifyEvent,
		7:  c.newEnterNotifyEvent,
		8:  c.newLeaveNotifyEvent,
		9:  c.newFocusInEvent,
		10: c.newFocusOutEvent,
		11: c.newKeymapNotifyEvent,
		12: c.newExposeEvent,
		13: c.newGraphicsExposureEvent,
		14: c.newNoExposureEvent,
		15: c.newVisibilityNotifyEvent,
		16: c.newCreateNotifyEvent,
		17: c.newDestroyNotifyEvent,
		18: c.newUnmapNotifyEvent,
		19: c.newMapNotifyEvent,
		20: c.newMapRequestEvent,
		21: c.newReparentNotifyEvent,
		22: c.newConfigureNotifyEvent,
		23: c.newConfigureRequestEvent,
		24: c.newGravityNotifyEvent,
		25: c.newResizeRequestEvent,
		26: c.newCirculateNotifyEvent,
		27: c.newCirculateRequestEvent,
		28: c.newPropertyNotifyEvent,
		29: c.newSelectionClearEvent,
		30: c.newSelectionRequestEvent,
		31: c.newSelectionNotifyEvent,
		32: c.newColormapNotifyEvent,
		33: c.newClientMessageEvent,
		34: c.newMappingNotifyEvent,
		35: c.newGenericEventEvent,
	}
	c.requestChan = make(chan requestProcessor, 1024)
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
			if c.defaultScreen, err = strconv.Atoi(c.screen); err != nil {
				return errs.New(invalidDisplayErr + c.envDisplay)
			}
		}
	}
	var err error
	if c.displayNum, err = strconv.Atoi(c.display); err != nil || c.displayNum < 0 {
		return errs.New(invalidDisplayErr + c.envDisplay)
	}
	if c.host == "" || c.host == "localhost" {
		if c.host, err = os.Hostname(); err != nil {
			return errs.NewWithCause("cannot determine hostname", err)
		}
	}
	return nil
}

func (c *Conn) connect() error {
	var err error
	switch {
	case c.socket != "":
		c.conn, err = net.Dial("unix", c.socket+":"+c.display)
	case c.host != "" && c.host != "unix":
		if c.protocol == "" {
			c.protocol = "tcp"
		}
		c.conn, err = net.Dial(c.protocol, c.host+":"+strconv.Itoa(6000+c.displayNum))
	default:
		c.conn, err = net.Dial("unix", "/tmp/.X11-unix/X"+c.display)
	}
	if err != nil {
		return errs.NewWithCause("unable to connect to X server with DISPLAY "+c.envDisplay, err)
	}
	return nil
}

func (c *Conn) authenticate() error {
	authName, authData := c.readAuthority()
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
	data := NewReader(make([]byte, int(dataLen)))
	if err := data.Load(c.conn); err != nil {
		return errs.NewWithCause("failed to read authentication response data", err)
	}
	switch code {
	case 0:
		return errs.New("authentication refused: " + data.String(int(reasonLen)))
	case 1:
		c.protoRead(data)
		return nil
	case 2:
		return errs.New("further authentication required: " + data.ZeroedString(int(dataLen)))
	default:
		return errs.Newf("unexpected response code: %d", code)
	}
}

func (c *Conn) readAuthority() (name string, data []byte) {
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
		if ((family == 65535) || (family == 256 && addr == c.host)) &&
			((disp == "") || (disp == c.display)) {
			return name, data
		}
	}
	return "", nil
}

func (c *Conn) protoRead(r *Reader) {
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
	c.minKeycode = r.Byte()
	c.maxKeycode = r.Byte()
	r.Skip(4)
	c.vendor = r.String(int(vendorLen))
	r.SkipTo4ByteAlignment()
	c.pixmapFormats = ReadList[*Format](int(pixmapFormatsLen), r)
	c.roots = ReadList[*Screen](int(rootsLen), r)
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
				if idRange, err := c.ExtMisc.GetXIDRange(); err != nil {
					id = xid{err: err}
				} else {
					last = idRange.StartID
					idMax = idRange.StartID + (idRange.Count-1)*idInc
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

func (c *Conn) newRequest(data *Writer, req requestProcessor) {
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
	req := newRequest(c, true, true, &GetInputFocusReply{})
	req.setSequenceID(c.newSequenceID())
	c.requestChan <- req
	if err := getInputFocusRequest().Send(c.conn); err != nil {
		return err
	}
	req.Reply() //nolint:errcheck // Don't care about errors here
	return nil
}

// Sync causes all outstanding requests to be processed before returning.
func (c *Conn) Sync() {
	GetInputFocus(c) //nolint:errcheck // Don't care about errors here
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
			code := r.Byte()
			r.Seek(0)
			var xerr Error
			xerr.protoRead(r)
			c.errorCodeLock.RLock()
			name, ok := c.errorCodeMap[code]
			c.errorCodeLock.RUnlock()
			if ok {
				xerr.Name = name
			} else {
				xerr.Name = "unknown error"
			}
			err = &xerr
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

// WaitForEvent blocks until the next event is available and returns it, or an error if the connection is closed.
func (c *Conn) WaitForEvent() Event {
	return <-c.eventChan
}

// PollForEvent returns the next event if one is available, or nil if no events are available.
func (c *Conn) PollForEvent() Event {
	select {
	case ev := <-c.eventChan:
		return ev
	default:
		return nil
	}
}

func (c *Conn) hasExtension(name string) *QueryExtensionReply {
	c.extensionsLock.RLock()
	data, ok := c.extensions[name]
	c.extensionsLock.RUnlock()
	if ok {
		return data
	}
	c.extensionsLock.Lock()
	defer c.extensionsLock.Unlock()
	var err error
	data, err = QueryExtension(c, name)
	if err != nil {
		data = &QueryExtensionReply{}
	}
	if c.extensions == nil {
		c.extensions = make(map[string]*QueryExtensionReply)
	}
	c.extensions[name] = data
	return data
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

func (c *Conn) newKeyPressEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newKeyReleaseEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newButtonPressEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newButtonReleaseEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newMotionNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newEnterNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newLeaveNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newFocusInEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newFocusOutEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newKeymapNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newExposeEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newGraphicsExposureEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newNoExposureEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newVisibilityNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newCreateNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newDestroyNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newUnmapNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newMapNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newMapRequestEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newReparentNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newConfigureNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newConfigureRequestEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newGravityNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newResizeRequestEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newCirculateNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newCirculateRequestEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newPropertyNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newSelectionClearEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newSelectionRequestEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newSelectionNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newColormapNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newClientMessageEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newMappingNotifyEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

func (c *Conn) newGenericEventEvent(r *Reader) Event {
	// TODO: Implement
	return nil
}

// Close the connection after finishing any in-flight requests.
func (c *Conn) Close() {
	select {
	case c.reqChan <- nil:
	case <-c.doneSend:
	}
}
