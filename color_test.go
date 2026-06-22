// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"image/color"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison"
)

func TestColorConstructors(t *testing.T) {
	chk := check.New(t)

	// RGB produces an opaque color.
	chk.Equal(unison.Color(0xFFFF0000), unison.RGB(255, 0, 0))
	chk.Equal(unison.Red, unison.RGB(255, 0, 0))

	// RGB clamps out-of-range channel values.
	chk.Equal(unison.Color(0xFFFF00FF), unison.RGB(300, -20, 1000))

	// ARGB scales the 0-1 alpha into the high byte.
	chk.Equal(unison.Color(0x80FF0000), unison.ARGB(0.5, 255, 0, 0))
	chk.Equal(unison.Color(0x00FF0000), unison.ARGB(0, 255, 0, 0))
	chk.Equal(unison.Color(0xFFFF0000), unison.ARGB(1, 255, 0, 0))

	// ARGB clamps alpha to the 0-1 range.
	chk.Equal(unison.Color(0xFFFF0000), unison.ARGB(5, 255, 0, 0))
	chk.Equal(unison.Color(0x00FF0000), unison.ARGB(-5, 255, 0, 0))

	// ARGBfloat treats every component as a 0-1 intensity.
	chk.Equal(unison.Color(0xFFFF0000), unison.ARGBfloat(1, 1, 0, 0))
	chk.Equal(unison.Color(0x80804000), unison.ARGBfloat(0.5, 0.5, 0.25, 0))
}

func TestColorFromNRGBA(t *testing.T) {
	chk := check.New(t)
	chk.Equal(unison.Color(0x80FF8040), unison.ColorFromNRGBA(color.NRGBA{R: 0xFF, G: 0x80, B: 0x40, A: 0x80}))
	chk.Equal(unison.Red, unison.ColorFromNRGBA(color.NRGBA{R: 0xFF, A: 0xFF}))
}

func TestColorChannels(t *testing.T) {
	chk := check.New(t)
	c := unison.ARGB(0.5, 10, 20, 30)
	chk.Equal(10, c.Red())
	chk.Equal(20, c.Green())
	chk.Equal(30, c.Blue())
	chk.Equal(128, c.Alpha())

	chk.Equal(float32(10)/255, c.RedIntensity())
	chk.Equal(float32(20)/255, c.GreenIntensity())
	chk.Equal(float32(30)/255, c.BlueIntensity())
	chk.Equal(float32(128)/255, c.AlphaIntensity())
}

func TestColorSetChannels(t *testing.T) {
	chk := check.New(t)
	c := unison.RGB(10, 20, 30)
	chk.Equal(unison.RGB(99, 20, 30), c.SetRed(99))
	chk.Equal(unison.RGB(10, 99, 30), c.SetGreen(99))
	chk.Equal(unison.RGB(10, 20, 99), c.SetBlue(99))
	chk.Equal(unison.ARGB(0.5, 10, 20, 30), c.SetAlpha(128))

	chk.Equal(unison.RGB(255, 20, 30), c.SetRedIntensity(1))
	chk.Equal(unison.RGB(10, 255, 30), c.SetGreenIntensity(1))
	chk.Equal(unison.RGB(10, 20, 255), c.SetBlueIntensity(1))
	chk.Equal(unison.ARGB(0, 10, 20, 30), c.SetAlphaIntensity(0))

	// Setting a channel preserves the existing alpha.
	a := unison.ARGB(0.25, 10, 20, 30)
	chk.Equal(64, a.SetRed(99).Alpha())
}

func TestColorMultiplyAlpha(t *testing.T) {
	chk := check.New(t)
	chk.Equal(unison.ARGB(0.5, 255, 0, 0), unison.Red.MultiplyAlpha(0.5))
	chk.Equal(unison.ARGB(0.25, 255, 0, 0), unison.ARGB(0.5, 255, 0, 0).MultiplyAlpha(0.5))
	chk.Equal(unison.Red, unison.Red.MultiplyAlpha(5)) // Clamped to opaque
}

