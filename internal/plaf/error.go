package plaf

/*
#include "platform.h"

void goErrorCallback(const char* description);
*/
import "C"

import (
	"errors"
	"fmt"
	"log"
	"os"
	"unsafe"
)

func convertErrorResponse(errResp *C.plafError) error {
	if errResp == nil {
		return nil
	}
	s := C.GoString(&errResp.desc[0])
	next := errResp.next
	C.free(unsafe.Pointer(errResp))
	if next != nil {
		s = "Multiple errors:\n- " + s
		for next != nil {
			s += "\n- " + C.GoString(&next.desc[0])
			errResp = next
			next = next.next
			C.free(unsafe.Pointer(errResp))
		}
	}
	return errors.New(s)
}

// Holds the value of the last error.
var lastError = make(chan error, 1)

// Set the plaf callback internally
func init() {
	C.plafSetErrorCallback(C.errorFunc(C.goErrorCallback))
}

// flushErrors is called by Terminate before it actually calls C.plafTerminate,
// this ensures that any uncaught errors buffered in lastError are printed
// before the program exits.
func flushErrors() {
	if err := fetchError(); err != nil {
		fmt.Fprintln(os.Stderr, "go-gl/plaf: internal error: an uncaught error has occurred:", err)
		fmt.Fprintln(os.Stderr, "go-gl/plaf: Please report this in the Go package issue tracker.")
	}
}

func acceptError() {
	if err := fetchError(); err != nil {
		log.Println(err)
	}
}

// panicError is a helper used by functions which expect no errors (except
// programmer errors) to occur. It will panic if it finds any such error.
func panicError() {
	if err := fetchError(); err != nil {
		panic(err)
	}
}

// fetchError fetches the next error from the error channel, it does not block
// and returns nil if there is no error present.
func fetchError() error {
	select {
	case err := <-lastError:
		return err
	default:
		return nil
	}
}
