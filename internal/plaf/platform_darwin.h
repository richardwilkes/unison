#if defined(__APPLE__)
#include <stdint.h>

// NOTE: All of NSGL was deprecated in the 10.14 SDK
//       This disables the pointless warnings for every symbol we use
#ifndef GL_SILENCE_DEPRECATION
#define GL_SILENCE_DEPRECATION
#endif

#import <Cocoa/Cocoa.h>

#define GLFW_COCOA_LIBRARY_WINDOW_STATE _GLFWlibraryNS ns;

#define GLFW_NSGL_CONTEXT_STATE         _GLFWcontextNSGL nsgl;

// NSGL-specific per-context data
//
typedef struct _GLFWcontextNSGL {
    NSOpenGLPixelFormat* pixelFormat;
    NSOpenGLContext*     object;
} _GLFWcontextNSGL;

// Cocoa-specific per-window data
//
typedef struct _GLFWwindowNS
{
    NSWindow *  object;
    NSObject *  delegate;
    NSView *    view;
    // id              layer;

    IntBool        maximized;
    IntBool        scaleFramebuffer;

    // Cached window properties to filter out duplicate events
    int             width, height;
    int             fbWidth, fbHeight;
    float           xscale, yscale;

    // The total sum of the distances the cursor has been warped
    // since the last cursor motion event was processed
    // This is kept to counteract Cocoa doing the same internally
    double          cursorWarpDeltaX, cursorWarpDeltaY;
} _GLFWwindowNS;

// Cocoa-specific global data
//
typedef struct _GLFWlibraryNS
{
    CGEventSourceRef    eventSource;
    id                  delegate;
    IntBool            cursorHidden;
    id                  keyUpMonitor;
    id                  nibObjects;

    char                keynames[KEY_LAST + 1][17];
    short int           keycodes[256];
    short int           scancodes[KEY_LAST + 1];
    CGPoint             cascadePoint;
    // Where to place the cursor when re-enabled
    double              restoreCursorPosX, restoreCursorPosY;
} _GLFWlibraryNS;

IntBool _glfwCreateWindowCocoa(plafWindow* window, const WindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig);
void _glfwDestroyWindowCocoa(plafWindow* window);
void _glfwSetWindowTitleCocoa(plafWindow* window, const char* title);
void _glfwSetWindowIconCocoa(plafWindow* window, int count, const ImageData* images);
void _glfwGetWindowPosCocoa(plafWindow* window, int* xpos, int* ypos);
void _glfwSetWindowPosCocoa(plafWindow* window, int xpos, int ypos);
void _glfwGetWindowSizeCocoa(plafWindow* window, int* width, int* height);
void _glfwSetWindowSizeCocoa(plafWindow* window, int width, int height);
void _glfwSetWindowSizeLimitsCocoa(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight);
void _glfwSetWindowAspectRatioCocoa(plafWindow* window, int numer, int denom);
void _glfwGetFramebufferSizeCocoa(plafWindow* window, int* width, int* height);
void _glfwGetWindowFrameSizeCocoa(plafWindow* window, int* left, int* top, int* right, int* bottom);
void _glfwGetWindowContentScaleCocoa(plafWindow* window, float* xscale, float* yscale);
void _glfwIconifyWindowCocoa(plafWindow* window);
void _glfwRestoreWindowCocoa(plafWindow* window);
void _glfwMaximizeWindowCocoa(plafWindow* window);
void _glfwShowWindowCocoa(plafWindow* window);
void _glfwHideWindowCocoa(plafWindow* window);
void _glfwRequestWindowAttentionCocoa(plafWindow* window);
void _glfwFocusWindowCocoa(plafWindow* window);
void _glfwSetWindowMonitorCocoa(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate);
IntBool _glfwWindowFocusedCocoa(plafWindow* window);
IntBool _glfwWindowIconifiedCocoa(plafWindow* window);
IntBool _glfwWindowVisibleCocoa(plafWindow* window);
IntBool _glfwWindowMaximizedCocoa(plafWindow* window);
IntBool _glfwWindowHoveredCocoa(plafWindow* window);
IntBool _glfwFramebufferTransparentCocoa(plafWindow* window);
void _glfwSetWindowResizableCocoa(plafWindow* window, IntBool enabled);
void _glfwSetWindowDecoratedCocoa(plafWindow* window, IntBool enabled);
void _glfwSetWindowFloatingCocoa(plafWindow* window, IntBool enabled);
float _glfwGetWindowOpacityCocoa(plafWindow* window);
void _glfwSetWindowOpacityCocoa(plafWindow* window, float opacity);
void _glfwSetWindowMousePassthroughCocoa(plafWindow* window, IntBool enabled);

void _glfwPollEventsCocoa(void);
void _glfwWaitEventsCocoa(void);
void _glfwWaitEventsTimeoutCocoa(double timeout);
void _glfwPostEmptyEventCocoa(void);

void _glfwSetCursorModeCocoa(plafWindow* window, int mode);
int _glfwGetKeyScancodeCocoa(int key);
IntBool _glfwCreateCursorCocoa(plafCursor* cursor, const ImageData* image, int xhot, int yhot);
IntBool _glfwCreateStandardCursorCocoa(plafCursor* cursor, int shape);
void _glfwDestroyCursorCocoa(plafCursor* cursor);

void _glfwFreeMonitorCocoa(plafMonitor* monitor);
void _glfwGetMonitorPosCocoa(plafMonitor* monitor, int* xpos, int* ypos);
void _glfwGetMonitorContentScaleCocoa(plafMonitor* monitor, float* xscale, float* yscale);
void _glfwGetMonitorWorkareaCocoa(plafMonitor* monitor, int* xpos, int* ypos, int* width, int* height);
VideoMode* _glfwGetVideoModesCocoa(plafMonitor* monitor, int* count);
IntBool _glfwGetVideoModeCocoa(plafMonitor* monitor, VideoMode* mode);
IntBool _glfwGetGammaRampCocoa(plafMonitor* monitor, GammaRamp* ramp);
void _glfwSetGammaRampCocoa(plafMonitor* monitor, const GammaRamp* ramp);

void _glfwPollMonitorsCocoa(void);
void _glfwSetVideoModeCocoa(plafMonitor* monitor, const VideoMode* desired);
void _glfwRestoreVideoModeCocoa(plafMonitor* monitor);

float _glfwTransformYCocoa(float y);

IntBool _glfwInitNSGL(void);
void _glfwTerminateNSGL(void);
IntBool _glfwCreateContextNSGL(plafWindow* window,
                                const plafCtxCfg* ctxconfig,
                                const plafFrameBufferCfg* fbconfig);
void _glfwDestroyContextNSGL(plafWindow* window);

#endif // __APPLE__