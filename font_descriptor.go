// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/fatal"
	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/unison/enums/slant"
	"github.com/richardwilkes/unison/enums/spacing"
	"github.com/richardwilkes/unison/enums/weight"
)

// FontDescriptor holds information necessary to construct a Font. The Size field is the value that was passed to
// FontFace.Font() when creating the font.
type FontDescriptor struct {
	FontFaceDescriptor
	Size float32 `json:"size"`
}

// Font returns the matching Font. If the specified font family cannot be found, the DefaultSystemFamilyName will be
// substituted.
func (fd FontDescriptor) Font() Font {
	f := fd.Face()
	if f == nil {
		if fd.Family == DefaultSystemFamilyName {
			fatal.IfErr(errs.New("default system font family is unavailable"))
		}
		other := fd
		other.Family = DefaultSystemFamilyName
		return other.Font()
	}
	return f.Font(fd.Size)
}

// String this returns a string suitable for display. It is not suitable for converting back into a FontDescriptor.
func (fd FontDescriptor) String() string {
	return fmt.Sprintf("%s %v%s", fd.Family, fd.Size, fd.variants())
}

// MarshalText implements the encoding.TextMarshaler interface.
func (fd FontDescriptor) MarshalText() (text []byte, err error) {
	return []byte(fmt.Sprintf("%s %v %s %s %s", fd.Family, fd.Size, fd.Weight.Key(), fd.Spacing.Key(), fd.Slant.Key())), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (fd *FontDescriptor) UnmarshalText(text []byte) error {
	parts := strings.Split(txt.CollapseSpaces(string(text)), " ")
	if len(parts) < 5 {
		return errs.Newf("invalid font descriptor: %s", string(text))
	}
	fd.Slant = slant.Extract(parts[len(parts)-1])
	fd.Spacing = spacing.Extract(parts[len(parts)-2])
	fd.Weight = weight.Extract(parts[len(parts)-3])
	size, err := strconv.ParseFloat(parts[len(parts)-4], 32)
	if err != nil || size <= 0 {
		return errs.Newf("invalid font descriptor: %s", string(text))
	}
	fd.Size = float32(size)
	fd.Family = strings.Join(parts[:len(parts)-4], " ")
	return nil
}
