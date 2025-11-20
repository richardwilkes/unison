#if defined(__linux__)

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

	memset(_glfw.x11Keycodes, -1, sizeof(_glfw.x11Keycodes));
	memset(_glfw.x11Scancodes, -1, sizeof(_glfw.x11Scancodes));

	if (_glfw.xkbAvailable)
	{
		// Use XKB to determine physical key locations independently of the
		// current keyboard layout

		XkbDescPtr desc = _glfw.xkbGetMap(_glfw.x11Display, 0, XkbUseCoreKbd);
		_glfw.xkbGetNames(_glfw.x11Display, XkbKeyNamesMask | XkbKeyAliasesMask, desc);

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

			_glfw.x11Keycodes[scancode] = key;
		}

		_glfw.xkbFreeNames(desc, XkbKeyNamesMask, True);
		_glfw.xkbFreeKeyboard(desc, 0, True);
	}
	else
		_glfw.xlibDisplayKeycodes(_glfw.x11Display, &scancodeMin, &scancodeMax);

	int width;
	KeySym* keysyms = _glfw.xlibGetKeyboardMapping(_glfw.x11Display,
										  scancodeMin,
										  scancodeMax - scancodeMin + 1,
										  &width);

	for (int scancode = scancodeMin;  scancode <= scancodeMax;  scancode++)
	{
		// Translate the un-translated key codes using traditional X11 KeySym
		// lookups
		if (_glfw.x11Keycodes[scancode] < 0)
		{
			const size_t base = (scancode - scancodeMin) * width;
			_glfw.x11Keycodes[scancode] = translateKeySyms(&keysyms[base], width);
		}

		// Store the reverse translation for faster key name lookup
		if (_glfw.x11Keycodes[scancode] > 0)
			_glfw.x11Scancodes[_glfw.x11Keycodes[scancode]] = scancode;
	}

	_glfw.xlibFree(keysyms);
}

// Check whether the IM has a usable style
//
static IntBool hasUsableInputMethodStyle(void)
{
	IntBool found = false;
	XIMStyles* styles = NULL;

	if (_glfw.xlibGetIMValues(_glfw.x11IM, XNQueryInputStyle, &styles, NULL) != NULL)
		return false;

	for (unsigned int i = 0;  i < styles->count_styles;  i++)
	{
		if (styles->supported_styles[i] == (XIMPreeditNothing | XIMStatusNothing))
		{
			found = true;
			break;
		}
	}

	_glfw.xlibFree(styles);
	return found;
}

static void inputMethodDestroyCallback(XIM im, XPointer clientData, XPointer callData)
{
	_glfw.x11IM = NULL;
}

static void inputMethodInstantiateCallback(Display* display,
										   XPointer clientData,
										   XPointer callData)
{
	if (_glfw.x11IM)
		return;

	_glfw.x11IM = _glfw.xlibOpenIM(_glfw.x11Display, 0, NULL, NULL);
	if (_glfw.x11IM)
	{
		if (!hasUsableInputMethodStyle())
		{
			_glfw.xlibCloseIM(_glfw.x11IM);
			_glfw.x11IM = NULL;
		}
	}

	if (_glfw.x11IM)
	{
		XIMCallback callback;
		callback.callback = (XIMProc) inputMethodDestroyCallback;
		callback.client_data = NULL;
		_glfw.xlibSetIMValues(_glfw.x11IM, XNDestroyCallback, &callback, NULL);

		for (plafWindow* window = _glfw.windowListHead;  window;  window = window->next)
			_glfwCreateInputContextX11(window);
	}
}

