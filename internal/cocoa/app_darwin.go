// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"net/url"
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
)

const (
	// nsTerminateCancel is NSApplicationTerminateReply's NSTerminateCancel.
	nsTerminateCancel uint64 = 0
	// nsApplicationActivationPolicyRegular is NSApplicationActivationPolicy's NSApplicationActivationPolicyRegular.
	nsApplicationActivationPolicyRegular int64 = 0
)

// Callbacks invoked by the application delegate installed by InstallMacAppDelegate. Set them before installing the
// delegate; they are invoked on the main thread from within the event loop.
var (
	// AppShouldTerminateCallback is invoked when the application has been asked to terminate (e.g. via the Quit menu
	// item or a Command-Q key equivalent handled by the system). The delegate always reports NSTerminateCancel to
	// AppKit, so the callback is responsible for actually exiting if that is the desired outcome.
	AppShouldTerminateCallback func()
	// AppDidChangeScreenParameters is invoked when the screen configuration changes.
	AppDidChangeScreenParameters func()
	// AppWillFinishLaunchingCallback is invoked just before the application finishes launching.
	AppWillFinishLaunchingCallback func()
	// AppDidFinishLaunchingCallback is invoked when the application finishes launching.
	AppDidFinishLaunchingCallback func()
	// AppDidHideCallback is invoked when the application is hidden.
	AppDidHideCallback func()
	// OpenFilesCallback is invoked when the system asks the application to open files or URLs.
	OpenFilesCallback func([]string)
)

var (
	appDelegateClassOnce sync.Once
	appDelegateClass     objc.Class
	appDelegateClassErr  error
	appDelegate          objc.ID
	keyUpMonitor         objc.ID
	keyUpBlock           objc.Block
)

// sharedApp returns the shared NSApplication instance, creating it if it does not already exist.
func sharedApp() objc.ID {
	return objc.ID(Cls("NSApplication")).Send(Sel("sharedApplication"))
}

// registerAppDelegateClass registers the macAppDelegate Objective-C class. Registration is process-global and can
// only happen once per class name, so it is guarded by appDelegateClassOnce; instances are created per install.
func registerAppDelegateClass() {
	LoadAppKit()
	var protocols []*objc.Protocol
	if p := objc.GetProtocol("NSApplicationDelegate"); p != nil {
		protocols = append(protocols, p)
	}
	cls, err := objc.RegisterClass("macAppDelegate", Cls("NSObject"), protocols, nil, []objc.MethodDef{
		{
			Cmd: Sel("applicationShouldTerminate:"),
			Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) uint64 {
				if AppShouldTerminateCallback != nil {
					AppShouldTerminateCallback()
				}
				return nsTerminateCancel
			},
		},
		{
			Cmd: Sel("applicationDidChangeScreenParameters:"),
			Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) {
				if AppDidChangeScreenParameters != nil {
					AppDidChangeScreenParameters()
				}
			},
		},
		{
			Cmd: Sel("applicationWillFinishLaunching:"),
			Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) {
				if AppWillFinishLaunchingCallback != nil {
					AppWillFinishLaunchingCallback()
				}
			},
		},
		{
			Cmd: Sel("applicationDidFinishLaunching:"),
			Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) {
				if AppDidFinishLaunchingCallback != nil {
					AppDidFinishLaunchingCallback()
				}
			},
		},
		{
			Cmd: Sel("applicationDidHide:"),
			Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) {
				if AppDidHideCallback != nil {
					AppDidHideCallback()
				}
			},
		},
		{
			Cmd: Sel("application:openURLs:"),
			Fn: func(_ objc.ID, _ objc.SEL, _, urls objc.ID) {
				if OpenFilesCallback != nil {
					if paths := filePathsFromNSURLArray(urls); len(paths) > 0 {
						OpenFilesCallback(paths)
					}
				}
			},
		},
	})
	if err != nil {
		appDelegateClassErr = errs.NewWithCause("InstallMacAppDelegate: unable to register app delegate class", err)
		return
	}
	appDelegateClass = cls
}

// filePathsFromNSURLArray converts an NSArray of NSURL into the path components of those URLs, mirroring the cgo
// bridge's Array.ArrayOfURLToStringSlice.
func filePathsFromNSURLArray(urls objc.ID) []string {
	ids := IDsFromNSArray(urls)
	result := make([]string, 0, len(ids))
	for _, u := range ids {
		urlStr := GoStringFromNSString(u.Send(Sel("absoluteString")))
		parsed, err := url.Parse(urlStr)
		if err != nil {
			errs.Log(errs.NewWithCause("unable to parse URL", err), "url", urlStr)
			continue
		}
		result = append(result, parsed.Path)
	}
	return result
}

