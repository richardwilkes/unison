package svg

import (
	"image/color"
	"log/slog"
	"strconv"
	"strings"

	"golang.org/x/image/colornames"
	"golang.org/x/image/math/fixed"
)

// Pattern groups a basic color and a gradient pattern
// A nil value may by used to indicated that the function (fill or stroke) is off
type Pattern interface {
	isPattern()
}

// PlainColor is a simple color value
type PlainColor struct {
	color.NRGBA
}

// NewPlainColor creates a new PlainColor from RGBA values
func NewPlainColor(r, g, b, a uint8) PlainColor {
	return PlainColor{NRGBA: color.NRGBA{r, g, b, a}}
}

func (PlainColor) isPattern() {}

// enables to differentiate between black and nil color
type optionalColor struct {
	valid bool
	color PlainColor
}

func toOptColor(p PlainColor) optionalColor {
	return optionalColor{valid: true, color: p}
}

func (o optionalColor) asColor() color.Color {
	if o.valid {
		return o.color
	}
	return nil
}

func (o optionalColor) asPattern() Pattern {
	if o.valid {
		return o.color
	}
	return nil
}

func parseSVGColor(colorStr string) (optionalColor, error) {
	v := strings.ToLower(colorStr)
	if strings.HasPrefix(v, "url") {
		slog.Warn("svg: url() color is unsupported", "url", colorStr)
		return toOptColor(NewPlainColor(0, 0, 0, 255)), nil
	}
	switch v {
	case none:
		return optionalColor{}, nil
	default:
		if cn, ok := colornames.Map[v]; ok {
			r, g, b, a := cn.RGBA()
			return toOptColor(NewPlainColor(uint8(r), uint8(g), uint8(b), uint8(a))), nil
		}
	}
	if cStr := strings.TrimPrefix(colorStr, "rgb("); cStr != colorStr {
		cStr = strings.TrimSuffix(cStr, ")")
		vals := strings.Split(cStr, ",")
		if len(vals) != 3 {
			return toOptColor(PlainColor{}), errParamMismatch
		}
		var cvals [3]uint8
		for i := range cvals {
			var err error
			if cvals[i], err = parseColorValue(vals[i]); err != nil {
				return optionalColor{}, err
			}
		}
		return toOptColor(NewPlainColor(cvals[0], cvals[1], cvals[2], 255)), nil
	}
	if colorStr[0] == '#' {
		r, g, b, err := parseSVGColorNum(colorStr)
		if err != nil {
			return optionalColor{}, err
		}
		return toOptColor(NewPlainColor(r, g, b, 255)), nil
	}
	return optionalColor{}, errParamMismatch
}

func parseColorValue(v string) (uint8, error) {
	if v[len(v)-1] == '%' {
		n, err := strconv.Atoi(strings.TrimSpace(v[:len(v)-1]))
		if err != nil {
			return 0, err
		}
		return uint8(n * 255 / 100), nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if n > 255 {
		n = 255
	}
	return uint8(n), err
}

func parseSVGColorNum(colorStr string) (r, g, b uint8, err error) {
	colorStr = strings.TrimPrefix(colorStr, "#")
	var t uint64
	if len(colorStr) == 3 {
		colorStr = string([]byte{
			colorStr[0], colorStr[0],
			colorStr[1], colorStr[1],
			colorStr[2], colorStr[2],
		})
	} else if len(colorStr) != 6 {
		return 0, 0, 0, errParamMismatch
	}
	for _, v := range []struct {
		c *uint8
		s string
	}{
		{&r, colorStr[0:2]},
		{&g, colorStr[2:4]},
		{&b, colorStr[4:6]},
	} {
		t, err = strconv.ParseUint(v.s, 16, 8)
		if err != nil {
			return r, g, b, err
		}
		*v.c = uint8(t)
	}
	return r, g, b, nil
}

// GradientUnits is the type for gradient units
type GradientUnits byte

// SVG bounds parameter constants
const (
	ObjectBoundingBox GradientUnits = iota
	UserSpaceOnUse
)

// SpreadMethod is the type for spread parameters
type SpreadMethod byte

// SVG spread parameter constants
const (
	PadSpread SpreadMethod = iota
	ReflectSpread
	RepeatSpread
)

// GradStop represents a stop in the SVG 2.0 gradient specification
type GradStop struct {
	StopColor color.Color
	Offset    float64
	Opacity   float64
}

// Gradient holds a description of an SVG 2.0 gradient
type Gradient struct {
	Direction gradientDirection
	Stops     []GradStop
	Bounds    Bounds
	Matrix    Matrix2D
	Spread    SpreadMethod
	Units     GradientUnits
}

func (g *Gradient) isPattern() {}

// ApplyPathExtent uses the given path extent to adjust the bounding box, if required by `Units`. The `Direction` field
// is not modified, but a matrix accounting for both the bounding box and the gradient matrix is returned.
func (g *Gradient) ApplyPathExtent(extent fixed.Rectangle26_6) Matrix2D {
	if g.Units == ObjectBoundingBox {
		mnx, mny := float64(extent.Min.X)/64, float64(extent.Min.Y)/64
		mxx, mxy := float64(extent.Max.X)/64, float64(extent.Max.Y)/64
		g.Bounds.X, g.Bounds.Y = mnx, mny
		g.Bounds.W, g.Bounds.H = mxx-mnx, mxy-mny
		return Identity.Scale(g.Bounds.W, g.Bounds.H).Mult(g.Matrix)
	}
	return g.Matrix
}

type gradientDirection interface {
	isRadial() bool
}

// Linear holds x1, y1, x2, y2
type Linear [4]float64

func (Linear) isRadial() bool { return false }

// Radial holds cx, cy, fx, fy, r, fr
type Radial [6]float64

func (Radial) isRadial() bool { return true }

// GetColor is a helper function to get the background color
func GetColor(clr Pattern) color.Color {
	switch c := clr.(type) {
	case *Gradient:
		for _, s := range c.Stops {
			if s.StopColor != nil {
				return s.StopColor
			}
		}
	case PlainColor:
		return c
	}
	return colornames.Black
}
