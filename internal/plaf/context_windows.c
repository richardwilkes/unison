#include "platform.h"

#if defined(_WIN32)

// Return the value corresponding to the specified attribute
//
static int findPixelFormatAttribValueWGL(const int* attribs,
										 int attribCount,
										 const int* values,
										 int attrib)
{
	int i;

	for (i = 0;  i < attribCount;  i++)
	{
		if (attribs[i] == attrib)
			return values[i];
	}

	_plafInputError("WGL: Unknown pixel format attribute requested");
	return 0;
}

#define ADD_ATTRIB(a) \
{ \
	attribs[attribCount++] = a; \
}
#define FIND_ATTRIB_VALUE(a) \
	findPixelFormatAttribValueWGL(attribs, attribCount, values, a)

// Return a list of available and usable framebuffer configs
//
static int choosePixelFormatWGL(plafWindow* window,
								const plafCtxCfg* ctxconfig,
								const plafFrameBufferCfg* fbconfig)
{
	plafFrameBufferCfg* usableConfigs;
	const plafFrameBufferCfg* closest;
	int i, pixelFormat, nativeCount, usableCount = 0, attribCount = 0;
	int attribs[40];
	int values[sizeof(attribs) / sizeof(attribs[0])];

	nativeCount = DescribePixelFormat(window->context.wglDC,
									  1,
									  sizeof(PIXELFORMATDESCRIPTOR),
									  NULL);

	if (_plaf.wglARB_pixel_format)
	{
		ADD_ATTRIB(WGL_SUPPORT_OPENGL_ARB);
		ADD_ATTRIB(WGL_DRAW_TO_WINDOW_ARB);
		ADD_ATTRIB(WGL_PIXEL_TYPE_ARB);
		ADD_ATTRIB(WGL_ACCELERATION_ARB);
		ADD_ATTRIB(WGL_RED_BITS_ARB);
		ADD_ATTRIB(WGL_RED_SHIFT_ARB);
		ADD_ATTRIB(WGL_GREEN_BITS_ARB);
		ADD_ATTRIB(WGL_GREEN_SHIFT_ARB);
		ADD_ATTRIB(WGL_BLUE_BITS_ARB);
		ADD_ATTRIB(WGL_BLUE_SHIFT_ARB);
		ADD_ATTRIB(WGL_ALPHA_BITS_ARB);
		ADD_ATTRIB(WGL_ALPHA_SHIFT_ARB);
		ADD_ATTRIB(WGL_DEPTH_BITS_ARB);
		ADD_ATTRIB(WGL_STENCIL_BITS_ARB);
		ADD_ATTRIB(WGL_ACCUM_BITS_ARB);
		ADD_ATTRIB(WGL_ACCUM_RED_BITS_ARB);
		ADD_ATTRIB(WGL_ACCUM_GREEN_BITS_ARB);
		ADD_ATTRIB(WGL_ACCUM_BLUE_BITS_ARB);
		ADD_ATTRIB(WGL_ACCUM_ALPHA_BITS_ARB);
		ADD_ATTRIB(WGL_AUX_BUFFERS_ARB);
		ADD_ATTRIB(WGL_DOUBLE_BUFFER_ARB);

		if (_plaf.wglARB_multisample)
			ADD_ATTRIB(WGL_SAMPLES_ARB);

		if (_plaf.wglARB_framebuffer_sRGB || _plaf.wglEXT_framebuffer_sRGB)
			ADD_ATTRIB(WGL_FRAMEBUFFER_SRGB_CAPABLE_ARB);

		// NOTE: In a Parallels VM WGL_ARB_pixel_format returns fewer pixel formats than
		//       DescribePixelFormat, violating the guarantees of the extension spec
		// HACK: Iterate through the minimum of both counts

		const int attrib = WGL_NUMBER_PIXEL_FORMATS_ARB;
		int extensionCount;

		if (!_plaf.wglGetPixelFormatAttribivARB(window->context.wglDC, 1, 0, 1, &attrib, &extensionCount))
		{
			_plafInputError("WGL: Failed to retrieve pixel format attribute");
			return 0;
		}

		nativeCount = _plaf_min(nativeCount, extensionCount);
	}

	usableConfigs = _plaf_calloc(nativeCount, sizeof(plafFrameBufferCfg));

	for (i = 0;  i < nativeCount;  i++)
	{
		plafFrameBufferCfg* u = usableConfigs + usableCount;
		pixelFormat = i + 1;

		if (_plaf.wglARB_pixel_format)
		{
			// Get pixel format attributes through "modern" extension

			if (!_plaf.wglGetPixelFormatAttribivARB(window->context.wglDC, pixelFormat, 0, attribCount, attribs, values))
			{
				_plafInputError("WGL: Failed to retrieve pixel format attributes");

				_plaf_free(usableConfigs);
				return 0;
			}

			if (!FIND_ATTRIB_VALUE(WGL_SUPPORT_OPENGL_ARB) ||
				!FIND_ATTRIB_VALUE(WGL_DRAW_TO_WINDOW_ARB))
			{
				continue;
			}

			if (FIND_ATTRIB_VALUE(WGL_PIXEL_TYPE_ARB) != WGL_TYPE_RGBA_ARB)
				continue;

			if (FIND_ATTRIB_VALUE(WGL_ACCELERATION_ARB) == WGL_NO_ACCELERATION_ARB)
				continue;

			if (FIND_ATTRIB_VALUE(WGL_DOUBLE_BUFFER_ARB) != fbconfig->doublebuffer)
				continue;

			u->redBits = FIND_ATTRIB_VALUE(WGL_RED_BITS_ARB);
			u->greenBits = FIND_ATTRIB_VALUE(WGL_GREEN_BITS_ARB);
			u->blueBits = FIND_ATTRIB_VALUE(WGL_BLUE_BITS_ARB);
			u->alphaBits = FIND_ATTRIB_VALUE(WGL_ALPHA_BITS_ARB);

			u->depthBits = FIND_ATTRIB_VALUE(WGL_DEPTH_BITS_ARB);
			u->stencilBits = FIND_ATTRIB_VALUE(WGL_STENCIL_BITS_ARB);

			u->accumRedBits = FIND_ATTRIB_VALUE(WGL_ACCUM_RED_BITS_ARB);
			u->accumGreenBits = FIND_ATTRIB_VALUE(WGL_ACCUM_GREEN_BITS_ARB);
			u->accumBlueBits = FIND_ATTRIB_VALUE(WGL_ACCUM_BLUE_BITS_ARB);
			u->accumAlphaBits = FIND_ATTRIB_VALUE(WGL_ACCUM_ALPHA_BITS_ARB);

			u->auxBuffers = FIND_ATTRIB_VALUE(WGL_AUX_BUFFERS_ARB);

			if (_plaf.wglARB_multisample)
				u->samples = FIND_ATTRIB_VALUE(WGL_SAMPLES_ARB);

			if (_plaf.wglARB_framebuffer_sRGB ||
				_plaf.wglEXT_framebuffer_sRGB)
			{
				if (FIND_ATTRIB_VALUE(WGL_FRAMEBUFFER_SRGB_CAPABLE_ARB))
					u->sRGB = true;
			}
		}
		else
		{
			// Get pixel format attributes through legacy PFDs

			PIXELFORMATDESCRIPTOR pfd;

			if (!DescribePixelFormat(window->context.wglDC,
									 pixelFormat,
									 sizeof(PIXELFORMATDESCRIPTOR),
									 &pfd))
			{
				_plafInputError("WGL: Failed to describe pixel format");

				_plaf_free(usableConfigs);
				return 0;
			}

			if (!(pfd.dwFlags & PFD_DRAW_TO_WINDOW) ||
				!(pfd.dwFlags & PFD_SUPPORT_OPENGL))
			{
				continue;
			}

			if (!(pfd.dwFlags & PFD_GENERIC_ACCELERATED) &&
				(pfd.dwFlags & PFD_GENERIC_FORMAT))
			{
				continue;
			}

			if (pfd.iPixelType != PFD_TYPE_RGBA)
				continue;

			if (!!(pfd.dwFlags & PFD_DOUBLEBUFFER) != fbconfig->doublebuffer)
				continue;

			u->redBits = pfd.cRedBits;
			u->greenBits = pfd.cGreenBits;
			u->blueBits = pfd.cBlueBits;
			u->alphaBits = pfd.cAlphaBits;

			u->depthBits = pfd.cDepthBits;
			u->stencilBits = pfd.cStencilBits;

			u->accumRedBits = pfd.cAccumRedBits;
			u->accumGreenBits = pfd.cAccumGreenBits;
			u->accumBlueBits = pfd.cAccumBlueBits;
			u->accumAlphaBits = pfd.cAccumAlphaBits;

			u->auxBuffers = pfd.cAuxBuffers;
		}

		u->handle = pixelFormat;
		usableCount++;
	}

	if (!usableCount)
	{
		_plafInputError("WGL: The driver does not appear to support OpenGL");

		_plaf_free(usableConfigs);
		return 0;
	}

	closest = _plafChooseFBConfig(fbconfig, usableConfigs, usableCount);
	if (!closest)
	{
		_plafInputError("WGL: Failed to find a suitable pixel format");

		_plaf_free(usableConfigs);
		return 0;
	}

	pixelFormat = (int) closest->handle;
	_plaf_free(usableConfigs);

	return pixelFormat;
}

