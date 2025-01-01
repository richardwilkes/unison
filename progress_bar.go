// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/unison/enums/paintstyle"
)

// DefaultProgressBarTheme holds the default ProgressBarTheme values for ProgressBars. Modifying this data will not
// alter existing ProgressBars, but will alter any ProgressBars created in the future.
var DefaultProgressBarTheme = ProgressBarTheme{
	BackgroundInk:      ThemeSurface,
	FillInk:            ThemeFocus,
	EdgeInk:            ThemeSurfaceEdge,
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
	lastAnimationTime time.Time
	ProgressBarTheme
	Panel
	current float32
	maximum float32
}

// NewProgressBar creates a new progress bar. A max of zero will create an indeterminate progress bar, i.e. one whose
// meter animates back and forth.
func NewProgressBar(maximum float32) *ProgressBar {
	p := &ProgressBar{
		ProgressBarTheme: DefaultProgressBarTheme,
		maximum:          maximum,
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
	} else if value > p.maximum {
		value = p.maximum
	}
	if p.current != value {
		p.current = value
		p.MarkForRedraw()
	}
}

// Maximum returns the maximum value of the progress bar.
func (p *ProgressBar) Maximum() float32 {
	return p.maximum
}

// SetMaximum sets the maximum value.
func (p *ProgressBar) SetMaximum(value float32) {
	if value < 0 {
		value = 0
	}
	if p.maximum != value {
		p.maximum = value
		if p.maximum == 0 {
			p.lastAnimationTime = time.Time{}
		}
		if p.current > p.maximum {
			p.current = p.maximum
		}
		p.MarkForRedraw()
	}
}

// DefaultSizes provides the default sizing.
func (p *ProgressBar) DefaultSizes(hint Size) (minSize, prefSize, maxSize Size) {
	minSize.Width = 80
	minSize.Height = p.PreferredBarHeight
	prefSize.Width = 100
	prefSize.Height = p.PreferredBarHeight
	maxSize.Width = DefaultMaxSize
	maxSize.Height = p.PreferredBarHeight
	if border := p.Border(); border != nil {
		insets := border.Insets().Size()
		minSize = minSize.Add(insets)
		prefSize = prefSize.Add(insets)
		maxSize = maxSize.Add(insets)
	}
	return minSize.Ceil(), prefSize.Ceil().ConstrainForHint(hint), MaxSize(maxSize.Ceil())
}

// DefaultDraw provides the default drawing.
func (p *ProgressBar) DefaultDraw(canvas *Canvas, _ Rect) {
	bounds := p.ContentRect(false)
	meter := bounds
	meter.Width = 0
	if p.maximum <= 0 {
		meter.Width = p.IndeterminateWidth
		if p.lastAnimationTime.IsZero() {
			p.lastAnimationTime = time.Now()
		} else {
			maximum := bounds.Width - p.IndeterminateWidth
			elapsed := time.Since(p.lastAnimationTime) % (2 * p.FullTraversalSpeed)
			if elapsed >= p.FullTraversalSpeed {
				elapsed = p.FullTraversalSpeed - (elapsed - p.FullTraversalSpeed)
			}
			meter.X = maximum * float32(elapsed) / float32(p.FullTraversalSpeed)
		}
	} else if p.current > 0 {
		meter.Width = bounds.Width * (p.current / p.maximum)
	}
	canvas.DrawRoundedRect(bounds, p.CornerRadius, p.CornerRadius,
		p.BackgroundInk.Paint(canvas, bounds, paintstyle.Fill))
	if meter.Width > 0 {
		trimmedMeter := meter
		trimmedMeter.X += 0.5
		trimmedMeter.Width--
		canvas.DrawRoundedRect(trimmedMeter, p.CornerRadius, p.CornerRadius,
			p.FillInk.Paint(canvas, trimmedMeter, paintstyle.Fill))
	}
	bounds = bounds.Inset(NewUniformInsets(p.EdgeThickness / 2))
	paint := p.EdgeInk.Paint(canvas, bounds, paintstyle.Stroke)
	paint.SetStrokeWidth(p.EdgeThickness)
	canvas.DrawRoundedRect(bounds, p.CornerRadius, p.CornerRadius, paint)
	if meter.Width > 0 {
		meter = meter.Inset(NewUniformInsets(p.EdgeThickness / 2))
		canvas.DrawRoundedRect(meter, p.CornerRadius, p.CornerRadius, paint)
	}
	if p.maximum == 0 {
		InvokeTaskAfter(p.MarkForRedraw, p.TickSpeed)
	}
}
