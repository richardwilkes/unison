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
	"fmt"

	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

// DefaultDockTheme holds the default DockTheme values for Docks. Modifying this data will not alter existing Docks, but
// will alter any Docks created in the future.
var DefaultDockTheme = DockTheme{
	BackgroundInk: BackgroundColor,
	GripInk:       DividerColor,
	DropAreaInk:   DropAreaColor,
	GripCount:     5,
	GripGap:       1,
	GripWidth:     4,
	GripHeight:    2,
	GripMargin:    2,
}

// DockTheme holds theming data for a Dock.
type DockTheme struct {
	BackgroundInk Ink
	GripInk       Ink
	DropAreaInk   Ink
	GripCount     int
	GripGap       float32
	GripWidth     float32
	GripHeight    float32
	GripMargin    float32
}

// Dock provides an area where Dockable panels can be displayed and rearranged.
type Dock struct {
	Panel
	DockTheme
	layout                     *DockLayout
	MaximizedContainer         *DockContainer
	DragKey                    string
	dragDockable               Dockable
	dragOverNode               DockLayoutNode
	dividerDragLayout          *DockLayout
	dividerDragInitialPosition float32
	dividerDragEventPosition   float32
	dragSide                   Side
	dividerDragIsValid         bool
}

var dockCounter = 0

// NewDock creates a new, empty, dock.
func NewDock() *Dock {
	dockCounter++
	d := &Dock{
		DockTheme: DefaultDockTheme,
		DragKey:   fmt.Sprintf("dock-%d", dockCounter),
	}
	d.Self = d
	d.layout = &DockLayout{
		dock:    d,
		divider: -1,
	}
	d.SetLayout(d.layout)
	d.DrawCallback = d.DefaultDraw
	d.DrawOverCallback = d.DefaultDrawOver
	d.FocusChangeInHierarchyCallback = d.DefaultFocusChangeInHierarchy
	d.UpdateCursorCallback = d.DefaultUpdateCursor
	d.MouseDownCallback = d.DefaultMouseDown
	d.MouseDragCallback = d.DefaultMouseDrag
	d.MouseUpCallback = d.DefaultMouseUp
	d.DataDragOverCallback = d.DefaultDataDragOver
	d.DataDragExitCallback = d.DefaultDataDragExit
	d.DataDragDropCallback = d.DefaultDataDrop
	return d
}

// RootDockLayout returns the root DockLayout.
func (d *Dock) RootDockLayout() *DockLayout {
	return d.layout
}

// DockTo a Dockable within this Dock. If the Dockable already exists in this Dock, it will be moved to the new location. nil may be passed in for the target, in which case the top-most layout is used.
func (d *Dock) DockTo(dockable Dockable, target DockLayoutNode, side Side) {
	if target == nil {
		target = d.layout
	}
	if d.layout.Contains(target) {
		dc := DockContainerFor(dockable)
		if dc == target {
			if len(dc.Dockables()) == 1 {
				// It's already where it needs to be
				return
			}
		}
		if dc != nil {
			// Remove it from it's old position
			var layouts []DockLayoutNode
			dl, ok2 := target.(*DockLayout)
			if ok2 {
				for dl != nil {
					layouts = append(layouts, dl)
					dl = dl.parent
				}
				target = layouts[0]
				if dl, ok2 = target.(*DockLayout); ok2 {
					for _, child := range dl.nodes {
						if child != dc {
							layouts = append(layouts, nil)
							copy(layouts[2:], layouts[1:])
							layouts[1] = child
						}
					}
				}
			}
			dc.Close(dockable)
			if ok2 {
				i := 1
				for !d.layout.Contains(target) {
					if i >= len(layouts) {
						target = d.layout
						break
					}
					target = layouts[i]
					i++
				}
			}
		}
		dc = NewDockContainer(d, dockable)
		d.layout.DockTo(dc, target, side)
		d.AddChild(dc)
		d.MarkForLayoutAndRedraw()
		dc.SetCurrentDockable(dockable)
	}
}

