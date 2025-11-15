#if defined(PLATFORM_LINUX)

#include "platform.h"
#include <limits.h>
#include <stdio.h>
#include <locale.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>


// Translate the X11 KeySyms for a key to a GLFW key code
// NOTE: This is only used as a fallback, in case the XKB method fails
//       It is layout-dependent and will fail partially on most non-US layouts
//
static int translateKeySyms(const KeySym* keysyms, int width)
{
	if (width > 1)
	{
		switch (keysyms[1])
		{
			case XK_KP_0:           return KEY_KP_0;
			case XK_KP_1:           return KEY_KP_1;
			case XK_KP_2:           return KEY_KP_2;
			case XK_KP_3:           return KEY_KP_3;
			case XK_KP_4:           return KEY_KP_4;
			case XK_KP_5:           return KEY_KP_5;
			case XK_KP_6:           return KEY_KP_6;
			case XK_KP_7:           return KEY_KP_7;
			case XK_KP_8:           return KEY_KP_8;
			case XK_KP_9:           return KEY_KP_9;
			case XK_KP_Separator:
			case XK_KP_Decimal:     return KEY_KP_DECIMAL;
			case XK_KP_Equal:       return KEY_KP_EQUAL;
			case XK_KP_Enter:       return KEY_KP_ENTER;
			default:                break;
		}
	}

	switch (keysyms[0])
	{
		case XK_Escape:         return KEY_ESCAPE;
		case XK_Tab:            return KEY_TAB;
		case XK_Shift_L:        return KEY_LEFT_SHIFT;
		case XK_Shift_R:        return KEY_RIGHT_SHIFT;
		case XK_Control_L:      return KEY_LEFT_CONTROL;
		case XK_Control_R:      return KEY_RIGHT_CONTROL;
		case XK_Meta_L:
		case XK_Alt_L:          return KEY_LEFT_ALT;
		case XK_Mode_switch: // Mapped to Alt_R on many keyboards
		case XK_ISO_Level3_Shift: // AltGr on at least some machines
		case XK_Meta_R:
		case XK_Alt_R:          return KEY_RIGHT_ALT;
		case XK_Super_L:        return KEY_LEFT_SUPER;
		case XK_Super_R:        return KEY_RIGHT_SUPER;
		case XK_Menu:           return KEY_MENU;
		case XK_Num_Lock:       return KEY_NUM_LOCK;
		case XK_Caps_Lock:      return KEY_CAPS_LOCK;
		case XK_Print:          return KEY_PRINT_SCREEN;
		case XK_Scroll_Lock:    return KEY_SCROLL_LOCK;
		case XK_Pause:          return KEY_PAUSE;
		case XK_Delete:         return KEY_DELETE;
		case XK_BackSpace:      return KEY_BACKSPACE;
		case XK_Return:         return KEY_ENTER;
		case XK_Home:           return KEY_HOME;
		case XK_End:            return KEY_END;
		case XK_Page_Up:        return KEY_PAGE_UP;
		case XK_Page_Down:      return KEY_PAGE_DOWN;
		case XK_Insert:         return KEY_INSERT;
		case XK_Left:           return KEY_LEFT;
		case XK_Right:          return KEY_RIGHT;
		case XK_Down:           return KEY_DOWN;
		case XK_Up:             return KEY_UP;
		case XK_F1:             return KEY_F1;
		case XK_F2:             return KEY_F2;
		case XK_F3:             return KEY_F3;
		case XK_F4:             return KEY_F4;
		case XK_F5:             return KEY_F5;
		case XK_F6:             return KEY_F6;
		case XK_F7:             return KEY_F7;
		case XK_F8:             return KEY_F8;
		case XK_F9:             return KEY_F9;
		case XK_F10:            return KEY_F10;
		case XK_F11:            return KEY_F11;
		case XK_F12:            return KEY_F12;
		case XK_F13:            return KEY_F13;
		case XK_F14:            return KEY_F14;
		case XK_F15:            return KEY_F15;
		case XK_F16:            return KEY_F16;
		case XK_F17:            return KEY_F17;
		case XK_F18:            return KEY_F18;
		case XK_F19:            return KEY_F19;
		case XK_F20:            return KEY_F20;
		case XK_F21:            return KEY_F21;
		case XK_F22:            return KEY_F22;
		case XK_F23:            return KEY_F23;
		case XK_F24:            return KEY_F24;
		case XK_F25:            return KEY_F25;

		// Numeric keypad
		case XK_KP_Divide:      return KEY_KP_DIVIDE;
		case XK_KP_Multiply:    return KEY_KP_MULTIPLY;
		case XK_KP_Subtract:    return KEY_KP_SUBTRACT;
		case XK_KP_Add:         return KEY_KP_ADD;

		// These should have been detected in secondary keysym test above!
		case XK_KP_Insert:      return KEY_KP_0;
		case XK_KP_End:         return KEY_KP_1;
		case XK_KP_Down:        return KEY_KP_2;
		case XK_KP_Page_Down:   return KEY_KP_3;
		case XK_KP_Left:        return KEY_KP_4;
		case XK_KP_Right:       return KEY_KP_6;
		case XK_KP_Home:        return KEY_KP_7;
		case XK_KP_Up:          return KEY_KP_8;
		case XK_KP_Page_Up:     return KEY_KP_9;
		case XK_KP_Delete:      return KEY_KP_DECIMAL;
		case XK_KP_Equal:       return KEY_KP_EQUAL;
		case XK_KP_Enter:       return KEY_KP_ENTER;

		// Last resort: Check for printable keys (should not happen if the XKB
		// extension is available). This will give a layout dependent mapping
		// (which is wrong, and we may miss some keys, especially on non-US
		// keyboards), but it's better than nothing...
		case XK_a:              return KEY_A;
		case XK_b:              return KEY_B;
		case XK_c:              return KEY_C;
		case XK_d:              return KEY_D;
		case XK_e:              return KEY_E;
		case XK_f:              return KEY_F;
		case XK_g:              return KEY_G;
		case XK_h:              return KEY_H;
		case XK_i:              return KEY_I;
		case XK_j:              return KEY_J;
		case XK_k:              return KEY_K;
		case XK_l:              return KEY_L;
		case XK_m:              return KEY_M;
		case XK_n:              return KEY_N;
		case XK_o:              return KEY_O;
		case XK_p:              return KEY_P;
		case XK_q:              return KEY_Q;
		case XK_r:              return KEY_R;
		case XK_s:              return KEY_S;
		case XK_t:              return KEY_T;
		case XK_u:              return KEY_U;
		case XK_v:              return KEY_V;
		case XK_w:              return KEY_W;
		case XK_x:              return KEY_X;
		case XK_y:              return KEY_Y;
		case XK_z:              return KEY_Z;
		case XK_1:              return KEY_1;
		case XK_2:              return KEY_2;
		case XK_3:              return KEY_3;
		case XK_4:              return KEY_4;
		case XK_5:              return KEY_5;
		case XK_6:              return KEY_6;
		case XK_7:              return KEY_7;
		case XK_8:              return KEY_8;
		case XK_9:              return KEY_9;
		case XK_0:              return KEY_0;
		case XK_space:          return KEY_SPACE;
		case XK_minus:          return KEY_MINUS;
		case XK_equal:          return KEY_EQUAL;
		case XK_bracketleft:    return KEY_LEFT_BRACKET;
		case XK_bracketright:   return KEY_RIGHT_BRACKET;
		case XK_backslash:      return KEY_BACKSLASH;
		case XK_semicolon:      return KEY_SEMICOLON;
		case XK_apostrophe:     return KEY_APOSTROPHE;
		case XK_grave:          return KEY_GRAVE_ACCENT;
		case XK_comma:          return KEY_COMMA;
		case XK_period:         return KEY_PERIOD;
		case XK_slash:          return KEY_SLASH;
		case XK_less:           return KEY_WORLD_1; // At least in some layouts...
		default:                break;
	}

	// No matching translation was found
	return KEY_UNKNOWN;
}

