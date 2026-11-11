// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package updater

import (
	"context"
	"sync"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xstrings"
	"github.com/richardwilkes/unison"
)

// stageTimeout bounds the whole prepare step. It is generous because the download can be large and some connections
// are slow; the Cancel button, not this, is how an impatient user stops it. A variable only so tests can shorten it.
var stageTimeout = 30 * time.Minute

// pendingUpdate holds an update that has been prepared and is waiting for the application to quit.
type pendingUpdate struct {
	state     *state
	statePath string
	lock      sync.Mutex
	applying  bool
}

// applyPending starts the helper that finishes a prepared update. It is registered to run at exit for as long as an
// update is waiting, since unison.AttemptQuit does not return when the quit succeeds, so that it runs on every exit
// that still executes Go code -- a quit request, the last window closing, or a termination signal. It is registered
// that way rather than as unison's quitting callback because there is only one of those, and it belongs to the
// application. It only starts a process, so it is safe on whichever goroutine the exit happens to run it.
func (u *Updater) applyPending() {
	u.pending.lock.Lock()
	s, statePath := u.pending.state, u.pending.statePath
	u.pending.state = nil
	u.pending.lock.Unlock()
	if s == nil {
		return
	}
	if err := u.spawnFn(s, statePath); err != nil {
		// There is no interface left to report this through; the application is already on its way out. The prepared
		// update is left behind, and the next launch reports it and clears it away.
		errs.Log(errs.NewWithCause("unable to start the update", err))
	}
}

// install prepares the update described by p and, once it is verified and ready, asks the application to quit so that
// it can be applied. The work happens in a staging directory, so canceling, failing or a refused quit leaves the
// installation untouched.
func (u *Updater) install(p *plan) {
	u.pending.lock.Lock()
	if u.pending.applying {
		u.pending.lock.Unlock()
		return
	}
	u.pending.applying = true
	u.pending.lock.Unlock()
	defer func() {
		u.pending.lock.Lock()
		u.pending.applying = false
		u.pending.lock.Unlock()
	}()

	prepared, ok := u.stageWithProgress(p)
	if !ok {
		return
	}

	s := u.newState(p, prepared)
	statePath := u.statePath()
	if err := s.save(statePath); err != nil {
		u.reportError(i18n.Text("Unable to prepare the update"), err)
		discard(prepared)
		return
	}

	u.pending.lock.Lock()
	u.pending.state = s
	u.pending.statePath = statePath
	u.pending.lock.Unlock()
	exitID := u.atExitFn(u.applyPending)

	// Quitting only returns if the quit was refused, which is what reaching the next line means. An open document with
	// unsaved changes is the usual reason.
	u.quitFn()

	u.cancelAtExitFn(exitID)
	u.pending.lock.Lock()
	u.pending.state = nil
	u.pending.lock.Unlock()
	discard(prepared)
	if err := clearState(statePath); err != nil {
		errs.Log(err)
	}
	u.warnFn(i18n.Text("The update was not installed"),
		xstrings.Wrap("", i18n.Text("%s could not quit, so the update was discarded. It will be offered again the next time you check for updates.",
			u.cfg.AppName), 100))
}

// stageWithProgress runs the download and verification behind a progress dialog, returning the prepared update. It
// reports its own failures, so the caller only needs to know whether to carry on.
func (u *Updater) stageWithProgress(p *plan) (*staged, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), stageTimeout)
	defer cancel()
	// The bar's maximum is 1, so that the fraction downloaded is also its position.
	dialog, err := unison.NewProgressDialog(i18n.Text("Updating…"), u.phaseTitle(phaseDownloading, p.ToVersion), 1,
		cancel)
	if err != nil {
		u.reportError(i18n.Text("Unable to prepare the update"), err)
		return nil, false
	}
	type stageResult struct {
		staged *staged
		err    error
	}
	progress := dialog.FractionReporter()
	result := unison.RunProgressDialog(dialog, func() stageResult {
		prepared, stageErr := u.stageFn(ctx, p, progress, func(ph phase) {
			unison.InvokeTask(func() { dialog.SetMessage(u.phaseTitle(ph, p.ToVersion)) })
		})
		return stageResult{staged: prepared, err: stageErr}
	}, nil)
	if dialog.Canceled() {
		// The user asked for this, so there is nothing to report. Staging may have finished anyway, if Cancel came
		// after the last step that watches ctx.
		discard(result.staged)
		return nil, false
	}
	if result.err != nil {
		u.reportError(i18n.Text("Unable to prepare the update"), result.err)
		return nil, false
	}
	return result.staged, true
}

// phaseTitle describes what the update is doing at the moment.
func (u *Updater) phaseTitle(ph phase, version string) string {
	switch ph {
	case phaseExtracting:
		return i18n.Text("Unpacking the update…")
	case phaseVerifying:
		return i18n.Text("Verifying the update…")
	case phasePreparing:
		return i18n.Text("Preparing to install…")
	default:
		return i18n.Text("Downloading %s %s…", u.cfg.AppName, u.formatVersion(version))
	}
}
