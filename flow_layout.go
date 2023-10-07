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
	"math"

	"github.com/richardwilkes/unison/enums/align"
)

var _ Layout = &FlowLayout{}

// FlowLayout is a Layout that lays components out left to right, then top to bottom.
type FlowLayout struct {
	HSpacing float32
	VSpacing float32
}

// LayoutSizes implements Layout.
func (f *FlowLayout) LayoutSizes(target *Panel, hint Size) (minSize, prefSize, maxSize Size) {
	var insets Insets
	if b := target.Border(); b != nil {
		insets = b.Insets()
	}
	if hint.Width < 1 {
		hint.Width = math.MaxFloat32
	}
	if hint.Height < 1 {
		hint.Height = math.MaxFloat32
	}
	width := hint.Width - insets.Width()
	pt := Point{X: insets.Left, Y: insets.Top}
	result := Size{Width: pt.Y, Height: pt.Y}
	availWidth := width
	availHeight := hint.Height - insets.Height()
	var maxHeight float32
	var largestChildMin Size
	for _, child := range target.Children() {
		minSize, prefSize, _ = child.Sizes(Size{})
		if largestChildMin.Width < minSize.Width {
			largestChildMin.Width = minSize.Width
		}
		if largestChildMin.Height < minSize.Height {
			largestChildMin.Height = minSize.Height
		}
		if prefSize.Width > availWidth {
			switch {
			case minSize.Width <= availWidth:
				prefSize.Width = availWidth
			case pt.X == insets.Left:
				prefSize.Width = minSize.Width
			default:
				pt.X = insets.Left
				pt.Y += maxHeight + f.VSpacing
				availWidth = width
				availHeight -= maxHeight + f.VSpacing
				maxHeight = 0
				if prefSize.Width > availWidth {
					if minSize.Width <= availWidth {
						prefSize.Width = availWidth
					} else {
						prefSize.Width = minSize.Width
					}
				}
			}
			savedWidth := prefSize.Width
			minSize, prefSize, _ = child.Sizes(Size{Width: prefSize.Width})
			prefSize.Width = savedWidth
			if prefSize.Height > availHeight {
				if minSize.Height <= availHeight {
					prefSize.Height = availHeight
				} else {
					prefSize.Height = minSize.Height
				}
			}
		}
		extent := pt.X + prefSize.Width
		if result.Width < extent {
			result.Width = extent
		}
		extent = pt.Y + prefSize.Height
		if result.Height < extent {
			result.Height = extent
		}
		if maxHeight < prefSize.Height {
			maxHeight = prefSize.Height
		}
		availWidth -= prefSize.Width + f.HSpacing
		if availWidth <= 0 {
			pt.X = insets.Left
			pt.Y += maxHeight + f.VSpacing
			availWidth = width
			availHeight -= maxHeight + f.VSpacing
			maxHeight = 0
		} else {
			pt.X += prefSize.Width + f.HSpacing
		}
	}
	result.Width += insets.Right
	result.Height += insets.Bottom
	largestChildMin.Width += insets.Width()
	largestChildMin.Height += insets.Height()
	return largestChildMin, result, MaxSize(result)
}

// PerformLayout implements Layout.
func (f *FlowLayout) PerformLayout(target *Panel) {
	var insets Insets
	if b := target.Border(); b != nil {
		insets = b.Insets()
	}
	size := target.ContentRect(true).Size
	width := size.Width - insets.Width()
	pt := Point{X: insets.Left, Y: insets.Top}
	availWidth := width
	availHeight := size.Height - insets.Height()
	var maxHeight float32
	children := target.Children()
	rects := make([]Rect, len(children))
	start := 0
	for i, child := range children {
		minSize, prefSize, _ := child.Sizes(Size{})
		if prefSize.Width > availWidth {
			switch {
			case minSize.Width <= availWidth:
				prefSize.Width = availWidth
			case pt.X == insets.Left:
				prefSize.Width = minSize.Width
			default:
				pt.X = insets.Left
				pt.Y += maxHeight + f.VSpacing
				availWidth = width
				availHeight -= maxHeight + f.VSpacing
				if i > start {
					f.applyRects(children[start:i], rects[start:i], maxHeight)
					start = i
				}
				maxHeight = 0
				if prefSize.Width > availWidth {
					if minSize.Width <= availWidth {
						prefSize.Width = availWidth
					} else {
						prefSize.Width = minSize.Width
					}
				}
			}
			savedWidth := prefSize.Width
			minSize, prefSize, _ = child.Sizes(Size{Width: prefSize.Width})
			prefSize.Width = savedWidth
			if prefSize.Height > availHeight {
				if minSize.Height <= availHeight {
					prefSize.Height = availHeight
				} else {
					prefSize.Height = minSize.Height
				}
			}
		}
		rects[i] = Rect{Point: pt, Size: prefSize}
		if maxHeight < prefSize.Height {
			maxHeight = prefSize.Height
		}
		availWidth -= prefSize.Width + f.HSpacing
		if availWidth <= 0 {
			pt.X = insets.Left
			pt.Y += maxHeight + f.VSpacing
			availWidth = width
			availHeight -= maxHeight + f.VSpacing
			f.applyRects(children[start:i+1], rects[start:i+1], maxHeight)
			start = i + 1
			maxHeight = 0
		} else {
			pt.X += prefSize.Width + f.HSpacing
		}
	}
	if start < len(children) {
		f.applyRects(children[start:], rects[start:], maxHeight)
	}
}

func (f *FlowLayout) applyRects(children []*Panel, rects []Rect, maxHeight float32) {
	for i, child := range children {
		vAlign, ok := child.LayoutData().(align.Enum)
		if !ok {
			vAlign = align.Start
		}
		switch vAlign {
		case align.Middle:
			if rects[i].Height < maxHeight {
				rects[i].Y += (maxHeight - rects[i].Height) / 2
			}
		case align.End:
			rects[i].Y += maxHeight - rects[i].Height
		case align.Fill:
			rects[i].Height = maxHeight
		default: // same as align.Start
		}
		child.SetFrameRect(rects[i])
	}
}
