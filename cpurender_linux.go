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
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/x11"
)

// apiPresentCPUPixels displays a CPU-rendered frame by uploading the pixels to the window with PutImage.
func (w *Window) apiPresentCPUPixels(pixels *raster.Pixmap) {
	if w.wnd.gc == 0 {
		if w.wnd.gc = x11Conn.CreateGC(x11.DrawableID(w.wnd.id), 0, nil); w.wnd.gc == 0 {
			errs.Log(errs.New("failed to create X11 graphics context for CPU rendering"))
			return
		}
	}
	x11Conn.PutImageRGBAPremul(x11.DrawableID(w.wnd.id), w.wnd.gc, 0, 0, pixels.Width, pixels.Height,
		pixels.RowPixels, pixels.Pix, w.wnd.depth)
	x11Conn.Flush()
}
