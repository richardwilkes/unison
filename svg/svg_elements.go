package svg

import (
	"encoding/xml"
	"errors"
	"log/slog"
	"strings"

	"golang.org/x/image/math/fixed"
)

var errZeroLengthID = errors.New("zero length id")

func init() {
	drawFuncs["use"] = useF // Can't be done statically, since useF uses drawFuncs
}

type svgFunc func(c *svgParser, attrs []xml.Attr) error

var drawFuncs = map[string]svgFunc{
	"svg":            svgF,
	"g":              nil,
	"line":           lineF,
	"stop":           stopF,
	"rect":           rectF,
	"circle":         circleF,
	"ellipse":        circleF, // circleF handles ellipse also
	"polyline":       polylineF,
	"polygon":        polygonF,
	"path":           pathF,
	"desc":           nil,
	"defs":           defsF,
	"title":          nil,
	"linearGradient": linearGradientF,
	"radialGradient": radialGradientF,
	"mask":           maskF,
}

func svgF(c *svgParser, attrs []xml.Attr) error {
	c.svg.ViewBox.X = 0
	c.svg.ViewBox.Y = 0
	c.svg.ViewBox.W = 0
	c.svg.ViewBox.H = 0
	var width, height float64
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "viewBox":
			err = c.addPoints(attr.Value)
			if len(c.pts) != 4 {
				return errParamMismatch
			}
			c.svg.ViewBox.X = c.pts[0]
			c.svg.ViewBox.Y = c.pts[1]
			c.svg.ViewBox.W = c.pts[2]
			c.svg.ViewBox.H = c.pts[3]
		case "width": //nolint:goconst // Can't use const named width
			c.svg.Width = attr.Value
			width, err = parseBasicFloat(attr.Value)
		case "height": //nolint:goconst // Can't use const named height
			c.svg.Height = attr.Value
			height, err = parseBasicFloat(attr.Value)
		}
		if err != nil {
			return err
		}
	}
	if c.svg.ViewBox.W == 0 {
		c.svg.ViewBox.W = width
	}
	if c.svg.ViewBox.H == 0 {
		c.svg.ViewBox.H = height
	}
	return nil
}

func rectF(c *svgParser, attrs []xml.Attr) error {
	var x, y, w, h, rx, ry float64
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "x":
			x, err = c.parseUnitToPx(attr.Value, widthPercentage)
		case "y":
			y, err = c.parseUnitToPx(attr.Value, heightPercentage)
		case "width":
			w, err = c.parseUnitToPx(attr.Value, widthPercentage)
		case "height":
			h, err = c.parseUnitToPx(attr.Value, heightPercentage)
		case "rx":
			rx, err = c.parseUnitToPx(attr.Value, widthPercentage)
		case "ry":
			ry, err = c.parseUnitToPx(attr.Value, heightPercentage)
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
	c.path.addRoundRect(x+c.curX, y+c.curY, w+x+c.curX, h+y+c.curY, rx, ry, 0)
	return nil
}

func maskF(c *svgParser, attrs []xml.Attr) error {
	var mask Mask
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "id":
			if attr.Value == "" {
				return errZeroLengthID
			}
			mask.ID = attr.Value
		case "x":
			mask.Bounds.X, err = c.parseUnitToPx(attr.Value, widthPercentage)
		case "y":
			mask.Bounds.Y, err = c.parseUnitToPx(attr.Value, heightPercentage)
		case "width":
			mask.Bounds.W, err = c.parseUnitToPx(attr.Value, widthPercentage)
		case "height":
			mask.Bounds.H, err = c.parseUnitToPx(attr.Value, heightPercentage)
		}
		if err != nil {
			return err
		}
	}
	mask.Transform = Identity
	c.inMask = true
	c.mask = &mask
	return nil
}

