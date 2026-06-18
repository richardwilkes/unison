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
	"context"
	"path/filepath"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/blendmode"
	"github.com/richardwilkes/unison/enums/imgfmt"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/pathop"
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
	BackgroundInk:      ThemeAboveSurface,
	EdgeInk:            ThemeSurfaceEdge,
	SelectionInk:       ThemeFocus,
	ImageScale:         geom.NewPoint(0.5, 0.5),
	ContentSize:        20,
	CornerRadius:       geom.NewUniformSize(4),
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
	ImageScale         geom.Point
	ContentSize        float32
	CornerRadius       geom.Size
	Mask               WellMask
}

// Well represents a control that holds and lets a user choose an ink.
type Well struct {
	ImageFromSpecCallback func(ctx context.Context, filePathOrURL string, scale geom.Point) (*Image, error)
	InkChangedCallback    func()
	ClickCallback         func()
	ValidateImageCallback func(*Image) *Image
	ink                   Ink
	WellTheme
	Panel
	Pressed       bool
	dropHighlight bool
}

// WellDragTypes returns the list of DataTypes that Wells will accept in drag and drop operations.
func WellDragTypes() []*uti.DataType {
	return append(imgfmt.AllReadableUTIs(), uti.FileURL)
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
	well.CanAcceptDropCallback = well.DefaultCanAcceptDrop
	well.DragEnteredCallback = well.DefaultDragEnter
	well.DragExitedCallback = well.DefaultDragExit
	well.DropCallback = well.DefaultDrop
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
			xos.SafeCall(w.InkChangedCallback, nil)
		}
	}
}

// DefaultSizes provides the default sizing.
func (w *Well) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	prefSize.Width = 4 + w.ContentSize
	prefSize.Height = 4 + w.ContentSize
	if border := w.Border(); border != nil {
		prefSize = prefSize.Add(border.Insets().Size())
	}
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, prefSize
}

// DefaultFocusGained provides the default focus gained handling.
func (w *Well) DefaultFocusGained() {
	w.ScrollIntoView()
	w.MarkForRedraw()
}

// DefaultDraw provides the default drawing.
func (w *Well) DefaultDraw(canvas *Canvas, _ geom.Rect) {
	r := w.ContentRect(false)
	var bg Ink
	switch {
	case w.Pressed:
		bg = w.SelectionInk
	default:
		bg = w.BackgroundInk
	}
	edge := w.EdgeInk
	thickness := float32(1)
	wellInset := thickness + 2.5
	if w.dropHighlight || w.Focused() {
		thickness++
		edge = w.SelectionInk
	}
	DrawRoundedRectBase(canvas, r, w.CornerRadius, thickness, bg, edge)
	r = r.Inset(geom.NewUniformInsets(wellInset))
	radius := w.CornerRadius.Sub(geom.NewUniformSize(wellInset - 2))
	if pattern, ok := w.ink.(*Pattern); ok {
		canvas.Save()
		path := NewPath()
		path.RoundedRect(r, radius)
		canvas.ClipPath(path, pathop.Intersect, true)
		canvas.DrawImageInRect(pattern.Image, r, nil, nil)
		canvas.Restore()
		path.Dispose()
	} else {
		fillPaint := w.ink.Paint(canvas, r, paintstyle.Fill)
		canvas.DrawRoundedRect(r, radius, fillPaint)
		fillPaint.Dispose()
	}
	if !w.Enabled() {
		p := Black.Paint(canvas, r, paintstyle.Stroke)
		p.SetBlendMode(blendmode.Xor)
		canvas.DrawLine(geom.NewPoint(r.X+1, r.Y+1), geom.NewPoint(r.Right()-1, r.Bottom()-1), p)
		canvas.DrawLine(geom.NewPoint(r.X+1, r.Bottom()-1), geom.NewPoint(r.Right()-1, r.Y+1), p)
		p.Dispose()
	}
	edgePaint := edge.Paint(canvas, r, paintstyle.Stroke)
	defer edgePaint.Dispose()
	canvas.DrawRoundedRect(r, radius, edgePaint)
}

