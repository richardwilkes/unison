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

    _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Unknown pixel format attribute requested");
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

    if (_glfw.wglARB_pixel_format)
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

        if (_glfw.wglARB_multisample)
            ADD_ATTRIB(WGL_SAMPLES_ARB);

        if (_glfw.wglARB_framebuffer_sRGB || _glfw.wglEXT_framebuffer_sRGB)
            ADD_ATTRIB(WGL_FRAMEBUFFER_SRGB_CAPABLE_ARB);

        // NOTE: In a Parallels VM WGL_ARB_pixel_format returns fewer pixel formats than
        //       DescribePixelFormat, violating the guarantees of the extension spec
        // HACK: Iterate through the minimum of both counts

        const int attrib = WGL_NUMBER_PIXEL_FORMATS_ARB;
        int extensionCount;

        if (!_glfw.wglGetPixelFormatAttribivARB(window->context.wglDC, 1, 0, 1, &attrib, &extensionCount))
        {
            _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to retrieve pixel format attribute");
            return 0;
        }

        nativeCount = _glfw_min(nativeCount, extensionCount);
    }

    usableConfigs = _glfw_calloc(nativeCount, sizeof(plafFrameBufferCfg));

    for (i = 0;  i < nativeCount;  i++)
    {
        plafFrameBufferCfg* u = usableConfigs + usableCount;
        pixelFormat = i + 1;

        if (_glfw.wglARB_pixel_format)
        {
            // Get pixel format attributes through "modern" extension

            if (!_glfw.wglGetPixelFormatAttribivARB(window->context.wglDC, pixelFormat, 0, attribCount, attribs, values))
            {
                _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to retrieve pixel format attributes");

                _glfw_free(usableConfigs);
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

            if (_glfw.wglARB_multisample)
                u->samples = FIND_ATTRIB_VALUE(WGL_SAMPLES_ARB);

			if (_glfw.wglARB_framebuffer_sRGB ||
				_glfw.wglEXT_framebuffer_sRGB)
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
                _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to describe pixel format");

                _glfw_free(usableConfigs);
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
        _glfwInputError(ERR_API_UNAVAILABLE, "WGL: The driver does not appear to support OpenGL");

        _glfw_free(usableConfigs);
        return 0;
    }

    closest = _glfwChooseFBConfig(fbconfig, usableConfigs, usableCount);
    if (!closest)
    {
        _glfwInputError(ERR_FORMAT_UNAVAILABLE, "WGL: Failed to find a suitable pixel format");

        _glfw_free(usableConfigs);
        return 0;
    }

    pixelFormat = (int) closest->handle;
    _glfw_free(usableConfigs);

    return pixelFormat;
}

#undef ADD_ATTRIB
#undef FIND_ATTRIB_VALUE

static void makeContextCurrentWGL(plafWindow* window)
{
    if (window)
    {
        if (_glfw.wglMakeCurrent(window->context.wglDC, window->context.wglGLRC))
			_glfw.contextSlot = window;
        else
        {
            _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to make context current");
			_glfw.contextSlot = NULL;
        }
    }
    else
    {
        if (!_glfw.wglMakeCurrent(NULL, NULL))
        {
            _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to clear current context");
        }
		_glfw.contextSlot = NULL;
    }
}

static void swapBuffersWGL(plafWindow* window)
{
    SwapBuffers(window->context.wglDC);
}

static void swapIntervalWGL(int interval)
{
    _glfw.contextSlot->context.wglInterval = interval;
    if (_glfw.wglEXT_swap_control)
        _glfw.wglSwapIntervalEXT(interval);
}

static int extensionSupportedWGL(const char* extension)
{
    const char* extensions = NULL;

    if (_glfw.wglGetExtensionsStringARB)
        extensions = _glfw.wglGetExtensionsStringARB(_glfw.wglGetCurrentDC());
    else if (_glfw.wglGetExtensionsStringEXT)
        extensions = _glfw.wglGetExtensionsStringEXT();

    if (!extensions)
        return false;

    return _glfwStringInExtensionString(extension, extensions);
}

static glFunc getProcAddressWGL(const char* procname)
{
    const glFunc proc = (glFunc) _glfw.wglGetProcAddress(procname);
    if (proc)
        return proc;

    return (glFunc) _glfwPlatformGetModuleSymbol(_glfw.wglInstance, procname);
}

static void destroyContextWGL(plafWindow* window)
{
    if (window->context.wglGLRC)
    {
        _glfw.wglDeleteContext(window->context.wglGLRC);
        window->context.wglGLRC = NULL;
    }
}

// Initialize WGL
//
IntBool _glfwInitWGL(void)
{
    PIXELFORMATDESCRIPTOR pfd;
    HGLRC prc, rc;
    HDC pdc, dc;

    if (_glfw.wglInstance)
        return true;

    _glfw.wglInstance = _glfwPlatformLoadModule("opengl32.dll");
    if (!_glfw.wglInstance)
    {
        _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to load opengl32.dll");
        return false;
    }

    _glfw.wglCreateContext = (FN_wglCreateContext)
        _glfwPlatformGetModuleSymbol(_glfw.wglInstance, "wglCreateContext");
    _glfw.wglDeleteContext = (FN_wglDeleteContext)
        _glfwPlatformGetModuleSymbol(_glfw.wglInstance, "wglDeleteContext");
    _glfw.wglGetProcAddress = (FN_wglGetProcAddress)
        _glfwPlatformGetModuleSymbol(_glfw.wglInstance, "wglGetProcAddress");
    _glfw.wglGetCurrentDC = (FN_wglGetCurrentDC)
        _glfwPlatformGetModuleSymbol(_glfw.wglInstance, "wglGetCurrentDC");
    _glfw.wglGetCurrentContext = (FN_wglGetCurrentContext)
        _glfwPlatformGetModuleSymbol(_glfw.wglInstance, "wglGetCurrentContext");
    _glfw.wglMakeCurrent = (FN_wglMakeCurrent)
        _glfwPlatformGetModuleSymbol(_glfw.wglInstance, "wglMakeCurrent");
    _glfw.wglShareLists = (FN_wglShareLists)
        _glfwPlatformGetModuleSymbol(_glfw.wglInstance, "wglShareLists");

    // NOTE: A dummy context has to be created for opengl32.dll to load the
    //       OpenGL ICD, from which we can then query WGL extensions
    // NOTE: This code will accept the Microsoft GDI ICD; accelerated context
    //       creation failure occurs during manual pixel format enumeration

    dc = GetDC(_glfw.win32HelperWindowHandle);

    ZeroMemory(&pfd, sizeof(pfd));
    pfd.nSize = sizeof(pfd);
    pfd.nVersion = 1;
    pfd.dwFlags = PFD_DRAW_TO_WINDOW | PFD_SUPPORT_OPENGL | PFD_DOUBLEBUFFER;
    pfd.iPixelType = PFD_TYPE_RGBA;
    pfd.cColorBits = 24;

    if (!SetPixelFormat(dc, ChoosePixelFormat(dc, &pfd), &pfd))
    {
        _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to set pixel format for dummy context");
        return false;
    }

    rc = _glfw.wglCreateContext(dc);
    if (!rc)
    {
        _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to create dummy context");
        return false;
    }

    pdc = _glfw.wglGetCurrentDC();
    prc = _glfw.wglGetCurrentContext();

    if (!_glfw.wglMakeCurrent(dc, rc))
    {
        _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to make dummy context current");
        _glfw.wglMakeCurrent(pdc, prc);
        _glfw.wglDeleteContext(rc);
        return false;
    }

    // NOTE: Functions must be loaded first as they're needed to retrieve the
    //       extension string that tells us whether the functions are supported
    _glfw.wglGetExtensionsStringEXT = (FN_WGLGETEXTENSIONSSTRINGEXT)
        _glfw.wglGetProcAddress("wglGetExtensionsStringEXT");
    _glfw.wglGetExtensionsStringARB = (FN_WGLGETEXTENSIONSSTRINGARB)
        _glfw.wglGetProcAddress("wglGetExtensionsStringARB");
    _glfw.wglCreateContextAttribsARB = (FN_WGLCREATECONTEXTATTRIBSARB)
        _glfw.wglGetProcAddress("wglCreateContextAttribsARB");
    _glfw.wglSwapIntervalEXT = (FN_WGLSWAPINTERVALEXT)
        _glfw.wglGetProcAddress("wglSwapIntervalEXT");
    _glfw.wglGetPixelFormatAttribivARB = (FN_WGLGETPIXELFORMATATTRIBIVARB)
        _glfw.wglGetProcAddress("wglGetPixelFormatAttribivARB");

    // NOTE: WGL_ARB_extensions_string and WGL_EXT_extensions_string are not
    //       checked below as we are already using them
    _glfw.wglARB_multisample =
        extensionSupportedWGL("WGL_ARB_multisample");
    _glfw.wglARB_framebuffer_sRGB =
        extensionSupportedWGL("WGL_ARB_framebuffer_sRGB");
    _glfw.wglEXT_framebuffer_sRGB =
        extensionSupportedWGL("WGL_EXT_framebuffer_sRGB");
    _glfw.wglARB_create_context =
        extensionSupportedWGL("WGL_ARB_create_context");
    _glfw.wglARB_create_context_profile =
        extensionSupportedWGL("WGL_ARB_create_context_profile");
    _glfw.wglARB_create_context_robustness =
        extensionSupportedWGL("WGL_ARB_create_context_robustness");
    _glfw.wglARB_create_context_no_error =
        extensionSupportedWGL("WGL_ARB_create_context_no_error");
    _glfw.wglEXT_swap_control =
        extensionSupportedWGL("WGL_EXT_swap_control");
    _glfw.wglARB_pixel_format =
        extensionSupportedWGL("WGL_ARB_pixel_format");
    _glfw.wglARB_context_flush_control =
        extensionSupportedWGL("WGL_ARB_context_flush_control");

    _glfw.wglMakeCurrent(pdc, prc);
    _glfw.wglDeleteContext(rc);
    return true;
}

// Terminate WGL
//
void _glfwTerminateWGL(void)
{
    if (_glfw.wglInstance)
        _glfwPlatformFreeModule(_glfw.wglInstance);
}

#define SET_ATTRIB(a, v) \
{ \
    attribs[index++] = a; \
    attribs[index++] = v; \
}

// Create the OpenGL or OpenGL ES context
//
IntBool _glfwCreateContextWGL(plafWindow* window,
                               const plafCtxCfg* ctxconfig,
                               const plafFrameBufferCfg* fbconfig)
{
    int attribs[40];
    int pixelFormat;
    PIXELFORMATDESCRIPTOR pfd;
    HGLRC share = NULL;

    if (ctxconfig->share)
        share = ctxconfig->share->context.wglGLRC;

    window->context.wglDC = GetDC(window->win32Window);
    if (!window->context.wglDC)
    {
        _glfwInputError(ERR_PLATFORM_ERROR, "WGL: Failed to retrieve DC for window");
        return false;
    }

    pixelFormat = choosePixelFormatWGL(window, ctxconfig, fbconfig);
    if (!pixelFormat)
        return false;

    if (!DescribePixelFormat(window->context.wglDC,
                             pixelFormat, sizeof(pfd), &pfd))
    {
        _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to retrieve PFD for selected pixel format");
        return false;
    }

    if (!SetPixelFormat(window->context.wglDC, pixelFormat, &pfd))
    {
        _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to set selected pixel format");
        return false;
    }

	if (ctxconfig->forward)
	{
		if (!_glfw.wglARB_create_context)
		{
			_glfwInputError(ERR_VERSION_UNAVAILABLE, "WGL: A forward compatible OpenGL context requested but WGL_ARB_create_context is unavailable");
			return false;
		}
	}

	if (ctxconfig->profile)
	{
		if (!_glfw.wglARB_create_context_profile)
		{
			_glfwInputError(ERR_VERSION_UNAVAILABLE, "WGL: OpenGL profile requested but WGL_ARB_create_context_profile is unavailable");
			return false;
		}
	}

    if (_glfw.wglARB_create_context)
    {
        int index = 0, mask = 0, flags = 0;

		if (ctxconfig->forward)
			flags |= WGL_CONTEXT_FORWARD_COMPATIBLE_BIT_ARB;

		if (ctxconfig->profile == OPENGL_PROFILE_CORE)
			mask |= WGL_CONTEXT_CORE_PROFILE_BIT_ARB;
		else if (ctxconfig->profile == OPENGL_PROFILE_COMPAT)
			mask |= WGL_CONTEXT_COMPATIBILITY_PROFILE_BIT_ARB;

        if (ctxconfig->debug)
            flags |= WGL_CONTEXT_DEBUG_BIT_ARB;

        if (ctxconfig->robustness)
        {
            if (_glfw.wglARB_create_context_robustness)
            {
                if (ctxconfig->robustness == CONTEXT_ROBUSTNESS_NO_RESET_NOTIFICATION)
                {
                    SET_ATTRIB(WGL_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB,
                               WGL_NO_RESET_NOTIFICATION_ARB);
                }
                else if (ctxconfig->robustness == CONTEXT_ROBUSTNESS_LOSE_CONTEXT_ON_RESET)
                {
                    SET_ATTRIB(WGL_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB,
                               WGL_LOSE_CONTEXT_ON_RESET_ARB);
                }

                flags |= WGL_CONTEXT_ROBUST_ACCESS_BIT_ARB;
            }
        }

        if (ctxconfig->release)
        {
            if (_glfw.wglARB_context_flush_control)
            {
                if (ctxconfig->release == RELEASE_BEHAVIOR_NONE)
                {
                    SET_ATTRIB(WGL_CONTEXT_RELEASE_BEHAVIOR_ARB,
                               WGL_CONTEXT_RELEASE_BEHAVIOR_NONE_ARB);
                }
                else if (ctxconfig->release == RELEASE_BEHAVIOR_FLUSH)
                {
                    SET_ATTRIB(WGL_CONTEXT_RELEASE_BEHAVIOR_ARB,
                               WGL_CONTEXT_RELEASE_BEHAVIOR_FLUSH_ARB);
                }
            }
        }

        if (ctxconfig->noerror)
        {
            if (_glfw.wglARB_create_context_no_error)
                SET_ATTRIB(WGL_CONTEXT_OPENGL_NO_ERROR_ARB, true);
        }

        // NOTE: Only request an explicitly versioned context when necessary, as
        //       explicitly requesting version 1.0 does not always return the
        //       highest version supported by the driver
        if (ctxconfig->major != 1 || ctxconfig->minor != 0)
        {
            SET_ATTRIB(WGL_CONTEXT_MAJOR_VERSION_ARB, ctxconfig->major);
            SET_ATTRIB(WGL_CONTEXT_MINOR_VERSION_ARB, ctxconfig->minor);
        }

        if (flags)
            SET_ATTRIB(WGL_CONTEXT_FLAGS_ARB, flags);

        if (mask)
            SET_ATTRIB(WGL_CONTEXT_PROFILE_MASK_ARB, mask);

        SET_ATTRIB(0, 0);

        window->context.wglGLRC =
            _glfw.wglCreateContextAttribsARB(window->context.wglDC, share, attribs);
        if (!window->context.wglGLRC)
        {
            const DWORD error = GetLastError();

            if (error == (0xc0070000 | ERROR_INVALID_VERSION_ARB))
            {
				_glfwInputError(ERR_VERSION_UNAVAILABLE, "WGL: Driver does not support OpenGL version %i.%i", ctxconfig->major, ctxconfig->minor);
            }
            else if (error == (0xc0070000 | ERROR_INVALID_PROFILE_ARB))
            {
                _glfwInputError(ERR_VERSION_UNAVAILABLE, "WGL: Driver does not support the requested OpenGL profile");
            }
            else if (error == (0xc0070000 | ERROR_INCOMPATIBLE_DEVICE_CONTEXTS_ARB))
            {
                _glfwInputError(ERR_INVALID_VALUE, "WGL: The share context is not compatible with the requested context");
            }
            else
            {
				_glfwInputError(ERR_VERSION_UNAVAILABLE, "WGL: Failed to create OpenGL context");
            }

            return false;
        }
    }
    else
    {
        window->context.wglGLRC = _glfw.wglCreateContext(window->context.wglDC);
        if (!window->context.wglGLRC)
        {
            _glfwInputErrorWin32(ERR_VERSION_UNAVAILABLE, "WGL: Failed to create OpenGL context");
            return false;
        }

        if (share)
        {
            if (!_glfw.wglShareLists(share, window->context.wglGLRC))
            {
                _glfwInputErrorWin32(ERR_PLATFORM_ERROR, "WGL: Failed to enable sharing with specified OpenGL context");
                return false;
            }
        }
    }

    window->context.makeCurrent = makeContextCurrentWGL;
    window->context.swapBuffers = swapBuffersWGL;
    window->context.swapInterval = swapIntervalWGL;
    window->context.extensionSupported = extensionSupportedWGL;
    window->context.getProcAddress = getProcAddressWGL;
    window->context.destroy = destroyContextWGL;

    return true;
}

#undef SET_ATTRIB

HGLRC glfwGetWGLContext(plafWindow* window) {
	return window->context.wglGLRC;
}

#endif // _WIN32
