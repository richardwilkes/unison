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
	seq    chan struct{}
	cookie cookieProcessor
	data   *protoBufferWriter
}

// Conn represents a connection to an X server.
type Conn struct {
	conn                     net.Conn
	eventChan                chan any
	cookieChan               chan cookieProcessor
	xidChan                  chan xid
	seqChan                  chan uint16
	reqChan                  chan *request
	doneSend                 chan struct{}
	doneRead                 chan struct{}
	ExtMisc                  *ExtMisc
	extensions               map[string]*QueryExtensionReply
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
	c.cookieChan = make(chan cookieProcessor, 1024)
	c.xidChan = make(chan xid, 8)
	c.seqChan = make(chan uint16, 8)
	c.reqChan = make(chan *request, 128)
	c.eventChan = make(chan any, 8192)
	c.doneSend = make(chan struct{})
	c.doneRead = make(chan struct{})
	c.ExtMisc = &ExtMisc{conn: &c}
	go c.generateXIds()
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
	w := newProtoBufferWriter(18 + len(authName) + len(authData))
	w.byte(0x6C) // Use little endian
	w.zero(1)
	w.uint16(11) // Major version
	w.uint16(0)  // Minor version
	w.uint16(uint16(len(authName)))
	w.uint16(uint16(len(authData)))
	w.zero(2)
	w.string(authName)
	w.zeroTo4ByteAlignment()
	w.bytes(authData)
	w.zeroTo4ByteAlignment()
	if err := w.send(c.conn); err != nil {
		return errs.NewWithCause("failed to send authentication data", err)
	}
	header := newProtoBufferReader(make([]byte, 8))
	if err := header.load(c.conn); err != nil {
		return errs.NewWithCause("failed to read authentication response header", err)
	}
	code := header.byte()
	reasonLen := header.byte()
	c.protocolMajorVersion = header.uint16()
	c.protocolMinorVersion = header.uint16()
	dataLen := header.uint16() * 4
	if c.protocolMajorVersion != 11 || c.protocolMinorVersion != 0 {
		return errs.Newf("unsupported X protocol version: %d.%d", c.protocolMajorVersion, c.protocolMinorVersion)
	}
	data := newProtoBufferReader(make([]byte, int(dataLen)))
	if err := data.load(c.conn); err != nil {
		return errs.NewWithCause("failed to read authentication response data", err)
	}
	switch code {
	case 0:
		return errs.New("authentication refused: " + data.string(int(reasonLen)))
	case 1:
		c.protoRead(data)
		return nil
	case 2:
		return errs.New("further authentication required: " + data.zeroedString(int(dataLen)))
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
	r := newProtoBufferReaderWithOrder(binary.BigEndian, fileData)
	for r.len() != 0 {
		family := r.uint16()
		addr := r.sizePrefixedString()
		disp := r.sizePrefixedString()
		name = r.sizePrefixedString()
		data = r.sizePrefixedBytes()
		if ((family == 65535) || (family == 256 && addr == c.host)) &&
			((disp == "") || (disp == c.display)) {
			return name, data
		}
	}
	return "", nil
}

func (c *Conn) protoRead(r *protoBufferReader) {
	c.releaseNumber = r.uint32()
	c.resourceIDBase = r.uint32()
	c.resourceIDMask = r.uint32()
	c.motionBufferSize = r.uint32()
	vendorLen := r.uint16()
	c.maximumRequestLength = r.uint16()
	rootsLen := r.byte()
	pixmapFormatsLen := r.byte()
	c.imageByteOrder = r.byte()
	c.bitmapFormatBitOrder = r.byte()
	c.bitmapFormatScanlineUnit = r.byte()
	c.bitmapFormatScanlinePad = r.byte()
	c.minKeycode = r.byte()
	c.maxKeycode = r.byte()
	r.skip(4)
	c.vendor = r.string(int(vendorLen))
	r.skipTo4ByteAlignment()
	c.pixmapFormats = readProtoList[*Format](int(pixmapFormatsLen), r)
	c.roots = readProtoList[*Screen](int(rootsLen), r)
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

func (c *Conn) generateXIds() {
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

func (c *Conn) newRequest(data *protoBufferWriter, cook cookieProcessor) {
	seq := make(chan struct{})
	select {
	case c.reqChan <- &request{seq: seq, cookie: cook, data: data}:
		select {
		case <-seq:
		case <-c.doneSend:
		}
	case <-c.doneSend:
	}
}

func (c *Conn) sendRequests() {
	defer close(c.cookieChan)
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
			if len(c.cookieChan) == cap(c.cookieChan)-1 {
				if err := c.noop(); err != nil {
					xio.CloseIgnoringErrors(c.conn)
					<-c.doneRead
					return
				}
			}
			req.cookie.setSequenceID(c.newSequenceID())
			c.cookieChan <- req.cookie
			if err := req.data.send(c.conn); err != nil {
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
	cook := newCookie(c, true, true, &GetInputFocusReply{})
	cook.setSequenceID(c.newSequenceID())
	c.cookieChan <- cook
	if err := getInputFocusRequest().send(c.conn); err != nil {
		return err
	}
	cook.Reply() //nolint:errcheck // Don't care about errors here
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
		r := newProtoBufferReader(make([]byte, 32))
		if err = r.load(c.conn); err != nil {
			c.bail(err)
			return
		}
		switch r.byte() {
		case 0: // Error
			// TODO: Implement
			// Use the constructor function for this error (that is auto
			// generated) by looking it up by the error number.
			// newErrFun, ok := NewErrorFuncs[int(buf[1])]
			// if !ok {
			// 	Logger.Printf("BUG: Could not find error constructor function "+
			// 		"for error with number %d.", buf[1])
			continue
			// }
			// err = newErrFun(buf)
			// seq = err.SequenceId()
		case 1: // Reply
			r.skip(1)
			seq = r.uint16()
			if size := r.uint32(); size > 0 {
				if err = r.append(int(size)*4, c.conn); err != nil {
					c.bail(err)
					return
				}
			}
			r.seek(0)
		default: // Event
			// TODO: Implement
			// Use the constructor function for this event (like for errors,
			// and is also auto generated) by looking it up by the event number.
			// Note that we AND the event number with 127 so that we ignore
			// the most significant bit (which is set when it was sent from
			// a SendEvent request).
			// evNum := int(buf[0] & 127)
			// newEventFun, ok := NewEventFuncs[evNum]
			// if !ok {
			// 	Logger.Printf("BUG: Could not find event construct function "+
			// 		"for event with number %d.", evNum)
			continue
			// }
			// c.eventChan <- newEventFun(buf)
			// continue
		}
		for one := range c.cookieChan {
			if one.processCookie(seq, r, err) {
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
		c.eventChan <- err
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

// Close the connection after finishing any in-flight requests.
func (c *Conn) Close() {
	select {
	case c.reqChan <- nil:
	case <-c.doneSend:
	}
}
