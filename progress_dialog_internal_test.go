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
	"errors"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
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
	report(0.25) // From this goroutine, as a download would
	screen.Do(func() {
		defer d.Window().Dispose()
		c.Equal(float32(50), d.ProgressBar().Current())
	})
}

// TestRunProgressDialogReturnsTheWorkResult checks that the dialog stays up until both the work and its follow-up have
// run, and that what the work returned is what the caller gets back.
func TestRunProgressDialogReturnsTheWorkResult(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 600})
	screen.Do(func() {
		d, err := NewProgressDialog("Updating…", "Downloading…", 0, nil)
		c.NoError(err)
		var order []string
		got := RunProgressDialog(d, func() string {
			order = append(order, "work")
			return "done"
		}, func() { order = append(order, "after") })
		// Both ran on the one background goroutine, which finished before the modal loop was released.
		c.Equal([]string{"work", "after"}, order)
		c.Equal("done", got)
		c.False(d.Window().IsValid(), "the dialog is disposed of once its modal loop ends")
	})
}
