// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison"
)

var windowCounter int

// NewDemoWindow creates and displays our demo window.
func NewDemoWindow(where geom32.Point) (*unison.Window, error) {
	// Create the window
	windowCounter++
	wnd, err := unison.NewWindow(fmt.Sprintf("Demo #%d", windowCounter))
	if err != nil {
		return nil, err
	}

	// Install our menus
	unison.DefaultMenuFactory().BarForWindow(wnd, func(m unison.Menu) {
		unison.InsertStdMenus(m, ShowAboutWindow, nil, nil)
		fileMenu := m.Menu(unison.FileMenuID)
		f := fileMenu.Factory()
		fileMenu.InsertItem(0, NewWindowAction.NewMenuItem(f))
		fileMenu.InsertItem(1, OpenAction.NewMenuItem(f))
	})

	// Put some empty space around the edges of our window and apply a single column layout.
	content := wnd.Content()
	content.SetBorder(unison.NewEmptyBorder(geom32.NewUniformInsets(10)))
	content.SetLayout(&unison.FlexLayout{
		Columns:  1,
		HSpacing: unison.StdHSpacing,
		VSpacing: 10,
	})

	// Create a wrappable row of buttons
	panel := createButtonsPanel()
	panel.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})
	content.AddChild(panel)

	// Create a wrappable row of image buttons
	panel = createImageButtonsPanel()
	panel.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})
	content.AddChild(panel)

	addSeparator(content)

	// Create a column of checkboxes
	panel = createCheckBoxPanel()
	panel.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})
	content.AddChild(panel)

	addSeparator(content)

	// Create a column of radio buttons and a progress bar they control
	panel = createRadioButtonsAndProgressBarsPanel()
	panel.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: unison.FillAlignment,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})
	content.AddChild(panel)

	addSeparator(content)

	// Create a column of popup menus
	panel = createPopupMenusPanel()
	panel.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})
	content.AddChild(panel)

	addSeparator(content)

	// Create some fields and a list, side-by-side
	panel = createFieldsAndListPanel()
	panel.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: unison.FillAlignment,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})
	content.AddChild(panel)

	addSeparator(content)

	// Create an image panel, but don't add it yet
	imgPanel := createImagePanel()

	// Create some color wells and pass it our image panel
	panel = createWellsPanel(imgPanel)
	panel.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})
	content.AddChild(panel)

	// Create a scroll panel and place the image panel inside it
	scrollArea := unison.NewScrollPanel()
	scrollArea.SetContent(imgPanel, unison.UnmodifiedBehavior)
	scrollArea.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: unison.FillAlignment,
		VAlign: unison.FillAlignment,
		HGrab:  true,
		VGrab:  true,
	})
	content.AddChild(scrollArea)

	// Pack our window to fit its content, then set its location on the display and make it visible.
	wnd.Pack()
	rect := wnd.FrameRect()
	rect.Point = where
	wnd.SetFrameRect(rect)
	wnd.ToFront()

	return wnd, nil
}

func createButtonsPanel() *unison.Panel {
	// Create a panel to place some buttons into.
	panel := unison.NewPanel()
	panel.SetLayout(&unison.FlowLayout{
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})

	// Add some buttons
	for i, title := range []string{"First", "Second", "Third", "Fourth", "Fifth", "Sixth", "Seventh", "Eighth", "Ninth"} {
		btn := createButton(title, panel)
		if i == 2 {
			btn.SetEnabled(false)
		}
	}

	return panel
}

func createButton(title string, panel *unison.Panel) *unison.Button {
	btn := unison.NewButton()
	btn.Text = title
	btn.ClickCallback = func() { jot.Info(title) }
	btn.Tooltip = unison.NewTooltipWithText(fmt.Sprintf("Tooltip for: %s", title))
	btn.SetLayoutData(unison.MiddleAlignment)
	panel.AddChild(btn)
	return btn
}

