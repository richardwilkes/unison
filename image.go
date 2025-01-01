// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
	"strconv"
	"sync"
	"unsafe"

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/softref"
	"github.com/richardwilkes/toolbox/xio"
	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/toolbox/xmath/hashhelper"
	"github.com/richardwilkes/unison/internal/skia"
	"github.com/zeebo/xxh3"
)

var (
	_       Drawable = &Image{}
	imgPool          = softref.NewPool()
)

// Image holds a reference to an image.
type Image softref.SoftRef

// NewImageFromFilePathOrURL creates a new image from data retrieved from the file path or URL. The http.DefaultClient
// will be used if the data is remote.
func NewImageFromFilePathOrURL(filePathOrURL string, scale float32) (*Image, error) {
	return NewImageFromFilePathOrURLWithContext(context.Background(), filePathOrURL, scale)
}

// NewImageFromFilePathOrURLWithContext creates a new image from data retrieved from the file path or URL. The
// http.DefaultClient will be used if the data is remote.
func NewImageFromFilePathOrURLWithContext(ctx context.Context, filePathOrURL string, scale float32) (*Image, error) {
	data, err := xio.RetrieveDataWithContext(ctx, filePathOrURL)
	if err != nil {
		return nil, errs.NewWithCause(filePathOrURL, err)
	}
	return NewImageFromBytes(data, scale)
}

// NewImageFromBytes creates a new image from raw bytes.
func NewImageFromBytes(buffer []byte, scale float32) (*Image, error) {
	if scale <= 0 {
		return nil, errs.New("invalid scale")
	}
	if len(buffer) < 1 {
		return nil, errs.New("no data in input buffer")
	}
	data := skia.DataNewWithCopy(buffer)
	defer skia.DataUnref(data)
	img := skia.ImageNewFromEncoded(data)
	if img == nil {
		return nil, errs.New("unable to decode image data")
	}
	return newImage(img, scale, hashImageData(skia.ImageGetWidth(img), skia.ImageGetHeight(img), scale, buffer))
}

// NewImageFromPixels creates a new image from pixel data.
func NewImageFromPixels(width, height int, pixels []byte, scale float32) (*Image, error) {
	if scale <= 0 {
		return nil, errs.New("invalid scale")
	}
	if width < 1 || height < 1 || scale <= 0 || len(pixels) != width*height*4 {
		return nil, errs.New("invalid image data")
	}
	data := skia.DataNewWithCopy(pixels)
	defer skia.DataUnref(data)
	img := skia.ImageNewRasterData(&skia.ImageInfo{
		Colorspace: skiaColorspace,
		Width:      int32(width),
		Height:     int32(height),
		ColorType:  skia.ColorTypeRGBA8888,
		AlphaType:  skia.AlphaTypeUnPreMul,
	}, data, width*4)
	if img == nil {
		return nil, errs.New("unable to create image")
	}
	return newImage(img, scale, hashImageData(width, height, scale, pixels))
}

// NewImageFromDrawing creates a new image by drawing into it. This is currently fairly inefficient, so take care to use
// it sparingly.
func NewImageFromDrawing(width, height, ppi int, draw func(*Canvas)) (*Image, error) {
	scale := float32(ppi) / 72
	s := &surface{
		context: skia.ContextMakeGL(defaultSkiaGL()),
		surface: skia.SurfaceMakeRasterN32PreMul(&skia.ImageInfo{
			Colorspace: skiaColorspace,
			Width:      int32(xmath.Ceil(float32(width) * scale)),
			Height:     int32(xmath.Ceil(float32(height) * scale)),
			ColorType:  skia.ColorTypeRGBA8888,
			AlphaType:  skia.AlphaTypeUnPreMul,
		}, defaultSurfaceProps()),
	}
	c := &Canvas{
		canvas:  skia.SurfaceGetCanvas(s.surface),
		surface: s,
	}
	c.RestoreToCount(1)
	c.SetMatrix(NewScaleMatrix(scale, scale))
	c.Save()
	toolbox.Call(func() { draw(c) })
	c.Restore()
	c.Flush()
	defer s.dispose()
	img := skia.SurfaceMakeImageSnapshot(s.surface)
	width = skia.ImageGetWidth(img)
	height = skia.ImageGetHeight(img)
	pixels := make([]byte, width*height*4)
	if !skia.ImageReadPixels(img, &skia.ImageInfo{
		Colorspace: skiaColorspace,
		Width:      int32(width),
		Height:     int32(height),
		ColorType:  skia.ColorTypeRGBA8888,
		AlphaType:  skia.AlphaTypeUnPreMul,
	}, pixels, width*4, 0, 0, skia.ImageCachingHintDisallow) {
		return nil, errs.New("unable to read raw pixels from image")
	}
	skia.ImageUnref(img)
	return NewImageFromPixels(width, height, pixels, 1)
}

func newImage(img skia.Image, scale float32, hash uint64) (*Image, error) {
	imgRef := &imageRef{
		hash:  hash,
		key:   strconv.FormatUint(hash, 36),
		img:   img,
		scale: scale,
	}
	ref, existedPreviously := imgPool.NewSoftRef(imgRef)
	if existedPreviously {
		imgRef.Release()
	}
	return (*Image)(ref), nil
}

func (img *Image) ref() *imageRef {
	if ref, ok := img.Resource.(*imageRef); ok {
		return ref
	}
	return nil
}

