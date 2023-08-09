// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/toolbox/xmath/geom"
	"github.com/richardwilkes/unison/internal/skia"
)

// ColorChannel specifies a specific channel within an RGBA color.
type ColorChannel byte

// Possible values for ColorChannel.
const (
	RedChannel ColorChannel = iota
	GreenChannel
	BlueChannel
	AlphaChannel
)

// ImageFilter performs a transformation on the image before drawing it.
type ImageFilter struct {
	filter skia.ImageFilter
}

func newImageFilter(filter skia.ImageFilter) *ImageFilter {
	if filter == nil {
		return nil
	}
	f := &ImageFilter{filter: filter}
	runtime.SetFinalizer(f, func(obj *ImageFilter) {
		ReleaseOnUIThread(func() {
			skia.ImageFilterUnref(obj.filter)
		})
	})
	return f
}

func (f *ImageFilter) filterOrNil() skia.ImageFilter {
	if f == nil {
		return nil
	}
	return f.filter
}

// NewArithmeticImageFilter returns a new arithmetic image filter. Each output pixel is the result of combining the
// corresponding background and foreground pixels using the 4 coefficients:
// k1 * foreground * background + k2 * foreground + k3 * background + k4
// Both background and foreground may be nil, in which case the source bitmap is used.
// If enforcePMColor is true, the RGB channels will clamped to the calculated alpha.
// cropRect may be nil.
func NewArithmeticImageFilter(k1, k2, k3, k4 float32, background, foreground *ImageFilter, enforcePMColor bool, cropRect *Rect) *ImageFilter {
	var bg, fg skia.ImageFilter
	if background != nil {
		bg = background.filter
	}
	if foreground != nil {
		fg = foreground.filter
	}
	return newImageFilter(skia.ImageFilterNewArithmetic(k1, k2, k3, k4, enforcePMColor, bg, fg, cropRect))
}

// NewBlurImageFilter returns a new blur image filter. input may be nil, in which case the source bitmap is used.
// cropRect may be nil.
func NewBlurImageFilter(sigmaX, sigmaY float32, tileMode TileMode, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewBlur(sigmaX, sigmaY, skia.TileMode(tileMode), in, cropRect))
}

// NewColorImageFilter returns a new color image filter. input may be nil, in which case the source bitmap is used.
// cropRect may be nil.
func NewColorImageFilter(colorFilter *ColorFilter, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewColorFilter(colorFilter.filter, in, cropRect))
}

// NewComposeImageFilter returns a new combining image filter.
func NewComposeImageFilter(outer, inner *ImageFilter) *ImageFilter {
	return newImageFilter(skia.ImageFilterNewCompose(outer.filter, inner.filter))
}

// NewDisplacementImageFilter returns a new displacement image filter. displayment may be nil, in which case the source
// bitmap will be used. cropRect may be nil.
func NewDisplacementImageFilter(xChannelSelector, yChannelSelector ColorChannel, scale float32, displacement, color *ImageFilter, cropRect *Rect) *ImageFilter {
	var dis skia.ImageFilter
	if displacement != nil {
		dis = displacement.filter
	}
	return newImageFilter(skia.ImageFilterNewDisplacementMapEffect(skia.ColorChannel(xChannelSelector),
		skia.ColorChannel(yChannelSelector), scale, dis, color.filter, cropRect))
}

// NewDropShadowImageFilter returns a new drop shadow image filter. input may be nil, in which case the source bitmap
// will be used. cropRect may be nil.
func NewDropShadowImageFilter(dx, dy, sigmaX, sigmaY float32, color Color, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewDropShadow(dx, dy, sigmaX, sigmaY, skia.Color(color), in, cropRect))
}

// NewDropShadowOnlyImageFilter returns a new drop shadow only image filter. input may be nil, in which case the source
// bitmap will be used. cropRect may be nil.
func NewDropShadowOnlyImageFilter(dx, dy, sigmaX, sigmaY float32, color Color, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewDropShadowOnly(dx, dy, sigmaX, sigmaY, skia.Color(color), in, cropRect))
}

// NewImageSourceImageFilter returns a new image source image filter. If canvas is not nil, a hardware-accellerated
// image will be used if possible.
func NewImageSourceImageFilter(canvas *Canvas, img *Image, srcRect, dstRect Rect, sampling *SamplingOptions) *ImageFilter {
	var image skia.Image
	ref := img.ref()
	if canvas == nil {
		image = ref.img
	} else {
		image = ref.contextImg(canvas.surface)
	}
	return newImageFilter(skia.ImageFilterNewImageSource(image, &srcRect, &dstRect, sampling.skSamplingOptions()))
}

// NewImageSourceDefaultImageFilter returns a new image source image filter that uses the default quality and the full
// image size. If canvas is not nil, a hardware-accellerated image will be used if possible.
func NewImageSourceDefaultImageFilter(canvas *Canvas, img *Image) *ImageFilter {
	var image skia.Image
	ref := img.ref()
	if canvas == nil {
		image = ref.img
	} else {
		image = ref.contextImg(canvas.surface)
	}
	return newImageFilter(skia.ImageFilterNewImageSourceDefault(image))
}

// NewMagnifierImageFilter returns a new magnifier image filter. input may be nil, in which case the source bitmap will
// be used. cropRect may be nil.
func NewMagnifierImageFilter(src Rect, inset float32, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewMagnifier(&src, inset, in, cropRect))
}

