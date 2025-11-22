#include "platform.h"

#if defined(_WIN32)

#include <limits.h>
#include <windowsx.h>
#include <shellapi.h>

static WCHAR* createWideStringFromUTF8(const char* src) {
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

// Returns the window style for the specified window
//
static DWORD getWindowStyle(const plafWindow* window)
{
	DWORD style = WS_CLIPSIBLINGS | WS_CLIPCHILDREN;

	if (window->monitor)
		style |= WS_POPUP;
	else
	{
		style |= WS_SYSMENU | WS_MINIMIZEBOX;

		if (window->decorated)
		{
			style |= WS_CAPTION;

			if (window->resizable)
				style |= WS_MAXIMIZEBOX | WS_THICKFRAME;
		}
		else
			style |= WS_POPUP;
	}

	return style;
}

// Returns the extended window style for the specified window
//
static DWORD getWindowExStyle(const plafWindow* window)
{
	DWORD style = WS_EX_APPWINDOW;

	if (window->monitor || window->floating)
		style |= WS_EX_TOPMOST;

	return style;
}

// Returns the image whose area most closely matches the desired one
//
static const plafImageData* chooseImage(int count, const plafImageData* images,
									int width, int height)
{
	int i, leastDiff = INT_MAX;
	const plafImageData* closest = NULL;

	for (i = 0;  i < count;  i++)
	{
		const int currDiff = abs(images[i].width * images[i].height -
								 width * height);
		if (currDiff < leastDiff)
		{
			closest = images + i;
			leastDiff = currDiff;
		}
	}

	return closest;
}

// Creates an RGBA icon or cursor
//
static HICON createIcon(const plafImageData* image, int xhot, int yhot, IntBool icon)
{
	int i;
	HDC dc;
	HICON handle;
	HBITMAP color, mask;
	BITMAPV5HEADER bi;
	ICONINFO ii;
	unsigned char* target = NULL;
	unsigned char* source = image->pixels;

	ZeroMemory(&bi, sizeof(bi));
	bi.bV5Size        = sizeof(bi);
	bi.bV5Width       = image->width;
	bi.bV5Height      = -image->height;
	bi.bV5Planes      = 1;
	bi.bV5BitCount    = 32;
	bi.bV5Compression = BI_BITFIELDS;
	bi.bV5RedMask     = 0x00ff0000;
	bi.bV5GreenMask   = 0x0000ff00;
	bi.bV5BlueMask    = 0x000000ff;
	bi.bV5AlphaMask   = 0xff000000;

	dc = GetDC(NULL);
	color = CreateDIBSection(dc,
							 (BITMAPINFO*) &bi,
							 DIB_RGB_COLORS,
							 (void**) &target,
							 NULL,
							 (DWORD) 0);
	ReleaseDC(NULL, dc);

	if (!color)
	{
		_plafInputError("Win32: Failed to create RGBA bitmap");
		return NULL;
	}

	mask = CreateBitmap(image->width, image->height, 1, 1, NULL);
	if (!mask)
	{
		_plafInputError("Win32: Failed to create mask bitmap");
		DeleteObject(color);
		return NULL;
	}

	for (i = 0;  i < image->width * image->height;  i++)
	{
		target[0] = source[2];
		target[1] = source[1];
		target[2] = source[0];
		target[3] = source[3];
		target += 4;
		source += 4;
	}

	ZeroMemory(&ii, sizeof(ii));
	ii.fIcon    = icon;
	ii.xHotspot = xhot;
	ii.yHotspot = yhot;
	ii.hbmMask  = mask;
	ii.hbmColor = color;

	handle = CreateIconIndirect(&ii);

	DeleteObject(color);
	DeleteObject(mask);

	if (!handle)
	{
		if (icon)
		{
			_plafInputError("Win32: Failed to create icon");
		}
		else
		{
			_plafInputError("Win32: Failed to create cursor");
		}
	}

	return handle;
}

// Enforce the content area aspect ratio based on which edge is being dragged
//
static void applyAspectRatio(plafWindow* window, int edge, RECT* area)
{
	RECT frame = {0};
	const float ratio = (float) window->numer / (float) window->denom;
	const DWORD style = getWindowStyle(window);
	const DWORD exStyle = getWindowExStyle(window);

	if (IsWindows10Version1607OrGreater())
	{
		_plaf.win32User32AdjustWindowRectExForDpi_(&frame, style, FALSE, exStyle,
								 _plaf.win32User32GetDpiForWindow_(window->win32Window));
	}
	else
		AdjustWindowRectEx(&frame, style, FALSE, exStyle);

	if (edge == WMSZ_LEFT  || edge == WMSZ_BOTTOMLEFT ||
		edge == WMSZ_RIGHT || edge == WMSZ_BOTTOMRIGHT)
	{
		area->bottom = area->top + (frame.bottom - frame.top) +
			(int) (((area->right - area->left) - (frame.right - frame.left)) / ratio);
	}
	else if (edge == WMSZ_TOPLEFT || edge == WMSZ_TOPRIGHT)
	{
		area->top = area->bottom - (frame.bottom - frame.top) -
			(int) (((area->right - area->left) - (frame.right - frame.left)) / ratio);
	}
	else if (edge == WMSZ_TOP || edge == WMSZ_BOTTOM)
	{
		area->right = area->left + (frame.right - frame.left) +
			(int) (((area->bottom - area->top) - (frame.bottom - frame.top)) * ratio);
	}
}

// Updates the cursor image according to its cursor mode
void _plafUpdateCursorImage(plafWindow* window) {
	if (window->cursorHidden) {
		SetCursor(_plaf.win32BlankCursor);
	} else {
		if (window->cursor) {
			SetCursor(window->cursor->win32Cursor);
		} else {
			SetCursor(LoadCursorW(NULL, IDC_ARROW));
		}
	}
}

// Update native window styles to match attributes
static void updateWindowStyles(const plafWindow* window) {
	RECT rect;
	DWORD style = GetWindowLongW(window->win32Window, GWL_STYLE);
	style &= ~(WS_OVERLAPPEDWINDOW | WS_POPUP);
	style |= getWindowStyle(window);

	GetClientRect(window->win32Window, &rect);

	if (IsWindows10Version1607OrGreater())
	{
		_plaf.win32User32AdjustWindowRectExForDpi_(&rect, style, FALSE,
								 getWindowExStyle(window),
								 _plaf.win32User32GetDpiForWindow_(window->win32Window));
	}
	else
		AdjustWindowRectEx(&rect, style, FALSE, getWindowExStyle(window));

	ClientToScreen(window->win32Window, (POINT*) &rect.left);
	ClientToScreen(window->win32Window, (POINT*) &rect.right);
	SetWindowLongW(window->win32Window, GWL_STYLE, style);
	SetWindowPos(window->win32Window, HWND_TOP,
				 rect.left, rect.top,
				 rect.right - rect.left, rect.bottom - rect.top,
				 SWP_FRAMECHANGED | SWP_NOACTIVATE | SWP_NOZORDER);
}

// Update window framebuffer transparency
static void updateFramebufferTransparency(const plafWindow* window) {
	BOOL composition;
	if (FAILED(_plaf.win32DwmIsCompositionEnabled(&composition)) || !composition) {
	   return;
	}
	HRGN region = CreateRectRgn(0, 0, -1, -1);
	DWM_BLURBEHIND bb = {0};
	bb.dwFlags = DWM_BB_ENABLE | DWM_BB_BLURREGION;
	bb.hRgnBlur = region;
	bb.fEnable = TRUE;
	_plaf.win32DwmEnableBlurBehindWindow(window->win32Window, &bb);
	DeleteObject(region);
}

// Retrieves and translates modifier keys
//
static int getKeyMods(void)
{
	int mods = 0;

	if (GetKeyState(VK_SHIFT) & 0x8000)
		mods |= KEYMOD_SHIFT;
	if (GetKeyState(VK_CONTROL) & 0x8000)
		mods |= KEYMOD_CONTROL;
	if (GetKeyState(VK_MENU) & 0x8000)
		mods |= KEYMOD_ALT;
	if ((GetKeyState(VK_LWIN) | GetKeyState(VK_RWIN)) & 0x8000)
		mods |= KEYMOD_SUPER;
	if (GetKeyState(VK_CAPITAL) & 1)
		mods |= KEYMOD_CAPS_LOCK;
	if (GetKeyState(VK_NUMLOCK) & 1)
		mods |= KEYMOD_NUM_LOCK;

	return mods;
}

static void fitToMonitor(plafWindow* window) {
	MONITORINFO mi = { sizeof(mi) };
	GetMonitorInfoW(window->monitor->win32Handle, &mi);
	SetWindowPos(window->win32Window, HWND_TOPMOST, mi.rcMonitor.left, mi.rcMonitor.top,
		mi.rcMonitor.right - mi.rcMonitor.left, mi.rcMonitor.bottom - mi.rcMonitor.top,
		SWP_NOZORDER | SWP_NOACTIVATE | SWP_NOCOPYBITS);
}

// Make the specified window and its video mode active on its monitor
static void acquireMonitor(plafWindow* window) {
	if (!_plaf.win32AcquiredMonitorCount) {
		SetThreadExecutionState(ES_CONTINUOUS | ES_DISPLAY_REQUIRED);
		SystemParametersInfoW(SPI_GETMOUSETRAILS, 0, &_plaf.win32MouseTrailSize, 0);
		SystemParametersInfoW(SPI_SETMOUSETRAILS, 0, 0, 0);
	}
	if (!window->monitor->window) {
		_plaf.win32AcquiredMonitorCount++;
	}
	_plafSetVideoMode(window->monitor, &window->videoMode);
	window->monitor->window = window;
}

// Remove the window and restore the original video mode
static void releaseMonitor(plafWindow* window) {
	if (window->monitor->window == window) {
		_plaf.win32AcquiredMonitorCount--;
		if (!_plaf.win32AcquiredMonitorCount) {
			SetThreadExecutionState(ES_CONTINUOUS);
			SystemParametersInfoW(SPI_SETMOUSETRAILS, _plaf.win32MouseTrailSize, 0, 0);
		}
		window->monitor->window = NULL;
		_plafRestoreVideoMode(window->monitor);
	}
}

// Manually maximize the window, for when SW_MAXIMIZE cannot be used
//
static void maximizeWindowManually(plafWindow* window)
{
	RECT rect;
	DWORD style;
	MONITORINFO mi = { sizeof(mi) };

	GetMonitorInfoW(MonitorFromWindow(window->win32Window,
									  MONITOR_DEFAULTTONEAREST), &mi);

	rect = mi.rcWork;

	if (window->maxwidth != DONT_CARE && window->maxheight != DONT_CARE)
	{
		rect.right = _plaf_min(rect.right, rect.left + window->maxwidth);
		rect.bottom = _plaf_min(rect.bottom, rect.top + window->maxheight);
	}

	style = GetWindowLongW(window->win32Window, GWL_STYLE);
	style |= WS_MAXIMIZE;
	SetWindowLongW(window->win32Window, GWL_STYLE, style);

	if (window->decorated)
	{
		const DWORD exStyle = GetWindowLongW(window->win32Window, GWL_EXSTYLE);

		if (IsWindows10Version1607OrGreater())
		{
			const UINT dpi = _plaf.win32User32GetDpiForWindow_(window->win32Window);
			_plaf.win32User32AdjustWindowRectExForDpi_(&rect, style, FALSE, exStyle, dpi);
			OffsetRect(&rect, 0, _plaf.win32User32GetSystemMetricsForDpi_(SM_CYCAPTION, dpi));
		}
		else
		{
			AdjustWindowRectEx(&rect, style, FALSE, exStyle);
			OffsetRect(&rect, 0, GetSystemMetrics(SM_CYCAPTION));
		}

		rect.bottom = _plaf_min(rect.bottom, mi.rcWork.bottom);
	}

	SetWindowPos(window->win32Window, HWND_TOP,
				 rect.left,
				 rect.top,
				 rect.right - rect.left,
				 rect.bottom - rect.top,
				 SWP_NOACTIVATE | SWP_NOZORDER | SWP_FRAMECHANGED);
}

// Window procedure for user-created windows
//
static LRESULT CALLBACK windowProc(HWND hWnd, UINT uMsg, WPARAM wParam, LPARAM lParam)
{
	plafWindow* window = GetPropW(hWnd, L"PLAF");
	if (!window)
	{
		if (uMsg == WM_NCCREATE)
		{
			if (IsWindows10Version1607OrGreater())
			{
				const CREATESTRUCTW* cs = (const CREATESTRUCTW*) lParam;
				const plafWindowConfig* wndconfig = cs->lpCreateParams;

				// On per-monitor DPI aware V1 systems, only enable
				// non-client scaling for windows that scale the client area
				// We need WM_GETDPISCALEDSIZE from V2 to keep the client
				// area static when the non-client area is scaled
				if (wndconfig && wndconfig->scaleToMonitor)
					_plaf.win32User32EnableNonClientDpiScaling_(hWnd);
			}
		}

		return DefWindowProcW(hWnd, uMsg, wParam, lParam);
	}

	switch (uMsg)
	{
		case WM_MOUSEACTIVATE:
		{
			// HACK: Postpone cursor disabling when the window was activated by
			//       clicking a caption button
			if (HIWORD(lParam) == WM_LBUTTONDOWN)
			{
				if (LOWORD(lParam) != HTCLIENT)
					window->win32FrameAction = true;
			}

			break;
		}

		case WM_CAPTURECHANGED:
		{
			// HACK: Disable the cursor once the caption button action has been
			//       completed or cancelled
			if (lParam == 0 && window->win32FrameAction)
			{
				window->win32FrameAction = false;
			}

			break;
		}

		case WM_SETFOCUS:
		{
			_plafInputWindowFocus(window, true);

			// HACK: Do not disable cursor while the user is interacting with
			//       a caption button
			if (window->win32FrameAction)
				break;

			return 0;
		}

		case WM_KILLFOCUS:
		{
			_plafInputWindowFocus(window, false);
			return 0;
		}

		case WM_SYSCOMMAND:
		{
			switch (wParam & 0xfff0)
			{
				case SC_SCREENSAVE:
				case SC_MONITORPOWER:
				{
					if (window->monitor)
					{
						// We are running in full screen mode, so disallow
						// screen saver and screen blanking
						return 0;
					}
					else
						break;
				}

				// User trying to access application menu using ALT?
				case SC_KEYMENU:
					return 0;
			}
			break;
		}

		case WM_CLOSE:
		{
			_plafInputWindowCloseRequest(window);
			return 0;
		}

		case WM_INPUTLANGCHANGE:
		{
			break;
		}

		case WM_CHAR:
		case WM_SYSCHAR:
		{
			if (wParam >= 0xd800 && wParam <= 0xdbff)
				window->win32HighSurrogate = (WCHAR) wParam;
			else
			{
				uint32_t codepoint = 0;

				if (wParam >= 0xdc00 && wParam <= 0xdfff)
				{
					if (window->win32HighSurrogate)
					{
						codepoint += (window->win32HighSurrogate - 0xd800) << 10;
						codepoint += (WCHAR) wParam - 0xdc00;
						codepoint += 0x10000;
					}
				}
				else
					codepoint = (WCHAR) wParam;

				window->win32HighSurrogate = 0;
				_plafInputChar(window, codepoint, getKeyMods(), uMsg != WM_SYSCHAR);
			}
			return 0;
		}

		case WM_UNICHAR:
		{
			if (wParam == UNICODE_NOCHAR)
			{
				// WM_UNICHAR is not sent by Windows, but is sent by some
				// third-party input method engine
				// Returning TRUE here announces support for this message
				return TRUE;
			}

			_plafInputChar(window, (uint32_t) wParam, getKeyMods(), true);
			return 0;
		}

		case WM_KEYDOWN:
		case WM_SYSKEYDOWN:
		case WM_KEYUP:
		case WM_SYSKEYUP:
		{
			int key, scancode;
			const int action = (HIWORD(lParam) & KF_UP) ? INPUT_RELEASE : INPUT_PRESS;
			const int mods = getKeyMods();

			scancode = (HIWORD(lParam) & (KF_EXTENDED | 0xff));
			if (!scancode)
			{
				// NOTE: Some synthetic key messages have a scancode of zero
				// HACK: Map the virtual key back to a usable scancode
				scancode = MapVirtualKeyW((UINT) wParam, MAPVK_VK_TO_VSC);
			}

			// HACK: Alt+PrtSc has a different scancode than just PrtSc
			if (scancode == 0x54)
				scancode = 0x137;

			// HACK: Ctrl+Pause has a different scancode than just Pause
			if (scancode == 0x146)
				scancode = 0x45;

			// HACK: CJK IME sets the extended bit for right Shift
			if (scancode == 0x136)
				scancode = 0x36;

			if (scancode < 0 || scancode >= MAX_KEY_CODES) {
				key = KEY_UNKNOWN;
			} else {
				key = _plaf.keyCodes[scancode];
			}

			// The Ctrl keys require special handling
			if (wParam == VK_CONTROL)
			{
				if (HIWORD(lParam) & KF_EXTENDED)
				{
					// Right side keys have the extended key bit set
					key = KEY_RIGHT_CONTROL;
				}
				else
				{
					// NOTE: Alt Gr sends Left Ctrl followed by Right Alt
					// HACK: We only want one event for Alt Gr, so if we detect
					//       this sequence we discard this Left Ctrl message now
					//       and later report Right Alt normally
					MSG next;
					const DWORD time = GetMessageTime();

					if (PeekMessageW(&next, NULL, 0, 0, PM_NOREMOVE))
					{
						if (next.message == WM_KEYDOWN ||
							next.message == WM_SYSKEYDOWN ||
							next.message == WM_KEYUP ||
							next.message == WM_SYSKEYUP)
						{
							if (next.wParam == VK_MENU &&
								(HIWORD(next.lParam) & KF_EXTENDED) &&
								next.time == time)
							{
								// Next message is Right Alt down so discard this
								break;
							}
						}
					}

					// This is a regular Left Ctrl message
					key = KEY_LEFT_CONTROL;
				}
			}
			else if (wParam == VK_PROCESSKEY)
			{
				// IME notifies that keys have been filtered by setting the
				// virtual key-code to VK_PROCESSKEY
				break;
			}

			if (action == INPUT_RELEASE && wParam == VK_SHIFT)
			{
				// HACK: Release both Shift keys on Shift up event, as when both
				//       are pressed the first release does not emit any event
				// NOTE: The other half of this is in plafPollEvents
				_plafInputKey(window, KEY_LEFT_SHIFT, scancode, action, mods);
				_plafInputKey(window, KEY_RIGHT_SHIFT, scancode, action, mods);
			}
			else if (wParam == VK_SNAPSHOT)
			{
				// HACK: Key down is not reported for the Print Screen key
				_plafInputKey(window, key, scancode, INPUT_PRESS, mods);
				_plafInputKey(window, key, scancode, INPUT_RELEASE, mods);
			}
			else
				_plafInputKey(window, key, scancode, action, mods);

			break;
		}

		case WM_LBUTTONDOWN:
		case WM_RBUTTONDOWN:
		case WM_MBUTTONDOWN:
		case WM_XBUTTONDOWN:
		case WM_LBUTTONUP:
		case WM_RBUTTONUP:
		case WM_MBUTTONUP:
		case WM_XBUTTONUP:
		{
			int i, button, action;

			if (uMsg == WM_LBUTTONDOWN || uMsg == WM_LBUTTONUP)
				button = MOUSE_BUTTON_LEFT;
			else if (uMsg == WM_RBUTTONDOWN || uMsg == WM_RBUTTONUP)
				button = MOUSE_BUTTON_RIGHT;
			else if (uMsg == WM_MBUTTONDOWN || uMsg == WM_MBUTTONUP)
				button = MOUSE_BUTTON_MIDDLE;
			else if (GET_XBUTTON_WPARAM(wParam) == XBUTTON1)
				button = MOUSE_BUTTON_4;
			else
				button = MOUSE_BUTTON_5;

			if (uMsg == WM_LBUTTONDOWN || uMsg == WM_RBUTTONDOWN ||
				uMsg == WM_MBUTTONDOWN || uMsg == WM_XBUTTONDOWN)
			{
				action = INPUT_PRESS;
			}
			else
				action = INPUT_RELEASE;

			for (i = 0;  i <= MOUSE_BUTTON_LAST;  i++)
			{
				if (window->mouseButtons[i] == INPUT_PRESS)
					break;
			}

			if (i > MOUSE_BUTTON_LAST)
				SetCapture(hWnd);

			_plafInputMouseClick(window, button, action, getKeyMods());

			for (i = 0;  i <= MOUSE_BUTTON_LAST;  i++)
			{
				if (window->mouseButtons[i] == INPUT_PRESS)
					break;
			}

			if (i > MOUSE_BUTTON_LAST)
				ReleaseCapture();

			if (uMsg == WM_XBUTTONDOWN || uMsg == WM_XBUTTONUP)
				return TRUE;

			return 0;
		}

		case WM_MOUSEMOVE:
		{
			const int x = GET_X_LPARAM(lParam);
			const int y = GET_Y_LPARAM(lParam);

			if (!window->win32CursorTracked)
			{
				TRACKMOUSEEVENT tme;
				ZeroMemory(&tme, sizeof(tme));
				tme.cbSize = sizeof(tme);
				tme.dwFlags = TME_LEAVE;
				tme.hwndTrack = window->win32Window;
				TrackMouseEvent(&tme);

				window->win32CursorTracked = true;
				_plafInputCursorEnter(window, true);
			}

			_plafInputCursorPos(window, x, y);
			return 0;
		}

		case WM_INPUT:
		{
			break;
		}

		case WM_MOUSELEAVE:
		{
			window->win32CursorTracked = false;
			_plafInputCursorEnter(window, false);
			return 0;
		}

		case WM_MOUSEWHEEL:
		{
			_plafInputScroll(window, 0.0, (SHORT) HIWORD(wParam) / (double) WHEEL_DELTA);
			return 0;
		}

		case WM_MOUSEHWHEEL:
		{
			// NOTE: The X-axis is inverted for consistency with macOS and X11
			_plafInputScroll(window, -((SHORT) HIWORD(wParam) / (double) WHEEL_DELTA), 0.0);
			return 0;
		}

		case WM_ENTERSIZEMOVE:
		case WM_ENTERMENULOOP:
		{
			if (window->win32FrameAction)
				break;

			break;
		}

		case WM_EXITSIZEMOVE:
		case WM_EXITMENULOOP:
		{
			break;
		}

		case WM_SIZE:
		{
			const int width = LOWORD(lParam);
			const int height = HIWORD(lParam);
			const IntBool minimized = wParam == SIZE_MINIMIZED;
			const IntBool maximized = wParam == SIZE_MAXIMIZED ||
									   (window->maximized &&
										wParam != SIZE_RESTORED);

			if (window->win32Minimized != minimized)
				_plafInputWindowMinimize(window, minimized);

			if (window->maximized != maximized)
				_plafInputWindowMaximize(window, maximized);

			if (width != window->width || height != window->height)
			{
				window->width = width;
				window->height = height;

				_plafInputFramebufferSize(window, width, height);
				_plafInputWindowSize(window, width, height);
			}

			if (window->monitor && window->win32Minimized != minimized)
			{
				if (minimized)
					releaseMonitor(window);
				else
				{
					acquireMonitor(window);
					fitToMonitor(window);
				}
			}

			window->win32Minimized = minimized;
			window->maximized = maximized;
			return 0;
		}

		case WM_MOVE:
		{
			// NOTE: This cannot use LOWORD/HIWORD recommended by MSDN, as
			// those macros do not handle negative window positions correctly
			_plafInputWindowPos(window,
								GET_X_LPARAM(lParam),
								GET_Y_LPARAM(lParam));
			return 0;
		}

		case WM_SIZING:
		{
			if (window->numer == DONT_CARE ||
				window->denom == DONT_CARE)
			{
				break;
			}

			applyAspectRatio(window, (int) wParam, (RECT*) lParam);
			return TRUE;
		}

		case WM_GETMINMAXINFO:
		{
			RECT frame = {0};
			MINMAXINFO* mmi = (MINMAXINFO*) lParam;
			const DWORD style = getWindowStyle(window);
			const DWORD exStyle = getWindowExStyle(window);

			if (window->monitor)
				break;

			if (IsWindows10Version1607OrGreater())
			{
				_plaf.win32User32AdjustWindowRectExForDpi_(&frame, style, FALSE, exStyle,
										 _plaf.win32User32GetDpiForWindow_(window->win32Window));
			}
			else
				AdjustWindowRectEx(&frame, style, FALSE, exStyle);

			if (window->minwidth != DONT_CARE &&
				window->minheight != DONT_CARE)
			{
				mmi->ptMinTrackSize.x = window->minwidth + frame.right - frame.left;
				mmi->ptMinTrackSize.y = window->minheight + frame.bottom - frame.top;
			}

			if (window->maxwidth != DONT_CARE &&
				window->maxheight != DONT_CARE)
			{
				mmi->ptMaxTrackSize.x = window->maxwidth + frame.right - frame.left;
				mmi->ptMaxTrackSize.y = window->maxheight + frame.bottom - frame.top;
			}

			if (!window->decorated)
			{
				MONITORINFO mi;
				const HMONITOR mh = MonitorFromWindow(window->win32Window,
													  MONITOR_DEFAULTTONEAREST);

				ZeroMemory(&mi, sizeof(mi));
				mi.cbSize = sizeof(mi);
				GetMonitorInfoW(mh, &mi);

				mmi->ptMaxPosition.x = mi.rcWork.left - mi.rcMonitor.left;
				mmi->ptMaxPosition.y = mi.rcWork.top - mi.rcMonitor.top;
				mmi->ptMaxSize.x = mi.rcWork.right - mi.rcWork.left;
				mmi->ptMaxSize.y = mi.rcWork.bottom - mi.rcWork.top;
			}

			return 0;
		}

		case WM_PAINT:
		{
			_plafInputWindowDamage(window);
			break;
		}

		case WM_ERASEBKGND:
		{
			return TRUE;
		}

		case WM_NCACTIVATE:
		case WM_NCPAINT:
		{
			// Prevent title bar from being drawn after restoring a minimized
			// undecorated window
			if (!window->decorated)
				return TRUE;

			break;
		}

		case WM_DWMCOMPOSITIONCHANGED:
		case WM_DWMCOLORIZATIONCOLORCHANGED:
		{
			if (window->win32Transparent)
				updateFramebufferTransparency(window);
			return 0;
		}

		case WM_GETDPISCALEDSIZE:
		{
			if (window->win32ScaleToMonitor)
				break;

			// Adjust the window size to keep the content area size constant
			if (IsWindows10Version1703OrGreater())
			{
				RECT source = {0}, target = {0};
				SIZE* size = (SIZE*) lParam;

				_plaf.win32User32AdjustWindowRectExForDpi_(&source, getWindowStyle(window),
										 FALSE, getWindowExStyle(window),
										 _plaf.win32User32GetDpiForWindow_(window->win32Window));
				_plaf.win32User32AdjustWindowRectExForDpi_(&target, getWindowStyle(window),
										 FALSE, getWindowExStyle(window),
										 LOWORD(wParam));

				size->cx += (target.right - target.left) -
							(source.right - source.left);
				size->cy += (target.bottom - target.top) -
							(source.bottom - source.top);
				return TRUE;
			}

			break;
		}

		case WM_DPICHANGED:
		{
			const float xscale = HIWORD(wParam) / (float) USER_DEFAULT_SCREEN_DPI;
			const float yscale = LOWORD(wParam) / (float) USER_DEFAULT_SCREEN_DPI;

			// Resize windowed mode windows that either permit rescaling or that
			// need it to compensate for non-client area scaling
			if (!window->monitor &&
				(window->win32ScaleToMonitor ||
				 IsWindows10Version1703OrGreater()))
			{
				RECT* suggested = (RECT*) lParam;
				SetWindowPos(window->win32Window, HWND_TOP,
							 suggested->left,
							 suggested->top,
							 suggested->right - suggested->left,
							 suggested->bottom - suggested->top,
							 SWP_NOACTIVATE | SWP_NOZORDER);
			}

			_plafInputWindowContentScale(window, xscale, yscale);
			break;
		}

		case WM_SETCURSOR:
		{
			if (LOWORD(lParam) == HTCLIENT)
			{
				_plafUpdateCursorImage(window);
				return TRUE;
			}

			break;
		}

		case WM_DROPFILES:
		{
			HDROP drop = (HDROP) wParam;
			POINT pt;
			int i;

			const int count = DragQueryFileW(drop, 0xffffffff, NULL, 0);
			char** paths = _plaf_calloc(count, sizeof(char*));

			// Move the mouse to the position of the drop
			DragQueryPoint(drop, &pt);
			_plafInputCursorPos(window, pt.x, pt.y);

			for (i = 0;  i < count;  i++)
			{
				const UINT length = DragQueryFileW(drop, i, NULL, 0);
				WCHAR* buffer = _plaf_calloc((size_t) length + 1, sizeof(WCHAR));

				DragQueryFileW(drop, i, buffer, length + 1);
				paths[i] = _plafCreateUTF8FromWideString(buffer);

				_plaf_free(buffer);
			}

			_plafInputDrop(window, count, (const char**) paths);

			for (i = 0;  i < count;  i++)
				_plaf_free(paths[i]);
			_plaf_free(paths);

			DragFinish(drop);
			return 0;
		}
	}

	return DefWindowProcW(hWnd, uMsg, wParam, lParam);
}

// Creates the PLAF window
static plafError* createNativeWindow(plafWindow* window, const plafWindowConfig* wndconfig, const plafFrameBufferCfg* fbconfig) {
	int frameX, frameY, frameWidth, frameHeight;
	WCHAR* wideTitle;
	DWORD style = getWindowStyle(window);
	DWORD exStyle = getWindowExStyle(window);

	if (!_plaf.win32MainWindowClass) {
		WNDCLASSEXW wc = { sizeof(wc) };
		wc.style         = CS_HREDRAW | CS_VREDRAW | CS_OWNDC;
		wc.lpfnWndProc   = windowProc;
		wc.hInstance     = _plaf.win32Instance;
		wc.hCursor       = LoadCursorW(NULL, IDC_ARROW);
		wc.lpszClassName = L"Unison";
		// Load user-provided icon if available
		wc.hIcon = LoadImageW(GetModuleHandleW(NULL), L"PLAF_ICON", IMAGE_ICON, 0, 0, LR_DEFAULTSIZE | LR_SHARED);
		if (!wc.hIcon) {
			// No user-provided icon found, load default icon
			wc.hIcon = LoadImageW(NULL, IDI_APPLICATION, IMAGE_ICON, 0, 0, LR_DEFAULTSIZE | LR_SHARED);
		}
		_plaf.win32MainWindowClass = RegisterClassExW(&wc);
		if (!_plaf.win32MainWindowClass) {
			return _plafNewError("Win32: Failed to register window class");
		}
	}

	if (GetSystemMetrics(SM_REMOTESESSION)) {
		// NOTE: On Remote Desktop, setting the cursor to NULL does not hide it
		// HACK: Create a transparent cursor and always set that instead of NULL
		//       When not on Remote Desktop, this handle is NULL and normal hiding is used
		if (!_plaf.win32BlankCursor) {
			const int cursorWidth = GetSystemMetrics(SM_CXCURSOR);
			const int cursorHeight = GetSystemMetrics(SM_CYCURSOR);
			unsigned char* cursorPixels = _plaf_calloc(cursorWidth * cursorHeight, 4);
			if (!cursorPixels) {
				return _plafNewError("Win32: Failed to allocate blank cursor pixels");
			}

			// NOTE: Windows checks whether the image is fully transparent and if so
			//       just ignores the alpha channel and makes the whole cursor opaque
			// HACK: Make one pixel slightly less transparent
			cursorPixels[3] = 1;

			const plafImageData cursorImage = { cursorWidth, cursorHeight, cursorPixels };
			_plaf.win32BlankCursor = createIcon(&cursorImage, 0, 0, FALSE);
			_plaf_free(cursorPixels);

			if (!_plaf.win32BlankCursor) {
				return _plafNewError("Win32: Failed to create blank cursor");
			}
		}
	}

	if (window->monitor) {
		MONITORINFO mi = { sizeof(mi) };
		GetMonitorInfoW(window->monitor->win32Handle, &mi);

		// NOTE: This window placement is temporary and approximate, as the
		//       correct position and size cannot be known until the monitor
		//       video mode has been picked in _plafSetVideoModeWin32
		frameX = mi.rcMonitor.left;
		frameY = mi.rcMonitor.top;
		frameWidth  = mi.rcMonitor.right - mi.rcMonitor.left;
		frameHeight = mi.rcMonitor.bottom - mi.rcMonitor.top;
	} else {
		RECT rect = { 0, 0, wndconfig->width, wndconfig->height };

		window->maximized = wndconfig->maximized;
		if (wndconfig->maximized) {
			style |= WS_MAXIMIZE;
		}

		AdjustWindowRectEx(&rect, style, FALSE, exStyle);

		if (wndconfig->xpos == ANY_POSITION && wndconfig->ypos == ANY_POSITION) {
			frameX = CW_USEDEFAULT;
			frameY = CW_USEDEFAULT;
		} else {
			frameX = wndconfig->xpos + rect.left;
			frameY = wndconfig->ypos + rect.top;
		}
		frameWidth  = rect.right - rect.left;
		frameHeight = rect.bottom - rect.top;
	}

	wideTitle = createWideStringFromUTF8(window->title);
	if (!wideTitle) {
		return _plafNewError("Win32: Failed to allocate title");
	}

	window->win32Window = CreateWindowExW(exStyle, MAKEINTATOM(_plaf.win32MainWindowClass), wideTitle, style, frameX,
		frameY, frameWidth, frameHeight, NULL, NULL, _plaf.win32Instance, (LPVOID) wndconfig);

	_plaf_free(wideTitle);

	if (!window->win32Window) {
		return _plafNewError("Win32: Failed to create window");
	}

	SetPropW(window->win32Window, L"PLAF", window);
	ChangeWindowMessageFilterEx(window->win32Window, WM_DROPFILES, MSGFLT_ALLOW, NULL);
	ChangeWindowMessageFilterEx(window->win32Window, WM_COPYDATA, MSGFLT_ALLOW, NULL);
	ChangeWindowMessageFilterEx(window->win32Window, WM_COPYGLOBALDATA, MSGFLT_ALLOW, NULL);

	window->win32ScaleToMonitor = wndconfig->scaleToMonitor;

	if (!window->monitor) {
		RECT rect = { 0, 0, wndconfig->width, wndconfig->height };
		WINDOWPLACEMENT wp = { sizeof(wp) };
		const HMONITOR mh = MonitorFromWindow(window->win32Window, MONITOR_DEFAULTTONEAREST);

		// Adjust window rect to account for DPI scaling of the window frame and
		// (if enabled) DPI scaling of the content area
		// This cannot be done until we know what monitor the window was placed on
		// Only update the restored window rect as the window may be maximized
		if (wndconfig->scaleToMonitor) {
			float xscale, yscale;
			_plafGetHMONITORContentScale(mh, &xscale, &yscale);
			if (xscale > 0.f && yscale > 0.f) {
				rect.right = (int) (rect.right * xscale);
				rect.bottom = (int) (rect.bottom * yscale);
			}
		}

		if (IsWindows10Version1607OrGreater()) {
			_plaf.win32User32AdjustWindowRectExForDpi_(&rect, style, FALSE, exStyle,
				_plaf.win32User32GetDpiForWindow_(window->win32Window));
		} else {
			AdjustWindowRectEx(&rect, style, FALSE, exStyle);
		}

		GetWindowPlacement(window->win32Window, &wp);
		OffsetRect(&rect, wp.rcNormalPosition.left - rect.left, wp.rcNormalPosition.top - rect.top);

		wp.rcNormalPosition = rect;
		wp.showCmd = SW_HIDE;
		SetWindowPlacement(window->win32Window, &wp);

		// Adjust rect of maximized undecorated window, because by default Windows will
		// make such a window cover the whole monitor instead of its workarea

		if (wndconfig->maximized && !wndconfig->decorated) {
			MONITORINFO mi = { sizeof(mi) };
			GetMonitorInfoW(mh, &mi);
			SetWindowPos(window->win32Window, HWND_TOP, mi.rcWork.left, mi.rcWork.top,
				mi.rcWork.right - mi.rcWork.left, mi.rcWork.bottom - mi.rcWork.top, SWP_NOACTIVATE | SWP_NOZORDER);
		}
	}

	DragAcceptFiles(window->win32Window, TRUE);

	if (fbconfig->transparent) {
		updateFramebufferTransparency(window);
		window->win32Transparent = true;
	}

	_plafGetWindowSize(window, &window->width, &window->height);
	return NULL;
}

plafError* _plafCreateWindow(plafWindow* window, const plafWindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig) {
	plafError* err = createNativeWindow(window, wndconfig, fbconfig);
	if (err) {
		return err;
	}

	err = _plafInitOpenGL();
	if (err) {
		return err;
	}

	err = _plafCreateOpenGLContext(window, ctxconfig, fbconfig);
	if (err) {
		return err;
	}

	err = _plafRefreshContextAttribs(window, ctxconfig);
	if (err) {
		return err;
	}

	if (wndconfig->mousePassthrough) {
		_plafSetWindowMousePassthrough(window, true);
	}

	if (window->monitor) {
		_plafShowWindow(window);
		plafFocusWindow(window);
		acquireMonitor(window);
		fitToMonitor(window);
	}
	return err;
}

void _plafDestroyWindow(plafWindow* window) {
	if (window->monitor)
		releaseMonitor(window);

	if (window->context.destroy)
		window->context.destroy(window);

	if (window->win32Window)
	{
		RemovePropW(window->win32Window, L"PLAF");
		DestroyWindow(window->win32Window);
		window->win32Window = NULL;
	}

	if (window->win32BigIcon)
		DestroyIcon(window->win32BigIcon);

	if (window->win32SmallIcon)
		DestroyIcon(window->win32SmallIcon);
}

void _plafSetWindowTitle(plafWindow* window, const char* title) {
	WCHAR* wideTitle = createWideStringFromUTF8(title);
	if (!wideTitle)
		return;

	SetWindowTextW(window->win32Window, wideTitle);
	_plaf_free(wideTitle);
}

void _plafSetWindowIcon(plafWindow* window, int count, const plafImageData* images) {
	HICON bigIcon = NULL, smallIcon = NULL;

	if (count)
	{
		const plafImageData* bigImage = chooseImage(count, images,
												GetSystemMetrics(SM_CXICON),
												GetSystemMetrics(SM_CYICON));
		const plafImageData* smallImage = chooseImage(count, images,
												  GetSystemMetrics(SM_CXSMICON),
												  GetSystemMetrics(SM_CYSMICON));

		bigIcon = createIcon(bigImage, 0, 0, true);
		smallIcon = createIcon(smallImage, 0, 0, true);
	}
	else
	{
		bigIcon = (HICON) GetClassLongPtrW(window->win32Window, GCLP_HICON);
		smallIcon = (HICON) GetClassLongPtrW(window->win32Window, GCLP_HICONSM);
	}

	SendMessageW(window->win32Window, WM_SETICON, ICON_BIG, (LPARAM) bigIcon);
	SendMessageW(window->win32Window, WM_SETICON, ICON_SMALL, (LPARAM) smallIcon);

	if (window->win32BigIcon)
		DestroyIcon(window->win32BigIcon);

	if (window->win32SmallIcon)
		DestroyIcon(window->win32SmallIcon);

	if (count)
	{
		window->win32BigIcon = bigIcon;
		window->win32SmallIcon = smallIcon;
	}
}

void _plafGetWindowPos(plafWindow* window, int* xpos, int* ypos) {
	POINT pos = { 0, 0 };
	ClientToScreen(window->win32Window, &pos);

	if (xpos)
		*xpos = pos.x;
	if (ypos)
		*ypos = pos.y;
}

void _plafSetWindowPos(plafWindow* window, int x, int y) {
	RECT rect = { x, y, x, y };

	if (IsWindows10Version1607OrGreater())
	{
		_plaf.win32User32AdjustWindowRectExForDpi_(&rect, getWindowStyle(window),
								 FALSE, getWindowExStyle(window),
								 _plaf.win32User32GetDpiForWindow_(window->win32Window));
	}
	else
	{
		AdjustWindowRectEx(&rect, getWindowStyle(window),
						   FALSE, getWindowExStyle(window));
	}

	SetWindowPos(window->win32Window, NULL, rect.left, rect.top, 0, 0,
				 SWP_NOACTIVATE | SWP_NOZORDER | SWP_NOSIZE);
}

void _plafGetWindowSize(plafWindow* window, int* width, int* height) {
	RECT area;
	GetClientRect(window->win32Window, &area);

	if (width)
		*width = area.right;
	if (height)
		*height = area.bottom;
}

void _plafSetWindowSize(plafWindow* window, int width, int height) {
	if (window->monitor)
	{
		if (window->monitor->window == window)
		{
			acquireMonitor(window);
			fitToMonitor(window);
		}
	}
	else
	{
		RECT rect = { 0, 0, width, height };

		if (IsWindows10Version1607OrGreater())
		{
			_plaf.win32User32AdjustWindowRectExForDpi_(&rect, getWindowStyle(window),
									 FALSE, getWindowExStyle(window),
									 _plaf.win32User32GetDpiForWindow_(window->win32Window));
		}
		else
		{
			AdjustWindowRectEx(&rect, getWindowStyle(window),
							   FALSE, getWindowExStyle(window));
		}

		SetWindowPos(window->win32Window, HWND_TOP,
					 0, 0, rect.right - rect.left, rect.bottom - rect.top,
					 SWP_NOACTIVATE | SWP_NOOWNERZORDER | SWP_NOMOVE | SWP_NOZORDER);
	}
}

void _plafSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight) {
	RECT area;

	if ((minwidth == DONT_CARE || minheight == DONT_CARE) &&
		(maxwidth == DONT_CARE || maxheight == DONT_CARE))
	{
		return;
	}

	GetWindowRect(window->win32Window, &area);
	MoveWindow(window->win32Window,
			   area.left, area.top,
			   area.right - area.left,
			   area.bottom - area.top, TRUE);
}

