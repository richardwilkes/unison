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
	"sync"
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

// TestImageDisposeKeepsSuccessorTextures covers the other half of the successor-clobbering protection that
// TestImageDisposeStopsGCCleanup covers for the GC path. Dispose only queues its texture release, and in the window
// before that task runs, a successor Image built from identical data (the disposed Image's cache entry having just been
// evicted) can be created and drawn, adopting the textures still cached under the shared hash. The queued release must
// leave those alone, or the successor is forced into a redundant GPU re-upload on its next draw.
func TestImageDisposeKeepsSuccessorTextures(t *testing.T) {
	c := check.New(t)

	const w, h = 2, 2
	pixels := distinctPixels(w, h, 67)
	img, err := NewImageFromPixels(w, h, pixels, geom.NewPoint(1, 1))
	c.NoError(err)

	hash := img.hash
	ctx := new(gl.DirectContext)
	defer delete(imageCtxMap, ctx)
	tex := &stubTexture{}
	imageCtxMap[ctx] = map[uint64]genericImage{hash: tex}
	img.registerTextureCleanup()

	img.Dispose()

	// The successor resolves to the same hash, so imageForCanvas would hand it the texture still sitting in the map.
	successor, err := NewImageFromPixels(w, h, append([]byte(nil), pixels...), geom.NewPoint(1, 1))
	c.NoError(err)
	c.True(successor != img, "a disposed image must not be handed back out of the cache")
	c.Equal(hash, successor.hash)

	drainTasks()

	entry, present := imageCtxMap[ctx][hash]
	c.True(present, "the disposed image's release must not evict a live successor's texture")
	c.Equal(genericImage(tex), entry)
	c.False(tex.released, "the disposed image's release must not release a live successor's texture")

	// With the successor gone as well, nothing claims the hash any longer, so the texture must actually be released.
	successor.Dispose()
	drainTasks()

	_, present = imageCtxMap[ctx][hash]
	c.False(present, "disposing the last image using the hash should evict its texture")
	c.True(tex.released, "disposing the last image using the hash should release its texture")
}

// TestImageForCanvasAdoptsCachedTexture verifies that an Image which finds a texture already cached under its hash —
// uploaded by an earlier Image built from identical data — registers the GC cleanup that evicts it. Without this, a
// texture uploaded by an Image that was later disposed (its release skipped because this successor claimed the hash)
// would linger in imageCtxMap until its whole GL context was torn down.
func TestImageForCanvasAdoptsCachedTexture(t *testing.T) {
	c := check.New(t)

	const w, h = 2, 2
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 73), geom.NewPoint(1, 1))
	c.NoError(err)
	defer img.Dispose()

	ctx := new(gl.DirectContext)
	defer delete(imageCtxMap, ctx)
	tex := &stubTexture{}
	imageCtxMap[ctx] = map[uint64]genericImage{img.hash: tex}

	// A canvas whose surface has a GL context takes imageForCanvas's texture path; the texture is already cached, so
	// no upload (and hence no real GL work) is needed to reach the adoption path.
	got := img.imageForCanvas(&Canvas{surface: &surface{context: ctx}})
	c.Equal(genericImage(tex), got, "a texture already cached for the context should be reused")
	c.True(img.hasTexCleanup, "adopting a cached texture should register the eviction cleanup")
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

// TestImageForCanvasNilNonTextureFallsBack covers the failed-readback branch of imageForCanvas: when
// MakeNonTextureImage returns nil (its documented failure result for a texture image), imageForCanvas must fall back to
// the base image and must not cache the nil. Before this, the nil *imagecore.Image was assigned directly to the
// genericImage interface field, producing a non-nil interface holding a typed nil that the nil checks could never
// catch, so the typed nil was cached and handed to callers as a "valid" image on every subsequent call.
func TestImageForCanvasNilNonTextureFallsBack(t *testing.T) {
	c := check.New(t)

	stub := &stubTexture{} // its MakeNonTextureImage returns nil, modeling a failed GPU readback
	img := &Image{image: stub}

	got := img.imageForCanvas(nil)
	c.Equal(genericImage(stub), got, "a nil MakeNonTextureImage result must fall back to the base image")
	c.True(img.nonTextureImage == nil, "a nil MakeNonTextureImage result must not be cached as a typed nil")

	// The failure must not poison the cache: a second call takes the same fallback rather than returning a cached
	// typed nil.
	c.Equal(genericImage(stub), img.imageForCanvas(nil))
}

