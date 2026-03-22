// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

// Constants for X11 request opcodes.
const (
	opcodeCreateWindow = 1 + iota
	opcodeChangeWindowAttributes
	opcodeGetWindowAttributes
	opcodeDestroyWindow
	opcodeDestroySubwindows
	opcodeChangeSaveSet
	opcodeReparentWindow
	opcodeMapWindow
	opcodeMapSubwindows
	opcodeUnmapWindow
	opcodeUnmapSubwindows
	opcodeConfigureWindow
	opcodeCirculateWindow
	opcodeGetGeometry
	opcodeQueryTree
	opcodeInternAtom
	opcodeGetAtomName
	opcodeChangeProperty
	opcodeDeleteProperty
	opcodeGetProperty
	opcodeListProperties
	opcodeSetSelectionOwner
	opcodeGetSelectionOwner
	opcodeConvertSelection
	opcodeSendEvent
	opcodeGrabPointer
	opcodeUngrabPointer
	opcodeGrabButton
	opcodeUngrabButton
	opcodeChangeActivePointerGrab
	opcodeGrabKeyboard
	opcodeUngrabKeyboard
	opcodeGrabKey
	opcodeUngrabKey
	opcodeAllowEvents
	opcodeGrabServer
	opcodeUngrabServer
	opcodeQueryPointer
	opcodeGetMotionEvents
	opcodeTranslateCoordinates
	opcodeWarpPointer
	opcodeSetInputFocus
	opcodeGetInputFocus
	opcodeQueryKeymap
	opcodeOpenFont
	opcodeCloseFont
	opcodeQueryFont
	opcodeQueryTextExtents
	opcodeListFonts
	opcodeListFontsWithInfo
	opcodeSetFontPath
	opcodeGetFontPath
	opcodeCreatePixmap
	opcodeFreePixmap
	opcodeCreateGC
	opcodeChangeGC
	opcodeCopyGC
	opcodeSetDashes
	opcodeSetClipRectangles
	opcodeFreeGC
	opcodeClearArea
	opcodeCopyArea
	opcodeCopyPlane
	opcodePolyPoint
	opcodePolyLine
	opcodePolySegment
	opcodePolyRectangle
	opcodePolyArc
	opcodeFillPoly
	opcodePolyFillRectangle
	opcodePolyFillArc
	opcodePutImage
	opcodeGetImage
	opcodePolyText8
	opcodePolyText16
	opcodeImageText8
	opcodeImageText16
	opcodeCreateColormap
	opcodeFreeColormap
	opcodeCopyColormapAndFree
	opcodeInstallColormap
	opcodeUninstallColormap
	opcodeListInstalledColormaps
	opcodeAllocColor
	opcodeAllocNamedColor
	opcodeAllocColorCells
	opcodeAllocColorPlanes
	opcodeFreeColors
	opcodeStoreColors
	opcodeStoreNamedColor
	opcodeQueryColors
	opcodeLookupColor
	opcodeCreateCursor
	opcodeCreateGlyphCursor
	opcodeFreeCursor
	opcodeRecolorCursor
	opcodeQueryBestSize
	opcodeQueryExtension
	opcodeListExtensions
	opcodeChangeKeyboardMapping
	opcodeGetKeyboardMapping
	opcodeChangeKeyboardControl
	opcodeGetKeyboardControl
	opcodeBell
	opcodeChangePointerControl
	opcodeGetPointerControl
	opcodeSetScreenSaver
	opcodeGetScreenSaver
	opcodeChangeHosts
	opcodeListHosts
	opcodeSetAccessControl
	opcodeSetCloseDownMode
	opcodeKillClient
	opcodeRotateProperties
	opcodeForceScreenSaver
	opcodeSetPointerMapping
	opcodeGetPointerMapping
	opcodeSetModifierMapping
	opcodeGetModifierMapping
	opcodeUndefined1
	opcodeUndefined2
	opcodeUndefined3
	opcodeUndefined4
	opcodeUndefined5
	opcodeUndefined6
	opcodeUndefined7
	opcodeNoOperation
)

