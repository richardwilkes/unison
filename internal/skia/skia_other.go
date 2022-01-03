// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build !windows

package skia

/*
#cgo CFLAGS: -I${SRCDIR}
#cgo darwin LDFLAGS: -L${SRCDIR} -lc++ -framework Cocoa -framework Metal
#cgo darwin,amd64 LDFLAGS: -lskia_darwin_amd64
#cgo darwin,arm64 LDFLAGS: -lskia_darwin_arm64
#cgo linux LDFLAGS: -L${SRCDIR} -lskia_linux -lfontconfig -lfreetype -lGL -ldl -lm -lstdc++

#include <stdlib.h>
#include "sk_capi.h"
*/
import "C"

import (
	"unsafe"

	"github.com/richardwilkes/toolbox/xmath/geom32"
)

type (
	BackendRenderTarget = *C.gr_backendrendertarget_t
	DirectContext       = *C.gr_direct_context_t
	GLInterface         = *C.gr_glinterface_t
	Canvas              = *C.sk_canvas_t
	ColorFilter         = *C.sk_color_filter_t
	ColorSpace          = *C.sk_color_space_t
	Data                = *C.sk_data_t
	Font                = *C.sk_font_t
	FontMgr             = *C.sk_font_mgr_t
	FontStyle           = *C.sk_font_style_t
	FontStyleSet        = *C.sk_font_style_set_t
	Image               = *C.sk_image_t
	ImageFilter         = *C.sk_image_filter_t
	MaskFilter          = *C.sk_mask_filter_t
	OpBuilder           = *C.sk_op_builder_t
	Paint               = *C.sk_paint_t
	Path                = *C.sk_path_t
	PathEffect          = *C.sk_path_effect_t
	SamplingOptions     = *C.sk_sampling_options_t
	Shader              = *C.sk_shader_t
	String              = *C.sk_string_t
	Surface             = *C.sk_surface_t
	SurfaceProps        = *C.sk_surface_props_t
	TextBlob            = *C.sk_text_blob_t
	TextBlobBuilder     = *C.sk_text_blob_builder_t
	TypeFace            = *C.sk_typeface_t
)

func BackendRenderTargetNewGL(width, height, samples, stencilBits int, info *GLFrameBufferInfo) BackendRenderTarget {
	return C.gr_backendrendertarget_new_gl(C.int(width), C.int(height), C.int(samples), C.int(stencilBits),
		(*C.gr_gl_framebufferinfo_t)(unsafe.Pointer(info)))
}

func BackendRenderTargetDelete(backend BackendRenderTarget) {
	C.gr_backendrendertarget_delete(backend)
}

func ContextMakeGL(gl GLInterface) DirectContext {
	return C.gr_direct_context_make_gl(gl)
}

func ContextAbandonContext(ctx DirectContext) {
	C.gr_direct_context_abandon_context(ctx)
}

func GLInterfaceCreateNativeInterface() GLInterface {
	return C.gr_glinterface_create_native_interface()
}

func CanvasGetSaveCount(canvas Canvas) int {
	return int(C.sk_canvas_get_save_count(canvas))
}

func CanvasSave(canvas Canvas) int {
	return int(C.sk_canvas_save(canvas))
}

func CanvasSaveLayer(canvas Canvas, paint Paint) int {
	return int(C.sk_canvas_save_layer(canvas, nil, paint))
}

func CanvasSaveLayerAlpha(canvas Canvas, opacity byte) int {
	return int(C.sk_canvas_save_layer_alpha(canvas, nil, C.uint8_t(opacity)))
}

func CanvasRestore(canvas Canvas) {
	C.sk_canvas_restore(canvas)
}

func CanvasRestoreToCount(canvas Canvas, count int) {
	C.sk_canvas_restore_to_count(canvas, C.int(count))
}

func CanvasTranslate(canvas Canvas, dx, dy float32) {
	C.sk_canvas_translate(canvas, C.float(dx), C.float(dy))
}

func CanvasScale(canvas Canvas, xScale, yScale float32) {
	C.sk_canvas_scale(canvas, C.float(xScale), C.float(yScale))
}

func CanvasRotateRadians(canvas Canvas, radians float32) {
	C.sk_canvas_rotate_radians(canvas, C.float(radians))
}

func CanvasSkew(canvas Canvas, sx, sy float32) {
	C.sk_canvas_skew(canvas, C.float(sx), C.float(sy))
}

func CanvasConcat(canvas Canvas, matrix *Matrix) {
	C.sk_canvas_concat(canvas, (*C.sk_matrix_t)(unsafe.Pointer(matrix)))
}

func CanvasResetMatrix(canvas Canvas) {
	C.sk_canvas_reset_matrix(canvas)
}

func CanvasGetTotalMatrix(canvas Canvas) *Matrix {
	var matrix Matrix
	C.sk_canvas_get_total_matrix(canvas, (*C.sk_matrix_t)(unsafe.Pointer(&matrix)))
	return &matrix
}

func CanvasSetMatrix(canvas Canvas, matrix *Matrix) {
	C.sk_canvas_set_matrix(canvas, (*C.sk_matrix_t)(unsafe.Pointer(matrix)))
}

func CanvasQuickRejectPath(canvas Canvas, path Path) bool {
	return bool(C.sk_canvas_quick_reject_path(canvas, path))
}

func CanvasQuickRejectRect(canvas Canvas, rect *Rect) bool {
	return bool(C.sk_canvas_quick_reject_rect(canvas, (*C.sk_rect_t)(unsafe.Pointer(rect))))
}

func CanvasClear(canvas Canvas, color Color) {
	C.sk_canvas_clear(canvas, C.sk_color_t(color))
}

func CanvasDrawPaint(canvas Canvas, paint Paint) {
	C.sk_canvas_draw_paint(canvas, paint)
}

func CanvasDrawRect(canvas Canvas, rect *Rect, paint Paint) {
	C.sk_canvas_draw_rect(canvas, (*C.sk_rect_t)(unsafe.Pointer(rect)), paint)
}

func CanvasDrawRoundRect(canvas Canvas, rect *Rect, radiusX, radiusY float32, paint Paint) {
	C.sk_canvas_draw_round_rect(canvas, (*C.sk_rect_t)(unsafe.Pointer(rect)), C.float(radiusX), C.float(radiusY), paint)
}

