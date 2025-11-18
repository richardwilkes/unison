#include "platform.h"

void glfwGetCursorPos(plafWindow* handle, double* xpos, double* ypos) {
	getCursorPosInternal((plafWindow*) handle, xpos, ypos);
}

void glfwSetCursorPos(plafWindow* handle, double xpos, double ypos) {
	plafWindow* window = (plafWindow*) handle;
	if (xpos != xpos || xpos < -DBL_MAX || xpos > DBL_MAX || ypos != ypos || ypos < -DBL_MAX || ypos > DBL_MAX) {
		return;
	}
	if (!_glfw.platform.windowFocused(window)) {
		return;
	}
	setCursorPosInternal(window, xpos, ypos);
}

void glfwSetCursor(plafWindow* windowHandle, plafCursor* cursor) {
	plafWindow* window = (plafWindow*) windowHandle;
	window->cursor = cursor;
	setCursorInternal(window);
}