// TestNewImageFromDrawingInvalidDimensions verifies that non-positive width, height, or ppi produce an error like every
// other invalid-input path in this file. Before this, the nil surface returned by NewRasterN32Premul was used
// unchecked, so these inputs crashed with a nil dereference instead.
func TestNewImageFromDrawingInvalidDimensions(t *testing.T) {
	c := check.New(t)

	for _, in := range []struct{ width, height, ppi int }{
		{0, 10, 72},
		{10, 0, 72},
		{10, 10, 0},
		{-1, 10, 72},
		{10, -1, 72},
		{10, 10, -72},
	} {
		c.NotPanics(func() {
			img, err := NewImageFromDrawing(in.width, in.height, in.ppi, func(_ *Canvas) {})
			c.HasError(err, "width=%d height=%d ppi=%d", in.width, in.height, in.ppi)
			c.Nil(img, "width=%d height=%d ppi=%d", in.width, in.height, in.ppi)
		})
	}
}

// TestNewImageFromDrawingStaysOnCPU verifies that NewImageFromDrawing renders entirely on the CPU: its canvas has no GL
// DirectContext, so drawing an *Image inside the callback takes imageForCanvas's raster path instead of uploading a
// texture that the raster device would immediately read back. This also makes the function usable headless, which the
// pixel round-trip below relies on.
func TestNewImageFromDrawingStaysOnCPU(t *testing.T) {
	c := check.New(t)

	// A 2x2 opaque green inner image drawn at the origin over a red background gives every checked pixel an exact
	// expected value (opaque pixels survive the premul round trip losslessly).
	const innerSize = 2
	innerPixels := bytes.Repeat([]byte{0, 255, 0, 255}, innerSize*innerSize)
	inner, err := NewImageFromPixels(innerSize, innerSize, innerPixels, geom.NewPoint(1, 1))
	c.NoError(err)
	defer inner.Dispose()

	const w, h = 4, 3
	texturedContexts := len(imageCtxMap)
	var sawContext, sawRaster bool
	img, err := NewImageFromDrawing(w, h, 72, func(cv *Canvas) {
		// NewImageFromDrawing disposes its surface before returning, so inspect it here rather than afterward.
		sawContext = cv.surface.context != nil
		sawRaster = cv.surface.raster != nil
		cv.Clear(RGB(255, 0, 0))
		cv.DrawImage(inner, geom.Point{}, nil, nil)
	})
	c.NoError(err)
	c.NotNil(img)
	defer img.Dispose()

	c.False(sawContext, "NewImageFromDrawing must not create a GL context for its raster surface")
	c.True(sawRaster, "the drawing surface should be flagged as raster")
	c.Equal(texturedContexts, len(imageCtxMap), "drawing an image into a raster surface must not upload GL textures")

	// The drawn content must survive the snapshot/readback: the inner image covers the top-left, the cleared red shows
	// everywhere else.
	c.Equal(geom.NewSize(w, h), img.Size())
	nrgba, err := img.ToNRGBA()
	c.NoError(err)
	c.Equal(color.NRGBA{G: 255, A: 255}, nrgba.NRGBAAt(0, 0), "the drawn inner image should survive the readback")
	c.Equal(color.NRGBA{R: 255, A: 255}, nrgba.NRGBAAt(w-1, h-1), "the cleared background should survive the readback")
}

// distinctPNGForTest returns PNG-encoded data for a width x height image built from the given seed, so each test gets
// its own cache hash and does not collide with images created by other tests in this shared-cache package.
func distinctPNGForTest(c check.Checker, width, height, seed int) []byte {
	src := image.NewNRGBA(image.Rect(0, 0, width, height))
	copy(src.Pix, distinctPixels(width, height, seed))
	for i := 3; i < len(src.Pix); i += 4 {
		src.Pix[i] = 255 // fully opaque, so the encoded bytes stay a direct function of the seed
	}
	var buf bytes.Buffer
	c.NoError(png.Encode(&buf, src))
	return buf.Bytes()
}

