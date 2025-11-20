#if defined(__linux__)
#include "platform.h"

void glfwGetCursorPos(plafWindow* window, double* xpos, double* ypos) {
	Window root, child;
	int rootX, rootY, childX, childY;
	unsigned int mask;
	_glfw.xlibQueryPointer(_glfw.x11Display, window->x11Window, &root, &child, &rootX, &rootY, &childX, &childY, &mask);
	*xpos = childX;
	*ypos = childY;
}

void _glfwSetCursorPos(plafWindow* window, double xpos, double ypos) {
	window->x11WarpCursorPosX = (int)xpos;
	window->x11WarpCursorPosY = (int)ypos;
	_glfw.xlibWarpPointer(_glfw.x11Display, None, window->x11Window, 0, 0, 0, 0, (int)xpos, (int)ypos);
	_glfw.xlibFlush(_glfw.x11Display);
}

void _glfwSetCursor(plafWindow* window) {
	if (window->cursorMode == CURSOR_NORMAL) {
		updateCursorImage(window);
		_glfw.xlibFlush(_glfw.x11Display);
	}
}

#endif // __linux__