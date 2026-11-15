// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package updater

import (
	"fmt"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const (
	// testDownloadPage is where the tests say a release can be downloaded by hand.
	testDownloadPage = "https://example.com/download"
	// testSponsorPage is where the tests say the application can be sponsored.
	testSponsorPage = "https://example.com/sponsor"
)

// TestNotifyOffersOnlyWhatIsPossible verifies the buttons the dialog has: Install & Restart only when this installation
// can replace itself, and Download Page and Sponsor only when the application has a page for each. The dialog records
// the release as seen and starts with the focus on Cancel, the Sponsor button opens its page and leaves the dialog up,
// and the Download Page button opens its page and closes the dialog.
func TestNotifyOffersOnlyWhatIsPossible(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t)
	for _, tc := range []struct {
		planErr      error
		name         string
		downloadPage string
		sponsorPage  string
		wantInstall  bool
	}{
		{name: "installable, with a download page", downloadPage: testDownloadPage, wantInstall: true},
		{
			name:         "installable, with download and sponsor pages",
			downloadPage: testDownloadPage,
			sponsorPage:  testSponsorPage,
			wantInstall:  true,
		},
		{name: "installable, with a sponsor page", sponsorPage: testSponsorPage, wantInstall: true},
		{name: "installable, without a download page", wantInstall: true},
		{
			name:         "not installable",
			downloadPage: testDownloadPage,
			planErr:      &unavailable{Blocker: blockerReadOnly},
		},
	} {
		var seen []string
		u := New(&Config{
			AppName:            testApp,
			CmdName:            testCmd,
			Version:            testVersion,
			DownloadPageURL:    tc.downloadPage,
			SponsorURL:         tc.sponsorPage,
			SetLastSeenVersion: func(version string) { seen = append(seen, version) },
		})
		u.planFn = func(string, []Asset) (*plan, error) {
			if tc.planErr != nil {
				return nil, tc.planErr
			}
			return &plan{ToVersion: testNextVersion}, nil
		}
		u.setReleases([]Release{{Version: testNextVersion, Notes: "- Something new"}})
		c.True(screen.Post(u.Notify))
		screen.Sync()
		dialog := modalDialog(screen)
		if dialog == nil {
			t.Fatalf("%s: the dialog did not open", tc.name)
		}
		var cancel, download, sponsor, install *unison.Button
		var cancelFocused bool
		c.True(screen.Do(func() {
			cancel = dialog.Button(unison.ModalResponseCancel)
			download = dialog.Button(downloadPageResponse)
			sponsor = dialog.Button(sponsorResponse)
			cancelFocused = cancel != nil && cancel.Focused()
			install = dialog.Button(unison.ModalResponseOK)
		}))
		c.NotNil(cancel, "%s", tc.name)
		c.True(cancelFocused, "%s: Cancel must start with the focus", tc.name)
		c.Equal(tc.wantInstall, install != nil, "%s", tc.name)
		c.Equal(tc.downloadPage != "", download != nil, "%s", tc.name)
		c.Equal(tc.sponsorPage != "", sponsor != nil, "%s", tc.name)
		c.Equal([]string{testNextVersion}, seen, "%s", tc.name)

		screen.OpenedURLs() // Forgets anything opened before the clicks
		if sponsor != nil {
			screen.Click(screen.PanelCenter(sponsor))
			c.Equal([]string{testSponsorPage}, screen.OpenedURLs(), "%s", tc.name)
			c.Equal(dialog, modalDialog(screen), "%s: the dialog must stay up", tc.name)
		}
		if download != nil {
			screen.Click(screen.PanelCenter(download))
			c.Equal([]string{testDownloadPage}, screen.OpenedURLs(), "%s", tc.name)
		} else {
			screen.Click(screen.PanelCenter(cancel))
		}
		c.Nil(modalDialog(screen), "%s: the dialog must close", tc.name)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestNotifyKeepsReleaseNotesApart verifies that the notes of each release are read on their own: whatever block they
// end in ends before the rule and heading of the next release, and their links lead where their own reference
// definitions say, whatever another release defines under the same label.
func TestNotifyKeepsReleaseNotesApart(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t)
	const olderLink = "https://example.com/1.2.4"
	for _, tc := range []struct {
		name  string
		notes string
		want  []string // The top-level blocks the notes become
		links []string // Where the links of the notes lead
	}{
		{
			name:  "a paragraph",
			notes: "- New\n\n**Full Changelog**: https://example.com/compare/v1.2.4...v1.3.0",
			want:  []string{"List", "Paragraph"},
		},
		{name: "an unclosed code fence", notes: "```go\nfmt.Println()", want: []string{`code "fmt.Println()\n"`}},
		{
			name:  "a link reference the older release defines too",
			notes: "- [New][1]\n\n[1]: https://example.com/1.3.0",
			want:  []string{"List"},
			links: []string{"https://example.com/1.3.0"},
		},
	} {
		u := New(&Config{AppName: testApp, CmdName: testCmd, Version: testVersion})
		u.planFn = func(string, []Asset) (*plan, error) { return &plan{ToVersion: testNextVersion}, nil }
		u.setReleases([]Release{
			{Version: testNextVersion, Notes: tc.notes},
			{Version: "1.2.4", Notes: "- [Older][1]\n\n[1]: " + olderLink},
		})
		c.True(screen.Post(u.Notify))
		screen.Sync()
		dialog := modalDialog(screen)
		if dialog == nil {
			t.Fatalf("%s: the dialog did not open", tc.name)
		}
		var contents [][]byte
		var cancel *unison.Button
		c.True(screen.Do(func() {
			contents = markdownContents(dialog.Window().Content())
			cancel = dialog.Button(unison.ModalResponseCancel)
		}))
		var got, links []string
		for _, content := range contents {
			doc := unison.NewMarkdownParser().Parse(text.NewReader(content))
			for block := doc.FirstChild(); block != nil; block = block.NextSibling() {
				switch b := block.(type) {
				case *ast.Heading:
					got = append(got, fmt.Sprintf("h%d %s", b.Level, b.Lines().Value(content)))
				case *ast.ThematicBreak:
					got = append(got, "rule")
				case *ast.FencedCodeBlock:
					got = append(got, fmt.Sprintf("code %q", b.Lines().Value(content)))
				case *ast.LinkReferenceDefinition:
					// Shows nothing
				default:
					got = append(got, block.Kind().String())
				}
			}
			c.NoError(ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
				if link, ok := n.(*ast.Link); ok && entering {
					links = append(links, string(link.Destination))
				}
				return ast.WalkContinue, nil
			}))
		}
		want := append([]string{"h1 Example 1.3.0 is available!", "h2 Release Notes for Example 1.3.0"}, tc.want...)
		want = append(want, "rule", "h2 Release Notes for Example 1.2.4", "List")
		c.Equal(want, got, "%s", tc.name)
		c.Equal(append(tc.links, olderLink), links, "%s: where the links lead", tc.name)
		screen.Click(screen.PanelCenter(cancel))
		c.Nil(modalDialog(screen), "%s: the dialog must close", tc.name)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// markdownContents returns the content of each Markdown found in the panel or its descendants, in the order they are
// shown.
func markdownContents(p *unison.Panel) [][]byte {
	if md, ok := p.Self.(*unison.Markdown); ok {
		return [][]byte{md.ContentBytes()}
	}
	var contents [][]byte
	for _, child := range p.Children() {
		contents = append(contents, markdownContents(child)...)
	}
	return contents
}

// With no update known there is nothing to show, so no dialog opens.
func TestNotifyDoesNothingWithoutAnUpdate(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t)
	u := newTestUpdater(t)
	// Posted rather than run with Do, which would never return were a dialog to open.
	c.True(screen.Post(u.Notify))
	screen.Sync()
	c.Nil(modalDialog(screen))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// Each reason an installation cannot update itself has its own explanation, and the one for Homebrew names the cask the
// application says it is installed by.
func TestBlockerMessages(t *testing.T) {
	c := check.New(t)
	u := New(&Config{AppName: testApp, CmdName: testCmd, Version: testVersion, HomebrewCask: "example-cask"})
	fallback := u.blockerMessage("not-yet-invented")
	seen := map[string]blocker{}
	for _, b := range []blocker{
		blockerDevBuild, blockerRenamedExecutable, blockerNotABundle, blockerTranslocated, blockerPackageManaged,
		blockerHomebrew, blockerReadOnly, blockerNoAsset, blockerNoDigest,
	} {
		msg := u.blockerMessage(b)
		c.NotEqual(fallback, msg, "%s must have its own message", b)
		other, dup := seen[msg]
		c.False(dup, "%s and %s share a message", b, other)
		seen[msg] = b
	}
	c.Contains(u.blockerMessage(blockerHomebrew), "brew upgrade --cask example-cask")
}
