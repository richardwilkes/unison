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
	"testing"

	"github.com/richardwilkes/canvas/imagecore"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestImageBackedByRaster guards the invariant DrawImageNine relies on: an Image's underlying image is always a raster
// *imagecore.Image. If that ever changes, asRaster(img.image) would stop being a plain type assertion and start forcing
// a GPU->CPU readback (or fail) — the regression fixed in canvas.go's DrawImageNine.
func TestImageBackedByRaster(t *testing.T) {
	c := check.New(t)

	const w, h = 4, 3
	img, err := NewImageFromPixels(w, h, make([]byte, w*h*4), geom.NewPoint(1, 1))
	c.NoError(err)
	c.NotNil(img)

	raster, ok := img.image.(*imagecore.Image)
	c.True(ok, "img.image should always be a raster *imagecore.Image")

	// asRaster on the base image must be a pure type assertion: it returns the identical pointer, never allocating a
	// new CPU image via MakeNonTextureImage.
	c.Equal(raster, asRaster(img.image))
}

// distinctPixels returns width*height*4 bytes of deterministic, non-repeating data so each test gets its own cache hash
// and does not collide with images created by other tests in this shared-cache package.
func distinctPixels(width, height, seed int) []byte {
	pixels := make([]byte, width*height*4)
	for i := range pixels {
		pixels[i] = byte(i*31 + seed)
	}
	return pixels
}

// TestImageDisposeEvictsFromCache exercises the simplified Dispose: it must evict the image from imgCache, drop the
// underlying references, and be safely idempotent — all without the removed per-Image cleanup registrations.
func TestImageDisposeEvictsFromCache(t *testing.T) {
	c := check.New(t)

	const w, h = 2, 2
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 3), geom.NewPoint(1, 1))
	c.NoError(err)
	c.NotNil(img)

	hash := img.hash
	imgCacheLock.Lock()
	_, cached := imgCache[hash]
	imgCacheLock.Unlock()
	c.True(cached, "a freshly created image should be present in the cache")

	img.Dispose()

	imgCacheLock.Lock()
	_, cached = imgCache[hash]
	imgCacheLock.Unlock()
	c.False(cached, "Dispose should evict the image from the cache")
	c.Nil(img.image, "Dispose should drop the underlying image reference")
	c.Nil(img.nonTextureImage, "Dispose should drop the non-texture image reference")

	// Dispose is guarded by disposeOnce, so a second call must be a safe no-op.
	c.NotPanics(func() { img.Dispose() })
}

// TestImageDeduplication verifies that two images built from identical data resolve to the same cached *Image, the
// path where newImage now simply returns the existing entry (the removed imgUnref of the duplicate was a no-op).
func TestImageDeduplication(t *testing.T) {
	c := check.New(t)

	const w, h = 3, 2
	pixels := distinctPixels(w, h, 11)
	img1, err := NewImageFromPixels(w, h, pixels, geom.NewPoint(1, 1))
	c.NoError(err)
	img2, err := NewImageFromPixels(w, h, append([]byte(nil), pixels...), geom.NewPoint(1, 1))
	c.NoError(err)
	c.True(img1 == img2, "identical pixel data should resolve to the same cached *Image")

	img1.Dispose()
}

// TestImageForCanvasReturnsRasterForNilContext covers the non-texture branch of imageForCanvas: with no GL context it
// must return a cached CPU raster image (MakeNonTextureImage is the identity for a raster image) without registering
// any of the removed cleanups.
func TestImageForCanvasReturnsRasterForNilContext(t *testing.T) {
	c := check.New(t)

	const w, h = 2, 2
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 23), geom.NewPoint(1, 1))
	c.NoError(err)

	got := img.imageForCanvas(nil)
	c.NotNil(got)
	_, ok := got.(*imagecore.Image)
	c.True(ok, "imageForCanvas(nil) should return a raster CPU image")

	// The result is cached on the Image, so a second call returns the identical value.
	c.Equal(got, img.imageForCanvas(nil))

	img.Dispose()
}
