#include "platform.h"


//////////////////////////////////////////////////////////////////////////
//////                         PLAF event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code that a window has lost or received input focus
//
void _plafInputWindowFocus(plafWindow* window, IntBool focused)
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
				_plafInputKey(window, key, _plaf.scanCodes[key], INPUT_RELEASE, 0);
			}
		}

		for (button = 0;  button <= MOUSE_BUTTON_LAST;  button++)
		{
			if (window->mouseButtons[button] == INPUT_PRESS)
				_plafInputMouseClick(window, button, INPUT_RELEASE, 0);
		}
	}
}

// Notifies shared code that a window has moved
// The position is specified in content area relative screen coordinates
//
void _plafInputWindowPos(plafWindow* window, int x, int y)
{
	if (window->posCallback)
		window->posCallback(window, x, y);
}

// Notifies shared code that a window has been resized
// The size is specified in screen coordinates
//
void _plafInputWindowSize(plafWindow* window, int width, int height)
{
	if (window->sizeCallback)
		window->sizeCallback(window, width, height);
}

// Notifies shared code that a window has been minimized or restored
//
void _plafInputWindowMinimize(plafWindow* window, IntBool minimized)
{
	if (window->minimizeCallback)
		window->minimizeCallback(window, minimized);
}

// Notifies shared code that a window has been maximized or restored
//
void _plafInputWindowMaximize(plafWindow* window, IntBool maximized)
{
	if (window->maximizeCallback)
		window->maximizeCallback(window, maximized);
}

// Notifies shared code that a window framebuffer has been resized
// The size is specified in pixels
//
void _plafInputFramebufferSize(plafWindow* window, int width, int height)
{
	if (window->fbsizeCallback)
		window->fbsizeCallback(window, width, height);
}

// Notifies shared code that a window content scale has changed
// The scale is specified as the ratio between the current and default DPI
//
void _plafInputWindowContentScale(plafWindow* window, float xscale, float yscale)
{
	if (window->scaleCallback)
		window->scaleCallback(window, xscale, yscale);
}

// Notifies shared code that the window contents needs updating
//
void _plafInputWindowDamage(plafWindow* window)
{
	if (window->refreshCallback)
		window->refreshCallback(window);
}

// Notifies shared code that the user wishes to close a window
//
void _plafInputWindowCloseRequest(plafWindow* window)
{
	window->shouldClose = true;

	if (window->closeCallback)
		window->closeCallback(window);
}

//////////////////////////////////////////////////////////////////////////
//////                        PLAF public API                       //////
//////////////////////////////////////////////////////////////////////////

 plafError* plafCreateWindow(int width, int height, const char* title, plafMonitor* monitor, plafWindow* share, plafWindow** outWindow) {
	if (width <= 0 || height <= 0) {
		return _plafNewError("Invalid window size %ix%i", width, height);
	}

	plafCtxCfg ctxconfig = _plaf.contextCfg;
	ctxconfig.share      = share;
	plafError* err   = plafCheckContextConfig(&ctxconfig);
	if (err) {
		return err;
	}

	plafFrameBufferCfg fbconfig = _plaf.frameBufferCfg;

	plafWindowConfig wndconfig = _plaf.windowCfg;
	wndconfig.width        = width;
	wndconfig.height       = height;

	plafWindow* window = _plaf_calloc(1, sizeof(plafWindow));
	window->next                  = _plaf.windowListHead;
	_plaf.windowListHead          = window;
	window->videoMode.width       = width;
	window->videoMode.height      = height;
	window->videoMode.redBits     = fbconfig.redBits;
	window->videoMode.greenBits   = fbconfig.greenBits;
	window->videoMode.blueBits    = fbconfig.blueBits;
	window->videoMode.refreshRate = _plaf.desiredRefreshRate;
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
	window->title                 = _plaf_strdup(title);

	err = _plafCreateWindow(window, &wndconfig, &ctxconfig, &fbconfig);
	if (err) {
		plafDestroyWindow(window);
		return err;
	}

	*outWindow = window;
	return NULL;
}

