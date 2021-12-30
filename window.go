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
	"runtime"
	"time"

	"github.com/go-gl/gl/v3.2-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

var (
	windowMap  = make(map[*glfw.Window]*Window)
	windowList []*Window
	modalStack []*Window
	glInited   = false
)

// DragData holds data drag information.
type DragData struct {
	Data            map[string]interface{}
	Drawable        Drawable
	SamplingOptions *SamplingOptions
	Ink             Ink
	Offset          geom32.Point
}

// Window holds window information.
type Window struct {
	InputCallbacks
	// MinMaxContentSizeCallback returns the minimum and maximum size for the window content.
	MinMaxContentSizeCallback func() (min, max geom32.Size)
	// MovedCallback is called when the window is moved.
	MovedCallback func()
	// ResizedCallback is called when the window is resized. If constrained is true, then the resulting window size has
	// already been constrained and there is no need to do so again.
	ResizedCallback func(constrained bool)
	// AllowCloseCallback is called when the user has requested that the window be closed. Return true to permit it,
	// false to cancel the operation. Defaults to always returning true.
	AllowCloseCallback func() bool
	// WillCloseCallback is called just prior to the window closing.
	WillCloseCallback   func()
	title               string
	wnd                 *glfw.Window
	surface             *surface
	data                map[string]interface{}
	root                *rootPanel
	focus               *Panel
	cursor              *Cursor
	lastMouseDownPanel  *Panel
	lastMouseOverPanel  *Panel
	lastKeyDownPanel    *Panel
	lastTooltip         *Panel
	lastTooltipShownAt  time.Time
	tooltipSequence     int
	modalResultCode     int
	lastButton          int
	lastButtonCount     int
	lastButtonTime      time.Time
	lastContentRect     geom32.Rect
	firstButtonLocation geom32.Point
	dragDataLocation    geom32.Point
	dragDataPanel       *Panel
	dragData            *DragData
	lastKeyModifiers    Modifiers
	valid               bool
	focused             bool
	transient           bool
	notResizable        bool
	undecorated         bool
	floating            bool
	inModal             bool
	inMouseDown         bool
	cursorHidden        bool
}

// WindowOption holds an option for window creation.
type WindowOption func(*Window) error

// NotResizableWindowOption prevents the window from being resized by the user.
func NotResizableWindowOption() WindowOption {
	return func(w *Window) error {
		w.notResizable = true
		return nil
	}
}

// UndecoratedWindowOption prevents the standard window decorations (title as well as things like close buttons) from
// being shown.
func UndecoratedWindowOption() WindowOption {
	return func(w *Window) error {
		w.undecorated = true
		return nil
	}
}

// FloatingWindowOption causes the window to float in front of all other non-floating windows.
func FloatingWindowOption() WindowOption {
	return func(w *Window) error {
		w.floating = true
		return nil
	}
}

// TransientWindowOption causes the window to be marked as transient, which means it will never be considered the active
// window.
func TransientWindowOption() WindowOption {
	return func(w *Window) error {
		w.transient = true
		return nil
	}
}

// AllWindowsToFront brings all of the application's windows to the foreground.
func AllWindowsToFront() {
	if len(windowList) != 0 {
		list := make([]*Window, len(windowList))
		copy(list, windowList)
		for i := len(list) - 1; i >= 0; i-- {
			list[i].Show()
			if i == 0 {
				list[i].wnd.Focus()
			}
		}
	}
}

// WindowCount returns the number of windows that are open.
func WindowCount() int {
	return len(windowList)
}

// Windows returns a slice containing the current set of open windows.
func Windows() []*Window {
	list := make([]*Window, len(windowList))
	copy(list, windowList)
	return list
}

// ActiveWindow returns the window that currently has the keyboard focus, or nil if none of your application windows
// has the keyboard focus.
func ActiveWindow() *Window {
	nextNonTransientIsFocus := false
	for _, w := range windowList {
		if nextNonTransientIsFocus && !w.transient {
			return w
		}
		if w.focused {
			if w.transient {
				nextNonTransientIsFocus = true
				continue
			}
			return w
		}
	}
	return nil
}

