// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

var (
	nameToColor               = make(map[string]Color)
	colorToName               = make(map[Color]string)
	_           ColorProvider = Color(0)
)

// ColorProvider allows for different types of objects that hold a color to be used interchangeably.
type ColorProvider interface {
	GetColor() Color
	Ink
}

// Color contains the value of a color used for drawing, stored as 0xAARRGGBB.
type Color uint32

// RGB creates a new opaque Color from RGB (Red, Green Blue) values in the range 0-255.
func RGB(red, green, blue int) Color {
	return ARGB(1, red, green, blue)
}

// ARGB creates a new Color from RGB (Red, Green Blue) values in the range 0-255 and an alpha value in the range 0-1.
func ARGB(alpha float32, red, green, blue int) Color {
	return Color(clamp0To1AndScale255(alpha)<<24 | clamp0To255(red)<<16 | clamp0To255(green)<<8 | clamp0To255(blue))
}

// ARGBfloat creates a new Color from ARGB (Alpha, Red, Green Blue) values in the range 0-1.
func ARGBfloat(alpha, red, green, blue float32) Color {
	return Color(clamp0To1AndScale255(alpha)<<24 | clamp0To1AndScale255(red)<<16 | clamp0To1AndScale255(green)<<8 |
		clamp0To1AndScale255(blue))
}

// ColorFromNRGBA creates a new Color from a color.NRGBA value.
func ColorFromNRGBA(c color.NRGBA) Color {
	return Color(uint32(c.A)<<24 | uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B))
}

// HSB creates a new opaque Color from HSB (Hue, Saturation, Brightness) values in the range 0-1.
func HSB(hue, saturation, brightness float32) Color {
	return HSBA(hue, saturation, brightness, 1)
}

// HSBA creates a new Color from HSBA (Hue, Saturation, Brightness, Alpha) values in the range 0-1.
func HSBA(hue, saturation, brightness, alpha float32) Color {
	saturation = clamp0To1(saturation)
	brightness = clamp0To1(brightness)
	v := clamp0To1AndScale255(brightness)
	if saturation == 0 {
		return ARGB(alpha, v, v, v)
	}
	h := (hue - xmath.Floor(hue)) * 6
	f := h - xmath.Floor(h)
	p := clamp0To1AndScale255(brightness * (1 - saturation))
	q := clamp0To1AndScale255(brightness * (1 - saturation*f))
	t := clamp0To1AndScale255(brightness * (1 - (saturation * (1 - f))))
	switch int(h) {
	case 0:
		return ARGB(alpha, v, t, p)
	case 1:
		return ARGB(alpha, q, v, p)
	case 2:
		return ARGB(alpha, p, v, t)
	case 3:
		return ARGB(alpha, p, q, v)
	case 4:
		return ARGB(alpha, t, p, v)
	default:
		return ARGB(alpha, v, p, q)
	}
}

// HSL creates a new opaque Color from HSL (Hue, Saturation, Lightness) values in the range 0-1.
func HSL(hue, saturation, lightness float32) Color {
	return HSLA(hue, saturation, lightness, 1)
}

// HSLA creates a new Color from HSLA (Hue, Saturation, Lightness, Alpha) values in the range 0-1.
func HSLA(hue, saturation, lightness, alpha float32) Color {
	saturation = clamp0To1(saturation)
	lightness = clamp0To1(lightness)
	if saturation == 0 {
		v := clamp0To1AndScale255(lightness)
		return ARGB(alpha, v, v, v)
	}
	var q float32
	if lightness < 0.5 {
		q = lightness * (1 + saturation)
	} else {
		q = lightness + saturation - lightness*saturation
	}
	p := 2*lightness - q
	hue -= xmath.Floor(hue)
	return ARGB(alpha, clamp0To1AndScale255(hueToRGB(p, q, hue+1.0/3.0)),
		clamp0To1AndScale255(hueToRGB(p, q, hue)),
		clamp0To1AndScale255(hueToRGB(p, q, hue-1.0/3.0)))
}

