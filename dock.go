// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
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
	"strings"

	"github.com/richardwilkes/toolbox/xmath/geom32"
)

const (
	DockGripGap     = 1
	DockGripWidth   = 4
	DockGripHeight  = 2
	DockGripLength  = DockGripHeight*5 + DockGripGap*4
	DockDividerSize = DockGripWidth + 4
)

// Dock provides an area where Dockable panels can be displayed and rearranged.
type Dock struct {
	Panel
	layout             *DockLayout
	MaximizedContainer *DockContainer
	BackgroundColor    Ink
}

// NewDock creates a new, empty, dock.
func NewDock() *Dock {
	d := &Dock{
		layout: &DockLayout{Divider: -1},
	}
	d.Self = d
	d.SetLayout(d.layout)
	d.DrawCallback = d.DefaultDraw
	d.DrawOverCallback = d.DefaultDrawOver
	d.FocusChangeInHierarchyCallback = d.DefaultFocusChangeInHierarchy
	return d
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
				for target != nil {
					layouts = append(layouts, target)
					target = dl.parent
				}
				target = layouts[0]
				for _, child := range dl.nodes {
					if child != dc {
						layouts = append(layouts, nil)
						copy(layouts[2:], layouts[1:])
						layouts[1] = child
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

func (d *Dock) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	rect := d.ContentRect(true)
	canvas.DrawRect(rect, ChooseInk(d.BackgroundColor, BackgroundColor).Paint(canvas, rect, Fill))
}

func (d *Dock) DefaultDrawOver(canvas *Canvas, dirty geom32.Rect) {
	if d.MaximizedContainer == nil {
		d.drawDividers(canvas, d.layout, dirty)
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

func (d *Dock) drawHorizontalGripper(canvas *Canvas, node DockLayoutNode) {
	frame := node.FrameRect()
	x := frame.X - DockDividerSize + (DockDividerSize-DockGripWidth)/2
	y := frame.Y + (frame.Height-DockGripLength)/2
	paint := DividerColor.Paint(canvas, frame, Fill)
	for yy := y; yy < y+DockGripLength; yy += DockGripHeight + DockGripGap {
		canvas.DrawRect(geom32.NewRect(x, yy, DockGripWidth-1, DockGripHeight), paint)
	}
}

func (d *Dock) drawVerticalGripper(canvas *Canvas, node DockLayoutNode) {
	frame := node.FrameRect()
	x := frame.X + (frame.Width-DockGripLength)/2
	y := frame.Y - DockDividerSize + (DockDividerSize-DockGripWidth)/2
	paint := DividerColor.Paint(canvas, frame, Fill)
	for xx := x; xx < x+DockGripLength; xx += DockGripHeight + DockGripGap {
		canvas.DrawRect(geom32.NewRect(xx, y, DockGripHeight, DockGripWidth-1), paint)
	}
}

func (d *Dock) Maximize(dc *DockContainer) {
	if d.MaximizedContainer != nil {
		d.MaximizedContainer.header.adjustToRestoredState()
	}
	d.MaximizedContainer = dc
	d.MaximizedContainer.header.adjustToMaximizedState()
	d.MaximizedContainer.AcquireFocus()
	d.MarkForLayoutAndRedraw()
}

func (d *Dock) Restore() {
	if d.MaximizedContainer != nil {
		d.layout.forEachDockContainer(func(dc *DockContainer) { dc.Hidden = false })
		d.MaximizedContainer.header.adjustToRestoredState()
		d.MaximizedContainer = nil
		d.MarkForLayoutAndRedraw()
	}
}

func (d *Dock) DefaultFocusChangeInHierarchy(from, to *Panel) {
	d.MarkForRedraw()
}

func (d *Dock) DebugDump() {
	fmt.Println()
	fmt.Println("Dock Debug Dump")
	fmt.Print("---------------")
	dumpNode(0, d.layout)
	fmt.Println()
}

func dumpNode(depth int, node DockLayoutNode) {
	fmt.Println()
	fmt.Print(strings.Repeat(".", depth*2))
	switch n := node.(type) {
	case *DockLayout:
		fmt.Printf("Layout [x:%f y:%f w:%f h:%f]", n.frame.X, n.frame.Y, n.frame.Width, n.frame.Height)
		if n.Horizontal {
			fmt.Print(" Horizontal")
		} else {
			fmt.Print(" Vertical")
		}
		if n.Divider >= 0 {
			fmt.Printf(" Divider:%f", n.Divider)
		}
		for _, c := range n.nodes {
			if c != nil {
				dumpNode(depth+1, c)
			}
		}
	case *DockContainer:
		fmt.Printf("Container [x:%f y:%f w:%f h:%f]", n.frame.X, n.frame.Y, n.frame.Width, n.frame.Height)
		for i, d := range n.Dockables() {
			fmt.Println()
			fmt.Print(strings.Repeat(".", (depth+1)*2))
			fmt.Printf("Dockable %d [%s]", i+1, d.Title())
			if d == n.CurrentDockable() {
				fmt.Print(" (Current)")
			}
		}
	default:
		fmt.Print("<unknown type>")
	}
}