// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package printing

import (
	"github.com/OpenPrinting/goipp"
	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/xmath/geom"
)

// Possible orientations
const (
	Portrait         = "portrait"
	Landscape        = "landscape"
	ReverseLandscape = "reverse-landscape"
	ReversePortrait  = "reverse-portrait"
)

// Possible sides
const (
	OneSided      = "one-sided"
	TwoSidedLong  = "two-sided-long-edge"
	TwoSidedShort = "two-sided-short-edge"
)

// PrinterAttributes holds attributes specific to a printer.
type PrinterAttributes struct {
	Attributes
}

// Icons returns a list of icon URLs that can be used to represent this printer.
func (a *PrinterAttributes) Icons() []string {
	return a.Strings("printer-icons", nil)
}

// PageRangesSupported returns true if page ranges are supported.
func (a *PrinterAttributes) PageRangesSupported() bool {
	return a.Boolean("page-ranges-supported", false)
}

// DefaultMedia returns the default media (page size) that will be used.
func (a *PrinterAttributes) DefaultMedia() string {
	return a.String("media-default", "")
}

// SupportedMedia returns the supported media (page sizes) that may be used.
func (a *PrinterAttributes) SupportedMedia() []string {
	return a.Strings("media-supported", nil)
}

// DefaultPDFFitToPage returns the default PDF fit-to-page value that will be used.
func (a *PrinterAttributes) DefaultPDFFitToPage() bool {
	return a.Boolean("pdf-fit-to-page-default", false)
}

// SupportedPDFFitToPage returns the supported PDF fit-to-page values that may be used.
func (a *PrinterAttributes) SupportedPDFFitToPage() []bool {
	return a.Booleans("pdf-fit-to-page-supported", nil)
}

// DefaultPrintScaling returns the default print scaling that will be used.
func (a *PrinterAttributes) DefaultPrintScaling() string {
	return a.String("print-scaling-default", "")
}

// SupportedPrintScaling returns the supported print scaling that may be used.
func (a *PrinterAttributes) SupportedPrintScaling() []string {
	return a.Strings("print-scaling-supported", nil)
}

// DefaultColorMode returns the default color mode.
func (a *PrinterAttributes) DefaultColorMode() string {
	return a.String("print-color-mode-default", "")
}

// SupportedColorModes returns the supported color modes.
func (a *PrinterAttributes) SupportedColorModes() []string {
	return a.Strings("print-color-mode-supported", nil)
}

// MaxCopies returns the maximum number of copies that are supported.
func (a *PrinterAttributes) MaxCopies() int {
	return a.Range("copies-supported", goipp.Range{
		Lower: 1,
		Upper: 1,
	}).Upper
}

// SupportedDocumentTypes returns the supported document MIME types.
func (a *PrinterAttributes) SupportedDocumentTypes() []string {
	return a.Strings("document-format-supported", nil)
}

// SupportedJobCreationAttributes returns the set of attributes that are supported when creating a new job.
func (a *PrinterAttributes) SupportedJobCreationAttributes() []string {
	return a.Strings("job-creation-attributes-supported", nil)
}

// DefaultMediaSource returns the default media source.
func (a *PrinterAttributes) DefaultMediaSource() string {
	return a.String("media-source-default", "")
}

// SupportedMediaSources returns the supported media sources.
func (a *PrinterAttributes) SupportedMediaSources() []string {
	return a.Strings("media-source-supported", nil)
}

// DefaultContentOptimization returns the default content optimization to perform.
func (a *PrinterAttributes) DefaultContentOptimization() string {
	return a.String("print-content-optimize-default", "")
}

// SupportedContentOptimizations returns the supported content optimizations.
func (a *PrinterAttributes) SupportedContentOptimizations() []string {
	return a.Strings("print-content-optimize-supported", nil)
}

// DefaultSides returns the default sides to print on.
func (a *PrinterAttributes) DefaultSides() string {
	return a.String("sides-default", "")
}

// SupportedSides returns the supported sides that may be printed on.
func (a *PrinterAttributes) SupportedSides() []string {
	return a.Strings("sides-supported", nil)
}

// SidePresentationName returns the presentation name (i.e. for humans to read) for the given side key.
func SidePresentationName(key string) string {
	switch key {
	case OneSided:
		return i18n.Text("One-Sided")
	case TwoSidedLong:
		return i18n.Text("Two-Sided, Long Edge")
	case TwoSidedShort:
		return i18n.Text("Two-Sided, Short Edge")
	default:
		return key
	}
}

// DefaultOrientation returns the default page orientation.
func (a *PrinterAttributes) DefaultOrientation() string {
	return orientationKeyFromInt(a.Integer("orientation-requested-default", 7))
}

// SupportedOrientations returns the supported page orientations.
func (a *PrinterAttributes) SupportedOrientations() []string {
	var keys []string
	for _, one := range a.Integers("orientation-requested-supported", nil) {
		if key := orientationKeyFromInt(one); key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

// OrientationPresentationName returns the presentation name (i.e. for humans to read) for the given orientation key.
func OrientationPresentationName(key string) string {
	switch key {
	case Portrait:
		return i18n.Text("Portrait")
	case Landscape:
		return i18n.Text("Landscape")
	case ReverseLandscape:
		return i18n.Text("Reverse Landscape")
	case ReversePortrait:
		return i18n.Text("Reverse Portrait")
	default:
		return key
	}
}

func orientationKeyFromInt(value int) string {
	switch value {
	case 3:
		return Portrait
	case 4:
		return Landscape
	case 5:
		return ReverseLandscape
	case 6:
		return ReversePortrait
	default:
		return ""
	}
}

func orientationFromKey(key string) int {
	switch key {
	case Portrait:
		return 3
	case Landscape:
		return 4
	case ReverseLandscape:
		return 5
	case ReversePortrait:
		return 6
	default:
		return 7 // "none"
	}
}

// MinimumMargins returns the minimum margins for a page, given in hundreths of millimeters (the equivalent of 1/2540's
// of an inch).
func (a *PrinterAttributes) MinimumMargins() geom.Insets[int] {
	return geom.Insets[int]{
		Top:    a.Integer("media-top-margin-supported", 0),
		Left:   a.Integer("media-left-margin-supported", 0),
		Bottom: a.Integer("media-bottom-margin-supported", 0),
		Right:  a.Integer("media-right-margin-supported", 0),
	}
}
