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

var pasteboardClass = objc.Get("NSPasteboard")

// Pasteboard https://developer.apple.com/documentation/appkit/nspasteboard?language=objc
type Pasteboard struct {
	objc.Object
}

// GeneralPasteboard https://developer.apple.com/documentation/appkit/nspasteboard/1530091-generalpasteboard?language=objc
func GeneralPasteboard() Pasteboard {
	return Pasteboard{Object: pasteboardClass.Get("generalPasteboard")}
}

// ClearContents https://developer.apple.com/documentation/appkit/nspasteboard/1533599-clearcontents?language=objc
func (p Pasteboard) ClearContents() {
	p.Send("clearContents")
}

// ChangeCount https://developer.apple.com/documentation/appkit/nspasteboard/1533544-changecount?language=objc
func (p Pasteboard) ChangeCount() int {
	return int(p.Send("changeCount").Int())
}

// Types https://developer.apple.com/documentation/appkit/nspasteboard/1529599-types?language=objc
func (p Pasteboard) Types() Array {
	return Array{Object: p.Send("types")}
}

// Items https://developer.apple.com/documentation/appkit/nspasteboard/1529995-pasteboarditems?language=objc
func (p Pasteboard) Items() Array {
	return Array{Object: p.Send("pasteboardItems")}
}

// WriteItems https://developer.apple.com/documentation/appkit/nspasteboard/1525945-writeobjects?language=objc
func (p Pasteboard) WriteItems(data Array) {
	p.Send("writeObjects:", data)
}
