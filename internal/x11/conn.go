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
	"image"
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

// MaxRequestSize is the maximum size of an X11 request in bytes.
const MaxRequestSize = math.MaxUint16 * 4

// Constants for X11 request opcodes.
const (
	opCreateWindow = 1 + iota
	opChangeWindowAttributes
	opGetWindowAttributes
	opDestroyWindow
	opDestroySubwindows
	opChangeSaveSet
	opReparentWindow
	opMapWindow
	opMapSubwindows
	opUnmapWindow
	opUnmapSubwindows
	opConfigureWindow
	opCirculateWindow
	opGetGeometry
	opQueryTree
	opInternAtom
	opGetAtomName
	opChangeProperty
	opDeleteProperty
	opGetProperty
	opListProperties
	opSetSelectionOwner
	opGetSelectionOwner
	opConvertSelection
	opSendEvent
	opGrabPointer
	opUngrabPointer
	opGrabButton
	opUngrabButton
	opChangeActivePointerGrab
	opGrabKeyboard
	opUngrabKeyboard
	opGrabKey
	opUngrabKey
	opAllowEvents
	opGrabServer
	opUngrabServer
	opQueryPointer
	opGetMotionEvents
	opTranslateCoordinates
	opWarpPointer
	opSetInputFocus
	opGetInputFocus
	opQueryKeymap
	opOpenFont
	opCloseFont
	opQueryFont
	opQueryTextExtents
	opListFonts
	opListFontsWithInfo
	opSetFontPath
	opGetFontPath
	opCreatePixmap
	opFreePixmap
	opCreateGC
	opChangeGC
	opCopyGC
	opSetDashes
	opSetClipRectangles
	opFreeGC
	opClearArea
	opCopyArea
	opCopyPlane
	opPolyPoint
	opPolyLine
	opPolySegment
	opPolyRectangle
	opPolyArc
	opFillPoly
	opPolyFillRectangle
	opPolyFillArc
	opPutImage
	opGetImage
	opPolyText8
	opPolyText16
	opImageText8
	opImageText16
	opCreateColormap
	opFreeColormap
	opCopyColormapAndFree
	opInstallColormap
	opUninstallColormap
	opListInstalledColormaps
	opAllocColor
	opAllocNamedColor
	opAllocColorCells
	opAllocColorPlanes
	opFreeColors
	opStoreColors
	opStoreNamedColor
	opQueryColors
	opLookupColor
	opCreateCursor
	opCreateGlyphCursor
	opFreeCursor
	opRecolorCursor
	opQueryBestSize
	opQueryExtension
	opListExtensions
	opChangeKeyboardMapping
	opGetKeyboardMapping
	opChangeKeyboardControl
	opGetKeyboardControl
	opBell
	opChangePointerControl
	opGetPointerControl
	opSetScreenSaver
	opGetScreenSaver
	opChangeHosts
	opListHosts
	opSetAccessControl
	opSetCloseDownMode
	opKillClient
	opRotateProperties
	opForceScreenSaver
	opSetPointerMapping
	opGetPointerMapping
	opSetModifierMapping
	opGetModifierMapping
	opUndefined1
	opUndefined2
	opUndefined3
	opUndefined4
	opUndefined5
	opUndefined6
	opUndefined7
	opNoOperation
)

// Constants for X11 window classes.
const (
	WindowClassCopyFromParent = iota
	WindowClassInputOutput
	WindowClassInputOnly
)

// Constants for X11 property events.
const (
	propertyNewValue = iota
	propertyDelete
)

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

// HasVersion returns true if the extension is present and has at least the specified major and minor version.
func (e *extensionInfo) HasVersion(minMajorVersion, minMinorVersion uint32) bool {
	if !e.Present {
		return false
	}
	if e.MajorVersion < minMajorVersion {
		return false
	}
	if e.MajorVersion == minMajorVersion && e.MinorVersion < minMinorVersion {
		return false
	}
	return true
}

