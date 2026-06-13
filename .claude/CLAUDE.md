# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Unison is a cross-platform (macOS, Windows, Linux) GUI toolkit for Go desktop applications. It renders with Skia (via CGO bindings) on an OpenGL context, and defines its own consistent look & feel rather than using native widgets. Requires Go 1.26+ and CGO; the platform C/Objective-C dependencies are listed in [README.md](README.md#required-setup).

## Commands

```sh
./build.sh            # Generate enums, then `go build -v ./...`
./build.sh --lint     # Also install/run golangci-lint (pinned to latest release)
./build.sh --test     # Also run `go test ./...`
./build.sh --race     # Tests with -race
./build.sh --all      # Lint + race tests
go generate ./cmd/enumgen/main.go   # Regenerate enum sources only
go run ./cmd/example/main.go        # Run the demo app exercising most widgets

go test -run TestColorAdjustments ./...   # Run a single test by name
```

There are very few tests ([color_test.go](color_test.go), [undo_manager_test.go](undo_manager_test.go)); this is primarily an interactive UI library, so changes are usually validated by running the example app, not by tests.

## Architecture

### Single top-level package

Nearly everything lives in one flat `unison` package at the repo root. This is intentional (see [README.md](README.md#organization)) — UI code crosses concerns constantly, so the author collapsed logical separations into one package. Don't try to introduce sub-packages for widgets; follow the existing convention of one `.go` file per widget/concept (e.g. `button.go`, `table.go`, `field.go`).

### Panel — the core abstraction

[panel.go](panel.go) defines `Panel`, the base UI element, and the `Paneler` interface (`AsPanel() *Panel`). Every widget embeds `*Panel` (or `Panel`) and **must set `Self` to the final concrete object during construction** — failure to do so breaks behavior dispatch. Widgets customize behavior by assigning callback fields (`DrawCallback`, `UpdateCursorCallback`, etc.) and by registering "can-perform"/"perform" handlers keyed by action ID. Layout is pluggable via the `Layout`/`Sizer` interfaces (`flex_layout.go`, `flow_layout.go`).

### Platform abstraction via build-tag file suffixes

Cross-platform code is split by filename suffix, not build tags in most cases:
- `foo.go` — shared logic and the public API
- `foo_darwin.go`, `foo_linux.go`, `foo_windows.go` — per-OS implementations
- `foo_other.go` — fallback for platforms without a specific need

The actual OS integration (windowing, menus, dialogs, clipboard, drag & drop, OpenGL context) lives in `internal/`:
- `internal/mac/` — Objective-C (`.m`) + CGO, Cocoa framework
- `internal/w32/` — Win32 API bindings
- `internal/x11/` — X11 bindings for Linux
- `internal/skia/` — Skia graphics bindings; prebuilt static libs (`libskia_*.a`) and `skia_windows.dll` are committed, with `sk_capi.h` as the C API surface

When adding a feature that touches the OS, expect to implement it three times (one file per platform) plus the shared API file.

### Skia rendering

Drawing goes through Skia: `Canvas`, `Paint`, `Path`, `Font`, `Image`, `Surface`, `Shader`, gradients, and filters all wrap Skia objects from `internal/skia`. These hold native memory — respect the existing dispose/finalizer patterns when creating wrappers.

### Generated enums

Enum types in `enums/<name>/` are **fully generated** — the `<name>_gen.go` file is the only file in each enum dir and must not be hand-edited. The source of truth is the data tables inside [cmd/enumgen/main.go](cmd/enumgen/main.go): to add or change an enum value, edit the `enumInfo`/`enumValue` structs there, then run `go generate ./cmd/enumgen/main.go` (also invoked automatically by `build.sh`). Generated enums get `String()`, `Key()`, JSON/YAML marshaling, localization, and an `All` slice. Localized strings use the toolbox `i18n` package.

### Application lifecycle

[app.go](app.go) drives startup/shutdown on the locked main OS thread (`runtime.LockOSThread` in `init`). Apps call `unison.Start(opts...)` (never returns) and create windows inside a `StartupFinishedCallback`. See [cmd/example/main.go](cmd/example/main.go) for the canonical entry point and [cmd/example/demo/](cmd/example/demo/) for usage of dock, tables, markdown, SVG, and theming.

## Other commands in `cmd/`

- `cmd/upack` — `upack`, the app packager for distribution (installed by `build.sh`); per-platform packaging in `cmd/upack/packager/`
- `cmd/printerscan` — IPP printer discovery utility (see `printing/`)
- `cmd/enumgen` — the enum generator described above

## Dependencies & conventions

- Built heavily on `github.com/richardwilkes/toolbox/v2` (`xos`, `geom`, `i18n`, `errs`, `xflag`, `xslog`, `xreflect`, …) — prefer these helpers over reinventing them; e.g. geometry uses `toolbox/v2/geom` types (`geom.Rect`, `geom.Point`), not a local package.
- All source files carry the MPL-2.0 "Incompatible With Secondary Licenses" header — copy it onto any new file.
- Lint config is in [.golangci.yml](.golangci.yml); run `./build.sh --lint` before finishing changes.
