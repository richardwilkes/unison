// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
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
	"maps"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

var _ UndoManagerProvider = &Window{}

var (
	// DefaultTitleIcons are the default title icons that will be used for all newly created windows. The image closest
	// to the size desired by the system will be selected and used, scaling if needed. If no images are specified, the
	// system's default icon will be used.
	DefaultTitleIcons []*Image
	windowList        []*Window
	modalStack        []*Window
	wndWithCurrentCtx *Window
)

// WindowKind represents the kind of window, which can be used by the system to determine how to treat the window in
// various ways, such as how to group it with other windows and what decorations to apply.
type WindowKind byte

// Possible values for WindowKind.
const (
	WindowKindNormal WindowKind = iota
	WindowKindDialog
	WindowKindMenu
	WindowKindTooltip
)

// Window holds window information.
type Window struct {
	InputCallbacks
	drag.Callbacks
	// MinMaxContentSizeCallback returns the minimum and maximum size for the window content.
	MinMaxContentSizeCallback func() (minimum, maximum geom.Size)
	// MovedCallback is called when the window is moved.
	MovedCallback func()
	// ResizedCallback is called when the window is resized.
	ResizedCallback func()
	// MinimizedCallback is called when the window is about to beminimized or restored from minimization.
	MinimizedCallback func(minimized bool)
	// MaximizedCallback is called when the window is about to be maximized or restored from maximization.
	MaximizedCallback func(maximized bool)
	// AllowCloseCallback is called when the user has requested that the window be closed. Return true to permit it,
	// false to cancel the operation. Defaults to always returning true.
	AllowCloseCallback func() bool
	// WillCloseCallback is called just prior to the window closing.
	WillCloseCallback func()
	// ContentScaleCallback is called when the backing scale of the window changes.
	ContentScaleCallback        func(scale geom.Point)
	wnd                         *apiWindow
	surface                     *surface
	glCtx                       *apiGLContext
	root                        *rootPanel
	focus                       *Panel
	cursor                      *Cursor
	lastDropTarget              *Panel
	dragSourceCleanup           func()
	lastMouseDownPanel          *Panel
	lastMouseOverPanel          *Panel
	lastKeyDownPanel            *Panel
	lastTooltip                 *Panel
	lastTooltipShownAt          time.Time
	lastButtonTime              time.Time
	pressedKeys                 map[KeyCode]bool
	pressedButtons              map[int]bool
	data                        map[string]any
	dragTypes                   map[string]*uti.DataType
	title                       string
	titleIcons                  []*Image
	lastDrawDuration            time.Duration
	tooltipSequence             int
	modalResultCode             int
	lastButton                  int
	lastButtonCount             int
	lastContentRect             geom.Rect
	firstButtonLocation         geom.Point
	dragDataLocation            geom.Point
	lastWidth                   float32
	lastHeight                  float32
	lastKeyModifiers            mod.Modifiers
	kind                        WindowKind
	lastDragOp                  drag.Op
	valid                       bool
	keepHidden                  bool
	focused                     bool
	transient                   bool
	notResizable                bool
	transparent                 bool
	undecorated                 bool
	floating                    bool
	inModal                     bool
	inMouseDown                 bool
	cursorHiddenUntilMouseMoves bool
	cursorHidden                bool
	minimized                   bool
	maximized                   bool
}

// WindowOption holds an option for window creation.
type WindowOption func(*Window) error

// WindowKindWindowOption sets the kind of the window, which can affect how the system treats it in various ways.
func WindowKindWindowOption(kind WindowKind) WindowOption {
	return func(w *Window) error {
		w.kind = kind
		return nil
	}
}

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

