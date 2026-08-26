// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"image"
	"image/color"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// These tests drive real widgets the way a person would: they run the application's own event loop against an in-memory
// screen, click and type at it, and read back what the widgets made of it. Nothing here needs a display, which is what
// lets them run on a build machine with no windowing system. A session owns most of the package's mutable globals while
// it runs, so none of them may call t.Parallel.

// startHeadless starts a session and arranges for it to be shut down when the test ends, however it ends.
func startHeadless(t *testing.T, cfg unison.HeadlessConfig, options ...unison.StartupOption) *unison.HeadlessScreen {
	t.Helper()
	screen, err := unison.StartHeadless(cfg, options...)
	if err != nil {
		t.Fatalf("unable to start headless session: %v", err)
	}
	t.Cleanup(screen.Stop)
	return screen
}

// newHeadlessWindow creates a window holding the given panel, sized and positioned as asked, and shows it. The panel
// fills the window's content area, so a click at the panel's center is a click at the window's center.
func newHeadlessWindow(t *testing.T, title string, rect geom.Rect, panel unison.Paneler) *unison.Window {
	t.Helper()
	w, err := unison.NewWindow(title)
	if err != nil {
		t.Errorf("unable to create window %q: %v", title, err)
		return nil
	}
	content := w.Content()
	content.SetLayout(&unison.FlexLayout{
		Columns:  1,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	panel.AsPanel().SetLayoutData(&unison.FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Fill,
		HGrab:  true,
		VGrab:  true,
	})
	content.AddChild(panel)
	w.SetContentRect(rect)
	w.Show()
	return w
}

// TestHeadlessButtonClick verifies the whole path from an injected click to a widget's callback: the press is routed to
// the window under it, that window takes the focus, and the button it lands on reports exactly one click.
func TestHeadlessButtonClick(t *testing.T) {
	c := check.New(t)
	var button *unison.Button
	var clicks int
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			button = unison.NewButton()
			button.SetTitle("Press Me")
			button.ClickCallback = func() { clicks++ }
			newHeadlessWindow(t, "button", geom.NewRect(20, 20, 200, 80), button)
		}))
	c.NotNil(button)

	screen.Click(screen.PanelCenter(button))
	var clicked int
	c.True(screen.Do(func() { clicked = clicks }))
	c.Equal(1, clicked, "the button should have been clicked exactly once")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessFieldTyping verifies that typed text reaches the focused field and that the keys which are not text are
// delivered as the keys they are.
func TestHeadlessFieldTyping(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			newHeadlessWindow(t, "field", geom.NewRect(20, 20, 240, 60), field)
		}))
	c.NotNil(field)

	// Click the field to focus it, exactly as a person would, rather than reaching in and setting the focus.
	screen.Click(screen.PanelCenter(field))
	var focused bool
	c.True(screen.Do(func() { focused = field.Focused() }))
	c.True(focused, "clicking the field should have given it the focus")

	screen.Type("hello")
	var text string
	c.True(screen.Do(func() { text = field.Text() }))
	c.Equal("hello", text)

	screen.KeyPress(unison.KeyBackspace, 0)
	c.True(screen.Do(func() { text = field.Text() }))
	c.Equal("hell", text, "backspace should have deleted the last rune rather than typing one")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessModalDialog verifies that a nested modal event loop works: the click that opens the dialog does not
// return until the application has gone quiet inside that loop, the driver can go on injecting input while it is
// running, and dismissing the dialog unwinds everything back to where it started.
func TestHeadlessModalDialog(t *testing.T) {
	c := check.New(t)
	var button *unison.Button
	var main, dialogWnd *unison.Window
	result := unison.ModalResponseDiscard
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			button = unison.NewButton()
			button.SetTitle("Ask")
			button.ClickCallback = func() {
				d, err := unison.NewDialog(nil, nil, unison.NewMessagePanel("Proceed?", ""),
					[]*unison.DialogButtonInfo{unison.NewCancelButtonInfo(), unison.NewOKButtonInfo()})
				if err != nil {
					t.Errorf("unable to create dialog: %v", err)
					return
				}
				dialogWnd = d.Window()
				// Does not return until the dialog is dismissed, which is several driver calls from now.
				result = d.RunModal()
			}
			main = newHeadlessWindow(t, "main", geom.NewRect(50, 50, 300, 150), button)
		}))
	c.NotNil(button)
	c.NotNil(main)

	// Returns once the application is idle inside the dialog's modal loop rather than once the click's handler has
	// returned, since that handler will not return until the dialog is gone.
	screen.Click(screen.PanelCenter(button))
	var windows int
	c.True(screen.Do(func() { windows = len(unison.Windows()) }))
	c.Equal(2, windows, "the dialog should be open alongside the window that opened it")
	c.NotNil(dialogWnd)
	c.True(screen.FocusedWindow() == dialogWnd, "the dialog should hold the focus while it is modal")

	screen.KeyPress(unison.KeyEscape, 0)
	var response int
	c.True(screen.Do(func() {
		response = result
		windows = len(unison.Windows())
	}))
	c.Equal(unison.ModalResponseCancel, response, "escape should have triggered the cancel button")
	c.Equal(1, windows, "the dialog should have been disposed of")
	c.True(screen.FocusedWindow() == main, "the focus should have returned to the window that opened the dialog")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// The drag & drop tests below use the two helpers that follow. A drag that this application starts needs no special
// driver support: the source panel does what a real widget does — StartDrag from its MouseDragCallback once the
// pointer has moved far enough — and the session that starts consumes the rest of the events the injected drag posted,
// exactly as the drag loops on Linux and Windows consume theirs.

// headlessDragPayload is the text an in-app drag carries in these tests.
const headlessDragPayload = "payload"

// newHeadlessDragSource returns a panel that starts a drag carrying the given data as soon as the pointer has moved
// far enough for the press to count as a drag gesture. This mirrors Table.InstallDragSupport, which is how the widgets
// in this package do it.
func newHeadlessDragSource(cleanup func(), data ...drag.Data) *unison.Panel {
	p := unison.NewPanel()
	// The press has to be claimed: a window only remembers which panel to send the drags of a press to if that panel
	// answered the press itself.
	p.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
	p.MouseDragCallback = func(where geom.Point, _ int, _ mod.Modifiers) bool {
		w := p.Window()
		if w == nil {
			return true
		}
		root := p.PointToRoot(where)
		if w.IsDragGesture(root) {
			w.StartDrag(nil, root, cleanup, drag.Copy, data...)
		}
		return true
	}
	return p
}

// headlessDropTarget is a panel that records what a drag did to it. The data is re-read on every callback that is
// handed it, so an assertion made after any step of a drag sees what the drag was carrying at that step.
type headlessDropTarget struct {
	*unison.Panel
	text    string
	paths   []string
	entered int
	updated int
	exited  int
	drops   int
}

