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
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/grandcat/zeroconf"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xstrings"
)

// newResolver is a hook for tests to simulate resolver creation failure.
var newResolver = zeroconf.NewResolver

// PrintManager holds the data needed by the print manager.
type PrintManager struct {
	printers map[string]*Printer
	lock     sync.RWMutex
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
	printers := maps.Values(p.printers)
	p.lock.RUnlock()
	return slices.SortedFunc(printers, func(a, b *Printer) int {
		if result := xstrings.NaturalCmp(a.Name, b.Name, true); result != 0 {
			return result
		}
		return xstrings.NaturalCmp(a.ID, b.ID, true)
	})
}

// ScanForPrinters first clears the previously known set of printers, then creates a goroutine to scan for printers,
// adding them to the set of known printers. You may pass nil for the out parameter if you do not need to receive the
// printers as they are discovered. If out is not nil, it will be closed when the scan completes.
func (p *PrintManager) ScanForPrinters(ctx context.Context, printers chan<- *Printer) {
	p.lock.Lock()
	p.printers = make(map[string]*Printer)
	p.lock.Unlock()
	resolver, err := newResolver()
	if err != nil {
		errs.Log(errs.NewWithCause("unable to create zeroconf resolver", err))
		if printers != nil {
			close(printers)
		}
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
			// Keep draining rather than returning: zeroconf's mainloop sends to this channel without checking the
			// context, so abandoning it would leave that goroutine blocked mid-send forever once the buffer fills.
			continue
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
		host := strings.TrimSuffix(entry.HostName, ".")
		if id == "" {
			// Printers that advertise neither UUID nor DUUID would otherwise all share the map key "", leaving only
			// one of them visible, so fall back to the host and port, which are unique per advertised printer.
			id = host + ":" + strconv.Itoa(entry.Port)
		}
		authInfo := m["air"]
		if authInfo == "" {
			authInfo = "none"
		}
		printer := &Printer{
			PrinterID: PrinterID{
				ID:   id,
				Name: m["ty"],
				Host: host,
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
