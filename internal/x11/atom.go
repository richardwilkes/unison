// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

// Atom represents an X11 Atom, which is a unique identifier for a string used in various parts of the X11 protocol,
// such as properties and types.
type Atom uint32

// Predefined Atom values as per the X11 protocol specification.
const (
	AtomNone Atom = iota
	AtomPrimary
	AtomSecondary
	AtomArc
	AtomAtom
	AtomBitmap
	AtomCardinal
	AtomColormap
	AtomCursor
	AtomCutBuffer0
	AtomCutBuffer1
	AtomCutBuffer2
	AtomCutBuffer3
	AtomCutBuffer4
	AtomCutBuffer5
	AtomCutBuffer6
	AtomCutBuffer7
	AtomDrawable
	AtomFont
	AtomInteger
	AtomPixmap
	AtomPoint
	AtomRectangle
	AtomResourceManager
	AtomRgbColorMap
	AtomRgbBestMap
	AtomRgbBlueMap
	AtomRgbDefaultMap
	AtomRgbGrayMap
	AtomRgbGreenMap
	AtomRgbRedMap
	AtomString
	AtomVisualid
	AtomWindow
	AtomWMCommand
	AtomWMHints
	AtomWMClientMachine
	AtomWMIconName
	AtomWMIconSize
	AtomWMName
	AtomWMNormalHints
	AtomWMSizeHints
	AtomWMZoomHints
	AtomMinSpace
	AtomNormSpace
	AtomMaxSpace
	AtomEndSpace
	AtomSuperscriptX
	AtomSuperscriptY
	AtomSubscriptX
	AtomSubscriptY
	AtomUnderlinePosition
	AtomUnderlineThickness
	AtomStrikeoutAscent
	AtomStrikeoutDescent
	AtomItalicAngle
	AtomXHeight
	AtomQuadWidth
	AtomWeight
	AtomPointSize
	AtomResolution
	AtomCopyright
	AtomNotice
	AtomFontName
	AtomFamilyName
	AtomFullName
	AtomCapHeight
	AtomWMClass
	AtomWMTransientFor
	AtomAny = AtomNone
)

// Atoms holds the Atom values for commonly used X11 Atoms that don't have predefined constants.
type Atoms struct {
	Clipboard               Atom
	ClipboardIncremental    Atom
	ClipboardManager        Atom
	ClipboardMultiple       Atom
	ClipboardSaveTargets    Atom
	ClipboardSelection      Atom
	ClipboardTargets        Atom
	DnDActionCopy           Atom
	DnDAware                Atom
	DnDDrop                 Atom
	DnDEnter                Atom
	DnDFinished             Atom
	DnDLeave                Atom
	DnDPosition             Atom
	DnDSelection            Atom
	DnDStatus               Atom
	DndTypeList             Atom
	MotifWMHints            Atom
	NetActiveWindow         Atom
	NetCurrentDesktop       Atom
	NetFrameExtents         Atom
	NetRequestFrameExtents  Atom
	NetWMIconName           Atom
	NetWMName               Atom
	NetWMPing               Atom
	NetWMState              Atom
	NetWMStateAbove         Atom
	NetWMStateMaximizedHorz Atom
	NetWMStateMaximizedVert Atom
	NetWMWindowType         Atom
	NetWMWindowTypeDialog   Atom
	NetWMWindowTypeMenu     Atom
	NetWMWindowTypeNormal   Atom
	NetWMWindowTypeTooltip  Atom
	NetWorkArea             Atom
	Null                    Atom
	Pair                    Atom
	TextURIList             Atom
	UTF8String              Atom
	WMChangeState           Atom
	WMDeleteWindow          Atom
	WMPID                   Atom
	WMProtocols             Atom
	WMState                 Atom
}

func (a *Atoms) init(c *Conn) error {
	var err error
	for _, data := range []struct {
		atom *Atom
		name string
	}{
		{&a.Clipboard, "CLIPBOARD"},
		{&a.ClipboardIncremental, "INCR"},
		{&a.ClipboardManager, "CLIPBOARD_MANAGER"},
		{&a.ClipboardMultiple, "MULTIPLE"},
		{&a.ClipboardSaveTargets, "SAVE_TARGETS"},
		{&a.ClipboardSelection, "CLIPBOARD_SELECTION"},
		{&a.ClipboardTargets, "TARGETS"},
		{&a.DnDActionCopy, "XdndActionCopy"},
		{&a.DnDAware, "XdndAware"},
		{&a.DnDDrop, "XdndDrop"},
		{&a.DnDEnter, "XdndEnter"},
		{&a.DnDFinished, "XdndFinished"},
		{&a.DnDLeave, "XdndLeave"},
		{&a.DnDPosition, "XdndPosition"},
		{&a.DnDSelection, "XdndSelection"},
		{&a.DnDStatus, "XdndStatus"},
		{&a.DndTypeList, "XdndTypeList"},
		{&a.MotifWMHints, "_MOTIF_WM_HINTS"},
		{&a.NetActiveWindow, "_NET_ACTIVE_WINDOW"},
		{&a.NetCurrentDesktop, "_NET_CURRENT_DESKTOP"},
		{&a.NetFrameExtents, "_NET_FRAME_EXTENTS"},
		{&a.NetRequestFrameExtents, "_NET_REQUEST_FRAME_EXTENTS"},
		{&a.NetWMIconName, "_NET_WM_ICON_NAME"},
		{&a.NetWMName, "_NET_WM_NAME"},
		{&a.NetWMPing, "_NET_WM_PING"},
		{&a.NetWMState, "_NET_WM_STATE"},
		{&a.NetWMStateAbove, "_NET_WM_STATE_ABOVE"},
		{&a.NetWMStateMaximizedHorz, "_NET_WM_STATE_MAXIMIZED_HORZ"},
		{&a.NetWMStateMaximizedVert, "_NET_WM_STATE_MAXIMIZED_VERT"},
		{&a.NetWMWindowType, "_NET_WM_WINDOW_TYPE"},
		{&a.NetWMWindowTypeDialog, "_NET_WM_WINDOW_TYPE_DIALOG"},
		{&a.NetWMWindowTypeMenu, "_NET_WM_WINDOW_TYPE_MENU"},
		{&a.NetWMWindowTypeNormal, "_NET_WM_WINDOW_TYPE_NORMAL"},
		{&a.NetWMWindowTypeTooltip, "_NET_WM_WINDOW_TYPE_TOOLTIP"},
		{&a.NetWorkArea, "_NET_WORKAREA"},
		{&a.Null, "NULL"},
		{&a.Pair, "ATOM_PAIR"},
		{&a.TextURIList, "text/uri-list"},
		{&a.UTF8String, "UTF8_STRING"},
		{&a.WMChangeState, "WM_CHANGE_STATE"},
		{&a.WMDeleteWindow, "WM_DELETE_WINDOW"},
		{&a.WMPID, "_NET_WM_PID"},
		{&a.WMProtocols, "WM_PROTOCOLS"},
		{&a.WMState, "WM_STATE"},
	} {
		if *data.atom, err = c.InternAtom(data.name, false); err != nil {
			return err
		}
	}
	return nil
}
