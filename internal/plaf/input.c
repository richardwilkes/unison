#include "platform.h"

#include <math.h>

// Internal key state used for sticky keys
#define _GLFW_STICK 3

#define MOD_MASK (MOD_SHIFT | \
					   MOD_CONTROL | \
					   MOD_ALT | \
					   MOD_SUPER | \
					   MOD_CAPS_LOCK | \
					   MOD_NUM_LOCK)


//////////////////////////////////////////////////////////////////////////
//////                         GLFW event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code of a physical key event
//
void _glfwInputKey(plafWindow* window, int key, int scancode, int action, int mods)
{
	if (key >= 0 && key <= KEY_LAST)
	{
		IntBool repeated = false;

		if (action == INPUT_RELEASE && window->keys[key] == INPUT_RELEASE)
			return;

		if (action == INPUT_PRESS && window->keys[key] == INPUT_PRESS)
			repeated = true;

		if (action == INPUT_RELEASE && window->stickyKeys)
			window->keys[key] = _GLFW_STICK;
		else
			window->keys[key] = (char) action;

		if (repeated)
			action = INPUT_REPEAT;
	}

	if (!window->lockKeyMods)
		mods &= ~(MOD_CAPS_LOCK | MOD_NUM_LOCK);

	if (window->callbacks.key)
		window->callbacks.key((plafWindow*) window, key, scancode, action, mods);
}

// Notifies shared code of a Unicode codepoint input event
// The 'plain' parameter determines whether to emit a regular character event
//
void _glfwInputChar(plafWindow* window, uint32_t codepoint, int mods, IntBool plain)
{
	if (codepoint < 32 || (codepoint > 126 && codepoint < 160))
		return;

	if (!window->lockKeyMods)
		mods &= ~(MOD_CAPS_LOCK | MOD_NUM_LOCK);

	if (window->callbacks.charmods)
		window->callbacks.charmods((plafWindow*) window, codepoint, mods);

	if (plain)
	{
		if (window->callbacks.character)
			window->callbacks.character((plafWindow*) window, codepoint);
	}
}

// Notifies shared code of a scroll event
//
void _glfwInputScroll(plafWindow* window, double xoffset, double yoffset)
{
	if (window->callbacks.scroll)
		window->callbacks.scroll((plafWindow*) window, xoffset, yoffset);
}

// Notifies shared code of a mouse button click event
//
void _glfwInputMouseClick(plafWindow* window, int button, int action, int mods)
{
	if (button < 0 || (!window->disableMouseButtonLimit && button > MOUSE_BUTTON_LAST))
		return;

	if (!window->lockKeyMods)
		mods &= ~(MOD_CAPS_LOCK | MOD_NUM_LOCK);

	if (button <= MOUSE_BUTTON_LAST)
	{
		if (action == INPUT_RELEASE && window->stickyMouseButtons)
			window->mouseButtons[button] = _GLFW_STICK;
		else
			window->mouseButtons[button] = (char) action;
	}

	if (window->callbacks.mouseButton)
		window->callbacks.mouseButton((plafWindow*) window, button, action, mods);
}

// Notifies shared code of a cursor motion event
// The position is specified in content area relative screen coordinates
//
void _glfwInputCursorPos(plafWindow* window, double xpos, double ypos)
{
	if (window->virtualCursorPosX == xpos && window->virtualCursorPosY == ypos)
		return;

	window->virtualCursorPosX = xpos;
	window->virtualCursorPosY = ypos;

	if (window->callbacks.cursorPos)
		window->callbacks.cursorPos((plafWindow*) window, xpos, ypos);
}

// Notifies shared code of a cursor enter/leave event
//
void _glfwInputCursorEnter(plafWindow* window, IntBool entered)
{
	if (window->callbacks.cursorEnter)
		window->callbacks.cursorEnter((plafWindow*) window, entered);
}

