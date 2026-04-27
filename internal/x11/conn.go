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
	"slices"
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
	PropertyNewValue = iota
	PropertyDelete
)

const (
	// MWMHintsDecorations specifies that the decorations field is defined.
	MWMHintsDecorations = 2
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
	ColorMapID uint32
	CursorID   uint32
	DrawableID uint32
	FontID     uint32
	GCID       uint32
	PictureID  uint32
	PixMapID   uint32
	VisualID   uint32
	WindowID   uint32
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

const (
	netWMStateRemove = iota
	netWMStateAdd
	newWMStateToggle
)

// Possible window states.
const (
	StateWithdrawn = iota
	StateNormal
	_
	StateIconic
)

const (
	_ = iota
	sourceNormalApp
	sourcePager
)

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

// WindowCreationAttributes holds the attributes that can be set on a window you are about to create.
type WindowCreationAttributes struct {
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

func (a *WindowCreationAttributes) values(mask WindowValueMask) []uint32 {
	values := make([]uint32, 0, 15)
	if mask&WindowMaskBackPixMap != 0 {
		values = append(values, uint32(a.BackgroundPixMap))
	}
	if mask&WindowMaskBackPixel != 0 {
		values = append(values, a.BackgroundPixel)
	}
	if mask&WindowMaskBorderPixMap != 0 {
		values = append(values, uint32(a.BorderPixMap))
	}
	if mask&WindowMaskBorderPixel != 0 {
		values = append(values, a.BorderPixel)
	}
	if mask&WindowMaskBitGravity != 0 {
		values = append(values, a.BitGravity)
	}
	if mask&WindowMaskWinGravity != 0 {
		values = append(values, a.WinGravity)
	}
	if mask&WindowMaskBackingStore != 0 {
		values = append(values, a.BackingStore)
	}
	if mask&WindowMaskBackingPlanes != 0 {
		values = append(values, a.BackingPlanes)
	}
	if mask&WindowMaskBackingPixel != 0 {
		values = append(values, a.BackingPixel)
	}
	if mask&WindowMaskOverrideRedirect != 0 {
		if a.OverrideRedirect {
			values = append(values, 1)
		} else {
			values = append(values, 0)
		}
	}
	if mask&WindowMaskSaveUnder != 0 {
		if a.SaveUnder {
			values = append(values, 1)
		} else {
			values = append(values, 0)
		}
	}
	if mask&WindowMaskEventMask != 0 {
		values = append(values, a.EventMask)
	}
	if mask&WindowMaskDontPropagate != 0 {
		values = append(values, a.DoNotPropagateMask)
	}
	if mask&WindowMaskColorMap != 0 {
		values = append(values, uint32(a.ColorMap))
	}
	if mask&WindowMaskCursor != 0 {
		values = append(values, uint32(a.Cursor))
	}
	return values
}

// Possible MapState values.
const (
	MapStateUnmapped = iota
	MapStateUnviewable
	MapStateViewable
)

// WindowAttributes holds the attributes that can be retrieved from a window.
type WindowAttributes struct {
	Visual             VisualID
	Colormap           ColorMapID
	BackingPlanes      uint32
	BackingPixel       uint32
	AllEventMasks      uint32
	YourEventMask      uint32
	Class              uint16
	DoNotPropagateMask uint16
	BackingStore       byte
	BitGravity         byte
	WinGravity         byte
	SaveUnder          bool
	MapIsInstalled     bool
	MapState           byte
	OverrideRedirect   bool
}

// Possible gravity values.
const (
	ForgetGravity = iota
	NorthWestGravity
	NorthGravity
	NorthEastGravity
	WestGravity
	CenterGravity
	EastGravity
	SouthWestGravity
	SouthGravity
	SouthEastGravity
	StaticGravity
)

// WindowSizeHintsMask represents the bitmask for specifying which size hints to set or get.
type WindowSizeHintsMask uint32

// Possible WindowSizeHintsMask values.
const (
	WSHMUSPosition WindowSizeHintsMask = 1 << iota
	WSHMUSSize
	WSHMPPosition
	WSHMPSize
	WSHMPMinSize
	WSHMPMaxSize
	WSHMPResizeInc
	WSHMPAspect
	WSHMPBaseSize
	WSHMPWinGravity
)

// WindowSizeHints holds the size hints that can be set on a window.
type WindowSizeHints struct {
	Flags      WindowSizeHintsMask
	X          int32
	Y          int32
	Width      uint32
	Height     uint32
	MinWidth   uint32
	MinHeight  uint32
	MaxWidth   uint32
	MaxHeight  uint32
	WidthInc   uint32
	HeightInc  uint32
	MinAspectX uint32
	MinAspectY uint32
	MaxAspectX uint32
	MaxAspectY uint32
	BaseWidth  uint32
	BaseHeight uint32
	WinGravity uint32
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

// KeyboardMapping holds the key symbol mapping for a range of key codes.
type KeyboardMapping struct {
	KeySyms           []uint32
	KeySymsPerKeyCode byte
}

// Geometry represents the geometry of a drawable.
type Geometry struct {
	Root        WindowID
	X           int16
	Y           int16
	Width       uint16
	Height      uint16
	BorderWidth uint16
	Depth       byte
}

// Conn represents a connection to an X server.
type Conn struct {
	conn                     net.Conn
	events                   chan Event
	requests                 chan *request
	closed                   chan struct{}
	readClosed               chan struct{}
	ExtXFixes                *ExtXFixes
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
	eventQueue               []Event
	eventQueueLock           sync.Mutex
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
	cachedScale              float32
	Atoms                    Atoms
	protocolMajorVersion     uint16
	protocolMinorVersion     uint16
	maximumRequestLength     uint16
	imageByteOrder           byte
	bitmapFormatBitOrder     byte
	bitmapFormatScanlineUnit byte
	bitmapFormatScanlinePad  byte
	MinKeyCode               byte
	MaxKeyCode               byte
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
	if c.ExtXFixes = newExtXFixes(&c); !c.ExtXFixes.HasVersion(4, 0) {
		return nil, errs.New("X11 extension XFIXES 4.0 or higher is required")
	}
	c.ExtMisc = newExtMisc(&c)
	if c.ExtRandr = newExtRandr(&c); !c.ExtRandr.HasVersion(1, 5) {
		return nil, errs.New("X11 extension RANDR 1.5 or higher is required")
	}
	if c.ExtRender = newExtRender(&c); !c.ExtRender.HasVersion(0, 6) {
		return nil, errs.New("X11 extension RENDER 0.6 or higher is required")
	}
	if c.helperWindow = c.CreateWindow(c.RootWindow(), 0, 0, 1, 1, 0, WindowClassInputOnly, 0, c.DefaultVisual(),
		WindowMaskEventMask, &WindowCreationAttributes{EventMask: EventMaskPropertyChange}); c.helperWindow == 0 {
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
		c.MinKeyCode = r.Byte()
		c.MaxKeyCode = r.Byte()
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
			if req.data == nil { // Flush request, just ack
				close(req.sentChan)
				continue
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
			if req.data != nil {
				if err := req.data.Send(c.conn); err != nil {
					errs.Log(err)
					xio.CloseIgnoringErrors(c.conn)
					<-c.readClosed
					return
				}
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

// Flush all pending requests to the X server.
func (c *Conn) Flush() {
	if err := c.sendNewRequest(&request{
		sentChan: make(chan struct{}),
	}); err != nil {
		errs.Log(err)
	}
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

func (c *Conn) queuedEvent(filter func(Event) bool) (Event, bool) {
	c.eventQueueLock.Lock()
	defer c.eventQueueLock.Unlock()
	if len(c.eventQueue) == 0 {
		return nil, false
	}
	for i, e := range c.eventQueue {
		if filter == nil || filter(e) {
			c.eventQueue = slices.Delete(c.eventQueue, i, i+1)
			return e, true
		}
	}
	return nil, false
}

// PullEvents retrieves all currently queued events, including any pending events in the channel.
func (c *Conn) PullEvents() []Event {
	c.eventQueueLock.Lock()
	defer c.eventQueueLock.Unlock()
	events := c.eventQueue
	c.eventQueue = nil
	for {
		select {
		case e, ok := <-c.events:
			if !ok {
				return events
			}
			events = append(events, e)
		default:
			return events
		}
	}
}

// WaitEvents blocks until the next event is available. If the optional filter function is provided, only events for
// which the filter returns true will be returned, and other events will be queued for later retrieval. nil may be
// returned if the connection is closed.
func (c *Conn) WaitEvents(filter func(Event) bool) Event {
	e, ok := c.queuedEvent(filter)
	if ok {
		return e
	}
	for {
		if e, ok = <-c.events; !ok {
			return nil
		}
		if filter == nil || filter(e) {
			return e
		}
		c.eventQueueLock.Lock()
		c.eventQueue = append(c.eventQueue, e)
		c.eventQueueLock.Unlock()
	}
}

// PollEvents processes the next event if one is available. If the optional filter function is provided, only events for
// which the filter returns true will be returned, and other events will be queued for later retrieval. nil will be
// returned if no events are currently available or if the connection is closed.
func (c *Conn) PollEvents(filter func(Event) bool) Event {
	e, ok := c.queuedEvent(filter)
	if ok {
		return e
	}
	select {
	case e = <-c.events:
		if filter == nil || filter(e) {
			return e
		}
		c.eventQueueLock.Lock()
		c.eventQueue = append(c.eventQueue, e)
		c.eventQueueLock.Unlock()
	default:
	}
	return nil
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

// GetAtomName returns the name of the specified Atom.
func (c *Conn) GetAtomName(atom Atom) (string, error) {
	w := NewWriter(8)
	w.Byte(opGetAtomName)
	w.Zero(1)
	w.Uint16(2)
	w.Atom(atom)
	var name string
	if err := c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(8)
		length := r.Uint16()
		r.Skip(22)
		name = r.String(int(length))
	})); err != nil {
		return "", err
	}
	return name, nil
}

// CreateWindow creates a new window with the specified parameters and attributes.
func (c *Conn) CreateWindow(parent WindowID, x, y int16, width, height, borderWidth, windowClass uint16, depth byte, visual VisualID, mask WindowValueMask, attrs *WindowCreationAttributes) WindowID {
	id := nextXID[WindowID](c)
	if id != 0 {
		values := attrs.values(mask)
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
func (c *Conn) DestroyWindow(window WindowID) {
	w := NewWriter(8)
	w.Byte(opDestroyWindow)
	w.Zero(1)
	w.Uint16(2)
	w.WindowID(window)
	if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
	}
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

// DeleteProperty deletes the specified property from the given window.
func (c *Conn) DeleteProperty(window WindowID, property Atom) {
	w := NewWriter(12)
	w.Byte(opDeleteProperty)
	w.Zero(1)
	w.Uint16(3)
	w.WindowID(window)
	w.Atom(property)
	if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
	}
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
// with the specified format (8, 16, or 32 bits per unit). Automatic chunking will occur if the data exceeds the maximum
// request size and mode is PropModeReplace, so there is no need to manually split the data into multiple requests for
// large properties.
func (c *Conn) ChangeProperty(window WindowID, property, propertyType Atom, format, mode byte, data []byte) {
	if format != 0 && format != 8 && format != 16 && format != 32 {
		slog.Error("invalid format for ChangeProperty (must be 0, 8, 16, or 32)", "format", format)
		return
	}
	unitSize := int(format / 8)
	offset := 0
	remaining := len(data) / unitSize
	onlyOnce := format == 0 || mode != PropModeReplace
	for remaining > 0 || format == 0 {
		size := remaining
		if size > math.MaxUint32-24 {
			size = math.MaxUint32 - 24
			size /= unitSize
			size *= unitSize
		}
		w := NewWriter(24 + pad4(size))
		w.Byte(opChangeProperty)
		w.Byte(mode)
		w.Uint16(uint16((24 + pad4(size)) / 4))
		w.WindowID(window)
		w.Atom(property)
		w.Atom(propertyType)
		w.Byte(format)
		w.Zero(3)
		w.Uint32(uint32(size / unitSize))
		w.Bytes(data[offset : offset+size])
		w.ZeroTo4ByteAlignment()
		if err := c.sendNewRequest(newUncheckedRequest(w)); err != nil {
			errs.Log(err)
			break
		}
		if onlyOnce {
			break
		}
		mode = PropModeAppend
		offset += size
		remaining -= size
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
		ev := c.WaitEvents(func(e Event) bool {
			if sne, ok := e.(*SelectionNotifyEvent); ok && sne.Requestor == c.helperWindow {
				return true
			}
			return false
		})
		if sne, ok := ev.(*SelectionNotifyEvent); ok && sne.Property != AtomNone {
			filter := func(e Event) bool {
				if pne, valid := e.(*PropertyNotifyEvent); valid && pne.State == PropertyNewValue &&
					pne.Window == sne.Requestor && pne.Atom == sne.Property {
					return true
				}
				return false
			}
			c.PollEvents(filter) // Ensure no existing PropertyNotifyEvent is already pending
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
					c.WaitEvents(filter)
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

// DefaultDepth returns the root depth for the default screen.
func (c *Conn) DefaultDepth() byte {
	return c.Roots[c.DefaultScreen].RootDepth
}

// ContentScale returns the content scale factor for the default screen.
func (c *Conn) ContentScale() (float32, error) {
	if c.cachedScale != 0 {
		return c.cachedScale, nil
	}
	format, actualPropertyType, value, err := c.GetProperty(c.RootWindow(), AtomResourceManager, AtomString, 0, 100_000_000, false)
	if err != nil {
		errs.Log(err)
		c.cachedScale = 1
		return 1, err
	}
	if format == 8 && actualPropertyType == AtomString {
		for _, line := range strings.Split(string(value), "\n") {
			const xftDPI = "Xft.dpi:"
			if strings.HasPrefix(line, xftDPI) {
				var dpi int
				if dpi, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, xftDPI))); err == nil {
					c.cachedScale = float32(dpi) / 96
					return c.cachedScale, nil
				}
			}
		}
	}
	c.cachedScale = 1
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
		base := y * img.Stride * 4
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

// GetKeyboardMapping retrieves the keyboard mapping, which includes the keysyms associated with each keycode in the
// range defined by the connection's minKeyCode and maxKeyCode.
func (c *Conn) GetKeyboardMapping() KeyboardMapping {
	w := NewWriter(8)
	w.Byte(opGetKeyboardMapping)
	w.Zero(1)
	w.Uint16(2)
	w.Byte(c.MinKeyCode)
	w.Byte(c.MaxKeyCode - c.MinKeyCode + 1)
	w.Zero(2)
	var km KeyboardMapping
	if err := c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(1)
		km.KeySymsPerKeyCode = r.Byte()
		r.Skip(30)
		km.KeySyms = r.Uint32Slice(int(c.MaxKeyCode-c.MinKeyCode+1) * int(km.KeySymsPerKeyCode))
	})); err != nil {
		errs.Log(err)
	}
	return km
}

// QueryPointerResult represents the result of a QueryPointer request, containing information about the pointer's
// position and state.
type QueryPointerResult struct {
	Root       WindowID
	Child      WindowID
	RootX      int16
	RootY      int16
	WinX       int16
	WinY       int16
	Mask       uint16
	SameScreen bool
}

// QueryPointer retrieves the current pointer position.
func (c *Conn) QueryPointer(window WindowID) *QueryPointerResult {
	w := NewWriter(8)
	w.Byte(opQueryPointer)
	w.Zero(1)
	w.Uint16(2)
	w.WindowID(window)
	var result QueryPointerResult
	if err := c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(1)
		result.SameScreen = r.Bool()
		r.Skip(6)
		result.Root = r.WindowID()
		result.Child = r.WindowID()
		result.RootX = r.Int16()
		result.RootY = r.Int16()
		result.WinX = r.Int16()
		result.WinY = r.Int16()
		result.Mask = r.Uint16()
		r.Skip(6)
	})); err != nil {
		errs.Log(err)
		return nil
	}
	return &result
}

// CreateColormap creates a new colormap with the specified visual, window, and allocation policy, and returns its
// ColorMapID.
func (c *Conn) CreateColormap(visual VisualID, window WindowID, alloc bool) ColorMapID {
	id := nextXID[ColorMapID](c)
	if id == 0 {
		return 0
	}
	w := NewWriter(16)
	w.Byte(opCreateColormap)
	w.Bool(alloc)
	w.Uint16(4)
	w.ColorMapID(id)
	w.WindowID(window)
	w.VisualID(visual)
	if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
		return 0
	}
	return id
}

// FreeColormap frees the specified colormap.
func (c *Conn) FreeColormap(colormapID ColorMapID) {
	w := NewWriter(8)
	w.Byte(opFreeColormap)
	w.Zero(1)
	w.Uint16(2)
	w.ColorMapID(colormapID)
	if err := c.sendNewRequest(newUncheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// SetSizeHints sets the size hints data on the specified window for the given key atom.
func (c *Conn) SetSizeHints(window WindowID, atom Atom, hints *WindowSizeHints) {
	buf := NewWriter(72)
	buf.Uint32(uint32(hints.Flags))
	buf.Int32(hints.X)
	buf.Int32(hints.Y)
	buf.Uint32(hints.Width)
	buf.Uint32(hints.Height)
	buf.Uint32(hints.MinWidth)
	buf.Uint32(hints.MinHeight)
	buf.Uint32(hints.MaxWidth)
	buf.Uint32(hints.MaxHeight)
	buf.Uint32(hints.WidthInc)
	buf.Uint32(hints.HeightInc)
	buf.Uint32(hints.MinAspectX)
	buf.Uint32(hints.MinAspectY)
	buf.Uint32(hints.MaxAspectX)
	buf.Uint32(hints.MaxAspectY)
	buf.Uint32(hints.BaseWidth)
	buf.Uint32(hints.BaseHeight)
	buf.Uint32(hints.WinGravity)
	c.ChangeProperty(window, atom, AtomWMSizeHints, 32, PropModeReplace, buf.Retrieve())
}

// MapWindow maps the specified window, making it visible on the screen if its parent is also mapped.
func (c *Conn) MapWindow(window WindowID) {
	w := NewWriter(8)
	w.Byte(opMapWindow)
	w.Zero(1)
	w.Uint16(2)
	w.WindowID(window)
	if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// UnmapWindow unmaps the specified window, making it invisible on the screen.
func (c *Conn) UnmapWindow(window WindowID) {
	w := NewWriter(8)
	w.Byte(opUnmapWindow)
	w.Zero(1)
	w.Uint16(2)
	w.WindowID(window)
	if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// TranslateCoordinates translates the specified source coordinates from the coordinate space of the source window to
// that of the destination window. It returns the translated coordinates in the destination window's coordinate space,
// as well as the ID of the child window that contains the translated coordinates, if any.
func (c *Conn) TranslateCoordinates(src, dst WindowID, srcX, srcY int16) (dstX, dstY int16, sameScreen bool, child WindowID, err error) {
	w := NewWriter(16)
	w.Byte(opTranslateCoordinates)
	w.Zero(1)
	w.Uint16(4)
	w.WindowID(src)
	w.WindowID(dst)
	w.Int16(srcX)
	w.Int16(srcY)
	err = c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(1)
		sameScreen = r.Bool()
		r.Skip(6)
		child = r.WindowID()
		dstX = r.Int16()
		dstY = r.Int16()
	}))
	return dstX, dstY, sameScreen, child, err
}

// IsWindowVisible checks if the specified window is currently visible on the screen.
func (c *Conn) IsWindowVisible(window WindowID) bool {
	attr, err := c.GetWindowAttributes(window)
	if err != nil {
		errs.Log(err)
		return false
	}
	return attr.MapState == MapStateViewable
}

// RespondToPing sends a ClientMessage event to the root window in response to a ping request, allowing the window
// manager to determine that the client is still responsive. This is typically used in response to a _NET_WM_PING
// message sent by the window manager to check if the client is alive.
func (c *Conn) RespondToPing() {
	var msg ClientMessageEvent
	msg.Window = c.RootWindow()
	if err := c.sendEvent(msg.Window, false, EventMaskSubstructureNotify|EventMaskSubstructureRedirect, &msg); err != nil {
		errs.Log(err)
	}
}

// QueryKeymap retrieves the current state of the keyboard, returning an array of 32 bytes where each bit represents the
// state of a key (1 for pressed, 0 for released) corresponding to the keycodes defined by the connection's minKeyCode
// and maxKeyCode.
func (c *Conn) QueryKeymap() [32]byte {
	w := NewWriter(4)
	w.Byte(opQueryKeymap)
	w.Zero(1)
	w.Uint16(1)
	var keys [32]byte
	if err := c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(8)
		r.IntoBytes(keys[:])
	})); err != nil {
		errs.Log(err)
	}
	return keys
}