func CanvasDrawCircle(canvas Canvas, centerX, centerY, radius float32, paint Paint) {
	C.sk_canvas_draw_circle(canvas, C.float(centerX), C.float(centerY), C.float(radius), paint)
}

func CanvasDrawOval(canvas Canvas, rect *Rect, paint Paint) {
	C.sk_canvas_draw_oval(canvas, (*C.sk_rect_t)(unsafe.Pointer(rect)), paint)
}

func CanvasDrawPath(canvas Canvas, path Path, paint Paint) {
	C.sk_canvas_draw_path(canvas, path, paint)
}

func CanvasDrawImageRect(canvas Canvas, img Image, srcRect, dstRect *Rect, sampling SamplingOptions, paint Paint) {
	C.sk_canvas_draw_image_rect(canvas, img, (*C.sk_rect_t)(unsafe.Pointer(srcRect)),
		(*C.sk_rect_t)(unsafe.Pointer(dstRect)), sampling, paint, C.SRC_RECT_CONSTRAINT_STRICT)
}

func CanvasDrawImageNine(canvas Canvas, img Image, centerRect *IRect, dstRect *Rect, filter FilterMode, paint Paint) {
	C.sk_canvas_draw_image_nine(canvas, img, (*C.sk_irect_t)(unsafe.Pointer(centerRect)),
		(*C.sk_rect_t)(unsafe.Pointer(dstRect)), C.sk_filter_mode_t(filter), paint)
}

func CanvasDrawColor(canvas Canvas, color Color, mode BlendMode) {
	C.sk_canvas_draw_color(canvas, C.sk_color_t(color), C.sk_blend_mode_t(mode))
}

func CanvasDrawPoint(canvas Canvas, x, y float32, paint Paint) {
	C.sk_canvas_draw_point(canvas, C.float(x), C.float(y), paint)
}

func CanvasDrawPoints(canvas Canvas, mode PointMode, pts []geom32.Point, paint Paint) {
	C.sk_canvas_draw_points(canvas, C.sk_point_mode_t(mode), C.size_t(len(pts)),
		(*C.sk_point_t)(unsafe.Pointer(&pts[0])), paint)
}

func CanvasDrawLine(canvas Canvas, sx, sy, ex, ey float32, paint Paint) {
	C.sk_canvas_draw_line(canvas, C.float(sx), C.float(sy), C.float(ex), C.float(ey), paint)
}

func CanvasDrawArc(canvas Canvas, oval *Rect, startAngle, sweepAngle float32, useCenter bool, paint Paint) {
	C.sk_canvas_draw_arc(canvas, (*C.sk_rect_t)(unsafe.Pointer(oval)), C.float(startAngle), C.float(sweepAngle),
		C.bool(useCenter), paint)
}

func CanvasDrawSimpleText(canvas Canvas, str string, x, y float32, font Font, paint Paint) {
	b := []byte(str)
	C.sk_canvas_draw_simple_text(canvas, unsafe.Pointer(&b[0]), C.size_t(len(b)),
		C.sk_text_encoding_t(TextEncodingUTF8), C.float(x), C.float(y), font, paint)
}

func CanvasDrawTextBlob(canvas Canvas, txt TextBlob, x, y float32, paint Paint) {
	C.sk_canvas_draw_text_blob(canvas, txt, C.float(x), C.float(y), paint)
}

func CanavasClipRectWithOperation(canvas Canvas, rect *Rect, op ClipOp, antialias bool) {
	C.sk_canvas_clip_rect_with_operation(canvas, (*C.sk_rect_t)(unsafe.Pointer(rect)), C.sk_clip_op_t(op),
		C.bool(antialias))
}

func CanavasClipPathWithOperation(canvas Canvas, path Path, op ClipOp, antialias bool) {
	C.sk_canvas_clip_path_with_operation(canvas, path, C.sk_clip_op_t(op), C.bool(antialias))
}

func CanvasGetLocalClipBounds(canvas Canvas) *Rect {
	var rect Rect
	C.sk_canvas_get_local_clip_bounds(canvas, (*C.sk_rect_t)(unsafe.Pointer(&rect)))
	return &rect
}

func CanvasIsClipEmpty(canvas Canvas) bool {
	return bool(C.sk_canvas_is_clip_empty(canvas))
}

func CanvasIsClipRect(canvas Canvas) bool {
	return bool(C.sk_canvas_is_clip_rect(canvas))
}

func CanvasFlush(canvas Canvas) {
	C.sk_canvas_flush(canvas)
}

func ColorFilterNewMode(color Color, blendMode BlendMode) ColorFilter {
	return C.sk_colorfilter_new_mode(C.sk_color_t(color), C.sk_blend_mode_t(blendMode))
}

func ColorFilterNewLighting(mul, add Color) ColorFilter {
	return C.sk_colorfilter_new_lighting(C.sk_color_t(mul), C.sk_color_t(add))
}

func ColorFilterNewCompose(outer, inner ColorFilter) ColorFilter {
	return C.sk_colorfilter_new_compose(outer, inner)
}

func ColorFilterNewColorMatrix(array []float32) ColorFilter {
	return C.sk_colorfilter_new_color_matrix((*C.float)(unsafe.Pointer(&array[0])))
}

func ColorFilterNewLumaColor() ColorFilter {
	return C.sk_colorfilter_new_luma_color()
}

func ColorFilterNewHighContrast(config *HighContrastConfig) ColorFilter {
	return C.sk_colorfilter_new_high_contrast((*C.sk_high_contrast_config_t)(unsafe.Pointer(config)))
}

func ColorFilterNewTableARGB(a, r, g, b []byte) ColorFilter {
	return C.sk_colorfilter_new_table_argb((*C.uint8_t)(unsafe.Pointer(&a[0])), (*C.uint8_t)(unsafe.Pointer(&r[0])),
		(*C.uint8_t)(unsafe.Pointer(&g[0])), (*C.uint8_t)(unsafe.Pointer(&b[0])))
}

func ColorFilterUnref(filter ColorFilter) {
	C.sk_colorfilter_unref(filter)
}

func ColorSpaceNewSRGB() ColorSpace {
	return C.sk_colorspace_new_srgb()
}

func DataNewWithCopy(data []byte) Data {
	return C.sk_data_new_with_copy(unsafe.Pointer(&data[0]), C.size_t(len(data)))
}

