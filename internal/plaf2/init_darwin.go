package plaf2

import (
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/internal/mac"
)

func initialize() error { // formerly _plafInit
	mac.AppShouldTerminateCallback = func() {
		var last *Window
		for len(windowList) > 0 {
			windowList[0].RequestClose()
			if len(windowList) != 0 {
				if windowList[0] == last {
					break
				}
				last = windowList[0]
			}
		}
		xos.Exit(0)
	}
	mac.AppDidChangeScreenParameters = func() {
		for _, w := range windowList {
			w.plGctx.ctx.Update()
		}
	}
	mac.AppDidFinishLaunchingCallback = func() {
		mac.PostEmptyEvent()
		mac.StopMainEventLoop()
	}
	mac.OpenFilesCallback = func(paths []string) {
		if OpenFilesCallback != nil {
			OpenFilesCallback(paths)
		}
	}
	// NOTE: Two additional app delegate callbacks exist: AppWillFinishLaunchingCallback and AppDidHideCallback.
	if err := mac.InstallMacAppDelegate(); err != nil {
		return err
	}
	createKeyTables()
	initWindowCallbacks()
	mac.FinishLaunching()
	return nil
}

func terminate() error { // formerly _plafTerminate
	mac.UninstallMacAppDelegate()
	return nil
}
