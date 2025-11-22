#include "platform.h"

#if defined(__linux__)

#ifndef GLXBadProfileARB
 #define GLXBadProfileARB 13
#endif


// Returns the specified attribute of the specified GLXFBConfig
//
static int getGLXFBConfigAttrib(GLXFBConfig fbconfig, int attrib)
{
	int value;
	_plaf.glxGetFBConfigAttrib(_plaf.x11Display, fbconfig, attrib, &value);
	return value;
}

// Return the GLXFBConfig most closely matching the specified hints
//
static bool chooseGLXFBConfig(const plafFrameBufferCfg* desired, GLXFBConfig* result) {
	GLXFBConfig* nativeConfigs;
	plafFrameBufferCfg* usableConfigs;
	const plafFrameBufferCfg* closest;
	int nativeCount, usableCount;
	const char* vendor;
	bool trustWindowBit = true;

	// HACK: This is a (hopefully temporary) workaround for Chromium
	//       (VirtualBox GL) not setting the window bit on any GLXFBConfigs
	vendor = _plaf.glxGetClientString(_plaf.x11Display, GLX_VENDOR);
	if (vendor && strcmp(vendor, "Chromium") == 0)
		trustWindowBit = false;

	nativeConfigs = _plaf.glxGetFBConfigs(_plaf.x11Display, _plaf.x11Screen, &nativeCount);
	if (!nativeConfigs || !nativeCount)
	{
		_plafInputError("GLX: No GLXFBConfigs returned");
		return false;
	}

	usableConfigs = _plaf_calloc(nativeCount, sizeof(plafFrameBufferCfg));
	usableCount = 0;

	for (int i = 0;  i < nativeCount;  i++)
	{
		const GLXFBConfig n = nativeConfigs[i];
		plafFrameBufferCfg* u = usableConfigs + usableCount;

		// Only consider RGBA GLXFBConfigs
		if (!(getGLXFBConfigAttrib(n, GLX_RENDER_TYPE) & GLX_RGBA_BIT))
			continue;

		// Only consider window GLXFBConfigs
		if (!(getGLXFBConfigAttrib(n, GLX_DRAWABLE_TYPE) & GLX_WINDOW_BIT))
		{
			if (trustWindowBit)
				continue;
		}

		if (getGLXFBConfigAttrib(n, GLX_DOUBLEBUFFER) != desired->doublebuffer)
			continue;

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

		u->auxBuffers = getGLXFBConfigAttrib(n, GLX_AUX_BUFFERS);

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

// Create the OpenGL context using legacy API
//
static GLXContext createLegacyContextGLX(plafWindow* window,
										 GLXFBConfig fbconfig,
										 GLXContext share)
{
	return _plaf.glxCreateNewContext(_plaf.x11Display,
							   fbconfig,
							   GLX_RGBA_TYPE,
							   share,
							   True);
}

static plafError* makeContextCurrentGLX(plafWindow* window) {
	if (window) {
		if (!_plaf.glxMakeCurrent(_plaf.x11Display, window->context.glxWindow, window->context.glxHandle)) {
			return _plafNewError("GLX: Failed to make context current");
		}
	} else {
		if (!_plaf.glxMakeCurrent(_plaf.x11Display, None, NULL)) {
			return _plafNewError("GLX: Failed to clear current context");
		}
	}
	_plaf.contextSlot = window;
	return NULL;
}

static void swapBuffersGLX(plafWindow* window)
{
	_plaf.glxSwapBuffers(_plaf.x11Display, window->context.glxWindow);
}

static void swapIntervalGLX(int interval)
{
	if (_plaf.glxEXT_swap_control)
	{
		_plaf.glxSwapIntervalEXT(_plaf.x11Display,
								  _plaf.contextSlot->context.glxWindow,
								  interval);
	}
	else if (_plaf.glxSGI_swap_control)
	{
		if (interval > 0)
			_plaf.glxSwapIntervalSGI(interval);
	}
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
		return _plaf.glxGetProcAddress((const GLubyte*) procname);
	else if (_plaf.glxGetProcAddressARB)
		return _plaf.glxGetProcAddressARB((const GLubyte*) procname);
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
plafError* _plafInitOpenGL(void) {
	if (_plaf.glxHandle) {
		return NULL;
	}
	const char* sonames[] = {
		"libGLX.so.0",
		"libGL.so.1",
		"libGL.so",
		NULL
	};
	for (int i = 0;  sonames[i];  i++) {
		_plaf.glxHandle = _plafLoadModule(sonames[i]);
		if (_plaf.glxHandle)
			break;
	}
	if (!_plaf.glxHandle) {
		return _plafNewError("GLX: Failed to load GLX");
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
		return _plafNewError("GLX: Failed to load required entry points");
	}

	// NOTE: Unlike GLX 1.3 entry points these are not required to be present
	_plaf.glxGetProcAddress = (FN_GLXGETPROCADDRESS)_plafGetModuleSymbol(_plaf.glxHandle, "glXGetProcAddress");
	_plaf.glxGetProcAddressARB = (FN_GLXGETPROCADDRESS)_plafGetModuleSymbol(_plaf.glxHandle, "glXGetProcAddressARB");

	int errorBase;
	int eventBase;
	if (!_plaf.glxQueryExtension(_plaf.x11Display, &_plaf.glxErrorBase, &eventBase)) {
		return _plafNewError("GLX: GLX extension not found");
	}

	int major;
	int minor;
	if (!_plaf.glxQueryVersion(_plaf.x11Display, &major, &minor)) {
		return _plafNewError("GLX: Failed to query GLX version");
	}
	if (major == 1 && minor < 3) {
		return _plafNewError("GLX: GLX version 1.3 is required");
	}

	if (extensionSupportedGLX("GLX_EXT_swap_control")) {
		_plaf.glxSwapIntervalEXT = (FN_GLXSWAPINTERVALEXT)getProcAddressGLX("glXSwapIntervalEXT");
		if (_plaf.glxSwapIntervalEXT) {
			_plaf.glxEXT_swap_control = true;
		}
	}

	if (extensionSupportedGLX("GLX_SGI_swap_control")) {
		_plaf.glxSwapIntervalSGI = (FN_GLXSWAPINTERVALSGI)getProcAddressGLX("glXSwapIntervalSGI");
		if (_plaf.glxSwapIntervalSGI) {
			_plaf.glxSGI_swap_control = true;
		}
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

	if (extensionSupportedGLX("GLX_ARB_create_context_robustness")) {
		_plaf.glxARB_create_context_robustness = true;
	}

	if (extensionSupportedGLX("GLX_ARB_create_context_profile")) {
		_plaf.glxARB_create_context_profile = true;
	}

	if (extensionSupportedGLX("GLX_ARB_create_context_no_error")) {
		_plaf.glxARB_create_context_no_error = true;
	}

	if (extensionSupportedGLX("GLX_ARB_context_flush_control")) {
		_plaf.glxARB_context_flush_control = true;
	}
	return NULL;
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

#define SET_ATTRIB(a, v) \
{ \
	attribs[index++] = a; \
	attribs[index++] = v; \
}

// Create the OpenGL or OpenGL ES context
plafError* _plafCreateOpenGLContext(plafWindow* window, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig) {
	int attribs[40];
	GLXFBConfig native = NULL;
	GLXContext share = NULL;

	if (ctxconfig->share) {
		share = ctxconfig->share->context.glxHandle;
	}

	if (!chooseGLXFBConfig(fbconfig, &native)) {
		return _plafNewError("GLX: Failed to find a suitable GLXFBConfig");
	}

	if (ctxconfig->forward) {
		if (!_plaf.glxARB_create_context) {
			return _plafNewError("GLX: Forward compatibility requested but GLX_ARB_create_context_profile is unavailable");
		}
	}

	if (ctxconfig->profile) {
		if (!_plaf.glxARB_create_context || !_plaf.glxARB_create_context_profile) {
			return _plafNewError("GLX: An OpenGL profile requested but GLX_ARB_create_context_profile is unavailable");
		}
	}

	_plafGrabErrorHandler();

	if (_plaf.glxARB_create_context) {
		int index = 0, mask = 0, flags = 0;

		if (ctxconfig->forward) {
			flags |= GLX_CONTEXT_FORWARD_COMPATIBLE_BIT_ARB;
		}

		if (ctxconfig->profile == OPENGL_PROFILE_CORE) {
			mask |= GLX_CONTEXT_CORE_PROFILE_BIT_ARB;
		} else if (ctxconfig->profile == OPENGL_PROFILE_COMPAT) {
			mask |= GLX_CONTEXT_COMPATIBILITY_PROFILE_BIT_ARB;
		}

		if (ctxconfig->debug) {
			flags |= GLX_CONTEXT_DEBUG_BIT_ARB;
		}

		if (ctxconfig->robustness) {
			if (_plaf.glxARB_create_context_robustness) {
				if (ctxconfig->robustness == CONTEXT_ROBUSTNESS_NO_RESET_NOTIFICATION) {
					SET_ATTRIB(GLX_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB, GLX_NO_RESET_NOTIFICATION_ARB);
				} else if (ctxconfig->robustness == CONTEXT_ROBUSTNESS_LOSE_CONTEXT_ON_RESET) {
					SET_ATTRIB(GLX_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB, GLX_LOSE_CONTEXT_ON_RESET_ARB);
				}
				flags |= GLX_CONTEXT_ROBUST_ACCESS_BIT_ARB;
			}
		}

		if (ctxconfig->release) {
			if (_plaf.glxARB_context_flush_control) {
				if (ctxconfig->release == RELEASE_BEHAVIOR_NONE) {
					SET_ATTRIB(GLX_CONTEXT_RELEASE_BEHAVIOR_ARB, GLX_CONTEXT_RELEASE_BEHAVIOR_NONE_ARB);
				} else if (ctxconfig->release == RELEASE_BEHAVIOR_FLUSH) {
					SET_ATTRIB(GLX_CONTEXT_RELEASE_BEHAVIOR_ARB, GLX_CONTEXT_RELEASE_BEHAVIOR_FLUSH_ARB);
				}
			}
		}

		if (ctxconfig->noerror) {
			if (_plaf.glxARB_create_context_no_error) {
				SET_ATTRIB(GLX_CONTEXT_OPENGL_NO_ERROR_ARB, true);
			}
		}

		// NOTE: Only request an explicitly versioned context when necessary, as
		//       explicitly requesting version 1.0 does not always return the
		//       highest version supported by the driver
		if (ctxconfig->major != 1 || ctxconfig->minor != 0) {
			SET_ATTRIB(GLX_CONTEXT_MAJOR_VERSION_ARB, ctxconfig->major);
			SET_ATTRIB(GLX_CONTEXT_MINOR_VERSION_ARB, ctxconfig->minor);
		}

		if (mask) {
			SET_ATTRIB(GLX_CONTEXT_PROFILE_MASK_ARB, mask);
		}

		if (flags) {
			SET_ATTRIB(GLX_CONTEXT_FLAGS_ARB, flags);
		}

		SET_ATTRIB(None, None);

		window->context.glxHandle = _plaf.glxCreateContextAttribsARB(_plaf.x11Display, native, share, True, attribs);

		// HACK: This is a fallback for broken versions of the Mesa
		//       implementation of GLX_ARB_create_context_profile that fail
		//       default 1.0 context creation with a GLXBadProfileARB error in
		//       violation of the extension spec
		if (!window->context.glxHandle) {
			if (_plaf.x11ErrorCode == _plaf.glxErrorBase + GLXBadProfileARB &&
				ctxconfig->profile == OPENGL_PROFILE_ANY && ctxconfig->forward == false) {
				window->context.glxHandle = createLegacyContextGLX(window, native, share);
			}
		}
	} else {
		window->context.glxHandle = createLegacyContextGLX(window, native, share);
	}

	_plafReleaseErrorHandler();

	if (!window->context.glxHandle) {
		return _plafNewError("GLX: Failed to create context");
	}

	window->context.glxWindow = _plaf.glxCreateWindow(_plaf.x11Display, native, window->x11Window, NULL);
	if (!window->context.glxWindow) {
		return _plafNewError("GLX: Failed to create window");
	}

	window->context.glxFBConfig = native;
	window->context.makeCurrent = makeContextCurrentGLX;
	window->context.swapBuffers = swapBuffersGLX;
	window->context.swapInterval = swapIntervalGLX;
	window->context.extensionSupported = extensionSupportedGLX;
	window->context.getProcAddress = getProcAddressGLX;
	window->context.destroy = destroyContextGLX;
	return NULL;
}

#undef SET_ATTRIB

// Returns the Visual and depth of the chosen GLXFBConfig
plafError* _plafChooseVisual(const plafWindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig, Visual** visual, int* depth) {
	GLXFBConfig native;
	if (!chooseGLXFBConfig(fbconfig, &native)) {
		return _plafNewError("GLX: Failed to find a suitable GLXFBConfig");
	}
	XVisualInfo* result = _plaf.glxGetVisualFromFBConfig(_plaf.x11Display, native);
	if (!result) {
		return _plafNewError("GLX: Failed to retrieve Visual for GLXFBConfig");
	}
	*visual = result->visual;
	*depth  = result->depth;
	_plaf.xlibFree(result);
	return NULL;
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
