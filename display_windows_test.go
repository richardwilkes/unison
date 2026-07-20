// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"fmt"
	"sync"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestW32EnumDisplaysSerializesScratchSlice verifies that concurrent display enumerations cannot interleave their use
// of the package-level scratch slice the EnumDisplayMonitors callback appends to. Display queries normally happen on
// the UI thread, but they are also reachable from other goroutines, and without serialization one caller's reset of
// the scratch slice races another caller's appends, corrupting both display lists. Each goroutine here simulates an
// enumeration that yields a distinctive display count and PPI, then checks it got back exactly what it produced; run
// with -race, any unsynchronized access to the scratch slice is reported as well.
func TestW32EnumDisplaysSerializesScratchSlice(t *testing.T) {
	c := check.New(t)
	const goroutines = 8
	const iterations = 200
	errCh := make(chan string, goroutines)
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count := g + 1
			for range iterations {
				displays := w32EnumDisplays(func() {
					for range count {
						w32Displays = append(w32Displays, &Display{PPI: g})
					}
				})
				if len(displays) != count {
					errCh <- fmt.Sprintf("goroutine %d: got %d displays, expected %d", g, len(displays), count)
					return
				}
				for _, d := range displays {
					if d == nil || d.PPI != g {
						errCh <- fmt.Sprintf("goroutine %d: got a display belonging to another enumeration", g)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for msg := range errCh {
		t.Error(msg)
	}
	// The scratch slice must be left empty so no caller's result is retained or handed to a later caller.
	c.Equal(0, len(w32Displays))
}

// TestW32DisplayDPI verifies that the zero DPI reported by a failed query maps to the default 96, since callers turn
// DPI values into scale factors that are divided by, and a zero scale would turn downstream geometry into Inf/NaN.
func TestW32DisplayDPI(t *testing.T) {
	c := check.New(t)
	c.Equal(uint32(96), w32DisplayDPI(0))
	c.Equal(uint32(96), w32DisplayDPI(96))
	c.Equal(uint32(144), w32DisplayDPI(144))
}

// TestUsableInWindowUnits verifies that the usable rect handed to cross-platform window math keeps its origin in the
// raw global pixel space window positions use on this platform, while its size is converted to the logical units that
// window sizes use.
func TestUsableInWindowUnits(t *testing.T) {
	c := check.New(t)
	d := &Display{
		Usable: geom.NewRect(3840, 32, 3840, 2128),
		Scale:  geom.NewPoint(2, 2),
	}
	c.Equal(geom.NewRect(3840, 32, 1920, 1064), d.usableInWindowUnits())
}
