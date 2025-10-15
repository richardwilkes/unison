// Provides parsing and rendering of SVG images.
// SVG files are parsed into an abstract representation,
// which can then be consumed by painting drivers.
package svg

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"golang.org/x/image/math/fixed"
	"golang.org/x/net/html/charset"
)

const (
	none      = "none"
	round     = "round"
	cubic     = "cubic"
	quadratic = "quadratic"
)

// SVG holds data from parsed SVGs.
type SVG struct {
	Masks     map[string]*Mask
	grads     map[string]*Gradient
	defs      map[string][]definition
	Width     string
	Height    string
	Paths     []StyledPath
	ViewBox   Bounds
	Transform Matrix2D
}

// PathStyle holds the state of the style.
type PathStyle struct {
	Masks             []string
	FillerColor       Pattern
	LinerColor        Pattern
	Dash              DashOptions
	Join              JoinOptions
	FillOpacity       float32
	LineOpacity       float32
	LineWidth         float32
	Transform         Matrix2D
	UseNonZeroWinding bool
}

// StyledPath binds a PathStyle to a Path.
type StyledPath struct {
	Path  Path
	Style PathStyle
}

// Mask is the element that defines a mask for the referenced elements.
type Mask struct {
	ID        string
	SvgPaths  []StyledPath
	Bounds    Bounds
	Transform Matrix2D
}

type definition struct {
	ID    string
	Tag   string
	Attrs []xml.Attr
}

type svgParser struct {
	svg        *SVG
	grad       *Gradient
	mask       *Mask
	styleStack []PathStyle
	currentDef []definition
	pathParser
	inGrad bool
	inDefs bool
	inMask bool
}

// Parse reads the Icon from the given io.Reader
// This only supports a sub-set of SVG, but
// is enough to draw many svgs. errMode determines if the svg ignores, errors out, or logs a warning
// if it does not handle an element found in the svg file.
func Parse(stream io.Reader) (*SVG, error) {
	svg := &SVG{
		defs:      make(map[string][]definition),
		grads:     make(map[string]*Gradient),
		Masks:     make(map[string]*Mask),
		Transform: Identity,
	}
	p := &svgParser{
		styleStack: []PathStyle{DefaultStyle},
		svg:        svg,
	}
	d := xml.NewDecoder(stream)
	d.CharsetReader = charset.NewReaderLabel
	seenTag := false
	for {
		t, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if !seenTag {
					return nil, errs.New("invalid svg data")
				}
				return svg, nil
			}
			return svg, err
		}
		switch se := t.(type) {
		case xml.StartElement:
			seenTag = true
			if err = p.pushStyle(se.Attr); err != nil {
				return svg, err
			}
			if err = p.readStartElement(se); err != nil {
				return svg, err
			}
		case xml.EndElement:
			p.styleStack = p.styleStack[:len(p.styleStack)-1]
			switch se.Name.Local {
			case "g":
				if p.inDefs {
					p.currentDef = append(p.currentDef, definition{Tag: "endg"})
				}
			case "mask":
				if p.mask != nil {
					p.svg.Masks[p.mask.ID] = p.mask
					p.mask = nil
				}
				p.inMask = false
			case "defs":
				if len(p.currentDef) > 0 {
					p.svg.defs[p.currentDef[0].ID] = p.currentDef
					p.currentDef = make([]definition, 0)
				}
				p.inDefs = false
			case "radialGradient", "linearGradient":
				p.inGrad = false
			}
		}
	}
}

