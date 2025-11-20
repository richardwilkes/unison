#include "platform.h"

#include <stdio.h>
#include <stdarg.h>


// NOTE: The global variables below comprise all mutable global data in GLFW
//       Any other mutable global variable is a bug

// This contains all mutable state shared between compilation units of GLFW
//
_GLFWlibrary _glfw = { false };

// These are outside of _glfw so they can be used before initialization and
// after termination without special handling when _glfw is cleared to zero
//
static errorFunc _glfwErrorCallback;

// Terminate the library
//
void _terminate(void) {
	int i;

	_glfw.monitorCallback = NULL;
	while (_glfw.windowListHead) {
		glfwDestroyWindow((plafWindow*) _glfw.windowListHead);
	}
	while (_glfw.cursorListHead) {
		glfwDestroyCursor(_glfw.cursorListHead);
	}
	for (i = 0;  i < _glfw.monitorCount;  i++) {
		plafMonitor* monitor = _glfw.monitors[i];
		if (monitor->originalRamp.size) {
			_glfw.platform.setGammaRamp(monitor, &monitor->originalRamp);
		}
		_glfwFreeMonitor(monitor);
	}
	_glfw_free(_glfw.monitors);
	_glfw.monitors = NULL;
	_glfw.monitorCount = 0;
	platformTerminate();
	_glfw_free(_glfw.clipboardString);
	_glfw.initialized = false;
	memset(&_glfw, 0, sizeof(_glfw));
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Encode a Unicode code point to a UTF-8 stream
// Based on cutef8 by Jeff Bezanson (Public Domain)
//
size_t _glfwEncodeUTF8(char* s, uint32_t codepoint)
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
char** _glfwParseUriList(char* text, int* count)
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

        path = _glfw_calloc(strlen(line) + 1, 1);
        paths = _glfw_realloc(paths, *count * sizeof(char*));
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

char* _glfw_strdup(const char* src)
{
    const size_t length = strlen(src);
    char* result = _glfw_calloc(length + 1, 1);
    strcpy(result, src);
    return result;
}

int _glfw_min(int a, int b)
{
    return a < b ? a : b;
}

int _glfw_max(int a, int b)
{
    return a > b ? a : b;
}

void* _glfw_calloc(size_t count, size_t size)
{
    if (count && size)
    {
        void* block;

        if (count > SIZE_MAX / size)
        {
            _glfwInputError(ERR_INVALID_VALUE, "Allocation size overflow");
            return NULL;
        }

        block = malloc(count * size);
        if (block)
            return memset(block, 0, count * size);
        else
        {
            _glfwInputError(ERR_OUT_OF_MEMORY, "Out of memory");
            return NULL;
        }
    }
    else
        return NULL;
}

void* _glfw_realloc(void* block, size_t size)
{
    if (block && size)
    {
        void* resized = realloc(block, size);
        if (resized)
            return resized;
        else
        {
            _glfwInputError(ERR_OUT_OF_MEMORY, "Out of memory");
            return NULL;
        }
    }
    else if (block)
    {
        _glfw_free(block);
        return NULL;
    }
    else
        return _glfw_calloc(1, size);
}

void _glfw_free(void* block)
{
    if (block)
        free(block);
}


//////////////////////////////////////////////////////////////////////////
//////                         GLFW event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code of an error
void _glfwInputError(int code, const char* format, ...) {
	char description[ERROR_MSG_SIZE];
	va_list vl;

	va_start(vl, format);
	vsnprintf(description, sizeof(description), format, vl);
	va_end(vl);

	description[sizeof(description) - 1] = '\0';

	_glfw.errorSlot.code = code;
	strcpy(_glfw.errorSlot.desc, description);

	if (_glfwErrorCallback) {
		_glfwErrorCallback(code, description);
	}
}

ErrorResponse* createErrorResponse(int code, const char* format, ...) {
	va_list args;
	ErrorResponse* errResp = (ErrorResponse*)malloc(sizeof(ErrorResponse));
	errResp->code = code;
	va_start(args, format);
	vsnprintf(errResp->desc, ERROR_MSG_SIZE, format, args);
	va_end(args);
	errResp->desc[sizeof(errResp->desc) - 1] = '\0';
	return errResp;
}

//////////////////////////////////////////////////////////////////////////
//////                        GLFW public API                       //////
//////////////////////////////////////////////////////////////////////////

ErrorResponse* glfwInit(void) {
    if (_glfw.initialized) {
        return NULL;
	}
	memset(&_glfw, 0, sizeof(_glfw));
	ErrorResponse* errRsp = platformInit(&_glfw.platform);
	if (errRsp != NULL) {
		return errRsp;
	}
    glfwDefaultWindowHints();
    _glfw.initialized = true;
    return NULL;
}

void glfwTerminate(void) {
	if (_glfw.initialized) {
		_terminate();
	}
}

errorFunc glfwSetErrorCallback(errorFunc cbfun)
{
    SWAP(errorFunc, _glfwErrorCallback, cbfun);
    return cbfun;
}
