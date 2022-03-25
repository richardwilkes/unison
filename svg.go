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
	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/toolbox/xmath/geom"
)

var (
	_                      Drawable = &DrawableSVG{}
	chevronRightSVG        *SVG
	circledChevronRightSVG *SVG
	circledExclamationSVG  *SVG
	circledQuestionSVG     *SVG
	circledXSVG            *SVG
	documentSVG            *SVG
	sortAscendingSVG       *SVG
	sortDescendingSVG      *SVG
	triangleExclamationSVG *SVG
	windowMaximizeSVG      *SVG
	windowRestoreSVG       *SVG
)

// DrawableSVG makes an SVG conform to the Drawable interface.
type DrawableSVG struct {
	SVG  *SVG
	Size geom.Size[float32]
}

// SVG holds an SVG path. Note that this is a subset of SVG: just the 'd' attribute of the 'path' directive.
type SVG struct {
	size          geom.Size[float32]
	unscaledPath  *Path
	scaledPathMap map[geom.Size[float32]]*Path
}

// NewSVG creates a new SVG. The 'size' should be gotten from the original SVG's 'viewBox' parameter.
func NewSVG(size geom.Size[float32], svg string) (*SVG, error) {
	unscaledPath, err := NewPathFromSVGString(svg)
	if err != nil {
		return nil, err
	}
	return &SVG{
		size:          size,
		unscaledPath:  unscaledPath,
		scaledPathMap: make(map[geom.Size[float32]]*Path),
	}, nil
}

// Size returns the original size.
func (s *SVG) Size() geom.Size[float32] {
	return s.size
}

// OffsetToCenterWithinScaledSize returns the scaled offset values to use to keep the image centered within the given
// size.
func (s *SVG) OffsetToCenterWithinScaledSize(size geom.Size[float32]) geom.Point[float32] {
	scale := xmath.Min(size.Width/s.size.Width, size.Height/s.size.Height)
	return geom.NewPoint((size.Width-s.size.Width*scale)/2, (size.Height-s.size.Height*scale)/2)
}

// PathScaledTo returns the path with the specified scaling. You should not modify this path, as it is cached.
func (s *SVG) PathScaledTo(scale float32) *Path {
	if scale == 1 {
		return s.unscaledPath
	}
	scaledSize := geom.NewSize[float32](scale, scale)
	p, ok := s.scaledPathMap[scaledSize]
	if !ok {
		p = s.unscaledPath.NewScaled(scale, scale)
		s.scaledPathMap[scaledSize] = p
	}
	return p
}

// PathForSize returns the path scaled to fit in the specified size. You should not modify this path, as it is cached.
func (s *SVG) PathForSize(size geom.Size[float32]) *Path {
	return s.PathScaledTo(xmath.Min(size.Width/s.size.Width, size.Height/s.size.Height))
}

// LogicalSize implements the Drawable interface.
func (s *DrawableSVG) LogicalSize() geom.Size[float32] {
	return s.Size
}

// DrawInRect implements the Drawable interface.
func (s *DrawableSVG) DrawInRect(canvas *Canvas, rect geom.Rect[float32], _ *SamplingOptions, paint *Paint) {
	canvas.Save()
	defer canvas.Restore()
	offset := s.SVG.OffsetToCenterWithinScaledSize(rect.Size)
	canvas.Translate(rect.X+offset.X, rect.Y+offset.Y)
	canvas.DrawPath(s.SVG.PathForSize(rect.Size), paint)
}

// ChevronRightSVG returns an SVG that holds a chevron pointing towards the right.
func ChevronRightSVG() *SVG {
	if chevronRightSVG == nil {
		var err error
		chevronRightSVG, err = NewSVG(geom.NewSize[float32](320, 512), "M285.476 272.971 91.132 467.314c-9.373 9.373-24.569 9.373-33.941 0l-22.667-22.667c-9.357-9.357-9.375-24.522-.04-33.901L188.505 256 34.484 101.255c-9.335-9.379-9.317-24.544.04-33.901l22.667-22.667c9.373-9.373 24.569-9.373 33.941 0L285.475 239.03c9.373 9.372 9.373 24.568.001 33.941z")
		jot.FatalIfErr(err)
	}
	return chevronRightSVG
}

