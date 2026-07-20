// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"math"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xruntime"
	"golang.org/x/sys/windows"
)

var (
	user32                            = windows.NewLazySystemDLL("user32.dll")
	adjustWindowRectExProc            = user32.NewProc("AdjustWindowRectEx")
	adjustWindowRectExForDpiProc      = user32.NewProc("AdjustWindowRectExForDpi")
	attachThreadInputProc             = user32.NewProc("AttachThreadInput")
	bringWindowToTopProc              = user32.NewProc("BringWindowToTop")
	callNextHookExProc                = user32.NewProc("CallNextHookEx")
	changeWindowMessageFilterExProc   = user32.NewProc("ChangeWindowMessageFilterEx")
	clientToScreenProc                = user32.NewProc("ClientToScreen")
	closeClipboardProc                = user32.NewProc("CloseClipboard")
	createCursorProc                  = user32.NewProc("CreateCursor")
	createIconIndirectProc            = user32.NewProc("CreateIconIndirect")
	createWindowExWProc               = user32.NewProc("CreateWindowExW")
	defWindowProcWProc                = user32.NewProc("DefWindowProcW")
	destroyIconProc                   = user32.NewProc("DestroyIcon")
	destroyWindowProc                 = user32.NewProc("DestroyWindow")
	dispatchMessageWProc              = user32.NewProc("DispatchMessageW")
	emptyClipboardProc                = user32.NewProc("EmptyClipboard")
	enumClipboardFormatsProc          = user32.NewProc("EnumClipboardFormats")
	enumDisplayDevicesWProc           = user32.NewProc("EnumDisplayDevicesW")
	enumDisplayMonitorsProc           = user32.NewProc("EnumDisplayMonitors")
	getActiveWindowProc               = user32.NewProc("GetActiveWindow")
	getClassLongPtrWProc              = user32.NewProc("GetClassLongPtrW")
	getClientRectProc                 = user32.NewProc("GetClientRect")
	getClipboardDataProc              = user32.NewProc("GetClipboardData")
	getClipboardFormatNameWProc       = user32.NewProc("GetClipboardFormatNameW")
	getClipboardSequenceNumberProc    = user32.NewProc("GetClipboardSequenceNumber")
	isClipboardFormatAvailableProc    = user32.NewProc("IsClipboardFormatAvailable")
	getCursorPosProc                  = user32.NewProc("GetCursorPos")
	getDCProc                         = user32.NewProc("GetDC")
	getDoubleClickTimeProc            = user32.NewProc("GetDoubleClickTime")
	getDpiForWindowProc               = user32.NewProc("GetDpiForWindow")
	getForegroundWindowProc           = user32.NewProc("GetForegroundWindow")
	getKeyStateProc                   = user32.NewProc("GetKeyState")
	getMessageTimeProc                = user32.NewProc("GetMessageTime")
	getMonitorInfoWProc               = user32.NewProc("GetMonitorInfoW")
	getSysColorProc                   = user32.NewProc("GetSysColor")
	getSystemMetricsProc              = user32.NewProc("GetSystemMetrics")
	getWindowPlacementProc            = user32.NewProc("GetWindowPlacement")
	getWindowRectProc                 = user32.NewProc("GetWindowRect")
	getWindowThreadProcessIdProc      = user32.NewProc("GetWindowThreadProcessId")
	loadImageWProc                    = user32.NewProc("LoadImageW")
	mapVirtualKeyWProc                = user32.NewProc("MapVirtualKeyW")
	messageBeepProc                   = user32.NewProc("MessageBeep")
	monitorFromWindowProc             = user32.NewProc("MonitorFromWindow")
	openClipboardProc                 = user32.NewProc("OpenClipboard")
	peekMessageWProc                  = user32.NewProc("PeekMessageW")
	postMessageWProc                  = user32.NewProc("PostMessageW")
	postThreadMessageWProc            = user32.NewProc("PostThreadMessageW")
	registerClassExWProc              = user32.NewProc("RegisterClassExW")
	registerClipboardFormatWProc      = user32.NewProc("RegisterClipboardFormatW")
	releaseDCProc                     = user32.NewProc("ReleaseDC")
	screenToClientProc                = user32.NewProc("ScreenToClient")
	sendMessageWProc                  = user32.NewProc("SendMessageW")
	setClipboardDataProc              = user32.NewProc("SetClipboardData")
	setCursorProc                     = user32.NewProc("SetCursor")
	setFocusProc                      = user32.NewProc("SetFocus")
	setForegroundWindowProc           = user32.NewProc("SetForegroundWindow")
	setProcessDpiAwarenessContextProc = user32.NewProc("SetProcessDpiAwarenessContext")
	setWindowPlacementProc            = user32.NewProc("SetWindowPlacement")
	setWindowPosProc                  = user32.NewProc("SetWindowPos")
	setWindowsHookExWProc             = user32.NewProc("SetWindowsHookExW")
	setWindowTextWProc                = user32.NewProc("SetWindowTextW")
	showWindowProc                    = user32.NewProc("ShowWindow")
	trackMouseEventProc               = user32.NewProc("TrackMouseEvent")
	translateMessageProc              = user32.NewProc("TranslateMessage")
	unhookWindowsHookExProc           = user32.NewProc("UnhookWindowsHookEx")
	waitMessageProc                   = user32.NewProc("WaitMessage")
	windowFromPointProc               = user32.NewProc("WindowFromPoint")
)

// Clipboard format types https://docs.microsoft.com/en-us/windows/desktop/dataxchg/standard-clipboard-formats
const (
	CFNone         ClipboardFormat = 0
	CFText         ClipboardFormat = 1
	CFOEMText      ClipboardFormat = 7
	CFUnicodeText  ClipboardFormat = 13
	CFHDrop        ClipboardFormat = 15
	CFPrivateFirst ClipboardFormat = 0x0200
)

type DPI_AWARENESS_CONTEXT windows.Handle

const DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 DPI_AWARENESS_CONTEXT = math.MaxUint - 3

const IDI_APPLICATION = 32512

// ColorHighlight https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getsyscolor
const ColorHighlight = 13

// BeepType https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-messagebeep
type BeepType uint

// Possible beep types https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-messagebeep
const (
	MB_OK              BeepType = 0
	MB_ICONERROR       BeepType = 0x10
	MB_ICONQUESTION    BeepType = 0x20
	MB_ICONWARNING     BeepType = 0x30
	MB_ICONINFORMATION BeepType = 0x40
	MB_SIMPLE          BeepType = 0xFFFFFFFF
)

// Constants for some standard cursors
const (
	OCR_NORMAL = 32512
	OCR_HAND   = 32649
	OCR_IBEAM  = 32513
)

// Constants for image types.
const (
	IMAGE_ICON   = 1
	IMAGE_CURSOR = 2
)

// Constants for LoadImageW function.
const (
	LR_DEFAULT_SIZE = 0x40
	LR_SHARED       = 0x8000
)

// https://learn.microsoft.com/openspecs/windows_protocols/ms-wmf/4e588f70-bd92-4a6f-b77f-35d0feaf7a57
const (
	BI_RGB       = 0
	BI_RLE8      = 1
	BI_RLE4      = 2
	BI_BITFIELDS = 3
	BI_JPEG      = 4
	BI_PNG       = 5
	BI_CMYK      = 11
	BI_CMYKRLE8  = 12
	BI_CMYKRLE4  = 13
	BI_1632      = 842217009
)

// https://learn.microsoft.com/windows/win32/winmsg/window-class-styles
const (
	CS_VREDRAW         = 1
	CS_HREDRAW         = 2
	CS_DBLCLKS         = 8
	CS_OWNDC           = 32
	CS_CLASSDC         = 64
	CS_PARENTDC        = 128
	CS_NOCLOSE         = 512
	CS_SAVEBITS        = 2048
	CS_BYTEALIGNCLIENT = 4096
	CS_BYTEALIGNWINDOW = 8192
	CS_GLOBALCLASS     = 16384
	CS_IME             = 65536
	CS_DROPSHADOW      = 131072
)