// Return the atom ID only if it is listed in the specified array
//
static Atom getAtomIfSupported(Atom* supportedAtoms,
							   unsigned long atomCount,
							   const char* atomName)
{
	const Atom atom = _glfw.xlibInternAtom(_glfw.x11Display, atomName, False);

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
	if (!_glfwGetWindowPropertyX11(_glfw.x11Root,
								   _glfw.x11NET_SUPPORTING_WM_CHECK,
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
								   _glfw.x11NET_SUPPORTING_WM_CHECK,
								   XA_WINDOW,
								   (unsigned char**) &windowFromChild))
	{
		_glfwReleaseErrorHandlerX11();
		_glfw.xlibFree(windowFromRoot);
		return;
	}

	_glfwReleaseErrorHandlerX11();

	// If the property exists, it should contain the XID of the window

	if (*windowFromRoot != *windowFromChild)
	{
		_glfw.xlibFree(windowFromRoot);
		_glfw.xlibFree(windowFromChild);
		return;
	}

	_glfw.xlibFree(windowFromRoot);
	_glfw.xlibFree(windowFromChild);

	// We are now fairly sure that an EWMH-compliant WM is currently running
	// We can now start querying the WM about what features it supports by
	// looking in the _NET_SUPPORTED property on the root window
	// It should contain a list of supported EWMH protocol and state atoms

	Atom* supportedAtoms = NULL;
	const unsigned long atomCount =
		_glfwGetWindowPropertyX11(_glfw.x11Root,
								  _glfw.x11NET_SUPPORTED,
								  XA_ATOM,
								  (unsigned char**) &supportedAtoms);

	// See which of the atoms we support that are supported by the WM

	_glfw.x11NET_WM_STATE =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE");
	_glfw.x11NET_WM_STATE_ABOVE =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_ABOVE");
	_glfw.x11NET_WM_STATE_FULLSCREEN =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_FULLSCREEN");
	_glfw.x11NET_WM_STATE_MAXIMIZED_VERT =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_MAXIMIZED_VERT");
	_glfw.x11NET_WM_STATE_MAXIMIZED_HORZ =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_MAXIMIZED_HORZ");
	_glfw.x11NET_WM_STATE_DEMANDS_ATTENTION =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_DEMANDS_ATTENTION");
	_glfw.x11NET_WM_FULLSCREEN_MONITORS =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_FULLSCREEN_MONITORS");
	_glfw.x11NET_WM_WINDOW_TYPE =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_WINDOW_TYPE");
	_glfw.x11NET_WM_WINDOW_TYPE_NORMAL =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_WINDOW_TYPE_NORMAL");
	_glfw.x11NET_WORKAREA =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WORKAREA");
	_glfw.x11NET_CURRENT_DESKTOP =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_CURRENT_DESKTOP");
	_glfw.x11NET_ACTIVE_WINDOW =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_ACTIVE_WINDOW");
	_glfw.x11NET_FRAME_EXTENTS =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_FRAME_EXTENTS");
	_glfw.x11NET_REQUEST_FRAME_EXTENTS =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_REQUEST_FRAME_EXTENTS");

	if (supportedAtoms)
		_glfw.xlibFree(supportedAtoms);
}

