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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xio"
)

const (
	// staleAge is how old a leftover has to be before the startup sweep will remove it on sight. Anything younger might
	// belong to an update being applied right now by a helper this instance knows nothing about.
	staleAge = 7 * 24 * time.Hour
	// abandonedStagingAge is the shorter bound used for staging directories just before a new update is prepared. By
	// then any helper still working would have had to survive since before the application started, and the wait it
	// performs gives up long before this, so anything older is from an attempt that did not clean up after itself.
	// Without it, each failed attempt would leave a copy of the application in the user's applications folder for a
	// week.
	abandonedStagingAge = time.Hour
	// helperRunLimit bounds how long after being recorded the helper can still be at work without having recorded how
	// the update ended: its wait gives up after waitTimeout, and the swap that follows takes seconds.
	helperRunLimit = waitTimeout + time.Minute
)

// maxHelperLog bounds how much of the end of the helper's log is copied into the application log.
const maxHelperLog = 64 << 10

// removeAttempts and removeDelay bound the retries when clearing leftovers. On Windows the helper may still be exiting
// as the newly installed application starts, and its image cannot be deleted until it has.
const (
	removeAttempts = 5
	removeDelay    = 500 * time.Millisecond
)

// outcome reports what became of an update that was in progress when the application last exited. A nil outcome means
// there was nothing to report.
type outcome struct {
	Reason    reason
	Detail    string
	ToVersion string
	Applied   bool
}

// sweepDir is a directory an update may have left things in, along with the base name of the installation in it whose
// displaced copies may be removed.
type sweepDir struct {
	dir        string
	targetBase string
}

// reportAndSweep settles up after an update: it repairs an installation left half-swapped, reports what happened, and
// clears away what the helper deliberately left behind.
//
// The helper never deletes anything; it moves the old installation aside, puts the new one in place, and starts it.
// Removing the old copy happens here, which means it only happens once the replacement has demonstrably started, so a
// version that cannot run is still on disk next to one that can.
//
// This must run only in the instance that owns the installation, after any single-instance handoff has established that
// this process is it. Failures are logged rather than surfaced: the application has already started successfully by
// this point, and a leftover directory is not worth interrupting the user for.
func (u *Updater) reportAndSweep() *outcome {
	statePath := u.statePath()
	s, err := loadState(statePath)
	if err != nil {
		// An unreadable or unrecognized state file cannot be acted on, and keeping it would mean reporting the same
		// confusion on every launch from here on.
		errs.Log(errs.NewWithCause("discarding an unusable update state", err), "path", statePath)
		removeIgnoringErrors(statePath)
		u.sweepStrays(currentSweepDirs(), staleAge)
		return nil
	}
	if s == nil || (s.Status == statusStaged && stagingInUse(s, statePath)) {
		// A staged update is not abandoned while the copy that prepared it runs, nor while its helper does: that copy
		// may yet quit and have its helper apply the update, or have its quit refused and discard it.
		u.sweepStrays(currentSweepDirs(), staleAge)
		return nil
	}
	// The helper's log is in the staging directory, which settle removes. Only a success the helper recorded leaves
	// nothing in it worth keeping.
	helperLog := readTail(s.LogPath, maxHelperLog)
	out := u.settle(s)
	if helperLog != "" && (s.Status != statusApplied || out == nil || out.Reason != reasonNone) {
		slog.Warn("output from the update helper", "path", s.LogPath, "output", helperLog)
	}
	removeIgnoringErrors(statePath)
	dirs := currentSweepDirs()
	if s.Target != "" {
		dirs = append(dirs, sweepDir{dir: filepath.Dir(s.Target), targetBase: filepath.Base(s.Target)})
	}
	u.sweepStrays(dirs, staleAge)
	return out
}

// settle repairs and interprets a state left behind by a previous run, removing whatever is safe to remove.
func (u *Updater) settle(s *state) *outcome {
	// Repair first. Reaching here means the helper died between moving the installation aside and putting the
	// replacement in its place -- a window of microseconds its own rollback normally covers, so this is a net rather
	// than an expected path.
	if s.Backup != "" && s.Target != "" && !exists(s.Target) && exists(s.Backup) {
		if err := os.Rename(s.Backup, s.Target); err != nil {
			errs.Log(errs.NewWithCause("unable to restore the previous version", err), "backup", s.Backup,
				"target", s.Target)
		} else {
			slog.Warn("restored the previous version after an interrupted update", "target", s.Target)
		}
		removeWithRetry(s.WorkDir)
		return &outcome{Reason: reasonSwapFailed, Detail: s.Detail, ToVersion: s.ToVersion}
	}

	st := s.Status
	if st == statusStaged && s.ToVersion != "" && s.ToVersion == u.cfg.Version {
		// Still staged, yet the version it installs is the one running, so it is in place: most likely the helper
		// swapped it in but its save of that failed, or it was killed first.
		slog.Warn("the update is in place but was never recorded as applied", "version", s.ToVersion)
		st = statusApplied
	}
	switch st {
	case statusApplied:
		if s.ToVersion != "" && s.ToVersion != u.cfg.Version {
			// The swap claimed success, yet this is not the version it installed, so keep the previous version rather
			// than deleting the only other copy on the machine.
			errs.Log(errs.New("the installed version is not the one the update reported installing"),
				"expected", s.ToVersion, "actual", u.cfg.Version, "backup", s.Backup)
			removeWithRetry(s.WorkDir)
			return &outcome{Reason: reasonVersionMismatch, ToVersion: s.ToVersion}
		}
		slog.Info("update applied", "version", s.ToVersion, "from", s.FromVersion)
		removeWithRetry(s.Backup)
		removeWithRetry(s.WorkDir)
		if s.Reason != reasonNone {
			// Installed, but something afterwards went wrong -- the relaunch, most likely, which is why we are here
			// rather than having been started by it.
			return &outcome{Applied: true, Reason: s.Reason, Detail: s.Detail, ToVersion: s.ToVersion}
		}
		return &outcome{Applied: true, ToVersion: s.ToVersion}
	case statusFailed:
		errs.Log(errs.Newf("the update was not applied: %s", s.Reason), "detail", s.Detail)
		removeWithRetry(s.WorkDir)
		// The installation was left untouched by a failure at this point, so any backup is a stray.
		if exists(s.Target) {
			removeWithRetry(s.Backup)
		}
		return &outcome{Reason: s.Reason, Detail: s.Detail, ToVersion: s.ToVersion}
	case statusStaged:
		// Staged but never applied: the helper never ran, or stopped short of the swap without recording why. Nothing
		// was touched, so there is nothing to repair.
		slog.Info("discarding an update that was staged but never applied", "version", s.ToVersion)
		removeWithRetry(s.WorkDir)
		return &outcome{Reason: reasonAbandoned, ToVersion: s.ToVersion}
	default:
		errs.Log(errs.Newf("unrecognized update status %q", s.Status))
		removeWithRetry(s.WorkDir)
		return nil
	}
}

