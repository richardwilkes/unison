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
	typedef int (* FN_XGetErrorText)(Display*,int,char*,int);
	typedef char* (* FN_XGetICValues)(XIC,...);
	typedef char* (* FN_XGetIMValues)(XIM,...);
	typedef int (* FN_XGetInputFocus)(Display*,Window*,int*);
	typedef KeySym* (* FN_XGetKeyboardMapping)(Display*,KeyCode,int,int*);
	typedef int (* FN_XGetScreenSaver)(Display*,int*,int*,int*,int*);
	typedef Window (* FN_XGetSelectionOwner)(Display*,Atom);
	typedef Status (* FN_XGetWMNormalHints)(Display*,Window,XSizeHints*,long*);
	typedef Status (* FN_XGetWindowAttributes)(Display*,Window,XWindowAttributes*);
	typedef int (* FN_XGetWindowProperty)(Display*,Window,Atom,long,long,Bool,Atom,Atom*,int*,unsigned long*,unsigned long*,unsigned char**);
	typedef Status (* FN_XIconifyWindow)(Display*,Window,int);
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
	// Require Windows 7 or later
	#if WINVER < 0x0601
		#undef WINVER
		#define WINVER 0x0601
	#endif
	#if _WIN32_WINNT < 0x0601
		#undef _WIN32_WINNT
		#define _WIN32_WINNT 0x0601
	#endif

	#include <wctype.h>
	#include <windows.h>
	#include <dwmapi.h>
	#include <dinput.h>
	#include <dbt.h>

	#ifndef WM_COPYGLOBALDATA
		#define WM_COPYGLOBALDATA 0x0049
	#endif
	#ifndef WM_DPICHANGED
		#define WM_DPICHANGED 0x02E0
	#endif
	#ifndef EDS_ROTATEDMODE
		#define EDS_ROTATEDMODE 0x00000004
	#endif
	#ifndef _WIN32_WINNT_WINBLUE
		#define _WIN32_WINNT_WINBLUE 0x0603
	#endif
	#ifndef _WIN32_WINNT_WIN8
		#define _WIN32_WINNT_WIN8 0x0602
	#endif
	#ifndef WM_GETDPISCALEDSIZE
		#define WM_GETDPISCALEDSIZE 0x02e4
	#endif
	#ifndef USER_DEFAULT_SCREEN_DPI
		#define USER_DEFAULT_SCREEN_DPI 96
	#endif
	#if !defined(WINGDIAPI)
		#define WINGDIAPI __declspec(dllimport)
		#define GLFW_WINGDIAPI_DEFINED
	#endif
	#if !defined(CALLBACK)
		#define CALLBACK __stdcall
		#define GLFW_CALLBACK_DEFINED
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
	#ifndef DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
		#define DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 ((HANDLE) -4)
	#endif
	// Windows 10 Anniversary Update
	#define IsWindows10Version1607OrGreater() IsWindows10BuildOrGreater(14393)
	// Windows 10 Creators Update
	#define IsWindows10Version1703OrGreater() IsWindows10BuildOrGreater(15063)
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
	typedef HRESULT(WINAPI * FN_DwmEnableBlurBehindWindow)(HWND,const DWM_BLURBEHIND*);
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
#define WINDOW_ATTR_ICONIFIED                      0x00020002
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

// Input modes
#define INPUT_MODE_CURSOR                  0x00033001
#define INPUT_MODE_STICKY_KEYS             0x00033002
#define INPUT_MODE_STICKY_MOUSE_BUTTONS    0x00033003
#define INPUT_MODE_LOCK_KEY_MODS           0x00033004
#define INPUT_MODE_RAW_MOUSE_MOTION        0x00033005
#define INPUT_MODE_UNLIMITED_MOUSE_BUTTONS 0x00033006

// Cursor mode values
#define CURSOR_NORMAL   0x00034001
#define CURSOR_HIDDEN   0x00034002

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



// Error codes
/*! @brief GLFW has not been initialized.
 *
 *  This occurs if a GLFW function was called that must not be called unless the
 *  library is [initialized](@ref intro_init).
 *
 *  @analysis Application programmer error.  Initialize GLFW before calling any
 *  function that requires initialization.
 */
#define ERR_NOT_INITIALIZED        0x00010001
/*! @brief No context is current for this thread.
 *
 *  This occurs if a GLFW function was called that needs and operates on the
 *  current OpenGL or OpenGL ES context but no context is current on the calling
 *  thread.  One such function is @ref glfwSwapInterval.
 *
 *  @analysis Application programmer error.  Ensure a context is current before
 *  calling functions that require a current context.
 */
#define ERR_NO_CURRENT_CONTEXT     0x00010002
/*! @brief One of the arguments to the function was an invalid enum value.
 *
 *  One of the arguments to the function was an invalid enum value, for example
 *  requesting @ref WINDOW_HINT_RED_BITS with @ref glfwGetWindowAttrib.
 *
 *  @analysis Application programmer error.  Fix the offending call.
 */
#define ERR_INVALID_ENUM           0x00010003
/*! @brief One of the arguments to the function was an invalid value.
 *
 *  One of the arguments to the function was an invalid value, for example
 *  requesting a non-existent OpenGL or OpenGL ES version like 2.7.
 *
 *  Requesting a valid but unavailable OpenGL or OpenGL ES version will instead
 *  result in a @ref ERR_VERSION_UNAVAILABLE error.
 *
 *  @analysis Application programmer error.  Fix the offending call.
 */
#define ERR_INVALID_VALUE          0x00010004
/*! @brief A memory allocation failed.
 *
 *  A memory allocation failed.
 *
 *  @analysis A bug in GLFW or the underlying operating system.  Report the bug
 *  to our [issue tracker](https://github.com/glfw/glfw/issues).
 */
#define ERR_OUT_OF_MEMORY          0x00010005
/*! @brief GLFW could not find support for the requested API on the system.
 *
 *  GLFW could not find support for the requested API on the system.
 *
 *  @analysis The installed graphics driver does not support the requested
 *  API, or does not support it via the chosen context creation API.
 *  Below are a few examples.
 *
 *  @par
 *  Some pre-installed Windows graphics drivers do not support OpenGL.  AMD only
 *  supports OpenGL ES via EGL, while Nvidia and Intel only support it via
 *  a WGL or GLX extension.  macOS does not provide OpenGL ES at all.  The Mesa
 *  EGL, OpenGL and OpenGL ES libraries do not interface with the Nvidia binary
 *  driver.
 */
#define ERR_API_UNAVAILABLE        0x00010006
/*! @brief The requested OpenGL or OpenGL ES version is not available.
 *
 *  The requested OpenGL or OpenGL ES version (including any requested context
 *  or framebuffer hints) is not available on this machine.
 *
 *  @analysis The machine does not support your requirements.  If your
 *  application is sufficiently flexible, downgrade your requirements and try
 *  again.  Otherwise, inform the user that their machine does not match your
 *  requirements.
 *
 *  @par
 *  Future invalid OpenGL and OpenGL ES versions, for example OpenGL 4.8 if 5.0
 *  comes out before the 4.x series gets that far, also fail with this error and
 *  not @ref ERR_INVALID_VALUE, because GLFW cannot know what future versions
 *  will exist.
 */
#define ERR_VERSION_UNAVAILABLE    0x00010007
/*! @brief A platform-specific error occurred that does not match any of the
 *  more specific categories.
 *
 *  A platform-specific error occurred that does not match any of the more
 *  specific categories.
 *
 *  @analysis A bug or configuration error in GLFW, the underlying operating
 *  system or its drivers, or a lack of required resources.  Report the issue to
 *  our [issue tracker](https://github.com/glfw/glfw/issues).
 */
#define ERR_PLATFORM_ERROR         0x00010008
/*! @brief The requested format is not supported or available.
 *
 *  If emitted during window creation, the requested pixel format is not
 *  supported.
 *
 *  If emitted when querying the clipboard, the contents of the clipboard could
 *  not be converted to the requested format.
 *
 *  @analysis If emitted during window creation, one or more
 *  [hard constraints](@ref window_hints_hard) did not match any of the
 *  available pixel formats.  If your application is sufficiently flexible,
 *  downgrade your requirements and try again.  Otherwise, inform the user that
 *  their machine does not match your requirements.
 *
 *  @par
 *  If emitted when querying the clipboard, ignore the error or report it to
 *  the user, as appropriate.
 */
#define ERR_FORMAT_UNAVAILABLE     0x00010009
/*! @brief The specified window does not have an OpenGL or OpenGL ES context.
 *
 *  A window that does not have an OpenGL or OpenGL ES context was passed to
 *  a function that requires it to have one.
 *
 *  @analysis Application programmer error.  Fix the offending call.
 */
#define ERR_NO_WINDOW_CONTEXT      0x0001000A
/*! @brief The specified cursor shape is not available.
 *
 *  The specified standard cursor shape is not available, either because the
 *  current platform cursor theme does not provide it or because it is not
 *  available on the platform.
 *
 *  @analysis Platform or system settings limitation.  Pick another
 *  [standard cursor shape](@ref shapes) or create a
 *  [custom cursor](@ref cursor_custom).
 */
#define GLFW_CURSOR_UNAVAILABLE     0x0001000B
/*! @brief The requested feature is not provided by the platform.
 *
 *  The requested feature is not provided by the platform, so GLFW is unable to
 *  implement it.  The documentation for each function notes if it could emit
 *  this error.
 *
 *  @analysis Platform or platform version limitation.  The error can be ignored
 *  unless the feature is critical to the application.
 *
 *  @par
 *  A function call that emits this error has no effect other than the error and
 *  updating any existing out parameters.
 */
#define ERR_FEATURE_UNAVAILABLE    0x0001000C
/*! @brief The requested feature is not implemented for the platform.
 *
 *  The requested feature has not yet been implemented in GLFW for this platform.
 *
 *  @analysis An incomplete implementation of GLFW for this platform, hopefully
 *  fixed in a future release.  The error can be ignored unless the feature is
 *  critical to the application.
 *
 *  @par
 *  A function call that emits this error has no effect other than the error and
 *  updating any existing out parameters.
 */
#define ERR_FEATURE_UNIMPLEMENTED  0x0001000D
/*! @brief Platform unavailable or no matching platform was found.
 *
 *  If emitted during initialization, no matching platform was found.
 *
 *  If emitted by a native access function, GLFW was initialized for a different platform
 *  than the function is for.
 *
 *  @analysis Failure to detect any platform usually only happens on non-macOS Unix
 *  systems, either when no window system is running or the program was run from
 *  a terminal that does not have the necessary environment variables.  Fall back to
 *  a different platform if possible or notify the user that no usable platform was
 *  detected.
 *
 *  Failure to detect a specific platform may have the same cause as above or be because
 *  support for that platform was not compiled in.
 */
#define ERR_PLATFORM_UNAVAILABLE   0x0001000E

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
typedef void (*errorFunc)(int error_code, const char* description);
typedef void (*frameBufferSizeFunc)(plafWindow* window, int width, int height);
typedef void (*glFunc)(void);
typedef void (*keyFunc)(plafWindow* window, int key, int scancode, int action, int mods);
typedef void (*monitorFunc)(plafMonitor* monitor, int event);
typedef void (*mouseButtonFunc)(plafWindow* window, int button, int action, int mods);
typedef void (*scrollFunc)(plafWindow* window, double xoffset, double yoffset);
typedef void (*windowCloseFunc)(plafWindow* window);
typedef void (*windowContextScaleFunc)(plafWindow* window, float xscale, float yscale);
typedef void (*windowFocusFunc)(plafWindow* window, int focused);
typedef void (*windowIconifyFunc)(plafWindow* window, int iconified);
typedef void (*windowMaximizeFunc)(plafWindow* window, int maximized);
typedef void (*windowPosFunc)(plafWindow* window, int xpos, int ypos); // coordinates are content area upper-left
typedef void (*windowRefreshFunc)(plafWindow* window);
typedef void (*windowSizeFunc)(plafWindow* window, int width, int height);

// An error response
typedef struct ErrorResponse {
	int  code;
	char desc[ERROR_MSG_SIZE];
} ErrorResponse;

// A single video mode
typedef struct VideoMode {
	int width;
	int height;
	int redBits;
	int greenBits;
	int blueBits;
	int refreshRate;
} VideoMode;

// Gamma ramp for a monitor
typedef struct GammaRamp {
	unsigned short* red;
	unsigned short* green;
	unsigned short* blue;
	unsigned int    size;
} GammaRamp;

typedef struct ImageData {
	int            width;
	int            height;
	unsigned char* pixels;
} ImageData;

typedef struct WindowConfig {
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
} WindowConfig;


/*************************************************************************
 * Global definition cleanup
 *************************************************************************/

/* ------------------- BEGIN SYSTEM/COMPILER SPECIFIC -------------------- */

#ifdef GLFW_WINGDIAPI_DEFINED
 #undef WINGDIAPI
 #undef GLFW_WINGDIAPI_DEFINED
#endif

#ifdef GLFW_CALLBACK_DEFINED
 #undef CALLBACK
 #undef GLFW_CALLBACK_DEFINED
#endif

/* Some OpenGL related headers need GLAPIENTRY, but it is unconditionally
 * defined by some gl.h variants (OpenBSD) so define it after if needed.
 */
#ifndef GLAPIENTRY
 #define GLAPIENTRY APIENTRY
 #define GLFW_GLAPIENTRY_DEFINED
#endif

/* -------------------- END SYSTEM/COMPILER SPECIFIC --------------------- */


