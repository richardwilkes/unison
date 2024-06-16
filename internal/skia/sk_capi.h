#ifndef CSKIA_DEFINED
#define CSKIA_DEFINED

#include <stdint.h>
#include <stddef.h>
#ifndef __cplusplus
#include <stdbool.h>
#endif // __cplusplus

#if !defined(SK_C_API)
    #if defined(SKIA_C_DLL)
        #if defined(_MSC_VER)
            #if SKIA_IMPLEMENTATION
                #define SK_C_API __declspec(dllexport)
            #else
                #define SK_C_API __declspec(dllimport)
            #endif
        #else
            #define SK_C_API __attribute__((visibility("default")))
        #endif
    #else
        #define SK_C_API
    #endif
#endif

#ifdef __cplusplus
extern "C" {
#endif // __cplusplus

// ===== Types from include/core/SkString.h =====

typedef struct sk_string_t sk_string_t;

// ===== Types from include/core/SkTextBlob.h =====

typedef struct sk_text_blob_t sk_text_blob_t;

typedef struct sk_text_blob_builder_t sk_text_blob_builder_t;

typedef struct {
    uint16_t* glyphs;
    float*    pos;
    char*     utf8text;
    uint32_t* clusters;
} sk_text_blob_builder_run_buffer_t;

// ===== Types from include/core/SkData.h =====

typedef struct sk_data_t sk_data_t;

// ===== Types from include/core/SkPoint.h =====

typedef struct {
    int32_t x;
    int32_t y;
} sk_ipoint_t;

typedef struct {
    float x;
    float y;
} sk_point_t;

// ===== Types from include/core/SkPoint3.h ====

typedef struct {
    float x;
    float y;
    float z;
} sk_point3_t;

// ===== Types from include/core/SkSize.h =====

typedef struct {
    int32_t w;
    int32_t h;
} sk_isize_t;

// ===== Types from include/core/SkRect.h =====

typedef struct {
    int32_t left;
    int32_t top;
    int32_t right;
    int32_t bottom;
} sk_irect_t;

typedef struct {
    float left;
    float top;
    float right;
    float bottom;
} sk_rect_t;

// ===== Types from include/core/SkMatrix.h =====

typedef struct {
    float scaleX;
    float skewX;
    float transX;
    float skewY;
    float scaleY;
    float transY;
    float persp0;
    float persp1;
    float persp2;
} sk_matrix_t;

// ===== Types from include/core/SkPath.h =====

typedef enum {
    SK_PATH_ADD_MODE_APPEND, // appended to destination unaltered
    SK_PATH_ADD_MODE_EXTEND, // add line if prior contour is not closed
} sk_path_add_mode_t;

typedef enum {
    SK_PATH_ARC_SIZE_SMALL, // smaller of arc pair
    SK_PATH_ARC_SIZE_LARGE, // larger of arc pair
} sk_path_arc_size_t;

typedef struct sk_path_t sk_path_t;

// ===== Types from include/core/SkPathTypes.h =====

typedef enum {
    SK_PATH_DIRECTION_CW,  // clockwise direction for adding closed contours
    SK_PATH_DIRECTION_CCW, // counter-clockwise direction for adding closed contours
} sk_path_direction_t;

typedef enum {
    SK_PATH_FILLTYPE_WINDING,         // Specifies that "inside" is computed by a non-zero sum of signed edge crossings
    SK_PATH_FILLTYPE_EVENODD,         // Specifies that "inside" is computed by an odd number of edge crossings
    SK_PATH_FILLTYPE_INVERSE_WINDING, // Same as Winding, but draws outside of the path, rather than inside
    SK_PATH_FILLTYPE_INVERSE_EVENODD, // Same as EvenOdd, but draws outside of the path, rather than inside
} sk_path_fill_type_t;

// ===== Types from include/effects/Sk1DPathEffect.h =====

typedef enum {
    SK_PATH_EFFECT_1D_STYLE_TRANSLATE, // translate the shape to each position
    SK_PATH_EFFECT_1D_STYLE_ROTATE,    // rotate the shape about its center
    SK_PATH_EFFECT_1D_STYLE_MORPH,     // transform each point, and turn lines into curves
    SK_PATH_EFFECT_1D_STYLE_LAST = SK_PATH_EFFECT_1D_STYLE_MORPH,
} sk_path_effect_1d_style_t;

// ===== Types from include/effects/SkTrimPathEffect.h =====

typedef enum {
    SK_PATH_EFFECT_TRIM_MODE_NORMAL,   // return the subset path [start,stop]
    SK_PATH_EFFECT_TRIM_MODE_INVERTED, // return the complement/subset paths [0,start] + [stop,1]
} sk_path_effect_trim_mode_t;

// ===== Types from include/core/SkPathEffect.h =====

typedef struct sk_path_effect_t sk_path_effect_t;

// ===== Types from include/pathops/SkPathOps.h =====

typedef enum {
    SK_PATH_OP_DIFFERENCE,         // subtract the op path from the first path
    SK_PATH_OP_INTERSECT,          // intersect the two paths
    SK_PATH_OP_UNION,              // union (inclusive-or) the two paths
    SK_PATH_OP_XOR,                // exclusive-or the two paths
    SK_PATH_OP_REVERSE_DIFFERENCE, // subtract the first path from the op path
} sk_path_op_t;

typedef struct sk_op_builder_t sk_op_builder_t;

// ===== Types from include/core/SkSurfaceProps.h =====

typedef enum {
    SK_PIXEL_GEOMETRY_UNKNOWN,
    SK_PIXEL_GEOMETRY_RGB_H,
    SK_PIXEL_GEOMETRY_BGR_H,
    SK_PIXEL_GEOMETRY_RGB_V,
    SK_PIXEL_GEOMETRY_BGR_V,
} sk_pixel_geometry_t;

typedef struct sk_surface_props_t sk_surface_props_t;

// ===== Types from include/core/SkSurface.h =====

typedef struct sk_surface_t sk_surface_t;

// ===== Types from include/gpu/GrTypes.h =====

typedef enum {
    GR_SURFACE_ORIGIN_TOP_LEFT,
    GR_SURFACE_ORIGIN_BOTTOM_LEFT,
} gr_surface_origin_t;

// ===== Types from include/gpu/gl/GrGLTypes.h =====

typedef struct {
    unsigned int fFBOID;
    unsigned int fFormat;
    bool         fProtected;
} gr_gl_framebufferinfo_t;

// ===== Types from include/gpu/GrDirectContext.h =====

typedef struct gr_direct_context_t gr_direct_context_t;

// ===== Types from include/gpu/gl/GrGLInterface.h =====

typedef struct gr_glinterface_t gr_glinterface_t;

// ===== Types from include/gpu/GrBackendSurface.h =====

typedef struct gr_backendrendertarget_t gr_backendrendertarget_t;

// ===== Types from include/core/SkBlendMode.h =====

typedef enum {
    SK_BLEND_MODE_CLEAR,      // r = 0
    SK_BLEND_MODE_SRC,        // r = s
    SK_BLEND_MODE_DST,        // r = d
    SK_BLEND_MODE_SRCOVER,    // r = s + (1-sa)*d
    SK_BLEND_MODE_DSTOVER,    // r = d + (1-da)*s
    SK_BLEND_MODE_SRCIN,      // r = s * da
    SK_BLEND_MODE_DSTIN,      // r = d * sa
    SK_BLEND_MODE_SRCOUT,     // r = s * (1-da)
    SK_BLEND_MODE_DSTOUT,     // r = d * (1-sa)
    SK_BLEND_MODE_SRCATOP,    // r = s*da + d*(1-sa)
    SK_BLEND_MODE_DSTATOP,    // r = d*sa + s*(1-da)
    SK_BLEND_MODE_XOR,        // r = s*(1-da) + d*(1-sa)
    SK_BLEND_MODE_PLUS,       // r = min(s + d, 1)
    SK_BLEND_MODE_MODULATE,   // r = s*d
    SK_BLEND_MODE_SCREEN,     // r = s + d - s*d
    SK_BLEND_MODE_OVERLAY,    // multiply or screen, depending on destination
    SK_BLEND_MODE_DARKEN,     // rc = s + d - max(s*da, d*sa), ra = kSrcOver
    SK_BLEND_MODE_LIGHTEN,    // rc = s + d - min(s*da, d*sa), ra = kSrcOver
    SK_BLEND_MODE_COLORDODGE, // brighten destination to reflect source
    SK_BLEND_MODE_COLORBURN,  // darken destination to reflect source
    SK_BLEND_MODE_HARDLIGHT,  // multiply or screen, depending on source
    SK_BLEND_MODE_SOFTLIGHT,  // lighten or darken, depending on source
    SK_BLEND_MODE_DIFFERENCE, // rc = s + d - 2*(min(s*da, d*sa)), ra = kSrcOver
    SK_BLEND_MODE_EXCLUSION,  // rc = s + d - two(s*d), ra = kSrcOver
    SK_BLEND_MODE_MULTIPLY,   // r = s*(1-da) + d*(1-sa) + s*d
    SK_BLEND_MODE_HUE,        // hue of source with saturation and luminosity of destination
    SK_BLEND_MODE_SATURATION, // saturation of source with hue and luminosity of destination
    SK_BLEND_MODE_COLOR,      // hue and saturation of source with luminosity of destination
    SK_BLEND_MODE_LUMINOSITY, // luminosity of source with hue and saturation of destination
    SK_BLEND_MODE_LAST_COEFF = SK_BLEND_MODE_SCREEN,
    SK_BLEND_MODE_LAST_SEPARABLE = SK_BLEND_MODE_MULTIPLY,
    SK_BLEND_MODE_LAST = SK_BLEND_MODE_LUMINOSITY,
} sk_blend_mode_t;

// ===== Types from include/core/SkBlurTypes.h =====

typedef enum {
    SK_BLUR_STYLE_NORMAL, // fuzzy inside and outside
    SK_BLUR_STYLE_SOLID,  // solid inside, fuzzy outside
    SK_BLUR_STYLE_OUTER,  // nothing inside, fuzzy outside
    SK_BLUR_STYLE_INNER,  // fuzzy inside, nothing outside
    SK_BLUR_STYLE_LAST = SK_BLUR_STYLE_INNER,
} sk_blur_style_t;

// ===== Types from include/core/SkClipOp.h =====

typedef enum {
    SK_CLIP_OP_DIFFERENCE,
    SK_CLIP_OP_INTERSECT,
    SK_CLIP_OP_LAST = SK_CLIP_OP_INTERSECT,
} sk_clip_op_t;

// ===== Types from include/effects/SkHighContrastFilter.h =====

typedef enum {
    SK_HIGH_CONTRAST_CONFIG_INVERT_STYLE_NO_INVERT,
    SK_HIGH_CONTRAST_CONFIG_INVERT_STYLE_INVERT_BRIGHTNESS,
    SK_HIGH_CONTRAST_CONFIG_INVERT_STYLE_INVERT_LIGHTNESS,
    SK_HIGH_CONTRAST_CONFIG_INVERT_STYLE_LAST = SK_HIGH_CONTRAST_CONFIG_INVERT_STYLE_INVERT_LIGHTNESS,
} sk_high_contrast_config_invert_style_t;

typedef struct {
    bool                                   grayscale;
    sk_high_contrast_config_invert_style_t invertStyle;
    float                                  contrast;
} sk_high_contrast_config_t;

// ===== Types from include/core/SkColor.h =====

typedef enum {
    SK_COLOR_CHANNEL_RED,
    SK_COLOR_CHANNEL_GREEN,
    SK_COLOR_CHANNEL_BLUE,
    SK_COLOR_CHANNEL_ALPHA,
    SK_COLOR_CHANNEL_LAST = SK_COLOR_CHANNEL_ALPHA,
} sk_color_channel_t;

typedef uint32_t sk_color_t;

// ===== Types from include/core/SkColorSpace.h =====

typedef struct sk_color_space_t sk_color_space_t;

// ===== Types from include/core/SkPaint.h =====

typedef enum {
    SK_PAINT_STYLE_FILL,            // set to fill geometry
    SK_PAINT_STYLE_STROKE,          // set to stroke geometry
    SK_PAINT_STYLE_STROKE_AND_FILL, // set to stroke and fill geometry
} sk_paint_style_t;

typedef enum {
    SK_STROKE_CAP_BUTT,   // no stroke extension
    SK_STROKE_CAP_ROUND,  // adds circle
    SK_STROKE_CAP_SQUARE, // adds square
    SK_STROKE_CAP_LAST    = SK_STROKE_CAP_SQUARE,
    SK_STROKE_CAP_DEFAULT = SK_STROKE_CAP_BUTT,
} sk_stroke_cap_t;

typedef enum {
    SK_STROKE_JOIN_MITER, // extends to miter limit
    SK_STROKE_JOIN_ROUND, // adds circle
    SK_STROKE_JOIN_BEVEL, // connects outside edges
    SK_STROKE_JOIN_LAST    = SK_STROKE_JOIN_BEVEL,
    SK_STROKE_JOIN_DEFAULT = SK_STROKE_JOIN_MITER,
} sk_stroke_join_t;

typedef struct sk_paint_t sk_paint_t;

// ===== Types from include/core/SkColorType.h =====

typedef enum {
    SK_COLOR_TYPE_UNKNOWN,          // uninitialized
    SK_COLOR_TYPE_ALPHA_8,          // pixel with alpha in 8-bit byte
    SK_COLOR_TYPE_RGB_565,          // pixel with 5 bits red, 6 bits green, 5 bits blue, in 16-bit word
    SK_COLOR_TYPE_ARGB_4444,        // pixel with 4 bits for alpha, red, green, blue; in 16-bit word
    SK_COLOR_TYPE_RGBA_8888,        // pixel with 8 bits for red, green, blue, alpha; in 32-bit word
    SK_COLOR_TYPE_RGB_888X,         // pixel with 8 bits each for red, green, blue; in 32-bit word
    SK_COLOR_TYPE_BGRA_8888,        // pixel with 8 bits for blue, green, red, alpha; in 32-bit word
    SK_COLOR_TYPE_RGBA_1010102,     // 10 bits for red, green, blue; 2 bits for alpha; in 32-bit word
    SK_COLOR_TYPE_BGRA_1010102,     // 10 bits for blue, green, red; 2 bits for alpha; in 32-bit word
    SK_COLOR_TYPE_RGB_101010X,      // pixel with 10 bits each for red, green, blue; in 32-bit word
    SK_COLOR_TYPE_BGR_101010X,      // pixel with 10 bits each for blue, green, red; in 32-bit word
    SK_COLOR_TYPE_BGR_101010X_XR,   // pixel with 10 bits each for blue, green, red; in 32-bit word, extended range
	SK_COLOR_TYPE_BGRA_10101010_XR, // pixel with 10 bits each for blue, green, red, alpha; in 64-bit word, extended range
    SK_COLOR_TYPE_RGBA_10x6,        // pixel with 10 used bits (most significant) followed by 6 unused bits for red, green, blue, alpha; in 64-bit word
    SK_COLOR_TYPE_GRAY_8,           // pixel with grayscale level in 8-bit byte
    SK_COLOR_TYPE_RGBA_F16_NORM,    // pixel with half floats in [0,1] for red, green, blue, alpha; in 64-bit word
    SK_COLOR_TYPE_RGBA_F16,         // pixel with half floats for red, green, blue, alpha; in 64-bit word
    SK_COLOR_TYPE_RGBA_F32,         // pixel using C float for red, green, blue, alpha; in 128-bit word

    // The following color types are read-only
    SK_COLOR_TYPE_R8G8_UNORM,         // pixel with a uint8_t for red and green
    SK_COLOR_TYPE_A16_FLOAT,          // pixel with a half float for alpha
    SK_COLOR_TYPE_R16G16_FLOAT,       // pixel with a half float for red and green
    SK_COLOR_TYPE_A16_UNORM,          // pixel with a little endian uint16_t for alpha
    SK_COLOR_TYPE_R16G16_UNORM,       // pixel with a little endian uint16_t for red and green
    SK_COLOR_TYPE_R16G16B16A16_UNORM, // pixel with a little endian uint16_t for red, green, blue and alpha

    SK_COLOR_TYPE_SRGBA_8888,         // pixel with 8 bits for red, green, blue, alpha; in 32-bit word with conversion between sRGB and linear space
    SK_COLOR_TYPE_R8_UNORM,
    SK_COLOR_TYPE_LAST = SK_COLOR_TYPE_R8_UNORM,
#if defined(SK_BUILD_FOR_WIN)
    SK_COLOR_TYPE_N32 = SK_COLOR_TYPE_BGRA_8888, // native 32-bit BGRA encoding
#else
    SK_COLOR_TYPE_N32 = SK_COLOR_TYPE_RGBA_8888, // native 32-bit RGBA encoding
#endif
} sk_color_type_t;

// ===== Types from include/core/SkAlphaType.h =====

typedef enum {
    SK_ALPHA_TYPE_UNKNOWN,
    SK_ALPHA_TYPE_OPAQUE,
    SK_ALPHA_TYPE_PREMUL,
    SK_ALPHA_TYPE_UNPREMUL,
    SK_ALPHA_TYPE_LAST = SK_ALPHA_TYPE_UNPREMUL,
} sk_alpha_type_t;

// ===== Types from include/core/SkImageInfo.h =====

typedef struct {
    sk_color_space_t* colorSpace;
    sk_color_type_t   colorType;
    sk_alpha_type_t   alphaType;
    int32_t           width;
    int32_t           height;
} sk_image_info_t;

// ===== Types from include/core/SkImage.h =====

typedef enum {
    SK_IMAGE_CACHING_HINT_ALLOW,    // allows internally caching decoded and copied pixels
    SK_IMAGE_CACHING_HINT_DISALLOW, // disallows internally caching decoded and copied pixels
} sk_image_caching_hint_t;

typedef struct sk_image_t sk_image_t;

// ===== Types from include/core/SkImageFilter.h =====

typedef struct sk_image_filter_t sk_image_filter_t;

// ===== Types from include/core/SkMaskFilter.h =====

typedef struct sk_mask_filter_t sk_mask_filter_t;

// ===== Types from include/core/SkColorFilter.h =====

typedef struct sk_color_filter_t sk_color_filter_t;

// ===== Types from include/core/SkSamplingOptions.h =====

typedef struct {
    float B;
    float C;
} sk_cubic_resampler_t;

typedef enum {
    SK_FILTER_MODE_NEAREST,
    SK_FILTER_MODE_LINEAR,
    SK_FILTER_MODE_LAST = SK_FILTER_MODE_LINEAR,
} sk_filter_mode_t;

typedef enum {
    SK_MIPMAP_MODE_NONE,
    SK_MIPMAP_MODE_NEAREST,
    SK_MIPMAP_MODE_LINEAR,
    SK_MIPMAP_MODE_LAST = SK_MIPMAP_MODE_LINEAR,
} sk_mipmap_mode_t;

typedef struct {
	int                  maxAniso;
    bool                 useCubic;
    sk_cubic_resampler_t cubic;
    sk_filter_mode_t     filter;
    sk_mipmap_mode_t     mipmap;
} sk_sampling_options_t;

// ===== Types from include/core/SkTypeface.h =====

typedef struct sk_typeface_t sk_typeface_t;

// ===== Types from include/core/SkFontTypes.h =====

typedef enum {
    SK_FONT_HINTING_NONE,   // glyph outlines unchanged
    SK_FONT_HINTING_SLIGHT, // minimal modification to improve contrast
    SK_FONT_HINTING_NORMAL, // glyph outlines modified to improve contrast
    SK_FONT_HINTING_FULL,   // modifies glyph outlines for maximum contrast
} sk_font_hinting_t;

typedef enum {
    SK_TEXT_ENCODING_UTF8,     // uses bytes to represent UTF-8 or ASCII
    SK_TEXT_ENCODING_UTF16,    // uses two byte words to represent most of Unicode
    SK_TEXT_ENCODING_UTF32,    // uses four byte words to represent all of Unicode
    SK_TEXT_ENCODING_GLYPH_ID, // uses two byte words to represent glyph indices
} sk_text_encoding_t;

// ===== Types from include/core/SkFontMgr.h =====

typedef struct sk_font_mgr_t sk_font_mgr_t;
typedef struct sk_font_style_set_t sk_font_style_set_t;

// ===== Types from include/core/SkFontStyle.h =====

typedef enum {
    SK_FONT_STYLE_WEIGHT_INVISIBLE   = 0,
    SK_FONT_STYLE_WEIGHT_THIN        = 100,
    SK_FONT_STYLE_WEIGHT_EXTRA_LIGHT = 200,
    SK_FONT_STYLE_WEIGHT_LIGHT       = 300,
    SK_FONT_STYLE_WEIGHT_NORMAL      = 400,
    SK_FONT_STYLE_WEIGHT_MEDIUM      = 500,
    SK_FONT_STYLE_WEIGHT_SEMI_BOLD   = 600,
    SK_FONT_STYLE_WEIGHT_BOLD        = 700,
    SK_FONT_STYLE_WEIGHT_EXTRA_BOLD  = 800,
    SK_FONT_STYLE_WEIGHT_BLACK       = 900,
    SK_FONT_STYLE_WEIGHT_EXTRA_BLACK = 1000,
} sk_font_style_weight_t;

typedef enum {
    SK_FONT_STYLE_WIDTH_ULTRA_CONDENSED = 1,
    SK_FONT_STYLE_WIDTH_EXTRA_CONDENSED = 2,
    SK_FONT_STYLE_WIDTH_CONDENSED       = 3,
    SK_FONT_STYLE_WIDTH_SEMI_CONDENSED  = 4,
    SK_FONT_STYLE_WIDTH_NORMAL          = 5,
    SK_FONT_STYLE_WIDTH_SEMI_EXPANDED   = 6,
    SK_FONT_STYLE_WIDTH_EXPANDED        = 7,
    SK_FONT_STYLE_WIDTH_EXTRA_EXPANDED  = 8,
    SK_FONT_STYLE_WIDTH_ULTRA_EXPANDED  = 9,
} sk_font_style_width_t;

typedef enum {
    SK_FONT_STYLE_SLANT_UPRIGHT,
    SK_FONT_STYLE_SLANT_ITALIC,
    SK_FONT_STYLE_SLANT_OBLIQUE,
} sk_font_style_slant_t;

typedef struct sk_font_style_t sk_font_style_t;

// ===== Types from include/core/SkFontMetrics.h =====

typedef enum {
    SK_FONT_METRICS_FLAG_UNDERLINE_THICKNESS_IS_VALID = 1 << 0, // set if underlineThickness is valid
    SK_FONT_METRICS_FLAG_UNDERLINE_POSITION_IS_VALID  = 1 << 1, // set if underlinePosition is valid
    SK_FONT_METRICS_FLAG_STRIKEOUT_THICKNESS_IS_VALID = 1 << 2, // set if strikeoutThickness is valid
    SK_FONT_METRICS_FLAG_STRIKEOUT_POSITION_IS_VALID  = 1 << 3, // set if strikeoutPosition is valid
    SK_FONT_METRICS_FLAG_BOUNDS_INVALID               = 1 << 4, // set if top, bottom, xMin, xMax invalid
} sk_font_metrics_flags_t;

typedef struct {
    uint32_t flags;              // FontMetricsFlags indicating which metrics are valid
    float    top;                // greatest extent above origin of any glyph bounding box, typically negative; deprecated with variable fonts
    float    ascent;             // distance to reserve above baseline, typically negative
    float    descent;            // distance to reserve below baseline, typically positive
    float    bottom;             // greatest extent below origin of any glyph bounding box, typically positive; deprecated with variable fonts
    float    leading;            // distance to add between lines, typically positive or zero
    float    avgCharWidth;       // average character width, zero if unknown
    float    maxCharWidth;       // maximum character width, zero if unknown
    float    xMin;               // greatest extent to left of origin of any glyph bounding box, typically negative; deprecated with variable fonts
    float    xMax;               // greatest extent to right of origin of any glyph bounding box, typically positive; deprecated with variable fonts
    float    xHeight;            // height of lower-case 'x', zero if unknown, typically negative
    float    capHeight;          // height of an upper-case letter, zero if unknown, typically negative
    float    underlineThickness; // underline thickness
    float    underlinePosition;  // distance from baseline to top of stroke, typically positive
    float    strikeoutThickness; // strikeout thickness
    float    strikeoutPosition;  // distance from baseline to bottom of stroke, typically negative
} sk_font_metrics_t;

// ===== Types from include/core/SkFont.h =====

typedef struct sk_font_t sk_font_t;

// ===== Types from include/core/SkCanvas.h =====

typedef enum {
    SK_POINT_MODE_POINTS,  // draw each point separately
    SK_POINT_MODE_LINES,   // draw each pair of points as a line segment
    SK_POINT_MODE_POLYGON, // draw the array of points as a open polygon
} sk_point_mode_t;

typedef enum {
    SRC_RECT_CONSTRAINT_STRICT, // Sample only inside bounds; slower
    SRC_RECT_CONSTRAINT_FAST,   // Sample outside bounds; faster
} sk_src_rect_constraint_t;

typedef struct sk_canvas_t sk_canvas_t;

// ===== Types from include/core/SkShader.h =====

typedef enum {
    SK_TILE_MODE_CLAMP, // Replicate the edge color if the shader draws outside of its original bounds
    SK_TILE_REPEAT,     // Repeat the shader's image horizontally and vertically
    SK_TILE_MIRROR,     // Repeat the shader's image horizontally and vertically, alternating mirror images so that adjacent images always seam
    SK_TILE_DECAL,      // Only draw within the original domain, return transparent-black everywhere else
    SK_TILE_LAST = SK_TILE_DECAL,
} sk_tile_mode_t;

typedef struct sk_shader_t sk_shader_t;

// ===== Types from include/core/SkTime.h =====

typedef struct {
	int16_t  timeZoneMinutes;
	uint16_t year;
	uint8_t  month;
	uint8_t  dayOfWeek;
	uint8_t  day;
	uint8_t  hour;
	uint8_t  minute;
	uint8_t  second;
} sk_date_time_t;

// ===== Types from include/core/SkStream.h =====

typedef struct sk_wstream_t sk_wstream_t;
typedef struct sk_file_wstream_t sk_file_wstream_t;
typedef struct sk_dynamic_memory_wstream_t sk_dynamic_memory_wstream_t;

// ===== Types from include/docs/SkPDFDocument.h =====

typedef struct {
    char*          title;
    char*          author;
    char*          subject;
    char*          keywords;
    char*          creator;
    char*          producer;
    sk_date_time_t creation;
    sk_date_time_t modified;
    float          rasterDPI;
    float          unused;
    int            encodingQuality;
} sk_metadata_t;

// ===== Types from include/core/SkDocument.h =====

typedef struct sk_document_t sk_document_t;


// ======================================================


// ===== Functions from include/gpu/GrBackendSurface.h =====
SK_C_API gr_backendrendertarget_t* gr_backendrendertarget_new_gl(int width, int height, int samples, int stencils, const gr_gl_framebufferinfo_t* glInfo);
SK_C_API void gr_backendrendertarget_delete(gr_backendrendertarget_t* rendertarget);
SK_C_API gr_direct_context_t* gr_direct_context_make_gl(const gr_glinterface_t* glInterface);

// ===== Functions from include/gpu/GrDirectContext.h =====
SK_C_API void gr_direct_context_abandon_context(gr_direct_context_t* context);
SK_C_API void gr_direct_context_delete(gr_direct_context_t* context);
SK_C_API void gr_direct_context_flush_and_submit(gr_direct_context_t* context, bool syncCPU);
SK_C_API void gr_direct_context_release_resources_and_abandon_context(gr_direct_context_t* context);
SK_C_API void gr_direct_context_reset(gr_direct_context_t* context);
SK_C_API void gr_direct_context_reset_gl_texture_bindings(gr_direct_context_t* context);
SK_C_API void gr_direct_context_unref(const gr_direct_context_t* context);

// ===== Functions from include/gpu/gl/GrGLInterface.h =====
SK_C_API const gr_glinterface_t* gr_glinterface_create_native_interface(void);
SK_C_API void gr_glinterface_unref(const gr_glinterface_t* intf);

// ===== Functions from include/core/SkCanvas.h =====
SK_C_API sk_surface_t* sk_canvas_get_surface(sk_canvas_t* canvas);
SK_C_API void sk_canvas_clear(sk_canvas_t* canvas, sk_color_t color);
SK_C_API void sk_canvas_clip_path_with_operation(sk_canvas_t* t, const sk_path_t* crect, sk_clip_op_t op, bool doAA);
SK_C_API void sk_canvas_clip_rect_with_operation(sk_canvas_t* t, const sk_rect_t* crect, sk_clip_op_t op, bool doAA);
SK_C_API void sk_canvas_concat(sk_canvas_t* canvas, const sk_matrix_t* matrix);
SK_C_API void sk_canvas_draw_arc(sk_canvas_t* canvas, const sk_rect_t* oval, float startAngle, float sweepAngle, bool useCenter, const sk_paint_t* paint);
SK_C_API void sk_canvas_draw_circle(sk_canvas_t* canvas, float cx, float cy, float rad, const sk_paint_t* cpaint);
SK_C_API void sk_canvas_draw_color(sk_canvas_t* canvas, sk_color_t color, sk_blend_mode_t mode);
SK_C_API void sk_canvas_draw_image_nine(sk_canvas_t* t, const sk_image_t* image, const sk_irect_t* center, const sk_rect_t* dst, sk_filter_mode_t filter, const sk_paint_t* paint);
SK_C_API void sk_canvas_draw_image_rect(sk_canvas_t* canvas, const sk_image_t* cimage, const sk_rect_t* csrcR, const sk_rect_t* cdstR, const sk_sampling_options_t *samplingOptions, const sk_paint_t* cpaint, sk_src_rect_constraint_t constraint);
SK_C_API void sk_canvas_draw_line(sk_canvas_t* canvas, float x0, float y0, float x1, float y1, const sk_paint_t* cpaint);
SK_C_API void sk_canvas_draw_oval(sk_canvas_t* canvas, const sk_rect_t* crect, const sk_paint_t* cpaint);
SK_C_API void sk_canvas_draw_paint(sk_canvas_t* canvas, const sk_paint_t* cpaint);
SK_C_API void sk_canvas_draw_path(sk_canvas_t* canvas, const sk_path_t* cpath, const sk_paint_t* cpaint);
SK_C_API void sk_canvas_draw_point(sk_canvas_t* canvas, float x, float y, const sk_paint_t* cpaint);
SK_C_API void sk_canvas_draw_points(sk_canvas_t* canvas, sk_point_mode_t pointMode, size_t count, const sk_point_t points [], const sk_paint_t* cpaint);
SK_C_API void sk_canvas_draw_rect(sk_canvas_t* canvas, const sk_rect_t* crect, const sk_paint_t* cpaint);
SK_C_API void sk_canvas_draw_round_rect(sk_canvas_t* canvas, const sk_rect_t* crect, float rx, float ry, const sk_paint_t* cpaint);
SK_C_API void sk_canvas_draw_simple_text(sk_canvas_t* canvas, const void* text, size_t byte_length, sk_text_encoding_t encoding, float x, float y, const sk_font_t* cfont, const sk_paint_t* cpaint);
SK_C_API void sk_canvas_draw_text_blob (sk_canvas_t* canvas, sk_text_blob_t* text, float x, float y, const sk_paint_t* paint);
SK_C_API bool sk_canvas_get_local_clip_bounds(sk_canvas_t* canvas, sk_rect_t* cbounds);
SK_C_API int sk_canvas_get_save_count(sk_canvas_t* canvas);
SK_C_API void sk_canvas_get_total_matrix(sk_canvas_t* canvas, sk_matrix_t* matrix);
SK_C_API bool sk_canvas_is_clip_empty(sk_canvas_t* canvas);
SK_C_API bool sk_canvas_is_clip_rect(sk_canvas_t* canvas);
SK_C_API bool sk_canvas_quick_reject_path(sk_canvas_t* canvas, const sk_path_t* path);
SK_C_API bool sk_canvas_quick_reject_rect(sk_canvas_t* canvas, const sk_rect_t* rect);
SK_C_API void sk_canvas_reset_matrix(sk_canvas_t* canvas);
SK_C_API void sk_canvas_restore(sk_canvas_t* canvas);
SK_C_API void sk_canvas_restore_to_count(sk_canvas_t* canvas, int saveCount);
SK_C_API void sk_canvas_rotate_radians(sk_canvas_t* canvas, float radians);
SK_C_API int sk_canvas_save(sk_canvas_t* canvas);
SK_C_API int sk_canvas_save_layer(sk_canvas_t* canvas, const sk_rect_t* crect, const sk_paint_t* cpaint);
SK_C_API int sk_canvas_save_layer_alpha(sk_canvas_t* canvas, const sk_rect_t* crect, uint8_t alpha);
SK_C_API void sk_canvas_scale(sk_canvas_t* canvas, float sx, float sy);
SK_C_API void sk_canvas_set_matrix(sk_canvas_t* canvas, const sk_matrix_t* matrix);
SK_C_API void sk_canvas_skew(sk_canvas_t* canvas, float sx, float sy);
SK_C_API void sk_canvas_translate(sk_canvas_t* canvas, float dx, float dy);

// ===== Functions from include/core/SkColorFilter.h =====
SK_C_API sk_color_filter_t* sk_colorfilter_new_color_matrix(const float array[20]);
SK_C_API sk_color_filter_t* sk_colorfilter_new_compose(sk_color_filter_t* outer, sk_color_filter_t* inner);
SK_C_API sk_color_filter_t* sk_colorfilter_new_high_contrast(const sk_high_contrast_config_t* config);
SK_C_API sk_color_filter_t* sk_colorfilter_new_lighting(sk_color_t mul, sk_color_t add);
SK_C_API sk_color_filter_t* sk_colorfilter_new_luma_color(void);
SK_C_API sk_color_filter_t* sk_colorfilter_new_mode(sk_color_t c, sk_blend_mode_t mode);
SK_C_API void sk_colorfilter_unref(sk_color_filter_t* filter);

// ===== Functions from include/core/SkColorSpace.h =====
SK_C_API sk_color_space_t* sk_colorspace_new_srgb(void);

// ===== Functions from include/core/SkData.h =====
SK_C_API const void* sk_data_get_data(const sk_data_t* data);
SK_C_API size_t sk_data_get_size(const sk_data_t* data);
SK_C_API sk_data_t* sk_data_new_with_copy(const void* src, size_t length);
SK_C_API void sk_data_unref(const sk_data_t* data);

// ===== Functions from include/encode/SkJpegEncoder.h =====
SK_C_API sk_data_t* sk_encode_jpeg(gr_direct_context_t* ctx, const sk_image_t* img, int quality);

// ===== Functions from include/encode/SkPngEncoder.h =====
SK_C_API sk_data_t* sk_encode_png(gr_direct_context_t* ctx, const sk_image_t* img, int compressionLevel);

// ===== Functions from include/encode/SkWebpEncoder.h =====
SK_C_API sk_data_t* sk_encode_webp(gr_direct_context_t* ctx, const sk_image_t* img, float quality, bool lossy);

// ===== Functions from include/core/SkFont.h =====
SK_C_API void sk_font_delete(sk_font_t* font);
SK_C_API float sk_font_get_metrics(const sk_font_t* font, sk_font_metrics_t* metrics);
SK_C_API void sk_font_get_xpos(const sk_font_t* font, const uint16_t glyphs[], int count, float xpos[], float origin);
SK_C_API float sk_font_measure_text(const sk_font_t* font, const void* text, size_t byteLength, sk_text_encoding_t encoding, sk_rect_t* bounds, const sk_paint_t* paint);
SK_C_API sk_font_t* sk_font_new_with_values(sk_typeface_t* typeface, float size, float scaleX, float skewX);
SK_C_API void sk_font_set_force_auto_hinting(sk_font_t* font, bool value);
SK_C_API void sk_font_set_hinting(sk_font_t* font, sk_font_hinting_t value);
SK_C_API void sk_font_set_subpixel(sk_font_t* font, bool value);
SK_C_API int sk_font_text_to_glyphs(const sk_font_t* font, const void* text, size_t byteLength, sk_text_encoding_t encoding, uint16_t glyphs[], int maxGlyphCount);
SK_C_API uint16_t sk_font_unichar_to_glyph(const sk_font_t* font, int32_t unichar);
SK_C_API void sk_font_unichars_to_glyphs(const sk_font_t* font, const int32_t* unichars, int count, uint16_t* glyphs);
SK_C_API void sk_font_glyph_widths(const sk_font_t* font, const uint16_t *glyphs, int count, float *widths);

// ===== Functions from include/core/SkFontMgr.h =====
SK_C_API int sk_fontmgr_count_families(sk_font_mgr_t* fontmgr);
SK_C_API sk_typeface_t* sk_fontmgr_create_from_data(sk_font_mgr_t* fontmgr, sk_data_t* data, int index);
SK_C_API void sk_fontmgr_get_family_name(sk_font_mgr_t* fontmgr, int index, sk_string_t* familyName);
SK_C_API sk_font_style_set_t* sk_fontmgr_match_family(sk_font_mgr_t* fontmgr, const char* familyName);
SK_C_API sk_typeface_t* sk_fontmgr_match_family_style(sk_font_mgr_t* fontmgr, const char* familyName, sk_font_style_t* style);
SK_C_API sk_typeface_t* sk_fontmgr_match_family_style_character(sk_font_mgr_t* fontmgr, const char* familyName, sk_font_style_t* style, const char** bcp47, int bcp47Count, int32_t character);
SK_C_API sk_font_mgr_t* sk_fontmgr_ref_default(void);

SK_C_API sk_typeface_t* sk_fontstyleset_create_typeface(sk_font_style_set_t* fss, int index);
SK_C_API int sk_fontstyleset_get_count(sk_font_style_set_t* fss);
SK_C_API void sk_fontstyleset_get_style(sk_font_style_set_t* fss, int index, sk_font_style_t* fs, sk_string_t* style);
SK_C_API sk_typeface_t* sk_fontstyleset_match_style(sk_font_style_set_t* fss, sk_font_style_t* style);
SK_C_API void sk_fontstyleset_unref(sk_font_style_set_t* fss);

// ===== Functions from include/core/SkFontStyle.h =====
SK_C_API void sk_fontstyle_delete(sk_font_style_t* fs);
SK_C_API sk_font_style_slant_t sk_fontstyle_get_slant(const sk_font_style_t* fs);
SK_C_API int sk_fontstyle_get_weight(const sk_font_style_t* fs);
SK_C_API int sk_fontstyle_get_width(const sk_font_style_t* fs);
SK_C_API sk_font_style_t* sk_fontstyle_new(int weight, int width, sk_font_style_slant_t slant);

// ===== Functions from include/core/SkImage.h =====
SK_C_API sk_alpha_type_t sk_image_get_alpha_type(const sk_image_t* image);
SK_C_API sk_color_type_t sk_image_get_color_type(const sk_image_t* image);
SK_C_API sk_color_space_t* sk_image_get_colorspace(const sk_image_t* image);
SK_C_API int sk_image_get_height(const sk_image_t* image);
SK_C_API int sk_image_get_width(const sk_image_t* image);
SK_C_API sk_image_t* sk_image_make_non_texture_image(const sk_image_t* image);
SK_C_API sk_shader_t* sk_image_make_shader(const sk_image_t* image, sk_tile_mode_t tileX, sk_tile_mode_t tileY, const sk_sampling_options_t *samplingOptions, const sk_matrix_t* cmatrix);
SK_C_API sk_image_t* sk_image_new_from_encoded(sk_data_t* encoded);
SK_C_API sk_image_t* sk_image_new_raster_data(const sk_image_info_t* cinfo, sk_data_t* pixels, size_t rowBytes);
SK_C_API bool sk_image_read_pixels(const sk_image_t* image, const sk_image_info_t* dstInfo, void* dstPixels, size_t dstRowBytes, int srcX, int srcY, sk_image_caching_hint_t cachingHint);
SK_C_API void sk_image_unref(const sk_image_t* image);

// ===== Functions from include/gpu/ganesh/SkImageGanesh.h =====
SK_C_API sk_image_t* sk_image_texture_from_image(gr_direct_context_t* ctx, const sk_image_t* image, bool mipmapped, bool budgeted);

// ===== Functions from include/core/SkImageFilter.h =====
SK_C_API sk_image_filter_t* sk_imagefilter_new_arithmetic(float k1, float k2, float k3, float k4, bool enforcePMColor, sk_image_filter_t* background, sk_image_filter_t* foreground, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_blur(float sigmaX, float sigmaY, sk_tile_mode_t tileMode, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_color_filter(sk_color_filter_t* cf, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_compose(sk_image_filter_t* outer, sk_image_filter_t* inner);
SK_C_API sk_image_filter_t* sk_imagefilter_new_dilate(int radiusX, int radiusY, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_displacement_map_effect(sk_color_channel_t xChannelSelector, sk_color_channel_t yChannelSelector, float scale, sk_image_filter_t* displacement, sk_image_filter_t* color, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_distant_lit_diffuse(const sk_point3_t* direction, sk_color_t lightColor, float surfaceScale, float kd, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_distant_lit_specular(const sk_point3_t* direction, sk_color_t lightColor, float surfaceScale, float ks, float shininess, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_drop_shadow(float dx, float dy, float sigmaX, float sigmaY, sk_color_t color, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_drop_shadow_only(float dx, float dy, float sigmaX, float sigmaY, sk_color_t color, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_erode(int radiusX, int radiusY, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_image_source(sk_image_t* image, const sk_rect_t* srcRect, const sk_rect_t* dstRect, const sk_sampling_options_t* samplingOptions);
SK_C_API sk_image_filter_t* sk_imagefilter_new_image_source_default(sk_image_t* image, const sk_sampling_options_t* samplingOptions);
SK_C_API sk_image_filter_t* sk_imagefilter_new_magnifier(const sk_rect_t* lensBounds, float zoomAmount, float inset, const sk_sampling_options_t* samplingOptions, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_matrix_convolution(const sk_isize_t* kernelSize, const float kernel[], float gain, float bias, const sk_ipoint_t* kernelOffset, sk_tile_mode_t tileMode, bool convolveAlpha, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_matrix_transform(const sk_matrix_t* matrix, const sk_sampling_options_t *samplingOptions, sk_image_filter_t* input);
SK_C_API sk_image_filter_t* sk_imagefilter_new_merge(sk_image_filter_t* filters[], int count, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_offset(float dx, float dy, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_point_lit_diffuse(const sk_point3_t* location, sk_color_t lightColor, float surfaceScale, float kd, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_point_lit_specular(const sk_point3_t* location, sk_color_t lightColor, float surfaceScale, float ks, float shininess, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_spot_lit_diffuse(const sk_point3_t* location, const sk_point3_t* target, float specularExponent, float cutoffAngle, sk_color_t lightColor, float surfaceScale, float kd, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_spot_lit_specular(const sk_point3_t* location, const sk_point3_t* target, float specularExponent, float cutoffAngle, sk_color_t lightColor, float surfaceScale, float ks, float shininess, sk_image_filter_t* input, const sk_rect_t* cropRect);
SK_C_API sk_image_filter_t* sk_imagefilter_new_tile(const sk_rect_t* src, const sk_rect_t* dst, sk_image_filter_t* input);
SK_C_API void sk_imagefilter_unref(sk_image_filter_t* filter);

// ===== Functions from include/core/SkMaskFilter.h =====
SK_C_API sk_mask_filter_t* sk_maskfilter_new_blur_with_flags(sk_blur_style_t, float sigma, bool respectCTM);
SK_C_API sk_mask_filter_t* sk_maskfilter_new_clip(uint8_t min, uint8_t max);
SK_C_API sk_mask_filter_t* sk_maskfilter_new_gamma(float gamma);
SK_C_API sk_mask_filter_t* sk_maskfilter_new_shader(sk_shader_t* cshader);
SK_C_API sk_mask_filter_t* sk_maskfilter_new_table(const uint8_t table[256]);
SK_C_API void sk_maskfilter_unref(sk_mask_filter_t* filter);

// ===== Functions from include/core/SkPaint.h =====

SK_C_API bool sk_paint_equivalent(sk_paint_t* cpaint, sk_paint_t* other);
SK_C_API sk_paint_t* sk_paint_clone(sk_paint_t* cpaint);
SK_C_API void sk_paint_delete(sk_paint_t* cpaint);
SK_C_API sk_blend_mode_t sk_paint_get_blend_mode_or(sk_paint_t* cpaint, sk_blend_mode_t defaultMode);
SK_C_API sk_color_t sk_paint_get_color(const sk_paint_t* cpaint);
SK_C_API sk_color_filter_t* sk_paint_get_colorfilter(sk_paint_t* cpaint);
SK_C_API bool sk_paint_get_fill_path(const sk_paint_t* cpaint, const sk_path_t* src, sk_path_t* dst, const sk_rect_t* cullRect, float resScale);
SK_C_API sk_image_filter_t* sk_paint_get_imagefilter(sk_paint_t* cpaint);
SK_C_API sk_mask_filter_t* sk_paint_get_maskfilter(sk_paint_t* cpaint);
SK_C_API sk_path_effect_t* sk_paint_get_path_effect(sk_paint_t* cpaint);
SK_C_API sk_shader_t* sk_paint_get_shader(sk_paint_t* cpaint);
SK_C_API sk_stroke_cap_t sk_paint_get_stroke_cap(const sk_paint_t* cpaint);
SK_C_API sk_stroke_join_t sk_paint_get_stroke_join(const sk_paint_t* cpaint);
SK_C_API float sk_paint_get_stroke_miter(const sk_paint_t* cpaint);
SK_C_API float sk_paint_get_stroke_width(const sk_paint_t* cpaint);
SK_C_API sk_paint_style_t sk_paint_get_style(const sk_paint_t* cpaint);
SK_C_API bool sk_paint_is_antialias(const sk_paint_t* cpaint);
SK_C_API bool sk_paint_is_dither(const sk_paint_t* cpaint);
SK_C_API sk_paint_t* sk_paint_new(void);
SK_C_API void sk_paint_reset(sk_paint_t* cpaint);
SK_C_API void sk_paint_set_antialias(sk_paint_t* cpaint, bool aa);
SK_C_API void sk_paint_set_blend_mode(sk_paint_t* paint, sk_blend_mode_t mode);
SK_C_API void sk_paint_set_color(sk_paint_t* cpaint, sk_color_t c);
SK_C_API void sk_paint_set_colorfilter(sk_paint_t* cpaint, sk_color_filter_t* cfilter);
SK_C_API void sk_paint_set_dither(sk_paint_t* cpaint, bool isdither);
SK_C_API void sk_paint_set_imagefilter(sk_paint_t* cpaint, sk_image_filter_t* cfilter);
SK_C_API void sk_paint_set_maskfilter(sk_paint_t* cpaint, sk_mask_filter_t* cfilter);
SK_C_API void sk_paint_set_path_effect(sk_paint_t* cpaint, sk_path_effect_t* effect);
SK_C_API void sk_paint_set_shader(sk_paint_t* cpaint, sk_shader_t* cshader);
SK_C_API void sk_paint_set_stroke_cap(sk_paint_t* cpaint, sk_stroke_cap_t ccap);
SK_C_API void sk_paint_set_stroke_join(sk_paint_t* cpaint, sk_stroke_join_t cjoin);
SK_C_API void sk_paint_set_stroke_miter(sk_paint_t* cpaint, float miter);
SK_C_API void sk_paint_set_stroke_width(sk_paint_t* cpaint, float width);
SK_C_API void sk_paint_set_style(sk_paint_t* cpaint, sk_paint_style_t style);

// ===== Functions from include/core/SkPath.h =====
SK_C_API void sk_path_add_circle(sk_path_t* cpath, float x, float y, float radius, sk_path_direction_t dir);
SK_C_API void sk_path_add_oval(sk_path_t* cpath, const sk_rect_t* crect, sk_path_direction_t cdir);
SK_C_API void sk_path_add_path (sk_path_t* cpath, sk_path_t* other, sk_path_add_mode_t add_mode);
SK_C_API void sk_path_add_path_matrix(sk_path_t* cpath, sk_path_t* other, sk_matrix_t *matrix, sk_path_add_mode_t add_mode);
SK_C_API void sk_path_add_path_offset(sk_path_t* cpath, sk_path_t* other, float dx, float dy, sk_path_add_mode_t add_mode);
SK_C_API void sk_path_add_path_reverse(sk_path_t* cpath, sk_path_t* other);
SK_C_API void sk_path_add_poly(sk_path_t* cpath, const sk_point_t* points, int count, bool close);
SK_C_API void sk_path_add_rect(sk_path_t* cpath, const sk_rect_t* crect, sk_path_direction_t cdir);
SK_C_API void sk_path_add_rounded_rect(sk_path_t* cpath, const sk_rect_t* crect, float rx, float ry, sk_path_direction_t cdir);
SK_C_API void sk_path_arc_to(sk_path_t* cpath, float rx, float ry, float xAxisRotate, sk_path_arc_size_t largeArc, sk_path_direction_t sweep, float x, float y);
SK_C_API void sk_path_arc_to_with_oval(sk_path_t* cpath, const sk_rect_t* oval, float startAngle, float sweepAngle, bool forceMoveTo);
SK_C_API void sk_path_arc_to_with_points(sk_path_t* cpath, float x1, float y1, float x2, float y2, float radius);
SK_C_API sk_path_t* sk_path_clone(const sk_path_t* cpath);
SK_C_API void sk_path_close(sk_path_t* cpath);
SK_C_API void sk_path_compute_tight_bounds(const sk_path_t* cpath, sk_rect_t* crect);
SK_C_API void sk_path_conic_to(sk_path_t* cpath, float x0, float y0, float x1, float y1, float w);
SK_C_API bool sk_path_contains(const sk_path_t* cpath, float x, float y);
SK_C_API int sk_path_count_points(const sk_path_t* cpath);
SK_C_API void sk_path_cubic_to(sk_path_t*, float x0, float y0, float x1, float y1, float x2, float y2);
SK_C_API void sk_path_delete(sk_path_t* cpath);
SK_C_API void sk_path_get_bounds(const sk_path_t* cpath, sk_rect_t* crect);
SK_C_API int sk_path_get_points(const sk_path_t* cpath, sk_point_t* points, int max);
SK_C_API sk_path_fill_type_t sk_path_get_filltype(sk_path_t *cpath);
SK_C_API bool sk_path_get_last_point(const sk_path_t* cpath, sk_point_t* point);
SK_C_API void sk_path_line_to(sk_path_t *cpath, float x, float y);
SK_C_API void sk_path_move_to(sk_path_t *cpath, float x, float y);
SK_C_API sk_path_t* sk_path_new(void);
SK_C_API bool sk_path_parse_svg_string(sk_path_t* cpath, const char* str);
SK_C_API void sk_path_quad_to(sk_path_t *cpath, float x0, float y0, float x1, float y1);
SK_C_API void sk_path_rarc_to(sk_path_t *cpath, float rx, float ry, float xAxisRotate, sk_path_arc_size_t largeArc, sk_path_direction_t sweep, float x, float y);
SK_C_API void sk_path_rconic_to(sk_path_t *cpath, float dx0, float dy0, float dx1, float dy1, float w);
SK_C_API void sk_path_rcubic_to(sk_path_t *cpath, float dx0, float dy0, float dx1, float dy1, float dx2, float dy2);
SK_C_API void sk_path_reset(sk_path_t* cpath);
SK_C_API void sk_path_rewind(sk_path_t* cpath);
SK_C_API void sk_path_rline_to(sk_path_t *cpath, float dx, float yd);
SK_C_API void sk_path_rmove_to(sk_path_t *cpath, float dx, float dy);
SK_C_API void sk_path_set_filltype(sk_path_t* cpath, sk_path_fill_type_t cfilltype);
SK_C_API sk_string_t* sk_path_to_svg_string(const sk_path_t* cpath, bool absolute);
SK_C_API void sk_path_transform(sk_path_t* cpath, const sk_matrix_t* cmatrix);
SK_C_API void sk_path_transform_to_dest(const sk_path_t* cpath, const sk_matrix_t* cmatrix, sk_path_t* destination);

// ===== Functions from include/core/SkPathEffect.h =====
SK_C_API sk_path_effect_t* sk_path_effect_create_1d_path(const sk_path_t* path, float advance, float phase, sk_path_effect_1d_style_t style);
SK_C_API sk_path_effect_t* sk_path_effect_create_2d_line(float width, const sk_matrix_t* matrix);
SK_C_API sk_path_effect_t* sk_path_effect_create_2d_path(const sk_matrix_t* matrix, const sk_path_t* path);
SK_C_API sk_path_effect_t* sk_path_effect_create_compose(sk_path_effect_t* outer, sk_path_effect_t* inner);
SK_C_API sk_path_effect_t* sk_path_effect_create_corner(float radius);
SK_C_API sk_path_effect_t* sk_path_effect_create_dash(const float intervals[], int count, float phase);
SK_C_API sk_path_effect_t* sk_path_effect_create_discrete(float segLength, float deviation, uint32_t seedAssist /*0*/);
SK_C_API sk_path_effect_t* sk_path_effect_create_sum(sk_path_effect_t* first, sk_path_effect_t* second);
SK_C_API sk_path_effect_t* sk_path_effect_create_trim(float start, float stop, sk_path_effect_trim_mode_t mode);
SK_C_API void sk_path_effect_unref(sk_path_effect_t* effect);

// ===== Functions from include/pathops/SkPathOps.h =====
SK_C_API bool sk_path_op(const sk_path_t* path, const sk_path_t* other, sk_path_op_t op, sk_path_t *result);
SK_C_API bool sk_path_simplify(const sk_path_t* path, sk_path_t *result);
SK_C_API void sk_opbuilder_add(sk_op_builder_t* builder, const sk_path_t* path, sk_path_op_t op);
SK_C_API void sk_opbuilder_destroy(sk_op_builder_t* builder);
SK_C_API sk_op_builder_t* sk_opbuilder_new(void);
SK_C_API bool sk_opbuilder_resolve(sk_op_builder_t* builder, sk_path_t* result);

// ===== Functions from include/core/SkShader.h =====
SK_C_API sk_shader_t* sk_shader_new_blend(sk_blend_mode_t mode, const sk_shader_t* dst, const sk_shader_t* src);
SK_C_API sk_shader_t* sk_shader_new_color(sk_color_t color);
SK_C_API sk_shader_t* sk_shader_new_linear_gradient(const sk_point_t points[2], const sk_color_t colors[], const float colorPos[], int colorCount, sk_tile_mode_t tileMode, const sk_matrix_t* localMatrix);
SK_C_API sk_shader_t* sk_shader_new_perlin_noise_fractal_noise(float baseFrequencyX, float baseFrequencyY, int numOctaves, float seed, const sk_isize_t* tileSize);
SK_C_API sk_shader_t* sk_shader_new_perlin_noise_turbulence(float baseFrequencyX, float baseFrequencyY, int numOctaves, float seed, const sk_isize_t* tileSize);
SK_C_API sk_shader_t* sk_shader_new_radial_gradient(const sk_point_t* center, float radius, const sk_color_t colors[], const float colorPos[], int colorCount, sk_tile_mode_t tileMode, const sk_matrix_t* localMatrix);
SK_C_API sk_shader_t* sk_shader_new_sweep_gradient(const sk_point_t* center, const sk_color_t colors[], const float colorPos[], int colorCount, sk_tile_mode_t tileMode, float startAngle, float endAngle, const sk_matrix_t* localMatrix);
SK_C_API sk_shader_t* sk_shader_new_two_point_conical_gradient(const sk_point_t* start, float startRadius, const sk_point_t* end, float endRadius, const sk_color_t colors[], const float colorPos[], int colorCount, sk_tile_mode_t tileMode, const sk_matrix_t* localMatrix);
SK_C_API void sk_shader_unref(sk_shader_t* shader);
SK_C_API sk_shader_t* sk_shader_with_color_filter(const sk_shader_t* shader, const sk_color_filter_t* filter);
SK_C_API sk_shader_t* sk_shader_with_local_matrix(const sk_shader_t* shader, const sk_matrix_t* localMatrix);

// ===== Functions from include/core/SkString.h =====
SK_C_API sk_string_t* sk_string_new(const char* text, size_t len);
SK_C_API sk_string_t* sk_string_new_empty(void);
SK_C_API void sk_string_delete(const sk_string_t* str);
SK_C_API const char* sk_string_get_c_str(const sk_string_t* str);
SK_C_API size_t sk_string_get_size(const sk_string_t* str);

// ===== Functions from include/core/SkSurface.h =====
SK_C_API sk_surface_t* sk_surface_make_raster_direct(const sk_image_info_t *imageInfo, void *pixels, size_t rowBytes, sk_surface_props_t* surfaceProps);
SK_C_API sk_surface_t* sk_surface_make_raster_n32_premul(const sk_image_info_t *imageInfo, sk_surface_props_t* surfaceProps);
SK_C_API sk_surface_t* sk_surface_make_surface(sk_surface_t *surface, int width, int height);
SK_C_API sk_image_t* sk_surface_make_image_snapshot(sk_surface_t* surface);
SK_C_API sk_canvas_t* sk_surface_get_canvas(sk_surface_t* surface);
SK_C_API sk_surface_t* sk_surface_new_backend_render_target(gr_direct_context_t* context, const gr_backendrendertarget_t* target, gr_surface_origin_t origin, sk_color_type_t colorType, sk_color_space_t* colorspace, const sk_surface_props_t* props);
SK_C_API void sk_surface_unref(sk_surface_t* surface);

// ===== Functions from include/core/SkSurfaceProps.h =====
SK_C_API sk_surface_props_t* sk_surfaceprops_new(uint32_t flags, sk_pixel_geometry_t geometry);
SK_C_API void sk_surfaceprops_delete(sk_surface_props_t *surface_props);

// ===== Functions from include/core/SkTextBlob.h =====
SK_C_API const sk_text_blob_builder_run_buffer_t* sk_textblob_builder_alloc_run(sk_text_blob_builder_t* builder, const sk_font_t* font, int count, float x, float y, const sk_rect_t* bounds);
SK_C_API const sk_text_blob_builder_run_buffer_t* sk_textblob_builder_alloc_run_pos(sk_text_blob_builder_t* builder, const sk_font_t* font, int count, const sk_rect_t* bounds);
SK_C_API const sk_text_blob_builder_run_buffer_t* sk_textblob_builder_alloc_run_pos_h(sk_text_blob_builder_t* builder, const sk_font_t* font, int count, float y, const sk_rect_t* bounds);
SK_C_API void sk_textblob_builder_delete(sk_text_blob_builder_t* builder);
SK_C_API sk_text_blob_t* sk_textblob_builder_make(sk_text_blob_builder_t* builder);
SK_C_API sk_text_blob_builder_t* sk_textblob_builder_new(void);

SK_C_API void sk_textblob_get_bounds(const sk_text_blob_t* blob, sk_rect_t* bounds);
SK_C_API int sk_textblob_get_intercepts(const sk_text_blob_t* blob, const float bounds[2], float intervals[], const sk_paint_t* paint);
SK_C_API sk_text_blob_t* sk_textblob_make_from_text(const void* text, size_t byteLength, const sk_font_t* font, sk_text_encoding_t encoding);
SK_C_API void sk_textblob_unref(const sk_text_blob_t* blob);

// ===== Functions from include/core/SkTypeface.h =====
SK_C_API sk_string_t* sk_typeface_get_family_name(const sk_typeface_t* typeface);
SK_C_API sk_font_style_t* sk_typeface_get_fontstyle(const sk_typeface_t* typeface);
SK_C_API int sk_typeface_get_units_per_em(const sk_typeface_t* typeface);
SK_C_API bool sk_typeface_is_fixed_pitch(const sk_typeface_t* typeface);
SK_C_API void sk_typeface_unref(sk_typeface_t* typeface);

// ===== Functions from include/core/SkStream.h =====

SK_C_API sk_dynamic_memory_wstream_t* sk_dynamic_memory_wstream_new(void);
SK_C_API sk_wstream_t* sk_dynamic_memory_wstream_as_wstream(sk_dynamic_memory_wstream_t* stream);
SK_C_API bool sk_dynamic_memory_wstream_write(sk_dynamic_memory_wstream_t* stream, const void *buffer, size_t size);
SK_C_API size_t sk_dynamic_memory_wstream_bytes_written(sk_dynamic_memory_wstream_t* stream);
SK_C_API size_t sk_dynamic_memory_wstream_read(sk_dynamic_memory_wstream_t* stream, void *buffer, size_t offset, size_t size);
SK_C_API void sk_dynamic_memory_wstream_delete(sk_dynamic_memory_wstream_t* stream);

SK_C_API sk_file_wstream_t* sk_file_wstream_new(const char* path);
SK_C_API sk_wstream_t* sk_file_wstream_as_wstream(sk_file_wstream_t* stream);
SK_C_API bool sk_file_wstream_write(sk_file_wstream_t* stream, const void *buffer, size_t size);
SK_C_API size_t sk_file_wstream_bytes_written(sk_file_wstream_t* stream);
SK_C_API void sk_file_wstream_flush(sk_file_wstream_t* stream);
SK_C_API void sk_file_wstream_delete(sk_file_wstream_t* stream);

// ===== Functions from include/core/SKDocument.h =====
SK_C_API sk_canvas_t* sk_document_begin_page(sk_document_t* doc, float width, float height);
SK_C_API void sk_document_end_page(sk_document_t* doc);
SK_C_API void sk_document_close(sk_document_t* doc);
SK_C_API void sk_document_abort(sk_document_t* doc);

// ===== Functions from include/docs/SkPDFDocument.h =====
SK_C_API sk_document_t* sk_document_make_pdf(sk_wstream_t* stream, sk_metadata_t* metadata);

// ===== Functions from include/codec/SkCodec.h =====
SK_C_API void register_image_codecs();

#ifdef __cplusplus
}
#endif // __cplusplus

#endif // SKIA_DEFINED