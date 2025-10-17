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
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"unicode"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/enums/arcsize"
	"github.com/richardwilkes/unison/enums/direction"
	"github.com/richardwilkes/unison/enums/filltype"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/strokecap"
	"github.com/richardwilkes/unison/enums/strokejoin"
	"github.com/richardwilkes/unison/enums/tilemode"
	"golang.org/x/net/html/charset"
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

var (
	errParamMismatch = errors.New("param mismatch")
	errZeroLengthID  = errors.New("zero length id")
	defaultSVGStyle  = svgPathStyle{
		fillOpacity:       1.0,
		strokeOpacity:     1.0,
		strokeWidth:       2.0,
		useNonZeroWinding: true,
		strokeMiter:       4,
		strokeJoin:        strokejoin.Bevel,
		strokeCap:         strokecap.Butt,
		fillInk:           Black,
		transform:         geom.NewIdentityMatrix(),
	}
	svgUnits = []struct {
		suffix     string
		multiplier float64
	}{
		{"px", 1},
		{"cm", 96.0 / 2.54},
		{"mm", 96.0 / 25.4},
		{"pt", 96.0 / 72.0},
		{"in", 96.0},
		{"Q", 96.0 / 40.0},
		{"pc", 96.0 / 6.0},
		{"%", 1},
	}
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

type svgPercentRef uint8

const (
	svgPercentWidth svgPercentRef = iota
	svgPercentHeight
	svgPercentDiag
)

type svgData struct {
	masks     map[string]*svgMask
	grads     map[string]*Gradient
	defs      map[string][]svgDef
	paths     []svgStyledPath
	transform geom.Matrix
}

type svgPathStyle struct {
	fillInk           Ink
	strokeInk         Ink
	masks             []string // Currently unused
	dash              []float32
	dashOffset        float32
	fillOpacity       float32
	strokeOpacity     float32
	strokeWidth       float32
	strokeMiter       float32
	transform         geom.Matrix
	strokeJoin        strokejoin.Enum
	strokeCap         strokecap.Enum
	useNonZeroWinding bool
}

type svgStyledPath struct {
	path  *Path
	style svgPathStyle
}

type svgMask struct {
	id        string
	paths     []svgStyledPath
	bounds    geom.Rect
	transform geom.Matrix
}

type svgDef struct {
	id    string
	tag   string
	attrs []xml.Attr
}

type svgParser struct {
	svg        *SVG
	data       *svgData
	grad       *Gradient
	mask       *svgMask
	styleStack []svgPathStyle
	currentDef []svgDef
	path       *Path
	pts        []float32
	placeX     float32
	placeY     float32
	curX       float32
	curY       float32
	cntlPtX    float32
	cntlPtY    float32
	pathStartX float32
	pathStartY float32
	lastKey    uint8
	inPath     bool
	inGrad     bool
	inDefs     bool
	inMask     bool
}

// MustSVGFromContentString creates a new SVG and panics if an error would be generated. The content should contain
// valid SVG file data. See notes on NewSVGFromReader.
func MustSVGFromContentString(content string) *SVG {
	s, err := NewSVGFromContentString(content)
	xos.ExitIfErr(err)
	return s
}

// NewSVGFromContentString creates a new SVG. The content should contain valid SVG file data. See notes on
// NewSVGFromReader.
func NewSVGFromContentString(content string) (*SVG, error) {
	return NewSVGFromReader(strings.NewReader(content))
}

// MustSVGFromReader creates a new SVG and panics if an error would be generated. The reader should contain valid SVG
// file data. See notes on NewSVGFromReader.
func MustSVGFromReader(r io.Reader) *SVG {
	s, err := NewSVGFromReader(r)
	xos.ExitIfErr(err)
	return s
}

// NewSVGFromReader creates a new SVG. The reader should contain valid SVG file data.
//
// Current Limitations:
// - Mask elements are ignored
// - Text elements are ignored
// - Mostly supports svg 1.1 and not higher versions of the standard
// - Style attributes are only partially supported
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
// with the provided paint, ignoring any fill or stroke attributes within the source SVG.
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
// within the source SVG.
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

func parseSVG(stream io.Reader) (*SVG, error) {
	svg := &svgData{
		defs:      make(map[string][]svgDef),
		grads:     make(map[string]*Gradient),
		masks:     make(map[string]*svgMask),
		transform: geom.NewIdentityMatrix(),
	}
	p := &svgParser{
		svg:        &SVG{},
		data:       svg,
		styleStack: []svgPathStyle{defaultSVGStyle},
		path:       NewPath(),
	}
	d := xml.NewDecoder(stream)
	d.CharsetReader = charset.NewReaderLabel
	seenTag := false
	for {
		t, err := d.Token()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, err
			}
			if !seenTag {
				return nil, errs.New("invalid svg data")
			}
			break
		}
		switch se := t.(type) {
		case xml.StartElement:
			seenTag = true
			if err = p.pushStyle(se.Attr); err != nil {
				return nil, err
			}
			if err = p.readStartElement(se); err != nil {
				return nil, err
			}
		case xml.EndElement:
			p.styleStack = p.styleStack[:len(p.styleStack)-1]
			switch se.Name.Local {
			case "g":
				if p.inDefs {
					p.currentDef = append(p.currentDef, svgDef{tag: "endg"})
				}
			case "mask":
				if p.mask != nil {
					p.data.masks[p.mask.id] = p.mask
					p.mask = nil
				}
				p.inMask = false
			case "defs":
				if len(p.currentDef) > 0 {
					p.data.defs[p.currentDef[0].id] = p.currentDef
					p.currentDef = make([]svgDef, 0)
				}
				p.inDefs = false
			case "radialGradient", "linearGradient":
				p.inGrad = false
			}
		}
	}

	// From here down converts to unison's internal representation
	p.svg.paths = make([]*svgPath, len(svg.paths))
	for i := range svg.paths {
		p1 := svg.paths[i].path
		if svg.paths[i].style.useNonZeroWinding {
			p1.SetFillType(filltype.Winding)
		} else {
			p1.SetFillType(filltype.EvenOdd)
		}
		if !svg.paths[i].style.transform.IsIdentity() {
			p1.Transform(svg.paths[i].style.transform)
		}
		sp := &svgPath{path: p1}
		if svg.paths[i].style.fillInk != nil && svg.paths[i].style.fillOpacity != 0 {
			sp.fillInk = createInkForSVG(svg.paths[i].style.fillInk, svg.paths[i].style.fillOpacity)
		}
		if svg.paths[i].style.strokeInk != nil && svg.paths[i].style.strokeOpacity != 0 &&
			svg.paths[i].style.strokeWidth != 0 {
			if len(svg.paths[i].style.dash) != 0 {
				sp.dash = NewDashPathEffect(svg.paths[i].style.dash, svg.paths[i].style.dashOffset)
			}
			sp.strokeInk = createInkForSVG(svg.paths[i].style.strokeInk, svg.paths[i].style.strokeOpacity)
			sp.strokeCap = svg.paths[i].style.strokeCap
			sp.strokeJoin = svg.paths[i].style.strokeJoin
			sp.strokeMiter = svg.paths[i].style.strokeMiter
			sp.strokeWidth = svg.paths[i].style.strokeWidth
		}
		p.svg.paths[i] = sp
		if len(svg.paths[i].style.masks) != 0 {
			slog.Warn("svg: masks are not currently supported")
		}
	}
	return p.svg, nil
}

