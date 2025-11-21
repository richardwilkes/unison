#include "platform.h"


//////////////////////////////////////////////////////////////////////////
//////                         GLFW event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code that a window has lost or received input focus
//
void _glfwInputWindowFocus(plafWindow* window, IntBool focused)
{
	if (window->focusCallback)
		window->focusCallback(window, focused);

	if (!focused)
	{
		int key, button;

		for (key = 0;  key <= KEY_LAST;  key++)
		{
			if (window->keys[key] == INPUT_PRESS)
			{
				_glfwInputKey(window, key, _glfw.scanCodes[key], INPUT_RELEASE, 0);
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
		window->posCallback(window, x, y);
}

// Notifies shared code that a window has been resized
// The size is specified in screen coordinates
//
void _glfwInputWindowSize(plafWindow* window, int width, int height)
{
	if (window->sizeCallback)
		window->sizeCallback(window, width, height);
}

// Notifies shared code that a window has been minimized or restored
//
void _glfwInputWindowMinimize(plafWindow* window, IntBool minimized)
{
	if (window->minimizeCallback)
		window->minimizeCallback(window, minimized);
}

// Notifies shared code that a window has been maximized or restored
//
void _glfwInputWindowMaximize(plafWindow* window, IntBool maximized)
{
	if (window->maximizeCallback)
		window->maximizeCallback(window, maximized);
}

// Notifies shared code that a window framebuffer has been resized
// The size is specified in pixels
//
void _glfwInputFramebufferSize(plafWindow* window, int width, int height)
{
	if (window->fbsizeCallback)
		window->fbsizeCallback(window, width, height);
}

// Notifies shared code that a window content scale has changed
// The scale is specified as the ratio between the current and default DPI
//
void _glfwInputWindowContentScale(plafWindow* window, float xscale, float yscale)
{
	if (window->scaleCallback)
		window->scaleCallback(window, xscale, yscale);
}

// Notifies shared code that the window contents needs updating
//
void _glfwInputWindowDamage(plafWindow* window)
{
	if (window->refreshCallback)
		window->refreshCallback(window);
}

// Notifies shared code that the user wishes to close a window
//
void _glfwInputWindowCloseRequest(plafWindow* window)
{
	window->shouldClose = true;

	if (window->closeCallback)
		window->closeCallback(window);
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

 ErrorResponse* glfwCreateWindow(int width, int height, const char* title, plafMonitor* monitor, plafWindow* share, plafWindow** outWindow) {
	if (width <= 0 || height <= 0) {
		return createErrorResponse("Invalid window size %ix%i", width, height);
	}

	plafCtxCfg ctxconfig = _glfw.contextCfg;
	ctxconfig.share      = share;
	ErrorResponse* err   = plafCheckContextConfig(&ctxconfig);
	if (err) {
		return err;
	}

	plafFrameBufferCfg fbconfig = _glfw.frameBufferCfg;

	WindowConfig wndconfig = _glfw.windowCfg;
	wndconfig.width        = width;
	wndconfig.height       = height;

	plafWindow* window = _glfw_calloc(1, sizeof(plafWindow));
	window->next                  = _glfw.windowListHead;
	_glfw.windowListHead          = window;
	window->videoMode.width       = width;
	window->videoMode.height      = height;
	window->videoMode.redBits     = fbconfig.redBits;
	window->videoMode.greenBits   = fbconfig.greenBits;
	window->videoMode.blueBits    = fbconfig.blueBits;
	window->videoMode.refreshRate = _glfw.desiredRefreshRate;
	window->monitor               = monitor;
	window->resizable             = wndconfig.resizable;
	window->decorated             = wndconfig.decorated;
	window->floating              = wndconfig.floating;
	window->mousePassthrough      = wndconfig.mousePassthrough;
	window->doublebuffer          = fbconfig.doublebuffer;
	window->minwidth              = DONT_CARE;
	window->minheight             = DONT_CARE;
	window->maxwidth              = DONT_CARE;
	window->maxheight             = DONT_CARE;
	window->numer                 = DONT_CARE;
	window->denom                 = DONT_CARE;
	window->title                 = _glfw_strdup(title);

	err = _glfwCreateWindow(window, &wndconfig, &ctxconfig, &fbconfig);
	if (err) {
		glfwDestroyWindow(window);
		return err;
	}

	*outWindow = window;
	return NULL;
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

	_glfwInputError("Invalid window hint 0x%08X", hint);
}

void glfwDestroyWindow(plafWindow* window)
{
	// Allow closing of NULL (to match the behavior of free)
	if (window == NULL)
		return;

	// Clear all callbacks to avoid exposing a half torn-down window object
	window->posCallback = NULL;
	window->sizeCallback = NULL;
	window->closeCallback = NULL;
	window->refreshCallback = NULL;
	window->focusCallback = NULL;
	window->minimizeCallback = NULL;
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
	if (window == _glfw.contextSlot) {
		glfwMakeContextCurrent(NULL);
	}

	_glfwDestroyWindow(window);

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

int glfwWindowShouldClose(plafWindow* window) {
	return window->shouldClose;
}

void glfwSetWindowShouldClose(plafWindow* window, int value) {
	window->shouldClose = value;
}

const char* glfwGetWindowTitle(plafWindow* window) {
	return window->title;
}

void glfwSetWindowTitle(plafWindow* window, const char* title) {
	char* prev = window->title;
	window->title = _glfw_strdup(title);
	_glfwSetWindowTitle(window, window->title);
	_glfw_free(prev);
}

void glfwSetWindowIcon(plafWindow* window, int count, const ImageData* images)
{
	int i;

	if (count < 0)
	{
		_glfwInputError("Invalid image count for window icon");
		return;
	}

	for (i = 0; i < count; i++)
	{
		if (images[i].width <= 0 || images[i].height <= 0)
		{
			_glfwInputError("Invalid image dimensions for window icon");
			return;
		}
	}

	_glfwSetWindowIcon(window, count, images);
}

void glfwGetWindowPos(plafWindow* window, int* xpos, int* ypos) {
	if (xpos)
		*xpos = 0;
	if (ypos)
		*ypos = 0;
	_glfwGetWindowPos(window, xpos, ypos);
}

void glfwSetWindowPos(plafWindow* window, int xpos, int ypos) {
	if (window->monitor) {
		return;
	}
	_glfwSetWindowPos(window, xpos, ypos);
}

void glfwGetWindowSize(plafWindow* window, int* width, int* height) {
	if (width)
		*width = 0;
	if (height)
		*height = 0;
	_glfwGetWindowSize(window, width, height);
}

void glfwSetWindowSize(plafWindow* window, int width, int height) {
	window->videoMode.width  = width;
	window->videoMode.height = height;
	_glfwSetWindowSize(window, width, height);
}

void glfwSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight)
{
	if (minwidth != DONT_CARE && minheight != DONT_CARE)
	{
		if (minwidth < 0 || minheight < 0)
		{
			_glfwInputError("Invalid window minimum size %ix%i", minwidth, minheight);
			return;
		}
	}

	if (maxwidth != DONT_CARE && maxheight != DONT_CARE)
	{
		if (maxwidth < 0 || maxheight < 0 ||
			maxwidth < minwidth || maxheight < minheight)
		{
			_glfwInputError("Invalid window maximum size %ix%i", maxwidth, maxheight);
			return;
		}
	}

	window->minwidth  = minwidth;
	window->minheight = minheight;
	window->maxwidth  = maxwidth;
	window->maxheight = maxheight;

	if (window->monitor || !window->resizable)
		return;

	_glfwSetWindowSizeLimits(window, minwidth, minheight, maxwidth, maxheight);
}

void glfwGetFramebufferSize(plafWindow* window, int* width, int* height) {
	if (width)
		*width = 0;
	if (height)
		*height = 0;
	_glfwGetFramebufferSize(window, width, height);
}

void glfwGetWindowFrameSize(plafWindow* window, int* left, int* top, int* right, int* bottom) {
	if (left)
		*left = 0;
	if (top)
		*top = 0;
	if (right)
		*right = 0;
	if (bottom)
		*bottom = 0;
	_glfwGetWindowFrameSize(window, left, top, right, bottom);
}

void glfwGetWindowContentScale(plafWindow* window, float* xscale, float* yscale) {
	if (xscale)
		*xscale = 0.f;
	if (yscale)
		*yscale = 0.f;
	_glfwGetWindowContentScale(window, xscale, yscale);
}

void glfwSetWindowOpacity(plafWindow* window, float opacity) {
	if (opacity != opacity || opacity < 0.f || opacity > 1.f)
	{
		_glfwInputError("Invalid window opacity %f", opacity);
		return;
	}

	_glfwSetWindowOpacity(window, opacity);
}

void glfwMaximizeWindow(plafWindow* window) {
	if (window->monitor)
		return;

	_glfwMaximizeWindow(window);
}

void glfwShowWindow(plafWindow* window) {
	if (!window->monitor) {
		_glfwShowWindow(window);
	}
}

void glfwHideWindow(plafWindow* window) {
	if (window->monitor)
		return;

	_glfwHideWindow(window);
}

int glfwGetWindowAttrib(plafWindow* window, int attrib) {
	switch (attrib)
	{
		case WINDOW_ATTR_FOCUSED:
			return _glfwWindowFocused(window);
		case WINDOW_ATTR_MINIMIZED:
			return _glfwWindowMinimized(window);
		case WINDOW_ATTR_VISIBLE:
			return _glfwWindowVisible(window);
		case WINDOW_ATTR_HINT_MAXIMIZED:
			return _glfwWindowMaximized(window);
		case WINDOW_ATTR_HOVERED:
			return _glfwWindowHovered(window);
		case WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH:
			return window->mousePassthrough;
		case WINDOW_ATTR_HINT_TRANSPARENT_FRAMEBUFFER:
			return _glfwFramebufferTransparent(window);
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

	_glfwInputError("Invalid window attribute 0x%08X", attrib);
	return 0;
}

void glfwSetWindowAttrib(plafWindow* window, int attrib, int value) {
	value = value ? true : false;

	switch (attrib)
	{
		case WINDOW_ATTR_HINT_RESIZABLE:
			window->resizable = value;
			if (!window->monitor)
				_glfwSetWindowResizable(window, value);
			return;

		case WINDOW_ATTR_HINT_DECORATED:
			window->decorated = value;
			if (!window->monitor)
				_glfwSetWindowDecorated(window, value);
			return;

		case WINDOW_ATTR_HINT_FLOATING:
			window->floating = value;
			if (!window->monitor)
				_glfwSetWindowFloating(window, value);
			return;

		case WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH:
			window->mousePassthrough = value;
			_glfwSetWindowMousePassthrough(window, value);
			return;
	}

	_glfwInputError("Invalid window attribute 0x%08X", attrib);
}

plafMonitor* glfwGetWindowMonitor(plafWindow* window) {
	return window->monitor;
}

void glfwSetWindowMonitor(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate) {
	if (width <= 0 || height <= 0)
	{
		_glfwInputError("Invalid window size %ix%i", width, height);
		return;
	}

	if (refreshRate < 0 && refreshRate != DONT_CARE)
	{
		_glfwInputError("Invalid refresh rate %i", refreshRate);
		return;
	}

	window->videoMode.width       = width;
	window->videoMode.height      = height;
	window->videoMode.refreshRate = refreshRate;
	_glfwSetWindowMonitor(window, monitor, xpos, ypos, width, height, refreshRate);
}

windowPosFunc glfwSetWindowPosCallback(plafWindow* window, windowPosFunc cbfun) {
	SWAP(windowPosFunc, window->posCallback, cbfun);
	return cbfun;
}

windowSizeFunc glfwSetWindowSizeCallback(plafWindow* window, windowSizeFunc cbfun) {
	SWAP(windowSizeFunc, window->sizeCallback, cbfun);
	return cbfun;
}

windowCloseFunc glfwSetWindowCloseCallback(plafWindow* window, windowCloseFunc cbfun) {
	SWAP(windowCloseFunc, window->closeCallback, cbfun);
	return cbfun;
}

windowRefreshFunc glfwSetWindowRefreshCallback(plafWindow* window, windowRefreshFunc cbfun) {
	SWAP(windowRefreshFunc, window->refreshCallback, cbfun);
	return cbfun;
}

windowFocusFunc glfwSetWindowFocusCallback(plafWindow* window, windowFocusFunc cbfun) {
	SWAP(windowFocusFunc, window->focusCallback, cbfun);
	return cbfun;
}

windowMinimizeFunc glfwSetWindowMinimizeCallback(plafWindow* window, windowMinimizeFunc cbfun) {
	SWAP(windowMinimizeFunc, window->minimizeCallback, cbfun);
	return cbfun;
}

windowMaximizeFunc glfwSetWindowMaximizeCallback(plafWindow* window, windowMaximizeFunc cbfun) {
	SWAP(windowMaximizeFunc, window->maximizeCallback, cbfun);
	return cbfun;
}

frameBufferSizeFunc glfwSetFramebufferSizeCallback(plafWindow* window, frameBufferSizeFunc cbfun) {
	SWAP(frameBufferSizeFunc, window->fbsizeCallback, cbfun);
	return cbfun;
}

windowContextScaleFunc glfwSetWindowContentScaleCallback(plafWindow* window, windowContextScaleFunc cbfun) {
	SWAP(windowContextScaleFunc, window->scaleCallback, cbfun);
	return cbfun;
}

void glfwWaitEventsTimeout(double timeout)
{
	if (timeout != timeout || timeout < 0.0 || timeout > DBL_MAX)
	{
		_glfwInputError("Invalid time %f", timeout);
		return;
	}
	_glfwWaitEventsTimeout(timeout);
}
