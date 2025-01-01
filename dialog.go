// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"errors"
	"strings"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/unison/enums/align"
)

// Pre-defined modal response codes. Apps should start their codes at ModalResponseUserBase.
const (
	ModalResponseDiscard = iota - 1
	ModalResponseOK
	ModalResponseCancel
	ModalResponseUserBase = 100
)

// DialogClientDataKey is the key used in the ClientData() of the Window the dialog puts up which contains the *Dialog
// of the owning dialog.
const DialogClientDataKey = "dialog"

// DefaultDialogTheme holds the default DialogTheme values for Dialogs. Modifying this data will not alter existing
// Dialogs, but will alter any Dialogs created in the future.
var DefaultDialogTheme = DialogTheme{
	ErrorIcon: &DrawableSVG{
		SVG:  CircledExclamationSVG,
		Size: Size{Width: 48, Height: 48},
	},
	ErrorIconInk: ThemeError,
	WarningIcon: &DrawableSVG{
		SVG:  TriangleExclamationSVG,
		Size: Size{Width: 48, Height: 48},
	},
	WarningIconInk: ThemeWarning,
	QuestionIcon: &DrawableSVG{
		SVG:  CircledQuestionSVG,
		Size: Size{Width: 48, Height: 48},
	},
	QuestionIconInk: ThemeOnSurface,
}

// DialogTheme holds theming data for a Dialog.
type DialogTheme struct {
	ErrorIcon       Drawable
	ErrorIconInk    Ink
	WarningIcon     Drawable
	WarningIconInk  Ink
	QuestionIcon    Drawable
	QuestionIconInk Ink
}

type buttonData struct {
	info   *DialogButtonInfo
	button *Button
}

// Dialog holds information about a dialog.
type Dialog struct {
	wnd     *Window
	buttons map[int]*buttonData
	err     error
}

