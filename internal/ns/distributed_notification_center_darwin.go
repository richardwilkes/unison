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

var distributedNotificationCenterClass = objc.Get("NSDistributedNotificationCenter")

// DistributedNotificationCenter https://developer.apple.com/documentation/foundation/nsdistributednotificationcenter/
type DistributedNotificationCenter struct {
	objc.Object
}

// DefaultCenter https://developer.apple.com/documentation/foundation/nsdistributednotificationcenter/1412063-defaultcenter
func DefaultCenter() DistributedNotificationCenter {
	return DistributedNotificationCenter{Object: distributedNotificationCenterClass.Send("defaultCenter")}
}

// AddObserver https://developer.apple.com/documentation/foundation/nsdistributednotificationcenter/1414151-addobserver
func (c DistributedNotificationCenter) AddObserver(delegate objc.Object, selector objc.Selector, name string) {
	str := StringFromString(name)
	defer str.Release()
	c.Send("addObserver:selector:name:object:", delegate, selector, str, nil)
}
