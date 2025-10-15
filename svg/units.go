package svg

import (
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/v2/xmath"
)

var root2 = xmath.Sqrt(2)

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
func parseUnit(s string) (f float32, isPercent bool, err error) {
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

type percentageReference uint8

const (
	widthPercentage percentageReference = iota
	heightPercentage
	diagPercentage
)

// resolveUnit converts a length with a unit into its value in 'px'
// percentage are supported, and refer to the viewBox
// `asPerc` is only applied when `s` contains a percentage.
func (viewBox Bounds) resolveUnit(s string, asPerc percentageReference) (float32, error) {
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
			normalizedDiag := xmath.Sqrt(w*w+h*h) / root2
			return value / 100 * normalizedDiag, nil
		}
	}
	return value, nil
}

func parseBasicFloat(s string) (float32, error) {
	value, _, err := parseUnit(s)
	return value, err
}

func readFraction(v string) (f float32, err error) {
	v = strings.TrimSpace(v)
	d := float32(1.0)
	if strings.HasSuffix(v, "%") {
		d = 100
		v = strings.TrimSuffix(v, "%")
	}
	f, err = parseBasicFloat(v)
	return f / d, err
}
