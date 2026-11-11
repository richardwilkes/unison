// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison/enums/align"
)

// progressReportSteps is how finely a FractionReporter divides the progress it is given. Only a change of step is
// posted to the UI thread, so that a fast operation doesn't flood the task queue with updates nobody could see.
const progressReportSteps = 1000

// ProgressDialog is a small floating window that reports on a long-running operation with a message, a progress bar
// and, optionally, a Cancel button. Other than the function FractionReporter returns, it may only be used on the UI
// thread.
type ProgressDialog struct {
	wnd          *Window
	label        *Label
	bar          *ProgressBar
	cancelButton *Button
	canceled     bool
}

// NewProgressDialog creates a progress dialog. A maximum of zero gives an indeterminate progress bar. Unless cancel is
// nil, the dialog has a Cancel button which, when pressed, disables itself, changes the message to say the operation is
// being canceled and calls cancel. To show the dialog, call RunModal or pass it to RunProgressDialog.
func NewProgressDialog(title, message string, maximum float32, cancel func()) (*ProgressDialog, error) {
	wnd, err := NewWindow(title, FloatingWindowOption(), NotResizableWindowOption(), UndecoratedWindowOption(),
		TransientWindowOption())
	if err != nil {
		return nil, errs.NewWithCause("unable to create progress dialog", err)
	}
	d := &ProgressDialog{
		wnd:   wnd,
		label: NewLabel(),
		bar:   NewProgressBar(maximum),
	}
	content := NewPanel()
	content.SetBorder(NewCompoundBorder(NewLineBorder(ThemeSurfaceEdge, geom.Size{}, geom.NewUniformInsets(1), false),
		NewEmptyBorder(geom.NewUniformInsets(2*StdHSpacing))))
	content.SetLayout(&FlexLayout{
		Columns:  1,
		VSpacing: StdVSpacing,
	})
	d.label.SetTitle(message)
	content.AddChild(d.label)
	d.bar.SetLayoutData(&FlexLayoutData{
		MinSize: geom.Size{Width: 500},
		HAlign:  align.Fill,
		HGrab:   true,
	})
	content.AddChild(d.bar)
	if cancel != nil {
		d.cancelButton = NewButton()
		d.cancelButton.SetTitle(i18n.Text("Cancel"))
		d.cancelButton.SetLayoutData(&FlexLayoutData{HAlign: align.End})
		d.cancelButton.SetBorder(NewEmptyBorder(geom.Insets{Top: 2 * StdHSpacing}))
		d.cancelButton.ClickCallback = func() {
			d.cancelButton.SetEnabled(false)
			d.label.SetTitle(i18n.Text("Canceling…"))
			d.canceled = true
			cancel()
		}
		content.AddChild(d.cancelButton)
	}
	wnd.SetContent(content)
	wnd.Pack()
	wnd.MoveToModalCenter(ActiveWindow())
	return d, nil
}

// Window returns the underlying window.
func (d *ProgressDialog) Window() *Window {
	return d.wnd
}

// ProgressBar returns the dialog's progress bar.
func (d *ProgressDialog) ProgressBar() *ProgressBar {
	return d.bar
}

// Message returns the message being shown.
func (d *ProgressDialog) Message() string {
	return d.label.String()
}

// SetMessage changes the message. Once Cancel has been pressed this does nothing, since the message then says the
// operation is being canceled and should go on saying so until the dialog closes.
func (d *ProgressDialog) SetMessage(message string) {
	if !d.canceled {
		d.label.SetTitle(message)
	}
}

// Canceled returns true once the Cancel button has been pressed.
func (d *ProgressDialog) Canceled() bool {
	return d.canceled
}

// FractionReporter returns a function that moves the progress bar to the given fraction, from 0 to 1, of its maximum.
// Unlike the dialog's methods, that function may be called from any goroutine, though only one at a time: it posts the
// change to the UI thread, and only when the fraction has moved far enough to be visible.
func (d *ProgressDialog) FractionReporter() func(fraction float64) {
	last := -1
	return func(fraction float64) {
		step := int(fraction * progressReportSteps)
		if step == last {
			return
		}
		last = step
		InvokeTask(func() { d.bar.SetCurrent(float32(fraction) * d.bar.Maximum()) })
	}
}

// RunModal displays and brings this dialog to the front, then runs a modal event loop until StopModal is called.
// Disposes the dialog before it returns.
func (d *ProgressDialog) RunModal() int {
	return d.wnd.RunModal()
}

// StopModal stops the current modal event loop and propagates the provided code as the result to RunModal().
func (d *ProgressDialog) StopModal(code int) {
	d.wnd.StopModal(code)
}

// RunProgressDialog shows d modally while work runs on another goroutine, and returns what work returned once the
// dialog has closed. If afterWork is not nil, it runs on that goroutine once work has returned and before the dialog is
// closed, for any follow-up that would otherwise hold up the UI thread. Must be called on the UI thread.
func RunProgressDialog[T any](d *ProgressDialog, work func() T, afterWork func()) T {
	// The channel must be buffered, since nothing can receive from it until the modal loop has been stopped.
	resultChan := make(chan T, 1)
	runInBackground(resultChan, work, func() {
		if afterWork != nil {
			afterWork()
		}
		// StopModal changes modal state the UI thread is spinning on inside RunModal, so it has to run there.
		InvokeTask(func() { d.StopModal(ModalResponseOK) })
	})
	d.RunModal()
	return <-resultChan
}

// runInBackground runs work on a goroutine, sends its result to resultChan and only then calls finish. Sending before
// finishing is what makes the hand-off safe when finish releases a thread that then reads the result: handing it over
// through shared variables instead would allow a failed operation to be observed as a success.
func runInBackground[T any](resultChan chan<- T, work func() T, finish func()) {
	go func() {
		var result T
		defer func() {
			resultChan <- result
			finish()
		}()
		result = work()
	}()
}