// newHeadlessDropTarget returns a drop target that accepts the drags accept approves of, or every drag if accept is
// nil, and reports a copy as the operation it would perform.
func newHeadlessDropTarget(accept func(di drag.Info) bool) *headlessDropTarget {
	t := &headlessDropTarget{Panel: unison.NewPanel()}
	t.CanAcceptDropCallback = func(di drag.Info) bool { return accept == nil || accept(di) }
	t.DragEnteredCallback = func(di drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op {
		t.entered++
		t.record(di)
		return drag.Copy
	}
	t.DragUpdatedCallback = func(di drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op {
		t.updated++
		t.record(di)
		return drag.Copy
	}
	t.DragExitedCallback = func() { t.exited++ }
	t.DropCallback = func(di drag.Info, _ geom.Point, _ mod.Modifiers) bool {
		t.drops++
		t.record(di)
		return true
	}
	return t
}

func (t *headlessDropTarget) record(di drag.Info) {
	t.text = di.Text()
	t.paths = di.FilePaths()
}

// newHeadlessStackWindow creates a window whose content is the given panels stacked top to bottom, each filling an
// equal share of it, and shows it. The stacking is what gives a drag test somewhere to pick data up and somewhere else
// to put it down, far enough apart that a step of the drag covers more than the drift that makes a press a gesture.
func newHeadlessStackWindow(t *testing.T, title string, rect geom.Rect, panels ...unison.Paneler) *unison.Window {
	t.Helper()
	container := unison.NewPanel()
	container.SetLayout(&unison.FlexLayout{
		Columns:  1,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	for _, panel := range panels {
		panel.AsPanel().SetLayoutData(&unison.FlexLayoutData{
			HAlign: align.Fill,
			VAlign: align.Fill,
			HGrab:  true,
			VGrab:  true,
		})
		container.AddChild(panel)
	}
	return newHeadlessWindow(t, title, rect, container)
}

// TestHeadlessInAppDrag runs a drag & drop from one panel to another the way a person would: press on the source, move
// to the target, release. The widget code that reacts to it is the ordinary kind, with nothing in it that knows the
// events were injected. The source supplies a drag image, so that the result can be checked for what it chose to
// show and where it picked it up.
func TestHeadlessInAppDrag(t *testing.T) {
	c := check.New(t)
	var w *unison.Window
	var src *unison.Panel
	var dst *headlessDropTarget
	var dragImage *unison.Image
	var dragOrigin geom.Point
	cleanups := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			var err error
			dragImage, err = unison.NewImageFromPixels(1, 1, []byte{255, 0, 0, 255}, geom.NewPoint(1, 1))
			if err != nil {
				t.Errorf("unable to create drag image: %v", err)
				return
			}
			src = unison.NewPanel()
			src.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
			src.MouseDragCallback = func(where geom.Point, _ int, _ mod.Modifiers) bool {
				if wnd := src.Window(); wnd != nil {
					root := src.PointToRoot(where)
					if wnd.IsDragGesture(root) {
						dragOrigin = root
						wnd.StartDrag(dragImage, root, func() { cleanups++ }, drag.Copy,
							drag.Data{Type: uti.UTF8PlainText, Data: []byte(headlessDragPayload)})
					}
				}
				return true
			}
			dst = newHeadlessDropTarget(func(di drag.Info) bool { return di.HasString() })
			w = newHeadlessStackWindow(t, "drag", geom.NewRect(20, 20, 300, 240), src, dst)
			if w != nil {
				w.RegisterForDragTypes(uti.UTF8PlainText)
			}
		}))
	c.NotNil(w)
	c.NotNil(dragImage)

	screen.Drag(screen.PanelCenter(src), screen.PanelCenter(dst), 4)

	var entered, updated, exited, drops, cleaned int
	var text string
	c.True(screen.Do(func() {
		entered, updated, exited, drops = dst.entered, dst.updated, dst.exited, dst.drops
		text = dst.text
		cleaned = cleanups
	}))
	c.Equal(1, entered, "the target should have been entered exactly once")
	c.True(updated >= 1, "the target should have been asked what it would do with the drag")
	c.Equal(0, exited, "a drag that ends in a drop should not exit the target first")
	c.Equal(1, drops)
	c.Equal(headlessDragPayload, text, "the drop should have received what the source was carrying")
	c.Equal(1, cleaned, "the source's cleanup should have run when the drag finished")

	result := screen.LastDrag()
	c.True(result.Dropped)
	c.True(result.Handled, "the target reported that it dealt with the data")
	c.Equal(drag.Copy, result.Op)
	c.True(result.Source == w)
	c.True(result.Target == w)
	c.False(result.Canceled)
	c.True(result.Image == dragImage, "the result should report the drag image the source supplied")
	var origin geom.Point
	c.True(screen.Do(func() { origin = dragOrigin }))
	c.Equal(origin, result.Origin, "the result should report where the source picked the drag image up")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessDragOverUnregisteredWindow verifies the registration rule: a window that never asked for the type being
// dragged is not a drop target, so the drag passes over it as though it belonged to another application.
func TestHeadlessDragOverUnregisteredWindow(t *testing.T) {
	c := check.New(t)
	var source, other *unison.Window
	var src *unison.Panel
	var dst *headlessDropTarget
	cleanups := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 300},
		unison.StartupFinishedCallback(func() {
			src = newHeadlessDragSource(func() { cleanups++ },
				drag.Data{Type: uti.UTF8PlainText, Data: []byte(headlessDragPayload)})
			source = newHeadlessStackWindow(t, "source", geom.NewRect(0, 0, 200, 200), src)
			if source != nil {
				source.RegisterForDragTypes(uti.UTF8PlainText)
			}
			dst = newHeadlessDropTarget(nil)
			// Deliberately never registered for anything, so its drop target must never be reached.
			other = newHeadlessStackWindow(t, "other", geom.NewRect(260, 0, 200, 200), dst)
		}))
	c.NotNil(source)
	c.NotNil(other)

	screen.Drag(screen.PanelCenter(src), screen.PanelCenter(dst), 4)

	var entered, drops, cleaned int
	c.True(screen.Do(func() {
		entered, drops = dst.entered, dst.drops
		cleaned = cleanups
	}))
	c.Equal(0, entered, "an unregistered window should see no drag callbacks at all")
	c.Equal(0, drops)
	c.Equal(1, cleaned, "the source's cleanup runs however the drag ended")

	result := screen.LastDrag()
	c.False(result.Dropped)
	c.Nil(result.Target)
	c.True(result.Source == source)
	c.Equal(drag.None, result.Op)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessDragCanceledByEscape verifies that Escape abandons a drag, that the target it was over is told so, and
// that the application is left in working order rather than stuck inside the drag's event loop.
func TestHeadlessDragCanceledByEscape(t *testing.T) {
	c := check.New(t)
	var w *unison.Window
	var src *unison.Panel
	var dst *headlessDropTarget
	var button *unison.Button
	clicks := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 400},
		unison.StartupFinishedCallback(func() {
			src = newHeadlessDragSource(nil,
				drag.Data{Type: uti.UTF8PlainText, Data: []byte(headlessDragPayload)})
			dst = newHeadlessDropTarget(nil)
			button = unison.NewButton()
			button.SetTitle("After")
			button.ClickCallback = func() { clicks++ }
			w = newHeadlessStackWindow(t, "escape", geom.NewRect(20, 20, 300, 340), src, dst, button)
			if w != nil {
				w.RegisterForDragTypes(uti.UTF8PlainText)
			}
		}))
	c.NotNil(w)

	// Driven a step at a time rather than with Drag, so that the press and the move that turns it into a drag are
	// separate calls and the Escape below lands while the drag's event loop is running.
	from := screen.PanelCenter(src)
	to := screen.PanelCenter(dst)
	screen.MouseMove(from, 0)
	screen.MouseDown(from, unison.ButtonLeft, 0)
	screen.MouseMove(to, 0)
	var entered int
	c.True(screen.Do(func() { entered = dst.entered }))
	c.Equal(1, entered, "the move should have started the drag and entered the target")

	screen.KeyPress(unison.KeyEscape, 0)
	var exited, drops int
	c.True(screen.Do(func() { exited, drops = dst.exited, dst.drops }))
	c.Equal(1, exited, "canceling should have taken the drag back out of the target")
	c.Equal(0, drops, "nothing should have been dropped")

	result := screen.LastDrag()
	c.True(result.Canceled)
	c.False(result.Dropped)

	// The drag ran a nested event loop; if it had not unwound, nothing below would ever happen.
	screen.Click(screen.PanelCenter(button))
	var clicked int
	c.True(screen.Do(func() { clicked = clicks }))
	c.Equal(1, clicked, "the application should still be responsive after a canceled drag")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessCrossWindowDrag verifies that a drag started in one window can be dropped into another, which is the
