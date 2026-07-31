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
	"image"
	"image/png"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OpenPrinting/goipp"
	"github.com/richardwilkes/canvas/codecs"
	"github.com/richardwilkes/toolbox/v2/check"
)

// encodeIPPAttributesResponse builds an encoded IPP Get-Printer-Attributes success response containing the given
// printer attributes.
func encodeIPPAttributesResponse(t *testing.T, attributes Attributes) []byte {
	t.Helper()
	rsp := goipp.NewResponse(goipp.DefaultVersion, goipp.StatusOk, 1)
	rsp.Printer = attributes.toIPP()
	data, err := rsp.EncodeBytes()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// newTestPrinter creates a Printer that talks to the given test server.
func newTestPrinter(t *testing.T, id string, srv *httptest.Server) *Printer {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	return &Printer{
		httpClient: srv.Client(),
		PrinterID: PrinterID{
			ID:   id,
			Name: id,
			Host: host,
			Port: port,
		},
	}
}

// newTestJobDialog creates a JobDialog with its content built, but without a window, a printer scan, or a UI event
// loop. The provided invoke function stands in for unison.InvokeTask; tests act as the UI thread by running the
// functions it receives.
func newTestJobDialog(invoke func(func())) *JobDialog {
	d := &JobDialog{
		mgr:               &PrintManager{},
		invoke:            invoke,
		printerAttributes: NewAttributes(nil).ForPrinter(),
		jobAttributes:     make(Attributes).ForJob(),
	}
	d.createContent()
	return d
}

// mustWrite writes an HTTP response body, reporting any error. It uses t.Error rather than t.Fatal because it runs on
// the test server's goroutine, not the test's.
func mustWrite(t *testing.T, w http.ResponseWriter, data []byte) {
	t.Helper()
	if _, err := w.Write(data); err != nil {
		t.Error(err)
	}
}

// waitForInvoke waits for the background fetch to hand a task to the UI thread and returns it.
func waitForInvoke(t *testing.T, ch <-chan func()) func() {
	t.Helper()
	select {
	case f := <-ch:
		return f
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for a UI task")
		return nil
	}
}

func TestSetPrinterDoesNotBlockOnSlowPrinter(t *testing.T) {
	c := check.New(t)
	attrs := make(Attributes)
	attrs.SetRange("copies-supported", goipp.Range{Lower: 1, Upper: 99}, true)
	attrs.SetKeyword("media-supported", "iso_a4_210x297mm", true)
	attrs.SetKeyword("media-default", "iso_a4_210x297mm", true)
	rspData := encodeIPPAttributesResponse(t, attrs)
	gate := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-gate:
		case <-r.Context().Done():
			return
		}
		mustWrite(t, w, rspData)
	}))
	defer srv.Close()
	defer close(gate) // must run before srv.Close, which waits for outstanding requests

	tasks := make(chan func(), 4)
	d := newTestJobDialog(func(f func()) { tasks <- f })
	p := newTestPrinter(t, "slow", srv)

	// The server will not respond until the gate is released, so setPrinter returning at all proves the network I/O
	// happens off the UI thread. The old synchronous implementation would sit here for the full 15 second timeout.
	d.setPrinter(p)
	c.Equal(p, d.printer)
	c.True(d.fetchingAttributes)
	c.False(d.copies.Enabled()) // controls are disabled while the fetch is in flight
	select {
	case <-tasks:
		t.Fatal("no UI task should have been delivered while the printer has not yet responded")
	default:
	}

	// Release the printer's response and apply the resulting UI task, as the event loop would.
	gate <- struct{}{}
	waitForInvoke(t, tasks)()
	c.False(d.fetchingAttributes)
	c.Equal(99, d.printerAttributes.MaxCopies())
	c.Equal([]string{"iso_a4_210x297mm"}, d.printerAttributes.SupportedMedia())
	c.True(d.copies.Enabled())
	media, ok := d.media.colorMode.Selected()
	c.True(ok)
	c.Equal(mediaString("iso_a4_210x297mm"), media)
}