// Size returns the size, in pixels, of the image. These dimensions will always be whole numbers > 0 for valid images.
func (img *Image) Size() Size {
	ref := img.ref()
	return Size{
		Width:  float32(skia.ImageGetWidth(ref.img)),
		Height: float32(skia.ImageGetHeight(ref.img)),
	}
}

// LogicalSize returns the logical (device-independent) size.
func (img *Image) LogicalSize() Size {
	ref := img.ref()
	return Size{
		Width:  float32(skia.ImageGetWidth(ref.img)) * ref.scale,
		Height: float32(skia.ImageGetHeight(ref.img)) * ref.scale,
	}
}

// DrawInRect draws this image in the given rectangle.
func (img *Image) DrawInRect(canvas *Canvas, rect Rect, sampling *SamplingOptions, paint *Paint) {
	canvas.DrawImageInRect(img, rect, sampling, paint)
}

// Scale returns the internal scaling factor for this image.
func (img *Image) Scale() float32 {
	return img.ref().scale
}

// ToNRGBA creates an image.NRGBA from the image.
func (img *Image) ToNRGBA() (*image.NRGBA, error) {
	imgData := img.ref().img
	width := skia.ImageGetWidth(imgData)
	height := skia.ImageGetHeight(imgData)
	pixels := make([]byte, width*height*4)
	if !skia.ImageReadPixels(imgData, &skia.ImageInfo{
		Colorspace: skia.ImageGetColorSpace(imgData),
		Width:      int32(width),
		Height:     int32(height),
		ColorType:  skia.ColorTypeRGBA8888,
		AlphaType:  skia.ImageGetAlphaType(imgData),
	}, pixels, width*4, 0, 0, skia.ImageCachingHintDisallow) {
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
	data := skia.EncodePNG(nil, img.ref().img, compressionLevel)
	if data == nil {
		return nil, errs.New("unable to create PNG from image")
	}
	buffer := make([]byte, skia.DataGetSize(data))
	copy(buffer, unsafe.Slice((*byte)(skia.DataGetData(data)), len(buffer)))
	skia.DataUnref(data)
	return buffer, nil
}

// ToJPEG creates JPEG data from the image. quality should be greater than 0 and equal to or less than 100.
func (img *Image) ToJPEG(quality int) ([]byte, error) {
	data := skia.EncodeJPEG(nil, img.ref().img, quality)
	if data == nil {
		return nil, errs.New("unable to create JPEG from image")
	}
	buffer := make([]byte, skia.DataGetSize(data))
	copy(buffer, unsafe.Slice((*byte)(skia.DataGetData(data)), len(buffer)))
	skia.DataUnref(data)
	return buffer, nil
}

// ToWebp creates Webp data from the image. quality should be greater than 0 and equal to or less than 100.
func (img *Image) ToWebp(quality float32, lossy bool) ([]byte, error) {
	data := skia.EncodeWebp(nil, img.ref().img, quality, lossy)
	if data == nil {
		return nil, errs.New("unable to create WEBP from image")
	}
	buffer := make([]byte, skia.DataGetSize(data))
	copy(buffer, unsafe.Slice((*byte)(skia.DataGetData(data)), len(buffer)))
	skia.DataUnref(data)
	return buffer, nil
}

func releaseImagesForContext(ctx skia.DirectContext) {
	if m, ok := imageCtxMap[ctx]; ok {
		delete(imageCtxMap, ctx)
		for _, img := range m {
			skia.ImageUnref(img)
		}
	}
}

var (
	imageCtxMapLock sync.Mutex
	imageCtxMap     = make(map[skia.DirectContext]map[uint64]skia.Image)
)

type imageRef struct {
	img   skia.Image
	key   string
	hash  uint64
	scale float32
}

func (ref *imageRef) contextImg(s *surface) skia.Image {
	imageCtxMapLock.Lock()
	defer imageCtxMapLock.Unlock()
	var ctx skia.DirectContext
	if s != nil {
		ctx = s.context
	}
	m, ok := imageCtxMap[ctx]
	if !ok {
		m = make(map[uint64]skia.Image)
		imageCtxMap[ctx] = m
	}
	i, ok2 := m[ref.hash]
	if !ok2 {
		if ctx == nil {
			i = skia.ImageMakeNonTextureImage(ref.img)
		} else {
			i = skia.ImageTextureFromImage(ctx, ref.img, false, true)
		}
		if i != nil {
			m[ref.hash] = i
		} else {
			errs.Log(errs.New("failed to create texture from image"))
			i = ref.img
		}
	}
	return i
}

func (ref *imageRef) Key() string {
	return ref.key
}

func (ref *imageRef) Release() {
	imageCtxMapLock.Lock()
	var list []skia.Image
	for _, m := range imageCtxMap {
		if img, ok := m[ref.hash]; ok {
			list = append(list, img)
			delete(m, ref.hash)
		}
	}
	imageCtxMapLock.Unlock()
	list = append(list, ref.img)
	ref.img = nil
	// We have to do the actual release on the UI thread
	InvokeTask(func() {
		for _, img := range list {
			skia.ImageUnref(img)
		}
	})
}

func hashImageData(width, height int, scale float32, data []byte) uint64 {
	hasher := xxh3.New()
	hashhelper.Float32(hasher, scale)
	hashhelper.Num32(hasher, uint32(width))
	hashhelper.Num32(hasher, uint32(height))
	_, _ = hasher.Write(data) //nolint:errcheck // No real chance of failure here
	return hasher.Sum64()
}
