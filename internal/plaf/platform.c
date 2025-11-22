#include "platform.h"

#include <stdio.h>
#include <stdarg.h>

plafLib _plaf = { false };

// These are outside of _plaf so they can be used before initialization and
// after termination without special handling when _plaf is cleared to zero
//
static errorFunc _plafErrorCallback;

// Terminate the library
void plafTerminate(void) {
	if (_plaf.initialized) {
		int i;

		_plaf.monitorCallback = NULL;
		while (_plaf.windowListHead) {
			plafDestroyWindow(_plaf.windowListHead);
		}
		while (_plaf.cursorListHead) {
			plafDestroyCursor(_plaf.cursorListHead);
		}
		for (i = 0;  i < _plaf.monitorCount;  i++) {
			plafMonitor* monitor = _plaf.monitors[i];
			if (monitor->originalRamp.size) {
				_plafSetGammaRamp(monitor, &monitor->originalRamp);
			}
			_plafFreeMonitor(monitor);
		}
		_plaf_free(_plaf.monitors);
		_plaf.monitors = NULL;
		_plaf.monitorCount = 0;
		_plafTerminate();
		_plaf_free(_plaf.clipboardString);
		_plaf.initialized = false;
		memset(&_plaf, 0, sizeof(_plaf));
	}
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Encode a Unicode code point to a UTF-8 stream
// Based on cutef8 by Jeff Bezanson (Public Domain)
//
size_t _plafEncodeUTF8(char* s, uint32_t codepoint)
{
	size_t count = 0;

	if (codepoint < 0x80)
		s[count++] = (char) codepoint;
	else if (codepoint < 0x800)
	{
		s[count++] = (codepoint >> 6) | 0xc0;
		s[count++] = (codepoint & 0x3f) | 0x80;
	}
	else if (codepoint < 0x10000)
	{
		s[count++] = (codepoint >> 12) | 0xe0;
		s[count++] = ((codepoint >> 6) & 0x3f) | 0x80;
		s[count++] = (codepoint & 0x3f) | 0x80;
	}
	else if (codepoint < 0x110000)
	{
		s[count++] = (codepoint >> 18) | 0xf0;
		s[count++] = ((codepoint >> 12) & 0x3f) | 0x80;
		s[count++] = ((codepoint >> 6) & 0x3f) | 0x80;
		s[count++] = (codepoint & 0x3f) | 0x80;
	}

	return count;
}

// Splits and translates a text/uri-list into separate file paths
// NOTE: This function destroys the provided string
//
char** _plafParseUriList(char* text, int* count)
{
	const char* prefix = "file://";
	char** paths = NULL;
	char* line;

	*count = 0;

	while ((line = strtok(text, "\r\n")))
	{
		char* path;

		text = NULL;

		if (line[0] == '#')
			continue;

		if (strncmp(line, prefix, strlen(prefix)) == 0)
		{
			line += strlen(prefix);
			// TODO: Validate hostname
			while (*line != '/')
				line++;
		}

		(*count)++;

		path = _plaf_calloc(strlen(line) + 1, 1);
		paths = _plaf_realloc(paths, *count * sizeof(char*));
		paths[*count - 1] = path;

		while (*line)
		{
			if (line[0] == '%' && line[1] && line[2])
			{
				const char digits[3] = { line[1], line[2], '\0' };
				*path = (char) strtol(digits, NULL, 16);
				line += 2;
			}
			else
				*path = *line;

			path++;
			line++;
		}
	}

	return paths;
}

char* _plaf_strdup(const char* src)
{
	const size_t length = strlen(src);
	char* result = _plaf_calloc(length + 1, 1);
	strcpy(result, src);
	return result;
}

int _plaf_min(int a, int b)
{
	return a < b ? a : b;
}

void* _plaf_calloc(size_t count, size_t size)
{
	if (count && size)
	{
		void* block;

		if (count > SIZE_MAX / size)
		{
			_plafInputError("Allocation size overflow");
			return NULL;
		}

		block = malloc(count * size);
		if (block)
			return memset(block, 0, count * size);
		else
		{
			_plafInputError("Out of memory");
			return NULL;
		}
	}
	else
		return NULL;
}

void* _plaf_realloc(void* block, size_t size)
{
	if (block && size)
	{
		void* resized = realloc(block, size);
		if (resized)
			return resized;
		else
		{
			_plafInputError("Out of memory");
			return NULL;
		}
	}
	else if (block)
	{
		_plaf_free(block);
		return NULL;
	}
	else
		return _plaf_calloc(1, size);
}

void _plaf_free(void* block)
{
	if (block)
		free(block);
}


//////////////////////////////////////////////////////////////////////////
//////                         PLAF event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code of an error
void _plafInputError(const char* format, ...) {
	char description[ERROR_MSG_SIZE];
	va_list vl;

	va_start(vl, format);
	vsnprintf(description, sizeof(description), format, vl);
	va_end(vl);

	description[sizeof(description) - 1] = '\0';

	strcpy(_plaf.errorSlot.desc, description);

	if (_plafErrorCallback) {
		_plafErrorCallback(description);
	}
}

plafError* _plafNewError(const char* format, ...) {
	va_list args;
	plafError* errResp = (plafError*)malloc(sizeof(plafError));
	errResp->next = NULL;
	va_start(args, format);
	vsnprintf(errResp->desc, ERROR_MSG_SIZE, format, args);
	va_end(args);
	errResp->desc[sizeof(errResp->desc) - 1] = '\0';
	return errResp;
}

//////////////////////////////////////////////////////////////////////////
//////                        PLAF public API                       //////
//////////////////////////////////////////////////////////////////////////

plafError* plafInit(void) {
	if (_plaf.initialized) {
		return NULL;
	}
	memset(&_plaf, 0, sizeof(_plaf));
	plafError* errRsp = _plafInit();
	if (errRsp != NULL) {
		return errRsp;
	}
	plafDefaultWindowHints();
	_plaf.initialized = true;
	return NULL;
}

errorFunc plafSetErrorCallback(errorFunc cbfun)
{
	SWAP(errorFunc, _plafErrorCallback, cbfun);
	return cbfun;
}
