// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
	"github.com/richardwilkes/unison/enums/align"
)

var _ Layout = &FlexLayout{}

// FlexLayout lays out the children of its Layoutable based on the FlexLayoutData assigned to each child.
type FlexLayout struct {
	sizingCache  map[*Panel]map[geom.Size]*flexSizingCacheData
	rows         int
	Columns      int
	HSpacing     float32
	VSpacing     float32
	HAlign       align.Enum
	VAlign       align.Enum
	EqualColumns bool
}

type flexSizingCacheData struct {
	minSize  geom.Size
	prefSize geom.Size
	maxSize  geom.Size
}

// FlexLayoutData is used to control how an object is laid out by the FlexLayout layout.
type FlexLayoutData struct {
	cacheSize    geom.Size
	minCacheSize geom.Size
	SizeHint     geom.Size
	MinSize      geom.Size
	HSpan        int
	VSpan        int
	HAlign       align.Enum
	VAlign       align.Enum
	HGrab        bool
	VGrab        bool
}

// LayoutSizes implements the Layout interface.
func (f *FlexLayout) LayoutSizes(target *Panel, hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	f.sizingCache = make(map[*Panel]map[geom.Size]*flexSizingCacheData)
	var insets geom.Size
	if b := target.Border(); b != nil {
		insets = b.Insets().Size()
		hint = hint.Sub(insets).Max(geom.Size{})
	}
	prefSize = f.layout(target, geom.Point{}, hint, false, false).Add(insets)
	return f.layout(target, geom.Point{}, hint, false, true).Add(insets), prefSize, MaxSize(prefSize)
}

// PerformLayout implements the Layout interface.
func (f *FlexLayout) PerformLayout(target *Panel) {
	f.sizingCache = make(map[*Panel]map[geom.Size]*flexSizingCacheData)
	var insets geom.Insets
	if b := target.Border(); b != nil {
		insets = b.Insets()
	}
	f.layout(target, geom.Point{X: insets.Left, Y: insets.Top}, target.ContentRect(true).Size.Sub(insets.Size()), true, false)
}

func (f *FlexLayout) layout(target *Panel, location geom.Point, hint geom.Size, move, useMinimumSize bool) geom.Size {
	var totalSize geom.Size
	if f.Columns > 0 {
		children := f.prepChildren(target, useMinimumSize)
		if len(children) > 0 {
			if f.HSpacing < 0 {
				f.HSpacing = 0
			}
			if f.VSpacing < 0 {
				f.VSpacing = 0
			}
			grid := f.buildGrid(children)
			widths := f.adjustColumnWidths(hint.Width, grid)
			f.wrap(hint.Width, grid, widths, useMinimumSize)
			heights := f.adjustRowHeights(hint.Height, grid)
			totalSize.Width += f.HSpacing * float32(f.Columns-1)
			totalSize.Height += f.VSpacing * float32(f.rows-1)
			for i := 0; i < f.Columns; i++ {
				totalSize.Width += widths[i]
			}
			for i := 0; i < f.rows; i++ {
				totalSize.Height += heights[i]
			}
			if move {
				if totalSize.Width < hint.Width {
					switch f.HAlign {
					case align.Middle:
						location.X += xmath.Round((hint.Width - totalSize.Width) / 2)
					case align.End:
						location.X += hint.Width - totalSize.Width
					}
				}
				if totalSize.Height < hint.Height {
					switch f.VAlign {
					case align.Middle:
						location.Y += xmath.Round((hint.Height - totalSize.Height) / 2)
					case align.End:
						location.Y += hint.Height - totalSize.Height
					}
				}
				f.positionChildren(location, grid, widths, heights)
			}
		}
	}
	return totalSize
}

func (f *FlexLayout) sizingCacheData(panel *Panel, hint geom.Size) *flexSizingCacheData {
	m, ok := f.sizingCache[panel]
	if !ok {
		m = make(map[geom.Size]*flexSizingCacheData)
		f.sizingCache[panel] = m
	}
	var data *flexSizingCacheData
	if data, ok = m[hint]; !ok {
		var sizing flexSizingCacheData
		sizing.minSize, sizing.prefSize, sizing.maxSize = panel.Sizes(hint)
		data = &sizing
		m[hint] = data
	}
	return data
}

func (f *FlexLayout) prepChildren(target *Panel, useMinimumSize bool) []*Panel {
	var hint geom.Size
	children := target.Children()
	for _, child := range children {
		getDataFromTarget(child).computeCacheSize(f.sizingCacheData(child, hint), hint, useMinimumSize)
	}
	return children
}

