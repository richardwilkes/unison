#pragma once

#ifndef _PLATFORM_H
#define _PLATFORM_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdlib.h>
#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>
#include <string.h>
#include <float.h>

typedef int GLint;
typedef unsigned int GLuint;
typedef unsigned int GLenum;
typedef unsigned int GLbitfield;
typedef unsigned char GLubyte;

#if defined(__APPLE__)
	#define APIENTRY
	// NOTE: All of NSGL was deprecated in the 10.14 SDK. This disables the pointless warnings for every symbol we use.
	#ifndef GL_SILENCE_DEPRECATION
		#define GL_SILENCE_DEPRECATION
	#endif

	#import <Cocoa/Cocoa.h>
#elif defined(__linux__)
	#define APIENTRY
	#define GLX_VENDOR 1
	#define GLX_RGBA_BIT 0x00000001
	#define GLX_WINDOW_BIT 0x00000001
	#define GLX_DRAWABLE_TYPE 0x8010
	#define GLX_RENDER_TYPE 0x8011
	#define GLX_RGBA_TYPE 0x8014
	#define GLX_DOUBLEBUFFER 5
	#define GLX_AUX_BUFFERS 7
	#define GLX_RED_SIZE 8
	#define GLX_GREEN_SIZE 9
	#define GLX_BLUE_SIZE 10
	#define GLX_ALPHA_SIZE 11
	#define GLX_DEPTH_SIZE 12
	#define GLX_STENCIL_SIZE 13
	#define GLX_ACCUM_RED_SIZE 14
	#define GLX_ACCUM_GREEN_SIZE 15
	#define GLX_ACCUM_BLUE_SIZE 16
	#define GLX_ACCUM_ALPHA_SIZE 17
	#define GLX_SAMPLES 0x186a1
	#define GLX_VISUAL_ID 0x800b
	#define GLX_FRAMEBUFFER_SRGB_CAPABLE_ARB 0x20b2
	#define GLX_CONTEXT_DEBUG_BIT_ARB 0x00000001
	#define GLX_CONTEXT_COMPATIBILITY_PROFILE_BIT_ARB 0x00000002
	#define GLX_CONTEXT_CORE_PROFILE_BIT_ARB 0x00000001
	#define GLX_CONTEXT_PROFILE_MASK_ARB 0x9126
	#define GLX_CONTEXT_FORWARD_COMPATIBLE_BIT_ARB 0x00000002
	#define GLX_CONTEXT_MAJOR_VERSION_ARB 0x2091
	#define GLX_CONTEXT_MINOR_VERSION_ARB 0x2092
	#define GLX_CONTEXT_FLAGS_ARB 0x2094
	#define GLX_CONTEXT_ES2_PROFILE_BIT_EXT 0x00000004
	#define GLX_CONTEXT_ROBUST_ACCESS_BIT_ARB 0x00000004
	#define GLX_LOSE_CONTEXT_ON_RESET_ARB 0x8252
	#define GLX_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB 0x8256
	#define GLX_NO_RESET_NOTIFICATION_ARB 0x8261
	#define GLX_CONTEXT_RELEASE_BEHAVIOR_ARB 0x2097
	#define GLX_CONTEXT_RELEASE_BEHAVIOR_NONE_ARB 0
	#define GLX_CONTEXT_RELEASE_BEHAVIOR_FLUSH_ARB 0x2098
	#define GLX_CONTEXT_OPENGL_NO_ERROR_ARB 0x31b3

	#include <unistd.h>
	#include <X11/Xlib.h>
	#include <X11/Xatom.h>
	#include <X11/Xresource.h>
	#include <X11/Xcursor/Xcursor.h>
	#include <X11/extensions/Xrandr.h>
	#include <X11/XKBlib.h>
	#include <X11/extensions/Xinerama.h>
	#include <X11/extensions/shape.h>

	typedef XID GLXWindow;
	typedef XID GLXDrawable;
	typedef struct __GLXFBConfig* GLXFBConfig;
	typedef struct __GLXcontext* GLXContext;
	typedef void (*__GLXextproc)(void);

	typedef XSizeHints* (* FN_XAllocSizeHints)(void);
	typedef XWMHints* (* FN_XAllocWMHints)(void);
	typedef int (* FN_XChangeProperty)(Display*,Window,Atom,Atom,int,int,const unsigned char*,int);
	typedef int (* FN_XChangeWindowAttributes)(Display*,Window,unsigned long,XSetWindowAttributes*);
	typedef Bool (* FN_XCheckIfEvent)(Display*,XEvent*,Bool(*)(Display*,XEvent*,XPointer),XPointer);
	typedef Bool (* FN_XCheckTypedWindowEvent)(Display*,Window,int,XEvent*);
	typedef int (* FN_XCloseDisplay)(Display*);
	typedef Status (* FN_XCloseIM)(XIM);
	typedef int (* FN_XConvertSelection)(Display*,Atom,Atom,Atom,Window,Time);
	typedef Colormap (* FN_XCreateColormap)(Display*,Window,Visual*,int);
	typedef Cursor (* FN_XCreateFontCursor)(Display*,unsigned int);
	typedef XIC (* FN_XCreateIC)(XIM,...);
	typedef Region (* FN_XCreateRegion)(void);
	typedef Window (* FN_XCreateWindow)(Display*,Window,int,int,unsigned int,unsigned int,unsigned int,int,unsigned int,Visual*,unsigned long,XSetWindowAttributes*);
	typedef int (* FN_XDefineCursor)(Display*,Window,Cursor);
	typedef int (* FN_XDeleteContext)(Display*,XID,XContext);
	typedef int (* FN_XDeleteProperty)(Display*,Window,Atom);
	typedef void (*FN_XDestroyIC)(XIC);
	typedef int (* FN_XDestroyRegion)(Region);
	typedef int (* FN_XDestroyWindow)(Display*,Window);
	typedef int (* FN_XDisplayKeycodes)(Display*,int*,int*);
	typedef int (* FN_XEventsQueued)(Display*,int);
	typedef Bool (* FN_XFilterEvent)(XEvent*,Window);
	typedef int (* FN_XFindContext)(Display*,XID,XContext,XPointer*);
	typedef int (* FN_XFlush)(Display*);
	typedef int (* FN_XFree)(void*);
	typedef int (* FN_XFreeColormap)(Display*,Colormap);
	typedef int (* FN_XFreeCursor)(Display*,Cursor);
	typedef void (*FN_XFreeEventData)(Display*,XGenericEventCookie*);
	typedef char* (* FN_XGetICValues)(XIC,...);
	typedef char* (* FN_XGetIMValues)(XIM,...);
	typedef int (* FN_XGetInputFocus)(Display*,Window*,int*);
	typedef KeySym* (* FN_XGetKeyboardMapping)(Display*,KeyCode,int,int*);
	typedef int (* FN_XGetScreenSaver)(Display*,int*,int*,int*,int*);
	typedef Window (* FN_XGetSelectionOwner)(Display*,Atom);
	typedef Status (* FN_XGetWMNormalHints)(Display*,Window,XSizeHints*,long*);
	typedef Status (* FN_XGetWindowAttributes)(Display*,Window,XWindowAttributes*);
	typedef int (* FN_XGetWindowProperty)(Display*,Window,Atom,long,long,Bool,Atom,Atom*,int*,unsigned long*,unsigned long*,unsigned char**);
	typedef Status (* FN_XMinimizeWindow)(Display*,Window,int);
	typedef Status (* FN_XInitThreads)(void);
	typedef Atom (* FN_XInternAtom)(Display*,const char*,Bool);
	typedef int (* FN_XLookupString)(XKeyEvent*,char*,int,KeySym*,XComposeStatus*);
	typedef int (* FN_XMapRaised)(Display*,Window);
	typedef int (* FN_XMapWindow)(Display*,Window);
	typedef int (* FN_XMoveResizeWindow)(Display*,Window,int,int,unsigned int,unsigned int);
	typedef int (* FN_XMoveWindow)(Display*,Window,int,int);
	typedef int (* FN_XNextEvent)(Display*,XEvent*);
	typedef Display* (* FN_XOpenDisplay)(const char*);
	typedef XIM (* FN_XOpenIM)(Display*,XrmDatabase*,char*,char*);
	typedef int (* FN_XPeekEvent)(Display*,XEvent*);
	typedef int (* FN_XPending)(Display*);
	typedef Bool (* FN_XQueryExtension)(Display*,const char*,int*,int*,int*);
	typedef Bool (* FN_XQueryPointer)(Display*,Window,Window*,Window*,int*,int*,int*,int*,unsigned int*);
	typedef int (* FN_XRaiseWindow)(Display*,Window);
	typedef Bool (* FN_XRegisterIMInstantiateCallback)(Display*,void*,char*,char*,XIDProc,XPointer);
	typedef int (* FN_XResizeWindow)(Display*,Window,unsigned int,unsigned int);
	typedef char* (* FN_XResourceManagerString)(Display*);
	typedef int (* FN_XSaveContext)(Display*,XID,XContext,const char*);
	typedef int (* FN_XSelectInput)(Display*,Window,long);
	typedef Status (* FN_XSendEvent)(Display*,Window,Bool,long,XEvent*);
	typedef XErrorHandler (* FN_XSetErrorHandler)(XErrorHandler);
	typedef void (*FN_XSetICFocus)(XIC);
	typedef char* (* FN_XSetIMValues)(XIM,...);
	typedef int (* FN_XSetInputFocus)(Display*,Window,int,Time);
	typedef char* (* FN_XSetLocaleModifiers)(const char*);
	typedef int (* FN_XSetScreenSaver)(Display*,int,int,int,int);
	typedef int (* FN_XSetSelectionOwner)(Display*,Atom,Window,Time);
	typedef int (* FN_XSetWMHints)(Display*,Window,XWMHints*);
	typedef void (*FN_XSetWMNormalHints)(Display*,Window,XSizeHints*);
	typedef Status (* FN_XSetWMProtocols)(Display*,Window,Atom*,int);
	typedef Bool (* FN_XSupportsLocale)(void);
	typedef int (* FN_XSync)(Display*,Bool);
	typedef Bool (* FN_XTranslateCoordinates)(Display*,Window,Window,int,int,int*,int*,Window*);
	typedef int (* FN_XUndefineCursor)(Display*,Window);
	typedef int (* FN_XUnmapWindow)(Display*,Window);
	typedef void (*FN_XUnsetICFocus)(XIC);
	typedef int (* FN_XWarpPointer)(Display*,Window,Window,int,int,unsigned int,unsigned int,int,int);
	typedef void (*FN_XkbFreeKeyboard)(XkbDescPtr,unsigned int,Bool);
	typedef void (*FN_XkbFreeNames)(XkbDescPtr,unsigned int,Bool);
	typedef XkbDescPtr (* FN_XkbGetMap)(Display*,unsigned int,unsigned int);
	typedef Status (* FN_XkbGetNames)(Display*,unsigned int,XkbDescPtr);
	typedef Status (* FN_XkbGetState)(Display*,unsigned int,XkbStatePtr);
	typedef Bool (* FN_XkbQueryExtension)(Display*,int*,int*,int*,int*,int*);
	typedef Bool (* FN_XkbSelectEventDetails)(Display*,unsigned int,unsigned int,unsigned long,unsigned long);
	typedef Bool (* FN_XkbSetDetectableAutoRepeat)(Display*,Bool,Bool*);
	typedef void (*FN_XrmDestroyDatabase)(XrmDatabase);
	typedef Bool (* FN_XrmGetResource)(XrmDatabase,const char*,const char*,char**,XrmValue*);
	typedef XrmDatabase (* FN_XrmGetStringDatabase)(const char*);
	typedef void (*FN_XrmInitialize)(void);
	typedef Bool (* FN_XUnregisterIMInstantiateCallback)(Display*,void*,char*,char*,XIDProc,XPointer);
	typedef int (* FN_Xutf8LookupString)(XIC,XKeyPressedEvent*,char*,int,KeySym*,Status*);
	typedef void (*FN_Xutf8SetWMProperties)(Display*,Window,const char*,const char*,char**,int,XSizeHints*,XWMHints*,XClassHint*);

	typedef XRRCrtcGamma* (* FN_XRRAllocGamma)(int);
	typedef void (*FN_XRRFreeCrtcInfo)(XRRCrtcInfo*);
	typedef void (*FN_XRRFreeGamma)(XRRCrtcGamma*);
	typedef void (*FN_XRRFreeOutputInfo)(XRROutputInfo*);
	typedef void (*FN_XRRFreeScreenResources)(XRRScreenResources*);
	typedef XRRCrtcGamma* (* FN_XRRGetCrtcGamma)(Display*,RRCrtc);
	typedef int (* FN_XRRGetCrtcGammaSize)(Display*,RRCrtc);
	typedef XRRCrtcInfo* (* FN_XRRGetCrtcInfo) (Display*,XRRScreenResources*,RRCrtc);
	typedef XRROutputInfo* (* FN_XRRGetOutputInfo)(Display*,XRRScreenResources*,RROutput);
	typedef RROutput (* FN_XRRGetOutputPrimary)(Display*,Window);
	typedef XRRScreenResources* (* FN_XRRGetScreenResourcesCurrent)(Display*,Window);
	typedef Bool (* FN_XRRQueryExtension)(Display*,int*,int*);
	typedef Status (* FN_XRRQueryVersion)(Display*,int*,int*);
	typedef void (*FN_XRRSelectInput)(Display*,Window,int);
	typedef Status (* FN_XRRSetCrtcConfig)(Display*,XRRScreenResources*,RRCrtc,Time,int,int,RRMode,Rotation,RROutput*,int);
	typedef void (*FN_XRRSetCrtcGamma)(Display*,RRCrtc,XRRCrtcGamma*);
	typedef int (* FN_XRRUpdateConfiguration)(XEvent*);

	typedef XcursorImage* (* FN_XcursorImageCreate)(int,int);
	typedef void (*FN_XcursorImageDestroy)(XcursorImage*);
	typedef Cursor (* FN_XcursorImageLoadCursor)(Display*,const XcursorImage*);
	typedef char* (* FN_XcursorGetTheme)(Display*);
	typedef int (* FN_XcursorGetDefaultSize)(Display*);
	typedef XcursorImage* (* FN_XcursorLibraryLoadImage)(const char*,const char*,int);

	typedef Bool (* FN_XineramaIsActive)(Display*);
	typedef Bool (* FN_XineramaQueryExtension)(Display*,int*,int*);
	typedef XineramaScreenInfo* (* FN_XineramaQueryScreens)(Display*,int*);

	typedef Bool (* FN_XF86VidModeQueryExtension)(Display*,int*,int*);
	typedef Bool (* FN_XF86VidModeGetGammaRamp)(Display*,int,int,unsigned short*,unsigned short*,unsigned short*);
	typedef Bool (* FN_XF86VidModeSetGammaRamp)(Display*,int,int,unsigned short*,unsigned short*,unsigned short*);
	typedef Bool (* FN_XF86VidModeGetGammaRampSize)(Display*,int,int*);

	typedef Status (* FN_XIQueryVersion)(Display*,int*,int*);

	typedef Bool (* FN_XRenderQueryExtension)(Display*,int*,int*);
	typedef Status (* FN_XRenderQueryVersion)(Display*dpy,int*,int*);
	typedef XRenderPictFormat* (* FN_XRenderFindVisualFormat)(Display*,Visual const*);

	typedef Bool (* FN_XShapeQueryExtension)(Display*,int*,int*);
	typedef Status (* FN_XShapeQueryVersion)(Display*dpy,int*,int*);
	typedef void (*FN_XShapeCombineRegion)(Display*,Window,int,int,int,Region,int);
	typedef void (*FN_XShapeCombineMask)(Display*,Window,int,int,int,Pixmap,int);

	typedef int (*FN_GLXGETFBCONFIGATTRIB)(Display*,GLXFBConfig,int,int*);
	typedef const char* (*FN_GLXGETCLIENTSTRING)(Display*,int);
	typedef Bool (*FN_GLXQUERYEXTENSION)(Display*,int*,int*);
	typedef Bool (*FN_GLXQUERYVERSION)(Display*,int*,int*);
	typedef void (*FN_GLXDESTROYCONTEXT)(Display*,GLXContext);
	typedef Bool (*FN_GLXMAKECURRENT)(Display*,GLXDrawable,GLXContext);
	typedef void (*FN_GLXSWAPBUFFERS)(Display*,GLXDrawable);
	typedef const char* (*FN_GLXQUERYEXTENSIONSSTRING)(Display*,int);
	typedef GLXFBConfig* (*FN_GLXGETFBCONFIGS)(Display*,int,int*);
	typedef GLXContext (*FN_GLXCREATENEWCONTEXT)(Display*,GLXFBConfig,int,GLXContext,Bool);
	typedef __GLXextproc (* FN_GLXGETPROCADDRESS)(const GLubyte *procName);
	typedef void (*FN_GLXSWAPINTERVALEXT)(Display*,GLXDrawable,int);
	typedef XVisualInfo* (*FN_GLXGETVISUALFROMFBCONFIG)(Display*,GLXFBConfig);
	typedef GLXWindow (*FN_GLXCREATEWINDOW)(Display*,GLXFBConfig,Window,const int*);
	typedef void (*FN_GLXDESTROYWINDOW)(Display*,GLXWindow);

	typedef int (*FN_GLXSWAPINTERVALSGI)(int);
	typedef GLXContext (*FN_GLXCREATECONTEXTATTRIBSARB)(Display*,GLXFBConfig,GLXContext,Bool,const int*);
