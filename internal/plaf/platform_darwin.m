#if defined(__APPLE__)

#include "platform.h"
#include <sys/param.h> // For MAXPATHLEN
#include <crt_externs.h> // For _NSGetProgname

// Create key code translation tables.
static void createKeyTables(void) {
	memset(_plaf.keyCodes, -1, sizeof(_plaf.keyCodes));
	memset(_plaf.scanCodes, -1, sizeof(_plaf.scanCodes));

	_plaf.keyCodes[0x1D] = KEY_0;
	_plaf.keyCodes[0x12] = KEY_1;
	_plaf.keyCodes[0x13] = KEY_2;
	_plaf.keyCodes[0x14] = KEY_3;
	_plaf.keyCodes[0x15] = KEY_4;
	_plaf.keyCodes[0x17] = KEY_5;
	_plaf.keyCodes[0x16] = KEY_6;
	_plaf.keyCodes[0x1A] = KEY_7;
	_plaf.keyCodes[0x1C] = KEY_8;
	_plaf.keyCodes[0x19] = KEY_9;
	_plaf.keyCodes[0x00] = KEY_A;
	_plaf.keyCodes[0x0B] = KEY_B;
	_plaf.keyCodes[0x08] = KEY_C;
	_plaf.keyCodes[0x02] = KEY_D;
	_plaf.keyCodes[0x0E] = KEY_E;
	_plaf.keyCodes[0x03] = KEY_F;
	_plaf.keyCodes[0x05] = KEY_G;
	_plaf.keyCodes[0x04] = KEY_H;
	_plaf.keyCodes[0x22] = KEY_I;
	_plaf.keyCodes[0x26] = KEY_J;
	_plaf.keyCodes[0x28] = KEY_K;
	_plaf.keyCodes[0x25] = KEY_L;
	_plaf.keyCodes[0x2E] = KEY_M;
	_plaf.keyCodes[0x2D] = KEY_N;
	_plaf.keyCodes[0x1F] = KEY_O;
	_plaf.keyCodes[0x23] = KEY_P;
	_plaf.keyCodes[0x0C] = KEY_Q;
	_plaf.keyCodes[0x0F] = KEY_R;
	_plaf.keyCodes[0x01] = KEY_S;
	_plaf.keyCodes[0x11] = KEY_T;
	_plaf.keyCodes[0x20] = KEY_U;
	_plaf.keyCodes[0x09] = KEY_V;
	_plaf.keyCodes[0x0D] = KEY_W;
	_plaf.keyCodes[0x07] = KEY_X;
	_plaf.keyCodes[0x10] = KEY_Y;
	_plaf.keyCodes[0x06] = KEY_Z;

	_plaf.keyCodes[0x27] = KEY_APOSTROPHE;
	_plaf.keyCodes[0x2A] = KEY_BACKSLASH;
	_plaf.keyCodes[0x2B] = KEY_COMMA;
	_plaf.keyCodes[0x18] = KEY_EQUAL;
	_plaf.keyCodes[0x32] = KEY_GRAVE_ACCENT;
	_plaf.keyCodes[0x21] = KEY_LEFT_BRACKET;
	_plaf.keyCodes[0x1B] = KEY_MINUS;
	_plaf.keyCodes[0x2F] = KEY_PERIOD;
	_plaf.keyCodes[0x1E] = KEY_RIGHT_BRACKET;
	_plaf.keyCodes[0x29] = KEY_SEMICOLON;
	_plaf.keyCodes[0x2C] = KEY_SLASH;
	_plaf.keyCodes[0x0A] = KEY_WORLD_1;

	_plaf.keyCodes[0x33] = KEY_BACKSPACE;
	_plaf.keyCodes[0x39] = KEY_CAPS_LOCK;
	_plaf.keyCodes[0x75] = KEY_DELETE;
	_plaf.keyCodes[0x7D] = KEY_DOWN;
	_plaf.keyCodes[0x77] = KEY_END;
	_plaf.keyCodes[0x24] = KEY_ENTER;
	_plaf.keyCodes[0x35] = KEY_ESCAPE;
	_plaf.keyCodes[0x7A] = KEY_F1;
	_plaf.keyCodes[0x78] = KEY_F2;
	_plaf.keyCodes[0x63] = KEY_F3;
	_plaf.keyCodes[0x76] = KEY_F4;
	_plaf.keyCodes[0x60] = KEY_F5;
	_plaf.keyCodes[0x61] = KEY_F6;
	_plaf.keyCodes[0x62] = KEY_F7;
	_plaf.keyCodes[0x64] = KEY_F8;
	_plaf.keyCodes[0x65] = KEY_F9;
	_plaf.keyCodes[0x6D] = KEY_F10;
	_plaf.keyCodes[0x67] = KEY_F11;
	_plaf.keyCodes[0x6F] = KEY_F12;
	_plaf.keyCodes[0x69] = KEY_PRINT_SCREEN;
	_plaf.keyCodes[0x6B] = KEY_F14;
	_plaf.keyCodes[0x71] = KEY_F15;
	_plaf.keyCodes[0x6A] = KEY_F16;
	_plaf.keyCodes[0x40] = KEY_F17;
	_plaf.keyCodes[0x4F] = KEY_F18;
	_plaf.keyCodes[0x50] = KEY_F19;
	_plaf.keyCodes[0x5A] = KEY_F20;
	_plaf.keyCodes[0x73] = KEY_HOME;
	_plaf.keyCodes[0x72] = KEY_INSERT;
	_plaf.keyCodes[0x7B] = KEY_LEFT;
	_plaf.keyCodes[0x3A] = KEY_LEFT_ALT;
	_plaf.keyCodes[0x3B] = KEY_LEFT_CONTROL;
	_plaf.keyCodes[0x38] = KEY_LEFT_SHIFT;
	_plaf.keyCodes[0x37] = KEY_LEFT_SUPER;
	_plaf.keyCodes[0x6E] = KEY_MENU;
	_plaf.keyCodes[0x47] = KEY_NUM_LOCK;
	_plaf.keyCodes[0x79] = KEY_PAGE_DOWN;
	_plaf.keyCodes[0x74] = KEY_PAGE_UP;
	_plaf.keyCodes[0x7C] = KEY_RIGHT;
	_plaf.keyCodes[0x3D] = KEY_RIGHT_ALT;
	_plaf.keyCodes[0x3E] = KEY_RIGHT_CONTROL;
	_plaf.keyCodes[0x3C] = KEY_RIGHT_SHIFT;
	_plaf.keyCodes[0x36] = KEY_RIGHT_SUPER;
	_plaf.keyCodes[0x31] = KEY_SPACE;
	_plaf.keyCodes[0x30] = KEY_TAB;
	_plaf.keyCodes[0x7E] = KEY_UP;

	_plaf.keyCodes[0x52] = KEY_KP_0;
	_plaf.keyCodes[0x53] = KEY_KP_1;
	_plaf.keyCodes[0x54] = KEY_KP_2;
	_plaf.keyCodes[0x55] = KEY_KP_3;
	_plaf.keyCodes[0x56] = KEY_KP_4;
	_plaf.keyCodes[0x57] = KEY_KP_5;
	_plaf.keyCodes[0x58] = KEY_KP_6;
	_plaf.keyCodes[0x59] = KEY_KP_7;
	_plaf.keyCodes[0x5B] = KEY_KP_8;
	_plaf.keyCodes[0x5C] = KEY_KP_9;
	_plaf.keyCodes[0x45] = KEY_KP_ADD;
	_plaf.keyCodes[0x41] = KEY_KP_DECIMAL;
	_plaf.keyCodes[0x4B] = KEY_KP_DIVIDE;
	_plaf.keyCodes[0x4C] = KEY_KP_ENTER;
	_plaf.keyCodes[0x51] = KEY_KP_EQUAL;
	_plaf.keyCodes[0x43] = KEY_KP_MULTIPLY;
	_plaf.keyCodes[0x4E] = KEY_KP_SUBTRACT;

	for (int scancode = 0;  scancode < MAX_KEY_CODES;  scancode++) {
		// Store the reverse translation for faster key name lookup
		if (_plaf.keyCodes[scancode] >= 0) {
			_plaf.scanCodes[_plaf.keyCodes[scancode]] = scancode;
		}
	}
}

