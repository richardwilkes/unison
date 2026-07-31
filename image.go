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
	"context"
	"image"
	"net/http"
	"runtime"
	"sync"
	"time"
	"weak"

	"github.com/richardwilkes/canvas/codecs"
	"github.com/richardwilkes/canvas/gpu"
	"github.com/richardwilkes/canvas/gpu/gl"
	"github.com/richardwilkes/canvas/imagecore"
	canvassurface "github.com/richardwilkes/canvas/surface"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xhash"
	"github.com/richardwilkes/toolbox/v2/xhttp"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/zeebo/xxh3"
)

var (
	_            Drawable = &Image{}
	imgCacheLock sync.Mutex
	imgCache     = make(map[uint64]weak.Pointer[Image])
	// imageCtxMap is only accessed on the UI (rendering) thread, since each entry is tied to a GL DirectContext, so it
	// does not require its own lock.
	imageCtxMap = make(map[*gl.DirectContext]map[uint64]genericImage)
)

type genericImage interface {
	// Width mirrors SkImage::width.
	Width() int32
	// Height mirrors SkImage::height.
	Height() int32
	// AlphaType mirrors SkImage::alphaType.
	AlphaType() imagecore.AlphaType
	// IsAlphaOnly mirrors SkImage::isAlphaOnly.
	IsAlphaOnly() bool
	// UniqueID mirrors SkImage::uniqueID.
	UniqueID() uint32
	// MakeNonTextureImage mirrors SkImage::makeNonTextureImage: a CPU image (the identity for a raster image, a
	// readback for a texture image).
	MakeNonTextureImage() *imagecore.Image
	// ReadPixels mirrors SkImage::readPixels (a texture image reads its pixels back internally).
	ReadPixels(dstInfo imagecore.ImageInfo, dst []byte, dstRowBytes int, srcX, srcY int32, hint imagecore.CachingHint) bool
}

// Image holds a reference to an image.
type Image struct {
	image           genericImage
	nonTextureImage genericImage
	hash            uint64
	scale           geom.Point
	texCleanup      runtime.Cleanup
	hasTexCleanup   bool
	disposeOnce     sync.Once
}

// NewImageFromFilePathOrURL creates a new image from data retrieved from the file path or URL. You may pass nil for the
// client to use the http.DefaultClient if the data is remote. A maxBytes of 0 or less means no limit on the number of
// bytes allowed.
func NewImageFromFilePathOrURL(ctx context.Context, client *http.Client, filePathOrURL string, scale geom.Point, maxBytes int64) (*Image, error) {
	data, err := xhttp.RetrieveDataWithLimit(ctx, client, filePathOrURL, maxBytes)
	if err != nil {
		return nil, errs.NewWithCause(filePathOrURL, err)
	}
	return NewImageFromBytes(data, scale)
}

// NewImageFromBytes creates a new image from raw bytes.
func NewImageFromBytes(buffer []byte, scale geom.Point) (*Image, error) {
	if scale.X <= 0 || scale.Y <= 0 {
		return nil, errs.New("invalid scale")
	}
	if len(buffer) < 1 {
		return nil, errs.New("no data in input buffer")
	}
	localData := make([]byte, len(buffer))
	copy(localData, buffer)
	if img := imagecore.NewFromEncoded(localData); img != nil {
		return newImage(img, scale, hashImageData(int(img.Width()), int(img.Height()), scale, buffer))
	}
	return nil, errs.New("unable to decode image data")
}

// NewImageFromPixels creates a new image from pixel data.
func NewImageFromPixels(width, height int, pixels []byte, scale geom.Point) (*Image, error) {
	if scale.X <= 0 || scale.Y <= 0 {
		return nil, errs.New("invalid scale")
	}
	if width < 1 || height < 1 || len(pixels) != width*height*4 {
		return nil, errs.New("invalid image data")
	}
	localData := make([]byte, len(pixels))
	copy(localData, pixels)
	if img := imagecore.NewRasterData(imagecore.ImageInfo{
		Width:     int32(width),
		Height:    int32(height),
		ColorType: imagecore.ColorTypeRGBA8888,
		AlphaType: imagecore.AlphaTypeUnpremul,
	}, localData, width*4); img != nil {
		return newImage(img, scale, hashImageData(width, height, scale, pixels))
	}
	return nil, errs.New("unable to create image")
}

