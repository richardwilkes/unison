#if defined(__linux__)
#include <unistd.h>
#include <signal.h>
#include <stdint.h>

#include <X11/Xlib.h>
#include <X11/keysym.h>
#include <X11/Xatom.h>
#include <X11/Xresource.h>
#include <X11/Xcursor/Xcursor.h>

// The XRandR extension provides mode setting and gamma control
#include <X11/extensions/Xrandr.h>

// The Xkb extension provides improved keyboard support
#include <X11/XKBlib.h>

// The Xinerama extension provides legacy monitor indices
#include <X11/extensions/Xinerama.h>

// The XInput extension provides raw mouse motion input
#include <X11/extensions/XInput2.h>

// The Shape extension provides custom window shapes
#include <X11/extensions/shape.h>

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

typedef XID GLXWindow;
typedef XID GLXDrawable;
typedef struct __GLXFBConfig* GLXFBConfig;
typedef struct __GLXcontext* GLXContext;
typedef void (*__GLXextproc)(void);

typedef XClassHint* (* PFN_XAllocClassHint)(void);
typedef XSizeHints* (* PFN_XAllocSizeHints)(void);
typedef XWMHints* (* PFN_XAllocWMHints)(void);
typedef int (* PFN_XChangeProperty)(Display*,Window,Atom,Atom,int,int,const unsigned char*,int);
typedef int (* PFN_XChangeWindowAttributes)(Display*,Window,unsigned long,XSetWindowAttributes*);
typedef Bool (* PFN_XCheckIfEvent)(Display*,XEvent*,Bool(*)(Display*,XEvent*,XPointer),XPointer);
typedef Bool (* PFN_XCheckTypedWindowEvent)(Display*,Window,int,XEvent*);
typedef int (* PFN_XCloseDisplay)(Display*);
typedef Status (* PFN_XCloseIM)(XIM);
typedef int (* PFN_XConvertSelection)(Display*,Atom,Atom,Atom,Window,Time);
typedef Colormap (* PFN_XCreateColormap)(Display*,Window,Visual*,int);
typedef Cursor (* PFN_XCreateFontCursor)(Display*,unsigned int);
typedef XIC (* PFN_XCreateIC)(XIM,...);
typedef Region (* PFN_XCreateRegion)(void);
typedef Window (* PFN_XCreateWindow)(Display*,Window,int,int,unsigned int,unsigned int,unsigned int,int,unsigned int,Visual*,unsigned long,XSetWindowAttributes*);
typedef int (* PFN_XDefineCursor)(Display*,Window,Cursor);
typedef int (* PFN_XDeleteContext)(Display*,XID,XContext);
typedef int (* PFN_XDeleteProperty)(Display*,Window,Atom);
typedef void (*PFN_XDestroyIC)(XIC);
typedef int (* PFN_XDestroyRegion)(Region);
typedef int (* PFN_XDestroyWindow)(Display*,Window);
typedef int (* PFN_XDisplayKeycodes)(Display*,int*,int*);
typedef int (* PFN_XEventsQueued)(Display*,int);
typedef Bool (* PFN_XFilterEvent)(XEvent*,Window);
typedef int (* PFN_XFindContext)(Display*,XID,XContext,XPointer*);
typedef int (* PFN_XFlush)(Display*);
typedef int (* PFN_XFree)(void*);
typedef int (* PFN_XFreeColormap)(Display*,Colormap);
typedef int (* PFN_XFreeCursor)(Display*,Cursor);
typedef void (*PFN_XFreeEventData)(Display*,XGenericEventCookie*);
typedef int (* PFN_XGetErrorText)(Display*,int,char*,int);
typedef char* (* PFN_XGetICValues)(XIC,...);
typedef char* (* PFN_XGetIMValues)(XIM,...);
typedef int (* PFN_XGetInputFocus)(Display*,Window*,int*);
typedef KeySym* (* PFN_XGetKeyboardMapping)(Display*,KeyCode,int,int*);
typedef int (* PFN_XGetScreenSaver)(Display*,int*,int*,int*,int*);
typedef Window (* PFN_XGetSelectionOwner)(Display*,Atom);
typedef Status (* PFN_XGetWMNormalHints)(Display*,Window,XSizeHints*,long*);
typedef Status (* PFN_XGetWindowAttributes)(Display*,Window,XWindowAttributes*);
typedef int (* PFN_XGetWindowProperty)(Display*,Window,Atom,long,long,Bool,Atom,Atom*,int*,unsigned long*,unsigned long*,unsigned char**);
typedef Status (* PFN_XIconifyWindow)(Display*,Window,int);
typedef Status (* PFN_XInitThreads)(void);
typedef Atom (* PFN_XInternAtom)(Display*,const char*,Bool);
typedef int (* PFN_XLookupString)(XKeyEvent*,char*,int,KeySym*,XComposeStatus*);
typedef int (* PFN_XMapRaised)(Display*,Window);
typedef int (* PFN_XMapWindow)(Display*,Window);
typedef int (* PFN_XMoveResizeWindow)(Display*,Window,int,int,unsigned int,unsigned int);
typedef int (* PFN_XMoveWindow)(Display*,Window,int,int);
typedef int (* PFN_XNextEvent)(Display*,XEvent*);
typedef Display* (* PFN_XOpenDisplay)(const char*);
typedef XIM (* PFN_XOpenIM)(Display*,XrmDatabase*,char*,char*);
typedef int (* PFN_XPeekEvent)(Display*,XEvent*);
typedef int (* PFN_XPending)(Display*);
typedef Bool (* PFN_XQueryExtension)(Display*,const char*,int*,int*,int*);
typedef Bool (* PFN_XQueryPointer)(Display*,Window,Window*,Window*,int*,int*,int*,int*,unsigned int*);
typedef int (* PFN_XRaiseWindow)(Display*,Window);
typedef Bool (* PFN_XRegisterIMInstantiateCallback)(Display*,void*,char*,char*,XIDProc,XPointer);
typedef int (* PFN_XResizeWindow)(Display*,Window,unsigned int,unsigned int);
typedef char* (* PFN_XResourceManagerString)(Display*);
typedef int (* PFN_XSaveContext)(Display*,XID,XContext,const char*);
typedef int (* PFN_XSelectInput)(Display*,Window,long);
typedef Status (* PFN_XSendEvent)(Display*,Window,Bool,long,XEvent*);
typedef XErrorHandler (* PFN_XSetErrorHandler)(XErrorHandler);
typedef void (*PFN_XSetICFocus)(XIC);
typedef char* (* PFN_XSetIMValues)(XIM,...);
typedef int (* PFN_XSetInputFocus)(Display*,Window,int,Time);
typedef char* (* PFN_XSetLocaleModifiers)(const char*);
typedef int (* PFN_XSetScreenSaver)(Display*,int,int,int,int);
typedef int (* PFN_XSetSelectionOwner)(Display*,Atom,Window,Time);
typedef int (* PFN_XSetWMHints)(Display*,Window,XWMHints*);
typedef void (*PFN_XSetWMNormalHints)(Display*,Window,XSizeHints*);
typedef Status (* PFN_XSetWMProtocols)(Display*,Window,Atom*,int);
typedef Bool (* PFN_XSupportsLocale)(void);
typedef int (* PFN_XSync)(Display*,Bool);
typedef Bool (* PFN_XTranslateCoordinates)(Display*,Window,Window,int,int,int*,int*,Window*);
typedef int (* PFN_XUndefineCursor)(Display*,Window);
typedef int (* PFN_XUnmapWindow)(Display*,Window);
typedef void (*PFN_XUnsetICFocus)(XIC);
typedef int (* PFN_XWarpPointer)(Display*,Window,Window,int,int,unsigned int,unsigned int,int,int);
typedef void (*PFN_XkbFreeKeyboard)(XkbDescPtr,unsigned int,Bool);
typedef void (*PFN_XkbFreeNames)(XkbDescPtr,unsigned int,Bool);
typedef XkbDescPtr (* PFN_XkbGetMap)(Display*,unsigned int,unsigned int);
typedef Status (* PFN_XkbGetNames)(Display*,unsigned int,XkbDescPtr);
typedef Status (* PFN_XkbGetState)(Display*,unsigned int,XkbStatePtr);
typedef Bool (* PFN_XkbQueryExtension)(Display*,int*,int*,int*,int*,int*);
typedef Bool (* PFN_XkbSelectEventDetails)(Display*,unsigned int,unsigned int,unsigned long,unsigned long);
typedef Bool (* PFN_XkbSetDetectableAutoRepeat)(Display*,Bool,Bool*);
typedef void (*PFN_XrmDestroyDatabase)(XrmDatabase);
typedef Bool (* PFN_XrmGetResource)(XrmDatabase,const char*,const char*,char**,XrmValue*);
typedef XrmDatabase (* PFN_XrmGetStringDatabase)(const char*);
typedef void (*PFN_XrmInitialize)(void);
typedef Bool (* PFN_XUnregisterIMInstantiateCallback)(Display*,void*,char*,char*,XIDProc,XPointer);
typedef int (* PFN_Xutf8LookupString)(XIC,XKeyPressedEvent*,char*,int,KeySym*,Status*);
typedef void (*PFN_Xutf8SetWMProperties)(Display*,Window,const char*,const char*,char**,int,XSizeHints*,XWMHints*,XClassHint*);

