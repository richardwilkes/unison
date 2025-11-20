#include "platform.h"

#include <stdio.h>
#include <limits.h>


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Checks whether the desired context attributes are valid
//
// This function checks things like whether the specified client API version
// exists and whether all relevant options have supported and non-conflicting
// values
//
IntBool _glfwIsValidContextConfig(const plafCtxCfg* ctxconfig)
{
	if (ctxconfig->profile)
	{
		if (ctxconfig->profile != OPENGL_PROFILE_CORE &&
			ctxconfig->profile != OPENGL_PROFILE_COMPAT)
		{
			_glfwInputError(ERR_INVALID_ENUM, "Invalid OpenGL profile 0x%08X", ctxconfig->profile);
			return false;
		}
	}

    if (ctxconfig->robustness)
    {
        if (ctxconfig->robustness != CONTEXT_ROBUSTNESS_NO_RESET_NOTIFICATION &&
            ctxconfig->robustness != CONTEXT_ROBUSTNESS_LOSE_CONTEXT_ON_RESET)
        {
            _glfwInputError(ERR_INVALID_ENUM, "Invalid context robustness mode 0x%08X", ctxconfig->robustness);
            return false;
        }
    }

    if (ctxconfig->release)
    {
        if (ctxconfig->release != RELEASE_BEHAVIOR_NONE &&
            ctxconfig->release != RELEASE_BEHAVIOR_FLUSH)
        {
            _glfwInputError(ERR_INVALID_ENUM, "Invalid context release behavior 0x%08X", ctxconfig->release);
            return false;
        }
    }

    return true;
}

// Chooses the framebuffer config that best matches the desired one
//
const plafFrameBufferCfg* _glfwChooseFBConfig(const plafFrameBufferCfg* desired,
                                         const plafFrameBufferCfg* alternatives,
                                         unsigned int count)
{
    unsigned int i;
    unsigned int missing, leastMissing = UINT_MAX;
    unsigned int colorDiff, leastColorDiff = UINT_MAX;
    unsigned int extraDiff, leastExtraDiff = UINT_MAX;
    const plafFrameBufferCfg* current;
    const plafFrameBufferCfg* closest = NULL;

    for (i = 0;  i < count;  i++)
    {
        current = alternatives + i;

        // Count number of missing buffers
        {
            missing = 0;

            if (desired->alphaBits > 0 && current->alphaBits == 0)
                missing++;

            if (desired->depthBits > 0 && current->depthBits == 0)
                missing++;

            if (desired->stencilBits > 0 && current->stencilBits == 0)
                missing++;

            if (desired->auxBuffers > 0 &&
                current->auxBuffers < desired->auxBuffers)
            {
                missing += desired->auxBuffers - current->auxBuffers;
            }

            if (desired->samples > 0 && current->samples == 0)
            {
                // Technically, several multisampling buffers could be
                // involved, but that's a lower level implementation detail and
                // not important to us here, so we count them as one
                missing++;
            }

            if (desired->transparent != current->transparent)
                missing++;
        }

        // These polynomials make many small channel size differences matter
        // less than one large channel size difference

        // Calculate color channel size difference value
        {
            colorDiff = 0;

            if (desired->redBits != DONT_CARE)
            {
                colorDiff += (desired->redBits - current->redBits) *
                             (desired->redBits - current->redBits);
            }

            if (desired->greenBits != DONT_CARE)
            {
                colorDiff += (desired->greenBits - current->greenBits) *
                             (desired->greenBits - current->greenBits);
            }

            if (desired->blueBits != DONT_CARE)
            {
                colorDiff += (desired->blueBits - current->blueBits) *
                             (desired->blueBits - current->blueBits);
            }
        }

        // Calculate non-color channel size difference value
        {
            extraDiff = 0;

            if (desired->alphaBits != DONT_CARE)
            {
                extraDiff += (desired->alphaBits - current->alphaBits) *
                             (desired->alphaBits - current->alphaBits);
            }

            if (desired->depthBits != DONT_CARE)
            {
                extraDiff += (desired->depthBits - current->depthBits) *
                             (desired->depthBits - current->depthBits);
            }

            if (desired->stencilBits != DONT_CARE)
            {
                extraDiff += (desired->stencilBits - current->stencilBits) *
                             (desired->stencilBits - current->stencilBits);
            }

            if (desired->accumRedBits != DONT_CARE)
            {
                extraDiff += (desired->accumRedBits - current->accumRedBits) *
                             (desired->accumRedBits - current->accumRedBits);
            }

            if (desired->accumGreenBits != DONT_CARE)
            {
                extraDiff += (desired->accumGreenBits - current->accumGreenBits) *
                             (desired->accumGreenBits - current->accumGreenBits);
            }

            if (desired->accumBlueBits != DONT_CARE)
            {
                extraDiff += (desired->accumBlueBits - current->accumBlueBits) *
                             (desired->accumBlueBits - current->accumBlueBits);
            }

            if (desired->accumAlphaBits != DONT_CARE)
            {
                extraDiff += (desired->accumAlphaBits - current->accumAlphaBits) *
                             (desired->accumAlphaBits - current->accumAlphaBits);
            }

            if (desired->samples != DONT_CARE)
            {
                extraDiff += (desired->samples - current->samples) *
                             (desired->samples - current->samples);
            }

            if (desired->sRGB && !current->sRGB)
                extraDiff++;
        }

        // Figure out if the current one is better than the best one found so far
        // Least number of missing buffers is the most important heuristic,
        // then color buffer size match and lastly size match for other buffers

        if (missing < leastMissing)
            closest = current;
        else if (missing == leastMissing)
        {
            if ((colorDiff < leastColorDiff) ||
                (colorDiff == leastColorDiff && extraDiff < leastExtraDiff))
            {
                closest = current;
            }
        }

        if (current == closest)
        {
            leastMissing = missing;
            leastColorDiff = colorDiff;
            leastExtraDiff = extraDiff;
        }
    }

    return closest;
}

