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
	"image"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// Everything in this file is deliberately restricted to the pure-CPU portions of the cursor machinery: no native cursor
// is ever created, no window is ever made, and the application is never started, so these tests are safe to run in a
// headless environment. Each test restores any package state it mutates, so test ordering doesn't matter.

// newCursorTestNRGBA returns a solid-color NRGBA image of the given pixel size.
func newCursorTestNRGBA(width, height int, p svgTestPixel) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = p.r
		img.Pix[i+1] = p.g
		img.Pix[i+2] = p.b
		img.Pix[i+3] = p.a
	}
	return img
}

// cursorPixelSet returns the set of exact pixel values present in an image. Working with the set of values present,
// rather than counts or positions, keeps the assertions insensitive to anti-aliasing.
func cursorPixelSet(img *image.NRGBA) map[svgTestPixel]bool {
	present := make(map[svgTestPixel]bool)
	for y := range img.Rect.Dy() {
		row := img.Pix[y*img.Stride : y*img.Stride+img.Rect.Dx()*4]
		for x := range img.Rect.Dx() {
			present[svgTestPixel{r: row[x*4], g: row[x*4+1], b: row[x*4+2], a: row[x*4+3]}] = true
		}
	}
	return present
}

func TestClampCursorSize(t *testing.T) {
	c := check.New(t)

	c.Equal(geom.NewSize(24, 24), clampCursorSize(geom.NewSize(24, 24)), "an in-range whole size passes through")
	c.Equal(geom.NewSize(23, 24), clampCursorSize(geom.NewSize(23.4, 23.6)), "fractional sizes round to whole points")
	c.Equal(geom.NewSize(MinCursorSize, MinCursorSize), clampCursorSize(geom.NewSize(5, 5)),
		"an undersized cursor is raised to the minimum")
	c.Equal(geom.NewSize(MaxCursorSize, MaxCursorSize), clampCursorSize(geom.NewSize(200, 200)),
		"an oversized cursor is lowered to the maximum")
	c.Equal(geom.NewSize(MinCursorSize, MinCursorSize), clampCursorSize(geom.NewSize(-10, 0)),
		"zero and negative sizes are raised to the minimum")
	c.Equal(geom.NewSize(MinCursorSize, MaxCursorSize), clampCursorSize(geom.NewSize(1, 500)),
		"each axis is clamped independently")

	// Rounding happens before clamping, so a size that only reaches the minimum by rounding is accepted as-is rather
	// than being treated as undersized.
	c.Equal(geom.NewSize(MinCursorSize, MinCursorSize), clampCursorSize(geom.NewSize(7.6, 7.6)),
		"a size that rounds up to the minimum is not clamped further")
}

func TestCursorSizeIsAlwaysClamped(t *testing.T) {
	c := check.New(t)

	c.Equal(geom.NewSize(24, 24), DefaultCursorSize(), "the default cursor size is 24x24 points")
	size := CursorSize()
	c.Equal(clampCursorSize(size), size, "the stored cursor size is always a clamped value")

	// Setting the size it already has is a no-op, so nothing is queued onto the (non-running) UI thread.
	SetCursorSize(size)
	c.Equal(size, CursorSize(), "setting the current size leaves it unchanged")
}

