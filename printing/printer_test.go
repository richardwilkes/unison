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
	"time"

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

// checkOperationAttributeOrder decodes an IPP request body and verifies the RFC 8011 §4.1.4 requirement that
// attributes-charset is the first operation attribute and attributes-natural-language is the second. CUPS enforces
// exactly this and rejects requests that violate it with client-error-bad-request.
func checkOperationAttributeOrder(t *testing.T, body []byte) {
	t.Helper()
	c := check.New(t)
	var req goipp.Message
	// Decode from a reader rather than DecodeBytes so trailing document data after the IPP portion is ignored.
	c.NoError(req.Decode(bytes.NewReader(body)))
	c.True(len(req.Operation) >= 2)
	c.Equal("attributes-charset", req.Operation[0].Name)
	c.Equal(goipp.TagCharset, req.Operation[0].Values[0].T)
	c.Equal("utf-8", req.Operation[0].Values[0].V.String())
	c.Equal("attributes-natural-language", req.Operation[1].Name)
	c.Equal(goipp.TagLanguage, req.Operation[1].Values[0].T)
}

func TestRequestOperationAttributeOrdering(t *testing.T) {
	c := check.New(t)
	rspData, err := goipp.NewResponse(goipp.DefaultVersion, goipp.StatusOk, 1).EncodeBytes()
	c.NoError(err)
	bodies := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Error(readErr)
		}
		bodies <- body
		mustWrite(t, w, rspData)
	}))
	defer srv.Close()
	p := newTestPrinter(t, "ordering", srv)

	_, err = p.Attributes(time.Minute, false)
	c.NoError(err)
	checkOperationAttributeOrder(t, <-bodies)

	_, err = p.Validate(context.Background(), "job", "application/pdf", make(Attributes).ForJob())
	c.NoError(err)
	checkOperationAttributeOrder(t, <-bodies)

	fileData := []byte("pretend this is a document")
	c.NoError(p.Print(context.Background(), "job", "application/octet-stream", bytes.NewReader(fileData),
		len(fileData), make(Attributes).ForJob()))
	checkOperationAttributeOrder(t, <-bodies)
}
