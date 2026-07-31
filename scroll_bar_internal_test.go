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

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestScrollBarThumbStaysWithinTrack verifies that the thumb rect never extends past the end of the track, even when
// the proportional thumb size has been clamped up to MinimumThumb. The old forward mapping computed the thumb start
// from the unclamped proportion, so at maximum scroll the clamped thumb stuck out past the track and rendered clipped.
func TestScrollBarThumbStaysWithinTrack(t *testing.T) {
	c := check.New(t)
	s := NewScrollBar(false)
	s.SetFrameRect(geom.NewRect(0, 0, s.MinimumThickness, 100))
	s.SetRange(90, 10, 100) // proportional thumb would be 10, so it clamps to MinimumThumb (16)
	thumb := s.Thumb()
	c.Equal(s.MinimumThumb, thumb.Height)
	c.True(thumb.Bottom() <= 100, "thumb bottom %v must not extend past the track end", thumb.Bottom())

	horizontal := NewScrollBar(true)
	horizontal.SetFrameRect(geom.NewRect(0, 0, 100, horizontal.MinimumThickness))
	horizontal.SetRange(90, 10, 100)
	thumb = horizontal.Thumb()
	c.Equal(horizontal.MinimumThumb, thumb.Width)
	c.True(thumb.Right() <= 100, "thumb right %v must not extend past the track end", thumb.Right())
}

// TestScrollBarThumbRoundTrip verifies that Thumb() and adjustValueForPoint are exact inverses of one another, so the
// thumb does not drift away from the cursor during a drag, including when a border shifts the content rect's origin.
func TestScrollBarThumbRoundTrip(t *testing.T) {
	c := check.New(t)
	for _, withBorder := range []bool{false, true} {
		s := NewScrollBar(false)
		if withBorder {
			s.SetBorder(NewEmptyBorder(geom.NewUniformInsets(5)))
		}
		s.SetFrameRect(geom.NewRect(0, 0, s.MinimumThickness+10, 110))
		for _, value := range []float32{0, 25, 50, 72.5, 90} {
			s.SetRange(value, 10, 100)
			thumb := s.Thumb()
			r := s.ContentRect(false)
			c.True(thumb.Y >= r.Y, "thumb top %v must not precede the track start %v", thumb.Y, r.Y)
			c.True(thumb.Bottom() <= r.Bottom(), "thumb bottom %v must not extend past the track end %v",
				thumb.Bottom(), r.Bottom())
			s.dragOffset = 0
			s.adjustValueForPoint(geom.NewPoint(0, thumb.Y))
			diff := s.value - value
			if diff < 0 {
				diff = -diff
			}
			c.True(diff < 0.001, "value %v drifted to %v after a round trip (border: %v)", value, s.value, withBorder)
		}
	}
}
