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
	"time"

	"github.com/richardwilkes/toolbox/xmath/geom32"
)

// DefaultProgressBarTheme holds the default ProgressBarTheme values for ProgressBars. Modifying this data will not
// alter existing ProgressBars, but will alter any ProgressBars created in the future.
var DefaultProgressBarTheme = ProgressBarTheme{
	BackgroundInk:      BackgroundColor,
	FillInk:            SelectionColor,
	EdgeInk:            ControlEdgeColor,
	TickSpeed:          time.Second / 30,
	FullTraversalSpeed: time.Second,
	PreferredBarHeight: 8,
	CornerRadius:       8,
	IndeterminateWidth: 15,
	EdgeThickness:      1,
}

// ProgressBarTheme holds theming data for a ProgressBar.
type ProgressBarTheme struct {
	BackgroundInk      Ink
	FillInk            Ink
	EdgeInk            Ink
	TickSpeed          time.Duration
	FullTraversalSpeed time.Duration
	PreferredBarHeight float32
	CornerRadius       float32
	IndeterminateWidth float32
	EdgeThickness      float32
}

// ProgressBar provides a meter showing progress.
type ProgressBar struct {
	Panel
	ProgressBarTheme
	current           float32
	max               float32
	lastAnimationTime time.Time
}

// NewProgressBar creates a new progress bar. A max of zero will create an indeterminate progress bar, i.e. one whose
// meter animates back and forth.
func NewProgressBar(max float32) *ProgressBar {
	p := &ProgressBar{
		ProgressBarTheme: DefaultProgressBarTheme,
		max:              max,
	}
	p.Self = p
	p.SetSizer(p.DefaultSizes)
	p.DrawCallback = p.DefaultDraw
	return p
}

// Current returns the current value of the progress bar towards its maximum.
func (p *ProgressBar) Current() float32 {
	return p.current
}

// SetCurrent sets the current value.
func (p *ProgressBar) SetCurrent(value float32) {
	if value < 0 {
		value = 0
	} else if value > p.max {
		value = p.max
	}
	if p.current != value {
		p.current = value
		p.MarkForRedraw()
	}
}

// Maximum returns the maximum value of the progress bar.
func (p *ProgressBar) Maximum() float32 {
	return p.max
}

// SetMaximum sets the maximum value.
func (p *ProgressBar) SetMaximum(value float32) {
	if value < 0 {
		value = 0
	}
	if p.max != value {
		p.max = value
		if p.max == 0 {
			p.lastAnimationTime = time.Time{}
		}
		if p.current > p.max {
			p.current = p.max
		}
		p.MarkForRedraw()
	}
}

// DefaultSizes provides the default sizing.
func (p *ProgressBar) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	min.Width = 80
	min.Height = p.PreferredBarHeight
	pref.Width = 100
	pref.Height = p.PreferredBarHeight
	max.Width = DefaultMaxSize
	max.Height = p.PreferredBarHeight
	if border := p.Border(); border != nil {
		insets := border.Insets()
		min.AddInsets(insets)
		pref.AddInsets(insets)
		max.AddInsets(insets)
	}
	min.GrowToInteger()
	pref.GrowToInteger()
	max.GrowToInteger()
	pref.ConstrainForHint(hint)
	return pref, pref, MaxSize(pref)
}

// DefaultDraw provides the default drawing.
func (p *ProgressBar) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	bounds := p.ContentRect(false)
	meter := bounds
	meter.Width = 0
	if p.max <= 0 {
		meter.Width = p.IndeterminateWidth
		if p.lastAnimationTime.IsZero() {
			p.lastAnimationTime = time.Now()
		} else {
			max := bounds.Width - p.IndeterminateWidth
			elapsed := time.Since(p.lastAnimationTime) % (2 * p.FullTraversalSpeed)
			if elapsed >= p.FullTraversalSpeed {
				elapsed = p.FullTraversalSpeed - (elapsed - p.FullTraversalSpeed)
			}
			meter.X = max * float32(elapsed) / float32(p.FullTraversalSpeed)
		}
	} else if p.current > 0 {
		meter.Width = bounds.Width * (p.current / p.max)
	}
	canvas.DrawRoundedRect(bounds, p.CornerRadius, p.BackgroundInk.Paint(canvas, bounds, Fill))
	if meter.Width > 0 {
		trimmedMeter := meter
		trimmedMeter.X += 0.5
		trimmedMeter.Width--
		canvas.DrawRoundedRect(trimmedMeter, p.CornerRadius, p.FillInk.Paint(canvas, trimmedMeter, Fill))
	}
	bounds.InsetUniform(p.EdgeThickness / 2)
	paint := p.EdgeInk.Paint(canvas, bounds, Stroke)
	paint.SetStrokeWidth(p.EdgeThickness)
	canvas.DrawRoundedRect(bounds, p.CornerRadius, paint)
	if meter.Width > 0 {
		meter.InsetUniform(p.EdgeThickness / 2)
		canvas.DrawRoundedRect(meter, p.CornerRadius, paint)
	}
	if p.max == 0 {
		InvokeTaskAfter(p.MarkForRedraw, p.TickSpeed)
	}
}