#elif defined(_WIN32)
	#define DIRECTINPUT_VERSION 0x0800
	#define OEMRESOURCE
	#ifndef NOMINMAX
		#define NOMINMAX
	#endif
	#ifndef VC_EXTRALEAN
		#define VC_EXTRALEAN
	#endif
	#ifndef WIN32_LEAN_AND_MEAN
		#define WIN32_LEAN_AND_MEAN
	#endif
	#ifndef UNICODE
		#define UNICODE
	#endif
	// Require Windows 10 or later
	#if WINVER < 0x0A00
		#undef WINVER
		#define WINVER 0x0A00
	#endif
	#if _WIN32_WINNT < 0x0A00
		#undef _WIN32_WINNT
		#define _WIN32_WINNT 0x0A00
	#endif

	#include <wctype.h>
	#include <windows.h>
	#include <dwmapi.h>
	#include <dinput.h>
	#include <dbt.h>

	#ifndef WM_COPYGLOBALDATA
		#define WM_COPYGLOBALDATA 0x0049
	#endif
	#ifndef DPI_ENUMS_DECLARED
		typedef enum {
			PROCESS_DPI_UNAWARE = 0,
			PROCESS_SYSTEM_DPI_AWARE = 1,
			PROCESS_PER_MONITOR_DPI_AWARE = 2
		} PROCESS_DPI_AWARENESS;
		typedef enum {
			MDT_EFFECTIVE_DPI = 0,
			MDT_ANGULAR_DPI = 1,
			MDT_RAW_DPI = 2,
			MDT_DEFAULT = MDT_EFFECTIVE_DPI
		} MONITOR_DPI_TYPE;
	#endif
	// Windows 10 Anniversary Update
	#define IsWindows10Version1607OrGreater() _plafIsWindows10BuildOrGreater(14393)
	// Windows 10 Creators Update
	#define IsWindows10Version1703OrGreater() _plafIsWindows10BuildOrGreater(15063)
	#define WGL_NUMBER_PIXEL_FORMATS_ARB 0x2000
	#define WGL_SUPPORT_OPENGL_ARB 0x2010
	#define WGL_DRAW_TO_WINDOW_ARB 0x2001
	#define WGL_PIXEL_TYPE_ARB 0x2013
	#define WGL_TYPE_RGBA_ARB 0x202b
	#define WGL_ACCELERATION_ARB 0x2003
	#define WGL_NO_ACCELERATION_ARB 0x2025
	#define WGL_RED_BITS_ARB 0x2015
	#define WGL_RED_SHIFT_ARB 0x2016
	#define WGL_GREEN_BITS_ARB 0x2017
	#define WGL_GREEN_SHIFT_ARB 0x2018
	#define WGL_BLUE_BITS_ARB 0x2019
	#define WGL_BLUE_SHIFT_ARB 0x201a
	#define WGL_ALPHA_BITS_ARB 0x201b
	#define WGL_ALPHA_SHIFT_ARB 0x201c
	#define WGL_ACCUM_BITS_ARB 0x201d
	#define WGL_ACCUM_RED_BITS_ARB 0x201e
	#define WGL_ACCUM_GREEN_BITS_ARB 0x201f
	#define WGL_ACCUM_BLUE_BITS_ARB 0x2020
	#define WGL_ACCUM_ALPHA_BITS_ARB 0x2021
	#define WGL_DEPTH_BITS_ARB 0x2022
	#define WGL_STENCIL_BITS_ARB 0x2023
	#define WGL_AUX_BUFFERS_ARB 0x2024
	#define WGL_DOUBLE_BUFFER_ARB 0x2011
	#define WGL_SAMPLES_ARB 0x2042
	#define WGL_FRAMEBUFFER_SRGB_CAPABLE_ARB 0x20a9
	#define WGL_CONTEXT_DEBUG_BIT_ARB 0x00000001
	#define WGL_CONTEXT_FORWARD_COMPATIBLE_BIT_ARB 0x00000002
	#define WGL_CONTEXT_PROFILE_MASK_ARB 0x9126
	#define WGL_CONTEXT_CORE_PROFILE_BIT_ARB 0x00000001
	#define WGL_CONTEXT_COMPATIBILITY_PROFILE_BIT_ARB 0x00000002
	#define WGL_CONTEXT_MAJOR_VERSION_ARB 0x2091
	#define WGL_CONTEXT_MINOR_VERSION_ARB 0x2092
	#define WGL_CONTEXT_FLAGS_ARB 0x2094
	#define WGL_CONTEXT_ES2_PROFILE_BIT_EXT 0x00000004
	#define WGL_CONTEXT_ROBUST_ACCESS_BIT_ARB 0x00000004
	#define WGL_LOSE_CONTEXT_ON_RESET_ARB 0x8252
	#define WGL_CONTEXT_RESET_NOTIFICATION_STRATEGY_ARB 0x8256
	#define WGL_NO_RESET_NOTIFICATION_ARB 0x8261
	#define WGL_CONTEXT_RELEASE_BEHAVIOR_ARB 0x2097
	#define WGL_CONTEXT_RELEASE_BEHAVIOR_NONE_ARB 0
	#define WGL_CONTEXT_RELEASE_BEHAVIOR_FLUSH_ARB 0x2098
	#define WGL_CONTEXT_OPENGL_NO_ERROR_ARB 0x31b3
	#define WGL_COLORSPACE_EXT 0x309d
	#define WGL_COLORSPACE_SRGB_EXT 0x3089
	#define ERROR_INVALID_VERSION_ARB 0x2095
	#define ERROR_INVALID_PROFILE_ARB 0x2096
	#define ERROR_INCOMPATIBLE_DEVICE_CONTEXTS_ARB 0x2054

	// user32.dll function pointer typedefs
	typedef BOOL (WINAPI * FN_EnableNonClientDpiScaling)(HWND);
	typedef BOOL (WINAPI * FN_SetProcessDpiAwarenessContext)(HANDLE);
	typedef UINT (WINAPI * FN_GetDpiForWindow)(HWND);
	typedef BOOL (WINAPI * FN_AdjustWindowRectExForDpi)(LPRECT,DWORD,BOOL,DWORD,UINT);
	typedef int (WINAPI * FN_GetSystemMetricsForDpi)(int,UINT);

	// dwmapi.dll function pointer typedefs
	typedef HRESULT (WINAPI * FN_DwmIsCompositionEnabled)(BOOL*);
	typedef HRESULT (WINAPI * FN_DwmFlush)(VOID);
	typedef HRESULT (WINAPI * FN_DwmEnableBlurBehindWindow)(HWND,const DWM_BLURBEHIND*);
	typedef HRESULT (WINAPI * FN_DwmGetColorizationColor)(DWORD*,BOOL*);

	// shcore.dll function pointer typedefs
	typedef HRESULT (WINAPI * FN_SetProcessDpiAwareness)(PROCESS_DPI_AWARENESS);
	typedef HRESULT (WINAPI * FN_GetDpiForMonitor)(HMONITOR,MONITOR_DPI_TYPE,UINT*,UINT*);

	// ntdll.dll function pointer typedefs
	typedef LONG (WINAPI * FN_RtlVerifyVersionInfo)(OSVERSIONINFOEXW*,ULONG,ULONGLONG);

	// WGL extension pointer typedefs
	typedef BOOL (WINAPI * FN_WGLSWAPINTERVALEXT)(int);
	typedef BOOL (WINAPI * FN_WGLGETPIXELFORMATATTRIBIVARB)(HDC,int,int,UINT,const int*,int*);
	typedef const char* (WINAPI * FN_WGLGETEXTENSIONSSTRINGEXT)(void);
	typedef const char* (WINAPI * FN_WGLGETEXTENSIONSSTRINGARB)(HDC);
	typedef HGLRC (WINAPI * FN_WGLCREATECONTEXTATTRIBSARB)(HDC,HGLRC,const int*);

	// opengl32.dll function pointer typedefs
	typedef HGLRC (WINAPI * FN_wglCreateContext)(HDC);
	typedef BOOL (WINAPI * FN_wglDeleteContext)(HGLRC);
	typedef PROC (WINAPI * FN_wglGetProcAddress)(LPCSTR);
	typedef HDC (WINAPI * FN_wglGetCurrentDC)(void);
	typedef HGLRC (WINAPI * FN_wglGetCurrentContext)(void);
	typedef BOOL (WINAPI * FN_wglMakeCurrent)(HDC,HGLRC);
	typedef BOOL (WINAPI * FN_wglShareLists)(HGLRC,HGLRC);