// DefaultDraw fills in the background.
func (d *Dock) DefaultDraw(gc *Canvas, dirty geom32.Rect) {
	rect := d.ContentRect(true)
	gc.DrawRect(rect, d.BackgroundInk.Paint(gc, rect, Fill))
}

// DefaultDrawOver draws the dividers and any drag markers.
func (d *Dock) DefaultDrawOver(gc *Canvas, dirty geom32.Rect) {
	if d.MaximizedContainer == nil {
		d.drawDividers(gc, d.layout, dirty)
	}
	if d.dragDockable != nil && d.dragOverNode != nil {
		r := d.dragOverNode.FrameRect()
		switch d.dragSide {
		case TopSide:
			r.Height = mathf32.Max(r.Height/2, 1)
		case LeftSide:
			r.Width = mathf32.Max(r.Width/2, 1)
		case BottomSide:
			half := mathf32.Max(r.Height/2, 1)
			r.Y += r.Height - half
			r.Height = half
		default:
			half := mathf32.Max(r.Width/2, 1)
			r.X += r.Width - half
			r.Width = half
		}
		paint := d.DropAreaInk.Paint(gc, r, Fill)
		paint.SetColorFilter(NewAlphaFilter(0.25))
		gc.DrawRect(r, paint)
		r.InsetUniform(1)
		p := d.DropAreaInk.Paint(gc, r, Stroke)
		p.SetStrokeWidth(2)
		gc.DrawRect(r, p)
	}
}

func (d *Dock) drawDividers(canvas *Canvas, layout *DockLayout, clip geom32.Rect) {
	frame := layout.FrameRect()
	frame.Inset(geom32.NewUniformInsets(1))
	if clip.Intersects(frame) {
		if layout.Full() {
			if layout.Horizontal {
				d.drawHorizontalGripper(canvas, layout.nodes[1])
			} else {
				d.drawVerticalGripper(canvas, layout.nodes[1])
			}
		}
		for _, node := range layout.nodes {
			d.drawDockLayoutNode(canvas, node, clip)
		}
	}
}

func (d *Dock) drawDockLayoutNode(canvas *Canvas, node DockLayoutNode, clip geom32.Rect) {
	if dl, ok := node.(*DockLayout); ok {
		d.drawDividers(canvas, dl, clip)
	}
}

// DockGripLength returns the length (running along the divider) of a divider's grip area.
func (d *Dock) DockGripLength() float32 {
	return (d.GripHeight+d.GripGap)*float32(d.GripCount) - d.GripGap
}

// DockDividerSize returns the size (running across the divider) of a divider.
func (d *Dock) DockDividerSize() float32 {
	return d.GripWidth + d.GripMargin*2
}

func (d *Dock) drawHorizontalGripper(canvas *Canvas, node DockLayoutNode) {
	gripLength := d.DockGripLength()
	dividerSize := d.DockDividerSize()
	frame := node.FrameRect()
	x := frame.X - dividerSize + (dividerSize-d.GripWidth)/2
	y := frame.Y + (frame.Height-gripLength)/2
	paint := d.GripInk.Paint(canvas, frame, Fill)
	for yy := y; yy < y+gripLength; yy += d.GripHeight + d.GripGap {
		canvas.DrawRect(geom32.NewRect(x, yy, d.GripWidth, d.GripHeight), paint)
	}
	x = frame.X - dividerSize + 0.5
	canvas.DrawLine(x, frame.Y, x, frame.Bottom(), paint)
	x = frame.X - 0.5
	canvas.DrawLine(x, frame.Y, x, frame.Bottom(), paint)
}