func TestCursorSettingsStaleness(t *testing.T) {
	c := check.New(t)

	c.Equal(currentCursorSettings(), currentCursorSettings(), "unchanged state produces equal settings")

	// The defaults use the same color for light and dark mode on purpose, since cursors conventionally do not invert
	// with the system appearance. That means a dark mode flip alone can never make the built cursors stale.
	fg := DefaultThemeCursorForeground()
	c.Equal(fg.Light, fg.Dark, "the default cursor foreground does not vary with the theme mode")
	bg := DefaultThemeCursorBackground()
	c.Equal(bg.Light, bg.Dark, "the default cursor background does not vary with the theme mode")

	saved := *ThemeCursorForeground
	defer func() { *ThemeCursorForeground = saved }()

	before := currentCursorSettings()
	// Both halves are changed, since GetColor() returns one or the other depending on the OS's dark mode state and the
	// tests must not depend on which mode the machine running them happens to be in.
	ThemeCursorForeground.Light = Red
	ThemeCursorForeground.Dark = Red
	after := currentCursorSettings()

	c.NotEqual(before, after, "a foreground color change makes the built cursors stale")
	c.Equal(Red, after.fg, "the new foreground color is picked up")
	c.Equal(before.bg, after.bg, "the background color is untouched")
	c.Equal(before.size, after.size, "the size is untouched")

	*ThemeCursorForeground = saved
	c.Equal(before, currentCursorSettings(), "restoring the color restores the settings")
}

func TestSyncBuiltInCursorsEarlyOuts(t *testing.T) {
	c := check.New(t)

	saved := builtCursorSettings
	defer func() { builtCursorSettings = saved }()

	builtCursorSettings = nil
	c.False(syncBuiltInCursors(), "nothing has been built, so there is nothing to destroy")
	c.Nil(builtCursorSettings, "the record stays empty")

	s := currentCursorSettings()
	builtCursorSettings = &s
	c.False(syncBuiltInCursors(), "the built cursors still match the current settings")
	c.NotNil(builtCursorSettings, "the record is left intact when nothing has gone stale")

	// Neither early-out may construct a cursor, which is what keeps this test headless-safe.
	for _, p := range builtInCursors() {
		c.Nil(*p, "no built-in cursor may be constructed by an early-out")
	}
}

func TestCursorChangedCallback(t *testing.T) {
	c := check.New(t)

	saved := cursorChangedCallbacks
	defer func() { cursorChangedCallbacks = saved }()
	cursorChangedCallbacks = nil

	var firstCalls, secondCalls int
	unregisterFirst := CursorChangedCallback(func() { firstCalls++ })
	c.Equal(1, len(cursorChangedCallbacks), "registering appends a callback")
	unregisterSecond := CursorChangedCallback(func() { secondCalls++ })
	c.Equal(2, len(cursorChangedCallbacks), "registering again appends another callback")

	unregisterFirst()
	c.Equal(1, len(cursorChangedCallbacks), "unregistering removes the callback")
	for _, p := range cursorChangedCallbacks {
		(*p)()
	}
	c.Equal(0, firstCalls, "the unregistered callback is not called")
	c.Equal(1, secondCalls, "the callback that is still registered is the one that survived")

	// Unregistering is keyed off the pointer handed out at registration, so a second call simply finds nothing to
	// delete rather than panicking or taking someone else's entry with it.
	c.NotPanics(unregisterFirst, "unregistering twice does not panic")
	c.Equal(1, len(cursorChangedCallbacks), "unregistering twice does not remove another callback")

	unregisterSecond()
	c.Equal(0, len(cursorChangedCallbacks), "the last unregister empties the list")

	// The same func value registered twice yields two independent registrations, since func values are not comparable
	// and each registration is tracked by its own pointer.
	same := func() {}
	unregisterA := CursorChangedCallback(same)
	unregisterB := CursorChangedCallback(same)
	c.Equal(2, len(cursorChangedCallbacks), "the same func registered twice occupies two slots")
	unregisterA()
	c.Equal(1, len(cursorChangedCallbacks), "identical func values unregister independently")
	unregisterB()
	c.Equal(0, len(cursorChangedCallbacks), "the second registration can still be unregistered")
}

func TestCursorInkReplacements(t *testing.T) {
	c := check.New(t)

	replacements := cursorInkReplacements(cursorSettings{fg: Red, bg: Green, size: geom.NewSize(24, 24)})
	c.Equal(2, len(replacements), "exactly the two colors the cursor art convention uses are substituted")
	c.Equal(Ink(Red), replacements[Black], "pure black in the source art is the cursor's foreground")
	c.Equal(Ink(Green), replacements[White], "pure white in the source art is the cursor's background")
	_, exists := replacements[Transparent]
	c.False(exists, "transparent is never substituted, so cursors keep their transparent areas")
}

