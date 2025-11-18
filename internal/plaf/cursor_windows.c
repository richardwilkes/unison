#if defined(_WIN32)
#include "platform.h"

void getCursorPosInternal(plafWindow* window, double* xpos, double* ypos) {
	POINT pos;
	if (GetCursorPos(&pos)) {
		ScreenToClient(window->win32.handle, &pos);
		*xpos = pos.x;
		*ypos = pos.y;
	} else {
		*xpos = 0;
		*ypos = 0;
	}
}

void setCursorPosInternal(plafWindow* window, double xpos, double ypos) {
	window->x11.warpCursorPosX = (int)xpos;
	window->x11.warpCursorPosY = (int)ypos;
	_glfw.x11.xlib.WarpPointer(_glfw.x11.display, None, window->x11.handle, 0, 0, 0, 0, (int)xpos, (int)ypos);
	_glfw.x11.xlib.Flush(_glfw.x11.display);
}

IntBool cursorInContentArea(plafWindow* window) {
	POINT pos;
	if (!GetCursorPos(&pos))
		return false;
	if (WindowFromPoint(pos) != window->win32.handle)
		return false;
	RECT area;
	GetClientRect(window->win32.handle, &area);
	ClientToScreen(window->win32.handle, (POINT*)&area.left);
	ClientToScreen(window->win32.handle, (POINT*)&area.right);
	return PtInRect(&area, pos);
}

void setCursorInternal(plafWindow* window) {
	if (cursorInContentArea(window)) {
		updateCursorImage(window);
	}
}

#endif // _WIN32