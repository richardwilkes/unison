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
	ntdll                    = windows.NewLazySystemDLL("ntdll.dll")
	rtlVerifyVersionInfoProc = ntdll.NewProc("RtlVerifyVersionInfo")
)

const (
	VER_BUILDNUMBER      = 0x00000004
	VER_GREATER_EQUAL    = 3
	VER_MAJORVERSION     = 0x00000002
	VER_MINORVERSION     = 0x00000001
	VER_SERVICEPACKMAJOR = 0x00000020
	WIN32_WINNT_VISTA    = 0x0600
	WIN32_WINNT_WIN7     = 0x0601
	WIN32_WINNT_WIN8     = 0x0602
	WIN32_WINNT_WINBLUE  = 0x0603
	WIN32_WINNT_WINXP    = 0x0501
)

const (
	Windows10AnniversaryUpdateBuild = 14393
	Windows10CreatorsUpdateBuild    = 15063
)

type OSVERSIONINFOEXW struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformId        uint32
	CSDVersion        [128]uint16
	ServicePackMajor  uint16
	ServicePackMinor  uint16
	SuiteMask         uint16
	ProductType       byte
	Reserved          byte
}

// RtlVerifyVersionInfo https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/wdm/nf-wdm-rtlverifyversioninfo
func RtlVerifyVersionInfo(info *OSVERSIONINFOEXW, typeMask uint32, conditionMask uint64) int32 {
	info.OSVersionInfoSize = uint32(unsafe.Sizeof(*info))
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := rtlVerifyVersionInfoProc.Call(uintptr(unsafe.Pointer(info)), uintptr(typeMask), uintptr(conditionMask))
	return int32(ret)
}