func createImageButtonsPanel() *unison.Panel {
	// Create a panel to place some buttons into.
	panel := unison.NewPanel()
	panel.SetLayout(&unison.FlowLayout{
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})

	// Load our home image, and if successful (we should be!), add two buttons with it, one enabled and one not.
	homeImg, err := HomeImage()
	if err != nil {
		jot.Error(err)
	} else {
		createImageButton(homeImg, "home_enabled", panel)
		createImageButton(homeImg, "home_disabled", panel).SetEnabled(false)
	}

	// Load our logo image, and if successful (we should be!), add two buttons with it, one enabled and one not.
	var logoImg *unison.Image
	if logoImg, err = ClassicAppleLogoImage(); err != nil {
		jot.Error(err)
	} else {
		createImageButton(logoImg, "logo_enabled", panel)
		createImageButton(logoImg, "logo_disabled", panel).SetEnabled(false)
	}

	if homeImg != nil && logoImg != nil {
		// Add spacer
		spacer := &unison.Panel{}
		spacer.Self = spacer
		spacer.SetSizer(func(_ geom32.Size) (min, pref, max geom32.Size) {
			min.Width = 40
			pref.Width = 40
			max.Width = 40
			return
		})
		panel.AddChild(spacer)

		// Add some sticky buttons in a group with our images
		group := unison.NewGroup()
		first := createImageButton(homeImg, "home_toggle", panel)
		first.Sticky = true
		group.Add(first.AsGroupPanel())
		second := createImageButton(logoImg, "logo_toggle", panel)
		second.Sticky = true
		group.Add(second.AsGroupPanel())
		group.Select(first.AsGroupPanel())
	}

	return panel
}

func createImageButton(img *unison.Image, actionText string, panel *unison.Panel) *unison.Button {
	btn := unison.NewButton()
	btn.Image = img
	btn.ClickCallback = func() { jot.Info(actionText) }
	btn.Tooltip = unison.NewTooltipWithText(fmt.Sprintf("Tooltip for: %s", actionText))
	btn.SetLayoutData(unison.MiddleAlignment)
	panel.AddChild(btn)
	return btn
}

func addSeparator(parent *unison.Panel) {
	sep := unison.NewSeparator()
	sep.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: unison.FillAlignment,
		VAlign: unison.MiddleAlignment,
	})
	parent.AddChild(sep)
}

