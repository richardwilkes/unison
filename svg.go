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
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/enums/filltype"
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

var strokeCaps = map[SVGCapMode]strokecap.Enum{
	SVGNilCap:    strokecap.Butt,
	SVGButtCap:   strokecap.Butt,
	SVGSquareCap: strokecap.Square,
	SVGRoundCap:  strokecap.Round,
}

var strokeJoins = map[SVGJoinMode]strokejoin.Enum{
	SVGRound: strokejoin.Round,
	SVGBevel: strokejoin.Bevel,
	SVGMiter: strokejoin.Miter,
}

// DrawableSVG makes an SVG conform to the Drawable interface.
type DrawableSVG struct {
	SVG             *SVG
	Size            geom.Size
	RotationDegrees float32
}

// SVG holds an SVG.
type SVG struct {
	paths         []*svgPath
	size          geom.Size
	suggestedSize geom.Size
}

type svgPath struct {
	*Path
	fillPaint   *Paint
	strokePaint *Paint
}

// MustSVG creates a new SVG the given svg path string (the contents of a single "d" attribute from an SVG "path"
// element) and panics if an error would be generated. The 'size' should be gotten from the original SVG's 'viewBox'
// parameter.
//
// Note: It is probably better to use one of the other Must... methods that take the full SVG content.
func MustSVG(size geom.Size, svgPathStr string) *SVG {
	s, err := NewSVG(size, svgPathStr)
	xos.ExitIfErr(err)
	return s
}

// NewSVG creates a new SVG the given svg path string (the contents of a single "d" attribute from an SVG "path"
// element). The 'size' should be gotten from the original SVG's 'viewBox' parameter.
//
// Note: It is probably better to use one of the other New... methods that take the full SVG content.
func NewSVG(size geom.Size, svgPathStr string) (*SVG, error) {
	path, err := NewPathFromSVGString(svgPathStr)
	if err != nil {
		return nil, err
	}
	return &SVG{
		paths: []*svgPath{{Path: path}},
		size:  size,
	}, nil
}

// MustSVGFromContentString creates a new SVG and panics if an error would be generated. The content should contain
// valid SVG file data. Note that this only reads a very small subset of an SVG currently. Specifically, the "viewBox"
// attribute and any "d" attributes from enclosed SVG "path" elements.
func MustSVGFromContentString(content string) *SVG {
	s, err := NewSVGFromContentString(content)
	xos.ExitIfErr(err)
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
	xos.ExitIfErr(err)
	return s
}

// NewSVGFromReader creates a new SVG. The reader should contain valid SVG file data. Note that this only reads a very
// small subset of an SVG currently. Specifically, the "viewBox" attribute and any "d" attributes from enclosed SVG
// "path" elements.
func NewSVGFromReader(r io.Reader) (*SVG, error) {
	s := &SVG{}

	sData, err := SVGParse(r)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	s.size = sData.ViewBox.Size
	s.suggestedSize = sData.SuggestedSize
	s.paths = make([]*svgPath, len(sData.Paths))
	for i := range sData.Paths {
		p := NewPath()
		if sData.Paths[i].Style.UseNonZeroWinding {
			p.SetFillType(filltype.Winding)
		} else {
			p.SetFillType(filltype.EvenOdd)
		}
		for _, op := range sData.Paths[i].Path {
			// The coordinates used in SVGOp are of type fixed.Int26_6, which has a fractional part of 6 bits.
			// When converting to a float, the values are divided by 64.
			switch op := op.(type) {
			case SVGOpMoveTo:
				p.MoveTo(geom.NewPoint(float32(op.X)/64, float32(op.Y)/64))
			case SVGOpLineTo:
				p.LineTo(geom.NewPoint(float32(op.X)/64, float32(op.Y)/64))
			case SVGOpQuadTo:
				p.QuadTo(
					geom.NewPoint(float32(op[0].X)/64, float32(op[0].Y)/64),
					geom.NewPoint(float32(op[1].X)/64, float32(op[1].Y)/64),
				)
			case SVGOpCubicTo:
				p.CubicTo(
					geom.NewPoint(float32(op[0].X)/64, float32(op[0].Y)/64),
					geom.NewPoint(float32(op[1].X)/64, float32(op[1].Y)/64),
					geom.NewPoint(float32(op[2].X)/64, float32(op[2].Y)/64),
				)
			case SVGOpClose:
				p.Close()
			}
		}

		if !sData.Paths[i].Style.Transform.IsIdentity() {
			p.Transform(sData.Paths[i].Style.Transform)
		}
		sp := &svgPath{Path: p}

		if sData.Paths[i].Style.FillerColor != nil && sData.Paths[i].Style.FillOpacity != 0 {
			if sp.fillPaint, err = createPaintFromSVGPattern(paintstyle.Fill, sData.Paths[i].Style.FillerColor,
				sData.Paths[i].Style.FillOpacity); err != nil {
				return nil, err
			}
		}

		if sData.Paths[i].Style.LinerColor != nil && sData.Paths[i].Style.LineOpacity != 0 &&
			sData.Paths[i].Style.LineWidth != 0 {
			if sp.strokePaint, err = createPaintFromSVGPattern(paintstyle.Stroke, sData.Paths[i].Style.LinerColor,
				sData.Paths[i].Style.LineOpacity); err != nil {
				return nil, err
			}
			if strokeCap, ok := strokeCaps[sData.Paths[i].Style.Join.TrailLineCap]; !ok {
				slog.Warn("svg: unsupported path stroke cap", "cap", sData.Paths[i].Style.Join.TrailLineCap)
				sp.strokePaint.SetStrokeCap(strokecap.Butt)
			} else {
				sp.strokePaint.SetStrokeCap(strokeCap)
			}
			if strokeJoin, ok := strokeJoins[sData.Paths[i].Style.Join.LineJoin]; !ok {
				slog.Warn("svg: unsupported path stroke join", "join", sData.Paths[i].Style.Join.LineJoin)
				sp.strokePaint.SetStrokeJoin(strokejoin.Round)
			} else {
				sp.strokePaint.SetStrokeJoin(strokeJoin)
			}
			sp.strokePaint.SetStrokeMiter(float32(sData.Paths[i].Style.Join.MiterLimit))
			sp.strokePaint.SetStrokeWidth(sData.Paths[i].Style.LineWidth)
		}

		s.paths[i] = sp
	}
	return s, nil
}

