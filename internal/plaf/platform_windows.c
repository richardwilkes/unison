#if defined(_WIN32)

#include "platform.h"

static const GUID GUID_DEVINTERFACE_HID = {0x4d1e55b2,0xf16f,0x11cf,{0x88,0xcb,0x00,0x11,0x11,0x00,0x00,0x30}};

// Load necessary libraries (DLLs)
static plafError* loadLibraries(void)
{
	if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
								GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
							(const WCHAR*) &_plaf,
							(HMODULE*) &_plaf.win32Instance))
	{
		return createErrorResponse("Failed to retrieve own module handle");
	}

	_plaf.win32User32Instance = _plafLoadModule("user32.dll");
	if (!_plaf.win32User32Instance)
	{
		return createErrorResponse("Failed to load user32.dll");
	}

	_plaf.win32User32EnableNonClientDpiScaling_ = (FN_EnableNonClientDpiScaling)
		_plafGetModuleSymbol(_plaf.win32User32Instance, "EnableNonClientDpiScaling");
	_plaf.win32User32SetProcessDpiAwarenessContext_ = (FN_SetProcessDpiAwarenessContext)
		_plafGetModuleSymbol(_plaf.win32User32Instance, "SetProcessDpiAwarenessContext");
	_plaf.win32User32GetDpiForWindow_ = (FN_GetDpiForWindow)
		_plafGetModuleSymbol(_plaf.win32User32Instance, "GetDpiForWindow");
	_plaf.win32User32AdjustWindowRectExForDpi_ = (FN_AdjustWindowRectExForDpi)
		_plafGetModuleSymbol(_plaf.win32User32Instance, "AdjustWindowRectExForDpi");
	_plaf.win32User32GetSystemMetricsForDpi_ = (FN_GetSystemMetricsForDpi)
		_plafGetModuleSymbol(_plaf.win32User32Instance, "GetSystemMetricsForDpi");

	_plaf.win32DwmInstance = _plafLoadModule("dwmapi.dll");
	if (_plaf.win32DwmInstance)
	{
		_plaf.win32DwmIsCompositionEnabled = (FN_DwmIsCompositionEnabled)
			_plafGetModuleSymbol(_plaf.win32DwmInstance, "DwmIsCompositionEnabled");
		_plaf.win32DwmFlush = (FN_DwmFlush)
			_plafGetModuleSymbol(_plaf.win32DwmInstance, "DwmFlush");
		_plaf.win32DwmEnableBlurBehindWindow = (FN_DwmEnableBlurBehindWindow)
			_plafGetModuleSymbol(_plaf.win32DwmInstance, "DwmEnableBlurBehindWindow");
		_plaf.win32DwmGetColorizationColor = (FN_DwmGetColorizationColor)
			_plafGetModuleSymbol(_plaf.win32DwmInstance, "DwmGetColorizationColor");
	}

	_plaf.win32ShCoreInstance = _plafLoadModule("shcore.dll");
	if (_plaf.win32ShCoreInstance)
	{
		_plaf.win32ShCoreSetProcessDpiAwareness_ = (FN_SetProcessDpiAwareness)
			_plafGetModuleSymbol(_plaf.win32ShCoreInstance, "SetProcessDpiAwareness");
		_plaf.win32ShCoreGetDpiForMonitor_ = (FN_GetDpiForMonitor)
			_plafGetModuleSymbol(_plaf.win32ShCoreInstance, "GetDpiForMonitor");
	}

	_plaf.win32NTInstance = _plafLoadModule("ntdll.dll");
	if (_plaf.win32NTInstance)
	{
		_plaf.win32NTRtlVerifyVersionInfo_ = (FN_RtlVerifyVersionInfo)
			_plafGetModuleSymbol(_plaf.win32NTInstance, "RtlVerifyVersionInfo");
	}

	return NULL;
}

