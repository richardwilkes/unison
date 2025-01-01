// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/go-gl/gl/v3.2-core/gl"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/unison/internal/skia"
)

var (
	skiaGL           skia.GLInterface
	skiaColorspace   skia.ColorSpace
	skiaSurfaceProps skia.SurfaceProps
)

type surface struct {
	context skia.DirectContext
	backend skia.BackendRenderTarget
	surface skia.Surface
	size    Size
	scaleX  float32
	scaleY  float32
}

func (s *surface) prepareCanvas(size Size, _ Rect, scaleX, scaleY float32) (*Canvas, error) {
	if s.size != size || scaleX != s.scaleX || scaleY != s.scaleY {
		s.partialDispose()
		s.size = size
		s.scaleX = scaleX
		s.scaleY = scaleY
	}
	if s.surface == nil {
		if s.context == nil {
			s.context = skia.ContextMakeGL(defaultSkiaGL())
		}
		var fbo int32
		gl.GetIntegerv(gl.FRAMEBUFFER_BINDING, &fbo)
		if s.backend = skia.BackendRenderTargetNewGL(int(size.Width*scaleX), int(size.Height*scaleY), 1, 8,
			&skia.GLFrameBufferInfo{
				Fboid:  uint32(fbo),
				Format: gl.RGBA8,
			}); s.backend == nil {
			return nil, errs.New("unable to create backend render target")
		}
		if s.surface = skia.SurfaceNewBackendRenderTarget(s.context, s.backend, skia.SurfaceOriginBottomLeft,
			skia.ColorTypeRGBA8888, skiaColorspace, defaultSurfaceProps()); s.surface == nil {
			return nil, errs.New("unable to create backend rendering surface")
		}
	}
	c := &Canvas{
		canvas:  skia.SurfaceGetCanvas(s.surface),
		surface: s,
	}
	skia.ContextReset(s.context)
	c.RestoreToCount(1)
	c.SetMatrix(NewScaleMatrix(scaleX, scaleY))
	return c, nil
}

func (s *surface) flush(syncCPU bool) {
	if s != nil && s.surface != nil {
		skia.ContextFlushAndSubmit(s.context, syncCPU)
	}
}

func (s *surface) partialDispose() {
	if s.surface != nil {
		skia.SurfaceUnref(s.surface)
		s.surface = nil
	}
	if s.backend != nil {
		skia.BackendRenderTargetDelete(s.backend)
		s.backend = nil
	}
}

func (s *surface) dispose() {
	s.partialDispose()
	if s.context != nil {
		releaseImagesForContext(s.context)
		skia.ContextAbandonContext(s.context)
		skia.ContextUnref(s.context)
		s.context = nil
	}
}

func defaultSkiaGL() skia.GLInterface {
	if skiaGL == nil {
		skiaGL = skia.GLInterfaceCreateNativeInterface()
	}
	return skiaGL
}

func defaultSurfaceProps() skia.SurfaceProps {
	if skiaSurfaceProps == nil {
		skiaSurfaceProps = skia.SurfacePropsNew(skia.PixelGeometryRGBH)
	}
	return skiaSurfaceProps
}
