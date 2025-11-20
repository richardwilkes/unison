#if defined(_WIN32)

#include "platform.h"

static const GUID GUID_DEVINTERFACE_HID = {0x4d1e55b2,0xf16f,0x11cf,{0x88,0xcb,0x00,0x11,0x11,0x00,0x00,0x30}};

// Load necessary libraries (DLLs)
static ErrorResponse* loadLibraries(void)
{
	if (!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS |
								GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
							(const WCHAR*) &_glfw,
							(HMODULE*) &_glfw.win32Instance))
	{
		return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to retrieve own module handle");
	}

	_glfw.win32User32Instance = _glfwPlatformLoadModule("user32.dll");
	if (!_glfw.win32User32Instance)
	{
		return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to load user32.dll");
	}

	_glfw.win32User32EnableNonClientDpiScaling_ = (FN_EnableNonClientDpiScaling)
		_glfwPlatformGetModuleSymbol(_glfw.win32User32Instance, "EnableNonClientDpiScaling");
	_glfw.win32User32SetProcessDpiAwarenessContext_ = (FN_SetProcessDpiAwarenessContext)
		_glfwPlatformGetModuleSymbol(_glfw.win32User32Instance, "SetProcessDpiAwarenessContext");
	_glfw.win32User32GetDpiForWindow_ = (FN_GetDpiForWindow)
		_glfwPlatformGetModuleSymbol(_glfw.win32User32Instance, "GetDpiForWindow");
	_glfw.win32User32AdjustWindowRectExForDpi_ = (FN_AdjustWindowRectExForDpi)
		_glfwPlatformGetModuleSymbol(_glfw.win32User32Instance, "AdjustWindowRectExForDpi");
	_glfw.win32User32GetSystemMetricsForDpi_ = (FN_GetSystemMetricsForDpi)
		_glfwPlatformGetModuleSymbol(_glfw.win32User32Instance, "GetSystemMetricsForDpi");

	_glfw.win32DwmInstance = _glfwPlatformLoadModule("dwmapi.dll");
	if (_glfw.win32DwmInstance)
	{
		_glfw.win32DwmIsCompositionEnabled = (FN_DwmIsCompositionEnabled)
			_glfwPlatformGetModuleSymbol(_glfw.win32DwmInstance, "DwmIsCompositionEnabled");
		_glfw.win32DwmFlush = (FN_DwmFlush)
			_glfwPlatformGetModuleSymbol(_glfw.win32DwmInstance, "DwmFlush");
		_glfw.win32DwmEnableBlurBehindWindow = (FN_DwmEnableBlurBehindWindow)
			_glfwPlatformGetModuleSymbol(_glfw.win32DwmInstance, "DwmEnableBlurBehindWindow");
		_glfw.win32DwmGetColorizationColor = (FN_DwmGetColorizationColor)
			_glfwPlatformGetModuleSymbol(_glfw.win32DwmInstance, "DwmGetColorizationColor");
	}

	_glfw.win32ShCoreInstance = _glfwPlatformLoadModule("shcore.dll");
	if (_glfw.win32ShCoreInstance)
	{
		_glfw.win32ShCoreSetProcessDpiAwareness_ = (FN_SetProcessDpiAwareness)
			_glfwPlatformGetModuleSymbol(_glfw.win32ShCoreInstance, "SetProcessDpiAwareness");
		_glfw.win32ShCoreGetDpiForMonitor_ = (FN_GetDpiForMonitor)
			_glfwPlatformGetModuleSymbol(_glfw.win32ShCoreInstance, "GetDpiForMonitor");
	}

	_glfw.win32NTInstance = _glfwPlatformLoadModule("ntdll.dll");
	if (_glfw.win32NTInstance)
	{
		_glfw.win32NTRtlVerifyVersionInfo_ = (FN_RtlVerifyVersionInfo)
			_glfwPlatformGetModuleSymbol(_glfw.win32NTInstance, "RtlVerifyVersionInfo");
	}

	return NULL;
}

