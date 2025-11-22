#if defined(__linux__)

#include "platform.h"
#include <limits.h>
#include <stdio.h>
#include <locale.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>


// Translate the X11 KeySyms for a key to a PLAF key code
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

	memset(_plaf.keyCodes, -1, sizeof(_plaf.keyCodes));
	memset(_plaf.scanCodes, -1, sizeof(_plaf.scanCodes));

	if (_plaf.xkbAvailable)
	{
		// Use XKB to determine physical key locations independently of the
		// current keyboard layout

		XkbDescPtr desc = _plaf.xkbGetMap(_plaf.x11Display, 0, XkbUseCoreKbd);
		_plaf.xkbGetNames(_plaf.x11Display, XkbKeyNamesMask | XkbKeyAliasesMask, desc);

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

		// Find the X11 key code -> PLAF key code mapping
		for (int scancode = scancodeMin;  scancode <= scancodeMax;  scancode++)
		{
			int key = KEY_UNKNOWN;

			// Map the key name to a PLAF key code. Note: We use the US
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

			_plaf.keyCodes[scancode] = key;
		}

		_plaf.xkbFreeNames(desc, XkbKeyNamesMask, True);
		_plaf.xkbFreeKeyboard(desc, 0, True);
	}
	else
		_plaf.xlibDisplayKeycodes(_plaf.x11Display, &scancodeMin, &scancodeMax);

	int width;
	KeySym* keysyms = _plaf.xlibGetKeyboardMapping(_plaf.x11Display,
										  scancodeMin,
										  scancodeMax - scancodeMin + 1,
										  &width);

	for (int scancode = scancodeMin;  scancode <= scancodeMax;  scancode++)
	{
		// Translate the un-translated key codes using traditional X11 KeySym
		// lookups
		if (_plaf.keyCodes[scancode] < 0)
		{
			const size_t base = (scancode - scancodeMin) * width;
			_plaf.keyCodes[scancode] = translateKeySyms(&keysyms[base], width);
		}

		// Store the reverse translation for faster key name lookup
		if (_plaf.keyCodes[scancode] > 0)
			_plaf.scanCodes[_plaf.keyCodes[scancode]] = scancode;
	}

	_plaf.xlibFree(keysyms);
}

// Check whether the IM has a usable style
//
static IntBool hasUsableInputMethodStyle(void)
{
	IntBool found = false;
	XIMStyles* styles = NULL;

	if (_plaf.xlibGetIMValues(_plaf.x11IM, XNQueryInputStyle, &styles, NULL) != NULL)
		return false;

	for (unsigned int i = 0;  i < styles->count_styles;  i++)
	{
		if (styles->supported_styles[i] == (XIMPreeditNothing | XIMStatusNothing))
		{
			found = true;
			break;
		}
	}

	_plaf.xlibFree(styles);
	return found;
}

static void inputMethodDestroyCallback(XIM im, XPointer clientData, XPointer callData)
{
	_plaf.x11IM = NULL;
}

static void inputMethodInstantiateCallback(Display* display,
										   XPointer clientData,
										   XPointer callData)
{
	if (_plaf.x11IM)
		return;

	_plaf.x11IM = _plaf.xlibOpenIM(_plaf.x11Display, 0, NULL, NULL);
	if (_plaf.x11IM)
	{
		if (!hasUsableInputMethodStyle())
		{
			_plaf.xlibCloseIM(_plaf.x11IM);
			_plaf.x11IM = NULL;
		}
	}

	if (_plaf.x11IM)
	{
		XIMCallback callback;
		callback.callback = (XIMProc) inputMethodDestroyCallback;
		callback.client_data = NULL;
		_plaf.xlibSetIMValues(_plaf.x11IM, XNDestroyCallback, &callback, NULL);

		for (plafWindow* window = _plaf.windowListHead;  window;  window = window->next)
			_plafCreateInputContextX11(window);
	}
}