// https://learn.microsoft.com/windows/win32/winmsg/window-styles
const (
	WS_ACTIVECAPTION    = 1
	WS_BORDER           = 8388608
	WS_CAPTION          = 12582912
	WS_CHILD            = 1073741824
	WS_CHILDWINDOW      = 1073741824
	WS_CLIPCHILDREN     = 33554432
	WS_CLIPSIBLINGS     = 67108864
	WS_DISABLED         = 134217728
	WS_DLGFRAME         = 4194304
	WS_GROUP            = 131072
	WS_HSCROLL          = 1048576
	WS_ICONIC           = 536870912
	WS_MAXIMIZE         = 16777216
	WS_MAXIMIZEBOX      = 65536
	WS_MINIMIZE         = 536870912
	WS_MINIMIZEBOX      = 131072
	WS_OVERLAPPED       = 0
	WS_OVERLAPPEDWINDOW = 13565952
	WS_POPUP            = 2147483648
	WS_POPUPWINDOW      = 2156396544
	WS_SIZEBOX          = 262144
	WS_SYSMENU          = 524288
	WS_TABSTOP          = 65536
	WS_THICKFRAME       = 262144
	WS_TILED            = 0
	WS_TILEDWINDOW      = 13565952
	WS_VISIBLE          = 268435456
	WS_VSCROLL          = 2097152
)

// https://learn.microsoft.com/windows/win32/winmsg/extended-window-styles
const (
	WS_EX_ACCEPTFILES         = 16
	WS_EX_APPWINDOW           = 262144
	WS_EX_CLIENTEDGE          = 512
	WS_EX_COMPOSITED          = 33554432
	WS_EX_CONTEXTHELP         = 1024
	WS_EX_CONTROLPARENT       = 65536
	WS_EX_DLGMODALFRAME       = 1
	WS_EX_LAYERED             = 524288
	WS_EX_LAYOUTRTL           = 4194304
	WS_EX_LEFT                = 0
	WS_EX_LEFTSCROLLBAR       = 16384
	WS_EX_LTRREADING          = 0
	WS_EX_MDICHILD            = 64
	WS_EX_NOACTIVATE          = 134217728
	WS_EX_NOINHERITLAYOUT     = 1048576
	WS_EX_NOPARENTNOTIFY      = 4
	WS_EX_NOREDIRECTIONBITMAP = 2097152
	WS_EX_OVERLAPPEDWINDOW    = 768
	WS_EX_PALETTEWINDOW       = 392
	WS_EX_RIGHT               = 4096
	WS_EX_RIGHTSCROLLBAR      = 0
	WS_EX_RTLREADING          = 8192
	WS_EX_STATICEDGE          = 131072
	WS_EX_TOOLWINDOW          = 128
	WS_EX_TOPMOST             = 8
	WS_EX_TRANSPARENT         = 32
	WS_EX_WINDOWEDGE          = 256
)

// QS_... constants for PeekMessage and GetQueueStatus
// https://learn.microsoft.com/windows/win32/winmsg/queue-status-flags
const (
	QS_KEY = 1 << iota
	QS_MOUSEMOVE
	QS_MOUSEBUTTON
	QS_POSTMESSAGE
	QS_TIMER
	QS_PAINT
	QS_SENDMESSAGE
	QS_HOTKEY
	QS_ALLPOSTMESSAGE
	_QS_UNUSED
	QS_RAWINPUT
	QS_TOUCH
	QS_POINTER
	QS_MOUSE     = QS_MOUSEMOVE | QS_MOUSEBUTTON
	QS_INPUT     = QS_MOUSE | QS_KEY | QS_RAWINPUT | QS_TOUCH | QS_POINTER
	QS_ALLEVENTS = QS_INPUT | QS_POSTMESSAGE | QS_TIMER | QS_PAINT | QS_HOTKEY
	QS_ALLINPUT  = QS_ALLEVENTS | QS_SENDMESSAGE
)

// PeekMessage flags https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-peekmessagew
const (
	PM_NOREMOVE       = 0x0000
	PM_REMOVE         = 0x0001
	PM_NOYIELD        = 0x0002
	PM_QS_INPUT       = QS_INPUT << 16
	PM_QS_POSTMESSAGE = (QS_POSTMESSAGE | QS_HOTKEY | QS_TIMER) << 16
	PM_QS_PAINT       = QS_PAINT << 16
	PM_QS_SENDMESSAGE = QS_SENDMESSAGE << 16
)

