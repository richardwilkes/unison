// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
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
	"slices"
	"strings"
	"sync"

	"github.com/grandcat/zeroconf"
	"github.com/richardwilkes/toolbox/collection/dict"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/txt"
)

// PrintManager holds the data needed by the print manager.
type PrintManager struct {
	lock     sync.RWMutex
	printers map[string]*Printer
}

// LookupPrinter returns a printer by ID, or nil if it is not in our currently discovered set.
func (p *PrintManager) LookupPrinter(id PrinterID) *Printer {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.printers[id.ID]
}

// Printers returns the previously discovered available printers, sorted by name.
func (p *PrintManager) Printers() []*Printer {
	p.lock.RLock()
	printers := dict.Values(p.printers)
	p.lock.RUnlock()
	slices.SortFunc(printers, func(a, b *Printer) int {
		if result := txt.NaturalCmp(a.Name, b.Name, true); result != 0 {
			return result
		}
		return txt.NaturalCmp(a.ID, b.ID, true)
	})
	return printers
}

// ScanForPrinters first clears the previously known set of printers, then creates a goroutine to scan for printers,
// adding them to the set of known printers. You may pass nil for the out parameter if you do not need to receive the
// printers as they are discovered. If out is not nil, it will be closed when the scan completes.
func (p *PrintManager) ScanForPrinters(ctx context.Context, printers chan<- *Printer) {
	p.lock.Lock()
	p.printers = make(map[string]*Printer)
	p.lock.Unlock()
	resolver, err := zeroconf.NewResolver()
	if err != nil {
		errs.Log(errs.NewWithCause("unable to create zeroconf resolver", err))
		return
	}
	entries := make(chan *zeroconf.ServiceEntry, 8)
	go p.collectPrinters(ctx, entries, printers)
	if err = resolver.Browse(ctx, "_ipp._tcp", "local.", entries); err != nil {
		errs.Log(errs.NewWithCause("browsing for printers failed", err))
	}
}

func (p *PrintManager) collectPrinters(ctx context.Context, in <-chan *zeroconf.ServiceEntry, out chan<- *Printer) {
	defer func() {
		if out != nil {
			close(out)
		}
	}()
	for entry := range in {
		if ctx.Err() != nil {
			return
		}
		m := make(map[string]string, len(entry.Text)+1)
		for _, text := range entry.Text {
			parts := strings.SplitN(text, "=", 2)
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
		printer := &Printer{
			PrinterID: PrinterID{
				ID:   id,
				Name: m["ty"],
				Host: strings.TrimSuffix(entry.HostName, "."),
				Port: entry.Port,
			},
			RemotePath:       m["rp"],
			AuthInfoRequired: authInfo,
			MimeTypes:        append([]string(nil), strings.Split(m["pdl"], ",")...),
			Color:            m["Color"] == "T",
			Duplex:           m["Duplex"] == "T",
			httpClient:       &http.Client{},
		}
		p.lock.Lock()
		p.printers[printer.ID] = printer
		p.lock.Unlock()
		if out != nil {
			out <- printer
		}
	}
}

// NewJobDialog creates a dialog to configure a print job. 'printerID' may be empty or an ID for a printer that is no
// longer available.
func (p *PrintManager) NewJobDialog(id PrinterID, mimeType string, attributes *JobAttributes) *JobDialog {
	return newJobDialog(p, id, mimeType, attributes)
}
