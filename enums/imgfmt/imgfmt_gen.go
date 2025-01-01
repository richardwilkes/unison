// Code generated from "enum.go.tmpl" - DO NOT EDIT.

// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package imgfmt

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
)

// Possible values.
const (
	Unknown Enum = iota
	BMP
	GIF
	ICO
	JPEG
	PNG
	WBMP
	WEBP
)

// All possible values.
var All = []Enum{
	Unknown,
	BMP,
	GIF,
	ICO,
	JPEG,
	PNG,
	WBMP,
	WEBP,
}

// Enum holds the type of encoding an image was stored with.
type Enum byte

// EnsureValid ensures this is of a known value.
func (e Enum) EnsureValid() Enum {
	if e <= WEBP {
		return e
	}
	return Unknown
}

// Key returns the key used in serialization.
func (e Enum) Key() string {
	switch e {
	case Unknown:
		return "unknown"
	case BMP:
		return "bmp"
	case GIF:
		return "gif"
	case ICO:
		return "ico"
	case JPEG:
		return "jpeg"
	case PNG:
		return "png"
	case WBMP:
		return "wbmp"
	case WEBP:
		return "webp"
	default:
		return Unknown.Key()
	}
}

// String implements fmt.Stringer.
func (e Enum) String() string {
	switch e {
	case Unknown:
		return i18n.Text("Unknown")
	case BMP:
		return "BMP"
	case GIF:
		return "GIF"
	case ICO:
		return "ICO"
	case JPEG:
		return "JPEG"
	case PNG:
		return "PNG"
	case WBMP:
		return "WBMP"
	case WEBP:
		return "WEBP"
	default:
		return Unknown.String()
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
	return Unknown
}
