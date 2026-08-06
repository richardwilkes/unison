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
	"slices"
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"golang.org/x/image/draw"
)

// The minimum and maximum logical sizes permitted for cursors. X11 and Windows cursor planes commonly cap out somewhere
// between 64 and 256 pixels, so the upper bound leaves room for a 2x backing scale.
const (
	// MinCursorSize is the minimum logical size a cursor may be built at.
	MinCursorSize = 8
	// MaxCursorSize is the maximum logical size a cursor may be built at.
	MaxCursorSize = 128
)

var (
	arrowCursor               *Cursor
	moveCursor                *Cursor
	pointingCursor            *Cursor
	openHandCursor            *Cursor
	closedHandCursor          *Cursor
	resizeHorizontalCursor    *Cursor
	resizeLeftDiagonalCursor  *Cursor
	resizeRightDiagonalCursor *Cursor
	resizeVerticalCursor      *Cursor
	textCursor                *Cursor
)

var cursorList []*Cursor

var (
	cursorSizeLock sync.RWMutex
	cursorSize     = DefaultCursorSize()
	// builtCursorSettings holds the settings the built-in cursors were last built with. nil when none are built. Only
	// touched on the UI thread.
	builtCursorSettings    *cursorSettings
	cursorChangedCallbacks []*func()
)

// Cursor provides a graphical cursor for the mouse location.
type Cursor struct {
	cursor apiNativeCursor
}

// cursorSettings captures the inputs the built-in cursors are rasterized from. It is comparable, so determining
// whether the built cursors have gone stale is a simple equality check.
type cursorSettings struct {
	fg   Color
	bg   Color
	size geom.Size
}

// currentCursorSettings returns the settings a cursor built right now would use.
func currentCursorSettings() cursorSettings {
	return cursorSettings{
		fg:   ThemeCursorForeground.GetColor(),
		bg:   ThemeCursorBackground.GetColor(),
		size: CursorSize(),
	}
}

// DefaultCursorSize returns the default size for cursors. This is the initial value of CursorSize().
func DefaultCursorSize() geom.Size {
	return geom.NewSize(24, 24)
}

// CursorSize returns the logical size the built-in cursors, as well as those created by NewThemedCursorFromSVG, are
// built at. It is safe to call from any goroutine.
func CursorSize() geom.Size {
	cursorSizeLock.RLock()
	defer cursorSizeLock.RUnlock()
	return cursorSize
}

// SetCursorSize sets the logical size the built-in cursors, as well as those created by NewThemedCursorFromSVG, are
// built at. The size is rounded to whole points and clamped to a usable range. If this results in a change, the
// built-in cursors are rebuilt and any open windows are updated to use them. It is safe to call from any goroutine.
func SetCursorSize(size geom.Size) {
	size = clampCursorSize(size)
	cursorSizeLock.Lock()
	changed := cursorSize != size
	if changed {
		cursorSize = size
	}
	cursorSizeLock.Unlock()
	if changed {
		InvokeTask(ThemeChanged)
	}
}

// clampCursorSize rounds the size to whole points and constrains it to the range cursors may be built at.
func clampCursorSize(size geom.Size) geom.Size {
	return geom.NewSize(min(max(xmath.Round(size.Width), MinCursorSize), MaxCursorSize),
		min(max(xmath.Round(size.Height), MinCursorSize), MaxCursorSize))
}

// ArrowCursor returns the standard arrow cursor.
func ArrowCursor() *Cursor {
	return retrieveCursorWithHotSpot(CursorArrowSVG, &arrowCursor, geom.NewPoint(8.0/24, 4.0/24))
}

// PointingCursor returns the standard pointing cursor.
func PointingCursor() *Cursor {
	return retrieveCursorWithHotSpot(CursorHandPointingSVG, &pointingCursor, geom.NewPoint(9.0/24, 5.0/24))
}

// OpenHandCursor returns the standard open hand cursor.
func OpenHandCursor() *Cursor {
	return retrieveCursor(CursorHandOpenSVG, &openHandCursor)
}

// ClosedHandCursor returns the standard closed hand cursor.
func ClosedHandCursor() *Cursor {
	return retrieveCursor(CursorHandClosedSVG, &closedHandCursor)
}

// TextCursor returns the standard text cursor.
func TextCursor() *Cursor {
	return retrieveCursor(CursorTextSVG, &textCursor)
}

// MoveCursor returns the standard move cursor.
func MoveCursor() *Cursor {
	return retrieveCursor(CursorMoveSVG, &moveCursor)
}

