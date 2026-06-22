// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import "golang.org/x/sys/windows"

// Simple types https://docs.microsoft.com/en-us/windows/desktop/WinProg/windows-data-types
type (
	ATOM            uintptr
	ClipboardFormat uint
	HBITMAP         windows.Handle
	HBRUSH          windows.Handle
	HCURSOR         windows.Handle
	HDC             windows.Handle
	HDROP           windows.Handle
	HGDIOBJ         windows.Handle
	HGLRC           windows.Handle
	HHOOK           windows.Handle
	HICON           windows.Handle
	HINSTANCE       windows.Handle
	HKEY            windows.Handle
	HMENU           windows.Handle
	HMODULE         windows.Handle
	HMONITOR        windows.Handle
	HRGN            windows.Handle
	LPARAM          uintptr
	LPVOID          uintptr
	LRESULT         uintptr
	UTF16String     *uint16
	WPARAM          uintptr
)