// Unload used libraries (DLLs)
//
static void freeLibraries(void)
{
	if (_plaf.win32User32Instance)
		_plafFreeModule(_plaf.win32User32Instance);

	if (_plaf.win32DwmInstance)
		_plafFreeModule(_plaf.win32DwmInstance);

	if (_plaf.win32ShCoreInstance)
		_plafFreeModule(_plaf.win32ShCoreInstance);

	if (_plaf.win32NTInstance)
		_plafFreeModule(_plaf.win32NTInstance);
}

// Create key code translation tables
//
static void createKeyTables(void)
{
	memset(_plaf.keyCodes, -1, sizeof(_plaf.keyCodes));
	memset(_plaf.scanCodes, -1, sizeof(_plaf.scanCodes));

	_plaf.keyCodes[0x00B] = KEY_0;
	_plaf.keyCodes[0x002] = KEY_1;
	_plaf.keyCodes[0x003] = KEY_2;
	_plaf.keyCodes[0x004] = KEY_3;
	_plaf.keyCodes[0x005] = KEY_4;
	_plaf.keyCodes[0x006] = KEY_5;
	_plaf.keyCodes[0x007] = KEY_6;
	_plaf.keyCodes[0x008] = KEY_7;
	_plaf.keyCodes[0x009] = KEY_8;
	_plaf.keyCodes[0x00A] = KEY_9;
	_plaf.keyCodes[0x01E] = KEY_A;
	_plaf.keyCodes[0x030] = KEY_B;
	_plaf.keyCodes[0x02E] = KEY_C;
	_plaf.keyCodes[0x020] = KEY_D;
	_plaf.keyCodes[0x012] = KEY_E;
	_plaf.keyCodes[0x021] = KEY_F;
	_plaf.keyCodes[0x022] = KEY_G;
	_plaf.keyCodes[0x023] = KEY_H;
	_plaf.keyCodes[0x017] = KEY_I;
	_plaf.keyCodes[0x024] = KEY_J;
	_plaf.keyCodes[0x025] = KEY_K;
	_plaf.keyCodes[0x026] = KEY_L;
	_plaf.keyCodes[0x032] = KEY_M;
	_plaf.keyCodes[0x031] = KEY_N;
	_plaf.keyCodes[0x018] = KEY_O;
	_plaf.keyCodes[0x019] = KEY_P;
	_plaf.keyCodes[0x010] = KEY_Q;
	_plaf.keyCodes[0x013] = KEY_R;
	_plaf.keyCodes[0x01F] = KEY_S;
	_plaf.keyCodes[0x014] = KEY_T;
	_plaf.keyCodes[0x016] = KEY_U;
	_plaf.keyCodes[0x02F] = KEY_V;
	_plaf.keyCodes[0x011] = KEY_W;
	_plaf.keyCodes[0x02D] = KEY_X;
	_plaf.keyCodes[0x015] = KEY_Y;
	_plaf.keyCodes[0x02C] = KEY_Z;

	_plaf.keyCodes[0x028] = KEY_APOSTROPHE;
	_plaf.keyCodes[0x02B] = KEY_BACKSLASH;
	_plaf.keyCodes[0x033] = KEY_COMMA;
	_plaf.keyCodes[0x00D] = KEY_EQUAL;
	_plaf.keyCodes[0x029] = KEY_GRAVE_ACCENT;
	_plaf.keyCodes[0x01A] = KEY_LEFT_BRACKET;
	_plaf.keyCodes[0x00C] = KEY_MINUS;
	_plaf.keyCodes[0x034] = KEY_PERIOD;
	_plaf.keyCodes[0x01B] = KEY_RIGHT_BRACKET;
	_plaf.keyCodes[0x027] = KEY_SEMICOLON;
	_plaf.keyCodes[0x035] = KEY_SLASH;
	_plaf.keyCodes[0x056] = KEY_WORLD_2;

	_plaf.keyCodes[0x00E] = KEY_BACKSPACE;
	_plaf.keyCodes[0x153] = KEY_DELETE;
	_plaf.keyCodes[0x14F] = KEY_END;
	_plaf.keyCodes[0x01C] = KEY_ENTER;
	_plaf.keyCodes[0x001] = KEY_ESCAPE;
	_plaf.keyCodes[0x147] = KEY_HOME;
	_plaf.keyCodes[0x152] = KEY_INSERT;
	_plaf.keyCodes[0x15D] = KEY_MENU;
	_plaf.keyCodes[0x151] = KEY_PAGE_DOWN;
	_plaf.keyCodes[0x149] = KEY_PAGE_UP;
	_plaf.keyCodes[0x045] = KEY_PAUSE;
	_plaf.keyCodes[0x039] = KEY_SPACE;
	_plaf.keyCodes[0x00F] = KEY_TAB;
	_plaf.keyCodes[0x03A] = KEY_CAPS_LOCK;
	_plaf.keyCodes[0x145] = KEY_NUM_LOCK;
	_plaf.keyCodes[0x046] = KEY_SCROLL_LOCK;
	_plaf.keyCodes[0x03B] = KEY_F1;
	_plaf.keyCodes[0x03C] = KEY_F2;
	_plaf.keyCodes[0x03D] = KEY_F3;
	_plaf.keyCodes[0x03E] = KEY_F4;
	_plaf.keyCodes[0x03F] = KEY_F5;
	_plaf.keyCodes[0x040] = KEY_F6;
	_plaf.keyCodes[0x041] = KEY_F7;
	_plaf.keyCodes[0x042] = KEY_F8;
	_plaf.keyCodes[0x043] = KEY_F9;
	_plaf.keyCodes[0x044] = KEY_F10;
	_plaf.keyCodes[0x057] = KEY_F11;
	_plaf.keyCodes[0x058] = KEY_F12;
	_plaf.keyCodes[0x064] = KEY_F13;
	_plaf.keyCodes[0x065] = KEY_F14;
	_plaf.keyCodes[0x066] = KEY_F15;
	_plaf.keyCodes[0x067] = KEY_F16;
	_plaf.keyCodes[0x068] = KEY_F17;
	_plaf.keyCodes[0x069] = KEY_F18;
	_plaf.keyCodes[0x06A] = KEY_F19;
	_plaf.keyCodes[0x06B] = KEY_F20;
	_plaf.keyCodes[0x06C] = KEY_F21;
	_plaf.keyCodes[0x06D] = KEY_F22;
	_plaf.keyCodes[0x06E] = KEY_F23;
	_plaf.keyCodes[0x076] = KEY_F24;
	_plaf.keyCodes[0x038] = KEY_LEFT_ALT;
	_plaf.keyCodes[0x01D] = KEY_LEFT_CONTROL;
	_plaf.keyCodes[0x02A] = KEY_LEFT_SHIFT;
	_plaf.keyCodes[0x15B] = KEY_LEFT_SUPER;
	_plaf.keyCodes[0x137] = KEY_PRINT_SCREEN;
	_plaf.keyCodes[0x138] = KEY_RIGHT_ALT;
	_plaf.keyCodes[0x11D] = KEY_RIGHT_CONTROL;
	_plaf.keyCodes[0x036] = KEY_RIGHT_SHIFT;
	_plaf.keyCodes[0x15C] = KEY_RIGHT_SUPER;
	_plaf.keyCodes[0x150] = KEY_DOWN;
	_plaf.keyCodes[0x14B] = KEY_LEFT;
	_plaf.keyCodes[0x14D] = KEY_RIGHT;
	_plaf.keyCodes[0x148] = KEY_UP;

	_plaf.keyCodes[0x052] = KEY_KP_0;
	_plaf.keyCodes[0x04F] = KEY_KP_1;
	_plaf.keyCodes[0x050] = KEY_KP_2;
	_plaf.keyCodes[0x051] = KEY_KP_3;
	_plaf.keyCodes[0x04B] = KEY_KP_4;
	_plaf.keyCodes[0x04C] = KEY_KP_5;
	_plaf.keyCodes[0x04D] = KEY_KP_6;
	_plaf.keyCodes[0x047] = KEY_KP_7;
	_plaf.keyCodes[0x048] = KEY_KP_8;
	_plaf.keyCodes[0x049] = KEY_KP_9;
	_plaf.keyCodes[0x04E] = KEY_KP_ADD;
	_plaf.keyCodes[0x053] = KEY_KP_DECIMAL;
	_plaf.keyCodes[0x135] = KEY_KP_DIVIDE;
	_plaf.keyCodes[0x11C] = KEY_KP_ENTER;
	_plaf.keyCodes[0x059] = KEY_KP_EQUAL;
	_plaf.keyCodes[0x037] = KEY_KP_MULTIPLY;
	_plaf.keyCodes[0x04A] = KEY_KP_SUBTRACT;

	for (int scanCode = 0;  scanCode < MAX_KEY_CODES;  scanCode++) {
		if (_plaf.keyCodes[scanCode] > 0)
			_plaf.scanCodes[_plaf.keyCodes[scanCode]] = scanCode;
	}
}

