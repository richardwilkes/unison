package plaf

//#include "platform.h"
import "C"

import "unsafe"

// GetClipboardString returns the contents of the system clipboard, if it contains or is convertible to a UTF-8 encoded
// string.
func GetClipboardString() string {
	s := C.plafGetClipboardString()
	if s == nil {
		return ""
	}
	return C.GoString(s)
}

// SetClipboardString sets the system clipboard to the specified UTF-8 encoded string.
func SetClipboardString(str string) {
	s := C.CString(str)
	defer C.free(unsafe.Pointer(s))
	C.plafSetClipboardString(s)
}