// Create key code translation tables
//
static void createKeyTables(void)
{
	int scancodeMin, scancodeMax;

	memset(_glfw.x11.keycodes, -1, sizeof(_glfw.x11.keycodes));
	memset(_glfw.x11.scancodes, -1, sizeof(_glfw.x11.scancodes));

	if (_glfw.x11.xkb.available)
	{
		// Use XKB to determine physical key locations independently of the
		// current keyboard layout

		XkbDescPtr desc = XkbGetMap(_glfw.x11.display, 0, XkbUseCoreKbd);
		XkbGetNames(_glfw.x11.display, XkbKeyNamesMask | XkbKeyAliasesMask, desc);

		scancodeMin = desc->min_key_code;
		scancodeMax = desc->max_key_code;

		const struct
		{
			int key;
			char* name;
		} keymap[] =
		{
			{ KEY_GRAVE_ACCENT, "TLDE" },
			{ KEY_1, "AE01" },
			{ KEY_2, "AE02" },
			{ KEY_3, "AE03" },
			{ KEY_4, "AE04" },
			{ KEY_5, "AE05" },
			{ KEY_6, "AE06" },
			{ KEY_7, "AE07" },
			{ KEY_8, "AE08" },
			{ KEY_9, "AE09" },
			{ KEY_0, "AE10" },
			{ KEY_MINUS, "AE11" },
			{ KEY_EQUAL, "AE12" },
			{ KEY_Q, "AD01" },
			{ KEY_W, "AD02" },
			{ KEY_E, "AD03" },
			{ KEY_R, "AD04" },
			{ KEY_T, "AD05" },
			{ KEY_Y, "AD06" },
			{ KEY_U, "AD07" },
			{ KEY_I, "AD08" },
			{ KEY_O, "AD09" },
			{ KEY_P, "AD10" },
			{ KEY_LEFT_BRACKET, "AD11" },
			{ KEY_RIGHT_BRACKET, "AD12" },
			{ KEY_A, "AC01" },
			{ KEY_S, "AC02" },
			{ KEY_D, "AC03" },
			{ KEY_F, "AC04" },
			{ KEY_G, "AC05" },
			{ KEY_H, "AC06" },
			{ KEY_J, "AC07" },
			{ KEY_K, "AC08" },
			{ KEY_L, "AC09" },
			{ KEY_SEMICOLON, "AC10" },
			{ KEY_APOSTROPHE, "AC11" },
			{ KEY_Z, "AB01" },
			{ KEY_X, "AB02" },
			{ KEY_C, "AB03" },
			{ KEY_V, "AB04" },
			{ KEY_B, "AB05" },
			{ KEY_N, "AB06" },
			{ KEY_M, "AB07" },
			{ KEY_COMMA, "AB08" },
			{ KEY_PERIOD, "AB09" },
			{ KEY_SLASH, "AB10" },
			{ KEY_BACKSLASH, "BKSL" },
			{ KEY_WORLD_1, "LSGT" },
			{ KEY_SPACE, "SPCE" },
			{ KEY_ESCAPE, "ESC" },
			{ KEY_ENTER, "RTRN" },
			{ KEY_TAB, "TAB" },
			{ KEY_BACKSPACE, "BKSP" },
			{ KEY_INSERT, "INS" },
			{ KEY_DELETE, "DELE" },
			{ KEY_RIGHT, "RGHT" },
			{ KEY_LEFT, "LEFT" },
			{ KEY_DOWN, "DOWN" },
			{ KEY_UP, "UP" },
			{ KEY_PAGE_UP, "PGUP" },
			{ KEY_PAGE_DOWN, "PGDN" },
			{ KEY_HOME, "HOME" },
			{ KEY_END, "END" },
			{ KEY_CAPS_LOCK, "CAPS" },
			{ KEY_SCROLL_LOCK, "SCLK" },
			{ KEY_NUM_LOCK, "NMLK" },
			{ KEY_PRINT_SCREEN, "PRSC" },
			{ KEY_PAUSE, "PAUS" },
			{ KEY_F1, "FK01" },
			{ KEY_F2, "FK02" },
			{ KEY_F3, "FK03" },
			{ KEY_F4, "FK04" },
			{ KEY_F5, "FK05" },
			{ KEY_F6, "FK06" },
			{ KEY_F7, "FK07" },
			{ KEY_F8, "FK08" },
			{ KEY_F9, "FK09" },
			{ KEY_F10, "FK10" },
			{ KEY_F11, "FK11" },
			{ KEY_F12, "FK12" },
			{ KEY_F13, "FK13" },
			{ KEY_F14, "FK14" },
			{ KEY_F15, "FK15" },
			{ KEY_F16, "FK16" },
			{ KEY_F17, "FK17" },
			{ KEY_F18, "FK18" },
			{ KEY_F19, "FK19" },
			{ KEY_F20, "FK20" },
			{ KEY_F21, "FK21" },
			{ KEY_F22, "FK22" },
			{ KEY_F23, "FK23" },
			{ KEY_F24, "FK24" },
			{ KEY_F25, "FK25" },
			{ KEY_KP_0, "KP0" },
			{ KEY_KP_1, "KP1" },
			{ KEY_KP_2, "KP2" },
			{ KEY_KP_3, "KP3" },
			{ KEY_KP_4, "KP4" },
			{ KEY_KP_5, "KP5" },
			{ KEY_KP_6, "KP6" },
			{ KEY_KP_7, "KP7" },
			{ KEY_KP_8, "KP8" },
			{ KEY_KP_9, "KP9" },
			{ KEY_KP_DECIMAL, "KPDL" },
			{ KEY_KP_DIVIDE, "KPDV" },
			{ KEY_KP_MULTIPLY, "KPMU" },
			{ KEY_KP_SUBTRACT, "KPSU" },
			{ KEY_KP_ADD, "KPAD" },
			{ KEY_KP_ENTER, "KPEN" },
			{ KEY_KP_EQUAL, "KPEQ" },
			{ KEY_LEFT_SHIFT, "LFSH" },
			{ KEY_LEFT_CONTROL, "LCTL" },
			{ KEY_LEFT_ALT, "LALT" },
			{ KEY_LEFT_SUPER, "LWIN" },
			{ KEY_RIGHT_SHIFT, "RTSH" },
			{ KEY_RIGHT_CONTROL, "RCTL" },
			{ KEY_RIGHT_ALT, "RALT" },
			{ KEY_RIGHT_ALT, "LVL3" },
			{ KEY_RIGHT_ALT, "MDSW" },
			{ KEY_RIGHT_SUPER, "RWIN" },
			{ KEY_MENU, "MENU" }
		};

		// Find the X11 key code -> GLFW key code mapping
		for (int scancode = scancodeMin;  scancode <= scancodeMax;  scancode++)
		{
			int key = KEY_UNKNOWN;

			// Map the key name to a GLFW key code. Note: We use the US
			// keyboard layout. Because function keys aren't mapped correctly
			// when using traditional KeySym translations, they are mapped
			// here instead.
			for (int i = 0;  i < sizeof(keymap) / sizeof(keymap[0]);  i++)
			{
				if (strncmp(desc->names->keys[scancode].name,
							keymap[i].name,
							XkbKeyNameLength) == 0)
				{
					key = keymap[i].key;
					break;
				}
			}

			// Fall back to key aliases in case the key name did not match
			for (int i = 0;  i < desc->names->num_key_aliases;  i++)
			{
				if (key != KEY_UNKNOWN)
					break;

				if (strncmp(desc->names->key_aliases[i].real,
							desc->names->keys[scancode].name,
							XkbKeyNameLength) != 0)
				{
					continue;
				}

				for (int j = 0;  j < sizeof(keymap) / sizeof(keymap[0]);  j++)
				{
					if (strncmp(desc->names->key_aliases[i].alias,
								keymap[j].name,
								XkbKeyNameLength) == 0)
					{
						key = keymap[j].key;
						break;
					}
				}
			}

			_glfw.x11.keycodes[scancode] = key;
		}

		XkbFreeNames(desc, XkbKeyNamesMask, True);
		XkbFreeKeyboard(desc, 0, True);
	}
	else
		XDisplayKeycodes(_glfw.x11.display, &scancodeMin, &scancodeMax);

	int width;
	KeySym* keysyms = XGetKeyboardMapping(_glfw.x11.display,
										  scancodeMin,
										  scancodeMax - scancodeMin + 1,
										  &width);

	for (int scancode = scancodeMin;  scancode <= scancodeMax;  scancode++)
	{
		// Translate the un-translated key codes using traditional X11 KeySym
		// lookups
		if (_glfw.x11.keycodes[scancode] < 0)
		{
			const size_t base = (scancode - scancodeMin) * width;
			_glfw.x11.keycodes[scancode] = translateKeySyms(&keysyms[base], width);
		}

		// Store the reverse translation for faster key name lookup
		if (_glfw.x11.keycodes[scancode] > 0)
			_glfw.x11.scancodes[_glfw.x11.keycodes[scancode]] = scancode;
	}

	XFree(keysyms);
}