// Various X11 type IDs.
//
//nolint:revive // No need to have separate comments for each of these.
type (
	ColorMapID   uint32
	CursorID     uint32
	DrawableID   uint32
	FontID       uint32
	GCID         uint32
	GLXContextID uint32
	GLXWindowID  uint32
	PictureID    uint32
	PixMapID     uint32
	VisualID     uint32
	WindowID     uint32
)

// Format holds the configuration of a pixmap.
type Format struct {
	Depth        byte
	BitsPerPixel byte
	ScanlinePad  byte
}

// Visual holds the configuration of a screen's pixel composition for a specific bit depth.
type Visual struct {
	VisualID        VisualID
	RedMask         uint32
	GreenMask       uint32
	BlueMask        uint32
	ColormapEntries uint16
	Class           byte
	BitsPerRgbValue byte
}

// Depth holds the Visuals for a given screen bit depth.
type Depth struct {
	Visuals []Visual
	Depth   byte
}

// Screen holds the configuration of a monitor.
type Screen struct {
	AllowedDepths       []Depth
	Root                WindowID
	DefaultColorMap     ColorMapID
	WhitePixel          uint32
	BlackPixel          uint32
	CurrentInputMasks   uint32
	WidthInPixels       uint16
	HeightInPixels      uint16
	WidthInMillimeters  uint16
	HeightInMillimeters uint16
	MinInstalledMaps    uint16
	MaxInstalledMaps    uint16
	RootVisual          VisualID
	BackingStores       byte
	SaveUnders          bool
	RootDepth           byte
}

// WindowValueMask represents the bitmask for specifying which window attributes to set or get.
type WindowValueMask uint32

// Window value mask bits.
const (
	WindowMaskBackPixMap WindowValueMask = 1 << iota
	WindowMaskBackPixel
	WindowMaskBorderPixMap
	WindowMaskBorderPixel
	WindowMaskBitGravity
	WindowMaskWinGravity
	WindowMaskBackingStore
	WindowMaskBackingPlanes
	WindowMaskBackingPixel
	WindowMaskOverrideRedirect
	WindowMaskSaveUnder
	WindowMaskEventMask
	WindowMaskDontPropagate
	WindowMaskColorMap
	WindowMaskCursor
)

// WindowAttributes holds the attributes that can be set on a window.
type WindowAttributes struct {
	BackgroundPixMap   PixMapID
	BackgroundPixel    uint32
	BorderPixMap       PixMapID
	BorderPixel        uint32
	BitGravity         uint32
	WinGravity         uint32
	BackingStore       uint32
	BackingPlanes      uint32
	BackingPixel       uint32
	EventMask          uint32
	DoNotPropagateMask uint32
	ColorMap           ColorMapID
	Cursor             CursorID
	OverrideRedirect   bool
	SaveUnder          bool
}

// GCValueMask represents the bitmask for specifying which GC attributes to set or get.
type GCValueMask uint32

// GC value mask bits.
const (
	GCMaskFunction GCValueMask = 1 << iota
	GCMaskPlaneMask
	GCMaskForeground
	GCMaskBackground
	GCMaskLineWidth
	GCMaskLineStyle
	GCMaskCapStyle
	GCMaskJoinStyle
	GCMaskFillStyle
	GCMaskFillRule
	GCMaskTile
	GCMaskStipple
	GCMaskTileStippleOriginX
	GCMaskTileStippleOriginY
	GCMaskFont
	GCMaskSubwindowMode
	GCMaskGraphicsExposures
	GCMaskClipOriginX
	GCMaskClipOriginY
	GCMaskClipMask
	GCMaskDashOffset
	GCMaskDashList
	GCMaskArcMode
)

// GCFunction represents an X11 graphics function.
type GCFunction byte

// Graphics function constants.
const (
	GxClear GCFunction = iota
	GxAnd
	GxAndReverse
	GxCopy
	GxAndInverted
	GxNoop
	GxXor
	GxOr
	GxNor
	GxEquiv
	GxInvert
	GxOrReverse
	GxCopyInverted
	GxOrInverted
	GxNand
	GxSet
)

// LineStyle represents the line style for drawing operations.
type LineStyle byte

// Possible LineStyle values.
const (
	LineStyleSolid LineStyle = iota
	LineStyleOnOffDash
	LineStyleDoubleDash
)