// Window procedure for the hidden helper window
//
static LRESULT CALLBACK helperWindowProc(HWND hWnd, UINT uMsg, WPARAM wParam, LPARAM lParam)
{
	if (uMsg == WM_DISPLAYCHANGE) {
		_plafPollMonitorsWin32();
	}
	return DefWindowProcW(hWnd, uMsg, wParam, lParam);
}

// Creates a dummy window for behind-the-scenes work
//
static plafError* createHelperWindow(void)
{
	MSG msg;
	WNDCLASSEXW wc = { sizeof(wc) };

	wc.style         = CS_OWNDC;
	wc.lpfnWndProc   = (WNDPROC) helperWindowProc;
	wc.hInstance     = _plaf.win32Instance;
	wc.lpszClassName = L"PLAF3 Helper";

	_plaf.win32HelperWindowClass = RegisterClassExW(&wc);
	if (!_plaf.win32HelperWindowClass)
	{
		return createErrorResponse("Failed to register helper window class");
	}

	_plaf.win32HelperWindowHandle =
		CreateWindowExW(WS_EX_OVERLAPPEDWINDOW,
						MAKEINTATOM(_plaf.win32HelperWindowClass),
						L"PLAF message window",
						WS_CLIPSIBLINGS | WS_CLIPCHILDREN,
						0, 0, 1, 1,
						NULL, NULL,
						_plaf.win32Instance,
						NULL);

	if (!_plaf.win32HelperWindowHandle)
	{
		return createErrorResponse("Failed to create helper window");
	}

	// HACK: The command to the first ShowWindow call is ignored if the parent
	//       process passed along a STARTUPINFO, so clear that with a no-op call
	ShowWindow(_plaf.win32HelperWindowHandle, SW_HIDE);

	// Register for HID device notifications
	// TODO: Consider eliminating this, as we no longer need HID support, do we?
	{
		DEV_BROADCAST_DEVICEINTERFACE_W dbi;
		ZeroMemory(&dbi, sizeof(dbi));
		dbi.dbcc_size = sizeof(dbi);
		dbi.dbcc_devicetype = DBT_DEVTYP_DEVICEINTERFACE;
		dbi.dbcc_classguid = GUID_DEVINTERFACE_HID;

		_plaf.win32DeviceNotificationHandle =
			RegisterDeviceNotificationW(_plaf.win32HelperWindowHandle,
										(DEV_BROADCAST_HDR*) &dbi,
										DEVICE_NOTIFY_WINDOW_HANDLE);
	}

	while (PeekMessageW(&msg, _plaf.win32HelperWindowHandle, 0, 0, PM_REMOVE))
	{
		TranslateMessage(&msg);
		DispatchMessageW(&msg);
	}

   return NULL;
}