void _plafGetFramebufferSize(plafWindow* window, int* width, int* height) {
	_plafGetWindowSize(window, width, height);
}

void _plafGetWindowFrameSize(plafWindow* window, int* left, int* top, int* right, int* bottom) {
	RECT rect;
	int width, height;

	_plafGetWindowSize(window, &width, &height);
	SetRect(&rect, 0, 0, width, height);

	if (IsWindows10Version1607OrGreater())
	{
		_plaf.win32User32AdjustWindowRectExForDpi_(&rect, getWindowStyle(window),
								 FALSE, getWindowExStyle(window),
								 _plaf.win32User32GetDpiForWindow_(window->win32Window));
	}
	else
	{
		AdjustWindowRectEx(&rect, getWindowStyle(window),
						   FALSE, getWindowExStyle(window));
	}

	if (left)
		*left = -rect.left;
	if (top)
		*top = -rect.top;
	if (right)
		*right = rect.right - width;
	if (bottom)
		*bottom = rect.bottom - height;
}

void _plafGetWindowContentScale(plafWindow* window, float* xscale, float* yscale) {
	const HANDLE handle = MonitorFromWindow(window->win32Window, MONITOR_DEFAULTTONEAREST);
	_plafGetHMONITORContentScale(handle, xscale, yscale);
}

