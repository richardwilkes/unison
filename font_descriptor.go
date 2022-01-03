// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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
	"github.com/richardwilkes/toolbox/log/jot"
)

// FontDescriptor holds information necessary to construct a Font. The Size field is the value that was passed to
// FontFace.Font() when creating the font.
type FontDescriptor struct {
	Family  string      `json:"family"`
	Size    float32     `json:"size"`
	Weight  FontWeight  `json:"weight"`
	Spacing FontSpacing `json:"spacing"`
	Slant   FontSlant   `json:"slant"`
}

// Face returns the matching FontFace, if any.
func (fd FontDescriptor) Face() *FontFace {
	return MatchFontFace(fd.Family, fd.Weight, fd.Spacing, fd.Slant)
}

// Font returns the matching Font. If the specified font family cannot be found, the DefaultSystemFamilyName will be
// substituted.
func (fd FontDescriptor) Font() *Font {
	f := fd.Face()
	if f == nil {
		if fd.Family == DefaultSystemFamilyName {
			jot.Fatal(1, "default system font family is unavailable")
		}
		other := fd
		other.Family = DefaultSystemFamilyName
		return other.Font()
	}
	return f.Font(fd.Size)
}

// MarshalText implements the encoding.TextMarshaler interface.
func (fd FontDescriptor) MarshalText() (text []byte, err error) {
	return []byte(fd.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (fd *FontDescriptor) UnmarshalText(text []byte) error {
	v, err := FontDescriptorFromString(string(text))
	if err != nil {
		return err
	}
	*fd = v
	return nil
}

// FontDescriptorFromString extracts the FontDescriptor from a string.
func FontDescriptorFromString(str string) (FontDescriptor, error) {
	var fd FontDescriptor
	parts := strings.Split(str, " ")
	if len(parts) < 5 {
		return fd, errs.New("invalid format: " + str)
	}
	fd.Slant = SlantFromString(parts[len(parts)-1])
	fd.Spacing = SpacingFromString(parts[len(parts)-2])
	fd.Weight = WeightFromString(parts[len(parts)-3])
	size, err := strconv.ParseFloat(parts[len(parts)-4], 32)
	if err != nil {
		return fd, errs.NewWithCause("invalid format: "+str, err)
	}
	if size <= 0 {
		return fd, errs.New("invalid format: " + str)
	}
	fd.Size = float32(size)
	fd.Family = strings.Join(parts[:len(parts)-4], " ")
	return fd, nil
}

func (fd *FontDescriptor) String() string {
	return fmt.Sprintf("%s %v %s %s %s", fd.Family, fd.Size, fd.Weight, fd.Spacing, fd.Slant)
}