typedef XRRCrtcGamma* (* PFN_XRRAllocGamma)(int);
typedef void (*PFN_XRRFreeCrtcInfo)(XRRCrtcInfo*);
typedef void (*PFN_XRRFreeGamma)(XRRCrtcGamma*);
typedef void (*PFN_XRRFreeOutputInfo)(XRROutputInfo*);
typedef void (*PFN_XRRFreeScreenResources)(XRRScreenResources*);
typedef XRRCrtcGamma* (* PFN_XRRGetCrtcGamma)(Display*,RRCrtc);
typedef int (* PFN_XRRGetCrtcGammaSize)(Display*,RRCrtc);
typedef XRRCrtcInfo* (* PFN_XRRGetCrtcInfo) (Display*,XRRScreenResources*,RRCrtc);
typedef XRROutputInfo* (* PFN_XRRGetOutputInfo)(Display*,XRRScreenResources*,RROutput);
typedef RROutput (* PFN_XRRGetOutputPrimary)(Display*,Window);
typedef XRRScreenResources* (* PFN_XRRGetScreenResourcesCurrent)(Display*,Window);
typedef Bool (* PFN_XRRQueryExtension)(Display*,int*,int*);
typedef Status (* PFN_XRRQueryVersion)(Display*,int*,int*);
typedef void (*PFN_XRRSelectInput)(Display*,Window,int);
typedef Status (* PFN_XRRSetCrtcConfig)(Display*,XRRScreenResources*,RRCrtc,Time,int,int,RRMode,Rotation,RROutput*,int);
typedef void (*PFN_XRRSetCrtcGamma)(Display*,RRCrtc,XRRCrtcGamma*);
typedef int (* PFN_XRRUpdateConfiguration)(XEvent*);