// case the source window's own event loop has to keep the rest of the application alive for.
func TestHeadlessCrossWindowDrag(t *testing.T) {
	c := check.New(t)
	var source, destination *unison.Window
	var src *unison.Panel
	var dst *headlessDropTarget
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 300},
		unison.StartupFinishedCallback(func() {
			src = newHeadlessDragSource(nil,
				drag.Data{Type: uti.UTF8PlainText, Data: []byte(headlessDragPayload)})
			source = newHeadlessStackWindow(t, "source", geom.NewRect(0, 0, 200, 200), src)
			dst = newHeadlessDropTarget(nil)
			destination = newHeadlessStackWindow(t, "destination", geom.NewRect(260, 0, 200, 200), dst)
			for _, w := range []*unison.Window{source, destination} {
				if w != nil {
					w.RegisterForDragTypes(uti.UTF8PlainText)
				}
			}
		}))
	c.NotNil(source)
	c.NotNil(destination)

	screen.Drag(screen.PanelCenter(src), screen.PanelCenter(dst), 4)

	var drops int
	var text string
	c.True(screen.Do(func() {
		drops = dst.drops
		text = dst.text
	}))
	c.Equal(1, drops, "the other window should have received the drop")
	c.Equal(headlessDragPayload, text)

	result := screen.LastDrag()
	c.True(result.Dropped)
	c.True(result.Handled)
	c.True(result.Source == source)
	c.True(result.Target == destination)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessExternalDrag runs a drag that arrives from outside the application, carrying file paths as a file
// manager would, and steps it through the whole of a target's lifecycle: entered, left, entered again, dropped.
func TestHeadlessExternalDrag(t *testing.T) {
	c := check.New(t)
	var w *unison.Window
	var filler *unison.Panel
	var dst *headlessDropTarget
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			filler = unison.NewPanel()
			dst = newHeadlessDropTarget(func(di drag.Info) bool { return di.HasFilePaths() })
			w = newHeadlessStackWindow(t, "external", geom.NewRect(60, 40, 240, 200), filler, dst)
			if w != nil {
				w.RegisterForDragTypes(uti.FileURL)
			}
		}))
	c.NotNil(w)

	// Outside every window, which is where a drag coming from another application starts.
	d := screen.BeginExternalDrag(geom.NewPoint(10, 10), drag.Copy,
		drag.Data{Type: uti.FileURL, Data: []byte("/a\n/b")})

	d.MoveTo(screen.PanelCenter(dst))
	var entered, exited, drops int
	var paths []string
	c.True(screen.Do(func() {
		entered, exited = dst.entered, dst.exited
		paths = dst.paths
	}))
	c.Equal(1, entered)
	c.Equal(0, exited)
	c.Equal([]string{"/a", "/b"}, paths, "the file paths should have been decoded from the lines they were given as")

	d.MoveTo(screen.PanelCenter(filler))
	c.True(screen.Do(func() { exited = dst.exited }))
	c.Equal(1, exited, "moving off the target should have left it")

	d.MoveTo(screen.PanelCenter(dst))
	c.True(screen.Do(func() { entered = dst.entered }))
	c.Equal(2, entered, "moving back onto the target should have entered it again")

	// The wheel keeps working while a drag is held over the window, and since scrolling may have moved the content
	// under the pointer, the target is asked again what it would do with a drop there.
	var wheels, updatedBefore, updatedAfter int
	c.True(screen.Do(func() {
		w.MouseWheelCallback = func(_, _ geom.Point, _ mod.Modifiers) bool {
			wheels++
			return false
		}
		updatedBefore = dst.updated
	}))
	screen.Wheel(screen.PanelCenter(dst), geom.NewPoint(0, 1), mod.None)
	c.True(screen.Do(func() { updatedAfter = dst.updated }))
	c.Equal(1, wheels, "the wheel should have reached the window under the drag")
	c.Equal(updatedBefore+1, updatedAfter, "the drop feedback should have been recomputed after the wheel")

	result := d.Drop()
	c.True(screen.Do(func() { drops = dst.drops }))
	c.Equal(1, drops)
	c.True(result.Dropped)
	c.True(result.Handled)
	c.Equal(drag.Copy, result.Op)
	c.Nil(result.Source, "a drag from outside the application has no source window")
	c.True(result.Target == w)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessDropExternal covers the one-call form of an external drag, and then the canceled form, which leaves the
