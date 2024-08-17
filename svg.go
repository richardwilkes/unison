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
	"image/color"
	"io"
	"strings"

	"github.com/lafriks/go-svg"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/fatal"
	"github.com/richardwilkes/toolbox/xmath/geom"
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

var strokeCaps = map[svg.CapMode]strokecap.Enum{
	svg.NilCap:    strokecap.Butt,
	svg.ButtCap:   strokecap.Butt,
	svg.SquareCap: strokecap.Square,
	svg.RoundCap:  strokecap.Round,
}

var strokeJoins = map[svg.JoinMode]strokejoin.Enum{
	svg.Round: strokejoin.Round,
	svg.Bevel: strokejoin.Bevel,
	svg.Miter: strokejoin.Miter,
}

// DrawableSVG makes an SVG conform to the Drawable interface.
type DrawableSVG struct {
	SVG  *SVG
	Size Size
}

// SVG holds an SVG.
type SVG struct {
	paths []*svgPath
	size  Size
}

type svgPath struct {
	*Path
	fillPaint   *Paint
	strokePaint *Paint
}

// SVGOption is an option that may be passed to SVG construction functions.
type SVGOption func(s *svgOptions) error

type svgOptions struct {
	parseErrorMode    svg.ErrorMode
	ignoreUnsupported bool
}

// SVGOptionIgnoreParseErrors is an option that will ignore some errors when parsing an SVG.
// If the XML is not well formed an error will still be generated.
func SVGOptionIgnoreParseErrors() SVGOption {
	return func(opts *svgOptions) error {
		opts.parseErrorMode = svg.IgnoreErrorMode
		return nil
	}
}

// SVGOptionWarnParseErrors is an option that will issue warnings to the log for some errors when parsing an SVG.
// If the XML is not well formed an error will still be generated.
func SVGOptionWarnParseErrors() SVGOption {
	return func(opts *svgOptions) error {
		opts.parseErrorMode = svg.WarnErrorMode
		return nil
	}
}

// SVGOptionIgnoreUnsupported is an option that will ignore unsupported SVG features that might be encountered
// in an SVG.
//
// If this option is not present, then unsupported features will result in an error when constructing the SVG.
func SVGOptionIgnoreUnsupported() SVGOption {
	return func(opts *svgOptions) error {
		opts.ignoreUnsupported = true
		return nil
	}
}

// MustSVG creates a new SVG the given svg path string (the contents of a single "d" attribute from an SVG "path"
// element) and panics if an error would be generated. The 'size' should be gotten from the original SVG's 'viewBox'
// parameter.
//
// Note: It is probably better to use one of the other Must... methods that take the full SVG content.
func MustSVG(size Size, svg string) *SVG {
	s, err := NewSVG(size, svg)
	fatal.IfErr(err)
	return s
}

