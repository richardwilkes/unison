#include "platform.h"

#if defined(__APPLE__)

#include <unistd.h>
#include <math.h>

static plafError* makeContextCurrentNSGL(plafWindow* window) {
	@autoreleasepool {
		if (window) {
			[window->context.nsglCtx makeCurrentContext];
		} else {
			[NSOpenGLContext clearCurrentContext];
		}
		_plaf.contextSlot = window;
	}
	return NULL;
}

static void swapBuffersNSGL(plafWindow* window)
{
	@autoreleasepool {

	[window->context.nsglCtx flushBuffer];

	}
}

static void swapIntervalNSGL(int interval)
{
	@autoreleasepool {

		[_plaf.contextSlot->context.nsglCtx setValues:&interval forParameter:NSOpenGLContextParameterSwapInterval];

	}
}

static int extensionSupportedNSGL(const char* extension)
{
	// There are no NSGL extensions
	return false;
}

static glFunc getProcAddressNSGL(const char* procname)
{
	CFStringRef symbolName = CFStringCreateWithCString(kCFAllocatorDefault, procname, kCFStringEncodingASCII);
	glFunc symbol = CFBundleGetFunctionPointerForName(_plaf.nsglFramework, symbolName);
	CFRelease(symbolName);
	return symbol;
}

static void destroyContextNSGL(plafWindow* window)
{
	@autoreleasepool {

	[window->context.nsglPixelFormat release];
	window->context.nsglPixelFormat = nil;

	[window->context.nsglCtx release];
	window->context.nsglCtx = nil;

	}
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Initialize OpenGL support
plafError* _plafInitNSGL(void) {
	if (_plaf.nsglFramework) {
		return NULL;
	}
	_plaf.nsglFramework = CFBundleGetBundleWithIdentifier(CFSTR("com.apple.opengl"));
	if (_plaf.nsglFramework == NULL) {
		return createErrorResponse("NSGL: Failed to locate OpenGL framework");
	}
	return NULL;
}

// Terminate OpenGL support
//
void _plafTerminateNSGL(void)
{
}

// Create the OpenGL context
plafError* _plafCreateContextNSGL(plafWindow* window, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig) {
	if (ctxconfig->major > 2) {
		if (ctxconfig->major == 3 && ctxconfig->minor < 2) {
			return createErrorResponse("NSGL: The targeted version of macOS does not support OpenGL 3.0 or 3.1 but may support 3.2 and above");
		}
	}
	if (ctxconfig->major >= 3 && ctxconfig->profile == OPENGL_PROFILE_COMPAT) {
		return createErrorResponse("NSGL: The compatibility profile is not available on macOS");
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

	if (ctxconfig->major >= 4) {
		SET_ATTRIB(NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion4_1Core);
	} else if (ctxconfig->major >= 3) {
		SET_ATTRIB(NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion3_2Core);
	}

	if (ctxconfig->major <= 2) {
		if (fbconfig->auxBuffers != DONT_CARE) {
			SET_ATTRIB(NSOpenGLPFAAuxBuffers, fbconfig->auxBuffers);
		}
		if (fbconfig->accumRedBits != DONT_CARE && fbconfig->accumGreenBits != DONT_CARE &&
			fbconfig->accumBlueBits != DONT_CARE && fbconfig->accumAlphaBits != DONT_CARE) {
			const int accumBits = fbconfig->accumRedBits + fbconfig->accumGreenBits + fbconfig->accumBlueBits +
				fbconfig->accumAlphaBits;
			SET_ATTRIB(NSOpenGLPFAAccumSize, accumBits);
		}
	}
	if (fbconfig->redBits != DONT_CARE && fbconfig->greenBits != DONT_CARE && fbconfig->blueBits != DONT_CARE) {
		int colorBits = fbconfig->redBits + fbconfig->greenBits + fbconfig->blueBits;
		// macOS needs non-zero color size, so set reasonable values
		if (colorBits == 0) {
			colorBits = 24;
		} else if (colorBits < 15) {
			colorBits = 15;
		}
		SET_ATTRIB(NSOpenGLPFAColorSize, colorBits);
	}

	if (fbconfig->alphaBits != DONT_CARE) {
		SET_ATTRIB(NSOpenGLPFAAlphaSize, fbconfig->alphaBits);
	}

	if (fbconfig->depthBits != DONT_CARE) {
		SET_ATTRIB(NSOpenGLPFADepthSize, fbconfig->depthBits);
	}

	if (fbconfig->stencilBits != DONT_CARE) {
		SET_ATTRIB(NSOpenGLPFAStencilSize, fbconfig->stencilBits);
	}

	if (fbconfig->doublebuffer) {
		ADD_ATTRIB(NSOpenGLPFADoubleBuffer);
	}

	if (fbconfig->samples != DONT_CARE) {
		if (fbconfig->samples == 0) {
			SET_ATTRIB(NSOpenGLPFASampleBuffers, 0);
		} else {
			SET_ATTRIB(NSOpenGLPFASampleBuffers, 1);
			SET_ATTRIB(NSOpenGLPFASamples, fbconfig->samples);
		}
	}

	// NOTE: All NSOpenGLPixelFormats on the relevant cards support sRGB
	//       framebuffer, so there's no need (and no way) to request it
	ADD_ATTRIB(0);

#undef ADD_ATTRIB
#undef SET_ATTRIB

	window->context.nsglPixelFormat = [[NSOpenGLPixelFormat alloc] initWithAttributes:attribs];
	if (!window->context.nsglPixelFormat) {
		return createErrorResponse("NSGL: Failed to find a suitable pixel format");
	}

	NSOpenGLContext* share = nil;
	if (ctxconfig->share) {
		share = ctxconfig->share->context.nsglCtx;
	}

	window->context.nsglCtx = [[NSOpenGLContext alloc]
		initWithFormat:window->context.nsglPixelFormat shareContext:share];
	if (!window->context.nsglCtx) {
		return createErrorResponse("NSGL: Failed to create OpenGL context");
	}

	if (fbconfig->transparent) {
		GLint opaque = 0;
		[window->context.nsglCtx setValues:&opaque forParameter:NSOpenGLContextParameterSurfaceOpacity];
	}

	[window->nsView setWantsBestResolutionOpenGLSurface:window->nsScaleFramebuffer];
	[window->context.nsglCtx setView:window->nsView];

	window->context.makeCurrent = makeContextCurrentNSGL;
	window->context.swapBuffers = swapBuffersNSGL;
	window->context.swapInterval = swapIntervalNSGL;
	window->context.extensionSupported = extensionSupportedNSGL;
	window->context.getProcAddress = getProcAddressNSGL;
	window->context.destroy = destroyContextNSGL;
	return NULL;
}


//////////////////////////////////////////////////////////////////////////
//////                        PLAF native API                       //////
//////////////////////////////////////////////////////////////////////////

id plafGetNSGLContext(plafWindow* window) {
	return window->context.nsglCtx;
}

#endif // __APPLE__