func getDataFromTarget(target *Panel) *FlexLayoutData {
	if data, ok := target.LayoutData().(*FlexLayoutData); ok {
		return data
	}
	data := &FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		VAlign: align.Middle,
	}
	target.layoutData = data
	return data
}

func (f *FlexLayout) buildGrid(children []*Panel) [][]*Panel {
	var grid [][]*Panel
	var row, column int
	f.rows = 0
	for _, child := range children {
		data := getDataFromTarget(child)
		hSpan := max(1, min(data.HSpan, f.Columns))
		vSpan := max(1, data.VSpan)
		for {
			lastRow := row + vSpan
			for lastRow >= len(grid) {
				grid = append(grid, make([]*Panel, f.Columns))
			}
			// noinspection GoNilness
			for column < f.Columns && grid[row][column] != nil {
				column++
			}
			endCount := column + hSpan
			if endCount <= f.Columns {
				index := column
				// noinspection GoNilness
				for index < endCount && grid[row][index] == nil {
					index++
				}
				if index == endCount {
					break
				}
				column = index
			}
			if column+hSpan >= f.Columns {
				column = 0
				row++
			}
		}
		for j := 0; j < vSpan; j++ {
			pos := row + j
			for k := 0; k < hSpan; k++ {
				// noinspection GoNilness
				grid[pos][column+k] = child
			}
		}
		f.rows = max(f.rows, row+vSpan)
		column += hSpan
	}
	return grid
}

func (f *FlexLayout) adjustColumnWidths(width float32, grid [][]*Panel) []float32 {
	availableWidth := width - f.HSpacing*float32(f.Columns-1)
	expandCount := 0
	widths := make([]float32, f.Columns)
	minWidths := make([]float32, f.Columns)
	expandColumn := make([]bool, f.Columns)
	for j := 0; j < f.Columns; j++ {
		for i := 0; i < f.rows; i++ {
			data := f.getData(grid, i, j, true)
			if data != nil {
				hSpan := max(1, min(data.HSpan, f.Columns))
				if hSpan == 1 {
					w := data.cacheSize.Width
					if widths[j] < w {
						widths[j] = w
					}
					if data.HGrab {
						if !expandColumn[j] {
							expandCount++
						}
						expandColumn[j] = true
					}
					minimumWidth := data.minCacheSize.Width
					if !data.HGrab {
						if minimumWidth < 1 {
							w = data.cacheSize.Width
						} else {
							w = minimumWidth
						}
						if minWidths[j] < w {
							minWidths[j] = w
						}
					}
				}
			}
		}
		for i := 0; i < f.rows; i++ {
			data := f.getData(grid, i, j, false)
			if data != nil {
				hSpan := max(1, min(data.HSpan, f.Columns))
				if hSpan > 1 {
					var spanWidth, spanMinWidth float32
					spanExpandCount := 0
					for k := 0; k < hSpan; k++ {
						spanWidth += widths[j-k]
						spanMinWidth += minWidths[j-k]
						if expandColumn[j-k] {
							spanExpandCount++
						}
					}
					if data.HGrab && spanExpandCount == 0 {
						expandCount++
						expandColumn[j] = true
					}
					w := data.cacheSize.Width - spanWidth - float32(hSpan-1)*f.HSpacing
					if w > 0 {
						if f.EqualColumns {
							equalWidth := xmath.Floor((w + spanWidth) / float32(hSpan))
							for k := 0; k < hSpan; k++ {
								if widths[j-k] < equalWidth {
									widths[j-k] = equalWidth
								}
							}
						} else {
							f.apportionExtra(w, j, spanExpandCount, hSpan, expandColumn, widths)
						}
					}
					minimumWidth := data.minCacheSize.Width
					if !data.HGrab || minimumWidth != 0 {
						if !data.HGrab || minimumWidth < 1 {
							w = data.cacheSize.Width
						} else {
							w = minimumWidth
						}
						w -= spanMinWidth + float32(hSpan-1)*f.HSpacing
						if w > 0 {
							f.apportionExtra(w, j, spanExpandCount, hSpan, expandColumn, minWidths)
						}
					}
				}
			}
		}
	}
	if f.EqualColumns {
		var minColumnWidth, columnWidth float32
		for i := 0; i < f.Columns; i++ {
			if minColumnWidth < minWidths[i] {
				minColumnWidth = minWidths[i]
			}
			if columnWidth < widths[i] {
				columnWidth = widths[i]
			}
		}
		if width > 0 && expandCount != 0 {
			columnWidth = max(minColumnWidth, xmath.Floor(availableWidth/float32(f.Columns)))
		}
		for i := 0; i < f.Columns; i++ {
			expandColumn[i] = expandCount > 0
			widths[i] = columnWidth
		}
	}
	// The following is still run even when EqualColumns is set, so that the overall width is consistent.
	expandCount = 0
	for i := 0; i < f.Columns; i++ {
		if expandColumn[i] {
			expandCount++
		}
	}
	if width > 0 && expandCount > 0 {
		var totalWidth float32
		for i := 0; i < f.Columns; i++ {
			totalWidth += widths[i]
		}
		c := expandCount
		for xmath.Abs(totalWidth-availableWidth) > 0.01 {
			delta := (availableWidth - totalWidth) / float32(c)
			for j := 0; j < f.Columns; j++ {
				if expandColumn[j] {
					if widths[j]+delta > minWidths[j] {
						widths[j] += delta
					} else {
						widths[j] = minWidths[j]
						expandColumn[j] = false
						c--
					}
				}
			}
			for j := 0; j < f.Columns; j++ {
				for i := 0; i < f.rows; i++ {
					data := f.getData(grid, i, j, false)
					if data != nil {
						hSpan := max(1, min(data.HSpan, f.Columns))
						if hSpan > 1 {
							minimumWidth := data.minCacheSize.Width
							if !data.HGrab || minimumWidth != 0 {
								var spanWidth float32
								spanExpandCount := 0
								for k := 0; k < hSpan; k++ {
									spanWidth += widths[j-k]
									if expandColumn[j-k] {
										spanExpandCount++
									}
								}
								var w float32
								if !data.HGrab || minimumWidth < 1 {
									w = data.cacheSize.Width
								} else {
									w = minimumWidth
								}
								w -= spanWidth + float32(hSpan-1)*f.HSpacing
								if w > 0 {
									f.apportionExtra(w, j, spanExpandCount, hSpan, expandColumn, widths)
								}
							}
						}
					}
				}
			}
			if c == 0 {
				break
			}
			totalWidth = 0
			for i := 0; i < f.Columns; i++ {
				totalWidth += widths[i]
			}
		}
	}
	return widths
}

