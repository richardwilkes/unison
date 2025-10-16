// Provides parsing and rendering of SVG images.
// SVG files are parsed into an abstract representation,
// which can then be consumed by painting drivers.
package unison

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/enums/strokecap"
	"github.com/richardwilkes/unison/enums/strokejoin"
	"github.com/richardwilkes/unison/enums/tilemode"
	"golang.org/x/image/math/fixed"
	"golang.org/x/net/html/charset"
)

var errParamMismatch = errors.New("param mismatch")

// SVGData holds data from parsed SVGs.
type SVGData struct {
	Masks         map[string]*SVGMask
	grads         map[string]*Gradient
	defs          map[string][]svgDef
	Paths         []SVGStyledPath
	ViewBox       geom.Rect
	SuggestedSize geom.Size
	Transform     geom.Matrix
}

// SVGPathStyle holds the state of the style.
type SVGPathStyle struct {
	Masks             []string
	FillerColor       Ink
	LinerColor        Ink
	Dash              SVGDashOptions
	Join              SVGJoinOptions
	FillOpacity       float32
	LineOpacity       float32
	LineWidth         float32
	Transform         geom.Matrix
	UseNonZeroWinding bool
}

// SVGStyledPath binds a PathStyle to a Path.
type SVGStyledPath struct {
	Path  SVGPath
	Style SVGPathStyle
}

// SVGMask is the element that defines a mask for the referenced elements.
type SVGMask struct {
	ID        string
	SvgPaths  []SVGStyledPath
	Bounds    geom.Rect
	Transform geom.Matrix
}

type svgDef struct {
	ID    string
	Tag   string
	Attrs []xml.Attr
}

type svgParser struct {
	svg        *SVGData
	grad       *Gradient
	mask       *SVGMask
	styleStack []SVGPathStyle
	currentDef []svgDef
	svgPathParser
	inGrad bool
	inDefs bool
	inMask bool
}

// SVGParse reads the Icon from the given io.Reader
// This only supports a sub-set of SVG, but
// is enough to draw many svgs. errMode determines if the svg ignores, errors out, or logs a warning
// if it does not handle an element found in the svg file.
func SVGParse(stream io.Reader) (*SVGData, error) {
	svg := &SVGData{
		defs:      make(map[string][]svgDef),
		grads:     make(map[string]*Gradient),
		Masks:     make(map[string]*SVGMask),
		Transform: geom.NewIdentityMatrix(),
	}
	p := &svgParser{
		styleStack: []SVGPathStyle{SVGDefaultStyle},
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
					p.currentDef = append(p.currentDef, svgDef{Tag: "endg"})
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
					p.currentDef = make([]svgDef, 0)
				}
				p.inDefs = false
			case "radialGradient", "linearGradient":
				p.inGrad = false
			}
		}
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

func (p *svgParser) readStyleAttr(curStyle *SVGPathStyle, k, v string) error {
	var err error
	v = strings.TrimSpace(v)
	switch strings.TrimSpace(strings.ToLower(k)) {
	case "fill":
		if gradient, ok := p.readGradientURL(v, curStyle.FillerColor); ok {
			curStyle.FillerColor = gradient
		} else if curStyle.FillerColor, err = ColorDecode(v); err != nil {
			return err
		}
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
		} else if curStyle.LinerColor, err = ColorDecode(v); err != nil {
			return err
		}
	case "stroke-linecap":
		switch v {
		case "butt":
			curStyle.Join.TrailLineCap = strokecap.Butt
		case "round":
			curStyle.Join.TrailLineCap = strokecap.Round
		case "square":
			curStyle.Join.TrailLineCap = strokecap.Square
		default:
			slog.Warn("svg: unsupported value for <stroke-linecap>", "value", v)
		}
	case "stroke-linejoin":
		switch v {
		case "miter":
			curStyle.Join.LineJoin = strokejoin.Miter
		case "round":
			curStyle.Join.LineJoin = strokejoin.Round
		case "bevel":
			curStyle.Join.LineJoin = strokejoin.Bevel
		default:
			slog.Warn("svg: unsupported value for <stroke-linejoin>", "value", v)
		}
	case "stroke-miterlimit":
		if curStyle.Join.MiterLimit, err = svgParseBasicFloat(v); err != nil {
			return err
		}
	case "stroke-width":
		if curStyle.LineWidth, err = p.parseUnitToPx(v, svgWidthPercentage); err != nil {
			return err
		}
	case "stroke-dashoffset":
		if curStyle.Dash.DashOffset, err = p.parseUnitToPx(v, svgDiagPercentage); err != nil {
			return err
		}
	case "stroke-dasharray":
		if v != "none" {
			dashes := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' })
			dList := make([]float32, len(dashes))
			for i, dstr := range dashes {
				if dList[i], err = p.parseUnitToPx(strings.TrimSpace(dstr), svgDiagPercentage); err != nil {
					return err
				}
			}
			curStyle.Dash.Dash = dList
		}
	case "opacity":
		var opacity float32
		if opacity, err = svgParseBasicFloat(v); err != nil {
			return err
		}
		curStyle.FillOpacity *= opacity
		curStyle.LineOpacity *= opacity
	case "stroke-opacity":
		var opacity float32
		if opacity, err = svgParseBasicFloat(v); err != nil {
			return err
		}
		curStyle.LineOpacity *= opacity
	case "fill-opacity":
		var opacity float32
		if opacity, err = svgParseBasicFloat(v); err != nil {
			return err
		}
		curStyle.FillOpacity *= opacity
	case "transform":
		if curStyle.Transform, err = p.parseTransform(v); err != nil {
			return err
		}
	case "mask":
		var id string
		if id, err = p.parseSelector(v); err != nil {
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
			p.currentDef = make([]svgDef, 0)
		}
		p.currentDef = append(p.currentDef, svgDef{
			ID:    id,
			Tag:   se.Name.Local,
			Attrs: se.Attr,
		})
		return nil
	}
	df, ok := svgDrawFuncs[se.Name.Local]
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
				SVGStyledPath{Path: append(SVGPath{}, p.path...), Style: p.styleStack[len(p.styleStack)-1]})
		} else if !p.inMask {
			p.svg.Paths = append(p.svg.Paths,
				SVGStyledPath{Path: append(SVGPath{}, p.path...), Style: p.styleStack[len(p.styleStack)-1]})
		}
		p.path = p.path[:0]
	}
	return err
}

