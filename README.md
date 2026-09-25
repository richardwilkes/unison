# Unison

[![Go Reference](https://pkg.go.dev/badge/github.com/richardwilkes/unison.svg)](https://pkg.go.dev/github.com/richardwilkes/unison)
[![Build](https://github.com/richardwilkes/unison/actions/workflows/build.yml/badge.svg)](https://github.com/richardwilkes/unison/actions/workflows/build.yml)

Unison is a cross-platform GUI toolkit for Go desktop applications. It runs on macOS, Windows and Linux, draws its own
widgets so an application looks and behaves the same on all three, and is pure Go from the window system up: no C
toolchain and no bundled native libraries.

The API reference is on [pkg.go.dev](https://pkg.go.dev/github.com/richardwilkes/unison).

## Requirements

Go 1.27 or later. Unison builds with cgo disabled (`CGO_ENABLED=0`) on every platform, so cross-compilation just works,
e.g. `GOOS=linux GOARCH=arm64 go build` from a macOS host.

At runtime, the operating system's own libraries are loaded dynamically:

- **macOS**: no setup required.
- **Windows**: no setup required. Windows 10 or later.
- **Linux**: `libX11.so.6` and `libGL.so.1` must be present. Any desktop system already has both; minimal or container
  installs need the runtime packages, e.g. `libx11-6` and `libgl1` on Debian/Ubuntu, or `libX11` and `libglvnd-glx` on
  Fedora. Development headers and `pkg-config` are *not* required. Unison talks to the display server via X11, so
  Wayland desktops need XWayland, which is virtually always present.

Building with `GOEXPERIMENT=simd` switches the rendering kernels to vector implementations on amd64 and arm64. The
output is identical; only throughput changes.

## Getting started

```sh
go get github.com/richardwilkes/unison
```

```go
package main

import (
	"log"

	"github.com/richardwilkes/unison"
)

func main() {
	unison.Start(unison.StartupFinishedCallback(func() {
		wnd, err := unison.NewWindow("Hello")
		if err != nil {
			log.Fatal(err)
		}
		label := unison.NewLabel()
		label.SetTitle("Hello, world!")
		content := wnd.Content()
		content.SetLayout(&unison.FlexLayout{Columns: 1})
		content.AddChild(label)
		wnd.Pack()
		wnd.ToFront()
	})) // Never returns
}
```

Everything a window shows is a `Panel`. Widgets embed one, and layouts such as `FlexLayout` arrange them. `Start()` runs
the event loop and never returns, so an application creates its initial windows in the `StartupFinishedCallback`.

A larger demonstration covering most of the widgets lives in `cmd/example`:

```sh
go run ./cmd/example
```

On Windows, build with `-ldflags=-H=windowsgui` so that launching the application from Explorer does not open a console
window. When the application is launched from a terminal instead, Unison attaches to that terminal, so logging and
command-line usage keep working.

## What's included

- Widgets: buttons, check boxes, radio buttons, text fields (plain, numeric and combo), labels, links, popup menus,
  lists, tables with column headers, sorting, in-place editing and drag & drop, sliders, progress bars, scroll panels,
  separators, tooltips, and color, gradient and font pickers.
- Layouts: `FlexLayout`, `FlowLayout`, and a docking system for tool-window style user interfaces.
- Menus and menu bars, using the global menu bar on macOS, plus configurable key bindings.
- Message dialogs and file dialogs, using the platform's own where it has them.
- Drag & drop within an application, between applications and with the operating system.
- Text layout, Markdown rendering, SVG and bitmap images, and PDF output.
- Undo/redo management.
- Light and dark themes that follow the system, with customizable colors and fonts.
- Printing to network printers via IPP, in the `printing` package.
- Screen-reader support and display-free testing, described below.

## Threading

Unison is single-threaded: panels, windows, drawing, and the native graphics objects behind them are owned by one UI
thread and are not safe for concurrent use. Code invoked by Unison (input and draw callbacks, layout, command handlers,
`StartupFinishedCallback`) already runs on that thread. Work done on other goroutines must marshal back via
`InvokeTask`, `InvokeTaskAfter` or `InvokeTaskAndWait` before touching UI objects. See the package documentation for
the full threading model.

## Look & feel

Unison defines its own look and feel for widgets and will likely adjust it over time. This provides as much consistency
as possible between all supported platforms and side-steps issues where a platform itself has no, or poorly defined,
standards. Colors, fonts, spacing, how the widgets behave, and more are customizable, so if you are feeling particularly
ambitious, you could create your own theming that matches a given platform.

## Testing without a display

`StartHeadless()` runs a real application — its event loop, windows, focus handling, modal dialogs, menus and drawing —
against an in-memory screen instead of the operating system's. No display is needed, so user interface tests run on a
build machine with no windowing system at all. It runs `Start()` on its own goroutine and hands back a `*HeadlessScreen`
once the application has settled, so the windows the `StartupFinishedCallback` created already exist. Use the
`Headless(cfg)` startup option instead if your code calls `Start()` itself; in headless mode `Start()` returns when the
session ends.

A session behaves the same way on every host. Where the platforms differ in convention it follows the one shared by
everything other than macOS: the menu command key is `Control`, so `mod.OSMenuCommand()` returns it and the standard
actions and menu items bind to it, and `Modifiers.String()` spells modifiers out as `Ctrl+Shift+` rather than drawing
the macOS glyphs. `mod.SetPlatformNeutral()` is the switch the session turns on to get that, and it is put back when
the session ends. A wheel event scrolls by the amount it does on Windows and Linux, `MouseWheelMultiplier` being set to
that value for the session, the quit menu item is titled `Exit`, and the application menu gets none of the entries
macOS adds to its own. Where the platforms' screen readers differ, what an assistive technology is handed follows
Linux: a heading takes the focus while one is being served, a flat table is described as a table, and the block holding
the reading caret reports the focus. A request to open a URL in the browser is recorded rather than carried out, and
`OpenedURLs()` hands the requests back; `DefaultMarkdownLinkHandler` makes its request through `OpenBrowser()`, and an
application's own link handlers get the same treatment by calling that rather than `xos.OpenBrowser()`.

Input is injected in the screen's logical coordinate space, which is also the space window content rects are in;
`PanelCenter()` and `PanelPoint()` convert a widget's own coordinates into it. Every injection method — `Click()`,
`Drag()`, key presses and the rest — waits for the application to finish reacting before it returns: the callbacks have
run, the tasks they queued have run, and the redraws those asked for have been performed, which is what makes an
assertion right after a click meaningful. The one exception is a call made from the UI thread itself, as when a widget
callback drives the screen: there the events are only queued and the call returns at once. Work scheduled with
`InvokeTaskAfter()` is deliberately not waited for. Anything a test wants to read out of the application belongs to the
UI thread, so read it inside `Do()`. An application that never goes quiet — one whose `DrawCallback` marks itself for
redraw, say — would leave that wait hanging, so it is bounded by `HeadlessConfig.SyncTimeout` (10 seconds by default)
and giving up is reported through `Errors()`.

Drag & drop works too. A drag the application starts itself needs nothing special: `Drag()` posts the press and motions,
and a widget calling `StartDrag()` from its `MouseDragCallback` turns them into a drag exactly as it would on a real
platform. For data arriving from outside the application, such as files from a file manager, use `BeginExternalDrag()`
or `DropExternal()`. `Capture()` returns the screen as an image, for golden-file comparisons. Sessions run one after
another within a process, never side by side, so tests using one must not call `t.Parallel()`.

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

## Accessibility

Unison describes its windows to the assistive technology each operating system provides — NSAccessibility on macOS
(VoiceOver), UI Automation on Windows (Narrator, NVDA, JAWS) and AT-SPI2 on Linux (Orca) — so a screen reader can read
an application's controls, follow the keyboard focus, report what is being typed and act on the user's behalf. Like the
rest of Unison it is pure Go: no accessibility bridge or extra library is needed on any platform.

Every widget in the library describes itself, so an application is largely accessible without doing anything. What
cannot be derived is what has no text to derive it from: a control whose appearance is its only label, such as an
icon-only `Button` or a `DrawablePanel`, has to be named, and a label that is not simply the sibling preceding the
control it names has to say what it labels.

```go
search.Accessibility.Name = "Search"  // Name a control that has no text of its own
field.Accessibility.LabeledBy = label // Point a control at the label that names it
```

Everything an application says about a panel goes through its `Accessibility` field. All of its fields are optional:
`Name` is what is announced, overriding whatever name would otherwise be derived; `Description` elaborates on it,
defaulting to the panel's tooltip text; `Role` says what kind of element the panel is, with `role.Auto` deriving it from
the widget and `role.None` hiding the panel and promoting its children into its parent; `LabeledBy` names the panel that
labels this one; `Callback` runs last and may adjust anything on the finished node, which is how a fact with no field of
its own, such as a heading's level, gets reported; and `ActionCallback` handles requests from an assistive technology
for a panel with no widget type of its own.

A custom widget describes itself by implementing `AccessibilityProvider`, and carries out what an assistive technology
asks of it by implementing `AccessibilityActor`. Both must be implemented by the widget type itself rather than by the
embedded `Panel`, and both are looked up on `Panel.Self`, so a widget whose constructor does not point that at the
widget is never asked to describe or act on itself. The node handed to `ProvideAccessibility` already holds everything
derivable from the panel alone — its bounds, whether it is enabled, focusable and focused, and the actions those imply —
so an implementation sets only what it knows better:

```go
type Rating struct {
	unison.Panel
	stars int
}

func NewRating() *Rating {
	r := &Rating{}
	r.Self = r // Without this, the two methods below are never found
	return r
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
what can be seen, and one that draws elements without a panel apiece, as `Table` and `List` do for their rows and cells,
adds them with `AccessibilityBuilder.AddVirtualChild()`. `AnnounceForAccessibility()` speaks a message that no change to
a window expresses, such as a background task finishing; it does nothing when no assistive technology is listening, so
it may be called unconditionally, and it is safe to call from any goroutine.

None of this costs anything until an assistive technology actually asks for it: no hierarchy is walked and nothing is
allocated. To see what a screen reader would be told without running one, set `UNISON_ACCESSIBILITY=1` in the
environment. `UNISON_ACCESSIBILITY=0` refuses support entirely, as does `NO_AT_BRIDGE=1` on Linux. An application can
also decide for itself: the `NoAccessibility()` startup option refuses it from the start, and
`SetAccessibilityEnabled()` turns it off or back on while the application runs, so the decision can be left to a
preference rather than to whatever on the desktop happens to ask.

What an assistive technology would be handed can be asserted on in a headless session, with no display and no screen
reader involved. `AccessibilityTree()` describes a window as it is now and returns the tree a platform adapter would
have been given; `AccessibilityNodeFor()` finds the node describing one panel within it; `AccessibilityEvents()` drains
the events published for a window; `Announcements()` drains what `AnnounceForAccessibility()` was asked to speak; and
`PerformAccessibilityAction()` makes a request of a node exactly as a screen reader would.

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

## Packaging for distribution

`upack` turns a built executable into something you can ship. It reads a YAML description of the application — name,
icon, copyright, the file types it owns, and so on; the fields are those of the `Config` type in `cmd/upack/packager` —
and does the platform-specific work: on macOS it assembles the `.app` bundle with its `Info.plist` and icons around the
executable you built; on Windows it emits `.syso` resource files carrying the icon, version information and an
application manifest, which `go build` then links into the executable, so run it before building there. With `-dist` it
goes on to produce the distributable: a signed and notarized DMG on macOS, a ZIP on Windows and a tarball on Linux.

```sh
go install github.com/richardwilkes/unison/cmd/upack@latest
upack -r 1.2.3 -d app.yml
```

## Stability

Unison is developed first for my own projects, so it may not be a good fit for yours. It is also still a 0.x release
series: breaking changes happen between releases, and the version number will stay at 0.x until I'm comfortable locking
the API down. Please keep this in mind when deciding whether to use it. Bug reports, suggestions and pull requests with
fixes or features are welcome.

## License

Unison is released under the [Mozilla Public License 2.0](LICENSE).