func TestColorPredicates(t *testing.T) {
	chk := check.New(t)

	chk.True(unison.Transparent.Invisible())
	chk.False(unison.Red.Invisible())

	chk.True(unison.Red.Opaque())
	chk.False(unison.ARGB(0.5, 255, 0, 0).Opaque())

	chk.False(unison.Red.HasAlpha())
	chk.True(unison.ARGB(0.5, 255, 0, 0).HasAlpha())
	chk.True(unison.Transparent.HasAlpha())

	chk.True(unison.Gray.Monochrome())
	chk.True(unison.Black.Monochrome())
	chk.True(unison.White.Monochrome())
	chk.False(unison.Red.Monochrome())
}

func TestColorString(t *testing.T) {
	chk := check.New(t)

	// Named colors round-trip to their name.
	chk.Equal("Red", unison.Red.String())
	chk.Equal("None", unison.Transparent.String())

	// Opaque, unnamed colors render as hex.
	chk.Equal("#0A141E", unison.RGB(10, 20, 30).String())

	// Colors with alpha render as rgba().
	chk.Equal("rgba(255,0,0,0.5019608)", unison.ARGB(0.5, 255, 0, 0).String())

	// GoString renders constructor-style output.
	chk.Equal("Red", unison.Red.GoString())
	chk.Equal("RGB(10, 20, 30)", unison.RGB(10, 20, 30).GoString())
	chk.Equal("ARGB(0.5019608, 255, 0, 0)", unison.ARGB(0.5, 255, 0, 0).GoString())
}

func TestColorMarshalText(t *testing.T) {
	chk := check.New(t)

	text, err := unison.RGB(10, 20, 30).MarshalText()
	chk.NoError(err)
	chk.Equal("#0A141E", string(text))

	var c unison.Color
	chk.NoError(c.UnmarshalText([]byte("#0A141E")))
	chk.Equal(unison.RGB(10, 20, 30), c)

	chk.HasError(c.UnmarshalText([]byte("not-a-color")))
}

func TestColorDecodeNamed(t *testing.T) {
	chk := check.New(t)
	chk.Equal(unison.Yellow, unison.MustColorDecode("Yellow"))
	chk.Equal(unison.Yellow, unison.MustColorDecode("yellow"))
	chk.Equal(unison.Yellow, unison.MustColorDecode("  YELLOW  "))
	chk.Equal(unison.Transparent, unison.MustColorDecode("None"))
}

func TestColorDecodeHex(t *testing.T) {
	chk := check.New(t)
	chk.Equal(unison.Yellow, unison.MustColorDecode("#FF0"))
	chk.Equal(unison.Yellow, unison.MustColorDecode("#FFFF00"))
	chk.Equal(unison.RGB(0x11, 0x22, 0x33), unison.MustColorDecode("#123"))
	chk.Equal(unison.RGB(0xAB, 0xCD, 0xEF), unison.MustColorDecode("#abcdef"))

	// Malformed hex strings fail.
	_, err := unison.ColorDecode("#GG0")
	chk.HasError(err)
	_, err = unison.ColorDecode("#12345")
	chk.HasError(err)
}

func TestColorDecodeRGB(t *testing.T) {
	chk := check.New(t)
	chk.Equal(unison.RGB(255, 127, 0), unison.MustColorDecode("rgb(255, 127, 0)"))
	chk.Equal(unison.RGB(255, 127, 0), unison.MustColorDecode("rgb(100%, 50%, 0%)"))
	chk.Equal(unison.ARGB(0.3, 255, 127, 0), unison.MustColorDecode("rgba(255, 127, 0, 0.3)"))
	chk.Equal(unison.ARGB(0.3, 255, 127, 0), unison.MustColorDecode("rgba(100%, 50%, 0%, 0.3)"))

	// Out-of-range channels clamp instead of failing.
	chk.Equal(unison.RGB(255, 0, 0), unison.MustColorDecode("rgb(999, -5, 0)"))

	// Wrong component counts and bad alpha values fail.
	_, err := unison.ColorDecode("rgb(1, 2)")
	chk.HasError(err)
	_, err = unison.ColorDecode("rgb(1, x, 3)")
	chk.HasError(err)
	_, err = unison.ColorDecode("rgba(1, 2, 3, 2)")
	chk.HasError(err)
}