// ResizeHorizontalCursor returns the standard horizontal resize cursor.
func ResizeHorizontalCursor() *Cursor {
	return retrieveCursor(CursorResizeHorizontalSVG, &resizeHorizontalCursor)
}

// ResizeLeftDiagonalCursor returns the standard left diagonal resize cursor.
func ResizeLeftDiagonalCursor() *Cursor {
	return retrieveCursor(CursorResizeLeftDiagonalSVG, &resizeLeftDiagonalCursor)
}

// ResizeRightDiagonalCursor returns the standard right diagonal resize cursor.
func ResizeRightDiagonalCursor() *Cursor {
	return retrieveCursor(CursorResizeRightDiagonalSVG, &resizeRightDiagonalCursor)
}

// ResizeVerticalCursor returns the standard vertical resize cursor.
func ResizeVerticalCursor() *Cursor {
	return retrieveCursor(CursorResizeVerticalSVG, &resizeVerticalCursor)
}

func retrieveCursor(svg *SVG, cursor **Cursor) *Cursor {
	return retrieveCursorWithHotSpot(svg, cursor, geom.NewPoint(0.5, 0.5))
}

func retrieveCursorWithHotSpot(svg *SVG, cursor **Cursor, relativeHotSpot geom.Point) *Cursor {
	syncBuiltInCursors()
	if *cursor == nil {
		s := currentCursorSettings()
		*cursor = newThemedCursor(svg, relativeHotSpot, s)
		if builtCursorSettings == nil {
			builtCursorSettings = &s
		}
	}
	return *cursor
}

// builtInCursors returns the addresses of the variables holding the built-in cursors.
func builtInCursors() []**Cursor {
	return []**Cursor{
		&arrowCursor,
		&moveCursor,
		&pointingCursor,
		&openHandCursor,
		&closedHandCursor,
		&resizeHorizontalCursor,
		&resizeLeftDiagonalCursor,
		&resizeRightDiagonalCursor,
		&resizeVerticalCursor,
		&textCursor,
	}
}

// syncBuiltInCursors destroys the built-in cursors if the cursor settings have changed since they were built, so the
// next retrieval rebuilds them, and notifies any registered cursor change callbacks. The notification lives here, at
// the single point every rebuild passes through, because a rebuild is not always triggered by ThemeChanged(): cursor
// retrieval also syncs, so a window resolving its cursor can perform the rebuild before a queued ThemeChanged() runs,
// and callbacks fired only from there would be silently skipped. Returns true if anything was destroyed. UI thread
// only.
func syncBuiltInCursors() bool {
	if builtCursorSettings == nil || *builtCursorSettings == currentCursorSettings() {
		return false
	}
	// Nil the singletons and clear the record BEFORE destroying: Destroy() reaches Window.adjustToCursorChange(),
	// whose Windows apply path falls back to ArrowCursor() when the window's cursor is nil, reentering retrieval
	// mid-destroy. With everything already nilled, that reentry builds a fresh arrow at the new settings.
	builtCursorSettings = nil
	var stale []*Cursor
	for _, p := range builtInCursors() {
		if *p != nil {
			stale = append(stale, *p)
			*p = nil
		}
	}
	if len(stale) == 0 {
		return false
	}
	for _, c := range stale {
		c.Destroy()
	}
	for _, f := range slices.Clone(cursorChangedCallbacks) {
		SafeCall(*f)
	}
	return true
}

// CursorChangedCallback registers f to be called on the UI thread after the built-in cursors have been rebuilt due to
// a size or color change, but before open windows have their cursors refreshed, so replacement cursors created within
// f are picked up immediately. The returned function unregisters f.
func CursorChangedCallback(f func()) func() {
	p := &f
	cursorChangedCallbacks = append(cursorChangedCallbacks, p)
	return func() {
		cursorChangedCallbacks = slices.DeleteFunc(cursorChangedCallbacks, func(other *func()) bool {
			return other == p
		})
	}
}

// cursorSource holds everything needed to produce a cursor's pixels at an arbitrary backing scale. svg is non-nil for
// vector-backed cursors, which re-rasterize at exactly the requested scale; img is the fallback for image-backed
// cursors, which can only be resampled.
type cursorSource struct {
	svg     *SVG
	inks    map[Color]Ink // ink substitutions applied when rasterizing from svg; nil for none
	img     *image.NRGBA  // non-nil only when svg is nil
	size    geom.Size     // logical size in points
	hotSpot geom.Point    // in logical points (absolute)
}

