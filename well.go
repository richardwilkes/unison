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
	"context"
	"time"

	"github.com/richardwilkes/toolbox/log/jot"
)

// WellMask is used to limit the types of ink permitted in the ink well.
type WellMask uint8

// Possible ink well masks.
const (
	ColorWellMask WellMask = 1 << iota
	GradientWellMask
	PatternWellMask
)

// DefaultWellTheme holds the default WellTheme values for Wells. Modifying this data will not alter existing Wells, but
// will alter any Wells created in the future.
var DefaultWellTheme = WellTheme{
	BackgroundInk:      ControlColor,
	EdgeInk:            ControlEdgeColor,
	SelectionInk:       SelectionColor,
	ImageScale:         0.5,
	ContentSize:        20,
	CornerRadius:       4,
	ClickAnimationTime: 100 * time.Millisecond,
	ImageLoadTimeout:   30 * time.Second,
	Mask:               ColorWellMask | GradientWellMask | PatternWellMask,
}

// WellTheme holds theming data for a Well.
type WellTheme struct {
	BackgroundInk      Ink
	EdgeInk            Ink
	SelectionInk       Ink
	ClickAnimationTime time.Duration
	ImageLoadTimeout   time.Duration
	ImageScale         float32
	ContentSize        float32
	CornerRadius       float32
	Mask               WellMask
}

// Well represents a control that holds and lets a user choose an ink.
type Well struct {
	Panel
	WellTheme
	ImageFromSpecCallback func(ctx context.Context, filePathOrURL string, scale float32) (*Image, error)
	InkChangedCallback    func()
	ClickCallback         func()
	ValidateImageCallback func(*Image) *Image
	ink                   Ink
	Pressed               bool
}

// NewWell creates a new Well.
func NewWell() *Well {
	well := &Well{
		WellTheme: DefaultWellTheme,
		ink:       Black,
	}
	well.Self = well
	well.SetFocusable(true)
	well.SetSizer(well.DefaultSizes)
	well.ImageFromSpecCallback = NewImageFromFilePathOrURLWithContext
	well.ClickCallback = well.DefaultClick
	well.DrawCallback = well.DefaultDraw
	well.GainedFocusCallback = well.DefaultFocusGained
	well.LostFocusCallback = well.MarkForRedraw
	well.MouseDownCallback = well.DefaultMouseDown
	well.MouseDragCallback = well.DefaultMouseDrag
	well.MouseUpCallback = well.DefaultMouseUp
	well.KeyDownCallback = well.DefaultKeyDown
	well.FileDropCallback = well.DefaultFileDrop
	well.UpdateCursorCallback = well.DefaultUpdateCursor
	return well
}

// Ink returns the well's ink.
func (w *Well) Ink() Ink {
	return w.ink
}

// SetInk sets the ink well's ink.
func (w *Well) SetInk(ink Ink) {
	if ink == nil {
		ink = Transparent
	}
	switch ink.(type) {
	case Color, *Color:
		if w.Mask&ColorWellMask == 0 {
			return
		}
	case *Gradient:
		if w.Mask&GradientWellMask == 0 {
			return
		}
	case *Pattern:
		if w.Mask&PatternWellMask == 0 {
			return
		}
	default:
		return
	}
	if ink != w.ink {
		w.ink = ink
		w.MarkForRedraw()
		if w.InkChangedCallback != nil {
			w.InkChangedCallback()
		}
	}
}

// DefaultSizes provides the default sizing.
func (w *Well) DefaultSizes(hint Size) (min, pref, max Size) {
	pref.Width = 4 + w.ContentSize
	pref.Height = 4 + w.ContentSize
	if border := w.Border(); border != nil {
		pref.AddInsets(border.Insets())
	}
	pref.GrowToInteger()
	pref.ConstrainForHint(hint)
	return pref, pref, pref
}

// DefaultFocusGained provides the default focus gained handling.
func (w *Well) DefaultFocusGained() {
	w.ScrollIntoView()
	w.MarkForRedraw()
}