void plafMinimizeWindow(plafWindow* window) {
	ShowWindow(window->win32Window, SW_MINIMIZE);
}

void plafRestoreWindow(plafWindow* window) {
	ShowWindow(window->win32Window, SW_RESTORE);
}

void _plafMaximizeWindow(plafWindow* window) {
	if (IsWindowVisible(window->win32Window))
		ShowWindow(window->win32Window, SW_MAXIMIZE);
	else
		maximizeWindowManually(window);
}

void _plafShowWindow(plafWindow* window) {
	ShowWindow(window->win32Window, SW_SHOWNA);
}

void _plafHideWindow(plafWindow* window) {
	ShowWindow(window->win32Window, SW_HIDE);
}

void plafRequestWindowAttention(plafWindow* window) {
	FlashWindow(window->win32Window, TRUE);
}

void plafFocusWindow(plafWindow* window) {
	BringWindowToTop(window->win32Window);
	SetForegroundWindow(window->win32Window);
	SetFocus(window->win32Window);
}

void _plafSetWindowMonitor(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate) {
	if (window->monitor == monitor)
	{
		if (monitor)
		{
			if (monitor->window == window)
			{
				acquireMonitor(window);
				fitToMonitor(window);
			}
		}
		else
		{
			RECT rect = { xpos, ypos, xpos + width, ypos + height };

			if (IsWindows10Version1607OrGreater())
			{
				_plaf.win32User32AdjustWindowRectExForDpi_(&rect, getWindowStyle(window),
										 FALSE, getWindowExStyle(window),
										 _plaf.win32User32GetDpiForWindow_(window->win32Window));
			}
			else
			{
				AdjustWindowRectEx(&rect, getWindowStyle(window),
								   FALSE, getWindowExStyle(window));
			}

			SetWindowPos(window->win32Window, HWND_TOP,
						 rect.left, rect.top,
						 rect.right - rect.left, rect.bottom - rect.top,
						 SWP_NOCOPYBITS | SWP_NOACTIVATE | SWP_NOZORDER);
		}

		return;
	}

	if (window->monitor) {
		releaseMonitor(window);
	}
	window->monitor = monitor;

	if (window->monitor)
	{
		MONITORINFO mi = { sizeof(mi) };
		UINT flags = SWP_SHOWWINDOW | SWP_NOACTIVATE | SWP_NOCOPYBITS;

		if (window->decorated)
		{
			DWORD style = GetWindowLongW(window->win32Window, GWL_STYLE);
			style &= ~WS_OVERLAPPEDWINDOW;
			style |= getWindowStyle(window);
			SetWindowLongW(window->win32Window, GWL_STYLE, style);
			flags |= SWP_FRAMECHANGED;
		}

		acquireMonitor(window);

		GetMonitorInfoW(window->monitor->win32Handle, &mi);
		SetWindowPos(window->win32Window, HWND_TOPMOST,
					 mi.rcMonitor.left,
					 mi.rcMonitor.top,
					 mi.rcMonitor.right - mi.rcMonitor.left,
					 mi.rcMonitor.bottom - mi.rcMonitor.top,
					 flags);
	}
	else
	{
		HWND after;
		RECT rect = { xpos, ypos, xpos + width, ypos + height };
		DWORD style = GetWindowLongW(window->win32Window, GWL_STYLE);
		UINT flags = SWP_NOACTIVATE | SWP_NOCOPYBITS;

		if (window->decorated)
		{
			style &= ~WS_POPUP;
			style |= getWindowStyle(window);
			SetWindowLongW(window->win32Window, GWL_STYLE, style);

			flags |= SWP_FRAMECHANGED;
		}

		if (window->floating)
			after = HWND_TOPMOST;
		else
			after = HWND_NOTOPMOST;

		if (IsWindows10Version1607OrGreater())
		{
			_plaf.win32User32AdjustWindowRectExForDpi_(&rect, getWindowStyle(window),
									 FALSE, getWindowExStyle(window),
									 _plaf.win32User32GetDpiForWindow_(window->win32Window));
		}
		else
		{
			AdjustWindowRectEx(&rect, getWindowStyle(window),
							   FALSE, getWindowExStyle(window));
		}

		SetWindowPos(window->win32Window, after,
					 rect.left, rect.top,
					 rect.right - rect.left, rect.bottom - rect.top,
					 flags);
	}
}

