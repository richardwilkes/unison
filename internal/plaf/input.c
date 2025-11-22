#include "platform.h"

#include <math.h>


//////////////////////////////////////////////////////////////////////////
//////                         PLAF event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code of a physical key event
//
void _plafInputKey(plafWindow* window, int key, int scancode, int action, int mods)
{
	if (key >= 0 && key <= KEY_LAST)
	{
		IntBool repeated = false;

		if (action == INPUT_RELEASE && window->keys[key] == INPUT_RELEASE)
			return;

		if (action == INPUT_PRESS && window->keys[key] == INPUT_PRESS)
			repeated = true;

		window->keys[key] = (char) action;

		if (repeated)
			action = INPUT_REPEAT;
	}
	if (window->keyCallback)
		window->keyCallback(window, key, scancode, action, mods);
}

// Notifies shared code of a Unicode codepoint input event
// The 'plain' parameter determines whether to emit a regular character event
//
void _plafInputChar(plafWindow* window, uint32_t codepoint, int mods, IntBool plain)
{
	if (codepoint < 32 || (codepoint > 126 && codepoint < 160))
		return;
	if (window->charModsCallback)
		window->charModsCallback(window, codepoint, mods);

	if (plain)
	{
		if (window->charCallback)
			window->charCallback(window, codepoint);
	}
}

// Notifies shared code of a scroll event
//
void _plafInputScroll(plafWindow* window, double xoffset, double yoffset)
{
	if (window->scrollCallback)
		window->scrollCallback(window, xoffset, yoffset);
}

// Notifies shared code of a mouse button click event
//
void _plafInputMouseClick(plafWindow* window, int button, int action, int mods)
{
	if (button < 0 || button > MOUSE_BUTTON_LAST)
		return;
	window->mouseButtons[button] = (char) action;
	if (window->mouseButtonCallback)
		window->mouseButtonCallback(window, button, action, mods);
}

// Notifies shared code of a cursor motion event
// The position is specified in content area relative screen coordinates
//
void _plafInputCursorPos(plafWindow* window, double xpos, double ypos)
{
	if (window->virtualCursorPosX == xpos && window->virtualCursorPosY == ypos)
		return;

	window->virtualCursorPosX = xpos;
	window->virtualCursorPosY = ypos;

	if (window->cursorPosCallback)
		window->cursorPosCallback(window, xpos, ypos);
}

// Notifies shared code of a cursor enter/leave event
//
void _plafInputCursorEnter(plafWindow* window, IntBool entered)
{
	if (window->cursorEnterCallback)
		window->cursorEnterCallback(window, entered);
}