func DataGetSize(data Data) int {
	return int(C.sk_data_get_size(data))
}

func DataGetData(data Data) unsafe.Pointer {
	return C.sk_data_get_data(data)
}

func DataUnref(data Data) {
	C.sk_data_unref(data)
}

func FontNewWithValues(face TypeFace, size, scaleX, skewX float32) Font {
	return C.sk_font_new_with_values(face, C.float(size), C.float(scaleX), C.float(skewX))
}

func FontSetSubPixel(font Font, enabled bool) {
	C.sk_font_set_subpixel(font, C.bool(enabled))
}

func FontSetForceAutoHinting(font Font, enabled bool) {
	C.sk_font_set_force_auto_hinting(font, C.bool(enabled))
}

func FontSetHinting(font Font, hinting FontHinting) {
	C.sk_font_set_hinting(font, C.sk_font_hinting_t(hinting))
}

func FontGetMetrics(font Font, metrics *FontMetrics) {
	C.sk_font_get_metrics(font, (*C.sk_font_metrics_t)(unsafe.Pointer(metrics)))
}

func FontMeasureText(font Font, str string) float32 {
	b := []byte(str)
	return float32(C.sk_font_measure_text(font, unsafe.Pointer(&b[0]), C.size_t(len(b)), C.sk_text_encoding_t(TextEncodingUTF8), nil, nil))
}

func FontTextToGlyphs(font Font, str string) []uint16 {
	b := []byte(str)
	glyphs := make([]uint16, len(str))
	count := C.sk_font_text_to_glyphs(font, unsafe.Pointer(&b[0]), C.size_t(len(b)), C.sk_text_encoding_t(TextEncodingUTF8), (*C.ushort)(&glyphs[0]), C.int(len(glyphs)))
	glyphs = glyphs[:count]
	return glyphs
}

func FontGetXPos(font Font, str string) []float32 {
	glyphs := FontTextToGlyphs(font, str+"a")
	pos := make([]float32, len(glyphs))
	C.sk_font_get_xpos(font, (*C.ushort)(&glyphs[0]), C.int(len(glyphs)), (*C.float)(unsafe.Pointer(&pos[0])), 0)
	return pos
}

func FontDelete(font Font) {
	C.sk_font_delete(font)
}

func FontMgrRefDefault() FontMgr {
	return C.sk_fontmgr_ref_default()
}

func FontMgrCreateFromData(mgr FontMgr, data Data) TypeFace {
	return C.sk_fontmgr_create_from_data(mgr, data, 0)
}

func FontMgrMatchFamily(mgr FontMgr, family string) FontStyleSet {
	cFamily := C.CString(family)
	defer C.free(unsafe.Pointer(cFamily))
	return C.sk_fontmgr_match_family(mgr, cFamily)
}

func FontMgrMatchFamilyStyle(mgr FontMgr, family string, style FontStyle) TypeFace {
	cFamily := C.CString(family)
	defer C.free(unsafe.Pointer(cFamily))
	return C.sk_fontmgr_match_family_style(mgr, cFamily, style)
}

func FontMgrCountFamilies(mgr FontMgr) int {
	return int(C.sk_fontmgr_count_families(mgr))
}

func FontMgrGetFamilyName(mgr FontMgr, index int, str String) {
	C.sk_fontmgr_get_family_name(mgr, C.int(index), str)
}

func FontStyleNew(weight FontWeight, spacing FontSpacing, slant FontSlant) FontStyle {
	return C.sk_fontstyle_new(C.int(weight), C.int(spacing), C.sk_font_style_slant_t(slant))
}

func FontStyleGetWeight(style FontStyle) FontWeight {
	return FontWeight(C.sk_fontstyle_get_weight(style))
}

func FontStyleGetWidth(style FontStyle) FontSpacing {
	return FontSpacing(C.sk_fontstyle_get_width(style))
}

func FontStyleGetSlant(style FontStyle) FontSlant {
	return FontSlant(C.sk_fontstyle_get_slant(style))
}

func FontStyleDelete(style FontStyle) {
	C.sk_fontstyle_delete(style)
}

func FontStyleSetGetCount(set FontStyleSet) int {
	return int(C.sk_fontstyleset_get_count(set))
}

func FontStyleSetGetStyle(set FontStyleSet, index int, style FontStyle, str String) {
	C.sk_fontstyleset_get_style(set, C.int(index), style, str)
}

func FontStyleSetCreateTypeFace(set FontStyleSet, index int) TypeFace {
	return C.sk_fontstyleset_create_typeface(set, C.int(index))
}

func FontStyleSetMatchStyle(set FontStyleSet, style FontStyle) TypeFace {
	return C.sk_fontstyleset_match_style(set, style)
}

func FontStyleSetUnref(set FontStyleSet) {
	C.sk_fontstyleset_unref(set)
}

func ImageNewFromEncoded(data Data) Image {
	return C.sk_image_new_from_encoded(data)
}

func ImageNewRasterData(info *ImageInfo, data Data, rowBytes int) Image {
	return C.sk_image_new_raster_data((*C.sk_image_info_t)(unsafe.Pointer(info)), data, C.size_t(rowBytes))
}

func ImageGetWidth(img Image) int {
	return int(C.sk_image_get_width(img))
}

func ImageGetHeight(img Image) int {
	return int(C.sk_image_get_height(img))
}

func ImageGetColorSpace(img Image) ColorSpace {
	return C.sk_image_get_colorspace(img)
}

func ImageGetColorType(img Image) ColorType {
	return ColorType(C.sk_image_get_color_type(img))
}

func ImageGetAlphaType(img Image) AlphaType {
	return AlphaType(C.sk_image_get_alpha_type(img))
}

func ImageReadPixels(img Image, info *ImageInfo, pixels []byte, dstRowBytes, srcX, srcY int, cachingHint ImageCachingHint) bool {
	return bool(C.sk_image_read_pixels(img, (*C.sk_image_info_t)(unsafe.Pointer(info)), unsafe.Pointer(&pixels[0]),
		C.size_t(dstRowBytes), C.int(srcX), C.int(srcY), C.sk_image_caching_hint_t(cachingHint)))
}

func ImageEncodeSpecific(img Image, format EncodedImageFormat, quality int) Data {
	return C.sk_image_encode_specific(img, C.sk_encoded_image_format_t(format), C.int(quality))
}

