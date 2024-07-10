// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
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
	"slices"
	"strings"

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/unison/enums/pathop"
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
	Self                                any
	layoutData                          any
	layout                              Layout
	sizer                               Sizer
	border                              Border
	DrawCallback                        func(gc *Canvas, rect Rect)
	DrawOverCallback                    func(gc *Canvas, rect Rect)
	UpdateCursorCallback                func(where Point) *Cursor
	UpdateTooltipCallback               func(where Point, suggestedAvoidInRoot Rect) Rect
	FrameChangeCallback                 func()
	FrameChangeInChildHierarchyCallback func(panel *Panel)
	ScrollRectIntoViewCallback          func(rect Rect) bool
	ParentChangedCallback               func()
	FocusChangeInHierarchyCallback      func(from, to *Panel)
	// DataDragOverCallback is called when a data drag is over a potential drop target. Return true to stop further
	// handling or false to propagate up to parents.
	DataDragOverCallback func(where Point, data map[string]any) bool
	// DataDragExitCallback is called when a previous call to DataDragOverCallback returned true and the data drag
	// leaves the component.
	DataDragExitCallback func()
	// DataDragDropCallback is called when a data drag is dropped and a previous call to DataDragOverCallback returned
	// true.
	DataDragDropCallback func(where Point, data map[string]any)
	Tooltip              *Panel
	parent               *Panel
	canPerformMap        map[int]func(any) bool
	performMap           map[int]func(any)
	data                 map[string]any
	RefKey               string
	children             []*Panel
	frame                Rect
	scale                float32
	NeedsLayout          bool
	focusable            bool
	disabled             bool
	Hidden               bool
	TooltipImmediate     bool
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
	if p != nil && !toolbox.IsNil(other) {
		p2 := other.AsPanel()
		return p2 != nil && p.Self == p2.Self
	}
	return false
}

