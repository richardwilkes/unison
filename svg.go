// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	_ "embed"
	"io"
	"strings"

	"github.com/lafriks/go-svg"
	"github.com/richardwilkes/toolbox/fatal"
)

var _ Drawable = &DrawableSVG{}

// Pre-defined SVG images used by Unison.
var (
	//go:embed resources/images/broken_image.svg
	brokenImageSVG string
	BrokenImageSVG = MustSVGFromContentString(brokenImageSVG)

	//go:embed resources/images/circled_chevron_right.svg
	circledChevronRightSVG string
	CircledChevronRightSVG = MustSVGFromContentString(circledChevronRightSVG)

	//go:embed resources/images/circled_exclamation.svg
	circledExclamationSVG string
	CircledExclamationSVG = MustSVGFromContentString(circledExclamationSVG)

	//go:embed resources/images/circled_question.svg
	circledQuestionSVG string
	CircledQuestionSVG = MustSVGFromContentString(circledQuestionSVG)

	//go:embed resources/images/checkmark.svg
	checkmarkSVG string
	CheckmarkSVG = MustSVGFromContentString(checkmarkSVG)

	//go:embed resources/images/chevron_right.svg
	chevronRightSVG string
	ChevronRightSVG = MustSVGFromContentString(chevronRightSVG)

	//go:embed resources/images/circled_x.svg
	circledXSVG string
	CircledXSVG = MustSVGFromContentString(circledXSVG)

	//go:embed resources/images/dash.svg
	dashSVG string
	DashSVG = MustSVGFromContentString(dashSVG)

	//go:embed resources/images/document.svg
	documentSVG string
	DocumentSVG = MustSVGFromContentString(documentSVG)

	//go:embed resources/images/sort_ascending.svg
	sortAscendingSVG string
	SortAscendingSVG = MustSVGFromContentString(sortAscendingSVG)

	//go:embed resources/images/sort_descending.svg
	sortDescendingSVG string
	SortDescendingSVG = MustSVGFromContentString(sortDescendingSVG)

	//go:embed resources/images/triangle_exclamation.svg
	triangleExclamationSVG string
	TriangleExclamationSVG = MustSVGFromContentString(triangleExclamationSVG)

	//go:embed resources/images/window_maximize.svg
	windowMaximizeSVG string
	WindowMaximizeSVG = MustSVGFromContentString(windowMaximizeSVG)

	//go:embed resources/images/window_restore.svg
	windowRestoreSVG string
	WindowRestoreSVG = MustSVGFromContentString(windowRestoreSVG)
)

// DrawableSVG makes an SVG conform to the Drawable interface.
type DrawableSVG struct {
	SVG  *SVG
	Size Size
}

// SVG holds an SVG.
type SVG struct {
	paths               []*Path
	combinedPath        *Path
	scaledCombinedPaths map[Size]*Path
	size                Size
}

// MustSVG creates a new SVG the given svg path string (the contents of a single "d" attribute from an SVG "path"
// element) and panics if an error would be generated. The 'size' should be gotten from the original SVG's 'viewBox'
// parameter.
func MustSVG(size Size, svg string) *SVG {
	s, err := NewSVG(size, svg)
	fatal.IfErr(err)
	return s
}

// NewSVG creates a new SVG the given svg path string (the contents of a single "d" attribute from an SVG "path"
// element). The 'size' should be gotten from the original SVG's 'viewBox' parameter.
func NewSVG(size Size, svg string) (*SVG, error) {
	path, err := NewPathFromSVGString(svg)
	if err != nil {
		return nil, err
	}
	return &SVG{
		size:                size,
		paths:               []*Path{path},
		scaledCombinedPaths: make(map[Size]*Path),
	}, nil
}

// MustSVGFromContentString creates a new SVG and panics if an error would be generated. The content should contain
// valid SVG file data. Note that this only reads a very small subset of an SVG currently. Specifically, the "viewBox"
// attribute and any "d" attributes from enclosed SVG "path" elements.
func MustSVGFromContentString(content string) *SVG {
	s, err := NewSVGFromContentString(content)
	fatal.IfErr(err)
	return s
}

// NewSVGFromContentString creates a new SVG. The content should contain valid SVG file data. Note that this only reads
// a very small subset of an SVG currently. Specifically, the "viewBox" attribute and any "d" attributes from enclosed
// SVG "path" elements.
func NewSVGFromContentString(content string) (*SVG, error) {
	return NewSVGFromReader(strings.NewReader(content))
}