func TestCursorSourceRenderFromSVG(t *testing.T) {
	c := check.New(t)

	src := &cursorSource{
		svg:  CursorArrowSVG,
		inks: svgTestReplacements,
		size: geom.NewSize(24, 24),
	}

	// A vector-backed cursor re-rasterizes at exactly the requested scale, including fractional scales, rather than
	// resampling, so the pixel dimensions are the logical size scaled and rounded up.
	for _, one := range []struct {
		scale float32
		want  int
	}{
		{scale: 1, want: 24},
		{scale: 2, want: 48},
		{scale: 1.5, want: 36},
	} {
		img, err := src.render(one.scale)
		c.NoError(err)
		c.NotNil(img)
		c.Equal(one.want, img.Rect.Dx(), "width at scale %v", one.scale)
		c.Equal(one.want, img.Rect.Dy(), "height at scale %v", one.scale)
	}

	// The ink substitutions must reach the rasterizer, so both replacement colors appear and neither of the source
	// colors survives. Anti-aliased edges blend the replacements, which can never reproduce the originals.
	img, err := src.render(2)
	c.NoError(err)
	present := cursorPixelSet(img)
	c.True(present[svgTestRed], "the cursor's foreground picks up the black replacement")
	c.True(present[svgTestGreen], "the cursor's background picks up the white replacement")
	c.False(present[svgTestBlack], "no pure black pixel may survive the replacement")
	c.False(present[svgTestWhite], "no pure white pixel may survive the replacement")
}

func TestCursorSourceRenderFromImage(t *testing.T) {
	c := check.New(t)

	src := &cursorSource{
		img:  newCursorTestNRGBA(10, 10, svgTestRed),
		size: geom.NewSize(10, 10),
	}

	// An image-backed cursor has no vector data to re-rasterize, so it can only be resampled to the pixel size the
	// scale calls for.
	img, err := src.render(2)
	c.NoError(err)
	c.NotNil(img)
	c.Equal(20, img.Rect.Dx(), "width at scale 2")
	c.Equal(20, img.Rect.Dy(), "height at scale 2")

	img, err = src.render(1)
	c.NoError(err)
	c.NotNil(img)
	c.Equal(10, img.Rect.Dx(), "width at scale 1")
	c.Equal(10, img.Rect.Dy(), "height at scale 1")
}

func TestResampleNRGBA(t *testing.T) {
	c := check.New(t)

	src := newCursorTestNRGBA(10, 10, svgTestRed)

	// A request for the size the image already has must not copy or resample it.
	c.True(resampleNRGBA(src, geom.NewSize(10, 10)) == src, "an image already at the requested size is returned as-is")

	// The target size is truncated to whole pixels, so a fractional request lands on the pixel count that fits.
	for _, one := range []struct {
		size          geom.Size
		width, height int
	}{
		{size: geom.NewSize(20, 20), width: 20, height: 20},
		{size: geom.NewSize(5, 40), width: 5, height: 40},
		{size: geom.NewSize(20.9, 20.9), width: 20, height: 20},
		// A degenerate target is floored at a single pixel rather than producing an empty image.
		{size: geom.NewSize(0, 0), width: 1, height: 1},
		{size: geom.NewSize(-5, -5), width: 1, height: 1},
		{size: geom.NewSize(0.5, 8), width: 1, height: 8},
	} {
		dst := resampleNRGBA(src, one.size)
		c.NotNil(dst)
		c.True(dst != src, "a differently sized result is a new image (%v)", one.size)
		c.Equal(one.width, dst.Rect.Dx(), "width for %v", one.size)
		c.Equal(one.height, dst.Rect.Dy(), "height for %v", one.size)
	}
}
