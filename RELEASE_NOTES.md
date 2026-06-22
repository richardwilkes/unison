# Release Notes

This release replaces Unison's GLFW-based foundation with a native platform layer written from scratch for each OS. It
is a large, breaking change for any code that uses the clipboard or drag & drop, but most other breaking changes can be
handled with search & replace through your code base.

## Highlights

### Native platform layer (no more GLFW)

- Removed the `github.com/go-gl/glfw` and `github.com/go-gl/gl` dependencies entirely.
- Windowing, menus, dialogs, monitors, OpenGL context creation, and event handling are now implemented natively per
  platform:
  - **macOS** — Cocoa/Objective-C in `internal/mac` (replaces the old `internal/ns` package).
  - **Windows** — Win32 bindings in `internal/w32`.
  - **Linux** — a custom X11 protocol implementation in `internal/x11` (atoms, events, RandR/Render/XFixes extensions,
    GLX context), with no external X client library.

### Drag & drop overhaul

- New `unison/drag` package defining `drag.Data`, `drag.Op`, `drag.Info`, and `drag.Callbacks`.
- Drag types are now identified by UTIs (`*uti.DataType`) rather than free-form strings.
- Full native drag & drop on all three platforms, including dragging to/from other applications and showing a drag
  image. Mouse-wheel scrolling within Unison windows continues to work during a drag.

### Clipboard rewrite

- `Clipboard` struct and `GlobalClipboard` are gone, replaced by package-level functions: `ClipboardHasText`,
  `ClipboardGetText`, `ClipboardSetText`, `ClipboardHasDataType`, `ClipboardGetData`, `ClipboardSetData`.
- Clipboard data is now typed by UTI and carried as `[]byte`, with multi-type entries and proper system-clipboard
  integration (no longer limited to plain strings). Works consistently across macOS, Windows, and Linux.

### Cursors

- Standard cursors are now rendered from SVGs (the cursor PNG resources were removed) and scale to the display
  resolution.
- Added `OpenHandCursor` and `ClosedHandCursor`; added `DefaultCursorSize`.

### Gradient overhaul & color/gradient editing

- The `Gradient` struct was restructured and now supports linear, radial, sweep, and conical gradients via a new
  `enums/gradienttype` enum (`Linear`, `Radial`, `Sweep`, `Conical`). Points are now normalized (0–1), with separate
  `Radius` and `Angle` (`StartEnd`) fields and an explicit `Kind`.
- Added a `Stops` slice type with `Sort()`/`Reverse()` helpers and `NewEvenlySpacedGradientStopsForColors`.
- New `ColorEditor` (`NewColorEditor`) and `GradientEditor` (`NewGradientEditor`) widgets, and the Well dialog now
  supports editing gradients in addition to solid colors.

### Dark mode on Linux

- Unison now detects and tracks the system light/dark color-mode preference on Linux (via the XDG desktop-portal /
  xsettings over D-Bus), matching the existing macOS and Windows behavior. `IsColorModeTrackingPossible()` reflects
  this on Linux.

## Breaking API changes

- **Modifiers moved** from the `unison` package to `unison/enums/mod` (`mod.Modifiers`). Callback and method signatures
  using modifiers changed accordingly.
- **Clipboard API** replaced as described above.
- **Drag & drop API** replaced: `Panel.StartDataDrag`/`DragData` and the `DataDrag*Callback` fields are gone. Use
  `Panel.StartDrag(...)` / `Window.StartDrag(...)` and the `drag.Callbacks` hooks (`DragEnteredCallback`,
  `DragUpdatedCallback`, `DropCallback`, etc.). Windows now register interest via `RegisterForDragTypes` /
  `UnregisterForDragTypes` / `ClearRegisteredDragTypes`.
- **`enums/imgfmt`**: `UTI()` and `ForUTI()` now use `*uti.DataType` instead of `string`; added
  `AllReadableUTIs`/`AllWritableUTIs`.
