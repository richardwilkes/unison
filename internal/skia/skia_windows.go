// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package skia

import (
	"crypto/sha256"
	_ "embed" // Needed for dll embedding
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xos"
	"golang.org/x/sys/windows"
)

const ColorTypeN32 = ColorTypeBGRA8888

var (
	//go:embed skia_windows.dll
	dllData                                        []byte
	grBackendRenderTargetNewGLProc                 *syscall.Proc
	grBackendRenderTargetDeleteProc                *syscall.Proc
	grContextMakeGLProc                            *syscall.Proc
	grContextDeleteProc                            *syscall.Proc
	grContextFlushAndSubmitProc                    *syscall.Proc
	grContextResetGLTextureBindings                *syscall.Proc
	grContextReset                                 *syscall.Proc
	grContextAbandonContextProc                    *syscall.Proc
	grContextReleaseResourcesAndAbandonContextProc *syscall.Proc
	grContextUnrefProc                             *syscall.Proc
	grGLInterfaceCreateNativeInterfaceProc         *syscall.Proc
	grGLInterfaceUnrefProc                         *syscall.Proc
	skCanvasGetSaveCountProc                       *syscall.Proc
	skCanvasSaveProc                               *syscall.Proc
	skCanvasSaveLayerProc                          *syscall.Proc
	skCanvasSaveLayerAlphaProc                     *syscall.Proc
	skCanvasRestoreProc                            *syscall.Proc
	skCanvasRestoreToCountProc                     *syscall.Proc
	skCanvasTranslateProc                          *syscall.Proc
	skCanvasScaleProc                              *syscall.Proc
	skCanvasRotateRadiansProc                      *syscall.Proc
	skCanvasSkewProc                               *syscall.Proc
	skCanvasConcatProc                             *syscall.Proc
	skCanvasResetMatrixProc                        *syscall.Proc
	skCanvasGetTotalMatrixProc                     *syscall.Proc
	skCanvasSetMatrixProc                          *syscall.Proc
	skCanvasQuickRejectPathProc                    *syscall.Proc
	skCanvasQuickRejectRectProc                    *syscall.Proc
	skCanvasClearProc                              *syscall.Proc
	skCanvasDrawPaintProc                          *syscall.Proc
	skCanvasDrawRectProc                           *syscall.Proc
	skCanvasDrawRoundRectProc                      *syscall.Proc
	skCanvasDrawCircleProc                         *syscall.Proc
	skCanvasDrawOvalProc                           *syscall.Proc
	skCanvasDrawPathProc                           *syscall.Proc
	skCanvasDrawImageRectProc                      *syscall.Proc
	skCanvasDrawImageNineProc                      *syscall.Proc
	skCanvasDrawColorProc                          *syscall.Proc
	skCanvasDrawPointProc                          *syscall.Proc
	skCanvasDrawPointsProc                         *syscall.Proc
	skCanvasDrawLineProc                           *syscall.Proc
	skCanvasDrawArcProc                            *syscall.Proc
	skCanvasDrawSimpleTextProc                     *syscall.Proc
	skCanvasDrawTextBlobProc                       *syscall.Proc
	skCanavasClipRectWithOperationProc             *syscall.Proc
	skCanavasClipPathWithOperationProc             *syscall.Proc
	skCanvasGetLocalClipBoundsProc                 *syscall.Proc
	skCanvasGetSurfaceProc                         *syscall.Proc
	skCanvasIsClipEmptyProc                        *syscall.Proc
	skCanvasIsClipRectProc                         *syscall.Proc
	skColorFilterNewModeProc                       *syscall.Proc
	skColorFilterNewLightingProc                   *syscall.Proc
	skColorFilterNewComposeProc                    *syscall.Proc
	skColorFilterNewColorMatrixProc                *syscall.Proc
	skColorFilterNewLumaColorProc                  *syscall.Proc
	skColorFilterNewHighContrastProc               *syscall.Proc
	skColorFilterUnrefProc                         *syscall.Proc
	skColorSpaceNewSRGBProc                        *syscall.Proc
	skDataNewWithCopyProc                          *syscall.Proc
	skDataGetSizeProc                              *syscall.Proc
	skDataGetDataProc                              *syscall.Proc
	skDataUnrefProc                                *syscall.Proc
	skEncodeJPEGProc                               *syscall.Proc
	skEncodePNGProc                                *syscall.Proc
	skEncodeWEBPProc                               *syscall.Proc
	skDocumentAbortProc                            *syscall.Proc
	skDocumentBeginPageProc                        *syscall.Proc
	skDocumentCloseProc                            *syscall.Proc
	skDocumentEndPageProc                          *syscall.Proc
	skDocumentMakePDFProc                          *syscall.Proc
	skDynamicMemoryWStreamNewProc                  *syscall.Proc
	skDynamicMemoryWStreamAsWStreamProc            *syscall.Proc
	skDynamicMemoryWStreamWriteProc                *syscall.Proc
	skDynamicMemoryWStreamBytesWrittenProc         *syscall.Proc
	skDynamicMemoryWStreamReadProc                 *syscall.Proc
	skDynamicMemoryWStreamDeleteProc               *syscall.Proc
	skFileWStreamNewProc                           *syscall.Proc
	skFileWStreamAsWStreamProc                     *syscall.Proc
	skFileWStreamWriteProc                         *syscall.Proc
	skFileWStreamBytesWrittenProc                  *syscall.Proc
	skFileWStreamFlushProc                         *syscall.Proc
	skFileWStreamDeleteProc                        *syscall.Proc
	skFontNewWithValuesProc                        *syscall.Proc
	skFontSetSubPixelProc                          *syscall.Proc
	skFontSetForceAutoHintingProc                  *syscall.Proc
	skFontSetHintingProc                           *syscall.Proc
	skFontGetMetricsProc                           *syscall.Proc
	skFontMeasureTextProc                          *syscall.Proc
	skFontTextToGlyphsProc                         *syscall.Proc
	skFontUnicharToGlyphProc                       *syscall.Proc
	skFontUnicharsToGlyphsProc                     *syscall.Proc
	skFontGlyphWidthsProc                          *syscall.Proc
	skFontGetXPosProc                              *syscall.Proc
	skFontDeleteProc                               *syscall.Proc
	skFontMgrRefDefaultProc                        *syscall.Proc
	skFontMgrCreateFromDataProc                    *syscall.Proc
	skFontMgrMatchFamilyProc                       *syscall.Proc
	skFontMgrMatchFamilyStyleProc                  *syscall.Proc
	skFontMgrMatchFamilyStyleCharacterProc         *syscall.Proc
	skFontMgrCountFamiliesProc                     *syscall.Proc
	skFontMgrGetFamilyNameProc                     *syscall.Proc
	skFontStyleNewProc                             *syscall.Proc
	skFontStyleGetWeightProc                       *syscall.Proc
	skFontStyleGetWidthProc                        *syscall.Proc
	skFontStyleGetSlantProc                        *syscall.Proc
	skFontStyleDeleteProc                          *syscall.Proc
	skFontStyleSetGetCountProc                     *syscall.Proc
	skFontStyleSetGetStyleProc                     *syscall.Proc
	skFontStyleSetCreateTypeFaceProc               *syscall.Proc
	skFontStyleSetMatchStyleProc                   *syscall.Proc
	skFontStyleSetUnrefProc                        *syscall.Proc
	skImageNewFromEncodedProc                      *syscall.Proc
	skImageNewRasterDataProc                       *syscall.Proc
	skImageGetWidthProc                            *syscall.Proc
	skImageGetHeightProc                           *syscall.Proc
	skImageGetColorSpaceProc                       *syscall.Proc
	skImageGetColorTypeProc                        *syscall.Proc
	skImageGetAlphaTypeProc                        *syscall.Proc
	skImageReadPixelsProc                          *syscall.Proc
	skImageMakeNonTextureImageProc                 *syscall.Proc
	skImageMakeShaderProc                          *syscall.Proc
	skImageTextureFromImageProc                    *syscall.Proc
	skImageUnrefProc                               *syscall.Proc
	skImageFilterNewArithmeticProc                 *syscall.Proc
	skImageFilterNewBlurProc                       *syscall.Proc
	skImageFilterNewColorFilterProc                *syscall.Proc
	skImageFilterNewComposeProc                    *syscall.Proc
	skImageFilterNewDisplacementMapEffectProc      *syscall.Proc
	skImageFilterNewDropShadowProc                 *syscall.Proc
	skImageFilterNewDropShadowOnlyProc             *syscall.Proc
	skImageFilterNewImageSourceProc                *syscall.Proc
	skImageFilterNewImageSourceDefaultProc         *syscall.Proc
	skImageFilterNewMagnifierProc                  *syscall.Proc
	skImageFilterNewMatrixConvolutionProc          *syscall.Proc
	skImageFilterNewMatrixTransformProc            *syscall.Proc
	skImageFilterNewMergeProc                      *syscall.Proc
	skImageFilterNewOffsetProc                     *syscall.Proc
	skImageFilterNewTileProc                       *syscall.Proc
	skImageFilterNewDilateProc                     *syscall.Proc
	skImageFilterNewErodeProc                      *syscall.Proc
	skImageFilterNewDistantLitDiffuseProc          *syscall.Proc
	skImageFilterNewPointLitDiffuseProc            *syscall.Proc
	skImageFilterNewSpotLitDiffuseProc             *syscall.Proc
	skImageFilterNewDistantLitSpecularProc         *syscall.Proc
	skImageFilterNewPointLitSpecularProc           *syscall.Proc
	skImageFilterNewSpotLitSpecularProc            *syscall.Proc
	skImageFilterUnrefProc                         *syscall.Proc
	skMaskFilterNewBlurWithFlagsProc               *syscall.Proc
	skMaskFilterNewTableProc                       *syscall.Proc
	skMaskFilterNewGammaProc                       *syscall.Proc
	skMaskFilterNewClipProc                        *syscall.Proc
	skMaskFilterNewShaderProc                      *syscall.Proc
	skMaskFilterUnrefProc                          *syscall.Proc
	skOpBuilderNewProc                             *syscall.Proc
	skOpBuilderAddProc                             *syscall.Proc
	skOpBuilderResolveProc                         *syscall.Proc
	skOpBuilderDestroyProc                         *syscall.Proc
	skPaintNewProc                                 *syscall.Proc
	skPaintDeleteProc                              *syscall.Proc
	skPaintCloneProc                               *syscall.Proc
	skPaintEquivalentProc                          *syscall.Proc
	skPaintResetProc                               *syscall.Proc
	skPaintIsAntialiasProc                         *syscall.Proc
	skPaintSetAntialiasProc                        *syscall.Proc
	skPaintIsDitherProc                            *syscall.Proc
	skPaintSetDitherProc                           *syscall.Proc
	skPaintGetColorProc                            *syscall.Proc
	skPaintSetColorProc                            *syscall.Proc
	skPaintGetStyleProc                            *syscall.Proc
	skPaintSetStyleProc                            *syscall.Proc
	skPaintGetStrokeWidthProc                      *syscall.Proc
	skPaintSetStrokeWidthProc                      *syscall.Proc
	skPaintGetStrokeMiterProc                      *syscall.Proc
	skPaintSetStrokeMiterProc                      *syscall.Proc
	skPaintGetStrokeCapProc                        *syscall.Proc
	skPaintSetStrokeCapProc                        *syscall.Proc
	skPaintGetStrokeJoinProc                       *syscall.Proc
	skPaintSetStrokeJoinProc                       *syscall.Proc
	skPaintGetBlendModeProc                        *syscall.Proc
	skPaintSetBlendModeProc                        *syscall.Proc
	skPaintGetShaderProc                           *syscall.Proc
	skPaintSetShaderProc                           *syscall.Proc
	skPaintGetColorFilterProc                      *syscall.Proc
	skPaintSetColorFilterProc                      *syscall.Proc
	skPaintGetMaskFilterProc                       *syscall.Proc
	skPaintSetMaskFilterProc                       *syscall.Proc
	skPaintGetImageFilterProc                      *syscall.Proc
	skPaintSetImageFilterProc                      *syscall.Proc
	skPaintGetPathEffectProc                       *syscall.Proc
	skPaintSetPathEffectProc                       *syscall.Proc
	skPaintGetFillPathProc                         *syscall.Proc
	skPathNewProc                                  *syscall.Proc
	skPathIsEmptyProc                              *syscall.Proc
	skPathParseSVGStringProc                       *syscall.Proc
	skPathToSVGStringProc                          *syscall.Proc
	skPathGetFillTypeProc                          *syscall.Proc
	skPathSetFillTypeProc                          *syscall.Proc
	skPathArcToProc                                *syscall.Proc
	skPathArcToWithPointsProc                      *syscall.Proc
	skPathArcToWithOvalProc                        *syscall.Proc
	skPathRArcToProc                               *syscall.Proc
	skPathGetBoundsProc                            *syscall.Proc
	skPathComputeTightBoundsProc                   *syscall.Proc
	skPathAddCircleProc                            *syscall.Proc
	skPathCloneProc                                *syscall.Proc
	skPathCloseProc                                *syscall.Proc
	skPathConicToProc                              *syscall.Proc
	skPathRConicToProc                             *syscall.Proc
	skPathCubicToProc                              *syscall.Proc
	skPathRCubicToProc                             *syscall.Proc
	skPathLineToProc                               *syscall.Proc
	skPathRLineToProc                              *syscall.Proc
	skPathMoveToProc                               *syscall.Proc
	skPathRMoveToProc                              *syscall.Proc
	skPathAddOvalProc                              *syscall.Proc
	skPathAddPathProc                              *syscall.Proc
	skPathAddPathReverseProc                       *syscall.Proc
	skPathAddPathMatrixProc                        *syscall.Proc
	skPathAddPathOffsetProc                        *syscall.Proc
	skPathAddPolyProc                              *syscall.Proc
	skPathQuadToProc                               *syscall.Proc
	skPathAddRectProc                              *syscall.Proc
	skPathAddRoundedRectProc                       *syscall.Proc
	skPathTransformProc                            *syscall.Proc
	skPathTransformToDestProc                      *syscall.Proc
	skPathResetProc                                *syscall.Proc
	skPathRewindProc                               *syscall.Proc
	skPathContainsProc                             *syscall.Proc
	skPathGetLastPointProc                         *syscall.Proc
	skPathDeleteProc                               *syscall.Proc
	skPathOpProc                                   *syscall.Proc
	skPathSimplifyProc                             *syscall.Proc
	skPathEffectCreateComposeProc                  *syscall.Proc
	skPathEffectCreateSumProc                      *syscall.Proc
	skPathEffectCreateDiscreteProc                 *syscall.Proc
	skPathEffectCreateCornerProc                   *syscall.Proc
	skPathEffectCreate1dPathProc                   *syscall.Proc
	skPathEffectCreate2dLineProc                   *syscall.Proc
	skPathEffectCreate2dPathProc                   *syscall.Proc
	skPathEffectCreateDashProc                     *syscall.Proc
	skPathEffectCreateTrimProc                     *syscall.Proc
	skPathEffectUnrefProc                          *syscall.Proc
	skRegisterImageCodecsProc                      *syscall.Proc
	skShaderNewColorProc                           *syscall.Proc
	skShaderNewBlendProc                           *syscall.Proc
	skShaderNewLinearGradientProc                  *syscall.Proc
	skShaderNewRadialGradientProc                  *syscall.Proc
	skShaderNewSweepGradientProc                   *syscall.Proc
	skShaderNewTwoPointConicalGradientProc         *syscall.Proc
	skShaderNewPerlinNoiseFractalNoiseProc         *syscall.Proc
	skShaderNewPerlinNoiseTurbulenceProc           *syscall.Proc
	skShaderWithLocalMatrixProc                    *syscall.Proc
	skShaderWithColorFilterProc                    *syscall.Proc
	skShaderUnrefProc                              *syscall.Proc
	skStringNewProc                                *syscall.Proc
	skStringNewEmptyProc                           *syscall.Proc
	skStringGetCStrProc                            *syscall.Proc
	skStringGetSizeProc                            *syscall.Proc
	skStringDeleteProc                             *syscall.Proc
	skSurfaceMakeRasterDirectProc                  *syscall.Proc
	skSurfaceMakeRasterN32PreMulProc               *syscall.Proc
	skSurfaceNewBackendRenderTargetProc            *syscall.Proc
	skSurfaceMakeImageSnapshotProc                 *syscall.Proc
	skSurfaceGetCanvasProc                         *syscall.Proc
	skSurfaceUnrefProc                             *syscall.Proc
	skSurfacePropsNewProc                          *syscall.Proc
	skTextBlobMakeFromTextProc                     *syscall.Proc
	skTextBlobGetBoundsProc                        *syscall.Proc
	skTextBlobGetInterceptsProc                    *syscall.Proc
	skTextBlobUnrefProc                            *syscall.Proc
	skTextBlobBuilderNewProc                       *syscall.Proc
	skTextBlobBuilderMakeProc                      *syscall.Proc
	skTextBlobBuilderAllocRunProc                  *syscall.Proc
	skTextBlobBuilderAllocRunPosProc               *syscall.Proc
	skTextBlobBuilderAllocRunPosHProc              *syscall.Proc
	skTextBlobBuilderDeleteProc                    *syscall.Proc
	skTypeFaceGetFontStyleProc                     *syscall.Proc
	skTypeFaceIsFixedPitchProc                     *syscall.Proc
	skTypeFaceGetFamilyNameProc                    *syscall.Proc
	skTypeFaceGetUnitsPerEmProc                    *syscall.Proc
	skTypeFaceUnrefProc                            *syscall.Proc
)