func createCheckBoxPanel() *unison.Panel {
	panel := unison.NewPanel()
	panel.SetLayout(&unison.FlexLayout{
		Columns:  1,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	createCheckBox("Initially Off", unison.OffCheckState, panel)
	createCheckBox("Initially On", unison.OnCheckState, panel)
	createCheckBox("Initially Mixed", unison.MixedCheckState, panel)
	createCheckBox("Disabled", unison.OffCheckState, panel).SetEnabled(false)
	createCheckBox("Disabled w/Check", unison.OnCheckState, panel).SetEnabled(false)
	createCheckBox("Disabled w/Mixed", unison.MixedCheckState, panel).SetEnabled(false)
	return panel
}

func createCheckBox(title string, initialState unison.CheckState, panel *unison.Panel) *unison.CheckBox {
	check := unison.NewCheckBox()
	check.Text = title
	check.State = initialState
	check.ClickCallback = func() { jot.Infof("'%s' was clicked.", title) }
	check.Tooltip = unison.NewTooltipWithText(fmt.Sprintf("This is the tooltip for '%s'", title))
	panel.AddChild(check)
	return check
}

func createRadioButtonsAndProgressBarsPanel() *unison.Panel {
	// Create a wrapper to put them side-by-side
	wrapper := unison.NewPanel()
	wrapper.SetLayout(&unison.FlexLayout{
		Columns:      2,
		HSpacing:     10,
		VSpacing:     unison.StdVSpacing,
		VAlign:       unison.MiddleAlignment,
		EqualColumns: true,
	})

	// Create the progress bar, but don't add it yet
	progress := unison.NewProgressBar(100)
	progress.SetCurrent(25)
	progress.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: unison.FillAlignment,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})

	// Create the radio buttons that will control the progress bar
	panel := unison.NewPanel()
	panel.SetLayout(&unison.FlexLayout{
		Columns:  1,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	group := unison.NewGroup()
	first := createRadioButton("25%", panel, group, progress, 25, 100)
	createRadioButton("50%", panel, group, progress, 50, 100)
	createRadioButton("75%", panel, group, progress, 75, 100).SetEnabled(false)
	createRadioButton("100%", panel, group, progress, 100, 100)
	createRadioButton("Indeterminate", panel, group, progress, 0, 0)
	group.Select(first.AsGroupPanel())

	// Add the radio buttons to the left
	wrapper.AddChild(panel)

	// Add the progress bar to the right
	wrapper.AddChild(progress)

	return wrapper
}

func createRadioButton(title string, panel *unison.Panel, group *unison.Group, progressBar *unison.ProgressBar, current, max float32) *unison.RadioButton {
	rb := unison.NewRadioButton()
	rb.Text = title
	rb.ClickCallback = func() {
		progressBar.SetMaximum(max)
		progressBar.SetCurrent(current)
		jot.Infof("%s was clicked.", title)
	}
	rb.Tooltip = unison.NewTooltipWithText(fmt.Sprintf("This is the tooltip for %s", title))
	panel.AddChild(rb)
	group.Add(rb.AsGroupPanel())
	return rb
}

func createPopupMenusPanel() *unison.Panel {
	panel := unison.NewPanel()
	panel.SetLayout(&unison.FlexLayout{
		Columns:  2,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	createPopupMenu(panel, 1, "Alphabet Tooltip", "Alpha", "Beta", "Charlie", "", "Delta", "Echo", "Foxtrot")
	createPopupMenu(panel, 2, "Color Tooltip", "Red", "Blue", "Green").SetEnabled(false)
	return panel
}

func createPopupMenu(panel *unison.Panel, selection int, tooltip string, titles ...string) *unison.PopupMenu {
	p := unison.NewPopupMenu()
	p.Tooltip = unison.NewTooltipWithText(tooltip)
	for _, title := range titles {
		if title == "" {
			p.AddSeparator()
		} else {
			p.AddItem(title)
		}
	}
	p.SelectIndex(selection)
	p.SelectionCallback = func() { jot.Infof("The '%v' item was selected from the %s PopupMenu.", p.Selected(), tooltip) }
	panel.AddChild(p)
	return p
}

func createFieldsAndListPanel() *unison.Panel {
	// Create a wrapper to put them side-by-side
	wrapper := unison.NewPanel()
	wrapper.SetLayout(&unison.FlexLayout{
		Columns:      2,
		HSpacing:     10,
		VSpacing:     unison.StdVSpacing,
		EqualColumns: true,
	})

	// Add the text fields to the left side
	textFieldsPanel := createTextFieldsPanel()
	textFieldsPanel.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: unison.FillAlignment,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})
	wrapper.AddChild(textFieldsPanel)

	// Add the list to the right side
	wrapper.AddChild(createListPanel())

	return wrapper
}

func createTextFieldsPanel() *unison.Panel {
	panel := unison.NewPanel()
	panel.SetLayout(&unison.FlexLayout{
		Columns:  2,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	createTextField("Field 1:", "First Text Field", panel)
	createTextField("Field 2:", "Second Text Field (disabled)", panel).SetEnabled(false)
	field := createTextField("Longer Label:", "", panel)
	field.Watermark = "Watermarked"
	field = createTextField("Field 4:", "", panel)
	field.Watermark = "Enter only numbers"
	field.ValidateCallback = func() bool {
		for _, r := range field.Text() {
			if !unicode.IsDigit(r) {
				return false
			}
		}
		return true
	}
	return panel
}

func createTextField(labelText, fieldText string, panel *unison.Panel) *unison.Field {
	lbl := unison.NewLabel()
	lbl.Text = labelText
	lbl.HAlign = unison.EndAlignment
	lbl.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: unison.EndAlignment,
		VAlign: unison.MiddleAlignment,
	})
	panel.AddChild(lbl)
	field := unison.NewField()
	field.SetText(fieldText)
	field.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: unison.FillAlignment,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})
	field.Tooltip = unison.NewTooltipWithText(fmt.Sprintf("This is the tooltip for %v", field))
	panel.AddChild(field)
	return field
}

