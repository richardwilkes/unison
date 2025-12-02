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
		goWindowMaximizeCallback(window, maximized);
	}
	const NSRect contentRect = [window->nsView frame];
	if (contentRect.size.width != window->width || contentRect.size.height != window->height) {
		window->width  = contentRect.size.width;
		window->height = contentRect.size.height;
		goWindowSizeCallback(window);
	}
}

- (void)windowDidMove:(NSNotification *)notification {
	[window->context.nsglCtx update];
	goWindowPosCallback(window);
}

- (void)windowDidMiniaturize:(NSNotification *)notification {
	goWindowMinimizeCallback(window, true);
}

- (void)windowDidDeminiaturize:(NSNotification *)notification {
	goWindowMinimizeCallback(window, false);
}

- (void)windowDidBecomeKey:(NSNotification *)notification {
	_plafNotifyOfFocusChange(window, true);
	if (_plafCursorInContentArea(window)) {
		_plafUpdateCursorImage(window);
	}
}

- (void)windowDidResignKey:(NSNotification *)notification {
	_plafNotifyOfFocusChange(window, false);
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
	goWindowDrawCallback(window);
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

- (void)mouseEntered:(NSEvent *)event {
	if (window->cursorHidden) {
		hideCursor(window);
	}
	goCursorEnterCallback(window, true);
}

- (void)mouseExited:(NSEvent *)event {
	if (window->cursorHidden) {
		showCursor(window);
	}
	goCursorEnterCallback(window, false);
}

- (void)viewDidChangeBackingProperties {
	const NSRect contentRect = [window->nsView frame];
	const NSRect fbRect = [window->nsView convertRectToBacking:contentRect];
	const float xscale = fbRect.size.width / contentRect.size.width;
	const float yscale = fbRect.size.height / contentRect.size.height;
	if (xscale != window->nsXScale || yscale != window->nsYScale) {
		window->nsXScale = xscale;
		window->nsYScale = yscale;
		goWindowContentScaleCallback(window);
	}
}

- (void)drawRect:(NSRect)rect {
	goWindowDrawCallback(window);
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
		goScrollCallback(window, deltaX, deltaY);
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
	int count = [urls count];
	if (count) {
		char** paths = _plaf_calloc(count, sizeof(char*));
		for (int i = 0; i < count; i++) {
			paths[i] = _plaf_strdup([urls[i] fileSystemRepresentation]);
		}
		goDropCallback(window, count, paths);
		for (NSUInteger i = 0; i < count; i++) {
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
	NSEvent* event = [NSApp currentEvent];
	const int mods = translateFlags([event modifierFlags]);
	if (mods & KEYMOD_SUPER) {
		return;
	}
	NSString* characters;
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
			_plafInputChar(window, codepoint);
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
static bool createNativeWindow(plafWindow* window, const plafWindowConfig* wndconfig, const plafFrameBufferCfg* fbconfig) {
	window->nsDelegate = [[MacWindowDelegate alloc] initWithPlafWindow:window];
	if (!window->nsDelegate) {
		return false;
	}

	NSRect contentRect = NSMakeRect(0, 0, 1, 1);
	NSUInteger styleMask = NSWindowStyleMaskMiniaturizable;
	if (!window->decorated) {
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
		return false;
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

	window->nsView = [[MacContentView alloc] initWithPlafWindow:window];

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

	plafGetWindowSize(window, &window->width, &window->height);
	plafGetFramebufferSize(window, &window->nsFrameBufferWidth, &window->nsFrameBufferHeight);
	return true;
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

bool _plafCreateWindow(plafWindow* window, const plafWindowConfig* wndconfig, plafWindow* share, const plafFrameBufferCfg* fbconfig) {
	@autoreleasepool {
		if (!createNativeWindow(window, wndconfig, fbconfig)) {
			return false;
		}
		if (!_plafInitOpenGL()) {
			return false;
		}
		if (!_plafCreateOpenGLContext(window, share, fbconfig)) {
			return false;
		}
		if (wndconfig->mousePassthrough) {
			_plafSetWindowMousePassthrough(window, true);
		}
		return true;
	}
}

void _plafDestroyWindow(plafWindow* window) {
	@autoreleasepool {
		[window->nsWindow orderOut:nil];
		if (window->context.destroy) {
			window->context.destroy(window);
		}
		[window->nsWindow setDelegate:nil];
		[window->nsDelegate release];
		window->nsDelegate = nil;
		[window->nsView release];
		window->nsView = nil;
		[window->nsWindow close];
		window->nsWindow = nil;
		plafPollEvents();
	}
}

void _plafSetWindowTitle(plafWindow* window, const char* title) {
	@autoreleasepool {
		NSString* string = @(title);
		[window->nsWindow setTitle:string];
		// Set the miniwindow title explicitly as setTitle: doesn't update it if the window lacks NSWindowStyleMaskTitled
		[window->nsWindow setMiniwindowTitle:string];
	}
}

void plafSetWindowIcon(plafWindow* window, int count, const plafImageData* images) {
	// Windows don't have icons on macOS
}

void plafGetWindowPos(plafWindow* window, int* xpos, int* ypos) {
	@autoreleasepool {
		const NSRect contentRect = [window->nsWindow contentRectForFrameRect:[window->nsWindow frame]];
		*xpos = contentRect.origin.x;
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

void plafGetWindowSize(plafWindow* window, int* width, int* height) {
	@autoreleasepool {
		const NSRect contentRect = [window->nsView frame];
		*width = contentRect.size.width;
		*height = contentRect.size.height;
	}
}

void _plafSetWindowSize(plafWindow* window, int width, int height) {
	@autoreleasepool {
		NSRect contentRect = [window->nsWindow contentRectForFrameRect:[window->nsWindow frame]];
		contentRect.origin.y += contentRect.size.height - height;
		contentRect.size = NSMakeSize(width, height);
		[window->nsWindow setFrame:[window->nsWindow frameRectForContentRect:contentRect] display:YES];
	}
}

void _plafSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight) {
	@autoreleasepool {
		if (minwidth == DONT_CARE || minheight == DONT_CARE) {
			[window->nsWindow setContentMinSize:NSMakeSize(0, 0)];
		} else {
			[window->nsWindow setContentMinSize:NSMakeSize(minwidth, minheight)];
		}
		if (maxwidth == DONT_CARE || maxheight == DONT_CARE) {
			[window->nsWindow setContentMaxSize:NSMakeSize(DBL_MAX, DBL_MAX)];
		} else {
			[window->nsWindow setContentMaxSize:NSMakeSize(maxwidth, maxheight)];
		}
	}
}

void plafGetFramebufferSize(plafWindow* window, int* width, int* height) {
	@autoreleasepool {
		const NSRect contentRect = [window->nsView frame];
		const NSRect fbRect = [window->nsView convertRectToBacking:contentRect];
		*width = (int) fbRect.size.width;
		*height = (int) fbRect.size.height;
	}
}

void plafGetWindowFrameSize(plafWindow* window, int* left, int* top, int* right, int* bottom) {
	@autoreleasepool {
		const NSRect contentRect = [window->nsView frame];
		const NSRect frameRect = [window->nsWindow frameRectForContentRect:contentRect];
		*left = contentRect.origin.x - frameRect.origin.x;
		*top = frameRect.origin.y + frameRect.size.height - contentRect.origin.y - contentRect.size.height;
		*right = frameRect.origin.x + frameRect.size.width - contentRect.origin.x - contentRect.size.width;
		*bottom = contentRect.origin.y - frameRect.origin.y;
	}
}

void plafGetWindowContentScale(plafWindow* window, float* xscale, float* yscale) {
	@autoreleasepool {
		const NSRect points = [window->nsView frame];
		const NSRect pixels = [window->nsView convertRectToBacking:points];
		*xscale = (float) (pixels.size.width / points.size.width);
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
		if ([window->nsWindow isMiniaturized]) {
			[window->nsWindow deminiaturize:nil];
		} else if ([window->nsWindow isZoomed]) {
			[window->nsWindow zoom:nil];
		}
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
		[NSApp activateIgnoringOtherApps:YES];
		[window->nsWindow makeKeyAndOrderFront:nil];
	}
}

bool plafIsWindowFocused(plafWindow* window) {
	return [window->nsWindow isKeyWindow];
}

bool plafIsWindowMinimized(plafWindow* window) {
	return [window->nsWindow isMiniaturized];
}

bool plafWindowVisible(plafWindow* window) {
	return [window->nsWindow isVisible];
}

bool plafIsWindowMaximized(plafWindow* window) {
	if (window->resizable) {
		return [window->nsWindow isZoomed];
	}
	return false;
}

bool plafIsFramebufferTransparent(plafWindow* window) {
	return ![window->nsWindow isOpaque] && ![window->nsView isOpaque];
}

void _plafSetWindowResizable(plafWindow* window, bool enabled) {
	@autoreleasepool {
		const NSUInteger styleMask = [window->nsWindow styleMask];
		if (enabled) {
			[window->nsWindow setStyleMask:(styleMask | NSWindowStyleMaskResizable)];
			[window->nsWindow setCollectionBehavior:NSWindowCollectionBehaviorFullScreenPrimary |
				NSWindowCollectionBehaviorManaged];
		} else {
			[window->nsWindow setStyleMask:(styleMask & ~NSWindowStyleMaskResizable)];
			[window->nsWindow setCollectionBehavior:NSWindowCollectionBehaviorFullScreenNone];
		}
	}
}

void _plafSetWindowDecorated(plafWindow* window, bool enabled) {
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

void _plafSetWindowFloating(plafWindow* window, bool enabled) {
	@autoreleasepool {
	if (enabled)
		[window->nsWindow setLevel:NSFloatingWindowLevel];
	else
		[window->nsWindow setLevel:NSNormalWindowLevel];
	}
}

void _plafSetWindowMousePassthrough(plafWindow* window, bool enabled) {
	@autoreleasepool {
		[window->nsWindow setIgnoresMouseEvents:enabled];
	}
}

float plafGetWindowOpacity(plafWindow* window) {
	@autoreleasepool {
		return (float) [window->nsWindow alphaValue];
	}
}

void plafSetWindowOpacity(plafWindow* window, float opacity) {
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

void plafWaitEventsTimeout(double timeout) {
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
		if (plafIsWindowFocused(window)) {
			if (_plafCursorInContentArea(window)) {
				_plafUpdateCursorImage(window);
			}
		}
	}
}

bool _plafCreateCursor(plafCursor* cursor, const plafImageData* image, int xhot, int yhot) {
	@autoreleasepool {
		NSBitmapImageRep* rep = [[NSBitmapImageRep alloc] initWithBitmapDataPlanes:NULL pixelsWide:image->width
			pixelsHigh:image->height bitsPerSample:8 samplesPerPixel:4 hasAlpha:YES isPlanar:NO
			colorSpaceName:NSCalibratedRGBColorSpace bitmapFormat:NSBitmapFormatAlphaNonpremultiplied
			bytesPerRow:image->width * 4 bitsPerPixel:32];
		if (rep == nil) {
			return false;
		}
		memcpy([rep bitmapData], image->pixels, image->width * image->height * 4);
		NSImage* img = [[NSImage alloc] initWithSize:NSMakeSize(image->width, image->height)];
		[img addRepresentation:rep];
		cursor->nsCursor = [[NSCursor alloc] initWithImage:img hotSpot:NSMakePoint(xhot, yhot)];
		[img release];
		[rep release];
		return cursor->nsCursor != nil;
	}
}

bool _plafCreateStandardCursor(plafCursor* cursor, int shape) {
	@autoreleasepool {
		if (!cursor->nsCursor) {
			switch (shape) {
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
		if (!cursor->nsCursor) {
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
