#if defined(_GLFW_COCOA)

#include "platform.h"
#include <sys/param.h> // For MAXPATHLEN
#include <crt_externs.h> // For _NSGetProgname

// Create key code translation tables.
static void createKeyTables(void) {
	memset(_glfw.ns.keycodes, -1, sizeof(_glfw.ns.keycodes));
	memset(_glfw.ns.scancodes, -1, sizeof(_glfw.ns.scancodes));

	_glfw.ns.keycodes[0x1D] = GLFW_KEY_0;
	_glfw.ns.keycodes[0x12] = GLFW_KEY_1;
	_glfw.ns.keycodes[0x13] = GLFW_KEY_2;
	_glfw.ns.keycodes[0x14] = GLFW_KEY_3;
	_glfw.ns.keycodes[0x15] = GLFW_KEY_4;
	_glfw.ns.keycodes[0x17] = GLFW_KEY_5;
	_glfw.ns.keycodes[0x16] = GLFW_KEY_6;
	_glfw.ns.keycodes[0x1A] = GLFW_KEY_7;
	_glfw.ns.keycodes[0x1C] = GLFW_KEY_8;
	_glfw.ns.keycodes[0x19] = GLFW_KEY_9;
	_glfw.ns.keycodes[0x00] = GLFW_KEY_A;
	_glfw.ns.keycodes[0x0B] = GLFW_KEY_B;
	_glfw.ns.keycodes[0x08] = GLFW_KEY_C;
	_glfw.ns.keycodes[0x02] = GLFW_KEY_D;
	_glfw.ns.keycodes[0x0E] = GLFW_KEY_E;
	_glfw.ns.keycodes[0x03] = GLFW_KEY_F;
	_glfw.ns.keycodes[0x05] = GLFW_KEY_G;
	_glfw.ns.keycodes[0x04] = GLFW_KEY_H;
	_glfw.ns.keycodes[0x22] = GLFW_KEY_I;
	_glfw.ns.keycodes[0x26] = GLFW_KEY_J;
	_glfw.ns.keycodes[0x28] = GLFW_KEY_K;
	_glfw.ns.keycodes[0x25] = GLFW_KEY_L;
	_glfw.ns.keycodes[0x2E] = GLFW_KEY_M;
	_glfw.ns.keycodes[0x2D] = GLFW_KEY_N;
	_glfw.ns.keycodes[0x1F] = GLFW_KEY_O;
	_glfw.ns.keycodes[0x23] = GLFW_KEY_P;
	_glfw.ns.keycodes[0x0C] = GLFW_KEY_Q;
	_glfw.ns.keycodes[0x0F] = GLFW_KEY_R;
	_glfw.ns.keycodes[0x01] = GLFW_KEY_S;
	_glfw.ns.keycodes[0x11] = GLFW_KEY_T;
	_glfw.ns.keycodes[0x20] = GLFW_KEY_U;
	_glfw.ns.keycodes[0x09] = GLFW_KEY_V;
	_glfw.ns.keycodes[0x0D] = GLFW_KEY_W;
	_glfw.ns.keycodes[0x07] = GLFW_KEY_X;
	_glfw.ns.keycodes[0x10] = GLFW_KEY_Y;
	_glfw.ns.keycodes[0x06] = GLFW_KEY_Z;

	_glfw.ns.keycodes[0x27] = GLFW_KEY_APOSTROPHE;
	_glfw.ns.keycodes[0x2A] = GLFW_KEY_BACKSLASH;
	_glfw.ns.keycodes[0x2B] = GLFW_KEY_COMMA;
	_glfw.ns.keycodes[0x18] = GLFW_KEY_EQUAL;
	_glfw.ns.keycodes[0x32] = GLFW_KEY_GRAVE_ACCENT;
	_glfw.ns.keycodes[0x21] = GLFW_KEY_LEFT_BRACKET;
	_glfw.ns.keycodes[0x1B] = GLFW_KEY_MINUS;
	_glfw.ns.keycodes[0x2F] = GLFW_KEY_PERIOD;
	_glfw.ns.keycodes[0x1E] = GLFW_KEY_RIGHT_BRACKET;
	_glfw.ns.keycodes[0x29] = GLFW_KEY_SEMICOLON;
	_glfw.ns.keycodes[0x2C] = GLFW_KEY_SLASH;
	_glfw.ns.keycodes[0x0A] = GLFW_KEY_WORLD_1;

	_glfw.ns.keycodes[0x33] = GLFW_KEY_BACKSPACE;
	_glfw.ns.keycodes[0x39] = GLFW_KEY_CAPS_LOCK;
	_glfw.ns.keycodes[0x75] = GLFW_KEY_DELETE;
	_glfw.ns.keycodes[0x7D] = GLFW_KEY_DOWN;
	_glfw.ns.keycodes[0x77] = GLFW_KEY_END;
	_glfw.ns.keycodes[0x24] = GLFW_KEY_ENTER;
	_glfw.ns.keycodes[0x35] = GLFW_KEY_ESCAPE;
	_glfw.ns.keycodes[0x7A] = GLFW_KEY_F1;
	_glfw.ns.keycodes[0x78] = GLFW_KEY_F2;
	_glfw.ns.keycodes[0x63] = GLFW_KEY_F3;
	_glfw.ns.keycodes[0x76] = GLFW_KEY_F4;
	_glfw.ns.keycodes[0x60] = GLFW_KEY_F5;
	_glfw.ns.keycodes[0x61] = GLFW_KEY_F6;
	_glfw.ns.keycodes[0x62] = GLFW_KEY_F7;
	_glfw.ns.keycodes[0x64] = GLFW_KEY_F8;
	_glfw.ns.keycodes[0x65] = GLFW_KEY_F9;
	_glfw.ns.keycodes[0x6D] = GLFW_KEY_F10;
	_glfw.ns.keycodes[0x67] = GLFW_KEY_F11;
	_glfw.ns.keycodes[0x6F] = GLFW_KEY_F12;
	_glfw.ns.keycodes[0x69] = GLFW_KEY_PRINT_SCREEN;
	_glfw.ns.keycodes[0x6B] = GLFW_KEY_F14;
	_glfw.ns.keycodes[0x71] = GLFW_KEY_F15;
	_glfw.ns.keycodes[0x6A] = GLFW_KEY_F16;
	_glfw.ns.keycodes[0x40] = GLFW_KEY_F17;
	_glfw.ns.keycodes[0x4F] = GLFW_KEY_F18;
	_glfw.ns.keycodes[0x50] = GLFW_KEY_F19;
	_glfw.ns.keycodes[0x5A] = GLFW_KEY_F20;
	_glfw.ns.keycodes[0x73] = GLFW_KEY_HOME;
	_glfw.ns.keycodes[0x72] = GLFW_KEY_INSERT;
	_glfw.ns.keycodes[0x7B] = GLFW_KEY_LEFT;
	_glfw.ns.keycodes[0x3A] = GLFW_KEY_LEFT_ALT;
	_glfw.ns.keycodes[0x3B] = GLFW_KEY_LEFT_CONTROL;
	_glfw.ns.keycodes[0x38] = GLFW_KEY_LEFT_SHIFT;
	_glfw.ns.keycodes[0x37] = GLFW_KEY_LEFT_SUPER;
	_glfw.ns.keycodes[0x6E] = GLFW_KEY_MENU;
	_glfw.ns.keycodes[0x47] = GLFW_KEY_NUM_LOCK;
	_glfw.ns.keycodes[0x79] = GLFW_KEY_PAGE_DOWN;
	_glfw.ns.keycodes[0x74] = GLFW_KEY_PAGE_UP;
	_glfw.ns.keycodes[0x7C] = GLFW_KEY_RIGHT;
	_glfw.ns.keycodes[0x3D] = GLFW_KEY_RIGHT_ALT;
	_glfw.ns.keycodes[0x3E] = GLFW_KEY_RIGHT_CONTROL;
	_glfw.ns.keycodes[0x3C] = GLFW_KEY_RIGHT_SHIFT;
	_glfw.ns.keycodes[0x36] = GLFW_KEY_RIGHT_SUPER;
	_glfw.ns.keycodes[0x31] = GLFW_KEY_SPACE;
	_glfw.ns.keycodes[0x30] = GLFW_KEY_TAB;
	_glfw.ns.keycodes[0x7E] = GLFW_KEY_UP;

	_glfw.ns.keycodes[0x52] = GLFW_KEY_KP_0;
	_glfw.ns.keycodes[0x53] = GLFW_KEY_KP_1;
	_glfw.ns.keycodes[0x54] = GLFW_KEY_KP_2;
	_glfw.ns.keycodes[0x55] = GLFW_KEY_KP_3;
	_glfw.ns.keycodes[0x56] = GLFW_KEY_KP_4;
	_glfw.ns.keycodes[0x57] = GLFW_KEY_KP_5;
	_glfw.ns.keycodes[0x58] = GLFW_KEY_KP_6;
	_glfw.ns.keycodes[0x59] = GLFW_KEY_KP_7;
	_glfw.ns.keycodes[0x5B] = GLFW_KEY_KP_8;
	_glfw.ns.keycodes[0x5C] = GLFW_KEY_KP_9;
	_glfw.ns.keycodes[0x45] = GLFW_KEY_KP_ADD;
	_glfw.ns.keycodes[0x41] = GLFW_KEY_KP_DECIMAL;
	_glfw.ns.keycodes[0x4B] = GLFW_KEY_KP_DIVIDE;
	_glfw.ns.keycodes[0x4C] = GLFW_KEY_KP_ENTER;
	_glfw.ns.keycodes[0x51] = GLFW_KEY_KP_EQUAL;
	_glfw.ns.keycodes[0x43] = GLFW_KEY_KP_MULTIPLY;
	_glfw.ns.keycodes[0x4E] = GLFW_KEY_KP_SUBTRACT;

	for (int scancode = 0;  scancode < 256;  scancode++) {
		// Store the reverse translation for faster key name lookup
		if (_glfw.ns.keycodes[scancode] >= 0) {
			_glfw.ns.scancodes[_glfw.ns.keycodes[scancode]] = scancode;
		}
	}
}

