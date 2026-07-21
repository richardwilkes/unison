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
	"bytes"
	"image"
	"image/color"
	"image/png"
	"runtime"
	"testing"
	"time"

	"github.com/richardwilkes/canvas/codecs"
	"github.com/richardwilkes/canvas/gpu/gl"
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

// stubTexture is a genericImage stand-in for the *gl.TextureImage entries imageCtxMap holds in production, recording
// whether its GPU-side release was invoked so tests can observe eviction without a live GL context.
type stubTexture struct {
	released bool
}

func (s *stubTexture) Width() int32                          { return 1 }
func (s *stubTexture) Height() int32                         { return 1 }
func (s *stubTexture) AlphaType() imagecore.AlphaType        { return imagecore.AlphaTypeUnpremul }
func (s *stubTexture) IsAlphaOnly() bool                     { return false }
func (s *stubTexture) UniqueID() uint32                      { return 0 }
func (s *stubTexture) MakeNonTextureImage() *imagecore.Image { return nil }
func (s *stubTexture) ReadPixels(_ imagecore.ImageInfo, _ []byte, _ int, _, _ int32, _ imagecore.CachingHint) bool {
	return false
}
func (s *stubTexture) Release() { s.released = true }

// drainTasks runs any queued UI tasks on the current goroutine, which stands in for the UI thread in these headless
// tests.
func drainTasks() {
	for {
		taskQueueLock.Lock()
		pending := len(taskQueue) - taskQueueHead
		taskQueueLock.Unlock()
		if pending == 0 {
			return
		}
		processNextTask()
	}
}

// TestImageDisposeReleasesTextures verifies that Dispose evicts the image's cached GPU textures from every context in
// imageCtxMap and releases them, while leaving other images' entries alone. Before this, only whole-context teardown
// (releaseImagesForContext) ever removed per-image texture entries, so long-lived windows accumulated textures without
// bound. The eviction must happen via a task on the UI thread, not on Dispose's caller's goroutine, since imageCtxMap
// is UI-thread-only state.
func TestImageDisposeReleasesTextures(t *testing.T) {
	c := check.New(t)

	const w, h = 2, 2
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 41), geom.NewPoint(1, 1))
	c.NoError(err)

	ctx1 := new(gl.DirectContext)
	ctx2 := new(gl.DirectContext)
	defer func() {
		delete(imageCtxMap, ctx1)
		delete(imageCtxMap, ctx2)
	}()
	tex1 := &stubTexture{}
	tex2 := &stubTexture{}
	bystander := &stubTexture{}
	imageCtxMap[ctx1] = map[uint64]genericImage{img.hash: tex1, img.hash + 1: bystander}
	imageCtxMap[ctx2] = map[uint64]genericImage{img.hash: tex2}

	img.Dispose()

	// Dispose only queues the release; nothing may touch imageCtxMap until the task runs on the UI thread.
	_, present := imageCtxMap[ctx1][img.hash]
	c.True(present, "Dispose must not touch imageCtxMap before the queued task runs")
	c.False(tex1.released, "Dispose must not release textures before the queued task runs")

	drainTasks()

	_, present = imageCtxMap[ctx1][img.hash]
	c.False(present, "Dispose should evict the image's texture from the first context")
	_, present = imageCtxMap[ctx2][img.hash]
	c.False(present, "Dispose should evict the image's texture from the second context")
	c.True(tex1.released, "Dispose should release the first context's texture")
	c.True(tex2.released, "Dispose should release the second context's texture")

	_, present = imageCtxMap[ctx1][img.hash+1]
	c.True(present, "Dispose should leave other images' textures alone")
	c.False(bystander.released, "Dispose should not release other images' textures")
}

// TestImageDisposeOffUIThreadDefersTextureRelease covers the scenario from issue 13: an app disposing images from a
// background loader goroutine. Dispose must not touch imageCtxMap (UI-thread-only, unlocked) or perform GL work on the
// caller's goroutine; it must instead queue the release through InvokeTask, exactly like the GC cleanup path. The
// caller's goroutine here stands in for the background loader, and the test goroutine drains the task queue as the UI
// thread would.
func TestImageDisposeOffUIThreadDefersTextureRelease(t *testing.T) {
	c := check.New(t)

	const w, h = 2, 2
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 59), geom.NewPoint(1, 1))
	c.NoError(err)

	hash := img.hash
	ctx := new(gl.DirectContext)
	defer delete(imageCtxMap, ctx)
	tex := &stubTexture{}
	imageCtxMap[ctx] = map[uint64]genericImage{hash: tex}

	done := make(chan struct{})
	go func() {
		defer close(done)
		img.Dispose()
	}()
	<-done

	// The background goroutine's Dispose has fully returned, yet the map and texture must be untouched: the release
	// runs only when the UI thread services the task queue.
	_, present := imageCtxMap[ctx][hash]
	c.True(present, "Dispose from a background goroutine must not mutate imageCtxMap directly")
	c.False(tex.released, "Dispose from a background goroutine must not release the texture directly")
	c.Nil(img.image, "Dispose should still drop the underlying image reference synchronously")

	drainTasks()

	_, present = imageCtxMap[ctx][hash]
	c.False(present, "the queued task should evict the texture on the UI thread")
	c.True(tex.released, "the queued task should release the texture on the UI thread")
}