func (p *svgParser) readGradientURL(v string, defaultColor Ink) (grad *Gradient, ok bool) {
	if strings.HasPrefix(v, "url(") && strings.HasSuffix(v, ")") {
		urlStr := strings.TrimSpace(v[4 : len(v)-1])
		if strings.HasPrefix(urlStr, "#") {
			var g *Gradient
			g, ok = p.svg.grads[urlStr[1:]]
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

func (p *svgParser) parseUnitToPx(s string, asPerc svgPercentageReference) (float32, error) {
	return svgResolveUnit(p.svg.ViewBox, s, asPerc)
}

type svgPathParser struct {
	pts        []float32
	path       SVGPath
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
}

func (c *svgPathParser) compilePath(svgPath string) error {
	c.placeX = 0
	c.placeY = 0
	c.pts = c.pts[0:0]
	c.lastKey = ' '
	c.path.Clear()
	c.inPath = false
	lastIndex := -1
	for i, v := range svgPath {
		if unicode.IsLetter(v) && v != 'e' {
			if lastIndex != -1 {
				if err := c.addSegment(svgPath[lastIndex:i]); err != nil {
					return err
				}
			}
			lastIndex = i
		}
	}
	if lastIndex != -1 {
		if err := c.addSegment(svgPath[lastIndex:]); err != nil {
			return err
		}
	}
	return nil
}

func (c *svgPathParser) valsToAbs(last float32) {
	for i := 0; i < len(c.pts); i++ {
		last += c.pts[i]
		c.pts[i] = last
	}
}

func (c *svgPathParser) pointsToAbs(sz int) {
	lastX := c.placeX
	lastY := c.placeY
	for j := 0; j < len(c.pts); j += sz {
		for i := 0; i < sz; i += 2 {
			c.pts[i+j] += lastX
			c.pts[i+1+j] += lastY
		}
		lastX = c.pts[(j+sz)-2]
		lastY = c.pts[(j+sz)-1]
	}
}

func (c *svgPathParser) hasSetsOrMore(sz int, rel bool) bool {
	if len(c.pts) < sz || len(c.pts)%sz != 0 {
		return false
	}
	if rel {
		c.pointsToAbs(sz)
	}
	return true
}

func (c *svgPathParser) addPoints(dataPoints string) error {
	lastIndex := -1
	c.pts = c.pts[0:0]
	lr := ' '
	for i, r := range dataPoints {
		if !unicode.IsNumber(r) && r != '.' && (r != '-' || lr != 'e') && r != 'e' {
			if lastIndex != -1 {
				if err := c.readFloatIntoPts(dataPoints[lastIndex:i]); err != nil {
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
		if err := c.readFloatIntoPts(dataPoints[lastIndex:]); err != nil {
			return err
		}
	}
	return nil
}

func (c *svgPathParser) readFloatIntoPts(numStr string) error {
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
		c.pts = append(c.pts, f)
		last = i
	}
	f, err := svgParseBasicFloat(numStr[last:])
	if err != nil {
		return err
	}
	c.pts = append(c.pts, f)
	return nil
}

func (c *svgPathParser) addSegment(segString string) error {
	if err := c.addPoints(segString[1:]); err != nil {
		return err
	}
	l := len(c.pts)
	k := segString[0]
	rel := false
	switch k {
	case 'Z', 'z':
		if len(c.pts) != 0 {
			return errParamMismatch
		}
		if c.inPath {
			c.path.Stop(true)
			c.placeX = c.pathStartX
			c.placeY = c.pathStartY
			c.inPath = false
		}
	case 'm':
		rel = true
		fallthrough
	case 'M':
		if !c.hasSetsOrMore(2, rel) {
			return errParamMismatch
		}
		c.pathStartX = c.pts[0]
		c.pathStartY = c.pts[1]
		c.inPath = true
		c.path.Start(fixed.Point26_6{
			X: fixed.Int26_6((c.pathStartX + c.curX) * 64),
			Y: fixed.Int26_6((c.pathStartY + c.curY) * 64),
		})
		for i := 2; i < l-1; i += 2 {
			c.path.Line(fixed.Point26_6{
				X: fixed.Int26_6((c.pts[i] + c.curX) * 64),
				Y: fixed.Int26_6((c.pts[i+1] + c.curY) * 64),
			})
		}
		c.placeX = c.pts[l-2]
		c.placeY = c.pts[l-1]
	case 'l':
		rel = true
		fallthrough
	case 'L':
		if !c.hasSetsOrMore(2, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-1; i += 2 {
			c.path.Line(fixed.Point26_6{
				X: fixed.Int26_6((c.pts[i] + c.curX) * 64),
				Y: fixed.Int26_6((c.pts[i+1] + c.curY) * 64),
			})
		}
		c.placeX = c.pts[l-2]
		c.placeY = c.pts[l-1]
	case 'v':
		c.valsToAbs(c.placeY)
		fallthrough
	case 'V':
		if !c.hasSetsOrMore(1, false) {
			return errParamMismatch
		}
		for _, p := range c.pts {
			c.path.Line(fixed.Point26_6{
				X: fixed.Int26_6((c.placeX + c.curX) * 64),
				Y: fixed.Int26_6((p + c.curY) * 64),
			})
		}
		c.placeY = c.pts[l-1]
	case 'h':
		c.valsToAbs(c.placeX)
		fallthrough
	case 'H':
		if !c.hasSetsOrMore(1, false) {
			return errParamMismatch
		}
		for _, p := range c.pts {
			c.path.Line(fixed.Point26_6{
				X: fixed.Int26_6((p + c.curX) * 64),
				Y: fixed.Int26_6((c.placeY + c.curY) * 64),
			})
		}
		c.placeX = c.pts[l-1]
	case 'q':
		rel = true
		fallthrough
	case 'Q':
		if !c.hasSetsOrMore(4, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-3; i += 4 {
			c.path.QuadBezier(
				fixed.Point26_6{
					X: fixed.Int26_6((c.pts[i] + c.curX) * 64),
					Y: fixed.Int26_6((c.pts[i+1] + c.curY) * 64),
				},
				fixed.Point26_6{
					X: fixed.Int26_6((c.pts[i+2] + c.curX) * 64),
					Y: fixed.Int26_6((c.pts[i+3] + c.curY) * 64),
				},
			)
		}
		c.cntlPtX, c.cntlPtY = c.pts[l-4], c.pts[l-3]
		c.placeX = c.pts[l-2]
		c.placeY = c.pts[l-1]
	case 't':
		rel = true
		fallthrough
	case 'T':
		if !c.hasSetsOrMore(2, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-1; i += 2 {
			c.reflectControl(true)
			c.path.QuadBezier(
				fixed.Point26_6{
					X: fixed.Int26_6((c.cntlPtX + c.curX) * 64),
					Y: fixed.Int26_6((c.cntlPtY + c.curY) * 64),
				},
				fixed.Point26_6{
					X: fixed.Int26_6((c.pts[i] + c.curX) * 64),
					Y: fixed.Int26_6((c.pts[i+1] + c.curY) * 64),
				},
			)
			c.lastKey = k
			c.placeX = c.pts[i]
			c.placeY = c.pts[i+1]
		}
	case 'c':
		rel = true
		fallthrough
	case 'C':
		if !c.hasSetsOrMore(6, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-5; i += 6 {
			c.path.CubeBezier(
				fixed.Point26_6{
					X: fixed.Int26_6((c.pts[i] + c.curX) * 64),
					Y: fixed.Int26_6((c.pts[i+1] + c.curY) * 64),
				},
				fixed.Point26_6{
					X: fixed.Int26_6((c.pts[i+2] + c.curX) * 64),
					Y: fixed.Int26_6((c.pts[i+3] + c.curY) * 64),
				},
				fixed.Point26_6{
					X: fixed.Int26_6((c.pts[i+4] + c.curX) * 64),
					Y: fixed.Int26_6((c.pts[i+5] + c.curY) * 64),
				},
			)
		}
		c.cntlPtX, c.cntlPtY = c.pts[l-4], c.pts[l-3]
		c.placeX = c.pts[l-2]
		c.placeY = c.pts[l-1]
	case 's':
		rel = true
		fallthrough
	case 'S':
		if !c.hasSetsOrMore(4, rel) {
			return errParamMismatch
		}
		for i := 0; i < l-3; i += 4 {
			c.reflectControl(false)
			c.path.CubeBezier(
				fixed.Point26_6{
					X: fixed.Int26_6((c.cntlPtX + c.curX) * 64),
					Y: fixed.Int26_6((c.cntlPtY + c.curY) * 64),
				},
				fixed.Point26_6{
					X: fixed.Int26_6((c.pts[i] + c.curX) * 64),
					Y: fixed.Int26_6((c.pts[i+1] + c.curY) * 64),
				},
				fixed.Point26_6{
					X: fixed.Int26_6((c.pts[i+2] + c.curX) * 64),
					Y: fixed.Int26_6((c.pts[i+3] + c.curY) * 64),
				},
			)
			c.lastKey = k
			c.cntlPtX, c.cntlPtY = c.pts[i], c.pts[i+1]
			c.placeX = c.pts[i+2]
			c.placeY = c.pts[i+3]
		}
	case 'a', 'A':
		if !c.hasSetsOrMore(7, false) {
			return errParamMismatch
		}
		for i := 0; i < l-6; i += 7 {
			if k == 'a' {
				c.pts[i+5] += c.placeX
				c.pts[i+6] += c.placeY
			}
			c.addArcFromA(c.pts[i:])
		}
	default:
		slog.Warn("Ignoring unknown svg path command", "command", string(k))
	}
	c.lastKey = k
	return nil
}

func (c *svgPathParser) reflectControl(forQuad bool) {
	if (forQuad && (c.lastKey == 'q' || c.lastKey == 'Q' || c.lastKey == 'T' || c.lastKey == 't')) ||
		(!forQuad && (c.lastKey == 'c' || c.lastKey == 'C' || c.lastKey == 's' || c.lastKey == 'S')) {
		c.cntlPtX = c.placeX*2 - c.cntlPtX
		c.cntlPtY = c.placeY*2 - c.cntlPtY
	} else {
		c.cntlPtX, c.cntlPtY = c.placeX, c.placeY
	}
}

func (c *svgPathParser) ellipseAt(cx, cy, rx, ry float32) {
	c.placeX, c.placeY = cx+rx, cy
	c.pts = c.pts[0:0]
	c.pts = append(c.pts, rx, ry, 0.0, 1.0, 0.0, c.placeX, c.placeY)
	c.path.Start(fixed.Point26_6{
		X: fixed.Int26_6(c.placeX * 64),
		Y: fixed.Int26_6(c.placeY * 64),
	})
	c.placeX, c.placeY = c.path.addArc(c.pts, cx, cy, c.placeX, c.placeY)
	c.path.Stop(true)
}

func (c *svgPathParser) addArcFromA(points []float32) {
	cx, cy := svgFindEllipseCenter(&points[0], &points[1], points[2]*math.Pi/180, c.placeX,
		c.placeY, points[5], points[6], points[4] == 0, points[3] == 0)
	c.placeX, c.placeY = c.path.addArc(c.pts, cx+c.curX, cy+c.curY, c.placeX+c.curX, c.placeY+c.curY)
}

// SVGOp groups the different SVG commands
type SVGOp interface {
	// SVG text representation of the command
	fmt.Stringer
}

// SVGOpMoveTo moves the current point.
type SVGOpMoveTo fixed.Point26_6

// SVGOpLineTo draws a line from the current point,
// and updates it.
type SVGOpLineTo fixed.Point26_6

// SVGOpQuadTo draws a quadratic Bezier curve from the current point,
// and updates it.
type SVGOpQuadTo [2]fixed.Point26_6

// SVGOpCubicTo draws a cubic Bezier curve from the current point,
// and updates it.
type SVGOpCubicTo [3]fixed.Point26_6

// SVGOpClose close the current path.
type SVGOpClose struct{}

func (op SVGOpMoveTo) String() string {
	return fmt.Sprintf("M%4.3f,%4.3f", float32(op.X)/64, float32(op.Y)/64)
}

func (op SVGOpLineTo) String() string {
	return fmt.Sprintf("L%4.3f,%4.3f", float32(op.X)/64, float32(op.Y)/64)
}

func (op SVGOpQuadTo) String() string {
	return fmt.Sprintf("Q%4.3f,%4.3f,%4.3f,%4.3f", float32(op[0].X)/64, float32(op[0].Y)/64,
		float32(op[1].X)/64, float32(op[1].Y)/64)
}

func (op SVGOpCubicTo) String() string {
	return "C" + fmt.Sprintf("C%4.3f,%4.3f,%4.3f,%4.3f,%4.3f,%4.3f", float32(op[0].X)/64, float32(op[0].Y)/64,
		float32(op[1].X)/64, float32(op[1].Y)/64, float32(op[2].X)/64, float32(op[2].Y)/64)
}

func (op SVGOpClose) String() string {
	return "Z"
}

// SVGPath describes a sequence of basic SVG operations, which should not be nil
// Higher-level shapes may be reduced to a path.
type SVGPath []SVGOp

// ToSVGPath returns a string representation of the path
func (p SVGPath) ToSVGPath() string {
	chunks := make([]string, len(p))
	for i, op := range p {
		chunks[i] = op.String()
	}
	return strings.Join(chunks, " ")
}

// String returns a readable representation of a Path.
func (p SVGPath) String() string {
	return p.ToSVGPath()
}

// Clear zeros the path slice
func (p *SVGPath) Clear() {
	*p = (*p)[:0]
}

// Start starts a new curve at the given point.
func (p *SVGPath) Start(a fixed.Point26_6) {
	*p = append(*p, SVGOpMoveTo{a.X, a.Y})
}

// Line adds a linear segment to the current curve.
func (p *SVGPath) Line(b fixed.Point26_6) {
	*p = append(*p, SVGOpLineTo{b.X, b.Y})
}

// QuadBezier adds a quadratic segment to the current curve.
func (p *SVGPath) QuadBezier(b, c fixed.Point26_6) {
	*p = append(*p, SVGOpQuadTo{b, c})
}

// CubeBezier adds a cubic segment to the current curve.
func (p *SVGPath) CubeBezier(b, c, d fixed.Point26_6) {
	*p = append(*p, SVGOpCubicTo{b, c, d})
}

// Stop joins the ends of the path
func (p *SVGPath) Stop(closeLoop bool) {
	if closeLoop {
		*p = append(*p, SVGOpClose{})
	}
}

// addRoundRect adds a rectangle of the indicated size with rounded corners of radius rx in the x axis and ry in the y
// axis.
func (p *SVGPath) addRoundRect(minX, minY, maxX, maxY, rx, ry float32) {
	if rx <= 0 || ry <= 0 {
		cx := (minX + maxX) / 2
		cy := (minY + maxY) / 2
		q := &svgMatrixAdder{M: geom.NewTranslationMatrix(cx, cy).Translate(-cx, -cy), path: p}
		q.Start(toSVGFixedPt(minX, minY))
		q.Line(toSVGFixedPt(maxX, minY))
		q.Line(toSVGFixedPt(maxX, maxY))
		q.Line(toSVGFixedPt(minX, maxY))
		q.path.Stop(true)
		return
	}

	w := maxX - minX
	if w < rx*2 {
		rx = w / 2
	}
	h := maxY - minY
	if h < ry*2 {
		ry = h / 2
	}
	stretch := rx / ry
	midY := minY + h/2

	q := &svgMatrixAdder{M: geom.NewTranslationMatrix(minX+w/2, midY).Scale(1, 1/stretch).Translate(-minX-w/2, -minY-h/2), path: p}
	maxY = midY + h/2*stretch
	minY = midY - h/2*stretch

	q.Start(toSVGFixedPt(minX+rx, minY))
	q.Line(toSVGFixedPt(maxX-rx, minY))
	svgRoundGap(q, toSVGFixedPt(maxX-rx, minY+rx), toSVGFixedPt(0, -rx), toSVGFixedPt(rx, 0))
	q.Line(toSVGFixedPt(maxX, maxY-rx))
	svgRoundGap(q, toSVGFixedPt(maxX-rx, maxY-rx), toSVGFixedPt(rx, 0), toSVGFixedPt(0, rx))
	q.Line(toSVGFixedPt(minX+rx, maxY))
	svgRoundGap(q, toSVGFixedPt(minX+rx, maxY-rx), toSVGFixedPt(0, rx), toSVGFixedPt(-rx, 0))
	q.Line(toSVGFixedPt(minX, minY+rx))
	svgRoundGap(q, toSVGFixedPt(minX+rx, minY+rx), toSVGFixedPt(-rx, 0), toSVGFixedPt(0, -rx))
	q.path.Stop(true)
}

// addArc adds an arc to the adder p
func (p *SVGPath) addArc(points []float32, cx, cy, px, py float32) (lx, ly float32) {
	rotX := points[2] * math.Pi / 180 // Convert degress to radians
	largeArc := points[3] != 0
	sweep := points[4] != 0
	startAngle := xmath.Atan2(py-cy, px-cx) - rotX
	endAngle := xmath.Atan2(points[6]-cy, points[5]-cx) - rotX
	deltaTheta := endAngle - startAngle
	arcBig := xmath.Abs(deltaTheta) > math.Pi

	// Approximate ellipse using cubic bezeir splines
	etaStart := xmath.Atan2(xmath.Sin(startAngle)/points[1], xmath.Cos(startAngle)/points[0])
	etaEnd := xmath.Atan2(xmath.Sin(endAngle)/points[1], xmath.Cos(endAngle)/points[0])
	deltaEta := etaEnd - etaStart
	if (arcBig && !largeArc) || (!arcBig && largeArc) { // Go has no boolean XOR
		if deltaEta < 0 {
			deltaEta += math.Pi * 2
		} else {
			deltaEta -= math.Pi * 2
		}
	}
	// This check might be needed if the center point of the elipse is
	// at the midpoint of the start and end lines.
	if deltaEta < 0 && sweep {
		deltaEta += math.Pi * 2
	} else if deltaEta >= 0 && !sweep {
		deltaEta -= math.Pi * 2
	}

	// Round up to determine number of cubic splines to approximate bezier curve
	segs := int(xmath.Abs(deltaEta)/svgMaxDx) + 1
	dEta := deltaEta / float32(segs) // span of each segment
	// Approximate the ellipse using a set of cubic bezier curves by the method of
	// L. Maisonobe, "Drawing an elliptical arc using polylines, quadratic
	// or cubic Bezier curves", 2003
	// https://www.spaceroots.org/documents/elllipse/elliptical-arc.pdf
	tde := xmath.Tan(dEta / 2)
	alpha := xmath.Sin(dEta) * (xmath.Sqrt(4+3*tde*tde) - 1) / 3 // Math is fun!
	lx, ly = px, py
	sinTheta, cosTheta := xmath.Sin(rotX), xmath.Cos(rotX)
	ldx, ldy := svgEllipsePrime(points[0], points[1], sinTheta, cosTheta, etaStart)
	for i := 1; i <= segs; i++ {
		eta := etaStart + dEta*float32(i)
		if i == segs {
			px, py = points[5], points[6] // Just makes the end point exact; no roundoff error
		} else {
			px, py = svgEllipsePointAt(points[0], points[1], sinTheta, cosTheta, eta, cx, cy)
		}
		dx, dy := svgEllipsePrime(points[0], points[1], sinTheta, cosTheta, eta)
		p.CubeBezier(toSVGFixedPt(lx+alpha*ldx, ly+alpha*ldy),
			toSVGFixedPt(px-alpha*dx, py-alpha*dy), toSVGFixedPt(px, py))
		lx, ly, ldx, ldy = px, py, dx, dy
	}
	return lx, ly
}

// svgRoundGap bridges miter-limit gaps with a circular arc
func svgRoundGap(p *svgMatrixAdder, a, tNorm, lNorm fixed.Point26_6) {
	svgAddArc(p, a, a.Add(tNorm), a.Add(lNorm), true, 0, 0, p.Line)
	p.Line(a.Add(lNorm)) // just to be sure line joins cleanly,
	// last pt in stoke arc may not be precisely s2
}

// svgMatrixAdder add points to path after applying a matrix M to all points
type svgMatrixAdder struct {
	path *SVGPath
	M    geom.Matrix
}

// Start starts a new path.
func (m *svgMatrixAdder) Start(a fixed.Point26_6) {
	m.path.Start(m.transformFixed(a))
}

// Line adds a linear segment to the current curve.
func (m *svgMatrixAdder) Line(b fixed.Point26_6) {
	m.path.Line(m.transformFixed(b))
}

// CubeBezier adds a cubic segment to the current curve.
func (m *svgMatrixAdder) CubeBezier(b, c, d fixed.Point26_6) {
	m.path.CubeBezier(m.transformFixed(b), m.transformFixed(c), m.transformFixed(d))
}

// TFixed transforms a fixed.Point26_6 by the matrix.
func (m *svgMatrixAdder) transformFixed(pt fixed.Point26_6) fixed.Point26_6 {
	return fixed.Point26_6{
		X: fixed.Int26_6((float32(pt.X)*m.M.ScaleX + float32(pt.Y)*m.M.SkewX) + m.M.TransX*64),
		Y: fixed.Int26_6((float32(pt.X)*m.M.SkewY + float32(pt.Y)*m.M.ScaleY) + m.M.TransY*64),
	}
}

const (
	svgCubicsPerHalfCircle = 8 // Number of cubic beziers to approx half a circle

	// fixed point t parameterization shift factor;
	// (2^this)/64 is the max length of t for fixed.Int26_6
	svgTStrokeShift = 14

	// svgMaxDx is the maximum radians a cubic splice is allowed to span
	// in ellipse parametric when approximating an off-axis ellipse.
	svgMaxDx float32 = math.Pi / 8
)

func toSVGFixedPt(x, y float32) (p fixed.Point26_6) {
	return fixed.Point26_6{
		X: fixed.Int26_6(x * 64),
		Y: fixed.Int26_6(y * 64),
	}
}

// svgLength is the distance from the origin of the point
func svgLength(v fixed.Point26_6) fixed.Int26_6 {
	vx := float32(v.X)
	vy := float32(v.Y)
	return fixed.Int26_6(xmath.Sqrt(vx*vx + vy*vy))
}

// svgAddArc strokes a circular arc by approximation with bezier curves
func svgAddArc(p *svgMatrixAdder, a, s1, s2 fixed.Point26_6, clockwise bool, trimStart, trimEnd fixed.Int26_6, firstPoint func(p fixed.Point26_6)) (ps1, ds1, ps2, ds2 fixed.Point26_6) {
	// Approximate the circular arc using a set of cubic bezier curves by the method of L. Maisonobe, "Drawing an
	// elliptical arc using polylines, quadratic or cubic Bezier curves", 2003
	// https://www.spaceroots.org/documents/elllipse/elliptical-arc.pdf The method was simplified for circles.
	theta1 := xmath.Atan2(float32(s1.Y-a.Y), float32(s1.X-a.X))
	theta2 := xmath.Atan2(float32(s2.Y-a.Y), float32(s2.X-a.X))
	if !clockwise {
		for theta1 < theta2 {
			theta1 += math.Pi * 2
		}
	} else {
		for theta2 < theta1 {
			theta2 += math.Pi * 2
		}
	}
	deltaTheta := theta2 - theta1
	if trimStart > 0 {
		ds := (deltaTheta * float32(trimStart)) / float32(1<<svgTStrokeShift)
		deltaTheta -= ds
		theta1 += ds
	}
	if trimEnd > 0 {
		ds := (deltaTheta * float32(trimEnd)) / float32(1<<svgTStrokeShift)
		deltaTheta -= ds
	}
	segs := int(xmath.Abs(deltaTheta)/(math.Pi/svgCubicsPerHalfCircle)) + 1
	dTheta := deltaTheta / float32(segs)
	tde := xmath.Tan(dTheta / 2)
	alpha := fixed.Int26_6(xmath.Sin(dTheta) * (xmath.Sqrt(4+3*tde*tde) - 1) * (64.0 / 3.0)) // Math is fun!
	r := float32(svgLength(s1.Sub(a)))                                                       // Note r is *64
	ldp := fixed.Point26_6{X: -fixed.Int26_6(r * xmath.Sin(theta1)), Y: fixed.Int26_6(r * xmath.Cos(theta1))}
	ds1 = ldp
	ps1 = fixed.Point26_6{X: a.X + ldp.Y, Y: a.Y - ldp.X}
	firstPoint(ps1)
	s1 = ps1
	for i := 1; i <= segs; i++ {
		eta := theta1 + dTheta*float32(i)
		ds2 = fixed.Point26_6{X: -fixed.Int26_6(r * xmath.Sin(eta)), Y: fixed.Int26_6(r * xmath.Cos(eta))}
		ps2 = fixed.Point26_6{X: a.X + ds2.Y, Y: a.Y - ds2.X} // Using deriviative to calc new pt, because circle
		p1 := s1.Add(ldp.Mul(alpha))
		p2 := ps2.Sub(ds2.Mul(alpha))
		p.CubeBezier(p1, p2, ps2)
		s1, ldp = ps2, ds2
	}
	return ps1, ds1, ps2, ds2
}

// svgEllipsePrime gives tangent vectors for parameterized elipse; a, b, radii, eta parameter
func svgEllipsePrime(a, b, sinTheta, cosTheta, eta float32) (px, py float32) {
	bCosEta := b * xmath.Cos(eta)
	aSinEta := a * xmath.Sin(eta)
	return -aSinEta*cosTheta - bCosEta*sinTheta, -aSinEta*sinTheta + bCosEta*cosTheta
}

// svgEllipsePointAt gives points for parameterized elipse; a, b, radii, eta parameter, center cx, cy
func svgEllipsePointAt(a, b, sinTheta, cosTheta, eta, cx, cy float32) (px, py float32) {
	aCosEta := a * xmath.Cos(eta)
	bSinEta := b * xmath.Sin(eta)
	return cx + aCosEta*cosTheta - bSinEta*sinTheta, cy + aCosEta*sinTheta + bSinEta*cosTheta
}

// svgFindEllipseCenter locates the center of the Ellipse if it exists. If it does not exist,
// the radius values will be increased minimally for a solution to be possible
// while preserving the ra to rb ratio.  ra and rb arguments are pointers that can be
// checked after the call to see if the values changed. This method uses coordinate transformations
// to reduce the problem to finding the center of a circle that includes the origin
// and an arbitrary point. The center of the circle is then transformed
// back to the original coordinates and returned.
func svgFindEllipseCenter(ra, rb *float32, rotX, startX, startY, endX, endY float32, sweep, smallArc bool) (cx, cy float32) {
	cos, sin := xmath.Cos(rotX), xmath.Sin(rotX)

	// Move origin to start point
	nx, ny := endX-startX, endY-startY

	// Rotate ellipse x-axis to coordinate x-axis
	nx, ny = nx*cos+ny*sin, -nx*sin+ny*cos
	// Scale X dimension so that ra = rb
	nx *= *rb / *ra // Now the ellipse is a circle radius rb; therefore foci and center coincide

	midX, midY := nx/2, ny/2
	midlenSq := midX*midX + midY*midY

	var hr float32
	if *rb**rb < midlenSq {
		// Requested ellipse does not exist; scale ra, rb to fit. Length of
		// span is greater than max width of ellipse, must scale *ra, *rb
		nrb := xmath.Sqrt(midlenSq)
		if *ra == *rb {
			*ra = nrb // prevents roundoff
		} else {
			*ra = *ra * nrb / *rb
		}
		*rb = nrb
	} else {
		hr = xmath.Sqrt(*rb**rb-midlenSq) / xmath.Sqrt(midlenSq)
	}
	// Notice that if hr is zero, both answers are the same.
	if (sweep && smallArc) || (!sweep && !smallArc) {
		cx = midX + midY*hr
		cy = midY - midX*hr
	} else {
		cx = midX - midY*hr
		cy = midY + midX*hr
	}

	// reverse scale
	cx *= *ra / *rb
	// Reverse rotate and translate back to original coordinates
	return cx*cos - cy*sin + startX, cx*sin + cy*cos + startY
}

var errZeroLengthID = errors.New("zero length id")

func init() {
	svgDrawFuncs["use"] = svgUseF // Can't be done statically, since useF uses drawFuncs
}

type svgFunc func(c *svgParser, attrs []xml.Attr) error

var svgDrawFuncs = map[string]svgFunc{
	"svg":            svgF,
	"g":              nil,
	"line":           svgLineF,
	"stop":           svgStopF,
	"rect":           svgRectF,
	"circle":         svgCircleF,
	"ellipse":        svgCircleF, // circleF handles ellipse also
	"polyline":       svgPolylineF,
	"polygon":        svgPolygonF,
	"path":           svgPathF,
	"desc":           nil,
	"defs":           svgDefsF,
	"title":          nil,
	"linearGradient": svgLinearGradientF,
	"radialGradient": svgRadialGradientF,
	"mask":           svgMaskF,
}

func svgF(c *svgParser, attrs []xml.Attr) error {
	c.svg.ViewBox = geom.Rect{}
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "viewBox":
			if err := c.addPoints(attr.Value); err != nil {
				return err
			}
			if len(c.pts) != 4 {
				return errParamMismatch
			}
			c.svg.ViewBox.X = c.pts[0]
			c.svg.ViewBox.Y = c.pts[1]
			c.svg.ViewBox.Width = c.pts[2]
			c.svg.ViewBox.Height = c.pts[3]
		case "width": //nolint:goconst // Can't use const named width
			width, err := svgParseBasicFloat(attr.Value)
			if err != nil {
				return err
			}
			c.svg.SuggestedSize.Width = width
		case "height": //nolint:goconst // Can't use const named height
			height, err := svgParseBasicFloat(attr.Value)
			if err != nil {
				return err
			}
			c.svg.SuggestedSize.Height = height
		}
	}
	if c.svg.ViewBox.Width == 0 {
		c.svg.ViewBox.Width = c.svg.SuggestedSize.Width
	}
	if c.svg.SuggestedSize.Width == 0 {
		c.svg.SuggestedSize.Width = c.svg.ViewBox.Width
	}
	if c.svg.ViewBox.Height == 0 {
		c.svg.ViewBox.Height = c.svg.SuggestedSize.Height
	}
	if c.svg.SuggestedSize.Height == 0 {
		c.svg.SuggestedSize.Height = c.svg.ViewBox.Height
	}
	return nil
}

func svgRectF(c *svgParser, attrs []xml.Attr) error {
	var x, y, w, h, rx, ry float32
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "x":
			x, err = c.parseUnitToPx(attr.Value, svgWidthPercentage)
		case "y":
			y, err = c.parseUnitToPx(attr.Value, svgHeightPercentage)
		case "width":
			w, err = c.parseUnitToPx(attr.Value, svgWidthPercentage)
		case "height":
			h, err = c.parseUnitToPx(attr.Value, svgHeightPercentage)
		case "rx":
			rx, err = c.parseUnitToPx(attr.Value, svgWidthPercentage)
		case "ry":
			ry, err = c.parseUnitToPx(attr.Value, svgHeightPercentage)
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
	c.path.addRoundRect(x+c.curX, y+c.curY, w+x+c.curX, h+y+c.curY, rx, ry)
	return nil
}

func svgMaskF(c *svgParser, attrs []xml.Attr) error {
	var mask SVGMask
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "id":
			if attr.Value == "" {
				return errZeroLengthID
			}
			mask.ID = attr.Value
		case "x":
			mask.Bounds.X, err = c.parseUnitToPx(attr.Value, svgWidthPercentage)
		case "y":
			mask.Bounds.Y, err = c.parseUnitToPx(attr.Value, svgHeightPercentage)
		case "width":
			mask.Bounds.Width, err = c.parseUnitToPx(attr.Value, svgWidthPercentage)
		case "height":
			mask.Bounds.Height, err = c.parseUnitToPx(attr.Value, svgHeightPercentage)
		}
		if err != nil {
			return err
		}
	}
	mask.Transform = geom.NewIdentityMatrix()
	c.inMask = true
	c.mask = &mask
	return nil
}

func svgCircleF(c *svgParser, attrs []xml.Attr) error {
	var cx, cy, rx, ry float32
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "cx":
			cx, err = c.parseUnitToPx(attr.Value, svgWidthPercentage)
		case "cy":
			cy, err = c.parseUnitToPx(attr.Value, svgHeightPercentage)
		case "r":
			rx, err = c.parseUnitToPx(attr.Value, svgDiagPercentage)
			ry = rx
		case "rx":
			rx, err = c.parseUnitToPx(attr.Value, svgWidthPercentage)
		case "ry":
			ry, err = c.parseUnitToPx(attr.Value, svgHeightPercentage)
		}
		if err != nil {
			return err
		}
	}
	if rx == 0 || ry == 0 { // not drawn, but not an error
		return nil
	}
	c.ellipseAt(cx+c.curX, cy+c.curY, rx, ry)
	return nil
}

func svgLineF(c *svgParser, attrs []xml.Attr) error {
	var x1, x2, y1, y2 float32
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "x1":
			x1, err = c.parseUnitToPx(attr.Value, svgWidthPercentage)
		case "x2":
			x2, err = c.parseUnitToPx(attr.Value, svgWidthPercentage)
		case "y1":
			y1, err = c.parseUnitToPx(attr.Value, svgHeightPercentage)
		case "y2":
			y2, err = c.parseUnitToPx(attr.Value, svgHeightPercentage)
		}
		if err != nil {
			return err
		}
	}
	c.path.Start(fixed.Point26_6{
		X: fixed.Int26_6((x1 + c.curX) * 64),
		Y: fixed.Int26_6((y1 + c.curY) * 64),
	})
	c.path.Line(fixed.Point26_6{
		X: fixed.Int26_6((x2 + c.curX) * 64),
		Y: fixed.Int26_6((y2 + c.curY) * 64),
	})
	return nil
}

func svgPolylineF(c *svgParser, attrs []xml.Attr) error {
	for _, attr := range attrs {
		if attr.Name.Local != "points" {
			continue
		}
		if err := c.addPoints(attr.Value); err != nil {
			return err
		}
		if len(c.pts)%2 != 0 {
			return errors.New("polygon has odd number of points")
		}
	}
	if len(c.pts) > 4 {
		c.path.Start(fixed.Point26_6{
			X: fixed.Int26_6((c.pts[0] + c.curX) * 64),
			Y: fixed.Int26_6((c.pts[1] + c.curY) * 64),
		})
		for i := 2; i < len(c.pts)-1; i += 2 {
			c.path.Line(fixed.Point26_6{
				X: fixed.Int26_6((c.pts[i] + c.curX) * 64),
				Y: fixed.Int26_6((c.pts[i+1] + c.curY) * 64),
			})
		}
	}
	return nil
}

func svgPolygonF(c *svgParser, attrs []xml.Attr) error {
	err := svgPolylineF(c, attrs)
	if len(c.pts) > 4 {
		c.path.Stop(true)
	}
	return err
}

func svgPathF(c *svgParser, attrs []xml.Attr) error {
	for _, attr := range attrs {
		if attr.Name.Local != "d" {
			continue
		}
		if err := c.compilePath(attr.Value); err != nil {
			return err
		}
	}
	return nil
}

func svgDefsF(c *svgParser, _ []xml.Attr) error {
	c.inDefs = true
	return nil
}

func svgLinearGradientF(c *svgParser, attrs []xml.Attr) error {
	userSpaceOnUse := false
	x1 := "0%"
	y1 := x1
	x2 := "100%"
	y2 := x1
	c.inGrad = true
	c.grad = &Gradient{Transform: geom.NewIdentityMatrix()}
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "id":
			if attr.Value == "" {
				return errZeroLengthID
			}
			c.svg.grads[attr.Value] = c.grad
		case "x1":
			x1 = attr.Value
		case "y1":
			y1 = attr.Value
		case "x2":
			x2 = attr.Value
		case "y2":
			y2 = attr.Value
		default:
			if err := c.readCommonGradientAttrs(attr, &userSpaceOnUse); err != nil {
				return err
			}
		}
	}
	bbox := geom.NewRect(0, 0, 1, 1)
	if userSpaceOnUse {
		bbox = c.svg.ViewBox
	}
	var err error
	c.grad.Start.X, err = svgResolveUnit(bbox, x1, svgWidthPercentage)
	if err != nil {
		return err
	}
	c.grad.Start.Y, err = svgResolveUnit(bbox, y1, svgHeightPercentage)
	if err != nil {
		return err
	}
	c.grad.End.X, err = svgResolveUnit(bbox, x2, svgWidthPercentage)
	if err != nil {
		return err
	}
	c.grad.End.Y, err = svgResolveUnit(bbox, y2, svgHeightPercentage)
	if err != nil {
		return err
	}
	c.normalizeGradientStartEnd()
	return nil
}

