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
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xio/fs"
)

const pathSeparator = string(os.PathSeparator)

type fileDialog struct {
	fileCommon
	currentDir     string
	currentExt     string
	dirEntries     []os.DirEntry
	dialog         *Dialog
	parentDirPopup *PopupMenu[*parentDirItem]
	fileNameField  *Field
	scroller       *ScrollPanel
	fileList       *List
	filterPopup    *PopupMenu[string]
	forOpen        bool
}

// NewCommonSaveDialog creates a new SaveDialog. This is the fallback Go-only version of the SaveDialog used when
// a non-platform-native version doesn't exist. Where possible, use of NewSaveDialog() should be preferred, since
// platforms like macOS and Windows usually have restrictions on file access that their native dialogs automatically
// remove for the user.
func NewCommonSaveDialog() SaveDialog {
	d := &fileDialog{}
	d.initialize()
	return d
}

// NewCommonOpenDialog creates a new OpenDialog. This is the fallback Go-only version of the OpenDialog used when a
// non-platform-native version doesn't exist. Where possible, use of NewOpenDialog() should be preferred, since
// platforms like macOS and Windows usually have restrictions on file access that their native dialogs automatically
// remove for the user.
func NewCommonOpenDialog() OpenDialog {
	d := &fileDialog{forOpen: true}
	d.initialize()
	return d
}

func (d *fileDialog) RunModal() bool {
	var dialogTitle, okTitle string
	if d.forOpen {
		dialogTitle = i18n.Text("Open…")
		okTitle = i18n.Text("Open")
	} else {
		dialogTitle = i18n.Text("Save…")
		okTitle = i18n.Text("Save")
	}
	dlg, err := NewDialog(nil, nil, d.createContent(), []*DialogButtonInfo{
		NewCancelButtonInfo(),
		NewOKButtonInfoWithTitle(okTitle),
	})
	if err != nil {
		ErrorDialogWithError(i18n.Text("Unable to create file dialog."), err)
		return false
	}
	dlg.Window().SetTitle(dialogTitle)
	d.dialog = dlg
	d.dialog.Button(ModalResponseOK).SetEnabled(false)
	return dlg.RunModal() == ModalResponseOK
}

func (d *fileDialog) createContent() *Panel {
	if len(d.extensions) > 0 {
		d.currentExt = d.extensions[0]
	} else {
		d.currentExt = "*"
	}
	d.prepareCurrentDir(d.initialDir)

	content := NewPanel()
	content.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})

	d.parentDirPopup = NewPopupMenu[*parentDirItem]()
	d.parentDirPopup.SelectionCallback = d.parentDirPopupSelectionHandler
	d.rebuildParentDirs()
	content.AddChild(d.parentDirPopup)
	d.parentDirPopup.SetLayoutData(&FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: MiddleAlignment,
		VAlign: MiddleAlignment,
		HGrab:  true,
	})

	if !d.forOpen {
		d.fileNameField = NewField()
		d.fileNameField.ModifiedCallback = d.fileNameFieldModified
		d.fileNameField.KeyDownCallback = d.fileNameFieldKeyDown
		content.AddChild(d.fileNameField)
		d.fileNameField.SetLayoutData(&FlexLayoutData{
			HSpan:  1,
			VSpan:  1,
			HAlign: FillAlignment,
			VAlign: MiddleAlignment,
			HGrab:  true,
		})
	}

	d.fileList = NewList().SetAllowMultipleSelection(d.allowMultipleSelection)
	d.fileList.NewSelectionCallback = d.fileListSelectionHandler
	d.fileList.DoubleClickCallback = d.fileListDoubleClickHandler
	d.rebuildFileList()
	d.scroller = NewScrollPanel()
	d.scroller.SetBorder(NewLineBorder(ControlEdgeColor, 0, NewUniformInsets(1), false))
	d.scroller.SetContent(d.fileList, FollowsWidthBehavior)
	content.AddChild(d.scroller)
	d.scroller.SetLayoutData(&FlexLayoutData{
		MinSize: NewSize(300, 200),
		HSpan:   1,
		VSpan:   1,
		HAlign:  FillAlignment,
		VAlign:  FillAlignment,
		HGrab:   true,
		VGrab:   true,
	})

	if len(d.extensions) > 1 {
		d.filterPopup = NewPopupMenu[string]()
		for _, ext := range d.extensions {
			if ext == "*" {
				d.filterPopup.AddItem(i18n.Text("Any File"))
			} else {
				d.filterPopup.AddItem("*." + ext)
			}
		}
		d.filterPopup.SelectionCallback = d.filterHandler
		content.AddChild(d.filterPopup)
		d.filterPopup.SetLayoutData(&FlexLayoutData{
			HSpan:  1,
			VSpan:  1,
			HAlign: MiddleAlignment,
			VAlign: MiddleAlignment,
			HGrab:  true,
		})
	}

	return content
}

