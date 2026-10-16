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
	"github.com/richardwilkes/unison/enums/mod"
)

// TestFitTooltipRect verifies that a tooltip that fits below the panel it belongs to stays there, that one that does
// not is shifted onto the usable area sideways, narrowed to it, or flipped above the panel, and that one with no room
// above either is pinned to the top of the usable area.
func TestFitTooltipRect(t *testing.T) {
	c := check.New(t)
	usable := geom.NewRect(0, 0, 400, 300)
	avoid := geom.NewRect(100, 100, 50, 20)
	c.Equal(geom.NewRect(100, 121, 60, 20), fitTooltipRect(geom.NewRect(100, 121, 60, 20), avoid, usable, 1),
		"one that fits is placed as asked")
	c.Equal(geom.NewRect(340, 121, 60, 20), fitTooltipRect(geom.NewRect(380, 121, 60, 20), avoid, usable, 1),
		"one past the right edge is shifted left")
	c.Equal(geom.NewRect(50, 121, 60, 20),
		fitTooltipRect(geom.NewRect(20, 121, 60, 20), avoid, geom.NewRect(50, 0, 350, 300), 1),
		"one past the left edge is shifted right")
	c.Equal(geom.NewRect(0, 121, 400, 20), fitTooltipRect(geom.NewRect(100, 121, 500, 20), avoid, usable, 1),
		"one wider than the usable area is narrowed to it")
	avoid = geom.NewRect(100, 270, 50, 20)
	c.Equal(geom.NewRect(100, 249, 60, 20), fitTooltipRect(geom.NewRect(100, 291, 60, 20), avoid, usable, 1),
		"one past the bottom edge is placed above the panel instead")
	c.Equal(geom.NewRect(100, 246, 60, 20), fitTooltipRect(geom.NewRect(100, 292, 60, 20), avoid, usable, 4),
		"with the gap asked for")
	avoid = geom.NewRect(100, 10, 50, 80)
	c.Equal(geom.NewRect(100, 0, 60, 40),
		fitTooltipRect(geom.NewRect(100, 91, 60, 40), avoid, geom.NewRect(0, 0, 400, 100), 1),
		"one with room neither below nor above the panel is pinned to the top")
}

// TestTooltipDisplayAmongGoesByThePanel verifies that a tooltip belongs on the display it overlaps most, and that one
// asked for just below a panel at the bottom of a display's usable area, where it overlaps none, belongs on the panel's
// display rather than being left to the owner window's, which may be a screen away from the panel.
func TestTooltipDisplayAmongGoesByThePanel(t *testing.T) {
	c := check.New(t)
	left := &Display{Usable: geom.NewRect(0, 0, 1920, 1040), headless: true}
	right := &Display{Usable: geom.NewRect(1920, 0, 1920, 1040), headless: true}
	displays := []*Display{left, right}
	c.True(tooltipDisplayAmong(displays, geom.NewRect(1900, 500, 100, 20), geom.NewRect(1900, 480, 10, 20)) == right,
		"one overlapping a display goes by that, whatever the panel's display")
	c.True(tooltipDisplayAmong(displays, geom.NewRect(1550, 1041, 100, 20), geom.NewRect(1550, 1020, 40, 20)) == left,
		"one overlapping none goes by the panel's display")
	c.Nil(tooltipDisplayAmong(displays, geom.NewRect(1550, 1041, 100, 20), geom.NewRect(1550, 1041, 40, 20)),
		"and is left to the caller when the panel overlaps none either")
}

// TestTooltipWindowRectWithoutADisplay verifies that a tooltip is fitted onto its display when it has one and left
// where it was asked to go when it has none, as on macOS while every display is asleep, rather than crashing on the
// display.
func TestTooltipWindowRectWithoutADisplay(t *testing.T) {
	c := check.New(t)
	tip := NewPanel()
	where := geom.NewRect(500, 271, 100, 50)
	avoid := geom.NewRect(500, 250, 40, 20)
	c.Equal(where, tooltipWindowRectOn(tip, where, avoid, nil))
	d := &Display{Usable: geom.NewRect(0, 0, 400, 300), headless: true}
	c.Equal(geom.NewRect(300, 199, 100, 50), tooltipWindowRectOn(tip, where, avoid, d))
}

