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
	// pendingFrontWindow is the window whose platform activation the most recent ToFront() asked for and that
	// flushPendingFront has yet to perform. UI thread only, like windowList. It may point at a window that has since
	// been disposed, so every reader checks IsValid() before acting on it.
	pendingFrontWindow *Window
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
	ContentScaleCallback func(scale geom.Point)
	wnd                  *apiWindow
	surface              *surface
	glCtx                *apiGLContext
	root                 *rootPanel
	ax                   *windowAccessibility
	focus                *Panel
	cursor               *Cursor
	lastDropTarget       *Panel
	dragSourceCleanup    func()
	lastMouseDownPanel   *Panel
	contextMenuPanel     *Panel
	lastMouseOverPanel   *Panel
	lastKeyDownPanel     *Panel
	lastTooltip          *Panel
	lastTooltipShownAt   time.Time
	lastButtonTime       time.Time
	pressedKeys          map[KeyCode]bool
	pressedButtons       map[int]bool
	data                 map[string]any
	dragTypes            map[string]*uti.DataType
	title                string
	titleIcons           []*Image
	lastDrawDuration     time.Duration
	tooltipSequence      int
	modalResultCode      int
	lastButton           int
	lastButtonCount      int
	contextMenuCount     int
	lastContentRect      geom.Rect
	firstButtonLocation  geom.Point
	dragDataLocation     geom.Point
	contextMenuPress     geom.Point
	lastWidth            float32
	lastHeight           float32
	lastKeyModifiers     mod.Modifiers
	contextMenuMods      mod.Modifiers
	// contextMenuChordKey is the key of the chord that opened an in-window contextual menu, until keyReleased sees its
	// key up, or KeyNone. It is not recorded for a native menu, whose tracking loop swallows the key up. The Windows
	// platform code keeps such an F10 from DefWindowProc, which would otherwise open the system menu on its release.
	contextMenuChordKey         KeyCode
	kind                        WindowKind
	lastDragOp                  drag.Op
	contextMenuReplay           bool
	rightPressTaken             bool
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
	dropRunes                   bool
	minimized                   bool
	maximized                   bool
	// axGeneration counts the accessibility snapshots taken of this window and is what accessibility.Tree.Generation
	// carries. It is held here rather than with the rest of the window's accessibility state, which is dropped when the
	// window is withdrawn from what an assistive technology holds (see Window.axWindowHidden), because the count must
	// not restart: the generation is what an adapter tells a stale tree from a current one by, so a window that is
	// hidden and shown again has to go on counting rather than hand out numbers it has already used. Last in the
	// struct, where it costs no more padding than it would among the other eight-byte fields. UI thread only.
	axGeneration uint64
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
// has the keyboard focus. A window whose activation has been requested with ToFront() but not yet performed by the
// event loop counts as active, since it will hold the focus by the time anything positioned relative to it appears.
func ActiveWindow() *Window {
	if w := pendingFrontWindow; w.IsValid() && !w.transient {
		return w
	}
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
	if w := pendingFrontWindow; w.IsValid() && !w.transient && w.IsVisible() {
		return w
	}
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
	if err == nil && !cpuRenderingActive {
		if glErr := w.glCtx.apiCreate(w); glErr != nil {
			fallbackToCPURendering(glErr)
		}
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
	if focus := w.CurrentFocus(); focus != nil {
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
	wasActive := ActiveWindow()
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
	// A window may become the active one without anything in it drawing differently — it may hold nothing that can take
	// the focus at all — and an assistive technology has to be told which window the person is now in, and which
	// window stopped being the active one, which need not be one that lost the focus.
	w.axMarkForPublish()
	axMarkActiveWindowChange(wasActive)
	SafeCall(w.GainedFocusCallback)
	w.mouseEnter(w.MouseLocation(), 0)
	if w.apiCursorInContentArea() {
		w.apiUpdateCursorImage()
	}
}

func (w *Window) lostFocus() {
	w.restoreHiddenCursor()
	wasActive := ActiveWindow()
	w.focused = false
	w.ClearTooltip()
	if w.focus != nil {
		w.focus.MarkForRedraw()
	}
	// As in gainedFocus: the window no longer being the active one is worth describing whether or not anything in it
	// looks any different for it, and so is whichever window is the active one now.
	w.axMarkForPublish()
	axMarkActiveWindowChange(wasActive)
	SafeCall(w.LostFocusCallback)
	w.root.postLostFocus(w)
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
	cancelPressesForModal(w)
	w.ToFront()
	for w.inModal {
		processEvents()
	}
	return w.modalResultCode
}

// cancelPressesForModal ends any mouse press still in progress in the other windows before a nested modal event loop
// takes over the thread. This is what Win32 does with WM_CANCELMODE before it runs a dialog's own message loop, and for
// the same reason: a modal is very often started from a click, so the press that led here was delivered to a window the
// modal is about to block, and the platform capture that press installed would otherwise go on routing every mouse
// message to that window for as long as the modal ran. The modal must already be on the modal stack when this is
// called, so that the synthesized releases take the bookkeeping-only path in mouseUp rather than being delivered to the
// panels, where a button under the pointer would fire its click again and recurse into RunModal.
func cancelPressesForModal(modal *Window) {
	for _, w := range slices.Clone(windowList) {
		if w != modal {
			w.synthesizeMouseUp()
			w.apiCancelMouseCapture()
		}
	}
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
	if w.ax != nil {
		// Before the platform window goes away, since an adapter's teardown talks to it. One nil check is all this
		// costs a window no assistive technology ever asked about.
		w.apiAccessibilityShutdown()
		w.ax = nil
	}
	if w == wndWithCurrentCtx {
		w.releaseGLCtxCurrent()
	}
	w.apiDestroy()
	windowList = slices.DeleteFunc(windowList, func(wnd *Window) bool { return wnd == w })
	if pendingFrontWindow == w {
		pendingFrontWindow = nil
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
			w.apiSetTitle(title)
			// The title is the accessible name of the window itself, and nothing about setting it repaints the window:
			// every platform's nativeSetTitle only talks to the window manager. Without this a screen reader would go
			// on reporting the name from before the title changed until something unrelated happened to redraw. See
			// Window.axMarkForPublish.
			w.axMarkForPublish()
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
// Note that macOS no longer has window icons, so this does nothing on that platform. A window belonging to a headless
// session is the exception: it records the icons wherever the test is running, since behaving the same way on every
// host is the point of such a session.
func (w *Window) SetTitleIcons(images []*Image) {
	if w.IsValid() && (runtime.GOOS != xos.MacOS || headlessWindowFor(w) != nil) {
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
		within = d.apiUsableInWindowUnits()
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

// CurrentFocus returns the panel that has the keyboard focus in this window. May return nil if no panel has the focus,
// or if the window is not valid. Unlike a call to .Focus(), this does not attempt to move the focus to a new panel if
// none currently has it.
func (w *Window) CurrentFocus() *Panel {
	if w == nil || w.focus == nil || w.focus.Window() != w {
		return nil
	}
	return w.focus
}

// Focus returns the panel with the keyboard focus in this window. If no panel currently has the focus, it will move the
// focus to the next focusable panel and return that. May return nil if no panel can be focused.
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
			// Where the focus is is the single most important thing an assistive technology is told, and a panel is
			// under no obligation to draw itself differently for holding it, so the window is described again whether
			// or not anything repainted. See Window.axMarkForPublish.
			w.axMarkForPublish()
		}
	}
}

func (w *Window) removeFocus() {
	oldFocus := w.focus
	if oldFocus != nil {
		SafeCall(oldFocus.LostFocusCallback)
		w.focus = nil
		w.notifyOfFocusChangeInHierarchy(oldFocus, nil)
		// The focus going nowhere has to be described as surely as it moving does. See Window.axMarkForPublish.
		w.axMarkForPublish()
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
//
// When nothing holds the focus yet, the first real tab stop is preferred over anything that can take the focus only
// for an assistive technology's sake, rather than simply taking the first panel that can take the focus at all. Those
// differ only while an assistive technology is being served, when a heading and a document can take the focus as well
// — see Panel.axTakesFocus. A dialog whose first element is a title heading or an explanatory document should still
// open with the person in its first field, where what they type goes somewhere. Only when there is no real tab stop
// does a document win: it is exactly where a screen reader has to begin, since with nothing in the window holding the
// focus Narrator's cursor stays on the window's own element, from which it will not move into the content. A window
// holding nothing but headings falls back to the first of them, since something in it has to be where a screen reader
// starts. See seedFocus, which Panel.FirstFocusableChild matches tier for tier.
func (w *Window) FocusNext() {
	if w.root.contentPanel != nil {
		current := w.focus
		seeding := current == nil
		if seeding {
			current = w.root.contentPanel
		}
		i, focusables := collectFocusables(w.root.contentPanel, current, nil)
		if len(focusables) > 0 {
			if seeding {
				current = seedFocus(dropSeedingCandidate(focusables, i), false)
			} else {
				i++
				if i >= len(focusables) {
					i = 0
				}
				current = focusables[i]
			}
		}
		w.SetFocus(current)
	}
}

// FocusPrevious moves the keyboard focus to the previous focusable panel. When nothing holds the focus yet, the same
// rule FocusNext documents decides where the person is put, applied from the other end.
func (w *Window) FocusPrevious() {
	if w.root.contentPanel != nil {
		current := w.focus
		seeding := current == nil
		if seeding {
			current = w.root.contentPanel
		}
		i, focusables := collectFocusables(w.root.contentPanel, current, nil)
		if len(focusables) > 0 {
			if seeding {
				current = seedFocus(dropSeedingCandidate(focusables, i), true)
			} else {
				i--
				if i < 0 {
					i = len(focusables) - 1
				}
				current = focusables[i]
			}
		}
		w.SetFocus(current)
	}
}

// dropSeedingCandidate returns the panels a window with nothing focused may seed the focus into, which is everything
// that can take it except the content panel itself at index i. The content panel is what the traversal started from,
// and an application that made it focusable meant it as the backstop the keyboard falls through to, not as the place
// the person is put when the window opens; the walk that a move from a real focus does passes over it for the same
// reason. It is kept when it is the only thing in the window that can take the focus at all, since the alternative is
// focusing nothing.
func dropSeedingCandidate(focusables []*Panel, i int) []*Panel {
	if i < 0 || len(focusables) < 2 {
		return focusables
	}
	return slices.Delete(focusables, i, i+1)
}

// seedFocus returns the panel a window with nothing focused hands the focus to, chosen from everything in it that can
// take the focus and scanned in the direction the move runs, so that FocusNext and FocusPrevious cannot drift apart on
// the rule. Panel.FirstFocusableChild and Panel.LastFocusableChild, which is how Window.SetFocus resolves a container,
// choose by the same rule within the subtree they are asked about.
//
// Three tiers are tried in turn. A panel that is a tab stop in its own right comes first: the keyboard belongs there,
// and a dialog whose first element is a Markdown explanation above its fields must still open with the person in the
// first field rather than in the document, where what they type would go nowhere. Next comes one that asked for the
// focus for an assistive technology's sake through Panel.axFocusable — a document, which is where a screen reader has
// to begin reading, and which is the right answer for a window that holds nothing but content. Last comes anything
// else, which is a heading that can take the focus only through the other arm of Panel.axTakesFocus; when there is
// nothing but headings, the first one the scan reaches is used anyway, since something in the window has to be where a
// screen reader starts. The scan records the best of the later tiers as it goes, so the list is walked once.
func seedFocus(focusables []*Panel, backward bool) *Panel {
	if len(focusables) == 0 {
		return nil
	}
	var reader *Panel
	for i := range focusables {
		p := focusables[i]
		if backward {
			p = focusables[len(focusables)-1-i]
		}
		if p.focusable {
			return p
		}
		if reader == nil && p.axFocusable {
			reader = p
		}
	}
	if reader != nil {
		return reader
	}
	if backward {
		return focusables[len(focusables)-1]
	}
	return focusables[0]
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
		if child.Hidden {
			continue
		}
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
		// As in Hide, but the other way round: a window that is back on the screen has to be described again, since a
		// platform that withdrew it while it was off the screen holds nothing about it at all. The mark is made after
		// the platform call rather than before it because Linux draws the window inline at the end of nativeShow, and
		// the filtered wait it does first discards the wake-up an earlier mark would have posted: marking here leaves
		// that inline draw to publish what it painted — every draw does, see Window.draw — and still has a redraw
		// pending afterwards, so a platform whose show paints nothing describes the window on the next pass instead.
		// See Window.axMarkForPublish.
		w.axMarkForPublish()
	}
}

// Hide hides the window, if it was previously visible. If the window is already hidden or is in full screen mode, this
// function does nothing.
func (w *Window) Hide() {
	if w.IsValid() {
		w.apiHide()
		// A window that has gone off the screen has to be taken out of what an assistive technology has been told, and
		// the event loop does that for the windows it finds it cannot draw. See Window.axMarkForPublish.
		w.axMarkForPublish()
	}
}

// ToFront attempts to bring the window to the foreground and give it the keyboard focus. If it is hidden, it will be
// made visible first. Does nothing for windows marked keepHidden.
//
// The window is shown immediately, but the platform activation is performed by the event loop at the end of the
// current pass rather than here, and repeated requests made within one pass collapse into the last one. Doing it here
// would perform the activation from inside whatever callback asked for it, which on some platforms is inside the
// dispatch of the very event that triggered it: the platform may then re-enter the focus callbacks on the spot, or
// finish its own handling of that event afterwards and undo the activation. Deferring also means that a sequence such
// as a menu closing (which fronts the window underneath) followed by a dialog opening activates only the dialog.
func (w *Window) ToFront() {
	if w.IsValid() && !w.keepHidden {
		w.Show()
		w.focused = true // Don't wait for the focus event to set this, as Linux delays the notification too much
		requestFront(w)
	}
}

// requestFront records w as the window to activate at the end of the current pass of the event loop, replacing any
// earlier request. The empty event wakes a loop that is about to block, such as the one RunModal is about to start,
// so that the activation is not left waiting on the next real event.
func requestFront(w *Window) {
	if pendingFrontWindow == w {
		return
	}
	pendingFrontWindow = w
	apiPostEmptyEvent()
}

// flushPendingFront performs the activation the most recent ToFront() asked for, if any. It is called by the event
// loop once per pass, after the windows have been drawn, so that a window is painted before it is raised. The request
// is cleared before the platform is called, so that a ToFront() made from the focus callbacks the activation provokes
// is a fresh request for the next pass rather than one that is lost.
func flushPendingFront() {
	w := pendingFrontWindow
	if w == nil {
		return
	}
	pendingFrontWindow = nil
	if !w.IsValid() || w.keepHidden {
		return
	}
	w.apiAcquireFocusAndBringToFront()
}

// IsMinimized returns true if the window is currently minimized.
func (w *Window) IsMinimized() bool {
	return w.IsValid() && w.minimized
}

// Minimize performs the minimize function on the window, or restores it if it is already minimized.
func (w *Window) Minimize() {
	if w.IsValid() {
		w.apiMinimize()
		// As in Hide: a window that has just been minimized, or restored, is described again rather than being left as
		// whatever it was last said to be.
		w.axMarkForPublish()
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

// draw paints the window and then describes what it painted to an assistive technology.
//
// The publish lives here rather than at the event loop's redraw call site because that is not the only path that
// paints a window: Window.FlushDrawing draws on the spot, and so does every platform's own paint notification —
// WM_PAINT on Windows, AppKit's update and redraw callbacks on macOS, the X11 Expose and the inline draw at the end of
// nativeShow on Linux. Each of them begins by taking the window out of redrawSet, so a description that was riding on
// that pending redraw — a name that changed, a panel that came or went — would simply be dropped by any of them if
// only the event loop published. It is gated on the window being on the screen, as the event loop's own call site is,
// because FlushDrawing will paint a window that is hidden or minimized. An application no assistive technology is
// watching pays one atomic load for each window it draws and nothing else.
func (w *Window) draw() {
	delete(redrawSet, w)
	RebuildDynamicColors()
	if w.IsValid() {
		scale := w.BackingScale()
		if w.usesGLRendering() {
			w.makeGLCtxCurrent()
		}
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
		if pixels := w.surface.rasterPixmap(); pixels != nil {
			// The window may have a live GL context even though rendering fell back to the CPU (the fallback was
			// triggered while preparing this window's canvas). Destroy it so it cannot obscure the CPU-rendered
			// content.
			w.discardGLCtx()
			w.apiPresentCPUPixels(pixels)
		} else {
			w.glCtx.apiSwapBuffers()
		}
		if accessibilityActive.Load() {
			if w.IsVisible() {
				// Last, and only for a draw that got as far as putting something on the screen, so that what is
				// described is what was just painted. The description is built from the panels rather than from the
				// canvas, so nothing here depends on the rendering state this leaves behind, and a panel that marks
				// itself for redraw while being described is simply drawn again on the next pass, exactly as one that
				// does it from its DrawCallback is.
				w.publishAccessibility()
			} else {
				// A window that is not on the screen is described no more, whatever painted it. The event loop never
				// reaches here for one — it withdraws the window instead, see finishProcessingEvents — but
				// Window.FlushDrawing draws whatever is in redrawSet with no visibility test of its own, and a window
				// that is hidden or minimized is a permanent resident of that set. Publishing from there would put back
				// the description that was just withdrawn and leave it standing, which on AT-SPI, where the application
				// lists its own windows, is a window a person cannot see reported as showing and visible.
				//
				// The redraw goes back so that the withdrawal branch sees the window again on the next pass, since the
				// delete above has just taken it out of the set the branch works from, and so that the window is drawn
				// and described afresh when it is shown again.
				redrawSet[w] = struct{}{}
			}
		}
	}
}

// usesGLRendering returns true if this window's drawing should go through its OpenGL context. Windows created before a
// CPU-rendering fallback keep using their existing GL rendering surface; windows created after it never get one.
func (w *Window) usesGLRendering() bool {
	return !cpuRenderingActive || w.surface.context != nil
}

// discardGLCtx destroys the window's OpenGL context, if any, releasing it first if it is the current one.
func (w *Window) discardGLCtx() {
	if wndWithCurrentCtx == w {
		w.releaseGLCtxCurrent()
	}
	w.glCtx.apiDestroy()
}

// LastDrawDuration returns the duration of the window's most recent draw.
func (w *Window) LastDrawDuration() time.Duration {
	return w.lastDrawDuration
}

// MarkForRedraw marks this window for drawing at the next update. Does nothing if the window has been disposed.
//
// The event loop may be blocked waiting for something to happen, so a request made when nothing else is going on has
// to wake it, or the window would sit unpainted until an unrelated event arrived. One wake-up is posted per pass of
// the loop: redrawWakePending records that this pass has already asked for the next one, and finishProcessingEvents
// clears it as each pass begins. Whether the window was already in the set says nothing about whether a wake-up is
// coming — a window that is valid but not on the screen is put straight back into redrawSet by every pass and is
// therefore a permanent resident of it, so a request that finds one there is still a request nothing has posted for.
//
// X11 is the one place where a posted wake-up can be thrown away rather than delivered: a filtered wait — what shows a
// window, transfers a selection or asks the window manager for a window's frame — drains the event queue looking for
// the event it is after and discards the wake-up token along with everything else it is not interested in. Every call
// that can enter one of those waits repairs the bookkeeping on its way out, so that the flag never goes on claiming a
// wake-up that nothing will deliver. See x11FilteredWaitDone.
func (w *Window) MarkForRedraw() {
	if !w.IsValid() {
		return
	}
	redrawSet[w] = struct{}{}
	if !redrawWakePending {
		redrawWakePending = true
		apiPostEmptyEvent()
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
		avoid = target.RectToRoot(target.ContentRect(true)).Align()
		if target.UpdateTooltipCallback != nil {
			SafeCall(func() { avoid = target.UpdateTooltipCallback(target.PointFromRoot(where), avoid) })
		}
		// A tooltip the panel has borrowed on behalf of something that is not a panel of its own — the table cell or
		// the column header the pointer is over — comes first, since that is what is actually under the pointer. See
		// Panel.borrowedTooltip.
		if tip = target.borrowedTooltip; tip == nil {
			tip = target.Tooltip
		}
		if tip != nil {
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
		// Unlike keyboard events, mouse events are positional: the coordinates are in this window's space and would
		// land on arbitrary panels if rerouted to the top modal window, so a window blocked by a modal simply ignores
		// mouse presses.
		return
	}
	w.inMouseDown = true
	w.pressedButtons[button] = true
	if button == ButtonRight && len(w.pressedButtons) > 1 && (w.focused || w.transient) &&
		w.contextMenuOfferedAt(where) {
		// A right press made while another button is down is not a right-click. Over a panel with a contextual menu it
		// is ignored entirely: not counted as down, so mouseUp drops its release, nor as a click, so drags keep
		// reporting the button that began the gesture, and not told to the window's callbacks. contextMenuOfferedAt is
		// asked rather than contextMenuOwnerAt, which may move the focus.
		delete(w.pressedButtons, ButtonRight)
		return
	}
	// Any press ends a right-click still waiting to open a menu. A right press is delivered unless takeContextMenuPress
	// takes it below.
	w.forgetContextMenuPress()
	if button == ButtonRight {
		w.rightPressTaken = false
	}
	maxDelay, maxMouseDrift := DoubleClickParameters()
	now := time.Now()
	if button == w.lastButton && time.Since(w.lastButtonTime) <= maxDelay &&
		xmath.Abs(where.X-w.firstButtonLocation.X) <= maxMouseDrift &&
		xmath.Abs(where.Y-w.firstButtonLocation.Y) <= maxMouseDrift {
		w.lastButtonCount++
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
		if button == ButtonRight && len(w.pressedButtons) == 1 {
			// A right-click over a panel with a contextual menu is taken for that menu whatever its click count. One
			// made with another button down is not passed to contextMenuOwnerAt, which may ready an owner for a menu
			// that will not open.
			if owner := w.contextMenuOwnerAt(where); owner != nil {
				w.takeContextMenuPress(owner, where, mods)
				return
			}
		}
		w.dispatchMouseDown(where, button, w.lastButtonCount, mods)
	}
}

// dispatchMouseDown offers a press to the panel under it and then its ancestors until an enabled one's
// MouseDownCallback claims it; that panel then receives the drags and the release. mouseDrag also uses it to hand back
// a right press taken for a contextual menu once it becomes a drag. A panel that claims a press its own callback has
// already ended, by calling Panel.ShowContextMenu, is not recorded as holding it: the release is already spent, and
// recording it would leave the window believing a press was in progress until the next one.
func (w *Window) dispatchMouseDown(where geom.Point, button, count int, mods mod.Modifiers) {
	w.lastMouseDownPanel = nil
	panel := w.root.PanelAt(where)
	for panel != nil {
		if panel.MouseDownCallback != nil && panel.Enabled() {
			stop := false
			SafeCall(func() { stop = panel.MouseDownCallback(panel.PointFromRoot(where), button, count, mods) })
			if stop {
				if w.pressedButtons[button] {
					w.lastMouseDownPanel = panel
				}
				return
			}
		}
		panel = panel.parent
	}
}

// contextMenuOwnerAt returns the panel whose contextual menu a right press at the given window position is for, or nil.
// An enabled contextMenuOwnerResolver under the pointer is asked first, since PanelAt cannot see the panels it draws
// without holding them as children; otherwise it is the panel under the pointer, when that panel itself offers a menu.
// Its ancestors are never considered in its place. Asking a resolver may ready the owner, by giving it the focus. There
// is no owner in a window other than ActiveWindow(), where a popup menu opens (see axMayPopupMenu): taking the press
// for a menu that could not open would swallow the click.
func (w *Window) contextMenuOwnerAt(where geom.Point) *Panel {
	if w != ActiveWindow() {
		return nil
	}
	panel := w.root.PanelAt(where)
	if panel == nil {
		return nil
	}
	if resolver, ok := panel.Self.(contextMenuOwnerResolver); ok && panel.Enabled() {
		var owner *Panel
		SafeCall(func() { owner = resolver.contextMenuOwnerWithin(panel.PointFromRoot(where)) })
		if owner != nil {
			return owner
		}
	}
	if panel.offersContextMenu() {
		return panel
	}
	return nil
}

// contextMenuOfferedAt reports whether contextMenuOwnerAt would find an owner at the given window position, without
// readying anything for a menu (finding one in a Table cell gives its widget the focus).
func (w *Window) contextMenuOfferedAt(where geom.Point) bool {
	if w != ActiveWindow() {
		return false
	}
	panel := w.root.PanelAt(where)
	if panel == nil {
		return false
	}
	if resolver, ok := panel.Self.(contextMenuOwnerResolver); ok && panel.Enabled() {
		offered := false
		SafeCall(func() { offered = resolver.offersContextMenuWithin(panel.PointFromRoot(where)) })
		if offered {
			return true
		}
	}
	return panel.offersContextMenu()
}

// focusedContextMenuOwner returns the panel holding the keyboard focus when it offers a contextual menu and this is
// the active window, where the menu would open, or nil: in any other window, a press held on the panel would otherwise
// be ended and the panel scrolled to its anchor for a menu that Panel.ShowContextMenu then refuses. A widget in a Table
// cell holds the focus itself, so no resolver is needed here.
func (w *Window) focusedContextMenuOwner() *Panel {
	if w != ActiveWindow() {
		return nil
	}
	if focus := w.CurrentFocus(); focus != nil && focus.offersContextMenu() {
		return focus
	}
	return nil
}

// endPressesForContextMenu ends any mouse press in progress and gives up the pointer capture. It must be called before
// opening a contextual menu other than on a right-click's release, since a native menu's tracking loop swallows the
// release of any button still held, leaving the window believing that button is down. The releases are delivered at
// offPanelPoint, outside every panel, so that the panel holding a press ends its gesture without the release counting
// as a click the person never made; a taken right press opens no menu. A widget that readies itself for a menu, a table
// selecting a row, calls this before doing so, since a release delivered to it may change what it is about to ready.
func (w *Window) endPressesForContextMenu() {
	if w.inMouseDown {
		w.synthesizeMouseUpAt(offPanelPoint)
		w.apiCancelMouseCapture()
	}
}

// offPanelPoint is a position, in window coordinates, that lies outside every panel of any window, since a root panel
// sits at the origin and nothing within it reaches that far up and to the left.
var offPanelPoint = geom.NewPoint(-1e6, -1e6)

// takeContextMenuPress takes a right-click with no other button down, over owner, for owner's contextual menu: no
// panel is sent the press, the drags or the release, unless mouseDrag hands the press back as a drag. The owner's
// ContextMenuPressHandler runs before the window focuses the owner, so that a widget that scrolls itself into view on
// gaining the focus can take it first without scrolling. Pressing another button while this one is held ends the
// right-click, but the right press stays taken, so its release still reaches no panel.
func (w *Window) takeContextMenuPress(owner *Panel, where geom.Point, mods mod.Modifiers) {
	w.rightPressTaken = true
	w.lastMouseDownPanel = nil
	replay := false
	if handler, ok := owner.Self.(ContextMenuPressHandler); ok {
		local := owner.PointFromRoot(where)
		SafeCall(func() { replay = handler.ContextMenuPressed(local, mods) })
	}
	if owner.Focusable() {
		owner.RequestFocus()
	}
	if !w.pressedButtons[ButtonRight] {
		// The handler or the focus change ended the press (a modal opened, or the window lost the focus), so there is
		// no right-click left to wait on.
		return
	}
	w.contextMenuPanel = owner
	w.contextMenuPress = where
	w.contextMenuCount = w.lastButtonCount
	w.contextMenuMods = mods
	w.contextMenuReplay = replay
}

// forgetContextMenuPress discards a right-click waiting for its release to open a contextual menu. It leaves
// rightPressTaken alone, so the release of a taken press still reaches no panel.
func (w *Window) forgetContextMenuPress() {
	w.contextMenuPanel = nil
	w.contextMenuPress = geom.Point{}
	w.contextMenuCount = 0
	w.contextMenuMods = 0
	w.contextMenuReplay = false
}

func (w *Window) mouseDrag(where geom.Point, button int, mods mod.Modifiers) {
	w.lastKeyModifiers = mods
	w.dragDataLocation = where
	w.restoreHiddenCursor()
	if w.contextMenuPanel != nil {
		// A right-click that moves beyond the drift is a drag and opens no menu. The drift is measured here rather than
		// with IsDragGesture, which also counts a press held still long enough, and a slow right-click is still one.
		_, minMouseDrift := DragGestureParameters()
		if press := w.contextMenuPress; xmath.Abs(press.X-where.X) > minMouseDrift ||
			xmath.Abs(press.Y-where.Y) > minMouseDrift {
			owner := w.contextMenuPanel
			count, pressMods, replay := w.contextMenuCount, w.contextMenuMods, w.contextMenuReplay
			w.forgetContextMenuPress()
			if replay && owner.Window() == w && owner.Enabled() && w.contextMenuOwnerUnder(owner, press) {
				// The owner asked for the press back (see ContextMenuPressHandler): deliver it as it was made, no
				// longer taken so that its release is delivered too, and let this move carry on below as its first
				// drag. Not when the owner has since been removed, disabled or moved from under the press, since the
				// press would then reach a panel that was never offered it.
				w.rightPressTaken = false
				w.dispatchMouseDown(press, ButtonRight, count, pressMods)
			}
		}
	}
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

// contextMenuOwnerUnder reports whether the owner of a taken right press is still under the given window position: the
// panel under it is the owner or within the owner, or is a widget that draws the owner without holding it as a child (a
// Table with the owner in one of its cells) and every panel from the owner up to that widget is visible and holds the
// position within its own bounds. PanelAt descends only into children, so for a borrowed cell widget it answers with
// the table, and a press handed back then reaches the widget through the table's own forwarding into the cell. The
// walk up from the owner tells that apart from an owner the window cannot see any more because a container between it
// and the panel under the position was hidden, or shrank or scrolled the position out of it, which the owner's own
// Hidden flag and bounds say nothing about.
func (w *Window) contextMenuOwnerUnder(owner *Panel, where geom.Point) bool {
	hit := w.root.PanelAt(where)
	if panelContains(owner, hit) {
		return true
	}
	if hit == nil || !panelContains(hit, owner) {
		return false
	}
	for p := owner; p != hit; p = p.parent {
		if p.Hidden || !p.PointFromRoot(where).In(p.ContentRect(true)) {
			return false
		}
	}
	return true
}

func (w *Window) synthesizeMouseUp() {
	w.synthesizeMouseUpAt(w.MouseLocation())
}

// synthesizeMouseUpAt releases every button the window believes is down, as if at the given window position.
func (w *Window) synthesizeMouseUpAt(where geom.Point) {
	// A synthesized release must not open the contextual menu a right press was waiting to open.
	w.forgetContextMenuPress()
	if len(w.pressedButtons) != 0 {
		buttons := make([]int, 0, len(w.pressedButtons))
		for button := range w.pressedButtons {
			buttons = append(buttons, button)
		}
		for _, button := range buttons {
			w.mouseUp(where, button, 0)
		}
	}
}

func (w *Window) mouseUp(where geom.Point, button int, mods mod.Modifiers) {
	if !w.pressedButtons[button] {
		return
	}
	delete(w.pressedButtons, button)
	w.inMouseDown = len(w.pressedButtons) != 0
	// Cleared before any early return below, so that no menu is left waiting past this release.
	menuOwner := w.contextMenuPanel
	w.forgetContextMenuPress()
	// The release of a taken right press reaches no panel, since none was sent the press, and does not become
	// lastButton, so drags of a button still down keep reporting that button.
	taken := button == ButtonRight && w.rightPressTaken
	if button == ButtonRight {
		w.rightPressTaken = false
	}
	// A gesture goes on while any of its buttons is down. A taken right press is part of none, so a gesture begun while
	// it was held ends on the release of its own button, whether or not the right button is still down.
	takenRightDown := w.rightPressTaken && w.pressedButtons[ButtonRight]
	gestureGoesOn := len(w.pressedButtons) > 1 || (len(w.pressedButtons) == 1 && !takenRightDown)
	if !w.okToProcess() {
		// Delivery is suppressed while blocked by a modal (and, unlike key events, not rerouted to the modal, since
		// mouse events are positional — see the comment in mouseDown), but the bookkeeping above must still happen so
		// that a release arriving while blocked (e.g. one synthesized by lostFocus when a modal opens mid-press)
		// cannot leave stale pressed-button state behind. Otherwise mouseMovedOrDragged, which has no modal gate,
		// would keep feeding drag events to lastMouseDownPanel for the modal's entire lifetime.
		if !gestureGoesOn {
			w.lastMouseDownPanel = nil
		}
		return
	}
	if !taken {
		w.lastButton = button
	}
	w.lastKeyModifiers = mods
	if w.MouseUpCallback != nil {
		stop := false
		SafeCall(func() { stop = w.MouseUpCallback(where, button, mods) })
		if stop {
			return
		}
	}
	if !taken && w.lastMouseDownPanel != nil && w.lastMouseDownPanel.MouseUpCallback != nil &&
		w.lastMouseDownPanel.Enabled() {
		SafeCall(func() {
			w.lastMouseDownPanel.MouseUpCallback(w.lastMouseDownPanel.PointFromRoot(where), button, mods)
		})
	}
	if gestureGoesOn {
		// Other buttons are still down, so the drag in progress continues and the panel it is targeting must keep
		// receiving events until the last of its buttons is released.
		return
	}
	at := where
	if where == offPanelPoint {
		// A release endPressesForContextMenu delivers outside every panel brings the hover state up to date at the
		// pointer itself, which may have been dragged since the press: mouseDrag leaves the panel under it and the
		// cursor as they were at the press, and a platform withholds its exit while a button is held (Windows drops
		// WM_MOUSELEAVE while the mouse is captured), so a press dragged off the panel it began on is only noticed
		// here.
		at = w.MouseLocation()
	}
	panel := w.root.PanelAt(at)
	if !panel.Is(w.lastMouseOverPanel) {
		w.mouseExit()
	}
	w.updateCursor(panel, at)
	w.updateTooltip(w.lastMouseDownPanel, at)
	w.lastMouseDownPanel = nil
	// The menu opens last, since a native menu does not return until dismissed, and on the release rather than the
	// press, since a native menu's tracking loop would swallow the release. A release the owner is no longer under
	// cancels it; see contextMenuOwnerUnder.
	if button == ButtonRight && menuOwner != nil && menuOwner.Window() == w && menuOwner.Enabled() &&
		w.contextMenuOwnerUnder(menuOwner, where) {
		menuOwner.ShowContextMenu(menuOwner.PointFromRoot(where))
	}
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
	w.dropRunes = false
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
	if !repeat && isContextMenuKey(key, mods) {
		// The Menu key and shift+F10 open the contextual menu of the panel holding the focus. This comes before the
		// panels are offered the key, since a Field takes nearly every key and a fallback after the walk below would
		// never be reached while one held the focus.
		if owner := w.focusedContextMenuOwner(); owner != nil {
			if w.inMouseDown {
				// A held press is ended before the owner is asked for its menu, since a widget readies itself for the
				// menu before building it and a release delivered after that could undo it; the release may move the
				// focus, so the owner is looked up again. A callback that then has nothing to offer has still ended the
				// press, and the chord goes on to the owner as an ordinary key.
				w.endPressesForContextMenu()
				owner = w.focusedContextMenuOwner()
			}
			if owner != nil && owner.ShowContextMenu(owner.contextMenuAnchor()) {
				if len(w.root.openMenuPanels) == 0 {
					// Only a native menu is gone by now, and its tracking loop swallowed the key up. Forget the key so
					// the next press of the chord is not taken for a repeat; keyReleased drops a late key up.
					delete(w.pressedKeys, key)
				} else {
					w.contextMenuChordKey = key
				}
				// lastKeyDownPanel stays nil so that the key ups of the chord, and of keys an in-window menu consumes,
				// reach no panel that missed their key downs.
				return
			}
		}
	}
	if focus := w.CurrentFocus(); focus != nil {
		panel := focus
		w.lastKeyDownPanel = panel
		for panel != nil {
			if panel.Enabled() && panel.KeyDownCallback != nil {
				stop := false
				SafeCall(func() { stop = panel.KeyDownCallback(key, mods, repeat) })
				if stop {
					w.lastKeyDownPanel = panel
					// A panel that took the key and moved the focus elsewhere in doing so -- a table opening its
					// selection in a new editor in response to Space, say -- has used up the keystroke. The platforms
					// deliver the runes a key produces after its key down, so without this they would land in whatever
					// now holds the focus, typically a text field that would then replace its content with a space the
					// person never meant for it. The next key down or key up ends the suppression.
					w.dropRunes = w.CurrentFocus() != focus
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
	if w.dropRunes {
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
	if focus := w.CurrentFocus(); focus != nil {
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
	w.dropRunes = false
	if key == w.contextMenuChordKey {
		w.contextMenuChordKey = KeyNone
	}
	pressed := w.pressedKeys[key]
	delete(w.pressedKeys, key)
	if !w.okToProcess() {
		// The matching key down was routed to the top modal window, so deliver the key up there as well. The
		// bookkeeping above is still done locally so that releases synthesized by lostFocus keep this window's pressed
		// key state clean.
		modalStack[len(modalStack)-1].keyReleased(key, mods)
		return
	}
	if !pressed {
		// No matching key down was seen here — e.g. a release forwarded by a blocked window's lostFocus for a key
		// that was pressed before the modal opened — so there is nothing to deliver.
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
			accept := panel.CanAcceptDropCallback == nil
			if !accept {
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
	clear(w.pressedButtons)
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
