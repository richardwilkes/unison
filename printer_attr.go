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

// PrinterAttr holds attributes specific to a printer.
type PrinterAttr struct {
	All map[string]goipp.Values
}

func newPrinterAttr(attrs goipp.Attributes) *PrinterAttr {
	a := &PrinterAttr{All: make(map[string]goipp.Values)}
	for _, one := range attrs {
		a.All[one.Name] = one.Values
	}
	return a
}

// Icons returns a list of icon URLs that can be used to represent this printer.
func (a *PrinterAttr) Icons() []string {
	return a.Strings("printer-icons", nil)
}

// DefaultMedia returns the default media (page size) that will be used.
func (a *PrinterAttr) DefaultMedia() string {
	return a.FirstString("media-default", "")
}

// SupportedMedia returns the supported media (page sizes) that may be used.
func (a *PrinterAttr) SupportedMedia() []string {
	return a.Strings("media-supported", nil)
}

// DefaultColorMode returns the default color mode.
func (a *PrinterAttr) DefaultColorMode() string {
	return a.FirstString("print-color-mode-default", "")
}

// SupportedColorModes returns the supported color modes.
func (a *PrinterAttr) SupportedColorModes() []string {
	return a.Strings("print-color-mode-supported", nil)
}

// MaxCopies returns the maximum number of copies that are supported.
func (a *PrinterAttr) MaxCopies() int {
	return a.FirstInteger("copies-supported", 1)
}

// SupportedDocumentTypes returns the supported document MIME types.
func (a *PrinterAttr) SupportedDocumentTypes() []string {
	return a.Strings("document-format-supported", nil)
}

// SupportedJobCreationAttributes returns the set of attributes that are supported when creating a new job.
func (a *PrinterAttr) SupportedJobCreationAttributes() []string {
	return a.Strings("job-creation-attributes-supported", nil)
}

// SupportedMediaSources returns the supported media sources.
func (a *PrinterAttr) SupportedMediaSources() []string {
	return a.Strings("media-source-supported", nil)
}

// DefaultContentOptimization returns the default content optimization to perform.
func (a *PrinterAttr) DefaultContentOptimization() string {
	return a.FirstString("print-content-optimize-default", "")
}

// SupportedContentOptimizations returns the supported content optimizations.
func (a *PrinterAttr) SupportedContentOptimizations() []string {
	return a.Strings("print-content-optimize-supported", nil)
}

// DefaultSides returns the default sides to print on.
func (a *PrinterAttr) DefaultSides() string {
	return a.FirstString("sides-default", "")
}

