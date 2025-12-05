package plaf2

import (
	"log/slog"
	"slices"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/mac"
)

type platformWindow struct {
	wnd            mac.Window
	view           mac.View
	nsCursorHidden bool
}

func initWindowCallbacks() {
	mac.WindowShouldCloseCallback = func(w mac.Window) {
		if i := slices.IndexFunc(windowList, func(wnd *Window) bool {
			return wnd.plWnd.wnd == w
		}); i != -1 {
			// TODO: Initiate close sequence
			// _plafInputWindowCloseRequest(window);
			//windowList[i].CloseRequest()
		} else {
			slog.Warn("received window should close callback for unknown window", "window", w)
		}
	}
}

func newWindow(cfg *WindowConfig) *Window {
	styleMask := mac.WindowStyleMaskMiniaturizable
	if cfg.Undecorated {
		styleMask |= mac.WindowStyleMaskBorderless
	} else {
		styleMask |= mac.WindowStyleMaskTitled | mac.WindowStyleMaskClosable
		if !cfg.NotResizable {
			styleMask |= mac.WindowStyleMaskResizable
		}
	}
	nw := mac.NewWindow(geom.NewRect(0, 0, 1, 1), styleMask, true, true)
	if nw == 0 {
		return nil
	}
	if cfg.NotResizable {
		nw.SetCollectionBehavior(mac.WindowCollectionBehaviorFullScreenNone)
	} else {
		nw.SetCollectionBehavior(mac.WindowCollectionBehaviorFullScreenPrimary | mac.WindowCollectionBehaviorManaged)
	}
	if cfg.Floating {
		nw.SetLevel(mac.WindowLevelFloating)
	}
	// TODO
	// window->nsView = [[MacContentView alloc] initWithPlafWindow:window];
	if cfg.Transparent {
		nw.SetTransparent()
	}
	// TODO
	// [window->nsWindow setContentView:window->nsView];
	// [window->nsWindow makeFirstResponder:window->nsView];
	// [window->nsWindow setTitle:@(window->title)];
	//delegate :=
	mac.NewWindowDelegate(nw)
	// TODO
	// [window->nsWindow setDelegate:(id<NSWindowDelegate>)window->nsDelegate];
	// [window->nsWindow setAcceptsMouseMovedEvents:YES];
	// [window->nsWindow setRestorable:NO];
	// if ([window->nsWindow respondsToSelector:@selector(setTabbingMode:)]) {
	// 	[window->nsWindow setTabbingMode:NSWindowTabbingModeDisallowed];
	// }
	// plafGetWindowSize(window, &window->width, &window->height);
	// plafGetFramebufferSize(window, &window->nsFrameBufferWidth, &window->nsFrameBufferHeight);
	return &Window{
		plWnd: platformWindow{
			wnd: nw,
		},
	}
}

func (w *Window) adjustToCursorChange() { // formerly plafAdjustToCursorChange
	if w.cursorInContentArea() {
		w.updateCursorImage()
	}
}

func (w *Window) updateCursor() { // formerly _plafUpdateCursor
	if w.Focused() {
		if w.cursorInContentArea() {
			w.updateCursorImage()
		}
	}
}

func (w *Window) updateCursorImage() { // formerly _plafUpdateCursorImage
	if w.cursorHidden {
		if !w.plWnd.nsCursorHidden {
			mac.HideCursor()
			w.plWnd.nsCursorHidden = true
		}
	} else {
		if w.plWnd.nsCursorHidden {
			mac.ShowCursor()
			w.plWnd.nsCursorHidden = false
		}
		if w.cursor != nil {
			w.cursor.plCursor.Set()
		} else {
			mac.ArrowCursor().Set()
		}
	}
}

func (w *Window) cursorInContentArea() bool { // formerly _plafCursorInContentArea
	view := w.plWnd.wnd.ContentView()
	return view.MouseInRect(w.plWnd.wnd.MouseLocationOutsideOfEventStream(), view.Frame())
}

func (w *Window) CursorPosition() geom.Point { // formerly plafGetCursorPos
	loc := w.plWnd.wnd.MouseLocationOutsideOfEventStream()
	frame := w.plWnd.wnd.ContentView().Frame()
	return geom.NewPoint(loc.X, frame.Height-loc.Y)
}

func (w *Window) Focused() bool { // formerly plafIsWindowFocused
	return w.plWnd.wnd.Focused()
}

func (w *Window) destroy() { // formerly _plafDestroyWindow
	w.plWnd.wnd.OrderOut()
	w.plGctx.destroy()
	delegate := w.plWnd.wnd.Delegate()
	w.plWnd.wnd.SetDelegate(0)
	delegate.Release()
	w.plWnd.wnd.ContentView().Release()
	w.plWnd.wnd.Close()
	w.plWnd.wnd = 0
	/* TODO
	plafPollEvents();
	*/
}