func createPaintFromSVGPattern(kind paintstyle.Enum, pattern SVGPattern, opacity float32) (*Paint, error) {
	if c, ok := pattern.(SVGPlainColor); ok {
		return convertSVGColor(c, opacity).Paint(nil, geom.Rect{}, kind), nil
	}
	if gr, ok := pattern.(*SVGGradient); ok {
		grad := Gradient{
			Stops: make([]Stop, len(gr.Stops)),
		}
		for i, stop := range gr.Stops {
			grad.Stops[i] = Stop{
				Color:    convertSVGColor(stop.StopColor, stop.Opacity),
				Location: stop.Offset,
			}
		}
		switch d := gr.Direction.(type) {
		case SVGLinear:
			grad.Start.X = d[0]
			grad.Start.Y = d[1]
			grad.End.X = d[2]
			grad.End.Y = d[3]
		case SVGRadial:
			grad.Start.X = d[0]
			grad.Start.Y = d[1]
			grad.End.X = d[2]
			grad.End.Y = d[3]
			grad.StartRadius = d[4]
			grad.EndRadius = d[5]
		default:
			return nil, errs.Newf("unsupported %s gradient direction %T", kind.Key(), gr.Direction)
		}
		grad.Start.X = (grad.Start.X - gr.Bounds.X) / gr.Bounds.Width
		grad.Start.Y = (grad.Start.Y - gr.Bounds.Y) / gr.Bounds.Height
		grad.End.X = (grad.End.X - gr.Bounds.X) / gr.Bounds.Width
		grad.End.Y = (grad.End.Y - gr.Bounds.Y) / gr.Bounds.Height
		return grad.Paint(nil, gr.Bounds, kind), nil
	}
	slog.Warn("svg: unsupported style", "pattern", fmt.Sprintf("%T", pattern))
	return Black.Paint(nil, geom.Rect{}, kind), nil
}

func convertSVGColor(c color.Color, opacity float32) Color {
	if c == nil {
		return 0
	}
	if plain, ok := c.(SVGPlainColor); ok {
		alpha := uint8(float32(plain.A) * opacity)
		return ColorFromNRGBA(color.NRGBA{A: alpha, R: plain.R, G: plain.G, B: plain.B})
	}
	r, g, b, a := c.RGBA()
	return ARGBfloat((float32(a)/65535)*opacity, float32(r)/65535, float32(g)/65535, float32(b)/65535)
}

// Size returns the original (viewBox) size.
func (s *SVG) Size() geom.Size {
	return s.size
}

// SuggestedSize returns the suggested size, if one was specified in the SVG file via the width and height parameters.
// If no suggested size was specified, then the original (viewBox) size is returned.
func (s *SVG) SuggestedSize() geom.Size {
	if s.suggestedSize.Width == 0 || s.suggestedSize.Height == 0 {
		return s.size
	}
	return s.suggestedSize
}

// OffsetToCenterWithinScaledSize returns the scaled offset values to use to keep the image centered within the given
// size.
func (s *SVG) OffsetToCenterWithinScaledSize(size geom.Size) geom.Point {
	scale := min(size.Width/s.size.Width, size.Height/s.size.Height)
	return geom.NewPoint((size.Width-s.size.Width*scale)/2, (size.Height-s.size.Height*scale)/2)
}

// AspectRatio returns the SVG's width to height ratio.
func (s *SVG) AspectRatio() float32 {
	return s.size.Width / s.size.Height
}

// DrawInRect draws this SVG resized to fit in the given rectangle. If paint is not nil, the SVG paths will be drawn
// with the provided paint, ignoring any fill or stroke attributes within the source SVG. Be sure to set the Paint's
// style (fill or stroke) as desired.
func (s *SVG) DrawInRect(canvas *Canvas, rect geom.Rect, _ *SamplingOptions, paint *Paint) {
	canvas.Save()
	defer canvas.Restore()
	offset := s.OffsetToCenterWithinScaledSize(rect.Size)
	canvas.Translate(rect.Point.Add(offset))
	canvas.Scale(geom.PointFromSize(rect.Size.DivSize(s.size)))
	for _, path := range s.paths {
		if paint == nil {
			if path.fillPaint != nil {
				canvas.DrawPath(path.Path, path.fillPaint)
			}
			if path.strokePaint != nil {
				canvas.DrawPath(path.Path, path.strokePaint)
			}
		} else {
			canvas.DrawPath(path.Path, paint)
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
