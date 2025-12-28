package plaf2

import (
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
)

var (
	OpenFilesCallback func([]string)
	initTermLock      sync.Mutex
	initialized       bool
	initializing      bool
	terminating       bool
)

// Init must be called exactly once before most things in this package may be used.
func Init() error { // formerly plafInit
	initTermLock.Lock()
	if initialized {
		initTermLock.Unlock()
		return errs.New("already initialized")
	}
	if initializing {
		initTermLock.Unlock()
		return errs.New("initialization already in progress")
	}
	if terminating {
		initTermLock.Unlock()
		return errs.New("termination in progress")
	}
	initializing = true
	initTermLock.Unlock()
	var err error
	defer func() {
		initTermLock.Lock()
		initializing = false
		initialized = err == nil
		initTermLock.Unlock()
	}()
	err = initialize()
	return err
}

// Terminate should be called before exiting, as it destroys all remaining windows and frees any allocated resources.
func Terminate() error { // formerly plafTerminate
	initTermLock.Lock()
	if terminating {
		initTermLock.Unlock()
		return errs.New("termination already in progress")
	}
	if !initialized {
		initTermLock.Unlock()
		return errs.New("initialization has not been performed")
	}
	terminating = true
	initTermLock.Unlock()
	defer func() {
		initTermLock.Lock()
		terminating = false
		initTermLock.Unlock()
	}()
	for len(windowList) != 0 {
		windowList[len(windowList)-1].Destroy()
	}
	for len(cursorList) != 0 {
		cursorList[len(cursorList)-1].Destroy()
	}
	return terminate()
}