func (p *svgParser) readTransformAttr(op string, m Matrix2D) (Matrix2D, error) {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "rotate":
		switch len(p.pts) {
		case 1:
			return m.Rotate(p.pts[0] * math.Pi / 180), nil
		case 3:
			return m.Translate(p.pts[1], p.pts[2]).Rotate(p.pts[0]*math.Pi/180).Translate(-p.pts[1], -p.pts[2]), nil
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
			return m.SkewX(p.pts[0] * math.Pi / 180), nil
		default:
			return m, errParamMismatch
		}
	case "skewy":
		switch len(p.pts) {
		case 1:
			return m.SkewY(p.pts[0] * math.Pi / 180), nil
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
			return m.Mult(Matrix2D{
				ScaleX: p.pts[0],
				SkwX:   p.pts[2],
				TransX: p.pts[4],
				SkwY:   p.pts[1],
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

func (p *svgParser) parseTransform(v string) (Matrix2D, error) {
	s := strings.Split(v, ")")
	m := p.styleStack[len(p.styleStack)-1].Transform
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
	if v == "" || v == none {
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

func (p *svgParser) readStyleAttr(curStyle *PathStyle, k, v string) error {
	v = strings.TrimSpace(v)
	switch strings.TrimSpace(strings.ToLower(k)) {
	case "fill":
		gradient, ok := p.readGradientURL(v, curStyle.FillerColor)
		if ok {
			curStyle.FillerColor = gradient
			return nil
		}
		optCol, err := parseSVGColor(v)
		curStyle.FillerColor = optCol.asPattern()
		return err
	case "fill-rule":
		switch v {
		case "evenodd":
			curStyle.UseNonZeroWinding = false
		case "nonzero":
			curStyle.UseNonZeroWinding = true
		default:
			slog.Warn("svg: unsupported value for fill-rule", "value", v)
		}
	case "stroke":
		if gradient, ok := p.readGradientURL(v, curStyle.LinerColor); ok {
			curStyle.LinerColor = gradient
		} else {
			optCol, err := parseSVGColor(v)
			if err != nil {
				return err
			}
			curStyle.LinerColor = optCol.asPattern()
		}
	case "stroke-linegap":
		switch v {
		case "flat":
			curStyle.Join.LineGap = FlatGap
		case round:
			curStyle.Join.LineGap = RoundGap
		case cubic:
			curStyle.Join.LineGap = CubicGap
		case quadratic:
			curStyle.Join.LineGap = QuadraticGap
		default:
			slog.Warn("svg: unsupported value for stroke-linegap", "value", v)
		}
	case "stroke-leadlinecap":
		switch v {
		case "butt":
			curStyle.Join.LeadLineCap = ButtCap
		case round:
			curStyle.Join.LeadLineCap = RoundCap
		case "square":
			curStyle.Join.LeadLineCap = SquareCap
		case cubic:
			curStyle.Join.LeadLineCap = CubicCap
		case quadratic:
			curStyle.Join.LeadLineCap = QuadraticCap
		default:
			slog.Warn("svg: unsupported value for <stroke-leadlinecap>", "value", v)
		}
	case "stroke-linecap":
		switch v {
		case "butt":
			curStyle.Join.TrailLineCap = ButtCap
		case round:
			curStyle.Join.TrailLineCap = RoundCap
		case "square":
			curStyle.Join.TrailLineCap = SquareCap
		case cubic:
			curStyle.Join.TrailLineCap = CubicCap
		case quadratic:
			curStyle.Join.TrailLineCap = QuadraticCap
		default:
			slog.Warn("svg: unsupported value for <stroke-linecap>", "value", v)
		}
	case "stroke-linejoin":
		switch v {
		case "miter":
			curStyle.Join.LineJoin = Miter
		case "miter-clip":
			curStyle.Join.LineJoin = MiterClip
		case "arc-clip":
			curStyle.Join.LineJoin = ArcClip
		case round:
			curStyle.Join.LineJoin = Round
		case "arc":
			curStyle.Join.LineJoin = Arc
		case "bevel":
			curStyle.Join.LineJoin = Bevel
		default:
			slog.Warn("svg: unsupported value for <stroke-linejoin>", "value", v)
		}
	case "stroke-miterlimit":
		mLimit, err := parseBasicFloat(v)
		if err != nil {
			return err
		}
		curStyle.Join.MiterLimit = fixed.Int26_6(mLimit * 64)
	case "stroke-width":
		width, err := p.parseUnitToPx(v, widthPercentage)
		if err != nil {
			return err
		}
		curStyle.LineWidth = width
	case "stroke-dashoffset":
		dashOffset, err := p.parseUnitToPx(v, diagPercentage)
		if err != nil {
			return err
		}
		curStyle.Dash.DashOffset = dashOffset
	case "stroke-dasharray":
		if v != none {
			dashes := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })
			dList := make([]float32, len(dashes))
			for i, dstr := range dashes {
				d, err := p.parseUnitToPx(strings.TrimSpace(dstr), diagPercentage)
				if err != nil {
					return err
				}
				dList[i] = d
			}
			curStyle.Dash.Dash = dList
		}
	case "opacity":
		op, err := parseBasicFloat(v)
		if err != nil {
			return err
		}
		curStyle.FillOpacity *= op
		curStyle.LineOpacity *= op
	case "stroke-opacity":
		op, err := parseBasicFloat(v)
		if err != nil {
			return err
		}
		curStyle.LineOpacity *= op
	case "fill-opacity":
		op, err := parseBasicFloat(v)
		if err != nil {
			return err
		}
		curStyle.FillOpacity *= op
	case "transform":
		m, err := p.parseTransform(v)
		if err != nil {
			return err
		}
		curStyle.Transform = m
	case "mask":
		id, err := p.parseSelector(v)
		if err != nil {
			return err
		}
		curStyle.Masks = append(curStyle.Masks, id)
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
	if len(s.Masks) != 0 {
		// Make a copy of the current masks, so that we don't modify the one below us on the stack
		s.Masks = append([]string{}, s.Masks...)
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
			p.svg.defs[p.currentDef[0].ID] = p.currentDef
			p.currentDef = make([]definition, 0)
		}
		p.currentDef = append(p.currentDef, definition{
			ID:    id,
			Tag:   se.Name.Local,
			Attrs: se.Attr,
		})
		return nil
	}
	df, ok := drawFuncs[se.Name.Local]
	if !ok {
		slog.Warn("svg: cannot process svg element", "element", se.Name.Local)
		return nil
	}
	var err error
	if df != nil {
		err = df(p, se.Attr)
	}
	if len(p.path) > 0 {
		if p.inMask && p.mask != nil {
			p.mask.SvgPaths = append(p.mask.SvgPaths,
				StyledPath{Path: append(Path{}, p.path...), Style: p.styleStack[len(p.styleStack)-1]})
		} else if !p.inMask {
			p.svg.Paths = append(p.svg.Paths,
				StyledPath{Path: append(Path{}, p.path...), Style: p.styleStack[len(p.styleStack)-1]})
		}
		p.path = p.path[:0]
	}
	return err
}

func (p *svgParser) readGradientURL(v string, defaultColor Pattern) (grad *Gradient, ok bool) {
	if strings.HasPrefix(v, "url(") && strings.HasSuffix(v, ")") {
		urlStr := strings.TrimSpace(v[4 : len(v)-1])
		if strings.HasPrefix(urlStr, "#") {
			var g *Gradient
			g, ok = p.svg.grads[urlStr[1:]]
			if ok {
				g2 := *g
				for _, s := range g2.Stops {
					if s.StopColor != nil {
						continue
					}
					stops := append([]GradStop{}, g2.Stops...)
					g2.Stops = stops
					clr := GetColor(defaultColor)
					for i, s := range stops {
						if s.StopColor == nil {
							g2.Stops[i].StopColor = clr
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

func (p *svgParser) readGradientAttr(attr xml.Attr) error {
	switch attr.Name.Local {
	case "gradientTransform":
		var err error
		if p.grad.Matrix, err = p.parseTransform(attr.Value); err != nil {
			return err
		}
	case "gradientUnits":
		switch strings.TrimSpace(attr.Value) {
		case "userSpaceOnUse":
			p.grad.Units = UserSpaceOnUse
		case "objectBoundingBox":
			p.grad.Units = ObjectBoundingBox
		}
	case "spreadMethod":
		switch strings.TrimSpace(attr.Value) {
		case "pad":
			p.grad.Spread = PadSpread
		case "reflect":
			p.grad.Spread = ReflectSpread
		case "repeat":
			p.grad.Spread = RepeatSpread
		}
	}
	return nil
}

func (p *svgParser) parseUnitToPx(s string, asPerc percentageReference) (float32, error) {
	return p.svg.ViewBox.resolveUnit(s, asPerc)
}
