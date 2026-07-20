// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package printing

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenPrinting/goipp"
	"github.com/richardwilkes/toolbox/v2/check"
)

func TestPrintSendsContentLength(t *testing.T) {
	c := check.New(t)
	rsp := goipp.NewResponse(goipp.DefaultVersion, goipp.StatusOk, 1)
	rspData, err := rsp.EncodeBytes()
	c.NoError(err)
	type captured struct {
		body             []byte
		transferEncoding []string
		contentLength    int64
	}
	capture := make(chan captured, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Error(readErr)
		}
		capture <- captured{body: body, transferEncoding: r.TransferEncoding, contentLength: r.ContentLength}
		mustWrite(t, w, rspData)
	}))
	defer srv.Close()
	p := newTestPrinter(t, "lengths", srv)
	fileData := []byte("pretend this is a document")
	c.NoError(p.Print(context.Background(), "job", "application/octet-stream", bytes.NewReader(fileData),
		len(fileData), make(Attributes).ForJob()))
	got := <-capture
	// Setting the Content-Length header on an outgoing request is silently ignored by net/http; only the request's
	// ContentLength field produces a declared length. Many embedded IPP implementations cannot handle a chunked body,
	// so the request must arrive with an accurate Content-Length and no chunked transfer encoding.
	c.Equal(0, len(got.transferEncoding))
	c.Equal(int64(len(got.body)), got.contentLength)
	c.True(bytes.HasSuffix(got.body, fileData))
}