// NewWindow creates a new, initially hidden, window. Call Show() or ToFront() to make it visible.
func NewWindow(title string, options ...WindowOption) (*Window, error) {
	w := &Window{
		title:   title,
		surface: &surface{},
	}
	for _, option := range options {
		if err := option(w); err != nil {
			return nil, err
		}
	}
	glfw.WindowHint(glfw.Visible, glfw.False)
	glfw.WindowHint(glfw.Resizable, glfwEnabled(!w.notResizable))
	glfw.WindowHint(glfw.Decorated, glfwEnabled(!w.undecorated))
	glfw.WindowHint(glfw.Floating, glfwEnabled(w.floating))
	glfw.WindowHint(glfw.AutoIconify, glfw.False)
	glfw.WindowHint(glfw.TransparentFramebuffer, glfw.False)
	glfw.WindowHint(glfw.FocusOnShow, glfw.False)
	glfw.WindowHint(glfw.ScaleToMonitor, glfw.False)
	var err error
	if w.wnd, err = glfw.CreateWindow(1, 1, title, nil, nil); err != nil {
		return nil, err
	}
	w.wnd.SetRefreshCallback(func(_ *glfw.Window) {
		delete(redrawSet, w)
		w.draw()
	})
	w.wnd.SetPosCallback(func(_ *glfw.Window, xpos, ypos int) {
		w.moved()
	})
	w.wnd.SetSizeCallback(func(_ *glfw.Window, width, height int) {
		w.resized(false)
	})
	w.wnd.SetCloseCallback(func(_ *glfw.Window) {
		w.AttemptClose()
	})
	w.wnd.SetFocusCallback(func(_ *glfw.Window, focused bool) {
		if focused {
			if len(modalStack) != 0 {
				modal := modalStack[len(modalStack)-1]
				if modal != w {
					Beep()
					modal.ToFront()
					return
				}
			}
			w.gainedFocus()
		} else {
			w.lostFocus()
		}
	})
	w.wnd.SetMouseButtonCallback(func(_ *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
		where := w.MouseLocation()
		w.lastKeyModifiers = Modifiers(mods)
		if action == glfw.Press {
			maxDelay, maxMouseDrift := DoubleClickParameters()
			now := time.Now()
			if int(button) == w.lastButton && time.Since(w.lastButtonTime) <= maxDelay &&
				mathf32.Abs(where.X-w.firstButtonLocation.X) <= maxMouseDrift &&
				mathf32.Abs(where.Y-w.firstButtonLocation.Y) <= maxMouseDrift {
				w.lastButtonCount++
				time.Since(w.lastButtonTime)
			} else {
				w.lastButtonCount = 1
				w.firstButtonLocation = where
			}
			w.lastButton = int(button)
			w.lastButtonTime = now
			w.inMouseDown = true
			w.mouseDown(where, w.lastButton, w.lastButtonCount, w.lastKeyModifiers)
		} else {
			w.lastButton = int(button)
			w.inMouseDown = false
			w.mouseUp(where, w.lastButton, w.lastKeyModifiers)
		}
	})
	w.wnd.SetCursorPosCallback(func(_ *glfw.Window, x, y float64) {
		where := w.convertMouseLocation(x, y)
		if w.inMouseDown {
			w.mouseDrag(where, w.lastButton, w.lastKeyModifiers)
		} else {
			w.mouseMove(where, w.lastKeyModifiers)
		}
	})
	w.wnd.SetCursorEnterCallback(func(_ *glfw.Window, entered bool) {
		if entered {
			w.mouseEnter(w.MouseLocation(), w.lastKeyModifiers)
		} else {
			w.mouseExit()
		}
	})
	w.wnd.SetScrollCallback(func(_ *glfw.Window, xoff, yoff float64) {
		w.mouseWheel(w.MouseLocation(), w.convertMouseLocation(xoff, yoff), w.lastKeyModifiers)
	})
	w.wnd.SetKeyCallback(func(_ *glfw.Window, key glfw.Key, code int, action glfw.Action, mods glfw.ModifierKey) {
		switch action {
		case glfw.Press:
			w.keyDown(KeyCode(key), Modifiers(mods), false)
		case glfw.Release:
			w.keyUp(KeyCode(key), Modifiers(mods))
		case glfw.Repeat:
			w.keyDown(KeyCode(key), Modifiers(mods), true)
		}
	})
	w.wnd.SetCharCallback(func(_ *glfw.Window, ch rune) {
		w.runeTyped(ch)
	})
	// Real drag & drop support can't really be added due to the way glfw has already hooked in for their primitive
	// file drop capability... so we'll just live with that for now.
	w.wnd.SetDropCallback(func(_ *glfw.Window, files []string) {
		w.fileDrop(files)
	})
	w.valid = true
	windowList = append(windowList, w)
	windowMap[w.wnd] = w
	w.root = newRootPanel(w)
	w.ValidateLayout()
	return w, nil
}

func glfwEnabled(enabled bool) int {
	if enabled {
		return glfw.True
	}
	return glfw.False
}

func (w *Window) moved() {
	if w.root.preMovedCallback != nil {
		toolbox.Call(func() { w.root.preMovedCallback(w) })
	}
	if w.MovedCallback != nil {
		toolbox.Call(w.MovedCallback)
	}
}

func (w *Window) resized(constrained bool) {
	if constrained {
		w.ValidateLayout()
	} else {
		current := w.ContentRect()
		adjusted := w.adjustContentRectForMinMax(current)
		if adjusted != current {
			w.SetContentRect(adjusted)
		} else {
			w.ValidateLayout()
		}
	}
	if w.ResizedCallback != nil {
		toolbox.Call(func() { w.ResizedCallback(true) })
	}
}

