# Progress

Running log of work sessions against [plan.md](plan.md) (removing all cgo usage from unison). Newest session first.

## Session 4 — 2026-07-10: Phase 2 — app + event loop ported to purego; app_darwin.m and event_darwin.m deleted

The "App + event loop" bullet of Phase 2 is done. `macAppDelegate` is now a Go-registered Objective-C class, the
Cmd+keyUp local event monitor is an Objective-C **block created from Go** (`objc.NewBlock` — first block in shipped
code; purego v0.11.0-alpha.6 has full block support via `__NSMallocBlock__`, which lives in libsystem so blocks work
before AppKit loads), and the event pump (`PollEvents`/`WaitEvents`/`WaitEventsTimeout`/`PostEmptyEvent`/
`StopMainEventLoop`) is pure msgSend. Exported API unchanged; the root package needed zero edits.

### What changed (session 4)

- **[internal/mac/app_darwin.go](internal/mac/app_darwin.go)** (new) — `InstallMacAppDelegate` registers
  `macAppDelegate` (6 delegate methods calling the App*Callback vars directly, incl. the uint64-returning
  `applicationShouldTerminate:` → NSTerminateCancel and `application:openURLs:` with NSURL→path conversion);
  class registration is `sync.Once`-guarded so install/uninstall/reinstall cycles work (the class can only be
  registered once per process; instances are per-install). `FinishLaunching` runs `[NSApp run]` until the
  didFinishLaunching callback stops it, then sets activation policy Regular — same flow as the ObjC bridge.
  Hide/unhide/activate and the four menu setters are one-line msgSends; `Menu` (still cgo-typed) converts to
  `objc.ID` legally because cgo maps CFTypeRef-family typedefs to uintptr.
- **[internal/mac/event_darwin.go](internal/mac/event_darwin.go)** (new) — `EventModifierFlags` + constants moved
  here from all_darwin.go; `DoubleClickInterval`, `CurrentModifierFlags`, `PostEmptyEvent` (struct NSPoint arg is
  all-float/SSE-class, so the amd64 straddle constraint does not apply), the pump functions, and
  `StopMainEventLoop`. `NSDefaultRunLoopMode` is resolved once via `NSStringConstant`. Autorelease pools bracket
  exactly the regions the old `@autoreleasepool` blocks did.
- **all_darwin.go / macos.h** — App and Event sections removed (~140 + ~40 lines, all 6 `//export goApp*` funcs);
  `app_darwin.m` and `event_darwin.m` deleted.
- **[internal/mac/objc_darwin.go](internal/mac/objc_darwin.go)** — **real bug found and fixed by the amd64 test
  run**: `WithPool` pushed/popped an autorelease pool without pinning the OS thread, but pools are per-thread and
  a Go goroutine can migrate between push and pop (the cgo bridge never had this hazard — its `@autoreleasepool`
  blocks lived inside single C calls). `PostEmptyEvent` is called from arbitrary goroutines in production, so this
  was a real crash waiting to happen (reproduced as a SIGSEGV in `objc_autoreleasePoolPop` under Rosetta,
  `-test.count=3`). `WithPool` now does `runtime.LockOSThread`/`UnlockOSThread` around the pool; `PoolPush`/
  `PoolPop` docs state the same-thread requirement for direct users.
- **Test infrastructure**: [testmain_darwin_test.go](internal/mac/testmain_darwin_test.go) (new) pins the main
  goroutine to the main OS thread (`init` + `runtime.LockOSThread`) and turns `TestMain` into a main-thread
  dispatcher: tests run on a secondary goroutine and submit main-thread work via `runOnMain`, while the dispatcher
  also pumps the main run loop between work items. The theme observer is now installed from the main thread in
  `TestMain` (the session-3 dedicated pump thread is gone — see discovery 1); later window/view test sessions can
  reuse `runOnMain` as-is.
