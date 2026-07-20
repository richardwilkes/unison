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
	"embed"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/richardwilkes/canvas/font"
	"github.com/richardwilkes/canvas/textblob"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/enums/slant"
	"github.com/richardwilkes/unison/enums/spacing"
	"github.com/richardwilkes/unison/enums/weight"
)

const (
	// DefaultSystemFamilyName is the default system font family name and will be used as a fallback where needed.
	DefaultSystemFamilyName = "Roboto"
	// DefaultMonospacedFamilyName is the default monospaced font family name.
	DefaultMonospacedFamilyName = "DejaVu Sans Mono"
)

//go:embed resources/fonts
var fontFS embed.FS

var (
	internalFontLock sync.RWMutex
	internalFonts         = make(map[string]*internalFont)
	_                Font = &fontImpl{}
)

// Pre-defined fonts
var (
	SystemFont           = &IndirectFont{}
	EmphasizedSystemFont = &IndirectFont{}
	LabelFont            = &IndirectFont{}
	FieldFont            = &IndirectFont{}
	KeyboardFont         = &IndirectFont{}
	MonospacedFont       = &IndirectFont{}
)

// Font holds a realized FontFace of a specific size that can be used to render text.
type Font interface {
	// Face returns the FontFace this Font belongs to.
	Face() *FontFace
	// Size returns the size of the font. This is the value that was passed to FontFace.Font() when creating the font.
	Size() float32
	// Metrics returns a copy of the FontMetrics for this font.
	Metrics() font.Metrics
	// Baseline returns the number of logical pixels to the bottom of characters without descenders.
	Baseline() float32
	// LineHeight returns the recommended line height of the font.
	LineHeight() float32
	// RuneToGlyph converts a rune into a glyph. Missing glyphs will have a value of 0.
	RuneToGlyph(r rune) uint16
	// RunesToGlyphs converts the runes into glyphs. Missing glyphs will have a value of 0.
	RunesToGlyphs(r []rune) []uint16
	// GlyphWidth returns the width for the glyph. This does not do font fallback for missing glyphs.
	GlyphWidth(glyph uint16) float32
	// GlyphWidths returns the widths for each glyph. This does not do font fallback for missing glyphs.
	GlyphWidths(glyphs []uint16) []float32
	// SimpleWidth returns the width of a string. It does not do font fallback, nor does it consider tabs or line
	// endings.
	SimpleWidth(str string) float32
	// TextBlobPosH creates a text blob for glyphs, with specified horizontal positions. The glyphs and positions slices
	// should have the same length.
	TextBlobPosH(glyphs []uint16, positions []float32, y float32) *textblob.Blob
	// Descriptor returns a FontDescriptor for this Font.
	Descriptor() FontDescriptor
	canvasFont() *font.Font
}

type internalFont struct {
	family string
	faces  []*FontFace
}

type fontImpl struct {
	face    *FontFace
	font    *font.Font
	metrics font.Metrics
	size    float32
}

func (f *fontImpl) Face() *FontFace {
	return f.face
}

func (f *fontImpl) Size() float32 {
	return f.size
}

func (f *fontImpl) Metrics() font.Metrics {
	return f.metrics
}

func (f *fontImpl) Baseline() float32 {
	return f.metrics.Descent + f.size
}

func (f *fontImpl) LineHeight() float32 {
	return f.size + f.metrics.Descent*2
}

func (f *fontImpl) RuneToGlyph(r rune) uint16 {
	return f.font.UnicharToGlyph(r)
}

func (f *fontImpl) RunesToGlyphs(r []rune) []uint16 {
	if len(r) == 0 {
		return nil
	}
	unichars := make([]int32, len(r))
	copy(unichars, r)
	glyphs := make([]uint16, len(r))
	f.font.UnicharsToGlyphs(unichars, glyphs)
	return glyphs
}

func (f *fontImpl) GlyphWidth(glyph uint16) float32 {
	widths := make([]float32, 1)
	f.font.GlyphWidths([]uint16{glyph}, widths)
	return widths[0]
}

func (f *fontImpl) GlyphWidths(glyphs []uint16) []float32 {
	if len(glyphs) == 0 {
		return nil
	}
	widths := make([]float32, len(glyphs))
	f.font.GlyphWidths(glyphs, widths)
	return widths
}

