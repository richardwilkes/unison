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
	_ "embed"
	"io"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/strokecap"
	"github.com/richardwilkes/unison/enums/strokejoin"
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

	//go:embed resources/images/markdown_caution.svg
	markdownCautionSVG string
	MarkdownCautionSVG = MustSVGFromContentString(markdownCautionSVG)

	//go:embed resources/images/markdown_important.svg
	markdownImportantSVG string
	MarkdownImportantSVG = MustSVGFromContentString(markdownImportantSVG)

	//go:embed resources/images/markdown_note.svg
	markdownNoteSVG string
	MarkdownNoteSVG = MustSVGFromContentString(markdownNoteSVG)

	//go:embed resources/images/markdown_tip.svg
	markdownTipSVG string
	MarkdownTipSVG = MustSVGFromContentString(markdownTipSVG)

	//go:embed resources/images/markdown_warning.svg
	markdownWarningSVG string
	MarkdownWarningSVG = MustSVGFromContentString(markdownWarningSVG)

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
	SVG             *SVG
	Size            geom.Size
	RotationDegrees float32
}

// SVG holds an SVG.
type SVG struct {
	paths         []*svgPath
	viewBox       geom.Rect
	suggestedSize geom.Size
}

type svgPath struct {
	path        *Path
	fillInk     Ink
	strokeInk   Ink
	dash        *PathEffect
	strokeMiter float32
	strokeWidth float32
	strokeCap   strokecap.Enum
	strokeJoin  strokejoin.Enum
}

// MustSVGFromContentString creates a new SVG and panics if an error would be generated. The content should contain
// valid SVG file data.
func MustSVGFromContentString(content string) *SVG {
	s, err := NewSVGFromContentString(content)
	xos.ExitIfErr(err)
	return s
}

// NewSVGFromContentString creates a new SVG. The content should contain valid SVG file data.
func NewSVGFromContentString(content string) (*SVG, error) {
	return NewSVGFromReader(strings.NewReader(content))
}

// MustSVGFromReader creates a new SVG and panics if an error would be generated. The reader should contain valid SVG
// file data.
func MustSVGFromReader(r io.Reader) *SVG {
	s, err := NewSVGFromReader(r)
	xos.ExitIfErr(err)
	return s
}

// NewSVGFromReader creates a new SVG. The reader should contain valid SVG file data.
func NewSVGFromReader(r io.Reader) (*SVG, error) {
	s, err := parseSVG(r)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return s, nil
}

// Size returns the original (viewBox) size.
func (s *SVG) Size() geom.Size {
	return s.viewBox.Size
}

// SuggestedSize returns the suggested size, if one was specified in the SVG file via the width and height parameters.
// If no suggested size was specified, then the original (viewBox) size is returned.
func (s *SVG) SuggestedSize() geom.Size {
	if s.suggestedSize.Width == 0 || s.suggestedSize.Height == 0 {
		return s.Size()
	}
	return s.suggestedSize
}

// OffsetToCenterWithinScaledSize returns the scaled offset values to use to keep the image centered within the given
// size.
func (s *SVG) OffsetToCenterWithinScaledSize(size geom.Size) geom.Point {
	scale := min(size.Width/s.viewBox.Width, size.Height/s.viewBox.Height)
	return geom.NewPoint((size.Width-s.viewBox.Width*scale)/2, (size.Height-s.viewBox.Height*scale)/2)
}

// AspectRatio returns the SVG's width to height ratio.
func (s *SVG) AspectRatio() float32 {
	return s.viewBox.Width / s.viewBox.Height
}

// DrawInRect draws this SVG resized to fit in the given rectangle. If paint is not nil, the SVG paths will be drawn
// with the provided paint, ignoring any fill or stroke attributes within the source SVG. Be sure to set the Paint's
// style (fill or stroke) as desired.
func (s *SVG) DrawInRect(canvas *Canvas, rect geom.Rect, _ *SamplingOptions, paint *Paint) {
	canvas.Save()
	defer canvas.Restore()
	offset := s.OffsetToCenterWithinScaledSize(rect.Size)
	canvas.Translate(rect.Point.Add(offset))
	canvas.Scale(geom.PointFromSize(rect.Size.DivSize(s.viewBox.Size)))
	for _, path := range s.paths {
		if paint == nil {
			if path.fillInk != nil {
				canvas.DrawPath(path.path, path.fillInk.Paint(canvas, s.viewBox, paintstyle.Fill))
			}
			if path.strokeInk != nil {
				p := path.strokeInk.Paint(canvas, s.viewBox, paintstyle.Stroke)
				p.SetStrokeCap(path.strokeCap)
				p.SetStrokeJoin(path.strokeJoin)
				p.SetStrokeMiter(path.strokeMiter)
				p.SetStrokeWidth(path.strokeWidth)
				if path.dash != nil {
					p.SetPathEffect(path.dash)
				}
				canvas.DrawPath(path.path, p)
			}
		} else {
			canvas.DrawPath(path.path, paint)
		}
	}
}

// DrawInRectPreservingAspectRatio draws this SVG resized to fit in the given rectangle, preserving the aspect ratio.
// If paint is not nil, the SVG paths will be drawn with the provided paint, ignoring any fill or stroke attributes
// within the source SVG. Be sure to set the Paint's style (fill or stroke) as desired.
func (s *SVG) DrawInRectPreservingAspectRatio(canvas *Canvas, rect geom.Rect, opts *SamplingOptions, paint *Paint) {
	ratio := s.AspectRatio()
	w := rect.Width
	h := w / ratio
	if h > rect.Height {
		h = rect.Height
		w = h * ratio
	}
	s.DrawInRect(canvas, geom.NewRect(rect.X+(rect.Width-w)/2, rect.Y+(rect.Height-h)/2, w, h), opts, paint)
}

// LogicalSize implements the Drawable interface.
func (s *DrawableSVG) LogicalSize() geom.Size {
	return s.Size
}

// DrawInRect implements the Drawable interface.
//
// If paint is not nil, the SVG paths will be drawn with the provided paint, ignoring any fill or stroke attributes
// within the source SVG. Be sure to set the Paint's style (fill or stroke) as desired.
func (s *DrawableSVG) DrawInRect(canvas *Canvas, rect geom.Rect, opts *SamplingOptions, paint *Paint) {
	if s.RotationDegrees != 0 {
		canvas.Save()
		defer canvas.Restore()
		center := rect.Center()
		canvas.Translate(center)
		canvas.Rotate(s.RotationDegrees)
		canvas.Translate(center.Neg())
	}
	s.SVG.DrawInRectPreservingAspectRatio(canvas, rect, opts, paint)
}