func createInkForSVG(pattern Ink, opacity float32) Ink {
	switch t := pattern.(type) {
	case Color:
		return t.MultiplyAlpha(opacity)
	case *Gradient:
		t = t.Clone()
		for i := range t.Stops {
			c := t.Stops[i].Color.GetColor()
			t.Stops[i].Color = c.MultiplyAlpha(opacity)
		}
		return t
	default:
		return Black
	}
}

func (p *svgParser) readTransformAttr(op string, m geom.Matrix) (geom.Matrix, error) {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "rotate":
		switch len(p.pts) {
		case 1:
			return m.RotateByDegrees(p.pts[0]), nil
		case 3:
			return m.Translate(p.pts[1], p.pts[2]).RotateByDegrees(p.pts[0]).Translate(-p.pts[1], -p.pts[2]), nil
		default:
			return m, errParamMismatch
		}
	case "translate":
		switch len(p.pts) {
		case 1:
			return m.Translate(p.pts[0], 0), nil
		case 2:
			return m.Translate(p.pts[0], p.pts[1]), nil
		default:
			return m, errParamMismatch
		}
	case "skewx":
		switch len(p.pts) {
		case 1:
			return m.SkewByDegrees(p.pts[0], 0), nil
		default:
			return m, errParamMismatch
		}
	case "skewy":
		switch len(p.pts) {
		case 1:
			return m.SkewByDegrees(0, p.pts[0]), nil
		default:
			return m, errParamMismatch
		}
	case "scale":
		switch len(p.pts) {
		case 1:
			return m.Scale(p.pts[0], p.pts[0]), nil
		case 2:
			return m.Scale(p.pts[0], p.pts[1]), nil
		default:
			return m, errParamMismatch
		}
	case "matrix":
		switch len(p.pts) {
		case 6:
			return m.Multiply(geom.Matrix{
				ScaleX: p.pts[0],
				SkewX:  p.pts[2],
				TransX: p.pts[4],
				SkewY:  p.pts[1],
				ScaleY: p.pts[3],
				TransY: p.pts[5],
			}), nil
		default:
			return m, errParamMismatch
		}
	default:
		return m, errParamMismatch
	}
}

func (p *svgParser) parseTransform(v string) (geom.Matrix, error) {
	s := strings.Split(v, ")")
	m := p.styleStack[len(p.styleStack)-1].transform
	for i := len(s) - 1; i >= 0; i-- {
		t := strings.TrimSpace(s[i])
		if t == "" {
			continue
		}
		data := strings.Split(t, "(")
		if len(data) != 2 || len(data[1]) < 1 {
			return m, errParamMismatch
		}
		err := p.addPoints(data[1])
		if err != nil {
			return m, err
		}
		if m, err = p.readTransformAttr(data[0], m); err != nil {
			return m, err
		}
	}
	return m, nil
}

func (p *svgParser) parseSelector(v string) (string, error) {
	if v == "" || v == "none" {
		return "", nil
	}
	if strings.HasPrefix(v, "url(") {
		i := strings.Index(v, ")")
		if i < 0 {
			return "", errParamMismatch
		}
		v = v[4:i]
		if !strings.HasPrefix(v, "#") {
			return "", fmt.Errorf("unsupported url selector: %s", v)
		}
		return v[1:], nil
	}
	return "", errs.Newf("unsupported selector: %s", v)
}

func swapGradientForColorIfNeeded(g *Gradient) Ink {
	switch len(g.Stops) {
	case 0:
		slog.Warn("svg: gradient has no stops, using black")
		return Black
	case 1:
		slog.Warn("svg: gradient has only one stop, using solid color")
		return g.Stops[0].Color.GetColor()
	default:
		return g
	}
}

