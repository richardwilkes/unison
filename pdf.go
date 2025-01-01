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
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/unison/internal/skia"
)

// PDFMetaData holds the metadata about a PDF document.
type PDFMetaData = skia.MetaData

// PageProvider defines the methods required of a PDF producer.
type PageProvider interface {
	HasPage(pageNumber int) bool
	PageSize() Size
	DrawPage(canvas *Canvas, pageNumber int) error
}

// CreatePDF writes a PDF to the given stream. md may be nil.
func CreatePDF(s Stream, md *PDFMetaData, pageProvider PageProvider) error {
	if md == nil {
		md = &PDFMetaData{}
	}
	d := skia.DocumentMakePDF(s.asWStream(), md)
	if d == nil {
		return errs.New("unable to create PDF")
	}
	pageNumber := 1
	for pageProvider.HasPage(pageNumber) {
		size := pageProvider.PageSize()
		canvas := &Canvas{canvas: skia.DocumentBeginPage(d, size.Width, size.Height)}
		if err := pageProvider.DrawPage(canvas, pageNumber); err != nil {
			skia.DocumentAbort(d)
			return errs.Wrap(err)
		}
		skia.DocumentEndPage(d)
		pageNumber++
	}
	skia.DocumentClose(d)
	return nil
}
