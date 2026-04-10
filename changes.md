# Changes related to reducing memory growth

## Goal of the changes

The goal of this series of fixes was to reduce visible memory growth during redraw, clicks,
and animations, especially in `cmd/example`.

The problem was most noticeable in two scenarios:
- when clicking buttons,
- when running `ProgressBar` in `indeterminate` mode.

This did not look like a classic leak caused by losing references in Go logic. The main cause
was the creation of a large number of short-lived native Skia objects during drawing,
combined with releasing them too late.

---

## What caused the problem

During each redraw, temporary objects wrapping native resources were created, including:
- `Paint,
- `Path`,
- `Shader`,
- `TextBlob`.

Their release previously depended mostly on:
- the Go garbage collector,
- `runtime.AddCleanup`,
- and the `ReleaseOnUIThread()` queue.

That meant that under heavy redraw, the number of allocated objects could grow faster than
the system was able to clean them up.

The issue was especially visible when:
- redraw covered the whole window instead of only the changed area,
- a control scheduled repeated redraw on its own,
- the user performed many quick interactions,
- drawing created new objects per frame.

`ProgressBar` in `indeterminate` mode was a good example because it continuously forced redraw
through a timer.

---

## What was fixed

### 1. Deterministic release of `Paint`

An explicit `Dispose()` was added for `Paint`, and temporary objects were attached to the lifecycle
of a single frame.

Effect:
- `Paint` created for the current draw no longer depends only on the GC,
- it is released after `Flush()` completes.

Files involved in this part of the change:
- `paint.go`
- `canvas.go`
- `color.go`
- `gradient.go`
- `pattern.go`

### 2. Deterministic release of `Path`

A similar mechanism was added for temporary `Path` objects created during drawing of some controls.

Files involved in this part of the change:
- `path.go`
- `canvas.go`
- `line_border.go`
- `check_box.go`
- `dock_tab.go`
- `well.go`
- `popup_menu.go`

### 3. Deterministic release of `Shader`

Dynamically created `Shader` objects were also moved under controlled release after the frame ends.

Files involved in this part of the change:
- `shader.go`
- `canvas.go`
- `gradient.go`
- `pattern.go`

### 4. Deterministic release of `TextBlob`

`TextBlob` received an explicit `Dispose()`, and its usage was attached to `Canvas` so that
temporary text blobs no longer depend only on runtime cleanup.

Files involved in this part of the change:
- `text_blob.go`
- `canvas.go`

---

## How the current model works

The current model is based on temporary resources that live for a single frame.

In practice, this means:
- objects created only for the current draw are registered in `Canvas`,
- after `Flush()` they are released deterministically,
- the GC is no longer the only recovery mechanism for these paths.

This significantly reduces the risk of memory growth during:
- animations,
- frequent clicking,
- hover changes,
- full-window redraw.