typedef XcursorImage* (* PFN_XcursorImageCreate)(int,int);
typedef void (*PFN_XcursorImageDestroy)(XcursorImage*);
typedef Cursor (* PFN_XcursorImageLoadCursor)(Display*,const XcursorImage*);
typedef char* (* PFN_XcursorGetTheme)(Display*);
typedef int (* PFN_XcursorGetDefaultSize)(Display*);
typedef XcursorImage* (* PFN_XcursorLibraryLoadImage)(const char*,const char*,int);

typedef Bool (* PFN_XineramaIsActive)(Display*);
typedef Bool (* PFN_XineramaQueryExtension)(Display*,int*,int*);
typedef XineramaScreenInfo* (* PFN_XineramaQueryScreens)(Display*,int*);

typedef Bool (* PFN_XF86VidModeQueryExtension)(Display*,int*,int*);
typedef Bool (* PFN_XF86VidModeGetGammaRamp)(Display*,int,int,unsigned short*,unsigned short*,unsigned short*);
typedef Bool (* PFN_XF86VidModeSetGammaRamp)(Display*,int,int,unsigned short*,unsigned short*,unsigned short*);
typedef Bool (* PFN_XF86VidModeGetGammaRampSize)(Display*,int,int*);

typedef Status (* PFN_XIQueryVersion)(Display*,int*,int*);

typedef Bool (* PFN_XRenderQueryExtension)(Display*,int*,int*);
typedef Status (* PFN_XRenderQueryVersion)(Display*dpy,int*,int*);
typedef XRenderPictFormat* (* PFN_XRenderFindVisualFormat)(Display*,Visual const*);

