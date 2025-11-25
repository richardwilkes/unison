#include "platform.h"

#if defined(__linux__)

#define GLX_VENDOR 1
#define GLX_DOUBLEBUFFER 5
#define GLX_RED_SIZE 8
#define GLX_GREEN_SIZE 9
#define GLX_BLUE_SIZE 10
#define GLX_ALPHA_SIZE 11
#define GLX_DEPTH_SIZE 12
#define GLX_STENCIL_SIZE 13
#define GLX_ACCUM_RED_SIZE 14
#define GLX_ACCUM_GREEN_SIZE 15
#define GLX_ACCUM_BLUE_SIZE 16
#define GLX_ACCUM_ALPHA_SIZE 17
#define GLX_RGBA_BIT 0x00000001
#define GLX_WINDOW_BIT 0x00000001
#define GLX_DRAWABLE_TYPE 0x8010
#define GLX_RENDER_TYPE 0x8011
#define GLX_RGBA_TYPE 0x8014
#define GLX_SAMPLES 0x186a1
#define GLX_FRAMEBUFFER_SRGB_CAPABLE_ARB 0x20b2
#define GLX_CONTEXT_MAJOR_VERSION_ARB 0x2091
#define GLX_CONTEXT_MINOR_VERSION_ARB 0x2092

// Returns the specified attribute of the specified GLXFBConfig
static int getGLXFBConfigAttrib(GLXFBConfig fbconfig, int attrib)
{
	int value;
	_plaf.glxGetFBConfigAttrib(_plaf.x11Display, fbconfig, attrib, &value);
	return value;
}

// Return the GLXFBConfig most closely matching the specified hints
bool _plafChooseGLXFBConfig(const plafFrameBufferCfg* desired, GLXFBConfig* result) {
	GLXFBConfig* nativeConfigs;
	plafFrameBufferCfg* usableConfigs;
	const plafFrameBufferCfg* closest;
	int nativeCount, usableCount;
	const char* vendor;

	nativeConfigs = _plaf.glxGetFBConfigs(_plaf.x11Display, _plaf.x11Screen, &nativeCount);
	if (!nativeConfigs || !nativeCount) {
		return false;
	}

	usableConfigs = _plaf_calloc(nativeCount, sizeof(plafFrameBufferCfg));
	usableCount = 0;

	for (int i = 0;  i < nativeCount;  i++) {
		const GLXFBConfig n = nativeConfigs[i];
		plafFrameBufferCfg* u = usableConfigs + usableCount;

		// Only consider RGBA GLXFBConfigs
		if (!(getGLXFBConfigAttrib(n, GLX_RENDER_TYPE) & GLX_RGBA_BIT))
			continue;

		// Only consider window GLXFBConfigs
		if (!(getGLXFBConfigAttrib(n, GLX_DRAWABLE_TYPE) & GLX_WINDOW_BIT)) {
			continue;
		}

		// Only consider double-buffered GLXFBConfigs
		if (!getGLXFBConfigAttrib(n, GLX_DOUBLEBUFFER)) {
			continue;
		}

		if (desired->transparent)
		{
			XVisualInfo* vi = _plaf.glxGetVisualFromFBConfig(_plaf.x11Display, n);
			if (vi)
			{
				u->transparent = _plafIsVisualTransparent(vi->visual);
				_plaf.xlibFree(vi);
			}
		}

		u->redBits = getGLXFBConfigAttrib(n, GLX_RED_SIZE);
		u->greenBits = getGLXFBConfigAttrib(n, GLX_GREEN_SIZE);
		u->blueBits = getGLXFBConfigAttrib(n, GLX_BLUE_SIZE);

		u->alphaBits = getGLXFBConfigAttrib(n, GLX_ALPHA_SIZE);
		u->depthBits = getGLXFBConfigAttrib(n, GLX_DEPTH_SIZE);
		u->stencilBits = getGLXFBConfigAttrib(n, GLX_STENCIL_SIZE);

		u->accumRedBits = getGLXFBConfigAttrib(n, GLX_ACCUM_RED_SIZE);
		u->accumGreenBits = getGLXFBConfigAttrib(n, GLX_ACCUM_GREEN_SIZE);
		u->accumBlueBits = getGLXFBConfigAttrib(n, GLX_ACCUM_BLUE_SIZE);
		u->accumAlphaBits = getGLXFBConfigAttrib(n, GLX_ACCUM_ALPHA_SIZE);

		if (_plaf.glxARB_multisample)
			u->samples = getGLXFBConfigAttrib(n, GLX_SAMPLES);

		if (_plaf.glxARB_framebuffer_sRGB || _plaf.glxEXT_framebuffer_sRGB)
			u->sRGB = getGLXFBConfigAttrib(n, GLX_FRAMEBUFFER_SRGB_CAPABLE_ARB);

		u->handle = (uintptr_t) n;
		usableCount++;
	}

	closest = _plafChooseFBConfig(desired, usableConfigs, usableCount);
	if (closest)
		*result = (GLXFBConfig) closest->handle;

	_plaf.xlibFree(nativeConfigs);
	_plaf_free(usableConfigs);

	return closest != NULL;
}

