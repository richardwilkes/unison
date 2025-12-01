package plaf2

import (
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
)

var (
	initTermLock            sync.Mutex
	initialized             bool
	initializing            bool
	terminating             bool
	CommonFrameBufferConfig = FrameBufferConfig{
		RedBits:     8,
		GreenBits:   8,
		BlueBits:    8,
		AlphaBits:   8,
		DepthBits:   24,
		StencilBits: 8,
	}
)

type FrameBufferConfig struct {
	RedBits        int
	GreenBits      int
	BlueBits       int
	AlphaBits      int
	DepthBits      int
	StencilBits    int
	AccumRedBits   int
	AccumGreenBits int
	AccumBlueBits  int
	AccumAlphaBits int
	Samples        int
	IsSRGB         bool
	IsTransparent  bool
}

// Init must be called exactly once before most things in this package may be used.
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
	return initialize()
}

// Terminate should be called before exiting, as it destroys all remaining windows and frees any allocated resources.
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
	for len(windowList) != 0 {
		windowList[len(windowList)-1].Destroy()
	}
	for len(cursorList) != 0 {
		cursorList[len(cursorList)-1].Destroy()
	}
	for _, m := range monitorList {
		if m.originalGammaRamp != nil {
			m.SetGammaRamp(m.originalGammaRamp)
			m.originalGammaRamp = nil
		}
	}
	return terminate()
}