// CircledChevronRightSVG returns an SVG that holds a circled chevron pointing towards the right.
func CircledChevronRightSVG() *SVG {
	if circledChevronRightSVG == nil {
		var err error
		circledChevronRightSVG, err = NewSVG(geom.NewSize[float32](512, 512), "M256 8c137 0 248 111 248 248S393 504 256 504 8 393 8 256 119 8 256 8zm113.9 231L234.4 103.5c-9.4-9.4-24.6-9.4-33.9 0l-17 17c-9.4 9.4-9.4 24.6 0 33.9L285.1 256 183.5 357.6c-9.4 9.4-9.4 24.6 0 33.9l17 17c9.4 9.4 24.6 9.4 33.9 0L369.9 273c9.4-9.4 9.4-24.6 0-34z")
		jot.FatalIfErr(err)
	}
	return circledChevronRightSVG
}

// CircledExclamationSVG returns an SVG that holds a circled exclamation mark.
func CircledExclamationSVG() *SVG {
	if circledExclamationSVG == nil {
		var err error
		circledExclamationSVG, err = NewSVG(geom.NewSize[float32](512, 512), "M504 256c0 136.997-111.043 248-248 248S8 392.997 8 256C8 119.083 119.043 8 256 8s248 111.083 248 248zm-248 50c-25.405 0-46 20.595-46 46s20.595 46 46 46 46-20.595 46-46-20.595-46-46-46zm-43.673-165.346 7.418 136c.347 6.364 5.609 11.346 11.982 11.346h48.546c6.373 0 11.635-4.982 11.982-11.346l7.418-136c.375-6.874-5.098-12.654-11.982-12.654h-63.383c-6.884 0-12.356 5.78-11.981 12.654z")
		jot.FatalIfErr(err)
	}
	return circledExclamationSVG
}

// CircledQuestionSVG returns an SVG that holds a circled question mark.
func CircledQuestionSVG() *SVG {
	if circledQuestionSVG == nil {
		var err error
		circledQuestionSVG, err = NewSVG(geom.NewSize[float32](512, 512), "M504 256c0 136.997-111.043 248-248 248S8 392.997 8 256C8 119.083 119.043 8 256 8s248 111.083 248 248zM262.655 90c-54.497 0-89.255 22.957-116.549 63.758-3.536 5.286-2.353 12.415 2.715 16.258l34.699 26.31c5.205 3.947 12.621 3.008 16.665-2.122 17.864-22.658 30.113-35.797 57.303-35.797 20.429 0 45.698 13.148 45.698 32.958 0 14.976-12.363 22.667-32.534 33.976C247.128 238.528 216 254.941 216 296v4c0 6.627 5.373 12 12 12h56c6.627 0 12-5.373 12-12v-1.333c0-28.462 83.186-29.647 83.186-106.667 0-58.002-60.165-102-116.531-102zM256 338c-25.365 0-46 20.635-46 46 0 25.364 20.635 46 46 46s46-20.636 46-46c0-25.365-20.635-46-46-46z")
		jot.FatalIfErr(err)
	}
	return circledQuestionSVG
}

// CircledXSVG returns an SVG that holds an icon for closing content.
func CircledXSVG() *SVG {
	if circledXSVG == nil {
		var err error
		circledXSVG, err = NewSVG(geom.NewSize[float32](512, 512), "M256 8C119 8 8 119 8 256s111 248 248 248 248-111 248-248S393 8 256 8zm121.6 313.1c4.7 4.7 4.7 12.3 0 17L338 377.6c-4.7 4.7-12.3 4.7-17 0L256 312l-65.1 65.6c-4.7 4.7-12.3 4.7-17 0L134.4 338c-4.7-4.7-4.7-12.3 0-17l65.6-65-65.6-65.1c-4.7-4.7-4.7-12.3 0-17l39.6-39.6c4.7-4.7 12.3-4.7 17 0l65 65.7 65.1-65.6c4.7-4.7 12.3-4.7 17 0l39.6 39.6c4.7 4.7 4.7 12.3 0 17L312 256l65.6 65.1z")
		jot.FatalIfErr(err)
	}
	return circledXSVG
}

// DocumentSVG returns an SVG that holds an icon for a document.
func DocumentSVG() *SVG {
	if documentSVG == nil {
		var err error
		documentSVG, err = NewSVG(geom.NewSize[float32](384, 512), "M224 136V0H24C10.7 0 0 10.7 0 24v464c0 13.3 10.7 24 24 24h336c13.3 0 24-10.7 24-24V160H248c-13.2 0-24-10.8-24-24zm160-14.1v6.1H256V0h6.1c6.4 0 12.5 2.5 17 7l97.9 98c4.5 4.5 7 10.6 7 16.9z")
		jot.FatalIfErr(err)
	}
	return documentSVG
}