/* ------------------------- Internal ----------------------- */

#define MONITOR_INSERT_FIRST      0
#define MONITOR_INSERT_LAST       1

typedef void (*moduleFunc)(void);

typedef struct plafCtxCfg         plafCtxCfg;
typedef struct plafFrameBufferCfg plafFrameBufferCfg;
typedef struct plafCtx            plafCtx;
typedef struct _GLFWplatform      _GLFWplatform;
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
//
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
//
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
	void (*makeCurrent)(plafWindow*);
	void (*swapBuffers)(plafWindow*);
	void (*swapInterval)(int);
	int (*extensionSupported)(const char*);
	glFunc (*getProcAddress)(const char*);
	void (*destroy)(plafWindow*);
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
	VideoMode              videoMode;
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
	IntBool                stickyKeys;
	IntBool                stickyMouseButtons;
	IntBool                lockKeyMods;
	IntBool                disableMouseButtonLimit;
	int                    cursorMode;
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
	windowIconifyFunc      iconifyCallback;
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
	IntBool                x11Iconified;
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
	IntBool                win32Iconified;
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
	VideoMode*        modes;
	int               modeCount;
	VideoMode         currentMode;
	GammaRamp         originalRamp;
	GammaRamp         currentRamp;
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

// Platform API structure
struct _GLFWplatform {
	IntBool (*createWindow)(plafWindow*,const WindowConfig*,const plafCtxCfg*,const plafFrameBufferCfg*);
	void (*destroyWindow)(plafWindow*);
	void (*setWindowTitle)(plafWindow*,const char*);
	void (*setWindowIcon)(plafWindow*,int,const ImageData*);
	void (*getWindowPos)(plafWindow*,int*,int*);
	void (*setWindowPos)(plafWindow*,int,int);
	void (*getWindowSize)(plafWindow*,int*,int*);
	void (*setWindowSize)(plafWindow*,int,int);
	void (*setWindowSizeLimits)(plafWindow*,int,int,int,int);
	void (*setWindowAspectRatio)(plafWindow*,int,int);
	void (*getFramebufferSize)(plafWindow*,int*,int*);
	void (*getWindowFrameSize)(plafWindow*,int*,int*,int*,int*);
	void (*getWindowContentScale)(plafWindow*,float*,float*);
	void (*iconifyWindow)(plafWindow*);
	void (*restoreWindow)(plafWindow*);
	void (*maximizeWindow)(plafWindow*);
	void (*showWindow)(plafWindow*);
	void (*hideWindow)(plafWindow*);
	void (*requestWindowAttention)(plafWindow*);
	void (*focusWindow)(plafWindow*);
	void (*setWindowMonitor)(plafWindow*,plafMonitor*,int,int,int,int,int);
	IntBool (*windowFocused)(plafWindow*);
	IntBool (*windowIconified)(plafWindow*);
	IntBool (*windowVisible)(plafWindow*);
	IntBool (*windowMaximized)(plafWindow*);
	IntBool (*windowHovered)(plafWindow*);
	IntBool (*framebufferTransparent)(plafWindow*);
	float (*getWindowOpacity)(plafWindow*);
	void (*setWindowResizable)(plafWindow*,IntBool);
	void (*setWindowDecorated)(plafWindow*,IntBool);
	void (*setWindowFloating)(plafWindow*,IntBool);
	void (*setWindowOpacity)(plafWindow*,float);
	void (*setWindowMousePassthrough)(plafWindow*,IntBool);
	void (*pollEvents)(void);
	void (*waitEvents)(void);
	void (*waitEventsTimeout)(double);
	void (*postEmptyEvent)(void);
};

