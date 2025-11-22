#if defined(__APPLE__)

#include "platform.h"

#import <QuartzCore/CAMetalLayer.h>

// Hides the cursor if not already hidden
static void hideCursor(plafWindow* window) {
	if (!_plaf.nsCursorHidden) {
		[NSCursor hide];
		_plaf.nsCursorHidden = true;
	}
}

// Shows the cursor if not already shown
static void showCursor(plafWindow* window) {
	if (_plaf.nsCursorHidden) {
		[NSCursor unhide];
		_plaf.nsCursorHidden = false;
	}
}

// Updates the cursor image according to its cursor mode.
void _plafUpdateCursorImage(plafWindow* window) {
	if (window->cursorHidden) {
		hideCursor(window);
	} else {
		showCursor(window);
		if (window->cursor) {
			[window->cursor->nsCursor set];
		} else {
			[[NSCursor arrowCursor] set];
		}
	}
}

// Make the specified window and its video mode active on its monitor
static void acquireMonitor(plafWindow* window) {
	_plafSetVideoMode(window->monitor, &window->videoMode);
	const CGRect bounds = CGDisplayBounds(window->monitor->nsDisplayID);
	const NSRect frame = NSMakeRect(bounds.origin.x, _plafTransformYCocoa(bounds.origin.y + bounds.size.height - 1),
		bounds.size.width, bounds.size.height);
	[window->nsWindow setFrame:frame display:YES];
	window->monitor->window = window;
}

// Remove the window and restore the original video mode
static void releaseMonitor(plafWindow* window) {
	if (window->monitor->window == window) {
		window->monitor->window = NULL;
		_plafRestoreVideoMode(window->monitor);
	}
}

// Translates macOS key modifiers into PLAF ones
//
static int translateFlags(NSUInteger flags)
{
	int mods = 0;

	if (flags & NSEventModifierFlagShift)
		mods |= KEYMOD_SHIFT;
	if (flags & NSEventModifierFlagControl)
		mods |= KEYMOD_CONTROL;
	if (flags & NSEventModifierFlagOption)
		mods |= KEYMOD_ALT;
	if (flags & NSEventModifierFlagCommand)
		mods |= KEYMOD_SUPER;
	if (flags & NSEventModifierFlagCapsLock)
		mods |= KEYMOD_CAPS_LOCK;

	return mods;
}

// Translates a macOS keycode to a PLAF keycode
//
static int translateKey(unsigned int key) {
	if (key >= MAX_KEY_CODES) {
		return KEY_UNKNOWN;
	}
	return _plaf.keyCodes[key];
}

// Translate a PLAF keycode to a Cocoa modifier flag
//
static NSUInteger translateKeyToModifierFlag(int key)
{
	switch (key)
	{
		case KEY_LEFT_SHIFT:
		case KEY_RIGHT_SHIFT:
			return NSEventModifierFlagShift;
		case KEY_LEFT_CONTROL:
		case KEY_RIGHT_CONTROL:
			return NSEventModifierFlagControl;
		case KEY_LEFT_ALT:
		case KEY_RIGHT_ALT:
			return NSEventModifierFlagOption;
		case KEY_LEFT_SUPER:
		case KEY_RIGHT_SUPER:
			return NSEventModifierFlagCommand;
		case KEY_CAPS_LOCK:
			return NSEventModifierFlagCapsLock;
	}

	return 0;
}

// Defines a constant for empty ranges in NSTextInputClient
//
static const NSRange kEmptyRange = { NSNotFound, 0 };


//------------------------------------------------------------------------
// Delegate for window related notifications
//------------------------------------------------------------------------

@interface MacWindowDelegate : NSObject {
	plafWindow* window;
}

- (instancetype)initWithPlafWindow:(plafWindow *)initWindow;

@end

@implementation MacWindowDelegate

- (instancetype)initWithPlafWindow:(plafWindow *)initWindow {
	self = [super init];
	if (self != nil) {
		window = initWindow;
	}
	return self;
}

- (BOOL)windowShouldClose:(id)sender {
	_plafInputWindowCloseRequest(window);
	return NO;
}

- (void)windowDidResize:(NSNotification *)notification {
	[window->context.nsglCtx update];
	const int maximized = [window->nsWindow isZoomed];
	if (window->maximized != maximized) {
		window->maximized = maximized;
		_plafInputWindowMaximize(window, maximized);
	}
	const NSRect contentRect = [window->nsView frame];
	const NSRect fbRect = [window->nsView convertRectToBacking:contentRect];
	if (fbRect.size.width != window->nsFrameBufferWidth || fbRect.size.height != window->nsFrameBufferHeight) {
		window->nsFrameBufferWidth  = fbRect.size.width;
		window->nsFrameBufferHeight = fbRect.size.height;
		_plafInputFramebufferSize(window, fbRect.size.width, fbRect.size.height);
	}
	if (contentRect.size.width != window->width || contentRect.size.height != window->height) {
		window->width  = contentRect.size.width;
		window->height = contentRect.size.height;
		_plafInputWindowSize(window, contentRect.size.width, contentRect.size.height);
	}
}