// Unload used libraries (DLLs)
//
static void freeLibraries(void)
{
	if (_glfw.win32User32Instance)
		_glfwPlatformFreeModule(_glfw.win32User32Instance);

	if (_glfw.win32DwmInstance)
		_glfwPlatformFreeModule(_glfw.win32DwmInstance);

	if (_glfw.win32ShCoreInstance)
		_glfwPlatformFreeModule(_glfw.win32ShCoreInstance);

	if (_glfw.win32NTInstance)
		_glfwPlatformFreeModule(_glfw.win32NTInstance);
}

// Create key code translation tables
//
static void createKeyTables(void)
{
	int scancode;

	memset(_glfw.win32Keycodes, -1, sizeof(_glfw.win32Keycodes));
	memset(_glfw.scanCodes, -1, sizeof(_glfw.scanCodes));

	_glfw.win32Keycodes[0x00B] = KEY_0;
	_glfw.win32Keycodes[0x002] = KEY_1;
	_glfw.win32Keycodes[0x003] = KEY_2;
	_glfw.win32Keycodes[0x004] = KEY_3;
	_glfw.win32Keycodes[0x005] = KEY_4;
	_glfw.win32Keycodes[0x006] = KEY_5;
	_glfw.win32Keycodes[0x007] = KEY_6;
	_glfw.win32Keycodes[0x008] = KEY_7;
	_glfw.win32Keycodes[0x009] = KEY_8;
	_glfw.win32Keycodes[0x00A] = KEY_9;
	_glfw.win32Keycodes[0x01E] = KEY_A;
	_glfw.win32Keycodes[0x030] = KEY_B;
	_glfw.win32Keycodes[0x02E] = KEY_C;
	_glfw.win32Keycodes[0x020] = KEY_D;
	_glfw.win32Keycodes[0x012] = KEY_E;
	_glfw.win32Keycodes[0x021] = KEY_F;
	_glfw.win32Keycodes[0x022] = KEY_G;
	_glfw.win32Keycodes[0x023] = KEY_H;
	_glfw.win32Keycodes[0x017] = KEY_I;
	_glfw.win32Keycodes[0x024] = KEY_J;
	_glfw.win32Keycodes[0x025] = KEY_K;
	_glfw.win32Keycodes[0x026] = KEY_L;
	_glfw.win32Keycodes[0x032] = KEY_M;
	_glfw.win32Keycodes[0x031] = KEY_N;
	_glfw.win32Keycodes[0x018] = KEY_O;
	_glfw.win32Keycodes[0x019] = KEY_P;
	_glfw.win32Keycodes[0x010] = KEY_Q;
	_glfw.win32Keycodes[0x013] = KEY_R;
	_glfw.win32Keycodes[0x01F] = KEY_S;
	_glfw.win32Keycodes[0x014] = KEY_T;
	_glfw.win32Keycodes[0x016] = KEY_U;
	_glfw.win32Keycodes[0x02F] = KEY_V;
	_glfw.win32Keycodes[0x011] = KEY_W;
	_glfw.win32Keycodes[0x02D] = KEY_X;
	_glfw.win32Keycodes[0x015] = KEY_Y;
	_glfw.win32Keycodes[0x02C] = KEY_Z;

	_glfw.win32Keycodes[0x028] = KEY_APOSTROPHE;
	_glfw.win32Keycodes[0x02B] = KEY_BACKSLASH;
	_glfw.win32Keycodes[0x033] = KEY_COMMA;
	_glfw.win32Keycodes[0x00D] = KEY_EQUAL;
	_glfw.win32Keycodes[0x029] = KEY_GRAVE_ACCENT;
	_glfw.win32Keycodes[0x01A] = KEY_LEFT_BRACKET;
	_glfw.win32Keycodes[0x00C] = KEY_MINUS;
	_glfw.win32Keycodes[0x034] = KEY_PERIOD;
	_glfw.win32Keycodes[0x01B] = KEY_RIGHT_BRACKET;
	_glfw.win32Keycodes[0x027] = KEY_SEMICOLON;
	_glfw.win32Keycodes[0x035] = KEY_SLASH;
	_glfw.win32Keycodes[0x056] = KEY_WORLD_2;

	_glfw.win32Keycodes[0x00E] = KEY_BACKSPACE;
	_glfw.win32Keycodes[0x153] = KEY_DELETE;
	_glfw.win32Keycodes[0x14F] = KEY_END;
	_glfw.win32Keycodes[0x01C] = KEY_ENTER;
	_glfw.win32Keycodes[0x001] = KEY_ESCAPE;
	_glfw.win32Keycodes[0x147] = KEY_HOME;
	_glfw.win32Keycodes[0x152] = KEY_INSERT;
	_glfw.win32Keycodes[0x15D] = KEY_MENU;
	_glfw.win32Keycodes[0x151] = KEY_PAGE_DOWN;
	_glfw.win32Keycodes[0x149] = KEY_PAGE_UP;
	_glfw.win32Keycodes[0x045] = KEY_PAUSE;
	_glfw.win32Keycodes[0x039] = KEY_SPACE;
	_glfw.win32Keycodes[0x00F] = KEY_TAB;
	_glfw.win32Keycodes[0x03A] = KEY_CAPS_LOCK;
	_glfw.win32Keycodes[0x145] = KEY_NUM_LOCK;
	_glfw.win32Keycodes[0x046] = KEY_SCROLL_LOCK;
	_glfw.win32Keycodes[0x03B] = KEY_F1;
	_glfw.win32Keycodes[0x03C] = KEY_F2;
	_glfw.win32Keycodes[0x03D] = KEY_F3;
	_glfw.win32Keycodes[0x03E] = KEY_F4;
	_glfw.win32Keycodes[0x03F] = KEY_F5;
	_glfw.win32Keycodes[0x040] = KEY_F6;
	_glfw.win32Keycodes[0x041] = KEY_F7;
	_glfw.win32Keycodes[0x042] = KEY_F8;
	_glfw.win32Keycodes[0x043] = KEY_F9;
	_glfw.win32Keycodes[0x044] = KEY_F10;
	_glfw.win32Keycodes[0x057] = KEY_F11;
	_glfw.win32Keycodes[0x058] = KEY_F12;
	_glfw.win32Keycodes[0x064] = KEY_F13;
	_glfw.win32Keycodes[0x065] = KEY_F14;
	_glfw.win32Keycodes[0x066] = KEY_F15;
	_glfw.win32Keycodes[0x067] = KEY_F16;
	_glfw.win32Keycodes[0x068] = KEY_F17;
	_glfw.win32Keycodes[0x069] = KEY_F18;
	_glfw.win32Keycodes[0x06A] = KEY_F19;
	_glfw.win32Keycodes[0x06B] = KEY_F20;
	_glfw.win32Keycodes[0x06C] = KEY_F21;
	_glfw.win32Keycodes[0x06D] = KEY_F22;
	_glfw.win32Keycodes[0x06E] = KEY_F23;
	_glfw.win32Keycodes[0x076] = KEY_F24;
	_glfw.win32Keycodes[0x038] = KEY_LEFT_ALT;
	_glfw.win32Keycodes[0x01D] = KEY_LEFT_CONTROL;
	_glfw.win32Keycodes[0x02A] = KEY_LEFT_SHIFT;
	_glfw.win32Keycodes[0x15B] = KEY_LEFT_SUPER;
	_glfw.win32Keycodes[0x137] = KEY_PRINT_SCREEN;
	_glfw.win32Keycodes[0x138] = KEY_RIGHT_ALT;
	_glfw.win32Keycodes[0x11D] = KEY_RIGHT_CONTROL;
	_glfw.win32Keycodes[0x036] = KEY_RIGHT_SHIFT;
	_glfw.win32Keycodes[0x15C] = KEY_RIGHT_SUPER;
	_glfw.win32Keycodes[0x150] = KEY_DOWN;
	_glfw.win32Keycodes[0x14B] = KEY_LEFT;
	_glfw.win32Keycodes[0x14D] = KEY_RIGHT;
	_glfw.win32Keycodes[0x148] = KEY_UP;

	_glfw.win32Keycodes[0x052] = KEY_KP_0;
	_glfw.win32Keycodes[0x04F] = KEY_KP_1;
	_glfw.win32Keycodes[0x050] = KEY_KP_2;
	_glfw.win32Keycodes[0x051] = KEY_KP_3;
	_glfw.win32Keycodes[0x04B] = KEY_KP_4;
	_glfw.win32Keycodes[0x04C] = KEY_KP_5;
	_glfw.win32Keycodes[0x04D] = KEY_KP_6;
	_glfw.win32Keycodes[0x047] = KEY_KP_7;
	_glfw.win32Keycodes[0x048] = KEY_KP_8;
	_glfw.win32Keycodes[0x049] = KEY_KP_9;
	_glfw.win32Keycodes[0x04E] = KEY_KP_ADD;
	_glfw.win32Keycodes[0x053] = KEY_KP_DECIMAL;
	_glfw.win32Keycodes[0x135] = KEY_KP_DIVIDE;
	_glfw.win32Keycodes[0x11C] = KEY_KP_ENTER;
	_glfw.win32Keycodes[0x059] = KEY_KP_EQUAL;
	_glfw.win32Keycodes[0x037] = KEY_KP_MULTIPLY;
	_glfw.win32Keycodes[0x04A] = KEY_KP_SUBTRACT;

	for (scancode = 0;  scancode < 512;  scancode++)
	{
		if (_glfw.win32Keycodes[scancode] > 0)
			_glfw.scanCodes[_glfw.win32Keycodes[scancode]] = scancode;
	}
}

