// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"image"
	"math"
	"sync"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/softref"
	"github.com/richardwilkes/toolbox/xio"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison/internal/skia"
)

var (
	_       Drawable = &Image{}
	imgPool          = softref.NewPool(&jot.Logger{})
)

// Image holds a reference to an image.
type Image softref.SoftRef

// NewImageFromFilePathOrURL creates a new image from data retrieved from the file path or URL. The http.DefaultClient
// will be used if the data is remote.
func NewImageFromFilePathOrURL(filePathOrURL string, scale float32) (*Image, error) {
	data, err := xio.RetrieveData(filePathOrURL)
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
	return newImage(img, scale)
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
	return newImage(img, scale)
}

func newImage(img skia.Image, scale float32) (*Image, error) {
	imgRef := &imageRef{
		img:   img,
		scale: scale,
	}
	width := skia.ImageGetWidth(img)
	height := skia.ImageGetHeight(img)
	pixels := make([]byte, width*height*4)
	// TODO: Consider generating the key from the input rather than the raw pixels, as this call is potentially expensive
	if !skia.ImageReadPixels(img, &skia.ImageInfo{
		Colorspace: skia.ImageGetColorSpace(img),
		Width:      int32(width),
		Height:     int32(height),
		ColorType:  skia.ImageGetColorType(img),
		AlphaType:  skia.ImageGetAlphaType(img),
	}, pixels, width*4, 0, 0, skia.ImageCachingHintAllow) {
		return nil, errs.New("unable to read raw pixels from image")
	}
	s := sha256.New224()
	buffer := make([]byte, 8)
	binary.LittleEndian.PutUint64(buffer, math.Float64bits(float64(scale)))
	if _, err := s.Write(buffer); err != nil {
		return nil, errs.Wrap(err)
	}
	if _, err := s.Write(pixels); err != nil {
		return nil, errs.Wrap(err)
	}
	imgRef.key = base64.RawURLEncoding.EncodeToString(s.Sum(nil)[:sha256.Size224])
	ref, existedPreviously := imgPool.NewSoftRef(imgRef)
	if existedPreviously {
		imgRef.Release()
	}
	return (*Image)(ref), nil
}

func (img *Image) ref() *imageRef {
	return img.Resource.(*imageRef)
}

// Size returns the size, in pixels, of the image. These dimensions will always be whole numbers > 0 for valid images.
func (img *Image) Size() geom32.Size {
	ref := img.ref()
	return geom32.Size{
		Width:  float32(skia.ImageGetWidth(ref.img)),
		Height: float32(skia.ImageGetHeight(ref.img)),
	}
}

// LogicalSize returns the logical (device-independent) size.
func (img *Image) LogicalSize() geom32.Size {
	ref := img.ref()
	return geom32.Size{
		Width:  float32(skia.ImageGetWidth(ref.img)) * ref.scale,
		Height: float32(skia.ImageGetHeight(ref.img)) * ref.scale,
	}
}

// DrawInRect draws this image in the given rectangle.
func (img *Image) DrawInRect(canvas *Canvas, rect geom32.Rect, sampling *SamplingOptions, paint *Paint) {
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
	}, pixels, width*4, 0, 0, skia.ImageCachingHintAllow) {
		return nil, errs.New("unable to read raw pixels from image")
	}
	return &image.NRGBA{
		Pix:    pixels,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}, nil
}

// ToPNG creates PNG data from the image.
func (img *Image) ToPNG() ([]byte, error) {
	return img.encode(PNG, 100)
}

// ToJPEG creates JPEG data from the image. quality should be greater than 0 and equal to or less than 100.
func (img *Image) ToJPEG(quality int) ([]byte, error) {
	return img.encode(JPEG, quality)
}

// ToWebp creates Webp data from the image. quality should be greater than 0 and equal to or less than 100.
func (img *Image) ToWebp(quality int) ([]byte, error) {
	return img.encode(WEBP, quality)
}

func (img *Image) encode(format EncodedImageFormat, quality int) ([]byte, error) {
	if quality < 1 {
		quality = 1
	} else if quality > 100 {
		quality = 100
	}
	data := skia.ImageEncodeSpecific(img.ref().img, skia.EncodedImageFormat(format), quality)
	if data == nil {
		return nil, errs.Newf("unable to create %s from image", format)
	}
	buffer := make([]byte, skia.DataGetSize(data))
	copy(buffer, ((*[1 << 30]byte)(skia.DataGetData(data)))[:len(buffer)])
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
	imageCtxMap     = make(map[skia.DirectContext]map[string]skia.Image)
)

type imageRef struct {
	key   string
	img   skia.Image
	scale float32
}

func (ref *imageRef) contextImg(ctx skia.DirectContext) skia.Image {
	imageCtxMapLock.Lock()
	defer imageCtxMapLock.Unlock()
	m, ok := imageCtxMap[ctx]
	if !ok {
		m = make(map[string]skia.Image)
		imageCtxMap[ctx] = m
	}
	i, ok2 := m[ref.key]
	if !ok2 {
		if i = skia.ImageMakeTextureImage(ref.img, ctx, false); i != nil {
			m[ref.key] = i
		} else {
			jot.Warn("failed to create texture from image")
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
		if img, ok := m[ref.key]; ok {
			list = append(list, img)
			delete(m, ref.key)
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
