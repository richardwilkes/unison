// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/side"
)

// DefaultTagTheme holds the default TagTheme values for Tags. Modifying this data will not alter existing Tags, but
// will alter any Tags created in the future.
var DefaultTagTheme = TagTheme{
	TextDecoration: TextDecoration{
		Font: &DynamicFont{
			Resolver: func() FontDescriptor {
				desc := LabelFont.Descriptor()
				desc.Size = max(desc.Size-2, 1)
				return desc
			},
		},
		BackgroundInk:   ThemeOnSurface,
		OnBackgroundInk: ThemeSurface,
	},
	Gap:       3,
	SideInset: 3,
	RadiusX:   6,
	RadiusY:   6,
	HAlign:    align.Start,
	VAlign:    align.Middle,
	Side:      side.Left,
}

// TagTheme holds theming data for a Tag.
type TagTheme struct {
	TextDecoration
	Gap       float32
	SideInset float32
	RadiusX   float32
	RadiusY   float32
	HAlign    align.Enum
	VAlign    align.Enum
	Side      side.Enum
}

// Tag represents non-interactive text and/or a Drawable with a bubble around it.
type Tag struct {
	Drawable Drawable
	Text     *Text
	TagTheme
	Panel
}

// NewTag creates a new, empty Tag.
func NewTag() *Tag {
	t := &Tag{TagTheme: DefaultTagTheme}
	t.Self = t
	t.SetSizer(t.DefaultSizes)
	t.DrawCallback = t.DefaultDraw
	return t
}

// SetTitle sets the text of the tag to the specified text. The theme's TextDecoration will be used, so any
// changes you want to make to it should be done before calling this method. Alternatively, you can directly set the
// .Text field.
func (t *Tag) SetTitle(text string) {
	t.Text = NewText(text, &t.TextDecoration)
}

// DefaultSizes provides the default sizing.
func (t *Tag) DefaultSizes(hint Size) (minSize, prefSize, maxSize Size) {
	prefSize, _ = LabelContentSizes(t.Text, t.Drawable, t.Font, t.Side, t.Gap)
	if b := t.Border(); b != nil {
		prefSize = prefSize.Add(b.Insets().Size())
	}
	prefSize = prefSize.Ceil()
	prefSize.Width += t.SideInset * 2
	prefSize = prefSize.ConstrainForHint(hint)
	return prefSize, prefSize, prefSize
}

// DefaultDraw provides the default drawing.
func (t *Tag) DefaultDraw(canvas *Canvas, _ Rect) {
	r := t.ContentRect(false)
	canvas.DrawRoundedRect(r, t.RadiusX, t.RadiusY, t.BackgroundInk.Paint(canvas, r, paintstyle.Fill))
	r.X += t.SideInset
	r.Width -= t.SideInset * 2
	DrawLabel(canvas, r, t.HAlign, t.VAlign, t.Font, t.Text, t.OnBackgroundInk, nil, t.Drawable, t.Side, t.Gap,
		!t.Enabled())
}
