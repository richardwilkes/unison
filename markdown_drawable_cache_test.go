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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestMarkdownFailedImageLoadIsRetried verifies that a failed image load does not permanently poison the drawable
// cache: the failed entry must be removed so a later rebuild retries the load (and does not keep appending dead
// placeholder panels to the stale entry's target list). The load is then made to succeed at the same path to prove
// the retry actually happens.
func TestMarkdownFailedImageLoadIsRetried(t *testing.T) {
	c := check.New(t)
	m := NewMarkdown(false)
	dir := t.TempDir()
	m.WorkingDirProvider = func(_ Paneler) string { return dir }
	const target = "image.svg"

	// First attempt: the file does not exist, so the load fails and must not remain cached.
	c.Nil(m.retrieveImage(target, NewDrawablePanel()))
	deadline := time.Now().Add(10 * time.Second)
	for {
		m.drawableCacheLock.Lock()
		remaining := len(m.drawableCache)
		m.drawableCacheLock.Unlock()
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the failed image load was never removed from the drawable cache")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Second attempt: the file now exists, so the retry must succeed rather than being served the poisoned entry.
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`
	c.NoError(os.WriteFile(filepath.Join(dir, target), []byte(svg), 0o600))
	c.NotNil(m.retrieveImage(target, NewDrawablePanel()))
}
