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
	"reflect"
	"strings"

	"github.com/richardwilkes/toolbox/xmath/geom32"
)

var _ Paneler = &Panel{}

// Paneler is used to convert widgets into the base Panel type.
type Paneler interface {
	AsPanel() *Panel
}

// Panel is the basic user interface element that interacts with the user. During construction, new objects must always
// set the Self field to the final object. Failure to do so may result in incorrect behavior.
type Panel struct {
	InputCallbacks
	Self                                interface{}
	DrawCallback                        func(gc *Canvas, rect geom32.Rect)
	DrawOverCallback                    func(gc *Canvas, rect geom32.Rect)
	UpdateCursorCallback                func(where geom32.Point) *Cursor
	UpdateTooltipCallback               func(where geom32.Point, suggestedAvoidInRoot geom32.Rect) geom32.Rect
	CanPerformCmdCallback               func(source interface{}, id int) bool
	PerformCmdCallback                  func(source interface{}, id int)
	FrameChangeCallback                 func()
	FrameChangeInChildHierarchyCallback func(panel *Panel)
	ScrollRectIntoViewCallback          func(rect geom32.Rect) bool
	ParentChangedCallback               func()
	FocusChangeInHierarchyCallback      func(from, to *Panel)
	// DataDragOverCallback is called when a data drag is over a potential drop target. Return true to stop further
	// handling or false to propagate up to parents.
	DataDragOverCallback func(where geom32.Point, data map[string]interface{}) bool
	// DataDragExitCallback is called when a previous call to DataDragOverCallback returned true and the data drag
	// leaves the component.
	DataDragExitCallback func()
	// DataDragDropCallback is called when a data drag is dropped and a previous call to DataDragOverCallback returned
	// true.
	DataDragDropCallback func(where geom32.Point, data map[string]interface{})
	Tooltip              *Panel
	parent               *Panel
	frame                geom32.Rect
	border               Border
	sizer                Sizer
	layout               Layout
	layoutData           interface{}
	children             []*Panel
	data                 map[string]interface{}
	scale                float32
	NeedsLayout          bool
	focusable            bool
	disabled             bool
	Hidden               bool
}

// NewPanel creates a new panel.
func NewPanel() *Panel {
	p := &Panel{}
	p.Self = p
	return p
}

// AsPanel returns this object as a panel.
func (p *Panel) AsPanel() *Panel {
	return p
}

// Is returns true if this panel is the other panel.
func (p *Panel) Is(other Paneler) bool {
	if p != nil && other != nil {
		p2 := other.AsPanel()
		return p2 != nil && p.Self == p2.Self
	}
	return false
}

func (p *Panel) String() string {
	name := reflect.Indirect(reflect.ValueOf(p.Self)).Type().String()
	if i := strings.LastIndex(name, "."); i != -1 {
		name = name[i+1:]
	}
	return name
}

// Children returns the direct descendents of this panel.
func (p *Panel) Children() []*Panel {
	return p.children
}

// IndexOfChild returns the index of the specified child, or -1 if the passed in panel is not a child of this panel.
func (p *Panel) IndexOfChild(child Paneler) int {
	for i, one := range p.children {
		if one.Is(child) {
			return i
		}
	}
	return -1
}

// AddChild adds child to this panel, removing it from any previous parent it may have had.
func (p *Panel) AddChild(child Paneler) {
	c := child.AsPanel()
	c.RemoveFromParent()
	p.children = append(p.children, c)
	c.parent = p
	p.NeedsLayout = true
	if c.ParentChangedCallback != nil {
		c.ParentChangedCallback()
	}
}

// AddChildAtIndex adds child to this panel at the index, removing it from any previous parent it may have had. Passing
// in a negative value for the index will add it to the end.
func (p *Panel) AddChildAtIndex(child Paneler, index int) {
	c := child.AsPanel()
	c.RemoveFromParent()
	if index < 0 || index >= len(p.children) {
		p.children = append(p.children, c)
	} else {
		p.children = append(p.children, nil)
		copy(p.children[index+1:], p.children[index:])
		p.children[index] = c
	}
	c.parent = p
	p.NeedsLayout = true
	if c.ParentChangedCallback != nil {
		c.ParentChangedCallback()
	}
}

// RemoveAllChildren removes all child panels from this panel.
func (p *Panel) RemoveAllChildren() {
	children := p.children
	for _, child := range children {
		child.parent = nil
	}
	p.children = nil
	p.NeedsLayout = true
	for _, child := range children {
		if child.ParentChangedCallback != nil {
			child.ParentChangedCallback()
		}
	}
}