static void makeContextCurrentGLX(plafWindow* window) {
	if (window) {
		if (_plaf.glxMakeCurrent(_plaf.x11Display, window->context.glxWindow, window->context.glxHandle)) {
			_plaf.wndWithCurrentCtx = window;
			return;
		}
		_plaf.wndWithCurrentCtx = NULL;
		return;
	}
	_plaf.wndWithCurrentCtx = NULL;
	_plaf.glxMakeCurrent(_plaf.x11Display, None, NULL);
}

static void swapBuffersGLX(plafWindow* window) {
	_plaf.glxSwapBuffers(_plaf.x11Display, window->context.glxWindow);
}

static bool extensionSupportedGLX(const char* extension) {
	const char* extensions = _plaf.glxQueryExtensionsString(_plaf.x11Display, _plaf.x11Screen);
	if (extensions) {
		if (_plafStringInExtensionString(extension, extensions)) {
			return true;
		}
	}
	return false;
}

static glFunc getProcAddressGLX(const char* procname)
{
	if (_plaf.glxGetProcAddress)
		return _plaf.glxGetProcAddress((const unsigned char*) procname);
	else if (_plaf.glxGetProcAddressARB)
		return _plaf.glxGetProcAddressARB((const unsigned char*) procname);
	else
	{
		// NOTE: glvnd provides GLX 1.4, so this can only happen with libGL
		return _plafGetModuleSymbol(_plaf.glxHandle, procname);
	}
}

