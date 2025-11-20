#include "platform.h"


//////////////////////////////////////////////////////////////////////////
//////                         GLFW event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code that a window has lost or received input focus
//
void _glfwInputWindowFocus(plafWindow* window, IntBool focused)
{
    if (window->focusCallback)
        window->focusCallback((plafWindow*) window, focused);

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
void _glfwInputWindowPos(plafWindow* window, int x, int y)
{
    if (window->posCallback)
        window->posCallback((plafWindow*) window, x, y);
}

// Notifies shared code that a window has been resized
// The size is specified in screen coordinates
//
void _glfwInputWindowSize(plafWindow* window, int width, int height)
{
    if (window->sizeCallback)
        window->sizeCallback((plafWindow*) window, width, height);
}

// Notifies shared code that a window has been iconified or restored
//
void _glfwInputWindowIconify(plafWindow* window, IntBool iconified)
{
    if (window->iconifyCallback)
        window->iconifyCallback((plafWindow*) window, iconified);
}

// Notifies shared code that a window has been maximized or restored
//
void _glfwInputWindowMaximize(plafWindow* window, IntBool maximized)
{
    if (window->maximizeCallback)
        window->maximizeCallback((plafWindow*) window, maximized);
}

// Notifies shared code that a window framebuffer has been resized
// The size is specified in pixels
//
void _glfwInputFramebufferSize(plafWindow* window, int width, int height)
{
    if (window->fbsizeCallback)
        window->fbsizeCallback((plafWindow*) window, width, height);
}

// Notifies shared code that a window content scale has changed
// The scale is specified as the ratio between the current and default DPI
//
void _glfwInputWindowContentScale(plafWindow* window, float xscale, float yscale)
{
    if (window->scaleCallback)
        window->scaleCallback((plafWindow*) window, xscale, yscale);
}

// Notifies shared code that the window contents needs updating
//
void _glfwInputWindowDamage(plafWindow* window)
{
    if (window->refreshCallback)
        window->refreshCallback((plafWindow*) window);
}

// Notifies shared code that the user wishes to close a window
//
void _glfwInputWindowCloseRequest(plafWindow* window)
{
    window->shouldClose = true;

    if (window->closeCallback)
        window->closeCallback((plafWindow*) window);
}

// Notifies shared code that a window has changed its desired monitor
//
void _glfwInputWindowMonitor(plafWindow* window, plafMonitor* monitor)
{
    window->monitor = monitor;
}

//////////////////////////////////////////////////////////////////////////
//////                        GLFW public API                       //////
//////////////////////////////////////////////////////////////////////////

plafWindow* glfwCreateWindow(int width, int height,
                                     const char* title,
                                     plafMonitor* monitor,
                                     plafWindow* share)
{
    plafFrameBufferCfg fbconfig;
    plafCtxCfg ctxconfig;
    WindowConfig wndconfig;
    plafWindow* window;

    if (width <= 0 || height <= 0)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid window size %ix%i", width, height);

        return NULL;
    }

    fbconfig  = _glfw.frameBufferCfg;
    ctxconfig = _glfw.contextCfg;
    wndconfig = _glfw.windowCfg;

    wndconfig.width   = width;
    wndconfig.height  = height;
    ctxconfig.share   = (plafWindow*) share;

    if (!_glfwIsValidContextConfig(&ctxconfig))
        return NULL;

    window = _glfw_calloc(1, sizeof(plafWindow));
    window->next = _glfw.windowListHead;
    _glfw.windowListHead = window;

    window->videoMode.width       = width;
    window->videoMode.height      = height;
    window->videoMode.redBits     = fbconfig.redBits;
    window->videoMode.greenBits   = fbconfig.greenBits;
    window->videoMode.blueBits    = fbconfig.blueBits;
    window->videoMode.refreshRate = _glfw.desiredRefreshRate;

    window->monitor          = monitor;
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
        glfwDestroyWindow((plafWindow*) window);
        return NULL;
    }

    return (plafWindow*) window;
}

