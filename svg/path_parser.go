package svg

import (
	"errors"
	"log/slog"
	"math"
	"unicode"

	"golang.org/x/image/math/fixed"
)

var errParamMismatch = errors.New("param mismatch")

type pathParser struct {
	pts        []float32
	path       Path
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

func (c *pathParser) compilePath(svgPath string) error {
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

func (c *pathParser) valsToAbs(last float32) {
	for i := 0; i < len(c.pts); i++ {
		last += c.pts[i]
		c.pts[i] = last
	}
}

func (c *pathParser) pointsToAbs(sz int) {
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

func (c *pathParser) hasSetsOrMore(sz int, rel bool) bool {
	if len(c.pts) < sz || len(c.pts)%sz != 0 {
		return false
	}
	if rel {
		c.pointsToAbs(sz)
	}
	return true
}

func (c *pathParser) addPoints(dataPoints string) error {
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

func (c *pathParser) readFloatIntoPts(numStr string) error {
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
		f, err := parseBasicFloat(numStr[last:i])
		if err != nil {
			return err
		}
		c.pts = append(c.pts, f)
		last = i
	}
	f, err := parseBasicFloat(numStr[last:])
	if err != nil {
		return err
	}
	c.pts = append(c.pts, f)
	return nil
}

func (c *pathParser) addSegment(segString string) error {
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

func (c *pathParser) reflectControl(forQuad bool) {
	if (forQuad && (c.lastKey == 'q' || c.lastKey == 'Q' || c.lastKey == 'T' || c.lastKey == 't')) ||
		(!forQuad && (c.lastKey == 'c' || c.lastKey == 'C' || c.lastKey == 's' || c.lastKey == 'S')) {
		c.cntlPtX = c.placeX*2 - c.cntlPtX
		c.cntlPtY = c.placeY*2 - c.cntlPtY
	} else {
		c.cntlPtX, c.cntlPtY = c.placeX, c.placeY
	}
}

func (c *pathParser) ellipseAt(cx, cy, rx, ry float32) {
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

func (c *pathParser) addArcFromA(points []float32) {
	cx, cy := findEllipseCenter(&points[0], &points[1], points[2]*math.Pi/180, c.placeX,
		c.placeY, points[5], points[6], points[4] == 0, points[3] == 0)
	c.placeX, c.placeY = c.path.addArc(c.pts, cx+c.curX, cy+c.curY, c.placeX+c.curX, c.placeY+c.curY)
}
