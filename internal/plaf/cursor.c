#include "platform.h"

void plafSetCursorPos(plafWindow* window, double xpos, double ypos) {
	if (xpos != xpos || xpos < -DBL_MAX || xpos > DBL_MAX || ypos != ypos || ypos < -DBL_MAX || ypos > DBL_MAX) {
		return;
	}
	if (!plafIsWindowFocused(window)) {
		return;
	}
	_plafSetCursorPos(window, xpos, ypos);
}

void plafSetCursor(plafWindow* window, plafCursor* cursor) {
	window->cursor = cursor;
	_plafSetCursor(window);
}
