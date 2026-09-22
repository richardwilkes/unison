// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"strconv"

	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// The names of the text attributes this package reports, which are the ones ATK's atk_text_attribute_get_name returns
// for the members of AtkTextAttribute that describe a font. They are what an assistive technology matches against: Orca
// reads weight to decide whether a stretch of text is bold, style for italic, underline and strikethrough for the two
// decorations, and family-name and size when the user asks what the text is drawn in.
const (
	weightTextAttribute        = "weight"
	styleTextAttribute         = "style"
	underlineTextAttribute     = "underline"
	strikethroughTextAttribute = "strikethrough"
	familyNameTextAttribute    = "family-name"
	sizeTextAttribute          = "size"
)

// The values of the two text attributes that are neither a number nor a boolean. AT-SPI names more of each — oblique
// for a slanted face that is not italic, double and low for the other underlines — but a Unison font says only whether
// it is italic and whether it is underlined at all.
const (
	normalStyleValue     = "normal"
	italicStyleValue     = "italic"
	noUnderlineValue     = "none"
	singleUnderlineValue = "single"
)

// regularWeight is the weight reported for a run that does not say what its own is, which is what 400 means on the
// hundred-to-nine-hundred scale every font system shares.
const regularWeight = 400

// runAttributes returns the AT-SPI text attributes of one styled run. The order is fixed rather than derived from the
// run, so that the same style always produces the same dictionary: a client that compares two runs to find out where a
// style changes — which is what a review cursor walking a document does — would otherwise be told that two identical
// runs differ.
//
// A run that says nothing about its family or its size reports neither rather than reporting an empty name or a size of
// zero, since an assistive technology reads those as the font itself rather than as the absence of one.
func runAttributes(run accessibility.TextRun) dbus.Dict {
	attributes := make(dbus.Dict, 0, 6)
	attributes = append(attributes,
		dbus.DictEntry{Key: weightTextAttribute, Value: strconv.Itoa(runWeight(run))},
		dbus.DictEntry{Key: styleTextAttribute, Value: runStyle(run)},
		dbus.DictEntry{Key: underlineTextAttribute, Value: runUnderline(run)},
		dbus.DictEntry{Key: strikethroughTextAttribute, Value: strconv.FormatBool(run.Strikethrough)},
	)
	if run.Family != "" {
		attributes = append(attributes, dbus.DictEntry{Key: familyNameTextAttribute, Value: run.Family})
	}
	if run.Size > 0 {
		attributes = append(attributes, dbus.DictEntry{
			Key:   sizeTextAttribute,
			Value: strconv.FormatFloat(float64(run.Size), 'g', -1, 32),
		})
	}
	return attributes
}

// runWeight returns the weight a run is drawn at, which is the regular one for a run that does not say.
func runWeight(run accessibility.TextRun) int {
	if run.Weight <= 0 {
		return regularWeight
	}
	return run.Weight
}

// runStyle returns the value of a run's style attribute.
func runStyle(run accessibility.TextRun) string {
	if run.Italic {
		return italicStyleValue
	}
	return normalStyleValue
}

// runUnderline returns the value of a run's underline attribute. AT-SPI carries it as a name rather than a boolean,
// since there is more than one kind of underline; a Unison font either has one or has not.
func runUnderline(run accessibility.TextRun) string {
	if run.Underline {
		return singleUnderlineValue
	}
	return noUnderlineValue
}

// textRunAt returns the attributes of the text at an offset along with the range over which they hold, which is what
// the three methods that ask about attributes all answer from. count is how many runes the node's text holds.
//
// The range is the run of uniform style that holds the offset, narrowed so that it never crosses into or out of one of
// the objects the text holds; see [narrowToSpans]. An offset outside the text has no attributes and an empty range,
// which is what ATK answers with and what stops a client walking the runs of a control — asking about the offset each
// run ended at — from being handed the whole content again at the end and walking it forever.
//
// A node whose text is synthesized from its value has no runs at all, so the whole of it is one run with no attributes;
// see [textualValue].
func textRunAt(info *accessibility.TextInfo, offset, count int) (attributes dbus.Dict, start, end int) {
	if offset < 0 || offset >= count {
		return dbus.Dict{}, 0, 0
	}
	attributes, start, end = dbus.Dict{}, 0, count
	if info == nil {
		return attributes, start, end
	}
	if run, found := runAt(info.Runs, offset); found {
		attributes = runAttributes(*run)
		start, end = run.Start, run.End
	}
	start, end = narrowToSpans(info.Spans, offset, start, end)
	// The range is held around the offset it was asked about whatever the runs and spans say, since a widget that
	// published runs which do not tile its text would otherwise have a client reading the same run over and over.
	return attributes, clamp(start, 0, offset), clamp(end, offset+1, count)
}

// runAt returns the styled run that holds an offset, or nothing when the runs do not cover it, which is the case for a
// control that draws the whole of its content in one style and publishes no runs at all.
func runAt(runs []accessibility.TextRun, offset int) (run *accessibility.TextRun, found bool) {
	for i := range runs {
		if offset >= runs[i].Start && offset < runs[i].End {
			return &runs[i], true
		}
	}
	return nil, false
}

// narrowToSpans confines a range of uniform style so that it holds the objects the text holds rather than crossing
// their edges: it is cut down to the innermost object that occupies the offset, and kept clear of every object that
// occupies the text on either side of it.
//
// The edges matter as much as the style does. A link within a paragraph is an object of its own, and a client reading
// the paragraph run by run uses those runs to decide where to announce it, so a run that ran from the middle of the
// prose into the middle of the link would have the link announced in the wrong place, or not at all. It is also what
// keeps the runs in step with the spans that [nodeObject.links] hands out.
func narrowToSpans(spans []accessibility.TextSpan, offset, start, end int) (narrowedStart, narrowedEnd int) {
	for _, span := range spans {
		switch {
		case offset >= span.Start && offset < span.End:
			start, end = max(start, span.Start), min(end, span.End)
		case span.Start > offset:
			end = min(end, span.Start)
		default: // The span ends at or before the offset, so the run cannot reach back into it
			start = max(start, span.End)
		}
	}
	return start, end
}

// defaultTextAttributes returns the attributes that hold throughout a node's text unless one of its runs says
// otherwise, which are the attributes of its first run. A Unison text control draws the whole of its content in one
// style, and a document's block begins in the style its prose is in, so the first run is what an assistive technology
// comparing a run against the defaults is best served by. A node with no runs has no defaults to report.
func defaultTextAttributes(info *accessibility.TextInfo) dbus.Dict {
	if info == nil || len(info.Runs) == 0 {
		return dbus.Dict{}
	}
	return runAttributes(info.Runs[0])
}

// attributeValue returns the value of one attribute of a set, or an empty string when the set does not hold it, which
// is how org.a11y.atspi.Text.GetAttributeValue says that an attribute is not there.
func attributeValue(attributes dbus.Dict, name string) string {
	for _, entry := range attributes {
		if key, ok := entry.Key.(string); !ok || key != name {
			continue
		}
		if value, ok := entry.Value.(string); ok {
			return value
		}
		return ""
	}
	return ""
}