// NewSVG creates a new SVG the given svg path string (the contents of a single "d" attribute from an SVG "path"
// element). The 'size' should be gotten from the original SVG's 'viewBox' parameter.
//
// Note: It is probably better to use one of the other New... methods that take the full SVG content.
func NewSVG(size Size, svg string) (*SVG, error) {
	path, err := NewPathFromSVGString(svg)
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
func MustSVGFromContentString(content string, options ...SVGOption) *SVG {
	s, err := NewSVGFromContentString(content, options...)
	fatal.IfErr(err)
	return s
}

// NewSVGFromContentString creates a new SVG. The content should contain valid SVG file data. Note that this only reads
// a very small subset of an SVG currently. Specifically, the "viewBox" attribute and any "d" attributes from enclosed
// SVG "path" elements.
func NewSVGFromContentString(content string, options ...SVGOption) (*SVG, error) {
	return NewSVGFromReader(strings.NewReader(content), options...)
}

// MustSVGFromReader creates a new SVG and panics if an error would be generated. The reader should contain valid SVG
// file data. Note that this only reads a very small subset of an SVG currently. Specifically, the "viewBox" attribute
// and any "d" attributes from enclosed SVG "path" elements.
func MustSVGFromReader(r io.Reader, options ...SVGOption) *SVG {
	s, err := NewSVGFromReader(r, options...)
	fatal.IfErr(err)
	return s
}

// NewSVGFromReader creates a new SVG. The reader should contain valid SVG file data. Note that this only reads a very
// small subset of an SVG currently. Specifically, the "viewBox" attribute and any "d" attributes from enclosed SVG
// "path" elements.
func NewSVGFromReader(r io.Reader, options ...SVGOption) (*SVG, error) {
	s := &SVG{}
	opts := svgOptions{parseErrorMode: svg.StrictErrorMode}
	for _, option := range options {
		if err := option(&opts); err != nil {
			return nil, err
		}
	}

	sData, err := svg.Parse(r, opts.parseErrorMode)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	s.size = NewSize(float32(sData.ViewBox.W), float32(sData.ViewBox.H))
	s.paths = make([]*svgPath, len(sData.SvgPaths))
	for i, path := range sData.SvgPaths {
		p := NewPath()
		for _, op := range path.Path {
			// The coordinates used in svg.Operation are of type fixed.Int26_6, which has a fractional part of 6 bits.
			// When converting to a float, the values are divided by 64.
			switch op := op.(type) {
			case svg.OpMoveTo:
				p.MoveTo(float32(op.X)/64, float32(op.Y)/64)
			case svg.OpLineTo:
				p.LineTo(float32(op.X)/64, float32(op.Y)/64)
			case svg.OpQuadTo:
				p.QuadTo(
					float32(op[0].X)/64, float32(op[0].Y)/64,
					float32(op[1].X)/64, float32(op[1].Y)/64,
				)
			case svg.OpCubicTo:
				p.CubicTo(
					float32(op[0].X)/64, float32(op[0].Y)/64,
					float32(op[1].X)/64, float32(op[1].Y)/64,
					float32(op[2].X)/64, float32(op[2].Y)/64,
				)
			case svg.OpClose:
				p.Close()
			}
		}

		if path.Style.Transform != svg.Identity {
			p.Transform(geom.Matrix[float32]{
				ScaleX: float32(path.Style.Transform.A),
				SkewX:  float32(path.Style.Transform.C),
				TransX: float32(path.Style.Transform.E),
				SkewY:  float32(path.Style.Transform.B),
				ScaleY: float32(path.Style.Transform.D),
				TransY: float32(path.Style.Transform.F),
			})
		}
		sp := &svgPath{Path: p}

		if path.Style.FillerColor != nil && path.Style.FillOpacity != 0 {
			sp.fillPaint = NewPaint()
			sp.fillPaint.SetStyle(paintstyle.Fill)
			if c, ok := path.Style.FillerColor.(svg.PlainColor); ok {
				alpha := uint8(float64(c.A) * path.Style.FillOpacity)
				sp.fillPaint.SetColor(ColorFromNRGBA(color.NRGBA{A: alpha, R: c.R, G: c.G, B: c.B}))
			} else if opts.ignoreUnsupported {
				sp.fillPaint.SetColor(Black)
			} else {
				return nil, errs.Newf("unsupported path fill style %T", path.Style.FillerColor)
			}
		}

		if path.Style.LinerColor != nil && path.Style.LineOpacity != 0 && path.Style.LineWidth != 0 {
			sp.strokePaint = NewPaint()
			if strokeCap, ok := strokeCaps[path.Style.Join.TrailLineCap]; !ok {
				if !opts.ignoreUnsupported {
					return nil, errs.Newf("unsupported path stroke cap %s", path.Style.Join.TrailLineCap)
				}
				sp.strokePaint.SetStrokeCap(strokecap.Butt)
			} else {
				sp.strokePaint.SetStrokeCap(strokeCap)
			}
			if strokeJoin, ok := strokeJoins[path.Style.Join.LineJoin]; !ok {
				if !opts.ignoreUnsupported {
					return nil, errs.Newf("unsupported path stroke join %s", path.Style.Join.LineJoin)
				}
				sp.strokePaint.SetStrokeJoin(strokejoin.Round)
			} else {
				sp.strokePaint.SetStrokeJoin(strokeJoin)
			}
			sp.strokePaint.SetStrokeMiter(float32(path.Style.Join.MiterLimit))
			sp.strokePaint.SetStrokeWidth(float32(path.Style.LineWidth))
			sp.strokePaint.SetStyle(paintstyle.Stroke)
			if c, ok := path.Style.LinerColor.(svg.PlainColor); ok {
				alpha := uint8(float64(c.A) * path.Style.FillOpacity)
				sp.strokePaint.SetColor(ColorFromNRGBA(color.NRGBA{A: alpha, R: c.R, G: c.G, B: c.B}))
			} else if opts.ignoreUnsupported {
				sp.fillPaint.SetColor(Black)
			} else {
				return nil, errs.Newf("unsupported path stroke style %T", path.Style.LinerColor)
			}
		}

		s.paths[i] = sp
	}
	return s, nil
}

// Size returns the original size.
func (s *SVG) Size() Size {
	return s.size
}

// OffsetToCenterWithinScaledSize returns the scaled offset values to use to keep the image centered within the given
// size.
func (s *SVG) OffsetToCenterWithinScaledSize(size Size) Point {
	scale := min(size.Width/s.size.Width, size.Height/s.size.Height)
	return Point{X: (size.Width - s.size.Width*scale) / 2, Y: (size.Height - s.size.Height*scale) / 2}
}

// AspectRatio returns the SVG's width to height ratio.
func (s *SVG) AspectRatio() float32 {
	return s.size.Width / s.size.Height
}

// DrawInRect draws this SVG resized to fit in the given rectangle. If paint is not nil, the SVG paths will be drawn
// with the provided paint, ignoring any fill or stroke attributes within the source SVG. Be sure to set the Paint's
// style (fill or stroke) as desired.
func (s *SVG) DrawInRect(canvas *Canvas, rect Rect, _ *SamplingOptions, paint *Paint) {
	canvas.Save()
	defer canvas.Restore()
	offset := s.OffsetToCenterWithinScaledSize(rect.Size)
	canvas.Translate(rect.X+offset.X, rect.Y+offset.Y)
	canvas.Scale(rect.Width/s.size.Width, rect.Height/s.size.Height)
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
func (s *SVG) DrawInRectPreservingAspectRatio(canvas *Canvas, rect Rect, opts *SamplingOptions, paint *Paint) {
	ratio := s.AspectRatio()
	w := rect.Width
	h := w / ratio
	if h > rect.Height {
		h = rect.Height
		w = h * ratio
	}
	s.DrawInRect(canvas, NewRect(rect.X+(rect.Width-w)/2, rect.Y+(rect.Height-h)/2, w, h), opts, paint)
}

// LogicalSize implements the Drawable interface.
func (s *DrawableSVG) LogicalSize() Size {
	return s.Size
}

// DrawInRect implements the Drawable interface.
//
// If paint is not nil, the SVG paths will be drawn with the provided paint, ignoring any fill or stroke attributes
// within the source SVG. Be sure to set the Paint's style (fill or stroke) as desired.
func (s *DrawableSVG) DrawInRect(canvas *Canvas, rect Rect, opts *SamplingOptions, paint *Paint) {
	s.SVG.DrawInRectPreservingAspectRatio(canvas, rect, opts, paint)
}
