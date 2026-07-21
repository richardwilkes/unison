// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/canvas/raster"
)

// apiPresentCPUPixels displays a CPU-rendered frame by handing the pixels to the content view's backing layer.
func (w *Window) apiPresentCPUPixels(pixels *raster.Pixmap) {
	size := w.ContentRect().Size
	w.wnd.view.SetLayerContentsRGBAPremul(pixels.RGBA8888Bytes(), int(size.Width), int(size.Height),
		int(pixels.Width), int(pixels.Height))
}