func (p *svgParser) readStyleAttr(curStyle *svgPathStyle, k, v string) error {
	var err error
	v = strings.TrimSpace(v)
	switch strings.TrimSpace(strings.ToLower(k)) {
	case "fill":
		if gradient, ok := p.readGradientURL(v, curStyle.fillInk); ok {
			curStyle.fillInk = swapGradientForColorIfNeeded(gradient)
		} else if curStyle.fillInk, err = ColorDecode(v); err != nil {
			return err
		}
	case "fill-rule":
		switch v {
		case "evenodd":
			curStyle.useNonZeroWinding = false
		case "nonzero":
			curStyle.useNonZeroWinding = true
		default:
			slog.Warn("svg: unsupported value for fill-rule", "value", v)
		}
	case "stroke":
		if gradient, ok := p.readGradientURL(v, curStyle.strokeInk); ok {
			curStyle.strokeInk = swapGradientForColorIfNeeded(gradient)
		} else if curStyle.strokeInk, err = ColorDecode(v); err != nil {
			return err
		}
	case "stroke-linecap":
		switch v {
		case "butt":
			curStyle.strokeCap = strokecap.Butt
		case "round":
			curStyle.strokeCap = strokecap.Round
		case "square":
			curStyle.strokeCap = strokecap.Square
		default:
			slog.Warn("svg: unsupported value for <stroke-linecap>", "value", v)
		}
	case "stroke-linejoin":
		switch v {
		case "miter":
			curStyle.strokeJoin = strokejoin.Miter
		case "round":
			curStyle.strokeJoin = strokejoin.Round
		case "bevel":
			curStyle.strokeJoin = strokejoin.Bevel
		default:
			slog.Warn("svg: unsupported value for <stroke-linejoin>", "value", v)
		}
	case "stroke-miterlimit":
		if curStyle.strokeMiter, err = svgParseBasicFloat(v); err != nil {
			return err
		}
	case "stroke-width":
		if curStyle.strokeWidth, err = p.parseUnitToPx(v, svgPercentWidth); err != nil {
			return err
		}
	case "stroke-dashoffset":
		if curStyle.dashOffset, err = p.parseUnitToPx(v, svgPercentDiag); err != nil {
			return err
		}
	case "stroke-dasharray":
		if v != "none" {
			dashes := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })
			dList := make([]float32, len(dashes))
			for i, dstr := range dashes {
				if dList[i], err = p.parseUnitToPx(strings.TrimSpace(dstr), svgPercentDiag); err != nil {
					return err
				}
			}
			curStyle.dash = dList
		}
	case "opacity":
		var opacity float32
		if opacity, err = svgParseBasicFloat(v); err != nil {
			return err
		}
		curStyle.fillOpacity *= opacity
		curStyle.strokeOpacity *= opacity
	case "stroke-opacity":
		var opacity float32
		if opacity, err = svgParseBasicFloat(v); err != nil {
			return err
		}
		curStyle.strokeOpacity *= opacity
	case "fill-opacity":
		var opacity float32
		if opacity, err = svgParseBasicFloat(v); err != nil {
			return err
		}
		curStyle.fillOpacity *= opacity
	case "transform":
		if curStyle.transform, err = p.parseTransform(v); err != nil {
			return err
		}
	case "mask":
		var id string
		if id, err = p.parseSelector(v); err != nil {
			return err
		}
		curStyle.masks = append(curStyle.masks, id)
	}
	return nil
}

func (p *svgParser) pushStyle(attrs []xml.Attr) error {
	var pairs []string
	for _, attr := range attrs {
		switch strings.ToLower(attr.Name.Local) {
		case "style":
			pairs = append(pairs, strings.Split(attr.Value, ";")...)
		default:
			pairs = append(pairs, attr.Name.Local+":"+attr.Value)
		}
	}
	s := p.styleStack[len(p.styleStack)-1]
	if len(s.masks) != 0 {
		// Make a copy of the current masks, so that we don't modify the one below us on the stack
		s.masks = append([]string{}, s.masks...)
	}
	for _, pair := range pairs {
		kv := strings.Split(pair, ":")
		if len(kv) >= 2 {
			if err := p.readStyleAttr(&s, kv[0], kv[1]); err != nil {
				return err
			}
		}
	}
	p.styleStack = append(p.styleStack, s)
	return nil
}

func (p *svgParser) readStartElement(se xml.StartElement) error {
	var skipDef bool
	if p.inGrad || se.Name.Local == "radialGradient" || se.Name.Local == "linearGradient" {
		skipDef = true
	}
	if !skipDef && p.inDefs {
		id := ""
		for _, attr := range se.Attr {
			if attr.Name.Local == "id" {
				id = attr.Value
			}
		}
		if id != "" && len(p.currentDef) > 0 {
			p.data.defs[p.currentDef[0].id] = p.currentDef
			p.currentDef = make([]svgDef, 0)
		}
		p.currentDef = append(p.currentDef, svgDef{
			id:    id,
			tag:   se.Name.Local,
			attrs: se.Attr,
		})
		return nil
	}
	if err := p.executeDrawFunc(se.Name.Local, se.Attr); err != nil {
		return err
	}
	if !p.path.Empty() {
		if p.inMask && p.mask != nil {
			p.mask.paths = append(p.mask.paths,
				svgStyledPath{path: p.path, style: p.styleStack[len(p.styleStack)-1]})
		} else if !p.inMask {
			p.data.paths = append(p.data.paths,
				svgStyledPath{path: p.path, style: p.styleStack[len(p.styleStack)-1]})
		}
		p.path = NewPath()
	}
	return nil
}

