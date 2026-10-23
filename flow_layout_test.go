// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
)

// newFlowParent creates a parent panel with the supplied FlowLayout and children attached.
func newFlowParent(layout *unison.FlowLayout, children ...*unison.Panel) *unison.Panel {
	parent := unison.NewPanel()
	parent.SetLayout(layout)
	for _, child := range children {
		parent.AddChild(child)
	}
	return parent
}

// TestFlowLayoutSingleRow checks the sizes of children that fit on one row: the preferred width sums them plus the
// spacing between them, and the height is the tallest.
func TestFlowLayoutSingleRow(t *testing.T) {
	for _, tc := range []struct {
		name     string
		want     geom.Size
		hSpacing float32
	}{
		{name: "no spacing", want: geom.NewSize(150, 30)},
		// One 4px horizontal gap between the two children: 100 + 4 + 50 = 154.
		{name: "spacing", want: geom.NewSize(154, 30), hSpacing: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			// With no hint, the available width is effectively infinite, so everything stays on one row.
			parent := newFlowParent(&unison.FlowLayout{HSpacing: tc.hSpacing}, fixedPanel(100, 20), fixedPanel(50, 30))
			minSize, prefSize, maxSize := parent.Sizes(geom.Size{})
			c.Equal(tc.want, prefSize)
			// The minimum size reports the largest child's minimum in each dimension.
			c.Equal(geom.NewSize(100, 30), minSize)
			c.True(maxSize.Width >= unison.DefaultMaxSize)
			c.True(maxSize.Height >= unison.DefaultMaxSize)
		})
	}
}

func TestFlowLayoutNegativeSpacingApplied(t *testing.T) {
	c := check.New(t)
	// Unlike FlexLayout, FlowLayout does not clamp negative spacing; it is applied literally, so the
	// children overlap by the spacing amount.
	layout := &unison.FlowLayout{HSpacing: -10}
	parent := newFlowParent(layout, fixedPanel(100, 20), fixedPanel(50, 30))
	parent.SetFrameRect(geom.NewRect(0, 0, 200, 30))
	parent.ValidateLayout()
	c.Equal(float32(-10), layout.HSpacing)
	c.Equal(geom.NewRect(0, 0, 100, 20), parent.Children()[0].FrameRect())
	// The second child starts 10px to the left of where it would with zero spacing.
	c.Equal(geom.NewRect(90, 0, 50, 30), parent.Children()[1].FrameRect())
}

func TestFlowLayoutWraps(t *testing.T) {
	c := check.New(t)
	// A 120px-wide hint cannot fit both children side by side, so the second wraps to a new row.
	parent := newFlowParent(&unison.FlowLayout{}, fixedPanel(100, 20), fixedPanel(50, 30))
	minSize, prefSize, _ := parent.Sizes(geom.NewSize(120, 0))
	// Width is the widest row (100); height stacks the two rows (20 + 30).
	c.Equal(geom.NewSize(100, 50), prefSize)
	c.Equal(geom.NewSize(100, 30), minSize)
}

// TestFlowLayoutClampsToAvailableWidth checks that a child whose preferred width exceeds a 60px hint is clamped to the
// hint, or to its own minimum width when that is wider still.
func TestFlowLayoutClampsToAvailableWidth(t *testing.T) {
	for _, tc := range []struct {
		name      string
		minWidth  float32
		wantWidth float32
	}{
		// The minimum width fits, so the child is clamped to the hint.
		{name: "minimum fits the hint", minWidth: 40, wantWidth: 60},
		// The minimum width doesn't fit either, and the child is first on its row, so its minimum width wins even
		// though it overflows the hint.
		{name: "minimum overflows the hint", minWidth: 80, wantWidth: 80},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			child := sizedPanel(geom.NewSize(tc.minWidth, 10), geom.NewSize(100, 20))
			parent := newFlowParent(&unison.FlowLayout{}, child)
			minSize, prefSize, _ := parent.Sizes(geom.NewSize(60, 0))
			c.Equal(geom.NewSize(tc.wantWidth, 20), prefSize)
			c.Equal(geom.NewSize(tc.minWidth, 10), minSize)
		})
	}
}

// TestFlowLayoutBorderInsets checks that the minimum and preferred sizes add the border's insets to the content, which
// leaves a panel with neither children nor a border at zero.
func TestFlowLayoutBorderInsets(t *testing.T) {
	for _, tc := range []struct {
		border   unison.Border
		name     string
		children []*unison.Panel
		want     geom.Size
	}{
		{name: "no children and no border"},
		// 5px on every side adds 10 to each dimension.
		{
			border:   unison.NewEmptyBorder(geom.NewUniformInsets(5)),
			name:     "uniform insets around a child",
			children: []*unison.Panel{fixedPanel(100, 20)},
			want:     geom.NewSize(110, 30),
		},
		// An empty panel's preferred size is just its insets, so nothing masks the accumulator's seed values: the
		// width must be Left+Right (3+5) and the height Top+Bottom (7+11). Seeding the width from the top inset
		// instead of the left one reported Top+Right (12) here.
		{
			border: unison.NewEmptyBorder(geom.NewInsets(7, 3, 11, 5)),
			name:   "asymmetric insets with no children",
			want:   geom.NewSize(8, 18),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			parent := newFlowParent(&unison.FlowLayout{}, tc.children...)
			parent.SetBorder(tc.border)
			minSize, prefSize, maxSize := parent.Sizes(geom.Size{})
			c.Equal(tc.want, prefSize)
			c.Equal(tc.want, minSize)
			c.Equal(unison.MaxSize(tc.want), maxSize)
		})
	}
}

func TestFlowLayoutVerticalAlignment(t *testing.T) {
	c := check.New(t)
	// All four children share a single 40-tall row (the tallest child, b). Each exercises a different
	// vertical alignment, and the final child also triggers the "available width exhausted" wrap branch.
	a := fixedPanel(60, 20)
	a.SetLayoutData(align.Middle)
	b := fixedPanel(60, 40)
	d := fixedPanel(60, 10)
	d.SetLayoutData(align.End)
	e := fixedPanel(60, 10)
	e.SetLayoutData(align.Fill)
	parent := newFlowParent(&unison.FlowLayout{}, a, b, d, e)
	parent.SetFrameRect(geom.NewRect(0, 0, 240, 40))
	parent.ValidateLayout()

	c.Equal(geom.NewRect(0, 10, 60, 20), a.FrameRect())   // centered within the 40-tall row
	c.Equal(geom.NewRect(60, 0, 60, 40), b.FrameRect())   // defines the row height, default Start alignment
	c.Equal(geom.NewRect(120, 30, 60, 10), d.FrameRect()) // pinned to the bottom of the row
	c.Equal(geom.NewRect(180, 0, 60, 40), e.FrameRect())  // stretched to fill the row height
}

func TestFlowLayoutPerformLayoutWraps(t *testing.T) {
	c := check.New(t)
	a := fixedPanel(100, 20)
	b := fixedPanel(50, 30)
	parent := newFlowParent(&unison.FlowLayout{VSpacing: 6}, a, b)
	// Only wide enough for the first child, forcing the second onto a new row below it.
	parent.SetFrameRect(geom.NewRect(0, 0, 120, 100))
	parent.ValidateLayout()
	c.Equal(geom.NewRect(0, 0, 100, 20), a.FrameRect())
	// Second row starts below the first row's height (20) plus the vertical spacing (6).
	c.Equal(geom.NewRect(0, 26, 50, 30), b.FrameRect())
}