#endif

// Input constants
#define INPUT_RELEASE 0 // The key or button was released.
#define INPUT_PRESS   1 // The key or button was pressed.
#define INPUT_REPEAT  2 // The key was held down until it repeated.

// Key codes
#define KEY_UNKNOWN       -1
#define KEY_SPACE         32
#define KEY_APOSTROPHE    39
#define KEY_COMMA         44
#define KEY_MINUS         45
#define KEY_PERIOD        46
#define KEY_SLASH         47
#define KEY_0             48
#define KEY_1             49
#define KEY_2             50
#define KEY_3             51
#define KEY_4             52
#define KEY_5             53
#define KEY_6             54
#define KEY_7             55
#define KEY_8             56
#define KEY_9             57
#define KEY_SEMICOLON     59
#define KEY_EQUAL         61
#define KEY_A             65
#define KEY_B             66
#define KEY_C             67
#define KEY_D             68
#define KEY_E             69
#define KEY_F             70
#define KEY_G             71
#define KEY_H             72
#define KEY_I             73
#define KEY_J             74
#define KEY_K             75
#define KEY_L             76
#define KEY_M             77
#define KEY_N             78
#define KEY_O             79
#define KEY_P             80
#define KEY_Q             81
#define KEY_R             82
#define KEY_S             83
#define KEY_T             84
#define KEY_U             85
#define KEY_V             86
#define KEY_W             87
#define KEY_X             88
#define KEY_Y             89
#define KEY_Z             90
#define KEY_LEFT_BRACKET  91
#define KEY_BACKSLASH     92
#define KEY_RIGHT_BRACKET 93
#define KEY_GRAVE_ACCENT  96
#define KEY_WORLD_1       161
#define KEY_WORLD_2       162
#define KEY_ESCAPE        256
#define KEY_ENTER         257
#define KEY_TAB           258
#define KEY_BACKSPACE     259
#define KEY_INSERT        260
#define KEY_DELETE        261
#define KEY_RIGHT         262
#define KEY_LEFT          263
#define KEY_DOWN          264
#define KEY_UP            265
#define KEY_PAGE_UP       266
#define KEY_PAGE_DOWN     267
#define KEY_HOME          268
#define KEY_END           269
#define KEY_CAPS_LOCK     280
#define KEY_SCROLL_LOCK   281
#define KEY_NUM_LOCK      282
#define KEY_PRINT_SCREEN  283
#define KEY_PAUSE         284
#define KEY_F1            290
#define KEY_F2            291
#define KEY_F3            292
#define KEY_F4            293
#define KEY_F5            294
#define KEY_F6            295
#define KEY_F7            296
#define KEY_F8            297
#define KEY_F9            298
#define KEY_F10           299
#define KEY_F11           300
#define KEY_F12           301
#define KEY_F13           302
#define KEY_F14           303
#define KEY_F15           304
#define KEY_F16           305
#define KEY_F17           306
#define KEY_F18           307
#define KEY_F19           308
#define KEY_F20           309
#define KEY_F21           310
#define KEY_F22           311
#define KEY_F23           312
#define KEY_F24           313
#define KEY_F25           314
#define KEY_KP_0          320
#define KEY_KP_1          321
#define KEY_KP_2          322
#define KEY_KP_3          323
#define KEY_KP_4          324
#define KEY_KP_5          325
#define KEY_KP_6          326
#define KEY_KP_7          327
#define KEY_KP_8          328
#define KEY_KP_9          329
#define KEY_KP_DECIMAL    330
#define KEY_KP_DIVIDE     331
#define KEY_KP_MULTIPLY   332
#define KEY_KP_SUBTRACT   333
#define KEY_KP_ADD        334
#define KEY_KP_ENTER      335
#define KEY_KP_EQUAL      336
#define KEY_LEFT_SHIFT    340
#define KEY_LEFT_CONTROL  341
#define KEY_LEFT_ALT      342
#define KEY_LEFT_SUPER    343
#define KEY_RIGHT_SHIFT   344
#define KEY_RIGHT_CONTROL 345
#define KEY_RIGHT_ALT     346
#define KEY_RIGHT_SUPER   347
#define KEY_MENU          348
#define KEY_LAST          KEY_MENU
#define MAX_KEY_CODES     512

