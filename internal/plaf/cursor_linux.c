#if defined(__linux__)
#include "platform.h"

void getCursorPosInternal(plafWindow* window, double* xpos, double* ypos) {
	Window root, child;
	int rootX, rootY, childX, childY;
	unsigned int mask;
	_glfw.x11.xlib.QueryPointer(_glfw.x11.display, window->x11.handle, &root, &child, &rootX, &rootY, &childX, &childY, &mask);
	*xpos = childX;
	*ypos = childY;
}

void setCursorPosInternal(plafWindow* window, double xpos, double ypos) {
	window->x11.warpCursorPosX = (int)xpos;
	window->x11.warpCursorPosY = (int)ypos;
	_glfw.x11.xlib.WarpPointer(_glfw.x11.display, None, window->x11.handle, 0, 0, 0, 0, (int)xpos, (int)ypos);
	_glfw.x11.xlib.Flush(_glfw.x11.display);
}

void setCursorInternal(plafWindow* window) {
	if (window->cursorMode == CURSOR_NORMAL) {
		updateCursorImage(window);
		_glfw.x11.xlib.Flush(_glfw.x11.display);
	}
}

#endif // __linux__