//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Returns a wide string version of the specified UTF-8 string
//
WCHAR* _plafCreateWideStringFromUTF8Win32(const char* src) {
	int count = MultiByteToWideChar(CP_UTF8, 0, src, -1, NULL, 0);
	if (!count) {
		return NULL;
	}
	WCHAR* target = _plaf_calloc(count, sizeof(WCHAR));
	if (!MultiByteToWideChar(CP_UTF8, 0, src, -1, target, count)) {
		_plaf_free(target);
		return NULL;
	}
	return target;
}

// Returns a UTF-8 string version of the specified wide string
//
char* _plafCreateUTF8FromWideStringWin32(const WCHAR* src) {
	int size = WideCharToMultiByte(CP_UTF8, 0, src, -1, NULL, 0, NULL, NULL);
	if (!size) {
		return NULL;
	}
	char* target = _plaf_calloc(size, 1);
	if (!WideCharToMultiByte(CP_UTF8, 0, src, -1, target, size, NULL, NULL)) {
		_plaf_free(target);
		return NULL;
	}
	return target;
}

// Replacement for IsWindowsVersionOrGreater, as we cannot rely on the
// application having a correct embedded manifest
//
BOOL _plafIsWindowsVersionOrGreaterWin32(WORD major, WORD minor, WORD sp)
{
	OSVERSIONINFOEXW osvi = { sizeof(osvi), major, minor, 0, 0, {0}, sp };
	DWORD mask = VER_MAJORVERSION | VER_MINORVERSION | VER_SERVICEPACKMAJOR;
	ULONGLONG cond = VerSetConditionMask(0, VER_MAJORVERSION, VER_GREATER_EQUAL);
	cond = VerSetConditionMask(cond, VER_MINORVERSION, VER_GREATER_EQUAL);
	cond = VerSetConditionMask(cond, VER_SERVICEPACKMAJOR, VER_GREATER_EQUAL);
	// HACK: Use RtlVerifyVersionInfo instead of VerifyVersionInfoW as the
	//       latter lies unless the user knew to embed a non-default manifest
	//       announcing support for Windows 10 via supportedOS GUID
	return _plaf.win32NTRtlVerifyVersionInfo_(&osvi, mask, cond) == 0;
}

