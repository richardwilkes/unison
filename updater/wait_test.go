// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package updater

import (
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/unison/internal/testenv"
)

// holdUntilStdinClosesEnv, when set, turns a run of this test binary into a process that lives until its standard input
// is closed, for TestWaitForPredecessorWithoutAPortWaitsForTheProcess to wait on.
const holdUntilStdinClosesEnv = "UPDATER_TEST_HOLD_UNTIL_STDIN_CLOSES"

// holdPort takes a port and returns it along with the listener holding it. A port allocated by the system is used
// rather than a real handoff port, so these tests cannot interfere with an application the developer is running.
// The listener is closed when the test ends, if the test has not closed it already.
func holdPort(t *testing.T) (listener net.Listener, port int) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { xio.CloseIgnoringErrors(listener) })
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected a TCP address, got %T", listener.Addr())
	}
	return listener, addr.Port
}

// exitedPID returns the identifier of a process that has certainly exited, and been reaped. The -h makes the
// re-executed test binary print its usage and exit before running anything.
func exitedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run", "^$", "-h") //nolint:gosec // Re-executes this test binary
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait() //nolint:errcheck // Only that it has exited matters, not how
	return cmd.Process.Pid
}

// The ordinary case: the application has already exited, so there is nothing to wait for.
func TestWaitForPredecessorReturnsWhenThePortIsFree(t *testing.T) {
	testenv.SkipTimingSensitive(t)
	c := check.New(t)
	listener, port := holdPort(t)
	xio.CloseIgnoringErrors(listener)

	started := time.Now()
	c.NoError(waitForPredecessor(os.Getpid(), port, testApp))
	c.True(time.Since(started) < 5*time.Second, "a free port should be noticed immediately")
}

// The property the whole wait exists for. Relaunching while the port is still held would make the new instance hand its
// arguments to the old one and exit, so the update would look as though it had done nothing at all. That holds whoever
// has the port: a copy the user started during the wait can take it the moment the application exits, and the
// installation must not be replaced under that copy. That case is watched for longer, allowing for a check that waits
// out the moment a socket outlives its process.
func TestWaitForPredecessorWaitsUntilThePortIsReleased(t *testing.T) {
	testenv.SkipTimingSensitive(t)
	for _, one := range []struct {
		name string
		pid  int
		held time.Duration
	}{
		{name: "by the application", pid: os.Getpid(), held: 500 * time.Millisecond},
		{name: "by another copy after the application has exited", pid: exitedPID(t), held: 3 * time.Second},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			listener, port := holdPort(t)

			done := make(chan error, 1)
			go func() { done <- waitForPredecessor(one.pid, port, testApp) }()

			// The wait must still be in progress while the port is held.
			select {
			case err := <-done:
				t.Fatalf("the wait finished while the port was still held: %v", err)
			case <-time.After(one.held):
			}

			xio.CloseIgnoringErrors(listener)

			select {
			case err := <-done:
				c.NoError(err)
			case <-time.After(15 * time.Second):
				t.Fatal("the wait did not finish after the port was released")
			}
		})
	}
}

// An application with no handoff port has only its process to wait on: the helper must not touch the installation while
// that process lives, and must carry on once it has gone. A state carrying no process at all has nothing to wait for.
func TestWaitForPredecessorWithoutAPortWaitsForTheProcess(t *testing.T) {
	if os.Getenv(holdUntilStdinClosesEnv) != "" {
		_, _ = io.Copy(io.Discard, os.Stdin) //nolint:errcheck // Only that standard input has closed matters
		return
	}
	testenv.SkipTimingSensitive(t)
	c := check.New(t)

	started := time.Now()
	c.NoError(waitForPredecessor(0, 0, testApp))
	c.True(time.Since(started) < 5*time.Second, "with no process to watch, there is nothing to wait for")

	//nolint:gosec // Re-executes this test binary
	cmd := exec.Command(os.Args[0], "-test.run", "^TestWaitForPredecessorWithoutAPortWaitsForTheProcess$")
	cmd.Env = append(os.Environ(), holdUntilStdinClosesEnv+"=1")
	stdin, err := cmd.StdinPipe()
	c.NoError(err)
	c.NoError(cmd.Start())
	t.Cleanup(func() {
		xio.CloseIgnoringErrors(stdin)
		_ = cmd.Wait() //nolint:errcheck // Only that it has exited matters, not how
	})

	done := make(chan error, 1)
	go func() { done <- waitForPredecessor(cmd.Process.Pid, 0, testApp) }()

	// The wait must still be in progress while the process lives.
	select {
	case err = <-done:
		t.Fatalf("the wait finished while the process was still running: %v", err)
	case <-time.After(500 * time.Millisecond):
	}

	xio.CloseIgnoringErrors(stdin)
	c.NoError(cmd.Wait())

	select {
	case err = <-done:
		c.NoError(err)
	case <-time.After(15 * time.Second):
		t.Fatal("the wait did not finish after the process exited")
	}
}
