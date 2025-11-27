package plaf

//#include "platform.h"
import "C"

import (
	"github.com/richardwilkes/unison/internal/mac"
)

//export goAppOpenURLsCallback
func goAppOpenURLsCallback(a C.CFArrayRef) {
	if OpenFilesCallback != nil {
		if urls := mac.Array(a).ArrayOfURLToStringSlice(); len(urls) > 0 {
			OpenFilesCallback(urls)
		}
	}
}
