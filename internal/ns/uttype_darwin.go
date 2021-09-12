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

var utTypeClass = objc.Get("UTType")

// UTType https://developer.apple.com/documentation/uniformtypeidentifiers/uttype?language=objc
type UTType struct {
	objc.Object
}

// ImportedTypeWithIdentifier https://developer.apple.com/documentation/uniformtypeidentifiers/uttype/3600610-importedtypewithidentifier?language=objc
func ImportedTypeWithIdentifier(id String) UTType {
	return UTType{Object: utTypeClass.Send("importedTypeWithIdentifier:", id)}
}

// PreferredMIMEType https://developer.apple.com/documentation/uniformtypeidentifiers/uttype/3548211-preferredmimetype?language=objc
func (t UTType) PreferredMIMEType() String {
	return String{Object: t.Send("preferredMIMEType")}
}
