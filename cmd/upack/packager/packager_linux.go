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
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
)

func prepareBinary(_ *Config) error {
	return nil
}

func generateDistribution(cfg *Config) (err error) {
	dstPath := cfg.ExecutableName + "-" + cfg.version + "-linux-" + runtime.GOARCH + ".tgz"
	if xos.FileExists(dstPath) {
		if err = os.Remove(dstPath); err != nil {
			return errs.Wrap(err)
		}
	}
	var fi os.FileInfo
	if fi, err = os.Stat(cfg.ExecutableName); err != nil {
		return errs.Wrap(err)
	}
	var in, out *os.File
	if in, err = os.Open(cfg.ExecutableName); err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	if out, err = os.Create(dstPath); err != nil {
		err = errs.Wrap(err)
		return err
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	var gw *gzip.Writer
	if gw, err = gzip.NewWriterLevel(out, gzip.BestCompression); err != nil {
		err = errs.Wrap(err)
		return err
	}
	defer func() {
		if closeErr := gw.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	w := tar.NewWriter(gw)
	if err = w.WriteHeader(&tar.Header{
		Name:    cfg.ExecutableName,
		Size:    fi.Size(),
		Mode:    0o755,
		ModTime: time.Now(),
	}); err != nil {
		err = errs.Wrap(err)
		return err
	}
	defer func() {
		if closeErr := w.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	if _, err = io.Copy(w, in); err != nil {
		err = errs.Wrap(err)
		return err
	}
	return nil
}
