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
	"errors"
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/gradienttype"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/tilemode"
)

const (
	gradientMinStops        = 2
	gradientBarHeight       = 18
	gradientHandleHalfWidth = 6
	gradientHandleHeight    = 10
)

// GradientEditor provides a widget for editing a Gradient. It allows the stops (both their position and color) to be
// added, removed, and edited, along with the other public fields of the Gradient. It always operates on a copy of the
// Gradient it was given, so the original is left untouched; retrieve the edited result via Gradient().
type GradientEditor struct {
	ChangedCallback  func() // If set, is called whenever the gradient is modified.
	gradient         *Gradient
	bar              *Panel
	posField         *Field
	colorWell        *Well
	removeButton     *Button
	typePopup        *PopupMenu[gradienttype.Enum]
	startLabel       *Label
	startXField      *Field
	startYField      *Field
	endLabel         *Label
	endXField        *Field
	endYField        *Field
	radiusLabel      *Label
	startRadiusField *Field
	endRadiusField   *Field
	angleLabel       *Label
	startAngleField  *Field
	endAngleField    *Field
	tileModePopup    *PopupMenu[tilemode.Enum]
	Panel
	selectedStop int
	syncing      bool
}

// NewGradientEditor creates a new GradientEditor. If gradient is nil, a default two-stop linear gradient from black to
// white is used. The editor works on a clone of the provided gradient. If the provided gradient has fewer than two
// stops, additional stops will be added to ensure there are at least two.
func NewGradientEditor(gradient *Gradient) *GradientEditor {
	e := &GradientEditor{}
	e.Self = e
	e.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})

	e.bar = NewPanel()
	e.bar.SetFocusable(true)
	e.bar.SetLayoutData(&FlexLayoutData{
		SizeHint: geom.NewSize(0, gradientBarHeight+gradientHandleHeight/2),
		HAlign:   align.Fill,
		VAlign:   align.Middle,
		HGrab:    true,
	})
	e.bar.DrawCallback = e.drawBar
	e.bar.MouseDownCallback = e.barMouseDown
	e.bar.MouseDragCallback = e.barMouseDrag
	e.bar.KeyDownCallback = e.barKeyDown
	e.AddChild(e.bar)

	e.addStopEditor()
	e.addGeometryEditor()

	e.SetGradient(gradient)
	return e
}

// Gradient returns a copy of the gradient being edited.
func (e *GradientEditor) Gradient() *Gradient {
	return e.gradient.Clone()
}

// SetGradient replaces the gradient being edited with a clone of the one passed in. You may pass in nil to create a
// two-stop linear gradient from black to white. If the provided gradient has fewer than two stops, additional stops
// will be added to ensure there are at least two.
func (e *GradientEditor) SetGradient(gradient *Gradient) {
	if gradient == nil {
		e.gradient = &Gradient{
			Stops:     NewEvenlySpacedGradientStopsForColors(Black, White),
			EndPt:     geom.NewPoint(1, 0),
			Transform: geom.NewIdentityMatrix(),
		}
	} else {
		e.gradient = gradient.Clone()
		if len(e.gradient.Stops) < gradientMinStops {
			for len(e.gradient.Stops) < gradientMinStops {
				stop := Stop{Color: Black}
				if len(e.gradient.Stops) > 0 {
					stop.Color = White
					stop.Location = 1
				}
				e.gradient.Stops = append(e.gradient.Stops, stop)
			}
		}
		e.gradient.Stops.Sort()
	}
	if e.selectedStop >= len(e.gradient.Stops) {
		e.selectedStop = len(e.gradient.Stops) - 1
	}
	e.sync()
}