void glfwDefaultWindowHints(void)
{
    // The default is OpenGL with minimum version 1.0
    memset(&_glfw.contextCfg, 0, sizeof(_glfw.contextCfg));
    _glfw.contextCfg.major  = 3;
    _glfw.contextCfg.minor  = 2;
#if defined(__APPLE__)
	// These don't appear to be necessary to set on macOS anymore, but keeping for now
	_glfw.contextCfg.forward = true;
	_glfw.contextCfg.profile = OPENGL_PROFILE_CORE;
#endif

    // The default is a resizable window with decorations
    memset(&_glfw.windowCfg, 0, sizeof(_glfw.windowCfg));
    _glfw.windowCfg.resizable    = true;
    _glfw.windowCfg.decorated    = true;
    _glfw.windowCfg.xpos         = ANY_POSITION;
    _glfw.windowCfg.ypos         = ANY_POSITION;
    _glfw.windowCfg.scaleFramebuffer = true;

    // The default is 24 bits of color, 24 bits of depth and 8 bits of stencil, double buffered
    memset(&_glfw.frameBufferCfg, 0, sizeof(_glfw.frameBufferCfg));
    _glfw.frameBufferCfg.redBits      = 8;
    _glfw.frameBufferCfg.greenBits    = 8;
    _glfw.frameBufferCfg.blueBits     = 8;
    _glfw.frameBufferCfg.alphaBits    = 8;
    _glfw.frameBufferCfg.depthBits    = 24;
    _glfw.frameBufferCfg.stencilBits  = 8;
    _glfw.frameBufferCfg.doublebuffer = true;

    // The default is to select the highest available refresh rate
    _glfw.desiredRefreshRate = DONT_CARE;
}

void glfwWindowHint(int hint, int value)
{
    switch (hint)
    {
        case WINDOW_HINT_RED_BITS:
            _glfw.frameBufferCfg.redBits = value;
            return;
        case WINDOW_HINT_GREEN_BITS:
            _glfw.frameBufferCfg.greenBits = value;
            return;
        case WINDOW_HINT_BLUE_BITS:
            _glfw.frameBufferCfg.blueBits = value;
            return;
        case WINDOW_HINT_ALPHA_BITS:
            _glfw.frameBufferCfg.alphaBits = value;
            return;
        case WINDOW_HINT_DEPTH_BITS:
            _glfw.frameBufferCfg.depthBits = value;
            return;
        case WINDOW_HINT_STENCIL_BITS:
            _glfw.frameBufferCfg.stencilBits = value;
            return;
        case WINDOW_HINT_ACCUM_RED_BITS:
            _glfw.frameBufferCfg.accumRedBits = value;
            return;
        case WINDOW_HINT_ACCUM_GREEN_BITS:
            _glfw.frameBufferCfg.accumGreenBits = value;
            return;
        case WINDOW_HINT_ACCUM_BLUE_BITS:
            _glfw.frameBufferCfg.accumBlueBits = value;
            return;
        case WINDOW_HINT_ACCUM_ALPHA_BITS:
            _glfw.frameBufferCfg.accumAlphaBits = value;
            return;
        case WINDOW_HINT_AUX_BUFFERS:
            _glfw.frameBufferCfg.auxBuffers = value;
            return;
        case WINDOW_ATTR_HINT_DOUBLE_BUFFER:
            _glfw.frameBufferCfg.doublebuffer = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_TRANSPARENT_FRAMEBUFFER:
            _glfw.frameBufferCfg.transparent = value ? true : false;
            return;
        case WINDOW_HINT_SAMPLES:
            _glfw.frameBufferCfg.samples = value;
            return;
        case WINDOW_HINT_SRGB_CAPABLE:
            _glfw.frameBufferCfg.sRGB = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_RESIZABLE:
            _glfw.windowCfg.resizable = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_DECORATED:
            _glfw.windowCfg.decorated = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_FLOATING:
            _glfw.windowCfg.floating = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_MAXIMIZED:
            _glfw.windowCfg.maximized = value ? true : false;
            return;
        case WINDOW_HINT_POSITION_X:
            _glfw.windowCfg.xpos = value;
            return;
        case WINDOW_HINT_POSITION_Y:
            _glfw.windowCfg.ypos = value;
            return;
        case WINDOW_HINT_SCALE_TO_MONITOR:
            _glfw.windowCfg.scaleToMonitor = value ? true : false;
            return;
        case WINDOW_HINT_SCALE_FRAMEBUFFER:
            _glfw.windowCfg.scaleFramebuffer = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH:
            _glfw.windowCfg.mousePassthrough = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_VERSION_MAJOR:
            _glfw.contextCfg.major = value;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_VERSION_MINOR:
            _glfw.contextCfg.minor = value;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_ROBUSTNESS:
            _glfw.contextCfg.robustness = value;
            return;
        case WINDOW_ATTR_HINT_OPENGL_FORWARD_COMPAT:
            _glfw.contextCfg.forward = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_DEBUG:
            _glfw.contextCfg.debug = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_ERROR_SUPPRESSION:
            _glfw.contextCfg.noerror = value ? true : false;
            return;
        case WINDOW_ATTR_HINT_OPENGL_PROFILE:
            _glfw.contextCfg.profile = value;
            return;
        case WINDOW_ATTR_HINT_CONTEXT_RELEASE_BEHAVIOR:
            _glfw.contextCfg.release = value;
            return;
        case WINDOW_HINT_REFRESH_RATE:
            _glfw.desiredRefreshRate = value;
            return;
    }

    _glfwInputError(ERR_INVALID_ENUM, "Invalid window hint 0x%08X", hint);
}

