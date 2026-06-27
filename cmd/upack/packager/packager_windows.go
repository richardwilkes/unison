// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package packager

import (
	"archive/zip"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
)

func prepareBinary(cfg *Config) error {
	rs := &resourceSet{}
	rs.setManifest(cfg.Description)
	if err := rs.addWindowsIcons(cfg.AppIcon, cfg.FileInfo); err != nil {
		return err
	}
	rs.setVersionInfo(&versionInfo{
		productName:     cfg.FullName,
		fullVersion:     cfg.version,
		shortVersion:    cfg.shortAppVersion(),
		fileName:        cfg.ExecutableName + ".exe",
		companyName:     cfg.CopyrightHolder,
		fileDescription: cfg.Description,
		copyright:       cfg.copyright(),
		trademarks:      cfg.Trademarks,
	})
	return rs.writeSyso()
}

func generateDistribution(cfg *Config) (err error) {
	dstPath := cfg.ExecutableName + "-" + cfg.version + "-windows-" + runtime.GOARCH + ".zip"
	if xos.FileExists(dstPath) {
		if err = os.Remove(dstPath); err != nil {
			return errs.Wrap(err)
		}
	}
	exeName := cfg.ExecutableName + ".exe"
	var in, out *os.File
	if in, err = os.Open(exeName); err != nil {
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
	zw := zip.NewWriter(out)
	defer func() {
		if closeErr := zw.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	var fw io.Writer
	hdr := &zip.FileHeader{
		Name:     exeName,
		Method:   zip.Deflate,
		Modified: time.Now(),
	}
	hdr.SetMode(0o755)
	if fw, err = zw.CreateHeader(hdr); err != nil {
		err = errs.Wrap(err)
		return err
	}
	if _, err = io.Copy(fw, in); err != nil {
		err = errs.Wrap(err)
		return err
	}
	return nil
}
