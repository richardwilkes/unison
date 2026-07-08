# Unison

[![Go Reference](https://pkg.go.dev/badge/github.com/richardwilkes/unison.svg)](https://pkg.go.dev/github.com/richardwilkes/unison)
[![Build](https://github.com/richardwilkes/unison/actions/workflows/build.yml/badge.svg)](https://github.com/richardwilkes/unison/actions/workflows/build.yml)

A unified graphical user experience toolkit for Go desktop applications. macOS, Windows, and Linux are supported.

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
`StartupFinishedCallback`) already runs on that thread; work done on other goroutines must marshal back via
`InvokeTask`, `InvokeTaskAfter`, or `ReleaseOnUIThread` before touching UI objects. See the package documentation for
the full threading model.