// DefaultMouseDown provides the default mouse down handling.
func (w *Well) DefaultMouseDown(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
	w.Pressed = true
	w.MarkForRedraw()
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (w *Well) DefaultMouseDrag(where geom.Point, _ int, _ mod.Modifiers) bool {
	rect := w.ContentRect(false)
	if pressed := where.In(rect); pressed != w.Pressed {
		w.Pressed = pressed
		w.MarkForRedraw()
	}
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (w *Well) DefaultMouseUp(where geom.Point, _ int, _ mod.Modifiers) bool {
	w.Pressed = false
	w.MarkForRedraw()
	if where.In(w.ContentRect(false)) {
		if w.ClickCallback != nil {
			xos.SafeCall(w.ClickCallback, nil)
		}
	}
	return true
}

// DefaultKeyDown provides the default key down handling.
func (w *Well) DefaultKeyDown(keyCode KeyCode, mods mod.Modifiers, _repeat bool) bool {
	if IsControlAction(keyCode, mods) {
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
		xos.SafeCall(w.ClickCallback, nil)
	}
}

// DefaultCanAcceptDrop reports whether this well is a candidate for the given drag, independent of pointer position.
func (w *Well) DefaultCanAcceptDrop(di drag.Info) bool {
	if !w.Enabled() {
		return false
	}
	if di.HasFilePaths() {
		for _, f := range di.FilePaths() {
			if imgfmt.ForExtension(filepath.Ext(f)).CanRead() {
				return true
			}
		}
	}
	for _, dataType := range imgfmt.AllReadableUTIs() {
		if di.HasDataType(dataType.UTI) {
			return true
		}
	}
	return false
}

// DefaultDragEnter provides the default drag enter handling.
func (w *Well) DefaultDragEnter(di drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op {
	op := drag.None
	if w.DefaultCanAcceptDrop(di) {
		op = drag.Copy
	}
	if op != drag.None {
		if !w.dropHighlight {
			w.dropHighlight = true
			w.MarkForRedraw()
			w.FlushDrawing()
		}
	}
	return op
}

// DefaultDragExit provides the default drag exit handling.
func (w *Well) DefaultDragExit() {
	if w.dropHighlight {
		w.dropHighlight = false
		w.MarkForRedraw()
		w.FlushDrawing()
	}
}

// DefaultDrop provides the default drop handling. Handles image files dropped onto the well.
func (w *Well) DefaultDrop(di drag.Info, _ geom.Point, _ mod.Modifiers) bool {
	w.DefaultDragExit()
	if w.Enabled() {
		if di.HasFilePaths() {
			for _, f := range di.FilePaths() {
				if imgfmt.ForExtension(filepath.Ext(f)).CanRead() {
					img, err := w.loadImage(f)
					if err != nil {
						errs.Log(err, "spec", f)
						continue
					}
					if w.ValidateImageCallback != nil {
						xos.SafeCall(func() { img = w.ValidateImageCallback(img) }, nil)
					}
					if img != nil {
						w.SetInk(&Pattern{Image: img})
						return true
					}
				}
			}
		}
		for _, dataType := range imgfmt.AllReadableUTIs() {
			if di.HasDataType(dataType.UTI) {
				img, err := NewImageFromBytes(di.Data(dataType.UTI), w.ImageScale)
				if err != nil {
					errs.Log(err, "image data", dataType.UTI)
					continue
				}
				if w.ValidateImageCallback != nil {
					xos.SafeCall(func() { img = w.ValidateImageCallback(img) }, nil)
				}
				if img != nil {
					w.SetInk(&Pattern{Image: img})
					return true
				}
			}
		}
	}
	return false
}

func (w *Well) loadImage(imageSpec string) (img *Image, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), w.ImageLoadTimeout)
	defer cancel()
	xos.SafeCall(func() { img, err = w.ImageFromSpecCallback(ctx, imageSpec, w.ImageScale) }, nil)
	return img, err
}

// DefaultUpdateCursor provides the default cursor for wells.
func (w *Well) DefaultUpdateCursor(_ geom.Point) *Cursor {
	if !w.Enabled() {
		return ArrowCursor()
	}
	return PointingCursor()
}
