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
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/gradienttype"
)

// ColorEditor provides a widget for editing a Color. It always operates on a copy of the Color it was given, so the
// original is left untouched; retrieve the edited result via Color().
type ColorEditor struct {
	ChangedCallback  func() // If set, is called whenever the color is modified.
	redSlider        *Slider
	redField         *Field
	greenSlider      *Slider
	greenField       *Field
	blueSlider       *Slider
	blueField        *Field
	alphaSlider      *Slider
	alphaField       *Field
	hueSlider        *Slider
	hueField         *Field
	saturationSlider *Slider
	saturationField  *Field
	brightnessSlider *Slider
	brightnessField  *Field
	cssField         *Field
	Panel
	color   Color
	syncing bool
}

// NewColorEditor creates a new ColorEditor.
func NewColorEditor(color Color) *ColorEditor {
	e := &ColorEditor{
		color: color,
	}
	e.Self = e
	e.SetLayout(&FlexLayout{
		Columns:  4,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	e.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	e.redSlider, e.redField = e.addChannelField(i18n.Text("Red"), color.Red(),
		func(value int) { e.color = e.color.SetRed(value) })
	e.greenSlider, e.greenField = e.addChannelField(i18n.Text("Green"), color.Green(),
		func(value int) { e.color = e.color.SetGreen(value) })
	e.blueSlider, e.blueField = e.addChannelField(i18n.Text("Blue"), color.Blue(),
		func(value int) { e.color = e.color.SetBlue(value) })
	e.alphaSlider, e.alphaField = e.addChannelField(i18n.Text("Alpha"), color.Alpha(),
		func(value int) { e.color = e.color.SetAlpha(value) })
	e.hueSlider, e.hueField = e.addHueField()
	e.saturationSlider, e.saturationField = e.addPercentageField(i18n.Text("Saturation"), color.Saturation(),
		func(value float32) { e.color = e.color.SetSaturation(value) })
	e.brightnessSlider, e.brightnessField = e.addPercentageField(i18n.Text("Brightness"), color.Brightness(),
		func(value float32) { e.color = e.color.SetBrightness(value) })
	e.cssField = e.addCSSField()
	e.sync()
	return e
}

// Color returns the currently selected color.
func (e *ColorEditor) Color() Color {
	return e.color
}

// SetColor sets the currently selected color.
func (e *ColorEditor) SetColor(color ColorProvider) {
	actual := color.GetColor()
	if e.color != actual {
		e.color = actual
		e.sync()
	}
}

func (e *ColorEditor) addChannelField(title string, value int, adjuster func(value int)) (*Slider, *Field) {
	l := NewLabel()
	l.SetTitle(title)
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	e.AddChild(l)

	slider := NewSlider(0, 255, float32(value))
	slider.ValueSnapCallback = func(v float32) float32 { return float32(int(v + 0.5)) }
	slider.ValueChangedCallback = func() {
		if !e.syncing {
			adjuster(int(slider.Value()))
			e.sync()
		}
	}
	slider.SetLayoutData(&FlexLayoutData{
		SizeHint: geom.NewSize(100, 0),
		HAlign:   align.Fill,
		VAlign:   align.Middle,
		HGrab:    true,
	})
	e.AddChild(slider)

	field := NewField()
	field.SetText(strconv.Itoa(value))
	field.Watermark = "0"
	field.SetMinimumTextWidthUsing("255", "100%")
	field.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
	})
	field.ValidateCallback = func() bool {
		text := strings.TrimSpace(field.Text())
		if text == "" {
			text = "0"
		}
		var v int
		if strings.HasSuffix(text, "%") {
			percentage, err := extractColorPercentage(text)
			if err != nil {
				return false
			}
			v = clamp0To1AndScale255(percentage)
		} else {
			var err error
			if v, err = strconv.Atoi(text); err != nil || v < 0 || v > 255 {
				return false
			}
		}
		if !e.syncing {
			adjuster(v)
			e.sync()
		}
		return true
	}
	e.AddChild(field)

	l = NewLabel()
	l.SetTitle(i18n.Text("0-255 or 0-100%"))
	l.SetEnabled(false)
	l.SetLayoutData(&FlexLayoutData{
		VAlign: align.Middle,
	})
	e.AddChild(l)
	return slider, field
}