// Window messages are sent to the window class' window procedure. They identify the type of event that happened.
//
// https://learn.microsoft.com/windows/win32/learnwin32/window-messages
const (
	WM_CTLCOLOR                                  = 25
	WM_MOUSEHOVER                                = 673
	WM_MOUSELEAVE                                = 675
	WM_CAP_START                                 = 1024
	WM_CAP_UNICODE_START                         = 1124
	WM_CAP_GET_CAPSTREAMPTR                      = 1025
	WM_CAP_SET_CALLBACK_ERRORW                   = 1126
	WM_CAP_SET_CALLBACK_STATUSW                  = 1127
	WM_CAP_SET_CALLBACK_ERRORA                   = 1026
	WM_CAP_SET_CALLBACK_STATUSA                  = 1027
	WM_CAP_SET_CALLBACK_ERROR                    = 1126
	WM_CAP_SET_CALLBACK_STATUS                   = 1127
	WM_CAP_SET_CALLBACK_YIELD                    = 1028
	WM_CAP_SET_CALLBACK_FRAME                    = 1029
	WM_CAP_SET_CALLBACK_VIDEOSTREAM              = 1030
	WM_CAP_SET_CALLBACK_WAVESTREAM               = 1031
	WM_CAP_GET_USER_DATA                         = 1032
	WM_CAP_SET_USER_DATA                         = 1033
	WM_CAP_DRIVER_CONNECT                        = 1034
	WM_CAP_DRIVER_DISCONNECT                     = 1035
	WM_CAP_DRIVER_GET_NAMEA                      = 1036
	WM_CAP_DRIVER_GET_VERSIONA                   = 1037
	WM_CAP_DRIVER_GET_NAMEW                      = 1136
	WM_CAP_DRIVER_GET_VERSIONW                   = 1137
	WM_CAP_DRIVER_GET_NAME                       = 1136
	WM_CAP_DRIVER_GET_VERSION                    = 1137
	WM_CAP_DRIVER_GET_CAPS                       = 1038
	WM_CAP_FILE_SET_CAPTURE_FILEA                = 1044
	WM_CAP_FILE_GET_CAPTURE_FILEA                = 1045
	WM_CAP_FILE_SAVEASA                          = 1047
	WM_CAP_FILE_SAVEDIBA                         = 1049
	WM_CAP_FILE_SET_CAPTURE_FILEW                = 1144
	WM_CAP_FILE_GET_CAPTURE_FILEW                = 1145
	WM_CAP_FILE_SAVEASW                          = 1147
	WM_CAP_FILE_SAVEDIBW                         = 1149
	WM_CAP_FILE_SET_CAPTURE_FILE                 = 1144
	WM_CAP_FILE_GET_CAPTURE_FILE                 = 1145
	WM_CAP_FILE_SAVEAS                           = 1147
	WM_CAP_FILE_SAVEDIB                          = 1149
	WM_CAP_FILE_ALLOCATE                         = 1046
	WM_CAP_FILE_SET_INFOCHUNK                    = 1048
	WM_CAP_EDIT_COPY                             = 1054
	WM_CAP_SET_AUDIOFORMAT                       = 1059
	WM_CAP_GET_AUDIOFORMAT                       = 1060
	WM_CAP_DLG_VIDEOFORMAT                       = 1065
	WM_CAP_DLG_VIDEOSOURCE                       = 1066
	WM_CAP_DLG_VIDEODISPLAY                      = 1067
	WM_CAP_GET_VIDEOFORMAT                       = 1068
	WM_CAP_SET_VIDEOFORMAT                       = 1069
	WM_CAP_DLG_VIDEOCOMPRESSION                  = 1070
	WM_CAP_SET_PREVIEW                           = 1074
	WM_CAP_SET_OVERLAY                           = 1075
	WM_CAP_SET_PREVIEWRATE                       = 1076
	WM_CAP_SET_SCALE                             = 1077
	WM_CAP_GET_STATUS                            = 1078
	WM_CAP_SET_SCROLL                            = 1079
	WM_CAP_GRAB_FRAME                            = 1084
	WM_CAP_GRAB_FRAME_NOSTOP                     = 1085
	WM_CAP_SEQUENCE                              = 1086
	WM_CAP_SEQUENCE_NOFILE                       = 1087
	WM_CAP_SET_SEQUENCE_SETUP                    = 1088
	WM_CAP_GET_SEQUENCE_SETUP                    = 1089
	WM_CAP_SET_MCI_DEVICEA                       = 1090
	WM_CAP_GET_MCI_DEVICEA                       = 1091
	WM_CAP_SET_MCI_DEVICEW                       = 1190
	WM_CAP_GET_MCI_DEVICEW                       = 1191
	WM_CAP_SET_MCI_DEVICE                        = 1190
	WM_CAP_GET_MCI_DEVICE                        = 1191
	WM_CAP_STOP                                  = 1092
	WM_CAP_ABORT                                 = 1093
	WM_CAP_SINGLE_FRAME_OPEN                     = 1094
	WM_CAP_SINGLE_FRAME_CLOSE                    = 1095
	WM_CAP_SINGLE_FRAME                          = 1096
	WM_CAP_PAL_OPENA                             = 1104
	WM_CAP_PAL_SAVEA                             = 1105
	WM_CAP_PAL_OPENW                             = 1204
	WM_CAP_PAL_SAVEW                             = 1205
	WM_CAP_PAL_OPEN                              = 1204
	WM_CAP_PAL_SAVE                              = 1205
	WM_CAP_PAL_PASTE                             = 1106
	WM_CAP_PAL_AUTOCREATE                        = 1107
	WM_CAP_PAL_MANUALCREATE                      = 1108
	WM_CAP_SET_CALLBACK_CAPCONTROL               = 1109
	WM_CAP_UNICODE_END                           = 1205
	WM_CAP_END                                   = 1205
	WM_CPL_LAUNCH                                = 2024
	WM_CPL_LAUNCHED                              = 2025
	WM_TABLET_DEFBASE                            = 704
	WM_TABLET_MAXOFFSET                          = 32
	WM_TABLET_ADDED                              = 712
	WM_TABLET_DELETED                            = 713
	WM_TABLET_FLICK                              = 715
	WM_TABLET_QUERYSYSTEMGESTURESTATUS           = 716
	WM_FI_FILENAME                               = 900
	WM_CT_REPEAT_FIRST_FIELD                     = 16
	WM_CT_BOTTOM_FIELD_FIRST                     = 32
	WM_CT_TOP_FIELD_FIRST                        = 64
	WM_CT_INTERLACED                             = 128
	WM_CL_INTERLACED420                          = 0
	WM_CL_PROGRESSIVE420                         = 1
	WM_MAX_VIDEO_STREAMS                         = 63
	WM_MAX_STREAMS                               = 63
	WM_ADSPROP_NOTIFY_PAGEINIT                   = 2125
	WM_ADSPROP_NOTIFY_PAGEHWND                   = 2126
	WM_ADSPROP_NOTIFY_CHANGE                     = 2127
	WM_ADSPROP_NOTIFY_APPLY                      = 2128
	WM_ADSPROP_NOTIFY_SETFOCUS                   = 2129
	WM_ADSPROP_NOTIFY_FOREGROUND                 = 2130
	WM_ADSPROP_NOTIFY_EXIT                       = 2131
	WM_ADSPROP_NOTIFY_ERROR                      = 2134
	WM_RASDIALEVENT                              = 52429
	WM_DDE_FIRST                                 = 992
	WM_DDE_INITIATE                              = 992
	WM_DDE_TERMINATE                             = 993
	WM_DDE_ADVISE                                = 994
	WM_DDE_UNADVISE                              = 995
	WM_DDE_ACK                                   = 996
	WM_DDE_DATA                                  = 997
	WM_DDE_REQUEST                               = 998
	WM_DDE_POKE                                  = 999
	WM_DDE_EXECUTE                               = 1000
	WM_DDE_LAST                                  = 1000
	WM_IME_REPORT                                = 640
	WM_WNT_CONVERTREQUESTEX                      = 265
	WM_CONVERTREQUEST                            = 266
	WM_CONVERTRESULT                             = 267
	WM_INTERIM                                   = 268
	WM_IMEKEYDOWN                                = 656
	WM_IMEKEYUP                                  = 657
	WM_CHOOSEFONT_GETLOGFONT                     = 1025
	WM_CHOOSEFONT_SETLOGFONT                     = 1125
	WM_CHOOSEFONT_SETFLAGS                       = 1126
	WM_PSD_FULLPAGERECT                          = 1025
	WM_PSD_MINMARGINRECT                         = 1026
	WM_PSD_MARGINRECT                            = 1027
	WM_PSD_GREEKTEXTRECT                         = 1028
	WM_PSD_ENVSTAMPRECT                          = 1029
	WM_PSD_YAFULLPAGERECT                        = 1030
	WM_CONTEXTMENU                               = 123
	WM_UNICHAR                                   = 265
	WM_PRINTCLIENT                               = 792
	WM_NOTIFY                                    = 78
	WM_DEVICECHANGE                              = 537
	WM_NULL                                      = 0
	WM_CREATE                                    = 1
	WM_DESTROY                                   = 2
	WM_MOVE                                      = 3
	WM_SIZE                                      = 5
	WM_ACTIVATE                                  = 6
	WM_SETFOCUS                                  = 7
	WM_KILLFOCUS                                 = 8
	WM_ENABLE                                    = 10
	WM_SETREDRAW                                 = 11
	WM_SETTEXT                                   = 12
	WM_GETTEXT                                   = 13
	WM_GETTEXTLENGTH                             = 14
	WM_PAINT                                     = 15
	WM_CLOSE                                     = 16
	WM_QUERYENDSESSION                           = 17
	WM_QUERYOPEN                                 = 19
	WM_ENDSESSION                                = 22
	WM_QUIT                                      = 18
	WM_ERASEBKGND                                = 20
	WM_SYSCOLORCHANGE                            = 21
	WM_SHOWWINDOW                                = 24
	WM_WININICHANGE                              = 26
	WM_SETTINGCHANGE                             = 26
	WM_DEVMODECHANGE                             = 27
	WM_ACTIVATEAPP                               = 28
	WM_FONTCHANGE                                = 29
	WM_TIMECHANGE                                = 30
	WM_CANCELMODE                                = 31
	WM_SETCURSOR                                 = 32
	WM_MOUSEACTIVATE                             = 33
	WM_CHILDACTIVATE                             = 34
	WM_QUEUESYNC                                 = 35
	WM_GETMINMAXINFO                             = 36
	WM_PAINTICON                                 = 38
	WM_ICONERASEBKGND                            = 39
	WM_NEXTDLGCTL                                = 40
	WM_SPOOLERSTATUS                             = 42
	WM_DRAWITEM                                  = 43
	WM_MEASUREITEM                               = 44
	WM_DELETEITEM                                = 45
	WM_VKEYTOITEM                                = 46
	WM_CHARTOITEM                                = 47
	WM_SETFONT                                   = 48
	WM_GETFONT                                   = 49
	WM_SETHOTKEY                                 = 50
	WM_GETHOTKEY                                 = 51
	WM_QUERYDRAGICON                             = 55
	WM_COMPAREITEM                               = 57
	WM_GETOBJECT                                 = 61
	WM_COMPACTING                                = 65
	WM_COMMNOTIFY                                = 68
	WM_WINDOWPOSCHANGING                         = 70
	WM_WINDOWPOSCHANGED                          = 71
	WM_POWER                                     = 72
	WM_COPYGLOBALDATA                            = 73
	WM_COPYDATA                                  = 74
	WM_CANCELJOURNAL                             = 75
	WM_INPUTLANGCHANGEREQUEST                    = 80
	WM_INPUTLANGCHANGE                           = 81
	WM_TCARD                                     = 82
	WM_HELP                                      = 83
	WM_USERCHANGED                               = 84
	WM_NOTIFYFORMAT                              = 85
	WM_STYLECHANGING                             = 124
	WM_STYLECHANGED                              = 125
	WM_DISPLAYCHANGE                             = 126
	WM_GETICON                                   = 127
	WM_SETICON                                   = 128
	WM_NCCREATE                                  = 129
	WM_NCDESTROY                                 = 130
	WM_NCCALCSIZE                                = 131
	WM_NCHITTEST                                 = 132
	WM_NCPAINT                                   = 133
	WM_NCACTIVATE                                = 134
	WM_GETDLGCODE                                = 135
	WM_SYNCPAINT                                 = 136
	WM_NCMOUSEMOVE                               = 160
	WM_NCLBUTTONDOWN                             = 161
	WM_NCLBUTTONUP                               = 162
	WM_NCLBUTTONDBLCLK                           = 163
	WM_NCRBUTTONDOWN                             = 164
	WM_NCRBUTTONUP                               = 165
	WM_NCRBUTTONDBLCLK                           = 166
	WM_NCMBUTTONDOWN                             = 167
	WM_NCMBUTTONUP                               = 168
	WM_NCMBUTTONDBLCLK                           = 169
	WM_NCXBUTTONDOWN                             = 171
	WM_NCXBUTTONUP                               = 172
	WM_NCXBUTTONDBLCLK                           = 173
	WM_INPUT_DEVICE_CHANGE                       = 254
	WM_INPUT                                     = 255
	WM_KEYFIRST                                  = 256
	WM_KEYDOWN                                   = 256
	WM_KEYUP                                     = 257
	WM_CHAR                                      = 258
	WM_DEADCHAR                                  = 259
	WM_SYSKEYDOWN                                = 260
	WM_SYSKEYUP                                  = 261
	WM_SYSCHAR                                   = 262
	WM_SYSDEADCHAR                               = 263
	WM_KEYLAST                                   = 265
	WM_IME_STARTCOMPOSITION                      = 269
	WM_IME_ENDCOMPOSITION                        = 270
	WM_IME_COMPOSITION                           = 271
	WM_IME_KEYLAST                               = 271
	WM_INITDIALOG                                = 272
	WM_COMMAND                                   = 273
	WM_SYSCOMMAND                                = 274
	WM_TIMER                                     = 275
	WM_HSCROLL                                   = 276
	WM_VSCROLL                                   = 277
	WM_INITMENU                                  = 278
	WM_INITMENUPOPUP                             = 279
	WM_GESTURE                                   = 281
	WM_GESTURENOTIFY                             = 282
	WM_MENUSELECT                                = 287
	WM_MENUCHAR                                  = 288
	WM_ENTERIDLE                                 = 289
	WM_MENURBUTTONUP                             = 290
	WM_MENUDRAG                                  = 291
	WM_MENUGETOBJECT                             = 292
	WM_UNINITMENUPOPUP                           = 293
	WM_MENUCOMMAND                               = 294
	WM_CHANGEUISTATE                             = 295
	WM_UPDATEUISTATE                             = 296
	WM_QUERYUISTATE                              = 297
	WM_CTLCOLORMSGBOX                            = 306
	WM_CTLCOLOREDIT                              = 307
	WM_CTLCOLORLISTBOX                           = 308
	WM_CTLCOLORBTN                               = 309
	WM_CTLCOLORDLG                               = 310
	WM_CTLCOLORSCROLLBAR                         = 311
	WM_CTLCOLORSTATIC                            = 312
	WM_MOUSEFIRST                                = 512
	WM_MOUSEMOVE                                 = 512
	WM_LBUTTONDOWN                               = 513
	WM_LBUTTONUP                                 = 514
	WM_LBUTTONDBLCLK                             = 515
	WM_RBUTTONDOWN                               = 516
	WM_RBUTTONUP                                 = 517
	WM_RBUTTONDBLCLK                             = 518
	WM_MBUTTONDOWN                               = 519
	WM_MBUTTONUP                                 = 520
	WM_MBUTTONDBLCLK                             = 521
	WM_MOUSEWHEEL                                = 522
	WM_XBUTTONDOWN                               = 523
	WM_XBUTTONUP                                 = 524
	WM_XBUTTONDBLCLK                             = 525
	WM_MOUSEHWHEEL                               = 526
	WM_MOUSELAST                                 = 526
	WM_PARENTNOTIFY                              = 528
	WM_ENTERMENULOOP                             = 529
	WM_EXITMENULOOP                              = 530
	WM_NEXTMENU                                  = 531
	WM_SIZING                                    = 532
	WM_CAPTURECHANGED                            = 533
	WM_MOVING                                    = 534
	WM_POWERBROADCAST                            = 536
	WM_MDICREATE                                 = 544
	WM_MDIDESTROY                                = 545
	WM_MDIACTIVATE                               = 546
	WM_MDIRESTORE                                = 547
	WM_MDINEXT                                   = 548
	WM_MDIMAXIMIZE                               = 549
	WM_MDITILE                                   = 550
	WM_MDICASCADE                                = 551
	WM_MDIICONARRANGE                            = 552
	WM_MDIGETACTIVE                              = 553
	WM_MDISETMENU                                = 560
	WM_ENTERSIZEMOVE                             = 561
	WM_EXITSIZEMOVE                              = 562
	WM_DROPFILES                                 = 563
	WM_MDIREFRESHMENU                            = 564
	WM_POINTERDEVICECHANGE                       = 568
	WM_POINTERDEVICEINRANGE                      = 569
	WM_POINTERDEVICEOUTOFRANGE                   = 570
	WM_TOUCH                                     = 576
	WM_NCPOINTERUPDATE                           = 577
	WM_NCPOINTERDOWN                             = 578
	WM_NCPOINTERUP                               = 579
	WM_POINTERUPDATE                             = 581
	WM_POINTERDOWN                               = 582
	WM_POINTERUP                                 = 583
	WM_POINTERENTER                              = 585
	WM_POINTERLEAVE                              = 586
	WM_POINTERACTIVATE                           = 587
	WM_POINTERCAPTURECHANGED                     = 588
	WM_TOUCHHITTESTING                           = 589
	WM_POINTERWHEEL                              = 590
	WM_POINTERHWHEEL                             = 591
	WM_POINTERROUTEDTO                           = 593
	WM_POINTERROUTEDAWAY                         = 594
	WM_POINTERROUTEDRELEASED                     = 595
	WM_IME_SETCONTEXT                            = 641
	WM_IME_NOTIFY                                = 642
	WM_IME_CONTROL                               = 643
	WM_IME_COMPOSITIONFULL                       = 644
	WM_IME_SELECT                                = 645
	WM_IME_CHAR                                  = 646
	WM_IME_REQUEST                               = 648
	WM_IME_KEYDOWN                               = 656
	WM_IME_KEYUP                                 = 657
	WM_NCMOUSEHOVER                              = 672
	WM_NCMOUSELEAVE                              = 674
	WM_WTSSESSION_CHANGE                         = 689
	WM_TABLET_FIRST                              = 704
	WM_TABLET_LAST                               = 735
	WM_DPICHANGED                                = 736
	WM_DPICHANGED_BEFOREPARENT                   = 738
	WM_DPICHANGED_AFTERPARENT                    = 739
	WM_GETDPISCALEDSIZE                          = 740
	WM_CUT                                       = 768
	WM_COPY                                      = 769
	WM_PASTE                                     = 770
	WM_CLEAR                                     = 771
	WM_UNDO                                      = 772
	WM_RENDERFORMAT                              = 773
	WM_RENDERALLFORMATS                          = 774
	WM_DESTROYCLIPBOARD                          = 775
	WM_DRAWCLIPBOARD                             = 776
	WM_PAINTCLIPBOARD                            = 777
	WM_VSCROLLCLIPBOARD                          = 778
	WM_SIZECLIPBOARD                             = 779
	WM_ASKCBFORMATNAME                           = 780
	WM_CHANGECBCHAIN                             = 781
	WM_HSCROLLCLIPBOARD                          = 782
	WM_QUERYNEWPALETTE                           = 783
	WM_PALETTEISCHANGING                         = 784
	WM_PALETTECHANGED                            = 785
	WM_HOTKEY                                    = 786
	WM_PRINT                                     = 791
	WM_APPCOMMAND                                = 793
	WM_THEMECHANGED                              = 794
	WM_CLIPBOARDUPDATE                           = 797
	WM_DWMCOMPOSITIONCHANGED                     = 798
	WM_DWMNCRENDERINGCHANGED                     = 799
	WM_DWMCOLORIZATIONCOLORCHANGED               = 800
	WM_DWMWINDOWMAXIMIZEDCHANGE                  = 801
	WM_DWMSENDICONICTHUMBNAIL                    = 803
	WM_DWMSENDICONICLIVEPREVIEWBITMAP            = 806
	WM_GETTITLEBARINFOEX                         = 831
	WM_HANDHELDFIRST                             = 856
	WM_HANDHELDLAST                              = 863
	WM_AFXFIRST                                  = 864
	WM_AFXLAST                                   = 895
	WM_PENWINFIRST                               = 896
	WM_PENWINLAST                                = 911
	WM_APP                                       = 32768
	WM_USER                                      = 1024
	WM_TOOLTIPDISMISS                            = 837
	WM_SF_CLEANPOINT                             = 1
	WM_SF_DISCONTINUITY                          = 2
	WM_SF_DATALOSS                               = 4
	WM_SFEX_NOTASYNCPOINT                        = 2
	WM_SFEX_DATALOSS                             = 4
	WM_DM_NOTINTERLACED                          = 0
	WM_DM_DEINTERLACE_NORMAL                     = 1
	WM_DM_DEINTERLACE_HALFSIZE                   = 2
	WM_DM_DEINTERLACE_HALFSIZEDOUBLERATE         = 3
	WM_DM_DEINTERLACE_INVERSETELECINE            = 4
	WM_DM_DEINTERLACE_VERTICALHALFSIZEDOUBLERATE = 5
	WM_DM_IT_DISABLE_COHERENT_MODE               = 0
	WM_DM_IT_FIRST_FRAME_IN_CLIP_IS_AA_TOP       = 1
	WM_DM_IT_FIRST_FRAME_IN_CLIP_IS_BB_TOP       = 2
	WM_DM_IT_FIRST_FRAME_IN_CLIP_IS_BC_TOP       = 3
	WM_DM_IT_FIRST_FRAME_IN_CLIP_IS_CD_TOP       = 4
	WM_DM_IT_FIRST_FRAME_IN_CLIP_IS_DD_TOP       = 5
	WM_DM_IT_FIRST_FRAME_IN_CLIP_IS_AA_BOTTOM    = 6
	WM_DM_IT_FIRST_FRAME_IN_CLIP_IS_BB_BOTTOM    = 7
	WM_DM_IT_FIRST_FRAME_IN_CLIP_IS_BC_BOTTOM    = 8
	WM_DM_IT_FIRST_FRAME_IN_CLIP_IS_CD_BOTTOM    = 9
	WM_DM_IT_FIRST_FRAME_IN_CLIP_IS_DD_BOTTOM    = 10
	WM_PLAYBACK_DRC_HIGH                         = 0
	WM_PLAYBACK_DRC_MEDIUM                       = 1
	WM_PLAYBACK_DRC_LOW                          = 2
	WM_AETYPE_INCLUDE                            = 105
	WM_AETYPE_EXCLUDE                            = 101
)