func TestColorDecodeHSL(t *testing.T) {
	chk := check.New(t)
	chk.Equal(unison.Lime, unison.MustColorDecode("hsl(120, 100%, 50%)"))
	chk.Equal(unison.Red, unison.MustColorDecode("hsl(0, 100%, 50%)"))
	chk.Equal(unison.Green, unison.MustColorDecode("hsl(120, 100%, 25%)"))
	chk.Equal(unison.ARGB(0.3, 0, 255, 0), unison.MustColorDecode("hsla(120, 100%, 50%, 0.3)"))

	// hsla wraps out-of-range hues and clamps alpha.
	chk.Equal(unison.MustColorDecode("hsla(120, 100%, 50%, 1)"), unison.MustColorDecode("hsla(480, 100%, 50%, 5)"))

	// hsl requires percentages for saturation and brightness.
	_, err := unison.ColorDecode("hsl(120, 100, 50%)")
	chk.HasError(err)
	// hsl hue must be within 0-359.
	_, err = unison.ColorDecode("hsl(400, 100%, 50%)")
	chk.HasError(err)
}

func TestColorDecodeInvalid(t *testing.T) {
	chk := check.New(t)
	for _, s := range []string{"", "bogus", "rgb()", "#", "rgb(1,2,3", "hsl(1,2,3,4,5)"} {
		_, err := unison.ColorDecode(s)
		chk.HasError(err, s)
	}
	// MustColorDecode falls back to the zero Color (transparent) on error.
	chk.Equal(unison.Transparent, unison.MustColorDecode("bogus"))
}

func TestColorRGBA(t *testing.T) {
	chk := check.New(t)

	// Opaque red premultiplies to full-range red.
	r, g, b, a := unison.Red.RGBA()
	chk.Equal(uint32(0xFFFF), r)
	chk.Equal(uint32(0), g)
	chk.Equal(uint32(0), b)
	chk.Equal(uint32(0xFFFF), a)

	// Fully transparent premultiplies all channels to zero.
	r, g, b, a = unison.ARGB(0, 255, 255, 255).RGBA()
	chk.Equal(uint32(0), r)
	chk.Equal(uint32(0), g)
	chk.Equal(uint32(0), b)
	chk.Equal(uint32(0), a)

	// Color satisfies the standard color.Color interface.
	var _ color.Color = unison.Red
}

func TestColorHSB(t *testing.T) {
	chk := check.New(t)

	h, s, b := unison.Red.HSB()
	chk.Equal(float32(0), h)
	chk.Equal(float32(1), s)
	chk.Equal(float32(1), b)

	chk.Equal(float32(0), unison.Black.Brightness())
	chk.Equal(float32(1), unison.White.Brightness())
	chk.Equal(float32(0), unison.White.Saturation())
	chk.Equal(float32(1), unison.Red.Saturation())

	// Round-trip primary colors through HSB.
	for _, c := range []unison.Color{unison.Red, unison.Green, unison.Blue, unison.Yellow, unison.Cyan, unison.Magenta} {
		h, s, b = c.HSB()
		chk.Equal(c, unison.HSB(h, s, b), c.String())
	}

	// A gray has zero saturation and brightness equal to its level.
	gray := unison.RGB(128, 128, 128)
	h, s, b = gray.HSB()
	chk.Equal(float32(0), h)
	chk.Equal(float32(0), s)
	chk.Equal(float32(128)/255, b)
}

