#include "platform.h"

// Notifies that a window has lost or received input focus
void _plafNotifyOfFocusChange(plafWindow* window, bool focused) {
	goWindowFocusCallback(window, focused);
	if (!focused) {
		for (int key = 0; key <= KEY_LAST; key++) {
			if (window->keys[key] == INPUT_PRESS) {
				_plafInputKey(window, key, _plaf.scanCodes[key], INPUT_RELEASE, 0);
			}
		}
		for (int button = 0; button <= MOUSE_BUTTON_LAST; button++) {
			if (window->mouseButtons[button] == INPUT_PRESS) {
				_plafInputMouseClick(window, button, INPUT_RELEASE, 0);
			}
		}
	}
}

// Notifies shared code that the user wishes to close a window
void _plafInputWindowCloseRequest(plafWindow* window) {
	window->shouldClose = true;
	goWindowCloseCallback(window);
}

//////////////////////////////////////////////////////////////////////////
//////                        PLAF public API                       //////
//////////////////////////////////////////////////////////////////////////

 plafWindow* plafCreateWindow(const char* title, plafWindowConfig* wndCfg, plafMonitor* monitor, plafWindow* share) {
	plafFrameBufferCfg fbconfig   = _plaf.frameBufferCfg;
	fbconfig.transparent          = wndCfg->transparent; // TODO: only use one of these
	plafWindow* window            = _plaf_calloc(1, sizeof(plafWindow));
	window->next                  = _plaf.windowListHead;
	_plaf.windowListHead          = window;
	window->videoMode.width       = 1;
	window->videoMode.height      = 1;
	window->videoMode.redBits     = fbconfig.redBits;
	window->videoMode.greenBits   = fbconfig.greenBits;
	window->videoMode.blueBits    = fbconfig.blueBits;
	window->videoMode.refreshRate = _plaf.desiredRefreshRate;
	window->monitor               = monitor;
	window->resizable             = wndCfg->resizable;
	window->decorated             = wndCfg->decorated;
	window->floating              = wndCfg->floating;
	window->mousePassthrough      = wndCfg->mousePassthrough;
	window->minwidth              = DONT_CARE;
	window->minheight             = DONT_CARE;
	window->maxwidth              = DONT_CARE;
	window->maxheight             = DONT_CARE;
	window->title                 = _plaf_strdup(title);
	if (!_plafCreateWindow(window, wndCfg, share, &fbconfig)) {
		plafDestroyWindow(window);
		return NULL;
	}
	return window;
}

void plafDestroyWindow(plafWindow* window) {
	if (window == NULL) {
		return;
	}
	if (window == _plaf.wndWithCurrentCtx) {
		plafMakeContextCurrent(NULL);
	}
	_plafDestroyWindow(window);
	plafWindow** prev = &_plaf.windowListHead;
	while (*prev != window) {
		prev = &((*prev)->next);
	}
	*prev = window->next;
	_plaf_free(window->title);
	_plaf_free(window);
}

const char* plafGetWindowTitle(plafWindow* window) {
	return window->title;
}

void plafSetWindowTitle(plafWindow* window, const char* title) {
	char* prev = window->title;
	window->title = _plaf_strdup(title);
	_plafSetWindowTitle(window, window->title);
	_plaf_free(prev);
}

void plafSetWindowPos(plafWindow* window, int xpos, int ypos) {
	if (window->monitor) {
		return;
	}
	_plafSetWindowPos(window, xpos, ypos);
}

void plafSetWindowSize(plafWindow* window, int width, int height) {
	window->videoMode.width  = width;
	window->videoMode.height = height;
	_plafSetWindowSize(window, width, height);
}

void plafSetWindowSizeLimits(plafWindow* window, int minwidth, int minheight, int maxwidth, int maxheight) {
	if (minwidth != DONT_CARE && minheight != DONT_CARE) {
		if (minwidth < 1 || minheight < 1) {
			return;
		}
	}
	if (maxwidth != DONT_CARE && maxheight != DONT_CARE) {
		if (maxwidth < 1 || maxheight < 1 || maxwidth < minwidth || maxheight < minheight) {
			return;
		}
	}
	window->minwidth  = minwidth;
	window->minheight = minheight;
	window->maxwidth  = maxwidth;
	window->maxheight = maxheight;
	if (window->monitor || !window->resizable) {
		return;
	}
	_plafSetWindowSizeLimits(window, minwidth, minheight, maxwidth, maxheight);
}

void plafMaximizeWindow(plafWindow* window) {
	if (window->monitor) {
		return;
	}
	_plafMaximizeWindow(window);
}

void plafShowWindow(plafWindow* window) {
	if (!window->monitor) {
		_plafShowWindow(window);
	}
}

void plafHideWindow(plafWindow* window) {
	if (window->monitor) {
		return;
	}
	_plafHideWindow(window);
}

void plafSetWindowResizable(plafWindow* window, bool enabled) {
	if (window->resizable != enabled) {
		window->resizable = enabled;
		if (!window->monitor) {
			_plafSetWindowResizable(window, enabled);
		}
	}
}

void plafSetWindowDecorated(plafWindow* window, bool enabled) {
	if (window->decorated != enabled) {
		window->decorated = enabled;
		if (!window->monitor) {
			_plafSetWindowDecorated(window, enabled);
		}
	}
}

void plafSetWindowFloating(plafWindow* window, bool enabled) {
	if (window->floating != enabled) {
		window->floating = enabled;
		if (!window->monitor) {
			_plafSetWindowFloating(window, enabled);
		}
	}
}

void plafSetWindowMousePassthrough(plafWindow* window, bool enabled) {
	if (window->mousePassthrough != enabled) {
		window->mousePassthrough = enabled;
		_plafSetWindowMousePassthrough(window, enabled);
	}
}

void plafSetWindowMonitor(plafWindow* window, plafMonitor* monitor, int xpos, int ypos, int width, int height, int refreshRate) {
	window->videoMode.width       = width;
	window->videoMode.height      = height;
	window->videoMode.refreshRate = refreshRate;
	_plafSetWindowMonitor(window, monitor, xpos, ypos, width, height, refreshRate);
}
