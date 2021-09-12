// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"embed"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison/internal/skia"
)

const (
	// DefaultSystemFamilyName is the default system font family name and will be used as a fallback where needed.
	DefaultSystemFamilyName = "Roboto"
	// FontAwesomeFreeFamilyName is the name of the FontAwesome Free font that has been loaded automatically.
	FontAwesomeFreeFamilyName = "Font Awesome 5 Free"
)

//go:embed resources/fonts
var fontFS embed.FS

var (
	internalFontLock sync.RWMutex
	internalFonts                 = make(map[string]*internalFont)
	_                FontProvider = &Font{}
)

// Pre-defined fonts
var (
	SystemFont                FontProvider
	EmphasizedSystemFont      FontProvider
	SmallSystemFont           FontProvider
	EmphasizedSmallSystemFont FontProvider
	LabelFont                 FontProvider
	FieldFont                 FontProvider
)

type internalFont struct {
	family string
	faces  []*FontFace
}

// FontProvider holds a font to draw with.
type FontProvider interface {
	ResolvedFont() *Font
}

// FontHinting holds the type of font hinting to use.
type FontHinting byte

// Possible values for FontHinting.
const (
	FontHintingNone FontHinting = iota
	FontHintingSlight
	FontHintingNormal
	FontHintingFull
)

// FontMetrics flags
const (
	UnderlineThicknessIsValidFontMetricsFlag = 1 << iota
	UnderlinePositionIsValidFontMetricsFlag
	StrikeoutThicknessIsValidFontMetricsFlag
	StrikeoutPositionIsValidFontMetricsFlag
	BoundsInvalidFontMetricsFlag
)

// FontMetrics holds various metrics about a font.
type FontMetrics struct {
	Flags              uint32  // Flags indicating which metrics are valid
	Top                float32 // Greatest extent above origin of any glyph bounding box; typically negative; deprecated with variable fonts; only if Flags & BoundsInvalidFontMetricsFlag == 0
	Ascent             float32 // Distance to reserve above baseline; typically negative
	Descent            float32 // Distance to reserve below baseline; typically positive
	Bottom             float32 // Greatest extent below origin of any glyph bounding box; typically positive; deprecated with variable fonts; only if Flags & BoundsInvalidFontMetricsFlag == 0
	Leading            float32 // Distance to add between lines; typically positive or zero
	AvgCharWidth       float32 // Average character width; zero if unknown
	MaxCharWidth       float32 // Maximum character width; zero if unknown
	XMin               float32 // Greatest extent to left of origin of any glyph bounding box; typically negative; deprecated with variable fonts; only if Flags & BoundsInvalidFontMetricsFlag == 0
	XMax               float32 // Greatest extent to right of origin of any glyph bounding box; typically positive; deprecated with variable fonts; only if Flags & BoundsInvalidFontMetricsFlag == 0
	XHeight            float32 // Height of lowercase 'x'; zero if unknown; typically negative
	CapHeight          float32 // Height of uppercase letter; zero if unknown; typically negative
	UnderlineThickness float32 // Underline thickness; only if Flags & UnderlineThicknessIsValidFontMetricsFlag != 0
	UnderlinePosition  float32 // Distance from baseline to top of stroke; typically positive; only if Flags & UnderlinePositionIsValidFontMetricsFlag != 0
	StrikeoutThickness float32 // Strikeout thickness; only if Flags & StrikeoutThicknessIsValidFontMetricsFlag != 0
	StrikeoutPosition  float32 // Distance from baseline to bottom of stroke; typically negative; only if Flags & StrikeoutPositionIsValidFontMetricsFlag != 0
}

// Font holds a realized FontFace of a specific size that can be used to render text.
type Font struct {
	size    float32
	face    *FontFace
	font    skia.Font
	metrics FontMetrics
}

// ResolvedFont implements the FontProvider interface.
func (f *Font) ResolvedFont() *Font {
	return f
}

// Face returns the FontFace this Font belongs to.
func (f *Font) Face() *FontFace {
	return f.face
}

// Size returns the size of the font. This is the value that was passed to FontFace.Font() when creating the font.
func (f *Font) Size() float32 {
	return f.size
}

// Metrics returns a copy of the FontMetrics for this font.
func (f *Font) Metrics() FontMetrics {
	return f.metrics
}

// Baseline returns the number of logical pixels to the bottom of characters without descenders.
func (f *Font) Baseline() float32 {
	return f.metrics.Descent + f.size
}

// LineHeight returns the recommended line height of the font.
func (f *Font) LineHeight() float32 {
	return f.size + f.metrics.Descent*2
}

