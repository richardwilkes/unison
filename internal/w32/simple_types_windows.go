// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

// Simple types https://docs.microsoft.com/en-us/windows/desktop/WinProg/windows-data-types
type (
	ATOM            uintptr
	ClipboardFormat uint
	HBITMAP         uintptr
	HBRUSH          uintptr
	HCURSOR         uintptr
	HDC             uintptr
	HGDIOBJ         uintptr
	HGLRC           uintptr
	HICON           uintptr
	HINSTANCE       uintptr
	HKEY            uintptr
	HMENU           uintptr
	HMODULE         uintptr
	HMONITOR        uintptr
	LPARAM          uintptr
	LPVOID          uintptr
	LRESULT         uintptr
	UTF16String     *uint16
	WPARAM          uintptr
)