// NewDialog creates a new standard dialog. To show the dialog you must call .RunModal() on the returned dialog.
// 'windowOptions' are additional options to be passed to the Window constructor for the dialog. The dialog will be
// always have te FloatingWindowOption() set.
func NewDialog(icon Drawable, iconInk Ink, msgPanel Paneler, buttonInfo []*DialogButtonInfo, windowOptions ...WindowOption) (*Dialog, error) {
	d := &Dialog{buttons: make(map[int]*buttonData)}
	var frame Rect
	if focused := ActiveWindow(); focused != nil {
		frame = focused.FrameRect()
	} else {
		frame = PrimaryDisplay().Usable
	}
	opts := []WindowOption{FloatingWindowOption()}
	if len(windowOptions) > 0 {
		opts = append(opts, windowOptions...)
	}
	d.wnd, d.err = NewWindow("", opts...)
	if d.err != nil {
		return nil, errs.NewWithCause("unable to create dialog", d.err)
	}
	d.wnd.ClientData()[DialogClientDataKey] = d
	content := d.wnd.Content()
	content.SetBorder(NewEmptyBorder(NewUniformInsets(2 * StdHSpacing)))
	columns := 1
	if icon != nil {
		columns++
		iconLabel := NewLabel()
		iconLabel.Drawable = icon
		iconLabel.OnBackgroundInk = iconInk
		iconLabel.SetBorder(NewEmptyBorder(Insets{Bottom: 2 * StdHSpacing, Right: StdHSpacing}))
		iconLabel.SetLayoutData(&FlexLayoutData{})
		content.AddChild(iconLabel)
	}
	content.SetLayout(&FlexLayout{
		Columns:  columns,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	p := msgPanel.AsPanel()
	if b := p.Border(); b != nil {
		p.SetBorder(NewCompoundBorder(NewEmptyBorder(Insets{Bottom: 2 * StdHSpacing}), b))
	} else {
		p.SetBorder(NewEmptyBorder(Insets{Bottom: 2 * StdHSpacing}))
	}
	p.SetLayoutData(&FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: align.Fill,
		VAlign: align.Fill,
		HGrab:  true,
		VGrab:  true,
	})
	content.AddChild(msgPanel)
	buttonPanel := NewPanel()
	buttonPanel.SetLayout(&FlexLayout{
		Columns:      len(buttonInfo) + 1,
		HSpacing:     StdHSpacing * 2,
		VSpacing:     StdVSpacing,
		EqualColumns: true,
	})
	buttonPanel.AddChild(NewPanel())
	for _, bi := range buttonInfo {
		b := bi.NewButton(d)
		d.buttons[bi.ResponseCode] = &buttonData{
			info:   bi,
			button: b,
		}
		buttonPanel.AddChild(b)
	}
	buttonPanel.SetLayoutData(&FlexLayoutData{
		HSpan:  columns,
		VSpan:  1,
		HAlign: align.End,
		VAlign: align.Middle,
	})
	content.AddChild(buttonPanel)
	originalKeyDownCallback := content.KeyDownCallback
	content.KeyDownCallback = func(keyCode KeyCode, mod Modifiers, repeat bool) bool {
		if originalKeyDownCallback == nil || !originalKeyDownCallback(keyCode, mod, repeat) {
			if mod&NonStickyModifiers == 0 {
				for _, one := range d.buttons {
					for _, kc := range one.info.KeyCodes {
						if kc == keyCode {
							if one.button.Enabled() {
								one.button.Click()
							}
							return true
						}
					}
				}
			}
			return false
		}
		return true
	}
	d.wnd.Pack()
	wndFrame := d.wnd.FrameRect()
	frame.Y += (frame.Height - wndFrame.Height) / 3
	frame.Height = wndFrame.Height
	frame.X += (frame.Width - wndFrame.Width) / 2
	frame.Width = wndFrame.Width
	d.wnd.SetFrameRect(frame.Align())
	return d, nil
}

// Window returns the underlying window.
func (d *Dialog) Window() *Window {
	return d.wnd
}

// Button returns the button mapped to the given response code.
func (d *Dialog) Button(responseCode int) *Button {
	if bd, ok := d.buttons[responseCode]; ok {
		return bd.button
	}
	return nil
}

// RunModal displays and brings this dialog to the front, the runs a modal event loop until StopModal is called.
// Disposes the dialog before it returns.
func (d *Dialog) RunModal() int {
	return d.wnd.RunModal()
}

// StopModal stops the current modal event loop and propagates the provided code as the result to RunModal().
func (d *Dialog) StopModal(code int) {
	d.wnd.StopModal(code)
}

// NewMessagePanel creates a new panel containing the given primary and detail messages. Embedded line feeds are OK.
func NewMessagePanel(primary, detail string) *Panel {
	panel := NewPanel()
	panel.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	breakTextIntoLabels(panel, primary, EmphasizedSystemFont, false)
	breakTextIntoLabels(panel, detail, SystemFont, true)
	panel.SetLayoutData(&FlexLayoutData{
		MinSize: Size{Width: 200},
		HSpan:   1,
		VSpan:   1,
		VAlign:  align.Middle,
	})
	return panel
}

func breakTextIntoLabels(panel *Panel, text string, font Font, addSpaceAbove bool) {
	if text != "" {
		returns := 0
		for {
			if i := strings.Index(text, "\n"); i != -1 {
				if i == 0 {
					returns++
					text = text[1:]
				} else {
					part := text[:i]
					l := NewLabel()
					l.Font = font
					l.SetTitle(part)
					if returns > 1 || addSpaceAbove {
						addSpaceAbove = false
						l.SetBorder(NewEmptyBorder(Insets{Top: StdHSpacing}))
					}
					panel.AddChild(l)
					text = text[i+1:]
					returns = 1
				}
			} else {
				if text != "" {
					l := NewLabel()
					l.Font = font
					l.SetTitle(text)
					if returns > 1 || addSpaceAbove {
						l.SetBorder(NewEmptyBorder(Insets{Top: StdHSpacing}))
					}
					panel.AddChild(l)
				}
				break
			}
		}
	}
}

// ErrorDialogWithError displays a standard error dialog with the specified primary message and extracts the message
// from the error for its detail. The full error will be logged via errs.Log(). Embedded line feeds are OK.
func ErrorDialogWithError(primary string, detail error) {
	var msg string
	var err errs.StackError
	if errors.As(detail, &err) {
		errs.Log(detail)
		msg = err.Message()
	} else {
		msg = detail.Error()
	}
	ErrorDialogWithMessage(primary, msg)
}

// ErrorDialogWithMessage displays a standard error dialog with the specified primary and detail messages. Embedded line
// feeds are OK.
func ErrorDialogWithMessage(primary, detail string) {
	ErrorDialogWithPanel(NewMessagePanel(primary, detail))
}

// ErrorDialogWithPanel displays a standard error dialog with the specified panel.
func ErrorDialogWithPanel(msgPanel Paneler) {
	if dialog, err := NewDialog(DefaultDialogTheme.ErrorIcon, DefaultDialogTheme.ErrorIconInk, msgPanel,
		[]*DialogButtonInfo{NewOKButtonInfo()}); err != nil {
		errs.Log(err)
	} else {
		dialog.RunModal()
	}
}

// WarningDialogWithMessage displays a standard warning dialog with the specified primary and detail messages. Embedded
// line feeds are OK.
func WarningDialogWithMessage(primary, detail string) {
	WarningDialogWithPanel(NewMessagePanel(primary, detail))
}

// WarningDialogWithPanel displays a standard error dialog with the specified panel.
func WarningDialogWithPanel(msgPanel Paneler) {
	if dialog, err := NewDialog(DefaultDialogTheme.WarningIcon, DefaultDialogTheme.WarningIconInk, msgPanel,
		[]*DialogButtonInfo{NewOKButtonInfo()}); err != nil {
		errs.Log(err)
	} else {
		dialog.RunModal()
	}
}

// QuestionDialog displays a standard question dialog with the specified primary and detail messages. Embedded line
// feeds are OK. This function returns ids.ModalResponseOK if the OK button was pressed and ids.ModalResponseCancel if
// the Cancel button was pressed.
func QuestionDialog(primary, detail string) int {
	return QuestionDialogWithPanel(NewMessagePanel(primary, detail))
}

// QuestionDialogWithPanel displays a standard question dialog with the specified panel. This function returns
// ids.ModalResponseOK if the OK button was pressed and ids.ModalResponseCancel if the Cancel button was pressed.
func QuestionDialogWithPanel(msgPanel Paneler) int {
	if dialog, err := NewDialog(DefaultDialogTheme.QuestionIcon, DefaultDialogTheme.QuestionIconInk, msgPanel,
		[]*DialogButtonInfo{NewCancelButtonInfo(), NewOKButtonInfo()}); err != nil {
		errs.Log(err)
	} else {
		return dialog.RunModal()
	}
	return ModalResponseCancel
}

// YesNoDialog displays a standard question dialog with the specified primary and detail messages. Embedded line
// feeds are OK. This function returns ids.ModalResponseOK if the Yes button was pressed and ids.ModalResponseDiscard if
// the No button was pressed.
func YesNoDialog(primary, detail string) int {
	return YesNoDialogWithPanel(NewMessagePanel(primary, detail))
}

// YesNoDialogWithPanel displays a standard question dialog with the specified panel. Embedded line feeds are OK.
// This function returns ids.ModalResponseOK if the Yes button was pressed and ids.ModalResponseDiscard if the No button
// was pressed.
func YesNoDialogWithPanel(msgPanel Paneler) int {
	if dialog, err := NewDialog(DefaultDialogTheme.QuestionIcon,
		DefaultDialogTheme.QuestionIconInk, msgPanel,
		[]*DialogButtonInfo{NewNoButtonInfo(), NewYesButtonInfo()}); err != nil {
		errs.Log(err)
	} else {
		return dialog.RunModal()
	}
	return ModalResponseDiscard
}

// YesNoCancelDialog displays a standard question dialog with the specified primary and detail messages. Embedded line
// feeds are OK. This function returns ids.ModalResponseOK if the Yes button was pressed, ids.ModalResponseDiscard if
// the No button was pressed, and ids.ModalResponseCancel if the Cancel button was pressed.
func YesNoCancelDialog(primary, detail string) int {
	return YesNoCancelDialogWithPanel(NewMessagePanel(primary, detail))
}

// YesNoCancelDialogWithPanel displays a standard question dialog with the specified panel. Embedded line feeds are OK.
// This function returns ids.ModalResponseOK if the Yes button was pressed, ids.ModalResponseDiscard if the No button
// was pressed, and ids.ModalResponseCancel if the Cancel button was pressed.
func YesNoCancelDialogWithPanel(msgPanel Paneler) int {
	if dialog, err := NewDialog(DefaultDialogTheme.QuestionIcon, DefaultDialogTheme.QuestionIconInk, msgPanel,
		[]*DialogButtonInfo{NewCancelButtonInfo(), NewNoButtonInfo(), NewYesButtonInfo()}); err != nil {
		errs.Log(err)
	} else {
		return dialog.RunModal()
	}
	return ModalResponseCancel
}
