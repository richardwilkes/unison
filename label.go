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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/pathop"
	"github.com/richardwilkes/unison/enums/side"
)

// DefaultLabelTheme holds the default LabelTheme values for Labels. Modifying this data will not alter existing Labels,
// but will alter any Labels created in the future.
var DefaultLabelTheme = LabelTheme{
	TextDecoration: TextDecoration{
		Font:            LabelFont,
		OnBackgroundInk: ThemeOnSurface,
	},
	Gap:    StdIconGap,
	HAlign: align.Start,
	VAlign: align.Middle,
	Side:   side.Left,
}

// LabelTheme holds theming data for a Label.
type LabelTheme struct {
	TextDecoration
	Gap    float32
	HAlign align.Enum
	VAlign align.Enum
	Side   side.Enum
}

// Label represents non-interactive text and/or a Drawable.
type Label struct {
	Drawable Drawable
	Text     *Text
	LabelTheme
	Panel
}

// NewLabel creates a new, empty label.
func NewLabel() *Label {
	l := &Label{LabelTheme: DefaultLabelTheme}
	l.Self = l
	l.SetSizer(l.DefaultSizes)
	l.DrawCallback = l.DefaultDraw
	return l
}

func (l *Label) String() string {
	if l.Text == nil {
		return ""
	}
	return l.Text.String()
}

// axLabel returns the label itself. It is what axLabelOf recognizes a label by, and it is unexported so that only a
// widget built by embedding a Label — which promotes this along with everything else the label offers — can be taken
// for one. A widget that merely has a title of its own cannot claim to be a label by accident.
func (l *Label) axLabel() *Label {
	return l
}

// SetTitle sets the text of the label to the specified text. The theme's TextDecoration will be used, so any
// changes you want to make to it should be done before calling this method. Alternatively, you can directly set the
// .Text field.
func (l *Label) SetTitle(text string) {
	l.Text = NewText(text, &l.TextDecoration)
}

// DefaultSizes provides the default sizing.
func (l *Label) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	prefSize, _ = LabelContentSizes(l.Text, l.Drawable, l.Font, l.Side, l.Gap)
	if b := l.Border(); b != nil {
		prefSize = prefSize.Add(b.Insets().Size())
	}
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, prefSize
}

// DefaultDraw provides the default drawing.
func (l *Label) DefaultDraw(canvas *Canvas, _ geom.Rect) {
	DrawLabel(canvas, l.ContentRect(false), l.HAlign, l.VAlign, l.Font, l.Text, l.OnBackgroundInk, l.BackgroundInk,
		l.Drawable, l.Side, l.Gap, !l.Enabled())
}

// ProvideAccessibility describes the label to assistive technologies. A label is static text, or an image when a
// drawable is all it holds, and one that nothing describes is skipped: see axDescribeStaticContent, which Tag and
// DrawablePanel share. An explicitly set role is left alone, which is how NewLink turns a label into a link and how
// Markdown turns one into a heading.
//
// Static text also carries the text itself, along with the one line it was drawn on, so that a screen reader can read
// it by line, by word and by character and put its review cursor where each character actually is. That is the text
// that was drawn, even when the application has given the label a name of its own: the lines and the styled runs index
// the drawn runes, and reporting them against words that were never on the screen would put every highlight and every
// review cursor in the wrong place. Both the runes and the line are asked for through Self, so a widget built by
// embedding a *Label and drawing words of its own publishes those words rather than the ones the label beneath it
// holds; see axTextLiner. A label showing only an image, and one nothing describes, have no text to carry.
// See role.Enum.IsText, which is what decides that a role reads as a body of text — a link does not, since it is
// announced by its name and followed.
func (l *Label) ProvideAccessibility(b *AccessibilityBuilder) {
	axDescribeStaticContent(b, l.String(), l.Drawable != nil)
	node := b.Node()
	if !node.Ignored && node.Role.IsText() {
		// Asked for through Self, since a widget built by embedding a *Label may draw other words, or draw them
		// somewhere other than where a plain label would, and shadow axTextLine to say so — a column header shrunk by
		// its sort indicator does. Calling the label's own method here would publish what the label would have drawn
		// rather than what is on the screen, which is where every highlight and review cursor would then be put. The
		// runes that come back are what the text is built from as well, since the line boundaries, the advances and
		// the styled runs all index them, and taking the text from the label instead would leave every offset an
		// adapter derives addressing a different string. See axTextLiner.
		liner, ok := l.Self.(axTextLiner)
		if !ok {
			liner = l
		}
		runes, decorations, line := liner.axTextLine()
		node.Text = axStaticTextInfo(string(runes), decorations, line)
	}
}

