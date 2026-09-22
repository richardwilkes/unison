// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"golang.org/x/sys/windows"
)

// TestSystemExportsResolve verifies that every export this package looks up by name is actually present in the system
// library it names. Each binding guards its call with Find and answers a missing export with a quiet failure, since
// that is the right behavior on a system without the library, so a misspelled name is never a panic and never a test
// failure anywhere else: it is a provider that is silently never handed to UI Automation, which a screen reader
// reports as a window with nothing inside it.
func TestSystemExportsResolve(t *testing.T) {
	c := check.New(t)
	for _, proc := range []*windows.LazyProc{
		clientsAreListeningProc,
		disconnectProviderProc,
		getReservedMixedAttributeValueProc,
		getReservedNotSupportedValueProc,
		hostProviderFromHwndProc,
		raiseAutomationEventProc,
		raiseAutomationPropertyChangedEventProc,
		raiseNotificationEventProc,
		raiseStructureChangedEventProc,
		returnRawElementProviderProc,
		sysAllocStringLenProc,
		sysFreeStringProc,
		sysStringLenProc,
		variantClearProc,
		safeArrayCreateVectorProc,
		safeArrayDestroyProc,
		safeArrayGetElementProc,
		safeArrayGetUBoundProc,
		safeArrayPutElementProc,
	} {
		c.NoError(proc.Find(), proc.Name)
	}
}