func ImageMakeShader(img Image, tileModeX, tileModeY TileMode, sampling SamplingOptions, matrix *Matrix) Shader {
	return C.sk_image_make_shader(img, C.sk_tile_mode_t(tileModeX), C.sk_tile_mode_t(tileModeY),
		sampling, (*C.sk_matrix_t)(unsafe.Pointer(matrix)))
}

func ImageMakeTextureImage(img Image, ctx DirectContext, mipMapped bool) Image {
	return C.sk_image_make_texture_image(img, ctx, C.bool(mipMapped))
}

func ImageUnref(img Image) {
	C.sk_image_unref(img)
}

func optionalCropRect(cropRect *geom32.Rect) *C.sk_rect_t {
	if cropRect == nil {
		cropRect = &geom32.Rect{
			Size: geom32.Size{
				Width:  32767,
				Height: 32767,
			},
		}
	}
	return (*C.sk_rect_t)(unsafe.Pointer(RectToSkRect(cropRect)))
}

func ImageFilterNewArithmetic(k1, k2, k3, k4 float32, enforcePMColor bool, background, foreground ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_arithmetic(C.float(k1), C.float(k2), C.float(k3), C.float(k4), C.bool(enforcePMColor),
		background, foreground, optionalCropRect(cropRect))
}

func ImageFilterNewBlur(sigmaX, sigmaY float32, tileMode TileMode, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_blur(C.float(sigmaX), C.float(sigmaY), C.sk_tile_mode_t(tileMode), input,
		optionalCropRect(cropRect))
}

func ImageFilterNewColorFilter(colorFilter ColorFilter, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_color_filter(colorFilter, input, optionalCropRect(cropRect))
}

func ImageFilterNewCompose(outer, inner ImageFilter) ImageFilter {
	return C.sk_imagefilter_new_compose(outer, inner)
}

func ImageFilterNewDisplacementMapEffect(xChannelSelector, yChannelSelector ColorChannel, scale float32, displacement, color ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_displacement_map_effect(C.sk_color_channel_t(xChannelSelector),
		C.sk_color_channel_t(yChannelSelector), C.float(scale), displacement, color, optionalCropRect(cropRect))
}

func ImageFilterNewDropShadow(dx, dy, sigmaX, sigmaY float32, color Color, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_drop_shadow(C.float(dx), C.float(dy), C.float(sigmaX), C.float(sigmaY),
		C.sk_color_t(color), input, optionalCropRect(cropRect))
}

func ImageFilterNewDropShadowOnly(dx, dy, sigmaX, sigmaY float32, color Color, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_drop_shadow_only(C.float(dx), C.float(dy), C.float(sigmaX), C.float(sigmaY),
		C.sk_color_t(color), input, optionalCropRect(cropRect))
}

func ImageFilterNewImageSource(img Image, srcRect, dstRect *geom32.Rect, sampling SamplingOptions) ImageFilter {
	return C.sk_imagefilter_new_image_source(img, (*C.sk_rect_t)(unsafe.Pointer(RectToSkRect(srcRect))),
		(*C.sk_rect_t)(unsafe.Pointer(RectToSkRect(dstRect))), sampling)
}

func ImageFilterNewImageSourceDefault(img Image) ImageFilter {
	return C.sk_imagefilter_new_image_source_default(img)
}

func ImageFilterNewMagnifier(src *geom32.Rect, inset float32, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_magnifier((*C.sk_rect_t)(unsafe.Pointer(RectToSkRect(src))), C.float(inset), input,
		optionalCropRect(cropRect))
}

func ImageFilterNewMatrixConvolution(size *ISize, kernel []float32, gain, bias float32, offset *IPoint, tileMode TileMode, convolveAlpha bool, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_matrix_convolution((*C.sk_isize_t)(unsafe.Pointer(size)),
		(*C.float)(unsafe.Pointer(&kernel[0])), C.float(gain), C.float(bias), (*C.sk_ipoint_t)(unsafe.Pointer(offset)),
		C.sk_tile_mode_t(tileMode), C.bool(convolveAlpha), input, optionalCropRect(cropRect))
}

func ImageFilterNewMatrixTransform(matrix *Matrix, sampling SamplingOptions, input ImageFilter) ImageFilter {
	return C.sk_imagefilter_new_matrix_transform((*C.sk_matrix_t)(unsafe.Pointer(matrix)), sampling, input)
}

func ImageFilterNewMerge(filters []ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_merge((**C.sk_image_filter_t)(unsafe.Pointer(&filters[0])), C.int(len(filters)),
		optionalCropRect(cropRect))
}

func ImageFilterNewOffset(dx, dy float32, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_offset(C.float(dx), C.float(dy), input, optionalCropRect(cropRect))
}

func ImageFilterNewTile(src, dst *geom32.Rect, input ImageFilter) ImageFilter {
	return C.sk_imagefilter_new_tile((*C.sk_rect_t)(unsafe.Pointer(RectToSkRect(src))),
		(*C.sk_rect_t)(unsafe.Pointer(RectToSkRect(dst))), input)
}

func ImageFilterNewDilate(radiusX, radiusY int, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_dilate(C.int(radiusX), C.int(radiusY), input, optionalCropRect(cropRect))
}

func ImageFilterNewErode(radiusX, radiusY int, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_erode(C.int(radiusX), C.int(radiusY), input, optionalCropRect(cropRect))
}

func ImageFilterNewDistantLitDiffuse(pt *Point3, color Color, scale, reflectivity float32, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_distant_lit_diffuse((*C.sk_point3_t)(unsafe.Pointer(pt)), C.sk_color_t(color),
		C.float(scale), C.float(reflectivity), input, optionalCropRect(cropRect))
}

func ImageFilterNewPointLitDiffuse(pt *Point3, color Color, scale, reflectivity float32, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_point_lit_diffuse((*C.sk_point3_t)(unsafe.Pointer(pt)), C.sk_color_t(color),
		C.float(scale), C.float(reflectivity), input, optionalCropRect(cropRect))
}

func ImageFilterNewSpotLitDiffuse(pt, targetPt *Point3, specularExponent, cutoffAngle, scale, reflectivity float32, color Color, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_spot_lit_diffuse((*C.sk_point3_t)(unsafe.Pointer(pt)),
		(*C.sk_point3_t)(unsafe.Pointer(targetPt)), C.float(specularExponent), C.float(cutoffAngle),
		C.sk_color_t(color), C.float(scale), C.float(reflectivity), input, optionalCropRect(cropRect))
}

