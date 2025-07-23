// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package packager

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// Package performs the platform-specific packaging for a Unison application.
func Package(cfg *Config, version string, createDist bool) error {
	cfg.prepare(version)
	if err := prepareBinary(cfg); err != nil {
		return err
	}
	if createDist {
		return generateDistribution(cfg)
	}
	return nil
}

func copyFile(from, to string, mode fs.FileMode) error { //nolint:unused // This is used only on some platforms
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return errs.Wrap(err)
	}
	f, err := os.OpenFile(to, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	var s *os.File
	if s, err = os.Open(from); err != nil {
		err = errs.Wrap(err)
		return err
	}
	defer func() {
		if closeErr := s.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	if _, err = io.Copy(f, s); err != nil {
		err = errs.Wrap(err)
		return err
	}
	return nil
}