// Modifier key flags
#define KEYMOD_SHIFT     0x0001
#define KEYMOD_CONTROL   0x0002
#define KEYMOD_ALT       0x0004
#define KEYMOD_SUPER     0x0008
#define KEYMOD_CAPS_LOCK 0x0010
#define KEYMOD_NUM_LOCK  0x0020

// Mouse button IDs
#define MOUSE_BUTTON_1         0
#define MOUSE_BUTTON_2         1
#define MOUSE_BUTTON_3         2
#define MOUSE_BUTTON_4         3
#define MOUSE_BUTTON_5         4
#define MOUSE_BUTTON_6         5
#define MOUSE_BUTTON_7         6
#define MOUSE_BUTTON_8         7
#define MOUSE_BUTTON_LAST      MOUSE_BUTTON_8
#define MOUSE_BUTTON_LEFT      MOUSE_BUTTON_1
#define MOUSE_BUTTON_RIGHT     MOUSE_BUTTON_2
#define MOUSE_BUTTON_MIDDLE    MOUSE_BUTTON_3

// Peripheral connection status codes
#define CONNECTED    0x00040001
#define DISCONNECTED 0x00040002

// Window attributes and/or hints
#define WINDOW_ATTR_FOCUSED                        0x00020001
#define WINDOW_ATTR_MINIMIZED                      0x00020002
#define WINDOW_ATTR_HINT_RESIZABLE                 0x00020003
#define WINDOW_ATTR_VISIBLE                        0x00020004
#define WINDOW_ATTR_HINT_DECORATED                 0x00020005
#define WINDOW_ATTR_HINT_FLOATING                  0x00020007
#define WINDOW_ATTR_HINT_MAXIMIZED                 0x00020008
#define WINDOW_ATTR_HINT_TRANSPARENT_FRAMEBUFFER   0x0002000A
#define WINDOW_ATTR_HOVERED                        0x0002000B
#define WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH         0x0002000D
#define WINDOW_HINT_POSITION_X                     0x0002000E
#define WINDOW_HINT_POSITION_Y                     0x0002000F
#define WINDOW_HINT_RED_BITS                       0x00021001
#define WINDOW_HINT_GREEN_BITS                     0x00021002
#define WINDOW_HINT_BLUE_BITS                      0x00021003
#define WINDOW_HINT_ALPHA_BITS                     0x00021004
#define WINDOW_HINT_DEPTH_BITS                     0x00021005
#define WINDOW_HINT_STENCIL_BITS                   0x00021006
#define WINDOW_HINT_ACCUM_RED_BITS                 0x00021007
#define WINDOW_HINT_ACCUM_GREEN_BITS               0x00021008
#define WINDOW_HINT_ACCUM_BLUE_BITS                0x00021009
#define WINDOW_HINT_ACCUM_ALPHA_BITS               0x0002100A
#define WINDOW_HINT_AUX_BUFFERS                    0x0002100B
#define WINDOW_HINT_SAMPLES                        0x0002100D
#define WINDOW_HINT_SRGB_CAPABLE                   0x0002100E
#define WINDOW_HINT_REFRESH_RATE                   0x0002100F
#define WINDOW_ATTR_HINT_DOUBLE_BUFFER             0x00021010
#define WINDOW_ATTR_HINT_CONTEXT_VERSION_MAJOR     0x00022002
#define WINDOW_ATTR_HINT_CONTEXT_VERSION_MINOR     0x00022003
#define WINDOW_ATTR_CONTEXT_REVISION               0x00022004
#define WINDOW_ATTR_HINT_CONTEXT_ROBUSTNESS        0x00022005
#define WINDOW_ATTR_HINT_OPENGL_FORWARD_COMPAT     0x00022006
#define WINDOW_ATTR_HINT_CONTEXT_DEBUG             0x00022007
#define WINDOW_ATTR_HINT_OPENGL_PROFILE            0x00022008
#define WINDOW_ATTR_HINT_CONTEXT_RELEASE_BEHAVIOR  0x00022009
#define WINDOW_ATTR_HINT_CONTEXT_ERROR_SUPPRESSION 0x0002200A
#define WINDOW_HINT_SCALE_TO_MONITOR               0x0002200C
#define WINDOW_HINT_SCALE_FRAMEBUFFER              0x0002200D

// Context robustness values
#define CONTEXT_ROBUSTNESS_NONE                   0
#define CONTEXT_ROBUSTNESS_NO_RESET_NOTIFICATION  0x00031001
#define CONTEXT_ROBUSTNESS_LOSE_CONTEXT_ON_RESET  0x00031002

// OpenGL profile values
#define OPENGL_PROFILE_ANY     0
#define OPENGL_PROFILE_CORE    0x00032001
#define OPENGL_PROFILE_COMPAT  0x00032002

// Context release behavior values
#define RELEASE_BEHAVIOR_ANY   0
#define RELEASE_BEHAVIOR_FLUSH 0x00035001
#define RELEASE_BEHAVIOR_NONE  0x00035002

// Standard cursor IDs
#define STD_CURSOR_ARROW             0x00036001
#define STD_CURSOR_IBEAM             0x00036002
#define STD_CURSOR_CROSSHAIR         0x00036003
#define STD_CURSOR_POINTING_HAND     0x00036004
#define STD_CURSOR_HORIZONTAL_RESIZE 0x00036005
#define STD_CURSOR_VERTICAL_RESIZE   0x00036006

#define ANY_POSITION 0x80000000

#define DONT_CARE    -1

#define ERROR_MSG_SIZE 1024

typedef int IntBool;

// Forward declarations
typedef struct plafCursor plafCursor;
typedef struct plafMonitor plafMonitor;
typedef struct plafWindow plafWindow;

// Function pointer definitions
typedef void (*charFunc)(plafWindow* window, unsigned int codepoint);
typedef void (*charModsFunc)(plafWindow* window, unsigned int codepoint, int mods);
typedef void (*cursorEnterFunc)(plafWindow* window, int entered);
typedef void (*cursorPosFunc)(plafWindow* window, double xpos, double ypos);
typedef void (*dropFunc)(plafWindow* window, int path_count, const char* paths[]);
typedef void (*errorFunc)(const char* description);
typedef void (*frameBufferSizeFunc)(plafWindow* window, int width, int height);
typedef void (*glFunc)(void);
typedef void (*keyFunc)(plafWindow* window, int key, int scancode, int action, int mods);
typedef void (*monitorFunc)(plafMonitor* monitor, int event);
typedef void (*mouseButtonFunc)(plafWindow* window, int button, int action, int mods);
typedef void (*scrollFunc)(plafWindow* window, double xoffset, double yoffset);
typedef void (*windowCloseFunc)(plafWindow* window);
typedef void (*windowContextScaleFunc)(plafWindow* window, float xscale, float yscale);
typedef void (*windowFocusFunc)(plafWindow* window, int focused);
typedef void (*windowMinimizeFunc)(plafWindow* window, int minimize);
typedef void (*windowMaximizeFunc)(plafWindow* window, int maximized);
typedef void (*windowPosFunc)(plafWindow* window, int xpos, int ypos); // coordinates are content area upper-left
typedef void (*windowRefreshFunc)(plafWindow* window);
typedef void (*windowSizeFunc)(plafWindow* window, int width, int height);

// An error response
typedef struct plafError {
	struct plafError* next;
	char              desc[ERROR_MSG_SIZE];
} plafError;

// A single video mode
typedef struct plafVideoMode {
	int width;
	int height;
	int redBits;
	int greenBits;
	int blueBits;
	int refreshRate;
} plafVideoMode;

// Gamma ramp for a monitor
typedef struct plafGammaRamp {
	unsigned short* red;
	unsigned short* green;
	unsigned short* blue;
	unsigned int    size;
} plafGammaRamp;

typedef struct plafImageData {
	int            width;
	int            height;
	unsigned char* pixels;
} plafImageData;

typedef struct plafWindowConfig {
	int     xpos;
	int     ypos;
	int     width;
	int     height;
	IntBool resizable;
	IntBool decorated;
	IntBool floating;
	IntBool maximized;
	IntBool mousePassthrough;
	IntBool scaleToMonitor;
	IntBool scaleFramebuffer;
} plafWindowConfig;

/* ------------------------- Internal ----------------------- */

#define MONITOR_INSERT_FIRST      0
#define MONITOR_INSERT_LAST       1

typedef void (*moduleFunc)(void);

typedef struct plafCtxCfg         plafCtxCfg;
typedef struct plafFrameBufferCfg plafFrameBufferCfg;
typedef struct plafCtx            plafCtx;
typedef struct plafLib            plafLib;

#define GL_VERSION 0x1f02
#define GL_NONE 0
#define GL_COLOR_BUFFER_BIT 0x00004000
#define GL_UNSIGNED_BYTE 0x1401
#define GL_EXTENSIONS 0x1f03
#define GL_NUM_EXTENSIONS 0x821d
#define GL_CONTEXT_FLAGS 0x821e
#define GL_CONTEXT_FLAG_FORWARD_COMPATIBLE_BIT 0x00000001
#define GL_CONTEXT_FLAG_DEBUG_BIT 0x00000002
#define GL_CONTEXT_PROFILE_MASK 0x9126
#define GL_CONTEXT_COMPATIBILITY_PROFILE_BIT 0x00000002
#define GL_CONTEXT_CORE_PROFILE_BIT 0x00000001
#define GL_RESET_NOTIFICATION_STRATEGY_ARB 0x8256
#define GL_LOSE_CONTEXT_ON_RESET_ARB 0x8252
#define GL_NO_RESET_NOTIFICATION_ARB 0x8261
#define GL_CONTEXT_RELEASE_BEHAVIOR 0x82fb
#define GL_CONTEXT_RELEASE_BEHAVIOR_FLUSH 0x82fc
#define GL_CONTEXT_FLAG_NO_ERROR_BIT_KHR 0x00000008