// RemoveChild removes 'child' from this panel. If 'child' is not a direct descendent of this panel, nothing happens.
func (p *Panel) RemoveChild(child Paneler) {
	p.RemoveChildAtIndex(p.IndexOfChild(child))
}

// RemoveChildAtIndex removes the child panel at 'index' from this panel. If 'index' is out of range, nothing happens.
func (p *Panel) RemoveChildAtIndex(index int) {
	if index >= 0 && index < len(p.children) {
		child := p.children[index]
		child.parent = nil
		copy(p.children[index:], p.children[index+1:])
		p.children[len(p.children)-1] = nil
		p.children = p.children[:len(p.children)-1]
		p.NeedsLayout = true
		if child.ParentChangedCallback != nil {
			child.ParentChangedCallback()
		}
	}
}

// RemoveFromParent removes this panel from its parent, if any.
func (p *Panel) RemoveFromParent() {
	if p.parent != nil {
		p.parent.RemoveChild(p)
	}
}

// Parent returns the parent panel, if any.
func (p *Panel) Parent() *Panel {
	return p.parent
}

// Window returns the containing window, if any.
func (p *Panel) Window() *Window {
	var prev *Panel
	panel := p
	for {
		if panel == nil {
			if prev != nil {
				if root, ok := prev.Self.(*rootPanel); ok {
					return root.window
				}
			}
			return nil
		}
		prev = panel
		panel = panel.parent
	}
}

// Scale returns the scale that has been applied to this panel. This will be automatically applied, transforming the
// graphics and mouse events.
func (p *Panel) Scale() float32 {
	if p.scale <= 0 { // This happens if not explicitly set. 0 or less isn't valid, so make it 1
		p.scale = 1
	}
	return p.scale
}

// SetScale sets the scale for this panel and the panels in the hierarchy below it.
func (p *Panel) SetScale(scale float32) {
	p.scale = scale
}

// FrameRect returns the location and size of the panel in its parent's coordinate system.
func (p *Panel) FrameRect() geom32.Rect {
	scale := p.Scale()
	r := p.frame
	r.Width *= scale
	r.Height *= scale
	return r
}

// SetFrameRect sets the location and size of the panel in its parent's coordinate system.
func (p *Panel) SetFrameRect(rect geom32.Rect) {
	scale := p.Scale()
	rect.Width /= scale
	rect.Height /= scale
	moved := p.frame.X != rect.X || p.frame.Y != rect.Y
	resized := p.frame.Width != rect.Width || p.frame.Height != rect.Height
	if moved || resized {
		if moved {
			p.frame.Point = rect.Point
		}
		if resized {
			p.frame.Size = rect.Size
			p.NeedsLayout = true
		}
		if p.FrameChangeCallback != nil {
			p.FrameChangeCallback()
		}
		parent := p.parent
		for parent != nil {
			if parent.FrameChangeInChildHierarchyCallback != nil {
				parent.FrameChangeInChildHierarchyCallback(p)
			}
			parent = parent.parent
		}
		p.MarkForRedraw()
	}
}

// ContentRect returns the location and size of the panel in local coordinates.
func (p *Panel) ContentRect(includeBorder bool) geom32.Rect {
	rect := p.frame.CopyAndZeroLocation()
	if !includeBorder && p.border != nil {
		rect.Inset(p.border.Insets())
	}
	return rect
}

// Border returns the border for this panel, if any.
func (p *Panel) Border() Border {
	return p.border
}

// SetBorder sets the border for this panel. May be nil.
func (p *Panel) SetBorder(b Border) {
	if p.border != b {
		p.border = b
		p.MarkForLayoutAndRedraw()
	}
}

// Sizer returns the sizer for this panel, if any.
func (p *Panel) Sizer() Sizer {
	return p.sizer
}

// SetSizer sets the sizer for this panel. May be nil.
func (p *Panel) SetSizer(sizer Sizer) {
	p.sizer = sizer
	p.NeedsLayout = true
}

// Sizes returns the minimum, preferred, and maximum sizes the panel wishes to be. It does this by first asking the
// panel's layout. If no layout is present, then the panel's sizer is asked. If no sizer is present, then it finally
// uses a default set of sizes that are used for all panels.
func (p *Panel) Sizes(hint geom32.Size) (min, pref, max geom32.Size) {
	scale := p.Scale()
	hint.Width /= scale
	hint.Height /= scale
	switch {
	case p.layout != nil:
		min, pref, max = p.layout.LayoutSizes(p, hint)
	case p.sizer != nil:
		min, pref, max = p.sizer(hint)
	default:
		return min, pref, geom32.Size{Width: DefaultMaxSize, Height: DefaultMaxSize}
	}
	min.Width *= scale
	min.Height *= scale
	pref.Width *= scale
	pref.Height *= scale
	max.Width *= scale
	max.Height *= scale
	return
}

