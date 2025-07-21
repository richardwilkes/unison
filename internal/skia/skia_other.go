// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
#include <string.h>
#include "sk_capi.h"
*/
import "C"

import (
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/geom"
)

const ColorTypeN32 = ColorTypeRGBA8888

type (
	BackendRenderTarget  = *C.gr_backendrendertarget_t
	DirectContext        = *C.gr_direct_context_t
	GLInterface          = *C.gr_glinterface_t
	Canvas               = *C.sk_canvas_t
	ColorFilter          = *C.sk_color_filter_t
	ColorSpace           = *C.sk_color_space_t
	Data                 = *C.sk_data_t
	Document             = *C.sk_document_t
	DynamicMemoryWStream = *C.sk_dynamic_memory_wstream_t
	FileWStream          = *C.sk_file_wstream_t
	Font                 = *C.sk_font_t
	FontMgr              = *C.sk_font_mgr_t
	FontStyle            = *C.sk_font_style_t
	FontStyleSet         = *C.sk_font_style_set_t
	Image                = *C.sk_image_t
	ImageFilter          = *C.sk_image_filter_t
	MaskFilter           = *C.sk_mask_filter_t
	OpBuilder            = *C.sk_op_builder_t
	Paint                = *C.sk_paint_t
	Path                 = *C.sk_path_t
	PathEffect           = *C.sk_path_effect_t
	SamplingOptions      = *C.sk_sampling_options_t
	Shader               = *C.sk_shader_t
	String               = *C.sk_string_t
	Surface              = *C.sk_surface_t
	SurfaceProps         = *C.sk_surface_props_t
	TextBlob             = *C.sk_text_blob_t
	TextBlobBuilder      = *C.sk_text_blob_builder_t
	TypeFace             = *C.sk_typeface_t
	WStream              = *C.sk_wstream_t
)

func fromGeomMatrix(m *geom.Matrix) *C.sk_matrix_t {
	if m == nil {
		return nil
	}
	return (*C.sk_matrix_t)(unsafe.Pointer(&Matrix{Matrix: *m, Persp2: 1}))
}

func fromGeomRect(r *geom.Rect) *C.sk_rect_t {
	if r == nil {
		return nil
	}
	return (*C.sk_rect_t)(unsafe.Pointer(&Rect{Left: r.X, Top: r.Y, Right: r.Right(), Bottom: r.Bottom()}))
}

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

func ContextDelete(ctx DirectContext) {
	C.gr_direct_context_delete(ctx)
}

func ContextFlushAndSubmit(ctx DirectContext, syncCPU bool) {
	C.gr_direct_context_flush_and_submit(ctx, C.bool(syncCPU))
}

func ContextResetGLTextureBindings(ctx DirectContext) {
	C.gr_direct_context_reset_gl_texture_bindings(ctx)
}

func ContextReset(ctx DirectContext) {
	C.gr_direct_context_reset(ctx)
}

func ContextAbandonContext(ctx DirectContext) {
	C.gr_direct_context_abandon_context(ctx)
}

func ContextReleaseResourcesAndAbandonContext(ctx DirectContext) {
	C.gr_direct_context_release_resources_and_abandon_context(ctx)
}

func ContextUnref(ctx DirectContext) {
	C.gr_direct_context_unref(ctx)
}

func GLInterfaceCreateNativeInterface() GLInterface {
	return C.gr_glinterface_create_native_interface()
}