// axTextOrigin returns the top-left corner of the text this label draws, in the label's own coordinates. It is where
// the label actually put the text — the same answer DrawLabel placed it with, see labelPlacement — so that a document
// composing its content out of labels can say where each run of it sits on the screen.
func (l *Label) axTextOrigin() geom.Point {
	_, _, textPt, _ := labelPlacement(l.ContentRect(false), l.HAlign, l.VAlign, l.Font, l.Text, l.Drawable, l.Side,
		l.Gap)
	return textPt
}

// axTextLine returns what this label contributes to a document's text: its runes, the decoration each of them is drawn
// with, and the one line they occupy, with the line's bounds in the label's own coordinates and one advance per rune
// boundary. The decorations are what the styled runs of that text are worked out from, and they are the label's own
// slices, so neither they nor the runes may be modified.
//
// A label holding no text still occupies a line, since it is still as tall as one and a caret can sit in it: the line
// carries the single advance that says where that caret goes and covers no runes at all.
func (l *Label) axTextLine() (runes []rune, decorations []*TextDecoration, line accessibility.Line) {
	return axStaticTextLine(l.ContentRect(false), l.HAlign, l.VAlign, l.Font, l.Text, l.Drawable, l.Side, l.Gap)
}

// LabelContentSizes returns the preferred size of a label, as well as the preferred size of the text within the label.
// When no drawable is present, the two values will be the same. Provided as a standalone function so that other types
// of panels can make use of it.
func LabelContentSizes(text *Text, drawable Drawable, font Font, drawableSide side.Enum, gap float32) (size, txtSize geom.Size) {
	empty := text.Empty()
	if empty {
		// Use the text's own single-line height so that an empty line is exactly as tall as one containing text.
		// Only fall back to the passed-in font when there is no text object to consult. This is worked out whether or
		// not there is a drawable, since the text of a label showing only an image is still a line as tall as one — a
		// caret can sit in it, and an assistive technology is told where and how tall it is; see Label.axTextLine. What
		// the drawable decides is the size of the label, not the size of the text within it.
		if text != nil {
			txtSize.Height = text.Height()
		} else {
			txtSize.Height = font.LineHeight()
		}
		size = txtSize
	} else {
		txtSize = text.Extents()
		size = txtSize
	}
	if drawable != nil {
		logicalSize := drawable.LogicalSize()
		switch {
		case empty:
			size = logicalSize
		case drawableSide.Horizontal():
			size.Width += logicalSize.Width + gap
			size.Height = max(size.Height, logicalSize.Height)
		default:
			size.Height += logicalSize.Height + gap
			size.Width = max(size.Width, logicalSize.Width)
		}
	}
	return size.Ceil(), txtSize
}