func ImageFilterNewDistantLitSpecular(pt *Point3, color Color, scale, reflectivity, shine float32, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_distant_lit_specular((*C.sk_point3_t)(unsafe.Pointer(pt)), C.sk_color_t(color),
		C.float(scale), C.float(reflectivity), C.float(shine), input, optionalCropRect(cropRect))
}

func ImageFilterNewPointLitSpecular(pt *Point3, color Color, scale, reflectivity, shine float32, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_point_lit_specular((*C.sk_point3_t)(unsafe.Pointer(pt)), C.sk_color_t(color),
		C.float(scale), C.float(reflectivity), C.float(shine), input, optionalCropRect(cropRect))
}

func ImageFilterNewSpotLitSpecular(pt, targetPt *Point3, specularExponent, cutoffAngle, scale, reflectivity, shine float32, color Color, input ImageFilter, cropRect *geom32.Rect) ImageFilter {
	return C.sk_imagefilter_new_spot_lit_specular((*C.sk_point3_t)(unsafe.Pointer(pt)),
		(*C.sk_point3_t)(unsafe.Pointer(targetPt)), C.float(specularExponent), C.float(cutoffAngle),
		C.sk_color_t(color), C.float(scale), C.float(reflectivity), C.float(shine), input, optionalCropRect(cropRect))
}

func ImageFilterUnref(filter ImageFilter) {
	C.sk_imagefilter_unref(filter)
}

func MaskFilterNewBlurWithFlags(style Blur, sigma float32, respectMatrix bool) MaskFilter {
	return C.sk_maskfilter_new_blur_with_flags(C.sk_blur_style_t(style), C.float(sigma), C.bool(respectMatrix))
}

func MaskFilterNewTable(table []byte) MaskFilter {
	return C.sk_maskfilter_new_table((*C.uint8_t)(unsafe.Pointer(&table[0])))
}

func MaskFilterNewGamma(gamma float32) MaskFilter {
	return C.sk_maskfilter_new_gamma(C.float(gamma))
}

func MaskFilterNewClip(min, max byte) MaskFilter {
	return C.sk_maskfilter_new_clip(C.uint8_t(min), C.uint8_t(max))
}

func MaskFilterNewShader(shader Shader) MaskFilter {
	return C.sk_maskfilter_new_shader(shader)
}

func MaskFilterUnref(filter MaskFilter) {
	C.sk_maskfilter_unref(filter)
}

func OpBuilderNew() OpBuilder {
	return C.sk_opbuilder_new()
}

func OpBuilderAdd(builder OpBuilder, path Path, op PathOp) {
	C.sk_opbuilder_add(builder, path, C.sk_path_op_t(op))
}

func OpBuilderResolve(builder OpBuilder, path Path) bool {
	return bool(C.sk_opbuilder_resolve(builder, path))
}

func OpBuilderDestroy(builder OpBuilder) {
	C.sk_opbuilder_destroy(builder)
}

func PaintNew() Paint {
	return C.sk_paint_new()
}

func PaintDelete(paint Paint) {
	C.sk_paint_delete(paint)
}

func PaintClone(paint Paint) Paint {
	return C.sk_paint_clone(paint)
}

func PaintReset(paint Paint) {
	C.sk_paint_reset(paint)
}

func PaintIsAntialias(paint Paint) bool {
	return bool(C.sk_paint_is_antialias(paint))
}

func PaintSetAntialias(paint Paint, enabled bool) {
	C.sk_paint_set_antialias(paint, C.bool(enabled))
}

func PaintIsDither(paint Paint) bool {
	return bool(C.sk_paint_is_dither(paint))
}

func PaintSetDither(paint Paint, enabled bool) {
	C.sk_paint_set_dither(paint, C.bool(enabled))
}

func PaintGetColor(paint Paint) Color {
	return Color(C.sk_paint_get_color(paint))
}

func PaintSetColor(paint Paint, color Color) {
	C.sk_paint_set_color(paint, C.sk_color_t(color))
}

func PaintGetStyle(paint Paint) PaintStyle {
	return PaintStyle(C.sk_paint_get_style(paint))
}

func PaintSetStyle(paint Paint, style PaintStyle) {
	C.sk_paint_set_style(paint, C.sk_paint_style_t(style))
}

func PaintGetStrokeWidth(paint Paint) float32 {
	return float32(C.sk_paint_get_stroke_width(paint))
}

func PaintSetStrokeWidth(paint Paint, width float32) {
	C.sk_paint_set_stroke_width(paint, C.float(width))
}

func PaintGetStrokeMiter(paint Paint) float32 {
	return float32(C.sk_paint_get_stroke_miter(paint))
}

func PaintSetStrokeMiter(paint Paint, miter float32) {
	C.sk_paint_set_stroke_miter(paint, C.float(miter))
}

func PaintGetStrokeCap(paint Paint) StrokeCap {
	return StrokeCap(C.sk_paint_get_stroke_cap(paint))
}

func PaintSetStrokeCap(paint Paint, strokeCap StrokeCap) {
	C.sk_paint_set_stroke_cap(paint, C.sk_stroke_cap_t(strokeCap))
}

func PaintGetStrokeJoin(paint Paint) StrokeJoin {
	return StrokeJoin(C.sk_paint_get_stroke_join(paint))
}

func PaintSetStrokeJoin(paint Paint, strokeJoin StrokeJoin) {
	C.sk_paint_set_stroke_join(paint, C.sk_stroke_join_t(strokeJoin))
}

func PaintGetBlendMode(paint Paint) BlendMode {
	return BlendMode(C.sk_paint_get_blend_mode_or(paint, C.SK_BLEND_MODE_SRCOVER))
}

func PaintSetBlendMode(paint Paint, blendMode BlendMode) {
	C.sk_paint_set_blend_mode(paint, C.sk_blend_mode_t(blendMode))
}

func PaintGetShader(paint Paint) Shader {
	return C.sk_paint_get_shader(paint)
}

func PaintSetShader(paint Paint, shader Shader) {
	C.sk_paint_set_shader(paint, shader)
}

func PaintGetColorFilter(paint Paint) ColorFilter {
	return C.sk_paint_get_colorfilter(paint)
}