func (f *FlexLayout) apportionExtra(extra float32, base, count, span int, expand []bool, values []float32) {
	if count == 0 {
		values[base] += extra
	} else {
		extraInt := int(xmath.Floor(extra))
		delta := extraInt / count
		remainder := extraInt - delta*count
		for i := 0; i < span; i++ {
			j := base - i
			if expand[j] {
				values[j] += float32(delta)
			}
		}
		for remainder > 0 {
			for i := 0; i < span; i++ {
				j := base - i
				if expand[j] {
					values[j]++
					remainder--
					if remainder == 0 {
						break
					}
				}
			}
		}
	}
}

func (f *FlexLayout) getData(grid [][]*Panel, row, column int, first bool) *FlexLayoutData {
	target := grid[row][column]
	if target != nil {
		data := getDataFromTarget(target)
		hSpan := max(1, min(data.HSpan, f.Columns))
		vSpan := max(1, data.VSpan)
		var i, j int
		if first {
			i = row + vSpan - 1
			j = column + hSpan - 1
		} else {
			i = row - vSpan + 1
			j = column - hSpan + 1
		}
		if i >= 0 && i < f.rows {
			if j >= 0 && j < f.Columns {
				if target == grid[i][j] {
					return data
				}
			}
		}
	}
	return nil
}

func (f *FlexLayout) wrap(width float32, grid [][]*Panel, widths []float32, useMinimumSize bool) {
	if width > 0 {
		for j := 0; j < f.Columns; j++ {
			for i := 0; i < f.rows; i++ {
				data := f.getData(grid, i, j, false)
				if data != nil {
					if data.SizeHint.Height < 1 {
						hSpan := max(1, min(data.HSpan, f.Columns))
						var currentWidth float32
						for k := 0; k < hSpan; k++ {
							currentWidth += widths[j-k]
						}
						currentWidth += float32(hSpan-1) * f.HSpacing
						if currentWidth != data.cacheSize.Width && data.HAlign == align.Fill || data.cacheSize.Width > currentWidth {
							hint := geom.Size{Width: max(data.minCacheSize.Width, currentWidth)}
							data.computeCacheSize(f.sizingCacheData(grid[i][j], hint), hint, useMinimumSize)
							minimumHeight := data.MinSize.Height
							if data.VGrab && minimumHeight > 0 && data.cacheSize.Height < minimumHeight {
								data.cacheSize.Height = minimumHeight
							}
						}
					}
				}
			}
		}
	}
}

