#include "platform.h"

#include <math.h>
#include <limits.h>


// Lexically compare video modes, used by qsort
//
static int compareVideoModes(const void* fp, const void* sp)
{
    const plafVideoMode* fm = fp;
    const plafVideoMode* sm = sp;
    const int fbpp = fm->redBits + fm->greenBits + fm->blueBits;
    const int sbpp = sm->redBits + sm->greenBits + sm->blueBits;
    const int farea = fm->width * fm->height;
    const int sarea = sm->width * sm->height;

    // First sort on color bits per pixel
    if (fbpp != sbpp)
        return fbpp - sbpp;

    // Then sort on screen area
    if (farea != sarea)
        return farea - sarea;

    // Then sort on width
    if (fm->width != sm->width)
        return fm->width - sm->width;

    // Lastly sort on refresh rate
    return fm->refreshRate - sm->refreshRate;
}

// Retrieves the available modes for the specified monitor
//
static IntBool refreshVideoModes(plafMonitor* monitor)
{
    int modeCount;
    plafVideoMode* modes;

    if (monitor->modes)
        return true;

    modes = _plafGetVideoModes(monitor, &modeCount);
    if (!modes)
        return false;

    qsort(modes, modeCount, sizeof(plafVideoMode), compareVideoModes);

    _plaf_free(monitor->modes);
    monitor->modes = modes;
    monitor->modeCount = modeCount;

    return true;
}