void glfwDestroyWindow(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;

    // Allow closing of NULL (to match the behavior of free)
    if (window == NULL)
        return;

    // Clear all callbacks to avoid exposing a half torn-down window object
	window->posCallback = NULL;
	window->sizeCallback = NULL;
	window->closeCallback = NULL;
	window->refreshCallback = NULL;
	window->focusCallback = NULL;
	window->iconifyCallback = NULL;
	window->maximizeCallback = NULL;
	window->fbsizeCallback = NULL;
	window->scaleCallback = NULL;
	window->mouseButtonCallback = NULL;
	window->cursorPosCallback = NULL;
	window->cursorEnterCallback = NULL;
	window->scrollCallback = NULL;
	window->keyCallback = NULL;
	window->charCallback = NULL;
	window->charModsCallback = NULL;
	window->dropCallback = NULL;

    // The window's context must not be current when the window is destroyed
    if (window == _glfw.contextSlot)
        glfwMakeContextCurrent(NULL);

    _glfw.platform.destroyWindow(window);

    // Unlink window from global linked list
    {
        plafWindow** prev = &_glfw.windowListHead;

        while (*prev != window)
            prev = &((*prev)->next);

        *prev = window->next;
    }

    _glfw_free(window->title);
    _glfw_free(window);
}

int glfwWindowShouldClose(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    return window->shouldClose;
}

void glfwSetWindowShouldClose(plafWindow* handle, int value)
{
    plafWindow* window = (plafWindow*) handle;
    window->shouldClose = value;
}

const char* glfwGetWindowTitle(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    return window->title;
}

void glfwSetWindowTitle(plafWindow* handle, const char* title)
{
    plafWindow* window = (plafWindow*) handle;

    char* prev = window->title;
    window->title = _glfw_strdup(title);

    _glfw.platform.setWindowTitle(window, title);
    _glfw_free(prev);
}

