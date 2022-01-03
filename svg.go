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
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

var (
	_                      Drawable = &DrawableSVG{}
	circledChevronRightSVG *SVG
	circledXSVG            *SVG
	documentSVG            *SVG
	sortAscendingSVG       *SVG
	sortDescendingSVG      *SVG
	windowMaximizeSVG      *SVG
	windowRestoreSVG       *SVG
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

// CircledXSVG returns an SVG that holds an icon for closing content.
func CircledXSVG() *SVG {
	if circledXSVG == nil {
		var err error
		circledXSVG, err = NewSVG(geom32.NewSize(512, 512), "M256 8C119 8 8 119 8 256s111 248 248 248 248-111 248-248S393 8 256 8zm121.6 313.1c4.7 4.7 4.7 12.3 0 17L338 377.6c-4.7 4.7-12.3 4.7-17 0L256 312l-65.1 65.6c-4.7 4.7-12.3 4.7-17 0L134.4 338c-4.7-4.7-4.7-12.3 0-17l65.6-65-65.6-65.1c-4.7-4.7-4.7-12.3 0-17l39.6-39.6c4.7-4.7 12.3-4.7 17 0l65 65.7 65.1-65.6c4.7-4.7 12.3-4.7 17 0l39.6 39.6c4.7 4.7 4.7 12.3 0 17L312 256l65.6 65.1z")
		jot.FatalIfErr(err)
	}
	return circledXSVG
}

// DocumentSVG returns an SVG that holds an icon for a document.
func DocumentSVG() *SVG {
	if documentSVG == nil {
		var err error
		documentSVG, err = NewSVG(geom32.NewSize(384, 512), "M224 136V0H24C10.7 0 0 10.7 0 24v464c0 13.3 10.7 24 24 24h336c13.3 0 24-10.7 24-24V160H248c-13.2 0-24-10.8-24-24zm160-14.1v6.1H256V0h6.1c6.4 0 12.5 2.5 17 7l97.9 98c4.5 4.5 7 10.6 7 16.9z")
		jot.FatalIfErr(err)
	}
	return documentSVG
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

// WindowMaximizeSVG returns an SVG that holds an icon for maximizing a window.
func WindowMaximizeSVG() *SVG {
	if windowMaximizeSVG == nil {
		var err error
		windowMaximizeSVG, err = NewSVG(geom32.NewSize(512, 512), "M464 32H48C21.5 32 0 53.5 0 80v352c0 26.5 21.5 48 48 48h416c26.5 0 48-21.5 48-48V80c0-26.5-21.5-48-48-48zm-16 160H64v-84c0-6.6 5.4-12 12-12h360c6.6 0 12 5.4 12 12v84z")
		jot.FatalIfErr(err)
	}
	return windowMaximizeSVG
}

// WindowRestoreSVG returns an SVG that holds an icon for restoring a maximized window.
func WindowRestoreSVG() *SVG {
	if windowRestoreSVG == nil {
		var err error
		windowRestoreSVG, err = NewSVG(geom32.NewSize(512, 512), "M512 48v288c0 26.5-21.5 48-48 48h-48V176c0-44.1-35.9-80-80-80H128V48c0-26.5 21.5-48 48-48h288c26.5 0 48 21.5 48 48zM384 176v288c0 26.5-21.5 48-48 48H48c-26.5 0-48-21.5-48-48V176c0-26.5 21.5-48 48-48h288c26.5 0 48 21.5 48 48zm-68 28c0-6.6-5.4-12-12-12H76c-6.6 0-12 5.4-12 12v52h252v-52z")
		jot.FatalIfErr(err)
	}
	return windowRestoreSVG
}