func (d *Dock) drawVerticalGripper(canvas *Canvas, node DockLayoutNode) {
	gripLength := d.DockGripLength()
	dividerSize := d.DockDividerSize()
	frame := node.FrameRect()
	x := frame.X + (frame.Width-gripLength)/2
	y := frame.Y - dividerSize + (dividerSize-d.GripWidth)/2
	paint := d.GripInk.Paint(canvas, frame, Fill)
	for xx := x; xx < x+gripLength; xx += d.GripHeight + d.GripGap {
		canvas.DrawRect(geom32.NewRect(xx, y, d.GripHeight, d.GripWidth), paint)
	}
	y = frame.Y - dividerSize + 0.5
	canvas.DrawLine(frame.X, y, frame.Right(), y, paint)
	y = frame.Y - 0.5
	canvas.DrawLine(frame.X, y, frame.Right(), y, paint)
}

// Maximize the current Dockable.
func (d *Dock) Maximize(dc *DockContainer) {
	if d.MaximizedContainer != nil {
		d.MaximizedContainer.header.adjustToRestoredState()
	}
	d.MaximizedContainer = dc
	d.MaximizedContainer.header.adjustToMaximizedState()
	d.MaximizedContainer.AcquireFocus()
	d.MarkForLayoutAndRedraw()
}

// Restore the current Dockable to its non-maximized state.
func (d *Dock) Restore() {
	if d.MaximizedContainer != nil {
		d.layout.ForEachDockContainer(func(dc *DockContainer) bool {
			dc.Hidden = false
			return false
		})
		d.MaximizedContainer.header.adjustToRestoredState()
		d.MaximizedContainer = nil
		d.MarkForLayoutAndRedraw()
	}
}

// DefaultFocusChangeInHierarchy marks the dock for redraw whenever the focus changes within it so that the tabs get the
// correct highlight state.
func (d *Dock) DefaultFocusChangeInHierarchy(from, to *Panel) {
	d.MarkForRedraw()
}

// DefaultUpdateCursor adjusts the cursor for any dividers it may be over.
func (d *Dock) DefaultUpdateCursor(where geom32.Point) *Cursor {
	over := d.overNode(d.layout, where)
	if dl, ok := over.(*DockLayout); ok {
		if dl.Horizontal {
			return ResizeHorizontalCursor()
		}
		return ResizeVerticalCursor()
	}
	return ArrowCursor()
}

func (d *Dock) overNode(node DockLayoutNode, where geom32.Point) DockLayoutNode {
	if dockLayoutNodeContains(node, where) {
		switch n := node.(type) {
		case *DockLayout:
			for _, c := range n.nodes {
				if dockLayoutNodeContains(c, where) {
					return d.overNode(c, where)
				}
			}
			if n.Full() {
				return node
			}
		case *DockContainer:
			return node
		}
	}
	return nil
}

func dockLayoutNodeContains(node DockLayoutNode, where geom32.Point) bool {
	if node != nil {
		return node.FrameRect().ContainsPoint(where)
	}
	return false
}

// DefaultMouseDown provides the default mouse down handling.
func (d *Dock) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	over := d.overNode(d.layout, where)
	if dl, ok := over.(*DockLayout); ok {
		d.dividerDragLayout = dl
		d.dividerDragInitialPosition = dl.DividerPosition()
		if dl.Horizontal {
			d.dividerDragEventPosition = where.X
		} else {
			d.dividerDragEventPosition = where.Y
		}
		d.dividerDragIsValid = false
		return true
	}
	return false
}

// DefaultMouseDrag provides the default mouse drag handling.
func (d *Dock) DefaultMouseDrag(where geom32.Point, button int, mod Modifiers) bool {
	d.dragDivider(where)
	return true
}

func (d *Dock) dragDivider(where geom32.Point) {
	if d.dividerDragLayout != nil {
		if !d.dividerDragIsValid {
			d.dividerDragIsValid = d.IsDragGesture(where)
		}
		if d.dividerDragIsValid {
			pos := d.dividerDragEventPosition
			if d.dividerDragLayout.Horizontal {
				pos -= where.X
			} else {
				pos -= where.Y
			}
			d.dividerDragLayout.SetDividerPosition(mathf32.Max(d.dividerDragInitialPosition-pos, 0))
		}
	}
}