void glfwSetWindowIcon(plafWindow* handle,
                               int count, const ImageData* images)
{
    int i;

    plafWindow* window = (plafWindow*) handle;

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

void glfwGetWindowPos(plafWindow* handle, int* xpos, int* ypos)
{
    if (xpos)
        *xpos = 0;
    if (ypos)
        *ypos = 0;

    plafWindow* window = (plafWindow*) handle;
    _glfw.platform.getWindowPos(window, xpos, ypos);
}

void glfwSetWindowPos(plafWindow* handle, int xpos, int ypos)
{
    plafWindow* window = (plafWindow*) handle;
    if (window->monitor)
        return;
    _glfw.platform.setWindowPos(window, xpos, ypos);
}

void glfwGetWindowSize(plafWindow* handle, int* width, int* height)
{
    if (width)
        *width = 0;
    if (height)
        *height = 0;
    plafWindow* window = (plafWindow*) handle;
    _glfw.platform.getWindowSize(window, width, height);
}

void glfwSetWindowSize(plafWindow* handle, int width, int height)
{
    plafWindow* window = (plafWindow*) handle;
    window->videoMode.width  = width;
    window->videoMode.height = height;
    _glfw.platform.setWindowSize(window, width, height);
}

void glfwSetWindowSizeLimits(plafWindow* handle,
                                     int minwidth, int minheight,
                                     int maxwidth, int maxheight)
{
    plafWindow* window = (plafWindow*) handle;
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

void glfwSetWindowAspectRatio(plafWindow* handle, int numer, int denom)
{
    plafWindow* window = (plafWindow*) handle;
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

void glfwGetFramebufferSize(plafWindow* handle, int* width, int* height)
{
    if (width)
        *width = 0;
    if (height)
        *height = 0;
    plafWindow* window = (plafWindow*) handle;
    _glfw.platform.getFramebufferSize(window, width, height);
}

void glfwGetWindowFrameSize(plafWindow* handle,
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
    plafWindow* window = (plafWindow*) handle;
    _glfw.platform.getWindowFrameSize(window, left, top, right, bottom);
}

void glfwGetWindowContentScale(plafWindow* handle,
                                       float* xscale, float* yscale)
{
    if (xscale)
        *xscale = 0.f;
    if (yscale)
        *yscale = 0.f;
    plafWindow* window = (plafWindow*) handle;
    _glfw.platform.getWindowContentScale(window, xscale, yscale);
}

float glfwGetWindowOpacity(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    return _glfw.platform.getWindowOpacity(window);
}

void glfwSetWindowOpacity(plafWindow* handle, float opacity)
{
    plafWindow* window = (plafWindow*) handle;
    if (opacity != opacity || opacity < 0.f || opacity > 1.f)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid window opacity %f", opacity);
        return;
    }

    _glfw.platform.setWindowOpacity(window, opacity);
}

void glfwIconifyWindow(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    _glfw.platform.iconifyWindow(window);
}

void glfwRestoreWindow(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    _glfw.platform.restoreWindow(window);
}

void glfwMaximizeWindow(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    if (window->monitor)
        return;

    _glfw.platform.maximizeWindow(window);
}

void glfwShowWindow(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    if (!window->monitor) {
	    _glfw.platform.showWindow(window);
	}
}

void glfwRequestWindowAttention(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    _glfw.platform.requestWindowAttention(window);
}

void glfwHideWindow(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    if (window->monitor)
        return;

    _glfw.platform.hideWindow(window);
}

void glfwFocusWindow(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    _glfw.platform.focusWindow(window);
}

int glfwGetWindowAttrib(plafWindow* handle, int attrib)
{
    plafWindow* window = (plafWindow*) handle;
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

void glfwSetWindowAttrib(plafWindow* handle, int attrib, int value)
{
    plafWindow* window = (plafWindow*) handle;
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

plafMonitor* glfwGetWindowMonitor(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    return window->monitor;
}

void glfwSetWindowMonitor(plafWindow* wh,
                                  plafMonitor* monitor,
                                  int xpos, int ypos,
                                  int width, int height,
                                  int refreshRate)
{
    plafWindow* window = (plafWindow*) wh;
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

windowPosFunc glfwSetWindowPosCallback(plafWindow* handle,
                                                  windowPosFunc cbfun)
{
    plafWindow* window = (plafWindow*) handle;
    SWAP(windowPosFunc, window->posCallback, cbfun);
    return cbfun;
}

windowSizeFunc glfwSetWindowSizeCallback(plafWindow* handle,
                                                    windowSizeFunc cbfun)
{
    plafWindow* window = (plafWindow*) handle;
    SWAP(windowSizeFunc, window->sizeCallback, cbfun);
    return cbfun;
}

windowCloseFunc glfwSetWindowCloseCallback(plafWindow* handle,
                                                      windowCloseFunc cbfun)
{
    plafWindow* window = (plafWindow*) handle;
    SWAP(windowCloseFunc, window->closeCallback, cbfun);
    return cbfun;
}

windowRefreshFunc glfwSetWindowRefreshCallback(plafWindow* handle,
                                                          windowRefreshFunc cbfun)
{
    plafWindow* window = (plafWindow*) handle;
    SWAP(windowRefreshFunc, window->refreshCallback, cbfun);
    return cbfun;
}

windowFocusFunc glfwSetWindowFocusCallback(plafWindow* handle,
                                                      windowFocusFunc cbfun)
{
    plafWindow* window = (plafWindow*) handle;
    SWAP(windowFocusFunc, window->focusCallback, cbfun);
    return cbfun;
}

windowIconifyFunc glfwSetWindowIconifyCallback(plafWindow* handle,
                                                          windowIconifyFunc cbfun)
{
    plafWindow* window = (plafWindow*) handle;
    SWAP(windowIconifyFunc, window->iconifyCallback, cbfun);
    return cbfun;
}

windowMaximizeFunc glfwSetWindowMaximizeCallback(plafWindow* handle,
                                                            windowMaximizeFunc cbfun)
{
    plafWindow* window = (plafWindow*) handle;
    SWAP(windowMaximizeFunc, window->maximizeCallback, cbfun);
    return cbfun;
}

frameBufferSizeFunc glfwSetFramebufferSizeCallback(plafWindow* handle,
                                                              frameBufferSizeFunc cbfun)
{
    plafWindow* window = (plafWindow*) handle;
    SWAP(frameBufferSizeFunc, window->fbsizeCallback, cbfun);
    return cbfun;
}

windowContextScaleFunc glfwSetWindowContentScaleCallback(plafWindow* handle,
                                                                    windowContextScaleFunc cbfun)
{
    plafWindow* window = (plafWindow*) handle;
    SWAP(windowContextScaleFunc, window->scaleCallback, cbfun);
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