// Library global data
struct plafLib
{
	IntBool                             initialized;
	_GLFWplatform                       platform;
	char*                               clipboardString;
	plafFrameBufferCfg                  frameBufferCfg;
	WindowConfig                        windowCfg;
	plafCtxCfg                          contextCfg;
	int                                 desiredRefreshRate;
	plafCursor*                         cursorListHead;
	plafWindow*                         windowListHead;
	plafMonitor**                       monitors;
	int                                 monitorCount;
	ErrorResponse                       errorSlot;
	plafWindow*                         contextSlot;
	monitorFunc                         monitorCallback;
	short int                           scanCodes[KEY_LAST + 1];
#if defined(__APPLE__)
	CGEventSourceRef                    nsEventSource;
	id                                  nsDelegate;
	IntBool                             nsCursorHidden;
	id                                  nsKeyUpMonitor;
	short int                           nsKeycodes[256];
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
	short int                           x11Keycodes[256];
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
	FN_XGetErrorText                    xlibGetErrorText;
	FN_XGetICValues                     xlibGetICValues;
	FN_XGetIMValues                     xlibGetIMValues;
	FN_XGetInputFocus                   xlibGetInputFocus;
	FN_XGetKeyboardMapping              xlibGetKeyboardMapping;
	FN_XGetScreenSaver                  xlibGetScreenSaver;
	FN_XGetSelectionOwner               xlibGetSelectionOwner;
	FN_XGetWMNormalHints                xlibGetWMNormalHints;
	FN_XGetWindowAttributes             xlibGetWindowAttributes;
	FN_XGetWindowProperty               xlibGetWindowProperty;
	FN_XIconifyWindow                   xlibIconifyWindow;
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
	short int                           win32Keycodes[512];
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
extern plafLib _glfw;





/*************************************************************************
 * GLFW API functions
 *************************************************************************/

/*! @brief Initializes the GLFW library.
 *
 *  This function initializes the GLFW library.  Before most GLFW functions can
 *  be used, GLFW must be initialized, and before an application terminates GLFW
 *  should be terminated in order to free any resources allocated during or
 *  after initialization.
 *
 *  If this function fails, it calls @ref glfwTerminate before returning.  If it
 *  succeeds, you should call @ref glfwTerminate before the application exits.
 *
 *  Additional calls to this function after successful initialization but before
 *  termination will return `true` immediately.
 *
 *  @return `true` if successful, or `false` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_PLATFORM_UNAVAILABLE and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @remark __X11:__ This function will set the `LC_CTYPE` category of the
 *  application locale according to the current environment if that category is
 *  still "C".  This is because the "C" locale breaks Unicode text input.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref intro_init
 *  @sa @ref glfwTerminate
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup init
 */
ErrorResponse* glfwInit(void);

/*! @brief Terminates the GLFW library.
 *
 *  This function destroys all remaining windows and cursors, restores any
 *  modified gamma ramps and frees any other allocated resources.  Once this
 *  function is called, you must again call @ref glfwInit successfully before
 *  you will be able to use most GLFW functions.
 *
 *  If GLFW has been successfully initialized, this function should be called
 *  before the application exits.  If initialization fails, there is no need to
 *  call this function, as it is called by @ref glfwInit before it returns
 *  failure.
 *
 *  This function has no effect if GLFW is not initialized.
 *
 *  @errors Possible errors include @ref ERR_PLATFORM_ERROR.
 *
 *  @remark This function may be called before @ref glfwInit.
 *
 *  @warning The contexts of any remaining windows must not be current on any
 *  other thread when this function is called.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref intro_init
 *  @sa @ref glfwInit
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup init
 */
void glfwTerminate(void);

/*! @brief Sets the error callback.
 *
 *  This function sets the error callback, which is called with an error code
 *  and a human-readable description each time a GLFW error occurs.
 *
 *  The error code is set before the callback is called.  Calling @ref
 *  glfwGetError from the error callback will return the same value as the error
 *  code argument.
 *
 *  The error callback is called on the thread where the error occurred.  If you
 *  are using GLFW from multiple threads, your error callback needs to be
 *  written accordingly.
 *
 *  Because the description string may have been generated specifically for that
 *  error, it is not guaranteed to be valid after the callback has returned.  If
 *  you wish to use it after the callback returns, you need to make a copy.
 *
 *  Once set, the error callback remains set even after the library has been
 *  terminated.
 *
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set.
 *
 *  @callback_signature
 *  @code
 *  void callback_name(int error_code, const char* description)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [callback pointer type](@ref errorFunc).
 *
 *  @errors None.
 *
 *  @remark This function may be called before @ref glfwInit.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref error_handling
 *  @sa @ref glfwGetError
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup init
 */
errorFunc glfwSetErrorCallback(errorFunc callback);

/*! @brief Returns the currently connected monitors.
 *
 *  This function returns an array of handles for all currently connected
 *  monitors.  The primary monitor is always first in the returned array.  If no
 *  monitors were found, this function returns `NULL`.
 *
 *  @param[out] count Where to store the number of monitors in the returned
 *  array.  This is set to zero if an error occurred.
 *  @return An array of monitor handles, or `NULL` if no monitors were found or
 *  if an [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @pointer_lifetime The returned array is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is guaranteed to be valid only until the
 *  monitor configuration changes or the library is terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_monitors
 *  @sa @ref monitor_event
 *  @sa @ref glfwGetPrimaryMonitor
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
plafMonitor** glfwGetMonitors(int* count);

/*! @brief Returns the primary monitor.
 *
 *  This function returns the primary monitor.  This is usually the monitor
 *  where elements like the task bar or global menu bar are located.
 *
 *  @return The primary monitor, or `NULL` if no monitors were found or if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @remark The primary monitor is always first in the array returned by @ref
 *  glfwGetMonitors.
 *
 *  @sa @ref monitor_monitors
 *  @sa @ref glfwGetMonitors
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
plafMonitor* glfwGetPrimaryMonitor(void);

void glfwGetMonitorPos(plafMonitor* monitor, int* xpos, int* ypos);
void glfwGetMonitorWorkarea(plafMonitor* monitor, int* xpos, int* ypos, int* width, int* height);

/*! @brief Returns the physical size of the monitor.
 *
 *  This function returns the size, in millimetres, of the display area of the
 *  specified monitor.
 *
 *  Some platforms do not provide accurate monitor size information, either
 *  because the monitor [EDID][] data is incorrect or because the driver does
 *  not report it accurately.
 *
 *  [EDID]: https://en.wikipedia.org/wiki/Extended_display_identification_data
 *
 *  Any or all of the size arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` size arguments will be set to zero.
 *
 *  @param[in] monitor The monitor to query.
 *  @param[out] widthMM Where to store the width, in millimetres, of the
 *  monitor's display area, or `NULL`.
 *  @param[out] heightMM Where to store the height, in millimetres, of the
 *  monitor's display area, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @remark __Win32:__ On Windows 8 and earlier the physical size is calculated from
 *  the current resolution and system DPI instead of querying the monitor EDID data.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_properties
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
void glfwGetMonitorPhysicalSize(plafMonitor* monitor, int* widthMM, int* heightMM);

void glfwGetMonitorContentScale(plafMonitor* monitor, float* xscale, float* yscale);

/*! @brief Returns the name of the specified monitor.
 *
 *  This function returns a human-readable name, encoded as UTF-8, of the
 *  specified monitor.  The name typically reflects the make and model of the
 *  monitor and is not guaranteed to be unique among the connected monitors.
 *
 *  @param[in] monitor The monitor to query.
 *  @return The UTF-8 encoded name of the monitor, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @pointer_lifetime The returned string is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is valid until the specified monitor is
 *  disconnected or the library is terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_properties
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
const char* glfwGetMonitorName(plafMonitor* monitor);

/*! @brief Sets the monitor configuration callback.
 *
 *  This function sets the monitor configuration callback, or removes the
 *  currently set callback.  This is called when a monitor is connected to or
 *  disconnected from the system.
 *
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafMonitor* monitor, int event)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref monitorFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_event
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
monitorFunc glfwSetMonitorCallback(monitorFunc callback);

const VideoMode* glfwGetVideoModes(plafMonitor* monitor, int* count);
const VideoMode* glfwGetVideoMode(plafMonitor* monitor);

/*! @brief Generates a gamma ramp and sets it for the specified monitor.
 *
 *  This function generates an appropriately sized gamma ramp from the specified
 *  exponent and then calls @ref glfwSetGammaRamp with it.  The value must be
 *  a finite number greater than zero.
 *
 *  The software controlled gamma ramp is applied _in addition_ to the hardware
 *  gamma correction, which today is usually an approximation of sRGB gamma.
 *  This means that setting a perfectly linear ramp, or gamma 1.0, will produce
 *  the default (usually sRGB-like) behavior.
 *
 *  For gamma correct rendering with OpenGL or OpenGL ES, see the @ref
 *  WINDOW_HINT_SRGB_CAPABLE hint.
 *
 *  @param[in] monitor The monitor whose gamma ramp to set.
 *  @param[in] gamma The desired exponent.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref ERR_INVALID_VALUE,
 *  @ref ERR_PLATFORM_ERROR and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref monitor_gamma
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup monitor
 */
void glfwSetGamma(plafMonitor* monitor, float gamma);

const GammaRamp* glfwGetGammaRamp(plafMonitor* monitor);
void glfwSetGammaRamp(plafMonitor* monitor, const GammaRamp* ramp);

/*! @brief Resets all window hints to their default values.
 *
 *  This function resets all window hints to their
 *  [default values](@ref window_hints_values).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_hints
 *  @sa @ref glfwWindowHint
 *  @sa @ref glfwWindowHintString
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwDefaultWindowHints(void);

/*! @brief Sets the specified window hint to the desired value.
 *
 *  This function sets hints for the next call to @ref glfwCreateWindow.  The
 *  hints, once set, retain their values until changed by a call to this
 *  function or @ref glfwDefaultWindowHints, or until the library is terminated.
 *
 *  Only integer value hints can be set with this function.  String value hints
 *  are set with @ref glfwWindowHintString.
 *
 *  This function does not check whether the specified hint values are valid.
 *  If you set hints to invalid values this will instead be reported by the next
 *  call to @ref glfwCreateWindow.
 *
 *  Some hints are platform specific.  These may be set on any platform but they
 *  will only affect their specific platform.  Other platforms will ignore them.
 *  Setting these hints requires no platform specific headers or functions.
 *
 *  @param[in] hint The [window hint](@ref window_hints) to set.
 *  @param[in] value The new value of the window hint.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_hints
 *  @sa @ref glfwWindowHintString
 *  @sa @ref glfwDefaultWindowHints
 *
 *  @since Added in version 3.0.  Replaces `glfwOpenWindowHint`.
 *
 *  @ingroup window
 */
void glfwWindowHint(int hint, int value);

/*! @brief Sets the specified window hint to the desired value.
 *
 *  This function sets hints for the next call to @ref glfwCreateWindow.  The
 *  hints, once set, retain their values until changed by a call to this
 *  function or @ref glfwDefaultWindowHints, or until the library is terminated.
 *
 *  Only string type hints can be set with this function.  Integer value hints
 *  are set with @ref glfwWindowHint.
 *
 *  This function does not check whether the specified hint values are valid.
 *  If you set hints to invalid values this will instead be reported by the next
 *  call to @ref glfwCreateWindow.
 *
 *  Some hints are platform specific.  These may be set on any platform but they
 *  will only affect their specific platform.  Other platforms will ignore them.
 *  Setting these hints requires no platform specific headers or functions.
 *
 *  @param[in] hint The [window hint](@ref window_hints) to set.
 *  @param[in] value The new value of the window hint.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @pointer_lifetime The specified string is copied before this function
 *  returns.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_hints
 *  @sa @ref glfwWindowHint
 *  @sa @ref glfwDefaultWindowHints
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
void glfwWindowHintString(int hint, const char* value);

/*! @brief Creates a window and its associated context.
 *
 *  This function creates a window and its associated OpenGL or OpenGL ES
 *  context.  Most of the options controlling how the window and its context
 *  should be created are specified with [window hints](@ref window_hints).
 *
 *  Successful creation does not change which context is current.  Before you
 *  can use the newly created context, you need to
 *  [make it current](@ref context_current).  For information about the `share`
 *  parameter, see @ref context_sharing.
 *
 *  The created window, framebuffer and context may differ from what you
 *  requested, as not all parameters and hints are
 *  [hard constraints](@ref window_hints_hard).  This includes the size of the
 *  window, especially for full screen windows.  To query the actual attributes
 *  of the created window, framebuffer and context, see @ref
 *  glfwGetWindowAttrib, @ref glfwGetWindowSize and @ref glfwGetFramebufferSize.
 *
 *  To create a full screen window, you need to specify the monitor the window
 *  will cover.  If no monitor is specified, the window will be windowed mode.
 *  Unless you have a way for the user to choose a specific monitor, it is
 *  recommended that you pick the primary monitor.  For more information on how
 *  to query connected monitors, see @ref monitor_monitors.
 *
 *  For full screen windows, the specified size becomes the resolution of the
 *  window's _desired video mode_.  As long as a full screen window is not
 *  iconified, the supported video mode most closely matching the desired video
 *  mode is set for the specified monitor.  For more information about full
 *  screen windows, including the creation of so called _windowed full screen_
 *  or _borderless full screen_ windows, see @ref window_windowed_full_screen.
 *
 *  Once you have created the window, you can switch it between windowed and
 *  full screen mode with @ref glfwSetWindowMonitor.  This will not affect its
 *  OpenGL or OpenGL ES context.
 *
 *  By default, newly created windows use the placement recommended by the
 *  window system.  To create the window at a specific position, set the @ref
 *  WINDOW_HINT_POSITION_X and @ref WINDOW_HINT_POSITION_Y window hints before creation.  To
 *  restore the default behavior, set either or both hints back to
 *  `ANY_POSITION`.
 *
 *  As long as at least one full screen window is not iconified, the screensaver
 *  is prohibited from starting.
 *
 *  Window systems put limits on window sizes.  Very large or very small window
 *  dimensions may be overridden by the window system on creation.  Check the
 *  actual [size](@ref window_size) after creation.
 *
 *  The [swap interval](@ref buffer_swap) is not set during window creation and
 *  the initial value may vary depending on driver settings and defaults.
 *
 *  @param[in] width The desired width, in screen coordinates, of the window.
 *  This must be greater than zero.
 *  @param[in] height The desired height, in screen coordinates, of the window.
 *  This must be greater than zero.
 *  @param[in] title The initial, UTF-8 encoded window title.
 *  @param[in] monitor The monitor to use for full screen mode, or `NULL` for
 *  windowed mode.
 *  @param[in] share The window whose context to share resources with, or `NULL`
 *  to not share resources.
 *  @return The handle of the created window, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_ENUM, @ref ERR_INVALID_VALUE, @ref ERR_API_UNAVAILABLE, @ref
 *  ERR_VERSION_UNAVAILABLE, @ref ERR_FORMAT_UNAVAILABLE, @ref
 *  ERR_NO_WINDOW_CONTEXT and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark __Win32:__ Window creation will fail if the Microsoft GDI software
 *  OpenGL implementation is the only one available.
 *
 *  @remark __Win32:__ If the executable has an icon resource named `GLFW_ICON,` it
 *  will be set as the initial icon for the window.  If no such icon is present,
 *  the `IDI_APPLICATION` icon will be used instead.  To set a different icon,
 *  see @ref glfwSetWindowIcon.
 *
 *  @remark __Win32:__ The context to share resources with must not be current on
 *  any other thread.
 *
 *  @remark __macOS:__ The OS only supports core profile contexts for OpenGL
 *  versions 3.2 and later.  Before creating an OpenGL context of version 3.2 or
 *  later you must set the [WINDOW_ATTR_HINT_OPENGL_PROFILE](@ref GLFW_OPENGL_PROFILE_hint)
 *  hint accordingly.  OpenGL 3.0 and 3.1 contexts are not supported at all
 *  on macOS.
 *
 *  @remark __macOS:__ The GLFW window has no icon, as it is not a document
 *  window, but the dock icon will be the same as the application bundle's icon.
 *  For more information on bundles, see the
 *  [Bundle Programming Guide][bundle-guide] in the Mac Developer Library.
 *
 *  [bundle-guide]: https://developer.apple.com/library/mac/documentation/CoreFoundation/Conceptual/CFBundles/
 *
 *  @remark __macOS:__  The window frame will not be rendered at full resolution
 *  on Retina displays unless the
 *  [WINDOW_HINT_SCALE_FRAMEBUFFER](@ref GLFW_SCALE_FRAMEBUFFER_hint)
 *  hint is `true` and the `NSHighResolutionCapable` key is enabled in the
 *  application bundle's `Info.plist`.  For more information, see
 *  [High Resolution Guidelines for OS X][hidpi-guide] in the Mac Developer
 *  Library.  The GLFW test and example programs use a custom `Info.plist`
 *  template for this, which can be found as `CMake/Info.plist.in` in the source
 *  tree.
 *
 *  [hidpi-guide]: https://developer.apple.com/library/mac/documentation/GraphicsAnimation/Conceptual/HighResolutionOSX/Explained/Explained.html
 *
 *  @remark __X11:__ Some window managers will not respect the placement of
 *  initially hidden windows.
 *
 *  @remark __X11:__ Due to the asynchronous nature of X11, it may take a moment for
 *  a window to reach its requested state.  This means you may not be able to
 *  query the final size, position or other attributes directly after window
 *  creation.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_creation
 *  @sa @ref glfwDestroyWindow
 *
 *  @since Added in version 3.0.  Replaces `glfwOpenWindow`.
 *
 *  @ingroup window
 */
plafWindow* glfwCreateWindow(int width, int height, const char* title, plafMonitor* monitor, plafWindow* share);

/*! @brief Destroys the specified window and its context.
 *
 *  This function destroys the specified window and its context.  On calling
 *  this function, no further callbacks will be called for that window.
 *
 *  If the context of the specified window is current on the main thread, it is
 *  detached before being destroyed.
 *
 *  @param[in] window The window to destroy.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @note The context of the specified window must not be current on any other
 *  thread when this function is called.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_creation
 *  @sa @ref glfwCreateWindow
 *
 *  @since Added in version 3.0.  Replaces `glfwCloseWindow`.
 *
 *  @ingroup window
 */
void glfwDestroyWindow(plafWindow* window);

#if defined(__APPLE__)
/*! @brief Returns the `NSWindow` of the specified window.
 *
 *  @return The `NSWindow` of the specified window, or `nil` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_UNAVAILABLE.
 *
 *  @thread_safety This function may be called from any thread.  Access is not
 *  synchronized.
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup native
 */
id glfwGetCocoaWindow(plafWindow* window);
#endif

/*! @brief Checks the close flag of the specified window.
 *
 *  This function returns the value of the close flag of the specified window.
 *
 *  @param[in] window The window to query.
 *  @return The value of the close flag.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function may be called from any thread.  Access is not
 *  synchronized.
 *
 *  @sa @ref window_close
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
int glfwWindowShouldClose(plafWindow* window);

/*! @brief Sets the close flag of the specified window.
 *
 *  This function sets the value of the close flag of the specified window.
 *  This can be used to override the user's attempt to close the window, or
 *  to signal that it should be closed.
 *
 *  @param[in] window The window whose flag to change.
 *  @param[in] value The new value.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function may be called from any thread.  Access is not
 *  synchronized.
 *
 *  @sa @ref window_close
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwSetWindowShouldClose(plafWindow* window, int value);

/*! @brief Returns the title of the specified window.
 *
 *  This function returns the window title, encoded as UTF-8, of the specified
 *  window.  This is the title set previously by @ref glfwCreateWindow
 *  or @ref glfwSetWindowTitle.
 *
 *  @param[in] window The window to query.
 *  @return The UTF-8 encoded window title, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @remark The returned title is currently a copy of the title last set by @ref
 *  glfwCreateWindow or @ref glfwSetWindowTitle.  It does not include any
 *  additional text which may be appended by the platform or another program.
 *
 *  @pointer_lifetime The returned string is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is valid until the next call to @ref
 *  glfwGetWindowTitle or @ref glfwSetWindowTitle, or until the library is
 *  terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_title
 *  @sa @ref glfwSetWindowTitle
 *
 *  @since Added in version 3.4.
 *
 *  @ingroup window
 */
const char* glfwGetWindowTitle(plafWindow* window);

/*! @brief Sets the title of the specified window.
 *
 *  This function sets the window title, encoded as UTF-8, of the specified
 *  window.
 *
 *  @param[in] window The window whose title to change.
 *  @param[in] title The UTF-8 encoded window title.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @remark __macOS:__ The window title will not be updated until the next time you
 *  process events.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_title
 *  @sa @ref glfwGetWindowTitle
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwSetWindowTitle(plafWindow* window, const char* title);

/*! @brief Sets the icon for the specified window.
 *
 *  This function sets the icon of the specified window.  If passed an array of
 *  candidate images, those of or closest to the sizes desired by the system are
 *  selected.  If no images are specified, the window reverts to its default
 *  icon.
 *
 *  The pixels are 32-bit, little-endian, non-premultiplied RGBA, i.e. eight
 *  bits per channel with the red channel first.  They are arranged canonically
 *  as packed sequential rows, starting from the top-left corner.
 *
 *  The desired image sizes varies depending on platform and system settings.
 *  The selected images will be rescaled as needed.  Good sizes include 16x16,
 *  32x32 and 48x48.
 *
 *  @param[in] window The window whose icon to set.
 *  @param[in] count The number of images in the specified array, or zero to
 *  revert to the default window icon.
 *  @param[in] images The images to create the icon from.  This is ignored if
 *  count is zero.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE, @ref ERR_PLATFORM_ERROR and @ref
 *  ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @pointer_lifetime The specified image data is copied before this function
 *  returns.
 *
 *  @remark __macOS:__ Regular windows do not have icons on macOS.  This function
 *  will emit @ref ERR_FEATURE_UNAVAILABLE.  The dock icon will be the same as
 *  the application bundle's icon.  For more information on bundles, see the
 *  [Bundle Programming Guide][bundle-guide] in the Mac Developer Library.
 *
 *  [bundle-guide]: https://developer.apple.com/library/mac/documentation/CoreFoundation/Conceptual/CFBundles/
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_icon
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwSetWindowIcon(plafWindow* window, int count, const ImageData* images);

/*! @brief Retrieves the position of the content area of the specified window.
 *
 *  This function retrieves the position, in screen coordinates, of the
 *  upper-left corner of the content area of the specified window.
 *
 *  Any or all of the position arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` position arguments will be set to zero.
 *
 *  @param[in] window The window to query.
 *  @param[out] xpos Where to store the x-coordinate of the upper-left corner of
 *  the content area, or `NULL`.
 *  @param[out] ypos Where to store the y-coordinate of the upper-left corner of
 *  the content area, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_PLATFORM_ERROR and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_pos
 *  @sa @ref glfwSetWindowPos
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwGetWindowPos(plafWindow* window, int* xpos, int* ypos);

/*! @brief Sets the position of the content area of the specified window.
 *
 *  This function sets the position, in screen coordinates, of the upper-left
 *  corner of the content area of the specified windowed mode window.  If the
 *  window is a full screen window, this function does nothing.
 *
 *  __Do not use this function__ to move an already visible window unless you
 *  have very good reasons for doing so, as it will confuse and annoy the user.
 *
 *  The window manager may put limits on what positions are allowed.  GLFW
 *  cannot and should not override these limits.
 *
 *  @param[in] window The window to query.
 *  @param[in] xpos The x-coordinate of the upper-left corner of the content area.
 *  @param[in] ypos The y-coordinate of the upper-left corner of the content area.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_PLATFORM_ERROR and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_pos
 *  @sa @ref glfwGetWindowPos
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwSetWindowPos(plafWindow* window, int xpos, int ypos);

/*! @brief Retrieves the size of the content area of the specified window.
 *
 *  This function retrieves the size, in screen coordinates, of the content area
 *  of the specified window.  If you wish to retrieve the size of the
 *  framebuffer of the window in pixels, see @ref glfwGetFramebufferSize.
 *
 *  Any or all of the size arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` size arguments will be set to zero.
 *
 *  @param[in] window The window whose size to retrieve.
 *  @param[out] width Where to store the width, in screen coordinates, of the
 *  content area, or `NULL`.
 *  @param[out] height Where to store the height, in screen coordinates, of the
 *  content area, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_size
 *  @sa @ref glfwSetWindowSize
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwGetWindowSize(plafWindow* window, int* width, int* height);

/*! @brief Sets the size limits of the specified window.
 *
 *  This function sets the size limits of the content area of the specified
 *  window.  If the window is full screen, the size limits only take effect
 *  once it is made windowed.  If the window is not resizable, this function
 *  does nothing.
 *
 *  The size limits are applied immediately to a windowed mode window and may
 *  cause it to be resized.
 *
 *  The maximum dimensions must be greater than or equal to the minimum
 *  dimensions and all must be greater than or equal to zero.
 *
 *  @param[in] window The window to set limits for.
 *  @param[in] minwidth The minimum width, in screen coordinates, of the content
 *  area, or `DONT_CARE`.
 *  @param[in] minheight The minimum height, in screen coordinates, of the
 *  content area, or `DONT_CARE`.
 *  @param[in] maxwidth The maximum width, in screen coordinates, of the content
 *  area, or `DONT_CARE`.
 *  @param[in] maxheight The maximum height, in screen coordinates, of the
 *  content area, or `DONT_CARE`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark If you set size limits and an aspect ratio that conflict, the
 *  results are undefined.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_sizelimits
 *  @sa @ref glfwSetWindowAspectRatio
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight);

/*! @brief Sets the aspect ratio of the specified window.
 *
 *  This function sets the required aspect ratio of the content area of the
 *  specified window.  If the window is full screen, the aspect ratio only takes
 *  effect once it is made windowed.  If the window is not resizable, this
 *  function does nothing.
 *
 *  The aspect ratio is specified as a numerator and a denominator and both
 *  values must be greater than zero.  For example, the common 16:9 aspect ratio
 *  is specified as 16 and 9, respectively.
 *
 *  If the numerator and denominator is set to `DONT_CARE` then the aspect
 *  ratio limit is disabled.
 *
 *  The aspect ratio is applied immediately to a windowed mode window and may
 *  cause it to be resized.
 *
 *  @param[in] window The window to set limits for.
 *  @param[in] numer The numerator of the desired aspect ratio, or
 *  `DONT_CARE`.
 *  @param[in] denom The denominator of the desired aspect ratio, or
 *  `DONT_CARE`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark If you set size limits and an aspect ratio that conflict, the
 *  results are undefined.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_sizelimits
 *  @sa @ref glfwSetWindowSizeLimits
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwSetWindowAspectRatio(plafWindow* window, int numer, int denom);

/*! @brief Sets the size of the content area of the specified window.
 *
 *  This function sets the size, in screen coordinates, of the content area of
 *  the specified window.
 *
 *  For full screen windows, this function updates the resolution of its desired
 *  video mode and switches to the video mode closest to it, without affecting
 *  the window's context.  As the context is unaffected, the bit depths of the
 *  framebuffer remain unchanged.
 *
 *  If you wish to update the refresh rate of the desired video mode in addition
 *  to its resolution, see @ref glfwSetWindowMonitor.
 *
 *  The window manager may put limits on what sizes are allowed.  GLFW cannot
 *  and should not override these limits.
 *
 *  @param[in] window The window to resize.
 *  @param[in] width The desired width, in screen coordinates, of the window
 *  content area.
 *  @param[in] height The desired height, in screen coordinates, of the window
 *  content area.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_size
 *  @sa @ref glfwGetWindowSize
 *  @sa @ref glfwSetWindowMonitor
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwSetWindowSize(plafWindow* window, int width, int height);

/*! @brief Retrieves the size of the framebuffer of the specified window.
 *
 *  This function retrieves the size, in pixels, of the framebuffer of the
 *  specified window.  If you wish to retrieve the size of the window in screen
 *  coordinates, see @ref glfwGetWindowSize.
 *
 *  Any or all of the size arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` size arguments will be set to zero.
 *
 *  @param[in] window The window whose framebuffer to query.
 *  @param[out] width Where to store the width, in pixels, of the framebuffer,
 *  or `NULL`.
 *  @param[out] height Where to store the height, in pixels, of the framebuffer,
 *  or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_fbsize
 *  @sa @ref glfwSetFramebufferSizeCallback
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwGetFramebufferSize(plafWindow* window, int* width, int* height);

/*! @brief Retrieves the size of the frame of the window.
 *
 *  This function retrieves the size, in screen coordinates, of each edge of the
 *  frame of the specified window.  This size includes the title bar, if the
 *  window has one.  The size of the frame may vary depending on the
 *  [window-related hints](@ref window_hints_wnd) used to create it.
 *
 *  Because this function retrieves the size of each window frame edge and not
 *  the offset along a particular coordinate axis, the retrieved values will
 *  always be zero or positive.
 *
 *  Any or all of the size arguments may be `NULL`.  If an error occurs, all
 *  non-`NULL` size arguments will be set to zero.
 *
 *  @param[in] window The window whose frame size to query.
 *  @param[out] left Where to store the size, in screen coordinates, of the left
 *  edge of the window frame, or `NULL`.
 *  @param[out] top Where to store the size, in screen coordinates, of the top
 *  edge of the window frame, or `NULL`.
 *  @param[out] right Where to store the size, in screen coordinates, of the
 *  right edge of the window frame, or `NULL`.
 *  @param[out] bottom Where to store the size, in screen coordinates, of the
 *  bottom edge of the window frame, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_size
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup window
 */
void glfwGetWindowFrameSize(plafWindow* window, int* left, int* top, int* right, int* bottom);

/*! @brief Retrieves the content scale for the specified window.
 *
 *  This function retrieves the content scale for the specified window.  The
 *  content scale is the ratio between the current DPI and the platform's
 *  default DPI.  This is especially important for text and any UI elements.  If
 *  the pixel dimensions of your UI scaled by this look appropriate on your
 *  machine then it should appear at a reasonable size on other machines
 *  regardless of their DPI and scaling settings.  This relies on the system DPI
 *  and scaling settings being somewhat correct.
 *
 *  On platforms where each monitors can have its own content scale, the window
 *  content scale will depend on which monitor the system considers the window
 *  to be on.
 *
 *  @param[in] window The window to query.
 *  @param[out] xscale Where to store the x-axis content scale, or `NULL`.
 *  @param[out] yscale Where to store the y-axis content scale, or `NULL`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_scale
 *  @sa @ref glfwSetWindowContentScaleCallback
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
void glfwGetWindowContentScale(plafWindow* window, float* xscale, float* yscale);

/*! @brief Returns the opacity of the whole window.
 *
 *  This function returns the opacity of the window, including any decorations.
 *
 *  The opacity (or alpha) value is a positive finite number between zero and
 *  one, where zero is fully transparent and one is fully opaque.  If the system
 *  does not support whole window transparency, this function always returns one.
 *
 *  The initial opacity value for newly created windows is one.
 *
 *  @param[in] window The window to query.
 *  @return The opacity value of the specified window.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_transparency
 *  @sa @ref glfwSetWindowOpacity
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
float glfwGetWindowOpacity(plafWindow* window);

/*! @brief Sets the opacity of the whole window.
 *
 *  This function sets the opacity of the window, including any decorations.
 *
 *  The opacity (or alpha) value is a positive finite number between zero and
 *  one, where zero is fully transparent and one is fully opaque.
 *
 *  The initial opacity value for newly created windows is one.
 *
 *  A window created with framebuffer transparency may not use whole window
 *  transparency.  The results of doing this are undefined.
 *
 *  @param[in] window The window to set the opacity for.
 *  @param[in] opacity The desired opacity of the specified window.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_PLATFORM_ERROR and @ref ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_transparency
 *  @sa @ref glfwGetWindowOpacity
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
void glfwSetWindowOpacity(plafWindow* window, float opacity);

/*! @brief Iconifies the specified window.
 *
 *  This function iconifies (minimizes) the specified window if it was
 *  previously restored.  If the window is already iconified, this function does
 *  nothing.
 *
 *  If the specified window is a full screen window, GLFW restores the original
 *  video mode of the monitor.  The window's desired video mode is set again
 *  when the window is restored.
 *
 *  @param[in] window The window to iconify.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_iconify
 *  @sa @ref glfwRestoreWindow
 *  @sa @ref glfwMaximizeWindow
 *
 *  @since Added in version 2.1.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwIconifyWindow(plafWindow* window);

/*! @brief Restores the specified window.
 *
 *  This function restores the specified window if it was previously iconified
 *  (minimized) or maximized.  If the window is already restored, this function
 *  does nothing.
 *
 *  If the specified window is an iconified full screen window, its desired
 *  video mode is set again for its monitor when the window is restored.
 *
 *  @param[in] window The window to restore.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_iconify
 *  @sa @ref glfwIconifyWindow
 *  @sa @ref glfwMaximizeWindow
 *
 *  @since Added in version 2.1.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwRestoreWindow(plafWindow* window);

/*! @brief Maximizes the specified window.
 *
 *  This function maximizes the specified window if it was previously not
 *  maximized.  If the window is already maximized, this function does nothing.
 *
 *  If the specified window is a full screen window, this function does nothing.
 *
 *  @param[in] window The window to maximize.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @par Thread Safety
 *  This function may only be called from the main thread.
 *
 *  @sa @ref window_iconify
 *  @sa @ref glfwIconifyWindow
 *  @sa @ref glfwRestoreWindow
 *
 *  @since Added in GLFW 3.2.
 *
 *  @ingroup window
 */
void glfwMaximizeWindow(plafWindow* window);

/*! @brief Makes the specified window visible.
 *
 *  This function makes the specified window visible if it was previously
 *  hidden.  If the window is already visible or is in full screen mode, this
 *  function does nothing.
 *
 *  @param[in] window The window to make visible.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_hide
 *  @sa @ref glfwHideWindow
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwShowWindow(plafWindow* window);

/*! @brief Hides the specified window.
 *
 *  This function hides the specified window if it was previously visible.  If
 *  the window is already hidden or is in full screen mode, this function does
 *  nothing.
 *
 *  @param[in] window The window to hide.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_hide
 *  @sa @ref glfwShowWindow
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
void glfwHideWindow(plafWindow* window);

/*! @brief Brings the specified window to front and sets input focus.
 *
 *  This function brings the specified window to front and sets input focus.
 *  The window should already be visible and not iconified.
 *
 *  For a less disruptive way of getting the user's attention, see
 *  [attention requests](@ref window_attention).
 *
 *  @param[in] window The window to give input focus.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_focus
 *  @sa @ref window_attention
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwFocusWindow(plafWindow* window);

/*! @brief Requests user attention to the specified window.
 *
 *  This function requests user attention to the specified window.  On
 *  platforms where this is not supported, attention is requested to the
 *  application as a whole.
 *
 *  Once the user has given attention, usually by focusing the window or
 *  application, the system will end the request automatically.
 *
 *  @param[in] window The window to request attention to.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @remark __macOS:__ Attention is requested to the application as a whole, not the
 *  specific window.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_attention
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
void glfwRequestWindowAttention(plafWindow* window);

/*! @brief Returns the monitor that the window uses for full screen mode.
 *
 *  This function returns the handle of the monitor that the specified window is
 *  in full screen on.
 *
 *  @param[in] window The window to query.
 *  @return The monitor, or `NULL` if the window is in windowed mode or an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_monitor
 *  @sa @ref glfwSetWindowMonitor
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
plafMonitor* glfwGetWindowMonitor(plafWindow* window);

/*! @brief Sets the mode, monitor, video mode and placement of a window.
 *
 *  This function sets the monitor that the window uses for full screen mode or,
 *  if the monitor is `NULL`, makes it windowed mode.
 *
 *  When setting a monitor, this function updates the width, height and refresh
 *  rate of the desired video mode and switches to the video mode closest to it.
 *  The window position is ignored when setting a monitor.
 *
 *  When the monitor is `NULL`, the position, width and height are used to
 *  place the window content area.  The refresh rate is ignored when no monitor
 *  is specified.
 *
 *  If you only wish to update the resolution of a full screen window or the
 *  size of a windowed mode window, see @ref glfwSetWindowSize.
 *
 *  When a window transitions from full screen to windowed mode, this function
 *  restores any previous window settings such as whether it is decorated,
 *  floating, resizable, has size or aspect ratio limits, etc.
 *
 *  @param[in] window The window whose monitor, size or video mode to set.
 *  @param[in] monitor The desired monitor, or `NULL` to set windowed mode.
 *  @param[in] xpos The desired x-coordinate of the upper-left corner of the
 *  content area.
 *  @param[in] ypos The desired y-coordinate of the upper-left corner of the
 *  content area.
 *  @param[in] width The desired with, in screen coordinates, of the content
 *  area or video mode.
 *  @param[in] height The desired height, in screen coordinates, of the content
 *  area or video mode.
 *  @param[in] refreshRate The desired refresh rate, in Hz, of the video mode,
 *  or `DONT_CARE`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @remark The OpenGL or OpenGL ES context will not be destroyed or otherwise
 *  affected by any resizing or mode switching, although you may need to update
 *  your viewport if the framebuffer size has changed.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_monitor
 *  @sa @ref window_full_screen
 *  @sa @ref glfwGetWindowMonitor
 *  @sa @ref glfwSetWindowSize
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwSetWindowMonitor(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate);

/*! @brief Returns an attribute of the specified window.
 *
 *  This function returns the value of an attribute of the specified window or
 *  its OpenGL or OpenGL ES context.
 *
 *  @param[in] window The window to query.
 *  @param[in] attrib The [window attribute](@ref window_attribs) whose value to
 *  return.
 *  @return The value of the attribute, or zero if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_ENUM and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark Framebuffer related hints are not window attributes.  See @ref
 *  window_attribs_fb for more information.
 *
 *  @remark Zero is a valid value for many window and context related
 *  attributes so you cannot use a return value of zero as an indication of
 *  errors.  However, this function should not fail as long as it is passed
 *  valid arguments and the library has been [initialized](@ref intro_init).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_attribs
 *  @sa @ref glfwSetWindowAttrib
 *
 *  @since Added in version 3.0.  Replaces `glfwGetWindowParam` and
 *  `glfwGetGLVersion`.
 *
 *  @ingroup window
 */
int glfwGetWindowAttrib(plafWindow* window, int attrib);

/*! @brief Sets an attribute of the specified window.
 *
 *  This function sets the value of an attribute of the specified window.
 *
 *  The supported attributes are [WINDOW_ATTR_HINT_DECORATED](@ref GLFW_DECORATED_attrib),
 *  [WINDOW_ATTR_HINT_RESIZABLE](@ref GLFW_RESIZABLE_attrib),
 *  [WINDOW_ATTR_HINT_FLOATING](@ref GLFW_FLOATING_attrib),
 *  [WINDOW_ATTR_HINT_MOUSE_PASSTHROUGH](@ref GLFW_MOUSE_PASSTHROUGH_attrib)
 *
 *  Some of these attributes are ignored for full screen windows.  The new
 *  value will take effect if the window is later made windowed.
 *
 *  Some of these attributes are ignored for windowed mode windows.  The new
 *  value will take effect if the window is later made full screen.
 *
 *  @param[in] window The window to set the attribute for.
 *  @param[in] attrib A supported window attribute.
 *  @param[in] value `true` or `false`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_ENUM, @ref ERR_INVALID_VALUE, @ref ERR_PLATFORM_ERROR and @ref
 *  ERR_FEATURE_UNAVAILABLE (see remarks).
 *
 *  @remark Calling @ref glfwGetWindowAttrib will always return the latest
 *  value, even if that value is ignored by the current mode of the window.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_attribs
 *  @sa @ref glfwGetWindowAttrib
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
void glfwSetWindowAttrib(plafWindow* window, int attrib, int value);

/*! @brief Sets the position callback for the specified window.
 *
 *  This function sets the position callback of the specified window, which is
 *  called when the window is moved.  The callback is provided with the
 *  position, in screen coordinates, of the upper-left corner of the content
 *  area of the window.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, int xpos, int ypos)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowPosFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_pos
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
windowPosFunc glfwSetWindowPosCallback(plafWindow* window, windowPosFunc callback);

/*! @brief Sets the size callback for the specified window.
 *
 *  This function sets the size callback of the specified window, which is
 *  called when the window is resized.  The callback is provided with the size,
 *  in screen coordinates, of the content area of the window.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, int width, int height)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowSizeFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_size
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup window
 */
windowSizeFunc glfwSetWindowSizeCallback(plafWindow* window, windowSizeFunc callback);

/*! @brief Sets the close callback for the specified window.
 *
 *  This function sets the close callback of the specified window, which is
 *  called when the user attempts to close the window, for example by clicking
 *  the close widget in the title bar.
 *
 *  The close flag is set before this callback is called, but you can modify it
 *  at any time with @ref glfwSetWindowShouldClose.
 *
 *  The close callback is not triggered by @ref glfwDestroyWindow.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowCloseFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @remark __macOS:__ Selecting Quit from the application menu will trigger the
 *  close callback for all windows.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_close
 *
 *  @since Added in version 2.5.
 *  __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup window
 */
windowCloseFunc glfwSetWindowCloseCallback(plafWindow* window, windowCloseFunc callback);

/*! @brief Sets the refresh callback for the specified window.
 *
 *  This function sets the refresh callback of the specified window, which is called when the content area of the window
 *  needs to be redrawn, for example if the window has been exposed after having been covered by another window.
 *
 *  On compositing window systems where the window contents are saved off-screen, this callback may be called only very
 *  infrequently or never at all.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the library had not been [initialized](@ref
 *  intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window);
 *  @endcode
 *  For more information about the callback parameters, see the [function pointer type](@ref windowRefreshFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_refresh
 *
 *  @since Added in version 2.5. __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup window
 */
windowRefreshFunc glfwSetWindowRefreshCallback(plafWindow* window, windowRefreshFunc callback);

/*! @brief Sets the focus callback for the specified window.
 *
 *  This function sets the focus callback of the specified window, which is
 *  called when the window gains or loses input focus.
 *
 *  After the focus callback is called for a window that lost input focus,
 *  synthetic key and mouse button release events will be generated for all such
 *  that had been pressed.  For more information, see @ref glfwSetKeyCallback
 *  and @ref glfwSetMouseButtonCallback.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, int focused)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowFocusFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_focus
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
windowFocusFunc glfwSetWindowFocusCallback(plafWindow* window, windowFocusFunc callback);

/*! @brief Sets the iconify callback for the specified window.
 *
 *  This function sets the iconification callback of the specified window, which
 *  is called when the window is iconified or restored.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, int iconified)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowIconifyFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_iconify
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
windowIconifyFunc glfwSetWindowIconifyCallback(plafWindow* window, windowIconifyFunc callback);

/*! @brief Sets the maximize callback for the specified window.
 *
 *  This function sets the maximization callback of the specified window, which
 *  is called when the window is maximized or restored.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, int maximized)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowMaximizeFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_maximize
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
windowMaximizeFunc glfwSetWindowMaximizeCallback(plafWindow* window, windowMaximizeFunc callback);

/*! @brief Sets the framebuffer resize callback for the specified window.
 *
 *  This function sets the framebuffer resize callback of the specified window,
 *  which is called when the framebuffer of the specified window is resized.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, int width, int height)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref frameBufferSizeFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_fbsize
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup window
 */
frameBufferSizeFunc glfwSetFramebufferSizeCallback(plafWindow* window, frameBufferSizeFunc callback);

/*! @brief Sets the window content scale callback for the specified window.
 *
 *  This function sets the window content scale callback of the specified window,
 *  which is called when the content scale of the specified window changes.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, float xscale, float yscale)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref windowContextScaleFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref window_scale
 *  @sa @ref glfwGetWindowContentScale
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup window
 */
windowContextScaleFunc glfwSetWindowContentScaleCallback(plafWindow* window, windowContextScaleFunc callback);

/*! @brief Processes all pending events.
 *
 *  This function processes only those events that are already in the event
 *  queue and then returns immediately.  Processing events will cause the window
 *  and input callbacks associated with those events to be called.
 *
 *  On some platforms, a window move, resize or menu operation will cause event
 *  processing to block.  This is due to how event processing is designed on
 *  those platforms.  You can use the
 *  [window refresh callback](@ref window_refresh) to redraw the contents of
 *  your window when necessary during such operations.
 *
 *  Do not assume that callbacks you set will _only_ be called in response to
 *  event processing functions like this one.  While it is necessary to poll for
 *  events, window systems that require GLFW to register callbacks of its own
 *  can pass events to GLFW in response to many window system function calls.
 *  GLFW will pass those events on to the application callbacks before
 *  returning.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref events
 *  @sa @ref glfwWaitEvents
 *  @sa @ref glfwWaitEventsTimeout
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup window
 */
void glfwPollEvents(void);

/*! @brief Waits until events are queued and processes them.
 *
 *  This function puts the calling thread to sleep until at least one event is
 *  available in the event queue.  Once one or more events are available,
 *  it behaves exactly like @ref glfwPollEvents, i.e. the events in the queue
 *  are processed and the function then returns immediately.  Processing events
 *  will cause the window and input callbacks associated with those events to be
 *  called.
 *
 *  Since not all events are associated with callbacks, this function may return
 *  without a callback having been called even if you are monitoring all
 *  callbacks.
 *
 *  On some platforms, a window move, resize or menu operation will cause event
 *  processing to block.  This is due to how event processing is designed on
 *  those platforms.  You can use the
 *  [window refresh callback](@ref window_refresh) to redraw the contents of
 *  your window when necessary during such operations.
 *
 *  Do not assume that callbacks you set will _only_ be called in response to
 *  event processing functions like this one.  While it is necessary to poll for
 *  events, window systems that require GLFW to register callbacks of its own
 *  can pass events to GLFW in response to many window system function calls.
 *  GLFW will pass those events on to the application callbacks before
 *  returning.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref events
 *  @sa @ref glfwPollEvents
 *  @sa @ref glfwWaitEventsTimeout
 *
 *  @since Added in version 2.5.
 *
 *  @ingroup window
 */
void glfwWaitEvents(void);

/*! @brief Waits with timeout until events are queued and processes them.
 *
 *  This function puts the calling thread to sleep until at least one event is
 *  available in the event queue, or until the specified timeout is reached.  If
 *  one or more events are available, it behaves exactly like @ref
 *  glfwPollEvents, i.e. the events in the queue are processed and the function
 *  then returns immediately.  Processing events will cause the window and input
 *  callbacks associated with those events to be called.
 *
 *  The timeout value must be a positive finite number.
 *
 *  Since not all events are associated with callbacks, this function may return
 *  without a callback having been called even if you are monitoring all
 *  callbacks.
 *
 *  On some platforms, a window move, resize or menu operation will cause event
 *  processing to block.  This is due to how event processing is designed on
 *  those platforms.  You can use the
 *  [window refresh callback](@ref window_refresh) to redraw the contents of
 *  your window when necessary during such operations.
 *
 *  Do not assume that callbacks you set will _only_ be called in response to
 *  event processing functions like this one.  While it is necessary to poll for
 *  events, window systems that require GLFW to register callbacks of its own
 *  can pass events to GLFW in response to many window system function calls.
 *  GLFW will pass those events on to the application callbacks before
 *  returning.
 *
 *  @param[in] timeout The maximum amount of time, in seconds, to wait.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE and @ref ERR_PLATFORM_ERROR.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref events
 *  @sa @ref glfwPollEvents
 *  @sa @ref glfwWaitEvents
 *
 *  @since Added in version 3.2.
 *
 *  @ingroup window
 */
void glfwWaitEventsTimeout(double timeout);

/*! @brief Posts an empty event to the event queue.
 *
 *  This function posts an empty event from the current thread to the event
 *  queue, causing @ref glfwWaitEvents or @ref glfwWaitEventsTimeout to return.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref events
 *  @sa @ref glfwWaitEvents
 *  @sa @ref glfwWaitEventsTimeout
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup window
 */
void glfwPostEmptyEvent(void);

/*! @brief Returns the value of an input option for the specified window.
 *
 *  This function returns the value of an input option for the specified window.
 *  The mode must be one of @ref INPUT_MODE_CURSOR, @ref INPUT_MODE_STICKY_KEYS,
 *  @ref INPUT_MODE_STICKY_MOUSE_BUTTONS, @ref INPUT_MODE_LOCK_KEY_MODS or
 *  @ref INPUT_MODE_RAW_MOUSE_MOTION.
 *
 *  @param[in] window The window to query.
 *  @param[in] mode One of `INPUT_MODE_CURSOR`, `INPUT_MODE_STICKY_KEYS`,
 *  `INPUT_MODE_STICKY_MOUSE_BUTTONS`, `INPUT_MODE_LOCK_KEY_MODS` or
 *  `INPUT_MODE_RAW_MOUSE_MOTION`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref glfwSetInputMode
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup input
 */
int glfwGetInputMode(plafWindow* window, int mode);

/*! @brief Sets an input option for the specified window.
 *
 *  This function sets an input mode option for the specified window.  The mode
 *  must be one of @ref INPUT_MODE_CURSOR, @ref INPUT_MODE_STICKY_KEYS,
 *  @ref INPUT_MODE_STICKY_MOUSE_BUTTONS, @ref INPUT_MODE_LOCK_KEY_MODS
 *  @ref INPUT_MODE_RAW_MOUSE_MOTION, or @ref INPUT_MODE_UNLIMITED_MOUSE_BUTTONS.
 *
 *  If the mode is `INPUT_MODE_CURSOR`, the value must be one of the following cursor
 *  modes:
 *  - `CURSOR_NORMAL` makes the cursor visible and behaving normally.
 *  - `CURSOR_HIDDEN` makes the cursor invisible when it is over the
 *    content area of the window but does not restrict the cursor from leaving.
 *
 *  If the mode is `INPUT_MODE_STICKY_KEYS`, the value must be either `true` to
 *  enable sticky keys, or `false` to disable it.  If sticky keys are
 *  enabled, a key press will ensure that @ref glfwGetKey returns `INPUT_PRESS`
 *  the next time it is called even if the key had been released before the
 *  call.  This is useful when you are only interested in whether keys have been
 *  pressed but not when or in which order.
 *
 *  If the mode is `INPUT_MODE_STICKY_MOUSE_BUTTONS`, the value must be either
 *  `true` to enable sticky mouse buttons, or `false` to disable it.
 *  If sticky mouse buttons are enabled, a mouse button press will ensure that
 *  @ref glfwGetMouseButton returns `INPUT_PRESS` the next time it is called even
 *  if the mouse button had been released before the call.  This is useful when
 *  you are only interested in whether mouse buttons have been pressed but not
 *  when or in which order.
 *
 *  If the mode is `INPUT_MODE_LOCK_KEY_MODS`, the value must be either `true` to
 *  enable lock key modifier bits, or `false` to disable them.  If enabled,
 *  callbacks that receive modifier bits will also have the @ref
 *  KEYMOD_CAPS_LOCK bit set when the event was generated with Caps Lock on,
 *  and the @ref KEYMOD_NUM_LOCK bit when Num Lock was on.
 *
 *  If the mode is `INPUT_MODE_UNLIMITED_MOUSE_BUTTONS`, the value must be either
 *  `true` to disable the mouse button limit when calling the mouse button
 *  callback, or `false` to limit the mouse buttons sent to the callback
 *  to the mouse button token values up to `MOUSE_BUTTON_LAST`.
 *
 *  @param[in] window The window whose input mode to set.
 *  @param[in] mode One of `INPUT_MODE_CURSOR`, `INPUT_MODE_STICKY_KEYS`,
 *  `INPUT_MODE_STICKY_MOUSE_BUTTONS`, `INPUT_MODE_LOCK_KEY_MODS` or
 *  `INPUT_MODE_RAW_MOUSE_MOTION`.
 *  @param[in] value The new value of the specified input mode.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_ENUM, @ref ERR_PLATFORM_ERROR and @ref
 *  ERR_FEATURE_UNAVAILABLE (see above).
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref glfwGetInputMode
 *
 *  @since Added in version 3.0.  Replaces `glfwEnable` and `glfwDisable`.
 *
 *  @ingroup input
 */
void glfwSetInputMode(plafWindow* window, int mode, int value);

/*! @brief Returns the platform-specific scancode of the specified key.
 *
 *  This function returns the platform-specific scancode of the specified key.
 *
 *  If the specified [key token](@ref keys) corresponds to a physical key not
 *  supported on the current platform then this method will return `-1`.
 *  Calling this function with anything other than a key token will return `-1`
 *  and generate a @ref ERR_INVALID_ENUM error.
 *
 *  @param[in] key Any [key token](@ref keys).
 *  @return The platform-specific scancode for the key, or `-1` if the key is
 *  not supported on the current platform or an [error](@ref error_handling)
 *  occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref input_key
 *
 *  @since Added in version 3.3.
 *
 *  @ingroup input
 */
int glfwGetKeyScancode(int key);

/*! @brief Returns the last reported state of a keyboard key for the specified
 *  window.
 *
 *  This function returns the last state reported for the specified key to the
 *  specified window.  The returned state is one of `INPUT_PRESS` or
 *  `INPUT_RELEASE`.  The action `INPUT_REPEAT` is only reported to the key callback.
 *
 *  If the @ref INPUT_MODE_STICKY_KEYS input mode is enabled, this function returns
 *  `INPUT_PRESS` the first time you call it for a key that was pressed, even if
 *  that key has already been released.
 *
 *  The key functions deal with physical keys, with [key tokens](@ref keys)
 *  named after their use on the standard US keyboard layout.  If you want to
 *  input text, use the Unicode character callback instead.
 *
 *  The [modifier key bit masks](@ref mods) are not key tokens and cannot be
 *  used with this function.
 *
 *  __Do not use this function__ to implement [text input](@ref input_char).
 *
 *  @param[in] window The desired window.
 *  @param[in] key The desired [keyboard key](@ref keys).  `KEY_UNKNOWN` is
 *  not a valid key for this function.
 *  @return One of `INPUT_PRESS` or `INPUT_RELEASE`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_key
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup input
 */
int glfwGetKey(plafWindow* window, int key);

/*! @brief Returns the last reported state of a mouse button for the specified
 *  window.
 *
 *  This function returns the last state reported for the specified mouse button
 *  to the specified window.  The returned state is one of `INPUT_PRESS` or
 *  `INPUT_RELEASE`.
 *
 *  If the @ref INPUT_MODE_STICKY_MOUSE_BUTTONS input mode is enabled, this function
 *  returns `INPUT_PRESS` the first time you call it for a mouse button that was
 *  pressed, even if that mouse button has already been released.
 *
 *  The @ref INPUT_MODE_UNLIMITED_MOUSE_BUTTONS input mode does not effect the
 *  limit on buttons which can be polled with this function.
 *
 *  @param[in] window The desired window.
 *  @param[in] button The desired [mouse button token](@ref buttons).
 *  @return One of `INPUT_PRESS` or `INPUT_RELEASE`.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_INVALID_ENUM.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_mouse_button
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup input
 */
int glfwGetMouseButton(plafWindow* window, int button);

void glfwGetCursorPos(plafWindow* window, double* xpos, double* ypos);
void glfwSetCursorPos(plafWindow* window, double xpos, double ypos);
void glfwSetCursor(plafWindow* window, plafCursor* cursor);

/*! @brief Creates a custom cursor.
 *
 *  Creates a new custom cursor image that can be set for a window with @ref
 *  glfwSetCursor.  The cursor can be destroyed with @ref glfwDestroyCursor.
 *  Any remaining cursors are destroyed by @ref glfwTerminate.
 *
 *  The pixels are 32-bit, little-endian, non-premultiplied RGBA, i.e. eight
 *  bits per channel with the red channel first.  They are arranged canonically
 *  as packed sequential rows, starting from the top-left corner.
 *
 *  The cursor hotspot is specified in pixels, relative to the upper-left corner
 *  of the cursor image.  Like all other coordinate systems in GLFW, the X-axis
 *  points to the right and the Y-axis points down.
 *
 *  @param[in] image The desired cursor image.
 *  @param[in] xhot The desired x-coordinate, in pixels, of the cursor hotspot.
 *  @param[in] yhot The desired y-coordinate, in pixels, of the cursor hotspot.
 *  @return The handle of the created cursor, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_VALUE and @ref ERR_PLATFORM_ERROR.
 *
 *  @pointer_lifetime The specified image data is copied before this function
 *  returns.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_object
 *  @sa @ref glfwDestroyCursor
 *  @sa @ref glfwCreateStandardCursor
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
plafCursor* glfwCreateCursor(const ImageData* image, int xhot, int yhot);

/*! @brief Creates a cursor with a standard shape.
 *
 *  Returns a cursor with a standard shape, that can be set for a window with
 *  @ref glfwSetCursor.  The images for these cursors come from the system
 *  cursor theme and their exact appearance will vary between platforms.
 *
 *  Most of these shapes are guaranteed to exist on every supported platform but
 *  a few may not be present.  See the table below for details.
 *
 *  Cursor shape                   | Windows | macOS | X11
 *  ------------------------------ | ------- | ----- | ------
 *  @ref STD_CURSOR_ARROW         | Yes     | Yes   | Yes
 *  @ref STD_CURSOR_IBEAM         | Yes     | Yes   | Yes
 *  @ref STD_CURSOR_CROSSHAIR     | Yes     | Yes   | Yes
 *  @ref STD_CURSOR_POINTING_HAND | Yes     | Yes   | Yes
 *  @ref STD_CURSOR_HORIZONTAL_RESIZE     | Yes     | Yes   | Yes
 *  @ref STD_CURSOR_VERTICAL_RESIZE     | Yes     | Yes   | Yes
 *
 *  1) This uses a private system API and may fail in the future.
 *
 *  2) This uses a newer standard that not all cursor themes support.
 *
 *  If the requested shape is not available, this function emits a @ref
 *  GLFW_CURSOR_UNAVAILABLE error and returns `NULL`.
 *
 *  @param[in] shape One of the [standard shapes](@ref shapes).
 *  @return A new cursor ready to use or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_INVALID_ENUM, @ref GLFW_CURSOR_UNAVAILABLE and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_standard
 *  @sa @ref glfwCreateCursor
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
plafCursor* glfwCreateStandardCursor(int shape);

/*! @brief Destroys a cursor.
 *
 *  This function destroys a cursor previously created with @ref
 *  glfwCreateCursor.  Any remaining cursors will be destroyed by @ref
 *  glfwTerminate.
 *
 *  If the specified cursor is current for any window, that window will be
 *  reverted to the default cursor.  This does not affect the cursor mode.
 *
 *  @param[in] cursor The cursor object to destroy.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @reentrancy This function must not be called from a callback.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_object
 *  @sa @ref glfwCreateCursor
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
void glfwDestroyCursor(plafCursor* cursor);

/*! @brief Sets the key callback.
 *
 *  This function sets the key callback of the specified window, which is called
 *  when a key is pressed, repeated or released.
 *
 *  The key functions deal with physical keys, with layout independent
 *  [key tokens](@ref keys) named after their values in the standard US keyboard
 *  layout.  If you want to input text, use the
 *  [character callback](@ref glfwSetCharCallback) instead.
 *
 *  When a window loses input focus, it will generate synthetic key release
 *  events for all pressed keys with associated key tokens.  You can tell these
 *  events from user-generated events by the fact that the synthetic ones are
 *  generated after the focus loss event has been processed, i.e. after the
 *  [window focus callback](@ref glfwSetWindowFocusCallback) has been called.
 *
 *  The scancode of a key is specific to that platform or sometimes even to that
 *  machine.  Scancodes are intended to allow users to bind keys that don't have
 *  a GLFW key token.  Such keys have `key` set to `KEY_UNKNOWN`, their
 *  state is not saved and so it cannot be queried with @ref glfwGetKey.
 *
 *  Sometimes GLFW needs to generate synthetic key events, in which case the
 *  scancode may be zero.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new key callback, or `NULL` to remove the currently
 *  set callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, int key, int scancode, int action, int mods)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref keyFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_key
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup input
 */
keyFunc glfwSetKeyCallback(plafWindow* window, keyFunc callback);

/*! @brief Sets the Unicode character callback.
 *
 *  This function sets the character callback of the specified window, which is
 *  called when a Unicode character is input.
 *
 *  The character callback is intended for Unicode text input.  As it deals with
 *  characters, it is keyboard layout dependent, whereas the
 *  [key callback](@ref glfwSetKeyCallback) is not.  Characters do not map 1:1
 *  to physical keys, as a key may produce zero, one or more characters.  If you
 *  want to know whether a specific physical key was pressed or released, see
 *  the key callback instead.
 *
 *  The character callback behaves as system text input normally does and will
 *  not be called if modifier keys are held down that would prevent normal text
 *  input on that platform, for example a Super (Command) key on macOS or Alt key
 *  on Windows.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, unsigned int codepoint)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref charFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_char
 *
 *  @since Added in version 2.4.
 *  __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup input
 */
charFunc glfwSetCharCallback(plafWindow* window, charFunc callback);

/*! @brief Sets the Unicode character with modifiers callback.
 *
 *  This function sets the character with modifiers callback of the specified
 *  window, which is called when a Unicode character is input regardless of what
 *  modifier keys are used.
 *
 *  The character with modifiers callback is intended for implementing custom
 *  Unicode character input.  For regular Unicode text input, see the
 *  [character callback](@ref glfwSetCharCallback).  Like the character
 *  callback, the character with modifiers callback deals with characters and is
 *  keyboard layout dependent.  Characters do not map 1:1 to physical keys, as
 *  a key may produce zero, one or more characters.  If you want to know whether
 *  a specific physical key was pressed or released, see the
 *  [key callback](@ref glfwSetKeyCallback) instead.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or an
 *  [error](@ref error_handling) occurred.
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, unsigned int codepoint, int mods)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref charModsFunc).
 *
 *  @deprecated Scheduled for removal in version 4.0.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_char
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
charModsFunc glfwSetCharModsCallback(plafWindow* window, charModsFunc callback);

/*! @brief Sets the mouse button callback.
 *
 *  This function sets the mouse button callback of the specified window, which
 *  is called when a mouse button is pressed or released.
 *
 *  When a window loses input focus, it will generate synthetic mouse button
 *  release events for all pressed mouse buttons with associated button tokens.
 *  You can tell these events from user-generated events by the fact that the
 *  synthetic ones are generated after the focus loss event has been processed,
 *  i.e. after the [window focus callback](@ref glfwSetWindowFocusCallback) has
 *  been called.
 *
 *  The reported `button` value can be higher than `MOUSE_BUTTON_LAST` if
 *  the button does not have an associated [button token](@ref buttons) and the
 *  @ref INPUT_MODE_UNLIMITED_MOUSE_BUTTONS input mode is set.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, int button, int action, int mods)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref mouseButtonFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref input_mouse_button
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter and return value.
 *
 *  @ingroup input
 */
mouseButtonFunc glfwSetMouseButtonCallback(plafWindow* window, mouseButtonFunc callback);

/*! @brief Sets the cursor position callback.
 *
 *  This function sets the cursor position callback of the specified window,
 *  which is called when the cursor is moved.  The callback is provided with the
 *  position, in screen coordinates, relative to the upper-left corner of the
 *  content area of the window.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, double xpos, double ypos);
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref cursorPosFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_pos
 *
 *  @since Added in version 3.0.  Replaces `glfwSetMousePosCallback`.
 *
 *  @ingroup input
 */
cursorPosFunc glfwSetCursorPosCallback(plafWindow* window, cursorPosFunc callback);

/*! @brief Sets the cursor enter/leave callback.
 *
 *  This function sets the cursor boundary crossing callback of the specified
 *  window, which is called when the cursor enters or leaves the content area of
 *  the window.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new callback, or `NULL` to remove the currently set
 *  callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, int entered)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref cursorEnterFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref cursor_enter
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup input
 */
cursorEnterFunc glfwSetCursorEnterCallback(plafWindow* window, cursorEnterFunc callback);

/*! @brief Sets the scroll callback.
 *
 *  This function sets the scroll callback of the specified window, which is
 *  called when a scrolling device is used, such as a mouse wheel or scrolling
 *  area of a touchpad.
 *
 *  The scroll callback receives all scrolling input, like that from a mouse
 *  wheel or a touchpad scrolling area.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new scroll callback, or `NULL` to remove the
 *  currently set callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, double xoffset, double yoffset)
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref scrollFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref scrolling
 *
 *  @since Added in version 3.0.  Replaces `glfwSetMouseWheelCallback`.
 *
 *  @ingroup input
 */
scrollFunc glfwSetScrollCallback(plafWindow* window, scrollFunc callback);

/*! @brief Sets the path drop callback.
 *
 *  This function sets the path drop callback of the specified window, which is
 *  called when one or more dragged paths are dropped on the window.
 *
 *  Because the path array and its strings may have been generated specifically
 *  for that event, they are not guaranteed to be valid after the callback has
 *  returned.  If you wish to use them after the callback returns, you need to
 *  make a deep copy.
 *
 *  @param[in] window The window whose callback to set.
 *  @param[in] callback The new file drop callback, or `NULL` to remove the
 *  currently set callback.
 *  @return The previously set callback, or `NULL` if no callback was set or the
 *  library had not been [initialized](@ref intro_init).
 *
 *  @callback_signature
 *  @code
 *  void function_name(plafWindow* window, int path_count, const char* paths[])
 *  @endcode
 *  For more information about the callback parameters, see the
 *  [function pointer type](@ref dropFunc).
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref path_drop
 *
 *  @since Added in version 3.1.
 *
 *  @ingroup input
 */
dropFunc glfwSetDropCallback(plafWindow* window, dropFunc callback);

/*! @brief Sets the clipboard to the specified string.
 *
 *  This function sets the system clipboard to the specified, UTF-8 encoded
 *  string.
 *
 *  @param[in] string A UTF-8 encoded string.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @remark __Win32:__ The clipboard on Windows has a single global lock for reading and
 *  writing.  GLFW tries to acquire it a few times, which is almost always enough.  If it
 *  cannot acquire the lock then this function emits @ref ERR_PLATFORM_ERROR and returns.
 *  It is safe to try this multiple times.
 *
 *  @pointer_lifetime The specified string is copied before this function
 *  returns.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref clipboard
 *  @sa @ref glfwGetClipboardString
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup input
 */
void setClipboardString(const char* string);

/*! @brief Returns the contents of the clipboard as a string.
 *
 *  This function returns the contents of the system clipboard, if it contains
 *  or is convertible to a UTF-8 encoded string.  If the clipboard is empty or
 *  if its contents cannot be converted, `NULL` is returned and a @ref
 *  ERR_FORMAT_UNAVAILABLE error is generated.
 *
 *  @return The contents of the clipboard as a UTF-8 encoded string, or `NULL`
 *  if an [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_FORMAT_UNAVAILABLE and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark __Win32:__ The clipboard on Windows has a single global lock for reading and
 *  writing.  GLFW tries to acquire it a few times, which is almost always enough.  If it
 *  cannot acquire the lock then this function emits @ref ERR_PLATFORM_ERROR and returns.
 *  It is safe to try this multiple times.
 *
 *  @pointer_lifetime The returned string is allocated and freed by GLFW.  You
 *  should not free it yourself.  It is valid until the next call to @ref
 *  glfwGetClipboardString or @ref glfwSetClipboardString, or until the library
 *  is terminated.
 *
 *  @thread_safety This function must only be called from the main thread.
 *
 *  @sa @ref clipboard
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup input
 */
const char* getClipboardString(void);

/*! @brief Makes the context of the specified window current for the calling
 *  thread.
 *
 *  This function makes the OpenGL or OpenGL ES context of the specified window
 *  current on the calling thread.  It can also detach the current context from
 *  the calling thread without making a new one current by passing in `NULL`.
 *
 *  A context must only be made current on a single thread at a time and each
 *  thread can have only a single current context at a time.  Making a context
 *  current detaches any previously current context on the calling thread.
 *
 *  When moving a context between threads, you must detach it (make it
 *  non-current) on the old thread before making it current on the new one.
 *
 *  By default, making a context non-current implicitly forces a pipeline flush.
 *  On machines that support `GL_KHR_context_flush_control`, you can control
 *  whether a context performs this flush by setting the
 *  [WINDOW_ATTR_HINT_CONTEXT_RELEASE_BEHAVIOR](@ref GLFW_CONTEXT_RELEASE_BEHAVIOR_hint)
 *  hint.
 *
 *  The specified window must have an OpenGL or OpenGL ES context.  Specifying
 *  a window without a context will generate a @ref ERR_NO_WINDOW_CONTEXT
 *  error.
 *
 *  @param[in] window The window whose context to make current, or `NULL` to
 *  detach the current context.
 *
 *  @remarks If the previously current context was created via a different
 *  context creation API than the one passed to this function, GLFW will still
 *  detach the previous one from its API before making the new one current.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_NO_WINDOW_CONTEXT and @ref ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref context_current
 *  @sa @ref glfwGetCurrentContext
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup context
 */
void glfwMakeContextCurrent(plafWindow* window);

/*! @brief Returns the window whose context is current on the calling thread.
 *
 *  This function returns the window whose OpenGL or OpenGL ES context is
 *  current on the calling thread.
 *
 *  @return The window whose context is current, or `NULL` if no window's
 *  context is current.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref context_current
 *  @sa @ref glfwMakeContextCurrent
 *
 *  @since Added in version 3.0.
 *
 *  @ingroup context
 */
plafWindow* glfwGetCurrentContext(void);

/*! @brief Swaps the front and back buffers of the specified window.
 *
 *  This function swaps the front and back buffers of the specified window when
 *  rendering with OpenGL.  If the swap interval is greater than
 *  zero, the GPU driver waits the specified number of screen updates before
 *  swapping the buffers.
 *
 *  The specified window must have an OpenGL context.  Specifying
 *  a window without a context will generate a @ref ERR_NO_WINDOW_CONTEXT
 *  error.
 *
 *  @param[in] window The window whose buffers to swap.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_NO_WINDOW_CONTEXT and @ref ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref buffer_swap
 *  @sa @ref glfwSwapInterval
 *
 *  @since Added in version 1.0.
 *  __GLFW 3:__ Added window handle parameter.
 *
 *  @ingroup window
 */
void glfwSwapBuffers(plafWindow* window);

/*! @brief Sets the swap interval for the current context.
 *
 *  This function sets the swap interval for the current OpenGL or OpenGL ES
 *  context, i.e. the number of screen updates to wait from the time @ref
 *  glfwSwapBuffers was called before swapping the buffers and returning.  This
 *  is sometimes called _vertical synchronization_, _vertical retrace
 *  synchronization_ or just _vsync_.
 *
 *  A context that supports either of the `WGL_EXT_swap_control_tear` and
 *  `GLX_EXT_swap_control_tear` extensions also accepts _negative_ swap
 *  intervals, which allows the driver to swap immediately even if a frame
 *  arrives a little bit late.  You can check for these extensions with @ref
 *  glfwExtensionSupported.
 *
 *  A context must be current on the calling thread.  Calling this function
 *  without a current context will cause a @ref ERR_NO_CURRENT_CONTEXT error.
 *
 *  @param[in] interval The minimum number of screen updates to wait for
 *  until the buffers are swapped by @ref glfwSwapBuffers.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_NO_CURRENT_CONTEXT and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark This function is not called during context creation, leaving the
 *  swap interval set to whatever is the default for that API.  This is done
 *  because some swap interval extensions used by GLFW do not allow the swap
 *  interval to be reset to zero once it has been set to a non-zero value.
 *
 *  @remark Some GPU drivers do not honor the requested swap interval, either
 *  because of a user setting that overrides the application's request or due to
 *  bugs in the driver.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref buffer_swap
 *  @sa @ref glfwSwapBuffers
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup context
 */
void glfwSwapInterval(int interval);

/*! @brief Returns whether the specified extension is available.
 *
 *  This function returns whether the specified
 *  [API extension](@ref context_glext) is supported by the current OpenGL or
 *  OpenGL ES context.  It searches both for client API extension and context
 *  creation API extensions.
 *
 *  A context must be current on the calling thread.  Calling this function
 *  without a current context will cause a @ref ERR_NO_CURRENT_CONTEXT error.
 *
 *  As this functions retrieves and searches one or more extension strings each
 *  call, it is recommended that you cache its results if it is going to be used
 *  frequently.  The extension strings will not change during the lifetime of
 *  a context, so there is no danger in doing this.
 *
 *  @param[in] extension The ASCII encoded name of the extension.
 *  @return `true` if the extension is available, or `false`
 *  otherwise.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_NO_CURRENT_CONTEXT, @ref ERR_INVALID_VALUE and @ref
 *  ERR_PLATFORM_ERROR.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref context_glext
 *  @sa @ref glfwGetProcAddress
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup context
 */
int glfwExtensionSupported(const char* extension);

/*! @brief Returns the address of the specified function for the current
 *  context.
 *
 *  This function returns the address of the specified OpenGL or OpenGL ES
 *  [core or extension function](@ref context_glext), if it is supported
 *  by the current context.
 *
 *  A context must be current on the calling thread.  Calling this function
 *  without a current context will cause a @ref ERR_NO_CURRENT_CONTEXT error.
 *
 *  @param[in] procname The ASCII encoded name of the function.
 *  @return The address of the function, or `NULL` if an
 *  [error](@ref error_handling) occurred.
 *
 *  @errors Possible errors include @ref ERR_NOT_INITIALIZED, @ref
 *  ERR_NO_CURRENT_CONTEXT and @ref ERR_PLATFORM_ERROR.
 *
 *  @remark The address of a given function is not guaranteed to be the same
 *  between contexts.
 *
 *  @remark This function may return a non-`NULL` address despite the
 *  associated version or extension not being available.  Always check the
 *  context version or extension string first.
 *
 *  @pointer_lifetime The returned function pointer is valid until the context
 *  is destroyed or the library is terminated.
 *
 *  @thread_safety This function may be called from any thread.
 *
 *  @sa @ref context_glext
 *  @sa @ref glfwExtensionSupported
 *
 *  @since Added in version 1.0.
 *
 *  @ingroup context
 */
glFunc glfwGetProcAddress(const char* procname);





// --------- Internal API below ---------

void _terminate(void);

ErrorResponse* platformInit(_GLFWplatform* platform);
void platformTerminate(void);

//////////////////////////////////////////////////////////////////////////
//////                       GLFW platform API                      //////
//////////////////////////////////////////////////////////////////////////

void* _glfwPlatformLoadModule(const char* path);
void _glfwPlatformFreeModule(void* module);
moduleFunc _glfwPlatformGetModuleSymbol(void* module, const char* name);


//////////////////////////////////////////////////////////////////////////
//////                         GLFW event API                       //////
//////////////////////////////////////////////////////////////////////////

void _glfwInputWindowFocus(plafWindow* window, IntBool focused);
void _glfwInputWindowPos(plafWindow* window, int xpos, int ypos);
void _glfwInputWindowSize(plafWindow* window, int width, int height);
void _glfwInputFramebufferSize(plafWindow* window, int width, int height);
void _glfwInputWindowContentScale(plafWindow* window,
								  float xscale, float yscale);
void _glfwInputWindowIconify(plafWindow* window, IntBool iconified);
void _glfwInputWindowMaximize(plafWindow* window, IntBool maximized);
void _glfwInputWindowDamage(plafWindow* window);
void _glfwInputWindowCloseRequest(plafWindow* window);
void _glfwInputWindowMonitor(plafWindow* window, plafMonitor* monitor);

void _glfwInputKey(plafWindow* window,
				   int key, int scancode, int action, int mods);
void _glfwInputChar(plafWindow* window,
					uint32_t codepoint, int mods, IntBool plain);
void _glfwInputScroll(plafWindow* window, double xoffset, double yoffset);
void _glfwInputMouseClick(plafWindow* window, int button, int action, int mods);
void _glfwInputCursorPos(plafWindow* window, double xpos, double ypos);
void _glfwInputCursorEnter(plafWindow* window, IntBool entered);
void _glfwInputDrop(plafWindow* window, int count, const char** names);

void _glfwInputMonitor(plafMonitor* monitor, int action, int placement);
void _glfwInputMonitorWindow(plafMonitor* monitor, plafWindow* window);

#if defined(__GNUC__)
void _glfwInputError(int code, const char* format, ...)
	__attribute__((format(printf, 2, 3)));
ErrorResponse* createErrorResponse(int code, const char* format, ...) __attribute__((format(printf, 2, 3)));
#else
void _glfwInputError(int code, const char* format, ...);
ErrorResponse* createErrorResponse(int code, const char* format, ...);
#endif


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

IntBool _glfwGetGammaRamp(plafMonitor* monitor, GammaRamp* ramp);
void _glfwSetGammaRamp(plafMonitor* monitor, const GammaRamp* ramp);
IntBool _glfwGetVideoMode(plafMonitor* monitor, VideoMode* mode);
VideoMode* _glfwGetVideoModes(plafMonitor* monitor, int* count);
void _glfwSetVideoMode(plafMonitor* monitor, const VideoMode* desired);
void _glfwDestroyCursor(plafCursor* cursor);
IntBool _glfwCreateStandardCursor(plafCursor* cursor, int shape);
void glfwSetCursorMode(plafWindow* window, int mode);
IntBool _glfwCreateCursor(plafCursor* cursor, const ImageData* image, int xhot, int yhot);
IntBool _glfwStringInExtensionString(const char* string, const char* extensions);
const plafFrameBufferCfg* _glfwChooseFBConfig(const plafFrameBufferCfg* desired, const plafFrameBufferCfg* alternatives, unsigned int count);
IntBool _glfwRefreshContextAttribs(plafWindow* window, const plafCtxCfg* ctxconfig);
IntBool _glfwIsValidContextConfig(const plafCtxCfg* ctxconfig);

const VideoMode* _glfwChooseVideoMode(plafMonitor* monitor, const VideoMode* desired);
int _glfwCompareVideoModes(const VideoMode* first, const VideoMode* second);
plafMonitor* _glfwAllocMonitor(const char* name, int widthMM, int heightMM);
void _glfwFreeMonitor(plafMonitor* monitor);
void _glfwAllocGammaArrays(GammaRamp* ramp, unsigned int size);
void _glfwFreeGammaArrays(GammaRamp* ramp);
void _glfwSplitBPP(int bpp, int* red, int* green, int* blue);

void _glfwCenterCursorInContentArea(plafWindow* window);

size_t _glfwEncodeUTF8(char* s, uint32_t codepoint);
char** _glfwParseUriList(char* text, int* count);

char* _glfw_strdup(const char* src);
int _glfw_min(int a, int b);
int _glfw_max(int a, int b);

void* _glfw_calloc(size_t count, size_t size);
void* _glfw_realloc(void* pointer, size_t size);
void _glfw_free(void* pointer);

void _glfwTerminateGLX(void);

void updateCursorImage(plafWindow* window);
void _glfwSetCursor(plafWindow* window);
void _glfwSetCursorPos(plafWindow* window, double xpos, double ypos);
#if defined(__APPLE__) || defined(_WIN32)
IntBool cursorInContentArea(plafWindow* window);
#endif

#if defined(__APPLE__)
IntBool _glfwCreateWindowCocoa(plafWindow* window, const WindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig);
void _glfwDestroyWindowCocoa(plafWindow* window);
void _glfwSetWindowTitleCocoa(plafWindow* window, const char* title);
void _glfwSetWindowIconCocoa(plafWindow* window, int count, const ImageData* images);
void _glfwGetWindowPosCocoa(plafWindow* window, int* xpos, int* ypos);
void _glfwSetWindowPosCocoa(plafWindow* window, int xpos, int ypos);
void _glfwGetWindowSizeCocoa(plafWindow* window, int* width, int* height);
void _glfwSetWindowSizeCocoa(plafWindow* window, int width, int height);
void _glfwSetWindowSizeLimitsCocoa(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight);
void _glfwSetWindowAspectRatioCocoa(plafWindow* window, int numer, int denom);
void _glfwGetFramebufferSizeCocoa(plafWindow* window, int* width, int* height);
void _glfwGetWindowFrameSizeCocoa(plafWindow* window, int* left, int* top, int* right, int* bottom);
void _glfwGetWindowContentScaleCocoa(plafWindow* window, float* xscale, float* yscale);
void _glfwIconifyWindowCocoa(plafWindow* window);
void _glfwRestoreWindowCocoa(plafWindow* window);
void _glfwMaximizeWindowCocoa(plafWindow* window);
void _glfwShowWindowCocoa(plafWindow* window);
void _glfwHideWindowCocoa(plafWindow* window);
void _glfwRequestWindowAttentionCocoa(plafWindow* window);
void _glfwFocusWindowCocoa(plafWindow* window);
void _glfwSetWindowMonitorCocoa(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate);
IntBool _glfwWindowFocusedCocoa(plafWindow* window);
IntBool _glfwWindowIconifiedCocoa(plafWindow* window);
IntBool _glfwWindowVisibleCocoa(plafWindow* window);
IntBool _glfwWindowMaximizedCocoa(plafWindow* window);
IntBool _glfwWindowHoveredCocoa(plafWindow* window);
IntBool _glfwFramebufferTransparentCocoa(plafWindow* window);
void _glfwSetWindowResizableCocoa(plafWindow* window, IntBool enabled);
void _glfwSetWindowDecoratedCocoa(plafWindow* window, IntBool enabled);
void _glfwSetWindowFloatingCocoa(plafWindow* window, IntBool enabled);
float _glfwGetWindowOpacityCocoa(plafWindow* window);
void _glfwSetWindowOpacityCocoa(plafWindow* window, float opacity);
void _glfwSetWindowMousePassthroughCocoa(plafWindow* window, IntBool enabled);

void _glfwPollEventsCocoa(void);
void _glfwWaitEventsCocoa(void);
void _glfwWaitEventsTimeoutCocoa(double timeout);
void _glfwPostEmptyEventCocoa(void);

void _glfwPollMonitorsCocoa(void);
void _glfwRestoreVideoModeCocoa(plafMonitor* monitor);

float _glfwTransformYCocoa(float y);

IntBool _glfwInitNSGL(void);
void _glfwTerminateNSGL(void);
IntBool _glfwCreateContextNSGL(plafWindow* window, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig);
void _glfwDestroyContextNSGL(plafWindow* window);
#elif defined(__linux__)
IntBool _glfwCreateWindowX11(plafWindow* window, const WindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig);
void _glfwDestroyWindowX11(plafWindow* window);
void _glfwSetWindowTitleX11(plafWindow* window, const char* title);
void _glfwSetWindowIconX11(plafWindow* window, int count, const ImageData* images);
void _glfwGetWindowPosX11(plafWindow* window, int* xpos, int* ypos);
void _glfwSetWindowPosX11(plafWindow* window, int xpos, int ypos);
void _glfwGetWindowSizeX11(plafWindow* window, int* width, int* height);
void _glfwSetWindowSizeX11(plafWindow* window, int width, int height);
void _glfwSetWindowSizeLimitsX11(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight);
void _glfwSetWindowAspectRatioX11(plafWindow* window, int numer, int denom);
void _glfwGetFramebufferSizeX11(plafWindow* window, int* width, int* height);
void _glfwGetWindowFrameSizeX11(plafWindow* window, int* left, int* top, int* right, int* bottom);
void _glfwGetWindowContentScaleX11(plafWindow* window, float* xscale, float* yscale);
void _glfwIconifyWindowX11(plafWindow* window);
void _glfwRestoreWindowX11(plafWindow* window);
void _glfwMaximizeWindowX11(plafWindow* window);
void _glfwShowWindowX11(plafWindow* window);
void _glfwHideWindowX11(plafWindow* window);
void _glfwRequestWindowAttentionX11(plafWindow* window);
void _glfwFocusWindowX11(plafWindow* window);
void _glfwSetWindowMonitorX11(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate);
IntBool _glfwWindowFocusedX11(plafWindow* window);
IntBool _glfwWindowIconifiedX11(plafWindow* window);
IntBool _glfwWindowVisibleX11(plafWindow* window);
IntBool _glfwWindowMaximizedX11(plafWindow* window);
IntBool _glfwWindowHoveredX11(plafWindow* window);
IntBool _glfwFramebufferTransparentX11(plafWindow* window);
void _glfwSetWindowResizableX11(plafWindow* window, IntBool enabled);
void _glfwSetWindowDecoratedX11(plafWindow* window, IntBool enabled);
void _glfwSetWindowFloatingX11(plafWindow* window, IntBool enabled);
float _glfwGetWindowOpacityX11(plafWindow* window);
void _glfwSetWindowOpacityX11(plafWindow* window, float opacity);
void _glfwSetWindowMousePassthroughX11(plafWindow* window, IntBool enabled);

void _glfwPollEventsX11(void);
void _glfwWaitEventsX11(void);
void _glfwWaitEventsTimeoutX11(double timeout);
void _glfwPostEmptyEventX11(void);

void _glfwPollMonitorsX11(void);
void _glfwRestoreVideoModeX11(plafMonitor* monitor);

Cursor _glfwCreateNativeCursorX11(const ImageData* image, int xhot, int yhot);

unsigned long _glfwGetWindowPropertyX11(Window window,
										Atom property,
										Atom type,
										unsigned char** value);
IntBool _glfwIsVisualTransparentX11(Visual* visual);

void _glfwGrabErrorHandlerX11(void);
void _glfwReleaseErrorHandlerX11(void);
void _glfwInputErrorX11(int error, const char* message);

void _glfwPushSelectionToManagerX11(void);
void _glfwCreateInputContextX11(plafWindow* window);

IntBool _glfwInitGLX(void);
IntBool _glfwCreateContextGLX(plafWindow* window,
							   const plafCtxCfg* ctxconfig,
							   const plafFrameBufferCfg* fbconfig);
void _glfwDestroyContextGLX(plafWindow* window);
IntBool _glfwChooseVisualGLX(const WindowConfig* wndconfig,
							  const plafCtxCfg* ctxconfig,
							  const plafFrameBufferCfg* fbconfig,
							  Visual** visual, int* depth);

IntBool waitForX11Event(double timeout);
#elif defined(_WIN32)
WCHAR* _glfwCreateWideStringFromUTF8Win32(const char* src);
char* _glfwCreateUTF8FromWideStringWin32(const WCHAR* src);
BOOL _glfwIsWindowsVersionOrGreaterWin32(WORD major, WORD minor, WORD sp);
BOOL IsWindows10BuildOrGreater(WORD build);
void _glfwInputErrorWin32(int error, const char* description);

void _glfwPollMonitorsWin32(void);
void _glfwRestoreVideoModeWin32(plafMonitor* monitor);
void _glfwGetHMONITORContentScaleWin32(HMONITOR handle, float* xscale, float* yscale);

IntBool _glfwCreateWindowWin32(plafWindow* window, const WindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig);
void _glfwDestroyWindowWin32(plafWindow* window);
void _glfwSetWindowTitleWin32(plafWindow* window, const char* title);
void _glfwSetWindowIconWin32(plafWindow* window, int count, const ImageData* images);
void _glfwGetWindowPosWin32(plafWindow* window, int* xpos, int* ypos);
void _glfwSetWindowPosWin32(plafWindow* window, int xpos, int ypos);
void _glfwGetWindowSizeWin32(plafWindow* window, int* width, int* height);
void _glfwSetWindowSizeWin32(plafWindow* window, int width, int height);
void _glfwSetWindowSizeLimitsWin32(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight);
void _glfwSetWindowAspectRatioWin32(plafWindow* window, int numer, int denom);
void _glfwGetFramebufferSizeWin32(plafWindow* window, int* width, int* height);
void _glfwGetWindowFrameSizeWin32(plafWindow* window, int* left, int* top, int* right, int* bottom);
void _glfwGetWindowContentScaleWin32(plafWindow* window, float* xscale, float* yscale);
void _glfwIconifyWindowWin32(plafWindow* window);
void _glfwRestoreWindowWin32(plafWindow* window);
void _glfwMaximizeWindowWin32(plafWindow* window);
void _glfwShowWindowWin32(plafWindow* window);
void _glfwHideWindowWin32(plafWindow* window);
void _glfwRequestWindowAttentionWin32(plafWindow* window);
void _glfwFocusWindowWin32(plafWindow* window);
void _glfwSetWindowMonitorWin32(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate);
IntBool _glfwWindowFocusedWin32(plafWindow* window);
IntBool _glfwWindowIconifiedWin32(plafWindow* window);
IntBool _glfwWindowVisibleWin32(plafWindow* window);
IntBool _glfwWindowMaximizedWin32(plafWindow* window);
IntBool _glfwWindowHoveredWin32(plafWindow* window);
IntBool _glfwFramebufferTransparentWin32(plafWindow* window);
void _glfwSetWindowResizableWin32(plafWindow* window, IntBool enabled);
void _glfwSetWindowDecoratedWin32(plafWindow* window, IntBool enabled);
void _glfwSetWindowFloatingWin32(plafWindow* window, IntBool enabled);
void _glfwSetWindowMousePassthroughWin32(plafWindow* window, IntBool enabled);
float _glfwGetWindowOpacityWin32(plafWindow* window);
void _glfwSetWindowOpacityWin32(plafWindow* window, float opacity);

void _glfwPollEventsWin32(void);
void _glfwWaitEventsWin32(void);
void _glfwWaitEventsTimeoutWin32(double timeout);
void _glfwPostEmptyEventWin32(void);

IntBool _glfwInitWGL(void);
void _glfwTerminateWGL(void);
IntBool _glfwCreateContextWGL(plafWindow* window, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig);
#endif

#ifdef __cplusplus
}
#endif

#endif