func TestColorHSL(t *testing.T) {
	chk := check.New(t)

	// Standard CSS HSL reference points.
	chk.Equal(unison.Red, unison.HSL(0, 1, 0.5))
	chk.Equal(unison.Lime, unison.HSL(120.0/360, 1, 0.5))
	chk.Equal(unison.Blue, unison.HSL(240.0/360, 1, 0.5))
	chk.Equal(unison.Green, unison.HSL(120.0/360, 1, 0.25))

	// Lightness extremes are pure black and white regardless of hue/saturation.
	chk.Equal(unison.Black, unison.HSL(0.5, 1, 0))
	chk.Equal(unison.White, unison.HSL(0.5, 1, 1))

	// Zero saturation yields a gray at the given lightness.
	chk.Equal(unison.RGB(128, 128, 128), unison.HSL(0.25, 0, 128.0/255))

	// HSLA carries alpha through; HSL is opaque.
	chk.Equal(unison.ARGB(0.5, 255, 0, 0), unison.HSLA(0, 1, 0.5, 0.5))
	chk.Equal(unison.HSLA(0, 1, 0.5, 1), unison.HSL(0, 1, 0.5))

	// Hue wraps around the unit interval.
	chk.Equal(unison.HSL(0, 1, 0.5), unison.HSL(1, 1, 0.5))
}

func TestColorHueSaturationBrightness(t *testing.T) {
	chk := check.New(t)

	// Set replaces a single HSB component. Applying green's hue to fully
	// saturated, full-brightness red yields Lime (pure green).
	chk.Equal(unison.Lime, unison.Red.SetHue(unison.Green.Hue()))
	chk.Equal(float32(0), unison.Red.SetSaturation(0).Saturation())
	chk.Equal(unison.RGB(128, 0, 0), unison.Red.SetBrightness(128.0/255))

	// Adjust nudges a component while keeping the others.
	chk.Equal(unison.Red.AdjustHue(0.5), unison.Red.SetHue(unison.Red.Hue()+0.5))
	chk.Equal(unison.Red.SetSaturation(0.5), unison.Red.AdjustSaturation(-0.5))
	chk.Equal(unison.Red.SetBrightness(0.5), unison.Red.AdjustBrightness(-0.5))
}

func TestColorBlend(t *testing.T) {
	chk := check.New(t)

	// Blending halfway between red and blue gives the midpoint.
	chk.Equal(unison.RGB(128, 0, 128), unison.Red.Blend(unison.Blue, 0.5))

	// The endpoints select one or the other color.
	chk.Equal(unison.Red, unison.Red.Blend(unison.Blue, 0))
	chk.Equal(unison.RGB(0, 0, 255), unison.Red.Blend(unison.Blue, 1))

	// pct is clamped to the 0-1 range.
	chk.Equal(unison.Red.Blend(unison.Blue, 1), unison.Red.Blend(unison.Blue, 5))
	chk.Equal(unison.Red.Blend(unison.Blue, 0), unison.Red.Blend(unison.Blue, -5))

	// Blend keeps the receiver's alpha.
	chk.Equal(128, unison.ARGB(0.5, 255, 0, 0).Blend(unison.Blue, 0.5).Alpha())
}

func TestColorPremultiply(t *testing.T) {
	chk := check.New(t)

	// Opaque and transparent are handled as special cases.
	chk.Equal(unison.Red, unison.Red.Premultiply())
	chk.Equal(unison.Color(0), unison.ARGB(0, 255, 0, 0).Premultiply())

	// Half-alpha white halves each channel while keeping alpha.
	pm := unison.ARGB(0.5, 255, 255, 255).Premultiply()
	chk.Equal(128, pm.Alpha())
	chk.Equal(128, pm.Red())
	chk.Equal(128, pm.Green())
	chk.Equal(128, pm.Blue())

	// Unpremultiply reverses the special cases exactly.
	chk.Equal(unison.Red, unison.Red.Unpremultiply())
	chk.Equal(unison.Color(0), unison.ARGB(0, 255, 0, 0).Unpremultiply())

	// Unpremultiply undoes Premultiply for fully saturated channels.
	chk.Equal(unison.ARGB(0.5, 255, 255, 255), pm.Unpremultiply())
}

