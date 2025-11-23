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

static bool extensionSupportedNSGL(const char* extension) {
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
plafError* _plafInitOpenGL(void) {
	if (_plaf.nsglFramework) {
		return NULL;
	}
	_plaf.nsglFramework = CFBundleGetBundleWithIdentifier(CFSTR("com.apple.opengl"));
	if (_plaf.nsglFramework == NULL) {
		return _plafNewError("NSGL: Failed to locate OpenGL framework");
	}
	return NULL;
}

// Terminate OpenGL support
void _plafTerminateOpenGL(void) {
}

// Create the OpenGL context
plafError* _plafCreateOpenGLContext(plafWindow* window, plafWindow* share, const plafFrameBufferCfg* fbconfig) {
	int colorBits = fbconfig->redBits + fbconfig->greenBits + fbconfig->blueBits;
	if (colorBits == 0) {
		colorBits = 24;
	} else if (colorBits < 15) {
		colorBits = 15;
	}
	NSOpenGLPixelFormatAttribute attribs[] = {
		NSOpenGLPFAAccelerated,
		NSOpenGLPFAClosestPolicy,
		NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion3_2Core,
		NSOpenGLPFAColorSize, colorBits,
		NSOpenGLPFAAlphaSize, fbconfig->alphaBits,
		NSOpenGLPFADepthSize, fbconfig->depthBits,
		NSOpenGLPFAStencilSize, fbconfig->stencilBits,
		NSOpenGLPFASampleBuffers, fbconfig->samples > 0 ? 1 : 0,
		0, 0, 0, // Reserved for the conditional ones, below
		0
	};
	int i = sizeof(attribs) / sizeof(attribs[0]) - 4; // Adjust this constant if more reserved slots are added
	if (fbconfig->samples > 0) {
		attribs[i++] = NSOpenGLPFASamples;
		attribs[i++] = fbconfig->samples;
	}
	if (fbconfig->doublebuffer) {
		attribs[i++] = NSOpenGLPFADoubleBuffer;
	}
	window->context.nsglPixelFormat = [[NSOpenGLPixelFormat alloc] initWithAttributes:attribs];
	if (!window->context.nsglPixelFormat) {
		return _plafNewError("NSGL: Failed to find a suitable pixel format");
	}

	NSOpenGLContext* shareCtx = nil;
	if (share) {
		shareCtx = share->context.nsglCtx;
	}

	window->context.nsglCtx = [[NSOpenGLContext alloc]
		initWithFormat:window->context.nsglPixelFormat shareContext:shareCtx];
	if (!window->context.nsglCtx) {
		return _plafNewError("NSGL: Failed to create OpenGL context");
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