func (p *svgParser) readGradientURL(v string, defaultColor Ink) (grad *Gradient, ok bool) {
	if strings.HasPrefix(v, "url(") && strings.HasSuffix(v, ")") {
		urlStr := strings.TrimSpace(v[4 : len(v)-1])
		if strings.HasPrefix(urlStr, "#") {
			var g *Gradient
			g, ok = p.data.grads[urlStr[1:]]
			if ok {
				g2 := *g
				for _, s := range g2.Stops {
					if s.Color != nil {
						continue
					}
					stops := append([]Stop{}, g2.Stops...)
					g2.Stops = stops
					c := getSVGBackgroundColor(defaultColor)
					for i, s := range stops {
						if s.Color == nil {
							g2.Stops[i].Color = c
						}
					}
					break
				}
				grad = &g2
			}
		}
	}
	return grad, ok
}

func getSVGBackgroundColor(clr Ink) Color {
	switch c := clr.(type) {
	case *Gradient:
		for _, s := range c.Stops {
			if color := s.Color.GetColor(); !color.Invisible() {
				return color
			}
		}
	case Color:
		return c
	}
	return Black
}

func (p *svgParser) parseUnitToPx(s string, asPerc svgPercentRef) (float32, error) {
	return svgResolveUnit(p.svg.viewBox, s, asPerc)
}

func (p *svgParser) compilePath(svgPath string) error {
	p.placeX = 0
	p.placeY = 0
	p.pts = p.pts[0:0]
	p.lastKey = ' '
	p.path = NewPath()
	p.inPath = false
	lastIndex := -1
	for i, v := range svgPath {
		if unicode.IsLetter(v) && v != 'e' {
			if lastIndex != -1 {
				if err := p.addSegment(svgPath[lastIndex:i]); err != nil {
					return err
				}
			}
			lastIndex = i
		}
	}
	if lastIndex != -1 {
		if err := p.addSegment(svgPath[lastIndex:]); err != nil {
			return err
		}
	}
	return nil
}

func (p *svgParser) valsToAbs(last float32) {
	for i := 0; i < len(p.pts); i++ {
		last += p.pts[i]
		p.pts[i] = last
	}
}

func (p *svgParser) pointsToAbs(sz int) {
	lastX := p.placeX
	lastY := p.placeY
	for j := 0; j < len(p.pts); j += sz {
		for i := 0; i < sz; i += 2 {
			p.pts[i+j] += lastX
			p.pts[i+1+j] += lastY
		}
		lastX = p.pts[(j+sz)-2]
		lastY = p.pts[(j+sz)-1]
	}
}

func (p *svgParser) hasSetsOrMore(sz int, rel bool) bool {
	if len(p.pts) < sz || len(p.pts)%sz != 0 {
		return false
	}
	if rel {
		p.pointsToAbs(sz)
	}
	return true
}

func (p *svgParser) addPoints(dataPoints string) error {
	lastIndex := -1
	p.pts = p.pts[0:0]
	lr := ' '
	for i, r := range dataPoints {
		if !unicode.IsNumber(r) && r != '.' && (r != '-' || lr != 'e') && r != 'e' {
			if lastIndex != -1 {
				if err := p.readFloatIntoPts(dataPoints[lastIndex:i]); err != nil {
					return err
				}
			}
			if r == '-' {
				lastIndex = i
			} else {
				lastIndex = -1
			}
		} else if lastIndex == -1 {
			lastIndex = i
		}
		lr = r
	}
	if lastIndex != -1 && lastIndex != len(dataPoints) {
		if err := p.readFloatIntoPts(dataPoints[lastIndex:]); err != nil {
			return err
		}
	}
	return nil
}

func (p *svgParser) readFloatIntoPts(numStr string) error {
	last := 0
	isFirst := true
	for i, n := range numStr {
		if n != '.' {
			continue
		}
		if isFirst {
			isFirst = false
			continue
		}
		f, err := svgParseBasicFloat(numStr[last:i])
		if err != nil {
			return err
		}
		p.pts = append(p.pts, f)
		last = i
	}
	f, err := svgParseBasicFloat(numStr[last:])
	if err != nil {
		return err
	}
	p.pts = append(p.pts, f)
	return nil
}