const (
	SM_CXICON   = 11
	SM_CYICON   = 12
	SM_CXSMICON = 49
	SM_CYSMICON = 50
)

const (
	ICON_SMALL = iota
	ICON_BIG
)

const (
	GCLP_HICON   = -14
	GCLP_HICONSM = -34
)

// WM_NCHITTEST and MOUSEHOOKSTRUCT Mouse Position Codes
const (
	HTERROR = iota - 2
	HTTRANSPARENT
	HTNOWHERE
	HTCLIENT
	HTCAPTION
	HTSYSMENU
	HTGROWBOX
	HTMENU
	HTHSCROLL
	HTVSCROLL
	HTMINBUTTON
	HTMAXBUTTON
	HTLEFT
	HTRIGHT
	HTTOP
	HTTOPLEFT
	HTTOPRIGHT
	HTBOTTOM
	HTBOTTOMLEFT
	HTBOTTOMRIGHT
	HTBORDER
	HTOBJECT
	HTCLOSE
	HTHELP
	HTSIZE      = HTGROWBOX
	HTREDUCE    = HTMINBUTTON
	HTZOOM      = HTMAXBUTTON
	HTSIZEFIRST = HTLEFT
	HTSIZELAST  = HTBOTTOMRIGHT
)

const (
	MAPVK_VK_TO_VSC = iota
	MAPVK_VSC_TO_VK
	MAPVK_VK_TO_CHAR
	MAPVK_VSC_TO_VK_EX
	MAPVK_VK_TO_VSC_EX
)