func createListPanel() *unison.Panel {
	lst := unison.NewList()
	lst.Append(
		"One",
		"Two",
		"Three with some long text to make it interesting",
		"Four",
		"Five",
	)
	lst.NewSelectionCallback = func() {
		var buffer strings.Builder
		buffer.WriteString("Selection changed in the list. Now:")
		index := -1
		first := true
		for {
			index = lst.Selection.NextSet(index + 1)
			if index == -1 {
				break
			}
			if first {
				first = false
			} else {
				buffer.WriteString(",")
			}
			fmt.Fprintf(&buffer, " %d", index)
		}
		jot.Info(buffer.String())
	}
	lst.DoubleClickCallback = func() {
		jot.Info("Double-clicked on the list")
	}
	_, prefSize, _ := lst.Sizes(geom32.Size{})
	lst.SetFrameRect(geom32.Rect{Size: prefSize})
	scroller := unison.NewScrollPanel()
	scroller.SetBorder(unison.NewLineBorder(unison.DividerColor, 0, geom32.NewUniformInsets(1), false))
	scroller.SetContent(lst, unison.FillBehavior)
	scroller.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: unison.FillAlignment,
		VAlign: unison.FillAlignment,
		HGrab:  true,
		VGrab:  true,
	})
	return scroller.AsPanel()
}

func createImagePanel() *unison.Label {
	// Create the label and make it focusable
	imgPanel := unison.NewLabel()
	imgPanel.SetFocusable(true)

	// Prepare a cursor for when the mouse is over the image
	cursor := unison.ClosedHandCursor()
	if logoImg, err := ClassicAppleLogoImage(); err != nil {
		jot.Error(err)
	} else {
		size := logoImg.LogicalSize()
		cursor = unison.NewCursor(logoImg, geom32.Point{
			X: size.Width / 2,
			Y: size.Height / 2,
		})
	}
	imgPanel.UpdateCursorCallback = func(where geom32.Point) *unison.Cursor { return cursor }

	// Add a tooltip that shows the current mouse coordinates
	imgPanel.UpdateTooltipCallback = func(where geom32.Point, avoid geom32.Rect) geom32.Rect {
		imgPanel.Tooltip = unison.NewTooltipWithText(where.String())
		avoid.X = where.X - 16
		avoid.Y = where.Y - 16
		avoid.Point = imgPanel.PointToRoot(avoid.Point)
		avoid.Width = 32
		avoid.Height = 32
		return avoid
	}

	// Set the initial image
	img, err := MountainsImage()
	if err != nil {
		jot.Error(err)
	} else {
		imgPanel.Image = img
	}

	// Set the set of the widget to match its preferred size
	_, prefSize, _ := imgPanel.Sizes(geom32.Size{})
	imgPanel.SetFrameRect(geom32.Rect{Size: prefSize})

	return imgPanel
}

func createWellsPanel(imgPanel *unison.Label) *unison.Panel {
	// Create the panel that's going to hold the wells
	panel := unison.NewPanel()
	panel.SetLayout(&unison.FlowLayout{
		HSpacing: 5,
		VSpacing: 5,
	})

	// Add a well
	well1 := unison.NewWell()
	well1.SetInk(unison.Yellow)
	panel.AddChild(well1)

	// When this well is changed, if the user sets an image, we'll change the image panel to match it
	well1.InkChangedCallback = func() {
		ink := well1.Ink()
		if pattern, ok := ink.(*unison.Pattern); ok {
			imgPanel.Image = pattern.Image
			_, pSize, _ := imgPanel.Sizes(geom32.Size{})
			imgPanel.SetFrameRect(geom32.Rect{Size: pSize})
			imgPanel.MarkForRedraw()
		}
	}

	// Add another, disabled, well
	well2 := unison.NewWell()
	well2.SetInk(unison.Orange)
	well2.SetEnabled(false)

	panel.AddChild(well2)
	return panel
}