- (void)windowDidMove:(NSNotification *)notification {
	[window->context.nsglCtx update];
	int x, y;
	_plafGetWindowPos(window, &x, &y);
	_plafInputWindowPos(window, x, y);
}

- (void)windowDidMiniaturize:(NSNotification *)notification {
	if (window->monitor) {
		releaseMonitor(window);
	}
	_plafInputWindowMinimize(window, true);
}

- (void)windowDidDeminiaturize:(NSNotification *)notification {
	if (window->monitor) {
		acquireMonitor(window);
	}
	_plafInputWindowMinimize(window, false);
}

- (void)windowDidBecomeKey:(NSNotification *)notification {
	_plafInputWindowFocus(window, true);
	if (_plafCursorInContentArea(window)) {
		_plafUpdateCursorImage(window);
	}
}

- (void)windowDidResignKey:(NSNotification *)notification {
	_plafInputWindowFocus(window, false);
}

@end


//------------------------------------------------------------------------
// Content view class for the PLAF window
//------------------------------------------------------------------------

@interface MacContentView : NSView <NSTextInputClient> {
	plafWindow*                window;
	NSTrackingArea*            trackingArea;
	NSMutableAttributedString* markedText;
}

- (instancetype)initWithPlafWindow:(plafWindow *)initWindow;

@end

@implementation MacContentView

- (instancetype)initWithPlafWindow:(plafWindow *)initWindow {
	self = [super init];
	if (self != nil) {
		window = initWindow;
		trackingArea = nil;
		markedText = [[NSMutableAttributedString alloc] init];
		[self updateTrackingAreas];
		[self registerForDraggedTypes:@[NSPasteboardTypeURL]];
	}
	return self;
}

- (void)dealloc {
	[trackingArea release];
	[markedText release];
	[super dealloc];
}

- (BOOL)isOpaque {
	return [window->nsWindow isOpaque];
}

- (BOOL)canBecomeKeyView {
	return YES;
}

- (BOOL)acceptsFirstResponder {
	return YES;
}

- (BOOL)wantsUpdateLayer {
	return YES;
}

- (void)updateLayer {
	[window->context.nsglCtx update];
	_plafInputWindowDamage(window);
}

- (void)cursorUpdate:(NSEvent *)event {
	_plafUpdateCursorImage(window);
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
	return YES;
}

- (void)mouseDown:(NSEvent *)event {
	_plafInputMouseClick(window, MOUSE_BUTTON_LEFT, INPUT_PRESS, translateFlags([event modifierFlags]));
}

- (void)mouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)mouseUp:(NSEvent *)event {
	_plafInputMouseClick(window, MOUSE_BUTTON_LEFT, INPUT_RELEASE, translateFlags([event modifierFlags]));
}

- (void)mouseMoved:(NSEvent *)event {
	const NSRect contentRect = [window->nsView frame];
	const NSPoint pos = [event locationInWindow];
	_plafInputCursorPos(window, pos.x, contentRect.size.height - pos.y);
}

- (void)rightMouseDown:(NSEvent *)event {
	_plafInputMouseClick(window, MOUSE_BUTTON_RIGHT, INPUT_PRESS, translateFlags([event modifierFlags]));
}

- (void)rightMouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)rightMouseUp:(NSEvent *)event {
	_plafInputMouseClick(window, MOUSE_BUTTON_RIGHT, INPUT_RELEASE, translateFlags([event modifierFlags]));
}

- (void)otherMouseDown:(NSEvent *)event {
	_plafInputMouseClick(window, (int) [event buttonNumber], INPUT_PRESS, translateFlags([event modifierFlags]));
}

- (void)otherMouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)otherMouseUp:(NSEvent *)event {
	_plafInputMouseClick(window, (int) [event buttonNumber], INPUT_RELEASE, translateFlags([event modifierFlags]));
}

- (void)mouseExited:(NSEvent *)event {
	if (window->cursorHidden) {
		showCursor(window);
	}
	_plafInputCursorEnter(window, false);
}

- (void)mouseEntered:(NSEvent *)event {
	if (window->cursorHidden) {
		hideCursor(window);
	}
	_plafInputCursorEnter(window, true);
}

- (void)viewDidChangeBackingProperties {
	const NSRect contentRect = [window->nsView frame];
	const NSRect fbRect = [window->nsView convertRectToBacking:contentRect];
	const float xscale = fbRect.size.width / contentRect.size.width;
	const float yscale = fbRect.size.height / contentRect.size.height;
	if (xscale != window->nsXScale || yscale != window->nsYScale) {
		window->nsXScale = xscale;
		window->nsYScale = yscale;
		_plafInputWindowContentScale(window, xscale, yscale);
	}
	if (fbRect.size.width != window->nsFrameBufferWidth || fbRect.size.height != window->nsFrameBufferHeight) {
		window->nsFrameBufferWidth  = fbRect.size.width;
		window->nsFrameBufferHeight = fbRect.size.height;
		_plafInputFramebufferSize(window, fbRect.size.width, fbRect.size.height);
	}
}