func (w *Window) gainedFocus() {
	w.focused = true
	if len(windowList) != 0 && windowList[0] != w {
		w.removeFromWindowList()
		windowList = append(windowList, nil)
		copy(windowList[1:], windowList)
		windowList[0] = w
	}
	w.ClearTooltip()
	if w.focus == nil {
		w.FocusNext()
	}
	if w.focus != nil {
		w.focus.MarkForRedraw()
	}
	if w.GainedFocusCallback != nil {
		w.GainedFocusCallback()
	}
	w.mouseEnter(w.MouseLocation(), 0)
}

func (w *Window) lostFocus() {
	w.focused = false
	w.ClearTooltip()
	if w.focus != nil {
		w.focus.MarkForRedraw()
	}
	if w.LostFocusCallback != nil {
		w.LostFocusCallback()
	}
	if w.root.postLostFocusCallback != nil {
		w.root.postLostFocusCallback(w)
	}
}

// RunModal displays and brings this window to the front, the runs a modal event loop until StopModal is called.
// Disposes the window before it returns.
func (w *Window) RunModal() int {
	defer func() {
		w.removeFromModalStack()
		w.Dispose()
	}()
	active := ActiveWindow()
	w.modalResultCode = -1 // Can't use dialog.ModalResponseDiscard because of package import cycles
	w.inModal = true
	modalStack = append(modalStack, w)
	w.ToFront()
	for w.inModal {
		processEvents()
	}
	if active != nil && active.IsVisible() {
		active.ToFront()
	}
	return w.modalResultCode
}

// StopModal stops the current modal event loop and propagates the provided code as the result to RunModal().
func (w *Window) StopModal(code int) {
	w.modalResultCode = code
	w.removeFromModalStack()
}

func (w *Window) removeFromModalStack() {
	w.inModal = false
	for i, wnd := range modalStack {
		if w != wnd {
			continue
		}
		copy(modalStack[i:], modalStack[i+1:])
		count := len(modalStack) - 1
		modalStack[count] = nil
		modalStack = modalStack[:count]
		break
	}
}

// IsValid returns true if the window is still valid (i.e. hasn't been disposed).
func (w *Window) IsValid() bool {
	return w.valid
}

func (w *Window) String() string {
	return fmt.Sprintf("Window[%s]", w.title)
}

// AttemptClose closes the window if permitted.
func (w *Window) AttemptClose() {
	if w.AllowCloseCallback != nil {
		allow := false
		toolbox.Call(func() { allow = w.AllowCloseCallback() })
		if !allow {
			return
		}
	}
	w.Dispose()
}

func (w *Window) removeFromWindowList() {
	for i, wnd := range windowList {
		if w != wnd {
			continue
		}
		copy(windowList[i:], windowList[i+1:])
		count := len(windowList) - 1
		windowList[count] = nil
		windowList = windowList[:count]
		break
	}
}

// Dispose of the window.
func (w *Window) Dispose() {
	active := ActiveWindow()
	if w.WillCloseCallback != nil {
		toolbox.Call(w.WillCloseCallback)
		w.WillCloseCallback = nil
	}
	if w.inModal {
		w.StopModal(-1) // Can't use dialog.ModalResponseDiscard because of package import cycles
	}
	if w.root.content != nil {
		w.root.content.RemoveFromParent()
	}
	w.removeFromWindowList()
	delete(windowMap, w.wnd)
	if w.valid {
		w.valid = false
		w.surface.dispose()
		delete(windowMap, w.wnd)
		w.wnd.Destroy()
		w.wnd = nil
	}
	if len(windowMap) == 0 && quitAfterLastWindowClosed() {
		quitting()
	}
	if active != nil && active == w && len(windowList) != 0 {
		windowList[0].ToFront()
	}
}

// Title returns the title of this window.
func (w *Window) Title() string {
	return w.title
}

// SetTitle sets the title of this window.
func (w *Window) SetTitle(title string) {
	if w.title != title {
		w.title = title
		w.wnd.SetTitle(title)
	}
}

// Content returns the content panel for the window.
func (w *Window) Content() *Panel {
	return w.root.content
}

// SetContent sets the content panel for the window.
func (w *Window) SetContent(panel Paneler) {
	w.root.setContent(panel)
	w.ValidateLayout()
	w.MarkForRedraw()
}

// ValidateLayout performs any layout that needs to be run by this window or its children.
func (w *Window) ValidateLayout() {
	rect := w.ContentRect()
	rect.X = 0
	rect.Y = 0
	w.root.SetFrameRect(rect)
	w.root.ValidateLayout()
}