// TransparentWindowOption causes the window's framebuffer to be transparent.
func TransparentWindowOption() WindowOption {
	return func(w *Window) error {
		w.transparent = true
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

// FrontmostWindow returns the frontmost visible, non-transient window, or nil if there is none. Unlike ActiveWindow(),
// this does not require the window to currently hold the keyboard focus, making it a reliable anchor for positioning
// new windows and dialogs even at moments when no window has the focus (e.g. immediately after a menu window closes,
// but before the delayed focus notification for the underlying window has arrived).
func FrontmostWindow() *Window {
	for _, w := range windowList {
		if !w.transient && w.IsVisible() {
			return w
		}
	}
	return nil
}

// NewWindow creates a new, initially hidden, window. Call Show() or ToFront() to make it visible.
func NewWindow(title string, options ...WindowOption) (*Window, error) {
	w := &Window{
		wnd:            &apiWindow{},
		glCtx:          &apiGLContext{},
		title:          title,
		titleIcons:     DefaultTitleIcons,
		surface:        &surface{},
		pressedKeys:    make(map[KeyCode]bool),
		pressedButtons: make(map[int]bool),
	}
	for _, option := range options {
		if err := option(w); err != nil {
			return nil, err
		}
	}
	windowList = append(windowList, w)
	err := w.apiInit()
	if err == nil {
		err = w.glCtx.apiCreate(w)
	}
	if err != nil {
		w.apiDestroy()
		windowList = slices.DeleteFunc(windowList, func(wnd *Window) bool { return wnd == w })
		return nil, err
	}
	w.valid = true
	w.root = newRootPanel(w)
	w.ValidateLayout()
	w.SetTitleIcons(w.titleIcons)
	return w, nil
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
	SafeCall(w.MovedCallback)
}

func (w *Window) resized() {
	SafeCall(w.ResizedCallback)
	w.ValidateLayout()
}

func (w *Window) gainedFocus() {
	if !w.okToProcess() {
		// Deliberately do NOT bring the top modal window back to the front here. Input routing already ignores windows
		// that are blocked by a modal, so the modal does not need to hold the focus. Re-activating it on every focus
		// change creates a feedback loop under focus-follows-mouse window managers, where merely hovering over another
		// of this app's windows re-activates the modal; compositors that warp the pointer on activation (e.g. Hyprland)
		// then appear to trap the cursor inside the modal window.
		return
	}
	w.focused = true
	if len(windowList) != 0 && windowList[0] != w {
		windowList = slices.DeleteFunc(windowList, func(wnd *Window) bool { return wnd == w })
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
	SafeCall(w.GainedFocusCallback)
	w.mouseEnter(w.MouseLocation(), 0)
	if w.apiCursorInContentArea() {
		w.apiUpdateCursorImage()
	}
}

func (w *Window) lostFocus() {
	w.restoreHiddenCursor()
	w.focused = false
	w.ClearTooltip()
	if w.focus != nil {
		w.focus.MarkForRedraw()
	}
	SafeCall(w.LostFocusCallback)
	if w.root.menuBar != nil {
		w.root.menuBar.postLostFocus(w)
	}
	if len(w.pressedKeys) != 0 {
		keys := make([]KeyCode, 0, len(w.pressedKeys))
		for key := range w.pressedKeys {
			keys = append(keys, key)
		}
		for _, key := range keys {
			w.keyReleased(key, 0)
		}
	}
	w.synthesizeMouseUp()
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
		modalStack = slices.Delete(modalStack, i, i+1)
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

func (w *Window) requestClose() {
	if w.okToProcess() {
		w.AttemptClose()
	}
}

// AttemptClose closes the window if permitted. Returns true on success.
func (w *Window) AttemptClose() bool {
	if w.AllowCloseCallback != nil {
		allow := false
		SafeCall(func() { allow = w.AllowCloseCallback() })
		if !allow {
			return false
		}
	}
	w.Dispose()
	return true
}

// Dispose of the window.
func (w *Window) Dispose() {
	active := ActiveWindow()
	if active == w {
		w.ShowCursor()
	}
	SafeCall(w.WillCloseCallback)
	w.WillCloseCallback = nil
	if w.inModal {
		w.StopModal(ModalResponseDiscard)
	}
	if w.root.contentPanel != nil {
		w.root.contentPanel.RemoveFromParent()
	}
	if w.IsValid() {
		w.valid = false
		w.surface.dispose()
		w.destroy()
	}
	// Drop any pending redraw request, since a disposed window can never be drawn again. Without this, the window (and
	// its entire panel tree) would be retained in redrawSet for the life of the process.
	delete(redrawSet, w)
	if len(windowList) == 0 && quitAfterLastWindowClosed() {
		quitting()
	}
	if active != nil && active == w && len(windowList) != 0 {
		windowList[0].ToFront()
	}
}

func (w *Window) destroy() {
	if w == nil {
		return
	}
	if w == wndWithCurrentCtx {
		w.releaseGLCtxCurrent()
	}
	w.apiDestroy()
	windowList = slices.DeleteFunc(windowList, func(wnd *Window) bool { return wnd == w })
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
			w.apiSetTitle(title)
		}
	}
}

// TitleIcons the title icons that were previously set, if any.
func (w *Window) TitleIcons() []*Image {
	return w.titleIcons
}

// SetTitleIcons sets the title icon of the window. The image closest to the size desired by the system will be selected
// and used, scaling if needed. If no images are specified, the window reverts to its default icon.
//
// Note that macOS no longer has window icons, so this does nothing on that platform.
func (w *Window) SetTitleIcons(images []*Image) {
	if runtime.GOOS != xos.MacOS && w.IsValid() {
		w.titleIcons = make([]*Image, 0, len(images))
		imgs := make([]*image.NRGBA, 0, len(images))
		for _, img := range images {
			if nrgba, err := img.ToNRGBA(); err != nil {
				errs.Log(err)
			} else {
				w.titleIcons = append(w.titleIcons, img)
				imgs = append(imgs, nrgba)
			}
		}
		w.apiSetTitleIcons(imgs)
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
	w.root.SetFrameRect(w.LocalContentRect())
	w.root.ValidateLayout()
}

// Display returns the display that this window is currently on, or the primary display if the window is not valid.
func (w *Window) Display() *Display {
	if w.IsValid() {
		return w.apiDisplay()
	}
	return PrimaryDisplay()
}

// FrameRect returns the boundaries in display coordinates of the frame of this window (i.e. the area that includes both
// the content and its border and window controls).
func (w *Window) FrameRect() geom.Rect {
	if w.IsValid() {
		return w.apiFrameRect()
	}
	return geom.NewRect(0, 0, 1, 1)
}

// FrameRectForContentRect returns the frame rect for the given content rect.
func (w *Window) FrameRectForContentRect(contentRect geom.Rect) geom.Rect {
	if w.IsValid() {
		return w.apiFrameRectForContentRect(contentRect)
	}
	return contentRect
}

// SetFrameRect sets the boundaries of the frame of this window.
func (w *Window) SetFrameRect(rect geom.Rect) {
	w.SetContentRect(w.ContentRectForFrameRect(rect))
}

// EnsureOnDisplay moves the window fully onto a display if it is not already fully within the area of a display, trying
// to preserve its size if it is necessary to reposition it, though shrinking the window if necessary to fit.
func (w *Window) EnsureOnDisplay() {
	if w.IsValid() {
		w.apiEnsureOnDisplay()
	}
}

// ContentRect returns the boundaries in display coordinates of the window's content area.
func (w *Window) ContentRect() geom.Rect {
	if w.IsValid() {
		return w.apiContentRect()
	}
	return geom.NewRect(0, 0, 1, 1)
}

// ContentRectForFrameRect returns the content rect for the given frame rect.
func (w *Window) ContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	if w.IsValid() {
		return w.apiContentRectForFrameRect(frameRect)
	}
	return frameRect
}

// SetContentRect sets the boundaries of the frame of this window by converting the content rect into a suitable frame
// rect and then applying it to the window.
func (w *Window) SetContentRect(rect geom.Rect) {
	if w.IsValid() {
		rect = w.adjustContentRectForMinMax(rect)
		w.apiSetContentRect(rect)
	}
}

func (w *Window) minMaxContentSize() (minimum, maximum geom.Size) {
	if w.MinMaxContentSizeCallback != nil {
		SafeCall(func() { minimum, maximum = w.MinMaxContentSizeCallback() })
	} else {
		minimum, _, maximum = w.root.Sizes(geom.Size{})
	}
	return minimum, maximum
}

func (w *Window) adjustContentRectForMinMax(rect geom.Rect) geom.Rect {
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
func (w *Window) LocalContentRect() geom.Rect {
	r := w.ContentRect()
	r.X = 0
	r.Y = 0
	return r
}

// Pack sets the window's content size to match the preferred size of the root panel and forces it onto a display.
func (w *Window) Pack() {
	w.PackWithLocation(w.ContentRect().Point)
}

// PackWithDefaultInitialLocation sets the window's content size to match the preferred size of the root panel and
// forces it onto a display, trying to position the new window to the right of the currently active window. Failing
// that, position it at the top-left of the display's usable area.
func (w *Window) PackWithDefaultInitialLocation() {
	w.PackWithLocation(DefaultInitialWindowContentLocation())
}

// PackWithLocation sets the window's content size to match the preferred size of the root panel and attempts to use the
// provided point for its location, but will force it onto a display if needed.
func (w *Window) PackWithLocation(pt geom.Point) {
	_, pref, _ := w.root.Sizes(geom.Size{})
	w.SetFrameRect(w.FrameRectForContentRect(geom.Rect{
		Point: pt,
		Size:  pref,
	}))
	w.EnsureOnDisplay()
}

// MoveToModalCenter moves the window to the center (horizontally) and above center (vertically) of the other window.
// If the other window is nil, the frontmost window will be used in its place, so that the window opens on the same
// display the user is working on; only if no window is available will it be centered on the primary display. The
// window will be forced onto a display if needed.
func (w *Window) MoveToModalCenter(other *Window) {
	if other == nil {
		other = FrontmostWindow()
	}
	var within geom.Rect
	if other != nil && other != w {
		within = other.FrameRect()
	} else if d := PrimaryDisplay(); d != nil {
		within = d.Usable
	}
	wndFrame := w.FrameRect()
	within.Y += (within.Height - wndFrame.Height) / 3
	within.Height = wndFrame.Height
	within.X += (within.Width - wndFrame.Width) / 2
	within.Width = wndFrame.Width
	w.SetFrameRect(within.Align())
	w.EnsureOnDisplay()
}

// DefaultInitialWindowContentLocation selects an upper-left corner for a window by offsetting the current active
// window's position down and to the right. If there is no active window, the frontmost window will be used in its
// place, so that new windows open on the same display the user is working on. Only if no window is available will it
// return the upper-left corner of the primary display.
func DefaultInitialWindowContentLocation() geom.Point {
	w := ActiveWindow()
	if w == nil {
		w = FrontmostWindow()
	}
	if w != nil {
		r := w.ContentRect()
		r.X += 32
		r.Y += 32
		return r.Point
	}
	if d := PrimaryDisplay(); d != nil {
		return d.Usable.Point
	}
	return geom.Point{}
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
				SafeCall(oldFocus.LostFocusCallback)
			}
			w.focus = newFocus
			if newFocus != nil {
				SafeCall(newFocus.GainedFocusCallback)
			}
			w.notifyOfFocusChangeInHierarchy(oldFocus, newFocus)
		}
	}
}

