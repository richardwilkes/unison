#include "platform.h"

#include <string.h>
#include <float.h>


//////////////////////////////////////////////////////////////////////////
//////                         GLFW event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code that a window has lost or received input focus
//
void _glfwInputWindowFocus(_GLFWwindow* window, IntBool focused)
{
    if (window->callbacks.focus)
        window->callbacks.focus((GLFWwindow*) window, focused);

    if (!focused)
    {
        int key, button;

        for (key = 0;  key <= KEY_LAST;  key++)
        {
            if (window->keys[key] == INPUT_PRESS)
            {
                const int scancode = _glfw.platform.getKeyScancode(key);
                _glfwInputKey(window, key, scancode, INPUT_RELEASE, 0);
            }
        }

        for (button = 0;  button <= MOUSE_BUTTON_LAST;  button++)
        {
            if (window->mouseButtons[button] == INPUT_PRESS)
                _glfwInputMouseClick(window, button, INPUT_RELEASE, 0);
        }
    }
}

// Notifies shared code that a window has moved
// The position is specified in content area relative screen coordinates
//
void _glfwInputWindowPos(_GLFWwindow* window, int x, int y)
{
    if (window->callbacks.pos)
        window->callbacks.pos((GLFWwindow*) window, x, y);
}

// Notifies shared code that a window has been resized
// The size is specified in screen coordinates
//
void _glfwInputWindowSize(_GLFWwindow* window, int width, int height)
{
    if (window->callbacks.size)
        window->callbacks.size((GLFWwindow*) window, width, height);
}

// Notifies shared code that a window has been iconified or restored
//
void _glfwInputWindowIconify(_GLFWwindow* window, IntBool iconified)
{
    if (window->callbacks.iconify)
        window->callbacks.iconify((GLFWwindow*) window, iconified);
}

// Notifies shared code that a window has been maximized or restored
//
void _glfwInputWindowMaximize(_GLFWwindow* window, IntBool maximized)
{
    if (window->callbacks.maximize)
        window->callbacks.maximize((GLFWwindow*) window, maximized);
}

// Notifies shared code that a window framebuffer has been resized
// The size is specified in pixels
//
void _glfwInputFramebufferSize(_GLFWwindow* window, int width, int height)
{
    if (window->callbacks.fbsize)
        window->callbacks.fbsize((GLFWwindow*) window, width, height);
}

// Notifies shared code that a window content scale has changed
// The scale is specified as the ratio between the current and default DPI
//
void _glfwInputWindowContentScale(_GLFWwindow* window, float xscale, float yscale)
{
    if (window->callbacks.scale)
        window->callbacks.scale((GLFWwindow*) window, xscale, yscale);
}

// Notifies shared code that the window contents needs updating
//
void _glfwInputWindowDamage(_GLFWwindow* window)
{
    if (window->callbacks.refresh)
        window->callbacks.refresh((GLFWwindow*) window);
}

// Notifies shared code that the user wishes to close a window
//
void _glfwInputWindowCloseRequest(_GLFWwindow* window)
{
    window->shouldClose = true;

    if (window->callbacks.close)
        window->callbacks.close((GLFWwindow*) window);
}

// Notifies shared code that a window has changed its desired monitor
//
void _glfwInputWindowMonitor(_GLFWwindow* window, _GLFWmonitor* monitor)
{
    window->monitor = monitor;
}

//////////////////////////////////////////////////////////////////////////
//////                        GLFW public API                       //////
//////////////////////////////////////////////////////////////////////////