typedef void (APIENTRY * FN_GLCLEAR)(GLbitfield);
typedef const GLubyte* (APIENTRY * FN_GLGETSTRING)(GLenum);
typedef void (APIENTRY * FN_GLGETINTEGERV)(GLenum,GLint*);
typedef const GLubyte* (APIENTRY * FN_GLGETSTRINGI)(GLenum,GLuint);

// Swaps the provided pointers
#define SWAP(type, x, y) \
	{                          \
		type t;                \
		t = x;                 \
		x = y;                 \
		y = t;                 \
	}

// Context configuration
//
// Parameters relating to the creation of the context but not directly related
// to the framebuffer.  This is used to pass context creation parameters from
// shared code to the platform API.
struct plafCtxCfg {
	int         major;
	int         minor;
	IntBool     forward;
	IntBool     debug;
	IntBool     noerror;
	int         profile;
	int         robustness;
	int         release;
	plafWindow* share;
};

// Framebuffer configuration
//
// This describes buffers and their sizes.  It also contains
// a platform-specific ID used to map back to the backend API object.
//
// It is used to pass framebuffer parameters from shared code to the platform
// API and also to enumerate and select available framebuffer configs.
struct plafFrameBufferCfg {
	int       redBits;
	int       greenBits;
	int       blueBits;
	int       alphaBits;
	int       depthBits;
	int       stencilBits;
	int       accumRedBits;
	int       accumGreenBits;
	int       accumBlueBits;
	int       accumAlphaBits;
	int       auxBuffers;
	int       samples;
	IntBool   sRGB;
	IntBool   doublebuffer;
	IntBool   transparent;
	uintptr_t handle;
};

// Context structure
struct plafCtx {
	int                  major;
	int                  minor;
	int                  revision;
	IntBool              forward;
	IntBool              debug;
	IntBool              noerror;
	int                  profile;
	int                  robustness;
	int                  release;
	FN_GLGETSTRINGI      GetStringi;
	FN_GLGETINTEGERV     GetIntegerv;
	FN_GLGETSTRING       GetString;
	plafError*           (*makeCurrent)(plafWindow*);
	void                 (*swapBuffers)(plafWindow*);
	void                 (*swapInterval)(int);
	int                  (*extensionSupported)(const char*);
	glFunc               (*getProcAddress)(const char*);
	void                 (*destroy)(plafWindow*);
#if defined(__APPLE__)
	NSOpenGLPixelFormat* nsglPixelFormat;
	NSOpenGLContext*     nsglCtx;
#elif defined(__linux__)
	GLXContext           glxHandle;
	GLXWindow            glxWindow;
	GLXFBConfig          glxFBConfig;
#elif defined(_WIN32)
	HDC                  wglDC;
	HGLRC                wglGLRC;
	int                  wglInterval;
#endif
};

// Window and context structure
struct plafWindow {
	struct plafWindow*     next;
	IntBool                resizable;
	IntBool                decorated;
	IntBool                floating;
	IntBool                maximized;
	IntBool                mousePassthrough;
	IntBool                shouldClose;
	IntBool                doublebuffer;
	plafVideoMode          videoMode;
	plafMonitor*           monitor;
	plafCursor*            cursor;
	char*                  title;
	int                    width;
	int                    height;
	int                    minwidth;
	int                    minheight;
	int                    maxwidth;
	int                    maxheight;
	int                    numer;
	int                    denom;
	IntBool                cursorHidden;
	char                   mouseButtons[MOUSE_BUTTON_LAST + 1];
	char                   keys[KEY_LAST + 1];
	double                 virtualCursorPosX;
	double                 virtualCursorPosY;
	plafCtx                context;
	windowPosFunc          posCallback;
	windowSizeFunc         sizeCallback;
	windowCloseFunc        closeCallback;
	windowRefreshFunc      refreshCallback;
	windowFocusFunc        focusCallback;
	windowMinimizeFunc     minimizeCallback;
	windowMaximizeFunc     maximizeCallback;
	frameBufferSizeFunc    fbsizeCallback;
	windowContextScaleFunc scaleCallback;
	mouseButtonFunc        mouseButtonCallback;
	cursorPosFunc          cursorPosCallback;
	cursorEnterFunc        cursorEnterCallback;
	scrollFunc             scrollCallback;
	keyFunc                keyCallback;
	charFunc               charCallback;
	charModsFunc           charModsCallback;
	dropFunc               dropCallback;
#if defined(__APPLE__)
	NSWindow *             nsWindow;
	NSObject *             nsDelegate;
	NSView *               nsView;
	IntBool                nsScaleFramebuffer;
	int                    nsFrameBufferWidth;
	int                    nsFrameBufferHeight;
	float                  nsXScale;
	float                  nsYScale;
#elif defined(__linux__)
	Colormap               x11Colormap;
	Window                 x11Window;
	Window                 x11Parent;
	XIC                    x11IC;
	IntBool                x11OverrideRedirect;
	IntBool                x11Minimized;
	IntBool                x11Transparent;
	int                    x11XPos;
	int                    x11YPos;
	int                    x11WarpCursorPosX;
	int                    x11WarpCursorPosY;
	Time                   x11KeyPressTimes[256];
#elif defined(_WIN32)
	HWND                   win32Window;
	HICON                  win32BigIcon;
	HICON                  win32SmallIcon;
	IntBool                win32CursorTracked;
	IntBool                win32FrameAction;
	IntBool                win32Minimized;
	IntBool                win32Transparent;
	IntBool                win32ScaleToMonitor;
	WCHAR                  win32HighSurrogate;
#endif
};

// Monitor structure
struct plafMonitor {
	char              name[128];
	int               widthMM;
	int               heightMM;
	plafWindow*       window;
	plafVideoMode*    modes;
	int               modeCount;
	plafVideoMode     currentMode;
	plafGammaRamp     originalRamp;
	plafGammaRamp     currentRamp;
#if defined(__APPLE__)
	CGDirectDisplayID nsDisplayID;
	CGDisplayModeRef  nsPreviousMode;
	uint32_t          nsUnitNumber;
	NSScreen*         nsScreen;
#elif defined(__linux__)
	RROutput          x11Output;
	RRCrtc            x11Crtc;
	RRMode            x11OldMode;
	int               x11Index;
#elif defined(_WIN32)
	HMONITOR          win32Handle;
	WCHAR             win32AdapterName[32];
	WCHAR             win32DisplayName[32];
	char              win32PublicAdapterName[32];
	char              win32PublicDisplayName[32];
	IntBool           win32ModesPruned;
	IntBool           win32ModeChanged;
#endif
};

// Cursor structure
struct plafCursor {
	plafCursor* next;
#if defined(__APPLE__)
	NSCursor*   nsCursor;
#elif defined(__linux__)
	Cursor      x11Cursor;
#elif defined(_WIN32)
	HCURSOR		win32Cursor;
#endif
};