// target as it found it.
func TestHeadlessDropExternal(t *testing.T) {
	c := check.New(t)
	var w *unison.Window
	var dst *headlessDropTarget
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			dst = newHeadlessDropTarget(nil)
			w = newHeadlessStackWindow(t, "external", geom.NewRect(60, 40, 240, 200), dst)
			if w != nil {
				w.RegisterForDragTypes(uti.FileURL)
			}
		}))
	c.NotNil(w)

	result := screen.DropExternal(geom.NewPoint(10, 10), screen.PanelCenter(dst), 3, drag.Copy,
		drag.Data{Type: uti.FileURL, Data: []byte("/a\n/b")})
	var drops int
	var paths []string
	c.True(screen.Do(func() {
		drops = dst.drops
		paths = dst.paths
	}))
	c.Equal(1, drops)
	c.Equal([]string{"/a", "/b"}, paths)
	c.True(result.Dropped)
	c.True(result.Handled)
	c.True(result.Target == w)

	// A canceled drag reaches the target and then takes itself away again.
	dropsBefore := drops
	d := screen.BeginExternalDrag(geom.NewPoint(10, 10), drag.Copy,
		drag.Data{Type: uti.FileURL, Data: []byte("/c")})
	d.MoveTo(screen.PanelCenter(dst))
	canceled := d.Cancel()
	var entered, exited int
	c.True(screen.Do(func() {
		entered, exited, drops = dst.entered, dst.exited, dst.drops
	}))
	c.Equal(2, entered, "the canceled drag should have entered the target as well")
	c.Equal(1, exited, "canceling should have taken the drag back out of the target")
	c.Equal(dropsBefore, drops, "the canceled drag should not have dropped anything")
	c.True(canceled.Canceled)
	c.False(canceled.Dropped)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())

	// The pointer belongs to whatever holds it: an external drag cannot start while another is in progress, nor while a
	// button is down.
	inFlight := screen.BeginExternalDrag(geom.NewPoint(10, 10), drag.Copy,
		drag.Data{Type: uti.FileURL, Data: []byte("/d")})
	refused := screen.BeginExternalDrag(geom.NewPoint(10, 10), drag.Copy,
		drag.Data{Type: uti.FileURL, Data: []byte("/e")})
	c.Equal(1, len(screen.Errors()), "the refusal should have been recorded: %v", screen.Errors())
	c.True(refused.Drop().Canceled, "a refused drag reports itself as canceled")
	inFlight.MoveTo(screen.PanelCenter(dst))
	c.True(inFlight.Drop().Dropped, "the drag that was already in flight should have been left alone")
	c.True(screen.Do(func() { drops = dst.drops }))
	c.Equal(dropsBefore+1, drops, "only the drag that was in flight should have dropped")

	screen.MouseDown(screen.PanelCenter(dst), unison.ButtonLeft, 0)
	refused = screen.BeginExternalDrag(geom.NewPoint(10, 10), drag.Copy,
		drag.Data{Type: uti.FileURL, Data: []byte("/f")})
	c.Equal(2, len(screen.Errors()), "the refusal should have been recorded: %v", screen.Errors())
	c.True(refused.Drop().Canceled, "a refused drag reports itself as canceled")
	c.True(screen.Do(func() { drops = dst.drops }))
	c.Equal(dropsBefore+1, drops, "the refused drag should not have dropped anything")
}

// TestHeadlessDarkMode verifies that the session, rather than the machine the test is running on, decides what dark
// mode is, and that changing its mind repaints the application.
func TestHeadlessDarkMode(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	draws := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 200, Height: 200, DarkMode: true},
		unison.StartupFinishedCallback(func() {
			w, err := unison.NewWindow("dark")
			if err != nil {
				t.Errorf("unable to create window: %v", err)
				return
			}
			// The window paints its own background with ThemeSurface before the content is drawn, so the callback only
			// has to count the repaints; the pixels the assertions look at are the theme's.
			w.Content().DrawCallback = func(_ *unison.Canvas, _ geom.Rect) { draws++ }
			w.SetContentRect(geom.NewRect(0, 0, 100, 100))
			w.Show()
			wnd = w
		}))
	c.NotNil(wnd)
	screen.Sync()

	var dark bool
	var trackable bool
	var drawnDark int
	c.True(screen.Do(func() {
		dark = unison.IsDarkModeEnabled()
		trackable = unison.IsColorModeTrackingPossible()
		drawnDark = draws
	}))
	c.True(dark, "the session was configured for dark mode")
	c.True(trackable, "a headless session always knows its own dark mode state")
	c.True(drawnDark > 0, "the window should have been drawn")
	img := screen.Capture()
	c.NotNil(img)
	c.Equal(toNRGBAColor(unison.DefaultThemeSurface().Dark), img.NRGBAAt(50, 50),
		"the window should have been painted with the dark surface color")

	screen.SetDarkMode(false)
	var drawnLight int
	c.True(screen.Do(func() {
		dark = unison.IsDarkModeEnabled()
		drawnLight = draws
	}))
	c.False(dark, "the session should have changed its mind")
	c.True(drawnLight > drawnDark, "changing the dark mode state should have redrawn the window")
	img = screen.Capture()
	c.NotNil(img)
	c.Equal(toNRGBAColor(unison.DefaultThemeSurface().Light), img.NRGBAAt(50, 50),
		"the window should have been repainted with the light surface color")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// toNRGBAColor converts a unison Color into the form a captured image reports pixels in.
func toNRGBAColor(clr unison.Color) color.NRGBA {
	return color.NRGBA{R: uint8(clr.Red()), G: uint8(clr.Green()), B: uint8(clr.Blue()), A: uint8(clr.Alpha())}
}

// TestHeadlessSessionAfterLastWindowClosed verifies that a test binary can run one session after another: the first
// ends the way an application normally does, by closing its last window, and the second is a fully working application
// again.
func TestHeadlessSessionAfterLastWindowClosed(t *testing.T) {
	c := check.New(t)
	var first *unison.Window
	one := startHeadless(t, unison.HeadlessConfig{Width: 300, Height: 200},
		unison.StartupFinishedCallback(func() {
			first = newHeadlessWindow(t, "one", geom.NewRect(0, 0, 200, 100), unison.NewPanel())
		}))
	c.NotNil(first)
	var closed bool
	c.True(one.Do(func() { closed = first.AttemptClose() }))
	c.True(closed, "the window should have closed")
	// Closing the last window quits the application, which ends the session; the teardown that follows is what releases
	// the globals the next session needs.
	waitWithTimeout(t, one)
	c.False(one.Running())

	var button *unison.Button
	clicks := 0
	two := startHeadless(t, unison.HeadlessConfig{Width: 300, Height: 200},
		unison.StartupFinishedCallback(func() {
			button = unison.NewButton()
			button.SetTitle("Press Me")
			button.ClickCallback = func() { clicks++ }
			newHeadlessWindow(t, "two", geom.NewRect(20, 20, 200, 80), button)
		}))
	c.NotNil(button)
	two.Click(two.PanelCenter(button))
	var clicked int
	c.True(two.Do(func() { clicked = clicks }))
	c.Equal(1, clicked, "the second session should be a working application")
	c.Equal(0, len(two.Errors()), "nothing should have panicked: %v", two.Errors())
}

// TestHeadlessClipboard verifies that the clipboard a session offers is its own: it round trips within the session and
// starts empty in the next one, so a test can neither see nor disturb what is on the clipboard of the machine it is
// running on.
func TestHeadlessClipboard(t *testing.T) {
	c := check.New(t)
	one := startHeadless(t, unison.HeadlessConfig{Width: 100, Height: 100})
	var hadText bool
	var text string
	c.True(one.Do(func() {
		hadText = unison.ClipboardHasText()
		unison.ClipboardSetText("headless clipboard")
		text = unison.ClipboardGetText()
	}))
	c.False(hadText, "the session's clipboard should have started empty")
	c.Equal("headless clipboard", text)

	var payload []byte
	c.True(one.Do(func() {
		unison.ClipboardSetData(drag.Data{Type: uti.UTF8PlainText, Data: []byte("bytes")})
		payload = unison.ClipboardGetData(uti.UTF8PlainText)
	}))
	c.Equal("bytes", string(payload), "setting data should replace what was there")
	// What was handed back is a copy, so a caller writing into it alters nothing a later reader sees.
	payload[0] = '!'
	c.True(one.Do(func() { payload = unison.ClipboardGetData(uti.UTF8PlainText) }))
	c.Equal("bytes", string(payload), "writing into what ClipboardGetData returned should not alter the clipboard")
	one.Stop()

	two := startHeadless(t, unison.HeadlessConfig{Width: 100, Height: 100})
	c.True(two.Do(func() {
		hadText = unison.ClipboardHasText()
		text = unison.ClipboardGetText()
	}))
	c.False(hadText, "a new session should start with an empty clipboard")
	c.Equal("", text)
}