- (void)drawRect:(NSRect)rect {
	_plafInputWindowDamage(window);
}

- (void)updateTrackingAreas {
	if (trackingArea != nil) {
		[self removeTrackingArea:trackingArea];
		[trackingArea release];
	}
	trackingArea = [[NSTrackingArea alloc] initWithRect:[self bounds]
		options:NSTrackingMouseEnteredAndExited | NSTrackingActiveInKeyWindow | NSTrackingEnabledDuringMouseDrag |
			NSTrackingCursorUpdate | NSTrackingInVisibleRect | NSTrackingAssumeInside
		owner:self userInfo:nil];
	[self addTrackingArea:trackingArea];
	[super updateTrackingAreas];
}

- (void)keyDown:(NSEvent *)event {
	const int key = translateKey([event keyCode]);
	const int mods = translateFlags([event modifierFlags]);
	_plafInputKey(window, key, [event keyCode], INPUT_PRESS, mods);
	[self interpretKeyEvents:@[event]];
}

- (void)flagsChanged:(NSEvent *)event {
	int action;
	const unsigned int modifierFlags = [event modifierFlags] & NSEventModifierFlagDeviceIndependentFlagsMask;
	const int key = translateKey([event keyCode]);
	const int mods = translateFlags(modifierFlags);
	const NSUInteger keyFlag = translateKeyToModifierFlag(key);
	if (keyFlag & modifierFlags) {
		if (window->keys[key] == INPUT_PRESS) {
			action = INPUT_RELEASE;
		} else {
			action = INPUT_PRESS;
		}
	} else {
		action = INPUT_RELEASE;
	}
	_plafInputKey(window, key, [event keyCode], action, mods);
}

- (void)keyUp:(NSEvent *)event {
	const int key = translateKey([event keyCode]);
	const int mods = translateFlags([event modifierFlags]);
	_plafInputKey(window, key, [event keyCode], INPUT_RELEASE, mods);
}

- (void)scrollWheel:(NSEvent *)event {
	double deltaX = [event scrollingDeltaX];
	double deltaY = [event scrollingDeltaY];
	if ([event hasPreciseScrollingDeltas]) {
		deltaX *= 0.1;
		deltaY *= 0.1;
	}
	if (fabs(deltaX) > 0.0 || fabs(deltaY) > 0.0) {
		_plafInputScroll(window, deltaX, deltaY);
	}
}

- (NSDragOperation)draggingEntered:(id <NSDraggingInfo>)sender {
	return NSDragOperationGeneric;
}

- (BOOL)performDragOperation:(id <NSDraggingInfo>)sender {
	const NSRect contentRect = [window->nsView frame];
	const NSPoint pos = [sender draggingLocation];
	_plafInputCursorPos(window, pos.x, contentRect.size.height - pos.y);
	NSPasteboard* pasteboard = [sender draggingPasteboard];
	NSDictionary* options = @{NSPasteboardURLReadingFileURLsOnlyKey:@YES};
	NSArray* urls = [pasteboard readObjectsForClasses:@[[NSURL class]] options:options];
	const NSUInteger count = [urls count];
	if (count) {
		char** paths = _plaf_calloc(count, sizeof(char*));
		for (NSUInteger i = 0;  i < count;  i++) {
			paths[i] = _plaf_strdup([urls[i] fileSystemRepresentation]);
		}
		_plafInputDrop(window, (int) count, (const char**) paths);
		for (NSUInteger i = 0;  i < count;  i++) {
			_plaf_free(paths[i]);
		}
		_plaf_free(paths);
	}
	return YES;
}

- (BOOL)hasMarkedText {
	return [markedText length] > 0;
}

- (NSRange)markedRange {
	if ([markedText length] > 0) {
		return NSMakeRange(0, [markedText length] - 1);
	}
	return kEmptyRange;
}

- (NSRange)selectedRange {
	return kEmptyRange;
}

- (void)setMarkedText:(id)string selectedRange:(NSRange)selectedRange replacementRange:(NSRange)replacementRange {
	[markedText release];
	if ([string isKindOfClass:[NSAttributedString class]]) {
		markedText = [[NSMutableAttributedString alloc] initWithAttributedString:string];
	} else {
		markedText = [[NSMutableAttributedString alloc] initWithString:string];
	}
}

- (void)unmarkText {
	[[markedText mutableString] setString:@""];
}

- (NSArray*)validAttributesForMarkedText {
	return [NSArray array];
}

- (NSAttributedString*)attributedSubstringForProposedRange:(NSRange)range actualRange:(NSRangePointer)actualRange {
	return nil;
}

