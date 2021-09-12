// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package ns

import (
	"github.com/progrium/macdriver/objc"
)

var popupButtonCellClass = objc.Get("NSPopUpButtonCell")

// PopupButtonCell https://developer.apple.com/documentation/appkit/nspopupbuttoncell?language=objc
type PopupButtonCell struct {
	objc.Object
}

// NewPopupButtonCell https://developer.apple.com/documentation/appkit/nspopupbuttoncell/1528591-inittextcell?language=objc
func NewPopupButtonCell(text string, pullsDown bool) PopupButtonCell {
	textStr := StringFromString(text)
	defer textStr.Release()
	return PopupButtonCell{Object: popupButtonCellClass.Alloc().Send("initTextCell:pullsDown:", textStr, pullsDown)}
}

// SetAutoEnablesItems https://developer.apple.com/documentation/appkit/nspopupbuttoncell/1530889-autoenablesitems?language=objc
func (p PopupButtonCell) SetAutoEnablesItems(enabled bool) {
	p.Send("setAutoenablesItems:", enabled)
}

// SetAltersStateOfSelectedItem https://developer.apple.com/documentation/appkit/nspopupbuttoncell/1528446-altersstateofselecteditem?language=objc
func (p PopupButtonCell) SetAltersStateOfSelectedItem(enabled bool) {
	p.Send("setAltersStateOfSelectedItem:", enabled)
}

// SetMenu https://developer.apple.com/documentation/appkit/nspopupbuttoncell/1529059-menu?language=objc
func (p PopupButtonCell) SetMenu(menu Menu) {
	p.Send("setMenu:", menu)
}

// SelectItem https://developer.apple.com/documentation/appkit/nspopupbuttoncell/1525225-selectitem?language=objc
func (p PopupButtonCell) SelectItem(menuItem MenuItem) {
	p.Send("selectItem:", menuItem)
}

// PerformClickWithFrameInView https://developer.apple.com/documentation/appkit/nspopupbuttoncell/1530205-performclickwithframe?language=objc
func (p PopupButtonCell) PerformClickWithFrameInView(frame Rect, view View) {
	p.Send("performClickWithFrame:inView:", frame, view)
}