// FrameRect returns the boundaries in display coordinates of the frame of this window (i.e. the area that includes both
// the content and its border and window controls).
func (w *Window) FrameRect() geom32.Rect {
	fr := w.frameRect()
	cr := w.ContentRect()
	cr.X -= fr.X
	cr.Y -= fr.Y
	cr.Width += fr.Width
	cr.Height += fr.Height
	return cr
}

func (w *Window) frameRect() geom32.Rect {
	left, top, right, bottom := w.wnd.GetFrameSize()
	r := geom32.NewRect(float32(left), float32(top), float32(right-left), float32(bottom-top))
	if runtime.GOOS != toolbox.MacOS {
		sx, sy := w.wnd.GetContentScale()
		r.X /= sx
		r.Y /= sy
		r.Width /= sx
		r.Height /= sy
	}
	return r
}

// ContentRectForFrameRect returns the content rect for the given frame rect.
func (w *Window) ContentRectForFrameRect(frame geom32.Rect) geom32.Rect {
	fr := w.frameRect()
	frame.X += fr.X
	frame.Y += fr.Y
	frame.Width -= fr.Width
	frame.Height -= fr.Height
	return frame
}

// FrameRectForContentRect returns the frame rect for the given content rect.
func (w *Window) FrameRectForContentRect(cr geom32.Rect) geom32.Rect {
	fr := w.frameRect()
	cr.X -= fr.X
	cr.Y -= fr.Y
	cr.Width += fr.Width
	cr.Height += fr.Height
	return cr
}

// SetFrameRect sets the boundaries of the frame of this window.
func (w *Window) SetFrameRect(rect geom32.Rect) {
	w.SetContentRect(w.ContentRectForFrameRect(rect))
}

func (w *Window) minMaxContentSize() (min, max geom32.Size) {
	if w.MinMaxContentSizeCallback != nil {
		return w.MinMaxContentSizeCallback()
	}
	min, _, max = w.root.Sizes(geom32.Size{})
	return
}

func (w *Window) adjustContentRectForMinMax(rect geom32.Rect) geom32.Rect {
	min, max := w.minMaxContentSize()
	if rect.Width < min.Width {
		rect.Width = min.Width
	} else if rect.Width > max.Width {
		rect.Width = max.Width
	}
	if rect.Height < min.Height {
		rect.Height = min.Height
	} else if rect.Height > max.Height {
		rect.Height = max.Height
	}
	return rect
}

// ContentRect returns the boundaries in display coordinates of the window's content area.
func (w *Window) ContentRect() geom32.Rect {
	x, y := w.wnd.GetPos()
	width, height := w.wnd.GetSize()
	r := geom32.NewRect(float32(x), float32(y), float32(width), float32(height))
	if runtime.GOOS != toolbox.MacOS {
		sx, sy := w.wnd.GetContentScale()
		r.X /= sx
		r.Y /= sy
		r.Width /= sx
		r.Height /= sy
	}
	return r
}

// LocalContentRect returns the boundaries in local coordinates of the window's content area.
func (w *Window) LocalContentRect() geom32.Rect {
	r := w.ContentRect()
	r.Point.X = 0
	r.Point.Y = 0
	return r
}

// SetContentRect sets the boundaries of the frame of this window by converting the content rect into a suitable frame
// rect and then applying it to the window.
func (w *Window) SetContentRect(rect geom32.Rect) {
	rect = w.adjustContentRectForMinMax(rect)
	w.lastContentRect = rect
	if runtime.GOOS != toolbox.MacOS {
		sx, sy := w.wnd.GetContentScale()
		rect.X *= sx
		rect.Y *= sy
		rect.Width *= sx
		rect.Height *= sy
	}
	w.wnd.SetPos(int(rect.X), int(rect.Y))
	w.wnd.SetSize(int(rect.Width), int(rect.Height))
}

// Pack sets the window's content size to match the preferred size of the root panel.
func (w *Window) Pack() {
	_, pref, _ := w.root.Sizes(geom32.Size{})
	rect := w.ContentRect()
	rect.Size = pref
	w.SetContentRect(rect)
}

// Focused returns true if the window has the current keyboard focus.
func (w *Window) Focused() bool {
	return w.focused
}

// Focus returns the panel with the keyboard focus in this window.
func (w *Window) Focus() *Panel {
	if w.focus == nil {
		w.FocusNext()
	}
	return w.focus
}

