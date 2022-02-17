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
	"embed"
	"path"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison/internal/skia"
)

// DefaultSystemFamilyName is the default system font family name and will be used as a fallback where needed.
const DefaultSystemFamilyName = "Roboto"

//go:embed resources/fonts
var fontFS embed.FS

var (
	internalFontLock sync.RWMutex
	internalFonts         = make(map[string]*internalFont)
	_                Font = &fontImpl{}
)

// Pre-defined fonts
var (
	SystemFont                = &IndirectFont{}
	EmphasizedSystemFont      = &IndirectFont{}
	SmallSystemFont           = &IndirectFont{}
	EmphasizedSmallSystemFont = &IndirectFont{}
	LabelFont                 = &IndirectFont{}
	FieldFont                 = &IndirectFont{}
	KeyboardFont              = &IndirectFont{}
)

// Font holds a realized FontFace of a specific size that can be used to render text.
type Font interface {
	// Face returns the FontFace this Font belongs to.
	Face() *FontFace
	// Size returns the size of the font. This is the value that was passed to FontFace.Font() when creating the font.
	Size() float32
	// Metrics returns a copy of the FontMetrics for this font.
	Metrics() FontMetrics
	// Baseline returns the number of logical pixels to the bottom of characters without descenders.
	Baseline() float32
	// LineHeight returns the recommended line height of the font.
	LineHeight() float32
	// Width of the string rendered with this font. Note that this does not account for any embedded line endings nor
	// tabs.
	Width(str string) float32
	// Extents of the string rendered with this font. Note that this does not account for any embedded line endings nor
	// tabs.
	Extents(str string) geom32.Size
	// Glyphs converts the text into a series of glyphs.
	Glyphs(text string) []uint16
	// IndexForPosition returns the rune index within the string for the specified x-coordinate, where 0 is the start of
	// the string. Note that this does not account for any embedded line endings nor tabs.
	IndexForPosition(x float32, str string) int
	// PositionForIndex returns the x-coordinate where the specified rune index starts. The returned coordinate assumes
	// 0 is the start of the string. Note that this does not account for any embedded line endings nor tabs.
	PositionForIndex(index int, str string) float32
	// WrapText breaks the given text into multiple lines that are <= width. Embedded line feeds are respected. Trailing
	// whitespace is not considered for purposes of fitting within the given width.
	WrapText(text string, width float32) []string
	// Descriptor returns a FontDescriptor for this Font.
	Descriptor() FontDescriptor
	skiaFont() skia.Font
}

type internalFont struct {
	family string
	faces  []*FontFace
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

type fontImpl struct {
	size    float32
	face    *FontFace
	font    skia.Font
	metrics FontMetrics
}

func (f *fontImpl) Face() *FontFace {
	return f.face
}

func (f *fontImpl) Size() float32 {
	return f.size
}

func (f *fontImpl) Metrics() FontMetrics {
	return f.metrics
}

func (f *fontImpl) Baseline() float32 {
	return f.metrics.Descent + f.size
}

func (f *fontImpl) LineHeight() float32 {
	return f.size + f.metrics.Descent*2
}

func (f *fontImpl) Width(str string) float32 {
	if str == "" {
		return 0
	}
	return skia.FontMeasureText(f.font, str)
}

func (f *fontImpl) Extents(str string) geom32.Size {
	return geom32.Size{Width: f.Width(str), Height: f.LineHeight()}
}

func (f *fontImpl) Glyphs(text string) []uint16 {
	return skia.FontTextToGlyphs(f.font, text)
}

func (f *fontImpl) runeStarts(str string) []float32 {
	// TODO: Revisit -- can we use the Text object instead?
	return skia.FontGetXPos(f.font, str)
}

func (f *fontImpl) skiaFont() skia.Font {
	return f.font
}

func (f *fontImpl) IndexForPosition(x float32, str string) int {
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

func (f *fontImpl) PositionForIndex(index int, str string) float32 {
	if index <= 0 || str == "" {
		return 0
	}
	pos := f.runeStarts(str)
	if index > len(pos)-1 {
		index = len(pos) - 1
	}
	return pos[index]
}

func (f *fontImpl) WrapText(text string, width float32) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		positions := f.runeStarts(line) // returns 1 more than there are runes
		runes := []rune(line)
		start := 0
		for start < len(runes) {
			i := start
			for i < len(runes) && positions[i+1]-positions[start] < width {
				i++
			}
			if i == len(runes) {
				lines = append(lines, string(runes[start:]))
				break
			}
			// Forward past any additional whitespace
			for i < len(runes) && isWhitespace(runes[i]) {
				i++
			}
			// Backup to first break
			for i > start && !isWordBreak(runes[i-1]) {
				i--
			}
			if i == start {
				// Nothing found that fits, so take the first word and any trailing whitespace after it
				for i < len(runes) && !isWordBreak(runes[i]) {
					i++
				}
				if i < len(runes) && isWordBreak(runes[i]) {
					i++
				}
				for i < len(runes) && isWhitespace(runes[i]) {
					i++
				}
			}
			lines = append(lines, string(runes[start:i]))
			start = i
		}
	}
	return lines
}

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t'
}

func isWordBreak(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '/' || ch == '\\'
}

func (f *fontImpl) Descriptor() FontDescriptor {
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

	SystemFont.Font = MatchFontFace(DefaultSystemFamilyName, MediumFontWeight, StandardSpacing, NoSlant).Font(10)
	EmphasizedSystemFont.Font = MatchFontFace(DefaultSystemFamilyName, BoldFontWeight, StandardSpacing, NoSlant).Font(10)
	SmallSystemFont.Font = MatchFontFace(DefaultSystemFamilyName, MediumFontWeight, StandardSpacing, NoSlant).Font(8)
	EmphasizedSmallSystemFont.Font = MatchFontFace(DefaultSystemFamilyName, BoldFontWeight, StandardSpacing, NoSlant).Font(8)
	LabelFont.Font = MatchFontFace(DefaultSystemFamilyName, NormalFontWeight, StandardSpacing, NoSlant).Font(8)
	FieldFont.Font = MatchFontFace(DefaultSystemFamilyName, NormalFontWeight, StandardSpacing, NoSlant).Font(10)
	keyboardFamilyName := DefaultSystemFamilyName
	if runtime.GOOS == toolbox.MacOS {
		// This is a special font on macOS. Ideally, I'd find a source for an equivalent font and embed it so that the
		// same font could be used on all platforms.
		keyboardFamilyName = ".Keyboard"
	}
	KeyboardFont.Font = MatchFontFace(keyboardFamilyName, MediumFontWeight, StandardSpacing, NoSlant).Font(10)
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
