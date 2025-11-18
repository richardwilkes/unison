#if defined(__APPLE__)

#include "platform.h"

#import <QuartzCore/CAMetalLayer.h>

// Hides the cursor if not already hidden
//
static void hideCursor(plafWindow* window)
{
	if (!_glfw.ns.cursorHidden)
	{
		[NSCursor hide];
		_glfw.ns.cursorHidden = true;
	}
}

// Shows the cursor if not already shown
//
static void showCursor(plafWindow* window)
{
	if (_glfw.ns.cursorHidden)
	{
		[NSCursor unhide];
		_glfw.ns.cursorHidden = false;
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
	_glfwSetVideoModeCocoa(window->monitor, &window->videoMode);
	const CGRect bounds = CGDisplayBounds(window->monitor->nsDisplayID);
	const NSRect frame = NSMakeRect(bounds.origin.x,
									_glfwTransformYCocoa(bounds.origin.y + bounds.size.height - 1),
									bounds.size.width,
									bounds.size.height);

	[window->ns.object setFrame:frame display:YES];

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
		mods |= MOD_SHIFT;
	if (flags & NSEventModifierFlagControl)
		mods |= MOD_CONTROL;
	if (flags & NSEventModifierFlagOption)
		mods |= MOD_ALT;
	if (flags & NSEventModifierFlagCommand)
		mods |= MOD_SUPER;
	if (flags & NSEventModifierFlagCapsLock)
		mods |= MOD_CAPS_LOCK;

	return mods;
}

// Translates a macOS keycode to a GLFW keycode
//
static int translateKey(unsigned int key)
{
	if (key >= sizeof(_glfw.ns.keycodes) / sizeof(_glfw.ns.keycodes[0]))
		return KEY_UNKNOWN;

	return _glfw.ns.keycodes[key];
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
	[window->context.nsgl.object update];

	const int maximized = [window->ns.object isZoomed];
	if (window->ns.maximized != maximized)
	{
		window->ns.maximized = maximized;
		_glfwInputWindowMaximize(window, maximized);
	}

	const NSRect contentRect = [window->ns.view frame];
	const NSRect fbRect = [window->ns.view convertRectToBacking:contentRect];

	if (fbRect.size.width != window->ns.fbWidth ||
		fbRect.size.height != window->ns.fbHeight)
	{
		window->ns.fbWidth  = fbRect.size.width;
		window->ns.fbHeight = fbRect.size.height;
		_glfwInputFramebufferSize(window, fbRect.size.width, fbRect.size.height);
	}

	if (contentRect.size.width != window->ns.width ||
		contentRect.size.height != window->ns.height)
	{
		window->ns.width  = contentRect.size.width;
		window->ns.height = contentRect.size.height;
		_glfwInputWindowSize(window, contentRect.size.width, contentRect.size.height);
	}
}

- (void)windowDidMove:(NSNotification *)notification
{
	[window->context.nsgl.object update];

	int x, y;
	_glfwGetWindowPosCocoa(window, &x, &y);
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
	return [window->ns.object isOpaque];
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
	[window->context.nsgl.object update];
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
	const NSRect contentRect = [window->ns.view frame];
	// NOTE: The returned location uses base 0,1 not 0,0
	const NSPoint pos = [event locationInWindow];

	_glfwInputCursorPos(window, pos.x, contentRect.size.height - pos.y);
	window->ns.cursorWarpDeltaX = 0;
	window->ns.cursorWarpDeltaY = 0;
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
	const NSRect contentRect = [window->ns.view frame];
	const NSRect fbRect = [window->ns.view convertRectToBacking:contentRect];
	const float xscale = fbRect.size.width / contentRect.size.width;
	const float yscale = fbRect.size.height / contentRect.size.height;

	if (xscale != window->ns.xscale || yscale != window->ns.yscale)
	{
		// if (window->ns.scaleFramebuffer && window->ns.layer)
		// 	[window->ns.layer setContentsScale:[window->ns.object backingScaleFactor]];

		window->ns.xscale = xscale;
		window->ns.yscale = yscale;
		_glfwInputWindowContentScale(window, xscale, yscale);
	}

	if (fbRect.size.width != window->ns.fbWidth ||
		fbRect.size.height != window->ns.fbHeight)
	{
		window->ns.fbWidth  = fbRect.size.width;
		window->ns.fbHeight = fbRect.size.height;
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
	const NSRect contentRect = [window->ns.view frame];
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
	const NSRect frame = [window->ns.view frame];
	return NSMakeRect(frame.origin.x, frame.origin.y, 0.0, 0.0);
}

- (void)insertText:(id)string replacementRange:(NSRange)replacementRange
{
	NSString* characters;
	NSEvent* event = [NSApp currentEvent];
	const int mods = translateFlags([event modifierFlags]);
	const int plain = !(mods & MOD_SUPER);

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
//
static IntBool createNativeWindow(plafWindow* window,
								   const WindowConfig* wndconfig,
								   const plafFrameBufferCfg* fbconfig)
{
	window->ns.delegate = [[GLFWWindowDelegate alloc] initWithGlfwWindow:window];
	if (window->ns.delegate == nil)
	{
		_glfwInputError(ERR_PLATFORM_ERROR, "Cocoa: Failed to create window delegate");
		return false;
	}

	NSRect contentRect;

	if (window->monitor)
	{
		VideoMode mode;
		int xpos, ypos;

		_glfwGetVideoModeCocoa(window->monitor, &mode);
		_glfwGetMonitorPosCocoa(window->monitor, &xpos, &ypos);

		contentRect = NSMakeRect(xpos, ypos, mode.width, mode.height);
	}
	else
	{
		if (wndconfig->xpos == ANY_POSITION ||
			wndconfig->ypos == ANY_POSITION)
		{
			contentRect = NSMakeRect(0, 0, wndconfig->width, wndconfig->height);
		}
		else
		{
			const int xpos = wndconfig->xpos;
			const int ypos = _glfwTransformYCocoa(wndconfig->ypos + wndconfig->height - 1);
			contentRect = NSMakeRect(xpos, ypos, wndconfig->width, wndconfig->height);
		}
	}

	NSUInteger styleMask = NSWindowStyleMaskMiniaturizable;

	if (window->monitor || !window->decorated)
		styleMask |= NSWindowStyleMaskBorderless;
	else
	{
		styleMask |= (NSWindowStyleMaskTitled | NSWindowStyleMaskClosable);

		if (window->resizable)
			styleMask |= NSWindowStyleMaskResizable;
	}

	window->ns.object = [[GLFWWindow alloc]
		initWithContentRect:contentRect
				  styleMask:styleMask
					backing:NSBackingStoreBuffered
					  defer:NO];

	if (window->ns.object == nil)
	{
		_glfwInputError(ERR_PLATFORM_ERROR, "Cocoa: Failed to create window");
		return false;
	}

	if (window->monitor)
		[window->ns.object setLevel:NSMainMenuWindowLevel + 1];
	else
	{
		if (wndconfig->xpos == ANY_POSITION ||
			wndconfig->ypos == ANY_POSITION)
		{
			[(NSWindow*) window->ns.object center];
			_glfw.ns.cascadePoint =
				NSPointToCGPoint([window->ns.object cascadeTopLeftFromPoint:
								NSPointFromCGPoint(_glfw.ns.cascadePoint)]);
		}

		if (wndconfig->resizable)
		{
			const NSWindowCollectionBehavior behavior =
				NSWindowCollectionBehaviorFullScreenPrimary |
				NSWindowCollectionBehaviorManaged;
			[window->ns.object setCollectionBehavior:behavior];
		}
		else
		{
			const NSWindowCollectionBehavior behavior =
				NSWindowCollectionBehaviorFullScreenNone;
			[window->ns.object setCollectionBehavior:behavior];
		}

		if (wndconfig->floating)
			[window->ns.object setLevel:NSFloatingWindowLevel];

		if (wndconfig->maximized)
			[window->ns.object zoom:nil];
	}

	window->ns.view = [[GLFWContentView alloc] initWithGlfwWindow:window];
	window->ns.scaleFramebuffer = wndconfig->scaleFramebuffer;

	if (fbconfig->transparent)
	{
		[window->ns.object setOpaque:NO];
		[window->ns.object setHasShadow:NO];
		[window->ns.object setBackgroundColor:[NSColor clearColor]];
	}

	[window->ns.object setContentView:window->ns.view];
	[window->ns.object makeFirstResponder:window->ns.view];
	[window->ns.object setTitle:@(window->title)];
	[window->ns.object setDelegate:(id<NSWindowDelegate>)window->ns.delegate];
	[window->ns.object setAcceptsMouseMovedEvents:YES];
	[window->ns.object setRestorable:NO];

	if ([window->ns.object respondsToSelector:@selector(setTabbingMode:)])
		[window->ns.object setTabbingMode:NSWindowTabbingModeDisallowed];

	_glfwGetWindowSizeCocoa(window, &window->ns.width, &window->ns.height);
	_glfwGetFramebufferSizeCocoa(window, &window->ns.fbWidth, &window->ns.fbHeight);

	return true;
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

IntBool _glfwCreateWindowCocoa(plafWindow* window,
								const WindowConfig* wndconfig,
								const plafCtxCfg* ctxconfig,
								const plafFrameBufferCfg* fbconfig)
{
	@autoreleasepool {

	if (!createNativeWindow(window, wndconfig, fbconfig))
		return false;

	if (!_glfwInitNSGL())
		return false;

	if (!_glfwCreateContextNSGL(window, ctxconfig, fbconfig))
		return false;

	if (!_glfwRefreshContextAttribs(window, ctxconfig))
		return false;

	if (wndconfig->mousePassthrough)
		_glfwSetWindowMousePassthroughCocoa(window, true);

	if (window->monitor)
	{
		_glfwShowWindowCocoa(window);
		_glfwFocusWindowCocoa(window);
		acquireMonitor(window);
	}

	return true;

	} // autoreleasepool
}

void _glfwDestroyWindowCocoa(plafWindow* window)
{
	@autoreleasepool {

	[window->ns.object orderOut:nil];

	if (window->monitor)
		releaseMonitor(window);

	if (window->context.destroy)
		window->context.destroy(window);

	[window->ns.object setDelegate:nil];
	[window->ns.delegate release];
	window->ns.delegate = nil;

	[window->ns.view release];
	window->ns.view = nil;

	[window->ns.object close];
	window->ns.object = nil;

	// HACK: Allow Cocoa to catch up before returning
	_glfwPollEventsCocoa();

	} // autoreleasepool
}

void _glfwSetWindowTitleCocoa(plafWindow* window, const char* title)
{
	@autoreleasepool {
	NSString* string = @(title);
	[window->ns.object setTitle:string];
	// HACK: Set the miniwindow title explicitly as setTitle: doesn't update it
	//       if the window lacks NSWindowStyleMaskTitled
	[window->ns.object setMiniwindowTitle:string];
	} // autoreleasepool
}

void _glfwSetWindowIconCocoa(plafWindow* window,
							 int count, const ImageData* images)
{
	_glfwInputError(ERR_FEATURE_UNAVAILABLE, "Cocoa: Regular windows do not have icons on macOS");
}

void _glfwGetWindowPosCocoa(plafWindow* window, int* xpos, int* ypos)
{
	@autoreleasepool {

	const NSRect contentRect =
		[window->ns.object contentRectForFrameRect:[window->ns.object frame]];

	if (xpos)
		*xpos = contentRect.origin.x;
	if (ypos)
		*ypos = _glfwTransformYCocoa(contentRect.origin.y + contentRect.size.height - 1);

	} // autoreleasepool
}

void _glfwSetWindowPosCocoa(plafWindow* window, int x, int y)
{
	@autoreleasepool {

	const NSRect contentRect = [window->ns.view frame];
	const NSRect dummyRect = NSMakeRect(x, _glfwTransformYCocoa(y + contentRect.size.height - 1), 0, 0);
	const NSRect frameRect = [window->ns.object frameRectForContentRect:dummyRect];
	[window->ns.object setFrameOrigin:frameRect.origin];

	} // autoreleasepool
}

void _glfwGetWindowSizeCocoa(plafWindow* window, int* width, int* height)
{
	@autoreleasepool {

	const NSRect contentRect = [window->ns.view frame];

	if (width)
		*width = contentRect.size.width;
	if (height)
		*height = contentRect.size.height;

	} // autoreleasepool
}

void _glfwSetWindowSizeCocoa(plafWindow* window, int width, int height)
{
	@autoreleasepool {

	if (window->monitor)
	{
		if (window->monitor->window == window)
			acquireMonitor(window);
	}
	else
	{
		NSRect contentRect =
			[window->ns.object contentRectForFrameRect:[window->ns.object frame]];
		contentRect.origin.y += contentRect.size.height - height;
		contentRect.size = NSMakeSize(width, height);
		[window->ns.object setFrame:[window->ns.object frameRectForContentRect:contentRect] display:YES];
	}

	} // autoreleasepool
}

void _glfwSetWindowSizeLimitsCocoa(plafWindow* window,
								   int minwidth, int minheight,
								   int maxwidth, int maxheight)
{
	@autoreleasepool {

	if (minwidth == DONT_CARE || minheight == DONT_CARE)
		[window->ns.object setContentMinSize:NSMakeSize(0, 0)];
	else
		[window->ns.object setContentMinSize:NSMakeSize(minwidth, minheight)];

	if (maxwidth == DONT_CARE || maxheight == DONT_CARE)
		[window->ns.object setContentMaxSize:NSMakeSize(DBL_MAX, DBL_MAX)];
	else
		[window->ns.object setContentMaxSize:NSMakeSize(maxwidth, maxheight)];

	} // autoreleasepool
}

void _glfwSetWindowAspectRatioCocoa(plafWindow* window, int numer, int denom)
{
	@autoreleasepool {
	if (numer == DONT_CARE || denom == DONT_CARE)
		[window->ns.object setResizeIncrements:NSMakeSize(1.0, 1.0)];
	else
		[window->ns.object setContentAspectRatio:NSMakeSize(numer, denom)];
	} // autoreleasepool
}

void _glfwGetFramebufferSizeCocoa(plafWindow* window, int* width, int* height)
{
	@autoreleasepool {

	const NSRect contentRect = [window->ns.view frame];
	const NSRect fbRect = [window->ns.view convertRectToBacking:contentRect];

	if (width)
		*width = (int) fbRect.size.width;
	if (height)
		*height = (int) fbRect.size.height;

	} // autoreleasepool
}

void _glfwGetWindowFrameSizeCocoa(plafWindow* window,
								  int* left, int* top,
								  int* right, int* bottom)
{
	@autoreleasepool {

	const NSRect contentRect = [window->ns.view frame];
	const NSRect frameRect = [window->ns.object frameRectForContentRect:contentRect];

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

	} // autoreleasepool
}

void _glfwGetWindowContentScaleCocoa(plafWindow* window,
									 float* xscale, float* yscale)
{
	@autoreleasepool {

	const NSRect points = [window->ns.view frame];
	const NSRect pixels = [window->ns.view convertRectToBacking:points];

	if (xscale)
		*xscale = (float) (pixels.size.width / points.size.width);
	if (yscale)
		*yscale = (float) (pixels.size.height / points.size.height);

	} // autoreleasepool
}

void _glfwIconifyWindowCocoa(plafWindow* window)
{
	@autoreleasepool {
	[window->ns.object miniaturize:nil];
	} // autoreleasepool
}

void _glfwRestoreWindowCocoa(plafWindow* window)
{
	@autoreleasepool {
	if ([window->ns.object isMiniaturized])
		[window->ns.object deminiaturize:nil];
	else if ([window->ns.object isZoomed])
		[window->ns.object zoom:nil];
	} // autoreleasepool
}

void _glfwMaximizeWindowCocoa(plafWindow* window)
{
	@autoreleasepool {
	if (![window->ns.object isZoomed])
		[window->ns.object zoom:nil];
	} // autoreleasepool
}

void _glfwShowWindowCocoa(plafWindow* window)
{
	@autoreleasepool {
	[window->ns.object orderFront:nil];
	} // autoreleasepool
}

void _glfwHideWindowCocoa(plafWindow* window)
{
	@autoreleasepool {
	[window->ns.object orderOut:nil];
	} // autoreleasepool
}

void _glfwRequestWindowAttentionCocoa(plafWindow* window)
{
	@autoreleasepool {
	[NSApp requestUserAttention:NSInformationalRequest];
	} // autoreleasepool
}

void _glfwFocusWindowCocoa(plafWindow* window)
{
	@autoreleasepool {
	// Make us the active application
	// HACK: This is here to prevent applications using only hidden windows from
	//       being activated, but should probably not be done every time any
	//       window is shown
	[NSApp activateIgnoringOtherApps:YES];
	[window->ns.object makeKeyAndOrderFront:nil];
	} // autoreleasepool
}

void _glfwSetWindowMonitorCocoa(plafWindow* window,
								plafMonitor* monitor,
								int xpos, int ypos,
								int width, int height,
								int refreshRate)
{
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
			const NSUInteger styleMask = [window->ns.object styleMask];
			const NSRect frameRect = [NSWindow frameRectForContentRect:contentRect styleMask:styleMask];
			[window->ns.object setFrame:frameRect display:YES];
		}

		return;
	}

	if (window->monitor)
		releaseMonitor(window);

	_glfwInputWindowMonitor(window, monitor);

	// HACK: Allow the state cached in Cocoa to catch up to reality
	// TODO: Solve this in a less terrible way
	_glfwPollEventsCocoa();

	NSUInteger styleMask = [window->ns.object styleMask];

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

	[window->ns.object setStyleMask:styleMask];
	// HACK: Changing the style mask can cause the first responder to be cleared
	[window->ns.object makeFirstResponder:window->ns.view];

	if (window->monitor)
	{
		[window->ns.object setLevel:NSMainMenuWindowLevel + 1];
		[window->ns.object setHasShadow:NO];

		acquireMonitor(window);
	}
	else
	{
		NSRect contentRect = NSMakeRect(xpos, _glfwTransformYCocoa(ypos + height - 1),
										width, height);
		NSRect frameRect = [NSWindow frameRectForContentRect:contentRect styleMask:styleMask];
		[window->ns.object setFrame:frameRect display:YES];

		if (window->numer != DONT_CARE &&
			window->denom != DONT_CARE)
		{
			[window->ns.object setContentAspectRatio:NSMakeSize(window->numer,
																window->denom)];
		}

		if (window->minwidth != DONT_CARE &&
			window->minheight != DONT_CARE)
		{
			[window->ns.object setContentMinSize:NSMakeSize(window->minwidth,
															window->minheight)];
		}

		if (window->maxwidth != DONT_CARE &&
			window->maxheight != DONT_CARE)
		{
			[window->ns.object setContentMaxSize:NSMakeSize(window->maxwidth,
															window->maxheight)];
		}

		if (window->floating)
			[window->ns.object setLevel:NSFloatingWindowLevel];
		else
			[window->ns.object setLevel:NSNormalWindowLevel];

		if (window->resizable)
		{
			const NSWindowCollectionBehavior behavior =
				NSWindowCollectionBehaviorFullScreenPrimary |
				NSWindowCollectionBehaviorManaged;
			[window->ns.object setCollectionBehavior:behavior];
		}
		else
		{
			const NSWindowCollectionBehavior behavior =
				NSWindowCollectionBehaviorFullScreenNone;
			[window->ns.object setCollectionBehavior:behavior];
		}

		[window->ns.object setHasShadow:YES];
		// HACK: Clearing NSWindowStyleMaskTitled resets and disables the window
		//       title property but the miniwindow title property is unaffected
		[window->ns.object setTitle:[window->ns.object miniwindowTitle]];
	}

	} // autoreleasepool
}

IntBool _glfwWindowFocusedCocoa(plafWindow* window)
{
	@autoreleasepool {
	return [window->ns.object isKeyWindow];
	} // autoreleasepool
}

IntBool _glfwWindowIconifiedCocoa(plafWindow* window)
{
	@autoreleasepool {
	return [window->ns.object isMiniaturized];
	} // autoreleasepool
}

IntBool _glfwWindowVisibleCocoa(plafWindow* window)
{
	@autoreleasepool {
	return [window->ns.object isVisible];
	} // autoreleasepool
}

IntBool _glfwWindowMaximizedCocoa(plafWindow* window)
{
	@autoreleasepool {

	if (window->resizable)
		return [window->ns.object isZoomed];
	else
		return false;

	} // autoreleasepool
}

IntBool _glfwWindowHoveredCocoa(plafWindow* window)
{
	@autoreleasepool {

	const NSPoint point = [NSEvent mouseLocation];

	if ([NSWindow windowNumberAtPoint:point belowWindowWithWindowNumber:0] !=
		[window->ns.object windowNumber])
	{
		return false;
	}

	return NSMouseInRect(point,
		[window->ns.object convertRectToScreen:[window->ns.view frame]], NO);

	} // autoreleasepool
}

IntBool _glfwFramebufferTransparentCocoa(plafWindow* window)
{
	@autoreleasepool {
	return ![window->ns.object isOpaque] && ![window->ns.view isOpaque];
	} // autoreleasepool
}

void _glfwSetWindowResizableCocoa(plafWindow* window, IntBool enabled)
{
	@autoreleasepool {

	const NSUInteger styleMask = [window->ns.object styleMask];
	if (enabled)
	{
		[window->ns.object setStyleMask:(styleMask | NSWindowStyleMaskResizable)];
		const NSWindowCollectionBehavior behavior =
			NSWindowCollectionBehaviorFullScreenPrimary |
			NSWindowCollectionBehaviorManaged;
		[window->ns.object setCollectionBehavior:behavior];
	}
	else
	{
		[window->ns.object setStyleMask:(styleMask & ~NSWindowStyleMaskResizable)];
		const NSWindowCollectionBehavior behavior =
			NSWindowCollectionBehaviorFullScreenNone;
		[window->ns.object setCollectionBehavior:behavior];
	}

	} // autoreleasepool
}

void _glfwSetWindowDecoratedCocoa(plafWindow* window, IntBool enabled)
{
	@autoreleasepool {

	NSUInteger styleMask = [window->ns.object styleMask];
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

	[window->ns.object setStyleMask:styleMask];
	[window->ns.object makeFirstResponder:window->ns.view];

	} // autoreleasepool
}

void _glfwSetWindowFloatingCocoa(plafWindow* window, IntBool enabled)
{
	@autoreleasepool {
	if (enabled)
		[window->ns.object setLevel:NSFloatingWindowLevel];
	else
		[window->ns.object setLevel:NSNormalWindowLevel];
	} // autoreleasepool
}

void _glfwSetWindowMousePassthroughCocoa(plafWindow* window, IntBool enabled)
{
	@autoreleasepool {
	[window->ns.object setIgnoresMouseEvents:enabled];
	}
}

float _glfwGetWindowOpacityCocoa(plafWindow* window)
{
	@autoreleasepool {
	return (float) [window->ns.object alphaValue];
	} // autoreleasepool
}

void _glfwSetWindowOpacityCocoa(plafWindow* window, float opacity)
{
	@autoreleasepool {
	[window->ns.object setAlphaValue:opacity];
	} // autoreleasepool
}

void _glfwPollEventsCocoa(void)
{
	@autoreleasepool {

	for (;;)
	{
		NSEvent* event = [NSApp nextEventMatchingMask:NSEventMaskAny
											untilDate:[NSDate distantPast]
											   inMode:NSDefaultRunLoopMode
											  dequeue:YES];
		if (event == nil)
			break;

		[NSApp sendEvent:event];
	}

	} // autoreleasepool
}

void _glfwWaitEventsCocoa(void)
{
	@autoreleasepool {

	// I wanted to pass NO to dequeue:, and rely on PollEvents to
	// dequeue and send.  For reasons not at all clear to me, passing
	// NO to dequeue: causes this method never to return.
	NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny
										untilDate:[NSDate distantFuture]
										   inMode:NSDefaultRunLoopMode
										  dequeue:YES];
	[NSApp sendEvent:event];

	_glfwPollEventsCocoa();

	} // autoreleasepool
}

