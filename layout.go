// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

const (
	// DefaultMaxSize is the default size that should be used for a maximum dimension if the target has no real
	// preference and can be expanded beyond its preferred size. This is intentionally not something very large to allow
	// basic math operations an opportunity to succeed when laying out panels. It is perfectly acceptable to use a
	// larger value than this, however, if that makes sense for your specific target.
	DefaultMaxSize = 10000
	// StdHSpacing is the typical spacing between columns.
	StdHSpacing = 8
	// StdVSpacing is the typical spacing between rows.
	StdVSpacing = 4
)

// Sizer returns minimum, preferred, and maximum sizes. The hint will contain
// values other than zero for a dimension that has already been determined.
type Sizer func(hint Size) (minSize, prefSize, maxSize Size)

// Layout defines methods that all layouts must provide.
type Layout interface {
	LayoutSizes(target *Panel, hint Size) (minSize, prefSize, maxSize Size)
	PerformLayout(target *Panel)
}

// MaxSize returns the size that is at least as large as DefaultMaxSize in
// both dimensions, but larger if the size that is passed in is larger.
func MaxSize(size Size) Size {
	return Size{
		Width:  max(DefaultMaxSize, size.Width),
		Height: max(DefaultMaxSize, size.Height),
	}
}