// Check whether the IM has a usable style
//
static IntBool hasUsableInputMethodStyle(void)
{
	IntBool found = false;
	XIMStyles* styles = NULL;

	if (XGetIMValues(_glfw.x11.im, XNQueryInputStyle, &styles, NULL) != NULL)
		return false;

	for (unsigned int i = 0;  i < styles->count_styles;  i++)
	{
		if (styles->supported_styles[i] == (XIMPreeditNothing | XIMStatusNothing))
		{
			found = true;
			break;
		}
	}

	XFree(styles);
	return found;
}

static void inputMethodDestroyCallback(XIM im, XPointer clientData, XPointer callData)
{
	_glfw.x11.im = NULL;
}

static void inputMethodInstantiateCallback(Display* display,
										   XPointer clientData,
										   XPointer callData)
{
	if (_glfw.x11.im)
		return;

	_glfw.x11.im = XOpenIM(_glfw.x11.display, 0, NULL, NULL);
	if (_glfw.x11.im)
	{
		if (!hasUsableInputMethodStyle())
		{
			XCloseIM(_glfw.x11.im);
			_glfw.x11.im = NULL;
		}
	}

	if (_glfw.x11.im)
	{
		XIMCallback callback;
		callback.callback = (XIMProc) inputMethodDestroyCallback;
		callback.client_data = NULL;
		XSetIMValues(_glfw.x11.im, XNDestroyCallback, &callback, NULL);

		for (_GLFWwindow* window = _glfw.windowListHead;  window;  window = window->next)
			_glfwCreateInputContextX11(window);
	}
}

// Return the atom ID only if it is listed in the specified array
//
static Atom getAtomIfSupported(Atom* supportedAtoms,
							   unsigned long atomCount,
							   const char* atomName)
{
	const Atom atom = XInternAtom(_glfw.x11.display, atomName, False);

	for (unsigned long i = 0;  i < atomCount;  i++)
	{
		if (supportedAtoms[i] == atom)
			return atom;
	}

	return None;
}

// Check whether the running window manager is EWMH-compliant
//
static void detectEWMH(void)
{
	// First we read the _NET_SUPPORTING_WM_CHECK property on the root window

	Window* windowFromRoot = NULL;
	if (!_glfwGetWindowPropertyX11(_glfw.x11.root,
								   _glfw.x11.NET_SUPPORTING_WM_CHECK,
								   XA_WINDOW,
								   (unsigned char**) &windowFromRoot))
	{
		return;
	}

	_glfwGrabErrorHandlerX11();

	// If it exists, it should be the XID of a top-level window
	// Then we look for the same property on that window

	Window* windowFromChild = NULL;
	if (!_glfwGetWindowPropertyX11(*windowFromRoot,
								   _glfw.x11.NET_SUPPORTING_WM_CHECK,
								   XA_WINDOW,
								   (unsigned char**) &windowFromChild))
	{
		_glfwReleaseErrorHandlerX11();
		XFree(windowFromRoot);
		return;
	}

	_glfwReleaseErrorHandlerX11();

	// If the property exists, it should contain the XID of the window

	if (*windowFromRoot != *windowFromChild)
	{
		XFree(windowFromRoot);
		XFree(windowFromChild);
		return;
	}

	XFree(windowFromRoot);
	XFree(windowFromChild);

	// We are now fairly sure that an EWMH-compliant WM is currently running
	// We can now start querying the WM about what features it supports by
	// looking in the _NET_SUPPORTED property on the root window
	// It should contain a list of supported EWMH protocol and state atoms

	Atom* supportedAtoms = NULL;
	const unsigned long atomCount =
		_glfwGetWindowPropertyX11(_glfw.x11.root,
								  _glfw.x11.NET_SUPPORTED,
								  XA_ATOM,
								  (unsigned char**) &supportedAtoms);

	// See which of the atoms we support that are supported by the WM

	_glfw.x11.NET_WM_STATE =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE");
	_glfw.x11.NET_WM_STATE_ABOVE =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_ABOVE");
	_glfw.x11.NET_WM_STATE_FULLSCREEN =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_FULLSCREEN");
	_glfw.x11.NET_WM_STATE_MAXIMIZED_VERT =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_MAXIMIZED_VERT");
	_glfw.x11.NET_WM_STATE_MAXIMIZED_HORZ =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_MAXIMIZED_HORZ");
	_glfw.x11.NET_WM_STATE_DEMANDS_ATTENTION =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_DEMANDS_ATTENTION");
	_glfw.x11.NET_WM_FULLSCREEN_MONITORS =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_FULLSCREEN_MONITORS");
	_glfw.x11.NET_WM_WINDOW_TYPE =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_WINDOW_TYPE");
	_glfw.x11.NET_WM_WINDOW_TYPE_NORMAL =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_WINDOW_TYPE_NORMAL");
	_glfw.x11.NET_WORKAREA =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WORKAREA");
	_glfw.x11.NET_CURRENT_DESKTOP =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_CURRENT_DESKTOP");
	_glfw.x11.NET_ACTIVE_WINDOW =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_ACTIVE_WINDOW");
	_glfw.x11.NET_FRAME_EXTENTS =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_FRAME_EXTENTS");
	_glfw.x11.NET_REQUEST_FRAME_EXTENTS =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_REQUEST_FRAME_EXTENTS");

	if (supportedAtoms)
		XFree(supportedAtoms);
}