const WHEEL_DELTA = 120

// Windows hook types https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-setwindowshookexw
const WH_MOUSE = 7

// HC_ACTION indicates the hook procedure must process the message.
const HC_ACTION = 0

// MOUSEHOOKSTRUCTEX https://learn.microsoft.com/windows/win32/api/winuser/ns-winuser-mousehookstructex
type MOUSEHOOKSTRUCTEX struct {
	Pt          POINT
	Hwnd        windows.HWND
	HitTestCode uint32
	ExtraInfo   uintptr
	MouseData   uint32
}

const (
	VK_SHIFT      = 0x10
	VK_CONTROL    = 0x11
	VK_MENU       = 0x12
	VK_CAPITAL    = 0x14
	VK_SNAPSHOT   = 0x2C
	VK_LWIN       = 0x5B
	VK_RWIN       = 0x5C
	VK_NUMLOCK    = 0x90
	VK_LSHIFT     = 0xA0
	VK_RSHIFT     = 0xA1
	VK_PROCESSKEY = 0xE5
)

const (
	UNICODE_NOCHAR = 0xFFFF
	KF_EXTENDED    = 0x0100
	KF_REPEAT      = 0x4000
	KF_UP          = 0x8000
)

const (
	XBUTTON1 = 1 << iota
	XBUTTON2
)

// WM_SIZE message wParam values
const (
	SIZE_RESTORED = iota
	SIZE_MINIMIZED
	SIZE_MAXIMIZED
	SIZE_MAXSHOW
	SIZE_MAXHIDE
)

// https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-showwindow
const (
	SW_HIDE               = 0
	SW_SHOWNORMAL         = 1
	SW_NORMAL             = 1
	SW_SHOWMINIMIZED      = 2
	SW_SHOWMAXIMIZED      = 3
	SW_MAXIMIZE           = 3
	SW_SHOWNOACTIVATE     = 4
	SW_SHOW               = 5
	SW_MINIMIZE           = 6
	SW_SHOWMINNOACTIVE    = 7
	SW_SHOWNA             = 8
	SW_RESTORE            = 9
	SW_SHOWDEFAULT        = 10
	SW_FORCEMINIMIZE      = 11
	SW_MAX                = 11
	SW_PARENTCLOSING      = 1
	SW_OTHERZOOM          = 2
	SW_PARENTOPENING      = 3
	SW_OTHERUNZOOM        = 4
	SW_SCROLLCHILDREN     = 1
	SW_INVALIDATE         = 2
	SW_ERASE              = 4
	SW_SMOOTHSCROLL       = 16
	SW_AUTOPROF_LOAD_MASK = 1
	SW_AUTOPROF_SAVE_MASK = 2
)

// Constants for SetWindowPos flags https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-setwindowpos
const (
	SWP_NOSIZE         = 0x0001
	SWP_NOMOVE         = 0x0002
	SWP_NOZORDER       = 0x0004
	SWP_NOREDRAW       = 0x0008
	SWP_NOACTIVATE     = 0x0010
	SWP_DRAWFRAME      = 0x0020
	SWP_FRAMECHANGED   = 0x0020
	SWP_SHOWWINDOW     = 0x0040
	SWP_HIDEWINDOW     = 0x0080
	SWP_NOCOPYBITS     = 0x0100
	SWP_NOOWNERZORDER  = 0x0200
	SWP_NOREPOSITION   = 0x0200
	SWP_NOSENDCHANGING = 0x0400
	SWP_DEFERERASE     = 0x2000
	SWP_ASYNCWINDOWPOS = 0x4000
)

