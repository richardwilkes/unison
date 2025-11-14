#if defined(PLATFORM_DARWIN)

#include "platform.h"
#include <sys/param.h> // For MAXPATHLEN
#include <crt_externs.h> // For _NSGetProgname

// Create key code translation tables.
static void createKeyTables(void) {
	memset(_glfw.ns.keycodes, -1, sizeof(_glfw.ns.keycodes));
	memset(_glfw.ns.scancodes, -1, sizeof(_glfw.ns.scancodes));

	_glfw.ns.keycodes[0x1D] = KEY_0;
	_glfw.ns.keycodes[0x12] = KEY_1;
	_glfw.ns.keycodes[0x13] = KEY_2;
	_glfw.ns.keycodes[0x14] = KEY_3;
	_glfw.ns.keycodes[0x15] = KEY_4;
	_glfw.ns.keycodes[0x17] = KEY_5;
	_glfw.ns.keycodes[0x16] = KEY_6;
	_glfw.ns.keycodes[0x1A] = KEY_7;
	_glfw.ns.keycodes[0x1C] = KEY_8;
	_glfw.ns.keycodes[0x19] = KEY_9;
	_glfw.ns.keycodes[0x00] = KEY_A;
	_glfw.ns.keycodes[0x0B] = KEY_B;
	_glfw.ns.keycodes[0x08] = KEY_C;
	_glfw.ns.keycodes[0x02] = KEY_D;
	_glfw.ns.keycodes[0x0E] = KEY_E;
	_glfw.ns.keycodes[0x03] = KEY_F;
	_glfw.ns.keycodes[0x05] = KEY_G;
	_glfw.ns.keycodes[0x04] = KEY_H;
	_glfw.ns.keycodes[0x22] = KEY_I;
	_glfw.ns.keycodes[0x26] = KEY_J;
	_glfw.ns.keycodes[0x28] = KEY_K;
	_glfw.ns.keycodes[0x25] = KEY_L;
	_glfw.ns.keycodes[0x2E] = KEY_M;
	_glfw.ns.keycodes[0x2D] = KEY_N;
	_glfw.ns.keycodes[0x1F] = KEY_O;
	_glfw.ns.keycodes[0x23] = KEY_P;
	_glfw.ns.keycodes[0x0C] = KEY_Q;
	_glfw.ns.keycodes[0x0F] = KEY_R;
	_glfw.ns.keycodes[0x01] = KEY_S;
	_glfw.ns.keycodes[0x11] = KEY_T;
	_glfw.ns.keycodes[0x20] = KEY_U;
	_glfw.ns.keycodes[0x09] = KEY_V;
	_glfw.ns.keycodes[0x0D] = KEY_W;
	_glfw.ns.keycodes[0x07] = KEY_X;
	_glfw.ns.keycodes[0x10] = KEY_Y;
	_glfw.ns.keycodes[0x06] = KEY_Z;

	_glfw.ns.keycodes[0x27] = KEY_APOSTROPHE;
	_glfw.ns.keycodes[0x2A] = KEY_BACKSLASH;
	_glfw.ns.keycodes[0x2B] = KEY_COMMA;
	_glfw.ns.keycodes[0x18] = KEY_EQUAL;
	_glfw.ns.keycodes[0x32] = KEY_GRAVE_ACCENT;
	_glfw.ns.keycodes[0x21] = KEY_LEFT_BRACKET;
	_glfw.ns.keycodes[0x1B] = KEY_MINUS;
	_glfw.ns.keycodes[0x2F] = KEY_PERIOD;
	_glfw.ns.keycodes[0x1E] = KEY_RIGHT_BRACKET;
	_glfw.ns.keycodes[0x29] = KEY_SEMICOLON;
	_glfw.ns.keycodes[0x2C] = KEY_SLASH;
	_glfw.ns.keycodes[0x0A] = KEY_WORLD_1;

	_glfw.ns.keycodes[0x33] = KEY_BACKSPACE;
	_glfw.ns.keycodes[0x39] = KEY_CAPS_LOCK;
	_glfw.ns.keycodes[0x75] = KEY_DELETE;
	_glfw.ns.keycodes[0x7D] = KEY_DOWN;
	_glfw.ns.keycodes[0x77] = KEY_END;
	_glfw.ns.keycodes[0x24] = KEY_ENTER;
	_glfw.ns.keycodes[0x35] = KEY_ESCAPE;
	_glfw.ns.keycodes[0x7A] = KEY_F1;
	_glfw.ns.keycodes[0x78] = KEY_F2;
	_glfw.ns.keycodes[0x63] = KEY_F3;
	_glfw.ns.keycodes[0x76] = KEY_F4;
	_glfw.ns.keycodes[0x60] = KEY_F5;
	_glfw.ns.keycodes[0x61] = KEY_F6;
	_glfw.ns.keycodes[0x62] = KEY_F7;
	_glfw.ns.keycodes[0x64] = KEY_F8;
	_glfw.ns.keycodes[0x65] = KEY_F9;
	_glfw.ns.keycodes[0x6D] = KEY_F10;
	_glfw.ns.keycodes[0x67] = KEY_F11;
	_glfw.ns.keycodes[0x6F] = KEY_F12;
	_glfw.ns.keycodes[0x69] = KEY_PRINT_SCREEN;
	_glfw.ns.keycodes[0x6B] = KEY_F14;
	_glfw.ns.keycodes[0x71] = KEY_F15;
	_glfw.ns.keycodes[0x6A] = KEY_F16;
	_glfw.ns.keycodes[0x40] = KEY_F17;
	_glfw.ns.keycodes[0x4F] = KEY_F18;
	_glfw.ns.keycodes[0x50] = KEY_F19;
	_glfw.ns.keycodes[0x5A] = KEY_F20;
	_glfw.ns.keycodes[0x73] = KEY_HOME;
	_glfw.ns.keycodes[0x72] = KEY_INSERT;
	_glfw.ns.keycodes[0x7B] = KEY_LEFT;
	_glfw.ns.keycodes[0x3A] = KEY_LEFT_ALT;
	_glfw.ns.keycodes[0x3B] = KEY_LEFT_CONTROL;
	_glfw.ns.keycodes[0x38] = KEY_LEFT_SHIFT;
	_glfw.ns.keycodes[0x37] = KEY_LEFT_SUPER;
	_glfw.ns.keycodes[0x6E] = KEY_MENU;
	_glfw.ns.keycodes[0x47] = KEY_NUM_LOCK;
	_glfw.ns.keycodes[0x79] = KEY_PAGE_DOWN;
	_glfw.ns.keycodes[0x74] = KEY_PAGE_UP;
	_glfw.ns.keycodes[0x7C] = KEY_RIGHT;
	_glfw.ns.keycodes[0x3D] = KEY_RIGHT_ALT;
	_glfw.ns.keycodes[0x3E] = KEY_RIGHT_CONTROL;
	_glfw.ns.keycodes[0x3C] = KEY_RIGHT_SHIFT;
	_glfw.ns.keycodes[0x36] = KEY_RIGHT_SUPER;
	_glfw.ns.keycodes[0x31] = KEY_SPACE;
	_glfw.ns.keycodes[0x30] = KEY_TAB;
	_glfw.ns.keycodes[0x7E] = KEY_UP;

	_glfw.ns.keycodes[0x52] = KEY_KP_0;
	_glfw.ns.keycodes[0x53] = KEY_KP_1;
	_glfw.ns.keycodes[0x54] = KEY_KP_2;
	_glfw.ns.keycodes[0x55] = KEY_KP_3;
	_glfw.ns.keycodes[0x56] = KEY_KP_4;
	_glfw.ns.keycodes[0x57] = KEY_KP_5;
	_glfw.ns.keycodes[0x58] = KEY_KP_6;
	_glfw.ns.keycodes[0x59] = KEY_KP_7;
	_glfw.ns.keycodes[0x5B] = KEY_KP_8;
	_glfw.ns.keycodes[0x5C] = KEY_KP_9;
	_glfw.ns.keycodes[0x45] = KEY_KP_ADD;
	_glfw.ns.keycodes[0x41] = KEY_KP_DECIMAL;
	_glfw.ns.keycodes[0x4B] = KEY_KP_DIVIDE;
	_glfw.ns.keycodes[0x4C] = KEY_KP_ENTER;
	_glfw.ns.keycodes[0x51] = KEY_KP_EQUAL;
	_glfw.ns.keycodes[0x43] = KEY_KP_MULTIPLY;
	_glfw.ns.keycodes[0x4E] = KEY_KP_SUBTRACT;

	for (int scancode = 0;  scancode < 256;  scancode++) {
		// Store the reverse translation for faster key name lookup
		if (_glfw.ns.keycodes[scancode] >= 0) {
			_glfw.ns.scancodes[_glfw.ns.keycodes[scancode]] = scancode;
		}
	}
}

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
		[NSApplication sharedApplication];

		_glfw.ns.delegate = [[GLFWApplicationDelegate alloc] init];
		if (_glfw.ns.delegate == nil) {
			_terminate();
			return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to create application delegate");
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

		createKeyTables();

		_glfw.ns.eventSource = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
		if (!_glfw.ns.eventSource) {
			_terminate();
			return createErrorResponse(ERR_PLATFORM_ERROR, "Failed to create event source");
		}

		CGEventSourceSetLocalEventsSuppressionInterval(_glfw.ns.eventSource, 0.0);

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
		if (_glfw.ns.eventSource) {
			CFRelease(_glfw.ns.eventSource);
			_glfw.ns.eventSource = NULL;
		}
		if (_glfw.ns.delegate) {
			[NSApp setDelegate:nil];
			[_glfw.ns.delegate release];
			_glfw.ns.delegate = nil;
		}
		if (_glfw.ns.keyUpMonitor) {
			[NSEvent removeMonitor:_glfw.ns.keyUpMonitor];
		}
		_glfwTerminateNSGL();
	} // autoreleasepool
}

#endif // PLATFORM_DARWIN
