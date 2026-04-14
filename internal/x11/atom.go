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
	AtomWmCommand
	AtomWmHints
	AtomWmClientMachine
	AtomWmIconName
	AtomWmIconSize
	AtomWmName
	AtomWmNormalHints
	AtomWmSizeHints
	AtomWmZoomHints
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
	AtomWmClass
	AtomWmTransientFor
	AtomAny = AtomNone
)

// Atoms holds the Atom values for commonly used X11 Atoms that don't have predefined constants.
type Atoms struct {
	Clipboard            Atom
	ClipboardIncremental Atom
	ClipboardManager     Atom
	ClipboardMultiple    Atom
	ClipboardSaveTargets Atom
	ClipboardSelection   Atom
	ClipboardTargets     Atom
	NetCurrentDesktop    Atom
	NetWorkArea          Atom
	Null                 Atom
	Pair                 Atom
	UTF8String           Atom
}

func (a *Atoms) init(c *Conn) error {
	var err error
	if a.Clipboard, err = c.InternAtom("CLIPBOARD", false); err != nil {
		return err
	}
	if a.ClipboardIncremental, err = c.InternAtom("INCR", false); err != nil {
		return err
	}
	if a.ClipboardManager, err = c.InternAtom("CLIPBOARD_MANAGER", false); err != nil {
		return err
	}
	if a.ClipboardMultiple, err = c.InternAtom("MULTIPLE", false); err != nil {
		return err
	}
	if a.ClipboardSaveTargets, err = c.InternAtom("SAVE_TARGETS", false); err != nil {
		return err
	}
	if a.ClipboardSelection, err = c.InternAtom("CLIPBOARD_SELECTION", false); err != nil {
		return err
	}
	if a.ClipboardTargets, err = c.InternAtom("TARGETS", false); err != nil {
		return err
	}
	if a.NetCurrentDesktop, err = c.InternAtom("_NET_CURRENT_DESKTOP", false); err != nil {
		return err
	}
	if a.NetWorkArea, err = c.InternAtom("_NET_WORKAREA", false); err != nil {
		return err
	}
	if a.Null, err = c.InternAtom("NULL", false); err != nil {
		return err
	}
	if a.Pair, err = c.InternAtom("ATOM_PAIR", false); err != nil {
		return err
	}
	if a.UTF8String, err = c.InternAtom("UTF8_STRING", false); err != nil {
		return err
	}
	return nil
}