GLFWwindow* glfwCreateWindow(int width, int height,
                                     const char* title,
                                     GLFWmonitor* monitor,
                                     GLFWwindow* share)
{
    _GLFWfbconfig fbconfig;
    _GLFWctxconfig ctxconfig;
    WindowConfig wndconfig;
    _GLFWwindow* window;

    if (width <= 0 || height <= 0)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid window size %ix%i", width, height);

        return NULL;
    }

    fbconfig  = _glfw.hints.framebuffer;
    ctxconfig = _glfw.hints.context;
    wndconfig = _glfw.hints.window;

    wndconfig.width   = width;
    wndconfig.height  = height;
    ctxconfig.share   = (_GLFWwindow*) share;

    if (!_glfwIsValidContextConfig(&ctxconfig))
        return NULL;

    window = _glfw_calloc(1, sizeof(_GLFWwindow));
    window->next = _glfw.windowListHead;
    _glfw.windowListHead = window;

    window->videoMode.width       = width;
    window->videoMode.height      = height;
    window->videoMode.redBits     = fbconfig.redBits;
    window->videoMode.greenBits   = fbconfig.greenBits;
    window->videoMode.blueBits    = fbconfig.blueBits;
    window->videoMode.refreshRate = _glfw.hints.refreshRate;

    window->monitor          = (_GLFWmonitor*) monitor;
    window->resizable        = wndconfig.resizable;
    window->decorated        = wndconfig.decorated;
    window->floating         = wndconfig.floating;
    window->mousePassthrough = wndconfig.mousePassthrough;
    window->cursorMode       = CURSOR_NORMAL;

    window->doublebuffer = fbconfig.doublebuffer;

    window->minwidth    = DONT_CARE;
    window->minheight   = DONT_CARE;
    window->maxwidth    = DONT_CARE;
    window->maxheight   = DONT_CARE;
    window->numer       = DONT_CARE;
    window->denom       = DONT_CARE;
    window->title       = _glfw_strdup(title);

    if (!_glfw.platform.createWindow(window, &wndconfig, &ctxconfig, &fbconfig))
    {
        glfwDestroyWindow((GLFWwindow*) window);
        return NULL;
    }

    return (GLFWwindow*) window;
}

void glfwDefaultWindowHints(void)
{
    // The default is OpenGL with minimum version 1.0
    memset(&_glfw.hints.context, 0, sizeof(_glfw.hints.context));
    _glfw.hints.context.major  = 3;
    _glfw.hints.context.minor  = 2;
#if defined(PLATFORM_DARWIN)
	// These don't appear to be necessary to set on macOS anymore, but keeping for now
	_glfw.hints.context.forward = true;
	_glfw.hints.context.profile = OPENGL_PROFILE_CORE;
#endif

    // The default is a resizable window with decorations
    memset(&_glfw.hints.window, 0, sizeof(_glfw.hints.window));
    _glfw.hints.window.resizable    = true;
    _glfw.hints.window.decorated    = true;
    _glfw.hints.window.xpos         = ANY_POSITION;
    _glfw.hints.window.ypos         = ANY_POSITION;
    _glfw.hints.window.scaleFramebuffer = true;

    // The default is 24 bits of color, 24 bits of depth and 8 bits of stencil, double buffered
    memset(&_glfw.hints.framebuffer, 0, sizeof(_glfw.hints.framebuffer));
    _glfw.hints.framebuffer.redBits      = 8;
    _glfw.hints.framebuffer.greenBits    = 8;
    _glfw.hints.framebuffer.blueBits     = 8;
    _glfw.hints.framebuffer.alphaBits    = 8;
    _glfw.hints.framebuffer.depthBits    = 24;
    _glfw.hints.framebuffer.stencilBits  = 8;
    _glfw.hints.framebuffer.doublebuffer = true;

    // The default is to select the highest available refresh rate
    _glfw.hints.refreshRate = DONT_CARE;
}

