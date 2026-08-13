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
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// CPURenderingEnvKey names the environment variable that forces rendering onto the CPU. When it is set to a true
// value, as understood by strconv.ParseBool (e.g. "1"), no OpenGL call is ever made: windows are created without a
// rendering context and their contents are presented through the platform's CPU blit path instead.
//
// This exists as an escape hatch for machines whose OpenGL stack is broken in a way the automatic fallback cannot
// catch. That fallback only engages when GL setup returns an error, so a driver -- or third-party software that has
// hooked one -- which hangs rather than fails would otherwise wedge the UI thread with no way to get the application
// running at all.
const CPURenderingEnvKey = "UNISON_CPU_RENDERING"

// cpuRenderingActive is true once hardware-accelerated (OpenGL) rendering has been found to be unavailable and the
// process has fallen back to CPU rendering, or once CPURenderingEnvKey has asked for it up front. The fallback is
// sticky and process-wide: once any window fails to obtain a usable OpenGL environment, all subsequent rendering
// happens on the CPU rather than repeatedly re-attempting (and failing) GL setup. Only accessed on the UI thread.
var cpuRenderingActive bool

// IsCPURenderingActive returns true if rendering is being performed on the CPU rather than with hardware acceleration
// (OpenGL), whether because OpenGL was unavailable or because CPURenderingEnvKey asked for it.
func IsCPURenderingActive() bool {
	return cpuRenderingActive
}

// applyCPURenderingEnvRequest turns on CPU rendering if CPURenderingEnvKey asks for it. This runs during startup,
// before any window can exist, so that the OpenGL paths are never entered at all rather than being abandoned partway
// through.
func applyCPURenderingEnvRequest() {
	v, ok := os.LookupEnv(CPURenderingEnvKey)
	if !ok {
		return
	}
	if on, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil && on && !cpuRenderingActive {
		cpuRenderingActive = true
		slog.Info("CPU rendering was requested via the environment; OpenGL will not be used", "var",
			CPURenderingEnvKey)
	}
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
