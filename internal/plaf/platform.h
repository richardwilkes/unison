#pragma once

#ifdef __cplusplus
extern "C" {
#endif

#include <stdlib.h>
#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>

#if !defined(APIENTRY)
	#if defined(PLAF_WINDOWS)
		#define APIENTRY __stdcall
	#else
		#define APIENTRY
	#endif
#endif

#if defined(PLAF_WINDOWS)
	#if !defined(WINGDIAPI)
		#define WINGDIAPI __declspec(dllimport)
		#define GLFW_WINGDIAPI_DEFINED
	#endif
	#if !defined(CALLBACK)
		#define CALLBACK __stdcall
		#define GLFW_CALLBACK_DEFINED
	#endif
#endif

// Input constants
#define INPUT_RELEASE 0 // The key or button was released.
#define INPUT_PRESS   1 // The key or button was pressed.
#define INPUT_REPEAT  2 // The key was held down until it repeated.

// Key codes
#define KEY_UNKNOWN       -1
#define KEY_SPACE         32
#define KEY_APOSTROPHE    39
#define KEY_COMMA         44
#define KEY_MINUS         45
#define KEY_PERIOD        46
#define KEY_SLASH         47
#define KEY_0             48
#define KEY_1             49
#define KEY_2             50
#define KEY_3             51
#define KEY_4             52
#define KEY_5             53
#define KEY_6             54
#define KEY_7             55
#define KEY_8             56
#define KEY_9             57
#define KEY_SEMICOLON     59
#define KEY_EQUAL         61
#define KEY_A             65
#define KEY_B             66
#define KEY_C             67
#define KEY_D             68
#define KEY_E             69
#define KEY_F             70
#define KEY_G             71
#define KEY_H             72
#define KEY_I             73
#define KEY_J             74
#define KEY_K             75
#define KEY_L             76
#define KEY_M             77
#define KEY_N             78
#define KEY_O             79
#define KEY_P             80
#define KEY_Q             81
#define KEY_R             82
#define KEY_S             83
#define KEY_T             84
#define KEY_U             85
#define KEY_V             86
#define KEY_W             87
#define KEY_X             88
#define KEY_Y             89
#define KEY_Z             90
#define KEY_LEFT_BRACKET  91
#define KEY_BACKSLASH     92
#define KEY_RIGHT_BRACKET 93
#define KEY_GRAVE_ACCENT  96
#define KEY_WORLD_1       161
#define KEY_WORLD_2       162
#define KEY_ESCAPE        256
#define KEY_ENTER         257
#define KEY_TAB           258
#define KEY_BACKSPACE     259
#define KEY_INSERT        260
#define KEY_DELETE        261
#define KEY_RIGHT         262
#define KEY_LEFT          263
#define KEY_DOWN          264
#define KEY_UP            265
#define KEY_PAGE_UP       266
#define KEY_PAGE_DOWN     267
#define KEY_HOME          268
#define KEY_END           269
#define KEY_CAPS_LOCK     280
#define KEY_SCROLL_LOCK   281
#define KEY_NUM_LOCK      282
#define KEY_PRINT_SCREEN  283
#define KEY_PAUSE         284
#define KEY_F1            290
#define KEY_F2            291
#define KEY_F3            292
#define KEY_F4            293
#define KEY_F5            294
#define KEY_F6            295
#define KEY_F7            296
#define KEY_F8            297
#define KEY_F9            298
#define KEY_F10           299
#define KEY_F11           300
#define KEY_F12           301
#define KEY_F13           302
#define KEY_F14           303
#define KEY_F15           304
#define KEY_F16           305
#define KEY_F17           306
#define KEY_F18           307
#define KEY_F19           308
#define KEY_F20           309
#define KEY_F21           310
#define KEY_F22           311
#define KEY_F23           312
#define KEY_F24           313
#define KEY_F25           314
#define KEY_KP_0          320
#define KEY_KP_1          321
#define KEY_KP_2          322
#define KEY_KP_3          323
#define KEY_KP_4          324
#define KEY_KP_5          325
#define KEY_KP_6          326
#define KEY_KP_7          327
#define KEY_KP_8          328
#define KEY_KP_9          329
#define KEY_KP_DECIMAL    330
#define KEY_KP_DIVIDE     331
#define KEY_KP_MULTIPLY   332
#define KEY_KP_SUBTRACT   333
#define KEY_KP_ADD        334
#define KEY_KP_ENTER      335
#define KEY_KP_EQUAL      336
#define KEY_LEFT_SHIFT    340
#define KEY_LEFT_CONTROL  341
#define KEY_LEFT_ALT      342
#define KEY_LEFT_SUPER    343
#define KEY_RIGHT_SHIFT   344
#define KEY_RIGHT_CONTROL 345
#define KEY_RIGHT_ALT     346
#define KEY_RIGHT_SUPER   347
#define KEY_MENU          348
#define KEY_LAST          KEY_MENU

// Modifier key flags
#define MOD_SHIFT     0x0001
#define MOD_CONTROL   0x0002
#define MOD_ALT       0x0004
#define MOD_SUPER     0x0008
#define MOD_CAPS_LOCK 0x0010
#define MOD_NUM_LOCK  0x0020

// Mouse button IDs
#define MOUSE_BUTTON_1         0
#define MOUSE_BUTTON_2         1
#define MOUSE_BUTTON_3         2
#define MOUSE_BUTTON_4         3
#define MOUSE_BUTTON_5         4
#define MOUSE_BUTTON_6         5
#define MOUSE_BUTTON_7         6
#define MOUSE_BUTTON_8         7
#define MOUSE_BUTTON_LAST      MOUSE_BUTTON_8
#define MOUSE_BUTTON_LEFT      MOUSE_BUTTON_1
#define MOUSE_BUTTON_RIGHT     MOUSE_BUTTON_2
#define MOUSE_BUTTON_MIDDLE    MOUSE_BUTTON_3

// Peripheral connection status codes
#define CONNECTED    0x00040001
#define DISCONNECTED 0x00040002

// Window attributes and/or hints
#define WINDOW_ATTR_HINT_FOCUSED                   0x00020001
#define WINDOW_ATTR_ICONIFIED                      0x00020002
#define WINDOW_ATTR_HINT_RESIZABLE                 0x00020003
#define WINDOW_ATTR_HINT_VISIBLE                   0x00020004
#define WINDOW_ATTR_HINT_DECORATED                 0x00020005
#define WINDOW_ATTR_HINT_AUTO_ICONIFY              0x00020006
#define WINDOW_ATTR_HINT_FLOATING                  0x00020007
#define WINDOW_ATTR_HINT_MAXIMIZED                 0x00020008
#define WINDOW_HINT_CENTER_CURSOR                  0x00020009
#define WINDOW_ATTR_HINT_TRANSPARENT_FRAMEBUFFER   0x0002000A
#define WINDOW_ATTR_HOVERED                        0x0002000B
#define WINDOW_ATTR_HINT_FOCUS_ON_SHOW             0x0002000C
#define WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH         0x0002000D
#define WINDOW_HINT_POSITION_X                     0x0002000E
#define WINDOW_HINT_POSITION_Y                     0x0002000F
#define WINDOW_HINT_RED_BITS                       0x00021001
#define WINDOW_HINT_GREEN_BITS                     0x00021002
#define WINDOW_HINT_BLUE_BITS                      0x00021003
#define WINDOW_HINT_ALPHA_BITS                     0x00021004
#define WINDOW_HINT_DEPTH_BITS                     0x00021005
#define WINDOW_HINT_STENCIL_BITS                   0x00021006
#define WINDOW_HINT_ACCUM_RED_BITS                 0x00021007
#define WINDOW_HINT_ACCUM_GREEN_BITS               0x00021008
#define WINDOW_HINT_ACCUM_BLUE_BITS                0x00021009
#define WINDOW_HINT_ACCUM_ALPHA_BITS               0x0002100A
#define WINDOW_HINT_AUX_BUFFERS                    0x0002100B
#define WINDOW_HINT_SAMPLES                        0x0002100D
#define WINDOW_HINT_SRGB_CAPABLE                   0x0002100E
#define WINDOW_HINT_REFRESH_RATE                   0x0002100F
#define WINDOW_ATTR_HINT_DOUBLE_BUFFER             0x00021010
#define WINDOW_ATTR_HINT_CONTEXT_VERSION_MAJOR     0x00022002
#define WINDOW_ATTR_HINT_CONTEXT_VERSION_MINOR     0x00022003
#define WINDOW_ATTR_CONTEXT_REVISION               0x00022004
#define WINDOW_ATTR_HINT_CONTEXT_ROBUSTNESS        0x00022005
#define WINDOW_ATTR_HINT_OPENGL_FORWARD_COMPAT     0x00022006
#define WINDOW_ATTR_HINT_CONTEXT_DEBUG             0x00022007
#define WINDOW_ATTR_HINT_OPENGL_PROFILE            0x00022008
#define WINDOW_ATTR_HINT_CONTEXT_RELEASE_BEHAVIOR  0x00022009
#define WINDOW_ATTR_HINT_CONTEXT_ERROR_SUPPRESSION 0x0002200A
#define WINDOW_HINT_SCALE_TO_MONITOR               0x0002200C
#define WINDOW_HINT_SCALE_FRAMEBUFFER              0x0002200D
#define WINDOW_HINT_COCOA_FRAME_NAME               0x00023002
#define WINDOW_HINT_COCOA_GRAPHICS_SWITCHING       0x00023003
#define WINDOW_HINT_X11_CLASS_NAME                 0x00024001
#define WINDOW_HINT_X11_INSTANCE_NAME              0x00024002
#define WINDOW_HINT_WIN32_KEYBOARD_MENU            0x00025001
#define WINDOW_HINT_WIN32_SHOWDEFAULT              0x00025002

// Context robustness values
#define CONTEXT_ROBUSTNESS_NONE                   0
#define CONTEXT_ROBUSTNESS_NO_RESET_NOTIFICATION  0x00031001
#define CONTEXT_ROBUSTNESS_LOSE_CONTEXT_ON_RESET  0x00031002

// OpenGL profile values
#define OPENGL_PROFILE_ANY     0
#define OPENGL_PROFILE_CORE    0x00032001
#define OPENGL_PROFILE_COMPAT  0x00032002

// Context release behavior values
#define RELEASE_BEHAVIOR_ANY   0
#define RELEASE_BEHAVIOR_FLUSH 0x00035001
#define RELEASE_BEHAVIOR_NONE  0x00035002

// Input modes
#define INPUT_MODE_CURSOR                  0x00033001
#define INPUT_MODE_STICKY_KEYS             0x00033002
#define INPUT_MODE_STICKY_MOUSE_BUTTONS    0x00033003
#define INPUT_MODE_LOCK_KEY_MODS           0x00033004
#define INPUT_MODE_RAW_MOUSE_MOTION        0x00033005
#define INPUT_MODE_UNLIMITED_MOUSE_BUTTONS 0x00033006

// Cursor mode values
#define CURSOR_NORMAL   0x00034001
#define CURSOR_HIDDEN   0x00034002
#define CURSOR_DISABLED 0x00034003
#define CURSOR_CAPTURED 0x00034004

// Standard cursor IDs
#define STD_CURSOR_ARROW             0x00036001
#define STD_CURSOR_IBEAM             0x00036002
#define STD_CURSOR_CROSSHAIR         0x00036003
#define STD_CURSOR_POINTING_HAND     0x00036004
#define STD_CURSOR_HORIZONTAL_RESIZE 0x00036005
#define STD_CURSOR_VERTICAL_RESIZE   0x00036006

#define ANY_POSITION 0x80000000

#define DONT_CARE    -1

#define ERROR_MSG_SIZE 1024



// Error codes
/*! @brief GLFW has not been initialized.
 *
 *  This occurs if a GLFW function was called that must not be called unless the
 *  library is [initialized](@ref intro_init).
 *
 *  @analysis Application programmer error.  Initialize GLFW before calling any
 *  function that requires initialization.
 */
#define ERR_NOT_INITIALIZED        0x00010001
/*! @brief No context is current for this thread.
 *
 *  This occurs if a GLFW function was called that needs and operates on the
 *  current OpenGL or OpenGL ES context but no context is current on the calling
 *  thread.  One such function is @ref glfwSwapInterval.
 *
 *  @analysis Application programmer error.  Ensure a context is current before
 *  calling functions that require a current context.
 */
#define ERR_NO_CURRENT_CONTEXT     0x00010002
/*! @brief One of the arguments to the function was an invalid enum value.
 *
 *  One of the arguments to the function was an invalid enum value, for example
 *  requesting @ref WINDOW_HINT_RED_BITS with @ref glfwGetWindowAttrib.
 *
 *  @analysis Application programmer error.  Fix the offending call.
 */
#define ERR_INVALID_ENUM           0x00010003
/*! @brief One of the arguments to the function was an invalid value.
 *
 *  One of the arguments to the function was an invalid value, for example
 *  requesting a non-existent OpenGL or OpenGL ES version like 2.7.
 *
 *  Requesting a valid but unavailable OpenGL or OpenGL ES version will instead
 *  result in a @ref ERR_VERSION_UNAVAILABLE error.
 *
 *  @analysis Application programmer error.  Fix the offending call.
 */
#define ERR_INVALID_VALUE          0x00010004
/*! @brief A memory allocation failed.
 *
 *  A memory allocation failed.
 *
 *  @analysis A bug in GLFW or the underlying operating system.  Report the bug
 *  to our [issue tracker](https://github.com/glfw/glfw/issues).
 */
#define ERR_OUT_OF_MEMORY          0x00010005
/*! @brief GLFW could not find support for the requested API on the system.
 *
 *  GLFW could not find support for the requested API on the system.
 *
 *  @analysis The installed graphics driver does not support the requested
 *  API, or does not support it via the chosen context creation API.
 *  Below are a few examples.
 *
 *  @par
 *  Some pre-installed Windows graphics drivers do not support OpenGL.  AMD only
 *  supports OpenGL ES via EGL, while Nvidia and Intel only support it via
 *  a WGL or GLX extension.  macOS does not provide OpenGL ES at all.  The Mesa
 *  EGL, OpenGL and OpenGL ES libraries do not interface with the Nvidia binary
 *  driver.
 */
#define ERR_API_UNAVAILABLE        0x00010006
/*! @brief The requested OpenGL or OpenGL ES version is not available.
 *
 *  The requested OpenGL or OpenGL ES version (including any requested context
 *  or framebuffer hints) is not available on this machine.
 *
 *  @analysis The machine does not support your requirements.  If your
 *  application is sufficiently flexible, downgrade your requirements and try
 *  again.  Otherwise, inform the user that their machine does not match your
 *  requirements.
 *
 *  @par
 *  Future invalid OpenGL and OpenGL ES versions, for example OpenGL 4.8 if 5.0
 *  comes out before the 4.x series gets that far, also fail with this error and
 *  not @ref ERR_INVALID_VALUE, because GLFW cannot know what future versions
 *  will exist.
 */
#define ERR_VERSION_UNAVAILABLE    0x00010007
/*! @brief A platform-specific error occurred that does not match any of the
 *  more specific categories.
 *
 *  A platform-specific error occurred that does not match any of the more
 *  specific categories.
 *
 *  @analysis A bug or configuration error in GLFW, the underlying operating
 *  system or its drivers, or a lack of required resources.  Report the issue to
 *  our [issue tracker](https://github.com/glfw/glfw/issues).
 */
#define ERR_PLATFORM_ERROR         0x00010008
/*! @brief The requested format is not supported or available.
 *
 *  If emitted during window creation, the requested pixel format is not
 *  supported.
 *
 *  If emitted when querying the clipboard, the contents of the clipboard could
 *  not be converted to the requested format.
 *
 *  @analysis If emitted during window creation, one or more
 *  [hard constraints](@ref window_hints_hard) did not match any of the
 *  available pixel formats.  If your application is sufficiently flexible,
 *  downgrade your requirements and try again.  Otherwise, inform the user that
 *  their machine does not match your requirements.
 *
 *  @par
 *  If emitted when querying the clipboard, ignore the error or report it to
 *  the user, as appropriate.
 */
#define ERR_FORMAT_UNAVAILABLE     0x00010009
/*! @brief The specified window does not have an OpenGL or OpenGL ES context.
 *
 *  A window that does not have an OpenGL or OpenGL ES context was passed to
 *  a function that requires it to have one.
 *
 *  @analysis Application programmer error.  Fix the offending call.
 */
#define ERR_NO_WINDOW_CONTEXT      0x0001000A
/*! @brief The specified cursor shape is not available.
 *
 *  The specified standard cursor shape is not available, either because the
 *  current platform cursor theme does not provide it or because it is not
 *  available on the platform.
 *
 *  @analysis Platform or system settings limitation.  Pick another
 *  [standard cursor shape](@ref shapes) or create a
 *  [custom cursor](@ref cursor_custom).
 */
#define GLFW_CURSOR_UNAVAILABLE     0x0001000B
/*! @brief The requested feature is not provided by the platform.
 *
 *  The requested feature is not provided by the platform, so GLFW is unable to
 *  implement it.  The documentation for each function notes if it could emit
 *  this error.
 *
 *  @analysis Platform or platform version limitation.  The error can be ignored
 *  unless the feature is critical to the application.
 *
 *  @par
 *  A function call that emits this error has no effect other than the error and
 *  updating any existing out parameters.
 */
#define ERR_FEATURE_UNAVAILABLE    0x0001000C
/*! @brief The requested feature is not implemented for the platform.
 *
 *  The requested feature has not yet been implemented in GLFW for this platform.
 *
 *  @analysis An incomplete implementation of GLFW for this platform, hopefully
 *  fixed in a future release.  The error can be ignored unless the feature is
 *  critical to the application.
 *
 *  @par
 *  A function call that emits this error has no effect other than the error and
 *  updating any existing out parameters.
 */
#define ERR_FEATURE_UNIMPLEMENTED  0x0001000D
/*! @brief Platform unavailable or no matching platform was found.
 *
 *  If emitted during initialization, no matching platform was found.
 *
 *  If emitted by a native access function, GLFW was initialized for a different platform
 *  than the function is for.
 *
 *  @analysis Failure to detect any platform usually only happens on non-macOS Unix
 *  systems, either when no window system is running or the program was run from
 *  a terminal that does not have the necessary environment variables.  Fall back to
 *  a different platform if possible or notify the user that no usable platform was
 *  detected.
 *
 *  Failure to detect a specific platform may have the same cause as above or be because
 *  support for that platform was not compiled in.
 */
#define ERR_PLATFORM_UNAVAILABLE   0x0001000E

// Forward declarations
typedef struct GLFWcursor GLFWcursor;
typedef struct GLFWmonitor GLFWmonitor;
typedef struct GLFWwindow GLFWwindow;

// Function pointer definitions
typedef void (*charFunc)(GLFWwindow* window, unsigned int codepoint);
typedef void (*charModsFunc)(GLFWwindow* window, unsigned int codepoint, int mods);
typedef void (*cursorEnterFunc)(GLFWwindow* window, int entered);
typedef void (*cursorPosFunc)(GLFWwindow* window, double xpos, double ypos);
typedef void (*dropFunc)(GLFWwindow* window, int path_count, const char* paths[]);
typedef void (*errorFunc)(int error_code, const char* description);
typedef void (*frameBufferSizeFunc)(GLFWwindow* window, int width, int height);
typedef void (*glFunc)(void);
typedef void (*keyFunc)(GLFWwindow* window, int key, int scancode, int action, int mods);
typedef void (*monitorFunc)(GLFWmonitor* monitor, int event);
typedef void (*mouseButtonFunc)(GLFWwindow* window, int button, int action, int mods);
typedef void (*scrollFunc)(GLFWwindow* window, double xoffset, double yoffset);
typedef void (*windowCloseFunc)(GLFWwindow* window);
typedef void (*windowContextScaleFunc)(GLFWwindow* window, float xscale, float yscale);
typedef void (*windowFocusFunc)(GLFWwindow* window, int focused);
typedef void (*windowIconifyFunc)(GLFWwindow* window, int iconified);
typedef void (*windowMaximizeFunc)(GLFWwindow* window, int maximized);
typedef void (*windowPosFunc)(GLFWwindow* window, int xpos, int ypos); // coordinates are content area upper-left
typedef void (*windowRefreshFunc)(GLFWwindow* window);
typedef void (*windowSizeFunc)(GLFWwindow* window, int width, int height);

// An error response
typedef struct ErrorResponse {
    int  code;
    char desc[ERROR_MSG_SIZE];
} ErrorResponse;

// A single video mode
typedef struct VideoMode {
    int width;
    int height;
    int redBits;
    int greenBits;
    int blueBits;
    int refreshRate;
} VideoMode;

// Gamma ramp for a monitor
typedef struct GammaRamp {
    unsigned short* red;
    unsigned short* green;
    unsigned short* blue;
    unsigned int    size;
} GammaRamp;

// Image data
typedef struct ImageData {
    int            width;
    int            height;
    unsigned char* pixels;
} ImageData;


/*************************************************************************
 * Global definition cleanup
 *************************************************************************/

/* ------------------- BEGIN SYSTEM/COMPILER SPECIFIC -------------------- */

#ifdef GLFW_WINGDIAPI_DEFINED
 #undef WINGDIAPI
 #undef GLFW_WINGDIAPI_DEFINED
#endif

#ifdef GLFW_CALLBACK_DEFINED
 #undef CALLBACK
 #undef GLFW_CALLBACK_DEFINED
#endif

/* Some OpenGL related headers need GLAPIENTRY, but it is unconditionally
 * defined by some gl.h variants (OpenBSD) so define it after if needed.
 */
#ifndef GLAPIENTRY
 #define GLAPIENTRY APIENTRY
 #define GLFW_GLAPIENTRY_DEFINED
#endif

/* -------------------- END SYSTEM/COMPILER SPECIFIC --------------------- */


/* ------------------------- Internal ----------------------- */

#define _GLFW_INSERT_FIRST      0
#define _GLFW_INSERT_LAST       1

typedef int GLFWbool;
typedef void (*moduleFunc)(void);

typedef struct _GLFWwndconfig   _GLFWwndconfig;
typedef struct _GLFWctxconfig   _GLFWctxconfig;
typedef struct _GLFWfbconfig    _GLFWfbconfig;
typedef struct _GLFWcontext     _GLFWcontext;
typedef struct _GLFWwindow      _GLFWwindow;
typedef struct _GLFWplatform    _GLFWplatform;
typedef struct _GLFWlibrary     _GLFWlibrary;
typedef struct _GLFWmonitor     _GLFWmonitor;
typedef struct _GLFWcursor      _GLFWcursor;
typedef struct _GLFWmapelement  _GLFWmapelement;
typedef struct _GLFWtls         _GLFWtls;
typedef struct _GLFWmutex       _GLFWmutex;

#define GL_VERSION 0x1f02
#define GL_NONE 0
#define GL_COLOR_BUFFER_BIT 0x00004000
#define GL_UNSIGNED_BYTE 0x1401
#define GL_EXTENSIONS 0x1f03
#define GL_NUM_EXTENSIONS 0x821d
#define GL_CONTEXT_FLAGS 0x821e
#define GL_CONTEXT_FLAG_FORWARD_COMPATIBLE_BIT 0x00000001
#define GL_CONTEXT_FLAG_DEBUG_BIT 0x00000002
#define GL_CONTEXT_PROFILE_MASK 0x9126
#define GL_CONTEXT_COMPATIBILITY_PROFILE_BIT 0x00000002
#define GL_CONTEXT_CORE_PROFILE_BIT 0x00000001
#define GL_RESET_NOTIFICATION_STRATEGY_ARB 0x8256
#define GL_LOSE_CONTEXT_ON_RESET_ARB 0x8252
#define GL_NO_RESET_NOTIFICATION_ARB 0x8261
#define GL_CONTEXT_RELEASE_BEHAVIOR 0x82fb
#define GL_CONTEXT_RELEASE_BEHAVIOR_FLUSH 0x82fc
#define GL_CONTEXT_FLAG_NO_ERROR_BIT_KHR 0x00000008

typedef int GLint;
typedef unsigned int GLuint;
typedef unsigned int GLenum;
typedef unsigned int GLbitfield;
typedef unsigned char GLubyte;

typedef void (APIENTRY * PFNGLCLEARPROC)(GLbitfield);
typedef const GLubyte* (APIENTRY * PFNGLGETSTRINGPROC)(GLenum);
typedef void (APIENTRY * PFNGLGETINTEGERVPROC)(GLenum,GLint*);
typedef const GLubyte* (APIENTRY * PFNGLGETSTRINGIPROC)(GLenum,GLuint);

#if defined(GLFW_BUILD_WIN32_MODULE) || \
    defined(GLFW_BUILD_POSIX_MODULE)
 #error "You must not define these; define zero or more _GLFW_<platform> macros instead"
#endif

#if defined(PLAF_WINDOWS)
 #include "platform_windows.h"
#else
 #define GLFW_WIN32_WINDOW_STATE
 #define GLFW_WIN32_MONITOR_STATE
 #define GLFW_WIN32_CURSOR_STATE
 #define GLFW_WIN32_LIBRARY_WINDOW_STATE
 #define GLFW_WGL_CONTEXT_STATE
 #define GLFW_WGL_LIBRARY_CONTEXT_STATE
#endif

#if defined(PLAF_DARWIN)
 #include "platform_darwin.h"
#else
 #define GLFW_COCOA_WINDOW_STATE
 #define GLFW_COCOA_MONITOR_STATE
 #define GLFW_COCOA_CURSOR_STATE
 #define GLFW_COCOA_LIBRARY_WINDOW_STATE
 #define GLFW_NSGL_CONTEXT_STATE
 #define GLFW_NSGL_LIBRARY_CONTEXT_STATE
#endif

#if defined(PLAF_LINUX)
 #include "platform_linux.h"
#else
 #define GLFW_X11_WINDOW_STATE
 #define GLFW_X11_MONITOR_STATE
 #define GLFW_X11_CURSOR_STATE
 #define GLFW_X11_LIBRARY_WINDOW_STATE
 #define GLFW_GLX_CONTEXT_STATE
 #define GLFW_GLX_LIBRARY_CONTEXT_STATE
#endif

#define GLFW_PLATFORM_WINDOW_STATE \
        GLFW_WIN32_WINDOW_STATE \
        GLFW_COCOA_WINDOW_STATE \
        GLFW_X11_WINDOW_STATE

#define GLFW_PLATFORM_MONITOR_STATE \
        GLFW_WIN32_MONITOR_STATE \
        GLFW_COCOA_MONITOR_STATE \
        GLFW_X11_MONITOR_STATE

#define GLFW_PLATFORM_CURSOR_STATE \
        GLFW_WIN32_CURSOR_STATE \
        GLFW_COCOA_CURSOR_STATE \
        GLFW_X11_CURSOR_STATE

#define GLFW_PLATFORM_LIBRARY_WINDOW_STATE \
        GLFW_WIN32_LIBRARY_WINDOW_STATE \
        GLFW_COCOA_LIBRARY_WINDOW_STATE \
        GLFW_X11_LIBRARY_WINDOW_STATE

#define GLFW_PLATFORM_CONTEXT_STATE \
        GLFW_WGL_CONTEXT_STATE \
        GLFW_NSGL_CONTEXT_STATE \
        GLFW_GLX_CONTEXT_STATE

#define GLFW_PLATFORM_LIBRARY_CONTEXT_STATE \
        GLFW_WGL_LIBRARY_CONTEXT_STATE \
        GLFW_NSGL_LIBRARY_CONTEXT_STATE \
        GLFW_GLX_LIBRARY_CONTEXT_STATE

#if defined(PLAF_WINDOWS)
 #define GLFW_BUILD_WIN32_MODULE
#else
 #define GLFW_BUILD_POSIX_MODULE
#endif

// Swaps the provided pointers
#define _GLFW_SWAP(type, x, y) \
    {                          \
        type t;                \
        t = x;                 \
        x = y;                 \
        y = t;                 \
    }

// Window configuration
//
// Parameters relating to the creation of the window but not directly related
// to the framebuffer.  This is used to pass window creation parameters from
// shared code to the platform API.
//
struct _GLFWwndconfig
{
    int           xpos;
    int           ypos;
    int           width;
    int           height;
    GLFWbool      resizable;
    GLFWbool      visible;
    GLFWbool      decorated;
    GLFWbool      focused;
    GLFWbool      autoIconify;
    GLFWbool      floating;
    GLFWbool      maximized;
    GLFWbool      centerCursor;
    GLFWbool      focusOnShow;
    GLFWbool      mousePassthrough;
    GLFWbool      scaleToMonitor;
    GLFWbool      scaleFramebuffer;
    struct {
        char      frameName[256];
    } ns;
    struct {
        char      className[256];
        char      instanceName[256];
    } x11;
    struct {
        GLFWbool  keymenu;
        GLFWbool  showDefault;
    } win32;
    struct {
        char      appId[256];
    } wl;
};

// Context configuration
//
// Parameters relating to the creation of the context but not directly related
// to the framebuffer.  This is used to pass context creation parameters from
// shared code to the platform API.
//
struct _GLFWctxconfig
{
    int           major;
    int           minor;
    GLFWbool      forward;
    GLFWbool      debug;
    GLFWbool      noerror;
    int           profile;
    int           robustness;
    int           release;
    _GLFWwindow*  share;
    struct {
        GLFWbool  offline;
    } nsgl;
};

// Framebuffer configuration
//
// This describes buffers and their sizes.  It also contains
// a platform-specific ID used to map back to the backend API object.
//
// It is used to pass framebuffer parameters from shared code to the platform
// API and also to enumerate and select available framebuffer configs.
//
struct _GLFWfbconfig
{
    int         redBits;
    int         greenBits;
    int         blueBits;
    int         alphaBits;
    int         depthBits;
    int         stencilBits;
    int         accumRedBits;
    int         accumGreenBits;
    int         accumBlueBits;
    int         accumAlphaBits;
    int         auxBuffers;
    int         samples;
    GLFWbool    sRGB;
    GLFWbool    doublebuffer;
    GLFWbool    transparent;
    uintptr_t   handle;
};

// Context structure
//
struct _GLFWcontext
{
    int                 major, minor, revision;
    GLFWbool            forward, debug, noerror;
    int                 profile;
    int                 robustness;
    int                 release;

    PFNGLGETSTRINGIPROC  GetStringi;
    PFNGLGETINTEGERVPROC GetIntegerv;
    PFNGLGETSTRINGPROC   GetString;

    void (*makeCurrent)(_GLFWwindow*);
    void (*swapBuffers)(_GLFWwindow*);
    void (*swapInterval)(int);
    int (*extensionSupported)(const char*);
    glFunc (*getProcAddress)(const char*);
    void (*destroy)(_GLFWwindow*);

    // This is defined in platform.h
    GLFW_PLATFORM_CONTEXT_STATE
};

// Window and context structure
//
struct _GLFWwindow
{
    struct _GLFWwindow* next;

    // Window settings and state
    GLFWbool            resizable;
    GLFWbool            decorated;
    GLFWbool            autoIconify;
    GLFWbool            floating;
    GLFWbool            focusOnShow;
    GLFWbool            mousePassthrough;
    GLFWbool            shouldClose;
    void*               userPointer;
    GLFWbool            doublebuffer;
    VideoMode         videoMode;
    _GLFWmonitor*       monitor;
    _GLFWcursor*        cursor;
    char*               title;

    int                 minwidth, minheight;
    int                 maxwidth, maxheight;
    int                 numer, denom;

    GLFWbool            stickyKeys;
    GLFWbool            stickyMouseButtons;
    GLFWbool            lockKeyMods;
    GLFWbool            disableMouseButtonLimit;
    int                 cursorMode;
    char                mouseButtons[MOUSE_BUTTON_LAST + 1];
    char                keys[KEY_LAST + 1];
    // Virtual cursor position when cursor is disabled
    double              virtualCursorPosX, virtualCursorPosY;
    GLFWbool            rawMouseMotion;

    _GLFWcontext        context;

    struct {
        windowPosFunc          pos;
        windowSizeFunc         size;
        windowCloseFunc        close;
        windowRefreshFunc      refresh;
        windowFocusFunc        focus;
        windowIconifyFunc      iconify;
        windowMaximizeFunc     maximize;
        frameBufferSizeFunc    fbsize;
        windowContextScaleFunc scale;
        mouseButtonFunc        mouseButton;
        cursorPosFunc          cursorPos;
        cursorEnterFunc        cursorEnter;
        scrollFunc             scroll;
        keyFunc                key;
        charFunc               character;
        charModsFunc           charmods;
        dropFunc               drop;
    } callbacks;

    // This is defined in platform.h
    GLFW_PLATFORM_WINDOW_STATE
};

// Monitor structure
//
struct _GLFWmonitor
{
    char            name[128];
    void*           userPointer;

    // Physical dimensions in millimeters.
    int             widthMM, heightMM;

    // The window whose video mode is current on this monitor
    _GLFWwindow*    window;

    VideoMode*    modes;
    int             modeCount;
    VideoMode     currentMode;

    GammaRamp   originalRamp;
    GammaRamp   currentRamp;

    // This is defined in platform.h
    GLFW_PLATFORM_MONITOR_STATE
};

// Cursor structure
//
struct _GLFWcursor
{
    _GLFWcursor*    next;
    // This is defined in platform.h
    GLFW_PLATFORM_CURSOR_STATE
};

// Platform API structure
//
struct _GLFWplatform
{
    // input
    void (*getCursorPos)(_GLFWwindow*,double*,double*);
    void (*setCursorPos)(_GLFWwindow*,double,double);
    void (*setCursorMode)(_GLFWwindow*,int);
    void (*setRawMouseMotion)(_GLFWwindow*,GLFWbool);
    GLFWbool (*rawMouseMotionSupported)(void);
    GLFWbool (*createCursor)(_GLFWcursor*,const ImageData*,int,int);
    GLFWbool (*createStandardCursor)(_GLFWcursor*,int);
    void (*destroyCursor)(_GLFWcursor*);
    void (*setCursor)(_GLFWwindow*,_GLFWcursor*);
    const char* (*getScancodeName)(int);
    int (*getKeyScancode)(int);
    // monitor
    void (*freeMonitor)(_GLFWmonitor*);
    void (*getMonitorPos)(_GLFWmonitor*,int*,int*);
    void (*getMonitorContentScale)(_GLFWmonitor*,float*,float*);
    void (*getMonitorWorkarea)(_GLFWmonitor*,int*,int*,int*,int*);
    VideoMode* (*getVideoModes)(_GLFWmonitor*,int*);
    GLFWbool (*getVideoMode)(_GLFWmonitor*,VideoMode*);
    GLFWbool (*getGammaRamp)(_GLFWmonitor*,GammaRamp*);
    void (*setGammaRamp)(_GLFWmonitor*,const GammaRamp*);
    // window
    GLFWbool (*createWindow)(_GLFWwindow*,const _GLFWwndconfig*,const _GLFWctxconfig*,const _GLFWfbconfig*);
    void (*destroyWindow)(_GLFWwindow*);
    void (*setWindowTitle)(_GLFWwindow*,const char*);
    void (*setWindowIcon)(_GLFWwindow*,int,const ImageData*);
    void (*getWindowPos)(_GLFWwindow*,int*,int*);
    void (*setWindowPos)(_GLFWwindow*,int,int);
    void (*getWindowSize)(_GLFWwindow*,int*,int*);
    void (*setWindowSize)(_GLFWwindow*,int,int);
    void (*setWindowSizeLimits)(_GLFWwindow*,int,int,int,int);
    void (*setWindowAspectRatio)(_GLFWwindow*,int,int);
    void (*getFramebufferSize)(_GLFWwindow*,int*,int*);
    void (*getWindowFrameSize)(_GLFWwindow*,int*,int*,int*,int*);
    void (*getWindowContentScale)(_GLFWwindow*,float*,float*);
    void (*iconifyWindow)(_GLFWwindow*);
    void (*restoreWindow)(_GLFWwindow*);
    void (*maximizeWindow)(_GLFWwindow*);
    void (*showWindow)(_GLFWwindow*);
    void (*hideWindow)(_GLFWwindow*);
    void (*requestWindowAttention)(_GLFWwindow*);
    void (*focusWindow)(_GLFWwindow*);
    void (*setWindowMonitor)(_GLFWwindow*,_GLFWmonitor*,int,int,int,int,int);
    GLFWbool (*windowFocused)(_GLFWwindow*);
    GLFWbool (*windowIconified)(_GLFWwindow*);
    GLFWbool (*windowVisible)(_GLFWwindow*);
    GLFWbool (*windowMaximized)(_GLFWwindow*);
    GLFWbool (*windowHovered)(_GLFWwindow*);
    GLFWbool (*framebufferTransparent)(_GLFWwindow*);
    float (*getWindowOpacity)(_GLFWwindow*);
    void (*setWindowResizable)(_GLFWwindow*,GLFWbool);
    void (*setWindowDecorated)(_GLFWwindow*,GLFWbool);
    void (*setWindowFloating)(_GLFWwindow*,GLFWbool);
    void (*setWindowOpacity)(_GLFWwindow*,float);
    void (*setWindowMousePassthrough)(_GLFWwindow*,GLFWbool);
    void (*pollEvents)(void);
    void (*waitEvents)(void);
    void (*waitEventsTimeout)(double);
    void (*postEmptyEvent)(void);
};

// Library global data
//
struct _GLFWlibrary
{
    GLFWbool            initialized;

    _GLFWplatform       platform;
    char*               clipboardString;

    struct {
        _GLFWfbconfig   framebuffer;
        _GLFWwndconfig  window;
        _GLFWctxconfig  context;
        int             refreshRate;
    } hints;

    _GLFWcursor*        cursorListHead;
    _GLFWwindow*        windowListHead;

    _GLFWmonitor**      monitors;
    int                 monitorCount;

    ErrorResponse       errorSlot;
    _GLFWwindow*        contextSlot;

	struct {
        monitorFunc  monitor;
    } callbacks;

    // These are defined in platform.h
    GLFW_PLATFORM_LIBRARY_WINDOW_STATE
    GLFW_PLATFORM_LIBRARY_CONTEXT_STATE
};

// Global state shared between compilation units of GLFW
//
extern _GLFWlibrary _glfw;





/*************************************************************************
 * GLFW API functions
 *************************************************************************/

/*! @brief Initializes the GLFW library.
 *
 *  This function initializes the GLFW library.  Before most GLFW functions can
 *  be used, GLFW must be initialized, and before an application terminates GLFW
 *  should be terminated in order to free any resources allocated during or
 *  after initialization.
 *
 *  If this function fails, it calls @ref glfwTerminate before returning.  If it
 *  succeeds, you should call @ref glfwTerminate before the application exits.
 *
 *  Additional calls to this function after successful initialization but before
 *  termination will return `true` immediately.
 *
 *  @return `true` if successful, or `false` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_PLATFORM_UNAVAILABLE and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @remark __X11:__ This function will set the `LC_CTYPE` category of the
 *  application locale according to the current environment if that category is
 *  still "C".  This is because the "C" locale breaks Unicode text input.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref intro_init
 *  @sa @ref glfwTerminate
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup init
 */
ErrorResponse* glfwInit(void);

/*! @brief Terminates the GLFW library.
 *
 *  This function destroys all remaining windows and cursors, restores any
 *  modified gamma ramps and frees any other allocated resources.  Once this
 *  function is called, you must again call @ref glfwInit successfully before
 *  you will be able to use most GLFW functions.
 *
 *  If GLFW has been successfully initialized, this function should be called
 *  before the application exits.  If initialization fails, there is no need to
 *  call this function, as it is called by @ref glfwInit before it returns
 *  failure.
 *
 *  This function has no effect if GLFW is not initialized.
 *
 *  @errors Possible errors include @ref ERR_PLATFORM_ERROR.
 *
 *  @remark This function may be called before @ref glfwInit.
 *
 *  @warning The contexts of any remaining windows must not be current on any
 *  other thread when this function is called.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref intro_init
 *  @sa @ref glfwInit
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup init
 */
void glfwTerminate(void);

/*! @brief Sets the error callback.
 *
 *  This function sets the error callback, which is called with an error code
 *  and a human-readable description each time a GLFW error occurs.
 *
 *  The error code is set before the callback is called.  Calling @ref
 *  glfwGetError from the error callback will return the same value as the error
 *  code argument.
 *
 *  The error callback is called on the thread where the error occurred.  If you
 *  are using GLFW from multiple threads, your error callback needs to be
 *  written accordingly.
 *
 *  Because the description string may have been generated specifically for that
 *  error, it is not guaranteed to be valid after the callback has returned.  If
 *  you wish to use it after the callback returns, you need to make a copy.
 *
 *  Once set, the error callback remains set even after the library has been
 *  terminated.
 *
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set.
 *
 *  @callback_signature
 *  @code
 *  void callback_name(int error_code, const char* description)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [callback pointer type](@ref errorFunc).
 *
 *  @errors None.
 *
 *  @remark This function may be called before @ref glfwInit.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref error_handling
 *  @sa @ref glfwGetError
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup init
 */
errorFunc glfwSetErrorCallback(errorFunc callback);

/*! @brief Returns the currently connected monitors.
 *
 *  This function returns an array of handles for all currently connected
 *  monitors.  The primary monitor is always first in the returned array.  If no
 *  monitors were found, this function returns `NULL`.
 *
 *  @param[out] count Where to store the number of monitors in the returned
 *  array.  This is set to zero if an error occurred.
 *  @return An array of monitor handles, or `NULL` if no monitors were found or
 *  if an [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @pointer_lifetime The returned array is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is guaranteed to be valid only until the
 *  monitor configuration changes or the library is terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_monitors
 *  @sa @ref monitor_event
 *  @sa @ref glfwGetPrimaryMonitor
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
GLFWmonitor** glfwGetMonitors(int* count);

/*! @brief Returns the primary monitor.
 *
 *  This function returns the primary monitor.  This is usually the monitor
 *  where elements like the task bar or global menu bar are located.
 *
 *  @return The primary monitor, or `NULL` if no monitors were found or if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @remark The primary monitor is always first in the array returned by @ref
 *  glfwGetMonitors.
 *
 *  @sa @ref monitor_monitors
 *  @sa @ref glfwGetMonitors
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
GLFWmonitor* glfwGetPrimaryMonitor(void);

/*! @brief Returns the position of the monitor's viewport on the virtual screen.
 *
 *  This function returns the position, in screen coordinates, of the upper-left
 *  corner of the specified monitor.
 *
 *  Any or all of the position arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` position arguments will be set to zero.
 *
 *  @param[in] monitor The monitor to query.
 *  @param[out] xpos Where to store the monitor x-coordinate, or `NULL`.
 *  @param[out] ypos Where to store the monitor y-coordinate, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_properties
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
void glfwGetMonitorPos(GLFWmonitor* monitor, int* xpos, int* ypos);

/*! @brief Retrieves the work area of the monitor.
 *
 *  This function returns the position, in screen coordinates, of the upper-left
 *  corner of the work area of the specified monitor along with the work area
 *  size in screen coordinates. The work area is defined as the area of the
 *  monitor not occluded by the window system task bar where present. If no
 *  task bar exists then the work area is the monitor resolution in screen
 *  coordinates.
 *
 *  Any or all of the position and size arguments may be `NULL`.  If an error
 *  occurs, all non-`NULL` position and size arguments will be set to zero.
 *
 *  @param[in] monitor The monitor to query.
 *  @param[out] xpos Where to store the monitor x-coordinate, or `NULL`.
 *  @param[out] ypos Where to store the monitor y-coordinate, or `NULL`.
 *  @param[out] width Where to store the monitor width, or `NULL`.
 *  @param[out] height Where to store the monitor height, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_workarea
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup monitor
 */
void glfwGetMonitorWorkarea(GLFWmonitor* monitor, int* xpos, int* ypos, int* width, int* height);

/*! @brief Returns the physical size of the monitor.
 *
 *  This function returns the size, in millimetres, of the display area of the
 *  specified monitor.
 *
 *  Some platforms do not provide accurate monitor size information, either
 *  because the monitor [EDID][] data is incorrect or because the driver does
 *  not report it accurately.
 *
 *  [EDID]: https://en.wikipedia.org/wiki/Extended_display_identification_data
 *
 *  Any or all of the size arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` size arguments will be set to zero.
 *
 *  @param[in] monitor The monitor to query.
 *  @param[out] widthMM Where to store the width, in millimetres, of the
 *  monitor's display area, or `NULL`.
 *  @param[out] heightMM Where to store the height, in millimetres, of the
 *  monitor's display area, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @remark __Win32:__ On Windows 8 and earlier the physical size is calculated from
 *  the current resolution and system DPI instead of querying the monitor EDID data.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_properties
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
void glfwGetMonitorPhysicalSize(GLFWmonitor* monitor, int* widthMM, int* heightMM);

/*! @brief Retrieves the content scale for the specified monitor.
 *
 *  This function retrieves the content scale for the specified monitor.  The
 *  content scale is the ratio between the current DPI and the platform's
 *  default DPI.  This is especially important for text and any UI elements.  If
 *  the pixel dimensions of your UI scaled by this look appropriate on your
 *  machine then it should appear at a reasonable size on other machines
 *  regardless of their DPI and scaling settings.  This relies on the system DPI
 *  and scaling settings being somewhat correct.
 *
 *  The content scale may depend on both the monitor resolution and pixel
 *  density and on user settings.  It may be very different from the raw DPI
 *  calculated from the physical size and current resolution.
 *
 *  @param[in] monitor The monitor to query.
 *  @param[out] xscale Where to store the x-axis content scale, or `NULL`.
 *  @param[out] yscale Where to store the y-axis content scale, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_scale
 *  @sa @ref glfwGetWindowContentScale
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup monitor
 */
void glfwGetMonitorContentScale(GLFWmonitor* monitor, float* xscale, float* yscale);

/*! @brief Returns the name of the specified monitor.
 *
 *  This function returns a human-readable name, encoded as UTF-8, of the
 *  specified monitor.  The name typically reflects the make and model of the
 *  monitor and is not guaranteed to be unique among the connected monitors.
 *
 *  @param[in] monitor The monitor to query.
 *  @return The UTF-8 encoded name of the monitor, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @pointer_lifetime The returned string is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is valid until the specified monitor is
 *  disconnected or the library is terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_properties
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
const char* glfwGetMonitorName(GLFWmonitor* monitor);

/*! @brief Sets the user pointer of the specified monitor.
 *
 *  This function sets the user-defined pointer of the specified monitor.  The
 *  current value is retained until the monitor is disconnected.  The initial
 *  value is `NULL`.
 *
 *  This function may be called from the monitor callback, even for a monitor
 *  that is being disconnected.
 *
 *  @param[in] monitor The monitor whose pointer to set.
 *  @param[in] pointer The new value.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function may be called from any thread.  Access is not
 *  synchronized.
 *
 *  @sa @ref monitor_userptr
 *  @sa @ref glfwGetMonitorUserPointer
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup monitor
 */
void glfwSetMonitorUserPointer(GLFWmonitor* monitor, void* pointer);

/*! @brief Returns the user pointer of the specified monitor.
 *
 *  This function returns the current value of the user-defined pointer of the
 *  specified monitor.  The initial value is `NULL`.
 *
 *  This function may be called from the monitor callback, even for a monitor
 *  that is being disconnected.
 *
 *  @param[in] monitor The monitor whose pointer to return.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function may be called from any thread.  Access is not
 *  synchronized.
 *
 *  @sa @ref monitor_userptr
 *  @sa @ref glfwSetMonitorUserPointer
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup monitor
 */
void* glfwGetMonitorUserPointer(GLFWmonitor* monitor);

/*! @brief Sets the monitor configuration callback.
 *
 *  This function sets the monitor configuration callback, or removes the
 *  currently set callback.  This is called when a monitor is connected to or
 *  disconnected from the system.
 *
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWmonitor* monitor, int event)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref monitorFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_event
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
monitorFunc glfwSetMonitorCallback(monitorFunc callback);

/*! @brief Returns the available video modes for the specified monitor.
 *
 *  This function returns an array of all video modes supported by the specified
 *  monitor.  The returned array is sorted in ascending order, first by color
 *  bit depth (the sum of all channel depths), then by resolution area (the
 *  product of width and height), then resolution width and finally by refresh
 *  rate.
 *
 *  @param[in] monitor The monitor to query.
 *  @param[out] count Where to store the number of video modes in the returned
 *  array.  This is set to zero if an error occurred.
 *  @return An array of video modes, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @pointer_lifetime The returned array is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is valid until the specified monitor is
 *  disconnected, this function is called again for that monitor or the library
 *  is terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_modes
 *  @sa @ref glfwGetVideoMode
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Changed to return an array of modes for a specific monitor.
 *
 *  @ingroup monitor
 */
const VideoMode* glfwGetVideoModes(GLFWmonitor* monitor, int* count);

/*! @brief Returns the current mode of the specified monitor.
 *
 *  This function returns the current video mode of the specified monitor.  If
 *  you have created a full screen window for that monitor, the return value
 *  will depend on whether that window is iconified.
 *
 *  @param[in] monitor The monitor to query.
 *  @return The current mode of the monitor, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @pointer_lifetime The returned array is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is valid until the specified monitor is
 *  disconnected or the library is terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_modes
 *  @sa @ref glfwGetVideoModes
 *
 *  @since Added in version 3.0.  Replaces `glfwGetDesktopMode`.
 *
 *  @ingroup monitor
 */
const VideoMode* glfwGetVideoMode(GLFWmonitor* monitor);

/*! @brief Generates a gamma ramp and sets it for the specified monitor.
 *
 *  This function generates an appropriately sized gamma ramp from the specified
 *  exponent and then calls @ref glfwSetGammaRamp with it.  The value must be
 *  a finite number greater than zero.
 *
 *  The software controlled gamma ramp is applied _in addition_ to the hardware
 *  gamma correction, which today is usually an approximation of sRGB gamma.
 *  This means that setting a perfectly linear ramp, or gamma 1.0, will produce
 *  the default (usually sRGB-like) behavior.
 *
 *  For gamma correct rendering with OpenGL or OpenGL ES, see the @ref
 *  WINDOW_HINT_SRGB_CAPABLE hint.
 *
 *  @param[in] monitor The monitor whose gamma ramp to set.
 *  @param[in] gamma The desired exponent.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref ERR_INVALID_VALUE,
 *  @ref ERR_PLATFORM_ERROR and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_gamma
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
void glfwSetGamma(GLFWmonitor* monitor, float gamma);

/*! @brief Returns the current gamma ramp for the specified monitor.
 *
 *  This function returns the current gamma ramp of the specified monitor.
 *
 *  @param[in] monitor The monitor to query.
 *  @return The current gamma ramp, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref ERR_PLATFORM_ERROR
 *  and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @pointer_lifetime The returned structure and its arrays are allocated and
 *  freed by GLFW.  You should not free them yourself.  They are valid until the
 *  specified monitor is disconnected, this function is called again for that
 *  monitor or the library is terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_gamma
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
const GammaRamp* glfwGetGammaRamp(GLFWmonitor* monitor);

/*! @brief Sets the current gamma ramp for the specified monitor.
 *
 *  This function sets the current gamma ramp for the specified monitor.  The
 *  original gamma ramp for that monitor is saved by GLFW the first time this
 *  function is called and is restored by @ref glfwTerminate.
 *
 *  The software controlled gamma ramp is applied _in addition_ to the hardware
 *  gamma correction, which today is usually an approximation of sRGB gamma.
 *  This means that setting a perfectly linear ramp, or gamma 1.0, will produce
 *  the default (usually sRGB-like) behavior.
 *
 *  For gamma correct rendering with OpenGL or OpenGL ES, see the @ref
 *  WINDOW_HINT_SRGB_CAPABLE hint.
 *
 *  @param[in] monitor The monitor whose gamma ramp to set.
 *  @param[in] ramp The gamma ramp to use.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref ERR_PLATFORM_ERROR
 *  and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @remark The size of the specified gamma ramp should match the size of the
 *  current ramp for that monitor.
 *
 *  @remark __Win32:__ The gamma ramp size must be 256.
 *
 *  @pointer_lifetime The specified gamma ramp is copied before this function
 *  returns.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_gamma
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
void glfwSetGammaRamp(GLFWmonitor* monitor, const GammaRamp* ramp);

/*! @brief Resets all window hints to their default values.
 *
 *  This function resets all window hints to their
 *  [default values](@ref window_hints_values).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_hints
 *  @sa @ref glfwWindowHint
 *  @sa @ref glfwWindowHintString
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwDefaultWindowHints(void);

/*! @brief Sets the specified window hint to the desired value.
 *
 *  This function sets hints for the next call to @ref glfwCreateWindow.  The
 *  hints, once set, retain their values until changed by a call to this
 *  function or @ref glfwDefaultWindowHints, or until the library is terminated.
 *
 *  Only integer value hints can be set with this function.  String value hints
 *  are set with @ref glfwWindowHintString.
 *
 *  This function does not check whether the specified hint values are valid.
 *  If you set hints to invalid values this will instead be reported by the next
 *  call to @ref glfwCreateWindow.
 *
 *  Some hints are platform specific.  These may be set on any platform but they
 *  will only affect their specific platform.  Other platforms will ignore them.
 *  Setting these hints requires no platform specific headers or functions.
 *
 *  @param[in] hint The [window hint](@ref window_hints) to set.
 *  @param[in] value The new value of the window hint.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_hints
 *  @sa @ref glfwWindowHintString
 *  @sa @ref glfwDefaultWindowHints
 *
 *  @since Added in version 3.0.  Replaces `glfwOpenWindowHint`.
 *
 *  @ingroup window
 */
void glfwWindowHint(int hint, int value);

/*! @brief Sets the specified window hint to the desired value.
 *
 *  This function sets hints for the next call to @ref glfwCreateWindow.  The
 *  hints, once set, retain their values until changed by a call to this
 *  function or @ref glfwDefaultWindowHints, or until the library is terminated.
 *
 *  Only string type hints can be set with this function.  Integer value hints
 *  are set with @ref glfwWindowHint.
 *
 *  This function does not check whether the specified hint values are valid.
 *  If you set hints to invalid values this will instead be reported by the next
 *  call to @ref glfwCreateWindow.
 *
 *  Some hints are platform specific.  These may be set on any platform but they
 *  will only affect their specific platform.  Other platforms will ignore them.
 *  Setting these hints requires no platform specific headers or functions.
 *
 *  @param[in] hint The [window hint](@ref window_hints) to set.
 *  @param[in] value The new value of the window hint.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @pointer_lifetime The specified string is copied before this function
 *  returns.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_hints
 *  @sa @ref glfwWindowHint
 *  @sa @ref glfwDefaultWindowHints
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
void glfwWindowHintString(int hint, const char* value);

/*! @brief Creates a window and its associated context.
 *
 *  This function creates a window and its associated OpenGL or OpenGL ES
 *  context.  Most of the options controlling how the window and its context
 *  should be created are specified with [window hints](@ref window_hints).
 *
 *  Successful creation does not change which context is current.  Before you
 *  can use the newly created context, you need to
 *  [make it current](@ref context_current).  For information about the `share`
 *  parameter, see @ref context_sharing.
 *
 *  The created window, framebuffer and context may differ from what you
 *  requested, as not all parameters and hints are
 *  [hard constraints](@ref window_hints_hard).  This includes the size of the
 *  window, especially for full screen windows.  To query the actual attributes
 *  of the created window, framebuffer and context, see @ref
 *  glfwGetWindowAttrib, @ref glfwGetWindowSize and @ref glfwGetFramebufferSize.
 *
 *  To create a full screen window, you need to specify the monitor the window
 *  will cover.  If no monitor is specified, the window will be windowed mode.
 *  Unless you have a way for the user to choose a specific monitor, it is
 *  recommended that you pick the primary monitor.  For more information on how
 *  to query connected monitors, see @ref monitor_monitors.
 *
 *  For full screen windows, the specified size becomes the resolution of the
 *  window's _desired video mode_.  As long as a full screen window is not
 *  iconified, the supported video mode most closely matching the desired video
 *  mode is set for the specified monitor.  For more information about full
 *  screen windows, including the creation of so called _windowed full screen_
 *  or _borderless full screen_ windows, see @ref window_windowed_full_screen.
 *
 *  Once you have created the window, you can switch it between windowed and
 *  full screen mode with @ref glfwSetWindowMonitor.  This will not affect its
 *  OpenGL or OpenGL ES context.
 *
 *  By default, newly created windows use the placement recommended by the
 *  window system.  To create the window at a specific position, set the @ref
 *  WINDOW_HINT_POSITION_X and @ref WINDOW_HINT_POSITION_Y window hints before creation.  To
 *  restore the default behavior, set either or both hints back to
 *  `ANY_POSITION`.
 *
 *  As long as at least one full screen window is not iconified, the screensaver
 *  is prohibited from starting.
 *
 *  Window systems put limits on window sizes.  Very large or very small window
 *  dimensions may be overridden by the window system on creation.  Check the
 *  actual [size](@ref window_size) after creation.
 *
 *  The [swap interval](@ref buffer_swap) is not set during window creation and
 *  the initial value may vary depending on driver settings and defaults.
 *
 *  @param[in] width The desired width, in screen coordinates, of the window.
 *  This must be greater than zero.
 *  @param[in] height The desired height, in screen coordinates, of the window.
 *  This must be greater than zero.
 *  @param[in] title The initial, UTF-8 encoded window title.
 *  @param[in] monitor The monitor to use for full screen mode, or `NULL` for
 *  windowed mode.
 *  @param[in] share The window whose context to share resources with, or `NULL`
 *  to not share resources.
 *  @return The handle of the created window, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_ENUM, @ref ERR_INVALID_VALUE, @ref ERR_API_UNAVAILABLE, @ref
 *  ERR_VERSION_UNAVAILABLE, @ref ERR_FORMAT_UNAVAILABLE, @ref
 *  ERR_NO_WINDOW_CONTEXT and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark __Win32:__ Window creation will fail if the Microsoft GDI software
 *  OpenGL implementation is the only one available.
 *
 *  @remark __Win32:__ If the executable has an icon resource named `GLFW_ICON,` it
 *  will be set as the initial icon for the window.  If no such icon is present,
 *  the `IDI_APPLICATION` icon will be used instead.  To set a different icon,
 *  see @ref glfwSetWindowIcon.
 *
 *  @remark __Win32:__ The context to share resources with must not be current on
 *  any other thread.
 *
 *  @remark __macOS:__ The OS only supports core profile contexts for OpenGL
 *  versions 3.2 and later.  Before creating an OpenGL context of version 3.2 or
 *  later you must set the [WINDOW_ATTR_HINT_OPENGL_PROFILE](@ref GLFW_OPENGL_PROFILE_hint)
 *  hint accordingly.  OpenGL 3.0 and 3.1 contexts are not supported at all
 *  on macOS.
 *
 *  @remark __macOS:__ The GLFW window has no icon, as it is not a document
 *  window, but the dock icon will be the same as the application bundle's icon.
 *  For more information on bundles, see the
 *  [Bundle Programming Guide][bundle-guide] in the Mac Developer Library.
 *
 *  [bundle-guide]: https://developer.apple.com/library/mac/documentation/CoreFoundation/Conceptual/CFBundles/
 *
 *  @remark __macOS:__  The window frame will not be rendered at full resolution
 *  on Retina displays unless the
 *  [WINDOW_HINT_SCALE_FRAMEBUFFER](@ref GLFW_SCALE_FRAMEBUFFER_hint)
 *  hint is `true` and the `NSHighResolutionCapable` key is enabled in the
 *  application bundle's `Info.plist`.  For more information, see
 *  [High Resolution Guidelines for OS X][hidpi-guide] in the Mac Developer
 *  Library.  The GLFW test and example programs use a custom `Info.plist`
 *  template for this, which can be found as `CMake/Info.plist.in` in the source
 *  tree.
 *
 *  [hidpi-guide]: https://developer.apple.com/library/mac/documentation/GraphicsAnimation/Conceptual/HighResolutionOSX/Explained/Explained.html
 *
 *  @remark __macOS:__ When activating frame autosaving with
 *  [WINDOW_HINT_COCOA_FRAME_NAME](@ref GLFW_COCOA_FRAME_NAME_hint), the specified
 *  window size and position may be overridden by previously saved values.
 *
 *  @remark __X11:__ Some window managers will not respect the placement of
 *  initially hidden windows.
 *
 *  @remark __X11:__ Due to the asynchronous nature of X11, it may take a moment for
 *  a window to reach its requested state.  This means you may not be able to
 *  query the final size, position or other attributes directly after window
 *  creation.
 *
 *  @remark __X11:__ The class part of the `WM_CLASS` window property will by
 *  default be set to the window title passed to this function.  The instance
 *  part will use the contents of the `RESOURCE_NAME` environment variable, if
 *  present and not empty, or fall back to the window title.  Set the
 *  [WINDOW_HINT_X11_CLASS_NAME](@ref GLFW_X11_CLASS_NAME_hint) and
 *  [WINDOW_HINT_X11_INSTANCE_NAME](@ref GLFW_X11_INSTANCE_NAME_hint) window hints to
 *  override this.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_creation
 *  @sa @ref glfwDestroyWindow
 *
 *  @since Added in version 3.0.  Replaces `glfwOpenWindow`.
 *
 *  @ingroup window
 */
GLFWwindow* glfwCreateWindow(int width, int height, const char* title, GLFWmonitor* monitor, GLFWwindow* share);

/*! @brief Destroys the specified window and its context.
 *
 *  This function destroys the specified window and its context.  On calling
 *  this function, no further callbacks will be called for that window.
 *
 *  If the context of the specified window is current on the main thread, it is
 *  detached before being destroyed.
 *
 *  @param[in] window The window to destroy.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @note The context of the specified window must not be current on any other
 *  thread when this function is called.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_creation
 *  @sa @ref glfwCreateWindow
 *
 *  @since Added in version 3.0.  Replaces `glfwCloseWindow`.
 *
 *  @ingroup window
 */
void glfwDestroyWindow(GLFWwindow* window);

#if defined(PLAF_DARWIN)
/*! @brief Returns the `NSWindow` of the specified window.
 *
 *  @return The `NSWindow` of the specified window, or `nil` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_UNAVAILABLE.
 *
 *  @thread_safety This function may be called from any thread.  Access is not
 *  synchronized.
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup native
 */
id glfwGetCocoaWindow(GLFWwindow* window);
#endif

/*! @brief Checks the close flag of the specified window.
 *
 *  This function returns the value of the close flag of the specified window.
 *
 *  @param[in] window The window to query.
 *  @return The value of the close flag.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function may be called from any thread.  Access is not
 *  synchronized.
 *
 *  @sa @ref window_close
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
int glfwWindowShouldClose(GLFWwindow* window);

/*! @brief Sets the close flag of the specified window.
 *
 *  This function sets the value of the close flag of the specified window.
 *  This can be used to override the user's attempt to close the window, or
 *  to signal that it should be closed.
 *
 *  @param[in] window The window whose flag to change.
 *  @param[in] value The new value.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function may be called from any thread.  Access is not
 *  synchronized.
 *
 *  @sa @ref window_close
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwSetWindowShouldClose(GLFWwindow* window, int value);

/*! @brief Returns the title of the specified window.
 *
 *  This function returns the window title, encoded as UTF-8, of the specified
 *  window.  This is the title set previously by @ref glfwCreateWindow
 *  or @ref glfwSetWindowTitle.
 *
 *  @param[in] window The window to query.
 *  @return The UTF-8 encoded window title, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @remark The returned title is currently a copy of the title last set by @ref
 *  glfwCreateWindow or @ref glfwSetWindowTitle.  It does not include any
 *  additional text which may be appended by the platform or another program.
 *
 *  @pointer_lifetime The returned string is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is valid until the next call to @ref
 *  glfwGetWindowTitle or @ref glfwSetWindowTitle, or until the library is
 *  terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_title
 *  @sa @ref glfwSetWindowTitle
 *
 *  @since Added in version 3.4.
 *
 *  @ingroup window
 */
const char* glfwGetWindowTitle(GLFWwindow* window);

/*! @brief Sets the title of the specified window.
 *
 *  This function sets the window title, encoded as UTF-8, of the specified
 *  window.
 *
 *  @param[in] window The window whose title to change.
 *  @param[in] title The UTF-8 encoded window title.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @remark __macOS:__ The window title will not be updated until the next time you
 *  process events.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_title
 *  @sa @ref glfwGetWindowTitle
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwSetWindowTitle(GLFWwindow* window, const char* title);

/*! @brief Sets the icon for the specified window.
 *
 *  This function sets the icon of the specified window.  If passed an array of
 *  candidate images, those of or closest to the sizes desired by the system are
 *  selected.  If no images are specified, the window reverts to its default
 *  icon.
 *
 *  The pixels are 32-bit, little-endian, non-premultiplied RGBA, i.e. eight
 *  bits per channel with the red channel first.  They are arranged canonically
 *  as packed sequential rows, starting from the top-left corner.
 *
 *  The desired image sizes varies depending on platform and system settings.
 *  The selected images will be rescaled as needed.  Good sizes include 16x16,
 *  32x32 and 48x48.
 *
 *  @param[in] window The window whose icon to set.
 *  @param[in] count The number of images in the specified array, or zero to
 *  revert to the default window icon.
 *  @param[in] images The images to create the icon from.  This is ignored if
 *  count is zero.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE, @ref ERR_PLATFORM_ERROR and @ref
 *  ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @pointer_lifetime The specified image data is copied before this function
 *  returns.
 *
 *  @remark __macOS:__ Regular windows do not have icons on macOS.  This function
 *  will emit @ref ERR_FEATURE_UNAVAILABLE.  The dock icon will be the same as
 *  the application bundle's icon.  For more information on bundles, see the
 *  [Bundle Programming Guide][bundle-guide] in the Mac Developer Library.
 *
 *  [bundle-guide]: https://developer.apple.com/library/mac/documentation/CoreFoundation/Conceptual/CFBundles/
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_icon
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwSetWindowIcon(GLFWwindow* window, int count, const ImageData* images);

/*! @brief Retrieves the position of the content area of the specified window.
 *
 *  This function retrieves the position, in screen coordinates, of the
 *  upper-left corner of the content area of the specified window.
 *
 *  Any or all of the position arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` position arguments will be set to zero.
 *
 *  @param[in] window The window to query.
 *  @param[out] xpos Where to store the x-coordinate of the upper-left corner of
 *  the content area, or `NULL`.
 *  @param[out] ypos Where to store the y-coordinate of the upper-left corner of
 *  the content area, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_PLATFORM_ERROR and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_pos
 *  @sa @ref glfwSetWindowPos
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwGetWindowPos(GLFWwindow* window, int* xpos, int* ypos);

/*! @brief Sets the position of the content area of the specified window.
 *
 *  This function sets the position, in screen coordinates, of the upper-left
 *  corner of the content area of the specified windowed mode window.  If the
 *  window is a full screen window, this function does nothing.
 *
 *  __Do not use this function__ to move an already visible window unless you
 *  have very good reasons for doing so, as it will confuse and annoy the user.
 *
 *  The window manager may put limits on what positions are allowed.  GLFW
 *  cannot and should not override these limits.
 *
 *  @param[in] window The window to query.
 *  @param[in] xpos The x-coordinate of the upper-left corner of the content area.
 *  @param[in] ypos The y-coordinate of the upper-left corner of the content area.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_PLATFORM_ERROR and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_pos
 *  @sa @ref glfwGetWindowPos
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwSetWindowPos(GLFWwindow* window, int xpos, int ypos);

/*! @brief Retrieves the size of the content area of the specified window.
 *
 *  This function retrieves the size, in screen coordinates, of the content area
 *  of the specified window.  If you wish to retrieve the size of the
 *  framebuffer of the window in pixels, see @ref glfwGetFramebufferSize.
 *
 *  Any or all of the size arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` size arguments will be set to zero.
 *
 *  @param[in] window The window whose size to retrieve.
 *  @param[out] width Where to store the width, in screen coordinates, of the
 *  content area, or `NULL`.
 *  @param[out] height Where to store the height, in screen coordinates, of the
 *  content area, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_size
 *  @sa @ref glfwSetWindowSize
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwGetWindowSize(GLFWwindow* window, int* width, int* height);

/*! @brief Sets the size limits of the specified window.
 *
 *  This function sets the size limits of the content area of the specified
 *  window.  If the window is full screen, the size limits only take effect
 *  once it is made windowed.  If the window is not resizable, this function
 *  does nothing.
 *
 *  The size limits are applied immediately to a windowed mode window and may
 *  cause it to be resized.
 *
 *  The maximum dimensions must be greater than or equal to the minimum
 *  dimensions and all must be greater than or equal to zero.
 *
 *  @param[in] window The window to set limits for.
 *  @param[in] minwidth The minimum width, in screen coordinates, of the content
 *  area, or `DONT_CARE`.
 *  @param[in] minheight The minimum height, in screen coordinates, of the
 *  content area, or `DONT_CARE`.
 *  @param[in] maxwidth The maximum width, in screen coordinates, of the content
 *  area, or `DONT_CARE`.
 *  @param[in] maxheight The maximum height, in screen coordinates, of the
 *  content area, or `DONT_CARE`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark If you set size limits and an aspect ratio that conflict, the
 *  results are undefined.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_sizelimits
 *  @sa @ref glfwSetWindowAspectRatio
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwSetWindowSizeLimits(GLFWwindow* window, int minwidth, int minheight, int maxwidth, int maxheight);

/*! @brief Sets the aspect ratio of the specified window.
 *
 *  This function sets the required aspect ratio of the content area of the
 *  specified window.  If the window is full screen, the aspect ratio only takes
 *  effect once it is made windowed.  If the window is not resizable, this
 *  function does nothing.
 *
 *  The aspect ratio is specified as a numerator and a denominator and both
 *  values must be greater than zero.  For example, the common 16:9 aspect ratio
 *  is specified as 16 and 9, respectively.
 *
 *  If the numerator and denominator is set to `DONT_CARE` then the aspect
 *  ratio limit is disabled.
 *
 *  The aspect ratio is applied immediately to a windowed mode window and may
 *  cause it to be resized.
 *
 *  @param[in] window The window to set limits for.
 *  @param[in] numer The numerator of the desired aspect ratio, or
 *  `DONT_CARE`.
 *  @param[in] denom The denominator of the desired aspect ratio, or
 *  `DONT_CARE`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark If you set size limits and an aspect ratio that conflict, the
 *  results are undefined.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_sizelimits
 *  @sa @ref glfwSetWindowSizeLimits
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwSetWindowAspectRatio(GLFWwindow* window, int numer, int denom);

/*! @brief Sets the size of the content area of the specified window.
 *
 *  This function sets the size, in screen coordinates, of the content area of
 *  the specified window.
 *
 *  For full screen windows, this function updates the resolution of its desired
 *  video mode and switches to the video mode closest to it, without affecting
 *  the window's context.  As the context is unaffected, the bit depths of the
 *  framebuffer remain unchanged.
 *
 *  If you wish to update the refresh rate of the desired video mode in addition
 *  to its resolution, see @ref glfwSetWindowMonitor.
 *
 *  The window manager may put limits on what sizes are allowed.  GLFW cannot
 *  and should not override these limits.
 *
 *  @param[in] window The window to resize.
 *  @param[in] width The desired width, in screen coordinates, of the window
 *  content area.
 *  @param[in] height The desired height, in screen coordinates, of the window
 *  content area.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_size
 *  @sa @ref glfwGetWindowSize
 *  @sa @ref glfwSetWindowMonitor
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwSetWindowSize(GLFWwindow* window, int width, int height);

/*! @brief Retrieves the size of the framebuffer of the specified window.
 *
 *  This function retrieves the size, in pixels, of the framebuffer of the
 *  specified window.  If you wish to retrieve the size of the window in screen
 *  coordinates, see @ref glfwGetWindowSize.
 *
 *  Any or all of the size arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` size arguments will be set to zero.
 *
 *  @param[in] window The window whose framebuffer to query.
 *  @param[out] width Where to store the width, in pixels, of the framebuffer,
 *  or `NULL`.
 *  @param[out] height Where to store the height, in pixels, of the framebuffer,
 *  or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_fbsize
 *  @sa @ref glfwSetFramebufferSizeCallback
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwGetFramebufferSize(GLFWwindow* window, int* width, int* height);

/*! @brief Retrieves the size of the frame of the window.
 *
 *  This function retrieves the size, in screen coordinates, of each edge of the
 *  frame of the specified window.  This size includes the title bar, if the
 *  window has one.  The size of the frame may vary depending on the
 *  [window-related hints](@ref window_hints_wnd) used to create it.
 *
 *  Because this function retrieves the size of each window frame edge and not
 *  the offset along a particular coordinate axis, the retrieved values will
 *  always be zero or positive.
 *
 *  Any or all of the size arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` size arguments will be set to zero.
 *
 *  @param[in] window The window whose frame size to query.
 *  @param[out] left Where to store the size, in screen coordinates, of the left
 *  edge of the window frame, or `NULL`.
 *  @param[out] top Where to store the size, in screen coordinates, of the top
 *  edge of the window frame, or `NULL`.
 *  @param[out] right Where to store the size, in screen coordinates, of the
 *  right edge of the window frame, or `NULL`.
 *  @param[out] bottom Where to store the size, in screen coordinates, of the
 *  bottom edge of the window frame, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_size
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup window
 */
void glfwGetWindowFrameSize(GLFWwindow* window, int* left, int* top, int* right, int* bottom);

/*! @brief Retrieves the content scale for the specified window.
 *
 *  This function retrieves the content scale for the specified window.  The
 *  content scale is the ratio between the current DPI and the platform's
 *  default DPI.  This is especially important for text and any UI elements.  If
 *  the pixel dimensions of your UI scaled by this look appropriate on your
 *  machine then it should appear at a reasonable size on other machines
 *  regardless of their DPI and scaling settings.  This relies on the system DPI
 *  and scaling settings being somewhat correct.
 *
 *  On platforms where each monitors can have its own content scale, the window
 *  content scale will depend on which monitor the system considers the window
 *  to be on.
 *
 *  @param[in] window The window to query.
 *  @param[out] xscale Where to store the x-axis content scale, or `NULL`.
 *  @param[out] yscale Where to store the y-axis content scale, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_scale
 *  @sa @ref glfwSetWindowContentScaleCallback
 *  @sa @ref glfwGetMonitorContentScale
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
void glfwGetWindowContentScale(GLFWwindow* window, float* xscale, float* yscale);

/*! @brief Returns the opacity of the whole window.
 *
 *  This function returns the opacity of the window, including any decorations.
 *
 *  The opacity (or alpha) value is a positive finite number between zero and
 *  one, where zero is fully transparent and one is fully opaque.  If the system
 *  does not support whole window transparency, this function always returns one.
 *
 *  The initial opacity value for newly created windows is one.
 *
 *  @param[in] window The window to query.
 *  @return The opacity value of the specified window.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_transparency
 *  @sa @ref glfwSetWindowOpacity
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
float glfwGetWindowOpacity(GLFWwindow* window);

/*! @brief Sets the opacity of the whole window.
 *
 *  This function sets the opacity of the window, including any decorations.
 *
 *  The opacity (or alpha) value is a positive finite number between zero and
 *  one, where zero is fully transparent and one is fully opaque.
 *
 *  The initial opacity value for newly created windows is one.
 *
 *  A window created with framebuffer transparency may not use whole window
 *  transparency.  The results of doing this are undefined.
 *
 *  @param[in] window The window to set the opacity for.
 *  @param[in] opacity The desired opacity of the specified window.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_PLATFORM_ERROR and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_transparency
 *  @sa @ref glfwGetWindowOpacity
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
void glfwSetWindowOpacity(GLFWwindow* window, float opacity);

/*! @brief Iconifies the specified window.
 *
 *  This function iconifies (minimizes) the specified window if it was
 *  previously restored.  If the window is already iconified, this function does
 *  nothing.
 *
 *  If the specified window is a full screen window, GLFW restores the original
 *  video mode of the monitor.  The window's desired video mode is set again
 *  when the window is restored.
 *
 *  @param[in] window The window to iconify.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_iconify
 *  @sa @ref glfwRestoreWindow
 *  @sa @ref glfwMaximizeWindow
 *
 *  @since Added in version 2.1.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwIconifyWindow(GLFWwindow* window);

/*! @brief Restores the specified window.
 *
 *  This function restores the specified window if it was previously iconified
 *  (minimized) or maximized.  If the window is already restored, this function
 *  does nothing.
 *
 *  If the specified window is an iconified full screen window, its desired
 *  video mode is set again for its monitor when the window is restored.
 *
 *  @param[in] window The window to restore.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_iconify
 *  @sa @ref glfwIconifyWindow
 *  @sa @ref glfwMaximizeWindow
 *
 *  @since Added in version 2.1.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwRestoreWindow(GLFWwindow* window);

/*! @brief Maximizes the specified window.
 *
 *  This function maximizes the specified window if it was previously not
 *  maximized.  If the window is already maximized, this function does nothing.
 *
 *  If the specified window is a full screen window, this function does nothing.
 *
 *  @param[in] window The window to maximize.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @par Thread Safety
 *  This function may only be called from the main thread.
 *
 *  @sa @ref window_iconify
 *  @sa @ref glfwIconifyWindow
 *  @sa @ref glfwRestoreWindow
 *
 *  @since Added in GLFW 3.2.
 *
 *  @ingroup window
 */
void glfwMaximizeWindow(GLFWwindow* window);

/*! @brief Makes the specified window visible.
 *
 *  This function makes the specified window visible if it was previously
 *  hidden.  If the window is already visible or is in full screen mode, this
 *  function does nothing.
 *
 *  By default, windowed mode windows are focused when shown
 *  Set the [WINDOW_ATTR_HINT_FOCUS_ON_SHOW](@ref GLFW_FOCUS_ON_SHOW_hint) window hint
 *  to change this behavior for all newly created windows, or change the
 *  behavior for an existing window with @ref glfwSetWindowAttrib.
 *
 *  @param[in] window The window to make visible.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_hide
 *  @sa @ref glfwHideWindow
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwShowWindow(GLFWwindow* window);

/*! @brief Hides the specified window.
 *
 *  This function hides the specified window if it was previously visible.  If
 *  the window is already hidden or is in full screen mode, this function does
 *  nothing.
 *
 *  @param[in] window The window to hide.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_hide
 *  @sa @ref glfwShowWindow
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwHideWindow(GLFWwindow* window);

/*! @brief Brings the specified window to front and sets input focus.
 *
 *  This function brings the specified window to front and sets input focus.
 *  The window should already be visible and not iconified.
 *
 *  By default, both windowed and full screen mode windows are focused when
 *  initially created.  Set the [WINDOW_ATTR_HINT_FOCUSED](@ref GLFW_FOCUSED_hint) to
 *  disable this behavior.
 *
 *  Also by default, windowed mode windows are focused when shown
 *  with @ref glfwShowWindow. Set the
 *  [WINDOW_ATTR_HINT_FOCUS_ON_SHOW](@ref GLFW_FOCUS_ON_SHOW_hint) to disable this behavior.
 *
 *  __Do not use this function__ to steal focus from other applications unless
 *  you are certain that is what the user wants.  Focus stealing can be
 *  extremely disruptive.
 *
 *  For a less disruptive way of getting the user's attention, see
 *  [attention requests](@ref window_attention).
 *
 *  @param[in] window The window to give input focus.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_focus
 *  @sa @ref window_attention
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwFocusWindow(GLFWwindow* window);

/*! @brief Requests user attention to the specified window.
 *
 *  This function requests user attention to the specified window.  On
 *  platforms where this is not supported, attention is requested to the
 *  application as a whole.
 *
 *  Once the user has given attention, usually by focusing the window or
 *  application, the system will end the request automatically.
 *
 *  @param[in] window The window to request attention to.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @remark __macOS:__ Attention is requested to the application as a whole, not the
 *  specific window.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_attention
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
void glfwRequestWindowAttention(GLFWwindow* window);

/*! @brief Returns the monitor that the window uses for full screen mode.
 *
 *  This function returns the handle of the monitor that the specified window is
 *  in full screen on.
 *
 *  @param[in] window The window to query.
 *  @return The monitor, or `NULL` if the window is in windowed mode or an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_monitor
 *  @sa @ref glfwSetWindowMonitor
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
GLFWmonitor* glfwGetWindowMonitor(GLFWwindow* window);

/*! @brief Sets the mode, monitor, video mode and placement of a window.
 *
 *  This function sets the monitor that the window uses for full screen mode or,
 *  if the monitor is `NULL`, makes it windowed mode.
 *
 *  When setting a monitor, this function updates the width, height and refresh
 *  rate of the desired video mode and switches to the video mode closest to it.
 *  The window position is ignored when setting a monitor.
 *
 *  When the monitor is `NULL`, the position, width and height are used to
 *  place the window content area.  The refresh rate is ignored when no monitor
 *  is specified.
 *
 *  If you only wish to update the resolution of a full screen window or the
 *  size of a windowed mode window, see @ref glfwSetWindowSize.
 *
 *  When a window transitions from full screen to windowed mode, this function
 *  restores any previous window settings such as whether it is decorated,
 *  floating, resizable, has size or aspect ratio limits, etc.
 *
 *  @param[in] window The window whose monitor, size or video mode to set.
 *  @param[in] monitor The desired monitor, or `NULL` to set windowed mode.
 *  @param[in] xpos The desired x-coordinate of the upper-left corner of the
 *  content area.
 *  @param[in] ypos The desired y-coordinate of the upper-left corner of the
 *  content area.
 *  @param[in] width The desired with, in screen coordinates, of the content
 *  area or video mode.
 *  @param[in] height The desired height, in screen coordinates, of the content
 *  area or video mode.
 *  @param[in] refreshRate The desired refresh rate, in Hz, of the video mode,
 *  or `DONT_CARE`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @remark The OpenGL or OpenGL ES context will not be destroyed or otherwise
 *  affected by any resizing or mode switching, although you may need to update
 *  your viewport if the framebuffer size has changed.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_monitor
 *  @sa @ref window_full_screen
 *  @sa @ref glfwGetWindowMonitor
 *  @sa @ref glfwSetWindowSize
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwSetWindowMonitor(GLFWwindow* window, GLFWmonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate);

/*! @brief Returns an attribute of the specified window.
 *
 *  This function returns the value of an attribute of the specified window or
 *  its OpenGL or OpenGL ES context.
 *
 *  @param[in] window The window to query.
 *  @param[in] attrib The [window attribute](@ref window_attribs) whose value to
 *  return.
 *  @return The value of the attribute, or zero if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_ENUM and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark Framebuffer related hints are not window attributes.  See @ref
 *  window_attribs_fb for more information.
 *
 *  @remark Zero is a valid value for many window and context related
 *  attributes so you cannot use a return value of zero as an indication of
 *  errors.  However, this function should not fail as long as it is passed
 *  valid arguments and the library has been [initialized](@ref intro_init).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_attribs
 *  @sa @ref glfwSetWindowAttrib
 *
 *  @since Added in version 3.0.  Replaces `glfwGetWindowParam` and
 *  `glfwGetGLVersion`.
 *
 *  @ingroup window
 */
int glfwGetWindowAttrib(GLFWwindow* window, int attrib);

/*! @brief Sets an attribute of the specified window.
 *
 *  This function sets the value of an attribute of the specified window.
 *
 *  The supported attributes are [WINDOW_ATTR_HINT_DECORATED](@ref GLFW_DECORATED_attrib),
 *  [WINDOW_ATTR_HINT_RESIZABLE](@ref GLFW_RESIZABLE_attrib),
 *  [WINDOW_ATTR_HINT_FLOATING](@ref GLFW_FLOATING_attrib),
 *  [WINDOW_ATTR_HINT_AUTO_ICONIFY](@ref GLFW_AUTO_ICONIFY_attrib) and
 *  [WINDOW_ATTR_HINT_FOCUS_ON_SHOW](@ref GLFW_FOCUS_ON_SHOW_attrib).
 *  [WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH](@ref GLFW_MOUSE_PASSTHROUGH_attrib)
 *
 *  Some of these attributes are ignored for full screen windows.  The new
 *  value will take effect if the window is later made windowed.
 *
 *  Some of these attributes are ignored for windowed mode windows.  The new
 *  value will take effect if the window is later made full screen.
 *
 *  @param[in] window The window to set the attribute for.
 *  @param[in] attrib A supported window attribute.
 *  @param[in] value `true` or `false`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_ENUM, @ref ERR_INVALID_VALUE, @ref ERR_PLATFORM_ERROR and @ref
 *  ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @remark Calling @ref glfwGetWindowAttrib will always return the latest
 *  value, even if that value is ignored by the current mode of the window.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_attribs
 *  @sa @ref glfwGetWindowAttrib
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
void glfwSetWindowAttrib(GLFWwindow* window, int attrib, int value);

/*! @brief Sets the user pointer of the specified window.
 *
 *  This function sets the user-defined pointer of the specified window.  The
 *  current value is retained until the window is destroyed.  The initial value
 *  is `NULL`.
 *
 *  @param[in] window The window whose pointer to set.
 *  @param[in] pointer The new value.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function may be called from any thread.  Access is not
 *  synchronized.
 *
 *  @sa @ref window_userptr
 *  @sa @ref glfwGetWindowUserPointer
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwSetWindowUserPointer(GLFWwindow* window, void* pointer);

/*! @brief Returns the user pointer of the specified window.
 *
 *  This function returns the current value of the user-defined pointer of the
 *  specified window.  The initial value is `NULL`.
 *
 *  @param[in] window The window whose pointer to return.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function may be called from any thread.  Access is not
 *  synchronized.
 *
 *  @sa @ref window_userptr
 *  @sa @ref glfwSetWindowUserPointer
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void* glfwGetWindowUserPointer(GLFWwindow* window);

/*! @brief Sets the position callback for the specified window.
 *
 *  This function sets the position callback of the specified window, which is
 *  called when the window is moved.  The callback is provided with the
 *  position, in screen coordinates, of the upper-left corner of the content
 *  area of the window.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, int xpos, int ypos)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowPosFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_pos
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
windowPosFunc glfwSetWindowPosCallback(GLFWwindow* window, windowPosFunc callback);

/*! @brief Sets the size callback for the specified window.
 *
 *  This function sets the size callback of the specified window, which is
 *  called when the window is resized.  The callback is provided with the size,
 *  in screen coordinates, of the content area of the window.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, int width, int height)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowSizeFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_size
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup window
 */
windowSizeFunc glfwSetWindowSizeCallback(GLFWwindow* window, windowSizeFunc callback);

/*! @brief Sets the close callback for the specified window.
 *
 *  This function sets the close callback of the specified window, which is
 *  called when the user attempts to close the window, for example by clicking
 *  the close widget in the title bar.
 *
 *  The close flag is set before this callback is called, but you can modify it
 *  at any time with @ref glfwSetWindowShouldClose.
 *
 *  The close callback is not triggered by @ref glfwDestroyWindow.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowCloseFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @remark __macOS:__ Selecting Quit from the application menu will trigger the
 *  close callback for all windows.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_close
 *
 *  @since Added in version 2.5.
 *  __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup window
 */
windowCloseFunc glfwSetWindowCloseCallback(GLFWwindow* window, windowCloseFunc callback);

/*! @brief Sets the refresh callback for the specified window.
 *
 *  This function sets the refresh callback of the specified window, which is called when the content area of the window
 *  needs to be redrawn, for example if the window has been exposed after having been covered by another window.
 *
 *  On compositing window systems where the window contents are saved off-screen, this callback may be called only very
 *  infrequently or never at all.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the library had not been [initialized](@ref
 *  intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window);
 *  @endcode
 *  For more information about the callback parameters, see the [function pointer type](@ref windowRefreshFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_refresh
 *
 *  @since Added in version 2.5. __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup window
 */
windowRefreshFunc glfwSetWindowRefreshCallback(GLFWwindow* window, windowRefreshFunc callback);

/*! @brief Sets the focus callback for the specified window.
 *
 *  This function sets the focus callback of the specified window, which is
 *  called when the window gains or loses input focus.
 *
 *  After the focus callback is called for a window that lost input focus,
 *  synthetic key and mouse button release events will be generated for all such
 *  that had been pressed.  For more information, see @ref glfwSetKeyCallback
 *  and @ref glfwSetMouseButtonCallback.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, int focused)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowFocusFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_focus
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
windowFocusFunc glfwSetWindowFocusCallback(GLFWwindow* window, windowFocusFunc callback);

/*! @brief Sets the iconify callback for the specified window.
 *
 *  This function sets the iconification callback of the specified window, which
 *  is called when the window is iconified or restored.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, int iconified)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowIconifyFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_iconify
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
windowIconifyFunc glfwSetWindowIconifyCallback(GLFWwindow* window, windowIconifyFunc callback);

/*! @brief Sets the maximize callback for the specified window.
 *
 *  This function sets the maximization callback of the specified window, which
 *  is called when the window is maximized or restored.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, int maximized)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowMaximizeFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_maximize
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
windowMaximizeFunc glfwSetWindowMaximizeCallback(GLFWwindow* window, windowMaximizeFunc callback);

/*! @brief Sets the framebuffer resize callback for the specified window.
 *
 *  This function sets the framebuffer resize callback of the specified window,
 *  which is called when the framebuffer of the specified window is resized.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, int width, int height)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref frameBufferSizeFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_fbsize
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
frameBufferSizeFunc glfwSetFramebufferSizeCallback(GLFWwindow* window, frameBufferSizeFunc callback);

/*! @brief Sets the window content scale callback for the specified window.
 *
 *  This function sets the window content scale callback of the specified window,
 *  which is called when the content scale of the specified window changes.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, float xscale, float yscale)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowContextScaleFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_scale
 *  @sa @ref glfwGetWindowContentScale
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
windowContextScaleFunc glfwSetWindowContentScaleCallback(GLFWwindow* window, windowContextScaleFunc callback);

/*! @brief Processes all pending events.
 *
 *  This function processes only those events that are already in the event
 *  queue and then returns immediately.  Processing events will cause the window
 *  and input callbacks associated with those events to be called.
 *
 *  On some platforms, a window move, resize or menu operation will cause event
 *  processing to block.  This is due to how event processing is designed on
 *  those platforms.  You can use the
 *  [window refresh callback](@ref window_refresh) to redraw the contents of
 *  your window when necessary during such operations.
 *
 *  Do not assume that callbacks you set will _only_ be called in response to
 *  event processing functions like this one.  While it is necessary to poll for
 *  events, window systems that require GLFW to register callbacks of its own
 *  can pass events to GLFW in response to many window system function calls.
 *  GLFW will pass those events on to the application callbacks before
 *  returning.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref events
 *  @sa @ref glfwWaitEvents
 *  @sa @ref glfwWaitEventsTimeout
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup window
 */
void glfwPollEvents(void);

/*! @brief Waits until events are queued and processes them.
 *
 *  This function puts the calling thread to sleep until at least one event is
 *  available in the event queue.  Once one or more events are available,
 *  it behaves exactly like @ref glfwPollEvents, i.e. the events in the queue
 *  are processed and the function then returns immediately.  Processing events
 *  will cause the window and input callbacks associated with those events to be
 *  called.
 *
 *  Since not all events are associated with callbacks, this function may return
 *  without a callback having been called even if you are monitoring all
 *  callbacks.
 *
 *  On some platforms, a window move, resize or menu operation will cause event
 *  processing to block.  This is due to how event processing is designed on
 *  those platforms.  You can use the
 *  [window refresh callback](@ref window_refresh) to redraw the contents of
 *  your window when necessary during such operations.
 *
 *  Do not assume that callbacks you set will _only_ be called in response to
 *  event processing functions like this one.  While it is necessary to poll for
 *  events, window systems that require GLFW to register callbacks of its own
 *  can pass events to GLFW in response to many window system function calls.
 *  GLFW will pass those events on to the application callbacks before
 *  returning.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref events
 *  @sa @ref glfwPollEvents
 *  @sa @ref glfwWaitEventsTimeout
 *
 *  @since Added in version 2.5.
 *
 *  @ingroup window
 */
void glfwWaitEvents(void);

/*! @brief Waits with timeout until events are queued and processes them.
 *
 *  This function puts the calling thread to sleep until at least one event is
 *  available in the event queue, or until the specified timeout is reached.  If
 *  one or more events are available, it behaves exactly like @ref
 *  glfwPollEvents, i.e. the events in the queue are processed and the function
 *  then returns immediately.  Processing events will cause the window and input
 *  callbacks associated with those events to be called.
 *
 *  The timeout value must be a positive finite number.
 *
 *  Since not all events are associated with callbacks, this function may return
 *  without a callback having been called even if you are monitoring all
 *  callbacks.
 *
 *  On some platforms, a window move, resize or menu operation will cause event
 *  processing to block.  This is due to how event processing is designed on
 *  those platforms.  You can use the
 *  [window refresh callback](@ref window_refresh) to redraw the contents of
 *  your window when necessary during such operations.
 *
 *  Do not assume that callbacks you set will _only_ be called in response to
 *  event processing functions like this one.  While it is necessary to poll for
 *  events, window systems that require GLFW to register callbacks of its own
 *  can pass events to GLFW in response to many window system function calls.
 *  GLFW will pass those events on to the application callbacks before
 *  returning.
 *
 *  @param[in] timeout The maximum amount of time, in seconds, to wait.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE and @ref ERR_PLATFORM_ERROR.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref events
 *  @sa @ref glfwPollEvents
 *  @sa @ref glfwWaitEvents
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwWaitEventsTimeout(double timeout);

/*! @brief Posts an empty event to the event queue.
 *
 *  This function posts an empty event from the current thread to the event
 *  queue, causing @ref glfwWaitEvents or @ref glfwWaitEventsTimeout to return.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref events
 *  @sa @ref glfwWaitEvents
 *  @sa @ref glfwWaitEventsTimeout
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup window
 */
void glfwPostEmptyEvent(void);

/*! @brief Returns the value of an input option for the specified window.
 *
 *  This function returns the value of an input option for the specified window.
 *  The mode must be one of @ref INPUT_MODE_CURSOR, @ref INPUT_MODE_STICKY_KEYS,
 *  @ref INPUT_MODE_STICKY_MOUSE_BUTTONS, @ref INPUT_MODE_LOCK_KEY_MODS or
 *  @ref INPUT_MODE_RAW_MOUSE_MOTION.
 *
 *  @param[in] window The window to query.
 *  @param[in] mode One of `INPUT_MODE_CURSOR`, `INPUT_MODE_STICKY_KEYS`,
 *  `INPUT_MODE_STICKY_MOUSE_BUTTONS`, `INPUT_MODE_LOCK_KEY_MODS` or
 *  `INPUT_MODE_RAW_MOUSE_MOTION`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref glfwSetInputMode
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup input
 */
int glfwGetInputMode(GLFWwindow* window, int mode);

/*! @brief Sets an input option for the specified window.
 *
 *  This function sets an input mode option for the specified window.  The mode
 *  must be one of @ref INPUT_MODE_CURSOR, @ref INPUT_MODE_STICKY_KEYS,
 *  @ref INPUT_MODE_STICKY_MOUSE_BUTTONS, @ref INPUT_MODE_LOCK_KEY_MODS
 *  @ref INPUT_MODE_RAW_MOUSE_MOTION, or @ref INPUT_MODE_UNLIMITED_MOUSE_BUTTONS.
 *
 *  If the mode is `INPUT_MODE_CURSOR`, the value must be one of the following cursor
 *  modes:
 *  - `CURSOR_NORMAL` makes the cursor visible and behaving normally.
 *  - `CURSOR_HIDDEN` makes the cursor invisible when it is over the
 *    content area of the window but does not restrict the cursor from leaving.
 *  - `CURSOR_DISABLED` hides and grabs the cursor, providing virtual
 *    and unlimited cursor movement.  This is useful for implementing for
 *    example 3D camera controls.
 *  - `CURSOR_CAPTURED` makes the cursor visible and confines it to the
 *    content area of the window.
 *
 *  If the mode is `INPUT_MODE_STICKY_KEYS`, the value must be either `true` to
 *  enable sticky keys, or `false` to disable it.  If sticky keys are
 *  enabled, a key press will ensure that @ref glfwGetKey returns `INPUT_PRESS`
 *  the next time it is called even if the key had been released before the
 *  call.  This is useful when you are only interested in whether keys have been
 *  pressed but not when or in which order.
 *
 *  If the mode is `INPUT_MODE_STICKY_MOUSE_BUTTONS`, the value must be either
 *  `true` to enable sticky mouse buttons, or `false` to disable it.
 *  If sticky mouse buttons are enabled, a mouse button press will ensure that
 *  @ref glfwGetMouseButton returns `INPUT_PRESS` the next time it is called even
 *  if the mouse button had been released before the call.  This is useful when
 *  you are only interested in whether mouse buttons have been pressed but not
 *  when or in which order.
 *
 *  If the mode is `INPUT_MODE_LOCK_KEY_MODS`, the value must be either `true` to
 *  enable lock key modifier bits, or `false` to disable them.  If enabled,
 *  callbacks that receive modifier bits will also have the @ref
 *  MOD_CAPS_LOCK bit set when the event was generated with Caps Lock on,
 *  and the @ref MOD_NUM_LOCK bit when Num Lock was on.
 *
 *  If the mode is `INPUT_MODE_RAW_MOUSE_MOTION`, the value must be either `true`
 *  to enable raw (unscaled and unaccelerated) mouse motion when the cursor is
 *  disabled, or `false` to disable it.  If raw motion is not supported,
 *  attempting to set this will emit @ref ERR_FEATURE_UNAVAILABLE.  Call @ref
 *  glfwRawMouseMotionSupported to check for support.
 *
 *  If the mode is `INPUT_MODE_UNLIMITED_MOUSE_BUTTONS`, the value must be either
 *  `true` to disable the mouse button limit when calling the mouse button
 *  callback, or `false` to limit the mouse buttons sent to the callback
 *  to the mouse button token values up to `MOUSE_BUTTON_LAST`.
 *
 *  @param[in] window The window whose input mode to set.
 *  @param[in] mode One of `INPUT_MODE_CURSOR`, `INPUT_MODE_STICKY_KEYS`,
 *  `INPUT_MODE_STICKY_MOUSE_BUTTONS`, `INPUT_MODE_LOCK_KEY_MODS` or
 *  `INPUT_MODE_RAW_MOUSE_MOTION`.
 *  @param[in] value The new value of the specified input mode.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_ENUM, @ref ERR_PLATFORM_ERROR and @ref
 *  ERR_FEATURE_UNAVAILABLE (see above).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref glfwGetInputMode
 *
 *  @since Added in version 3.0.  Replaces `glfwEnable` and `glfwDisable`.
 *
 *  @ingroup input
 */
void glfwSetInputMode(GLFWwindow* window, int mode, int value);

/*! @brief Returns whether raw mouse motion is supported.
 *
 *  This function returns whether raw mouse motion is supported on the current
 *  system.  This status does not change after GLFW has been initialized so you
 *  only need to check this once.  If you attempt to enable raw motion on
 *  a system that does not support it, @ref ERR_PLATFORM_ERROR will be emitted.
 *
 *  Raw mouse motion is closer to the actual motion of the mouse across
 *  a surface.  It is not affected by the scaling and acceleration applied to
 *  the motion of the desktop cursor.  That processing is suitable for a cursor
 *  while raw motion is better for controlling for example a 3D camera.  Because
 *  of this, raw mouse motion is only provided when the cursor is disabled.
 *
 *  @return `true` if raw mouse motion is supported on the current machine,
 *  or `false` otherwise.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref raw_mouse_motion
 *  @sa @ref glfwSetInputMode
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup input
 */
int glfwRawMouseMotionSupported(void);

/*! @brief Returns the layout-specific name of the specified printable key.
 *
 *  This function returns the name of the specified printable key, encoded as
 *  UTF-8.  This is typically the character that key would produce without any
 *  modifier keys, intended for displaying key bindings to the user.  For dead
 *  keys, it is typically the diacritic it would add to a character.
 *
 *  __Do not use this function__ for [text input](@ref input_char).  You will
 *  break text input for many languages even if it happens to work for yours.
 *
 *  If the key is `KEY_UNKNOWN`, the scancode is used to identify the key,
 *  otherwise the scancode is ignored.  If you specify a non-printable key, or
 *  `KEY_UNKNOWN` and a scancode that maps to a non-printable key, this
 *  function returns `NULL` but does not emit an error.
 *
 *  This behavior allows you to always pass in the arguments in the
 *  [key callback](@ref input_key) without modification.
 *
 *  The printable keys are:
 *  - `KEY_APOSTROPHE`
 *  - `KEY_COMMA`
 *  - `KEY_MINUS`
 *  - `KEY_PERIOD`
 *  - `KEY_SLASH`
 *  - `KEY_SEMICOLON`
 *  - `KEY_EQUAL`
 *  - `KEY_LEFT_BRACKET`
 *  - `KEY_RIGHT_BRACKET`
 *  - `KEY_BACKSLASH`
 *  - `KEY_WORLD_1`
 *  - `KEY_WORLD_2`
 *  - `KEY_0` to `KEY_9`
 *  - `KEY_A` to `KEY_Z`
 *  - `KEY_KP_0` to `KEY_KP_9`
 *  - `KEY_KP_DECIMAL`
 *  - `KEY_KP_DIVIDE`
 *  - `KEY_KP_MULTIPLY`
 *  - `KEY_KP_SUBTRACT`
 *  - `KEY_KP_ADD`
 *  - `KEY_KP_EQUAL`
 *
 *  Names for printable keys depend on keyboard layout, while names for
 *  non-printable keys are the same across layouts but depend on the application
 *  language and should be localized along with other user interface text.
 *
 *  @param[in] key The key to query, or `KEY_UNKNOWN`.
 *  @param[in] scancode The scancode of the key to query.
 *  @return The UTF-8 encoded, layout-specific name of the key, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE, @ref ERR_INVALID_ENUM and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark The contents of the returned string may change when a keyboard
 *  layout change event is received.
 *
 *  @pointer_lifetime The returned string is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is valid until the library is terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_key_name
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup input
 */
const char* glfwGetKeyName(int key, int scancode);

/*! @brief Returns the platform-specific scancode of the specified key.
 *
 *  This function returns the platform-specific scancode of the specified key.
 *
 *  If the specified [key token](@ref keys) corresponds to a physical key not
 *  supported on the current platform then this method will return `-1`.
 *  Calling this function with anything other than a key token will return `-1`
 *  and generate a @ref ERR_INVALID_ENUM error.
 *
 *  @param[in] key Any [key token](@ref keys).
 *  @return The platform-specific scancode for the key, or `-1` if the key is
 *  not supported on the current platform or an [error](@ref error_handling)
 *  occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref input_key
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup input
 */
int glfwGetKeyScancode(int key);

/*! @brief Returns the last reported state of a keyboard key for the specified
 *  window.
 *
 *  This function returns the last state reported for the specified key to the
 *  specified window.  The returned state is one of `INPUT_PRESS` or
 *  `INPUT_RELEASE`.  The action `INPUT_REPEAT` is only reported to the key callback.
 *
 *  If the @ref INPUT_MODE_STICKY_KEYS input mode is enabled, this function returns
 *  `INPUT_PRESS` the first time you call it for a key that was pressed, even if
 *  that key has already been released.
 *
 *  The key functions deal with physical keys, with [key tokens](@ref keys)
 *  named after their use on the standard US keyboard layout.  If you want to
 *  input text, use the Unicode character callback instead.
 *
 *  The [modifier key bit masks](@ref mods) are not key tokens and cannot be
 *  used with this function.
 *
 *  __Do not use this function__ to implement [text input](@ref input_char).
 *
 *  @param[in] window The desired window.
 *  @param[in] key The desired [keyboard key](@ref keys).  `KEY_UNKNOWN` is
 *  not a valid key for this function.
 *  @return One of `INPUT_PRESS` or `INPUT_RELEASE`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_key
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup input
 */
int glfwGetKey(GLFWwindow* window, int key);

/*! @brief Returns the last reported state of a mouse button for the specified
 *  window.
 *
 *  This function returns the last state reported for the specified mouse button
 *  to the specified window.  The returned state is one of `INPUT_PRESS` or
 *  `INPUT_RELEASE`.
 *
 *  If the @ref INPUT_MODE_STICKY_MOUSE_BUTTONS input mode is enabled, this function
 *  returns `INPUT_PRESS` the first time you call it for a mouse button that was
 *  pressed, even if that mouse button has already been released.
 *
 *  The @ref INPUT_MODE_UNLIMITED_MOUSE_BUTTONS input mode does not effect the
 *  limit on buttons which can be polled with this function.
 *
 *  @param[in] window The desired window.
 *  @param[in] button The desired [mouse button token](@ref buttons).
 *  @return One of `INPUT_PRESS` or `INPUT_RELEASE`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_mouse_button
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup input
 */
int glfwGetMouseButton(GLFWwindow* window, int button);

/*! @brief Retrieves the position of the cursor relative to the content area of
 *  the window.
 *
 *  This function returns the position of the cursor, in screen coordinates,
 *  relative to the upper-left corner of the content area of the specified
 *  window.
 *
 *  If the cursor is disabled (with `CURSOR_DISABLED`) then the cursor
 *  position is unbounded and limited only by the minimum and maximum values of
 *  a `double`.
 *
 *  The coordinate can be converted to their integer equivalents with the
 *  `floor` function.  Casting directly to an integer type works for positive
 *  coordinates, but fails for negative ones.
 *
 *  Any or all of the position arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` position arguments will be set to zero.
 *
 *  @param[in] window The desired window.
 *  @param[out] xpos Where to store the cursor x-coordinate, relative to the
 *  left edge of the content area, or `NULL`.
 *  @param[out] ypos Where to store the cursor y-coordinate, relative to the to
 *  top edge of the content area, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_pos
 *  @sa @ref glfwSetCursorPos
 *
 *  @since Added in version 3.0.  Replaces `glfwGetMousePos`.
 *
 *  @ingroup input
 */
void glfwGetCursorPos(GLFWwindow* window, double* xpos, double* ypos);

/*! @brief Sets the position of the cursor, relative to the content area of the
 *  window.
 *
 *  This function sets the position, in screen coordinates, of the cursor
 *  relative to the upper-left corner of the content area of the specified
 *  window.  The window must have input focus.  If the window does not have
 *  input focus when this function is called, it fails silently.
 *
 *  __Do not use this function__ to implement things like camera controls.  GLFW
 *  already provides the `CURSOR_DISABLED` cursor mode that hides the
 *  cursor, transparently re-centers it and provides unconstrained cursor
 *  motion.  See @ref glfwSetInputMode for more information.
 *
 *  If the cursor mode is `CURSOR_DISABLED` then the cursor position is
 *  unconstrained and limited only by the minimum and maximum values of
 *  a `double`.
 *
 *  @param[in] window The desired window.
 *  @param[in] xpos The desired x-coordinate, relative to the left edge of the
 *  content area.
 *  @param[in] ypos The desired y-coordinate, relative to the top edge of the
 *  content area.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_PLATFORM_ERROR and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_pos
 *  @sa @ref glfwGetCursorPos
 *
 *  @since Added in version 3.0.  Replaces `glfwSetMousePos`.
 *
 *  @ingroup input
 */
void glfwSetCursorPos(GLFWwindow* window, double xpos, double ypos);

/*! @brief Creates a custom cursor.
 *
 *  Creates a new custom cursor image that can be set for a window with @ref
 *  glfwSetCursor.  The cursor can be destroyed with @ref glfwDestroyCursor.
 *  Any remaining cursors are destroyed by @ref glfwTerminate.
 *
 *  The pixels are 32-bit, little-endian, non-premultiplied RGBA, i.e. eight
 *  bits per channel with the red channel first.  They are arranged canonically
 *  as packed sequential rows, starting from the top-left corner.
 *
 *  The cursor hotspot is specified in pixels, relative to the upper-left corner
 *  of the cursor image.  Like all other coordinate systems in GLFW, the X-axis
 *  points to the right and the Y-axis points down.
 *
 *  @param[in] image The desired cursor image.
 *  @param[in] xhot The desired x-coordinate, in pixels, of the cursor hotspot.
 *  @param[in] yhot The desired y-coordinate, in pixels, of the cursor hotspot.
 *  @return The handle of the created cursor, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE and @ref ERR_PLATFORM_ERROR.
 *
 *  @pointer_lifetime The specified image data is copied before this function
 *  returns.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_object
 *  @sa @ref glfwDestroyCursor
 *  @sa @ref glfwCreateStandardCursor
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
GLFWcursor* glfwCreateCursor(const ImageData* image, int xhot, int yhot);

/*! @brief Creates a cursor with a standard shape.
 *
 *  Returns a cursor with a standard shape, that can be set for a window with
 *  @ref glfwSetCursor.  The images for these cursors come from the system
 *  cursor theme and their exact appearance will vary between platforms.
 *
 *  Most of these shapes are guaranteed to exist on every supported platform but
 *  a few may not be present.  See the table below for details.
 *
 *  Cursor shape                   | Windows | macOS | X11
 *  ------------------------------ | ------- | ----- | ------
 *  @ref STD_CURSOR_ARROW         | Yes     | Yes   | Yes
 *  @ref STD_CURSOR_IBEAM         | Yes     | Yes   | Yes
 *  @ref STD_CURSOR_CROSSHAIR     | Yes     | Yes   | Yes
 *  @ref STD_CURSOR_POINTING_HAND | Yes     | Yes   | Yes
 *  @ref STD_CURSOR_HORIZONTAL_RESIZE     | Yes     | Yes   | Yes
 *  @ref STD_CURSOR_VERTICAL_RESIZE     | Yes     | Yes   | Yes
 *
 *  1) This uses a private system API and may fail in the future.
 *
 *  2) This uses a newer standard that not all cursor themes support.
 *
 *  If the requested shape is not available, this function emits a @ref
 *  GLFW_CURSOR_UNAVAILABLE error and returns `NULL`.
 *
 *  @param[in] shape One of the [standard shapes](@ref shapes).
 *  @return A new cursor ready to use or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_ENUM, @ref GLFW_CURSOR_UNAVAILABLE and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_standard
 *  @sa @ref glfwCreateCursor
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
GLFWcursor* glfwCreateStandardCursor(int shape);

/*! @brief Destroys a cursor.
 *
 *  This function destroys a cursor previously created with @ref
 *  glfwCreateCursor.  Any remaining cursors will be destroyed by @ref
 *  glfwTerminate.
 *
 *  If the specified cursor is current for any window, that window will be
 *  reverted to the default cursor.  This does not affect the cursor mode.
 *
 *  @param[in] cursor The cursor object to destroy.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_object
 *  @sa @ref glfwCreateCursor
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
void glfwDestroyCursor(GLFWcursor* cursor);

/*! @brief Sets the cursor for the window.
 *
 *  This function sets the cursor image to be used when the cursor is over the
 *  content area of the specified window.  The set cursor will only be visible
 *  when the [cursor mode](@ref cursor_mode) of the window is
 *  `CURSOR_NORMAL`.
 *
 *  On some platforms, the set cursor may not be visible unless the window also
 *  has input focus.
 *
 *  @param[in] window The window to set the cursor for.
 *  @param[in] cursor The cursor to set, or `NULL` to switch back to the default
 *  arrow cursor.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_object
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
void glfwSetCursor(GLFWwindow* window, GLFWcursor* cursor);

/*! @brief Sets the key callback.
 *
 *  This function sets the key callback of the specified window, which is called
 *  when a key is pressed, repeated or released.
 *
 *  The key functions deal with physical keys, with layout independent
 *  [key tokens](@ref keys) named after their values in the standard US keyboard
 *  layout.  If you want to input text, use the
 *  [character callback](@ref glfwSetCharCallback) instead.
 *
 *  When a window loses input focus, it will generate synthetic key release
 *  events for all pressed keys with associated key tokens.  You can tell these
 *  events from user-generated events by the fact that the synthetic ones are
 *  generated after the focus loss event has been processed, i.e. after the
 *  [window focus callback](@ref glfwSetWindowFocusCallback) has been called.
 *
 *  The scancode of a key is specific to that platform or sometimes even to that
 *  machine.  Scancodes are intended to allow users to bind keys that don't have
 *  a GLFW key token.  Such keys have `key` set to `KEY_UNKNOWN`, their
 *  state is not saved and so it cannot be queried with @ref glfwGetKey.
 *
 *  Sometimes GLFW needs to generate synthetic key events, in which case the
 *  scancode may be zero.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new key callback, or `NULL` to remove the currently
 *  set callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, int key, int scancode, int action, int mods)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref keyFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_key
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup input
 */
keyFunc glfwSetKeyCallback(GLFWwindow* window, keyFunc callback);

/*! @brief Sets the Unicode character callback.
 *
 *  This function sets the character callback of the specified window, which is
 *  called when a Unicode character is input.
 *
 *  The character callback is intended for Unicode text input.  As it deals with
 *  characters, it is keyboard layout dependent, whereas the
 *  [key callback](@ref glfwSetKeyCallback) is not.  Characters do not map 1:1
 *  to physical keys, as a key may produce zero, one or more characters.  If you
 *  want to know whether a specific physical key was pressed or released, see
 *  the key callback instead.
 *
 *  The character callback behaves as system text input normally does and will
 *  not be called if modifier keys are held down that would prevent normal text
 *  input on that platform, for example a Super (Command) key on macOS or Alt key
 *  on Windows.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, unsigned int codepoint)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref charFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_char
 *
 *  @since Added in version 2.4.
 *  __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup input
 */
charFunc glfwSetCharCallback(GLFWwindow* window, charFunc callback);

/*! @brief Sets the Unicode character with modifiers callback.
 *
 *  This function sets the character with modifiers callback of the specified
 *  window, which is called when a Unicode character is input regardless of what
 *  modifier keys are used.
 *
 *  The character with modifiers callback is intended for implementing custom
 *  Unicode character input.  For regular Unicode text input, see the
 *  [character callback](@ref glfwSetCharCallback).  Like the character
 *  callback, the character with modifiers callback deals with characters and is
 *  keyboard layout dependent.  Characters do not map 1:1 to physical keys, as
 *  a key may produce zero, one or more characters.  If you want to know whether
 *  a specific physical key was pressed or released, see the
 *  [key callback](@ref glfwSetKeyCallback) instead.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or an
 *  [error](@ref error_handling) occurred.
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, unsigned int codepoint, int mods)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref charModsFunc).
 *
 *  @deprecated Scheduled for removal in version 4.0.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_char
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
charModsFunc glfwSetCharModsCallback(GLFWwindow* window, charModsFunc callback);

/*! @brief Sets the mouse button callback.
 *
 *  This function sets the mouse button callback of the specified window, which
 *  is called when a mouse button is pressed or released.
 *
 *  When a window loses input focus, it will generate synthetic mouse button
 *  release events for all pressed mouse buttons with associated button tokens.
 *  You can tell these events from user-generated events by the fact that the
 *  synthetic ones are generated after the focus loss event has been processed,
 *  i.e. after the [window focus callback](@ref glfwSetWindowFocusCallback) has
 *  been called.
 *
 *  The reported `button` value can be higher than `MOUSE_BUTTON_LAST` if
 *  the button does not have an associated [button token](@ref buttons) and the
 *  @ref INPUT_MODE_UNLIMITED_MOUSE_BUTTONS input mode is set.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, int button, int action, int mods)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref mouseButtonFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_mouse_button
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup input
 */
mouseButtonFunc glfwSetMouseButtonCallback(GLFWwindow* window, mouseButtonFunc callback);

/*! @brief Sets the cursor position callback.
 *
 *  This function sets the cursor position callback of the specified window,
 *  which is called when the cursor is moved.  The callback is provided with the
 *  position, in screen coordinates, relative to the upper-left corner of the
 *  content area of the window.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, double xpos, double ypos);
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref cursorPosFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_pos
 *
 *  @since Added in version 3.0.  Replaces `glfwSetMousePosCallback`.
 *
 *  @ingroup input
 */
cursorPosFunc glfwSetCursorPosCallback(GLFWwindow* window, cursorPosFunc callback);

/*! @brief Sets the cursor enter/leave callback.
 *
 *  This function sets the cursor boundary crossing callback of the specified
 *  window, which is called when the cursor enters or leaves the content area of
 *  the window.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, int entered)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref cursorEnterFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_enter
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup input
 */
cursorEnterFunc glfwSetCursorEnterCallback(GLFWwindow* window, cursorEnterFunc callback);

/*! @brief Sets the scroll callback.
 *
 *  This function sets the scroll callback of the specified window, which is
 *  called when a scrolling device is used, such as a mouse wheel or scrolling
 *  area of a touchpad.
 *
 *  The scroll callback receives all scrolling input, like that from a mouse
 *  wheel or a touchpad scrolling area.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new scroll callback, or `NULL` to remove the
 *  currently set callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, double xoffset, double yoffset)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref scrollFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref scrolling
 *
 *  @since Added in version 3.0.  Replaces `glfwSetMouseWheelCallback`.
 *
 *  @ingroup input
 */
scrollFunc glfwSetScrollCallback(GLFWwindow* window, scrollFunc callback);

/*! @brief Sets the path drop callback.
 *
 *  This function sets the path drop callback of the specified window, which is
 *  called when one or more dragged paths are dropped on the window.
 *
 *  Because the path array and its strings may have been generated specifically
 *  for that event, they are not guaranteed to be valid after the callback has
 *  returned.  If you wish to use them after the callback returns, you need to
 *  make a deep copy.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new file drop callback, or `NULL` to remove the
 *  currently set callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(GLFWwindow* window, int path_count, const char* paths[])
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref dropFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref path_drop
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
dropFunc glfwSetDropCallback(GLFWwindow* window, dropFunc callback);

/*! @brief Sets the clipboard to the specified string.
 *
 *  This function sets the system clipboard to the specified, UTF-8 encoded
 *  string.
 *
 *  @param[in] string A UTF-8 encoded string.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @remark __Win32:__ The clipboard on Windows has a single global lock for reading and
 *  writing.  GLFW tries to acquire it a few times, which is almost always enough.  If it
 *  cannot acquire the lock then this function emits @ref ERR_PLATFORM_ERROR and returns.
 *  It is safe to try this multiple times.
 *
 *  @pointer_lifetime The specified string is copied before this function
 *  returns.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref clipboard
 *  @sa @ref glfwGetClipboardString
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup input
 */
void setClipboardString(const char* string);

/*! @brief Returns the contents of the clipboard as a string.
 *
 *  This function returns the contents of the system clipboard, if it contains
 *  or is convertible to a UTF-8 encoded string.  If the clipboard is empty or
 *  if its contents cannot be converted, `NULL` is returned and a @ref
 *  ERR_FORMAT_UNAVAILABLE error is generated.
 *
 *  @return The contents of the clipboard as a UTF-8 encoded string, or `NULL`
 *  if an [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_FORMAT_UNAVAILABLE and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark __Win32:__ The clipboard on Windows has a single global lock for reading and
 *  writing.  GLFW tries to acquire it a few times, which is almost always enough.  If it
 *  cannot acquire the lock then this function emits @ref ERR_PLATFORM_ERROR and returns.
 *  It is safe to try this multiple times.
 *
 *  @pointer_lifetime The returned string is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is valid until the next call to @ref
 *  glfwGetClipboardString or @ref glfwSetClipboardString, or until the library
 *  is terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref clipboard
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup input
 */
const char* getClipboardString(void);

/*! @brief Makes the context of the specified window current for the calling
 *  thread.
 *
 *  This function makes the OpenGL or OpenGL ES context of the specified window
 *  current on the calling thread.  It can also detach the current context from
 *  the calling thread without making a new one current by passing in `NULL`.
 *
 *  A context must only be made current on a single thread at a time and each
 *  thread can have only a single current context at a time.  Making a context
 *  current detaches any previously current context on the calling thread.
 *
 *  When moving a context between threads, you must detach it (make it
 *  non-current) on the old thread before making it current on the new one.
 *
 *  By default, making a context non-current implicitly forces a pipeline flush.
 *  On machines that support `GL_KHR_context_flush_control`, you can control
 *  whether a context performs this flush by setting the
 *  [WINDOW_ATTR_HINT_CONTEXT_RELEASE_BEHAVIOR](@ref GLFW_CONTEXT_RELEASE_BEHAVIOR_hint)
 *  hint.
 *
 *  The specified window must have an OpenGL or OpenGL ES context.  Specifying
 *  a window without a context will generate a @ref ERR_NO_WINDOW_CONTEXT
 *  error.
 *
 *  @param[in] window The window whose context to make current, or `NULL` to
 *  detach the current context.
 *
 *  @remarks If the previously current context was created via a different
 *  context creation API than the one passed to this function, GLFW will still
 *  detach the previous one from its API before making the new one current.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_NO_WINDOW_CONTEXT and @ref ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref context_current
 *  @sa @ref glfwGetCurrentContext
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup context
 */
void glfwMakeContextCurrent(GLFWwindow* window);

/*! @brief Returns the window whose context is current on the calling thread.
 *
 *  This function returns the window whose OpenGL or OpenGL ES context is
 *  current on the calling thread.
 *
 *  @return The window whose context is current, or `NULL` if no window's
 *  context is current.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref context_current
 *  @sa @ref glfwMakeContextCurrent
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup context
 */
GLFWwindow* glfwGetCurrentContext(void);

/*! @brief Swaps the front and back buffers of the specified window.
 *
 *  This function swaps the front and back buffers of the specified window when
 *  rendering with OpenGL.  If the swap interval is greater than
 *  zero, the GPU driver waits the specified number of screen updates before
 *  swapping the buffers.
 *
 *  The specified window must have an OpenGL context.  Specifying
 *  a window without a context will generate a @ref ERR_NO_WINDOW_CONTEXT
 *  error.
 *
 *  @param[in] window The window whose buffers to swap.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_NO_WINDOW_CONTEXT and @ref ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref buffer_swap
 *  @sa @ref glfwSwapInterval
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwSwapBuffers(GLFWwindow* window);

/*! @brief Sets the swap interval for the current context.
 *
 *  This function sets the swap interval for the current OpenGL or OpenGL ES
 *  context, i.e. the number of screen updates to wait from the time @ref
 *  glfwSwapBuffers was called before swapping the buffers and returning.  This
 *  is sometimes called _vertical synchronization_, _vertical retrace
 *  synchronization_ or just _vsync_.
 *
 *  A context that supports either of the `WGL_EXT_swap_control_tear` and
 *  `GLX_EXT_swap_control_tear` extensions also accepts _negative_ swap
 *  intervals, which allows the driver to swap immediately even if a frame
 *  arrives a little bit late.  You can check for these extensions with @ref
 *  glfwExtensionSupported.
 *
 *  A context must be current on the calling thread.  Calling this function
 *  without a current context will cause a @ref ERR_NO_CURRENT_CONTEXT error.
 *
 *  @param[in] interval The minimum number of screen updates to wait for
 *  until the buffers are swapped by @ref glfwSwapBuffers.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_NO_CURRENT_CONTEXT and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark This function is not called during context creation, leaving the
 *  swap interval set to whatever is the default for that API.  This is done
 *  because some swap interval extensions used by GLFW do not allow the swap
 *  interval to be reset to zero once it has been set to a non-zero value.
 *
 *  @remark Some GPU drivers do not honor the requested swap interval, either
 *  because of a user setting that overrides the application's request or due to
 *  bugs in the driver.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref buffer_swap
 *  @sa @ref glfwSwapBuffers
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup context
 */
void glfwSwapInterval(int interval);

/*! @brief Returns whether the specified extension is available.
 *
 *  This function returns whether the specified
 *  [API extension](@ref context_glext) is supported by the current OpenGL or
 *  OpenGL ES context.  It searches both for client API extension and context
 *  creation API extensions.
 *
 *  A context must be current on the calling thread.  Calling this function
 *  without a current context will cause a @ref ERR_NO_CURRENT_CONTEXT error.
 *
 *  As this functions retrieves and searches one or more extension strings each
 *  call, it is recommended that you cache its results if it is going to be used
 *  frequently.  The extension strings will not change during the lifetime of
 *  a context, so there is no danger in doing this.
 *
 *  @param[in] extension The ASCII encoded name of the extension.
 *  @return `true` if the extension is available, or `false`
 *  otherwise.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_NO_CURRENT_CONTEXT, @ref ERR_INVALID_VALUE and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref context_glext
 *  @sa @ref glfwGetProcAddress
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup context
 */
int glfwExtensionSupported(const char* extension);

/*! @brief Returns the address of the specified function for the current
 *  context.
 *
 *  This function returns the address of the specified OpenGL or OpenGL ES
 *  [core or extension function](@ref context_glext), if it is supported
 *  by the current context.
 *
 *  A context must be current on the calling thread.  Calling this function
 *  without a current context will cause a @ref ERR_NO_CURRENT_CONTEXT error.
 *
 *  @param[in] procname The ASCII encoded name of the function.
 *  @return The address of the function, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_NO_CURRENT_CONTEXT and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark The address of a given function is not guaranteed to be the same
 *  between contexts.
 *
 *  @remark This function may return a non-`NULL` address despite the
 *  associated version or extension not being available.  Always check the
 *  context version or extension string first.
 *
 *  @pointer_lifetime The returned function pointer is valid until the context
 *  is destroyed or the library is terminated.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref context_glext
 *  @sa @ref glfwExtensionSupported
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup context
 */
glFunc glfwGetProcAddress(const char* procname);





// --------- Internal API below ---------

void _terminate(void);

ErrorResponse* platformInit(_GLFWplatform* platform);
void platformTerminate(void);

//////////////////////////////////////////////////////////////////////////
//////                       GLFW platform API                      //////
//////////////////////////////////////////////////////////////////////////

void* _glfwPlatformLoadModule(const char* path);
void _glfwPlatformFreeModule(void* module);
moduleFunc _glfwPlatformGetModuleSymbol(void* module, const char* name);


//////////////////////////////////////////////////////////////////////////
//////                         GLFW event API                       //////
//////////////////////////////////////////////////////////////////////////

void _glfwInputWindowFocus(_GLFWwindow* window, GLFWbool focused);
void _glfwInputWindowPos(_GLFWwindow* window, int xpos, int ypos);
void _glfwInputWindowSize(_GLFWwindow* window, int width, int height);
void _glfwInputFramebufferSize(_GLFWwindow* window, int width, int height);
void _glfwInputWindowContentScale(_GLFWwindow* window,
                                  float xscale, float yscale);
void _glfwInputWindowIconify(_GLFWwindow* window, GLFWbool iconified);
void _glfwInputWindowMaximize(_GLFWwindow* window, GLFWbool maximized);
void _glfwInputWindowDamage(_GLFWwindow* window);
void _glfwInputWindowCloseRequest(_GLFWwindow* window);
void _glfwInputWindowMonitor(_GLFWwindow* window, _GLFWmonitor* monitor);

void _glfwInputKey(_GLFWwindow* window,
                   int key, int scancode, int action, int mods);
void _glfwInputChar(_GLFWwindow* window,
                    uint32_t codepoint, int mods, GLFWbool plain);
void _glfwInputScroll(_GLFWwindow* window, double xoffset, double yoffset);
void _glfwInputMouseClick(_GLFWwindow* window, int button, int action, int mods);
void _glfwInputCursorPos(_GLFWwindow* window, double xpos, double ypos);
void _glfwInputCursorEnter(_GLFWwindow* window, GLFWbool entered);
void _glfwInputDrop(_GLFWwindow* window, int count, const char** names);

void _glfwInputMonitor(_GLFWmonitor* monitor, int action, int placement);
void _glfwInputMonitorWindow(_GLFWmonitor* monitor, _GLFWwindow* window);

#if defined(__GNUC__)
void _glfwInputError(int code, const char* format, ...)
    __attribute__((format(printf, 2, 3)));
ErrorResponse* createErrorResponse(int code, const char* format, ...) __attribute__((format(printf, 2, 3)));
#else
void _glfwInputError(int code, const char* format, ...);
ErrorResponse* createErrorResponse(int code, const char* format, ...);
#endif


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

GLFWbool _glfwStringInExtensionString(const char* string, const char* extensions);
const _GLFWfbconfig* _glfwChooseFBConfig(const _GLFWfbconfig* desired,
                                         const _GLFWfbconfig* alternatives,
                                         unsigned int count);
GLFWbool _glfwRefreshContextAttribs(_GLFWwindow* window,
                                    const _GLFWctxconfig* ctxconfig);
GLFWbool _glfwIsValidContextConfig(const _GLFWctxconfig* ctxconfig);

const VideoMode* _glfwChooseVideoMode(_GLFWmonitor* monitor,
                                        const VideoMode* desired);
int _glfwCompareVideoModes(const VideoMode* first, const VideoMode* second);
_GLFWmonitor* _glfwAllocMonitor(const char* name, int widthMM, int heightMM);
void _glfwFreeMonitor(_GLFWmonitor* monitor);
void _glfwAllocGammaArrays(GammaRamp* ramp, unsigned int size);
void _glfwFreeGammaArrays(GammaRamp* ramp);
void _glfwSplitBPP(int bpp, int* red, int* green, int* blue);

void _glfwCenterCursorInContentArea(_GLFWwindow* window);

size_t _glfwEncodeUTF8(char* s, uint32_t codepoint);
char** _glfwParseUriList(char* text, int* count);

char* _glfw_strdup(const char* src);
int _glfw_min(int a, int b);
int _glfw_max(int a, int b);

void* _glfw_calloc(size_t count, size_t size);
void* _glfw_realloc(void* pointer, size_t size);
void _glfw_free(void* pointer);

void _glfwTerminateGLX(void);

#ifdef __cplusplus
}
#endif