// Library global data
struct plafLib {
	IntBool                             initialized;
	char*                               clipboardString;
	plafFrameBufferCfg                  frameBufferCfg;
	plafWindowConfig                    windowCfg;
	plafCtxCfg                          contextCfg;
	int                                 desiredRefreshRate;
	plafCursor*                         cursorListHead;
	plafWindow*                         windowListHead;
	plafMonitor**                       monitors;
	int                                 monitorCount;
	plafError                           errorSlot;
	plafWindow*                         contextSlot;
	monitorFunc                         monitorCallback;
	short int                           scanCodes[KEY_LAST + 1];
	short int                           keyCodes[MAX_KEY_CODES];
#if defined(__APPLE__)
	CGEventSourceRef                    nsEventSource;
	id                                  nsDelegate;
	IntBool                             nsCursorHidden;
	id                                  nsKeyUpMonitor;
	CGPoint                             nsCascadePoint;
	CFBundleRef                         nsglFramework;
#elif defined(__linux__)
	Display*                            x11Display;
	int                                 x11Screen;
	Window                              x11Root;
	float                               x11ContentScaleX;
	float                               x11ContentScaleY;
	Window                              x11HelperWindowHandle;
	Cursor                              x11HiddenCursorHandle;
	XContext                            x11Context;
	XIM                                 x11IM;
	XErrorHandler                       x11ErrorHandler;
	int                                 x11ErrorCode;
	int                                 x11EmptyEventPipe[2];
	Atom                                x11NET_SUPPORTED;
	Atom                                x11NET_SUPPORTING_WM_CHECK;
	Atom                                x11WM_PROTOCOLS;
	Atom                                x11WM_STATE;
	Atom                                x11WM_DELETE_WINDOW;
	Atom                                x11NET_WM_NAME;
	Atom                                x11NET_WM_ICON_NAME;
	Atom                                x11NET_WM_ICON;
	Atom                                x11NET_WM_PID;
	Atom                                x11NET_WM_PING;
	Atom                                x11NET_WM_WINDOW_TYPE;
	Atom                                x11NET_WM_WINDOW_TYPE_NORMAL;
	Atom                                x11NET_WM_STATE;
	Atom                                x11NET_WM_STATE_ABOVE;
	Atom                                x11NET_WM_STATE_FULLSCREEN;
	Atom                                x11NET_WM_STATE_MAXIMIZED_VERT;
	Atom                                x11NET_WM_STATE_MAXIMIZED_HORZ;
	Atom                                x11NET_WM_STATE_DEMANDS_ATTENTION;
	Atom                                x11NET_WM_BYPASS_COMPOSITOR;
	Atom                                x11NET_WM_FULLSCREEN_MONITORS;
	Atom                                x11NET_WM_WINDOW_OPACITY;
	Atom                                x11NET_WM_CM_Sx;
	Atom                                x11NET_WORKAREA;
	Atom                                x11NET_CURRENT_DESKTOP;
	Atom                                x11NET_ACTIVE_WINDOW;
	Atom                                x11NET_FRAME_EXTENTS;
	Atom                                x11NET_REQUEST_FRAME_EXTENTS;
	Atom                                x11MOTIF_WM_HINTS;
	Atom                                x11DnDAware;
	Atom                                x11DnDEnter;
	Atom                                x11DnDPosition;
	Atom                                x11DnDStatus;
	Atom                                x11DnDActionCopy;
	Atom                                x11DnDDrop;
	Atom                                x11DnDFinished;
	Atom                                x11DnDSelection;
	Atom                                x11DnDTypeList;
	Atom                                x11Text_uri_list;
	Atom                                x11ClipTARGETS;
	Atom                                x11ClipMULTIPLE;
	Atom                                x11ClipINCR;
	Atom                                x11ClipCLIPBOARD;
	Atom                                x11ClipCLIPBOARD_MANAGER;
	Atom                                x11ClipSAVE_TARGETS;
	Atom                                x11ClipNULL_;
	Atom                                x11ClipUTF8_STRING;
	Atom                                x11ClipATOM_PAIR;
	Atom                                x11ClipSELECTION;
	void*                               xlibHandle;
	IntBool                             xlibUTF8;
	FN_XAllocSizeHints                  xlibAllocSizeHints;
	FN_XAllocWMHints                    xlibAllocWMHints;
	FN_XChangeProperty                  xlibChangeProperty;
	FN_XChangeWindowAttributes          xlibChangeWindowAttributes;
	FN_XCheckIfEvent                    xlibCheckIfEvent;
	FN_XCheckTypedWindowEvent           xlibCheckTypedWindowEvent;
	FN_XCloseDisplay                    xlibCloseDisplay;
	FN_XCloseIM                         xlibCloseIM;
	FN_XConvertSelection                xlibConvertSelection;
	FN_XCreateColormap                  xlibCreateColormap;
	FN_XCreateFontCursor                xlibCreateFontCursor;
	FN_XCreateIC                        xlibCreateIC;
	FN_XCreateRegion                    xlibCreateRegion;
	FN_XCreateWindow                    xlibCreateWindow;
	FN_XDefineCursor                    xlibDefineCursor;
	FN_XDeleteContext                   xlibDeleteContext;
	FN_XDeleteProperty                  xlibDeleteProperty;
	FN_XDestroyIC                       xlibDestroyIC;
	FN_XDestroyRegion                   xlibDestroyRegion;
	FN_XDestroyWindow                   xlibDestroyWindow;
	FN_XDisplayKeycodes                 xlibDisplayKeycodes;
	FN_XEventsQueued                    xlibEventsQueued;
	FN_XFilterEvent                     xlibFilterEvent;
	FN_XFindContext                     xlibFindContext;
	FN_XFlush                           xlibFlush;
	FN_XFree                            xlibFree;
	FN_XFreeColormap                    xlibFreeColormap;
	FN_XFreeCursor                      xlibFreeCursor;
	FN_XFreeEventData                   xlibFreeEventData;
	FN_XGetICValues                     xlibGetICValues;
	FN_XGetIMValues                     xlibGetIMValues;
	FN_XGetInputFocus                   xlibGetInputFocus;
	FN_XGetKeyboardMapping              xlibGetKeyboardMapping;
	FN_XGetScreenSaver                  xlibGetScreenSaver;
	FN_XGetSelectionOwner               xlibGetSelectionOwner;
	FN_XGetWMNormalHints                xlibGetWMNormalHints;
	FN_XGetWindowAttributes             xlibGetWindowAttributes;
	FN_XGetWindowProperty               xlibGetWindowProperty;
	FN_XMinimizeWindow                  xlibMinimizeWindow;
	FN_XInternAtom                      xlibInternAtom;
	FN_XLookupString                    xlibLookupString;
	FN_XMapRaised                       xlibMapRaised;
	FN_XMapWindow                       xlibMapWindow;
	FN_XMoveResizeWindow                xlibMoveResizeWindow;
	FN_XMoveWindow                      xlibMoveWindow;
	FN_XNextEvent                       xlibNextEvent;
	FN_XOpenIM                          xlibOpenIM;
	FN_XPeekEvent                       xlibPeekEvent;
	FN_XPending                         xlibPending;
	FN_XQueryExtension                  xlibQueryExtension;
	FN_XQueryPointer                    xlibQueryPointer;
	FN_XRaiseWindow                     xlibRaiseWindow;
	FN_XRegisterIMInstantiateCallback   xlibRegisterIMInstantiateCallback;
	FN_XResizeWindow                    xlibResizeWindow;
	FN_XResourceManagerString           xlibResourceManagerString;
	FN_XSaveContext                     xlibSaveContext;
	FN_XSelectInput                     xlibSelectInput;
	FN_XSendEvent                       xlibSendEvent;
	FN_XSetErrorHandler                 xlibSetErrorHandler;
	FN_XSetICFocus                      xlibSetICFocus;
	FN_XSetIMValues                     xlibSetIMValues;
	FN_XSetInputFocus                   xlibSetInputFocus;
	FN_XSetLocaleModifiers              xlibSetLocaleModifiers;
	FN_XSetScreenSaver                  xlibSetScreenSaver;
	FN_XSetSelectionOwner               xlibSetSelectionOwner;
	FN_XSetWMHints                      xlibSetWMHints;
	FN_XSetWMNormalHints                xlibSetWMNormalHints;
	FN_XSetWMProtocols                  xlibSetWMProtocols;
	FN_XSupportsLocale                  xlibSupportsLocale;
	FN_XSync                            xlibSync;
	FN_XTranslateCoordinates            xlibTranslateCoordinates;
	FN_XUndefineCursor                  xlibUndefineCursor;
	FN_XUnmapWindow                     xlibUnmapWindow;
	FN_XUnsetICFocus                    xlibUnsetICFocus;
	FN_XWarpPointer                     xlibWarpPointer;
	FN_XUnregisterIMInstantiateCallback xlibUnregisterIMInstantiateCallback;
	FN_Xutf8LookupString                xlibUTF8LookupString;
	FN_Xutf8SetWMProperties             xlibUTF8SetWMProperties;
	FN_XrmDestroyDatabase               xrmDestroyDatabase;
	FN_XrmGetResource                   xrmGetResource;
	FN_XrmGetStringDatabase             xrmGetStringDatabase;
	IntBool                             randrAvailable;
	void*                               randrHandle;
	int                                 randrEventBase;
	IntBool                             randrGammaBroken;
	IntBool                             randrMonitorBroken;
	FN_XRRAllocGamma                    randrAllocGamma;
	FN_XRRFreeCrtcInfo                  randrFreeCrtcInfo;
	FN_XRRFreeGamma                     randrFreeGamma;
	FN_XRRFreeOutputInfo                randrFreeOutputInfo;
	FN_XRRFreeScreenResources           randrFreeScreenResources;
	FN_XRRGetCrtcGamma                  randrGetCrtcGamma;
	FN_XRRGetCrtcGammaSize              randrGetCrtcGammaSize;
	FN_XRRGetCrtcInfo                   randrGetCrtcInfo;
	FN_XRRGetOutputInfo                 randrGetOutputInfo;
	FN_XRRGetOutputPrimary              randrGetOutputPrimary;
	FN_XRRGetScreenResourcesCurrent     randrGetScreenResourcesCurrent;
	FN_XRRQueryExtension                randrQueryExtension;
	FN_XRRQueryVersion                  randrQueryVersion;
	FN_XRRSelectInput                   randrSelectInput;
	FN_XRRSetCrtcConfig                 randrSetCrtcConfig;
	FN_XRRSetCrtcGamma                  randrSetCrtcGamma;
	FN_XRRUpdateConfiguration           randrUpdateConfiguration;
	IntBool                             xkbAvailable;
	IntBool                             xkbDetectable;
	int                                 xkbEventBase;
	unsigned int                        xkbGroup;
	FN_XkbFreeKeyboard                  xkbFreeKeyboard;
	FN_XkbFreeNames                     xkbFreeNames;
	FN_XkbGetMap                        xkbGetMap;
	FN_XkbGetNames                      xkbGetNames;
	FN_XkbGetState                      xkbGetState;
	FN_XkbQueryExtension                xkbQueryExtension;
	FN_XkbSelectEventDetails            xkbSelectEventDetails;
	FN_XkbSetDetectableAutoRepeat       xkbSetDetectableAutoRepeat;
	int                                 xsaverCount;
	int                                 xsaverTimeout;
	int                                 xsaverInterval;
	int                                 xsaverBlanking;
	int                                 xsaverExposure;
	int                                 xdndVersion;
	Window                              xdndSource;
	Atom                                xdndFormat;
	void*                               xcursorHandle;
	FN_XcursorImageCreate               xcursorImageCreate;
	FN_XcursorImageDestroy              xcursorImageDestroy;
	FN_XcursorImageLoadCursor           xcursorImageLoadCursor;
	FN_XcursorGetTheme                  xcursorGetTheme;
	FN_XcursorGetDefaultSize            xcursorGetDefaultSize;
	FN_XcursorLibraryLoadImage          xcursorLibraryLoadImage;
	IntBool                             xineramaAvailable;
	void*                               xineramaHandle;
	FN_XineramaIsActive                 xineramaIsActive;
	FN_XineramaQueryExtension           xineramaQueryExtension;
	FN_XineramaQueryScreens             xineramaQueryScreens;
	IntBool                             xvidmodeAvailable;
	void*                               xvidmodeHandle;
	FN_XF86VidModeQueryExtension        xvidmodeQueryExtension;
	FN_XF86VidModeGetGammaRamp          xvidmodeGetGammaRamp;
	FN_XF86VidModeSetGammaRamp          xvidmodeSetGammaRamp;
	FN_XF86VidModeGetGammaRampSize      xvidmodeGetGammaRampSize;
	IntBool                             xiAvailable;
	void*                               xiHandle;
	FN_XIQueryVersion                   xiQueryVersion;
	IntBool                             xrenderAvailable;
	void*                               xrenderHandle;
	FN_XRenderQueryExtension            xrenderQueryExtension;
	FN_XRenderQueryVersion              xrenderQueryVersion;
	FN_XRenderFindVisualFormat          xrenderFindVisualFormat;
	IntBool                             xshapeAvailable;
	void*                               xshapeHandle;
	FN_XShapeQueryExtension             xshapeQueryExtension;
	FN_XShapeQueryVersion               xshapeQueryVersion;
	FN_XShapeCombineRegion              xshapeShapeCombineRegion;
	FN_XShapeCombineMask                xshapeShapeCombineMask;
	int                                 glxErrorBase;
	void*                               glxHandle;
	FN_GLXGETFBCONFIGS                  glxGetFBConfigs;
	FN_GLXGETFBCONFIGATTRIB             glxGetFBConfigAttrib;
	FN_GLXGETCLIENTSTRING               glxGetClientString;
	FN_GLXQUERYEXTENSION                glxQueryExtension;
	FN_GLXQUERYVERSION                  glxQueryVersion;
	FN_GLXDESTROYCONTEXT                glxDestroyContext;
	FN_GLXMAKECURRENT                   glxMakeCurrent;
	FN_GLXSWAPBUFFERS                   glxSwapBuffers;
	FN_GLXQUERYEXTENSIONSSTRING         glxQueryExtensionsString;
	FN_GLXCREATENEWCONTEXT              glxCreateNewContext;
	FN_GLXGETVISUALFROMFBCONFIG         glxGetVisualFromFBConfig;
	FN_GLXCREATEWINDOW                  glxCreateWindow;
	FN_GLXDESTROYWINDOW                 glxDestroyWindow;
	FN_GLXGETPROCADDRESS                glxGetProcAddress;
	FN_GLXGETPROCADDRESS                glxGetProcAddressARB;
	FN_GLXSWAPINTERVALSGI               glxSwapIntervalSGI;
	FN_GLXSWAPINTERVALEXT               glxSwapIntervalEXT;
	FN_GLXCREATECONTEXTATTRIBSARB       glxCreateContextAttribsARB;
	IntBool                             glxSGI_swap_control;
	IntBool                             glxEXT_swap_control;
	IntBool                             glxARB_multisample;
	IntBool                             glxARB_framebuffer_sRGB;
	IntBool                             glxEXT_framebuffer_sRGB;
	IntBool                             glxARB_create_context;
	IntBool                             glxARB_create_context_profile;
	IntBool                             glxARB_create_context_robustness;
	IntBool                             glxARB_create_context_no_error;
	IntBool                             glxARB_context_flush_control;
#elif defined(_WIN32)
	HINSTANCE                           win32Instance;
	HWND                                win32HelperWindowHandle;
	ATOM                                win32HelperWindowClass;
	ATOM                                win32MainWindowClass;
	HDEVNOTIFY                          win32DeviceNotificationHandle;
	int                                 win32AcquiredMonitorCount;
	UINT                                win32MouseTrailSize;
	HCURSOR                             win32BlankCursor;
	HINSTANCE                           win32User32Instance;
	FN_EnableNonClientDpiScaling        win32User32EnableNonClientDpiScaling_;
	FN_SetProcessDpiAwarenessContext    win32User32SetProcessDpiAwarenessContext_;
	FN_GetDpiForWindow                  win32User32GetDpiForWindow_;
	FN_AdjustWindowRectExForDpi         win32User32AdjustWindowRectExForDpi_;
	FN_GetSystemMetricsForDpi           win32User32GetSystemMetricsForDpi_;
	HINSTANCE                           win32DwmInstance;
	FN_DwmIsCompositionEnabled          win32DwmIsCompositionEnabled;
	FN_DwmFlush                         win32DwmFlush;
	FN_DwmEnableBlurBehindWindow        win32DwmEnableBlurBehindWindow;
	FN_DwmGetColorizationColor          win32DwmGetColorizationColor;
	HINSTANCE                           win32ShCoreInstance;
	FN_SetProcessDpiAwareness           win32ShCoreSetProcessDpiAwareness_;
	FN_GetDpiForMonitor                 win32ShCoreGetDpiForMonitor_;
	HINSTANCE                           win32NTInstance;
	FN_RtlVerifyVersionInfo             win32NTRtlVerifyVersionInfo_;
	HINSTANCE                           wglInstance;
	FN_wglCreateContext                 wglCreateContext;
	FN_wglDeleteContext                 wglDeleteContext;
	FN_wglGetProcAddress                wglGetProcAddress;
	FN_wglGetCurrentDC                  wglGetCurrentDC;
	FN_wglGetCurrentContext             wglGetCurrentContext;
	FN_wglMakeCurrent                   wglMakeCurrent;
	FN_wglShareLists                    wglShareLists;
	FN_WGLSWAPINTERVALEXT               wglSwapIntervalEXT;
	FN_WGLGETPIXELFORMATATTRIBIVARB     wglGetPixelFormatAttribivARB;
	FN_WGLGETEXTENSIONSSTRINGEXT        wglGetExtensionsStringEXT;
	FN_WGLGETEXTENSIONSSTRINGARB        wglGetExtensionsStringARB;
	FN_WGLCREATECONTEXTATTRIBSARB       wglCreateContextAttribsARB;
	IntBool                             wglEXT_swap_control;
	IntBool                             wglARB_multisample;
	IntBool                             wglARB_framebuffer_sRGB;
	IntBool                             wglEXT_framebuffer_sRGB;
	IntBool                             wglARB_pixel_format;
	IntBool                             wglARB_create_context;
	IntBool                             wglARB_create_context_profile;
	IntBool                             wglARB_create_context_robustness;
	IntBool                             wglARB_create_context_no_error;
	IntBool                             wglARB_context_flush_control;
#endif
};