// DefaultDraw provides the default drawing.
func (w *Well) DefaultDraw(canvas *Canvas, dirty Rect) {
	r := w.ContentRect(false)
	var bg Ink
	switch {
	case w.Pressed:
		bg = w.SelectionInk
	default:
		bg = w.BackgroundInk
	}
	thickness := float32(1)
	wellInset := thickness + 2.5
	if w.Focused() {
		thickness++
	}
	DrawRoundedRectBase(canvas, r, w.CornerRadius, thickness, bg, w.EdgeInk)
	r.InsetUniform(wellInset)
	radius := w.CornerRadius - (wellInset - 2)
	if pattern, ok := w.ink.(*Pattern); ok {
		canvas.Save()
		path := NewPath()
		path.RoundedRect(r, radius, radius)
		canvas.ClipPath(path, IntersectClipOp, true)
		canvas.DrawImageInRect(pattern.Image, r, nil, nil)
		canvas.Restore()
	} else {
		canvas.DrawRoundedRect(r, radius, radius, w.ink.Paint(canvas, r, Fill))
	}
	if !w.Enabled() {
		p := Black.Paint(canvas, r, Stroke)
		p.SetBlendMode(XorBlendMode)
		canvas.DrawLine(r.X+1, r.Y+1, r.Right()-1, r.Bottom()-1, p)
		canvas.DrawLine(r.X+1, r.Bottom()-1, r.Right()-1, r.Y+1, p)
	}
	canvas.DrawRoundedRect(r, radius, radius, w.EdgeInk.Paint(canvas, r, Stroke))
}

// DefaultMouseDown provides the default mouse down handling.
func (w *Well) DefaultMouseDown(where Point, button, clickCount int, mod Modifiers) bool {
	w.Pressed = true
	w.MarkForRedraw()
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (w *Well) DefaultMouseDrag(where Point, button int, mod Modifiers) bool {
	rect := w.ContentRect(false)
	pressed := rect.ContainsPoint(where)
	if w.Pressed != pressed {
		w.Pressed = pressed
		w.MarkForRedraw()
	}
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (w *Well) DefaultMouseUp(where Point, button int, mod Modifiers) bool {
	w.Pressed = false
	w.MarkForRedraw()
	rect := w.ContentRect(false)
	if rect.ContainsPoint(where) {
		if w.ClickCallback != nil {
			w.ClickCallback()
		}
	}
	return true
}

// DefaultKeyDown provides the default key down handling.
func (w *Well) DefaultKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if IsControlAction(keyCode, mod) {
		w.Click()
		return true
	}
	return false
}

// DefaultClick provides the default click handling, which shows a dialog for selecting an ink.
func (w *Well) DefaultClick() {
	showWellDialog(w)
}

// Click makes the ink well behave as if a user clicked on it.
func (w *Well) Click() {
	pressed := w.Pressed
	w.Pressed = true
	w.MarkForRedraw()
	w.FlushDrawing()
	w.Pressed = pressed
	time.Sleep(w.ClickAnimationTime)
	w.MarkForRedraw()
	if w.ClickCallback != nil {
		w.ClickCallback()
	}
}

// DefaultFileDrop provides the default file drop behavior.
func (w *Well) DefaultFileDrop(files []string) {
	for _, one := range files {
		if imageSpec := DistillImageSpecFor(one); imageSpec != "" {
			img, err := w.loadImage(imageSpec)
			if err != nil {
				jot.Warn(err)
				continue
			}
			if w.ValidateImageCallback != nil {
				img = w.ValidateImageCallback(img)
			}
			if img != nil {
				w.SetInk(&Pattern{Image: img})
				return
			}
		}
	}
}

func (w *Well) loadImage(imageSpec string) (*Image, error) {
	ctx, cancel := context.WithTimeout(context.Background(), w.ImageLoadTimeout)
	defer cancel()
	return w.ImageFromSpecCallback(ctx, imageSpec, w.ImageScale)
}

// DefaultUpdateCursor provides the default cursor for wells.
func (w *Well) DefaultUpdateCursor(_ Point) *Cursor {
	return PointingCursor()
}