#undef ADD_ATTRIB
#undef FIND_ATTRIB_VALUE

static plafError* makeContextCurrentWGL(plafWindow* window) {
	if (window) {
		if (_plaf.wglMakeCurrent(window->context.wglDC, window->context.wglGLRC)) {
			_plaf.contextSlot = window;
		} else {
			_plaf.contextSlot = NULL;
			return _plafNewError("WGL: Failed to make context current");
		}
	} else {
		_plaf.contextSlot = NULL;
		if (!_plaf.wglMakeCurrent(NULL, NULL)) {
			return _plafNewError("WGL: Failed to clear current context");
		}
	}
	return NULL;
}

static void swapBuffersWGL(plafWindow* window)
{
	SwapBuffers(window->context.wglDC);
}

static void swapIntervalWGL(int interval)
{
	_plaf.contextSlot->context.wglInterval = interval;
	if (_plaf.wglEXT_swap_control)
		_plaf.wglSwapIntervalEXT(interval);
}

static bool extensionSupportedWGL(const char* extension) {
	const char* extensions = NULL;

	if (_plaf.wglGetExtensionsStringARB)
		extensions = _plaf.wglGetExtensionsStringARB(_plaf.wglGetCurrentDC());
	else if (_plaf.wglGetExtensionsStringEXT)
		extensions = _plaf.wglGetExtensionsStringEXT();

	if (!extensions)
		return false;

	return _plafStringInExtensionString(extension, extensions);
}