// TestImageGCReleasesTextures verifies the garbage-collection path: once a texture has been uploaded for an image (and
// registerTextureCleanup has run), dropping the last reference to the Image must eventually evict and release its
// cached textures via a task on the UI thread. This restores the eviction role of the per-image runtime.Cleanup
// registrations that existed before the canvas port.
func TestImageGCReleasesTextures(t *testing.T) {
	c := check.New(t)

	const w, h = 2, 2
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 43), geom.NewPoint(1, 1))
	c.NoError(err)

	hash := img.hash
	ctx := new(gl.DirectContext)
	defer delete(imageCtxMap, ctx)
	tex := &stubTexture{}
	imageCtxMap[ctx] = map[uint64]genericImage{hash: tex}

	img.registerTextureCleanup()
	img.registerTextureCleanup() // must be idempotent, as imageForCanvas calls it on every texture upload

	img = nil //nolint:wastedassign // drops the last strong reference so the cleanup can run
	deadline := time.Now().Add(30 * time.Second)
	for !tex.released && time.Now().Before(deadline) {
		runtime.GC()
		drainTasks()
		time.Sleep(10 * time.Millisecond)
	}

	c.True(tex.released, "collecting an Image should release its cached textures")
	_, present := imageCtxMap[ctx][hash]
	c.False(present, "collecting an Image should evict its texture from imageCtxMap")
}

// TestImageGCRemovesCacheEntry verifies that an Image garbage collected without Dispose() ever being called has its
// imgCache entry removed. Before this, the entry's weak pointer went permanently stale and the key was retained
// forever, slowly growing the map without bound.
func TestImageGCRemovesCacheEntry(t *testing.T) {
	c := check.New(t)

	const w, h = 2, 2
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 53), geom.NewPoint(1, 1))
	c.NoError(err)

	hash := img.hash
	imgCacheLock.Lock()
	_, cached := imgCache[hash]
	imgCacheLock.Unlock()
	c.True(cached, "a freshly created image should be present in the cache")

	img = nil //nolint:wastedassign // drops the last strong reference so the cleanup can run
	deadline := time.Now().Add(30 * time.Second)
	for cached && time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
		imgCacheLock.Lock()
		_, cached = imgCache[hash]
		imgCacheLock.Unlock()
	}

	c.False(cached, "collecting an Image without Dispose should remove its cache entry")
}

// TestImageDisposeStopsGCCleanup verifies that Dispose cancels the pending GC cleanup: after Dispose, a later texture
// cached under the same hash (by a new Image built from the same data) must not be evicted when the old disposed Image
// is collected. Stop is called while the Image is still reachable, so the cleanup is guaranteed never to fire, making
// this check deterministic.
func TestImageDisposeStopsGCCleanup(t *testing.T) {
	c := check.New(t)

	const w, h = 2, 2
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 47), geom.NewPoint(1, 1))
	c.NoError(err)

	hash := img.hash
	ctx := new(gl.DirectContext)
	defer delete(imageCtxMap, ctx)
	imageCtxMap[ctx] = map[uint64]genericImage{hash: &stubTexture{}}
	img.registerTextureCleanup()
	img.Dispose()
	drainTasks() // run Dispose's queued texture release before caching the successor

	// Simulate a successor image's texture cached under the same hash after the old image was disposed.
	successor := &stubTexture{}
	imageCtxMap[ctx][hash] = successor

	img = nil //nolint:wastedassign // drops the last strong reference to the disposed image
	for range 5 {
		runtime.GC()
		drainTasks()
		time.Sleep(10 * time.Millisecond)
	}

	_, present := imageCtxMap[ctx][hash]
	c.True(present, "a stopped cleanup must not evict a successor's texture")
	c.False(successor.released, "a stopped cleanup must not release a successor's texture")
}

// TestImageToNRGBAUnpremultiplies verifies that ToNRGBA returns non-premultiplied pixels even when the underlying
// image is premultiplied, as it is for images decoded from encoded data with transparency. Before this, ToNRGBA read
// pixels using the image's own alpha type, so translucent pixels came back premultiplied (darkened) despite
// image.NRGBA's non-premultiplied contract.
func TestImageToNRGBAUnpremultiplies(t *testing.T) {
	c := check.New(t)

	const w, h = 2, 2
	colors := []color.NRGBA{
		{R: 200, G: 100, B: 40, A: 128},
		{R: 255, G: 255, B: 255, A: 51},
		{R: 10, G: 20, B: 30, A: 255},
		{R: 0, G: 0, B: 0, A: 0},
	}
	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i, col := range colors {
		src.SetNRGBA(i%w, i/w, col)
	}
	var buf bytes.Buffer
	c.NoError(png.Encode(&buf, src))

	codecs.Register() // normally done at app startup, which these headless tests skip
	img, err := NewImageFromBytes(buf.Bytes(), geom.NewPoint(1, 1))
	c.NoError(err)
	defer img.Dispose()
	c.Equal(imagecore.AlphaTypePremul, img.image.AlphaType(),
		"decoded translucent PNGs must be premultiplied for this test to exercise the conversion")

	nrgba, err := img.ToNRGBA()
	c.NoError(err)
	for i, want := range colors {
		got := nrgba.NRGBAAt(i%w, i/w)
		c.Equal(want.A, got.A, "pixel %d alpha", i)
		// The premultiply/unpremultiply round trip can shift color channels by a little due to rounding.
		for ch, pair := range [][2]uint8{{want.R, got.R}, {want.G, got.G}, {want.B, got.B}} {
			diff := int(pair[0]) - int(pair[1])
			if diff < 0 {
				diff = -diff
			}
			c.True(diff <= 2, "pixel %d channel %d: want %d, got %d", i, ch, pair[0], pair[1])
		}
	}
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
