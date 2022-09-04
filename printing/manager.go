// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
)

// PrintManager holds the data needed by the print manager.
type PrintManager struct {
	resolver *zeroconf.Resolver
	lock     sync.RWMutex
	printers []*Printer
	lastScan time.Time
}

// NewPrintManager creates a new print manager.
func NewPrintManager() (*PrintManager, error) {
	resolver, err := zeroconf.NewResolver()
	if err != nil {
		return nil, errs.NewWithCause("unable to create zeroconf resolver", err)
	}
	return &PrintManager{resolver: resolver}, nil
}

// LastScan returns the time .Printers() was called with a scanDuration > 0.
func (p *PrintManager) LastScan() time.Time {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.lastScan
}

// Printers returns the available printers. If scanDuration > 0, a fresh scan will be performed, otherwise, the last
// discovered set of printers will be returned.
func (p *PrintManager) Printers(scanDuration time.Duration) []*Printer {
	if scanDuration < 1 {
		p.lock.RLock()
		printers := make([]*Printer, len(p.printers))
		copy(printers, p.printers)
		p.lock.RUnlock()
		return printers
	}
	done := make(chan struct{})
	entries := make(chan *zeroconf.ServiceEntry, 8)
	var printers []*Printer
	go func() {
		defer func() { done <- struct{}{} }()
		for entry := range entries {
			m := make(map[string]string, len(entry.Text)+1)
			for _, txt := range entry.Text {
				parts := strings.SplitN(txt, "=", 2)
				if len(parts) == 2 {
					m[parts[0]] = parts[1]
				}
			}
			id := m["UUID"]
			if id == "" {
				id = m["DUUID"]
			}
			authInfo := m["air"]
			if authInfo == "" {
				authInfo = "none"
			}
			printers = append(printers, &Printer{
				ID:               id,
				Name:             m["ty"],
				Host:             strings.TrimSuffix(entry.HostName, "."),
				Port:             entry.Port,
				RemotePath:       m["rp"],
				AuthInfoRequired: authInfo,
				MimeTypes:        append([]string(nil), strings.Split(m["pdl"], ",")...),
				Color:            m["Color"] == "T",
				Duplex:           m["Duplex"] == "T",
				httpClient:       &http.Client{},
			})
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.resolver.Browse(ctx, "_ipp._tcp", "local.", entries); err != nil {
		jot.Error(errs.NewWithCause("browsing for printers failed", err))
	}
	<-done
	p.lock.Lock()
	defer p.lock.Unlock()
	p.printers = make([]*Printer, len(printers))
	copy(p.printers, printers)
	p.lastScan = time.Now()
	return printers
}
