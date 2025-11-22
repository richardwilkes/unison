#include "platform.h"

#if defined(_WIN32)

#include <limits.h>
#include <wchar.h>


// Callback for EnumDisplayMonitors in createMonitor
//
static BOOL CALLBACK monitorCallback(HMONITOR handle,
									 HDC dc,
									 RECT* rect,
									 LPARAM data)
{
	MONITORINFOEXW mi;
	ZeroMemory(&mi, sizeof(mi));
	mi.cbSize = sizeof(mi);

	if (GetMonitorInfoW(handle, (MONITORINFO*) &mi))
	{
		plafMonitor* monitor = (plafMonitor*) data;
		if (wcscmp(mi.szDevice, monitor->win32AdapterName) == 0)
			monitor->win32Handle = handle;
	}

	return TRUE;
}

// Create monitor from an adapter and (optionally) a display
//
static plafMonitor* createMonitor(DISPLAY_DEVICEW* adapter,
								   DISPLAY_DEVICEW* display)
{
	int widthMM, heightMM;
	char* name;
	HDC dc;
	DEVMODEW dm;
	RECT rect;

	if (display)
		name = _plafCreateUTF8FromWideString(display->DeviceString);
	else
		name = _plafCreateUTF8FromWideString(adapter->DeviceString);
	if (!name)
		return NULL;

	ZeroMemory(&dm, sizeof(dm));
	dm.dmSize = sizeof(dm);
	EnumDisplaySettingsW(adapter->DeviceName, ENUM_CURRENT_SETTINGS, &dm);

	dc = CreateDCW(L"DISPLAY", adapter->DeviceName, NULL, NULL);

	widthMM  = GetDeviceCaps(dc, HORZSIZE);
	heightMM = GetDeviceCaps(dc, VERTSIZE);

	DeleteDC(dc);

	plafMonitor* monitor = _plafAllocMonitor(name, widthMM, heightMM);
	_plaf_free(name);

	if (adapter->StateFlags & DISPLAY_DEVICE_MODESPRUNED)
		monitor->win32ModesPruned = true;

	wcscpy(monitor->win32AdapterName, adapter->DeviceName);
	WideCharToMultiByte(CP_UTF8, 0,
						adapter->DeviceName, -1,
						monitor->win32PublicAdapterName,
						sizeof(monitor->win32PublicAdapterName),
						NULL, NULL);

	if (display)
	{
		wcscpy(monitor->win32DisplayName, display->DeviceName);
		WideCharToMultiByte(CP_UTF8, 0,
							display->DeviceName, -1,
							monitor->win32PublicDisplayName,
							sizeof(monitor->win32PublicDisplayName),
							NULL, NULL);
	}

	rect.left   = dm.dmPosition.x;
	rect.top    = dm.dmPosition.y;
	rect.right  = dm.dmPosition.x + dm.dmPelsWidth;
	rect.bottom = dm.dmPosition.y + dm.dmPelsHeight;

	EnumDisplayMonitors(NULL, &rect, monitorCallback, (LPARAM) monitor);
	return monitor;
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Poll for changes in the set of connected monitors
//
void _plafPollMonitors(void)
{
	int i, disconnectedCount;
	plafMonitor** disconnected = NULL;
	DWORD adapterIndex, displayIndex;
	DISPLAY_DEVICEW adapter, display;

	disconnectedCount = _plaf.monitorCount;
	if (disconnectedCount)
	{
		disconnected = _plaf_calloc(_plaf.monitorCount, sizeof(plafMonitor*));
		memcpy(disconnected,
			   _plaf.monitors,
			   _plaf.monitorCount * sizeof(plafMonitor*));
	}

	for (adapterIndex = 0;  ;  adapterIndex++)
	{
		int type = MONITOR_INSERT_LAST;

		ZeroMemory(&adapter, sizeof(adapter));
		adapter.cb = sizeof(adapter);

		if (!EnumDisplayDevicesW(NULL, adapterIndex, &adapter, 0))
			break;

		if (!(adapter.StateFlags & DISPLAY_DEVICE_ACTIVE))
			continue;

		if (adapter.StateFlags & DISPLAY_DEVICE_PRIMARY_DEVICE)
			type = MONITOR_INSERT_FIRST;

		for (displayIndex = 0;  ;  displayIndex++)
		{
			ZeroMemory(&display, sizeof(display));
			display.cb = sizeof(display);

			if (!EnumDisplayDevicesW(adapter.DeviceName, displayIndex, &display, 0))
				break;

			if (!(display.StateFlags & DISPLAY_DEVICE_ACTIVE))
				continue;

			for (i = 0;  i < disconnectedCount;  i++)
			{
				if (disconnected[i] &&
					wcscmp(disconnected[i]->win32DisplayName,
						   display.DeviceName) == 0)
				{
					disconnected[i] = NULL;
					// handle may have changed, update
					EnumDisplayMonitors(NULL, NULL, monitorCallback, (LPARAM) _plaf.monitors[i]);
					break;
				}
			}

			if (i < disconnectedCount)
				continue;

			plafMonitor* monitor = createMonitor(&adapter, &display);
			if (!monitor)
			{
				_plaf_free(disconnected);
				return;
			}

			_plafMonitorNotify(monitor, CONNECTED, type);

			type = MONITOR_INSERT_LAST;
		}

		// HACK: If an active adapter does not have any display devices
		//       (as sometimes happens), add it directly as a monitor
		if (displayIndex == 0)
		{
			for (i = 0;  i < disconnectedCount;  i++)
			{
				if (disconnected[i] &&
					wcscmp(disconnected[i]->win32AdapterName,
						   adapter.DeviceName) == 0)
				{
					disconnected[i] = NULL;
					break;
				}
			}

			if (i < disconnectedCount)
				continue;

			plafMonitor* monitor = createMonitor(&adapter, NULL);
			if (!monitor)
			{
				_plaf_free(disconnected);
				return;
			}

			_plafMonitorNotify(monitor, CONNECTED, type);
		}
	}

	for (i = 0;  i < disconnectedCount;  i++)
	{
		if (disconnected[i])
			_plafMonitorNotify(disconnected[i], DISCONNECTED, 0);
	}

	_plaf_free(disconnected);
}

// Change the current video mode
//
void _plafSetVideoMode(plafMonitor* monitor, const plafVideoMode* desired)
{
	plafVideoMode current;
	const plafVideoMode* best;
	DEVMODEW dm;
	LONG result;

	best = _plafChooseVideoMode(monitor, desired);
	_plafGetVideoMode(monitor, &current);
	if (_plafCompareVideoModes(&current, best) == 0)
		return;

	ZeroMemory(&dm, sizeof(dm));
	dm.dmSize = sizeof(dm);
	dm.dmFields           = DM_PELSWIDTH | DM_PELSHEIGHT | DM_BITSPERPEL |
							DM_DISPLAYFREQUENCY;
	dm.dmPelsWidth        = best->width;
	dm.dmPelsHeight       = best->height;
	dm.dmBitsPerPel       = best->redBits + best->greenBits + best->blueBits;
	dm.dmDisplayFrequency = best->refreshRate;

	if (dm.dmBitsPerPel < 15 || dm.dmBitsPerPel >= 24)
		dm.dmBitsPerPel = 32;

	result = ChangeDisplaySettingsExW(monitor->win32AdapterName,
									  &dm,
									  NULL,
									  CDS_FULLSCREEN,
									  NULL);
	if (result == DISP_CHANGE_SUCCESSFUL)
		monitor->win32ModeChanged = true;
	else
	{
		const char* description = "Unknown error";

		if (result == DISP_CHANGE_BADDUALVIEW)
			description = "The system uses DualView";
		else if (result == DISP_CHANGE_BADFLAGS)
			description = "Invalid flags";
		else if (result == DISP_CHANGE_BADMODE)
			description = "Graphics mode not supported";
		else if (result == DISP_CHANGE_BADPARAM)
			description = "Invalid parameter";
		else if (result == DISP_CHANGE_FAILED)
			description = "Graphics mode failed";
		else if (result == DISP_CHANGE_NOTUPDATED)
			description = "Failed to write to registry";
		else if (result == DISP_CHANGE_RESTART)
			description = "Computer restart required";

		_plafInputError("Win32: Failed to set video mode: %s", description);
	}
}

// Restore the previously saved (original) video mode
//
void _plafRestoreVideoMode(plafMonitor* monitor)
{
	if (monitor->win32ModeChanged)
	{
		ChangeDisplaySettingsExW(monitor->win32AdapterName,
								 NULL, NULL, CDS_FULLSCREEN, NULL);
		monitor->win32ModeChanged = false;
	}
}

void _plafGetHMONITORContentScale(HMONITOR handle, float* xscale, float* yscale)
{
	UINT xdpi, ydpi;

	*xscale = 0.f;
	*yscale = 0.f;

	if (_plaf.win32ShCoreGetDpiForMonitor_(handle, MDT_EFFECTIVE_DPI, &xdpi, &ydpi) != S_OK)
	{
		_plafInputError("Win32: Failed to query monitor DPI");
		return;
	}

	*xscale = xdpi / (float) USER_DEFAULT_SCREEN_DPI;
	*yscale = ydpi / (float) USER_DEFAULT_SCREEN_DPI;
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF platform API                      //////
//////////////////////////////////////////////////////////////////////////

void plafGetMonitorPos(plafMonitor* monitor, int* xpos, int* ypos) {
	DEVMODEW dm;
	ZeroMemory(&dm, sizeof(dm));
	dm.dmSize = sizeof(dm);
	EnumDisplaySettingsExW(monitor->win32AdapterName, ENUM_CURRENT_SETTINGS, &dm, EDS_ROTATEDMODE);
	*xpos = dm.dmPosition.x;
	*ypos = dm.dmPosition.y;
}

void plafGetMonitorContentScale(plafMonitor* monitor, float* xscale, float* yscale) {
	_plafGetHMONITORContentScale(monitor->win32Handle, xscale, yscale);
}

void plafGetMonitorWorkarea(plafMonitor* monitor, int* xpos, int* ypos, int* width, int* height) {
	MONITORINFO mi = { sizeof(mi) };
	GetMonitorInfoW(monitor->win32Handle, &mi);
	*xpos = mi.rcWork.left;
	*ypos = mi.rcWork.top;
	*width = mi.rcWork.right - mi.rcWork.left;
	*height = mi.rcWork.bottom - mi.rcWork.top;
}

plafVideoMode* _plafGetVideoModes(plafMonitor* monitor, int* count)
{
	int modeIndex = 0, size = 0;
	plafVideoMode* result = NULL;

	*count = 0;

	for (;;)
	{
		int i;
		plafVideoMode mode;
		DEVMODEW dm;

		ZeroMemory(&dm, sizeof(dm));
		dm.dmSize = sizeof(dm);

		if (!EnumDisplaySettingsW(monitor->win32AdapterName, modeIndex, &dm))
			break;

		modeIndex++;

		// Skip modes with less than 15 BPP
		if (dm.dmBitsPerPel < 15)
			continue;

		mode.width  = dm.dmPelsWidth;
		mode.height = dm.dmPelsHeight;
		mode.refreshRate = dm.dmDisplayFrequency;
		_plafSplitBPP(dm.dmBitsPerPel, &mode.redBits, &mode.greenBits, &mode.blueBits);

		for (i = 0;  i < *count;  i++)
		{
			if (_plafCompareVideoModes(result + i, &mode) == 0)
				break;
		}

		// Skip duplicate modes
		if (i < *count)
			continue;

		if (monitor->win32ModesPruned)
		{
			// Skip modes not supported by the connected displays
			if (ChangeDisplaySettingsExW(monitor->win32AdapterName,
										 &dm,
										 NULL,
										 CDS_TEST,
										 NULL) != DISP_CHANGE_SUCCESSFUL)
			{
				continue;
			}
		}

		if (*count == size)
		{
			size += 128;
			result = (plafVideoMode*) _plaf_realloc(result, size * sizeof(plafVideoMode));
		}

		(*count)++;
		result[*count - 1] = mode;
	}

	if (!*count)
	{
		// HACK: Report the current mode if no valid modes were found
		result = _plaf_calloc(1, sizeof(plafVideoMode));
		_plafGetVideoMode(monitor, result);
		*count = 1;
	}

	return result;
}

IntBool _plafGetVideoMode(plafMonitor* monitor, plafVideoMode* mode) {
	DEVMODEW dm;
	ZeroMemory(&dm, sizeof(dm));
	dm.dmSize = sizeof(dm);

	if (!EnumDisplaySettingsW(monitor->win32AdapterName, ENUM_CURRENT_SETTINGS, &dm))
	{
		_plafInputError("Win32: Failed to query display settings");
		return false;
	}

	mode->width  = dm.dmPelsWidth;
	mode->height = dm.dmPelsHeight;
	mode->refreshRate = dm.dmDisplayFrequency;
	_plafSplitBPP(dm.dmBitsPerPel, &mode->redBits, &mode->greenBits, &mode->blueBits);

	return true;
}

IntBool _plafGetGammaRamp(plafMonitor* monitor, plafGammaRamp* ramp) {
	HDC dc;
	WORD values[3][256];

	dc = CreateDCW(L"DISPLAY", monitor->win32AdapterName, NULL, NULL);
	GetDeviceGammaRamp(dc, values);
	DeleteDC(dc);

	_plafAllocGammaArrays(ramp, 256);

	memcpy(ramp->red,   values[0], sizeof(values[0]));
	memcpy(ramp->green, values[1], sizeof(values[1]));
	memcpy(ramp->blue,  values[2], sizeof(values[2]));

	return true;
}

void _plafSetGammaRamp(plafMonitor* monitor, const plafGammaRamp* ramp) {
	HDC dc;
	WORD values[3][256];

	if (ramp->size != 256)
	{
		_plafInputError("Win32: Gamma ramp size must be 256");
		return;
	}

	memcpy(values[0], ramp->red,   sizeof(values[0]));
	memcpy(values[1], ramp->green, sizeof(values[1]));
	memcpy(values[2], ramp->blue,  sizeof(values[2]));

	dc = CreateDCW(L"DISPLAY", monitor->win32AdapterName, NULL, NULL);
	SetDeviceGammaRamp(dc, values);
	DeleteDC(dc);
}

#endif // _WIN32
