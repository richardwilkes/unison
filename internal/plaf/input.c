#include "platform.h"

#include <float.h>
#include <math.h>
#include <string.h>

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
void _glfwInputKey(_GLFWwindow* window, int key, int scancode, int action, int mods)
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
        window->callbacks.key((GLFWwindow*) window, key, scancode, action, mods);
}

// Notifies shared code of a Unicode codepoint input event
// The 'plain' parameter determines whether to emit a regular character event
//
void _glfwInputChar(_GLFWwindow* window, uint32_t codepoint, int mods, IntBool plain)
{
    if (codepoint < 32 || (codepoint > 126 && codepoint < 160))
        return;

    if (!window->lockKeyMods)
        mods &= ~(MOD_CAPS_LOCK | MOD_NUM_LOCK);

    if (window->callbacks.charmods)
        window->callbacks.charmods((GLFWwindow*) window, codepoint, mods);

    if (plain)
    {
        if (window->callbacks.character)
            window->callbacks.character((GLFWwindow*) window, codepoint);
    }
}

// Notifies shared code of a scroll event
//
void _glfwInputScroll(_GLFWwindow* window, double xoffset, double yoffset)
{
    if (window->callbacks.scroll)
        window->callbacks.scroll((GLFWwindow*) window, xoffset, yoffset);
}

// Notifies shared code of a mouse button click event
//
void _glfwInputMouseClick(_GLFWwindow* window, int button, int action, int mods)
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
        window->callbacks.mouseButton((GLFWwindow*) window, button, action, mods);
}

// Notifies shared code of a cursor motion event
// The position is specified in content area relative screen coordinates
//
void _glfwInputCursorPos(_GLFWwindow* window, double xpos, double ypos)
{
    if (window->virtualCursorPosX == xpos && window->virtualCursorPosY == ypos)
        return;

    window->virtualCursorPosX = xpos;
    window->virtualCursorPosY = ypos;

    if (window->callbacks.cursorPos)
        window->callbacks.cursorPos((GLFWwindow*) window, xpos, ypos);
}

// Notifies shared code of a cursor enter/leave event
//
void _glfwInputCursorEnter(_GLFWwindow* window, IntBool entered)
{
    if (window->callbacks.cursorEnter)
        window->callbacks.cursorEnter((GLFWwindow*) window, entered);
}

// Notifies shared code of files or directories dropped on a window
//
void _glfwInputDrop(_GLFWwindow* window, int count, const char** paths)
{
    if (window->callbacks.drop)
        window->callbacks.drop((GLFWwindow*) window, count, paths);
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Center the cursor in the content area of the specified window
//
void _glfwCenterCursorInContentArea(_GLFWwindow* window)
{
    int width, height;

    _glfw.platform.getWindowSize(window, &width, &height);
    _glfw.platform.setCursorPos(window, width / 2.0, height / 2.0);
}


//////////////////////////////////////////////////////////////////////////
//////                        GLFW public API                       //////
//////////////////////////////////////////////////////////////////////////

int glfwGetInputMode(GLFWwindow* handle, int mode)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
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
        case INPUT_MODE_RAW_MOUSE_MOTION:
            return window->rawMouseMotion;
        case INPUT_MODE_UNLIMITED_MOUSE_BUTTONS:
            return window->disableMouseButtonLimit;
    }

    _glfwInputError(ERR_INVALID_ENUM, "Invalid input mode 0x%08X", mode);
    return 0;
}

