// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32                   = syscall.NewLazyDLL("gdi32.dll")
	createBitmapProc        = gdi32.NewProc("CreateBitmap")
	createDIBSectionProc    = gdi32.NewProc("CreateDIBSection")
	createRectRgnProc       = gdi32.NewProc("CreateRectRgn")
	deleteObjectProc        = gdi32.NewProc("DeleteObject")
	describePixelFormatProc = gdi32.NewProc("DescribePixelFormat")
	setPixelFormatProc      = gdi32.NewProc("SetPixelFormat")
	swapBuffersProc         = gdi32.NewProc("SwapBuffers")
)

type HRGN uintptr

// https://learn.microsoft.com/openspecs/windows_protocols/ms-emf/a5e722e3-891a-4a67-be1a-ed5a48a7fda1
const (
	DIB_RGB_COLORS  = 0
	DIB_PAL_COLORS  = 1
	DIB_PAL_INDICES = 2
)

// https://learn.microsoft.com/en-us/windows/win32/api/wingdi/ns-wingdi-pixelformatdescriptor
const (
	PFD_TYPE_RGBA             = 0
	PFD_TYPE_COLORINDEX       = 1
	PFD_DOUBLEBUFFER          = 0x00000001
	PFD_STEREO                = 0x00000002
	PFD_DRAW_TO_WINDOW        = 0x00000004
	PFD_DRAW_TO_BITMAP        = 0x00000008
	PFD_SUPPORT_GDI           = 0x00000010
	PFD_SUPPORT_OPENGL        = 0x00000020
	PFD_GENERIC_FORMAT        = 0x00000040
	PFD_NEED_PALETTE          = 0x00000080
	PFD_NEED_SYSTEM_PALETTE   = 0x00000100
	PFD_SWAP_EXCHANGE         = 0x00000200
	PFD_SWAP_COPY             = 0x00000400
	PFD_SWAP_LAYER_BUFFERS    = 0x00000800
	PFD_GENERIC_ACCELERATED   = 0x00001000
	PFD_DEPTH_DONTCARE        = 0x20000000
	PFD_DOUBLEBUFFER_DONTCARE = 0x40000000
	PFD_STEREO_DONTCARE       = 0x80000000
)

type FXPT2DOT30 int32

type CIEXYZ struct {
	CiexyzX FXPT2DOT30
	CiexyzY FXPT2DOT30
	CiexyzZ FXPT2DOT30
}

type CIEXYZTRIPLE struct {
	CiexyzRed   CIEXYZ
	CiexyzGreen CIEXYZ
	CiexyzBlue  CIEXYZ
}

type BITMAPV5HEADER struct {
	BV5Size          uint32
	BV5Width         int32
	BV5Height        int32
	BV5Planes        uint16
	BV5BitCount      uint16
	BV5Compression   uint32
	BV5SizeImage     uint32
	BV5XPelsPerMeter int32
	BV5YPelsPerMeter int32
	BV5ClrUsed       uint32
	BV5ClrImportant  uint32
	BV5RedMask       uint32
	BV5GreenMask     uint32
	BV5BlueMask      uint32
	BV5AlphaMask     uint32
	BV5CSType        uint32
	BV5Endpoints     CIEXYZTRIPLE
	BV5GammaRed      uint32
	BV5GammaGreen    uint32
	BV5GammaBlue     uint32
	BV5Intent        uint32
	BV5ProfileData   uint32
	BV5ProfileSize   uint32
	BV5Reserved      uint32
}

// PIXELFORMATDESCRIPTOR http://msdn.microsoft.com/en-us/library/windows/desktop/dd368826.aspx
type PIXELFORMATDESCRIPTOR struct {
	Size                   uint16
	Version                uint16
	DwFlags                uint32
	IPixelType             byte
	ColorBits              byte
	RedBits, RedShift      byte
	GreenBits, GreenShift  byte
	BlueBits, BlueShift    byte
	AlphaBits, AlphaShift  byte
	AccumBits              byte
	AccumRedBits           byte
	AccumGreenBits         byte
	AccumBlueBits          byte
	AccumAlphaBits         byte
	DepthBits, StencilBits byte
	AuxBuffers             byte
	ILayerType             byte
	Reserved               byte
	DwLayerMask            uint32
	DwVisibleMask          uint32
	DwDamageMask           uint32
}

// CreateBitmap https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-createbitmap
func CreateBitmap(width, height int32, planes, bitsPerPixel uint16, bits []byte) HBITMAP {
	var bitsPtr uintptr
	if len(bits) != 0 {
		bitsPtr = uintptr(unsafe.Pointer(&bits[0]))
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	h, _, _ := createBitmapProc.Call(uintptr(width), uintptr(height), uintptr(planes), uintptr(bitsPerPixel), bitsPtr)
	return HBITMAP(h)
}

// CreateDIBSection https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-createdibsection
func CreateDIBSection(hdc HDC, pbmi *BITMAPV5HEADER, iUsage uint, ppvBits **byte, hSection windows.Handle, dwOffset uint) HBITMAP {
	pbmi.BV5Size = uint32(unsafe.Sizeof(*pbmi))
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := createDIBSectionProc.Call(uintptr(hdc), uintptr(unsafe.Pointer(pbmi)), uintptr(iUsage), uintptr(unsafe.Pointer(ppvBits)), uintptr(hSection), uintptr(dwOffset))
	return HBITMAP(ret)
}

// CreateRectRgn https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-createrectrgn
func CreateRectRgn(left, top, right, bottom int32) HRGN {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := createRectRgnProc.Call(uintptr(left), uintptr(top), uintptr(right), uintptr(bottom))
	return HRGN(ret)
}

// DeleteObject https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-deleteobject
func DeleteObject(hObject HGDIOBJ) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := deleteObjectProc.Call(uintptr(hObject))
	return ret&0xff != 0
}

// DescribePixelFormat https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-describepixelformat
func DescribePixelFormat(hdc HDC, iPixelFormat int32, nBytes uint32, ppfd *PIXELFORMATDESCRIPTOR) int32 {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := describePixelFormatProc.Call(uintptr(hdc), uintptr(iPixelFormat), uintptr(nBytes),
		uintptr(unsafe.Pointer(ppfd)))
	return int32(ret)
}

// SetPixelFormat https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-setpixelformat
func SetPixelFormat(hdc HDC, iPixelFormat int32, pfd *PIXELFORMATDESCRIPTOR) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := setPixelFormatProc.Call(uintptr(hdc), uintptr(iPixelFormat), uintptr(unsafe.Pointer(pfd)))
	return ret&0xff != 0
}

// SwapBuffers https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-swapbuffers
func SwapBuffers(hdc HDC) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := swapBuffersProc.Call(uintptr(hdc))
	return ret&0xff != 0
}
