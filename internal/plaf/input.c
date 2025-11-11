#include "platform.h"

#include <float.h>
#include <math.h>
#include <string.h>

// Internal key state used for sticky keys
#define _GLFW_STICK 3

#define GLFW_MOD_MASK (GLFW_MOD_SHIFT | \
                       GLFW_MOD_CONTROL | \
                       GLFW_MOD_ALT | \
                       GLFW_MOD_SUPER | \
                       GLFW_MOD_CAPS_LOCK | \
                       GLFW_MOD_NUM_LOCK)


//////////////////////////////////////////////////////////////////////////
//////                         GLFW event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code of a physical key event
//
void _glfwInputKey(_GLFWwindow* window, int key, int scancode, int action, int mods)
{
    if (key >= 0 && key <= GLFW_KEY_LAST)
    {
        GLFWbool repeated = false;

        if (action == GLFW_RELEASE && window->keys[key] == GLFW_RELEASE)
            return;

        if (action == GLFW_PRESS && window->keys[key] == GLFW_PRESS)
            repeated = true;

        if (action == GLFW_RELEASE && window->stickyKeys)
            window->keys[key] = _GLFW_STICK;
        else
            window->keys[key] = (char) action;

        if (repeated)
            action = GLFW_REPEAT;
    }

    if (!window->lockKeyMods)
        mods &= ~(GLFW_MOD_CAPS_LOCK | GLFW_MOD_NUM_LOCK);

    if (window->callbacks.key)
        window->callbacks.key((GLFWwindow*) window, key, scancode, action, mods);
}

// Notifies shared code of a Unicode codepoint input event
// The 'plain' parameter determines whether to emit a regular character event
//
void _glfwInputChar(_GLFWwindow* window, uint32_t codepoint, int mods, GLFWbool plain)
{
    if (codepoint < 32 || (codepoint > 126 && codepoint < 160))
        return;

    if (!window->lockKeyMods)
        mods &= ~(GLFW_MOD_CAPS_LOCK | GLFW_MOD_NUM_LOCK);

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
    if (button < 0 || (!window->disableMouseButtonLimit && button > GLFW_MOUSE_BUTTON_LAST))
        return;

    if (!window->lockKeyMods)
        mods &= ~(GLFW_MOD_CAPS_LOCK | GLFW_MOD_NUM_LOCK);

    if (button <= GLFW_MOUSE_BUTTON_LAST)
    {
        if (action == GLFW_RELEASE && window->stickyMouseButtons)
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
void _glfwInputCursorEnter(_GLFWwindow* window, GLFWbool entered)
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
        case GLFW_CURSOR:
            return window->cursorMode;
        case GLFW_STICKY_KEYS:
            return window->stickyKeys;
        case GLFW_STICKY_MOUSE_BUTTONS:
            return window->stickyMouseButtons;
        case GLFW_LOCK_KEY_MODS:
            return window->lockKeyMods;
        case GLFW_RAW_MOUSE_MOTION:
            return window->rawMouseMotion;
        case GLFW_UNLIMITED_MOUSE_BUTTONS:
            return window->disableMouseButtonLimit;
    }

    _glfwInputError(GLFW_INVALID_ENUM, "Invalid input mode 0x%08X", mode);
    return 0;
}

void glfwSetInputMode(GLFWwindow* handle, int mode, int value)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    switch (mode)
    {
        case GLFW_CURSOR:
        {
            if (value != GLFW_CURSOR_NORMAL &&
                value != GLFW_CURSOR_HIDDEN &&
                value != GLFW_CURSOR_DISABLED &&
                value != GLFW_CURSOR_CAPTURED)
            {
                _glfwInputError(GLFW_INVALID_ENUM, "Invalid cursor mode 0x%08X", value);
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

        case GLFW_STICKY_KEYS:
        {
            value = value ? true : false;
            if (window->stickyKeys == value)
                return;

            if (!value)
            {
                int i;

                // Release all sticky keys
                for (i = 0;  i <= GLFW_KEY_LAST;  i++)
                {
                    if (window->keys[i] == _GLFW_STICK)
                        window->keys[i] = GLFW_RELEASE;
                }
            }

            window->stickyKeys = value;
            return;
        }

        case GLFW_STICKY_MOUSE_BUTTONS:
        {
            value = value ? true : false;
            if (window->stickyMouseButtons == value)
                return;

            if (!value)
            {
                int i;

                // Release all sticky mouse buttons
                for (i = 0;  i <= GLFW_MOUSE_BUTTON_LAST;  i++)
                {
                    if (window->mouseButtons[i] == _GLFW_STICK)
                        window->mouseButtons[i] = GLFW_RELEASE;
                }
            }

            window->stickyMouseButtons = value;
            return;
        }

        case GLFW_LOCK_KEY_MODS:
        {
            window->lockKeyMods = value ? true : false;
            return;
        }

        case GLFW_RAW_MOUSE_MOTION:
        {
            if (!_glfw.platform.rawMouseMotionSupported())
            {
                _glfwInputError(GLFW_PLATFORM_ERROR, "Raw mouse motion is not supported on this system");
                return;
            }

            value = value ? true : false;
            if (window->rawMouseMotion == value)
                return;

            window->rawMouseMotion = value;
            _glfw.platform.setRawMouseMotion(window, value);
            return;
        }

        case GLFW_UNLIMITED_MOUSE_BUTTONS:
        {
            window->disableMouseButtonLimit = value ? true : false;
            return;
        }
    }

    _glfwInputError(GLFW_INVALID_ENUM, "Invalid input mode 0x%08X", mode);
}

int glfwRawMouseMotionSupported(void)
{
    return _glfw.platform.rawMouseMotionSupported();
}

const char* glfwGetKeyName(int key, int scancode)
{
    if (key != GLFW_KEY_UNKNOWN)
    {
        if (key < GLFW_KEY_SPACE || key > GLFW_KEY_LAST)
        {
            _glfwInputError(GLFW_INVALID_ENUM, "Invalid key %i", key);
            return NULL;
        }

        if (key != GLFW_KEY_KP_EQUAL &&
            (key < GLFW_KEY_KP_0 || key > GLFW_KEY_KP_ADD) &&
            (key < GLFW_KEY_APOSTROPHE || key > GLFW_KEY_WORLD_2))
        {
            return NULL;
        }

        scancode = _glfw.platform.getKeyScancode(key);
    }

    return _glfw.platform.getScancodeName(scancode);
}

int glfwGetKeyScancode(int key)
{
    if (key < GLFW_KEY_SPACE || key > GLFW_KEY_LAST)
    {
        _glfwInputError(GLFW_INVALID_ENUM, "Invalid key %i", key);
        return -1;
    }

    return _glfw.platform.getKeyScancode(key);
}

int glfwGetKey(GLFWwindow* handle, int key)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;

    if (key < GLFW_KEY_SPACE || key > GLFW_KEY_LAST)
    {
        _glfwInputError(GLFW_INVALID_ENUM, "Invalid key %i", key);
        return GLFW_RELEASE;
    }

    if (window->keys[key] == _GLFW_STICK)
    {
        // Sticky mode: release key now
        window->keys[key] = GLFW_RELEASE;
        return GLFW_PRESS;
    }

    return (int) window->keys[key];
}