- (NSUInteger)characterIndexForPoint:(NSPoint)point {
	return 0;
}

- (NSRect)firstRectForCharacterRange:(NSRange)range actualRange:(NSRangePointer)actualRange {
	const NSRect frame = [window->nsView frame];
	return NSMakeRect(frame.origin.x, frame.origin.y, 0.0, 0.0);
}

- (void)insertText:(id)string replacementRange:(NSRange)replacementRange {
	NSString* characters;
	NSEvent* event = [NSApp currentEvent];
	const int mods = translateFlags([event modifierFlags]);
	const int plain = !(mods & KEYMOD_SUPER);
	if ([string isKindOfClass:[NSAttributedString class]]) {
		characters = [string string];
	} else {
		characters = (NSString*) string;
	}
	NSRange range = NSMakeRange(0, [characters length]);
	while (range.length) {
		uint32_t codepoint = 0;
		if ([characters getBytes:&codepoint maxLength:sizeof(codepoint) usedLength:NULL encoding:NSUTF32StringEncoding
			options:0 range:range remainingRange:&range]) {
			if (codepoint >= 0xf700 && codepoint <= 0xf7ff) {
				continue;
			}
			_plafInputChar(window, codepoint, mods, plain);
		}
	}
}

- (void)doCommandBySelector:(SEL)selector {
}

@end


//------------------------------------------------------------------------
// PLAF window class
//------------------------------------------------------------------------

@interface MacWindow : NSWindow {}
@end

@implementation MacWindow

- (BOOL)canBecomeKeyWindow {
	return YES;
}

- (BOOL)canBecomeMainWindow {
	return YES;
}

@end