func svgRadialGradientF(c *svgParser, attrs []xml.Attr) error {
	userSpaceOnUse := false
	cx := "50%"
	cy := cx
	fx := ""
	fy := ""
	r := cx
	fr := cx
	c.inGrad = true
	c.grad = &Gradient{Transform: geom.NewIdentityMatrix()}
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "id":
			if attr.Value == "" {
				return errZeroLengthID
			}
			c.svg.grads[attr.Value] = c.grad
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
			if err := c.readCommonGradientAttrs(attr, &userSpaceOnUse); err != nil {
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
		bbox = c.svg.ViewBox
	}
	var err error
	c.grad.Start.X, err = svgResolveUnit(bbox, cx, svgWidthPercentage)
	if err != nil {
		return err
	}
	c.grad.Start.Y, err = svgResolveUnit(bbox, cy, svgHeightPercentage)
	if err != nil {
		return err
	}
	c.grad.End.X, err = svgResolveUnit(bbox, fx, svgWidthPercentage)
	if err != nil {
		return err
	}
	c.grad.End.Y, err = svgResolveUnit(bbox, fy, svgHeightPercentage)
	if err != nil {
		return err
	}
	c.grad.StartRadius, err = svgResolveUnit(bbox, r, svgDiagPercentage)
	if err != nil {
		return err
	}
	c.grad.EndRadius, err = svgResolveUnit(bbox, fr, svgDiagPercentage)
	if err != nil {
		return err
	}
	c.normalizeGradientStartEnd()
	return nil
}