func (e *GradientEditor) addStopEditor() {
	panel := NewPanel()
	panel.SetLayout(&FlexLayout{
		Columns:  6,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	panel.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})

	e.removeButton = NewSVGButton(TrashSVG)
	e.removeButton.Tooltip = NewTooltipWithText(i18n.Text("Remove Stop"))
	e.removeButton.ClickCallback = func() { e.removeStop(e.selectedStop) }
	panel.AddChild(e.removeButton)

	e.addLabel(panel, i18n.Text("Position"))
	e.posField = e.newPercentField(panel, func(v float32) {
		sel := e.gradient.Stops[e.selectedStop]
		sel.Location = v
		e.gradient.Stops[e.selectedStop] = sel
		e.gradient.Stops.Sort()
		e.reselect(sel)
		e.changed()
	})

	e.addLabel(panel, i18n.Text("Color"))
	e.colorWell = NewWell()
	e.colorWell.Mask = ColorWellMask
	e.colorWell.SetLayoutData(&FlexLayoutData{VAlign: align.Middle})
	e.colorWell.InkChangedCallback = func() {
		if e.syncing {
			return
		}
		if color, ok := e.colorWell.Ink().(Color); ok {
			sel := e.gradient.Stops[e.selectedStop]
			sel.Color = color
			e.gradient.Stops[e.selectedStop] = sel
			e.changed()
		}
	}
	panel.AddChild(e.colorWell)

	addButton := NewSVGButton(CircledAddSVG)
	addButton.Tooltip = NewTooltipWithText(i18n.Text("Add Stop"))
	addButton.ClickCallback = e.addStopInLargestGap
	panel.AddChild(addButton)

	e.AddChild(panel)
}

func (e *GradientEditor) addStopInLargestGap() {
	stops := e.gradient.Stops
	location := float32(0.5)
	if len(stops) >= 2 {
		var bestGap float32 = -1
		for i := range len(stops) - 1 {
			if gap := stops[i+1].Location - stops[i].Location; gap > bestGap {
				bestGap = gap
				location = stops[i].Location + gap/2
			}
		}
	}
	e.insertStop(location)
}

func (e *GradientEditor) insertStop(location float32) {
	location = clamp0To1(location)
	stop := Stop{Color: e.colorAt(location), Location: location}
	e.gradient.Stops = append(e.gradient.Stops, stop)
	e.gradient.Stops.Sort()
	e.reselect(stop)
	e.changed()
}

func (e *GradientEditor) removeStop(index int) {
	if len(e.gradient.Stops) <= gradientMinStops || index < 0 || index >= len(e.gradient.Stops) {
		return
	}
	e.gradient.Stops = append(e.gradient.Stops[:index:index], e.gradient.Stops[index+1:]...)
	if e.selectedStop >= len(e.gradient.Stops) {
		e.selectedStop = len(e.gradient.Stops) - 1
	}
	e.changed()
}

func (e *GradientEditor) colorAt(location float32) Color {
	stops := e.gradient.Stops
	if location <= stops[0].Location {
		return stops[0].Color.GetColor()
	}
	last := len(stops) - 1
	if location >= stops[last].Location {
		return stops[last].Color.GetColor()
	}
	for i := range last {
		left := stops[i]
		right := stops[i+1]
		if location >= left.Location && location <= right.Location {
			span := right.Location - left.Location
			if span <= 0 {
				return left.Color.GetColor()
			}
			return left.Color.GetColor().Blend(right.Color.GetColor(), (location-left.Location)/span)
		}
	}
	return stops[last].Color.GetColor()
}