// SetFocus sets the keyboard focus to the specified target.
func (w *Window) SetFocus(target Paneler) {
	if target != nil {
		newFocus := target.AsPanel()
		tw := newFocus.Window()
		if tw == w && !newFocus.Is(w.focus) {
			oldFocus := w.focus
			if w.focus != nil {
				if oldFocus.LostFocusCallback != nil {
					toolbox.Call(oldFocus.LostFocusCallback)
				}
			}
			w.focus = newFocus
			if newFocus != nil {
				if newFocus.GainedFocusCallback != nil {
					toolbox.Call(newFocus.GainedFocusCallback)
				}
				newFocus.ScrollIntoView()
			}
			for _, p := range []*Panel{oldFocus, newFocus} {
				if p != nil {
					p = p.Parent()
					for p != nil {
						if p.FocusChangeInHierarchyCallback != nil {
							toolbox.Call(func() { p.FocusChangeInHierarchyCallback(oldFocus, newFocus) })
						}
						p = p.Parent()
					}
				}
			}
		}
	}
}

// FocusNext moves the keyboard focus to the next focusable panel.
func (w *Window) FocusNext() {
	if w.root.content != nil {
		current := w.focus
		if current == nil {
			current = w.root.content
		}
		i, focusables := collectFocusables(w.root.content, current, nil)
		if len(focusables) > 0 {
			i++
			if i >= len(focusables) {
				i = 0
			}
			current = focusables[i]
		}
		w.SetFocus(current)
	}
}

// FocusPrevious moves the keyboard focus to the previous focusable panel.
func (w *Window) FocusPrevious() {
	if w.root.content != nil {
		current := w.focus
		if current == nil {
			current = w.root.content
		}
		i, focusables := collectFocusables(w.root.content, current, nil)
		if len(focusables) > 0 {
			i--
			if i < 0 {
				i = len(focusables) - 1
			}
			current = focusables[i]
		}
		w.SetFocus(current)
	}
}

func collectFocusables(current, target *Panel, focusables []*Panel) (match int, result []*Panel) {
	match = -1
	if current.Focusable() {
		if current.Is(target) {
			match = len(focusables)
		}
		focusables = append(focusables, current)
	}
	for _, child := range current.Children() {
		var m int
		m, focusables = collectFocusables(child, target, focusables)
		if match == -1 && m != -1 {
			match = m
		}
	}
	return match, focusables
}

// IsVisible returns true if the window is currently being shown.
func (w *Window) IsVisible() bool {
	if !w.valid {
		return false
	}
	return w.wnd.GetAttrib(glfw.Visible) == glfw.True
}

// Show makes the window visible, if it was previously hidden. If the window is already visible or is in full screen
// mode, this function does nothing.
func (w *Window) Show() {
	w.wnd.Show()
	if runtime.GOOS == toolbox.LinuxOS {
		// For some reason, Linux is ignoring some window positioning calls prior to showing, so immediately reissue the
		// last one we had.
		w.SetContentRect(w.lastContentRect)
	}
}

// Hide hides the window, if it was previously visible. If the window is already hidden or is in full screen mode, this
// function does nothing.
func (w *Window) Hide() {
	w.wnd.Hide()
}

// ToFront attempts to bring the window to the foreground and give it the keyboard focus. If it is hidden, it will be
// made visible first.
func (w *Window) ToFront() {
	w.wnd.Show()
	w.focused = true // Don't wait for the focus event to set this, as Linux delays the notification too much
	w.wnd.Focus()
}

// Minimize performs the minimize function on the window.
func (w *Window) Minimize() {
	w.wnd.Iconify()
}

// Zoom performs the zoom function on the window.
func (w *Window) Zoom() {
	w.wnd.Maximize()
}

// Resizable returns true if the window can be resized by the user.
func (w *Window) Resizable() bool {
	return !w.notResizable
}

// MouseLocation returns the current mouse location relative to this window.
func (w *Window) MouseLocation() geom32.Point {
	return w.convertMouseLocation(w.wnd.GetCursorPos())
}

func (w *Window) convertMouseLocation(x, y float64) geom32.Point {
	pt := geom32.Point{X: float32(x), Y: float32(y)}
	if runtime.GOOS != toolbox.MacOS {
		sx, sy := w.wnd.GetContentScale()
		pt.X /= sx
		pt.Y /= sy
	}
	return pt
}

// BackingScale returns the scale of the backing store for this window.
func (w *Window) BackingScale() (x, y float32) {
	return w.wnd.GetContentScale()
}

// Draw the window contents.
func (w *Window) Draw(c *Canvas) {
	if w.root != nil {
		toolbox.Call(func() {
			w.root.ValidateLayout()
			c.DrawPaint(BackgroundColor.Paint(c, w.LocalContentRect(), Fill))
			w.root.Draw(c, w.LocalContentRect())
			if w.dragData != nil {
				c.Save()
				c.Translate(w.dragDataLocation.X+w.dragData.Offset.X, w.dragDataLocation.Y+w.dragData.Offset.Y)
				r := geom32.Rect{Size: w.dragData.Drawable.LogicalSize()}
				c.ClipRect(r, IntersectClipOp, false)
				w.dragData.Drawable.DrawInRect(c, r, w.dragData.SamplingOptions, w.dragData.Ink.Paint(c, r, Fill))
				c.Restore()
			}
		})
	}
}

