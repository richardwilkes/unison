#include "platform.h"

#if defined(PLAF_DARWIN)

#include <unistd.h>
#include <math.h>

static void makeContextCurrentNSGL(_GLFWwindow* window)
{
    @autoreleasepool {

    if (window)
        [window->context.nsgl.object makeCurrentContext];
    else
        [NSOpenGLContext clearCurrentContext];

	_glfw.contextSlot = window;

    } // autoreleasepool
}

static void swapBuffersNSGL(_GLFWwindow* window)
{
    @autoreleasepool {

    [window->context.nsgl.object flushBuffer];

    } // autoreleasepool
}

static void swapIntervalNSGL(int interval)
{
    @autoreleasepool {

    	[_glfw.contextSlot->context.nsgl.object setValues:&interval forParameter:NSOpenGLContextParameterSwapInterval];

    } // autoreleasepool
}

static int extensionSupportedNSGL(const char* extension)
{
    // There are no NSGL extensions
    return false;
}

static GLFWglproc getProcAddressNSGL(const char* procname)
{
    CFStringRef symbolName = CFStringCreateWithCString(kCFAllocatorDefault,
                                                       procname,
                                                       kCFStringEncodingASCII);

    GLFWglproc symbol = CFBundleGetFunctionPointerForName(_glfw.nsgl.framework,
                                                          symbolName);

    CFRelease(symbolName);

    return symbol;
}