func (e *GradientEditor) addGeometryEditor() {
	panel := NewPanel()
	panel.SetBorder(NewEmptyBorder(geom.Insets{Top: StdVSpacing}))
	panel.SetLayout(&FlexLayout{
		Columns:  6,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	panel.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})

	popupPanel := NewPanel()
	popupPanel.SetLayout(&FlexLayout{
		Columns:  2,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	popupPanel.SetLayoutData(&FlexLayoutData{
		HAlign: align.Middle,
		HSpan:  6,
	})
	e.AddChild(popupPanel)

	e.typePopup = NewPopupMenu[gradienttype.Enum]()
	e.typePopup.AddItem(gradienttype.All...)
	e.typePopup.SelectionChangedCallback = func(popup *PopupMenu[gradienttype.Enum]) {
		if e.syncing {
			return
		}
		sel, ok := popup.Selected()
		if !ok {
			sel = gradienttype.Linear
		}
		e.gradient.Kind = sel
		switch sel {
		case gradienttype.Linear:
			e.gradient.Radius = StartEnd{}
			e.gradient.Angle = StartEnd{}
		case gradienttype.Radial:
			e.gradient.EndPt = geom.Point{}
			if e.gradient.Radius.Start <= 0 {
				e.gradient.Radius.Start = 32
			}
			e.gradient.Radius.End = 0
			e.gradient.Angle = StartEnd{}
		case gradienttype.Sweep:
			e.gradient.EndPt = geom.Point{}
			e.gradient.Radius = StartEnd{}
			e.gradient.Angle.Start = max(min(e.gradient.Angle.Start, 359), 0)
			e.gradient.Angle.End = max(min(e.gradient.Angle.End, 359), 0)
		case gradienttype.Conical:
			e.gradient.Angle = StartEnd{}
			if e.gradient.Radius.Start <= 0 {
				e.gradient.Radius.Start = 32
			}
			if e.gradient.Radius.End <= 0 {
				e.gradient.Radius.End = 32
			}
		}
		e.changed()
	}
	e.typePopup.SetLayoutData(&FlexLayoutData{
		HAlign: align.Start,
		VAlign: align.Middle,
	})
	popupPanel.AddChild(e.typePopup)

	e.tileModePopup = NewPopupMenu[tilemode.Enum]()
	e.tileModePopup.AddItem(tilemode.All...)
	e.tileModePopup.SelectionChangedCallback = func(popup *PopupMenu[tilemode.Enum]) {
		if e.syncing {
			return
		}
		if value, ok := popup.Selected(); ok {
			e.gradient.TileMode = value
			e.changed()
		}
	}
	e.tileModePopup.SetLayoutData(&FlexLayoutData{
		HAlign: align.Start,
		VAlign: align.Middle,
	})
	popupPanel.AddChild(e.tileModePopup)

	e.startLabel = e.addLabel(panel, i18n.Text("Start"))
	e.startXField, e.startYField = e.addPointRow(panel, func() *geom.Point { return &e.gradient.StartPt })
	e.endLabel = e.addLabel(panel, i18n.Text("End"))
	e.endXField, e.endYField = e.addPointRow(panel, func() *geom.Point { return &e.gradient.EndPt })

	e.radiusLabel = e.addLabel(panel, i18n.Text("Radius"))
	e.startRadiusField, e.endRadiusField = e.addRadiusRow(panel)

	e.angleLabel = e.addLabel(panel, i18n.Text("Angle"))
	e.startAngleField, e.endAngleField = e.addAngleRow(panel)

	e.AddChild(panel)
}

func (e *GradientEditor) addPointRow(parent *Panel, accessor func() *geom.Point) (xField, yField *Field) {
	e.addTrailingLabel(parent, i18n.Text("X"))
	xField = e.newPercentField(parent, func(v float32) { accessor().X = v })
	e.addTrailingLabel(parent, i18n.Text("Y"))
	yField = e.newPercentField(parent, func(v float32) { accessor().Y = v })
	parent.AddChild(NewLabel())
	return xField, yField
}

func (e *GradientEditor) addRadiusRow(parent *Panel) (startField, endField *Field) {
	e.addTrailingLabel(parent, i18n.Text("Start"))
	startField = e.newPixelsField(parent, func(v float32) { e.gradient.Radius.Start = v })
	e.addTrailingLabel(parent, i18n.Text("End"))
	endField = e.newPixelsField(parent, func(v float32) { e.gradient.Radius.End = v })
	e.addTrailingLabel(parent, i18n.Text("px"))
	return startField, endField
}

func (e *GradientEditor) addAngleRow(parent *Panel) (startField, endField *Field) {
	e.addTrailingLabel(parent, i18n.Text("Start"))
	startField = e.newDegreesField(parent, func(v float32) { e.gradient.Angle.Start = v })
	e.addTrailingLabel(parent, i18n.Text("End"))
	endField = e.newDegreesField(parent, func(v float32) { e.gradient.Angle.End = v })
	parent.AddChild(NewLabel())
	return startField, endField
}

func (e *GradientEditor) addLabel(parent *Panel, title string) *Label {
	l := NewLabel()
	l.SetTitle(title)
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	return l
}

func (e *GradientEditor) addTrailingLabel(parent *Panel, title string) {
	l := NewLabel()
	l.SetTitle(title)
	l.SetEnabled(false)
	l.SetLayoutData(&FlexLayoutData{VAlign: align.Middle})
	parent.AddChild(l)
}

func (e *GradientEditor) newPercentField(parent *Panel, apply func(v float32)) *Field {
	field := NewField()
	field.Watermark = "0%"
	field.SetMinimumTextWidthUsing("9999")
	field.SetLayoutData(&FlexLayoutData{VAlign: align.Middle})
	field.ValidateCallback = func() bool {
		text := field.Text()
		if text != "" && text[len(text)-1] != '%' {
			text += "%"
		}
		percentage, err := extractColorPercentage(text)
		if err != nil {
			return false
		}
		if !e.syncing {
			apply(percentage)
			e.changed()
		}
		return true
	}
	parent.AddChild(field)
	return field
}

func (e *GradientEditor) newPixelsField(parent *Panel, apply func(v float32)) *Field {
	field := NewField()
	field.SetMinimumTextWidthUsing("9999")
	field.SetLayoutData(&FlexLayoutData{VAlign: align.Middle})
	field.ValidateCallback = func() bool {
		v, err := strconv.ParseFloat(field.Text(), 32)
		if err != nil || v < 0 {
			return false
		}
		if !e.syncing {
			apply(float32(v))
			e.changed()
		}
		return true
	}
	parent.AddChild(field)
	return field
}

func (e *GradientEditor) newDegreesField(parent *Panel, apply func(v float32)) *Field {
	field := NewField()
	field.Watermark = "0°"
	field.SetMinimumTextWidthUsing("9999")
	field.SetLayoutData(&FlexLayoutData{VAlign: align.Middle})
	field.ValidateCallback = func() bool {
		text := field.Text()
		if text != "" && text[len(text)-1] != '°' {
			text += "°"
		}
		degrees, err := extractDegrees(text)
		if err != nil {
			return false
		}
		if !e.syncing {
			apply(degrees)
			e.changed()
		}
		return true
	}
	parent.AddChild(field)
	return field
}

func extractDegrees(s string) (float32, error) {
	var isDegrees bool
	s, isDegrees = strings.CutSuffix(strings.TrimSpace(s), "°")
	if !isDegrees {
		return 0, errors.New("expected degrees value")
	}
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, err
	}
	if v < 0 {
		v = 0
	} else if v > 359 {
		v = 359
	}
	return float32(v), nil
}

func (e *GradientEditor) barContentRect() geom.Rect {
	r := e.bar.ContentRect(false)
	r.Height = gradientBarHeight
	return r
}

func (e *GradientEditor) drawBar(canvas *Canvas, _ geom.Rect) {
	r := e.barContentRect()
	preview := &Gradient{
		Stops:     e.gradient.Stops,
		StartPt:   geom.Point{},
		EndPt:     geom.NewPoint(1, 0),
		Transform: geom.NewIdentityMatrix(),
	}
	paint := preview.Paint(canvas, r, paintstyle.Fill)
	canvas.DrawRect(r, paint)
	paint.Dispose()

	edge := ThemeSurfaceEdge.Paint(canvas, r, paintstyle.Stroke)
	edge.SetStrokeWidth(1)
	canvas.DrawRect(r, edge)
	edge.Dispose()

	for i, stop := range e.gradient.Stops {
		x := r.X + stop.Location*r.Width
		top := r.Y + r.Height - gradientHandleHeight/2
		path := NewPath()
		path.MoveTo(geom.NewPoint(x, top))
		path.LineTo(geom.NewPoint(x-gradientHandleHalfWidth, top+gradientHandleHeight))
		path.LineTo(geom.NewPoint(x+gradientHandleHalfWidth, top+gradientHandleHeight))
		path.Close()
		ink := Ink(ThemeOnSurface)
		if i == e.selectedStop {
			ink = ThemeFocus
		}
		fill := ink.Paint(canvas, r, paintstyle.Fill)
		stroke := ThemeSurfaceEdge.Paint(canvas, r, paintstyle.Stroke)
		canvas.DrawPath(path, fill)
		canvas.DrawPath(path, stroke)
		fill.Dispose()
		path.Dispose()
	}
}

func (e *GradientEditor) barMouseDown(where geom.Point, _, _ int, _ mod.Modifiers) bool {
	e.bar.RequestFocus()
	r := e.barContentRect()
	if r.Width <= 0 {
		return true
	}
	if index := e.handleAt(where, r); index >= 0 {
		e.selectStop(index)
		return true
	}
	e.insertStop((where.X - r.X) / r.Width)
	return true
}

func (e *GradientEditor) barMouseDrag(where geom.Point, _ int, _ mod.Modifiers) bool {
	r := e.barContentRect()
	if r.Width <= 0 || e.selectedStop < 0 || e.selectedStop >= len(e.gradient.Stops) {
		return true
	}
	sel := e.gradient.Stops[e.selectedStop]
	sel.Location = clamp0To1((where.X - r.X) / r.Width)
	e.gradient.Stops[e.selectedStop] = sel
	e.gradient.Stops.Sort()
	e.reselect(sel)
	e.changed()
	return true
}

func (e *GradientEditor) barKeyDown(keyCode KeyCode, _ mod.Modifiers, _ bool) bool {
	switch keyCode {
	case KeyDelete, KeyBackspace:
		e.removeStop(e.selectedStop)
		return true
	default:
		return false
	}
}

func (e *GradientEditor) handleAt(where geom.Point, r geom.Rect) int {
	best := -1
	var bestDist float32 = gradientHandleHalfWidth + 2
	for i, stop := range e.gradient.Stops {
		x := r.X + stop.Location*r.Width
		dist := where.X - x
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			best = i
		}
	}
	return best
}

