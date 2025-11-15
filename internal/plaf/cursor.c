#include "platform.h"

void glfwGetCursorPos(GLFWwindow* handle, double* xpos, double* ypos) {
	getCursorPosInternal((_GLFWwindow*) handle, xpos, ypos);
}

void getCursorPosInternal(_GLFWwindow* window, double* xpos, double* ypos) {
#if defined(PLATFORM_DARWIN)
	const NSPoint pos = [window->ns.object mouseLocationOutsideOfEventStream];
	*xpos = pos.x;
	*ypos = [window->ns.view frame].size.height - pos.y;
#elif defined(PLATFORM_LINUX)
	Window root, child;
	int rootX, rootY, childX, childY;
	unsigned int mask;
	XQueryPointer(_glfw.x11.display, window->x11.handle, &root, &child, &rootX, &rootY, &childX, &childY, &mask);
	*xpos = childX;
	*ypos = childY;
#elif defined(PLATFORM_WINDOWS)
	POINT pos;
	if (GetCursorPos(&pos)) {
		ScreenToClient(window->win32.handle, &pos);
		*xpos = pos.x;
		*ypos = pos.y;
	} else {
		*xpos = 0;
		*ypos = 0;
	}
#endif
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

void setCursorPosInternal(_GLFWwindow* window, double xpos, double ypos) {
#if defined(PLATFORM_DARWIN)
	updateCursorImage(window);
	const NSRect contentRect = [window->ns.view frame];
	const NSPoint pos = [window->ns.object mouseLocationOutsideOfEventStream];
	window->ns.cursorWarpDeltaX += xpos - pos.x;
	window->ns.cursorWarpDeltaY += ypos - contentRect.size.height + pos.y;
	if (window->monitor) {
		CGDisplayMoveCursorToPoint(window->monitor->ns.displayID, CGPointMake(xpos, ypos));
	} else {
		const NSRect localRect = NSMakeRect(xpos, contentRect.size.height - ypos - 1, 0, 0);
		const NSRect globalRect = [window->ns.object convertRectToScreen:localRect];
		const NSPoint globalPoint = globalRect.origin;
		CGWarpMouseCursorPosition(CGPointMake(globalPoint.x, _glfwTransformYCocoa(globalPoint.y)));
	}
	CGAssociateMouseAndMouseCursorPosition(true);
#elif defined(PLATFORM_LINUX)
	window->x11.warpCursorPosX = (int)xpos;
	window->x11.warpCursorPosY = (int)ypos;
	XWarpPointer(_glfw.x11.display, None, window->x11.handle, 0, 0, 0, 0, (int)xpos, (int)ypos);
	XFlush(_glfw.x11.display);
#elif defined(PLATFORM_WINDOWS)
	POINT pos = { (int)xpos, (int)ypos };
	window->win32.lastCursorPosX = pos.x;
	window->win32.lastCursorPosY = pos.y;
	ClientToScreen(window->win32.handle, &pos);
	SetCursorPos(pos.x, pos.y);
#endif
}

#if defined(PLATFORM_DARWIN)
IntBool cursorInContentArea(_GLFWwindow* window) {
	const NSPoint pos = [window->ns.object mouseLocationOutsideOfEventStream];
	return [window->ns.view mouse:pos inRect:[window->ns.view frame]];
}
#elif defined (PLATFORM_WINDOWS)
IntBool cursorInContentArea(_GLFWwindow* window) {
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
#endif

void glfwSetCursor(GLFWwindow* windowHandle, plafCursor* cursor) {
	_GLFWwindow* window = (_GLFWwindow*) windowHandle;
	window->cursor = cursor;
#if defined(PLATFORM_DARWIN) || defined(PLATFORM_WINDOWS)
	if (cursorInContentArea(window)) {
		updateCursorImage(window);
	}
#elif defined(PLATFORM_LINUX)
	if (window->cursorMode == CURSOR_NORMAL) {
		updateCursorImage(window);
		XFlush(_glfw.x11.display);
	}
#endif
}
