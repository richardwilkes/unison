// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

// SvgPath holds an svg path. Note that this is a subset of svg: just the 'd' attribute of the 'path' directive.
type SvgPath struct {
	size          geom32.Size
	unscaledPath  *Path
	scaledPathMap map[geom32.Size]*Path
}

// NewSvgPath creates a new SvgPath. The 'size' should be gotten from the original svg's 'viewBox' parameter.
func NewSvgPath(size geom32.Size, svgPath string) (*SvgPath, error) {
	unscaledPath, err := NewPathFromSVGString(svgPath)
	if err != nil {
		return nil, err
	}
	return &SvgPath{
		size:          size,
		unscaledPath:  unscaledPath,
		scaledPathMap: make(map[geom32.Size]*Path),
	}, nil
}

// Size returns the original size.
func (s *SvgPath) Size() geom32.Size {
	return s.size
}

// OffsetToCenterWithinScaledSize returns the scaled offset values to use to keep the image centered within the given
// size.
func (s *SvgPath) OffsetToCenterWithinScaledSize(size geom32.Size) geom32.Point {
	scale := mathf32.Min(size.Width/s.size.Width, size.Height/s.size.Height)
	return geom32.NewPoint((size.Width-s.size.Width*scale)/2, (size.Height-s.size.Height*scale)/2)
}

// PathScaledTo returns the path with the specified scaling. You should not modify this path, as it is cached.
func (s *SvgPath) PathScaledTo(scale float32) *Path {
	if scale == 1 {
		return s.unscaledPath
	}
	scaledSize := geom32.NewSize(scale, scale)
	p, ok := s.scaledPathMap[scaledSize]
	if !ok {
		p = s.unscaledPath.NewScaledSize(scaledSize)
		s.scaledPathMap[scaledSize] = p
	}
	return p
}

// PathForSize returns the path scaled to fit in the specified size. You should not modify this path, as it is cached.
func (s *SvgPath) PathForSize(size geom32.Size) *Path {
	return s.PathScaledTo(mathf32.Min(size.Width/s.size.Width, size.Height/s.size.Height))
}
