#include "platform.h"

#if defined(__linux__)

#include <limits.h>
#include <math.h>


// Check whether the display mode should be included in enumeration
//
static IntBool modeIsGood(const XRRModeInfo* mi)
{
    return (mi->modeFlags & RR_Interlace) == 0;
}

// Calculates the refresh rate, in Hz, from the specified RandR mode info
//
static int calculateRefreshRate(const XRRModeInfo* mi)
{
    if (mi->hTotal && mi->vTotal)
        return (int) round((double) mi->dotClock / ((double) mi->hTotal * (double) mi->vTotal));
    else
        return 0;
}

// Returns the mode info for a RandR mode XID
//
static const XRRModeInfo* getModeInfo(const XRRScreenResources* sr, RRMode id)
{
    for (int i = 0;  i < sr->nmode;  i++)
    {
        if (sr->modes[i].id == id)
            return sr->modes + i;
    }

    return NULL;
}

// Convert RandR mode info to GLFW video mode
//
static VideoMode vidmodeFromModeInfo(const XRRModeInfo* mi,
                                       const XRRCrtcInfo* ci)
{
    VideoMode mode;

    if (ci->rotation == RR_Rotate_90 || ci->rotation == RR_Rotate_270)
    {
        mode.width  = mi->height;
        mode.height = mi->width;
    }
    else
    {
        mode.width  = mi->width;
        mode.height = mi->height;
    }

    mode.refreshRate = calculateRefreshRate(mi);

    _glfwSplitBPP(DefaultDepth(_glfw.x11.display, _glfw.x11.screen),
                  &mode.redBits, &mode.greenBits, &mode.blueBits);

    return mode;
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Poll for changes in the set of connected monitors
//
void _glfwPollMonitorsX11(void)
{
    if (_glfw.x11.randr.available && !_glfw.x11.randr.monitorBroken)
    {
        int disconnectedCount, screenCount = 0;
        plafMonitor** disconnected = NULL;
        XineramaScreenInfo* screens = NULL;
        XRRScreenResources* sr = _glfw.x11.randr.GetScreenResourcesCurrent(_glfw.x11.display, _glfw.x11.root);
        RROutput primary = _glfw.x11.randr.GetOutputPrimary(_glfw.x11.display, _glfw.x11.root);

        if (_glfw.x11.xinerama.available)
            screens = _glfw.x11.xinerama.QueryScreens(_glfw.x11.display, &screenCount);

        disconnectedCount = _glfw.monitorCount;
        if (disconnectedCount)
        {
            disconnected = _glfw_calloc(_glfw.monitorCount, sizeof(plafMonitor*));
            memcpy(disconnected,
                   _glfw.monitors,
                   _glfw.monitorCount * sizeof(plafMonitor*));
        }

        for (int i = 0;  i < sr->noutput;  i++)
        {
            int j, type, widthMM, heightMM;

            XRROutputInfo* oi = _glfw.x11.randr.GetOutputInfo(_glfw.x11.display, sr, sr->outputs[i]);
            if (oi->connection != RR_Connected || oi->crtc == None)
            {
                _glfw.x11.randr.FreeOutputInfo(oi);
                continue;
            }

            for (j = 0;  j < disconnectedCount;  j++)
            {
                if (disconnected[j] &&
                    disconnected[j]->x11Output == sr->outputs[i])
                {
                    disconnected[j] = NULL;
                    break;
                }
            }

            if (j < disconnectedCount)
            {
                _glfw.x11.randr.FreeOutputInfo(oi);
                continue;
            }

            XRRCrtcInfo* ci = _glfw.x11.randr.GetCrtcInfo(_glfw.x11.display, sr, oi->crtc);
            if (ci->rotation == RR_Rotate_90 || ci->rotation == RR_Rotate_270)
            {
                widthMM  = oi->mm_height;
                heightMM = oi->mm_width;
            }
            else
            {
                widthMM  = oi->mm_width;
                heightMM = oi->mm_height;
            }

            if (widthMM <= 0 || heightMM <= 0)
            {
                // HACK: If RandR does not provide a physical size, assume the
                //       X11 default 96 DPI and calculate from the CRTC viewport
                // NOTE: These members are affected by rotation, unlike the mode
                //       info and output info members
                widthMM  = (int) (ci->width * 25.4f / 96.f);
                heightMM = (int) (ci->height * 25.4f / 96.f);
            }

            plafMonitor* monitor = _glfwAllocMonitor(oi->name, widthMM, heightMM);
            monitor->x11Output = sr->outputs[i];
            monitor->x11Crtc   = oi->crtc;

            for (j = 0;  j < screenCount;  j++)
            {
                if (screens[j].x_org == ci->x &&
                    screens[j].y_org == ci->y &&
                    screens[j].width == ci->width &&
                    screens[j].height == ci->height)
                {
                    monitor->x11Index = j;
                    break;
                }
            }

            if (monitor->x11Output == primary)
                type = _GLFW_INSERT_FIRST;
            else
                type = _GLFW_INSERT_LAST;

            _glfwInputMonitor(monitor, CONNECTED, type);

            _glfw.x11.randr.FreeOutputInfo(oi);
            _glfw.x11.randr.FreeCrtcInfo(ci);
        }

        _glfw.x11.randr.FreeScreenResources(sr);

        if (screens)
            _glfw.x11.xlib.Free(screens);

        for (int i = 0;  i < disconnectedCount;  i++)
        {
            if (disconnected[i])
                _glfwInputMonitor(disconnected[i], DISCONNECTED, 0);
        }

        _glfw_free(disconnected);
    }
    else
    {
        const int widthMM = DisplayWidthMM(_glfw.x11.display, _glfw.x11.screen);
        const int heightMM = DisplayHeightMM(_glfw.x11.display, _glfw.x11.screen);

        _glfwInputMonitor(_glfwAllocMonitor("Display", widthMM, heightMM),
                          CONNECTED,
                          _GLFW_INSERT_FIRST);
    }
}

// Set the current video mode for the specified monitor
//
void _glfwSetVideoModeX11(plafMonitor* monitor, const VideoMode* desired)
{
    if (_glfw.x11.randr.available && !_glfw.x11.randr.monitorBroken)
    {
        VideoMode current;
        RRMode native = None;

        const VideoMode* best = _glfwChooseVideoMode(monitor, desired);
        _glfwGetVideoModeX11(monitor, &current);
        if (_glfwCompareVideoModes(&current, best) == 0)
            return;

        XRRScreenResources* sr = _glfw.x11.randr.GetScreenResourcesCurrent(_glfw.x11.display, _glfw.x11.root);
        XRRCrtcInfo* ci = _glfw.x11.randr.GetCrtcInfo(_glfw.x11.display, sr, monitor->x11Crtc);
        XRROutputInfo* oi = _glfw.x11.randr.GetOutputInfo(_glfw.x11.display, sr, monitor->x11Output);

        for (int i = 0;  i < oi->nmode;  i++)
        {
            const XRRModeInfo* mi = getModeInfo(sr, oi->modes[i]);
            if (!modeIsGood(mi))
                continue;

            const VideoMode mode = vidmodeFromModeInfo(mi, ci);
            if (_glfwCompareVideoModes(best, &mode) == 0)
            {
                native = mi->id;
                break;
            }
        }

        if (native)
        {
            if (monitor->x11OldMode == None)
                monitor->x11OldMode = ci->mode;

            _glfw.x11.randr.SetCrtcConfig(_glfw.x11.display,
                             sr, monitor->x11Crtc,
                             CurrentTime,
                             ci->x, ci->y,
                             native,
                             ci->rotation,
                             ci->outputs,
                             ci->noutput);
        }

        _glfw.x11.randr.FreeOutputInfo(oi);
        _glfw.x11.randr.FreeCrtcInfo(ci);
        _glfw.x11.randr.FreeScreenResources(sr);
    }
}

// Restore the saved (original) video mode for the specified monitor
//
void _glfwRestoreVideoModeX11(plafMonitor* monitor)
{
    if (_glfw.x11.randr.available && !_glfw.x11.randr.monitorBroken)
    {
        if (monitor->x11OldMode == None)
            return;

        XRRScreenResources* sr = _glfw.x11.randr.GetScreenResourcesCurrent(_glfw.x11.display, _glfw.x11.root);
        XRRCrtcInfo* ci = _glfw.x11.randr.GetCrtcInfo(_glfw.x11.display, sr, monitor->x11Crtc);

        _glfw.x11.randr.SetCrtcConfig(_glfw.x11.display,
                         sr, monitor->x11Crtc,
                         CurrentTime,
                         ci->x, ci->y,
                         monitor->x11OldMode,
                         ci->rotation,
                         ci->outputs,
                         ci->noutput);

        _glfw.x11.randr.FreeCrtcInfo(ci);
        _glfw.x11.randr.FreeScreenResources(sr);

        monitor->x11OldMode = None;
    }
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW platform API                      //////
//////////////////////////////////////////////////////////////////////////

void _glfwFreeMonitorX11(plafMonitor* monitor)
{
}

void _glfwGetMonitorPosX11(plafMonitor* monitor, int* xpos, int* ypos)
{
    if (_glfw.x11.randr.available && !_glfw.x11.randr.monitorBroken) {
        XRRScreenResources* sr = _glfw.x11.randr.GetScreenResourcesCurrent(_glfw.x11.display, _glfw.x11.root);
        XRRCrtcInfo* ci = _glfw.x11.randr.GetCrtcInfo(_glfw.x11.display, sr, monitor->x11Crtc);
        if (ci) {
            *xpos = ci->x;
            *ypos = ci->y;
            _glfw.x11.randr.FreeCrtcInfo(ci);
        }
        _glfw.x11.randr.FreeScreenResources(sr);
    }
}

void _glfwGetMonitorContentScaleX11(plafMonitor* monitor,
                                    float* xscale, float* yscale)
{
    if (xscale)
        *xscale = _glfw.x11.contentScaleX;
    if (yscale)
        *yscale = _glfw.x11.contentScaleY;
}

void _glfwGetMonitorWorkareaX11(plafMonitor* monitor,
                                int* xpos, int* ypos,
                                int* width, int* height)
{
    int areaX = 0, areaY = 0, areaWidth = 0, areaHeight = 0;

    if (_glfw.x11.randr.available && !_glfw.x11.randr.monitorBroken)
    {
        XRRScreenResources* sr = _glfw.x11.randr.GetScreenResourcesCurrent(_glfw.x11.display, _glfw.x11.root);
        XRRCrtcInfo* ci = _glfw.x11.randr.GetCrtcInfo(_glfw.x11.display, sr, monitor->x11Crtc);

        areaX = ci->x;
        areaY = ci->y;

        const XRRModeInfo* mi = getModeInfo(sr, ci->mode);

        if (ci->rotation == RR_Rotate_90 || ci->rotation == RR_Rotate_270)
        {
            areaWidth  = mi->height;
            areaHeight = mi->width;
        }
        else
        {
            areaWidth  = mi->width;
            areaHeight = mi->height;
        }

        _glfw.x11.randr.FreeCrtcInfo(ci);
        _glfw.x11.randr.FreeScreenResources(sr);
    }
    else
    {
        areaWidth  = DisplayWidth(_glfw.x11.display, _glfw.x11.screen);
        areaHeight = DisplayHeight(_glfw.x11.display, _glfw.x11.screen);
    }

    if (_glfw.x11.NET_WORKAREA && _glfw.x11.NET_CURRENT_DESKTOP)
    {
        Atom* extents = NULL;
        Atom* desktop = NULL;
        const unsigned long extentCount =
            _glfwGetWindowPropertyX11(_glfw.x11.root,
                                      _glfw.x11.NET_WORKAREA,
                                      XA_CARDINAL,
                                      (unsigned char**) &extents);

        if (_glfwGetWindowPropertyX11(_glfw.x11.root,
                                      _glfw.x11.NET_CURRENT_DESKTOP,
                                      XA_CARDINAL,
                                      (unsigned char**) &desktop) > 0)
        {
            if (extentCount >= 4 && *desktop < extentCount / 4)
            {
                const int globalX = extents[*desktop * 4 + 0];
                const int globalY = extents[*desktop * 4 + 1];
                const int globalWidth  = extents[*desktop * 4 + 2];
                const int globalHeight = extents[*desktop * 4 + 3];

                if (areaX < globalX)
                {
                    areaWidth -= globalX - areaX;
                    areaX = globalX;
                }

                if (areaY < globalY)
                {
                    areaHeight -= globalY - areaY;
                    areaY = globalY;
                }

                if (areaX + areaWidth > globalX + globalWidth)
                    areaWidth = globalX - areaX + globalWidth;
                if (areaY + areaHeight > globalY + globalHeight)
                    areaHeight = globalY - areaY + globalHeight;
            }
        }

        if (extents)
            _glfw.x11.xlib.Free(extents);
        if (desktop)
            _glfw.x11.xlib.Free(desktop);
    }

    if (xpos)
        *xpos = areaX;
    if (ypos)
        *ypos = areaY;
    if (width)
        *width = areaWidth;
    if (height)
        *height = areaHeight;
}

VideoMode* _glfwGetVideoModesX11(plafMonitor* monitor, int* count)
{
    VideoMode* result;

    *count = 0;

    if (_glfw.x11.randr.available && !_glfw.x11.randr.monitorBroken)
    {
        XRRScreenResources* sr = _glfw.x11.randr.GetScreenResourcesCurrent(_glfw.x11.display, _glfw.x11.root);
        XRRCrtcInfo* ci = _glfw.x11.randr.GetCrtcInfo(_glfw.x11.display, sr, monitor->x11Crtc);
        XRROutputInfo* oi = _glfw.x11.randr.GetOutputInfo(_glfw.x11.display, sr, monitor->x11Output);

        result = _glfw_calloc(oi->nmode, sizeof(VideoMode));

        for (int i = 0;  i < oi->nmode;  i++)
        {
            const XRRModeInfo* mi = getModeInfo(sr, oi->modes[i]);
            if (!modeIsGood(mi))
                continue;

            const VideoMode mode = vidmodeFromModeInfo(mi, ci);
            int j;

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

        _glfw.x11.randr.FreeOutputInfo(oi);
        _glfw.x11.randr.FreeCrtcInfo(ci);
        _glfw.x11.randr.FreeScreenResources(sr);
    }
    else
    {
        *count = 1;
        result = _glfw_calloc(1, sizeof(VideoMode));
        _glfwGetVideoModeX11(monitor, result);
    }

    return result;
}

IntBool _glfwGetVideoModeX11(plafMonitor* monitor, VideoMode* mode)
{
    if (_glfw.x11.randr.available && !_glfw.x11.randr.monitorBroken)
    {
        XRRScreenResources* sr = _glfw.x11.randr.GetScreenResourcesCurrent(_glfw.x11.display, _glfw.x11.root);
        const XRRModeInfo* mi = NULL;

        XRRCrtcInfo* ci = _glfw.x11.randr.GetCrtcInfo(_glfw.x11.display, sr, monitor->x11Crtc);
        if (ci)
        {
            mi = getModeInfo(sr, ci->mode);
            if (mi)
                *mode = vidmodeFromModeInfo(mi, ci);

            _glfw.x11.randr.FreeCrtcInfo(ci);
        }

        _glfw.x11.randr.FreeScreenResources(sr);

        if (!mi)
        {
            _glfwInputError(ERR_PLATFORM_ERROR, "X11: Failed to query video mode");
            return false;
        }
    }
    else
    {
        mode->width = DisplayWidth(_glfw.x11.display, _glfw.x11.screen);
        mode->height = DisplayHeight(_glfw.x11.display, _glfw.x11.screen);
        mode->refreshRate = 0;

        _glfwSplitBPP(DefaultDepth(_glfw.x11.display, _glfw.x11.screen),
                      &mode->redBits, &mode->greenBits, &mode->blueBits);
    }

    return true;
}

IntBool _glfwGetGammaRampX11(plafMonitor* monitor, GammaRamp* ramp)
{
    if (_glfw.x11.randr.available && !_glfw.x11.randr.gammaBroken)
    {
        const size_t size = _glfw.x11.randr.GetCrtcGammaSize(_glfw.x11.display, monitor->x11Crtc);
        XRRCrtcGamma* gamma = _glfw.x11.randr.GetCrtcGamma(_glfw.x11.display, monitor->x11Crtc);

        _glfwAllocGammaArrays(ramp, size);

        memcpy(ramp->red,   gamma->red,   size * sizeof(unsigned short));
        memcpy(ramp->green, gamma->green, size * sizeof(unsigned short));
        memcpy(ramp->blue,  gamma->blue,  size * sizeof(unsigned short));

        _glfw.x11.randr.FreeGamma(gamma);
        return true;
    }
    else if (_glfw.x11.vidmode.available)
    {
        int size;
        _glfw.x11.vidmode.GetGammaRampSize(_glfw.x11.display, _glfw.x11.screen, &size);

        _glfwAllocGammaArrays(ramp, size);

        _glfw.x11.vidmode.GetGammaRamp(_glfw.x11.display,
                                _glfw.x11.screen,
                                ramp->size, ramp->red, ramp->green, ramp->blue);
        return true;
    }
    else
    {
        _glfwInputError(ERR_PLATFORM_ERROR, "X11: Gamma ramp access not supported by server");
        return false;
    }
}

void _glfwSetGammaRampX11(plafMonitor* monitor, const GammaRamp* ramp)
{
    if (_glfw.x11.randr.available && !_glfw.x11.randr.gammaBroken)
    {
        if (_glfw.x11.randr.GetCrtcGammaSize(_glfw.x11.display, monitor->x11Crtc) != ramp->size)
        {
            _glfwInputError(ERR_PLATFORM_ERROR, "X11: Gamma ramp size must match current ramp size");
            return;
        }

        XRRCrtcGamma* gamma = _glfw.x11.randr.AllocGamma(ramp->size);

        memcpy(gamma->red,   ramp->red,   ramp->size * sizeof(unsigned short));
        memcpy(gamma->green, ramp->green, ramp->size * sizeof(unsigned short));
        memcpy(gamma->blue,  ramp->blue,  ramp->size * sizeof(unsigned short));

        _glfw.x11.randr.SetCrtcGamma(_glfw.x11.display, monitor->x11Crtc, gamma);
        _glfw.x11.randr.FreeGamma(gamma);
    }
    else if (_glfw.x11.vidmode.available)
    {
        _glfw.x11.vidmode.SetGammaRamp(_glfw.x11.display,
                                _glfw.x11.screen,
                                ramp->size,
                                (unsigned short*) ramp->red,
                                (unsigned short*) ramp->green,
                                (unsigned short*) ramp->blue);
    }
    else
    {
        _glfwInputError(ERR_PLATFORM_ERROR, "X11: Gamma ramp access not supported by server");
    }
}

#endif // __linux__
