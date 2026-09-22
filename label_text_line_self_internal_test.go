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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
)

// axShadowedLabel is a widget built the documented way — by embedding a *Label and pointing Self at itself — which
// draws other words than the label beneath it holds, and draws them somewhere other than where a plain label would,
// and shadows axTextLine to say so, much as DefaultTableColumnHeader does for a column shrunk by its sort indicator.
// What it reports is its own runes and a fixed rectangle, so that a description taken from the label's own answer
// instead is unmistakable.
type axShadowedLabel struct {
	*Label
}

// axShadowedLabelBounds is where axShadowedLabel claims its text was drawn.
var axShadowedLabelBounds = geom.NewRect(7, 11, 23, 29)

// axShadowedLabelDrawn is what axShadowedLabel claims it actually drew, which is not what the label beneath it holds.
const axShadowedLabelDrawn = "Shad\u2026"

func (s *axShadowedLabel) axTextLine() (runes []rune, decorations []*TextDecoration, line accessibility.Line) {
	runes, decorations, line = axStaticTextLine(s.ContentRect(false), s.HAlign, s.VAlign, s.Font,
		NewText(axShadowedLabelDrawn, &TextDecoration{Font: s.Font}), nil, s.Side, s.Gap)
	line.Bounds = axShadowedLabelBounds
	return runes, decorations, line
}

// TestLabelPublishesShadowedTextLine verifies that both the runes and the line a label publishes for a screen reader to
// read by are asked for through Self, so that a widget embedding a *Label and answering for its own text has that
// answer published rather than the one the label would have given — text and line boundaries alike, since an adapter
// derives every offset it uses from the two together. See Label.ProvideAccessibility and axTextLiner. A session owns
// most of the package's mutable globals while it runs, so this may not call t.Parallel.
func TestLabelPublishesShadowedTextLine(t *testing.T) {
	c := check.New(t)
	var shadowed, plain *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			shadowed = NewLabel()
			shadowed.SetTitle("Shadowed")
			shadowed.Self = &axShadowedLabel{Label: shadowed}
			plain = NewLabel()
			plain.SetTitle("Plain")
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(shadowed)
			column.AddChild(plain)
			wnd = axNewTestWindow(t, "shadowed text line", geom.NewRect(10, 10, 500, 300), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))

	node := screen.AccessibilityNodeFor(shadowed)
	c.NotNil(node)
	if node != nil {
		c.NotNil(node.Text, "a label showing text reports the line it was drawn on")
		if node.Text != nil {
			c.Equal(1, len(node.Text.Lines), "static text occupies exactly one line")
			c.Equal(axShadowedLabelDrawn, node.Text.Text,
				"the text is the runes the widget says it drew, not the ones the label beneath it holds")
			if len(node.Text.Lines) == 1 {
				c.Equal(axShadowedLabelBounds, node.Text.Lines[0].Bounds,
					"the line comes from the widget's own answer, not from the label beneath it")
				c.Equal(len([]rune(axShadowedLabelDrawn)), node.Text.Lines[0].End,
					"and the line covers those same runes, so every offset an adapter derives addresses them")
			}
		}
	}

	plainNode := screen.AccessibilityNodeFor(plain)
	c.NotNil(plainNode)
	if plainNode != nil {
		c.NotNil(plainNode.Text, "a plain label still carries the text it drew")
		if plainNode.Text != nil {
			c.Equal("Plain", plainNode.Text.Text, "a label that answers for itself reports its own words")
			c.Equal(1, len(plainNode.Text.Lines))
			if len(plainNode.Text.Lines) == 1 {
				c.True(plainNode.Text.Lines[0].Bounds != axShadowedLabelBounds,
					"a label that answers for itself is unaffected")
			}
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
