#include "platform.h"

#include <stdio.h>
#include <limits.h>


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Chooses the framebuffer config that best matches the desired one
const plafFrameBufferCfg* _plafChooseFBConfig(const plafFrameBufferCfg* desired, const plafFrameBufferCfg* alternatives, unsigned int count) {
	int leastMissing = INT_MAX;
	int leastColorDiff = INT_MAX;
	int leastExtraDiff = INT_MAX;
	const plafFrameBufferCfg* closest = NULL;

	for (unsigned int i = 0;  i < count;  i++) {
		const plafFrameBufferCfg* current = alternatives + i;

		// Count number of missing buffers
		int missing = 0;
		if (current->alphaBits == 0) {
			missing++;
		}
		if (current->depthBits == 0) {
			missing++;
		}
		if (current->stencilBits == 0) {
			missing++;
		}
		if (desired->transparent != current->transparent) {
			missing++;
		}

		// Calculate color channel size difference value
		int colorDiff = (desired->redBits - current->redBits) * (desired->redBits - current->redBits) +
						(desired->greenBits - current->greenBits) * (desired->greenBits - current->greenBits) +
						(desired->blueBits - current->blueBits) * (desired->blueBits - current->blueBits);

		// Calculate non-color channel size difference value
		int extraDiff = (desired->alphaBits - current->alphaBits) * (desired->alphaBits - current->alphaBits) +
			(desired->depthBits - current->depthBits) * (desired->depthBits - current->depthBits) +
			(desired->stencilBits - current->stencilBits) * (desired->stencilBits - current->stencilBits) +
			(desired->accumRedBits - current->accumRedBits) * (desired->accumRedBits - current->accumRedBits) +
			(desired->accumGreenBits - current->accumGreenBits) * (desired->accumGreenBits - current->accumGreenBits) +
			(desired->accumBlueBits - current->accumBlueBits) * (desired->accumBlueBits - current->accumBlueBits) +
			(desired->accumAlphaBits - current->accumAlphaBits) * (desired->accumAlphaBits - current->accumAlphaBits) +
			(desired->samples - current->samples) * (desired->samples - current->samples);
		if (desired->sRGB && !current->sRGB) {
			extraDiff++;
		}
		if (missing < leastMissing) {
			closest = current;
		} else if (missing == leastMissing) {
			if (colorDiff < leastColorDiff || (colorDiff == leastColorDiff && extraDiff < leastExtraDiff)) {
				closest = current;
			}
		}
		if (current == closest) {
			leastMissing = missing;
			leastColorDiff = colorDiff;
			leastExtraDiff = extraDiff;
		}
	}
	return closest;
}

// Retrieves the attributes of the current context
bool _plafRefreshContextAttribs(plafWindow* window) {
	plafWindow* previous = _plaf.wndWithCurrentCtx;
	plafMakeContextCurrent(window);

	const char* (APIENTRY * getString)(unsigned int) = (const char* (APIENTRY *)(unsigned int))window->context.getProcAddress("glGetString");
	if (!getString) {
		plafMakeContextCurrent(previous);
		return false;
	}
	const char* version = getString(GL_VERSION);
	if (!version) {
		plafMakeContextCurrent(previous);
		return false;
	}
	int major = 0;
	int minor = 0;
	if (!sscanf(version, "%d.%d", &major, &minor)) {
		plafMakeContextCurrent(previous);
		return false;
	}
	if (major < 3 || (major == 3 && minor < 2)) {
		plafMakeContextCurrent(previous);
		return false;
	}

	FN_GLCLEAR glClear = (FN_GLCLEAR)window->context.getProcAddress("glClear");
	glClear(GL_COLOR_BUFFER_BIT);
	plafSwapBuffers(window);
	plafMakeContextCurrent(previous);
	return true;
}

// Searches an extension string for the specified extension
//
bool _plafStringInExtensionString(const char* string, const char* extensions) {
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
//////                        PLAF public API                       //////
//////////////////////////////////////////////////////////////////////////

void plafMakeContextCurrent(plafWindow* window) {
	if (window) {
		window->context.makeCurrent(window);
	} else if (_plaf.wndWithCurrentCtx) {
		_plaf.wndWithCurrentCtx->context.makeCurrent(NULL);
	}
}

void plafSwapBuffers(plafWindow* window) {
	window->context.swapBuffers(window);
}