// SupportedSides returns the supported sides that may be printed on.
func (a *PrinterAttr) SupportedSides() []string {
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
func (a *PrinterAttr) DefaultOrientation() string {
	return orientationKeyFromInt(a.FirstInteger("orientation-requested-default", 7))
}

// SupportedOrientations returns the supported page orientations.
func (a *PrinterAttr) SupportedOrientations() []string {
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

//nolint:unused,deadcode // Will eventually be used
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
func (a *PrinterAttr) MinimumMargins() geom.Insets[int] {
	return geom.Insets[int]{
		Top:    a.FirstInteger("media-top-margin-supported", 0),
		Left:   a.FirstInteger("media-left-margin-supported", 0),
		Bottom: a.FirstInteger("media-bottom-margin-supported", 0),
		Right:  a.FirstInteger("media-right-margin-supported", 0),
	}
}

// FirstBoolean returns the first boolean value for the given key.
func (a *PrinterAttr) FirstBoolean(key string, def bool) bool {
	if v, ok := a.All[key]; ok && v[0].T.Type() == goipp.TypeBoolean {
		return bool(v[0].V.(goipp.Boolean))
	}
	return def
}

// Booleans returns the boolean values for the given key.
func (a *PrinterAttr) Booleans(key string, def []bool) []bool {
	if v, ok := a.All[key]; ok {
		all := make([]bool, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeBoolean {
				all = append(all, bool(one.V.(goipp.Boolean)))
			}
		}
		return all
	}
	return def
}

// FirstInteger returns the first integer value for the given key.
func (a *PrinterAttr) FirstInteger(key string, def int) int {
	if v, ok := a.All[key]; ok && v[0].T.Type() == goipp.TypeInteger {
		return int(v[0].V.(goipp.Integer))
	}
	return def
}

// Integers returns the integer values for the given key.
func (a *PrinterAttr) Integers(key string, def []int) []int {
	if v, ok := a.All[key]; ok {
		all := make([]int, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeInteger {
				all = append(all, int(one.V.(goipp.Integer)))
			}
		}
		return all
	}
	return def
}

// FirstString returns the first string value for the given key.
func (a *PrinterAttr) FirstString(key, def string) string {
	if v, ok := a.All[key]; ok && v[0].T.Type() == goipp.TypeString {
		return v[0].V.String()
	}
	return def
}

// Strings returns the string values for the given key.
func (a *PrinterAttr) Strings(key string, def []string) []string {
	if v, ok := a.All[key]; ok {
		keywords := make([]string, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeString {
				keywords = append(keywords, one.V.String())
			}
		}
		return keywords
	}
	return def
}

// FirstTime returns the first time value for the given key.
func (a *PrinterAttr) FirstTime(key string, def time.Time) time.Time {
	if v, ok := a.All[key]; ok && v[0].T.Type() == goipp.TypeDateTime {
		return v[0].V.(goipp.Time).Time
	}
	return def
}

// Times returns the time values for the given key.
func (a *PrinterAttr) Times(key string, def []time.Time) []time.Time {
	if v, ok := a.All[key]; ok {
		all := make([]time.Time, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeDateTime {
				all = append(all, one.V.(goipp.Time).Time)
			}
		}
		return all
	}
	return def
}

// FirstResolution returns the first resolution value for the given key.
func (a *PrinterAttr) FirstResolution(key string, def goipp.Resolution) goipp.Resolution {
	if v, ok := a.All[key]; ok && v[0].T.Type() == goipp.TypeResolution {
		return v[0].V.(goipp.Resolution)
	}
	return def
}

// Resolutions returns the Resolution values for the given key.
func (a *PrinterAttr) Resolutions(key string, def []goipp.Resolution) []goipp.Resolution {
	if v, ok := a.All[key]; ok {
		all := make([]goipp.Resolution, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeResolution {
				all = append(all, one.V.(goipp.Resolution))
			}
		}
		return all
	}
	return def
}

// FirstRange returns the first Range value for the given key.
func (a *PrinterAttr) FirstRange(key string, def goipp.Range) goipp.Range {
	if v, ok := a.All[key]; ok && v[0].T.Type() == goipp.TypeRange {
		return v[0].V.(goipp.Range)
	}
	return def
}

// Ranges returns the Range values for the given key.
func (a *PrinterAttr) Ranges(key string, def []goipp.Range) []goipp.Range {
	if v, ok := a.All[key]; ok {
		all := make([]goipp.Range, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeRange {
				all = append(all, one.V.(goipp.Range))
			}
		}
		return all
	}
	return def
}

// FirstTextWithLang returns the first TextWithLang value for the given key.
func (a *PrinterAttr) FirstTextWithLang(key string, def goipp.TextWithLang) goipp.TextWithLang {
	if v, ok := a.All[key]; ok && v[0].T.Type() == goipp.TypeTextWithLang {
		return v[0].V.(goipp.TextWithLang)
	}
	return def
}

// TextWithLangs returns the TextWithLang values for the given key.
func (a *PrinterAttr) TextWithLangs(key string, def []goipp.TextWithLang) []goipp.TextWithLang {
	if v, ok := a.All[key]; ok {
		all := make([]goipp.TextWithLang, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeTextWithLang {
				all = append(all, one.V.(goipp.TextWithLang))
			}
		}
		return all
	}
	return def
}

// FirstBinary returns the first binary value for the given key.
func (a *PrinterAttr) FirstBinary(key string) []byte {
	if v, ok := a.All[key]; ok && v[0].T.Type() == goipp.TypeBinary {
		return v[0].V.(goipp.Binary)
	}
	return nil
}

// Binaries returns the binary values for the given key.
func (a *PrinterAttr) Binaries(key string) [][]byte {
	if v, ok := a.All[key]; ok {
		all := make([][]byte, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeBinary {
				all = append(all, one.V.(goipp.Binary))
			}
		}
		return all
	}
	return nil
}

// FirstCollection returns the first collection value for the given key.
func (a *PrinterAttr) FirstCollection(key string) *PrinterAttr {
	if v, ok := a.All[key]; ok && v[0].T.Type() == goipp.TypeCollection {
		return newPrinterAttr(goipp.Attributes(v[0].V.(goipp.Collection)))
	}
	return &PrinterAttr{}
}

// Collections returns the collection values for the given key.
func (a *PrinterAttr) Collections(key string) []*PrinterAttr {
	if v, ok := a.All[key]; ok {
		all := make([]*PrinterAttr, 0, len(v))
		for _, one := range v {
			if one.T.Type() == goipp.TypeCollection {
				all = append(all, newPrinterAttr(goipp.Attributes(one.V.(goipp.Collection))))
			}
		}
		return all
	}
	return nil
}