func (e *ColorEditor) addHueField() (*Slider, *Field) {
	l := NewLabel()
	l.SetTitle(i18n.Text("Hue"))
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	e.AddChild(l)

	slider := NewSlider(0, 359, e.color.Hue())
	slider.ValueChangedCallback = func() {
		if !e.syncing {
			e.color = e.color.SetHue(slider.Value() / 360)
			e.sync()
		}
	}
	slider.SetLayoutData(&FlexLayoutData{
		SizeHint: geom.NewSize(100, 0),
		HAlign:   align.Fill,
		VAlign:   align.Middle,
		HGrab:    true,
	})
	e.AddChild(slider)

	field := NewField()
	field.SetText(strconv.Itoa(int(e.color.Hue()*360 + 0.5)))
	field.Watermark = "0"
	field.SetMinimumTextWidthUsing("359")
	field.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
	})
	field.ValidateCallback = func() bool {
		text := strings.TrimSpace(field.Text())
		if text == "" {
			text = "0"
		}
		v, err := strconv.Atoi(text)
		if err != nil || v < 0 || v > 360 {
			return false
		}
		if !e.syncing {
			e.color = e.color.SetHue(float32(v) / 360)
			e.sync()
		}
		return true
	}
	e.AddChild(field)

	l = NewLabel()
	l.SetTitle(i18n.Text("0-359"))
	l.SetEnabled(false)
	l.SetLayoutData(&FlexLayoutData{
		VAlign: align.Middle,
	})
	e.AddChild(l)
	return slider, field
}

func (e *ColorEditor) addPercentageField(title string, value float32, adjuster func(value float32)) (*Slider, *Field) {
	l := NewLabel()
	l.SetTitle(title)
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	e.AddChild(l)

	slider := NewSlider(0, 1, value)
	slider.ValueChangedCallback = func() {
		if !e.syncing {
			adjuster(slider.Value())
			e.sync()
		}
	}
	slider.SetLayoutData(&FlexLayoutData{
		SizeHint: geom.NewSize(100, 0),
		HAlign:   align.Fill,
		VAlign:   align.Middle,
		HGrab:    true,
	})
	e.AddChild(slider)

	field := NewField()
	field.SetText(strconv.Itoa(int(value*100+0.5)) + "%")
	field.Watermark = "0%"
	field.SetMinimumTextWidthUsing("100%")
	field.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
	})
	field.ValidateCallback = func() bool {
		text := strings.TrimSpace(field.Text())
		if text == "" {
			text = "0%"
		}
		if !strings.HasSuffix(text, "%") {
			text += "%"
		}
		percentage, err := extractColorPercentage(text)
		if err != nil {
			return false
		}
		if !e.syncing {
			adjuster(percentage)
			e.sync()
		}
		return true
	}
	e.AddChild(field)

	l = NewLabel()
	l.SetTitle(i18n.Text("0-100%"))
	l.SetEnabled(false)
	l.SetLayoutData(&FlexLayoutData{
		VAlign: align.Middle,
	})
	e.AddChild(l)
	return slider, field
}

