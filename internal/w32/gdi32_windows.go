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
)

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

var (
	gdi32                = syscall.NewLazyDLL("gdi32.dll")
	createBitmapProc     = gdi32.NewProc("CreateBitmap")
	createDIBSectionProc = gdi32.NewProc("CreateDIBSection")
	deleteObjectProc     = gdi32.NewProc("DeleteObject")
)

// CreateBitmap https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-createbitmap
func CreateBitmap(width, height int32, planes, bitsPerPixel uint16, bits []byte) HBITMAP {
	h, _, _ := createBitmapProc.Call(uintptr(width), uintptr(height), uintptr(planes), uintptr(bitsPerPixel), uintptr(unsafe.Pointer(&bits[0])))
	return HBITMAP(h)
}

// CreateDIBSection https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-createdibsection
func CreateDIBSection(hdc HDC, pbmi *BITMAPV5HEADER, iUsage uint, ppvBits **byte, hSection HANDLE, dwOffset uint) HBITMAP {
	ret, _, _ := createDIBSectionProc.Call(uintptr(hdc), uintptr(unsafe.Pointer(pbmi)), uintptr(iUsage), uintptr(unsafe.Pointer(ppvBits)), uintptr(hSection), uintptr(dwOffset))
	return HBITMAP(ret)
}

// DeleteObject https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-deleteobject
func DeleteObject(hObject HGDIOBJ) bool {
	ret, _, _ := deleteObjectProc.Call(uintptr(hObject))
	return ret&0xff != 0
}