// Checks whether we are on at least the specified build of Windows 10
//
BOOL IsWindows10BuildOrGreater(WORD build)
{
	OSVERSIONINFOEXW osvi = { sizeof(osvi), 10, 0, build };
	DWORD mask = VER_MAJORVERSION | VER_MINORVERSION | VER_BUILDNUMBER;
	ULONGLONG cond = VerSetConditionMask(0, VER_MAJORVERSION, VER_GREATER_EQUAL);
	cond = VerSetConditionMask(cond, VER_MINORVERSION, VER_GREATER_EQUAL);
	cond = VerSetConditionMask(cond, VER_BUILDNUMBER, VER_GREATER_EQUAL);
	// HACK: Use RtlVerifyVersionInfo instead of VerifyVersionInfoW as the
	//       latter lies unless the user knew to embed a non-default manifest
	//       announcing support for Windows 10 via supportedOS GUID
	return _plaf.win32NTRtlVerifyVersionInfo_(&osvi, mask, cond) == 0;
}

plafError* _plafInit(void) {
	plafError* errRsp = loadLibraries();
	if (errRsp) {
		plafTerminate();
		return errRsp;
	}

	createKeyTables();

	if (IsWindows10Version1703OrGreater())
		_plaf.win32User32SetProcessDpiAwarenessContext_(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2);
	else {
		_plaf.win32ShCoreSetProcessDpiAwareness_(PROCESS_PER_MONITOR_DPI_AWARE);
	}

	errRsp = createHelperWindow();
	if (errRsp) {
		plafTerminate();
		return errRsp;
	}

	_plafPollMonitorsWin32();
	return NULL;
}

void _plafTerminate(void)
{
	if (_plaf.win32BlankCursor)
		DestroyIcon((HICON) _plaf.win32BlankCursor);

	if (_plaf.win32DeviceNotificationHandle)
		UnregisterDeviceNotification(_plaf.win32DeviceNotificationHandle);

	if (_plaf.win32HelperWindowHandle)
		DestroyWindow(_plaf.win32HelperWindowHandle);
	if (_plaf.win32HelperWindowClass)
		UnregisterClassW(MAKEINTATOM(_plaf.win32HelperWindowClass), _plaf.win32Instance);
	if (_plaf.win32MainWindowClass)
		UnregisterClassW(MAKEINTATOM(_plaf.win32MainWindowClass), _plaf.win32Instance);


	_plafTerminateWGL();

	freeLibraries();
}

#endif // _WIN32
