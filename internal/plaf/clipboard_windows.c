#if defined(_WIN32)

#include "platform.h"

#define MAX_OPEN_CLIPBOARD_TRIES 3

const char* plafGetClipboardString(void) {
	int tries = 0;
	while (!OpenClipboard(_plaf.win32HelperWindowHandle)) {
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

	_plaf_free(_plaf.clipboardString);
	_plaf.clipboardString = _plafCreateUTF8FromWideStringWin32(buffer);

	GlobalUnlock(object);
	CloseClipboard();
	return _plaf.clipboardString;
}

void plafSetClipboardString(const char* string) {
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
	while (!OpenClipboard(_plaf.win32HelperWindowHandle)) {
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
