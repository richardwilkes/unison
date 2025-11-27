package plaf

//#include "platform.h"
import "C"

import (
	"math"
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
)

var (
	initTermLock sync.Mutex
	initialized  bool
	initializing bool
	terminating  bool
)

// Init must be called exactly once before most things in this package.
func Init() error {
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
	var success bool
	defer func() {
		initTermLock.Lock()
		initializing = false
		initialized = success
		initTermLock.Unlock()
	}()
	if success = bool(C.plafInit()); !success {
		return errs.New("unable to initialize platform")
	}
	return nil
}

// Terminate should be called before exiting. It will destroy all remaining windows and free any allocated resources.
func Terminate() error {
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
	C.plafTerminate()
	return nil
}

func isInfOrNaN(value float64) bool {
	return value != value || value < -math.MaxFloat64 || value > math.MaxFloat64
}
