#if defined(_GLFW_WIN32)

#include "platform.h"

#define MAX_OPEN_CLIPBOARD_TRIES 3

const char* getClipboardString(void) {
	HANDLE object;
	WCHAR* buffer;
	int tries = 0;

	while (!OpenClipboard(_glfw.win32.helperWindowHandle)) {
		Sleep(1);
		if (++tries == MAX_OPEN_CLIPBOARD_TRIES) {
			return NULL;
		}
	}

	object = GetClipboardData(CF_UNICODETEXT);
	if (!object) {
		CloseClipboard();
		return NULL;
	}

	buffer = GlobalLock(object);
	if (!buffer) {
		CloseClipboard();
		return NULL;
	}

	_glfw_free(_glfw.clipboardString);
	_glfw.clipboardString = _glfwCreateUTF8FromWideStringWin32(buffer);

	GlobalUnlock(object);
	CloseClipboard();
	return _glfw.clipboardString;
}

void setClipboardString(const char* string) {
	int characterCount, tries = 0;
	HANDLE object;
	WCHAR* buffer;

	characterCount = MultiByteToWideChar(CP_UTF8, 0, string, -1, NULL, 0);
	if (!characterCount) {
		return;
	}

	object = GlobalAlloc(GMEM_MOVEABLE, characterCount * sizeof(WCHAR));
	if (!object) {
		return;
	}

	buffer = GlobalLock(object);
	if (!buffer) {
		GlobalFree(object);
		return;
	}

	MultiByteToWideChar(CP_UTF8, 0, string, -1, buffer, characterCount);
	GlobalUnlock(object);

	while (!OpenClipboard(_glfw.win32.helperWindowHandle)) {
		Sleep(1);
		if (++tries == MAX_OPEN_CLIPBOARD_TRIES) {
			GlobalFree(object);
			return;
		}
	}

	EmptyClipboard();
	SetClipboardData(CF_UNICODETEXT, object);
	CloseClipboard();
}

#endif // _GLFW_WIN32
