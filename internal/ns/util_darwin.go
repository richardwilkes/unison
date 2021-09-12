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
	"net/url"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
)

// StringSliceToArray converts a slice of Go strings into an NSArray of NSString.
func StringSliceToArray(slice []string) Array {
	a := MutableArrayWithCapacity(len(slice))
	for _, s := range slice {
		str := StringFromString(s)
		a.AddObject(str)
		str.Release()
	}
	return a.Array
}

// StringArrayToSlice converts an NSArray of NSString into a slice of Go strings.
func StringArrayToSlice(array Array) []string {
	count := array.Count()
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, array.StringAtIndex(i).String())
	}
	return result
}

// URLArrayToStringSlice converts an NSArray of NSURL into a slice of Go strings.
func URLArrayToStringSlice(array Array) []string {
	count := array.Count()
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		u, err := url.Parse(URL{Object: array.ObjectAtIndex(i)}.AbsoluteString())
		if err != nil {
			jot.Warn(errs.NewWithCause("unable to parse URL", err))
			continue
		}
		result = append(result, u.Path)
	}
	return result
}