// textBlobBuilderRunBuffer supplies storage for glyphs and positions within a run.
// It has the same layout as skia's SkTextBlobBuilder::RunBuffer type.
type textBlobBuilderRunBuffer struct {
	Glyphs   unsafe.Pointer
	Pos      unsafe.Pointer
	UTF8Text unsafe.Pointer
	Clusters unsafe.Pointer
}

func init() {
	dir, err := os.UserCacheDir()
	xos.ExitIfErr(err)
	dir = filepath.Join(dir, "unison", "dll_cache")
	xos.ExitIfErr(os.MkdirAll(dir, 0755))
	xos.ExitIfErr(windows.SetDllDirectory(dir))
	sha := sha256.Sum256(dllData)
	dllName := fmt.Sprintf("skia-%s.dll", base64.RawURLEncoding.EncodeToString(sha[:]))
	filePath := filepath.Join(dir, dllName)
	if !xos.FileExists(filePath) {
		xos.ExitIfErr(os.WriteFile(filePath, dllData, 0644))
	}
	skia := syscall.MustLoadDLL(dllName)
	grBackendRenderTargetNewGLProc = skia.MustFindProc("gr_backendrendertarget_new_gl")
	grBackendRenderTargetDeleteProc = skia.MustFindProc("gr_backendrendertarget_delete")
	grContextMakeGLProc = skia.MustFindProc("gr_direct_context_make_gl")
	grContextDeleteProc = skia.MustFindProc("gr_direct_context_delete")
	grContextFlushAndSubmitProc = skia.MustFindProc("gr_direct_context_flush_and_submit")
	grContextResetGLTextureBindings = skia.MustFindProc("gr_direct_context_reset_gl_texture_bindings")
	grContextReset = skia.MustFindProc("gr_direct_context_reset")
	grContextAbandonContextProc = skia.MustFindProc("gr_direct_context_abandon_context")
	grContextReleaseResourcesAndAbandonContextProc = skia.MustFindProc("gr_direct_context_release_resources_and_abandon_context")
	grContextUnrefProc = skia.MustFindProc("gr_direct_context_unref")
	grGLInterfaceCreateNativeInterfaceProc = skia.MustFindProc("gr_glinterface_create_native_interface")
	grGLInterfaceUnrefProc = skia.MustFindProc("gr_glinterface_unref")
	skCanvasGetSaveCountProc = skia.MustFindProc("sk_canvas_get_save_count")
	skCanvasSaveProc = skia.MustFindProc("sk_canvas_save")
	skCanvasSaveLayerProc = skia.MustFindProc("sk_canvas_save_layer")
	skCanvasSaveLayerAlphaProc = skia.MustFindProc("sk_canvas_save_layer_alpha")
	skCanvasRestoreProc = skia.MustFindProc("sk_canvas_restore")
	skCanvasRestoreToCountProc = skia.MustFindProc("sk_canvas_restore_to_count")
	skCanvasTranslateProc = skia.MustFindProc("sk_canvas_translate")
	skCanvasScaleProc = skia.MustFindProc("sk_canvas_scale")
	skCanvasRotateRadiansProc = skia.MustFindProc("sk_canvas_rotate_radians")
	skCanvasSkewProc = skia.MustFindProc("sk_canvas_skew")
	skCanvasConcatProc = skia.MustFindProc("sk_canvas_concat")
	skCanvasResetMatrixProc = skia.MustFindProc("sk_canvas_reset_matrix")
	skCanvasGetTotalMatrixProc = skia.MustFindProc("sk_canvas_get_total_matrix")
	skCanvasSetMatrixProc = skia.MustFindProc("sk_canvas_set_matrix")
	skCanvasQuickRejectPathProc = skia.MustFindProc("sk_canvas_quick_reject_path")
	skCanvasQuickRejectRectProc = skia.MustFindProc("sk_canvas_quick_reject_rect")
	skCanvasClearProc = skia.MustFindProc("sk_canvas_clear")
	skCanvasDrawPaintProc = skia.MustFindProc("sk_canvas_draw_paint")
	skCanvasDrawRectProc = skia.MustFindProc("sk_canvas_draw_rect")
	skCanvasDrawRoundRectProc = skia.MustFindProc("sk_canvas_draw_round_rect")
	skCanvasDrawCircleProc = skia.MustFindProc("sk_canvas_draw_circle")
	skCanvasDrawOvalProc = skia.MustFindProc("sk_canvas_draw_oval")
	skCanvasDrawPathProc = skia.MustFindProc("sk_canvas_draw_path")
	skCanvasDrawImageRectProc = skia.MustFindProc("sk_canvas_draw_image_rect")
	skCanvasDrawImageNineProc = skia.MustFindProc("sk_canvas_draw_image_nine")
	skCanvasDrawColorProc = skia.MustFindProc("sk_canvas_draw_color")
	skCanvasDrawPointProc = skia.MustFindProc("sk_canvas_draw_point")
	skCanvasDrawPointsProc = skia.MustFindProc("sk_canvas_draw_points")
	skCanvasDrawLineProc = skia.MustFindProc("sk_canvas_draw_line")
	skCanvasDrawArcProc = skia.MustFindProc("sk_canvas_draw_arc")
	skCanvasDrawSimpleTextProc = skia.MustFindProc("sk_canvas_draw_simple_text")
	skCanvasDrawTextBlobProc = skia.MustFindProc("sk_canvas_draw_text_blob")
	skCanavasClipRectWithOperationProc = skia.MustFindProc("sk_canvas_clip_rect_with_operation")
	skCanavasClipPathWithOperationProc = skia.MustFindProc("sk_canvas_clip_path_with_operation")
	skCanvasGetLocalClipBoundsProc = skia.MustFindProc("sk_canvas_get_local_clip_bounds")
	skCanvasGetSurfaceProc = skia.MustFindProc("sk_canvas_get_surface")
	skCanvasIsClipEmptyProc = skia.MustFindProc("sk_canvas_is_clip_empty")
	skCanvasIsClipRectProc = skia.MustFindProc("sk_canvas_is_clip_rect")
	skColorFilterNewModeProc = skia.MustFindProc("sk_colorfilter_new_mode")
	skColorFilterNewLightingProc = skia.MustFindProc("sk_colorfilter_new_lighting")
	skColorFilterNewComposeProc = skia.MustFindProc("sk_colorfilter_new_compose")
	skColorFilterNewColorMatrixProc = skia.MustFindProc("sk_colorfilter_new_color_matrix")
	skColorFilterNewLumaColorProc = skia.MustFindProc("sk_colorfilter_new_luma_color")
	skColorFilterNewHighContrastProc = skia.MustFindProc("sk_colorfilter_new_high_contrast")
	skColorFilterUnrefProc = skia.MustFindProc("sk_colorfilter_unref")
	skColorSpaceNewSRGBProc = skia.MustFindProc("sk_colorspace_new_srgb")
	skDataNewWithCopyProc = skia.MustFindProc("sk_data_new_with_copy")
	skDataGetSizeProc = skia.MustFindProc("sk_data_get_size")
	skDataGetDataProc = skia.MustFindProc("sk_data_get_data")
	skDataUnrefProc = skia.MustFindProc("sk_data_unref")
	skEncodeJPEGProc = skia.MustFindProc("sk_encode_jpeg")
	skEncodePNGProc = skia.MustFindProc("sk_encode_png")
	skEncodeWEBPProc = skia.MustFindProc("sk_encode_webp")
	skDocumentAbortProc = skia.MustFindProc("sk_document_abort")
	skDocumentBeginPageProc = skia.MustFindProc("sk_document_begin_page")
	skDocumentCloseProc = skia.MustFindProc("sk_document_close")
	skDocumentEndPageProc = skia.MustFindProc("sk_document_end_page")
	skDocumentMakePDFProc = skia.MustFindProc("sk_document_make_pdf")
	skDynamicMemoryWStreamNewProc = skia.MustFindProc("sk_dynamic_memory_wstream_new")
	skDynamicMemoryWStreamAsWStreamProc = skia.MustFindProc("sk_dynamic_memory_wstream_as_wstream")
	skDynamicMemoryWStreamWriteProc = skia.MustFindProc("sk_dynamic_memory_wstream_write")
	skDynamicMemoryWStreamBytesWrittenProc = skia.MustFindProc("sk_dynamic_memory_wstream_bytes_written")
	skDynamicMemoryWStreamReadProc = skia.MustFindProc("sk_dynamic_memory_wstream_read")
	skDynamicMemoryWStreamDeleteProc = skia.MustFindProc("sk_dynamic_memory_wstream_delete")
	skFileWStreamNewProc = skia.MustFindProc("sk_file_wstream_new")
	skFileWStreamAsWStreamProc = skia.MustFindProc("sk_file_wstream_as_wstream")
	skFileWStreamWriteProc = skia.MustFindProc("sk_file_wstream_write")
	skFileWStreamBytesWrittenProc = skia.MustFindProc("sk_file_wstream_bytes_written")
	skFileWStreamFlushProc = skia.MustFindProc("sk_file_wstream_flush")
	skFileWStreamDeleteProc = skia.MustFindProc("sk_file_wstream_delete")
	skFontNewWithValuesProc = skia.MustFindProc("sk_font_new_with_values")
	skFontSetSubPixelProc = skia.MustFindProc("sk_font_set_subpixel")
	skFontSetForceAutoHintingProc = skia.MustFindProc("sk_font_set_force_auto_hinting")
	skFontSetHintingProc = skia.MustFindProc("sk_font_set_hinting")
	skFontGetMetricsProc = skia.MustFindProc("sk_font_get_metrics")
	skFontMeasureTextProc = skia.MustFindProc("sk_font_measure_text")
	skFontTextToGlyphsProc = skia.MustFindProc("sk_font_text_to_glyphs")
	skFontUnicharToGlyphProc = skia.MustFindProc("sk_font_unichar_to_glyph")
	skFontUnicharsToGlyphsProc = skia.MustFindProc("sk_font_unichars_to_glyphs")
	skFontGlyphWidthsProc = skia.MustFindProc("sk_font_glyph_widths")
	skFontGetXPosProc = skia.MustFindProc("sk_font_get_xpos")
	skFontDeleteProc = skia.MustFindProc("sk_font_delete")
	skFontMgrRefDefaultProc = skia.MustFindProc("sk_fontmgr_ref_default")
	skFontMgrCreateFromDataProc = skia.MustFindProc("sk_fontmgr_create_from_data")
	skFontMgrMatchFamilyProc = skia.MustFindProc("sk_fontmgr_match_family")
	skFontMgrMatchFamilyStyleProc = skia.MustFindProc("sk_fontmgr_match_family_style")
	skFontMgrMatchFamilyStyleCharacterProc = skia.MustFindProc("sk_fontmgr_match_family_style_character")
	skFontMgrCountFamiliesProc = skia.MustFindProc("sk_fontmgr_count_families")
	skFontMgrGetFamilyNameProc = skia.MustFindProc("sk_fontmgr_get_family_name")
	skFontStyleNewProc = skia.MustFindProc("sk_fontstyle_new")
	skFontStyleGetWeightProc = skia.MustFindProc("sk_fontstyle_get_weight")
	skFontStyleGetWidthProc = skia.MustFindProc("sk_fontstyle_get_width")
	skFontStyleGetSlantProc = skia.MustFindProc("sk_fontstyle_get_slant")
	skFontStyleDeleteProc = skia.MustFindProc("sk_fontstyle_delete")
	skFontStyleSetGetCountProc = skia.MustFindProc("sk_fontstyleset_get_count")
	skFontStyleSetGetStyleProc = skia.MustFindProc("sk_fontstyleset_get_style")
	skFontStyleSetCreateTypeFaceProc = skia.MustFindProc("sk_fontstyleset_create_typeface")
	skFontStyleSetMatchStyleProc = skia.MustFindProc("sk_fontstyleset_match_style")
	skFontStyleSetUnrefProc = skia.MustFindProc("sk_fontstyleset_unref")
	skImageNewFromEncodedProc = skia.MustFindProc("sk_image_new_from_encoded")
	skImageNewRasterDataProc = skia.MustFindProc("sk_image_new_raster_data")
	skImageGetWidthProc = skia.MustFindProc("sk_image_get_width")
	skImageGetHeightProc = skia.MustFindProc("sk_image_get_height")
	skImageGetColorSpaceProc = skia.MustFindProc("sk_image_get_colorspace")
	skImageGetColorTypeProc = skia.MustFindProc("sk_image_get_color_type")
	skImageGetAlphaTypeProc = skia.MustFindProc("sk_image_get_alpha_type")
	skImageReadPixelsProc = skia.MustFindProc("sk_image_read_pixels")
	skImageMakeNonTextureImageProc = skia.MustFindProc("sk_image_make_non_texture_image")
	skImageMakeShaderProc = skia.MustFindProc("sk_image_make_shader")
	skImageTextureFromImageProc = skia.MustFindProc("sk_image_texture_from_image")
	skImageUnrefProc = skia.MustFindProc("sk_image_unref")
	skImageFilterNewArithmeticProc = skia.MustFindProc("sk_imagefilter_new_arithmetic")
	skImageFilterNewBlurProc = skia.MustFindProc("sk_imagefilter_new_blur")
	skImageFilterNewColorFilterProc = skia.MustFindProc("sk_imagefilter_new_color_filter")
	skImageFilterNewComposeProc = skia.MustFindProc("sk_imagefilter_new_compose")
	skImageFilterNewDisplacementMapEffectProc = skia.MustFindProc("sk_imagefilter_new_displacement_map_effect")
	skImageFilterNewDropShadowProc = skia.MustFindProc("sk_imagefilter_new_drop_shadow")
	skImageFilterNewDropShadowOnlyProc = skia.MustFindProc("sk_imagefilter_new_drop_shadow_only")
	skImageFilterNewImageSourceProc = skia.MustFindProc("sk_imagefilter_new_image_source")
	skImageFilterNewImageSourceDefaultProc = skia.MustFindProc("sk_imagefilter_new_image_source_default")
	skImageFilterNewMagnifierProc = skia.MustFindProc("sk_imagefilter_new_magnifier")
	skImageFilterNewMatrixConvolutionProc = skia.MustFindProc("sk_imagefilter_new_matrix_convolution")
	skImageFilterNewMatrixTransformProc = skia.MustFindProc("sk_imagefilter_new_matrix_transform")
	skImageFilterNewMergeProc = skia.MustFindProc("sk_imagefilter_new_merge")
	skImageFilterNewOffsetProc = skia.MustFindProc("sk_imagefilter_new_offset")
	skImageFilterNewTileProc = skia.MustFindProc("sk_imagefilter_new_tile")
	skImageFilterNewDilateProc = skia.MustFindProc("sk_imagefilter_new_dilate")
	skImageFilterNewErodeProc = skia.MustFindProc("sk_imagefilter_new_erode")
	skImageFilterNewDistantLitDiffuseProc = skia.MustFindProc("sk_imagefilter_new_distant_lit_diffuse")
	skImageFilterNewPointLitDiffuseProc = skia.MustFindProc("sk_imagefilter_new_point_lit_diffuse")
	skImageFilterNewSpotLitDiffuseProc = skia.MustFindProc("sk_imagefilter_new_spot_lit_diffuse")
	skImageFilterNewDistantLitSpecularProc = skia.MustFindProc("sk_imagefilter_new_distant_lit_specular")
	skImageFilterNewPointLitSpecularProc = skia.MustFindProc("sk_imagefilter_new_point_lit_specular")
	skImageFilterNewSpotLitSpecularProc = skia.MustFindProc("sk_imagefilter_new_spot_lit_specular")
	skImageFilterUnrefProc = skia.MustFindProc("sk_imagefilter_unref")
	skMaskFilterNewBlurWithFlagsProc = skia.MustFindProc("sk_maskfilter_new_blur_with_flags")
	skMaskFilterNewTableProc = skia.MustFindProc("sk_maskfilter_new_table")
	skMaskFilterNewGammaProc = skia.MustFindProc("sk_maskfilter_new_gamma")
	skMaskFilterNewClipProc = skia.MustFindProc("sk_maskfilter_new_clip")
	skMaskFilterNewShaderProc = skia.MustFindProc("sk_maskfilter_new_shader")
	skMaskFilterUnrefProc = skia.MustFindProc("sk_maskfilter_unref")
	skOpBuilderNewProc = skia.MustFindProc("sk_opbuilder_new")
	skOpBuilderAddProc = skia.MustFindProc("sk_opbuilder_add")
	skOpBuilderResolveProc = skia.MustFindProc("sk_opbuilder_resolve")
	skOpBuilderDestroyProc = skia.MustFindProc("sk_opbuilder_destroy")
	skPaintNewProc = skia.MustFindProc("sk_paint_new")
	skPaintDeleteProc = skia.MustFindProc("sk_paint_delete")
	skPaintCloneProc = skia.MustFindProc("sk_paint_clone")
	skPaintEquivalentProc = skia.MustFindProc("sk_paint_equivalent")
	skPaintResetProc = skia.MustFindProc("sk_paint_reset")
	skPaintIsAntialiasProc = skia.MustFindProc("sk_paint_is_antialias")
	skPaintSetAntialiasProc = skia.MustFindProc("sk_paint_set_antialias")
	skPaintIsDitherProc = skia.MustFindProc("sk_paint_is_dither")
	skPaintSetDitherProc = skia.MustFindProc("sk_paint_set_dither")
	skPaintGetColorProc = skia.MustFindProc("sk_paint_get_color")
	skPaintSetColorProc = skia.MustFindProc("sk_paint_set_color")
	skPaintGetStyleProc = skia.MustFindProc("sk_paint_get_style")
	skPaintSetStyleProc = skia.MustFindProc("sk_paint_set_style")
	skPaintGetStrokeWidthProc = skia.MustFindProc("sk_paint_get_stroke_width")
	skPaintSetStrokeWidthProc = skia.MustFindProc("sk_paint_set_stroke_width")
	skPaintGetStrokeMiterProc = skia.MustFindProc("sk_paint_get_stroke_miter")
	skPaintSetStrokeMiterProc = skia.MustFindProc("sk_paint_set_stroke_miter")
	skPaintGetStrokeCapProc = skia.MustFindProc("sk_paint_get_stroke_cap")
	skPaintSetStrokeCapProc = skia.MustFindProc("sk_paint_set_stroke_cap")
	skPaintGetStrokeJoinProc = skia.MustFindProc("sk_paint_get_stroke_join")
	skPaintSetStrokeJoinProc = skia.MustFindProc("sk_paint_set_stroke_join")
	skPaintGetBlendModeProc = skia.MustFindProc("sk_paint_get_blend_mode_or")
	skPaintSetBlendModeProc = skia.MustFindProc("sk_paint_set_blend_mode")
	skPaintGetShaderProc = skia.MustFindProc("sk_paint_get_shader")
	skPaintSetShaderProc = skia.MustFindProc("sk_paint_set_shader")
	skPaintGetColorFilterProc = skia.MustFindProc("sk_paint_get_colorfilter")
	skPaintSetColorFilterProc = skia.MustFindProc("sk_paint_set_colorfilter")
	skPaintGetMaskFilterProc = skia.MustFindProc("sk_paint_get_maskfilter")
	skPaintSetMaskFilterProc = skia.MustFindProc("sk_paint_set_maskfilter")
	skPaintGetImageFilterProc = skia.MustFindProc("sk_paint_get_imagefilter")
	skPaintSetImageFilterProc = skia.MustFindProc("sk_paint_set_imagefilter")
	skPaintGetPathEffectProc = skia.MustFindProc("sk_paint_get_path_effect")
	skPaintSetPathEffectProc = skia.MustFindProc("sk_paint_set_path_effect")
	skPaintGetFillPathProc = skia.MustFindProc("sk_paint_get_fill_path")
	skPathNewProc = skia.MustFindProc("sk_path_new")
	skPathIsEmptyProc = skia.MustFindProc("sk_path_is_empty")
	skPathParseSVGStringProc = skia.MustFindProc("sk_path_parse_svg_string")
	skPathToSVGStringProc = skia.MustFindProc("sk_path_to_svg_string")
	skPathGetFillTypeProc = skia.MustFindProc("sk_path_get_filltype")
	skPathSetFillTypeProc = skia.MustFindProc("sk_path_set_filltype")
	skPathArcToProc = skia.MustFindProc("sk_path_arc_to")
	skPathArcToWithPointsProc = skia.MustFindProc("sk_path_arc_to_with_points")
	skPathArcToWithOvalProc = skia.MustFindProc("sk_path_arc_to_with_oval")
	skPathRArcToProc = skia.MustFindProc("sk_path_rarc_to")
	skPathGetBoundsProc = skia.MustFindProc("sk_path_get_bounds")
	skPathComputeTightBoundsProc = skia.MustFindProc("sk_path_compute_tight_bounds")
	skPathAddCircleProc = skia.MustFindProc("sk_path_add_circle")
	skPathCloneProc = skia.MustFindProc("sk_path_clone")
	skPathCloseProc = skia.MustFindProc("sk_path_close")
	skPathConicToProc = skia.MustFindProc("sk_path_conic_to")
	skPathRConicToProc = skia.MustFindProc("sk_path_rconic_to")
	skPathCubicToProc = skia.MustFindProc("sk_path_cubic_to")
	skPathRCubicToProc = skia.MustFindProc("sk_path_rcubic_to")
	skPathLineToProc = skia.MustFindProc("sk_path_line_to")
	skPathRLineToProc = skia.MustFindProc("sk_path_rline_to")
	skPathMoveToProc = skia.MustFindProc("sk_path_move_to")
	skPathRMoveToProc = skia.MustFindProc("sk_path_rmove_to")
	skPathAddOvalProc = skia.MustFindProc("sk_path_add_oval")
	skPathAddPathProc = skia.MustFindProc("sk_path_add_path")
	skPathAddPathReverseProc = skia.MustFindProc("sk_path_add_path_reverse")
	skPathAddPathMatrixProc = skia.MustFindProc("sk_path_add_path_matrix")
	skPathAddPathOffsetProc = skia.MustFindProc("sk_path_add_path_offset")
	skPathAddPolyProc = skia.MustFindProc("sk_path_add_poly")
	skPathQuadToProc = skia.MustFindProc("sk_path_quad_to")
	skPathAddRectProc = skia.MustFindProc("sk_path_add_rect")
	skPathAddRoundedRectProc = skia.MustFindProc("sk_path_add_rounded_rect")
	skPathTransformProc = skia.MustFindProc("sk_path_transform")
	skPathTransformToDestProc = skia.MustFindProc("sk_path_transform_to_dest")
	skPathResetProc = skia.MustFindProc("sk_path_reset")
	skPathRewindProc = skia.MustFindProc("sk_path_rewind")
	skPathContainsProc = skia.MustFindProc("sk_path_contains")
	skPathGetLastPointProc = skia.MustFindProc("sk_path_get_last_point")
	skPathDeleteProc = skia.MustFindProc("sk_path_delete")
	skPathOpProc = skia.MustFindProc("sk_path_op")
	skPathSimplifyProc = skia.MustFindProc("sk_path_simplify")
	skPathEffectCreateComposeProc = skia.MustFindProc("sk_path_effect_create_compose")
	skPathEffectCreateSumProc = skia.MustFindProc("sk_path_effect_create_sum")
	skPathEffectCreateDiscreteProc = skia.MustFindProc("sk_path_effect_create_discrete")
	skPathEffectCreateCornerProc = skia.MustFindProc("sk_path_effect_create_corner")
	skPathEffectCreate1dPathProc = skia.MustFindProc("sk_path_effect_create_1d_path")
	skPathEffectCreate2dLineProc = skia.MustFindProc("sk_path_effect_create_2d_line")
	skPathEffectCreate2dPathProc = skia.MustFindProc("sk_path_effect_create_2d_path")
	skPathEffectCreateDashProc = skia.MustFindProc("sk_path_effect_create_dash")
	skPathEffectCreateTrimProc = skia.MustFindProc("sk_path_effect_create_trim")
	skPathEffectUnrefProc = skia.MustFindProc("sk_path_effect_unref")
	skRegisterImageCodecsProc = skia.MustFindProc("register_image_codecs")
	skShaderNewColorProc = skia.MustFindProc("sk_shader_new_color")
	skShaderNewBlendProc = skia.MustFindProc("sk_shader_new_blend")
	skShaderNewLinearGradientProc = skia.MustFindProc("sk_shader_new_linear_gradient")
	skShaderNewRadialGradientProc = skia.MustFindProc("sk_shader_new_radial_gradient")
	skShaderNewSweepGradientProc = skia.MustFindProc("sk_shader_new_sweep_gradient")
	skShaderNewTwoPointConicalGradientProc = skia.MustFindProc("sk_shader_new_two_point_conical_gradient")
	skShaderNewPerlinNoiseFractalNoiseProc = skia.MustFindProc("sk_shader_new_perlin_noise_fractal_noise")
	skShaderNewPerlinNoiseTurbulenceProc = skia.MustFindProc("sk_shader_new_perlin_noise_turbulence")
	skShaderWithLocalMatrixProc = skia.MustFindProc("sk_shader_with_local_matrix")
	skShaderWithColorFilterProc = skia.MustFindProc("sk_shader_with_color_filter")
	skShaderUnrefProc = skia.MustFindProc("sk_shader_unref")
	skStringNewProc = skia.MustFindProc("sk_string_new")
	skStringNewEmptyProc = skia.MustFindProc("sk_string_new_empty")
	skStringGetCStrProc = skia.MustFindProc("sk_string_get_c_str")
	skStringGetSizeProc = skia.MustFindProc("sk_string_get_size")
	skStringDeleteProc = skia.MustFindProc("sk_string_delete")
	skSurfaceMakeRasterDirectProc = skia.MustFindProc("sk_surface_make_raster_direct")
	skSurfaceMakeRasterN32PreMulProc = skia.MustFindProc("sk_surface_make_raster_n32_premul")
	skSurfaceNewBackendRenderTargetProc = skia.MustFindProc("sk_surface_new_backend_render_target")
	skSurfaceMakeImageSnapshotProc = skia.MustFindProc("sk_surface_make_image_snapshot")
	skSurfaceGetCanvasProc = skia.MustFindProc("sk_surface_get_canvas")
	skSurfaceUnrefProc = skia.MustFindProc("sk_surface_unref")
	skSurfacePropsNewProc = skia.MustFindProc("sk_surfaceprops_new")
	skTextBlobMakeFromTextProc = skia.MustFindProc("sk_textblob_make_from_text")
	skTextBlobGetBoundsProc = skia.MustFindProc("sk_textblob_get_bounds")
	skTextBlobGetInterceptsProc = skia.MustFindProc("sk_textblob_get_intercepts")
	skTextBlobUnrefProc = skia.MustFindProc("sk_textblob_unref")
	skTextBlobBuilderNewProc = skia.MustFindProc("sk_textblob_builder_new")
	skTextBlobBuilderMakeProc = skia.MustFindProc("sk_textblob_builder_make")
	skTextBlobBuilderAllocRunProc = skia.MustFindProc("sk_textblob_builder_alloc_run")
	skTextBlobBuilderAllocRunPosProc = skia.MustFindProc("sk_textblob_builder_alloc_run_pos")
	skTextBlobBuilderAllocRunPosHProc = skia.MustFindProc("sk_textblob_builder_alloc_run_pos_h")
	skTextBlobBuilderDeleteProc = skia.MustFindProc("sk_textblob_builder_delete")
	skTypeFaceGetFontStyleProc = skia.MustFindProc("sk_typeface_get_fontstyle")
	skTypeFaceIsFixedPitchProc = skia.MustFindProc("sk_typeface_is_fixed_pitch")
	skTypeFaceGetFamilyNameProc = skia.MustFindProc("sk_typeface_get_family_name")
	skTypeFaceGetUnitsPerEmProc = skia.MustFindProc("sk_typeface_get_units_per_em")
	skTypeFaceUnrefProc = skia.MustFindProc("sk_typeface_unref")
}