typedef Bool (* PFN_XShapeQueryExtension)(Display*,int*,int*);
typedef Status (* PFN_XShapeQueryVersion)(Display*dpy,int*,int*);
typedef void (*PFN_XShapeCombineRegion)(Display*,Window,int,int,int,Region,int);
typedef void (*PFN_XShapeCombineMask)(Display*,Window,int,int,int,Pixmap,int);

typedef int (*PFNGLXGETFBCONFIGATTRIBPROC)(Display*,GLXFBConfig,int,int*);
typedef const char* (*PFNGLXGETCLIENTSTRINGPROC)(Display*,int);
typedef Bool (*PFNGLXQUERYEXTENSIONPROC)(Display*,int*,int*);
typedef Bool (*PFNGLXQUERYVERSIONPROC)(Display*,int*,int*);
typedef void (*PFNGLXDESTROYCONTEXTPROC)(Display*,GLXContext);
typedef Bool (*PFNGLXMAKECURRENTPROC)(Display*,GLXDrawable,GLXContext);
typedef void (*PFNGLXSWAPBUFFERSPROC)(Display*,GLXDrawable);
typedef const char* (*PFNGLXQUERYEXTENSIONSSTRINGPROC)(Display*,int);
typedef GLXFBConfig* (*PFNGLXGETFBCONFIGSPROC)(Display*,int,int*);
typedef GLXContext (*PFNGLXCREATENEWCONTEXTPROC)(Display*,GLXFBConfig,int,GLXContext,Bool);
typedef __GLXextproc (* PFNGLXGETPROCADDRESSPROC)(const GLubyte *procName);
typedef void (*PFNGLXSWAPINTERVALEXTPROC)(Display*,GLXDrawable,int);
typedef XVisualInfo* (*PFNGLXGETVISUALFROMFBCONFIGPROC)(Display*,GLXFBConfig);
typedef GLXWindow (*PFNGLXCREATEWINDOWPROC)(Display*,GLXFBConfig,Window,const int*);
typedef void (*PFNGLXDESTROYWINDOWPROC)(Display*,GLXWindow);

typedef int (*PFNGLXSWAPINTERVALSGIPROC)(int);
typedef GLXContext (*PFNGLXCREATECONTEXTATTRIBSARBPROC)(Display*,GLXFBConfig,GLXContext,Bool,const int*);

#define GLFW_X11_LIBRARY_WINDOW_STATE   _GLFWlibraryX11 x11;

#define GLFW_GLX_CONTEXT_STATE          _GLFWcontextGLX glx;


// GLX-specific per-context data
//
typedef struct _GLFWcontextGLX
{
    GLXContext      handle;
    GLXWindow       window;
    GLXFBConfig     fbconfig;
} _GLFWcontextGLX;

// X11-specific per-window data
//
typedef struct _GLFWwindowX11
{
    Colormap        colormap;
    Window          handle;
    Window          parent;
    XIC             ic;

    IntBool        overrideRedirect;
    IntBool        iconified;
    IntBool        maximized;

    // Whether the visual supports framebuffer transparency
    IntBool        transparent;

    // Cached position and size used to filter out duplicate events
    int             width, height;
    int             xpos, ypos;

    // The last received cursor position, regardless of source
    int             lastCursorPosX, lastCursorPosY;
    // The last position the cursor was warped to by GLFW
    int             warpCursorPosX, warpCursorPosY;

    // The time of the last KeyPress event per keycode, for discarding
    // duplicate key events generated for some keys by ibus
    Time            keyPressTimes[256];
} _GLFWwindowX11;

