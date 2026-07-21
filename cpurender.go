// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "log/slog"

// cpuRenderingActive is true once hardware-accelerated (OpenGL) rendering has been found to be unavailable and the
// process has fallen back to CPU rendering. The fallback is sticky and process-wide: once any window fails to obtain a
// usable OpenGL environment, all subsequent rendering happens on the CPU rather than repeatedly re-attempting (and
// failing) GL setup. Only accessed on the UI thread.
var cpuRenderingActive bool

// IsCPURenderingActive returns true if hardware-accelerated (OpenGL) rendering was unavailable and rendering is being
// performed on the CPU instead.
func IsCPURenderingActive() bool {
	return cpuRenderingActive
}

// fallbackToCPURendering switches the process to CPU rendering. The first time it is called, a warning with the cause
// is emitted to the log; subsequent calls do nothing.
func fallbackToCPURendering(cause error) {
	if !cpuRenderingActive {
		cpuRenderingActive = true
		slog.Warn("hardware-accelerated (OpenGL) rendering is unavailable; falling back to CPU rendering",
			"cause", cause)
	}
}