// NewImageFromDrawing creates a new image by drawing into it. This is currently fairly inefficient, so take care to use
// it sparingly.
func NewImageFromDrawing(width, height, ppi int, draw func(*Canvas)) (*Image, error) {
	scale := float32(ppi) / 72
	ss := canvassurface.NewRasterN32Premul(int32(xmath.Ceil(float32(width)*scale)),
		int32(xmath.Ceil(float32(height)*scale)), &canvassurface.Props{PixelGeometry: canvassurface.PixelGeometryRGBH})
	if ss == nil {
		return nil, errs.New("invalid dimensions")
	}
	// The surface is always raster, so no GL context is set: leaving surface.context nil keeps imageForCanvas (and
	// everything else) on the pure-CPU path, avoiding a pointless texture upload plus GPU→CPU readback for each image
	// drawn by the callback.
	s := &surface{
		surface: ss,
		raster:  ss,
	}
	c := &Canvas{
		canvas:  s.surface.Canvas(),
		surface: s,
	}
	c.RestoreToCount(1)
	c.SetMatrix(geom.NewScaleMatrix(scale, scale))
	c.Save()
	SafeCall(func() { draw(c) })
	c.Restore()
	c.Flush()
	defer s.dispose()
	img := ss.MakeImageSnapshot()
	width = int(img.Width())
	height = int(img.Height())
	pixels := make([]byte, width*height*4)
	if !img.ReadPixels(imagecore.ImageInfo{
		Width:     int32(width),
		Height:    int32(height),
		ColorType: imagecore.ColorTypeRGBA8888,
		AlphaType: imagecore.AlphaTypeUnpremul,
	}, pixels, width*4, 0, 0, imagecore.CachingDisallow) {
		return nil, errs.New("unable to read raw pixels from image")
	}
	return NewImageFromPixels(width, height, pixels, geom.NewPoint(1/scale, 1/scale))
}

// newImage may be called from any goroutine, so access to imgCache is guarded by imgCacheLock.
func newImage(baseImage genericImage, scale geom.Point, hash uint64) (*Image, error) {
	imgCacheLock.Lock()
	if existing, ok := imgCache[hash]; ok {
		if actual := existing.Value(); actual != nil {
			imgCacheLock.Unlock()
			return actual, nil
		}
	}
	img := &Image{
		image: baseImage,
		hash:  hash,
		scale: scale,
	}
	imgCache[hash] = weak.Make(img)
	// If the Image is garbage collected without Dispose() ever being called, its cache entry would hold a permanently
	// stale weak pointer, so arrange for the entry to be removed when the Image is collected. The cleanup must not
	// capture img itself, or the Image would never become collectable. If another Image with the same hash has been
	// cached in the meantime, its entry is live and must be left alone.
	runtime.AddCleanup(img, func(h uint64) {
		imgCacheLock.Lock()
		if p, ok := imgCache[h]; ok && p.Value() == nil {
			delete(imgCache, h)
		}
		imgCacheLock.Unlock()
	}, hash)
	imgCacheLock.Unlock()
	return img, nil
}

func asRaster(img genericImage) *imagecore.Image {
	switch im := img.(type) {
	case *imagecore.Image:
		return im
	case *gl.TextureImage:
		return im.MakeNonTextureImage()
	default:
		return nil
	}
}

// Dispose releases the native resource. Use this if you wish to force cleanup earlier than a gc run would normally
// trigger it.
func (img *Image) Dispose() {
	if img == nil {
		return
	}
	img.disposeOnce.Do(func() {
		imgCacheLock.Lock()
		if p, ok := imgCache[img.hash]; ok && p.Value() == img {
			delete(imgCache, img.hash)
		}
		imgCacheLock.Unlock()
		img.texCleanup.Stop()
		// Dispose may be called from any goroutine, but imageCtxMap is UI-thread-only state and releasing a texture is
		// GL work, so marshal the release onto the UI thread, just as the GC cleanup path does. Capture the hash rather
		// than img so the task does not extend the Image's lifetime.
		hash := img.hash
		InvokeTask(func() { releaseTexturesForImage(hash) })
		img.image = nil
		img.nonTextureImage = nil
	})
}

// Size returns the size, in pixels, of the image. These dimensions will always be whole numbers > 0 for valid images.
func (img *Image) Size() geom.Size {
	return geom.NewSize(float32(img.image.Width()), float32(img.image.Height()))
}

// LogicalSize returns the logical (device-independent) size.
func (img *Image) LogicalSize() geom.Size {
	return geom.NewSize(float32(img.image.Width())*img.scale.X, float32(img.image.Height())*img.scale.Y)
}

// DrawInRect draws this image in the given rectangle.
func (img *Image) DrawInRect(canvas *Canvas, rect geom.Rect, sampling *SamplingOptions, paint *Paint) {
	canvas.DrawImageInRect(img, rect, sampling, paint)
}

// Scale returns the internal scaling factor for this image.
func (img *Image) Scale() geom.Point {
	return img.scale
}

// ToNRGBA creates an image.NRGBA from the image.
func (img *Image) ToNRGBA() (*image.NRGBA, error) {
	width := int(img.image.Width())
	height := int(img.image.Height())
	pixels := make([]byte, width*height*4)
	if !img.image.ReadPixels(imagecore.ImageInfo{
		Width:     int32(width),
		Height:    int32(height),
		ColorType: imagecore.ColorTypeRGBA8888,
		AlphaType: imagecore.AlphaTypeUnpremul,
	}, pixels, width*4, 0, 0, imagecore.CachingDisallow) {
		return nil, errs.New("unable to read raw pixels from image")
	}
	return &image.NRGBA{
		Pix:    pixels,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}, nil
}

