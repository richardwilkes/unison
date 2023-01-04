// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package printing

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xio"
	"github.com/richardwilkes/unison"
)

// JobDialog provides a print job dialog.
type JobDialog struct {
	mgr                   *PrintManager
	printerID             PrinterID
	printer               *Printer
	printersChan          chan *Printer
	scanCancel            func()
	mimeType              string
	printerAttributes     *PrinterAttributes
	jobAttributes         *JobAttributes
	dialog                *unison.Dialog
	printers              *unison.PopupMenu[*Printer]
	img                   *unison.Label
	copies                *unison.NumericField[int]
	pageRanges            *unison.Field
	media                 stringPopup[mediaString]
	mediaSource           stringPopup[capString]
	scaling               stringPopup[capString]
	colorMode             stringPopup[capString]
	contentOptimization   stringPopup[capString]
	sides                 stringPopup[capString]
	orientation           stringPopup[capString]
	lock                  sync.Mutex
	awaitingPrinterUpdate bool
}

func newJobDialog(p *PrintManager, id PrinterID, mimeType string, attributes *JobAttributes) *JobDialog {
	if attributes == nil {
		attributes = make(Attributes).ForJob()
	}
	d := &JobDialog{
		mgr:               p,
		printerID:         id,
		printer:           p.LookupPrinter(id),
		printersChan:      make(chan *Printer, 8),
		mimeType:          mimeType,
		printerAttributes: NewAttributes(nil).ForPrinter(),
		jobAttributes:     attributes.Copy().ForJob(),
	}
	go d.collectPrinters()
	var ctx context.Context
	ctx, d.scanCancel = context.WithCancel(context.Background())
	p.ScanForPrinters(ctx, d.printersChan)
	return d
}

// Printer returns the printer that the dialog was configured for.
func (d *JobDialog) Printer() *Printer {
	return d.printer
}

// JobAttributes returns the job attributes that were configured.
func (d *JobDialog) JobAttributes() *JobAttributes {
	return d.jobAttributes
}

// RunModal presents the dialog and returns true if the user pressed OK.
func (d *JobDialog) RunModal() bool {
	defer d.scanCancel()
	dlg, err := unison.NewDialog(nil, nil, d.createContent(), []*unison.DialogButtonInfo{
		unison.NewCancelButtonInfo(),
		unison.NewOKButtonInfoWithTitle(i18n.Text("Print")),
	})
	if err != nil {
		unison.ErrorDialogWithError(i18n.Text("Unable to create print dialog."), err)
		return false
	}
	dlg.Window().SetTitle(i18n.Text("Print"))
	d.dialog = dlg
	d.dialog.Button(unison.ModalResponseOK).SetEnabled(false)
	dlg.Window().MinMaxContentSizeCallback = func() (min, max unison.Size) {
		_, pref, _ := dlg.Window().Content().Parent().Sizes(unison.Size{})
		return pref, pref
	}
	dlg.Window().Pack()
	d.adjustOKButton(nil, nil)
	if dlg.RunModal() != unison.ModalResponseOK {
		return false
	}
	d.jobAttributes.SetCopies(d.copies.Value())
	if d.printerAttributes.PageRangesSupported() {
		ranges, _ := ExtractPageRanges(d.pageRanges.Text())
		d.jobAttributes.SetPageRanges(ranges)
	} else {
		d.jobAttributes.SetPageRanges(nil)
	}
	d.media.apply(d.printerAttributes.SupportedMedia, d.jobAttributes.SetMedia)
	d.mediaSource.apply(d.printerAttributes.SupportedMediaSources, d.jobAttributes.SetMediaSource)
	d.scaling.apply(d.printerAttributes.SupportedPrintScaling, d.jobAttributes.SetPrintScaling)
	d.colorMode.apply(d.printerAttributes.SupportedColorModes, d.jobAttributes.SetColorMode)
	d.contentOptimization.apply(d.printerAttributes.SupportedContentOptimizations, d.jobAttributes.SetContentOptimization)
	d.sides.apply(d.printerAttributes.SupportedSides, d.jobAttributes.SetSides)
	d.orientation.apply(d.printerAttributes.SupportedOrientations, d.jobAttributes.SetOrientation)
	return true
}

