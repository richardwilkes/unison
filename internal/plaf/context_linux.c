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
    _glfw.glxGetFBConfigAttrib(_glfw.x11Display, fbconfig, attrib, &value);
    return value;
}

// Return the GLXFBConfig most closely matching the specified hints
//
static IntBool chooseGLXFBConfig(const plafFrameBufferCfg* desired,
                                  GLXFBConfig* result)
{
    GLXFBConfig* nativeConfigs;
    plafFrameBufferCfg* usableConfigs;
    const plafFrameBufferCfg* closest;
    int nativeCount, usableCount;
    const char* vendor;
    IntBool trustWindowBit = true;

    // HACK: This is a (hopefully temporary) workaround for Chromium
    //       (VirtualBox GL) not setting the window bit on any GLXFBConfigs
    vendor = _glfw.glxGetClientString(_glfw.x11Display, GLX_VENDOR);
    if (vendor && strcmp(vendor, "Chromium") == 0)
        trustWindowBit = false;

    nativeConfigs =
        _glfw.glxGetFBConfigs(_glfw.x11Display, _glfw.x11Screen, &nativeCount);
    if (!nativeConfigs || !nativeCount)
    {
        _glfwInputError(ERR_API_UNAVAILABLE, "GLX: No GLXFBConfigs returned");
        return false;
    }

    usableConfigs = _glfw_calloc(nativeCount, sizeof(plafFrameBufferCfg));
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
            XVisualInfo* vi = _glfw.glxGetVisualFromFBConfig(_glfw.x11Display, n);
            if (vi)
            {
                u->transparent = _glfwIsVisualTransparentX11(vi->visual);
                _glfw.xlibFree(vi);
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

        if (_glfw.glxARB_multisample)
            u->samples = getGLXFBConfigAttrib(n, GLX_SAMPLES);

        if (_glfw.glxARB_framebuffer_sRGB || _glfw.glxEXT_framebuffer_sRGB)
            u->sRGB = getGLXFBConfigAttrib(n, GLX_FRAMEBUFFER_SRGB_CAPABLE_ARB);

        u->handle = (uintptr_t) n;
        usableCount++;
    }

    closest = _glfwChooseFBConfig(desired, usableConfigs, usableCount);
    if (closest)
        *result = (GLXFBConfig) closest->handle;

    _glfw.xlibFree(nativeConfigs);
    _glfw_free(usableConfigs);

    return closest != NULL;
}

// Create the OpenGL context using legacy API
//
static GLXContext createLegacyContextGLX(plafWindow* window,
                                         GLXFBConfig fbconfig,
                                         GLXContext share)
{
    return _glfw.glxCreateNewContext(_glfw.x11Display,
                               fbconfig,
                               GLX_RGBA_TYPE,
                               share,
                               True);
}

static void makeContextCurrentGLX(plafWindow* window)
{
    if (window)
    {
        if (!_glfw.glxMakeCurrent(_glfw.x11Display,
                            window->context.glxWindow,
                            window->context.glxHandle))
        {
            _glfwInputError(ERR_PLATFORM_ERROR, "GLX: Failed to make context current");
            return;
        }
    }
    else
    {
        if (!_glfw.glxMakeCurrent(_glfw.x11Display, None, NULL))
        {
            _glfwInputError(ERR_PLATFORM_ERROR, "GLX: Failed to clear current context");
            return;
        }
    }
    _glfw.contextSlot = window;
}

static void swapBuffersGLX(plafWindow* window)
{
    _glfw.glxSwapBuffers(_glfw.x11Display, window->context.glxWindow);
}

static void swapIntervalGLX(int interval)
{
    if (_glfw.glxEXT_swap_control)
    {
        _glfw.glxSwapIntervalEXT(_glfw.x11Display,
                                  _glfw.contextSlot->context.glxWindow,
                                  interval);
    }
    else if (_glfw.glxSGI_swap_control)
    {
        if (interval > 0)
            _glfw.glxSwapIntervalSGI(interval);
    }
}

