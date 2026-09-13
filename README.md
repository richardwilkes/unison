# Unison

[![Go Reference](https://pkg.go.dev/badge/github.com/richardwilkes/unison.svg)](https://pkg.go.dev/github.com/richardwilkes/unison)
[![Build](https://github.com/richardwilkes/unison/actions/workflows/build.yml/badge.svg)](https://github.com/richardwilkes/unison/actions/workflows/build.yml)

A unified graphical user experience toolkit for Go desktop applications. macOS, Windows, and Linux are supported.

## Required setup

Unison is pure Go: it requires Go 1.26+ and builds with cgo disabled (`CGO_ENABLED=0`) on every platform, so no C or
Objective-C toolchain is needed. That also means cross-compilation just works — e.g. `GOOS=linux GOARCH=arm64 go build`
from a macOS host.

At runtime, the operating system's libraries are loaded dynamically (via
[purego](https://github.com/ebitengine/purego)):

- **macOS**: no setup required; the system frameworks (AppKit, CoreGraphics, OpenGL) are always present.
- **Windows**: no setup required; only standard system DLLs are used.
- **Linux**: the X11 and OpenGL client libraries must be present, since they are dlopen'd at startup: `libX11.so.6`
  and `libGL.so.1`. Any desktop system already has both; minimal or container installs need the runtime packages —
  e.g. `libx11-6` and `libgl1` on Debian/Ubuntu, or `libX11` and `libglvnd-glx` on Fedora. Development headers and
  `pkg-config` are *not* required. Unison talks to the display server via X11, so Wayland desktops need XWayland
  (virtually always present).

## Example

An example application can be found in the `cmd/example` directory:

```sh
go run cmd/example/main.go
```

## Notes

Unison was developed with the needs of my personal projects in mind, so may not be a good fit for your particular needs.
I'm open to suggestions on ways to improve the code and will happily consider Pull Requests with bug fixes or feature
additions.

### Compatibility

Unison is very much a work in progress. As such, it is likely to have breaking changes. To reflect this, a version
number of 0.x.x will be in use until such time that I'm comfortable locking things down to ensure compatibility between
releases. Please keep this in mind when making the decision to use Unison in your own code.

### Look & Feel

Unison defines its own look and feel for widgets and will likely be adjusted over time. This was done to provide as much
consistency as possible between all supported platforms. It also side-steps issues where a given platform itself has no
or poorly defined standards. Colors, fonts, spacing, how the widgets behave, and more are customizable, so if you are
feeling particularly ambitious, you could create your own theming that matches a given platform.

### Organization

There are a large number of Go source files in a single, top-level package. Unison didn't start out this way, but user
experience code tends to need to have its tentacles in many places, and the logical separations I made kept hindering
the ability to do things. Ultimately, I made the decision to collapse nearly everything into a single package to
simplify development and greatly reduce the overall complexity of things.

### Threading

Unison is single-threaded: panels, windows, drawing, and the native graphics objects behind them are owned by one UI
thread and are not safe for concurrent use. Code invoked by Unison (input/draw callbacks, layout, command handlers,
`StartupFinishedCallback`) already runs on that thread; work done on other goroutines must marshal back via `InvokeTask`
or `InvokeTaskAfter` before touching UI objects. See the package documentation for the full threading model.

### Headless testing

`StartHeadless()` runs a real application — its event loop, windows, focus handling, modal dialogs, menus and drawing —
against an in-memory screen instead of the operating system's. No display is needed, so user interface tests run on a
build machine with no windowing system at all. It runs `Start()` on its own goroutine and hands back a
`*HeadlessScreen`, which is both the stand-in screen and the handle used to drive it. It returns once the application
has settled, so the windows the `StartupFinishedCallback` created already exist; a callback that runs a nested event
loop of its own, such as a first-run dialog put up with `RunModal()`, hands back an application parked in that loop with
nothing left to do, ready to be driven. Use `Headless(cfg)` instead if your code calls `Start()` itself; in headless
mode `Start()` returns when the session ends rather than never returning.

Input is injected in the screen's logical coordinate space, which is also the space window content rects are in;
`PanelCenter()` and `PanelPoint()` convert a widget's own coordinates into it. Every injection method waits for the
application to finish reacting before it returns — the callbacks have run, the tasks they queued have run, and the
redraws those asked for have been performed — which is what `Sync()` does and what makes assertions right after a click
meaningful. The one exception is a call made from the UI thread itself, as when a widget callback drives the screen:
there the events are only queued, to be dispatched by the event loop the caller is suspended inside once it has
returned there, so the call comes back at once without waiting for anything. Work scheduled with `InvokeTaskAfter()` is
deliberately not waited for. Anything a test wants to read out of the application belongs to the UI thread, so it must
be read inside `Do()`. An application that never goes quiet — one whose `DrawCallback` marks itself for redraw, say —
would leave that wait hanging, so it is bounded by `HeadlessConfig.SyncTimeout` (10 seconds by default) and giving up is
reported through `Errors()`.

Drag & drop works too. A drag the application starts itself needs nothing special: `Drag()` posts the press and motions,
and a widget calling `StartDrag()` from its `MouseDragCallback` turns them into a drag exactly as it would on a real
platform. For data arriving from outside the application, such as files from a file manager, use `BeginExternalDrag()`
or `DropExternal()`. Sessions run one after another within a process, never side by side, and a session owns most of the
package's mutable globals while it runs, so tests using one must not call `t.Parallel()`.

```go
import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
)

func TestButton(t *testing.T) {
	var button *unison.Button
	clicks := 0
	screen, err := unison.StartHeadless(unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			button = unison.NewButton()
			button.SetTitle("Press Me")
			button.ClickCallback = func() { clicks++ }
			button.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Fill, VAlign: align.Fill, HGrab: true, VGrab: true})
			wnd, wndErr := unison.NewWindow("example")
			if wndErr != nil {
				t.Error(wndErr)
				return
			}
			wnd.Content().SetLayout(&unison.FlexLayout{Columns: 1})
			wnd.Content().AddChild(button)
			wnd.SetContentRect(geom.NewRect(20, 20, 200, 80))
			wnd.ToFront()
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Stop)

	screen.Click(screen.PanelCenter(button))
	var count int
	screen.Do(func() { count = clicks })
	if count != 1 {
		t.Errorf("expected 1 click, got %d", count)
	}

	f, err := os.Create(filepath.Join(t.TempDir(), "button.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, screen.Capture()); err != nil { // Capture() returns an *image.NRGBA of the whole screen
		t.Error(err)
	}
	if err = f.Close(); err != nil {
		t.Error(err)
	}
}
```

### Accessibility

Unison describes its windows to the assistive technologies the operating system provides, so a screen reader can read an
application's controls, follow the keyboard focus, report what is being typed and act on the user's behalf. It speaks
NSAccessibility on macOS (VoiceOver), UI Automation on Windows (Narrator, NVDA, JAWS) and AT-SPI2 on Linux (Orca), and
it is pure Go on all three: no C library and no accessibility bridge is involved, and on Linux the D-Bus client the
accessibility bus needs is part of Unison, so `libdbus` is not required either.

Every widget in the library describes itself, so an application is largely accessible without doing anything. What
cannot be derived is what has no text to derive it from: a control whose appearance is its only label, such as an
icon-only `Button` or a `DrawablePanel`, has to be named, and a label that is not simply the sibling preceding the
control it names has to say what it labels.

```go
search.Accessibility.Name = "Search"  // Name a control that has no text of its own
field.Accessibility.LabeledBy = label // Point a control at the label that names it
```

Everything an application says about a panel goes through its `Accessibility` field, an `AccessibilityInfo` held in the
panel by value, so its fields are set in place rather than allocated. All of them are optional: `Name` is what is
announced, overriding whatever name would otherwise be derived; `Description` elaborates on it, defaulting to the
panel's tooltip text; `Role` says what kind of element the panel is, with `role.Auto` deriving it from the widget and
`role.None` hiding the panel and promoting its children into its parent; `LabeledBy` names the panel that labels this
one; `Callback` runs last and may adjust anything on the finished node, which is how a fact with no field of its own,
such as a heading's level, gets reported; and `ActionCallback` handles requests from an assistive technology for a panel
with no widget type of its own.

A custom widget describes itself by implementing `AccessibilityProvider`, and carries out what an assistive technology
asks of it by implementing `AccessibilityActor`. Both are looked for on `Panel.Self`, which a widget's constructor sets
to the widget itself, so they have to be implemented by the widget type rather than by the embedded `Panel`. The node
handed to `ProvideAccessibility` already holds everything derivable from the panel alone — its bounds, whether it is
enabled, focusable and focused, and the actions those imply — so an implementation sets only what it knows better:

```go
type Rating struct {
	unison.Panel
	stars int
}

func (r *Rating) ProvideAccessibility(b *unison.AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto { // Auto unless the application asked for something else
		node.Role = role.Slider
	}
	node.HasNumber = true
	node.Number = float64(r.stars)
	node.Max = 5
	node.Step = 1
	node.Actions = node.Actions.With(accessibility.Increment, accessibility.Decrement)
}

func (r *Rating) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	switch req.Action {
	case accessibility.Increment:
		r.stars = min(r.stars+1, 5)
	case accessibility.Decrement:
		r.stars = max(r.stars-1, 0)
	default:
		return false
	}
	r.MarkForRedraw()
	return true
}
```

A widget that draws more content than it can show at once uses `AccessibilityBuilder.VisibleRect()` to describe only
what can be seen, and one that draws elements without a panel apiece — the rows and cells of `Table` and `List`, say —
adds them with `AccessibilityBuilder.AddVirtualChild()`. `AnnounceForAccessibility()` speaks a message that no change to
a window expresses, such as a background task finishing; it does nothing when no assistive technology is being served,
so it may be called unconditionally, and it is safe to call from any goroutine.

Until an assistive technology has actually asked for it, none of this costs anything beyond the `AccessibilityInfo` each
panel carries and one atomic load per pass of the event loop that redrew something: no hierarchy is walked, nothing is
allocated, and no goroutine or platform object exists. What counts as asking differs by platform. On macOS it is the
first accessibility query a window's content view receives, which is what VoiceOver or Accessibility Inspector sends on
reaching the application. On Windows it is the first `WM_GETOBJECT` asking for the UI Automation root object, and events
are raised only while UI Automation reports a listening client. On Linux it is the accessibility bus launcher's
`org.a11y.Status.IsEnabled` property, read once at startup over the session bus connection the color-scheme watcher
already keeps and then watched, so a screen reader started or stopped while the application runs is followed both ways;
`NO_AT_BRIDGE=1` is honored there exactly as it is for GTK. Setting `UNISON_ACCESSIBILITY=1` turns support on whatever
the platform reports, which is useful for seeing what a screen reader would be told, `UNISON_ACCESSIBILITY=0` refuses it
entirely, and the `NoAccessibility()` startup option refuses it from the start. `SetAccessibilityEnabled(false)` turns
it off while the application runs, shutting down whatever is being served and freeing everything built for it, and
`SetAccessibilityEnabled(true)` lets the next request start it again, so an application can leave the decision to a
preference rather than to whatever on the desktop happens to ask.

What an assistive technology would be handed can be asserted on in a headless session, with no display and no screen
reader involved. `AccessibilityTree()` turns support on if it is not on already, describes the window as it is now and
hands back the immutable tree a platform adapter would have been given; `AccessibilityNodeFor()` finds the node
describing one panel within it. `AccessibilityEvents()` drains the events published for a window, `Announcements()`
drains what `AnnounceForAccessibility()` was asked to speak, and `PerformAccessibilityAction()` makes a request of a
node exactly as a screen reader would.

```go
screen.Do(func() { wnd.SetFocus(field) })
tree := screen.AccessibilityTree(wnd)
node := screen.AccessibilityNodeFor(field)
if node.Role != role.TextField || node.Name != "Search" {
	t.Errorf("expected a text field named Search, got a %v named %q", node.Role, node.Name)
}
if tree.Focus != node.ID {
	t.Error("expected the field to hold the focus")
}
if !screen.PerformAccessibilityAction(accessibility.ActionRequest{
	Node:   node.ID,
	Action: accessibility.SetValue,
	Value:  "hello",
}) {
	t.Error("expected the field to accept a new value")
}
```