func (w *Window) removeFocus() {
	oldFocus := w.focus
	if oldFocus != nil {
		SafeCall(oldFocus.LostFocusCallback)
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
					SafeCall(func() { p.FocusChangeInHierarchyCallback(oldFocus, newFocus) })
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
	return w.IsValid() && w.apiVisible()
}

// IsTransparent returns true if the window was created with a transparent backing buffer.
func (w *Window) IsTransparent() bool {
	return w.transparent
}

// Show makes the window visible, if it was previously hidden. If the window is already visible or is in full screen
// mode, this function does nothing.
func (w *Window) Show() {
	if w.IsValid() {
		w.apiShow()
	}
}

// Hide hides the window, if it was previously visible. If the window is already hidden or is in full screen mode, this
// function does nothing.
func (w *Window) Hide() {
	if w.IsValid() {
		w.apiHide()
	}
}

// ToFront attempts to bring the window to the foreground and give it the keyboard focus. If it is hidden, it will be
// made visible first. Does nothing for windows marked keepHidden.
func (w *Window) ToFront() {
	if w.IsValid() && !w.keepHidden {
		w.Show()
		w.focused = true // Don't wait for the focus event to set this, as Linux delays the notification too much
		w.apiAcquireFocusAndBringToFront()
	}
}

// IsMinimized returns true if the window is currently minimized.
func (w *Window) IsMinimized() bool {
	return w.IsValid() && w.minimized
}

// Minimize performs the minimize function on the window, or restores it if it is already minimized.
func (w *Window) Minimize() {
	if w.IsValid() {
		w.apiMinimize()
	}
}

// IsMaximized returns true if the window is currently maximized.
func (w *Window) IsMaximized() bool {
	return w.IsValid() && w.maximized
}

// Maximize performs the maximize function on the window, or restores it if it is already maximized.
func (w *Window) Maximize() {
	if w.IsValid() {
		w.apiMaximize()
	}
}

// Resizable returns true if the window can be resized by the user.
func (w *Window) Resizable() bool {
	return w.IsValid() && !w.notResizable
}

// MouseLocation returns the current mouse location relative to this window.
func (w *Window) MouseLocation() geom.Point {
	if w.IsValid() {
		return w.apiCursorPosition()
	}
	return geom.Point{}
}

func (w *Window) adjustToCursorChange() {
	if w.apiCursorInContentArea() {
		w.apiUpdateCursorImage()
	}
}

// BackingScale returns the scale of the backing store for this window.
func (w *Window) BackingScale() geom.Point {
	if w.IsValid() {
		return w.apiBackingScale()
	}
	return geom.NewPoint(1, 1)
}

func (w *Window) makeGLCtxCurrent() {
	w.glCtx.apiMakeCurrent()
	wndWithCurrentCtx = w
}

func (w *Window) releaseGLCtxCurrent() {
	w.glCtx.apiReleaseCurrent()
	wndWithCurrentCtx = nil
}

// Draw the window contents.
func (w *Window) Draw(c *Canvas) {
	if w.root != nil {
		SafeCall(func() {
			w.root.ValidateLayout()
			r := w.LocalContentRect()
			if !w.transparent {
				paint := ThemeSurface.Paint(c, r, paintstyle.Fill)
				c.DrawPaint(paint)
			}
			w.root.Draw(c, r)
		})
	}
}

func (w *Window) draw() {
	delete(redrawSet, w)
	RebuildDynamicColors()
	if w.IsValid() {
		scale := w.BackingScale()
		w.makeGLCtxCurrent()
		size := w.ContentRect().Size
		c, err := w.surface.prepareCanvas(size, scale)
		if err != nil {
			errs.Log(err, "size", size, "scale", scale)
			return
		}
		start := time.Now()
		c.Save()
		w.Draw(c)
		c.Restore()
		c.Flush()
		w.lastDrawDuration = time.Since(start)
		w.glCtx.apiSwapBuffers()
	}
}

// LastDrawDuration returns the duration of the window's most recent draw.
func (w *Window) LastDrawDuration() time.Duration {
	return w.lastDrawDuration
}

// MarkForRedraw marks this window for drawing at the next update. Does nothing if the window has been disposed.
func (w *Window) MarkForRedraw() {
	if !w.IsValid() {
		return
	}
	if _, exists := redrawSet[w]; !exists {
		redrawSet[w] = struct{}{}
		if len(redrawSet) == 1 {
			apiPostEmptyEvent()
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
	if w.IsValid() && !w.cursorHidden {
		w.cursorHidden = true
		w.updateCursorVisibility()
	}
}

// ShowCursor shows the cursor.
func (w *Window) ShowCursor() {
	if w.IsValid() && w.cursorHidden {
		w.cursorHidden = false
		w.updateCursorVisibility()
	}
}

func (w *Window) updateCursorVisibility() {
	if w.focused {
		if w.apiCursorInContentArea() {
			w.apiUpdateCursorImage()
		}
	}
}

// HideCursorUntilMouseMoves hides the cursor until the mouse is moved.
func (w *Window) HideCursorUntilMouseMoves() {
	if !w.cursorHiddenUntilMouseMoves {
		w.cursorHiddenUntilMouseMoves = true
		w.HideCursor()
	}
}

func (w *Window) restoreHiddenCursor() {
	if w.cursorHiddenUntilMouseMoves {
		w.cursorHiddenUntilMouseMoves = false
		w.ShowCursor()
	}
}

func (w *Window) updateTooltipAndCursor(target *Panel, where geom.Point) {
	w.updateCursor(target, where)
	w.updateTooltip(target, where)
}

func (w *Window) updateTooltip(target *Panel, where geom.Point) {
	var avoid geom.Rect
	var tip *Panel
	for target != nil {
		avoid = target.RectToRoot(target.ContentRect(true))
		avoid.Align()
		if target.UpdateTooltipCallback != nil {
			SafeCall(func() { avoid = target.UpdateTooltipCallback(target.PointFromRoot(where), avoid) })
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
	w.updateCursor(w.root.PanelAt(where), where)
}

func (w *Window) updateCursor(target *Panel, where geom.Point) {
	var cursor *Cursor
	for target != nil {
		if target.UpdateCursorCallback == nil {
			target = target.parent
		} else {
			SafeCall(func() { cursor = target.UpdateCursorCallback(target.PointFromRoot(where)) })
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
			w.adjustToCursorChange()
		}
	}
}

func (w *Window) mouseDown(where geom.Point, button int, mods mod.Modifiers) {
	if !w.okToProcess() {
		modalStack[len(modalStack)-1].mouseDown(where, button, mods)
		return
	}
	w.inMouseDown = true
	w.pressedButtons[button] = true
	maxDelay, maxMouseDrift := DoubleClickParameters()
	now := time.Now()
	if button == w.lastButton && time.Since(w.lastButtonTime) <= maxDelay &&
		xmath.Abs(where.X-w.firstButtonLocation.X) <= maxMouseDrift &&
		xmath.Abs(where.Y-w.firstButtonLocation.Y) <= maxMouseDrift {
		w.lastButtonCount++
		time.Since(w.lastButtonTime)
	} else {
		w.lastButtonCount = 1
		w.firstButtonLocation = where
	}
	w.lastButton = button
	w.lastButtonTime = now
	w.lastKeyModifiers = mods
	if w.root.preMouseDown(w, where) {
		return
	}
	if w.MouseDownCallback != nil {
		stop := false
		SafeCall(func() { stop = w.MouseDownCallback(where, button, w.lastButtonCount, mods) })
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
				SafeCall(func() {
					stop = panel.MouseDownCallback(panel.PointFromRoot(where), button, w.lastButtonCount, mods)
				})
				if stop {
					w.lastMouseDownPanel = panel
					return
				}
			}
			panel = panel.parent
		}
	}
}

func (w *Window) mouseDrag(where geom.Point, button int, mods mod.Modifiers) {
	w.lastKeyModifiers = mods
	w.dragDataLocation = where
	w.restoreHiddenCursor()
	if w.MouseDragCallback != nil {
		stop := false
		SafeCall(func() { stop = w.MouseDragCallback(where, button, mods) })
		if stop {
			return
		}
	}
	if w.lastMouseDownPanel != nil && w.lastMouseDownPanel.MouseDragCallback != nil && w.lastMouseDownPanel.Enabled() {
		SafeCall(func() {
			w.lastMouseDownPanel.MouseDragCallback(w.lastMouseDownPanel.PointFromRoot(where), button, mods)
		})
	}
}

func (w *Window) synthesizeMouseUp() {
	if len(w.pressedButtons) != 0 {
		buttons := make([]int, 0, len(w.pressedButtons))
		for button := range w.pressedButtons {
			buttons = append(buttons, button)
		}
		where := w.MouseLocation()
		for _, button := range buttons {
			w.mouseUp(where, button, 0)
		}
	}
}

func (w *Window) mouseUp(where geom.Point, button int, mods mod.Modifiers) {
	if !w.okToProcess() {
		modalStack[len(modalStack)-1].mouseUp(where, button, mods)
		return
	}
	if !w.inMouseDown {
		return
	}
	w.inMouseDown = false
	w.pressedButtons[button] = false
	w.lastButton = button
	w.lastKeyModifiers = mods
	if w.MouseUpCallback != nil {
		stop := false
		SafeCall(func() { stop = w.MouseUpCallback(where, button, mods) })
		if stop {
			return
		}
	}
	if w.lastMouseDownPanel != nil && w.lastMouseDownPanel.MouseUpCallback != nil && w.lastMouseDownPanel.Enabled() {
		SafeCall(func() {
			w.lastMouseDownPanel.MouseUpCallback(w.lastMouseDownPanel.PointFromRoot(where), button, mods)
		})
	}
	panel := w.root.PanelAt(where)
	if w.root != nil && !panel.Is(w.lastMouseOverPanel) {
		w.mouseExit()
	}
	w.updateCursor(panel, where)
	w.updateTooltip(w.lastMouseDownPanel, where)
	w.lastMouseDownPanel = nil
}

func (w *Window) mouseEnter(where geom.Point, mods mod.Modifiers) {
	w.lastKeyModifiers = mods
	w.restoreHiddenCursor()
	w.mouseExit()
	if w.MouseEnterCallback != nil {
		stop := false
		SafeCall(func() { stop = w.MouseEnterCallback(where, mods) })
		if stop {
			return
		}
	}
	panel := w.root.PanelAt(where)
	if panel.MouseEnterCallback != nil {
		SafeCall(func() { panel.MouseEnterCallback(panel.PointFromRoot(where), mods) })
	}
	w.updateTooltipAndCursor(panel, where)
	w.lastMouseOverPanel = panel
}

func (w *Window) mouseMovedOrDragged(where geom.Point, mods mod.Modifiers) {
	if w.inMouseDown {
		w.mouseDrag(where, w.lastButton, mods)
	} else {
		w.mouseMove(where, mods)
	}
}

func (w *Window) mouseMove(where geom.Point, mods mod.Modifiers) {
	w.lastKeyModifiers = mods
	w.restoreHiddenCursor()
	panel := w.root.PanelAt(where)
	if panel.Is(w.lastMouseOverPanel) {
		if w.MouseMoveCallback != nil {
			stop := false
			SafeCall(func() { stop = w.MouseMoveCallback(where, mods) })
			if stop {
				return
			}
		}
		if panel.MouseMoveCallback != nil {
			SafeCall(func() { panel.MouseMoveCallback(panel.PointFromRoot(where), mods) })
		}
		w.updateTooltipAndCursor(panel, where)
	} else {
		w.mouseEnter(where, mods)
	}
}

func (w *Window) mouseExit() {
	if w.MouseExitCallback != nil {
		stop := false
		SafeCall(func() { stop = w.MouseExitCallback() })
		if stop {
			return
		}
	}
	if w.lastMouseDownPanel == nil && w.lastMouseOverPanel != nil {
		if w.lastMouseOverPanel.MouseExitCallback != nil {
			SafeCall(func() { w.lastMouseOverPanel.MouseExitCallback() })
		}
		w.lastMouseOverPanel = nil
		w.cursor = nil
	}
}

func (w *Window) mouseWheel(where, delta geom.Point, mods mod.Modifiers) {
	// Deliberately not gated by okToProcess(). Platforms deliver wheel events to the window under the cursor rather
	// than the focused window, so scrolling a window blocked by a modal is both possible and desirable, as it only
	// adjusts the view and cannot trigger actions.
	w.lastKeyModifiers = mods
	if w.MouseWheelCallback != nil {
		stop := false
		SafeCall(func() { stop = w.MouseWheelCallback(where, delta, mods) })
		if stop {
			return
		}
	}
	panel := w.root.PanelAt(where)
	for panel != nil {
		if panel.Enabled() && panel.MouseWheelCallback != nil {
			stop := false
			SafeCall(func() { stop = panel.MouseWheelCallback(panel.PointFromRoot(where), delta, mods) })
			if stop {
				break
			}
		}
		panel = panel.parent
	}
	if w.inMouseDown && w.lastMouseDownPanel != nil {
		w.mouseDrag(where, w.lastButton, mods)
	} else {
		w.mouseMove(where, mods)
	}
}

func (w *Window) keyPressed(key KeyCode, mods mod.Modifiers) {
	if !w.okToProcess() {
		// A window blocked by a modal may still hold the platform focus (see the comment in gainedFocus), so route
		// keyboard input to the top modal window rather than processing it here.
		modalStack[len(modalStack)-1].keyPressed(key, mods)
		return
	}
	w.lastKeyModifiers = mods
	repeat := w.pressedKeys[key]
	w.pressedKeys[key] = true
	if w.root.preKeyDown(w, key, mods, repeat) {
		return
	}
	if w.KeyDownCallback != nil {
		stop := false
		SafeCall(func() { stop = w.KeyDownCallback(key, mods, repeat) })
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
				SafeCall(func() { stop = panel.KeyDownCallback(key, mods, repeat) })
				if stop {
					w.lastKeyDownPanel = panel
					return
				}
			}
			panel = panel.parent
		}
		if key == KeyTab && (mods&(mod.NonSticky&^mod.Shift)) == 0 {
			if mods.ShiftDown() {
				w.FocusPrevious()
			} else {
				w.FocusNext()
			}
		}
	}
}

func (w *Window) runeTyped(ch rune) {
	if !w.okToProcess() {
		// See the comment in keyPressed.
		modalStack[len(modalStack)-1].runeTyped(ch)
		return
	}
	if w.root.preRuneTyped(w, ch) {
		return
	}
	if w.RuneTypedCallback != nil {
		stop := false
		SafeCall(func() { stop = w.RuneTypedCallback(ch) })
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
				SafeCall(func() { stop = panel.RuneTypedCallback(ch) })
				if stop {
					w.lastKeyDownPanel = panel
					return
				}
			}
			panel = panel.parent
		}
	}
}