// Message filter actions for ChangeWindowMessageFilterEx
// https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-changewindowmessagefilterex
const (
	MSGFLT_RESET = iota
	MSGFLT_ALLOW
	MSGFLT_DISALLOW
)

// MonitorFromWindow flags https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-monitorfromwindow
const (
	MONITOR_DEFAULTTONULL = iota
	MONITOR_DEFAULTTOPRIMARY
	MONITOR_DEFAULTTONEAREST
)

const (
	HWND_NOTOPMOST windows.HWND = windows.HWND(^uintptr(1)) // -2 as uintptr
	HWND_TOPMOST   windows.HWND = windows.HWND(^uintptr(0)) // -1 as uintptr
	HWND_TOP       windows.HWND = 0
	HWND_BOTTOM    windows.HWND = 1
)

const (
	DISPLAY_DEVICE_ACTIVE         = 0x00000001
	DISPLAY_DEVICE_MODESPRUNED    = 0x08000000
	DISPLAY_DEVICE_PRIMARY_DEVICE = 0x00000004
)

const (
	MONITORINFOF_PRIMARY = 0x00000001
)

type MONITOR_DPI_TYPE int32

const (
	MDT_EFFECTIVE_DPI MONITOR_DPI_TYPE = 0
	MDT_ANGULAR_DPI   MONITOR_DPI_TYPE = 1
	MDT_RAW_DPI       MONITOR_DPI_TYPE = 2
	MDT_DEFAULT       MONITOR_DPI_TYPE = MDT_EFFECTIVE_DPI
)

const (
	TME_HOVER     = 1
	TME_LEAVE     = 2
	TME_NONCLIENT = 0x00000010
	TME_QUERY     = 0x40000000
	TME_CANCEL    = 0x80000000
)

// ICONINFO https://learn.microsoft.com/windows/win32/api/winuser/ns-winuser-iconinfo
type ICONINFO struct {
	Icon     int32 // 1 for icon, 0 for cursor.
	XHotspot uint32
	YHotspot uint32
	Mask     HBITMAP
	Color    HBITMAP
}

// WNDCLASSEX https://learn.microsoft.com/windows/win32/api/winuser/ns-winuser-wndclassexw
//
//nolint:govet // The field order is dictated by the Windows API and cannot be changed.
type WNDCLASSEX struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   HINSTANCE
	Icon       HICON
	Cursor     HCURSOR
	Background HBRUSH
	MenuName   UTF16String
	ClassName  UTF16String
	IconSm     HICON
}

// POINT https://learn.microsoft.com/windows/win32/api/windef/ns-windef-point
type POINT struct {
	X int32
	Y int32
}

// SIZE https://learn.microsoft.com/windows/win32/api/windef/ns-windef-size
type SIZE struct {
	CX int32
	CY int32
}

// RECT https://learn.microsoft.com/windows/win32/api/windef/ns-windef-rect
type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type ChangeFilterStruct struct {
	Size      uint32
	ExtStatus uint32
}

// WINDOWPLACEMENT https://learn.microsoft.com/windows/win32/api/winuser/ns-winuser-windowplacement
type WINDOWPLACEMENT struct {
	Length         uint32
	Flags          uint32
	ShowCmd        uint32
	MinPosition    POINT
	MaxPosition    POINT
	NormalPosition RECT
	Device         RECT
}

// WINDOWPOS https://learn.microsoft.com/windows/win32/api/winuser/ns-winuser-windowpos
type WINDOWPOS struct {
	Hwnd            windows.HWND
	HwndInsertAfter windows.HWND
	X               int32
	Y               int32
	CX              int32
	CY              int32
	Flags           uint32
}

// DISPLAY_DEVICEW https://learn.microsoft.com/en-us/windows/win32/api/wingdi/ns-wingdi-display_devicew
type DISPLAY_DEVICEW struct {
	size         uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

// MONITORINFO https://docs.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-monitorinfo
type MONITORINFO struct {
	size    uint32
	Monitor RECT
	Work    RECT
	Flags   uint32
}

// MSG https://docs.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-msg
type MSG struct {
	Hwnd    windows.HWND
	Message uint32
	WParam  WPARAM
	LParam  LPARAM
	Time    uint32
	Pt      POINT
	Private uint32
}

// MINMAXINFO https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-minmaxinfo
type MINMAXINFO struct {
	Reserved     POINT
	MaxSize      POINT
	MaxPosition  POINT
	MinTrackSize POINT
	MaxTrackSize POINT
}

// TRACKMOUSEEVENT https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-trackmouseevent
type TRACKMOUSEEVENT struct {
	size      uint32
	Flags     uint32
	HwndTrack windows.HWND
	HoverTime uint32
}

// AdjustWindowRectEx https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-adjustwindowrectex
func AdjustWindowRectEx(rect *RECT, style uint32, hasMenu bool, exStyle uint32) bool {
	var menu uint32
	if hasMenu {
		menu = 1
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := adjustWindowRectExProc.Call(uintptr(unsafe.Pointer(rect)), uintptr(style), uintptr(menu),
		uintptr(exStyle))
	return b&0xff != 0
}

// AdjustWindowRectExForDpi https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-adjustwindowrectexfordpi
func AdjustWindowRectExForDpi(rect *RECT, style uint32, hasMenu bool, exStyle, dpi uint32) bool {
	var menu uint32
	if hasMenu {
		menu = 1
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := adjustWindowRectExForDpiProc.Call(uintptr(unsafe.Pointer(rect)), uintptr(style), uintptr(menu),
		uintptr(exStyle), uintptr(dpi))
	return b&0xff != 0
}

// AttachThreadInput https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-attachthreadinput
func AttachThreadInput(idAttach, idAttachTo uint32, attach bool) bool {
	var fAttach uintptr
	if attach {
		fAttach = 1
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := attachThreadInputProc.Call(uintptr(idAttach), uintptr(idAttachTo), fAttach)
	return b&0xff != 0
}

// BringWindowToTop https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-bringwindowtotop
func BringWindowToTop(hwnd windows.HWND) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := bringWindowToTopProc.Call(uintptr(hwnd))
	return b&0xff != 0
}

// ChangeWindowMessageFilterEx https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-changewindowmessagefilterex
func ChangeWindowMessageFilterEx(hwnd windows.HWND, msg, action uint32, info *ChangeFilterStruct) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := changeWindowMessageFilterExProc.Call(uintptr(hwnd), uintptr(msg), uintptr(action), uintptr(unsafe.Pointer(info)))
	return b&0xff != 0
}

// ClientToScreen https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-clienttoscreen
func ClientToScreen(hwnd windows.HWND, point *POINT) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := clientToScreenProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(point)))
	return b&0xff != 0
}

// CloseClipboard https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-closeclipboard
func CloseClipboard() bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := closeClipboardProc.Call()
	return b&0xff != 0
}

// CreateCursor https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-createcursor
func CreateCursor(instance HINSTANCE, xHotspot, yHotspot, width, height uint32, andMask, xorMask []byte) HCURSOR {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := createCursorProc.Call(uintptr(instance), uintptr(xHotspot), uintptr(yHotspot), uintptr(width),
		uintptr(height), uintptr(unsafe.Pointer(&andMask[0])), uintptr(unsafe.Pointer(&xorMask[0])))
	return HCURSOR(ret)
}

// CreateIconIndirect https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-createiconindirect
func CreateIconIndirect(info *ICONINFO) HICON {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := createIconIndirectProc.Call(uintptr(unsafe.Pointer(info)))
	return HICON(ret)
}

