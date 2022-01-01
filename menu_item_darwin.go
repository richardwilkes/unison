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
	"github.com/richardwilkes/unison/internal/ns"
)

var _ MenuItem = &macMenuItem{}

type macMenuItem struct {
	factory *macMenuFactory
	item    ns.MenuItem
}

func newMacMenuItemForSubMenu(f *macMenuFactory, subMenu *macMenu) ns.MenuItem {
	mi := ns.NewMenuItem(subMenu.id, subMenu.Title(), "", 0, nil, nil)
	mi.SetSubMenu(subMenu.menu)
	return mi
}

func (mi *macMenuItem) Factory() MenuFactory {
	return mi.factory
}

func (mi *macMenuItem) ID() int {
	return mi.item.Tag()
}

func (mi *macMenuItem) IsSame(other MenuItem) bool {
	if mi2, ok := other.(*macMenuItem); ok {
		return mi.item == mi2.item
	}
	return false
}

func (mi *macMenuItem) Menu() Menu {
	m := mi.item.Menu()
	if m == 0 {
		return nil
	}
	return &macMenu{
		factory: mi.factory,
		id:      mi.ID(),
		menu:    m,
	}
}

func (mi *macMenuItem) Index() int {
	if m := mi.Menu(); m != nil {
		count := m.Count()
		for i := 0; i < count; i++ {
			if mi.IsSame(m.ItemAtIndex(i)) {
				return i
			}
		}
	}
	return -1
}

func (mi *macMenuItem) IsSeparator() bool {
	return mi.item.IsSeparatorItem()
}

func (mi *macMenuItem) Title() string {
	return mi.item.Title()
}

func (mi *macMenuItem) SetTitle(title string) {
	mi.item.SetTitle(title)
}

func (mi *macMenuItem) SubMenu() Menu {
	subMenu := mi.item.SubMenu()
	if subMenu == 0 {
		return nil
	}
	return &macMenu{
		factory: mi.factory,
		id:      mi.ID(),
		menu:    subMenu,
	}
}

func (mi *macMenuItem) CheckState() CheckState {
	switch mi.item.State() {
	case ns.ControlStateValueOn:
		return OnCheckState
	case ns.ControlStateValueOff:
		return OffCheckState
	default:
		return MixedCheckState
	}
}

func (mi *macMenuItem) SetCheckState(s CheckState) {
	var itemState ns.ControlStateValue
	switch s {
	case OnCheckState:
		itemState = ns.ControlStateValueOn
	case OffCheckState:
		itemState = ns.ControlStateValueOff
	default:
		itemState = ns.ControlStateValueMixed
	}
	mi.item.SetState(itemState)
}

var macKeyCodeToMenuEquivalentMap = map[KeyCode]string{
	KeySpace:        " ",
	KeyApostrophe:   "'",
	KeyComma:        ",",
	KeyMinus:        "-",
	KeyPeriod:       ".",
	KeySlash:        "/",
	Key0:            "0",
	Key1:            "1",
	Key2:            "2",
	Key3:            "3",
	Key4:            "4",
	Key5:            "5",
	Key6:            "6",
	Key7:            "7",
	Key8:            "8",
	Key9:            "9",
	KeySemiColon:    ";",
	KeyEqual:        "=",
	KeyA:            "a",
	KeyB:            "b",
	KeyC:            "c",
	KeyD:            "d",
	KeyE:            "e",
	KeyF:            "f",
	KeyG:            "g",
	KeyH:            "h",
	KeyI:            "i",
	KeyJ:            "j",
	KeyK:            "k",
	KeyL:            "l",
	KeyM:            "m",
	KeyN:            "n",
	KeyO:            "o",
	KeyP:            "p",
	KeyQ:            "q",
	KeyR:            "r",
	KeyS:            "s",
	KeyT:            "t",
	KeyU:            "u",
	KeyV:            "v",
	KeyW:            "w",
	KeyX:            "x",
	KeyY:            "y",
	KeyZ:            "z",
	KeyOpenBracket:  "[",
	KeyBackslash:    `\`,
	KeyCloseBracket: "]",
	KeyBackQuote:    "`",
	KeyEscape:       "\x1b",
	KeyReturn:       "\x0d",
	KeyTab:          "\x09",
	KeyBackspace:    "\x08",
	KeyDelete:       "\uf728",
	KeyRight:        "\uf703",
	KeyLeft:         "\uf702",
	KeyDown:         "\uf701",
	KeyUp:           "\uf700",
	KeyPageUp:       "\uf72c",
	KeyPageDown:     "\uf72d",
	KeyHome:         "\uf729",
	KeyEnd:          "\uf72b",
	KeyF1:           "\uf704",
	KeyF2:           "\uf705",
	KeyF3:           "\uf706",
	KeyF4:           "\uf707",
	KeyF5:           "\uf708",
	KeyF6:           "\uf709",
	KeyF7:           "\uf70a",
	KeyF8:           "\uf70b",
	KeyF9:           "\uf70c",
	KeyF10:          "\uf70d",
	KeyF11:          "\uf70e",
	KeyF12:          "\uf70f",
	KeyF13:          "\uf710",
	KeyF14:          "\uf711",
	KeyF15:          "\uf712",
	KeyF16:          "\uf713",
	KeyF17:          "\uf714",
	KeyF18:          "\uf715",
	KeyF19:          "\uf716",
}