// Look for and initialize supported X11 extensions
//
static void initExtensions(void)
{
	_glfw.x11.vidmode.handle = _glfwPlatformLoadModule("libXxf86vm.so.1");
	if (_glfw.x11.vidmode.handle)
	{
		_glfw.x11.vidmode.QueryExtension = (PFN_XF86VidModeQueryExtension)
			_glfwPlatformGetModuleSymbol(_glfw.x11.vidmode.handle, "XF86VidModeQueryExtension");
		_glfw.x11.vidmode.GetGammaRamp = (PFN_XF86VidModeGetGammaRamp)
			_glfwPlatformGetModuleSymbol(_glfw.x11.vidmode.handle, "XF86VidModeGetGammaRamp");
		_glfw.x11.vidmode.SetGammaRamp = (PFN_XF86VidModeSetGammaRamp)
			_glfwPlatformGetModuleSymbol(_glfw.x11.vidmode.handle, "XF86VidModeSetGammaRamp");
		_glfw.x11.vidmode.GetGammaRampSize = (PFN_XF86VidModeGetGammaRampSize)
			_glfwPlatformGetModuleSymbol(_glfw.x11.vidmode.handle, "XF86VidModeGetGammaRampSize");

		_glfw.x11.vidmode.available =
			XF86VidModeQueryExtension(_glfw.x11.display,
									  &_glfw.x11.vidmode.eventBase,
									  &_glfw.x11.vidmode.errorBase);
	}

	_glfw.x11.xi.handle = _glfwPlatformLoadModule("libXi.so.6");
	if (_glfw.x11.xi.handle)
	{
		_glfw.x11.xi.QueryVersion = (PFN_XIQueryVersion)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xi.handle, "XIQueryVersion");
		_glfw.x11.xi.SelectEvents = (PFN_XISelectEvents)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xi.handle, "XISelectEvents");

		if (XQueryExtension(_glfw.x11.display,
							"XInputExtension",
							&_glfw.x11.xi.majorOpcode,
							&_glfw.x11.xi.eventBase,
							&_glfw.x11.xi.errorBase))
		{
			_glfw.x11.xi.major = 2;
			_glfw.x11.xi.minor = 0;

			if (XIQueryVersion(_glfw.x11.display,
							   &_glfw.x11.xi.major,
							   &_glfw.x11.xi.minor) == Success)
			{
				_glfw.x11.xi.available = true;
			}
		}
	}
	_glfw.x11.randr.handle = _glfwPlatformLoadModule("libXrandr.so.2");
	if (_glfw.x11.randr.handle)
	{
		_glfw.x11.randr.AllocGamma = (PFN_XRRAllocGamma)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRAllocGamma");
		_glfw.x11.randr.FreeGamma = (PFN_XRRFreeGamma)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRFreeGamma");
		_glfw.x11.randr.FreeCrtcInfo = (PFN_XRRFreeCrtcInfo)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRFreeCrtcInfo");
		_glfw.x11.randr.FreeGamma = (PFN_XRRFreeGamma)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRFreeGamma");
		_glfw.x11.randr.FreeOutputInfo = (PFN_XRRFreeOutputInfo)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRFreeOutputInfo");
		_glfw.x11.randr.FreeScreenResources = (PFN_XRRFreeScreenResources)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRFreeScreenResources");
		_glfw.x11.randr.GetCrtcGamma = (PFN_XRRGetCrtcGamma)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRGetCrtcGamma");
		_glfw.x11.randr.GetCrtcGammaSize = (PFN_XRRGetCrtcGammaSize)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRGetCrtcGammaSize");
		_glfw.x11.randr.GetCrtcInfo = (PFN_XRRGetCrtcInfo)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRGetCrtcInfo");
		_glfw.x11.randr.GetOutputInfo = (PFN_XRRGetOutputInfo)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRGetOutputInfo");
		_glfw.x11.randr.GetOutputPrimary = (PFN_XRRGetOutputPrimary)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRGetOutputPrimary");
		_glfw.x11.randr.GetScreenResourcesCurrent = (PFN_XRRGetScreenResourcesCurrent)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRGetScreenResourcesCurrent");
		_glfw.x11.randr.QueryExtension = (PFN_XRRQueryExtension)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRQueryExtension");
		_glfw.x11.randr.QueryVersion = (PFN_XRRQueryVersion)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRQueryVersion");
		_glfw.x11.randr.SelectInput = (PFN_XRRSelectInput)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRSelectInput");
		_glfw.x11.randr.SetCrtcConfig = (PFN_XRRSetCrtcConfig)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRSetCrtcConfig");
		_glfw.x11.randr.SetCrtcGamma = (PFN_XRRSetCrtcGamma)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRSetCrtcGamma");
		_glfw.x11.randr.UpdateConfiguration = (PFN_XRRUpdateConfiguration)
			_glfwPlatformGetModuleSymbol(_glfw.x11.randr.handle, "XRRUpdateConfiguration");

		if (XRRQueryExtension(_glfw.x11.display,
							  &_glfw.x11.randr.eventBase,
							  &_glfw.x11.randr.errorBase))
		{
			if (XRRQueryVersion(_glfw.x11.display,
								&_glfw.x11.randr.major,
								&_glfw.x11.randr.minor))
			{
				// The GLFW RandR path requires at least version 1.3
				if (_glfw.x11.randr.major > 1 || _glfw.x11.randr.minor >= 3)
					_glfw.x11.randr.available = true;
			}
		}
	}

	if (_glfw.x11.randr.available)
	{
		XRRScreenResources* sr = XRRGetScreenResourcesCurrent(_glfw.x11.display,
															  _glfw.x11.root);

		if (!sr->ncrtc || !XRRGetCrtcGammaSize(_glfw.x11.display, sr->crtcs[0]))
		{
			// This is likely an older Nvidia driver with broken gamma support
			// Flag it as useless and fall back to xf86vm gamma, if available
			_glfw.x11.randr.gammaBroken = true;
		}

		if (!sr->ncrtc)
		{
			// A system without CRTCs is likely a system with broken RandR
			// Disable the RandR monitor path and fall back to core functions
			_glfw.x11.randr.monitorBroken = true;
		}

		XRRFreeScreenResources(sr);
	}

	if (_glfw.x11.randr.available && !_glfw.x11.randr.monitorBroken)
	{
		XRRSelectInput(_glfw.x11.display, _glfw.x11.root,
					   RROutputChangeNotifyMask);
	}

	_glfw.x11.xcursor.handle = _glfwPlatformLoadModule("libXcursor.so.1");
	if (_glfw.x11.xcursor.handle)
	{
		_glfw.x11.xcursor.ImageCreate = (PFN_XcursorImageCreate)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xcursor.handle, "XcursorImageCreate");
		_glfw.x11.xcursor.ImageDestroy = (PFN_XcursorImageDestroy)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xcursor.handle, "XcursorImageDestroy");
		_glfw.x11.xcursor.ImageLoadCursor = (PFN_XcursorImageLoadCursor)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xcursor.handle, "XcursorImageLoadCursor");
		_glfw.x11.xcursor.GetTheme = (PFN_XcursorGetTheme)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xcursor.handle, "XcursorGetTheme");
		_glfw.x11.xcursor.GetDefaultSize = (PFN_XcursorGetDefaultSize)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xcursor.handle, "XcursorGetDefaultSize");
		_glfw.x11.xcursor.LibraryLoadImage = (PFN_XcursorLibraryLoadImage)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xcursor.handle, "XcursorLibraryLoadImage");
	}

	_glfw.x11.xinerama.handle = _glfwPlatformLoadModule("libXinerama.so.1");
	if (_glfw.x11.xinerama.handle)
	{
		_glfw.x11.xinerama.IsActive = (PFN_XineramaIsActive)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xinerama.handle, "XineramaIsActive");
		_glfw.x11.xinerama.QueryExtension = (PFN_XineramaQueryExtension)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xinerama.handle, "XineramaQueryExtension");
		_glfw.x11.xinerama.QueryScreens = (PFN_XineramaQueryScreens)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xinerama.handle, "XineramaQueryScreens");

		if (XineramaQueryExtension(_glfw.x11.display,
								   &_glfw.x11.xinerama.major,
								   &_glfw.x11.xinerama.minor))
		{
			if (XineramaIsActive(_glfw.x11.display))
				_glfw.x11.xinerama.available = true;
		}
	}

	_glfw.x11.xkb.major = 1;
	_glfw.x11.xkb.minor = 0;
	_glfw.x11.xkb.available =
		XkbQueryExtension(_glfw.x11.display,
						  &_glfw.x11.xkb.majorOpcode,
						  &_glfw.x11.xkb.eventBase,
						  &_glfw.x11.xkb.errorBase,
						  &_glfw.x11.xkb.major,
						  &_glfw.x11.xkb.minor);

	if (_glfw.x11.xkb.available)
	{
		Bool supported;

		if (XkbSetDetectableAutoRepeat(_glfw.x11.display, True, &supported))
		{
			if (supported)
				_glfw.x11.xkb.detectable = true;
		}

		XkbStateRec state;
		if (XkbGetState(_glfw.x11.display, XkbUseCoreKbd, &state) == Success)
			_glfw.x11.xkb.group = (unsigned int)state.group;

		XkbSelectEventDetails(_glfw.x11.display, XkbUseCoreKbd, XkbStateNotify,
							  XkbGroupStateMask, XkbGroupStateMask);
	}

	if (_glfw.x11.x11xcb.handle)
	{
		_glfw.x11.x11xcb.GetXCBConnection = (PFN_XGetXCBConnection)
			_glfwPlatformGetModuleSymbol(_glfw.x11.x11xcb.handle, "XGetXCBConnection");
	}

	_glfw.x11.xrender.handle = _glfwPlatformLoadModule("libXrender.so.1");
	if (_glfw.x11.xrender.handle)
	{
		_glfw.x11.xrender.QueryExtension = (PFN_XRenderQueryExtension)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xrender.handle, "XRenderQueryExtension");
		_glfw.x11.xrender.QueryVersion = (PFN_XRenderQueryVersion)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xrender.handle, "XRenderQueryVersion");
		_glfw.x11.xrender.FindVisualFormat = (PFN_XRenderFindVisualFormat)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xrender.handle, "XRenderFindVisualFormat");

		if (XRenderQueryExtension(_glfw.x11.display,
								  &_glfw.x11.xrender.errorBase,
								  &_glfw.x11.xrender.eventBase))
		{
			if (XRenderQueryVersion(_glfw.x11.display,
									&_glfw.x11.xrender.major,
									&_glfw.x11.xrender.minor))
			{
				_glfw.x11.xrender.available = true;
			}
		}
	}

	_glfw.x11.xshape.handle = _glfwPlatformLoadModule("libXext.so.6");
	if (_glfw.x11.xshape.handle)
	{
		_glfw.x11.xshape.QueryExtension = (PFN_XShapeQueryExtension)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xshape.handle, "XShapeQueryExtension");
		_glfw.x11.xshape.ShapeCombineRegion = (PFN_XShapeCombineRegion)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xshape.handle, "XShapeCombineRegion");
		_glfw.x11.xshape.QueryVersion = (PFN_XShapeQueryVersion)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xshape.handle, "XShapeQueryVersion");
		_glfw.x11.xshape.ShapeCombineMask = (PFN_XShapeCombineMask)
			_glfwPlatformGetModuleSymbol(_glfw.x11.xshape.handle, "XShapeCombineMask");

		if (XShapeQueryExtension(_glfw.x11.display,
			&_glfw.x11.xshape.errorBase,
			&_glfw.x11.xshape.eventBase))
		{
			if (XShapeQueryVersion(_glfw.x11.display,
				&_glfw.x11.xshape.major,
				&_glfw.x11.xshape.minor))
			{
				_glfw.x11.xshape.available = true;
			}
		}
	}

	// Update the key code LUT
	// FIXME: We should listen to XkbMapNotify events to track changes to
	// the keyboard mapping.
	createKeyTables();

	// String format atoms
	_glfw.x11.NULL_ = XInternAtom(_glfw.x11.display, "NULL", False);
	_glfw.x11.UTF8_STRING = XInternAtom(_glfw.x11.display, "UTF8_STRING", False);
	_glfw.x11.ATOM_PAIR = XInternAtom(_glfw.x11.display, "ATOM_PAIR", False);

	// Custom selection property atom
	_glfw.x11.GLFW_SELECTION =
		XInternAtom(_glfw.x11.display, "GLFW_SELECTION", False);

	// ICCCM standard clipboard atoms
	_glfw.x11.TARGETS = XInternAtom(_glfw.x11.display, "TARGETS", False);
	_glfw.x11.MULTIPLE = XInternAtom(_glfw.x11.display, "MULTIPLE", False);
	_glfw.x11.INCR = XInternAtom(_glfw.x11.display, "INCR", False);
	_glfw.x11.CLIPBOARD = XInternAtom(_glfw.x11.display, "CLIPBOARD", False);

	// Clipboard manager atoms
	_glfw.x11.CLIPBOARD_MANAGER =
		XInternAtom(_glfw.x11.display, "CLIPBOARD_MANAGER", False);
	_glfw.x11.SAVE_TARGETS =
		XInternAtom(_glfw.x11.display, "SAVE_TARGETS", False);

	// Xdnd (drag and drop) atoms
	_glfw.x11.XdndAware = XInternAtom(_glfw.x11.display, "XdndAware", False);
	_glfw.x11.XdndEnter = XInternAtom(_glfw.x11.display, "XdndEnter", False);
	_glfw.x11.XdndPosition = XInternAtom(_glfw.x11.display, "XdndPosition", False);
	_glfw.x11.XdndStatus = XInternAtom(_glfw.x11.display, "XdndStatus", False);
	_glfw.x11.XdndActionCopy = XInternAtom(_glfw.x11.display, "XdndActionCopy", False);
	_glfw.x11.XdndDrop = XInternAtom(_glfw.x11.display, "XdndDrop", False);
	_glfw.x11.XdndFinished = XInternAtom(_glfw.x11.display, "XdndFinished", False);
	_glfw.x11.XdndSelection = XInternAtom(_glfw.x11.display, "XdndSelection", False);
	_glfw.x11.XdndTypeList = XInternAtom(_glfw.x11.display, "XdndTypeList", False);
	_glfw.x11.text_uri_list = XInternAtom(_glfw.x11.display, "text/uri-list", False);

	// ICCCM, EWMH and Motif window property atoms
	// These can be set safely even without WM support
	// The EWMH atoms that require WM support are handled in detectEWMH
	_glfw.x11.WM_PROTOCOLS =
		XInternAtom(_glfw.x11.display, "WM_PROTOCOLS", False);
	_glfw.x11.WM_STATE =
		XInternAtom(_glfw.x11.display, "WM_STATE", False);
	_glfw.x11.WM_DELETE_WINDOW =
		XInternAtom(_glfw.x11.display, "WM_DELETE_WINDOW", False);
	_glfw.x11.NET_SUPPORTED =
		XInternAtom(_glfw.x11.display, "_NET_SUPPORTED", False);
	_glfw.x11.NET_SUPPORTING_WM_CHECK =
		XInternAtom(_glfw.x11.display, "_NET_SUPPORTING_WM_CHECK", False);
	_glfw.x11.NET_WM_ICON =
		XInternAtom(_glfw.x11.display, "_NET_WM_ICON", False);
	_glfw.x11.NET_WM_PING =
		XInternAtom(_glfw.x11.display, "_NET_WM_PING", False);
	_glfw.x11.NET_WM_PID =
		XInternAtom(_glfw.x11.display, "_NET_WM_PID", False);
	_glfw.x11.NET_WM_NAME =
		XInternAtom(_glfw.x11.display, "_NET_WM_NAME", False);
	_glfw.x11.NET_WM_ICON_NAME =
		XInternAtom(_glfw.x11.display, "_NET_WM_ICON_NAME", False);
	_glfw.x11.NET_WM_BYPASS_COMPOSITOR =
		XInternAtom(_glfw.x11.display, "_NET_WM_BYPASS_COMPOSITOR", False);
	_glfw.x11.NET_WM_WINDOW_OPACITY =
		XInternAtom(_glfw.x11.display, "_NET_WM_WINDOW_OPACITY", False);
	_glfw.x11.MOTIF_WM_HINTS =
		XInternAtom(_glfw.x11.display, "_MOTIF_WM_HINTS", False);

	// The compositing manager selection name contains the screen number
	{
		char name[32];
		snprintf(name, sizeof(name), "_NET_WM_CM_S%u", _glfw.x11.screen);
		_glfw.x11.NET_WM_CM_Sx = XInternAtom(_glfw.x11.display, name, False);
	}

	// Detect whether an EWMH-conformant window manager is running
	detectEWMH();
}

