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
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// backgroundHandshakeTimeout is a generous bound on the wait for the background goroutine, so that a broken hand-off
// fails the test rather than hanging it.
const backgroundHandshakeTimeout = 10 * time.Second

// TestRunInBackgroundDeliversResultBeforeFinishing checks the invariant the hand-off rests on: finish stops the modal
// loop and the UI thread reads the result as soon as that loop exits, so the result, whether a failure or a success
// made of more than an error, must already be in the channel by the time finish runs. The receive is non-blocking on
// purpose -- an empty channel means the UI thread could have been released with nothing to read.
func TestRunInBackgroundDeliversResultBeforeFinishing(t *testing.T) {
	type outcome struct {
		err   error
		value string
	}
	for _, one := range []struct {
		name string
		want outcome
	}{
		{name: "failure", want: outcome{err: errors.New("download failed")}},
		{name: "success", want: outcome{value: "downloaded"}},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			resultChan := make(chan outcome, 1)
			var (
				delivered bool
				got       outcome
			)
			done := make(chan struct{})
			runInBackground(resultChan, func() outcome { return one.want }, func() {
				select {
				case got = <-resultChan:
					delivered = true
				default:
				}
				close(done)
			})
			select {
			case <-done:
			case <-time.After(backgroundHandshakeTimeout):
				t.Fatal("timed out waiting for the background work to finish")
			}
			c.True(delivered, "the result must be in the channel before the modal loop can be stopped")
			c.Equal(one.want, got)
		})
	}
}

func TestProgressDialogCancelButton(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 600})
	screen.Do(func() {
		d, err := NewProgressDialog("Downloading…", "Downloading the data…", 0, nil)
		c.NoError(err)
		defer d.Window().Dispose()
		c.Nil(d.cancelButton, "no Cancel button without a cancel function")
		c.Equal("Downloading the data…", d.Message())
		d.SetMessage("Unpacking the data…")
		c.Equal("Unpacking the data…", d.Message())
	})
	screen.Do(func() {
		canceled := false
		d, err := NewProgressDialog("Updating…", "Downloading the library…", 0, func() { canceled = true })
		c.NoError(err)
		defer d.Window().Dispose()
		c.NotNil(d.cancelButton)
		c.True(d.cancelButton.Enabled())
		c.False(d.Canceled())
		d.cancelButton.Click()
		c.True(canceled, "pressing Cancel must call the cancel function")
		c.True(d.Canceled())
		c.False(d.cancelButton.Enabled(), "Cancel must disable itself so it cannot be pressed twice")
		c.Equal("Canceling…", d.Message())
		d.SetMessage("Installing the library…")
		c.Equal("Canceling…", d.Message(), "a later phase must not replace the notice that the operation is stopping")
	})
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestProgressDialogSetMessageRedrawsTheWholeMessage checks that a new message is painted at its full width without
// waiting for something else to redraw the window, which nothing does once a determinate bar has stopped moving.
func TestProgressDialogSetMessageRedrawsTheWholeMessage(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 600})
	var (
		d *ProgressDialog
		w *Window
	)
	screen.Do(func() {
		var err error
		d, err = NewProgressDialog("Updating…", "Verifying…", 1, nil)
		c.NoError(err)
		w = d.Window()
		w.Show()
	})
	if errList := screen.Errors(); len(errList) != 0 || d == nil {
		t.Fatalf("unable to create the dialog: %v", errList)
	}
	before := screen.CaptureWindow(w)
	c.NotNil(before, "the dialog must have been drawn once shown")
	var labelWidth, wanted float32
	screen.Do(func() {
		d.SetMessage("Preparing to install the update…")
	})
	after := screen.CaptureWindow(w)
	screen.Do(func() {
		defer w.Dispose()
		labelWidth = d.label.FrameRect().Width
		_, pref, _ := d.label.Sizes(geom.Size{})
		wanted = pref.Width
	})
	c.True(before != nil && after != nil && !bytes.Equal(before.Pix, after.Pix), "the new message must be painted")
	c.Equal(wanted, labelWidth, "the label must be wide enough for the new message")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestProgressDialogRefusesCloseRequests checks that a request to close the dialog, such as Alt+F4, cannot take it down