// CapStyle represents the cap style for line endpoints.
type CapStyle byte

// Possible CapStyle values.
const (
	CapStyleNotLast CapStyle = iota
	CapStyleButt
	CapStyleRound
	CapStyleProjecting
)

// JoinStyle represents the join style for line segments.
type JoinStyle byte

// Possible JoinStyle values.
const (
	JoinStyleMiter JoinStyle = iota
	JoinStyleRound
	JoinStyleBevel
)

// FillStyle represents the fill style for drawing operations.
type FillStyle byte

// Possible FillStyle values.
const (
	FillStyleSolid FillStyle = iota
	FillStyleTiled
	FillStyleStippled
	FillStyleOpaqueStippled
)

// FillRule represents the fill rule for polygon filling operations.
type FillRule byte

// Possible FillRule values.
const (
	FillRuleEvenOdd FillRule = iota
	FillRuleWinding
)

// SubwindowMode represents the subwindow mode for graphics contexts and pictures.
type SubwindowMode byte

// Possible SubwindowMode values.
const (
	SubwindowModeClipByChildren SubwindowMode = iota
	SubwindowModeIncludeInferiors
)

// ArcMode represents the mode for rendering arcs in a graphics context.
type ArcMode byte

// Possible ArcMode values.
const (
	ArcModeChord ArcMode = iota
	ArcModePieSlice
)

// ImageFormat represents the format for image data in X11 operations.
type ImageFormat byte

// Possible ImageFormat values.
const (
	ImageFormatXYBitmap ImageFormat = iota
	ImageFormatXYPixmap
	ImageFormatZPixmap
)

// GCAttrs specifies the attributes of a graphics context resource.
type GCAttrs struct {
	PlaneMask          uint32
	Foreground         uint32
	Background         uint32
	DashOffset         uint32
	Font               FontID
	ClipMask           PixMapID
	Tile               PixMapID
	Stipple            PixMapID
	ClipOriginX        int16
	ClipOriginY        int16
	TileStippleOriginX int16
	TileStippleOriginY int16
	LineWidth          uint16
	LineStyle          LineStyle
	CapStyle           CapStyle
	JoinStyle          JoinStyle
	FillStyle          FillStyle
	FillRule           FillRule
	SubwindowMode      SubwindowMode
	Function           GCFunction
	GraphicsExposures  bool
	Dashes             byte
	ArcMode            ArcMode
}

