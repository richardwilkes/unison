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

	"github.com/richardwilkes/toolbox/v2/errs"
	"golang.org/x/sys/windows"
)

var (
	// opengl32.dll is NOT on the KnownDLLs list, so it must be loaded with NewLazySystemDLL to force resolution from
	// the system directory; syscall.NewLazyDLL would search the application directory first, allowing a planted DLL
	// next to the executable to hijack the process at the first GL call.
	opengl32              = windows.NewLazySystemDLL("opengl32.dll")
	wglCreateContextProc  = opengl32.NewProc("wglCreateContext")
	wglDeleteContextProc  = opengl32.NewProc("wglDeleteContext")
	wglGetProcAddressProc = opengl32.NewProc("wglGetProcAddress")
	wglMakeCurrentProc    = opengl32.NewProc("wglMakeCurrent")
)

var (
	wglCreateContextAttribsARBProc   uintptr
	wglChoosePixelFormatARBProc      uintptr
	wglGetPixelFormatAttribivARBProc uintptr
)

const (
	WGL_ACCUM_BITS_ARB                          = 0x201D
	WGL_ACCELERATION_ARB                        = 0x2003
	WGL_ACCUM_ALPHA_BITS_ARB                    = 0x2021
	WGL_ACCUM_BLUE_BITS_ARB                     = 0x2020
	WGL_ACCUM_GREEN_BITS_ARB                    = 0x201F
	WGL_ACCUM_RED_BITS_ARB                      = 0x201E
	WGL_AUX_BUFFERS_ARB                         = 0x2024
	WGL_ALPHA_BITS_ARB                          = 0x201B
	WGL_ALPHA_SHIFT_ARB                         = 0x201C
	WGL_BLUE_BITS_ARB                           = 0x2019
	WGL_BLUE_SHIFT_ARB                          = 0x201A
	WGL_COLOR_BITS_ARB                          = 0x2014
	WGL_COLORSPACE_EXT                          = 0x309D
	WGL_COLORSPACE_SRGB_EXT                     = 0x3089
	WGL_CONTEXT_COMPATIBILITY_PROFILE_BIT_ARB   = 0x00000002
	WGL_CONTEXT_CORE_PROFILE_BIT_ARB            = 0x00000001
	WGL_CONTEXT_DEBUG_BIT_ARB                   = 0x0001
	WGL_CONTEXT_ES2_PROFILE_BIT_EXT             = 0x00000004
	WGL_CONTEXT_FLAGS_ARB                       = 0x2094
	WGL_CONTEXT_FORWARD_COMPATIBLE_BIT_ARB      = 0x0002
	WGL_CONTEXT_MAJOR_VERSION_ARB               = 0x2091
	WGL_CONTEXT_MINOR_VERSION_ARB               = 0x2092
	WGL_CONTEXT_OPENGL_NO_ERROR_ARB             = 0x31B3
	WGL_CONTEXT_PROFILE_MASK_ARB                = 0x9126
	WGL_CONTEXT_RELEASE_BEHAVIOR_ARB            = 0x2097
	WGL_CONTEXT_RELEASE_BEHAVIOR_NONE_ARB       = 0x0000
	WGL_CONTEXT_RELEASE_BEHAVIOR_FLUSH_ARB      = 0x2098
	WGL_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB = 0x8256
	WGL_CONTEXT_ROBUST_ACCESS_BIT_ARB           = 0x00000004
	WGL_DEPTH_BITS_ARB                          = 0x2022
	WGL_DRAW_TO_BITMAP_ARB                      = 0x2002
	WGL_DRAW_TO_WINDOW_ARB                      = 0x2001
	WGL_DOUBLE_BUFFER_ARB                       = 0x2011
	WGL_FRAMEBUFFER_SRGB_CAPABLE_ARB            = 0x20A9
	WGL_FULL_ACCELERATION_ARB                   = 0x2027
	WGL_GREEN_BITS_ARB                          = 0x2017
	WGL_GREEN_SHIFT_ARB                         = 0x2018
	WGL_LOSE_CONTEXT_ON_RESET_ARB               = 0x8252
	WGL_NEED_PALETTE_ARB                        = 0x2004
	WGL_NEED_SYSTEM_PALETTE_ARB                 = 0x2005
	WGL_NO_ACCELERATION_ARB                     = 0x2025
	WGL_NO_RESET_NOTIFICATION_ARB               = 0x8261
	WGL_NUMBER_OVERLAYS_ARB                     = 0x2008
	WGL_NUMBER_PIXEL_FORMATS_ARB                = 0x2000
	WGL_NUMBER_UNDERLAYS_ARB                    = 0x2009
	WGL_PIXEL_TYPE_ARB                          = 0x2013
	WGL_RED_BITS_ARB                            = 0x2015
	WGL_RED_SHIFT_ARB                           = 0x2016
	WGL_SAMPLES_ARB                             = 0x2042
	WGL_SHARE_ACCUM_ARB                         = 0x200E
	WGL_SHARE_DEPTH_ARB                         = 0x200C
	WGL_SHARE_STENCIL_ARB                       = 0x200D
	WGL_STENCIL_BITS_ARB                        = 0x2023
	WGL_STEREO_ARB                              = 0x2012
	WGL_SUPPORT_GDI_ARB                         = 0x200F
	WGL_SUPPORT_OPENGL_ARB                      = 0x2010
	WGL_SWAP_LAYER_BUFFERS_ARB                  = 0x2006
	WGL_SWAP_METHOD_ARB                         = 0x2007
	WGL_TRANSPARENT_ARB                         = 0x200A
	WGL_TRANSPARENT_ALPHA_VALUE_ARB             = 0x203A
	WGL_TRANSPARENT_BLUE_VALUE_ARB              = 0x2039
	WGL_TRANSPARENT_GREEN_VALUE_ARB             = 0x2038
	WGL_TRANSPARENT_INDEX_VALUE_ARB             = 0x203B
	WGL_TRANSPARENT_RED_VALUE_ARB               = 0x2037
	WGL_TYPE_RGBA_ARB                           = 0x202B
)

