package plaf2

import (
	"log/slog"

	"github.com/richardwilkes/unison/internal/mac"
)

func initialize() error { // formerly _plafInit
	mac.AppShouldTerminateCallback = func() {
		// TODO: Initiate termination sequence, typically closing all windows, then exiting
	}
	mac.AppDidChangeScreenParameters = func() {
		for _, w := range windowList {
			slog.Info("here to temporarily ignore compiler error about unused variable w", "window", w)
			/* TODO
			[window->context.nsglCtx update];
			*/
		}
	}
	mac.AppDidFinishLaunchingCallback = func() {
		mac.PostEmptyEvent()
		mac.StopMainEventLoop()
	}
	mac.AppDidHideCallback = func() {
	}
	// NOTE: Three additional app delegate callbacks exist: AppWillFinishLaunchingCallback, AppDidHideCallback and
	//       OpenFilesCallback.
	if err := mac.InstallMacAppDelegate(); err != nil {
		return err
	}
	createKeyTables()
	mac.FinishLaunching()
	return nil
}

func terminate() error { // formerly _plafTerminate
	mac.UninstallMacAppDelegate()
	return nil
}
