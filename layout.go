// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/toolbox/v2/geom"

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
	// StdIconGap is the typical gap between an icon and text.
	StdIconGap = 3
)

// Sizer returns minimum, preferred, and maximum sizes. The hint will contain
// values other than zero for a dimension that has already been determined.
type Sizer func(hint geom.Size) (minSize, prefSize, maxSize geom.Size)

// Layout defines methods that all layouts must provide.
type Layout interface {
	LayoutSizes(target *Panel, hint geom.Size) (minSize, prefSize, maxSize geom.Size)
	PerformLayout(target *Panel)
}

// MaxSize returns the size that is at least as large as DefaultMaxSize in
// both dimensions, but larger if the size that is passed in is larger.
func MaxSize(size geom.Size) geom.Size {
	return geom.Size{
		Width:  max(DefaultMaxSize, size.Width),
		Height: max(DefaultMaxSize, size.Height),
	}
}