func (p *svgParser) addSegment(segString string) error {
	if err := p.addPoints(segString[1:]); err != nil {
		return err
	}
	l := len(p.pts)
	k := segString[0]
	rel := false
	switch k {
	case 'Z', 'z':
		if len(p.pts) != 0 {
			return errParamMismatch
		}
		if p.inPath {
			p.path.Close()
			p.placeX = p.pathStartX
			p.placeY = p.pathStartY
			p.inPath = false
		}
	case 'm':
		rel = true
		fallthrough
	case 'M':
		if !p.hasSetsOrMore(2, rel) {
			return errParamMismatch
		}
		p.pathStartX = p.pts[0]
		p.pathStartY = p.pts[1]
		p.inPath = true
		p.path.MoveTo(geom.NewPoint(p.pathStartX+p.curX, p.pathStartY+p.curY))
		for i := 2; i < l-1; i += 2 {
			p.path.LineTo(geom.NewPoint(p.pts[i]+p.curX, p.pts[i+1]+p.curY))
		}
		p.placeX = p.pts[l-2]
		p.placeY = p.pts[l-1]
	case 'l':
		rel = true
		fallthrough
	case 'L':
		if !p.hasSetsOrMore(2, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-1; i += 2 {
			p.path.LineTo(geom.NewPoint(p.pts[i]+p.curX, p.pts[i+1]+p.curY))
		}
		p.placeX = p.pts[l-2]
		p.placeY = p.pts[l-1]
	case 'v':
		p.valsToAbs(p.placeY)
		fallthrough
	case 'V':
		if !p.hasSetsOrMore(1, false) {
			return errParamMismatch
		}
		for _, pt := range p.pts {
			p.path.LineTo(geom.NewPoint(p.placeX+p.curX, pt+p.curY))
		}
		p.placeY = p.pts[l-1]
	case 'h':
		p.valsToAbs(p.placeX)
		fallthrough
	case 'H':
		if !p.hasSetsOrMore(1, false) {
			return errParamMismatch
		}
		for _, pt := range p.pts {
			p.path.LineTo(geom.NewPoint(pt+p.curX, p.placeY+p.curY))
		}
		p.placeX = p.pts[l-1]
	case 'q', 'Q':
		if !p.hasSetsOrMore(4, k == 'q') {
			return errParamMismatch
		}
		for i := 0; i < l-3; i += 4 {
			p.path.QuadTo(
				geom.NewPoint(p.pts[i]+p.curX, p.pts[i+1]+p.curY),
				geom.NewPoint(p.pts[i+2]+p.curX, p.pts[i+3]+p.curY),
			)
		}
		p.cntlPtX, p.cntlPtY = p.pts[l-4], p.pts[l-3]
		p.placeX = p.pts[l-2]
		p.placeY = p.pts[l-1]
	case 't':
		rel = true
		fallthrough
	case 'T':
		if !p.hasSetsOrMore(2, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-1; i += 2 {
			p.reflectControl(true)
			p.path.QuadTo(
				geom.NewPoint(p.cntlPtX+p.curX, p.cntlPtY+p.curY),
				geom.NewPoint(p.pts[i]+p.curX, p.pts[i+1]+p.curY),
			)
			p.lastKey = k
			p.placeX = p.pts[i]
			p.placeY = p.pts[i+1]
		}
	case 'c', 'C':
		if !p.hasSetsOrMore(6, k == 'c') {
			return errParamMismatch
		}
		for i := 0; i < l-5; i += 6 {
			p.path.CubicTo(
				geom.NewPoint(p.pts[i]+p.curX, p.pts[i+1]+p.curY),
				geom.NewPoint(p.pts[i+2]+p.curX, p.pts[i+3]+p.curY),
				geom.NewPoint(p.pts[i+4]+p.curX, p.pts[i+5]+p.curY),
			)
		}
		p.cntlPtX, p.cntlPtY = p.pts[l-4], p.pts[l-3]
		p.placeX = p.pts[l-2]
		p.placeY = p.pts[l-1]
	case 's', 'S':
		if !p.hasSetsOrMore(4, k == 's') {
			return errParamMismatch
		}
		for i := 0; i < l-3; i += 4 {
			p.reflectControl(false)
			p.path.CubicTo(
				geom.NewPoint(p.cntlPtX+p.curX, p.cntlPtY+p.curY),
				geom.NewPoint(p.pts[i]+p.curX, p.pts[i+1]+p.curY),
				geom.NewPoint(p.pts[i+2]+p.curX, p.pts[i+3]+p.curY),
			)
			p.lastKey = k
			p.cntlPtX, p.cntlPtY = p.pts[i], p.pts[i+1]
			p.placeX = p.pts[i+2]
			p.placeY = p.pts[i+3]
		}
	case 'a', 'A':
		if !p.hasSetsOrMore(7, false) {
			return errParamMismatch
		}
		for i := 0; i < l-6; i += 7 {
			if k == 'a' {
				p.pts[i+5] += p.placeX
				p.pts[i+6] += p.placeY
			}
			x := p.pts[i+5] + p.curX
			y := p.pts[i+6] + p.curY
			as := arcsize.Small
			if p.pts[i+3] != 0 {
				as = arcsize.Large
			}
			dir := direction.CounterClockwise
			if p.pts[i+4] != 0 {
				dir = direction.Clockwise
			}
			p.path.ArcTo(geom.NewPoint(x, y), geom.NewSize(p.pts[i], p.pts[i+1]), p.pts[i+2], as, dir)
			p.placeX = x
			p.placeY = y
		}
	default:
		slog.Warn("svg: ignoring unknown path command", "command", string(k))
	}
	p.lastKey = k
	return nil
}

func (p *svgParser) reflectControl(forQuad bool) {
	if (forQuad && (p.lastKey == 'q' || p.lastKey == 'Q' || p.lastKey == 'T' || p.lastKey == 't')) ||
		(!forQuad && (p.lastKey == 'c' || p.lastKey == 'C' || p.lastKey == 's' || p.lastKey == 'S')) {
		p.cntlPtX = p.placeX*2 - p.cntlPtX
		p.cntlPtY = p.placeY*2 - p.cntlPtY
	} else {
		p.cntlPtX, p.cntlPtY = p.placeX, p.placeY
	}
}

func (p *svgParser) executeDrawFunc(name string, attrs []xml.Attr) error {
	switch name {
	case "path":
		return p.handlePathElement(attrs)
	case "defs":
		p.inDefs = true
		return nil
	case "stop":
		return p.handleStopElement(attrs)
	case "linearGradient":
		return p.handleLinearGradientElement(attrs)
	case "radialGradient":
		return p.handleRadialGradientElement(attrs)
	case "rect":
		return p.handleRectElement(attrs)
	case "circle", "ellipse":
		return p.handleCircleElement(attrs)
	case "line":
		return p.handleLineElement(attrs)
	case "polyline":
		return p.handlePolylineElement(attrs)
	case "polygon":
		if err := p.handlePolylineElement(attrs); err != nil {
			return err
		}
		if len(p.pts) > 4 {
			p.path.Close()
		}
		return nil
	case "svg":
		return p.handleSVGElement(attrs)
	case "use":
		return p.handleUseElement(attrs)
	case "mask":
		return p.handleMaskElement(attrs)
	case "g", "desc", "title":
		return nil
	default:
		slog.Warn("svg: cannot process element", "element", name)
		return nil
	}
}

func (p *svgParser) handlePathElement(attrs []xml.Attr) error {
	for _, attr := range attrs {
		if attr.Name.Local != "d" {
			continue
		}
		if err := p.compilePath(attr.Value); err != nil {
			return err
		}
	}
	return nil
}

func (p *svgParser) handleStopElement(attrs []xml.Attr) error {
	if p.inGrad {
		for _, attr := range attrs {
			if attr.Name.Local != "style" {
				continue
			}
			if attr.Value == "" {
				break
			}
			for _, s := range strings.Split(attr.Value, ";") {
				key, val, ok := strings.Cut(s, ":")
				if !ok {
					continue
				}
				key = strings.ToLower(strings.TrimSpace(key))
				if key == "stop-color" || key == "stop-opacity" {
					attrs = append(attrs, xml.Attr{
						Name:  xml.Name{Local: key},
						Value: strings.TrimSpace(val),
					})
				}
			}
			break
		}
		var stop Stop
		var err error
		opacity := float32(1.0)
		for _, attr := range attrs {
			switch attr.Name.Local {
			case "offset":
				if stop.Location, err = svgReadFraction(attr.Value); err != nil {
					return err
				}
			case "stop-color":
				if stop.Color, err = ColorDecode(attr.Value); err != nil {
					return err
				}
			case "stop-opacity":
				if opacity, err = svgParseBasicFloat(attr.Value); err != nil {
					return err
				}
			}
		}
		if stop.Color != nil && opacity != 1 {
			stop.Color = stop.Color.GetColor().SetAlphaIntensity(opacity)
		}
		p.grad.Stops = append(p.grad.Stops, stop)
	}
	return nil
}

func (p *svgParser) handleLinearGradientElement(attrs []xml.Attr) error {
	userSpaceOnUse := false
	x1 := "0%"
	y1 := x1
	x2 := "100%"
	y2 := x1
	p.inGrad = true
	p.grad = &Gradient{Transform: geom.NewIdentityMatrix()}
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "id":
			if attr.Value == "" {
				return errZeroLengthID
			}
			p.data.grads[attr.Value] = p.grad
		case "x1":
			x1 = attr.Value
		case "y1":
			y1 = attr.Value
		case "x2":
			x2 = attr.Value
		case "y2":
			y2 = attr.Value
		default:
			if err := p.readCommonGradientAttrs(attr, &userSpaceOnUse); err != nil {
				return err
			}
		}
	}
	bbox := geom.NewRect(0, 0, 1, 1)
	if userSpaceOnUse {
		bbox = p.svg.viewBox
	}
	var err error
	p.grad.Start.X, err = svgResolveUnit(bbox, x1, svgPercentWidth)
	if err != nil {
		return err
	}
	p.grad.Start.Y, err = svgResolveUnit(bbox, y1, svgPercentHeight)
	if err != nil {
		return err
	}
	p.grad.End.X, err = svgResolveUnit(bbox, x2, svgPercentWidth)
	if err != nil {
		return err
	}
	p.grad.End.Y, err = svgResolveUnit(bbox, y2, svgPercentHeight)
	if err != nil {
		return err
	}
	p.normalizeGradientStartEnd()
	return nil
}