// GetGeometry retrieves the geometry of the specified drawable.
func (c *Conn) GetGeometry(drawable DrawableID) (Geometry, error) {
	w := NewWriter(8)
	w.Byte(opGetGeometry)
	w.Zero(1)
	w.Uint16(2)
	w.DrawableID(drawable)
	var g Geometry
	err := c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(1)
		g.Depth = r.Byte()
		r.Skip(6)
		g.Root = r.WindowID()
		g.X = r.Int16()
		g.Y = r.Int16()
		g.Width = r.Uint16()
		g.Height = r.Uint16()
		g.BorderWidth = r.Uint16()
		r.Skip(10)
	}))
	return g, err
}

// GetWindowBorderWidths retrieves the widths of the borders of the specified window.
func (c *Conn) GetWindowBorderWidths(window WindowID) (top, left, bottom, right uint32) {
	if !c.IsWindowVisible(window) {
		var msg ClientMessageEvent
		msg.Window = window
		msg.Type = c.Atoms.NetRequestFrameExtents
		msg.Format = 32
		if err := c.sendEvent(c.RootWindow(), false, EventMaskSubstructureNotify|EventMaskSubstructureRedirect, &msg); err != nil {
			errs.Log(err)
		} else {
			c.WaitEvents(func(e Event) bool {
				pne, ok := e.(*PropertyNotifyEvent)
				return ok && pne.Window == window && pne.Atom == c.Atoms.NetFrameExtents &&
					pne.State == PropertyNewValue
			})
		}
	}
	format, actualType, value, err := c.GetProperty(window, c.Atoms.NetFrameExtents, AtomCardinal, 0, 32, false)
	if err != nil {
		errs.Log(err)
		return 0, 0, 0, 0
	}
	if format == 32 && actualType == AtomCardinal && len(value) >= 8 {
		r := NewReader(value)
		left = r.Uint32()
		right = r.Uint32()
		top = r.Uint32()
		bottom = r.Uint32()
	}
	return top, left, bottom, right
}

