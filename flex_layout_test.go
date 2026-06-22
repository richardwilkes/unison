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

// fixedPanel returns a panel whose minimum, preferred, and maximum sizes are all the given size.
func fixedPanel(width, height float32) *unison.Panel {
	return sizedPanel(geom.NewSize(width, height), geom.NewSize(width, height))
}

// sizedPanel returns a panel that reports the given minimum and preferred sizes, ignoring any hint.
func sizedPanel(minSize, prefSize geom.Size) *unison.Panel {
	p := unison.NewPanel()
	p.SetSizer(func(_ geom.Size) (mn, pref, mx geom.Size) {
		return minSize, prefSize, unison.MaxSize(prefSize)
	})
	return p
}

// newFlexParent creates a parent panel with the supplied FlexLayout and children attached.
func newFlexParent(layout *unison.FlexLayout, children ...*unison.Panel) *unison.Panel {
	parent := unison.NewPanel()
	parent.SetLayout(layout)
	for _, child := range children {
		parent.AddChild(child)
	}
	return parent
}

func TestFlexLayoutZeroColumns(t *testing.T) {
	c := check.New(t)
	parent := newFlexParent(&unison.FlexLayout{}, fixedPanel(100, 20), fixedPanel(50, 30))
	minSize, prefSize, maxSize := parent.Sizes(geom.Size{})
	c.Equal(geom.Size{}, minSize)
	c.Equal(geom.Size{}, prefSize)
	c.Equal(unison.MaxSize(geom.Size{}), maxSize)

	// PerformLayout with zero columns must not touch the children.
	parent.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	parent.ValidateLayout()
	c.Equal(geom.Rect{}, parent.Children()[0].FrameRect())
	c.Equal(geom.Rect{}, parent.Children()[1].FrameRect())
}

func TestFlexLayoutPreferredSizes(t *testing.T) {
	c := check.New(t)
	// Two columns, two rows. Column widths are the max of their cells; row heights likewise.
	parent := newFlexParent(&unison.FlexLayout{Columns: 2},
		fixedPanel(100, 20),
		fixedPanel(50, 30),
		fixedPanel(60, 40),
		fixedPanel(30, 10),
	)
	_, prefSize, maxSize := parent.Sizes(geom.Size{})
	c.Equal(geom.NewSize(150, 70), prefSize)
	c.True(maxSize.Width >= unison.DefaultMaxSize)
	c.True(maxSize.Height >= unison.DefaultMaxSize)
}

func TestFlexLayoutMinimumSizes(t *testing.T) {
	c := check.New(t)
	parent := newFlexParent(&unison.FlexLayout{Columns: 1},
		sizedPanel(geom.NewSize(50, 10), geom.NewSize(100, 20)),
	)
	minSize, prefSize, _ := parent.Sizes(geom.Size{})
	c.Equal(geom.NewSize(50, 10), minSize)
	c.Equal(geom.NewSize(100, 20), prefSize)
}

func TestFlexLayoutSpacing(t *testing.T) {
	c := check.New(t)
	parent := newFlexParent(&unison.FlexLayout{Columns: 2, HSpacing: 4, VSpacing: 6},
		fixedPanel(100, 20),
		fixedPanel(50, 30),
		fixedPanel(60, 40),
		fixedPanel(30, 10),
	)
	_, prefSize, _ := parent.Sizes(geom.Size{})
	// Widths: 100 + 50 + one 4px gap = 154. Heights: 30 + 40 + one 6px gap = 76.
	c.Equal(geom.NewSize(154, 76), prefSize)
}

func TestFlexLayoutNegativeSpacingClamped(t *testing.T) {
	c := check.New(t)
	layout := &unison.FlexLayout{Columns: 2, HSpacing: -10, VSpacing: -10}
	parent := newFlexParent(layout, fixedPanel(100, 20), fixedPanel(50, 30))
	_, prefSize, _ := parent.Sizes(geom.Size{})
	c.Equal(geom.NewSize(150, 30), prefSize)
	// The layout clamps the negative spacing back to zero as a side effect.
	c.Equal(float32(0), layout.HSpacing)
	c.Equal(float32(0), layout.VSpacing)
}