func (p *svgParser) normalizeGradientStartEnd() {
	p.grad.Start.X = (p.grad.Start.X - p.svg.ViewBox.X) / p.svg.ViewBox.Width
	p.grad.Start.Y = (p.grad.Start.Y - p.svg.ViewBox.Y) / p.svg.ViewBox.Height
	p.grad.End.X = (p.grad.End.X - p.svg.ViewBox.X) / p.svg.ViewBox.Width
	p.grad.End.Y = (p.grad.End.Y - p.svg.ViewBox.Y) / p.svg.ViewBox.Height
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

func svgStopF(c *svgParser, attrs []xml.Attr) error {
	if c.inGrad {
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
		c.grad.Stops = append(c.grad.Stops, stop)
	}
	return nil
}

func svgUseF(c *svgParser, attrs []xml.Attr) error {
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
			x, err = c.parseUnitToPx(attr.Value, svgWidthPercentage)
		case "y":
			y, err = c.parseUnitToPx(attr.Value, svgHeightPercentage)
		}
		if err != nil {
			return err
		}
	}
	c.curX, c.curY = x, y
	defer func() {
		c.curX, c.curY = 0, 0
	}()
	if href == "" {
		return errors.New("only use tags with href is supported")
	}
	if !strings.HasPrefix(href, "#") {
		return errors.New("only the ID CSS selector is supported")
	}
	defs, ok := c.svg.defs[href[1:]]
	if !ok {
		return errors.New("href ID in use statement was not found in saved defs")
	}
	for _, def := range defs {
		if def.Tag == "endg" {
			// pop style
			c.styleStack = c.styleStack[:len(c.styleStack)-1]
			continue
		}
		if err = c.pushStyle(def.Attrs); err != nil {
			return err
		}
		var df svgFunc
		if df, ok = svgDrawFuncs[def.Tag]; !ok {
			slog.Warn("svg: cannot process svg element", "element", def.Tag)
			return nil
		}
		if df != nil {
			if err = df(c, def.Attrs); err != nil {
				return err
			}
		}
		if def.Tag != "g" {
			// pop style
			c.styleStack = c.styleStack[:len(c.styleStack)-1]
		}
	}
	return nil
}