void glfwSetInputMode(GLFWwindow* handle, int mode, int value)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    switch (mode)
    {
        case INPUT_MODE_CURSOR:
        {
            if (value != CURSOR_NORMAL &&
                value != CURSOR_HIDDEN &&
                value != CURSOR_DISABLED &&
                value != CURSOR_CAPTURED)
            {
                _glfwInputError(ERR_INVALID_ENUM, "Invalid cursor mode 0x%08X", value);
                return;
            }

            if (window->cursorMode == value)
                return;

            window->cursorMode = value;

            _glfw.platform.getCursorPos(window,
                                        &window->virtualCursorPosX,
                                        &window->virtualCursorPosY);
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

        case INPUT_MODE_RAW_MOUSE_MOTION:
        {
            if (!_glfw.platform.rawMouseMotionSupported())
            {
                _glfwInputError(ERR_PLATFORM_ERROR, "Raw mouse motion is not supported on this system");
                return;
            }

            value = value ? true : false;
            if (window->rawMouseMotion == value)
                return;

            window->rawMouseMotion = value;
            _glfw.platform.setRawMouseMotion(window, value);
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

int glfwRawMouseMotionSupported(void)
{
    return _glfw.platform.rawMouseMotionSupported();
}

const char* glfwGetKeyName(int key, int scancode)
{
    if (key != KEY_UNKNOWN)
    {
        if (key < KEY_SPACE || key > KEY_LAST)
        {
            _glfwInputError(ERR_INVALID_ENUM, "Invalid key %i", key);
            return NULL;
        }

        if (key != KEY_KP_EQUAL &&
            (key < KEY_KP_0 || key > KEY_KP_ADD) &&
            (key < KEY_APOSTROPHE || key > KEY_WORLD_2))
        {
            return NULL;
        }

        scancode = _glfw.platform.getKeyScancode(key);
    }

    return _glfw.platform.getScancodeName(scancode);
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

int glfwGetKey(GLFWwindow* handle, int key)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;

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

int glfwGetMouseButton(GLFWwindow* handle, int button)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;

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

void glfwGetCursorPos(GLFWwindow* handle, double* xpos, double* ypos)
{
    if (xpos)
        *xpos = 0;
    if (ypos)
        *ypos = 0;

    _GLFWwindow* window = (_GLFWwindow*) handle;

    if (window->cursorMode == CURSOR_DISABLED)
    {
        if (xpos)
            *xpos = window->virtualCursorPosX;
        if (ypos)
            *ypos = window->virtualCursorPosY;
    }
    else
        _glfw.platform.getCursorPos(window, xpos, ypos);
}

void glfwSetCursorPos(GLFWwindow* handle, double xpos, double ypos)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;

    if (xpos != xpos || xpos < -DBL_MAX || xpos > DBL_MAX ||
        ypos != ypos || ypos < -DBL_MAX || ypos > DBL_MAX)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid cursor position %f %f", xpos, ypos);
        return;
    }

    if (!_glfw.platform.windowFocused(window))
        return;

    if (window->cursorMode == CURSOR_DISABLED)
    {
        // Only update the accumulated position if the cursor is disabled
        window->virtualCursorPosX = xpos;
        window->virtualCursorPosY = ypos;
    }
    else
    {
        // Update system cursor position
        _glfw.platform.setCursorPos(window, xpos, ypos);
    }
}

GLFWcursor* glfwCreateCursor(const ImageData* image, int xhot, int yhot)
{
    _GLFWcursor* cursor;

	if (image->width <= 0 || image->height <= 0)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid image dimensions for cursor");
        return NULL;
    }

    cursor = _glfw_calloc(1, sizeof(_GLFWcursor));
    cursor->next = _glfw.cursorListHead;
    _glfw.cursorListHead = cursor;

    if (!_glfw.platform.createCursor(cursor, image, xhot, yhot))
    {
        glfwDestroyCursor((GLFWcursor*) cursor);
        return NULL;
    }

    return (GLFWcursor*) cursor;
}

GLFWcursor* glfwCreateStandardCursor(int shape)
{
    _GLFWcursor* cursor;

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

    cursor = _glfw_calloc(1, sizeof(_GLFWcursor));
    cursor->next = _glfw.cursorListHead;
    _glfw.cursorListHead = cursor;

    if (!_glfw.platform.createStandardCursor(cursor, shape))
    {
        glfwDestroyCursor((GLFWcursor*) cursor);
        return NULL;
    }

    return (GLFWcursor*) cursor;
}

void glfwDestroyCursor(GLFWcursor* handle)
{
    _GLFWcursor* cursor = (_GLFWcursor*) handle;

    if (cursor == NULL)
        return;

    // Make sure the cursor is not being used by any window
    {
        _GLFWwindow* window;

        for (window = _glfw.windowListHead;  window;  window = window->next)
        {
            if (window->cursor == cursor)
                glfwSetCursor((GLFWwindow*) window, NULL);
        }
    }

    _glfw.platform.destroyCursor(cursor);

    // Unlink cursor from global linked list
    {
        _GLFWcursor** prev = &_glfw.cursorListHead;

        while (*prev != cursor)
            prev = &((*prev)->next);

        *prev = cursor->next;
    }

    _glfw_free(cursor);
}

void glfwSetCursor(GLFWwindow* windowHandle, GLFWcursor* cursorHandle)
{
    _GLFWwindow* window = (_GLFWwindow*) windowHandle;
    _GLFWcursor* cursor = (_GLFWcursor*) cursorHandle;
    window->cursor = cursor;
    _glfw.platform.setCursor(window, cursor);
}

keyFunc glfwSetKeyCallback(GLFWwindow* handle, keyFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(keyFunc, window->callbacks.key, cbfun);
    return cbfun;
}

charFunc glfwSetCharCallback(GLFWwindow* handle, charFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(charFunc, window->callbacks.character, cbfun);
    return cbfun;
}

charModsFunc glfwSetCharModsCallback(GLFWwindow* handle, charModsFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(charModsFunc, window->callbacks.charmods, cbfun);
    return cbfun;
}

mouseButtonFunc glfwSetMouseButtonCallback(GLFWwindow* handle,
                                                      mouseButtonFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(mouseButtonFunc, window->callbacks.mouseButton, cbfun);
    return cbfun;
}

cursorPosFunc glfwSetCursorPosCallback(GLFWwindow* handle,
                                                  cursorPosFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(cursorPosFunc, window->callbacks.cursorPos, cbfun);
    return cbfun;
}

cursorEnterFunc glfwSetCursorEnterCallback(GLFWwindow* handle,
                                                      cursorEnterFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(cursorEnterFunc, window->callbacks.cursorEnter, cbfun);
    return cbfun;
}

scrollFunc glfwSetScrollCallback(GLFWwindow* handle,
                                            scrollFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(scrollFunc, window->callbacks.scroll, cbfun);
    return cbfun;
}

dropFunc glfwSetDropCallback(GLFWwindow* handle, dropFunc cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    SWAP(dropFunc, window->callbacks.drop, cbfun);
    return cbfun;
}
