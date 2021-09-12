// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/toolbox/cmdline"
	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/unison/internal/ns"
)

func platformAddAppMenuEntries(m Menu) {
	m.InsertSeparator(-1, true)
	m.InsertMenu(-1, m.Factory().NewMenu(ServicesMenuID, i18n.Text("Services"), nil))
	m.InsertSeparator(-1, false)
	m.InsertItem(-1, m.Factory().NewItem(HideItemID, fmt.Sprintf(i18n.Text("Hide %s"), cmdline.AppName), KeyH,
		OSMenuCmdModifier(), nil, func(MenuItem) { ns.CurrentApplication().Hide() }))
	m.InsertItem(-1, m.Factory().NewItem(HideOthersItemID, i18n.Text("Hide Others"), KeyH,
		OptionModifier|OSMenuCmdModifier(), nil, func(MenuItem) { ns.App().HideOtherApplications() }))
	m.InsertItem(-1, m.Factory().NewItem(ShowAllItemID, i18n.Text("Show All"), KeyNone, NoModifiers, nil,
		func(MenuItem) { ns.App().UnhideAllApplications() }))
}
