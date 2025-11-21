#if defined(__APPLE__)

#include "platform.h"

#import <QuartzCore/CAMetalLayer.h>

// Hides the cursor if not already hidden
//
static void hideCursor(plafWindow* window)
{
	if (!_glfw.nsCursorHidden)
	{
		[NSCursor hide];
		_glfw.nsCursorHidden = true;
	}
}

// Shows the cursor if not already shown
//
static void showCursor(plafWindow* window)
{
	if (_glfw.nsCursorHidden)
	{
		[NSCursor unhide];
		_glfw.nsCursorHidden = false;
	}
}

// Updates the cursor image according to its cursor mode
//
void updateCursorImage(plafWindow* window)
{
	if (window->cursorMode == CURSOR_NORMAL)
	{
		showCursor(window);

		if (window->cursor)
			[window->cursor->nsCursor set];
		else
			[[NSCursor arrowCursor] set];
	}
	else
		hideCursor(window);
}

// Apply chosen cursor mode to a focused window
//
static void updateCursorMode(plafWindow* window)
{
	if (cursorInContentArea(window))
		updateCursorImage(window);
}

// Make the specified window and its video mode active on its monitor
//
static void acquireMonitor(plafWindow* window)
{
	_glfwSetVideoMode(window->monitor, &window->videoMode);
	const CGRect bounds = CGDisplayBounds(window->monitor->nsDisplayID);
	const NSRect frame = NSMakeRect(bounds.origin.x,
									_glfwTransformYCocoa(bounds.origin.y + bounds.size.height - 1),
									bounds.size.width,
									bounds.size.height);

	[window->nsWindow setFrame:frame display:YES];

	_glfwInputMonitorWindow(window->monitor, window);
}

// Remove the window and restore the original video mode
//
static void releaseMonitor(plafWindow* window)
{
	if (window->monitor->window != window)
		return;

	_glfwInputMonitorWindow(window->monitor, NULL);
	_glfwRestoreVideoModeCocoa(window->monitor);
}

// Translates macOS key modifiers into GLFW ones
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

// Translates a macOS keycode to a GLFW keycode
//
static int translateKey(unsigned int key) {
	if (key >= MAX_KEY_CODES) {
		return KEY_UNKNOWN;
	}
	return _glfw.keyCodes[key];
}

// Translate a GLFW keycode to a Cocoa modifier flag
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

@interface GLFWWindowDelegate : NSObject
{
	plafWindow* window;
}

- (instancetype)initWithGlfwWindow:(plafWindow *)initWindow;

@end

@implementation GLFWWindowDelegate

- (instancetype)initWithGlfwWindow:(plafWindow *)initWindow
{
	self = [super init];
	if (self != nil)
		window = initWindow;

	return self;
}

- (BOOL)windowShouldClose:(id)sender
{
	_glfwInputWindowCloseRequest(window);
	return NO;
}

- (void)windowDidResize:(NSNotification *)notification
{
	[window->context.nsglCtx update];

	const int maximized = [window->nsWindow isZoomed];
	if (window->maximized != maximized)
	{
		window->maximized = maximized;
		_glfwInputWindowMaximize(window, maximized);
	}

	const NSRect contentRect = [window->nsView frame];
	const NSRect fbRect = [window->nsView convertRectToBacking:contentRect];

	if (fbRect.size.width != window->nsFrameBufferWidth ||
		fbRect.size.height != window->nsFrameBufferHeight)
	{
		window->nsFrameBufferWidth  = fbRect.size.width;
		window->nsFrameBufferHeight = fbRect.size.height;
		_glfwInputFramebufferSize(window, fbRect.size.width, fbRect.size.height);
	}

	if (contentRect.size.width != window->width ||
		contentRect.size.height != window->height)
	{
		window->width  = contentRect.size.width;
		window->height = contentRect.size.height;
		_glfwInputWindowSize(window, contentRect.size.width, contentRect.size.height);
	}
}

- (void)windowDidMove:(NSNotification *)notification
{
	[window->context.nsglCtx update];

	int x, y;
	_glfwGetWindowPos(window, &x, &y);
	_glfwInputWindowPos(window, x, y);
}