// TestTooltipWindowRectShrinksWideTip verifies that a tooltip wider than its display is measured again with the
// display's width as a hint, so that one that can wrap does so, and that one that cannot is cut to the display's width.
func TestTooltipWindowRectShrinksWideTip(t *testing.T) {
	c := check.New(t)
	d := &Display{Usable: geom.NewRect(0, 0, 200, 300), headless: true}
	avoid := geom.NewRect(50, 100, 40, 20)
	var hints []geom.Size
	wraps := NewPanel()
	wraps.SetSizer(func(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
		hints = append(hints, hint)
		prefSize = geom.NewSize(300, 20)
		if hint.Width > 0 && hint.Width < prefSize.Width {
			prefSize = geom.NewSize(hint.Width, 60)
		}
		return prefSize, prefSize, prefSize
	})
	c.Equal(geom.NewRect(0, 121, 200, 60), tooltipWindowRectOn(wraps, geom.NewRect(50, 121, 300, 20), avoid, d))
	c.Equal([]geom.Size{geom.NewSize(200, 0)}, hints, "measured again with the display's width as the hint")
	rigid := NewPanel()
	rigid.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
		prefSize = geom.NewSize(300, 20)
		return prefSize, prefSize, prefSize
	})
	c.Equal(geom.NewRect(0, 121, 200, 20), tooltipWindowRectOn(rigid, geom.NewRect(50, 121, 300, 20), avoid, d))
	hints = nil
	c.Equal(geom.NewRect(50, 121, 150, 20), tooltipWindowRectOn(wraps, geom.NewRect(50, 121, 150, 20), avoid, d))
	c.Equal(0, len(hints), "one that fits the display's width is not measured again")
}

// TestTooltipWindowRectIsNeverEmpty verifies that a tooltip with nothing to show, such as a bare panel, still gets a
// window of at least one unit each way, with a display to fit it onto and without one, since a window cannot be empty.
func TestTooltipWindowRectIsNeverEmpty(t *testing.T) {
	c := check.New(t)
	usable := geom.NewRect(0, 0, 400, 300)
	avoid := geom.NewRect(100, 100, 50, 20)
	c.Equal(geom.NewRect(100, 121, 1, 1), fitTooltipRect(geom.NewRect(100, 121, 0, 0), avoid, usable, 1))
	c.Equal(geom.NewRect(100, 121, 1, 20), fitTooltipRect(geom.NewRect(100, 121, 0, 20), avoid, usable, 1))
	c.Equal(geom.NewRect(100, 121, 60, 1), fitTooltipRect(geom.NewRect(100, 121, 60, 0), avoid, usable, 1))
	c.Equal(geom.NewRect(399, 121, 1, 1), fitTooltipRect(geom.NewRect(400, 121, 0, 0), avoid, usable, 1),
		"kept on the usable area at its new size")
	tip := NewPanel()
	where := geom.NewRect(100, 121, 0, 0)
	c.Equal(geom.NewRect(100, 121, 1, 1), tooltipWindowRectOn(tip, where, avoid, nil))
	c.Equal(geom.NewRect(100, 121, 1, 1), tooltipWindowRectOn(tip, where, avoid,
		&Display{Usable: usable, headless: true}))
}

// TestTooltipWindowRectGoesByTheVisiblePart verifies that the tooltip of a panel extending past the window's edges is
// placed by the part of the panel within the window, rather than by an edge of the panel nobody can see.
func TestTooltipWindowRectGoesByTheVisiblePart(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 50, 200), geom.NewSize(60, 20))
		return []*Panel{panel}
	})
	screen.MouseMove(geom.NewPoint(45, 70), mod.None)
	tw := screen.TooltipWindow(owner)
	c.NotNil(tw)
	var rect, ownerRect geom.Rect
	c.True(screen.Do(func() {
		rect = tw.ContentRect()
		ownerRect = owner.ContentRect()
	}))
	c.Equal(geom.NewRect(30, 171, 60, 20), rect, "one unit below the window's bottom edge, where the panel is cut off")
	c.Equal(ownerRect.Bottom()+1, rect.Y)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