// Retrieve system content scale via folklore heuristics
//
static void getSystemContentScale(float* xscale, float* yscale)
{
	// Start by assuming the default X11 DPI
	// NOTE: Some desktop environments (KDE) may remove the Xft.dpi field when it
	//       would be set to 96, so assume that is the case if we cannot find it
	float xdpi = 96.f, ydpi = 96.f;

	// NOTE: Basing the scale on Xft.dpi where available should provide the most
	//       consistent user experience (matches Qt, Gtk, etc), although not
	//       always the most accurate one
	char* rms = XResourceManagerString(_glfw.x11.display);
	if (rms)
	{
		XrmDatabase db = XrmGetStringDatabase(rms);
		if (db)
		{
			XrmValue value;
			char* type = NULL;

			if (XrmGetResource(db, "Xft.dpi", "Xft.Dpi", &type, &value))
			{
				if (type && strcmp(type, "String") == 0)
					xdpi = ydpi = atof(value.addr);
			}

			XrmDestroyDatabase(db);
		}
	}

	*xscale = xdpi / 96.f;
	*yscale = ydpi / 96.f;
}

// Create a blank cursor for hidden and disabled cursor modes
//
static Cursor createHiddenCursor(void)
{
	unsigned char pixels[16 * 16 * 4] = { 0 };
	ImageData image = { 16, 16, pixels };
	return _glfwCreateNativeCursorX11(&image, 0, 0);
}