// Global state
extern plafLib _plaf;

/*************************************************************************
 * Public API functions
 *************************************************************************/

// Setup & teardown
plafError* plafInit(void);
void plafTerminate(void);
errorFunc plafSetErrorCallback(errorFunc callback);

// Monitors
plafMonitor** plafGetMonitors(int* count);
plafMonitor* plafGetPrimaryMonitor(void);
void plafGetMonitorPos(plafMonitor* monitor, int* xpos, int* ypos);
void plafGetMonitorWorkarea(plafMonitor* monitor, int* xpos, int* ypos, int* width, int* height);
void plafGetMonitorPhysicalSize(plafMonitor* monitor, int* widthMM, int* heightMM);
void plafGetMonitorContentScale(plafMonitor* monitor, float* xscale, float* yscale);
const char* plafGetMonitorName(plafMonitor* monitor);
monitorFunc plafSetMonitorCallback(monitorFunc callback);
const plafVideoMode* plafGetVideoModes(plafMonitor* monitor, int* count);
const plafVideoMode* plafGetVideoMode(plafMonitor* monitor);
void plafSetGamma(plafMonitor* monitor, float gamma);
const plafGammaRamp* plafGetGammaRamp(plafMonitor* monitor);
void plafSetGammaRamp(plafMonitor* monitor, const plafGammaRamp* ramp);

// Windows
void plafDefaultWindowHints(void);
void plafWindowHint(int hint, int value);
plafError* plafCreateWindow(int width, int height, const char* title, plafMonitor* monitor, plafWindow* share, plafWindow** outWindow);
void* plafGetNativeWindow(plafWindow* window);
int plafWindowShouldClose(plafWindow* window);
void plafSetWindowShouldClose(plafWindow* window, int value);
const char* plafGetWindowTitle(plafWindow* window);
void plafSetWindowTitle(plafWindow* window, const char* title);
void plafSetWindowIcon(plafWindow* window, int count, const plafImageData* images);
void plafGetWindowPos(plafWindow* window, int* xpos, int* ypos);
void plafSetWindowPos(plafWindow* window, int xpos, int ypos);
void plafGetWindowSize(plafWindow* window, int* width, int* height);
void plafSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight);
void plafSetWindowSize(plafWindow* window, int width, int height);
void plafGetFramebufferSize(plafWindow* window, int* width, int* height);
void plafGetWindowFrameSize(plafWindow* window, int* left, int* top, int* right, int* bottom);
void plafGetWindowContentScale(plafWindow* window, float* xscale, float* yscale);
float plafGetWindowOpacity(plafWindow* window);
void plafSetWindowOpacity(plafWindow* window, float opacity);
void plafMinimizeWindow(plafWindow* window);
void plafMaximizeWindow(plafWindow* window);
void plafRestoreWindow(plafWindow* window);
void plafShowWindow(plafWindow* window);
void plafHideWindow(plafWindow* window);
void plafFocusWindow(plafWindow* window);
void plafRequestWindowAttention(plafWindow* window);
plafMonitor* plafGetWindowMonitor(plafWindow* window);
void plafSetWindowMonitor(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate);
int plafGetWindowAttrib(plafWindow* window, int attrib);
void plafSetWindowAttrib(plafWindow* window, int attrib, int value);
void plafHideCursor(plafWindow* window);
void plafShowCursor(plafWindow* window);
int plafGetKey(plafWindow* window, int key);
int plafGetMouseButton(plafWindow* window, int button);
void plafGetCursorPos(plafWindow* window, double* xpos, double* ypos);
void plafSetCursorPos(plafWindow* window, double xpos, double ypos);
void plafSetCursor(plafWindow* window, plafCursor* cursor);
void plafSwapBuffers(plafWindow* window);
plafError* plafMakeContextCurrent(plafWindow* window);
windowPosFunc plafSetWindowPosCallback(plafWindow* window, windowPosFunc callback);
windowSizeFunc plafSetWindowSizeCallback(plafWindow* window, windowSizeFunc callback);
windowCloseFunc plafSetWindowCloseCallback(plafWindow* window, windowCloseFunc callback);
windowRefreshFunc plafSetWindowRefreshCallback(plafWindow* window, windowRefreshFunc callback);
windowFocusFunc plafSetWindowFocusCallback(plafWindow* window, windowFocusFunc callback);
windowMinimizeFunc plafSetWindowMinimizeCallback(plafWindow* window, windowMinimizeFunc callback);
windowMaximizeFunc plafSetWindowMaximizeCallback(plafWindow* window, windowMaximizeFunc callback);
frameBufferSizeFunc plafSetFramebufferSizeCallback(plafWindow* window, frameBufferSizeFunc callback);
windowContextScaleFunc plafSetWindowContentScaleCallback(plafWindow* window, windowContextScaleFunc callback);
keyFunc plafSetKeyCallback(plafWindow* window, keyFunc callback);
charFunc plafSetCharCallback(plafWindow* window, charFunc callback);
charModsFunc plafSetCharModsCallback(plafWindow* window, charModsFunc callback);
mouseButtonFunc plafSetMouseButtonCallback(plafWindow* window, mouseButtonFunc callback);
cursorPosFunc plafSetCursorPosCallback(plafWindow* window, cursorPosFunc callback);
cursorEnterFunc plafSetCursorEnterCallback(plafWindow* window, cursorEnterFunc callback);
scrollFunc plafSetScrollCallback(plafWindow* window, scrollFunc callback);
dropFunc plafSetDropCallback(plafWindow* window, dropFunc callback);
void plafDestroyWindow(plafWindow* window);
plafWindow* plafGetCurrentContext(void);
int plafGetKeyScancode(int key);

