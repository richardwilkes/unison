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

var savePanelClass = objc.Get("NSSavePanel")

// SavePanel https://developer.apple.com/documentation/appkit/nssavepanel?language=objc
type SavePanel struct {
	objc.Object
}

// NewSavePanel https://developer.apple.com/documentation/appkit/nssavepanel/1539016-savepanel?language=objc
func NewSavePanel() SavePanel {
	return SavePanel{Object: savePanelClass.Send("savePanel")}
}

// DirectoryURL https://developer.apple.com/documentation/appkit/nssavepanel/1531279-directoryurl?language=objc
func (p SavePanel) DirectoryURL() URL {
	return URL{Object: p.Send("directoryURL")}
}

// SetDirectoryURL https://developer.apple.com/documentation/appkit/nssavepanel/1531279-directoryurl?language=objc
func (p SavePanel) SetDirectoryURL(url URL) {
	p.Send("setDirectoryURL:", url)
}

// AllowedFileTypes https://developer.apple.com/documentation/appkit/nssavepanel/1534419-allowedfiletypes?language=objc
func (p SavePanel) AllowedFileTypes() Array {
	return Array{Object: p.Send("allowedFiledTypes")}
}

// SetAllowedFileTypes https://developer.apple.com/documentation/appkit/nssavepanel/1534419-allowedfiletypes?language=objc
func (p SavePanel) SetAllowedFileTypes(types Array) {
	p.Send("setAllowedFileTypes:", types)
}

// URL https://developer.apple.com/documentation/appkit/nssavepanel/1534384-url?language=objc
func (p SavePanel) URL() URL {
	return URL{Object: p.Send("URL")}
}

// RunModal https://developer.apple.com/documentation/appkit/nssavepanel/1525357-runmodal?language=objc
func (p SavePanel) RunModal() int {
	return int(p.Send("runModal").Int())
}
