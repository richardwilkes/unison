#if defined(__APPLE__)

#include "platform.h"

#include <limits.h>
#include <math.h>

#include <ApplicationServices/ApplicationServices.h>


// Get the name of the specified display, or NULL
//
static char* getMonitorName(CGDirectDisplayID displayID, NSScreen* screen) {
    if (screen) {
		NSString* name = [screen valueForKey:@"localizedName"];
		if (name) {
			return _glfw_strdup([name UTF8String]);
		}
    }
    return _glfw_strdup("Display");
}

// Check whether the display mode should be included in enumeration
//
static IntBool modeIsGood(CGDisplayModeRef mode)
{
    uint32_t flags = CGDisplayModeGetIOFlags(mode);
    if (!(flags & kDisplayModeValidFlag) || !(flags & kDisplayModeSafeFlag))
        return false;
    if (flags & kDisplayModeInterlacedFlag)
        return false;
    if (flags & kDisplayModeStretchedFlag)
        return false;
    return true;
}

// Convert Core Graphics display mode to GLFW video mode
//
static VideoMode vidmodeFromCGDisplayMode(CGDisplayModeRef mode)
{
    VideoMode result;
    result.redBits = 8;
    result.greenBits = 8;
    result.blueBits = 8;
    result.width = (int) CGDisplayModeGetWidth(mode);
    result.height = (int) CGDisplayModeGetHeight(mode);
    result.refreshRate = (int) round(CGDisplayModeGetRefreshRate(mode));
    return result;
}

// Starts reservation for display fading
//
static CGDisplayFadeReservationToken beginFadeReservation(void)
{
    CGDisplayFadeReservationToken token = kCGDisplayFadeReservationInvalidToken;

    if (CGAcquireDisplayFadeReservation(5, &token) == kCGErrorSuccess)
    {
        CGDisplayFade(token, 0.3,
                      kCGDisplayBlendNormal,
                      kCGDisplayBlendSolidColor,
                      0.0, 0.0, 0.0,
                      TRUE);
    }

    return token;
}

