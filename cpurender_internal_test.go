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
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// withCPURendering runs fn with CPU rendering forced on, restoring the previous state afterward so other tests are not
// affected by the sticky fallback flag.
func withCPURendering(fn func()) {
	saved := cpuRenderingActive
	cpuRenderingActive = true
	defer func() { cpuRenderingActive = saved }()
	fn()
}

func TestFallbackToCPURenderingWarnsOnce(t *testing.T) {
	c := check.New(t)
	saved := cpuRenderingActive
	defer func() { cpuRenderingActive = saved }()
	cpuRenderingActive = false
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)
	c.False(IsCPURenderingActive())
	fallbackToCPURendering(errs.New("gl exploded"))
	c.True(IsCPURenderingActive())
	c.True(strings.Contains(buf.String(), "falling back to CPU rendering"))
	c.True(strings.Contains(buf.String(), "gl exploded"))
	buf.Reset()
	fallbackToCPURendering(errs.New("second failure"))
	c.True(IsCPURenderingActive())
	c.Equal("", buf.String())
}

func TestSurfaceCPURendering(t *testing.T) {
	c := check.New(t)
	withCPURendering(func() {
		s := &surface{}
		defer s.dispose()
		size := geom.NewSize(4, 3)
		scale := geom.NewPoint(2, 2)
		cnv, err := s.prepareCanvas(size, scale)
		c.NoError(err)
		c.NotNil(cnv)
		c.Nil(s.context)
		pixels := s.rasterPixmap()
		c.NotNil(pixels)
		c.Equal(int32(8), pixels.Width)
		c.Equal(int32(6), pixels.Height)
		// Fill with an opaque color and verify the pixels landed in the pixmap (premultiplied RGBA device words).
		paint := NewPaint()
		paint.SetColor(RGB(255, 0, 0))
		cnv.DrawPaint(paint)
		cnv.Flush()
		c.Equal(uint32(0xff0000ff), pixels.Pix[0])
		c.Equal(uint32(0xff0000ff), pixels.Pix[len(pixels.Pix)-1])
		// The same size must reuse the surface; a different size must rebuild it.
		first := s.surface
		_, err = s.prepareCanvas(size, scale)
		c.NoError(err)
		c.Equal(first, s.surface)
		_, err = s.prepareCanvas(geom.NewSize(5, 3), scale)
		c.NoError(err)
		c.NotEqual(first, s.surface)
		c.Equal(int32(10), s.rasterPixmap().Width)
	})
}

func TestSurfaceCPURenderingFlushAndDisposeAreSafe(t *testing.T) {
	c := check.New(t)
	withCPURendering(func() {
		s := &surface{}
		_, err := s.prepareCanvas(geom.NewSize(2, 2), geom.NewPoint(1, 1))
		c.NoError(err)
		// Neither flush (which only has GL work to do) nor dispose may touch a GL context that does not exist.
		s.flush(true)
		s.dispose()
		c.Nil(s.rasterPixmap())
	})
}