// FindRefKey looks for refKey starting with this panel and then descending into its children recursively, returning the
// first match or nil if none is found.
func (p *Panel) FindRefKey(refKey string) *Panel {
	if p.RefKey == refKey {
		return p
	}
	for _, child := range p.children {
		if found := child.FindRefKey(refKey); found != nil {
			return found
		}
	}
	return nil
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
	if toolbox.IsNil(child) {
		return
	}
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
	if toolbox.IsNil(child) {
		return
	}
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
		p.children = slices.Delete(p.children, index, index+1)
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
	if p == nil {
		return nil
	}
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
func (p *Panel) FrameRect() Rect {
	scale := p.Scale()
	r := p.frame
	r.Width *= scale
	r.Height *= scale
	return r
}

// SetFrameRect sets the location and size of the panel in its parent's coordinate system.
func (p *Panel) SetFrameRect(rect Rect) {
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
func (p *Panel) ContentRect(includeBorder bool) Rect {
	r := Rect{Size: p.frame.Size}
	if !includeBorder && p.border != nil {
		r = r.Inset(p.border.Insets())
	}
	return r
}

// Border returns the border for this panel, if any.
func (p *Panel) Border() Border {
	return p.border
}

// SetBorder sets the border for this panel. May be nil.
func (p *Panel) SetBorder(b Border) {
	if p.border != b {
		if toolbox.IsNil(b) {
			p.border = nil
		} else {
			p.border = b
		}
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
func (p *Panel) Sizes(hint Size) (minSize, prefSize, maxSize Size) {
	scale := p.Scale()
	hint.Width /= scale
	hint.Height /= scale
	switch {
	case p.layout != nil:
		minSize, prefSize, maxSize = p.layout.LayoutSizes(p, hint)
	case p.sizer != nil:
		minSize, prefSize, maxSize = p.sizer(hint)
	default:
		return minSize, prefSize, Size{Width: DefaultMaxSize, Height: DefaultMaxSize}
	}
	minSize.Width *= scale
	minSize.Height *= scale
	prefSize.Width *= scale
	prefSize.Height *= scale
	maxSize.Width *= scale
	maxSize.Height *= scale
	return
}

// Pack resizes the panel to its preferred size.
func (p *Panel) Pack() {
	_, pref, _ := p.Sizes(Size{})
	p.SetFrameRect(Rect{Point: p.frame.Point, Size: pref})
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
func (p *Panel) LayoutData() any {
	return p.layoutData
}

// SetLayoutData sets layout data on this panel. May be nil.
func (p *Panel) SetLayoutData(data any) {
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

// MarkForLayoutRecursivelyUpward marks this panel and all of its parents as needing to be laid out.
func (p *Panel) MarkForLayoutRecursivelyUpward() {
	one := p
	for one != nil {
		one.NeedsLayout = true
		one = one.Parent()
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
func (p *Panel) Draw(gc *Canvas, rect Rect) {
	if p.Hidden {
		return
	}
	rect = rect.Intersect(Rect{Size: p.frame.Size})
	if !rect.Empty() {
		gc.Save()
		scale := p.Scale()
		gc.Scale(scale, scale)
		gc.ClipRect(rect, pathop.Intersect, false)
		if p.DrawCallback != nil {
			gc.Save()
			p.DrawCallback(gc, rect)
			gc.Restore()
		}
		// Drawn from last to first, to get correct ordering in case of overlap
		for i := len(p.children) - 1; i >= 0; i-- {
			if child := p.children[i]; !child.Hidden {
				childFrame := child.FrameRect()
				if adjusted := rect.Intersect(childFrame); !adjusted.Empty() {
					gc.Save()
					gc.Translate(childFrame.X, childFrame.Y)
					scale = child.Scale()
					adjusted.Point = adjusted.Point.Sub(childFrame.Point).Div(scale)
					adjusted.Size = adjusted.Size.Div(scale)
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

// FirstFocusableChild returns the first focusable child or nil.
func (p *Panel) FirstFocusableChild() *Panel {
	for _, child := range p.children {
		if child.Focusable() {
			return child
		}
		if found := child.FirstFocusableChild(); found != nil {
			return found
		}
	}
	return nil
}

// LastFocusableChild returns the last focusable child or nil.
func (p *Panel) LastFocusableChild() *Panel {
	for i := len(p.children) - 1; i >= 0; i-- {
		child := p.children[i]
		if child.Focusable() {
			return child
		}
		if found := child.LastFocusableChild(); found != nil {
			return found
		}
	}
	return nil
}

// PanelAt returns the leaf-most child panel containing the point, or this panel if no child is found.
func (p *Panel) PanelAt(pt Point) *Panel {
	for _, child := range p.children {
		if !child.Hidden {
			if r := child.FrameRect(); pt.In(r) {
				scale := child.Scale()
				return child.PanelAt(pt.Sub(r.Point).Div(scale))
			}
		}
	}
	return p
}

// PointToRoot converts panel-local coordinates into root coordinates, which when rooted within a window, will be
// window-local coordinates.
func (p *Panel) PointToRoot(pt Point) Point {
	panel := p
	for panel != nil {
		pt = pt.Mul(panel.Scale()).Add(panel.frame.Point)
		panel = panel.parent
	}
	return pt
}

// PointFromRoot converts root coordinates (i.e. window-local, when rooted within a window) into panel-local
// coordinates.
func (p *Panel) PointFromRoot(pt Point) Point {
	list := make([]*Panel, 0, 32)
	panel := p
	for panel != nil {
		list = append(list, panel)
		panel = panel.parent
	}
	for i := len(list) - 1; i >= 0; i-- {
		panel = list[i]
		pt = pt.Sub(panel.frame.Point).Div(panel.Scale())
	}
	return pt
}

// PointTo converts panel-local coordinates into another panel's coordinates.
func (p *Panel) PointTo(pt Point, target *Panel) Point {
	return target.PointFromRoot(p.PointToRoot(pt))
}

// RectToRoot converts panel-local coordinates into root coordinates, which when rooted within a window, will be
// window-local coordinates.
func (p *Panel) RectToRoot(rect Rect) Rect {
	pt := p.PointToRoot(rect.BottomRight())
	rect.Point = p.PointToRoot(rect.Point)
	rect.Width = pt.X - rect.X
	rect.Height = pt.Y - rect.Y
	return rect
}

// RectFromRoot converts root coordinates (i.e. window-local, when rooted within a window) into panel-local coordinates.
func (p *Panel) RectFromRoot(rect Rect) Rect {
	pt := p.PointFromRoot(rect.BottomRight())
	rect.Point = p.PointFromRoot(rect.Point)
	rect.Width = pt.X - rect.X
	rect.Height = pt.Y - rect.Y
	return rect
}

// RectTo converts panel-local coordinates into another panel's coordinates.
func (p *Panel) RectTo(rect Rect, target *Panel) Rect {
	return target.RectFromRoot(p.RectToRoot(rect))
}

// ScrollIntoView attempts to scroll this panel into the current view if it is not already there, using ScrollAreas in
// this Panel's hierarchy.
func (p *Panel) ScrollIntoView() {
	p.ScrollRectIntoView(p.ContentRect(true))
}

// ScrollRectIntoView attempts to scroll the rect (in coordinates local to this Panel) into the current view if it is
// not already there, using scroll areas in this Panel's hierarchy.
func (p *Panel) ScrollRectIntoView(rect Rect) {
	look := p
	for look != nil {
		if look.ScrollRectIntoViewCallback != nil {
			if look.ScrollRectIntoViewCallback(rect) {
				return
			}
		}
		scale := look.Scale()
		pt := rect.BottomRight().Mul(scale)
		rect.Point = rect.Point.Mul(scale)
		rect.Width = pt.X - rect.X
		rect.Height = pt.Y - rect.Y
		rect.Point = rect.Point.Add(look.frame.Point)
		look = look.parent
	}
}

// ClientData returns a map of client data for this Panel.
func (p *Panel) ClientData() map[string]any {
	if p.data == nil {
		p.data = make(map[string]any)
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
func (p *Panel) IsDragGesture(where Point) bool {
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

// ScrollRoot returns the containing ScrollPanel, if any.
func (p *Panel) ScrollRoot() *ScrollPanel {
	one := p.Parent()
	for one != nil {
		if s, ok := one.Self.(*ScrollPanel); ok {
			return s
		}
		one = one.Parent()
	}
	return nil
}

// ValidateScrollRoot calls ValidateLayout() on the containing ScrollPanel, if any.
func (p *Panel) ValidateScrollRoot() {
	if s := p.ScrollRoot(); s != nil {
		s.ValidateLayout()
	}
}

// InstallCmdHandlers installs handlers for the command with the given ID, returning any previously installed handlers.
func (p *Panel) InstallCmdHandlers(id int, can func(any) bool, do func(any)) (formerCan func(any) bool, formerDo func(any)) {
	if p.canPerformMap == nil {
		p.canPerformMap = make(map[int]func(any) bool)
		p.performMap = make(map[int]func(any))
	}
	formerCan = p.canPerformMap[id]
	formerDo = p.performMap[id]
	p.canPerformMap[id] = can
	p.performMap[id] = do
	return
}

// RemoveCmdHandler removes the handlers for the command with the given ID.
func (p *Panel) RemoveCmdHandler(id int) {
	delete(p.canPerformMap, id)
	delete(p.performMap, id)
	if len(p.canPerformMap) == 0 {
		p.canPerformMap = nil
		p.performMap = nil
	}
}

// CanPerformCmd checks if this panel or its ancestors can perform the command. May be called on a nil Panel object.
func (p *Panel) CanPerformCmd(src any, id int) bool {
	current := p
	for current != nil {
		if f, ok := current.canPerformMap[id]; ok {
			enabled := false
			toolbox.Call(func() { enabled = f(src) })
			return enabled
		}
		current = current.parent
	}
	return false
}

// PerformCmd returns true if the command was handled, either by this panel or its ancestors. May be called on a nil
// Panel object. First calls CanPerformCmd() to ensure the command is permitted to be performed.
func (p *Panel) PerformCmd(src any, id int) {
	if p.CanPerformCmd(src, id) {
		current := p
		for current != nil {
			if f, ok := current.performMap[id]; ok {
				toolbox.Call(func() { f(src) })
				return
			}
			current = current.parent
		}
	}
}

// HasInSelfOrDescendants calls checker for this panel and each of its descendants (depth-first), returning true for the
// first one that checker() returns true for.
func (p *Panel) HasInSelfOrDescendants(checker func(*Panel) bool) bool {
	if checker(p) {
		return true
	}
	for _, child := range p.Children() {
		if child.HasInSelfOrDescendants(checker) {
			return true
		}
	}
	return false
}

// AlwaysEnabled is a helper function whose signature matches the 'can' function signature required for
// InstallCmdHandlers() that always returns true.
func AlwaysEnabled(_ any) bool {
	return true
}

// Ancestor returns the first ancestor of the given type. May return nil if no parent matches.
func Ancestor[T any](paneler Paneler) T {
	if paneler != nil {
		p := paneler.AsPanel().Parent()
		for p != nil {
			if one, ok := p.Self.(T); ok {
				return one
			}
			p = p.Parent()
		}
	}
	var zero T
	return zero
}

// AncestorOrSelf returns the provided panel or the first ancestor of the given type. May return nil if nothing matches.
func AncestorOrSelf[T any](paneler Paneler) T {
	if one, ok := paneler.AsPanel().Self.(T); ok {
		return one
	}
	return Ancestor[T](paneler)
}

// AncestorIs returns true if the paneler has the given ancestor.
func AncestorIs(paneler, ancestor Paneler) bool {
	if toolbox.IsNil(paneler) || toolbox.IsNil(ancestor) {
		return false
	}
	target := ancestor.AsPanel().Self
	p := paneler.AsPanel().Parent()
	for p != nil {
		if p.Self == target {
			return true
		}
		p = p.Parent()
	}
	return false
}

// AncestorIsOrSelf returns true if the paneler has the given ancestor or is the ancestor.
func AncestorIsOrSelf(paneler, ancestor Paneler) bool {
	if toolbox.IsNil(paneler) || toolbox.IsNil(ancestor) {
		return false
	}
	target := ancestor.AsPanel().Self
	p := paneler.AsPanel()
	for p != nil {
		if p.Self == target {
			return true
		}
		p = p.Parent()
	}
	return false
}