func circleF(c *svgParser, attrs []xml.Attr) error {
	var cx, cy, rx, ry float64
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "cx":
			cx, err = c.parseUnitToPx(attr.Value, widthPercentage)
		case "cy":
			cy, err = c.parseUnitToPx(attr.Value, heightPercentage)
		case "r":
			rx, err = c.parseUnitToPx(attr.Value, diagPercentage)
			ry = rx
		case "rx":
			rx, err = c.parseUnitToPx(attr.Value, widthPercentage)
		case "ry":
			ry, err = c.parseUnitToPx(attr.Value, heightPercentage)
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

func lineF(c *svgParser, attrs []xml.Attr) error {
	var x1, x2, y1, y2 float64
	var err error
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "x1":
			x1, err = c.parseUnitToPx(attr.Value, widthPercentage)
		case "x2":
			x2, err = c.parseUnitToPx(attr.Value, widthPercentage)
		case "y1":
			y1, err = c.parseUnitToPx(attr.Value, heightPercentage)
		case "y2":
			y2, err = c.parseUnitToPx(attr.Value, heightPercentage)
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

func polylineF(c *svgParser, attrs []xml.Attr) error {
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

func polygonF(c *svgParser, attrs []xml.Attr) error {
	err := polylineF(c, attrs)
	if len(c.pts) > 4 {
		c.path.Stop(true)
	}
	return err
}

func pathF(c *svgParser, attrs []xml.Attr) error {
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

func defsF(c *svgParser, _ []xml.Attr) error {
	c.inDefs = true
	return nil
}

func linearGradientF(c *svgParser, attrs []xml.Attr) error {
	var err error
	c.inGrad = true
	// interpretation of percentage in direction depends
	// on gradientUnits: we first store the string values
	// and resolve them in a second pass
	directionStrings := [4]string{"0%", "0%", "100%", "0"} // default value
	c.grad = &Gradient{Bounds: c.svg.ViewBox, Matrix: Identity}
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "id":
			if attr.Value == "" {
				return errZeroLengthID
			}
			c.svg.grads[attr.Value] = c.grad
		case "x1":
			directionStrings[0] = attr.Value
		case "y1":
			directionStrings[1] = attr.Value
		case "x2":
			directionStrings[2] = attr.Value
		case "y2":
			directionStrings[3] = attr.Value
		default:
			err = c.readGradientAttr(attr)
		}
		if err != nil {
			return err
		}
	}
	// now we can resolve percentages
	bbox := Bounds{W: 1, H: 1} // default is ObjectBoundingBox
	if c.grad.Units == UserSpaceOnUse {
		bbox = c.grad.Bounds
	}
	var direction Linear
	direction[0], err = bbox.resolveUnit(directionStrings[0], widthPercentage)
	if err != nil {
		return err
	}
	direction[1], err = bbox.resolveUnit(directionStrings[1], heightPercentage)
	if err != nil {
		return err
	}
	direction[2], err = bbox.resolveUnit(directionStrings[2], widthPercentage)
	if err != nil {
		return err
	}
	direction[3], err = bbox.resolveUnit(directionStrings[3], heightPercentage)
	if err != nil {
		return err
	}
	c.grad.Direction = direction
	return nil
}

func radialGradientF(c *svgParser, attrs []xml.Attr) error {
	c.inGrad = true
	c.grad = &Gradient{Bounds: c.svg.ViewBox, Matrix: Identity}
	var setFx, setFy bool
	var err error
	directionStrings := [6]string{"50%", "50%", "50%", "50%", "50%", "50%"} // default values
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "id":
			if attr.Value == "" {
				return errZeroLengthID
			}
			c.svg.grads[attr.Value] = c.grad
		case "cx":
			directionStrings[0] = attr.Value
		case "cy":
			directionStrings[1] = attr.Value
		case "fx":
			setFx = true
			directionStrings[2] = attr.Value
		case "fy":
			setFy = true
			directionStrings[3] = attr.Value
		case "r":
			directionStrings[4] = attr.Value
		case "fr":
			directionStrings[5] = attr.Value
		default:
			err = c.readGradientAttr(attr)
		}
		if err != nil {
			return err
		}
	}
	if !setFx { // set fx to cx by default
		directionStrings[2] = directionStrings[0]
	}
	if !setFy { // set fy to cy by default
		directionStrings[3] = directionStrings[1]
	}

	// now we can resolve percentages
	bbox := Bounds{W: 1, H: 1} // default is ObjectBoundingBox
	if c.grad.Units == UserSpaceOnUse {
		bbox = c.grad.Bounds
	}
	var direction Radial
	direction[0], err = bbox.resolveUnit(directionStrings[0], widthPercentage)
	if err != nil {
		return err
	}
	direction[1], err = bbox.resolveUnit(directionStrings[1], heightPercentage)
	if err != nil {
		return err
	}
	direction[2], err = bbox.resolveUnit(directionStrings[2], widthPercentage)
	if err != nil {
		return err
	}
	direction[3], err = bbox.resolveUnit(directionStrings[3], heightPercentage)
	if err != nil {
		return err
	}
	direction[4], err = bbox.resolveUnit(directionStrings[4], diagPercentage)
	if err != nil {
		return err
	}
	direction[5], err = bbox.resolveUnit(directionStrings[5], diagPercentage)
	if err != nil {
		return err
	}

	c.grad.Direction = direction
	return nil
}

func stopF(c *svgParser, attrs []xml.Attr) error {
	var err error
	if c.inGrad {
		stop := GradStop{Opacity: 1.0}
		// parse style and push into attrs
		attrs, err = appendStyleAttrs(attrs, "stop-color", "stop-opacity")
		if err != nil {
			return err
		}

		for _, attr := range attrs {
			switch attr.Name.Local {
			case "offset":
				stop.Offset, err = readFraction(attr.Value)
			case "stop-color":
				// todo: add current color inherit
				var optColor optionalColor
				optColor, err = parseSVGColor(attr.Value)
				stop.StopColor = optColor.asColor()
			case "stop-opacity":
				stop.Opacity, err = parseBasicFloat(attr.Value)
			}
			if err != nil {
				return err
			}
		}
		c.grad.Stops = append(c.grad.Stops, stop)
	}
	return nil
}

// appendStyleAttrs appends style attributes to the given attributes.
func appendStyleAttrs(attrs []xml.Attr, names ...string) ([]xml.Attr, error) {
	var style string

	for _, attr := range attrs {
		if attr.Name.Local == "style" {
			style = attr.Value
			break
		}
	}

	if style == "" {
		return attrs, nil
	}

	styleEl := strings.Split(style, ";")
	styleAttrs := make([]xml.Attr, 0, len(styleEl))
	for _, s := range styleEl {
		key, val, ok := strings.Cut(s, ":")
		if !ok {
			continue
		}

		key = strings.ToLower(strings.TrimSpace(key))

		if len(names) == 0 {
			styleAttrs = append(styleAttrs, xml.Attr{
				Name:  xml.Name{Local: key},
				Value: strings.TrimSpace(val),
			})
			continue
		}

		for _, name := range names {
			if key == name {
				attrs = append(attrs, xml.Attr{
					Name:  xml.Name{Local: key},
					Value: strings.TrimSpace(val),
				})
				break
			}
		}
	}

	return append(attrs, styleAttrs...), nil
}

func useF(c *svgParser, attrs []xml.Attr) error {
	var (
		href string
		x, y float64
		err  error
	)
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "href":
			href = attr.Value
		case "x":
			x, err = c.parseUnitToPx(attr.Value, widthPercentage)
		case "y":
			y, err = c.parseUnitToPx(attr.Value, heightPercentage)
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
		if df, ok = drawFuncs[def.Tag]; !ok {
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
