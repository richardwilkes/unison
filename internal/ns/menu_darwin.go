// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package ns

import "github.com/progrium/macdriver/objc"

var menuClass = objc.Get("NSMenu")

// Menu https://developer.apple.com/documentation/appkit/nsmenu?language=objc
type Menu struct {
	objc.Object
}

// NewMenu https://developer.apple.com/documentation/appkit/nsmenu/1518144-initwithtitle?language=objc
func NewMenu(title string) Menu {
	titleStr := StringFromString(title)
	defer titleStr.Release()
	obj := menuClass.Alloc()
	obj.Send("initWithTitle:", titleStr)
	obj.Retain()
	return Menu{Object: obj}
}

// SetDelegate https://developer.apple.com/documentation/appkit/nsmenu/1518169-delegate?language=objc
func (m Menu) SetDelegate(delegate objc.Object) {
	m.Send("setDelegate:", delegate)
}

// NumberOfItems https://developer.apple.com/documentation/appkit/nsmenu/1518202-numberofitems?language=objc
func (m Menu) NumberOfItems() int {
	return int(m.Send("numberOfItems").Int())
}

// ItemAtIndex https://developer.apple.com/documentation/appkit/nsmenu/1518218-itematindex?language=objc
func (m Menu) ItemAtIndex(index int) MenuItem {
	return MenuItem{Object: m.Send("itemAtIndex:", index)}
}

// InsertItemAtIndex https://developer.apple.com/documentation/appkit/nsmenu/1518201-insertitem?language=objc
func (m Menu) InsertItemAtIndex(item MenuItem, index int) {
	m.Send("insertItem:atIndex:", item, index)
}

// RemoveItemAtIndex https://developer.apple.com/documentation/appkit/nsmenu/1518207-removeitematindex?language=objc
func (m Menu) RemoveItemAtIndex(index int) {
	m.Send("removeItemAtIndex:", index)
}

// Title https://developer.apple.com/documentation/appkit/nsmenu/1518192-title?language=objc
func (m Menu) Title() string {
	return m.Send("title").String()
}