// Create a helper window for IPC
//
static Window createHelperWindow(void)
{
	XSetWindowAttributes wa;
	wa.event_mask = PropertyChangeMask;

	return XCreateWindow(_glfw.x11.display, _glfw.x11.root,
						 0, 0, 1, 1, 0, 0,
						 InputOnly,
						 DefaultVisual(_glfw.x11.display, _glfw.x11.screen),
						 CWEventMask, &wa);
}

// Create the pipe for empty events without assumuing the OS has pipe2(2)
//
static ErrorResponse* createEmptyEventPipe(void)
{
	if (pipe(_glfw.x11.emptyEventPipe) != 0)
	{
		return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to create empty event pipe: %s", strerror(errno));
	}

	for (int i = 0; i < 2; i++)
	{
		const int sf = fcntl(_glfw.x11.emptyEventPipe[i], F_GETFL, 0);
		const int df = fcntl(_glfw.x11.emptyEventPipe[i], F_GETFD, 0);

		if (sf == -1 || df == -1 ||
			fcntl(_glfw.x11.emptyEventPipe[i], F_SETFL, sf | O_NONBLOCK) == -1 ||
			fcntl(_glfw.x11.emptyEventPipe[i], F_SETFD, df | FD_CLOEXEC) == -1)
		{
			return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to set flags for empty event pipe: %s", strerror(errno));
		}
	}

	return NULL;
}

