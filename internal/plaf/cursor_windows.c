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
	POINT pos = { (int)xpos, (int)ypos };
	window->win32.lastCursorPosX = pos.x;
	window->win32.lastCursorPosY = pos.y;
	ClientToScreen(window->win32.handle, &pos);
	SetCursorPos(pos.x, pos.y);
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