func (w *Window) keyReleased(key KeyCode, mods mod.Modifiers) {
	w.lastKeyModifiers = mods
	delete(w.pressedKeys, key)
	if !w.okToProcess() {
		// The matching key down was routed to the top modal window, so deliver the key up there as well. The
		// bookkeeping above is still done locally so that releases synthesized by lostFocus keep this window's pressed
		// key state clean.
		modalStack[len(modalStack)-1].keyReleased(key, mods)
		return
	}
	if w.root.preKeyUp(w, key, mods) {
		return
	}
	if w.KeyUpCallback != nil {
		stop := false
		SafeCall(func() { stop = w.KeyUpCallback(key, mods) })
		if stop {
			return
		}
	}
	if w.lastKeyDownPanel != nil && w.lastKeyDownPanel.KeyUpCallback != nil {
		SafeCall(func() { w.lastKeyDownPanel.KeyUpCallback(key, mods) })
	}
}

// CurrentKeyModifiers returns the current key modifiers, which is usually the same as calling .LastKeyModifiers(),
// however, on platforms that are using native menus, this will also capture modifier changes that occurred while the
// menu is being displayed.
func (w *Window) CurrentKeyModifiers() mod.Modifiers {
	return w.apiCurrentKeyModifiers()
}

// LastKeyModifiers returns the last set of key modifiers that this window has received.
func (w *Window) LastKeyModifiers() mod.Modifiers {
	return w.lastKeyModifiers
}