func PaintSetColorFilter(paint Paint, filter ColorFilter) {
	C.sk_paint_set_colorfilter(paint, filter)
}

func PaintGetMaskFilter(paint Paint) MaskFilter {
	return C.sk_paint_get_maskfilter(paint)
}

func PaintSetMaskFilter(paint Paint, filter MaskFilter) {
	C.sk_paint_set_maskfilter(paint, filter)
}

func PaintGetImageFilter(paint Paint) ImageFilter {
	return C.sk_paint_get_imagefilter(paint)
}

func PaintSetImageFilter(paint Paint, filter ImageFilter) {
	C.sk_paint_set_imagefilter(paint, filter)
}

func PaintGetPathEffect(paint Paint) PathEffect {
	return C.sk_paint_get_path_effect(paint)
}

func PaintSetPathEffect(paint Paint, effect PathEffect) {
	C.sk_paint_set_path_effect(paint, effect)
}

func PaintGetFillPath(paint Paint, inPath, outPath Path, cullRect *Rect, resScale float32) bool {
	return bool(C.sk_paint_get_fill_path(paint, inPath, outPath, (*C.sk_rect_t)(unsafe.Pointer(cullRect)), C.float(resScale)))
}

func PathNew() Path {
	return C.sk_path_new()
}

func PathParseSVGString(path Path, svg string) bool {
	cstr := C.CString(svg)
	defer C.free(unsafe.Pointer(cstr))
	return bool(C.sk_path_parse_svg_string(path, cstr))
}

func PathToSVGString(path Path, str String) {
	C.sk_path_to_svg_string(path, str)
}

func PathGetFillType(path Path) FillType {
	return FillType(C.sk_path_get_filltype(path))
}

func PathSetFillType(path Path, fillType FillType) {
	C.sk_path_set_filltype(path, C.sk_path_fill_type_t(fillType))
}

func PathArcTo(path Path, x, y, rx, ry, rotation float32, arcSize ArcSize, direction Direction) {
	C.sk_path_arc_to(path, C.float(rx), C.float(ry), C.float(rotation), C.sk_path_arc_size_t(arcSize),
		C.sk_path_direction_t(direction), C.float(x), C.float(y))
}

func PathArcToWithPoints(path Path, x1, y1, x2, y2, radius float32) {
	C.sk_path_arc_to_with_points(path, C.float(x1), C.float(y1), C.float(x2), C.float(y2), C.float(radius))
}

func PathArcToWithOval(path Path, rect *Rect, startAngle, sweepAngle float32, forceMoveTo bool) {
	C.sk_path_arc_to_with_oval(path, (*C.sk_rect_t)(unsafe.Pointer(rect)), C.float(startAngle), C.float(sweepAngle),
		C.bool(forceMoveTo))
}

func PathRArcTo(path Path, dx, dy, rx, ry, rotation float32, arcSize ArcSize, direction Direction) {
	C.sk_path_rarc_to(path, C.float(rx), C.float(ry), C.float(rotation), C.sk_path_arc_size_t(arcSize),
		C.sk_path_direction_t(direction), C.float(dx), C.float(dy))
}

func PathGetBounds(path Path) *Rect {
	var r Rect
	C.sk_path_get_bounds(path, (*C.sk_rect_t)(unsafe.Pointer(&r)))
	return &r
}

func PathComputeTightBounds(path Path) *Rect {
	var r Rect
	C.sk_path_compute_tight_bounds(path, (*C.sk_rect_t)(unsafe.Pointer(&r)))
	return &r
}

func PathAddCircle(path Path, x, y, radius float32, direction Direction) {
	C.sk_path_add_circle(path, C.float(x), C.float(y), C.float(radius), C.sk_path_direction_t(direction))
}

func PathClone(path Path) Path {
	return C.sk_path_clone(path)
}

func PathClose(path Path) {
	C.sk_path_close(path)
}

func PathConicTo(path Path, cpx, cpy, x, y, weight float32) {
	C.sk_path_conic_to(path, C.float(cpx), C.float(cpy), C.float(x), C.float(y), C.float(weight))
}

func PathRConicTo(path Path, cpdx, cpdy, dx, dy, weight float32) {
	C.sk_path_rconic_to(path, C.float(cpdx), C.float(cpdy), C.float(dx), C.float(dy), C.float(weight))
}

func PathCubicTo(path Path, cp1x, cp1y, cp2x, cp2y, x, y float32) {
	C.sk_path_cubic_to(path, C.float(cp1x), C.float(cp1y), C.float(cp2x), C.float(cp2y), C.float(x), C.float(y))
}

func PathRCubicTo(path Path, cp1dx, cp1dy, cp2dx, cp2dy, dx, dy float32) {
	C.sk_path_rcubic_to(path, C.float(cp1dx), C.float(cp1dy), C.float(cp2dx), C.float(cp2dy), C.float(dx), C.float(dy))
}

func PathLineTo(path Path, x, y float32) {
	C.sk_path_line_to(path, C.float(x), C.float(y))
}

func PathRLineTo(path Path, x, y float32) {
	C.sk_path_rline_to(path, C.float(x), C.float(y))
}

func PathMoveTo(path Path, x, y float32) {
	C.sk_path_move_to(path, C.float(x), C.float(y))
}

func PathRMoveTo(path Path, x, y float32) {
	C.sk_path_rmove_to(path, C.float(x), C.float(y))
}

func PathAddOval(path Path, rect *Rect, direction Direction) {
	C.sk_path_add_oval(path, (*C.sk_rect_t)(unsafe.Pointer(rect)), C.sk_path_direction_t(direction))
}

func PathAddPath(path, other Path, mode PathAddMode) {
	C.sk_path_add_path(path, other, C.sk_path_add_mode_t(mode))
}

func PathAddPathReverse(path, other Path) {
	C.sk_path_add_path_reverse(path, other)
}

func PathAddPathMatrix(path, other Path, matrix *Matrix, mode PathAddMode) {
	C.sk_path_add_path_matrix(path, other, (*C.sk_matrix_t)(unsafe.Pointer(matrix)), C.sk_path_add_mode_t(mode))
}

func PathAddPathOffset(path, other Path, offsetX, offsetY float32, mode PathAddMode) {
	C.sk_path_add_path_offset(path, other, C.float(offsetX), C.float(offsetY), C.sk_path_add_mode_t(mode))
}