// ChangeWindowAttributes changes the attributes of the specified window based on the provided value mask and
// attributes.
func (c *Conn) ChangeWindowAttributes(window WindowID, mask WindowValueMask, attrs *WindowCreationAttributes) {
	values := attrs.values(mask)
	w := NewWriter(12 + len(values)*4)
	w.Byte(opChangeWindowAttributes)
	w.Zero(1)
	w.Uint16(3 + uint16(len(values)))
	w.WindowID(window)
	w.Uint32(uint32(mask))
	w.Uint32Slice(values)
	if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// GetWindowAttributes retrieves the attributes of the specified window.
func (c *Conn) GetWindowAttributes(window WindowID) (*WindowAttributes, error) {
	w := NewWriter(8)
	w.Byte(opGetWindowAttributes)
	w.Zero(1)
	w.Uint16(2)
	w.WindowID(window)
	var attrs WindowAttributes
	err := c.sendNewRequest(newReplyRequest(w, func(r *Reader) {
		r.Skip(1)
		attrs.BackingStore = r.Byte()
		r.Skip(6)
		attrs.Visual = r.VisualID()
		attrs.Class = r.Uint16()
		attrs.BitGravity = r.Byte()
		attrs.WinGravity = r.Byte()
		attrs.BackingPlanes = r.Uint32()
		attrs.BackingPixel = r.Uint32()
		attrs.SaveUnder = r.Bool()
		attrs.MapIsInstalled = r.Bool()
		attrs.MapState = r.Byte()
		attrs.OverrideRedirect = r.Bool()
		attrs.Colormap = r.ColorMapID()
		attrs.AllEventMasks = r.Uint32()
		attrs.YourEventMask = r.Uint32()
		attrs.DoNotPropagateMask = r.Uint16()
		r.Skip(2)
	}))
	return &attrs, err
}

// StackMode represents the possible stack modes for ConfigureWindow requests.
type StackMode byte

// Possible stack modes for ConfigureWindow requests.
const (
	StackModeAbove StackMode = iota
	StackModeBelow
	StackModeTopIf
	StackModeBottomIf
	StackModeOpposite
)

// ConfigureWindowValueMask holds the possible bitmask values for the ConfigureWindow request.
type ConfigureWindowValueMask uint16

// Possible bitmask values for ConfigureWindow requests.
const (
	ConfigureWindowMaskX ConfigureWindowValueMask = 1 << iota
	ConfigureWindowMaskY
	ConfigureWindowMaskWidth
	ConfigureWindowMaskHeight
	ConfigureWindowMaskBorderWidth
	ConfigureWindowMaskSibling
	ConfigureWindowMaskStackMode
)

// ConfigureWindowRequest represents the values that can be specified in a ConfigureWindow request.
type ConfigureWindowRequest struct {
	Sibling     WindowID
	X           int16
	Y           int16
	Width       uint16
	Height      uint16
	BorderWidth uint16
	StackMode   byte
}

func (c *ConfigureWindowRequest) values() []uint32 {
	values := make([]uint32, 0, 7)
	if c.X != 0 {
		values = append(values, uint32(c.X))
	}
	if c.Y != 0 {
		values = append(values, uint32(c.Y))
	}
	if c.Width != 0 {
		values = append(values, uint32(c.Width))
	}
	if c.Height != 0 {
		values = append(values, uint32(c.Height))
	}
	if c.BorderWidth != 0 {
		values = append(values, uint32(c.BorderWidth))
	}
	if c.Sibling != 0 {
		values = append(values, uint32(c.Sibling))
	}
	if c.StackMode != 0 {
		values = append(values, uint32(c.StackMode))
	}
	return values
}

// ConfigureWindow configures the specified window by changing its position, size, border width, sibling, and/or stack
// mode.
func (c *Conn) ConfigureWindow(window WindowID, mask ConfigureWindowValueMask, req *ConfigureWindowRequest) {
	values := req.values()
	w := NewWriter(12 + len(values)*4)
	w.Byte(opConfigureWindow)
	w.Zero(1)
	w.Uint16(3 + uint16(len(values)))
	w.WindowID(window)
	w.Uint16(uint16(mask))
	w.Zero(2)
	w.Uint32Slice(values)
	if err := c.sendNewRequest(newCheckedRequest(w)); err != nil {
		errs.Log(err)
	}
}

// FocusWindow sets the input focus to the specified window, making it the recipient of keyboard events.
func (c *Conn) FocusWindow(window WindowID) {
	var msg ClientMessageEvent
	msg.Data32[0] = 1
	msg.Window = window
	msg.Type = c.Atoms.NetActiveWindow
	msg.Format = 32
	if err := c.sendEvent(c.RootWindow(), false, EventMaskSubstructureNotify|EventMaskSubstructureRedirect, &msg); err != nil {
		errs.Log(err)
	}
}

// IconifyWindow sends a ClientMessage event to the root window to request that the specified window be iconified
// (minimized).
func (c *Conn) IconifyWindow(window WindowID) {
	var msg ClientMessageEvent
	msg.Data32[0] = StateIconic
	msg.Window = window
	msg.Type = c.Atoms.WMChangeState
	msg.Format = 32
	if err := c.sendEvent(c.RootWindow(), false, EventMaskSubstructureNotify|EventMaskSubstructureRedirect, &msg); err != nil {
		errs.Log(err)
	}
}

// DeiconifyWindow sends a ClientMessage event to the root window to request that the specified window be deiconified
// (restored from a minimized state).
func (c *Conn) DeiconifyWindow(window WindowID) {
	var msg ClientMessageEvent
	msg.Data32[0] = StateNormal
	msg.Window = window
	msg.Type = c.Atoms.WMChangeState
	msg.Format = 32
	if err := c.sendEvent(c.RootWindow(), false, EventMaskSubstructureNotify|EventMaskSubstructureRedirect, &msg); err != nil {
		errs.Log(err)
	}
}

// MaximizeWindow sends a ClientMessage event to the root window to request that the specified window be maximized both
// vertically and horizontally.
func (c *Conn) MaximizeWindow(window WindowID) {
	var msg ClientMessageEvent
	msg.Data32[0] = netWMStateAdd
	msg.Data32[1] = uint32(c.Atoms.NetWMStateMaximizedVert)
	msg.Data32[2] = uint32(c.Atoms.NetWMStateMaximizedHorz)
	msg.Data32[3] = sourceNormalApp
	msg.Window = window
	msg.Type = c.Atoms.NetWMState
	msg.Format = 32
	if err := c.sendEvent(c.RootWindow(), false, EventMaskSubstructureNotify|EventMaskSubstructureRedirect, &msg); err != nil {
		errs.Log(err)
	}
}

// DemaximizeWindow sends a ClientMessage event to the root window to request that the specified window be restored from
// a maximized state.
func (c *Conn) DemaximizeWindow(window WindowID) {
	var msg ClientMessageEvent
	msg.Data32[0] = netWMStateRemove
	msg.Data32[1] = uint32(c.Atoms.NetWMStateMaximizedVert)
	msg.Data32[2] = uint32(c.Atoms.NetWMStateMaximizedHorz)
	msg.Data32[3] = sourceNormalApp
	msg.Window = window
	msg.Type = c.Atoms.NetWMState
	msg.Format = 32
	if err := c.sendEvent(c.RootWindow(), false, EventMaskSubstructureNotify|EventMaskSubstructureRedirect, &msg); err != nil {
		errs.Log(err)
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
			if xreflect.IsNil(c.WaitEvents(func(e Event) bool {
				switch ev := e.(type) {
				case *SelectionNotifyEvent:
					if ev.Requestor == c.helperWindow {
						if ev.Target == c.Atoms.ClipboardSaveTargets {
							again = false
						}
						return true
					}
				case *SelectionRequestEvent:
					if ev.Owner == c.helperWindow {
						if err = c.sendEvent(ev.Requestor, false, 0, &SelectionNotifyEvent{
							Time:      ev.Time,
							Requestor: ev.Requestor,
							Selection: ev.Selection,
							Target:    ev.Target,
							Property:  ev.writeTargetToProperty(c),
						}); err != nil {
							errs.Log(err)
						}
						return true
					}
				case *SelectionClearEvent:
					if ev.Owner == c.helperWindow {
						return true
					}
				}
				return false
			})) {
				again = false
			}
		}
	}
	c.DestroyWindow(c.helperWindow)
	c.helperWindow = 0
}

// Close the connection after finishing any in-flight requests.
func (c *Conn) Close() {
	c.pushClipboardToManager()
	c.Sync()
	close(c.requests)
	<-c.closed
}