// InstallMacAppDelegate creates the shared NSApplication if needed, installs the application delegate that routes
// AppKit's application lifecycle notifications to the App*Callback funcs, and installs a local event monitor that
// forwards Command-modified key-up events to the key window (AppKit's sendEvent: swallows those, so without the
// monitor no key-up would ever be reported while the Command key is held).
func InstallMacAppDelegate() error {
	sharedApp()
	appDelegateClassOnce.Do(registerAppDelegateClass)
	if appDelegateClassErr != nil {
		return appDelegateClassErr
	}
	delegate := objc.ID(appDelegateClass).Send(Sel("new"))
	if delegate == 0 {
		return errs.New("InstallMacAppDelegate: unable to install app delegate")
	}
	// A repeated install replaces any prior installation rather than leaking its delegate instance and key-up
	// monitor — a leaked monitor would forward every Cmd+keyUp event once per leaked install.
	UninstallMacAppDelegate()
	appDelegate = delegate
	sharedApp().Send(Sel("setDelegate:"), delegate)
	keyUpBlock = objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
		if EventModifierFlags(objc.Send[uint64](event, Sel("modifierFlags")))&EventModifierFlagCommand != 0 {
			sharedApp().Send(Sel("keyWindow")).Send(Sel("sendEvent:"), event)
		}
		return event
	})
	keyUpMonitor = objc.ID(Cls("NSEvent")).Send(Sel("addLocalMonitorForEventsMatchingMask:handler:"),
		nsEventMaskKeyUp, keyUpBlock)
	return nil
}

// UninstallMacAppDelegate removes the application delegate and key-up event monitor installed by
// InstallMacAppDelegate.
func UninstallMacAppDelegate() {
	sharedApp().Send(Sel("setDelegate:"), objc.ID(0))
	if keyUpMonitor != 0 {
		objc.ID(Cls("NSEvent")).Send(Sel("removeMonitor:"), keyUpMonitor)
		keyUpMonitor = 0
	}
	if keyUpBlock != 0 {
		keyUpBlock.Release()
		keyUpBlock = 0
	}
	if appDelegate != 0 {
		Release(appDelegate)
		appDelegate = 0
	}
}

// FinishLaunching runs the main event loop via [NSApp run] until something stops it (unison's
// AppDidFinishLaunchingCallback posts an empty event and stops the loop, so in practice this returns as soon as the
// application has finished launching), then switches the activation policy to a regular application.
func FinishLaunching() {
	if !objc.Send[bool](objc.ID(Cls("NSRunningApplication")).Send(Sel("currentApplication")),
		Sel("isFinishedLaunching")) {
		sharedApp().Send(Sel("run"))
	}
	sharedApp().Send(Sel("setActivationPolicy:"), nsApplicationActivationPolicyRegular)
}

// ActivateIgnoringOtherApps makes the application the active application.
func ActivateIgnoringOtherApps() {
	sharedApp().Send(Sel("activateIgnoringOtherApps:"), true)
}

// HideApplication hides the application.
func HideApplication() {
	objc.ID(Cls("NSRunningApplication")).Send(Sel("currentApplication")).Send(Sel("hide"))
}

// HideOtherApplications hides all applications other than this one.
func HideOtherApplications() {
	app := sharedApp()
	app.Send(Sel("hideOtherApplications:"), app)
}

// UnhideAllApplications unhides all applications.
func UnhideAllApplications() {
	app := sharedApp()
	app.Send(Sel("unhideAllApplications:"), app)
}

// SetMainMenu sets the application's main menu bar.
func SetMainMenu(menu Menu) {
	sharedApp().Send(Sel("setMainMenu:"), objc.ID(menu))
}

// SetServicesMenu sets the application's Services menu.
func SetServicesMenu(menu Menu) {
	sharedApp().Send(Sel("setServicesMenu:"), objc.ID(menu))
}

// SetWindowsMenu sets the application's Window menu.
func SetWindowsMenu(menu Menu) {
	sharedApp().Send(Sel("setWindowsMenu:"), objc.ID(menu))
}

// SetHelpMenu sets the application's Help menu.
func SetHelpMenu(menu Menu) {
	sharedApp().Send(Sel("setHelpMenu:"), objc.ID(menu))
}