func TestStalePrinterFetchIsDiscarded(t *testing.T) {
	c := check.New(t)
	slowAttrs := make(Attributes)
	slowAttrs.SetRange("copies-supported", goipp.Range{Lower: 1, Upper: 99}, true)
	slowData := encodeIPPAttributesResponse(t, slowAttrs)
	gate := make(chan struct{})
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-gate:
		case <-r.Context().Done():
			return
		}
		mustWrite(t, w, slowData)
	}))
	defer slowSrv.Close()
	defer close(gate)

	fastAttrs := make(Attributes)
	fastAttrs.SetRange("copies-supported", goipp.Range{Lower: 1, Upper: 7}, true)
	fastData := encodeIPPAttributesResponse(t, fastAttrs)
	fastSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mustWrite(t, w, fastData)
	}))
	defer fastSrv.Close()

	tasks := make(chan func(), 4)
	d := newTestJobDialog(func(f func()) { tasks <- f })
	slow := newTestPrinter(t, "slow", slowSrv)
	fast := newTestPrinter(t, "fast", fastSrv)

	// Select the slow printer, then switch to the fast one before the slow one has responded.
	d.setPrinter(slow)
	d.setPrinter(fast)
	waitForInvoke(t, tasks)()
	c.Equal(fast, d.printer)
	c.False(d.fetchingAttributes)
	c.Equal(7, d.printerAttributes.MaxCopies())

	// Now let the slow printer respond. Its stale result must be discarded rather than clobbering the fast printer's
	// attributes.
	gate <- struct{}{}
	waitForInvoke(t, tasks)()
	c.Equal(fast, d.printer)
	c.False(d.fetchingAttributes)
	c.Equal(7, d.printerAttributes.MaxCopies())
}

func TestRetrieveIcon(t *testing.T) {
	c := check.New(t)
	codecs.Register() // normally done during app startup, which these headless tests skip
	var buf bytes.Buffer
	c.NoError(png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mustWrite(t, w, buf.Bytes())
	}))
	defer srv.Close()
	p := newTestPrinter(t, "icons", srv)

	attrs := make(Attributes)
	attrs.SetURI("printer-icons", srv.URL+"/icon.png", true)
	c.NotNil(retrieveIcon(p, attrs.ForPrinter()))

	// No icon URLs means no icon and no network access.
	c.Nil(retrieveIcon(p, NewAttributes(nil).ForPrinter()))
}

func TestRetrieveIconChecksHTTPStatus(t *testing.T) {
	c := check.New(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		mustWrite(t, w, []byte("<html><body>Not Found</body></html>"))
	}))
	defer srv.Close()
	p := newTestPrinter(t, "icons", srv)
	attrs := make(Attributes)
	attrs.SetURI("printer-icons", srv.URL+"/icon.png", true)

	// A printer that returns an error page for its advertised icon URL must be reported as an HTTP failure, not as an
	// image decoding failure from feeding the HTML error page to the image codecs.
	var logBuf bytes.Buffer
	saved := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, nil)))
	defer slog.SetDefault(saved)
	c.Nil(retrieveIcon(p, attrs.ForPrinter()))
	c.Contains(logBuf.String(), "404")
	c.NotContains(logBuf.String(), "unable to create image")
}

func TestPrinterDiscoveredAtCloseDoesNotRearmFetches(t *testing.T) {
	c := check.New(t)
	rspData := encodeIPPAttributesResponse(t, make(Attributes))
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		mustWrite(t, w, rspData)
	}))
	defer srv.Close()

	tasks := make(chan func(), 4)
	d := newTestJobDialog(func(f func()) { tasks <- f })
	p := newTestPrinter(t, "late", srv)
	d.mgr.lock.Lock()
	d.mgr.printers = map[string]*Printer{p.ID: p}
	d.mgr.lock.Unlock()

	// A printer discovered just before the dialog is dismissed queues a popup rebuild that only runs on the UI thread
	// after RunModal has returned.
	d.printersChan = make(chan *Printer, 1)
	go d.collectPrinters()
	d.printersChan <- p
	rebuild := waitForInvoke(t, tasks)
	close(d.printersChan)

	// The user dismisses the dialog (RunModal's deferred close), then the queued rebuild runs. It must not select the
	// newly discovered printer and spawn a fresh attribute/icon fetch against the closed dialog's widgets.
	d.close()
	rebuild()
	c.Nil(d.printer)
	c.False(d.fetchingAttributes)
	select {
	case <-tasks:
		t.Fatal("no UI task should have been queued after the dialog was closed")
	default:
	}
	c.Equal(int32(0), requests.Load())
}
