#include "platform.h"

#include <math.h>


//////////////////////////////////////////////////////////////////////////
//////                         PLAF event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code of a physical key event
void _plafInputKey(plafWindow* window, int key, int code, int action, int mods) {
	if (key >= 0 && key <= KEY_LAST) {
		bool repeated = false;
		if (action == INPUT_RELEASE && window->keys[key] == INPUT_RELEASE) {
			return;
		}
		if (action == INPUT_PRESS && window->keys[key] == INPUT_PRESS) {
			repeated = true;
		}
		window->keys[key] = (char) action;
		if (repeated) {
			action = INPUT_REPEAT;
		}
	}
	goKeyCallback(window, key, code, action, mods);
}

// Notifies shared code of a Unicode codepoint input event
void _plafInputChar(plafWindow* window, uint32_t ch) {
	if (ch < 32 || (ch > 126 && ch < 160)) {
		return;
	}
	goCharCallback(window, ch);
}

// Notifies shared code of a mouse button click event
void _plafInputMouseClick(plafWindow* window, int button, int action, int mods) {
	if (button < 0 || button > MOUSE_BUTTON_LAST) {
		return;
	}
	window->mouseButtons[button] = (char) action;
	goMouseButtonCallback(window, button, action, mods);
}

// Notifies shared code of a cursor motion event
// The position is specified in content area relative screen coordinates
void _plafInputCursorPos(plafWindow* window, double x, double y) {
	if (window->virtualCursorPosX == x && window->virtualCursorPosY == y) {
		return;
	}
	window->virtualCursorPosX = x;
	window->virtualCursorPosY = y;
	goCursorPosCallback(window, x, y);
}

//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////


//////////////////////////////////////////////////////////////////////////
//////                        PLAF public API                       //////
//////////////////////////////////////////////////////////////////////////

void plafHideCursor(plafWindow* window) {
	if (!window->cursorHidden) {
		window->cursorHidden = true;
		plafGetCursorPos(window, &window->virtualCursorPosX, &window->virtualCursorPosY);
		_plafUpdateCursor(window);
	}
}

void plafShowCursor(plafWindow* window) {
	if (window->cursorHidden) {
		window->cursorHidden = false;
		_plafUpdateCursor(window);
	}
}

int plafGetKeyScancode(int key) {
	if (key < KEY_SPACE || key > KEY_LAST) {
		return KEY_UNKNOWN;
	}
	return _plaf.scanCodes[key];
}

int plafGetKey(plafWindow* window, int key) {
	if (key < KEY_SPACE || key > KEY_LAST) {
		return INPUT_RELEASE;
	}
	return (int)window->keys[key];
}

int plafGetMouseButton(plafWindow* window, int button) {
	if (button < MOUSE_BUTTON_1 || button > MOUSE_BUTTON_LAST) {
		return INPUT_RELEASE;
	}
	return (int)window->mouseButtons[button];
}

plafCursor* plafCreateCursor(const plafImageData* image, int xhot, int yhot) {
	plafCursor* cursor = _plaf_calloc(1, sizeof(plafCursor));
	cursor->next = _plaf.cursorListHead;
	_plaf.cursorListHead = cursor;
	if (!_plafCreateCursor(cursor, image, xhot, yhot)) {
		plafDestroyCursor(cursor);
		return NULL;
	}
	return cursor;
}

plafCursor* plafCreateStandardCursor(int shape) {
	if (shape != STD_CURSOR_ARROW &&
		shape != STD_CURSOR_IBEAM &&
		shape != STD_CURSOR_CROSSHAIR &&
		shape != STD_CURSOR_POINTING_HAND &&
		shape != STD_CURSOR_HORIZONTAL_RESIZE &&
		shape != STD_CURSOR_VERTICAL_RESIZE) {
		return NULL;
	}
	plafCursor* cursor = _plaf_calloc(1, sizeof(plafCursor));
	cursor->next = _plaf.cursorListHead;
	_plaf.cursorListHead = cursor;
	if (!_plafCreateStandardCursor(cursor, shape)) {
		plafDestroyCursor(cursor);
		return NULL;
	}
	return cursor;
}

void plafDestroyCursor(plafCursor* cursor) {
	if (cursor == NULL) {
		return;
	}
	plafWindow* window;
	for (window = _plaf.windowListHead;  window;  window = window->next) {
		if (window->cursor == cursor) {
			plafSetCursor(window, NULL);
		}
	}
	_plafDestroyCursor(cursor);
	plafCursor** prev = &_plaf.cursorListHead;
	while (*prev != cursor) {
		prev = &((*prev)->next);
	}
	*prev = cursor->next;
	_plaf_free(cursor);
}