func hueToRGB(p, q, t float32) float32 {
	if t < 0 {
		t++
	} else if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

// OKLCH creates a Color from lightness (0-1), chroma (0-0.37), hue (0-360), alpha (0-1) values using the OKLCH color space.
func OKLCH(lightness, chroma, hue, alpha float32) Color {
	x := float64(normalizeHue(float64(hue))) * math.Pi / 180
	c := float64(clampChromaForOKLCH(chroma))
	y := c * math.Cos(x)
	z := c * math.Sin(x)
	light := float64(clamp0To1(lightness))
	l := light*0.99999999845051981432 + 0.39633779217376785678*y + 0.21580375806075880339*z
	L := l * l * l
	m := light*1.0000000088817607767 - 0.1055613423236563494*y - 0.063854174771705903402*z
	M := m * m * m
	s := light*1.0000000546724109177 - 0.089484182094965759684*y - 1.2914855378640917399*z
	S := s * s * s
	return ARGBfloat(alpha, fromLinear(4.076741661347994*L-3.307711590408193*M+0.230969928729428*S),
		fromLinear(-1.2684380040921763*L+2.6097574006633715*M-0.3413193963102197*S),
		fromLinear(-0.004196086541837188*L-0.7034186144594493*M+1.7076147009309444*S))
}

func fromLinear(value float64) float32 {
	abs := math.Abs(value)
	if abs > 0.0031308 {
		var m float64
		if math.Signbit(value) {
			m = -1
		} else {
			m = 1
		}
		return float32(m * (1.055*math.Pow(abs, 1/2.4) - 0.055))
	}
	return float32(value * 12.92)
}

// MustColorDecode is the same as ColorDecode(), but returns a fully transparent color (the zero value) if an error
// occurs.
func MustColorDecode(buffer string) Color {
	c, _ := ColorDecode(buffer) //nolint:errcheck // Intentional dropping of the error
	return c
}

// ColorDecode creates a Color from a string. The string may be in any of the standard CSS formats:
//
// - CSS predefined color name, e.g. "Yellow"
// - CSS rgb(), e.g. "rgb(255, 127, 0)" or "rgb(100%, 50%, 0%)"
// - CSS rgba(), e.g. "rgba(255, 127, 0, 0.3)" or "rgba(100%, 50%, 0%, 0.3)"
// - CSS short hexadecimal colors, e.g. "#FF0"
// - CSS long hexadecimal colors, e.g. "#FFFF00"
// - CCS hsl(), e.g. "hsl(120, 100%, 50%)"
// - CSS hsla(), e.g. "hsla(120, 100%, 50%, 0.3)"
func ColorDecode(buffer string) (Color, error) {
	const invalid = "invalid color string: %q"
	original := buffer
	buffer = strings.ToLower(strings.TrimSpace(buffer))
	if c, ok := nameToColor[buffer]; ok {
		return c, nil
	}
	switch {
	case strings.HasPrefix(buffer, "#"):
		buffer = buffer[1:]
		switch len(buffer) {
		case 3:
			red, err := strconv.ParseInt(buffer[0:1], 16, 64)
			if err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var green int64
			if green, err = strconv.ParseInt(buffer[1:2], 16, 64); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var blue int64
			if blue, err = strconv.ParseInt(buffer[2:3], 16, 64); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			return RGB(int((red<<4)|red), int((green<<4)|green), int((blue<<4)|blue)), nil
		case 6:
			red, err := strconv.ParseInt(strings.TrimSpace(buffer[0:2]), 16, 64)
			if err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var green int64
			if green, err = strconv.ParseInt(strings.TrimSpace(buffer[2:4]), 16, 64); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var blue int64
			if blue, err = strconv.ParseInt(strings.TrimSpace(buffer[4:6]), 16, 64); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			return RGB(int(red), int(green), int(blue)), nil
		}
	case strings.HasPrefix(buffer, "rgb(") && strings.HasSuffix(buffer, ")"):
		parts := strings.SplitN(strings.TrimSpace(buffer[4:len(buffer)-1]), ",", 4)
		if len(parts) == 3 {
			red, err := extractIntegerOrPercentColorValue(parts[0])
			if err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var green int
			if green, err = extractIntegerOrPercentColorValue(parts[1]); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var blue int
			if blue, err = extractIntegerOrPercentColorValue(parts[2]); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			return RGB(red, green, blue), nil
		}
	case strings.HasPrefix(buffer, "rgba(") && strings.HasSuffix(buffer, ")"):
		parts := strings.SplitN(strings.TrimSpace(buffer[5:len(buffer)-1]), ",", 5)
		if len(parts) == 4 {
			red, err := extractIntegerOrPercentColorValue(parts[0])
			if err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var green int
			if green, err = extractIntegerOrPercentColorValue(parts[1]); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var blue int
			if blue, err = extractIntegerOrPercentColorValue(parts[2]); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var alpha float64
			if alpha, err = strconv.ParseFloat(strings.TrimSpace(parts[3]), 32); err != nil || alpha < 0 || alpha > 1 {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			return ARGB(float32(alpha), red, green, blue), nil
		}
	case strings.HasPrefix(buffer, "hsl(") && strings.HasSuffix(buffer, ")"):
		parts := strings.SplitN(strings.TrimSpace(buffer[4:len(buffer)-1]), ",", 4)
		if len(parts) == 3 {
			hue, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
			if err != nil || hue < 0 || hue > 359 {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var saturation float32
			if saturation, err = extractColorPercentage(parts[1]); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var lightness float32
			if lightness, err = extractColorPercentage(parts[2]); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			return HSL(float32(hue)/360, saturation, lightness), nil
		}
	case strings.HasPrefix(buffer, "hsla(") && strings.HasSuffix(buffer, ")"):
		parts := strings.SplitN(strings.TrimSpace(buffer[5:len(buffer)-1]), ",", 5)
		if len(parts) == 4 {
			hue, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
			if err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			if hue < 0 {
				hue = 360 - ((-hue) % 360)
			} else if hue > 359 {
				hue %= 360
			}
			var saturation float32
			if saturation, err = extractColorPercentage(parts[1]); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var lightness float32
			if lightness, err = extractColorPercentage(parts[2]); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			var alpha float64
			if alpha, err = strconv.ParseFloat(strings.TrimSpace(parts[3]), 32); err != nil {
				return 0, errs.NewWithCausef(err, invalid, original)
			}
			if alpha < 0 {
				alpha = 0
			} else if alpha > 1 {
				alpha = 1
			}
			return HSLA(float32(hue)/360, saturation, lightness, float32(alpha)), nil
		}
	}
	return 0, errs.Newf(invalid, original)
}

func extractIntegerOrPercentColorValue(s string) (int, error) {
	var isPercent bool
	s, isPercent = strings.CutSuffix(strings.TrimSpace(s), "%")
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, err
	}
	switch {
	case v < 0:
		v = 0
	case isPercent:
		if v >= 100 {
			v = 255
		} else {
			v = v * 255 / 100
		}
	case v > 255:
		v = 255
	}
	return v, nil
}

func extractColorPercentage(s string) (float32, error) {
	var isPercent bool
	s, isPercent = strings.CutSuffix(strings.TrimSpace(s), "%")
	if !isPercent {
		return 0, errors.New("expected percentage value")
	}
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, err
	}
	if v < 0 {
		v = 0
	} else if v > 100 {
		v = 100
	}
	return float32(v) / 100, nil
}

// Paint returns a Paint for this Color. Here to satisfy the Ink interface.
func (c Color) Paint(_ *Canvas, _ geom.Rect, style paintstyle.Enum) *Paint {
	paint := NewPaint()
	paint.SetStyle(style)
	paint.SetColor(c)
	return paint
}

// GetColor returns this Color. Here to satisfy the ColorProvider interface.
func (c Color) GetColor() Color {
	return c
}

// String implements the fmt.Stringer interface. The output can be used as a color in CSS.
func (c Color) String() string {
	if name, ok := colorToName[c]; ok {
		return name
	}
	if c.HasAlpha() {
		return fmt.Sprintf("rgba(%d,%d,%d,%v)", c.Red(), c.Green(), c.Blue(), c.AlphaIntensity())
	}
	return fmt.Sprintf("#%02X%02X%02X", c.Red(), c.Green(), c.Blue())
}

// GoString implements the fmt.GoStringer interface.
func (c Color) GoString() string {
	if name, ok := colorToName[c]; ok {
		return name
	}
	if c.HasAlpha() {
		return fmt.Sprintf("ARGB(%v, %d, %d, %d)", c.AlphaIntensity(), c.Red(), c.Green(), c.Blue())
	}
	return fmt.Sprintf("RGB(%d, %d, %d)", c.Red(), c.Green(), c.Blue())
}

// MarshalText implements encoding.TextMarshaler.
func (c Color) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (c *Color) UnmarshalText(text []byte) error {
	clr, err := ColorDecode(string(text))
	if err != nil {
		return err
	}
	*c = clr
	return nil
}

// RGBA implements color.Color. Assumes the color is not premultiplied and must be premultiplied to meet the contract of
// color.Color.
func (c Color) RGBA() (r, g, b, a uint32) {
	a = uint32(c.Alpha())
	r = uint32(c.Red())
	r |= r << 8
	r *= a
	r /= 0xff
	g = uint32(c.Green())
	g |= g << 8
	g *= a
	g /= 0xff
	b = uint32(c.Blue())
	b |= b << 8
	b *= a
	b /= 0xff
	a |= a << 8
	return r, g, b, a
}

// Red returns the red channel, in the range of 0-255.
func (c Color) Red() int {
	return int((c >> 16) & 0xFF)
}

// SetRed returns a new color based on this color, but with the red channel replaced.
func (c Color) SetRed(red int) Color {
	return ARGB(c.AlphaIntensity(), red, c.Green(), c.Blue())
}

// RedIntensity returns the red channel, in the range of 0-1.
func (c Color) RedIntensity() float32 {
	return float32(c.Red()) / 255
}

// SetRedIntensity returns a new color based on this color, but with the red channel replaced.
func (c Color) SetRedIntensity(red float32) Color {
	return ARGB(c.AlphaIntensity(), clamp0To1AndScale255(red), c.Green(), c.Blue())
}

// Green returns the green channel, in the range of 0-255.
func (c Color) Green() int {
	return int((c >> 8) & 0xFF)
}

// SetGreen returns a new color based on this color, but with the green channel replaced.
func (c Color) SetGreen(green int) Color {
	return ARGB(c.AlphaIntensity(), c.Red(), green, c.Blue())
}

// GreenIntensity returns the green channel, in the range of 0-1.
func (c Color) GreenIntensity() float32 {
	return float32(c.Green()) / 255
}

// SetGreenIntensity returns a new color based on this color, but with the green channel replaced.
func (c Color) SetGreenIntensity(green float32) Color {
	return ARGB(c.AlphaIntensity(), c.Red(), clamp0To1AndScale255(green), c.Blue())
}

// Blue returns the blue channel, in the range of 0-255.
func (c Color) Blue() int {
	return int(c & 0xFF)
}

// SetBlue returns a new color based on this color, but with the blue channel replaced.
func (c Color) SetBlue(blue int) Color {
	return ARGB(c.AlphaIntensity(), c.Red(), c.Green(), blue)
}

// BlueIntensity returns the blue channel, in the range of 0-1.
func (c Color) BlueIntensity() float32 {
	return float32(c.Blue()) / 255
}

// SetBlueIntensity returns a new color based on this color, but with the blue channel replaced.
func (c Color) SetBlueIntensity(blue float32) Color {
	return ARGB(c.AlphaIntensity(), c.Red(), c.Green(), clamp0To1AndScale255(blue))
}

// Invisible returns true if the color is fully transparent.
func (c Color) Invisible() bool {
	return c.Alpha() == 0
}

// Opaque returns true if the color is fully opaque.
func (c Color) Opaque() bool {
	return c.Alpha() == 255
}

// HasAlpha returns true if the color is not fully opaque.
func (c Color) HasAlpha() bool {
	return (c & 0xFF000000) != 0xFF000000
}

// Alpha returns the alpha channel, in the range of 0-255.
func (c Color) Alpha() int {
	return int((c >> 24) & 0xFF)
}

// SetAlpha returns a new color based on this color, but with the alpha channel replaced.
func (c Color) SetAlpha(alpha int) Color {
	return Color((clamp0To255(alpha) << 24) | (int(c) & 0x00FFFFFF))
}

// AlphaIntensity returns the alpha channel, in the range of 0-1.
func (c Color) AlphaIntensity() float32 {
	return float32(c.Alpha()) / 255
}

// SetAlphaIntensity returns a new color based on this color, but with the alpha channel replaced.
func (c Color) SetAlphaIntensity(alpha float32) Color {
	return ARGB(alpha, c.Red(), c.Green(), c.Blue())
}

// MultiplyAlpha returns a new color based on this color, but with the alpha channel multiplied by the specified factor.
func (c Color) MultiplyAlpha(factor float32) Color {
	return ARGB(c.AlphaIntensity()*factor, c.Red(), c.Green(), c.Blue())
}

// Monochrome returns true if each color component is the same, making it a shade of gray.
func (c Color) Monochrome() bool {
	green := c.Green()
	return c.Red() == green && green == c.Blue()
}

// Hue of the color, a value from 0-1.
func (c Color) Hue() float32 {
	hue, _, _ := c.HSB()
	return hue
}

// SetHue creates a new color from this color with the specified hue, a value from 0-1.
func (c Color) SetHue(hue float32) Color {
	_, s, b := c.HSB()
	return HSBA(hue, s, b, c.AlphaIntensity())
}

// AdjustHue creates a new color from this color with its hue adjusted by the specified amount.
func (c Color) AdjustHue(amount float32) Color {
	h, s, b := c.HSB()
	return HSBA(h+amount, s, b, c.AlphaIntensity())
}

// Saturation of the color, a value from 0-1.
func (c Color) Saturation() float32 {
	if brightness := c.Brightness(); brightness != 0 {
		return (brightness - (float32(min(c.Red(), c.Green(), c.Blue())) / 255)) / brightness
	}
	return 0
}

// SetSaturation creates a new color from this color with the specified saturation.
func (c Color) SetSaturation(saturation float32) Color {
	h, _, b := c.HSB()
	return HSBA(h, saturation, b, c.AlphaIntensity())
}

// AdjustSaturation creates a new color from this color with its saturation adjusted by the specified amount.
func (c Color) AdjustSaturation(amount float32) Color {
	h, s, b := c.HSB()
	return HSBA(h, s+amount, b, c.AlphaIntensity())
}

// Brightness of the color, a value from 0-1.
func (c Color) Brightness() float32 {
	return float32(max(c.Red(), c.Green(), c.Blue())) / 255
}

// SetBrightness creates a new color from this color with the specified brightness.
func (c Color) SetBrightness(brightness float32) Color {
	h, s, _ := c.HSB()
	return HSBA(h, s, brightness, c.AlphaIntensity())
}

// AdjustBrightness creates a new color from this color with its brightness adjusted by the specified amount.
func (c Color) AdjustBrightness(amount float32) Color {
	h, s, b := c.HSB()
	return HSBA(h, s, b+amount, c.AlphaIntensity())
}

// HSB returns the hue, saturation and brightness of the color. Values are in the range 0-1.
func (c Color) HSB() (hue, saturation, brightness float32) {
	r := c.Red()
	g := c.Green()
	b := c.Blue()
	cMax := max(r, g, b)
	cMin := min(r, g, b)
	brightness = float32(cMax) / 255
	if cMax != 0 {
		saturation = float32(cMax-cMin) / float32(cMax)
	} else {
		saturation = 0
	}
	if saturation == 0 {
		hue = 0
	} else {
		div := float32(cMax - cMin)
		rc := float32(cMax-r) / div
		gc := float32(cMax-g) / div
		bc := float32(cMax-b) / div
		switch {
		case r == cMax:
			hue = bc - gc
		case g == cMax:
			hue = 2 + rc - bc
		default:
			hue = 4 + gc - rc
		}
		hue /= 6
		if hue < 0 {
			hue++
		}
	}
	return hue, saturation, brightness
}

// PerceivedLightness returns a value from 0-1 representing the perceived lightness. Lower values represent darker
// colors, while higher values represent brighter colors. This is the same as the lightness value returned by calling
// the .OKLCH() method.
func (c Color) PerceivedLightness() float32 {
	lr := toLinear(float64(c.RedIntensity()))
	lg := toLinear(float64(c.GreenIntensity()))
	lb := toLinear(float64(c.BlueIntensity()))
	L := math.Cbrt(0.41222147079999993*lr + 0.5363325363*lg + 0.0514459929*lb)
	M := math.Cbrt(0.2119034981999999*lr + 0.6806995450999999*lg + 0.1073969566*lb)
	S := math.Cbrt(0.08830246189999998*lr + 0.2817188376*lg + 0.6299787005000002*lb)
	return clamp0To1(float32(0.2104542553*L + 0.793617785*M - 0.0040720468*S))
}

// AdjustPerceivedLightness returns a new color based on this color with its perceived lightness adjusted by the
// specified amount.
func (c Color) AdjustPerceivedLightness(adj float32) Color {
	rl, rc, rh := c.OKLCH()
	return OKLCH(rl+adj, rc, rh, c.AlphaIntensity())
}

// Colors used for the On() method.
var (
	OnLight = RGB(16, 16, 16)
	OnDark  = RGB(240, 240, 240)
)

// On returns OnLight if the input color is light, otherwise OnDark.
func (c Color) On() Color {
	return c.OnCustom(OnLight, OnDark)
}

// OnCustom returns onLightColor if the input color is light, otherwise onDarkColor.
func (c Color) OnCustom(onLightColor, onDarkColor Color) Color {
	if c.PerceivedLightness() > 0.6 {
		return onLightColor
	}
	return onDarkColor
}

// OKLCH returns the lightness (0-1), chroma (0-0.37), and hue (0-360) values using the OKLCH color space.
func (c Color) OKLCH() (rl, rc, rh float32) {
	lr := toLinear(float64(c.RedIntensity()))
	lg := toLinear(float64(c.GreenIntensity()))
	lb := toLinear(float64(c.BlueIntensity()))
	L := math.Cbrt(0.41222147079999993*lr + 0.5363325363*lg + 0.0514459929*lb)
	M := math.Cbrt(0.2119034981999999*lr + 0.6806995450999999*lg + 0.1073969566*lb)
	S := math.Cbrt(0.08830246189999998*lr + 0.2817188376*lg + 0.6299787005000002*lb)
	b := c.Blue()
	if c.Red() != b || b != c.Green() {
		ra := 1.9779984951*L - 2.428592205*M + 0.4505937099*S
		rb := 0.0259040371*L + 0.7827717662*M - 0.808675766*S
		if rc = float32(math.Sqrt(ra*ra + rb*rb)); rc < 0 {
			rc = 0
		} else {
			rc = clampChromaForOKLCH(rc)
		}
		if rc != 0 {
			rh = normalizeHue(math.Atan2(rb, ra) * 180 / math.Pi)
		}
	}
	return clamp0To1(float32(0.2104542553*L + 0.793617785*M - 0.0040720468*S)), rc, rh
}

func toLinear(value float64) float64 {
	abs := math.Abs(value)
	if abs < 0.04045 {
		return value / 12.92
	}
	var m float64
	if math.Signbit(value) {
		m = -1
	} else {
		m = 1
	}
	return m * math.Pow((abs+0.055)/1.055, 2.4)
}

// NormalizeOKLCH returns the normalized lightness (0-1), chroma (0-0.37), and hue (0-360) values using the OKLCH color
// space.
func NormalizeOKLCH(lightness, chroma, hue, alpha float32) (l, c, h, a float32) {
	return clamp0To1(lightness), clampChromaForOKLCH(chroma), normalizeHue(float64(hue)), clamp0To1(alpha)
}

// Blend blends this color with another color. pct is the amount of the other
// color to use.
func (c Color) Blend(other Color, pct float32) Color {
	pct = clamp0To1(pct)
	rem := 1 - pct
	return ARGB(c.AlphaIntensity(), clamp0To1AndScale255(c.RedIntensity()*rem+other.RedIntensity()*pct), clamp0To1AndScale255(c.GreenIntensity()*rem+other.GreenIntensity()*pct), clamp0To1AndScale255(c.BlueIntensity()*rem+other.BlueIntensity()*pct))
}

// Premultiply multiplies the color channels by the alpha channel.
func (c Color) Premultiply() Color {
	alpha := c.Alpha()
	switch alpha {
	case 0:
		return 0
	case 255:
		return c
	default:
		a := uint32(alpha)
		r := uint32(c.Red())
		r |= r << 8
		r *= a
		r /= 0xff
		g := uint32(c.Green())
		g |= g << 8
		g *= a
		g /= 0xff
		b := uint32(c.Blue())
		b |= b << 8
		b *= a
		b /= 0xff
		return ARGB(c.AlphaIntensity(), int(r>>8), int(g>>8), int(b>>8))
	}
}

// Unpremultiply divides the color channels by the alpha channel, effectively undoing a Premultiply call. Note that you
// will not necessarily get the original value back after calling Premultiply followed by Unpremultiply.
func (c Color) Unpremultiply() Color {
	alpha := c.Alpha()
	switch alpha {
	case 0:
		return 0
	case 255:
		return c
	default:
		a := uint32(alpha)
		r := uint32(c.Red())
		r |= r << 8
		r *= 0xff
		r /= a
		g := uint32(c.Green())
		g |= g << 8
		g *= 0xff
		g /= a
		b := uint32(c.Blue())
		b |= b << 8
		b *= 0xff
		b /= a
		return ARGB(c.AlphaIntensity(), int(r>>8), int(g>>8), int(b>>8))
	}
}

func normalizeHue(hue float64) float32 {
	hue = math.Mod(hue, 360)
	if hue < 0 {
		hue += 360
	}
	h := float32(hue)
	if h >= 360 {
		// Guard against float rounding: a tiny negative input can land on exactly 360 after the conversion to float32,
		// which would violate the [0,360) contract.
		h = 0
	}
	return h
}

func clamp0To1(value float32) float32 {
	return min(max(value, 0), 1)
}

func clamp0To255(value int) int {
	return min(max(value, 0), 255)
}

func clamp0To1AndScale255(value float32) int {
	return clamp0To255(int(clamp0To1(value)*255 + 0.5))
}

func clampChromaForOKLCH(value float32) float32 {
	return min(max(value, 0), 0.37)
}

// CSS named colors.
var (
	AliceBlue            = registerColor("AliceBlue", RGB(240, 248, 255))
	AntiqueWhite         = registerColor("AntiqueWhite", RGB(250, 235, 215))
	Aqua                 = registerColor("Aqua", RGB(0, 255, 255))
	Aquamarine           = registerColor("Aquamarine", RGB(127, 255, 212))
	Azure                = registerColor("Azure", RGB(240, 255, 255))
	Beige                = registerColor("Beige", RGB(245, 245, 220))
	Bisque               = registerColor("Bisque", RGB(255, 228, 196))
	Black                = registerColor("Black", RGB(0, 0, 0))
	BlanchedAlmond       = registerColor("BlanchedAlmond", RGB(255, 235, 205))
	Blue                 = registerColor("Blue", RGB(0, 0, 255))
	BlueViolet           = registerColor("BlueViolet", RGB(138, 43, 226))
	Brown                = registerColor("Brown", RGB(165, 42, 42))
	BurlyWood            = registerColor("BurlyWood", RGB(222, 184, 135))
	CadetBlue            = registerColor("CadetBlue", RGB(95, 158, 160))
	Chartreuse           = registerColor("Chartreuse", RGB(127, 255, 0))
	Chocolate            = registerColor("Chocolate", RGB(210, 105, 30))
	Coral                = registerColor("Coral", RGB(255, 127, 80))
	CornflowerBlue       = registerColor("CornflowerBlue", RGB(100, 149, 237))
	Cornsilk             = registerColor("Cornsilk", RGB(255, 248, 220))
	Crimson              = registerColor("Crimson", RGB(220, 20, 60))
	Cyan                 = registerColor("Cyan", RGB(0, 255, 255))
	DarkBlue             = registerColor("DarkBlue", RGB(0, 0, 139))
	DarkCyan             = registerColor("DarkCyan", RGB(0, 139, 139))
	DarkGoldenRod        = registerColor("DarkGoldenRod", RGB(184, 134, 11))
	DarkGray             = registerColor("DarkGray", RGB(169, 169, 169))
	DarkGreen            = registerColor("DarkGreen", RGB(0, 100, 0))
	DarkGrey             = registerColor("DarkGrey", RGB(169, 169, 169))
	DarkKhaki            = registerColor("DarkKhaki", RGB(189, 183, 107))
	DarkMagenta          = registerColor("DarkMagenta", RGB(139, 0, 139))
	DarkOliveGreen       = registerColor("DarkOliveGreen", RGB(85, 107, 47))
	DarkOrange           = registerColor("DarkOrange", RGB(255, 140, 0))
	DarkOrchid           = registerColor("DarkOrchid", RGB(153, 50, 204))
	DarkRed              = registerColor("DarkRed", RGB(139, 0, 0))
	DarkSalmon           = registerColor("DarkSalmon", RGB(233, 150, 122))
	DarkSeaGreen         = registerColor("DarkSeaGreen", RGB(143, 188, 143))
	DarkSlateBlue        = registerColor("DarkSlateBlue", RGB(72, 61, 139))
	DarkSlateGray        = registerColor("DarkSlateGray", RGB(47, 79, 79))
	DarkSlateGrey        = registerColor("DarkSlateGrey", RGB(47, 79, 79))
	DarkTurquoise        = registerColor("DarkTurquoise", RGB(0, 206, 209))
	DarkViolet           = registerColor("DarkViolet", RGB(148, 0, 211))
	DeepPink             = registerColor("DeepPink", RGB(255, 20, 147))
	DeepSkyBlue          = registerColor("DeepSkyBlue", RGB(0, 191, 255))
	DimGray              = registerColor("DimGray", RGB(105, 105, 105))
	DimGrey              = registerColor("DimGrey", RGB(105, 105, 105))
	DodgerBlue           = registerColor("DodgerBlue", RGB(30, 144, 255))
	FireBrick            = registerColor("FireBrick", RGB(178, 34, 34))
	FloralWhite          = registerColor("FloralWhite", RGB(255, 250, 240))
	ForestGreen          = registerColor("ForestGreen", RGB(34, 139, 34))
	Fuchsia              = registerColor("Fuchsia", RGB(255, 0, 255))
	Gainsboro            = registerColor("Gainsboro", RGB(220, 220, 220))
	GhostWhite           = registerColor("GhostWhite", RGB(248, 248, 255))
	Gold                 = registerColor("Gold", RGB(255, 215, 0))
	GoldenRod            = registerColor("GoldenRod", RGB(218, 165, 32))
	Gray                 = registerColor("Gray", RGB(128, 128, 128))
	Green                = registerColor("Green", RGB(0, 128, 0))
	GreenYellow          = registerColor("GreenYellow", RGB(173, 255, 47))
	Grey                 = registerColor("Grey", RGB(128, 128, 128))
	HoneyDew             = registerColor("HoneyDew", RGB(240, 255, 240))
	HotPink              = registerColor("HotPink", RGB(255, 105, 180))
	IndianRed            = registerColor("IndianRed", RGB(205, 92, 92))
	Indigo               = registerColor("Indigo", RGB(75, 0, 130))
	Ivory                = registerColor("Ivory", RGB(255, 255, 240))
	Khaki                = registerColor("Khaki", RGB(240, 230, 140))
	Lavender             = registerColor("Lavender", RGB(230, 230, 250))
	LavenderBlush        = registerColor("LavenderBlush", RGB(255, 240, 245))
	LawnGreen            = registerColor("LawnGreen", RGB(124, 252, 0))
	LemonChiffon         = registerColor("LemonChiffon", RGB(255, 250, 205))
	LightBlue            = registerColor("LightBlue", RGB(173, 216, 230))
	LightCoral           = registerColor("LightCoral", RGB(240, 128, 128))
	LightCyan            = registerColor("LightCyan", RGB(224, 255, 255))
	LightGoldenRodYellow = registerColor("LightGoldenRodYellow", RGB(250, 250, 210))
	LightGray            = registerColor("LightGray", RGB(211, 211, 211))
	LightGreen           = registerColor("LightGreen", RGB(144, 238, 144))
	LightGrey            = registerColor("LightGrey", RGB(211, 211, 211))
	LightPink            = registerColor("LightPink", RGB(255, 182, 193))
	LightSalmon          = registerColor("LightSalmon", RGB(255, 160, 122))
	LightSeaGreen        = registerColor("LightSeaGreen", RGB(32, 178, 170))
	LightSkyBlue         = registerColor("LightSkyBlue", RGB(135, 206, 250))
	LightSlateGray       = registerColor("LightSlateGray", RGB(119, 136, 153))
	LightSlateGrey       = registerColor("LightSlateGrey", RGB(119, 136, 153))
	LightSteelBlue       = registerColor("LightSteelBlue", RGB(176, 196, 222))
	LightYellow          = registerColor("LightYellow", RGB(255, 255, 224))
	Lime                 = registerColor("Lime", RGB(0, 255, 0))
	LimeGreen            = registerColor("LimeGreen", RGB(50, 205, 50))
	Linen                = registerColor("Linen", RGB(250, 240, 230))
	Magenta              = registerColor("Magenta", RGB(255, 0, 255))
	Maroon               = registerColor("Maroon", RGB(128, 0, 0))
	MediumAquaMarine     = registerColor("MediumAquaMarine", RGB(102, 205, 170))
	MediumBlue           = registerColor("MediumBlue", RGB(0, 0, 205))
	MediumOrchid         = registerColor("MediumOrchid", RGB(186, 85, 211))
	MediumPurple         = registerColor("MediumPurple", RGB(147, 112, 219))
	MediumSeaGreen       = registerColor("MediumSeaGreen", RGB(60, 179, 113))
	MediumSlateBlue      = registerColor("MediumSlateBlue", RGB(123, 104, 238))
	MediumSpringGreen    = registerColor("MediumSpringGreen", RGB(0, 250, 154))
	MediumTurquoise      = registerColor("MediumTurquoise", RGB(72, 209, 204))
	MediumVioletRed      = registerColor("MediumVioletRed", RGB(199, 21, 133))
	MidnightBlue         = registerColor("MidnightBlue", RGB(25, 25, 112))
	MintCream            = registerColor("MintCream", RGB(245, 255, 250))
	MistyRose            = registerColor("MistyRose", RGB(255, 228, 225))
	Moccasin             = registerColor("Moccasin", RGB(255, 228, 181))
	NavajoWhite          = registerColor("NavajoWhite", RGB(255, 222, 173))
	Navy                 = registerColor("Navy", RGB(0, 0, 128))
	OldLace              = registerColor("OldLace", RGB(253, 245, 230))
	Olive                = registerColor("Olive", RGB(128, 128, 0))
	OliveDrab            = registerColor("OliveDrab", RGB(107, 142, 35))
	Orange               = registerColor("Orange", RGB(255, 165, 0))
	OrangeRed            = registerColor("OrangeRed", RGB(255, 69, 0))
	Orchid               = registerColor("Orchid", RGB(218, 112, 214))
	PaleGoldenRod        = registerColor("PaleGoldenRod", RGB(238, 232, 170))
	PaleGreen            = registerColor("PaleGreen", RGB(152, 251, 152))
	PaleTurquoise        = registerColor("PaleTurquoise", RGB(175, 238, 238))
	PaleVioletRed        = registerColor("PaleVioletRed", RGB(219, 112, 147))
	PapayaWhip           = registerColor("PapayaWhip", RGB(255, 239, 213))
	PeachPuff            = registerColor("PeachPuff", RGB(255, 218, 185))
	Peru                 = registerColor("Peru", RGB(205, 133, 63))
	Pink                 = registerColor("Pink", RGB(255, 192, 203))
	Plum                 = registerColor("Plum", RGB(221, 160, 221))
	PowderBlue           = registerColor("PowderBlue", RGB(176, 224, 230))
	Purple               = registerColor("Purple", RGB(128, 0, 128))
	Red                  = registerColor("Red", RGB(255, 0, 0))
	RosyBrown            = registerColor("RosyBrown", RGB(188, 143, 143))
	RoyalBlue            = registerColor("RoyalBlue", RGB(65, 105, 225))
	SaddleBrown          = registerColor("SaddleBrown", RGB(139, 69, 19))
	Salmon               = registerColor("Salmon", RGB(250, 128, 114))
	SandyBrown           = registerColor("SandyBrown", RGB(244, 164, 96))
	SeaGreen             = registerColor("SeaGreen", RGB(46, 139, 87))
	SeaShell             = registerColor("SeaShell", RGB(255, 245, 238))
	Sienna               = registerColor("Sienna", RGB(160, 82, 45))
	Silver               = registerColor("Silver", RGB(192, 192, 192))
	SkyBlue              = registerColor("SkyBlue", RGB(135, 206, 235))
	SlateBlue            = registerColor("SlateBlue", RGB(106, 90, 205))
	SlateGray            = registerColor("SlateGray", RGB(112, 128, 144))
	SlateGrey            = registerColor("SlateGrey", RGB(112, 128, 144))
	Snow                 = registerColor("Snow", RGB(255, 250, 250))
	SpringGreen          = registerColor("SpringGreen", RGB(0, 255, 127))
	SteelBlue            = registerColor("SteelBlue", RGB(70, 130, 180))
	Tan                  = registerColor("Tan", RGB(210, 180, 140))
	Teal                 = registerColor("Teal", RGB(0, 128, 128))
	Thistle              = registerColor("Thistle", RGB(216, 191, 216))
	Tomato               = registerColor("Tomato", RGB(255, 99, 71))
	Transparent          = registerColor("None", Color(0))
	Turquoise            = registerColor("Turquoise", RGB(64, 224, 208))
	Violet               = registerColor("Violet", RGB(238, 130, 238))
	Wheat                = registerColor("Wheat", RGB(245, 222, 179))
	White                = registerColor("White", RGB(255, 255, 255))
	WhiteSmoke           = registerColor("WhiteSmoke", RGB(245, 245, 245))
	Yellow               = registerColor("Yellow", RGB(255, 255, 0))
	YellowGreen          = registerColor("YellowGreen", RGB(154, 205, 50))
)

func registerColor(name string, c Color) Color {
	nameToColor[strings.ToLower(name)] = c
	colorToName[c] = name
	return c
}
