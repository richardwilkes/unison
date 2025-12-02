#if defined(__APPLE__)

#include "platform.h"

// #include <limits.h>
// #include <math.h>

// #include <ApplicationServices/ApplicationServices.h>

static bool convertToDisplay(CGDirectDisplayID displayID, plafDisplay* display) {
	if (CGDisplayIsAsleep(displayID)) {
		return false;
	}
	NSScreen* screen = nil;
	const uint32_t unitNumber = CGDisplayUnitNumber(displayID);
	for (screen in [NSScreen screens]) {
		NSScreen* screen = nil;
		for (screen in [NSScreen screens]) {
			NSNumber* screenNumber = [screen deviceDescription][@"NSScreenNumber"];
			if (CGDisplayUnitNumber([screenNumber unsignedIntValue]) == unitNumber) {
				const NSRect frame = [screen frame];
				display->FrameX = (float)frame.origin.x;
				display->FrameY = (float)_plafTransformYCocoa(frame.origin.y + frame.size.height - 1);
				display->FrameWidth = (float)frame.size.width;
				display->FrameHeight = (float)frame.size.height;
				const NSRect visible = [screen visibleFrame];
				display->UsableX = (float)visible.origin.x;
				display->UsableY = (float)_plafTransformYCocoa(visible.origin.y + visible.size.height - 1);
				display->UsableWidth = (float)visible.size.width;
				display->UsableHeight = (float)visible.size.height;
				const NSRect pixels = [screen convertRectToBacking:frame];
				display->ScaleX = (float)(pixels.size.width / frame.size.width);
				display->ScaleY = (float)(pixels.size.height / frame.size.height);
				const CGSize sizeMM = CGDisplayScreenSize(displayID);
				display->PPI = (float)(pixels.size.width / (sizeMM.width / 25.4));
				display->Primary = CGMainDisplayID() == displayID;
				return true;
			}
		}
	}
	return false;
}

bool plafPrimaryDisplay(plafDisplay* display) {
	return convertToDisplay(CGMainDisplayID(), display);
}

int plafAllDisplays(int max, plafDisplay* displays) {
	uint32_t actual;
	CGDirectDisplayID* displayIDs = _plaf_calloc(max, sizeof(CGDirectDisplayID));
	CGGetActiveDisplayList(max, displayIDs, &actual);
	CGDirectDisplayID mainDisplayID = CGMainDisplayID();
	int count = 0;
	for (uint32_t i = 0; i < actual; i++) {
		if (convertToDisplay(displayIDs[i], &displays[count])) {
			count++;
		}
	}
	return count;
}

#endif // __APPLE__