// ClientData returns a map of client data for this window.
func (w *Window) ClientData() map[string]any {
	if w.data == nil {
		w.data = make(map[string]any)
	}
	return w.data
}

// IsDragGesture returns true if a gesture to start a drag operation was made.
func (w *Window) IsDragGesture(where geom.Point) bool {
	minDelay, minMouseDrift := DragGestureParameters()
	return w.inMouseDown &&
		(xmath.Abs(w.firstButtonLocation.X-where.X) > minMouseDrift ||
			xmath.Abs(w.firstButtonLocation.Y-where.Y) > minMouseDrift ||
			time.Since(w.lastButtonTime) > minDelay)
}

// DragSpec describes a drag & drop operation to start.
type DragSpec struct {
	// Image is the drag image shown while dragging. May be nil.
	Image *Image
	// Cleanup is called when the drag source finishes, if not nil.
	Cleanup func()
	// Data holds the payload for the drag.
	Data []drag.Data
	// Origin is the origin of the drag image. For Panel.StartDrag it is in the panel's coordinate space; for
	// Window.StartDrag it is in the window's root coordinate space.
	Origin geom.Point
	// OpMask holds the permitted drag operations.
	OpMask drag.Op
}

// StartDrag starts a drag & drop operation. 'img' is the drag image shown while dragging and may be nil. 'origin' is
// the origin of the drag image in the window's root coordinate space. 'cleanup' is called when the drag source
// finishes, if not nil. 'opMask' holds the permitted drag operations.
func (w *Window) StartDrag(img *Image, origin geom.Point, cleanup func(), opMask drag.Op, data ...drag.Data) {
	if len(data) == 0 {
		return
	}
	w.synthesizeMouseUp()
	w.dragSourceCleanup = cleanup
	w.apiStartDrag(img, origin, opMask, data...)
}

