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
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/imgfmt"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

type wellDialog struct {
	well             *Well
	originalInk      Ink
	ink              Ink
	dialog           *Dialog
	panel            *Panel
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
	syncing          bool
}

// TODO: Implement gradient selection

func showWellDialog(w *Well) {
	d := &wellDialog{
		well:        w,
		originalInk: w.Ink(),
		ink:         w.Ink(),
		panel:       NewPanel(),
	}
	d.panel.SetLayout(&FlexLayout{
		Columns:  2,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})

	left := NewPanel()
	left.SetBorder(NewEmptyBorder(geom.Insets{Right: 2 * StdHSpacing}))
	left.SetLayoutData(&FlexLayoutData{})
	left.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	d.panel.AddChild(left)
	d.addPreviewBlock(left, i18n.Text("Preview"), 0, func() Ink { return d.ink })
	d.addPreviewBlock(left, i18n.Text("Original"), 16, func() Ink { return d.originalInk })

	right := NewPanel()
	right.SetLayout(&FlexLayout{
		Columns:  2,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	right.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	d.panel.AddChild(right)

	if w.Mask&PatternWellMask != 0 {
		d.addPatternSelector(right)
	}
	if w.Mask&ColorWellMask != 0 {
		d.addColorSelector(right)
	}
	d.sync()

	var err error
	d.dialog, err = NewDialog(nil, nil, d.panel, []*DialogButtonInfo{NewCancelButtonInfo(), NewOKButtonInfo()})
	if err != nil {
		errs.Log(err)
		return
	}
	d.dialog.Window().SetTitle(i18n.Text("Choose an ink"))
	if d.dialog.RunModal() == ModalResponseOK {
		w.SetInk(d.ink)
	}
}

func (d *wellDialog) addPatternSelector(parent *Panel) {
	b := NewButton()
	b.SetTitle(i18n.Text("Select Image…"))
	b.SetLayoutData(&FlexLayoutData{
		HSpan:  2,
		HAlign: align.Middle,
		VAlign: align.Middle,
	})
	b.ClickCallback = func() {
		openDialog := NewOpenDialog()
		openDialog.SetAllowedExtensions(imgfmt.AllReadableExtensions()...)
		if openDialog.RunModal() {
			unable := i18n.Text("Unable to load image")
			paths := openDialog.Paths()
			if len(paths) == 0 {
				ErrorDialogWithMessage(unable, "Invalid path")
				return
			}
			imageSpec := imgfmt.Distill(paths[0])
			if imageSpec == "" {
				ErrorDialogWithMessage(unable, "Invalid image file")
				return
			}
			img, err := d.well.loadImage(imageSpec)
			if err != nil {
				ErrorDialogWithError(unable, err)
				return
			}
			if d.well.ValidateImageCallback != nil {
				img = d.well.ValidateImageCallback(img)
			}
			if img == nil {
				ErrorDialogWithMessage(unable, "")
				return
			}
			d.ink = &Pattern{Image: img}
			d.dialog.Window().MarkForRedraw()
		}
	}
	if d.well.Mask&^PatternWellMask != 0 {
		b.SetBorder(NewEmptyBorder(geom.Insets{Bottom: 2 * StdHSpacing}))
	}
	parent.AddChild(b)
}

func (d *wellDialog) addColorSelector(parent *Panel) {
	color := Black
	switch inkColor := d.ink.(type) {
	case Color:
		color = inkColor
	case *Color:
		color = *inkColor
	default:
	}

	panel := NewPanel()
	panel.SetLayout(&FlexLayout{
		Columns:  4,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	panel.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		HGrab:  true,
	})
	d.redSlider, d.redField = d.addChannelField(panel, i18n.Text("Red"), color.Red(),
		func(value int, color Color) Color { return color.SetRed(value) })
	d.greenSlider, d.greenField = d.addChannelField(panel, i18n.Text("Green"), color.Green(),
		func(value int, color Color) Color { return color.SetGreen(value) })
	d.blueSlider, d.blueField = d.addChannelField(panel, i18n.Text("Blue"), color.Blue(),
		func(value int, color Color) Color { return color.SetBlue(value) })
	d.alphaSlider, d.alphaField = d.addChannelField(panel, i18n.Text("Alpha"), color.Alpha(),
		func(value int, color Color) Color { return color.SetAlpha(value) })
	d.hueSlider, d.hueField = d.addHueField(panel, color)
	d.saturationSlider, d.saturationField = d.addPercentageField(panel, i18n.Text("Saturation"), color.Saturation(),
		func(value float32, color Color) Color { return color.SetSaturation(value) })
	d.brightnessSlider, d.brightnessField = d.addPercentageField(panel, i18n.Text("Brightness"), color.Brightness(),
		func(value float32, color Color) Color { return color.SetBrightness(value) })
	d.cssField = d.addCSSField(panel, color)
	parent.AddChild(panel)
}

func (d *wellDialog) addChannelField(parent *Panel, title string, value int, adjuster func(value int, color Color) Color) (*Slider, *Field) {
	l := NewLabel()
	l.SetTitle(title)
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	parent.AddChild(l)

	slider := NewSlider(0, 255, float32(value))
	slider.ValueSnapCallback = func(v float32) float32 { return float32(int(v + 0.5)) }
	slider.ValueChangedCallback = func() {
		if !d.syncing {
			color, ok := d.ink.(Color)
			if !ok {
				color = Black
			}
			d.ink = adjuster(int(slider.Value()), color)
			d.dialog.Button(ModalResponseCancel).RequestFocus()
			d.sync()
		}
	}
	slider.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	parent.AddChild(slider)

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
		if !d.syncing {
			color, ok := d.ink.(Color)
			if !ok {
				color = Black
			}
			d.ink = adjuster(v, color)
			d.sync()
		}
		return true
	}
	parent.AddChild(field)

	l = NewLabel()
	l.SetTitle(i18n.Text("0-255 or 0-100%"))
	l.SetEnabled(false)
	l.SetLayoutData(&FlexLayoutData{
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	return slider, field
}

func (d *wellDialog) addHueField(parent *Panel, color Color) (*Slider, *Field) {
	l := NewLabel()
	l.SetTitle(i18n.Text("Hue"))
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	parent.AddChild(l)

	slider := NewSlider(0, 359, color.Hue())
	slider.ValueChangedCallback = func() {
		if !d.syncing {
			c, ok := d.ink.(Color)
			if !ok {
				c = Black
			}
			d.ink = c.SetHue(slider.Value() / 360)
			d.dialog.Button(ModalResponseCancel).RequestFocus()
			d.sync()
		}
	}
	slider.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	parent.AddChild(slider)

	field := NewField()
	field.SetText(strconv.Itoa(int(color.Hue()*360 + 0.5)))
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
		if !d.syncing {
			c, ok := d.ink.(Color)
			if !ok {
				c = Black
			}
			d.ink = c.SetHue(float32(v) / 360)
			d.sync()
		}
		return true
	}
	parent.AddChild(field)

	l = NewLabel()
	l.SetTitle(i18n.Text("0-359"))
	l.SetEnabled(false)
	l.SetLayoutData(&FlexLayoutData{
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	return slider, field
}

func (d *wellDialog) addPercentageField(parent *Panel, title string, value float32, adjuster func(value float32, color Color) Color) (*Slider, *Field) {
	l := NewLabel()
	l.SetTitle(title)
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	parent.AddChild(l)

	slider := NewSlider(0, 1, value)
	slider.ValueChangedCallback = func() {
		if !d.syncing {
			color, ok := d.ink.(Color)
			if !ok {
				color = Black
			}
			d.ink = adjuster(slider.Value(), color)
			d.dialog.Button(ModalResponseCancel).RequestFocus()
			d.sync()
		}
	}
	slider.SetLayoutData(&FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	parent.AddChild(slider)

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
		if !d.syncing {
			color, ok := d.ink.(Color)
			if !ok {
				color = Black
			}
			d.ink = adjuster(percentage, color)
			d.sync()
		}
		return true
	}
	parent.AddChild(field)

	l = NewLabel()
	l.SetTitle(i18n.Text("0-100%"))
	l.SetEnabled(false)
	l.SetLayoutData(&FlexLayoutData{
		VAlign: align.Middle,
	})
	parent.AddChild(l)
	return slider, field
}

func (d *wellDialog) addCSSField(parent *Panel, color Color) *Field {
	l := NewLabel()
	l.SetTitle(i18n.Text("CSS"))
	l.HAlign = align.End
	l.SetLayoutData(&FlexLayoutData{
		HAlign: align.End,
		VAlign: align.Middle,
	})
	parent.AddChild(l)

	field := NewField()
	field.SetText(color.String())
	field.Watermark = "CSS"
	field.SetLayoutData(&FlexLayoutData{
		HSpan:  3,
		HAlign: align.Fill,
		VAlign: align.Middle,
		HGrab:  true,
	})
	field.ValidateCallback = func() bool {
		if !d.syncing {
			adjustedColor, err := ColorDecode(field.Text())
			if err != nil {
				return false
			}
			d.ink = adjustedColor
			d.sync()
		}
		return true
	}
	field.Tooltip = NewTooltipWithText(`One of:
- predefined color name, e.g. "Yellow"
- rgb(), e.g. "rgb(255, 255, 0)"
- rgba(), e.g. "rgba(255, 255, 0, 0.3)"
- short hexadecimal colors, e.g. "#FF0"
- long hexadecimal colors, e.g. "#FFFF00"
- hsl(), e.g. "hsl(120, 100%, 50%)"
- hsla(), e.g. "hsla(120, 100%, 50%, 0.3)"`)
	field.TooltipImmediate = true
	parent.AddChild(field)
	return field
}

func (d *wellDialog) sync() {
	d.syncing = true
	switch t := d.ink.(type) {
	case Color:
		red := t.Red()
		green := t.Green()
		blue := t.Blue()
		d.redSlider.SetValue(float32(red))
		d.redSlider.FillInk = NewEvenlySpacedGradient(geom.Point{}, geom.Point{X: 1}, 0, 0, RGB(0, green, blue),
			RGB(255, green, blue))
		d.syncText(d.redField, strconv.Itoa(red))
		d.greenSlider.SetValue(float32(green))
		d.greenSlider.FillInk = NewEvenlySpacedGradient(geom.Point{}, geom.Point{X: 1}, 0, 0, RGB(red, 0, blue),
			RGB(red, 255, blue))
		d.syncText(d.greenField, strconv.Itoa(green))
		d.blueSlider.SetValue(float32(blue))
		d.blueSlider.FillInk = NewEvenlySpacedGradient(geom.Point{}, geom.Point{X: 1}, 0, 0, RGB(red, green, 0),
			RGB(red, green, 255))
		d.syncText(d.blueField, strconv.Itoa(blue))
		d.alphaSlider.SetValue(float32(t.Alpha()))
		d.alphaSlider.FillInk = NewEvenlySpacedGradient(geom.Point{}, geom.Point{X: 1}, 0, 0, ARGB(0, red, green, blue),
			ARGB(1, red, green, blue))
		d.syncText(d.alphaField, strconv.Itoa(t.Alpha()))
		hue, saturation, brightness := t.HSB()
		d.hueSlider.SetValue(t.Hue() * 360)
		colors := make([]ColorProvider, 360)
		for i := range colors {
			colors[i] = HSB(float32(i)/float32(len(colors)-1), saturation, brightness)
		}
		d.hueSlider.FillInk = NewEvenlySpacedGradient(geom.Point{}, geom.Point{X: 1}, 0, 0, colors...)
		d.syncText(d.hueField, strconv.Itoa(int(t.Hue()*360+0.5)))
		colors = make([]ColorProvider, 101)
		for i := range colors {
			colors[i] = HSB(hue, float32(i)/float32(len(colors)-1), brightness)
		}
		d.saturationSlider.FillInk = NewEvenlySpacedGradient(geom.Point{}, geom.Point{X: 1}, 0, 0, colors...)
		d.saturationSlider.SetValue(t.Saturation())
		d.syncText(d.saturationField, strconv.Itoa(int(t.Saturation()*100+0.5))+"%")
		d.brightnessSlider.SetValue(t.Brightness())
		colors = make([]ColorProvider, 101)
		for i := range colors {
			colors[i] = HSB(hue, saturation, float32(i)/float32(len(colors)-1))
		}
		d.brightnessSlider.FillInk = NewEvenlySpacedGradient(geom.Point{}, geom.Point{X: 1}, 0, 0, colors...)
		d.syncText(d.brightnessField, strconv.Itoa(int(t.Brightness()*100+0.5))+"%")
		d.syncText(d.cssField, t.String())
	default:
	}
	d.syncing = false
}

func (d *wellDialog) syncText(field *Field, text string) {
	if !field.Focused() {
		field.SetText(text)
	}
}

func (d *wellDialog) addPreviewBlock(parent *Panel, title string, spaceBefore float32, inkRetriever func() Ink) {
	label := NewLabel()
	label.SetTitle(title)
	label.HAlign = align.Middle
	label.SetLayoutData(&FlexLayoutData{
		HAlign: align.Middle,
		VAlign: align.Middle,
	})
	if spaceBefore > 0 {
		label.SetBorder(NewEmptyBorder(geom.Insets{Top: spaceBefore}))
	}
	parent.AddChild(label)

	preview := NewPanel()
	preview.SetBorder(NewCompoundBorder(
		NewLineBorder(ThemeOnSurface, 0, geom.NewUniformInsets(1), false),
		NewLineBorder(ThemeSurface, 0, geom.NewUniformInsets(1), false),
	))
	preview.SetLayoutData(&FlexLayoutData{
		SizeHint: geom.Size{Width: 64, Height: 64},
	})
	preview.DrawCallback = func(canvas *Canvas, _ geom.Rect) {
		r := preview.ContentRect(false)
		ink := inkRetriever()
		if pattern, ok := ink.(*Pattern); ok {
			canvas.DrawImageInRect(pattern.Image, r, nil, nil)
		} else {
			canvas.DrawRect(r, ink.Paint(canvas, r, paintstyle.Fill))
		}
	}
	parent.AddChild(preview)
}