// Create the Cocoa window
static plafError* createNativeWindow(plafWindow* window, const plafWindowConfig* wndconfig, const plafFrameBufferCfg* fbconfig) {
	window->nsDelegate = [[MacWindowDelegate alloc] initWithPlafWindow:window];
	if (!window->nsDelegate) {
		return _plafNewError("Cocoa: Failed to create window delegate");
	}

	NSRect contentRect;
	if (window->monitor) {
		plafVideoMode mode;
		int xpos;
		int ypos;
		_plafGetVideoMode(window->monitor, &mode);
		plafGetMonitorPos(window->monitor, &xpos, &ypos);
		contentRect = NSMakeRect(xpos, ypos, mode.width, mode.height);
	} else {
		if (wndconfig->xpos == ANY_POSITION || wndconfig->ypos == ANY_POSITION) {
			contentRect = NSMakeRect(0, 0, wndconfig->width, wndconfig->height);
		} else {
			const int xpos = wndconfig->xpos;
			const int ypos = _plafTransformYCocoa(wndconfig->ypos + wndconfig->height - 1);
			contentRect = NSMakeRect(xpos, ypos, wndconfig->width, wndconfig->height);
		}
	}

	NSUInteger styleMask = NSWindowStyleMaskMiniaturizable;
	if (window->monitor || !window->decorated) {
		styleMask |= NSWindowStyleMaskBorderless;
	} else {
		styleMask |= (NSWindowStyleMaskTitled | NSWindowStyleMaskClosable);
		if (window->resizable) {
			styleMask |= NSWindowStyleMaskResizable;
		}
	}

	window->nsWindow = [[MacWindow alloc] initWithContentRect:contentRect styleMask:styleMask
		backing:NSBackingStoreBuffered defer:NO];
	if (!window->nsWindow) {
		return _plafNewError("Cocoa: Failed to create window");
	}

	if (window->monitor) {
		[window->nsWindow setLevel:NSMainMenuWindowLevel + 1];
	} else {
		if (wndconfig->xpos == ANY_POSITION || wndconfig->ypos == ANY_POSITION) {
			[(NSWindow*) window->nsWindow center];
			_plaf.nsCascadePoint = NSPointToCGPoint([window->nsWindow cascadeTopLeftFromPoint: NSPointFromCGPoint(_plaf.nsCascadePoint)]);
		}

		if (wndconfig->resizable) {
			[window->nsWindow setCollectionBehavior:NSWindowCollectionBehaviorFullScreenPrimary |
				NSWindowCollectionBehaviorManaged];
		} else {
			[window->nsWindow setCollectionBehavior:NSWindowCollectionBehaviorFullScreenNone];
		}

		if (wndconfig->floating) {
			[window->nsWindow setLevel:NSFloatingWindowLevel];
		}

		if (wndconfig->maximized) {
			[window->nsWindow zoom:nil];
		}
	}

	window->nsView = [[MacContentView alloc] initWithPlafWindow:window];
	window->nsScaleFramebuffer = wndconfig->scaleFramebuffer;

	if (fbconfig->transparent) {
		[window->nsWindow setOpaque:NO];
		[window->nsWindow setHasShadow:NO];
		[window->nsWindow setBackgroundColor:[NSColor clearColor]];
	}

	[window->nsWindow setContentView:window->nsView];
	[window->nsWindow makeFirstResponder:window->nsView];
	[window->nsWindow setTitle:@(window->title)];
	[window->nsWindow setDelegate:(id<NSWindowDelegate>)window->nsDelegate];
	[window->nsWindow setAcceptsMouseMovedEvents:YES];
	[window->nsWindow setRestorable:NO];

	if ([window->nsWindow respondsToSelector:@selector(setTabbingMode:)]) {
		[window->nsWindow setTabbingMode:NSWindowTabbingModeDisallowed];
	}

	_plafGetWindowSize(window, &window->width, &window->height);
	_plafGetFramebufferSize(window, &window->nsFrameBufferWidth, &window->nsFrameBufferHeight);
	return NULL;
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Transforms a y-coordinate between the CG display and NS screen spaces
//
float _plafTransformYCocoa(float y)
{
	return CGDisplayBounds(CGMainDisplayID()).size.height - y - 1;
}


//////////////////////////////////////////////////////////////////////////
//////                       PLAF platform API                      //////
//////////////////////////////////////////////////////////////////////////

plafError* _plafCreateWindow(plafWindow* window, const plafWindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig) {
	@autoreleasepool {
		plafError* err = createNativeWindow(window, wndconfig, fbconfig);
		if (err) {
			return err;
		}

		err = _plafInitOpenGL();
		if (err) {
			return err;
		}

		err = _plafCreateOpenGLContext(window, ctxconfig, fbconfig);
		if (err) {
			return err;
		}

		err = _plafRefreshContextAttribs(window, ctxconfig);
		if (err) {
			return err;
		}

		if (wndconfig->mousePassthrough) {
			_plafSetWindowMousePassthrough(window, true);
		}

		if (window->monitor) {
			_plafShowWindow(window);
			plafFocusWindow(window);
			acquireMonitor(window);
		}
		return NULL;
	}
}

void _plafDestroyWindow(plafWindow* window) {
	@autoreleasepool {

	[window->nsWindow orderOut:nil];

	if (window->monitor)
		releaseMonitor(window);

	if (window->context.destroy)
		window->context.destroy(window);

	[window->nsWindow setDelegate:nil];
	[window->nsDelegate release];
	window->nsDelegate = nil;

	[window->nsView release];
	window->nsView = nil;

	[window->nsWindow close];
	window->nsWindow = nil;

	// HACK: Allow Cocoa to catch up before returning
	plafPollEvents();

	}
}

void _plafSetWindowTitle(plafWindow* window, const char* title) {
	@autoreleasepool {
	NSString* string = @(title);
	[window->nsWindow setTitle:string];
	// HACK: Set the miniwindow title explicitly as setTitle: doesn't update it
	//       if the window lacks NSWindowStyleMaskTitled
	[window->nsWindow setMiniwindowTitle:string];
	}
}

void _plafSetWindowIcon(plafWindow* window, int count, const plafImageData* images) {
	// Windows don't have icons on macOS
}

void _plafGetWindowPos(plafWindow* window, int* xpos, int* ypos) {
	@autoreleasepool {

	const NSRect contentRect =
		[window->nsWindow contentRectForFrameRect:[window->nsWindow frame]];

	if (xpos)
		*xpos = contentRect.origin.x;
	if (ypos)
		*ypos = _plafTransformYCocoa(contentRect.origin.y + contentRect.size.height - 1);

	}
}

void _plafSetWindowPos(plafWindow* window, int x, int y) {
	@autoreleasepool {

	const NSRect contentRect = [window->nsView frame];
	const NSRect dummyRect = NSMakeRect(x, _plafTransformYCocoa(y + contentRect.size.height - 1), 0, 0);
	const NSRect frameRect = [window->nsWindow frameRectForContentRect:dummyRect];
	[window->nsWindow setFrameOrigin:frameRect.origin];

	}
}

void _plafGetWindowSize(plafWindow* window, int* width, int* height) {
	@autoreleasepool {

	const NSRect contentRect = [window->nsView frame];

	if (width)
		*width = contentRect.size.width;
	if (height)
		*height = contentRect.size.height;

	}
}

void _plafSetWindowSize(plafWindow* window, int width, int height) {
	@autoreleasepool {

	if (window->monitor)
	{
		if (window->monitor->window == window)
			acquireMonitor(window);
	}
	else
	{
		NSRect contentRect =
			[window->nsWindow contentRectForFrameRect:[window->nsWindow frame]];
		contentRect.origin.y += contentRect.size.height - height;
		contentRect.size = NSMakeSize(width, height);
		[window->nsWindow setFrame:[window->nsWindow frameRectForContentRect:contentRect] display:YES];
	}

	}
}

void _plafSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight) {
	@autoreleasepool {

	if (minwidth == DONT_CARE || minheight == DONT_CARE)
		[window->nsWindow setContentMinSize:NSMakeSize(0, 0)];
	else
		[window->nsWindow setContentMinSize:NSMakeSize(minwidth, minheight)];

	if (maxwidth == DONT_CARE || maxheight == DONT_CARE)
		[window->nsWindow setContentMaxSize:NSMakeSize(DBL_MAX, DBL_MAX)];
	else
		[window->nsWindow setContentMaxSize:NSMakeSize(maxwidth, maxheight)];

	}
}