type (
	BackendRenderTarget  unsafe.Pointer
	DirectContext        unsafe.Pointer
	GLInterface          unsafe.Pointer
	Canvas               unsafe.Pointer
	ColorFilter          unsafe.Pointer
	ColorSpace           unsafe.Pointer
	Data                 unsafe.Pointer
	Document             unsafe.Pointer
	DynamicMemoryWStream unsafe.Pointer
	FileWStream          unsafe.Pointer
	Font                 unsafe.Pointer
	FontMgr              unsafe.Pointer
	FontStyle            unsafe.Pointer
	FontStyleSet         unsafe.Pointer
	Image                unsafe.Pointer
	ImageFilter          unsafe.Pointer
	MaskFilter           unsafe.Pointer
	OpBuilder            unsafe.Pointer
	Paint                unsafe.Pointer
	Path                 unsafe.Pointer
	PathEffect           unsafe.Pointer
	SamplingOptions      uintptr
	Shader               unsafe.Pointer
	String               unsafe.Pointer
	Surface              unsafe.Pointer
	SurfaceProps         unsafe.Pointer
	TextBlob             unsafe.Pointer
	TextBlobBuilder      unsafe.Pointer
	TypeFace             unsafe.Pointer
	WStream              unsafe.Pointer
)

func fromGeomMatrix(m *geom.Matrix) uintptr {
	if m == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(&Matrix{Matrix: *m, Persp2: 1}))
}