// X error handler
//
static int errorHandler(Display *display, XErrorEvent* event)
{
	if (_glfw.x11.display != display)
		return 0;

	_glfw.x11.errorCode = event->error_code;
	return 0;
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Sets the X error handler callback
//
void _glfwGrabErrorHandlerX11(void)
{
	_glfw.x11.errorCode = Success;
	_glfw.x11.errorHandler = XSetErrorHandler(errorHandler);
}

// Clears the X error handler callback
//
void _glfwReleaseErrorHandlerX11(void)
{
	// Synchronize to make sure all commands are processed
	XSync(_glfw.x11.display, False);
	XSetErrorHandler(_glfw.x11.errorHandler);
	_glfw.x11.errorHandler = NULL;
}

// Reports the specified error, appending information about the last X error
//
void _glfwInputErrorX11(int error, const char* message)
{
	char buffer[ERROR_MSG_SIZE];
	XGetErrorText(_glfw.x11.display, _glfw.x11.errorCode,
				  buffer, sizeof(buffer));

	_glfwInputError(error, "%s: %s", message, buffer);
}

// Creates a native cursor object from the specified image and hotspot
//
Cursor _glfwCreateNativeCursorX11(const ImageData* image, int xhot, int yhot)
{
	Cursor cursor;

	if (!_glfw.x11.xcursor.handle)
		return None;

	XcursorImage* native = XcursorImageCreate(image->width, image->height);
	if (native == NULL)
		return None;

	native->xhot = xhot;
	native->yhot = yhot;

	unsigned char* source = (unsigned char*) image->pixels;
	XcursorPixel* target = native->pixels;

	for (int i = 0;  i < image->width * image->height;  i++, target++, source += 4)
	{
		unsigned int alpha = source[3];

		*target = (alpha << 24) |
				  ((unsigned char) ((source[0] * alpha) / 255) << 16) |
				  ((unsigned char) ((source[1] * alpha) / 255) <<  8) |
				  ((unsigned char) ((source[2] * alpha) / 255) <<  0);
	}

	cursor = XcursorImageLoadCursor(_glfw.x11.display, native);
	XcursorImageDestroy(native);

	return cursor;
}

//////////////////////////////////////////////////////////////////////////
//////                       GLFW platform API                      //////
//////////////////////////////////////////////////////////////////////////

ErrorResponse* platformInit(_GLFWplatform* platform)
{
	// HACK: If the application has left the locale as "C" then both wide
	//       character text input and explicit UTF-8 input via XIM will break
	//       This sets the CTYPE part of the current locale from the environment
	//       in the hope that it is set to something more sane than "C"
	if (strcmp(setlocale(LC_CTYPE, NULL), "C") == 0)
		setlocale(LC_CTYPE, "");

	void* module = _glfwPlatformLoadModule("libX11.so.6");
	if (!module)
	{
		return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to load Xlib");
	}

	PFN_XInitThreads XInitThreads = (PFN_XInitThreads)_glfwPlatformGetModuleSymbol(module, "XInitThreads");
	PFN_XrmInitialize XrmInitialize = (PFN_XrmInitialize)_glfwPlatformGetModuleSymbol(module, "XrmInitialize");
	PFN_XOpenDisplay XOpenDisplay = (PFN_XOpenDisplay)_glfwPlatformGetModuleSymbol(module, "XOpenDisplay");
	if (!XInitThreads || !XrmInitialize || !XOpenDisplay) {
		_glfwPlatformFreeModule(module);
		return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to load Xlib entry point");
	}

	XInitThreads();
	XrmInitialize();

	Display* display = XOpenDisplay(NULL);
	if (!display) {
		ErrorResponse* errRsp;
		const char* name = getenv("DISPLAY");
		if (name) {
			errRsp = createErrorResponse(ERR_PLATFORM_UNAVAILABLE, "Failed to open display %s", name);
		} else {
			errRsp = createErrorResponse(ERR_PLATFORM_UNAVAILABLE, "The DISPLAY environment variable is missing");
		}
		_glfwPlatformFreeModule(module);
		return errRsp;
	}

	_glfw.x11.display = display;
	_glfw.x11.xlib.handle = module;

	platform->setCursorMode = _glfwSetCursorModeX11;
	platform->createCursor = _glfwCreateCursorX11;
	platform->createStandardCursor = _glfwCreateStandardCursorX11;
	platform->destroyCursor = _glfwDestroyCursorX11;
	platform->setCursor = _glfwSetCursorX11;
	platform->getKeyScancode = _glfwGetKeyScancodeX11;
	platform->freeMonitor = _glfwFreeMonitorX11;
	platform->getMonitorPos = _glfwGetMonitorPosX11;
	platform->getMonitorContentScale = _glfwGetMonitorContentScaleX11;
	platform->getMonitorWorkarea = _glfwGetMonitorWorkareaX11;
	platform->getVideoModes = _glfwGetVideoModesX11;
	platform->getVideoMode = _glfwGetVideoModeX11;
	platform->getGammaRamp = _glfwGetGammaRampX11;
	platform->setGammaRamp = _glfwSetGammaRampX11;
	platform->createWindow = _glfwCreateWindowX11;
	platform->destroyWindow = _glfwDestroyWindowX11;
	platform->setWindowTitle = _glfwSetWindowTitleX11;
	platform->setWindowIcon = _glfwSetWindowIconX11;
	platform->getWindowPos = _glfwGetWindowPosX11;
	platform->setWindowPos = _glfwSetWindowPosX11;
	platform->getWindowSize = _glfwGetWindowSizeX11;
	platform->setWindowSize = _glfwSetWindowSizeX11;
	platform->setWindowSizeLimits = _glfwSetWindowSizeLimitsX11;
	platform->setWindowAspectRatio = _glfwSetWindowAspectRatioX11;
	platform->getFramebufferSize = _glfwGetFramebufferSizeX11;
	platform->getWindowFrameSize = _glfwGetWindowFrameSizeX11;
	platform->getWindowContentScale = _glfwGetWindowContentScaleX11;
	platform->iconifyWindow = _glfwIconifyWindowX11;
	platform->restoreWindow = _glfwRestoreWindowX11;
	platform->maximizeWindow = _glfwMaximizeWindowX11;
	platform->showWindow = _glfwShowWindowX11;
	platform->hideWindow = _glfwHideWindowX11;
	platform->requestWindowAttention = _glfwRequestWindowAttentionX11;
	platform->focusWindow = _glfwFocusWindowX11;
	platform->setWindowMonitor = _glfwSetWindowMonitorX11;
	platform->windowFocused = _glfwWindowFocusedX11;
	platform->windowIconified = _glfwWindowIconifiedX11;
	platform->windowVisible = _glfwWindowVisibleX11;
	platform->windowMaximized = _glfwWindowMaximizedX11;
	platform->windowHovered = _glfwWindowHoveredX11;
	platform->framebufferTransparent = _glfwFramebufferTransparentX11;
	platform->getWindowOpacity = _glfwGetWindowOpacityX11;
	platform->setWindowResizable = _glfwSetWindowResizableX11;
	platform->setWindowDecorated = _glfwSetWindowDecoratedX11;
	platform->setWindowFloating = _glfwSetWindowFloatingX11;
	platform->setWindowOpacity = _glfwSetWindowOpacityX11;
	platform->setWindowMousePassthrough = _glfwSetWindowMousePassthroughX11;
	platform->pollEvents = _glfwPollEventsX11;
	platform->waitEvents = _glfwWaitEventsX11;
	platform->waitEventsTimeout = _glfwWaitEventsTimeoutX11;
	platform->postEmptyEvent = _glfwPostEmptyEventX11;

	_glfw.x11.xlib.AllocClassHint = (PFN_XAllocClassHint)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XAllocClassHint");
	_glfw.x11.xlib.AllocSizeHints = (PFN_XAllocSizeHints)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XAllocSizeHints");
	_glfw.x11.xlib.AllocWMHints = (PFN_XAllocWMHints)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XAllocWMHints");
	_glfw.x11.xlib.ChangeProperty = (PFN_XChangeProperty)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XChangeProperty");
	_glfw.x11.xlib.ChangeWindowAttributes = (PFN_XChangeWindowAttributes)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XChangeWindowAttributes");
	_glfw.x11.xlib.CheckIfEvent = (PFN_XCheckIfEvent)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XCheckIfEvent");
	_glfw.x11.xlib.CheckTypedWindowEvent = (PFN_XCheckTypedWindowEvent)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XCheckTypedWindowEvent");
	_glfw.x11.xlib.CloseDisplay = (PFN_XCloseDisplay)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XCloseDisplay");
	_glfw.x11.xlib.CloseIM = (PFN_XCloseIM)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XCloseIM");
	_glfw.x11.xlib.ConvertSelection = (PFN_XConvertSelection)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XConvertSelection");
	_glfw.x11.xlib.CreateColormap = (PFN_XCreateColormap)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XCreateColormap");
	_glfw.x11.xlib.CreateFontCursor = (PFN_XCreateFontCursor)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XCreateFontCursor");
	_glfw.x11.xlib.CreateIC = (PFN_XCreateIC)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XCreateIC");
	_glfw.x11.xlib.CreateRegion = (PFN_XCreateRegion)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XCreateRegion");
	_glfw.x11.xlib.CreateWindow = (PFN_XCreateWindow)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XCreateWindow");
	_glfw.x11.xlib.DefineCursor = (PFN_XDefineCursor)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XDefineCursor");
	_glfw.x11.xlib.DeleteContext = (PFN_XDeleteContext)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XDeleteContext");
	_glfw.x11.xlib.DeleteProperty = (PFN_XDeleteProperty)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XDeleteProperty");
	_glfw.x11.xlib.DestroyIC = (PFN_XDestroyIC)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XDestroyIC");
	_glfw.x11.xlib.DestroyRegion = (PFN_XDestroyRegion)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XDestroyRegion");
	_glfw.x11.xlib.DestroyWindow = (PFN_XDestroyWindow)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XDestroyWindow");
	_glfw.x11.xlib.DisplayKeycodes = (PFN_XDisplayKeycodes)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XDisplayKeycodes");
	_glfw.x11.xlib.EventsQueued = (PFN_XEventsQueued)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XEventsQueued");
	_glfw.x11.xlib.FilterEvent = (PFN_XFilterEvent)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XFilterEvent");
	_glfw.x11.xlib.FindContext = (PFN_XFindContext)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XFindContext");
	_glfw.x11.xlib.Flush = (PFN_XFlush)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XFlush");
	_glfw.x11.xlib.Free = (PFN_XFree)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XFree");
	_glfw.x11.xlib.FreeColormap = (PFN_XFreeColormap)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XFreeColormap");
	_glfw.x11.xlib.FreeCursor = (PFN_XFreeCursor)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XFreeCursor");
	_glfw.x11.xlib.FreeEventData = (PFN_XFreeEventData)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XFreeEventData");
	_glfw.x11.xlib.GetErrorText = (PFN_XGetErrorText)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetErrorText");
	_glfw.x11.xlib.GetEventData = (PFN_XGetEventData)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetEventData");
	_glfw.x11.xlib.GetICValues = (PFN_XGetICValues)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetICValues");
	_glfw.x11.xlib.GetIMValues = (PFN_XGetIMValues)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetIMValues");
	_glfw.x11.xlib.GetInputFocus = (PFN_XGetInputFocus)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetInputFocus");
	_glfw.x11.xlib.GetKeyboardMapping = (PFN_XGetKeyboardMapping)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetKeyboardMapping");
	_glfw.x11.xlib.GetScreenSaver = (PFN_XGetScreenSaver)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetScreenSaver");
	_glfw.x11.xlib.GetSelectionOwner = (PFN_XGetSelectionOwner)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetSelectionOwner");
	_glfw.x11.xlib.GetVisualInfo = (PFN_XGetVisualInfo)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetVisualInfo");
	_glfw.x11.xlib.GetWMNormalHints = (PFN_XGetWMNormalHints)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetWMNormalHints");
	_glfw.x11.xlib.GetWindowAttributes = (PFN_XGetWindowAttributes)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetWindowAttributes");
	_glfw.x11.xlib.GetWindowProperty = (PFN_XGetWindowProperty)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGetWindowProperty");
	_glfw.x11.xlib.GrabPointer = (PFN_XGrabPointer)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XGrabPointer");
	_glfw.x11.xlib.IconifyWindow = (PFN_XIconifyWindow)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XIconifyWindow");
	_glfw.x11.xlib.InternAtom = (PFN_XInternAtom)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XInternAtom");
	_glfw.x11.xlib.LookupString = (PFN_XLookupString)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XLookupString");
	_glfw.x11.xlib.MapRaised = (PFN_XMapRaised)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XMapRaised");
	_glfw.x11.xlib.MapWindow = (PFN_XMapWindow)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XMapWindow");
	_glfw.x11.xlib.MoveResizeWindow = (PFN_XMoveResizeWindow)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XMoveResizeWindow");
	_glfw.x11.xlib.MoveWindow = (PFN_XMoveWindow)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XMoveWindow");
	_glfw.x11.xlib.NextEvent = (PFN_XNextEvent)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XNextEvent");
	_glfw.x11.xlib.OpenIM = (PFN_XOpenIM)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XOpenIM");
	_glfw.x11.xlib.PeekEvent = (PFN_XPeekEvent)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XPeekEvent");
	_glfw.x11.xlib.Pending = (PFN_XPending)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XPending");
	_glfw.x11.xlib.QueryExtension = (PFN_XQueryExtension)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XQueryExtension");
	_glfw.x11.xlib.QueryPointer = (PFN_XQueryPointer)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XQueryPointer");
	_glfw.x11.xlib.RaiseWindow = (PFN_XRaiseWindow)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XRaiseWindow");
	_glfw.x11.xlib.RegisterIMInstantiateCallback = (PFN_XRegisterIMInstantiateCallback)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XRegisterIMInstantiateCallback");
	_glfw.x11.xlib.ResizeWindow = (PFN_XResizeWindow)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XResizeWindow");
	_glfw.x11.xlib.ResourceManagerString = (PFN_XResourceManagerString)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XResourceManagerString");
	_glfw.x11.xlib.SaveContext = (PFN_XSaveContext)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSaveContext");
	_glfw.x11.xlib.SelectInput = (PFN_XSelectInput)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSelectInput");
	_glfw.x11.xlib.SendEvent = (PFN_XSendEvent)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSendEvent");
	_glfw.x11.xlib.SetClassHint = (PFN_XSetClassHint)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetClassHint");
	_glfw.x11.xlib.SetErrorHandler = (PFN_XSetErrorHandler)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetErrorHandler");
	_glfw.x11.xlib.SetICFocus = (PFN_XSetICFocus)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetICFocus");
	_glfw.x11.xlib.SetIMValues = (PFN_XSetIMValues)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetIMValues");
	_glfw.x11.xlib.SetInputFocus = (PFN_XSetInputFocus)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetInputFocus");
	_glfw.x11.xlib.SetLocaleModifiers = (PFN_XSetLocaleModifiers)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetLocaleModifiers");
	_glfw.x11.xlib.SetScreenSaver = (PFN_XSetScreenSaver)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetScreenSaver");
	_glfw.x11.xlib.SetSelectionOwner = (PFN_XSetSelectionOwner)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetSelectionOwner");
	_glfw.x11.xlib.SetWMHints = (PFN_XSetWMHints)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetWMHints");
	_glfw.x11.xlib.SetWMNormalHints = (PFN_XSetWMNormalHints)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetWMNormalHints");
	_glfw.x11.xlib.SetWMProtocols = (PFN_XSetWMProtocols)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSetWMProtocols");
	_glfw.x11.xlib.SupportsLocale = (PFN_XSupportsLocale)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSupportsLocale");
	_glfw.x11.xlib.Sync = (PFN_XSync)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XSync");
	_glfw.x11.xlib.TranslateCoordinates = (PFN_XTranslateCoordinates)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XTranslateCoordinates");
	_glfw.x11.xlib.UndefineCursor = (PFN_XUndefineCursor)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XUndefineCursor");
	_glfw.x11.xlib.UngrabPointer = (PFN_XUngrabPointer)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XUngrabPointer");
	_glfw.x11.xlib.UnmapWindow = (PFN_XUnmapWindow)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XUnmapWindow");
	_glfw.x11.xlib.UnsetICFocus = (PFN_XUnsetICFocus)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XUnsetICFocus");
	_glfw.x11.xlib.VisualIDFromVisual = (PFN_XVisualIDFromVisual)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XVisualIDFromVisual");
	_glfw.x11.xlib.WarpPointer = (PFN_XWarpPointer)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XWarpPointer");
	_glfw.x11.xkb.FreeKeyboard = (PFN_XkbFreeKeyboard)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XkbFreeKeyboard");
	_glfw.x11.xkb.FreeNames = (PFN_XkbFreeNames)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XkbFreeNames");
	_glfw.x11.xkb.GetMap = (PFN_XkbGetMap)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XkbGetMap");
	_glfw.x11.xkb.GetNames = (PFN_XkbGetNames)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XkbGetNames");
	_glfw.x11.xkb.GetState = (PFN_XkbGetState)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XkbGetState");
	_glfw.x11.xkb.KeycodeToKeysym = (PFN_XkbKeycodeToKeysym)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XkbKeycodeToKeysym");
	_glfw.x11.xkb.QueryExtension = (PFN_XkbQueryExtension)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XkbQueryExtension");
	_glfw.x11.xkb.SelectEventDetails = (PFN_XkbSelectEventDetails)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XkbSelectEventDetails");
	_glfw.x11.xkb.SetDetectableAutoRepeat = (PFN_XkbSetDetectableAutoRepeat)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XkbSetDetectableAutoRepeat");
	_glfw.x11.xrm.DestroyDatabase = (PFN_XrmDestroyDatabase)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XrmDestroyDatabase");
	_glfw.x11.xrm.GetResource = (PFN_XrmGetResource)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XrmGetResource");
	_glfw.x11.xrm.GetStringDatabase = (PFN_XrmGetStringDatabase)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XrmGetStringDatabase");
	_glfw.x11.xrm.UniqueQuark = (PFN_XrmUniqueQuark)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XrmUniqueQuark");
	_glfw.x11.xlib.UnregisterIMInstantiateCallback = (PFN_XUnregisterIMInstantiateCallback)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "XUnregisterIMInstantiateCallback");
	_glfw.x11.xlib.utf8LookupString = (PFN_Xutf8LookupString)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "Xutf8LookupString");
	_glfw.x11.xlib.utf8SetWMProperties = (PFN_Xutf8SetWMProperties)
		_glfwPlatformGetModuleSymbol(_glfw.x11.xlib.handle, "Xutf8SetWMProperties");

	if (_glfw.x11.xlib.utf8LookupString && _glfw.x11.xlib.utf8SetWMProperties)
		_glfw.x11.xlib.utf8 = true;

	_glfw.x11.screen = DefaultScreen(_glfw.x11.display);
	_glfw.x11.root = RootWindow(_glfw.x11.display, _glfw.x11.screen);
	_glfw.x11.context = XUniqueContext();

	getSystemContentScale(&_glfw.x11.contentScaleX, &_glfw.x11.contentScaleY);

	ErrorResponse* errRsp = createEmptyEventPipe();
	if (errRsp) {
		_terminate();
		return errRsp;
	}

	initExtensions();

	_glfw.x11.helperWindowHandle = createHelperWindow();
	_glfw.x11.hiddenCursorHandle = createHiddenCursor();

	if (XSupportsLocale() && _glfw.x11.xlib.utf8)
	{
		XSetLocaleModifiers("");

		// If an IM is already present our callback will be called right away
		XRegisterIMInstantiateCallback(_glfw.x11.display,
									   NULL, NULL, NULL,
									   inputMethodInstantiateCallback,
									   NULL);
	}

	_glfwPollMonitorsX11();
	return NULL;
}