- **New tests**: [app_darwin_test.go](internal/mac/app_darwin_test.go) covers the full delegate lifecycle
  (install → `FinishLaunching` through a real `[NSApp run]` with watchdog → willFinish/didFinish delivered →
  `terminate:` round-trip through the uint64-returning delegate method → `application:openURLs:` driven via
  objc_msgSend → keyUp-monitor block logic via `objc.InvokeBlock` plus a synthesized Cmd+keyUp NSEvent routed
  through the real queue → uninstall/reinstall) and the four menu setters against their AppKit getters (which also
  proves cgo Menu handles interoperate with the purego side). [event_darwin_test.go](internal/mac/event_darwin_test.go)
  proves the production wake-up contract (PostEmptyEvent → blocked WaitEvents returns) and both WaitEventsTimeout
  paths (expiry, and mid-wait wake by a posted event). Launch-once-per-process effects are handled: on `-count=N`
  reruns the launch notifications can't fire again and post-launch AppKit substitutes its own services/windows
  menus, so those asserts are gated on `isFinishedLaunching`.

### Discoveries

1. **The session-3 "dedicated pump thread" for NSDistributedNotificationCenter only worked by accident.** With the
   main goroutine unlocked (old TestMain), the pump goroutine's `LockOSThread` evidently landed on the real main
   thread (m0) while the main goroutine was parked. Once session 4's `init` locked the main goroutine to m0, the
   pump got a genuinely different thread and delivery silently stopped (theme test failed even in isolation, while
   HEAD passed). Delivery requires the default center to be created from and pumped on the **process main thread**
   — which is exactly the production configuration, so production was never affected. The test infrastructure now
   does it that way explicitly.
2. **Autorelease pools + goroutine migration** (the `WithPool` bug above): any purego port that brackets work with
   `objc_autoreleasePoolPush/Pop` from Go must pin the OS thread for the pool's lifetime. Worth remembering for
   every remaining Phase 2 file.
3. `-count=N` in one process is a genuinely different regime for AppKit lifecycle code (launch-once, AppKit-managed
   menus after launch). Keep running it — it caught both of the above along with the Rosetta run.

### Verification performed (session 4)

- `./build.sh --test` green (twice — after the port and after the WithPool fix); `golangci-lint run ./...` 0
  issues; `golangci-lint fmt` applied (no changes beyond the edits themselves).
- `go test ./internal/mac/` 10/10 fresh processes + 5 more after the fixes; `-count=5` single process; `-race`.
- darwin/amd64 under Rosetta 2 (`CGO_ENABLED=1 GOARCH=amd64 go test -c`): 5/5 fresh runs + 3/3 `-test.count=3`
  runs (this configuration exposed the WithPool bug before the fix).
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and windows/amd64 pass.
- `cmd/example` smoke-run: alive after 8s with empty stderr/stdout, killed with SIGKILL per the session-3 note.
  Startup exercises the entire ported path in production shape: InstallMacAppDelegate → FinishLaunching
  (`[NSApp run]` → delegate → stop) → WaitEvents/PollEvents pumping with real windows rendering.