// TestHeadlessBeep verifies that the beep an application asks for is counted rather than played.
func TestHeadlessBeep(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t, unison.HeadlessConfig{Width: 100, Height: 100})
	c.Equal(0, screen.Beeps())
	c.True(screen.Do(func() {
		unison.Beep()
		unison.Beep()
	}))
	c.Equal(2, screen.Beeps())
}

// TestHeadlessCursor verifies that the cursor a panel asks for is the one the session reports, and that leaving the
// panel puts it back.
func TestHeadlessCursor(t *testing.T) {
	c := check.New(t)
	var plain, texty *unison.Panel
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			plain = unison.NewPanel()
			texty = unison.NewPanel()
			texty.UpdateCursorCallback = func(_ geom.Point) *unison.Cursor { return unison.TextCursor() }
			newHeadlessStackWindow(t, "cursors", geom.NewRect(20, 20, 200, 200), plain, texty)
		}))
	c.NotNil(plain)
	c.NotNil(texty)

	// The built-in cursors are resolved lazily and belong to the UI thread, so the ones to compare against are fetched
	// there rather than on this goroutine.
	var text, arrow *unison.Cursor
	c.True(screen.Do(func() {
		text = unison.TextCursor()
		arrow = unison.ArrowCursor()
	}))
	c.NotNil(text)
	c.NotNil(arrow)

	screen.MouseMove(screen.PanelCenter(texty), 0)
	c.True(screen.Cursor() == text, "the cursor the panel asked for should be the one in use")

	screen.MouseMove(screen.PanelCenter(plain), 0)
	c.True(screen.Cursor() == arrow, "leaving the panel should have put the arrow back")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessDisplays verifies that the one display a session has is the screen it was configured with.
func TestHeadlessDisplays(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t, unison.HeadlessConfig{Width: 320, Height: 240, Scale: 2})
	var primary *unison.Display
	var all []*unison.Display
	c.True(screen.Do(func() {
		primary = unison.PrimaryDisplay()
		all = unison.AllDisplays()
	}))
	c.NotNil(primary)
	c.Equal(geom.NewRect(0, 0, 320, 240), primary.Frame)
	c.Equal(geom.NewRect(0, 0, 320, 240), primary.Usable, "there are no menu bars or docks to make room for")
	c.Equal(geom.NewPoint(2, 2), primary.Scale)
	c.Equal(192, primary.PPI, "the assumed 96 dpi at 1x, scaled")
	c.True(primary.Primary)
	c.Equal(1, len(all), "a session has exactly one display")
	if len(all) == 1 {
		c.True(all[0] == primary)
	}
}

// TestHeadlessSessionEndsDuringDrag verifies that a session can be shut down while the source side of a drag & drop is
// holding the thread in its own event loop. That loop is what would otherwise spin forever: nothing further is going to
// arrive to end the drag, so the shutdown has to end it, and the source's cleanup still has to run on the way out.
func TestHeadlessSessionEndsDuringDrag(t *testing.T) {
	c := check.New(t)
	var w *unison.Window
	var src *unison.Panel
	var dst *headlessDropTarget
	cleanups := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 400},
		unison.StartupFinishedCallback(func() {
			src = newHeadlessDragSource(func() { cleanups++ },
				drag.Data{Type: uti.UTF8PlainText, Data: []byte(headlessDragPayload)})
			dst = newHeadlessDropTarget(nil)
			w = newHeadlessStackWindow(t, "abandoned", geom.NewRect(20, 20, 300, 340), src, dst)
			if w != nil {
				w.RegisterForDragTypes(uti.UTF8PlainText)
			}
		}))
	c.NotNil(w)

	// Press and move, but never release, so the drag's event loop is still running when the session is stopped.
	from := screen.PanelCenter(src)
	screen.MouseMove(from, 0)
	screen.MouseDown(from, unison.ButtonLeft, 0)
	screen.MouseMove(screen.PanelCenter(dst), 0)
	var entered int
	c.True(screen.Do(func() { entered = dst.entered }))
	c.Equal(1, entered, "the drag should be in flight")

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		screen.Stop()
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() never returned while a drag's event loop was running")
	}
	c.False(screen.Running())
	// cleanups was written on the UI goroutine, and is read here rather than inside a Do, since the session that would
	// run a Do is gone. The read is still ordered after the write: Stop() returns only once the session's done channel
	// has been closed, which happens on the UI goroutine after the drag's loop unwound and ran the cleanup.
	c.Equal(1, cleanups, "the source's cleanup should have run as the drag's loop unwound")

	// The session that follows must be unaware that any of that happened.
	next := startHeadless(t, unison.HeadlessConfig{Width: 100, Height: 100})
	var windows int
	c.True(next.Do(func() { windows = len(unison.Windows()) }))
	c.Equal(0, windows, "the next session should have started with no windows")
}

// waitWithTimeout waits for the session to end, failing the test rather than hanging the whole binary if it never does.
// The wait itself is Wait(), on a goroutine of its own so that a timeout can be put on it.
func waitWithTimeout(t *testing.T, screen *unison.HeadlessScreen) {
	t.Helper()
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		screen.Wait()
	}()
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the headless session never ended")
	}
}

// quitWithTimeout calls Quit() on a goroutine of its own and reports what it returned, failing the test rather than
// hanging it if the call never comes back.
func quitWithTimeout(t *testing.T, screen *unison.HeadlessScreen) bool {
	t.Helper()
	result := make(chan bool, 1)
	go func() { result <- screen.Quit() }()
	select {
	case quit := <-result:
		return quit
	case <-time.After(5 * time.Second):
		t.Fatal("Quit() never returned while the application had a dialog up")
		return false
	}
}

