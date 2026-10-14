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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestCPUOnlySurfaceSkipsGL verifies that a CPU-only surface renders into a raster pixmap while the rest of the process
// renders with OpenGL, without ever asking for a GL context or tripping the process-wide CPU fallback.
func TestCPUOnlySurfaceSkipsGL(t *testing.T) {
	c := check.New(t)
	saved := cpuRenderingActive
	t.Cleanup(func() { cpuRenderingActive = saved })
	cpuRenderingActive = false
	s := &surface{cpuOnly: true}
	defer s.dispose()
	cnv, err := s.prepareCanvas(geom.NewSize(4, 3), geom.NewPoint(2, 2))
	c.NoError(err)
	c.NotNil(cnv)
	c.True(s.context == nil, "no GL context should have been created")
	pixels := s.rasterPixmap()
	c.NotNil(pixels)
	c.Equal(int32(8), pixels.Width)
	c.Equal(int32(6), pixels.Height)
	c.False(cpuRenderingActive, "the process should not have fallen back to CPU rendering")
}