// MustSVGFromReader creates a new SVG and panics if an error would be generated. The reader should contain valid SVG
// file data. Note that this only reads a very small subset of an SVG currently. Specifically, the "viewBox" attribute
// and any "d" attributes from enclosed SVG "path" elements.
func MustSVGFromReader(r io.Reader) *SVG {
	s, err := NewSVGFromReader(r)
	fatal.IfErr(err)
	return s
}

// NewSVGFromReader creates a new SVG. The reader should contain valid SVG file data. Note that this only reads a very
// small subset of an SVG currently. Specifically, the "viewBox" attribute and any "d" attributes from enclosed SVG
// "path" elements.
func NewSVGFromReader(r io.Reader) (*SVG, error) {
	svg, err := svg.Parse(r, svg.StrictErrorMode)
	if err != nil {
		return nil, err
	}

	s := &SVG{
		paths:               make([]*Path, len(svg.SvgPaths)),
		scaledCombinedPaths: make(map[Size]*Path),
		size: Size{
			Width:  float32(svg.ViewBox.W),
			Height: float32(svg.ViewBox.H),
		},
	}

	for i, path := range svg.SvgPaths {
		p, err := newPathFromSvgPath(path)
		if err != nil {
			return nil, err
		}
		s.paths[i] = p
	}

	return s, nil
}

// Size returns the original size.
func (s *SVG) Size() Size {
	return s.size
}

// CombinedPath combines all paths for an SVG in to a single path,
// by extending the first path will all of the other paths.
// The combined path will have the fill and stroke attributes of the first path.
func (s *SVG) CombinedPath() *Path {
	// Lazily create the combined path.
	if s.combinedPath == nil {
		for _, path := range s.paths {
			if s.combinedPath == nil {
				s.combinedPath = path.Clone()
			} else {
				s.combinedPath.Path(path, false)
			}
		}
	}

	return s.combinedPath
}

// OffsetToCenterWithinScaledSize returns the scaled offset values to use to keep the image centered within the given
// size.
func (s *SVG) OffsetToCenterWithinScaledSize(size Size) Point {
	scale := min(size.Width/s.size.Width, size.Height/s.size.Height)
	return Point{X: (size.Width - s.size.Width*scale) / 2, Y: (size.Height - s.size.Height*scale) / 2}
}

// PathScaledTo returns the path with the specified scaling. You should not modify this path, as it is cached.
//
// Deprecated: PathScaledTo and PathForSize are used for drawing a scaled SVG.
// This can be achieved with DrawableSVG#DrawInRect.
func (s *SVG) PathScaledTo(scale float32) *Path {
	if scale == 1 {
		return s.CombinedPath()
	}
	scaledSize := Size{Width: scale, Height: scale}
	p, ok := s.scaledCombinedPaths[scaledSize]
	if !ok {
		p = s.CombinedPath().NewScaled(scale, scale)
		s.scaledCombinedPaths[scaledSize] = p
	}
	return p
}

// PathForSize returns the path scaled to fit in the specified size. You should not modify this path, as it is cached.
//
// Deprecated: PathForSize and PathScaledTo are used for drawing a scaled SVG.
// This can be achieved with DrawableSVG#DrawInRect.
func (s *SVG) PathForSize(size Size) *Path {
	return s.PathScaledTo(min(size.Width/s.size.Width, size.Height/s.size.Height))
}

// AspectRatio returns the SVG's width to height ratio.
func (s *SVG) AspectRatio() float32 {
	return s.size.Width / s.size.Height
}

// LogicalSize implements the Drawable interface.
func (s *DrawableSVG) LogicalSize() Size {
	return s.Size
}

// DrawInRect implements the Drawable interface.
//
// If paint is not nil the SVG paths will be drawn with the provided paint.
// Be sure to set the Paint's style (fill or stroke) as desired.
// Any fill or stroke attributes in the SVG source will be ignored.
// This is for backwards compatabality with an earlier SVG implementation.
func (s *DrawableSVG) DrawInRect(canvas *Canvas, rect Rect, _ *SamplingOptions, paint *Paint) {
	canvas.Save()
	defer canvas.Restore()

	offset := s.SVG.OffsetToCenterWithinScaledSize(rect.Size)
	canvas.Translate(rect.X+offset.X, rect.Y+offset.Y)
	canvas.Scale(rect.Width/s.SVG.size.Width, rect.Height/s.SVG.size.Height)

	for _, path := range s.SVG.paths {
		if paint == nil {
			if path.fillPaint != nil {
				canvas.DrawPath(path, path.fillPaint)
			}
			if path.strokePaint != nil {
				canvas.DrawPath(path, path.strokePaint)
			}
		} else {
			canvas.DrawPath(path, paint)
		}
	}
}
