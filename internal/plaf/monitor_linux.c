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

// Convert RandR mode info to PLAF video mode
//
static plafVideoMode vidmodeFromModeInfo(const XRRModeInfo* mi,
									   const XRRCrtcInfo* ci)
{
	plafVideoMode mode;

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

	_plafSplitBPP(DefaultDepth(_plaf.x11Display, _plaf.x11Screen), &mode.redBits, &mode.greenBits, &mode.blueBits);

	return mode;
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Poll for changes in the set of connected monitors
//
void _plafPollMonitors(void)
{
	if (_plaf.randrAvailable && !_plaf.randrMonitorBroken)
	{
		int disconnectedCount, screenCount = 0;
		plafMonitor** disconnected = NULL;
		XineramaScreenInfo* screens = NULL;
		XRRScreenResources* sr = _plaf.randrGetScreenResourcesCurrent(_plaf.x11Display, _plaf.x11Root);
		RROutput primary = _plaf.randrGetOutputPrimary(_plaf.x11Display, _plaf.x11Root);

		if (_plaf.xineramaAvailable)
			screens = _plaf.xineramaQueryScreens(_plaf.x11Display, &screenCount);

		disconnectedCount = _plaf.monitorCount;
		if (disconnectedCount)
		{
			disconnected = _plaf_calloc(_plaf.monitorCount, sizeof(plafMonitor*));
			memcpy(disconnected,
				   _plaf.monitors,
				   _plaf.monitorCount * sizeof(plafMonitor*));
		}

		for (int i = 0;  i < sr->noutput;  i++)
		{
			int j, type, widthMM, heightMM;

			XRROutputInfo* oi = _plaf.randrGetOutputInfo(_plaf.x11Display, sr, sr->outputs[i]);
			if (oi->connection != RR_Connected || oi->crtc == None)
			{
				_plaf.randrFreeOutputInfo(oi);
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
				_plaf.randrFreeOutputInfo(oi);
				continue;
			}

			XRRCrtcInfo* ci = _plaf.randrGetCrtcInfo(_plaf.x11Display, sr, oi->crtc);
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

			plafMonitor* monitor = _plafAllocMonitor(oi->name, widthMM, heightMM);
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
				type = MONITOR_INSERT_FIRST;
			else
				type = MONITOR_INSERT_LAST;

			_plafMonitorNotify(monitor, CONNECTED, type);

			_plaf.randrFreeOutputInfo(oi);
			_plaf.randrFreeCrtcInfo(ci);
		}

		_plaf.randrFreeScreenResources(sr);

		if (screens)
			_plaf.xlibFree(screens);

		for (int i = 0;  i < disconnectedCount;  i++)
		{
			if (disconnected[i])
				_plafMonitorNotify(disconnected[i], DISCONNECTED, 0);
		}

		_plaf_free(disconnected);
	}
	else
	{
		const int widthMM = DisplayWidthMM(_plaf.x11Display, _plaf.x11Screen);
		const int heightMM = DisplayHeightMM(_plaf.x11Display, _plaf.x11Screen);

		_plafMonitorNotify(_plafAllocMonitor("Display", widthMM, heightMM),
						  CONNECTED,
						  MONITOR_INSERT_FIRST);
	}
}

// Set the current video mode for the specified monitor
//
void _plafSetVideoMode(plafMonitor* monitor, const plafVideoMode* desired)
{
	if (_plaf.randrAvailable && !_plaf.randrMonitorBroken)
	{
		plafVideoMode current;
		RRMode native = None;

		const plafVideoMode* best = _plafChooseVideoMode(monitor, desired);
		_plafGetVideoMode(monitor, &current);
		if (_plafCompareVideoModes(&current, best) == 0)
			return;

		XRRScreenResources* sr = _plaf.randrGetScreenResourcesCurrent(_plaf.x11Display, _plaf.x11Root);
		XRRCrtcInfo* ci = _plaf.randrGetCrtcInfo(_plaf.x11Display, sr, monitor->x11Crtc);
		XRROutputInfo* oi = _plaf.randrGetOutputInfo(_plaf.x11Display, sr, monitor->x11Output);

		for (int i = 0;  i < oi->nmode;  i++)
		{
			const XRRModeInfo* mi = getModeInfo(sr, oi->modes[i]);
			if (!modeIsGood(mi))
				continue;

			const plafVideoMode mode = vidmodeFromModeInfo(mi, ci);
			if (_plafCompareVideoModes(best, &mode) == 0)
			{
				native = mi->id;
				break;
			}
		}

		if (native)
		{
			if (monitor->x11OldMode == None)
				monitor->x11OldMode = ci->mode;

			_plaf.randrSetCrtcConfig(_plaf.x11Display,
							 sr, monitor->x11Crtc,
							 CurrentTime,
							 ci->x, ci->y,
							 native,
							 ci->rotation,
							 ci->outputs,
							 ci->noutput);
		}

		_plaf.randrFreeOutputInfo(oi);
		_plaf.randrFreeCrtcInfo(ci);
		_plaf.randrFreeScreenResources(sr);
	}
}

// Restore the saved (original) video mode for the specified monitor
//
void _plafRestoreVideoMode(plafMonitor* monitor)
{
	if (_plaf.randrAvailable && !_plaf.randrMonitorBroken)
	{
		if (monitor->x11OldMode == None)
			return;

		XRRScreenResources* sr = _plaf.randrGetScreenResourcesCurrent(_plaf.x11Display, _plaf.x11Root);
		XRRCrtcInfo* ci = _plaf.randrGetCrtcInfo(_plaf.x11Display, sr, monitor->x11Crtc);

		_plaf.randrSetCrtcConfig(_plaf.x11Display,
						 sr, monitor->x11Crtc,
						 CurrentTime,
						 ci->x, ci->y,
						 monitor->x11OldMode,
						 ci->rotation,
						 ci->outputs,
						 ci->noutput);

		_plaf.randrFreeCrtcInfo(ci);
		_plaf.randrFreeScreenResources(sr);

		monitor->x11OldMode = None;
	}
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF platform API                      //////
//////////////////////////////////////////////////////////////////////////

void plafGetMonitorPos(plafMonitor* monitor, int* xpos, int* ypos)
{
	*xpos = 0;
	*ypos = 0;
	if (_plaf.randrAvailable && !_plaf.randrMonitorBroken) {
		XRRScreenResources* sr = _plaf.randrGetScreenResourcesCurrent(_plaf.x11Display, _plaf.x11Root);
		XRRCrtcInfo* ci = _plaf.randrGetCrtcInfo(_plaf.x11Display, sr, monitor->x11Crtc);
		if (ci) {
			*xpos = ci->x;
			*ypos = ci->y;
			_plaf.randrFreeCrtcInfo(ci);
		}
		_plaf.randrFreeScreenResources(sr);
	}
}

void plafGetMonitorContentScale(plafMonitor* monitor, float* xscale, float* yscale) {
	*xscale = _plaf.x11ContentScaleX;
	*yscale = _plaf.x11ContentScaleY;
}

void plafGetMonitorWorkarea(plafMonitor* monitor, int* xpos, int* ypos, int* width, int* height) {
	int areaX = 0, areaY = 0, areaWidth = 0, areaHeight = 0;

	if (_plaf.randrAvailable && !_plaf.randrMonitorBroken)
	{
		XRRScreenResources* sr = _plaf.randrGetScreenResourcesCurrent(_plaf.x11Display, _plaf.x11Root);
		XRRCrtcInfo* ci = _plaf.randrGetCrtcInfo(_plaf.x11Display, sr, monitor->x11Crtc);

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

		_plaf.randrFreeCrtcInfo(ci);
		_plaf.randrFreeScreenResources(sr);
	}
	else
	{
		areaWidth  = DisplayWidth(_plaf.x11Display, _plaf.x11Screen);
		areaHeight = DisplayHeight(_plaf.x11Display, _plaf.x11Screen);
	}

	if (_plaf.x11NET_WORKAREA && _plaf.x11NET_CURRENT_DESKTOP)
	{
		Atom* extents = NULL;
		Atom* desktop = NULL;
		const unsigned long extentCount =
			_plafGetWindowProperty(_plaf.x11Root,
									  _plaf.x11NET_WORKAREA,
									  XA_CARDINAL,
									  (unsigned char**) &extents);

		if (_plafGetWindowProperty(_plaf.x11Root,
									  _plaf.x11NET_CURRENT_DESKTOP,
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
			_plaf.xlibFree(extents);
		if (desktop)
			_plaf.xlibFree(desktop);
	}

	*xpos = areaX;
	*ypos = areaY;
	*width = areaWidth;
	*height = areaHeight;
}

plafVideoMode* _plafGetVideoModes(plafMonitor* monitor, int* count)
{
	plafVideoMode* result;

	*count = 0;

	if (_plaf.randrAvailable && !_plaf.randrMonitorBroken)
	{
		XRRScreenResources* sr = _plaf.randrGetScreenResourcesCurrent(_plaf.x11Display, _plaf.x11Root);
		XRRCrtcInfo* ci = _plaf.randrGetCrtcInfo(_plaf.x11Display, sr, monitor->x11Crtc);
		XRROutputInfo* oi = _plaf.randrGetOutputInfo(_plaf.x11Display, sr, monitor->x11Output);

		result = _plaf_calloc(oi->nmode, sizeof(plafVideoMode));

		for (int i = 0;  i < oi->nmode;  i++)
		{
			const XRRModeInfo* mi = getModeInfo(sr, oi->modes[i]);
			if (!modeIsGood(mi))
				continue;

			const plafVideoMode mode = vidmodeFromModeInfo(mi, ci);
			int j;

			for (j = 0;  j < *count;  j++)
			{
				if (_plafCompareVideoModes(result + j, &mode) == 0)
					break;
			}

			// Skip duplicate modes
			if (j < *count)
				continue;

			(*count)++;
			result[*count - 1] = mode;
		}

		_plaf.randrFreeOutputInfo(oi);
		_plaf.randrFreeCrtcInfo(ci);
		_plaf.randrFreeScreenResources(sr);
	}
	else
	{
		*count = 1;
		result = _plaf_calloc(1, sizeof(plafVideoMode));
		_plafGetVideoMode(monitor, result);
	}

	return result;
}

IntBool _plafGetVideoMode(plafMonitor* monitor, plafVideoMode* mode) {
	if (_plaf.randrAvailable && !_plaf.randrMonitorBroken)
	{
		XRRScreenResources* sr = _plaf.randrGetScreenResourcesCurrent(_plaf.x11Display, _plaf.x11Root);
		const XRRModeInfo* mi = NULL;

		XRRCrtcInfo* ci = _plaf.randrGetCrtcInfo(_plaf.x11Display, sr, monitor->x11Crtc);
		if (ci)
		{
			mi = getModeInfo(sr, ci->mode);
			if (mi)
				*mode = vidmodeFromModeInfo(mi, ci);

			_plaf.randrFreeCrtcInfo(ci);
		}

		_plaf.randrFreeScreenResources(sr);

		if (!mi)
		{
			_plafInputError("X11: Failed to query video mode");
			return false;
		}
	}
	else
	{
		mode->width = DisplayWidth(_plaf.x11Display, _plaf.x11Screen);
		mode->height = DisplayHeight(_plaf.x11Display, _plaf.x11Screen);
		mode->refreshRate = 0;

		_plafSplitBPP(DefaultDepth(_plaf.x11Display, _plaf.x11Screen), &mode->redBits, &mode->greenBits, &mode->blueBits);
	}

	return true;
}

IntBool _plafGetGammaRamp(plafMonitor* monitor, plafGammaRamp* ramp) {
	if (_plaf.randrAvailable && !_plaf.randrGammaBroken)
	{
		const size_t size = _plaf.randrGetCrtcGammaSize(_plaf.x11Display, monitor->x11Crtc);
		XRRCrtcGamma* gamma = _plaf.randrGetCrtcGamma(_plaf.x11Display, monitor->x11Crtc);

		_plafAllocGammaArrays(ramp, size);

		memcpy(ramp->red,   gamma->red,   size * sizeof(unsigned short));
		memcpy(ramp->green, gamma->green, size * sizeof(unsigned short));
		memcpy(ramp->blue,  gamma->blue,  size * sizeof(unsigned short));

		_plaf.randrFreeGamma(gamma);
		return true;
	}
	else if (_plaf.xvidmodeAvailable)
	{
		int size;
		_plaf.xvidmodeGetGammaRampSize(_plaf.x11Display, _plaf.x11Screen, &size);

		_plafAllocGammaArrays(ramp, size);

		_plaf.xvidmodeGetGammaRamp(_plaf.x11Display,
								_plaf.x11Screen,
								ramp->size, ramp->red, ramp->green, ramp->blue);
		return true;
	}
	else
	{
		_plafInputError("X11: Gamma ramp access not supported by server");
		return false;
	}
}

void _plafSetGammaRamp(plafMonitor* monitor, const plafGammaRamp* ramp) {
	if (_plaf.randrAvailable && !_plaf.randrGammaBroken)
	{
		if (_plaf.randrGetCrtcGammaSize(_plaf.x11Display, monitor->x11Crtc) != ramp->size)
		{
			_plafInputError("X11: Gamma ramp size must match current ramp size");
			return;
		}

		XRRCrtcGamma* gamma = _plaf.randrAllocGamma(ramp->size);

		memcpy(gamma->red,   ramp->red,   ramp->size * sizeof(unsigned short));
		memcpy(gamma->green, ramp->green, ramp->size * sizeof(unsigned short));
		memcpy(gamma->blue,  ramp->blue,  ramp->size * sizeof(unsigned short));

		_plaf.randrSetCrtcGamma(_plaf.x11Display, monitor->x11Crtc, gamma);
		_plaf.randrFreeGamma(gamma);
	}
	else if (_plaf.xvidmodeAvailable)
	{
		_plaf.xvidmodeSetGammaRamp(_plaf.x11Display,
								_plaf.x11Screen,
								ramp->size,
								(unsigned short*) ramp->red,
								(unsigned short*) ramp->green,
								(unsigned short*) ramp->blue);
	}
	else
	{
		_plafInputError("X11: Gamma ramp access not supported by server");
	}
}

#endif // __linux__