// Ends reservation for display fading
//
static void endFadeReservation(CGDisplayFadeReservationToken token)
{
    if (token != kCGDisplayFadeReservationInvalidToken)
    {
        CGDisplayFade(token, 0.5,
                      kCGDisplayBlendSolidColor,
                      kCGDisplayBlendNormal,
                      0.0, 0.0, 0.0,
                      FALSE);
        CGReleaseDisplayFadeReservation(token);
    }
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Poll for changes in the set of connected monitors
//
void _glfwPollMonitorsCocoa(void)
{
    uint32_t displayCount;
    CGGetOnlineDisplayList(0, NULL, &displayCount);
    CGDirectDisplayID* displays = _glfw_calloc(displayCount, sizeof(CGDirectDisplayID));
    CGGetOnlineDisplayList(displayCount, displays, &displayCount);

    for (int i = 0;  i < _glfw.monitorCount;  i++)
        _glfw.monitors[i]->nsScreen = nil;

    plafMonitor** disconnected = NULL;
    uint32_t disconnectedCount = _glfw.monitorCount;
    if (disconnectedCount)
    {
        disconnected = _glfw_calloc(_glfw.monitorCount, sizeof(plafMonitor*));
        memcpy(disconnected,
               _glfw.monitors,
               _glfw.monitorCount * sizeof(plafMonitor*));
    }

    for (uint32_t i = 0;  i < displayCount;  i++)
    {
        if (CGDisplayIsAsleep(displays[i]))
            continue;

        const uint32_t unitNumber = CGDisplayUnitNumber(displays[i]);
        NSScreen* screen = nil;

        for (screen in [NSScreen screens])
        {
            NSNumber* screenNumber = [screen deviceDescription][@"NSScreenNumber"];

            // HACK: Compare unit numbers instead of display IDs to work around
            //       display replacement on machines with automatic graphics
            //       switching
            if (CGDisplayUnitNumber([screenNumber unsignedIntValue]) == unitNumber)
                break;
        }

        // HACK: Compare unit numbers instead of display IDs to work around
        //       display replacement on machines with automatic graphics
        //       switching
        uint32_t j;
        for (j = 0;  j < disconnectedCount;  j++)
        {
            if (disconnected[j] && disconnected[j]->nsUnitNumber == unitNumber)
            {
                disconnected[j]->nsScreen = screen;
                disconnected[j] = NULL;
                break;
            }
        }

        if (j < disconnectedCount)
            continue;

        const CGSize size = CGDisplayScreenSize(displays[i]);
        char* name = getMonitorName(displays[i], screen);
        if (!name)
            continue;

        plafMonitor* monitor = _glfwAllocMonitor(name, size.width, size.height);
        monitor->nsDisplayID  = displays[i];
        monitor->nsUnitNumber = unitNumber;
        monitor->nsScreen     = screen;

        _glfw_free(name);
        _glfwInputMonitor(monitor, CONNECTED, MONITOR_INSERT_LAST);
    }

    for (uint32_t i = 0;  i < disconnectedCount;  i++)
    {
        if (disconnected[i])
            _glfwInputMonitor(disconnected[i], DISCONNECTED, 0);
    }

    _glfw_free(disconnected);
    _glfw_free(displays);
}

// Change the current video mode
//
void _glfwSetVideoModeCocoa(plafMonitor* monitor, const VideoMode* desired)
{
    VideoMode current;
    _glfwGetVideoModeCocoa(monitor, &current);

    const VideoMode* best = _glfwChooseVideoMode(monitor, desired);
    if (_glfwCompareVideoModes(&current, best) == 0)
        return;

    CFArrayRef modes = CGDisplayCopyAllDisplayModes(monitor->nsDisplayID, NULL);
    const CFIndex count = CFArrayGetCount(modes);
    CGDisplayModeRef native = NULL;

    for (CFIndex i = 0;  i < count;  i++)
    {
        CGDisplayModeRef dm = (CGDisplayModeRef) CFArrayGetValueAtIndex(modes, i);
        if (!modeIsGood(dm))
            continue;

        const VideoMode mode = vidmodeFromCGDisplayMode(dm);
        if (_glfwCompareVideoModes(best, &mode) == 0)
        {
            native = dm;
            break;
        }
    }

    if (native)
    {
        if (monitor->nsPreviousMode == NULL)
            monitor->nsPreviousMode = CGDisplayCopyDisplayMode(monitor->nsDisplayID);

        CGDisplayFadeReservationToken token = beginFadeReservation();
        CGDisplaySetDisplayMode(monitor->nsDisplayID, native, NULL);
        endFadeReservation(token);
    }

    CFRelease(modes);
}

// Restore the previously saved (original) video mode
//
void _glfwRestoreVideoModeCocoa(plafMonitor* monitor)
{
    if (monitor->nsPreviousMode)
    {
        CGDisplayFadeReservationToken token = beginFadeReservation();
        CGDisplaySetDisplayMode(monitor->nsDisplayID,
                                monitor->nsPreviousMode, NULL);
        endFadeReservation(token);

        CGDisplayModeRelease(monitor->nsPreviousMode);
        monitor->nsPreviousMode = NULL;
    }
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW platform API                      //////
//////////////////////////////////////////////////////////////////////////

void _glfwFreeMonitorCocoa(plafMonitor* monitor)
{
}

void _glfwGetMonitorPosCocoa(plafMonitor* monitor, int* xpos, int* ypos)
{
    const CGRect bounds = CGDisplayBounds(monitor->nsDisplayID);
    *xpos = (int) bounds.origin.x;
    *ypos = (int) bounds.origin.y;
}

void _glfwGetMonitorContentScaleCocoa(plafMonitor* monitor,
                                      float* xscale, float* yscale)
{
    @autoreleasepool {

    if (!monitor->nsScreen)
    {
        _glfwInputError(ERR_PLATFORM_ERROR, "Cocoa: Cannot query content scale without screen");
    }

    const NSRect points = [monitor->nsScreen frame];
    const NSRect pixels = [monitor->nsScreen convertRectToBacking:points];

    if (xscale)
        *xscale = (float) (pixels.size.width / points.size.width);
    if (yscale)
        *yscale = (float) (pixels.size.height / points.size.height);

    } // autoreleasepool
}

void _glfwGetMonitorWorkareaCocoa(plafMonitor* monitor,
                                  int* xpos, int* ypos,
                                  int* width, int* height)
{
    @autoreleasepool {

    if (!monitor->nsScreen)
    {
        _glfwInputError(ERR_PLATFORM_ERROR, "Cocoa: Cannot query workarea without screen");
    }

    const NSRect frameRect = [monitor->nsScreen visibleFrame];

    if (xpos)
        *xpos = frameRect.origin.x;
    if (ypos)
        *ypos = _glfwTransformYCocoa(frameRect.origin.y + frameRect.size.height - 1);
    if (width)
        *width = frameRect.size.width;
    if (height)
        *height = frameRect.size.height;

    } // autoreleasepool
}

VideoMode* _glfwGetVideoModesCocoa(plafMonitor* monitor, int* count)
{
    @autoreleasepool {

    *count = 0;

    CFArrayRef modes = CGDisplayCopyAllDisplayModes(monitor->nsDisplayID, NULL);
    const CFIndex found = CFArrayGetCount(modes);
    VideoMode* result = _glfw_calloc(found, sizeof(VideoMode));

    for (CFIndex i = 0;  i < found;  i++)
    {
        CGDisplayModeRef dm = (CGDisplayModeRef) CFArrayGetValueAtIndex(modes, i);
        if (!modeIsGood(dm))
            continue;

        const VideoMode mode = vidmodeFromCGDisplayMode(dm);
        CFIndex j;

        for (j = 0;  j < *count;  j++)
        {
            if (_glfwCompareVideoModes(result + j, &mode) == 0)
                break;
        }

        // Skip duplicate modes
        if (j < *count)
            continue;

        (*count)++;
        result[*count - 1] = mode;
    }

    CFRelease(modes);
    return result;

    } // autoreleasepool
}

IntBool _glfwGetVideoModeCocoa(plafMonitor* monitor, VideoMode *mode)
{
    @autoreleasepool {

    CGDisplayModeRef native = CGDisplayCopyDisplayMode(monitor->nsDisplayID);
    if (!native)
    {
        _glfwInputError(ERR_PLATFORM_ERROR, "Cocoa: Failed to query display mode");
        return false;
    }

    *mode = vidmodeFromCGDisplayMode(native);
    CGDisplayModeRelease(native);
    return true;

    } // autoreleasepool
}

IntBool _glfwGetGammaRampCocoa(plafMonitor* monitor, GammaRamp* ramp)
{
    @autoreleasepool {

    uint32_t size = CGDisplayGammaTableCapacity(monitor->nsDisplayID);
    CGGammaValue* values = _glfw_calloc(size * 3, sizeof(CGGammaValue));

    CGGetDisplayTransferByTable(monitor->nsDisplayID,
                                size,
                                values,
                                values + size,
                                values + size * 2,
                                &size);

    _glfwAllocGammaArrays(ramp, size);

    for (uint32_t i = 0; i < size; i++)
    {
        ramp->red[i]   = (unsigned short) (values[i] * 65535);
        ramp->green[i] = (unsigned short) (values[i + size] * 65535);
        ramp->blue[i]  = (unsigned short) (values[i + size * 2] * 65535);
    }

    _glfw_free(values);
    return true;

    } // autoreleasepool
}

void _glfwSetGammaRampCocoa(plafMonitor* monitor, const GammaRamp* ramp)
{
    @autoreleasepool {

    CGGammaValue* values = _glfw_calloc(ramp->size * 3, sizeof(CGGammaValue));

    for (unsigned int i = 0;  i < ramp->size;  i++)
    {
        values[i]                  = ramp->red[i] / 65535.f;
        values[i + ramp->size]     = ramp->green[i] / 65535.f;
        values[i + ramp->size * 2] = ramp->blue[i] / 65535.f;
    }

    CGSetDisplayTransferByTable(monitor->nsDisplayID,
                                ramp->size,
                                values,
                                values + ramp->size,
                                values + ramp->size * 2);

    _glfw_free(values);

    } // autoreleasepool
}

#endif // __APPLE__
