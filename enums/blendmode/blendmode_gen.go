// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package blendmode

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	Clear Enum = iota
	Src
	Dst
	SrcOver
	DstOver
	SrcIn
	DstIn
	SrcOut
	DstOut
	SrcAtop
	DstAtop
	Xor
	Plus
	Modulate
	Screen
	Overlay
	Darken
	Lighten
	ColorDodge
	ColorBurn
	HardLight
	SoftLight
	Difference
	Exclusion
	Multiply
	Hue
	Saturation
	Color
	Luminosity
)

// All possible values.
var All = []Enum{
	Clear,
	Src,
	Dst,
	SrcOver,
	DstOver,
	SrcIn,
	DstIn,
	SrcOut,
	DstOut,
	SrcAtop,
	DstAtop,
	Xor,
	Plus,
	Modulate,
	Screen,
	Overlay,
	Darken,
	Lighten,
	ColorDodge,
	ColorBurn,
	HardLight,
	SoftLight,
	Difference,
	Exclusion,
	Multiply,
	Hue,
	Saturation,
	Color,
	Luminosity,
}

// Enum holds the mode used for blending pixels.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= Luminosity {
		return e
	}
	return Clear
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Clear:
		return "clear"
	case Src:
		return "src"
	case Dst:
		return "dst"
	case SrcOver:
		return "src-over"
	case DstOver:
		return "dst-over"
	case SrcIn:
		return "src-in"
	case DstIn:
		return "dst-in"
	case SrcOut:
		return "src-out"
	case DstOut:
		return "dst-out"
	case SrcAtop:
		return "src-atop"
	case DstAtop:
		return "dst-atop"
	case Xor:
		return "xor"
	case Plus:
		return "plus"
	case Modulate:
		return "modulate"
	case Screen:
		return "screen"
	case Overlay:
		return "overlay"
	case Darken:
		return "darken"
	case Lighten:
		return "lighten"
	case ColorDodge:
		return "color-dodge"
	case ColorBurn:
		return "color-burn"
	case HardLight:
		return "hard-light"
	case SoftLight:
		return "soft-light"
	case Difference:
		return "difference"
	case Exclusion:
		return "exclusion"
	case Multiply:
		return "multiply"
	case Hue:
		return "hue"
	case Saturation:
		return "saturation"
	case Color:
		return "color"
	case Luminosity:
		return "luminosity"
	default:
		return Clear.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Clear:
		return i18n.Text("Clear")
	case Src:
		return i18n.Text("Src")
	case Dst:
		return i18n.Text("Dst")
	case SrcOver:
		return i18n.Text("Src-Over")
	case DstOver:
		return i18n.Text("Dst-Over")
	case SrcIn:
		return i18n.Text("Src-In")
	case DstIn:
		return i18n.Text("Dst-In")
	case SrcOut:
		return i18n.Text("Src-Out")
	case DstOut:
		return i18n.Text("Dst-Out")
	case SrcAtop:
		return i18n.Text("Src-Atop")
	case DstAtop:
		return i18n.Text("Dst-Atop")
	case Xor:
		return i18n.Text("Xor")
	case Plus:
		return i18n.Text("Plus")
	case Modulate:
		return i18n.Text("Modulate")
	case Screen:
		return i18n.Text("Screen")
	case Overlay:
		return i18n.Text("Overlay")
	case Darken:
		return i18n.Text("Darken")
	case Lighten:
		return i18n.Text("Lighten")
	case ColorDodge:
		return i18n.Text("Color-Dodge")
	case ColorBurn:
		return i18n.Text("Color-Burn")
	case HardLight:
		return i18n.Text("Hard-Light")
	case SoftLight:
		return i18n.Text("Soft-Light")
	case Difference:
		return i18n.Text("Difference")
	case Exclusion:
		return i18n.Text("Exclusion")
	case Multiply:
		return i18n.Text("Multiply")
	case Hue:
		return i18n.Text("Hue")
	case Saturation:
		return i18n.Text("Saturation")
	case Color:
		return i18n.Text("Color")
	case Luminosity:
		return i18n.Text("Luminosity")
	default:
		return Clear.String()
	}
}

// MarshalText implements the encoding.TextMarshaler interface.
func (e Enum) MarshalText() (text []byte, err error) {
	return []byte(e.Key()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (e *Enum) UnmarshalText(text []byte) error {
	*e = Extract(string(text))
	return nil
}

// Extract the value from a string.
func Extract(str string) Enum {
	for _, e := range All {
		if strings.EqualFold(e.Key(), str) {
			return e
		}
	}
	return Clear
}
