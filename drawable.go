// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/toolbox/xmath/geom32"

// Drawable represents a drawable object.
type Drawable interface {
	// LogicalSize returns the logical size of this object.
	LogicalSize() geom32.Size

	// DrawInRect draws this object in the given rectangle.
	DrawInRect(canvas *Canvas, rect geom32.Rect, sampling *SamplingOptions, paint *Paint)
}
