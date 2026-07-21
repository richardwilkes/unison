// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"runtime"

	"github.com/richardwilkes/unison/internal/cocoa"
)

// panelReleaser matches the owned cocoa panel handles the platform file dialogs hold (cocoa.OpenPanel and
// cocoa.SavePanel).
type panelReleaser interface {
	Release()
}

// releasePanelOnCleanup arranges for panel, an owned Objective-C reference, to be released once owner becomes
// unreachable. The OpenDialog/SaveDialog interfaces have no dispose method, so the Go wrapper being garbage collected
// is the only signal that the panel is no longer needed; without this, every NewOpenDialog/NewSaveDialog call leaked
// its panel (and everything it references) for the life of the process. The cleanup must not capture owner, or it
// would never become collectable, and it runs on the runtime's cleanup goroutine, so the release is marshaled onto
// the UI thread, where AppKit objects must be released.
func releasePanelOnCleanup[T any](owner *T, panel panelReleaser) {
	runtime.AddCleanup(owner, func(p panelReleaser) {
		InvokeTask(p.Release)
	}, panel)
}

// setAllowedFileTypes converts types into an owned NSArray, hands it to set (an open or save panel's
// SetAllowedFileTypes), and releases it. The panel's allowedFileTypes property copies the array, so ownership stays
// with the caller; passing the array inline without a release leaked one NSArray plus its NSStrings per call. An
// empty list clears the property with a nil handle instead.
func setAllowedFileTypes(set func(cocoa.Array), types []string) {
	if len(types) == 0 {
		set(0)
		return
	}
	allowed := cocoa.NewArrayFromStringSlice(types)
	defer allowed.Release()
	set(allowed)
}