// Window procedure for the hidden helper window
//
static LRESULT CALLBACK helperWindowProc(HWND hWnd, UINT uMsg, WPARAM wParam, LPARAM lParam)
{
	if (uMsg == WM_DISPLAYCHANGE) {
		_glfwPollMonitorsWin32();
	}
	return DefWindowProcW(hWnd, uMsg, wParam, lParam);
}

// Creates a dummy window for behind-the-scenes work
//
static ErrorResponse* createHelperWindow(void)
{
	MSG msg;
	WNDCLASSEXW wc = { sizeof(wc) };

	wc.style         = CS_OWNDC;
	wc.lpfnWndProc   = (WNDPROC) helperWindowProc;
	wc.hInstance     = _glfw.win32Instance;
	wc.lpszClassName = L"GLFW3 Helper";

	_glfw.win32HelperWindowClass = RegisterClassExW(&wc);
	if (!_glfw.win32HelperWindowClass)
	{
		return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to register helper window class");
	}

	_glfw.win32HelperWindowHandle =
		CreateWindowExW(WS_EX_OVERLAPPEDWINDOW,
						MAKEINTATOM(_glfw.win32HelperWindowClass),
						L"GLFW message window",
						WS_CLIPSIBLINGS | WS_CLIPCHILDREN,
						0, 0, 1, 1,
						NULL, NULL,
						_glfw.win32Instance,
						NULL);

	if (!_glfw.win32HelperWindowHandle)
	{
		return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to create helper window");
	}

	// HACK: The command to the first ShowWindow call is ignored if the parent
	//       process passed along a STARTUPINFO, so clear that with a no-op call
	ShowWindow(_glfw.win32HelperWindowHandle, SW_HIDE);

	// Register for HID device notifications
	// TODO: Consider eliminating this, as we no longer need HID support, do we?
	{
		DEV_BROADCAST_DEVICEINTERFACE_W dbi;
		ZeroMemory(&dbi, sizeof(dbi));
		dbi.dbcc_size = sizeof(dbi);
		dbi.dbcc_devicetype = DBT_DEVTYP_DEVICEINTERFACE;
		dbi.dbcc_classguid = GUID_DEVINTERFACE_HID;

		_glfw.win32DeviceNotificationHandle =
			RegisterDeviceNotificationW(_glfw.win32HelperWindowHandle,
										(DEV_BROADCAST_HDR*) &dbi,
										DEVICE_NOTIFY_WINDOW_HANDLE);
	}

	while (PeekMessageW(&msg, _glfw.win32HelperWindowHandle, 0, 0, PM_REMOVE))
	{
		TranslateMessage(&msg);
		DispatchMessageW(&msg);
	}

   return NULL;
}

