# Progress

Running log of work sessions against [plan.md](plan.md) (removing all cgo usage from unison). Newest session first.

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

### Not yet verified (needs a Linux machine with a display)

- Running `cmd/example` on real X11 and XWayland with the new dlopen-based GLX path (including a transparent window
  and an NVIDIA driver if available), and `clipboard_live_test.go` in `internal/x11`. This is the one unchecked
  Phase 1 bullet in plan.md.

## What remains (in plan order)

1. **Phase 1 leftover**: live verification on a Linux machine with a display — `cmd/example` on X11 and XWayland with
   the dlopen-based GLX path (including a transparent window and an NVIDIA driver if available), plus
   `clipboard_live_test.go` in `internal/x11`.
2. **Phase 2 (rest)**: port `internal/mac` to purego/objc, file-by-file in the order listed in plan.md. Foundation
   helpers are done ([internal/mac/objc_darwin.go](internal/mac/objc_darwin.go)); next up are the five trivial files
   (`sound`, `theme`, `screen`, `image`, `cursor`), then app/event loop → window → view/IME → GL context → menus →
   pasteboard/drag → panels → delete `.m` files. Estimated four to six sessions; every step must leave `./build.sh`
   green. Final manual verification must include a real CJK input source (IME).
3. **Phase 3**: build.sh/CI enforcement of `CGO_ENABLED=0`, `import "C"` guard, README setup-section rewrite,
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