void _plafGetFramebufferSize(plafWindow* window, int* width, int* height) {
	@autoreleasepool {

	const NSRect contentRect = [window->nsView frame];
	const NSRect fbRect = [window->nsView convertRectToBacking:contentRect];

	if (width)
		*width = (int) fbRect.size.width;
	if (height)
		*height = (int) fbRect.size.height;

	}
}

void _plafGetWindowFrameSize(plafWindow* window, int* left, int* top, int* right, int* bottom) {
	@autoreleasepool {

	const NSRect contentRect = [window->nsView frame];
	const NSRect frameRect = [window->nsWindow frameRectForContentRect:contentRect];

	if (left)
		*left = contentRect.origin.x - frameRect.origin.x;
	if (top)
		*top = frameRect.origin.y + frameRect.size.height -
			   contentRect.origin.y - contentRect.size.height;
	if (right)
		*right = frameRect.origin.x + frameRect.size.width -
				 contentRect.origin.x - contentRect.size.width;
	if (bottom)
		*bottom = contentRect.origin.y - frameRect.origin.y;

	}
}

void _plafGetWindowContentScale(plafWindow* window, float* xscale, float* yscale) {
	@autoreleasepool {

	const NSRect points = [window->nsView frame];
	const NSRect pixels = [window->nsView convertRectToBacking:points];

	if (xscale)
		*xscale = (float) (pixels.size.width / points.size.width);
	if (yscale)
		*yscale = (float) (pixels.size.height / points.size.height);

	}
}

void plafMinimizeWindow(plafWindow* window) {
	@autoreleasepool {
	[window->nsWindow miniaturize:nil];
	}
}

void plafRestoreWindow(plafWindow* window) {
	@autoreleasepool {
	if ([window->nsWindow isMiniaturized])
		[window->nsWindow deminiaturize:nil];
	else if ([window->nsWindow isZoomed])
		[window->nsWindow zoom:nil];
	}
}

void _plafMaximizeWindow(plafWindow* window) {
	@autoreleasepool {
	if (![window->nsWindow isZoomed])
		[window->nsWindow zoom:nil];
	}
}

void _plafShowWindow(plafWindow* window) {
	@autoreleasepool {
		[window->nsWindow orderFront:nil];
	}
}

void _plafHideWindow(plafWindow* window) {
	@autoreleasepool {
	[window->nsWindow orderOut:nil];
	}
}

void plafRequestWindowAttention(plafWindow* window) {
	@autoreleasepool {
	[NSApp requestUserAttention:NSInformationalRequest];
	}
}

void plafFocusWindow(plafWindow* window) {
	@autoreleasepool {
		// Make us the active application
		// HACK: This is here to prevent applications using only hidden windows from
		//       being activated, but should probably not be done every time any
		//       window is shown
		[NSApp activateIgnoringOtherApps:YES];
		[window->nsWindow makeKeyAndOrderFront:nil];
	}
}