// NewMatrixConvolutionImageFilter returns a new matrix convolution image filter.
//
// width, height: The kernel size in pixels.
// kernel: The image processing kernel. Must contain width * height elements, in row order. If less than this, zeroes
// will be added to make up the difference.
// gain: A scale factor applied to each pixel after convolution. This can be used to normalize the kernel, if it does
// not already sum to 1.
// bias: A bias factor added to each pixel after convolution.
// offsetX, offsetY: An offset applied to each pixel coordinate before convolution. This can be used to center the
// kernel over the image (e.g., a 3x3 kernel should have an offset of {1, 1}).
// tileMode: How accesses outside the image are treated.
// convolveAlpha: If true, all channels are convolved. If false, only the RGB channels are convolved, and alpha is
// copied from the source image.
// input: The input image filter, if nil the source bitmap is used instead.
// cropRect: Rectangle to which the output processing will be limited. May be nil.
func NewMatrixConvolutionImageFilter(width, height int, kernel []float32, gain, bias float32, offsetX, offsetY int, tileMode TileMode, convolveAlpha bool, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	if len(kernel) < width*height {
		k := make([]float32, width*height)
		copy(k, kernel)
		kernel = k
	}
	return newImageFilter(skia.ImageFilterNewMatrixConvolution(&skia.ISize{
		Width:  int32(width),
		Height: int32(height),
	}, kernel, gain, bias, &skia.IPoint{
		X: int32(offsetX),
		Y: int32(offsetY),
	}, skia.TileMode(tileMode), convolveAlpha, in, cropRect))
}

// NewMatrixTransformImageFilter returns a new matrix transform image filter. input may be nil, in which case the source
// bitmap will be used.
func NewMatrixTransformImageFilter(matrix *geom.Matrix2D32, sampling *SamplingOptions, input *ImageFilter) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewMatrixTransform(skia.Matrix2DtoMatrix(matrix),
		sampling.skSamplingOptions(), in))
}

// NewMergeImageFilter returns a new merge image filter. Each filter will draw their results in order with src-over
// blending. A nil filter will use the source bitmap instead. cropRect may be nil.
func NewMergeImageFilter(filters []*ImageFilter, cropRect *Rect) *ImageFilter {
	ff := make([]skia.ImageFilter, len(filters))
	for i, one := range filters {
		if one != nil {
			ff[i] = one.filter
		}
	}
	return newImageFilter(skia.ImageFilterNewMerge(ff, cropRect))
}

// NewOffsetImageFilter returns a new offset image filter. input may be nil, in which case the source bitmap will be
// used. cropRect may be nil.
func NewOffsetImageFilter(dx, dy float32, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewOffset(dx, dy, in, cropRect))
}

// NewTileImageFilter returns a new tile image filter. input may be nil, in which case the source bitmap will be used.
func NewTileImageFilter(src, dst Rect, input *ImageFilter) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewTile(&src, &dst, in))
}

// NewDilateImageFilter returns a new dilate image filter. input may be nil, in which case the source bitmap will be
// used. cropRect may be nil.
func NewDilateImageFilter(radiusX, radiusY int, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewDilate(radiusX, radiusY, in, cropRect))
}

// NewErodeImageFilter returns a new erode image filter. input may be nil, in which case the source bitmap will be
// used. cropRect may be nil.
func NewErodeImageFilter(radiusX, radiusY int, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewErode(radiusX, radiusY, in, cropRect))
}

// NewDistantLitDiffuseImageFilter returns a new distant lit diffuse image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewDistantLitDiffuseImageFilter(x, y, z, scale, reflectivity float32, color Color, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewDistantLitDiffuse(&skia.Point3{
		X: x,
		Y: y,
		Z: z,
	}, skia.Color(color), scale, reflectivity, in, cropRect))
}

// NewPointLitDiffuseImageFilter returns a new point lit diffuse image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewPointLitDiffuseImageFilter(x, y, z, scale, reflectivity float32, color Color, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewPointLitDiffuse(&skia.Point3{
		X: x,
		Y: y,
		Z: z,
	}, skia.Color(color), scale, reflectivity, in, cropRect))
}

// NewSpotLitDiffuseImageFilter returns a new spot lit diffuse image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewSpotLitDiffuseImageFilter(x, y, z, targetX, targetY, targetZ, specularExponent, cutoffAngle, scale, reflectivity float32, color Color, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewSpotLitDiffuse(&skia.Point3{
		X: x,
		Y: y,
		Z: z,
	}, &skia.Point3{
		X: targetX,
		Y: targetY,
		Z: targetZ,
	}, specularExponent, cutoffAngle, scale, reflectivity, skia.Color(color), in, cropRect))
}

// NewDistantLitSpecularImageFilter returns a new distant lit specular image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewDistantLitSpecularImageFilter(x, y, z, scale, reflectivity, shine float32, color Color, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewDistantLitSpecular(&skia.Point3{
		X: x,
		Y: y,
		Z: z,
	}, skia.Color(color), scale, reflectivity, shine, in, cropRect))
}

// NewPointLitSpecularImageFilter returns a new point lit specular image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewPointLitSpecularImageFilter(x, y, z, scale, reflectivity, shine float32, color Color, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewPointLitSpecular(&skia.Point3{
		X: x,
		Y: y,
		Z: z,
	}, skia.Color(color), scale, reflectivity, shine, in, cropRect))
}

// NewSpotLitSpecularImageFilter returns a new spot lit specular image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewSpotLitSpecularImageFilter(x, y, z, targetX, targetY, targetZ, specularExponent, cutoffAngle, scale, reflectivity, shine float32, color Color, input *ImageFilter, cropRect *Rect) *ImageFilter {
	var in skia.ImageFilter
	if input != nil {
		in = input.filter
	}
	return newImageFilter(skia.ImageFilterNewSpotLitSpecular(&skia.Point3{
		X: x,
		Y: y,
		Z: z,
	}, &skia.Point3{
		X: targetX,
		Y: targetY,
		Z: targetZ,
	}, specularExponent, cutoffAngle, scale, reflectivity, shine, skia.Color(color), in, cropRect))
}
