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
	"image"
	"time"

	"github.com/go-gl/gl/v3.2-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/collection/slice"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/fatal"
	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/pathop"
)

var _ UndoManagerProvider = &Window{}

var (
	// DefaultTitleIcons are the default title icons that will be used for all newly created windows. The image closest to
	// the size desired by the system will be selected and used, scaling if needed. If no images are specified, the system's
	// default icon will be used.
	DefaultTitleIcons []*Image
	windowMap         = make(map[*glfw.Window]*Window)
	windowList        []*Window
	modalStack        []*Window
	glInited          = false
)

// DragData holds data drag information.
type DragData struct {
	Data            map[string]any
	Drawable        Drawable
	SamplingOptions *SamplingOptions
	Ink             Ink
	Offset          Point
}

// Window holds window information.
type Window struct {
	InputCallbacks
	// MinMaxContentSizeCallback returns the minimum and maximum size for the window content.
	MinMaxContentSizeCallback func() (minimum, maximum Size)
	// MovedCallback is called when the window is moved.
	MovedCallback func()
	// ResizedCallback is called when the window is resized.
	ResizedCallback func()
	// AllowCloseCallback is called when the user has requested that the window be closed. Return true to permit it,
	// false to cancel the operation. Defaults to always returning true.
	AllowCloseCallback func() bool
	// WillCloseCallback is called just prior to the window closing.
	WillCloseCallback func()
	// DragIntoWindowWillStart is called just prior to a drag into the window starting.
	DragIntoWindowWillStart func()
	// DragIntoWindowFinished is called just after a drag into the window completes, whether a drop occurs or not.
	DragIntoWindowFinished func()
	title                  string
	titleIcons             []*Image
	wnd                    *glfw.Window
	surface                *surface
	data                   map[string]any
	root                   *rootPanel
	focus                  *Panel
	cursor                 *Cursor
	lastMouseDownPanel     *Panel
	lastMouseOverPanel     *Panel
	lastKeyDownPanel       *Panel
	lastTooltip            *Panel
	lastTooltipShownAt     time.Time
	lastDrawDuration       time.Duration
	tooltipSequence        int
	modalResultCode        int
	lastButton             int
	lastButtonCount        int
	lastButtonTime         time.Time
	lastContentRect        Rect
	firstButtonLocation    Point
	dragDataLocation       Point
	dragDataPanel          *Panel
	dragData               *DragData
	lastKeyModifiers       Modifiers
	valid                  bool
	focused                bool
	transient              bool
	notResizable           bool
	undecorated            bool
	floating               bool
	inModal                bool
	inMouseDown            bool
	cursorHidden           bool
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

// TitleIconsWindowOption sets the title icon of the window. The image closest to the size desired by the system will be
// selected and used, scaling if needed. If no images are specified, the system's default window icon will be used.
func TitleIconsWindowOption(images []*Image) WindowOption {
	return func(w *Window) error {
		w.titleIcons = images
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
		title:      title,
		titleIcons: DefaultTitleIcons,
		surface:    &surface{},
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
	toolbox.CallWithHandler(func() {
		w.wnd, err = glfw.CreateWindow(1, 1, title, nil, nil)
	}, func(panicErr error) {
		err = panicErr
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}
	w.wnd.SetRefreshCallback(func(_ *glfw.Window) {
		delete(redrawSet, w)
		w.draw()
	})
	w.wnd.SetPosCallback(func(_ *glfw.Window, _, _ int) {
		w.moved()
	})
	w.wnd.SetSizeCallback(func(_ *glfw.Window, width, height int) {
		if width > 0 && height > 0 {
			w.resized()
		}
	})
	w.wnd.SetCloseCallback(func(_ *glfw.Window) {
		if w.okToProcess() {
			w.AttemptClose()
		}
	})
	w.wnd.SetFocusCallback(func(_ *glfw.Window, focused bool) {
		if focused {
			if w.okToProcess() {
				w.gainedFocus()
			} else {
				modalStack[len(modalStack)-1].ToFront()
			}
		} else {
			w.lostFocus()
		}
	})
	w.wnd.SetMouseButtonCallback(w.mouseButtonCallback)
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
		w.mouseWheel(w.MouseLocation(), Point{X: float32(xoff), Y: float32(yoff)}, w.lastKeyModifiers)
	})
	w.wnd.SetKeyCallback(w.keyCallbackForGLFW)
	w.wnd.SetCharCallback(func(_ *glfw.Window, ch rune) {
		if w.okToProcess() {
			w.runeTyped(ch)
		}
	})
	// Real drag & drop support can't really be added due to the way glfw has already hooked in for their primitive
	// file drop capability... so we'll just live with that for now.
	w.wnd.SetDropCallback(func(_ *glfw.Window, files []string) {
		if w.okToProcess() {
			w.fileDrop(files)
		}
	})
	w.valid = true
	windowList = append(windowList, w)
	windowMap[w.wnd] = w
	w.root = newRootPanel(w)
	w.ValidateLayout()
	w.SetTitleIcons(w.titleIcons)
	return w, nil
}

func (w *Window) commonKeyCallbackForGLFW(key glfw.Key, action glfw.Action, mods glfw.ModifierKey) {
	w.lastKeyModifiers = Modifiers(mods)
	switch action {
	case glfw.Press:
		w.keyDown(KeyCode(key), Modifiers(mods), false)
	case glfw.Release:
		w.keyUp(KeyCode(key), Modifiers(mods))
	case glfw.Repeat:
		w.keyDown(KeyCode(key), Modifiers(mods), true)
	}
}

// LastKeyModifiers returns the last set of key modifiers that this window has received.
func (w *Window) LastKeyModifiers() Modifiers {
	return w.lastKeyModifiers
}

func glfwEnabled(enabled bool) int {
	if enabled {
		return glfw.True
	}
	return glfw.False
}

func (w *Window) mouseButtonCallback(_ *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	if !w.okToProcess() {
		modalStack[len(modalStack)-1].mouseButtonCallback(nil, button, action, mods)
		return
	}
	w.lastKeyModifiers = Modifiers(mods)
	where := w.MouseLocation()
	if action == glfw.Press {
		maxDelay, maxMouseDrift := DoubleClickParameters()
		now := time.Now()
		if int(button) == w.lastButton && time.Since(w.lastButtonTime) <= maxDelay &&
			xmath.Abs(where.X-w.firstButtonLocation.X) <= maxMouseDrift &&
			xmath.Abs(where.Y-w.firstButtonLocation.Y) <= maxMouseDrift {
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
	} else if w.inMouseDown {
		w.lastButton = int(button)
		w.inMouseDown = false
		w.mouseUp(where, w.lastButton, w.lastKeyModifiers)
	}
}

func (w *Window) okToProcess() bool {
	return len(modalStack) == 0 || modalStack[len(modalStack)-1] == w
}

// UndoManager returns the UndoManager for the currently focused panel in the Window. May return nil.
func (w *Window) UndoManager() *UndoManager {
	if focus := w.Focus(); focus != nil {
		return UndoManagerFor(focus)
	}
	return nil
}

func (w *Window) moved() {
	w.lastContentRect = w.ContentRect()
	w.root.preMoved(w)
	if w.MovedCallback != nil {
		toolbox.Call(w.MovedCallback)
	}
}

func (w *Window) resized() {
	current := w.ContentRect()
	adjusted := w.adjustContentRectForMinMax(current)
	if adjusted != current {
		w.SetContentRect(adjusted)
	}
	w.ValidateLayout()
	if w.ResizedCallback != nil {
		toolbox.Call(func() { w.ResizedCallback() })
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
	w.restoreHiddenCursor()
	w.focused = false
	w.ClearTooltip()
	if w.focus != nil {
		w.focus.MarkForRedraw()
	}
	if w.LostFocusCallback != nil {
		w.LostFocusCallback()
	}
	if w.root.menuBar != nil {
		w.root.menuBar.postLostFocus(w)
	}
}

// RunModal displays and brings this window to the front, the runs a modal event loop until StopModal is called.
// Disposes the window before it returns.
func (w *Window) RunModal() int {
	active := ActiveWindow()
	if active != nil {
		active.restoreHiddenCursor()
	}
	defer func() {
		w.removeFromModalStack()
		w.Dispose()
		if active != nil && active.IsVisible() {
			active.ToFront()
		}
	}()
	w.modalResultCode = ModalResponseDiscard
	w.inModal = true
	modalStack = append(modalStack, w)
	w.ToFront()
	for w.inModal {
		processEvents()
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
		modalStack = slice.ZeroedDelete(modalStack, i, i+1)
		break
	}
}

// IsValid returns true if the window is still valid (i.e. hasn't been disposed).
func (w *Window) IsValid() bool {
	return w != nil && w.valid
}

func (w *Window) String() string {
	return fmt.Sprintf("Window[%s]", w.title)
}

// AttemptClose closes the window if permitted. Returns true on success.
func (w *Window) AttemptClose() bool {
	if w.AllowCloseCallback != nil {
		allow := false
		toolbox.Call(func() { allow = w.AllowCloseCallback() })
		if !allow {
			return false
		}
	}
	w.Dispose()
	return true
}

func (w *Window) removeFromWindowList() {
	for i, wnd := range windowList {
		if w != wnd {
			continue
		}
		windowList = slice.ZeroedDelete(windowList, i, i+1)
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
		w.StopModal(ModalResponseDiscard)
	}
	if w.root.contentPanel != nil {
		w.root.contentPanel.RemoveFromParent()
	}
	w.removeFromWindowList()
	delete(windowMap, w.wnd)
	if w.IsValid() {
		w.valid = false
		w.surface.dispose()
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
		if w.IsValid() {
			w.wnd.SetTitle(title)
		}
	}
}

// TitleIcons the title icons that were previously set, if any.
func (w *Window) TitleIcons() []*Image {
	return w.titleIcons
}

// SetTitleIcons sets the title icon of the window. The image closest to the size desired by the system will be selected
// and used, scaling if needed. If no images are specified, the window reverts to its default icon.
func (w *Window) SetTitleIcons(images []*Image) {
	w.titleIcons = images
	imgs := make([]image.Image, 0, len(images))
	for _, img := range images {
		if nrgba, err := img.ToNRGBA(); err != nil {
			errs.Log(err)
		} else {
			w.titleIcons = append(w.titleIcons, img)
			imgs = append(imgs, nrgba)
		}
	}
	if w.IsValid() {
		w.wnd.SetIcon(imgs)
	}
}

// Content returns the content panel for the window.
func (w *Window) Content() *Panel {
	return w.root.contentPanel
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
func (w *Window) FrameRect() Rect {
	fr := w.frameRect()
	cr := w.ContentRect()
	cr.X -= fr.X
	cr.Y -= fr.Y
	cr.Width += fr.Width
	cr.Height += fr.Height
	return cr
}

// ContentRectForFrameRect returns the content rect for the given frame rect.
func (w *Window) ContentRectForFrameRect(frame Rect) Rect {
	fr := w.frameRect()
	frame.X += fr.X
	frame.Y += fr.Y
	frame.Width -= fr.Width
	frame.Height -= fr.Height
	return frame
}

// FrameRectForContentRect returns the frame rect for the given content rect.
func (w *Window) FrameRectForContentRect(cr Rect) Rect {
	fr := w.frameRect()
	cr.X -= fr.X
	cr.Y -= fr.Y
	cr.Width += fr.Width
	cr.Height += fr.Height
	return cr
}

// SetFrameRect sets the boundaries of the frame of this window.
func (w *Window) SetFrameRect(rect Rect) {
	w.SetContentRect(w.ContentRectForFrameRect(rect))
}

func (w *Window) minMaxContentSize() (minimum, maximum Size) {
	if w.MinMaxContentSizeCallback != nil {
		return w.MinMaxContentSizeCallback()
	}
	minimum, _, maximum = w.root.Sizes(Size{})
	return
}

func (w *Window) adjustContentRectForMinMax(rect Rect) Rect {
	minimum, maximum := w.minMaxContentSize()
	if rect.Width < minimum.Width {
		rect.Width = minimum.Width
	} else if rect.Width > maximum.Width {
		rect.Width = maximum.Width
	}
	if rect.Height < minimum.Height {
		rect.Height = minimum.Height
	} else if rect.Height > maximum.Height {
		rect.Height = maximum.Height
	}
	w.lastContentRect = rect
	return rect
}

// LocalContentRect returns the boundaries in local coordinates of the window's content area.
func (w *Window) LocalContentRect() Rect {
	r := w.ContentRect()
	r.Point.X = 0
	r.Point.Y = 0
	return r
}

// Pack sets the window's content size to match the preferred size of the root panel.
func (w *Window) Pack() {
	_, pref, _ := w.root.Sizes(Size{})
	rect := w.ContentRect()
	rect.Size = pref
	w.SetContentRect(BestDisplayForRect(rect).FitRectOnto(rect))
}

// Focused returns true if the window has the current keyboard focus.
func (w *Window) Focused() bool {
	return w.focused
}

// Focus returns the panel with the keyboard focus in this window.
func (w *Window) Focus() *Panel {
	if w == nil {
		return nil
	}
	if w.focus == nil || w.focus.Window() != w {
		w.FocusNext()
	}
	return w.focus
}

// SetFocus sets the keyboard focus to the specified target.
func (w *Window) SetFocus(target Paneler) {
	var newFocus *Panel
	if target != nil {
		newFocus = target.AsPanel()
	}
	oldFocus := w.focus
	if newFocus == nil {
		w.removeFocus()
		return
	}
	if newFocus.Window() == w {
		if !newFocus.Focusable() {
			if newFocus = newFocus.FirstFocusableChild(); newFocus == nil {
				w.removeFocus()
				return
			}
		}
		if !newFocus.Is(w.focus) {
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
			}
			w.notifyOfFocusChangeInHierarchy(oldFocus, newFocus)
		}
	}
}

func (w *Window) removeFocus() {
	oldFocus := w.focus
	if oldFocus != nil {
		if oldFocus.LostFocusCallback != nil {
			toolbox.Call(oldFocus.LostFocusCallback)
		}
		w.focus = nil
		w.notifyOfFocusChangeInHierarchy(oldFocus, nil)
	}
}

func (w *Window) notifyOfFocusChangeInHierarchy(oldFocus, newFocus *Panel) {
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

// FocusNext moves the keyboard focus to the next focusable panel.
func (w *Window) FocusNext() {
	if w.root.contentPanel != nil {
		current := w.focus
		if current == nil {
			current = w.root.contentPanel
		}
		i, focusables := collectFocusables(w.root.contentPanel, current, nil)
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
	if w.root.contentPanel != nil {
		current := w.focus
		if current == nil {
			current = w.root.contentPanel
		}
		i, focusables := collectFocusables(w.root.contentPanel, current, nil)
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
	if w.IsValid() {
		return w.wnd.GetAttrib(glfw.Visible) == glfw.True
	}
	return false
}

// Show makes the window visible, if it was previously hidden. If the window is already visible or is in full screen
// mode, this function does nothing.
func (w *Window) Show() {
	if w.IsValid() {
		w.wnd.Show()
		// For some reason, Linux is ignoring some window positioning calls prior to showing, so immediately reissue the
		// last one we had.
		w.SetContentRect(w.lastContentRect)
	}
}

// Hide hides the window, if it was previously visible. If the window is already hidden or is in full screen mode, this
// function does nothing.
func (w *Window) Hide() {
	if w.IsValid() {
		w.wnd.Hide()
	}
}

// ToFront attempts to bring the window to the foreground and give it the keyboard focus. If it is hidden, it will be
// made visible first.
func (w *Window) ToFront() {
	if w.IsValid() {
		w.Show()
		w.focused = true // Don't wait for the focus event to set this, as Linux delays the notification too much
		w.wnd.Focus()
	}
}

// Minimize performs the minimize function on the window.
func (w *Window) Minimize() {
	if w.IsValid() {
		w.wnd.Iconify()
	}
}

// Zoom performs the zoom function on the window.
func (w *Window) Zoom() {
	if w.IsValid() {
		w.wnd.Maximize()
	}
}

// Resizable returns true if the window can be resized by the user.
func (w *Window) Resizable() bool {
	return !w.notResizable
}

// MouseLocation returns the current mouse location relative to this window.
func (w *Window) MouseLocation() Point {
	if w.IsValid() {
		return w.convertMouseLocation(w.wnd.GetCursorPos())
	}
	return Point{}
}

// BackingScale returns the scale of the backing store for this window.
func (w *Window) BackingScale() (x, y float32) {
	if w.IsValid() {
		return w.wnd.GetContentScale()
	}
	return 1, 1
}

// Draw the window contents.
func (w *Window) Draw(c *Canvas) {
	if w.root != nil {
		toolbox.Call(func() {
			w.root.ValidateLayout()
			c.DrawPaint(BackgroundColor.Paint(c, w.LocalContentRect(), paintstyle.Fill))
			w.root.Draw(c, w.LocalContentRect())
			if w.InDrag() {
				c.Save()
				c.Translate(w.dragDataLocation.X+w.dragData.Offset.X, w.dragDataLocation.Y+w.dragData.Offset.Y)
				r := Rect{Size: w.dragData.Drawable.LogicalSize()}
				c.ClipRect(r, pathop.Intersect, false)
				w.dragData.Drawable.DrawInRect(c, r, w.dragData.SamplingOptions,
					w.dragData.Ink.Paint(c, r, paintstyle.Fill))
				c.Restore()
			}
		})
	}
}

func (w *Window) draw() {
	RebuildDynamicColors()
	if w.IsValid() {
		sx, sy := w.BackingScale()
		w.wnd.MakeContextCurrent()
		if !glInited {
			fatal.IfErr(gl.Init())
			glInited = true
		}
		c, err := w.surface.prepareCanvas(w.ContentRect().Size, w.LocalContentRect(), sx, sy)
		if err != nil {
			errs.Log(err, "size", w.ContentRect().Size, "rect", w.LocalContentRect(), "scaleX", sx, "scaleY", sy)
			return
		}
		start := time.Now()
		c.Save()
		w.Draw(c)
		c.Restore()
		c.Flush()
		w.lastDrawDuration = time.Since(start)
		w.wnd.SwapBuffers()
	}
}

// LastDrawDuration returns the duration of the window's most recent draw.
func (w *Window) LastDrawDuration() time.Duration {
	return w.lastDrawDuration
}

// MarkForRedraw marks this window for drawing at the next update.
func (w *Window) MarkForRedraw() {
	if _, exists := redrawSet[w]; !exists {
		redrawSet[w] = struct{}{}
		if len(redrawSet) == 1 {
			postEmptyEvent()
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
	if w.IsValid() {
		w.wnd.SetInputMode(glfw.CursorMode, glfw.CursorHidden)
	}
}

// ShowCursor shows the cursor.
func (w *Window) ShowCursor() {
	if w.IsValid() {
		w.wnd.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
	}
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

func (w *Window) updateTooltipAndCursor(target *Panel, where Point) {
	w.updateCursor(target, where)
	w.updateTooltip(target, where)
}

func (w *Window) updateTooltip(target *Panel, where Point) {
	var avoid Rect
	var tip *Panel
	for target != nil {
		avoid = target.RectToRoot(target.ContentRect(true))
		avoid.Align()
		if target.UpdateTooltipCallback != nil {
			toolbox.Call(func() { avoid = target.UpdateTooltipCallback(target.PointFromRoot(where), avoid) })
		}
		if target.Tooltip != nil {
			tip = target.Tooltip
			tip.TooltipImmediate = target.TooltipImmediate
			break
		}
		target = target.parent
	}
	if !w.lastTooltip.Is(tip) {
		wasShowing := w.root.tooltipPanel != nil
		w.ClearTooltip()
		w.lastTooltip = tip
		if tip != nil {
			ts := &tooltipSequencer{window: w, avoid: avoid, sequence: w.tooltipSequence}
			if tip.TooltipImmediate || wasShowing || time.Since(w.lastTooltipShownAt) < DefaultTooltipTheme.Dismissal {
				ts.show()
			} else {
				InvokeTaskAfter(ts.show, DefaultTooltipTheme.Delay)
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

func (w *Window) updateCursor(target *Panel, where Point) {
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
		if w.IsValid() {
			w.wnd.SetCursor(w.cursor)
		}
	}
}

func (w *Window) mouseDown(where Point, button, clickCount int, mod Modifiers) {
	if w.root.preMouseDown(w, where) {
		return
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

// InDrag returns true if a drag is currently in progress in this window.
func (w *Window) InDrag() bool {
	return w.dragData != nil
}

func (w *Window) mouseDrag(where Point, button int, mod Modifiers) {
	w.dragDataLocation = where
	w.restoreHiddenCursor()
	if w.InDrag() {
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

func (w *Window) mouseUp(where Point, button int, mod Modifiers) {
	if w.InDrag() {
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

func (w *Window) mouseEnter(where Point, mod Modifiers) {
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

func (w *Window) mouseMove(where Point, mod Modifiers) {
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

func (w *Window) mouseWheel(where, delta Point, mod Modifiers) {
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
	if w.inMouseDown && w.lastMouseDownPanel != nil {
		w.mouseDrag(where, w.lastButton, mod)
	} else {
		w.mouseMove(where, mod)
	}
}

func (w *Window) keyDown(keyCode KeyCode, mod Modifiers, repeat bool) {
	if w.root.preKeyDown(w, keyCode, mod, repeat) {
		return
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
	if w.root.preKeyUp(w, keyCode, mod) {
		return
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
	if w.root.preRuneTyped(w, ch) {
		return
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
func (w *Window) ClientData() map[string]any {
	if w.data == nil {
		w.data = make(map[string]any)
	}
	return w.data
}

// IsDragGesture returns true if a gesture to start a drag operation was made.
func (w *Window) IsDragGesture(where Point) bool {
	minDelay, minMouseDrift := DragGestureParameters()
	return w.inMouseDown &&
		xmath.Abs(w.firstButtonLocation.X-where.X) > minMouseDrift ||
		xmath.Abs(w.firstButtonLocation.Y-where.Y) > minMouseDrift ||
		time.Since(w.lastButtonTime) > minDelay
}

// StartDataDrag starts a data drag operation.
func (w *Window) StartDataDrag(data *DragData) {
	if data != nil && len(data.Data) != 0 && data.Drawable != nil && data.Ink != nil {
		w.dragData = data
		w.dragDataPanel = nil
		if w.DragIntoWindowWillStart != nil {
			toolbox.Call(w.DragIntoWindowWillStart)
		}
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
			panel = panel.Parent()
		}
	}
	if w.dragDataPanel != nil && w.dragDataPanel.DataDragExitCallback != nil {
		toolbox.Call(w.dragDataPanel.DataDragExitCallback)
	}
	w.dragDataPanel = nil
}

func (w *Window) dataDragFinish() {
	w.MarkForRedraw()
	dragData := w.dragData
	dragDataLocation := w.dragDataLocation
	dragDataPanel := w.dragDataPanel
	w.dragData = nil
	w.dragDataPanel = nil
	if dragDataPanel != nil && dragDataPanel.DataDragDropCallback != nil {
		toolbox.Call(func() {
			dragDataPanel.DataDragDropCallback(dragDataPanel.PointFromRoot(dragDataLocation), dragData.Data)
		})
	}
	if w.DragIntoWindowFinished != nil {
		toolbox.Call(w.DragIntoWindowFinished)
	}
}
