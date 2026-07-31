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
	"slices"

	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
)

// ClipboardHasText returns true if the clipboard contains text.
func ClipboardHasText() bool {
	return ClipboardHasDataType(uti.UTF8PlainText)
}

// ClipboardGetText returns text from the clipboard.
func ClipboardGetText() string {
	data := ClipboardGetData(uti.UTF8PlainText)
	if data == nil {
		return ""
	}
	return string(data)
}

// ClipboardSetText sets text onto the clipboard, replacing the previous content.
func ClipboardSetText(text string) {
	ClipboardSetData(drag.Data{
		Type: uti.UTF8PlainText,
		Data: []byte(text),
	})
}

// ClipboardHasDataType returns true if the clipboard contains data of the specified type.
func ClipboardHasDataType(dataType *uti.DataType) bool {
	return apiClipboardHasDataType(selectDataType(dataType, apiClipboardAvailableDataTypes()))
}

// ClipboardGetData returns the data associated with the specified type on the clipboard.
func ClipboardGetData(dataType *uti.DataType) []byte {
	return apiClipboardGetData(selectDataType(dataType, apiClipboardAvailableDataTypes()))
}

// ClipboardSetData sets data onto the clipboard, replacing the previous content.
func ClipboardSetData(data ...drag.Data) {
	apiClipboardSetData(data...)
}

func selectDataType(desiredType *uti.DataType, availableDataTypes []string) *uti.DataType {
	if slices.Contains(availableDataTypes, desiredType.UTI) {
		return desiredType
	}
	for _, one := range availableDataTypes {
		// Accept an available type that is a descendant of the desired type (e.g. public.utf8-plain-text satisfies a
		// request for public.plain-text) as well as an ancestor of it, since either can service the request.
		if lookup := uti.ByUTI(one); lookup != nil &&
			(lookup.ConformsTo(desiredType) || desiredType.ConformsTo(lookup)) {
			return lookup
		}
	}
	return desiredType
}
