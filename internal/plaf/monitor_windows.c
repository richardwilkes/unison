#include "platform.h"

#if defined(_WIN32)

#include <limits.h>
#include <wchar.h>


// Callback for EnumDisplayMonitors in createMonitor
static BOOL CALLBACK monitorCallback(HMONITOR handle, HDC dc, RECT* rect, LPARAM data) {
	MONITORINFOEXW mi;
	ZeroMemory(&mi, sizeof(mi));
	mi.cbSize = sizeof(mi);
	if (GetMonitorInfoW(handle, (MONITORINFO*) &mi)) {
		plafMonitor* monitor = (plafMonitor*) data;
		if (wcscmp(mi.szDevice, monitor->win32Adapter) == 0) {
			monitor->win32Handle = handle;
		}
	}
	return TRUE;
}

// Create monitor from an adapter and (optionally) a display
static plafMonitor* createMonitor(DISPLAY_DEVICEW* adapter, DISPLAY_DEVICEW* display) {
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

	wcscpy(monitor->win32Adapter, adapter->DeviceName);

	if (display)
	{
		wcscpy(monitor->win32Display, display->DeviceName);
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
void _plafPollMonitors(void) {
	plafMonitor** disconnected = NULL;
	int disconnectedCount = _plaf.monitorCount;
	if (disconnectedCount) {
		disconnected = _plaf_calloc(_plaf.monitorCount, sizeof(plafMonitor*));
		memcpy(disconnected, _plaf.monitors, _plaf.monitorCount * sizeof(plafMonitor*));
	}
	int i;
	DISPLAY_DEVICEW adapter;
	for (DWORD adapterIndex = 0; ; adapterIndex++) {
		ZeroMemory(&adapter, sizeof(adapter));
		adapter.cb = sizeof(adapter);
		if (!EnumDisplayDevicesW(NULL, adapterIndex, &adapter, 0)) {
			break;
		}
		if (!(adapter.StateFlags & DISPLAY_DEVICE_ACTIVE)) {
			continue;
		}
		bool insertFirst = false;
		if (adapter.StateFlags & DISPLAY_DEVICE_PRIMARY_DEVICE) {
			insertFirst = true;
		}
		DWORD displayIndex;
		for (displayIndex = 0; ; displayIndex++) {
			DISPLAY_DEVICEW display;
			ZeroMemory(&display, sizeof(display));
			display.cb = sizeof(display);
			if (!EnumDisplayDevicesW(adapter.DeviceName, displayIndex, &display, 0)) {
				break;
			}
			if (!(display.StateFlags & DISPLAY_DEVICE_ACTIVE)) {
				continue;
			}
			for (i = 0; i < disconnectedCount; i++) {
				if (disconnected[i] && wcscmp(disconnected[i]->win32Display, display.DeviceName) == 0) {
					disconnected[i] = NULL;
					EnumDisplayMonitors(NULL, NULL, monitorCallback, (LPARAM)_plaf.monitors[i]);
					break;
				}
			}
			if (i < disconnectedCount) {
				continue;
			}
			plafMonitor* monitor = createMonitor(&adapter, &display);
			if (!monitor) {
				_plaf_free(disconnected);
				return;
			}
			_plafMonitorNotify(monitor, true, insertFirst);
			insertFirst = false;
		}
		if (displayIndex == 0) {
			for (i = 0; i < disconnectedCount; i++) {
				if (disconnected[i] && wcscmp(disconnected[i]->win32Adapter, adapter.DeviceName) == 0) {
					disconnected[i] = NULL;
					break;
				}
			}
			if (i < disconnectedCount) {
				continue;
			}
			plafMonitor* monitor = createMonitor(&adapter, NULL);
			if (!monitor) {
				_plaf_free(disconnected);
				return;
			}
			_plafMonitorNotify(monitor, true, insertFirst);
		}
	}
	for (i = 0; i < disconnectedCount; i++) {
		if (disconnected[i]) {
			_plafMonitorNotify(disconnected[i], false, false);
		}
	}
	_plaf_free(disconnected);
}

void _plafGetHMONITORContentScale(HMONITOR handle, float* xscale, float* yscale) {
	*xscale = 0;
	*yscale = 0;
	UINT xdpi;
	UINT ydpi;
	if (_plaf.win32ShCoreGetDpiForMonitor_(handle, MDT_EFFECTIVE_DPI, &xdpi, &ydpi) == S_OK) {
		*xscale = xdpi / (float) USER_DEFAULT_SCREEN_DPI;
		*yscale = ydpi / (float) USER_DEFAULT_SCREEN_DPI;
	} else {
		*xscale = 1;
		*yscale = 1;
	}
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF platform API                      //////
//////////////////////////////////////////////////////////////////////////

void plafGetMonitorPos(plafMonitor* monitor, int* xpos, int* ypos) {
	DEVMODEW dm;
	ZeroMemory(&dm, sizeof(dm));
	dm.dmSize = sizeof(dm);
	EnumDisplaySettingsExW(monitor->win32Adapter, ENUM_CURRENT_SETTINGS, &dm, EDS_ROTATEDMODE);
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

bool _plafGetVideoMode(plafMonitor* monitor, plafVideoMode* mode) {
	DEVMODEW dm;
	ZeroMemory(&dm, sizeof(dm));
	dm.dmSize = sizeof(dm);
	if (!EnumDisplaySettingsW(monitor->win32Adapter, ENUM_CURRENT_SETTINGS, &dm)) {
		return false;
	}
	mode->width  = dm.dmPelsWidth;
	mode->height = dm.dmPelsHeight;
	return true;
}

#endif // _WIN32
