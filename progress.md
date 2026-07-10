# Progress

Running log of work sessions against [plan.md](plan.md) (removing all cgo usage from unison). Newest session first.

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

1. **Phase 0 (rest)**: the purego/objc feasibility spike on macOS — register an `NSView` subclass with `drawRect:`
   (struct arg), `firstRectForCharacterRange:actualRange:` (large struct return), `markedRange` (16-byte struct
   return); verify `objc_msgSend` struct pass/return, `purego.NewCallback`, autorelease pool push/pop; decide the
   shared helper set for `internal/mac/objc_darwin.go`. This needs an interactive GUI session (windows must open and
   receive events), so it is best done by the user or a session that can drive the screen. The spike is throwaway
   code — do not commit it.
2. **Phase 1 leftover**: live verification on Linux (see above).
3. **Phase 2**: port `internal/mac` to purego/objc, file-by-file in the order listed in plan.md (foundation helpers →
   trivial files → app/event loop → window → view/IME → GL context → menus → pasteboard/drag → panels → delete `.m`
   files). Estimated four to six sessions; every step must leave `./build.sh` green.
4. **Phase 3**: build.sh/CI enforcement of `CGO_ENABLED=0`, `import "C"` guard, README setup-section rewrite,
   `.claude/CLAUDE.md` architecture-note update, upack audit.

## Notes for future sessions

- purego v0.10.1 `RegisterFunc` supports args/returns of Kind Uintptr, Ptr, UnsafePointer, ints, string, bool, floats,
  and Func — named types are matched by Kind, so the pattern used in glx_linux.go (typed handle types over
  unsafe.Pointer/uintptr) carries over directly to the Phase 2 mac port.
- golangci-lint can cross-lint: `GOOS=linux golangci-lint run ./internal/x11/` works from macOS and caught real issues
  (gofumpt formatting, govet fieldalignment) that the native run never sees. Use `GOOS=<os> golangci-lint run` on any
  platform-suffixed files touched in future sessions.