void plafDefaultWindowHints(void)
{
	// The default is OpenGL with minimum version 1.0
	memset(&_plaf.contextCfg, 0, sizeof(_plaf.contextCfg));
	_plaf.contextCfg.major  = 3;
	_plaf.contextCfg.minor  = 2;
#if defined(__APPLE__)
	// These don't appear to be necessary to set on macOS anymore, but keeping for now
	_plaf.contextCfg.forward = true;
	_plaf.contextCfg.profile = OPENGL_PROFILE_CORE;
#endif

	// The default is a resizable window with decorations
	memset(&_plaf.windowCfg, 0, sizeof(_plaf.windowCfg));
	_plaf.windowCfg.resizable    = true;
	_plaf.windowCfg.decorated    = true;
	_plaf.windowCfg.xpos         = ANY_POSITION;
	_plaf.windowCfg.ypos         = ANY_POSITION;
	_plaf.windowCfg.scaleFramebuffer = true;

	// The default is 24 bits of color, 24 bits of depth and 8 bits of stencil, double buffered
	memset(&_plaf.frameBufferCfg, 0, sizeof(_plaf.frameBufferCfg));
	_plaf.frameBufferCfg.redBits      = 8;
	_plaf.frameBufferCfg.greenBits    = 8;
	_plaf.frameBufferCfg.blueBits     = 8;
	_plaf.frameBufferCfg.alphaBits    = 8;
	_plaf.frameBufferCfg.depthBits    = 24;
	_plaf.frameBufferCfg.stencilBits  = 8;
	_plaf.frameBufferCfg.doublebuffer = true;

	// The default is to select the highest available refresh rate
	_plaf.desiredRefreshRate = DONT_CARE;
}