// Events
void plafPollEvents(void);
void plafWaitEvents(void);
void plafWaitEventsTimeout(double timeout);
void plafPostEmptyEvent(void);

// Cursors
plafCursor* plafCreateCursor(const plafImageData* image, int xhot, int yhot);
plafCursor* plafCreateStandardCursor(int shape);
void plafDestroyCursor(plafCursor* cursor);

// Clipboard
const char* plafGetClipboardString(void);
void plafSetClipboardString(const char* string);

// OpenGL
void plafSwapInterval(int interval);
int plafExtensionSupported(const char* extension);
glFunc plafGetProcAddress(const char* procname);

// --------- Internal API below ---------

// Setup & teardown
plafError* _plafInit(void);
void _plafTerminate(void);
void* _plafLoadModule(const char* path);
void _plafFreeModule(void* module);
moduleFunc _plafGetModuleSymbol(void* module, const char* name);
#if defined(__GNUC__)
void _plafInputError(const char* format, ...) __attribute__((format(printf, 1, 2)));
plafError* _plafNewError(const char* format, ...) __attribute__((format(printf, 1, 2)));
#else
void _plafInputError(const char* format, ...);
plafError* _plafNewError(const char* format, ...);
#endif

// Monitors
plafMonitor* _plafAllocMonitor(const char* name, int widthMM, int heightMM);
void _plafFreeMonitor(plafMonitor* monitor);
void _plafAllocGammaArrays(plafGammaRamp* ramp, unsigned int size);
void _plafFreeGammaArrays(plafGammaRamp* ramp);
IntBool _plafGetGammaRamp(plafMonitor* monitor, plafGammaRamp* ramp);
void _plafSetGammaRamp(plafMonitor* monitor, const plafGammaRamp* ramp);
plafVideoMode* _plafGetVideoModes(plafMonitor* monitor, int* count);
IntBool _plafGetVideoMode(plafMonitor* monitor, plafVideoMode* mode);
void _plafSetVideoMode(plafMonitor* monitor, const plafVideoMode* desired);
const plafVideoMode* _plafChooseVideoMode(plafMonitor* monitor, const plafVideoMode* desired);
int _plafCompareVideoModes(const plafVideoMode* first, const plafVideoMode* second);
void _plafSplitBPP(int bpp, int* red, int* green, int* blue);
void _plafMonitorNotify(plafMonitor* monitor, int action, int placement);
void _plafPollMonitors(void);
void _plafRestoreVideoMode(plafMonitor* monitor);
#if defined(_WIN32)
void _plafGetHMONITORContentScale(HMONITOR handle, float* xscale, float* yscale);
#endif

// Windows
void _plafInputWindowFocus(plafWindow* window, IntBool focused);
void _plafInputWindowPos(plafWindow* window, int xpos, int ypos);
void _plafInputWindowSize(plafWindow* window, int width, int height);
void _plafInputFramebufferSize(plafWindow* window, int width, int height);
void _plafInputWindowContentScale(plafWindow* window, float xscale, float yscale);
void _plafInputWindowMinimize(plafWindow* window, IntBool minimized);
void _plafInputWindowMaximize(plafWindow* window, IntBool maximized);
void _plafInputWindowDamage(plafWindow* window);
void _plafInputWindowCloseRequest(plafWindow* window);
void _plafInputKey(plafWindow* window, int key, int scancode, int action, int mods);
void _plafInputChar(plafWindow* window, uint32_t codepoint, int mods, IntBool plain);
void _plafInputScroll(plafWindow* window, double xoffset, double yoffset);
void _plafInputMouseClick(plafWindow* window, int button, int action, int mods);
void _plafInputCursorPos(plafWindow* window, double xpos, double ypos);
void _plafInputCursorEnter(plafWindow* window, IntBool entered);
void _plafInputDrop(plafWindow* window, int count, const char** names);
plafError* _plafCreateWindow(plafWindow* window, const plafWindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig);
void _plafSetWindowTitle(plafWindow* window, const char* title);
void _plafSetWindowIcon(plafWindow* window, int count, const plafImageData* images);
void _plafGetWindowPos(plafWindow* window, int* xpos, int* ypos);
void _plafSetWindowPos(plafWindow* window, int xpos, int ypos);
void _plafGetWindowSize(plafWindow* window, int* width, int* height);
void _plafSetWindowSize(plafWindow* window, int width, int height);
void _plafSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight);
void _plafGetFramebufferSize(plafWindow* window, int* width, int* height);
void _plafGetWindowFrameSize(plafWindow* window, int* left, int* top, int* right, int* bottom);
void _plafGetWindowContentScale(plafWindow* window, float* xscale, float* yscale);
void _plafMaximizeWindow(plafWindow* window);
void _plafShowWindow(plafWindow* window);
void _plafHideWindow(plafWindow* window);
void _plafSetWindowMonitor(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate);
IntBool _plafWindowFocused(plafWindow* window);
IntBool _plafWindowMinimized(plafWindow* window);
IntBool _plafWindowVisible(plafWindow* window);
IntBool _plafWindowMaximized(plafWindow* window);
IntBool _plafWindowHovered(plafWindow* window);
IntBool _plafFramebufferTransparent(plafWindow* window);
void _plafSetWindowResizable(plafWindow* window, IntBool enabled);
void _plafSetWindowDecorated(plafWindow* window, IntBool enabled);
void _plafSetWindowFloating(plafWindow* window, IntBool enabled);
void _plafSetWindowOpacity(plafWindow* window, float opacity);
void _plafSetWindowMousePassthrough(plafWindow* window, IntBool enabled);
void _plafUpdateCursor(plafWindow* window);
plafError* _plafRefreshContextAttribs(plafWindow* window, const plafCtxCfg* ctxconfig);
void _plafSetCursor(plafWindow* window);
void _plafSetCursorPos(plafWindow* window, double xpos, double ypos);
#if defined(__APPLE__) || defined(_WIN32)
IntBool _plafCursorInContentArea(plafWindow* window);
#endif
void _plafUpdateCursorImage(plafWindow* window);
void _plafDestroyWindow(plafWindow* window);
#if defined(__linux__)
void _plafCreateInputContext(plafWindow* window);
unsigned long _plafGetWindowProperty(Window window, Atom property, Atom type, unsigned char** value);
IntBool _plafIsVisualTransparent(Visual* visual);
plafError* _plafChooseVisual(const plafWindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig, Visual** visual, int* depth);
#endif

// Events
void _plafWaitEventsTimeout(double timeout);
#if defined(__linux__)
IntBool _plafWaitForX11Event(double timeout);
#endif

// Cursors
void _plafDestroyCursor(plafCursor* cursor);
IntBool _plafCreateStandardCursor(plafCursor* cursor, int shape);
IntBool _plafCreateCursor(plafCursor* cursor, const plafImageData* image, int xhot, int yhot);
#if defined(__linux__)
Cursor _plafCreateNativeCursorX11(const plafImageData* image, int xhot, int yhot);
#endif

// Clipboard
#if defined(__linux__)
void _plafPushSelectionToManager(void);
#endif

// OpenGL
plafError* _plafInitOpenGL(void);
plafError* _plafCreateOpenGLContext(plafWindow* window, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig);
IntBool _plafStringInExtensionString(const char* string, const char* extensions);
const plafFrameBufferCfg* _plafChooseFBConfig(const plafFrameBufferCfg* desired, const plafFrameBufferCfg* alternatives, unsigned int count);
plafError* plafCheckContextConfig(const plafCtxCfg* ctxconfig);
void _plafTerminateOpenGL(void);

// Utility
size_t _plafEncodeUTF8(char* s, uint32_t codepoint);
char** _plafParseUriList(char* text, int* count);
char* _plaf_strdup(const char* src);
int _plaf_min(int a, int b);
void* _plaf_calloc(size_t count, size_t size);
void* _plaf_realloc(void* pointer, size_t size);
void _plaf_free(void* pointer);
#if defined(__APPLE__)
float _plafTransformYCocoa(float y);
#elif defined(__linux__)
void _plafGrabErrorHandler(void);
void _plafReleaseErrorHandler(void);
#elif defined(_WIN32)
char* _plafCreateUTF8FromWideString(const WCHAR* src);
BOOL _plafIsWindows10BuildOrGreater(WORD build);
#endif

#ifdef __cplusplus
}
#endif

#endif
