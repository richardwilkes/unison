#if defined(_WIN32)
#include "platform.h"

void plafGetCursorPos(plafWindow* window, double* xpos, double* ypos) {
	POINT pos;
	if (GetCursorPos(&pos)) {
		ScreenToClient(window->win32Window, &pos);
		*xpos = pos.x;
		*ypos = pos.y;
	} else {
		*xpos = 0;
		*ypos = 0;
	}
}

void _plafSetCursorPos(plafWindow* window, double xpos, double ypos) {
	POINT pos = { (int)xpos, (int)ypos };
	ClientToScreen(window->win32Window, &pos);
	SetCursorPos(pos.x, pos.y);
}

bool _plafCursorInContentArea(plafWindow* window) {
	POINT pos;
	if (!GetCursorPos(&pos))
		return false;
	if (WindowFromPoint(pos) != window->win32Window)
		return false;
	RECT area;
	GetClientRect(window->win32Window, &area);
	ClientToScreen(window->win32Window, (POINT*)&area.left);
	ClientToScreen(window->win32Window, (POINT*)&area.right);
	return PtInRect(&area, pos);
}

void _plafSetCursor(plafWindow* window) {
	if (_plafCursorInContentArea(window)) {
		_plafUpdateCursorImage(window);
	}
}

#endif // _WIN32