// ToPNG creates PNG data from the image. 'compressionLevel' should in the range 0-9 and is equivalent to
// the zlib compression level. A typical compression level is 6 and is equivalent to the zlib default.
func (img *Image) ToPNG(compressionLevel int) ([]byte, error) {
	if data := codecs.EncodePNG(asRaster(img.image), compressionLevel); data != nil {
		return data, nil
	}
	return nil, errs.New("unable to create PNG from image")
}

// ToJPEG creates JPEG data from the image. quality should be greater than 0 and equal to or less than 100.
func (img *Image) ToJPEG(quality int) ([]byte, error) {
	if data := codecs.EncodeJPEG(asRaster(img.image), quality); data != nil {
		return data, nil
	}
	return nil, errs.New("unable to create JPEG from image")
}

// ToWebp creates Webp data from the image. quality should be greater than 0 and equal to or less than 100.
func (img *Image) ToWebp(quality float32, lossy bool) ([]byte, error) {
	if data := codecs.EncodeWebP(asRaster(img.image), quality, lossy); data != nil {
		return data, nil
	}
	return nil, errs.New("unable to create WEBP from image")
}

// Hash returns a hash of the image data.
func (img *Image) Hash() uint64 {
	return img.hash
}

func (img *Image) imageForCanvas(canvas *Canvas) genericImage {
	if canvas == nil || canvas.surface == nil || canvas.surface.context == nil {
		if img.nonTextureImage != nil {
			return img.nonTextureImage
		}
		// Assign through a concrete-typed local so a nil *imagecore.Image is never stored in the interface field, where
		// it would become a non-nil interface holding a typed nil that both this nil check and the cached-value check
		// above would then treat as a valid image.
		if nti := img.image.MakeNonTextureImage(); nti != nil {
			img.nonTextureImage = nti
			return img.nonTextureImage
		}
		return img.image
	}
	m, ok := imageCtxMap[canvas.surface.context]
	if !ok {
		m = make(map[uint64]genericImage)
		imageCtxMap[canvas.surface.context] = m
	}
	if cached, present := m[img.hash]; present {
		return cached
	}
	// img.image is always a raster *imagecore.Image (see the constructors and TestImageBackedByRaster), so upload it to
	// a texture-backed image for this context.
	tex := gl.TextureFromImage(canvas.surface.context, asRaster(img.image), gpu.MipmappedNo, gpu.BudgetedYes)
	if tex == nil {
		return img.image
	}
	m[img.hash] = tex
	img.registerTextureCleanup()
	return tex
}

// registerTextureCleanup arranges for the image's cached GPU textures to be evicted from imageCtxMap when the Image is
// garbage collected, since entries would otherwise live until their whole GL context is destroyed. Called on the UI
// thread (from imageForCanvas) when the first texture is uploaded; registering once covers every context, as the
// cleanup evicts by hash across all of imageCtxMap.
func (img *Image) registerTextureCleanup() {
	if img.hasTexCleanup {
		return
	}
	img.hasTexCleanup = true
	// The cleanup must not capture img itself, or the Image would never become collectable. It runs on the runtime's
	// cleanup goroutine, so the actual eviction is marshaled onto the UI thread, which owns imageCtxMap.
	img.texCleanup = runtime.AddCleanup(img, func(hash uint64) {
		InvokeTask(func() { releaseTexturesForImage(hash) })
	}, img.hash)
}

// releaseTexturesForImage evicts and releases the given image hash's texture entries from every live GL context's
// cache. Must be called on the UI thread, since imageCtxMap is UI-thread-only state.
func releaseTexturesForImage(hash uint64) {
	for _, m := range imageCtxMap {
		if img, ok := m[hash]; ok {
			delete(m, hash)
			releaseTexture(img)
		}
	}
}

// releaseTexture releases the GPU texture behind a cached context-map entry, if it has one. In production the entries
// are always *gl.TextureImage, whose Release drops the texture proxy ref so the GPU resource is freed deterministically
// rather than waiting on the GC.
func releaseTexture(img genericImage) {
	if tex, ok := img.(interface{ Release() }); ok {
		tex.Release()
	}
}

func releaseImagesForContext(ctx *gl.DirectContext) {
	if m, ok := imageCtxMap[ctx]; ok {
		delete(imageCtxMap, ctx)
		for _, img := range m {
			// The context map only ever holds texture-backed images produced by gl.TextureFromImage, so releasing
			// their texture proxies is the one place a real unref happens.
			releaseTexture(img)
		}
	}
	InvokeTaskAfter(func() {
		// Encourage unused images to be cleaned up
		runtime.GC()
		runtime.GC()
	}, time.Millisecond)
}

func hashImageData(width, height int, scale geom.Point, data []byte) uint64 {
	hasher := xxh3.New()
	xhash.Float32(hasher, scale.X)
	xhash.Float32(hasher, scale.Y)
	xhash.Num32(hasher, uint32(width))
	xhash.Num32(hasher, uint32(height))
	_, _ = hasher.Write(data) //nolint:errcheck // No real chance of failure here
	return hasher.Sum64()
}
