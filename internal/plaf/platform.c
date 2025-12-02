#include "platform.h"

#include <stdio.h>
#include <stdarg.h>

plafLib _plaf;

bool plafInit(void) {
	memset(&_plaf, 0, sizeof(_plaf));
	_plaf.frameBufferCfg.redBits     = 8;
	_plaf.frameBufferCfg.greenBits   = 8;
	_plaf.frameBufferCfg.blueBits    = 8;
	_plaf.frameBufferCfg.alphaBits   = 8;
	_plaf.frameBufferCfg.depthBits   = 24;
	_plaf.frameBufferCfg.stencilBits = 8;
	_plaf.desiredRefreshRate         = DONT_CARE;
	if (!_plafInit()) {
		return false;
	}
	return true;
}

void plafTerminate(void) {
	while (_plaf.windowListHead) {
		plafDestroyWindow(_plaf.windowListHead);
	}
	while (_plaf.cursorListHead) {
		plafDestroyCursor(_plaf.cursorListHead);
	}
	_plafTerminate();
	_plaf_free(_plaf.clipboardString);
	memset(&_plaf, 0, sizeof(_plaf));
}

size_t _plafEncodeUTF8(char* s, uint32_t codepoint) {
	size_t count = 0;
	if (codepoint < 0x80) {
		s[count++] = (char) codepoint;
	} else if (codepoint < 0x800) {
		s[count++] = (codepoint >> 6) | 0xc0;
		s[count++] = (codepoint & 0x3f) | 0x80;
	} else if (codepoint < 0x10000) {
		s[count++] = (codepoint >> 12) | 0xe0;
		s[count++] = ((codepoint >> 6) & 0x3f) | 0x80;
		s[count++] = (codepoint & 0x3f) | 0x80;
	} else if (codepoint < 0x110000) {
		s[count++] = (codepoint >> 18) | 0xf0;
		s[count++] = ((codepoint >> 12) & 0x3f) | 0x80;
		s[count++] = ((codepoint >> 6) & 0x3f) | 0x80;
		s[count++] = (codepoint & 0x3f) | 0x80;
	}
	return count;
}

char* _plaf_strdup(const char* src) {
	const size_t length = strlen(src);
	char* result = _plaf_calloc(length + 1, 1);
	strcpy(result, src);
	return result;
}

int _plaf_min(int a, int b) {
	return a < b ? a : b;
}

void* _plaf_calloc(size_t count, size_t size) {
	if (count && size) {
		void* block;
		if (count > SIZE_MAX / size) {
			return NULL;
		}
		block = malloc(count * size);
		if (block) {
			return memset(block, 0, count * size);
		}
	}
	return NULL;
}

void* _plaf_realloc(void* block, size_t size) {
	if (block && size) {
		void* resized = realloc(block, size);
		if (resized) {
			return resized;
		}
		return NULL;
	}
	if (block) {
		_plaf_free(block);
		return NULL;
	}
	return _plaf_calloc(1, size);
}

void _plaf_free(void* block) {
	if (block) {
		free(block);
	}
}