bool plafIsWindowFocused(plafWindow* window) {
	return window->win32Window == GetActiveWindow();
}

IntBool _plafWindowMinimized(plafWindow* window) {
	return IsIconic(window->win32Window);
}

IntBool _plafWindowVisible(plafWindow* window) {
	return IsWindowVisible(window->win32Window);
}

IntBool _plafWindowMaximized(plafWindow* window) {
	return IsZoomed(window->win32Window);
}

IntBool _plafWindowHovered(plafWindow* window) {
	return _plafCursorInContentArea(window);
}

IntBool _plafFramebufferTransparent(plafWindow* window) {
	BOOL composition;
	if (!window->win32Transparent) {
		return false;
	}
	if (FAILED(_plaf.win32DwmIsCompositionEnabled(&composition)) || !composition) {
		return false;
	}
	return true;
}

void _plafSetWindowResizable(plafWindow* window, IntBool enabled) {
	updateWindowStyles(window);
}

void _plafSetWindowDecorated(plafWindow* window, IntBool enabled) {
	updateWindowStyles(window);
}

void _plafSetWindowFloating(plafWindow* window, IntBool enabled) {
	const HWND after = enabled ? HWND_TOPMOST : HWND_NOTOPMOST;
	SetWindowPos(window->win32Window, after, 0, 0, 0, 0,
				 SWP_NOACTIVATE | SWP_NOMOVE | SWP_NOSIZE);
}

