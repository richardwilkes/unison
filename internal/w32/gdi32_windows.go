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
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32                   = windows.NewLazySystemDLL("gdi32.dll")
	createBitmapProc        = gdi32.NewProc("CreateBitmap")
	createDIBSectionProc    = gdi32.NewProc("CreateDIBSection")
	createRectRgnProc       = gdi32.NewProc("CreateRectRgn")
	deleteObjectProc        = gdi32.NewProc("DeleteObject")
	describePixelFormatProc = gdi32.NewProc("DescribePixelFormat")
	setPixelFormatProc      = gdi32.NewProc("SetPixelFormat")
	stretchDIBitsProc       = gdi32.NewProc("StretchDIBits")
	swapBuffersProc         = gdi32.NewProc("SwapBuffers")
)

// SRCCOPY https://learn.microsoft.com/en-us/windows/win32/gdi/ternary-raster-operations
const SRCCOPY = 0x00CC0020

// https://learn.microsoft.com/openspecs/windows_protocols/ms-emf/a5e722e3-891a-4a67-be1a-ed5a48a7fda1
const (
	DIB_RGB_COLORS  = 0
	DIB_PAL_COLORS  = 1
	DIB_PAL_INDICES = 2
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

// BITMAPINFOHEADER https://learn.microsoft.com/en-us/windows/win32/api/wingdi/ns-wingdi-bitmapinfoheader
type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
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

// CreateBitmap https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-createbitmap
func CreateBitmap(width, height int32, planes, bitsPerPixel uint16, bits []byte) HBITMAP {
	// The uintptr(unsafe.Pointer(...)) conversion must be written directly in the Call argument list; hoisting it
	// into a local would end bits' liveness before the call, letting the GC collect the backing array mid-call (see
	// doc.go), hence the duplicated call.
	if len(bits) == 0 {
		//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
		h, _, _ := createBitmapProc.Call(uintptr(width), uintptr(height), uintptr(planes), uintptr(bitsPerPixel), 0)
		return HBITMAP(h)
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	h, _, _ := createBitmapProc.Call(uintptr(width), uintptr(height), uintptr(planes), uintptr(bitsPerPixel),
		uintptr(unsafe.Pointer(&bits[0])))
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

// StretchDIBits https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-stretchdibits. bits holds the
// pixels as 32-bit BGRA words; bmi's header fields must describe them (a BITMAPINFO with no color table is just its
// BITMAPINFOHEADER, so the header pointer is passed directly).
func StretchDIBits(hdc HDC, xDest, yDest, destWidth, destHeight, xSrc, ySrc, srcWidth, srcHeight int32, bits []uint32, bmi *BITMAPINFOHEADER, iUsage, rop uint32) int32 {
	bmi.BiSize = uint32(unsafe.Sizeof(*bmi))
	// The uintptr(unsafe.Pointer(...)) conversion must be written directly in the Call argument list; hoisting it
	// into a local would end bits' liveness before the call, letting the GC collect the backing array mid-call (see
	// doc.go), hence the duplicated call.
	if len(bits) == 0 {
		//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
		ret, _, _ := stretchDIBitsProc.Call(uintptr(hdc), uintptr(xDest), uintptr(yDest), uintptr(destWidth),
			uintptr(destHeight), uintptr(xSrc), uintptr(ySrc), uintptr(srcWidth), uintptr(srcHeight), 0,
			uintptr(unsafe.Pointer(bmi)), uintptr(iUsage), uintptr(rop))
		return int32(ret)
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := stretchDIBitsProc.Call(uintptr(hdc), uintptr(xDest), uintptr(yDest), uintptr(destWidth),
		uintptr(destHeight), uintptr(xSrc), uintptr(ySrc), uintptr(srcWidth), uintptr(srcHeight),
		uintptr(unsafe.Pointer(&bits[0])), uintptr(unsafe.Pointer(bmi)), uintptr(iUsage), uintptr(rop))
	return int32(ret)
}

// SwapBuffers https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-swapbuffers
func SwapBuffers(hdc HDC) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := swapBuffersProc.Call(uintptr(hdc))
	return ret&0xff != 0
}