func (w *Window) draw() {
	RebuildDynamicColors()
	sx, sy := w.BackingScale()
	w.wnd.MakeContextCurrent()
	if !glInited {
		if err := gl.Init(); err != nil {
			jot.Fatal(1, errs.Wrap(err))
		}
		glInited = true
	}
	c, err := w.surface.prepareCanvas(w.ContentRect().Size, w.LocalContentRect(), sx, sy)
	if err != nil {
		jot.Error(err)
		return
	}
	c.Save()
	w.Draw(c)
	c.Restore()
	c.Flush()
	w.wnd.SwapBuffers()
}

// MarkForRedraw marks this window for drawing at the next update.
func (w *Window) MarkForRedraw() {
	if _, exists := redrawSet[w]; !exists {
		redrawSet[w] = struct{}{}
		if len(redrawSet) == 1 {
			glfw.PostEmptyEvent()
		}
	}
}

// FlushDrawing causes any areas marked for drawing to be drawn now.
func (w *Window) FlushDrawing() {
	if _, exists := redrawSet[w]; exists {
		w.draw()
	}
}

// HideCursor hides the cursor.
func (w *Window) HideCursor() {
	w.wnd.SetInputMode(glfw.CursorMode, glfw.CursorHidden)
}

// ShowCursor shows the cursor.
func (w *Window) ShowCursor() {
	w.wnd.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
}

// HideCursorUntilMouseMoves hides the cursor until the mouse is moved.
func (w *Window) HideCursorUntilMouseMoves() {
	if !w.cursorHidden {
		w.cursorHidden = true
		w.HideCursor()
	}
}

func (w *Window) restoreHiddenCursor() {
	if w.cursorHidden {
		w.cursorHidden = false
		w.ShowCursor()
	}
}

func (w *Window) updateTooltipAndCursor(target *Panel, where geom32.Point) {
	w.updateCursor(target, where)
	w.updateTooltip(target, where)
}

func (w *Window) updateTooltip(target *Panel, where geom32.Point) {
	var avoid geom32.Rect
	var tip *Panel
	for target != nil {
		avoid = target.ContentRect(true)
		avoid.Point = target.PointToRoot(avoid.Point)
		avoid.Align()
		if target.UpdateTooltipCallback != nil {
			toolbox.Call(func() { avoid = target.UpdateTooltipCallback(target.PointFromRoot(where), avoid) })
		}
		if target.Tooltip != nil {
			tip = target.Tooltip
			break
		}
		target = target.parent
	}
	if !w.lastTooltip.Is(tip) {
		wasShowing := w.root.tooltip != nil
		w.ClearTooltip()
		w.lastTooltip = tip
		if tip != nil {
			ts := &tooltipSequencer{window: w, avoid: avoid, sequence: w.tooltipSequence}
			if wasShowing || time.Since(w.lastTooltipShownAt) < TooltipDismissal {
				ts.show()
			} else {
				InvokeTaskAfter(ts.show, TooltipDelay)
			}
		}
	}
}

// ClearTooltip clears any existing tooltip and resets the timer.
func (w *Window) ClearTooltip() {
	w.tooltipSequence++
	w.lastTooltipShownAt = time.Time{}
	w.root.setTooltip(nil)
}

// UpdateCursorNow causes the cursor to be updated as if the mouse had moved.
func (w *Window) UpdateCursorNow() {
	where := w.MouseLocation()
	target := w.root.PanelAt(where)
	w.updateCursor(target, target.PointFromRoot(where))
}

func (w *Window) updateCursor(target *Panel, where geom32.Point) {
	var cursor *Cursor
	for target != nil {
		if target.UpdateCursorCallback == nil {
			target = target.parent
		} else {
			toolbox.Call(func() { cursor = target.UpdateCursorCallback(target.PointFromRoot(where)) })
			break
		}
	}
	if cursor == nil {
		cursor = ArrowCursor()
	}
	if w.cursor != cursor {
		w.cursor = cursor
		w.restoreHiddenCursor()
		w.wnd.SetCursor(w.cursor)
	}
}

func (w *Window) mouseDown(where geom32.Point, button, clickCount int, mod Modifiers) {
	if w.root.preMouseDownCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.root.preMouseDownCallback(w, where) })
		if stop {
			return
		}
	}
	if w.MouseDownCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.MouseDownCallback(where, button, clickCount, mod) })
		if stop {
			return
		}
	}
	if w.focused || w.transient {
		w.ClearTooltip()
		w.lastMouseDownPanel = nil
		panel := w.root.PanelAt(where)
		for panel != nil {
			if panel.MouseDownCallback != nil && panel.Enabled() {
				stop := false
				toolbox.Call(func() { stop = panel.MouseDownCallback(panel.PointFromRoot(where), button, clickCount, mod) })
				if stop {
					w.lastMouseDownPanel = panel
					return
				}
			}
			panel = panel.parent
		}
	}
}