func (w *Window) dragSourceFinished() {
	if w.dragSourceCleanup != nil {
		w.dragSourceCleanup()
	}
}

func (w *Window) findDropTarget(di drag.Info, where geom.Point) *Panel {
	if !w.okToProcess() {
		return nil
	}
	for panel := w.root.PanelAt(where); panel != nil; panel = panel.Parent() {
		if panel.DropCallback != nil && panel.Enabled() {
			accept := false
			if panel.CanAcceptDropCallback != nil {
				SafeCall(func() { accept = panel.CanAcceptDropCallback(di) })
			}
			if accept {
				return panel
			}
		}
	}
	return nil
}

func (w *Window) dragEntered(di drag.Info, where geom.Point, mods mod.Modifiers) drag.Op {
	op := drag.None
	panel := w.findDropTarget(di, where)
	if panel != nil {
		w.dragExitTarget()
		if panel.DragEnteredCallback != nil {
			SafeCall(func() { op = panel.DragEnteredCallback(di, panel.PointFromRoot(where), mods) })
		}
	}
	w.lastDropTarget = panel
	w.lastDragOp = op
	return op
}

func (w *Window) dragUpdate(di drag.Info, where geom.Point, mods mod.Modifiers) drag.Op {
	panel := w.findDropTarget(di, where)
	if panel == nil {
		w.dragExitTarget()
		return drag.None
	}
	if !panel.Is(w.lastDropTarget) {
		w.dragEntered(di, where, mods)
	}
	if panel.DragUpdatedCallback != nil {
		SafeCall(func() { w.lastDragOp = panel.DragUpdatedCallback(di, panel.PointFromRoot(where), mods) })
	}
	return w.lastDragOp
}

