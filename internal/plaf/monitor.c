#include "platform.h"

#include <math.h>
#include <limits.h>

//////////////////////////////////////////////////////////////////////////
//////                         PLAF event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code of a monitor connection or disconnection
void _plafMonitorNotify(plafMonitor* monitor, int action, int placement) {
	if (action == CONNECTED) {
		_plaf.monitorCount++;
		_plaf.monitors = _plaf_realloc(_plaf.monitors, sizeof(plafMonitor*) * _plaf.monitorCount);
		if (placement == MONITOR_INSERT_FIRST) {
			memmove(_plaf.monitors + 1, _plaf.monitors, ((size_t) _plaf.monitorCount - 1) * sizeof(plafMonitor*));
			_plaf.monitors[0] = monitor;
		} else {
			_plaf.monitors[_plaf.monitorCount - 1] = monitor;
		}
		goMonitorCallback(monitor, true);
	} else if (action == DISCONNECTED) {
		for (plafWindow* window = _plaf.windowListHead;  window;  window = window->next) {
			if (window->monitor == monitor) {
				int width;
				int height;
				int xOffset;
				int yOffset;
				_plafGetWindowSize(window, &width, &height);
				_plafSetWindowMonitor(window, NULL, 0, 0, width, height, 0);
				_plafGetWindowFrameSize(window, &xOffset, &yOffset, NULL, NULL);
				_plafSetWindowPos(window, xOffset, yOffset);
			}
		}
		for (int i = 0;  i < _plaf.monitorCount;  i++) {
			if (_plaf.monitors[i] == monitor) {
				_plaf.monitorCount--;
				memmove(_plaf.monitors + i, _plaf.monitors + i + 1,
					((size_t) _plaf.monitorCount - i) * sizeof(plafMonitor*));
				break;
			}
		}
		goMonitorCallback(monitor, false);
		_plafFreeMonitor(monitor);
	}
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Allocates and returns a monitor object with the specified name and dimensions
plafMonitor* _plafAllocMonitor(const char* name, int widthMM, int heightMM) {
	plafMonitor* monitor = _plaf_calloc(1, sizeof(plafMonitor));
	monitor->widthMM = widthMM;
	monitor->heightMM = heightMM;
	strncpy(monitor->name, name, sizeof(monitor->name) - 1);
	return monitor;
}

// Frees a monitor object and any data associated with it
void _plafFreeMonitor(plafMonitor* monitor) {
	if (monitor != NULL) {
		_plafFreeGammaArrays(&monitor->originalRamp);
		_plafFreeGammaArrays(&monitor->currentRamp);
		_plaf_free(monitor->modes);
		_plaf_free(monitor);
	}
}

// Allocates red, green and blue value arrays of the specified size
void _plafAllocGammaArrays(plafGammaRamp* ramp, unsigned int size) {
	ramp->red = _plaf_calloc(size, sizeof(unsigned short));
	ramp->green = _plaf_calloc(size, sizeof(unsigned short));
	ramp->blue = _plaf_calloc(size, sizeof(unsigned short));
	ramp->size = size;
}

// Frees the red, green and blue value arrays and clears the struct
void _plafFreeGammaArrays(plafGammaRamp* ramp) {
	_plaf_free(ramp->red);
	_plaf_free(ramp->green);
	_plaf_free(ramp->blue);
	memset(ramp, 0, sizeof(plafGammaRamp));
}

// Chooses the video mode most closely matching the desired one
const plafVideoMode* _plafChooseVideoMode(plafMonitor* monitor, const plafVideoMode* desired) {
	int i;
	unsigned int sizeDiff, leastSizeDiff = UINT_MAX;
	unsigned int rateDiff, leastRateDiff = UINT_MAX;
	unsigned int colorDiff, leastColorDiff = UINT_MAX;
	const plafVideoMode* current;
	const plafVideoMode* closest = NULL;

	if (!plafRefreshVideoModes(monitor))
		return NULL;

	for (i = 0;  i < monitor->modeCount;  i++)
	{
		current = monitor->modes + i;

		colorDiff = 0;

		if (desired->redBits != DONT_CARE)
			colorDiff += abs(current->redBits - desired->redBits);
		if (desired->greenBits != DONT_CARE)
			colorDiff += abs(current->greenBits - desired->greenBits);
		if (desired->blueBits != DONT_CARE)
			colorDiff += abs(current->blueBits - desired->blueBits);

		sizeDiff = abs((current->width - desired->width) *
					   (current->width - desired->width) +
					   (current->height - desired->height) *
					   (current->height - desired->height));

		if (desired->refreshRate != DONT_CARE)
			rateDiff = abs(current->refreshRate - desired->refreshRate);
		else
			rateDiff = UINT_MAX - current->refreshRate;

		if ((colorDiff < leastColorDiff) ||
			(colorDiff == leastColorDiff && sizeDiff < leastSizeDiff) ||
			(colorDiff == leastColorDiff && sizeDiff == leastSizeDiff && rateDiff < leastRateDiff))
		{
			closest = current;
			leastSizeDiff = sizeDiff;
			leastRateDiff = rateDiff;
			leastColorDiff = colorDiff;
		}
	}

	return closest;
}

// Performs lexical comparison between two plafVideoMode structures
int _plafCompareVideoModes(const plafVideoMode* fm, const plafVideoMode* sm) {
	// First sort on color bits per pixel
	const int fbpp = fm->redBits + fm->greenBits + fm->blueBits;
	const int sbpp = sm->redBits + sm->greenBits + sm->blueBits;
	if (fbpp != sbpp) {
		return fbpp - sbpp;
	}
	// Then sort on screen area
	const int farea = fm->width * fm->height;
	const int sarea = sm->width * sm->height;
	if (farea != sarea) {
		return farea - sarea;
	}
	// Then sort on width
	if (fm->width != sm->width) {
		return fm->width - sm->width;
	}
	// Lastly sort on refresh rate
	return fm->refreshRate - sm->refreshRate;
}

// Splits a color depth into red, green and blue bit depths
void _plafSplitBPP(int bpp, int* red, int* green, int* blue)
{
	int delta;

	// We assume that by 32 the user really meant 24
	if (bpp == 32)
		bpp = 24;

	// Convert "bits per pixel" to red, green & blue sizes

	*red = *green = *blue = bpp / 3;
	delta = bpp - (*red * 3);
	if (delta >= 1)
		*green = *green + 1;

	if (delta == 2)
		*red = *red + 1;
}


//////////////////////////////////////////////////////////////////////////
//////                        PLAF public API                       //////
//////////////////////////////////////////////////////////////////////////

bool plafRefreshVideoModes(plafMonitor* monitor) {
	if (monitor->modes) {
		return true;
	}
	int modeCount;
	monitor->modes = _plafGetVideoModes(monitor, &modeCount);
	if (!monitor->modes) {
		return false;
	}
	qsort(monitor->modes, modeCount, sizeof(plafVideoMode), (int (*)(const void *,const void *))_plafCompareVideoModes);
	monitor->modeCount = modeCount;
	return true;
}

const plafVideoMode* plafGetVideoMode(plafMonitor* monitor) {
	if (!_plafGetVideoMode(monitor, &monitor->currentMode)) {
		return NULL;
	}
	return &monitor->currentMode;
}

const plafGammaRamp* plafGetGammaRamp(plafMonitor* monitor) {
	_plafFreeGammaArrays(&monitor->currentRamp);
	if (!_plafGetGammaRamp(monitor, &monitor->currentRamp)) {
		return NULL;
	}
	return &monitor->currentRamp;
}

void plafSetGammaRamp(plafMonitor* monitor, const plafGammaRamp* ramp) {
	if (!monitor->originalRamp.size) {
		if (!_plafGetGammaRamp(monitor, &monitor->originalRamp)) {
			return;
		}
	}
	_plafSetGammaRamp(monitor, ramp);
}
