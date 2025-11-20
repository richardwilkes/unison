#if defined(_WIN32)

#include "platform.h"

#define MAX_OPEN_CLIPBOARD_TRIES 3

const char* getClipboardString(void) {
	int tries = 0;
	while (!OpenClipboard(_glfw.win32HelperWindowHandle)) {
		Sleep(1);
		if (++tries == MAX_OPEN_CLIPBOARD_TRIES) {
			return NULL;
		}
	}

	HANDLE object = GetClipboardData(CF_UNICODETEXT);
	if (!object) {
		CloseClipboard();
		return NULL;
	}

	WCHAR* buffer = GlobalLock(object);
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
	int characterCount = MultiByteToWideChar(CP_UTF8, 0, string, -1, NULL, 0);
	if (!characterCount) {
		return;
	}

	HANDLE object = GlobalAlloc(GMEM_MOVEABLE, characterCount * sizeof(WCHAR));
	if (!object) {
		return;
	}

	WCHAR* buffer = GlobalLock(object);
	if (!buffer) {
		GlobalFree(object);
		return;
	}

	MultiByteToWideChar(CP_UTF8, 0, string, -1, buffer, characterCount);
	GlobalUnlock(object);

	int tries = 0;
	while (!OpenClipboard(_glfw.win32HelperWindowHandle)) {
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

#endif // _WIN32
