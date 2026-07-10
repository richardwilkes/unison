# Plan: Remove all cgo usage from unison

## Goal

Make the entire module build with `CGO_ENABLED=0` on macOS, Linux, and Windows by replacing the remaining cgo bridges
with purego (github.com/ebitengine/purego) and pure-Go equivalents, without changing unison's public API or observable
behavior.

## Current cgo inventory

After the canvas-branch migration (cgo Skia → pure-Go github.com/richardwilkes/canvas), exactly two areas still use
cgo:

1. **`internal/mac/`** — the Cocoa bridge. `all_darwin.go` (~1,374 lines) declares ~150 C bridge functions and 33
   `//export go*Callback` functions, backed by 18 Objective-C `.m` files (~1,500 lines) plus `macos.h`. It defines 7
   custom Objective-C classes: `macAppDelegate`, `macWindow`, `macWindowDelegate`, `macContentView` (an `NSView`
   subclass implementing `NSTextInputClient` and `NSDraggingSource`), `MenuDelegate`, `MenuItemDelegate`, and
   `ThemeDelegate`. Functional areas: app lifecycle/activation, event loop, windows, views (mouse/key/tracking
   areas/drawing), IME text input, menus + validation, cursors, images, pasteboard, drag & drop (source and
   destination), open/save panels, screens, theme-change observation, beep, and `NSOpenGLContext`/pixel format.

2. **`internal/x11/glx_linux.go`** (~216 lines) — GLX context management on Linux via
   `#cgo linux pkg-config: x11 gl`. Uses four Xlib calls (`XOpenDisplay`, `XCloseDisplay`, `XFree`, `XSync`) and nine
   GLX calls (`glXChooseFBConfig`, `glXGetVisualFromFBConfig`, `glXGetFBConfigAttrib`, `glXCreateWindow`,
   `glXMakeContextCurrent`, `glXSwapBuffers`, `glXDestroyWindow`, `glXDestroyContext`, and
   `glXGetProcAddressARB` → `glXCreateContextAttribsARB`). This is a *separate* X connection used only for GL; all
   other X11 work already goes through the pure-Go wire-protocol implementation in `internal/x11`.

Everything else is already cgo-free: `internal/w32` is `golang.org/x/sys`-style syscall bindings, the X11 protocol
code speaks the wire protocol over a socket in pure Go, and the canvas library loads OpenGL functions itself via
purego (`canvas/gpu/gl/native_{darwin,linux,windows}.go`) — so unison only has to provide context
creation/make-current/swap, not GL function loading.

Prior art to lean on: the canvas repo's `gpu/gl/native_darwin.go` (purego + dlopen pattern) and Ebitengine, which is
fully cgo-free on macOS using `purego/objc` for exactly this kind of AppKit work (window, view, event loop, IME).

## Phase 0 — Feasibility spike and dependency setup

- [x] Promote `github.com/ebitengine/purego` from indirect to direct dependency in `go.mod` (v0.10.1 already in the
      module graph via canvas).
- [x] Spike (throwaway program, not committed): using `purego/objc`, register an `NSView` subclass with
      `objc.RegisterClass` that implements `drawRect:` (struct *argument*, `NSRect` = 32 bytes),
      `firstRectForCharacterRange:actualRange:` (struct *return* > 16 bytes, uses the hidden indirect-return
      pointer), and `markedRange` (16-byte struct return in registers). Verify on both darwin/arm64 and darwin/amd64.
      purego's objc package has struct type-encoding support (`maxRegAllocStructSize`, struct encoders), but this is
      the single biggest technical risk of the whole migration and must be proven first.
- [x] In the same spike, verify: `objc_msgSend` calls that pass/return `CGRect`/`CGPoint`/`NSRange`;
      `purego.NewCallback` for plain C callbacks; autorelease pool push/pop (`objc_autoreleasePoolPush`/`Pop`).
- [x] Decide the shared helper set the real port will use and put it in a new `internal/mac/objc_darwin.go` (or reuse
      `purego/objc` directly): class/selector caching, NSString⇄Go string, NSArray⇄slice, CGRect/CGPoint/NSRange Go
      struct definitions matching the C ABI.

