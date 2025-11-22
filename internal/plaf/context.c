#include "platform.h"

#include <stdio.h>
#include <limits.h>


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Chooses the framebuffer config that best matches the desired one
//
const plafFrameBufferCfg* _plafChooseFBConfig(const plafFrameBufferCfg* desired,
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
plafError* _plafRefreshContextAttribs(plafWindow* window) {
	plafWindow* previous = _plaf.contextSlot;
	plafError* err = plafMakeContextCurrent(window);
	if (err) {
		return err;
	}

	window->context.GetIntegerv = (FN_GLGETINTEGERV)window->context.getProcAddress("glGetIntegerv");
	window->context.GetString = (FN_GLGETSTRING)window->context.getProcAddress("glGetString");
	if (!window->context.GetIntegerv || !window->context.GetString) {
		plafMakeContextCurrent(previous);
		return _plafNewError("Entry point retrieval is broken");
	}

	const char* version = (const char*) window->context.GetString(GL_VERSION);
	if (!version) {
		plafMakeContextCurrent(previous);
		return _plafNewError("OpenGL version string retrieval is broken");
	}

	int major = 0;
	int minor = 0;
	if (!sscanf(version, "%d.%d", &major, &minor)) {
		plafMakeContextCurrent(previous);
		return _plafNewError("No version found in OpenGL version string");
	}
	if (major < 3 || (major == 3 && minor < 2)) {
		plafMakeContextCurrent(previous);
		return _plafNewError("Requested OpenGL version 3.2, got version %i.%i", major, minor);
	}

	window->context.GetStringi = (FN_GLGETSTRINGI)window->context.getProcAddress("glGetStringi");
	if (!window->context.GetStringi) {
		plafMakeContextCurrent(previous);
		return _plafNewError("Entry point retrieval is broken");
	}

	FN_GLCLEAR glClear = (FN_GLCLEAR)window->context.getProcAddress("glClear");
	glClear(GL_COLOR_BUFFER_BIT);
	if (window->doublebuffer) {
		window->context.swapBuffers(window);
	}
	return plafMakeContextCurrent(previous);
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

plafError* plafMakeContextCurrent(plafWindow* window) {
	plafError* err = NULL;
	if (_plaf.contextSlot) {
		if (!window) {
			err = _plaf.contextSlot->context.makeCurrent(NULL);
		}
	}
	if (window) {
		plafError* err2 = window->context.makeCurrent(window);
		if (err2) {
			err2->next = err;
			err = err2;
		}
	}
	return err;
}

plafWindow* plafGetCurrentContext(void)
{
	return _plaf.contextSlot;
}

void plafSwapBuffers(plafWindow* window) {
	window->context.swapBuffers(window);
}

void plafSwapInterval(int interval)
{
	if (!_plaf.contextSlot)
	{
		_plafInputError("Cannot set swap interval without a current OpenGL or OpenGL ES context");
		return;
	}
	_plaf.contextSlot->context.swapInterval(interval);
}

bool plafExtensionSupported(const char* extension) {
	if (!_plaf.contextSlot) {
		_plafInputError("Cannot query extension without a current OpenGL or OpenGL ES context");
		return false;
	}

	if (*extension == '\0') {
		_plafInputError("Extension name cannot be an empty string");
		return false;
	}

	// Check if extension is in the OpenGL extensions string list
	GLint count;
	_plaf.contextSlot->context.GetIntegerv(GL_NUM_EXTENSIONS, &count);
	for (int i = 0;  i < count;  i++) {
		const char* en = (const char*)_plaf.contextSlot->context.GetStringi(GL_EXTENSIONS, i);
		if (!en) {
			_plafInputError("Extension string retrieval is broken");
			return false;
		}
		if (strcmp(en, extension) == 0) {
			return true;
		}
	}

	// Check if extension is in the platform-specific string
	return _plaf.contextSlot->context.extensionSupported(extension);
}

glFunc plafGetProcAddress(const char* procname)
{
	if (!_plaf.contextSlot)
	{
		_plafInputError("Cannot query entry point without a current OpenGL or OpenGL ES context");
		return NULL;
	}
	return _plaf.contextSlot->context.getProcAddress(procname);
}