- Not covered (need a human or a real session): applicationDidHide/hideOtherApplications side effects (would hide
  the user's other apps), ActivateIgnoringOtherApps (would steal focus), Cmd+keyUp forwarding to a real key window
  (needs the view/window port; the block logic and AppKit→block dispatch are covered).

## Session 3 — 2026-07-10: Phase 2 — sound, theme, screen (+display), image, and cursor ported to purego; their .m files deleted

The "five trivial files" bullet of Phase 2 is done. Each area moved from the cgo bridge to a pure-Go `_darwin.go`
file with the exported API unchanged; the corresponding `.m` files, `macos.h` declarations, and `all_darwin.go`
sections are gone. The CGDirectDisplay functions (the old "Display" section of `all_darwin.go`, plain C calls into
CoreGraphics) were ported alongside screen since `ScreenForDisplayID` needs `CGDisplayUnitNumber`.

### What changed

- **[internal/mac/sound_darwin.go](internal/mac/sound_darwin.go)** — `Beep` via `NSBeep` (dlsym from AppKit).
- **[internal/mac/theme_darwin.go](internal/mac/theme_darwin.go)** — `ThemeDelegate` is now a Go-registered
  Objective-C class (`objc.RegisterClass`, the first in shipped code); observer install is `sync.Once`-guarded.
  `IsDarkModeEnabled` reads AppleInterfaceStyle through NSUserDefaults instead of `CFPreferencesCopyAppValue` —
  same domains searched, same result, and it fixes a small CFString leak the old code had.
- **[internal/mac/display_darwin.go](internal/mac/display_darwin.go)** — the six CG functions via
  `purego.RegisterLibFunc` against the CoreGraphics framework; `DisplayID` is now `= uint32` (was
  `= C.CGDirectDisplayID`, same underlying type). `CGDisplayBounds`/`CGDisplayScreenSize` return CGRect/CGSize by
  value — first use of struct returns through RegisterLibFunc, verified on both arches.
- **[internal/mac/screen_darwin.go](internal/mac/screen_darwin.go)** — `Screen` is now `objc.ID`-based (was
  `C.NSScreenRef`; both uintptr-kinded, so root-package `== 0` checks still compile). Frame/VisibleFrame use
  `objc.Send[NSRect]` struct returns; `ConvertRectToBacking` passes NSRect by value (all-float structs, so the
  amd64 straddle constraint from Session 2 does not apply).
- **[internal/mac/image_darwin.go](internal/mac/image_darwin.go)** — `newNSImage` (unexported, +1 retain) builds
  the NSBitmapImageRep/NSImage pair; the colorSpaceName uses the real `NSCalibratedRGBColorSpace` constant via
  dlsym rather than a lookalike string. The still-cgo drag path in `all_darwin.go` now calls it and converts the
  handle with `C.NSImageRef(imgRef)` (legal: cgo maps CFTypeRef-family typedefs to uintptr).
- **[internal/mac/cursor_darwin.go](internal/mac/cursor_darwin.go)** — `Cursor` is `objc.ID`-based. `NewCursor`
  reproduces the old bridge's +2 retain count (alloc/init + retain) and the int truncation of the hot spot; the
  shared cursors keep their +1 retain so `Release` balances.
- **[internal/mac/objc_darwin.go](internal/mac/objc_darwin.go)** — helper additions: `LoadFramework` (generalizes
  `LoadAppKit`, returns the dlopen handle for symbol lookup), `NSStringConstant` (dlsym + deref of exported
  NSString* constants), and geom converters (`PointFromNSPoint`, `NSPointFromPoint`, `SizeFromNSSize`,
  `RectFromNSRect`, `NSRectFromRect`).
- Per-area `_darwin_test.go` files cover every ported function that can be exercised headlessly, including a
  delivery test for the full ThemeDelegate path (class registration → distributed notification → Go callback).

### Discoveries (all verified empirically)

1. **NSDistributedNotificationCenter delivers on the run loop of the thread that FIRST created the default
   center — the `addObserver:` thread is irrelevant.** Proven with a standalone control/poisoned repro: touching
   `defaultCenter` from a throwaway thread before registering breaks delivery permanently (0/3 vs 3/3). This was
   the cause of a ~40% full-suite test flake (another test's transient thread would win the first-touch race).
   Fixed in tests via `TestMain` starting a dedicated, permanently locked pump thread before any test runs, and
   documented on `InstallSystemThemeChangedCallback`: production is safe because unison installs the observer from
   the main thread at startup before anything else touches the center — same ordering the cgo bridge relied on.
2. **`NSCursor arrowCursor`/`IBeamCursor` return nil until NSApplication is initialized** (the other four shared
   cursors work regardless). Identical under the old ObjC bridge — unison always creates the shared application
   first, so only the tests needed a `sharedApplication` call.
3. **Pre-existing SIGTERM shutdown crash, NOT a regression**: `xos.Exit` runs `quitting()` on the signal-handler
   goroutine, so `Window.OrderOut` → AppKit `orderOut:` executes off the main thread while the main thread sits in
   `waitEvents`, and AppKit SIGTRAPs. Reproduced 3/3 with identical stacks on the *unmodified* tree (via
   `git stash`). Session 2's "exits cleanly on SIGTERM" observation does not hold today; the app runs fine and the
   crash is only in process teardown. Worth fixing separately (marshal shutdown to the main thread) — out of scope
   for the port.

### Verification performed (session 3)

- `./build.sh --test` green; `golangci-lint run ./...` 0 issues; `golangci-lint fmt` applied to internal/mac.
- `go test -race ./internal/mac/` 10/10 in fresh processes (was flaky ~40% before the TestMain fix), plus
  `-count=5` in a single process (validates the observer surviving reruns).
- darwin/amd64: `CGO_ENABLED=1 GOARCH=amd64 go test -c` binary run under Rosetta 2 — all tests pass, covering the
  amd64 hidden-pointer struct returns from C (CGDisplayBounds) and objc_msgSend_stret paths.
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and windows/amd64 still pass.
- `cmd/example` smoke-run: launches, renders, alive after 6s, empty stderr — startup exercises the ported
  display/screen (window placement), cursor, and theme (apiLateInit) paths in-process.

## Session 2 — 2026-07-10: Phase 0 complete (purego/objc feasibility spike proven on both arches) + Phase 2 foundation helpers

The spike turned out to be runnable without a human at the keyboard: windows open on this machine from a normal
session, drawRect: fires via AppKit, and mouse/key events can be synthesized as real NSEvents posted to the app's own
queue and dispatched through `sendEvent:`. All Phase 0 exit criteria were met on darwin/arm64 (native) and
darwin/amd64 (Rosetta 2).

### Headline finding: the port requires purego v0.11.0-alpha.6, not v0.10.1

- **purego v0.10.1 cannot do the port at all.** Its `NewCallback`/`NewIMP` panic on struct arguments and only allow
  integer/pointer/bool returns, and the arm64 callback assembly does not preserve x8 (the indirect struct-return
  register). `drawRect:` (NSRect arg) and `markedRange` (NSRange return) are unimplementable.
- **v0.11.0-alpha.6 (tagged 2026-07-02) adds full struct support for callbacks** on amd64/arm64: SysV eightbyte
  classification for args (including reading >16-byte structs from the stack and the "whole struct spills to stack
  when registers run out" rule), arm64 HFA args/returns in d0–d3 (NSRect return is an HFA — no x8 needed),
  16-byte returns in x0/x1 / rax/rdx, x8 indirect returns, and the amd64 hidden-pointer return (the callee skips the
  hidden first register arg). `go.mod` was bumped accordingly; `go mod tidy` run.
- The plan's "Ebitengine is fully cgo-free on macOS" prior-art claim is **wrong**: ebiten v2.9.9 still ships `.m`
  files in `internal/glfw` and uses cgo `//export` in `exp/textinput` on darwin. Feasibility no longer rests on that
  claim — the spike proved everything directly.

### Spike results (throwaway program, per plan not committed; all checks on BOTH arches)

PASS on darwin/arm64 and darwin/amd64:

- `RegisterClass` of an NSView subclass declaring the NSTextInputClient protocol; `conformsToProtocol:` true.
- IMPs written in Go, invoked through real objc_msgSend dispatch: `markedRange` (16-byte NSRange return),
  `firstRectForCharacterRange:actualRange:` (32-byte NSRect return + NSRange arg + out-pointer writeback),
  `characterIndexForPoint:` (NSPoint HFA arg), `insertText:replacementRange:` (id + NSRange args),
  `initWithFrame:` calling `objc.SendSuper` with a struct arg.
- AppKit-initiated calls: `drawRect:` received a correct NSRect after `orderFront` + `setNeedsDisplay:`;
  synthesized `NSEvent` mouse/key events posted via `postEvent:atStart:` were dispatched to `mouseDown:`/`keyDown:`
  through `nextEventMatchingMask:...` + `sendEvent:` loop turns wrapped in explicit autorelease pools.
- **IME text path**: `keyDown:` → `interpretKeyEvents:` → AppKit called our `insertText:replacementRange:` with the
  typed character — and it worked even with the app *inactive* under the accessory activation policy. (Real CJK
  input-source testing still needs a human; see Phase 2 verification bullet.)
- msgSend calling side: NSRange/NSRect/NSPoint pass+return round-trips via NSValue and `rangeOfString:`;
  `purego.NewCallback` plain-C callback (qsort comparator); `objc_autoreleasePoolPush`/`Pop`.

Two gotchas discovered, both worked around and documented:

1. **purego alpha.6 call-side bug (amd64)**: a 16-byte struct argument that no longer fits entirely in the remaining
   integer registers is *split* across r9 and the stack (`tryPlaceRegister` never checks remaining-register count;
   `addInt` silently overflows per-eightbyte). SysV requires the whole struct to go to memory. Hit by
   `setMarkedText:selectedRange:replacementRange:` (id + 2×NSRange) when *purego is the caller*. Proven callee-side
   correct with an NSInvocation (Foundation's compiled marshaling) as the caller — so AppKit→Go, the direction IME
   actually uses, is fine. Constraint documented in the header comment of
   [internal/mac/objc_darwin.go](internal/mac/objc_darwin.go): don't `Send` struct args after ≥4 preceding
   integer-register args on amd64 (audit each Phase 2 call; almost no bridge calls have that shape). Worth reporting
   upstream to ebitengine/purego.
2. **Click-through**: clicks on an inactive window are swallowed (used only to activate) unless the view implements
   `acceptsFirstMouse:` returning YES. Also, AppKit recomputes `locationInWindow` for synthesized events at delivery
   time, so exact coordinate round-trips can be off by a couple of points — irrelevant for real user events.

### Committed to the working tree

- **go.mod/go.sum**: purego v0.10.1 → v0.11.0-alpha.6 (hard requirement, see above). Verified the bump against
  everything that already uses purego: full `./build.sh --test`, linux amd64+arm64 and windows amd64
  `CGO_ENABLED=0 go build ./...`, and a 6-second live smoke-run of `cmd/example` (canvas's purego-based GL loading
  works under alpha.6).
- **[internal/mac/objc_darwin.go](internal/mac/objc_darwin.go)** — the Phase 0 "shared helper set" decision,
  realized as the first Phase 2 file (foundation helpers, coexists with the cgo bridge until the port lands):
  reuse `purego/objc` directly for classes/selectors/msgSend, plus a thin helper layer: NSPoint/NSSize/NSRect/NSRange
  ABI structs (Go type names chosen so purego's derived @encode strings resemble the real ones), `Sel`/`Cls` caches
  (`Cls` panics on unknown class names and lazily dlopens AppKit), `LoadAppKit`, `Retain`/`Release`/`Autorelease`,
  `PoolPush`/`PoolPop`/`WithPool`, NSString⇄Go string, NSArray⇄[]objc.ID, NSNumber int64/float64, NSURL⇄file path.
- **[internal/mac/objc_darwin_test.go](internal/mac/objc_darwin_test.go)** — permanent unit tests for the helper
  layer (string/array/number/URL round-trips, retain/release across nested pools) plus a struct-msgSend guard test
  (NSRect/NSPoint/NSRange round-trips) that will catch ABI regressions in future purego bumps on every
  `go test ./...` run on macOS. Found during testing: macOS returns NFD-normalized path strings from `NSURL.path`
  (pre-existing Cocoa behavior, same under the old cgo bridge) — documented on `FilePathFromNSURL`.

### Verification performed (session 2)

- `./build.sh --test` green (includes the new internal/mac tests); `go test -race ./internal/mac/` green.
- `golangci-lint run ./...` — 0 issues (note: `.golangci.yml` excludes `internal/mac/` from linting).
- `GOOS=linux GOARCH={amd64,arm64} CGO_ENABLED=0 go build ./...` and `GOOS=windows GOARCH=amd64 CGO_ENABLED=0
  go build ./...` pass with alpha.6 (Phase 1 GLX port unaffected).
- Spike binary: SPIKE OK on darwin/arm64 (native) and darwin/amd64 (Rosetta 2, `GOARCH=amd64 CGO_ENABLED=0`).
- `cmd/example` smoke-run: launches, renders, stays alive, exits cleanly on SIGTERM with purego v0.11.0-alpha.6.

## Session 1 — 2026-07-10: Phase 1 complete (Linux GLX → purego) + purego promoted to direct dependency

Phase 1 was done first (the plan marks it "small, do first") because it is fully verifiable from this machine via
cross-compilation, whereas the Phase 0 spike needs an interactive macOS GUI run.

### Done

- **Promoted `github.com/ebitengine/purego` v0.10.1 from indirect to direct** in `go.mod` (Phase 0, first bullet) via
  `go mod tidy` after adding the import. Checked for newer releases per the plan's risk note: v0.10.1 is still the
  latest stable; only v0.11.0 alphas exist, so we stay on v0.10.1.
- **Rewrote [internal/x11/glx_linux.go](internal/x11/glx_linux.go) with no `import "C"`** (was ~216 lines of cgo with
  `#cgo linux pkg-config: x11 gl`):
  - libX11 (`libX11.so.6` → `libX11.so`) and libGL (`libGL.so.1` → `libGL.so`) are dlopen'd on first `NewGLX` call
    (`sync.Once`), with clear errors naming the distro package to install when a library is missing. Missing symbols
    surface as errors, not panics (manual `Dlsym` + `RegisterFunc` instead of `purego.RegisterLibFunc`).
  - All 13 entry points from the plan's inventory are registered with typed Go func vars.
    `glXCreateContextAttribsARB` is resolved through `glXGetProcAddressARB` (not dlsym) and called with the same
    3.2-core attribs array as the old C `createContext` helper; if resolution fails, `CreateContext` returns nil,
    matching the old behavior.
  - `XVisualInfo` is a Go struct (`xVisualInfo`) matching the C layout on both 32-bit and 64-bit Linux (uintptr for
    pointers/`unsigned long`, int32 for `int`; unread members are blank `_` fields so the `unused` linter stays quiet).
  - Exported API is unchanged: `Display`, `FBConfig`, `GLXContext`, `GLXWindow`, `GLX` and all its methods,
    `Conn.NewGLX`. **Deviation from the plan's letter**: the plan said "uintptr-based named types" for all four, but
    `Display`/`FBConfig`/`GLXContext` are defined over `unsafe.Pointer` instead (only `GLXWindow`, an XID, is
    uintptr-based). Reason: [glcontext_linux.go](glcontext_linux.go) compares/assigns `x11.GLXContext` against `nil`,
    so a uintptr-based type would have forced caller changes the plan explicitly wanted to avoid; purego handles
    UnsafePointer-kind args/returns fine. glcontext_linux.go needed zero changes.
  - Two-connection design, transparent-visual selection logic, and the NVIDIA BadMatch comment preserved verbatim.
  - **Fixed a pre-existing double-free**: the old code both `defer C.XFree(configs)`'d and explicitly freed `configs`
    on the "no suitable framebuffer configuration" error path. The port keeps only the defer.

### Verification performed

- `GOOS=linux CGO_ENABLED=0 go build ./...` passes on amd64 and arm64 (was failing before with `undefined: x11.GLX`
  etc., since the cgo file was excluded); `internal/x11` also compiles for linux/386 and linux/arm.
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/x11/` compiles the test binary.
- `GOOS=linux GOARCH=amd64 golangci-lint run ./internal/x11/` — 0 issues (also ran `golangci-lint fmt` for gofumpt).
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` passes (sanity check).
- Native macOS: `./build.sh --test` green; `golangci-lint run ./...` — 0 issues.

### Live Linux verification — done (2026-07-10)

- Rich ran the checked-in code on a live Linux machine after Session 2 and reports it working, closing the last
  Phase 1 bullet. (The finer-grained scenarios — XWayland vs X11, transparent window, NVIDIA driver,
  `clipboard_live_test.go` — were not individually itemized; re-check them if a Linux display regression appears.)

## What remains (in plan order)

1. **Phase 2 (rest)**: port `internal/mac` to purego/objc, file-by-file in the order listed in plan.md. Foundation
   helpers, the five trivial areas (sound, theme, screen+display, image, cursor), and app + event loop are done;
   next up is **window + window delegate** (`window_darwin.m`, `window_delegate_darwin.m`), then view/IME → GL
   context → menus → pasteboard/drag → panels → delete the remaining `.m` files. Every step must leave
   `./build.sh` green. Final manual verification must include a real CJK input source (IME).
2. **Phase 3**: build.sh/CI enforcement of `CGO_ENABLED=0`, `import "C"` guard, README setup-section rewrite,
   `.claude/CLAUDE.md` architecture-note update, upack audit.

## Notes for future sessions

- **purego is now v0.11.0-alpha.6** (required — see session 2). When a stable v0.11.0 ships, bump to it and check
  whether the amd64 call-side struct-straddle bug (session 2) was fixed; the guard test in
  `internal/mac/objc_darwin_test.go` plus a re-run of the spike checks would validate the bump.
- purego `RegisterFunc` supports args/returns of Kind Uintptr, Ptr, UnsafePointer, ints, string, bool, floats, and
  Func — named types are matched by Kind, so the pattern used in glx_linux.go (typed handle types over
  unsafe.Pointer/uintptr) carries over directly to the Phase 2 mac port. As of v0.11.0-alpha.6, struct args/returns
  also work in both directions on amd64/arm64 (subject to the caller-side straddle constraint documented in
  internal/mac/objc_darwin.go).
- Phase 2 method-registration recipe proven by the spike: `objc.RegisterClass` with `MethodDef` Go funcs taking
  `(objc.ID, objc.SEL, ...)`; declare protocol conformance via `objc.GetProtocol("NSTextInputClient")`; name the Go
  ABI structs `NSPoint`/`NSSize`/`NSRect`/`NSRange` so derived @encode strings look right; call super with
  `objc.SendSuper` (struct args work). The content view must implement `acceptsFirstMouse:` → the old ObjC bridge got
  click-through behavior implicitly; check what `view_darwin.m` does today before changing behavior.
- Windows and the event loop work headlessly-ish from an agent session: accessory activation policy + synthesized
  `NSEvent`s posted with `postEvent:atStart:` + a `nextEventMatchingMask:`/`sendEvent:` pump wrapped in explicit
  autorelease pools. AppKit recomputes `locationInWindow` at delivery, so assert approximate coordinates. The
  darwin/amd64 side runs fine under Rosetta 2 (`GOARCH=amd64 CGO_ENABLED=0 go build`, run the binary directly) —
  re-verify both arches after each risky Phase 2 step.
- macOS returns NFD-normalized strings from `NSURL.path` — do not compare Cocoa-sourced paths to Go literals
  byte-for-byte in tests; use ASCII/CJK test data (see `TestNSURLFilePathRoundTrip`).
- `.golangci.yml` excludes `internal/mac/` from linting entirely, so lint passes say nothing about that directory —
  review new code there manually (gofumpt formatting still applies via `golangci-lint fmt` if desired).
- golangci-lint can cross-lint: `GOOS=linux golangci-lint run ./internal/x11/` works from macOS and caught real issues
  (gofumpt formatting, govet fieldalignment) that the native run never sees. Use `GOOS=<os> golangci-lint run` on any
  platform-suffixed files touched in future sessions.
- NSDistributedNotificationCenter delivery requires the default center to be created from and pumped on the
  **process main thread** (Session 3 discovery 1, sharpened by Session 4 discovery 1 — a non-main "first-touch"
  thread is not enough). Any future code observing distributed notifications must be installed from the main
  thread before other threads can touch the center. In internal/mac tests, `TestMain`
  ([testmain_darwin_test.go](internal/mac/testmain_darwin_test.go)) owns the locked main thread: it installs the
  theme observer first, pumps the main run loop, and services `runOnMain` closures — use `runOnMain` for anything
  AppKit requires on the main thread ([NSApp run], event pumping, and the upcoming window/view work).
- `WithPool` locks the goroutine to its OS thread for the pool's lifetime (Session 4 discovery 2). Direct
  `PoolPush`/`PoolPop` pairs are only safe on an already-locked thread — prefer `WithPool`.
- While cgo and purego coexist in internal/mac, a Go-registered Objective-C class name must not collide with one
  still compiled from a `.m` file (objc_allocateClassPair fails). Delete the `.m` file in the same step that
  registers the class from Go, as done for ThemeDelegate.
- The SIGTERM shutdown crash (Session 3 discovery 3) is pre-existing and unrelated to the port: `xos.Exit` calls
  window teardown from the signal-handler goroutine, off the main thread. Smoke-testing the example app should
  assert liveness and use SIGKILL, not judge success by SIGTERM exit status. Consider a separate fix that marshals
  quitting() to the main event loop.