// CreateWindowExW https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-createwindowexw
func CreateWindowExW(exStyle uint32, className, windowName string, style uint32, x, y, width, height int32, parent windows.HWND, menu HMENU, instance HINSTANCE, param LPVOID) windows.HWND {
	var lpClassName *uint16
	if className != "" {
		var err error
		if lpClassName, err = windows.UTF16PtrFromString(className); err != nil {
			return 0
		}
		defer runtime.KeepAlive(lpClassName)
	}

	var lpWindowName *uint16
	if windowName != "" {
		var err error
		if lpWindowName, err = windows.UTF16PtrFromString(windowName); err != nil {
			return 0
		}
		defer runtime.KeepAlive(lpWindowName)
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := createWindowExWProc.Call(uintptr(exStyle), uintptr(unsafe.Pointer(lpClassName)),
		uintptr(unsafe.Pointer(lpWindowName)), uintptr(style), uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		uintptr(parent), uintptr(menu), uintptr(instance), uintptr(param),
	)
	return windows.HWND(ret)
}

// DefWindowProcW https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-defwindowprocw
func DefWindowProcW(hwnd windows.HWND, msg uint32, wParam WPARAM, lParam LPARAM) uintptr {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := defWindowProcWProc.Call(uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lParam))
	return ret
}

// DestroyIcon https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-destroyicon
func DestroyIcon(icon HICON) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := destroyIconProc.Call(uintptr(icon))
	return b&0xff != 0
}

// DestroyWindow https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-destroywindow
func DestroyWindow(hwnd windows.HWND) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := destroyWindowProc.Call(uintptr(hwnd))
	return b&0xff != 0
}

// DispatchMessageW https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-dispatchmessagew
func DispatchMessageW(msg *MSG) LRESULT {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := dispatchMessageWProc.Call(uintptr(unsafe.Pointer(msg)))
	return LRESULT(ret)
}

// EmptyClipboard https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-emptyclipboard
func EmptyClipboard() bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := emptyClipboardProc.Call()
	return b&0xff != 0
}

// EnumClipboardFormats https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-enumclipboardformats
func EnumClipboardFormats(format ClipboardFormat) ClipboardFormat {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := enumClipboardFormatsProc.Call(uintptr(format))
	return ClipboardFormat(r)
}

// EnumDisplayDevicesW https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-enumdisplaydevicesw
func EnumDisplayDevicesW(device string, iDevNum, dwFlags uint32, displayDevice *DISPLAY_DEVICEW) bool {
	var lpDevice *uint16
	if device != "" {
		var err error
		lpDevice, err = windows.UTF16PtrFromString(device)
		if err != nil {
			return false
		}
		runtime.KeepAlive(lpDevice)
	}
	displayDevice.size = uint32(unsafe.Sizeof(displayDevice))
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := enumDisplayDevicesWProc.Call(uintptr(unsafe.Pointer(lpDevice)), uintptr(iDevNum),
		uintptr(unsafe.Pointer(displayDevice)), uintptr(dwFlags))
	return b&0xff != 0
}

// EnumDisplayMonitors https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-enumdisplaymonitors
func EnumDisplayMonitors(hdc HDC, lprcClip *RECT, callback, dwData uintptr) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := enumDisplayMonitorsProc.Call(uintptr(hdc), uintptr(unsafe.Pointer(lprcClip)), callback, dwData)
	return ret&0xff != 0
}

// NewEnumDisplayMonitorsCallback creates a new callback for EnumDisplayMonitors. There are a limited number of
// callbacks that may be created on Windows, so allocate these once and reuse them where possible.
func NewEnumDisplayMonitorsCallback(callback func(monitor HMONITOR, hdc HDC, bounds RECT, lParam uintptr) bool) uintptr {
	return syscall.NewCallback(
		func(monitor HMONITOR, hdc HDC, bounds *RECT, lParam uintptr) uintptr {
			var r RECT
			if bounds != nil {
				r = *bounds
			}
			if callback(monitor, hdc, r, lParam) {
				return 1
			}
			return 0
		},
	)
}

// GetActiveWindow https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getactivewindow
func GetActiveWindow() windows.HWND {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	hwnd, _, _ := getActiveWindowProc.Call()
	return windows.HWND(hwnd)
}

// GetForegroundWindow https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getforegroundwindow
func GetForegroundWindow() windows.HWND {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	hwnd, _, _ := getForegroundWindowProc.Call()
	return windows.HWND(hwnd)
}

// GetWindowThreadProcessId https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowthreadprocessid
func GetWindowThreadProcessId(hwnd windows.HWND) uint32 {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	tid, _, _ := getWindowThreadProcessIdProc.Call(uintptr(hwnd), 0)
	return uint32(tid)
}

// GetClassLongPtrW https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getclasslongptrw
func GetClassLongPtrW(hwnd windows.HWND, index int) uintptr {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := getClassLongPtrWProc.Call(uintptr(hwnd), uintptr(index))
	return ret
}

// GetClientRect https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getclientrect
func GetClientRect(hwnd windows.HWND, rect *RECT) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := getClientRectProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(rect)))
	return b&0xff != 0
}

// GetClipboardData https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getclipboarddata
func GetClipboardData(format ClipboardFormat) syscall.Handle {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	h, _, _ := getClipboardDataProc.Call(uintptr(format))
	return syscall.Handle(h)
}

// GetClipboardFormatNameW https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getclipboardformatnamew
func GetClipboardFormatNameW(format ClipboardFormat) string {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	buf := make([]uint16, 256)
	n, _, _ := getClipboardFormatNameWProc.Call(uintptr(format), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// GetClipboardSequenceNumber https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getclipboardsequencenumber
func GetClipboardSequenceNumber() int {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	num, _, _ := getClipboardSequenceNumberProc.Call()
	return int(num)
}

// GetCursorPos https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getcursorpos
func GetCursorPos(point *POINT) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := getCursorPosProc.Call(uintptr(unsafe.Pointer(point)))
	return b&0xff != 0
}

// GetDC https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-getdc
func GetDC(hwnd windows.HWND) HDC {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	dc, _, _ := getDCProc.Call(uintptr(hwnd))
	return HDC(dc)
}

// GetDoubleClickTime https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getdoubleclicktime
func GetDoubleClickTime() time.Duration {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	millis, _, _ := getDoubleClickTimeProc.Call()
	return time.Millisecond * time.Duration(millis)
}

// GetDpiForWindow https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getdpiforwindow
func GetDpiForWindow(hwnd windows.HWND) uint32 {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	dpi, _, _ := getDpiForWindowProc.Call(uintptr(hwnd))
	return uint32(dpi)
}

// GetKeyState https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getkeystate
func GetKeyState(virtualKey int) uint16 {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	state, _, _ := getKeyStateProc.Call(uintptr(virtualKey))
	return uint16(state & 0xFFFF)
}

// GetMessageTime https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getmessagetime
func GetMessageTime() uint32 {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	millis, _, _ := getMessageTimeProc.Call()
	return uint32(millis)
}

// GetMonitorInfoW https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getmonitorinfow
func GetMonitorInfoW(monitor HMONITOR, monitorInfo *MONITORINFO) bool {
	monitorInfo.size = uint32(unsafe.Sizeof(*monitorInfo))
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := getMonitorInfoWProc.Call(uintptr(monitor), uintptr(unsafe.Pointer(monitorInfo)))
	return b&0xff != 0
}

// GetSysColor https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getsyscolor
func GetSysColor(index int) uint32 {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	color, _, _ := getSysColorProc.Call(uintptr(index))
	return uint32(color)
}

// GetSystemMetrics https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getsystemmetrics
func GetSystemMetrics(index int) int {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	metric, _, _ := getSystemMetricsProc.Call(uintptr(index))
	return int(metric)
}

// GetWindowPlacement https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowplacement
func GetWindowPlacement(hwnd windows.HWND, placement *WINDOWPLACEMENT) bool {
	placement.Length = uint32(unsafe.Sizeof(*placement))
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := getWindowPlacementProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(placement)))
	return b&0xff != 0
}

// GetWindowRect https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowrect
func GetWindowRect(hwnd windows.HWND, rect *RECT) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := getWindowRectProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(rect)))
	return b&0xff != 0
}

// IsClipboardFormatAvailable https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-isclipboardformatavailable
func IsClipboardFormatAvailable(format ClipboardFormat) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := isClipboardFormatAvailableProc.Call(uintptr(format))
	return b&0xff != 0
}