// Conn represents a connection to an X server.
type Conn struct {
	conn                     net.Conn
	events                   chan Event
	requests                 chan *request
	closed                   chan struct{}
	readClosed               chan struct{}
	ExtGLX                   *ExtGLX
	ExtMisc                  *ExtMisc
	ExtRandr                 *ExtRandr
	ExtRender                *ExtRender
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
	pixmapFormats            []Format
	Roots                    []Screen
	eventNewMapLock          sync.RWMutex
	errorCodeLock            sync.RWMutex
	requestMapLock           sync.RWMutex
	xidLock                  sync.Mutex
	xidBase                  uint32
	xidInc                   uint32
	xidMax                   uint32
	xidLast                  uint32
	DefaultScreen            int
	displayNum               int
	sequence                 atomic.Uint32
	releaseNumber            uint32
	motionBufferSize         uint32
	helperWindow             WindowID
	Atoms                    Atoms
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
	go c.sendRequests()
	go c.readResponses()
	if err = c.Atoms.init(&c); err != nil {
		return nil, err
	}
	c.ExtMisc = newExtMisc(&c)
	if c.ExtGLX = newExtGLX(&c); !c.ExtGLX.HasVersion(1, 4) {
		return nil, errs.New("X11 extension GLX 1.4 or higher is required")
	}
	if c.ExtRandr = newExtRandr(&c); !c.ExtRandr.HasVersion(1, 5) {
		return nil, errs.New("X11 extension RANDR 1.5 or higher is required")
	}
	if c.ExtRender = newExtRender(&c); !c.ExtRender.HasVersion(0, 6) {
		return nil, errs.New("X11 extension RENDER 0.6 or higher is required")
	}
	if c.helperWindow = c.CreateWindow(c.RootWindow(), 0, 0, 1, 1, 0, WindowClassInputOnly, 0, c.DefaultVisual(),
		WindowMaskEventMask, &WindowAttributes{EventMask: EventMaskPropertyChange}); c.helperWindow == 0 {
		return nil, errs.New("failed to create helper window")
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
		c.xidBase = r.Uint32()
		c.xidMax = r.Uint32()
		c.xidInc = c.xidMax & -c.xidMax
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
		c.pixmapFormats = ReadList(int(pixmapFormatsLen), r, func(rr *Reader) Format {
			var f Format
			f.Depth = rr.Byte()
			f.BitsPerPixel = rr.Byte()
			f.ScanlinePad = rr.Byte()
			rr.Skip(5)
			return f
		})
		c.Roots = ReadList(int(rootsLen), r, func(rr *Reader) Screen {
			var s Screen
			s.Root = rr.WindowID()
			s.DefaultColorMap = rr.ColorMapID()
			s.WhitePixel = rr.Uint32()
			s.BlackPixel = rr.Uint32()
			s.CurrentInputMasks = rr.Uint32()
			s.WidthInPixels = rr.Uint16()
			s.HeightInPixels = rr.Uint16()
			s.WidthInMillimeters = rr.Uint16()
			s.HeightInMillimeters = rr.Uint16()
			if s.WidthInMillimeters == 0 || s.HeightInMillimeters == 0 {
				// Assume 96 DPI if we don't receive useful info
				s.WidthInMillimeters = uint16(float64(s.WidthInPixels) * 25.4 / 96.0)
				s.HeightInMillimeters = uint16(float64(s.HeightInPixels) * 25.4 / 96.0)
			}
			s.MinInstalledMaps = rr.Uint16()
			s.MaxInstalledMaps = rr.Uint16()
			s.RootVisual = rr.VisualID()
			s.BackingStores = rr.Byte()
			s.SaveUnders = rr.Bool()
			s.RootDepth = rr.Byte()
			s.AllowedDepths = ReadList(int(rr.Byte()), rr, func(rrr *Reader) Depth {
				var d Depth
				d.Depth = rrr.Byte()
				rrr.Skip(1)
				count := rrr.Uint16()
				rrr.Skip(4)
				d.Visuals = ReadList(int(count), rrr, func(rrrr *Reader) Visual {
					var v Visual
					v.VisualID = rrrr.VisualID()
					v.Class = rrrr.Byte()
					v.BitsPerRgbValue = rrrr.Byte()
					v.ColormapEntries = rrrr.Uint16()
					v.RedMask = rrrr.Uint32()
					v.GreenMask = rrrr.Uint32()
					v.BlueMask = rrrr.Uint32()
					rrrr.Skip(4)
					return v
				})
				return d
			})
			return s
		})
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

func (c *Conn) nextXID() (uint32, error) {
	c.xidLock.Lock()
	defer c.xidLock.Unlock()
	switch {
	case c.xidLast <= c.xidMax-c.xidInc:
		c.xidLast += c.xidInc
	case c.ExtMisc.Present:
		startID, count, err := c.ExtMisc.GetXIDRange()
		if err != nil {
			return 0, err
		}
		c.xidLast = startID
		c.xidMax = startID + (count-1)*c.xidInc
	default:
		return 0, errs.New("no more IDs available")
	}
	return c.xidLast | c.xidBase, nil
}

func nextXID[T ~uint32](c *Conn) T {
	id, err := c.nextXID()
	if err != nil {
		errs.Log(err)
		return 0
	}
	return T(id)
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
	w.Byte(opSendEvent)
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

func (c *Conn) hasExtension(name string, versionOpCode byte, versionIs16Bit bool, majorMax, minorMax uint32) extensionInfo {
	size := 8 + pad4(len(name))
	w := NewWriter(size)
	w.Byte(opQueryExtension)
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
		w.Byte(versionOpCode)
		if versionIs16Bit {
			w.Uint16(2)
			w.Uint16(uint16(majorMax))
			w.Uint16(uint16(minorMax))
		} else {
			w.Uint16(3)
			w.Uint32(majorMax)
			w.Uint32(minorMax)
		}
		if err := c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
			r.Skip(8)
			if versionIs16Bit {
				info.MajorVersion = uint32(r.Uint16())
				info.MinorVersion = uint32(r.Uint16())
				r.Skip(20)
			} else {
				info.MajorVersion = r.Uint32()
				info.MinorVersion = r.Uint32()
				r.Skip(16)
			}
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
	w.Byte(opInternAtom)
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
func (c *Conn) CreateWindow(parent WindowID, x, y int16, width, height, borderWidth, windowClass uint16, depth byte, visual VisualID, mask WindowValueMask, attrs *WindowAttributes) WindowID {
	id := nextXID[WindowID](c)
	if id != 0 {
		var values []uint32
		if attrs != nil {
			values = make([]uint32, 0, 15)
			if mask&WindowMaskBackPixMap != 0 {
				values = append(values, uint32(attrs.BackgroundPixMap))
			}
			if mask&WindowMaskBackPixel != 0 {
				values = append(values, attrs.BackgroundPixel)
			}
			if mask&WindowMaskBorderPixMap != 0 {
				values = append(values, uint32(attrs.BorderPixMap))
			}
			if mask&WindowMaskBorderPixel != 0 {
				values = append(values, attrs.BorderPixel)
			}
			if mask&WindowMaskBitGravity != 0 {
				values = append(values, attrs.BitGravity)
			}
			if mask&WindowMaskWinGravity != 0 {
				values = append(values, attrs.WinGravity)
			}
			if mask&WindowMaskBackingStore != 0 {
				values = append(values, attrs.BackingStore)
			}
			if mask&WindowMaskBackingPlanes != 0 {
				values = append(values, attrs.BackingPlanes)
			}
			if mask&WindowMaskBackingPixel != 0 {
				values = append(values, attrs.BackingPixel)
			}
			if mask&WindowMaskOverrideRedirect != 0 {
				if attrs.OverrideRedirect {
					values = append(values, 1)
				} else {
					values = append(values, 0)
				}
			}
			if mask&WindowMaskSaveUnder != 0 {
				if attrs.SaveUnder {
					values = append(values, 1)
				} else {
					values = append(values, 0)
				}
			}
			if mask&WindowMaskEventMask != 0 {
				values = append(values, attrs.EventMask)
			}
			if mask&WindowMaskDontPropagate != 0 {
				values = append(values, attrs.DoNotPropagateMask)
			}
			if mask&WindowMaskColorMap != 0 {
				values = append(values, uint32(attrs.ColorMap))
			}
			if mask&WindowMaskCursor != 0 {
				values = append(values, uint32(attrs.Cursor))
			}
		}
		size := 32 + 4*len(values)
		w := NewWriter(size)
		w.Byte(opCreateWindow)
		w.Byte(depth)
		w.Uint16(uint16(size / 4))
		w.WindowID(id)
		w.WindowID(parent)
		w.Int16(x)
		w.Int16(y)
		w.Uint16(width)
		w.Uint16(height)
		w.Uint16(borderWidth)
		w.Uint16(windowClass)
		w.VisualID(visual)
		w.Uint32(uint32(mask))
		w.Uint32Slice(values)
		w.ZeroTo4ByteAlignment()
		if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
			errs.Log(err)
			return 0
		}
	}
	return id
}

// DestroyWindow destroys the specified window.
func (c *Conn) DestroyWindow(window WindowID) error {
	w := NewWriter(8)
	w.Byte(opDestroyWindow)
	w.Zero(1)
	w.Uint16(2)
	w.WindowID(window)
	return c.sendNewRequest(newCheckedRequest(w))
}

// GetInputFocus returns the current input focus window and the revert-to value.
func (c *Conn) GetInputFocus() (focus WindowID, revertTo byte, err error) {
	w := NewWriter(4)
	w.Byte(opGetInputFocus)
	w.Zero(1)
	w.Uint16(1)
	err = c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(1)
		revertTo = r.Byte()
		r.Skip(6)
		focus = r.WindowID()
	}))
	return focus, revertTo, err
}