void plafWindowHint(int hint, int value)
{
	switch (hint)
	{
		case WINDOW_HINT_RED_BITS:
			_plaf.frameBufferCfg.redBits = value;
			return;
		case WINDOW_HINT_GREEN_BITS:
			_plaf.frameBufferCfg.greenBits = value;
			return;
		case WINDOW_HINT_BLUE_BITS:
			_plaf.frameBufferCfg.blueBits = value;
			return;
		case WINDOW_HINT_ALPHA_BITS:
			_plaf.frameBufferCfg.alphaBits = value;
			return;
		case WINDOW_HINT_DEPTH_BITS:
			_plaf.frameBufferCfg.depthBits = value;
			return;
		case WINDOW_HINT_STENCIL_BITS:
			_plaf.frameBufferCfg.stencilBits = value;
			return;
		case WINDOW_HINT_ACCUM_RED_BITS:
			_plaf.frameBufferCfg.accumRedBits = value;
			return;
		case WINDOW_HINT_ACCUM_GREEN_BITS:
			_plaf.frameBufferCfg.accumGreenBits = value;
			return;
		case WINDOW_HINT_ACCUM_BLUE_BITS:
			_plaf.frameBufferCfg.accumBlueBits = value;
			return;
		case WINDOW_HINT_ACCUM_ALPHA_BITS:
			_plaf.frameBufferCfg.accumAlphaBits = value;
			return;
		case WINDOW_HINT_AUX_BUFFERS:
			_plaf.frameBufferCfg.auxBuffers = value;
			return;
		case WINDOW_ATTR_HINT_DOUBLE_BUFFER:
			_plaf.frameBufferCfg.doublebuffer = value ? true : false;
			return;
		case WINDOW_ATTR_HINT_TRANSPARENT_FRAMEBUFFER:
			_plaf.frameBufferCfg.transparent = value ? true : false;
			return;
		case WINDOW_HINT_SAMPLES:
			_plaf.frameBufferCfg.samples = value;
			return;
		case WINDOW_HINT_SRGB_CAPABLE:
			_plaf.frameBufferCfg.sRGB = value ? true : false;
			return;
		case WINDOW_ATTR_HINT_RESIZABLE:
			_plaf.windowCfg.resizable = value ? true : false;
			return;
		case WINDOW_ATTR_HINT_DECORATED:
			_plaf.windowCfg.decorated = value ? true : false;
			return;
		case WINDOW_ATTR_HINT_FLOATING:
			_plaf.windowCfg.floating = value ? true : false;
			return;
		case WINDOW_ATTR_HINT_MAXIMIZED:
			_plaf.windowCfg.maximized = value ? true : false;
			return;
		case WINDOW_HINT_POSITION_X:
			_plaf.windowCfg.xpos = value;
			return;
		case WINDOW_HINT_POSITION_Y:
			_plaf.windowCfg.ypos = value;
			return;
		case WINDOW_HINT_SCALE_TO_MONITOR:
			_plaf.windowCfg.scaleToMonitor = value ? true : false;
			return;
		case WINDOW_HINT_SCALE_FRAMEBUFFER:
			_plaf.windowCfg.scaleFramebuffer = value ? true : false;
			return;
		case WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH:
			_plaf.windowCfg.mousePassthrough = value ? true : false;
			return;
		case WINDOW_ATTR_HINT_CONTEXT_VERSION_MAJOR:
			_plaf.contextCfg.major = value;
			return;
		case WINDOW_ATTR_HINT_CONTEXT_VERSION_MINOR:
			_plaf.contextCfg.minor = value;
			return;
		case WINDOW_ATTR_HINT_CONTEXT_ROBUSTNESS:
			_plaf.contextCfg.robustness = value;
			return;
		case WINDOW_ATTR_HINT_OPENGL_FORWARD_COMPAT:
			_plaf.contextCfg.forward = value ? true : false;
			return;
		case WINDOW_ATTR_HINT_CONTEXT_DEBUG:
			_plaf.contextCfg.debug = value ? true : false;
			return;
		case WINDOW_ATTR_HINT_CONTEXT_ERROR_SUPPRESSION:
			_plaf.contextCfg.noerror = value ? true : false;
			return;
		case WINDOW_ATTR_HINT_OPENGL_PROFILE:
			_plaf.contextCfg.profile = value;
			return;
		case WINDOW_ATTR_HINT_CONTEXT_RELEASE_BEHAVIOR:
			_plaf.contextCfg.release = value;
			return;
		case WINDOW_HINT_REFRESH_RATE:
			_plaf.desiredRefreshRate = value;
			return;
	}

	_plafInputError("Invalid window hint 0x%08X", hint);
}

void plafDestroyWindow(plafWindow* window)
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
	if (window == _plaf.contextSlot) {
		plafMakeContextCurrent(NULL);
	}

	_plafDestroyWindow(window);

	// Unlink window from global linked list
	{
		plafWindow** prev = &_plaf.windowListHead;

		while (*prev != window)
			prev = &((*prev)->next);

		*prev = window->next;
	}

	_plaf_free(window->title);
	_plaf_free(window);
}

int plafWindowShouldClose(plafWindow* window) {
	return window->shouldClose;
}

void plafSetWindowShouldClose(plafWindow* window, int value) {
	window->shouldClose = value;
}

const char* plafGetWindowTitle(plafWindow* window) {
	return window->title;
}

void plafSetWindowTitle(plafWindow* window, const char* title) {
	char* prev = window->title;
	window->title = _plaf_strdup(title);
	_plafSetWindowTitle(window, window->title);
	_plaf_free(prev);
}

void plafSetWindowIcon(plafWindow* window, int count, const plafImageData* images)
{
	int i;

	if (count < 0)
	{
		_plafInputError("Invalid image count for window icon");
		return;
	}

	for (i = 0; i < count; i++)
	{
		if (images[i].width <= 0 || images[i].height <= 0)
		{
			_plafInputError("Invalid image dimensions for window icon");
			return;
		}
	}

	_plafSetWindowIcon(window, count, images);
}