static glFunc getProcAddressWGL(const char* procname)
{
	const glFunc proc = (glFunc) _plaf.wglGetProcAddress(procname);
	if (proc)
		return proc;

	return (glFunc) _plafGetModuleSymbol(_plaf.wglInstance, procname);
}

static void destroyContextWGL(plafWindow* window)
{
	if (window->context.wglGLRC)
	{
		_plaf.wglDeleteContext(window->context.wglGLRC);
		window->context.wglGLRC = NULL;
	}
}

// Initialize WGL
plafError* _plafInitOpenGL(void) {
	if (_plaf.wglInstance) {
		return NULL;
	}
	_plaf.wglInstance = _plafLoadModule("opengl32.dll");
	if (!_plaf.wglInstance) {
		return _plafNewError("WGL: Failed to load opengl32.dll");
	}

	_plaf.wglCreateContext = (FN_wglCreateContext)_plafGetModuleSymbol(_plaf.wglInstance, "wglCreateContext");
	_plaf.wglDeleteContext = (FN_wglDeleteContext)_plafGetModuleSymbol(_plaf.wglInstance, "wglDeleteContext");
	_plaf.wglGetProcAddress = (FN_wglGetProcAddress)_plafGetModuleSymbol(_plaf.wglInstance, "wglGetProcAddress");
	_plaf.wglGetCurrentDC = (FN_wglGetCurrentDC)_plafGetModuleSymbol(_plaf.wglInstance, "wglGetCurrentDC");
	_plaf.wglGetCurrentContext = (FN_wglGetCurrentContext)_plafGetModuleSymbol(_plaf.wglInstance, "wglGetCurrentContext");
	_plaf.wglMakeCurrent = (FN_wglMakeCurrent)_plafGetModuleSymbol(_plaf.wglInstance, "wglMakeCurrent");
	_plaf.wglShareLists = (FN_wglShareLists)_plafGetModuleSymbol(_plaf.wglInstance, "wglShareLists");

	// NOTE: A dummy context has to be created for opengl32.dll to load the
	//       OpenGL ICD, from which we can then query WGL extensions
	// NOTE: This code will accept the Microsoft GDI ICD; accelerated context
	//       creation failure occurs during manual pixel format enumeration

	PIXELFORMATDESCRIPTOR pfd;
	HGLRC prc, rc;
	HDC pdc, dc;

	dc = GetDC(_plaf.win32HelperWindowHandle);

	ZeroMemory(&pfd, sizeof(pfd));
	pfd.nSize = sizeof(pfd);
	pfd.nVersion = 1;
	pfd.dwFlags = PFD_DRAW_TO_WINDOW | PFD_SUPPORT_OPENGL | PFD_DOUBLEBUFFER;
	pfd.iPixelType = PFD_TYPE_RGBA;
	pfd.cColorBits = 24;

	if (!SetPixelFormat(dc, ChoosePixelFormat(dc, &pfd), &pfd)) {
		return _plafNewError("WGL: Failed to set pixel format for dummy context");
	}

	rc = _plaf.wglCreateContext(dc);
	if (!rc) {
		return _plafNewError("WGL: Failed to create dummy context");
	}

	pdc = _plaf.wglGetCurrentDC();
	prc = _plaf.wglGetCurrentContext();

	if (!_plaf.wglMakeCurrent(dc, rc)) {
		_plaf.wglMakeCurrent(pdc, prc);
		_plaf.wglDeleteContext(rc);
		return _plafNewError("WGL: Failed to make dummy context current");
	}

	// NOTE: Functions must be loaded first as they're needed to retrieve the
	//       extension string that tells us whether the functions are supported
	_plaf.wglGetExtensionsStringEXT = (FN_WGLGETEXTENSIONSSTRINGEXT)_plaf.wglGetProcAddress("wglGetExtensionsStringEXT");
	_plaf.wglGetExtensionsStringARB = (FN_WGLGETEXTENSIONSSTRINGARB)_plaf.wglGetProcAddress("wglGetExtensionsStringARB");
	_plaf.wglCreateContextAttribsARB = (FN_WGLCREATECONTEXTATTRIBSARB)_plaf.wglGetProcAddress("wglCreateContextAttribsARB");
	_plaf.wglSwapIntervalEXT = (FN_WGLSWAPINTERVALEXT)_plaf.wglGetProcAddress("wglSwapIntervalEXT");
	_plaf.wglGetPixelFormatAttribivARB = (FN_WGLGETPIXELFORMATATTRIBIVARB)_plaf.wglGetProcAddress("wglGetPixelFormatAttribivARB");

	// NOTE: WGL_ARB_extensions_string and WGL_EXT_extensions_string are not
	//       checked below as we are already using them
	_plaf.wglARB_multisample = extensionSupportedWGL("WGL_ARB_multisample");
	_plaf.wglARB_framebuffer_sRGB = extensionSupportedWGL("WGL_ARB_framebuffer_sRGB");
	_plaf.wglEXT_framebuffer_sRGB = extensionSupportedWGL("WGL_EXT_framebuffer_sRGB");
	_plaf.wglARB_create_context = extensionSupportedWGL("WGL_ARB_create_context");
	_plaf.wglARB_create_context_robustness = extensionSupportedWGL("WGL_ARB_create_context_robustness");
	_plaf.wglEXT_swap_control = extensionSupportedWGL("WGL_EXT_swap_control");
	_plaf.wglARB_pixel_format = extensionSupportedWGL("WGL_ARB_pixel_format");

	_plaf.wglMakeCurrent(pdc, prc);
	_plaf.wglDeleteContext(rc);
	return NULL;
}

