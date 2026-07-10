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
	skgeom "github.com/richardwilkes/canvas/geom"
	"github.com/richardwilkes/canvas/imagefilter"
	"github.com/richardwilkes/canvas/shaders"
	"github.com/richardwilkes/canvas/skcolor"
	"github.com/richardwilkes/canvas/skif"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/colorchannel"
	"github.com/richardwilkes/unison/enums/tilemode"
)

// ImageFilter performs a transformation on the image before drawing it.
type ImageFilter struct {
	filter skif.Filter
}

func newImageFilter(filter skif.Filter) *ImageFilter {
	if filter == nil {
		return nil
	}
	return &ImageFilter{filter: filter}
}

func (f *ImageFilter) filterOrNil() skif.Filter {
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
func NewArithmeticImageFilter(k1, k2, k3, k4 float32, background, foreground *ImageFilter, enforcePMColor bool, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.Arithmetic(k1, k2, k3, k4, enforcePMColor, background.filterOrNil(),
		foreground.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewBlurImageFilter returns a new blur image filter. input may be nil, in which case the source bitmap is used.
// cropRect may be nil.
func NewBlurImageFilter(sigmaX, sigmaY float32, tileMode tilemode.Enum, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.Blur(sigmaX, sigmaY, shaders.TileMode(tileMode), input.filterOrNil(),
		toSkRectPtr(cropRect)))
}

// NewColorImageFilter returns a new color image filter. input may be nil, in which case the source bitmap is used.
// cropRect may be nil.
func NewColorImageFilter(colorFilter *ColorFilter, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.ColorFilter(colorFilter.filter, input.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewComposeImageFilter returns a new combining image filter.
func NewComposeImageFilter(outer, inner *ImageFilter) *ImageFilter {
	return newImageFilter(imagefilter.Compose(outer.filter, inner.filter))
}

// NewDisplacementImageFilter returns a new displacement image filter. displayment may be nil, in which case the source
// bitmap will be used. cropRect may be nil.
func NewDisplacementImageFilter(xChannelSelector, yChannelSelector colorchannel.Enum, scale float32, displacement, color *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.DisplacementMap(imagefilter.ColorChannel(xChannelSelector),
		imagefilter.ColorChannel(yChannelSelector), scale, displacement.filterOrNil(), color.filter,
		toSkRectPtr(cropRect)))
}

// NewDropShadowImageFilter returns a new drop shadow image filter. input may be nil, in which case the source bitmap
// will be used. cropRect may be nil.
func NewDropShadowImageFilter(dx, dy, sigmaX, sigmaY float32, color Color, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.DropShadow(dx, dy, sigmaX, sigmaY, skcolor.Color(color), input.filterOrNil(),
		toSkRectPtr(cropRect)))
}

// NewDropShadowOnlyImageFilter returns a new drop shadow only image filter. input may be nil, in which case the source
// bitmap will be used. cropRect may be nil.
func NewDropShadowOnlyImageFilter(dx, dy, sigmaX, sigmaY float32, color Color, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.DropShadowOnly(dx, dy, sigmaX, sigmaY, skcolor.Color(color), input.filterOrNil(),
		toSkRectPtr(cropRect)))
}

// NewImageSourceImageFilter returns a new image source image filter. If canvas is not nil, a hardware-accellerated
// image will be used if possible.
func NewImageSourceImageFilter(canvas *Canvas, img *Image, srcRect, dstRect geom.Rect, sampling *SamplingOptions) *ImageFilter {
	return newImageFilter(imagefilter.Image(img.imageForCanvas(canvas), toSkRect(srcRect), toSkRect(dstRect),
		sampling.skSamplingOptions()))
}

// NewImageSourceDefaultImageFilter returns a new image source image filter that uses the default quality and the full
// image size. If canvas is not nil, a hardware-accellerated image will be used if possible.
func NewImageSourceDefaultImageFilter(canvas *Canvas, img *Image, sampling *SamplingOptions) *ImageFilter {
	return newImageFilter(imagefilter.ImageDefault(img.imageForCanvas(canvas), sampling.skSamplingOptions()))
}

// NewMagnifierImageFilter returns a new magnifier image filter. input may be nil, in which case the source bitmap will
// be used. cropRect may be nil.
func NewMagnifierImageFilter(lensBounds geom.Rect, zoomAmount, inset float32, sampling *SamplingOptions, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.Magnifier(toSkRect(lensBounds), zoomAmount, inset,
		sampling.skSamplingOptions(), input.filterOrNil(), toSkRectPtr(cropRect)))
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
func NewMatrixConvolutionImageFilter(width, height int, kernel []float32, gain, bias float32, offsetX, offsetY int, tileMode tilemode.Enum, convolveAlpha bool, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	if len(kernel) < width*height {
		k := make([]float32, width*height)
		copy(k, kernel)
		kernel = k
	}
	return newImageFilter(imagefilter.MatrixConvolution(skgeom.ISize{Width: int32(width), Height: int32(height)},
		kernel, gain, bias, skgeom.IPoint{X: int32(offsetX), Y: int32(offsetY)}, shaders.TileMode(tileMode),
		convolveAlpha, input.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewMatrixTransformImageFilter returns a new matrix transform image filter. input may be nil, in which case the source
// bitmap will be used.
func NewMatrixTransformImageFilter(matrix geom.Matrix, sampling *SamplingOptions, input *ImageFilter) *ImageFilter {
	return newImageFilter(imagefilter.MatrixTransform(toSkMatrixPtr(matrix), sampling.skSamplingOptions(),
		input.filterOrNil()))
}

// NewMergeImageFilter returns a new merge image filter. Each filter will draw their results in order with src-over
// blending. A nil filter will use the source bitmap instead. cropRect may be nil.
func NewMergeImageFilter(filters []*ImageFilter, cropRect *geom.Rect) *ImageFilter {
	ff := make([]skif.Filter, len(filters))
	for i, one := range filters {
		if one != nil {
			ff[i] = one.filter
		}
	}
	return newImageFilter(imagefilter.Merge(ff, toSkRectPtr(cropRect)))
}

// NewOffsetImageFilter returns a new offset image filter. input may be nil, in which case the source bitmap will be
// used. cropRect may be nil.
func NewOffsetImageFilter(dx, dy float32, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.Offset(dx, dy, input.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewTileImageFilter returns a new tile image filter. input may be nil, in which case the source bitmap will be used.
func NewTileImageFilter(src, dst geom.Rect, input *ImageFilter) *ImageFilter {
	return newImageFilter(imagefilter.Tile(toSkRect(src), toSkRect(dst), input.filterOrNil()))
}

// NewDilateImageFilter returns a new dilate image filter. input may be nil, in which case the source bitmap will be
// used. cropRect may be nil.
func NewDilateImageFilter(radiusX, radiusY float32, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.Dilate(radiusX, radiusY, input.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewErodeImageFilter returns a new erode image filter. input may be nil, in which case the source bitmap will be
// used. cropRect may be nil.
func NewErodeImageFilter(radiusX, radiusY float32, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.Erode(radiusX, radiusY, input.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewDistantLitDiffuseImageFilter returns a new distant lit diffuse image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewDistantLitDiffuseImageFilter(x, y, z, scale, reflectivity float32, color Color, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.DistantLitDiffuse(skgeom.Point3{X: x, Y: y, Z: z}, skcolor.Color(color), scale,
		reflectivity, input.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewPointLitDiffuseImageFilter returns a new point lit diffuse image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewPointLitDiffuseImageFilter(x, y, z, scale, reflectivity float32, color Color, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.PointLitDiffuse(skgeom.Point3{X: x, Y: y, Z: z}, skcolor.Color(color), scale,
		reflectivity, input.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewSpotLitDiffuseImageFilter returns a new spot lit diffuse image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewSpotLitDiffuseImageFilter(x, y, z, targetX, targetY, targetZ, specularExponent, cutoffAngle, scale, reflectivity float32, color Color, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.SpotLitDiffuse(skgeom.Point3{X: x, Y: y, Z: z},
		skgeom.Point3{X: targetX, Y: targetY, Z: targetZ}, specularExponent, cutoffAngle, skcolor.Color(color), scale,
		reflectivity, input.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewDistantLitSpecularImageFilter returns a new distant lit specular image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewDistantLitSpecularImageFilter(x, y, z, scale, reflectivity, shine float32, color Color, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.DistantLitSpecular(skgeom.Point3{X: x, Y: y, Z: z}, skcolor.Color(color), scale,
		reflectivity, shine, input.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewPointLitSpecularImageFilter returns a new point lit specular image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewPointLitSpecularImageFilter(x, y, z, scale, reflectivity, shine float32, color Color, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.PointLitSpecular(skgeom.Point3{X: x, Y: y, Z: z}, skcolor.Color(color), scale,
		reflectivity, shine, input.filterOrNil(), toSkRectPtr(cropRect)))
}

// NewSpotLitSpecularImageFilter returns a new spot lit specular image filter. input may be nil, in which case the
// source bitmap will be used. cropRect may be nil.
func NewSpotLitSpecularImageFilter(x, y, z, targetX, targetY, targetZ, specularExponent, cutoffAngle, scale, reflectivity, shine float32, color Color, input *ImageFilter, cropRect *geom.Rect) *ImageFilter {
	return newImageFilter(imagefilter.SpotLitSpecular(skgeom.Point3{X: x, Y: y, Z: z},
		skgeom.Point3{X: targetX, Y: targetY, Z: targetZ}, specularExponent, cutoffAngle, skcolor.Color(color), scale,
		reflectivity, shine, input.filterOrNil(), toSkRectPtr(cropRect)))
}