// render returns the cursor's pixels at the given backing scale.
func (s *cursorSource) render(scale float32) (*image.NRGBA, error) {
	if s.svg == nil {
		return resampleNRGBA(s.img, s.size.Mul(scale).Ceil()), nil
	}
	img, err := newImageFromDrawingAtScale(int(s.size.Width), int(s.size.Height), scale, func(gc *Canvas) {
		s.svg.DrawInRectPreservingAspectRatioWithReplacementInks(gc,
			geom.NewRect(0, 0, s.size.Width, s.size.Height), nil, s.inks)
	})
	if err != nil {
		return nil, err
	}
	return img.ToNRGBA()
}

// resampleNRGBA returns img resampled to the given pixel size, or img itself when it is already that size.
func resampleNRGBA(img *image.NRGBA, size geom.Size) *image.NRGBA {
	width := max(int(size.Width), 1)
	height := max(int(size.Height), 1)
	if img.Rect.Dx() == width && img.Rect.Dy() == height {
		return img
	}
	dstRect := image.Rect(0, 0, width, height)
	dst := image.NewNRGBA(dstRect)
	draw.CatmullRom.Scale(dst, dstRect, img, img.Bounds(), draw.Over, nil)
	return dst
}

// cursorInkReplacements returns the ink substitutions to use when rasterizing a themed cursor from its SVG. Pure black
// in the source art is the cursor's foreground, i.e. its body and linework, while pure white is its background, i.e.
// its outline and halo. The colors are resolved at build time and captured here, so the per-monitor rasters a single
// cursor renders lazily cannot mix pre- and post-theme-flip colors.
func cursorInkReplacements(s cursorSettings) map[Color]Ink {
	return map[Color]Ink{
		Black: s.fg,
		White: s.bg,
	}
}

func newThemedCursor(svg *SVG, relativeHotSpot geom.Point, s cursorSettings) *Cursor {
	return apiNewCursor(&cursorSource{
		svg:     svg,
		inks:    cursorInkReplacements(s),
		size:    s.size,
		hotSpot: geom.NewPoint(relativeHotSpot.X*s.size.Width, relativeHotSpot.Y*s.size.Height),
	})
}

// NewThemedCursorFromSVG creates a cursor from svg, rasterized at the current CursorSize() and drawn with the current
// ThemeCursorForeground/ThemeCursorBackground colors. relativeHotSpot is expressed as fractions of the cursor's size:
// (0,0) is the upper-left corner, (0.5,0.5) the center. The caller owns the result: register a CursorChangedCallback,
// and when it fires, Destroy this cursor and build a replacement. UI thread only.
func NewThemedCursorFromSVG(svg *SVG, relativeHotSpot geom.Point) *Cursor {
	return newThemedCursor(svg, relativeHotSpot, currentCursorSettings())
}

// NewCursorFromSVG creates a new custom cursor from a SVG. The SVG is rasterized at each display scale as needed and
// is drawn with its own fills and strokes. hotSpot is in logical points, measured from the upper-left corner of the
// cursor, and is clamped to lie within the cursor. Use NewThemedCursorFromSVG instead for a cursor that follows the
// cursor size and color settings. UI thread only.
func NewCursorFromSVG(svg *SVG, hotSpot geom.Point, size geom.Size) *Cursor {
	hotSpot.X = min(max(hotSpot.X, 0), size.Width-1)
	hotSpot.Y = min(max(hotSpot.Y, 0), size.Height-1)
	return apiNewCursor(&cursorSource{
		svg:     svg,
		size:    size,
		hotSpot: hotSpot,
	})
}

// NewCursor creates a new custom cursor from an image. UI thread only.
func NewCursor(img *Image, hotSpot geom.Point) *Cursor {
	logicalSize := img.LogicalSize()
	hotSpot.X = min(max(hotSpot.X, 0), logicalSize.Width-1)
	hotSpot.Y = min(max(hotSpot.Y, 0), logicalSize.Height-1)
	nrgba, err := img.ToNRGBA()
	if err != nil {
		errs.Log(err)
		return nil
	}
	return apiNewCursor(&cursorSource{
		img:     nrgba,
		size:    logicalSize,
		hotSpot: hotSpot,
	})
}

// Destroy releases the resources associated with the cursor.
func (c *Cursor) Destroy() {
	if c == nil {
		return
	}
	// Clear any built-in that refers to this cursor first, so neither the window adjustment below nor anything it
	// reenters can hand out a pointer to a cursor that is being destroyed.
	for _, p := range builtInCursors() {
		if *p == c {
			*p = nil
		}
	}
	for _, w := range windowList {
		if w.cursor == c {
			w.cursor = nil
			w.adjustToCursorChange()
		}
	}
	cursorList = slices.DeleteFunc(cursorList, func(cur *Cursor) bool { return cur == c })
	c.apiDestroy()
}
