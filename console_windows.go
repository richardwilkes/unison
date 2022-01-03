// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"os"
	"syscall"

	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

var (
	preservedStdin  *os.File
	preservedStdout *os.File
	preservedStderr *os.File
)

func attachConsole() {
	// Squirrel away the original stdin/stdout/stderr to prevent them from being garbage collected.
	preservedStdin = os.Stdin
	preservedStdout = os.Stdout
	preservedStderr = os.Stderr

	// Get the existing stdin/stdout/stderr handles.
	stdin, _ := syscall.GetStdHandle(syscall.STD_INPUT_HANDLE)
	stdout, _ := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	stderr, _ := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE)

	// Attach the console if any of stdin/stdout/stderr are currently unattached, loading the newly-found handles for
	// the unattached ones.
	var console syscall.Handle
	if stdin == 0 || stdout == 0 || stderr == 0 {
		if w32.AttachConsole(w32.AttachParentProcessID) {
			if stdin == 0 {
				stdin, _ = syscall.GetStdHandle(syscall.STD_INPUT_HANDLE)
			}
			if stdout == 0 {
				stdout, _ = syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
				console = stdout
			}
			if stderr == 0 {
				stderr, _ = syscall.GetStdHandle(syscall.STD_ERROR_HANDLE)
				console = stderr
			}
		}
	}

	// Set the console mode, if necessary, to ensure LF is turned into CRLF on output
	if console != 0 {
		var mode uint32
		if err := windows.GetConsoleMode(windows.Handle(console), &mode); err == nil {
			windows.SetConsoleMode(windows.Handle(console), mode&^windows.DISABLE_NEWLINE_AUTO_RETURN)
		}
	}

	// Setup the new stdin/stdout/stderr file handles
	if stdin != 0 {
		os.Stdin = os.NewFile(uintptr(stdin), "stdin")
	}
	if stdout != 0 {
		os.Stdout = os.NewFile(uintptr(stdout), "stdout")
	}
	if stderr != 0 {
		os.Stderr = os.NewFile(uintptr(stderr), "stderr")
	}
}
