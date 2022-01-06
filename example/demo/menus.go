// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"github.com/richardwilkes/unison"
)

func installDefaultMenus(wnd *unison.Window) {
	unison.DefaultMenuFactory().BarForWindow(wnd, func(m unison.Menu) {
		unison.InsertStdMenus(m, ShowAboutWindow, nil, nil)
		fileMenu := m.Menu(unison.FileMenuID)
		f := fileMenu.Factory()
		newMenu := f.NewMenu(NewMenuID, "New…", nil)
		newMenu.InsertItem(-1, NewWindowAction.NewMenuItem(f))
		newMenu.InsertItem(-1, NewTableWindowAction.NewMenuItem(f))
		newMenu.InsertItem(-1, NewDockWindowAction.NewMenuItem(f))
		fileMenu.InsertMenu(0, newMenu)
		fileMenu.InsertItem(1, OpenAction.NewMenuItem(f))
	})
}
