#include "platform.h"

void glfwGetCursorPos(GLFWwindow* handle, double* xpos, double* ypos) {
	getCursorPosInternal((_GLFWwindow*) handle, xpos, ypos);
}

void glfwSetCursorPos(GLFWwindow* handle, double xpos, double ypos) {
	_GLFWwindow* window = (_GLFWwindow*) handle;
	if (xpos != xpos || xpos < -DBL_MAX || xpos > DBL_MAX || ypos != ypos || ypos < -DBL_MAX || ypos > DBL_MAX) {
		return;
	}
	if (!_glfw.platform.windowFocused(window)) {
		return;
	}
	setCursorPosInternal(window, xpos, ypos);
}

void glfwSetCursor(GLFWwindow* windowHandle, plafCursor* cursor) {
	_GLFWwindow* window = (_GLFWwindow*) windowHandle;
	window->cursor = cursor;
	setCursorInternal(window);
}