// Return the atom ID only if it is listed in the specified array
//
static Atom getAtomIfSupported(Atom* supportedAtoms,
							   unsigned long atomCount,
							   const char* atomName)
{
	const Atom atom = _plaf.xlibInternAtom(_plaf.x11Display, atomName, False);

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
	if (!_plafGetWindowPropertyX11(_plaf.x11Root,
								   _plaf.x11NET_SUPPORTING_WM_CHECK,
								   XA_WINDOW,
								   (unsigned char**) &windowFromRoot))
	{
		return;
	}

	_plafGrabErrorHandlerX11();

	// If it exists, it should be the XID of a top-level window
	// Then we look for the same property on that window

	Window* windowFromChild = NULL;
	if (!_plafGetWindowPropertyX11(*windowFromRoot,
								   _plaf.x11NET_SUPPORTING_WM_CHECK,
								   XA_WINDOW,
								   (unsigned char**) &windowFromChild))
	{
		_plafReleaseErrorHandlerX11();
		_plaf.xlibFree(windowFromRoot);
		return;
	}

	_plafReleaseErrorHandlerX11();

	// If the property exists, it should contain the XID of the window

	if (*windowFromRoot != *windowFromChild)
	{
		_plaf.xlibFree(windowFromRoot);
		_plaf.xlibFree(windowFromChild);
		return;
	}

	_plaf.xlibFree(windowFromRoot);
	_plaf.xlibFree(windowFromChild);

	// We are now fairly sure that an EWMH-compliant WM is currently running
	// We can now start querying the WM about what features it supports by
	// looking in the _NET_SUPPORTED property on the root window
	// It should contain a list of supported EWMH protocol and state atoms

	Atom* supportedAtoms = NULL;
	const unsigned long atomCount =
		_plafGetWindowPropertyX11(_plaf.x11Root,
								  _plaf.x11NET_SUPPORTED,
								  XA_ATOM,
								  (unsigned char**) &supportedAtoms);

	// See which of the atoms we support that are supported by the WM

	_plaf.x11NET_WM_STATE =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE");
	_plaf.x11NET_WM_STATE_ABOVE =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_ABOVE");
	_plaf.x11NET_WM_STATE_FULLSCREEN =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_FULLSCREEN");
	_plaf.x11NET_WM_STATE_MAXIMIZED_VERT =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_MAXIMIZED_VERT");
	_plaf.x11NET_WM_STATE_MAXIMIZED_HORZ =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_MAXIMIZED_HORZ");
	_plaf.x11NET_WM_STATE_DEMANDS_ATTENTION =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_STATE_DEMANDS_ATTENTION");
	_plaf.x11NET_WM_FULLSCREEN_MONITORS =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_FULLSCREEN_MONITORS");
	_plaf.x11NET_WM_WINDOW_TYPE =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_WINDOW_TYPE");
	_plaf.x11NET_WM_WINDOW_TYPE_NORMAL =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WM_WINDOW_TYPE_NORMAL");
	_plaf.x11NET_WORKAREA =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_WORKAREA");
	_plaf.x11NET_CURRENT_DESKTOP =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_CURRENT_DESKTOP");
	_plaf.x11NET_ACTIVE_WINDOW =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_ACTIVE_WINDOW");
	_plaf.x11NET_FRAME_EXTENTS =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_FRAME_EXTENTS");
	_plaf.x11NET_REQUEST_FRAME_EXTENTS =
		getAtomIfSupported(supportedAtoms, atomCount, "_NET_REQUEST_FRAME_EXTENTS");

	if (supportedAtoms)
		_plaf.xlibFree(supportedAtoms);
}