func (f *FlexLayout) adjustRowHeights(height float32, grid [][]*Panel) []float32 {
	availableHeight := height - f.VSpacing*float32(f.rows-1)
	expandCount := 0
	heights := make([]float32, f.rows)
	minHeights := make([]float32, f.rows)
	expandRow := make([]bool, f.rows)
	for i := 0; i < f.rows; i++ {
		for j := 0; j < f.Columns; j++ {
			data := f.getData(grid, i, j, true)
			if data != nil {
				vSpan := max(1, min(data.VSpan, f.rows))
				if vSpan == 1 {
					h := data.cacheSize.Height
					if heights[i] < h {
						heights[i] = h
					}
					if data.VGrab {
						if !expandRow[i] {
							expandCount++
						}
						expandRow[i] = true
					}
					minimumHeight := data.MinSize.Height
					if !data.VGrab || minimumHeight != 0 {
						if !data.VGrab || minimumHeight < 1 {
							h = data.minCacheSize.Height
						} else {
							h = minimumHeight
						}
						if minHeights[i] < h {
							minHeights[i] = h
						}
					}
				}
			}
		}
		for j := 0; j < f.Columns; j++ {
			data := f.getData(grid, i, j, false)
			if data != nil {
				vSpan := max(1, min(data.VSpan, f.rows))
				if vSpan > 1 {
					var spanHeight, spanMinHeight float32
					spanExpandCount := 0
					for k := 0; k < vSpan; k++ {
						spanHeight += heights[i-k]
						spanMinHeight += minHeights[i-k]
						if expandRow[i-k] {
							spanExpandCount++
						}
					}
					if data.VGrab && spanExpandCount == 0 {
						expandCount++
						expandRow[i] = true
					}
					h := data.cacheSize.Height - spanHeight - float32(vSpan-1)*f.VSpacing
					if h > 0 {
						if spanExpandCount == 0 {
							heights[i] += h
						} else {
							delta := h / float32(spanExpandCount)
							for k := 0; k < vSpan; k++ {
								if expandRow[i-k] {
									heights[i-k] += delta
								}
							}
						}
					}
					minimumHeight := data.MinSize.Height
					if !data.VGrab || minimumHeight != 0 {
						if !data.VGrab || minimumHeight < 1 {
							h = data.minCacheSize.Height
						} else {
							h = minimumHeight
						}
						h -= spanMinHeight + float32(vSpan-1)*f.VSpacing
						if h > 0 {
							f.apportionExtra(h, i, spanExpandCount, vSpan, expandRow, minHeights)
						}
					}
				}
			}
		}
	}
	if height > 0 && expandCount > 0 {
		var totalHeight float32
		for i := 0; i < f.rows; i++ {
			totalHeight += heights[i]
		}
		c := expandCount
		delta := (availableHeight - totalHeight) / float32(c)
		for xmath.Abs(totalHeight-availableHeight) > 0.01 {
			for i := 0; i < f.rows; i++ {
				if expandRow[i] {
					if heights[i]+delta > minHeights[i] {
						heights[i] += delta
					} else {
						heights[i] = minHeights[i]
						expandRow[i] = false
						c--
					}
				}
			}
			for i := 0; i < f.rows; i++ {
				for j := 0; j < f.Columns; j++ {
					data := f.getData(grid, i, j, false)
					if data != nil {
						vSpan := max(1, min(data.VSpan, f.rows))
						if vSpan > 1 {
							minimumHeight := data.MinSize.Height
							if !data.VGrab || minimumHeight != 0 {
								var spanHeight float32
								spanExpandCount := 0
								for k := 0; k < vSpan; k++ {
									spanHeight += heights[i-k]
									if expandRow[i-k] {
										spanExpandCount++
									}
								}
								var h float32
								if !data.VGrab || minimumHeight < 1 {
									h = data.minCacheSize.Height
								} else {
									h = minimumHeight
								}
								h -= spanHeight + float32(vSpan-1)*f.VSpacing
								if h > 0 {
									f.apportionExtra(h, i, spanExpandCount, vSpan, expandRow, heights)
								}
							}
						}
					}
				}
			}
			if c == 0 {
				break
			}
			totalHeight = 0
			for i := 0; i < f.rows; i++ {
				totalHeight += heights[i]
			}
			delta = (availableHeight - totalHeight) / float32(c)
		}
	}
	return heights
}