func TestColorOnAndPerceivedLightness(t *testing.T) {
	chk := check.New(t)

	// Light colors get the dark "on" color and vice versa.
	chk.Equal(unison.OnLight, unison.White.On())
	chk.Equal(unison.OnDark, unison.Black.On())

	chk.Equal(unison.Red, unison.White.OnCustom(unison.Red, unison.Blue))
	chk.Equal(unison.Blue, unison.Black.OnCustom(unison.Red, unison.Blue))

	// PerceivedLightness is ordered black < gray < white.
	chk.True(unison.Black.PerceivedLightness() < unison.Gray.PerceivedLightness())
	chk.True(unison.Gray.PerceivedLightness() < unison.White.PerceivedLightness())
	chk.Equal(float32(0), unison.Black.PerceivedLightness())
	chk.Equal(float32(1), unison.White.PerceivedLightness())

	// PerceivedLightness matches the lightness reported by OKLCH.
	l, _, _ := unison.Red.OKLCH()
	chk.Equal(l, unison.Red.PerceivedLightness())
}

func TestColorAdjustPerceivedLightness(t *testing.T) {
	chk := check.New(t)
	brighter := unison.Gray.AdjustPerceivedLightness(0.2)
	chk.True(brighter.PerceivedLightness() > unison.Gray.PerceivedLightness())
	darker := unison.Gray.AdjustPerceivedLightness(-0.2)
	chk.True(darker.PerceivedLightness() < unison.Gray.PerceivedLightness())
}

func TestNormalizeOKLCH(t *testing.T) {
	chk := check.New(t)

	l, c, h, a := unison.NormalizeOKLCH(0.5, 0.1, 90, 0.5)
	chk.Equal(float32(0.5), l)
	chk.Equal(float32(0.1), c)
	chk.Equal(float32(90), h)
	chk.Equal(float32(0.5), a)

	// Lightness, chroma, and alpha clamp; hue wraps.
	l, c, h, a = unison.NormalizeOKLCH(5, 5, 450, 5)
	chk.Equal(float32(1), l)
	chk.Equal(float32(0.37), c)
	chk.Equal(float32(90), h)
	chk.Equal(float32(1), a)

	l, c, _, a = unison.NormalizeOKLCH(-5, -5, 0, -5)
	chk.Equal(float32(0), l)
	chk.Equal(float32(0), c)
	chk.Equal(float32(0), a)
}

func TestOKLCH(t *testing.T) {
	chk := check.New(t)
	chk.Equal(unison.White, unison.OKLCH(1, 0, 0, 1))
	l, c, h := unison.White.OKLCH()
	chk.Equal(float32(1), l)
	chk.Equal(float32(0), c)
	chk.Equal(float32(0), h)

	chk.Equal(unison.Black, unison.OKLCH(0, 0, 0, 1))
	l, c, h = unison.Black.OKLCH()
	chk.Equal(float32(0), l)
	chk.Equal(float32(0), c)
	chk.Equal(float32(0), h)

	lchGray := unison.RGB(0x11, 0x11, 0x11)
	chk.Equal(lchGray, unison.OKLCH(0.17763777, 0, 0, 1))
	l, c, h = lchGray.OKLCH()
	chk.Equal(float32(0.17763777), l)
	chk.Equal(float32(0), c)
	chk.Equal(float32(0), h)

	chk.Equal(unison.Red, unison.OKLCH(0.6279554, 0.2576833, 29.233885, 1))
	l, c, h = unison.Red.OKLCH()
	chk.Equal(float32(0.6279554), l)
	chk.Equal(float32(0.2576833), c)
	chk.Equal(float32(29.233885), h)

	chk.Equal(unison.Green, unison.OKLCH(0.51975185, 0.17685826, 142.4953389, 1))
	l, c, h = unison.Green.OKLCH()
	chk.Equal(float32(0.51975185), l)
	chk.Equal(float32(0.17685826), c)
	chk.Equal(float32(142.4953389), h)

	chk.Equal(unison.Blue, unison.OKLCH(0.45201373, 0.31321436, 264.0520206, 1))
	l, c, h = unison.Blue.OKLCH()
	chk.Equal(float32(0.45201373), l)
	chk.Equal(float32(0.31321436), c)
	chk.Equal(float32(264.052), h)
}