// Constants for X11 event codes.
const (
	eventCodeKeyPress = 2 + iota
	eventCodeKeyRelease
	eventCodeButtonPress
	eventCodeButtonRelease
	eventCodeMotionNotify
	eventCodeEnterNotify
	eventCodeLeaveNotify
	eventCodeFocusIn
	eventCodeFocusOut
	eventCodeKeymapNotify
	eventCodeExpose
	eventCodeGraphicsExposure
	eventCodeNoExposure
	eventCodeVisibilityNotify
	eventCodeCreateNotify
	eventCodeDestroyNotify
	eventCodeUnmapNotify
	eventCodeMapNotify
	eventCodeMapRequest
	eventCodeReparentNotify
	eventCodeConfigureNotify
	eventCodeConfigureRequest
	eventCodeGravityNotify
	eventCodeResizeRequest
	eventCodeCirculateNotify
	eventCodeCirculateRequest
	eventCodePropertyNotify
	eventCodeSelectionClear
	eventCodeSelectionRequest
	eventCodeSelectionNotify
	eventCodeColormapNotify
	eventCodeClientMessage
	eventCodeMappingNotify
	eventCodeNone = 0
)

// Constants for X11 error codes.
const (
	errorCodeRequest = 1 + iota
	errorCodeValue
	errorCodeWindow
	errorCodePixmap
	errorCodeAtom
	errorCodeCursor
	errorCodeFont
	errorCodeMatch
	errorCodeDrawable
	errorCodeAccess
	errorCodeAlloc
	errorCodeColormap
	errorCodeGContext
	errorCodeIDChoice
	errorCodeName
	errorCodeLength
	errorCodeImplementation
)

// Constants for X11 property events.
const (
	propertyNewValue = iota
	propertyDelete
)

func newEventMap() map[byte]func(*Reader) Event {
	return map[byte]func(r *Reader) Event{
		eventCodeKeyPress:         newKeyPressEvent,
		eventCodeKeyRelease:       newKeyReleaseEvent,
		eventCodeButtonPress:      newButtonPressEvent,
		eventCodeButtonRelease:    newButtonReleaseEvent,
		eventCodeMotionNotify:     newMotionNotifyEvent,
		eventCodeEnterNotify:      newEnterNotifyEvent,
		eventCodeLeaveNotify:      newLeaveNotifyEvent,
		eventCodeFocusIn:          newFocusInEvent,
		eventCodeFocusOut:         newFocusOutEvent,
		eventCodeKeymapNotify:     newKeymapNotifyEvent,
		eventCodeExpose:           newExposeEvent,
		eventCodeGraphicsExposure: newGraphicsExposureEvent,
		eventCodeNoExposure:       newNoExposureEvent,
		eventCodeVisibilityNotify: newVisibilityNotifyEvent,
		eventCodeCreateNotify:     newCreateNotifyEvent,
		eventCodeDestroyNotify:    newDestroyNotifyEvent,
		eventCodeUnmapNotify:      newUnmapNotifyEvent,
		eventCodeMapNotify:        newMapNotifyEvent,
		eventCodeMapRequest:       newMapRequestEvent,
		eventCodeReparentNotify:   newReparentNotifyEvent,
		eventCodeConfigureNotify:  newConfigureNotifyEvent,
		eventCodeConfigureRequest: newConfigureRequestEvent,
		eventCodeGravityNotify:    newGravityNotifyEvent,
		eventCodeResizeRequest:    newResizeRequestEvent,
		eventCodeCirculateNotify:  newCirculateNotifyEvent,
		eventCodeCirculateRequest: newCirculateRequestEvent,
		eventCodePropertyNotify:   newPropertyNotifyEvent,
		eventCodeSelectionClear:   newSelectionClearEvent,
		eventCodeSelectionRequest: newSelectionRequestEvent,
		eventCodeSelectionNotify:  newSelectionNotifyEvent,
		eventCodeColormapNotify:   newColormapNotifyEvent,
		eventCodeClientMessage:    newClientMessageEvent,
		eventCodeMappingNotify:    newMappingNotifyEvent,
	}
}

func newErrorMap() map[byte]string {
	return map[byte]string{
		errorCodeRequest:        "request",
		errorCodeValue:          "value",
		errorCodeWindow:         "window",
		errorCodePixmap:         "pixmap",
		errorCodeAtom:           "atom",
		errorCodeCursor:         "cursor",
		errorCodeFont:           "font",
		errorCodeMatch:          "match",
		errorCodeDrawable:       "drawable",
		errorCodeAccess:         "access",
		errorCodeAlloc:          "alloc",
		errorCodeColormap:       "colormap",
		errorCodeGContext:       "gcontext",
		errorCodeIDChoice:       "id choice",
		errorCodeName:           "name",
		errorCodeLength:         "length",
		errorCodeImplementation: "implementation",
	}
}
