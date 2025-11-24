package plaf

//#include "platform.h"
import "C"

// Init initializes the PLAF library. Before most PLAF functions can be used,
// PLAF must be initialized, and before a program terminates PLAF should be
// terminated in order to free any resources allocated during or after
// initialization.
//
// If this function fails, it calls Terminate before returning. If it succeeds,
// you should call Terminate before the program exits.
//
// Additional calls to this function after successful initialization but before
// termination will succeed but will do nothing.
func Init() error {
	return convertErrorResponse(C.plafInit())
}

// Terminate destroys all remaining windows, frees any allocated resources and
// sets the library to an uninitialized state. Once this is called, you must
// again call Init successfully before you will be able to use most PLAF
// functions.
//
// If PLAF has been successfully initialized, this function should be called
// before the program exits. If initialization fails, there is no need to call
// this function, as it is called by Init before it returns failure.
//
// This function may only be called from the main thread.
func Terminate() {
	flushErrors()
	C.plafTerminate()
}