// Retrieves the attributes of the current context
//
IntBool _glfwRefreshContextAttribs(plafWindow* window,
                                    const plafCtxCfg* ctxconfig)
{
    int i;
    plafWindow* previous = _glfw.contextSlot;
    const char* version;

    glfwMakeContextCurrent((plafWindow*) window);
    if (_glfw.contextSlot != window)
        return false;

    window->context.GetIntegerv = (FN_GLGETINTEGERV)
        window->context.getProcAddress("glGetIntegerv");
    window->context.GetString = (FN_GLGETSTRING)
        window->context.getProcAddress("glGetString");
    if (!window->context.GetIntegerv || !window->context.GetString)
    {
        _glfwInputError(ERR_PLATFORM_ERROR, "Entry point retrieval is broken");
        glfwMakeContextCurrent((plafWindow*) previous);
        return false;
    }

    version = (const char*) window->context.GetString(GL_VERSION);
    if (!version)
    {
		_glfwInputError(ERR_PLATFORM_ERROR, "OpenGL version string retrieval is broken");
        glfwMakeContextCurrent((plafWindow*) previous);
        return false;
    }

    if (!sscanf(version, "%d.%d.%d",
                &window->context.major,
                &window->context.minor,
                &window->context.revision))
    {
		_glfwInputError(ERR_PLATFORM_ERROR, "No version found in OpenGL version string");
        glfwMakeContextCurrent((plafWindow*) previous);
        return false;
    }

    if (window->context.major < ctxconfig->major ||
        (window->context.major == ctxconfig->major &&
         window->context.minor < ctxconfig->minor))
    {
        // The desired OpenGL version is greater than the actual version
        // This only happens if the machine lacks {GLX|WGL}_ARB_create_context
        // /and/ the user has requested an OpenGL version greater than 1.0

        // For API consistency, we emulate the behavior of the
        // {GLX|WGL}_ARB_create_context extension and fail here

		_glfwInputError(ERR_VERSION_UNAVAILABLE, "Requested OpenGL version %i.%i, got version %i.%i", ctxconfig->major, ctxconfig->minor, window->context.major, window->context.minor);
        glfwMakeContextCurrent((plafWindow*) previous);
        return false;
    }

    if (window->context.major >= 3)
    {
        // OpenGL 3.0+ uses a different function for extension string retrieval
        // We cache it here instead of in glfwExtensionSupported mostly to alert
        // users as early as possible that their build may be broken

        window->context.GetStringi = (FN_GLGETSTRINGI)
            window->context.getProcAddress("glGetStringi");
        if (!window->context.GetStringi)
        {
            _glfwInputError(ERR_PLATFORM_ERROR, "Entry point retrieval is broken");
            glfwMakeContextCurrent((plafWindow*) previous);
            return false;
        }
    }

	// Read back context flags (OpenGL 3.0 and above)
	if (window->context.major >= 3)
	{
		GLint flags;
		window->context.GetIntegerv(GL_CONTEXT_FLAGS, &flags);

		if (flags & GL_CONTEXT_FLAG_FORWARD_COMPATIBLE_BIT)
			window->context.forward = true;

		if (flags & GL_CONTEXT_FLAG_DEBUG_BIT)
			window->context.debug = true;

		if (flags & GL_CONTEXT_FLAG_NO_ERROR_BIT_KHR)
			window->context.noerror = true;
	}

	// Read back OpenGL context profile (OpenGL 3.2 and above)
	if (window->context.major >= 4 ||
		(window->context.major == 3 && window->context.minor >= 2))
	{
		GLint mask;
		window->context.GetIntegerv(GL_CONTEXT_PROFILE_MASK, &mask);

		if (mask & GL_CONTEXT_COMPATIBILITY_PROFILE_BIT)
			window->context.profile = OPENGL_PROFILE_COMPAT;
		else if (mask & GL_CONTEXT_CORE_PROFILE_BIT)
			window->context.profile = OPENGL_PROFILE_CORE;
	}

	// Read back robustness strategy
	if (glfwExtensionSupported("GL_ARB_robustness"))
	{
		// NOTE: We avoid using the context flags for detection, as they are
		//       only present from 3.0 while the extension applies from 1.1

		GLint strategy;
		window->context.GetIntegerv(GL_RESET_NOTIFICATION_STRATEGY_ARB,
									&strategy);

		if (strategy == GL_LOSE_CONTEXT_ON_RESET_ARB)
			window->context.robustness = CONTEXT_ROBUSTNESS_LOSE_CONTEXT_ON_RESET;
		else if (strategy == GL_NO_RESET_NOTIFICATION_ARB)
			window->context.robustness = CONTEXT_ROBUSTNESS_NO_RESET_NOTIFICATION;
	}

    if (glfwExtensionSupported("GL_KHR_context_flush_control"))
    {
        GLint behavior;
        window->context.GetIntegerv(GL_CONTEXT_RELEASE_BEHAVIOR, &behavior);

        if (behavior == GL_NONE)
            window->context.release = RELEASE_BEHAVIOR_NONE;
        else if (behavior == GL_CONTEXT_RELEASE_BEHAVIOR_FLUSH)
            window->context.release = RELEASE_BEHAVIOR_FLUSH;
    }

    // Clearing the front buffer to black to avoid garbage pixels left over from
    // previous uses of our bit of VRAM
    {
        FN_GLCLEAR glClear = (FN_GLCLEAR)
            window->context.getProcAddress("glClear");
        glClear(GL_COLOR_BUFFER_BIT);

        if (window->doublebuffer)
            window->context.swapBuffers(window);
    }

    glfwMakeContextCurrent((plafWindow*) previous);
    return true;
}

