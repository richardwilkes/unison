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
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

var (
	_                      Drawable = &DrawableSVG{}
	circledChevronRightSVG *SVG
	sortAscendingSVG       *SVG
	sortDescendingSVG      *SVG
)

// DrawableSVG makes an SVG conform to the Drawable interface.
type DrawableSVG struct {
	SVG  *SVG
	Size geom32.Size
}

// SVG holds an SVG path. Note that this is a subset of SVG: just the 'd' attribute of the 'path' directive.
type SVG struct {
	size          geom32.Size
	unscaledPath  *Path
	scaledPathMap map[geom32.Size]*Path
}

// NewSVG creates a new SVG. The 'size' should be gotten from the original SVG's 'viewBox' parameter.
func NewSVG(size geom32.Size, svg string) (*SVG, error) {
	unscaledPath, err := NewPathFromSVGString(svg)
	if err != nil {
		return nil, err
	}
	return &SVG{
		size:          size,
		unscaledPath:  unscaledPath,
		scaledPathMap: make(map[geom32.Size]*Path),
	}, nil
}

// Size returns the original size.
func (s *SVG) Size() geom32.Size {
	return s.size
}

// OffsetToCenterWithinScaledSize returns the scaled offset values to use to keep the image centered within the given
// size.
func (s *SVG) OffsetToCenterWithinScaledSize(size geom32.Size) geom32.Point {
	scale := mathf32.Min(size.Width/s.size.Width, size.Height/s.size.Height)
	return geom32.NewPoint((size.Width-s.size.Width*scale)/2, (size.Height-s.size.Height*scale)/2)
}

// PathScaledTo returns the path with the specified scaling. You should not modify this path, as it is cached.
func (s *SVG) PathScaledTo(scale float32) *Path {
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
func (s *SVG) PathForSize(size geom32.Size) *Path {
	return s.PathScaledTo(mathf32.Min(size.Width/s.size.Width, size.Height/s.size.Height))
}

// LogicalSize implements the Drawable interface.
func (s *DrawableSVG) LogicalSize() geom32.Size {
	return s.Size
}

// DrawInRect implements the Drawable interface.
func (s *DrawableSVG) DrawInRect(canvas *Canvas, rect geom32.Rect, _ *SamplingOptions, paint *Paint) {
	canvas.Save()
	defer canvas.Restore()
	canvas.Translate(rect.X, rect.Y)
	canvas.DrawPath(s.SVG.PathForSize(rect.Size), paint)
}

// CircledChevronRightSVG returns an SVG that holds a circled chevron pointing towards the right.
func CircledChevronRightSVG() *SVG {
	if circledChevronRightSVG == nil {
		var err error
		circledChevronRightSVG, err = NewSVG(geom32.NewSize(512, 512), "M256 8c137 0 248 111 248 248S393 504 256 504 8 393 8 256 119 8 256 8zm113.9 231L234.4 103.5c-9.4-9.4-24.6-9.4-33.9 0l-17 17c-9.4 9.4-9.4 24.6 0 33.9L285.1 256 183.5 357.6c-9.4 9.4-9.4 24.6 0 33.9l17 17c9.4 9.4 24.6 9.4 33.9 0L369.9 273c9.4-9.4 9.4-24.6 0-34z")
		jot.FatalIfErr(err)
	}
	return circledChevronRightSVG
}

// SortAscendingSVG returns an SVG that holds an icon for an ascending sort.
func SortAscendingSVG() *SVG {
	if sortAscendingSVG == nil {
		var err error
		sortAscendingSVG, err = NewSVG(geom32.NewSize(512, 512), "M240 96h64a16 16 0 0 0 16-16V48a16 16 0 0 0-16-16h-64a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16zm0 128h128a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16zm256 192H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h256a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16zm-256-64h192a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16zm-64 0h-48V48a16 16 0 0 0-16-16H80a16 16 0 0 0-16 16v304H16c-14.19 0-21.37 17.24-11.29 27.31l80 96a16 16 0 0 0 22.62 0l80-96C197.35 369.26 190.22 352 176 352z")
		jot.FatalIfErr(err)
	}
	return sortAscendingSVG
}

// SortDescendingSVG returns an SVG that holds an icon for an descending sort.
func SortDescendingSVG() *SVG {
	if sortDescendingSVG == nil {
		var err error
		sortDescendingSVG, err = NewSVG(geom32.NewSize(512, 512), "M304 416h-64a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h64a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16zM16 160h48v304a16 16 0 0 0 16 16h32a16 16 0 0 0 16-16V160h48c14.21 0 21.38-17.24 11.31-27.31l-80-96a16 16 0 0 0-22.62 0l-80 96C-5.35 142.74 1.77 160 16 160zm416 0H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h192a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16zm-64 128H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h128a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16zM496 32H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h256a16 16 0 0 0 16-16V48a16 16 0 0 0-16-16z")
		jot.FatalIfErr(err)
	}
	return sortDescendingSVG
}