func (p *svgParser) normalizeGradientStartEnd() {
	p.grad.Start.X = (p.grad.Start.X - p.svg.viewBox.X) / p.svg.viewBox.Width
	p.grad.Start.Y = (p.grad.Start.Y - p.svg.viewBox.Y) / p.svg.viewBox.Height
	p.grad.End.X = (p.grad.End.X - p.svg.viewBox.X) / p.svg.viewBox.Width
	p.grad.End.Y = (p.grad.End.Y - p.svg.viewBox.Y) / p.svg.viewBox.Height
}

func (p *svgParser) handleRadialGradientElement(attrs []xml.Attr) error {
	userSpaceOnUse := false
	cx := "50%"
	cy := cx
	fx := ""
	fy := ""
	r := cx
	fr := cx
	p.inGrad = true
	p.grad = &Gradient{Transform: geom.NewIdentityMatrix()}
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "id":
			if attr.Value == "" {
				return errZeroLengthID
			}
			p.data.grads[attr.Value] = p.grad
		case "cx":
			cx = attr.Value
		case "cy":
			cy = attr.Value
		case "fx":
			fx = attr.Value
		case "fy":
			fy = attr.Value
		case "r":
			r = attr.Value
		case "fr":
			fr = attr.Value
		default:
			if err := p.readCommonGradientAttrs(attr, &userSpaceOnUse); err != nil {
				return err
			}
		}
	}
	if fx == "" {
		fx = cx
	}
	if fy == "" {
		fy = cy
	}
	bbox := geom.NewRect(0, 0, 1, 1)
	if userSpaceOnUse {
		bbox = p.svg.viewBox
	}
	var err error
	p.grad.Start.X, err = svgResolveUnit(bbox, cx, svgPercentWidth)
	if err != nil {
		return err
	}
	p.grad.Start.Y, err = svgResolveUnit(bbox, cy, svgPercentHeight)
	if err != nil {
		return err
	}
	p.grad.End.X, err = svgResolveUnit(bbox, fx, svgPercentWidth)
	if err != nil {
		return err
	}
	p.grad.End.Y, err = svgResolveUnit(bbox, fy, svgPercentHeight)
	if err != nil {
		return err
	}
	p.grad.StartRadius, err = svgResolveUnit(bbox, r, svgPercentDiag)
	if err != nil {
		return err
	}
	p.grad.EndRadius, err = svgResolveUnit(bbox, fr, svgPercentDiag)
	if err != nil {
		return err
	}
	p.normalizeGradientStartEnd()
	return nil
}