// Error codes defined by WGL_ARB_create_context, which FormatMessage knows nothing about.
const (
	ERROR_INVALID_VERSION_ARB = 0x2095
	ERROR_INVALID_PROFILE_ARB = 0x2096
)

// wglErrorNames supplies text for the WGL extension error codes when reporting a failed call.
var wglErrorNames = map[syscall.Errno]string{
	ERROR_INVALID_VERSION_ARB: "ERROR_INVALID_VERSION_ARB",
	ERROR_INVALID_PROFILE_ARB: "ERROR_INVALID_PROFILE_ARB",
}

// WglCreateContext creates a legacy OpenGL rendering context for a device context that has already had its pixel
// format set. A failure is reported along with the Windows last-error code, which is the only clue the driver gives as
// to why it refused.
func WglCreateContext(dc HDC) (HGLRC, error) {
	ret, _, e := wglCreateContextProc.Call(uintptr(dc))
	if ret == 0 {
		return 0, lastError("wglCreateContext", e, wglErrorNames)
	}
	return HGLRC(ret), nil
}

// WglCreateContextAttribsARB creates a context via the WGL_ARB_create_context extension. The extension may be
// unavailable (e.g. under RDP or basic display adapters), which is reported as an error like any other failure.
func WglCreateContextAttribsARB(dc HDC, shareCtx HGLRC, attribList []int32) (HGLRC, error) {
	if wglCreateContextAttribsARBProc == 0 {
		if wglCreateContextAttribsARBProc = WglGetProcAddress("wglCreateContextAttribsARB"); wglCreateContextAttribsARBProc == 0 {
			return 0, errs.New("wglCreateContextAttribsARB is not available")
		}
	}
	r, _, e := syscall.SyscallN(wglCreateContextAttribsARBProc, uintptr(dc), uintptr(shareCtx),
		uintptr(unsafe.Pointer(&attribList[0])))
	if r == 0 {
		return 0, lastError("wglCreateContextAttribsARB", e, wglErrorNames)
	}
	return HGLRC(r), nil
}

