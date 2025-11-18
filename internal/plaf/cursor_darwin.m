#if defined(__APPLE__)
#include "platform.h"

void getCursorPosInternal(plafWindow* window, double* xpos, double* ypos) {
	const NSPoint pos = [window->ns.object mouseLocationOutsideOfEventStream];
	*xpos = pos.x;
	*ypos = [window->ns.view frame].size.height - pos.y;
}

void setCursorPosInternal(plafWindow* window, double xpos, double ypos) {
	updateCursorImage(window);
	const NSRect contentRect = [window->ns.view frame];
	const NSPoint pos = [window->ns.object mouseLocationOutsideOfEventStream];
	window->ns.cursorWarpDeltaX += xpos - pos.x;
	window->ns.cursorWarpDeltaY += ypos - contentRect.size.height + pos.y;
	if (window->monitor) {
		CGDisplayMoveCursorToPoint(window->monitor->nsDisplayID, CGPointMake(xpos, ypos));
	} else {
		const NSRect localRect = NSMakeRect(xpos, contentRect.size.height - ypos - 1, 0, 0);
		const NSRect globalRect = [window->ns.object convertRectToScreen:localRect];
		const NSPoint globalPoint = globalRect.origin;
		CGWarpMouseCursorPosition(CGPointMake(globalPoint.x, _glfwTransformYCocoa(globalPoint.y)));
	}
	CGAssociateMouseAndMouseCursorPosition(true);
}

IntBool cursorInContentArea(plafWindow* window) {
	const NSPoint pos = [window->ns.object mouseLocationOutsideOfEventStream];
	return [window->ns.view mouse:pos inRect:[window->ns.view frame]];
}

void setCursorInternal(plafWindow* window) {
	if (cursorInContentArea(window)) {
		updateCursorImage(window);
	}
}

#endif // __APPLE__