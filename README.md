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
* See [compilation dependencies](http://www.glfw.org/docs/latest/compile.html#compile_deps) for full details.

This version of Unison was built using Go 1.17.1. It has been compiled under many earlier versions of Go in the past,
but only Go 1.17.1+ will be considered as I make further changes.

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

#### Performance

There are some areas within Unison that still need to be optimized. On Windows and Linux platforms, for example, the
menus are particularly slow to appear. This is due to the use of top-level windows to back the menus that pop up. On
those platforms, various system animations occur that I've yet to find a way to disable programmatically, which causes a
noticeable delay as you move from menu to menu. I plan to fix this by using a hybrid model where a menu is only put into
a top-level window when it can't fit into the existing window's content area. Oddly enough, even if you force Unison to
use the per-window menus on macOS rather than macOS's standard global menus, there is no performance problem. This seems
to indicate that if I could find a way to elminate the system animations that Windows and Linux add when making a window
visible that the problem would go away without having to rework things into a hybrid display model.

#### Future Plans

- Fix menu performance by either discovering a way to remove the platform animations when windows are made visible, or
  introducing a hybrid menu display mode where menus that can be fully contained in the owning window are instead drawn
  inside the content area instead.
- Improve the generic file open and save dialogs (i.e. the ones that are used when there is no platform-specific
  version) to make them both more functional and better behaving.
- Improve the color well dialog to add additional ways to specify colors as well as adding a way to set gradients.
- More widgets...
