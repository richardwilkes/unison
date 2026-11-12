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

// downloadPageResponse is the Download Page button's response code; Cancel and OK already use the predefined ones.
const downloadPageResponse = unison.ModalResponseUserBase

// Notify shows the user the update the last check found: the release notes of every release newer than the running
// version, with a button that installs the newest and restarts the application and, when Config has a DownloadPageURL,
// one that opens it. It does nothing when no update is known. When the installation cannot replace itself, the dialog
// says why in place of offering to.
func (u *Updater) Notify() {
	title, releases, _ := u.Status()
	if releases == nil {
		return
	}
	// Check whether this installation can update itself before building the dialog, so it never offers an install
	// that would be refused after a large download.
	p, unavailableMsg := u.planFor(&releases[0])

	var buffer strings.Builder
	fmt.Fprintf(&buffer, "# %s\n", title)
	for i, rel := range releases {
		if i != 0 {
			buffer.WriteString("---\n")
		}
		fmt.Fprintf(&buffer, "## %s\n",
			i18n.Text("Release Notes for %s %s", u.cfg.AppName, u.formatVersion(rel.Version)))
		buffer.WriteString(rel.Notes)
		buffer.WriteByte('\n')
	}
	if unavailableMsg != "" {
		fmt.Fprintf(&buffer, "---\n%s\n", unavailableMsg)
	}

	md := unison.NewMarkdown(true)
	md.SetBorder(unison.NewEmptyBorder(unison.StdInsets()))
	md.SetContent(buffer.String(), 0)

	scroll := unison.NewScrollPanel()
	scroll.SetContent(md, behavior.Unmodified, behavior.Unmodified)

	buttons := []*unison.DialogButtonInfo{unison.NewCancelButtonInfo()}
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
// so, or a message explaining why it cannot, which the dialog shows alongside the release notes.
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
