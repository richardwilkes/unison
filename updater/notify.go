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
	"errors"
	"fmt"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/behavior"
)

// The response codes of the Download Page and Sponsor buttons; Cancel and OK already use the predefined ones.
const (
	downloadPageResponse = unison.ModalResponseUserBase + iota
	sponsorResponse
)

// Notify shows the user the update the last check found: the release notes of every release newer than the running
// version, with a button that installs the newest and restarts the application and, when Config has a DownloadPageURL,
// one that opens it. When Config has a SponsorURL, a further button opens that and leaves the dialog up. It does
// nothing when no update is known. When the installation cannot replace itself, the dialog says why in place of
// offering to.
func (u *Updater) Notify() {
	title, releases, _ := u.Status()
	if releases == nil {
		return
	}
	// Check whether this installation can update itself before building the dialog, so it never offers an install
	// that would be refused after a large download.
	p, unavailableMsg := u.planFor(&releases[0])

	// Each release's notes are a document of their own, so that their blocks, link reference definitions and heading
	// IDs cannot reach into another's. Each document but the first begins with the rule setting it apart from the one
	// before, so that nothing follows the notes for them to run on into.
	column := unison.NewPanel()
	column.SetLayout(&unison.FlexLayout{Columns: 1})
	column.SetBorder(unison.NewEmptyBorder(unison.StdInsets()))
	for i, rel := range releases {
		var buffer strings.Builder
		if i == 0 {
			fmt.Fprintf(&buffer, "# %s\n", title)
			if unavailableMsg != "" {
				// An alert ahead of the notes, so that it is not taken for one of them and shows when the dialog opens.
				fmt.Fprintf(&buffer, "> [!IMPORTANT]\n> %s\n\n", unavailableMsg)
			}
		} else {
			buffer.WriteString("---\n")
		}
		fmt.Fprintf(&buffer, "## %s\n",
			i18n.Text("Release Notes for %s %s", u.cfg.AppName, u.formatVersion(rel.Version)))
		buffer.WriteString(rel.Notes)
		md := unison.NewMarkdown(false)
		md.SetContent(buffer.String(), 0)
		column.AddChild(md)
	}

	scroll := unison.NewScrollPanel()
	scroll.SetContent(column, behavior.Unmodified, behavior.Unmodified)
	view := scroll.ContentView()
	// Wraps each document to the view's width, since sizing itself from its parent would wrap it to the column, which
	// is only as wide as the documents already are.
	view.FrameChangeCallback = func() {
		width := view.ContentRect(false).Width - column.Border().Insets().Width()
		for _, child := range column.Children() {
			if md, ok := child.Self.(*unison.Markdown); ok {
				md.SetContentBytes(md.ContentBytes(), width-md.Slop)
			}
		}
	}

	var buttons []*unison.DialogButtonInfo
	if u.cfg.SponsorURL != "" {
		buttons = append(buttons, &unison.DialogButtonInfo{
			Title:        i18n.Text("Sponsor %s", u.cfg.AppName),
			ResponseCode: sponsorResponse,
		})
	}
	buttons = append(buttons, unison.NewCancelButtonInfo())
	if u.cfg.DownloadPageURL != "" {
		buttons = append(buttons, &unison.DialogButtonInfo{
			Title:        i18n.Text("Download Page"),
			ResponseCode: downloadPageResponse,
		})
	}
	if p != nil {
		buttons = append(buttons, unison.NewOKButtonInfoWithTitle(i18n.Text("Install & Restart")))
	}

	icon, ink := u.cfg.Icon, u.cfg.IconInk
	if icon == nil {
		icon = unison.DefaultDialogTheme.QuestionIcon
		if ink == nil {
			ink = unison.DefaultDialogTheme.QuestionIconInk
		}
	} else if ink == nil {
		ink = unison.DefaultLabelTheme.OnBackgroundInk
	}
	dialog, err := unison.NewDialog(icon, ink, scroll, buttons)
	if err != nil {
		errs.Log(err)
		return
	}
	if sponsor := dialog.Button(sponsorResponse); sponsor != nil {
		// Replaces the click that would close the dialog, so the update can still be installed afterward.
		sponsor.ClickCallback = func() {
			if openErr := unison.OpenBrowser(u.cfg.SponsorURL); openErr != nil {
				u.reportError(i18n.Text("Unable to open web page for sponsoring"), openErr)
			}
		}
		// Keeps the initial focus on Cancel, where it is without this button, rather than on the first button.
		dialog.Button(unison.ModalResponseCancel).RequestFocus()
	}
	u.setLastSeen(releases[0].Version)
	switch dialog.RunModal() {
	case unison.ModalResponseOK:
		u.install(p)
	case downloadPageResponse:
		if err = unison.OpenBrowser(u.cfg.DownloadPageURL); err != nil {
			u.reportError(i18n.Text("Unable to open web page for download"), err)
		}
	}
}

// planFor checks whether this installation can replace itself with the given release. It returns either a plan to do
// so, or a message explaining why it cannot, which the dialog shows ahead of the release notes.
func (u *Updater) planFor(release *Release) (p *plan, unavailableMsg string) {
	p, err := u.planFn(release.Version, release.Assets)
	if err == nil {
		return p, ""
	}
	errs.Log(err)
	if refusal, ok := errors.AsType[*unavailable](err); ok {
		return nil, u.blockerMessage(refusal.Blocker)
	}
	return nil, i18n.Text("%s can't install this update automatically.", u.cfg.AppName)
}

// blockerMessage explains, in the user's language, why an update cannot be installed automatically and, where possible,
// what to do about it. Preflight reports stable identifiers so that translation happens here, at display time.
func (u *Updater) blockerMessage(b blocker) string {
	switch b {
	case blockerDevBuild:
		return i18n.Text("Development builds can't be updated automatically.")
	case blockerRenamedExecutable:
		return i18n.Text("This copy of %s has been renamed, so it can't be updated automatically.", u.cfg.AppName)
	case blockerNotABundle:
		return i18n.Text("This copy of %s isn't installed as an application, so it can't be updated automatically.",
			u.cfg.AppName)
	case blockerTranslocated:
		return i18n.Text("%s is running from a temporary copy. Move it to your Applications folder and open it from there to enable automatic updates.",
			u.cfg.AppName)
	case blockerPackageManaged:
		return i18n.Text("%s was installed by a package manager, which should be used to update it.", u.cfg.AppName)
	case blockerHomebrew:
		return i18n.Text("This copy was installed by Homebrew. Run `brew upgrade --cask %s` to update it.",
			u.cfg.HomebrewCask)
	case blockerReadOnly:
		return i18n.Text("%s can't write to the folder it's installed in, so it can't update itself. Installing it somewhere you have permission to write, such as your Applications folder, enables automatic updates.",
			u.cfg.AppName)
	case blockerNoAsset:
		return i18n.Text("This release doesn't include a build for this system.")
	case blockerNoDigest:
		return i18n.Text("This release can't be verified automatically, so it has to be installed by hand.")
	default:
		return i18n.Text("%s can't install this update automatically.", u.cfg.AppName)
	}
}

// reportError tells the user about a failure, through the application's handler if it has one.
func (u *Updater) reportError(title string, err error) {
	if u.cfg.ErrorHandler != nil {
		u.cfg.ErrorHandler(title, err)
		return
	}
	unison.ErrorDialogWithError(title, err)
}
