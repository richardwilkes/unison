package svg

import (
	"math"
	"strconv"
	"strings"
)

var root2 = math.Sqrt(2)

type unit uint8

// Absolute units supported.
const (
	Px unit = iota
	Cm
	Mm
	Pt
	In
	Q
	Pc
	Perc // Special case : percentage (%) relative to the viewbox
)

var absoluteUnits = [...]string{
	Px:   "px",
	Cm:   "cm",
	Mm:   "mm",
	Pt:   "pt",
	In:   "in",
	Q:    "Q",
	Pc:   "pc",
	Perc: "%",
}

const referencePPI = 96

var toPx = [...]float64{
	Px:   1,
	Cm:   referencePPI / 2.54,
	Mm:   9.6 / 2.54,
	Pt:   referencePPI / 72.0,
	In:   referencePPI,
	Q:    referencePPI / 40.0 / 2.54,
	Pc:   referencePPI / 6.0,
	Perc: 1,
}

// look for an absolute unit, or nothing (considered as pixels)
// % is also supported
func findUnit(s string) (u unit, value string) {
	s = strings.TrimSpace(s)
	for u, suffix := range absoluteUnits {
		if strings.HasSuffix(s, suffix) {
			valueS := strings.TrimSpace(strings.TrimSuffix(s, suffix))
			return unit(u), valueS
		}
	}
	return Px, s
}

// convert the unit to pixels. Return true if it is a %
func parseUnit(s string) (f float64, isPercent bool, err error) {
	u, value := findUnit(s)
	var out float64
	out, err = strconv.ParseFloat(value, 64)
	return out * toPx[u], u == Perc, err
}

type percentageReference uint8

const (
	widthPercentage percentageReference = iota
	heightPercentage
	diagPercentage
)

// resolveUnit converts a length with a unit into its value in 'px'
// percentage are supported, and refer to the viewBox
// `asPerc` is only applied when `s` contains a percentage.
func (viewBox Bounds) resolveUnit(s string, asPerc percentageReference) (float64, error) {
	value, isPercentage, err := parseUnit(s)
	if err != nil {
		return 0, err
	}
	if isPercentage {
		w, h := viewBox.W, viewBox.H
		switch asPerc {
		case widthPercentage:
			return value / 100 * w, nil
		case heightPercentage:
			return value / 100 * h, nil
		case diagPercentage:
			normalizedDiag := math.Sqrt(w*w+h*h) / root2
			return value / 100 * normalizedDiag, nil
		}
	}
	return value, nil
}

// parseUnit converts a length with a unit into its value in 'px'
// percentage are supported, and refer to the current ViewBox
func (c *cursor) parseUnit(s string, asPerc percentageReference) (float64, error) {
	return c.svg.ViewBox.resolveUnit(s, asPerc)
}

func parseBasicFloat(s string) (float64, error) {
	value, _, err := parseUnit(s)
	return value, err
}

func readFraction(v string) (f float64, err error) {
	v = strings.TrimSpace(v)
	d := 1.0
	if strings.HasSuffix(v, "%") {
		d = 100
		v = strings.TrimSuffix(v, "%")
	}
	f, err = parseBasicFloat(v)
	return f / d, err
}
