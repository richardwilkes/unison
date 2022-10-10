// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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
	skiaGLInterface  skia.GLInterface
	skiaColorspace   skia.ColorSpace
	skiaSurfaceProps skia.SurfaceProps
)

type surface struct {
	context skia.DirectContext
	backend skia.BackendRenderTarget
	surface skia.Surface
	size    Size
}

func (s *surface) prepareCanvas(size Size, dirty Rect, scaleX, scaleY float32) (*Canvas, error) {
	if s.size != size {
		if s.surface != nil {
			skia.SurfaceUnref(s.surface)
			s.surface = nil
			skia.BackendRenderTargetDelete(s.backend)
			s.backend = nil
		}
		s.size = size
	}
	if s.surface == nil {
		s.context = skia.ContextMakeGL(defaultGLInterface())
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
	c.RestoreToCount(1)
	c.SetMatrix(NewScaleMatrix(scaleX, scaleY))
	return c, nil
}

func (s *surface) dispose() {
	if s.surface != nil {
		releaseImagesForContext(s.context)
		skia.SurfaceUnref(s.surface)
		s.surface = nil
	}
	if s.backend != nil {
		skia.BackendRenderTargetDelete(s.backend)
		s.backend = nil
	}
	if s.context != nil {
		skia.ContextAbandonContext(s.context)
		s.context = nil
	}
}

func defaultGLInterface() skia.GLInterface {
	if skiaGLInterface == nil {
		skiaGLInterface = skia.GLInterfaceCreateNativeInterface()
	}
	return skiaGLInterface
}

func defaultSurfaceProps() skia.SurfaceProps {
	if skiaSurfaceProps == nil {
		skiaSurfaceProps = skia.SurfacePropsNew(skia.PixelGeometryRGBH)
	}
	return skiaSurfaceProps
}