// Layout returns the Layout for this panel, if any.
func (p *Panel) Layout() Layout {
	return p.layout
}

// SetLayout sets the Layout for this panel. May be nil.
func (p *Panel) SetLayout(lay Layout) {
	p.layout = lay
	p.NeedsLayout = true
}

// ValidateLayout performs any layout that needs to be run by this panel or its children.
func (p *Panel) ValidateLayout() {
	if p.NeedsLayout {
		if p.layout != nil {
			p.layout.PerformLayout(p)
			p.MarkForRedraw()
		}
		p.NeedsLayout = false
	}
	for _, child := range p.children {
		child.ValidateLayout()
	}
}

// LayoutData returns the layout data, if any, associated with this panel.
func (p *Panel) LayoutData() interface{} {
	return p.layoutData
}

// SetLayoutData sets layout data on this panel. May be nil.
func (p *Panel) SetLayoutData(data interface{}) {
	p.layoutData = data
	p.NeedsLayout = true
}

// MarkForLayoutRecursively marks this panel and all of its descendents as needing to be laid out.
func (p *Panel) MarkForLayoutRecursively() {
	p.NeedsLayout = true
	for _, child := range p.Children() {
		child.MarkForLayoutRecursively()
	}
}

// MarkForLayoutAndRedraw marks this panel as needing to be laid out as well as redrawn at the next update.
func (p *Panel) MarkForLayoutAndRedraw() {
	p.NeedsLayout = true
	p.MarkForRedraw()
}

// MarkForRedraw finds the parent window and marks it for drawing at the next update. Note that currently I have found
// no way to get glfw to both only redraw a subset of the window AND retain the previous contents of that window, such
// that incremental updates can be done. So... we just redraw everything in the window every time.
func (p *Panel) MarkForRedraw() {
	if w := p.Window(); w != nil {
		w.MarkForRedraw()
	}
}

// FlushDrawing is a convenience for calling the parent window's (if any) FlushDrawing() method.
func (p *Panel) FlushDrawing() {
	if w := p.Window(); w != nil {
		w.FlushDrawing()
	}
}

// Draw is called by its owning window when a panel needs to be drawn. The canvas has already had its clip set to rect.
func (p *Panel) Draw(gc *Canvas, rect geom32.Rect) {
	if p.Hidden {
		return
	}
	rect.Intersect(p.frame.CopyAndZeroLocation())
	if !rect.IsEmpty() {
		gc.Save()
		scale := p.Scale()
		gc.Scale(scale, scale)
		gc.ClipRect(rect, IntersectClipOp, false)
		if p.DrawCallback != nil {
			gc.Save()
			p.DrawCallback(gc, rect)
			gc.Restore()
		}
		// Drawn from last to first, to get correct ordering in case of overlap
		for i := len(p.children) - 1; i >= 0; i-- {
			if child := p.children[i]; !child.Hidden {
				adjusted := rect
				childFrame := child.FrameRect()
				adjusted.Intersect(childFrame)
				if !adjusted.IsEmpty() {
					gc.Save()
					gc.Translate(childFrame.X, childFrame.Y)
					adjusted.Point.Subtract(childFrame.Point)
					scale = child.Scale()
					adjusted.X /= scale
					adjusted.Y /= scale
					adjusted.Width /= scale
					adjusted.Height /= scale
					child.Draw(gc, adjusted)
					gc.Restore()
				}
			}
		}
		if p.border != nil {
			gc.Save()
			p.border.Draw(gc, p.ContentRect(true))
			gc.Restore()
		}
		if p.DrawOverCallback != nil {
			p.DrawOverCallback(gc, rect)
		}
		gc.Restore()
	}
}

// Enabled returns true if this panel is currently enabled and can receive events.
func (p *Panel) Enabled() bool {
	return !p.disabled && !p.Hidden
}

// SetEnabled sets this panel's enabled state.
func (p *Panel) SetEnabled(enabled bool) {
	if p.disabled == enabled {
		p.disabled = !enabled
		p.MarkForRedraw()
	}
}

// Focusable returns true if this panel can have the keyboard focus.
func (p *Panel) Focusable() bool {
	return p.focusable && p.Enabled()
}