// GetProperty returns information about the specified property.
func (c *Conn) GetProperty(window WindowID, property, propertyType Atom, offset, length uint32, remove bool) (format byte, actualPropertyType Atom, value []byte, err error) {
	w := NewWriter(24)
	w.Byte(opGetProperty)
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
	w.Byte(opChangeProperty)
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
	w.Byte(opBell)
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
	if c.helperWindow == 0 {
		return ""
	}
	owner, err := c.getSelectionOwner(c.Atoms.Clipboard)
	if err != nil {
		errs.Log(err)
		return ""
	}
	if owner == c.helperWindow {
		return c.clipboard
	}
	c.clipboard = ""
	for _, kind := range []Atom{c.Atoms.UTF8String, AtomString} {
		c.convertSelection(c.helperWindow, c.Atoms.Clipboard, kind, c.Atoms.ClipboardSelection, 0)
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
			case c.Atoms.ClipboardIncremental:
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
				if kind == c.Atoms.UTF8String {
					c.clipboard = buffer.String()
				} else {
					c.clipboard = convertLatin1ToUTF8(buffer.Bytes())
				}
			case c.Atoms.UTF8String:
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
	if c.helperWindow == 0 {
		return
	}
	c.clipboard = str
	c.setSelectionOwner(c.helperWindow, c.Atoms.Clipboard)
}

func (c *Conn) setSelectionOwner(owner WindowID, selection Atom) {
	w := NewWriter(16)
	w.Byte(opSetSelectionOwner)
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
	w.Byte(opGetSelectionOwner)
	w.Zero(1)
	w.Uint16(2)
	w.Atom(selection)
	err = c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(8)
		owner = r.WindowID()
		r.Skip(20)
	}))
	return owner, err
}