Exit criteria: spike renders into a window, receives mouse/key events, and round-trips a struct-returning
`NSTextInputClient` method without crashing on both architectures.

**Spike outcome (see progress.md session 2)**: all exit criteria met on darwin/arm64 and darwin/amd64, but only
after upgrading purego to **v0.11.0-alpha.6** — v0.10.1's callbacks reject struct args/returns outright, so the
whole Phase 2 port hard-requires the v0.11 line. One purego bug found (amd64 call-side splitting of a 16-byte
struct arg that straddles the register/stack boundary); the AppKit→Go direction is unaffected. Details and the
resulting call-shape constraint are documented in `internal/mac/objc_darwin.go`.

## Phase 1 — Linux: port `internal/x11/glx_linux.go` to purego (small, do first)

- [x] Rewrite `glx_linux.go` with no `import "C"`:
      - `purego.Dlopen("libX11.so.6", ...)` and `purego.Dlopen("libGL.so.1", ...)` at first use;
        `purego.RegisterLibFunc` for the 13 functions listed in the inventory. Resolve
        `glXCreateContextAttribsARB` through `glXGetProcAddressARB`, then call it via `purego.SyscallN` or a
        registered func — this replaces the C `createContext` helper verbatim (attribs 3.2 core, same array).
      - Replace `C.Display*`/`C.GLXFBConfig`/`C.GLXContext`/`C.GLXWindow` with `uintptr`-based named types, keeping
        the exported names (`Display`, `FBConfig`, `GLXContext`, `GLXWindow`) and every method on `GLX` identical so
        no caller changes.
      - `XVisualInfo` becomes a Go struct matching the C layout (visualid/ depth extraction only); `XFree` stays for
        server-allocated arrays.
- [x] Keep the two-connection design (GLX on its own display connection) — XIDs are server-global, so windows created
      by the pure-Go connection remain valid GLX drawables. Preserve the transparent-visual selection logic and the
      NVIDIA BadMatch comment/behavior exactly.
- [x] Verify `GOOS=linux CGO_ENABLED=0 go build ./...` passes and the example app runs on X11 and XWayland
      (`clipboard_live_test.go` in `internal/x11` still passes). — Cross-build verified in Session 1; Rich ran the
      checked-in code on a live Linux machine (2026-07-10) and reports it working.
- [x] Fallback library names: try `libGL.so.1` then `libGL.so`; `libX11.so.6` then `libX11.so`; fail with a clear
      error mentioning the package to install.

Estimated size: one session.

## Phase 2 — macOS: port `internal/mac` to purego/objc (the big one)

Strategy: keep the exported Go API of `internal/mac` byte-for-byte identical (same function names, signatures, and
semantics) so nothing in the unison root package changes. Replace the implementation file-by-file; each `.m` file
maps to one pure-Go `_darwin.go` file. Load AppKit with
`purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_GLOBAL)` (plus Foundation /
CoreGraphics as needed) — dlopen also makes the Objective-C classes visible to the runtime.

Suggested order (each step leaves the package compiling; use a temporary build tag split only if a step must land
half-done):

- [x] **Foundation helpers** (`objc_darwin.go`): class/SEL caches, NSString/CFString⇄string, NSArray⇄[]uintptr,
      NSNumber, NSURL⇄file path, CGRect/CGPoint/CGSize/NSRange structs, autorelease-pool helpers, retain/release
      helpers mirroring the current CFRetain/CFRelease ownership rules of the bridge (the `CFTypeRef`-based handle
      discipline in `macos.h` should carry over: handles stay `uintptr`, ownership stays explicit).
- [x] **Sound, theme, screen, image, cursor** — the five trivial files (`beep`, effective-appearance observation via
      `ThemeDelegate` + KVO or `NSDistributedNotificationCenter`, `NSScreen` queries, `NSImage` from RGBA pixels via
      `NSBitmapImageRep`, `NSCursor` constructors). These prove the helper layer with minimal risk. — Done in
      Session 3 (including the CGDirectDisplay functions, which the screen code depends on); the five `.m` files are
      deleted and each area has unit tests.