void _plafSetWindowMonitor(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate) {
	@autoreleasepool {

	if (window->monitor == monitor)
	{
		if (monitor)
		{
			if (monitor->window == window)
				acquireMonitor(window);
		}
		else
		{
			const NSRect contentRect = NSMakeRect(xpos, _plafTransformYCocoa(ypos + height - 1), width, height);
			const NSUInteger styleMask = [window->nsWindow styleMask];
			const NSRect frameRect = [NSWindow frameRectForContentRect:contentRect styleMask:styleMask];
			[window->nsWindow setFrame:frameRect display:YES];
		}

		return;
	}

	if (window->monitor) {
		releaseMonitor(window);
	}
	window->monitor = monitor;

	// HACK: Allow the state cached in Cocoa to catch up to reality
	// TODO: Solve this in a less terrible way
	plafPollEvents();

	NSUInteger styleMask = [window->nsWindow styleMask];

	if (window->monitor)
	{
		styleMask &= ~(NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskResizable);
		styleMask |= NSWindowStyleMaskBorderless;
	}
	else
	{
		if (window->decorated)
		{
			styleMask &= ~NSWindowStyleMaskBorderless;
			styleMask |= (NSWindowStyleMaskTitled | NSWindowStyleMaskClosable);
		}

		if (window->resizable)
			styleMask |= NSWindowStyleMaskResizable;
		else
			styleMask &= ~NSWindowStyleMaskResizable;
	}

	[window->nsWindow setStyleMask:styleMask];
	// HACK: Changing the style mask can cause the first responder to be cleared
	[window->nsWindow makeFirstResponder:window->nsView];

	if (window->monitor)
	{
		[window->nsWindow setLevel:NSMainMenuWindowLevel + 1];
		[window->nsWindow setHasShadow:NO];

		acquireMonitor(window);
	}
	else
	{
		NSRect contentRect = NSMakeRect(xpos, _plafTransformYCocoa(ypos + height - 1),
										width, height);
		NSRect frameRect = [NSWindow frameRectForContentRect:contentRect styleMask:styleMask];
		[window->nsWindow setFrame:frameRect display:YES];

		if (window->numer != DONT_CARE &&
			window->denom != DONT_CARE)
		{
			[window->nsWindow setContentAspectRatio:NSMakeSize(window->numer,
																window->denom)];
		}

		if (window->minwidth != DONT_CARE &&
			window->minheight != DONT_CARE)
		{
			[window->nsWindow setContentMinSize:NSMakeSize(window->minwidth,
															window->minheight)];
		}

		if (window->maxwidth != DONT_CARE &&
			window->maxheight != DONT_CARE)
		{
			[window->nsWindow setContentMaxSize:NSMakeSize(window->maxwidth,
															window->maxheight)];
		}

		if (window->floating)
			[window->nsWindow setLevel:NSFloatingWindowLevel];
		else
			[window->nsWindow setLevel:NSNormalWindowLevel];

		if (window->resizable)
		{
			const NSWindowCollectionBehavior behavior =
				NSWindowCollectionBehaviorFullScreenPrimary |
				NSWindowCollectionBehaviorManaged;
			[window->nsWindow setCollectionBehavior:behavior];
		}
		else
		{
			const NSWindowCollectionBehavior behavior =
				NSWindowCollectionBehaviorFullScreenNone;
			[window->nsWindow setCollectionBehavior:behavior];
		}

		[window->nsWindow setHasShadow:YES];
		// HACK: Clearing NSWindowStyleMaskTitled resets and disables the window
		//       title property but the miniwindow title property is unaffected
		[window->nsWindow setTitle:[window->nsWindow miniwindowTitle]];
	}

	}
}

IntBool _plafWindowFocused(plafWindow* window) {
	@autoreleasepool {
	return [window->nsWindow isKeyWindow];
	}
}

IntBool _plafWindowMinimized(plafWindow* window) {
	@autoreleasepool {
	return [window->nsWindow isMiniaturized];
	}
}

IntBool _plafWindowVisible(plafWindow* window) {
	@autoreleasepool {
	return [window->nsWindow isVisible];
	}
}

IntBool _plafWindowMaximized(plafWindow* window) {
	@autoreleasepool {

	if (window->resizable)
		return [window->nsWindow isZoomed];
	else
		return false;

	}
}

IntBool _plafWindowHovered(plafWindow* window) {
	@autoreleasepool {

	const NSPoint point = [NSEvent mouseLocation];

	if ([NSWindow windowNumberAtPoint:point belowWindowWithWindowNumber:0] !=
		[window->nsWindow windowNumber])
	{
		return false;
	}

	return NSMouseInRect(point,
		[window->nsWindow convertRectToScreen:[window->nsView frame]], NO);

	}
}

IntBool _plafFramebufferTransparent(plafWindow* window) {
	@autoreleasepool {
	return ![window->nsWindow isOpaque] && ![window->nsView isOpaque];
	}
}

void _plafSetWindowResizable(plafWindow* window, IntBool enabled) {
	@autoreleasepool {

	const NSUInteger styleMask = [window->nsWindow styleMask];
	if (enabled)
	{
		[window->nsWindow setStyleMask:(styleMask | NSWindowStyleMaskResizable)];
		const NSWindowCollectionBehavior behavior =
			NSWindowCollectionBehaviorFullScreenPrimary |
			NSWindowCollectionBehaviorManaged;
		[window->nsWindow setCollectionBehavior:behavior];
	}
	else
	{
		[window->nsWindow setStyleMask:(styleMask & ~NSWindowStyleMaskResizable)];
		const NSWindowCollectionBehavior behavior =
			NSWindowCollectionBehaviorFullScreenNone;
		[window->nsWindow setCollectionBehavior:behavior];
	}

	}
}

void _plafSetWindowDecorated(plafWindow* window, IntBool enabled) {
	@autoreleasepool {

	NSUInteger styleMask = [window->nsWindow styleMask];
	if (enabled)
	{
		styleMask |= (NSWindowStyleMaskTitled | NSWindowStyleMaskClosable);
		styleMask &= ~NSWindowStyleMaskBorderless;
	}
	else
	{
		styleMask |= NSWindowStyleMaskBorderless;
		styleMask &= ~(NSWindowStyleMaskTitled | NSWindowStyleMaskClosable);
	}

	[window->nsWindow setStyleMask:styleMask];
	[window->nsWindow makeFirstResponder:window->nsView];

	}
}

void _plafSetWindowFloating(plafWindow* window, IntBool enabled) {
	@autoreleasepool {
	if (enabled)
		[window->nsWindow setLevel:NSFloatingWindowLevel];
	else
		[window->nsWindow setLevel:NSNormalWindowLevel];
	}
}

