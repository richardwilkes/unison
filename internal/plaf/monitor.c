#include "platform.h"

#include <math.h>
#include <float.h>
#include <string.h>
#include <limits.h>


// Lexically compare video modes, used by qsort
//
static int compareVideoModes(const void* fp, const void* sp)
{
    const VideoMode* fm = fp;
    const VideoMode* sm = sp;
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
static IntBool refreshVideoModes(_GLFWmonitor* monitor)
{
    int modeCount;
    VideoMode* modes;

    if (monitor->modes)
        return true;

    modes = _glfw.platform.getVideoModes(monitor, &modeCount);
    if (!modes)
        return false;

    qsort(modes, modeCount, sizeof(VideoMode), compareVideoModes);

    _glfw_free(monitor->modes);
    monitor->modes = modes;
    monitor->modeCount = modeCount;

    return true;
}


//////////////////////////////////////////////////////////////////////////
//////                         GLFW event API                       //////
//////////////////////////////////////////////////////////////////////////

// Notifies shared code of a monitor connection or disconnection
//
void _glfwInputMonitor(_GLFWmonitor* monitor, int action, int placement)
{
    if (action == CONNECTED)
    {
        _glfw.monitorCount++;
        _glfw.monitors =
            _glfw_realloc(_glfw.monitors,
                          sizeof(_GLFWmonitor*) * _glfw.monitorCount);

        if (placement == _GLFW_INSERT_FIRST)
        {
            memmove(_glfw.monitors + 1,
                    _glfw.monitors,
                    ((size_t) _glfw.monitorCount - 1) * sizeof(_GLFWmonitor*));
            _glfw.monitors[0] = monitor;
        }
        else
            _glfw.monitors[_glfw.monitorCount - 1] = monitor;
    }
    else if (action == DISCONNECTED)
    {
        int i;
        _GLFWwindow* window;

        for (window = _glfw.windowListHead;  window;  window = window->next)
        {
            if (window->monitor == monitor)
            {
                int width, height, xoff, yoff;
                _glfw.platform.getWindowSize(window, &width, &height);
                _glfw.platform.setWindowMonitor(window, NULL, 0, 0, width, height, 0);
                _glfw.platform.getWindowFrameSize(window, &xoff, &yoff, NULL, NULL);
                _glfw.platform.setWindowPos(window, xoff, yoff);
            }
        }

        for (i = 0;  i < _glfw.monitorCount;  i++)
        {
            if (_glfw.monitors[i] == monitor)
            {
                _glfw.monitorCount--;
                memmove(_glfw.monitors + i,
                        _glfw.monitors + i + 1,
                        ((size_t) _glfw.monitorCount - i) * sizeof(_GLFWmonitor*));
                break;
            }
        }
    }

    if (_glfw.callbacks.monitor)
        _glfw.callbacks.monitor((GLFWmonitor*) monitor, action);

    if (action == DISCONNECTED)
        _glfwFreeMonitor(monitor);
}

// Notifies shared code that a full screen window has acquired or released
// a monitor
//
void _glfwInputMonitorWindow(_GLFWmonitor* monitor, _GLFWwindow* window)
{
    monitor->window = window;
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Allocates and returns a monitor object with the specified name and dimensions
//
_GLFWmonitor* _glfwAllocMonitor(const char* name, int widthMM, int heightMM)
{
    _GLFWmonitor* monitor = _glfw_calloc(1, sizeof(_GLFWmonitor));
    monitor->widthMM = widthMM;
    monitor->heightMM = heightMM;

    strncpy(monitor->name, name, sizeof(monitor->name) - 1);

    return monitor;
}

// Frees a monitor object and any data associated with it
//
void _glfwFreeMonitor(_GLFWmonitor* monitor)
{
    if (monitor == NULL)
        return;

    _glfw.platform.freeMonitor(monitor);

    _glfwFreeGammaArrays(&monitor->originalRamp);
    _glfwFreeGammaArrays(&monitor->currentRamp);

    _glfw_free(monitor->modes);
    _glfw_free(monitor);
}

// Allocates red, green and blue value arrays of the specified size
//
void _glfwAllocGammaArrays(GammaRamp* ramp, unsigned int size)
{
    ramp->red = _glfw_calloc(size, sizeof(unsigned short));
    ramp->green = _glfw_calloc(size, sizeof(unsigned short));
    ramp->blue = _glfw_calloc(size, sizeof(unsigned short));
    ramp->size = size;
}

// Frees the red, green and blue value arrays and clears the struct
//
void _glfwFreeGammaArrays(GammaRamp* ramp)
{
    _glfw_free(ramp->red);
    _glfw_free(ramp->green);
    _glfw_free(ramp->blue);

    memset(ramp, 0, sizeof(GammaRamp));
}

// Chooses the video mode most closely matching the desired one
//
const VideoMode* _glfwChooseVideoMode(_GLFWmonitor* monitor,
                                        const VideoMode* desired)
{
    int i;
    unsigned int sizeDiff, leastSizeDiff = UINT_MAX;
    unsigned int rateDiff, leastRateDiff = UINT_MAX;
    unsigned int colorDiff, leastColorDiff = UINT_MAX;
    const VideoMode* current;
    const VideoMode* closest = NULL;

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

// Performs lexical comparison between two @ref VideoMode structures
//
int _glfwCompareVideoModes(const VideoMode* fm, const VideoMode* sm)
{
    return compareVideoModes(fm, sm);
}

// Splits a color depth into red, green and blue bit depths
//
void _glfwSplitBPP(int bpp, int* red, int* green, int* blue)
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
//////                        GLFW public API                       //////
//////////////////////////////////////////////////////////////////////////

GLFWmonitor** glfwGetMonitors(int* count)
{
    *count = _glfw.monitorCount;
    return (GLFWmonitor**) _glfw.monitors;
}

GLFWmonitor* glfwGetPrimaryMonitor(void)
{
    if (!_glfw.monitorCount)
        return NULL;
    return (GLFWmonitor*) _glfw.monitors[0];
}

void glfwGetMonitorPos(GLFWmonitor* handle, int* xpos, int* ypos)
{
    if (xpos)
        *xpos = 0;
    if (ypos)
        *ypos = 0;

    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;
    _glfw.platform.getMonitorPos(monitor, xpos, ypos);
}

void glfwGetMonitorWorkarea(GLFWmonitor* handle,
                                    int* xpos, int* ypos,
                                    int* width, int* height)
{
    if (xpos)
        *xpos = 0;
    if (ypos)
        *ypos = 0;
    if (width)
        *width = 0;
    if (height)
        *height = 0;

    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;
    _glfw.platform.getMonitorWorkarea(monitor, xpos, ypos, width, height);
}

void glfwGetMonitorPhysicalSize(GLFWmonitor* handle, int* widthMM, int* heightMM)
{
    if (widthMM)
        *widthMM = 0;
    if (heightMM)
        *heightMM = 0;

    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;

    if (widthMM)
        *widthMM = monitor->widthMM;
    if (heightMM)
        *heightMM = monitor->heightMM;
}

void glfwGetMonitorContentScale(GLFWmonitor* handle,
                                        float* xscale, float* yscale)
{
    if (xscale)
        *xscale = 0.f;
    if (yscale)
        *yscale = 0.f;

    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;
    _glfw.platform.getMonitorContentScale(monitor, xscale, yscale);
}

const char* glfwGetMonitorName(GLFWmonitor* handle)
{
    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;
    return monitor->name;
}

void glfwSetMonitorUserPointer(GLFWmonitor* handle, void* pointer)
{
    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;
    monitor->userPointer = pointer;
}

void* glfwGetMonitorUserPointer(GLFWmonitor* handle)
{
    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;
    return monitor->userPointer;
}

monitorFunc glfwSetMonitorCallback(monitorFunc cbfun)
{
    SWAP(monitorFunc, _glfw.callbacks.monitor, cbfun);
    return cbfun;
}

const VideoMode* glfwGetVideoModes(GLFWmonitor* handle, int* count)
{
    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;
    if (!refreshVideoModes(monitor)) {
	    *count = 0;
        return NULL;
	}
    *count = monitor->modeCount;
    return monitor->modes;
}

const VideoMode* glfwGetVideoMode(GLFWmonitor* handle)
{
    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;
    if (!_glfw.platform.getVideoMode(monitor, &monitor->currentMode))
        return NULL;
    return &monitor->currentMode;
}

void glfwSetGamma(GLFWmonitor* handle, float gamma)
{
    unsigned int i;
    unsigned short* values;
    GammaRamp ramp;
    const GammaRamp* original;

    if (gamma != gamma || gamma <= 0.f || gamma > FLT_MAX)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid gamma value %f", gamma);
        return;
    }

    original = glfwGetGammaRamp(handle);
    if (!original)
        return;

    values = _glfw_calloc(original->size, sizeof(unsigned short));

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

    glfwSetGammaRamp(handle, &ramp);
    _glfw_free(values);
}

const GammaRamp* glfwGetGammaRamp(GLFWmonitor* handle)
{
    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;
    _glfwFreeGammaArrays(&monitor->currentRamp);
    if (!_glfw.platform.getGammaRamp(monitor, &monitor->currentRamp))
        return NULL;
    return &monitor->currentRamp;
}

void glfwSetGammaRamp(GLFWmonitor* handle, const GammaRamp* ramp)
{
    _GLFWmonitor* monitor = (_GLFWmonitor*) handle;
    if (ramp->size <= 0)
    {
        _glfwInputError(ERR_INVALID_VALUE, "Invalid gamma ramp size %i", ramp->size);
        return;
    }

    if (!monitor->originalRamp.size)
    {
        if (!_glfw.platform.getGammaRamp(monitor, &monitor->originalRamp))
            return;
    }

    _glfw.platform.setGammaRamp(monitor, ramp);
}