// while the work runs, since RunProgressDialog would then wait for that work on the UI thread, and that the request
// acts as Cancel when the dialog has that button.
func TestProgressDialogRefusesCloseRequests(t *testing.T) {
	for _, one := range []struct {
		name       string
		withCancel bool
	}{
		{name: "without Cancel"},
		{name: "with Cancel", withCancel: true},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 600})
			var (
				d            *ProgressDialog
				cancelCalled bool
			)
			screen.Do(func() {
				var cancel func()
				if one.withCancel {
					cancel = func() { cancelCalled = true }
				}
				var err error
				d, err = NewProgressDialog("Updating…", "Downloading…", 1, cancel)
				c.NoError(err)
			})
			if errList := screen.Errors(); len(errList) != 0 || d == nil {
				t.Fatalf("unable to create the dialog: %v", errList)
			}
			release := make(chan struct{})
			finished := make(chan struct{})
			var result string
			screen.Post(func() {
				defer close(finished)
				result = RunProgressDialog(d, func() string {
					<-release
					return "done"
				}, nil)
			})
			type state struct {
				message      string
				valid        bool
				cancelCalled bool
				canceled     bool
			}
			var got state
			observed := make(chan struct{})
			// Posted rather than run with Do, since a close request that ended the modal loop would leave the UI thread
			// waiting for the work, and Do waiting for the UI thread.
			screen.Post(func() {
				defer close(observed)
				d.Window().requestClose()
				got = state{
					message:      d.Message(),
					valid:        d.Window().IsValid(),
					cancelCalled: cancelCalled,
					canceled:     d.Canceled(),
				}
			})
			timedOut := false
			select {
			case <-observed:
			case <-time.After(backgroundHandshakeTimeout):
				timedOut = true
			}
			close(release)
			if timedOut {
				t.Fatal("timed out waiting for the close request to be handled")
			}
			select {
			case <-finished:
			case <-time.After(backgroundHandshakeTimeout):
				t.Fatal("RunProgressDialog did not return once the work had finished")
			}
			c.Equal("done", result)
			c.True(got.valid, "a close request must leave the dialog up while the work runs")
			c.Equal(one.withCancel, got.cancelCalled, "a close request must act as Cancel when there is one")
			c.Equal(one.withCancel, got.canceled)
			if one.withCancel {
				c.Equal("Canceling…", got.message)
			} else {
				c.Equal("Downloading…", got.message)
			}
			c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
		})
	}
}

func TestProgressDialogFractionReporterMovesTheBar(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 600})
	var (
		d      *ProgressDialog
		report func(float64)
	)
	screen.Do(func() {
		var err error
		d, err = NewProgressDialog("Updating…", "Downloading…", 200, nil)
		c.NoError(err)
		report = d.FractionReporter()
	})
	if errList := screen.Errors(); len(errList) != 0 || d == nil {
		t.Fatalf("unable to create the dialog: %v", errList)
	}
	report(0.25) // From this goroutine, as a download would
	screen.Do(func() {
		defer d.Window().Dispose()
		c.Equal(float32(50), d.ProgressBar().Current())
	})
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestRunProgressDialogReturnsTheWorkResult checks that the dialog stays up until both the work and its follow-up have
// run, and that what the work returned is what the caller gets back.
func TestRunProgressDialogReturnsTheWorkResult(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 600})
	var (
		order []string
		got   string
		valid bool
	)
	finished := make(chan struct{})
	// Posted rather than run with Do, since RunProgressDialog runs a modal loop, and Do would never return if nothing
	// ended it.
	screen.Post(func() {
		defer close(finished)
		d, err := NewProgressDialog("Updating…", "Downloading…", 0, nil)
		c.NoError(err)
		got = RunProgressDialog(d, func() string {
			order = append(order, "work")
			return "done"
		}, func() { order = append(order, "after") })
		valid = d.Window().IsValid()
	})
	select {
	case <-finished:
	case <-time.After(backgroundHandshakeTimeout):
		t.Fatal("RunProgressDialog did not return once the work had finished")
	}
	// Both ran on the one background goroutine, which finished before the modal loop was released.
	c.Equal([]string{"work", "after"}, order)
	c.Equal("done", got)
	c.False(valid, "the dialog is disposed of once its modal loop ends")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
