// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
)

// newTestLink creates a link with a frame rect and returns it along with a pointer to its click count.
func newTestLink() (link *unison.Label, clicks *int) {
	count := 0
	link = unison.NewLink("link", "", "target", nil, func(_ unison.Paneler, _ string) { count++ })
	link.SetFrameRect(geom.NewRect(0, 0, 100, 20))
	return link, &count
}

// TestLinkActivatesOnLeftClick verifies the normal activation path: a left press claims the mouse and a left release
// inside the link invokes the click handler.
func TestLinkActivatesOnLeftClick(t *testing.T) {
	c := check.New(t)
	link, clicks := newTestLink()
	pt := geom.NewPoint(10, 10)
	c.True(link.MouseDownCallback(pt, unison.ButtonLeft, 1, 0))
	c.True(link.MouseUpCallback(pt, unison.ButtonLeft, 0))
	c.Equal(1, *clicks)
}

// TestLinkIgnoresNonLeftButtons verifies that right- and middle-button presses are not claimed and do not invoke the
// click handler, matching how other widgets branch on the button.
func TestLinkIgnoresNonLeftButtons(t *testing.T) {
	c := check.New(t)
	link, clicks := newTestLink()
	pt := geom.NewPoint(10, 10)
	for _, button := range []int{unison.ButtonRight, unison.ButtonMiddle} {
		c.False(link.MouseDownCallback(pt, button, 1, 0))
		c.False(link.MouseDragCallback(pt, button, 0))
		c.False(link.MouseUpCallback(pt, button, 0))
		c.Equal(0, *clicks)
	}
	// The link must still work normally after non-left clicks.
	c.True(link.MouseDownCallback(pt, unison.ButtonLeft, 1, 0))
	c.True(link.MouseUpCallback(pt, unison.ButtonLeft, 0))
	c.Equal(1, *clicks)
}

// TestLinkDoesNotActivateWhenReleasedOutside verifies that dragging off the link before releasing cancels activation.
func TestLinkDoesNotActivateWhenReleasedOutside(t *testing.T) {
	c := check.New(t)
	link, clicks := newTestLink()
	inside := geom.NewPoint(10, 10)
	outside := geom.NewPoint(200, 200)
	c.True(link.MouseDownCallback(inside, unison.ButtonLeft, 1, 0))
	c.True(link.MouseDragCallback(outside, unison.ButtonLeft, 0))
	c.True(link.MouseUpCallback(outside, unison.ButtonLeft, 0))
	c.Equal(0, *clicks)
}
