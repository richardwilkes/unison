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
	"strconv"
	"strings"
	"unicode"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/enums/arcsize"
	"github.com/richardwilkes/unison/enums/direction"
	"github.com/richardwilkes/unison/enums/filltype"
	"github.com/richardwilkes/unison/enums/strokecap"
	"github.com/richardwilkes/unison/enums/strokejoin"
	"github.com/richardwilkes/unison/enums/tilemode"
	"golang.org/x/net/html/charset"
)

var (
	errParamMismatch = errors.New("param mismatch")
	errZeroLengthID  = errors.New("zero length id")
)

// SVGData holds data from parsed SVGs.
type SVGData struct {
	Masks     map[string]*SVGMask
	grads     map[string]*Gradient
	defs      map[string][]svgDef
	Paths     []SVGStyledPath
	Transform geom.Matrix
}

// SVGPathStyle holds the state of the style.
type SVGPathStyle struct {
	FillerColor       Ink
	LinerColor        Ink
	Masks             []string  // Currently unused
	Dash              []float32 // Currently unused
	DashOffset        float32   // Currently unused
	MiterLimit        float32
	FillOpacity       float32
	LineOpacity       float32
	LineWidth         float32
	Transform         geom.Matrix
	StrokeJoin        strokejoin.Enum
	StrokeCap         strokecap.Enum
	UseNonZeroWinding bool
}

// SVGStyledPath binds a PathStyle to a Path.
type SVGStyledPath struct {
	Path  *Path
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
	svg        *SVG
	data       *SVGData
	grad       *Gradient
	mask       *SVGMask
	styleStack []SVGPathStyle
	currentDef []svgDef
	svgPathParser
	inGrad bool
	inDefs bool
	inMask bool
}

func parseSVG(stream io.Reader) (*SVG, error) {
	svg := &SVGData{
		defs:      make(map[string][]svgDef),
		grads:     make(map[string]*Gradient),
		Masks:     make(map[string]*SVGMask),
		Transform: geom.NewIdentityMatrix(),
	}
	p := &svgParser{
		svg:        &SVG{},
		data:       svg,
		styleStack: []SVGPathStyle{SVGDefaultStyle},
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
					p.currentDef = append(p.currentDef, svgDef{Tag: "endg"})
				}
			case "mask":
				if p.mask != nil {
					p.data.Masks[p.mask.ID] = p.mask
					p.mask = nil
				}
				p.inMask = false
			case "defs":
				if len(p.currentDef) > 0 {
					p.data.defs[p.currentDef[0].ID] = p.currentDef
					p.currentDef = make([]svgDef, 0)
				}
				p.inDefs = false
			case "radialGradient", "linearGradient":
				p.inGrad = false
			}
		}
	}

	// From here down converts to unison's internal representation
	p.svg.paths = make([]*svgPath, len(svg.Paths))
	for i := range svg.Paths {
		p1 := svg.Paths[i].Path
		if svg.Paths[i].Style.UseNonZeroWinding {
			p1.SetFillType(filltype.Winding)
		} else {
			p1.SetFillType(filltype.EvenOdd)
		}

		if !svg.Paths[i].Style.Transform.IsIdentity() {
			p1.Transform(svg.Paths[i].Style.Transform)
		}
		sp := &svgPath{Path: p1}

		if svg.Paths[i].Style.FillerColor != nil && svg.Paths[i].Style.FillOpacity != 0 {
			sp.fill = createPaintFromSVGPattern(svg.Paths[i].Style.FillerColor, svg.Paths[i].Style.FillOpacity)
		}

		if svg.Paths[i].Style.LinerColor != nil && svg.Paths[i].Style.LineOpacity != 0 &&
			svg.Paths[i].Style.LineWidth != 0 {
			sp.stroke = createPaintFromSVGPattern(svg.Paths[i].Style.LinerColor, svg.Paths[i].Style.LineOpacity)
			sp.strokeCap = svg.Paths[i].Style.StrokeCap
			sp.strokeJoin = svg.Paths[i].Style.StrokeJoin
			sp.strokeMiter = svg.Paths[i].Style.MiterLimit
			sp.strokeWidth = svg.Paths[i].Style.LineWidth
		}

		p.svg.paths[i] = sp
	}
	return p.svg, nil
}

