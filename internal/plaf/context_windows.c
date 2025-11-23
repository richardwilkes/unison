#include "platform.h"

#if defined(_WIN32)

// Return the value corresponding to the specified attribute
static int findPixelFormatAttribValueWGL(const int* attribs, int attribCount, const int* values, int attrib) {
	for (int i = 0;  i < attribCount;  i++) {
		if (attribs[i] == attrib) {
			return values[i];
		}
	}
	_plafInputError("WGL: Unknown pixel format attribute requested");
	return 0;
}

// Return a list of available and usable framebuffer configs
static int choosePixelFormatWGL(plafWindow* window, const plafFrameBufferCfg* fbconfig) {
	int attribs[40];
	int attribCount = 0;
	int nativeCount = DescribePixelFormat(window->context.wglDC, 1, sizeof(PIXELFORMATDESCRIPTOR), NULL);
	if (_plaf.wglARB_pixel_format) {
		attribs[attribCount++] = WGL_SUPPORT_OPENGL_ARB;
		attribs[attribCount++] = WGL_DRAW_TO_WINDOW_ARB;
		attribs[attribCount++] = WGL_PIXEL_TYPE_ARB;
		attribs[attribCount++] = WGL_ACCELERATION_ARB;
		attribs[attribCount++] = WGL_RED_BITS_ARB;
		attribs[attribCount++] = WGL_RED_SHIFT_ARB;
		attribs[attribCount++] = WGL_GREEN_BITS_ARB;
		attribs[attribCount++] = WGL_GREEN_SHIFT_ARB;
		attribs[attribCount++] = WGL_BLUE_BITS_ARB;
		attribs[attribCount++] = WGL_BLUE_SHIFT_ARB;
		attribs[attribCount++] = WGL_ALPHA_BITS_ARB;
		attribs[attribCount++] = WGL_ALPHA_SHIFT_ARB;
		attribs[attribCount++] = WGL_DEPTH_BITS_ARB;
		attribs[attribCount++] = WGL_STENCIL_BITS_ARB;
		attribs[attribCount++] = WGL_ACCUM_BITS_ARB;
		attribs[attribCount++] = WGL_ACCUM_RED_BITS_ARB;
		attribs[attribCount++] = WGL_ACCUM_GREEN_BITS_ARB;
		attribs[attribCount++] = WGL_ACCUM_BLUE_BITS_ARB;
		attribs[attribCount++] = WGL_ACCUM_ALPHA_BITS_ARB;
		attribs[attribCount++] = WGL_AUX_BUFFERS_ARB;
		attribs[attribCount++] = WGL_DOUBLE_BUFFER_ARB;
		if (_plaf.wglARB_multisample) {
			attribs[attribCount++] = WGL_SAMPLES_ARB;
		}
		if (_plaf.wglARB_framebuffer_sRGB || _plaf.wglEXT_framebuffer_sRGB) {
			attribs[attribCount++] = WGL_FRAMEBUFFER_SRGB_CAPABLE_ARB;
		}
		const int attrib = WGL_NUMBER_PIXEL_FORMATS_ARB;
		int extensionCount;
		if (!_plaf.wglGetPixelFormatAttribivARB(window->context.wglDC, 1, 0, 1, &attrib, &extensionCount)) {
			_plafInputError("WGL: Failed to retrieve pixel format attribute");
			return 0;
		}
		nativeCount = _plaf_min(nativeCount, extensionCount);
	}

	int usableCount = 0;
	plafFrameBufferCfg* usableConfigs = _plaf_calloc(nativeCount, sizeof(plafFrameBufferCfg));
	for (int pixFmt = 1;  pixFmt <= nativeCount;  pixFmt++) {
		plafFrameBufferCfg* u = usableConfigs + usableCount;
		if (_plaf.wglARB_pixel_format) {
			int values[sizeof(attribs) / sizeof(attribs[0])];
			if (!_plaf.wglGetPixelFormatAttribivARB(window->context.wglDC, pixFmt, 0, attribCount, attribs, values)) {
				_plafInputError("WGL: Failed to retrieve pixel format attributes");
				_plaf_free(usableConfigs);
				return 0;
			}
			if (!findPixelFormatAttribValueWGL(attribs, attribCount, values, WGL_SUPPORT_OPENGL_ARB) ||
				!findPixelFormatAttribValueWGL(attribs, attribCount, values, WGL_DRAW_TO_WINDOW_ARB) ||
				findPixelFormatAttribValueWGL(attribs, attribCount, values, WGL_PIXEL_TYPE_ARB) != WGL_TYPE_RGBA_ARB ||
				findPixelFormatAttribValueWGL(attribs, attribCount, values, WGL_ACCELERATION_ARB) == WGL_NO_ACCELERATION_ARB ||
				findPixelFormatAttribValueWGL(attribs, attribCount, values, WGL_DOUBLE_BUFFER_ARB) != fbconfig->doublebuffer) {
				continue;
			}
			u->redBits = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_RED_BITS_ARB);
			u->greenBits = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_GREEN_BITS_ARB);
			u->blueBits = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_BLUE_BITS_ARB);
			u->alphaBits = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_ALPHA_BITS_ARB);
			u->depthBits = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_DEPTH_BITS_ARB);
			u->stencilBits = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_STENCIL_BITS_ARB);
			u->accumRedBits = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_ACCUM_RED_BITS_ARB);
			u->accumGreenBits = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_ACCUM_GREEN_BITS_ARB);
			u->accumBlueBits = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_ACCUM_BLUE_BITS_ARB);
			u->accumAlphaBits = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_ACCUM_ALPHA_BITS_ARB);
			u->auxBuffers = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_AUX_BUFFERS_ARB);
			if (_plaf.wglARB_multisample) {
				u->samples = findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_SAMPLES_ARB);
			}
			if (_plaf.wglARB_framebuffer_sRGB || _plaf.wglEXT_framebuffer_sRGB) {
				if (findPixelFormatAttribValueWGL(attribs, attribCount, values,WGL_FRAMEBUFFER_SRGB_CAPABLE_ARB)) {
					u->sRGB = true;
				}
			}
		} else {
			PIXELFORMATDESCRIPTOR pfd;
			if (!DescribePixelFormat(window->context.wglDC, pixFmt, sizeof(PIXELFORMATDESCRIPTOR), &pfd)) {
				_plafInputError("WGL: Failed to describe pixel format");
				_plaf_free(usableConfigs);
				return 0;
			}
			if (!(pfd.dwFlags & PFD_DRAW_TO_WINDOW) ||
				!(pfd.dwFlags & PFD_SUPPORT_OPENGL) ||
				(!(pfd.dwFlags & PFD_GENERIC_ACCELERATED) && (pfd.dwFlags & PFD_GENERIC_FORMAT)) ||
				pfd.iPixelType != PFD_TYPE_RGBA ||
				(!!(pfd.dwFlags & PFD_DOUBLEBUFFER) != fbconfig->doublebuffer)) {
				continue;
			}
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
		u->handle = pixFmt;
		usableCount++;
	}

	if (!usableCount) {
		_plafInputError("WGL: The driver does not appear to support OpenGL");
		_plaf_free(usableConfigs);
		return 0;
	}

	const plafFrameBufferCfg* closest = _plafChooseFBConfig(fbconfig, usableConfigs, usableCount);
	_plaf_free(usableConfigs);
	if (!closest) {
		_plafInputError("WGL: Failed to find a suitable pixel format");
		return 0;
	}
	return (int)closest->handle;
}

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
plafError* _plafCreateOpenGLContext(plafWindow* window, plafWindow* share, const plafFrameBufferCfg* fbconfig) {
	int attribs[40];
	int pixelFormat;
	PIXELFORMATDESCRIPTOR pfd;
	HGLRC shareCtx = NULL;

	if (share) {
		shareCtx = share->context.wglGLRC;
	}

	window->context.wglDC = GetDC(window->win32Window);
	if (!window->context.wglDC) {
		return _plafNewError("WGL: Failed to retrieve DC for window");
	}

	pixelFormat = choosePixelFormatWGL(window, fbconfig);
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
		SET_ATTRIB(WGL_CONTEXT_MAJOR_VERSION_ARB, 3);
		SET_ATTRIB(WGL_CONTEXT_MINOR_VERSION_ARB, 2);
		SET_ATTRIB(0, 0);
		window->context.wglGLRC = _plaf.wglCreateContextAttribsARB(window->context.wglDC, shareCtx, attribs);
		if (!window->context.wglGLRC) {
			const DWORD error = GetLastError();
			if (error == (0xc0070000 | ERROR_INVALID_VERSION_ARB)) {
				return _plafNewError("WGL: Driver does not support OpenGL version 3.2");
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