static void destroyContextNSGL(_GLFWwindow* window)
{
    @autoreleasepool {

    [window->context.nsgl.pixelFormat release];
    window->context.nsgl.pixelFormat = nil;

    [window->context.nsgl.object release];
    window->context.nsgl.object = nil;

    } // autoreleasepool
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Initialize OpenGL support
//
GLFWbool _glfwInitNSGL(void) {
    if (_glfw.nsgl.framework) {
        return true;
	}
    _glfw.nsgl.framework = CFBundleGetBundleWithIdentifier(CFSTR("com.apple.opengl"));
    if (_glfw.nsgl.framework == NULL) {
        _glfwInputError(GLFW_API_UNAVAILABLE, "NSGL: Failed to locate OpenGL framework");
        return false;
    }
    return true;
}

// Terminate OpenGL support
//
void _glfwTerminateNSGL(void)
{
}

// Create the OpenGL context
//
GLFWbool _glfwCreateContextNSGL(_GLFWwindow* window,
                                const _GLFWctxconfig* ctxconfig,
                                const _GLFWfbconfig* fbconfig)
{
    if (ctxconfig->major > 2)
    {
        if (ctxconfig->major == 3 && ctxconfig->minor < 2)
        {
            _glfwInputError(GLFW_VERSION_UNAVAILABLE, "NSGL: The targeted version of macOS does not support OpenGL 3.0 or 3.1 but may support 3.2 and above");
            return false;
        }
    }

    if (ctxconfig->major >= 3 && ctxconfig->profile == GLFW_OPENGL_COMPAT_PROFILE)
    {
        _glfwInputError(GLFW_VERSION_UNAVAILABLE, "NSGL: The compatibility profile is not available on macOS");
        return false;
    }

    // Context robustness modes (GL_KHR_robustness) are not supported by
    // macOS but are not a hard constraint, so ignore and continue

    // Context release behaviors (GL_KHR_context_flush_control) are not
    // supported by macOS but are not a hard constraint, so ignore and continue

    // Debug contexts (GL_KHR_debug) are not supported by macOS but are not
    // a hard constraint, so ignore and continue

    // No-error contexts (GL_KHR_no_error) are not supported by macOS but
    // are not a hard constraint, so ignore and continue

#define ADD_ATTRIB(a) \
{ \
    attribs[index++] = a; \
}
#define SET_ATTRIB(a, v) { ADD_ATTRIB(a); ADD_ATTRIB(v); }

    NSOpenGLPixelFormatAttribute attribs[40];
    int index = 0;

    ADD_ATTRIB(NSOpenGLPFAAccelerated);
    ADD_ATTRIB(NSOpenGLPFAClosestPolicy);

    if (ctxconfig->nsgl.offline)
    {
        ADD_ATTRIB(NSOpenGLPFAAllowOfflineRenderers);
        // NOTE: This replaces the NSSupportsAutomaticGraphicsSwitching key in
        //       Info.plist for unbundled applications
        // HACK: This assumes that NSOpenGLPixelFormat will remain
        //       a straightforward wrapper of its CGL counterpart
        ADD_ATTRIB(kCGLPFASupportsAutomaticGraphicsSwitching);
    }

    if (ctxconfig->major >= 4)
    {
        SET_ATTRIB(NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion4_1Core);
    }
    else if (ctxconfig->major >= 3)
    {
        SET_ATTRIB(NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion3_2Core);
    }

    if (ctxconfig->major <= 2)
    {
        if (fbconfig->auxBuffers != GLFW_DONT_CARE)
            SET_ATTRIB(NSOpenGLPFAAuxBuffers, fbconfig->auxBuffers);

        if (fbconfig->accumRedBits != GLFW_DONT_CARE &&
            fbconfig->accumGreenBits != GLFW_DONT_CARE &&
            fbconfig->accumBlueBits != GLFW_DONT_CARE &&
            fbconfig->accumAlphaBits != GLFW_DONT_CARE)
        {
            const int accumBits = fbconfig->accumRedBits +
                                  fbconfig->accumGreenBits +
                                  fbconfig->accumBlueBits +
                                  fbconfig->accumAlphaBits;

            SET_ATTRIB(NSOpenGLPFAAccumSize, accumBits);
        }
    }

    if (fbconfig->redBits != GLFW_DONT_CARE &&
        fbconfig->greenBits != GLFW_DONT_CARE &&
        fbconfig->blueBits != GLFW_DONT_CARE)
    {
        int colorBits = fbconfig->redBits +
                        fbconfig->greenBits +
                        fbconfig->blueBits;

        // macOS needs non-zero color size, so set reasonable values
        if (colorBits == 0)
            colorBits = 24;
        else if (colorBits < 15)
            colorBits = 15;

        SET_ATTRIB(NSOpenGLPFAColorSize, colorBits);
    }

    if (fbconfig->alphaBits != GLFW_DONT_CARE)
        SET_ATTRIB(NSOpenGLPFAAlphaSize, fbconfig->alphaBits);

    if (fbconfig->depthBits != GLFW_DONT_CARE)
        SET_ATTRIB(NSOpenGLPFADepthSize, fbconfig->depthBits);

    if (fbconfig->stencilBits != GLFW_DONT_CARE)
        SET_ATTRIB(NSOpenGLPFAStencilSize, fbconfig->stencilBits);

    if (fbconfig->doublebuffer)
        ADD_ATTRIB(NSOpenGLPFADoubleBuffer);

    if (fbconfig->samples != GLFW_DONT_CARE)
    {
        if (fbconfig->samples == 0)
        {
            SET_ATTRIB(NSOpenGLPFASampleBuffers, 0);
        }
        else
        {
            SET_ATTRIB(NSOpenGLPFASampleBuffers, 1);
            SET_ATTRIB(NSOpenGLPFASamples, fbconfig->samples);
        }
    }

    // NOTE: All NSOpenGLPixelFormats on the relevant cards support sRGB
    //       framebuffer, so there's no need (and no way) to request it

    ADD_ATTRIB(0);

#undef ADD_ATTRIB
#undef SET_ATTRIB

    window->context.nsgl.pixelFormat =
        [[NSOpenGLPixelFormat alloc] initWithAttributes:attribs];
    if (window->context.nsgl.pixelFormat == nil)
    {
        _glfwInputError(GLFW_FORMAT_UNAVAILABLE, "NSGL: Failed to find a suitable pixel format");
        return false;
    }

    NSOpenGLContext* share = nil;

    if (ctxconfig->share)
        share = ctxconfig->share->context.nsgl.object;

    window->context.nsgl.object =
        [[NSOpenGLContext alloc] initWithFormat:window->context.nsgl.pixelFormat
                                   shareContext:share];
    if (window->context.nsgl.object == nil)
    {
        _glfwInputError(GLFW_VERSION_UNAVAILABLE, "NSGL: Failed to create OpenGL context");
        return false;
    }

    if (fbconfig->transparent)
    {
        GLint opaque = 0;
        [window->context.nsgl.object setValues:&opaque
                                  forParameter:NSOpenGLContextParameterSurfaceOpacity];
    }

    [window->ns.view setWantsBestResolutionOpenGLSurface:window->ns.scaleFramebuffer];

    [window->context.nsgl.object setView:window->ns.view];

    window->context.makeCurrent = makeContextCurrentNSGL;
    window->context.swapBuffers = swapBuffersNSGL;
    window->context.swapInterval = swapIntervalNSGL;
    window->context.extensionSupported = extensionSupportedNSGL;
    window->context.getProcAddress = getProcAddressNSGL;
    window->context.destroy = destroyContextNSGL;

    return true;
}


//////////////////////////////////////////////////////////////////////////
//////                        GLFW native API                       //////
//////////////////////////////////////////////////////////////////////////

id glfwGetNSGLContext(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    return window->context.nsgl.object;
}

#endif // PLAF_DARWIN