void _plafSetWindowMousePassthrough(plafWindow* window, IntBool enabled) {
	COLORREF key = 0;
	BYTE alpha = 0;
	DWORD flags = 0;
	DWORD exStyle = GetWindowLongW(window->win32Window, GWL_EXSTYLE);

	if (exStyle & WS_EX_LAYERED)
		GetLayeredWindowAttributes(window->win32Window, &key, &alpha, &flags);

	if (enabled)
		exStyle |= (WS_EX_TRANSPARENT | WS_EX_LAYERED);
	else
	{
		exStyle &= ~WS_EX_TRANSPARENT;
		// NOTE: Window opacity also needs the layered window style so do not
		//       remove it if the window is alpha blended
		if (exStyle & WS_EX_LAYERED)
		{
			if (!(flags & LWA_ALPHA))
				exStyle &= ~WS_EX_LAYERED;
		}
	}

	SetWindowLongW(window->win32Window, GWL_EXSTYLE, exStyle);

	if (enabled)
		SetLayeredWindowAttributes(window->win32Window, key, alpha, flags);
}

float plafGetWindowOpacity(plafWindow* window) {
	BYTE alpha;
	DWORD flags;

	if ((GetWindowLongW(window->win32Window, GWL_EXSTYLE) & WS_EX_LAYERED) &&
		GetLayeredWindowAttributes(window->win32Window, NULL, &alpha, &flags))
	{
		if (flags & LWA_ALPHA)
			return alpha / 255.f;
	}

	return 1.f;
}

