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
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xhttp"
	"github.com/richardwilkes/toolbox/v2/xio"
)

// download streams the asset to dstPath, verifying both its length and its SHA-256 as it arrives.
//
// The checksum is the one GitHub publishes alongside the download URL. Both travel in the same TLS-protected API
// response, so this is an integrity check against a truncated or corrupted transfer, not an independent proof of
// authenticity. On macOS the code signature check that follows establishes the latter; elsewhere the trust is in GitHub
// and the transport.
//
// progress, when not nil, is called with the running byte count. It is called often, so it must be cheap and must not
// block; throttling and marshaling to a UI thread are the caller's business.
//
// dstPath must not already exist. Once created, it is removed on any failure, including cancellation, so a failed
// attempt cannot leave a partial file for a later step to mistake for a complete one.
func download(ctx context.Context, client *http.Client, asset Asset, dstPath string, progress func(read int64)) (err error) {
	if asset.SHA256 == "" {
		return errs.New("refusing to download " + asset.Name + " without a checksum")
	}
	body, err := xhttp.StreamData(ctx, client, asset.URL)
	if err != nil {
		return errs.NewWithCause("unable to download "+asset.Name, err)
	}
	defer xio.DiscardAndCloseIgnoringErrors(body)

	f, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_EXCL, 0o600)
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

	hash := sha256.New()
	counter := &progressWriter{progress: progress}
	// Allowing one byte past the expected size is what makes "longer than advertised" detectable: without it, a body of
	// exactly the limit and one that overruns it look identical.
	n, err := io.Copy(io.MultiWriter(f, hash, counter), io.LimitReader(body, asset.Size+1))
	if err != nil {
		return errs.NewWithCause("unable to download "+asset.Name, err)
	}
	if n != asset.Size {
		return errs.Newf("%s is %d bytes rather than the expected %d", asset.Name, n, asset.Size)
	}
	if sum := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(sum, asset.SHA256) {
		return errs.New(asset.Name + " failed its integrity check")
	}
	return nil
}

type progressWriter struct {
	progress func(read int64)
	total    int64
}

func (w *progressWriter) Write(data []byte) (int, error) {
	w.total += int64(len(data))
	if w.progress != nil {
		w.progress(w.total)
	}
	return len(data), nil
}