func (p *svgParser) handleRectElement(attrs []xml.Attr) error {
	var x, y, w, h, rx, ry float32
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "x":
			x, err = p.parseUnitToPx(attr.Value, svgPercentWidth)
		case "y":
			y, err = p.parseUnitToPx(attr.Value, svgPercentHeight)
		case "width":
			w, err = p.parseUnitToPx(attr.Value, svgPercentWidth)
		case "height":
			h, err = p.parseUnitToPx(attr.Value, svgPercentHeight)
		case "rx":
			rx, err = p.parseUnitToPx(attr.Value, svgPercentWidth)
		case "ry":
			ry, err = p.parseUnitToPx(attr.Value, svgPercentHeight)
		}
		if err != nil {
			return err
		}
	}
	if w == 0 || h == 0 {
		return nil
	}
	// If only one of rx or ry is specified, the other should be same
	if rx != 0 && ry == 0 {
		ry = rx
	}
	if ry != 0 && rx == 0 {
		rx = ry
	}
	if rx == 0 {
		p.path.Rect(geom.NewRect(x+p.curX, y+p.curY, w, h))
	} else {
		p.path.RoundedRect(geom.NewRect(x+p.curX, y+p.curY, w, h), geom.NewSize(rx, ry))
	}
	return nil
}

func (p *svgParser) handleCircleElement(attrs []xml.Attr) error {
	var cx, cy, rx, ry float32
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "cx":
			cx, err = p.parseUnitToPx(attr.Value, svgPercentWidth)
		case "cy":
			cy, err = p.parseUnitToPx(attr.Value, svgPercentHeight)
		case "r":
			rx, err = p.parseUnitToPx(attr.Value, svgPercentDiag)
			ry = rx
		case "rx":
			rx, err = p.parseUnitToPx(attr.Value, svgPercentWidth)
		case "ry":
			ry, err = p.parseUnitToPx(attr.Value, svgPercentHeight)
		}
		if err != nil {
			return err
		}
	}
	if rx == 0 || ry == 0 {
		return nil
	}
	cx += p.curX
	cy += p.curY
	if rx == ry {
		p.path.Circle(geom.NewPoint(cx, cy), rx)
	} else {
		p.path.Oval(geom.NewRect(cx-rx, cy-ry, rx*2, ry*2))
	}
	return nil
}

func (p *svgParser) handleLineElement(attrs []xml.Attr) error {
	var x1, x2, y1, y2 float32
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "x1":
			x1, err = p.parseUnitToPx(attr.Value, svgPercentWidth)
		case "x2":
			x2, err = p.parseUnitToPx(attr.Value, svgPercentWidth)
		case "y1":
			y1, err = p.parseUnitToPx(attr.Value, svgPercentHeight)
		case "y2":
			y2, err = p.parseUnitToPx(attr.Value, svgPercentHeight)
		}
		if err != nil {
			return err
		}
	}
	p.path.MoveTo(geom.NewPoint(x1+p.curX, y1+p.curY))
	p.path.LineTo(geom.NewPoint(x2+p.curX, y2+p.curY))
	return nil
}

func (p *svgParser) handlePolylineElement(attrs []xml.Attr) error {
	for _, attr := range attrs {
		if attr.Name.Local != "points" {
			continue
		}
		if err := p.addPoints(attr.Value); err != nil {
			return err
		}
		if len(p.pts)%2 != 0 {
			return errors.New("polygon has odd number of points")
		}
	}
	if len(p.pts) > 4 {
		p.path.MoveTo(geom.NewPoint(p.pts[0]+p.curX, p.pts[1]+p.curY))
		for i := 2; i < len(p.pts)-1; i += 2 {
			p.path.LineTo(geom.NewPoint(p.pts[i]+p.curX, p.pts[i+1]+p.curY))
		}
	}
	return nil
}

