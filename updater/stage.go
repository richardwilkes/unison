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
	"os"
	"path/filepath"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// phase is the step a staging operation has reached, so the caller can say what is happening.
type phase byte

const (
	// phaseDownloading means the release is being fetched.
	phaseDownloading phase = iota
	// phaseExtracting means the downloaded archive is being unpacked.
	phaseExtracting
	// phaseVerifying means the unpacked application is being checked.
	phaseVerifying
	// phasePreparing means the update is being made ready to apply.
	phasePreparing
)

// staged is a verified replacement sitting beside the installation, ready to be swapped in.
type staged struct {
	WorkDir string
	Payload string
	Helper  string
	LogPath string
}

// stage downloads the release, unpacks it, verifies it, and proves it runs -- all without touching the installation.
// Everything it creates lives in a single staging directory, and an abort at any point removes that directory and
// leaves the installation exactly as it was.
//
// The staging directory is created beside the installation rather than in the system temporary directory, which is what
// makes the swap a rename within one filesystem -- atomic and instant -- instead of a copy across two.
//
// progress reports the fraction of the download completed, from 0 to 1. phaseFn reports which step is running. Both are
// called from this goroutine, may be nil, and must not block.
func (u *Updater) stage(ctx context.Context, p *plan, progress func(fraction float64), phaseFn func(phase)) (s *staged, err error) {
	report := func(ph phase) {
		if phaseFn != nil {
			phaseFn(ph)
		}
	}

	// Clear away anything left by an attempt that did not clean up after itself; see abandonedStagingAge.
	u.sweepStrays([]sweepDir{{dir: p.Target.Parent, targetBase: filepath.Base(p.Target.Path)}}, abandonedStagingAge)

	workDir, err := os.MkdirTemp(p.Target.Parent, u.workDirPrefix())
	if err != nil {
		return nil, errs.NewWithCause("unable to prepare the update", err)
	}
	// The archive goes to the system temporary directory rather than the staging directory: it is thrown away as soon
	// as it has been unpacked, and keeping it out of the installation's directory means a failure part-way through
	// leaves nothing large sitting in the user's Applications folder.
	dlDir, err := os.MkdirTemp("", u.downloadDirPrefix())
	if err != nil {
		removeIgnoringErrors(workDir)
		return nil, errs.NewWithCause("unable to prepare the update", err)
	}
	defer removeIgnoringErrors(dlDir)
	defer func() {
		if err != nil {
			removeIgnoringErrors(workDir)
		}
	}()

	report(phaseDownloading)
	archivePath := filepath.Join(dlDir, p.Asset.Name)
	var reportProgress func(int64)
	if progress != nil && p.Asset.Size > 0 {
		reportProgress = func(read int64) { progress(float64(read) / float64(p.Asset.Size)) }
	}
	if err = download(ctx, u.cfg.HTTPClient, p.Asset, u.cfg.GitHubToken, archivePath, reportProgress); err != nil {
		return nil, err
	}

	report(phaseExtracting)
	payload := p.Target.payloadPath(workDir)
	if err = u.stagePayload(ctx, archivePath, payload); err != nil {
		return nil, err
	}

	report(phaseVerifying)
	if err = verifyPayload(ctx, payload, p.Target.Path); err != nil {
		return nil, err
	}
	if err = u.verifyRuns(ctx, p.Target.execWithin(payload), p.ToVersion); err != nil {
		return nil, err
	}

	// The helper is a copy of the *running* build rather than the downloaded one because the command line between the
	// two is a contract: having one compilation write it and read it means the compiler checks it. Handing the job to
	// the new build would make it a permanent cross-version interface, where a rename years from now silently breaks
	// every older version's ability to update.
	//
	// It is a copy rather than the installed file so that, once the application has exited, nothing under the
	// installation directory is open by any of its processes. That is what lets the swap and the cleanup be
	// unconditional: Windows will not delete a running image, and would otherwise leave the displaced copy behind for a
	// week.
	report(phasePreparing)
	helper, err := stageHelper(ctx, &p.Target, workDir)
	if err != nil {
		return nil, err
	}

	return &staged{
		WorkDir: workDir,
		Payload: payload,
		Helper:  helper,
		LogPath: filepath.Join(workDir, logName),
	}, nil
}

// discard throws away a prepared update. Nothing in the installation has been touched at this point, so this only has
// to remove the staging directory.
func discard(s *staged) {
	if s != nil {
		removeIgnoringErrors(s.WorkDir)
	}
}

// clearState removes the record of an update prepared and then abandoned before the helper ever ran. Leaving it would
// make the next launch report an update that is no longer waiting.
func clearState(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return errs.Wrap(err)
	}
	return nil
}

// newState builds the record handed to the helper and, after the swap, to the newly installed application.
func (u *Updater) newState(p *plan, s *staged) *state {
	return &state{
		Schema:      stateSchema,
		Status:      statusStaged,
		FromVersion: p.FromVersion,
		ToVersion:   p.ToVersion,
		Target:      p.Target.Path,
		Exec:        p.Target.Exec,
		Payload:     s.Payload,
		Backup:      p.Target.freeBackupPath(),
		WorkDir:     s.WorkDir,
		Helper:      s.Helper,
		LogPath:     s.LogPath,
		HandoffPort: u.cfg.HandoffPort,
		ParentPID:   os.Getpid(),
		ParentStart: processStartID(os.Getpid()),
		Bundle:      p.Target.Kind == kindBundle,
	}
}