func TestFlexLayoutEqualColumns(t *testing.T) {
	c := check.New(t)
	parent := newFlexParent(&unison.FlexLayout{Columns: 2, EqualColumns: true},
		fixedPanel(100, 20),
		fixedPanel(40, 20),
	)
	_, prefSize, _ := parent.Sizes(geom.Size{})
	// Both columns take the widest column's width (100), so the total is 200 rather than 140.
	c.Equal(geom.NewSize(200, 20), prefSize)
}

func TestFlexLayoutHSpan(t *testing.T) {
	c := check.New(t)
	spanning := fixedPanel(200, 20)
	data := &unison.FlexLayoutData{HSpan: 2, VSpan: 1, HAlign: align.Start, VAlign: align.Middle}
	spanning.SetLayoutData(data)
	parent := newFlexParent(&unison.FlexLayout{Columns: 2},
		spanning,
		fixedPanel(30, 10),
		fixedPanel(30, 10),
	)
	_, prefSize, _ := parent.Sizes(geom.Size{})
	// The spanning child forces the combined width of the two columns to 200.
	c.Equal(float32(200), prefSize.Width)
}

func TestFlexLayoutBorderInsets(t *testing.T) {
	c := check.New(t)
	parent := newFlexParent(&unison.FlexLayout{Columns: 1}, fixedPanel(100, 20))
	parent.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(5)))
	_, prefSize, _ := parent.Sizes(geom.Size{})
	// 5px on every side adds 10 to each dimension.
	c.Equal(geom.NewSize(110, 30), prefSize)
}

func TestFlexLayoutPerformLayoutPositions(t *testing.T) {
	c := check.New(t)
	a := fixedPanel(100, 20)
	b := fixedPanel(50, 30)
	d := fixedPanel(30, 10)
	dParent := fixedPanel(60, 40)
	// Give d an explicit End vertical alignment to exercise that branch.
	d.SetLayoutData(&unison.FlexLayoutData{HSpan: 1, VSpan: 1, HAlign: align.Start, VAlign: align.End})
	parent := newFlexParent(&unison.FlexLayout{Columns: 2, HSpacing: 4, VSpacing: 6}, a, b, dParent, d)

	_, prefSize, _ := parent.Sizes(geom.Size{})
	parent.SetFrameRect(geom.NewRect(0, 0, prefSize.Width, prefSize.Height))
	parent.ValidateLayout()

	// Column widths: [100, 50]; row heights: [30, 40]; spacing 4 horizontal, 6 vertical.
	c.Equal(geom.NewRect(0, 5, 100, 20), a.FrameRect())  // VAlign middle in a 30-tall row
	c.Equal(geom.NewRect(104, 0, 50, 30), b.FrameRect()) // fills its row height
	c.Equal(geom.NewRect(0, 36, 60, 40), dParent.FrameRect())
	c.Equal(geom.NewRect(104, 66, 30, 10), d.FrameRect()) // VAlign end at the bottom of the 40-tall row
}

func TestFlexLayoutFillAndGrab(t *testing.T) {
	c := check.New(t)
	child := fixedPanel(50, 20)
	child.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: align.Fill,
		VAlign: align.Fill,
		HGrab:  true,
		VGrab:  true,
	})
	parent := newFlexParent(&unison.FlexLayout{Columns: 1}, child)
	parent.SetFrameRect(geom.NewRect(0, 0, 200, 100))
	parent.ValidateLayout()
	// A grabbing, filling child expands to consume all available space.
	c.Equal(geom.NewRect(0, 0, 200, 100), child.FrameRect())
}

func TestFlexLayoutAlignWithinTarget(t *testing.T) {
	c := check.New(t)
	child := fixedPanel(50, 20)
	parent := newFlexParent(&unison.FlexLayout{Columns: 1, HAlign: align.Middle, VAlign: align.Middle}, child)
	parent.SetFrameRect(geom.NewRect(0, 0, 100, 60))
	parent.ValidateLayout()
	// The content is centered within the larger target: (100-50)/2 = 25, (60-20)/2 = 20.
	c.Equal(geom.NewRect(25, 20, 50, 20), child.FrameRect())
}

func TestFlexLayoutAlignEndWithinTarget(t *testing.T) {
	c := check.New(t)
	child := fixedPanel(50, 20)
	parent := newFlexParent(&unison.FlexLayout{Columns: 1, HAlign: align.End, VAlign: align.End}, child)
	parent.SetFrameRect(geom.NewRect(0, 0, 100, 60))
	parent.ValidateLayout()
	c.Equal(geom.NewRect(50, 40, 50, 20), child.FrameRect())
}