//////////////////////////////////////////////////////////////////////////
//////                         PLAF event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code of a monitor connection or disconnection
//
void _plafMonitorNotify(plafMonitor* monitor, int action, int placement)
{
    if (action == CONNECTED)
    {
        _plaf.monitorCount++;
        _plaf.monitors =
            _plaf_realloc(_plaf.monitors,
                          sizeof(plafMonitor*) * _plaf.monitorCount);

        if (placement == MONITOR_INSERT_FIRST)
        {
            memmove(_plaf.monitors + 1,
                    _plaf.monitors,
                    ((size_t) _plaf.monitorCount - 1) * sizeof(plafMonitor*));
            _plaf.monitors[0] = monitor;
        }
        else
            _plaf.monitors[_plaf.monitorCount - 1] = monitor;
    }
    else if (action == DISCONNECTED)
    {
        int i;
        plafWindow* window;

        for (window = _plaf.windowListHead;  window;  window = window->next)
        {
            if (window->monitor == monitor)
            {
                int width, height, xoff, yoff;
                _plafGetWindowSize(window, &width, &height);
                _plafSetWindowMonitor(window, NULL, 0, 0, width, height, 0);
                _plafGetWindowFrameSize(window, &xoff, &yoff, NULL, NULL);
                _plafSetWindowPos(window, xoff, yoff);
            }
        }

        for (i = 0;  i < _plaf.monitorCount;  i++)
        {
            if (_plaf.monitors[i] == monitor)
            {
                _plaf.monitorCount--;
                memmove(_plaf.monitors + i,
                        _plaf.monitors + i + 1,
                        ((size_t) _plaf.monitorCount - i) * sizeof(plafMonitor*));
                break;
            }
        }
    }

    if (_plaf.monitorCallback)
        _plaf.monitorCallback(monitor, action);

    if (action == DISCONNECTED)
        _plafFreeMonitor(monitor);
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Allocates and returns a monitor object with the specified name and dimensions
//
plafMonitor* _plafAllocMonitor(const char* name, int widthMM, int heightMM)
{
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
//
void _plafAllocGammaArrays(plafGammaRamp* ramp, unsigned int size)
{
    ramp->red = _plaf_calloc(size, sizeof(unsigned short));
    ramp->green = _plaf_calloc(size, sizeof(unsigned short));
    ramp->blue = _plaf_calloc(size, sizeof(unsigned short));
    ramp->size = size;
}

// Frees the red, green and blue value arrays and clears the struct
//
void _plafFreeGammaArrays(plafGammaRamp* ramp)
{
    _plaf_free(ramp->red);
    _plaf_free(ramp->green);
    _plaf_free(ramp->blue);

    memset(ramp, 0, sizeof(plafGammaRamp));
}

// Chooses the video mode most closely matching the desired one
//
const plafVideoMode* _plafChooseVideoMode(plafMonitor* monitor,
                                        const plafVideoMode* desired)
{
    int i;
    unsigned int sizeDiff, leastSizeDiff = UINT_MAX;
    unsigned int rateDiff, leastRateDiff = UINT_MAX;
    unsigned int colorDiff, leastColorDiff = UINT_MAX;
    const plafVideoMode* current;
    const plafVideoMode* closest = NULL;

    if (!refreshVideoModes(monitor))
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

// Performs lexical comparison between two @ref plafVideoMode structures
//
int _plafCompareVideoModes(const plafVideoMode* fm, const plafVideoMode* sm)
{
    return compareVideoModes(fm, sm);
}

// Splits a color depth into red, green and blue bit depths
//
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

plafMonitor** plafGetMonitors(int* count)
{
    *count = _plaf.monitorCount;
    return (plafMonitor**) _plaf.monitors;
}

plafMonitor* plafGetPrimaryMonitor(void)
{
    if (!_plaf.monitorCount)
        return NULL;
    return _plaf.monitors[0];
}

void plafGetMonitorPhysicalSize(plafMonitor* monitor, int* widthMM, int* heightMM)
{
    if (widthMM)
        *widthMM = 0;
    if (heightMM)
        *heightMM = 0;

    if (widthMM)
        *widthMM = monitor->widthMM;
    if (heightMM)
        *heightMM = monitor->heightMM;
}

const char* plafGetMonitorName(plafMonitor* monitor)
{
    return monitor->name;
}

monitorFunc plafSetMonitorCallback(monitorFunc cbfun)
{
    SWAP(monitorFunc, _plaf.monitorCallback, cbfun);
    return cbfun;
}

const plafVideoMode* plafGetVideoModes(plafMonitor* monitor, int* count)
{
    if (!refreshVideoModes(monitor)) {
	    *count = 0;
        return NULL;
	}
    *count = monitor->modeCount;
    return monitor->modes;
}

const plafVideoMode* plafGetVideoMode(plafMonitor* monitor) {
    if (!_plafGetVideoMode(monitor, &monitor->currentMode)) {
        return NULL;
	}
    return &monitor->currentMode;
}

void plafSetGamma(plafMonitor* monitor, float gamma)
{
    unsigned int i;
    unsigned short* values;
    plafGammaRamp ramp;
    const plafGammaRamp* original;

    if (gamma != gamma || gamma <= 0.f || gamma > FLT_MAX)
    {
        _plafInputError("Invalid gamma value %f", gamma);
        return;
    }

    original = plafGetGammaRamp(monitor);
    if (!original) {
        return;
	}

    values = _plaf_calloc(original->size, sizeof(unsigned short));

    for (i = 0;  i < original->size;  i++)
    {
        float value;

        // Calculate intensity
        value = i / (float) (original->size - 1);
        // Apply gamma curve
        value = powf(value, 1.f / gamma) * 65535.f + 0.5f;
        // Clamp to value range
        value = fminf(value, 65535.f);

        values[i] = (unsigned short) value;
    }

    ramp.red = values;
    ramp.green = values;
    ramp.blue = values;
    ramp.size = original->size;

    plafSetGammaRamp(monitor, &ramp);
    _plaf_free(values);
}

const plafGammaRamp* plafGetGammaRamp(plafMonitor* monitor) {
    _plafFreeGammaArrays(&monitor->currentRamp);
    if (!_plafGetGammaRamp(monitor, &monitor->currentRamp)) {
        return NULL;
	}
    return &monitor->currentRamp;
}

void plafSetGammaRamp(plafMonitor* monitor, const plafGammaRamp* ramp) {
    if (ramp->size <= 0) {
        _plafInputError("Invalid gamma ramp size %i", ramp->size);
        return;
    }
    if (!monitor->originalRamp.size) {
        if (!_plafGetGammaRamp(monitor, &monitor->originalRamp))
            return;
    }
    _plafSetGammaRamp(monitor, ramp);
}