// TestHeadlessQuitWithConfirmationDialog covers a quit that the application answers with a question of its own: the
// window's AllowCloseCallback puts up a modal dialog and does not decide until it has been dismissed. Quit() has to
// come back with that dialog still standing, since the goroutine it would otherwise block is the only one that can
// dismiss it.
func TestHeadlessQuitWithConfirmationDialog(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	prompts := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "confirm", geom.NewRect(20, 20, 200, 150), unison.NewPanel())
			if wnd == nil {
				return
			}
			wnd.AllowCloseCallback = func() bool {
				prompts++
				d, err := unison.NewDialog(nil, nil, unison.NewMessagePanel("Really quit?", ""),
					[]*unison.DialogButtonInfo{unison.NewCancelButtonInfo(), unison.NewOKButtonInfo()})
				if err != nil {
					t.Errorf("unable to create dialog: %v", err)
					return false
				}
				// Does not return until the dialog is dismissed, which only the test goroutine can arrange.
				return d.RunModal() == unison.ModalResponseOK
			}
		}))
	c.NotNil(wnd)

	c.False(quitWithTimeout(t, screen), "the quit is waiting on the dialog, so it has not finished")
	var windows, asked int
	c.True(screen.Do(func() {
		windows = len(unison.Windows())
		asked = prompts
	}))
	c.Equal(2, windows, "the confirmation dialog should be open alongside the window that put it up")
	c.Equal(1, asked)

	// Escape answers the dialog with a cancel, which refuses the close and with it the quit.
	screen.KeyPress(unison.KeyEscape, 0)
	c.True(screen.Running(), "a quit the application refused should leave the session running")
	c.True(screen.Do(func() { windows = len(unison.Windows()) }))
	c.Equal(1, windows, "the dialog should have been disposed of")

	// The same again, answered the other way this time.
	c.False(quitWithTimeout(t, screen), "the second quit is waiting on its dialog too")
	c.True(screen.Do(func() { asked = prompts }))
	c.Equal(2, asked, "the second quit should have put the question again")

	// Return triggers the dialog's OK button, which allows the close and lets the quit run to its end.
	screen.KeyPress(unison.KeyReturn, 0)
	waitWithTimeout(t, screen)
	c.False(screen.Running(), "accepting the dialog should have let the quit finish")
}

// TestHeadlessSyncTimeout verifies that an application which never goes quiet does not hang the test that is driving
// it: the wait is abandoned once HeadlessConfig.SyncTimeout has passed, giving up is reported through Errors(), and the
// session can still be shut down afterwards.
func TestHeadlessSyncTimeout(t *testing.T) {
	c := check.New(t)
	var panel *unison.Panel
	screen := startHeadless(t, unison.HeadlessConfig{Width: 200, Height: 200, SyncTimeout: 200 * time.Millisecond},
		unison.StartupFinishedCallback(func() {
			panel = unison.NewPanel()
			// Asking for a redraw from inside a draw means a redraw is outstanding again the instant the one being
			// performed finishes, so the application never has nothing left to do.
			panel.DrawCallback = func(_ *unison.Canvas, _ geom.Rect) { panel.MarkForRedraw() }
			newHeadlessWindow(t, "never quiet", geom.NewRect(20, 20, 100, 100), panel)
		}))
	c.NotNil(panel)
	// StartHeadless settles before it hands the screen back, and this application can never settle, so the wait it
	// performed was abandoned in exactly the same way — and, just as importantly, was bounded, rather than leaving the
	// test waiting for a screen it would never be given.
	c.Equal(1, len(screen.Errors()), "the wait StartHeadless performs should have been abandoned too")

	clicked := make(chan struct{})
	go func() {
		defer close(clicked)
		screen.Click(geom.NewPoint(50, 50))
	}()
	select {
	case <-clicked:
	case <-time.After(5 * time.Second):
		t.Fatal("the driver call never returned for an application that never goes quiet")
	}
	errors := screen.Errors()
	c.Equal(2, len(errors), "abandoning a wait should have been recorded once for each of them: %v", errors)

	// Nothing about the spin may stand in the way of ending the session, which is what a test's cleanup relies on.
	screen.Stop()
	c.False(screen.Running())
}

// TestHeadlessFractionalScaleCapture verifies that a composited screen puts a window's pixels precisely where the
// window says it is, even at a backing scale where both its position and its size land on a half pixel. The screen and
// the window have to agree on how a logical point becomes a device pixel, or the window ends up a pixel away from its
// own edges.
func TestHeadlessFractionalScaleCapture(t *testing.T) {
	c := check.New(t)
	background := unison.RGB(0, 0, 255)
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 300, Height: 200, Scale: 1.5, Background: background},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "fractional", geom.NewRect(11, 7, 101, 51), unison.NewPanel())
			if wnd == nil {
				return
			}
			wnd.Content().DrawCallback = func(gc *unison.Canvas, rect geom.Rect) {
				gc.DrawRect(rect, unison.Red.Paint(gc, rect, paintstyle.Fill))
			}
			wnd.MarkForRedraw()
		}))
	c.NotNil(wnd)
	screen.Sync() // the window must have been drawn before there is anything to capture

	img := screen.Capture()
	c.NotNil(img)
	// The window's own rendering surface is sized by truncating its logical size, so the capture has to place it by
	// truncating its logical position: it starts at (int(11*1.5), int(7*1.5)) and is int(101*1.5) by int(51*1.5)
	// device pixels.
	const (
		left   = 16
		top    = 10
		right  = left + 151 - 1
		bottom = top + 76 - 1
		midX   = left + 40
		midY   = top + 20
	)
	for _, one := range []struct {
		name    string
		inside  image.Point
		outside image.Point
	}{
		{name: "left", inside: image.Pt(left, midY), outside: image.Pt(left-1, midY)},
		{name: "right", inside: image.Pt(right, midY), outside: image.Pt(right+1, midY)},
		{name: "top", inside: image.Pt(midX, top), outside: image.Pt(midX, top-1)},
		{name: "bottom", inside: image.Pt(midX, bottom), outside: image.Pt(midX, bottom+1)},
	} {
		in := img.NRGBAAt(one.inside.X, one.inside.Y)
		c.True(in.R > 200 && in.G < 60 && in.B < 60 && in.A == 255,
			"the pixel just inside the %s edge should be the window's red, but was %v", one.name, in)
		c.Equal(toNRGBAColor(background), img.NRGBAAt(one.outside.X, one.outside.Y),
			"the pixel just outside the %s edge should be the background", one.name)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessTypeLineEndings verifies that each of the conventional line endings types exactly one Return and that a
// carriage return is never delivered as a character of its own, so that text carrying Windows line endings types what
// it says rather than a stray rune ahead of every line break.
func TestHeadlessTypeLineEndings(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	returns := 0
	keys := make(map[unison.KeyCode]int)
	var typed []rune
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			// The field's own handling is wrapped rather than replaced, so it still does what a field does with what it
			// is sent while the test records what that was.
			keyDown := field.KeyDownCallback
			field.KeyDownCallback = func(code unison.KeyCode, mods mod.Modifiers, repeat bool) bool {
				if code == unison.KeyReturn {
					returns++
				}
				keys[code]++
				return keyDown(code, mods, repeat)
			}
			runeTyped := field.RuneTypedCallback
			field.RuneTypedCallback = func(ch rune) bool {
				typed = append(typed, ch)
				return runeTyped(ch)
			}
			newHeadlessWindow(t, "line endings", geom.NewRect(20, 20, 240, 60), field)
		}))
	c.NotNil(field)

	screen.Click(screen.PanelCenter(field))
	var focused bool
	c.True(screen.Do(func() { focused = field.Focused() }))
	c.True(focused, "clicking the field should have given it the focus")

	// A single-line field ignores Return rather than inserting anything for it, so only the letters survive.
	screen.Type("a\r\nb")
	var text string
	var pressed int
	var runes []rune
	c.True(screen.Do(func() {
		text = field.Text()
		pressed = returns
		runes = slices.Clone(typed)
	}))
	c.Equal("ab", text, `"\r\n" should have typed one Return and no characters of its own`)
	c.Equal(1, pressed, "a carriage return and the newline that follows it are one line ending")
	c.Equal([]rune{'a', 'b'}, runes, "no rune should have been delivered for the line ending")

	screen.Type("c\rd")
	c.True(screen.Do(func() {
		text = field.Text()
		pressed = returns
		runes = slices.Clone(typed)
	}))
	c.Equal("abcd", text, "a lone carriage return should have typed a Return rather than a character")
	c.Equal(2, pressed)
	c.Equal([]rune{'a', 'b', 'c', 'd'}, runes, "a carriage return should never be delivered as a rune")

	// A bare newline is the third line ending; a tab is the Tab key, which a lone field in a window keeps the focus
	// through; a rune no key on the layout produces is delivered on its own; and a backspace and a delete are the keys
	// of those names, so they edit rather than insert.
	screen.Type("e\nf\tgé\x7f\b")
	var tabs, backspaces, deletes int
	c.True(screen.Do(func() {
		text = field.Text()
		pressed = returns
		tabs, backspaces, deletes = keys[unison.KeyTab], keys[unison.KeyBackspace], keys[unison.KeyDelete]
		runes = slices.Clone(typed)
		focused = field.Focused()
	}))
	c.Equal("abcdefg", text, "the newline should have typed a Return, the tab a Tab, and the backspace should have "+
		"deleted the rune that no key produces")
	c.Equal(3, pressed, "a bare newline is one line ending and therefore one Return")
	c.Equal(1, tabs, "a tab should have been typed as the Tab key")
	c.Equal(1, backspaces, "a backspace should have been typed as the Backspace key")
	c.Equal(1, deletes, "a delete should have been typed as the Delete key")
	c.Equal([]rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'é'}, runes,
		"only the runes that are text should have been delivered, the non-ASCII one included")
	c.True(focused, "the field should still hold the focus")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessLastDragZeroValue verifies that LastDrag() reports the zero value, nil data included, before any drag has