- **`NewImageFromFilePathOrURL`** now takes a `context.Context`, an `*http.Client` (pass `nil` to use
  `http.DefaultClient`), and a `maxBytes` limit (`0` or less means no limit), in addition to the path/URL and scale.
  `NewImageFromFilePathOrURLWithContext` was removed.
- File-drop callbacks (e.g. on `Well`) replaced by the new drag enter/update/exit/drop callbacks.
- **`Gradient`** struct fields changed: `Start`/`End`/`StartRadius`/`EndRadius` are replaced by normalized `StartPt`/
  `EndPt` plus `Radius`/`Angle` (`StartEnd`) and a `Kind` (`gradienttype.Enum`); `Stops` is now a `Stops` type.
  `NewHorizontalEvenlySpacedGradient`, `NewVerticalEvenlySpacedGradient`, and `NewEvenlySpacedGradient` were removed
  (use `NewEvenlySpacedGradientStopsForColors`); `Gradient.Reversed()` is replaced by `Stops.Reverse()`.

## Other changes

- New `NoPlatformFileDialogs()` startup option to force Unison's own file dialogs on platforms that normally use native
  ones.
- New `SafeCall(f)` helper: panics from tasks (`InvokeTask`, `InvokeTaskAfter`) and from `SafeCall`-wrapped callbacks
  are now routed to a single recovery callback, falling back to `errs.Log` when none is set. Internal callback dispatch
  throughout unison now goes through `SafeCall`.
- The client-data keys Unison stores internally (e.g. on borders and dialogs) are now namespaced with a `unison.`
  prefix to avoid collisions with application-supplied keys.
- Adding a panel via `AddChild`/`AddChildAtIndex` now fails fast (exits with an error) if its `Self` field is nil or
  points at the wrong object, surfacing a common construction mistake immediately.
- More efficient task processing: the task queue now drains via a head index instead of shifting the slice on every
  dequeue, with periodic compaction to keep the backing array bounded.
- `UndoManager.Add()` no longer reallocates and copies the entire edit slice on every call; it now trims the released
  redo tail in place (clearing the vacated slots for garbage collection) and appends the new edit.
- Improved thread-safety around image handling: image creation is now locked, the image cache entry is removed when an
  image is disposed, and the global color filters are created lazily via `sync.Once`.
- Drops can now fall through to enclosing drop targets: a new `Panel.CanAcceptDropCallback` governs drop-target
  candidacy, so a panel that declines lets the search continue up the parent hierarchy. Added `CreatePrivateDataType`
  and `WellDragTypes()`.
- Most native-resource wrappers (`Paint`, `Path`, `Image`, `Shader`, `Color/Image/Mask` filters, `PathEffect`,
  `TextBlob`) now expose an idempotent `Dispose()` method to release their Skia memory immediately rather than waiting
  on finalizers; many widgets now dispose temporary resources as soon as they are done with them.
- Drop-target updates now arrive even when the mouse is stationary.
- `MouseWheelMultiplier` now defaults to per-OS values.
- Added `Beep()`.
- Windows apps launched from the command line can now bring their main window to the foreground at launch.
- Windows and Linux ARM64 is now part of the cross-platform CI build matrix.
- Requires **Go 1.26+**.

## Bug fixes

- Fixed a use-after-free when writing PDF metadata (strings are now allocated with `C.CString` and freed).
- Windows GL context: device context is released on all error paths and on destroy; the temporary RC is deleted on the
  make-current failure path.
- `SetTitleIcons` no longer duplicates title icons.
- Tables only fire a selection-change notification when pruning actually drops a row.
- Gradients with no stops no longer attempt shader creation.
- Fixed an NPE caused by a nil context in `surface.flush()`.
- Fixed incorrect radio-button text color on press in light mode.
- Fixed the scale of images produced by `NewImageFromDrawing`.
- Reduced per-event scrolling overhead.
- Restored cursor visibility when a window closes.
- Fixed menu click dispatch when menus overlap on platforms using non-native menus.