// Retrieve Unicode data for the current keyboard layout.
static void updateUnicodeData(void) {
	if (_glfw.ns.inputSource) {
		CFRelease(_glfw.ns.inputSource);
		_glfw.ns.inputSource = NULL;
		_glfw.ns.unicodeData = nil;
	}
	_glfw.ns.inputSource = TISCopyCurrentKeyboardLayoutInputSource();
	if (_glfw.ns.inputSource) {
		_glfw.ns.unicodeData = TISGetInputSourceProperty(_glfw.ns.inputSource, kTISPropertyUnicodeKeyLayoutData);
	}
}

// Load HIToolbox.framework and the TIS symbols we need from it.
// This works only because Cocoa has already loaded it properly.
static ErrorResponse* initializeTIS(void) {
	_glfw.ns.tis.bundle = CFBundleGetBundleWithIdentifier(CFSTR("com.apple.HIToolbox"));
	if (!_glfw.ns.tis.bundle) {
		return createErrorResponse(GLFW_PLATFORM_ERROR, "Failed to load HIToolbox.framework");
	}
	CFStringRef* kPropertyUnicodeKeyLayoutData = CFBundleGetDataPointerForName(_glfw.ns.tis.bundle,
		CFSTR("kTISPropertyUnicodeKeyLayoutData"));
	_glfw.ns.tis.CopyCurrentKeyboardLayoutInputSource = CFBundleGetFunctionPointerForName(_glfw.ns.tis.bundle,
		CFSTR("TISCopyCurrentKeyboardLayoutInputSource"));
	_glfw.ns.tis.GetInputSourceProperty = CFBundleGetFunctionPointerForName(_glfw.ns.tis.bundle,
		CFSTR("TISGetInputSourceProperty"));
	_glfw.ns.tis.GetKbdType = CFBundleGetFunctionPointerForName(_glfw.ns.tis.bundle, CFSTR("LMGetKbdType"));
	if (!kPropertyUnicodeKeyLayoutData ||
		!TISCopyCurrentKeyboardLayoutInputSource ||
		!TISGetInputSourceProperty ||
		!LMGetKbdType) {
		return createErrorResponse(GLFW_PLATFORM_ERROR, "Failed to load TIS API symbols");
	}
	_glfw.ns.tis.kPropertyUnicodeKeyLayoutData = *kPropertyUnicodeKeyLayoutData;
	updateUnicodeData();
	return NULL;
}