func (f *FlexLayout) positionChildren(location geom.Point, grid [][]*Panel, widths, heights []float32) {
	gridY := location.Y
	for i := 0; i < f.rows; i++ {
		gridX := location.X
		for j := 0; j < f.Columns; j++ {
			data := f.getData(grid, i, j, true)
			if data != nil {
				hSpan := max(1, min(data.HSpan, f.Columns))
				vSpan := max(1, data.VSpan)
				var cellWidth, cellHeight float32
				for k := 0; k < hSpan; k++ {
					cellWidth += widths[j+k]
				}
				for k := 0; k < vSpan; k++ {
					cellHeight += heights[i+k]
				}
				cellWidth += f.HSpacing * float32(hSpan-1)
				childX := gridX
				childWidth := min(data.cacheSize.Width, cellWidth)
				switch data.HAlign {
				case align.Middle:
					childX += max(0, (cellWidth-childWidth)/2)
				case align.End:
					childX += max(0, cellWidth-childWidth)
				case align.Fill:
					childWidth = cellWidth
				default:
				}
				cellHeight += f.VSpacing * float32(vSpan-1)
				childY := gridY
				childHeight := min(data.cacheSize.Height, cellHeight)
				switch data.VAlign {
				case align.Middle:
					childY += max(0, (cellHeight-childHeight)/2)
				case align.End:
					childY += max(0, cellHeight-childHeight)
				case align.Fill:
					childHeight = cellHeight
				default:
				}
				child := grid[i][j]
				if child != nil {
					child.SetFrameRect(geom.Rect{Point: geom.Point{X: childX, Y: childY}, Size: geom.Size{Width: childWidth, Height: childHeight}})
				}
			}
			gridX += widths[j] + f.HSpacing
		}
		gridY += heights[i] + f.VSpacing
	}
}

func (f *FlexLayoutData) computeCacheSize(sizing *flexSizingCacheData, hint geom.Size, useMinimumSize bool) {
	f.cacheSize.Width = 0
	f.cacheSize.Height = 0
	f.minCacheSize.Width = 0
	f.minCacheSize.Height = 0
	if f.SizeHint.Width < 0 {
		f.SizeHint.Width = 0
	}
	if f.SizeHint.Height < 0 {
		f.SizeHint.Height = 0
	}
	if f.MinSize.Width < 0 {
		f.MinSize.Width = 0
	}
	if f.MinSize.Height < 0 {
		f.MinSize.Height = 0
	}
	if f.HSpan < 1 {
		f.HSpan = 1
	}
	if f.VSpan < 1 {
		f.VSpan = 1
	}
	if hint.Width > 0 || hint.Height > 0 {
		if f.MinSize.Width > 0 {
			f.minCacheSize.Width = f.MinSize.Width
		} else {
			f.minCacheSize.Width = sizing.minSize.Width
		}
		if hint.Width > 0 && hint.Width < f.minCacheSize.Width {
			hint.Width = f.minCacheSize.Width
		}
		if hint.Width > 0 && hint.Width > sizing.maxSize.Width {
			hint.Width = sizing.maxSize.Width
		}
		if f.MinSize.Height > 0 {
			f.minCacheSize.Height = f.MinSize.Height
		} else {
			f.minCacheSize.Height = sizing.minSize.Height
		}
		if hint.Height > 0 && hint.Height < f.minCacheSize.Height {
			hint.Height = f.minCacheSize.Height
		}
		if hint.Height > 0 && hint.Height > sizing.maxSize.Height {
			hint.Height = sizing.maxSize.Height
		}
	}
	if useMinimumSize {
		f.cacheSize = sizing.minSize
		if f.MinSize.Width > 0 {
			f.minCacheSize.Width = f.MinSize.Width
		} else {
			f.minCacheSize.Width = sizing.minSize.Width
		}
		if f.MinSize.Height > 0 {
			f.minCacheSize.Height = f.MinSize.Height
		} else {
			f.minCacheSize.Height = sizing.minSize.Height
		}
	} else {
		f.cacheSize = sizing.prefSize
	}
	if hint.Width > 0 {
		f.cacheSize.Width = hint.Width
	}
	if f.MinSize.Width > 0 && f.cacheSize.Width < f.MinSize.Width {
		f.cacheSize.Width = f.MinSize.Width
	}
	if f.SizeHint.Width > 0 {
		f.cacheSize.Width = f.SizeHint.Width
	}
	if hint.Height > 0 {
		f.cacheSize.Height = hint.Height
	}
	if f.MinSize.Height > 0 && f.cacheSize.Height < f.MinSize.Height {
		f.cacheSize.Height = f.MinSize.Height
	}
	if f.SizeHint.Height > 0 {
		f.cacheSize.Height = f.SizeHint.Height
	}
}
