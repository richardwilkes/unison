#include "platform.h"

#if defined(_WIN32)

#include <limits.h>
#include <windowsx.h>
#include <shellapi.h>

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
static const ImageData* chooseImage(int count, const ImageData* images,
									int width, int height)
{
	int i, leastDiff = INT_MAX;
	const ImageData* closest = NULL;

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
static HICON createIcon(const ImageData* image, int xhot, int yhot, IntBool icon)
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
		_glfwInputErrorWin32(ERR_PLATFORM_ERROR, "Win32: Failed to create RGBA bitmap");
		return NULL;
	}

	mask = CreateBitmap(image->width, image->height, 1, 1, NULL);
	if (!mask)
	{
		_glfwInputErrorWin32(ERR_PLATFORM_ERROR, "Win32: Failed to create mask bitmap");
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
			_glfwInputErrorWin32(ERR_PLATFORM_ERROR, "Win32: Failed to create icon");
		}
		else
		{
			_glfwInputErrorWin32(ERR_PLATFORM_ERROR, "Win32: Failed to create cursor");
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
		_glfw.win32User32AdjustWindowRectExForDpi_(&frame, style, FALSE, exStyle,
								 _glfw.win32User32GetDpiForWindow_(window->win32Window));
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
//
void updateCursorImage(plafWindow* window)
{
	if (window->cursorMode == CURSOR_NORMAL)
	{
		if (window->cursor)
			SetCursor(window->cursor->win32Cursor);
		else
			SetCursor(LoadCursorW(NULL, IDC_ARROW));
	}
	else
	{
		// NOTE: Via Remote Desktop, setting the cursor to NULL does not hide it.
		// HACK: When running locally, it is set to NULL, but when connected via Remote
		//       Desktop, this is a transparent cursor.
		SetCursor(_glfw.win32BlankCursor);
	}
}

// Update native window styles to match attributes
//
static void updateWindowStyles(const plafWindow* window)
{
	RECT rect;
	DWORD style = GetWindowLongW(window->win32Window, GWL_STYLE);
	style &= ~(WS_OVERLAPPEDWINDOW | WS_POPUP);
	style |= getWindowStyle(window);

	GetClientRect(window->win32Window, &rect);

	if (IsWindows10Version1607OrGreater())
	{
		_glfw.win32User32AdjustWindowRectExForDpi_(&rect, style, FALSE,
								 getWindowExStyle(window),
								 _glfw.win32User32GetDpiForWindow_(window->win32Window));
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
	if (FAILED(_glfw.win32DwmIsCompositionEnabled(&composition)) || !composition) {
	   return;
	}
	HRGN region = CreateRectRgn(0, 0, -1, -1);
	DWM_BLURBEHIND bb = {0};
	bb.dwFlags = DWM_BB_ENABLE | DWM_BB_BLURREGION;
	bb.hRgnBlur = region;
	bb.fEnable = TRUE;
	_glfw.win32DwmEnableBlurBehindWindow(window->win32Window, &bb);
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

static void fitToMonitor(plafWindow* window)
{
	MONITORINFO mi = { sizeof(mi) };
	GetMonitorInfoW(window->monitor->win32Handle, &mi);
	SetWindowPos(window->win32Window, HWND_TOPMOST,
				 mi.rcMonitor.left,
				 mi.rcMonitor.top,
				 mi.rcMonitor.right - mi.rcMonitor.left,
				 mi.rcMonitor.bottom - mi.rcMonitor.top,
				 SWP_NOZORDER | SWP_NOACTIVATE | SWP_NOCOPYBITS);
}

// Make the specified window and its video mode active on its monitor
//
static void acquireMonitor(plafWindow* window)
{
	if (!_glfw.win32AcquiredMonitorCount)
	{
		SetThreadExecutionState(ES_CONTINUOUS | ES_DISPLAY_REQUIRED);

		// HACK: When mouse trails are enabled the cursor becomes invisible when
		//       the OpenGL ICD switches to page flipping
		SystemParametersInfoW(SPI_GETMOUSETRAILS, 0, &_glfw.win32MouseTrailSize, 0);
		SystemParametersInfoW(SPI_SETMOUSETRAILS, 0, 0, 0);
	}

	if (!window->monitor->window)
		_glfw.win32AcquiredMonitorCount++;

	_glfwSetVideoMode(window->monitor, &window->videoMode);
	_glfwInputMonitorWindow(window->monitor, window);
}

// Remove the window and restore the original video mode
//
static void releaseMonitor(plafWindow* window)
{
	if (window->monitor->window != window)
		return;

	_glfw.win32AcquiredMonitorCount--;
	if (!_glfw.win32AcquiredMonitorCount)
	{
		SetThreadExecutionState(ES_CONTINUOUS);

		// HACK: Restore mouse trail length saved in acquireMonitor
		SystemParametersInfoW(SPI_SETMOUSETRAILS, _glfw.win32MouseTrailSize, 0, 0);
	}

	_glfwInputMonitorWindow(window->monitor, NULL);
	_glfwRestoreVideoModeWin32(window->monitor);
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
		rect.right = _glfw_min(rect.right, rect.left + window->maxwidth);
		rect.bottom = _glfw_min(rect.bottom, rect.top + window->maxheight);
	}

	style = GetWindowLongW(window->win32Window, GWL_STYLE);
	style |= WS_MAXIMIZE;
	SetWindowLongW(window->win32Window, GWL_STYLE, style);

	if (window->decorated)
	{
		const DWORD exStyle = GetWindowLongW(window->win32Window, GWL_EXSTYLE);

		if (IsWindows10Version1607OrGreater())
		{
			const UINT dpi = _glfw.win32User32GetDpiForWindow_(window->win32Window);
			_glfw.win32User32AdjustWindowRectExForDpi_(&rect, style, FALSE, exStyle, dpi);
			OffsetRect(&rect, 0, _glfw.win32User32GetSystemMetricsForDpi_(SM_CYCAPTION, dpi));
		}
		else
		{
			AdjustWindowRectEx(&rect, style, FALSE, exStyle);
			OffsetRect(&rect, 0, GetSystemMetrics(SM_CYCAPTION));
		}

		rect.bottom = _glfw_min(rect.bottom, mi.rcWork.bottom);
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
	plafWindow* window = GetPropW(hWnd, L"GLFW");
	if (!window)
	{
		if (uMsg == WM_NCCREATE)
		{
			if (IsWindows10Version1607OrGreater())
			{
				const CREATESTRUCTW* cs = (const CREATESTRUCTW*) lParam;
				const WindowConfig* wndconfig = cs->lpCreateParams;

				// On per-monitor DPI aware V1 systems, only enable
				// non-client scaling for windows that scale the client area
				// We need WM_GETDPISCALEDSIZE from V2 to keep the client
				// area static when the non-client area is scaled
				if (wndconfig && wndconfig->scaleToMonitor)
					_glfw.win32User32EnableNonClientDpiScaling_(hWnd);
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
			_glfwInputWindowFocus(window, true);

			// HACK: Do not disable cursor while the user is interacting with
			//       a caption button
			if (window->win32FrameAction)
				break;

			return 0;
		}

		case WM_KILLFOCUS:
		{
			_glfwInputWindowFocus(window, false);
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
			_glfwInputWindowCloseRequest(window);
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
				_glfwInputChar(window, codepoint, getKeyMods(), uMsg != WM_SYSCHAR);
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

			_glfwInputChar(window, (uint32_t) wParam, getKeyMods(), true);
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

			key = _glfw.win32Keycodes[scancode];

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
				// NOTE: The other half of this is in glfwPollEvents
				_glfwInputKey(window, KEY_LEFT_SHIFT, scancode, action, mods);
				_glfwInputKey(window, KEY_RIGHT_SHIFT, scancode, action, mods);
			}
			else if (wParam == VK_SNAPSHOT)
			{
				// HACK: Key down is not reported for the Print Screen key
				_glfwInputKey(window, key, scancode, INPUT_PRESS, mods);
				_glfwInputKey(window, key, scancode, INPUT_RELEASE, mods);
			}
			else
				_glfwInputKey(window, key, scancode, action, mods);

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

			_glfwInputMouseClick(window, button, action, getKeyMods());

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
				_glfwInputCursorEnter(window, true);
			}

			_glfwInputCursorPos(window, x, y);
			return 0;
		}

		case WM_INPUT:
		{
			break;
		}

		case WM_MOUSELEAVE:
		{
			window->win32CursorTracked = false;
			_glfwInputCursorEnter(window, false);
			return 0;
		}

		case WM_MOUSEWHEEL:
		{
			_glfwInputScroll(window, 0.0, (SHORT) HIWORD(wParam) / (double) WHEEL_DELTA);
			return 0;
		}

		case WM_MOUSEHWHEEL:
		{
			// NOTE: The X-axis is inverted for consistency with macOS and X11
			_glfwInputScroll(window, -((SHORT) HIWORD(wParam) / (double) WHEEL_DELTA), 0.0);
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
			const IntBool iconified = wParam == SIZE_MINIMIZED;
			const IntBool maximized = wParam == SIZE_MAXIMIZED ||
									   (window->maximized &&
										wParam != SIZE_RESTORED);

			if (window->win32Iconified != iconified)
				_glfwInputWindowIconify(window, iconified);

			if (window->maximized != maximized)
				_glfwInputWindowMaximize(window, maximized);

			if (width != window->width || height != window->height)
			{
				window->width = width;
				window->height = height;

				_glfwInputFramebufferSize(window, width, height);
				_glfwInputWindowSize(window, width, height);
			}

			if (window->monitor && window->win32Iconified != iconified)
			{
				if (iconified)
					releaseMonitor(window);
				else
				{
					acquireMonitor(window);
					fitToMonitor(window);
				}
			}

			window->win32Iconified = iconified;
			window->maximized = maximized;
			return 0;
		}

		case WM_MOVE:
		{
			// NOTE: This cannot use LOWORD/HIWORD recommended by MSDN, as
			// those macros do not handle negative window positions correctly
			_glfwInputWindowPos(window,
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
				_glfw.win32User32AdjustWindowRectExForDpi_(&frame, style, FALSE, exStyle,
										 _glfw.win32User32GetDpiForWindow_(window->win32Window));
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
			_glfwInputWindowDamage(window);
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

				_glfw.win32User32AdjustWindowRectExForDpi_(&source, getWindowStyle(window),
										 FALSE, getWindowExStyle(window),
										 _glfw.win32User32GetDpiForWindow_(window->win32Window));
				_glfw.win32User32AdjustWindowRectExForDpi_(&target, getWindowStyle(window),
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

			_glfwInputWindowContentScale(window, xscale, yscale);
			break;
		}

		case WM_SETCURSOR:
		{
			if (LOWORD(lParam) == HTCLIENT)
			{
				updateCursorImage(window);
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
			char** paths = _glfw_calloc(count, sizeof(char*));

			// Move the mouse to the position of the drop
			DragQueryPoint(drop, &pt);
			_glfwInputCursorPos(window, pt.x, pt.y);

			for (i = 0;  i < count;  i++)
			{
				const UINT length = DragQueryFileW(drop, i, NULL, 0);
				WCHAR* buffer = _glfw_calloc((size_t) length + 1, sizeof(WCHAR));

				DragQueryFileW(drop, i, buffer, length + 1);
				paths[i] = _glfwCreateUTF8FromWideStringWin32(buffer);

				_glfw_free(buffer);
			}

			_glfwInputDrop(window, count, (const char**) paths);

			for (i = 0;  i < count;  i++)
				_glfw_free(paths[i]);
			_glfw_free(paths);

			DragFinish(drop);
			return 0;
		}
	}

	return DefWindowProcW(hWnd, uMsg, wParam, lParam);
}

// Creates the GLFW window
//
static int createNativeWindow(plafWindow* window,
							  const WindowConfig* wndconfig,
							  const plafFrameBufferCfg* fbconfig)
{
	int frameX, frameY, frameWidth, frameHeight;
	WCHAR* wideTitle;
	DWORD style = getWindowStyle(window);
	DWORD exStyle = getWindowExStyle(window);

	if (!_glfw.win32MainWindowClass)
	{
		WNDCLASSEXW wc = { sizeof(wc) };
		wc.style         = CS_HREDRAW | CS_VREDRAW | CS_OWNDC;
		wc.lpfnWndProc   = windowProc;
		wc.hInstance     = _glfw.win32Instance;
		wc.hCursor       = LoadCursorW(NULL, IDC_ARROW);
#if defined(_GLFW_WNDCLASSNAME)
		wc.lpszClassName = _GLFW_WNDCLASSNAME;
#else
		wc.lpszClassName = L"GLFW30";
#endif
		// Load user-provided icon if available
		wc.hIcon = LoadImageW(GetModuleHandleW(NULL),
							  L"GLFW_ICON", IMAGE_ICON,
							  0, 0, LR_DEFAULTSIZE | LR_SHARED);
		if (!wc.hIcon)
		{
			// No user-provided icon found, load default icon
			wc.hIcon = LoadImageW(NULL,
								  IDI_APPLICATION, IMAGE_ICON,
								  0, 0, LR_DEFAULTSIZE | LR_SHARED);
		}

		_glfw.win32MainWindowClass = RegisterClassExW(&wc);
		if (!_glfw.win32MainWindowClass)
		{
			_glfwInputErrorWin32(ERR_PLATFORM_ERROR, "Win32: Failed to register window class");
			return false;
		}
	}

	if (GetSystemMetrics(SM_REMOTESESSION))
	{
		// NOTE: On Remote Desktop, setting the cursor to NULL does not hide it
		// HACK: Create a transparent cursor and always set that instead of NULL
		//       When not on Remote Desktop, this handle is NULL and normal hiding is used
		if (!_glfw.win32BlankCursor)
		{
			const int cursorWidth = GetSystemMetrics(SM_CXCURSOR);
			const int cursorHeight = GetSystemMetrics(SM_CYCURSOR);

			unsigned char* cursorPixels = _glfw_calloc(cursorWidth * cursorHeight, 4);
			if (!cursorPixels)
				return false;

			// NOTE: Windows checks whether the image is fully transparent and if so
			//       just ignores the alpha channel and makes the whole cursor opaque
			// HACK: Make one pixel slightly less transparent
			cursorPixels[3] = 1;

			const ImageData cursorImage = { cursorWidth, cursorHeight, cursorPixels };
			_glfw.win32BlankCursor = createIcon(&cursorImage, 0, 0, FALSE);
			_glfw_free(cursorPixels);

			if (!_glfw.win32BlankCursor)
				return false;
		}
	}

	if (window->monitor)
	{
		MONITORINFO mi = { sizeof(mi) };
		GetMonitorInfoW(window->monitor->win32Handle, &mi);

		// NOTE: This window placement is temporary and approximate, as the
		//       correct position and size cannot be known until the monitor
		//       video mode has been picked in _glfwSetVideoModeWin32
		frameX = mi.rcMonitor.left;
		frameY = mi.rcMonitor.top;
		frameWidth  = mi.rcMonitor.right - mi.rcMonitor.left;
		frameHeight = mi.rcMonitor.bottom - mi.rcMonitor.top;
	}
	else
	{
		RECT rect = { 0, 0, wndconfig->width, wndconfig->height };

		window->maximized = wndconfig->maximized;
		if (wndconfig->maximized)
			style |= WS_MAXIMIZE;

		AdjustWindowRectEx(&rect, style, FALSE, exStyle);

		if (wndconfig->xpos == ANY_POSITION && wndconfig->ypos == ANY_POSITION)
		{
			frameX = CW_USEDEFAULT;
			frameY = CW_USEDEFAULT;
		}
		else
		{
			frameX = wndconfig->xpos + rect.left;
			frameY = wndconfig->ypos + rect.top;
		}

		frameWidth  = rect.right - rect.left;
		frameHeight = rect.bottom - rect.top;
	}

	wideTitle = _glfwCreateWideStringFromUTF8Win32(window->title);
	if (!wideTitle)
		return false;

	window->win32Window = CreateWindowExW(exStyle,
										   MAKEINTATOM(_glfw.win32MainWindowClass),
										   wideTitle,
										   style,
										   frameX, frameY,
										   frameWidth, frameHeight,
										   NULL, // No parent window
										   NULL, // No window menu
										   _glfw.win32Instance,
										   (LPVOID) wndconfig);

	_glfw_free(wideTitle);

	if (!window->win32Window)
	{
		_glfwInputErrorWin32(ERR_PLATFORM_ERROR, "Win32: Failed to create window");
		return false;
	}

	SetPropW(window->win32Window, L"GLFW", window);

	ChangeWindowMessageFilterEx(window->win32Window, WM_DROPFILES, MSGFLT_ALLOW, NULL);
	ChangeWindowMessageFilterEx(window->win32Window, WM_COPYDATA, MSGFLT_ALLOW, NULL);
	ChangeWindowMessageFilterEx(window->win32Window, WM_COPYGLOBALDATA, MSGFLT_ALLOW, NULL);

	window->win32ScaleToMonitor = wndconfig->scaleToMonitor;

	if (!window->monitor)
	{
		RECT rect = { 0, 0, wndconfig->width, wndconfig->height };
		WINDOWPLACEMENT wp = { sizeof(wp) };
		const HMONITOR mh = MonitorFromWindow(window->win32Window,
											  MONITOR_DEFAULTTONEAREST);

		// Adjust window rect to account for DPI scaling of the window frame and
		// (if enabled) DPI scaling of the content area
		// This cannot be done until we know what monitor the window was placed on
		// Only update the restored window rect as the window may be maximized

		if (wndconfig->scaleToMonitor)
		{
			float xscale, yscale;
			_glfwGetHMONITORContentScaleWin32(mh, &xscale, &yscale);

			if (xscale > 0.f && yscale > 0.f)
			{
				rect.right = (int) (rect.right * xscale);
				rect.bottom = (int) (rect.bottom * yscale);
			}
		}

		if (IsWindows10Version1607OrGreater())
		{
			_glfw.win32User32AdjustWindowRectExForDpi_(&rect, style, FALSE, exStyle,
									 _glfw.win32User32GetDpiForWindow_(window->win32Window));
		}
		else
			AdjustWindowRectEx(&rect, style, FALSE, exStyle);

		GetWindowPlacement(window->win32Window, &wp);
		OffsetRect(&rect,
				   wp.rcNormalPosition.left - rect.left,
				   wp.rcNormalPosition.top - rect.top);

		wp.rcNormalPosition = rect;
		wp.showCmd = SW_HIDE;
		SetWindowPlacement(window->win32Window, &wp);

		// Adjust rect of maximized undecorated window, because by default Windows will
		// make such a window cover the whole monitor instead of its workarea

		if (wndconfig->maximized && !wndconfig->decorated)
		{
			MONITORINFO mi = { sizeof(mi) };
			GetMonitorInfoW(mh, &mi);

			SetWindowPos(window->win32Window, HWND_TOP,
						 mi.rcWork.left,
						 mi.rcWork.top,
						 mi.rcWork.right - mi.rcWork.left,
						 mi.rcWork.bottom - mi.rcWork.top,
						 SWP_NOACTIVATE | SWP_NOZORDER);
		}
	}

	DragAcceptFiles(window->win32Window, TRUE);

	if (fbconfig->transparent)
	{
		updateFramebufferTransparency(window);
		window->win32Transparent = true;
	}

	_glfwGetWindowSize(window, &window->width, &window->height);

	return true;
}

IntBool _glfwCreateWindow(plafWindow* window, const WindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig) {
	if (!createNativeWindow(window, wndconfig, fbconfig))
		return false;

	if (!_glfwInitWGL())
		return false;

		if (!_glfwCreateContextWGL(window, ctxconfig, fbconfig))
		return false;

	if (!_glfwRefreshContextAttribs(window, ctxconfig))
		return false;

	if (wndconfig->mousePassthrough)
		_glfwSetWindowMousePassthrough(window, true);

	if (window->monitor)
	{
		_glfwShowWindow(window);
		glfwFocusWindow(window);
		acquireMonitor(window);
		fitToMonitor(window);
	}

	return true;
}

void _glfwDestroyWindow(plafWindow* window) {
	if (window->monitor)
		releaseMonitor(window);

	if (window->context.destroy)
		window->context.destroy(window);

	if (window->win32Window)
	{
		RemovePropW(window->win32Window, L"GLFW");
		DestroyWindow(window->win32Window);
		window->win32Window = NULL;
	}

	if (window->win32BigIcon)
		DestroyIcon(window->win32BigIcon);

	if (window->win32SmallIcon)
		DestroyIcon(window->win32SmallIcon);
}

void _glfwSetWindowTitle(plafWindow* window, const char* title) {
	WCHAR* wideTitle = _glfwCreateWideStringFromUTF8Win32(title);
	if (!wideTitle)
		return;

	SetWindowTextW(window->win32Window, wideTitle);
	_glfw_free(wideTitle);
}

void _glfwSetWindowIcon(plafWindow* window, int count, const ImageData* images) {
	HICON bigIcon = NULL, smallIcon = NULL;

	if (count)
	{
		const ImageData* bigImage = chooseImage(count, images,
												GetSystemMetrics(SM_CXICON),
												GetSystemMetrics(SM_CYICON));
		const ImageData* smallImage = chooseImage(count, images,
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

void _glfwGetWindowPos(plafWindow* window, int* xpos, int* ypos) {
	POINT pos = { 0, 0 };
	ClientToScreen(window->win32Window, &pos);

	if (xpos)
		*xpos = pos.x;
	if (ypos)
		*ypos = pos.y;
}

void _glfwSetWindowPos(plafWindow* window, int x, int y) {
	RECT rect = { x, y, x, y };

	if (IsWindows10Version1607OrGreater())
	{
		_glfw.win32User32AdjustWindowRectExForDpi_(&rect, getWindowStyle(window),
								 FALSE, getWindowExStyle(window),
								 _glfw.win32User32GetDpiForWindow_(window->win32Window));
	}
	else
	{
		AdjustWindowRectEx(&rect, getWindowStyle(window),
						   FALSE, getWindowExStyle(window));
	}

	SetWindowPos(window->win32Window, NULL, rect.left, rect.top, 0, 0,
				 SWP_NOACTIVATE | SWP_NOZORDER | SWP_NOSIZE);
}

void _glfwGetWindowSize(plafWindow* window, int* width, int* height) {
	RECT area;
	GetClientRect(window->win32Window, &area);

	if (width)
		*width = area.right;
	if (height)
		*height = area.bottom;
}

void _glfwSetWindowSize(plafWindow* window, int width, int height) {
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
			_glfw.win32User32AdjustWindowRectExForDpi_(&rect, getWindowStyle(window),
									 FALSE, getWindowExStyle(window),
									 _glfw.win32User32GetDpiForWindow_(window->win32Window));
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

void _glfwSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight) {
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

void _glfwSetWindowAspectRatio(plafWindow* window, int numer, int denom) {
	RECT area;

	if (numer == DONT_CARE || denom == DONT_CARE)
		return;

	GetWindowRect(window->win32Window, &area);
	applyAspectRatio(window, WMSZ_BOTTOMRIGHT, &area);
	MoveWindow(window->win32Window,
			   area.left, area.top,
			   area.right - area.left,
			   area.bottom - area.top, TRUE);
}

void _glfwGetFramebufferSize(plafWindow* window, int* width, int* height) {
	_glfwGetWindowSize(window, width, height);
}

void _glfwGetWindowFrameSize(plafWindow* window, int* left, int* top, int* right, int* bottom) {
	RECT rect;
	int width, height;

	_glfwGetWindowSize(window, &width, &height);
	SetRect(&rect, 0, 0, width, height);

	if (IsWindows10Version1607OrGreater())
	{
		_glfw.win32User32AdjustWindowRectExForDpi_(&rect, getWindowStyle(window),
								 FALSE, getWindowExStyle(window),
								 _glfw.win32User32GetDpiForWindow_(window->win32Window));
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

void _glfwGetWindowContentScale(plafWindow* window, float* xscale, float* yscale) {
	const HANDLE handle = MonitorFromWindow(window->win32Window, MONITOR_DEFAULTTONEAREST);
	_glfwGetHMONITORContentScaleWin32(handle, xscale, yscale);
}

void glfwIconifyWindow(plafWindow* window) {
	ShowWindow(window->win32Window, SW_MINIMIZE);
}

void glfwRestoreWindow(plafWindow* window) {
	ShowWindow(window->win32Window, SW_RESTORE);
}

void _glfwMaximizeWindow(plafWindow* window) {
	if (IsWindowVisible(window->win32Window))
		ShowWindow(window->win32Window, SW_MAXIMIZE);
	else
		maximizeWindowManually(window);
}

void _glfwShowWindow(plafWindow* window) {
	ShowWindow(window->win32Window, SW_SHOWNA);
}

void _glfwHideWindow(plafWindow* window) {
	ShowWindow(window->win32Window, SW_HIDE);
}

void glfwRequestWindowAttention(plafWindow* window) {
	FlashWindow(window->win32Window, TRUE);
}

void glfwFocusWindow(plafWindow* window) {
	BringWindowToTop(window->win32Window);
	SetForegroundWindow(window->win32Window);
	SetFocus(window->win32Window);
}

void _glfwSetWindowMonitor(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate) {
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
				_glfw.win32User32AdjustWindowRectExForDpi_(&rect, getWindowStyle(window),
										 FALSE, getWindowExStyle(window),
										 _glfw.win32User32GetDpiForWindow_(window->win32Window));
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

	if (window->monitor)
		releaseMonitor(window);

	_glfwInputWindowMonitor(window, monitor);

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
			_glfw.win32User32AdjustWindowRectExForDpi_(&rect, getWindowStyle(window),
									 FALSE, getWindowExStyle(window),
									 _glfw.win32User32GetDpiForWindow_(window->win32Window));
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

IntBool _glfwWindowFocused(plafWindow* window) {
	return window->win32Window == GetActiveWindow();
}

IntBool _glfwWindowIconified(plafWindow* window) {
	return IsIconic(window->win32Window);
}

IntBool _glfwWindowVisible(plafWindow* window) {
	return IsWindowVisible(window->win32Window);
}

IntBool _glfwWindowMaximized(plafWindow* window) {
	return IsZoomed(window->win32Window);
}

IntBool _glfwWindowHovered(plafWindow* window) {
	return cursorInContentArea(window);
}

IntBool _glfwFramebufferTransparent(plafWindow* window) {
	BOOL composition;
	if (!window->win32Transparent) {
		return false;
	}
	if (FAILED(_glfw.win32DwmIsCompositionEnabled(&composition)) || !composition) {
		return false;
	}
	return true;
}

void _glfwSetWindowResizable(plafWindow* window, IntBool enabled) {
	updateWindowStyles(window);
}

void _glfwSetWindowDecorated(plafWindow* window, IntBool enabled) {
	updateWindowStyles(window);
}

void _glfwSetWindowFloating(plafWindow* window, IntBool enabled) {
	const HWND after = enabled ? HWND_TOPMOST : HWND_NOTOPMOST;
	SetWindowPos(window->win32Window, after, 0, 0, 0, 0,
				 SWP_NOACTIVATE | SWP_NOMOVE | SWP_NOSIZE);
}

void _glfwSetWindowMousePassthrough(plafWindow* window, IntBool enabled) {
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

float glfwGetWindowOpacity(plafWindow* window) {
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

void _glfwSetWindowOpacity(plafWindow* window, float opacity) {
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

void glfwPollEvents(void) {
	MSG msg;
	HWND handle;
	plafWindow* window;

	while (PeekMessageW(&msg, NULL, 0, 0, PM_REMOVE))
	{
		if (msg.message == WM_QUIT)
		{
			// NOTE: While GLFW does not itself post WM_QUIT, other processes
			//       may post it to this one, for example Task Manager
			// HACK: Treat WM_QUIT as a close on all windows

			window = _glfw.windowListHead;
			while (window)
			{
				_glfwInputWindowCloseRequest(window);
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
	//       Other Win hotkeys are handled implicitly by _glfwInputWindowFocus
	//       because they change the input focus
	// NOTE: The other half of this is in the WM_*KEY* handler in windowProc
	handle = GetActiveWindow();
	if (handle)
	{
		window = GetPropW(handle, L"GLFW");
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
				const int scancode = _glfw.scanCodes[key];

				if ((GetKeyState(vk) & 0x8000))
					continue;
				if (window->keys[key] != INPUT_PRESS)
					continue;

				_glfwInputKey(window, key, scancode, INPUT_RELEASE, getKeyMods());
			}
		}
	}
}

void glfwWaitEvents(void) {
	WaitMessage();
	glfwPollEvents();
}

void _glfwWaitEventsTimeout(double timeout) {
	MsgWaitForMultipleObjects(0, NULL, FALSE, (DWORD) (timeout * 1e3), QS_ALLINPUT);
	glfwPollEvents();
}

void glfwPostEmptyEvent(void) {
	PostMessageW(_glfw.win32HelperWindowHandle, WM_NULL, 0, 0);
}

void glfwSetCursorMode(plafWindow* window, int mode) {
	if (cursorInContentArea(window))
		updateCursorImage(window);
}

IntBool _glfwCreateCursor(plafCursor* cursor, const ImageData* image, int xhot, int yhot) {
	cursor->win32Cursor = (HCURSOR) createIcon(image, xhot, yhot, false);
	if (!cursor->win32Cursor)
		return false;

	return true;
}

IntBool _glfwCreateStandardCursor(plafCursor* cursor, int shape) {
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
			_glfwInputError(ERR_PLATFORM_ERROR, "Win32: Unknown standard cursor");
			return false;
	}

	cursor->win32Cursor = LoadImageW(NULL,
									  MAKEINTRESOURCEW(id), IMAGE_CURSOR, 0, 0,
									  LR_DEFAULTSIZE | LR_SHARED);
	if (!cursor->win32Cursor)
	{
		_glfwInputErrorWin32(ERR_PLATFORM_ERROR, "Win32: Failed to create standard cursor");
		return false;
	}

	return true;
}

void _glfwDestroyCursor(plafCursor* cursor) {
	if (cursor->win32Cursor)
		DestroyIcon((HICON) cursor->win32Cursor);
}

HWND glfwGetWin32Window(plafWindow* window) {
	return window->win32Window;
}

#endif // _WIN32
