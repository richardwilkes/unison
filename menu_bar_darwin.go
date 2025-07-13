// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"fmt"

	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/internal/ns"
)

func quitMenuTitle() string {
	return i18n.Text("Quit")
}

func platformAddAppMenuEntries(m Menu) {
	m.InsertSeparator(-1, true)
	m.InsertMenu(-1, m.Factory().NewMenu(ServicesMenuID, i18n.Text("Services"), nil))
	m.InsertSeparator(-1, false)
	m.InsertItem(-1, m.Factory().NewItem(HideItemID, fmt.Sprintf(i18n.Text("Hide %s"), xos.AppName),
		KeyBinding{KeyCode: KeyH, Modifiers: OSMenuCmdModifier()},
		nil, func(MenuItem) { ns.HideApplication() }))
	m.InsertItem(-1, m.Factory().NewItem(HideOthersItemID, i18n.Text("Hide Others"),
		KeyBinding{KeyCode: KeyH, Modifiers: OptionModifier | OSMenuCmdModifier()},
		nil, func(MenuItem) { ns.HideOtherApplications() }))
	m.InsertItem(-1, m.Factory().NewItem(ShowAllItemID, i18n.Text("Show All"), KeyBinding{}, nil,
		func(MenuItem) { ns.UnhideAllApplications() }))
}