- (void)windowDidMiniaturize:(NSNotification *)notification
{
	if (window->monitor)
		releaseMonitor(window);

	_glfwInputWindowIconify(window, true);
}

- (void)windowDidDeminiaturize:(NSNotification *)notification
{
	if (window->monitor)
		acquireMonitor(window);

	_glfwInputWindowIconify(window, false);
}

- (void)windowDidBecomeKey:(NSNotification *)notification
{

	_glfwInputWindowFocus(window, true);
	updateCursorMode(window);
}

- (void)windowDidResignKey:(NSNotification *)notification
{
	_glfwInputWindowFocus(window, false);
}

@end


//------------------------------------------------------------------------
// Content view class for the GLFW window
//------------------------------------------------------------------------

@interface GLFWContentView : NSView <NSTextInputClient>
{
	plafWindow* window;
	NSTrackingArea* trackingArea;
	NSMutableAttributedString* markedText;
}

- (instancetype)initWithGlfwWindow:(plafWindow *)initWindow;

@end

@implementation GLFWContentView

- (instancetype)initWithGlfwWindow:(plafWindow *)initWindow
{
	self = [super init];
	if (self != nil)
	{
		window = initWindow;
		trackingArea = nil;
		markedText = [[NSMutableAttributedString alloc] init];

		[self updateTrackingAreas];
		[self registerForDraggedTypes:@[NSPasteboardTypeURL]];
	}

	return self;
}

- (void)dealloc
{
	[trackingArea release];
	[markedText release];
	[super dealloc];
}

- (BOOL)isOpaque
{
	return [window->nsWindow isOpaque];
}

- (BOOL)canBecomeKeyView
{
	return YES;
}

- (BOOL)acceptsFirstResponder
{
	return YES;
}

- (BOOL)wantsUpdateLayer
{
	return YES;
}

- (void)updateLayer
{
	[window->context.nsglCtx update];
	_glfwInputWindowDamage(window);
}