func (w *Window) drop(di drag.Info, where geom.Point, mods mod.Modifiers) bool {
	panel := w.findDropTarget(di, where)
	if panel == nil {
		w.dragExit()
		return false
	}
	handled := false
	SafeCall(func() { handled = panel.DropCallback(di, panel.PointFromRoot(where), mods) })
	w.lastDropTarget = nil
	w.inMouseDown = false
	w.dragFinish()
	return handled
}

func (w *Window) dragExit() {
	w.dragExitTarget()
	w.dragFinish()
}

func (w *Window) dragExitTarget() {
	if w.lastDropTarget == nil {
		return
	}
	target := w.lastDropTarget
	w.lastDropTarget = nil
	if !w.okToProcess() {
		return
	}
	if target.DragExitedCallback != nil {
		SafeCall(target.DragExitedCallback)
	}
}

func (w *Window) dragFinish() {
	w.inMouseDown = false
	w.adjustToCursorChange()
	w.FlushDrawing()
}

// RegisterForDragTypes registers the window as a potential target for drags of the specified types. Some platforms
// require this to be called before drag & drop will work within the window, while others ignore it.
func (w *Window) RegisterForDragTypes(types ...*uti.DataType) {
	previous := w.collectedRegisteredDragTypes()
	if w.dragTypes == nil {
		w.dragTypes = make(map[string]*uti.DataType)
	}
	for _, t := range types {
		w.dragTypes[t.UTI] = t
	}
	w.finishRegisteredDragTypesUpdate(previous)
}

// UnregisterForDragTypes unregisters the window as a potential target for drags of the specified types.
func (w *Window) UnregisterForDragTypes(types ...*uti.DataType) {
	previous := w.collectedRegisteredDragTypes()
	for _, t := range types {
		delete(w.dragTypes, t.UTI)
	}
	w.finishRegisteredDragTypesUpdate(previous)
}

// ClearRegisteredDragTypes unregisters the window as a potential target for drags of all types.
func (w *Window) ClearRegisteredDragTypes() {
	needUpdate := len(w.dragTypes) != 0
	w.dragTypes = nil
	if needUpdate {
		w.apiUpdateRegisteredDragTypes(nil)
	}
}

func (w *Window) collectedRegisteredDragTypes() []*uti.DataType {
	return slices.SortedFunc(maps.Values(w.dragTypes), func(a, b *uti.DataType) int {
		return strings.Compare(a.UTI, b.UTI)
	})
}

func (w *Window) finishRegisteredDragTypesUpdate(previous []*uti.DataType) {
	revised := w.collectedRegisteredDragTypes()
	if !slices.Equal(previous, revised) {
		w.apiUpdateRegisteredDragTypes(revised)
	}
}