func (d *JobDialog) createContent() unison.Paneler {
	content := unison.NewPanel()
	content.SetLayout(&unison.FlexLayout{
		Columns:  1,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	d.createPrinterPopup(content)
	bottom := unison.NewPanel()
	bottom.SetLayout(&unison.FlexLayout{
		Columns:  2,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	content.AddChild(bottom)
	left := unison.NewPanel()
	left.SetLayout(&unison.FlexLayout{
		Columns:  2,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	left.SetLayoutData(&unison.FlexLayoutData{HGrab: true})
	bottom.AddChild(left)
	d.img = unison.NewLabel()
	d.img.SetBorder(unison.NewEmptyBorder(unison.Insets{Left: unison.StdHSpacing}))
	d.img.SetLayoutData(&unison.FlexLayoutData{
		HAlign: unison.MiddleAlignment,
		VAlign: unison.MiddleAlignment,
	})
	bottom.AddChild(d.img)
	d.createCopies(left)
	d.createPageRanges(left)
	d.sides = createCapStringPopup[capString](left, i18n.Text("Sides"))
	d.orientation = createCapStringPopup[capString](left, i18n.Text("Orientation"))
	d.media = createCapStringPopup[mediaString](left, i18n.Text("Media"))
	d.mediaSource = createCapStringPopup[capString](left, i18n.Text("Tray"))
	d.scaling = createCapStringPopup[capString](left, i18n.Text("Scaling"))
	d.colorMode = createCapStringPopup[capString](left, i18n.Text("Color Mode"))
	d.contentOptimization = createCapStringPopup[capString](left, i18n.Text("Optimize For"))
	d.rebuildPrinterPopup() // In case no printers are found, we need to manually trigger the rebuild
	return content
}

func (d *JobDialog) createPrinterPopup(parent *unison.Panel) {
	d.printers = unison.NewPopupMenu[*Printer]()
	d.printers.SetBorder(unison.NewEmptyBorder(unison.Insets{Bottom: unison.StdVSpacing * 4}))
	d.printers.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  2,
		HAlign: unison.MiddleAlignment,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})
	parent.AddChild(d.printers)
}

func (d *JobDialog) rebuildPrinterPopup() {
	d.printers.SelectionChangedCallback = nil
	d.printers.RemoveAllItems()
	d.lock.Lock()
	d.awaitingPrinterUpdate = false
	d.lock.Unlock()
	var sel *Printer
	for _, p := range d.mgr.Printers() {
		d.printers.AddItem(p)
		if p.UUID == d.printerID.UUID {
			sel = p
		}
	}
	disabled := d.printers.ItemCount() == 0
	if disabled {
		d.printers.AddDisabledItem(&Printer{
			PrinterID: PrinterID{
				Name: i18n.Text("Searching for printers…"),
			},
		})
	}
	d.printers.SetEnabled(!disabled)
	d.printers.Select(sel)
	d.printers.MarkForLayoutAndRedraw()
	if p := d.printers.Parent(); p != nil {
		p.NeedsLayout = true
	}
	if sel != d.printer {
		d.setPrinter(sel)
	}
	if !disabled && d.printer == nil {
		sel, _ = d.printers.ItemAt(0)
		d.setPrinter(sel)
	}
	d.adjustEnablement()
	d.printers.SelectionChangedCallback = d.printerPopupSelectionHandler
	if d.dialog != nil {
		d.dialog.Window().Pack()
	}
}

func (d *JobDialog) printerPopupSelectionHandler(popup *unison.PopupMenu[*Printer]) {
	if printer, ok := popup.Selected(); ok {
		d.setPrinter(printer)
	}
}

func (d *JobDialog) setPrinter(printer *Printer) {
	d.printer = printer
	d.printerID = d.printer.PrinterID
	var err error
	if d.printerAttributes, err = d.printer.Attributes(15*time.Second, true); err != nil {
		jot.Error(err)
	}
	if icon := d.retrieveIcon(); icon != nil {
		d.img.Drawable = icon
	}
	d.copies.SetMinMax(1, d.printerAttributes.MaxCopies())
	d.media.rebuild(d.printerAttributes.SupportedMedia, d.jobAttributes.Media, d.printerAttributes.DefaultMedia)
	d.mediaSource.rebuild(d.printerAttributes.SupportedMediaSources, d.jobAttributes.MediaSource,
		d.printerAttributes.DefaultMediaSource)
	d.scaling.rebuild(d.printerAttributes.SupportedPrintScaling, d.jobAttributes.PrintScaling,
		d.printerAttributes.DefaultPrintScaling)
	d.colorMode.rebuild(d.printerAttributes.SupportedColorModes, d.jobAttributes.ColorMode,
		d.printerAttributes.DefaultColorMode)
	d.contentOptimization.rebuild(d.printerAttributes.SupportedContentOptimizations, d.jobAttributes.ContentOptimization,
		d.printerAttributes.DefaultContentOptimization)
	d.sides.rebuild(d.printerAttributes.SupportedSides, d.jobAttributes.Sides,
		d.printerAttributes.DefaultSides)
	d.orientation.rebuild(d.printerAttributes.SupportedOrientations, d.jobAttributes.Orientation,
		d.printerAttributes.DefaultOrientation)
	d.adjustEnablement()
	d.printers.Select(printer)
}

func (d *JobDialog) retrieveIcon() *unison.Image {
	if icons := d.printerAttributes.Icons(); len(icons) != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, icons[len(icons)-1], http.NoBody)
		if err != nil {
			jot.Error(errs.NewWithCause("unable to create request for link: "+icons[len(icons)-1], err))
			return nil
		}
		req.Header.Add("Accept-Encoding", "identity")
		var rsp *http.Response
		if rsp, err = d.printer.httpClient.Do(req); err != nil { //nolint:bodyclose // Body is closed by xio.CloseIgnoringErrors
			jot.Error(errs.NewWithCause("unable to initiate download for link: "+icons[len(icons)-1], err))
			return nil
		}
		defer xio.CloseIgnoringErrors(rsp.Body)
		var content []byte
		if content, err = io.ReadAll(rsp.Body); err != nil {
			jot.Error(errs.NewWithCause("unable to read body for link: "+icons[len(icons)-1], err))
			return nil
		}
		var img *unison.Image
		if img, err = unison.NewImageFromBytes(content, 0.5); err != nil {
			jot.Error(errs.NewWithCause("unable to create image from data for link: "+icons[len(icons)-1], err))
			return nil
		}
		return img
	}
	return nil
}

func (d *JobDialog) createCopies(parent *unison.Panel) {
	d.copies = unison.NewNumericField(d.jobAttributes.Copies(), 1, d.printerAttributes.MaxCopies(), strconv.Itoa,
		strconv.Atoi, func(min, max int) []int { return []int{max} })
	d.copies.ModifiedCallback = d.adjustOKButton
	d.copies.SetLayoutData(&unison.FlexLayoutData{VAlign: unison.MiddleAlignment})

	parent.AddChild(createLabel(i18n.Text("Copies")))
	parent.AddChild(d.copies)
}

func (d *JobDialog) createPageRanges(parent *unison.Panel) {
	d.pageRanges = unison.NewField()
	d.pageRanges.Tooltip = unison.NewTooltipWithText(i18n.Text(`A page range in the form "5" or "9-12" or multiple
separated by commas, such as "1, 3-4 in ascending
order with no overlapping ranges`))
	d.pageRanges.ValidateCallback = func() bool {
		ranges, noErrors := ExtractPageRanges(d.pageRanges.Text())
		if noErrors {
			return ValidPageRanges(ranges)
		}
		return false
	}
	d.pageRanges.ModifiedCallback = d.adjustOKButton
	d.pageRanges.SetText(FormatPageRanges(d.jobAttributes.PageRanges()))
	d.pageRanges.SetLayoutData(&unison.FlexLayoutData{
		HAlign: unison.FillAlignment,
		VAlign: unison.MiddleAlignment,
		HGrab:  true,
	})

	parent.AddChild(createLabel(i18n.Text("Page Ranges")))
	parent.AddChild(d.pageRanges)
}

func createLabel(text string) *unison.Label {
	label := unison.NewLabel()
	label.Text = text
	label.HAlign = unison.EndAlignment
	label.SetLayoutData(&unison.FlexLayoutData{
		HAlign: unison.EndAlignment,
		VAlign: unison.MiddleAlignment,
	})
	return label
}

func (d *JobDialog) adjustEnablement() {
	enabled := d.printer != nil
	d.copies.SetEnabled(enabled)
	d.pageRanges.SetEnabled(enabled && d.printerAttributes.PageRangesSupported())
	d.media.setEnabled(enabled, d.printerAttributes.SupportedMedia)
	d.mediaSource.setEnabled(enabled, d.printerAttributes.SupportedMediaSources)
	d.scaling.setEnabled(enabled, d.printerAttributes.SupportedPrintScaling)
	d.colorMode.setEnabled(enabled, d.printerAttributes.SupportedColorModes)
	d.contentOptimization.setEnabled(enabled, d.printerAttributes.SupportedContentOptimizations)
	d.sides.setEnabled(enabled, d.printerAttributes.SupportedSides)
	d.orientation.setEnabled(enabled, d.printerAttributes.SupportedOrientations)
	d.adjustOKButton(nil, nil)
}

func (d *JobDialog) adjustOKButton(_, _ *unison.FieldState) {
	if d.dialog == nil {
		return
	}
	enabled := d.printer != nil
	if !d.copies.ValidateCallback() {
		enabled = false
	}
	if d.printerAttributes.PageRangesSupported() && !d.pageRanges.ValidateCallback() {
		enabled = false
	}
	d.dialog.Button(unison.ModalResponseOK).SetEnabled(enabled)
}

func (d *JobDialog) collectPrinters() {
	for range d.printersChan {
		d.lock.Lock()
		if !d.awaitingPrinterUpdate {
			d.awaitingPrinterUpdate = true
			unison.InvokeTask(d.rebuildPrinterPopup)
		}
		d.lock.Unlock()
	}
}
