#if defined(__linux__)
#include "platform.h"

void getCursorPosInternal(_GLFWwindow* window, double* xpos, double* ypos) {
	Window root, child;
	int rootX, rootY, childX, childY;
	unsigned int mask;
	XQueryPointer(_glfw.x11.display, window->x11.handle, &root, &child, &rootX, &rootY, &childX, &childY, &mask);
	*xpos = childX;
	*ypos = childY;
}

void setCursorPosInternal(_GLFWwindow* window, double xpos, double ypos) {
	window->x11.warpCursorPosX = (int)xpos;
	window->x11.warpCursorPosY = (int)ypos;
	XWarpPointer(_glfw.x11.display, None, window->x11.handle, 0, 0, 0, 0, (int)xpos, (int)ypos);
	XFlush(_glfw.x11.display);
}

void setCursorInternal(_GLFWwindow* window) {
	if (window->cursorMode == CURSOR_NORMAL) {
		updateCursorImage(window);
		XFlush(_glfw.x11.display);
	}
}

#endif // __linux__