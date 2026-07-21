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

func TestFlowLayoutEmpty(t *testing.T) {
	c := check.New(t)
	parent := newFlowParent(&unison.FlowLayout{})
	minSize, prefSize, maxSize := parent.Sizes(geom.Size{})
	c.Equal(geom.Size{}, minSize)
	c.Equal(geom.Size{}, prefSize)
	c.Equal(unison.MaxSize(geom.Size{}), maxSize)
}

func TestFlowLayoutSingleRow(t *testing.T) {
	c := check.New(t)
	// With no hint, the available width is effectively infinite, so everything stays on one row.
	parent := newFlowParent(&unison.FlowLayout{}, fixedPanel(100, 20), fixedPanel(50, 30))
	minSize, prefSize, maxSize := parent.Sizes(geom.Size{})
	// Preferred width is the sum of the children; height is the tallest child.
	c.Equal(geom.NewSize(150, 30), prefSize)
	// The minimum size reports the largest child's minimum in each dimension.
	c.Equal(geom.NewSize(100, 30), minSize)
	c.True(maxSize.Width >= unison.DefaultMaxSize)
	c.True(maxSize.Height >= unison.DefaultMaxSize)
}

func TestFlowLayoutSpacing(t *testing.T) {
	c := check.New(t)
	parent := newFlowParent(&unison.FlowLayout{HSpacing: 4}, fixedPanel(100, 20), fixedPanel(50, 30))
	_, prefSize, _ := parent.Sizes(geom.Size{})
	// One 4px horizontal gap between the two children: 100 + 4 + 50 = 154.
	c.Equal(geom.NewSize(154, 30), prefSize)
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

func TestFlowLayoutClampsToAvailableWidth(t *testing.T) {
	c := check.New(t)
	// The preferred width exceeds the hint, but the minimum width fits, so the child is clamped to the hint.
	child := sizedPanel(geom.NewSize(40, 10), geom.NewSize(100, 20))
	parent := newFlowParent(&unison.FlowLayout{}, child)
	minSize, prefSize, _ := parent.Sizes(geom.NewSize(60, 0))
	c.Equal(geom.NewSize(60, 20), prefSize)
	c.Equal(geom.NewSize(40, 10), minSize)
}

func TestFlowLayoutClampsToChildMinimum(t *testing.T) {
	c := check.New(t)
	// Neither the preferred nor the minimum width fits the hint, and the child is first on its row, so its
	// minimum width wins even though it overflows the hint.
	child := sizedPanel(geom.NewSize(80, 10), geom.NewSize(100, 20))
	parent := newFlowParent(&unison.FlowLayout{}, child)
	_, prefSize, _ := parent.Sizes(geom.NewSize(60, 0))
	c.Equal(geom.NewSize(80, 20), prefSize)
}

func TestFlowLayoutBorderInsets(t *testing.T) {
	c := check.New(t)
	parent := newFlowParent(&unison.FlowLayout{}, fixedPanel(100, 20))
	parent.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(5)))
	minSize, prefSize, _ := parent.Sizes(geom.Size{})
	// 5px on every side adds 10 to each dimension.
	c.Equal(geom.NewSize(110, 30), prefSize)
	c.Equal(geom.NewSize(110, 30), minSize)
}

func TestFlowLayoutAsymmetricBorderInsets(t *testing.T) {
	c := check.New(t)
	// An empty panel's preferred size is just its insets, so nothing masks the accumulator's seed values: the
	// width must be Left+Right (3+5) and the height Top+Bottom (7+11). Seeding the width from the top inset
	// instead of the left one reported Top+Right (12) here.
	parent := newFlowParent(&unison.FlowLayout{})
	parent.SetBorder(unison.NewEmptyBorder(geom.NewInsets(7, 3, 11, 5)))
	minSize, prefSize, _ := parent.Sizes(geom.Size{})
	c.Equal(geom.NewSize(8, 18), prefSize)
	c.Equal(geom.NewSize(8, 18), minSize)
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