// Width of the string rendered with this font. Note that this does not account for any embedded line endings nor tabs.
func (f *Font) Width(str string) float32 {
	if str == "" {
		return 0
	}
	return skia.FontMeasureText(f.font, str)
}

// Extents of the string rendered with this font. Note that this does not account for any embedded line endings nor tabs.
func (f *Font) Extents(str string) geom32.Size {
	return geom32.Size{Width: f.Width(str), Height: f.LineHeight()}
}

// Glyphs converts the text into a series of glyphs.
func (f *Font) Glyphs(text string) []uint16 {
	return skia.FontTextToGlyphs(f.font, text)
}

func (f *Font) runeStarts(str string) []float32 {
	// TODO: Revisit -- can we use the Text object instead?
	return skia.FontGetXPos(f.font, str)
}

// IndexForPosition returns the rune index within the string for the specified x-coordinate, where 0 is the start of the
// string. Note that this does not account for any embedded line endings nor tabs.
func (f *Font) IndexForPosition(x float32, str string) int {
	if x <= 0 || str == "" {
		return 0
	}
	pos := f.runeStarts(str)
	for i, p := range pos {
		if x < p {
			if p-x > x-pos[i-1] {
				return i - 1
			}
			return i
		}
	}
	return len(pos) - 1
}

// PositionForIndex returns the x-coordinate where the specified rune index starts. The returned coordinate assumes 0 is
// the start of the string. Note that this does not account for any embedded line endings nor tabs.
func (f *Font) PositionForIndex(index int, str string) float32 {
	if index <= 0 || str == "" {
		return 0
	}
	pos := f.runeStarts(str)
	if index > len(pos)-1 {
		index = len(pos) - 1
	}
	return pos[index]
}

// Descriptor returns a FontDescriptor for this Font.
func (f *Font) Descriptor() FontDescriptor {
	weight, spacing, slant := f.face.Style()
	return FontDescriptor{
		Family:  f.face.Family(),
		Size:    f.size,
		Weight:  weight,
		Spacing: spacing,
		Slant:   slant,
	}
}

func initSystemFonts() {
	const fontDir = "resources/fonts"
	entries, err := fontFS.ReadDir(fontDir)
	if err != nil {
		jot.Error(errs.NewWithCause("unable to read embedded file system", err))
		return
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			name := entry.Name()
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".otf") || strings.HasSuffix(lower, ".ttf") {
				var data []byte
				if data, err = fontFS.ReadFile(path.Join(fontDir, name)); err != nil {
					jot.Error(errs.NewWithCausef(err, "unable to read font %s", name))
				} else if _, err = RegisterFont(data); err != nil {
					jot.Error(errs.NewWithCause(name, err))
				}
			}
		}
	}

	SystemFont = newSystemFont(10, MediumFontWeight, StandardSpacing, NoSlant)
	EmphasizedSystemFont = newSystemFont(10, BoldFontWeight, StandardSpacing, NoSlant)
	SmallSystemFont = newSystemFont(8, MediumFontWeight, StandardSpacing, NoSlant)
	EmphasizedSmallSystemFont = newSystemFont(8, BoldFontWeight, StandardSpacing, NoSlant)
	LabelFont = newSystemFont(8, NormalFontWeight, StandardSpacing, NoSlant)
	FieldFont = newSystemFont(10, NormalFontWeight, StandardSpacing, NoSlant)
}

func newSystemFont(size float32, weight FontWeight, spacing FontSpacing, slant FontSlant) *IndirectFont {
	return &IndirectFont{Font: MatchFontFace(DefaultSystemFamilyName, weight, spacing, slant).Font(size)}
}

// RegisterFont registers a font with the font manager.
func RegisterFont(data []byte) (*FontDescriptor, error) {
	f := CreateFontFace(data)
	if f == nil {
		return nil, errs.New("unable to load font")
	}
	weight, spacing, slant := f.Style()
	fd := &FontDescriptor{
		Family:  f.Family(),
		Size:    12, // Arbitrary
		Weight:  weight,
		Spacing: spacing,
		Slant:   slant,
	}
	internalFontLock.Lock()
	defer internalFontLock.Unlock()
	if info, ok := internalFonts[fd.Family]; ok {
		add := true
		for _, one := range info.faces {
			weight2, spacing2, slant2 := one.Style()
			if weight == weight2 && spacing == spacing2 && slant == slant2 {
				add = false
				break
			}
		}
		if add {
			info.faces = append(info.faces, f)
			sort.Slice(info.faces, func(i, j int) bool {
				return txt.NaturalLess(info.faces[i].String(), info.faces[j].String(), true)
			})
		}
	} else {
		internalFonts[fd.Family] = &internalFont{
			family: fd.Family,
			faces:  []*FontFace{f},
		}
	}
	return fd, nil
}
