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
	"github.com/richardwilkes/canvas/pdf"
	"github.com/richardwilkes/canvas/stream"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// PageProvider defines the methods required of a PDF producer.
type PageProvider interface {
	HasPage(pageNumber int) bool
	PageSize() geom.Size
	DrawPage(canvas *Canvas, pageNumber int) error
}

// CreatePDF writes a PDF to the given stream. md may be nil.
func CreatePDF(s stream.WStream, md *pdf.Metadata, pageProvider PageProvider) error {
	d := pdf.NewDocument(s, md)
	if d == nil {
		return errs.New("unable to create PDF")
	}
	pageNumber := 1
	for pageProvider.HasPage(pageNumber) {
		size := pageProvider.PageSize()
		canvas := &Canvas{canvas: d.BeginPageCanvas(size.Width, size.Height)}
		if err := pageProvider.DrawPage(canvas, pageNumber); err != nil {
			d.Abort()
			return errs.Wrap(err)
		}
		d.EndPage()
		pageNumber++
	}
	d.Close()
	return nil
}
