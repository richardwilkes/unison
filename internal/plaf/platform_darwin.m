#if defined(__APPLE__)

#include "platform.h"
#include <sys/param.h> // For MAXPATHLEN
#include <crt_externs.h> // For _NSGetProgname

// Create key code translation tables.
static void createKeyTables(void) {
	memset(_glfw.keyCodes, -1, sizeof(_glfw.keyCodes));
	memset(_glfw.scanCodes, -1, sizeof(_glfw.scanCodes));

	_glfw.keyCodes[0x1D] = KEY_0;
	_glfw.keyCodes[0x12] = KEY_1;
	_glfw.keyCodes[0x13] = KEY_2;
	_glfw.keyCodes[0x14] = KEY_3;
	_glfw.keyCodes[0x15] = KEY_4;
	_glfw.keyCodes[0x17] = KEY_5;
	_glfw.keyCodes[0x16] = KEY_6;
	_glfw.keyCodes[0x1A] = KEY_7;
	_glfw.keyCodes[0x1C] = KEY_8;
	_glfw.keyCodes[0x19] = KEY_9;
	_glfw.keyCodes[0x00] = KEY_A;
	_glfw.keyCodes[0x0B] = KEY_B;
	_glfw.keyCodes[0x08] = KEY_C;
	_glfw.keyCodes[0x02] = KEY_D;
	_glfw.keyCodes[0x0E] = KEY_E;
	_glfw.keyCodes[0x03] = KEY_F;
	_glfw.keyCodes[0x05] = KEY_G;
	_glfw.keyCodes[0x04] = KEY_H;
	_glfw.keyCodes[0x22] = KEY_I;
	_glfw.keyCodes[0x26] = KEY_J;
	_glfw.keyCodes[0x28] = KEY_K;
	_glfw.keyCodes[0x25] = KEY_L;
	_glfw.keyCodes[0x2E] = KEY_M;
	_glfw.keyCodes[0x2D] = KEY_N;
	_glfw.keyCodes[0x1F] = KEY_O;
	_glfw.keyCodes[0x23] = KEY_P;
	_glfw.keyCodes[0x0C] = KEY_Q;
	_glfw.keyCodes[0x0F] = KEY_R;
	_glfw.keyCodes[0x01] = KEY_S;
	_glfw.keyCodes[0x11] = KEY_T;
	_glfw.keyCodes[0x20] = KEY_U;
	_glfw.keyCodes[0x09] = KEY_V;
	_glfw.keyCodes[0x0D] = KEY_W;
	_glfw.keyCodes[0x07] = KEY_X;
	_glfw.keyCodes[0x10] = KEY_Y;
	_glfw.keyCodes[0x06] = KEY_Z;

	_glfw.keyCodes[0x27] = KEY_APOSTROPHE;
	_glfw.keyCodes[0x2A] = KEY_BACKSLASH;
	_glfw.keyCodes[0x2B] = KEY_COMMA;
	_glfw.keyCodes[0x18] = KEY_EQUAL;
	_glfw.keyCodes[0x32] = KEY_GRAVE_ACCENT;
	_glfw.keyCodes[0x21] = KEY_LEFT_BRACKET;
	_glfw.keyCodes[0x1B] = KEY_MINUS;
	_glfw.keyCodes[0x2F] = KEY_PERIOD;
	_glfw.keyCodes[0x1E] = KEY_RIGHT_BRACKET;
	_glfw.keyCodes[0x29] = KEY_SEMICOLON;
	_glfw.keyCodes[0x2C] = KEY_SLASH;
	_glfw.keyCodes[0x0A] = KEY_WORLD_1;

	_glfw.keyCodes[0x33] = KEY_BACKSPACE;
	_glfw.keyCodes[0x39] = KEY_CAPS_LOCK;
	_glfw.keyCodes[0x75] = KEY_DELETE;
	_glfw.keyCodes[0x7D] = KEY_DOWN;
	_glfw.keyCodes[0x77] = KEY_END;
	_glfw.keyCodes[0x24] = KEY_ENTER;
	_glfw.keyCodes[0x35] = KEY_ESCAPE;
	_glfw.keyCodes[0x7A] = KEY_F1;
	_glfw.keyCodes[0x78] = KEY_F2;
	_glfw.keyCodes[0x63] = KEY_F3;
	_glfw.keyCodes[0x76] = KEY_F4;
	_glfw.keyCodes[0x60] = KEY_F5;
	_glfw.keyCodes[0x61] = KEY_F6;
	_glfw.keyCodes[0x62] = KEY_F7;
	_glfw.keyCodes[0x64] = KEY_F8;
	_glfw.keyCodes[0x65] = KEY_F9;
	_glfw.keyCodes[0x6D] = KEY_F10;
	_glfw.keyCodes[0x67] = KEY_F11;
	_glfw.keyCodes[0x6F] = KEY_F12;
	_glfw.keyCodes[0x69] = KEY_PRINT_SCREEN;
	_glfw.keyCodes[0x6B] = KEY_F14;
	_glfw.keyCodes[0x71] = KEY_F15;
	_glfw.keyCodes[0x6A] = KEY_F16;
	_glfw.keyCodes[0x40] = KEY_F17;
	_glfw.keyCodes[0x4F] = KEY_F18;
	_glfw.keyCodes[0x50] = KEY_F19;
	_glfw.keyCodes[0x5A] = KEY_F20;
	_glfw.keyCodes[0x73] = KEY_HOME;
	_glfw.keyCodes[0x72] = KEY_INSERT;
	_glfw.keyCodes[0x7B] = KEY_LEFT;
	_glfw.keyCodes[0x3A] = KEY_LEFT_ALT;
	_glfw.keyCodes[0x3B] = KEY_LEFT_CONTROL;
	_glfw.keyCodes[0x38] = KEY_LEFT_SHIFT;
	_glfw.keyCodes[0x37] = KEY_LEFT_SUPER;
	_glfw.keyCodes[0x6E] = KEY_MENU;
	_glfw.keyCodes[0x47] = KEY_NUM_LOCK;
	_glfw.keyCodes[0x79] = KEY_PAGE_DOWN;
	_glfw.keyCodes[0x74] = KEY_PAGE_UP;
	_glfw.keyCodes[0x7C] = KEY_RIGHT;
	_glfw.keyCodes[0x3D] = KEY_RIGHT_ALT;
	_glfw.keyCodes[0x3E] = KEY_RIGHT_CONTROL;
	_glfw.keyCodes[0x3C] = KEY_RIGHT_SHIFT;
	_glfw.keyCodes[0x36] = KEY_RIGHT_SUPER;
	_glfw.keyCodes[0x31] = KEY_SPACE;
	_glfw.keyCodes[0x30] = KEY_TAB;
	_glfw.keyCodes[0x7E] = KEY_UP;

	_glfw.keyCodes[0x52] = KEY_KP_0;
	_glfw.keyCodes[0x53] = KEY_KP_1;
	_glfw.keyCodes[0x54] = KEY_KP_2;
	_glfw.keyCodes[0x55] = KEY_KP_3;
	_glfw.keyCodes[0x56] = KEY_KP_4;
	_glfw.keyCodes[0x57] = KEY_KP_5;
	_glfw.keyCodes[0x58] = KEY_KP_6;
	_glfw.keyCodes[0x59] = KEY_KP_7;
	_glfw.keyCodes[0x5B] = KEY_KP_8;
	_glfw.keyCodes[0x5C] = KEY_KP_9;
	_glfw.keyCodes[0x45] = KEY_KP_ADD;
	_glfw.keyCodes[0x41] = KEY_KP_DECIMAL;
	_glfw.keyCodes[0x4B] = KEY_KP_DIVIDE;
	_glfw.keyCodes[0x4C] = KEY_KP_ENTER;
	_glfw.keyCodes[0x51] = KEY_KP_EQUAL;
	_glfw.keyCodes[0x43] = KEY_KP_MULTIPLY;
	_glfw.keyCodes[0x4E] = KEY_KP_SUBTRACT;

	for (int scancode = 0;  scancode < MAX_KEY_CODES;  scancode++) {
		// Store the reverse translation for faster key name lookup
		if (_glfw.keyCodes[scancode] >= 0) {
			_glfw.scanCodes[_glfw.keyCodes[scancode]] = scancode;
		}
	}
}