// Look for and initialize supported X11 extensions
//
static void initExtensions(void)
{
	_glfw.xvidmodeHandle = _glfwPlatformLoadModule("libXxf86vm.so.1");
	if (_glfw.xvidmodeHandle)
	{
		_glfw.xvidmodeQueryExtension = (FN_XF86VidModeQueryExtension)
			_glfwPlatformGetModuleSymbol(_glfw.xvidmodeHandle, "XF86VidModeQueryExtension");
		_glfw.xvidmodeGetGammaRamp = (FN_XF86VidModeGetGammaRamp)
			_glfwPlatformGetModuleSymbol(_glfw.xvidmodeHandle, "XF86VidModeGetGammaRamp");
		_glfw.xvidmodeSetGammaRamp = (FN_XF86VidModeSetGammaRamp)
			_glfwPlatformGetModuleSymbol(_glfw.xvidmodeHandle, "XF86VidModeSetGammaRamp");
		_glfw.xvidmodeGetGammaRampSize = (FN_XF86VidModeGetGammaRampSize)
			_glfwPlatformGetModuleSymbol(_glfw.xvidmodeHandle, "XF86VidModeGetGammaRampSize");

		int eventBase;
		int errorBase;
		_glfw.xvidmodeAvailable = _glfw.xvidmodeQueryExtension(_glfw.x11Display, &eventBase, &errorBase);
	}

	_glfw.xiHandle = _glfwPlatformLoadModule("libXi.so.6");
	if (_glfw.xiHandle)
	{
		_glfw.xiQueryVersion = (FN_XIQueryVersion)
			_glfwPlatformGetModuleSymbol(_glfw.xiHandle, "XIQueryVersion");

		int majorOpcode;
		int eventBase;
		int errorBase;
		if (_glfw.xlibQueryExtension(_glfw.x11Display, "XInputExtension", &majorOpcode, &eventBase, &errorBase)) {
			int major = 2;
			int minor = 0;
			if (_glfw.xiQueryVersion(_glfw.x11Display, &major, &minor) == Success) {
				_glfw.xiAvailable = true;
			}
		}
	}
	_glfw.randrHandle = _glfwPlatformLoadModule("libXrandr.so.2");
	if (_glfw.randrHandle)
	{
		_glfw.randrAllocGamma = (FN_XRRAllocGamma)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRAllocGamma");
		_glfw.randrFreeGamma = (FN_XRRFreeGamma)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRFreeGamma");
		_glfw.randrFreeCrtcInfo = (FN_XRRFreeCrtcInfo)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRFreeCrtcInfo");
		_glfw.randrFreeOutputInfo = (FN_XRRFreeOutputInfo)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRFreeOutputInfo");
		_glfw.randrFreeScreenResources = (FN_XRRFreeScreenResources)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRFreeScreenResources");
		_glfw.randrGetCrtcGamma = (FN_XRRGetCrtcGamma)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRGetCrtcGamma");
		_glfw.randrGetCrtcGammaSize = (FN_XRRGetCrtcGammaSize)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRGetCrtcGammaSize");
		_glfw.randrGetCrtcInfo = (FN_XRRGetCrtcInfo)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRGetCrtcInfo");
		_glfw.randrGetOutputInfo = (FN_XRRGetOutputInfo)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRGetOutputInfo");
		_glfw.randrGetOutputPrimary = (FN_XRRGetOutputPrimary)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRGetOutputPrimary");
		_glfw.randrGetScreenResourcesCurrent = (FN_XRRGetScreenResourcesCurrent)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRGetScreenResourcesCurrent");
		_glfw.randrQueryExtension = (FN_XRRQueryExtension)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRQueryExtension");
		_glfw.randrQueryVersion = (FN_XRRQueryVersion)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRQueryVersion");
		_glfw.randrSelectInput = (FN_XRRSelectInput)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRSelectInput");
		_glfw.randrSetCrtcConfig = (FN_XRRSetCrtcConfig)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRSetCrtcConfig");
		_glfw.randrSetCrtcGamma = (FN_XRRSetCrtcGamma)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRSetCrtcGamma");
		_glfw.randrUpdateConfiguration = (FN_XRRUpdateConfiguration)
			_glfwPlatformGetModuleSymbol(_glfw.randrHandle, "XRRUpdateConfiguration");

		int errorBase;
		if (_glfw.randrQueryExtension(_glfw.x11Display,
							  &_glfw.randrEventBase,
							  &errorBase))
		{
			int major;
			int minor;
			if (_glfw.randrQueryVersion(_glfw.x11Display, &major, &minor))
			{
				// The GLFW RandR path requires at least version 1.3
				if (major > 1 || minor >= 3)
					_glfw.randrAvailable = true;
			}
		}
	}

	if (_glfw.randrAvailable)
	{
		XRRScreenResources* sr = _glfw.randrGetScreenResourcesCurrent(_glfw.x11Display, _glfw.x11Root);

		if (!sr->ncrtc || !_glfw.randrGetCrtcGammaSize(_glfw.x11Display, sr->crtcs[0]))
		{
			// This is likely an older Nvidia driver with broken gamma support
			// Flag it as useless and fall back to xf86vm gamma, if available
			_glfw.randrGammaBroken = true;
		}

		if (!sr->ncrtc)
		{
			// A system without CRTCs is likely a system with broken RandR
			// Disable the RandR monitor path and fall back to core functions
			_glfw.randrMonitorBroken = true;
		}

		_glfw.randrFreeScreenResources(sr);
	}

	if (_glfw.randrAvailable && !_glfw.randrMonitorBroken)
	{
		_glfw.randrSelectInput(_glfw.x11Display, _glfw.x11Root, RROutputChangeNotifyMask);
	}

	_glfw.xcursorHandle = _glfwPlatformLoadModule("libXcursor.so.1");
	if (_glfw.xcursorHandle)
	{
		_glfw.xcursorImageCreate = (FN_XcursorImageCreate)
			_glfwPlatformGetModuleSymbol(_glfw.xcursorHandle, "XcursorImageCreate");
		_glfw.xcursorImageDestroy = (FN_XcursorImageDestroy)
			_glfwPlatformGetModuleSymbol(_glfw.xcursorHandle, "XcursorImageDestroy");
		_glfw.xcursorImageLoadCursor = (FN_XcursorImageLoadCursor)
			_glfwPlatformGetModuleSymbol(_glfw.xcursorHandle, "XcursorImageLoadCursor");
		_glfw.xcursorGetTheme = (FN_XcursorGetTheme)
			_glfwPlatformGetModuleSymbol(_glfw.xcursorHandle, "XcursorGetTheme");
		_glfw.xcursorGetDefaultSize = (FN_XcursorGetDefaultSize)
			_glfwPlatformGetModuleSymbol(_glfw.xcursorHandle, "XcursorGetDefaultSize");
		_glfw.xcursorLibraryLoadImage = (FN_XcursorLibraryLoadImage)
			_glfwPlatformGetModuleSymbol(_glfw.xcursorHandle, "XcursorLibraryLoadImage");
	}

	_glfw.xineramaHandle = _glfwPlatformLoadModule("libXinerama.so.1");
	if (_glfw.xineramaHandle)
	{
		_glfw.xineramaIsActive = (FN_XineramaIsActive)
			_glfwPlatformGetModuleSymbol(_glfw.xineramaHandle, "XineramaIsActive");
		_glfw.xineramaQueryExtension = (FN_XineramaQueryExtension)
			_glfwPlatformGetModuleSymbol(_glfw.xineramaHandle, "XineramaQueryExtension");
		_glfw.xineramaQueryScreens = (FN_XineramaQueryScreens)
			_glfwPlatformGetModuleSymbol(_glfw.xineramaHandle, "XineramaQueryScreens");

			int major;
			int minor;
		if (_glfw.xineramaQueryExtension(_glfw.x11Display,  &major, &minor))
		{
			if (_glfw.xineramaIsActive(_glfw.x11Display))
				_glfw.xineramaAvailable = true;
		}
	}

	int majorOpcode;
	int errorBase;
	int major = 1;
	int minor = 0;
	_glfw.xkbAvailable =
		_glfw.xkbQueryExtension(_glfw.x11Display, &majorOpcode, &_glfw.xkbEventBase, &errorBase, &major, &minor);

	if (_glfw.xkbAvailable)
	{
		Bool supported;

		if (_glfw.xkbSetDetectableAutoRepeat(_glfw.x11Display, True, &supported))
		{
			if (supported)
				_glfw.xkbDetectable = true;
		}

		XkbStateRec state;
		if (_glfw.xkbGetState(_glfw.x11Display, XkbUseCoreKbd, &state) == Success)
			_glfw.xkbGroup = (unsigned int)state.group;

		_glfw.xkbSelectEventDetails(_glfw.x11Display, XkbUseCoreKbd, XkbStateNotify,
							  XkbGroupStateMask, XkbGroupStateMask);
	}

	_glfw.xrenderHandle = _glfwPlatformLoadModule("libXrender.so.1");
	if (_glfw.xrenderHandle)
	{
		_glfw.xrenderQueryExtension = (FN_XRenderQueryExtension)
			_glfwPlatformGetModuleSymbol(_glfw.xrenderHandle, "XRenderQueryExtension");
		_glfw.xrenderQueryVersion = (FN_XRenderQueryVersion)
			_glfwPlatformGetModuleSymbol(_glfw.xrenderHandle, "XRenderQueryVersion");
		_glfw.xrenderFindVisualFormat = (FN_XRenderFindVisualFormat)
			_glfwPlatformGetModuleSymbol(_glfw.xrenderHandle, "XRenderFindVisualFormat");

		int errorBase;
		int eventBase;
		if (_glfw.xrenderQueryExtension(_glfw.x11Display, &errorBase, &eventBase))
		{
			int major;
			int minor;
			if (_glfw.xrenderQueryVersion(_glfw.x11Display, &major, &minor))
			{
				_glfw.xrenderAvailable = true;
			}
		}
	}

	_glfw.xshapeHandle = _glfwPlatformLoadModule("libXext.so.6");
	if (_glfw.xshapeHandle)
	{
		_glfw.xshapeQueryExtension = (FN_XShapeQueryExtension)
			_glfwPlatformGetModuleSymbol(_glfw.xshapeHandle, "XShapeQueryExtension");
		_glfw.xshapeShapeCombineRegion = (FN_XShapeCombineRegion)
			_glfwPlatformGetModuleSymbol(_glfw.xshapeHandle, "XShapeCombineRegion");
		_glfw.xshapeQueryVersion = (FN_XShapeQueryVersion)
			_glfwPlatformGetModuleSymbol(_glfw.xshapeHandle, "XShapeQueryVersion");
		_glfw.xshapeShapeCombineMask = (FN_XShapeCombineMask)
			_glfwPlatformGetModuleSymbol(_glfw.xshapeHandle, "XShapeCombineMask");

		int errorBase;
		int eventBase;
		if (_glfw.xshapeQueryExtension(_glfw.x11Display, &errorBase, &eventBase))
		{
			int major;
			int minor;
			if (_glfw.xshapeQueryVersion(_glfw.x11Display, &major, &minor))
			{
				_glfw.xshapeAvailable = true;
			}
		}
	}

	// Update the key code LUT
	// FIXME: We should listen to XkbMapNotify events to track changes to
	// the keyboard mapping.
	createKeyTables();

	// String format atoms
	_glfw.x11ClipNULL_ = _glfw.xlibInternAtom(_glfw.x11Display, "NULL", False);
	_glfw.x11ClipUTF8_STRING = _glfw.xlibInternAtom(_glfw.x11Display, "UTF8_STRING", False);
	_glfw.x11ClipATOM_PAIR = _glfw.xlibInternAtom(_glfw.x11Display, "ATOM_PAIR", False);

	// Custom selection property atom
	_glfw.x11ClipSELECTION = _glfw.xlibInternAtom(_glfw.x11Display, "GLFW_SELECTION", False);

	// ICCCM standard clipboard atoms
	_glfw.x11ClipTARGETS = _glfw.xlibInternAtom(_glfw.x11Display, "TARGETS", False);
	_glfw.x11ClipMULTIPLE = _glfw.xlibInternAtom(_glfw.x11Display, "MULTIPLE", False);
	_glfw.x11ClipINCR = _glfw.xlibInternAtom(_glfw.x11Display, "INCR", False);
	_glfw.x11ClipCLIPBOARD = _glfw.xlibInternAtom(_glfw.x11Display, "CLIPBOARD", False);

	// Clipboard manager atoms
	_glfw.x11ClipCLIPBOARD_MANAGER = _glfw.xlibInternAtom(_glfw.x11Display, "CLIPBOARD_MANAGER", False);
	_glfw.x11ClipSAVE_TARGETS = _glfw.xlibInternAtom(_glfw.x11Display, "SAVE_TARGETS", False);

	// Xdnd (drag and drop) atoms
	_glfw.x11DnDAware = _glfw.xlibInternAtom(_glfw.x11Display, "XdndAware", False);
	_glfw.x11DnDEnter = _glfw.xlibInternAtom(_glfw.x11Display, "XdndEnter", False);
	_glfw.x11DnDPosition = _glfw.xlibInternAtom(_glfw.x11Display, "XdndPosition", False);
	_glfw.x11DnDStatus = _glfw.xlibInternAtom(_glfw.x11Display, "XdndStatus", False);
	_glfw.x11DnDActionCopy = _glfw.xlibInternAtom(_glfw.x11Display, "XdndActionCopy", False);
	_glfw.x11DnDDrop = _glfw.xlibInternAtom(_glfw.x11Display, "XdndDrop", False);
	_glfw.x11DnDFinished = _glfw.xlibInternAtom(_glfw.x11Display, "XdndFinished", False);
	_glfw.x11DnDSelection = _glfw.xlibInternAtom(_glfw.x11Display, "XdndSelection", False);
	_glfw.x11DnDTypeList = _glfw.xlibInternAtom(_glfw.x11Display, "XdndTypeList", False);
	_glfw.x11Text_uri_list = _glfw.xlibInternAtom(_glfw.x11Display, "text/uri-list", False);

	// ICCCM, EWMH and Motif window property atoms
	// These can be set safely even without WM support
	// The EWMH atoms that require WM support are handled in detectEWMH
	_glfw.x11WM_PROTOCOLS = _glfw.xlibInternAtom(_glfw.x11Display, "WM_PROTOCOLS", False);
	_glfw.x11WM_STATE = _glfw.xlibInternAtom(_glfw.x11Display, "WM_STATE", False);
	_glfw.x11WM_DELETE_WINDOW = _glfw.xlibInternAtom(_glfw.x11Display, "WM_DELETE_WINDOW", False);
	_glfw.x11NET_SUPPORTED = _glfw.xlibInternAtom(_glfw.x11Display, "_NET_SUPPORTED", False);
	_glfw.x11NET_SUPPORTING_WM_CHECK = _glfw.xlibInternAtom(_glfw.x11Display, "_NET_SUPPORTING_WM_CHECK", False);
	_glfw.x11NET_WM_ICON = _glfw.xlibInternAtom(_glfw.x11Display, "_NET_WM_ICON", False);
	_glfw.x11NET_WM_PING = _glfw.xlibInternAtom(_glfw.x11Display, "_NET_WM_PING", False);
	_glfw.x11NET_WM_PID = _glfw.xlibInternAtom(_glfw.x11Display, "_NET_WM_PID", False);
	_glfw.x11NET_WM_NAME = _glfw.xlibInternAtom(_glfw.x11Display, "_NET_WM_NAME", False);
	_glfw.x11NET_WM_ICON_NAME = _glfw.xlibInternAtom(_glfw.x11Display, "_NET_WM_ICON_NAME", False);
	_glfw.x11NET_WM_BYPASS_COMPOSITOR = _glfw.xlibInternAtom(_glfw.x11Display, "_NET_WM_BYPASS_COMPOSITOR", False);
	_glfw.x11NET_WM_WINDOW_OPACITY = _glfw.xlibInternAtom(_glfw.x11Display, "_NET_WM_WINDOW_OPACITY", False);
	_glfw.x11MOTIF_WM_HINTS = _glfw.xlibInternAtom(_glfw.x11Display, "_MOTIF_WM_HINTS", False);

	// The compositing manager selection name contains the screen number
	{
		char name[32];
		snprintf(name, sizeof(name), "_NET_WM_CM_S%u", _glfw.x11Screen);
		_glfw.x11NET_WM_CM_Sx = _glfw.xlibInternAtom(_glfw.x11Display, name, False);
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
	char* rms = _glfw.xlibResourceManagerString(_glfw.x11Display);
	if (rms)
	{
		XrmDatabase db = _glfw.xrmGetStringDatabase(rms);
		if (db)
		{
			XrmValue value;
			char* type = NULL;

			if (_glfw.xrmGetResource(db, "Xft.dpi", "Xft.Dpi", &type, &value))
			{
				if (type && strcmp(type, "String") == 0)
					xdpi = ydpi = atof(value.addr);
			}

			_glfw.xrmDestroyDatabase(db);
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

	return _glfw.xlibCreateWindow(_glfw.x11Display, _glfw.x11Root,
						 0, 0, 1, 1, 0, 0,
						 InputOnly,
						 DefaultVisual(_glfw.x11Display, _glfw.x11Screen),
						 CWEventMask, &wa);
}

// Create the pipe for empty events without assumuing the OS has pipe2(2)
//
static ErrorResponse* createEmptyEventPipe(void)
{
	if (pipe(_glfw.x11EmptyEventPipe) != 0)
	{
		return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to create empty event pipe: %s", strerror(errno));
	}

	for (int i = 0; i < 2; i++)
	{
		const int sf = fcntl(_glfw.x11EmptyEventPipe[i], F_GETFL, 0);
		const int df = fcntl(_glfw.x11EmptyEventPipe[i], F_GETFD, 0);

		if (sf == -1 || df == -1 ||
			fcntl(_glfw.x11EmptyEventPipe[i], F_SETFL, sf | O_NONBLOCK) == -1 ||
			fcntl(_glfw.x11EmptyEventPipe[i], F_SETFD, df | FD_CLOEXEC) == -1)
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
	if (_glfw.x11Display != display)
		return 0;

	_glfw.x11ErrorCode = event->error_code;
	return 0;
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Sets the X error handler callback
//
void _glfwGrabErrorHandlerX11(void)
{
	_glfw.x11ErrorCode = Success;
	_glfw.x11ErrorHandler = _glfw.xlibSetErrorHandler(errorHandler);
}

// Clears the X error handler callback
//
void _glfwReleaseErrorHandlerX11(void)
{
	// Synchronize to make sure all commands are processed
	_glfw.xlibSync(_glfw.x11Display, False);
	_glfw.xlibSetErrorHandler(_glfw.x11ErrorHandler);
	_glfw.x11ErrorHandler = NULL;
}

// Reports the specified error, appending information about the last X error
//
void _glfwInputErrorX11(int error, const char* message)
{
	char buffer[ERROR_MSG_SIZE];
	_glfw.xlibGetErrorText(_glfw.x11Display, _glfw.x11ErrorCode,
				  buffer, sizeof(buffer));

	_glfwInputError(error, "%s: %s", message, buffer);
}

// Creates a native cursor object from the specified image and hotspot
//
Cursor _glfwCreateNativeCursorX11(const ImageData* image, int xhot, int yhot)
{
	Cursor cursor;

	if (!_glfw.xcursorHandle)
		return None;

	XcursorImage* native = _glfw.xcursorImageCreate(image->width, image->height);
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

	cursor = _glfw.xcursorImageLoadCursor(_glfw.x11Display, native);
	_glfw.xcursorImageDestroy(native);

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

	FN_XInitThreads XInitThreads = (FN_XInitThreads)_glfwPlatformGetModuleSymbol(module, "XInitThreads");
	FN_XrmInitialize XrmInitialize = (FN_XrmInitialize)_glfwPlatformGetModuleSymbol(module, "XrmInitialize");
	FN_XOpenDisplay XOpenDisplay = (FN_XOpenDisplay)_glfwPlatformGetModuleSymbol(module, "XOpenDisplay");
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

	_glfw.x11Display = display;
	_glfw.xlibHandle = module;

	platform->setCursorMode = _glfwSetCursorModeX11;
	platform->createCursor = _glfwCreateCursorX11;
	platform->createStandardCursor = _glfwCreateStandardCursorX11;
	platform->destroyCursor = _glfwDestroyCursorX11;
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

	_glfw.xlibAllocSizeHints = (FN_XAllocSizeHints)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XAllocSizeHints");
	_glfw.xlibAllocWMHints = (FN_XAllocWMHints)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XAllocWMHints");
	_glfw.xlibChangeProperty = (FN_XChangeProperty)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XChangeProperty");
	_glfw.xlibChangeWindowAttributes = (FN_XChangeWindowAttributes)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XChangeWindowAttributes");
	_glfw.xlibCheckIfEvent = (FN_XCheckIfEvent)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XCheckIfEvent");
	_glfw.xlibCheckTypedWindowEvent = (FN_XCheckTypedWindowEvent)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XCheckTypedWindowEvent");
	_glfw.xlibCloseDisplay = (FN_XCloseDisplay)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XCloseDisplay");
	_glfw.xlibCloseIM = (FN_XCloseIM)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XCloseIM");
	_glfw.xlibConvertSelection = (FN_XConvertSelection)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XConvertSelection");
	_glfw.xlibCreateColormap = (FN_XCreateColormap)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XCreateColormap");
	_glfw.xlibCreateFontCursor = (FN_XCreateFontCursor)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XCreateFontCursor");
	_glfw.xlibCreateIC = (FN_XCreateIC)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XCreateIC");
	_glfw.xlibCreateRegion = (FN_XCreateRegion)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XCreateRegion");
	_glfw.xlibCreateWindow = (FN_XCreateWindow)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XCreateWindow");
	_glfw.xlibDefineCursor = (FN_XDefineCursor)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XDefineCursor");
	_glfw.xlibDeleteContext = (FN_XDeleteContext)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XDeleteContext");
	_glfw.xlibDeleteProperty = (FN_XDeleteProperty)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XDeleteProperty");
	_glfw.xlibDestroyIC = (FN_XDestroyIC)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XDestroyIC");
	_glfw.xlibDestroyRegion = (FN_XDestroyRegion)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XDestroyRegion");
	_glfw.xlibDestroyWindow = (FN_XDestroyWindow)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XDestroyWindow");
	_glfw.xlibDisplayKeycodes = (FN_XDisplayKeycodes)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XDisplayKeycodes");
	_glfw.xlibEventsQueued = (FN_XEventsQueued)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XEventsQueued");
	_glfw.xlibFilterEvent = (FN_XFilterEvent)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XFilterEvent");
	_glfw.xlibFindContext = (FN_XFindContext)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XFindContext");
	_glfw.xlibFlush = (FN_XFlush)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XFlush");
	_glfw.xlibFree = (FN_XFree)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XFree");
	_glfw.xlibFreeColormap = (FN_XFreeColormap)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XFreeColormap");
	_glfw.xlibFreeCursor = (FN_XFreeCursor)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XFreeCursor");
	_glfw.xlibFreeEventData = (FN_XFreeEventData)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XFreeEventData");
	_glfw.xlibGetErrorText = (FN_XGetErrorText)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XGetErrorText");
	_glfw.xlibGetICValues = (FN_XGetICValues)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XGetICValues");
	_glfw.xlibGetIMValues = (FN_XGetIMValues)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XGetIMValues");
	_glfw.xlibGetInputFocus = (FN_XGetInputFocus)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XGetInputFocus");
	_glfw.xlibGetKeyboardMapping = (FN_XGetKeyboardMapping)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XGetKeyboardMapping");
	_glfw.xlibGetScreenSaver = (FN_XGetScreenSaver)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XGetScreenSaver");
	_glfw.xlibGetSelectionOwner = (FN_XGetSelectionOwner)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XGetSelectionOwner");
	_glfw.xlibGetWMNormalHints = (FN_XGetWMNormalHints)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XGetWMNormalHints");
	_glfw.xlibGetWindowAttributes = (FN_XGetWindowAttributes)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XGetWindowAttributes");
	_glfw.xlibGetWindowProperty = (FN_XGetWindowProperty)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XGetWindowProperty");
	_glfw.xlibIconifyWindow = (FN_XIconifyWindow)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XIconifyWindow");
	_glfw.xlibInternAtom = (FN_XInternAtom)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XInternAtom");
	_glfw.xlibLookupString = (FN_XLookupString)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XLookupString");
	_glfw.xlibMapRaised = (FN_XMapRaised)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XMapRaised");
	_glfw.xlibMapWindow = (FN_XMapWindow)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XMapWindow");
	_glfw.xlibMoveResizeWindow = (FN_XMoveResizeWindow)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XMoveResizeWindow");
	_glfw.xlibMoveWindow = (FN_XMoveWindow)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XMoveWindow");
	_glfw.xlibNextEvent = (FN_XNextEvent)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XNextEvent");
	_glfw.xlibOpenIM = (FN_XOpenIM)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XOpenIM");
	_glfw.xlibPeekEvent = (FN_XPeekEvent)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XPeekEvent");
	_glfw.xlibPending = (FN_XPending)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XPending");
	_glfw.xlibQueryExtension = (FN_XQueryExtension)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XQueryExtension");
	_glfw.xlibQueryPointer = (FN_XQueryPointer)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XQueryPointer");
	_glfw.xlibRaiseWindow = (FN_XRaiseWindow)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XRaiseWindow");
	_glfw.xlibRegisterIMInstantiateCallback = (FN_XRegisterIMInstantiateCallback)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XRegisterIMInstantiateCallback");
	_glfw.xlibResizeWindow = (FN_XResizeWindow)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XResizeWindow");
	_glfw.xlibResourceManagerString = (FN_XResourceManagerString)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XResourceManagerString");
	_glfw.xlibSaveContext = (FN_XSaveContext)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSaveContext");
	_glfw.xlibSelectInput = (FN_XSelectInput)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSelectInput");
	_glfw.xlibSendEvent = (FN_XSendEvent)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSendEvent");
	_glfw.xlibSetErrorHandler = (FN_XSetErrorHandler)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSetErrorHandler");
	_glfw.xlibSetICFocus = (FN_XSetICFocus)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSetICFocus");
	_glfw.xlibSetIMValues = (FN_XSetIMValues)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSetIMValues");
	_glfw.xlibSetInputFocus = (FN_XSetInputFocus)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSetInputFocus");
	_glfw.xlibSetLocaleModifiers = (FN_XSetLocaleModifiers)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSetLocaleModifiers");
	_glfw.xlibSetScreenSaver = (FN_XSetScreenSaver)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSetScreenSaver");
	_glfw.xlibSetSelectionOwner = (FN_XSetSelectionOwner)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSetSelectionOwner");
	_glfw.xlibSetWMHints = (FN_XSetWMHints)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSetWMHints");
	_glfw.xlibSetWMNormalHints = (FN_XSetWMNormalHints)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSetWMNormalHints");
	_glfw.xlibSetWMProtocols = (FN_XSetWMProtocols)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSetWMProtocols");
	_glfw.xlibSupportsLocale = (FN_XSupportsLocale)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSupportsLocale");
	_glfw.xlibSync = (FN_XSync)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XSync");
	_glfw.xlibTranslateCoordinates = (FN_XTranslateCoordinates)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XTranslateCoordinates");
	_glfw.xlibUndefineCursor = (FN_XUndefineCursor)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XUndefineCursor");
	_glfw.xlibUnmapWindow = (FN_XUnmapWindow)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XUnmapWindow");
	_glfw.xlibUnsetICFocus = (FN_XUnsetICFocus)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XUnsetICFocus");
	_glfw.xlibWarpPointer = (FN_XWarpPointer)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XWarpPointer");
	_glfw.xkbFreeKeyboard = (FN_XkbFreeKeyboard)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XkbFreeKeyboard");
	_glfw.xkbFreeNames = (FN_XkbFreeNames)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XkbFreeNames");
	_glfw.xkbGetMap = (FN_XkbGetMap)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XkbGetMap");
	_glfw.xkbGetNames = (FN_XkbGetNames)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XkbGetNames");
	_glfw.xkbGetState = (FN_XkbGetState)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XkbGetState");
	_glfw.xkbQueryExtension = (FN_XkbQueryExtension)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XkbQueryExtension");
	_glfw.xkbSelectEventDetails = (FN_XkbSelectEventDetails)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XkbSelectEventDetails");
	_glfw.xkbSetDetectableAutoRepeat = (FN_XkbSetDetectableAutoRepeat)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XkbSetDetectableAutoRepeat");
	_glfw.xrmDestroyDatabase = (FN_XrmDestroyDatabase)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XrmDestroyDatabase");
	_glfw.xrmGetResource = (FN_XrmGetResource)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XrmGetResource");
	_glfw.xrmGetStringDatabase = (FN_XrmGetStringDatabase)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XrmGetStringDatabase");
	_glfw.xlibUnregisterIMInstantiateCallback = (FN_XUnregisterIMInstantiateCallback)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "XUnregisterIMInstantiateCallback");
	_glfw.xlibUTF8LookupString = (FN_Xutf8LookupString)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "Xutf8LookupString");
	_glfw.xlibUTF8SetWMProperties = (FN_Xutf8SetWMProperties)
		_glfwPlatformGetModuleSymbol(_glfw.xlibHandle, "Xutf8SetWMProperties");

	if (_glfw.xlibUTF8LookupString && _glfw.xlibUTF8SetWMProperties)
		_glfw.xlibUTF8 = true;

	_glfw.x11Screen = DefaultScreen(_glfw.x11Display);
	_glfw.x11Root = RootWindow(_glfw.x11Display, _glfw.x11Screen);
	_glfw.x11Context = XUniqueContext();

	getSystemContentScale(&_glfw.x11ContentScaleX, &_glfw.x11ContentScaleY);

	ErrorResponse* errRsp = createEmptyEventPipe();
	if (errRsp) {
		_terminate();
		return errRsp;
	}

	initExtensions();

	_glfw.x11HelperWindowHandle = createHelperWindow();
	_glfw.x11HiddenCursorHandle = createHiddenCursor();

	if (_glfw.xlibSupportsLocale() && _glfw.xlibUTF8)
	{
		_glfw.xlibSetLocaleModifiers("");

		// If an IM is already present our callback will be called right away
		_glfw.xlibRegisterIMInstantiateCallback(_glfw.x11Display,
									   NULL, NULL, NULL,
									   inputMethodInstantiateCallback,
									   NULL);
	}

	_glfwPollMonitorsX11();
	return NULL;
}

void platformTerminate(void)
{
	if (_glfw.x11HelperWindowHandle)
	{
		if (_glfw.xlibGetSelectionOwner(_glfw.x11Display, _glfw.x11ClipCLIPBOARD) ==
			_glfw.x11HelperWindowHandle)
		{
			_glfwPushSelectionToManagerX11();
		}

		_glfw.xlibDestroyWindow(_glfw.x11Display, _glfw.x11HelperWindowHandle);
		_glfw.x11HelperWindowHandle = None;
	}

	if (_glfw.x11HiddenCursorHandle)
	{
		_glfw.xlibFreeCursor(_glfw.x11Display, _glfw.x11HiddenCursorHandle);
		_glfw.x11HiddenCursorHandle = (Cursor) 0;
	}

	_glfw.xlibUnregisterIMInstantiateCallback(_glfw.x11Display,
									 NULL, NULL, NULL,
									 inputMethodInstantiateCallback,
									 NULL);

	if (_glfw.x11IM)
	{
		_glfw.xlibCloseIM(_glfw.x11IM);
		_glfw.x11IM = NULL;
	}

	if (_glfw.x11Display)
	{
		_glfw.xlibCloseDisplay(_glfw.x11Display);
		_glfw.x11Display = NULL;
	}

	if (_glfw.xcursorHandle)
	{
		_glfwPlatformFreeModule(_glfw.xcursorHandle);
		_glfw.xcursorHandle = NULL;
	}

	if (_glfw.randrHandle)
	{
		_glfwPlatformFreeModule(_glfw.randrHandle);
		_glfw.randrHandle = NULL;
	}

	if (_glfw.xineramaHandle)
	{
		_glfwPlatformFreeModule(_glfw.xineramaHandle);
		_glfw.xineramaHandle = NULL;
	}

	if (_glfw.xrenderHandle)
	{
		_glfwPlatformFreeModule(_glfw.xrenderHandle);
		_glfw.xrenderHandle = NULL;
	}

	if (_glfw.xvidmodeHandle)
	{
		_glfwPlatformFreeModule(_glfw.xvidmodeHandle);
		_glfw.xvidmodeHandle = NULL;
	}

	if (_glfw.xiHandle)
	{
		_glfwPlatformFreeModule(_glfw.xiHandle);
		_glfw.xiHandle = NULL;
	}

	// NOTE: These need to be unloaded after XCloseDisplay, as they register
	//       cleanup callbacks that get called by that function
	_glfwTerminateGLX();

	if (_glfw.xlibHandle)
	{
		_glfwPlatformFreeModule(_glfw.xlibHandle);
		_glfw.xlibHandle = NULL;
	}

	if (_glfw.x11EmptyEventPipe[0] || _glfw.x11EmptyEventPipe[1])
	{
		close(_glfw.x11EmptyEventPipe[0]);
		close(_glfw.x11EmptyEventPipe[1]);
	}
}

#endif // __linux__
