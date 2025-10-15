package svg

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"golang.org/x/image/math/fixed"
)

const (
	none      = "none"
	round     = "round"
	cubic     = "cubic"
	quadratic = "quadratic"
)

// cursor tracks where we are in the SVG document.
type cursor struct {
	svg        *SVG
	grad       *Gradient
	mask       *Mask
	styleStack []PathStyle
	currentDef []definition
	pathCursor
	inGrad bool
	inDefs bool
	inMask bool
}

// definition is used to store what's given in a def tag
type definition struct {
	ID    string
	Tag   string
	Attrs []xml.Attr
}

func (c *cursor) readTransformAttr(op string, m Matrix2D) (Matrix2D, error) {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "rotate":
		switch len(c.pts) {
		case 1:
			return m.Rotate(c.pts[0] * math.Pi / 180), nil
		case 3:
			return m.Translate(c.pts[1], c.pts[2]).Rotate(c.pts[0]*math.Pi/180).Translate(-c.pts[1], -c.pts[2]), nil
		default:
			return m, errParamMismatch
		}
	case "translate":
		switch len(c.pts) {
		case 1:
			return m.Translate(c.pts[0], 0), nil
		case 2:
			return m.Translate(c.pts[0], c.pts[1]), nil
		default:
			return m, errParamMismatch
		}
	case "skewx":
		switch len(c.pts) {
		case 1:
			return m.SkewX(c.pts[0] * math.Pi / 180), nil
		default:
			return m, errParamMismatch
		}
	case "skewy":
		switch len(c.pts) {
		case 1:
			return m.SkewY(c.pts[0] * math.Pi / 180), nil
		default:
			return m, errParamMismatch
		}
	case "scale":
		switch len(c.pts) {
		case 1:
			return m.Scale(c.pts[0], c.pts[0]), nil
		case 2:
			return m.Scale(c.pts[0], c.pts[1]), nil
		default:
			return m, errParamMismatch
		}
	case "matrix":
		switch len(c.pts) {
		case 6:
			return m.Mult(Matrix2D{
				ScaleX: c.pts[0],
				SkwX:   c.pts[2],
				TransX: c.pts[4],
				SkwY:   c.pts[1],
				ScaleY: c.pts[3],
				TransY: c.pts[5],
			}), nil
		default:
			return m, errParamMismatch
		}
	default:
		return m, errParamMismatch
	}
}

func (c *cursor) parseTransform(v string) (Matrix2D, error) {
	s := strings.Split(v, ")")
	m := c.styleStack[len(c.styleStack)-1].Transform
	for i := len(s) - 1; i >= 0; i-- {
		t := strings.TrimSpace(s[i])
		if t == "" {
			continue
		}
		data := strings.Split(t, "(")
		if len(data) != 2 || len(data[1]) < 1 {
			return m, errParamMismatch
		}
		err := c.addPoints(data[1])
		if err != nil {
			return m, err
		}
		if m, err = c.readTransformAttr(data[0], m); err != nil {
			return m, err
		}
	}
	return m, nil
}

func (c *cursor) parseSelector(v string) (string, error) {
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

func (c *cursor) readStyleAttr(curStyle *PathStyle, k, v string) error {
	v = strings.TrimSpace(v)
	switch strings.TrimSpace(strings.ToLower(k)) {
	case "fill":
		gradient, ok := c.readGradientURL(v, curStyle.FillerColor)
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
		if gradient, ok := c.readGradientURL(v, curStyle.LinerColor); ok {
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
		width, err := c.parseUnit(v, widthPercentage)
		if err != nil {
			return err
		}
		curStyle.LineWidth = width
	case "stroke-dashoffset":
		dashOffset, err := c.parseUnit(v, diagPercentage)
		if err != nil {
			return err
		}
		curStyle.Dash.DashOffset = dashOffset
	case "stroke-dasharray":
		if v != none {
			dashes := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })
			dList := make([]float64, len(dashes))
			for i, dstr := range dashes {
				d, err := c.parseUnit(strings.TrimSpace(dstr), diagPercentage)
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
		m, err := c.parseTransform(v)
		if err != nil {
			return err
		}
		curStyle.Transform = m
	case "mask":
		id, err := c.parseSelector(v)
		if err != nil {
			return err
		}
		curStyle.Masks = append(curStyle.Masks, id)
	}
	return nil
}

func (c *cursor) pushStyle(attrs []xml.Attr) error {
	var pairs []string
	for _, attr := range attrs {
		switch strings.ToLower(attr.Name.Local) {
		case "style":
			pairs = append(pairs, strings.Split(attr.Value, ";")...)
		default:
			pairs = append(pairs, attr.Name.Local+":"+attr.Value)
		}
	}
	s := c.styleStack[len(c.styleStack)-1]
	if len(s.Masks) != 0 {
		// Make a copy of the current masks, so that we don't modify the one below us on the stack
		s.Masks = append([]string{}, s.Masks...)
	}
	for _, pair := range pairs {
		kv := strings.Split(pair, ":")
		if len(kv) >= 2 {
			if err := c.readStyleAttr(&s, kv[0], kv[1]); err != nil {
				return err
			}
		}
	}
	c.styleStack = append(c.styleStack, s)
	return nil
}

func (c *cursor) readStartElement(se xml.StartElement) error {
	var skipDef bool
	if c.inGrad || se.Name.Local == "radialGradient" || se.Name.Local == "linearGradient" {
		skipDef = true
	}
	if !skipDef && c.inDefs {
		id := ""
		for _, attr := range se.Attr {
			if attr.Name.Local == "id" {
				id = attr.Value
			}
		}
		if id != "" && len(c.currentDef) > 0 {
			c.svg.defs[c.currentDef[0].ID] = c.currentDef
			c.currentDef = make([]definition, 0)
		}
		c.currentDef = append(c.currentDef, definition{
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
		err = df(c, se.Attr)
	}
	if len(c.path) > 0 {
		if c.inMask && c.mask != nil {
			c.mask.SvgPaths = append(c.mask.SvgPaths,
				StyledPath{Path: append(Path{}, c.path...), Style: c.styleStack[len(c.styleStack)-1]})
		} else if !c.inMask {
			c.svg.SvgPaths = append(c.svg.SvgPaths,
				StyledPath{Path: append(Path{}, c.path...), Style: c.styleStack[len(c.styleStack)-1]})
		}
		c.path = c.path[:0]
	}
	return err
}