void platformTerminate(void)
{
	if (_glfw.x11.helperWindowHandle)
	{
		if (XGetSelectionOwner(_glfw.x11.display, _glfw.x11.CLIPBOARD) ==
			_glfw.x11.helperWindowHandle)
		{
			_glfwPushSelectionToManagerX11();
		}

		XDestroyWindow(_glfw.x11.display, _glfw.x11.helperWindowHandle);
		_glfw.x11.helperWindowHandle = None;
	}

	if (_glfw.x11.hiddenCursorHandle)
	{
		XFreeCursor(_glfw.x11.display, _glfw.x11.hiddenCursorHandle);
		_glfw.x11.hiddenCursorHandle = (Cursor) 0;
	}

	XUnregisterIMInstantiateCallback(_glfw.x11.display,
									 NULL, NULL, NULL,
									 inputMethodInstantiateCallback,
									 NULL);

	if (_glfw.x11.im)
	{
		XCloseIM(_glfw.x11.im);
		_glfw.x11.im = NULL;
	}

	if (_glfw.x11.display)
	{
		XCloseDisplay(_glfw.x11.display);
		_glfw.x11.display = NULL;
	}

	if (_glfw.x11.x11xcb.handle)
	{
		_glfwPlatformFreeModule(_glfw.x11.x11xcb.handle);
		_glfw.x11.x11xcb.handle = NULL;
	}

	if (_glfw.x11.xcursor.handle)
	{
		_glfwPlatformFreeModule(_glfw.x11.xcursor.handle);
		_glfw.x11.xcursor.handle = NULL;
	}

	if (_glfw.x11.randr.handle)
	{
		_glfwPlatformFreeModule(_glfw.x11.randr.handle);
		_glfw.x11.randr.handle = NULL;
	}

	if (_glfw.x11.xinerama.handle)
	{
		_glfwPlatformFreeModule(_glfw.x11.xinerama.handle);
		_glfw.x11.xinerama.handle = NULL;
	}

	if (_glfw.x11.xrender.handle)
	{
		_glfwPlatformFreeModule(_glfw.x11.xrender.handle);
		_glfw.x11.xrender.handle = NULL;
	}

	if (_glfw.x11.vidmode.handle)
	{
		_glfwPlatformFreeModule(_glfw.x11.vidmode.handle);
		_glfw.x11.vidmode.handle = NULL;
	}

	if (_glfw.x11.xi.handle)
	{
		_glfwPlatformFreeModule(_glfw.x11.xi.handle);
		_glfw.x11.xi.handle = NULL;
	}

	// NOTE: These need to be unloaded after XCloseDisplay, as they register
	//       cleanup callbacks that get called by that function
	_glfwTerminateGLX();

	if (_glfw.x11.xlib.handle)
	{
		_glfwPlatformFreeModule(_glfw.x11.xlib.handle);
		_glfw.x11.xlib.handle = NULL;
	}

	if (_glfw.x11.emptyEventPipe[0] || _glfw.x11.emptyEventPipe[1])
	{
		close(_glfw.x11.emptyEventPipe[0]);
		close(_glfw.x11.emptyEventPipe[1]);
	}
}

#endif // PLATFORM_LINUX