// happened, rather than an empty-but-allocated copy that never compares equal to it.
func TestHeadlessLastDragZeroValue(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t, unison.HeadlessConfig{Width: 100, Height: 100})
	result := screen.LastDrag()
	c.Nil(result.Data)
	c.Equal(unison.HeadlessDragResult{}, result)
}

// TestHeadlessReadmeExampleIsCurrent checks that the headless example in README.md is the one that lives in
// headless_readme_test.go, where it is compiled and run as TestButton. The README cannot be compiled, so this is what
// catches an example that has drifted from the API it demonstrates, or that leaves out an import it needs.
func TestHeadlessReadmeExampleIsCurrent(t *testing.T) {
	c := check.New(t)
	src, err := os.ReadFile("headless_readme_test.go")
	c.NoError(err)
	_, example, found := strings.Cut(string(src), "\npackage unison_test\n\n")
	c.True(found, "headless_readme_test.go should have a package clause followed by a blank line")

	readme, err := os.ReadFile("README.md")
	c.NoError(err)
	var block string
	for _, rest := range strings.Split(string(readme), "```go\n")[1:] {
		if candidate, _, ok := strings.Cut(rest, "```\n"); ok && strings.Contains(candidate, "unison.StartHeadless(") {
			block = candidate
			break
		}
	}
	c.NotEqual("", block, "README.md should contain a go code block that calls unison.StartHeadless")
	c.Equal(example, block, "the README example must match headless_readme_test.go below its package clause")
}

// TestHeadlessKeyDownAndUp covers the two halves of a key press injected separately: each goes to the focused window
// with the modifiers it was injected with and produces no rune of its own, and both are dropped when no window holds
// the focus.
func TestHeadlessKeyDownAndUp(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	var events []string
	screen := startHeadless(t, unison.HeadlessConfig{Width: 200, Height: 200},
		unison.StartupFinishedCallback(func() {
			panel := unison.NewPanel()
			panel.KeyDownCallback = func(code unison.KeyCode, mods mod.Modifiers, _ bool) bool {
				events = append(events, "down "+code.Key()+" "+mods.Key())
				return true
			}
			panel.KeyUpCallback = func(code unison.KeyCode, mods mod.Modifiers) bool {
				events = append(events, "up "+code.Key()+" "+mods.Key())
				return true
			}
			panel.RuneTypedCallback = func(ch rune) bool {
				events = append(events, "rune "+string(ch))
				return true
			}
			panel.SetFocusable(true)
			// Shown but not brought to the front, so nothing holds the focus until the test says so.
			wnd = newHeadlessWindow(t, "keys", geom.NewRect(0, 0, 100, 100), panel)
			if wnd != nil {
				wnd.SetFocus(panel)
			}
		}))
	c.NotNil(wnd)
	c.Nil(screen.FocusedWindow(), "a window that was only shown should not hold the focus")

	screen.KeyDown(unison.KeyA, mod.Shift)
	screen.KeyUp(unison.KeyA, mod.Shift)
	var seen []string
	c.True(screen.Do(func() { seen = slices.Clone(events) }))
	c.Equal(0, len(seen), "key events should be dropped while no window holds the focus")

	c.True(screen.Do(func() { wnd.ToFront() }))
	screen.KeyDown(unison.KeyA, mod.Shift)
	var afterDown []string
	c.True(screen.Do(func() { afterDown = slices.Clone(events) }))
	c.Equal([]string{"down A shift"}, afterDown, "a key down on its own should deliver the press and no rune")
	screen.KeyUp(unison.KeyA, mod.Shift)
	var mods mod.Modifiers
	c.True(screen.Do(func() {
		seen = slices.Clone(events)
		mods = wnd.CurrentKeyModifiers()
	}))
	c.Equal([]string{"down A shift", "up A shift"}, seen)
	c.Equal(mod.Shift, mods, "the modifiers last injected should be the ones the window reports as current")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessPanelPoint verifies the other way of aiming at a widget: an offset into the panel's own content area is
// taken to the same screen position a real pointer would have to be at to land there, so a click at it reaches the
// panel at exactly that offset.
func TestHeadlessPanelPoint(t *testing.T) {
	c := check.New(t)
	var panel *unison.Panel
	var pressedAt []geom.Point
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			panel = unison.NewPanel()
			panel.MouseDownCallback = func(where geom.Point, _, _ int, _ mod.Modifiers) bool {
				pressedAt = append(pressedAt, where)
				return true
			}
			// Away from the origin, so that the screen's coordinate space and the panel's are not the same one.
			newHeadlessWindow(t, "aim", geom.NewRect(40, 30, 200, 100), panel)
		}))
	c.NotNil(panel)

	offset := geom.NewPoint(7, 5)
	at := screen.PanelPoint(panel, offset)
	var expected geom.Point
	c.True(screen.Do(func() {
		content := panel.ContentRect(false)
		expected = panel.PointToRoot(content.Point.Add(offset)).Add(panel.Window().ContentRect().Point)
	}))
	c.Equal(expected, at, "the point should be the offset carried through the panel's and the window's origins")
	c.True(screen.WindowAt(at) == panel.Window())
	topLeft := screen.PanelPoint(panel, geom.Point{})
	c.True(topLeft != at)
	c.Equal(at, topLeft.Add(offset), "the offset should be applied in the panel's own space, which is unscaled here")

	screen.Click(at)
	var pressed []geom.Point
	c.True(screen.Do(func() { pressed = slices.Clone(pressedAt) }))
	c.Equal(1, len(pressed))
	if len(pressed) == 1 {
		var wanted geom.Point
		c.True(screen.Do(func() { wanted = panel.ContentRect(false).Point.Add(offset) }))
		c.Equal(wanted, pressed[0], "the press should have landed at the offset it was aimed at")
	}

	c.Equal(geom.Point{}, screen.PanelPoint(unison.NewPanel(), offset), "a panel in no window has no screen position")
	c.Equal(geom.Point{}, screen.PanelCenter(unison.NewPanel()))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessDriverAfterSessionEnds verifies the promise the driver makes about a session that is over: nothing
