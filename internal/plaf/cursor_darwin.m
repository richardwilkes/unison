#if defined(__APPLE__)
#include "platform.h"

void plafGetCursorPos(plafWindow* window, double* xpos, double* ypos) {
	const NSPoint pos = [window->nsWindow mouseLocationOutsideOfEventStream];
	*xpos = pos.x;
	*ypos = [window->nsView frame].size.height - pos.y;
}

void _plafSetCursorPos(plafWindow* window, double xpos, double ypos) {
	updateCursorImage(window);
	const NSRect contentRect = [window->nsView frame];
	const NSPoint pos = [window->nsWindow mouseLocationOutsideOfEventStream];
	if (window->monitor) {
		CGDisplayMoveCursorToPoint(window->monitor->nsDisplayID, CGPointMake(xpos, ypos));
	} else {
		const NSRect localRect = NSMakeRect(xpos, contentRect.size.height - ypos - 1, 0, 0);
		const NSRect globalRect = [window->nsWindow convertRectToScreen:localRect];
		const NSPoint globalPoint = globalRect.origin;
		CGWarpMouseCursorPosition(CGPointMake(globalPoint.x, _plafTransformYCocoa(globalPoint.y)));
	}
	CGAssociateMouseAndMouseCursorPosition(true);
}

IntBool cursorInContentArea(plafWindow* window) {
	const NSPoint pos = [window->nsWindow mouseLocationOutsideOfEventStream];
	return [window->nsView mouse:pos inRect:[window->nsView frame]];
}

void _plafSetCursor(plafWindow* window) {
	if (cursorInContentArea(window)) {
		updateCursorImage(window);
	}
}

#endif // __APPLE__