- (void)cursorUpdate:(NSEvent *)event
{
	updateCursorImage(window);
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event
{
	return YES;
}

- (void)mouseDown:(NSEvent *)event
{
	_glfwInputMouseClick(window,
						 MOUSE_BUTTON_LEFT,
						 INPUT_PRESS,
						 translateFlags([event modifierFlags]));
}

- (void)mouseDragged:(NSEvent *)event
{
	[self mouseMoved:event];
}

- (void)mouseUp:(NSEvent *)event
{
	_glfwInputMouseClick(window,
						 MOUSE_BUTTON_LEFT,
						 INPUT_RELEASE,
						 translateFlags([event modifierFlags]));
}

- (void)mouseMoved:(NSEvent *)event
{
	const NSRect contentRect = [window->nsView frame];
	// NOTE: The returned location uses base 0,1 not 0,0
	const NSPoint pos = [event locationInWindow];

	_glfwInputCursorPos(window, pos.x, contentRect.size.height - pos.y);
}

- (void)rightMouseDown:(NSEvent *)event
{
	_glfwInputMouseClick(window,
						 MOUSE_BUTTON_RIGHT,
						 INPUT_PRESS,
						 translateFlags([event modifierFlags]));
}

- (void)rightMouseDragged:(NSEvent *)event
{
	[self mouseMoved:event];
}

- (void)rightMouseUp:(NSEvent *)event
{
	_glfwInputMouseClick(window,
						 MOUSE_BUTTON_RIGHT,
						 INPUT_RELEASE,
						 translateFlags([event modifierFlags]));
}

- (void)otherMouseDown:(NSEvent *)event
{
	_glfwInputMouseClick(window,
						 (int) [event buttonNumber],
						 INPUT_PRESS,
						 translateFlags([event modifierFlags]));
}

- (void)otherMouseDragged:(NSEvent *)event
{
	[self mouseMoved:event];
}

- (void)otherMouseUp:(NSEvent *)event
{
	_glfwInputMouseClick(window,
						 (int) [event buttonNumber],
						 INPUT_RELEASE,
						 translateFlags([event modifierFlags]));
}

- (void)mouseExited:(NSEvent *)event
{
	if (window->cursorMode == CURSOR_HIDDEN)
		showCursor(window);

	_glfwInputCursorEnter(window, false);
}

- (void)mouseEntered:(NSEvent *)event
{
	if (window->cursorMode == CURSOR_HIDDEN)
		hideCursor(window);

	_glfwInputCursorEnter(window, true);
}

- (void)viewDidChangeBackingProperties
{
	const NSRect contentRect = [window->nsView frame];
	const NSRect fbRect = [window->nsView convertRectToBacking:contentRect];
	const float xscale = fbRect.size.width / contentRect.size.width;
	const float yscale = fbRect.size.height / contentRect.size.height;

	if (xscale != window->nsXScale || yscale != window->nsYScale)
	{
		// if (window->nsScaleFramebuffer && window->ns.layer)
		// 	[window->ns.layer setContentsScale:[window->nsWindow backingScaleFactor]];

		window->nsXScale = xscale;
		window->nsYScale = yscale;
		_glfwInputWindowContentScale(window, xscale, yscale);
	}

	if (fbRect.size.width != window->nsFrameBufferWidth ||
		fbRect.size.height != window->nsFrameBufferHeight)
	{
		window->nsFrameBufferWidth  = fbRect.size.width;
		window->nsFrameBufferHeight = fbRect.size.height;
		_glfwInputFramebufferSize(window, fbRect.size.width, fbRect.size.height);
	}
}

- (void)drawRect:(NSRect)rect
{
	_glfwInputWindowDamage(window);
}

- (void)updateTrackingAreas
{
	if (trackingArea != nil)
	{
		[self removeTrackingArea:trackingArea];
		[trackingArea release];
	}

	const NSTrackingAreaOptions options = NSTrackingMouseEnteredAndExited |
										  NSTrackingActiveInKeyWindow |
										  NSTrackingEnabledDuringMouseDrag |
										  NSTrackingCursorUpdate |
										  NSTrackingInVisibleRect |
										  NSTrackingAssumeInside;

	trackingArea = [[NSTrackingArea alloc] initWithRect:[self bounds]
												options:options
												  owner:self
											   userInfo:nil];

	[self addTrackingArea:trackingArea];
	[super updateTrackingAreas];
}

- (void)keyDown:(NSEvent *)event
{
	const int key = translateKey([event keyCode]);
	const int mods = translateFlags([event modifierFlags]);

	_glfwInputKey(window, key, [event keyCode], INPUT_PRESS, mods);

	[self interpretKeyEvents:@[event]];
}

- (void)flagsChanged:(NSEvent *)event
{
	int action;
	const unsigned int modifierFlags =
		[event modifierFlags] & NSEventModifierFlagDeviceIndependentFlagsMask;
	const int key = translateKey([event keyCode]);
	const int mods = translateFlags(modifierFlags);
	const NSUInteger keyFlag = translateKeyToModifierFlag(key);

	if (keyFlag & modifierFlags)
	{
		if (window->keys[key] == INPUT_PRESS)
			action = INPUT_RELEASE;
		else
			action = INPUT_PRESS;
	}
	else
		action = INPUT_RELEASE;

	_glfwInputKey(window, key, [event keyCode], action, mods);
}

- (void)keyUp:(NSEvent *)event
{
	const int key = translateKey([event keyCode]);
	const int mods = translateFlags([event modifierFlags]);
	_glfwInputKey(window, key, [event keyCode], INPUT_RELEASE, mods);
}

- (void)scrollWheel:(NSEvent *)event
{
	double deltaX = [event scrollingDeltaX];
	double deltaY = [event scrollingDeltaY];

	if ([event hasPreciseScrollingDeltas])
	{
		deltaX *= 0.1;
		deltaY *= 0.1;
	}

	if (fabs(deltaX) > 0.0 || fabs(deltaY) > 0.0)
		_glfwInputScroll(window, deltaX, deltaY);
}

- (NSDragOperation)draggingEntered:(id <NSDraggingInfo>)sender
{
	// HACK: We don't know what to say here because we don't know what the
	//       application wants to do with the paths
	return NSDragOperationGeneric;
}

- (BOOL)performDragOperation:(id <NSDraggingInfo>)sender
{
	const NSRect contentRect = [window->nsView frame];
	// NOTE: The returned location uses base 0,1 not 0,0
	const NSPoint pos = [sender draggingLocation];
	_glfwInputCursorPos(window, pos.x, contentRect.size.height - pos.y);

	NSPasteboard* pasteboard = [sender draggingPasteboard];
	NSDictionary* options = @{NSPasteboardURLReadingFileURLsOnlyKey:@YES};
	NSArray* urls = [pasteboard readObjectsForClasses:@[[NSURL class]]
											  options:options];
	const NSUInteger count = [urls count];
	if (count)
	{
		char** paths = _glfw_calloc(count, sizeof(char*));

		for (NSUInteger i = 0;  i < count;  i++)
			paths[i] = _glfw_strdup([urls[i] fileSystemRepresentation]);

		_glfwInputDrop(window, (int) count, (const char**) paths);

		for (NSUInteger i = 0;  i < count;  i++)
			_glfw_free(paths[i]);
		_glfw_free(paths);
	}

	return YES;
}

- (BOOL)hasMarkedText
{
	return [markedText length] > 0;
}

- (NSRange)markedRange
{
	if ([markedText length] > 0)
		return NSMakeRange(0, [markedText length] - 1);
	else
		return kEmptyRange;
}

- (NSRange)selectedRange
{
	return kEmptyRange;
}

- (void)setMarkedText:(id)string
		selectedRange:(NSRange)selectedRange
	 replacementRange:(NSRange)replacementRange
{
	[markedText release];
	if ([string isKindOfClass:[NSAttributedString class]])
		markedText = [[NSMutableAttributedString alloc] initWithAttributedString:string];
	else
		markedText = [[NSMutableAttributedString alloc] initWithString:string];
}

- (void)unmarkText
{
	[[markedText mutableString] setString:@""];
}

- (NSArray*)validAttributesForMarkedText
{
	return [NSArray array];
}

- (NSAttributedString*)attributedSubstringForProposedRange:(NSRange)range
											   actualRange:(NSRangePointer)actualRange
{
	return nil;
}

- (NSUInteger)characterIndexForPoint:(NSPoint)point
{
	return 0;
}

- (NSRect)firstRectForCharacterRange:(NSRange)range
						 actualRange:(NSRangePointer)actualRange
{
	const NSRect frame = [window->nsView frame];
	return NSMakeRect(frame.origin.x, frame.origin.y, 0.0, 0.0);
}

- (void)insertText:(id)string replacementRange:(NSRange)replacementRange
{
	NSString* characters;
	NSEvent* event = [NSApp currentEvent];
	const int mods = translateFlags([event modifierFlags]);
	const int plain = !(mods & KEYMOD_SUPER);

	if ([string isKindOfClass:[NSAttributedString class]])
		characters = [string string];
	else
		characters = (NSString*) string;

	NSRange range = NSMakeRange(0, [characters length]);
	while (range.length)
	{
		uint32_t codepoint = 0;

		if ([characters getBytes:&codepoint
					   maxLength:sizeof(codepoint)
					  usedLength:NULL
						encoding:NSUTF32StringEncoding
						 options:0
						   range:range
				  remainingRange:&range])
		{
			if (codepoint >= 0xf700 && codepoint <= 0xf7ff)
				continue;

			_glfwInputChar(window, codepoint, mods, plain);
		}
	}
}

- (void)doCommandBySelector:(SEL)selector
{
}

@end


//------------------------------------------------------------------------
// GLFW window class
//------------------------------------------------------------------------

@interface GLFWWindow : NSWindow {}
@end

@implementation GLFWWindow

- (BOOL)canBecomeKeyWindow
{
	// Required for NSWindowStyleMaskBorderless windows
	return YES;
}

- (BOOL)canBecomeMainWindow
{
	return YES;
}

@end


// Create the Cocoa window
static ErrorResponse* createNativeWindow(plafWindow* window, const WindowConfig* wndconfig, const plafFrameBufferCfg* fbconfig) {
	window->nsDelegate = [[GLFWWindowDelegate alloc] initWithGlfwWindow:window];
	if (!window->nsDelegate) {
		return createErrorResponse("Cocoa: Failed to create window delegate");
	}

	NSRect contentRect;
	if (window->monitor) {
		VideoMode mode;
		int xpos;
		int ypos;
		_glfwGetVideoMode(window->monitor, &mode);
		glfwGetMonitorPos(window->monitor, &xpos, &ypos);
		contentRect = NSMakeRect(xpos, ypos, mode.width, mode.height);
	} else {
		if (wndconfig->xpos == ANY_POSITION || wndconfig->ypos == ANY_POSITION) {
			contentRect = NSMakeRect(0, 0, wndconfig->width, wndconfig->height);
		} else {
			const int xpos = wndconfig->xpos;
			const int ypos = _glfwTransformYCocoa(wndconfig->ypos + wndconfig->height - 1);
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

	window->nsWindow = [[GLFWWindow alloc] initWithContentRect:contentRect styleMask:styleMask
		backing:NSBackingStoreBuffered defer:NO];
	if (!window->nsWindow) {
		return createErrorResponse("Cocoa: Failed to create window");
	}

	if (window->monitor) {
		[window->nsWindow setLevel:NSMainMenuWindowLevel + 1];
	} else {
		if (wndconfig->xpos == ANY_POSITION || wndconfig->ypos == ANY_POSITION) {
			[(NSWindow*) window->nsWindow center];
			_glfw.nsCascadePoint = NSPointToCGPoint([window->nsWindow cascadeTopLeftFromPoint: NSPointFromCGPoint(_glfw.nsCascadePoint)]);
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

	window->nsView = [[GLFWContentView alloc] initWithGlfwWindow:window];
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

	_glfwGetWindowSize(window, &window->width, &window->height);
	_glfwGetFramebufferSize(window, &window->nsFrameBufferWidth, &window->nsFrameBufferHeight);
	return NULL;
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW internal API                      //////
//////////////////////////////////////////////////////////////////////////

// Transforms a y-coordinate between the CG display and NS screen spaces
//
float _glfwTransformYCocoa(float y)
{
	return CGDisplayBounds(CGMainDisplayID()).size.height - y - 1;
}


//////////////////////////////////////////////////////////////////////////
//////                       GLFW platform API                      //////
//////////////////////////////////////////////////////////////////////////

ErrorResponse* _glfwCreateWindow(plafWindow* window, const WindowConfig* wndconfig, const plafCtxCfg* ctxconfig, const plafFrameBufferCfg* fbconfig) {
	@autoreleasepool {
		ErrorResponse* err = createNativeWindow(window, wndconfig, fbconfig);
		if (err) {
			return err;
		}

		err = _glfwInitNSGL();
		if (err) {
			return err;
		}

		err = _glfwCreateContextNSGL(window, ctxconfig, fbconfig);
		if (err) {
			return err;
		}

		err = _glfwRefreshContextAttribs(window, ctxconfig);
		if (err) {
			return err;
		}

		if (wndconfig->mousePassthrough) {
			_glfwSetWindowMousePassthrough(window, true);
		}

		if (window->monitor) {
			_glfwShowWindow(window);
			glfwFocusWindow(window);
			acquireMonitor(window);
		}
		return NULL;
	}
}

void _glfwDestroyWindow(plafWindow* window) {
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
	glfwPollEvents();

	}
}

void _glfwSetWindowTitle(plafWindow* window, const char* title) {
	@autoreleasepool {
	NSString* string = @(title);
	[window->nsWindow setTitle:string];
	// HACK: Set the miniwindow title explicitly as setTitle: doesn't update it
	//       if the window lacks NSWindowStyleMaskTitled
	[window->nsWindow setMiniwindowTitle:string];
	}
}

void _glfwSetWindowIcon(plafWindow* window, int count, const ImageData* images) {
	// Windows don't have icons on macOS
}

void _glfwGetWindowPos(plafWindow* window, int* xpos, int* ypos) {
	@autoreleasepool {

	const NSRect contentRect =
		[window->nsWindow contentRectForFrameRect:[window->nsWindow frame]];

	if (xpos)
		*xpos = contentRect.origin.x;
	if (ypos)
		*ypos = _glfwTransformYCocoa(contentRect.origin.y + contentRect.size.height - 1);

	}
}

void _glfwSetWindowPos(plafWindow* window, int x, int y) {
	@autoreleasepool {

	const NSRect contentRect = [window->nsView frame];
	const NSRect dummyRect = NSMakeRect(x, _glfwTransformYCocoa(y + contentRect.size.height - 1), 0, 0);
	const NSRect frameRect = [window->nsWindow frameRectForContentRect:dummyRect];
	[window->nsWindow setFrameOrigin:frameRect.origin];

	}
}

void _glfwGetWindowSize(plafWindow* window, int* width, int* height) {
	@autoreleasepool {

	const NSRect contentRect = [window->nsView frame];

	if (width)
		*width = contentRect.size.width;
	if (height)
		*height = contentRect.size.height;

	}
}

void _glfwSetWindowSize(plafWindow* window, int width, int height) {
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

void _glfwSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight) {
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

void _glfwSetWindowAspectRatio(plafWindow* window, int numer, int denom) {
	@autoreleasepool {
	if (numer == DONT_CARE || denom == DONT_CARE)
		[window->nsWindow setResizeIncrements:NSMakeSize(1.0, 1.0)];
	else
		[window->nsWindow setContentAspectRatio:NSMakeSize(numer, denom)];
	}
}

void _glfwGetFramebufferSize(plafWindow* window, int* width, int* height) {
	@autoreleasepool {

	const NSRect contentRect = [window->nsView frame];
	const NSRect fbRect = [window->nsView convertRectToBacking:contentRect];

	if (width)
		*width = (int) fbRect.size.width;
	if (height)
		*height = (int) fbRect.size.height;

	}
}

void _glfwGetWindowFrameSize(plafWindow* window, int* left, int* top, int* right, int* bottom) {
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

void _glfwGetWindowContentScale(plafWindow* window, float* xscale, float* yscale) {
	@autoreleasepool {

	const NSRect points = [window->nsView frame];
	const NSRect pixels = [window->nsView convertRectToBacking:points];

	if (xscale)
		*xscale = (float) (pixels.size.width / points.size.width);
	if (yscale)
		*yscale = (float) (pixels.size.height / points.size.height);

	}
}

void glfwIconifyWindow(plafWindow* window) {
	@autoreleasepool {
	[window->nsWindow miniaturize:nil];
	}
}

void glfwRestoreWindow(plafWindow* window) {
	@autoreleasepool {
	if ([window->nsWindow isMiniaturized])
		[window->nsWindow deminiaturize:nil];
	else if ([window->nsWindow isZoomed])
		[window->nsWindow zoom:nil];
	}
}

void _glfwMaximizeWindow(plafWindow* window) {
	@autoreleasepool {
	if (![window->nsWindow isZoomed])
		[window->nsWindow zoom:nil];
	}
}

void _glfwShowWindow(plafWindow* window) {
	@autoreleasepool {
		[window->nsWindow orderFront:nil];
	}
}

void _glfwHideWindow(plafWindow* window) {
	@autoreleasepool {
	[window->nsWindow orderOut:nil];
	}
}

void glfwRequestWindowAttention(plafWindow* window) {
	@autoreleasepool {
	[NSApp requestUserAttention:NSInformationalRequest];
	}
}

void glfwFocusWindow(plafWindow* window) {
	@autoreleasepool {
		// Make us the active application
		// HACK: This is here to prevent applications using only hidden windows from
		//       being activated, but should probably not be done every time any
		//       window is shown
		[NSApp activateIgnoringOtherApps:YES];
		[window->nsWindow makeKeyAndOrderFront:nil];
	}
}

void _glfwSetWindowMonitor(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate) {
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
			const NSRect contentRect = NSMakeRect(xpos, _glfwTransformYCocoa(ypos + height - 1), width, height);
			const NSUInteger styleMask = [window->nsWindow styleMask];
			const NSRect frameRect = [NSWindow frameRectForContentRect:contentRect styleMask:styleMask];
			[window->nsWindow setFrame:frameRect display:YES];
		}

		return;
	}

	if (window->monitor)
		releaseMonitor(window);

	_glfwInputWindowMonitor(window, monitor);

	// HACK: Allow the state cached in Cocoa to catch up to reality
	// TODO: Solve this in a less terrible way
	glfwPollEvents();

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
		NSRect contentRect = NSMakeRect(xpos, _glfwTransformYCocoa(ypos + height - 1),
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

IntBool _glfwWindowFocused(plafWindow* window) {
	@autoreleasepool {
	return [window->nsWindow isKeyWindow];
	}
}

IntBool _glfwWindowIconified(plafWindow* window) {
	@autoreleasepool {
	return [window->nsWindow isMiniaturized];
	}
}

IntBool _glfwWindowVisible(plafWindow* window) {
	@autoreleasepool {
	return [window->nsWindow isVisible];
	}
}

IntBool _glfwWindowMaximized(plafWindow* window) {
	@autoreleasepool {

	if (window->resizable)
		return [window->nsWindow isZoomed];
	else
		return false;

	}
}

IntBool _glfwWindowHovered(plafWindow* window) {
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

IntBool _glfwFramebufferTransparent(plafWindow* window) {
	@autoreleasepool {
	return ![window->nsWindow isOpaque] && ![window->nsView isOpaque];
	}
}

void _glfwSetWindowResizable(plafWindow* window, IntBool enabled) {
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

void _glfwSetWindowDecorated(plafWindow* window, IntBool enabled) {
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

void _glfwSetWindowFloating(plafWindow* window, IntBool enabled) {
	@autoreleasepool {
	if (enabled)
		[window->nsWindow setLevel:NSFloatingWindowLevel];
	else
		[window->nsWindow setLevel:NSNormalWindowLevel];
	}
}

void _glfwSetWindowMousePassthrough(plafWindow* window, IntBool enabled) {
	@autoreleasepool {
		[window->nsWindow setIgnoresMouseEvents:enabled];
	}
}

float glfwGetWindowOpacity(plafWindow* window) {
	@autoreleasepool {
	return (float) [window->nsWindow alphaValue];
	}
}

void _glfwSetWindowOpacity(plafWindow* window, float opacity) {
	@autoreleasepool {
	[window->nsWindow setAlphaValue:opacity];
	}
}

void glfwPollEvents(void) {
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

void glfwWaitEvents(void) {
	@autoreleasepool {
		NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:[NSDate distantFuture]
			inMode:NSDefaultRunLoopMode dequeue:YES];
		[NSApp sendEvent:event];
		glfwPollEvents();
	}
}

void _glfwWaitEventsTimeout(double timeout) {
	@autoreleasepool {
		NSDate* date = [NSDate dateWithTimeIntervalSinceNow:timeout];
		NSEvent* event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:date inMode:NSDefaultRunLoopMode
			dequeue:YES];
		if (event) {
			[NSApp sendEvent:event];
		}
		glfwPollEvents();
	}
}

void glfwPostEmptyEvent(void) {
	@autoreleasepool {
		NSEvent* event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSMakePoint(0, 0)
			modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
		[NSApp postEvent:event atStart:YES];
	}
}

void glfwSetCursorMode(plafWindow* window, int mode) {
	@autoreleasepool {

	if (_glfwWindowFocused(window))
		updateCursorMode(window);

	}
}

IntBool _glfwCreateCursor(plafCursor* cursor, const ImageData* image, int xhot, int yhot) {
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

IntBool _glfwCreateStandardCursor(plafCursor* cursor, int shape) {
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
		_glfwInputError("Cocoa: Standard cursor shape unavailable");
		return false;
	}

	[cursor->nsCursor retain];
	return true;

	}
}

void _glfwDestroyCursor(plafCursor* cursor) {
	if (cursor->nsCursor) {
		[cursor->nsCursor release];
		cursor->nsCursor = nil;
	}
}


//////////////////////////////////////////////////////////////////////////
//////                        GLFW native API                       //////
//////////////////////////////////////////////////////////////////////////

id glfwGetCocoaWindow(plafWindow* window) {
	return window->nsWindow;
}

id glfwGetCocoaView(plafWindow* window) {
	return window->nsView;
}

#endif // __APPLE__
