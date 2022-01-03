// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
)

// Pre-defined modal response codes. Apps should start their codes at ModalResponseUserBase.
const (
	ModalResponseDiscard = iota - 1
	ModalResponseOK
	ModalResponseCancel
	ModalResponseUserBase = 100
)

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
func NewDialog(img *Image, msgPanel *Panel, buttonInfo []*DialogButtonInfo) (*Dialog, error) {
	d := &Dialog{buttons: make(map[int]*buttonData)}
	var frame geom32.Rect
	if focused := ActiveWindow(); focused != nil {
		frame = focused.FrameRect()
	} else {
		frame = PrimaryDisplay().Usable
	}
	d.wnd, d.err = NewWindow("", FloatingWindowOption())
	if d.err != nil {
		return nil, errs.NewWithCause("unable to create dialog", d.err)
	}
	content := d.wnd.Content()
	content.SetBorder(NewEmptyBorder(geom32.NewUniformInsets(16)))
	columns := 1
	if img != nil {
		columns++
		icon := NewLabel()
		icon.Drawable = img
		icon.SetBorder(NewEmptyBorder(geom32.Insets{Bottom: 16, Right: 8}))
		icon.SetLayoutData(&FlexLayoutData{
			HSpan: 1,
			VSpan: 1,
		})
		content.AddChild(icon)
	}
	content.SetLayout(&FlexLayout{
		Columns:  columns,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	if b := msgPanel.Border(); b != nil {
		msgPanel.SetBorder(NewCompoundBorder(NewEmptyBorder(geom32.Insets{Bottom: 16}), b))
	} else {
		msgPanel.SetBorder(NewEmptyBorder(geom32.Insets{Bottom: 16}))
	}
	msgPanel.SetLayoutData(&FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: FillAlignment,
		VAlign: FillAlignment,
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
		HAlign: EndAlignment,
		VAlign: MiddleAlignment,
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
	frame.Align()
	d.wnd.SetFrameRect(frame)
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

// NewMessagePanel creates a new panel containing the given primary and detail
// messages. Embedded line feeds are OK.
func NewMessagePanel(primary, detail string) *Panel {
	panel := NewPanel()
	panel.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	breakTextIntoLabels(panel, primary, EmphasizedSystemFont)
	breakTextIntoLabels(panel, detail, SystemFont)
	panel.SetLayoutData(&FlexLayoutData{
		MinSize: geom32.Size{Width: 200},
		HSpan:   1,
		VSpan:   1,
		VAlign:  MiddleAlignment,
	})
	return panel
}

func breakTextIntoLabels(panel *Panel, text string, font FontProvider) {
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
					l.Text = part
					l.Font = font
					if returns > 1 {
						l.SetBorder(NewEmptyBorder(geom32.Insets{Top: 8}))
					}
					panel.AddChild(l)
					text = text[i+1:]
					returns = 1
				}
			} else {
				if text != "" {
					l := NewLabel()
					l.Text = text
					l.Font = font
					if returns > 1 {
						l.SetBorder(NewEmptyBorder(geom32.Insets{Top: 8}))
					}
					panel.AddChild(l)
				}
				break
			}
		}
	}
}

// ErrorDialogWithError displays a standard error dialog with the specified
// primary message and extracts the message from the error for its detail.
// The full error will be logged via jot.Error(). Embedded line feeds are OK.
func ErrorDialogWithError(primary string, detail error) {
	var msg string
	var err errs.StackError
	if errors.As(detail, &err) {
		jot.Error(detail)
		msg = err.Message()
	} else {
		msg = detail.Error()
	}
	ErrorDialogWithMessage(primary, msg)
}

// ErrorDialogWithMessage displays a standard error dialog with the specified
// primary and detail messages. Embedded line feeds are OK.
func ErrorDialogWithMessage(primary, detail string) {
	ErrorDialogWithPanel(NewMessagePanel(primary, detail))
}

// ErrorDialogWithPanel displays a standard error dialog with the specified
// panel.
func ErrorDialogWithPanel(msgPanel *Panel) {
	if dialog, err := NewDialog(ErrorImage(), msgPanel, []*DialogButtonInfo{NewOKButtonInfo()}); err != nil {
		jot.Error(err)
	} else {
		dialog.RunModal()
	}
}

// QuestionDialog displays a standard question dialog with the specified
// primary and detail messages. Embedded line feeds are OK. This function
// returns ids.ModalResponseOK if the OK button was pressed and
// ids.ModalResponseCancel if the Cancel button was pressed.
func QuestionDialog(primary, detail string) int {
	return QuestionDialogWithPanel(NewMessagePanel(primary, detail))
}

// QuestionDialogWithPanel displays a standard question dialog with the
// specified panel. This function returns ids.ModalResponseOK if the OK button
// was pressed and ids.ModalResponseCancel if the Cancel button was pressed.
func QuestionDialogWithPanel(msgPanel *Panel) int {
	if dialog, err := NewDialog(QuestionImage(), msgPanel, []*DialogButtonInfo{NewCancelButtonInfo(), NewOKButtonInfo()}); err != nil {
		jot.Error(err)
	} else {
		return dialog.RunModal()
	}
	return ModalResponseCancel
}