static void destroyContextGLX(plafWindow* window)
{
	if (window->context.glxWindow)
	{
		_plaf.glxDestroyWindow(_plaf.x11Display, window->context.glxWindow);
		window->context.glxWindow = None;
	}

	if (window->context.glxHandle)
	{
		_plaf.glxDestroyContext(_plaf.x11Display, window->context.glxHandle);
		window->context.glxHandle = NULL;
	}
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Initialize GLX
bool _plafInitOpenGL(void) {
	if (_plaf.glxHandle) {
		return true;
	}
	const char* sonames[] = {
		"libGLX.so.0",
		"libGL.so.1",
		"libGL.so",
		NULL
	};
	for (int i = 0;  sonames[i];  i++) {
		_plaf.glxHandle = _plafLoadModule(sonames[i]);
		if (_plaf.glxHandle) {
			break;
		}
	}
	if (!_plaf.glxHandle) {
		return false;
	}
	_plaf.glxGetFBConfigs = (FN_GLXGETFBCONFIGS)_plafGetModuleSymbol(_plaf.glxHandle, "glXGetFBConfigs");
	_plaf.glxGetFBConfigAttrib = (FN_GLXGETFBCONFIGATTRIB)_plafGetModuleSymbol(_plaf.glxHandle, "glXGetFBConfigAttrib");
	_plaf.glxGetClientString = (FN_GLXGETCLIENTSTRING)_plafGetModuleSymbol(_plaf.glxHandle, "glXGetClientString");
	_plaf.glxQueryExtension = (FN_GLXQUERYEXTENSION)_plafGetModuleSymbol(_plaf.glxHandle, "glXQueryExtension");
	_plaf.glxQueryVersion = (FN_GLXQUERYVERSION)_plafGetModuleSymbol(_plaf.glxHandle, "glXQueryVersion");
	_plaf.glxDestroyContext = (FN_GLXDESTROYCONTEXT)_plafGetModuleSymbol(_plaf.glxHandle, "glXDestroyContext");
	_plaf.glxMakeCurrent = (FN_GLXMAKECURRENT)_plafGetModuleSymbol(_plaf.glxHandle, "glXMakeCurrent");
	_plaf.glxSwapBuffers = (FN_GLXSWAPBUFFERS)_plafGetModuleSymbol(_plaf.glxHandle, "glXSwapBuffers");
	_plaf.glxQueryExtensionsString = (FN_GLXQUERYEXTENSIONSSTRING)_plafGetModuleSymbol(_plaf.glxHandle, "glXQueryExtensionsString");
	_plaf.glxCreateNewContext = (FN_GLXCREATENEWCONTEXT)_plafGetModuleSymbol(_plaf.glxHandle, "glXCreateNewContext");
	_plaf.glxCreateWindow = (FN_GLXCREATEWINDOW)_plafGetModuleSymbol(_plaf.glxHandle, "glXCreateWindow");
	_plaf.glxDestroyWindow = (FN_GLXDESTROYWINDOW)_plafGetModuleSymbol(_plaf.glxHandle, "glXDestroyWindow");
	_plaf.glxGetVisualFromFBConfig = (FN_GLXGETVISUALFROMFBCONFIG)_plafGetModuleSymbol(_plaf.glxHandle, "glXGetVisualFromFBConfig");
	if (!_plaf.glxGetFBConfigs ||
		!_plaf.glxGetFBConfigAttrib ||
		!_plaf.glxGetClientString ||
		!_plaf.glxQueryExtension ||
		!_plaf.glxQueryVersion ||
		!_plaf.glxDestroyContext ||
		!_plaf.glxMakeCurrent ||
		!_plaf.glxSwapBuffers ||
		!_plaf.glxQueryExtensionsString ||
		!_plaf.glxCreateNewContext ||
		!_plaf.glxCreateWindow ||
		!_plaf.glxDestroyWindow ||
		!_plaf.glxGetVisualFromFBConfig) {
		return false;
	}
	_plaf.glxGetProcAddress = (FN_GLXGETPROCADDRESS)_plafGetModuleSymbol(_plaf.glxHandle, "glXGetProcAddress");
	_plaf.glxGetProcAddressARB = (FN_GLXGETPROCADDRESS)_plafGetModuleSymbol(_plaf.glxHandle, "glXGetProcAddressARB");
	int errorBase;
	int eventBase;
	if (!_plaf.glxQueryExtension(_plaf.x11Display, &_plaf.glxErrorBase, &eventBase)) {
		return false;
	}
	int major;
	int minor;
	if (!_plaf.glxQueryVersion(_plaf.x11Display, &major, &minor)) {
		return false;
	}
	if (major == 1 && minor < 3) {
		return false;
	}
	if (extensionSupportedGLX("GLX_ARB_multisample")) {
		_plaf.glxARB_multisample = true;
	}
	if (extensionSupportedGLX("GLX_ARB_framebuffer_sRGB")) {
		_plaf.glxARB_framebuffer_sRGB = true;
	}
	if (extensionSupportedGLX("GLX_EXT_framebuffer_sRGB")) {
		_plaf.glxEXT_framebuffer_sRGB = true;
	}
	if (extensionSupportedGLX("GLX_ARB_create_context")) {
		_plaf.glxCreateContextAttribsARB = (FN_GLXCREATECONTEXTATTRIBSARB)getProcAddressGLX("glXCreateContextAttribsARB");
		if (_plaf.glxCreateContextAttribsARB) {
			_plaf.glxARB_create_context = true;
		}
	}
	return true;
}

// Terminate GLX
//
void _plafTerminateOpenGL(void) {
	// NOTE: This function must not call any X11 functions, as it is called after XCloseDisplay
	if (_plaf.glxHandle) {
		_plafFreeModule(_plaf.glxHandle);
		_plaf.glxHandle = NULL;
	}
}

// Create the OpenGL or OpenGL ES context
bool _plafCreateOpenGLContext(plafWindow* window, plafWindow* share, const plafFrameBufferCfg* fbconfig) {
	GLXFBConfig native = NULL;
	GLXContext shareCtx = NULL;
	if (share) {
		shareCtx = share->context.glxHandle;
	}
	if (!_plafChooseGLXFBConfig(fbconfig, &native)) {
		return false;
	}
	_plafGrabErrorHandler();
	if (_plaf.glxARB_create_context) {
		int attribs[] = {
			GLX_CONTEXT_MAJOR_VERSION_ARB, 3,
			GLX_CONTEXT_MINOR_VERSION_ARB, 2,
			0
		};
		window->context.glxHandle = _plaf.glxCreateContextAttribsARB(_plaf.x11Display, native, shareCtx, True, attribs);
	} else {
		window->context.glxHandle = _plaf.glxCreateNewContext(_plaf.x11Display, native, GLX_RGBA_TYPE, shareCtx, True);
	}
	_plafReleaseErrorHandler();
	if (!window->context.glxHandle) {
		return false;
	}
	window->context.glxWindow = _plaf.glxCreateWindow(_plaf.x11Display, native, window->x11Window, NULL);
	if (!window->context.glxWindow) {
		return false;
	}
	window->context.glxFBConfig = native;
	window->context.makeCurrent = makeContextCurrentGLX;
	window->context.swapBuffers = swapBuffersGLX;
	window->context.extensionSupported = extensionSupportedGLX;
	window->context.getProcAddress = getProcAddressGLX;
	window->context.destroy = destroyContextGLX;
	return true;
}



//////////////////////////////////////////////////////////////////////////
//////                        PLAF native API                       //////
//////////////////////////////////////////////////////////////////////////

GLXContext plafGetGLXContext(plafWindow* window) {
	return window->context.glxHandle;
}

GLXWindow plafGetGLXWindow(plafWindow* window) {
	return window->context.glxWindow;
}

int plafGetGLXFBConfig(plafWindow* window, GLXFBConfig* config) {
	*config = window->context.glxFBConfig;
	return true;
}

#endif // __linux__