// Look for and initialize supported X11 extensions
//
static void initExtensions(void)
{
	_plaf.xvidmodeHandle = _plafLoadModule("libXxf86vm.so.1");
	if (_plaf.xvidmodeHandle)
	{
		_plaf.xvidmodeQueryExtension = (FN_XF86VidModeQueryExtension)
			_plafGetModuleSymbol(_plaf.xvidmodeHandle, "XF86VidModeQueryExtension");
		_plaf.xvidmodeGetGammaRamp = (FN_XF86VidModeGetGammaRamp)
			_plafGetModuleSymbol(_plaf.xvidmodeHandle, "XF86VidModeGetGammaRamp");
		_plaf.xvidmodeSetGammaRamp = (FN_XF86VidModeSetGammaRamp)
			_plafGetModuleSymbol(_plaf.xvidmodeHandle, "XF86VidModeSetGammaRamp");
		_plaf.xvidmodeGetGammaRampSize = (FN_XF86VidModeGetGammaRampSize)
			_plafGetModuleSymbol(_plaf.xvidmodeHandle, "XF86VidModeGetGammaRampSize");

		int eventBase;
		int errorBase;
		_plaf.xvidmodeAvailable = _plaf.xvidmodeQueryExtension(_plaf.x11Display, &eventBase, &errorBase);
	}

	_plaf.xiHandle = _plafLoadModule("libXi.so.6");
	if (_plaf.xiHandle)
	{
		_plaf.xiQueryVersion = (FN_XIQueryVersion)
			_plafGetModuleSymbol(_plaf.xiHandle, "XIQueryVersion");

		int majorOpcode;
		int eventBase;
		int errorBase;
		if (_plaf.xlibQueryExtension(_plaf.x11Display, "XInputExtension", &majorOpcode, &eventBase, &errorBase)) {
			int major = 2;
			int minor = 0;
			if (_plaf.xiQueryVersion(_plaf.x11Display, &major, &minor) == Success) {
				_plaf.xiAvailable = true;
			}
		}
	}
	_plaf.randrHandle = _plafLoadModule("libXrandr.so.2");
	if (_plaf.randrHandle)
	{
		_plaf.randrAllocGamma = (FN_XRRAllocGamma)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRAllocGamma");
		_plaf.randrFreeGamma = (FN_XRRFreeGamma)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRFreeGamma");
		_plaf.randrFreeCrtcInfo = (FN_XRRFreeCrtcInfo)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRFreeCrtcInfo");
		_plaf.randrFreeOutputInfo = (FN_XRRFreeOutputInfo)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRFreeOutputInfo");
		_plaf.randrFreeScreenResources = (FN_XRRFreeScreenResources)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRFreeScreenResources");
		_plaf.randrGetCrtcGamma = (FN_XRRGetCrtcGamma)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRGetCrtcGamma");
		_plaf.randrGetCrtcGammaSize = (FN_XRRGetCrtcGammaSize)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRGetCrtcGammaSize");
		_plaf.randrGetCrtcInfo = (FN_XRRGetCrtcInfo)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRGetCrtcInfo");
		_plaf.randrGetOutputInfo = (FN_XRRGetOutputInfo)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRGetOutputInfo");
		_plaf.randrGetOutputPrimary = (FN_XRRGetOutputPrimary)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRGetOutputPrimary");
		_plaf.randrGetScreenResourcesCurrent = (FN_XRRGetScreenResourcesCurrent)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRGetScreenResourcesCurrent");
		_plaf.randrQueryExtension = (FN_XRRQueryExtension)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRQueryExtension");
		_plaf.randrQueryVersion = (FN_XRRQueryVersion)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRQueryVersion");
		_plaf.randrSelectInput = (FN_XRRSelectInput)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRSelectInput");
		_plaf.randrSetCrtcConfig = (FN_XRRSetCrtcConfig)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRSetCrtcConfig");
		_plaf.randrSetCrtcGamma = (FN_XRRSetCrtcGamma)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRSetCrtcGamma");
		_plaf.randrUpdateConfiguration = (FN_XRRUpdateConfiguration)
			_plafGetModuleSymbol(_plaf.randrHandle, "XRRUpdateConfiguration");

		int errorBase;
		if (_plaf.randrQueryExtension(_plaf.x11Display,
							  &_plaf.randrEventBase,
							  &errorBase))
		{
			int major;
			int minor;
			if (_plaf.randrQueryVersion(_plaf.x11Display, &major, &minor))
			{
				// The PLAF RandR path requires at least version 1.3
				if (major > 1 || minor >= 3)
					_plaf.randrAvailable = true;
			}
		}
	}

	if (_plaf.randrAvailable)
	{
		XRRScreenResources* sr = _plaf.randrGetScreenResourcesCurrent(_plaf.x11Display, _plaf.x11Root);

		if (!sr->ncrtc || !_plaf.randrGetCrtcGammaSize(_plaf.x11Display, sr->crtcs[0]))
		{
			// This is likely an older Nvidia driver with broken gamma support
			// Flag it as useless and fall back to xf86vm gamma, if available
			_plaf.randrGammaBroken = true;
		}

		if (!sr->ncrtc)
		{
			// A system without CRTCs is likely a system with broken RandR
			// Disable the RandR monitor path and fall back to core functions
			_plaf.randrMonitorBroken = true;
		}

		_plaf.randrFreeScreenResources(sr);
	}

	if (_plaf.randrAvailable && !_plaf.randrMonitorBroken)
	{
		_plaf.randrSelectInput(_plaf.x11Display, _plaf.x11Root, RROutputChangeNotifyMask);
	}

	_plaf.xcursorHandle = _plafLoadModule("libXcursor.so.1");
	if (_plaf.xcursorHandle)
	{
		_plaf.xcursorImageCreate = (FN_XcursorImageCreate)
			_plafGetModuleSymbol(_plaf.xcursorHandle, "XcursorImageCreate");
		_plaf.xcursorImageDestroy = (FN_XcursorImageDestroy)
			_plafGetModuleSymbol(_plaf.xcursorHandle, "XcursorImageDestroy");
		_plaf.xcursorImageLoadCursor = (FN_XcursorImageLoadCursor)
			_plafGetModuleSymbol(_plaf.xcursorHandle, "XcursorImageLoadCursor");
		_plaf.xcursorGetTheme = (FN_XcursorGetTheme)
			_plafGetModuleSymbol(_plaf.xcursorHandle, "XcursorGetTheme");
		_plaf.xcursorGetDefaultSize = (FN_XcursorGetDefaultSize)
			_plafGetModuleSymbol(_plaf.xcursorHandle, "XcursorGetDefaultSize");
		_plaf.xcursorLibraryLoadImage = (FN_XcursorLibraryLoadImage)
			_plafGetModuleSymbol(_plaf.xcursorHandle, "XcursorLibraryLoadImage");
	}

	_plaf.xineramaHandle = _plafLoadModule("libXinerama.so.1");
	if (_plaf.xineramaHandle)
	{
		_plaf.xineramaIsActive = (FN_XineramaIsActive)
			_plafGetModuleSymbol(_plaf.xineramaHandle, "XineramaIsActive");
		_plaf.xineramaQueryExtension = (FN_XineramaQueryExtension)
			_plafGetModuleSymbol(_plaf.xineramaHandle, "XineramaQueryExtension");
		_plaf.xineramaQueryScreens = (FN_XineramaQueryScreens)
			_plafGetModuleSymbol(_plaf.xineramaHandle, "XineramaQueryScreens");

			int major;
			int minor;
		if (_plaf.xineramaQueryExtension(_plaf.x11Display,  &major, &minor))
		{
			if (_plaf.xineramaIsActive(_plaf.x11Display))
				_plaf.xineramaAvailable = true;
		}
	}

	int majorOpcode;
	int errorBase;
	int major = 1;
	int minor = 0;
	_plaf.xkbAvailable =
		_plaf.xkbQueryExtension(_plaf.x11Display, &majorOpcode, &_plaf.xkbEventBase, &errorBase, &major, &minor);

	if (_plaf.xkbAvailable)
	{
		Bool supported;

		if (_plaf.xkbSetDetectableAutoRepeat(_plaf.x11Display, True, &supported))
		{
			if (supported)
				_plaf.xkbDetectable = true;
		}

		XkbStateRec state;
		if (_plaf.xkbGetState(_plaf.x11Display, XkbUseCoreKbd, &state) == Success)
			_plaf.xkbGroup = (unsigned int)state.group;

		_plaf.xkbSelectEventDetails(_plaf.x11Display, XkbUseCoreKbd, XkbStateNotify,
							  XkbGroupStateMask, XkbGroupStateMask);
	}

	_plaf.xrenderHandle = _plafLoadModule("libXrender.so.1");
	if (_plaf.xrenderHandle)
	{
		_plaf.xrenderQueryExtension = (FN_XRenderQueryExtension)
			_plafGetModuleSymbol(_plaf.xrenderHandle, "XRenderQueryExtension");
		_plaf.xrenderQueryVersion = (FN_XRenderQueryVersion)
			_plafGetModuleSymbol(_plaf.xrenderHandle, "XRenderQueryVersion");
		_plaf.xrenderFindVisualFormat = (FN_XRenderFindVisualFormat)
			_plafGetModuleSymbol(_plaf.xrenderHandle, "XRenderFindVisualFormat");

		int errorBase;
		int eventBase;
		if (_plaf.xrenderQueryExtension(_plaf.x11Display, &errorBase, &eventBase))
		{
			int major;
			int minor;
			if (_plaf.xrenderQueryVersion(_plaf.x11Display, &major, &minor))
			{
				_plaf.xrenderAvailable = true;
			}
		}
	}

	_plaf.xshapeHandle = _plafLoadModule("libXext.so.6");
	if (_plaf.xshapeHandle)
	{
		_plaf.xshapeQueryExtension = (FN_XShapeQueryExtension)
			_plafGetModuleSymbol(_plaf.xshapeHandle, "XShapeQueryExtension");
		_plaf.xshapeShapeCombineRegion = (FN_XShapeCombineRegion)
			_plafGetModuleSymbol(_plaf.xshapeHandle, "XShapeCombineRegion");
		_plaf.xshapeQueryVersion = (FN_XShapeQueryVersion)
			_plafGetModuleSymbol(_plaf.xshapeHandle, "XShapeQueryVersion");
		_plaf.xshapeShapeCombineMask = (FN_XShapeCombineMask)
			_plafGetModuleSymbol(_plaf.xshapeHandle, "XShapeCombineMask");

		int errorBase;
		int eventBase;
		if (_plaf.xshapeQueryExtension(_plaf.x11Display, &errorBase, &eventBase))
		{
			int major;
			int minor;
			if (_plaf.xshapeQueryVersion(_plaf.x11Display, &major, &minor))
			{
				_plaf.xshapeAvailable = true;
			}
		}
	}

	// Update the key code LUT
	// FIXME: We should listen to XkbMapNotify events to track changes to
	// the keyboard mapping.
	createKeyTables();

	// String format atoms
	_plaf.x11ClipNULL_ = _plaf.xlibInternAtom(_plaf.x11Display, "NULL", False);
	_plaf.x11ClipUTF8_STRING = _plaf.xlibInternAtom(_plaf.x11Display, "UTF8_STRING", False);
	_plaf.x11ClipATOM_PAIR = _plaf.xlibInternAtom(_plaf.x11Display, "ATOM_PAIR", False);

	// Custom selection property atom
	_plaf.x11ClipSELECTION = _plaf.xlibInternAtom(_plaf.x11Display, "PLAF_SELECTION", False);

	// ICCCM standard clipboard atoms
	_plaf.x11ClipTARGETS = _plaf.xlibInternAtom(_plaf.x11Display, "TARGETS", False);
	_plaf.x11ClipMULTIPLE = _plaf.xlibInternAtom(_plaf.x11Display, "MULTIPLE", False);
	_plaf.x11ClipINCR = _plaf.xlibInternAtom(_plaf.x11Display, "INCR", False);
	_plaf.x11ClipCLIPBOARD = _plaf.xlibInternAtom(_plaf.x11Display, "CLIPBOARD", False);

	// Clipboard manager atoms
	_plaf.x11ClipCLIPBOARD_MANAGER = _plaf.xlibInternAtom(_plaf.x11Display, "CLIPBOARD_MANAGER", False);
	_plaf.x11ClipSAVE_TARGETS = _plaf.xlibInternAtom(_plaf.x11Display, "SAVE_TARGETS", False);

	// Xdnd (drag and drop) atoms
	_plaf.x11DnDAware = _plaf.xlibInternAtom(_plaf.x11Display, "XdndAware", False);
	_plaf.x11DnDEnter = _plaf.xlibInternAtom(_plaf.x11Display, "XdndEnter", False);
	_plaf.x11DnDPosition = _plaf.xlibInternAtom(_plaf.x11Display, "XdndPosition", False);
	_plaf.x11DnDStatus = _plaf.xlibInternAtom(_plaf.x11Display, "XdndStatus", False);
	_plaf.x11DnDActionCopy = _plaf.xlibInternAtom(_plaf.x11Display, "XdndActionCopy", False);
	_plaf.x11DnDDrop = _plaf.xlibInternAtom(_plaf.x11Display, "XdndDrop", False);
	_plaf.x11DnDFinished = _plaf.xlibInternAtom(_plaf.x11Display, "XdndFinished", False);
	_plaf.x11DnDSelection = _plaf.xlibInternAtom(_plaf.x11Display, "XdndSelection", False);
	_plaf.x11DnDTypeList = _plaf.xlibInternAtom(_plaf.x11Display, "XdndTypeList", False);
	_plaf.x11Text_uri_list = _plaf.xlibInternAtom(_plaf.x11Display, "text/uri-list", False);

	// ICCCM, EWMH and Motif window property atoms
	// These can be set safely even without WM support
	// The EWMH atoms that require WM support are handled in detectEWMH
	_plaf.x11WM_PROTOCOLS = _plaf.xlibInternAtom(_plaf.x11Display, "WM_PROTOCOLS", False);
	_plaf.x11WM_STATE = _plaf.xlibInternAtom(_plaf.x11Display, "WM_STATE", False);
	_plaf.x11WM_DELETE_WINDOW = _plaf.xlibInternAtom(_plaf.x11Display, "WM_DELETE_WINDOW", False);
	_plaf.x11NET_SUPPORTED = _plaf.xlibInternAtom(_plaf.x11Display, "_NET_SUPPORTED", False);
	_plaf.x11NET_SUPPORTING_WM_CHECK = _plaf.xlibInternAtom(_plaf.x11Display, "_NET_SUPPORTING_WM_CHECK", False);
	_plaf.x11NET_WM_ICON = _plaf.xlibInternAtom(_plaf.x11Display, "_NET_WM_ICON", False);
	_plaf.x11NET_WM_PING = _plaf.xlibInternAtom(_plaf.x11Display, "_NET_WM_PING", False);
	_plaf.x11NET_WM_PID = _plaf.xlibInternAtom(_plaf.x11Display, "_NET_WM_PID", False);
	_plaf.x11NET_WM_NAME = _plaf.xlibInternAtom(_plaf.x11Display, "_NET_WM_NAME", False);
	_plaf.x11NET_WM_ICON_NAME = _plaf.xlibInternAtom(_plaf.x11Display, "_NET_WM_ICON_NAME", False);
	_plaf.x11NET_WM_BYPASS_COMPOSITOR = _plaf.xlibInternAtom(_plaf.x11Display, "_NET_WM_BYPASS_COMPOSITOR", False);
	_plaf.x11NET_WM_WINDOW_OPACITY = _plaf.xlibInternAtom(_plaf.x11Display, "_NET_WM_WINDOW_OPACITY", False);
	_plaf.x11MOTIF_WM_HINTS = _plaf.xlibInternAtom(_plaf.x11Display, "_MOTIF_WM_HINTS", False);

	// The compositing manager selection name contains the screen number
	{
		char name[32];
		snprintf(name, sizeof(name), "_NET_WM_CM_S%u", _plaf.x11Screen);
		_plaf.x11NET_WM_CM_Sx = _plaf.xlibInternAtom(_plaf.x11Display, name, False);
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
	char* rms = _plaf.xlibResourceManagerString(_plaf.x11Display);
	if (rms)
	{
		XrmDatabase db = _plaf.xrmGetStringDatabase(rms);
		if (db)
		{
			XrmValue value;
			char* type = NULL;

			if (_plaf.xrmGetResource(db, "Xft.dpi", "Xft.Dpi", &type, &value))
			{
				if (type && strcmp(type, "String") == 0)
					xdpi = ydpi = atof(value.addr);
			}

			_plaf.xrmDestroyDatabase(db);
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
	plafImageData image = { 16, 16, pixels };
	return _plafCreateNativeCursorX11(&image, 0, 0);
}

// Create a helper window for IPC
//
static Window createHelperWindow(void)
{
	XSetWindowAttributes wa;
	wa.event_mask = PropertyChangeMask;

	return _plaf.xlibCreateWindow(_plaf.x11Display, _plaf.x11Root,
						 0, 0, 1, 1, 0, 0,
						 InputOnly,
						 DefaultVisual(_plaf.x11Display, _plaf.x11Screen),
						 CWEventMask, &wa);
}

// Create the pipe for empty events without assumuing the OS has pipe2(2)
//
static plafError* createEmptyEventPipe(void)
{
	if (pipe(_plaf.x11EmptyEventPipe) != 0)
	{
		return createErrorResponse("Failed to create empty event pipe: %s", strerror(errno));
	}

	for (int i = 0; i < 2; i++)
	{
		const int sf = fcntl(_plaf.x11EmptyEventPipe[i], F_GETFL, 0);
		const int df = fcntl(_plaf.x11EmptyEventPipe[i], F_GETFD, 0);

		if (sf == -1 || df == -1 ||
			fcntl(_plaf.x11EmptyEventPipe[i], F_SETFL, sf | O_NONBLOCK) == -1 ||
			fcntl(_plaf.x11EmptyEventPipe[i], F_SETFD, df | FD_CLOEXEC) == -1)
		{
			return createErrorResponse("Failed to set flags for empty event pipe: %s", strerror(errno));
		}
	}

	return NULL;
}

// X error handler
//
static int errorHandler(Display *display, XErrorEvent* event)
{
	if (_plaf.x11Display != display)
		return 0;

	_plaf.x11ErrorCode = event->error_code;
	return 0;
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Sets the X error handler callback
//
void _plafGrabErrorHandlerX11(void)
{
	_plaf.x11ErrorCode = Success;
	_plaf.x11ErrorHandler = _plaf.xlibSetErrorHandler(errorHandler);
}

// Clears the X error handler callback
//
void _plafReleaseErrorHandlerX11(void)
{
	// Synchronize to make sure all commands are processed
	_plaf.xlibSync(_plaf.x11Display, False);
	_plaf.xlibSetErrorHandler(_plaf.x11ErrorHandler);
	_plaf.x11ErrorHandler = NULL;
}

// Creates a native cursor object from the specified image and hotspot
//
Cursor _plafCreateNativeCursorX11(const plafImageData* image, int xhot, int yhot)
{
	Cursor cursor;

	if (!_plaf.xcursorHandle)
		return None;

	XcursorImage* native = _plaf.xcursorImageCreate(image->width, image->height);
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

	cursor = _plaf.xcursorImageLoadCursor(_plaf.x11Display, native);
	_plaf.xcursorImageDestroy(native);

	return cursor;
}

//////////////////////////////////////////////////////////////////////////
//////                       PLAF platform API                      //////
//////////////////////////////////////////////////////////////////////////

plafError* _plafInit(void) {
	// HACK: If the application has left the locale as "C" then both wide
	//       character text input and explicit UTF-8 input via XIM will break
	//       This sets the CTYPE part of the current locale from the environment
	//       in the hope that it is set to something more sane than "C"
	if (strcmp(setlocale(LC_CTYPE, NULL), "C") == 0)
		setlocale(LC_CTYPE, "");

	void* module = _plafLoadModule("libX11.so.6");
	if (!module)
	{
		return createErrorResponse("Failed to load Xlib");
	}

	FN_XInitThreads XInitThreads = (FN_XInitThreads)_plafGetModuleSymbol(module, "XInitThreads");
	FN_XrmInitialize XrmInitialize = (FN_XrmInitialize)_plafGetModuleSymbol(module, "XrmInitialize");
	FN_XOpenDisplay XOpenDisplay = (FN_XOpenDisplay)_plafGetModuleSymbol(module, "XOpenDisplay");
	if (!XInitThreads || !XrmInitialize || !XOpenDisplay) {
		_plafFreeModule(module);
		return createErrorResponse("Failed to load Xlib entry point");
	}

	XInitThreads();
	XrmInitialize();

	Display* display = XOpenDisplay(NULL);
	if (!display) {
		plafError* errRsp;
		const char* name = getenv("DISPLAY");
		if (name) {
			errRsp = createErrorResponse("Failed to open display %s", name);
		} else {
			errRsp = createErrorResponse("The DISPLAY environment variable is missing");
		}
		_plafFreeModule(module);
		return errRsp;
	}

	_plaf.x11Display = display;
	_plaf.xlibHandle = module;

	_plaf.xlibAllocSizeHints = (FN_XAllocSizeHints)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XAllocSizeHints");
	_plaf.xlibAllocWMHints = (FN_XAllocWMHints)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XAllocWMHints");
	_plaf.xlibChangeProperty = (FN_XChangeProperty)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XChangeProperty");
	_plaf.xlibChangeWindowAttributes = (FN_XChangeWindowAttributes)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XChangeWindowAttributes");
	_plaf.xlibCheckIfEvent = (FN_XCheckIfEvent)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XCheckIfEvent");
	_plaf.xlibCheckTypedWindowEvent = (FN_XCheckTypedWindowEvent)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XCheckTypedWindowEvent");
	_plaf.xlibCloseDisplay = (FN_XCloseDisplay)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XCloseDisplay");
	_plaf.xlibCloseIM = (FN_XCloseIM)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XCloseIM");
	_plaf.xlibConvertSelection = (FN_XConvertSelection)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XConvertSelection");
	_plaf.xlibCreateColormap = (FN_XCreateColormap)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XCreateColormap");
	_plaf.xlibCreateFontCursor = (FN_XCreateFontCursor)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XCreateFontCursor");
	_plaf.xlibCreateIC = (FN_XCreateIC)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XCreateIC");
	_plaf.xlibCreateRegion = (FN_XCreateRegion)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XCreateRegion");
	_plaf.xlibCreateWindow = (FN_XCreateWindow)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XCreateWindow");
	_plaf.xlibDefineCursor = (FN_XDefineCursor)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XDefineCursor");
	_plaf.xlibDeleteContext = (FN_XDeleteContext)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XDeleteContext");
	_plaf.xlibDeleteProperty = (FN_XDeleteProperty)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XDeleteProperty");
	_plaf.xlibDestroyIC = (FN_XDestroyIC)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XDestroyIC");
	_plaf.xlibDestroyRegion = (FN_XDestroyRegion)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XDestroyRegion");
	_plaf.xlibDestroyWindow = (FN_XDestroyWindow)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XDestroyWindow");
	_plaf.xlibDisplayKeycodes = (FN_XDisplayKeycodes)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XDisplayKeycodes");
	_plaf.xlibEventsQueued = (FN_XEventsQueued)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XEventsQueued");
	_plaf.xlibFilterEvent = (FN_XFilterEvent)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XFilterEvent");
	_plaf.xlibFindContext = (FN_XFindContext)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XFindContext");
	_plaf.xlibFlush = (FN_XFlush)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XFlush");
	_plaf.xlibFree = (FN_XFree)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XFree");
	_plaf.xlibFreeColormap = (FN_XFreeColormap)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XFreeColormap");
	_plaf.xlibFreeCursor = (FN_XFreeCursor)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XFreeCursor");
	_plaf.xlibFreeEventData = (FN_XFreeEventData)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XFreeEventData");
	_plaf.xlibGetErrorText = (FN_XGetErrorText)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XGetErrorText");
	_plaf.xlibGetICValues = (FN_XGetICValues)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XGetICValues");
	_plaf.xlibGetIMValues = (FN_XGetIMValues)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XGetIMValues");
	_plaf.xlibGetInputFocus = (FN_XGetInputFocus)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XGetInputFocus");
	_plaf.xlibGetKeyboardMapping = (FN_XGetKeyboardMapping)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XGetKeyboardMapping");
	_plaf.xlibGetScreenSaver = (FN_XGetScreenSaver)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XGetScreenSaver");
	_plaf.xlibGetSelectionOwner = (FN_XGetSelectionOwner)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XGetSelectionOwner");
	_plaf.xlibGetWMNormalHints = (FN_XGetWMNormalHints)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XGetWMNormalHints");
	_plaf.xlibGetWindowAttributes = (FN_XGetWindowAttributes)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XGetWindowAttributes");
	_plaf.xlibGetWindowProperty = (FN_XGetWindowProperty)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XGetWindowProperty");
	_plaf.xlibMinimizeWindow = (FN_XMinimizeWindow)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XIconifyWindow");
	_plaf.xlibInternAtom = (FN_XInternAtom)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XInternAtom");
	_plaf.xlibLookupString = (FN_XLookupString)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XLookupString");
	_plaf.xlibMapRaised = (FN_XMapRaised)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XMapRaised");
	_plaf.xlibMapWindow = (FN_XMapWindow)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XMapWindow");
	_plaf.xlibMoveResizeWindow = (FN_XMoveResizeWindow)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XMoveResizeWindow");
	_plaf.xlibMoveWindow = (FN_XMoveWindow)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XMoveWindow");
	_plaf.xlibNextEvent = (FN_XNextEvent)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XNextEvent");
	_plaf.xlibOpenIM = (FN_XOpenIM)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XOpenIM");
	_plaf.xlibPeekEvent = (FN_XPeekEvent)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XPeekEvent");
	_plaf.xlibPending = (FN_XPending)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XPending");
	_plaf.xlibQueryExtension = (FN_XQueryExtension)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XQueryExtension");
	_plaf.xlibQueryPointer = (FN_XQueryPointer)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XQueryPointer");
	_plaf.xlibRaiseWindow = (FN_XRaiseWindow)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XRaiseWindow");
	_plaf.xlibRegisterIMInstantiateCallback = (FN_XRegisterIMInstantiateCallback)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XRegisterIMInstantiateCallback");
	_plaf.xlibResizeWindow = (FN_XResizeWindow)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XResizeWindow");
	_plaf.xlibResourceManagerString = (FN_XResourceManagerString)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XResourceManagerString");
	_plaf.xlibSaveContext = (FN_XSaveContext)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSaveContext");
	_plaf.xlibSelectInput = (FN_XSelectInput)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSelectInput");
	_plaf.xlibSendEvent = (FN_XSendEvent)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSendEvent");
	_plaf.xlibSetErrorHandler = (FN_XSetErrorHandler)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSetErrorHandler");
	_plaf.xlibSetICFocus = (FN_XSetICFocus)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSetICFocus");
	_plaf.xlibSetIMValues = (FN_XSetIMValues)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSetIMValues");
	_plaf.xlibSetInputFocus = (FN_XSetInputFocus)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSetInputFocus");
	_plaf.xlibSetLocaleModifiers = (FN_XSetLocaleModifiers)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSetLocaleModifiers");
	_plaf.xlibSetScreenSaver = (FN_XSetScreenSaver)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSetScreenSaver");
	_plaf.xlibSetSelectionOwner = (FN_XSetSelectionOwner)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSetSelectionOwner");
	_plaf.xlibSetWMHints = (FN_XSetWMHints)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSetWMHints");
	_plaf.xlibSetWMNormalHints = (FN_XSetWMNormalHints)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSetWMNormalHints");
	_plaf.xlibSetWMProtocols = (FN_XSetWMProtocols)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSetWMProtocols");
	_plaf.xlibSupportsLocale = (FN_XSupportsLocale)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSupportsLocale");
	_plaf.xlibSync = (FN_XSync)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XSync");
	_plaf.xlibTranslateCoordinates = (FN_XTranslateCoordinates)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XTranslateCoordinates");
	_plaf.xlibUndefineCursor = (FN_XUndefineCursor)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XUndefineCursor");
	_plaf.xlibUnmapWindow = (FN_XUnmapWindow)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XUnmapWindow");
	_plaf.xlibUnsetICFocus = (FN_XUnsetICFocus)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XUnsetICFocus");
	_plaf.xlibWarpPointer = (FN_XWarpPointer)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XWarpPointer");
	_plaf.xkbFreeKeyboard = (FN_XkbFreeKeyboard)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XkbFreeKeyboard");
	_plaf.xkbFreeNames = (FN_XkbFreeNames)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XkbFreeNames");
	_plaf.xkbGetMap = (FN_XkbGetMap)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XkbGetMap");
	_plaf.xkbGetNames = (FN_XkbGetNames)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XkbGetNames");
	_plaf.xkbGetState = (FN_XkbGetState)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XkbGetState");
	_plaf.xkbQueryExtension = (FN_XkbQueryExtension)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XkbQueryExtension");
	_plaf.xkbSelectEventDetails = (FN_XkbSelectEventDetails)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XkbSelectEventDetails");
	_plaf.xkbSetDetectableAutoRepeat = (FN_XkbSetDetectableAutoRepeat)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XkbSetDetectableAutoRepeat");
	_plaf.xrmDestroyDatabase = (FN_XrmDestroyDatabase)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XrmDestroyDatabase");
	_plaf.xrmGetResource = (FN_XrmGetResource)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XrmGetResource");
	_plaf.xrmGetStringDatabase = (FN_XrmGetStringDatabase)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XrmGetStringDatabase");
	_plaf.xlibUnregisterIMInstantiateCallback = (FN_XUnregisterIMInstantiateCallback)
		_plafGetModuleSymbol(_plaf.xlibHandle, "XUnregisterIMInstantiateCallback");
	_plaf.xlibUTF8LookupString = (FN_Xutf8LookupString)
		_plafGetModuleSymbol(_plaf.xlibHandle, "Xutf8LookupString");
	_plaf.xlibUTF8SetWMProperties = (FN_Xutf8SetWMProperties)
		_plafGetModuleSymbol(_plaf.xlibHandle, "Xutf8SetWMProperties");

	if (_plaf.xlibUTF8LookupString && _plaf.xlibUTF8SetWMProperties)
		_plaf.xlibUTF8 = true;

	_plaf.x11Screen = DefaultScreen(_plaf.x11Display);
	_plaf.x11Root = RootWindow(_plaf.x11Display, _plaf.x11Screen);
	_plaf.x11Context = XUniqueContext();

	getSystemContentScale(&_plaf.x11ContentScaleX, &_plaf.x11ContentScaleY);

	plafError* errRsp = createEmptyEventPipe();
	if (errRsp) {
		plafTerminate();
		return errRsp;
	}

	initExtensions();

	_plaf.x11HelperWindowHandle = createHelperWindow();
	_plaf.x11HiddenCursorHandle = createHiddenCursor();

	if (_plaf.xlibSupportsLocale() && _plaf.xlibUTF8)
	{
		_plaf.xlibSetLocaleModifiers("");

		// If an IM is already present our callback will be called right away
		_plaf.xlibRegisterIMInstantiateCallback(_plaf.x11Display,
									   NULL, NULL, NULL,
									   inputMethodInstantiateCallback,
									   NULL);
	}

	_plafPollMonitorsX11();
	return NULL;
}

void _plafTerminate(void)
{
	if (_plaf.x11HelperWindowHandle)
	{
		if (_plaf.xlibGetSelectionOwner(_plaf.x11Display, _plaf.x11ClipCLIPBOARD) ==
			_plaf.x11HelperWindowHandle)
		{
			_plafPushSelectionToManagerX11();
		}

		_plaf.xlibDestroyWindow(_plaf.x11Display, _plaf.x11HelperWindowHandle);
		_plaf.x11HelperWindowHandle = None;
	}

	if (_plaf.x11HiddenCursorHandle)
	{
		_plaf.xlibFreeCursor(_plaf.x11Display, _plaf.x11HiddenCursorHandle);
		_plaf.x11HiddenCursorHandle = (Cursor) 0;
	}

	_plaf.xlibUnregisterIMInstantiateCallback(_plaf.x11Display,
									 NULL, NULL, NULL,
									 inputMethodInstantiateCallback,
									 NULL);

	if (_plaf.x11IM)
	{
		_plaf.xlibCloseIM(_plaf.x11IM);
		_plaf.x11IM = NULL;
	}

	if (_plaf.x11Display)
	{
		_plaf.xlibCloseDisplay(_plaf.x11Display);
		_plaf.x11Display = NULL;
	}

	if (_plaf.xcursorHandle)
	{
		_plafFreeModule(_plaf.xcursorHandle);
		_plaf.xcursorHandle = NULL;
	}

	if (_plaf.randrHandle)
	{
		_plafFreeModule(_plaf.randrHandle);
		_plaf.randrHandle = NULL;
	}

	if (_plaf.xineramaHandle)
	{
		_plafFreeModule(_plaf.xineramaHandle);
		_plaf.xineramaHandle = NULL;
	}

	if (_plaf.xrenderHandle)
	{
		_plafFreeModule(_plaf.xrenderHandle);
		_plaf.xrenderHandle = NULL;
	}

	if (_plaf.xvidmodeHandle)
	{
		_plafFreeModule(_plaf.xvidmodeHandle);
		_plaf.xvidmodeHandle = NULL;
	}

	if (_plaf.xiHandle)
	{
		_plafFreeModule(_plaf.xiHandle);
		_plaf.xiHandle = NULL;
	}

	// NOTE: These need to be unloaded after XCloseDisplay, as they register
	//       cleanup callbacks that get called by that function
	_plafTerminateGLX();

	if (_plaf.xlibHandle)
	{
		_plafFreeModule(_plaf.xlibHandle);
		_plaf.xlibHandle = NULL;
	}

	if (_plaf.x11EmptyEventPipe[0] || _plaf.x11EmptyEventPipe[1])
	{
		close(_plaf.x11EmptyEventPipe[0]);
		close(_plaf.x11EmptyEventPipe[1]);
	}
}

#endif // __linux__