// X11-specific global data
//
typedef struct _GLFWlibraryX11
{
    Display*        display;
    int             screen;
    Window          root;

    // System content scale
    float           contentScaleX, contentScaleY;
    // Helper window for IPC
    Window          helperWindowHandle;
    // Invisible cursor for hidden cursor mode
    Cursor          hiddenCursorHandle;
    // Context for mapping window XIDs to plafWindow pointers
    XContext        context;
    // XIM input method
    XIM             im;
    // The previous X error handler, to be restored later
    XErrorHandler   errorHandler;
    // Most recent error code received by X error handler
    int             errorCode;
    // Key name string
    char            keynames[KEY_LAST + 1][5];
    // X11 keycode to GLFW key LUT
    short int       keycodes[256];
    // GLFW key to X11 keycode LUT
    short int       scancodes[KEY_LAST + 1];
    // Where to place the cursor when re-enabled
    double          restoreCursorPosX, restoreCursorPosY;
    int             emptyEventPipe[2];

    // Window manager atoms
    Atom            NET_SUPPORTED;
    Atom            NET_SUPPORTING_WM_CHECK;
    Atom            WM_PROTOCOLS;
    Atom            WM_STATE;
    Atom            WM_DELETE_WINDOW;
    Atom            NET_WM_NAME;
    Atom            NET_WM_ICON_NAME;
    Atom            NET_WM_ICON;
    Atom            NET_WM_PID;
    Atom            NET_WM_PING;
    Atom            NET_WM_WINDOW_TYPE;
    Atom            NET_WM_WINDOW_TYPE_NORMAL;
    Atom            NET_WM_STATE;
    Atom            NET_WM_STATE_ABOVE;
    Atom            NET_WM_STATE_FULLSCREEN;
    Atom            NET_WM_STATE_MAXIMIZED_VERT;
    Atom            NET_WM_STATE_MAXIMIZED_HORZ;
    Atom            NET_WM_STATE_DEMANDS_ATTENTION;
    Atom            NET_WM_BYPASS_COMPOSITOR;
    Atom            NET_WM_FULLSCREEN_MONITORS;
    Atom            NET_WM_WINDOW_OPACITY;
    Atom            NET_WM_CM_Sx;
    Atom            NET_WORKAREA;
    Atom            NET_CURRENT_DESKTOP;
    Atom            NET_ACTIVE_WINDOW;
    Atom            NET_FRAME_EXTENTS;
    Atom            NET_REQUEST_FRAME_EXTENTS;
    Atom            MOTIF_WM_HINTS;

    // Xdnd (drag and drop) atoms
    Atom            XdndAware;
    Atom            XdndEnter;
    Atom            XdndPosition;
    Atom            XdndStatus;
    Atom            XdndActionCopy;
    Atom            XdndDrop;
    Atom            XdndFinished;
    Atom            XdndSelection;
    Atom            XdndTypeList;
    Atom            text_uri_list;

    // Selection (clipboard) atoms
    Atom            TARGETS;
    Atom            MULTIPLE;
    Atom            INCR;
    Atom            CLIPBOARD;
    Atom            PRIMARY;
    Atom            CLIPBOARD_MANAGER;
    Atom            SAVE_TARGETS;
    Atom            NULL_;
    Atom            UTF8_STRING;
    Atom            COMPOUND_STRING;
    Atom            ATOM_PAIR;
    Atom            GLFW_SELECTION;

    struct {
        void*       handle;
        IntBool    utf8;
        PFN_XAllocSizeHints AllocSizeHints;
        PFN_XAllocWMHints AllocWMHints;
        PFN_XChangeProperty ChangeProperty;
        PFN_XChangeWindowAttributes ChangeWindowAttributes;
        PFN_XCheckIfEvent CheckIfEvent;
        PFN_XCheckTypedWindowEvent CheckTypedWindowEvent;
        PFN_XCloseDisplay CloseDisplay;
        PFN_XCloseIM CloseIM;
        PFN_XConvertSelection ConvertSelection;
        PFN_XCreateColormap CreateColormap;
        PFN_XCreateFontCursor CreateFontCursor;
        PFN_XCreateIC CreateIC;
        PFN_XCreateRegion CreateRegion;
        PFN_XCreateWindow CreateWindow;
        PFN_XDefineCursor DefineCursor;
        PFN_XDeleteContext DeleteContext;
        PFN_XDeleteProperty DeleteProperty;
        PFN_XDestroyIC DestroyIC;
        PFN_XDestroyRegion DestroyRegion;
        PFN_XDestroyWindow DestroyWindow;
        PFN_XDisplayKeycodes DisplayKeycodes;
        PFN_XEventsQueued EventsQueued;
        PFN_XFilterEvent FilterEvent;
        PFN_XFindContext FindContext;
        PFN_XFlush Flush;
        PFN_XFree Free;
        PFN_XFreeColormap FreeColormap;
        PFN_XFreeCursor FreeCursor;
        PFN_XFreeEventData FreeEventData;
        PFN_XGetErrorText GetErrorText;
        PFN_XGetICValues GetICValues;
        PFN_XGetIMValues GetIMValues;
        PFN_XGetInputFocus GetInputFocus;
        PFN_XGetKeyboardMapping GetKeyboardMapping;
        PFN_XGetScreenSaver GetScreenSaver;
        PFN_XGetSelectionOwner GetSelectionOwner;
        PFN_XGetWMNormalHints GetWMNormalHints;
        PFN_XGetWindowAttributes GetWindowAttributes;
        PFN_XGetWindowProperty GetWindowProperty;
        PFN_XIconifyWindow IconifyWindow;
        PFN_XInternAtom InternAtom;
        PFN_XLookupString LookupString;
        PFN_XMapRaised MapRaised;
        PFN_XMapWindow MapWindow;
        PFN_XMoveResizeWindow MoveResizeWindow;
        PFN_XMoveWindow MoveWindow;
        PFN_XNextEvent NextEvent;
        PFN_XOpenIM OpenIM;
        PFN_XPeekEvent PeekEvent;
        PFN_XPending Pending;
        PFN_XQueryExtension QueryExtension;
        PFN_XQueryPointer QueryPointer;
        PFN_XRaiseWindow RaiseWindow;
        PFN_XRegisterIMInstantiateCallback RegisterIMInstantiateCallback;
        PFN_XResizeWindow ResizeWindow;
        PFN_XResourceManagerString ResourceManagerString;
        PFN_XSaveContext SaveContext;
        PFN_XSelectInput SelectInput;
        PFN_XSendEvent SendEvent;
        PFN_XSetErrorHandler SetErrorHandler;
        PFN_XSetICFocus SetICFocus;
        PFN_XSetIMValues SetIMValues;
        PFN_XSetInputFocus SetInputFocus;
        PFN_XSetLocaleModifiers SetLocaleModifiers;
        PFN_XSetScreenSaver SetScreenSaver;
        PFN_XSetSelectionOwner SetSelectionOwner;
        PFN_XSetWMHints SetWMHints;
        PFN_XSetWMNormalHints SetWMNormalHints;
        PFN_XSetWMProtocols SetWMProtocols;
        PFN_XSupportsLocale SupportsLocale;
        PFN_XSync Sync;
        PFN_XTranslateCoordinates TranslateCoordinates;
        PFN_XUndefineCursor UndefineCursor;
        PFN_XUnmapWindow UnmapWindow;
        PFN_XUnsetICFocus UnsetICFocus;
        PFN_XWarpPointer WarpPointer;
        PFN_XUnregisterIMInstantiateCallback UnregisterIMInstantiateCallback;
        PFN_Xutf8LookupString utf8LookupString;
        PFN_Xutf8SetWMProperties utf8SetWMProperties;
    } xlib;

    struct {
        PFN_XrmDestroyDatabase DestroyDatabase;
        PFN_XrmGetResource GetResource;
        PFN_XrmGetStringDatabase GetStringDatabase;
    } xrm;

    struct {
        IntBool    available;
        void*       handle;
        int         eventBase;
        int         errorBase;
        int         major;
        int         minor;
        IntBool    gammaBroken;
        IntBool    monitorBroken;
        PFN_XRRAllocGamma AllocGamma;
        PFN_XRRFreeCrtcInfo FreeCrtcInfo;
        PFN_XRRFreeGamma FreeGamma;
        PFN_XRRFreeOutputInfo FreeOutputInfo;
        PFN_XRRFreeScreenResources FreeScreenResources;
        PFN_XRRGetCrtcGamma GetCrtcGamma;
        PFN_XRRGetCrtcGammaSize GetCrtcGammaSize;
        PFN_XRRGetCrtcInfo GetCrtcInfo;
        PFN_XRRGetOutputInfo GetOutputInfo;
        PFN_XRRGetOutputPrimary GetOutputPrimary;
        PFN_XRRGetScreenResourcesCurrent GetScreenResourcesCurrent;
        PFN_XRRQueryExtension QueryExtension;
        PFN_XRRQueryVersion QueryVersion;
        PFN_XRRSelectInput SelectInput;
        PFN_XRRSetCrtcConfig SetCrtcConfig;
        PFN_XRRSetCrtcGamma SetCrtcGamma;
        PFN_XRRUpdateConfiguration UpdateConfiguration;
    } randr;

    struct {
        IntBool     available;
        IntBool     detectable;
        int          majorOpcode;
        int          eventBase;
        int          errorBase;
        int          major;
        int          minor;
        unsigned int group;
        PFN_XkbFreeKeyboard FreeKeyboard;
        PFN_XkbFreeNames FreeNames;
        PFN_XkbGetMap GetMap;
        PFN_XkbGetNames GetNames;
        PFN_XkbGetState GetState;
        PFN_XkbQueryExtension QueryExtension;
        PFN_XkbSelectEventDetails SelectEventDetails;
        PFN_XkbSetDetectableAutoRepeat SetDetectableAutoRepeat;
    } xkb;

    struct {
        int         count;
        int         timeout;
        int         interval;
        int         blanking;
        int         exposure;
    } saver;

    struct {
        int         version;
        Window      source;
        Atom        format;
    } xdnd;

    struct {
        void*       handle;
        PFN_XcursorImageCreate ImageCreate;
        PFN_XcursorImageDestroy ImageDestroy;
        PFN_XcursorImageLoadCursor ImageLoadCursor;
        PFN_XcursorGetTheme GetTheme;
        PFN_XcursorGetDefaultSize GetDefaultSize;
        PFN_XcursorLibraryLoadImage LibraryLoadImage;
    } xcursor;

    struct {
        IntBool    available;
        void*       handle;
        int         major;
        int         minor;
        PFN_XineramaIsActive IsActive;
        PFN_XineramaQueryExtension QueryExtension;
        PFN_XineramaQueryScreens QueryScreens;
    } xinerama;

    struct {
        IntBool    available;
        void*       handle;
        int         eventBase;
        int         errorBase;
        PFN_XF86VidModeQueryExtension QueryExtension;
        PFN_XF86VidModeGetGammaRamp GetGammaRamp;
        PFN_XF86VidModeSetGammaRamp SetGammaRamp;
        PFN_XF86VidModeGetGammaRampSize GetGammaRampSize;
    } vidmode;

    struct {
        IntBool    available;
        void*       handle;
        int         majorOpcode;
        int         eventBase;
        int         errorBase;
        int         major;
        int         minor;
        PFN_XIQueryVersion QueryVersion;
    } xi;

    struct {
        IntBool    available;
        void*       handle;
        int         major;
        int         minor;
        int         eventBase;
        int         errorBase;
        PFN_XRenderQueryExtension QueryExtension;
        PFN_XRenderQueryVersion QueryVersion;
        PFN_XRenderFindVisualFormat FindVisualFormat;
    } xrender;

    struct {
        IntBool    available;
        void*       handle;
        int         major;
        int         minor;
        int         eventBase;
        int         errorBase;
        PFN_XShapeQueryExtension QueryExtension;
        PFN_XShapeCombineRegion ShapeCombineRegion;
        PFN_XShapeQueryVersion QueryVersion;
        PFN_XShapeCombineMask ShapeCombineMask;
    } xshape;
} _GLFWlibraryX11;