void glfwWindowHint(int hint, int value)
{
    switch (hint)
    {
        case WINDOW_HINT_RED_BITS:
            _glfw.hints.framebuffer.redBits = value;
            return;
        case WINDOW_HINT_GREEN_BITS:
            _glfw.hints.framebuffer.greenBits = value;
            return;
        case WINDOW_HINT_BLUE_BITS:
            _glfw.hints.framebuffer.blueBits = value;
            return;
        case WINDOW_HINT_ALPHA_BITS:
            _glfw.hints.framebuffer.alphaBits = value;
            return;
        case WINDOW_HINT_DEPTH_BITS:
            _glfw.hints.framebuffer.depthBits = value;
            return;
        case WINDOW_HINT_STENCIL_BITS:
            _glfw.hints.framebuffer.stencilBits = value;
            return;
        case WINDOW_HINT_ACCUM_RED_BITS:
            _glfw.hints.framebuffer.accumRedBits = value;
            return;
        case WINDOW_HINT_ACCUM_GREEN_BITS:
            _glfw.hints.framebuffer.accumGreenBits = value;
            return;
        case WINDOW_HINT_ACCUM_BLUE_BITS:
            _glfw.hints.framebuffer.accumBlueBits = value;
            return;
        case WINDOW_HINT_ACCUM_ALPHA_BITS:
            _glfw.hints.framebuffer.accumAlphaBits = value;
            return;
        case WINDOW_HINT_AUX_BUFFERS:
            _glfw.hints.framebuffer.auxBuffers = value;
            return;
        case WINDOW_ATTR_HINT_DOUBLE_BUFFER:
            _glfw.hints.framebuffer.doublebuffer = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_TRANSPARENT_FRAMEBUFFER:
            _glfw.hints.framebuffer.transparent = value ? true : false;
            return;
        case WINDOW_HINT_SAMPLES:
            _glfw.hints.framebuffer.samples = value;
            return;
        case WINDOW_HINT_SRGB_CAPABLE:
            _glfw.hints.framebuffer.sRGB = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_RESIZABLE:
            _glfw.hints.window.resizable = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_DECORATED:
            _glfw.hints.window.decorated = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_FLOATING:
            _glfw.hints.window.floating = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_MAXIMIZED:
            _glfw.hints.window.maximized = value ? true : false;
            return;
        case WINDOW_HINT_POSITION_X:
            _glfw.hints.window.xpos = value;
            return;
        case WINDOW_HINT_POSITION_Y:
            _glfw.hints.window.ypos = value;
            return;
        case WINDOW_HINT_SCALE_TO_MONITOR:
            _glfw.hints.window.scaleToMonitor = value ? true : false;
            return;
        case WINDOW_HINT_SCALE_FRAMEBUFFER:
            _glfw.hints.window.scaleFramebuffer = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH:
            _glfw.hints.window.mousePassthrough = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_VERSION_MAJOR:
            _glfw.hints.context.major = value;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_VERSION_MINOR:
            _glfw.hints.context.minor = value;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_ROBUSTNESS:
            _glfw.hints.context.robustness = value;
            return;
        case WINDOW_ATTR_HINT_OPENGL_FORWARD_COMPAT:
            _glfw.hints.context.forward = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_DEBUG:
            _glfw.hints.context.debug = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_ERROR_SUPPRESSION:
            _glfw.hints.context.noerror = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_OPENGL_PROFILE:
            _glfw.hints.context.profile = value;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_RELEASE_BEHAVIOR:
            _glfw.hints.context.release = value;
            return;
        case WINDOW_HINT_REFRESH_RATE:
            _glfw.hints.refreshRate = value;
            return;
    }

    _glfwInputError(ERR_INVALID_ENUM, "Invalid window hint 0x%08X", hint);
}

void glfwDestroyWindow(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;

    // Allow closing of NULL (to match the behavior of free)
    if (window == NULL)
        return;

    // Clear all callbacks to avoid exposing a half torn-down window object
    memset(&window->callbacks, 0, sizeof(window->callbacks));

    // The window's context must not be current when the window is destroyed
    if (window == _glfw.contextSlot)
        glfwMakeContextCurrent(NULL);

    _glfw.platform.destroyWindow(window);

    // Unlink window from global linked list
    {
        _GLFWwindow** prev = &_glfw.windowListHead;

        while (*prev != window)
            prev = &((*prev)->next);

        *prev = window->next;
    }

    _glfw_free(window->title);
    _glfw_free(window);
}

int glfwWindowShouldClose(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    return window->shouldClose;
}

void glfwSetWindowShouldClose(GLFWwindow* handle, int value)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    window->shouldClose = value;
}

const char* glfwGetWindowTitle(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    return window->title;
}

