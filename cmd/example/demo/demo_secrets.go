// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/secrets"
)

const (
	notFoundPrefix    = "Not found: "
	unavailablePrefix = "Unavailable: "
)

var secretsWindow *unison.Window

type secretsForm struct {
	serviceField *unison.Field
	accountField *unison.Field
	valueField   *unison.Field
	status       *unison.Label
	buttons      []*unison.Button
}

// NewDemoSecretsWindow creates and displays our demo secrets window, which stores, fetches, deletes and lists secrets
// in the operating system's credential store. Only one such window is permitted to exist at a time, so if one was
// created earlier and hasn't been closed, it is brought to the front and returned instead.
func NewDemoSecretsWindow() (*unison.Window, error) {
	if secretsWindow != nil {
		secretsWindow.ToFront()
		return secretsWindow, nil
	}

	wnd, err := unison.NewWindow("Secrets", unison.NotResizableWindowOption())
	if err != nil {
		return nil, err
	}
	trackSecretsWindow(wnd)

	installDefaultMenus(wnd)

	content := wnd.Content()
	content.SetLayout(&unison.FlexLayout{
		Columns:  2,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	content.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(20)))

	var form secretsForm
	form.serviceField = addSecretsField(content, "Service")
	form.serviceField.SetText("com.trollworks.unison.example")
	form.accountField = addSecretsField(content, "Account")
	form.valueField = addSecretsField(content, "Secret")
	form.valueField.ObscurementRune = '●'

	buttonPanel := unison.NewPanel()
	buttonPanel.SetLayout(&unison.FlexLayout{
		Columns:      4,
		HSpacing:     unison.StdHSpacing,
		EqualColumns: true,
	})
	buttonPanel.SetBorder(unison.NewEmptyBorder(geom.Insets{Top: 10}))
	buttonPanel.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  2,
		HAlign: align.Middle,
	})
	form.addButton(buttonPanel, "Set", storeSecret)
	form.addButton(buttonPanel, "Get", fetchSecret)
	form.addButton(buttonPanel, "Delete", deleteSecret)
	form.addButton(buttonPanel, "Accounts", listAccounts)
	content.AddChild(buttonPanel)

	form.status = unison.NewLabel()
	form.status.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  2,
		HAlign: align.Fill,
		HGrab:  true,
	})
	content.AddChild(form.status)

	wnd.PackWithDefaultInitialLocation()
	wnd.ToFront()

	return wnd, nil
}

// trackSecretsWindow records wnd as the shared secrets window, clearing the record when the window closes so that the
// next call to NewDemoSecretsWindow creates a new one.
func trackSecretsWindow(wnd *unison.Window) {
	secretsWindow = wnd
	wnd.WillCloseCallback = func() { secretsWindow = nil }
}

func addSecretsField(panel *unison.Panel, title string) *unison.Field {
	label := unison.NewLabel()
	label.SetTitle(title)
	label.HAlign = align.End
	label.SetLayoutData(&unison.FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	panel.AddChild(label)
	field := unison.NewField()
	field.SetLayoutData(&unison.FlexLayoutData{
		SizeHint: geom.NewSize(300, 0),
		HAlign:   align.Fill,
		VAlign:   align.Middle,
		HGrab:    true,
	})
	panel.AddChild(field)
	return field
}

// addButton adds a button that runs op with the field contents on another goroutine, since the platform store may
// block on a prompt, and then shows the text op returns as the status.
func (f *secretsForm) addButton(panel *unison.Panel, title string, op func(service, account, value string) string) {
	button := unison.NewButton()
	button.SetTitle(title)
	button.SetLayoutData(&unison.FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	button.ClickCallback = func() {
		service := f.serviceField.Text()
		account := f.accountField.Text()
		value := f.valueField.Text()
		f.setButtonsEnabled(false)
		go func() {
			text := op(service, account, value)
			unison.InvokeTask(func() {
				f.setButtonsEnabled(true)
				f.status.SetTitle(text)
				// The window can't grow to fit a long status, so the tooltip shows whatever gets clipped.
				f.status.Tooltip = unison.NewTooltipWithText(text)
				f.status.MarkForRedraw()
			})
		}()
	}
	panel.AddChild(button)
	f.buttons = append(f.buttons, button)
}

func (f *secretsForm) setButtonsEnabled(enabled bool) {
	for _, button := range f.buttons {
		button.SetEnabled(enabled)
	}
}

func storeSecret(service, account, value string) string {
	data := []byte(value)
	if err := secrets.Set(service, account, data); err != nil {
		return secretsErrorText(err)
	}
	return "Stored " + byteCount(len(data))
}

func fetchSecret(service, account, _ string) string {
	data, err := secrets.Get(service, account)
	if err != nil {
		return secretsErrorText(err)
	}
	return fmt.Sprintf("%q (%s)", data, byteCount(len(data)))
}

func deleteSecret(service, account, _ string) string {
	if err := secrets.Delete(service, account); err != nil {
		return secretsErrorText(err)
	}
	return "Deleted"
}

func listAccounts(service, _, _ string) string {
	accounts, err := secrets.Accounts(service)
	if err != nil {
		return secretsErrorText(err)
	}
	if len(accounts) == 0 {
		return "No accounts"
	}
	return strings.Join(accounts, ", ")
}

// secretsErrorText returns the message of err without the stack trace an errs error appends, prefixed with the kind
// of failure when err matches one of the secrets package's sentinels.
func secretsErrorText(err error) string {
	msg := err.Error()
	if stackErr, ok := errors.AsType[errs.StackError](err); ok {
		msg = stackErr.Message()
	}
	switch {
	case errors.Is(err, secrets.ErrNotFound):
		return notFoundPrefix + msg
	case errors.Is(err, secrets.ErrUnavailable):
		return unavailablePrefix + msg
	default:
		return msg
	}
}

func byteCount(n int) string {
	if n == 1 {
		return "1 byte"
	}
	return fmt.Sprintf("%d bytes", n)
}