void _plafSetWindowOpacity(plafWindow* window, float opacity) {
	LONG exStyle = GetWindowLongW(window->win32Window, GWL_EXSTYLE);
	if (opacity < 1.f || (exStyle & WS_EX_TRANSPARENT))
	{
		const BYTE alpha = (BYTE) (255 * opacity);
		exStyle |= WS_EX_LAYERED;
		SetWindowLongW(window->win32Window, GWL_EXSTYLE, exStyle);
		SetLayeredWindowAttributes(window->win32Window, 0, alpha, LWA_ALPHA);
	}
	else if (exStyle & WS_EX_TRANSPARENT)
	{
		SetLayeredWindowAttributes(window->win32Window, 0, 0, 0);
	}
	else
	{
		exStyle &= ~WS_EX_LAYERED;
		SetWindowLongW(window->win32Window, GWL_EXSTYLE, exStyle);
	}
}

void plafPollEvents(void) {
	MSG msg;
	HWND handle;
	plafWindow* window;

	while (PeekMessageW(&msg, NULL, 0, 0, PM_REMOVE))
	{
		if (msg.message == WM_QUIT)
		{
			// NOTE: While PLAF does not itself post WM_QUIT, other processes
			//       may post it to this one, for example Task Manager
			// HACK: Treat WM_QUIT as a close on all windows

			window = _plaf.windowListHead;
			while (window)
			{
				_plafInputWindowCloseRequest(window);
				window = window->next;
			}
		}
		else
		{
			TranslateMessage(&msg);
			DispatchMessageW(&msg);
		}
	}

	// HACK: Release modifier keys that the system did not emit KEYUP for
	// NOTE: Shift keys on Windows tend to "stick" when both are pressed as
	//       no key up message is generated by the first key release
	// NOTE: Windows key is not reported as released by the Win+V hotkey
	//       Other Win hotkeys are handled implicitly by _plafInputWindowFocus
	//       because they change the input focus
	// NOTE: The other half of this is in the WM_*KEY* handler in windowProc
	handle = GetActiveWindow();
	if (handle)
	{
		window = GetPropW(handle, L"PLAF");
		if (window)
		{
			int i;
			const int keys[4][2] =
			{
				{ VK_LSHIFT, KEY_LEFT_SHIFT },
				{ VK_RSHIFT, KEY_RIGHT_SHIFT },
				{ VK_LWIN, KEY_LEFT_SUPER },
				{ VK_RWIN, KEY_RIGHT_SUPER }
			};

			for (i = 0;  i < 4;  i++)
			{
				const int vk = keys[i][0];
				const int key = keys[i][1];
				const int scancode = _plaf.scanCodes[key];

				if ((GetKeyState(vk) & 0x8000))
					continue;
				if (window->keys[key] != INPUT_PRESS)
					continue;

				_plafInputKey(window, key, scancode, INPUT_RELEASE, getKeyMods());
			}
		}
	}
}