// WglChoosePixelFormatARB https://registry.khronos.org/OpenGL/extensions/ARB/WGL_ARB_pixel_format.txt
//
// Returns up to maxFormats pixel formats that the driver itself considers to satisfy the attributes, best first, which
// may be none. Unlike scanning DescribePixelFormat, this reflects attributes a PIXELFORMATDESCRIPTOR cannot express
// and only ever yields formats the driver is prepared to create a context on. A context must be current on the calling
// thread. A driver that lacks the extension is reported as an error like any other failure.
func WglChoosePixelFormatARB(hdc HDC, attribList []int32, maxFormats int) ([]int32, error) {
	if wglChoosePixelFormatARBProc == 0 {
		if wglChoosePixelFormatARBProc = WglGetProcAddress("wglChoosePixelFormatARB"); wglChoosePixelFormatARBProc == 0 {
			return nil, errs.New("wglChoosePixelFormatARB is not available")
		}
	}
	formats := make([]int32, maxFormats)
	floatAttribs := []float32{0, 0} // No float attributes, but some drivers reject a nil list
	var count uint32
	r, _, e := syscall.SyscallN(wglChoosePixelFormatARBProc, uintptr(hdc), uintptr(unsafe.Pointer(&attribList[0])),
		uintptr(unsafe.Pointer(&floatAttribs[0])), uintptr(maxFormats), uintptr(unsafe.Pointer(&formats[0])),
		uintptr(unsafe.Pointer(&count)))
	if r&0xff == 0 {
		return nil, lastError("wglChoosePixelFormatARB", e, wglErrorNames)
	}
	return formats[:min(int(count), maxFormats)], nil
}

// WglGetPixelFormatAttribivARB https://registry.khronos.org/OpenGL/extensions/ARB/WGL_ARB_pixel_format.txt
func WglGetPixelFormatAttribivARB(hdc HDC, iPixelFormat, iLayerPlane int32, piAttributes []int32) []int32 {
	if wglGetPixelFormatAttribivARBProc == 0 {
		if wglGetPixelFormatAttribivARBProc = WglGetProcAddress("wglGetPixelFormatAttribivARB"); wglGetPixelFormatAttribivARBProc == 0 {
			return nil
		}
	}
	values := make([]int32, len(piAttributes))
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := syscall.SyscallN(wglGetPixelFormatAttribivARBProc, uintptr(hdc), uintptr(iPixelFormat),
		uintptr(iLayerPlane), uintptr(len(piAttributes)), uintptr(unsafe.Pointer(&piAttributes[0])),
		uintptr(unsafe.Pointer(&values[0])))
	if r&0xff == 0 {
		return nil
	}
	return values
}

// WglGetProcAddress https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-wglgetprocaddress
//
// Returns 0 when the procedure is unavailable so callers can probe for optional extensions and degrade gracefully
// rather than terminating the process. Some implementations return the sentinel values 1, 2, 3, or -1 on failure
// instead of NULL, so those are mapped to 0 as well.
func WglGetProcAddress(name string) uintptr {
	ptr, err := windows.BytePtrFromString(name)
	if err != nil {
		return 0
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := wglGetProcAddressProc.Call(uintptr(unsafe.Pointer(ptr)))
	if !wglProcAddressValid(r) {
		return 0
	}
	return r
}

// WglDeleteContext https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-wgldeletecontext
func WglDeleteContext(hglrc HGLRC) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := wglDeleteContextProc.Call(uintptr(hglrc))
	return ret&0xff != 0
}

// WglMakeCurrent https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-wglmakecurrent
// WglMakeCurrent makes the context current on the calling thread, or releases the current context when both
// arguments are 0. A failure is reported along with the Windows last-error code.
func WglMakeCurrent(hdc HDC, hglrc HGLRC) error {
	ret, _, e := wglMakeCurrentProc.Call(uintptr(hdc), uintptr(hglrc))
	if ret&0xff == 0 {
		return lastError("wglMakeCurrent", e, wglErrorNames)
	}
	return nil
}