// DefaultMouseUp provides the default mouse up handling.
func (d *Dock) DefaultMouseUp(where geom32.Point, button int, mod Modifiers) bool {
	if d.dividerDragLayout != nil {
		if d.dividerDragIsValid {
			d.dragDivider(where)
		}
		d.dividerDragLayout = nil
	}
	return true
}

// DefaultDataDragOver provides the default data drag over handling.
func (d *Dock) DefaultDataDragOver(where geom32.Point, data map[string]interface{}) bool {
	if d.MaximizedContainer != nil {
		return false
	}
	d.updateDragDockable(where, data)
	return d.dragDockable != nil
}

// DockableFromDragData attempts to extract a Dockable from the given key in the data.
func DockableFromDragData(key string, data map[string]interface{}) Dockable {
	if keyData, ok := data[key]; ok {
		if dockable, ok2 := keyData.(Dockable); ok2 {
			return dockable
		}
	}
	return nil
}

func (d *Dock) updateDragDockable(where geom32.Point, data map[string]interface{}) {
	d.dragDockable = nil
	d.dragOverNode = nil
	if dockable := DockableFromDragData(d.DragKey, data); dockable != nil {
		if d.dragOverNode = d.overNode(d.layout, where); d.dragOverNode != nil {
			var extent float32
			r := d.dragOverNode.FrameRect()
			if where.X < r.CenterX() {
				d.dragSide = LeftSide
				extent = where.X - r.X
			} else {
				d.dragSide = RightSide
				extent = r.Width - (where.X - r.X)
			}
			if where.Y < r.CenterY() {
				if extent > where.Y-r.Y {
					d.dragSide = TopSide
				}
			} else if extent > r.Height-(where.Y-r.Y) {
				d.dragSide = BottomSide
			}
			d.dragDockable = dockable
		}
	}
}

// DefaultDataDragExit provides the default data drag exit handling.
func (d *Dock) DefaultDataDragExit() {
	d.dragDockable = nil
	d.dragOverNode = nil
}

// DefaultDataDrop provides the default data drop handling.
func (d *Dock) DefaultDataDrop(where geom32.Point, data map[string]interface{}) {
	d.updateDragDockable(where, data)
	if d.dragDockable != nil && d.dragOverNode != nil {
		d.DockTo(d.dragDockable, d.dragOverNode, d.dragSide)
	}
	d.dragDockable = nil
	d.dragOverNode = nil
}

// NextDockableFor returns the logical next Dockable in the Dock given the one passed in as a starting point. The
// returned value may be the same as the starting point if there are no other Dockables. Passing in nil will result in
// the first Dockable found being returned.
func (d *Dock) NextDockableFor(dockable Dockable) Dockable {
	var result Dockable
	next := false
	d.layout.ForEachDockContainer(func(container *DockContainer) bool {
		for _, one := range container.Dockables() {
			if next {
				result = one
				return true
			}
			if one == dockable {
				next = true
			}
		}
		return false
	})
	if result == nil {
		d.layout.ForEachDockContainer(func(container *DockContainer) bool {
			list := container.Dockables()
			if len(list) > 0 {
				result = list[0]
				return true
			}
			return false
		})
	}
	return result
}

// PreviousDockableFor returns the logical previous Dockable in the Dock given the one passed in as a starting point. The
// returned value may be the same as the starting point if there are no other Dockables. Passing in nil will result in
// the last Dockable found being returned.
func (d *Dock) PreviousDockableFor(dockable Dockable) Dockable {
	var previous, result Dockable
	d.layout.ForEachDockContainer(func(container *DockContainer) bool {
		for _, one := range container.Dockables() {
			if one == dockable {
				result = previous
				return true
			}
			previous = one
		}
		return false
	})
	if result == nil {
		d.layout.ForEachDockContainer(func(container *DockContainer) bool {
			list := container.Dockables()
			if len(list) > 0 {
				result = list[len(list)-1]
			}
			return false
		})
	}
	return result
}