// X11-specific per-monitor data
//
typedef struct _GLFWmonitorX11
{
    RROutput        output;
    RRCrtc          crtc;
    RRMode          oldMode;

    // Index of corresponding Xinerama screen,
    // for EWMH full screen window placement
    int             index;
} _GLFWmonitorX11;

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

void _glfwSetCursorModeX11(plafWindow* window, int mode);
int _glfwGetKeyScancodeX11(int key);
IntBool _glfwCreateCursorX11(plafCursor* cursor, const ImageData* image, int xhot, int yhot);
IntBool _glfwCreateStandardCursorX11(plafCursor* cursor, int shape);
void _glfwDestroyCursorX11(plafCursor* cursor);

void _glfwFreeMonitorX11(plafMonitor* monitor);
void _glfwGetMonitorPosX11(plafMonitor* monitor, int* xpos, int* ypos);
void _glfwGetMonitorContentScaleX11(plafMonitor* monitor, float* xscale, float* yscale);
void _glfwGetMonitorWorkareaX11(plafMonitor* monitor, int* xpos, int* ypos, int* width, int* height);
VideoMode* _glfwGetVideoModesX11(plafMonitor* monitor, int* count);
IntBool _glfwGetVideoModeX11(plafMonitor* monitor, VideoMode* mode);
IntBool _glfwGetGammaRampX11(plafMonitor* monitor, GammaRamp* ramp);
void _glfwSetGammaRampX11(plafMonitor* monitor, const GammaRamp* ramp);

void _glfwPollMonitorsX11(void);
void _glfwSetVideoModeX11(plafMonitor* monitor, const VideoMode* desired);
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

#endif // __linux__