int glfwGetMouseButton(GLFWwindow* handle, int button)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;

    if (button < GLFW_MOUSE_BUTTON_1 || button > GLFW_MOUSE_BUTTON_LAST)
    {
        _glfwInputError(GLFW_INVALID_ENUM, "Invalid mouse button %i", button);
        return GLFW_RELEASE;
    }

    if (window->mouseButtons[button] == _GLFW_STICK)
    {
        // Sticky mode: release mouse button now
        window->mouseButtons[button] = GLFW_RELEASE;
        return GLFW_PRESS;
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

    if (window->cursorMode == GLFW_CURSOR_DISABLED)
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
        _glfwInputError(GLFW_INVALID_VALUE, "Invalid cursor position %f %f", xpos, ypos);
        return;
    }

    if (!_glfw.platform.windowFocused(window))
        return;

    if (window->cursorMode == GLFW_CURSOR_DISABLED)
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

GLFWcursor* glfwCreateCursor(const GLFWimage* image, int xhot, int yhot)
{
    _GLFWcursor* cursor;

	if (image->width <= 0 || image->height <= 0)
    {
        _glfwInputError(GLFW_INVALID_VALUE, "Invalid image dimensions for cursor");
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

    if (shape != GLFW_ARROW_CURSOR &&
        shape != GLFW_IBEAM_CURSOR &&
        shape != GLFW_CROSSHAIR_CURSOR &&
        shape != GLFW_POINTING_HAND_CURSOR &&
        shape != GLFW_RESIZE_EW_CURSOR &&
        shape != GLFW_RESIZE_NS_CURSOR &&
        shape != GLFW_RESIZE_NWSE_CURSOR &&
        shape != GLFW_RESIZE_NESW_CURSOR &&
        shape != GLFW_RESIZE_ALL_CURSOR &&
        shape != GLFW_NOT_ALLOWED_CURSOR)
    {
        _glfwInputError(GLFW_INVALID_ENUM, "Invalid standard cursor 0x%08X", shape);
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

GLFWkeyfun glfwSetKeyCallback(GLFWwindow* handle, GLFWkeyfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWkeyfun, window->callbacks.key, cbfun);
    return cbfun;
}

GLFWcharfun glfwSetCharCallback(GLFWwindow* handle, GLFWcharfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWcharfun, window->callbacks.character, cbfun);
    return cbfun;
}

GLFWcharmodsfun glfwSetCharModsCallback(GLFWwindow* handle, GLFWcharmodsfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWcharmodsfun, window->callbacks.charmods, cbfun);
    return cbfun;
}

GLFWmousebuttonfun glfwSetMouseButtonCallback(GLFWwindow* handle,
                                                      GLFWmousebuttonfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWmousebuttonfun, window->callbacks.mouseButton, cbfun);
    return cbfun;
}

GLFWcursorposfun glfwSetCursorPosCallback(GLFWwindow* handle,
                                                  GLFWcursorposfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWcursorposfun, window->callbacks.cursorPos, cbfun);
    return cbfun;
}

GLFWcursorenterfun glfwSetCursorEnterCallback(GLFWwindow* handle,
                                                      GLFWcursorenterfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWcursorenterfun, window->callbacks.cursorEnter, cbfun);
    return cbfun;
}

GLFWscrollfun glfwSetScrollCallback(GLFWwindow* handle,
                                            GLFWscrollfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWscrollfun, window->callbacks.scroll, cbfun);
    return cbfun;
}

GLFWdropfun glfwSetDropCallback(GLFWwindow* handle, GLFWdropfun cbfun)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    _GLFW_SWAP(GLFWdropfun, window->callbacks.drop, cbfun);
    return cbfun;
}
