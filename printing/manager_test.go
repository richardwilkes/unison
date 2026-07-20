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