// Notifies shared code of files or directories dropped on a window
//
void _glfwInputDrop(plafWindow* window, int count, const char** paths)
{
	if (window->callbacks.drop)
		window->callbacks.drop((plafWindow*) window, count, paths);
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Center the cursor in the content area of the specified window
//
void _glfwCenterCursorInContentArea(plafWindow* window)
{
	int width, height;

	_glfw.platform.getWindowSize(window, &width, &height);
	setCursorPosInternal(window, width / 2.0, height / 2.0);
}


//////////////////////////////////////////////////////////////////////////
//////                        GLFW public API                       //////
//////////////////////////////////////////////////////////////////////////

int glfwGetInputMode(plafWindow* handle, int mode)
{
	plafWindow* window = (plafWindow*) handle;
	switch (mode)
	{
		case INPUT_MODE_CURSOR:
			return window->cursorMode;
		case INPUT_MODE_STICKY_KEYS:
			return window->stickyKeys;
		case INPUT_MODE_STICKY_MOUSE_BUTTONS:
			return window->stickyMouseButtons;
		case INPUT_MODE_LOCK_KEY_MODS:
			return window->lockKeyMods;
		case INPUT_MODE_UNLIMITED_MOUSE_BUTTONS:
			return window->disableMouseButtonLimit;
	}

	_glfwInputError(ERR_INVALID_ENUM, "Invalid input mode 0x%08X", mode);
	return 0;
}

void glfwSetInputMode(plafWindow* handle, int mode, int value)
{
	plafWindow* window = (plafWindow*) handle;
	switch (mode)
	{
		case INPUT_MODE_CURSOR:
		{
			if (value != CURSOR_NORMAL &&
				value != CURSOR_HIDDEN)
			{
				_glfwInputError(ERR_INVALID_ENUM, "Invalid cursor mode 0x%08X", value);
				return;
			}

			if (window->cursorMode == value)
				return;

			window->cursorMode = value;

			getCursorPosInternal(window, &window->virtualCursorPosX, &window->virtualCursorPosY);
			_glfw.platform.setCursorMode(window, value);
			return;
		}

		case INPUT_MODE_STICKY_KEYS:
		{
			value = value ? true : false;
			if (window->stickyKeys == value)
				return;

			if (!value)
			{
				int i;

				// Release all sticky keys
				for (i = 0;  i <= KEY_LAST;  i++)
				{
					if (window->keys[i] == _GLFW_STICK)
						window->keys[i] = INPUT_RELEASE;
				}
			}

			window->stickyKeys = value;
			return;
		}

		case INPUT_MODE_STICKY_MOUSE_BUTTONS:
		{
			value = value ? true : false;
			if (window->stickyMouseButtons == value)
				return;

			if (!value)
			{
				int i;

				// Release all sticky mouse buttons
				for (i = 0;  i <= MOUSE_BUTTON_LAST;  i++)
				{
					if (window->mouseButtons[i] == _GLFW_STICK)
						window->mouseButtons[i] = INPUT_RELEASE;
				}
			}

			window->stickyMouseButtons = value;
			return;
		}

		case INPUT_MODE_LOCK_KEY_MODS:
		{
			window->lockKeyMods = value ? true : false;
			return;
		}

		case INPUT_MODE_UNLIMITED_MOUSE_BUTTONS:
		{
			window->disableMouseButtonLimit = value ? true : false;
			return;
		}
	}

	_glfwInputError(ERR_INVALID_ENUM, "Invalid input mode 0x%08X", mode);
}

int glfwGetKeyScancode(int key)
{
	if (key < KEY_SPACE || key > KEY_LAST)
	{
		_glfwInputError(ERR_INVALID_ENUM, "Invalid key %i", key);
		return -1;
	}

	return _glfw.platform.getKeyScancode(key);
}

int glfwGetKey(plafWindow* handle, int key)
{
	plafWindow* window = (plafWindow*) handle;

	if (key < KEY_SPACE || key > KEY_LAST)
	{
		_glfwInputError(ERR_INVALID_ENUM, "Invalid key %i", key);
		return INPUT_RELEASE;
	}

	if (window->keys[key] == _GLFW_STICK)
	{
		// Sticky mode: release key now
		window->keys[key] = INPUT_RELEASE;
		return INPUT_PRESS;
	}

	return (int) window->keys[key];
}

int glfwGetMouseButton(plafWindow* handle, int button)
{
	plafWindow* window = (plafWindow*) handle;

	if (button < MOUSE_BUTTON_1 || button > MOUSE_BUTTON_LAST)
	{
		_glfwInputError(ERR_INVALID_ENUM, "Invalid mouse button %i", button);
		return INPUT_RELEASE;
	}

	if (window->mouseButtons[button] == _GLFW_STICK)
	{
		// Sticky mode: release mouse button now
		window->mouseButtons[button] = INPUT_RELEASE;
		return INPUT_PRESS;
	}

	return (int) window->mouseButtons[button];
}

plafCursor* glfwCreateCursor(const ImageData* image, int xhot, int yhot)
{
	if (image->width <= 0 || image->height <= 0) {
		_glfwInputError(ERR_INVALID_VALUE, "Invalid image dimensions for cursor");
		return NULL;
	}

	plafCursor* cursor = _glfw_calloc(1, sizeof(plafCursor));
	cursor->next = _glfw.cursorListHead;
	_glfw.cursorListHead = cursor;

	if (!_glfw.platform.createCursor(cursor, image, xhot, yhot))
	{
		glfwDestroyCursor(cursor);
		return NULL;
	}

	return cursor;
}

plafCursor* glfwCreateStandardCursor(int shape)
{
	if (shape != STD_CURSOR_ARROW &&
		shape != STD_CURSOR_IBEAM &&
		shape != STD_CURSOR_CROSSHAIR &&
		shape != STD_CURSOR_POINTING_HAND &&
		shape != STD_CURSOR_HORIZONTAL_RESIZE &&
		shape != STD_CURSOR_VERTICAL_RESIZE)
	{
		_glfwInputError(ERR_INVALID_ENUM, "Invalid standard cursor 0x%08X", shape);
		return NULL;
	}

	plafCursor* cursor = _glfw_calloc(1, sizeof(plafCursor));
	cursor->next = _glfw.cursorListHead;
	_glfw.cursorListHead = cursor;

	if (!_glfw.platform.createStandardCursor(cursor, shape))
	{
		glfwDestroyCursor(cursor);
		return NULL;
	}

	return cursor;
}

void glfwDestroyCursor(plafCursor* cursor)
{
	if (cursor == NULL)
		return;

	// Make sure the cursor is not being used by any window
	{
		plafWindow* window;

		for (window = _glfw.windowListHead;  window;  window = window->next)
		{
			if (window->cursor == cursor)
				glfwSetCursor((plafWindow*) window, NULL);
		}
	}

	_glfw.platform.destroyCursor(cursor);

	// Unlink cursor from global linked list
	{
		plafCursor** prev = &_glfw.cursorListHead;

		while (*prev != cursor)
			prev = &((*prev)->next);

		*prev = cursor->next;
	}

	_glfw_free(cursor);
}

keyFunc glfwSetKeyCallback(plafWindow* handle, keyFunc cbfun)
{
	plafWindow* window = (plafWindow*) handle;
	SWAP(keyFunc, window->callbacks.key, cbfun);
	return cbfun;
}

charFunc glfwSetCharCallback(plafWindow* handle, charFunc cbfun)
{
	plafWindow* window = (plafWindow*) handle;
	SWAP(charFunc, window->callbacks.character, cbfun);
	return cbfun;
}

charModsFunc glfwSetCharModsCallback(plafWindow* handle, charModsFunc cbfun)
{
	plafWindow* window = (plafWindow*) handle;
	SWAP(charModsFunc, window->callbacks.charmods, cbfun);
	return cbfun;
}

mouseButtonFunc glfwSetMouseButtonCallback(plafWindow* handle,
													  mouseButtonFunc cbfun)
{
	plafWindow* window = (plafWindow*) handle;
	SWAP(mouseButtonFunc, window->callbacks.mouseButton, cbfun);
	return cbfun;
}

cursorPosFunc glfwSetCursorPosCallback(plafWindow* handle,
												  cursorPosFunc cbfun)
{
	plafWindow* window = (plafWindow*) handle;
	SWAP(cursorPosFunc, window->callbacks.cursorPos, cbfun);
	return cbfun;
}

cursorEnterFunc glfwSetCursorEnterCallback(plafWindow* handle,
													  cursorEnterFunc cbfun)
{
	plafWindow* window = (plafWindow*) handle;
	SWAP(cursorEnterFunc, window->callbacks.cursorEnter, cbfun);
	return cbfun;
}

scrollFunc glfwSetScrollCallback(plafWindow* handle,
											scrollFunc cbfun)
{
	plafWindow* window = (plafWindow*) handle;
	SWAP(scrollFunc, window->callbacks.scroll, cbfun);
	return cbfun;
}

dropFunc glfwSetDropCallback(plafWindow* handle, dropFunc cbfun)
{
	plafWindow* window = (plafWindow*) handle;
	SWAP(dropFunc, window->callbacks.drop, cbfun);
	return cbfun;
}