// Searches an extension string for the specified extension
//
IntBool _glfwStringInExtensionString(const char* string, const char* extensions)
{
    const char* start = extensions;

    for (;;)
    {
        const char* where;
        const char* terminator;

        where = strstr(start, string);
        if (!where)
            return false;

        terminator = where + strlen(string);
        if (where == start || *(where - 1) == ' ')
        {
            if (*terminator == ' ' || *terminator == '\0')
                break;
        }

        start = terminator;
    }

    return true;
}


//////////////////////////////////////////////////////////////////////////
//////                        GLFW public API                       //////
//////////////////////////////////////////////////////////////////////////

void glfwMakeContextCurrent(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    plafWindow* previous = _glfw.contextSlot;

    if (previous)
    {
        if (!window)
            previous->context.makeCurrent(NULL);
    }

    if (window)
        window->context.makeCurrent(window);
}

plafWindow* glfwGetCurrentContext(void)
{
    return (plafWindow*)_glfw.contextSlot;
}

void glfwSwapBuffers(plafWindow* handle)
{
    plafWindow* window = (plafWindow*) handle;
    window->context.swapBuffers(window);
}

void glfwSwapInterval(int interval)
{
    if (!_glfw.contextSlot)
    {
        _glfwInputError(ERR_NO_CURRENT_CONTEXT, "Cannot set swap interval without a current OpenGL or OpenGL ES context");
        return;
    }
    _glfw.contextSlot->context.swapInterval(interval);
}

int glfwExtensionSupported(const char* extension)
{
    if (!_glfw.contextSlot)
    {
        _glfwInputError(ERR_NO_CURRENT_CONTEXT, "Cannot query extension without a current OpenGL or OpenGL ES context");
        return false;
    }

    if (*extension == '\0')
    {
        _glfwInputError(ERR_INVALID_VALUE, "Extension name cannot be an empty string");
        return false;
    }

    if (_glfw.contextSlot->context.major >= 3)
    {
        int i;
        GLint count;

        // Check if extension is in the modern OpenGL extensions string list

        _glfw.contextSlot->context.GetIntegerv(GL_NUM_EXTENSIONS, &count);

        for (i = 0;  i < count;  i++)
        {
            const char* en = (const char*)_glfw.contextSlot->context.GetStringi(GL_EXTENSIONS, i);
            if (!en)
            {
                _glfwInputError(ERR_PLATFORM_ERROR, "Extension string retrieval is broken");
                return false;
            }

            if (strcmp(en, extension) == 0)
                return true;
        }
    }
    else
    {
        // Check if extension is in the old style OpenGL extensions string

        const char* extensions = (const char*)_glfw.contextSlot->context.GetString(GL_EXTENSIONS);
        if (!extensions)
        {
            _glfwInputError(ERR_PLATFORM_ERROR, "Extension string retrieval is broken");
            return false;
        }

        if (_glfwStringInExtensionString(extension, extensions))
            return true;
    }

    // Check if extension is in the platform-specific string
    return _glfw.contextSlot->context.extensionSupported(extension);
}

glFunc glfwGetProcAddress(const char* procname)
{
    if (!_glfw.contextSlot)
    {
        _glfwInputError(ERR_NO_CURRENT_CONTEXT, "Cannot query entry point without a current OpenGL or OpenGL ES context");
        return NULL;
    }
    return _glfw.contextSlot->context.getProcAddress(procname);
}
