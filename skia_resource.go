// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"runtime"
	"sync"
)

// newSkiaCleanup registers a runtime cleanup that releases the native skia handle on the UI thread once owner becomes
// unreachable. The returned Cleanup should be stored on owner so that Dispose can stop it when releasing early. Used by
// the various reference-counted skia resource wrappers (ColorFilter, ImageFilter, MaskFilter, Paint, Path, PathEffect,
// Shader, TextBlob, Image).
func newSkiaCleanup[O, T any](owner *O, handle T, unref func(T)) runtime.Cleanup {
	return runtime.AddCleanup(owner, func(h T) { ReleaseOnUIThread(func() { unref(h) }) }, handle)
}

// disposeSkiaHandle runs once: it stops the runtime cleanup, then releases the native handle and zeroes it if it hasn't
// already been released. It is the shared body of the various skia resource wrappers' Dispose methods.
func disposeSkiaHandle[T comparable](once *sync.Once, cleanup runtime.Cleanup, handle *T, unref func(T)) {
	once.Do(func() {
		cleanup.Stop()
		var zero T
		if *handle != zero {
			unref(*handle)
			*handle = zero
		}
	})
}
