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

var openPanelClass = objc.Get("NSOpenPanel")

// OpenPanel https://developer.apple.com/documentation/appkit/nsopenpanel?language=objc
type OpenPanel struct {
	SavePanel
}

// NewOpenPanel https://developer.apple.com/documentation/appkit/nsopenpanel/1584365-openpanel?language=objc
func NewOpenPanel() OpenPanel {
	return OpenPanel{SavePanel: SavePanel{Object: openPanelClass.Send("openPanel")}}
}

// CanChooseFiles https://developer.apple.com/documentation/appkit/nsopenpanel/1527060-canchoosefiles?language=objc
func (p OpenPanel) CanChooseFiles() bool {
	return p.Send("canChooseFiles").Bool()
}

// SetCanChooseFiles https://developer.apple.com/documentation/appkit/nsopenpanel/1527060-canchoosefiles?language=objc
func (p OpenPanel) SetCanChooseFiles(set bool) {
	p.Send("setCanChooseFiles:", set)
}

// CanChooseDirectories https://developer.apple.com/documentation/appkit/nsopenpanel/1532668-canchoosedirectories?language=objc
func (p OpenPanel) CanChooseDirectories() bool {
	return p.Send("canChooseDirectories").Bool()
}

// SetCanChooseDirectories https://developer.apple.com/documentation/appkit/nsopenpanel/1532668-canchoosedirectories?language=objc
func (p OpenPanel) SetCanChooseDirectories(set bool) {
	p.Send("setCanChooseDirectories:", set)
}

// ResolvesAliases https://developer.apple.com/documentation/appkit/nsopenpanel/1533625-resolvesaliases?language=objc
func (p OpenPanel) ResolvesAliases() bool {
	return p.Send("resolvesAliases").Bool()
}

// SetResolvesAliases https://developer.apple.com/documentation/appkit/nsopenpanel/1533625-resolvesaliases?language=objc
func (p OpenPanel) SetResolvesAliases(set bool) {
	p.Send("setResolvesAliases:", set)
}

// AllowsMultipleSelection https://developer.apple.com/documentation/appkit/nsopenpanel/1530786-allowsmultipleselection?language=objc
func (p OpenPanel) AllowsMultipleSelection() bool {
	return p.Send("allowsMultipleSelection").Bool()
}

// SetAllowsMultipleSelection https://developer.apple.com/documentation/appkit/nsopenpanel/1530786-allowsmultipleselection?language=objc
func (p OpenPanel) SetAllowsMultipleSelection(set bool) {
	p.Send("setAllowsMultipleSelection:", set)
}

// URLs https://developer.apple.com/documentation/appkit/nsopenpanel/1529845-urls?language=objc
func (p OpenPanel) URLs() Array {
	return Array{Object: p.Send("URLs")}
}