func (d *fileDialog) rebuildParentDirs() {
	d.parentDirPopup.RemoveAllItems()
	dir := d.currentDir
	for {
		parent, f := filepath.Split(dir)
		if dir == pathSeparator {
			f = pathSeparator
		}
		d.parentDirPopup.AddItem(&parentDirItem{
			name: f,
			path: dir,
		})
		if dir == pathSeparator {
			break
		}
		if parent != pathSeparator {
			parent = strings.TrimRight(parent, pathSeparator)
		}
		if parent == "" {
			break
		}
		dir = parent
	}
	d.parentDirPopup.MarkForLayoutAndRedraw()
	if p := d.parentDirPopup.Parent(); p != nil {
		p.NeedsLayout = true
	}
}

func (d *fileDialog) parentDirPopupSelectionHandler(index int, item *parentDirItem) {
	if index != 0 {
		d.changeDirTo(item.path)
	}
}

func (d *fileDialog) changeDirTo(dir string) {
	d.prepareCurrentDir(dir)
	d.rebuildParentDirs()
	d.rebuildFileList()
}

func (d *fileDialog) fileListSelectionHandler() {
	d.paths = make([]string, 0, d.fileList.Selection.Count())
	okEnabled := false
	i := d.fileList.Selection.FirstSet()
	for i != -1 {
		okEnabled = true
		if item, ok := d.fileList.DataAtIndex(i).(*fileListItem); ok {
			d.paths = append(d.paths, filepath.Join(d.currentDir, item.entry.Name()))
			switch {
			case item.entry.IsDir():
				if !d.forOpen || !d.canChooseDirs {
					okEnabled = false
				}
			case d.forOpen:
				if !d.canChooseFiles {
					okEnabled = false
				}
			default:
				d.fileNameField.SetText(item.entry.Name())
				d.fileNameField.SelectAll()
			}
		}
		i = d.fileList.Selection.NextSet(i + 1)
	}
	d.dialog.Button(ModalResponseOK).SetEnabled(okEnabled)
}

func (d *fileDialog) fileListDoubleClickHandler() {
	i := d.fileList.Anchor()
	if i == -1 {
		i = d.fileList.Selection.FirstSet()
	}
	if item, ok := d.fileList.DataAtIndex(i).(*fileListItem); ok {
		if item.entry.IsDir() {
			d.fileList.Selection.Reset()
			d.fileList.Selection.Set(i)
			d.fileList.FlashSelection()
			d.fileList.Selection.Reset()
			d.changeDirTo(filepath.Join(d.currentDir, item.entry.Name()))
			d.paths = nil
			return
		}
	}
	if d.dialog.Button(ModalResponseOK).Enabled() {
		d.fileList.FlashSelection()
		d.dialog.StopModal(ModalResponseOK)
	}
}

func (d *fileDialog) fileNameFieldKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if mod == NoModifiers && (keyCode == KeyReturn || keyCode == KeyNumPadEnter) {
		if d.fileNameField.Text() != "" {
			d.dialog.StopModal(ModalResponseOK)
		}
		return true
	}
	return d.fileNameField.DefaultKeyDown(keyCode, mod, repeat)
}

func (d *fileDialog) fileNameFieldModified() {
	text := d.fileNameField.Text()
	if text != "" && d.currentExt != "*" {
		e := filepath.Ext(text)
		if e == "" || !strings.EqualFold(e[1:], d.currentExt) {
			text += "." + d.currentExt
		}
	}
	d.paths = []string{text}
	d.dialog.Button(ModalResponseOK).SetEnabled(text != "")
}