//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Returns a wide string version of the specified UTF-8 string
//
WCHAR* _glfwCreateWideStringFromUTF8Win32(const char* src) {
	int count = MultiByteToWideChar(CP_UTF8, 0, src, -1, NULL, 0);
	if (!count) {
		return NULL;
	}
	WCHAR* target = _glfw_calloc(count, sizeof(WCHAR));
	if (!MultiByteToWideChar(CP_UTF8, 0, src, -1, target, count)) {
		_glfw_free(target);
		return NULL;
	}
	return target;
}

// Returns a UTF-8 string version of the specified wide string
//
char* _glfwCreateUTF8FromWideStringWin32(const WCHAR* src) {
	int size = WideCharToMultiByte(CP_UTF8, 0, src, -1, NULL, 0, NULL, NULL);
	if (!size) {
		return NULL;
	}
	char* target = _glfw_calloc(size, 1);
	if (!WideCharToMultiByte(CP_UTF8, 0, src, -1, target, size, NULL, NULL)) {
		_glfw_free(target);
		return NULL;
	}
	return target;
}

// Reports the specified error, appending information about the last Win32 error
//
void _glfwInputErrorWin32(int error, const char* description)
{
	WCHAR buffer[ERROR_MSG_SIZE] = L"";
	char message[ERROR_MSG_SIZE] = "";

	FormatMessageW(FORMAT_MESSAGE_FROM_SYSTEM |
					   FORMAT_MESSAGE_IGNORE_INSERTS |
					   FORMAT_MESSAGE_MAX_WIDTH_MASK,
				   NULL,
				   GetLastError() & 0xffff,
				   MAKELANGID(LANG_NEUTRAL, SUBLANG_DEFAULT),
				   buffer,
				   sizeof(buffer) / sizeof(WCHAR),
				   NULL);
	WideCharToMultiByte(CP_UTF8, 0, buffer, -1, message, sizeof(message), NULL, NULL);

	_glfwInputError(error, "%s: %s", description, message);
}