func PathAddPoly(path Path, pts []geom32.Point, closePath bool) {
	C.sk_path_add_poly(path, (*C.sk_point_t)(unsafe.Pointer(&pts[0])), C.int(len(pts)), C.bool(closePath))
}

func PathQuadTo(path Path, cpx, cpy, x, y float32) {
	C.sk_path_quad_to(path, C.float(cpx), C.float(cpy), C.float(x), C.float(y))
}

func PathAddRect(path Path, rect *Rect, direction Direction) {
	C.sk_path_add_rect(path, (*C.sk_rect_t)(unsafe.Pointer(rect)), C.sk_path_direction_t(direction))
}

func PathAddRoundedRect(path Path, rect *Rect, radiusX, radiusY float32, direction Direction) {
	C.sk_path_add_rounded_rect(path, (*C.sk_rect_t)(unsafe.Pointer(rect)), C.float(radiusX), C.float(radiusY), C.sk_path_direction_t(direction))
}

func PathTransform(path Path, matrix *Matrix) {
	C.sk_path_transform(path, (*C.sk_matrix_t)(unsafe.Pointer(matrix)))
}

func PathTransformToDest(path, dstPath Path, matrix *Matrix) {
	C.sk_path_transform_to_dest(path, (*C.sk_matrix_t)(unsafe.Pointer(matrix)), dstPath)
}

func PathReset(path Path) {
	C.sk_path_reset(path)
}

func PathRewind(path Path) {
	C.sk_path_rewind(path)
}

func PathContains(path Path, x, y float32) bool {
	return bool(C.sk_path_contains(path, C.float(x), C.float(y)))
}

func PathGetLastPoint(path Path) geom32.Point {
	var pt geom32.Point
	C.sk_path_get_last_point(path, (*C.sk_point_t)(unsafe.Pointer(&pt)))
	return pt
}

func PathDelete(path Path) {
	C.sk_path_delete(path)
}

func PathEffectCreateCompose(outer, inner PathEffect) PathEffect {
	return C.sk_path_effect_create_compose(outer, inner)
}

func PathEffectCreateSum(first, second PathEffect) PathEffect {
	return C.sk_path_effect_create_sum(first, second)
}

func PathEffectCreateDiscrete(segLength, deviation float32, seedAssist uint32) PathEffect {
	return C.sk_path_effect_create_discrete(C.float(segLength), C.float(deviation), C.uint32_t(seedAssist))
}

func PathEffectCreateCorner(radius float32) PathEffect {
	return C.sk_path_effect_create_corner(C.float(radius))
}

func PathEffectCreate1dPath(path Path, advance, phase float32, style PathEffect1DStyle) PathEffect {
	return C.sk_path_effect_create_1d_path(path, C.float(advance), C.float(phase), C.sk_path_effect_1d_style_t(style))
}

func PathEffectCreate2dLine(width float32, matrix *Matrix) PathEffect {
	return C.sk_path_effect_create_2d_line(C.float(32), (*C.sk_matrix_t)(unsafe.Pointer(matrix)))
}

func PathEffectCreate2dPath(matrix *Matrix, path Path) PathEffect {
	return C.sk_path_effect_create_2d_path((*C.sk_matrix_t)(unsafe.Pointer(matrix)), path)
}

func PathEffectCreateDash(intervals []float32, phase float32) PathEffect {
	return C.sk_path_effect_create_dash((*C.float)(unsafe.Pointer(&intervals[0])), C.int(len(intervals)), C.float(phase))
}

func PathEffectCreateTrim(start, stop float32, mode TrimMode) PathEffect {
	return C.sk_path_effect_create_trim(C.float(start), C.float(stop), C.sk_path_effect_trim_mode_t(mode))
}

func PathEffectUnref(effect PathEffect) {
	C.sk_path_effect_unref(effect)
}

func ShaderNewColor(color Color) Shader {
	return C.sk_shader_new_color(C.sk_color_t(color))
}

func ShaderNewBlend(blendMode BlendMode, dst, src Shader) Shader {
	return C.sk_shader_new_blend(C.sk_blend_mode_t(blendMode), dst, src)
}

func ShaderNewLinearGradient(start, end geom32.Point, colors []Color, colorPos []float32, tileMode TileMode, matrix *Matrix) Shader {
	pts := make([]geom32.Point, 2)
	pts[0] = start
	pts[1] = end
	return C.sk_shader_new_linear_gradient((*C.sk_point_t)(unsafe.Pointer(&pts[0])),
		(*C.sk_color_t)(unsafe.Pointer(&colors[0])), (*C.float)(unsafe.Pointer(&colorPos[0])), C.int(len(colors)),
		C.sk_tile_mode_t(tileMode), (*C.sk_matrix_t)(unsafe.Pointer(matrix)))
}

func ShaderNewRadialGradient(center geom32.Point, radius float32, colors []Color, colorPos []float32, tileMode TileMode, matrix *Matrix) Shader {
	return C.sk_shader_new_radial_gradient((*C.sk_point_t)(unsafe.Pointer(&center)), C.float(radius),
		(*C.sk_color_t)(unsafe.Pointer(&colors[0])), (*C.float)(unsafe.Pointer(&colorPos[0])), C.int(len(colors)),
		C.sk_tile_mode_t(tileMode), (*C.sk_matrix_t)(unsafe.Pointer(matrix)))
}

func ShaderNewSweepGradient(center geom32.Point, startAngle, endAngle float32, colors []Color, colorPos []float32, tileMode TileMode, matrix *Matrix) Shader {
	return C.sk_shader_new_sweep_gradient((*C.sk_point_t)(unsafe.Pointer(&center)),
		(*C.sk_color_t)(unsafe.Pointer(&colors[0])), (*C.float)(unsafe.Pointer(&colorPos[0])), C.int(len(colors)),
		C.sk_tile_mode_t(tileMode), C.float(startAngle), C.float(endAngle), (*C.sk_matrix_t)(unsafe.Pointer(matrix)))
}