// SortAscendingSVG returns an SVG that holds an icon for an ascending sort.
func SortAscendingSVG() *SVG {
	if sortAscendingSVG == nil {
		var err error
		sortAscendingSVG, err = NewSVG(geom.NewSize[float32](512, 512), "M240 96h64a16 16 0 0 0 16-16V48a16 16 0 0 0-16-16h-64a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16zm0 128h128a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16zm256 192H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h256a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16zm-256-64h192a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16zm-64 0h-48V48a16 16 0 0 0-16-16H80a16 16 0 0 0-16 16v304H16c-14.19 0-21.37 17.24-11.29 27.31l80 96a16 16 0 0 0 22.62 0l80-96C197.35 369.26 190.22 352 176 352z")
		jot.FatalIfErr(err)
	}
	return sortAscendingSVG
}

// SortDescendingSVG returns an SVG that holds an icon for an descending sort.
func SortDescendingSVG() *SVG {
	if sortDescendingSVG == nil {
		var err error
		sortDescendingSVG, err = NewSVG(geom.NewSize[float32](512, 512), "M304 416h-64a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h64a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16zM16 160h48v304a16 16 0 0 0 16 16h32a16 16 0 0 0 16-16V160h48c14.21 0 21.38-17.24 11.31-27.31l-80-96a16 16 0 0 0-22.62 0l-80 96C-5.35 142.74 1.77 160 16 160zm416 0H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h192a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16zm-64 128H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h128a16 16 0 0 0 16-16v-32a16 16 0 0 0-16-16zM496 32H240a16 16 0 0 0-16 16v32a16 16 0 0 0 16 16h256a16 16 0 0 0 16-16V48a16 16 0 0 0-16-16z")
		jot.FatalIfErr(err)
	}
	return sortDescendingSVG
}

// TriangleExclamationSVG returns an SVG that holds an exclamation mark inside a triangle.
func TriangleExclamationSVG() *SVG {
	if triangleExclamationSVG == nil {
		var err error
		triangleExclamationSVG, err = NewSVG(geom.NewSize[float32](576, 512), "M569.517 440.013C587.975 472.007 564.806 512 527.94 512H48.054c-36.937 0-59.999-40.055-41.577-71.987L246.423 23.985c18.467-32.009 64.72-31.951 83.154 0l239.94 416.028zM288 354c-25.405 0-46 20.595-46 46s20.595 46 46 46 46-20.595 46-46-20.595-46-46-46zm-43.673-165.346 7.418 136c.347 6.364 5.609 11.346 11.982 11.346h48.546c6.373 0 11.635-4.982 11.982-11.346l7.418-136c.375-6.874-5.098-12.654-11.982-12.654h-63.383c-6.884 0-12.356 5.78-11.981 12.654z")
		jot.FatalIfErr(err)
	}
	return triangleExclamationSVG
}

// WindowMaximizeSVG returns an SVG that holds an icon for maximizing a window.
func WindowMaximizeSVG() *SVG {
	if windowMaximizeSVG == nil {
		var err error
		windowMaximizeSVG, err = NewSVG(geom.NewSize[float32](512, 512), "M464 32H48C21.5 32 0 53.5 0 80v352c0 26.5 21.5 48 48 48h416c26.5 0 48-21.5 48-48V80c0-26.5-21.5-48-48-48zm-16 160H64v-84c0-6.6 5.4-12 12-12h360c6.6 0 12 5.4 12 12v84z")
		jot.FatalIfErr(err)
	}
	return windowMaximizeSVG
}

// WindowRestoreSVG returns an SVG that holds an icon for restoring a maximized window.
func WindowRestoreSVG() *SVG {
	if windowRestoreSVG == nil {
		var err error
		windowRestoreSVG, err = NewSVG(geom.NewSize[float32](512, 512), "M512 48v288c0 26.5-21.5 48-48 48h-48V176c0-44.1-35.9-80-80-80H128V48c0-26.5 21.5-48 48-48h288c26.5 0 48 21.5 48 48zM384 176v288c0 26.5-21.5 48-48 48H48c-26.5 0-48-21.5-48-48V176c0-26.5 21.5-48 48-48h288c26.5 0 48 21.5 48 48zm-68 28c0-6.6-5.4-12-12-12H76c-6.6 0-12 5.4-12 12v52h252v-52z")
		jot.FatalIfErr(err)
	}
	return windowRestoreSVG
}