// Terminate WGL
//
void _plafTerminateOpenGL(void) {
	if (_plaf.wglInstance) {
		_plafFreeModule(_plaf.wglInstance);
	}
}

#define SET_ATTRIB(a, v) \
{ \
	attribs[index++] = a; \
	attribs[index++] = v; \
}

// Create the OpenGL or OpenGL ES context
plafError* _plafCreateOpenGLContext(plafWindow* window, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig) {
	int attribs[40];
	int pixelFormat;
	PIXELFORMATDESCRIPTOR pfd;
	HGLRC share = NULL;

	if (ctxconfig->share) {
		share = ctxconfig->share->context.wglGLRC;
	}

	window->context.wglDC = GetDC(window->win32Window);
	if (!window->context.wglDC) {
		return _plafNewError("WGL: Failed to retrieve DC for window");
	}

	pixelFormat = choosePixelFormatWGL(window, ctxconfig, fbconfig);
	if (!pixelFormat) {
		return _plafNewError("WGL: Failed to choose pixel format for window");
	}

	if (!DescribePixelFormat(window->context.wglDC, pixelFormat, sizeof(pfd), &pfd)) {
		return _plafNewError("WGL: Failed to retrieve PFD for selected pixel format");
	}

	if (!SetPixelFormat(window->context.wglDC, pixelFormat, &pfd)) {
		return _plafNewError("WGL: Failed to set selected pixel format");
	}

	if (_plaf.wglARB_create_context) {
		int index = 0, flags = 0;

		if (ctxconfig->robustness) {
			if (_plaf.wglARB_create_context_robustness) {
				if (ctxconfig->robustness == CONTEXT_ROBUSTNESS_NO_RESET_NOTIFICATION) {
					SET_ATTRIB(WGL_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB, WGL_NO_RESET_NOTIFICATION_ARB);
				} else if (ctxconfig->robustness == CONTEXT_ROBUSTNESS_LOSE_CONTEXT_ON_RESET) {
					SET_ATTRIB(WGL_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB, WGL_LOSE_CONTEXT_ON_RESET_ARB);
				}
				flags |= WGL_CONTEXT_ROBUST_ACCESS_BIT_ARB;
			}
		}

		// NOTE: Only request an explicitly versioned context when necessary, as
		//       explicitly requesting version 1.0 does not always return the
		//       highest version supported by the driver
		if (ctxconfig->major != 1 || ctxconfig->minor != 0) {
			SET_ATTRIB(WGL_CONTEXT_MAJOR_VERSION_ARB, ctxconfig->major);
			SET_ATTRIB(WGL_CONTEXT_MINOR_VERSION_ARB, ctxconfig->minor);
		}

		if (flags) {
			SET_ATTRIB(WGL_CONTEXT_FLAGS_ARB, flags);
		}

		SET_ATTRIB(0, 0);

		window->context.wglGLRC = _plaf.wglCreateContextAttribsARB(window->context.wglDC, share, attribs);
		if (!window->context.wglGLRC) {
			const DWORD error = GetLastError();
			if (error == (0xc0070000 | ERROR_INVALID_VERSION_ARB)) {
				return _plafNewError("WGL: Driver does not support OpenGL version %i.%i", ctxconfig->major, ctxconfig->minor);
			} else if (error == (0xc0070000 | ERROR_INCOMPATIBLE_DEVICE_CONTEXTS_ARB)) {
				return _plafNewError("WGL: The share context is not compatible with the requested context");
			}
			return _plafNewError("WGL: Failed to create OpenGL context");
		}
	} else {
		window->context.wglGLRC = _plaf.wglCreateContext(window->context.wglDC);
		if (!window->context.wglGLRC) {
			return _plafNewError("WGL: Failed to create OpenGL context");
		}

		if (share) {
			if (!_plaf.wglShareLists(share, window->context.wglGLRC)) {
				return _plafNewError("WGL: Failed to enable sharing with specified OpenGL context");
			}
		}
	}

	window->context.makeCurrent = makeContextCurrentWGL;
	window->context.swapBuffers = swapBuffersWGL;
	window->context.swapInterval = swapIntervalWGL;
	window->context.extensionSupported = extensionSupportedWGL;
	window->context.getProcAddress = getProcAddressWGL;
	window->context.destroy = destroyContextWGL;
	return NULL;
}

#undef SET_ATTRIB

HGLRC plafGetWGLContext(plafWindow* window) {
	return window->context.wglGLRC;
}

#endif // _WIN32