- [x] **App + event loop** (`app_darwin.m`, `event_darwin.m`): register `macAppDelegate` via `objc.RegisterClass`
      with the 6 delegate methods calling the existing Go callback funcs directly (no more `//export`);
      `finishLaunching`, activation, hide/unhide, `setMainMenu` etc.; `pollEvents`/`waitEvents`/`waitEventsTimeout`
      via `nextEventMatchingMask:untilDate:inMode:dequeue:` + `sendEvent:`, `postEmptyEvent`, `stopMainEventLoop`.
      Wrap each event-loop turn in an autorelease pool (the ObjC code currently gets this from `@autoreleasepool`;
      pure Go must do it explicitly). — Done in Session 4; the Cmd+keyUp local monitor is an Objective-C block
      created from Go (`objc.NewBlock`), and `WithPool` was hardened to pin the OS thread (see progress.md).
- [x] **Window + window delegate** (`window_darwin.m`, `window_delegate_darwin.m`): register `macWindow` (overrides
      `canBecomeKeyWindow`/`canBecomeMainWindow`) and `macWindowDelegate` (resize/move/miniaturize/key/should-close
      → Go callbacks). Port the ~30 window functions (frame/content-rect math uses CGRect by-pointer variants —
      keep the same out-parameter style internally). — Done in Session 5; the by-pointer style was dropped in favor
      of direct struct returns (`objc.Send[NSRect]`), which the Session 2/3 struct-return verification already
      covers on both arches.
- [ ] **View** (`view_darwin.m`, 350 lines — the riskiest file): register `macContentView` with all mouse/key/
      tracking/drawing overrides (`drawRect:`, `updateLayer`, `wantsUpdateLayer`, `updateTrackingAreas`,
      `cursorUpdate:`, `viewDidChangeBackingProperties`, `acceptsFirstResponder`, …) wired to the 15
      `goWindow*Callback` functions. Then the `NSTextInputClient` protocol methods (IME: marked text, insertText,
      firstRectForCharacterRange, doCommandBySelector) — this is where the Phase 0 struct-return spike pays off.
      Declare protocol conformance when registering the class so AppKit routes IME correctly.
- [ ] **OpenGL context + pixel format** (`opengl_context_darwin.m`, `opengl_pixel_format_darwin.m`):
      `NSOpenGLPixelFormat`/`NSOpenGLContext` via objc_msgSend (deprecated API, unchanged behavior; the transparent
      surface opacity handling moves to Go).
- [ ] **Menus** (`menu_darwin.m`, `menu_item_darwin.m`): `MenuDelegate` (`menuNeedsUpdate:` → `goUpdateMenuCallback`)
      and `MenuItemDelegate` (target/action + `validateMenuItem:` → the two Go callbacks); port the ~25 accessor
      functions and `menuPopup`.
- [ ] **Pasteboard, drag & drop** (`pasteboard_darwin.m`, `drag_darwin.m`): NSPasteboard read/write,
      NSPasteboardItem, NSDraggingItem, the drag-info accessors, `beginDraggingSessionWithItems:`, and the
      dragging-destination overrides on `macContentView` (registered in the view step).
- [ ] **Open/save panels** (`open_panel_darwin.m`, `save_panel_darwin.m`): NSOpenPanel/NSSavePanel accessors +
      `runModal`. `openPanelSetAllowedFileTypes` should keep using the same underlying property the ObjC code uses
      today (allowedFileTypes vs UTType-based — match current behavior, note the deprecation separately).
- [ ] Delete all `.m` files, `macos.h`, and the cgo preamble/`//export` blocks from `all_darwin.go`; split the
      remaining pure-Go code into per-area files mirroring the old `.m` layout.