func ShaderNewTwoPointConicalGradient(startPt, endPt geom32.Point, startRadius, endRadius float32, colors []Color, colorPos []float32, tileMode TileMode, matrix *Matrix) Shader {
	return C.sk_shader_new_two_point_conical_gradient((*C.sk_point_t)(unsafe.Pointer(&startPt)),
		C.float(startRadius), (*C.sk_point_t)(unsafe.Pointer(&endPt)), C.float(endRadius),
		(*C.sk_color_t)(unsafe.Pointer(&colors[0])), (*C.float)(unsafe.Pointer(&colorPos[0])), C.int(len(colors)),
		C.sk_tile_mode_t(tileMode), (*C.sk_matrix_t)(unsafe.Pointer(matrix)))
}

func ShaderNewPerlinNoiseFractalNoise(baseFreqX, baseFreqY, seed float32, numOctaves int, size ISize) Shader {
	return C.sk_shader_new_perlin_noise_fractal_noise(C.float(baseFreqX), C.float(baseFreqY), C.int(numOctaves),
		C.float(seed), (*C.sk_isize_t)(unsafe.Pointer(&size)))
}

func ShaderNewPerlinNoiseTurbulence(baseFreqX, baseFreqY, seed float32, numOctaves int, size ISize) Shader {
	return C.sk_shader_new_perlin_noise_turbulence(C.float(baseFreqX), C.float(baseFreqY), C.int(numOctaves),
		C.float(seed), (*C.sk_isize_t)(unsafe.Pointer(&size)))
}

func ShaderWithLocalMatrix(shader Shader, matrix *Matrix) Shader {
	return C.sk_shader_with_local_matrix(shader, (*C.sk_matrix_t)(unsafe.Pointer(matrix)))
}

func ShaderWithColorFilter(shader Shader, filter ColorFilter) Shader {
	return C.sk_shader_with_color_filter(shader, filter)
}

func ShaderUnref(shader Shader) {
	C.sk_shader_unref(shader)
}

func StringNewEmpty() String {
	return C.sk_string_new_empty()
}

func StringGetString(str String) string {
	return C.GoStringN(C.sk_string_get_c_str(str), C.int(C.sk_string_get_size(str)))
}

func StringDelete(str String) {
	C.sk_string_delete(str)
}

func SurfaceNewBackendRenderTarget(ctx DirectContext, backend BackendRenderTarget, origin SurfaceOrigin, colorType ColorType, colorSpace ColorSpace, surfaceProps SurfaceProps) Surface {
	return C.sk_surface_new_backend_render_target(ctx, backend, C.gr_surface_origin_t(origin),
		C.sk_color_type_t(colorType), colorSpace, surfaceProps)
}

func SurfaceGetCanvas(aSurface Surface) Canvas {
	return C.sk_surface_get_canvas(aSurface)
}

func SurfaceUnref(aSurface Surface) {
	C.sk_surface_unref(aSurface)
}

func SurfacePropsNew(geometry PixelGeometry) SurfaceProps {
	return C.sk_surfaceprops_new(0, C.sk_pixel_geometry_t(geometry))
}

func TextBlobMakeFromText(text string, font Font) TextBlob {
	b := []byte(text)
	return C.sk_textblob_make_from_text(unsafe.Pointer(&b[0]), C.size_t(len(b)), font, C.sk_text_encoding_t(TextEncodingUTF8))
}

func TextBlobGetBounds(txt TextBlob) *Rect {
	var r Rect
	C.sk_textblob_get_bounds(txt, (*C.sk_rect_t)(unsafe.Pointer(&r)))
	return &r
}

func TextBlobGetIntercepts(txt TextBlob, p Paint, start, end float32, intercepts []float32) int {
	pos := []float32{start, end}
	var dst *float32
	if len(intercepts) != 0 {
		dst = &intercepts[0]
	}
	return int(C.sk_textblob_get_intercepts(txt, (*C.float)(unsafe.Pointer(&pos[0])), (*C.float)(unsafe.Pointer(dst)), p))
}

func TextBlobUnref(txt TextBlob) {
	C.sk_textblob_unref(txt)
}

func TextBlobBuilderNew() TextBlobBuilder {
	return C.sk_textblob_builder_new()
}

func TextBlobBuilderMake(builder TextBlobBuilder) TextBlob {
	return C.sk_textblob_builder_make(builder)
}

func TextBlobBuilderAllocRun(builder TextBlobBuilder, font Font, glyphs []uint16, x, y float32) {
	buffer := &TextBlobBuilderRunBuffer{
		Glyphs: unsafe.Pointer(&glyphs[0]),
	}
	C.sk_textblob_builder_alloc_run(builder, font, C.int(len(glyphs)), C.float(x), C.float(y), nil, (*C.sk_text_blob_builder_run_buffer_t)(unsafe.Pointer(buffer)))
}

func TextBlobBuilderAllocRunPos(builder TextBlobBuilder, font Font, glyphs []uint16, pos []geom32.Point) {
	buffer := &TextBlobBuilderRunBuffer{
		Glyphs: unsafe.Pointer(&glyphs[0]),
		Pos:    unsafe.Pointer(&pos[0]),
	}
	C.sk_textblob_builder_alloc_run_pos(builder, font, C.int(len(glyphs)), nil, (*C.sk_text_blob_builder_run_buffer_t)(unsafe.Pointer(buffer)))
}

func TextBlobBuilderAllocRunPosH(builder TextBlobBuilder, font Font, glyphs []uint16, pos []float32, y float32) {
	buffer := &TextBlobBuilderRunBuffer{
		Glyphs: unsafe.Pointer(&glyphs[0]),
		Pos:    unsafe.Pointer(&pos[0]),
	}
	C.sk_textblob_builder_alloc_run_pos_h(builder, font, C.int(len(glyphs)), C.float(y), nil, (*C.sk_text_blob_builder_run_buffer_t)(unsafe.Pointer(buffer)))
}

func TextBlobBuilderDelete(builder TextBlobBuilder) {
	C.sk_textblob_builder_delete(builder)
}

func TypeFaceGetFontStyle(face TypeFace) FontStyle {
	return C.sk_typeface_get_fontstyle(face)
}

func TypeFaceIsFixedPitch(face TypeFace) bool {
	return bool(C.sk_typeface_is_fixed_pitch(face))
}

func TypeFaceGetFamilyName(face TypeFace) String {
	return C.sk_typeface_get_family_name(face)
}

func TypeFaceGetUnitsPerEm(face TypeFace) int {
	return int(C.sk_typeface_get_units_per_em(face))
}

func TypeFaceUnref(face TypeFace) {
	C.sk_typeface_unref(face)
}