@interface PLAFApplicationDelegate : NSObject <NSApplicationDelegate>
@end

@implementation PLAFApplicationDelegate

- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
    for (plafWindow* window = _plaf.windowListHead;  window;  window = window->next) {
        _plafInputWindowCloseRequest(window);
	}
    return NSTerminateCancel;
}

- (void)applicationDidChangeScreenParameters:(NSNotification *) notification {
    for (plafWindow* window = _plaf.windowListHead;  window;  window = window->next) {
		[window->context.nsglCtx update];
    }
    _plafPollMonitorsCocoa();
}

- (void)applicationWillFinishLaunching:(NSNotification *)notification {
}

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    plafPostEmptyEvent();
    [NSApp stop:nil];
}

- (void)applicationDidHide:(NSNotification *)notification {
    for (int i = 0;  i < _plaf.monitorCount;  i++) {
        _plafRestoreVideoModeCocoa(_plaf.monitors[i]);
	}
}

@end // PLAFApplicationDelegate

plafError* _plafInit(void) {
	@autoreleasepool {
		[NSApplication sharedApplication];

		_plaf.nsDelegate = [[PLAFApplicationDelegate alloc] init];
		if (_plaf.nsDelegate == nil) {
			plafTerminate();
			return createErrorResponse("Failed to create application delegate");
		}

		[NSApp setDelegate:_plaf.nsDelegate];

		NSEvent* (^block)(NSEvent*) = ^ NSEvent* (NSEvent* event) {
			if ([event modifierFlags] & NSEventModifierFlagCommand) {
				[[NSApp keyWindow] sendEvent:event];
			}
			return event;
		};

		_plaf.nsKeyUpMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyUp handler:block];

		// Press and Hold prevents some keys from emitting repeated characters
		NSDictionary* defaults = @{@"ApplePressAndHoldEnabled":@NO};
		[[NSUserDefaults standardUserDefaults] registerDefaults:defaults];

		createKeyTables();

		_plaf.nsEventSource = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
		if (!_plaf.nsEventSource) {
			plafTerminate();
			return createErrorResponse("Failed to create event source");
		}

		CGEventSourceSetLocalEventsSuppressionInterval(_plaf.nsEventSource, 0.0);

		_plafPollMonitorsCocoa();

		if (![[NSRunningApplication currentApplication] isFinishedLaunching]) {
			[NSApp run];
		}

		[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
		return NULL;
    }
}

void _plafTerminate(void) {
	@autoreleasepool {
		if (_plaf.nsEventSource) {
			CFRelease(_plaf.nsEventSource);
			_plaf.nsEventSource = NULL;
		}
		if (_plaf.nsDelegate) {
			[NSApp setDelegate:nil];
			[_plaf.nsDelegate release];
			_plaf.nsDelegate = nil;
		}
		if (_plaf.nsKeyUpMonitor) {
			[NSEvent removeMonitor:_plaf.nsKeyUpMonitor];
		}
		_plafTerminateNSGL();
	}
}

#endif // __APPLE__
