// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package updater lets an application packaged with upack and released on GitHub find newer releases of itself, show
// the user their release notes, and replace itself with one.
//
// An application makes one Updater, from a Config that describes it, and wires it up in four places:
//
//	var appUpdater *updater.Updater
//
//	func main() {
//		// Set xos.AppName, xos.AppCmdName, xos.AppVersion and xos.AppIdentifier before this, if not set by the linker.
//		appUpdater = updater.New(&updater.Config{GitHubOwner: "owner", GitHubRepo: "repo"})
//		appUpdater.FinishIfRequested(os.Args[1:])
//		unison.Start(unison.StartupFinishedCallback(func() {
//			appUpdater.ReportOutcome()
//			appUpdater.Check(false)
//			// ...open the application's windows
//		}))
//	}
//
// [Updater.FinishIfRequested] is how an update is completed: once the application has exited, a copy of it is started
// with FinishFlag, and that copy swaps the new build into place and starts it instead of running normally.
// [Updater.ReportOutcome] clears away what an update left behind and tells the user about one that failed.
// [Updater.Check] looks for releases newer than the running version and, on finding one the user has not yet been
// shown, opens the dialog [Updater.Notify] shows, which offers to install it and restart. The fourth place is the
// application's menus, where [Updater.Status] supplies the title of an item that calls Notify, and a "Check for
// Updates" item calls Check(true) while [Updater.Checking] is false. [Updater.CheckQuietly] is for an application that
// also checks on a schedule.
//
// Installing downloads the build for the running platform, verifies it and proves it runs, all beside the installation
// and without touching it, and then asks the application to quit. Nothing is replaced until the application has exited,
// and the previous version stays on disk until the new one has started.
//
// The package relies on the following:
//
//   - Releases are tagged "v<version>", and their assets carry the names upack gives them:
//     "<CmdName>-<version>-<os>-<arch>" followed by ".dmg", ".tgz" or ".zip", where os is macos, linux or windows and
//     arch is amd64 or arm64. GitHub must have published a SHA-256 digest for each.
//   - Run with Config.VersionArgs, "-v" unless set, the application exits successfully without opening a window, and
//     anything it prints starts with its version. xflag.AddVersionFlags provides this.
//   - On macOS, a release is signed by the same team as the installed copy.
//
// What happens after the application exits is recorded in a state file in the application data directory, as stable
// codes rather than messages: the build that reads it after the swap is not the one that wrote it, and may be running
// in a different language.
package updater