void plafGetWindowPos(plafWindow* window, int* xpos, int* ypos) {
	if (xpos)
		*xpos = 0;
	if (ypos)
		*ypos = 0;
	_plafGetWindowPos(window, xpos, ypos);
}

void plafSetWindowPos(plafWindow* window, int xpos, int ypos) {
	if (window->monitor) {
		return;
	}
	_plafSetWindowPos(window, xpos, ypos);
}

void plafGetWindowSize(plafWindow* window, int* width, int* height) {
	if (width)
		*width = 0;
	if (height)
		*height = 0;
	_plafGetWindowSize(window, width, height);
}

void plafSetWindowSize(plafWindow* window, int width, int height) {
	window->videoMode.width  = width;
	window->videoMode.height = height;
	_plafSetWindowSize(window, width, height);
}

void plafSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight)
{
	if (minwidth != DONT_CARE && minheight != DONT_CARE)
	{
		if (minwidth < 0 || minheight < 0)
		{
			_plafInputError("Invalid window minimum size %ix%i", minwidth, minheight);
			return;
		}
	}

	if (maxwidth != DONT_CARE && maxheight != DONT_CARE)
	{
		if (maxwidth < 0 || maxheight < 0 ||
			maxwidth < minwidth || maxheight < minheight)
		{
			_plafInputError("Invalid window maximum size %ix%i", maxwidth, maxheight);
			return;
		}
	}

	window->minwidth  = minwidth;
	window->minheight = minheight;
	window->maxwidth  = maxwidth;
	window->maxheight = maxheight;

	if (window->monitor || !window->resizable)
		return;

	_plafSetWindowSizeLimits(window, minwidth, minheight, maxwidth, maxheight);
}

void plafGetFramebufferSize(plafWindow* window, int* width, int* height) {
	if (width)
		*width = 0;
	if (height)
		*height = 0;
	_plafGetFramebufferSize(window, width, height);
}

void plafGetWindowFrameSize(plafWindow* window, int* left, int* top, int* right, int* bottom) {
	if (left)
		*left = 0;
	if (top)
		*top = 0;
	if (right)
		*right = 0;
	if (bottom)
		*bottom = 0;
	_plafGetWindowFrameSize(window, left, top, right, bottom);
}

void plafGetWindowContentScale(plafWindow* window, float* xscale, float* yscale) {
	if (xscale)
		*xscale = 0.f;
	if (yscale)
		*yscale = 0.f;
	_plafGetWindowContentScale(window, xscale, yscale);
}

void plafSetWindowOpacity(plafWindow* window, float opacity) {
	if (opacity != opacity || opacity < 0.f || opacity > 1.f)
	{
		_plafInputError("Invalid window opacity %f", opacity);
		return;
	}

	_plafSetWindowOpacity(window, opacity);
}

void plafMaximizeWindow(plafWindow* window) {
	if (window->monitor)
		return;

	_plafMaximizeWindow(window);
}

void plafShowWindow(plafWindow* window) {
	if (!window->monitor) {
		_plafShowWindow(window);
	}
}

void plafHideWindow(plafWindow* window) {
	if (window->monitor)
		return;

	_plafHideWindow(window);
}