func (w *Window) mouseDrag(where geom32.Point, button int, mod Modifiers) {
	w.dragDataLocation = where
	w.restoreHiddenCursor()
	if w.dragData != nil {
		w.dataDragOver()
		return
	}
	if w.MouseDragCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.MouseDragCallback(where, button, mod) })
		if stop {
			return
		}
	}
	if w.lastMouseDownPanel != nil && w.lastMouseDownPanel.MouseDragCallback != nil && w.lastMouseDownPanel.Enabled() {
		toolbox.Call(func() { w.lastMouseDownPanel.MouseDragCallback(w.lastMouseDownPanel.PointFromRoot(where), button, mod) })
	}
}

func (w *Window) mouseUp(where geom32.Point, button int, mod Modifiers) {
	if w.dragData != nil {
		w.dragDataLocation = where
		w.dataDragFinish()
		w.lastMouseDownPanel = nil
		return
	}
	if w.MouseUpCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.MouseUpCallback(where, button, mod) })
		if stop {
			return
		}
	}
	if w.lastMouseDownPanel != nil && w.lastMouseDownPanel.MouseUpCallback != nil && w.lastMouseDownPanel.Enabled() {
		toolbox.Call(func() { w.lastMouseDownPanel.MouseUpCallback(w.lastMouseDownPanel.PointFromRoot(where), button, mod) })
	}
	panel := w.root.PanelAt(where)
	if w.root != nil && !panel.Is(w.lastMouseOverPanel) {
		w.mouseExit()
	}
	w.updateCursor(panel, where)
	w.updateTooltip(w.lastMouseDownPanel, where)
	w.lastMouseDownPanel = nil
}

func (w *Window) mouseEnter(where geom32.Point, mod Modifiers) {
	w.restoreHiddenCursor()
	w.mouseExit()
	if w.MouseEnterCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.MouseEnterCallback(where, mod) })
		if stop {
			return
		}
	}
	panel := w.root.PanelAt(where)
	if panel.MouseEnterCallback != nil {
		toolbox.Call(func() { panel.MouseEnterCallback(panel.PointFromRoot(where), mod) })
	}
	w.updateTooltipAndCursor(panel, where)
	w.lastMouseOverPanel = panel
}

func (w *Window) mouseMove(where geom32.Point, mod Modifiers) {
	w.restoreHiddenCursor()
	panel := w.root.PanelAt(where)
	if panel.Is(w.lastMouseOverPanel) {
		if w.MouseMoveCallback != nil {
			stop := false
			toolbox.Call(func() { stop = w.MouseMoveCallback(where, mod) })
			if stop {
				return
			}
		}
		if panel.MouseMoveCallback != nil {
			toolbox.Call(func() { panel.MouseMoveCallback(panel.PointFromRoot(where), mod) })
		}
		w.updateTooltipAndCursor(panel, where)
	} else {
		w.mouseEnter(where, mod)
	}
}

func (w *Window) mouseExit() {
	if w.MouseExitCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.MouseExitCallback() })
		if stop {
			return
		}
	}
	if w.lastMouseDownPanel == nil && w.lastMouseOverPanel != nil {
		if w.lastMouseOverPanel.MouseExitCallback != nil {
			toolbox.Call(func() { w.lastMouseOverPanel.MouseExitCallback() })
		}
		w.lastMouseOverPanel = nil
		w.cursor = nil
	}
}

func (w *Window) mouseWheel(where, delta geom32.Point, mod Modifiers) {
	if w.MouseWheelCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.MouseWheelCallback(where, delta, mod) })
		if stop {
			return
		}
	}
	panel := w.root.PanelAt(where)
	for panel != nil {
		if panel.Enabled() && panel.MouseWheelCallback != nil {
			stop := false
			toolbox.Call(func() { stop = panel.MouseWheelCallback(panel.PointFromRoot(where), delta, mod) })
			if stop {
				break
			}
		}
		panel = panel.parent
	}
	if w.lastMouseDownPanel != nil {
		w.mouseDrag(where, 0, mod)
	} else {
		w.mouseMove(where, mod)
	}
}

