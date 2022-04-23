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
)

var _ Layout = &FlowLayout{}

// FlowLayout is a Layout that lays components out left to right, then top to bottom.
type FlowLayout struct {
	HSpacing float32
	VSpacing float32
}

// LayoutSizes implements Layout.
func (f *FlowLayout) LayoutSizes(target *Panel, hint Size) (min, pref, max Size) {
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
		min, pref, _ = child.Sizes(Size{})
		if largestChildMin.Width < min.Width {
			largestChildMin.Width = min.Width
		}
		if largestChildMin.Height < min.Height {
			largestChildMin.Height = min.Height
		}
		if pref.Width > availWidth {
			switch {
			case min.Width <= availWidth:
				pref.Width = availWidth
			case pt.X == insets.Left:
				pref.Width = min.Width
			default:
				pt.X = insets.Left
				pt.Y += maxHeight + f.VSpacing
				availWidth = width
				availHeight -= maxHeight + f.VSpacing
				maxHeight = 0
				if pref.Width > availWidth {
					if min.Width <= availWidth {
						pref.Width = availWidth
					} else {
						pref.Width = min.Width
					}
				}
			}
			savedWidth := pref.Width
			min, pref, _ = child.Sizes(Size{Width: pref.Width})
			pref.Width = savedWidth
			if pref.Height > availHeight {
				if min.Height <= availHeight {
					pref.Height = availHeight
				} else {
					pref.Height = min.Height
				}
			}
		}
		extent := pt.X + pref.Width
		if result.Width < extent {
			result.Width = extent
		}
		extent = pt.Y + pref.Height
		if result.Height < extent {
			result.Height = extent
		}
		if maxHeight < pref.Height {
			maxHeight = pref.Height
		}
		availWidth -= pref.Width + f.HSpacing
		if availWidth <= 0 {
			pt.X = insets.Left
			pt.Y += maxHeight + f.VSpacing
			availWidth = width
			availHeight -= maxHeight + f.VSpacing
			maxHeight = 0
		} else {
			pt.X += pref.Width + f.HSpacing
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
		min, pref, _ := child.Sizes(Size{})
		if pref.Width > availWidth {
			switch {
			case min.Width <= availWidth:
				pref.Width = availWidth
			case pt.X == insets.Left:
				pref.Width = min.Width
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
				if pref.Width > availWidth {
					if min.Width <= availWidth {
						pref.Width = availWidth
					} else {
						pref.Width = min.Width
					}
				}
			}
			savedWidth := pref.Width
			min, pref, _ = child.Sizes(Size{Width: pref.Width})
			pref.Width = savedWidth
			if pref.Height > availHeight {
				if min.Height <= availHeight {
					pref.Height = availHeight
				} else {
					pref.Height = min.Height
				}
			}
		}
		rects[i] = Rect{Point: pt, Size: pref}
		if maxHeight < pref.Height {
			maxHeight = pref.Height
		}
		availWidth -= pref.Width + f.HSpacing
		if availWidth <= 0 {
			pt.X = insets.Left
			pt.Y += maxHeight + f.VSpacing
			availWidth = width
			availHeight -= maxHeight + f.VSpacing
			f.applyRects(children[start:i+1], rects[start:i+1], maxHeight)
			start = i + 1
			maxHeight = 0
		} else {
			pt.X += pref.Width + f.HSpacing
		}
	}
	if start < len(children) {
		f.applyRects(children[start:], rects[start:], maxHeight)
	}
}

func (f *FlowLayout) applyRects(children []*Panel, rects []Rect, maxHeight float32) {
	for i, child := range children {
		vAlign, ok := child.LayoutData().(Alignment)
		if !ok {
			vAlign = StartAlignment
		}
		switch vAlign {
		case MiddleAlignment:
			if rects[i].Height < maxHeight {
				rects[i].Y += (maxHeight - rects[i].Height) / 2
			}
		case EndAlignment:
			rects[i].Y += maxHeight - rects[i].Height
		case FillAlignment:
			rects[i].Height = maxHeight
		default: // same as StartAlignment
		}
		child.SetFrameRect(rects[i])
	}
}
