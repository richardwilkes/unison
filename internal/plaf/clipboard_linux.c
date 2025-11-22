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
    char* target = _plaf_calloc(size, 1);
    char* tp = target;
    for (sp = src;  *sp;  sp++) {
        tp += _plafEncodeUTF8(tp, *sp);
	}
    return target;
}

const char* plafGetClipboardString(void) {
	if (_plaf.xlibGetSelectionOwner(_plaf.x11Display, _plaf.x11ClipCLIPBOARD) != _plaf.x11HelperWindowHandle) {
		_plaf_free(_plaf.clipboardString);
		_plaf.clipboardString = NULL;

		const Atom targets[] = { _plaf.x11ClipUTF8_STRING, XA_STRING };
		const size_t targetCount = sizeof(targets) / sizeof(targets[0]);
		for (size_t i = 0;  i < targetCount;  i++) {
			_plaf.xlibConvertSelection(_plaf.x11Display, _plaf.x11ClipCLIPBOARD, targets[i], _plaf.x11ClipSELECTION,
				_plaf.x11HelperWindowHandle, CurrentTime);

			XEvent notification;
			while (!_plaf.xlibCheckTypedWindowEvent(_plaf.x11Display, _plaf.x11HelperWindowHandle, SelectionNotify,
				&notification)) {
				_plafWaitForX11Event(-1);
			}

			if (notification.xselection.property == None) {
				continue;
			}

			XEvent dummy;
			_plaf.xlibCheckIfEvent(_plaf.x11Display, &dummy, isSelPropNewValueNotify, (XPointer)&notification);

			Atom actualType;
			int actualFormat;
			unsigned long itemCount;
			unsigned long bytesAfter;
			char* data;
			_plaf.xlibGetWindowProperty(_plaf.x11Display, notification.xselection.requestor,
				notification.xselection.property, 0, LONG_MAX, True, AnyPropertyType, &actualType, &actualFormat,
				&itemCount, &bytesAfter, (unsigned char**)&data);
			if (actualType == _plaf.x11ClipINCR) {
				size_t size = 1;
				char* string = NULL;
				for (;;) {
					while (!_plaf.xlibCheckIfEvent(_plaf.x11Display, &dummy, isSelPropNewValueNotify,
						(XPointer) &notification)) {
						_plafWaitForX11Event(-1);
					}

					_plaf.xlibFree(data);
					_plaf.xlibGetWindowProperty(_plaf.x11Display, notification.xselection.requestor,
						notification.xselection.property, 0, LONG_MAX, True, AnyPropertyType, &actualType,
						&actualFormat, &itemCount, &bytesAfter, (unsigned char**)&data);

					if (itemCount) {
						size += itemCount;
						string = _plaf_realloc(string, size);
						string[size - itemCount - 1] = '\0';
						strcat(string, data);
					} else {
						if (string) {
							if (targets[i] == XA_STRING) {
								_plaf.clipboardString = convertLatin1toUTF8(string);
								_plaf_free(string);
							} else {
								_plaf.clipboardString = string;
							}
						}
						break;
					}
				}
			} else if (actualType == targets[i]) {
				_plaf.clipboardString = (targets[i] == XA_STRING) ? convertLatin1toUTF8(data) : _plaf_strdup(data);
			}
			_plaf.xlibFree(data);
			if (_plaf.clipboardString) {
				break;
			}
		}
	}
	return _plaf.clipboardString;
}

void plafSetClipboardString(const char* string) {
	_plaf_free(_plaf.clipboardString);
	_plaf.clipboardString = _plaf_strdup(string);
	_plaf.xlibSetSelectionOwner(_plaf.x11Display, _plaf.x11ClipCLIPBOARD, _plaf.x11HelperWindowHandle, CurrentTime);
}

#endif // __linux__