static int extensionSupportedGLX(const char* extension)
{
    const char* extensions =
        _glfw.glxQueryExtensionsString(_glfw.x11Display, _glfw.x11Screen);
    if (extensions)
    {
        if (_glfwStringInExtensionString(extension, extensions))
            return true;
    }

    return false;
}

static glFunc getProcAddressGLX(const char* procname)
{
    if (_glfw.glxGetProcAddress)
        return _glfw.glxGetProcAddress((const GLubyte*) procname);
    else if (_glfw.glxGetProcAddressARB)
        return _glfw.glxGetProcAddressARB((const GLubyte*) procname);
    else
    {
        // NOTE: glvnd provides GLX 1.4, so this can only happen with libGL
        return _glfwPlatformGetModuleSymbol(_glfw.glxHandle, procname);
    }
}

static void destroyContextGLX(plafWindow* window)
{
    if (window->context.glxWindow)
    {
        _glfw.glxDestroyWindow(_glfw.x11Display, window->context.glxWindow);
        window->context.glxWindow = None;
    }

    if (window->context.glxHandle)
    {
        _glfw.glxDestroyContext(_glfw.x11Display, window->context.glxHandle);
        window->context.glxHandle = NULL;
    }
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Initialize GLX
//
IntBool _glfwInitGLX(void)
{
    const char* sonames[] =
    {
        "libGLX.so.0",
        "libGL.so.1",
        "libGL.so",
        NULL
    };

    if (_glfw.glxHandle)
        return true;

    for (int i = 0;  sonames[i];  i++)
    {
        _glfw.glxHandle = _glfwPlatformLoadModule(sonames[i]);
        if (_glfw.glxHandle)
            break;
    }

    if (!_glfw.glxHandle)
    {
        _glfwInputError(ERR_API_UNAVAILABLE, "GLX: Failed to load GLX");
        return false;
    }

    _glfw.glxGetFBConfigs = (FN_GLXGETFBCONFIGS)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXGetFBConfigs");
    _glfw.glxGetFBConfigAttrib = (FN_GLXGETFBCONFIGATTRIB)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXGetFBConfigAttrib");
    _glfw.glxGetClientString = (FN_GLXGETCLIENTSTRING)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXGetClientString");
    _glfw.glxQueryExtension = (FN_GLXQUERYEXTENSION)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXQueryExtension");
    _glfw.glxQueryVersion = (FN_GLXQUERYVERSION)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXQueryVersion");
    _glfw.glxDestroyContext = (FN_GLXDESTROYCONTEXT)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXDestroyContext");
    _glfw.glxMakeCurrent = (FN_GLXMAKECURRENT)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXMakeCurrent");
    _glfw.glxSwapBuffers = (FN_GLXSWAPBUFFERS)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXSwapBuffers");
    _glfw.glxQueryExtensionsString = (FN_GLXQUERYEXTENSIONSSTRING)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXQueryExtensionsString");
    _glfw.glxCreateNewContext = (FN_GLXCREATENEWCONTEXT)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXCreateNewContext");
    _glfw.glxCreateWindow = (FN_GLXCREATEWINDOW)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXCreateWindow");
    _glfw.glxDestroyWindow = (FN_GLXDESTROYWINDOW)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXDestroyWindow");
    _glfw.glxGetVisualFromFBConfig = (FN_GLXGETVISUALFROMFBCONFIG)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXGetVisualFromFBConfig");

    if (!_glfw.glxGetFBConfigs ||
        !_glfw.glxGetFBConfigAttrib ||
        !_glfw.glxGetClientString ||
        !_glfw.glxQueryExtension ||
        !_glfw.glxQueryVersion ||
        !_glfw.glxDestroyContext ||
        !_glfw.glxMakeCurrent ||
        !_glfw.glxSwapBuffers ||
        !_glfw.glxQueryExtensionsString ||
        !_glfw.glxCreateNewContext ||
        !_glfw.glxCreateWindow ||
        !_glfw.glxDestroyWindow ||
        !_glfw.glxGetVisualFromFBConfig)
    {
        _glfwInputError(ERR_PLATFORM_ERROR, "GLX: Failed to load required entry points");
        return false;
    }

    // NOTE: Unlike GLX 1.3 entry points these are not required to be present
    _glfw.glxGetProcAddress = (FN_GLXGETPROCADDRESS)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXGetProcAddress");
    _glfw.glxGetProcAddressARB = (FN_GLXGETPROCADDRESS)
        _glfwPlatformGetModuleSymbol(_glfw.glxHandle, "glXGetProcAddressARB");

	int errorBase;
	int eventBase;
    if (!_glfw.glxQueryExtension(_glfw.x11Display, &_glfw.glxErrorBase, &eventBase)) {
        _glfwInputError(ERR_API_UNAVAILABLE, "GLX: GLX extension not found");
        return false;
    }

	int major;
	int minor;
    if (!_glfw.glxQueryVersion(_glfw.x11Display, &major, &minor)) {
        _glfwInputError(ERR_API_UNAVAILABLE, "GLX: Failed to query GLX version");
        return false;
    }
    if (major == 1 && minor < 3) {
        _glfwInputError(ERR_API_UNAVAILABLE, "GLX: GLX version 1.3 is required");
        return false;
    }

    if (extensionSupportedGLX("GLX_EXT_swap_control"))
    {
        _glfw.glxSwapIntervalEXT = (FN_GLXSWAPINTERVALEXT)
            getProcAddressGLX("glXSwapIntervalEXT");

        if (_glfw.glxSwapIntervalEXT)
            _glfw.glxEXT_swap_control = true;
    }

    if (extensionSupportedGLX("GLX_SGI_swap_control"))
    {
        _glfw.glxSwapIntervalSGI = (FN_GLXSWAPINTERVALSGI)
            getProcAddressGLX("glXSwapIntervalSGI");

        if (_glfw.glxSwapIntervalSGI)
            _glfw.glxSGI_swap_control = true;
    }

    if (extensionSupportedGLX("GLX_ARB_multisample"))
        _glfw.glxARB_multisample = true;

    if (extensionSupportedGLX("GLX_ARB_framebuffer_sRGB"))
        _glfw.glxARB_framebuffer_sRGB = true;

    if (extensionSupportedGLX("GLX_EXT_framebuffer_sRGB"))
        _glfw.glxEXT_framebuffer_sRGB = true;

    if (extensionSupportedGLX("GLX_ARB_create_context"))
    {
        _glfw.glxCreateContextAttribsARB = (FN_GLXCREATECONTEXTATTRIBSARB)
            getProcAddressGLX("glXCreateContextAttribsARB");

        if (_glfw.glxCreateContextAttribsARB)
            _glfw.glxARB_create_context = true;
    }

    if (extensionSupportedGLX("GLX_ARB_create_context_robustness"))
        _glfw.glxARB_create_context_robustness = true;

    if (extensionSupportedGLX("GLX_ARB_create_context_profile"))
        _glfw.glxARB_create_context_profile = true;

    if (extensionSupportedGLX("GLX_ARB_create_context_no_error"))
        _glfw.glxARB_create_context_no_error = true;

    if (extensionSupportedGLX("GLX_ARB_context_flush_control"))
        _glfw.glxARB_context_flush_control = true;

    return true;
}

// Terminate GLX
//
void _glfwTerminateGLX(void)
{
    // NOTE: This function must not call any X11 functions, as it is called after XCloseDisplay

    if (_glfw.glxHandle)
    {
        _glfwPlatformFreeModule(_glfw.glxHandle);
        _glfw.glxHandle = NULL;
    }
}

#define SET_ATTRIB(a, v) \
{ \
    attribs[index++] = a; \
    attribs[index++] = v; \
}

// Create the OpenGL or OpenGL ES context
//
IntBool _glfwCreateContextGLX(plafWindow* window,
                               const plafCtxCfg* ctxconfig,
                               const plafFrameBufferCfg* fbconfig)
{
    int attribs[40];
    GLXFBConfig native = NULL;
    GLXContext share = NULL;

    if (ctxconfig->share)
        share = ctxconfig->share->context.glxHandle;

    if (!chooseGLXFBConfig(fbconfig, &native))
    {
        _glfwInputError(ERR_FORMAT_UNAVAILABLE, "GLX: Failed to find a suitable GLXFBConfig");
        return false;
    }

    if (ctxconfig->forward)
    {
        if (!_glfw.glxARB_create_context)
        {
            _glfwInputError(ERR_VERSION_UNAVAILABLE, "GLX: Forward compatibility requested but GLX_ARB_create_context_profile is unavailable");
            return false;
        }
    }

    if (ctxconfig->profile)
    {
        if (!_glfw.glxARB_create_context ||
            !_glfw.glxARB_create_context_profile)
        {
            _glfwInputError(ERR_VERSION_UNAVAILABLE, "GLX: An OpenGL profile requested but GLX_ARB_create_context_profile is unavailable");
            return false;
        }
    }

    _glfwGrabErrorHandlerX11();

    if (_glfw.glxARB_create_context)
    {
        int index = 0, mask = 0, flags = 0;

		if (ctxconfig->forward)
			flags |= GLX_CONTEXT_FORWARD_COMPATIBLE_BIT_ARB;

		if (ctxconfig->profile == OPENGL_PROFILE_CORE)
			mask |= GLX_CONTEXT_CORE_PROFILE_BIT_ARB;
		else if (ctxconfig->profile == OPENGL_PROFILE_COMPAT)
			mask |= GLX_CONTEXT_COMPATIBILITY_PROFILE_BIT_ARB;

        if (ctxconfig->debug)
            flags |= GLX_CONTEXT_DEBUG_BIT_ARB;

        if (ctxconfig->robustness)
        {
            if (_glfw.glxARB_create_context_robustness)
            {
                if (ctxconfig->robustness == CONTEXT_ROBUSTNESS_NO_RESET_NOTIFICATION)
                {
                    SET_ATTRIB(GLX_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB,
                               GLX_NO_RESET_NOTIFICATION_ARB);
                }
                else if (ctxconfig->robustness == CONTEXT_ROBUSTNESS_LOSE_CONTEXT_ON_RESET)
                {
                    SET_ATTRIB(GLX_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB,
                               GLX_LOSE_CONTEXT_ON_RESET_ARB);
                }

                flags |= GLX_CONTEXT_ROBUST_ACCESS_BIT_ARB;
            }
        }

        if (ctxconfig->release)
        {
            if (_glfw.glxARB_context_flush_control)
            {
                if (ctxconfig->release == RELEASE_BEHAVIOR_NONE)
                {
                    SET_ATTRIB(GLX_CONTEXT_RELEASE_BEHAVIOR_ARB,
                               GLX_CONTEXT_RELEASE_BEHAVIOR_NONE_ARB);
                }
                else if (ctxconfig->release == RELEASE_BEHAVIOR_FLUSH)
                {
                    SET_ATTRIB(GLX_CONTEXT_RELEASE_BEHAVIOR_ARB,
                               GLX_CONTEXT_RELEASE_BEHAVIOR_FLUSH_ARB);
                }
            }
        }

        if (ctxconfig->noerror)
        {
            if (_glfw.glxARB_create_context_no_error)
                SET_ATTRIB(GLX_CONTEXT_OPENGL_NO_ERROR_ARB, true);
        }

        // NOTE: Only request an explicitly versioned context when necessary, as
        //       explicitly requesting version 1.0 does not always return the
        //       highest version supported by the driver
        if (ctxconfig->major != 1 || ctxconfig->minor != 0)
        {
            SET_ATTRIB(GLX_CONTEXT_MAJOR_VERSION_ARB, ctxconfig->major);
            SET_ATTRIB(GLX_CONTEXT_MINOR_VERSION_ARB, ctxconfig->minor);
        }

        if (mask)
            SET_ATTRIB(GLX_CONTEXT_PROFILE_MASK_ARB, mask);

        if (flags)
            SET_ATTRIB(GLX_CONTEXT_FLAGS_ARB, flags);

        SET_ATTRIB(None, None);

        window->context.glxHandle =
            _glfw.glxCreateContextAttribsARB(_glfw.x11Display,
                                              native,
                                              share,
                                              True,
                                              attribs);

        // HACK: This is a fallback for broken versions of the Mesa
        //       implementation of GLX_ARB_create_context_profile that fail
        //       default 1.0 context creation with a GLXBadProfileARB error in
        //       violation of the extension spec
        if (!window->context.glxHandle)
        {
            if (_glfw.x11ErrorCode == _glfw.glxErrorBase + GLXBadProfileARB &&
                ctxconfig->profile == OPENGL_PROFILE_ANY &&
                ctxconfig->forward == false)
            {
                window->context.glxHandle =
                    createLegacyContextGLX(window, native, share);
            }
        }
    }
    else
    {
        window->context.glxHandle =
            createLegacyContextGLX(window, native, share);
    }

    _glfwReleaseErrorHandlerX11();

    if (!window->context.glxHandle)
    {
        _glfwInputErrorX11(ERR_VERSION_UNAVAILABLE, "GLX: Failed to create context");
        return false;
    }

    window->context.glxWindow = _glfw.glxCreateWindow(_glfw.x11Display, native, window->x11Window, NULL);
    if (!window->context.glxWindow)
    {
        _glfwInputError(ERR_PLATFORM_ERROR, "GLX: Failed to create window");
        return false;
    }

    window->context.glxFBConfig = native;

    window->context.makeCurrent = makeContextCurrentGLX;
    window->context.swapBuffers = swapBuffersGLX;
    window->context.swapInterval = swapIntervalGLX;
    window->context.extensionSupported = extensionSupportedGLX;
    window->context.getProcAddress = getProcAddressGLX;
    window->context.destroy = destroyContextGLX;

    return true;
}

#undef SET_ATTRIB

// Returns the Visual and depth of the chosen GLXFBConfig
//
IntBool _glfwChooseVisualGLX(const WindowConfig* wndconfig,
                              const plafCtxCfg* ctxconfig,
                              const plafFrameBufferCfg* fbconfig,
                              Visual** visual, int* depth)
{
    GLXFBConfig native;
    XVisualInfo* result;

    if (!chooseGLXFBConfig(fbconfig, &native))
    {
        _glfwInputError(ERR_FORMAT_UNAVAILABLE, "GLX: Failed to find a suitable GLXFBConfig");
        return false;
    }

    result = _glfw.glxGetVisualFromFBConfig(_glfw.x11Display, native);
    if (!result)
    {
        _glfwInputError(ERR_PLATFORM_ERROR, "GLX: Failed to retrieve Visual for GLXFBConfig");
        return false;
    }

    *visual = result->visual;
    *depth  = result->depth;

    _glfw.xlibFree(result);
    return true;
}


//////////////////////////////////////////////////////////////////////////
//////                        GLFW native API                       //////
//////////////////////////////////////////////////////////////////////////

GLXContext glfwGetGLXContext(plafWindow* window) {
    return window->context.glxHandle;
}

GLXWindow glfwGetGLXWindow(plafWindow* window) {
    return window->context.glxWindow;
}

int glfwGetGLXFBConfig(plafWindow* window, GLXFBConfig* config) {
    *config = window->context.glxFBConfig;
    return true;
}

#endif // __linux__
