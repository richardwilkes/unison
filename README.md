[![Go Reference](https://pkg.go.dev/badge/github.com/richardwilkes/unison.svg)](https://pkg.go.dev/github.com/richardwilkes/unison)
[![Go Report Card](https://goreportcard.com/badge/github.com/richardwilkes/unison)](https://goreportcard.com/report/github.com/richardwilkes/unison)

# Unison

A unified graphical user experience toolkit for Go desktop applications. macOS, Windows, and Linux are supported.

### Required setup

Unison is built upon [glfw](https://github.com/go-gl/glfw). As such, it requires some setup prior to being able to build
correctly:

* On macOS, you need Xcode or Command Line Tools for Xcode (`xcode-select --install`) for required headers and
  libraries.
* On Ubuntu/Debian-like Linux distributions, you need `libgl1-mesa-dev` and `xorg-dev` packages.
* On CentOS/Fedora-like Linux distributions, you
  need `libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel mesa-libGL-devel libXi-devel libXxf86vm-devel`
  packages.
* On Windows, you need https://jmeubank.github.io/tdm-gcc/download/ as well as https://git-scm.com for its bash shell.
* See [compilation dependencies](http://www.glfw.org/docs/latest/compile.html#compile_deps) for full details.

This version of Unison was built using Go 1.19. It has been compiled under many earlier versions of Go in the past, but
only Go 1.19+ will be considered as I make further changes.

### Example

An example application can be found in the `example` directory:

```
go run example/main.go
```

### Notes

Unison was developed with the needs of my personal projects in mind, so may not be a good fit for your particular needs.
I'm open to suggestions on ways to improve the code and will happily consider Pull Requests with bug fixes or feature
additions.

#### Compatibility

Unison is very much a work in progress. As such, it is likely to have breaking changes. To reflect this, a version
number of 0.x.x will be in use until such time that I'm comfortable locking things down to ensure compatibility between
releases. Please keep this in mind when making the decision to use Unison in your own code.

#### Look & Feel

Unison defines its own look and feel for widgets and will likely be adjusted over time. This was done to provide as much
consistency as possible between all supported platforms. It also side-steps issues where a given platform itself has no
or poorly defined standards. Colors, fonts, spacing, how the widgets behave, and more are customizable, so if you are
feeling particularly ambitious, you could create your own theming that matches a given platform.

#### Organization

There are a large number of Go source files in a single, top-level package. Unison didn't start out this way, but user
experience code tends to need to have its tentacles in many places, and the logical separations I made kept hindering
the ability to do things. Ultimately, I made the decision to collapse nearly everything into a single package to
simplify development and greatly reduce the overall complexity of things.