// SetFocusable sets whether this panel can have the keyboard focus.
func (p *Panel) SetFocusable(focusable bool) {
	if p.focusable != focusable {
		p.focusable = focusable
	}
}

// Focused returns true if this panel has the keyboard focus.
func (p *Panel) Focused() bool {
	if wnd := p.Window(); wnd != nil {
		return wnd.Focused() && p.Is(wnd.Focus())
	}
	return false
}

// RequestFocus attempts to make this panel the keyboard focus.
func (p *Panel) RequestFocus() {
	if wnd := p.Window(); wnd != nil {
		wnd.SetFocus(p)
	}
}

// PanelAt returns the leaf-most child panel containing the point, or this panel if no child is found.
func (p *Panel) PanelAt(pt geom32.Point) *Panel {
	for _, child := range p.children {
		if !child.Hidden {
			if r := child.FrameRect(); r.ContainsPoint(pt) {
				pt.Subtract(r.Point)
				scale := child.Scale()
				pt.X /= scale
				pt.Y /= scale
				return child.PanelAt(pt)
			}
		}
	}
	return p
}

// PointToRoot converts panel-local coordinates into root coordinates, which when rooted within a window, will be
// window-local coordinates.
func (p *Panel) PointToRoot(pt geom32.Point) geom32.Point {
	one := p
	for one != nil {
		scale := one.Scale()
		pt.X *= scale
		pt.Y *= scale
		pt.Add(one.frame.Point)
		one = one.parent
	}
	return pt
}

// PointFromRoot converts root coordinates (i.e. window-local, when rooted within a window) into panel-local
// coordinates.
func (p *Panel) PointFromRoot(pt geom32.Point) geom32.Point {
	list := make([]*Panel, 0, 32)
	one := p
	for one != nil {
		list = append(list, one)
		one = one.parent
	}
	for i := len(list) - 1; i >= 0; i-- {
		one = list[i]
		pt.Subtract(one.frame.Point)
		scale := one.Scale()
		pt.X /= scale
		pt.Y /= scale
	}
	return pt
}

// RectToRoot converts panel-local coordinates into root coordinates, which when rooted within a window, will be
// window-local coordinates.
func (p *Panel) RectToRoot(rect geom32.Rect) geom32.Rect {
	pt := p.PointToRoot(rect.BottomRight())
	rect.Point = p.PointToRoot(rect.Point)
	rect.Width = pt.X - rect.X
	rect.Height = pt.Y - rect.Y
	return rect
}

// RectFromRoot converts root coordinates (i.e. window-local, when rooted within a window) into panel-local coordinates.
func (p *Panel) RectFromRoot(rect geom32.Rect) geom32.Rect {
	pt := p.PointFromRoot(rect.BottomRight())
	rect.Point = p.PointFromRoot(rect.Point)
	rect.Width = pt.X - rect.X
	rect.Height = pt.Y - rect.Y
	return rect
}

// ScrollIntoView attempts to scroll this panel into the current view if it is not already there, using ScrollAreas in
// this Panel's hierarchy.
func (p *Panel) ScrollIntoView() {
	p.ScrollRectIntoView(p.ContentRect(true))
}

// ScrollRectIntoView attempts to scroll the rect (in coordinates local to this Panel) into the current view if it is
// not already there, using scroll areas in this Panel's hierarchy.
func (p *Panel) ScrollRectIntoView(rect geom32.Rect) {
	look := p
	for look != nil {
		if look.ScrollRectIntoViewCallback != nil {
			if look.ScrollRectIntoViewCallback(rect) {
				return
			}
		}
		scale := look.Scale()
		rect.X *= scale
		rect.Y *= scale
		rect.Point.Add(look.frame.Point)
		look = look.parent
	}
}

// ClientData returns a map of client data for this Panel.
func (p *Panel) ClientData() map[string]interface{} {
	if p.data == nil {
		p.data = make(map[string]interface{})
	}
	return p.data
}

// UpdateCursorNow causes the cursor to be updated as if the mouse had moved.
func (p *Panel) UpdateCursorNow() {
	if wnd := p.Window(); wnd != nil {
		wnd.UpdateCursorNow()
	}
}

// IsDragGesture returns true if a gesture to start a drag operation was made.
func (p *Panel) IsDragGesture(where geom32.Point) bool {
	if w := p.Window(); w != nil {
		return w.IsDragGesture(p.PointToRoot(where))
	}
	return false
}

// StartDataDrag starts a data drag operation.
func (p *Panel) StartDataDrag(data *DragData) {
	if w := p.Window(); w != nil {
		w.StartDataDrag(data)
	}
}