// TestImagePreloadDecodesLazyImage verifies the primary purpose of Preload: an image created from encoded bytes is
// lazy (only its header was parsed), and Preload performs and caches the pixel decode that would otherwise happen on
// the UI thread during the first draw. It must be repeatable, returning true again once the pixels are present.
func TestImagePreloadDecodesLazyImage(t *testing.T) {
	c := check.New(t)

	codecs.Register() // normally done at app startup, which these headless tests skip
	img, err := NewImageFromBytes(distinctPNGForTest(c, 6, 4, 101), geom.NewPoint(1, 1))
	c.NoError(err)
	c.NotNil(img)
	defer img.Dispose()

	raster, ok := img.image.(*imagecore.Image)
	c.True(ok, "an image created from encoded bytes should be a raster *imagecore.Image")
	c.True(raster.IsLazy(), "an image created from encoded bytes should start out undecoded")

	c.True(img.Preload(), "Preload should decode the image")
	c.True(img.Preload(), "Preload on an already decoded image should return true again")

	// The decode must have been cached rather than repeated, which is what spares the first draw the work: a cached
	// decode hands back the identical pixels on every subsequent request.
	pixels := raster.PeekPixels(imagecore.CachingAllow)
	c.NotNil(pixels)
	c.True(pixels == raster.PeekPixels(imagecore.CachingAllow), "Preload should have cached the decoded pixels")
}

// TestImagePreloadFailsForUndecodableData verifies that Preload reports failure for data whose header parses but whose
// pixels do not decode. The first 33 bytes of a PNG are the signature plus the IHDR chunk, which is enough for the
// header parse that NewImageFromBytes performs, so the failure only surfaces when the pixels are actually decoded.
// The failure must be sticky, since a caller may retry.
func TestImagePreloadFailsForUndecodableData(t *testing.T) {
	c := check.New(t)

	codecs.Register() // normally done at app startup, which these headless tests skip
	data := distinctPNGForTest(c, 5, 7, 103)
	c.True(len(data) > 33)
	img, err := NewImageFromBytes(data[:33], geom.NewPoint(1, 1))
	c.NoError(err, "a truncated PNG should still yield an image, since only its header is parsed")
	c.NotNil(img)
	defer img.Dispose()

	c.Equal(geom.NewSize(5, 7), img.Size(), "the size should come from the IHDR chunk")
	c.False(img.Preload(), "Preload should report failure when the pixels do not decode")
	c.False(img.Preload(), "a failed decode should stay failed")
}

// TestImagePreloadOnAlreadyDecodedImage verifies that Preload succeeds and returns at once for an image whose pixels
// were supplied directly, since there is nothing left to decode.
func TestImagePreloadOnAlreadyDecodedImage(t *testing.T) {
	c := check.New(t)

	const w, h = 3, 3
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 107), geom.NewPoint(1, 1))
	c.NoError(err)
	defer img.Dispose()

	raster, ok := img.image.(*imagecore.Image)
	c.True(ok)
	c.False(raster.IsLazy(), "an image created from pixels is never lazy")
	c.True(img.Preload(), "Preload on an image that already holds its pixels should succeed")
}

// TestImagePreloadOnDisposedImage verifies that Preload reports failure rather than panicking for a disposed image,
// whose underlying image reference has been dropped. A background loader can race a Dispose from the UI thread, so
// this must be safe.
func TestImagePreloadOnDisposedImage(t *testing.T) {
	c := check.New(t)

	const w, h = 3, 3
	img, err := NewImageFromPixels(w, h, distinctPixels(w, h, 109), geom.NewPoint(1, 1))
	c.NoError(err)

	img.Dispose()

	c.NotPanics(func() { c.False(img.Preload(), "Preload on a disposed image should report failure") })
}

// TestImagePreloadOnNilImage verifies that Preload tolerates a nil receiver, matching Dispose, so callers can preload
// an optional image without a nil check.
func TestImagePreloadOnNilImage(t *testing.T) {
	c := check.New(t)

	var img *Image
	c.NotPanics(func() { c.False(img.Preload(), "Preload on a nil image should report failure") })
}

// TestImagePreloadConcurrent verifies the documented promise that Preload may be called from any goroutine, even
// concurrently, which is what lets a loader decode many images in parallel on background goroutines. Under -race this
// also proves the decode itself is properly synchronized.
func TestImagePreloadConcurrent(t *testing.T) {
	c := check.New(t)

	codecs.Register() // normally done at app startup, which these headless tests skip
	img, err := NewImageFromBytes(distinctPNGForTest(c, 8, 6, 113), geom.NewPoint(1, 1))
	c.NoError(err)
	defer img.Dispose()

	raster, ok := img.image.(*imagecore.Image)
	c.True(ok)
	c.True(raster.IsLazy(), "the image must still be undecoded for the goroutines to race on the decode")

	const goroutines = 8
	results := make([]bool, goroutines)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // line the goroutines up so they hit the decode together
			results[i] = img.Preload()
		}()
	}
	close(start)
	wg.Wait()

	for i, result := range results {
		c.True(result, "goroutine %d should have seen a successful decode", i)
	}
}