func GLInterfaceUnref(intf GLInterface) {
	C.gr_glinterface_unref(intf)
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

func CanvasTranslate(canvas Canvas, offset geom.Point) {
	C.sk_canvas_translate(canvas, C.float(offset.X), C.float(offset.Y))
}

func CanvasScale(canvas Canvas, scale geom.Point) {
	C.sk_canvas_scale(canvas, C.float(scale.X), C.float(scale.Y))
}

func CanvasRotateRadians(canvas Canvas, radians float32) {
	C.sk_canvas_rotate_radians(canvas, C.float(radians))
}

func CanvasSkew(canvas Canvas, skew geom.Point) {
	C.sk_canvas_skew(canvas, C.float(skew.X), C.float(skew.Y))
}

func CanvasConcat(canvas Canvas, matrix geom.Matrix) {
	C.sk_canvas_concat(canvas, fromGeomMatrix(&matrix))
}

func CanvasResetMatrix(canvas Canvas) {
	C.sk_canvas_reset_matrix(canvas)
}

func CanvasGetTotalMatrix(canvas Canvas) geom.Matrix {
	var matrix Matrix
	C.sk_canvas_get_total_matrix(canvas, (*C.sk_matrix_t)(unsafe.Pointer(&matrix)))
	return matrix.Matrix
}

func CanvasSetMatrix(canvas Canvas, matrix geom.Matrix) {
	C.sk_canvas_set_matrix(canvas, fromGeomMatrix(&matrix))
}

func CanvasQuickRejectPath(canvas Canvas, path Path) bool {
	return bool(C.sk_canvas_quick_reject_path(canvas, path))
}

func CanvasQuickRejectRect(canvas Canvas, rect geom.Rect) bool {
	return bool(C.sk_canvas_quick_reject_rect(canvas, fromGeomRect(&rect)))
}

func CanvasClear(canvas Canvas, color Color) {
	C.sk_canvas_clear(canvas, C.sk_color_t(color))
}

func CanvasDrawPaint(canvas Canvas, paint Paint) {
	C.sk_canvas_draw_paint(canvas, paint)
}

func CanvasDrawRect(canvas Canvas, rect geom.Rect, paint Paint) {
	C.sk_canvas_draw_rect(canvas, fromGeomRect(&rect), paint)
}

func CanvasDrawRoundRect(canvas Canvas, rect geom.Rect, radius geom.Size, paint Paint) {
	C.sk_canvas_draw_round_rect(canvas, fromGeomRect(&rect), C.float(radius.Width), C.float(radius.Height), paint)
}

func CanvasDrawCircle(canvas Canvas, center geom.Point, radius float32, paint Paint) {
	C.sk_canvas_draw_circle(canvas, C.float(center.X), C.float(center.Y), C.float(radius), paint)
}

func CanvasDrawOval(canvas Canvas, rect geom.Rect, paint Paint) {
	C.sk_canvas_draw_oval(canvas, fromGeomRect(&rect), paint)
}

func CanvasDrawPath(canvas Canvas, path Path, paint Paint) {
	C.sk_canvas_draw_path(canvas, path, paint)
}

func CanvasDrawImageRect(canvas Canvas, img Image, srcRect, dstRect geom.Rect, sampling SamplingOptions, paint Paint) {
	C.sk_canvas_draw_image_rect(canvas, img, fromGeomRect(&srcRect), fromGeomRect(&dstRect), sampling, paint,
		C.SRC_RECT_CONSTRAINT_STRICT)
}

func CanvasDrawImageNine(canvas Canvas, img Image, centerRect, dstRect geom.Rect, filter FilterMode, paint Paint) {
	centerRect = centerRect.Align()
	C.sk_canvas_draw_image_nine(canvas, img, (*C.sk_irect_t)(unsafe.Pointer(&IRect{
		Left:   int32(centerRect.X),
		Top:    int32(centerRect.Y),
		Right:  int32(centerRect.Right()),
		Bottom: int32(centerRect.Bottom()),
	})), fromGeomRect(&dstRect), C.sk_filter_mode_t(filter), paint)
}

func CanvasDrawColor(canvas Canvas, color Color, mode BlendMode) {
	C.sk_canvas_draw_color(canvas, C.sk_color_t(color), C.sk_blend_mode_t(mode))
}

func CanvasDrawPoint(canvas Canvas, pt geom.Point, paint Paint) {
	C.sk_canvas_draw_point(canvas, C.float(pt.X), C.float(pt.Y), paint)
}

func CanvasDrawPoints(canvas Canvas, mode PointMode, pts []geom.Point, paint Paint) {
	C.sk_canvas_draw_points(canvas, C.sk_point_mode_t(mode), C.size_t(len(pts)),
		(*C.sk_point_t)(unsafe.Pointer(&pts[0])), paint)
}

func CanvasDrawLine(canvas Canvas, start, end geom.Point, paint Paint) {
	C.sk_canvas_draw_line(canvas, C.float(start.X), C.float(start.Y), C.float(end.X), C.float(end.Y), paint)
}

func CanvasDrawArc(canvas Canvas, oval geom.Rect, startAngle, sweepAngle float32, useCenter bool, paint Paint) {
	C.sk_canvas_draw_arc(canvas, fromGeomRect(&oval), C.float(startAngle), C.float(sweepAngle), C.bool(useCenter), paint)
}

func CanvasDrawSimpleText(canvas Canvas, str string, pt geom.Point, font Font, paint Paint) {
	b := []byte(str)
	C.sk_canvas_draw_simple_text(canvas, unsafe.Pointer(&b[0]), C.size_t(len(b)),
		C.sk_text_encoding_t(TextEncodingUTF8), C.float(pt.X), C.float(pt.Y), font, paint)
}

func CanvasDrawTextBlob(canvas Canvas, txt TextBlob, pt geom.Point, paint Paint) {
	C.sk_canvas_draw_text_blob(canvas, txt, C.float(pt.X), C.float(pt.Y), paint)
}

func CanavasClipRectWithOperation(canvas Canvas, rect geom.Rect, op ClipOp, antialias bool) {
	C.sk_canvas_clip_rect_with_operation(canvas, fromGeomRect(&rect), C.sk_clip_op_t(op), C.bool(antialias))
}

func CanavasClipPathWithOperation(canvas Canvas, path Path, op ClipOp, antialias bool) {
	C.sk_canvas_clip_path_with_operation(canvas, path, C.sk_clip_op_t(op), C.bool(antialias))
}

func CanvasGetLocalClipBounds(canvas Canvas) geom.Rect {
	var r Rect
	C.sk_canvas_get_local_clip_bounds(canvas, (*C.sk_rect_t)(unsafe.Pointer(&r)))
	return toGeomRect(r)
}

func CanvasGetSurface(canvas Canvas) Surface {
	return C.sk_canvas_get_surface(canvas)
}

func CanvasIsClipEmpty(canvas Canvas) bool {
	return bool(C.sk_canvas_is_clip_empty(canvas))
}

func CanvasIsClipRect(canvas Canvas) bool {
	return bool(C.sk_canvas_is_clip_rect(canvas))
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

func EncodeJPEG(ctx DirectContext, img Image, quality int) Data {
	return C.sk_encode_jpeg(ctx, img, C.int(quality))
}

func EncodePNG(ctx DirectContext, img Image, compressionLevel int) Data {
	return C.sk_encode_png(ctx, img, C.int(compressionLevel))
}

func EncodeWebp(ctx DirectContext, img Image, quality float32, lossy bool) Data {
	return C.sk_encode_webp(ctx, img, C.float(quality), C.bool(lossy))
}

func DocumentMakePDF(stream WStream, metadata *MetaData) Document {
	var md metaData
	md.set(metadata)
	return C.sk_document_make_pdf(stream, (*C.sk_metadata_t)(unsafe.Pointer(&md)))
}

func DocumentBeginPage(doc Document, size geom.Size) Canvas {
	return C.sk_document_begin_page(doc, C.float(size.Width), C.float(size.Height))
}

func DocumentEndPage(doc Document) {
	C.sk_document_end_page(doc)
}

func DocumentClose(doc Document) {
	C.sk_document_close(doc)
}

func DocumentAbort(doc Document) {
	C.sk_document_abort(doc)
}

func DynamicMemoryWStreamNew() DynamicMemoryWStream {
	return C.sk_dynamic_memory_wstream_new()
}

func DynamicMemoryWStreamAsWStream(s DynamicMemoryWStream) WStream {
	return C.sk_dynamic_memory_wstream_as_wstream(s)
}

func DynamicMemoryWStreamWrite(s DynamicMemoryWStream, data []byte) bool {
	return bool(C.sk_dynamic_memory_wstream_write(s, unsafe.Pointer(&data[0]), C.size_t(len(data))))
}

func DynamicMemoryWStreamBytesWritten(s DynamicMemoryWStream) int {
	return int(C.sk_dynamic_memory_wstream_bytes_written(s))
}

func DynamicMemoryWStreamRead(s DynamicMemoryWStream, data []byte) int {
	return int(C.sk_dynamic_memory_wstream_read(s, unsafe.Pointer(&data[0]), 0, C.size_t(len(data))))
}

func DynamicMemoryWStreamDelete(s DynamicMemoryWStream) {
	C.sk_dynamic_memory_wstream_delete(s)
}

func FileWStreamNew(filePath string) FileWStream {
	p := C.CString(filePath)
	defer C.free(unsafe.Pointer(p))
	return C.sk_file_wstream_new(p)
}

func FileWStreamAsWStream(s FileWStream) WStream {
	return C.sk_file_wstream_as_wstream(s)
}

func FileWStreamWrite(s FileWStream, data []byte) bool {
	return bool(C.sk_file_wstream_write(s, unsafe.Pointer(&data[0]), C.size_t(len(data))))
}

func FileWStreamBytesWritten(s FileWStream) int {
	return int(C.sk_file_wstream_bytes_written(s))
}

func FileWStreamFlush(s FileWStream) {
	C.sk_file_wstream_flush(s)
}

func FileWStreamDelete(s FileWStream) {
	C.sk_file_wstream_delete(s)
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

func FontRuneToGlyph(font Font, r rune) uint16 {
	return uint16(C.sk_font_unichar_to_glyph(font, C.int32_t(r)))
}

func FontRunesToGlyphs(font Font, r []rune) []uint16 {
	glyphs := make([]uint16, len(r))
	C.sk_font_unichars_to_glyphs(font, (*C.int32_t)(unsafe.Pointer(&r[0])), C.int(len(r)), (*C.uint16_t)(unsafe.Pointer(&glyphs[0])))
	return glyphs
}

func FontGlyphWidths(font Font, glyphs []uint16) []float32 {
	widths := make([]float32, len(glyphs))
	C.sk_font_glyph_widths(font, (*C.uint16_t)(unsafe.Pointer(&glyphs[0])), C.int(len(glyphs)), (*C.float)(unsafe.Pointer(&widths[0])))
	return widths
}

func FontGlyphsXPos(font Font, glyphs []uint16) []float32 {
	pos := make([]float32, len(glyphs)+1)
	g2 := make([]uint16, len(glyphs)+1)
	copy(g2, glyphs)
	C.sk_font_get_xpos(font, (*C.ushort)(&g2[0]), C.int(len(g2)), (*C.float)(unsafe.Pointer(&pos[0])), 0)
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

func FontMgrMatchFamilyStyleCharacter(mgr FontMgr, family string, style FontStyle, ch rune) TypeFace {
	cFamily := C.CString(family)
	defer C.free(unsafe.Pointer(cFamily))
	return C.sk_fontmgr_match_family_style_character(mgr, cFamily, style, nil, 0, C.int32_t(ch))
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

func ImageMakeNonTextureImage(img Image) Image {
	return C.sk_image_make_non_texture_image(img)
}

func ImageMakeShader(img Image, tileModeX, tileModeY TileMode, sampling SamplingOptions, matrix geom.Matrix) Shader {
	return C.sk_image_make_shader(img, C.sk_tile_mode_t(tileModeX), C.sk_tile_mode_t(tileModeY), sampling,
		fromGeomMatrix(&matrix))
}

func ImageTextureFromImage(ctx DirectContext, img Image, mipMapped, budgeted bool) Image {
	return C.sk_image_texture_from_image(ctx, img, C.bool(mipMapped), C.bool(budgeted))
}

func ImageUnref(img Image) {
	C.sk_image_unref(img)
}

func ImageFilterNewArithmetic(k1, k2, k3, k4 float32, enforcePMColor bool, background, foreground ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_arithmetic(C.float(k1), C.float(k2), C.float(k3), C.float(k4), C.bool(enforcePMColor),
		background, foreground, fromGeomRect(cropRect))
}

func ImageFilterNewBlur(sigmaX, sigmaY float32, tileMode TileMode, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_blur(C.float(sigmaX), C.float(sigmaY), C.sk_tile_mode_t(tileMode), input,
		fromGeomRect(cropRect))
}

func ImageFilterNewColorFilter(colorFilter ColorFilter, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_color_filter(colorFilter, input, fromGeomRect(cropRect))
}

func ImageFilterNewCompose(outer, inner ImageFilter) ImageFilter {
	return C.sk_imagefilter_new_compose(outer, inner)
}

func ImageFilterNewDisplacementMapEffect(xChannelSelector, yChannelSelector ColorChannel, scale float32, displacement, color ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_displacement_map_effect(C.sk_color_channel_t(xChannelSelector),
		C.sk_color_channel_t(yChannelSelector), C.float(scale), displacement, color, fromGeomRect(cropRect))
}

func ImageFilterNewDropShadow(dx, dy, sigmaX, sigmaY float32, color Color, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_drop_shadow(C.float(dx), C.float(dy), C.float(sigmaX), C.float(sigmaY),
		C.sk_color_t(color), input, fromGeomRect(cropRect))
}

func ImageFilterNewDropShadowOnly(dx, dy, sigmaX, sigmaY float32, color Color, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_drop_shadow_only(C.float(dx), C.float(dy), C.float(sigmaX), C.float(sigmaY),
		C.sk_color_t(color), input, fromGeomRect(cropRect))
}

func ImageFilterNewImageSource(img Image, srcRect, dstRect geom.Rect, sampling SamplingOptions) ImageFilter {
	return C.sk_imagefilter_new_image_source(img, fromGeomRect(&srcRect), fromGeomRect(&dstRect), sampling)
}

func ImageFilterNewImageSourceDefault(img Image, sampling SamplingOptions) ImageFilter {
	return C.sk_imagefilter_new_image_source_default(img, sampling)
}

func ImageFilterNewMagnifier(lensBounds geom.Rect, zoomAmount, inset float32, sampling SamplingOptions, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_magnifier(fromGeomRect(&lensBounds), C.float(zoomAmount), C.float(inset), sampling, input, fromGeomRect(cropRect))
}

func ImageFilterNewMatrixConvolution(size *ISize, kernel []float32, gain, bias float32, offset *IPoint, tileMode TileMode, convolveAlpha bool, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_matrix_convolution((*C.sk_isize_t)(unsafe.Pointer(size)),
		(*C.float)(unsafe.Pointer(&kernel[0])), C.float(gain), C.float(bias), (*C.sk_ipoint_t)(unsafe.Pointer(offset)),
		C.sk_tile_mode_t(tileMode), C.bool(convolveAlpha), input, fromGeomRect(cropRect))
}

func ImageFilterNewMatrixTransform(matrix geom.Matrix, sampling SamplingOptions, input ImageFilter) ImageFilter {
	return C.sk_imagefilter_new_matrix_transform(fromGeomMatrix(&matrix), sampling, input)
}

func ImageFilterNewMerge(filters []ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_merge((**C.sk_image_filter_t)(unsafe.Pointer(&filters[0])), C.int(len(filters)),
		fromGeomRect(cropRect))
}

func ImageFilterNewOffset(dx, dy float32, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_offset(C.float(dx), C.float(dy), input, fromGeomRect(cropRect))
}

func ImageFilterNewTile(src, dst geom.Rect, input ImageFilter) ImageFilter {
	return C.sk_imagefilter_new_tile(fromGeomRect(&src), fromGeomRect(&dst), input)
}

func ImageFilterNewDilate(radiusX, radiusY int, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_dilate(C.int(radiusX), C.int(radiusY), input, fromGeomRect(cropRect))
}

func ImageFilterNewErode(radiusX, radiusY int, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_erode(C.int(radiusX), C.int(radiusY), input, fromGeomRect(cropRect))
}

func ImageFilterNewDistantLitDiffuse(pt *Point3, color Color, scale, reflectivity float32, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_distant_lit_diffuse((*C.sk_point3_t)(unsafe.Pointer(pt)), C.sk_color_t(color),
		C.float(scale), C.float(reflectivity), input, fromGeomRect(cropRect))
}

func ImageFilterNewPointLitDiffuse(pt *Point3, color Color, scale, reflectivity float32, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_point_lit_diffuse((*C.sk_point3_t)(unsafe.Pointer(pt)), C.sk_color_t(color),
		C.float(scale), C.float(reflectivity), input, fromGeomRect(cropRect))
}

func ImageFilterNewSpotLitDiffuse(pt, targetPt *Point3, specularExponent, cutoffAngle, scale, reflectivity float32, color Color, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_spot_lit_diffuse((*C.sk_point3_t)(unsafe.Pointer(pt)),
		(*C.sk_point3_t)(unsafe.Pointer(targetPt)), C.float(specularExponent), C.float(cutoffAngle),
		C.sk_color_t(color), C.float(scale), C.float(reflectivity), input, fromGeomRect(cropRect))
}

func ImageFilterNewDistantLitSpecular(pt *Point3, color Color, scale, reflectivity, shine float32, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_distant_lit_specular((*C.sk_point3_t)(unsafe.Pointer(pt)), C.sk_color_t(color),
		C.float(scale), C.float(reflectivity), C.float(shine), input, fromGeomRect(cropRect))
}

func ImageFilterNewPointLitSpecular(pt *Point3, color Color, scale, reflectivity, shine float32, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_point_lit_specular((*C.sk_point3_t)(unsafe.Pointer(pt)), C.sk_color_t(color),
		C.float(scale), C.float(reflectivity), C.float(shine), input, fromGeomRect(cropRect))
}

func ImageFilterNewSpotLitSpecular(pt, targetPt *Point3, specularExponent, cutoffAngle, scale, reflectivity, shine float32, color Color, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	return C.sk_imagefilter_new_spot_lit_specular((*C.sk_point3_t)(unsafe.Pointer(pt)),
		(*C.sk_point3_t)(unsafe.Pointer(targetPt)), C.float(specularExponent), C.float(cutoffAngle),
		C.sk_color_t(color), C.float(scale), C.float(reflectivity), C.float(shine), input,
		fromGeomRect(cropRect))
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

func MaskFilterNewClip(minimum, maximum byte) MaskFilter {
	return C.sk_maskfilter_new_clip(C.uint8_t(minimum), C.uint8_t(maximum))
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

func PaintEquivalent(left, right Paint) bool {
	return bool(C.sk_paint_equivalent(left, right))
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

func PaintGetFillPath(paint Paint, inPath, outPath Path, cullRect *geom.Rect, resScale float32) bool {
	return bool(C.sk_paint_get_fill_path(paint, inPath, outPath, fromGeomRect(cullRect), C.float(resScale)))
}

func PathNew() Path {
	return C.sk_path_new()
}

func PathParseSVGString(path Path, svg string) bool {
	cstr := C.CString(svg)
	defer C.free(unsafe.Pointer(cstr))
	return bool(C.sk_path_parse_svg_string(path, cstr))
}

func PathToSVGString(path Path, absolute bool) String {
	return C.sk_path_to_svg_string(path, C.bool(absolute))
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

func PathArcToWithOval(path Path, rect geom.Rect, startAngle, sweepAngle float32, forceMoveTo bool) {
	C.sk_path_arc_to_with_oval(path, fromGeomRect(&rect), C.float(startAngle), C.float(sweepAngle),
		C.bool(forceMoveTo))
}

func PathRArcTo(path Path, dx, dy, rx, ry, rotation float32, arcSize ArcSize, direction Direction) {
	C.sk_path_rarc_to(path, C.float(rx), C.float(ry), C.float(rotation), C.sk_path_arc_size_t(arcSize),
		C.sk_path_direction_t(direction), C.float(dx), C.float(dy))
}

func PathGetBounds(path Path) geom.Rect {
	var r Rect
	C.sk_path_get_bounds(path, (*C.sk_rect_t)(unsafe.Pointer(&r)))
	return toGeomRect(r)
}

func PathComputeTightBounds(path Path) geom.Rect {
	var r Rect
	C.sk_path_compute_tight_bounds(path, (*C.sk_rect_t)(unsafe.Pointer(&r)))
	return toGeomRect(r)
}

func PathAddCircle(path Path, center geom.Point, radius float32, direction Direction) {
	C.sk_path_add_circle(path, C.float(center.X), C.float(center.Y), C.float(radius), C.sk_path_direction_t(direction))
}

func PathClone(path Path) Path {
	return C.sk_path_clone(path)
}

func PathClose(path Path) {
	C.sk_path_close(path)
}

func PathConicTo(path Path, ctrlPt, endPt geom.Point, weight float32) {
	C.sk_path_conic_to(path, C.float(ctrlPt.X), C.float(ctrlPt.Y), C.float(endPt.X), C.float(endPt.Y), C.float(weight))
}

func PathRConicTo(path Path, ctrlPt, endPt geom.Point, weight float32) {
	C.sk_path_rconic_to(path, C.float(ctrlPt.X), C.float(ctrlPt.Y), C.float(endPt.X), C.float(endPt.Y), C.float(weight))
}

func PathCubicTo(path Path, cp1, cp2, end geom.Point) {
	C.sk_path_cubic_to(path, C.float(cp1.X), C.float(cp1.Y), C.float(cp2.X), C.float(cp2.Y), C.float(end.X),
		C.float(end.Y))
}

func PathRCubicTo(path Path, cp1, cp2, end geom.Point) {
	C.sk_path_rcubic_to(path, C.float(cp1.X), C.float(cp1.Y), C.float(cp2.X), C.float(cp2.Y), C.float(end.X),
		C.float(end.Y))
}

func PathLineTo(path Path, pt geom.Point) {
	C.sk_path_line_to(path, C.float(pt.X), C.float(pt.Y))
}

func PathRLineTo(path Path, pt geom.Point) {
	C.sk_path_rline_to(path, C.float(pt.X), C.float(pt.Y))
}

func PathMoveTo(path Path, pt geom.Point) {
	C.sk_path_move_to(path, C.float(pt.X), C.float(pt.Y))
}

func PathRMoveTo(path Path, pt geom.Point) {
	C.sk_path_rmove_to(path, C.float(pt.X), C.float(pt.Y))
}

func PathAddOval(path Path, rect geom.Rect, direction Direction) {
	C.sk_path_add_oval(path, fromGeomRect(&rect), C.sk_path_direction_t(direction))
}

func PathAddPath(path, other Path, mode PathAddMode) {
	C.sk_path_add_path(path, other, C.sk_path_add_mode_t(mode))
}

func PathAddPathReverse(path, other Path) {
	C.sk_path_add_path_reverse(path, other)
}

func PathAddPathMatrix(path, other Path, matrix geom.Matrix, mode PathAddMode) {
	C.sk_path_add_path_matrix(path, other, fromGeomMatrix(&matrix), C.sk_path_add_mode_t(mode))
}

func PathAddPathOffset(path, other Path, offset geom.Point, mode PathAddMode) {
	C.sk_path_add_path_offset(path, other, C.float(offset.X), C.float(offset.Y), C.sk_path_add_mode_t(mode))
}

func PathAddPoly(path Path, pts []geom.Point, closePath bool) {
	C.sk_path_add_poly(path, (*C.sk_point_t)(unsafe.Pointer(&pts[0])), C.int(len(pts)), C.bool(closePath))
}

func PathQuadTo(path Path, ctrlPt, endPt geom.Point) {
	C.sk_path_quad_to(path, C.float(ctrlPt.X), C.float(ctrlPt.Y), C.float(endPt.X), C.float(endPt.Y))
}

func PathAddRect(path Path, rect geom.Rect, direction Direction) {
	C.sk_path_add_rect(path, fromGeomRect(&rect), C.sk_path_direction_t(direction))
}

func PathAddRoundedRect(path Path, rect geom.Rect, radius geom.Size, direction Direction) {
	C.sk_path_add_rounded_rect(path, fromGeomRect(&rect), C.float(radius.Width), C.float(radius.Height),
		C.sk_path_direction_t(direction))
}

func PathTransform(path Path, matrix geom.Matrix) {
	C.sk_path_transform(path, fromGeomMatrix(&matrix))
}

func PathTransformToDest(path, dstPath Path, matrix geom.Matrix) {
	C.sk_path_transform_to_dest(path, fromGeomMatrix(&matrix), dstPath)
}

func PathReset(path Path) {
	C.sk_path_reset(path)
}

func PathRewind(path Path) {
	C.sk_path_rewind(path)
}

func PathContains(path Path, pt geom.Point) bool {
	return bool(C.sk_path_contains(path, C.float(pt.X), C.float(pt.Y)))
}

func PathGetLastPoint(path Path) geom.Point {
	var pt geom.Point
	C.sk_path_get_last_point(path, (*C.sk_point_t)(unsafe.Pointer(&pt)))
	return pt
}

func PathDelete(path Path) {
	C.sk_path_delete(path)
}

func PathCompute(path, other Path, op PathOp) bool {
	return bool(C.sk_path_op(path, other, C.sk_path_op_t(op), path))
}

func PathSimplify(path Path) bool {
	return bool(C.sk_path_simplify(path, path))
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

func PathEffectCreate2dLine(width float32, matrix geom.Matrix) PathEffect {
	return C.sk_path_effect_create_2d_line(C.float(32), fromGeomMatrix(&matrix))
}

func PathEffectCreate2dPath(matrix geom.Matrix, path Path) PathEffect {
	return C.sk_path_effect_create_2d_path(fromGeomMatrix(&matrix), path)
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

func RegisterImageCodecs() {
	C.register_image_codecs()
}

func ShaderNewColor(color Color) Shader {
	return C.sk_shader_new_color(C.sk_color_t(color))
}

func ShaderNewBlend(blendMode BlendMode, dst, src Shader) Shader {
	return C.sk_shader_new_blend(C.sk_blend_mode_t(blendMode), dst, src)
}

func ShaderNewLinearGradient(start, end geom.Point, colors []Color, colorPos []float32, tileMode TileMode, matrix geom.Matrix) Shader {
	pts := make([]geom.Point, 2)
	pts[0] = start
	pts[1] = end
	return C.sk_shader_new_linear_gradient((*C.sk_point_t)(unsafe.Pointer(&pts[0])),
		(*C.sk_color_t)(unsafe.Pointer(&colors[0])), (*C.float)(unsafe.Pointer(&colorPos[0])), C.int(len(colors)),
		C.sk_tile_mode_t(tileMode), fromGeomMatrix(&matrix))
}

func ShaderNewRadialGradient(center geom.Point, radius float32, colors []Color, colorPos []float32, tileMode TileMode, matrix geom.Matrix) Shader {
	return C.sk_shader_new_radial_gradient((*C.sk_point_t)(unsafe.Pointer(&center)), C.float(radius),
		(*C.sk_color_t)(unsafe.Pointer(&colors[0])), (*C.float)(unsafe.Pointer(&colorPos[0])), C.int(len(colors)),
		C.sk_tile_mode_t(tileMode), fromGeomMatrix(&matrix))
}

func ShaderNewSweepGradient(center geom.Point, startAngle, endAngle float32, colors []Color, colorPos []float32, tileMode TileMode, matrix geom.Matrix) Shader {
	return C.sk_shader_new_sweep_gradient((*C.sk_point_t)(unsafe.Pointer(&center)),
		(*C.sk_color_t)(unsafe.Pointer(&colors[0])), (*C.float)(unsafe.Pointer(&colorPos[0])), C.int(len(colors)),
		C.sk_tile_mode_t(tileMode), C.float(startAngle), C.float(endAngle), fromGeomMatrix(&matrix))
}

func ShaderNewTwoPointConicalGradient(startPt, endPt geom.Point, startRadius, endRadius float32, colors []Color, colorPos []float32, tileMode TileMode, matrix geom.Matrix) Shader {
	return C.sk_shader_new_two_point_conical_gradient((*C.sk_point_t)(unsafe.Pointer(&startPt)),
		C.float(startRadius), (*C.sk_point_t)(unsafe.Pointer(&endPt)), C.float(endRadius),
		(*C.sk_color_t)(unsafe.Pointer(&colors[0])), (*C.float)(unsafe.Pointer(&colorPos[0])), C.int(len(colors)),
		C.sk_tile_mode_t(tileMode), fromGeomMatrix(&matrix))
}

func ShaderNewPerlinNoiseFractalNoise(baseFreqX, baseFreqY, seed float32, numOctaves int, size ISize) Shader {
	return C.sk_shader_new_perlin_noise_fractal_noise(C.float(baseFreqX), C.float(baseFreqY), C.int(numOctaves),
		C.float(seed), (*C.sk_isize_t)(unsafe.Pointer(&size)))
}

func ShaderNewPerlinNoiseTurbulence(baseFreqX, baseFreqY, seed float32, numOctaves int, size ISize) Shader {
	return C.sk_shader_new_perlin_noise_turbulence(C.float(baseFreqX), C.float(baseFreqY), C.int(numOctaves),
		C.float(seed), (*C.sk_isize_t)(unsafe.Pointer(&size)))
}

func ShaderWithLocalMatrix(shader Shader, matrix geom.Matrix) Shader {
	return C.sk_shader_with_local_matrix(shader, fromGeomMatrix(&matrix))
}

func ShaderWithColorFilter(shader Shader, filter ColorFilter) Shader {
	return C.sk_shader_with_color_filter(shader, filter)
}

func ShaderUnref(shader Shader) {
	C.sk_shader_unref(shader)
}

func StringNew(s string) String {
	if s == "" {
		return StringNewEmpty()
	}
	b := []byte(s)
	return C.sk_string_new((*C.char)(unsafe.Pointer(&b[0])), C.size_t(len(b)))
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

func SurfaceMakeRasterDirect(info *ImageInfo, pixels []byte, rowBytes int, surfaceProps SurfaceProps) Surface {
	return C.sk_surface_make_raster_direct((*C.sk_image_info_t)(unsafe.Pointer(info)), unsafe.Pointer(&pixels[0]), C.size_t(rowBytes), surfaceProps)
}

func SurfaceMakeRasterN32PreMul(info *ImageInfo, surfaceProps SurfaceProps) Surface {
	return C.sk_surface_make_raster_n32_premul((*C.sk_image_info_t)(unsafe.Pointer(info)), surfaceProps)
}

func SurfaceNewBackendRenderTarget(ctx DirectContext, backend BackendRenderTarget, origin SurfaceOrigin, colorType ColorType, colorSpace ColorSpace, surfaceProps SurfaceProps) Surface {
	return C.sk_surface_new_backend_render_target(ctx, backend, C.gr_surface_origin_t(origin),
		C.sk_color_type_t(colorType), colorSpace, surfaceProps)
}

func SurfaceMakeImageSnapshot(aSurface Surface) Image {
	return C.sk_surface_make_image_snapshot(aSurface)
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

func TextBlobGetBounds(txt TextBlob) geom.Rect {
	var r Rect
	C.sk_textblob_get_bounds(txt, (*C.sk_rect_t)(unsafe.Pointer(&r)))
	return toGeomRect(r)
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

func TextBlobBuilderAllocRun(builder TextBlobBuilder, font Font, glyphs []uint16, pt geom.Point) {
	buffer := C.sk_textblob_builder_alloc_run(builder, font, C.int(len(glyphs)), C.float(pt.X), C.float(pt.Y), nil)
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(buffer.glyphs)), len(glyphs)), glyphs)
}

func TextBlobBuilderAllocRunPosH(builder TextBlobBuilder, font Font, glyphs []uint16, positions []float32, y float32) {
	buffer := C.sk_textblob_builder_alloc_run_pos_h(builder, font, C.int(len(glyphs)), C.float(y), nil)
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(buffer.glyphs)), len(glyphs)), glyphs)
	copy(unsafe.Slice((*float32)(unsafe.Pointer(buffer.pos)), len(positions)), positions)
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
