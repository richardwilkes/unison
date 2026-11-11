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
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xio"
)

// maxPayloadSize bounds what an archive may expand to: generous headroom over any plausible executable, while still
// refusing a decompression bomb that could fill the disk from a few kilobytes of crafted gzip.
const maxPayloadSize = 512 << 20

// executableModePerm is the mode the extracted payload is written with. The archives record 0755 already, but it is set
// explicitly rather than taken from the archive so that a mangled or hostile entry cannot produce something
// unexecutable, or setuid.
const executableModePerm = 0o755

// The Linux and Windows distributions are built by upack and hold exactly one entry -- the executable, at the root,
// with no directory component (see packager_linux.go and packager_windows.go). Both extractors below enforce exactly
// that rather than a more permissive walk, because anything else means the file is not what we asked for. In
// particular, requiring the name to match with no directory component at all makes path traversal structurally
// impossible rather than something that has to be filtered out.

// extractSingleTGZ extracts the single regular file named wantName from the gzip-compressed tar stream in r, writing it
// to dstPath. Any other content is an error. dstPath is removed if extraction fails, so a failure never leaves a
// partial file behind for something else to pick up.
func extractSingleTGZ(r io.Reader, wantName, dstPath string) (err error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return errs.NewWithCause("unable to read the compressed archive", err)
	}
	defer xio.CloseIgnoringErrors(gr)
	tr := tar.NewReader(gr)
	hdr, err := tr.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return errs.New("the archive is empty")
		}
		return errs.NewWithCause("unable to read the archive", err)
	}
	if hdr.Typeflag != tar.TypeReg {
		return errs.Newf("the archive holds %s, which is not a regular file", hdr.Name)
	}
	if filepath.ToSlash(hdr.Name) != wantName {
		return errs.Newf("the archive holds %s rather than %s", hdr.Name, wantName)
	}
	if err = writePayload(tr, dstPath); err != nil {
		return err
	}
	// Only now check for trailing entries: an archive holding more than the one file we expect is not the archive the
	// packager produces, so treat it as untrustworthy rather than silently using the first entry.
	if _, err = tr.Next(); err == nil {
		removeIgnoringErrors(dstPath)
		return errs.New("the archive holds more than one file")
	} else if !errors.Is(err, io.EOF) {
		removeIgnoringErrors(dstPath)
		return errs.NewWithCause("unable to read the archive", err)
	}
	return nil
}

// extractSingleZip extracts the single file named wantName from the zip archive at archivePath, writing it to dstPath.
// Any other content is an error.
func extractSingleZip(archivePath, wantName, dstPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return errs.NewWithCause("unable to read the archive", err)
	}
	defer xio.CloseIgnoringErrors(zr)
	if len(zr.File) != 1 {
		return errs.Newf("the archive holds %d files rather than one", len(zr.File))
	}
	f := zr.File[0]
	if f.FileInfo().Mode()&os.ModeType != 0 {
		return errs.Newf("the archive holds %s, which is not a regular file", f.Name)
	}
	if filepath.ToSlash(f.Name) != wantName {
		return errs.Newf("the archive holds %s rather than %s", f.Name, wantName)
	}
	r, err := f.Open()
	if err != nil {
		return errs.NewWithCause("unable to read "+f.Name+" from the archive", err)
	}
	defer xio.CloseIgnoringErrors(r)
	return writePayload(r, dstPath)
}

// writePayload copies at most maxPayloadSize bytes from r into dstPath, which must not already exist and is created
// with an executable mode. Once created, it is removed on any failure.
func writePayload(r io.Reader, dstPath string) (err error) {
	if err = os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return errs.Wrap(err)
	}
	//nolint:gosec // The payload is an executable, so it must be executable by everyone who may run it
	f, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_EXCL, executableModePerm)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
		if err != nil {
			removeIgnoringErrors(dstPath)
		}
	}()
	// One extra byte beyond the limit is allowed through so that hitting exactly maxPayloadSize can be told apart from
	// exceeding it.
	n, err := io.Copy(f, io.LimitReader(r, maxPayloadSize+1))
	if err != nil {
		return errs.NewWithCause("unable to extract the archive", err)
	}
	if n > maxPayloadSize {
		return errs.Newf("the archive expands to more than %d bytes", int64(maxPayloadSize))
	}
	if n == 0 {
		return errs.New("the archive holds an empty file")
	}
	// The mode requested at creation is filtered by the umask; set it explicitly so that a umask of 077 does not
	// install an application no other account on the machine can run.
	if err = f.Chmod(executableModePerm); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func removeIgnoringErrors(path string) {
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		errs.Log(errs.NewWithCause("unable to remove "+path, err))
	}
}