func (e *ColorEditor) addCSSField() *Field {
	l := NewLabel()
	l.SetTitle(i18n.Text("CSS"))
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	e.AddChild(l)

	field := NewField()
	field.SetText(e.color.String())
	field.Watermark = "CSS"
	field.SetLayoutData(&FlexLayoutData{
		HSpan:  3,
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	field.ValidateCallback = func() bool {
		if !e.syncing {
			adjustedColor, err := ColorDecode(field.Text())
			if err != nil {
				return false
			}
			e.color = adjustedColor
			e.sync()
		}
		return true
	}
	e.AddChild(field)

	e.AddChild(NewPanel())
	wrapper := NewPanel()
	wrapper.SetLayout(&FlexLayout{Columns: 1})
	wrapper.SetLayoutData(&FlexLayoutData{
		HAlign: align.Start,
		VAlign: align.Start,
	})
	fd := DefaultLabelTheme.Font.Descriptor()
	fd.Size *= 0.8
	font := fd.Font()
	for _, line := range []string{
		i18n.Text("CSS can be one of:"),
		i18n.Text(`- predefined color name, e.g. "Yellow"`),
		i18n.Text(`- rgb(), e.g. "rgb(255, 255, 0)"`),
		i18n.Text(`- rgba(), e.g. "rgba(255, 255, 0, 0.3)"`),
		i18n.Text(`- short hexadecimal colors, e.g. "#FF0"`),
		i18n.Text(`- long hexadecimal colors, e.g. "#FFFF00"`),
		i18n.Text(`- hsl(), e.g. "hsl(120, 100%, 50%)"`),
		i18n.Text(`- hsla(), e.g. "hsla(120, 100%, 50%, 0.3)"`),
	} {
		l = NewLabel()
		l.Font = font
		l.SetTitle(line)
		wrapper.AddChild(l)
	}
	e.AddChild(wrapper)

	return field
}

func (e *ColorEditor) sync() {
	e.syncing = true
	red := e.color.Red()
	green := e.color.Green()
	blue := e.color.Blue()
	e.redSlider.SetValue(float32(red))
	e.redSlider.FillInk = e.newGradient(RGB(0, green, blue), RGB(255, green, blue))
	e.syncText(e.redField, strconv.Itoa(red))
	e.greenSlider.SetValue(float32(green))
	e.greenSlider.FillInk = e.newGradient(RGB(red, 0, blue), RGB(red, 255, blue))
	e.syncText(e.greenField, strconv.Itoa(green))
	e.blueSlider.SetValue(float32(blue))
	e.blueSlider.FillInk = e.newGradient(RGB(red, green, 0), RGB(red, green, 255))
	e.syncText(e.blueField, strconv.Itoa(blue))
	e.alphaSlider.SetValue(float32(e.color.Alpha()))
	e.alphaSlider.FillInk = e.newGradient(ARGB(0, red, green, blue), ARGB(1, red, green, blue))
	e.syncText(e.alphaField, strconv.Itoa(e.color.Alpha()))
	hue, saturation, brightness := e.color.HSB()
	e.hueSlider.SetValue(e.color.Hue() * 360)
	colors := make([]ColorProvider, 360)
	for i := range colors {
		colors[i] = HSB(float32(i)/float32(len(colors)-1), saturation, brightness)
	}
	e.hueSlider.FillInk = e.newGradient(colors...)
	e.syncText(e.hueField, strconv.Itoa(int(e.color.Hue()*360+0.5)))
	colors = make([]ColorProvider, 101)
	for i := range colors {
		colors[i] = HSB(hue, float32(i)/float32(len(colors)-1), brightness)
	}
	e.saturationSlider.FillInk = e.newGradient(colors...)
	e.saturationSlider.SetValue(e.color.Saturation())
	e.syncText(e.saturationField, strconv.Itoa(int(e.color.Saturation()*100+0.5))+"%")
	e.brightnessSlider.SetValue(e.color.Brightness())
	colors = make([]ColorProvider, 101)
	for i := range colors {
		colors[i] = HSB(hue, saturation, float32(i)/float32(len(colors)-1))
	}
	e.brightnessSlider.FillInk = e.newGradient(colors...)
	e.syncText(e.brightnessField, strconv.Itoa(int(e.color.Brightness()*100+0.5))+"%")
	e.syncText(e.cssField, e.color.String())
	e.syncing = false
	if e.ChangedCallback != nil {
		e.ChangedCallback()
	}
}

func (e *ColorEditor) newGradient(colors ...ColorProvider) *Gradient {
	return &Gradient{
		Stops:     NewEvenlySpacedGradientStopsForColors(colors...),
		EndPt:     geom.NewPoint(1, 0),
		Transform: geom.NewIdentityMatrix(),
		Kind:      gradienttype.Linear,
	}
}

func (e *ColorEditor) syncText(field *Field, text string) {
	if !field.Focused() {
		field.SetText(text)
	}
}
