#if defined(__linux__)

#include "platform.h"
#include <limits.h>

// Returns whether it is a property event for the specified selection transfer.
static Bool isSelPropNewValueNotify(Display* display, XEvent* event, XPointer pointer) {
    return event->type == PropertyNotify &&
           event->xproperty.state == PropertyNewValue &&
           event->xproperty.window == ((XEvent*) pointer)->xselection.requestor &&
           event->xproperty.atom == ((XEvent*) pointer)->xselection.property;
}

// Convert the specified Latin-1 string to UTF-8
static char* convertLatin1toUTF8(const char* src) {
    size_t size = 1;
    const char* sp;
    for (sp = src;  *sp;  sp++) {
        size += (*sp & 0x80) ? 2 : 1;
	}
    char* target = _glfw_calloc(size, 1);
    char* tp = target;
    for (sp = src;  *sp;  sp++) {
        tp += _glfwEncodeUTF8(tp, *sp);
	}
    return target;
}

const char* getClipboardString(void) {
	char** selectionString = NULL;
	const Atom targets[] = { _glfw.x11ClipUTF8_STRING, XA_STRING };
	const size_t targetCount = sizeof(targets) / sizeof(targets[0]);

	selectionString = &_glfw.clipboardString;

	if (_glfw.xlibGetSelectionOwner(_glfw.x11Display, _glfw.x11ClipCLIPBOARD) == _glfw.x11HelperWindowHandle) {
		return *selectionString;
	}

	_glfw_free(*selectionString);
	*selectionString = NULL;

	for (size_t i = 0;  i < targetCount;  i++) {
		char* data;
		Atom actualType;
		int actualFormat;
		unsigned long itemCount, bytesAfter;
		XEvent notification, dummy;

		_glfw.xlibConvertSelection(_glfw.x11Display, _glfw.x11ClipCLIPBOARD, targets[i], _glfw.x11ClipSELECTION,
			_glfw.x11HelperWindowHandle, CurrentTime);

		while (!_glfw.xlibCheckTypedWindowEvent(_glfw.x11Display, _glfw.x11HelperWindowHandle, SelectionNotify,
			&notification)) {
			waitForX11Event(-1);
		}

		if (notification.xselection.property == None) {
			continue;
		}

		_glfw.xlibCheckIfEvent(_glfw.x11Display, &dummy, isSelPropNewValueNotify, (XPointer) &notification);

		_glfw.xlibGetWindowProperty(_glfw.x11Display, notification.xselection.requestor, notification.xselection.property, 0,
			LONG_MAX, True, AnyPropertyType, &actualType, &actualFormat, &itemCount, &bytesAfter,
			(unsigned char**) &data);

		if (actualType == _glfw.x11ClipINCR) {
			size_t size = 1;
			char* string = NULL;

			for (;;) {
				while (!_glfw.xlibCheckIfEvent(_glfw.x11Display, &dummy, isSelPropNewValueNotify, (XPointer) &notification)) {
					waitForX11Event(-1);
				}

				_glfw.xlibFree(data);
				_glfw.xlibGetWindowProperty(_glfw.x11Display, notification.xselection.requestor,
					notification.xselection.property, 0, LONG_MAX, True, AnyPropertyType, &actualType, &actualFormat,
					&itemCount, &bytesAfter, (unsigned char**) &data);

				if (itemCount) {
					size += itemCount;
					string = _glfw_realloc(string, size);
					string[size - itemCount - 1] = '\0';
					strcat(string, data);
				}

				if (!itemCount) {
					if (string) {
						if (targets[i] == XA_STRING) {
							*selectionString = convertLatin1toUTF8(string);
							_glfw_free(string);
						} else {
							*selectionString = string;
						}
					}
					break;
				}
			}
		} else if (actualType == targets[i]) {
			*selectionString = (targets[i] == XA_STRING) ? convertLatin1toUTF8(data) : _glfw_strdup(data);
		}

		_glfw.xlibFree(data);
		if (*selectionString) {
			break;
		}
	}
	return *selectionString;
}

void setClipboardString(const char* string) {
	_glfw_free(_glfw.clipboardString);
	_glfw.clipboardString = _glfw_strdup(string);
	_glfw.xlibSetSelectionOwner(_glfw.x11Display, _glfw.x11ClipCLIPBOARD, _glfw.x11HelperWindowHandle, CurrentTime);
}

#endif // __linux__
