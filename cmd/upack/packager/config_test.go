// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package packager

import (
	"strings"
	"testing"
)

func TestValidateOwnerRequiresExtensions(t *testing.T) {
	// An Owner entry's first extension names its Windows icon resource, so an Owner entry without extensions must be
	// rejected up front rather than panicking with an index-out-of-range later.
	cfg := &Config{FileInfo: []*FileData{{Name: "Doc", Rank: rankOwner}}}
	err := cfg.validate()
	if err == nil {
		t.Fatal("expected an error for an Owner file_info entry with no extensions")
	}
	if !strings.Contains(err.Error(), "no extensions") {
		t.Errorf("unexpected error: %v", err)
	}
	cfg.FileInfo[0].Extensions = []string{"doc"}
	if err = cfg.validate(); err != nil {
		t.Errorf("unexpected error for an Owner entry with extensions: %v", err)
	}
	cfg = &Config{FileInfo: []*FileData{{Name: "Other", Rank: "Alternate"}}}
	if err = cfg.validate(); err != nil {
		t.Errorf("unexpected error for a non-Owner entry with no extensions: %v", err)
	}
	if err = (&Config{}).validate(); err != nil {
		t.Errorf("unexpected error for an empty configuration: %v", err)
	}
}
