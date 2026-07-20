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
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/internal/x11"
)

// fakeGLX implements glxAPI without a running X server, returning the canned context and window values and recording
// every destroy call that reaches the underlying API.
type fakeGLX struct {
	context           x11.GLXContext
	destroyedContexts []x11.GLXContext
	destroyedWindows  []x11.GLXWindow
	window            x11.GLXWindow
	closed            int
}

func (f *fakeGLX) Visual() x11.VisualID                          { return 0 }
func (f *fakeGLX) Depth() byte                                   { return 0 }
func (f *fakeGLX) CreateContext() x11.GLXContext                 { return f.context }
func (f *fakeGLX) CreateWindow(_ x11.WindowID) x11.GLXWindow     { return f.window }
func (f *fakeGLX) MakeCurrent(_ x11.GLXWindow, _ x11.GLXContext) {}
func (f *fakeGLX) ReleaseCurrent()                               {}
func (f *fakeGLX) SwapBuffers(_ x11.GLXWindow)                   {}

func (f *fakeGLX) DestroyWindow(window x11.GLXWindow) {
	f.destroyedWindows = append(f.destroyedWindows, window)
}

func (f *fakeGLX) DestroyContext(context x11.GLXContext) {
	f.destroyedContexts = append(f.destroyedContexts, context)
}

func (f *fakeGLX) Close() {
	f.closed++
}

// TestAPICreateWindowFailureDestroysContextOnce is the regression test for apiCreate's create-window failure path
// having destroyed the GLX context but left c.context set, so that apiDestroy — which NewWindow's error path always
// invokes — destroyed the same context a second time and raised GLXBadContext.
func TestAPICreateWindowFailureDestroysContextOnce(t *testing.T) {
	c := check.New(t)

	var backing int
	ctx := x11.GLXContext(unsafe.Pointer(&backing))
	fake := &fakeGLX{context: ctx} // window is left 0 so CreateWindow reports failure
	glc := &apiGLContext{glx: fake}
	c.HasError(glc.apiCreate(&Window{wnd: &apiWindow{}}))

	// The failure path must destroy the context it created, exactly once, and drop the reference to it.
	c.Equal([]x11.GLXContext{ctx}, fake.destroyedContexts)
	c.Nil(glc.context)

	// NewWindow's error path then calls apiDestroy, which must not hand the already-destroyed context back to GLX.
	glc.apiDestroy()
	c.Equal([]x11.GLXContext{ctx}, fake.destroyedContexts)
	c.Equal(1, fake.closed)
}

// TestAPIDestroyAfterSuccessfulCreate verifies the normal teardown still destroys each resource exactly once.
func TestAPIDestroyAfterSuccessfulCreate(t *testing.T) {
	c := check.New(t)

	var backing int
	ctx := x11.GLXContext(unsafe.Pointer(&backing))
	fake := &fakeGLX{context: ctx, window: 42}
	glc := &apiGLContext{glx: fake}
	c.NoError(glc.apiCreate(&Window{wnd: &apiWindow{}}))
	c.Equal(ctx, glc.context)
	c.Equal(x11.GLXWindow(42), glc.window)

	glc.apiDestroy()
	c.Equal([]x11.GLXWindow{42}, fake.destroyedWindows)
	c.Equal([]x11.GLXContext{ctx}, fake.destroyedContexts)
	c.Equal(1, fake.closed)
	c.Nil(glc.context)
	c.Equal(x11.GLXWindow(0), glc.window)
	c.Nil(glc.glx)

	// A second apiDestroy (e.g. Dispose after a failed NewWindow already cleaned up) must be a no-op.
	glc.apiDestroy()
	c.Equal(1, len(fake.destroyedWindows))
	c.Equal(1, len(fake.destroyedContexts))
	c.Equal(1, fake.closed)
}