func fromGeomRect(r *geom.Rect) uintptr {
	if r == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(&Rect{Left: r.X, Top: r.Y, Right: r.Right(), Bottom: r.Bottom()}))
}

func BackendRenderTargetNewGL(width, height, samples, stencilBits int, info *GLFrameBufferInfo) BackendRenderTarget {
	r1, _, _ := grBackendRenderTargetNewGLProc.Call(uintptr(width), uintptr(height), uintptr(samples),
		uintptr(stencilBits), uintptr(unsafe.Pointer(info)))
	return BackendRenderTarget(r1)
}

func BackendRenderTargetDelete(backend BackendRenderTarget) {
	grBackendRenderTargetDeleteProc.Call(uintptr(backend))
}

func ContextMakeGL(gl GLInterface) DirectContext {
	r1, _, _ := grContextMakeGLProc.Call(uintptr(gl))
	return DirectContext(r1)
}

func ContextDelete(ctx DirectContext) {
	grContextDeleteProc.Call(uintptr(ctx))
}

func ContextFlushAndSubmit(ctx DirectContext, syncCPU bool) {
	grContextFlushAndSubmitProc.Call(uintptr(ctx), boolToUintptr(syncCPU))
}

func ContextResetGLTextureBindings(ctx DirectContext) {
	grContextResetGLTextureBindings.Call(uintptr(ctx))
}

func ContextReset(ctx DirectContext) {
	grContextReset.Call(uintptr(ctx))
}

func ContextAbandonContext(ctx DirectContext) {
	grContextAbandonContextProc.Call(uintptr(ctx))
}

func ContextReleaseResourcesAndAbandonContext(ctx DirectContext) {
	grContextReleaseResourcesAndAbandonContextProc.Call(uintptr(ctx))
}

func ContextUnref(ctx DirectContext) {
	grContextUnrefProc.Call(uintptr(ctx))
}

func GLInterfaceCreateNativeInterface() GLInterface {
	r1, _, _ := grGLInterfaceCreateNativeInterfaceProc.Call()
	return GLInterface(r1)
}

func GLInterfaceUnref(intf GLInterface) {
	grGLInterfaceUnrefProc.Call(uintptr(intf))
}

func CanvasGetSaveCount(canvas Canvas) int {
	r1, _, _ := skCanvasGetSaveCountProc.Call(uintptr(canvas))
	return int(r1)
}

func CanvasSave(canvas Canvas) int {
	r1, _, _ := skCanvasSaveProc.Call(uintptr(canvas))
	return int(r1)
}

func CanvasSaveLayer(canvas Canvas, paint Paint) int {
	r1, _, _ := skCanvasSaveLayerProc.Call(uintptr(canvas), 0, uintptr(paint))
	return int(r1)
}

func CanvasSaveLayerAlpha(canvas Canvas, opacity byte) int {
	r1, _, _ := skCanvasSaveLayerAlphaProc.Call(uintptr(canvas), 0, uintptr(opacity))
	return int(r1)
}

func CanvasRestore(canvas Canvas) {
	skCanvasRestoreProc.Call(uintptr(canvas))
}

func CanvasRestoreToCount(canvas Canvas, count int) {
	skCanvasRestoreToCountProc.Call(uintptr(canvas), uintptr(count))
}

func CanvasTranslate(canvas Canvas, offset geom.Point) {
	skCanvasTranslateProc.Call(uintptr(canvas), uintptr(math.Float32bits(offset.X)),
		uintptr(math.Float32bits(offset.Y)))
}

func CanvasScale(canvas Canvas, scale geom.Point) {
	skCanvasScaleProc.Call(uintptr(canvas), uintptr(math.Float32bits(scale.X)), uintptr(math.Float32bits(scale.Y)))
}

func CanvasRotateRadians(canvas Canvas, radians float32) {
	skCanvasRotateRadiansProc.Call(uintptr(canvas), uintptr(math.Float32bits(radians)))
}

func CanvasSkew(canvas Canvas, skew geom.Point) {
	skCanvasSkewProc.Call(uintptr(canvas), uintptr(math.Float32bits(skew.X)), uintptr(math.Float32bits(skew.Y)))
}

func CanvasConcat(canvas Canvas, matrix geom.Matrix) {
	skCanvasConcatProc.Call(uintptr(canvas), fromGeomMatrix(&matrix))
}

func CanvasResetMatrix(canvas Canvas) {
	skCanvasResetMatrixProc.Call(uintptr(canvas))
}

func CanvasGetTotalMatrix(canvas Canvas) geom.Matrix {
	var matrix Matrix
	skCanvasGetTotalMatrixProc.Call(uintptr(canvas), uintptr(unsafe.Pointer(&matrix)))
	return matrix.Matrix
}

func CanvasSetMatrix(canvas Canvas, matrix geom.Matrix) {
	skCanvasSetMatrixProc.Call(uintptr(canvas), fromGeomMatrix(&matrix))
}

func CanvasQuickRejectPath(canvas Canvas, path Path) bool {
	r1, _, _ := skCanvasQuickRejectPathProc.Call(uintptr(canvas), uintptr(path))
	return r1&0xff != 0
}

func CanvasQuickRejectRect(canvas Canvas, rect geom.Rect) bool {
	r1, _, _ := skCanvasQuickRejectRectProc.Call(uintptr(canvas), fromGeomRect(&rect))
	return r1&0xff != 0
}

func CanvasClear(canvas Canvas, color Color) {
	skCanvasClearProc.Call(uintptr(canvas), uintptr(color))
}

func CanvasDrawPaint(canvas Canvas, paint Paint) {
	skCanvasDrawPaintProc.Call(uintptr(canvas), uintptr(paint))
}

func CanvasDrawRect(canvas Canvas, rect geom.Rect, paint Paint) {
	skCanvasDrawRectProc.Call(uintptr(canvas), fromGeomRect(&rect), uintptr(paint))
}

func CanvasDrawRoundRect(canvas Canvas, rect geom.Rect, radius geom.Size, paint Paint) {
	skCanvasDrawRoundRectProc.Call(uintptr(canvas), fromGeomRect(&rect), uintptr(math.Float32bits(radius.Width)),
		uintptr(math.Float32bits(radius.Height)), uintptr(paint))
}

func CanvasDrawCircle(canvas Canvas, center geom.Point, radius float32, paint Paint) {
	skCanvasDrawCircleProc.Call(uintptr(canvas), uintptr(math.Float32bits(center.X)),
		uintptr(math.Float32bits(center.Y)), uintptr(math.Float32bits(radius)), uintptr(paint))
}

func CanvasDrawOval(canvas Canvas, rect geom.Rect, paint Paint) {
	skCanvasDrawOvalProc.Call(uintptr(canvas), fromGeomRect(&rect), uintptr(paint))
}

func CanvasDrawPath(canvas Canvas, path Path, paint Paint) {
	skCanvasDrawPathProc.Call(uintptr(canvas), uintptr(path), uintptr(paint))
}

func CanvasDrawImageRect(canvas Canvas, img Image, srcRect, dstRect geom.Rect, sampling SamplingOptions, paint Paint) {
	skCanvasDrawImageRectProc.Call(uintptr(canvas), uintptr(img), fromGeomRect(&srcRect), fromGeomRect(&dstRect),
		uintptr(sampling), uintptr(paint))
}

func CanvasDrawImageNine(canvas Canvas, img Image, centerRect, dstRect geom.Rect, filter FilterMode, paint Paint) {
	centerRect = centerRect.Align()
	skCanvasDrawImageNineProc.Call(uintptr(canvas), uintptr(img), uintptr(unsafe.Pointer(&IRect{
		Left:   int32(centerRect.X),
		Top:    int32(centerRect.Y),
		Right:  int32(centerRect.Right()),
		Bottom: int32(centerRect.Bottom()),
	})), fromGeomRect(&dstRect), uintptr(filter), uintptr(paint))
}

func CanvasDrawColor(canvas Canvas, color Color, mode BlendMode) {
	skCanvasDrawColorProc.Call(uintptr(canvas), uintptr(color), uintptr(mode))
}

func CanvasDrawPoint(canvas Canvas, pt geom.Point, paint Paint) {
	skCanvasDrawPointProc.Call(uintptr(canvas), uintptr(math.Float32bits(pt.X)), uintptr(math.Float32bits(pt.Y)),
		uintptr(paint))
}

func CanvasDrawPoints(canvas Canvas, mode PointMode, pts []geom.Point, paint Paint) {
	skCanvasDrawPointsProc.Call(uintptr(canvas), uintptr(mode), uintptr(len(pts)),
		uintptr(unsafe.Pointer(&pts[0])), uintptr(paint))
}

func CanvasDrawLine(canvas Canvas, start, end geom.Point, paint Paint) {
	skCanvasDrawLineProc.Call(uintptr(canvas), uintptr(math.Float32bits(start.X)), uintptr(math.Float32bits(start.Y)),
		uintptr(math.Float32bits(end.X)), uintptr(math.Float32bits(end.Y)), uintptr(paint))
}

