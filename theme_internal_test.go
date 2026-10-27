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
	"github.com/richardwilkes/unison/enums/thememode"
)

// TestThemeColorDeriveBlendFollowsBothColors verifies that a blend follows a change to either of the colors it blends,
// by each mode's own amount, and that a color derived from the blend follows it in turn. This test shares global state
// and therefore must not call t.Parallel.
func TestThemeColorDeriveBlendFollowsBothColors(t *testing.T) {
	c := check.New(t)
	resetTaskQueue() // SetThemeMode queues a ThemeChanged task; keep the shared queue clean for other tests.
	prev := CurrentThemeMode()
	t.Cleanup(func() {
		SetThemeMode(prev)
		resetTaskQueue()
	})
	base := &ThemeColor{Light: RGB(200, 200, 200), Dark: RGB(20, 20, 20)}
	other := &ThemeColor{Light: RGB(0, 100, 200), Dark: RGB(0, 120, 240)}
	blend := base.DeriveBlend(other, 0.5, 0.25)
	on := blend.DeriveOn()
	for _, tc := range []struct {
		change func()
		name   string
	}{
		{name: "as created"},
		{name: "after the base changes", change: func() { *base = ThemeColor{Light: RGB(100, 100, 100), Dark: RGB(60, 60, 60)} }},
		{name: "after the other changes", change: func() { *other = ThemeColor{Light: RGB(200, 0, 0), Dark: RGB(240, 0, 0)} }},
	} {
		if tc.change != nil {
			tc.change()
		}
		SetThemeMode(thememode.Light)
		c.Equal(base.Light.Blend(other.Light, 0.5), blend.GetColor(), tc.name)
		c.Equal(blend.GetColor().On(), on.GetColor(), tc.name)
		SetThemeMode(thememode.Dark)
		c.Equal(base.Dark.Blend(other.Dark, 0.25), blend.GetColor(), tc.name)
		c.Equal(blend.GetColor().On(), on.GetColor(), tc.name)
	}
}
