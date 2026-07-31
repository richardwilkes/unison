// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package w32 provides the Win32 and COM bindings used by unison on Windows.
//
// Pointer lifetimes: every native call in this package goes through either syscall.SyscallN or
// golang.org/x/sys/windows (*LazyProc).Call / (*Proc).Call, all of which are marked //go:uintptrescapes. That pragma
// extends the unsafe-pointer keep-alive exemption to these calls: a uintptr(unsafe.Pointer(p)) conversion written
// directly in the call's argument list keeps p (and anything reachable from it) alive and unmoved for the duration of
// the call. Consequently, runtime.KeepAlive is neither needed nor used here — a hygiene test enforces its absence,
// since a stray KeepAlive suggests the conversion was hoisted out of the call expression, which is the one pattern
// that genuinely is unsafe. Pointers that must outlive a call (e.g. COM objects handed to the OS) are managed with
// runtime.Pinner and reference counting instead.
package w32