void plafWaitEvents(void) {
	WaitMessage();
	plafPollEvents();
}

void _plafWaitEventsTimeout(double timeout) {
	MsgWaitForMultipleObjects(0, NULL, FALSE, (DWORD) (timeout * 1e3), QS_ALLINPUT);
	plafPollEvents();
}

void plafPostEmptyEvent(void) {
	PostMessageW(_plaf.win32HelperWindowHandle, WM_NULL, 0, 0);
}

void _plafUpdateCursor(plafWindow* window) {
	if (_plafCursorInContentArea(window)) {
		_plafUpdateCursorImage(window);
	}
}

IntBool _plafCreateCursor(plafCursor* cursor, const plafImageData* image, int xhot, int yhot) {
	cursor->win32Cursor = (HCURSOR) createIcon(image, xhot, yhot, false);
	if (!cursor->win32Cursor)
		return false;

	return true;
}

IntBool _plafCreateStandardCursor(plafCursor* cursor, int shape) {
	int id = 0;

	switch (shape)
	{
		case STD_CURSOR_ARROW:
			id = OCR_NORMAL;
			break;
		case STD_CURSOR_IBEAM:
			id = OCR_IBEAM;
			break;
		case STD_CURSOR_CROSSHAIR:
			id = OCR_CROSS;
			break;
		case STD_CURSOR_POINTING_HAND:
			id = OCR_HAND;
			break;
		case STD_CURSOR_HORIZONTAL_RESIZE:
			id = OCR_SIZEWE;
			break;
		case STD_CURSOR_VERTICAL_RESIZE:
			id = OCR_SIZENS;
			break;
		default:
			_plafInputError("Win32: Unknown standard cursor");
			return false;
	}

	cursor->win32Cursor = LoadImageW(NULL,
									  MAKEINTRESOURCEW(id), IMAGE_CURSOR, 0, 0,
									  LR_DEFAULTSIZE | LR_SHARED);
	if (!cursor->win32Cursor)
	{
		_plafInputError("Win32: Failed to create standard cursor");
		return false;
	}

	return true;
}

void _plafDestroyCursor(plafCursor* cursor) {
	if (cursor->win32Cursor)
		DestroyIcon((HICON) cursor->win32Cursor);
}

void* plafGetNativeWindow(plafWindow* window) {
	return window->win32Window;
}

#endif // _WIN32