int plafGetWindowAttrib(plafWindow* window, int attrib) {
	switch (attrib)
	{
		case WINDOW_ATTR_FOCUSED:
			return _plafWindowFocused(window);
		case WINDOW_ATTR_MINIMIZED:
			return _plafWindowMinimized(window);
		case WINDOW_ATTR_VISIBLE:
			return _plafWindowVisible(window);
		case WINDOW_ATTR_HINT_MAXIMIZED:
			return _plafWindowMaximized(window);
		case WINDOW_ATTR_HOVERED:
			return _plafWindowHovered(window);
		case WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH:
			return window->mousePassthrough;
		case WINDOW_ATTR_HINT_TRANSPARENT_FRAMEBUFFER:
			return _plafFramebufferTransparent(window);
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

	_plafInputError("Invalid window attribute 0x%08X", attrib);
	return 0;
}

void plafSetWindowAttrib(plafWindow* window, int attrib, int value) {
	value = value ? true : false;

	switch (attrib)
	{
		case WINDOW_ATTR_HINT_RESIZABLE:
			window->resizable = value;
			if (!window->monitor)
				_plafSetWindowResizable(window, value);
			return;

		case WINDOW_ATTR_HINT_DECORATED:
			window->decorated = value;
			if (!window->monitor)
				_plafSetWindowDecorated(window, value);
			return;

		case WINDOW_ATTR_HINT_FLOATING:
			window->floating = value;
			if (!window->monitor)
				_plafSetWindowFloating(window, value);
			return;

		case WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH:
			window->mousePassthrough = value;
			_plafSetWindowMousePassthrough(window, value);
			return;
	}

	_plafInputError("Invalid window attribute 0x%08X", attrib);
}

plafMonitor* plafGetWindowMonitor(plafWindow* window) {
	return window->monitor;
}

void plafSetWindowMonitor(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate) {
	if (width <= 0 || height <= 0)
	{
		_plafInputError("Invalid window size %ix%i", width, height);
		return;
	}

	if (refreshRate < 0 && refreshRate != DONT_CARE)
	{
		_plafInputError("Invalid refresh rate %i", refreshRate);
		return;
	}

	window->videoMode.width       = width;
	window->videoMode.height      = height;
	window->videoMode.refreshRate = refreshRate;
	_plafSetWindowMonitor(window, monitor, xpos, ypos, width, height, refreshRate);
}

windowPosFunc plafSetWindowPosCallback(plafWindow* window, windowPosFunc cbfun) {
	SWAP(windowPosFunc, window->posCallback, cbfun);
	return cbfun;
}

windowSizeFunc plafSetWindowSizeCallback(plafWindow* window, windowSizeFunc cbfun) {
	SWAP(windowSizeFunc, window->sizeCallback, cbfun);
	return cbfun;
}

windowCloseFunc plafSetWindowCloseCallback(plafWindow* window, windowCloseFunc cbfun) {
	SWAP(windowCloseFunc, window->closeCallback, cbfun);
	return cbfun;
}

windowRefreshFunc plafSetWindowRefreshCallback(plafWindow* window, windowRefreshFunc cbfun) {
	SWAP(windowRefreshFunc, window->refreshCallback, cbfun);
	return cbfun;
}

windowFocusFunc plafSetWindowFocusCallback(plafWindow* window, windowFocusFunc cbfun) {
	SWAP(windowFocusFunc, window->focusCallback, cbfun);
	return cbfun;
}

windowMinimizeFunc plafSetWindowMinimizeCallback(plafWindow* window, windowMinimizeFunc cbfun) {
	SWAP(windowMinimizeFunc, window->minimizeCallback, cbfun);
	return cbfun;
}

windowMaximizeFunc plafSetWindowMaximizeCallback(plafWindow* window, windowMaximizeFunc cbfun) {
	SWAP(windowMaximizeFunc, window->maximizeCallback, cbfun);
	return cbfun;
}

frameBufferSizeFunc plafSetFramebufferSizeCallback(plafWindow* window, frameBufferSizeFunc cbfun) {
	SWAP(frameBufferSizeFunc, window->fbsizeCallback, cbfun);
	return cbfun;
}

windowContextScaleFunc plafSetWindowContentScaleCallback(plafWindow* window, windowContextScaleFunc cbfun) {
	SWAP(windowContextScaleFunc, window->scaleCallback, cbfun);
	return cbfun;
}

void plafWaitEventsTimeout(double timeout)
{
	if (timeout != timeout || timeout < 0.0 || timeout > DBL_MAX)
	{
		_plafInputError("Invalid time %f", timeout);
		return;
	}
	_plafWaitEventsTimeout(timeout);
}