- [ ] Verify: `CGO_ENABLED=0 go build ./...` on darwin/arm64 and darwin/amd64; run `cmd/example` and exercise every
      ported area — windows (resize/move/minimize/zoom/close/focus), mouse + keyboard, **IME input (e.g. Japanese
      via the system input source)**, menus + key equivalents + validation, popup menus, cursors, clipboard
      cut/copy/paste, drag & drop both directions (text, files, custom data), open/save dialogs, multi-monitor +
      retina scale changes, dark/light theme switching, beep, transparent windows.

Estimated size: four to six sessions. Keep a checkpoint after each bullet — every step must leave `./build.sh`
green.

## Phase 3 — Build system, CI, and docs

- [ ] `build.sh`: build and test with `CGO_ENABLED=0` explicitly so cgo can't creep back in silently.
- [ ] Add a guard that fails the build if `import "C"` appears anywhere (grep in `build.sh` or a `depguard`/custom
      lint rule in `.golangci.yml`).
- [ ] README: rewrite the "Required setup" section — no more C/Objective-C toolchain, no `pkg-config x11 gl` build
      dependency on Linux (runtime shared libraries `libX11`/`libGL` are still required, now loaded via dlopen;
      document the package names). Note that cross-compilation now works (`GOOS=linux go build` from macOS, etc.)
      and consider adding a cross-build matrix (darwin/linux/windows × amd64/arm64) to CI since compile breakage on
      other platforms is no longer masked by cgo requiring native toolchains.
- [ ] `.claude/CLAUDE.md`: update the architecture notes (internal/mac is now purego/objc, not Objective-C + CGO;
      the "expect to implement it three times" guidance stands, but all three are now Go).
- [ ] Check `cmd/upack` packager for any cgo-era assumptions (dylib bundling, notarization notes) — likely none, but
      confirm.

Estimated size: one session.

## Risks and mitigations

- **Struct args/returns in Objective-C method implementations** (NSRect in `drawRect:`, struct-returning
  `NSTextInputClient` methods). Mitigated by the Phase 0 spike. Fallbacks if a specific shape is unsupported: ignore
  the struct argument and query state instead (`[view bounds]` instead of the `drawRect:` rect — the code already
  redraws the full dirty view), or restructure so the struct crosses as a pointer.
- **IME regressions** are the most likely user-visible breakage and the hardest to test automatically. Test manually
  with at least one CJK input source before considering Phase 2 done.
- **Objective-C exceptions** thrown across a purego boundary will abort the process without ObjC unwinding. The cgo
  bridge had essentially the same failure mode, but be deliberate about argument validation before msgSend calls
  (e.g. never pass nil where AppKit throws).
- **Ownership/lifetime bugs**: the current bridge uses CFRetain/CFRelease discipline with `CFTypeRef` handles. Port
  the ownership rules mechanically (every `new*` returns +1, matching release where the old code had one); an
  imbalance shows up as a use-after-free crash or a leak, both hard to bisect later. Consider a debug build-tag
  counter that tracks retain/release pairs per handle during bring-up.
- **Main-thread affinity**: AppKit requires the main thread; `app.go` already locks the main OS thread in `init`, and
  purego calls happen on the caller's thread, so behavior is unchanged — but nothing enforces it anymore at the
  language boundary. Keep any existing dispatch-to-main helpers intact.
- **Dynamic linking**: purego uses `cgo_import_dynamic`-style resolution, so `CGO_ENABLED=0` binaries still link
  dynamically against libSystem/libX11/libGL at runtime. That is expected and unavoidable for a GUI toolkit; document
  it rather than fight it.
- **purego version**: v0.10.1 is current in the graph; check release notes for newer versions before starting —
  callback/struct support has been steadily expanding, and a newer purego may simplify Phase 0.

## Explicit non-goals

- No behavior changes, no new features, no API changes in the `unison` package.
- No replacement of deprecated APIs (NSOpenGLContext, allowedFileTypes) — same API surface, new call mechanism.
  Migrating macOS rendering to Metal/CAMetalLayer is a separate future effort.
- No Wayland-native backend; Linux remains X11/XWayland.
