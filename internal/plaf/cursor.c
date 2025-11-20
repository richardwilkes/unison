#include "platform.h"

void glfwSetCursorPos(plafWindow* window, double xpos, double ypos) {
	if (xpos != xpos || xpos < -DBL_MAX || xpos > DBL_MAX || ypos != ypos || ypos < -DBL_MAX || ypos > DBL_MAX) {
		return;
	}
	if (!_glfw.platform.windowFocused(window)) {
		return;
	}
	_glfwSetCursorPos(window, xpos, ypos);
}

void glfwSetCursor(plafWindow* window, plafCursor* cursor) {
	window->cursor = cursor;
	_glfwSetCursor(window);
}