func (p *svgParser) handleSVGElement(attrs []xml.Attr) error {
	p.svg.viewBox = geom.Rect{}
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "viewBox":
			if err := p.addPoints(attr.Value); err != nil {
				return err
			}
			if len(p.pts) != 4 {
				return errParamMismatch
			}
			p.svg.viewBox.X = p.pts[0]
			p.svg.viewBox.Y = p.pts[1]
			p.svg.viewBox.Width = p.pts[2]
			p.svg.viewBox.Height = p.pts[3]
		case "width": //nolint:goconst // Can't use const named width
			width, err := svgParseBasicFloat(attr.Value)
			if err != nil {
				return err
			}
			p.svg.suggestedSize.Width = width
		case "height": //nolint:goconst // Can't use const named height
			height, err := svgParseBasicFloat(attr.Value)
			if err != nil {
				return err
			}
			p.svg.suggestedSize.Height = height
		}
	}
	if p.svg.viewBox.Width == 0 {
		p.svg.viewBox.Width = p.svg.suggestedSize.Width
	}
	if p.svg.suggestedSize.Width == 0 {
		p.svg.suggestedSize.Width = p.svg.viewBox.Width
	}
	if p.svg.viewBox.Height == 0 {
		p.svg.viewBox.Height = p.svg.suggestedSize.Height
	}
	if p.svg.suggestedSize.Height == 0 {
		p.svg.suggestedSize.Height = p.svg.viewBox.Height
	}
	return nil
}

func (p *svgParser) handleUseElement(attrs []xml.Attr) error {
	var (
		href string
		x, y float32
		err  error
	)
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "href":
			href = attr.Value
		case "x":
			x, err = p.parseUnitToPx(attr.Value, svgPercentWidth)
		case "y":
			y, err = p.parseUnitToPx(attr.Value, svgPercentHeight)
		}
		if err != nil {
			return err
		}
	}
	p.curX, p.curY = x, y
	defer func() {
		p.curX, p.curY = 0, 0
	}()
	if href == "" {
		return errors.New("only use tags with href is supported")
	}
	if !strings.HasPrefix(href, "#") {
		return errors.New("only the ID CSS selector is supported")
	}
	defs, ok := p.data.defs[href[1:]]
	if !ok {
		return errors.New("href ID in use statement was not found in saved defs")
	}
	for _, def := range defs {
		if def.tag == "endg" {
			p.styleStack = p.styleStack[:len(p.styleStack)-1]
			continue
		}
		if err = p.pushStyle(def.attrs); err != nil {
			return err
		}
		if err = p.executeDrawFunc(def.tag, def.attrs); err != nil {
			return err
		}
		if def.tag != "g" {
			p.styleStack = p.styleStack[:len(p.styleStack)-1]
		}
	}
	return nil
}

func (p *svgParser) handleMaskElement(attrs []xml.Attr) error {
	var mask svgMask
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "id":
			if attr.Value == "" {
				return errZeroLengthID
			}
			mask.id = attr.Value
		case "x":
			mask.bounds.X, err = p.parseUnitToPx(attr.Value, svgPercentWidth)
		case "y":
			mask.bounds.Y, err = p.parseUnitToPx(attr.Value, svgPercentHeight)
		case "width":
			mask.bounds.Width, err = p.parseUnitToPx(attr.Value, svgPercentWidth)
		case "height":
			mask.bounds.Height, err = p.parseUnitToPx(attr.Value, svgPercentHeight)
		}
		if err != nil {
			return err
		}
	}
	mask.transform = geom.NewIdentityMatrix()
	p.inMask = true
	p.mask = &mask
	return nil
}

func (p *svgParser) readCommonGradientAttrs(attr xml.Attr, userSpaceOnUse *bool) error {
	switch attr.Name.Local {
	case "gradientTransform":
		var err error
		if p.grad.Transform, err = p.parseTransform(attr.Value); err != nil {
			return err
		}
	case "gradientUnits":
		switch strings.TrimSpace(attr.Value) {
		case "userSpaceOnUse":
			*userSpaceOnUse = true
		case "objectBoundingBox":
			*userSpaceOnUse = false
		}
	case "spreadMethod":
		switch strings.TrimSpace(attr.Value) {
		case "pad":
			p.grad.TileMode = tilemode.Clamp
		case "reflect":
			p.grad.TileMode = tilemode.Mirror
		case "repeat":
			p.grad.TileMode = tilemode.Repeat
		}
	}
	return nil
}

func svgParseUnit(s string) (f float32, isPercent bool, err error) {
	multiplier := 1.0
	s = strings.TrimSpace(s)
	for i := range svgUnits {
		var ok bool
		if s, ok = strings.CutSuffix(s, svgUnits[i].suffix); ok {
			multiplier = svgUnits[i].multiplier
			isPercent = i == len(svgUnits)-1
			break
		}
	}
	var out float64
	out, err = strconv.ParseFloat(strings.TrimSpace(s), 64)
	return float32(out * multiplier), isPercent, err
}

func svgResolveUnit(viewBox geom.Rect, s string, asPerc svgPercentRef) (float32, error) {
	value, isPercentage, err := svgParseUnit(s)
	if err != nil {
		return 0, err
	}
	if isPercentage {
		w, h := viewBox.Width, viewBox.Height
		switch asPerc {
		case svgPercentWidth:
			return value / 100 * w, nil
		case svgPercentHeight:
			return value / 100 * h, nil
		case svgPercentDiag:
			normalizedDiag := xmath.Sqrt(w*w+h*h) / xmath.Sqrt(2)
			return value / 100 * normalizedDiag, nil
		}
	}
	return value, nil
}

func svgParseBasicFloat(s string) (float32, error) {
	value, _, err := svgParseUnit(s)
	return value, err
}

func svgReadFraction(v string) (f float32, err error) {
	v = strings.TrimSpace(v)
	d := float32(1.0)
	if strings.HasSuffix(v, "%") {
		d = 100
		v = strings.TrimSuffix(v, "%")
	}
	f, err = svgParseBasicFloat(v)
	return f / d, err
}