func (d *fileDialog) rebuildFileList() {
	d.fileList.RemoveRange(0, d.fileList.Count()-1)
	for _, entry := range d.dirEntries {
		if d.currentExt != "*" && !entry.IsDir() {
			e := filepath.Ext(entry.Name())
			if e == "" {
				continue
			}
			if !strings.EqualFold(e[1:], d.currentExt) {
				continue
			}
		}
		d.fileList.Append(&fileListItem{entry: entry})
	}
	if d.fileList.Parent() != nil {
		_, pref, _ := d.fileList.Sizes(Size{})
		d.fileList.SetFrameRect(Rect{Size: pref})
		d.scroller.SetPosition(0, 0)
	}
}

func (d *fileDialog) filterHandler(index int, _ string) {
	d.currentExt = d.extensions[index]
	d.rebuildFileList()
}

func (d *fileDialog) prepareCurrentDir(dir string) {
	d.currentDir = resolveToAcceptableAbsDir(dir)
	var err error
	if d.dirEntries, err = os.ReadDir(d.currentDir); err != nil {
		jot.Error(err)
	}
}

type fileCommon struct {
	initialDir             string
	extensions             []string
	paths                  []string
	canChooseFiles         bool
	canChooseDirs          bool
	resolvesAliases        bool
	allowMultipleSelection bool
}

func (fc *fileCommon) initialize() {
	if lastWorkingDir == "" {
		lastWorkingDir = resolveToAcceptableAbsDir(".")
	}
	fc.initialDir = lastWorkingDir
	fc.extensions = nil
	fc.paths = nil
	fc.canChooseFiles = true
	fc.canChooseDirs = false
	fc.resolvesAliases = true
	fc.allowMultipleSelection = false
}

func (fc *fileCommon) InitialDirectory() string {
	return fc.initialDir
}

func (fc *fileCommon) SetInitialDirectory(dir string) {
	fc.initialDir = dir
}

func (fc *fileCommon) AllowedExtensions() []string {
	if len(fc.extensions) == 0 {
		return nil
	}
	ext := make([]string, len(fc.extensions))
	copy(ext, fc.extensions)
	return ext
}

func (fc *fileCommon) SetAllowedExtensions(types ...string) {
	fc.extensions = SanitizeExtensionList(types)
}

func (fc *fileCommon) CanChooseFiles() bool {
	return fc.canChooseFiles
}

func (fc *fileCommon) SetCanChooseFiles(canChoose bool) {
	fc.canChooseFiles = canChoose
}

func (fc *fileCommon) CanChooseDirectories() bool {
	return fc.canChooseDirs
}

func (fc *fileCommon) SetCanChooseDirectories(canChoose bool) {
	fc.canChooseDirs = canChoose
}

func (fc *fileCommon) ResolvesAliases() bool {
	return fc.resolvesAliases
}

func (fc *fileCommon) SetResolvesAliases(resolves bool) {
	fc.resolvesAliases = resolves
}

func (fc *fileCommon) AllowsMultipleSelection() bool {
	return fc.allowMultipleSelection
}

func (fc *fileCommon) SetAllowsMultipleSelection(allow bool) {
	fc.allowMultipleSelection = allow
}

func (fc *fileCommon) Path() string {
	if len(fc.paths) > 0 {
		return fc.paths[0]
	}
	return ""
}

func (fc *fileCommon) Paths() []string {
	return fc.paths
}

type parentDirItem struct {
	name string
	path string
}

func (p *parentDirItem) String() string {
	return p.name
}

type fileListItem struct {
	entry os.DirEntry
}

func (f *fileListItem) String() string {
	name := f.entry.Name()
	if f.entry.IsDir() {
		name += string(os.PathSeparator)
	}
	return name
}

func resolveToAcceptableAbsDir(dir string) string {
	if d := dirToAbsDirOnly(dir); d != "" {
		return d
	}
	if d := dirToAbsDirOnly("."); d != "" {
		return d
	}
	if d, err := os.UserHomeDir(); err == nil {
		if d = dirToAbsDirOnly(d); d != "" {
			return d
		}
	}
	if u, err := user.Current(); err == nil {
		if d := dirToAbsDirOnly(u.HomeDir); d != "" {
			return d
		}
	}
	return "/"
}

func dirToAbsDirOnly(dir string) string {
	if d, err := filepath.Abs(dir); err == nil && fs.IsDir(d) {
		return d
	}
	return ""
}