// SVGDashOptions defines the dash pattern for stroking a path.
type SVGDashOptions struct {
	Dash       []float32 // values for the dash pattern (nil or an empty slice for no dashes)
	DashOffset float32   // starting offset into the dash array
}

// SVGJoinOptions defines how path segments are joined and how line ends are capped.
type SVGJoinOptions struct {
	MiterLimit   float32         // The miter cutoff value for miter, arc, miterclip and arcClip joinModes
	LineJoin     strokejoin.Enum // JoinMode for curve segments
	TrailLineCap strokecap.Enum  // capping functions for leading and trailing line ends. If one is nil, the other function is used at both ends.
}

// SVGStrokeOptions defines the options for stroking a path.
type SVGStrokeOptions struct {
	Dash      SVGDashOptions
	Join      SVGJoinOptions
	LineWidth fixed.Int26_6 // width of the line
}

// SVGDefaultStyle sets the default PathStyle to fill black, winding rule,
// full opacity, no stroke, ButtCap line end and Bevel line connect.
var SVGDefaultStyle = SVGPathStyle{
	FillOpacity:       1.0,
	LineOpacity:       1.0,
	LineWidth:         2.0,
	UseNonZeroWinding: true,
	Join: SVGJoinOptions{
		MiterLimit:   4,
		LineJoin:     strokejoin.Bevel,
		TrailLineCap: strokecap.Butt,
	},
	FillerColor: Black,
	Transform:   geom.NewIdentityMatrix(),
	Masks:       make([]string, 0),
}

var svgRoot2 = xmath.Sqrt(2)

var svgUnits = []struct {
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

// convert the unit to pixels. Return true if it is a %
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

type svgPercentageReference uint8

const (
	svgWidthPercentage svgPercentageReference = iota
	svgHeightPercentage
	svgDiagPercentage
)

func svgResolveUnit(viewBox geom.Rect, s string, asPerc svgPercentageReference) (float32, error) {
	value, isPercentage, err := svgParseUnit(s)
	if err != nil {
		return 0, err
	}
	if isPercentage {
		w, h := viewBox.Width, viewBox.Height
		switch asPerc {
		case svgWidthPercentage:
			return value / 100 * w, nil
		case svgHeightPercentage:
			return value / 100 * h, nil
		case svgDiagPercentage:
			normalizedDiag := xmath.Sqrt(w*w+h*h) / svgRoot2
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