// labelPlacement works out where a label's content goes within rect: the area the content actually occupies once the
// alignments have been applied, and the top-left corner of the drawable and of the text within it. txtSize is the size
// of the text alone, which is what the line an assistive technology is told about is as tall and as wide as.
//
// It is what DrawLabel places its content with, and it is called rather than repeated wherever else the same answer is
// needed — see Label.axTextOrigin — since an assistive technology told the text sits somewhere it was not drawn puts
// its highlight, and a person's reading caret, in the wrong place.
func labelPlacement(rect geom.Rect, hAlign, vAlign align.Enum, font Font, text *Text, drawable Drawable,
	drawableSide side.Enum, imgGap float32,
) (content geom.Rect, drawablePt, textPt geom.Point, txtSize geom.Size) {
	empty := text.Empty()

	// Determine overall size of content
	size, txtSize := LabelContentSizes(text, drawable, font, drawableSide, imgGap)

	// Adjust the working area for the content size
	switch hAlign {
	case align.Middle, align.Fill:
		rect.X = xmath.Floor(rect.X + (rect.Width-size.Width)/2)
	case align.End:
		rect.X += rect.Width - size.Width
	default: // align.Start
	}
	switch vAlign {
	case align.Middle, align.Fill:
		rect.Y = xmath.Floor(rect.Y + (rect.Height-size.Height)/2)
	case align.End:
		rect.Y += rect.Height - size.Height
	default: // align.Start
	}
	rect.Size = size

	// Determine drawable and text areas
	imgPt := rect.Point
	txtPt := rect.Point
	if !empty && drawable != nil {
		logicalSize := drawable.LogicalSize()
		switch drawableSide {
		case side.Top:
			txtPt.Y += logicalSize.Height + imgGap
			if logicalSize.Width > txtSize.Width {
				txtPt.X = xmath.Floor(txtPt.X + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgPt.X = xmath.Floor(imgPt.X + (txtSize.Width-logicalSize.Width)/2)
			}
		case side.Left:
			txtPt.X += logicalSize.Width + imgGap
			if logicalSize.Height > txtSize.Height {
				txtPt.Y = xmath.Floor(txtPt.Y + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgPt.Y = xmath.Floor(imgPt.Y + (txtSize.Height-logicalSize.Height)/2)
			}
		case side.Bottom:
			imgPt.Y += rect.Height - logicalSize.Height
			txtPt.Y = imgPt.Y - (imgGap + txtSize.Height)
			if logicalSize.Width > txtSize.Width {
				txtPt.X = xmath.Floor(txtPt.X + (logicalSize.Width-txtSize.Width)/2)
			} else {
				imgPt.X = xmath.Floor(imgPt.X + (txtSize.Width-logicalSize.Width)/2)
			}
		case side.Right:
			imgPt.X += rect.Width - logicalSize.Width
			txtPt.X = imgPt.X - (imgGap + txtSize.Width)
			if logicalSize.Height > txtSize.Height {
				txtPt.Y = xmath.Floor(txtPt.Y + (logicalSize.Height-txtSize.Height)/2)
			} else {
				imgPt.Y = xmath.Floor(imgPt.Y + (txtSize.Height-logicalSize.Height)/2)
			}
		}
	}
	return rect, imgPt, txtPt, txtSize
}

// DrawLabel draws a label. Provided as a standalone function so that other types of panels can make use of it.
func DrawLabel(canvas *Canvas, rect geom.Rect, hAlign, vAlign align.Enum, font Font, text *Text, onBackgroundInk, backgroundInk Ink, drawable Drawable, drawableSide side.Enum, imgGap float32, applyDisabledFilter bool) {
	if !xreflect.IsNil(backgroundInk) {
		paint := backgroundInk.Paint(canvas, rect, paintstyle.Fill)
		canvas.DrawRect(rect, paint)
	}
	empty := text.Empty()
	if drawable == nil && empty {
		return
	}
	rect, imgPt, txtPt, _ := labelPlacement(rect, hAlign, vAlign, font, text, drawable, drawableSide, imgGap)

	canvas.Save()
	canvas.ClipRect(rect, pathop.Intersect, false)
	if drawable != nil {
		rect.Point = imgPt
		rect.Size = drawable.LogicalSize()
		fg := onBackgroundInk
		if applyDisabledFilter {
			fg = &ColorFilteredInk{
				OriginalInk: fg,
				ColorFilter: Grayscale30Filter(),
			}
		}
		paint := fg.Paint(canvas, rect, paintstyle.Fill)
		drawable.DrawInRect(canvas, rect, nil, paint)
	}
	if !empty {
		if applyDisabledFilter {
			defer text.RestoreDecorations(text.AdjustDecorations(func(decoration *TextDecoration) {
				decoration.OnBackgroundInk = &ColorFilteredInk{
					OriginalInk: decoration.OnBackgroundInk,
					ColorFilter: Grayscale30Filter(),
				}
			}))
		}
		txtPt.Y += text.Baseline()
		text.Draw(canvas, txtPt)
	}
	canvas.Restore()
}