void _plafSetWindowMousePassthrough(plafWindow* window, IntBool enabled) {
	@autoreleasepool {
		[window->nsWindow setIgnoresMouseEvents:enabled];
	}
}

float plafGetWindowOpacity(plafWindow* window) {
	@autoreleasepool {
	return (float) [window->nsWindow alphaValue];
	}
}

void _plafSetWindowOpacity(plafWindow* window, float opacity) {
	@autoreleasepool {
	[window->nsWindow setAlphaValue:opacity];
	}
}

void plafPollEvents(void) {
	@autoreleasepool {
		for (;;) {
			NSEvent* event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:[NSDate distantPast]
				inMode:NSDefaultRunLoopMode dequeue:YES];
			if (event == nil) {
				break;
			}
			[NSApp sendEvent:event];
		}
	}
}

void plafWaitEvents(void) {
	@autoreleasepool {
		NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:[NSDate distantFuture]
			inMode:NSDefaultRunLoopMode dequeue:YES];
		[NSApp sendEvent:event];
		plafPollEvents();
	}
}

void _plafWaitEventsTimeout(double timeout) {
	@autoreleasepool {
		NSDate* date = [NSDate dateWithTimeIntervalSinceNow:timeout];
		NSEvent* event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:date inMode:NSDefaultRunLoopMode
			dequeue:YES];
		if (event) {
			[NSApp sendEvent:event];
		}
		plafPollEvents();
	}
}

void plafPostEmptyEvent(void) {
	@autoreleasepool {
		NSEvent* event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSMakePoint(0, 0)
			modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
		[NSApp postEvent:event atStart:YES];
	}
}

void _plafUpdateCursor(plafWindow* window) {
	@autoreleasepool {
		if (_plafWindowFocused(window)) {
			if (_plafCursorInContentArea(window)) {
				_plafUpdateCursorImage(window);
			}
		}
	}
}

IntBool _plafCreateCursor(plafCursor* cursor, const plafImageData* image, int xhot, int yhot) {
	@autoreleasepool {

	NSImage* native;
	NSBitmapImageRep* rep;

	rep = [[NSBitmapImageRep alloc]
		initWithBitmapDataPlanes:NULL
					  pixelsWide:image->width
					  pixelsHigh:image->height
				   bitsPerSample:8
				 samplesPerPixel:4
						hasAlpha:YES
						isPlanar:NO
				  colorSpaceName:NSCalibratedRGBColorSpace
					bitmapFormat:NSBitmapFormatAlphaNonpremultiplied
					 bytesPerRow:image->width * 4
					bitsPerPixel:32];

	if (rep == nil)
		return false;

	memcpy([rep bitmapData], image->pixels, image->width * image->height * 4);

	native = [[NSImage alloc] initWithSize:NSMakeSize(image->width, image->height)];
	[native addRepresentation:rep];

	cursor->nsCursor = [[NSCursor alloc] initWithImage:native
												hotSpot:NSMakePoint(xhot, yhot)];

	[native release];
	[rep release];

	if (cursor->nsCursor == nil)
		return false;

	return true;

	}
}

IntBool _plafCreateStandardCursor(plafCursor* cursor, int shape) {
	@autoreleasepool {
	if (!cursor->nsCursor)
	{
		switch (shape)
		{
			case STD_CURSOR_ARROW:
				cursor->nsCursor = [NSCursor arrowCursor];
				break;
			case STD_CURSOR_IBEAM:
				cursor->nsCursor = [NSCursor IBeamCursor];
				break;
			case STD_CURSOR_CROSSHAIR:
				cursor->nsCursor = [NSCursor crosshairCursor];
				break;
			case STD_CURSOR_POINTING_HAND:
				cursor->nsCursor = [NSCursor pointingHandCursor];
				break;
			case STD_CURSOR_HORIZONTAL_RESIZE:
				cursor->nsCursor = [NSCursor resizeLeftRightCursor];
				break;
			case STD_CURSOR_VERTICAL_RESIZE:
				cursor->nsCursor = [NSCursor resizeUpDownCursor];
				break;
		}
	}

	if (!cursor->nsCursor)
	{
		_plafInputError("Cocoa: Standard cursor shape unavailable");
		return false;
	}

	[cursor->nsCursor retain];
	return true;

	}
}

void _plafDestroyCursor(plafCursor* cursor) {
	if (cursor->nsCursor) {
		[cursor->nsCursor release];
		cursor->nsCursor = nil;
	}
}


//////////////////////////////////////////////////////////////////////////
//////                        PLAF native API                       //////
//////////////////////////////////////////////////////////////////////////

void* plafGetNativeWindow(plafWindow* window) {
	return window->nsWindow;
}

id plafGetCocoaView(plafWindow* window) {
	return window->nsView;
}

#endif // __APPLE__
