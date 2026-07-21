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
	"context"
	"testing"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/errs"
)

func TestScanForPrintersClosesOutputOnResolverFailure(t *testing.T) {
	c := check.New(t)
	orig := newResolver
	newResolver = func(_ ...zeroconf.ClientOption) (*zeroconf.Resolver, error) {
		return nil, errs.New("simulated resolver failure")
	}
	defer func() { newResolver = orig }()
	var mgr PrintManager
	out := make(chan *Printer)
	mgr.ScanForPrinters(context.Background(), out)
	// The documented contract is that the output channel is closed when the scan completes, including when the scan
	// could not be started at all. A receiver such as cmd/printerscan would otherwise block forever.
	select {
	case _, ok := <-out:
		c.False(ok)
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for the printers channel to be closed")
	}
	// A nil output channel must not panic in the failure path.
	mgr.ScanForPrinters(context.Background(), nil)
}

func TestCollectPrintersDrainsEntriesAfterCancellation(t *testing.T) {
	c := check.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var mgr PrintManager
	mgr.printers = make(map[string]*Printer)
	in := make(chan *zeroconf.ServiceEntry)
	out := make(chan *Printer, 1)
	go mgr.collectPrinters(ctx, in, out)
	// zeroconf's mainloop sends to the entries channel without checking the context, so the collector must keep
	// draining after cancellation. It previously returned instead, which left the sender blocked forever once the
	// 8-slot buffer filled; with an unbuffered channel here, that regression would stall on the second send below.
	for range 16 {
		select {
		case in <- &zeroconf.ServiceEntry{HostName: "printer.local.", Port: 631}:
		case <-time.After(10 * time.Second):
			t.Fatal("collector stopped draining entries after cancellation")
		}
	}
	close(in)
	select {
	case _, ok := <-out:
		c.False(ok)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the printers channel to be closed")
	}
	// Entries received after cancellation must not have been recorded as printers.
	c.Equal(0, len(mgr.Printers()))
}

func TestCollectPrintersKeepsDistinctUUIDLessPrinters(t *testing.T) {
	c := check.New(t)
	var mgr PrintManager
	mgr.printers = make(map[string]*Printer)
	in := make(chan *zeroconf.ServiceEntry, 3)
	in <- &zeroconf.ServiceEntry{HostName: "one.local.", Port: 631, Text: []string{"ty=One"}}
	in <- &zeroconf.ServiceEntry{HostName: "two.local.", Port: 632, Text: []string{"ty=Two"}}
	in <- &zeroconf.ServiceEntry{HostName: "three.local.", Port: 631, Text: []string{"ty=Three", "UUID=abc"}}
	close(in)
	mgr.collectPrinters(context.Background(), in, nil)
	// Printers advertising neither UUID nor DUUID previously collapsed into the single map key "", so only one of
	// them survived. Each must instead receive a distinct, non-empty fallback ID, while an advertised UUID is still
	// used as-is.
	printers := mgr.Printers()
	c.Equal(3, len(printers))
	seen := make(map[string]bool)
	for _, p := range printers {
		c.True(p.ID != "")
		c.False(seen[p.ID])
		seen[p.ID] = true
	}
	c.True(seen["abc"])
	c.NotNil(mgr.LookupPrinter(PrinterID{ID: "one.local:631"}))
	c.NotNil(mgr.LookupPrinter(PrinterID{ID: "two.local:632"}))
}
