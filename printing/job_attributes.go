// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package printing

import "github.com/OpenPrinting/goipp"

const (
	copiesKey              = "copies"
	pageRangesKey          = "page-ranges"
	mediaKey               = "media"
	pdfFitToPageKey        = "pdf-fit-to-page"
	printScalingKey        = "print-scaling"
	printColorModeKey      = "print-color-mode"
	mediaSourceKey         = "media-source"
	contentOptimizationKey = "print-content-optimize"
	sidesKey               = "sides"
	orientationKey         = "orientation-requested"
)

// JobAttributes holds attributes specific to a printer job.
type JobAttributes struct {
	Attributes
}

// Copies returns the number of copies to make.
func (a *JobAttributes) Copies() int {
	return a.Integer(copiesKey, 1)
}

// SetCopies sets the number of copies to make.
func (a *JobAttributes) SetCopies(count int) {
	if count < 1 {
		count = 1
	}
	a.SetInteger(copiesKey, count, true)
}

// PageRanges returns the page ranges.
func (a *JobAttributes) PageRanges() []goipp.Range {
	return a.Ranges(pageRangesKey, nil)
}

// SetPageRanges sets the page ranges. Pass in nil to remove the page range restriction.
func (a *JobAttributes) SetPageRanges(ranges []goipp.Range) {
	if len(ranges) == 0 {
		delete(a.Attributes, pageRangesKey)
	} else {
		for i, r := range ranges {
			a.SetRange(pageRangesKey, r, i == 0)
		}
	}
}

// Media returns the media (page size).
func (a *JobAttributes) Media() string {
	return a.String(mediaKey, "")
}

// SetMedia sets the media.
func (a *JobAttributes) SetMedia(media string) {
	a.SetKeyword(mediaKey, media, true)
}

// PDFFitToPage returns true if PDFs should be scaled to fit to the page.
func (a *JobAttributes) PDFFitToPage() bool {
	return a.Boolean(pdfFitToPageKey, false)
}

// SetPDFFitToPage returns whether PDFs should be scaled to fit to the page.
func (a *JobAttributes) SetPDFFitToPage(fit bool) {
	a.SetBoolean(pdfFitToPageKey, fit, true)
}

// PrintScaling returns the print scaling.
func (a *JobAttributes) PrintScaling() string {
	return a.String(printScalingKey, "")
}

// SetPrintScaling sets the print scaling.
func (a *JobAttributes) SetPrintScaling(scaling string) {
	a.SetKeyword(printScalingKey, scaling, true)
}

// ColorMode returns the color mode.
func (a *JobAttributes) ColorMode() string {
	return a.String(printColorModeKey, "")
}

// SetColorMode sets the color mode.
func (a *JobAttributes) SetColorMode(mode string) {
	a.SetKeyword(printColorModeKey, mode, true)
}

// MediaSource returns the media source.
func (a *JobAttributes) MediaSource() string {
	return a.String(mediaSourceKey, "")
}

// SetMediaSource sets the media source.
func (a *JobAttributes) SetMediaSource(src string) {
	a.SetKeyword(mediaSourceKey, src, true)
}

// ContentOptimization returns the content optimization.
func (a *JobAttributes) ContentOptimization() string {
	return a.String(contentOptimizationKey, "")
}

// SetContentOptimization sets the content optimization.
func (a *JobAttributes) SetContentOptimization(optimization string) {
	a.SetKeyword(contentOptimizationKey, optimization, true)
}

// Sides returns the sides.
func (a *JobAttributes) Sides() string {
	return a.String(sidesKey, "")
}

// SetSides sets the sides.
func (a *JobAttributes) SetSides(sides string) {
	a.SetKeyword(sidesKey, sides, true)
}

// Orientation returns the page orientation.
func (a *JobAttributes) Orientation() string {
	return orientationKeyFromInt(a.Integer(orientationKey, 7))
}

// SetOrientation sets the page orientation.
func (a *JobAttributes) SetOrientation(orientation string) {
	a.SetEnum(orientationKey, orientationFromKey(orientation), true)
}
