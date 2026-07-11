# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Unison is a cross-platform (macOS, Windows, Linux) GUI toolkit for Go desktop applications. It renders with OpenGL, and
defines its own consistent look & feel rather than using native widgets. Requires Go 1.26+. The module is **pure Go —
cgo is forbidden** (`build.sh` enforces `CGO_ENABLED=0` and fails on any `import "C"`); OS libraries are reached at
runtime via `github.com/ebitengine/purego` instead. Consequently, cross-compilation works from any host (e.g.
`GOOS=linux GOARCH=arm64 go build ./...` from macOS — CI cross-builds all six OS/arch pairs). The runtime library
requirements are listed in [README.md](README.md#required-setup).

## Commands

```sh
./build.sh            # Generate enums, then `go build -v ./...`
./build.sh --lint     # Also install/run golangci-lint (pinned to latest release)
./build.sh --test     # Also run `go test ./...`
./build.sh --race     # Tests with -race
./build.sh --all      # Lint + race tests
go generate ./cmd/enumgen/main.go   # Regenerate enum sources only
go run ./cmd/example/main.go        # Run the demo app exercising most widgets

go test -run TestColorBlend ./...   # Run a single test by name
```

There is a focused unit-test suite covering the parts that can be exercised without a live GUI — color math, layouts
([flex_layout_test.go](flex_layout_test.go), [flow_layout_test.go](flow_layout_test.go),
[dock_layout_test.go](dock_layout_test.go)), text fields ([field_edit_test.go](field_edit_test.go)), panel
dispatch/transform, lists, tables, markdown, SVG, tasks, and undo. Most widget and rendering behavior still has no
automated coverage, so changes are usually validated by running the example app, not by tests.

## Architecture

### Single top-level package

Nearly everything lives in one flat `unison` package at the repo root. This is intentional (see
[README.md](README.md#organization)) — UI code crosses concerns constantly, so the author collapsed logical separations
into one package. Don't try to introduce sub-packages for widgets; follow the existing convention of one `.go` file per
widget/concept (e.g. `button.go`, `table.go`, `field.go`).

### Panel — the core abstraction

[panel.go](panel.go) defines `Panel`, the base UI element, and the `Paneler` interface (`AsPanel() *Panel`). Every
widget embeds `*Panel` (or `Panel`) and **must set `Self` to the final concrete object during construction** — failure
to do so breaks behavior dispatch. Widgets customize behavior by assigning callback fields (`DrawCallback`,
`UpdateCursorCallback`, etc.) and by registering "can-perform"/"perform" handlers keyed by action ID. Layout is
pluggable via the `Layout`/`Sizer` interfaces (`flex_layout.go`, `flow_layout.go`).

### Platform abstraction via build-tag file suffixes

Cross-platform code is split by filename suffix, not build tags in most cases:

- `foo.go` — shared logic and the public API
- `foo_darwin.go`, `foo_linux.go`, `foo_windows.go` — per-OS implementations
- `foo_other.go` — fallback for platforms without a specific need

The actual OS integration (windowing, menus, dialogs, clipboard, drag & drop) lives in `internal/`, all of it pure Go:

- `internal/mac/` — Cocoa via `purego/objc`: Objective-C classes are registered from Go (`objc.RegisterClass`) and
  AppKit is driven through `objc_msgSend`; `objc_darwin.go` holds the shared helper layer and documents the ABI
  constraints. This package has a real headless test suite (windows, views, IME, menus, pasteboard, panels) built on
  the `runOnMain` main-thread pump in `testmain_darwin_test.go`.
- `internal/w32/` — Win32 API bindings via `golang.org/x/sys/windows`-style syscalls
- `internal/x11/` — a pure-Go X11 wire-protocol implementation for Linux, plus GLX via purego/dlopen
  (`glx_linux.go`)

When adding a feature that touches the OS, expect to implement it three times (one file per platform) plus the shared
API file — all three implementations are Go; never introduce cgo. Note that `.golangci.yml` excludes `internal/mac/`
and `internal/w32/` from linting, so review code there manually.

### Generated enums

Enum types in `enums/<name>/` are **fully generated** — the `<name>_gen.go` file is the only file in each enum dir and
must not be hand-edited. The source of truth is the data tables inside [cmd/enumgen/main.go](cmd/enumgen/main.go): to
add or change an enum value, edit the `enumInfo`/`enumValue` structs there, then run `go generate ./cmd/enumgen/main.go`
(also invoked automatically by `build.sh`). Generated enums get `String()`, `Key()`, JSON/YAML marshaling, localization,
and an `All` slice. Localized strings use the toolbox `i18n` package.

### Application lifecycle

[app.go](app.go) drives startup/shutdown on the locked main OS thread (`runtime.LockOSThread` in `init`). Apps call
`unison.Start(opts...)` (never returns) and create windows inside a `StartupFinishedCallback`. See
[cmd/example/main.go](cmd/example/main.go) for the canonical entry point and [cmd/example/demo/](cmd/example/demo/) for
usage of dock, tables, markdown, SVG, and theming.

## Other commands in `cmd/`

- `cmd/upack` — `upack`, the app packager for distribution (installed by `build.sh`); per-platform packaging in
  `cmd/upack/packager/`
- `cmd/printerscan` — IPP printer discovery utility (see `printing/`)
- `cmd/enumgen` — the enum generator described above

## Dependencies & conventions

- Built heavily on `github.com/richardwilkes/toolbox/v2` (`xos`, `geom`, `i18n`, `errs`, `xflag`, `xslog`, `xreflect`,
  …) — prefer these helpers over reinventing them; e.g. geometry uses `toolbox/v2/geom` types (`geom.Rect`,
  `geom.Point`), not a local package.
- All source files carry the MPL-2.0 "Incompatible With Secondary Licenses" header — copy it onto any new file.
- Lint config is in [.golangci.yml](.golangci.yml); run `./build.sh --lint` before finishing changes.
