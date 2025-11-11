#include "platform.h"

#include <string.h>
#include <float.h>


//////////////////////////////////////////////////////////////////////////
//////                         GLFW event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code that a window has lost or received input focus
//
void _glfwInputWindowFocus(_GLFWwindow* window, GLFWbool focused)
{
    if (window->callbacks.focus)
        window->callbacks.focus((GLFWwindow*) window, focused);

    if (!focused)
    {
        int key, button;

        for (key = 0;  key <= GLFW_KEY_LAST;  key++)
        {
            if (window->keys[key] == GLFW_PRESS)
            {
                const int scancode = _glfw.platform.getKeyScancode(key);
                _glfwInputKey(window, key, scancode, GLFW_RELEASE, 0);
            }
        }

        for (button = 0;  button <= GLFW_MOUSE_BUTTON_LAST;  button++)
        {
            if (window->mouseButtons[button] == GLFW_PRESS)
                _glfwInputMouseClick(window, button, GLFW_RELEASE, 0);
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
void _glfwInputWindowIconify(_GLFWwindow* window, GLFWbool iconified)
{
    if (window->callbacks.iconify)
        window->callbacks.iconify((GLFWwindow*) window, iconified);
}

// Notifies shared code that a window has been maximized or restored
//
void _glfwInputWindowMaximize(_GLFWwindow* window, GLFWbool maximized)
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
    _GLFWwndconfig wndconfig;
    _GLFWwindow* window;

    if (width <= 0 || height <= 0)
    {
        _glfwInputError(GLFW_INVALID_VALUE, "Invalid window size %ix%i", width, height);

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
    window->autoIconify      = wndconfig.autoIconify;
    window->floating         = wndconfig.floating;
    window->focusOnShow      = wndconfig.focusOnShow;
    window->mousePassthrough = wndconfig.mousePassthrough;
    window->cursorMode       = GLFW_CURSOR_NORMAL;

    window->doublebuffer = fbconfig.doublebuffer;

    window->minwidth    = GLFW_DONT_CARE;
    window->minheight   = GLFW_DONT_CARE;
    window->maxwidth    = GLFW_DONT_CARE;
    window->maxheight   = GLFW_DONT_CARE;
    window->numer       = GLFW_DONT_CARE;
    window->denom       = GLFW_DONT_CARE;
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
#if defined(_GLFW_COCOA)
	// These don't appear to be necessary to set on macOS anymore, but keeping for now
	_glfw.hints.context.forward = true;
	_glfw.hints.context.profile = GLFW_OPENGL_CORE_PROFILE;
#endif

    // The default is a focused, visible, resizable window with decorations
    memset(&_glfw.hints.window, 0, sizeof(_glfw.hints.window));
    _glfw.hints.window.resizable    = true;
    _glfw.hints.window.visible      = true;
    _glfw.hints.window.decorated    = true;
    _glfw.hints.window.focused      = true;
    _glfw.hints.window.autoIconify  = true;
    _glfw.hints.window.centerCursor = true;
    _glfw.hints.window.focusOnShow  = true;
    _glfw.hints.window.xpos         = GLFW_ANY_POSITION;
    _glfw.hints.window.ypos         = GLFW_ANY_POSITION;
    _glfw.hints.window.scaleFramebuffer = true;

    // The default is 24 bits of color, 24 bits of depth and 8 bits of stencil,
    // double buffered
    memset(&_glfw.hints.framebuffer, 0, sizeof(_glfw.hints.framebuffer));
    _glfw.hints.framebuffer.redBits      = 8;
    _glfw.hints.framebuffer.greenBits    = 8;
    _glfw.hints.framebuffer.blueBits     = 8;
    _glfw.hints.framebuffer.alphaBits    = 8;
    _glfw.hints.framebuffer.depthBits    = 24;
    _glfw.hints.framebuffer.stencilBits  = 8;
    _glfw.hints.framebuffer.doublebuffer = true;

    // The default is to select the highest available refresh rate
    _glfw.hints.refreshRate = GLFW_DONT_CARE;
}

void glfwWindowHint(int hint, int value)
{
    switch (hint)
    {
        case GLFW_RED_BITS:
            _glfw.hints.framebuffer.redBits = value;
            return;
        case GLFW_GREEN_BITS:
            _glfw.hints.framebuffer.greenBits = value;
            return;
        case GLFW_BLUE_BITS:
            _glfw.hints.framebuffer.blueBits = value;
            return;
        case GLFW_ALPHA_BITS:
            _glfw.hints.framebuffer.alphaBits = value;
            return;
        case GLFW_DEPTH_BITS:
            _glfw.hints.framebuffer.depthBits = value;
            return;
        case GLFW_STENCIL_BITS:
            _glfw.hints.framebuffer.stencilBits = value;
            return;
        case GLFW_ACCUM_RED_BITS:
            _glfw.hints.framebuffer.accumRedBits = value;
            return;
        case GLFW_ACCUM_GREEN_BITS:
            _glfw.hints.framebuffer.accumGreenBits = value;
            return;
        case GLFW_ACCUM_BLUE_BITS:
            _glfw.hints.framebuffer.accumBlueBits = value;
            return;
        case GLFW_ACCUM_ALPHA_BITS:
            _glfw.hints.framebuffer.accumAlphaBits = value;
            return;
        case GLFW_AUX_BUFFERS:
            _glfw.hints.framebuffer.auxBuffers = value;
            return;
        case GLFW_DOUBLEBUFFER:
            _glfw.hints.framebuffer.doublebuffer = value ? true : false;
            return;
        case GLFW_TRANSPARENT_FRAMEBUFFER:
            _glfw.hints.framebuffer.transparent = value ? true : false;
            return;
        case GLFW_SAMPLES:
            _glfw.hints.framebuffer.samples = value;
            return;
        case GLFW_SRGB_CAPABLE:
            _glfw.hints.framebuffer.sRGB = value ? true : false;
            return;
        case GLFW_RESIZABLE:
            _glfw.hints.window.resizable = value ? true : false;
            return;
        case GLFW_DECORATED:
            _glfw.hints.window.decorated = value ? true : false;
            return;
        case GLFW_FOCUSED:
            _glfw.hints.window.focused = value ? true : false;
            return;
        case GLFW_AUTO_ICONIFY:
            _glfw.hints.window.autoIconify = value ? true : false;
            return;
        case GLFW_FLOATING:
            _glfw.hints.window.floating = value ? true : false;
            return;
        case GLFW_MAXIMIZED:
            _glfw.hints.window.maximized = value ? true : false;
            return;
        case GLFW_VISIBLE:
            _glfw.hints.window.visible = value ? true : false;
            return;
        case GLFW_POSITION_X:
            _glfw.hints.window.xpos = value;
            return;
        case GLFW_POSITION_Y:
            _glfw.hints.window.ypos = value;
            return;
        case GLFW_WIN32_KEYBOARD_MENU:
            _glfw.hints.window.win32.keymenu = value ? true : false;
            return;
        case GLFW_WIN32_SHOWDEFAULT:
            _glfw.hints.window.win32.showDefault = value ? true : false;
            return;
        case GLFW_COCOA_GRAPHICS_SWITCHING:
            _glfw.hints.context.nsgl.offline = value ? true : false;
            return;
        case GLFW_SCALE_TO_MONITOR:
            _glfw.hints.window.scaleToMonitor = value ? true : false;
            return;
        case GLFW_SCALE_FRAMEBUFFER:
        case GLFW_COCOA_RETINA_FRAMEBUFFER:
            _glfw.hints.window.scaleFramebuffer = value ? true : false;
            return;
        case GLFW_CENTER_CURSOR:
            _glfw.hints.window.centerCursor = value ? true : false;
            return;
        case GLFW_FOCUS_ON_SHOW:
            _glfw.hints.window.focusOnShow = value ? true : false;
            return;
        case GLFW_MOUSE_PASSTHROUGH:
            _glfw.hints.window.mousePassthrough = value ? true : false;
            return;
        case GLFW_CONTEXT_VERSION_MAJOR:
            _glfw.hints.context.major = value;
            return;
        case GLFW_CONTEXT_VERSION_MINOR:
            _glfw.hints.context.minor = value;
            return;
        case GLFW_CONTEXT_ROBUSTNESS:
            _glfw.hints.context.robustness = value;
            return;
        case GLFW_OPENGL_FORWARD_COMPAT:
            _glfw.hints.context.forward = value ? true : false;
            return;
        case GLFW_CONTEXT_DEBUG:
            _glfw.hints.context.debug = value ? true : false;
            return;
        case GLFW_CONTEXT_NO_ERROR:
            _glfw.hints.context.noerror = value ? true : false;
            return;
        case GLFW_OPENGL_PROFILE:
            _glfw.hints.context.profile = value;
            return;
        case GLFW_CONTEXT_RELEASE_BEHAVIOR:
            _glfw.hints.context.release = value;
            return;
        case GLFW_REFRESH_RATE:
            _glfw.hints.refreshRate = value;
            return;
    }

    _glfwInputError(GLFW_INVALID_ENUM, "Invalid window hint 0x%08X", hint);
}

void glfwWindowHintString(int hint, const char* value)
{
    switch (hint)
    {
        case GLFW_COCOA_FRAME_NAME:
            strncpy(_glfw.hints.window.ns.frameName, value,
                    sizeof(_glfw.hints.window.ns.frameName) - 1);
            return;
        case GLFW_X11_CLASS_NAME:
            strncpy(_glfw.hints.window.x11.className, value,
                    sizeof(_glfw.hints.window.x11.className) - 1);
            return;
        case GLFW_X11_INSTANCE_NAME:
            strncpy(_glfw.hints.window.x11.instanceName, value,
                    sizeof(_glfw.hints.window.x11.instanceName) - 1);
            return;
    }

    _glfwInputError(GLFW_INVALID_ENUM, "Invalid window hint string 0x%08X", hint);
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
                               int count, const GLFWimage* images)
{
    int i;

    _GLFWwindow* window = (_GLFWwindow*) handle;

    if (count < 0)
    {
        _glfwInputError(GLFW_INVALID_VALUE, "Invalid image count for window icon");
        return;
    }

    for (i = 0; i < count; i++)
    {
        if (images[i].width <= 0 || images[i].height <= 0)
        {
            _glfwInputError(GLFW_INVALID_VALUE, "Invalid image dimensions for window icon");
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
    if (minwidth != GLFW_DONT_CARE && minheight != GLFW_DONT_CARE)
    {
        if (minwidth < 0 || minheight < 0)
        {
            _glfwInputError(GLFW_INVALID_VALUE, "Invalid window minimum size %ix%i", minwidth, minheight);
            return;
        }
    }

    if (maxwidth != GLFW_DONT_CARE && maxheight != GLFW_DONT_CARE)
    {
        if (maxwidth < 0 || maxheight < 0 ||
            maxwidth < minwidth || maxheight < minheight)
        {
            _glfwInputError(GLFW_INVALID_VALUE, "Invalid window maximum size %ix%i", maxwidth, maxheight);
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
    if (numer != GLFW_DONT_CARE && denom != GLFW_DONT_CARE)
    {
        if (numer <= 0 || denom <= 0)
        {
            _glfwInputError(GLFW_INVALID_VALUE, "Invalid window aspect ratio %i:%i", numer, denom);
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
        _glfwInputError(GLFW_INVALID_VALUE, "Invalid window opacity %f", opacity);
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
    if (window->monitor)
        return;

    _glfw.platform.showWindow(window);

    if (window->focusOnShow)
        _glfw.platform.focusWindow(window);
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
        case GLFW_FOCUSED:
            return _glfw.platform.windowFocused(window);
        case GLFW_ICONIFIED:
            return _glfw.platform.windowIconified(window);
        case GLFW_VISIBLE:
            return _glfw.platform.windowVisible(window);
        case GLFW_MAXIMIZED:
            return _glfw.platform.windowMaximized(window);
        case GLFW_HOVERED:
            return _glfw.platform.windowHovered(window);
        case GLFW_FOCUS_ON_SHOW:
            return window->focusOnShow;
        case GLFW_MOUSE_PASSTHROUGH:
            return window->mousePassthrough;
        case GLFW_TRANSPARENT_FRAMEBUFFER:
            return _glfw.platform.framebufferTransparent(window);
        case GLFW_RESIZABLE:
            return window->resizable;
        case GLFW_DECORATED:
            return window->decorated;
        case GLFW_FLOATING:
            return window->floating;
        case GLFW_AUTO_ICONIFY:
            return window->autoIconify;
        case GLFW_DOUBLEBUFFER:
            return window->doublebuffer;
        case GLFW_CONTEXT_VERSION_MAJOR:
            return window->context.major;
        case GLFW_CONTEXT_VERSION_MINOR:
            return window->context.minor;
        case GLFW_CONTEXT_REVISION:
            return window->context.revision;
        case GLFW_CONTEXT_ROBUSTNESS:
            return window->context.robustness;
        case GLFW_OPENGL_FORWARD_COMPAT:
            return window->context.forward;
        case GLFW_CONTEXT_DEBUG:
            return window->context.debug;
        case GLFW_OPENGL_PROFILE:
            return window->context.profile;
        case GLFW_CONTEXT_RELEASE_BEHAVIOR:
            return window->context.release;
        case GLFW_CONTEXT_NO_ERROR:
            return window->context.noerror;
    }

    _glfwInputError(GLFW_INVALID_ENUM, "Invalid window attribute 0x%08X", attrib);
    return 0;
}

void glfwSetWindowAttrib(GLFWwindow* handle, int attrib, int value)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    value = value ? true : false;

    switch (attrib)
    {
        case GLFW_AUTO_ICONIFY:
            window->autoIconify = value;
            return;

        case GLFW_RESIZABLE:
            window->resizable = value;
            if (!window->monitor)
                _glfw.platform.setWindowResizable(window, value);
            return;

        case GLFW_DECORATED:
            window->decorated = value;
            if (!window->monitor)
                _glfw.platform.setWindowDecorated(window, value);
            return;

        case GLFW_FLOATING:
            window->floating = value;
            if (!window->monitor)
                _glfw.platform.setWindowFloating(window, value);
            return;

        case GLFW_FOCUS_ON_SHOW:
            window->focusOnShow = value;
            return;

        case GLFW_MOUSE_PASSTHROUGH:
            window->mousePassthrough = value;
            _glfw.platform.setWindowMousePassthrough(window, value);
            return;
    }

    _glfwInputError(GLFW_INVALID_ENUM, "Invalid window attribute 0x%08X", attrib);
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
        _glfwInputError(GLFW_INVALID_VALUE, "Invalid window size %ix%i", width, height);
        return;
    }

    if (refreshRate < 0 && refreshRate != GLFW_DONT_CARE)
    {
        _glfwInputError(GLFW_INVALID_VALUE, "Invalid refresh rate %i", refreshRate);
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

GLFWwindowposfun glfwSetWindowPosCallback(GLFWwindow* handle,
                                                  GLFWwindowposfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWwindowposfun, window->callbacks.pos, cbfun);
    return cbfun;
}

GLFWwindowsizefun glfwSetWindowSizeCallback(GLFWwindow* handle,
                                                    GLFWwindowsizefun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWwindowsizefun, window->callbacks.size, cbfun);
    return cbfun;
}

GLFWwindowclosefun glfwSetWindowCloseCallback(GLFWwindow* handle,
                                                      GLFWwindowclosefun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWwindowclosefun, window->callbacks.close, cbfun);
    return cbfun;
}

GLFWwindowrefreshfun glfwSetWindowRefreshCallback(GLFWwindow* handle,
                                                          GLFWwindowrefreshfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWwindowrefreshfun, window->callbacks.refresh, cbfun);
    return cbfun;
}

GLFWwindowfocusfun glfwSetWindowFocusCallback(GLFWwindow* handle,
                                                      GLFWwindowfocusfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWwindowfocusfun, window->callbacks.focus, cbfun);
    return cbfun;
}

GLFWwindowiconifyfun glfwSetWindowIconifyCallback(GLFWwindow* handle,
                                                          GLFWwindowiconifyfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWwindowiconifyfun, window->callbacks.iconify, cbfun);
    return cbfun;
}

GLFWwindowmaximizefun glfwSetWindowMaximizeCallback(GLFWwindow* handle,
                                                            GLFWwindowmaximizefun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWwindowmaximizefun, window->callbacks.maximize, cbfun);
    return cbfun;
}

GLFWframebuffersizefun glfwSetFramebufferSizeCallback(GLFWwindow* handle,
                                                              GLFWframebuffersizefun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWframebuffersizefun, window->callbacks.fbsize, cbfun);
    return cbfun;
}

GLFWwindowcontentscalefun glfwSetWindowContentScaleCallback(GLFWwindow* handle,
                                                                    GLFWwindowcontentscalefun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWwindowcontentscalefun, window->callbacks.scale, cbfun);
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
        _glfwInputError(GLFW_INVALID_VALUE, "Invalid time %f", timeout);
        return;
    }

    _glfw.platform.waitEventsTimeout(timeout);
}

void glfwPostEmptyEvent(void)
{
    _glfw.platform.postEmptyEvent();
}