@interface GLFWHelper : NSObject
@end

@implementation GLFWHelper

- (void)selectedKeyboardInputSourceChanged:(NSObject* )object {
    updateUnicodeData();
}

- (void)doNothing:(id)object {
}

@end // GLFWHelper

@interface GLFWApplicationDelegate : NSObject <NSApplicationDelegate>
@end

@implementation GLFWApplicationDelegate

- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
    for (_GLFWwindow* window = _glfw.windowListHead;  window;  window = window->next) {
        _glfwInputWindowCloseRequest(window);
	}
    return NSTerminateCancel;
}

- (void)applicationDidChangeScreenParameters:(NSNotification *) notification {
    for (_GLFWwindow* window = _glfw.windowListHead;  window;  window = window->next) {
		[window->context.nsgl.object update];
    }
    _glfwPollMonitorsCocoa();
}

- (void)applicationWillFinishLaunching:(NSNotification *)notification {
}

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    _glfwPostEmptyEventCocoa();
    [NSApp stop:nil];
}

- (void)applicationDidHide:(NSNotification *)notification {
    for (int i = 0;  i < _glfw.monitorCount;  i++) {
        _glfwRestoreVideoModeCocoa(_glfw.monitors[i]);
	}
}

@end // GLFWApplicationDelegate