func CanvasDrawArc(canvas Canvas, oval geom.Rect, startAngle, sweepAngle float32, useCenter bool, paint Paint) {
	skCanvasDrawArcProc.Call(uintptr(canvas), fromGeomRect(&oval), uintptr(math.Float32bits(startAngle)),
		uintptr(math.Float32bits(sweepAngle)), boolToUintptr(useCenter), uintptr(paint))
}

func CanvasDrawSimpleText(canvas Canvas, str string, pt geom.Point, font Font, paint Paint) {
	b := []byte(str)
	skCanvasDrawSimpleTextProc.Call(uintptr(canvas), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)),
		uintptr(TextEncodingUTF8), uintptr(math.Float32bits(pt.X)), uintptr(math.Float32bits(pt.Y)), uintptr(font),
		uintptr(paint))
}

func CanvasDrawTextBlob(canvas Canvas, txt TextBlob, pt geom.Point, paint Paint) {
	skCanvasDrawTextBlobProc.Call(uintptr(canvas), uintptr(txt), uintptr(math.Float32bits(pt.X)),
		uintptr(math.Float32bits(pt.Y)), uintptr(paint))
}

func CanavasClipRectWithOperation(canvas Canvas, rect geom.Rect, op ClipOp, antialias bool) {
	skCanavasClipRectWithOperationProc.Call(uintptr(canvas), fromGeomRect(&rect), uintptr(op), boolToUintptr(antialias))
}

func CanavasClipPathWithOperation(canvas Canvas, path Path, op ClipOp, antialias bool) {
	skCanavasClipPathWithOperationProc.Call(uintptr(canvas), uintptr(path), uintptr(op), boolToUintptr(antialias))
}

func CanvasGetLocalClipBounds(canvas Canvas) geom.Rect {
	var r Rect
	skCanvasGetLocalClipBoundsProc.Call(uintptr(canvas), uintptr(unsafe.Pointer(&r)))
	return toGeomRect(r)
}

func CanvasGetSurface(canvas Canvas) Surface {
	r1, _, _ := skCanvasGetSurfaceProc.Call(uintptr(canvas))
	return Surface(r1)
}

func CanvasIsClipEmpty(canvas Canvas) bool {
	r1, _, _ := skCanvasIsClipEmptyProc.Call(uintptr(canvas))
	return r1&0xff != 0
}

func CanvasIsClipRect(canvas Canvas) bool {
	r1, _, _ := skCanvasIsClipRectProc.Call(uintptr(canvas))
	return r1&0xff != 0
}

func ColorFilterNewMode(color Color, blendMode BlendMode) ColorFilter {
	r1, _, _ := skColorFilterNewModeProc.Call(uintptr(color), uintptr(blendMode))
	return ColorFilter(r1)
}

func ColorFilterNewLighting(mul, add Color) ColorFilter {
	r1, _, _ := skColorFilterNewLightingProc.Call(uintptr(mul), uintptr(add))
	return ColorFilter(r1)
}

func ColorFilterNewCompose(outer, inner ColorFilter) ColorFilter {
	r1, _, _ := skColorFilterNewComposeProc.Call(uintptr(outer), uintptr(inner))
	return ColorFilter(r1)
}

func ColorFilterNewColorMatrix(array []float32) ColorFilter {
	r1, _, _ := skColorFilterNewColorMatrixProc.Call(uintptr(unsafe.Pointer(&array[0])))
	return ColorFilter(r1)
}

func ColorFilterNewLumaColor() ColorFilter {
	r1, _, _ := skColorFilterNewLumaColorProc.Call()
	return ColorFilter(r1)
}

func ColorFilterNewHighContrast(config *HighContrastConfig) ColorFilter {
	r1, _, _ := skColorFilterNewHighContrastProc.Call(uintptr(unsafe.Pointer(config)))
	return ColorFilter(r1)
}

func ColorFilterUnref(filter ColorFilter) {
	skColorFilterUnrefProc.Call(uintptr(filter))
}

func ColorSpaceNewSRGB() ColorSpace {
	r1, _, _ := skColorSpaceNewSRGBProc.Call()
	return ColorSpace(r1)
}

func DataNewWithCopy(data []byte) Data {
	r1, _, _ := skDataNewWithCopyProc.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	return Data(r1)
}

func DataGetSize(data Data) int {
	r1, _, _ := skDataGetSizeProc.Call(uintptr(data))
	return int(r1)
}

func DataGetData(data Data) unsafe.Pointer {
	r1, _, _ := skDataGetDataProc.Call(uintptr(data))
	return unsafe.Pointer(r1)
}

func DataUnref(data Data) {
	skDataUnrefProc.Call(uintptr(data))
}

func EncodeJPEG(ctx DirectContext, img Image, quality int) Data {
	r1, _, _ := skEncodeJPEGProc.Call(uintptr(ctx), uintptr(img), uintptr(quality))
	return Data(r1)
}

func EncodePNG(ctx DirectContext, img Image, compressionLevel int) Data {
	r1, _, _ := skEncodePNGProc.Call(uintptr(ctx), uintptr(img), uintptr(compressionLevel))
	return Data(r1)
}

func EncodeWebp(ctx DirectContext, img Image, quality float32, lossy bool) Data {
	r1, _, _ := skEncodeWEBPProc.Call(uintptr(ctx), uintptr(img), uintptr(math.Float32bits(quality)), boolToUintptr(lossy))
	return Data(r1)
}

func DocumentMakePDF(stream WStream, metadata *MetaData) Document {
	var md metaData
	md.set(metadata)
	r1, _, _ := skDocumentMakePDFProc.Call(uintptr(stream), uintptr(unsafe.Pointer(&md)))
	return Document(r1)
}

func DocumentBeginPage(doc Document, size geom.Size) Canvas {
	r1, _, _ := skDocumentBeginPageProc.Call(uintptr(doc), uintptr(math.Float32bits(size.Width)),
		uintptr(math.Float32bits(size.Height)))
	return Canvas(r1)
}

func DocumentEndPage(doc Document) {
	skDocumentEndPageProc.Call(uintptr(doc))
}

func DocumentClose(doc Document) {
	skDocumentCloseProc.Call(uintptr(doc))
}

func DocumentAbort(doc Document) {
	skDocumentAbortProc.Call(uintptr(doc))
}

func DynamicMemoryWStreamNew() DynamicMemoryWStream {
	r1, _, _ := skDynamicMemoryWStreamNewProc.Call()
	return DynamicMemoryWStream(r1)
}

func DynamicMemoryWStreamAsWStream(s DynamicMemoryWStream) WStream {
	r1, _, _ := skDynamicMemoryWStreamAsWStreamProc.Call(uintptr(s))
	return WStream(r1)
}