// Replacement for IsWindowsVersionOrGreater, as we cannot rely on the
// application having a correct embedded manifest
//
BOOL _glfwIsWindowsVersionOrGreaterWin32(WORD major, WORD minor, WORD sp)
{
	OSVERSIONINFOEXW osvi = { sizeof(osvi), major, minor, 0, 0, {0}, sp };
	DWORD mask = VER_MAJORVERSION | VER_MINORVERSION | VER_SERVICEPACKMAJOR;
	ULONGLONG cond = VerSetConditionMask(0, VER_MAJORVERSION, VER_GREATER_EQUAL);
	cond = VerSetConditionMask(cond, VER_MINORVERSION, VER_GREATER_EQUAL);
	cond = VerSetConditionMask(cond, VER_SERVICEPACKMAJOR, VER_GREATER_EQUAL);
	// HACK: Use RtlVerifyVersionInfo instead of VerifyVersionInfoW as the
	//       latter lies unless the user knew to embed a non-default manifest
	//       announcing support for Windows 10 via supportedOS GUID
	return _glfw.win32NTRtlVerifyVersionInfo_(&osvi, mask, cond) == 0;
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
	return _glfw.win32NTRtlVerifyVersionInfo_(&osvi, mask, cond) == 0;
}

ErrorResponse* platformInit(void) {
	ErrorResponse* errRsp = loadLibraries();
	if (errRsp) {
		_terminate();
		return errRsp;
	}

	createKeyTables();

	if (IsWindows10Version1703OrGreater())
		_glfw.win32User32SetProcessDpiAwarenessContext_(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2);
	else {
		_glfw.win32ShCoreSetProcessDpiAwareness_(PROCESS_PER_MONITOR_DPI_AWARE);
	}

	errRsp = createHelperWindow();
	if (errRsp) {
		_terminate();
		return errRsp;
	}

	_glfwPollMonitorsWin32();
	return NULL;
}

void platformTerminate(void)
{
	if (_glfw.win32BlankCursor)
		DestroyIcon((HICON) _glfw.win32BlankCursor);

	if (_glfw.win32DeviceNotificationHandle)
		UnregisterDeviceNotification(_glfw.win32DeviceNotificationHandle);

	if (_glfw.win32HelperWindowHandle)
		DestroyWindow(_glfw.win32HelperWindowHandle);
	if (_glfw.win32HelperWindowClass)
		UnregisterClassW(MAKEINTATOM(_glfw.win32HelperWindowClass), _glfw.win32Instance);
	if (_glfw.win32MainWindowClass)
		UnregisterClassW(MAKEINTATOM(_glfw.win32MainWindowClass), _glfw.win32Instance);


	_glfwTerminateWGL();

	freeLibraries();
}

#endif // _WIN32