void _glfwWaitEventsTimeoutCocoa(double timeout)
{
	@autoreleasepool {

	NSDate* date = [NSDate dateWithTimeIntervalSinceNow:timeout];
	NSEvent* event = [NSApp nextEventMatchingMask:NSEventMaskAny
										untilDate:date
										   inMode:NSDefaultRunLoopMode
										  dequeue:YES];
	if (event)
		[NSApp sendEvent:event];

	_glfwPollEventsCocoa();

	} // autoreleasepool
}

void _glfwPostEmptyEventCocoa(void)
{
	@autoreleasepool {

	NSEvent* event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
										location:NSMakePoint(0, 0)
								   modifierFlags:0
									   timestamp:0
									windowNumber:0
										 context:nil
										 subtype:0
										   data1:0
										   data2:0];
	[NSApp postEvent:event atStart:YES];

	} // autoreleasepool
}

void _glfwSetCursorModeCocoa(plafWindow* window, int mode)
{
	@autoreleasepool {

	if (_glfwWindowFocusedCocoa(window))
		updateCursorMode(window);

	} // autoreleasepool
}

int _glfwGetKeyScancodeCocoa(int key)
{
	return _glfw.ns.scancodes[key];
}

IntBool _glfwCreateCursorCocoa(plafCursor* cursor,
								const ImageData* image,
								int xhot, int yhot)
{
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

	} // autoreleasepool
}

IntBool _glfwCreateStandardCursorCocoa(plafCursor* cursor, int shape)
{
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
		_glfwInputError(GLFW_CURSOR_UNAVAILABLE, "Cocoa: Standard cursor shape unavailable");
		return false;
	}

	[cursor->nsCursor retain];
	return true;

	} // autoreleasepool
}

void _glfwDestroyCursorCocoa(plafCursor* cursor)
{
	if (cursor->nsCursor) {
		[cursor->nsCursor release];
		cursor->nsCursor = nil;
	}
}


//////////////////////////////////////////////////////////////////////////
//////                        GLFW native API                       //////
//////////////////////////////////////////////////////////////////////////

id glfwGetCocoaWindow(plafWindow* handle)
{
	plafWindow* window = (plafWindow*) handle;
	return window->ns.object;
}

id glfwGetCocoaView(plafWindow* handle)
{
	plafWindow* window = (plafWindow*) handle;
	return window->ns.view;
}

#endif // __APPLE__