@interface GLFWApplicationDelegate : NSObject <NSApplicationDelegate>
@end

@implementation GLFWApplicationDelegate

- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
    for (plafWindow* window = _glfw.windowListHead;  window;  window = window->next) {
        _glfwInputWindowCloseRequest(window);
	}
    return NSTerminateCancel;
}

- (void)applicationDidChangeScreenParameters:(NSNotification *) notification {
    for (plafWindow* window = _glfw.windowListHead;  window;  window = window->next) {
		[window->context.nsglCtx update];
    }
    _glfwPollMonitorsCocoa();
}

- (void)applicationWillFinishLaunching:(NSNotification *)notification {
}

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    glfwPostEmptyEvent();
    [NSApp stop:nil];
}

- (void)applicationDidHide:(NSNotification *)notification {
    for (int i = 0;  i < _glfw.monitorCount;  i++) {
        _glfwRestoreVideoModeCocoa(_glfw.monitors[i]);
	}
}

@end // GLFWApplicationDelegate

ErrorResponse* platformInit(void) {
	@autoreleasepool {
		[NSApplication sharedApplication];

		_glfw.nsDelegate = [[GLFWApplicationDelegate alloc] init];
		if (_glfw.nsDelegate == nil) {
			_terminate();
			return createErrorResponse("Failed to create application delegate");
		}

		[NSApp setDelegate:_glfw.nsDelegate];

		NSEvent* (^block)(NSEvent*) = ^ NSEvent* (NSEvent* event) {
			if ([event modifierFlags] & NSEventModifierFlagCommand) {
				[[NSApp keyWindow] sendEvent:event];
			}
			return event;
		};

		_glfw.nsKeyUpMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyUp handler:block];

		// Press and Hold prevents some keys from emitting repeated characters
		NSDictionary* defaults = @{@"ApplePressAndHoldEnabled":@NO};
		[[NSUserDefaults standardUserDefaults] registerDefaults:defaults];

		createKeyTables();

		_glfw.nsEventSource = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
		if (!_glfw.nsEventSource) {
			_terminate();
			return createErrorResponse("Failed to create event source");
		}

		CGEventSourceSetLocalEventsSuppressionInterval(_glfw.nsEventSource, 0.0);

		_glfwPollMonitorsCocoa();

		if (![[NSRunningApplication currentApplication] isFinishedLaunching]) {
			[NSApp run];
		}

		[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
		return NULL;
    }
}

void platformTerminate(void) {
	@autoreleasepool {
		if (_glfw.nsEventSource) {
			CFRelease(_glfw.nsEventSource);
			_glfw.nsEventSource = NULL;
		}
		if (_glfw.nsDelegate) {
			[NSApp setDelegate:nil];
			[_glfw.nsDelegate release];
			_glfw.nsDelegate = nil;
		}
		if (_glfw.nsKeyUpMonitor) {
			[NSEvent removeMonitor:_glfw.nsKeyUpMonitor];
		}
		_glfwTerminateNSGL();
	}
}

#endif // __APPLE__
