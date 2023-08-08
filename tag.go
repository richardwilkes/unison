// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// DefaultTagTheme holds the default TagTheme values for Tags. Modifying this data will not alter existing Tags, but
// will alter any Tags created in the future.
var DefaultTagTheme = TagTheme{
	Font: &DynamicFont{
		Resolver: func() FontDescriptor {
			desc := LabelFont.Descriptor()
			desc.Size = max(desc.Size-2, 1)
			return desc
		},
	},
	BackgroundInk:   OnBackgroundColor,
	OnBackgroundInk: BackgroundColor,
	Gap:             3,
	SideInset:       3,
	RadiusX:         6,
	RadiusY:         6,
	HAlign:          StartAlignment,
	VAlign:          MiddleAlignment,
	Side:            LeftSide,
}

// TagTheme holds theming data for a Tag.
type TagTheme struct {
	Font            Font
	BackgroundInk   Ink
	OnBackgroundInk Ink
	Gap             float32
	SideInset       float32
	RadiusX         float32
	RadiusY         float32
	HAlign          Alignment
	VAlign          Alignment
	Side            Side
	Underline       bool
	StrikeThrough   bool
}

// Tag represents non-interactive text and/or a Drawable with a bubble around it.
type Tag struct {
	Panel
	TagTheme
	Drawable  Drawable
	Text      string
	textCache TextCache
}

// NewTag creates a new, empty Tag.
func NewTag() *Tag {
	t := &Tag{TagTheme: DefaultTagTheme}
	t.Self = t
	t.SetSizer(t.DefaultSizes)
	t.DrawCallback = t.DefaultDraw
	return t
}

// DefaultSizes provides the default sizing.
func (t *Tag) DefaultSizes(hint Size) (minSize, prefSize, maxSize Size) {
	text := t.textCache.Text(t.Text, t.Font)
	if text == nil && t.Drawable == nil {
		prefSize.Height = t.Font.LineHeight()
		prefSize.GrowToInteger()
	} else {
		prefSize = LabelSize(text, t.Drawable, t.Side, t.Gap)
	}
	if b := t.Border(); b != nil {
		prefSize.AddInsets(b.Insets())
	}
	prefSize.GrowToInteger()
	prefSize.Width += t.SideInset * 2
	prefSize.ConstrainForHint(hint)
	return prefSize, prefSize, prefSize
}

// DefaultDraw provides the default drawing.
func (t *Tag) DefaultDraw(canvas *Canvas, _ Rect) {
	txt := t.textCache.Text(t.Text, t.Font)
	if t.Underline || t.StrikeThrough {
		txt.AdjustDecorations(func(decoration *TextDecoration) {
			decoration.Underline = t.Underline
			decoration.StrikeThrough = t.StrikeThrough
		})
	}
	r := t.ContentRect(false)
	canvas.DrawRoundedRect(r, t.RadiusX, t.RadiusY, t.BackgroundInk.Paint(canvas, r, Fill))
	r.X += t.SideInset
	r.Width -= t.SideInset * 2
	DrawLabel(canvas, r, t.HAlign, t.VAlign, txt, t.OnBackgroundInk, t.Drawable, t.Side, t.Gap, !t.Enabled())
}