func (f *fontImpl) SimpleWidth(str string) float32 {
	if str == "" {
		return 0
	}
	return f.font.MeasureText([]byte(str), font.TextEncodingUTF8, nil, nil)
}

func (f *fontImpl) TextBlobPosH(glyphs []uint16, positions []float32, y float32) *textblob.Blob {
	builder := textblob.NewBuilder()
	buffer := builder.AllocRunPosH(f.font, len(glyphs), y, nil)
	copy(buffer.Glyphs, glyphs)
	copy(buffer.Pos, positions)
	return builder.Make()
}

func (f *fontImpl) canvasFont() *font.Font {
	return f.font
}

func (f *fontImpl) Descriptor() FontDescriptor {
	w, sp, sl := f.face.Style()
	return FontDescriptor{
		FontFaceDescriptor: FontFaceDescriptor{
			Family:  f.face.Family(),
			Weight:  w,
			Spacing: sp,
			Slant:   sl,
		},
		Size: f.size,
	}
}

func init() {
	const fontDir = "resources/fonts"
	entries, err := fontFS.ReadDir(fontDir)
	if err != nil {
		errs.Log(errs.NewWithCause("unable to read embedded file system", err), "path", fontDir)
		return
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			name := entry.Name()
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".otf") || strings.HasSuffix(lower, ".ttf") {
				var data []byte
				if data, err = fontFS.ReadFile(path.Join(fontDir, name)); err != nil {
					errs.Log(errs.NewWithCause("unable to read font", err), "name", name)
				} else if _, err = RegisterFont(data); err != nil {
					errs.Log(errs.NewWithCause("unable to register font", err), "name", name)
				}
			}
		}
	}

	baseSize := float32(10)
	SystemFont.Font = MatchFontFace(DefaultSystemFamilyName, weight.Medium, spacing.Standard, slant.Upright).Font(baseSize)
	EmphasizedSystemFont.Font = MatchFontFace(DefaultSystemFamilyName, weight.Bold, spacing.Standard, slant.Upright).Font(baseSize)
	LabelFont.Font = MatchFontFace(DefaultSystemFamilyName, weight.Regular, spacing.Standard, slant.Upright).Font(baseSize)
	FieldFont.Font = MatchFontFace(DefaultSystemFamilyName, weight.Regular, spacing.Standard, slant.Upright).Font(baseSize)
	KeyboardFont.Font = MatchFontFace(DefaultSystemFamilyName, weight.Medium, spacing.Standard, slant.Upright).Font(baseSize)
	MonospacedFont.Font = MatchFontFace(DefaultMonospacedFamilyName, weight.Regular, spacing.Standard, slant.Upright).Font(baseSize)
}

// RegisterFont registers a font with the font manager.
func RegisterFont(data []byte) (FontFaceDescriptor, error) {
	var ffd FontFaceDescriptor
	f := CreateFontFace(data)
	if f == nil {
		return ffd, errs.New("unable to load font")
	}
	w, sp, sl := f.Style()
	ffd.Family = f.Family()
	ffd.Weight = w
	ffd.Spacing = sp
	ffd.Slant = sl
	internalFontLock.Lock()
	if info, ok := internalFonts[ffd.Family]; ok {
		add := true
		for _, one := range info.faces {
			w2, sp2, sl2 := one.Style()
			if w == w2 && sp == sp2 && sl == sl2 {
				add = false
				break
			}
		}
		if add {
			info.faces = append(info.faces, f)
			sort.Slice(info.faces, func(i, j int) bool {
				return info.faces[i].Less(info.faces[j])
			})
		}
	} else {
		internalFonts[ffd.Family] = &internalFont{
			family: ffd.Family,
			faces:  []*FontFace{f},
		}
	}
	internalFontLock.Unlock()
	// Drop any cached family list so the newly registered font shows up in subsequent FontFamilies() calls. This must
	// happen after internalFontLock has been released, since FontFamiliesNoCache acquires the two locks in the opposite
	// order.
	cachedFontFamiliesLock.Lock()
	cachedFontFamilies = nil
	cachedFontFamiliesLock.Unlock()
	return ffd, nil
}