func (w *Window) keyDown(keyCode KeyCode, mod Modifiers, repeat bool) {
	if w.root.preKeyDownCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.root.preKeyDownCallback(w, keyCode, mod) })
		if stop {
			return
		}
	}
	if w.KeyDownCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.KeyDownCallback(keyCode, mod, repeat) })
		if stop {
			return
		}
	}
	w.ClearTooltip()
	w.lastKeyDownPanel = nil
	if focus := w.Focus(); focus != nil {
		panel := focus
		w.lastKeyDownPanel = panel
		for panel != nil {
			if panel.Enabled() && panel.KeyDownCallback != nil {
				stop := false
				toolbox.Call(func() { stop = panel.KeyDownCallback(keyCode, mod, repeat) })
				if stop {
					w.lastKeyDownPanel = panel
					return
				}
			}
			panel = panel.parent
		}
		if keyCode == KeyTab && (mod&(AllModifiers&^ShiftModifier)) == 0 {
			if mod.ShiftDown() {
				w.FocusPrevious()
			} else {
				w.FocusNext()
			}
		}
	}
}

func (w *Window) keyUp(keyCode KeyCode, mod Modifiers) {
	if w.root.preKeyUpCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.root.preKeyUpCallback(w, keyCode, mod) })
		if stop {
			return
		}
	}
	if w.KeyUpCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.KeyUpCallback(keyCode, mod) })
		if stop {
			return
		}
	}
	if w.lastKeyDownPanel != nil && w.lastKeyDownPanel.KeyUpCallback != nil {
		toolbox.Call(func() { w.lastKeyDownPanel.KeyUpCallback(keyCode, mod) })
	}
}

func (w *Window) runeTyped(ch rune) {
	if w.root.preRuneTypedCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.root.preRuneTypedCallback(w, ch) })
		if stop {
			return
		}
	}
	if w.RuneTypedCallback != nil {
		stop := false
		toolbox.Call(func() { stop = w.RuneTypedCallback(ch) })
		if stop {
			return
		}
	}
	w.ClearTooltip()
	w.lastKeyDownPanel = nil
	if focus := w.Focus(); focus != nil {
		panel := focus
		w.lastKeyDownPanel = panel
		for panel != nil {
			if panel.Enabled() && panel.RuneTypedCallback != nil {
				stop := false
				toolbox.Call(func() { stop = panel.RuneTypedCallback(ch) })
				if stop {
					w.lastKeyDownPanel = panel
					return
				}
			}
			panel = panel.parent
		}
	}
}

func (w *Window) fileDrop(files []string) {
	if w.FileDropCallback != nil {
		toolbox.Call(func() { w.FileDropCallback(files) })
		return
	}
	panel := w.root.PanelAt(w.MouseLocation())
	for panel != nil {
		if panel.FileDropCallback != nil && panel.Enabled() {
			toolbox.Call(func() { panel.FileDropCallback(files) })
			return
		}
		panel = panel.parent
	}
}

// ClientData returns a map of client data for this window.
func (w *Window) ClientData() map[string]interface{} {
	if w.data == nil {
		w.data = make(map[string]interface{})
	}
	return w.data
}

// IsDragGesture returns true if a gesture to start a drag operation was made.
func (w *Window) IsDragGesture(where geom32.Point) bool {
	minDelay, minMouseDrift := DragGestureParameters()
	return w.inMouseDown &&
		mathf32.Abs(w.firstButtonLocation.X-where.X) > minMouseDrift ||
		mathf32.Abs(w.firstButtonLocation.Y-where.Y) > minMouseDrift ||
		time.Since(w.lastButtonTime) > minDelay
}

// StartDataDrag starts a data drag operation.
func (w *Window) StartDataDrag(data *DragData) {
	if data != nil && len(data.Data) != 0 && data.Drawable != nil && data.Ink != nil {
		w.dragData = data
		w.dragDataPanel = nil
		w.dataDragOver()
	}
}

func (w *Window) dataDragOver() {
	w.MarkForRedraw()
	panel := w.root.PanelAt(w.dragDataLocation)
	for panel != nil {
		for panel != nil && panel.DataDragOverCallback == nil {
			panel = panel.Parent()
		}
		if panel != nil {
			handled := false
			toolbox.Call(func() { handled = panel.DataDragOverCallback(panel.PointFromRoot(w.dragDataLocation), w.dragData.Data) })
			if handled {
				if !panel.Is(w.dragDataPanel) {
					if w.dragDataPanel != nil && w.dragDataPanel.DataDragExitCallback != nil {
						toolbox.Call(w.dragDataPanel.DataDragExitCallback)
					}
					w.dragDataPanel = panel
				}
				return
			}
		}
	}
	if w.dragDataPanel != nil && w.dragDataPanel.DataDragExitCallback != nil {
		toolbox.Call(w.dragDataPanel.DataDragExitCallback)
	}
	w.dragDataPanel = nil
}

func (w *Window) dataDragFinish() {
	w.MarkForRedraw()
	if w.dragDataPanel != nil && w.dragDataPanel.DataDragDropCallback != nil {
		toolbox.Call(func() {
			w.dragDataPanel.DataDragDropCallback(w.dragDataPanel.PointFromRoot(w.dragDataLocation), w.dragData.Data)
		})
	}
	w.dragDataPanel = nil
	w.dragData = nil
}