// stagingInUse reports whether the application that prepared the update s describes, or the helper it started to apply
// it, may still act on it. A process with this one's identifier is neither, since this one has only just started and
// has prepared nothing, and a process with the identifier of either may be one that has since been given it.
//
// Until the helper is recorded, which the application does on its way out, the application counts for as long as it
// runs, which may be indefinitely while its quit waits on the user, unless the process with its identifier is not the
// one whose start the state records. Once the helper is recorded, only the helper counts, and only within
// helperRunLimit of the state at statePath being written.
func stagingInUse(s *state, statePath string) bool {
	pid := s.ParentPID
	if s.HelperPID > 0 {
		if fi, err := os.Stat(statePath); err == nil && time.Since(fi.ModTime()) >= helperRunLimit {
			return false
		}
		pid = s.HelperPID
	}
	if pid <= 0 || pid == os.Getpid() || !processIsAlive(pid) {
		return false
	}
	if s.HelperPID <= 0 && s.ParentStart != "" {
		if started := processStartID(pid); started != "" && started != s.ParentStart {
			return false
		}
	}
	return true
}

// currentSweepDirs returns where an update to this installation would have left things.
func currentSweepDirs() []sweepDir {
	t, err := currentTarget()
	if err != nil {
		errs.Log(errs.NewWithCause("unable to determine the installation directory", err))
		return nil
	}
	return []sweepDir{{dir: t.Parent, targetBase: filepath.Base(t.Path)}}
}

// sweepStrays removes staging directories older than stagingOlderThan and displaced installations older than staleAge,
// which no update in progress could still need. A displaced installation always gets the longer bound, since one kept
// on purpose (see settle) is the only other copy of the application. This is the backstop for a helper killed before it
// could record anything, and it is what eventually clears a Windows image that could not be deleted while it was still
// running.
func (u *Updater) sweepStrays(dirs []sweepDir, stagingOlderThan time.Duration) {
	seen := make(map[sweepDir]bool, len(dirs))
	now := time.Now()
	stagingCutoff := now.Add(-stagingOlderThan)
	backupCutoff := now.Add(-staleAge)
	for _, one := range dirs {
		if one.dir == "" || seen[one] {
			continue
		}
		seen[one] = true
		entries, err := os.ReadDir(one.dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !u.isLeftover(entry.Name(), one.targetBase) {
				continue
			}
			fi, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}
			lastUsed := fi.ModTime()
			if at, ok := displacedAt(entry.Name(), one.targetBase); ok && at.After(lastUsed) {
				lastUsed = at
			}
			cutoff := backupCutoff
			if strings.HasPrefix(entry.Name(), u.workDirPrefix()) {
				cutoff = stagingCutoff
			}
			if lastUsed.After(cutoff) {
				continue
			}
			path := filepath.Join(one.dir, entry.Name())
			slog.Info("removing a leftover from an earlier update", "path", path)
			removeWithRetry(path)
		}
	}
}

// displacedAt returns the time target.freeBackupPath recorded in name, a backup of targetBase, and whether it recorded
// one. That is when the update was prepared, shortly before it moved the installation aside, which the backup's
// modification time cannot tell, since the rename keeps it.
func displacedAt(name, targetBase string) (time.Time, bool) {
	unique, ok := strings.CutPrefix(name, "."+targetBase+backupSuffix)
	if !ok {
		return time.Time{}, false
	}
	stamp, _, _ := strings.Cut(unique, "-")
	nanos, err := strconv.ParseInt(stamp, 36, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, nanos), true
}

// removeWithRetry removes a path, retrying briefly. The retries are for Windows, where the helper's own image cannot be
// deleted until it has finished exiting, which may be a moment after it started the replacement.
func removeWithRetry(path string) {
	if path == "" {
		return
	}
	err := retry(removeAttempts, removeDelay, func() error {
		if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	})
	if err != nil {
		errs.Log(errs.NewWithCause("unable to remove a leftover from an update", err), "path", path)
	}
}

// readTail returns up to the last limit bytes of the file at path, without surrounding white space, or nothing if it
// cannot be read.
func readTail(path string, limit int64) string {
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer xio.CloseIgnoringErrors(f)
	if fi, statErr := f.Stat(); statErr == nil && fi.Size() > limit {
		if _, err = f.Seek(fi.Size()-limit, io.SeekStart); err != nil {
			return ""
		}
	}
	data, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func exists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Lstat(path)
	return err == nil
}