// Notifies shared code of files or directories dropped on a window
//
void _plafInputDrop(plafWindow* window, int count, const char** paths)
{
	if (window->dropCallback)
		window->dropCallback(window, count, paths);
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Center the cursor in the content area of the specified window
//
void _plafCenterCursorInContentArea(plafWindow* window)
{
	int width, height;

	_plafGetWindowSize(window, &width, &height);
	_plafSetCursorPos(window, width / 2.0, height / 2.0);
}


//////////////////////////////////////////////////////////////////////////
//////                        PLAF public API                       //////
//////////////////////////////////////////////////////////////////////////

void plafHideCursor(plafWindow* window) {
	if (!window->cursorHidden) {
		window->cursorHidden = true;
		plafGetCursorPos(window, &window->virtualCursorPosX, &window->virtualCursorPosY);
		plafUpdateCursor(window);
	}
}

void plafShowCursor(plafWindow* window) {
	if (window->cursorHidden) {
		window->cursorHidden = false;
		plafUpdateCursor(window);
	}
}

int plafGetKeyScancode(int key)
{
	if (key < KEY_SPACE || key > KEY_LAST)
	{
		_plafInputError("Invalid key %i", key);
		return -1;
	}

	return _plaf.scanCodes[key];
}

int plafGetKey(plafWindow* window, int key)
{
	if (key < KEY_SPACE || key > KEY_LAST)
	{
		_plafInputError("Invalid key %i", key);
		return INPUT_RELEASE;
	}
	return (int) window->keys[key];
}

int plafGetMouseButton(plafWindow* window, int button)
{
	if (button < MOUSE_BUTTON_1 || button > MOUSE_BUTTON_LAST)
	{
		_plafInputError("Invalid mouse button %i", button);
		return INPUT_RELEASE;
	}
	return (int) window->mouseButtons[button];
}

plafCursor* plafCreateCursor(const plafImageData* image, int xhot, int yhot)
{
	if (image->width <= 0 || image->height <= 0) {
		_plafInputError("Invalid image dimensions for cursor");
		return NULL;
	}

	plafCursor* cursor = _plaf_calloc(1, sizeof(plafCursor));
	cursor->next = _plaf.cursorListHead;
	_plaf.cursorListHead = cursor;

	if (!_plafCreateCursor(cursor, image, xhot, yhot)) {
		plafDestroyCursor(cursor);
		return NULL;
	}

	return cursor;
}

plafCursor* plafCreateStandardCursor(int shape)
{
	if (shape != STD_CURSOR_ARROW &&
		shape != STD_CURSOR_IBEAM &&
		shape != STD_CURSOR_CROSSHAIR &&
		shape != STD_CURSOR_POINTING_HAND &&
		shape != STD_CURSOR_HORIZONTAL_RESIZE &&
		shape != STD_CURSOR_VERTICAL_RESIZE)
	{
		_plafInputError("Invalid standard cursor 0x%08X", shape);
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

void plafDestroyCursor(plafCursor* cursor)
{
	if (cursor == NULL)
		return;

	// Make sure the cursor is not being used by any window
	{
		plafWindow* window;

		for (window = _plaf.windowListHead;  window;  window = window->next)
		{
			if (window->cursor == cursor)
				plafSetCursor(window, NULL);
		}
	}

	_plafDestroyCursor(cursor);

	// Unlink cursor from global linked list
	{
		plafCursor** prev = &_plaf.cursorListHead;

		while (*prev != cursor)
			prev = &((*prev)->next);

		*prev = cursor->next;
	}

	_plaf_free(cursor);
}

keyFunc plafSetKeyCallback(plafWindow* window, keyFunc cbfun) {
	SWAP(keyFunc, window->keyCallback, cbfun);
	return cbfun;
}

charFunc plafSetCharCallback(plafWindow* window, charFunc cbfun) {
	SWAP(charFunc, window->charCallback, cbfun);
	return cbfun;
}

charModsFunc plafSetCharModsCallback(plafWindow* window, charModsFunc cbfun) {
	SWAP(charModsFunc, window->charModsCallback, cbfun);
	return cbfun;
}

mouseButtonFunc plafSetMouseButtonCallback(plafWindow* window, mouseButtonFunc cbfun) {
	SWAP(mouseButtonFunc, window->mouseButtonCallback, cbfun);
	return cbfun;
}

cursorPosFunc plafSetCursorPosCallback(plafWindow* window, cursorPosFunc cbfun) {
	SWAP(cursorPosFunc, window->cursorPosCallback, cbfun);
	return cbfun;
}

cursorEnterFunc plafSetCursorEnterCallback(plafWindow* window, cursorEnterFunc cbfun) {
	SWAP(cursorEnterFunc, window->cursorEnterCallback, cbfun);
	return cbfun;
}

scrollFunc plafSetScrollCallback(plafWindow* window, scrollFunc cbfun) {
	SWAP(scrollFunc, window->scrollCallback, cbfun);
	return cbfun;
}

dropFunc plafSetDropCallback(plafWindow* window, dropFunc cbfun) {
	SWAP(dropFunc, window->dropCallback, cbfun);
	return cbfun;
}