ErrorResponse* platformInit(_GLFWplatform* platform) {
	platform->getCursorPos = _glfwGetCursorPosCocoa;
	platform->setCursorPos = _glfwSetCursorPosCocoa;
	platform->setCursorMode = _glfwSetCursorModeCocoa;
	platform->setRawMouseMotion = _glfwSetRawMouseMotionCocoa;
	platform->rawMouseMotionSupported = _glfwRawMouseMotionSupportedCocoa;
	platform->createCursor = _glfwCreateCursorCocoa;
	platform->createStandardCursor = _glfwCreateStandardCursorCocoa;
	platform->destroyCursor = _glfwDestroyCursorCocoa;
	platform->setCursor = _glfwSetCursorCocoa;
	platform->getScancodeName = _glfwGetScancodeNameCocoa;
	platform->getKeyScancode = _glfwGetKeyScancodeCocoa;
	platform->freeMonitor = _glfwFreeMonitorCocoa;
	platform->getMonitorPos = _glfwGetMonitorPosCocoa;
	platform->getMonitorContentScale = _glfwGetMonitorContentScaleCocoa;
	platform->getMonitorWorkarea = _glfwGetMonitorWorkareaCocoa;
	platform->getVideoModes = _glfwGetVideoModesCocoa;
	platform->getVideoMode = _glfwGetVideoModeCocoa;
	platform->getGammaRamp = _glfwGetGammaRampCocoa;
	platform->setGammaRamp = _glfwSetGammaRampCocoa;
	platform->createWindow = _glfwCreateWindowCocoa;
	platform->destroyWindow = _glfwDestroyWindowCocoa;
	platform->setWindowTitle = _glfwSetWindowTitleCocoa;
	platform->setWindowIcon = _glfwSetWindowIconCocoa;
	platform->getWindowPos = _glfwGetWindowPosCocoa;
	platform->setWindowPos = _glfwSetWindowPosCocoa;
	platform->getWindowSize = _glfwGetWindowSizeCocoa;
	platform->setWindowSize = _glfwSetWindowSizeCocoa;
	platform->setWindowSizeLimits = _glfwSetWindowSizeLimitsCocoa;
	platform->setWindowAspectRatio = _glfwSetWindowAspectRatioCocoa;
	platform->getFramebufferSize = _glfwGetFramebufferSizeCocoa;
	platform->getWindowFrameSize = _glfwGetWindowFrameSizeCocoa;
	platform->getWindowContentScale = _glfwGetWindowContentScaleCocoa;
	platform->iconifyWindow = _glfwIconifyWindowCocoa;
	platform->restoreWindow = _glfwRestoreWindowCocoa;
	platform->maximizeWindow = _glfwMaximizeWindowCocoa;
	platform->showWindow = _glfwShowWindowCocoa;
	platform->hideWindow = _glfwHideWindowCocoa;
	platform->requestWindowAttention = _glfwRequestWindowAttentionCocoa;
	platform->focusWindow = _glfwFocusWindowCocoa;
	platform->setWindowMonitor = _glfwSetWindowMonitorCocoa;
	platform->windowFocused = _glfwWindowFocusedCocoa;
	platform->windowIconified = _glfwWindowIconifiedCocoa;
	platform->windowVisible = _glfwWindowVisibleCocoa;
	platform->windowMaximized = _glfwWindowMaximizedCocoa;
	platform->windowHovered = _glfwWindowHoveredCocoa;
	platform->framebufferTransparent = _glfwFramebufferTransparentCocoa;
	platform->getWindowOpacity = _glfwGetWindowOpacityCocoa;
	platform->setWindowResizable = _glfwSetWindowResizableCocoa;
	platform->setWindowDecorated = _glfwSetWindowDecoratedCocoa;
	platform->setWindowFloating = _glfwSetWindowFloatingCocoa;
	platform->setWindowOpacity = _glfwSetWindowOpacityCocoa;
	platform->setWindowMousePassthrough = _glfwSetWindowMousePassthroughCocoa;
	platform->pollEvents = _glfwPollEventsCocoa;
	platform->waitEvents = _glfwWaitEventsCocoa;
	platform->waitEventsTimeout = _glfwWaitEventsTimeoutCocoa;
	platform->postEmptyEvent = _glfwPostEmptyEventCocoa;

	@autoreleasepool {
		_glfw.ns.helper = [[GLFWHelper alloc] init];

		[NSThread detachNewThreadSelector:@selector(doNothing:) toTarget:_glfw.ns.helper withObject:nil];
		[NSApplication sharedApplication];

		_glfw.ns.delegate = [[GLFWApplicationDelegate alloc] init];
		if (_glfw.ns.delegate == nil) {
			_terminate();
			return createErrorResponse(GLFW_PLATFORM_ERROR, "Failed to create application delegate");
		}

		[NSApp setDelegate:_glfw.ns.delegate];

		NSEvent* (^block)(NSEvent*) = ^ NSEvent* (NSEvent* event) {
			if ([event modifierFlags] & NSEventModifierFlagCommand) {
				[[NSApp keyWindow] sendEvent:event];
			}
			return event;
		};

		_glfw.ns.keyUpMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyUp handler:block];

		// Press and Hold prevents some keys from emitting repeated characters
		NSDictionary* defaults = @{@"ApplePressAndHoldEnabled":@NO};
		[[NSUserDefaults standardUserDefaults] registerDefaults:defaults];

		[[NSNotificationCenter defaultCenter] addObserver:_glfw.ns.helper
			selector:@selector(selectedKeyboardInputSourceChanged:)
			name:NSTextInputContextKeyboardSelectionDidChangeNotification object:nil];

		createKeyTables();

		_glfw.ns.eventSource = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
		if (!_glfw.ns.eventSource) {
			_terminate();
			return createErrorResponse(GLFW_PLATFORM_ERROR, "Failed to create event source");
		}

		CGEventSourceSetLocalEventsSuppressionInterval(_glfw.ns.eventSource, 0.0);

		ErrorResponse* errRsp = initializeTIS();
		if (errRsp != NULL) {
			_terminate();
			return errRsp;
		}

		_glfwPollMonitorsCocoa();

		if (![[NSRunningApplication currentApplication] isFinishedLaunching]) {
			[NSApp run];
		}

		[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
		return NULL;
    } // autoreleasepool
}

void platformTerminate(void) {
	@autoreleasepool {
		if (_glfw.ns.inputSource) {
			CFRelease(_glfw.ns.inputSource);
			_glfw.ns.inputSource = NULL;
			_glfw.ns.unicodeData = nil;
		}
		if (_glfw.ns.eventSource) {
			CFRelease(_glfw.ns.eventSource);
			_glfw.ns.eventSource = NULL;
		}
		if (_glfw.ns.delegate) {
			[NSApp setDelegate:nil];
			[_glfw.ns.delegate release];
			_glfw.ns.delegate = nil;
		}
		if (_glfw.ns.helper) {
			[[NSNotificationCenter defaultCenter] removeObserver:_glfw.ns.helper
				name:NSTextInputContextKeyboardSelectionDidChangeNotification object:nil];
			[[NSNotificationCenter defaultCenter] removeObserver:_glfw.ns.helper];
			[_glfw.ns.helper release];
			_glfw.ns.helper = nil;
		}
		if (_glfw.ns.keyUpMonitor) {
			[NSEvent removeMonitor:_glfw.ns.keyUpMonitor];
		}
		_glfwTerminateNSGL();
	} // autoreleasepool
}

#endif // _GLFW_COCOA
