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
	"github.com/richardwilkes/canvas/canvas"
	skgeom "github.com/richardwilkes/canvas/geom"
	"github.com/richardwilkes/canvas/gpu"
	"github.com/richardwilkes/canvas/gpu/gl"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
)

var opengl *gl.Interface

type genericSurface interface {
	Canvas() *canvas.Canvas
}

type surface struct {
	context *gl.DirectContext
	surface genericSurface
	size    geom.Size
	scale   geom.Point
}

func (s *surface) prepareCanvas(size geom.Size, scale geom.Point) (*Canvas, error) {
	if s.size != size || scale != s.scale {
		s.partialDispose()
		s.size = size
		s.scale = scale
	}
	if s.surface == nil {
		if s.context == nil {
			s.context = gl.MakeGLDirectContext(defaultOpenGL(), nil)
		}
		if s.surface = gl.NewRenderTargetSurfaceFromBackendRenderTarget(s.context, gpu.ColorTypeRGBA8888,
			skgeom.ISize{Width: int32(size.Width * scale.X), Height: int32(size.Height * scale.Y)},
			gl.FormatFromEnum(gl.RGBA8), 1, 8, 0, gpu.OriginBottomLeft, nil); s.surface == nil {
			return nil, errs.New("unable to create rendering surface")
		}
	}
	c := &Canvas{
		canvas:  s.surface.Canvas(),
		surface: s,
	}
	s.context.ResetContext(gl.AllBackendState)
	c.RestoreToCount(1)
	c.SetMatrix(geom.NewScaleMatrix(scale.X, scale.Y))
	return c, nil
}

func (s *surface) flush(syncCPU bool) {
	if s != nil && s.surface != nil && s.context != nil {
		s.context.FlushAndSubmit(syncCPU)
	}
}

func (s *surface) partialDispose() {
	if s.surface != nil {
		s.surface = nil
	}
}

func (s *surface) dispose() {
	s.partialDispose()
	if s.context != nil {
		releaseImagesForContext(s.context)
		s.context.AbandonContext()
		s.context.Destroy()
		s.context = nil
	}
}

func defaultOpenGL() *gl.Interface {
	if opengl == nil {
		opengl = gl.MakeNativeInterface()
	}
	return opengl
}