func (c *Conn) convertSelection(requestor WindowID, selection, target, property Atom, timestamp uint32) {
	w := NewWriter(8)
	w.Byte(opConvertSelection)
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
	_, _, extentsBytes, err := c.GetProperty(root, c.Atoms.NetWorkArea, AtomCardinal, 0, math.MaxUint32, false)
	if err != nil {
		return area
	}
	r := NewReader(extentsBytes)
	extents := r.Uint32Slice(len(extentsBytes) / 4)
	var desktopBytes []byte
	if _, _, desktopBytes, err = c.GetProperty(root, c.Atoms.NetCurrentDesktop, AtomCardinal, 0, math.MaxUint32, false); err != nil {
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

// OpenFont opens a font with the specified name and returns its FontID.
func (c *Conn) OpenFont(name string) FontID {
	id := nextXID[FontID](c)
	if id != 0 {
		w := NewWriter(12 + pad4(len(name)))
		w.Byte(opOpenFont)
		w.Zero(1)
		w.Uint16(uint16(3 + (pad4(len(name)) / 4)))
		w.FontID(id)
		w.Uint16(uint16(len(name)))
		w.Zero(2)
		w.String(name)
		w.ZeroTo4ByteAlignment()
		if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
			errs.Log(err)
			return 0
		}
	}
	return id
}

// CloseFont closes the specified font.
func (c *Conn) CloseFont(fontID FontID) {
	if fontID == 0 {
		return
	}
	w := NewWriter(8)
	w.Byte(opCloseFont)
	w.Zero(1)
	w.Uint16(2)
	w.FontID(fontID)
	if err := c.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// CreateGlyphCursor creates a new cursor with the specified source and mask fonts, character codes, and foreground and
// background colors. It returns the ID of the newly created cursor.
func (c *Conn) CreateGlyphCursor(srcFontID, maskFontID FontID, sourceChar, maskChar, fgRed, fgGreen, fgBlue, bgRed, bgGreen, bgBlue uint16) CursorID {
	id := nextXID[CursorID](c)
	if id != 0 {
		w := NewWriter(32)
		w.Byte(opCreateGlyphCursor)
		w.Zero(1)
		w.Uint16(8)
		w.CursorID(id)
		w.FontID(srcFontID)
		w.FontID(maskFontID)
		w.Uint16(sourceChar)
		w.Uint16(maskChar)
		w.Uint16(fgRed)
		w.Uint16(fgGreen)
		w.Uint16(fgBlue)
		w.Uint16(bgRed)
		w.Uint16(bgGreen)
		w.Uint16(bgBlue)
		if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
			errs.Log(err)
			return 0
		}
	}
	return id
}

// FreeCursor frees the specified cursor.
func (c *Conn) FreeCursor(cursorID CursorID) {
	w := NewWriter(8)
	w.Byte(opFreeCursor)
	w.Zero(1)
	w.Uint16(2)
	w.CursorID(cursorID)
	if err := c.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// CreatePixMap creates a new pixmap with the specified drawable, depth, width, and height, and returns its PixMapID.
func (c *Conn) CreatePixMap(drawable DrawableID, depth byte, width, height uint16) PixMapID {
	id := nextXID[PixMapID](c)
	if id != 0 {
		w := NewWriter(16)
		w.Byte(opCreatePixmap)
		w.Byte(depth)
		w.Uint16(4)
		w.PixMapID(id)
		w.DrawableID(drawable)
		w.Uint16(width)
		w.Uint16(height)
		if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
			errs.Log(err)
			return 0
		}
	}
	return id
}

// FreePixMap frees the specified pixmap.
func (c *Conn) FreePixMap(pixmapID PixMapID) {
	w := NewWriter(8)
	w.Byte(opFreePixmap)
	w.Zero(1)
	w.Uint16(2)
	w.PixMapID(pixmapID)
	if err := c.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// CreateGC creates a new graphics context with the specified drawable, value mask, and values, and returns its GCID.
func (c *Conn) CreateGC(drawable DrawableID, mask GCValueMask, attrs *GCAttrs) GCID {
	id := nextXID[GCID](c)
	if id == 0 {
		return 0
	}
	var values []uint32
	if attrs != nil {
		values = make([]uint32, 0, 23)
		if mask&GCMaskFunction != 0 {
			values = append(values, uint32(attrs.Function))
		}
		if mask&GCMaskPlaneMask != 0 {
			values = append(values, attrs.PlaneMask)
		}
		if mask&GCMaskForeground != 0 {
			values = append(values, attrs.Foreground)
		}
		if mask&GCMaskBackground != 0 {
			values = append(values, attrs.Background)
		}
		if mask&GCMaskLineWidth != 0 {
			values = append(values, uint32(attrs.LineWidth))
		}
		if mask&GCMaskLineStyle != 0 {
			values = append(values, uint32(attrs.LineStyle))
		}
		if mask&GCMaskCapStyle != 0 {
			values = append(values, uint32(attrs.CapStyle))
		}
		if mask&GCMaskJoinStyle != 0 {
			values = append(values, uint32(attrs.JoinStyle))
		}
		if mask&GCMaskFillStyle != 0 {
			values = append(values, uint32(attrs.FillStyle))
		}
		if mask&GCMaskFillRule != 0 {
			values = append(values, uint32(attrs.FillRule))
		}
		if mask&GCMaskTile != 0 {
			values = append(values, uint32(attrs.Tile))
		}
		if mask&GCMaskStipple != 0 {
			values = append(values, uint32(attrs.Stipple))
		}
		if mask&GCMaskTileStippleOriginX != 0 {
			values = append(values, uint32(attrs.TileStippleOriginX))
		}
		if mask&GCMaskTileStippleOriginY != 0 {
			values = append(values, uint32(attrs.TileStippleOriginY))
		}
		if mask&GCMaskFont != 0 {
			values = append(values, uint32(attrs.Font))
		}
		if mask&GCMaskSubwindowMode != 0 {
			values = append(values, uint32(attrs.SubwindowMode))
		}
		if mask&GCMaskGraphicsExposures != 0 {
			var ge uint32
			if attrs.GraphicsExposures {
				ge = 1
			}
			values = append(values, ge)
		}
		if mask&GCMaskClipOriginX != 0 {
			values = append(values, uint32(attrs.ClipOriginX))
		}
		if mask&GCMaskClipOriginY != 0 {
			values = append(values, uint32(attrs.ClipOriginY))
		}
		if mask&GCMaskClipMask != 0 {
			values = append(values, uint32(attrs.ClipMask))
		}
		if mask&GCMaskDashOffset != 0 {
			values = append(values, attrs.DashOffset)
		}
		if mask&GCMaskDashList != 0 {
			values = append(values, uint32(attrs.Dashes))
		}
		if mask&GCMaskArcMode != 0 {
			values = append(values, uint32(attrs.ArcMode))
		}
	}
	w := NewWriter(16 + 4*len(values))
	w.Byte(opCreateGC)
	w.Zero(1)
	w.Uint16(4 + uint16(len(values)))
	w.GCID(id)
	w.DrawableID(drawable)
	w.Uint32(uint32(mask))
	w.Uint32Slice(values)
	if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
		return 0
	}
	return id
}

// FreeGC frees the specified graphics context.
func (c *Conn) FreeGC(gcID GCID) {
	w := NewWriter(8)
	w.Byte(opFreeGC)
	w.Zero(1)
	w.Uint16(2)
	w.GCID(gcID)
	if err := c.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// PutImage uploads the pixel data from the provided image to the specified drawable at the given destination
// coordinates using the provided graphics context. The image is sent in chunks if it exceeds the maximum request size
// allowed by the X server in a single request.
func (c *Conn) PutImage(drawable DrawableID, gc GCID, dstX, dstY int16, img *image.NRGBA) {
	width := uint16(img.Rect.Dx())
	w := int(width)
	height := uint16(img.Rect.Dy())
	h := int(height)
	rowsPer := (MaxRequestSize - 24) / (w * 4)
	for y := 0; y < h; y += rowsPer {
		rows := min(rowsPer, h-y)

		// Convert the pixels to pre-multiplied BGRA order, which is what X expects for 32bpp images.
		pix := make([]byte, rows*w*4)
		base := y * w * 4
		for i := 0; i < len(pix); i += 4 {
			si := base + i
			a := uint16(img.Pix[si+3])
			pix[i] = uint8((uint16(img.Pix[si+2]) * a) / 0xff)
			pix[i+1] = uint8((uint16(img.Pix[si+1]) * a) / 0xff)
			pix[i+2] = uint8((uint16(img.Pix[si]) * a) / 0xff)
			pix[i+3] = img.Pix[si+3]
		}

		w := NewWriter(24 + pad4(len(pix)))
		w.Byte(opPutImage)
		w.Byte(byte(ImageFormatZPixmap))
		w.Uint16(6 + uint16(pad4(len(pix))/4))
		w.DrawableID(drawable)
		w.GCID(gc)
		w.Uint16(width)
		w.Uint16(uint16(rows))
		w.Int16(dstX)
		w.Int16(dstY + int16(y))
		w.Byte(0)
		w.Byte(32)
		w.Zero(2)
		w.Bytes(pix)
		w.ZeroTo4ByteAlignment()
		if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
			errs.Log(err)
		}
	}
}

// pushClipboardToManager checks if the helper window is currently the owner of the CLIPBOARD selection, and if so, it
// converts the selection to the CLIPBOARD_MANAGER with the SAVE_TARGETS property. It then waits for events related to
// this conversion, processing any SelectionRequestEvent or SelectionClearEvent that may occur during this time.
// Finally, it destroys the helper window and resets its ID to 0.
func (c *Conn) pushClipboardToManager() {
	if c.helperWindow == 0 {
		return
	}
	if owner, err := c.getSelectionOwner(c.Atoms.Clipboard); err == nil && owner == c.helperWindow {
		c.convertSelection(c.helperWindow, c.Atoms.ClipboardManager, c.Atoms.ClipboardSaveTargets, AtomNone, 0)
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
				if e.Target == c.Atoms.ClipboardSaveTargets {
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
	c.helperWindow = 0
}

// Close the connection after finishing any in-flight requests.
func (c *Conn) Close() {
	c.pushClipboardToManager()
	c.Sync()
	close(c.requests)
	<-c.closed
}