func DynamicMemoryWStreamWrite(s DynamicMemoryWStream, data []byte) bool {
	r1, _, _ := skDynamicMemoryWStreamWriteProc.Call(uintptr(s), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	return r1&0xff != 0
}

func DynamicMemoryWStreamBytesWritten(s DynamicMemoryWStream) int {
	r1, _, _ := skDynamicMemoryWStreamBytesWrittenProc.Call(uintptr(s))
	return int(r1)
}

func DynamicMemoryWStreamRead(s DynamicMemoryWStream, data []byte) int {
	r1, _, _ := skDynamicMemoryWStreamReadProc.Call(uintptr(s), uintptr(unsafe.Pointer(&data[0])), 0, uintptr(len(data)))
	return int(r1)
}

func DynamicMemoryWStreamDelete(s DynamicMemoryWStream) {
	skDynamicMemoryWStreamDeleteProc.Call(uintptr(s))
}

func FileWStreamNew(filePath string) FileWStream {
	cstr := make([]byte, len(filePath)+1)
	copy(cstr, filePath)
	r1, _, _ := skFileWStreamNewProc.Call(uintptr(unsafe.Pointer(&cstr[0])))
	return FileWStream(r1)
}

func FileWStreamAsWStream(s FileWStream) WStream {
	r1, _, _ := skFileWStreamAsWStreamProc.Call(uintptr(s))
	return WStream(r1)
}

func FileWStreamWrite(s FileWStream, data []byte) bool {
	r1, _, _ := skFileWStreamWriteProc.Call(uintptr(s), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	return r1&0xff != 0
}

func FileWStreamBytesWritten(s FileWStream) int {
	r1, _, _ := skFileWStreamBytesWrittenProc.Call(uintptr(s))
	return int(r1)
}

func FileWStreamFlush(s FileWStream) {
	skFileWStreamFlushProc.Call(uintptr(s))
}

func FileWStreamDelete(s FileWStream) {
	skFileWStreamDeleteProc.Call(uintptr(s))
}

func FontNewWithValues(face TypeFace, size, scaleX, skewX float32) Font {
	r1, _, _ := skFontNewWithValuesProc.Call(uintptr(face), uintptr(math.Float32bits(size)),
		uintptr(math.Float32bits(scaleX)), uintptr(math.Float32bits(skewX)))
	return Font(r1)
}

func FontSetSubPixel(font Font, enabled bool) {
	skFontSetSubPixelProc.Call(uintptr(font), boolToUintptr(enabled))
}

func FontSetForceAutoHinting(font Font, enabled bool) {
	skFontSetForceAutoHintingProc.Call(uintptr(font), boolToUintptr(enabled))
}

func FontSetHinting(font Font, hinting FontHinting) {
	skFontSetHintingProc.Call(uintptr(font), uintptr(hinting))
}

func FontGetMetrics(font Font, metrics *FontMetrics) {
	skFontGetMetricsProc.Call(uintptr(font), uintptr(unsafe.Pointer(metrics)))
}

func FontMeasureText(font Font, str string) float32 {
	b := []byte(str)
	_, r2, _ := skFontMeasureTextProc.Call(uintptr(font), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(TextEncodingUTF8), 0, 0)
	return math.Float32frombits(uint32(r2))
}

func FontTextToGlyphs(font Font, str string) []uint16 {
	b := []byte(str)
	glyphs := make([]uint16, len(str))
	r1, _, _ := skFontTextToGlyphsProc.Call(uintptr(font), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(TextEncodingUTF8), uintptr(unsafe.Pointer(&glyphs[0])), uintptr(len(glyphs)))
	glyphs = glyphs[:int(r1)]
	return glyphs
}

func FontRuneToGlyph(font Font, r rune) uint16 {
	r1, _, _ := skFontUnicharToGlyphProc.Call(uintptr(font), uintptr(r))
	return uint16(r1)
}

func FontRunesToGlyphs(font Font, r []rune) []uint16 {
	glyphs := make([]uint16, len(r))
	skFontUnicharsToGlyphsProc.Call(uintptr(font), uintptr(unsafe.Pointer(&r[0])), uintptr(len(r)), uintptr(unsafe.Pointer(&glyphs[0])))
	return glyphs
}

func FontGlyphWidths(font Font, glyphs []uint16) []float32 {
	widths := make([]float32, len(glyphs))
	skFontGlyphWidthsProc.Call(uintptr(font), uintptr(unsafe.Pointer(&glyphs[0])), uintptr(len(glyphs)), uintptr(unsafe.Pointer(&widths[0])))
	return widths
}

func FontGlyphsXPos(font Font, glyphs []uint16) []float32 {
	pos := make([]float32, len(glyphs)+1)
	g2 := make([]uint16, len(glyphs)+1)
	copy(g2, glyphs)
	skFontGetXPosProc.Call(uintptr(font), uintptr(unsafe.Pointer(&g2[0])), uintptr(len(g2)), uintptr(unsafe.Pointer(&pos[0])), 0)
	return pos
}

func FontDelete(font Font) {
	skFontDeleteProc.Call(uintptr(font))
}

func FontMgrRefDefault() FontMgr {
	r1, _, _ := skFontMgrRefDefaultProc.Call()
	return FontMgr(r1)
}

func FontMgrCreateFromData(mgr FontMgr, data Data) TypeFace {
	r1, _, _ := skFontMgrCreateFromDataProc.Call(uintptr(mgr), uintptr(data), 0)
	return TypeFace(r1)
}

func FontMgrMatchFamily(mgr FontMgr, family string) FontStyleSet {
	cstr := make([]byte, len(family)+1)
	copy(cstr, family)
	r1, _, _ := skFontMgrMatchFamilyProc.Call(uintptr(mgr), uintptr(unsafe.Pointer(&cstr[0])))
	return FontStyleSet(r1)
}

func FontMgrMatchFamilyStyle(mgr FontMgr, family string, style FontStyle) TypeFace {
	cstr := make([]byte, len(family)+1)
	copy(cstr, family)
	r1, _, _ := skFontMgrMatchFamilyStyleProc.Call(uintptr(mgr), uintptr(unsafe.Pointer(&cstr[0])), uintptr(style))
	return TypeFace(r1)
}

func FontMgrMatchFamilyStyleCharacter(mgr FontMgr, family string, style FontStyle, ch rune) TypeFace {
	cstr := make([]byte, len(family)+1)
	copy(cstr, family)
	r1, _, _ := skFontMgrMatchFamilyStyleCharacterProc.Call(uintptr(mgr), uintptr(unsafe.Pointer(&cstr[0])), uintptr(style), 0, 0, uintptr(ch))
	return TypeFace(r1)
}

func FontMgrCountFamilies(mgr FontMgr) int {
	r1, _, _ := skFontMgrCountFamiliesProc.Call(uintptr(mgr))
	return int(r1)
}

func FontMgrGetFamilyName(mgr FontMgr, index int, str String) {
	skFontMgrGetFamilyNameProc.Call(uintptr(mgr), uintptr(index), uintptr(str))
}

func FontStyleNew(weight FontWeight, spacing FontSpacing, slant FontSlant) FontStyle {
	r1, _, _ := skFontStyleNewProc.Call(uintptr(weight), uintptr(spacing), uintptr(slant))
	return FontStyle(r1)
}

func FontStyleGetWeight(style FontStyle) FontWeight {
	r1, _, _ := skFontStyleGetWeightProc.Call(uintptr(style))
	return FontWeight(r1)
}

func FontStyleGetWidth(style FontStyle) FontSpacing {
	r1, _, _ := skFontStyleGetWidthProc.Call(uintptr(style))
	return FontSpacing(r1)
}

func FontStyleGetSlant(style FontStyle) FontSlant {
	r1, _, _ := skFontStyleGetSlantProc.Call(uintptr(style))
	return FontSlant(r1)
}

func FontStyleDelete(style FontStyle) {
	skFontStyleDeleteProc.Call(uintptr(style))
}

func FontStyleSetGetCount(set FontStyleSet) int {
	r1, _, _ := skFontStyleSetGetCountProc.Call(uintptr(set))
	return int(r1)
}

func FontStyleSetGetStyle(set FontStyleSet, index int, style FontStyle, str String) {
	skFontStyleSetGetStyleProc.Call(uintptr(set), uintptr(index), uintptr(style), uintptr(str))
}

func FontStyleSetCreateTypeFace(set FontStyleSet, index int) TypeFace {
	r1, _, _ := skFontStyleSetCreateTypeFaceProc.Call(uintptr(set), uintptr(index))
	return TypeFace(r1)
}

func FontStyleSetMatchStyle(set FontStyleSet, style FontStyle) TypeFace {
	r1, _, _ := skFontStyleSetMatchStyleProc.Call(uintptr(set), uintptr(style))
	return TypeFace(r1)
}

func FontStyleSetUnref(set FontStyleSet) {
	skFontStyleSetUnrefProc.Call(uintptr(set))
}

func ImageNewFromEncoded(data Data) Image {
	r1, _, _ := skImageNewFromEncodedProc.Call(uintptr(data))
	return Image(r1)
}

func ImageNewRasterData(info *ImageInfo, data Data, rowBytes int) Image {
	r1, _, _ := skImageNewRasterDataProc.Call(uintptr(unsafe.Pointer(info)), uintptr(data), uintptr(rowBytes))
	return Image(r1)
}

func ImageGetWidth(img Image) int {
	r1, _, _ := skImageGetWidthProc.Call(uintptr(img))
	return int(r1)
}

func ImageGetHeight(img Image) int {
	r1, _, _ := skImageGetHeightProc.Call(uintptr(img))
	return int(r1)
}

func ImageGetColorSpace(img Image) ColorSpace {
	r1, _, _ := skImageGetColorSpaceProc.Call(uintptr(img))
	return ColorSpace(r1)
}

func ImageGetColorType(img Image) ColorType {
	r1, _, _ := skImageGetColorTypeProc.Call(uintptr(img))
	return ColorType(r1)
}

func ImageGetAlphaType(img Image) AlphaType {
	r1, _, _ := skImageGetAlphaTypeProc.Call(uintptr(img))
	return AlphaType(r1)
}

func ImageReadPixels(img Image, info *ImageInfo, pixels []byte, dstRowBytes, srcX, srcY int, cachingHint ImageCachingHint) bool {
	r1, _, _ := skImageReadPixelsProc.Call(uintptr(img), uintptr(unsafe.Pointer(info)),
		uintptr(unsafe.Pointer(&pixels[0])), uintptr(dstRowBytes), uintptr(srcX), uintptr(srcY), uintptr(cachingHint))
	return r1&0xff != 0
}

func ImageMakeNonTextureImage(img Image) Image {
	r1, _, _ := skImageMakeNonTextureImageProc.Call(uintptr(img))
	return Image(r1)
}

func ImageMakeShader(img Image, tileModeX, tileModeY TileMode, sampling SamplingOptions, matrix geom.Matrix) Shader {
	r1, _, _ := skImageMakeShaderProc.Call(uintptr(img), uintptr(tileModeX), uintptr(tileModeY),
		uintptr(sampling), fromGeomMatrix(&matrix))
	return Shader(r1)
}

func ImageTextureFromImage(ctx DirectContext, img Image, mipMapped, budgeted bool) Image {
	r1, _, _ := skImageTextureFromImageProc.Call(uintptr(ctx), uintptr(img), boolToUintptr(mipMapped), boolToUintptr(budgeted))
	return Image(r1)
}

func ImageUnref(img Image) {
	skImageUnrefProc.Call(uintptr(img))
}

func ImageFilterNewArithmetic(k1, k2, k3, k4 float32, enforcePMColor bool, background, foreground ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewArithmeticProc.Call(uintptr(math.Float32bits(k1)), uintptr(math.Float32bits(k2)),
		uintptr(math.Float32bits(k3)), uintptr(math.Float32bits(k4)), boolToUintptr(enforcePMColor),
		uintptr(background), uintptr(foreground), fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewBlur(sigmaX, sigmaY float32, tileMode TileMode, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewBlurProc.Call(uintptr(math.Float32bits(sigmaX)), uintptr(math.Float32bits(sigmaY)),
		uintptr(tileMode), uintptr(input), fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewColorFilter(colorFilter ColorFilter, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewColorFilterProc.Call(uintptr(colorFilter), uintptr(input),
		fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewCompose(outer, inner ImageFilter) ImageFilter {
	r1, _, _ := skImageFilterNewComposeProc.Call(uintptr(outer), uintptr(inner))
	return ImageFilter(r1)
}

func ImageFilterNewDisplacementMapEffect(xChannelSelector, yChannelSelector ColorChannel, scale float32, displacement, color ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewDisplacementMapEffectProc.Call(uintptr(xChannelSelector), uintptr(yChannelSelector),
		uintptr(math.Float32bits(scale)), uintptr(displacement), uintptr(color), fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewDropShadow(dx, dy, sigmaX, sigmaY float32, color Color, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewDropShadowProc.Call(uintptr(math.Float32bits(dx)), uintptr(math.Float32bits(dy)),
		uintptr(math.Float32bits(sigmaX)), uintptr(math.Float32bits(sigmaY)), uintptr(color), uintptr(input),
		fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewDropShadowOnly(dx, dy, sigmaX, sigmaY float32, color Color, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewDropShadowOnlyProc.Call(uintptr(math.Float32bits(dx)), uintptr(math.Float32bits(dy)),
		uintptr(math.Float32bits(sigmaX)), uintptr(math.Float32bits(sigmaY)), uintptr(color), uintptr(input),
		fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewImageSource(img Image, srcRect, dstRect geom.Rect, sampling SamplingOptions) ImageFilter {
	r1, _, _ := skImageFilterNewImageSourceProc.Call(uintptr(img), fromGeomRect(&srcRect), fromGeomRect(&dstRect),
		uintptr(sampling))
	return ImageFilter(r1)
}

func ImageFilterNewImageSourceDefault(img Image, sampling SamplingOptions) ImageFilter {
	r1, _, _ := skImageFilterNewImageSourceDefaultProc.Call(uintptr(img), uintptr(sampling))
	return ImageFilter(r1)
}

func ImageFilterNewMagnifier(lensBounds geom.Rect, zoomAmount, inset float32, sampling SamplingOptions, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewMagnifierProc.Call(fromGeomRect(&lensBounds), uintptr(math.Float32bits(zoomAmount)),
		uintptr(math.Float32bits(inset)), uintptr(sampling), uintptr(input), fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewMatrixConvolution(size *ISize, kernel []float32, gain, bias float32, offset *IPoint, tileMode TileMode, convolveAlpha bool, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewMatrixConvolutionProc.Call(uintptr(unsafe.Pointer(size)),
		uintptr(unsafe.Pointer(&kernel[0])), uintptr(math.Float32bits(gain)), uintptr(math.Float32bits(bias)),
		uintptr(unsafe.Pointer(offset)), uintptr(tileMode), boolToUintptr(convolveAlpha), uintptr(input),
		fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewMatrixTransform(matrix geom.Matrix, sampling SamplingOptions, input ImageFilter) ImageFilter {
	r1, _, _ := skImageFilterNewMatrixTransformProc.Call(fromGeomMatrix(&matrix), uintptr(sampling), uintptr(input))
	return ImageFilter(r1)
}

func ImageFilterNewMerge(filters []ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewMergeProc.Call(uintptr(unsafe.Pointer(&filters[0])), uintptr(len(filters)),
		fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewOffset(dx, dy float32, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewOffsetProc.Call(uintptr(math.Float32bits(dx)), uintptr(math.Float32bits(dy)),
		uintptr(input), fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewTile(src, dst geom.Rect, input ImageFilter) ImageFilter {
	r1, _, _ := skImageFilterNewTileProc.Call(fromGeomRect(&src), fromGeomRect(&dst), uintptr(input))
	return ImageFilter(r1)
}

func ImageFilterNewDilate(radiusX, radiusY int, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewDilateProc.Call(uintptr(radiusX), uintptr(radiusY), uintptr(input),
		fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewErode(radiusX, radiusY int, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewErodeProc.Call(uintptr(radiusX), uintptr(radiusY), uintptr(input),
		fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewDistantLitDiffuse(pt *Point3, color Color, scale, reflectivity float32, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewDistantLitDiffuseProc.Call(uintptr(unsafe.Pointer(pt)), uintptr(color),
		uintptr(math.Float32bits(scale)), uintptr(math.Float32bits(reflectivity)), uintptr(input),
		fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewPointLitDiffuse(pt *Point3, color Color, scale, reflectivity float32, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewPointLitDiffuseProc.Call(uintptr(unsafe.Pointer(pt)), uintptr(color),
		uintptr(math.Float32bits(scale)), uintptr(math.Float32bits(reflectivity)), uintptr(input),
		fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewSpotLitDiffuse(pt, targetPt *Point3, specularExponent, cutoffAngle, scale, reflectivity float32, color Color, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewSpotLitDiffuseProc.Call(uintptr(unsafe.Pointer(pt)),
		uintptr(unsafe.Pointer(targetPt)), uintptr(math.Float32bits(specularExponent)),
		uintptr(math.Float32bits(cutoffAngle)), uintptr(color), uintptr(math.Float32bits(scale)),
		uintptr(math.Float32bits(reflectivity)), uintptr(input), fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewDistantLitSpecular(pt *Point3, color Color, scale, reflectivity, shine float32, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewDistantLitSpecularProc.Call(uintptr(unsafe.Pointer(pt)), uintptr(color),
		uintptr(math.Float32bits(scale)), uintptr(math.Float32bits(reflectivity)), uintptr(math.Float32bits(shine)),
		uintptr(input), fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewPointLitSpecular(pt *Point3, color Color, scale, reflectivity, shine float32, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewPointLitSpecularProc.Call(uintptr(unsafe.Pointer(pt)), uintptr(color),
		uintptr(math.Float32bits(scale)), uintptr(math.Float32bits(reflectivity)), uintptr(math.Float32bits(shine)),
		uintptr(input), fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterNewSpotLitSpecular(pt, targetPt *Point3, specularExponent, cutoffAngle, scale, reflectivity, shine float32, color Color, input ImageFilter, cropRect *geom.Rect) ImageFilter {
	r1, _, _ := skImageFilterNewSpotLitSpecularProc.Call(uintptr(unsafe.Pointer(pt)),
		uintptr(unsafe.Pointer(targetPt)), uintptr(math.Float32bits(specularExponent)),
		uintptr(math.Float32bits(cutoffAngle)), uintptr(color), uintptr(math.Float32bits(scale)),
		uintptr(math.Float32bits(reflectivity)), uintptr(math.Float32bits(shine)), uintptr(input),
		fromGeomRect(cropRect))
	return ImageFilter(r1)
}

func ImageFilterUnref(filter ImageFilter) {
	skImageFilterUnrefProc.Call(uintptr(filter))
}

func MaskFilterNewBlurWithFlags(style Blur, sigma float32, respectMatrix bool) MaskFilter {
	r1, _, _ := skMaskFilterNewBlurWithFlagsProc.Call(uintptr(style), uintptr(math.Float32bits(sigma)),
		boolToUintptr(respectMatrix))
	return MaskFilter(r1)
}

func MaskFilterNewTable(table []byte) MaskFilter {
	r1, _, _ := skMaskFilterNewTableProc.Call(uintptr(unsafe.Pointer(&table[0])))
	return MaskFilter(r1)
}

func MaskFilterNewGamma(gamma float32) MaskFilter {
	r1, _, _ := skMaskFilterNewGammaProc.Call(uintptr(math.Float32bits(gamma)))
	return MaskFilter(r1)
}

func MaskFilterNewClip(minimum, maximum byte) MaskFilter {
	r1, _, _ := skMaskFilterNewClipProc.Call(uintptr(minimum), uintptr(maximum))
	return MaskFilter(r1)
}

func MaskFilterNewShader(shader Shader) MaskFilter {
	r1, _, _ := skMaskFilterNewShaderProc.Call(uintptr(shader))
	return MaskFilter(r1)
}

func MaskFilterUnref(filter MaskFilter) {
	skMaskFilterUnrefProc.Call(uintptr(filter))
}

func OpBuilderNew() OpBuilder {
	r1, _, _ := skOpBuilderNewProc.Call()
	return OpBuilder(r1)
}

func OpBuilderAdd(builder OpBuilder, path Path, op PathOp) {
	skOpBuilderAddProc.Call(uintptr(builder), uintptr(path), uintptr(op))
}

func OpBuilderResolve(builder OpBuilder, path Path) bool {
	r1, _, _ := skOpBuilderResolveProc.Call(uintptr(builder), uintptr(path))
	return r1&0xff != 0
}

func OpBuilderDestroy(builder OpBuilder) {
	skOpBuilderDestroyProc.Call(uintptr(builder))
}

func PaintNew() Paint {
	r1, _, _ := skPaintNewProc.Call()
	return Paint(r1)
}

func PaintEquivalent(left, right Paint) bool {
	r1, _, _ := skPaintEquivalentProc.Call(uintptr(left), uintptr(right))
	return r1&0xff != 0
}

func PaintDelete(paint Paint) {
	skPaintDeleteProc.Call(uintptr(paint))
}

func PaintClone(paint Paint) Paint {
	r1, _, _ := skPaintCloneProc.Call(uintptr(paint))
	return Paint(r1)
}

func PaintReset(paint Paint) {
	skPaintResetProc.Call(uintptr(paint))
}

func PaintIsAntialias(paint Paint) bool {
	r1, _, _ := skPaintIsAntialiasProc.Call(uintptr(paint))
	return r1&0xff != 0
}

func PaintSetAntialias(paint Paint, enabled bool) {
	skPaintSetAntialiasProc.Call(uintptr(paint), boolToUintptr(enabled))
}

func PaintIsDither(paint Paint) bool {
	r1, _, _ := skPaintIsDitherProc.Call(uintptr(paint))
	return r1&0xff != 0
}

func PaintSetDither(paint Paint, enabled bool) {
	skPaintSetDitherProc.Call(uintptr(paint), boolToUintptr(enabled))
}

func PaintGetColor(paint Paint) Color {
	r1, _, _ := skPaintGetColorProc.Call(uintptr(paint))
	return Color(r1)
}

func PaintSetColor(paint Paint, color Color) {
	skPaintSetColorProc.Call(uintptr(paint), uintptr(color))
}

func PaintGetStyle(paint Paint) PaintStyle {
	r1, _, _ := skPaintGetStyleProc.Call(uintptr(paint))
	return PaintStyle(r1)
}

func PaintSetStyle(paint Paint, style PaintStyle) {
	skPaintSetStyleProc.Call(uintptr(paint), uintptr(style))
}

func PaintGetStrokeWidth(paint Paint) float32 {
	_, r2, _ := skPaintGetStrokeWidthProc.Call(uintptr(paint))
	return math.Float32frombits(uint32(r2))
}

func PaintSetStrokeWidth(paint Paint, width float32) {
	skPaintSetStrokeWidthProc.Call(uintptr(paint), uintptr(math.Float32bits(width)))
}

func PaintGetStrokeMiter(paint Paint) float32 {
	_, r2, _ := skPaintGetStrokeMiterProc.Call(uintptr(paint))
	return math.Float32frombits(uint32(r2))
}

func PaintSetStrokeMiter(paint Paint, miter float32) {
	skPaintSetStrokeMiterProc.Call(uintptr(paint), uintptr(math.Float32bits(miter)))
}

func PaintGetStrokeCap(paint Paint) StrokeCap {
	r1, _, _ := skPaintGetStrokeCapProc.Call(uintptr(paint))
	return StrokeCap(r1)
}

func PaintSetStrokeCap(paint Paint, strokeCap StrokeCap) {
	skPaintSetStrokeCapProc.Call(uintptr(paint), uintptr(strokeCap))
}

func PaintGetStrokeJoin(paint Paint) StrokeJoin {
	r1, _, _ := skPaintGetStrokeJoinProc.Call(uintptr(paint))
	return StrokeJoin(r1)
}

func PaintSetStrokeJoin(paint Paint, strokeJoin StrokeJoin) {
	skPaintSetStrokeJoinProc.Call(uintptr(paint), uintptr(strokeJoin))
}

func PaintGetBlendMode(paint Paint) BlendMode {
	r1, _, _ := skPaintGetBlendModeProc.Call(uintptr(paint), uintptr(3)) // SrcOverBlendMode
	return BlendMode(r1)
}

func PaintSetBlendMode(paint Paint, blendMode BlendMode) {
	skPaintSetBlendModeProc.Call(uintptr(paint), uintptr(blendMode))
}

func PaintGetShader(paint Paint) Shader {
	r1, _, _ := skPaintGetShaderProc.Call(uintptr(paint))
	return Shader(r1)
}

func PaintSetShader(paint Paint, shader Shader) {
	skPaintSetShaderProc.Call(uintptr(paint), uintptr(shader))
}

func PaintGetColorFilter(paint Paint) ColorFilter {
	r1, _, _ := skPaintGetColorFilterProc.Call(uintptr(paint))
	return ColorFilter(r1)
}

func PaintSetColorFilter(paint Paint, filter ColorFilter) {
	skPaintSetColorFilterProc.Call(uintptr(paint), uintptr(filter))
}

func PaintGetMaskFilter(paint Paint) MaskFilter {
	r1, _, _ := skPaintGetMaskFilterProc.Call(uintptr(paint))
	return MaskFilter(r1)
}

func PaintSetMaskFilter(paint Paint, filter MaskFilter) {
	skPaintSetMaskFilterProc.Call(uintptr(paint), uintptr(filter))
}

func PaintGetImageFilter(paint Paint) ImageFilter {
	r1, _, _ := skPaintGetImageFilterProc.Call(uintptr(paint))
	return ImageFilter(r1)
}

func PaintSetImageFilter(paint Paint, filter ImageFilter) {
	skPaintSetImageFilterProc.Call(uintptr(paint), uintptr(filter))
}

func PaintGetPathEffect(paint Paint) PathEffect {
	r1, _, _ := skPaintGetPathEffectProc.Call(uintptr(paint))
	return PathEffect(r1)
}

func PaintSetPathEffect(paint Paint, effect PathEffect) {
	skPaintSetPathEffectProc.Call(uintptr(paint), uintptr(effect))
}

func PaintGetFillPath(paint Paint, inPath, outPath Path, cullRect *geom.Rect, resScale float32) bool {
	r1, _, _ := skPaintGetFillPathProc.Call(uintptr(paint), uintptr(inPath), uintptr(outPath), fromGeomRect(cullRect),
		uintptr(math.Float32bits(resScale)))
	return r1&0xff != 0
}

func PathNew() Path {
	r1, _, _ := skPathNewProc.Call()
	return Path(r1)
}

func PathIsEmpty(path Path) bool {
	r1, _, _ := skPathIsEmptyProc.Call(uintptr(path))
	return r1&0xff != 0
}

func PathParseSVGString(path Path, svg string) bool {
	buffer := make([]byte, len(svg)+1)
	copy(buffer, svg)
	r1, _, _ := skPathParseSVGStringProc.Call(uintptr(path), uintptr(unsafe.Pointer(&buffer[0])))
	return r1&0xff != 0
}

func PathToSVGString(path Path, absolute bool) String {
	r1, _, _ := skPathToSVGStringProc.Call(uintptr(path), boolToUintptr(absolute))
	return String(r1)
}

func PathGetFillType(path Path) FillType {
	r1, _, _ := skPathGetFillTypeProc.Call(uintptr(path))
	return FillType(r1)
}

func PathSetFillType(path Path, fillType FillType) {
	skPathSetFillTypeProc.Call(uintptr(path), uintptr(fillType))
}

func PathArcTo(path Path, x, y, rx, ry, rotation float32, arcSize ArcSize, direction Direction) {
	skPathArcToProc.Call(uintptr(path), uintptr(math.Float32bits(rx)), uintptr(math.Float32bits(ry)),
		uintptr(math.Float32bits(rotation)), uintptr(arcSize), uintptr(direction), uintptr(math.Float32bits(x)),
		uintptr(math.Float32bits(y)))
}

func PathArcToWithPoints(path Path, x1, y1, x2, y2, radius float32) {
	skPathArcToWithPointsProc.Call(uintptr(path), uintptr(math.Float32bits(x1)), uintptr(math.Float32bits(y1)),
		uintptr(math.Float32bits(x2)), uintptr(math.Float32bits(y2)), uintptr(math.Float32bits(radius)))
}

func PathArcToWithOval(path Path, rect geom.Rect, startAngle, sweepAngle float32, forceMoveTo bool) {
	skPathArcToWithOvalProc.Call(uintptr(path), fromGeomRect(&rect), uintptr(math.Float32bits(startAngle)),
		uintptr(math.Float32bits(sweepAngle)), boolToUintptr(forceMoveTo))
}

func PathRArcTo(path Path, dx, dy, rx, ry, rotation float32, arcSize ArcSize, direction Direction) {
	skPathRArcToProc.Call(uintptr(path), uintptr(math.Float32bits(rx)), uintptr(math.Float32bits(ry)),
		uintptr(math.Float32bits(rotation)), uintptr(arcSize), uintptr(direction), uintptr(math.Float32bits(dx)),
		uintptr(math.Float32bits(dy)))
}

func PathGetBounds(path Path) geom.Rect {
	var r Rect
	skPathGetBoundsProc.Call(uintptr(path), uintptr(unsafe.Pointer(&r)))
	return toGeomRect(r)
}

func PathComputeTightBounds(path Path) geom.Rect {
	var r Rect
	skPathComputeTightBoundsProc.Call(uintptr(path), uintptr(unsafe.Pointer(&r)))
	return toGeomRect(r)
}

func PathAddCircle(path Path, center geom.Point, radius float32, direction Direction) {
	skPathAddCircleProc.Call(uintptr(path), uintptr(math.Float32bits(center.X)), uintptr(math.Float32bits(center.Y)),
		uintptr(math.Float32bits(radius)), uintptr(direction))
}

func PathClone(path Path) Path {
	r1, _, _ := skPathCloneProc.Call(uintptr(path))
	return Path(r1)
}

func PathClose(path Path) {
	skPathCloseProc.Call(uintptr(path))
}

func PathConicTo(path Path, ctrlPt, endPt geom.Point, weight float32) {
	skPathConicToProc.Call(uintptr(path), uintptr(math.Float32bits(ctrlPt.X)), uintptr(math.Float32bits(ctrlPt.Y)),
		uintptr(math.Float32bits(endPt.X)), uintptr(math.Float32bits(endPt.Y)), uintptr(math.Float32bits(weight)))
}

func PathRConicTo(path Path, ctrlPt, endPt geom.Point, weight float32) {
	skPathRConicToProc.Call(uintptr(path), uintptr(math.Float32bits(ctrlPt.X)), uintptr(math.Float32bits(ctrlPt.Y)),
		uintptr(math.Float32bits(endPt.X)), uintptr(math.Float32bits(endPt.Y)), uintptr(math.Float32bits(weight)))
}

func PathCubicTo(path Path, cp1, cp2, endPt geom.Point) {
	skPathCubicToProc.Call(uintptr(path), uintptr(math.Float32bits(cp1.X)), uintptr(math.Float32bits(cp1.Y)),
		uintptr(math.Float32bits(cp2.X)), uintptr(math.Float32bits(cp2.Y)), uintptr(math.Float32bits(endPt.X)),
		uintptr(math.Float32bits(endPt.Y)))
}

func PathRCubicTo(path Path, cp1, cp2, endPt geom.Point) {
	skPathRCubicToProc.Call(uintptr(path), uintptr(math.Float32bits(cp1.X)), uintptr(math.Float32bits(cp1.Y)),
		uintptr(math.Float32bits(cp2.X)), uintptr(math.Float32bits(cp2.Y)), uintptr(math.Float32bits(endPt.X)),
		uintptr(math.Float32bits(endPt.Y)))
}

func PathLineTo(path Path, pt geom.Point) {
	skPathLineToProc.Call(uintptr(path), uintptr(math.Float32bits(pt.X)), uintptr(math.Float32bits(pt.Y)))
}

func PathRLineTo(path Path, pt geom.Point) {
	skPathRLineToProc.Call(uintptr(path), uintptr(math.Float32bits(pt.X)), uintptr(math.Float32bits(pt.Y)))
}

func PathMoveTo(path Path, pt geom.Point) {
	skPathMoveToProc.Call(uintptr(path), uintptr(math.Float32bits(pt.X)), uintptr(math.Float32bits(pt.Y)))
}

func PathRMoveTo(path Path, pt geom.Point) {
	skPathRMoveToProc.Call(uintptr(path), uintptr(math.Float32bits(pt.X)), uintptr(math.Float32bits(pt.Y)))
}

func PathAddOval(path Path, rect geom.Rect, direction Direction) {
	skPathAddOvalProc.Call(uintptr(path), fromGeomRect(&rect), uintptr(direction))
}

func PathAddPath(path, other Path, mode PathAddMode) {
	skPathAddPathProc.Call(uintptr(path), uintptr(other), uintptr(mode))
}

func PathAddPathReverse(path, other Path) {
	skPathAddPathReverseProc.Call(uintptr(path), uintptr(other))
}

func PathAddPathMatrix(path, other Path, matrix geom.Matrix, mode PathAddMode) {
	skPathAddPathMatrixProc.Call(uintptr(path), uintptr(other), fromGeomMatrix(&matrix), uintptr(mode))
}

func PathAddPathOffset(path, other Path, offset geom.Point, mode PathAddMode) {
	skPathAddPathOffsetProc.Call(uintptr(path), uintptr(other), uintptr(math.Float32bits(offset.X)),
		uintptr(math.Float32bits(offset.Y)), uintptr(mode))
}

func PathAddPoly(path Path, pts []geom.Point, closePath bool) {
	skPathAddPolyProc.Call(uintptr(path), uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)), boolToUintptr(closePath))
}

func PathQuadTo(path Path, ctrlPt, endPt geom.Point) {
	skPathQuadToProc.Call(uintptr(path), uintptr(math.Float32bits(ctrlPt.X)), uintptr(math.Float32bits(ctrlPt.Y)),
		uintptr(math.Float32bits(endPt.X)), uintptr(math.Float32bits(endPt.Y)))
}

func PathAddRect(path Path, rect geom.Rect, direction Direction) {
	skPathAddRectProc.Call(uintptr(path), fromGeomRect(&rect), uintptr(direction))
}

func PathAddRoundedRect(path Path, rect geom.Rect, radius geom.Size, direction Direction) {
	skPathAddRoundedRectProc.Call(uintptr(path), fromGeomRect(&rect), uintptr(math.Float32bits(radius.Width)),
		uintptr(math.Float32bits(radius.Height)), uintptr(direction))
}

func PathTransform(path Path, matrix geom.Matrix) {
	skPathTransformProc.Call(uintptr(path), fromGeomMatrix(&matrix))
}

func PathTransformToDest(path, dstPath Path, matrix geom.Matrix) {
	skPathTransformToDestProc.Call(uintptr(path), fromGeomMatrix(&matrix), uintptr(dstPath))
}

func PathReset(path Path) {
	skPathResetProc.Call(uintptr(path))
}

func PathRewind(path Path) {
	skPathRewindProc.Call(uintptr(path))
}

func PathContains(path Path, pt geom.Point) bool {
	r1, _, _ := skPathContainsProc.Call(uintptr(path), uintptr(math.Float32bits(pt.X)), uintptr(math.Float32bits(pt.Y)))
	return r1&0xff != 0
}

func PathGetLastPoint(path Path) geom.Point {
	var pt geom.Point
	skPathGetLastPointProc.Call(uintptr(path), uintptr(unsafe.Pointer(&pt)))
	return pt
}

func PathDelete(path Path) {
	skPathDeleteProc.Call(uintptr(path))
}

func PathCompute(path, other Path, op PathOp) bool {
	r1, _, _ := skPathOpProc.Call(uintptr(path), uintptr(other), uintptr(op), uintptr(path))
	return r1&0xff != 0
}

func PathSimplify(path Path) bool {
	r1, _, _ := skPathSimplifyProc.Call(uintptr(path), uintptr(path))
	return r1&0xff != 0
}

func PathEffectCreateCompose(outer, inner PathEffect) PathEffect {
	r1, _, _ := skPathEffectCreateComposeProc.Call(uintptr(outer), uintptr(inner))
	return PathEffect(r1)
}

func PathEffectCreateSum(first, second PathEffect) PathEffect {
	r1, _, _ := skPathEffectCreateSumProc.Call(uintptr(first), uintptr(second))
	return PathEffect(r1)
}

func PathEffectCreateDiscrete(segLength, deviation float32, seedAssist uint32) PathEffect {
	r1, _, _ := skPathEffectCreateDiscreteProc.Call(uintptr(math.Float32bits(segLength)),
		uintptr(math.Float32bits(deviation)), uintptr(seedAssist))
	return PathEffect(r1)
}

func PathEffectCreateCorner(radius float32) PathEffect {
	r1, _, _ := skPathEffectCreateCornerProc.Call(uintptr(math.Float32bits(radius)))
	return PathEffect(r1)
}

func PathEffectCreate1dPath(path Path, advance, phase float32, style PathEffect1DStyle) PathEffect {
	r1, _, _ := skPathEffectCreate1dPathProc.Call(uintptr(path), uintptr(math.Float32bits(advance)),
		uintptr(math.Float32bits(phase)), uintptr(style))
	return PathEffect(r1)
}

func PathEffectCreate2dLine(width float32, matrix geom.Matrix) PathEffect {
	r1, _, _ := skPathEffectCreate2dLineProc.Call(uintptr(math.Float32bits(width)), fromGeomMatrix(&matrix))
	return PathEffect(r1)
}

func PathEffectCreate2dPath(matrix geom.Matrix, path Path) PathEffect {
	r1, _, _ := skPathEffectCreate2dPathProc.Call(fromGeomMatrix(&matrix), uintptr(path))
	return PathEffect(r1)
}

func PathEffectCreateDash(intervals []float32, phase float32) PathEffect {
	r1, _, _ := skPathEffectCreateDashProc.Call(uintptr(unsafe.Pointer(&intervals[0])), uintptr(len(intervals)),
		uintptr(math.Float32bits(phase)))
	return PathEffect(r1)
}

func PathEffectCreateTrim(start, stop float32, mode TrimMode) PathEffect {
	r1, _, _ := skPathEffectCreateTrimProc.Call(uintptr(math.Float32bits(start)), uintptr(math.Float32bits(stop)),
		uintptr(mode))
	return PathEffect(r1)
}

func PathEffectUnref(effect PathEffect) {
	skPathEffectUnrefProc.Call(uintptr(effect))
}

func RegisterImageCodecs() {
	skRegisterImageCodecsProc.Call()
}

func ShaderNewColor(color Color) Shader {
	r1, _, _ := skShaderNewColorProc.Call(uintptr(color))
	return Shader(r1)
}

func ShaderNewBlend(blendMode BlendMode, dst, src Shader) Shader {
	r1, _, _ := skShaderNewBlendProc.Call(uintptr(blendMode), uintptr(dst), uintptr(src))
	return Shader(r1)
}

func ShaderNewLinearGradient(start, end geom.Point, colors []Color, colorPos []float32, tileMode TileMode, matrix geom.Matrix) Shader {
	pts := make([]geom.Point, 2)
	pts[0] = start
	pts[1] = end
	r1, _, _ := skShaderNewLinearGradientProc.Call(uintptr(unsafe.Pointer(&pts[0])),
		uintptr(unsafe.Pointer(&colors[0])), uintptr(unsafe.Pointer(&colorPos[0])), uintptr(len(colors)),
		uintptr(tileMode), fromGeomMatrix(&matrix))
	return Shader(r1)
}

func ShaderNewRadialGradient(center geom.Point, radius float32, colors []Color, colorPos []float32, tileMode TileMode, matrix geom.Matrix) Shader {
	r1, _, _ := skShaderNewRadialGradientProc.Call(uintptr(unsafe.Pointer(&center)), uintptr(math.Float32bits(radius)),
		uintptr(unsafe.Pointer(&colors[0])), uintptr(unsafe.Pointer(&colorPos[0])), uintptr(len(colors)),
		uintptr(tileMode), fromGeomMatrix(&matrix))
	return Shader(r1)
}

func ShaderNewSweepGradient(center geom.Point, startAngle, endAngle float32, colors []Color, colorPos []float32, tileMode TileMode, matrix geom.Matrix) Shader {
	r1, _, _ := skShaderNewSweepGradientProc.Call(uintptr(unsafe.Pointer(&center)), uintptr(unsafe.Pointer(&colors[0])),
		uintptr(unsafe.Pointer(&colorPos[0])), uintptr(len(colors)), uintptr(tileMode),
		uintptr(math.Float32bits(startAngle)), uintptr(math.Float32bits(endAngle)), fromGeomMatrix(&matrix))
	return Shader(r1)
}

func ShaderNewTwoPointConicalGradient(startPt, endPt geom.Point, startRadius, endRadius float32, colors []Color, colorPos []float32, tileMode TileMode, matrix geom.Matrix) Shader {
	r1, _, _ := skShaderNewTwoPointConicalGradientProc.Call(uintptr(unsafe.Pointer(&startPt)),
		uintptr(math.Float32bits(startRadius)), uintptr(unsafe.Pointer(&endPt)), uintptr(math.Float32bits(endRadius)),
		uintptr(unsafe.Pointer(&colors[0])), uintptr(unsafe.Pointer(&colorPos[0])), uintptr(len(colors)),
		uintptr(tileMode), fromGeomMatrix(&matrix))
	return Shader(r1)
}

func ShaderNewPerlinNoiseFractalNoise(baseFreqX, baseFreqY, seed float32, numOctaves int, size ISize) Shader {
	r1, _, _ := skShaderNewPerlinNoiseFractalNoiseProc.Call(uintptr(baseFreqX), uintptr(baseFreqY), uintptr(numOctaves),
		uintptr(math.Float32bits(seed)), uintptr(unsafe.Pointer(&size)))
	return Shader(r1)
}

func ShaderNewPerlinNoiseTurbulence(baseFreqX, baseFreqY, seed float32, numOctaves int, size ISize) Shader {
	r1, _, _ := skShaderNewPerlinNoiseTurbulenceProc.Call(uintptr(math.Float32bits(baseFreqX)),
		uintptr(math.Float32bits(baseFreqY)), uintptr(numOctaves), uintptr(math.Float32bits(seed)),
		uintptr(unsafe.Pointer(&size)))
	return Shader(r1)
}

func ShaderWithLocalMatrix(shader Shader, matrix geom.Matrix) Shader {
	r1, _, _ := skShaderWithLocalMatrixProc.Call(uintptr(shader), fromGeomMatrix(&matrix))
	return Shader(r1)
}

func ShaderWithColorFilter(shader Shader, filter ColorFilter) Shader {
	r1, _, _ := skShaderWithColorFilterProc.Call(uintptr(shader), uintptr(filter))
	return Shader(r1)
}

func ShaderUnref(shader Shader) {
	skShaderUnrefProc.Call(uintptr(shader))
}

func StringNew(s string) String {
	b := []byte(s)
	r1, _, _ := skStringNewProc.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return String(r1)
}

func StringNewEmpty() String {
	r1, _, _ := skStringNewEmptyProc.Call()
	return String(r1)
}

func StringGetString(str String) string {
	data, _, _ := skStringGetCStrProc.Call(uintptr(str))
	size, _, _ := skStringGetSizeProc.Call(uintptr(str))
	s := make([]byte, int(size))
	copy(s, unsafe.Slice((*byte)(unsafe.Pointer(data)), size))
	return string(s)
}

func StringDelete(str String) {
	skStringDeleteProc.Call(uintptr(str))
}

func SurfaceMakeRasterDirect(info *ImageInfo, pixels []byte, rowBytes int, surfaceProps SurfaceProps) Surface {
	r1, _, _ := skSurfaceMakeRasterDirectProc.Call(uintptr(unsafe.Pointer(info)), uintptr(unsafe.Pointer(&pixels[0])), uintptr(rowBytes), uintptr(surfaceProps))
	return Surface(r1)
}

func SurfaceMakeRasterN32PreMul(info *ImageInfo, surfaceProps SurfaceProps) Surface {
	r1, _, _ := skSurfaceMakeRasterN32PreMulProc.Call(uintptr(unsafe.Pointer(info)), uintptr(surfaceProps))
	return Surface(r1)
}

func SurfaceNewBackendRenderTarget(ctx DirectContext, backend BackendRenderTarget, origin SurfaceOrigin, colorType ColorType, colorSpace ColorSpace, surfaceProps SurfaceProps) Surface {
	r1, _, _ := skSurfaceNewBackendRenderTargetProc.Call(uintptr(ctx), uintptr(backend), uintptr(origin),
		uintptr(colorType), uintptr(colorSpace), uintptr(surfaceProps))
	return Surface(r1)
}

func SurfaceMakeImageSnapshot(aSurface Surface) Image {
	r1, _, _ := skSurfaceMakeImageSnapshotProc.Call(uintptr(aSurface))
	return Image(r1)
}

func SurfaceGetCanvas(aSurface Surface) Canvas {
	r1, _, _ := skSurfaceGetCanvasProc.Call(uintptr(aSurface))
	return Canvas(r1)
}

func SurfaceUnref(aSurface Surface) {
	skSurfaceUnrefProc.Call(uintptr(aSurface))
}

func SurfacePropsNew(geometry PixelGeometry) SurfaceProps {
	r1, _, _ := skSurfacePropsNewProc.Call(0, uintptr(geometry))
	return SurfaceProps(r1)
}

func TextBlobMakeFromText(text string, font Font) TextBlob {
	b := []byte(text)
	r1, _, _ := skTextBlobMakeFromTextProc.Call(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(font), uintptr(TextEncodingUTF8))
	return TextBlob(r1)
}

func TextBlobGetBounds(txt TextBlob) geom.Rect {
	var r Rect
	skTextBlobGetBoundsProc.Call(uintptr(txt), uintptr(unsafe.Pointer(&r)))
	return toGeomRect(r)
}

func TextBlobGetIntercepts(txt TextBlob, p Paint, start, end float32, intercepts []float32) int {
	pos := []float32{start, end}
	var dst *float32
	if len(intercepts) != 0 {
		dst = &intercepts[0]
	}
	r1, _, _ := skTextBlobGetInterceptsProc.Call(uintptr(txt), uintptr(unsafe.Pointer(&pos[0])), uintptr(unsafe.Pointer(dst)), uintptr(p))
	return int(r1)
}

func TextBlobUnref(txt TextBlob) {
	skTextBlobUnrefProc.Call(uintptr(txt))
}

func TextBlobBuilderNew() TextBlobBuilder {
	r1, _, _ := skTextBlobBuilderNewProc.Call()
	return TextBlobBuilder(r1)
}

func TextBlobBuilderMake(builder TextBlobBuilder) TextBlob {
	r1, _, _ := skTextBlobBuilderMakeProc.Call(uintptr(builder))
	return TextBlob(r1)
}

func TextBlobBuilderAllocRun(builder TextBlobBuilder, font Font, glyphs []uint16, pt geom.Point) {
	r1, _, _ := skTextBlobBuilderAllocRunProc.Call(uintptr(builder), uintptr(font), uintptr(len(glyphs)),
		uintptr(math.Float32bits(pt.X)), uintptr(math.Float32bits(pt.Y)), 0)
	buffer := (*textBlobBuilderRunBuffer)(unsafe.Pointer(r1))
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(buffer.Glyphs)), len(glyphs)), glyphs)
}

func TextBlobBuilderAllocRunPosH(builder TextBlobBuilder, font Font, glyphs []uint16, positions []float32, y float32) {
	r1, _, _ := skTextBlobBuilderAllocRunPosHProc.Call(uintptr(builder), uintptr(font), uintptr(len(glyphs)),
		uintptr(math.Float32bits(y)), 0)
	buffer := (*textBlobBuilderRunBuffer)(unsafe.Pointer(r1))
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(buffer.Glyphs)), len(glyphs)), glyphs)
	copy(unsafe.Slice((*float32)(unsafe.Pointer(buffer.Pos)), len(positions)), positions)
}

func TextBlobBuilderDelete(builder TextBlobBuilder) {
	skTextBlobBuilderDeleteProc.Call(uintptr(builder))
}

func TypeFaceGetFontStyle(face TypeFace) FontStyle {
	r1, _, _ := skTypeFaceGetFontStyleProc.Call(uintptr(face))
	return FontStyle(r1)
}

func TypeFaceIsFixedPitch(face TypeFace) bool {
	r1, _, _ := skTypeFaceIsFixedPitchProc.Call(uintptr(face))
	return r1&0xff != 0
}

func TypeFaceGetFamilyName(face TypeFace) String {
	r1, _, _ := skTypeFaceGetFamilyNameProc.Call(uintptr(face))
	return String(r1)
}

func TypeFaceGetUnitsPerEm(face TypeFace) int {
	r1, _, _ := skTypeFaceGetUnitsPerEmProc.Call(uintptr(face))
	return int(r1)
}

func TypeFaceUnref(face TypeFace) {
	skTypeFaceUnrefProc.Call(uintptr(face))
}

func boolToUintptr(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}