func createPaintFromSVGPattern(pattern Ink, opacity float32) Ink {
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
			curStyle.StrokeCap = strokecap.Butt
		case "round":
			curStyle.StrokeCap = strokecap.Round
		case "square":
			curStyle.StrokeCap = strokecap.Square
		default:
			slog.Warn("svg: unsupported value for <stroke-linecap>", "value", v)
		}
	case "stroke-linejoin":
		switch v {
		case "miter":
			curStyle.StrokeJoin = strokejoin.Miter
		case "round":
			curStyle.StrokeJoin = strokejoin.Round
		case "bevel":
			curStyle.StrokeJoin = strokejoin.Bevel
		default:
			slog.Warn("svg: unsupported value for <stroke-linejoin>", "value", v)
		}
	case "stroke-miterlimit":
		if curStyle.MiterLimit, err = svgParseBasicFloat(v); err != nil {
			return err
		}
	case "stroke-width":
		if curStyle.LineWidth, err = p.parseUnitToPx(v, svgWidthPercentage); err != nil {
			return err
		}
	case "stroke-dashoffset":
		if curStyle.DashOffset, err = p.parseUnitToPx(v, svgDiagPercentage); err != nil {
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
			curStyle.Dash = dList
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
			p.data.defs[p.currentDef[0].ID] = p.currentDef
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
	if p.path != nil && !p.path.Empty() {
		if p.inMask && p.mask != nil {
			p.mask.SvgPaths = append(p.mask.SvgPaths,
				SVGStyledPath{Path: p.path, Style: p.styleStack[len(p.styleStack)-1]})
		} else if !p.inMask {
			p.data.Paths = append(p.data.Paths,
				SVGStyledPath{Path: p.path, Style: p.styleStack[len(p.styleStack)-1]})
		}
		p.path = NewPath()
	}
	return err
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

func (p *svgParser) parseUnitToPx(s string, asPerc svgPercentageReference) (float32, error) {
	return svgResolveUnit(p.svg.viewBox, s, asPerc)
}

type svgPathParser struct {
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
}

func (c *svgPathParser) compilePath(svgPath string) error {
	c.placeX = 0
	c.placeY = 0
	c.pts = c.pts[0:0]
	c.lastKey = ' '
	c.path = NewPath()
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
			c.path.Close()
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
		c.path.MoveTo(geom.NewPoint(c.pathStartX+c.curX, c.pathStartY+c.curY))
		for i := 2; i < l-1; i += 2 {
			c.path.LineTo(geom.NewPoint(c.pts[i]+c.curX, c.pts[i+1]+c.curY))
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
			c.path.LineTo(geom.NewPoint(c.pts[i]+c.curX, c.pts[i+1]+c.curY))
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
			c.path.LineTo(geom.NewPoint(c.placeX+c.curX, p+c.curY))
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
			c.path.LineTo(geom.NewPoint(p+c.curX, c.placeY+c.curY))
		}
		c.placeX = c.pts[l-1]
	case 'q', 'Q':
		if !c.hasSetsOrMore(4, k == 'q') {
			return errParamMismatch
		}
		for i := 0; i < l-3; i += 4 {
			c.path.QuadTo(
				geom.NewPoint(c.pts[i]+c.curX, c.pts[i+1]+c.curY),
				geom.NewPoint(c.pts[i+2]+c.curX, c.pts[i+3]+c.curY),
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
			c.path.QuadTo(
				geom.NewPoint(c.cntlPtX+c.curX, c.cntlPtY+c.curY),
				geom.NewPoint(c.pts[i]+c.curX, c.pts[i+1]+c.curY),
			)
			c.lastKey = k
			c.placeX = c.pts[i]
			c.placeY = c.pts[i+1]
		}
	case 'c', 'C':
		if !c.hasSetsOrMore(6, k == 'c') {
			return errParamMismatch
		}
		for i := 0; i < l-5; i += 6 {
			c.path.CubicTo(
				geom.NewPoint(c.pts[i]+c.curX, c.pts[i+1]+c.curY),
				geom.NewPoint(c.pts[i+2]+c.curX, c.pts[i+3]+c.curY),
				geom.NewPoint(c.pts[i+4]+c.curX, c.pts[i+5]+c.curY),
			)
		}
		c.cntlPtX, c.cntlPtY = c.pts[l-4], c.pts[l-3]
		c.placeX = c.pts[l-2]
		c.placeY = c.pts[l-1]
	case 's', 'S':
		if !c.hasSetsOrMore(4, k == 's') {
			return errParamMismatch
		}
		for i := 0; i < l-3; i += 4 {
			c.reflectControl(false)
			c.path.CubicTo(
				geom.NewPoint(c.cntlPtX+c.curX, c.cntlPtY+c.curY),
				geom.NewPoint(c.pts[i]+c.curX, c.pts[i+1]+c.curY),
				geom.NewPoint(c.pts[i+2]+c.curX, c.pts[i+3]+c.curY),
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
			x := c.pts[i+5] + c.curX
			y := c.pts[i+6] + c.curY
			as := arcsize.Small
			if c.pts[i+3] != 0 {
				as = arcsize.Large
			}
			dir := direction.CounterClockwise
			if c.pts[i+4] != 0 {
				dir = direction.Clockwise
			}
			c.path.ArcTo(geom.NewPoint(x, y), geom.NewSize(c.pts[i], c.pts[i+1]), c.pts[i+2], as, dir)
			c.placeX = x
			c.placeY = y
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

func svgF(p *svgParser, attrs []xml.Attr) error {
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
	if rx == 0 {
		c.path.Rect(geom.NewRect(x+c.curX, y+c.curY, w, h))
	} else {
		c.path.RoundedRect(geom.NewRect(x+c.curX, y+c.curY, w, h), geom.NewSize(rx, ry))
	}
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
	if rx == 0 || ry == 0 {
		return nil
	}
	cx += c.curX
	cy += c.curY
	if rx == ry {
		c.path.Circle(geom.NewPoint(cx, cy), rx)
	} else {
		c.path.Oval(geom.NewRect(cx-rx, cy-ry, cx+rx, cy+ry))
	}
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
	c.path.MoveTo(geom.NewPoint(x1+c.curX, y1+c.curY))
	c.path.LineTo(geom.NewPoint(x2+c.curX, y2+c.curY))
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
		c.path.MoveTo(geom.NewPoint(c.pts[0]+c.curX, c.pts[1]+c.curY))
		for i := 2; i < len(c.pts)-1; i += 2 {
			c.path.LineTo(geom.NewPoint(c.pts[i]+c.curX, c.pts[i+1]+c.curY))
		}
	}
	return nil
}

func svgPolygonF(c *svgParser, attrs []xml.Attr) error {
	err := svgPolylineF(c, attrs)
	if len(c.pts) > 4 {
		c.path.Close()
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

func svgLinearGradientF(p *svgParser, attrs []xml.Attr) error {
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
	p.grad.Start.X, err = svgResolveUnit(bbox, x1, svgWidthPercentage)
	if err != nil {
		return err
	}
	p.grad.Start.Y, err = svgResolveUnit(bbox, y1, svgHeightPercentage)
	if err != nil {
		return err
	}
	p.grad.End.X, err = svgResolveUnit(bbox, x2, svgWidthPercentage)
	if err != nil {
		return err
	}
	p.grad.End.Y, err = svgResolveUnit(bbox, y2, svgHeightPercentage)
	if err != nil {
		return err
	}
	p.normalizeGradientStartEnd()
	return nil
}

func svgRadialGradientF(p *svgParser, attrs []xml.Attr) error {
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
	p.grad.Start.X, err = svgResolveUnit(bbox, cx, svgWidthPercentage)
	if err != nil {
		return err
	}
	p.grad.Start.Y, err = svgResolveUnit(bbox, cy, svgHeightPercentage)
	if err != nil {
		return err
	}
	p.grad.End.X, err = svgResolveUnit(bbox, fx, svgWidthPercentage)
	if err != nil {
		return err
	}
	p.grad.End.Y, err = svgResolveUnit(bbox, fy, svgHeightPercentage)
	if err != nil {
		return err
	}
	p.grad.StartRadius, err = svgResolveUnit(bbox, r, svgDiagPercentage)
	if err != nil {
		return err
	}
	p.grad.EndRadius, err = svgResolveUnit(bbox, fr, svgDiagPercentage)
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
	defs, ok := c.data.defs[href[1:]]
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

// SVGDefaultStyle sets the default PathStyle to fill black, winding rule,
// full opacity, no stroke, ButtCap line end and Bevel line connect.
var SVGDefaultStyle = SVGPathStyle{
	FillOpacity:       1.0,
	LineOpacity:       1.0,
	LineWidth:         2.0,
	UseNonZeroWinding: true,
	MiterLimit:        4,
	StrokeJoin:        strokejoin.Bevel,
	StrokeCap:         strokecap.Butt,
	FillerColor:       Black,
	Transform:         geom.NewIdentityMatrix(),
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