void glfwSetWindowTitle(GLFWwindow* handle, const char* title)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;

    char* prev = window->title;
    window->title = _glfw_strdup(title);

    _glfw.platform.setWindowTitle(window, title);
    _glfw_free(prev);
}

void glfwSetWindowIcon(GLFWwindow* handle,
                               int count, const ImageData* images)
{
    int i;

    _GLFWwindow* window = (_GLFWwindow*) handle;

    if (count < 0)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid image count for window icon");
        return;
    }

    for (i = 0; i < count; i++)
    {
        if (images[i].width <= 0 || images[i].height <= 0)
        {
            _glfwInputError(ERR_INVALID_VALUE, "Invalid image dimensions for window icon");
            return;
        }
    }

    _glfw.platform.setWindowIcon(window, count, images);
}

void glfwGetWindowPos(GLFWwindow* handle, int* xpos, int* ypos)
{
    if (xpos)
        *xpos = 0;
    if (ypos)
        *ypos = 0;

    _GLFWwindow* window = (_GLFWwindow*) handle;
    _glfw.platform.getWindowPos(window, xpos, ypos);
}

void glfwSetWindowPos(GLFWwindow* handle, int xpos, int ypos)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (window->monitor)
        return;
    _glfw.platform.setWindowPos(window, xpos, ypos);
}

void glfwGetWindowSize(GLFWwindow* handle, int* width, int* height)
{
    if (width)
        *width = 0;
    if (height)
        *height = 0;
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _glfw.platform.getWindowSize(window, width, height);
}

void glfwSetWindowSize(GLFWwindow* handle, int width, int height)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    window->videoMode.width  = width;
    window->videoMode.height = height;
    _glfw.platform.setWindowSize(window, width, height);
}

void glfwSetWindowSizeLimits(GLFWwindow* handle,
                                     int minwidth, int minheight,
                                     int maxwidth, int maxheight)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (minwidth != DONT_CARE && minheight != DONT_CARE)
    {
        if (minwidth < 0 || minheight < 0)
        {
            _glfwInputError(ERR_INVALID_VALUE, "Invalid window minimum size %ix%i", minwidth, minheight);
            return;
        }
    }

    if (maxwidth != DONT_CARE && maxheight != DONT_CARE)
    {
        if (maxwidth < 0 || maxheight < 0 ||
            maxwidth < minwidth || maxheight < minheight)
        {
            _glfwInputError(ERR_INVALID_VALUE, "Invalid window maximum size %ix%i", maxwidth, maxheight);
            return;
        }
    }

    window->minwidth  = minwidth;
    window->minheight = minheight;
    window->maxwidth  = maxwidth;
    window->maxheight = maxheight;

    if (window->monitor || !window->resizable)
        return;

    _glfw.platform.setWindowSizeLimits(window,
                                       minwidth, minheight,
                                       maxwidth, maxheight);
}

void glfwSetWindowAspectRatio(GLFWwindow* handle, int numer, int denom)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (numer != DONT_CARE && denom != DONT_CARE)
    {
        if (numer <= 0 || denom <= 0)
        {
            _glfwInputError(ERR_INVALID_VALUE, "Invalid window aspect ratio %i:%i", numer, denom);
            return;
        }
    }

    window->numer = numer;
    window->denom = denom;

    if (window->monitor || !window->resizable)
        return;

    _glfw.platform.setWindowAspectRatio(window, numer, denom);
}

void glfwGetFramebufferSize(GLFWwindow* handle, int* width, int* height)
{
    if (width)
        *width = 0;
    if (height)
        *height = 0;
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _glfw.platform.getFramebufferSize(window, width, height);
}

void glfwGetWindowFrameSize(GLFWwindow* handle,
                                    int* left, int* top,
                                    int* right, int* bottom)
{
    if (left)
        *left = 0;
    if (top)
        *top = 0;
    if (right)
        *right = 0;
    if (bottom)
        *bottom = 0;
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _glfw.platform.getWindowFrameSize(window, left, top, right, bottom);
}