func (e *GradientEditor) selectStop(index int) {
	e.selectedStop = index
	e.sync()
}

func (e *GradientEditor) reselect(stop Stop) {
	for i, s := range e.gradient.Stops {
		if s == stop {
			e.selectedStop = i
			return
		}
	}
}

func (e *GradientEditor) changed() {
	e.sync()
	SafeCall(e.ChangedCallback)
}

func (e *GradientEditor) sync() {
	e.syncing = true

	stop := e.gradient.Stops[e.selectedStop]
	e.syncFieldText(e.posField, e.percentString(stop.Location), true)
	e.colorWell.SetInk(stop.Color.GetColor())
	e.removeButton.SetEnabled(len(e.gradient.Stops) > gradientMinStops)

	e.typePopup.Select(e.gradient.Kind)
	e.tileModePopup.Select(e.gradient.TileMode)

	var startTitle string
	if e.gradient.Kind == gradienttype.Radial || e.gradient.Kind == gradienttype.Sweep {
		startTitle = i18n.Text("Center")
	} else {
		startTitle = i18n.Text("Start")
	}
	if startTitle != e.startLabel.Text.String() {
		e.startLabel.SetTitle(startTitle)
		e.startLabel.MarkForLayoutRecursivelyUpward()
	}
	e.syncFieldText(e.startXField, e.percentString(e.gradient.StartPt.X), true)
	e.syncFieldText(e.startYField, e.percentString(e.gradient.StartPt.Y), true)

	enabled := e.gradient.Kind == gradienttype.Linear || e.gradient.Kind == gradienttype.Conical
	e.endLabel.SetEnabled(enabled)
	e.syncFieldText(e.endXField, e.percentString(e.gradient.EndPt.X), enabled)
	e.syncFieldText(e.endYField, e.percentString(e.gradient.EndPt.Y), enabled)

	enabled = e.gradient.Kind == gradienttype.Radial || e.gradient.Kind == gradienttype.Conical
	e.radiusLabel.SetEnabled(enabled)
	e.syncFieldText(e.startRadiusField, e.floatString(e.gradient.Radius.Start), enabled)
	e.syncFieldText(e.endRadiusField, e.floatString(e.gradient.Radius.End), e.gradient.Kind == gradienttype.Conical)

	enabled = e.gradient.Kind == gradienttype.Sweep
	e.angleLabel.SetEnabled(enabled)
	e.syncFieldText(e.startAngleField, e.degreeString(e.gradient.Angle.Start), enabled)
	e.syncFieldText(e.endAngleField, e.degreeString(e.gradient.Angle.End), enabled)

	e.syncing = false
	e.MarkForRedraw()
}

func (e *GradientEditor) syncFieldText(field *Field, text string, enable bool) {
	if !field.Focused() {
		field.SetText(text)
	}
	field.SetEnabled(enable)
}

func (e *GradientEditor) percentString(value float32) string {
	return strconv.Itoa(int(value*100+0.5)) + "%"
}

func (e *GradientEditor) floatString(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', -1, 32)
}

func (e *GradientEditor) degreeString(value float32) string {
	return strconv.Itoa(int(value)) + "°"
}