// LoadImageW https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-loadimagew
func LoadImageW(inst HINSTANCE, name UTF16String, typ uint32, cx, cy int, load uint32) windows.Handle {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := loadImageWProc.Call(uintptr(inst), uintptr(unsafe.Pointer(name)), uintptr(typ), uintptr(cx),
		uintptr(cy), uintptr(load))
	return windows.Handle(ret)
}

// MakeIntResourceW https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-makeintresourcew
func MakeIntResourceW(id int) UTF16String {
	return UTF16String(xruntime.PtrFromUintptr[uint16](uintptr(id)))
}

// MapVirtualKeyW https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-mapvirtualkeyw
func MapVirtualKeyW(code, mapType uint32) uint32 {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	mapped, _, _ := mapVirtualKeyWProc.Call(uintptr(code), uintptr(mapType))
	return uint32(mapped)
}

// MessageBeep https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-messagebeep
func MessageBeep(beepType BeepType) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := messageBeepProc.Call(uintptr(beepType))
	return b&0xff != 0
}

// MonitorFromWindow https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-monitorfromwindow
func MonitorFromWindow(hwnd windows.HWND, flags uint32) HMONITOR {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	monitor, _, _ := monitorFromWindowProc.Call(uintptr(hwnd), uintptr(flags))
	return HMONITOR(monitor)
}

// OpenClipboard https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-openclipboard
func OpenClipboard(newOwner windows.HWND) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := openClipboardProc.Call(uintptr(newOwner))
	return b&0xff != 0
}

// PeekMessageW https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-peekmessagew
func PeekMessageW(msg *MSG, hwnd windows.HWND, msgFilterMin, msgFilterMax, removeMsg uint32) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := peekMessageWProc.Call(uintptr(unsafe.Pointer(msg)), uintptr(hwnd), uintptr(msgFilterMin),
		uintptr(msgFilterMax), uintptr(removeMsg))
	return b&0xff != 0
}

// PostMessageW https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-postmessagew
func PostMessageW(hwnd windows.HWND, msg uint32, wParam WPARAM, lParam LPARAM) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := postMessageWProc.Call(uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lParam))
	return b&0xff != 0
}

// PostThreadMessageW https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-postthreadmessagew
func PostThreadMessageW(threadID, msg uint32, wParam WPARAM, lParam LPARAM) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := postThreadMessageWProc.Call(uintptr(threadID), uintptr(msg), uintptr(wParam), uintptr(lParam))
	return b&0xff != 0
}

// SetWindowsHookExW https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-setwindowshookexw
func SetWindowsHookExW(idHook int, fn uintptr, mod HINSTANCE, threadID uint32) HHOOK {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := setWindowsHookExWProc.Call(uintptr(idHook), fn, uintptr(mod), uintptr(threadID))
	return HHOOK(ret)
}

// UnhookWindowsHookEx https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-unhookwindowshookex
func UnhookWindowsHookEx(hook HHOOK) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := unhookWindowsHookExProc.Call(uintptr(hook))
	return b&0xff != 0
}

// CallNextHookEx https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-callnexthookex
func CallNextHookEx(hook HHOOK, code int, wParam WPARAM, lParam LPARAM) uintptr {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := callNextHookExProc.Call(uintptr(hook), uintptr(code), uintptr(wParam), uintptr(lParam))
	return ret
}

// RegisterClassExW https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-registerclassexw
func RegisterClassExW(wndClassEx *WNDCLASSEX) ATOM {
	wndClassEx.Size = uint32(unsafe.Sizeof(*wndClassEx))
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := registerClassExWProc.Call(uintptr(unsafe.Pointer(wndClassEx)))
	return ATOM(ret)
}

// RegisterClipboardFormatW https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-registerclipboardformatw
func RegisterClipboardFormatW(name string) ClipboardFormat {
	var lpString *uint16
	if name != "" {
		var err error
		if lpString, err = windows.UTF16PtrFromString(name); err != nil {
			return CFNone
		}
		defer runtime.KeepAlive(lpString)
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := registerClipboardFormatWProc.Call(uintptr(unsafe.Pointer(lpString)))
	return ClipboardFormat(b)
}

// ReleaseDC https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-releasedc
func ReleaseDC(hwnd windows.HWND, dc HDC) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := releaseDCProc.Call(uintptr(hwnd), uintptr(dc))
	return ret == 1
}

// ScreenToClient https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-screentoclient
func ScreenToClient(hwnd windows.HWND, point *POINT) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := screenToClientProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(point)))
	return b&0xff != 0
}

// SendMessageW https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-sendmessagew
func SendMessageW(hwnd windows.HWND, msg uint32, wParam WPARAM, lParam LPARAM) LRESULT {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := sendMessageWProc.Call(uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lParam))
	return LRESULT(ret)
}

// SetClipboardData https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setclipboarddata
func SetClipboardData(format ClipboardFormat, handle syscall.Handle) syscall.Handle {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	h, _, _ := setClipboardDataProc.Call(uintptr(format), uintptr(handle))
	return syscall.Handle(h)
}

// SetCursor https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setcursor
func SetCursor(cursor HCURSOR) HCURSOR {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	prev, _, _ := setCursorProc.Call(uintptr(cursor))
	return HCURSOR(prev)
}

// SetFocus https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setfocus
func SetFocus(hwnd windows.HWND) windows.HWND {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	prev, _, _ := setFocusProc.Call(uintptr(hwnd))
	return windows.HWND(prev)
}

// SetForegroundWindow https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setforegroundwindow
func SetForegroundWindow(hwnd windows.HWND) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := setForegroundWindowProc.Call(uintptr(hwnd))
	return b&0xff != 0
}

// SetProcessDpiAwarenessContext https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setprocessdpiawarenesscontext
func SetProcessDpiAwarenessContext(value DPI_AWARENESS_CONTEXT) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := setProcessDpiAwarenessContextProc.Call(uintptr(value))
	return b&0xff != 0
}

// SetWindowPlacement https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowplacement
func SetWindowPlacement(hwnd windows.HWND, placement *WINDOWPLACEMENT) bool {
	placement.Length = uint32(unsafe.Sizeof(*placement))
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := setWindowPlacementProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(placement)))
	return b&0xff != 0
}

// SetWindowPos https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowpos
func SetWindowPos(hwnd, hwndInsertAfter windows.HWND, x, y, cx, cy int32, uFlags uint32) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := setWindowPosProc.Call(uintptr(hwnd), uintptr(hwndInsertAfter), uintptr(x), uintptr(y),
		uintptr(cx), uintptr(cy), uintptr(uFlags))
	return b&0xff != 0
}

// SetWindowTextW https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-setwindowtextw
func SetWindowTextW(hwnd windows.HWND, text string) bool {
	var lpString *uint16
	if text != "" {
		var err error
		if lpString, err = windows.UTF16PtrFromString(text); err != nil {
			return false
		}
		defer runtime.KeepAlive(lpString)
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := setWindowTextWProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(lpString)))
	return b&0xff != 0
}

// ShowWindow https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-showwindow
func ShowWindow(hwnd windows.HWND, cmdShow int32) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := showWindowProc.Call(uintptr(hwnd), uintptr(cmdShow))
	return b&0xff != 0
}

// TrackMouseEvent https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-trackmouseevent
func TrackMouseEvent(evt *TRACKMOUSEEVENT) bool {
	evt.size = uint32(unsafe.Sizeof(*evt))
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := trackMouseEventProc.Call(uintptr(unsafe.Pointer(evt)))
	return b&0xff != 0
}

// TranslateMessage https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-translatemessage
func TranslateMessage(msg *MSG) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := translateMessageProc.Call(uintptr(unsafe.Pointer(msg)))
	return b&0xff != 0
}

// WaitMessage https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-waitmessage
func WaitMessage() bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := waitMessageProc.Call()
	return b&0xff != 0
}

// WindowFromPoint https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-windowfrompoint
func WindowFromPoint(point POINT) windows.HWND {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	hwnd, _, _ := windowFromPointProc.Call(uintptr(point.X), uintptr(point.Y))
	return windows.HWND(hwnd)
}