void glfwGetWindowContentScale(GLFWwindow* handle,
                                       float* xscale, float* yscale)
{
    if (xscale)
        *xscale = 0.f;
    if (yscale)
        *yscale = 0.f;
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _glfw.platform.getWindowContentScale(window, xscale, yscale);
}

float glfwGetWindowOpacity(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    return _glfw.platform.getWindowOpacity(window);
}

void glfwSetWindowOpacity(GLFWwindow* handle, float opacity)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (opacity != opacity || opacity < 0.f || opacity > 1.f)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid window opacity %f", opacity);
        return;
    }

    _glfw.platform.setWindowOpacity(window, opacity);
}

void glfwIconifyWindow(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _glfw.platform.iconifyWindow(window);
}

void glfwRestoreWindow(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _glfw.platform.restoreWindow(window);
}

void glfwMaximizeWindow(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (window->monitor)
        return;

    _glfw.platform.maximizeWindow(window);
}

void glfwShowWindow(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (!window->monitor) {
	    _glfw.platform.showWindow(window);
	}
}

void glfwRequestWindowAttention(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _glfw.platform.requestWindowAttention(window);
}

void glfwHideWindow(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (window->monitor)
        return;

    _glfw.platform.hideWindow(window);
}

void glfwFocusWindow(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _glfw.platform.focusWindow(window);
}

int glfwGetWindowAttrib(GLFWwindow* handle, int attrib)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    switch (attrib)
    {
        case WINDOW_ATTR_FOCUSED:
            return _glfw.platform.windowFocused(window);
        case WINDOW_ATTR_ICONIFIED:
            return _glfw.platform.windowIconified(window);
        case WINDOW_ATTR_VISIBLE:
            return _glfw.platform.windowVisible(window);
        case WINDOW_ATTR_HINT_MAXIMIZED:
            return _glfw.platform.windowMaximized(window);
        case WINDOW_ATTR_HOVERED:
            return _glfw.platform.windowHovered(window);
        case WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH:
            return window->mousePassthrough;
        case WINDOW_ATTR_HINT_TRANSPARENT_FRAMEBUFFER:
            return _glfw.platform.framebufferTransparent(window);
        case WINDOW_ATTR_HINT_RESIZABLE:
            return window->resizable;
        case WINDOW_ATTR_HINT_DECORATED:
            return window->decorated;
        case WINDOW_ATTR_HINT_FLOATING:
            return window->floating;
        case WINDOW_ATTR_HINT_DOUBLE_BUFFER:
            return window->doublebuffer;
        case WINDOW_ATTR_HINT_CONTEXT_VERSION_MAJOR:
            return window->context.major;
        case WINDOW_ATTR_HINT_CONTEXT_VERSION_MINOR:
            return window->context.minor;
        case WINDOW_ATTR_CONTEXT_REVISION:
            return window->context.revision;
        case WINDOW_ATTR_HINT_CONTEXT_ROBUSTNESS:
            return window->context.robustness;
        case WINDOW_ATTR_HINT_OPENGL_FORWARD_COMPAT:
            return window->context.forward;
        case WINDOW_ATTR_HINT_CONTEXT_DEBUG:
            return window->context.debug;
        case WINDOW_ATTR_HINT_OPENGL_PROFILE:
            return window->context.profile;
        case WINDOW_ATTR_HINT_CONTEXT_RELEASE_BEHAVIOR:
            return window->context.release;
        case WINDOW_ATTR_HINT_CONTEXT_ERROR_SUPPRESSION:
            return window->context.noerror;
    }

    _glfwInputError(ERR_INVALID_ENUM, "Invalid window attribute 0x%08X", attrib);
    return 0;
}

void glfwSetWindowAttrib(GLFWwindow* handle, int attrib, int value)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    value = value ? true : false;

    switch (attrib)
    {
        case WINDOW_ATTR_HINT_RESIZABLE:
            window->resizable = value;
            if (!window->monitor)
                _glfw.platform.setWindowResizable(window, value);
            return;

        case WINDOW_ATTR_HINT_DECORATED:
            window->decorated = value;
            if (!window->monitor)
                _glfw.platform.setWindowDecorated(window, value);
            return;

        case WINDOW_ATTR_HINT_FLOATING:
            window->floating = value;
            if (!window->monitor)
                _glfw.platform.setWindowFloating(window, value);
            return;

        case WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH:
            window->mousePassthrough = value;
            _glfw.platform.setWindowMousePassthrough(window, value);
            return;
    }

    _glfwInputError(ERR_INVALID_ENUM, "Invalid window attribute 0x%08X", attrib);
}