// blocks, the methods that consult the UI thread report that they could not, and the answers are zero values — except
// for LastDrag, which still says how the final drag ended.
func TestHeadlessDriverAfterSessionEnds(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	var dst *headlessDropTarget
	screen := startHeadless(t, unison.HeadlessConfig{Width: 200, Height: 200},
		unison.StartupFinishedCallback(func() {
			dst = newHeadlessDropTarget(nil)
			wnd = newHeadlessWindow(t, "ends", geom.NewRect(0, 0, 100, 100), dst)
			if wnd != nil {
				wnd.RegisterForDragTypes(uti.UTF8PlainText)
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	c.NotNil(screen.CaptureWindow(wnd), "the window should have been drawn while the session ran")
	dropped := screen.DropExternal(geom.NewPoint(150, 150), screen.PanelCenter(dst), 2, drag.Copy,
		drag.Data{Type: uti.UTF8PlainText, Data: []byte("payload")})
	c.True(dropped.Dropped)

	screen.Stop()
	c.False(screen.Running())
	ran := false
	c.False(screen.Do(func() { ran = true }), "Do should report that it could not run anything")
	c.False(screen.Post(func() { ran = true }), "Post should report that it queued nothing")
	c.False(ran)
	c.Nil(screen.Capture())
	c.Nil(screen.CaptureWindow(wnd), "a window from a session that has ended has no frame to hand out")
	c.Nil(screen.WindowAt(geom.NewPoint(50, 50)))
	c.Nil(screen.FocusedWindow())
	c.Nil(screen.Cursor())
	c.Equal(geom.Point{}, screen.PanelCenter(dst))
	c.Equal(geom.NewSize(200, 200), screen.Size(), "the configuration is still the session's to report")
	// None of the injection methods may block, since there is no UI thread left to wait for.
	screen.Click(geom.NewPoint(50, 50))
	screen.Type("late")
	screen.Wheel(geom.NewPoint(50, 50), geom.NewPoint(0, 1), mod.None)
	c.True(screen.BeginExternalDrag(geom.NewPoint(50, 50), drag.Copy,
		drag.Data{Type: uti.UTF8PlainText, Data: []byte("late")}).Drop().Canceled,
		"a drag that could not begin reports itself as canceled")
	c.True(screen.LastDrag().Dropped, "the drag that completed before the end should still be reported")
	screen.Stop() // stopping again is harmless
	c.True(screen.Quit(), "a session that has already ended has nothing left to refuse the quit")
}

// TestHeadlessFileDialogs verifies that the file dialogs a session offers are the pure-Go in-window ones, which a test
// can drive like any other window: the dialog opens inside a nested event loop, and Escape dismisses it.
func TestHeadlessFileDialogs(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 800, Height: 600},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "dialogs", geom.NewRect(0, 0, 200, 100), unison.NewPanel())
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)

	for _, one := range []struct {
		run  func() bool
		name string
	}{
		{run: func() bool { return unison.NewOpenDialog().RunModal() }, name: "open"},
		{run: func() bool { return unison.NewSaveDialog().RunModal() }, name: "save"},
	} {
		result := true
		returned := false
		// Posted rather than run through Do, since RunModal does not return until the dialog has been dismissed.
		c.True(screen.Post(func() {
			result = one.run()
			returned = true
		}))
		screen.Sync()
		var windows int
		var focused *unison.Window
		c.True(screen.Do(func() {
			windows = len(unison.Windows())
			focused = screen.FocusedWindow()
		}))
		c.Equal(2, windows, "the %s dialog should be open alongside the window", one.name)
		c.True(focused != nil && focused != wnd, "the %s dialog should hold the focus", one.name)

		screen.KeyPress(unison.KeyEscape, mod.None)
		var done, accepted bool
		c.True(screen.Do(func() {
			windows = len(unison.Windows())
			done = returned
			accepted = result
		}))
		c.True(done, "escape should have dismissed the %s dialog", one.name)
		c.False(accepted, "a dismissed %s dialog should report that nothing was chosen", one.name)
		c.Equal(1, windows, "the %s dialog should have been disposed of", one.name)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessStandardMenus verifies that the standard menus can be built inside a session: the quit item carries the
// title the session gives it, and the application menu — a macOS construct, which a session declines to add entries
// to — is built all the same.
func TestHeadlessStandardMenus(t *testing.T) {
	c := check.New(t)
	var wnd *unison.Window
	var bar, app unison.Menu
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			wnd = newHeadlessWindow(t, "menus", geom.NewRect(0, 0, 300, 200), unison.NewPanel())
			if wnd == nil {
				return
			}
			factory := unison.DefaultMenuFactory()
			bar = factory.BarForWindow(wnd, func(m unison.Menu) {
				unison.InsertStdMenus(m, nil, nil, nil)
			})
			app = unison.NewAppMenu(factory, nil, nil, nil)
		}))
	c.NotNil(wnd)
	c.NotNil(bar)
	c.NotNil(app)

	var quitTitle, appQuitTitle string
	var appItems int
	c.True(screen.Do(func() {
		if file := bar.Menu(unison.FileMenuID); file != nil {
			if item := file.Item(unison.QuitItemID); item != nil {
				quitTitle = item.Title()
			}
		}
		if item := app.Item(unison.QuitItemID); item != nil {
			appQuitTitle = item.Title()
		}
		appItems = app.Count()
	}))
	c.Equal("Quit", quitTitle, "the per-window bar puts the quit item in the File menu, titled as the session says")
	c.Equal("Quit", appQuitTitle)
	c.True(appItems > 0, "the application menu should have been built")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
