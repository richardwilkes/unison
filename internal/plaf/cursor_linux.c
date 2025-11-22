#if defined(__linux__)
#include "platform.h"

void plafGetCursorPos(plafWindow* window, double* xpos, double* ypos) {
	Window root, child;
	int rootX, rootY, childX, childY;
	unsigned int mask;
	_plaf.xlibQueryPointer(_plaf.x11Display, window->x11Window, &root, &child, &rootX, &rootY, &childX, &childY, &mask);
	*xpos = childX;
	*ypos = childY;
}

void _plafSetCursorPos(plafWindow* window, double xpos, double ypos) {
	window->x11WarpCursorPosX = (int)xpos;
	window->x11WarpCursorPosY = (int)ypos;
	_plaf.xlibWarpPointer(_plaf.x11Display, None, window->x11Window, 0, 0, 0, 0, (int)xpos, (int)ypos);
	_plaf.xlibFlush(_plaf.x11Display);
}

void _plafSetCursor(plafWindow* window) {
	if (!window->cursorHidden) {
		_plafUpdateCursorImage(window);
		_plaf.xlibFlush(_plaf.x11Display);
	}
}

#endif // __linux__