GLFWmonitor* glfwGetWindowMonitor(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    return (GLFWmonitor*) window->monitor;
}

void glfwSetWindowMonitor(GLFWwindow* wh,
                                  GLFWmonitor* mh,
                                  int xpos, int ypos,
                                  int width, int height,
                                  int refreshRate)
{
    _GLFWwindow* window = (_GLFWwindow*) wh;
    _GLFWmonitor* monitor = (_GLFWmonitor*) mh;
    if (width <= 0 || height <= 0)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid window size %ix%i", width, height);
        return;
    }

    if (refreshRate < 0 && refreshRate != DONT_CARE)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid refresh rate %i", refreshRate);
        return;
    }

    window->videoMode.width       = width;
    window->videoMode.height      = height;
    window->videoMode.refreshRate = refreshRate;

    _glfw.platform.setWindowMonitor(window, monitor,
                                    xpos, ypos, width, height,
                                    refreshRate);
}

void glfwSetWindowUserPointer(GLFWwindow* handle, void* pointer)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    window->userPointer = pointer;
}

void* glfwGetWindowUserPointer(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    return window->userPointer;
}

windowPosFunc glfwSetWindowPosCallback(GLFWwindow* handle,
                                                  windowPosFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(windowPosFunc, window->callbacks.pos, cbfun);
    return cbfun;
}

windowSizeFunc glfwSetWindowSizeCallback(GLFWwindow* handle,
                                                    windowSizeFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(windowSizeFunc, window->callbacks.size, cbfun);
    return cbfun;
}

windowCloseFunc glfwSetWindowCloseCallback(GLFWwindow* handle,
                                                      windowCloseFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(windowCloseFunc, window->callbacks.close, cbfun);
    return cbfun;
}

windowRefreshFunc glfwSetWindowRefreshCallback(GLFWwindow* handle,
                                                          windowRefreshFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(windowRefreshFunc, window->callbacks.refresh, cbfun);
    return cbfun;
}

windowFocusFunc glfwSetWindowFocusCallback(GLFWwindow* handle,
                                                      windowFocusFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(windowFocusFunc, window->callbacks.focus, cbfun);
    return cbfun;
}

windowIconifyFunc glfwSetWindowIconifyCallback(GLFWwindow* handle,
                                                          windowIconifyFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(windowIconifyFunc, window->callbacks.iconify, cbfun);
    return cbfun;
}

windowMaximizeFunc glfwSetWindowMaximizeCallback(GLFWwindow* handle,
                                                            windowMaximizeFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(windowMaximizeFunc, window->callbacks.maximize, cbfun);
    return cbfun;
}

frameBufferSizeFunc glfwSetFramebufferSizeCallback(GLFWwindow* handle,
                                                              frameBufferSizeFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(frameBufferSizeFunc, window->callbacks.fbsize, cbfun);
    return cbfun;
}

windowContextScaleFunc glfwSetWindowContentScaleCallback(GLFWwindow* handle,
                                                                    windowContextScaleFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(windowContextScaleFunc, window->callbacks.scale, cbfun);
    return cbfun;
}

void glfwPollEvents(void)
{
    _glfw.platform.pollEvents();
}

void glfwWaitEvents(void)
{
    _glfw.platform.waitEvents();
}

void glfwWaitEventsTimeout(double timeout)
{
    if (timeout != timeout || timeout < 0.0 || timeout > DBL_MAX)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid time %f", timeout);
        return;
    }

    _glfw.platform.waitEventsTimeout(timeout);
}

void glfwPostEmptyEvent(void)
{
    _glfw.platform.postEmptyEvent();
}
