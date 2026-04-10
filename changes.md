# Changes related to reducing memory growth and fixing Linux window placement

## Goal of the changes

This series of changes had two main goals:
- reduce steady memory growth visible during normal application use,
- fix decorated window placement on Linux, where the title bar could end up above the top edge of the screen.

The memory issue was especially visible in two scenarios:
- when using `ProgressBar` in `indeterminate` mode,
- during normal application use, such as clicking buttons and other frequent redraw activity.

The window placement problem was observed on Fedora KDE, but its root cause points to a broader
class of issues related to how GLFW and the compositor report window decorations.

---

## Part 1: memory growth during redraw

### What caused the problem

The drawing path created a large number of short-lived objects wrapping native Skia resources, mainly:
- `Paint`,
- `Path`,
- `Shader`,
- `TextBlob`.

Their release previously depended mostly on:
- the Go garbage collector,
- `runtime.AddCleanup`,
- the `ReleaseOnUIThread()` queue.

Under heavy redraw, this meant that the number of native objects being created could grow faster
 than the system was able to release them.

Symptoms:
- the application showed steady memory growth during use,
- it was most visible with `ProgressBar` in `indeterminate` mode,
- but it also appeared during normal application interaction, without any special stress test.

This was not a classic leak caused by losing references in Go logic. It was mainly the result
of releasing a large number of short-lived native objects too late.

### What was fixed

#### Deterministic release of `Paint`
An explicit `Dispose()` was added for `Paint`, and temporary objects were attached to the lifecycle
 of a single frame.

Files:
- `paint.go`
- `canvas.go`
- `color.go`
- `gradient.go`
- `pattern.go`

#### Deterministic release of `Path`
A similar mechanism was added for temporary `Path` objects created while drawing controls.

Files:
- `path.go`
- `canvas.go`
- `line_border.go`
- `check_box.go`
- `dock_tab.go`
- `well.go`
- `popup_menu.go`

#### Deterministic release of `Shader`
Dynamically created `Shader` objects were also moved under controlled per-frame cleanup.

Files:
- `shader.go`
- `canvas.go`
- `gradient.go`
- `pattern.go`

#### Deterministic release of `TextBlob`
`TextBlob` received an explicit `Dispose()`, and `Canvas` was connected to releasing them
 after `Flush()`.

Files:
- `text_blob.go`
- `canvas.go`

### How the current model works

Temporary resources created only for the current frame are now registered in `Canvas`
 and released after `Flush()` completes.

This reduces the risk of further memory growth during:
- animation,
- continuous redraw,
- hover changes,
- clicking,
- normal UI interaction.

---

## Part 2: decorated window placement on Linux

### What caused the problem

The code that converted window frame extents interpreted the values returned
 by `GLFW.GetFrameSize()` incorrectly.

Instead of treating them as edge thicknesses, frame width and height were calculated as differences:
- `right - left`
- `bottom - top`

That was incorrect.

In addition, on Fedora KDE `GLFW.GetFrameSize()` could return all zeros even though window
 decorations were clearly present. As a result, the window was positioned so that its content
 started correctly inside the work area, but the title bar ended up above the top edge of the
 monitor and was not visible.

### What was fixed

#### Correct frame extent math
Frame width and height are now calculated as the sum of both sides:
- `left + right`
- `top + bottom`

Files:
- `window_linux.go`
- `window_darwin.go`
- `window_windows.go`

#### Better handling of requested `FrameRect`
On Linux, the requested `FrameRect` is now remembered and window placement is retried after
`Show()`, using the requested frame as the source of truth.

Files:
- `window.go`
- `window_linux.go`

#### Fallback for environments where GLFW reports zero decoration extents
A Linux fallback was added for cases where `GLFW.GetFrameSize()` returns `0,0,0,0`
for a decorated window.

This workaround was added specifically because the problem appeared on Fedora KDE, but it may
also help in other Linux environments if the visible symptom is caused by the same class
of decoration reporting failure in GLFW or the compositor.

### Test application for reproduction
A small test application was added to verify title-bar placement:
- `cmd/windowbar/main.go`

It makes it easy to check whether a decorated window placed at `display.Usable.Point`
actually shows its title bar and what frame/content rectangles are being reported.

---

## What may still need attention

### On the memory side
It is still worth watching places that may dynamically create:
- `ColorFilter`,
- `MaskFilter`,
- `ImageFilter`,
- `PathEffect`.

Especially if they are created per frame outside the paths already fixed.

### On the Linux window side
The decoration fallback is a practical workaround, but it is not a perfect source of truth.
If more environments appear where decoration reporting behaves differently, we may still need:
- cached runtime-detected insets,
- or more refined compositor-specific heuristics.

---

## Summary

The most important effects of this series of changes are:
- the main sources of steady memory growth during redraw were removed,
- the symptoms seen with `ProgressBar` in `indeterminate` mode and during normal application use
  were fixed or significantly reduced,
- decorated window placement on Linux was improved,
- workarounds were added for environments such as Fedora KDE, where GLFW may not report
  decoration extents correctly.

This does not mean that no future path can ever introduce new memory growth or another window
decoration issue. It does mean that the main identified problems were fixed or clearly reduced.